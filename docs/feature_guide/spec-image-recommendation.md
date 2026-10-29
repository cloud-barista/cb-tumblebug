# Spec and Image Recommendation

Guide for selecting optimal compute specs and images, verifying real-time CSP availability, enforcing spec-image compatibility, reviewing spec-image pairs, and finding alternative configurations across CSPs.

## 📑 Table of Contents

1. [Overview](#overview)
2. [Spec Recommendation](#spec-recommendation)
3. [Real-Time Spec Availability & Provisioning Feedback Loop](#real-time-spec-availability--provisioning-feedback-loop)
4. [Image Search & Spec-Image Compatibility](#image-search--spec-image-compatibility)
5. [Spec-Image Pair Review & Pre-Flight Validation](#spec-image-pair-review--pre-flight-validation)
6. [Alternative Node Config](#alternative-node-config)
7. [Workflow Example](#workflow-example)

---

## Overview

CB-Tumblebug provides a set of APIs to help users select compute resources for multi-cloud infrastructure provisioning. These APIs work together in a natural progression from discovery and compatibility filtering to pre-flight review and cross-CSP portability:

```
RecommendSpec  →  SearchImage  →  InfraDynamicReview  →  RecommendAlternativeNodeConfig
  Find spec &       Find spec-      Validate availability   Port config to another CSP
  check stock     compatible OS     & pair compatibility
```

| API | Method | Path |
|-----|--------|------|
| Recommend Spec | POST | `/tumblebug/recommendSpec` |
| Get Recommendation Options | GET | `/tumblebug/recommendSpecOptions` |
| Filter Specs by Range | POST | `/tumblebug/ns/{nsId}/resources/filterSpecsByRange` |
| Get Single Spec (On-Demand Availability) | GET | `/tumblebug/ns/{nsId}/resources/spec/{specId}?forceRefresh=true` |
| Search Image | POST | `/tumblebug/ns/{nsId}/resources/searchImage` |
| Get Search Options | GET | `/tumblebug/ns/{nsId}/resources/searchImageOptions` |
| Review Dynamic Infra (Spec+Image Validation) | POST | `/tumblebug/ns/{nsId}/infraDynamicReview` |
| Check Dynamic Infra Request | POST | `/tumblebug/ns/{nsId}/checkInfraDynamicReq` |
| Recommend Alternative Config | POST | `/tumblebug/recommendAlternativeNodeConfig` |

---

## Spec Recommendation

### What It Does

`POST /tumblebug/recommendSpec` filters and ranks compute specs from the global spec catalog based on user-defined conditions and priorities. It returns a sorted list of specs enriched with real-time CSP availability metadata.

### Request

```json
{
  "filter": {
    "policy": [
      { "metric": "vCPU",        "condition": [{ "operator": ">=", "operand": "4" }] },
      { "metric": "memoryGiB",   "condition": [{ "operator": ">=", "operand": "16" }] },
      { "metric": "costPerHour", "condition": [{ "operator": "<=", "operand": "0.5" }] }
    ]
  },
  "priority": {
    "policy": [
      { "metric": "cost",     "weight": 0.5 },
      { "metric": "location", "weight": 0.3,
        "parameter": [{ "key": "coordinateClose", "val": ["37.5665/126.9780"] }] }
    ]
  },
  "limit": 10
}
```

### Filter Metrics

| Metric | Operators | Example |
|--------|-----------|---------|
| `vCPU` | `>=`, `<=`, `==` | `>= 4` |
| `memoryGiB` | `>=`, `<=`, `==` | `>= 16` |
| `costPerHour` | `>=`, `<=` | `<= 0.5` |
| `architecture` | `==` | `== x86_64` |
| `providerName` | `==` | `== aws` |
| `regionName` | `==` | `== ap-northeast-2` |
| `acceleratorType` | `==` | `== gpu` |
| `acceleratorModel` | `==`, `like` | `NVIDIA H100` |
| `acceleratorCount` | `>=`, `<=`, `==` | `>= 1` |
| `acceleratorMemoryGB` | `>=`, `<=`, `==` | `>= 80` |

### Priority Metrics

| Metric | Description |
|--------|-------------|
| `cost` | Ascending by `costPerHour` (cheapest first) |
| `performance` | Descending by benchmark score |
| `location` | By proximity to given coordinates |
| `latency` | By measured network latency |
| `random` | Randomized order |

### Get Available Filter/Priority Options

```
GET /tumblebug/recommendSpecOptions
```

Returns all valid values for `metric`, `operator`, and `priority` fields — useful for building dynamic UIs or validating input before calling `recommendSpec`.

---

## Real-Time Spec Availability & Provisioning Feedback Loop

Static spec catalogs (`spec_infos`) cannot reflect whether a spec is currently in stock in a specific Availability Zone (AZ), whether the user's subscription has sufficient vCPU/GPU quota, or whether a recent VM creation failed due to transient capacity exhaustion. CB-Tumblebug enriches `SpecInfo` responses with standardized runtime availability fields.

### Availability Fields in `SpecInfo`

```json
{
  "id": "aws+us-east-1+p5.4xlarge",
  "providerName": "aws",
  "regionName": "us-east-1",
  "cspSpecName": "p5.4xlarge",
  "availabilityStatus": "Available",
  "availabilityReason": "zone us-east-1b unavailable/SoldOut (InsufficientInstanceCapacity: ...); available in [us-east-1a, us-east-1c, us-east-1d, us-east-1e, us-east-1f]",
  "availableZones": ["us-east-1a", "us-east-1c", "us-east-1d", "us-east-1e", "us-east-1f"],
  "unavailableZones": ["us-east-1b"],
  "availabilityCheckedAt": "2026-10-30T00:34:03Z",
  "availabilityRefreshAfter": "2026-10-30T01:04:03Z"
}
```

### Standardized `availabilityStatus` Values

| Status | Meaning | Provisioning Behavior |
|--------|---------|-----------------------|
| `Available` | Spec passed pre-flight offering/stock/quota checks in at least one zone. If some zones in the region are unavailable (`len(unavailableZones) > 0`), partial zone availability (e.g., `Available (5/6 AZ)`) and `availabilityReason` are reported. | Allowed; auto-zone selection avoids `unavailableZones`. |
| `SoldOut` | Spec is temporarily out of stock across all zones in the region (or in the user-pinned zone). | Blocked in review; warns user to select another region/spec. |
| `QuotaExceeded` | Credential holder's regional vCPU or GPU quota is insufficient, or the subscription requires opt-in/quota increase for this family. | Blocked in review with quota details. |
| `Unsupported` | Spec is retired, restricted, or not offered in any zone of the target region. | Blocked in review. |
| `Unverified` | Real-time availability has not been queried yet (e.g., beyond the initial active-check batch) or no checker is registered for the CSP (e.g., `openstack`). | Allowed; can be verified on demand via `GET /spec/{specId}`. |

### Pre-Flight Availability Checkers by CSP

| CSP | Underlying API / Mechanism | Verified Signals |
|-----|---------------------------|------------------|
| **AWS** | `EC2 DescribeInstanceTypeOfferings` (`LocationType=availability-zone`, cached per region) | Per-AZ offering presence (`availableZones` vs region zones in `cloudinfo.yaml`) |
| **Azure** | `Compute ResourceSkus` (`locationInfo` + `restrictions`) & `Usage` API | Per-zone SKU restrictions (`NotAvailableForSubscription`, `NotAvailableForRegion`) and regional vCPU family quota |
| **GCP** | Scoped per-region `zones.list` + parallel `machineTypes.list` (cached 30m/region) + `regions.get` | Per-zone machineType presence and regional GPU/CPU quota limits |
| **Alibaba** | `ECS DescribeAvailableResource` (`DestinationResource=InstanceType`) | Real-time per-zone stock (`WithStock`, `ClosedWithStock`, `WithoutStock`) and system disk compatibility |
| **Tencent** | `CVM DescribeZoneInstanceConfigInfos` | Real-time per-zone `STATUS` (`SELL` vs `SOLD_OUT`) |
| **IBM** | `VPC GetInstanceProfile` + `ListZones` | Regional profile offering across VPC zones |
| **NCP** | `vserver getServerProductList` + `getZoneList` | Server product availability in region/zone |
| **NHN** | `Nova /flavors/detail` + `/os-availability-zone` | Flavor presence and `OS-FLV-DISABLED:disabled` state |
| **KT** | `CloudStack listServiceOfferings` + `listZones` | Offering presence in zone |

### Two-Tier Latency Optimization (Fast Recommendation + Progressive Verification)

Live CSP availability API calls across dozens of regions can add latency if executed for every candidate at once. CB-Tumblebug uses a two-tier enrichment strategy:

1. **Tier 1 — Immediate Enrichment (`POST /tumblebug/recommendSpec`)**:
   - **Step 1 (0ms Passive Cache Lookup)**: Every matched spec is immediately enriched from the in-memory availability cache if a valid entry exists.
   - **Step 2 (Top-10 Active Check)**: Up to **10** uncached top-ranked candidates are actively verified in parallel (`8` workers, `6s` per-spec timeout). Candidates ranked 11+ that are not yet cached are returned immediately with `availabilityStatus = "Unverified"`, keeping API latency low (~1s).
   - **Re-Ranking**: Confirmed unavailable specs (`SoldOut`, `QuotaExceeded`, `Unsupported`) are demoted below `Available` and `Unverified` candidates so the top recommendations are actionable.
2. **Tier 2 — On-Demand / Progressive Verification (`GET /tumblebug/ns/{nsId}/resources/spec/{specId}`)**:
   - Calling `GET /tumblebug/ns/{nsId}/resources/spec/{specId}` (with optional `?forceRefresh=true`) performs an active availability check for that specific spec and caches the result.
   - UIs such as `cb-mapui` render the recommendation list immediately with the first 10 specs verified, and progressively resolve remaining `Unverified` rows or loaded template NodeGroup chips in the background.

### Provisioning Feedback Loop (Write-Through Cache & Zone Exclusion)

Some CSP pre-flight APIs (notably AWS `DescribeInstanceTypeOfferings`, GCP `machineTypes.list`, and IBM `GetInstanceProfile`) report **catalog offering** per AZ rather than real-time physical server pool stock. Consequently, a spec reported as `Available (6 AZ)` can still encounter a runtime failure such as AWS `InsufficientInstanceCapacity` in a specific AZ (e.g., `us-east-1b`) during `POST /infraDynamic`.

CB-Tumblebug closes this gap through a two-part runtime feedback loop:

```
VM Creation Error (e.g., AWS InsufficientInstanceCapacity in us-east-1b)
   │
   ├─► 1. RecordProvisioningFailure() updates in-memory Availability Cache (30-min TTL):
   │      - Parses error message for failed zone (us-east-1b) and hinted available zones (us-east-1a, 1c, 1d, 1e, 1f)
   │      - Marks us-east-1b as Available=false (SoldOut) while preserving Available=true for remaining 5 AZs
   │      - If failure is region-wide (QuotaExceeded / OperationNotAllowed / ZonalAllocationFailed with no remaining AZ),
   │        marks the entire spec as QuotaExceeded or SoldOut
   │
   └─► 2. Subsequent API calls & Provisioning Retries immediately use updated state:
          - recommendSpec / filterSpecsByRange / GET /spec/{specId} show "Available (5/6 AZ)" + failure reason
          - infraDynamicReview / checkInfraDynamicReq surface a warning (or error if pinned to us-east-1b)
          - Next infraDynamic call automatically selects from the remaining available zones (skipping us-east-1b)
```

---

## Image Search & Spec-Image Compatibility

### What It Does

`POST /tumblebug/ns/{nsId}/resources/searchImage` queries the image catalog with multiple filter conditions. When `matchedSpecId` is provided, CB-Tumblebug automatically inherits the spec's `providerName`, `regionName`, and `osArchitecture`, and enforces **CSP-specific hardware/firmware/hypervisor compatibility rules** *before* deduplicating images by date and version.

### Request

```json
{
  "matchedSpecId":   "aws+ap-northeast-2+p4d.24xlarge",
  "providerName":    "aws",
  "regionName":      "ap-northeast-2",
  "osType":          "ubuntu 22.04",
  "osArchitecture":  "x86_64",
  "isGPUImage":      true,
  "isBasicGpuImage": true,
  "maxResults":      20
}
```

### Filter Fields

| Field | Type | Description |
|-------|------|-------------|
| `matchedSpecId` | string | Enforces CSP-specific compatibility with this spec ID (`providerName`, `regionName`, `osArchitecture`, boot mode, hypervisor, controller, disk size, etc.) |
| `providerName` | string | CSP name (e.g., `aws`, `azure`, `gcp`) |
| `regionName` | string | CSP region (e.g., `us-east-1`) |
| `osType` | string | Space-separated AND condition (e.g., `"ubuntu 22.04"`) |
| `osArchitecture` | string | `x86_64`, `arm64`, etc. |
| `isGPUImage` | bool | GPU-capable image (drivers may or may not be pre-installed) |
| `isBasicGpuImage` | bool | GPU image with pre-installed drivers — recommended for GPU workloads |
| `isKubernetesImage` | bool | Suitable for Kubernetes nodes |
| `isRegisteredByAsset` | bool | Registered via CB-Tumblebug asset files |
| `includeBasicImageOnly` | bool | Return clean OS images only (no application stacks) |
| `includeDeprecatedImage` | bool | Include deprecated/end-of-life images (default: false) |
| `detailSearchKeys` | []string | AND keyword search in image details (e.g., `["tensorflow", "2.17"]`) |
| `maxResults` | int | Maximum number of results to return |

### Image Classification Flags

CB-Tumblebug classifies images into three mutually exclusive categories:

| Flag | Meaning |
|------|---------|
| `isBasicImage = true` | Clean OS install, no pre-installed GPU drivers. Prioritized first for non-GPU workloads. |
| `isBasicGpuImage = true` | Base OS image with pre-installed GPU drivers (CUDA, etc.). Always has `isBasicImage = false`. Prioritized for GPU workloads. |
| Neither | Specialized or application-specific image (marketplace, ML framework, DB appliance, etc.). |

> **Note:** `isBasicGpuImage = true` and `isGPUImage = false` is rejected as contradictory.

### CSP-Specific Spec ↔ Image Compatibility Rules

Even when an image matches a spec's CSP, region, and CPU architecture (`x86_64` / `arm64`), VM provisioning will fail if low-level firmware, hypervisor, network/disk controller, or fixed root disk constraints are violated. Both `searchImage` (when `matchedSpecId` is set) and `CheckSpecImageCompatibility` (during `infraDynamicReview` / `checkInfraDynamicReq`) enforce the following rules:

| CSP | Enforced Compatibility Dimensions | Details |
|-----|-----------------------------------|---------|
| **All CSPs** | CPU Architecture & Fixed Root Disk Size | • `spec.architecture` (`x86_64`, `arm64`) must match `image.osArchitecture`.<br>• For CSPs/flavors with fixed local OS root disks (`ncp`, `kt`, `openstack`, and `nhn` local-boot `u2.*` flavors), `image.osDiskSizeGB` must not exceed `spec.rootDiskSize`. |
| **AWS** | Boot Mode, Virtualization Type, ENA, SR-IOV, Bare Metal | • `image.BootMode` (`uefi`, `legacy-bios`, `uefi-preferred`) must be supported by `spec.SupportedBootModes`.<br>• `image.VirtualizationType` (`hvm` vs `paravirtual`) must match `spec.SupportedVirtualizationTypes`.<br>• Nitro / modern specs requiring ENA reject images without `EnaSupport=true`.<br>• Bare-metal specs (`*.metal`) reject `paravirtual` images. |
| **Azure** | Hyper-V Generation, Disk Controller (`NVMe` vs `SCSI`), Confidential VM | • `image.hyperVGeneration` (`V1`, `V2`) must match `spec.HyperVGenerations` (Gen1-only specs are filtered out during catalog ingestion).<br>• Specs supporting only `NVMe` (`DiskControllerTypes=NVMe`, e.g., v6/v7 families such as `Standard_D*s_v6`, `Standard_E*s_v6`) require images declaring `NVMe` support in `DiskControllerTypes`.<br>• Confidential computing specs (`ConfidentialComputingType=SNP` or `TDX`, e.g., `DCasv5`, `ECasv5`, `DCesv5`) require images with `ConfidentialVM` / `CVM` support. |
| **GCP** | `GuestOsFeatures` (`GVNIC`, `IDPF`, `UEFI_COMPATIBLE`) | • Gen3/Gen4 & GPU machine families (`c3`, `c3d`, `c4`, `c4a`, `c4d`, `n4`, `n4a`, `g2`, `g4`, `a2-ultragpu`, `a3`, `a4`, `m3`, `m4`, `z3`, `x4`, `h3`) require images with `GVNIC` in `GuestOsFeatures`.<br>• Gen4 families (`c4`, `c4a`, `c4d`, `n4`, `n4a`, `a4`, `x4`) additionally require `IDPF` (or compatible modern gVNIC/IDPF OS support).<br>• ARM (`t2a`, `c4a`, `n4a`) and Gen3+ specs require `UEFI_COMPATIBLE`. |
| **Alibaba** | Boot Mode (`UEFI` / `BIOS`) & `NVMeSupport` | • `image.BootMode` (`UEFI`, `BIOS`, `UEFI-Preferred`) must be compatible with the instance family.<br>• 8th-gen (`g8i`, `c8i`, `r8i`, `g8a`, `c8a`, `r8a`, `g8y`) and local-NVMe families (`i2`, `i3`, `i4`, `d2`, `d3`, `gn6e`, `gn7`, `gn8`, `ebm*`) require images with `NVMeSupport=supported`. |
| **NCP** | Hypervisor (`XEN` vs `KVM`) & `CorrespondingImageIds` | • `spec` hypervisor (`XEN` for G1/G2 vs `KVM` for G3/gpu-g3) must match `image` hypervisor.<br>• If `spec.Details` defines `CorrespondingImageIds`, the image's `cspImageName` must be in that allowlist. |
| **NHN** | Flavor & Boot Disk Properties | • Evaluates NHN image `Properties`: `nhncloud_allow_gpu_flavor` (for GPU specs), `nhncloud_allow_cpu_flavor` (for CPU specs), `nhncloud_allow_local_bootdisk_flavor` (for local-disk `u2.*` specs), and `min_cpu:<N>` against `spec.vCPU`. |
| **IBM** | VSI Restriction Flags | • Excludes images restricted to bare-metal-only or dedicated-host-only when provisioning standard VPC Virtual Server Instances (VSI). |

### Get Available Search Options

```
GET /tumblebug/ns/{nsId}/resources/searchImageOptions
```

Returns all distinct values currently available in the image catalog for each filter field — useful for populating dropdowns and avoiding empty result queries.

---

## Spec-Image Pair Review & Pre-Flight Validation

Before provisioning infrastructure with `POST /tumblebug/ns/{nsId}/infraDynamic`, callers can validate the entire request—including per-NodeGroup **spec availability**, **zone stockout history**, **image availability**, and **spec-image compatibility**—using `POST /tumblebug/ns/{nsId}/infraDynamicReview` (or `POST /tumblebug/ns/{nsId}/checkInfraDynamicReq`).

### Recommended Pattern

```
1. Call recommendSpec       → get specId (e.g., aws+us-east-1+p5.4xlarge)
                              and inspect availabilityStatus / availableZones
2. Call searchImage         → filter by matchedSpecId = <specId>
                              + isBasicImage = true (non-GPU)
                              or isBasicGpuImage = true (GPU)
3. Call infraDynamicReview  → pre-flight validation of all NodeGroups:
                              - Verifies spec availability/quota (and recent write-through zone failures)
                              - Verifies image existence (or auto-registers from CSP)
                              - Runs CheckSpecImageCompatibility(specInfo, imageInfo)
                              - Estimates hourly cost
4. Submit infraDynamic      → provision infrastructure
```

### Example: `POST /tumblebug/ns/{nsId}/infraDynamicReview`

When a spec-image pair is incompatible (e.g., an older SCSI-only Ubuntu image paired with an NVMe-only Azure `Standard_D4s_v6` spec) or when one of the zones in the region recently experienced a stockout, `infraDynamicReview` reports actionable diagnostics:

```json
{
  "infraName": "mc-demo",
  "overallStatus": "Error",
  "overallMessage": "1 of 2 node groups have errors preventing creation",
  "viable": false,
  "totalEstimatedCost": "$6.8800/hour",
  "nodeGroupReviews": [
    {
      "nodeGroupName": "g1",
      "canCreate": true,
      "status": "Warning",
      "specValidation": {
        "resourceId": "aws+us-east-1+p5.4xlarge",
        "resourceName": "p5.4xlarge",
        "isAvailable": true,
        "status": "Available",
        "message": "zone us-east-1b unavailable/SoldOut (InsufficientInstanceCapacity); available in [us-east-1a, us-east-1c, us-east-1d, us-east-1e, us-east-1f]"
      },
      "imageValidation": {
        "resourceId": "aws+us-east-1+ami-041bfdc361a4a68ef",
        "isAvailable": true,
        "status": "Available"
      },
      "warnings": [
        "Zone availability notice for 'p5.4xlarge': zone us-east-1b unavailable/SoldOut (InsufficientInstanceCapacity); available in [us-east-1a, us-east-1c, us-east-1d, us-east-1e, us-east-1f]"
      ]
    },
    {
      "nodeGroupName": "w1",
      "canCreate": false,
      "status": "Error",
      "specValidation": {
        "resourceId": "azure+koreacentral+standard_d4s_v6",
        "resourceName": "Standard_D4s_v6",
        "isAvailable": true,
        "status": "Available"
      },
      "imageValidation": {
        "resourceId": "azure+koreacentral+canonical:0001-com-ubuntu-server-focal:20_04-lts-gen2:20.04.202401010",
        "isAvailable": false,
        "status": "Incompatible",
        "message": "Azure spec 'Standard_D4s_v6' (HyperVGenerations=\"V2\", DiskControllerTypes=\"NVMe\", ConfidentialComputingType=\"\") is incompatible with image 'canonical:0001-com-ubuntu-server-focal:20_04-lts-gen2:20.04.202401010'"
      },
      "errors": [
        "Azure spec 'Standard_D4s_v6' (HyperVGenerations=\"V2\", DiskControllerTypes=\"NVMe\", ConfidentialComputingType=\"\") is incompatible with image 'canonical:0001-com-ubuntu-server-focal:20_04-lts-gen2:20.04.202401010'"
      ]
    }
  ]
}
```

### Image Selection Guidelines

| Workload | Recommended Image Flag |
|----------|----------------------|
| General compute | `isBasicImage = true` |
| GPU / ML training | `isBasicGpuImage = true` |
| Kubernetes node | `isKubernetesImage = true` |
| Custom software stack | Use `detailSearchKeys` to find matching marketplace images |

---

## Alternative Node Config

### What It Does

`POST /tumblebug/recommendAlternativeNodeConfig` takes an existing NodeGroup configuration (a source spec and optionally its image) and finds the best-matching configurations in a different target CSP and/or region.

This is useful when:
- Migrating a workload to a different cloud provider
- Finding a Replacement when a spec is `SoldOut` or `QuotaExceeded` in the primary region/CSP
- Finding a cost-optimal alternative for an existing configuration
- Evaluating equivalent resources across multiple CSPs simultaneously

### Request

```json
{
  "sourceSpecId":          "aws+ap-northeast-2+m5.2xlarge",
  "sourceImageId":         "ami-0c2d06d50ce30b442",
  "targetProviderName":    "gcp",
  "targetRegionName":      "asia-northeast3",
  "tolerancePercent":      20,
  "specCandidateLimit":    5,
  "imageAlternativeLimit": 3,
  "osType":                "ubuntu",
  "matchCriteria": {
    "architecture":     "required",
    "vCPU":             "preferred",
    "memoryGiB":        "preferred",
    "acceleratorType":  "required",
    "acceleratorModel": "open",
    "costPerHour":      "open"
  }
}
```

### Match Policies

Each spec field can be assigned one of three policies:

| Policy | Behavior | Score contribution |
|--------|----------|--------------------|
| `required` | Exact match filter — candidates that don't match are excluded | Not scored |
| `preferred` | Range filter using `±tolerancePercent`, and scored by proximity | Yes |
| `open` | No filter applied; diff is reported but not scored | No |

**Default policies** (applied when `matchCriteria` is omitted):

| Field | Default |
|-------|---------|
| `architecture` | `required` |
| `acceleratorType` | `required` |
| `vCPU` | `preferred` |
| `memoryGiB` | `preferred` |
| `acceleratorCount` | `preferred` |
| `acceleratorMemoryGB` | `preferred` |
| `acceleratorModel` | `open` |
| `costPerHour` | `open` |

### Similarity Score

The similarity score (0–100) is computed from `preferred` fields only.

**Non-GPU spec weights:**

| Field | Weight |
|-------|--------|
| vCPU | 40% |
| memoryGiB | 40% |
| Architecture | 20% |

**GPU spec weights:**

| Field | Weight |
|-------|--------|
| vCPU | 15% |
| memoryGiB | 15% |
| AcceleratorCount | 35% |
| AcceleratorMemoryGB | 25% |
| Architecture | 10% |

### Response

```json
{
  "sourceSpec":  { ... },
  "sourceImage": { ... },
  "candidates": [
    {
      "rank":            1,
      "similarityScore": 94.5,
      "spec":            { "id": "gcp+asia-northeast3+n2-standard-8", "availabilityStatus": "Available", ... },
      "specDiff": {
        "vCPUDiff":          0,
        "memoryGiBDiff":     -2.0,
        "costPerHourDiff":   -0.03,
        "architectureMatch": true
      },
      "primaryImage":      { "cspImageName": "ubuntu-2204-jammy-v20250101", ... },
      "alternativeImages": [ ... ]
    }
  ]
}
```

### Image Selection in Alternatives

For each candidate spec, `recommendAlternativeNodeConfig` automatically searches for compatible images using `matchedSpecId` + provider/region filter (which applies all CSP-specific compatibility checks):

| Source spec type | Primary image selection |
|------------------|------------------------|
| GPU | `isBasicGpuImage = true` first, then `isGPUImage = true` |
| Non-GPU | `isBasicImage = true` first |

`alternativeImages` contains up to `imageAlternativeLimit` additional options ranked after the primary.

### Key Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| `tolerancePercent` | 20 | ±% range applied to `preferred` fields |
| `specCandidateLimit` | 5 | Max number of candidate specs returned |
| `imageAlternativeLimit` | 3 | Max alternative images per candidate |
| `osType` | (from source image) | Filter images by OS type |

---

## Workflow Example

### Scenario: Provision or Migrate a GPU Workload with Availability & Compatibility Assurance

```
Step 1 — Find source spec (with real-time top-10 availability check)
  POST /tumblebug/recommendSpec
  Filter: providerName=aws, region=ap-northeast-2, acceleratorType=gpu, memoryGiB>=100
  → Confirm spec: aws+ap-northeast-2+p3.8xlarge
    (4× NVIDIA V100, 244 GiB RAM, availabilityStatus="Available", availableZones=["ap-northeast-2a", "ap-northeast-2c"])

Step 2 — Find compatible GPU image for the selected spec
  POST /tumblebug/ns/default/resources/searchImage
  { "matchedSpecId": "aws+ap-northeast-2+p3.8xlarge", "isBasicGpuImage": true }
  → Automatically filters for x86_64 + HVM + ENA + compatible BootMode
  → Select: Deep Learning Base OSS Nvidia Driver GPU AMI (Ubuntu 22.04)

Step 3 — Pre-flight review of the NodeGroup
  POST /tumblebug/ns/default/infraDynamicReview
  → Verifies p3.8xlarge availability/quota and checks spec-image compatibility

Step 4 — Find alternative in GCP (if SoldOut or migrating)
  POST /tumblebug/recommendAlternativeNodeConfig
  {
    "sourceSpecId":       "aws+ap-northeast-2+p3.8xlarge",
    "sourceImageId":      "ami-0abc12345...",
    "targetProviderName": "gcp",
    "tolerancePercent":   30,
    "matchCriteria": {
      "acceleratorType":  "required",
      "acceleratorCount": "preferred",
      "vCPU":             "preferred",
      "memoryGiB":        "preferred"
    }
  }
  → Candidates ranked by similarity score, enriched with GCP availability and matched GVNIC/CUDA GPU images
```

### Result Interpretation

| `specDiff` field | Positive value | Negative value |
|------------------|----------------|----------------|
| `vCPUDiff` | More vCPUs in target | Fewer vCPUs |
| `memoryGiBDiff` | More RAM in target | Less RAM |
| `costPerHourDiff` | More expensive | Cheaper |
| `accelCountDiff` | More GPUs | Fewer GPUs |
| `accelMemGBDiff` | More GPU memory | Less GPU memory |
| `architectureMatch` | `true` = same arch | `false` = different arch |
