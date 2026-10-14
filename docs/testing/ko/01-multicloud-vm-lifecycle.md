# 멀티 클라우드 가상머신 통합 운용·관리 통합 시험 리포트

## 0. 결과 요약

**CSP 10종 · 18 리전 · 40 노드에서 생성 → 조회 → 중단 → 재시작 → 재부팅 → 종료 → 삭제 전 단계 정상 완료.**

| 지표 | 값 |
|---|---|
| 생성 (요청 → 40/40 Running) | 446초 |
| 중단 → 재시작 (suspend 완료 553초 + resume 완료 94초) | 647초 (Alibaba 도쿄 2대 제외 시 중단 63초) |
| 재부팅 (요청 → 40/40 Running) | 555초 (같은 2대 제외 시 49초) |
| 종료 (요청 → 40/40 Terminated) | 66초 |
| 제어 실패 / 조회 정보 불일치 | 0건 / 0건 |

## 1. 시험 환경

| 항목 | 내용 |
|---|---|
| 일시 | 2026-10-13 06:58 ~ 07:38 UTC (전체 2,348초) |
| CB-Tumblebug | `6e7da269`, kind 클러스터 배포 |
| CB-Spider | `fcee1108` |
| 입력 | Infra 템플릿 `infra-across-global` (18 nodeGroup, 40 노드, 루트 디스크 `default`) |
| 대상 CSP | Alibaba, AWS, Azure, GCP, IBM, KT, NCP, NHN, OpenStack, Tencent |

| CSP | 리전 | 스펙 | 노드 |
|---|---|---|---|
| Alibaba | ap-northeast-1 / us-east-1 | ecs.e-c4m1.large | 2 / 2 |
| AWS | ap-northeast-3 / ca-central-1 | t3.small / t3a.small | 2 / 2 |
| Azure | koreasouth / australiaeast | standard_b2ts_v2 / standard_b1ms | 2 / 2 |
| GCP | asia-northeast1 / europe-west3 | e2-standard-2 / t2d-standard-1 | 2 / 2 |
| IBM | us-south / jp-osa | bxf-2x8 / nxf-2x1 | 2 / 2 |
| KT | kr1 | 2x2.itl | 4 |
| NCP | kr / sgn | s2-g3a / c2-g3 | 2 / 2 |
| NHN | kr2 / jp1 | c2.c2m2 | 2 / 2 |
| OpenStack | regionone | m1.medium | 4 |
| Tencent | ap-seoul / ap-hongkong | s2.medium2 | 2 / 2 |

## 2. 시험 절차

| 순서 | 단계 | 요청 | 완료 판정 |
|---|---|---|---|
| 1 | 생성 | `POST /ns/default/infra/template/infra-across-global` | 40 노드 전부 `Running` |
| 2 | 정보 조회 | 상세·목록·상태·접속정보·노드 단건 API | 필수 필드 채움, 뷰 간 값 일치 |
| 3 | 중단 | `GET /ns/default/control/infra/{id}?action=suspend` | 40 노드 전부 `Suspended` |
| 4 | 재시작 | `?action=resume` | 40 노드 전부 `Running`, IP 변경 시 전 뷰 반영 |
| 5 | 재부팅 | `?action=reboot` | 40 노드 전부 `Running` |
| 6 | 종료 | `?action=terminate` | 40 노드 전부 `Terminated` |
| 7 | 삭제 | `DELETE /ns/default/infra/{id}` | 목록·상세에서 제거 |

각 단계 완료 후 2절의 조회를 반복해 정보 정확성을 확인함.

## 3. 결과 매트릭스

| CSP | 생성 | 조회 | 중단 | 재시작 | 재부팅 | 종료 | 삭제 | 종합 |
|---|---|---|---|---|---|---|---|---|
| Alibaba (4) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| AWS (4) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Azure (4) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| GCP (4) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| IBM (4) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| KT (4) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| NCP (4) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| NHN (4) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| OpenStack (4) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Tencent (4) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **합계 (40)** | 40/40 | 40/40 | 40/40 | 40/40 | 40/40 | 40/40 | 40/40 | **통과** |

## 4. 단계별 수치

