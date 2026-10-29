# 멀티 클라우드 Object Storage 통합 운용·관리 및 Presigned URL 기반 CRUD 시험 리포트

## 0. 결과 요약

**지원되는 10개 CSP (Alibaba, AWS, Azure, GCP, IBM, KT, NCP, NHN, OpenStack, Tencent) 전체에서 MapUI 기본 설정 기준으로 버킷 생성 (`PUT /ns/default/resources/objectStorage`) → 존재 확인·상세 조회·리전 조회·동기화 (`HEAD`, `GET`, `GET /location`, `PUT /reconcile`) → CORS 규칙 설정 및 조회 (`PUT / GET /cors`) → 객체 버전 관리 설정 및 조회 (`PUT / GET /versioning`) → 5단계 Presigned URL 기반 객체(파일) 전체 CRUD 검증 → 버킷 삭제 (`DELETE /ns/default/resources/objectStorage/{osId}`) 전 단계 정상 완료.**

| 지표 | 값 |
|---|---|
| 버킷 생성 (요청 → 10/10 `Available` 및 고유 CSP 버킷명 할당) | 0.44 ~ 7.45초 (IBM 제외 시 0.44 ~ 3.62초) |
| 메타데이터·리전 확인 및 동기화 (`HEAD` + `GET` + `GET /location` + `PUT /reconcile`) | 0.10 ~ 6.51초 |
| CORS 설정 및 조회 (`PUT /cors` + `GET /cors`, 지원 7개 CSP) | 1.45 ~ 11.14초 (IBM 제외 시 / IBM 글로벌 전파 포함 시 37.38초) |
| 객체 버전 관리 활성화 및 조회 (`PUT /versioning` + `GET /versioning`, 지원 6개 CSP) | 0.45 ~ 6.50초 |
| 5단계 Presigned URL 기반 객체 CRUD 검증 (업로드 URL → `PUT` v1/v2 → 목록·`HEAD` → 다운로드·검증 → 객체·버전 삭제) | CSP별 0.48 ~ 28.62초 (IBM 제외 시 0.48 ~ 6.27초) |
| 버킷 삭제 (`DELETE /ns/default/resources/objectStorage/{osId}`) | 1.65 ~ 12.67초 |
| 10개 CSP 전체 병렬 수행 총 소요 시간 (Wall-Clock) | **107.7초 (1분 48초)** |
| 최종 잔여 리소스 (`objectStorage`) | **0건** |

---

## 1. 시험 환경

| 항목 | 내용 |
|---|---|
| 일시 | 2026-10-26 UTC (멀티 클라우드 Object Storage 10개 CSP 전 주기 벤치마크) |
| CB-Tumblebug | `aea9af49` + `ListDataObjects` S3 캐시 즉시 무효화 반영, kind 클러스터 배포 |
| CB-Spider | `ee952524` + NCP/NHN S3 엔드포인트 소문자 정규화(`strings.ToLower(regionID)`, RFC 3986 호스트 서명 호환) 반영 |
| CB-MapUI | `🪣 Object Storage Management` (단일·멀티 클라우드 병렬 버킷 생성, `⚡ Background Operations` 실시간 타이머, 5단계 Presigned URL CRUD 원클릭 검증) |
| 대상 CSP (10종) | Alibaba, AWS, Azure, GCP, IBM, KT, NCP, NHN, OpenStack, Tencent |

