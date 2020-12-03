
# v0.3.0-espresso (Staging 2020.12.03.)

### API Change
- MCIS 자동 제어 기능 API 추가
- 동적 시스템 환경 설정 변경 기능 API 추가
- MCIS 생성 API의 모니터링 에이전트 자동 배치 옵션 제공

### Feature
- MCIS 생성시 모니터링 에이전트 자동 배치 기능 추가
- MCIS 자동 제어 기능 추가
- MCIS 시나리오 테스트 스크립트 추가
- MCIS 마스터 VM 및 VM IP 정보 제공 기능 추가
- MCIR VM 사양 패치 및 등록 기능 추가
- 동적 시스템 환경 설정 변경 기능 추가

### Bug Fix
- MCIS 종료시 런타임 오류 수정



# v0.2.0-cappuccino (2020.06.02.)

### API Change
- MCIS 통합 원격 커맨드 기능 API 추가
- 개별 VM 원격 커맨드 기능 API 추가
- MCIR Subnet 관리 API 제거
- MCIR VNic 관리 API 제거
- MCIR PublicIP 관리 API 제거
- 전체 Request 및 Response Body의 상세 항목 변경 (API 예시 참고)

### Feature
- MCIS 및 VM에 현재 수행 중인 제어 명령 정보를 관리
- 멀티 클라우드 동적 성능 밴치마킹 기능 일부 추가 (PoC 수준)
- MCIS VM 생성 및 제어시 Goroutine을 적용하여 속도 개선
- MCIS 및 VM 원격 커맨드 기능 추가
- MCIS 오브젝트 정보 보완 (VM의 위경도 정보 제공)

### Bug Fix
- MCIS 라이프사이클 오류 개선



# v0.1.0-americano (2019.12.23.)

### Feature
- Namespace, MCIR, MCIS 관리 기본 기능 제공
