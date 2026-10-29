# Agent Guide: Updating `cloudspec.csv` & `cloudimage.csv` Assets

This document provides complete operational context for AI agents (LLMs) and maintainers when updating static cloud resource metadata in `assets/cloudspec.csv` and `assets/cloudimage.csv`.

---

## 1. Overview of the `assets/` Directory

`cb-tumblebug` fetches VM specs and OS images dynamically from CSPs via `cb-spider` (and CSP price APIs where available). However, certain CSPs either:
1. Do not expose pricing APIs via `cb-spider` (e.g., **NCP**, **NHN Cloud**, **KT Cloud VPC**),
2. Require curated baseline VM images because full marketplace indexing takes tens of minutes (e.g., **Azure**), or
3. Require abstract K8s node image type identifiers that are not returned by standard VM image list APIs (e.g., **AWS EKS**, **GCP GKE**, **Tencent TKE**, **Alibaba ACK**).

To handle these cases deterministically, `cb-tumblebug` merges static CSV assets into the system namespace (`nsId=system`) database during initialization or on-demand patching:

| File | Purpose | Core Logic in Codebase |
| :--- | :--- | :--- |
| `assets/cloudspec.csv` | Static hourly cost (`costPerHour`), currency, `infraType` (`vm`, `k8s`, `vm\|k8s`), and GPU/Accelerator metadata (`acceleratorType`, `acceleratorModel`, `acceleratorCount`, `acceleratorMemoryGB`). | `src/core/resource/spec_fetch.go` (`UpdateSpecsFromAsset`, `readSpecAssetFile`, `mergeSpecWithCSVData`) |
| `assets/cloudimage.csv` | Curated VM baseline images (`AZURE`) and Managed Kubernetes node image type identifiers (`AWS`, `GCP`, `TENCENT`, `ALIBABA`). | `src/core/resource/image_fetch.go` (`UpdateImagesFromAsset`, `registerImagesFromAsset`) & `src/core/resource/image.go` (`usesTypeIdentifierK8sImage`) |
| `assets/cloudspec_ignore.yaml` | Specs/regions excluded during CSP spec fetching. | `src/core/resource/spec_fetch.go` |
| `assets/cloudimage_ignore.yaml` | Images excluded during CSP image fetching. | `src/core/resource/image_fetch.go` |
| `assets/extractionpatterns.yaml` | Regex rules used by `ExtractOSInfo` to normalize `osType` and `osArchitecture` from image names/descriptions. | `src/core/resource/image.go` |
| `assets/azure-publisher-filters.yaml` | Publisher/Offer/SKU filters for full Azure image fetching (`init.py` Option `c`). | `src/core/resource/image_fetch.go` |

---

## 2. Updating `assets/cloudspec.csv`

### 2.1 CSV Schema (23 Columns)
Every row in `assets/cloudspec.csv` **must** have exactly 23 comma-separated fields:

```csv
providerName,regionName,cspSpecName,costPerHour,currency,numvCPU,memGiB,storageGiB,description,osType,maxNumStorage,maxTotalStorageTiB,netBwGbps,acceleratorType,acceleratorModel,acceleratorCount,acceleratorMemoryGB,orderInFilteredResult,evaluationScore01,evaluationScore02,rootDiskType,rootDiskSize,infraType
```