| CSP | 커넥션 / 리전 (`LocationConstraint`) | 프로토콜 / 백엔드 | CORS API 지원 | Versioning API 지원 | Presigned URL 지원 | 필수 업로드 헤더 (`requiredHeaders`) |
|---|---|---|---|---|---|---|
| **Alibaba** | `alibaba-ap-northeast-2` (`ap-northeast-2`) | Alibaba Cloud OSS (S3 호환) | ✅ | ✅ (`Enabled` / `Suspended`) | ✅ (S3 SigV4) | 없음 |
| **AWS** | `aws-ap-northeast-2` (`ap-northeast-2`) | Amazon S3 | ✅ | ✅ (`Enabled` / `Suspended`) | ✅ (S3 SigV4) | 없음 |
| **Azure** | `azure-koreacentral` (`koreacentral`) | Azure Blob Storage (Native SDK + SAS) | ⚪ (Storage Account 단위) | ⚪ (Storage Account 단위) | ✅ (Blob SAS URL) | `x-ms-blob-type: BlockBlob` |
| **GCP** | `gcp-asia-northeast3` (`asia-northeast3`) | Google Cloud Storage (Native SDK + S3 Interop) | ✅ (GCS SDK) | ✅ (GCS SDK Generation) | ✅ (S3 HMAC SigV4) | 없음 |
| **IBM** | `ibm-us-south` (`us-south`) | IBM Cloud Object Storage (S3 호환) | ✅ | ✅ (`Enabled` / `Suspended`) | ✅ (S3 SigV4) | 없음 |
| **KT** | `kt-kr1` (`KR1`) | KT Cloud Object Storage (S3 호환) | ✅ | ✅ (`Enabled` / `Suspended`) | ✅ (S3 SigV4) | 없음 |
| **NCP** | `ncp-kr` (`KR`) | Naver Cloud Object Storage (S3 호환) | ⚪ (콘솔/전용 설정) | ⚪ (미지원) | ✅ (S3 SigV4) | 없음 |
| **NHN** | `nhn-kr1` (`KR1`) | NHN Cloud Object Storage (S3 호환) | ⚪ (콘솔/컨테이너 헤더) | ⚪ (미지원) | ✅ (S3 SigV4) | 없음 |
| **OpenStack** | `openstack-regionone` (`RegionOne`) | Ceph RGW / Swift S3 인터페이스 | ✅ | ⚪ (미지원) | ✅ (S3 SigV4) | 없음 |
| **Tencent** | `tencent-ap-seoul` (`ap-seoul`) | Tencent Cloud COS (`<bucket>-<AppId>`) | ✅ | ✅ (`Enabled` / `Suspended`) | ✅ (S3 SigV4) | 없음 |

---

## 2. 시험 절차

| 순서 | 단계 | 요청 API | 완료 판정 기준 |
|---|---|---|---|
| 1 | 버킷 생성 | `PUT /ns/default/resources/objectStorage` (`bucketName`, `connectionName`, `description`) | 10개 CSP 모두 `200 OK`, `status: "Available"` 및 전역 고유 `cspResourceName`(`uid`) 생성 확인 |
| 2 | 존재·상세·리전 확인 및 동기화 | `HEAD .../objectStorage/{osId}`, `GET .../objectStorage/{osId}`, `GET .../location`, `PUT .../reconcile` | `HEAD 200`, 상세 조회 `200`, `locationConstraint` 반환, `reconcile` 후 `Synced=True` 확인 |
| 3a | CORS 규칙 설정 및 조회 | `PUT .../objectStorage/{osId}/cors` (`allowedOrigin: ["*"]`, `allowedMethod: ["GET","PUT","POST","DELETE","HEAD"]`) → `GET .../cors` | CORS API 지원 7개 CSP 전체에서 설정(`200/204`) 및 규칙 조회(`200 OK`) 일치 확인 |
| 3b | 객체 버전 관리 설정 및 조회 | `PUT .../objectStorage/{osId}/versioning` (`{"status": "Enabled"}`) → `GET .../versioning` | Versioning API 지원 6개 CSP 전체에서 `status: "Enabled"` 전환 확인 |
| 4 | 5단계 Presigned URL 객체 CRUD | `[4-1]` 업로드 Presigned URL 발급 (`POST .../object/{key}/presignedUrl?operation=upload`) → `[4-2]` CSP 직접 바이너리 `PUT` (v1 업로드 및 v2 덮어쓰기) → `[4-3]` 객체 목록 조회 (`GET .../object`) 및 메타데이터 확인 (`HEAD .../object/{key}`) → `[4-4]` 다운로드 Presigned URL 발급 (`POST ...?operation=download`) 및 콘텐츠 무결성(`updated-verified`) 검증 → `[4-5]` 객체 및 전체 버전/삭제 마커 삭제 (`DELETE .../object/{key}`, `DELETE .../versions/{key}?versionId=...`) | 10개 CSP 전체에서 Tumblebug 서버를 거치지 않는 클라이언트-CSP 직접 업로드/다운로드, 목록·ETag 확인, 콘텐츠 검증 및 버전 정리 100% 성공 |
| 5 | 버킷 삭제 및 정리 검증 | `DELETE /ns/default/resources/objectStorage/{osId}` (필요 시 `?option=force`) → `GET /ns/default/resources/objectStorage` | CSP 실제 버킷 삭제 및 Tumblebug 메타데이터 제거 완료, 최종 잔여 버킷 `0`건 확인 |

