# CB-Tumblebug 자격 증명(Credential), 자격 증명 소유자(Credential Holder) 및 연결(Connection) 관리

## 개요

CB-Tumblebug는 다양한 클라우드 제공자(CSP)에 걸쳐 통합된 인프라 프로비저닝을 지원하기 위해 **멀티클라우드 자격 증명**(Credentials)과 **연결**(Connections)을 관리합니다. 이 문서에서는 다음 세 가지 핵심 개념을 설명합니다:

- **Credential (자격 증명)**: 클라우드 제공자 인증에 사용되는 CSP API 키 및 시크릿
- **Credential Holder (자격 증명 소유자)**: 자격 증명 세트를 소유하는 논리적 주체 (예: `admin`, `team_a`, `role01`)
- **Connection (`ConnConfig`, 연결)**: 리소스 작업을 즉시 수행할 수 있도록 자격 증명과 특정 CSP 리전을 완전히 매핑한 연결 구성

이 세 가지 개념은 유기적으로 결합되어 **멀티테넌트 자격 증명 격리**(Multi-tenant credential isolation)를 지원합니다. 즉, 단일 CB-Tumblebug 인스턴스를 통해 서로 다른 팀이나 역할(Role)이 각기 다른 클라우드 계정 세트를 안전하게 운영할 수 있습니다.

## 핵심 개념

### 1. Credential (자격 증명)

**Credential**은 인증에 필요한 CSP별 API 키 세트입니다. 각 자격 증명은 하나의 클라우드 제공자와 하나의 자격 증명 소유자(Credential Holder)에 연결됩니다.

| CSP | 필수 키 (Required Keys) |
|-----|--------------|
| AWS | `aws_access_key_id`, `aws_secret_access_key` |
| Azure | `clientId`, `clientSecret`, `tenantId`, `subscriptionId` |
| GCP | `client_id`, `client_email`, `private_key_id`, `private_key`, `project_id`, `S3AccessKey`, `S3SecretKey` |
| Alibaba | `AccessKeyId`, `AccessKeySecret` |
| IBM | `ApiKey`, `S3AccessKey`, `S3SecretKey` |
| KT | `IdentityEndpoint`, `Username`, `Password`, `DomainName`, `ProjectID`, `S3AccessKey`, `S3SecretKey` |
| NCP | `ncloud_access_key`, `ncloud_secret_key` |
| NHN | `IdentityEndpoint`, `Username`, `Password`, `DomainName`, `TenantId`, `S3AccessKey`, `S3SecretKey`, `User Access Key`, `Secret Access Key`, `mysqlAppKey`, `mariadbAppKey` |
| OpenStack | `IdentityEndpoint`, `Username`, `Password`, `DomainName`, `ProjectID` |
| Tencent | `SecretId`, `SecretKey` |

**보안(Security):** 자격 증명은 절대 평문으로 저장되지 않으며, 다음과 같은 하이브리드 암호화 워크플로우를 거칩니다:
1. 클라이언트가 임시 AES 키로 자격 증명 값을 암호화합니다.
2. AES 키 자체를 서버의 RSA 공개 키로 암호화합니다.
3. 서버가 RSA 개인 키로 AES 키를 복호화한 후, 자격 증명 값을 복호화합니다.
4. 복호화된 값은 CSP 등록을 위해 CB-Spider로 전달됩니다.

### 2. Credential Holder (자격 증명 소유자)

**Credential Holder**는 자격 증명 그룹과 그로부터 생성된 연결들을 **소유하고 격리**하는 논리적 주체(Identity)입니다. 이를 통해 단일 CB-Tumblebug 인스턴스 내에서 멀티테넌트 자격 증명 관리가 가능해집니다.

