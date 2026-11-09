# CB-Tumblebug 기능 가이드

> 🌐 **Language**: **한국어** | [English](../README.md)

이 디렉토리는 CB-Tumblebug의 주요 기능별 상세 기술 문서를 포함한다. 각 가이드는 멀티클라우드 인프라 관리 시스템의 핵심 기능에 대한 개념, 내부 아키텍처 및 사용 방법을 다룬다.

## 📚 제공되는 가이드 목록

| 기능 가이드 | 설명 |
|---------|-------------|
| **[에셋 백업 및 복원](assets-backup-restore.md)** | PostgreSQL 에셋 데이터베이스(컴퓨팅 스펙, 이미지, 가격 정보) 백업 및 복원 가이드. |
| **[클라우드 독립 이미지](cloud-agnostic-image.md)** | 여러 클라우드에서 CSP에 종속되지 않는 커스텀 이미지를 자동으로 생성하는 통합 워크플로우(프로비저닝 → 설정 → 스냅샷 → 정리) 설명. |
| **[자격증명 및 커넥션 관리](credential-and-connection.md)** | 자격증명, 자격증명 소유자, 커넥션 관리: 멀티테넌트 자격증명 격리, 커넥션 명명 규칙 및 `X-Credential-Holder` 헤더 사용법 안내. |
| **[글로벌 DNS 관리](global-dns-management.md)** | AWS Route53을 활용한 글로벌 DNS 레코드 관리: Infra/레이블 기반 IP 자동 해석, 단순 라우팅 및 지리적 근접성 라우팅, 일괄 제어 기능 설명. |
| **[대규모 프로비저닝 아키텍처](high-scale-provisioning-architecture.md)** | 10개 CSP, 146개 리전, 10,000+ VM 규모 프로비저닝 및 생명주기 제어를 위한 4개 영역, 10대 핵심 병목 해결 아키텍처 및 실측 벤치마크 심층 분석. |
| **[인프라 프로비저닝](infra-provisioning.md)** | 인프라 생성 End-to-End 가이드: 요청 구조, 부분 실패 정책(`continue`/`refine`/`rollback`), 공유 리소스 자동 생성, 레이블, 배포 후 명령(단계별/타겟팅/비동기 스트리밍), 스케일링, 검증 및 정리. |
| **[인프라 리소스 모델 및 생명주기 관리](infra-resource-model-and-lifecycle-management.md)** | `Infra`, `NodeGroup`, `Node`의 계층적 리소스 모델 개념과 생명주기 제어(`Suspend`, `Resume`, `Reboot`, `Terminate`, `Refine`) 및 `NodeStatusAgent` 상태 관리 아키텍처 설명. |
| **[기존 CSP 리소스 동기화 작업 스케줄러](existing-csp-res-job-scheduler.md)** | 주기적인 CSP 기존 리소스 탐지, 동기화 및 자동 등록을 수행하는 작업 스케줄러 가이드. |
| **[`make init` 초기화 워크플로우](make-init-workflow.md)** | OpenBao 자격증명 등록과 Tumblebug 에셋 적재로 구성된 2단계 초기화 프로세스를 컴포넌트 간 시퀀스 다이어그램과 함께 상세히 설명. |
| **[네임스페이스 및 리소스 식별 체계](namespace-and-resource-identity.md)** | 논리적 격리 모델(Namespace)과 3중 리소스 식별 체계(사용자 지정 ID vs. 시스템 UID vs. CSP 네이티브 ID) 심층 분석. |
| **[네트워크 상태 관리](network-status-management.md)** | VNet, Subnet, VPN 등 멀티클라우드 네트워크 리소스의 상태 모델(Kubernetes 스타일 Conditions) 및 생명주기 관리 설명. |
| **[알림 채널 (Notifier, ntfy)](notifier-ntfy.md)** | ntfy 기반 알림 채널 가이드: 관리형 서버 프로비저닝(자체서명 또는 Route53 + Let's Encrypt), 기존 서버 등록, 쓰기/읽기 토큰, 사람용 구독 계정, 메시지 발행과 수신. |
| **[OpenStack 계열 CSP 지원](openstack-type-csp-support.md)** | 1:N 클라우드 플랫폼 매핑(`cloudinfo.yaml` + `credentials.yaml`)을 통해 OpenStack 기반 CSP를 추가하고 운영하는 방법 안내. |
| **[RDBMS 관리](rdbms-management.md)** | 멀티클라우드 관리형 데이터베이스(RDBMS) 프로비저닝, 보안 그룹 제어, Bastion 연동 및 생명주기 관리 가이드. |
| **[원격 명령 실행 및 파일 전송](remote-command-and-file-transfer.md)** | Bastion 호스트 및 최초 접속 시 SSH 호스트 키 자동 등록, 검증을 통해 인프라 노드에 원격 명령을 실행하고 파일을 전송하는 방법 설명. |
| **[리소스 정합성 조정](resource-reconciliation.md)** | 선언형 리소스 정합성 조정(`reconcile`), 진단 상태 머신, 자가 치유 및 고아 메타데이터 정리(`PUT .../reconcile`, `POST .../prune`) 가이드. |
| **[리소스 템플릿 관리](resource-template-management.md)** | 반복 가능한 멀티클라우드 프로비저닝을 위해 재사용 가능한 인프라 템플릿(`Infra`, `vNet`, `SecurityGroup`)을 정의, 저장 및 적용하는 방법 안내. |
| **[공유 리소스 관리](shared-resource-management.md)** | 공유 리소스(`VNet`, `SecurityGroup`, `SSHKey`)의 생성, 재사용, 해제 및 고아 CSP 리소스로 인한 `DependencyViolation` 복구 방법 안내. |
| **[스펙 및 이미지 추천](spec-image-recommendation.md)** | 멀티클라우드 전반에서 컴퓨팅 스펙 추천, 이미지 검색, 스펙-이미지 호환성 사전 검토 및 대체 구성 탐색(`recommendSpec`, `searchImage`, `recommendAlternativeNodeConfig`)을 수행하는 방법 설명. |
| **[스토리지 상태 관리](storage-status-management.md)** | DataDisk 및 Object Storage 리소스의 상태 관리, 연결/분리 및 생명주기 검증 메커니즘 설명. |

## 🚀 문서의 목적

이 가이드 문서는 개발자와 운영자가 다음 사항을 깊이 이해할 수 있도록 작성됨:
- **개념**: 각 기능이 왜 필요한지와 무엇을 해결하는지 설명함.
- **아키텍처**: 내부적으로 어떻게 구현되어 있는지 Mermaid 다이어그램과 함께 시각적으로 분석함.
- **사용법**: 단계별 실행 방법과 REST API 레퍼런스를 제공함.

CB-Tumblebug의 진화에 맞춰 새로운 기능 가이드가 지속적으로 추가된다.
