# CB-Tumblebug `make init` 초기화 워크플로우

## 개요

`make init`은 CB-Tumblebug의 기본 초기화 명령어입니다. 이 명령어는 다음 두 단계의 프로세스를 오케스트레이션합니다:

1. **1단계 — OpenBao 자격 증명 등록 (Phase 1 — OpenBao Credential Registration)**: `credentials.yaml.enc`를 복호화하고 CSP 자격 증명을 OpenBao(Vault 호환 시크릿 매니저)에 저장하여 MC-Terrarium의 OpenTofu/Terraform 템플릿에서 사용할 수 있도록 합니다.
2. **2단계 — Tumblebug 초기화 (Phase 2 — Tumblebug Initialization)**: 동일한 자격 증명을 (엔드투엔드 하이브리드 암호화를 사용하여) CB-Tumblebug에 등록한 다음, 클라우드 에셋 데이터(Compute 스펙, OS 이미지, 가격 정보)와 인프라 템플릿을 시스템에 로드합니다.

`make init`이 완료되면 CB-Tumblebug는 멀티클라우드 인프라 프로비저닝을 위한 모든 준비를 마치고 완전히 작동 가능한 상태가 됩니다.

---

## 0단계 — Docker Compose 및 `.env`

`make init`을 실행하려면 모든 서비스가 이미 실행 중이어야 합니다(`make up`). 서비스가 시작되기 전에 Docker Compose는 프로젝트 루트에 있는 `.env` 파일을 읽어 각 컨테이너에 환경 변수를 주입합니다.

### `.env`가 로드되는 방식

Docker Compose는 프로젝트 루트 디렉토리(`docker-compose.yaml`이 위치한 곳)에서 `.env`를 자동으로 읽어들입니다. 명시적으로 참조할 필요 없이 관례에 따라 자동으로 로드됩니다.

```bash
# Copy the example file and fill in your values
cp .env.example .env
# (edit .env)
make up
```

### `.env` 변수 맵

아래 다이어그램은 어떤 `.env` 변수가 어떤 컨테이너로 전달되는지, 그리고 변수가 누락되었을 때 어떻게 동작하는지를 보여줍니다.

```mermaid
graph TD
    EnvFile[".env\n(project root)"]

    TB_svc["cb-tumblebug\nTB_API_USERNAME / TB_API_PASSWORD\nTERRARIUM_API_USERNAME / TERRARIUM_API_PASSWORD\nVAULT_TOKEN"]
    Spider_svc["cb-spider\nSPIDER_USERNAME / SPIDER_PASSWORD"]
    Terrarium_svc["mc-terrarium\nTERRARIUM_API_USERNAME / TERRARIUM_API_PASSWORD\nVAULT_TOKEN"]
    OpenBao_svc["openbao\n(no .env vars injected)\nVAULT_TOKEN written back by init-openbao.sh"]
    Compose_meta["Docker Compose\nCOMPOSE_FILE"]

    EnvFile -->|interpolation| TB_svc
    EnvFile -->|interpolation| Spider_svc
    EnvFile -->|interpolation| Terrarium_svc
    EnvFile -.->|"no injection\n(token written back)"| OpenBao_svc
    EnvFile -->|compose config| Compose_meta

    style EnvFile fill:#fff8e1,stroke:#f57f17,stroke-width:2px
    style TB_svc fill:#fff0e0,stroke:#e07000
    style Spider_svc fill:#f3e5f5,stroke:#7b1fa2
    style Terrarium_svc fill:#f3e5f5,stroke:#7b1fa2
    style OpenBao_svc fill:#fce4ec,stroke:#c62828
    style Compose_meta fill:#eceff1,stroke:#607d8b
```

### 변수 레퍼런스

