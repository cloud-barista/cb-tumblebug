# CB-Tumblebug `make init` Initialization Workflow

## Overview

`make init` is the primary initialization command for CB-Tumblebug. It orchestrates a two-phase process that:

1. **Phase 1 — OpenBao Credential Registration**: Decrypts `credentials.yaml.enc` and stores CSP credentials into OpenBao (Vault-compatible secrets manager), making them available to MC-Terrarium's OpenTofu/Terraform templates.
2. **Phase 2 — Tumblebug Initialization**: Registers the same credentials into CB-Tumblebug (with end-to-end hybrid encryption), then loads cloud asset data (VM specs, OS images, pricing) and infrastructure templates into the system.

After `make init` completes, CB-Tumblebug is fully operational for multi-cloud infrastructure provisioning.

---

## Full Interaction Diagram

The diagram below shows how `make init` interacts with each component end-to-end.

```mermaid
sequenceDiagram
    autonumber
    actor User
    box Local Host
        participant Script as multi-init.sh<br/>init.sh / init.py
        participant EncFile as credentials.yaml.enc<br/>(~/.cloud-barista/)
        participant Templates as init/templates/<br/>(MCI / vNet / SG)
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
        else Option B – Fetch from CSPs, skip Azure (~20 min)
            Script->>TB: GET /tumblebug/loadAssets
            TB->>Spider: Fetch VM specs per region (parallel)
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
            Script->>TB: POST /tumblebug/ns/{nsId}/template/{type}<br/>(type: mci | vNet | securityGroup)
            TB-->>Script: template stored
        end
    end

    Script->>TB: PUT /tumblebug/readyz/init
    TB-->>Script: initialization marked complete
    Script-->>User: Init complete ✓<br/>(elapsed time summary)
```

---

## Component Roles During `make init`

| Component | Role During `make init` |
|-----------|------------------------|
| **multi-init.sh / init.py** | Orchestrator: decrypts credentials, calls all APIs, reports results |
| **credentials.yaml.enc** | Encrypted credential source — decrypted in-memory only, never written to disk as plaintext |
| **OpenBao** (`:8200`) | Stores CSP credentials in KV v2; later consumed by MC-Terrarium's OpenTofu templates |
| **CB-Tumblebug** (`:1323`) | Receives encrypted credentials, manages connections, triggers asset loading, stores templates |
| **CB-Spider** (`:1024`) | Registers connection configs per region, probes CSP APIs to verify connectivity |
| **PostgreSQL** (`:5432`) | Stores loaded VM specs, OS images, and pricing data |
| **CSPs** | Source of truth for connection verification, VM specs, OS images, and pricing |

---

## Phase Details

### Phase 1 — OpenBao Credential Registration

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

### Phase 2 — Tumblebug Initialization

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
            OptB["Option B: Fetch from CSPs\n~20 min (no Azure)\nGET /tumblebug/loadAssets"]
            OptC["Option C: Fetch ALL CSPs\n~40+ min (incl. Azure)\nGET /tumblebug/loadAssets?includeAzure=true"]
            Price["POST /tumblebug/fetchPrice\n~10 min (B/C only, cancellable)"]

            Choice -->|backup exists| OptA
            Choice -->|fresh fetch| OptB
            Choice -->|fresh + Azure| OptC
            OptB --> Price
            OptC --> Price
        end

        subgraph TplLoad["Template Loading"]
            TplLoop["For each JSON in init/templates/"]
            EnsureNS["POST /tumblebug/ns\n(ensure namespace exists)"]
            PostTpl["POST /tumblebug/ns/{nsId}/template/{type}\ntype: mci | vNet | securityGroup"]
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

## Security Design

`make init` applies a **two-layer encryption strategy** to protect CSP credentials at every stage:

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

| Stage | Mechanism |
|-------|-----------|
| Storage on disk | `credentials.yaml.enc` — AES-256-CBC with PBKDF2 key derivation |
| In-memory decryption | Plaintext exists only in RAM; never written to disk |
| Transit to Tumblebug | Hybrid RSA-OAEP + AES-256-CBC encryption; Tumblebug holds the RSA private key |
| OpenBao storage | Native KV v2 secret engine with access-control policies |

---

## Time Estimates

| Mode | Duration | Data Included |
|------|----------|---------------|
| Phase 1 (OpenBao) | ~1 min | CSP credentials in KV v2 |
| Phase 2 – Credential registration | ~1–2 min | All credential holders × CSPs × regions |
| Phase 2 – Asset restore (Option A) | ~1 min | Specs + images + pricing (from backup) |
| Phase 2 – Asset fetch, no Azure (Option B) | ~20 min | Specs + images (live from CSPs) |
| Phase 2 – Asset fetch, all CSPs (Option C) | ~40+ min | Specs + images incl. Azure (live) |
| Phase 2 – Price fetch (Options B/C only) | ~10 min | Pricing for all specs |

---

## Quick Reference

```bash
# Full initialization (recommended)
make init

# Skip the interactive prompt and auto-select asset backup restore
make init -y

# Initialize with specific options (see init/README.md)
./init/multi-init.sh --help
```

**Prerequisites:**
- All services running: `make up`
- Encrypted credentials file at `~/.cloud-barista/credentials.yaml.enc`
  - Generate from template: `./init/genCredential.sh`
  - Encrypt: `./init/encCredential.sh`

**Related documentation:**
- [Credential & Connection Guide](credential-and-connection.md)
- [Assets Backup & Restore Guide](assets-backup-restore.md)
- [Resource Template Management Guide](resource-template-management.md)
- [init/README.md](../../init/README.md)