```text
 1. providerName          (e.g., NCP, NHN, kt, AWS, GCP, ALIBABA, TENCENT) — case-insensitive matched via strings.ToLower
 2. regionName            ("all" for provider-wide fallback, or specific region like "kr", "sgn", "jpn", "jp1")
 3. cspSpecName           (Exact CSP spec name registered in cb-tumblebug, e.g., s2-g2, m2.c4m8, 8x16.itl)
 4. costPerHour           (Float hourly price in the row's currency; "-1" or "na" means unknown)
 5. currency              (USD, KRW, JPY, CNY, EUR; automatically converted to USD in DB via convertToUSD)
 6. numvCPU               ("na" to keep CSP value, or integer override)
 7. memGiB                ("na" to keep CSP value, or float override)
 8. storageGiB            ("na" to keep CSP value, or float override)
 9. description           ("na" or descriptive label, e.g., "Standard", "GPU V100", "D1 Compute")
10. osType                ("na")
11. maxNumStorage         ("na")
12. maxTotalStorageTiB    ("na")
13. netBwGbps             ("na")
14. acceleratorType       ("na" or "gpu")
15. acceleratorModel      ("na" or GPU model, e.g., "NVIDIA L4", "TESLA V100", "NVIDIA A100P", "NVIDIA A100")
16. acceleratorCount      ("na" or integer count, e.g., "1", "2", "4")
17. acceleratorMemoryGB   ("na" or total GPU VRAM in GB across all attached GPUs, e.g., "24", "40", "80")
18. orderInFilteredResult ("na")
19. evaluationScore01     ("na")
20. evaluationScore02     ("na")
21. rootDiskType          ("na")
22. rootDiskSize          ("na")
23. infraType             ("vm", "k8s", or "vm|k8s")
```

### 2.2 Merge & Precedence Rules (`readSpecAssetFile` & `mergeSpecWithCSVData`)
1. **Map Key Format**: `readSpecAssetFile` indexes rows by `{lower(providerName)}+{lower(regionName)}+{lower(cspSpecName)}`.
2. **`all` vs. Specific Region Override**:
   - First, `all+{cspSpecName}` is applied to matching specs across **all** regions of that provider.
   - Second, `{regionName}+{cspSpecName}` is applied, overriding the `all` entry for that specific region.
   - **Important**: Always place `all` rows before region-specific override rows (e.g., `NHN,all,...` followed by `NHN,jp1,...`), and never leave duplicate `(providerName, regionName, cspSpecName)` keys.
3. **Non-Destructive Merge**:
   - Fields set to `"na"` or `""` in the CSV preserve the existing value fetched from the CSP driver (`numvCPU`, `memGiB`, `storageGiB`, etc.).
   - `costPerHour` is converted to `USD` using `convertToUSD(cost, currency)` in `src/core/resource/spec_fetch.go`.

### 2.3 How to Check Missing/Unpriced Specs in Live `cb-tumblebug`
Before and after editing `assets/cloudspec.csv`, query the running `cb-tumblebug` server (`http://localhost:8080`, Basic Auth `default:default`):

```bash
# Quick summary of registered specs and images per CSP
curl -s -u default:default "http://localhost:8080/tumblebug/assetsSummary?nsId=system" | jq .

# Inspect all specs for a specific provider (e.g., ncp, nhn, kt)
curl -s -u default:default "http://localhost:8080/tumblebug/ns/system/resources/spec" \
  | jq '[.spec[] | select(.providerName=="ncp" and .costPerHour <= 0)]'
```

### 2.4 Official CSP Price Lookup Methods (Verified)

