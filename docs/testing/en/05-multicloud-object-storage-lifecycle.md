# Multi-Cloud Object Storage Lifecycle & Presigned URL CRUD Integration Test Report

## 0. Summary

**All stages completed normally across all 10 supported CSPs (Alibaba, AWS, Azure, GCP, IBM, KT, NCP, NHN, OpenStack, Tencent) using MapUI default settings: Bucket Create (`PUT /ns/default/resources/objectStorage`) → Existence, Detail, Location & Reconcile (`HEAD`, `GET`, `GET /location`, `PUT /reconcile`) → CORS Configuration (`PUT / GET /cors`) → Object Versioning Configuration (`PUT / GET /versioning`) → Full 5-Step Presigned URL Object CRUD Verification → Bucket Delete (`DELETE /ns/default/resources/objectStorage/{osId}`).**

| Metric | Value |
|---|---|
| Bucket Create (request → 10/10 `Available` with unique CSP bucket name) | 0.44 – 7.45 s (0.44 – 3.62 s excl. IBM) |
| Metadata, Location & Reconcile (`HEAD` + `GET` + `GET /location` + `PUT /reconcile`) | 0.10 – 6.51 s |
| CORS Configure & Read (`PUT /cors` + `GET /cors`, 7 supported CSPs) | 1.45 – 11.14 s (excl. IBM; 37.38 s on IBM global propagation) |
| Object Versioning Enable & Read (`PUT /versioning` + `GET /versioning`, 6 supported CSPs) | 0.45 – 6.50 s |
| Full 5-Step Presigned URL Object CRUD (Upload URL → `PUT` v1/v2 → List & `HEAD` → Download & Verify → Delete Object & Versions) | 0.48 – 28.62 s per CSP (0.48 – 6.27 s excl. IBM) |
| Bucket Delete (`DELETE /ns/default/resources/objectStorage/{osId}`) | 1.65 – 12.67 s |
| Total Parallel Wall-Clock Time (10 CSPs) | **107.7 s (1m 48s)** |
| Residual Cloud / Tumblebug Resources (`objectStorage`) | **0** |

---

## 1. Environment

| Item | Detail |
|---|---|
| Time | 2026-10-26 UTC (Full E2E Multi-Cloud Object Storage 10-CSP Benchmark) |
| CB-Tumblebug | `aea9af49` + immediate S3 GET cache invalidation in `ListDataObjects`, deployed on a kind cluster |
| CB-Spider | `ee952524` + lowercase S3 endpoint hostname normalization (`strings.ToLower(regionID)`) for NCP/NHN (RFC 3986 Host signature compatibility) |
| CB-MapUI | `🪣 Object Storage Management` (Single/Multi-Cloud parallel bucket provisioning, `⚡ Background Operations` live timer, 1-click 5-Step Presigned URL CRUD verification) |
| Target CSPs (10) | Alibaba, AWS, Azure, GCP, IBM, KT, NCP, NHN, OpenStack, Tencent |

