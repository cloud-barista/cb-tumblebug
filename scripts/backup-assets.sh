#!/bin/bash

# CB-Tumblebug Assets Database Backup Script
# Usage: ./scripts/backup-assets.sh [output-file]
# Default output: ./assets/assets.dump.gz
#
# Backend selection (docker | kubectl | direct): see scripts/lib/pg-backend.sh
# Non-interactive: BACKUP_SKIP_CONFIRM=yes

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
. "$SCRIPT_DIR/lib/pg-backend.sh"

TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# Default output: ./assets/assets.dump.gz
# Can be overridden with first argument for manual backups
OUTPUT_FILE="${1:-./assets/assets.dump.gz}"
OUTPUT_DIR=$(dirname "$OUTPUT_FILE")
TEMP_BACKUP_FILE="/tmp/tumblebug_backup_${TIMESTAMP}.dump"

echo -e "${GREEN}=== CB-Tumblebug Assets Database Backup ===${NC}"
echo ""

pg_backend_init
echo "Target: $(pg_backend_describe), database: $PG_DB"
echo ""

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Check if output file already exists and ask for confirmation
if [ -f "$OUTPUT_FILE" ] && [ "$BACKUP_SKIP_CONFIRM" != "yes" ]; then
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

# Create database dump
echo -e "${YELLOW}Step 1/3: Creating database dump...${NC}"
pg_dump_file "$PG_DB" "$TEMP_BACKUP_FILE"

# Compress and move to final location
echo -e "${YELLOW}Step 2/3: Compressing backup...${NC}"
gzip -c "$TEMP_BACKUP_FILE" > "$OUTPUT_FILE"

# Cleanup temporary files
echo -e "${YELLOW}Step 3/3: Cleaning up temporary files...${NC}"
rm -f "$TEMP_BACKUP_FILE"

# Display results
BACKUP_SIZE=$(du -h "$OUTPUT_FILE" | cut -f1)
echo ""
echo -e "${GREEN}✅ Backup completed successfully!${NC}"
echo ""
echo "Backup location: $OUTPUT_FILE"
echo "Backup size: $BACKUP_SIZE"
echo ""

# Manifest sidecar: a small human-readable summary (created time, row counts, CSP
# breakdown) so `restore-assets` can preview what this backup holds before restoring.
INFO_FILE="${OUTPUT_FILE}.info"
# Per-CSP asset summary, using the same definition as the server's /assetsSummary
# (priced = cost_per_hour <> -1) so the manifest mirrors what `make init` prints.
# Best-effort: any query failure just leaves that part blank, never fails the backup.
NS="system"
SUMMARY_ROWS=$(pg_psql_value "$PG_DB" "
SELECT COALESCE(s.provider_name,i.provider_name)||'|'||COALESCE(i.images,0)||'|'||COALESCE(s.specs,0)||'|'||
       CASE WHEN COALESCE(s.specs,0)>0 THEN round(100.0*s.priced/s.specs)::int::text||'%' ELSE 'N/A' END
FROM (SELECT provider_name, count(*) specs, sum(CASE WHEN cost_per_hour <> -1 THEN 1 ELSE 0 END) priced
      FROM spec_infos WHERE namespace='$NS' GROUP BY provider_name) s
FULL OUTER JOIN (SELECT provider_name, count(*) images
      FROM image_infos WHERE namespace='$NS' AND resource_type='image' GROUP BY provider_name) i
  ON s.provider_name=i.provider_name
ORDER BY 1;" 2>/dev/null || true)
TOT_SPECS=$(pg_psql_value "$PG_DB" "SELECT count(*) FROM spec_infos WHERE namespace='$NS'" 2>/dev/null || echo 0)
TOT_PRICED=$(pg_psql_value "$PG_DB" "SELECT count(*) FROM spec_infos WHERE namespace='$NS' AND cost_per_hour <> -1" 2>/dev/null || echo 0)
TOT_IMAGES=$(pg_psql_value "$PG_DB" "SELECT count(*) FROM image_infos WHERE namespace='$NS' AND resource_type='image'" 2>/dev/null || echo 0)
CSP_COUNT=$(printf '%s\n' "$SUMMARY_ROWS" | grep -c '|' || true)
TOT_PCT="N/A"
if [[ "$TOT_SPECS" =~ ^[0-9]+$ ]] && [ "$TOT_SPECS" -gt 0 ]; then TOT_PCT="$(( 100 * TOT_PRICED / TOT_SPECS ))%"; fi
{
    echo "CB-Tumblebug assets backup"
    echo "created: $(date -u '+%Y-%m-%d %H:%M:%S UTC')"
    echo "summary: ${TOT_SPECS} specs (${TOT_PCT} priced), ${TOT_IMAGES} images across ${CSP_COUNT} CSPs"
    echo "size:    $BACKUP_SIZE"
    echo ""
    echo "Asset Summary (by CSP)"
    printf '%-10s  %8s  %8s  %10s\n' "Provider" "Images" "Specs" "(Priced)"
    printf '%-10s  %8s  %8s  %10s\n' "----------" "--------" "--------" "----------"
    printf '%s\n' "$SUMMARY_ROWS" | while IFS='|' read -r prov img spc pct; do
        [ -n "$prov" ] && printf '%-10s  %8s  %8s  %10s\n' "$prov" "$img" "$spc" "$pct"
    done
    echo "Totals: specs=${TOT_SPECS} (${TOT_PCT} priced), images=${TOT_IMAGES}"
} > "$INFO_FILE" 2>/dev/null || true
echo "Backup manifest: $INFO_FILE"
echo ""

# Get database statistics
echo -e "${YELLOW}Database Statistics:${NC}"
pg_psql "$PG_DB" "
SELECT
    schemaname,
    relname AS tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||relname)) AS size,
    n_tup_ins AS inserts
FROM pg_stat_user_tables
ORDER BY pg_total_relation_size(schemaname||'.'||relname) DESC;
" 2>/dev/null || true

echo ""
echo -e "${GREEN}💡 Next steps:${NC}"
if [ "$OUTPUT_FILE" = "./assets/assets.dump.gz" ]; then
    echo "  1. Test the backup: make restore-assets"
    echo ""
    echo "  To contribute your assets to the open-source community:"
    echo "  2. Commit both files: git add assets/assets.dump.gz assets/assets.dump.gz.info && git commit -m 'Update assets database'"
    echo "  3. Open a Pull Request to share your updated assets"
else
    echo "  To restore this backup: ./scripts/restore-assets.sh $OUTPUT_FILE"
fi
echo ""