```mermaid
graph TB
    TB[CB-Tumblebug]
    
    subgraph ADMIN["Credential Holder: admin (default)"]
        CRED_A1[Credential: aws]
        CRED_A2[Credential: gcp]
        CRED_A3[Credential: azure]
        CONN_A1[aws-ap-northeast-2]
        CONN_A2[aws-us-east-1]
        CONN_A3[gcp-asia-northeast3]
        CONN_A4[azure-koreacentral]
        
        CRED_A1 --> CONN_A1
        CRED_A1 --> CONN_A2
        CRED_A2 --> CONN_A3
        CRED_A3 --> CONN_A4
    end
    
    subgraph ROLE01["Credential Holder: role01"]
        CRED_R1[Credential: aws]
        CONN_R1[role01-aws-ap-northeast-2]
        CONN_R2[role01-aws-us-east-1]
        
        CRED_R1 --> CONN_R1
        CRED_R1 --> CONN_R2
    end
    
    subgraph TEAM_A["Credential Holder: team_a"]
        CRED_T1[Credential: aws]
        CRED_T2[Credential: gcp]
        CONN_T1[team_a-aws-ap-northeast-2]
        CONN_T2[team_a-gcp-us-central1]
        
        CRED_T1 --> CONN_T1
        CRED_T2 --> CONN_T2
    end
    
    TB --> ADMIN
    TB --> ROLE01
    TB --> TEAM_A
    
    style ADMIN fill:#e1f5ff
    style ROLE01 fill:#fff4e1
    style TEAM_A fill:#ffe1f5
```

**주요 특징:**
- 각 소유자(Holder)는 **서로 다른 CSP 조합**에 대한 자격 증명을 가질 수 있습니다 (예: `admin`은 AWS+GCP+Azure 보유, `role01`은 AWS만 보유).
- **기본 소유자**(Default holder)인 `admin`은 시스템 기본값으로, 소유자를 별도로 지정하지 않으면 `admin`이 사용됩니다.
- 소유자 이름은 **대소문자를 구분하지 않으며** 소문자로 저장됩니다.
- 소유자 이름에는 **하이픈(`-`)을 사용할 수 없으며**, 영문 소문자, 숫자, 밑줄(`[a-z0-9_]`)만 허용됩니다. 이 제한은 연결 이름 지정 규칙에서 하이픈이 구분자로 사용되기 때문에 존재합니다(예: `role01-aws-ap-northeast-2`). 만약 하이픈이 허용된다면(예: `team-a`), 연결 이름이 모호해집니다(`team-a-aws-...`에서 소유자가 `team-a`인지 `team`인지 구분 불가).
- 소유자는 **명시적으로 생성되지 않으며**, 해당 소유자 이름으로 자격 증명이 등록될 때 자동으로 생성(도출)됩니다.

### 3. Connection (`ConnConfig`, 연결)

**Connection**(내부적으로 `ConnConfig`)은 자격 증명을 특정 CSP 리전에 연결하는 완전히 해석된 구성입니다. 이는 "즉시 사용 가능한(ready-to-use)" 클라우드 연결 엔드포인트를 나타냅니다.

```go
type ConnConfig struct {
    ConfigName           string     // Connection name (unique identifier)
    ProviderName         string     // Cloud provider (e.g., "aws", "gcp")
    DriverName           string     // CB-Spider driver name
    CredentialName       string     // Associated credential in CB-Spider
    CredentialHolder     string     // Owner of this connection
    RegionZoneInfoName   string     // Region/zone info name in CB-Spider
    RegionDetail         RegionDetail // Detailed region/location information
    RegionRepresentative bool       // Whether this is the representative for its region
    Verified             bool       // Whether connectivity was verified
}
```

**연결 명명 규칙 (Connection Naming Convention):**

| 소유자 유형 | 패턴 | 예시 |
|-------------|---------|---------|
| 기본 소유자 (`admin`) | `{provider}-{region}` | `aws-ap-northeast-2` |
| 일반 소유자 (Non-default) | `{holder}-{provider}-{region}` | `role01-aws-ap-northeast-2` |

이 명명 규칙은 매우 중요합니다. CB-Tumblebug는 모든 작업에서 어떤 CSP 계정을 사용할지 결정할 때 바로 이 규칙을 통해 연결을 해석합니다.

## 개념 간의 관계

```mermaid
graph LR
    subgraph "credentials.yaml"
        YAML["credentialholder:<br/>  admin:<br/>    aws: {keys...}<br/>    gcp: {keys...}<br/>  role01:<br/>    aws: {keys...}"]
    end
    
    subgraph "CB-Tumblebug"
        CRED[Credential<br/>소유자별 + 제공자별]
        CONN[Connection Configs<br/>소유자별 + 제공자별 + 리전별]
        HOLDER[Credential Holder<br/>연결 정보로부터 도출됨]
    end
    
    subgraph "CB-Spider"
        SP_CRED[Spider Credential]
        SP_REGION[Spider Region]
        SP_CONN[Spider ConnConfig]
    end
    
    YAML -->|init.py| CRED
    CRED -->|RegisterCredential| SP_CRED
    CRED -->|리전별 자동 생성| CONN
    CONN -->|RegisterConnectionConfig| SP_CONN
    SP_CRED --> SP_CONN
    SP_REGION --> SP_CONN
    CONN -->|소유자별 집계| HOLDER
    
    style YAML fill:#f5f5f5
    style CRED fill:#e1f5ff
    style CONN fill:#fff4e1
    style HOLDER fill:#ffe1f5
```

