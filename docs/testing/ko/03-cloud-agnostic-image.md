# Cloud-agnostic 이미지 생성·관리 통합 시험 리포트

## 0. 결과 요약

**CSP 10종 전부에서 nginx가 포함된 커스텀 이미지를 단일 호출로 자동 생성(10/10)하고, 9종에서 그 이미지로 인프라를 재생성해 SG 80 포트 개방과 외부 HTTP 접속까지 완료.** KT는 커스텀 이미지에서 부트 볼륨을 만드는 KT 측 작업이 error로 끝나(Nova가 600초 대기 후 확인) 이번 실행에서 재생성 실패, 같은 날 앞선 실행에서는 711초에 성공.

| 지표 | 값 |
|---|---|
| 이미지 생성 (소스 10노드 생성 → nginx 설치 → 스냅샷 → Available → 소스 정리) | 10/10, 1,359초 단일 호출 |
| 이미지로 재생성 | 9/10 (KT는 CSP 측 볼륨 생성 지연) |
| SG 80 개방 → 외부 HTTP 200, 구운 페이지 확인 | 9/9 |
| 정리 후 잔존 | 이미지 0, Infra 0 |

## 1. 시험 환경

| 항목 | 내용 |
|---|---|
| 일시 | 2026-10-13 08:27 ~ 09:06 UTC (10종), KT·NCP 보충 09:07 ~ 09:41 UTC |
| CB-Tumblebug | `6e7da269` + 스냅샷 전 sync(미커밋), kind 클러스터 |
| CB-Spider | `fcee1108` |
| 입력 | CSP당 1노드, Ubuntu 22.04 기본 이미지, 루트 디스크 50GB `default`. 스펙은 `infra-across-global` 템플릿과 동일하되 Alibaba는 ecs.e-c1m2.large(4GiB) |
| 대상 커넥션 | alibaba-ap-northeast-1, aws-ap-northeast-3, azure-koreasouth, gcp-asia-northeast1, ibm-us-south, kt-kr1, ncp-kr, nhn-kr2, openstack-regionone, tencent-ap-seoul |

## 2. 시험 절차

| 순서 | 단계 | 요청 | 완료 판정 |
|---|---|---|---|
| 1 | 이미지 자동 생성 | `POST /ns/default/buildAgnosticImage` — 소스 Infra(10노드) 생성, `postCommands`로 nginx 설치·페이지 작성·sync, 노드별 스냅샷, Available 대기, 소스 Infra 정리 | 이미지 10건 `Available`, 소스 Infra 삭제 |
| 2 | 이미지 확인 | `GET /ns/default/resources/customImage/{id}` | 상태·OS·CSP 이미지 ID 채움 |
| 3 | 이미지로 재생성 | `POST /ns/default/infraDynamic` (`imageId` = 커스텀 이미지, CSP당 1노드) | 노드 Running |
| 4 | SG 개방 | `PUT /ns/default/infra/{id}/associatedSecurityGroups` (inbound TCP 22·80, ICMP, outbound ALL) | 규칙 반영 |
| 5 | 웹 접속 | 외부에서 `http://<publicIP>/` (최대 4분 재시도) | HTTP 200, 이미지에 구운 페이지 내용 확인 |
| 6 | 정리 | Infra terminate·delete, 커스텀 이미지 삭제 | TB·CSP 잔존 0 |

단계 1은 하네스가 10초 간격으로 소스 노드의 Running 시각과 이미지의 Available 시각을 CSP별로 기록함.

## 3. 결과 매트릭스

| CSP | nginx 설치 | 이미지 생성 | 이미지 확인 | 재생성 | SG 80 개방 | HTTP 200 | 정리 | 종합 |
|---|---|---|---|---|---|---|---|---|
| Alibaba | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| AWS | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Azure | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| GCP | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| IBM | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| NHN | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| OpenStack | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Tencent | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| NCP | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ (보충 실행) |
| KT | ✅ | ✅ | ✅ | ✖ KT 측 볼륨 생성 실패 (600초 대기 후 error) | ✅ | — | ✅ | 부분 |
| **합계** | **10/10** | **10/10** | **10/10** | 9/10 | **10/10** | 9/9 | **10/10** | 9 완료 · 1 부분 |

NCP는 10종 실행에서 재생성 노드의 공인 IP가 생성 응답에 아직 없어 HTTP 확인을 건너뛰었고, 보충 실행에서 전 단계 통과. KT는 같은 날 02:36 실행(`agimg-kt2`)에서 재생성 711초로 전 단계를 통과했으나 이후 세 번의 실행에서 모두 같은 오류로 실패.

## 4. 단계별 수치

| 단계 | 소요 | 비고 |
|---|---|---|
| 이미지 생성 전체 (`buildAgnosticImage` 1회) | 1,359초 | 소스 생성 210초(KT 최장) + 부트스트랩 약 180초 + 스냅샷·Available 대기(KT 1,277초가 전체를 결정) + 소스 정리 |
| 이미지 10건 조회 | 즉시 | 전부 `Available`, OS `Ubuntu 22.04`, CSP 이미지 ID 채움 |
| 이미지로 10노드 재생성 | 720초 | 9노드는 185초 내 Running, KT 실패 판정까지 667초 |
| SG 규칙 적용 (10 SG) | 110초 | NCP ACG 반영 대기 재시도 포함 |
| HTTP 응답 확인 | SG 적용 직후 200 | 9 CSP 전부 첫 시도에 200 |
| 종료·삭제 + 이미지 10건 삭제 | 61초 + 60초 | Infra·이미지 잔존 0. 소스·재생성 Infra용 전용 SG·SSH 키(CSP당 2쌍)는 Infra 삭제와 독립이라 별도 삭제 필요 |