| CSP | Connection / Region (`LocationConstraint`) | Protocol / Backend | CORS API | Versioning API | Presigned URL | Required Upload Headers (`requiredHeaders`) |
|---|---|---|---|---|---|---|
| **Alibaba** | `alibaba-ap-northeast-2` (`ap-northeast-2`) | Alibaba Cloud OSS (S3-compatible) | ✅ | ✅ (`Enabled` / `Suspended`) | ✅ (S3 SigV4) | None |
| **AWS** | `aws-ap-northeast-2` (`ap-northeast-2`) | Amazon S3 | ✅ | ✅ (`Enabled` / `Suspended`) | ✅ (S3 SigV4) | None |
| **Azure** | `azure-koreacentral` (`koreacentral`) | Azure Blob Storage (Native SDK + SAS) | ⚪ (Account-level) | ⚪ (Account-level) | ✅ (Blob SAS URL) | `x-ms-blob-type: BlockBlob` |
| **GCP** | `gcp-asia-northeast3` (`asia-northeast3`) | Google Cloud Storage (Native SDK + S3 Interop) | ✅ (GCS SDK) | ✅ (GCS SDK Generation) | ✅ (S3 HMAC SigV4) | None |
| **IBM** | `ibm-us-south` (`us-south`) | IBM Cloud Object Storage (S3-compatible) | ✅ | ✅ (`Enabled` / `Suspended`) | ✅ (S3 SigV4) | None |
| **KT** | `kt-kr1` (`KR1`) | KT Cloud Object Storage (S3-compatible) | ✅ | ✅ (`Enabled` / `Suspended`) | ✅ (S3 SigV4) | None |
| **NCP** | `ncp-kr` (`KR`) | Naver Cloud Object Storage (S3-compatible) | ⚪ (Console-managed) | ⚪ (Unsupported) | ✅ (S3 SigV4) | None |
| **NHN** | `nhn-kr1` (`KR1`) | NHN Cloud Object Storage (S3-compatible) | ⚪ (Console/Container header) | ⚪ (Unsupported) | ✅ (S3 SigV4) | None |
| **OpenStack** | `openstack-regionone` (`RegionOne`) | Ceph RGW / Swift S3 Interface | ✅ | ⚪ (Unsupported) | ✅ (S3 SigV4) | None |
| **Tencent** | `tencent-ap-seoul` (`ap-seoul`) | Tencent Cloud COS (`<bucket>-<AppId>`) | ✅ | ✅ (`Enabled` / `Suspended`) | ✅ (S3 SigV4) | None |

---

## 2. Procedure

| Step | Stage | Request API | Pass Criteria |
|---|---|---|---|
| 1 | Bucket Create | `PUT /ns/default/resources/objectStorage` (`bucketName`, `connectionName`, `description`) | All 10 CSPs return `200 OK` with `status: "Available"` and globally unique `cspResourceName` (`uid`) |
| 2 | Existence, Detail, Location & Reconcile | `HEAD .../objectStorage/{osId}`, `GET .../objectStorage/{osId}`, `GET .../location`, `PUT .../reconcile` | `HEAD 200`, `GET 200`, valid `locationConstraint`, and `Synced=True` after `reconcile` |
| 3a | CORS Configure & Verify | `PUT .../objectStorage/{osId}/cors` (`allowedOrigin: ["*"]`, `allowedMethod: ["GET","PUT","POST","DELETE","HEAD"]`) → `GET .../cors` | All 7 CORS-supporting CSPs apply (`200/204`) and return matching CORS rules (`200 OK`) |
| 3b | Versioning Enable & Verify | `PUT .../objectStorage/{osId}/versioning` (`{"status": "Enabled"}`) → `GET .../versioning` | All 6 Versioning-supporting CSPs transition to `status: "Enabled"` |
| 4 | Full 5-Step Presigned URL Object CRUD | `[4-1]` Generate Upload Presigned URL (`POST .../object/{key}/presignedUrl?operation=upload`) → `[4-2]` Direct binary `PUT` to CSP (upload v1 and overwrite with v2) → `[4-3]` List objects (`GET .../object`) & check metadata (`HEAD .../object/{key}`) → `[4-4]` Generate Download Presigned URL (`POST ...?operation=download`) & verify payload (`updated-verified`) → `[4-5]` Delete object and all versions/delete markers (`DELETE .../object/{key}`, `DELETE .../versions/{key}?versionId=...`) | Direct client-to-CSP upload/download (bypassing Tumblebug data plane), immediate presence in object list, `ETag` header verification, payload match, and clean deletion across all 10 CSPs |
| 5 | Bucket Delete & Cleanup Verification | `DELETE /ns/default/resources/objectStorage/{osId}` (with `?option=force` fallback) → `GET /ns/default/resources/objectStorage` | Bucket deleted on CSP, metadata removed from Tumblebug, and `0` remaining buckets in `ns/default` |

---

## 3. Result Matrix

