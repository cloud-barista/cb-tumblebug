# Ultra-Large-Scale (10,000+ Nodes) Multi-Cloud VM Full Lifecycle & Performance Test Report

## 0. Summary

**All 8 full-lifecycle stages completed 100% across 10 CSPs, 146 regions (NodeGroups), and 10,456 target VMs using the `infra-global-extream` template: Provisioning (`POST /infra/template/...`) → Multi-View Status & Info Query Benchmarks (`GET /infra/...`) → Failed Node Refinement (`?action=refine`) → Bulk Suspend (`?action=suspend`) → Bulk Resume & Public IP Reassignment (`?action=resume`) → Bulk Terminate (`?action=terminate`) → Infra Metadata Deletion (`DELETE /infra/{id}?option=terminate`) → Shared Resource Cleanup (`DELETE /sharedResources`).**

*(Conducted to verify the complete large-scale architecture under Issue [`#2461`](https://github.com/cloud-barista/cb-tumblebug/issues/2461), including batch status sweeping, independent batch Public IP caching, and anti-regression guards across `Suspend`, `Resume`, and `Terminate`.)*

| Metric | Value | Notes |
|---|---|---|
| **Target Template Scale** | **10 CSPs · 146 Regions · 10,456 VMs** | `infra-global-extream` template (`default` namespace) |
| **Shared Resource Creation (`CreateSharedResources`)** | **146 / 146 Regions 100% Created** (`+198.1 s` mark) | **All 146 regions (including all 41 GCP regions)** created `vNet(146)` · `SG(146)` · `sshKey(146)` = **438 resources** (`0` failures) |
| **Provisioned & Running VMs** | **10,414 / 10,456 Running (99.60%)** | **9 CSPs (GCP, Azure, Tencent, IBM, Alibaba, NCP, NHN, OpenStack, KT) achieved 100% (`240/240`)**; 26 of 27 AWS regions achieved 100% (`42` `Failed` in `eu-central-2a` due to AZ capacity) |
| **Total Provisioning Time (`POST /infra/template/...`)** | **915.20 s (15m 15s)** | Synchronous response including spec/image validation (`~39 s`), 438 shared resources (`~159 s`), and 10,456-VM provisioning |
| **Per-Node `Running` Time Distribution (10,414 VMs)** | **Min 202.1 s · P50 506.1 s · P90 759.1 s · P95 795.1 s · P99 838.1 s · Max 871.1 s** | Mean `519.2 s` (Request start timestamp → node `createdTime`) |
| **10,456-Node Status Query (`?option=status`)** | **Median 131.04 ms** (Min `97.57 ms`, Max `163.83 ms`) | Served directly from in-memory `StatusStore` (`4.77 MB` payload) |
| **10,456-Node Detail Query (`?detail=refined` / `?detail=raw`)** | **Median 122.63 ms / 158.05 ms** (Min `80.17 ms`) | NodeGroup blueprint hydration & shared read cache (`22.61 MB` payload) |
| **10,456-Node Access Info Query (`?option=accessinfo`)** | **Median 788.65 ms** (Min `709.40 ms`, Max `1,005.68 ms`) | **>152× faster** than previous `120 s` timeout (`503`) by reading from `NodeInfo`/`StatusStore` (`6.82 MB` payload) |
| **Failed Node Refinement (`?action=refine`)** | **1.83 s** (`HTTP 200`) | Removed the 42 AZ-capacity `Failed` records → **10,414 surviving VMs 100% `Running`** |
| **Bulk Suspend (`?action=suspend`)** | **API Return 78.53 s · 99.95% Convergence 184.3 s (3m 04s)** | **10,414 / 10,414 VMs converged to `Suspended` (`Undefined=0`)**; `0 / 10,414` residual `publicIP` values (`100%` cleared) |
| **Bulk Resume & Public IP Refresh (`?action=resume`)** | **API Return 65.82 s · 100% `Running` Convergence 183.94 s (3m 04s)**<br>**100% Public IP Populated: 183.94 s (`+0.0 s` extra wait!)** | **10,414 / 10,414 VMs reached `Running` and populated `publicIP` simultaneously** (`public_ip_cache.go` for `10,174` AWS VMs + `refreshBatchNodePublicIP` attempt #1 for `236` non-AWS batch VMs) |
| **Bulk Terminate (`?action=terminate`)** | **100% `Terminated` Convergence 270.83 s (4m 31s)**<br>*(Synchronous API Return: 320.92 s / 5m 21s)* | **10,414 / 10,414 VMs (100.00%) reached `Terminated`** (`0` stragglers, `0` `Undefined` regressions) |
| **Delete Infra (`DELETE /infra/{id}?option=terminate`)** | **117.18 s (1m 57s)** | 146-region CSP cross-audit (`AuditInfra`) + 50-worker parallel etcd deletion of 10,414 nodes and 146 NodeGroups (`remaining_infras: []`) |
| **Shared Resource Cleanup (`DELETE /sharedResources`)** | **149.85 s (2m 30s, Single Pass #1)** | **Pass #1 single call deleted 438 / 438 resources (`146 vNets`, `146 SGs`, `146 sshKeys`, `failedCount: 0`)** |
| **System Stability & Resource Footprint** | **Steady TB `531–991 MiB` (Terminate peak `3,705 MiB`) · Spider `63–385 MiB` · etcd `380–1,427 MiB`** | **0 Pod Restarts, 0 OOMs, 0 Panics/Deadlocks** across the entire 44-minute run |

---

## 1. Environment & Target Configuration

| Item | Detail |
|---|---|
| Test Window | 2026-10-30 19:42:58 – 20:26:43 UTC (**2,625.1 s / 43m 45s** across all 8 lifecycle stages) |
| Cluster Environment | Kubernetes (`kind`, `cb-tumblebug` namespace) |
| Key Active Mechanisms | • `#2461` High-Scale Architecture (NodeGroup blueprint extraction, in-memory `StatusStore`, `StatusAgent` background poller)<br>• Direct CSP SDK batch status (`BatchVMStatusCached` lock-free singleflight across 9 CSPs) & AWS bulk control (`Start`/`Stop`/`TerminateInstances`)<br>• Independent AWS batch Public IP cache (`public_ip_cache.go`) + dedicated-semaphore exponential backoff IP refresh (`refreshBatchNodePublicIP`)<br>• 2-strike not-found streak (`recordBatchTerminateNotFound`) & `Terminated` immutability guard<br>• GCP global VPC quota wait semaphore decoupling (`quotaWaitSem`) & per-region shared VNet unification (`#2841`) |
| Input Template | `infra-global-extream` (146 NodeGroups, 10,456 nodes, `policyOnPartialFailure: "continue"`) |
| Target CSPs (10) | AWS (27 regions), GCP (41 regions), Azure (37 regions), Tencent (13 regions), IBM (11 regions), Alibaba (9 regions), NCP (3 regions), NHN (3 regions), OpenStack (1 region), KT (1 region) |

---

## 2. Full Lifecycle Test Procedure (8 Stages)

| Step | Stage | Request API | Pass Criteria |
|---|---|---|---|
| **Phase 1** | **Ultra-Large-Scale Provisioning** | `POST /ns/default/infra/template/infra-global-extream` (`{"name": "infra-global-extream", "policyOnPartialFailure": "continue"}`) | 438 shared resources across 146 regions and 10,456 VMs provisioned synchronously (`200 OK`) |
| **Phase 2** | **Multi-View Query Benchmarks** | 45 s stabilization wait followed by 9 query endpoints × 5 iterations (`45` calls) | All 45 calls return `200 OK`; latency distribution (Min/Avg/Median/Max), payload size, and cross-view consistency verified |
| **Phase 2b** | **Failed Node Refinement** | `GET /ns/default/control/infra/infra-global-extream?action=refine` | Removes the 42 AZ-capacity `Failed` node records, leaving all 10,414 surviving VMs in `Running` |
| **Phase 3** | **Bulk Suspend** | `GET /ns/default/control/infra/infra-global-extream?action=suspend` | All 10,414 VMs converge to `Suspended` (`Undefined=0`) and clear `publicIP` (`0/10,414` non-empty) |
| **Phase 4** | **Bulk Resume & Public IP Refresh** | `GET /ns/default/control/infra/infra-global-extream?action=resume` | All 10,414 VMs converge to `Running` (`Undefined=0`) and populate newly assigned `publicIP` (`10,414/10,414`) |
| **Phase 5** | **Bulk Terminate** | `GET /ns/default/control/infra/infra-global-extream?action=terminate` | All 10,414 VMs converge to `Terminated` (`Undefined=0`) |
| **Phase 6** | **Delete Infra** | `DELETE /ns/default/infra/infra-global-extream?option=terminate` | 146-region CSP cross-audit (`AuditInfra`) and parallel etcd metadata removal (`remaining_infras: []`) |
| **Phase 7** | **Shared Resource Cleanup** | `DELETE /ns/default/sharedResources` | All 438 shared resources (`146 vNets`, `146 SGs`, `146 sshKeys`) deleted with `0` residuals |

---

## 3. Comprehensive Per-CSP Performance & Multi-Perspective Analysis

### 3.1 End-to-End Per-CSP Lifecycle Matrix (Provisioning · Suspend · Resume · Terminate)

| CSP | Req. Regions | Active Regions (Success) | Representative Specs (`specId`) | Req. VMs | Reached `Running` (Success) | Provision P50 / Max (s) | `Suspend` 100% Conv. (s) | `Resume` 100% Conv. (s) | `Resume` Public IP Population Path | `Terminate` 100% Conv. (s) | Control & Status Path |
|---|---:|---:|---|---:|---:|---:|---:|---:|---|---:|---|
| **AWS** | 27 | **27 (100%)** | `t3.nano` | 10,216 | **10,174 (99.59%)**¹ | `513.1` / `871.1` | **184.3** | **183.94** | **Direct Batch IP Cache** (`0` Spider calls) | **240.63** | Direct SDK Bulk Control + Batch Status |
| **GCP** | 41 | **41 (100%)** | `t2d-standard-1`, `n1-standard-1`, `e2-highcpu-2` | 82 | **82 (100.0%)** | `258.1` / `318.1` | `< 135.0` | **120.80** | `refreshBatchNodePublicIP` (82/82 on attempt #1) | **270.83** | Spider Control + Direct SDK Batch Status |
| **Azure** | 37 | **37 (100%)** | `Standard_B1ls` | 74 | **74 (100.0%)** | `291.1` / `439.1` | `< 135.0` | **32.68** | `refreshBatchNodePublicIP` (74/74 on attempt #1) | **231.23** | Spider Control + Direct SDK Batch Status |
| **Tencent** | 13 | **13 (100%)** | `bf1.medium2`, `s5.medium2`, `sa2.medium2`, `sa9.medium2` | 26 | **26 (100.0%)** | `277.6` / `306.1` | `< 135.0` | **32.68** | `refreshBatchNodePublicIP` (26/26 on attempt #1) | **167.58** | Spider Control + Direct SDK Batch Status |
| **IBM** | 11 | **11 (100%)** | `nxf-2x1` | 22 | **22 (100.0%)** | `237.6` / `257.1` | `< 135.0` | **36.02** | `refreshBatchNodePublicIP` (22/22 on attempt #1) | **139.87** | Spider Control + Direct SDK Batch Status |
| **Alibaba** | 9 | **9 (100%)** | `ecs.e-c4m1.large`, `ecs.t6-c4m1.large`, `ecs.mn4.small`, `ecs.c9i.large` | 18 | **18 (100.0%)** | `206.6` / `211.1` | `< 135.0` (13 VMs)<br>`547.3` (5 VMs)² | **32.68** | `refreshBatchNodePublicIP` (18/18 on attempt #1) | **200.97** | Spider Control + Direct SDK Batch Status |
| **NCP** | 3 | **3 (100%)** | `c2-g3a`, `s2-g3a`, `c2-g3` | 6 | **6 (100.0%)** | `308.6` / `311.1` | `< 135.0` | **63.23** | `refreshBatchNodePublicIP` (6/6 on attempt #1) | **173.80** | Spider Control + Direct SDK Batch Status |
| **NHN** | 3 | **3 (100%)** | `t2.c1m1` | 6 | **6 (100.0%)** | `306.1` / `319.1` | `< 135.0` | **36.02** | `refreshBatchNodePublicIP` (6/6 on attempt #1) | **130.34** | Spider Control + Direct SDK Batch Status |
| **OpenStack** | 1 | **1 (100%)** | `m1.medium` | 4 | **4 (100.0%)** | `330.6` / `339.1` | `< 135.0` | **99.98** | Synchronous post-control `FetchNodeStatus` | **216.70** | Spider Control + Individual Status Poller |
| **KT** | 1 | **1 (100%)** | `1x1.itl` | 2 | **2 (100.0%)** | `431.6` / `433.1` | `< 135.0` | **32.68** | `refreshBatchNodePublicIP` (2/2 on attempt #1) | **102.62** | Spider Control + Direct SDK Batch Status |
| **Total (10 CSPs)** | **146** | **146 (100%)** | — | **10,456** | **10,414 (99.60%)** | **`506.1` / `871.1`** | **184.3 (99.95%)**<br>`547.3` (100%) | **183.94 (100%)** | **10,414 / 10,414 (100%) `+0.0 s` lag** | **270.83 (100%)** | — |

> **Notes**:
> 1. **AWS `eu-central-2` (Zurich) 42 `Failed` VMs**: AZ `eu-central-2a` exhausted its physical `t3.nano` host capacity (`InsufficientInstanceCapacity`) after launching 458/500 VMs (the other **26 AWS regions achieved 100.0% — 9,716/9,716 VMs `Running`**). `Phase 2b` (`action=refine`) cleaned up those 42 failed metadata entries in `1.83 s` prior to `Suspend` / `Resume` / `Terminate`.
> 2. **Alibaba 5-VM Guest OS ACPI Shutdown Tail**: 13 of 18 Alibaba VMs transitioned to `"Stopped"` (`Suspended`) within `78 s`, while 5 VMs across 4 regions (`ap-northeast-1` 2, `cn-huhehaote` 1, `cn-wulanchabu` 1, `us-west-1` 1) remained in Alibaba `"Stopping"` (`Suspending`) awaiting guest OS shutdown until `547.3 s`, when all 10,414 VMs reached `Suspended`.

---

### 3.2 Per-CSP Provisioning Time Distribution (`createdTime`)

Elapsed seconds from `POST /infra/template/infra-global-extream` start (including the `~198.1 s` 438-resource shared network/security creation phase) until each node reached `Running`:

| CSP | `Running` VMs | `Min` (s) | Median `P50` (s) | `P95` (s) | `Max` (s) | Mean `Avg` (s) | Net VM Provisioning Window (`Min – Max`, excl. shared resources) |
|---|---:|---:|---:|---:|---:|---:|---|
| **Alibaba** | 18 (9 regions) | **202.1** | **206.6** | **211.1** | **211.1** | **206.6** | `4.0 s – 13.0 s` (Fastest VM startup) |
| **IBM** | 22 (11 regions) | 210.1 | 237.6 | 257.1 | 257.1 | 238.7 | `12.0 s – 59.0 s` |
| **GCP** | 82 (41 regions) | 218.1 | 258.1 | 317.1 | 318.1 | 267.5 | `20.0 s – 120.0 s` (100% across all 41 regions) |
| **Tencent** | 26 (13 regions) | 242.1 | 277.6 | 304.1 | 306.1 | 276.5 | `44.0 s – 108.0 s` |
| **Azure** | 74 (37 regions) | 216.1 | 291.1 | 341.1 | 439.1 | 290.1 | `18.0 s – 241.0 s` (100% across all 37 regions) |
| **NHN** | 6 (3 regions) | 291.1 | 306.1 | 319.1 | 319.1 | 304.6 | `93.0 s – 121.0 s` (`jp1`, `kr1`, `kr2` 100%) |
| **NCP** | 6 (3 regions) | 303.1 | 308.6 | 311.1 | 311.1 | 308.3 | `105.0 s – 113.0 s` (`kr`, `jpn`, `sgn` 100%) |
| **OpenStack** | 4 (1 region) | 313.1 | 330.6 | 339.1 | 339.1 | 328.4 | `115.0 s – 141.0 s` |
| **KT** | 2 (1 region) | 430.1 | 431.6 | 433.1 | 433.1 | 431.6 | `232.0 s – 235.0 s` |
| **AWS** | **10,174 (27 regions)** | 212.1 | 513.1 | 796.1 | 871.1 | 525.0 | `14.0 s – 673.0 s` (Up to 500 VMs per region) |
| **Overall** | **10,414 (146 regions)** | **202.1** | **506.1** | **795.1** | **871.1** | **519.2** | `P90: 759.1 s`, `P99: 838.1 s` |

---

### 3.3 Deep Dive by CSP: Performance & Architectural Perspectives

#### (1) AWS (27 Regions · 10,216 Requested / 10,174 Running) — Bulk SDK Control & Zero-Overhead Batch Public IP Cache
* **Provisioning**: Distributed across eighteen 500-VM regions (`9,000` VMs), one 192-VM region (`eu-north-1`), and eight 128-VM regions (`1,024` VMs). The 128–192 VM regions completed 100% within `407.1 – 587.1 s`, and the eighteen 500-VM regions completed within `764.1 – 871.1 s` without throttling errors.
* **Bulk `Suspend` & `Resume`**:
  * Invokes AWS SDK v2 `StopInstances` and `StartInstances` directly in batches of 50 per region, bypassing CB-Spider.
  * During `Suspend`, all `10,174` AWS VMs across 27 regions converged to `Suspended` within **`184.3 s` (3m 04s)**.
  * During `Resume`, seven 128–192 VM regions reached 100% `Running` in **`61.0 – 62.0 s`**, twelve 500-VM regions converged in **`78.0 – 91.0 s`**, and all 27 regions (`10,174` VMs) finished converging at **`177.0 – 183.94 s` (3m 04s)**.
* **Instant Public IP Population via Independent Batch Cache (`public_ip_cache.go`)**:
  * Stopping and restarting an AWS EC2 instance releases its old public IPv4 address and allocates a new one. Previously, refreshing 10,174 public IPs required 10,174 individual `GET /spider/vm/{id}` calls taking 8–12 minutes.
  * With [`aws/vm_status.go`](file:///home/shson/go/src/github.com/cloud-barista/cb-tumblebug/src/core/csp/aws/vm_status.go#L56-L98) storing `*instance.PublicIpAddress` into the lock-free `sync.Map` in [`public_ip_cache.go`](file:///home/shson/go/src/github.com/cloud-barista/cb-tumblebug/src/core/csp/public_ip_cache.go) and `runBatchSweep` persisting `e.PublicIP` directly alongside `Running` status, **AWS required `0` CB-Spider calls to refresh 10,174 public IPs**, achieving **`+0.0 s` lag between `Running` convergence (`183.94 s`) and 100% Public IP availability (`183.94 s`)**.
* **Bulk `Terminate`**: All `10,174` AWS VMs across 27 regions converged to `Terminated` in **`240.63 s` (4m 01s)** with zero stragglers.

##### AWS 27-Region Breakdown (Provisioning & Resume Convergence)

| Region Tier | Regions (NodeGroup Count) | VMs / Region | `Running` / Requested | Provision `Min – Max` (s) | `Resume` 100% `Running` + IP Populated (s) |
|---|---|---:|---:|---:|---:|
| **Medium (128–192 VMs)** | `ap-northeast-3`, `ap-south-2`, `eu-south-1`, `eu-south-2`, `eu-north-1`(192), `il-central-1`, `af-south-1` (7 regions) | 128 – 192 | **960 / 960 (100%)** | `213.1 – 503.1` | **`61.0 – 62.0 s`** |
| **Medium (128 VMs, APAC)** | `ap-southeast-3`, `ap-southeast-4` (2 regions) | 128 | **256 / 256 (100%)** | `227.1 – 587.1` | `162.0 – 177.0 s` |
| **Large (500 VMs, Wave 1)** | `ca-central-1`, `ap-southeast-2`, `ap-northeast-1`, `ap-south-1`, `us-west-2`, `us-east-2`, `us-east-1`, `eu-central-1`, `eu-west-1`, `sa-east-1`, `eu-west-2`, `eu-west-3` (12 regions) | 500 | **6,000 / 6,000 (100%)** | `214.1 – 871.1` | **`78.0 – 91.0 s`** |
| **Large (500 VMs, Wave 2)** | `ap-northeast-2`, `ap-southeast-1`, `eu-central-2`(458/500), `us-west-1`, `ap-east-1`, `ca-west-1` (6 regions) | 500 | **2,958 / 3,000 (98.6%)** | `212.1 – 838.1` | `104.0 – 177.0 s` |

#### (2) GCP (41 Regions · 82 Requested / 82 Running) — 100% 41-Region Shared VNet Creation & 2-Strike Not-Found Verification
* **41/41 Regions & 82/82 VMs 100% Provisioned**: Decoupling the global VPC quota wait semaphore (`quotaWaitSem`) and serializing VPC peering/subnet retries enabled **all 41 GCP regions to create shared VNets/SGs/SSHKeys and launch all 82 VMs (100%)** within `218.1 – 318.1 s` (`P50 258.1 s`).
* **`Suspend`, `Resume`, & Public IP Refresh**: Converged to `Suspended` in `< 135 s` and resumed all 82 VMs to `Running` in **`120.80 s`**, with all 82 VMs refreshing their new public IP on **attempt #1** via `refreshBatchNodePublicIP`.
* **Production Verification of 2-Strike Not-Found Streak (`recordBatchTerminateNotFound`)**: Because GCP removes deleted instances from `instances.list` immediately (`found=0`), `tb_final.log` confirmed multiple GCP regions (`asia-east1`, `europe-west2`, `europe-west4`, `europe-west8`, `us-east4`, `us-west3`, `me-west1`, `me-central2`) hitting `queried=2, found=0`, holding `Terminating` on strike 1 and transitioning cleanly to `Terminated` on strike 2 (`270.83 s` total, `0` `Undefined` regressions).

#### (3) Azure (37 Regions · 74 Requested / 74 Running) — Flawless 37-Region Provisioning & Sub-33s Resume
* **Provisioning**: All 37 global Azure regions created shared resources and launched all **74 `Standard_B1ls` VMs (100%)** in `216.1 – 439.1 s` (`P50 291.1 s`).
* **Ultra-Fast `Resume` (`32.68 s`)**: All 74 Azure VMs across 37 regions reached `Running` in **`32.68 s`** and refreshed all 74 public IPs on `refreshBatchNodePublicIP` attempt #1.
* **`Terminate` (`231.23 s`)**: Verified 2-strike not-found streak handling across `southindia`, `eastasia`, `northcentralus`, `australiacentral`, `mexicocentral`, `eastus`, and `chilecentral` with `0` `Undefined` regressions.

#### (4) Tencent (13 Regions · 26 VMs), IBM (11 Regions · 22 VMs), Alibaba (9 Regions · 18 VMs)
* **100% Provisioning Success**: Alibaba (`202.1 – 211.1 s`, fastest P50 at `206.6 s`), IBM (`210.1 – 257.1 s`, P50 `237.6 s`), and Tencent (`242.1 – 306.1 s`, P50 `277.6 s`) all achieved 100% VM provisioning and 100% single-pass shared resource cleanup.
* **Sub-37s `Resume`**: Alibaba (`32.68 s`), Tencent (`32.68 s`), and IBM (`36.02 s`) all converged 100% to `Running` and refreshed all public IPs on attempt #1.

#### (5) Regional & Private Cloud CSPs — NCP (3 Regions · 6 VMs), NHN (3 Regions · 6 VMs), KT (1 Region · 2 VMs), OpenStack (1 Region · 4 VMs)
* **100% Full-Lifecycle Completion**:
  * **NHN (`6/6`)**: Provisioned in `306.1 s` (P50), resumed in `36.02 s`, terminated in `130.34 s`.
  * **NCP (`6/6`)**: Provisioned in `308.6 s` (P50), resumed in `63.23 s` (all 6 public IPs acquired on attempt #1), terminated in `173.80 s`.
  * **KT (`2/2`)**: Provisioned in `431.6 s`, resumed in `32.68 s`, terminated in `102.62 s` (fastest termination across all 10 CSPs).
  * **OpenStack (`4/4`)**: Exercised the individual Spider status poller path (`StatusAgent` → `/spider/vmstatus`), completing provisioning (`330.6 s`), `Suspend`, `Resume` (`99.98 s`), and `Terminate` (`216.70 s`) seamlessly alongside the 9 batch-SDK CSPs.

---

## 4. Detailed Stage & Convergence Timelines

### 4.1 End-to-End Lifecycle Stage Breakdown

| Stage | Sub-Phase | Duration (s) | Key Activity & Verification |
|---|---|---:|---|
| **Phase 1: Provisioning** | **Total (`POST /infra/template/...`)** | **915.20 s (15m 15s)** | `19:42:58.8Z – 19:57:43.4Z` (`HTTP 200`, `23.71 MB` response) |
| ↳ `1-a` | Spec & Image Pre-Validation (146 NodeGroups) | `~39.3 s` | `+0.0s – +39.3s` |
| ↳ `1-b` | Parallel Shared Resource Creation (146 Regions) | `~158.8 s` | `+39.3s – +198.1s` (`146 vNets` + `146 SGs` + `146 sshKeys` = **438/438 created, 100%**) |
| ↳ `1-c` | Multi-Cloud VM Creation & Convergence | `~717.1 s` | `+198.1s – +915.2s` (**10,414 `Running`**, 42 `Failed`, compact NodeGroup blueprints persisted) |
| **Phase 2: Query Benchmarks** | 45 s Stabilization + 9 Endpoints × 5 Runs | **65.80 s** | Stabilization (`45.0 s`) + **45/45 queries `200 OK` in `20.8 s`** |
| **Phase 2b: Refine** | `GET /control/infra/{id}?action=refine` | **1.83 s** | Removed 42 `Failed` nodes → **10,414 surviving nodes 100% `Running`** |
| **Phase 3: Suspend** | `GET /control/infra/{id}?action=suspend` | API **78.53 s**<br>Conv. **184.3 s (99.95%)** | **10,414 / 10,414 `Suspended`**, `Undefined=0`, `0` non-empty `publicIP` |
| **Phase 4: Resume** | `GET /control/infra/{id}?action=resume` | API **65.82 s**<br>Conv. **183.94 s (100%)** | **10,414 / 10,414 `Running` & 10,414 / 10,414 `publicIP` populated at `183.94 s` (`+0.0 s` lag)** |
| **Phase 5: Terminate** | `GET /control/infra/{id}?action=terminate` | Conv. **270.83 s (100%)**<br>API **320.92 s** | **10,414 / 10,414 `Terminated`** (`Undefined=0`, `0` stragglers) |
| **Phase 6: Delete Infra** | `DELETE /ns/default/infra/{id}?option=terminate` | **117.18 s (1m 57s)** | 146-region `AuditInfra` + 50-worker parallel etcd deletion |
| **Phase 7: Cleanup Shared** | `DELETE /ns/default/sharedResources` (Pass #1) | **149.85 s (2m 30s)** | **Single Pass #1 deleted 438 / 438 resources (`vNet=0, SG=0, sshKey=0`)** |

---

### 4.2 Shared Resource Creation & Single-Pass Cleanup Timeline (146 Regions · 438 Resources)

| Phase | Elapsed Time (`+s`) | `vNet` | `securityGroup` | `sshKey` | Activity |
|---|---:|---:|---:|---:|---|
| **Provisioning** | `+0.0s – +39.3s` | 0 | 0 | 0 | Pre-validation of 146 NodeGroups |
| | `+58.5s` | 116 | 6 | 13 | Parallel VNet creation across 10 CSPs (`79.5%` ready) |
| | `+77.6s` | 130 | 35 | 53 | VNet `89.0%`, SG `24.0%`, SSHKey `36.3%` ready |
| | `+111.9s` | 142 | 78 | 117 | VNet `97.3%`, SG `53.4%`, SSHKey `80.1%` ready |
| | `+127.1s` | **146** | 104 | 132 | **100% of VNets (146/146, incl. all 41 GCP regions) created** |
| | `+180.4s` | 146 | 145 | **146** | **100% of SSHKeys (146/146) created** |
| | **`+198.1s`** | **146** | **146** | **146** | **100% of Shared Resources (438/438 across 146 regions) created** |
| **Cleanup (`Pass #1`)** | `+0.0s` | 146 | 146 | 146 | `DELETE /ns/default/sharedResources` started (`SG → vNet → sshKey`) |
| | `+17.1s` | 146 | 41 | 146 | 105 SecurityGroups deleted in 17 seconds |
| | `+49.0s` | 87 | **0** | 146 | **All 146 SecurityGroups deleted**; VNet deletion underway |
| | `+66.0s` | 44 | 0 | 146 | 102 regional VNets (VPCs + Subnets) deleted |
| | `+113.0s` | 9 | 0 | 146 | 137 regional VNets deleted |
| | `+129.9s` | **0** | 0 | 31 | **All 146 VNets deleted**; 115 SSHKeys deleted |
| | **`+149.85s` (Pass #1 Done)** | **0** | **0** | **0** | **Single Pass #1 deleted 438 / 438 shared resources (`failedCount: 0`)** |

---

### 4.3 `Resume` Status vs. Public IP Population Co-Progression (10,414 VMs)

| Elapsed Time from `Resume` Start | `Running` VMs | `Resuming` VMs | VMs with `publicIP != ""` | Gap (`Running - ip_populated`) | Completion % |
|---:|---:|---:|---:|---:|---:|
| `+0.00s` (Prior `Suspended`) | 0 | 0 (`Suspended: 10,414`) | **0 / 10,414** | `0` | `0.0%` |
| `+32.68s` | 265 | 8,645 (`Suspended: 1,504`) | 118 / 10,414 | `147` (Non-AWS `refreshBatchNodePublicIP` in flight) | `2.5%` |
| `+60.09s` | 551 | 9,863 | 550 / 10,414 | **`1`** | `5.3%` |
| `+65.96s` (Right after API return) | 4,419 | 5,995 | **4,418 / 10,414** | **`1`** | `42.4%` |
| `+71.11s` | 5,268 | 5,146 | **5,268 / 10,414** | **`0`** | `50.6%` |
| `+81.44s` | 6,762 | 3,652 | **6,762 / 10,414** | **`0`** | `64.9%` |
| `+96.86s` | 9,558 | 856 | **9,558 / 10,414** | **`0`** | `91.8%` |
| `+127.63s` | 10,190 | 224 | **10,190 / 10,414** | **`0`** | `97.8%` |
| `+158.37s` | 10,371 | 43 | **10,371 / 10,414** | **`0`** | `99.6%` |
| `+173.72s` | 10,382 | 32 | **10,382 / 10,414** | **`0`** | `99.7%` |
| **`+183.94s` (Final Convergence)** | **10,414** | **0** | **10,414 / 10,414** | **`0` (`+0.0 s` extra wait)** | **`100.0%`** |

---

## 5. 10,456-Node Multi-View Status & Info Query Benchmarks

Measured across **5 consecutive iterations** for each of the 9 query endpoints (**45 calls total**) with **10,456 node records** active:

| Query Category | Endpoint | Internal Serving Path | Payload Size | Min (ms) | Median (ms) | Avg (ms) | Max (ms) | Result |
|---|---|---|---:|---:|---:|---:|---:|---|
| **Infra ID List** | `GET /ns/default/infra?option=id` | etcd `WithKeysOnly` | `36 B` | **36.86** | **48.61** | **46.05** | 53.87 | ✅ `5/5 200 OK` |
| **Refined Detail View** | `GET /ns/default/infra/{id}?detail=refined` | Parallel etcd read + blueprint hydration | `22.61 MB` | **80.17** | **122.63** | **258.14** | 799.54 | ✅ `5/5 200 OK` |
| **Single Infra Status** | `GET /ns/default/infra/{id}?option=status` | In-memory `StatusStore` (`GetInfraStatus`) | `4.77 MB` | **97.57** | **131.04** | **129.93** | 163.83 | ✅ `5/5 200 OK` |
| **All Infra Status List** | `GET /ns/default/infra?option=status` | In-memory `StatusStore` list | `4.77 MB` | **110.43** | **132.28** | **144.98** | 182.83 | ✅ `5/5 200 OK` |
| **Infra List (Simple)** | `GET /ns/default/infra?option=simple` | Brief assembly (`GetInfraInfoBrief`) | `26.45 MB` | **118.98** | **151.10** | **205.95** | 360.01 | ✅ `5/5 200 OK` |
| **Raw Detail View** | `GET /ns/default/infra/{id}?detail=raw` | Shared read cache / blueprint hydration | `22.61 MB` | **114.92** | **158.05** | **299.82** | 938.03 | ✅ `5/5 200 OK` |
| **Infra List (Default)** | `GET /ns/default/infra` | Brief assembly (`GetInfraInfoBrief`) | `26.45 MB` | **139.90** | **185.51** | **279.27** | 513.47 | ✅ `5/5 200 OK` |
| **Default Detail View** | `GET /ns/default/infra/{id}` | Shared read cache (`GetInfraInfoCached`, 5s TTL) | `22.61 MB` | **104.93** | **223.23** | **467.70** | 1,005.89 | ✅ `5/5 200 OK` |
| **Access Info View** | `GET /ns/default/infra/{id}?option=accessinfo` | Assembled from `NodeInfo` / `StatusStore` | `6.82 MB` | **709.40** | **788.65** | **834.12** | 1,005.68 | ✅ `5/5 200 OK` |

---

## 6. System Resource (CPU & Memory) Footprint

Pod CPU and memory metrics (`kubectl top pod -n cb-tumblebug`) sampled every 15 seconds across the 43m 45s run:

| Test Phase | `cb-tumblebug` CPU / Mem | `cb-spider` CPU / Mem | `cb-tumblebug-etcd` CPU / Mem | Notes |
|---|---|---|---|---|
| **0. Idle Baseline** | `2m` / **`36 MiB`** | `33m` / **`63 MiB`** | `6m` / **`381 MiB`** | Pre-test baseline (`0` Infras, `0` Shared Resources) |
| **1a. Creating 438 Shared Resources** | `33m – 142m` / `106 – 183 MiB` | **`762m`** / `148 MiB` | `35m` / `386 MiB` | Parallel creation of VNet/SG/SSHKey across 146 regions |
| **1b. Creating 10,456 VMs (Peak)** | **`1,312m`** / **`972 MiB`** | `291m` / **`385 MiB`** | **`946m`** / `492 MiB` | Concurrent VM creation and etcd blueprint persistence |
| **2. Query Benchmarks & Refine** | `63m – 236m` / `796 – 865 MiB` | `1m – 6m` / `325 MiB` | `24m – 167m` / `488 MiB` | 45 consecutive large-payload (`22.6 – 26.5 MB`) responses |
| **3. Bulk `Suspend` (10,414 VMs)** | `64m – 501m` / `710 – 991 MiB` | **`1m – 3m`** / **`324 MiB`** | `33m – 350m` / `452 – 509 MiB` | Direct AWS SDK bulk stop + batch status sweeping |
| **4. Bulk `Resume` + IP Refresh** | `64m – 333m` / `727 – 806 MiB` | `1m – 39m` / `324 – 353 MiB` | `11m – 250m` / `420 – 508 MiB` | Flat Spider memory thanks to AWS batch IP cache (`0` Spider calls) |
| **5. Bulk `Terminate` (10,414 VMs)** | `2,429m` / **`3,705 MiB`** *(brief peak)*<br>→ drops to **`53m` / `686 MiB`** | `8m – 201m` / `333 – 367 MiB` | `586m` / `1,427 MiB`<br>→ drops to `23m` / `869 MiB` | Brief spike during 10,414 Spider IID metadata cleanups, immediately reclaimed |
| **6. Delete Infra (`Delete Infra`)** | `19m – 232m` / `684 – 735 MiB` | `1m` / `334 – 366 MiB` | `7m – 74m` / `854 – 919 MiB` | 146-region `AuditInfra` & parallel etcd deletion |
| **7. Post-Cleanup Steady State** | `40m` / **`531 MiB`** | `9m` / **`354 MiB`** | `30m` / **`491 MiB`** | **0 Pod Restarts, 0 OOM Events across the entire lifecycle** |

---

## 7. Conclusions

1. **100% End-to-End Lifecycle Completion at 10,456-VM Scale Across 10 CSPs & 146 Regions**:
   * **438 / 438 shared resources (100% of 146 regions, including all 41 GCP regions) created and deleted in a single pass**, **10,414 / 10,456 VMs (99.60%, 100% across 9 CSPs) provisioned**, and **100% of surviving 10,414 VMs completed `Suspend` → `Resume` → `Terminate` → `Delete`** with zero `Undefined` regressions and zero orphaned cloud resources.
2. **Zero-Lag Public IP Reassignment on `Resume`**:
   * Combining the independent AWS batch Public IP cache ([`public_ip_cache.go`](file:///home/shson/go/src/github.com/cloud-barista/cb-tumblebug/src/core/csp/public_ip_cache.go)) with the dual-semaphore fallback ([`refreshBatchNodePublicIP`](file:///home/shson/go/src/github.com/cloud-barista/cb-tumblebug/src/core/infra/status_agent.go#L986-L1035)) enabled all 10,414 VMs to have their new `publicIP` populated at the exact moment `Running` convergence completed (`183.94 s`, `+0.0 s` extra wait).
3. **Bounded Memory & Near-Zero CB-Spider Overhead**:
   * Throughout the 44-minute 10,456-VM test, `cb-spider` memory stayed between **`63 MiB` and `385 MiB`**, confirming complete elimination of the pre-`#2461` 24 GiB+ OOM bottleneck.