#### A. Naver Cloud Platform (`NCP`)
- **Regions in `cb-tumblebug`**: `kr`, `sgn`, `jpn`
- **Currency**: `USD` (NCP's official Billing API provides exact `USD` hourly prices via `payCurrencyCode=USD`).
- **How to Query Official Prices**:
  1. Retrieve NCP API keys (`NCP_ACCESS_KEY`, `NCP_SECRET_KEY`) from OpenBao (`secret/csp/ncp`) in the running cluster and call NCP's official Billing Product Price List API (`GET https://billingapi.apigw.ntruss.com/billing/v1/product/getProductPriceList`):
     ```bash
     python3 -c '
     import subprocess, json, time, hmac, hashlib, base64, urllib.request

     # 1. Read NCP credentials from OpenBao in K8s (or .credential)
     cmd = """kubectl exec -n cb-tumblebug openbao-0 -- sh -c '\''VAULT_TOKEN=$(grep -o "\\"root_token\\":\\"[^\\"]*\\"" /openbao/file/init.json | cut -d"\\"" -f4) bao kv get -format=json secret/csp/ncp'\''"""
     bao = json.loads(subprocess.check_output(cmd, shell=True).decode())["data"]["data"]
     ak, sk = bao["NCP_ACCESS_KEY"], bao["NCP_SECRET_KEY"]

     # 2. Query official VSVR USD hourly prices per region (KR, SGN, JPN)
     for reg in ["KR", "SGN", "JPN"]:
         uri = f"/billing/v1/product/getProductPriceList?regionCode={reg}&productItemKindCode=VSVR&payCurrencyCode=USD&responseFormatType=json"
         ts = str(int(time.time() * 1000))
         msg = f"GET {uri}\n{ts}\n{ak}".encode("utf-8")
         sig = base64.b64encode(hmac.new(sk.encode("utf-8"), msg, hashlib.sha256).digest()).decode("utf-8")
         req = urllib.request.Request("https://billingapi.apigw.ntruss.com" + uri, headers={
             "x-ncp-apigw-timestamp": ts,
             "x-ncp-iam-access-key": ak,
             "x-ncp-apigw-signature-v2": sig,
         })
         data = json.loads(urllib.request.urlopen(req).read().decode())
         plist = data["getProductPriceListResponse"]["productPriceList"]
         print(f"Region {reg}: fetched {len(plist)} VSVR products")
     '
     ```
  2. Filter each product's `priceList` entries where `periodUnitCode.code == "USETM"` (hourly billing) and `conditionTypeCode.code == "U"` (usage time).
  3. Match NCP's `productCode` / `productName` / `productDescription` to `cb-tumblebug`'s `cspSpecName`:
     - **G2 specs** (`s2-g2`, `c2-g2`, `m2-g2`, `d2-g2`, `v2-g2`, `g2-g2`, `m2-g2-h`, etc.): Matched by product family (`SVR.VSVR.STAND.*` → `s2`, `HICPU.*` → `c2`, `HIMEM.*` → `m2`, `VDS.*` → `d2`, `GPU.V100.*` → `g2-g2` / `v2-g2`) and CPU/memory count (`C004.M008` → `vCPU 4, Mem 8GB`). Note that `-h` suffix indicates SSD (`NET.SSD.*`) while non-`-h` indicates HDD (`NET.HDD.*`), which have identical hourly compute rates in NCP.
     - **G3 & GPU specs** (`s2i-g3`, `c2i-g3`, `c4i-g3`, `m2i-g3`, `m4i-g3`, `m8i-g3`, `m16i-g3`, `c2a-g3`, `m2a-g3`, `m4a-g3`, `m8a-g3`, `g1i-l4-g3`, `g2i-a100p-g3`, `g2i-t4-g3`, `g2-v100-g2`, etc.): Matched directly by parsing `productName` (e.g., `c4i-g3(vCPU 8EA, Memory 16GB)`) and `productDescription` (`vCPU N개, 메모리가 MGB`, GPU model & count).
  4. **Important GPU Metadata**: Always populate `acceleratorType=gpu`, `acceleratorModel` (`NVIDIA L4`, `NVIDIA A100P`, `TESLA T4`, `TESLA V100`), `acceleratorCount`, and `acceleratorMemoryGB` (total VRAM across all GPUs in the spec: `L4` = 24GB/GPU, `A100P` = 40GB/GPU, `T4` = 16GB/GPU, `V100` = 32GB/GPU) so `SearchSpec` GPU filters work accurately.

#### B. NHN Cloud (`NHN`)
- **Regions in `cb-tumblebug`**: `kr1`, `kr2`, `jp1`
- **Currency**: `KRW` (`cb-tumblebug` converts `KRW` → `USD`).
- **How to Query Official Prices**:
  1. NHN Cloud exposes a public, unauthenticated pricing calculator API:
     - **Step 1 — Get Compute service `id`s (`Instance`, `GPU Instance`)**:
       ```bash
       curl -s "https://www.nhncloud.com/homepage/v2/services/hierarchy" | jq '.. | objects | select(.serviceName? == "Instance" or .serviceName? == "GPU Instance")'
       ```
     - **Step 2 — List Flavor `itemId`s per Region (`KR1`, `KR2`, `JP1`)**:
       ```bash
       # Replace {parent_id} with the service id from Step 1
       curl -s "https://www.nhncloud.com/homepage/v2/calculation-services/{parent_id}?region=KR1&lang=ko" | jq .
       curl -s "https://www.nhncloud.com/homepage/v2/calculation-services/{parent_id}?region=JP1&lang=ko" | jq .
       ```
     - **Step 3 — Calculate Exact Price (`monthlyPrice / 720` or `usageTimeUnit: "HOUR"`)**:
       ```bash
       curl -s -X POST "https://www.nhncloud.com/homepage/v2/calculation-services/calculate" \
         -H "Content-Type: application/json" \
         -d '[{"itemId": <itemId>, "region": "KR1", "count": 1, "usageTime": 1, "usageTimeUnit": "HOUR"}]'
       ```
  2. Note that `KR1` and `KR2` share the same base prices (use `NHN,all,{cspSpecName},{krPrice},KRW,...`), whereas `JP1` has higher regional prices (add `NHN,jp1,{cspSpecName},{jpPrice},KRW,...` overrides after the `all` rows).

#### C. KT Cloud VPC (`kt`)
- **Regions in `cb-tumblebug`**: `kr-1` (KT Cloud VPC / D1 platform)
- **Target Specs Rule**: Only `.itl`-suffixed specs (e.g., `1x1.itl`, `64x256.itl`, `16x90.itl.a100-1`) are used in KT Cloud VPC (`D1`). Legacy specs without `.itl` are ignored.
- **Currency**: `KRW`
- **How to Query Official Prices**:
  1. Query KT Cloud's public Product Detail API (`POST https://api.cloud.kt.com/api/prod/prodDetail`):
     ```bash
     # D1 Server (P000000005)
     curl -s -X POST "https://api.cloud.kt.com/api/prod/prodDetail" \
       -H "Content-Type: application/json" \
       -d '{"prodId":"P000000005"}' | jq '.data.priceTableList[].priceTableHtml'

     # GPU Server (P000000006)
     curl -s -X POST "https://api.cloud.kt.com/api/prod/prodDetail" \
       -H "Content-Type: application/json" \
       -d '{"prodId":"P000000006"}' | jq '.data.priceTableList[].priceTableHtml'
     ```
  2. You can also cross-check the KT Cloud Official Price Calculator frontend bundle (`https://cloud.kt.com/calculator/` → `ServerD-*.js`), which embeds the exact hourly KRW rates for all `vCore`/`RAM` combinations (including high-memory combinations like `4x64.itl` = `394` KRW/hr and `24x64.itl` = `1408` KRW/hr).

---

## 3. Updating `assets/cloudimage.csv` & `assets/extractionpatterns.yaml`

### 3.1 CSV Schema (9 Columns)
Every row in `assets/cloudimage.csv` **must** have exactly 9 comma-separated fields:

```csv
providerName,regionName,cspImageName,osType,description,supportedInstance,infraType,osArchitecture,osDistribution
```

| Column # | Name | Description & Rules |
| :--- | :--- | :--- |
| 1 | `providerName` | `AZURE`, `AWS`, `GCP`, `TENCENT`, `ALIBABA`, etc. |
| 2 | `regionName` | `"all"` (applies to all regions of that provider) or specific region name. |
| 3 | `cspImageName` | Exact CSP image identifier (`Offer:Publisher:SKU:Version` URN for Azure, or K8s node image type ID for `infraType=k8s`). |
| 4 | `osType` | Normalized OS family + major version (e.g., `Ubuntu 22.04`, `Ubuntu 24.04`, `Debian 13`, `Red Hat Enterprise Linux 9`, `Windows 2022`). |
| 5 | `description` | Human-readable description. **Quote the field if it contains commas.** |
| 6 | `supportedInstance` | Usually empty (`""`) or `na`. |
| 7 | `infraType` | `vm`, `k8s`, or `vm\|k8s`. |
| 8 | `osArchitecture` | `x86_64`, `arm64`, or `na`. **CRITICAL**: Never leave `osArchitecture` empty (`""`)! `SearchImage` filters by `LOWER(os_architecture) = 'x86_64' OR LOWER(os_architecture) = 'na'`. If `osArchitecture` is empty, the image will be invisible to `SearchImage` whenever `matchedSpecId` or `osArchitecture` is queried. |
| 9 | `osDistribution` | Detailed distribution string or URN. |

### 3.2 Two Roles of `assets/cloudimage.csv`

#### Role 1: Curated Azure VM Baseline Images (`AZURE,all,...,vm,...`)
- Full Azure Marketplace image fetching takes 40+ minutes (`init.py` Option `c`) and is skipped in standard initialization (`Option b`).
- Therefore, `assets/cloudimage.csv` registers a curated set of common Linux and Windows Server images for Azure (`systemLabel: "from-assets"`).
- **Maintenance Caution for Azure Image URNs**:
  - Azure periodically retires older date-stamped build versions (`Publisher:Offer:Sku:Version`, e.g., `22.04.202603110`).
  - When an Azure URN in `cloudimage.csv` is retired, `cb-spider` `GetImage` fails during live enrichment, or the image is marked `Deprecated`.
  - **Best Practice**:
    1. Always populate columns 8 (`osArchitecture`: `x86_64` or `arm64`) and 9 (`osDistribution`) explicitly in `assets/cloudimage.csv` for every `AZURE` row.
    2. Verify that every Azure URN in `assets/cloudimage.csv` is currently `Available` in the DB or Azure Marketplace:
       ```bash
       curl -s -u default:default -X POST "http://localhost:8080/tumblebug/ns/system/resources/searchImage" \
         -H "Content-Type: application/json" \
         -d '{"providerName":"azure","regionName":"koreacentral","includeDeprecatedImage":false,"detailSearchKeys":["Canonical"]}' | jq .
       ```

#### Role 2: Managed Kubernetes Node Image Type Identifiers (`infraType=k8s`)
- In `src/core/resource/image.go` (`usesTypeIdentifierK8sImage`), 4 CSPs use abstract node image type identifiers for Managed Kubernetes (`k8s`) node groups instead of regular VM image IDs:
  - **`AWS` (EKS)**: `AL2023_x86_64_STANDARD`, `AL2023_ARM_64_STANDARD`, `AL2023_x86_64_NVIDIA`, `AL2023_ARM_64_NVIDIA`, `AL2023_x86_64_NEURON`, `BOTTLEROCKET_x86_64`, `BOTTLEROCKET_ARM_64`, `BOTTLEROCKET_x86_64_NVIDIA`, `BOTTLEROCKET_ARM_64_NVIDIA`, `WINDOWS_FULL_2019_x86_64`, `WINDOWS_CORE_2019_x86_64`, `WINDOWS_FULL_2022_x86_64`, `WINDOWS_CORE_2022_x86_64`, `WINDOWS_FULL_2025_x86_64`, `WINDOWS_CORE_2025_x86_64`
  - **`GCP` (GKE)**: `COS_CONTAINERD`, `UBUNTU_CONTAINERD`, `WINDOWS_LTSC_CONTAINERD`
  - **`TENCENT` (TKE)**: `tlinux4_x86_64_public`, `tlinux3.1x86_64`, `tlinux2.4(tkernel4)x86_64_HCC`, `tlinux2.4(tkernel4)x86_64`, `tlinux2.4x86_64`, `centos7.8.0_x64`, `centos8.0x86_64`, `ubuntu20.04x86_64`, `ubuntu22.04x86_64`, `redhat7.9x86_64`
  - **`ALIBABA` (ACK)**: `AliyunLinux3ContainerOptimized`, `Ubuntu`
- Other CSPs (such as `AZURE` AKS, `NCP` NKS, `NHN` NKS, `KT`, `OPENSTACK`) either manage K8s node OS selection internally via the driver or validate standard VM images (`usesTypeIdentifierK8sImage == false`).

### 3.3 Maintaining `assets/extractionpatterns.yaml`
- `ExtractOSInfo` in `src/core/common/utility_image.go` iterates over `extractionPatterns.osType.<os>.versions` **in array order** and returns the first version whose regex `(?:^|[^0-9])<version>(?:$|[^0-9])` matches the image metadata string.
- **Rule**: Always list `versions` in **descending order** (newest to oldest, e.g., `["10", "9", "8"]` for `rocky`/`alma`/`rhel`, `["13", "12", "11", "10", "9"]` for `debian`, `["2025", "2022", "2019", "2016", "11", "10"]` for `windows`). Listing older versions first causes minor version numbers or date digits (e.g., `.8` in `rocky-9.8` or `9` in `20260819`) to false-match an older major version.

---

## 4. Validation & Applying Changes to a Running System

### Step 1: Validate CSV Syntax, Line Endings (`LF`), & Integrity Locally
Always use Unix line endings (`\n`, `lineterminator='\n'` in Python `csv.writer`) so `^M` (`\r`) characters are never introduced, and run a quick Python check before applying changes:

```bash
python3 -c '
import csv

for path in ["assets/cloudspec.csv", "assets/cloudimage.csv"]:
    raw = open(path, "rb").read()
    if b"\r\n" in raw:
        open(path, "wb").write(raw.replace(b"\r\n", b"\n"))
        print(f"Converted CRLF -> LF in {path}")

with open("assets/cloudspec.csv") as f:
    rows = [r for r in csv.reader(f) if r and not r[0].startswith("#")]
assert all(len(r) == 23 for r in rows), "Invalid column count in cloudspec.csv"
keys = [(r[0].lower(), r[1].lower(), r[2].lower()) for r in rows[1:]]
assert len(keys) == len(set(keys)), "Duplicate (provider, region, spec) in cloudspec.csv"
print(f"cloudspec.csv OK: {len(rows)-1} data rows")

with open("assets/cloudimage.csv") as f:
    irows = [r for r in csv.reader(f) if r and not r[0].startswith("#")]
assert all(len(r) == 9 for r in irows), "Invalid column count in cloudimage.csv"
assert all(r[7].strip() != "" for r in irows[1:]), "Empty osArchitecture in cloudimage.csv"
print(f"cloudimage.csv OK: {len(irows)-1} data rows")
'
```

### Step 2: Sync Assets to the Running Kubernetes Pod (if running on K8s / Kind)
- If **only `assets/*` files** were modified (no Go code changes):
  ```bash
  make k-assets
  ```
  *(Copies `./assets/*` into the running `cb-tumblebug` pod at `/app/assets/` without rebuilding the container image.)*
- If **Go source code** was also modified:
  ```bash
  make k-build-tb
  ```
  *(Builds the binary and container image, loads it into Kind, and rolls out the `cb-tumblebug` deployment.)*

### Step 3: Patch the Live Database Immediately (Without Full Re-Init)
Once `/app/assets/cloudspec.csv` and `/app/assets/cloudimage.csv` are inside the running container (or local process), trigger the live asset patch endpoints:

```bash
# 1. Patch existing specs in DB with updated prices / GPU info / infraType from assets/cloudspec.csv
curl -s -u default:default -X POST "http://localhost:8080/tumblebug/updateSpecsFromAsset" | jq '{count: (.spec | length)}'

# 2. Register/update images in DB from assets/cloudimage.csv
curl -s -u default:default -X POST "http://localhost:8080/tumblebug/updateImagesFromAsset" | jq '{count: (.image | length)}'

# 3. Verify updated counts
curl -s -u default:default "http://localhost:8080/tumblebug/assetsSummary?nsId=system" | jq .
```

Alternatively, you can run:
```bash
make k-init
```
and select **Option `a+`** (`Restore from backup + patch CSV assets`), which restores the SQLite/Postgres backup and runs both `POST /tumblebug/updateSpecsFromAsset` and `POST /tumblebug/updateImagesFromAsset`.

### Step 4: Persist Updated DB to Backup (Optional)
If you want future `Option a` (fast backup restore) runs to include the newly patched DB state by default:
```bash
make backup-assets
```
*(Exports the current `cb-tumblebug` and `cb-spider` metadata DB into `assets/backup/`.)*