### 등록 흐름 (Registration Flow)

특정 소유자 + 제공자에 대한 자격 증명이 등록되면 CB-Tumblebug는 자동으로 다음 작업을 수행합니다:

1. CB-Spider에 **자격 증명을 등록**합니다 (소유자+제공자당 1개의 자격 증명).
2. 해당 제공자에 대해 `cloudinfo.yaml`에 정의된 **모든 리전을 탐색**합니다.
3. 각 리전에 대해 **`ConnConfig`를 생성**합니다 (소유자+제공자+리전당 1개의 연결).
4. CSP API에 대해 각 `ConnConfig`를 테스트하여 **연결성(Connectivity)을 검증**합니다.
5. 여러 존(Zone)을 가진 리전의 경우 **대표 리전(Region Representative)을 선택**합니다.

```mermaid
sequenceDiagram
    participant User
    participant TB as CB-Tumblebug
    participant Spider as CB-Spider
    participant CSP as Cloud Provider
    
    User->>TB: POST /credential<br/>{credentialHolder: "role01",<br/> providerName: "aws", ...}
    
    TB->>Spider: 자격 증명 등록<br/>(name: "role01-aws")
    Spider-->>TB: OK
    
    loop cloudinfo.yaml의 각 리전에 대해 반복
        TB->>Spider: 연결 구성 등록<br/>(name: "role01-aws-{region}")
        Spider-->>TB: OK
    end
    
    par 연결 동시 검증
        TB->>Spider: "role01-aws-ap-northeast-2" 확인
        Spider->>CSP: DescribeRegions()
        CSP-->>Spider: OK
        Spider-->>TB: Verified ✓
    and
        TB->>Spider: "role01-aws-us-east-1" 확인
        Spider->>CSP: DescribeRegions()
        CSP-->>Spider: OK
        Spider-->>TB: Verified ✓
    end
    
    TB-->>User: 모든 연결이 포함된 CredentialInfo 반환
```

## Credential Holder 실제 동작 방식

### 1. Credential Holder 지정 방법

자격 증명 소유자는 **`X-Credential-Holder`** HTTP 헤더를 통해 전달됩니다:

```bash
# 기본 소유자(admin) 사용 — 헤더 생략 가능
curl -X POST http://localhost:1323/tumblebug/ns/default/infraDynamic \
  -H "Content-Type: application/json" \
  -d '{ ... }'

# 특정 소유자 지정 사용
curl -X POST http://localhost:1323/tumblebug/ns/default/infraDynamic \
  -H "Content-Type: application/json" \
  -H "X-Credential-Holder: role01" \
  -d '{ ... }'
```

### 2. 요청 처리 파이프라인

```mermaid
sequenceDiagram
    participant Client
    participant MW as Middleware
    participant Handler as REST Handler
    participant Core as Core Function
    participant Spider as CB-Spider

    Client->>MW: POST /ns/default/infraDynamic<br/>X-Credential-Holder: role01
    
    MW->>MW: 헤더 추출<br/>holder = "role01"<br/>(헤더가 없으면 기본값: "admin")
    MW->>MW: context.Context에 주입<br/>WithCredentialHolder(ctx, "role01")
    MW->>Handler: 정보가 추가된 컨텍스트 전달
    
    Handler->>Core: CreateInfraDynamic(ctx, nsId, req)
    
    Core->>Core: holder = CredentialHolderFromContext(ctx)<br/>→ "role01"
    Core->>Core: connectionName = ResolveConnectionName(<br/>"aws-ap-northeast-2", "role01")<br/>→ "role01-aws-ap-northeast-2"
    
    Core->>Spider: "role01-aws-ap-northeast-2"를 통해<br/>Node 생성
    Spider-->>Core: Node 생성 완료
    Core-->>Handler: Infra 결과 반환
    Handler-->>Client: 응답 반환
```

### 3. 연결 이름 해석 (Connection Name Resolution)

