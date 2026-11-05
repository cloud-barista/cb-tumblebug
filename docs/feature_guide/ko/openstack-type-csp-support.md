# OpenStack 유형 CSP 지원 (OpenStack-Type CSP Support)

## 클라우드 플랫폼(Cloud Platform) 개념

CB-Tumblebug는 각 CSP(Cloud Service Provider)의 기반이 되는 클라우드 기술을 식별하기 위해 **클라우드 플랫폼(Cloud Platform, 또는 Cloud OS Type)** 개념을 사용합니다.

대부분의 CSP에서는 클라우드 플랫폼과 CSP 이름이 **1:1 매핑** 관계를 가집니다:

| CSP 이름 | 클라우드 플랫폼 | 매핑 관계 |
|----------|---------------|---------|
| `aws` | `aws` | 1:1 |
| `azure` | `azure` | 1:1 |
| `gcp` | `gcp` | 1:1 |

하지만 **OpenStack 유형**의 CSP는 **1:N 매핑** 관계를 가집니다. 즉, 여러 개의 독립적인 CSP 인스턴스가 동일한 클라우드 플랫폼을 공유할 수 있습니다:

| CSP 이름 | 클라우드 플랫폼 | 매핑 관계 |
|----------|---------------|---------|
| `openstack` | `openstack` | 1:N |
| `openstack-new01` | `openstack` | ↑ |
| `openstack-private` | `openstack` | ↑ |

이를 통해 조직은 여러 OpenStack 배포 환경(예: 서로 다른 데이터 센터, 프라이빗 클라우드 등)을 개별 CSP로 분리하여 관리하면서도 동일한 드라이버, 아이콘 및 플랫폼 로직을 공유할 수 있습니다.

## 동작 방식

`cloudinfo.yaml`에서 특정 CSP에 `cloudPlatform` 필드가 지정되어 있으면, CB-Tumblebug는 이 값을 해석하여 기반 플랫폼을 결정합니다. 이 필드가 지정되지 않은 경우에는 CSP 이름 자체가 플랫폼 이름으로 사용됩니다. 이렇게 결정된 플랫폼 정보는 다음 항목들에 활용됩니다:

- **드라이버 선택**: OpenStack 드라이버(`openstack-driver-v1.0.so`)를 재사용합니다.
- **자격 증명 포맷**: 기본 OpenStack과 동일한 자격 증명 필드를 사용합니다.
- **UI 아이콘**: 지도 UI에서 자동으로 OpenStack 아이콘으로 폴백(Fallback)하여 표시합니다.
- **스펙/이미지 관리**: 동일한 플랫폼 수준의 처리 로직을 사용합니다.

## 새로운 OpenStack 기반 CSP 추가하기

단 **두 개의 파일**만 수정하면 됩니다:

### 1. `assets/cloudinfo.yaml`

`cloudPlatform: openstack`이 포함된 새 항목을 추가합니다:

```yaml
cloud:
  # ... 기존 CSP 목록 ...
  openstack-new01:
    description: My Private OpenStack Cloud
    cloudPlatform: openstack
    driver: openstack-driver-v1.0.so
    region:
      RegionOne:
        id: RegionOne
        description: My Region
        location:
          display: South Korea (Seoul)
          latitude: 37.5665
          longitude: 126.978
        zone:
        - nova
```

주요 필드 설명:
- **CSP 이름** (`openstack-new01`): 해당 CSP 인스턴스를 식별하기 위한 고유 식별자입니다(예: `my-private-cloud`, `datacenter-seoul`). 이름은 자유롭게 지정할 수 있으며 반드시 `openstack`이라는 단어를 포함할 필요는 없습니다.
- **`cloudPlatform: openstack`**: 기반 플랫폼을 명시적으로 선언합니다. 이 필드가 플랫폼 해석을 결정하므로, 이 값이 설정되면 CSP 이름은 플랫폼 로직에 영향을 주지 않습니다. (`aws`와 같은 표준 CSP처럼) 이 필드를 생략하면 CSP 이름 자체가 플랫폼으로 사용됩니다(동일 매핑).
- **`driver`**: 반드시 `openstack-driver-v1.0.so`를 사용해야 합니다.
- **`region`**: 해당 OpenStack 배포 환경에 맞는 리전(Region)과 존(Zone)을 정의합니다.

### 2. `init/credentials.yaml`

기본 `openstack` 항목과 동일한 필드로 자격 증명을 추가합니다:

```yaml
credentialholder:
  admin:
    # ... 기존 CSP 목록 ...
    openstack-new01:
      IdentityEndpoint: http://your-openstack:5000
      Username: your-user
      Password: your-password
      DomainName: default
      ProjectID: your-project-id
```

### 3. 변경 사항 적용

```bash
make enc-cred   # 업데이트된 자격 증명 암호화
make init        # 자산(Assets) 리로드 및 자격 증명 등록
```

이제 새로운 CSP가 연결 목록에 나타나며 Node 프로비저닝에 사용할 수 있게 됩니다. 지도 UI에서도 해당 CSP에 대해 자동으로 OpenStack 아이콘을 표시합니다.

## 참고 자료

- `cloudinfo.yaml` 예제: 파일 내 `openstack-ex01` 항목 참조
- `template.credentials.yaml`: `openstack` 섹션 아래의 주석 참조
