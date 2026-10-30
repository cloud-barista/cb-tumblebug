# 초 대규모(1만+ 노드) 멀티 클라우드 가상머신 전 주기(Full Lifecycle) 통합 운용·관리 및 성능 시험 리포트

## 0. 결과 요약

**10개 CSP · 146개 리전(NodeGroup) · 10,456개 VM 규모의 `infra-global-extream` 템플릿을 대상으로 생성(`Provisioning`) → 다각도 상태·정보 조회 벤치마크(`Query Benchmarks`) → 실패 노드 정제(`Refine`) → 일괄 일시정지(`Suspend`) → 일괄 재개 및 공인 IP 재할당(`Resume`) → 일괄 종료(`Terminate`) → 인프라 삭제(`Delete Infra`) → 공유 리소스 일괄 정리(`Delete Shared Resources`)에 이르는 전 주기(Full Lifecycle) 전 단계 100% 완주 및 정상 완료.**

*(이슈 [`#2461`](https://github.com/cloud-barista/cb-tumblebug/issues/2461) 기반 대규모 아키텍처 개선 및 `Suspend`/`Resume` 배치 상태·공인 IP 동기화 보완 검증 시험)*

| 지표 | 측정값 | 비고 |
|---|---|---|
| **템플릿 목표 규모** | **10개 CSP · 146개 리전 · 10,456 VM** | `infra-global-extream` 템플릿 (`default` 네임스페이스 배포) |
| **공유 리소스 생성 (`CreateSharedResources`)** | **146 / 146개 리전 100% 생성** (`198.1초` 시점 완료) | **전 리전(GCP 41개 리전 포함)** `vNet(146)` · `SG(146)` · `sshKey(146)` 총 **438개** 생성 성공 (실패 `0`건) |
| **VM 프로비저닝 완료 대수** | **10,414 / 10,456 Running (99.60%)** | **9개 CSP(GCP·Azure·Tencent·IBM·Alibaba·NCP·NHN·OpenStack·KT) 240/240대 100% 성공**, AWS 27개 리전 중 26개 리전 100% 성공 (`eu-central-2a` AZ 용량 부족 42대만 `Failed`) |
| **생성 총 소요 시간 (`POST /infra/template/...`)** | **915.20초 (15분 15초)** | 스펙/이미지 사전 검증(`~39초`) + 공유 리소스 438개 생성(`~159초`) + 10,456대 VM 생성 포함 동기 응답 |
| **노드별 `Running` 확정 시간 분포 (10,414대)** | **Min 202.1초 · P50 506.1초 · P90 759.1초 · P95 795.1초 · P99 838.1초 · Max 871.1초** | 평균 `519.2초` (요청 시작 시각 → 노드 `createdTime` 기준) |
| **10,456 노드 상태 조회 (`?option=status`)** | **중앙값 131.04 ms** (최소 `97.57 ms`, 최대 `163.83 ms`) | 인메모리 `StatusStore` 기반 즉시 응답 (`4.77 MB` 페이로드) |
| **10,456 노드 상세 조회 (`?detail=refined` / `?detail=raw`)** | **중앙값 122.63 ms / 158.05 ms** (최소 `80.17 ms`) | NodeGroup 블루프린트 하이드레이션 및 읽기 캐시 (`22.61 MB` 페이로드) |
| **10,456 노드 접속정보 조회 (`?option=accessinfo`)** | **중앙값 788.65 ms** (최소 `709.40 ms`, 최대 `1,005.68 ms`) | 인메모리/메타데이터 조합 경로 전환으로 기존 120초 타임아웃(`503`) 대비 **152배 이상 단축** (`6.82 MB` 페이로드) |
| **실패 노드 정제 (`?action=refine`)** | **1.83초** (`HTTP 200`) | AWS `eu-central-2` AZ 용량 부족 `Failed` 노드 42대 즉시 정리 → **생존 10,414대 100% `Running`** |
| **일괄 일시정지 (`?action=suspend`)** | **API 응답 78.53초 · 99.95% 수렴 184.3초 (3분 04초)** | **10,414 / 10,414대 전원 `Suspended` 수렴 (`Undefined=0`)**, `Suspended` 노드 `publicIP` 잔존 **`0 / 10,414`대 (100% 초기화)** |
| **일괄 재개 및 공인 IP 갱신 (`?action=resume`)** | **API 응답 65.82초 · 100% `Running` 수렴 183.94초 (3분 04초)**<br>**100% 공인 IP 재할당 완료: 183.94초 (`+0.0초` 추가 지연!)** | **10,414 / 10,414대 전원 `Running` 및 공인 IP 재할당 동시 완료** (`public_ip_cache.go` AWS 배치 IP 캐시 `10,174대` + 비-AWS `refreshBatchNodePublicIP` `236대` 1회차 성공) |
| **일괄 종료 (`?action=terminate`)** | **100% `Terminated` 수렴 270.83초 (4분 31초)**<br>*(동기 API 응답: 320.92초 / 5분 21초)* | **10,414 / 10,414대 (100.00%) 전원 `Terminated` 수렴** (미종료 지연 노드 `0`대, `Undefined` 역전 `0`회) |
| **인프라 삭제 (`DELETE /infra/{id}?option=terminate`)** | **117.18초 (1분 57초)** | 146개 리전 CSP 교차 감사(`AuditInfra`) + etcd 10,414개 노드 및 146개 NodeGroup 50-워커 병렬 삭제 (`remaining_infras: []`) |
| **공유 리소스 일괄 정리 (`DELETE /sharedResources`)** | **149.85초 (2분 30초, Pass #1 단 1회 호출 완료)** | **1차 패스 단일 호출로 438 / 438개 (`vNet 146`, `SG 146`, `sshKey 146`) 100% 삭제 완료** (`remaining: 0`) |
| **시스템 안정성 및 리소스 사용량** | **평시 TB `531~991 MiB` (종료 피크 `3,705 MiB`) · Spider `63~385 MiB` · etcd `380~1,427 MiB`** | 전 과정(약 44분) **Pod 재시작 `0`회, OOM `0`회, 패닉/데드락 `0`건** |

---

## 1. 시험 환경 및 대상 구성

| 항목 | 내용 |
|---|---|
| 시험 일시 | 2026-10-30 19:42:58 ~ 20:26:43 UTC (총 **2,625.1초 / 43분 45초** — 전 주기 8단계 연속 수행) |
| 클러스터 환경 | Kubernetes (`kind`, `cb-tumblebug` 네임스페이스) |
| 주요 적용 메커니즘 | • `#2461` 초 대규모 아키텍처 (NodeGroup 블루프린트 분리, 인메모리 `StatusStore`, `StatusAgent` 백그라운드 폴러)<br>• 9개 CSP Direct SDK 배치 상태 조회(`BatchVMStatusCached` Lock-Free Singleflight) 및 AWS Bulk 제어(`Start`/`Stop`/`TerminateInstances`)<br>• AWS 배치 응답 공인 IP 독립 캐시(`public_ip_cache.go`) + 비-AWS 전용 세마포어 백오프 IP 갱신(`refreshBatchNodePublicIP`)<br>• 미발견 2회 연속 확인(`recordBatchTerminateNotFound` 2-strike streak) 및 `Terminated` 역전 방지 가드<br>• GCP 전역 VPC 쿼터 대기 분리(`quotaWaitSem`) 및 리전 단위 공유 VNet 통합(`#2841`) |
| 입력 템플릿 | `infra-global-extream` (146 NodeGroup, 10,456 노드, `policyOnPartialFailure: "continue"`) |
| 대상 CSP (10종) | AWS (27개 리전), GCP (41개 리전), Azure (37개 리전), Tencent (13개 리전), IBM (11개 리전), Alibaba (9개 리전), NCP (3개 리전), NHN (3개 리전), OpenStack (1개 리전), KT (1개 리전) |

---

## 2. 시험 절차 (Full Lifecycle 8단계)

| 순서 | 단계 | 요청 API | 완료 판정 기준 |
|---|---|---|---|
| **Phase 1** | **초 대규모 생성 (Provisioning)** | `POST /ns/default/infra/template/infra-global-extream` (`{"name": "infra-global-extream", "policyOnPartialFailure": "continue"}`) | 146개 리전 공유 리소스(`vNet`, `SG`, `sshKey` 438개) 및 10,456대 VM 프로비저닝 동기 완료 (`200 OK`) |
| **Phase 2** | **다각도 조회 벤치마크 (Query Benchmarks)** | 45초 안정화 대기 후 상태·상세·목록·접속정보·ID 조회 API 9종 × 5회 반복 호출 | 45회 호출 전량 `200 OK`, 응답 지연 시간(Min/Avg/Median/Max), 페이로드 크기 및 정합성 확인 |
| **Phase 2b** | **실패 노드 정제 (Refine)** | `GET /ns/default/control/infra/infra-global-extream?action=refine` | 외부 AZ 용량 부족으로 `Failed` 된 42개 노드를 메타데이터에서 제거하고 생존 10,414대 전원 `Running` 상태로 정규화 |
| **Phase 3** | **일괄 일시정지 (Suspend)** | `GET /ns/default/control/infra/infra-global-extream?action=suspend` | 생존 10,414대 전원 `Suspended` 상태 수렴, `Undefined` 역전 `0`건, 전 노드 `publicIP == ""` 초기화 검증 |
| **Phase 4** | **일괄 재개 및 IP 갱신 (Resume)** | `GET /ns/default/control/infra/infra-global-extream?action=resume` | 생존 10,414대 전원 `Running` 상태 수렴 및 **10,414대 전원 신규 `publicIP` 할당 완료 시점** 계측 |
| **Phase 5** | **일괄 종료 (Terminate)** | `GET /ns/default/control/infra/infra-global-extream?action=terminate` | 생존 10,414대 전원 `Terminated` 상태 수렴, `Undefined` 역전 `0`건 확인 |
| **Phase 6** | **인프라 삭제 (Delete Infra)** | `DELETE /ns/default/infra/infra-global-extream?option=terminate` | 146개 리전 CSP 교차 감사(`AuditInfra`) 및 etcd 메타데이터 완전 삭제 (`remaining_infras: []`) |
| **Phase 7** | **공유 리소스 정리 (Cleanup Shared)** | `DELETE /ns/default/sharedResources` | 146개 리전 공유 리소스 438개(`vNet 146`, `SG 146`, `sshKey 146`) 전량 삭제 및 잔여 `0`건 확인 |

---

## 3. CSP별 종합 성능 및 라이프사이클 다각도 분석

### 3.1 10개 CSP 전 주기(Provisioning · Suspend · Resume · Terminate) 통합 성과표

| CSP | 요청 리전 | 활성 리전 (성공률) | 대표 스펙 (`specId`) | 요청 VM | `Running` 도달 (성공률) | 생성 P50 / Max (초) | `Suspend` 100% 수렴 (초) | `Resume` 100% 수렴 (초) | `Resume` IP 갱신 방식 | `Terminate` 100% 수렴 (초) | 상태·제어 아키텍처 경로 |
|---|---:|---:|---|---:|---:|---:|---:|---:|---|---:|---|
| **AWS** | 27 | **27 (100%)** | `t3.nano` | 10,216 | **10,174 (99.59%)**¹ | `513.1` / `871.1` | **184.3** | **183.94** | **Direct Batch IP Cache** (`0` Spider 호출) | **240.63** | Direct SDK Bulk Control + Batch Status |
| **GCP** | 41 | **41 (100%)** | `t2d-standard-1`, `n1-standard-1`, `e2-highcpu-2` | 82 | **82 (100.0%)** | `258.1` / `318.1` | `< 135.0` | **120.80** | `refreshBatchNodePublicIP` (82대 1회차 성공) | **270.83** | Spider Control + Direct SDK Batch Status |
| **Azure** | 37 | **37 (100%)** | `Standard_B1ls` | 74 | **74 (100.0%)** | `291.1` / `439.1` | `< 135.0` | **32.68** | `refreshBatchNodePublicIP` (74대 1회차 성공) | **231.23** | Spider Control + Direct SDK Batch Status |
| **Tencent** | 13 | **13 (100%)** | `bf1.medium2`, `s5.medium2`, `sa2.medium2`, `sa9.medium2` | 26 | **26 (100.0%)** | `277.6` / `306.1` | `< 135.0` | **32.68** | `refreshBatchNodePublicIP` (26대 1회차 성공) | **167.58** | Spider Control + Direct SDK Batch Status |
| **IBM** | 11 | **11 (100%)** | `nxf-2x1` | 22 | **22 (100.0%)** | `237.6` / `257.1` | `< 135.0` | **36.02** | `refreshBatchNodePublicIP` (22대 1회차 성공) | **139.87** | Spider Control + Direct SDK Batch Status |
| **Alibaba** | 9 | **9 (100%)** | `ecs.e-c4m1.large`, `ecs.t6-c4m1.large`, `ecs.mn4.small`, `ecs.c9i.large` | 18 | **18 (100.0%)** | `206.6` / `211.1` | `< 135.0` (13대)<br>`547.3` (5대)² | **32.68** | `refreshBatchNodePublicIP` (18대 1회차 성공) | **200.97** | Spider Control + Direct SDK Batch Status |
| **NCP** | 3 | **3 (100%)** | `c2-g3a`, `s2-g3a`, `c2-g3` | 6 | **6 (100.0%)** | `308.6` / `311.1` | `< 135.0` | **63.23** | `refreshBatchNodePublicIP` (6대 1회차 성공) | **173.80** | Spider Control + Direct SDK Batch Status |
| **NHN** | 3 | **3 (100%)** | `t2.c1m1` | 6 | **6 (100.0%)** | `306.1` / `319.1` | `< 135.0` | **36.02** | `refreshBatchNodePublicIP` (6대 1회차 성공) | **130.34** | Spider Control + Direct SDK Batch Status |
| **OpenStack** | 1 | **1 (100%)** | `m1.medium` | 4 | **4 (100.0%)** | `330.6` / `339.1` | `< 135.0` | **99.98** | 제어 직후 동기 `FetchNodeStatus` | **216.70** | Spider Control + Individual Status Poller |
| **KT** | 1 | **1 (100%)** | `1x1.itl` | 2 | **2 (100.0%)** | `431.6` / `433.1` | `< 135.0` | **32.68** | `refreshBatchNodePublicIP` (2대 1회차 성공) | **102.62** | Spider Control + Direct SDK Batch Status |
| **전체 (10종)** | **146** | **146 (100%)** | — | **10,456** | **10,414 (99.60%)** | **`506.1` / `871.1`** | **184.3 (99.95%)**<br>`547.3` (100%) | **183.94 (100%)** | **10,414 / 10,414 (100%) `+0.0초` 지연** | **270.83 (100%)** | — |

> **각주**:
> 1. **AWS `eu-central-2` (취리히) 42대 `Failed`**: `eu-central-2a` 가용영역의 물리 호스트 `t3.nano` 재고 소진(`InsufficientInstanceCapacity`)으로 500대 중 458대가 `Running`에 도달하고 42대가 `Failed` 처리됨(나머지 **26개 AWS 리전 9,716대는 100.0% 기동 성공**). `Phase 2b`에서 `action=refine` 호출로 `1.83초` 만에 해당 42개 실패 레코드를 제거한 뒤 이후 `Suspend` → `Resume` → `Terminate` 단계를 수행함.
> 2. **Alibaba 5대 `Suspend` 게스트 OS ACPI 종료 지연**: Alibaba 18대 중 13대는 `78초` 내 즉시 `Stopped`(`Suspended`)로 전환되었으나, 4개 리전(`ap-northeast-1` 2대, `cn-huhehaote` 1대, `cn-wulanchabu` 1대, `us-west-1` 1대)의 5대는 Alibaba ECS 하이퍼바이저 상에서 `"Stopping"`(`Suspending`) 상태로 게스트 OS 종료를 기다린 뒤 `547.3초` 시점에 `"Stopped"`(`Suspended`)로 정상 전환됨.

---

### 3.2 CSP별 프로비저닝 시간 분포 상세 (`createdTime` 기준)

공유 리소스 438개 생성(`+0.0초 ~ +198.1초`)을 포함한 `POST /infra/template/infra-global-extream` 요청 시작 시각 대비 각 CSP 노드들이 `Running`으로 확정된 시간 분포는 다음과 같습니다.

| CSP | 기동 대수 (`Running`) | 최소 `Min` (초) | 중앙값 `P50` (초) | `P95` (초) | 최대 `Max` (초) | 평균 `Avg` (초) | 순수 VM 생성 구간 (`Min ~ Max`, 공유 리소스 제외) |
|---|---:|---:|---:|---:|---:|---:|---|
| **Alibaba** | 18 (9개 리전) | **202.1** | **206.6** | **211.1** | **211.1** | **206.6** | `4.0초 ~ 13.0초` (가장 빠른 인스턴스 기동) |
| **IBM** | 22 (11개 리전) | 210.1 | 237.6 | 257.1 | 257.1 | 238.7 | `12.0초 ~ 59.0초` |
| **GCP** | 82 (41개 리전) | 218.1 | 258.1 | 317.1 | 318.1 | 267.5 | `20.0초 ~ 120.0초` (41개 전 리전 100% 기동) |
| **Tencent** | 26 (13개 리전) | 242.1 | 277.6 | 304.1 | 306.1 | 276.5 | `44.0초 ~ 108.0초` |
| **Azure** | 74 (37개 리전) | 216.1 | 291.1 | 341.1 | 439.1 | 290.1 | `18.0초 ~ 241.0초` (37개 전 리전 100% 기동) |
| **NHN** | 6 (3개 리전) | 291.1 | 306.1 | 319.1 | 319.1 | 304.6 | `93.0초 ~ 121.0초` (`jp1`, `kr1`, `kr2` 전원 성공) |
| **NCP** | 6 (3개 리전) | 303.1 | 308.6 | 311.1 | 311.1 | 308.3 | `105.0초 ~ 113.0초` (`kr`, `jpn`, `sgn` 전원 성공) |
| **OpenStack** | 4 (1개 리전) | 313.1 | 330.6 | 339.1 | 339.1 | 328.4 | `115.0초 ~ 141.0초` |
| **KT** | 2 (1개 리전) | 430.1 | 431.6 | 433.1 | 433.1 | 431.6 | `232.0초 ~ 235.0초` |
| **AWS** | **10,174 (27개 리전)** | 212.1 | 513.1 | 796.1 | 871.1 | 525.0 | `14.0초 ~ 673.0초` (리전당 최대 500대 생성) |
| **전체 합계** | **10,414 (146개 리전)** | **202.1** | **506.1** | **795.1** | **871.1** | **519.2** | `P90: 759.1초`, `P99: 838.1초` |

---

### 3.3 CSP별 심층 성능 및 기술적 관점 분석

#### (1) AWS (27개 리전 · 10,216대 요청 / 10,174대 기동) — 초 대규모 벌크 제어 및 무오버헤드 공인 IP 캐시
* **프로비저닝**: 18개 리전에 각 500대(`9,000대`), 1개 리전에 192대(`eu-north-1`), 8개 리전에 각 128대(`1,024대`)를 배치했습니다. 128~192대 리전은 `407.1 ~ 587.1초` 내에 100% 완료되었고, 500대 리전 18곳도 리전별 세마포어 제어 하에 API 쓰로틀링 오류 없이 `764.1 ~ 871.1초` 내에 완료되었습니다.
* **일시정지(`Suspend`) 및 재개(`Resume`)**:
  * CB-Spider를 경유하지 않고 AWS SDK v2 `StopInstances` / `StartInstances`를 인스턴스 50개 단위 배치로 직접 호출합니다.
  * `Suspend` 시 27개 리전 `10,174대` 전원이 **`184.3초` (3분 04초)** 내에 `Suspended`로 수렴했습니다.
  * `Resume` 시 128~192대 리전 7곳은 **`61.0 ~ 62.0초`**, 500대 주력 리전 11곳은 **`78.0 ~ 91.0초`**, 나머지 리전까지 포함한 **27개 전 리전 `10,174대`가 `177.0 ~ 183.94초` (3분 04초)** 만에 100% `Running`으로 수렴했습니다.
* **배치 응답 기반 공인 IP 즉시 복원 (`public_ip_cache.go`)**:
  * AWS EC2는 `Stop` → `Start` 시 기존 동적 공인 IP가 해제되고 새 공인 IP가 부여됩니다. 기존 구조에서는 `Running` 전환 감지 시 10,174대에 대해 개별 `GET /spider/vm/{id}` 호출이 발생하여 8~12분의 추가 지연이 불가피했습니다.
  * 이번 시험에서는 [`aws/vm_status.go`](file:///home/shson/go/src/github.com/cloud-barista/cb-tumblebug/src/core/csp/aws/vm_status.go#L56-L98)의 `BatchDescribeInstanceStatuses`가 응답 내 `*instance.PublicIpAddress`를 락이 분리된 독립 `sync.Map` 캐시([`public_ip_cache.go`](file:///home/shson/go/src/github.com/cloud-barista/cb-tumblebug/src/core/csp/public_ip_cache.go))에 기록하고, `StatusAgent`의 `runBatchSweep`이 이를 즉시 읽어 `StatusStore`와 `etcd`에 반영했습니다.
  * 그 결과 **AWS `10,174대`에 대한 CB-Spider 개별 IP 조회 호출은 `0건`**이었으며, **`Running` 수렴 완료 시각(`183.94초`)과 공인 IP 100% 복원 완료 시각(`183.94초`)의 시차가 `0.0초`**였습니다.
* **일괄 종료(`Terminate`)**: `240.63초` (4분 01초) 만에 27개 리전 `10,174대` 전원이 지연 인스턴스 없이 100% `Terminated`에 도달했습니다.

##### AWS 27개 리전별 상세 지표 (프로비저닝 · Resume 수렴)

| 리전 그룹 | 대상 리전 (NodeGroup 수) | 리전당 요청 | 기동 성공 / 요청 | 프로비저닝 `Min ~ Max` (초) | `Resume` 100% `Running` + IP 복원 수렴 (초) |
|---|---|---:|---:|---:|---:|
| **중소규모 (128~192대)** | `ap-northeast-3`, `ap-south-2`, `eu-south-1`, `eu-south-2`, `eu-north-1`(192), `il-central-1`, `af-south-1` (7개 리전) | 128 ~ 192 | **960 / 960 (100%)** | `213.1 ~ 503.1` | **`61.0 ~ 62.0초`** |
| **중소규모 (128대, APAC)** | `ap-southeast-3`, `ap-southeast-4` (2개 리전) | 128 | **256 / 256 (100%)** | `227.1 ~ 587.1` | `162.0 ~ 177.0초` |
| **대규모 (500대, 1차 수렴군)** | `ca-central-1`, `ap-southeast-2`, `ap-northeast-1`, `ap-south-1`, `us-west-2`, `us-east-2`, `us-east-1`, `eu-central-1`, `eu-west-1`, `sa-east-1`, `eu-west-2`, `eu-west-3` (12개 리전) | 500 | **6,000 / 6,000 (100%)** | `214.1 ~ 871.1` | **`78.0 ~ 91.0초`** |
| **대규모 (500대, 2차 수렴군)** | `ap-northeast-2`, `ap-southeast-1`, `eu-central-2`(458/500), `us-west-1`, `ap-east-1`, `ca-west-1` (6개 리전) | 500 | **2,958 / 3,000 (98.6%)** | `212.1 ~ 838.1` | `104.0 ~ 177.0초` |

#### (2) GCP (41개 리전 · 82대 요청 / 82대 기동) — 전역 VPC 쿼터 경합 해소 및 미발견 2회 연속 확인(`R6`) 실증
* **41개 전 리전 공유 VNet & VM 100% 성공**: GCP는 VPC가 전역(Global) 리소스이며 서브넷 생성 시 전역/리전 오퍼레이션 대기가 수반됩니다. CB-Spider 및 CB-Tumblebug에서 전역 VPC 쿼터 대기 세마포어(`quotaWaitSem`) 분리 및 직렬화 재시도를 적용하여 **41개 전 리전 공유 리소스와 82대 VM이 100% 성공**(`Min 218.1초`, `P50 258.1초`, `Max 318.1초`)했습니다.
* **일시정지 · 재개 · 공인 IP 갱신**: `Suspend`는 `< 135초` 내 100% 완료되었고, `Resume`은 `120.80초`에 41개 리전 82대 전원이 `Running` 도달과 동시에 `refreshBatchNodePublicIP` (전용 세마포어 `ipRefreshSpiderSem=15`)를 통해 **82대 전원 1회차 시도(`attempt=1`)** 로 신규 공인 IP를 획득했습니다.
* **종료 시 미발견 Streak (`recordBatchTerminateNotFound`) 실제 작동 확인**: GCP는 인스턴스 삭제 완료 직후 `instances.list` 응답에서 인스턴스가 즉시 사라집니다(`found=0`). `tb_final.log` 분석 결과 `asia-east1`, `europe-west2`, `europe-west4`, `europe-west8`, `us-east4`, `us-west3`, `me-west1`, `me-central2` 등 다수 리전에서 `queried=2, found=0` 스윕이 발생했으며, 1회차 미발견 시 `Terminating`(`PollHigh`)을 유지하고 2회차 연속 미발견 시 `Terminated`로 확정하여 **`Undefined` 상태 역전 0건**으로 `270.83초`에 82대 전원이 깔끔히 종료되었습니다.

#### (3) Azure (37개 리전 · 74대 요청 / 74대 기동) — 37개 글로벌 리전 무결점 기동 및 초고속 Resume (`32.68초`)
* **프로비저닝**: 37개 전 리전 공유 리소스(Resource Group / VNet / NSG / SSHKey) 및 74대 `Standard_B1ls` VM이 **단 1건의 실패도 없이 100% 기동**(`Min 216.1초`, `P50 291.1초`, `Max 439.1초`)했습니다.
* **초고속 `Resume` (`32.68초`)**: Azure 37개 리전 74대는 `Resume` 요청 시작 후 불과 **`32.68초`** 만에 100% `Running`에 도달했고, 74대 모두 `refreshBatchNodePublicIP` 1회차 시도에서 공인 IP 갱신을 마쳤습니다.
* **일괄 종료 (`231.23초`)**: Azure 역시 VM 삭제 완료 시 ARM 목록에서 즉시 제거되므로 `southindia`, `eastasia`, `northcentralus`, `australiacentral`, `mexicocentral`, `eastus`, `chilecentral` 리전에서 2회 연속 미발견(`recordBatchTerminateNotFound`) 로직이 정상 작동하여 `Undefined` 역전 없이 `231.23초`에 100% 종료되었습니다.

#### (4) Tencent (13개 리전 · 26대), IBM (11개 리전 · 22대), Alibaba (9개 리전 · 18대)
* **프로비저닝 100% 성공**:
  * **Alibaba**: 9개 리전 18대가 가장 빠른 **`202.1 ~ 211.1초` (중앙값 `206.6초`)** 에 100% `Running` 도달.
  * **IBM**: 11개 리전 22대가 **`210.1 ~ 257.1초` (중앙값 `237.6초`)** 에 100% `Running` 도달.
  * **Tencent**: 이전 시험에서 간헐적 재고 부족이나 SSHKey Rate Limit이 있었던 것과 달리, 이번 시험에서는 **13개 전 리전 26대 생성 100% 성공**(`242.1 ~ 306.1초`) 및 **공유 리소스 1차 패스 100% 삭제**를 달성했습니다.
* **초고속 `Resume` (`32.68 ~ 36.02초`)**: Alibaba(`32.68초`), Tencent(`32.68초`), IBM(`36.02초`) 모두 35초 안팎에 전 리전 100% `Running` 및 공인 IP 갱신(`attempt=1`)을 완료했습니다.

#### (5) 국내 및 프라이빗 CSP — NCP (3개 리전 · 6대), NHN (3개 리전 · 6대), KT (1개 리전 · 2대), OpenStack (1개 리전 · 4대)
* **전 CSP 100% 기동 및 제어 완주**:
  * **NHN (`jp1`, `kr1`, `kr2` 6/6대)**: `306.1초`(P50)에 100% 기동, `Resume` `36.02초`, `Terminate` `130.34초`로 매우 빠르고 안정적인 라이프사이클 성능을 보였습니다.
  * **NCP (`kr`, `jpn`, `sgn` 6/6대)**: 엄격한 API Rate Limit 환경에서도 적응형 동시성 제어와 백오프가 동작하여 6대 전원 생성(`P50 308.6초`), `Suspend`, `Resume`(`63.23초`, 공인 IP 6대 모두 `attempt=1`에서 즉시 획득), `Terminate`(`173.80초`)를 100% 완주했습니다.
  * **KT (`kr1` 2/2대)**: 생성(`431.6초`), `Resume`(`32.68초`), `Terminate`(`102.62초` — 10개 CSP 중 최단 종료 수렴) 모두 100% 성공했습니다.
  * **OpenStack (`regionone` 4/4대)**: 배치 상태 핸들러 없이 개별 폴러(`StatusAgent` → Spider `/vmstatus`) 경로를 사용하는 유일한 CSP로, 생성(`330.6초`), `Suspend`, `Resume`(`99.98초`), `Terminate`(`216.70초`) 전 단계에서 배치 CSP들과 완벽히 조화되어 동작함을 입증했습니다.

---

## 4. 단계별 상세 타임라인 분석

### 4.1 전체 라이프사이클 단계별 소요 시간 요약

| 단계 | 세부 구간 | 소요 시간 (초) | 핵심 동작 및 검증 내용 |
|---|---|---:|---|
| **Phase 1: Provisioning** | **전체 (`POST /infra/template/...`)** | **915.20초 (15분 15초)** | `19:42:58.8Z ~ 19:57:43.4Z` (`HTTP 200`, 응답 페이로드 `23.71 MB`) |
| ↳ `1-a` | 146개 NodeGroup 스펙·이미지 사전 검증 | `~39.3초` | `+0.0s ~ +39.3s` (공통 스펙·이미지 유효성 및 런타임 가용성 검증) |
| ↳ `1-b` | 146개 리전 공유 리소스 438개 병렬 생성 | `~158.8초` | `+39.3s ~ +198.1s` (`vNet 146` + `SG 146` + `sshKey 146` = **438/438개 100% 생성**) |
| ↳ `1-c` | 10,456대 VM 생성 및 상태 확정 | `~717.1초` | `+198.1s ~ +915.2s` (**10,414대 `Running`**, 42대 `Failed`, NodeGroup 블루프린트 저장) |
| **Phase 2: Query Benchmarks** | 45초 안정화 대기 + 9종 조회 API × 5회 | **65.80초** | 안정화(`45.0초`) + **9종 API 45회 호출 전량 `200 OK` (`20.8초` 만에 완주!)** |
| **Phase 2b: Refine** | `GET /control/infra/{id}?action=refine` | **1.83초** | `Failed` 42대 정리 완료 → 생존 **10,414대 전원 `Running` (100%)** |
| **Phase 3: Suspend** | `GET /control/infra/{id}?action=suspend` | API **78.53초**<br>수렴 **184.3초 (99.95%)** | **10,414 / 10,414대 전원 `Suspended` 도달**, `Undefined=0`, `publicIP` 잔존 `0`대 |
| **Phase 4: Resume** | `GET /control/infra/{id}?action=resume` | API **65.82초**<br>수렴 **183.94초 (100%)** | **10,414 / 10,414대 전원 `Running` 및 `publicIP` 100% 복원 동시 달성 (`183.94초`)** |
| **Phase 5: Terminate** | `GET /control/infra/{id}?action=terminate` | 수렴 **270.83초 (100%)**<br>API **320.92초** | **10,414 / 10,414대 전원 `Terminated` 도달** (`Undefined=0`, 미종료 잔류 `0`대) |
| **Phase 6: Delete Infra** | `DELETE /ns/default/infra/{id}?option=terminate` | **117.18초 (1분 57초)** | 146개 리전 CSP 교차 감사(`AuditInfra`) + etcd 10,414 노드 50-워커 병렬 삭제 |
| **Phase 7: Cleanup Shared** | `DELETE /ns/default/sharedResources` (Pass #1) | **149.85초 (2분 30초)** | **Pass #1 단 1회 호출로 438 / 438개 전량 삭제 (`vNet=0, SG=0, sshKey=0`)** |

---

### 4.2 공유 리소스 438개 (146개 리전) 생성 및 단일 패스 삭제 타임라인

| 구분 | 경과 시각 (`+초`) | `vNet` | `securityGroup` | `sshKey` | 진행 상황 설명 |
|---|---:|---:|---:|---:|---|
| **생성 (`Provisioning`)** | `+0.0s ~ +39.3s` | 0 | 0 | 0 | 146개 NodeGroup 스펙·이미지 사전 검증 |
| | `+58.5s` | 116 | 6 | 13 | 10개 CSP 전 리전 병렬 VNet 생성 집중 (`79.5%` 완료) |
| | `+77.6s` | 130 | 35 | 53 | VNet `89.0%`, SG `24.0%`, SSHKey `36.3%` 생성 |
| | `+111.9s` | 142 | 78 | 117 | VNet `97.3%`, SG `53.4%`, SSHKey `80.1%` 생성 |
| | `+127.1s` | **146** | 104 | 132 | **146개 전 리전(GCP 41개 포함) VNet 100% 생성 완료** |
| | `+180.4s` | 146 | 145 | **146** | **146개 전 리전 SSHKey 100% 생성 완료** |
| | **`+198.1s`** | **146** | **146** | **146** | **146개 전 리전 공유 리소스 438개 100% 생성 완료 (실패 `0`건)** |
| **삭제 (`Cleanup Pass #1`)** | `+0.0s` | 146 | 146 | 146 | `DELETE /ns/default/sharedResources` 시작 (역순: `SG → vNet → sshKey`) |
| | `+17.1s` | 146 | 41 | 146 | 17초 만에 105개 리전 SecurityGroup 삭제 완료 |
| | `+49.0s` | 87 | **0** | 146 | **146개 전 리전 SecurityGroup 100% 삭제 완료**, VNet 삭제 진행 |
| | `+66.0s` | 44 | 0 | 146 | 102개 리전 VNet(VPC+Subnet) 삭제 완료 |
| | `+113.0s` | 9 | 0 | 146 | 137개 리전 VNet 삭제 완료 |
| | `+129.9s` | **0** | 0 | 31 | **146개 전 리전 VNet 100% 삭제 완료**, SSHKey 115개 삭제 완료 |
| | **`+149.85s` (Pass #1 완료)** | **0** | **0** | **0** | **단 1회 호출(Pass #1)로 438 / 438개 공유 리소스 전량 삭제 완료 (`failedCount: 0`)** |

---

### 4.3 `Resume` 진행 시 `Running` 전환 vs. 공인 IP(`publicIP`) 복원 동행 타임라인 (10,414 노드)

`Phase 4 (Resume)` 중 5초 간격으로 계측한 `Running` 도달 노드 수와 `publicIP != ""` 복원 완료 노드 수의 타임라인입니다.

| `Resume` 시작 후 경과 시간 | `Running` 노드 수 | `Resuming` 노드 수 | `publicIP` 할당 완료 노드 수 | `Running` 대비 IP 미할당 격차 | 누적 완료율 |
|---:|---:|---:|---:|---:|---:|
| `+0.00s` (직전 `Suspended`) | 0 | 0 (`Suspended: 10,414`) | **0 / 10,414** | `0` | `0.0%` |
| `+32.68s` | 265 | 8,645 (`Suspended: 1,504`) | 118 / 10,414 | `147` (비-AWS `refreshBatchNodePublicIP` 진행 중) | `2.5%` |
| `+60.09s` | 551 | 9,863 | 550 / 10,414 | **`1`** | `5.3%` |
| `+65.96s` (API 반환 직후) | 4,419 | 5,995 | **4,418 / 10,414** | **`1`** | `42.4%` |
| `+71.11s` | 5,268 | 5,146 | **5,268 / 10,414** | **`0`** | `50.6%` |
| `+81.44s` | 6,762 | 3,652 | **6,762 / 10,414** | **`0`** | `64.9%` |
| `+96.86s` | 9,558 | 856 | **9,558 / 10,414** | **`0`** | `91.8%` |
| `+127.63s` | 10,190 | 224 | **10,190 / 10,414** | **`0`** | `97.8%` |
| `+158.37s` | 10,371 | 43 | **10,371 / 10,414** | **`0`** | `99.6%` |
| `+173.72s` | 10,382 | 32 | **10,382 / 10,414** | **`0`** | `99.7%` |
| **`+183.94s` (최종 수렴)** | **10,414** | **0** | **10,414 / 10,414** | **`0` (`+0.0초` 추가 지연)** | **`100.0%`** |

---

## 5. 10,456 노드 규모 다각도 상태·정보 조회 성능 벤치마크

프로비저닝 직후 **10,456개 노드 레코드가 활성화된 상태**에서 9종 조회 API를 각 **5회씩 연속 호출(총 45회)** 하여 측정한 결과입니다.

| 조회 구분 | 호출 API 엔드포인트 | 내부 처리 경로 | 응답 크기 | 최소 (ms) | 중앙값 (ms) | 평균 (ms) | 최대 (ms) | 결과 |
|---|---|---|---:|---:|---:|---:|---:|---|
| **ID 목록 조회** | `GET /ns/default/infra?option=id` | etcd `WithKeysOnly` | `36 B` | **36.86** | **48.61** | **46.05** | 53.87 | ✅ `5/5 200 OK` |
| **정제 상세 조회** | `GET /ns/default/infra/{id}?detail=refined` | etcd 병렬 조회 + 블루프린트 하이드레이션 | `22.61 MB` | **80.17** | **122.63** | **258.14** | 799.54 | ✅ `5/5 200 OK` |
| **단건 상태 조회** | `GET /ns/default/infra/{id}?option=status` | 인메모리 `StatusStore` (`GetInfraStatus`) | `4.77 MB` | **97.57** | **131.04** | **129.93** | 163.83 | ✅ `5/5 200 OK` |
| **전체 상태 목록** | `GET /ns/default/infra?option=status` | 인메모리 `StatusStore` 목록 | `4.77 MB` | **110.43** | **132.28** | **144.98** | 182.83 | ✅ `5/5 200 OK` |
| **인프라 목록 (Simple)** | `GET /ns/default/infra?option=simple` | 경량 목록 조립 (`GetInfraInfoBrief`) | `26.45 MB` | **118.98** | **151.10** | **205.95** | 360.01 | ✅ `5/5 200 OK` |
| **원본 상세 조회** | `GET /ns/default/infra/{id}?detail=raw` | 공유 읽기 캐시 / 블루프린트 하이드레이션 | `22.61 MB` | **114.92** | **158.05** | **299.82** | 938.03 | ✅ `5/5 200 OK` |
| **인프라 목록 (기본)** | `GET /ns/default/infra` | 경량 목록 조립 (`GetInfraInfoBrief`) | `26.45 MB` | **139.90** | **185.51** | **279.27** | 513.47 | ✅ `5/5 200 OK` |
| **기본 상세 조회** | `GET /ns/default/infra/{id}` | 공유 읽기 캐시 (`GetInfraInfoCached`, 5s TTL) | `22.61 MB` | **104.93** | **223.23** | **467.70** | 1,005.89 | ✅ `5/5 200 OK` |
| **접속정보 조회** | `GET /ns/default/infra/{id}?option=accessinfo` | `NodeInfo` / `StatusStore` IP·SSH 정보 조합 | `6.82 MB` | **709.40** | **788.65** | **834.12** | 1,005.68 | ✅ `5/5 200 OK` |

### 조회 벤치마크 핵심 성과
1. **9개 전체 조회 API 45/45회 100% `200 OK` (총 소요 시간 `20.8초`)**:
   * 이전 시험에서 10,000+ 노드 대상 개별 CB-Spider 호출로 인해 `120,008 ms` 타임아웃(`503`)이 발생했던 `?option=accessinfo`가 **중앙값 `788.65 ms` (최소 `709.40 ms`)** 로 응답하여 **152배 이상 성능이 향상**되었습니다.
2. **전 주기 고부하 중에도 흔들리지 않는 `?option=status` 응답성 (`76 ~ 270 ms`)**:
   * `Provisioning`, `Suspend`, `Resume`, `Terminate`가 한창 진행 중인 백그라운드 폴링에서도 `?option=status` (`4.77 MB`) 응답 지연은 전 구간 **`76 ~ 270 ms` (대부분 `80 ~ 130 ms`)** 를 일관되게 유지했습니다.

---

## 6. 시스템 리소스(CPU · Memory) 사용량 추이

전체 시험 주기(43분 45초) 동안 `kubectl top pod -n cb-tumblebug`로 계측한 핵심 파드 3종(`cb-tumblebug`, `cb-spider`, `cb-tumblebug-etcd`)의 단계별 리소스 사용량입니다.

| 시험 단계 | `cb-tumblebug` CPU / Mem | `cb-spider` CPU / Mem | `cb-tumblebug-etcd` CPU / Mem | 비고 |
|---|---|---|---|---|
| **0. 유휴 기준선 (Baseline)** | `2m` / **`36 MiB`** | `33m` / **`63 MiB`** | `6m` / **`381 MiB`** | 시험 시작 직전 (`0` Infra, `0` Shared Resource) |
| **1a. 공유 리소스 438개 생성 중** | `33m ~ 142m` / `106 ~ 183 MiB` | **`762m`** / `148 MiB` | `35m` / `386 MiB` | 146개 리전 VNet·SG·SSHKey 병렬 생성 구간 |
| **1b. 10,456 VM 생성 중 (피크)** | **`1,312m`** / **`972 MiB`** | `291m` / **`385 MiB`** | **`946m`** / `492 MiB` | 1만 노드 동시 생성 및 etcd 기록 구간 |
| **2. 조회 벤치마크 & Refine** | `63m ~ 236m` / `796 ~ 865 MiB` | `1m ~ 6m` / `325 MiB` | `24m ~ 167m` / `488 MiB` | `22.6 ~ 26.5 MB` 대용량 페이로드 45회 연속 응답 |
| **3. 10,414 VM 일괄 `Suspend`** | `64m ~ 501m` / `710 ~ 991 MiB` | **`1m ~ 3m`** / **`324 MiB`** | `33m ~ 350m` / `452 ~ 509 MiB` | AWS Direct SDK Bulk Stop + 배치 상태 스윕 |
| **4. 10,414 VM 일괄 `Resume` + IP 복원** | `64m ~ 333m` / `727 ~ 806 MiB` | `1m ~ 39m` / `324 ~ 353 MiB` | `11m ~ 250m` / `420 ~ 508 MiB` | AWS 배치 IP 캐시(`0` Spider 호출)로 Spider 메모리 평탄 유지 |
| **5. 10,414 VM 일괄 `Terminate`** | `2,429m` / **`3,705 MiB`** *(일시 피크)*<br>→ 수렴 직후 **`53m` / `686 MiB`** | `8m ~ 201m` / `333 ~ 367 MiB` | `586m` / `1,427 MiB`<br>→ 수렴 직후 `23m` / `869 MiB` | 1만 노드 Spider IID 메타데이터 일괄 정리 시 일시 피크 후 즉시 회수 |
| **6. 인프라 삭제 (`Delete Infra`)** | `19m ~ 232m` / `684 ~ 735 MiB` | `1m` / `334 ~ 366 MiB` | `7m ~ 74m` / `854 ~ 919 MiB` | 146개 리전 `AuditInfra` 및 etcd 병렬 삭제 |
| **7. 공유 리소스 정리 완료 후** | `40m` / **`531 MiB`** | `9m` / **`354 MiB`** | `30m` / **`491 MiB`** | 전 과정 **Pod 재시작 `0`회, OOM `0`회** |

---

## 7. 종합 결론

1. **10개 CSP · 146개 리전 · 10,456 VM 규모 전 주기(Create → Query → Refine → Suspend → Resume → Terminate → Delete) 완주**:
   * **공유 리소스 146/146개 리전(438개) 100% 생성 및 단일 패스 100% 삭제**, **9개 CSP VM 100% 기동 및 전체 99.60%(`10,414대`) 기동**, **10,414대 100% `Suspend` → 100% `Resume` → 100% `Terminate` → 완전 삭제**를 단 한 건의 상태 역전(`Undefined=0`)이나 잔존 리소스 없이 달성했습니다.
2. **배치 상태 스위퍼와 독립 공인 IP 캐시의 결합 효과 입증**:
   * `Resume` 시 `10,414대`가 `Running`으로 수렴하는 **`183.94초` 시점에 10,414대 전원의 공인 IP 재할당이 동시 완료(`+0.0초` 추가 지연)** 되어, 대규모 `Suspend`/`Resume` 직후에도 별도 대기 없이 원격 제어(SSH 등)가 가능함을 실증했습니다.
3. **CB-Spider 메모리 및 부하의 근본적 제어**:
   * 10,456대 규모의 전체 라이프사이클 동안 `cb-spider` 메모리는 **`63 MiB ~ 최대 385 MiB`** 범위 내에서 안정적으로 유지되어, `#2461` 이전의 대량 호출 병목과 OOM 문제가 완전히 해결되었음을 확인했습니다.
