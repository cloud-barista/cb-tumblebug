# 멀티 클라우드 관리형 RDBMS 통합 운용·관리 및 Bastion 기반 CRUD 시험 리포트

## 0. 결과 요약

**지원되는 9개 CSP (Alibaba, AWS, Azure, GCP, IBM, NCP, NHN, OpenStack, Tencent) 전체에서 MapUI 기본 설정 (`dbEngine: "mysql"`, `autoFillDefaults: true`, In-VPC Bastion Set 자동 구성) 기준으로 Bastion 인프라 생성 → 네트워크 해석 및 사전 검증 → RDBMS 인스턴스 생성 → 조회·동기화(Reconcile) → 5단계 DB·테이블 전체 CRUD 검증 → RDBMS 삭제 → Bastion 종료·삭제 → 공유 리소스 해제(`DELETE /sharedResources`) 전 단계 정상 완료.**

| 지표 | 값 |
|---|---|
| Bastion VM 인프라 생성 (요청 → 9/9 Running 및 `mysql-client` 설치 완료) | 50.8 ~ 220.1초 (병렬 실행 시 220.1초) |
| 관리형 RDBMS 생성 (요청 → 9/9 `Available` 및 엔드포인트 활성화) | 112.4 ~ 661.8초 (IBM 제외 시 / IBM 포함 시 1,072.2초 · 17분 52초) |
| 5단계 DB & 테이블 CRUD 검증 (Control Plane DB API + Bastion 경유 Data Plane SQL) | CSP별 1.63 ~ 37.93초 |
| 관리형 RDBMS 삭제 (요청 → CSP 클러스터 삭제 및 네트워크 안정화 대기 완료) | 22.3 ~ 500.2초 (Alibaba ENI 해제 대기 제외 시 22.3 ~ 256.6초) |
| Bastion 종료·삭제 + 공유 리소스 일괄 해제 (`DELETE /sharedResources`) | 인프라당 3.2 ~ 67.7초 + 공유 네트워크 해제 12.8 ~ 56.4초 |
| 최종 잔여 리소스 (`rdbms`, `infra`, `vNet`, `securityGroup`, `sshKey`) | 0 / 0 / 0 / 0 / 0 |

<img width="973" height="1260" alt="image" src="https://github.com/user-attachments/assets/fd142b88-151b-441d-a0ef-568680f1d7e7" />

---

<img width="936" height="1106" alt="image" src="https://github.com/user-attachments/assets/1dd98b7e-d697-40cf-a8c2-16886b814b47" />


---

<img width="930" height="911" alt="image" src="https://github.com/user-attachments/assets/b3d3adcd-4b25-48b7-bacc-93317ec03de3" />

---

## 1. 시험 환경

| 항목 | 내용 |
|---|---|
| 일시 | 2026-10-26 UTC (멀티 클라우드 RDBMS + Bastion Set 전 주기 벤치마크) |
| CB-Tumblebug | `aea9af49`, kind 클러스터 배포 |
| CB-Spider | `ee952524` (OpenStack Trove 게스트 에이전트 초기화 완료 검증 반영) |
| CB-MapUI | `RDBMS Management (Relational DB)` 기본 Bastion Set 배포 모드 (`mysql`, `autoFillDefaults: true`, `publicAccess: false` 기본 및 CSP별 자동 적응) |
| 대상 CSP (9종) | Alibaba, AWS, Azure, GCP, IBM, NCP, NHN, OpenStack, Tencent |

