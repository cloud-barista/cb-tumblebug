#!/bin/bash
set -e

# ==========================================
# 1. Default Configuration
# ==========================================
declare -a TARGET_IPS
PORT="8000"
PROFILE="sweep"
MAX_SECONDS=30
INPUT_LEN=256
OUTPUT_LEN=128

# Dataset Options
DATA=""
DATA_ARGS=""
DATA_COLUMN_MAPPER=""
DATA_SAMPLES=-1
PROCESSOR=""

# ==========================================
# 2. Argument Parsing
# ==========================================
usage() {
  echo "Usage: $0 --ip <IP1> [IP2 IP3 ...] [OPTIONS]"
  echo ""
  echo "Required:"
  echo "  --ip <IP1> [IP2 ...]   Target GPU VM IP address(es) (space-separated)"
  echo ""
  echo "Options:"
  echo "  --port <PORT>          (Default: $PORT)"
  echo "  --profile <TYPE>       sweep, concurrent, constant etc. (Default: $PROFILE)"
  echo "  --seconds <N>          Maximum duration in seconds per target (Default: $MAX_SECONDS)"
  echo ""
  echo "Dataset Options (if not specified, uses synthetic data):"
  echo "  --data <SOURCE>        Data source: HF dataset ID, file path, or 'prompt_tokens=N,output_tokens=M'"
  echo "  --data-args <JSON>     Dataset loading arguments (e.g., '{\"name\":\"3.0.0\"}')"
  echo "  --data-column-mapper <JSON>  Column mappings (e.g., '{\"text_column\":\"article\"}')"
  echo "  --data-samples <N>     Number of samples, -1 for all (Default: $DATA_SAMPLES)"
  echo "  --processor <NAME>     Tokenizer/processor name"
  echo ""
  echo "Synthetic Data Options (used when --data not specified):"
  echo "  --in-len <N>           Input prompt tokens (Default: $INPUT_LEN)"
  echo "  --out-len <N>          Output generated tokens (Default: $OUTPUT_LEN)"
  echo ""
  echo "  -h, --help             Show this help message"
  echo ""
  echo "Examples:"
  echo "  # Single target (synthetic data)"
  echo "  $0 --ip 15.223.5.153"
  echo ""
  echo "  # Multiple targets"
  echo "  $0 --ip 15.223.5.153 10.0.1.5 20.30.40.50 --seconds 120"
  echo ""
  echo "  # HuggingFace dataset"
  echo "  $0 --ip 15.223.5.153 \\"
  echo "    --data 'abisee/cnn_dailymail' \\"
  echo "    --data-args '{\"name\":\"3.0.0\"}' \\"
  echo "    --data-column-mapper '{\"text_column\":\"article\"}'"
  exit 1
}

while [[ "$#" -gt 0 ]]; do
    case $1 in
        --ip)
            shift
            # Collect all IPs until next option or end of args
            while [[ "$#" -gt 0 ]] && [[ "$1" != --* ]]; do
                TARGET_IPS+=("$1")
                shift
            done
            continue  # skip the trailing shift
            ;;
        --port) PORT="$2"; shift ;;
        --profile) PROFILE="$2"; shift ;;
        --seconds) MAX_SECONDS="$2"; shift ;;
        --in-len) INPUT_LEN="$2"; shift ;;
        --out-len) OUTPUT_LEN="$2"; shift ;;
        --data) DATA="$2"; shift ;;
        --data-args) DATA_ARGS="$2"; shift ;;
        --data-column-mapper) DATA_COLUMN_MAPPER="$2"; shift ;;
        --data-samples) DATA_SAMPLES="$2"; shift ;;
        --processor) PROCESSOR="$2"; shift ;;
        -h|--help) usage ;;
        *) echo "Error: Unknown parameter: $1"; usage ;;
    esac
    shift
done