| 변수 | 사용하는 컨테이너 | 누락 시 동작 | 설정 주체 |
|---|---|---|---|
| `COMPOSE_FILE` | Docker Compose (메타) | 기본값으로 `docker-compose.yaml`만 사용 | 사용자 (Traefik 오버레이 선택) |
| `TB_API_USERNAME` | `cb-tumblebug` | **에러 — 시작 실패** | 사용자 |
| `TB_API_PASSWORD` | `cb-tumblebug` | **에러 — 시작 실패** | 사용자 |
| `SPIDER_USERNAME` | `cb-spider` | **에러 — 시작 실패** | 사용자 |
| `SPIDER_PASSWORD` | `cb-spider` | **에러 — 시작 실패** | 사용자 |
| `TERRARIUM_API_USERNAME` | `cb-tumblebug`, `mc-terrarium` | **에러 — 시작 실패** | 사용자 |
| `TERRARIUM_API_PASSWORD` | `cb-tumblebug`, `mc-terrarium` | **에러 — 시작 실패** | 사용자 |
| `VAULT_TOKEN` | `cb-tumblebug`, `mc-terrarium` | 빈 문자열 (조용한 기본값 `:-`) | 첫 `make up` 이후 `init-openbao.sh`가 자동으로 기록 |
| `VAULT_ADDR` | (호스트 측 참조용) | `http://localhost:8200` | 사용자 (거의 변경하지 않음) |

> **`VAULT_TOKEN`에 대한 참고 사항**: `:?` 변수들은 정의되지 않은 경우 명확한 에러 메시지와 함께 `docker compose up`을 중단시킵니다. 반면 `VAULT_TOKEN`은 첫 시작 시 `init-openbao.sh`에 의해 `.env`에 자동으로 기록되므로 수동으로 설정할 필요가 없으며, 따라서 `:-`(빈 기본값)를 사용합니다.

### `COMPOSE_FILE` — Traefik 활성화하기

`COMPOSE_FILE`은 시작 시 어떤 Compose 파일들을 병합할지 제어하는 특별한 Docker Compose 변수입니다:

```bash
# Default: only core services
COMPOSE_FILE=docker-compose.yaml

# With Traefik reverse proxy (HTTPS, routing):
COMPOSE_FILE=docker-compose.yaml:docker-compose.traefik.yaml
```

### OpenBao 및 `VAULT_TOKEN` 수명 주기

`VAULT_TOKEN`은 `.env.example`에서 비어 있는 상태로 시작합니다. 첫 번째 `make up` 실행 시 `init-openbao.sh`가 OpenBao를 초기화하고 루트 토큰을 획득한 뒤 **이를 `.env`에 자동으로 다시 기록합니다**. 이후 실행되는 `make up`과 `make init`은 별도의 수동 작업 없이도 이 토큰을 자동으로 읽어 사용합니다.

```mermaid
sequenceDiagram
    actor User
    participant EnvFile as .env
    participant MakeUp as make up<br/>(docker compose up)
    participant OpenBao as openbao container
    participant InitScript as init-openbao.sh

    User->>MakeUp: make up (first run)
    MakeUp->>EnvFile: read VAULT_TOKEN (empty)
    MakeUp->>OpenBao: start container<br/>(VAULT_TOKEN="" injected)
    OpenBao-->>MakeUp: container healthy

    MakeUp->>InitScript: auto-run init-openbao.sh
    InitScript->>OpenBao: POST /v1/sys/init
    OpenBao-->>InitScript: root_token, unseal_keys
    InitScript->>OpenBao: PUT /v1/sys/unseal (× 3 keys)
    OpenBao-->>InitScript: unsealed=true
    InitScript->>EnvFile: write VAULT_TOKEN=<root_token>

    Note over EnvFile: .env now has VAULT_TOKEN set

    User->>MakeUp: make up (subsequent runs)
    MakeUp->>EnvFile: read VAULT_TOKEN (set)
    MakeUp->>OpenBao: start container<br/>(VAULT_TOKEN injected → mc-terrarium can auth)
```

---

## 전체 상호작용 다이어그램

아래 다이어그램은 `make init`이 처음부터 끝까지 각 컴포넌트와 어떻게 상호작용하는지 보여줍니다.