| CSP | 커넥션 / 리전 | 자동 결정된 엔진·버전 | 스펙 | 스토리지 | 유효 접속 모드 (`publicAccess`) | 유효 관리자 계정 |
|---|---|---|---|---|---|---|
| Alibaba | `alibaba-ap-northeast-2` (`ap-northeast-2`) | MySQL `8.0` | `mysql.n4.large.1` | `20 GB` (`general_essd`) | Private VPC (`false`) | `dbadmin` |
| AWS | `aws-ap-northeast-2` (`ap-northeast-2`) | MySQL `8.0.45` | `db.t3.micro` | `20 GB` (`gp2`) | Private VPC (`false`) | `dbadmin` |
| Azure | `azure-koreacentral` (`koreacentral`) | MySQL `8.0.21` | `Standard_B1ms` | `32 GB` (`Premium_LRS`) | Private VPC (`false`, 전용 빈 서브넷 #2) | `dbadmin` |
| GCP | `gcp-asia-northeast3` (`asia-northeast3`) | MySQL `MYSQL_8_0` | `db-n1-standard-1` | `32 GB` (`SSD`) | Public Endpoint (`true`, 자동 적응) | `dbadmin` |
| IBM | `ibm-us-south` (`us-south`) | MySQL `8.4` | `multitenant` (3멤버) | `32 GB` | Public Endpoint (`true`, 필수) | `admin` (고정) |
| NCP | `ncp-kr` (`kr`) | MySQL `8.0.36` | `SVR.VDBAS.STAND.C002.M008.G005` | `10 GB` (`SSD`) | Private VPC (`false`, 고정) | `dbadmin` |
| NHN | `nhn-kr2` (`kr2`) | MySQL `MYSQL_V848` (`8.4.8`) | `m2.c2m4` | `20 GB` (`GENERAL_SSD`) | Private VPC (`false`) | `dbadmin` |
| OpenStack | `openstack-region01` (`regionone`) | MySQL `8.4` | `m1.small` | `20 GB` | Private VPC (`false`) | `dbadmin` |
| Tencent | `tencent-ap-seoul` (`ap-seoul`) | MySQL `8.0` | `8000` (1000 MB RAM) | `100 GB` (`CLOUD_SSD`) | Private VPC (`false`) | `root` (고정) |

## 2. 시험 절차

| 순서 | 단계 | 요청 | 완료 판정 |
|---|---|---|---|
| 1 | 전용 Bastion 인프라 생성 | `POST /ns/default/infraDynamic` (CSP별 전용 1노드 Infra + `postCommands`로 `mysql-client`, `netcat` 자동 설치) | 모든 Bastion VM `Running` 및 원격 명령 실행 준비 완료 |
| 2 | 네트워크 해석 및 사전 검증 | `GET /ns/default/resources/vNet/{id}`, `POST .../securityGroup/{id}/rules` (포트 `3306`), `POST /ns/default/resources/rdbms/validate` | 서브넷·보안그룹 매핑 및 Dry-run 검증 `200 OK` |
| 3 | 관리형 RDBMS 생성 | `POST /ns/default/resources/rdbms` (`autoFillDefaults: true`) | 모든 인스턴스가 `Available` 상태 및 유효 `endpoint:port` 확보 |
| 4 | 정보 조회 및 동기화 | `GET /ns/default/resources/rdbms`, `GET .../rdbms/{id}`, `PUT .../rdbms/{id}/reconcile`, `GET .../certificate` (IBM) | 필수 메타데이터 채움, 목록·상세·동기화 뷰 간 값 일치 |
| 5 | 5단계 DB & 테이블 CRUD | `[1]` DB 생성 (`POST .../database` 또는 SQL) → `[2]` DB 목록 조회 (`GET .../database` 또는 SQL) → `[3]` Bastion 경유 테이블 생성·INSERT → `[4]` UPDATE·SELECT·DELETE·DROP TABLE → `[5]` DB 삭제 (`DELETE .../database/{dbName}` 또는 SQL) | 5단계 전 과정 성공, 갱신 데이터(`updated-verified`) 확인 및 테스트 DB 정상 제거 |
| 6 | 관리형 RDBMS 삭제 | `DELETE /ns/default/resources/rdbms/{id}` | CSP 상의 DB 클러스터 완전 삭제 및 Tumblebug 레코드 제거 |
| 7 | Bastion 및 공유 리소스 정리 | `GET /ns/default/control/infra/{id}?action=terminate` → `DELETE /ns/default/infra/{id}` → `DELETE /ns/default/sharedResources` | RDBMS, Infra, vNet, SecurityGroup, SSHKey 잔여 리소스 `0`건 |

## 3. 결과 매트릭스

| CSP | 1단계: Bastion 생성 | 2단계: 네트워크·검증 | 3단계: RDBMS 생성 | 4단계: 조회·Reconcile | 5단계: 5단계 DB/테이블 CRUD | 6단계: RDBMS 삭제 | 7단계: 전체 정리·해제 | 종합 |
|---|---|---|---|---|---|---|---|---|
| Alibaba | ✅ | ✅ | ✅ | ✅ | ✅ (API + SQL) | ✅ | ✅ | ✅ |
| AWS | ✅ | ✅ | ✅ | ✅ | ✅ (Direct SQL) | ✅ | ✅ | ✅ |
| Azure | ✅ | ✅ | ✅ | ✅ | ✅ (API + SQL) | ✅ | ✅ | ✅ |
| GCP | ✅ | ✅ | ✅ | ✅ | ✅ (API + SQL) | ✅ | ✅ | ✅ |
| IBM | ✅ | ✅ | ✅ | ✅ (+ TLS 인증서) | ✅ (TLS SQL) | ✅ | ✅ | ✅ |
| NCP | ✅ | ✅ | ✅ | ✅ | ✅ (API + SQL) | ✅ | ✅ | ✅ |
| NHN | ✅ | ✅ | ✅ | ✅ | ✅ (API + SQL) | ✅ | ✅ | ✅ |
| OpenStack | ✅ | ✅ | ✅ | ✅ | ✅ (API + SQL) | ✅ | ✅ | ✅ |
| Tencent | ✅ | ✅ | ✅ | ✅ | ✅ (API + SQL) | ✅ | ✅ | ✅ |
| **합계 (9종)** | **9/9** | **9/9** | **9/9** | **9/9** | **9/9** | **9/9** | **9/9 (잔여 `0`)** | **통과** |

## 4. 단계별 수치

| 단계 | 병렬 전체 완료 시간 (9개 CSP) | CSP별 소요 범위 | 비고 |
|---|---|---|---|
| 1단계: Bastion VM 인프라 생성 | 220.1초 (~3분 40초) | 50.8 ~ 220.1초 | 공유 vNet·서브넷·SG·SSHKey 자동 생성 + `apt-get install mysql-client netcat-openbsd` 포함 |
| 2단계: 네트워크 해석 및 사전 검증 | 77.7초 (Alibaba 실시간 스펙 조회) / 3.1초 (기타) | 0.45 ~ 77.66초 | Bastion SG에 TCP `3306` 인바운드 규칙 추가 및 기본값 Dry-run 검증 |
| 3단계: 관리형 RDBMS 인스턴스 생성 | 661.8초 (~11분 02초, IBM 제외) / 1,072.2초 (17분 52초, IBM 포함) | 112.4 ~ 1,072.2초 | Alibaba 최단(`112.4초`), GCP(`179.6초`), NCP(`661.8초`), IBM 3노드 클러스터(`1,072.2초`) |
| 4단계: 정보 조회 및 Reconcile | 3.9초 | 0.31 ~ 3.85초 | CSP 실시간 상태 조회 및 Tumblebug 메타데이터 동기화 |
| 5단계: 5단계 DB & 테이블 CRUD | 37.9초 | 1.63 ~ 37.93초 | Control Plane DB 생성·목록·삭제 API + SSH Bastion 경유 SQL 실행 합산 |
| 6단계: 관리형 RDBMS 인스턴스 삭제 | 500.2초 (~8분 20초) | 22.3 ~ 500.2초 | Alibaba ENI 해제 대기 제외 시 `22.3 ~ 256.6초` (IBM 최단 `22.3초`, NCP `256.6초`) |
| 7단계: Bastion 종료·삭제 + 공유 리소스 해제 | 80.5초 | 3.2 ~ 67.7초 + 12.8 ~ 56.4초 | 모든 Bastion VM 종료·삭제 후 공유 vNet, SG, SSHKey 일괄 삭제 |

### CSP별 소요 시간 (초)

| CSP | Bastion 생성 (초) | 네트워크·검증 (초) | RDBMS 생성 (초) | 조회·Reconcile (초) | Control Plane DB 생성 / 삭제 (초) | Bastion SQL CRUD (초) | 5단계 CRUD 합계 (초) | RDBMS 삭제 (초) | Bastion 종료 + 삭제 (초) |
|---|---|---|---|---|---|---|---|---|---|
| **Alibaba** (`ap-northeast-2`) | **50.8** | 77.66*** | **112.4** (1분 52초) | 1.56 | *(SQL / API)* | 1.77 | 1.77 | 500.2 (8분 20초)* | **3.2** |
| **GCP** (`asia-northeast3`) | 156.0 | 0.62 | **179.6** (2분 59초) | 1.15 | 0.97 / 1.15 | 1.15 | 3.55 | 155.1 (2분 35초) | 56.1 |
| **Tencent** (`ap-seoul`) | 95.1 | 0.65 | **200.7** (3분 21초) | 0.87 | 2.14 / 1.49 | 1.81 | 6.13 | 144.3 (2분 24초) | 16.4 |
| **OpenStack** (`regionone`) | 220.1 | 1.24 | **260.0** (4분 20초) | 0.31 | 5.77 / 5.77 | 2.14 | 13.99 | 52.0 (52초) | 15.1 |
| **AWS** (`ap-northeast-2`) | 144.3 | 0.45 | **296.6** (4분 57초) | 0.42 | *(Direct SQL)* | 1.63 | 1.63 | 135.8 (2분 16초) | 25.6 |
| **Azure** (`koreacentral`) | 105.2 | 1.52 | **442.3** (7분 22초) | 0.87 | 18.47 / 15.92 | 2.28 | 37.93 | 63.2 (1분 03초) | 40.3 |
| **NHN** (`kr2`) | 142.5 | 2.55 | **464.2** (7분 44초) | 1.88 | 15.53 / 16.46 | 2.02 | 35.67 | 64.1 (1분 04초) | 19.3 |
| **NCP** (`kr`) | 206.1 | 1.24 | **661.8** (11분 02초) | 0.69 | 7.22 / 7.21 | 2.15 | 17.27 | 256.6 (4분 17초) | 67.7 |
| **IBM** (`us-south`)** | 108.4 | 3.08 | **1,072.2** (17분 52초) | 3.85 | *(Direct SQL)* | 3.15 | 3.15 | **22.3** (22초) | 21.5 |

> \* **Alibaba RDBMS 삭제 시간 참고**: Alibaba Cloud RDS 인스턴스 자체는 약 13초 만에 삭제되나, 백엔드 VPC 컨트롤러가 RDS 전용 숨김 ENI를 완전히 반환할 때까지 CB-Spider가 최대 `8분 30초`(실측 `500.2초`) 안정화 대기를 수행합니다. 이를 통해 직후 공유 리소스 해제(`DELETE /sharedResources`) 시 VSwitch/VPC가 `DependencyViolation` 오류 없이 즉시 삭제됩니다.
> \*\* **IBM Cloud 참고**: IBM Cloud Databases for MySQL (`8.4`, `multitenant`, `32 GB`)은 기본 3멤버 다중 가용영역 클러스터로 프로비저닝되어 초기 생성 및 TLS 엔드포인트(`:32421`) 등록에 `17분 52초`(`1,072.2초`)가 소요되었으며, 삭제는 `22.3초` 만에 완료됩니다.
> \*\*\* **Alibaba 사전 검증(`validate`) 참고**: Alibaba Cloud의 `DescribeAvailableClasses` API가 리전 내 가용 스토리지(`general_essd`) 및 인스턴스 클래스(`mysql.n4.large.1`) 조합을 실시간 검증하는 데 약 77.7초가 소요됩니다.

## 5. 정보 정확성 및 E2E CRUD 검증

9개 CSP 전 인스턴스에서 다음 검사를 수행하여 모두 통과함을 확인함.

| 검사 항목 | 결과 |
|---|---|
| RDBMS 상세 뷰 필수 필드 (`id`, `name`, `status`, `connectionName`, `dbEngine`, `dbEngineVersion`, `dbInstanceSpec`, `storageType`, `storageSize`, `endpoint`, `port`, `publicAccess`) | `Available` 도달 시 공백·플레이스홀더 0건 |
| 상세 ↔ 목록 (`GET /ns/default/resources/rdbms`) | `status`, `endpoint`, `port`, `dbEngineVersion`, `publicAccess`, `vNetId` 100% 일치 |
| 동기화 API (`PUT /ns/default/resources/rdbms/{id}/reconcile`) | CSP 실시간 상태 및 엔드포인트가 드리프트 없이 동일하게 반영됨 |
| Control Plane DB 생성·조회·삭제 (`POST / GET / DELETE .../database`) | `cspNativeApi` 지원 7개 CSP (Alibaba, Azure, GCP, NCP, NHN, OpenStack, Tencent) 전체에서 무상태 헤더(`X-Admin-User-Name`, `X-Admin-User-Password`) 기반으로 정상 동작 |
| In-VPC Bastion 경유 Data Plane SQL (`POST /ns/default/cmd/infra/{id}`) | 9개 CSP 전체에서 TCP `3306` 경유 `CREATE TABLE` → `INSERT`(2건) → `UPDATE`(`updated-verified`) → `SELECT` → `DELETE` → `DROP TABLE` 검증 통과 |
| 최종 종료 및 공유 리소스 해제 (`DELETE /ns/default/sharedResources`) | `rdbms` 0건, `infra` 0건, `vNet` 0건, `securityGroup` 0건, `sshKey` 0건 확인 |

## 6. CSP별 참고 사항 (운영 및 MapUI 자동 적응)

| 특성 / 제약 사항 | 대상 CSP | 운영 동작 및 MapUI 처리 방식 |
|---|---|---|
| **네트워크 접근 정책 (`publicAccess`)** | **NCP** | VPC 내부 사설 접속(`publicAccess: false`)만 지원. MapUI에서 토글을 `OFF`로 잠그고 동일 VPC 내 Bastion VM을 통해 SQL 테스트를 수행함. |
| **네트워크 접근 정책 (`publicAccess`)** | **IBM, GCP** | IBM은 `service-endpoints: public` 필수, GCP Private IP는 별도 Service Networking VPC Peering(`servicenetworking.googleapis.com`) 사전 구성이 필요하여 멀티 클라우드 배포 시 `publicAccess: true`로 자동 전환함. |
| **네트워크 접근 정책 (`publicAccess`)** | **AWS, Azure, Alibaba, Tencent, NHN, OpenStack** | 사설 VPC(`false`, 기본값) 및 공인 접속(`true`) 모두 지원. 기본 사설 VPC 모드에서 동일 VNet의 Bastion VM을 통해 외부 노출 없이 안전하게 CRUD를 검증함. |
| **전용 빈 서브넷 위임 요구사항** | **Azure** | Azure MySQL Flexible Server는 VM NIC가 없는 전용 서브넷(`Microsoft.DBforMySQL/flexibleServers` 위임)을 요구하므로, MapUI가 공유 VNet의 두 번째 서브넷(`subnetIds[1]`)을 자동 선택함. |
| **고정 관리자 계정명** | **Tencent, IBM** | Tencent는 초기 슈퍼유저가 `root`, IBM은 `admin`으로 고정됨. MapUI가 CSP별 유효 계정명을 자동 변환하고 프리뷰에 표시함. |
| **비밀번호 복잡도 규칙 차이** | **IBM, AWS, Alibaba, NCP, Tencent** | IBM은 15-72자 영문·숫자만 허용(특수문자 및 `admin` 문자열 금지), AWS는 `@`, `/`, `"`, 공백 금지, NCP는 8~20자 및 특수문자 1개 이상 필수, Alibaba는 8-32자 3종 조합 필수. MapUI의 `getCspCompliantAdminPassword()`가 CSP별로 자동 보정함. |
| **데이터베이스 CRUD 방식 (`dbOperationMethod`)** | **AWS, IBM vs. 기타 7개 CSP** | AWS RDS와 IBM Cloud는 인스턴스 내부 논리 DB 생성을 위한 별도 REST API를 제공하지 않으므로(`sqlClientRequired`) Bastion SQL 클라이언트로 `CREATE/DROP DATABASE`를 수행하며, 나머지 7개 CSP는 Tumblebug Control Plane API(`/database`)를 사용함. |
| **Alibaba Control Plane DB 권한 자동 부여** | **Alibaba** | Alibaba RDS API로 신규 DB 생성 시 마스터 계정에 해당 DB의 DML 권한이 자동 부여되지 않는 특성이 있어, CB-Spider가 `CreateDatabase` 직후 `GrantAccountPrivilege`(`ReadWrite`)를 자동 호출하도록 개선함. |
| **OpenStack Trove 게스트 에이전트 준비 검증** | **OpenStack** | OpenStack Trove는 VM 부팅 직후 `ACTIVE`로 표시되나 내부 MySQL/MariaDB 초기화 및 계정 생성이 수십 초 뒤 완료되므로, CB-Spider가 `ListDatabases` 응답을 확인할 때까지 대기 후 반환하도록 개선함. |
| **삭제 후 네트워크 안정화 대기** | **Alibaba, Tencent 외** | DB 삭제 직후 VPC/서브넷 삭제 시 숨김 ENI 잔류로 인한 실패를 방지하기 위해 CB-Spider가 CSP별 안정화 대기(`Alibaba 8분 30초`, `Tencent 2분`, 기타 `10초`)를 수행함. |

