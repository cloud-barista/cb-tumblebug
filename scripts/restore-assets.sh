#!/bin/bash

# CB-Tumblebug Assets Database Restore Script
# Usage: ./scripts/restore-assets.sh [backup-file]
# Default: ./assets/assets.dump.gz

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
CONTAINER_NAME="cb-tumblebug-postgres"
DB_USER="tumblebug"
DB_NAME="tumblebug"
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

# Check if container is running
if ! docker ps | grep -q "$CONTAINER_NAME"; then
    echo -e "${RED}Error: PostgreSQL container '$CONTAINER_NAME' is not running${NC}"
    echo "Please start the container with: make up"
    exit 1
fi

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
    echo -e "${YELLOW}Step 1/5: Decompressing backup...${NC}"
    gunzip -c "$BACKUP_FILE" > "$TEMP_FILE"
else
    TEMP_FILE="$BACKUP_FILE"
fi

# Copy backup to container
echo -e "${YELLOW}Step 2/5: Copying backup to container...${NC}"
docker cp "$TEMP_FILE" "$CONTAINER_NAME:/var/lib/postgresql/data/restore.dump"

# Drop existing connections
echo -e "${YELLOW}Step 3/5: Terminating existing connections...${NC}"
docker exec "$CONTAINER_NAME" psql -U "$DB_USER" -d postgres -c "
SELECT pg_terminate_backend(pg_stat_activity.pid)
FROM pg_stat_activity
WHERE pg_stat_activity.datname = '$DB_NAME'
  AND pid <> pg_backend_pid();
" 2>/dev/null || true

# Drop and recreate database
echo -e "${YELLOW}Step 4/5: Recreating database...${NC}"
docker exec "$CONTAINER_NAME" psql -U "$DB_USER" -d postgres -c "DROP DATABASE IF EXISTS $DB_NAME;" 2>/dev/null || true
docker exec "$CONTAINER_NAME" psql -U "$DB_USER" -d postgres -c "CREATE DATABASE $DB_NAME;"

# Restore backup
echo -e "${YELLOW}Step 5/5: Restoring database...${NC}"
docker exec "$CONTAINER_NAME" pg_restore \
    -U "$DB_USER" \
    -d "$DB_NAME" \
    -v \
    /var/lib/postgresql/data/restore.dump

# Cleanup
docker exec "$CONTAINER_NAME" rm -f /var/lib/postgresql/data/restore.dump
if [[ "$BACKUP_FILE" == *.gz ]]; then
    rm -f "$TEMP_FILE"
fi

# Display results
echo ""
echo -e "${GREEN}✅ Database restored successfully!${NC}"
echo ""

# Get restored database statistics
echo -e "${YELLOW}Restored Database Statistics:${NC}"
docker exec "$CONTAINER_NAME" psql -U "$DB_USER" -d "$DB_NAME" -c "
SELECT 
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size,
    (SELECT COUNT(*) FROM pg_catalog.pg_class WHERE relname = tablename) AS exists
FROM pg_stat_user_tables
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
" 2>/dev/null || true

echo ""
echo -e "${GREEN}Database is ready to use!${NC}"
echo ""