---

## 3. 결과 매트릭스

| CSP | 1단계: 버킷 생성 | 2단계: 상세·리전·Reconcile | 3a단계: CORS 설정·조회 | 3b단계: Versioning 활성화 | 4a단계: Presigned URL 객체 CRUD (CLI / SDK 클라이언트) | 4b단계: Presigned URL 객체 CRUD (MapUI 웹 브라우저 직접 XHR) | 5단계: 버킷 삭제 | 종합 |
|---|---|---|---|---|---|---|---|---|
| **Alibaba** (`ap-northeast-2`) | ✅ | ✅ (`ap-northeast-2`) | ✅ | ✅ (`Enabled`) | ✅ (v1/v2 PUT · List · HEAD · GET · 버전 삭제) | ✅ (브라우저 직접 PUT · List · HEAD · GET · DEL) | ✅ | ✅ |
| **AWS** (`ap-northeast-2`) | ✅ | ✅ (`ap-northeast-2`) | ✅ | ✅ (`Enabled`) | ✅ (v1/v2 PUT · List · HEAD · GET · 버전 삭제) | ✅ (브라우저 직접 PUT · List · HEAD · GET · DEL) | ✅ | ✅ |
| **Azure** (`koreacentral`) | ✅ | ✅ (`koreacentral`) | ➖ (Account 단위) | ➖ (Account 단위) | ✅ (SAS PUT `BlockBlob` · List · HEAD · GET · 삭제) | ⚪ (컨트롤 플레인 `UpURL·DlURL·List` ✅ / 직접 PUT은 콘솔 CORS 설정 필요) | ✅ | ✅ |
| **GCP** (`asia-northeast3`) | ✅ | ✅ (`asia-northeast3`) | ✅ | ✅ (`Enabled`) | ✅ (v1/v2 PUT · List · HEAD · GET · 버전 삭제) | ✅ (브라우저 직접 PUT · List · HEAD · GET · DEL) | ✅ | ✅ |
| **IBM** (`us-south`) | ✅ | ✅ (`us-south`) | ✅ | ✅ (`Enabled`) | ✅ (v1/v2 PUT · List · HEAD · GET · 버전 삭제) | ✅ (브라우저 직접 PUT · List · HEAD · GET · DEL) | ✅ | ✅ |
| **KT** (`kr1`) | ✅ | ✅ (`KR1`) | ✅ | ✅ (`Enabled`) | ✅ (v1/v2 PUT · List · HEAD · GET · 버전 삭제) | ✅ (브라우저 직접 PUT · List · HEAD · GET · DEL) | ✅ | ✅ |
| **NCP** (`kr`) | ✅ | ✅ (`KR`) | ➖ (API 미지원) | ➖ (API 미지원) | ✅ (v1/v2 PUT · List · HEAD · GET · 삭제) | ⚪ (컨트롤 플레인 `UpURL·DlURL·List` ✅ / 직접 PUT은 콘솔 CORS 설정 필요) | ✅ | ✅ |
| **NHN** (`kr1`) | ✅ | ✅ (`KR1`) | ➖ (API 미지원) | ➖ (API 미지원) | ✅ (v1/v2 PUT · List · HEAD · GET · 삭제) | ⚪ (컨트롤 플레인 `UpURL·DlURL·List` ✅ / 직접 PUT은 콘솔 CORS 설정 필요) | ✅ | ✅ |
| **OpenStack** (`regionone`) | ✅ | ✅ (`RegionOne`) | ✅ | ➖ (API 미지원) | ✅ (v1/v2 PUT · List · HEAD · GET · 삭제) | ✅ (`http://` 접속 시 직접 PUT 통과 / `https://` 접속 시 Mixed Content로 컨트롤 플레인 검증) | ✅ | ✅ |
| **Tencent** (`ap-seoul`) | ✅ | ✅ (`ap-seoul`) | ✅ | ✅ (`Enabled`) | ✅ (v1/v2 PUT · List · HEAD · GET · 버전 삭제) | ✅ (브라우저 직접 PUT · List · HEAD · GET · DEL) | ✅ | ✅ |
| **합계 (10종)** | **10/10** | **10/10** | **7/7 (지원 전체)** | **6/6 (지원 전체)** | **10/10 (전체 통과)** | **7/7 (CORS API 지원 전체) + 3/3 (컨트롤 플레인 검증)** | **10/10 (잔여 `0`)** | **통과** |

