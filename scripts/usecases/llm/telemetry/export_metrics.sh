#!/bin/bash

# =============================================================================
# Prometheus Metrics to CSV Exporter (Multi-IP Support)
# =============================================================================
# Supports three execution modes:
#   1. No arguments: exports all metrics for all IPs (last 60 minutes)
#   2. CLI arguments: --minutes, --ips, --metrics, --step
#   3. Config file (legacy): ./export_metrics.sh my_config.conf
#
# Examples:
#   curl -fsSL .../export_metrics.sh | bash
#   curl -fsSL .../export_metrics.sh | bash -s -- --minutes 120 --ips "1.2.3.4,5.6.7.8"
#   curl -fsSL .../export_metrics.sh | bash -s -- --metrics "node_cpu_seconds_total,vllm:num_requests_running"
#   ./export_metrics.sh my_config.conf
# =============================================================================

set -e

# Display usage
usage() {
  cat <<EOF
📊 Prometheus Metrics CSV Exporter

Usage:
  ./export_metrics.sh [options]           # CLI mode
  ./export_metrics.sh <config_file>       # Config file mode (legacy)
  curl ... | bash                         # Remote execution (all defaults)
  curl ... | bash -s -- [options]         # Remote execution with options

Options:
  --minutes <N>       Time range in minutes (default: 60)
  --ips <ip1,ip2>     Comma-separated list of target VM IPs (default: all)
  --metrics <m1,m2>   Comma-separated list of metric names (default: all)
  --step <interval>   Scrape step interval (default: 15s)
  -h, --help          Show this help message

Config File Format Example:
--------------------------------------------------
MINUTES=60
IPS=( "104.42.74.157" "3.96.201.235" )
METRICS=( "gpu_average_package_power" "node_cpu_seconds_total" )
--------------------------------------------------
EOF
  exit 1
}

# Safely initialize arrays for METRICS and IPS
declare -a METRICS
declare -a IPS
IP="" # For backward compatibility
MINUTES=60
CUSTOM_STEP=""

# Parse arguments
if [ $# -eq 0 ]; then
  # No arguments: use all defaults (remote-friendly mode)
  echo "📊 Running with defaults (all metrics, all IPs, last 60 minutes)"
elif [ $# -eq 1 ] && [[ "$1" != --* ]] && [ "$1" != "-h" ]; then
  # Single non-flag argument: treat as config file (legacy mode)
  CONFIG_FILE="$1"
  if [ "$CONFIG_FILE" = "-h" ] || [ "$CONFIG_FILE" = "--help" ]; then usage; fi
  if [ ! -f "$CONFIG_FILE" ]; then echo "❌ Error: Configuration file not found: $CONFIG_FILE"; exit 1; fi
  echo "📋 Loading configuration file: $CONFIG_FILE"
  source "$CONFIG_FILE"
else
  # CLI argument mode
  while [ $# -gt 0 ]; do
    case "$1" in
      -h|--help) usage ;;
      --minutes) MINUTES="$2"; shift 2 ;;
      --ips)
        IFS=',' read -ra IPS <<< "$2"; shift 2 ;;
      --metrics)
        IFS=',' read -ra METRICS <<< "$2"; shift 2 ;;
      --step)
        CUSTOM_STEP="$2"; shift 2 ;;
      *)
        echo "❌ Unknown option: $1"; usage ;;
    esac
  done
fi

# Backward compatibility: If IP="xxx" is used, add it to the IPS array
if [ -n "$IP" ]; then IPS+=("$IP"); fi

PROMETHEUS_URL="http://localhost:9090"
OUTPUT_DIR="./metrics_export"
STEP="${CUSTOM_STEP:-15s}"

# Check and install required packages if missing
for cmd in curl jq; do
  if ! command -v $cmd >/dev/null 2>&1; then
    echo "📦 Installing $cmd..."
    sudo apt-get update -qq && sudo apt-get install -y $cmd
  fi
done

