#!/bin/bash

# CB-Tumblebug Assets Database Restore Script
# Usage: ./scripts/restore-assets.sh [backup-file]
# Default: ./assets/assets.dump.gz
#
# Backend selection (docker | kubectl | direct): see scripts/lib/pg-backend.sh
# Non-interactive: RESTORE_SKIP_CONFIRM=yes

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
. "$SCRIPT_DIR/lib/pg-backend.sh"

DEFAULT_BACKUP="./assets/assets.dump.gz"
BACKUP_FILE="${1:-$DEFAULT_BACKUP}"

echo -e "${GREEN}=== CB-Tumblebug Assets Database Restore ===${NC}"
echo ""

# Validation
if [ ! -f "$BACKUP_FILE" ]; then
    echo -e "${RED}Error: Backup file not found: $BACKUP_FILE${NC}"
    echo ""
    echo "Usage: $0 [backup-file]"
    echo "Default: $DEFAULT_BACKUP"
    echo ""
    if [ -f "$DEFAULT_BACKUP" ]; then
        echo "💡 Tip: Run without arguments to use default backup"
    else
        echo "⚠️  No default backup found. Create one with: make backup-assets"
    fi
    exit 1
fi

pg_backend_init
echo "Target: $(pg_backend_describe), database: $PG_DB"

# Preview what this backup holds, from the manifest written at backup time, so the user
# confirms with context. Older backups without a manifest fall back to file date + size.
echo ""
INFO_FILE="${BACKUP_FILE}.info"
if [ -f "$INFO_FILE" ]; then
    echo -e "${YELLOW}Backup contents:${NC}"
    cat "$INFO_FILE"
else
    echo -e "${YELLOW}Backup file:${NC} $BACKUP_FILE ($(du -h "$BACKUP_FILE" 2>/dev/null | cut -f1), dated $(date -r "$BACKUP_FILE" '+%Y-%m-%d %H:%M' 2>/dev/null))"
    echo "  (no manifest; regenerate with 'make backup-assets' to include row/CSP details)"
fi
echo ""

# Warning (skip if RESTORE_SKIP_CONFIRM=yes)
if [ "$RESTORE_SKIP_CONFIRM" != "yes" ]; then
    echo -e "${YELLOW}⚠️  WARNING: This will replace all existing data in the database!${NC}"
    echo ""
    read -p "Are you sure you want to continue? (yes/no): " CONFIRM

    if [ "$CONFIRM" != "yes" ]; then
        echo "Restore cancelled."
        exit 0
    fi
else
    echo -e "${GREEN}Auto-confirm mode: Proceeding with database restore...${NC}"
fi

echo ""

# Decompress if needed
TEMP_FILE="/tmp/tumblebug_restore_$$.dump"
if [[ "$BACKUP_FILE" == *.gz ]]; then
    echo -e "${YELLOW}Step 1/3: Decompressing backup...${NC}"
    gunzip -c "$BACKUP_FILE" > "$TEMP_FILE"
else
    TEMP_FILE="$BACKUP_FILE"
fi

# The dump is data-only; the schema is owned by cb-tumblebug's AutoMigrate, which runs at
# server startup. So the target tables must already exist — the server must have been
# started at least once against this database. This is what keeps a backup from ever
# carrying a stale schema: schema follows the code, only rows come from the backup.
echo -e "${YELLOW}Step 2/3: Verifying schema exists...${NC}"
TABLE_COUNT=$(pg_psql "$PG_DB" "SELECT count(*) FROM pg_tables WHERE schemaname = 'public';" 2>/dev/null | grep -Eo '[0-9]+' | head -1 || true)
if [ -z "$TABLE_COUNT" ] || [ "$TABLE_COUNT" -eq 0 ]; then
    echo -e "${RED}✖ No tables found in database '$PG_DB'.${NC}"
    echo "  The schema is created by the cb-tumblebug server at startup, not by this restore."
    echo "  Start the server first (make up / make k-up), then re-run: make restore-assets"
    exit 1
fi

# Clear existing rows, then load rows from the backup. TRUNCATE (not DROP) preserves the
# app-created schema; iterating every public table keeps this correct as models evolve.
echo -e "${YELLOW}Step 3/3: Clearing existing data and restoring rows...${NC}"
TRUNCATE_SQL=$(cat <<'SQL'
DO $$
DECLARE r RECORD;
BEGIN
  FOR r IN SELECT tablename FROM pg_tables WHERE schemaname = 'public' LOOP
    EXECUTE 'TRUNCATE TABLE public.' || quote_ident(r.tablename) || ' RESTART IDENTITY CASCADE';
  END LOOP;
END $$;
SQL
)
pg_psql "$PG_DB" "$TRUNCATE_SQL"
pg_restore_file "$TEMP_FILE" "$PG_DB"

# Cleanup
if [[ "$BACKUP_FILE" == *.gz ]]; then
    rm -f "$TEMP_FILE"
fi

# Display results
echo ""
echo -e "${GREEN}✅ Database restored successfully!${NC}"
echo ""

# Get restored database statistics
echo -e "${YELLOW}Restored Database Statistics:${NC}"
pg_psql "$PG_DB" "
SELECT
    t.schemaname,
    t.relname AS tablename,
    pg_size_pretty(pg_total_relation_size(t.schemaname||'.'||t.relname)) AS size,
    (SELECT COUNT(*) FROM pg_catalog.pg_class c WHERE c.relname = t.relname) AS exists
FROM pg_stat_user_tables t
ORDER BY pg_total_relation_size(t.schemaname||'.'||t.relname) DESC;
" 2>/dev/null || true

echo ""
echo -e "${GREEN}Database is ready to use!${NC}"
echo ""