```mermaid
sequenceDiagram
    autonumber
    actor User
    box Local Host
        participant Script as multi-init.sh<br/>init.sh / init.py
        participant EncFile as credentials.yaml.enc<br/>(~/.cloud-barista/)
        participant Templates as init/templates/<br/>(Infra / vNet / SG)
    end

    box Docker Compose Environment
        participant OpenBao as OpenBao<br/>:8200<br/>KV v2 Secret Engine
        participant TB as CB-Tumblebug<br/>:1323
        participant Spider as CB-Spider<br/>:1024
        participant PG as PostgreSQL<br/>:5432<br/>Specs / Images DB
    end

    box Cloud Providers
        participant CSPs as AWS / GCP / Azure<br/>Alibaba / IBM / NCP<br/>NHN / KT / Others
    end

    User->>Script: make init (enter password once)

    rect rgb(230, 245, 255)
        Note over Script,OpenBao: ── Phase 1: OpenBao Credential Registration ──

        Script->>EncFile: Decrypt credentials.yaml.enc<br/>(AES-256-CBC, in-memory only)
        EncFile-->>Script: Plaintext credentials (YAML)

        Script->>OpenBao: GET /v1/sys/seal-status
        OpenBao-->>Script: initialized=true, sealed=false

        loop For each CredentialHolder × CSP provider
            Script->>OpenBao: POST /v1/secret/data/csp/{provider}<br/>(admin holder)<br/>or /v1/secret/data/users/{holder}/csp/{provider}
            OpenBao-->>Script: 200 OK – secret stored
        end

        Script->>OpenBao: POST placeholder secrets<br/>for providers without credentials
        OpenBao-->>Script: Placeholders stored
        Note over Script: Phase 1 complete – summary printed<br/>(registered / skipped / failed / placeholders)
    end

    rect rgb(255, 248, 230)
        Note over Script,CSPs: ── Phase 2: Tumblebug Initialization ──

        loop Poll until healthy (max 50 retries)
            Script->>TB: GET /tumblebug/readyz
            TB-->>Script: 200 OK
        end

        loop For each CredentialHolder × CSP provider
            Script->>TB: GET /tumblebug/credential/publicKey
            TB-->>Script: RSA public key + tokenId

            Note over Script: Hybrid encrypt credentials<br/>① Generate random AES-256 key<br/>② Encrypt credential values (AES-CBC)<br/>③ Encrypt AES key with RSA public key<br/>④ Base64-encode all

            Script->>TB: POST /tumblebug/credential<br/>{providerName, credentialHolder,<br/>encryptedKeyValueList, encryptedAESKey}
            TB->>TB: Decrypt AES key (RSA private key)<br/>Decrypt credential values (AES key)
            TB->>Spider: Register cloud connection configs<br/>(per region × credential)
            Spider->>CSPs: Verify connection (cloud API probe)
            CSPs-->>Spider: Connection verified / failed
            Spider-->>TB: per-region status
            TB-->>Script: ✓ verified / ✗ unverified regions
        end
    end

    rect rgb(240, 255, 240)
        Note over Script,CSPs: ── Asset Loading (user selects one option) ──

        alt Option A – Restore from backup (~1 min)
            Script->>TB: POST /tumblebug/ns  (ensure 'system' namespace)
            TB-->>Script: namespace ready
            Script->>PG: pg_restore assets/assets.dump.gz<br/>(specs + images + pricing)
            PG-->>Script: Restore complete
        else Option A+ – Restore from backup + patch CSV (~2 min)
            Script->>TB: POST /tumblebug/ns  (ensure 'system' namespace)
            TB-->>Script: namespace ready
            Script->>PG: pg_restore assets/assets.dump.gz<br/>(specs + images + pricing)
            PG-->>Script: Restore complete
            Script->>TB: POST /tumblebug/updateImagesFromAsset<br/>(apply latest cloudimage.csv on top)
            TB->>PG: Upsert images from CSV
            PG-->>TB: Updated
            TB-->>Script: patch complete
        else Option B – Fetch from CSPs, skip Azure (~20 min)
            Script->>TB: GET /tumblebug/loadAssets
            TB->>Spider: Fetch compute specs per region (parallel)
            TB->>Spider: Fetch OS images per region (parallel)
            Spider->>CSPs: Cloud API calls
            CSPs-->>Spider: Spec & image data
            Spider-->>TB: Aggregated data
            TB->>PG: Store specs and images
            PG-->>TB: Stored
            TB-->>Script: loadAssets complete
        else Option C – Fetch ALL CSPs including Azure (~40 min)
            Script->>TB: GET /tumblebug/loadAssets?includeAzure=true
            TB->>Spider: Fetch specs + images (all CSPs, parallel)
            Spider->>CSPs: Cloud API calls (incl. Azure)
            CSPs-->>Spider: Full data
            Spider-->>TB: Aggregated data
            TB->>PG: Store specs and images
            PG-->>TB: Stored
            TB-->>Script: loadAssets complete
        end

        opt Price fetching (Options B / C only, ~10 min, cancellable)
            Script->>TB: POST /tumblebug/fetchPrice
            TB->>CSPs: Fetch pricing APIs
            CSPs-->>TB: Pricing data
            TB->>PG: Update price per spec
            PG-->>TB: Updated
            TB-->>Script: fetchPrice complete
        end
    end

    rect rgb(255, 240, 255)
        Note over Script,TB: ── Template Loading ──

        loop For each JSON file in init/templates/
            Script->>TB: POST /tumblebug/ns  (ensure namespace)
            TB-->>Script: namespace ready
            Script->>TB: POST /tumblebug/ns/{nsId}/template/{type}<br/>(type: infra | vNet | securityGroup)
            TB-->>Script: template stored
        end
    end

    Script->>TB: PUT /tumblebug/readyz/init
    TB-->>Script: initialization marked complete
    Script-->>User: Init complete ✓<br/>(elapsed time summary)
```