`ResolveConnectionName` 함수는 기본 소유자의 연결 이름을 현재 활성화된 자격 증명 소유자에 맞는 연결 이름으로 변환합니다:

```go
// ResolveConnectionName converts a default credential holder's connection name
// to the appropriate connection name for the given credential holder.
func ResolveConnectionName(defaultConnectionName string, credentialHolder string) string {
    if credentialHolder == "" || 
       strings.EqualFold(credentialHolder, model.DefaultCredentialHolder) {
        return defaultConnectionName
    }
    return credentialHolder + "-" + defaultConnectionName
}
```

**예시:**

| 입력 연결 이름 | 자격 증명 소유자 | 변환 결과 |
|-----------------|-------------------|--------|
| `aws-ap-northeast-2` | `admin` (기본값) | `aws-ap-northeast-2` |
| `aws-ap-northeast-2` | `role01` | `role01-aws-ap-northeast-2` |
| `gcp-us-central1` | `team_a` | `team_a-gcp-us-central1` |

### 4. 컨텍스트 전파 (Context Propagation)

자격 증명 소유자 정보는 함수마다 파라미터를 일일이 전달(Parameter drilling)하지 않고 Go의 `context.Context`를 통해 시스템 전반으로 전달됩니다:

```go
// 미들웨어에서 설정
ctx = common.WithCredentialHolder(ctx, holder)

// 모든 코어 함수에서 추출
credentialHolder := common.CredentialHolderFromContext(ctx)

// 내부/시스템 호출 시 기본값 사용
ctx := common.NewDefaultContext() // → admin 소유자
```

| 계층 (Layer) | 메커니즘 |
|-------|-----------|
| HTTP 요청 | `X-Credential-Holder` 헤더 |
| 미들웨어 (Middleware) | 헤더 읽기 → `context.Context`에 주입 |
| REST 핸들러 | 코어 함수에 `c.Request().Context()` 전달 |
| 코어 함수 (Core Function) | `CredentialHolderFromContext(ctx)`로 소유자 추출 |
| 연결 조회 (Connection Lookup) | `ResolveConnectionName()`으로 소유자별 연결 이름 생성 |
| CB-Spider 호출 | 해석된 연결 이름을 사용하여 CSP 작업 수행 |

### 5. API에 미치는 영향

자격 증명 소유자 설정은 여러 API 동작에 영향을 미칩니다:

| 기능 | 영향 |
|-----------|--------|
| **Infra 프로비저닝** | 해당 소유자의 CSP 계정을 사용하여 Node가 생성됨 |
| **리소스 생성** | VNet, SecurityGroup, SSHKey 생성 시 소유자별 연결을 사용함 |
| **스펙 추천 (Spec Recommendation)** | 추천 결과가 해당 소유자가 사용 가능한 CSP들로만 자동 필터링됨 |
| **연결 목록 조회** | `GET /connConfig` 호출 시 `filterCredentialHolder` 쿼리 파라미터로 필터링 가능 |
| **이미지 검색** | CSP 이미지 조회 시 해당 소유자의 연결을 사용함 |

## Credential Holder API

### 모든 자격 증명 소유자 목록 조회

```bash
GET /tumblebug/credentialHolder
```

**응답:**
```json
{
  "credentialHolderList": [
    {
      "credentialHolder": "admin",
      "providers": ["aws", "azure", "gcp"],
      "connectionCount": 42,
      "verifiedConnectionCount": 38,
      "isDefault": true
    },
    {
      "credentialHolder": "role01",
      "providers": ["aws"],
      "connectionCount": 14,
      "verifiedConnectionCount": 12,
      "isDefault": false
    }
  ]
}
```

### 특정 자격 증명 소유자 조회

```bash
GET /tumblebug/credentialHolder/{holderId}
```

**응답:**
```json
{
  "credentialHolder": "role01",
  "providers": ["aws"],
  "connectionCount": 14,
  "verifiedConnectionCount": 12,
  "isDefault": false
}
```

> **참고:** 자격 증명 소유자는 명시적으로 생성되는 객체가 아니라 등록된 연결 구성(`ConnConfig`)으로부터 **도출**(Derived)됩니다. API는 연결 데이터를 집계하여 소유자 요약 정보를 생성합니다.

## 설정: Credential Holder 정의하기

### 1. 자격 증명 YAML 구조