### CSP별 소요 시간 (초, 이미지 생성 요청 시각 기준)

| CSP | 소스 노드 Running | 이미지 Available | 재생성 (요청 → Running) | SG 규칙 추가 / 삭제* | HTTP 200 |
|---|---|---|---|---|---|
| Alibaba | 37 | 445 | 40 | <1 / <1 | SG 적용 직후 |
| AWS | 51 | 643 | 48 | <1 / <1 | SG 적용 직후 |
| Azure | 53 | 435 | 80 | 2.2 / 1.6 | SG 적용 직후 |
| GCP | 52 | 493 | 68 | 8.1 / 6.5 | SG 적용 직후 |
| IBM | 52 | 435 | 67 | 4.5 / 2.7 | SG 적용 직후 |
| NHN | 93 | 540 | 185 | <1 / 1.2 | SG 적용 직후 |
| OpenStack | 120 | 388 | 85 | <1 / <1 | SG 적용 직후 |
| Tencent | 55 | 420 | 56 | 1.1 / <1 | SG 적용 직후 |
| NCP | 127 (보충 118) | 403 (보충 278) | 124 (보충 107) | <1 / 13 | SG 적용 직후 (보충) |
| KT | 210 (보충 59) | 1,277 (보충 1,069) | 실패 (02:36 실행 711) | 2.1 / 1.9 | — (02:36 실행 200) |

`*` SG 규칙 추가·삭제는 별도 측정(10-14, CSP당 노드 1대가 붙은 SG에 inbound TCP 80 규칙을 순차로 추가·삭제)이며, 본 실행의 SG 적용(10 SG 동시 110초)은 NCP의 ACG 반영 대기 재시도가 전체 시간을 결정함. 부트스트랩(nginx 설치·페이지 작성·sync)은 모든 노드가 Running이 된 뒤 일괄 실행되어 약 388초 시점에 스냅샷이 요청됐으므로, "이미지 Available − 388"이 CSP별 스냅샷 소요. OpenStack은 즉시, AWS 255초, KT 889초. 재생성 시간은 재생성 응답의 노드 `createdTime`과 요청 시각의 차이.

## 5. 정보 정확성

| 검사 | 결과 |
|---|---|
| 커스텀 이미지 레코드 (status·osType·cspImageName) | 10/10 채움, 소스 노드 그룹별 이름(`<name>-<nodeGroup>`) 부여 |
| 재생성 노드의 `imageId`가 커스텀 이미지를 가리킴 | 9/9 |
| 재생성 노드가 구운 페이지를 서빙 (`CB-TB cloud-agnostic image`) | 9/9 |
| 삭제 후 TB·Spider·CSP 잔존 | 이미지 0, Infra 0 (전용 SG·키는 별도 삭제로 정리) |

## 6. CSP별 참고 사항 (운영)

| 메모 | CSP |
|---|---|
| 스냅샷 완료가 가장 오래 걸림 (AWS 약 4분, NHN 약 2.5분, KT 약 15분) | AWS, NHN, KT |
| 부트스트랩 직후 스냅샷을 뜨면 아직 디스크에 쓰이지 않은 파일이 이미지에서 빠짐. 패키지 파일은 남고 직접 쓴 페이지·설정 수정만 사라져 "nginx 기본 페이지 200"으로 나타남. 스냅샷 전 sync로 해소 | Alibaba, GCP, NHN, OpenStack, Tencent, NCP에서 관측 |
| 커스텀 이미지에서 부트 볼륨을 만드는 KT 측 작업이 10분 안팎 걸리고 때때로 error로 끝남. Nova는 600초 대기 후 볼륨 상태를 확인해 인스턴스 빌드를 중단하므로 VM이 ERROR로 남음. 같은 날 4회 중 1회 성공(711초). TB·Spider 타임아웃이 아니라 KT 플랫폼 동작 | KT |
| 재생성 VM은 OS 부팅 후 cloud-init이 새 SSH 키를 쓰기까지 수 분 걸려 직후의 수동 원격 명령은 인증 실패 | KT |
| 재생성 노드의 공인 IP가 생성 응답에 늦게 채워질 수 있음. 조회를 다시 하면 채워짐 | NCP |
| 이미지는 커널 옵션 ipv6.disable=1로 부팅되어 nginx 기본 사이트의 `listen [::]:80`이 실패. 부트스트랩에서 해당 줄을 제거 | NCP |
| ACG 변경 반영 중 다음 변경을 보내면 거부(1007009). CB-Spider NCP 드라이버의 재시도로 해소 | NCP |
| 템플릿 스펙 ecs.e-c4m1.large(메모리 0.5GiB)는 SSH 가능 상태까지 부팅되지 않음. 4GiB 스펙으로 교체 | Alibaba |
| 전역 machine image라 같은 이미지가 다른 리전 커넥션에서도 보임 | GCP |