---

## `make init` 실행 중 컴포넌트별 역할

| 컴포넌트 | `make init` 실행 중 역할 |
|-----------|------------------------|
| **multi-init.sh / init.py** | 오케스트레이터: 자격 증명을 복호화하고 모든 API를 호출하며 결과를 보고함 |
| **credentials.yaml.enc** | 암호화된 자격 증명 소스 — 메모리 내에서만 복호화되며 디스크에 평문으로 기록되지 않음 |
| **OpenBao** (`:8200`) | KV v2에 CSP 자격 증명을 저장함; 이후 MC-Terrarium의 OpenTofu 템플릿에서 사용됨 |
| **CB-Tumblebug** (`:1323`) | 암호화된 자격 증명을 수신하고 연결을 관리하며 에셋 로딩을 트리거하고 템플릿을 저장함 |
| **CB-Spider** (`:1024`) | 리전별 연결 구성을 등록하고 CSP API를 프로브(Probe)하여 연결 상태를 검증함 |
| **PostgreSQL** (`:5432`) | 로드된 Compute 스펙, OS 이미지 및 가격 데이터를 저장함 |
| **CSPs** | 연결 검증, Compute 스펙, OS 이미지 및 가격 정보에 대한 신뢰할 수 있는 원본(Source of truth) |

---

## 단계별 상세 설명

### 1단계 — OpenBao 자격 증명 등록 (Phase 1 — OpenBao Credential Registration)

```mermaid
flowchart TD
    Start([make init starts]) --> Decrypt

    subgraph Phase1["Phase 1: OpenBao Credential Registration"]
        Decrypt["Decrypt credentials.yaml.enc\n(AES-256-CBC, in-memory)"]
        CheckBao["GET /v1/sys/seal-status\nVerify OpenBao is initialized & unsealed"]
        Decrypt --> CheckBao

        CheckBao -->|sealed or error| ErrBao["Exit with error\n(run: make up first)"]
        CheckBao -->|OK| Loop1

        Loop1["For each CredentialHolder × CSP"]
        MapKeys["Map YAML keys → OpenTofu env var names\ne.g. aws_access_key_id → AWS_ACCESS_KEY_ID"]
        StoreBao["POST /v1/secret/data/...\nKV v2 path:\n• admin  → csp/{provider}\n• others → users/{holder}/csp/{provider}"]
        Loop1 --> MapKeys --> StoreBao --> Loop1

        StoreBao --> Placeholder["Store placeholder secrets\nfor missing CSP providers"]
        Placeholder --> Summary1["Print summary:\nregistered / skipped / failed / placeholders"]
    end

    Summary1 --> Phase2Start([Phase 2 begins])

    style Phase1 fill:#e6f2ff,stroke:#4a90d9
    style ErrBao fill:#ffe6e6,stroke:#cc0000
```