자격 증명 소유자는 `~/.cloud-barista/credentials.yaml`에 정의합니다:

```yaml
credentialholder:
  admin:                    # 기본 소유자 (전체 권한)
    aws:
      aws_access_key_id: AKIA...
      aws_secret_access_key: wJal...
    gcp:
      client_id: "107777777600845725910"
      client_email: svc@project.iam.gserviceaccount.com
      private_key_id: f89f5asfsesefsefsfefes0se0fse0f00ef565e33
      private_key: "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqh...iH0ew=\n-----END PRIVATE KEY-----\n"
      project_id: my-project
      S3AccessKey: GOOGGOOGLGGOOOSSSSSSSSSSSSSSSSSSSS
      S3SecretKey: wUsoOOooOO/ooooo3oO3o3o3o4ogo3o+OOo4oOOo
    azure:
      clientId: 12345-...
      clientSecret: secret...
      tenantId: tenant-...
      subscriptionId: sub-...

  role01:                   # 제한된 소유자 (AWS 전용)
    aws:
      aws_access_key_id: AKIA...     # 다른 AWS 계정
      aws_secret_access_key: xYzA...

  team_a:                   # 팀별 소유자 (주의: 하이픈이 아닌 밑줄 사용)
    aws:
      aws_access_key_id: AKIA...
      aws_secret_access_key: bCdE...
    gcp:
      client_id: "207777777600845725911"
      client_email: team-a@project.iam.gserviceaccount.com
      private_key_id: a12b3cdfsesefsefsfefes0se0fse0f00ef565e44
      private_key: "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqh...iH0ew=\n-----END PRIVATE KEY-----\n"
      project_id: team-a-project
      S3AccessKey: GOOGGOOGLGGOOOSSSSSSSSSSSSSSSSSSTT
      S3SecretKey: xVtoOOooOO/ooooo3oO3o3o3o4ogo3o+OOo4oOOo
```

### 2. 커스텀 OpenStack 기반 CSP 추가하기

OpenStack 기반의 프라이빗 클라우드(예: OpenStack으로 구축된 자체 클라우드 플랫폼)를 사용하는 경우, 다음 절차를 통해 새로운 제공자로 추가할 수 있습니다:

1. `credentials.yaml`에 `openstack`과 동일한 필드를 가진 자격 증명 섹션을 추가합니다:

```yaml
credentialholder:
  admin:
    openstack-new01:
      IdentityEndpoint: http://your-openstack-endpoint:5000
      Username:
      Password:
      DomainName:
      ProjectID:
```

2. `cloudinfo.yaml`에 `cloudPlatform: openstack`이 포함된 항목을 추가합니다.
   (자세한 내용은 `cloudinfo.yaml` 내의 `openstack-ex01` 예제를 참조하십시오.)

### 3. 초기화 프로세스

```bash
# 1. 자격 증명 템플릿 생성
make gen-cred

# 2. CSP 키 입력 및 편집 (필요에 따라 여러 소유자 추가)
vi ~/.cloud-barista/credentials.yaml

# 3. 안전한 보관을 위해 암호화 수행
make enc-cred

# 4. CB-Tumblebug 초기화 (모든 소유자 등록)
make init
```

`make init` 실행 중에 초기화 스크립트(`init.py`)는 다음 작업을 수행합니다:
1. `credentials.yaml.enc` 파일을 복호화합니다.
2. `credentialholder` 키 아래의 모든 소유자를 파싱합니다.
3. 각 소유자에 대해 `POST /tumblebug/credential`을 호출하여 제공자별 자격 증명을 등록합니다.
4. 각 등록 작업은 모든 리전에 대한 연결 구성을 자동으로 생성하고 검증합니다.

