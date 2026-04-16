# CB-Tumblebug Credential, Credential Holder, and Connection Management

## Overview

CB-Tumblebug manages **multi-cloud credentials** and **connections** to enable unified infrastructure provisioning across diverse cloud providers. This document explains three core concepts:

- **Credential**: CSP API keys/secrets used to authenticate with cloud providers
- **Credential Holder**: A logical identity that owns a set of credentials (e.g., `admin`, `team_a`, `role01`)
- **Connection (ConnConfig)**: A fully resolved link between a credential and a specific CSP region, ready for resource operations

These concepts work together to support **multi-tenant credential isolation** — different teams or roles can operate on different sets of cloud accounts through a single CB-Tumblebug instance.

## Core Concepts

### 1. Credential

A **Credential** is a set of CSP-specific API keys required for authentication. Each credential is associated with one cloud provider and one credential holder.

| CSP | Required Keys |
|-----|--------------|
| AWS | `aws_access_key_id`, `aws_secret_access_key` |
| Azure | `clientId`, `clientSecret`, `tenantId`, `subscriptionId` |
| GCP | `client_id`, `client_email`, `private_key_id`, `private_key`, `project_id`, `S3AccessKey`, `S3SecretKey` |
| Alibaba | `AccessKeyId`, `AccessKeySecret` |
| IBM | `ApiKey`, `S3AccessKey`, `S3SecretKey` |
| KT | `IdentityEndpoint`, `Username`, `Password`, `DomainName`, `ProjectID`, `S3AccessKey`, `S3SecretKey` |
| NCP | `ncloud_access_key`, `ncloud_secret_key` |
| NHN | `IdentityEndpoint`, `Username`, `Password`, `DomainName`, `TenantId`, `S3AccessKey`, `S3SecretKey` |
| OpenStack | `IdentityEndpoint`, `Username`, `Password`, `DomainName`, `ProjectID` |
| Tencent | `SecretId`, `SecretKey` |

**Security:** Credentials are never stored in plaintext. They undergo a hybrid encryption workflow:
1. Client encrypts credential values with a temporary AES key
2. AES key itself is encrypted with the server's RSA public key
3. Server decrypts AES key with RSA private key, then decrypts credential values
4. Values are forwarded to CB-Spider for CSP registration

### 2. Credential Holder

A **Credential Holder** is a logical identity that **owns and isolates** a group of credentials and their resulting connections. It enables multi-tenant credential management within a single CB-Tumblebug instance.

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

**Key Characteristics:**
- Each holder can have credentials for **different sets of CSPs** (e.g., `admin` has AWS+GCP+Azure, `role01` has AWS only)
- The **default holder** (`admin`) is the system default — when no holder is specified, `admin` is used
- Holder names are **case-insensitive** and stored in lowercase
- **Hyphens (`-`) are not allowed** in holder names — only lowercase alphanumeric characters and underscores (`[a-z0-9_]`) are permitted. This restriction exists because hyphens are used as delimiters in connection naming (e.g., `role01-aws-ap-northeast-2`). If hyphens were allowed (e.g., `team-a`), connection names would become ambiguous (`team-a-aws-...` — is the holder `team-a` or `team`?)
- A holder is **not explicitly created** — it emerges automatically when credentials are registered under that holder name

### 3. Connection (ConnConfig)

A **Connection** (internally `ConnConfig`) is the fully resolved configuration that links a credential to a specific CSP region. It represents a "ready-to-use" cloud connection endpoint.

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

**Connection Naming Convention:**

| Holder Type | Pattern | Example |
|-------------|---------|---------|
| Default holder (`admin`) | `{provider}-{region}` | `aws-ap-northeast-2` |
| Non-default holder | `{holder}-{provider}-{region}` | `role01-aws-ap-northeast-2` |

This naming convention is critical — it is how CB-Tumblebug resolves which CSP account to use for any given operation.

## Relationship Between Concepts

