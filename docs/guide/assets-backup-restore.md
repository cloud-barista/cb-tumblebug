# Assets Backup & Restore Guide

CB-Tumblebug uses PostgreSQL to store assets data (VM specs, images, pricing information). This guide explains how to backup and restore this database.

## Quick Start

```bash
# Backup assets database
make backup-assets

# Restore from backup
make restore-assets

# Restore from specific file
make restore-assets FILE=./backups/postgres/tumblebug_db_20240115_143022.dump.gz
```

## What Gets Backed Up

**PostgreSQL Assets Database:**
- VM specifications from all cloud providers
- OS images catalog
- Pricing information
- Performance metrics

**NOT Backed Up (stored in etcd):**
- Running MCIs (Multi-Cloud Infrastructures)
- Namespaces
- User credentials
- Runtime operational data

## Backup Process

### 1. Create Backup

```bash
make backup-assets
```

**What happens:**
1. Connects to `cb-tumblebug-postgres` container
2. Creates PostgreSQL custom format dump (`pg_dump -F c`)
3. Compresses with gzip
4. Saves to `./assets/assets.dump.gz`
5. Shows database statistics

**Backup file location:** `./assets/assets.dump.gz`

**Typical size:** ~78 MB (compressed)

### 2. Manual Backup

```bash
# Custom backup name
./scripts/backup-assets.sh my_backup_20240115
# Creates: ./backups/postgres/my_backup_20240115.dump.gz
```

### 3. Backup Format

- **Format:** PostgreSQL Custom Format (`-F c`)
- **Compression:** Built-in + gzip
- **Advantages:**
  - 93% size reduction (1.1 GB → 78 MB)
  - Parallel restore support
  - Selective table restore
  - Error tolerance

## Restore Process

### 1. Restore from Default Backup

```bash
make restore-assets
```

Restores from `./assets/assets.dump.gz`

### 2. Restore from Specific File

```bash
make restore-assets FILE=./backups/postgres/my_backup.dump.gz
```

### 3. What Happens During Restore

1. **Validation:** Checks if backup file exists
2. **Warning:** Prompts for confirmation (destructive operation)
3. **Decompression:** Extracts .gz if needed
4. **Database Reset:**
   - Terminates existing connections
   - Drops existing database
   - Creates fresh database
5. **Data Restore:** Uses `pg_restore` to load data
6. **Statistics:** Shows restored table information

**Duration:** ~1 minute

## Initialization Methods

When running `make init`, you have three options:

### Option A: Restore from Backup (~1 minute)
```
Uses: ./assets/assets.dump.gz
Includes: Specs, Images, Pricing
Duration: ~1 minute
```

### Option B: Fetch Fresh from CSPs (~20-30 minutes)
```
Fetches: All providers except Azure
Duration: ~20-30 minutes
Creates new: Specs, Images
Pricing: Optional separate step
```

### Option C: Fetch Including Azure (~40+ minutes)
```
Fetches: ALL providers including Azure
Duration: ~40+ minutes
Note: Azure image fetch is very slow
```

**Recommendation:** Use Option A (backup) for development and testing.

## Common Use Cases

### 1. Fast Development Setup
```bash
make up          # Start containers
make init        # Choose Option A (restore from backup)
```

### 2. Get Latest CSP Data
```bash
make clean-db    # Clear existing data
make init        # Choose Option B or C (fetch fresh)
```

### 3. Create Distributable Backup
```bash
make backup-assets           # Create backup
git add assets/assets.dump.gz
git commit -m "Update assets database backup"
git push
```

### 4. Restore After Failed Initialization
```bash
make restore-assets
```

## File Locations

```
cb-tumblebug/
├── assets/
│   └── assets.dump.gz          # Default backup for distribution
├── backups/
│   └── postgres/
│       ├── tumblebug_db_20240115_143022.dump.gz
│       └── tumblebug_db_20240120_091530.dump.gz
└── scripts/
    ├── backup-assets.sh        # Backup script
    └── restore-assets.sh       # Restore script
```

## Troubleshooting

### Container Not Running
```bash
# Error: PostgreSQL container is not running
make up    # Start containers first
```

### No Backup File Found
```bash
# Check available backups
ls -lh ./backups/postgres/

# Restore from specific file
make restore-assets FILE=./backups/postgres/tumblebug_db_20240115.dump.gz
```

### Restore Without Confirmation
```bash
# For automated scripts
RESTORE_SKIP_CONFIRM=yes ./scripts/restore-assets.sh ./assets/assets.dump.gz
```

### Check Database Size
```bash
docker exec cb-tumblebug-postgres psql -U tumblebug -d tumblebug -c "
SELECT pg_size_pretty(pg_database_size('tumblebug')) AS size;"
```

## Best Practices

1. **Regular Backups:** Create backups after major CSP data updates
2. **Version Control:** Commit `assets.dump.gz` to Git for team sharing
3. **Testing:** Always test backups with `make restore-assets`
4. **Development:** Use Option A (backup restore) for faster setup
5. **Production:** Use Option B/C (fresh fetch) for latest data

## Technical Details

### Backup Command
```bash
docker exec cb-tumblebug-postgres pg_dump \
    -U tumblebug \
    -d tumblebug \
    -F c \              # Custom format
    -f backup.dump

gzip backup.dump        # Compress
```

### Restore Command
```bash
gunzip -c backup.dump.gz > backup.dump

docker exec cb-tumblebug-postgres pg_restore \
    -U tumblebug \
    -d tumblebug \
    -v \                # Verbose
    backup.dump
```

### Database Schema
Main tables backed up:
- `spec_info`: VM specifications
- `image_info`: OS images
- `pricing_info`: Cost data
- `region_info`: Regional data

## Related Documentation

- [Development Guide](../.github/copilot-instructions.md)
- [Docker Compose Setup](../docker-compose.yaml)
- [Initialization Script](../init/init.py)

## Quick Reference

| Task | Command | Duration |
|------|---------|----------|
| Backup database | `make backup-assets` | ~1 minute |
| Restore default | `make restore-assets` | ~1 minute |
| Restore specific | `make restore-assets FILE=<path>` | ~1 minute |
| Init with backup | `make init` → Choose A | ~1 minute |
| Init fresh (no Azure) | `make init` → Choose B | ~20-30 min |
| Init fresh (with Azure) | `make init` → Choose C | ~40+ min |