> **CLI/SDK(`4a단계`) vs. MapUI 웹 브라우저(`4b단계`) 차이 설명**:
> * **CLI / 백엔드 SDK / `curl` (`4a단계`)**: 웹 브라우저가 아니므로 CORS Preflight(`OPTIONS`)나 Mixed Content 제약이 없으며, Tumblebug가 발급한 Presigned URL로 **10개 CSP 전체에서 실제 파일 업로드(`PUT`)·다운로드(`GET`)가 100% 성공**합니다.
> * **웹 브라우저 내부 JavaScript XHR (`4b단계`)**: 브라우저는 다른 도메인으로 `PUT` 전송 전 버킷의 CORS 허용 헤더(`Access-Control-Allow-Origin`)를 먼저 검사합니다. S3 CORS API를 지원하는 7개 CSP는 자동 설정된 CORS 규칙으로 브라우저 직접 업로드가 즉시 통과하며, S3 CORS API를 제공하지 않는 3개 CSP(`Azure`, `NCP`, `NHN`)는 각 CSP 웹 콘솔에서 CORS를 허용하기 전까지 브라우저 XHR 직접 `PUT`만 제한됩니다(단, 브라우저 `<a download>` 파일 다운로드 및 컨트롤 플레인 `UpURL·DlURL·List` 검증은 정상 동작).

---

## 4. 단계별 수치

| 단계 | 10개 CSP 병렬 소요 범위 | 비고 |
|---|---|---|
| 1단계: 빈 버킷(저장소) 리소스 생성 (`PUT /objectStorage`) | **0.44 ~ 7.45초** | NHN(`0.44초`), Azure(`0.74초`), NCP(`0.87초`) 최단 · IBM(`7.45초`) |
| 2단계: 존재·상세·리전 조회 및 Reconcile | **0.10 ~ 6.51초** | `HEAD` + `GET` + `GET /location` + `PUT /reconcile` 4종 API 연속 호출 합산 |
| 3a단계: CORS 규칙 설정 및 조회 (`PUT / GET /cors`) | **1.45 ~ 37.38초** | GCP(`1.45초`) 최단 · S3 표준 CSP(`8.97 ~ 11.14초`) · IBM 글로벌 전파(`37.38 ~ 68.92초`) |
| 3b단계: 객체 버전 관리 활성화 (`PUT / GET /versioning`) | **0.45 ~ 6.50초** | Alibaba(`0.45초`), KT(`0.98초`), AWS(`1.60초`), GCP(`1.77초`), Tencent(`2.63초`), IBM(`6.50초`) |
| 4단계: 5단계 Presigned URL 실제 파일(객체) CRUD 합계 | **0.48 ~ 28.62초** | 파일 업로드(`PUT`) + 목록·`HEAD` 조회 + 파일 다운로드(`GET`)·검증 + 파일 삭제 합산 |
| 5단계: 빈 버킷 리소스 삭제 (`DELETE /objectStorage/{osId}`) | **1.65 ~ 12.67초** | CSP 버킷 삭제 후 존재 확인 게이트(`PollResourceDeletedViaSpider`) 완료까지 포함 |
| **전체 병렬 실행 총 소요 시간 (Wall-Clock)** | **107.7초 (1분 48초)** | 10개 CSP 동시 시작 기준 전체 생성 → 설정 → 객체 CRUD → 삭제 완료 |