```mermaid
graph LR
    subgraph "credentials.yaml"
        YAML["credentialholder:<br/>  admin:<br/>    aws: {keys...}<br/>    gcp: {keys...}<br/>  role01:<br/>    aws: {keys...}"]
    end
    
    subgraph "CB-Tumblebug"
        CRED[Credential<br/>per holder + per provider]
        CONN[Connection Configs<br/>per holder + per provider + per region]
        HOLDER[Credential Holder<br/>derived from connections]
    end
    
    subgraph "CB-Spider"
        SP_CRED[Spider Credential]
        SP_REGION[Spider Region]
        SP_CONN[Spider ConnConfig]
    end
    
    YAML -->|init.py| CRED
    CRED -->|RegisterCredential| SP_CRED
    CRED -->|auto-generate per region| CONN
    CONN -->|RegisterConnectionConfig| SP_CONN
    SP_CRED --> SP_CONN
    SP_REGION --> SP_CONN
    CONN -->|aggregated by holder| HOLDER
    
    style YAML fill:#f5f5f5
    style CRED fill:#e1f5ff
    style CONN fill:#fff4e1
    style HOLDER fill:#ffe1f5
```

### Registration Flow

When a credential is registered for a holder + provider, CB-Tumblebug automatically:

1. **Registers the credential** in CB-Spider (one credential per holder+provider)
2. **Discovers all regions** defined in `cloudinfo.yaml` for that provider
3. **Creates a ConnConfig** for each region (one connection per holder+provider+region)
4. **Verifies connectivity** by testing each ConnConfig against the CSP API
5. **Selects region representatives** for regions with multiple zones

```mermaid
sequenceDiagram
    participant User
    participant TB as CB-Tumblebug
    participant Spider as CB-Spider
    participant CSP as Cloud Provider
    
    User->>TB: POST /credential<br/>{credentialHolder: "role01",<br/> providerName: "aws", ...}
    
    TB->>Spider: Register credential<br/>(name: "role01-aws")
    Spider-->>TB: OK
    
    loop For each region in cloudinfo.yaml
        TB->>Spider: Register connection config<br/>(name: "role01-aws-{region}")
        Spider-->>TB: OK
    end
    
    par Verify connections concurrently
        TB->>Spider: Check "role01-aws-ap-northeast-2"
        Spider->>CSP: DescribeRegions()
        CSP-->>Spider: OK
        Spider-->>TB: Verified ✓
    and
        TB->>Spider: Check "role01-aws-us-east-1"
        Spider->>CSP: DescribeRegions()
        CSP-->>Spider: OK
        Spider-->>TB: Verified ✓
    end
    
    TB-->>User: CredentialInfo with all connections
```

## Credential Holder in Action

### 1. How Credential Holder is Specified

The credential holder is communicated via the **`X-Credential-Holder`** HTTP header:

```bash
# Use default holder (admin) — header can be omitted
curl -X POST http://localhost:1323/tumblebug/ns/default/infraDynamic \
  -H "Content-Type: application/json" \
  -d '{ ... }'

# Use a specific holder
curl -X POST http://localhost:1323/tumblebug/ns/default/infraDynamic \
  -H "Content-Type: application/json" \
  -H "X-Credential-Holder: role01" \
  -d '{ ... }'
```

### 2. Request Processing Pipeline

```mermaid
sequenceDiagram
    participant Client
    participant MW as Middleware
    participant Handler as REST Handler
    participant Core as Core Function
    participant Spider as CB-Spider

    Client->>MW: POST /ns/default/infraDynamic<br/>X-Credential-Holder: role01
    
    MW->>MW: Extract header<br/>holder = "role01"<br/>(default: "admin" if absent)
    MW->>MW: Inject into context.Context<br/>WithCredentialHolder(ctx, "role01")
    MW->>Handler: Pass enriched context
    
    Handler->>Core: CreateInfraDynamic(ctx, nsId, req)
    
    Core->>Core: holder = CredentialHolderFromContext(ctx)<br/>→ "role01"
    Core->>Core: connectionName = ResolveConnectionName(<br/>"aws-ap-northeast-2", "role01")<br/>→ "role01-aws-ap-northeast-2"
    
    Core->>Spider: Create Node via<br/>"role01-aws-ap-northeast-2"
    Spider-->>Core: Node created
    Core-->>Handler: Infra result
    Handler-->>Client: Response
```

### 3. Connection Name Resolution

The `ResolveConnectionName` function converts a default holder's connection name to the appropriate name for the active credential holder:

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

**Examples:**

| Input Connection | Credential Holder | Result |
|-----------------|-------------------|--------|
| `aws-ap-northeast-2` | `admin` (default) | `aws-ap-northeast-2` |
| `aws-ap-northeast-2` | `role01` | `role01-aws-ap-northeast-2` |
| `gcp-us-central1` | `team_a` | `team_a-gcp-us-central1` |

### 4. Context Propagation

Credential holder flows through the system via Go's `context.Context`, avoiding parameter drilling:

```go
// Middleware sets it
ctx = common.WithCredentialHolder(ctx, holder)

// Any core function extracts it
credentialHolder := common.CredentialHolderFromContext(ctx)

// Internal/system calls use default
ctx := common.NewDefaultContext() // → admin holder
```

| Layer | Mechanism |
|-------|-----------|
| HTTP Request | `X-Credential-Holder` header |
| Middleware | Reads header → injects into `context.Context` |
| REST Handler | Passes `c.Request().Context()` to core function |
| Core Function | `CredentialHolderFromContext(ctx)` extracts holder |
| Connection Lookup | `ResolveConnectionName()` builds holder-specific connection name |
| CB-Spider Call | Uses the resolved connection name for CSP operations |

### 5. Credential Holder Impact on APIs

The credential holder affects multiple API behaviors:

| Capability | Effect |
|-----------|--------|
| **Infra Provisioning** | Nodes are created using the holder's CSP accounts |
| **Resource Creation** | VNet, SecurityGroup, SSHKey use holder-specific connections |
| **Spec Recommendation** | Results are automatically filtered to the holder's available CSPs |
| **Connection Listing** | `GET /connConfig` can be filtered by `filterCredentialHolder` query param |
| **Image Search** | Uses holder's connections for CSP image lookups |

## Credential Holder API

### List All Credential Holders

```bash
GET /tumblebug/credentialHolder
```

**Response:**
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

### Get Specific Credential Holder

```bash
GET /tumblebug/credentialHolder/{holderId}
```

**Response:**
```json
{
  "credentialHolder": "role01",
  "providers": ["aws"],
  "connectionCount": 14,
  "verifiedConnectionCount": 12,
  "isDefault": false
}
```

> **Note:** Credential holders are **derived** from registered connection configs, not explicitly created. The API aggregates connection data to produce holder summaries.

## Setup: Defining Credential Holders

### 1. Credential YAML Structure

Credential holders are defined in `~/.cloud-barista/credentials.yaml`:

```yaml
credentialholder:
  admin:                    # Default holder (full access)
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

  role01:                   # Restricted holder (AWS only)
    aws:
      aws_access_key_id: AKIA...     # Different AWS account
      aws_secret_access_key: xYzA...

  team_a:                   # Team-specific holder (note: use underscore, not hyphen)
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

### 2. Adding a Custom OpenStack-Based CSP

If you use a private cloud based on OpenStack (e.g., a proprietary cloud platform built on OpenStack), you can add it as a new provider by:

1. Adding a credential section in `credentials.yaml` with the same fields as `openstack`:

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

2. Adding a corresponding entry in `cloudinfo.yaml` with `cloudPlatform: openstack`.
   (See the `openstack-ex01` example in `cloudinfo.yaml` for reference.)

### 3. Initialization Process

```bash
# 1. Generate credential template
make gen-cred

# 2. Edit with your CSP keys (add multiple holders as needed)  
vi ~/.cloud-barista/credentials.yaml

# 3. Encrypt for secure storage
make enc-cred

# 4. Initialize CB-Tumblebug (registers all holders)
make init
```

During `make init`, the initialization script (`init.py`):
1. Decrypts `credentials.yaml.enc`
2. Parses all holders under the `credentialholder` key
3. For each holder, registers credentials for each provider via `POST /tumblebug/credential`
4. Each registration auto-creates and verifies connection configs for all regions

```mermaid
graph TB
    YAML[credentials.yaml.enc]
    INIT[init.py]
    
    subgraph "Registration per holder"
        REG_ADMIN["Register admin:<br/>aws, gcp, azure"]
        REG_ROLE01["Register role01:<br/>aws"]
        REG_TEAM["Register team_a:<br/>aws, gcp"]
    end

    subgraph "Auto-generated connections"
        CONN_ADMIN["admin connections:<br/>aws-ap-northeast-2<br/>aws-us-east-1<br/>gcp-asia-northeast3<br/>azure-koreacentral<br/>..."]
        CONN_ROLE01["role01 connections:<br/>role01-aws-ap-northeast-2<br/>role01-aws-us-east-1<br/>..."]
        CONN_TEAM["team_a connections:<br/>team_a-aws-ap-northeast-2<br/>team_a-gcp-us-central1<br/>..."]
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

## Default Credential Holder

The default credential holder is configurable via environment variable:

```bash
# In conf/setup.env or docker-compose.yaml
TB_DEFAULT_CREDENTIALHOLDER=admin  # default value
```