# Fetch all metrics if not specified in the config
if [ ${#METRICS[@]} -eq 0 ]; then
  echo "📊 No metrics specified. Fetching all available metrics..."
  ALL_METRICS_RESPONSE=$(curl -s -G "${PROMETHEUS_URL}/api/v1/label/__name__/values")
  if [ $? -ne 0 ] || [ -z "$ALL_METRICS_RESPONSE" ]; then echo "❌ Error: Failed to connect to Prometheus at $PROMETHEUS_URL"; exit 1; fi
  readarray -t METRICS < <(echo "$ALL_METRICS_RESPONSE" | jq -r '.data[]' 2>/dev/null)
  if [ ${#METRICS[@]} -eq 0 ]; then echo "❌ Error: No metrics found in Prometheus."; exit 1; fi
  echo "✅ Found ${#METRICS[@]} metrics."
fi

# Validate Multi-IPs
if [ ${#IPS[@]} -gt 0 ]; then
  echo "🔍 Validating target IPs: ${IPS[*]}"
  AVAILABLE_RESPONSE=$(curl -s -G "${PROMETHEUS_URL}/api/v1/query" --data-urlencode "query=up")
  AVAILABLE_IPS=$(echo "$AVAILABLE_RESPONSE" | jq -r '.data.result[].metric.instance' 2>/dev/null | cut -d: -f1 | sort -u)
  
  for TARGET_IP in "${IPS[@]}"; do
    if ! echo "$AVAILABLE_IPS" | grep -F -q "$TARGET_IP"; then
      echo "❌ Error: IP ($TARGET_IP) does not exist in Prometheus."
      exit 1
    fi
  done
  echo "✅ All target IPs validated."
fi

# Time calculation
END_TIMESTAMP=$(date +%s)
START_TIMESTAMP=$((END_TIMESTAMP - MINUTES * 60))

# Create output directories
mkdir -p "$OUTPUT_DIR"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
EXPORT_DIR="$OUTPUT_DIR/export_$TIMESTAMP"
mkdir -p "$EXPORT_DIR"

echo "=========================================="
echo "📊 Starting CSV Export"
echo "=========================================="
echo "Period: $(date -d @$START_TIMESTAMP '+%Y-%m-%d %H:%M:%S') ~ $(date -d @$END_TIMESTAMP '+%Y-%m-%d %H:%M:%S') ($MINUTES mins)"
echo "Target IPs: $(if [ ${#IPS[@]} -eq 0 ]; then echo "ALL"; else echo "${IPS[*]}"; fi)"
echo "Output path: $EXPORT_DIR"

# Function to export a single metric
export_metric() {
  local METRIC="$1"
  echo "📥 Exporting: $METRIC"
  
  RESPONSE=$(curl -s -G "${PROMETHEUS_URL}/api/v1/query_range" \
    --data-urlencode "query=$METRIC" \
    --data-urlencode "start=$START_TIMESTAMP" \
    --data-urlencode "end=$END_TIMESTAMP" \
    --data-urlencode "step=$STEP")
  
  if [ "$(echo "$RESPONSE" | jq -r '.status')" != "success" ]; then echo "  ⚠️ Warning: Query failed"; return; fi
  if [ "$(echo "$RESPONSE" | jq -r '.data.result | length')" -eq 0 ]; then return; fi
  
  BASE_FILENAME=$(echo "$METRIC" | sed 's/[^a-zA-Z0-9_]/_/g' | tr '[:upper:]' '[:lower:]')
  INSTANCES=$(echo "$RESPONSE" | jq -r '.data.result[].metric.instance' | sort -u)
  
  # Multi-IP filtering logic
  if [ ${#IPS[@]} -gt 0 ]; then
    FILTERED_INSTANCES=""
    for INSTANCE in $INSTANCES; do
      VM_IP=$(echo "$INSTANCE" | cut -d: -f1)
      for TARGET_IP in "${IPS[@]}"; do
        if [ "$VM_IP" = "$TARGET_IP" ]; then
          FILTERED_INSTANCES="$FILTERED_INSTANCES $INSTANCE"
          break
        fi
      done
    done
    INSTANCES=$FILTERED_INSTANCES
    if [ -z "$INSTANCES" ]; then return; fi
  fi
  
  for INSTANCE in $INSTANCES; do
    VM_IP=$(echo "$INSTANCE" | cut -d: -f1)
    IP_SAFE=$(echo "$VM_IP" | tr '.' '_')
    CSV_FILE="$EXPORT_DIR/${BASE_FILENAME}_${IP_SAFE}.csv"
    
    HEADER=$(echo "$RESPONSE" | jq -r --arg inst "$INSTANCE" '
      .data.result[] | select(.metric.instance == $inst) | .metric | to_entries | sort_by(.key) | map(.key) | join(",")
    ' | head -1)
    echo "timestamp,datetime,${HEADER},value" > "$CSV_FILE"
    
    echo "$RESPONSE" | jq -r --arg inst "$INSTANCE" '
      .data.result[] | select(.metric.instance == $inst) as $result |
      $result.values[] as $values |
      [
        $values[0],
        ($values[0] | floor | strftime("%Y-%m-%d %H:%M:%S"))
      ] + 
      ([$result.metric | to_entries | sort_by(.key) | .[].value]) + 
      [$values[1]]
      | @csv
    ' >> "$CSV_FILE"
    
    LINE_COUNT=$(wc -l < "$CSV_FILE")
    echo "  ✅ Saved: $(basename "$CSV_FILE") ($((LINE_COUNT - 1)) rows)"
  done
}

for metric in "${METRICS[@]}"; do
  export_metric "$metric"
done

echo "=========================================="
echo "🎉 Export completed! Output directory: $EXPORT_DIR"