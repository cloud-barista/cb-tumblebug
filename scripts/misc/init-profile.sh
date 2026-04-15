#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR=$(cd "$(dirname "$0")/../.." && pwd)
INIT_DIR="$ROOT_DIR/init"
OUT_BASE="$ROOT_DIR/tmp/init-profile"
RUN_ID=$(date +"%Y%m%d_%H%M%S")
OUT_DIR="$OUT_BASE/$RUN_ID"

mkdir -p "$OUT_DIR"

TIMED_LOG="$OUT_DIR/init.timed.log"
RESOURCE_CSV="$OUT_DIR/resources.csv"
SUMMARY_TXT="$OUT_DIR/summary.txt"

cat <<EOF
[init-profile] Maintainer diagnostic mode
[init-profile] Output dir: $OUT_DIR
[init-profile] Profiling scope: Step 2 (Tumblebug init.sh) only
EOF
echo "epoch,phase,host_used_gib,host_available_gib,containers_total_gib,tb_mem_mib,sp_mem_mib,pg_mem_mib,meta_mem_mib" > "$RESOURCE_CSV"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "CB-Tumblebug Initialization"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
read -r -s -p "Enter the password for credentials.yaml.enc: " MULTI_INIT_PWD
echo ""
export MULTI_INIT_PWD

# Step 1. OpenBao (same behavior as init/multi-init.sh; no profiling)
if [ -f "$INIT_DIR/openbao/openbao-register-creds.sh" ]; then
  OPENBAO_SH="$INIT_DIR/openbao/openbao-register-creds.sh"
elif [ -f "$INIT_DIR/../../openbao/openbao-register-creds.sh" ]; then
  OPENBAO_SH="$INIT_DIR/../../openbao/openbao-register-creds.sh"
else
  echo "Error: Cannot find openbao-register-creds.sh"
  exit 1
fi

echo ""
echo "Step 1. Registering credentials to OpenBao..."
chmod +x "$OPENBAO_SH" 2>/dev/null || true
bash "$OPENBAO_SH"

echo ""
echo "Step 2. Registering credentials to Tumblebug..."

to_mib() {
  local raw="$1"
  local value unit
  value=$(echo "$raw" | sed -E 's/([0-9.]+).*/\1/')
  unit=$(echo "$raw" | sed -E 's/[0-9.]+([A-Za-z]+).*/\1/')

  case "$unit" in
    B) awk -v v="$value" 'BEGIN {printf "%.2f", v/1024/1024}' ;;
    KiB|kB) awk -v v="$value" 'BEGIN {printf "%.2f", v/1024}' ;;
    MiB|MB) awk -v v="$value" 'BEGIN {printf "%.2f", v}' ;;
    GiB|GB) awk -v v="$value" 'BEGIN {printf "%.2f", v*1024}' ;;
    TiB|TB) awk -v v="$value" 'BEGIN {printf "%.2f", v*1024*1024}' ;;
    *) echo "0.00" ;;
  esac
}

container_mem_mib() {
  local name="$1"
  local usage raw
  usage=$(docker stats "$name" --no-stream --format '{{.MemUsage}}' 2>/dev/null || true)
  if [[ -z "$usage" ]]; then
    echo "0.00"
    return
  fi
  raw=$(echo "$usage" | awk -F' / ' '{print $1}')
  to_mib "$raw"
}

monitor_resources() {
  while [[ ! -f "$OUT_DIR/.stop" ]]; do
    local epoch host_used host_avail
    local tb_mib sp_mib pg_mib meta_mib containers_total_gib

    epoch=$(date +%s)

    read -r host_used host_avail < <(free -g | awk '/^Mem:/ {print $3, $7}')

    tb_mib=$(container_mem_mib "cb-tumblebug")
    sp_mib=$(container_mem_mib "cb-spider")
    pg_mib=$(container_mem_mib "cb-tumblebug-postgres")
    meta_mib=$(container_mem_mib "cb-tumblebug-metabase")

    containers_total_gib=$(awk -v a="$tb_mib" -v b="$sp_mib" -v c="$pg_mib" -v d="$meta_mib" 'BEGIN {printf "%.3f", (a+b+c+d)/1024}')

    echo "$epoch,tumblebug_init,$host_used,$host_avail,$containers_total_gib,$tb_mib,$sp_mib,$pg_mib,$meta_mib" >> "$RESOURCE_CSV"
    sleep 1
  done
}

