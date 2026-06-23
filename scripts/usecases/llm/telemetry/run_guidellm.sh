#!/bin/bash
set -e

# Reference: https://github.com/vllm-project/guidellm/tree/main/docs

# ==========================================
# 1. Default Configuration
# ==========================================
declare -a TARGET_IPS
PORT="8000"
PROFILE="sweep"
MAX_SECONDS=30
MAX_REQUESTS=""
RATE=""
RAMPUP=""
MODEL=""
RANDOM_SEED=""
OUTPUTS=""
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
  echo "For more information, visit:"
  echo "https://github.com/vllm-project/guidellm/blob/main/docs/getting-started/benchmark.md"
  echo ""
  echo "Required:"
  echo "  --ip <IP1> [IP2 ...]   Target GPU VM IP address(es) (space-separated)"
  echo ""
  echo "Options:"
  echo "  --port <PORT>                  Server port. Default: $PORT"
  echo "  --profile <TYPE>               Benchmark profile (synchronous, constant, async, sweep, poisson, concurrent, throughput). Default: $PROFILE"
  echo "  --rate <RATE>                  Request rate or number of sweep strategies"
  echo "  --max-seconds <N>              Maximum duration per target in seconds. Default: $MAX_SECONDS"
  echo "  --max-requests <N>             Maximum number of requests per benchmark"
  echo "  --model <NAME>                 Model name to benchmark (e.g. Qwen/Qwen2.5-1.5B-Instruct)"
  echo "  --rampup <N>                   Ramp-up duration in seconds"
  echo "  --random-seed <N>              Random seed for reproducibility"
  echo "  --outputs <FORMATS>            Comma-separated output formats (e.g. csv,json,html). Default: csv,json"
  echo ""
  echo "Dataset Options (uses synthetic data if omitted):"
  echo "  --data <SOURCE>                Dataset source (HF dataset ID or file path)"
  echo "  --data-args <JSON>             Dataset loading arguments (e.g. {\"name\":\"3.0.0\"})"
  echo "  --data-column-mapper <JSON>    Dataset column mappings (e.g. {\"text_column\":\"article\"})"
  echo "  --data-samples <N>             Number of samples (-1 for all). Default: $DATA_SAMPLES"
  echo "  --processor <NAME>             Tokenizer or processor name"
  echo ""
  echo "Synthetic Data Options (used when --data is not specified):"
  echo "  --in-len <N>                   Number of input tokens. Default: $INPUT_LEN"
  echo "  --out-len <N>                  Number of output tokens. Default: $OUTPUT_LEN"
  echo ""
  echo "  -h, --help                     Show this help message"
  echo ""
  echo "Examples:"
  echo "  # Single target (synthetic data)"
  echo "  $1 --ip 1.1.1.1"
  echo ""
  echo "  # Multiple targets"
  echo "  $1 --ip 1.1.1.1 2.2.2.2 --max-seconds 120"
  echo ""
  echo "  # HuggingFace dataset"
  echo "  $1 --ip 1.1.1.1 \\"
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
        --max-seconds) MAX_SECONDS="$2"; shift ;;
        --max-requests) MAX_REQUESTS="$2"; shift ;;
        --rate) RATE="$2"; shift ;;
        --rampup) RAMPUP="$2"; shift ;;
        --model) MODEL="$2"; shift ;;
        --random-seed) RANDOM_SEED="$2"; shift ;;
        --outputs) OUTPUTS="$2"; shift ;;
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
  # Create a unique directory for this specific run
  local RESULT_DIR="$WORK_DIR/bench_${RUN_TIMESTAMP}_${TARGET_IP}"
  mkdir -p "$RESULT_DIR"

  # Build the data source argument dynamically
  local DATA_SOURCE
  if [ -n "$DATA" ]; then
    # If --data is provided, use it directly
    DATA_SOURCE="$DATA"
    echo "------------------------------------------"
    echo "Target:   $TARGET_URL"
    echo "Profile:  $PROFILE (Max $MAX_SECONDS seconds)"
    echo "Data:     $DATA_SOURCE"
    if [ -n "$DATA_ARGS" ]; then echo "  Args: $DATA_ARGS"; fi
    if [ -n "$DATA_COLUMN_MAPPER" ]; then echo "  Mapper: $DATA_COLUMN_MAPPER"; fi
    if [ "$DATA_SAMPLES" != "-1" ]; then echo "  Samples: $DATA_SAMPLES"; fi
    if [ -n "$PROCESSOR" ]; then echo "  Processor: $PROCESSOR"; fi
    echo "Output:   $RESULT_DIR/"
    echo "------------------------------------------"
  else
    # If --data is not provided, construct it from synthetic data options
    DATA_SOURCE="kind=synthetic_text,prompt_tokens=${INPUT_LEN},output_tokens=${OUTPUT_LEN}"
    echo "------------------------------------------"
    echo "Target:  $TARGET_URL"
    echo "Profile: $PROFILE (Max $MAX_SECONDS seconds)"
    echo "Data:    $INPUT_LEN prompt tokens / $OUTPUT_LEN output tokens (synthetic)"
    echo "Output:  $RESULT_DIR/"
    echo "------------------------------------------"
  fi

  # Build command safely using an array to avoid eval-based injection
  local -a GUIDELLM_CMD_ARGS=(
    guidellm
    benchmark
    --target "$TARGET_URL"
    --profile "$PROFILE"
    --data "$DATA_SOURCE"
    # Output directly to the final destination directory
    --output-dir "$RESULT_DIR"
  )

  # Add optional arguments only if they are set
  if [ -n "$MAX_SECONDS" ]; then
    GUIDELLM_CMD_ARGS+=(--max-seconds "$MAX_SECONDS")
  fi
  if [ -n "$MAX_REQUESTS" ]; then
    GUIDELLM_CMD_ARGS+=(--max-requests "$MAX_REQUESTS")
  fi
  if [ -n "$RATE" ]; then
    GUIDELLM_CMD_ARGS+=(--rate "$RATE")
  fi
  if [ -n "$RAMPUP" ]; then
    GUIDELLM_CMD_ARGS+=(--rampup "$RAMPUP")
  fi
  if [ -n "$MODEL" ]; then
    GUIDELLM_CMD_ARGS+=(--model "$MODEL")
  fi
  if [ -n "$RANDOM_SEED" ]; then
    GUIDELLM_CMD_ARGS+=(--random-seed "$RANDOM_SEED")
  fi
  if [ -n "$OUTPUTS" ]; then
    GUIDELLM_CMD_ARGS+=(--outputs "$OUTPUTS")
  fi
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

  # Run the command and capture its exit code
  if ! "${GUIDELLM_CMD_ARGS[@]}"; then
    echo "Error: guidellm benchmark command failed." >&2
    # Clean up the directory if the benchmark failed
    rm -rf "$RESULT_DIR"
    return 1 # Explicitly return a failure code
  fi

  # Report only files that were actually generated
  local GENERATED_FILES=()
  for ext in json csv html; do
    if [ -f "$RESULT_DIR/benchmarks.$ext" ]; then
      GENERATED_FILES+=("$(basename "$RESULT_DIR")/benchmarks.$ext")
    fi
  done

  echo "------------------------------------------"
  echo "Benchmark completed for $TARGET_IP"
  if [ ${#GENERATED_FILES[@]} -gt 0 ]; then
    for f in "${GENERATED_FILES[@]}"; do echo "  $f"; done
  else
    echo "  (no output files generated)"
  fi
  for ext in json csv html; do
    if [ -f "$RESULT_DIR/benchmarks.$ext" ]; then
      echo "\$\$FILEPATH[Results ${ext^^}]($RESULT_DIR/benchmarks.$ext)"
    fi
  done
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

declare -A PIDS          # PID -> IP mapping
declare -A LOG_FILES     # IP -> log file mapping

echo ""
for ip in "${TARGET_IPS[@]}"; do
  LOG_FILE="$WORK_DIR/.bench_log_${RUN_TIMESTAMP}_${ip}.log"
  LOG_FILES["$ip"]="$LOG_FILE"

  echo "  Starting benchmark for $ip ..."
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
    STATUS_MSG="\033[1;32m✓ COMPLETED\033[0m"
  else
    STATUS_MSG="\033[1;31m✗ FAILED\033[0m"
    FAILED=$((FAILED + 1))
  fi
  
  echo -e "\n\033[1;36m======================================================================\033[0m"
  echo -e "  $STATUS_MSG : \033[1;37m$ip\033[0m "
  echo -e "\033[1;36m======================================================================\033[0m"
  
  if [ -f "${LOG_FILES[$ip]}" ]; then
    sed "s/^/[ $ip ] /" "${LOG_FILES[$ip]}"
    rm -f "${LOG_FILES[$ip]}"
  else
    echo "[ $ip ] (No log output found)"
  fi
  
  echo -e "\033[1;36m======================================================================\033[0m\n"
done

# ==========================================
# 5. Summary
# ==========================================

# Collect all result directories from this run
RESULT_DIRS=($(ls -1d "$WORK_DIR"/bench_${RUN_TIMESTAMP}_* 2>/dev/null || true))

echo -e "\n\033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m"
echo -e "\033[1;35m  🏆 [ FINAL SUMMARY ] Benchmark Execution Results\033[0m"
echo -e "\033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m"
echo -e "  Total Targets : $TOTAL VM(s)"
echo -e "  Success       : \033[1;32m$((TOTAL - FAILED))\033[0m"
if [ $FAILED -gt 0 ]; then
  echo -e "  Failed        : \033[1;31m$FAILED\033[0m"
else
  echo -e "  Failed        : 0"
fi
echo -e "\033[1;35m──────────────────────────────────────────────────────────────────────\033[0m"

if [ ${#RESULT_DIRS[@]} -gt 0 ]; then
  echo -e "  \033[1;36m📂 Saved Directories:\033[0m"
  for d in "${RESULT_DIRS[@]}"; do
    echo -e "   - $(basename "$d")"
  done
else
  echo -e "  \033[1;33m⚠ No result files generated.\033[0m"
fi
echo -e "\033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"