### CSP별 상세 소요 시간 (초)

| CSP | 버킷 생성 (초) | 메타·리전·Reconcile (초) | CORS 설정·조회 (초) | Versioning 활성화 (초) | Presigned 업로드 v1+v2 (초) | 목록 + `HEAD` 조회 (초) | Presigned 다운로드·검증 (초) | 객체·버전 삭제 (초) | 5단계 객체 CRUD 합계 (초) | 버킷 삭제 (초) |
|---|---|---|---|---|---|---|---|---|---|---|
| **NHN** (`nhn-kr1`) | **0.44** | **0.10** | *(미지원)* | *(미지원)* | 0.24 | 0.13 | **0.05** | **0.06** | **0.48** | **1.65** |
| **Azure** (`azure-koreacentral`) | 0.74 | 0.46 | *(Account 단위)* | *(Account 단위)* | **0.02** | 0.35 | 0.46 | 0.12 | 0.96 | 3.47 |
| **NCP** (`ncp-kr`) | 0.87 | 0.12 | *(미지원)* | *(미지원)* | 0.59 | 0.76 | 0.47 | 0.10 | 1.92 | 3.73 |
| **OpenStack** (`openstack-regionone`) | 1.34 | 0.45 | 8.97 | *(미지원)* | 0.63 | 1.08 | 0.40 | 0.38 | 2.48 | 4.32 |
| **Alibaba** (`alibaba-ap-northeast-2`) | 2.24 | 0.84 | 9.53 | **0.45** | 0.16 | 0.41 | 0.07 | 0.24 (3버전) | 0.89 | 4.45 |
| **KT** (`kt-kr1`) | 2.34 | 1.76 | 10.04 | 0.98 | 0.78 | 1.46 | 0.97 | 3.02 (3버전) | 6.24 | 3.59 |
| **Tencent** (`tencent-ap-seoul`) | 2.91 | 1.24 | 11.14 | 2.63 | 0.33 | 2.62 | 0.23 | 3.09 (3버전) | 6.27 | 9.00 |
| **AWS** (`aws-ap-northeast-2`) | 3.32 | 0.77 | 11.07 | 1.60 | 0.73 | 1.06 | 0.35 | 2.38 (3버전) | 4.51 | 2.30 |
| **GCP** (`gcp-asia-northeast3`) | 3.62 | 1.24 | **1.45** | 1.77 | 1.12 | 2.05 | 0.11 | 2.01 (버전 포함) | 5.28 | 3.67 |
| **IBM** (`ibm-us-south`)* | 7.45 | 6.51 | 37.38* | 6.50 | 5.33 | 5.58 | 3.56 | 14.15 (3버전) | 28.62 | 12.67 |

> \* **IBM Cloud Object Storage 참고**: IBM Cloud(`ibm-us-south`)는 북미(달라스) 리전 엔드포인트(`s3.us-south.cloud-object-storage.appdomain.cloud`)와의 왕복 지연 및 버킷 CORS 정책 글로벌 전파 대기로 인해 CORS 설정에 `37.4 ~ 68.9초`, 버전 관리 활성화 후 다중 버전 객체 정리에 `14.15초`가 소요됩니다.