| CSP | Step 1: Bucket Create | Step 2: Meta, Location & Reconcile | Step 3a: CORS Set & Get | Step 3b: Versioning Enable | Step 4a: Presigned URL Object CRUD (CLI / SDK Client) | Step 4b: Presigned URL Object CRUD (MapUI Browser Direct XHR) | Step 5: Bucket Delete | Overall |
|---|---|---|---|---|---|---|---|---|
| **Alibaba** (`ap-northeast-2`) | ✅ | ✅ (`ap-northeast-2`) | ✅ | ✅ (`Enabled`) | ✅ (v1/v2 PUT · List · HEAD · GET · Ver Del) | ✅ (Browser Direct PUT · List · HEAD · GET · DEL) | ✅ | ✅ |
| **AWS** (`ap-northeast-2`) | ✅ | ✅ (`ap-northeast-2`) | ✅ | ✅ (`Enabled`) | ✅ (v1/v2 PUT · List · HEAD · GET · Ver Del) | ✅ (Browser Direct PUT · List · HEAD · GET · DEL) | ✅ | ✅ |
| **Azure** (`koreacentral`) | ✅ | ✅ (`koreacentral`) | ➖ (Account-level) | ➖ (Account-level) | ✅ (SAS PUT `BlockBlob` · List · HEAD · GET · Del) | ⚪ (Control-Plane `UpURL·DlURL·List` ✅ / Direct PUT requires Console CORS) | ✅ | ✅ |
| **GCP** (`asia-northeast3`) | ✅ | ✅ (`asia-northeast3`) | ✅ | ✅ (`Enabled`) | ✅ (v1/v2 PUT · List · HEAD · GET · Ver Del) | ✅ (Browser Direct PUT · List · HEAD · GET · DEL) | ✅ | ✅ |
| **IBM** (`us-south`) | ✅ | ✅ (`us-south`) | ✅ | ✅ (`Enabled`) | ✅ (v1/v2 PUT · List · HEAD · GET · Ver Del) | ✅ (Browser Direct PUT · List · HEAD · GET · DEL) | ✅ | ✅ |
| **KT** (`kr1`) | ✅ | ✅ (`KR1`) | ✅ | ✅ (`Enabled`) | ✅ (v1/v2 PUT · List · HEAD · GET · Ver Del) | ✅ (Browser Direct PUT · List · HEAD · GET · DEL) | ✅ | ✅ |
| **NCP** (`kr`) | ✅ | ✅ (`KR`) | ➖ (Unsupported) | ➖ (Unsupported) | ✅ (v1/v2 PUT · List · HEAD · GET · Del) | ⚪ (Control-Plane `UpURL·DlURL·List` ✅ / Direct PUT requires Console CORS) | ✅ | ✅ |
| **NHN** (`kr1`) | ✅ | ✅ (`KR1`) | ➖ (Unsupported) | ➖ (Unsupported) | ✅ (v1/v2 PUT · List · HEAD · GET · Del) | ⚪ (Control-Plane `UpURL·DlURL·List` ✅ / Direct PUT requires Console CORS) | ✅ | ✅ |
| **OpenStack** (`regionone`) | ✅ | ✅ (`RegionOne`) | ✅ | ➖ (Unsupported) | ✅ (v1/v2 PUT · List · HEAD · GET · Del) | ✅ (Direct PUT passes on `http://`; Control-Plane verified on `https://` due to Mixed Content) | ✅ | ✅ |
| **Tencent** (`ap-seoul`) | ✅ | ✅ (`ap-seoul`) | ✅ | ✅ (`Enabled`) | ✅ (v1/v2 PUT · List · HEAD · GET · Ver Del) | ✅ (Browser Direct PUT · List · HEAD · GET · DEL) | ✅ | ✅ |
| **Total (10 CSPs)** | **10/10** | **10/10** | **7/7 (All Supported)** | **6/6 (All Supported)** | **10/10 (All Pass)** | **7/7 (CORS API CSPs) + 3/3 (Control-Plane Verified)** | **10/10 (`0` left)** | **Pass** |