```mermaid
graph TB
    YAML[credentials.yaml.enc]
    INIT[init.py]
    
    subgraph "소유자별 등록"
        REG_ADMIN["admin 등록:<br/>aws, gcp, azure"]
        REG_ROLE01["role01 등록:<br/>aws"]
        REG_TEAM["team_a 등록:<br/>aws, gcp"]
    end

    subgraph "자동 생성된 연결 (Connections)"
        CONN_ADMIN["admin 연결:<br/>aws-ap-northeast-2<br/>aws-us-east-1<br/>gcp-asia-northeast3<br/>azure-koreacentral<br/>..."]
        CONN_ROLE01["role01 연결:<br/>role01-aws-ap-northeast-2<br/>role01-aws-us-east-1<br/>..."]
        CONN_TEAM["team_a 연결:<br/>team_a-aws-ap-northeast-2<br/>team_a-gcp-us-central1<br/>..."]
    end
    
    YAML --> INIT
    INIT --> REG_ADMIN
    INIT --> REG_ROLE01
    INIT --> REG_TEAM
    
    REG_ADMIN --> CONN_ADMIN
    REG_ROLE01 --> CONN_ROLE01
    REG_TEAM --> CONN_TEAM
    
    style YAML fill:#f5f5f5
    style CONN_ADMIN fill:#e1f5ff
    style CONN_ROLE01 fill:#fff4e1
    style CONN_TEAM fill:#ffe1f5
```

## 기본 자격 증명 소유자 (Default Credential Holder)

기본 자격 증명 소유자는 환경 변수를 통해 구성할 수 있습니다:

```bash
# conf/setup.env 또는 docker-compose.yaml 내 설정
TB_DEFAULT_CREDENTIALHOLDER=admin  # 기본값
```

**기본 소유자 동작 특징:**
- `X-Credential-Holder` 헤더가 **없는 경우**, 기본 소유자가 사용됩니다.
- 기본 소유자의 연결은 소유자 접두사가 없는 **짧은 이름**을 사용합니다 (예: `aws-ap-northeast-2`).
- API 응답에서 기본 소유자의 `isDefault` 필드는 `true`로 표시됩니다.
- 기본 소유자에 대해서는 스펙 추천 시 제공자별 필터링을 수행하지 **않습니다** (모든 CSP 표시).

## 멀티테넌트 활용 사례

### 활용 사례 1: 팀 간 격리 (Team Isolation)

서로 다른 팀이 별도의 자격 증명 소유자를 통해 각기 다른 클라우드 계정을 사용합니다:

```mermaid
graph TB
    subgraph "Team Alpha"
        USER_A[개발자 A]
        USER_A -->|X-Credential-Holder: team_alpha| TB
    end

    subgraph "Team Beta"
        USER_B[개발자 B]
        USER_B -->|X-Credential-Holder: team_beta| TB
    end

    TB[CB-Tumblebug]

    TB -->|team_alpha-aws-us-east-1| AWS_A[AWS 계정 A]
    TB -->|team_beta-aws-us-east-1| AWS_B[AWS 계정 B]
    TB -->|team_beta-gcp-us-central1| GCP_B[GCP 프로젝트 B]
    
    style AWS_A fill:#fff4e1
    style AWS_B fill:#e1f5ff
    style GCP_B fill:#e1f5ff
```

### 활용 사례 2: 역할 기반 접근 제어 (Role-Based Access)

특정 역할(Role)이 사용할 수 있는 CSP를 제한합니다:

| 소유자 (Holder) | 사용 가능한 CSP | 활용 사례 |
|--------|---------------|----------|
| `admin` | AWS, GCP, Azure, Alibaba, NHN, ... | 전체 시스템 관리 |
| `developer` | AWS, GCP | 개발 및 테스트 |
| `auditor` | AWS (읽기 전용 계정) | 컴플라이언스 감사 |
| `demo` | AWS (쿼터 제한 계정) | 데모 시연 |

### 활용 사례 3: 환경 분리 (Environment Separation)

완전한 격리를 위해 네임스페이스(Namespace)와 조합하여 사용합니다:

```
Namespace: production  +  Holder: prod_admin   → 프로덕션 AWS/GCP 계정
Namespace: staging     +  Holder: staging_ops  → 스테이징 AWS 계정
Namespace: development +  Holder: dev_team     → 개발 샌드박스 계정
```

> **Namespace vs Credential Holder:** 네임스페이스는 **리소스**(Node, 네트워크 등)를 격리합니다. 반면 자격 증명 소유자는 **클라우드 계정**(API 키, CSP 접근 권한)을 격리합니다. 두 개념은 서로 독립적(Orthogonal)이므로 자유롭게 조합하여 사용할 수 있습니다.

## 데이터 모델 요약