---

## 5. 정보 정확성 및 E2E 데이터 플레인 검증

10개 CSP 전 버킷에서 다음 검사를 수행하여 모두 통과함을 확인함.

| 검사 항목 | 결과 |
|---|---|
| 버킷 상세 뷰 필수 필드 (`id`, `name`, `uid`, `cspResourceName`, `cspResourceId`, `connectionName`, `status`, `conditions`) | `Available` 도달 시 누락·불일치 0건 (Tencent는 `cspResourceId`에 `<uid>-<AppId>` 정상 기록) |
| 버킷 위치 조회 (`GET /ns/default/resources/objectStorage/{osId}/location`) | 10개 CSP 전체에서 실제 리전 제약(`ap-northeast-2`, `koreacentral`, `asia-northeast3`, `KR`, `KR1`, `RegionOne`, `ap-seoul`, `us-south`) 정확히 반환 |
| 동기화 및 고아 정리 (`PUT .../reconcile`, `POST .../reconcile/prune`) | CSP 실시간 버킷 존재 여부와 Tumblebug 상태(`Synced=True`) 100% 일치 |
| Presigned URL 직접 업로드·다운로드 (`POST .../presignedUrl`) | 10개 CSP 전체에서 Tumblebug 서버 부하 없이 클라이언트가 CSP 스토리지 엔드포인트로 직접 `PUT` / `GET` 수행 및 내용(`updated-verified`) 일치 확인 |
| 업로드 직후 실시간 객체 목록 반영 (`GET .../object`) | `ListDataObjects` 호출 시 내부 Spider S3 GET 캐시를 즉시 무효화하여 Presigned URL 외부 업로드 직후에도 지연 없이 최신 객체 목록 반환 |
| 최종 전체 정리 (`DELETE /ns/default/resources/objectStorage/{osId}`) | 10개 CSP 모두 버킷 완전 삭제 및 `ns/default` 내 잔여 `objectStorage` 0건 확인 |

---

## 6. CSP별 참고 사항 및 개선 내역 (Backend & MapUI UX 고도화)