> **CLI/SDK (`Step 4a`) vs. MapUI Web Browser (`Step 4b`)**:
> * **CLI / Backend SDK / `curl` (`Step 4a`)**: Non-browser HTTP clients are not subject to browser CORS Preflight (`OPTIONS`) or Mixed Content policies; all **10/10 CSPs pass full direct file upload (`PUT`) and download (`GET`)** using Tumblebug-issued Presigned URLs.
> * **Web Browser JavaScript XHR (`Step 4b`)**: Browsers enforce CORS preflight checks (`Access-Control-Allow-Origin`) before sending cross-origin `PUT` requests. All 7 CSPs supporting the S3 CORS API pass browser direct upload automatically, whereas the 3 CSPs without an S3 CORS API (`Azure`, `NCP`, `NHN`) require a one-time CORS rule in their respective cloud consoles for browser XHR `PUT` (while `<a download>` browser file downloads and Control-Plane `UpURL·DlURL·List` checks work out of the box).

---

## 4. Timing by Stage

| Stage | Per-CSP Range (10 CSPs) | Notes |
|---|---|---|
| Step 1: Empty Bucket Resource Create (`PUT /objectStorage`) | **0.44 – 7.45 s** | NHN (`0.44 s`), Azure (`0.74 s`), NCP (`0.87 s`) fastest · IBM (`7.45 s`) |
| Step 2: Meta, Location & Reconcile | **0.10 – 6.51 s** | Combined duration of `HEAD` + `GET` + `GET /location` + `PUT /reconcile` |
| Step 3a: CORS Configure & Read (`PUT / GET /cors`) | **1.45 – 37.38 s** | GCP (`1.45 s`) fastest · S3-standard CSPs (`8.97 – 11.14 s`) · IBM global propagation (`37.38 – 68.92 s`) |
| Step 3b: Versioning Enable (`PUT / GET /versioning`) | **0.45 – 6.50 s** | Alibaba (`0.45 s`), KT (`0.98 s`), AWS (`1.60 s`), GCP (`1.77 s`), Tencent (`2.63 s`), IBM (`6.50 s`) |
| Step 4: Full 5-Step Presigned URL File (Object) CRUD | **0.48 – 28.62 s** | Combined file upload (`PUT`) + List & `HEAD` + file download (`GET`) & verify + file delete |
| Step 5: Empty Bucket Resource Delete (`DELETE /objectStorage/{osId}`) | **1.65 – 12.67 s** | Includes post-delete CSP absence verification (`PollResourceDeletedViaSpider`) |
| **Total Parallel Wall-Clock Time (10 CSPs)** | **107.7 s (1m 48s)** | Full 10-CSP parallel lifecycle from bucket creation to final deletion |

### Time per CSP (seconds)

