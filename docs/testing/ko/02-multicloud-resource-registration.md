# 멀티 클라우드 기존 자원·서비스 등록 관리 통합 시험 리포트

## 0. 결과 요약

**CSP 10종에서 기존 자원(VM·vNet·서브넷·SG·SSH 키)을 TB에 등록하고, 10종 전부에서 등록 노드와 vNet·서브넷·SG·SSH 키의 연관관계(28항목)가 일치함을 확인했으며, 등록 해제 후 TB에 잔존 레코드가 없음을 확인.**

| 지표 | 값 |
|---|---|
| 소스 VM 10대 생성 → TB·Spider 기록 제거(기존 자원화) | 127초 + 1초 |
| 등록 (`registerCspResources`, 커넥션별 순차) | 10/10 성공, 커넥션당 8~60초, 전체 260초 |
| 등록 노드 필드·연관관계 검증 | 10/10 CSP 28/28 통과, CSP당 1.2초 |
| 등록 해제 후 TB 등록 레코드 | vNet·SG·키·디스크·이미지 0, Infra 0 (6초) |

## 1. 시험 환경

| 항목 | 내용 |
|---|---|
| 일시 | 2026-10-13 09:41 ~ 10:11 UTC (정리 포함) |
| CB-Tumblebug | `6e7da269`, kind 클러스터 |
| CB-Spider | `fcee1108` |
| 대상 커넥션 | alibaba-ap-northeast-1, aws-ap-northeast-3, azure-koreasouth, gcp-asia-northeast1, ibm-us-south, kt-kr1, ncp-kr, nhn-kr2, openstack-regionone, tencent-ap-seoul |
| 소스 자원 | CSP당 VM 1대(TB로 생성, vNet·서브넷·SG·SSH 키 포함) → TB·Spider 기록만 제거해 "기존 자원"으로 전환 |

## 2. 시험 절차

| 순서 | 단계 | 요청 | 완료 판정 |
|---|---|---|---|
| 1 | 기존 자원화 | 노드·SG·키·vNet `deregisterResource` (TB가 Spider IID도 함께 해제) | TB·Spider 기록 없음, CSP 자원 유지 |
| 2 | 등록 | `POST /tumblebug/registerCspResources?infraFlag=y` `{connectionName, nsId, infraNamePrefix}` | 유형별 등록 수, 실패 0 |
| 3 | 연관관계 검증 | 등록 Infra 노드 → vNet·서브넷·SG·키 레코드 조회 | 아래 5절 항목 전부 일치 |
| 4 | 등록 해제 | 노드·SG·키·디스크·이미지·vNet `deregisterResource` | TB 등록 레코드 0, Spider 매핑 0, CSP 자원 유지 |
| 5 | 정리 | 재등록 후 이 시험이 만든 자원만 삭제, 그 외는 해제 | 시험 자원 제거 |

하네스가 각 단계의 CSP별 소요 시간을 기록함(4절).

## 3. 결과 매트릭스

| CSP | 기존 자원화 | 등록 (vNet / SG / 키 / VM) | 연관관계 검증 | 등록 해제 | 종합 |
|---|---|---|---|---|---|
| Alibaba | ✅ | ✅ 1 / 1 / 1 / 1 (8초) | ✅ 28/28 | ✅ | ✅ |
| AWS | ✅ | ✅ 3 / 4 / 3 / 1 (12초) | ✅ 28/28 | ✅ | ✅ |
| Azure | ✅ | ✅ 5 / 7 / 6 / 4 (14초) | ✅ 28/28 | ✅ | ✅ |
| GCP | ✅ | ✅ 29 / 1 / 2 / 1 (26초) | ✅ 28/28 | ✅ | ✅ |
| IBM | ✅ | ✅ 1 / 2 / 7 / 1 (60초) | ✅ 28/28 | ✅ | ✅ |
| KT | ✅ | ✅ 1 / 1 / 1 / 1 (28초) | ✅ 28/28 | ✅ | ✅ |
| NCP | ✅ | ✅ 10 / 19 / 46 / 2 (17초) | ✅ 28/28 | ✅ | ✅ |
| NHN | ✅ | ✅ 4 / 3 / 4 / 1 (40초) | ✅ 28/28 | ✅ | ✅ |
| OpenStack | ✅ | ✅ 11 / 6 / 10 / 1 (22초) | ✅ 28/28 | ✅ | ✅ |
| Tencent | ✅ | ✅ 5 / 6 / 5 / 3 (28초) | ✅ 28/28 | ✅ | ✅ |
| **합계** | 10/10 | 10/10 | **10/10** | 10/10 | **10 완료** |

등록 수는 해당 리전에 이미 있던 미추적 자원(이전 시험 잔존물, 계정 기본 VPC 등) 포함. 시험이 만든 자원은 CSP당 vNet 1·SG 1·키 1·VM 1. KT는 미추적 데이터 디스크 34건도 함께 등록됨.

## 4. 단계별 수치

| 단계 | 소요 | 비고 |
|---|---|---|
| 소스 10노드 생성 | 127초 | 커넥션별 공유 vNet·SG·키 생성 포함 |
| 기존 자원화 (노드 10·SG 10·키 10·vNet 10 해제) | 1초 | TB 해제가 Spider IID 해제까지 수행 |
| 등록 10 커넥션 (순차) | 260초 | 커넥션당 8~60초, IBM이 가장 오래 |
| 연관관계 검증 (노드당 28항목) | 12초 | CSP당 1.2초 |
| 등록 해제 전체 | 6초 | |
| 정리 (재등록·종료·삭제) | 약 22분 | 아래 CSP별 표 참고. vNet 삭제 4건은 다른 시험이 남긴 전용 SG 때문에 실패했고, SG 제거 후 성공 |