if [ ${#TARGET_IPS[@]} -eq 0 ]; then
  echo "Error: At least one target IP address (--ip) is required."
  usage
fi

# ==========================================
# 3. System Requirements & Setup
# ==========================================
export DEBIAN_FRONTEND=noninteractive

install_python_venv() {
  echo "Installing Python3 and venv..."
  sudo apt-get update -qq
  PY_VER=$(python3 -c 'import sys; print(f"{sys.version_info.major}.{sys.version_info.minor}")' 2>/dev/null || echo "3")
  # Install base Python and venv support; fail fast on errors
  sudo apt-get install -y python3 python3-pip python3-venv
  # Optionally install the version-specific venv package; tolerate absence
  sudo apt-get install -y "python${PY_VER}-venv" 2>/dev/null || true
}

if ! command -v python3 >/dev/null 2>&1; then
  install_python_venv
fi

WORK_DIR="$HOME/guidellm_bench"
mkdir -p "$WORK_DIR"
cd "$WORK_DIR"

# Ensure valid venv exists (remove incomplete venv if activate is missing)
if [ -d "venv" ] && [ ! -f "venv/bin/activate" ]; then
  echo "Removing incomplete virtual environment..."
  rm -rf venv
fi

if [ ! -d "venv" ]; then
  echo "Creating virtual environment..."
  if ! python3 -m venv venv; then
    install_python_venv
    python3 -m venv venv || { echo "Error: Failed to create virtual environment"; exit 1; }
  fi
fi

source venv/bin/activate

# Check if GuideLLM is installed, install only if needed
if ! python3 -c "import guidellm" 2>/dev/null; then
  echo "Installing GuideLLM..."
  pip install -q --upgrade pip
  pip install -q "guidellm[recommended]"
else
  echo "GuideLLM already installed ✓"
fi

# ==========================================
# 4. Run Benchmark
# ==========================================

# Shared run timestamp (all VMs in the same run share this)
RUN_TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# Function to run benchmark for a single target IP
run_benchmark() {
  local TARGET_IP="$1"
  local TARGET_URL="http://${TARGET_IP}:${PORT}"
  local FILE_PREFIX="bench_${RUN_TIMESTAMP}_${TARGET_IP}"
  local TMP_DIR=$(mktemp -d "$WORK_DIR/.tmp_bench_XXXXXX")

  if [ -n "$DATA" ]; then
    # Use custom dataset (HuggingFace, file, or custom synthetic)
    echo "------------------------------------------"
    echo "Target:   $TARGET_URL"
    echo "Profile:  $PROFILE (Max $MAX_SECONDS seconds)"
    echo "Data:     $DATA"
    if [ -n "$DATA_ARGS" ]; then echo "  Args: $DATA_ARGS"; fi
    if [ -n "$DATA_COLUMN_MAPPER" ]; then echo "  Mapper: $DATA_COLUMN_MAPPER"; fi
    if [ "$DATA_SAMPLES" != "-1" ]; then echo "  Samples: $DATA_SAMPLES"; fi
    if [ -n "$PROCESSOR" ]; then echo "  Processor: $PROCESSOR"; fi
    echo "Output:   $WORK_DIR/${FILE_PREFIX}.*"
    echo "------------------------------------------"

    # Build command safely using an array to avoid eval-based injection
    local -a GUIDELLM_CMD_ARGS=(
      guidellm
      benchmark
      --target "$TARGET_URL"
      --profile "$PROFILE"
      --max-seconds "$MAX_SECONDS"
      --data "$DATA"
      --output-dir "$TMP_DIR"
    )

    if [ -n "$DATA_ARGS" ]; then
      GUIDELLM_CMD_ARGS+=(--data-args "$DATA_ARGS")
    fi
    if [ -n "$DATA_COLUMN_MAPPER" ]; then
      GUIDELLM_CMD_ARGS+=(--data-column-mapper "$DATA_COLUMN_MAPPER")
    fi
    if [ "$DATA_SAMPLES" != "-1" ]; then
      GUIDELLM_CMD_ARGS+=(--data-samples "$DATA_SAMPLES")
    fi
    if [ -n "$PROCESSOR" ]; then
      GUIDELLM_CMD_ARGS+=(--processor "$PROCESSOR")
    fi

    "${GUIDELLM_CMD_ARGS[@]}"
  else
    # Use synthetic data (default)
    local DATA_SOURCE="prompt_tokens=${INPUT_LEN},output_tokens=${OUTPUT_LEN}"

    echo "------------------------------------------"
    echo "Target:  $TARGET_URL"
    echo "Profile: $PROFILE (Max $MAX_SECONDS seconds)"
    echo "Data:    $INPUT_LEN prompt tokens / $OUTPUT_LEN output tokens (synthetic)"
    echo "Output:  $WORK_DIR/${FILE_PREFIX}.*"
    echo "------------------------------------------"

    guidellm benchmark \
      --target "$TARGET_URL" \
      --profile "$PROFILE" \
      --max-seconds "$MAX_SECONDS" \
      --data "$DATA_SOURCE" \
      --output-dir "$TMP_DIR"
  fi

  # Move results with descriptive filenames, then clean up temp dir
  for ext in json csv html; do
    if [ -f "$TMP_DIR/benchmarks.$ext" ]; then
      mv "$TMP_DIR/benchmarks.$ext" "$WORK_DIR/${FILE_PREFIX}.$ext"
    fi
  done
  rm -rf "$TMP_DIR"

  # Report only files that were actually generated
  local GENERATED_FILES=()
  for ext in json csv html; do
    if [ -f "$WORK_DIR/${FILE_PREFIX}.$ext" ]; then
      GENERATED_FILES+=("${FILE_PREFIX}.$ext")
    fi
  done

  echo "------------------------------------------"
  echo "Benchmark completed for $TARGET_IP"
  if [ ${#GENERATED_FILES[@]} -gt 0 ]; then
    for f in "${GENERATED_FILES[@]}"; do echo "  $f"; done
  else
    echo "  (no output files generated)"
  fi
  echo "------------------------------------------"
}

# Run benchmarks for all target IPs
echo "=========================================="
echo "GuideLLM Benchmark"
echo "Targets: ${#TARGET_IPS[@]} node(s)"
for ip in "${TARGET_IPS[@]}"; do echo "  - $ip"; done
echo "=========================================="

TOTAL=${#TARGET_IPS[@]}
FAILED=0

if [ "$TOTAL" -eq 1 ]; then
  # Single target: run directly (no background overhead)
  echo ""
  echo "=========================================="
  echo "[1/1] Benchmarking: ${TARGET_IPS[0]}"
  echo "=========================================="
  if run_benchmark "${TARGET_IPS[0]}"; then
    echo "✓ ${TARGET_IPS[0]} completed"
  else
    echo "✗ ${TARGET_IPS[0]} failed"
    FAILED=1
  fi
else
  # Multiple targets: run in parallel
  echo "Mode: parallel (all targets simultaneously)"
  echo ""

  declare -A PIDS          # PID -> IP mapping
  declare -A LOG_FILES     # IP -> log file mapping

  for ip in "${TARGET_IPS[@]}"; do
    LOG_FILE="$WORK_DIR/.bench_log_${RUN_TIMESTAMP}_${ip}.log"
    LOG_FILES["$ip"]="$LOG_FILE"

    echo "  Starting benchmark for $ip (background)..."
    run_benchmark "$ip" > "$LOG_FILE" 2>&1 &
    PIDS[$!]="$ip"
  done

  echo ""
  echo "Waiting for ${#PIDS[@]} benchmark(s) to complete..."
  echo ""

  # Wait for all background jobs and collect results
  for pid in "${!PIDS[@]}"; do
    ip="${PIDS[$pid]}"
    if wait "$pid"; then
      echo "✓ $ip completed (PID $pid)"
    else
      echo "✗ $ip failed (PID $pid)"
      FAILED=$((FAILED + 1))
    fi
    # Print the benchmark log
    if [ -f "${LOG_FILES[$ip]}" ]; then
      echo "--- Output from $ip ---"
      cat "${LOG_FILES[$ip]}"
      echo "--- End of $ip ---"
      echo ""
      rm -f "${LOG_FILES[$ip]}"
    fi
  done
fi

# ==========================================
# 5. Archive & Summary
# ==========================================

# Collect all result files from this run
RESULT_FILES=($(ls -1 "$WORK_DIR"/bench_${RUN_TIMESTAMP}_*.{json,csv,html} 2>/dev/null || true))
ARCHIVE_FILE="bench_${RUN_TIMESTAMP}.tar.gz"

echo ""
echo "=========================================="
echo "All benchmarks finished: $((TOTAL - FAILED))/$TOTAL succeeded"
if [ $FAILED -gt 0 ]; then
  echo "⚠ $FAILED target(s) failed"
fi
echo "=========================================="

if [ ${#RESULT_FILES[@]} -gt 0 ]; then
  # Create tar.gz archive of all result files (store flat, no directory prefix)
  tar czf "$WORK_DIR/$ARCHIVE_FILE" -C "$WORK_DIR" $(basename -a "${RESULT_FILES[@]}")
  echo ""
  echo "Result files (${#RESULT_FILES[@]}):"
  for f in "${RESULT_FILES[@]}"; do
    echo "  $(basename "$f")"
  done
  echo ""
  echo "Archive: $WORK_DIR/$ARCHIVE_FILE"
  echo "  ($(du -h "$WORK_DIR/$ARCHIVE_FILE" | cut -f1) compressed)"
else
  echo ""
  echo "No result files generated."
fi
echo "=========================================="