monitor_resources &
MONITOR_PID=$!

INIT_EXIT=0
(
  cd "$INIT_DIR"
  LOG=on MULTI_INIT_PWD="$MULTI_INIT_PWD" bash ./init.sh 2>&1 |
    awk -v timed_log="$TIMED_LOG" '
      {
        now = systime()
        line = $0

        print line
        printf "%d|%s\n", now, line >> timed_log
        fflush(timed_log)
      }
    '
) || INIT_EXIT=$?

touch "$OUT_DIR/.stop"
wait "$MONITOR_PID" 2>/dev/null || true

START_TS=$(awk -F'|' 'NR==1 {print $1}' "$TIMED_LOG" 2>/dev/null || date +%s)
END_TS=$(awk -F'|' 'END {print $1}' "$TIMED_LOG" 2>/dev/null || date +%s)

TOTAL_SEC=$((END_TS - START_TS))

{
  echo "Init Profile Summary"
  echo "run_id=$RUN_ID"
  echo "output_dir=$OUT_DIR"
  echo "init_exit_code=$INIT_EXIT"
  echo "profile_scope=step2_tumblebug_only"
  echo ""
  echo "Elapsed (seconds)"
  echo "- step2_tumblebug_init_total=$TOTAL_SEC"
  echo ""
  echo "Peak Memory"
  awk -F',' '
    NR==1 {next}
    {
      if ($3+0 > max_host_used) max_host_used=$3+0
      if ($5+0 > max_cont_gib) max_cont_gib=$5+0
      if ($6+0 > max_tb_mib) max_tb_mib=$6+0
      if ($7+0 > max_sp_mib) max_sp_mib=$7+0
      if ($8+0 > max_pg_mib) max_pg_mib=$8+0
      if ($9+0 > max_meta_mib) max_meta_mib=$9+0

      ph=$2
      if (!(ph in phase_max_host) || ($3+0 > phase_max_host[ph])) phase_max_host[ph]=$3+0
      if (!(ph in phase_max_cont) || ($5+0 > phase_max_cont[ph])) phase_max_cont[ph]=$5+0
      if (!(ph in phase_max_tb) || ($6+0 > phase_max_tb[ph])) phase_max_tb[ph]=$6+0
      phase_seen[ph]=1
    }
    END {
      printf "- host_used_gib=%d\n", max_host_used
      printf "- containers_total_gib=%.3f\n", max_cont_gib
      printf "- cb_tumblebug_mib=%.2f\n", max_tb_mib
      printf "- cb_spider_mib=%.2f\n", max_sp_mib
      printf "- cb_postgres_mib=%.2f\n", max_pg_mib
      printf "- cb_metabase_mib=%.2f\n", max_meta_mib
    }
  ' "$RESOURCE_CSV"

  echo ""
  echo "Price Fetch Milestones (from cb-tumblebug logs in init output)"
  grep -E "GCP direct pricing completed|Azure direct pricing completed|AWS direct pricing completed|Completed price fetching in" "$TIMED_LOG" | sed 's/^[0-9]\+|//' || true
} > "$SUMMARY_TXT"

if [[ $INIT_EXIT -eq 0 ]]; then
  echo ""
  echo "Initialization completed successfully."
fi

echo ""
echo "[init-profile] Summary: $SUMMARY_TXT"
echo "[init-profile] Timed log: $TIMED_LOG"
echo "[init-profile] Resource CSV: $RESOURCE_CSV"

if [[ $INIT_EXIT -ne 0 ]]; then
  echo "[init-profile] make init failed with exit code $INIT_EXIT"
  exit "$INIT_EXIT"
fi

echo "[init-profile] Completed successfully"
