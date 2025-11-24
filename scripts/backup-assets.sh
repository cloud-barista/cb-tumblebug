#!/bin/bash

# CB-Tumblebug Assets Database Backup Script
# Usage: ./scripts/backup-assets.sh [output-file]
# Default output: ./assets/assets.dump.gz

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
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# Default output: ./assets/assets.dump.gz
# Can be overridden with first argument for manual backups
OUTPUT_FILE="${1:-./assets/assets.dump.gz}"
OUTPUT_DIR=$(dirname "$OUTPUT_FILE")
TEMP_BACKUP_FILE="backup_${TIMESTAMP}.dump"

echo -e "${GREEN}=== CB-Tumblebug Assets Database Backup ===${NC}"
echo ""

# Check if container is running
if ! docker ps | grep -q "$CONTAINER_NAME"; then
    echo -e "${RED}Error: PostgreSQL container '$CONTAINER_NAME' is not running${NC}"
    echo "Please start the container with: make up"
    exit 1
fi

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Check if output file already exists and ask for confirmation
if [ -f "$OUTPUT_FILE" ]; then
    echo -e "${YELLOW}⚠️  Warning: Existing backup file found!${NC}"
    ls -lh "$OUTPUT_FILE"
    echo ""
    read -p "Do you want to overwrite it? (y/N): " confirm
    if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
        echo -e "${RED}❌ Backup cancelled. Keeping existing file.${NC}"
        exit 1
    fi
    echo ""
fi

# Create database backup
echo -e "${YELLOW}Step 1/4: Creating database dump...${NC}"
docker exec "$CONTAINER_NAME" pg_dump \
    -U "$DB_USER" \
    -d "$DB_NAME" \
    -F c \
    -f "/var/lib/postgresql/data/$TEMP_BACKUP_FILE"

# Copy backup from container to host (temporary location)
echo -e "${YELLOW}Step 2/4: Copying backup to host...${NC}"
docker cp "$CONTAINER_NAME:/var/lib/postgresql/data/$TEMP_BACKUP_FILE" "/tmp/$TEMP_BACKUP_FILE"

# Compress and move to final location
echo -e "${YELLOW}Step 3/4: Compressing backup...${NC}"
gzip -c "/tmp/$TEMP_BACKUP_FILE" > "$OUTPUT_FILE"

# Cleanup temporary files
echo -e "${YELLOW}Step 4/4: Cleaning up temporary files...${NC}"
docker exec "$CONTAINER_NAME" rm -f "/var/lib/postgresql/data/$TEMP_BACKUP_FILE"
rm -f "/tmp/$TEMP_BACKUP_FILE"

# Display results
BACKUP_SIZE=$(du -h "$OUTPUT_FILE" | cut -f1)
echo ""
echo -e "${GREEN}✅ Backup completed successfully!${NC}"
echo ""
echo "Backup location: $OUTPUT_FILE"
echo "Backup size: $BACKUP_SIZE"
echo ""

# Get database statistics
echo -e "${YELLOW}Database Statistics:${NC}"
docker exec "$CONTAINER_NAME" psql -U "$DB_USER" -d "$DB_NAME" -c "
SELECT 
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size,
    n_tup_ins AS inserts
FROM pg_stat_user_tables
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
" 2>/dev/null || true

echo ""
echo -e "${GREEN}💡 Next steps:${NC}"
if [ "$OUTPUT_FILE" = "./assets/assets.dump.gz" ]; then
    echo "  1. Test the backup: make restore-assets"
    echo ""
    echo "  To contribute your assets to the open-source community:"
    echo "  2. Commit the file: git add assets/assets.dump.gz && git commit -m 'Update assets database'"
    echo "  3. Open a Pull Request to share your updated assets"
else
    echo "  To restore this backup: ./scripts/restore-assets.sh $OUTPUT_FILE"
fi
echo ""