**Default holder behavior:**
- When `X-Credential-Holder` header is **absent**, the default holder is used
- Default holder's connections use **short names** without holder prefix (e.g., `aws-ap-northeast-2`)
- Default holder's `isDefault` field is `true` in API responses
- Spec recommendation does **not** filter by provider for the default holder (shows all CSPs)

## Multi-Tenant Use Cases

### Use Case 1: Team Isolation

Different teams use different cloud accounts via separate credential holders:

```mermaid
graph TB
    subgraph "Team Alpha"
        USER_A[Developer A]
        USER_A -->|X-Credential-Holder: team_alpha| TB
    end

    subgraph "Team Beta"
        USER_B[Developer B]
        USER_B -->|X-Credential-Holder: team_beta| TB
    end

    TB[CB-Tumblebug]

    TB -->|team_alpha-aws-us-east-1| AWS_A[AWS Account A]
    TB -->|team_beta-aws-us-east-1| AWS_B[AWS Account B]
    TB -->|team_beta-gcp-us-central1| GCP_B[GCP Project B]
    
    style AWS_A fill:#fff4e1
    style AWS_B fill:#e1f5ff
    style GCP_B fill:#e1f5ff
```

### Use Case 2: Role-Based Access

Restrict which CSPs are available to certain roles:

| Holder | Available CSPs | Use Case |
|--------|---------------|----------|
| `admin` | AWS, GCP, Azure, Alibaba, NHN, ... | Full system administration |
| `developer` | AWS, GCP | Development and testing |
| `auditor` | AWS (read-only account) | Compliance auditing |
| `demo` | AWS (limited quota account) | Demonstrations |

### Use Case 3: Environment Separation

Combined with namespaces for complete isolation:

```
Namespace: production  +  Holder: prod-admin   → Production AWS/GCP accounts
Namespace: staging     +  Holder: staging-ops  → Staging AWS account
Namespace: development +  Holder: dev-team     → Development sandbox accounts
```

> **Namespace vs Credential Holder:** Namespaces isolate **resources** (Nodes, networks, etc.). Credential holders isolate **cloud accounts** (API keys, CSP access). They are orthogonal concepts and can be combined freely.

## Data Model Summary

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

## Key Implementation Files

| File | Purpose |
|------|---------|
| [src/core/model/common.go](../../src/core/model/common.go) | Data structures: `ConnConfig`, `CredentialReq`, `CredentialHolderInfo` |
| [src/core/common/context.go](../../src/core/common/context.go) | Context helpers: `WithCredentialHolder`, `CredentialHolderFromContext` |
| [src/core/common/utility.go](../../src/core/common/utility.go) | `RegisterCredential`, `ResolveConnectionName`, `GetCredentialHolder`, `GetConnConfigList` |
| [src/interface/rest/server/server.go](../../src/interface/rest/server/server.go) | Middleware: `X-Credential-Holder` header extraction and context injection |
| [src/core/common/credential.go](../../src/core/common/credential.go) | RSA key generation, AES decryption for credential security |
| [init/init.py](../../init/init.py) | Initialization: multi-holder credential registration from `credentials.yaml` |
| [init/template.credentials.yaml](../../init/template.credentials.yaml) | Template for credential holder YAML structure |

## FAQ

**Q: What happens if I don't set `X-Credential-Holder`?**
A: The default holder (`admin`) is used automatically. All existing behavior is preserved — this is backward compatible.

**Q: What characters are allowed in holder names?**
A: Only lowercase alphanumeric characters and underscores: `[a-z0-9_]`. Hyphens (`-`) are **not allowed** because they serve as delimiters in connection naming. For example, use `team_a` or `role01` instead of `team-a`.

**Q: Can I add a new credential holder without restarting?**
A: Yes. Use `POST /tumblebug/credential` with the new `credentialHolder` value. Connections are auto-created and the holder becomes immediately available.

**Q: Can two holders share the same CSP account?**
A: Yes. Different holders can register the same CSP API keys. They will have separate connection configs but point to the same cloud account.

**Q: How does credential holder affect spec recommendation?**
A: For non-default holders, `POST /recommendSpec` automatically filters results to only include specs from the holder's registered CSPs. For example, if `role01` only has AWS credentials, only AWS specs are returned.

**Q: Is credential holder the same as namespace?**
A: No. **Namespace** isolates resources (Nodes, VNets, etc.). **Credential holder** isolates cloud accounts (API keys, connection configs). They are independent and can be combined: any namespace can use any credential holder.