| CSP | Bucket Create (s) | Meta, Location & Reconcile (s) | CORS Set & Get (s) | Versioning Enable (s) | Presigned Upload v1+v2 (s) | List + `HEAD` Verify (s) | Presigned Download & Verify (s) | Object & Versions Delete (s) | Total 5-Step Object CRUD (s) | Bucket Delete (s) |
|---|---|---|---|---|---|---|---|---|---|---|
| **NHN** (`nhn-kr1`) | **0.44** | **0.10** | *(N/A)* | *(N/A)* | 0.24 | 0.13 | **0.05** | **0.06** | **0.48** | **1.65** |
| **Azure** (`azure-koreacentral`) | 0.74 | 0.46 | *(Account-level)* | *(Account-level)* | **0.02** | 0.35 | 0.46 | 0.12 | 0.96 | 3.47 |
| **NCP** (`ncp-kr`) | 0.87 | 0.12 | *(N/A)* | *(N/A)* | 0.59 | 0.76 | 0.47 | 0.10 | 1.92 | 3.73 |
| **OpenStack** (`openstack-regionone`) | 1.34 | 0.45 | 8.97 | *(N/A)* | 0.63 | 1.08 | 0.40 | 0.38 | 2.48 | 4.32 |
| **Alibaba** (`alibaba-ap-northeast-2`) | 2.24 | 0.84 | 9.53 | **0.45** | 0.16 | 0.41 | 0.07 | 0.24 (3 vers) | 0.89 | 4.45 |
| **KT** (`kt-kr1`) | 2.34 | 1.76 | 10.04 | 0.98 | 0.78 | 1.46 | 0.97 | 3.02 (3 vers) | 6.24 | 3.59 |
| **Tencent** (`tencent-ap-seoul`) | 2.91 | 1.24 | 11.14 | 2.63 | 0.33 | 2.62 | 0.23 | 3.09 (3 vers) | 6.27 | 9.00 |
| **AWS** (`aws-ap-northeast-2`) | 3.32 | 0.77 | 11.07 | 1.60 | 0.73 | 1.06 | 0.35 | 2.38 (3 vers) | 4.51 | 2.30 |
| **GCP** (`gcp-asia-northeast3`) | 3.62 | 1.24 | **1.45** | 1.77 | 1.12 | 2.05 | 0.11 | 2.01 (w/ vers) | 5.28 | 3.67 |
| **IBM** (`ibm-us-south`)* | 7.45 | 6.51 | 37.38* | 6.50 | 5.33 | 5.58 | 3.56 | 14.15 (3 vers) | 28.62 | 12.67 |

> \* **IBM Cloud Object Storage Note**: IBM Cloud (`ibm-us-south`) exhibits higher latency (`37.4 – 68.9 s` for CORS rule propagation and `14.15 s` for multi-version deletion) due to trans-Pacific RTT to the `us-south` endpoint (`s3.us-south.cloud-object-storage.appdomain.cloud`) and global eventual consistency verification.

---

## 5. Information Accuracy & E2E Data Plane Verification

The following checks were executed across all 10 CSPs and passed with zero discrepancies:

| Check | Result |
|---|---|
| Required Bucket Metadata (`id`, `name`, `uid`, `cspResourceName`, `cspResourceId`, `connectionName`, `status`, `conditions`) | 0 missing or placeholder fields once `Available` (Tencent records `<uid>-<AppId>` in `cspResourceId`) |
| Bucket Location Constraint (`GET /ns/default/resources/objectStorage/{osId}/location`) | Accurately returns regional constraint (`ap-northeast-2`, `koreacentral`, `asia-northeast3`, `KR`, `KR1`, `RegionOne`, `ap-seoul`, `us-south`) across all 10 CSPs |
| Live State Reconcile & Prune (`PUT .../reconcile`, `POST .../reconcile/prune`) | 100% agreement between live CSP bucket state and Tumblebug `Synced=True` condition |
| Direct Client-to-CSP Presigned Upload & Download (`POST .../presignedUrl`) | All 10 CSPs completed direct binary `PUT` and `GET` via Presigned URLs with verified payload integrity (`updated-verified`) |
| Immediate Post-Upload Object Listing (`GET .../object`) | `ListDataObjects` invalidates the internal Spider S3 GET cache before querying, guaranteeing immediate visibility of newly uploaded objects |
| Final Teardown (`DELETE /ns/default/resources/objectStorage/{osId}`) | 0 remaining `objectStorage` resources in `ns/default` |

---

## 6. CSP Notes & Engineering Improvements (Backend & MapUI UX)