| 단계 | 전원 완료 (요청 → 40/40) | 비고 |
|---|---|---|
| 생성 | 446초 (동기 응답) | KT 4대가 401~444초로 가장 늦음 |
| 중단 | 553초 | 38대는 63초 내 Suspended, Alibaba ap-northeast-1 2대가 544·553초 |
| 재시작 | 94초 | OpenStack 4대가 80~94초, 나머지는 49초 내 |
| 재부팅 | 555초 | 38대는 49초 내 Running 복귀, Alibaba ap-northeast-1 2대가 539·555초 |
| 종료 | 66초 (동기 응답) | 응답 시점에 40/40 Terminated |
| 삭제 | 즉시 | Infra·노드 레코드 제거. Infra용 전용 SG·SSH 키(18 nodeGroup분)는 남으므로 별도 삭제 필요 |

### CSP별 소요 시간 (초, 노드 4대의 최소~최대)

| CSP | 생성 | 중단 | 재시작 | 재부팅 | 종료 |
|---|---|---|---|---|---|
| AWS (4) | 96~101 | 21~37 | 23 | 6 | 26~32 |
| Azure (4) | 134~142 | 15~26 | 23 | 6 | 9 |
| GCP (4) | 110~128 | 63 | 23 | 6~12 | 63 |
| IBM (4) | 99~103 | 21 | 23 | 6 | 21 |
| NHN (4) | 143~168 | 21~26 | 23~26 | 6~23 | 26~32 |
| OpenStack (4) | 181~224 | 32~37 | 80~94 | 49 | 32 |
| Tencent (4) | 106~109 | 15~21 | 17~23 | 6 | 9 |
| Alibaba (4) | 86~91 | 32~553 | 17~23 | 32~555 | 6 |
| NCP (4) | 178~183 | 15~21 | 49 | 32~49 | 63 |
| KT (4) | 401~444 | 37 | 23 | 23 | 37 |

생성은 "요청 시각 → 노드 createdTime(Running 확정)"이며 커넥션별 공유 vNet·SG·키 생성 포함. 제어 단계는 제어 요청 시각부터 노드가 목표 상태로 관측된 시각까지이며, 5초 간격 관측이라 최대 5초 오차 포함. 종료 제어는 동기 응답이어서 요청과 병행해 관측함. Alibaba ap-northeast-1의 2대는 중단·재부팅에서 약 9분간 CSP 상태가 바뀌지 않다가 완료(같은 스펙의 us-east-1 2대는 32초). 템플릿 스펙(메모리 0.5GiB)의 영향으로 추정.

## 5. 정보 정확성

전 단계에서 다음 검사를 40 노드에 수행, 모두 통과함.

| 검사 | 결과 |
|---|---|
| 상세 뷰 필수 필드 (spec·image·vNet·subnet·sshKey·SG·CSP ID·IP·region·location·spec/image 요약·createdTime) | 공백·플레이스홀더 0 |
| 상세 ↔ 목록 (`GET /infra`) | status·IP·spec·image·CSP ID·createdTime 일치 (목록 뷰는 nodeUserName을 제공하지 않아 비교에서 제외) |
| 상세 ↔ 상태 뷰 (`?option=status`, 단건·목록) | status·publicIP·cspResourceId 일치 |
| 상세 ↔ 접속정보 (`?option=accessinfo`) | publicIP·사용자명·SSH 키 일치 |
| 단계 간 불변 필드 (spec·image·key·vNet·subnet·SG·CSP ID·createdTime) | 변경 0 |
| 재시작 후 IP 변경 반영 | 변경 12건 전부 전 뷰 동일 |
| 삭제 후 | `?option=id`·상세에서 제거 |

## 6. CSP별 참고 사항 (운영)

| 특성 | CSP |
|---|---|
| 중단 시 공인 IP 해제, 재시작 시 새 IP 할당 | AWS, GCP |
| 중단 시 공인 IP 해제, 재시작 시 같은 IP 복원 | Alibaba |
| 중단 중에도 공인 IP 유지 | Azure, IBM, KT, NCP, NHN, OpenStack, Tencent |
| 종료 후 | 전 CSP에서 마지막 IP 보존 (삭제 전 감사 목적, 의도된 동작) |
| 재부팅 전이 | 대부분 49초 내 Running 복귀, Alibaba ap-northeast-1 2대는 약 9분 소요 |
| 중단 전이 | 대부분 63초 내 Suspended, Alibaba ap-northeast-1 2대는 약 9분 소요 (운영체제 정지 대기 후 CSP가 강제 정지한 것으로 보임) |
