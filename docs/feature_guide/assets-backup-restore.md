# Assets Backup & Restore

CB-Tumblebug stores VM specs, OS images, and pricing data in PostgreSQL. Fetching this from all cloud providers takes 20–50 minutes. A pre-built backup reduces that to **~1 minute**.

## Time Comparison

| Method (`make init` option) | Time | Notes |
|--------|------|-------|
| **a. Restore from backup** | **~1 min** | Backup data only; `cloudimage.csv` not re-applied |
| **a+. Restore from backup + patch CSV** | **~2 min** | Backup data + latest `cloudimage.csv` applied on top |
| b. Fetch from CSPs (no Azure) | ~20–30 min | Live fetch; `cloudimage.csv` merged at the end |
| c. Fetch from ALL CSPs | ~40–50 min | Live fetch incl. Azure; `cloudimage.csv` merged at the end |

---

## Quick Start

```bash
# Backup current database
make backup-assets

# Restore from backup (via make init — recommended)
# Select option a  : restore only       (~1 min)
# Select option a+ : restore + CSV patch (~2 min, useful when cloudimage.csv was updated)
make init

# Restore directly from backup file (bypasses init menu)
make restore-assets

# Restore from specific file
make restore-assets FILE=./backups/postgres/tumblebug_db_20240115.dump.gz
```

---

## How the Backup Ecosystem Works

The backup file (`assets/assets.dump.gz`) is committed to the GitHub repository. Maintainers keep it up-to-date; all users benefit from it automatically on `git pull`.

```mermaid
flowchart LR
    subgraph Maintainer["🛠️ Maintainer / Contributor"]
        A["make init\nOption B or C\nlive fetch from CSPs\n⏱ 20–50 min"]
        B["make backup-assets\n⏱ ~1 min"]
    end

    subgraph GitHub["☁️ GitHub — cloud-barista/cb-tumblebug"]
        D["assets/assets.dump.gz\n~78 MB\n(latest CSP data)"]
    end

    U1["👤 User A\ngit pull + make init\n⏱ ~1 min"]
    U2["👤 User B\ngit pull + make init\n⏱ ~1 min"]
    U3["👤 User C\ngit pull + make init\n⏱ ~1 min"]
    U4["👤 User D\ngit pull + make init\n⏱ ~1 min"]
    U5["👤 User E\ngit pull + make init\n⏱ ~1 min"]
    U6["👤 User F\ngit pull + make init\n⏱ ~1 min"]
    U7["👤 User G\ngit pull + make init\n⏱ ~1 min"]

    A --> B -->|contribute| D
    D -->|"each saves\n~20–50 min"| U1 & U2 & U3 & U4 & U5 & U6 & U7

    style Maintainer fill:#e8f5e9,stroke:#2e7d32
    style GitHub fill:#f5f5f5,stroke:#424242
    style U1 fill:#e3f2fd,stroke:#1565c0
    style U2 fill:#e3f2fd,stroke:#1565c0
    style U3 fill:#e3f2fd,stroke:#1565c0
    style U4 fill:#e3f2fd,stroke:#1565c0
    style U5 fill:#e3f2fd,stroke:#1565c0
    style U6 fill:#e3f2fd,stroke:#1565c0
    style U7 fill:#e3f2fd,stroke:#1565c0
```

---

## Backup Flow

```mermaid
flowchart LR
    A([make backup-assets]) --> B
    B["① pg_dump\ninside container\n-F c custom format"] --> C
    C["② docker cp\ncontainer → host /tmp/"] --> D
    D["③ gzip compress\n~1.1 GB → ~78 MB\n→ assets/assets.dump.gz"] --> E
    E["④ cleanup\ntemp files removed"] --> F([✅ ~1 min])

    style A fill:#fff8e1,stroke:#f57f17
    style F fill:#e8f5e9,stroke:#2e7d32
```

## Restore Flow

```mermaid
flowchart LR
    A([make restore-assets]) --> B
    B["① gunzip\nassets.dump.gz → .dump"] --> C
    C["② docker cp\nhost → container"] --> D
    D["③ drop & recreate DB\n⚠️ all existing data cleared"] --> E
    E["④ pg_restore\nload all tables"] --> F
    F["⑤ cleanup\ntemp files removed"] --> G([✅ ~1 min])

    style A fill:#fff8e1,stroke:#f57f17
    style D fill:#fff3e0,stroke:#e65100
    style G fill:#e8f5e9,stroke:#2e7d32
```

> Restore replaces all data in PostgreSQL. It does **not** affect etcd (namespaces, MCI state, credentials).

---

## What Gets Backed Up

**Included (PostgreSQL):** VM specs, OS images, pricing, region info

**Not included (stored in etcd):** Running MCIs, namespaces, credentials

---

## `cloudimage.csv` and the Backup

`assets/cloudimage.csv` is a hand-curated list of images that are difficult to obtain via CSP APIs — images that change too frequently, take too long to fetch, or are not exposed through standard APIs (e.g., Azure marketplace images, AWS/GCP Kubernetes node type identifiers).

During `make init` with options **b** or **c**, `cloudimage.csv` is automatically merged into the database *after* the CSP fetch completes. The backup (`assets.dump.gz`) captures whatever state was current *at backup time*, including any previously applied CSV data.

If `cloudimage.csv` has been updated since the backup was created, use **option a+** to restore the backup and immediately re-apply the latest CSV on top — without a full CSP fetch.

```
Option a  → pg_restore only
            (cloudimage.csv state as of backup time)

Option a+ → pg_restore
            + POST /tumblebug/updateImagesFromAsset
            (cloudimage.csv re-applied from current file)

Option b/c → live CSP fetch
            + cloudimage.csv merged at the end
```

---

## Contributing an Updated Backup

```bash
make init                          # Option B or C — live fetch
make backup-assets                 # capture to assets/assets.dump.gz
git add assets/assets.dump.gz
git commit -m "chore: update assets database — $(date +%Y-%m-%d)"
# open a Pull Request
```

---

## File Locations

```
cb-tumblebug/
├── assets/
│   └── assets.dump.gz     # committed to Git — shared via GitHub
├── backups/postgres/
│   └── *.dump.gz          # local manual backups (git-ignored)
└── scripts/
    ├── backup-assets.sh
    └── restore-assets.sh
```

---

## Troubleshooting

| Problem | Fix |
|---------|-----|
| `container is not running` | `make up` |
| `backup file not found` | check path or run `make backup-assets` first |
| skip confirmation prompt | `RESTORE_SKIP_CONFIRM=yes ./scripts/restore-assets.sh` |

---

## Related

- [`make init` Workflow](make-init-workflow.md)
- [Resource Template Management](resource-template-management.md)