| Aspect / Constraint | Affected CSPs | Root Cause & CB-Spider / CB-Tumblebug / CB-MapUI Handling |
|---|---|---|
| **Lowercase Hostname Normalization for Presigned URLs (`SignatureDoesNotMatch` Root-Cause Fix)** | **NCP, NHN** | NCP (`KR`) and NHN (`KR1`) store uppercase region IDs, previously producing uppercase S3 endpoints (`KR.object.ncloudstorage.com`, `KR1-api-object-storage.nhncloudservice.com`). Standard HTTP clients (`curl`, `requests`) and web browsers (`fetch`/`axios`) normalize URL hostnames to lowercase per RFC 3986 before sending the `Host` header, causing `403 SignatureDoesNotMatch`. Fixed in CB-Spider `GetS3ConnectionInfo` ([`S3Manager.go`](file:///home/shson/go/src/github.com/cloud-barista/cb-spider/api-runtime/common-runtime/S3Manager.go#L241-L254)) via `strings.ToLower(regionID)`. |
| **GCP Storage SDK CORS Preflight (`ResponseHeaders`) Compatibility** | **GCP** | Google Cloud Storage's `storage.CORS` struct uses `ResponseHeaders` for both `Access-Control-Expose-Headers` and `Access-Control-Allow-Headers` on `OPTIONS` preflight, and CB-Spider's `setGCPBucketCORS` maps `exposeHeader` to `ResponseHeaders`. Added `'Content-Type'` to `exposeHeader` in MapUI's `enableDefaultCors()` so browser direct `PUT` preflight requests pass immediately on GCS. |
| **Browser XHR CORS Constraints vs. Control-Plane Verification** | **Azure, NCP, NHN** | Azure (Storage Account-level CORS), NHN (OpenStack Swift container metadata headers/Console), and NCP (Console-managed CORS) do not expose the standard S3 `PutBucketCors` API (`cors: false`). While CLI/SDK clients and `<a download>` browser links work directly without CORS, browser JavaScript (`axios.put`) direct uploads are blocked by the browser unless CORS is enabled in the CSP console. Without introducing an SSRF-prone server proxy, MapUI verifies the 3 Control-Plane APIs (`UpURL✓ · DlURL✓ · List✓`) and reports `PASS (Control-Plane)`. |
| **Automatic `requiredHeaders` Merging for Azure Blob SAS Uploads** | **Azure** | Azure Blob Storage requires `x-ms-blob-type: BlockBlob` on direct SAS `PUT` uploads. Updated MapUI's `uploadFileToBucket()` and `runE2eObjectCrudBenchmark()` to automatically merge `urlRes.data.requiredHeaders` into the `axios.put` request headers. |
| **Immediate Object List Cache Invalidation After External Presigned Uploads** | **All CSPs (esp. fast CSPs)** | Because Presigned URL uploads go directly from the client/browser to the CSP, calling `GET .../object` within 2 seconds (`ShortDuration`) of an earlier bucket read could return a cached empty list. Fixed in Tumblebug `ListDataObjects()` by invalidating the Spider S3 GET cache prior to fetching. |
| **Tencent COS AppId Bucket Suffix (`-<AppId>`)** | **Tencent** | Tencent COS appends the account `AppId` to the physical bucket name (`tbi0bd0kcsop2lsms2gk-1383606637`). CB-Spider resolves the `AppId` dynamically via Tencent CAM API, and MapUI displays both the Tumblebug ID and actual CSP bucket name with a 1-click Copy button. |
| **Comprehensive MapUI `🪣 Object Storage Management` UX Upgrade** | **All 10 CSPs** | Aligned with `🗄️ RDBMS Management` UX: ① **CSP Filter Chips & Capability/Timing Info Box** (`Est. Create`, `Est. Delete`, `Est. 5-Step CRUD`), ② **Single / Multi-Cloud Parallel Bucket Provisioning Drawer** (`{baseName}-{csp}` naming, Auto-CORS, Auto-Versioning), ③ **`⚡ Background Operations` Live Progress Panel** (elapsed timer & step cards), ④ **1-Click 5-Step Presigned URL CRUD Test (`Test CRUD` / `Batch E2E Test`)**, ⑤ **Versioning Toggle & Object Versions Modal**, and ⑥ **Reconcile All / Prune Orphans / Empty Bucket (`?option=empty`) / Force Delete (`?option=force`)**. |

