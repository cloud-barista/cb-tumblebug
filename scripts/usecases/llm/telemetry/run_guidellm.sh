#!/bin/bash
set -e

# ==========================================
# 1. Default Configuration
# ==========================================
TARGET_IP=""
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
  echo "Usage: $0 --ip <IP_ADDRESS> [OPTIONS]"
  echo ""
  echo "Required:"
  echo "  --ip <IP_ADDRESS>      Target GPU VM IP address"
  echo ""
  echo "Options:"
  echo "  --port <PORT>          (Default: $PORT)"
  echo "  --profile <TYPE>       sweep, concurrent, constant 등 (Default: $PROFILE)"
  echo "  --seconds <N>          Maximum duration in seconds (Default: $MAX_SECONDS)"
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
  echo "  # Synthetic data (default)"
  echo "  $0 --ip 15.223.5.153"
  echo ""
  echo "  # HuggingFace dataset"
  echo "  $0 --ip 15.223.5.153 \\"
  echo "    --data 'abisee/cnn_dailymail' \\"
  echo "    --data-args '{\"name\":\"3.0.0\"}' \\"
  echo "    --data-column-mapper '{\"text_column\":\"article\"}'"
  echo ""
  echo "  # Local file"
  echo "  $0 --ip 15.223.5.153 --data './prompts.json'"
  exit 1
}

while [[ "$#" -gt 0 ]]; do
    case $1 in
        --ip) TARGET_IP="$2"; shift ;;
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

if [ -z "$TARGET_IP" ]; then
  echo "Error: Target IP address (--ip) is required."
  usage
fi

TARGET_URL="http://${TARGET_IP}:${PORT}"

# ==========================================
# 3. System Requirements & Setup
# ==========================================
export DEBIAN_FRONTEND=noninteractive

if ! command -v python3 >/dev/null 2>&1 || ! python3 -m venv -h >/dev/null 2>&1; then
  echo "Installing Python3 and venv..."
  sudo apt-get update -qq && sudo apt-get install -y python3 python3-pip python3-venv >/dev/null 2>&1
fi

WORK_DIR="$HOME/guidellm_bench"
mkdir -p "$WORK_DIR"
cd "$WORK_DIR"

if [ ! -d "venv" ]; then
  echo "Creating virtual environment..."
  python3 -m venv venv
fi

source venv/bin/activate

# GuideLLM 설치 여부 확인 후 필요시에만 설치
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
# 타임스탬프로 결과 저장 디렉토리 생성
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
IP_SAFE=$(echo "$TARGET_IP" | tr '.' '_')
RESULT_DIR="$WORK_DIR/results_${IP_SAFE}_${TIMESTAMP}"
mkdir -p "$RESULT_DIR"

# 데이터 소스 결정 (Custom Dataset vs Synthetic)
if [ -n "$DATA" ]; then
  # Custom 데이터셋 사용 (HuggingFace, file, 또는 custom synthetic)
  echo "------------------------------------------"
  echo "Target:   $TARGET_URL"
  echo "Profile:  $PROFILE (Max $MAX_SECONDS seconds)"
  echo "Data:     $DATA"
  if [ -n "$DATA_ARGS" ]; then
    echo "  Args: $DATA_ARGS"
  fi
  if [ -n "$DATA_COLUMN_MAPPER" ]; then
    echo "  Mapper: $DATA_COLUMN_MAPPER"
  fi
  if [ "$DATA_SAMPLES" != "-1" ]; then
    echo "  Samples: $DATA_SAMPLES"
  fi
  if [ -n "$PROCESSOR" ]; then
    echo "  Processor: $PROCESSOR"
  fi
  echo "Output:   $RESULT_DIR"
  echo "------------------------------------------"
  
  # Build command
  GUIDELLM_CMD="guidellm benchmark --target \"$TARGET_URL\" --profile \"$PROFILE\" --max-seconds \"$MAX_SECONDS\" --data \"$DATA\" --output-dir \"$RESULT_DIR\""
  
  if [ -n "$DATA_ARGS" ]; then
    GUIDELLM_CMD="$GUIDELLM_CMD --data-args '$DATA_ARGS'"
  fi
  
  if [ -n "$DATA_COLUMN_MAPPER" ]; then
    GUIDELLM_CMD="$GUIDELLM_CMD --data-column-mapper '$DATA_COLUMN_MAPPER'"
  fi
  
  if [ "$DATA_SAMPLES" != "-1" ]; then
    GUIDELLM_CMD="$GUIDELLM_CMD --data-samples \"$DATA_SAMPLES\""
  fi
  
  if [ -n "$PROCESSOR" ]; then
    GUIDELLM_CMD="$GUIDELLM_CMD --processor \"$PROCESSOR\""
  fi
  
  eval "$GUIDELLM_CMD"
else
  # Synthetic 데이터 사용 (기본)
  DATA_SOURCE="prompt_tokens=${INPUT_LEN},output_tokens=${OUTPUT_LEN}"
  
  echo "------------------------------------------"
  echo "Target:  $TARGET_URL"
  echo "Profile: $PROFILE (Max $MAX_SECONDS seconds)"
  echo "Data:    $INPUT_LEN prompt tokens / $OUTPUT_LEN output tokens (synthetic)"
  echo "Output:  $RESULT_DIR"
  echo "------------------------------------------"
  
  guidellm benchmark \
    --target "$TARGET_URL" \
    --profile "$PROFILE" \
    --max-seconds "$MAX_SECONDS" \
    --data "$DATA_SOURCE" \
    --output-dir "$RESULT_DIR"
fi

echo "------------------------------------------"
echo "Benchmark completed successfully!"
echo "Results saved in: $RESULT_DIR"
echo ""
echo "📊 Result files:"
echo "   JSON: $RESULT_DIR/benchmarks.json"
echo "   CSV:  $RESULT_DIR/benchmarks.csv"
echo "   HTML: $RESULT_DIR/benchmarks.html"
echo "------------------------------------------"