| 구분 / 특성 | 대상 CSP | 원인 분석 및 CB-Spider / CB-Tumblebug / CB-MapUI 처리 방식 |
|---|---|---|
| **Presigned URL 호스트명 대소문자 정규화 (`SignatureDoesNotMatch` 근본 해결)** | **NCP, NHN** | NCP(`KR`)와 NHN(`KR1`)은 리전 식별자가 대문자로 저장되어 기존에 `KR.object.ncloudstorage.com`, `KR1-api-object-storage.nhncloudservice.com` 형태로 Presigned URL이 서명됨. 그러나 웹 브라우저(`fetch`/`axios`) 및 표준 HTTP 클라이언트(`curl`, `requests`)는 RFC 3986에 따라 URL 호스트명을 소문자(`kr...`, `kr1...`)로 정규화하여 전송하므로 S3 SigV4 `Host` 헤더 서명 불일치(`403 SignatureDoesNotMatch`)가 발생했음. CB-Spider `GetS3ConnectionInfo`([`S3Manager.go`](file:///home/shson/go/src/github.com/cloud-barista/cb-spider/api-runtime/common-runtime/S3Manager.go#L241-L254))에서 엔드포인트 생성 시 `strings.ToLower(regionID)`를 적용하여 추가 헤더 없이 100% 정상 동작하도록 해결함. |
| **GCP Storage SDK CORS Preflight (`ResponseHeaders`) 호환성 보강** | **GCP** | GCP Cloud Storage SDK의 `storage.CORS` 구조체는 `ResponseHeaders` 필드를 브라우저 `OPTIONS` Preflight의 `Access-Control-Allow-Headers` 겸용으로 사용하며, CB-Spider의 `setGCPBucketCORS`는 `exposeHeader`를 `ResponseHeaders`에 매핑함. MapUI `enableDefaultCors()`의 `exposeHeader`에 `'Content-Type'`을 추가하여 브라우저 직접 `PUT` 업로드 시 GCS Preflight가 즉시 통과하도록 개선함. |
| **S3 CORS 미지원 CSP의 브라우저 XHR 제약 및 컨트롤 플레인 검증** | **Azure, NCP, NHN** | Azure(Storage Account 단위 CORS 설정), NHN(OpenStack Swift 컨테이너 메타데이터 헤더/콘솔 방식), NCP(콘솔 설정 방식)는 표준 S3 `PutBucketCors` API를 제공하지 않음(`cors: false`). 따라서 CLI/SDK 및 브라우저 `<a download>` 다운로드는 즉시 동작하지만, 웹 브라우저 내부 JS(`axios.put`) 직접 업로드는 CSP 콘솔에서 CORS를 허용하기 전까지 브라우저가 차단함. MapUI E2E 테스트에서는 해당 CSP들에 대해 보안상 취약한 프록시를 두지 않고 컨트롤 플레인 3종(`UpURL✓ · DlURL✓ · List✓`)을 검증한 뒤 `PASS (Control-Plane)`으로 명확히 구분 표시하도록 구현함. |
| **Azure Blob SAS 업로드 필수 헤더 자동 병합** | **Azure** | Azure Blob Storage는 SAS URL을 통한 `PUT` 업로드 시 `x-ms-blob-type: BlockBlob` 헤더가 필수임. Tumblebug가 반환하는 `requiredHeaders` 맵을 MapUI의 `uploadFileToBucket()` 및 `runE2eObjectCrudBenchmark()`에서 요청 헤더에 자동 병합하도록 개선함. |
| **Presigned URL 외부 업로드 직후 목록 캐시 일관성** | **전체 (특히 고속 CSP)** | Presigned URL 업로드는 Tumblebug를 경유하지 않고 CSP로 직접 전송되므로, 업로드 직후 2초(`ShortDuration`) 이내에 `GET .../object` 호출 시 이전 빈 목록 캐시가 반환될 수 있었음. Tumblebug `ListDataObjects()`에서 Spider S3 GET 캐시를 즉시 무효화하도록 개선함. |
| **Tencent COS 고유 버킷명(`-<AppId>`) 처리** | **Tencent** | Tencent COS는 CSP 실제 버킷명에 계정 `AppId` 접미사(`tbi0bd0kcsop2lsms2gk-1383606637`)를 자동 부여함. CB-Spider가 CAM API를 통해 `AppId`를 자동 조회·매핑하며, MapUI에서도 Tumblebug ID와 실제 CSP 버킷명을 함께 표시하고 복사 버튼을 제공함. |
| **MapUI `🪣 Object Storage Management` UI/UX 전면 개편** | **전체 10개 CSP** | `🗄️ RDBMS Management`의 UX 패턴을 반영하여 ① **CSP별 필터 칩 & 기능·소요시간 안내 박스**(`Est. Create`, `Est. Delete`, `Est. 5-Step CRUD`), ② **단일/멀티 클라우드 병렬 버킷 생성 드로어**(다중 CSP 선택 시 `{baseName}-{csp}` 자동 명명, Auto-CORS 및 Auto-Versioning 옵션), ③ **`⚡ Background Operations` 실시간 진행 패널**(경과 타이머 및 단계별 상태 카드), ④ **원클릭 5단계 Presigned URL CRUD 검증 버튼(`Test CRUD` / `Batch E2E Test`)**, ⑤ **버전 관리(`Versioning`) 조회·토글·버전 목록 모달**, ⑥ **Reconcile All / Prune Orphans / Empty Bucket(`?option=empty`) / Force Delete(`?option=force`)** 기능을 통합 제공함. |