```mermaid
erDiagram
    CREDENTIAL_HOLDER ||--o{ CREDENTIAL : owns
    CREDENTIAL ||--o{ CONNECTION : generates
    CONNECTION }o--|| CSP_REGION : targets
    CONNECTION }o--|| SPIDER_CREDENTIAL : uses
    
    CREDENTIAL_HOLDER {
        string credentialHolder PK "e.g., admin, role01"
        list providers "e.g., [aws, gcp]"
        int connectionCount "Total connections"
        int verifiedConnectionCount "Verified connections"
        bool isDefault "Is system default"
    }
    
    CREDENTIAL {
        string credentialName PK "e.g., role01-aws"
        string credentialHolder FK "Owner holder"
        string providerName "Cloud provider"
        list keyValueInfoList "API keys (encrypted)"
    }
    
    CONNECTION {
        string configName PK "e.g., role01-aws-ap-northeast-2"
        string providerName "Cloud provider"
        string credentialHolder "Owner holder"
        string credentialName "Associated credential"
        string regionZoneInfoName "Region in Spider"
        bool verified "Connectivity verified"
        bool regionRepresentative "Region representative"
    }
```

## 주요 구현 파일

| 파일 | 목적 |
|------|---------|
| [src/core/model/common.go](../../../src/core/model/common.go) | 데이터 구조체: `ConnConfig`, `CredentialReq`, `CredentialHolderInfo` |
| [src/core/common/context.go](../../../src/core/common/context.go) | 컨텍스트 헬퍼: `WithCredentialHolder`, `CredentialHolderFromContext` |
| [src/core/common/utility.go](../../../src/core/common/utility.go) | `RegisterCredential`, `ResolveConnectionName`, `GetCredentialHolder`, `GetConnConfigList` |
| [src/interface/rest/server/server.go](../../../src/interface/rest/server/server.go) | 미들웨어: `X-Credential-Holder` 헤더 추출 및 컨텍스트 주입 |
| [src/core/common/credential.go](../../../src/core/common/credential.go) | 자격 증명 보안을 위한 RSA 키 생성 및 AES 복호화 |
| [init/init.py](../../../init/init.py) | 초기화: `credentials.yaml`로부터 다중 소유자 자격 증명 등록 |
| [init/template.credentials.yaml](../../../init/template.credentials.yaml) | 자격 증명 소유자 YAML 구조를 위한 템플릿 |

## 자주 묻는 질문 (FAQ)

**Q: `X-Credential-Holder`를 설정하지 않으면 어떻게 되나요?**
A: 기본 소유자(`admin`)가 자동으로 사용됩니다. 기존의 모든 동작이 그대로 유지되므로 하위 호환성이 완벽하게 보장됩니다.

**Q: 소유자 이름에 허용되는 문자는 무엇인가요?**
A: 영문 소문자, 숫자, 밑줄(`[a-z0-9_]`)만 허용됩니다. 하이픈(`-`)은 연결 이름에서 구분자로 사용되므로 **허용되지 않습니다**. 예를 들어 `team-a` 대신 `team_a` 또는 `role01`을 사용하십시오.

**Q: 서버를 재시작하지 않고 새로운 자격 증명 소유자를 추가할 수 있나요?**
A: 예, 가능합니다. 새로운 `credentialHolder` 값을 포함하여 `POST /tumblebug/credential`을 호출하면 연결이 자동으로 생성되며 해당 소유자를 즉시 사용할 수 있습니다.

**Q: 두 소유자가 동일한 CSP 계정을 공유할 수 있나요?**
A: 예, 가능합니다. 서로 다른 소유자가 동일한 CSP API 키를 등록할 수 있습니다. 이 경우 각 소유자는 별도의 연결 구성을 갖게 되지만 실제로는 동일한 클라우드 계정을 가리키게 됩니다.

**Q: 자격 증명 소유자는 스펙 추천에 어떤 영향을 미치나요?**
A: 기본 소유자가 아닌 경우, `POST /recommendSpec` 호출 시 해당 소유자에게 등록된 CSP의 스펙만 포함되도록 결과가 자동 필터링됩니다. 예를 들어 `role01`에 AWS 자격 증명만 있다면 AWS 스펙만 반환됩니다.

**Q: 자격 증명 소유자와 네임스페이스는 같은 개념인가요?**
A: 아닙니다. **네임스페이스**(Namespace)는 리소스(Node, VNet 등)를 격리하고, **자격 증명 소유자**(Credential Holder)는 클라우드 계정(API 키, 연결 구성)을 격리합니다. 두 개념은 서로 독립적이며, 어떤 네임스페이스에서든 어떤 자격 증명 소유자라도 조합하여 사용할 수 있습니다.
