# Spec and Image Recommendation

Guide for selecting optimal compute specs and images, reviewing spec-image pairs, and finding alternative configurations across CSPs.

## 📑 Table of Contents

1. [Overview](#overview)
2. [Spec Recommendation](#spec-recommendation)
3. [Image Search](#image-search)
4. [Spec-Image Pair Review](#spec-image-pair-review)
5. [Alternative Node Config](#alternative-node-config)
6. [Workflow Example](#workflow-example)

---

## Overview

CB-Tumblebug provides a set of APIs to help users select compute resources for multi-cloud infrastructure provisioning. The four APIs work together in a natural progression:

```
RecommendSpec  →  SearchImage  →  (review pair)  →  RecommendAlternativeNodeConfig
   Find spec       Find image      Confirm pair       Port config to another CSP
```

| API | Method | Path |
|-----|--------|------|
| Recommend Spec | POST | `/tumblebug/recommendSpec` |
| Get Recommendation Options | GET | `/tumblebug/recommendSpecOptions` |
| Search Image | POST | `/tumblebug/ns/{nsId}/resources/searchImage` |
| Get Search Options | GET | `/tumblebug/ns/{nsId}/resources/searchImageOptions` |
| Recommend Alternative Config | POST | `/tumblebug/recommendAlternativeNodeConfig` |

---

## Spec Recommendation

### What It Does

`POST /tumblebug/recommendSpec` filters and ranks compute specs from the global spec catalog based on user-defined conditions and priorities. It returns a sorted list of specs that match the criteria.

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

## Image Search

### What It Does

`POST /tumblebug/ns/{nsId}/resources/searchImage` queries the image catalog with multiple filter conditions. It supports filtering by provider, region, OS, architecture, GPU capability, and image classification flags.

### Request

```json
{
  "matchedSpecId":  "aws+ap-northeast-2+p4d.24xlarge",
  "providerName":   "aws",
  "regionName":     "ap-northeast-2",
  "osType":         "ubuntu 22.04",
  "osArchitecture": "x86_64",
  "isGPUImage":     true,
  "isBasicGpuImage": true,
  "maxResults":     20
}
```

### Filter Fields

| Field | Type | Description |
|-------|------|-------------|
| `matchedSpecId` | string | Only images compatible with this spec ID |
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

CB-Tumblebug classifies images into three categories that are mutually exclusive:

| Flag | Meaning |
|------|---------|
| `isBasicImage = true` | Clean OS install, no pre-installed GPU drivers. For non-GPU workloads. |
| `isBasicGpuImage = true` | GPU-ready image with drivers pre-installed. Always has `isBasicImage = false`. |
| Neither | Specialized or application-specific image (marketplace, ML framework, etc.) |

> **Note:** `isBasicGpuImage = true` and `isGPUImage = false` is rejected as incompatible.

### Get Available Search Options

```
GET /tumblebug/ns/{nsId}/resources/searchImageOptions
```

Returns all distinct values currently available in the image catalog for each filter field — useful for populating dropdowns and avoiding empty result queries.

---

## Spec-Image Pair Review

There is no single API for "spec + image pair review" — it is a design-time pattern that combines the outputs of `recommendSpec` and `searchImage`.

### Recommended Pattern

```
1. Call recommendSpec → get specId (e.g., aws+ap-northeast-2+m5.2xlarge)
2. Call searchImage   → filter by matchedSpecId = <specId>
                       + isBasicImage = true (non-GPU)
                       or isBasicGpuImage = true (GPU)
3. Review the pair:
   - Spec:  vCPU, memoryGiB, architecture, accelerator, costPerHour
   - Image: osType, osDistribution, isBasicImage/isBasicGpuImage, isKubernetesImage
4. Submit as NodeGroup in infraDynamic / k8sClusterDynamic
```

### NodeGroup Structure

The selected spec and image are combined into a `NodeGroup` definition for provisioning:

```json
{
  "name":          "ng-01",
  "specId":        "aws+ap-northeast-2+m5.2xlarge",
  "imageId":       "ami-0c2d06d50ce30b442",
  "rootDiskType":  "default",
  "rootDiskSize":  0,
  "minNodeSize":   1,
  "maxNodeSize":   3,
  "desiredNodeSize": 2
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
      "spec":            { "id": "gcp+asia-northeast3+n2-standard-8", ... },
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

For each candidate spec, `recommendAlternativeNodeConfig` automatically searches for compatible images using the same `matchedSpecId` + provider/region filter:

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

### Scenario: Migrate an AWS GPU workload to GCP

```
Step 1 — Find source spec
  POST /recommendSpec
  Filter: providerName=aws, region=ap-northeast-2, acceleratorType=gpu, memoryGiB>=100

Step 2 — Confirm spec: aws+ap-northeast-2+p3.8xlarge (4× NVIDIA V100, 244 GiB RAM)

Step 3 — Find matching GPU image for the source spec
  POST /ns/default/resources/searchImage
  { "matchedSpecId": "aws+ap-northeast-2+p3.8xlarge", "isBasicGpuImage": true }
  → Select: Deep Learning Base OSS Nvidia Driver GPU AMI (Ubuntu 22.04)

Step 4 — Find alternative in GCP
  POST /recommendAlternativeNodeConfig
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
  → Candidates ranked by similarity score with matched GPU images
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