### 2단계 — Tumblebug 초기화 (Phase 2 — Tumblebug Initialization)

```mermaid
flowchart TD
    Start([Phase 2 begins]) --> Health

    subgraph Phase2["Phase 2: Tumblebug Initialization"]
        Health["GET /tumblebug/readyz\n(poll up to 50× with 1s interval)"]
        Health -->|unhealthy after 50 retries| ErrTB["Exit with error\n(run: make up first)"]
        Health -->|OK| CredLoop

        subgraph CredReg["Credential Registration (Hybrid Encryption)"]
            CredLoop["For each CredentialHolder × CSP provider"]
            GetKey["GET /tumblebug/credential/publicKey\n→ RSA public key + tokenId"]
            Encrypt["Client-side hybrid encrypt\n① Random AES-256 key\n② AES-CBC encrypt credential values\n③ RSA-OAEP encrypt AES key\n④ Base64-encode"]
            Post["POST /tumblebug/credential\n{providerName, credentialHolder,\n encryptedKeyValueList,\n encryptedClientAesKeyByPublicKey}"]
            Decrypt2["TB: RSA decrypt AES key\nAES decrypt credential values"]
            SpiderReg["CB-Spider: register connection configs\nper region (from cloudinfo.yaml)"]
            Probe["CB-Spider: probe CSP APIs\nper region → verify connectivity"]
            Report["Display per-region status:\n✓ verified / ✗ unverified / ~ mixed"]
            CredLoop --> GetKey --> Encrypt --> Post --> Decrypt2 --> SpiderReg --> Probe --> Report --> CredLoop
        end

        subgraph AssetLoad["Asset Loading"]
            Choice{User choice}
            OptA["Option A: Restore from backup\n~1 min\npg_restore assets.dump.gz\n→ specs + images + pricing"]
            OptAPlus["Option A+: Restore from backup + patch CSV\n~2 min\npg_restore assets.dump.gz\n→ POST /tumblebug/updateImagesFromAsset\n(apply latest cloudimage.csv on top)"]
            OptB["Option B: Fetch from CSPs\n~20 min (no Azure)\nGET /tumblebug/loadAssets"]
            OptC["Option C: Fetch ALL CSPs\n~40+ min (incl. Azure)\nGET /tumblebug/loadAssets?includeAzure=true"]
            Price["POST /tumblebug/fetchPrice\n~10 min (B/C only, cancellable)"]

            Choice -->|backup only| OptA
            Choice -->|backup + CSV patch| OptAPlus
            Choice -->|fresh fetch| OptB
            Choice -->|fresh + Azure| OptC
            OptB --> Price
            OptC --> Price
        end

        subgraph TplLoad["Template Loading"]
            TplLoop["For each JSON in init/templates/"]
            EnsureNS["POST /tumblebug/ns\n(ensure namespace exists)"]
            PostTpl["POST /tumblebug/ns/{nsId}/template/{type}\ntype: infra | vNet | securityGroup"]
            TplLoop --> EnsureNS --> PostTpl --> TplLoop
        end

        CredReg --> AssetLoad --> TplLoad
    end

    TplLoad --> Ready["PUT /tumblebug/readyz/init\nMark system ready"]
    Ready --> Done([Init complete ✓])

    style Phase2 fill:#fff8e6,stroke:#d9a000
    style ErrTB fill:#ffe6e6,stroke:#cc0000
    style CredReg fill:#fff0e0,stroke:#e07000
    style AssetLoad fill:#e8ffe8,stroke:#00aa44
    style TplLoad fill:#f0e8ff,stroke:#8844cc
```

---

## 보안 설계 (Security Design)

`make init`은 모든 단계에서 CSP 자격 증명을 보호하기 위해 **2계층 암호화 전략**(Two-layer encryption strategy)을 적용합니다:

```mermaid
flowchart LR
    subgraph Disk["On Disk"]
        Enc["credentials.yaml.enc\nAES-256-CBC + PBKDF2"]
    end

    subgraph Memory["In-Memory Only"]
        Plain["Plaintext credentials\n(never written to disk)"]
    end

    subgraph Transit["In Transit → Tumblebug"]
        HybridEnc["Hybrid Encrypted payload\nRSA-OAEP(AES-256 key)\n+ AES-CBC(credential values)"]
    end

    subgraph BaoStorage["OpenBao KV v2"]
        BaoSecret["CSP secrets\npath: csp/{provider}\nor users/{holder}/csp/{provider}"]
    end

    subgraph TBStorage["CB-Tumblebug Internal"]
        TBCred["Credentials stored\nserver-side (encrypted at rest)"]
    end

    Enc -->|"decrypt in-memory\n(password/key)"| Plain
    Plain -->|"hybrid encrypt\nbefore sending"| HybridEnc
    Plain -->|"direct write\nvia OpenBao API"| BaoSecret
    HybridEnc -->|"TB decrypts\nRSA private key → AES key → values"| TBCred

    style Disk fill:#fce4ec,stroke:#c62828
    style Memory fill:#fff8e1,stroke:#f57f17
    style Transit fill:#e8f5e9,stroke:#2e7d32
    style BaoStorage fill:#e3f2fd,stroke:#1565c0
    style TBStorage fill:#f3e5f5,stroke:#6a1b9a
```

| 단계 | 메커니즘 |
|-------|-----------|
| 디스크 저장 | `credentials.yaml.enc` — PBKDF2 키 파생을 사용한 AES-256-CBC |
| 인메모리 복호화 | 평문은 RAM에만 존재하며 디스크에는 절대 기록되지 않음 |
| Tumblebug로 전송 | 하이브리드 RSA-OAEP + AES-256-CBC 암호화; Tumblebug가 RSA 개인키를 보유함 |
| OpenBao 저장 | 접근 제어 정책이 적용된 네이티브 KV v2 시크릿 엔진 |

---

## 소요 시간 예측 (Time Estimates)

| 모드 | 소요 시간 | 포함되는 데이터 |
|------|----------|---------------|
| 1단계 (OpenBao) | 약 1분 | KV v2 내 CSP 자격 증명 |
| 2단계 – 자격 증명 등록 | 약 1~2분 | 모든 자격 증명 소유자(Holder) × CSP × 리전 |
| 2단계 – 에셋 복원 (Option A) | 약 1분 | 스펙 + 이미지 + 가격 정보 (백업으로부터 복원) |
| 2단계 – 에셋 복원 + CSV 패치 (Option A+) | 약 2분 | 스펙 + 이미지 + 가격 정보 (백업으로부터 복원) + 최신 `cloudimage.csv` 덮어쓰기 적용 |
| 2단계 – 에셋 조회, Azure 제외 (Option B) | 약 20분 | 스펙 + 이미지 (CSP로부터 실시간 조회) |
| 2단계 – 에셋 조회, 모든 CSP (Option C) | 약 40분 이상 | Azure를 포함한 스펙 + 이미지 (실시간 조회) |
| 2단계 – 가격 정보 조회 (Option B/C 전용) | 약 10분 | 모든 스펙에 대한 가격 정보 |

---

## 빠른 참조 (Quick Reference)

```bash
# Full initialization (recommended)
make init

# Skip the interactive prompt and auto-select asset backup restore
make init -y

# Initialize with specific options (see init/README.md)
./init/multi-init.sh --help
```

**사전 준비 사항:**
- 모든 서비스 실행 중: `make up`
- `~/.cloud-barista/credentials.yaml.enc` 경로에 암호화된 자격 증명 파일 준비
  - 템플릿에서 생성: `./init/genCredential.sh`
  - 암호화: `./init/encCredential.sh`

**관련 문서:**
- [자격 증명 및 연결 가이드](credential-and-connection.md)
- [에셋 백업 및 복원 가이드](assets-backup-restore.md)
- [리소스 템플릿 관리 가이드](resource-template-management.md)
- [init/README.md](../../../init/README.md)
