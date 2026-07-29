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
    echo -e "${YELLOW}Step 1/4: Decompressing backup...${NC}"
    gunzip -c "$BACKUP_FILE" > "$TEMP_FILE"
else
    TEMP_FILE="$BACKUP_FILE"
fi

# Drop existing connections
echo -e "${YELLOW}Step 2/4: Terminating existing connections...${NC}"
pg_psql postgres "
SELECT pg_terminate_backend(pg_stat_activity.pid)
FROM pg_stat_activity
WHERE pg_stat_activity.datname = '$PG_DB'
  AND pid <> pg_backend_pid();
" 2>/dev/null || true

# Drop and recreate database
echo -e "${YELLOW}Step 3/4: Recreating database...${NC}"
pg_psql postgres "DROP DATABASE IF EXISTS \"$PG_DB\";" 2>/dev/null || true
pg_psql postgres "CREATE DATABASE \"$PG_DB\";"

# Restore backup
echo -e "${YELLOW}Step 4/4: Restoring database...${NC}"
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