### CSP별 소요 시간 (초)

| CSP | 소스 VM 생성 | 등록 | 연관관계 검증 | 재등록 | VM 종료 | SG 삭제 | 키 삭제 | vNet 삭제 |
|---|---|---|---|---|---|---|---|---|
| Alibaba | 39 | 8 | 1.2 | 7 | 1 | 1.3 | 0.7 | 실패* → 재시도 성공 |
| AWS | 46 | 12 | 1.2 | 11 | 1 | 0.6 | 0.7 | 실패* → 재시도 성공 |
| Azure | 81 | 14 | 1.2 | 16 | 50 | 1.6 | 0.7 | 28.8 |
| GCP | 51 | 26 | 1.2 | 28 | 57 | 19.3 | <0.1 | 실패* → 재시도 성공 |
| IBM | 53 | 60 | 1.2 | 46 | 19 | 8.2 | 409 (135) | 16.6 |
| KT | 55 | 28 | 1.2 | 24 | 7 | <0.1 | 0.4 | 12.6 (tier) |
| NCP | 126 | 17 | 1.2 | 13 | 39 | 1.6 | 0.2 | 37.8 (2회 시도) |
| NHN | 85 | 40 | 1.3 | 35 | 20 | 2.4 | 0.3 | 24.4 |
| OpenStack | 117 | 22 | 1.2 | 27 | 27 | 2.3 | 0.1 | 실패** → Spider로 삭제 |
| Tencent | 49 | 28 | 1.2 | 20 | 15 | 0.4 | 1.1 | 2.8 |

소스 VM 생성은 요청 시각부터 노드 `createdTime`까지. 등록 시간은 커넥션 전체 자원을 훑는 시간이라 자원 수에 비례하며 GCP(VPC 29), NCP(키 46), IBM(키 7)이 오래 걸림. IBM 키 삭제 409는 드라이버가 등록된 키를 이름으로 조회해 찾지 못하는 기존 문제로 레코드만 해제함. `*` AWS·Alibaba·GCP의 vNet은 같은 날 앞선 시험(라이프사이클·이미지)이 남긴 전용 SG가 VPC에 붙어 있어 DependencyViolation으로 실패, 그 SG들을 삭제한 뒤 성공. `**` OpenStack은 라우터 포트가 서브넷을 쓰고 있어 TB 경로로는 실패, Spider VPC 삭제로 정리함.

## 5. 연관관계 검증 항목 (노드당 28항목, 10 CSP 전부 통과)

| 대상 | 검사 |
|---|---|
| 노드 필드 | status Running, specId·imageId·sshKeyId·vNetId·subnetId·securityGroupIds, cspVNetId·cspSubnetId·cspSshKeyId·cspSpecName·cspImageName, publicIP·privateIP 채움 |
| 소스 대비 | 노드의 CSP vNet·서브넷·키 ID가 소스 VM과 동일 |
| vNet | 레코드 존재, `cspResourceId` = 노드 `cspVNetId`, 노드 서브넷이 vNet 서브넷 목록에 포함, 서브넷 `cspResourceId` = 노드 `cspSubnetId`, `associatedObjectList`에 노드 포함 |
| SG | 레코드 존재, vNet 연결 일치, 규칙 존재, `associatedObjectList`에 노드 포함 |
| SSH 키 | 레코드 존재, CSP ID 일치(Azure ARM ID는 대소문자 무시), `associatedObjectList`에 노드 포함 |

## 6. CSP별 참고 사항 (운영)

| 메모 | CSP |
|---|---|
| 1차 실행(10-12)에서는 생성 노드의 `cspResourceId`가 ARM 전체 경로, 등록 노드는 VM 이름이라 자동 대조 실패. CB-Tumblebug가 Azure도 ARM 경로를 `cspResourceId`로 쓰고 이름은 마지막 세그먼트로 만들도록, CB-Spider가 키·VPC 등록 조회에 짧은 이름을 넘기도록 수정한 뒤 28/28 통과. ARM ID는 리소스 그룹 세그먼트의 대소문자가 응답마다 다를 수 있어(VM이 참조하는 키 ID는 `koreasouth`, 키 자원 ID는 `KOREASOUTH`) 대조는 대소문자 무시로 해야 함 | Azure |
| 한 커넥션 등록은 리전의 모든 미추적 자원을 끌어옴(GCP VPC 29, NCP 키 46, KT 디스크 34 등) | 전 CSP |
| Infra 삭제는 노드만 제거하고 그 Infra용 전용 SG·SSH 키 레코드와 CSP 자원은 남김. 남은 SG가 VPC에 붙어 있으면 vNet 삭제가 DependencyViolation으로 실패하므로 정리 시 SG·키를 먼저 삭제해야 함 | AWS, Alibaba, GCP (전 CSP 공통) |
| 등록된 키 삭제가 409: 드라이버가 등록 키를 이름으로 조회해 찾지 못함. CSP 키는 콘솔에서 정리 | IBM |
| 서브넷을 쓰는 라우터 포트가 있으면 TB 경로의 vNet 삭제가 실패. Spider VPC 삭제는 성공 | OpenStack |
| 계정 VPC는 하나이므로 vNet 등록은 VPC 레코드, 삭제는 이 시험이 만든 tier만 제거 | KT |
| 종료된 인스턴스가 한동안 목록에 남아 "CSP에만 있는 VM"으로 보임 | AWS |
