# 스펙 및 이미지 추천 (Spec and Image Recommendation)

최적의 컴퓨트 스펙 및 이미지를 선택하고, 실시간 CSP 가용성을 검증하며, 스펙-이미지 호환성을 강제하고, 스펙-이미지 쌍을 사전 검토하며, 여러 CSP 간 대체 구성을 찾는 방법에 대한 가이드입니다.

## 📑 목차

1. [개요](#개요)
2. [스펙 추천](#스펙-추천)
3. [실시간 스펙 가용성 및 프로비저닝 피드백 루프](#실시간-스펙-가용성-및-프로비저닝-피드백-루프)
4. [이미지 검색 및 스펙-이미지 호환성](#이미지-검색-및-스펙-이미지-호환성)
5. [스펙-이미지 쌍 검토 및 사전 검증 (Pre-Flight Validation)](#스펙-이미지-쌍-검토-및-사전-검증-pre-flight-validation)
6. [대체 노드 구성 (Alternative Node Config)](#대체-노드-구성-alternative-node-config)
7. [워크플로 예제](#워크플로-예제)

---

## 개요

CB-Tumblebug은 사용자가 멀티 클라우드 인프라 프로비저닝을 위한 컴퓨트 리소스를 선택할 수 있도록 일련의 API를 제공합니다. 이 API들은 리소스 탐색 및 호환성 필터링부터 사전 검증(Pre-flight Review) 및 CSP 간 이식성 확보에 이르기까지 자연스러운 흐름으로 함께 동작합니다:

```
RecommendSpec  →  SearchImage  →  InfraDynamicReview  →  RecommendAlternativeNodeConfig
  Find spec &       Find spec-      Validate availability   Port config to another CSP
  check stock     compatible OS     & pair compatibility
```

| API | 메서드 | 경로 |
|-----|--------|------|
| 스펙 추천 (Recommend Spec) | POST | `/tumblebug/recommendSpec` |
| 추천 옵션 조회 (Get Recommendation Options) | GET | `/tumblebug/recommendSpecOptions` |
| 범위별 스펙 필터링 (Filter Specs by Range) | POST | `/tumblebug/ns/{nsId}/resources/filterSpecsByRange` |
| 단일 스펙 조회 (온디맨드 가용성 확인) | GET | `/tumblebug/ns/{nsId}/resources/spec/{specId}?forceRefresh=true` |
| 이미지 검색 (Search Image) | POST | `/tumblebug/ns/{nsId}/resources/searchImage` |
| 검색 옵션 조회 (Get Search Options) | GET | `/tumblebug/ns/{nsId}/resources/searchImageOptions` |
| 동적 Infra 검토 (스펙+이미지 검증) | POST | `/tumblebug/ns/{nsId}/infraDynamicReview` |
| 동적 Infra 요청 확인 (Check Dynamic Infra Request) | POST | `/tumblebug/ns/{nsId}/checkInfraDynamicReq` |
| 대체 구성 추천 (Recommend Alternative Config) | POST | `/tumblebug/recommendAlternativeNodeConfig` |

---

## 스펙 추천

### 주요 역할

`POST /tumblebug/recommendSpec`은 사용자 정의 조건과 우선순위에 따라 글로벌 스펙 카탈로그에서 컴퓨트 스펙을 필터링하고 순위를 매깁니다. 실시간 CSP 가용성 메타데이터가 보강되어 정렬된 스펙 목록을 반환합니다.

### 요청 (Request)

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

### 필터 메트릭 (Filter Metrics)

| 메트릭 | 연산자 | 예시 |
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

### 우선순위 메트릭 (Priority Metrics)

| 메트릭 | 설명 |
|--------|-------------|
| `cost` | `costPerHour` 기준 오름차순 (가장 저렴한 항목 우선) |
| `performance` | 벤치마크 점수 기준 내림차순 |
| `location` | 주어진 좌표와의 근접성 기준 |
| `latency` | 측정된 네트워크 지연 시간(Latency) 기준 |
| `random` | 무작위 순서 |

### 사용 가능한 필터/우선순위 옵션 조회

```
GET /tumblebug/recommendSpecOptions
```

`metric`, `operator`, `priority` 필드에 사용할 수 있는 모든 유효한 값을 반환합니다. 동적 UI를 구축하거나 `recommendSpec`을 호출하기 전에 입력값을 검증할 때 유용합니다.

---

## 실시간 스펙 가용성 및 프로비저닝 피드백 루프

정적 스펙 카탈로그(`spec_infos`)만으로는 특정 가용 영역(AZ)에 해당 스펙의 재고가 현재 존재하는지, 사용자의 구독(Subscription)에 충분한 vCPU/GPU 쿼터(Quota)가 있는지, 혹은 일시적인 용량 고갈로 인해 최근 VM 생성이 실패했는지 여부를 반영할 수 없습니다. CB-Tumblebug은 표준화된 런타임 가용성 필드로 `SpecInfo` 응답을 보강합니다.

### `SpecInfo`의 가용성 필드

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

### 표준화된 `availabilityStatus` 값

| 상태 | 의미 | 프로비저닝 동작 |
|--------|---------|-----------------------|
| `Available` | 스펙이 최소 하나 이상의 가용 영역(Zone)에서 사전 제공/재고/쿼터 검사를 통과함. 리전 내 일부 영역을 사용할 수 없는 경우(`len(unavailableZones) > 0`), 부분 영역 가용성(예: `Available (5/6 AZ)`) 및 `availabilityReason`이 함께 보고됩니다. | 허용됨; 자동 영역(Auto-zone) 선택 시 `unavailableZones`를 회피합니다. |
| `SoldOut` | 스펙이 리전 내 모든 영역(또는 사용자가 고정한 영역)에서 일시적으로 품절(재고 부족)됨. | 검토(Review) 단계에서 차단됨; 사용자에게 다른 리전/스펙을 선택하도록 경고합니다. |
| `QuotaExceeded` | 자격 증명 소유자의 리전별 vCPU 또는 GPU 쿼터가 부족하거나, 해당 패밀리에 대해 구독 옵트인(Opt-in)/쿼터 증설이 필요함. | 쿼터 상세 정보와 함께 검토 단계에서 차단됨. |
| `Unsupported` | 스펙이 단종되었거나 제한되었거나 대상 리전의 어느 영역에서도 제공되지 않음. | 검토 단계에서 차단됨. |
| `Unverified` | 실시간 가용성이 아직 조회되지 않았거나(예: 초기 능동 검사 배치를 초과한 후보) 해당 CSP에 등록된 검사기가 없음(예: `openstack`). | 허용됨; `GET /spec/{specId}`를 통해 온디맨드로 검증할 수 있습니다. |

### CSP별 사전 가용성 검사기 (Pre-Flight Availability Checkers)

| CSP | 기반 API / 메커니즘 | 검증 신호 |
|-----|---------------------------|------------------|
| **AWS** | `EC2 DescribeInstanceTypeOfferings` (`LocationType=availability-zone`, 리전별 캐싱) | AZ별 제공 여부 (`availableZones` vs `cloudinfo.yaml`의 리전 영역 목록) |
| **Azure** | `Compute ResourceSkus` (`locationInfo` + `restrictions`) 및 `Usage` API | 영역별 SKU 제한 사항 (`NotAvailableForSubscription`, `NotAvailableForRegion`) 및 리전별 vCPU 패밀리 쿼터 |
| **GCP** | 리전 범위 `zones.list` + 병렬 `machineTypes.list` (리전당 30분 캐싱) + `regions.get` | 영역별 machineType 존재 여부 및 리전별 GPU/CPU 쿼터 한도 |
| **Alibaba** | `ECS DescribeAvailableResource` (`DestinationResource=InstanceType`) | 실시간 영역별 재고 (`WithStock`, `ClosedWithStock`, `WithoutStock`) 및 시스템 디스크 호환성 |
| **Tencent** | `CVM DescribeZoneInstanceConfigInfos` | 실시간 영역별 `STATUS` (`SELL` vs `SOLD_OUT`) |
| **IBM** | `VPC GetInstanceProfile` + `ListZones` | VPC 영역 전반의 리전 프로필 제공 여부 |
| **NCP** | `vserver getServerProductList` + `getZoneList` | 리전/영역 내 서버 상품 가용성 |
| **NHN** | `Nova /flavors/detail` + `/os-availability-zone` | 플레이버(Flavor) 존재 여부 및 `OS-FLV-DISABLED:disabled` 상태 |
| **KT** | `CloudStack listServiceOfferings` + `listZones` | 영역 내 서비스 오퍼링 존재 여부 |

### 2계층 지연 시간 최적화 (빠른 추천 + 점진적 검증)

수십 개 리전에 걸쳐 실시간 CSP 가용성 API를 모든 후보군에 대해 한꺼번에 호출하면 지연 시간(Latency)이 증가할 수 있습니다. CB-Tumblebug은 2계층(Two-Tier) 보강 전략을 사용합니다:

1. **계층 1 — 즉시 보강 (`POST /tumblebug/recommendSpec`)**:
   - **1단계 (0ms 수동 캐시 조회)**: 조건에 부합하는 모든 스펙은 인메모리 가용성 캐시에 유효한 항목이 존재할 경우 즉시 보강됩니다.
   - **2단계 (상위 10개 능동 검사)**: 아직 캐싱되지 않은 상위 순위 후보 중 최대 **10개**까지 병렬로 능동 검증을 수행합니다(워커 `8`개, 스펙당 타임아웃 `6s`). 아직 캐싱되지 않은 11위 이하 후보들은 `availabilityStatus = "Unverified"` 상태로 즉시 반환되어 API 지연 시간을 낮게(~1초) 유지합니다.
   - **순위 재조정 (Re-Ranking)**: 사용 불가로 확인된 스펙(`SoldOut`, `QuotaExceeded`, `Unsupported`)은 `Available` 및 `Unverified` 후보들 아래로 강등되어 최상위 추천 결과가 항상 실행 가능한 스펙이 되도록 보장합니다.
2. **계층 2 — 온디맨드 / 점진적 검증 (`GET /tumblebug/ns/{nsId}/resources/spec/{specId}`)**:
   - `GET /tumblebug/ns/{nsId}/resources/spec/{specId}`(선택적으로 `?forceRefresh=true` 포함)를 호출하면 해당 특정 스펙에 대한 능동 가용성 검사를 수행하고 결과를 캐싱합니다.
   - `cb-mapui`와 같은 UI는 상위 10개 스펙이 검증된 추천 목록을 즉시 렌더링한 뒤, 나머지 `Unverified` 행이나 로드된 템플릿 NodeGroup 칩들을 백그라운드에서 점진적으로 확인합니다.

### 프로비저닝 피드백 루프 (Write-Through 캐시 및 영역 제외)

일부 CSP 사전 검증 API(특히 AWS `DescribeInstanceTypeOfferings`, GCP `machineTypes.list`, IBM `GetInstanceProfile`)는 실시간 물리 서버 풀의 재고가 아니라 AZ별 **카탈로그 제공 여부**를 보고합니다. 결과적으로 `Available (6 AZ)`로 보고된 스펙이라도 `POST /infraDynamic` 실행 중에 특정 AZ(예: `us-east-1b`)에서 AWS `InsufficientInstanceCapacity`와 같은 런타임 오류가 발생할 수 있습니다.

CB-Tumblebug은 2단계 런타임 피드백 루프를 통해 이러한 격차를 해소합니다:

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

## 이미지 검색 및 스펙-이미지 호환성

### 주요 역할

`POST /tumblebug/ns/{nsId}/resources/searchImage`는 여러 필터 조건을 사용하여 이미지 카탈로그를 질의합니다. `matchedSpecId`가 제공되면 CB-Tumblebug은 해당 스펙의 `providerName`, `regionName`, `osArchitecture`를 자동으로 상속하며, 날짜 및 버전별로 이미지를 중복 제거하기 *전에* **CSP별 하드웨어/펌웨어/하이퍼바이저 호환성 규칙**을 강제 적용합니다.

### 요청 (Request)

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

### 필터 필드

| 필드 | 타입 | 설명 |
|-------|------|-------------|
| `matchedSpecId` | string | 이 스펙 ID와의 CSP별 호환성을 강제 적용합니다 (`providerName`, `regionName`, `osArchitecture`, 부팅 모드, 하이퍼바이저, 컨트롤러, 디스크 크기 등) |
| `providerName` | string | CSP 이름 (예: `aws`, `azure`, `gcp`) |
| `regionName` | string | CSP 리전 (예: `us-east-1`) |
| `osType` | string | 공백으로 구분된 AND 조건 (예: `"ubuntu 22.04"`) |
| `osArchitecture` | string | `x86_64`, `arm64` 등 |
| `isGPUImage` | bool | GPU 지원 이미지 (드라이버 사전 설치 여부는 무관) |
| `isBasicGpuImage` | bool | 드라이버가 사전 설치된 GPU 이미지 — GPU 워크로드에 권장됨 |
| `isKubernetesImage` | bool | Kubernetes 노드에 적합한 이미지 |
| `isRegisteredByAsset` | bool | CB-Tumblebug 에셋(Asset) 파일을 통해 등록된 이미지 |
| `includeBasicImageOnly` | bool | 순수 OS 이미지만 반환 (애플리케이션 스택 제외) |
| `includeDeprecatedImage` | bool | 지원 중단(Deprecated)/수명 종료(EOL) 이미지 포함 (기본값: false) |
| `detailSearchKeys` | []string | 이미지 상세 정보 내 AND 키워드 검색 (예: `["tensorflow", "2.17"]`) |
| `maxResults` | int | 반환할 최대 결과 수 |

### 이미지 분류 플래그

CB-Tumblebug은 이미지를 상호 배타적인 세 가지 카테고리로 분류합니다:

| 플래그 | 의미 |
|------|---------|
| `isBasicImage = true` | GPU 드라이버가 사전 설치되지 않은 순수 OS 설치 이미지입니다. 비-GPU 워크로드에서 최우선으로 선택됩니다. |
| `isBasicGpuImage = true` | GPU 드라이버(CUDA 등)가 사전 설치된 기본 OS 이미지입니다. 항상 `isBasicImage = false`를 가집니다. GPU 워크로드에서 최우선으로 선택됩니다. |
| 둘 다 아님 | 특수 목적 또는 애플리케이션 전용 이미지입니다 (마켓플레이스, ML 프레임워크, DB 어플라이언스 등). |

> **참고:** `isBasicGpuImage = true`와 `isGPUImage = false`를 동시에 지정하면 모순된 조건으로 거부됩니다.

### CSP별 스펙 ↔ 이미지 호환성 규칙

이미지가 스펙의 CSP, 리전, CPU 아키텍처(`x86_64` / `arm64`)와 일치하더라도 저수준 펌웨어, 하이퍼바이저, 네트워크/디스크 컨트롤러 또는 고정 루트 디스크 제약 조건을 위반하면 VM 프로비저닝이 실패합니다. `searchImage`(`matchedSpecId` 설정 시)와 `CheckSpecImageCompatibility`(`infraDynamicReview` / `checkInfraDynamicReq` 실행 시) 모두 다음 규칙을 강제 적용합니다:

| CSP | 강제 적용되는 호환성 차원 | 상세 내용 |
|-----|-----------------------------------|---------|
| **모든 CSP** | CPU 아키텍처 및 고정 루트 디스크 크기 | • `spec.architecture`(`x86_64`, `arm64`)가 `image.osArchitecture`와 일치해야 합니다.<br>• 고정 로컬 OS 루트 디스크를 사용하는 CSP/플레이버(`ncp`, `kt`, `openstack` 및 `nhn` 로컬 부팅 `u2.*` 플레이버)의 경우, `image.osDiskSizeGB`가 `spec.rootDiskSize`를 초과할 수 없습니다. |
| **AWS** | 부팅 모드, 가상화 유형, ENA, SR-IOV, 베어메탈 | • `image.BootMode`(`uefi`, `legacy-bios`, `uefi-preferred`)가 `spec.SupportedBootModes`에서 지원되어야 합니다.<br>• `image.VirtualizationType`(`hvm` vs `paravirtual`)이 `spec.SupportedVirtualizationTypes`와 일치해야 합니다.<br>• ENA를 요구하는 Nitro / 최신 스펙은 `EnaSupport=true`가 없는 이미지를 거부합니다.<br>• 베어메탈 스펙(`*.metal`)은 `paravirtual` 이미지를 거부합니다. |
| **Azure** | Hyper-V 세대, 디스크 컨트롤러(`NVMe` vs `SCSI`), Confidential VM | • `image.hyperVGeneration`(`V1`, `V2`)이 `spec.HyperVGenerations`와 일치해야 합니다(Gen1 전용 스펙은 카탈로그 수집 시 필터링됨).<br>• `NVMe`만 지원하는 스펙(`DiskControllerTypes=NVMe`, 예: `Standard_D*s_v6`, `Standard_E*s_v6` 등 v6/v7 패밀리)은 `DiskControllerTypes`에 `NVMe` 지원을 명시한 이미지가 필요합니다.<br>• 기밀 컴퓨팅(Confidential Computing) 스펙(`ConfidentialComputingType=SNP` 또는 `TDX`, 예: `DCasv5`, `ECasv5`, `DCesv5`)은 `ConfidentialVM` / `CVM`을 지원하는 이미지가 필요합니다. |
| **GCP** | `GuestOsFeatures` (`GVNIC`, `IDPF`, `UEFI_COMPATIBLE`) | • Gen3/Gen4 및 GPU 머신 패밀리(`c3`, `c3d`, `c4`, `c4a`, `c4d`, `n4`, `n4a`, `g2`, `g4`, `a2-ultragpu`, `a3`, `a4`, `m3`, `m4`, `z3`, `x4`, `h3`)는 `GuestOsFeatures`에 `GVNIC`이 포함된 이미지가 필요합니다.<br>• Gen4 패밀리(`c4`, `c4a`, `c4d`, `n4`, `n4a`, `a4`, `x4`)는 추가로 `IDPF`(또는 호환되는 최신 gVNIC/IDPF OS 지원)가 필요합니다.<br>• ARM(`t2a`, `c4a`, `n4a`) 및 Gen3+ 스펙은 `UEFI_COMPATIBLE`이 필요합니다. |
| **Alibaba** | 부팅 모드(`UEFI` / `BIOS`) 및 `NVMeSupport` | • `image.BootMode`(`UEFI`, `BIOS`, `UEFI-Preferred`)가 인스턴스 패밀리와 호환되어야 합니다.<br>• 8세대(`g8i`, `c8i`, `r8i`, `g8a`, `c8a`, `r8a`, `g8y`) 및 로컬 NVMe 패밀리(`i2`, `i3`, `i4`, `d2`, `d3`, `gn6e`, `gn7`, `gn8`, `ebm*`)는 `NVMeSupport=supported`인 이미지가 필요합니다. |
| **NCP** | 하이퍼바이저(`XEN` vs `KVM`) 및 `CorrespondingImageIds` | • `spec` 하이퍼바이저(G1/G2의 경우 `XEN`, G3/gpu-g3의 경우 `KVM`)가 `image` 하이퍼바이저와 일치해야 합니다.<br>• `spec.Details`에 `CorrespondingImageIds`가 정의된 경우, 이미지의 `cspImageName`이 해당 허용 목록에 포함되어야 합니다. |
| **NHN** | 플레이버 및 부팅 디스크 속성 | • NHN 이미지의 `Properties`를 평가합니다: `nhncloud_allow_gpu_flavor`(GPU 스펙용), `nhncloud_allow_cpu_flavor`(CPU 스펙용), `nhncloud_allow_local_bootdisk_flavor`(로컬 디스크 `u2.*` 스펙용), 그리고 `spec.vCPU` 대비 `min_cpu:<N>` 조건을 확인합니다. |
| **IBM** | VSI 제한 플래그 | • 표준 VPC 가상 서버 인스턴스(VSI)를 프로비저닝할 때 베어메탈 전용 또는 전용 호스트(Dedicated Host) 전용으로 제한된 이미지를 제외합니다. |

### 사용 가능한 검색 옵션 조회

```
GET /tumblebug/ns/{nsId}/resources/searchImageOptions
```

각 필터 필드에 대해 현재 이미지 카탈로그에서 사용 가능한 모든 고유 값을 반환합니다. 드롭다운 메뉴를 구성하거나 빈 결과 쿼리를 방지할 때 유용합니다.

---

## 스펙-이미지 쌍 검토 및 사전 검증 (Pre-Flight Validation)

`POST /tumblebug/ns/{nsId}/infraDynamic`으로 인프라를 프로비저닝하기 전에, 호출자는 `POST /tumblebug/ns/{nsId}/infraDynamicReview`(또는 `POST /tumblebug/ns/{nsId}/checkInfraDynamicReq`)를 사용하여 NodeGroup별 **스펙 가용성**, **영역 재고 소진 이력**, **이미지 가용성**, **스펙-이미지 호환성**을 포함한 전체 요청을 사전 검증할 수 있습니다.

### 권장 패턴

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

### 예제: `POST /tumblebug/ns/{nsId}/infraDynamicReview`

스펙-이미지 쌍이 호환되지 않거나(예: 구형 SCSI 전용 Ubuntu 이미지와 NVMe 전용 Azure `Standard_D4s_v6` 스펙을 조합한 경우) 리전 내 영역 중 하나에서 최근 재고 소진이 발생했을 때, `infraDynamicReview`는 즉각 조치 가능한 진단 결과를 보고합니다:

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

### 이미지 선택 가이드라인

| 워크로드 | 권장 이미지 플래그 |
|----------|----------------------|
| 일반 컴퓨트 (General compute) | `isBasicImage = true` |
| GPU / ML 학습 | `isBasicGpuImage = true` |
| Kubernetes 노드 | `isKubernetesImage = true` |
| 커스텀 소프트웨어 스택 | `detailSearchKeys`를 사용하여 일치하는 마켓플레이스 이미지 검색 |

---

## 대체 노드 구성 (Alternative Node Config)

### 주요 역할

`POST /tumblebug/recommendAlternativeNodeConfig`은 기존 NodeGroup 구성(소스 스펙 및 선택적으로 해당 이미지)을 입력받아 다른 대상 CSP 및/또는 리전에서 가장 잘 부합하는 대체 구성을 찾아줍니다.

이 기능은 다음과 같은 경우에 유용합니다:
- 워크로드를 다른 클라우드 프로바이더로 마이그레이션할 때
- 기본 리전/CSP에서 스펙이 `SoldOut` 또는 `QuotaExceeded` 상태여서 대체재를 찾아야 할 때
- 기존 구성에 대해 비용 최적화된 대안을 찾을 때
- 여러 CSP에 걸쳐 동등한 수준의 리소스를 동시에 평가할 때

### 요청 (Request)

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

### 매칭 정책 (Match Policies)

각 스펙 필드에는 세 가지 정책 중 하나를 할당할 수 있습니다:

| 정책 | 동작 방식 | 점수 반영 여부 |
|--------|----------|--------------------|
| `required` | 정확히 일치하는 항목만 필터링 — 일치하지 않는 후보는 제외됨 | 점수 계산 제외 |
| `preferred` | `±tolerancePercent`를 사용한 범위 필터링 및 근접도에 따른 점수 부여 | 반영됨 (Yes) |
| `open` | 필터를 적용하지 않음; 차이(Diff)는 보고되지만 점수에는 반영되지 않음 | 반영 안 됨 (No) |

**기본 정책** (`matchCriteria`가 생략되었을 때 적용됨):

| 필드 | 기본값 |
|-------|---------|
| `architecture` | `required` |
| `acceleratorType` | `required` |
| `vCPU` | `preferred` |
| `memoryGiB` | `preferred` |
| `acceleratorCount` | `preferred` |
| `acceleratorMemoryGB` | `preferred` |
| `acceleratorModel` | `open` |
| `costPerHour` | `open` |

### 유사도 점수 (Similarity Score)

유사도 점수(0–100)는 `preferred` 필드들만을 기반으로 계산됩니다.

**비-GPU 스펙 가중치:**

| 필드 | 가중치 |
|-------|--------|
| vCPU | 40% |
| memoryGiB | 40% |
| Architecture | 20% |

**GPU 스펙 가중치:**

| 필드 | 가중치 |
|-------|--------|
| vCPU | 15% |
| memoryGiB | 15% |
| AcceleratorCount | 35% |
| AcceleratorMemoryGB | 25% |
| Architecture | 10% |

### 응답 (Response)

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

### 대체 구성에서의 이미지 선택

각 후보 스펙에 대해 `recommendAlternativeNodeConfig`은 `matchedSpecId` + 프로바이더/리전 필터(모든 CSP별 호환성 검사를 적용함)를 사용하여 호환 가능한 이미지를 자동으로 검색합니다:

| 소스 스펙 유형 | 기본 이미지(Primary Image) 선택 기준 |
|------------------|------------------------|
| GPU | `isBasicGpuImage = true` 우선, 그 다음 `isGPUImage = true` |
| 비-GPU (Non-GPU) | `isBasicImage = true` 우선 |

`alternativeImages`에는 기본 이미지 다음으로 순위가 매겨진 최대 `imageAlternativeLimit`개의 추가 옵션이 포함됩니다.

### 주요 파라미터

| 파라미터 | 기본값 | 설명 |
|-----------|---------|-------------|
| `tolerancePercent` | 20 | `preferred` 필드에 적용되는 ±% 허용 범위 |
| `specCandidateLimit` | 5 | 반환할 최대 후보 스펙 수 |
| `imageAlternativeLimit` | 3 | 후보당 최대 대체 이미지 수 |
| `osType` | (소스 이미지에서 상속) | OS 유형별 이미지 필터링 |

---

## 워크플로 예제

### 시나리오: 가용성 및 호환성이 보장된 GPU 워크로드 프로비저닝 또는 마이그레이션

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

### 결과 해석

| `specDiff` 필드 | 양수 값 (+) | 음수 값 (-) |
|------------------|----------------|----------------|
| `vCPUDiff` | 대상 스펙의 vCPU가 더 많음 | vCPU가 더 적음 |
| `memoryGiBDiff` | 대상 스펙의 RAM이 더 많음 | RAM이 더 적음 |
| `costPerHourDiff` | 비용이 더 비쌈 | 비용이 더 저렴함 |
| `accelCountDiff` | GPU 개수가 더 많음 | GPU 개수가 더 적음 |
| `accelMemGBDiff` | GPU 메모리가 더 많음 | GPU 메모리가 더 적음 |
| `architectureMatch` | `true` = 동일한 아키텍처 | `false` = 다른 아키텍처 |
