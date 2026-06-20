#!/bin/bash

# This script starts the vLLM OpenAI-compatible API server with a specified model.
# It handles checking for existing servers, stopping them if needed, and health checks.
#
# Usage:
#   curl -fsSL <url> | bash -s -- <model_name> [host] [port]
#   bash servevLLM.sh <model_name> [host] [port]
#
# Arguments:
#   model_name  - Required. The HuggingFace model name (e.g., "Qwen/Qwen2.5-1.5B-Instruct")
#   host        - Optional. Host to bind to (default: 0.0.0.0)
#   port        - Optional. Port to listen on (default: 8000)
#
# Examples:
#   bash servevLLM.sh Qwen/Qwen2.5-1.5B-Instruct
#   bash servevLLM.sh meta-llama/Llama-3.2-3B-Instruct 0.0.0.0 8080

# Ensure script runs with bash
if [ -z "$BASH_VERSION" ]; then
  if [ ! -t 0 ]; then
    echo "Error: This script requires bash. Please use:"
    echo "  curl -fsSL <url> | bash -s -- <model_name>"
    exit 1
  else
    exec /bin/bash "$0" "$@"
  fi
fi

# =========================================================
# Argument Parsing (named parameters with backward compat)
# =========================================================
MODEL_NAME=""
HOST="0.0.0.0"
PORT="8000"
HF_TOKEN="${HF_TOKEN:-}"  # inherit from env if already set (avoids CLI arg exposure)
GPU_UTIL=""               # empty = vLLM default (~0.9)
CTX_LEN=""                # empty = model default
API_KEY=""

usage() {
  echo "Usage: bash servevLLM.sh --model MODEL [OPTIONS]"
  echo ""
  echo "Required:"
  echo "  --model MODEL       HuggingFace model name"
  echo ""
  echo "Options:"
  echo "  --host HOST         Bind address (default: 0.0.0.0)"
  echo "  --port PORT         Server port (default: 8000)"
  echo "  --hf-token TOKEN    HuggingFace token for gated models"
  echo "  --gpu-util FLOAT    GPU memory utilization 0.0-1.0 (default: vLLM default)"
  echo "  --ctx-len N         Max context length / max-model-len (default: model default)"
  echo "  --api-key KEY       API key for the vLLM server (default: none)"
  echo ""
  echo "Recommended models:"
  echo "  Qwen/Qwen2.5-1.5B-Instruct       (~3GB VRAM)"
  echo "  meta-llama/Llama-3.2-3B-Instruct  (~7GB VRAM)"
  echo "  mistralai/Mistral-7B-Instruct-v0.3 (~15GB VRAM)"
  exit "${1:-1}"
}

while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --model)    MODEL_NAME="${2:?Error: --model requires a value}";    shift 2 ;;
    --host)     HOST="${2:?Error: --host requires a value}";           shift 2 ;;
    --port)     PORT="${2:?Error: --port requires a value}";           shift 2 ;;
    --hf-token) HF_TOKEN="${2?Error: --hf-token requires an argument}";  shift 2 ;;
    --gpu-util) GPU_UTIL="${2?Error: --gpu-util requires an argument}"; shift 2 ;;
    --ctx-len)  CTX_LEN="${2?Error: --ctx-len requires an argument}";   shift 2 ;;
    --api-key)  API_KEY="${2?Error: --api-key requires an argument}";   shift 2 ;;
    -h|--help)  usage 0 ;;
    *)
      # Backward compatibility: treat first non-flag arg as model name
      if [ -z "$MODEL_NAME" ] && [[ "$1" != --* ]]; then
        MODEL_NAME="$1"; shift
      else
        echo "Error: Unknown parameter: $1"; usage
      fi
      ;;
  esac
done

if [ -z "$MODEL_NAME" ]; then
  echo "Error: --model is required."
  usage
fi

echo "=========================================="
echo "vLLM Model Serving"
echo "=========================================="
echo "Model: $MODEL_NAME"
echo "Host: $HOST"
echo "Port: $PORT"
echo ""

# Configuration
VENV_PATH="$HOME/venv_vllm"
LOG_FILE="$HOME/vllm-serve.log"
PID_FILE="$HOME/vllm-serve.pid"
MODEL_FILE="$HOME/vllm-serve.model"
HEALTH_CHECK_TIMEOUT=300  # 5 minutes max wait for server startup
HEALTH_CHECK_INTERVAL=5   # Check every 5 seconds

# Detect GPU type
if command -v nvidia-smi >/dev/null 2>&1; then
  GPU_TYPE="nvidia"
elif command -v rocm-smi >/dev/null 2>&1; then
  GPU_TYPE="amd"
  export VLLM_TARGET_DEVICE=rocm
else
  echo "Error: No supported GPU found (nvidia-smi or rocm-smi required)."
  exit 1
fi
echo "GPU type: $GPU_TYPE"

# Check if venv exists
if [ ! -d "$VENV_PATH" ]; then
  echo "Error: vLLM virtual environment not found at $VENV_PATH"
  echo "Please run deployvLLM.sh first to install vLLM."
  exit 1
fi

# Activate virtual environment
echo "Activating virtual environment..."
# shellcheck disable=SC1091
. "$VENV_PATH/bin/activate"

# Patch prometheus-fastapi-instrumentator for AMD/ROCm: pre-built wheels (vLLM 0.23.x+)
# bundle an old version that crashes on every HTTP request with:
#   AttributeError: '_IncludedRouter' object has no attribute 'path'
if [ "$GPU_TYPE" = "amd" ]; then
  pip install -q -U "prometheus-fastapi-instrumentator>=7.0.0" > /dev/null 2>&1 || \
    echo "Warning: Could not upgrade prometheus-fastapi-instrumentator (non-fatal)"
fi

# Function to get the currently running model
get_running_model() {
  if [ -f "$MODEL_FILE" ] && [ -f "$PID_FILE" ]; then
    local pid
    pid=$(cat "$PID_FILE" 2>/dev/null)
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
      cat "$MODEL_FILE" 2>/dev/null
      return 0
    fi
  fi
  echo ""
  return 1
}

# Function to stop existing vLLM server
stop_vllm_server() {
  echo "Stopping existing vLLM server..."

  # Try PID file first
  if [ -f "$PID_FILE" ]; then
    local pid
    pid=$(cat "$PID_FILE" 2>/dev/null)
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
      echo "Stopping process $pid..."
      kill "$pid" 2>/dev/null
      sleep 2
      # Force kill if still running
      if kill -0 "$pid" 2>/dev/null; then
        kill -9 "$pid" 2>/dev/null
        sleep 1
      fi
    fi
    rm -f "$PID_FILE"
  fi

  # Also try to find and kill any vllm processes on the same port
  local vllm_pids
  vllm_pids=$(pgrep -f "vllm.entrypoints.openai.api_server.*--port $PORT" 2>/dev/null || true)
  if [ -n "$vllm_pids" ]; then
    echo "Found additional vLLM processes: $vllm_pids"
    echo "$vllm_pids" | xargs kill 2>/dev/null || true
    sleep 2
    echo "$vllm_pids" | xargs kill -9 2>/dev/null || true
  fi

  rm -f "$MODEL_FILE"
  echo "Server stopped."
}

# Function to check server health
check_health() {
  local url="http://localhost:$PORT/v1/models"
  curl -s -o /dev/null -w "%{http_code}" "$url" 2>/dev/null
}

# Function to wait for server to be ready
wait_for_server() {
  local elapsed=0
  echo "Waiting for server to be ready..."

  while [ $elapsed -lt $HEALTH_CHECK_TIMEOUT ]; do
    local status
    status=$(check_health)
    if [ "$status" = "200" ]; then
      echo "Server is ready!"
      return 0
    fi

    # Check if process is still running
    if [ -f "$PID_FILE" ]; then
      local pid
      pid=$(cat "$PID_FILE" 2>/dev/null)
      if [ -n "$pid" ] && ! kill -0 "$pid" 2>/dev/null; then
        echo "Error: Server process died unexpectedly."
        echo "Check log file for details: $LOG_FILE"
        tail -20 "$LOG_FILE" 2>/dev/null || true
        return 1
      fi
    fi

    printf "."
    sleep $HEALTH_CHECK_INTERVAL
    elapsed=$((elapsed + HEALTH_CHECK_INTERVAL))
  done

  echo ""
  echo "Error: Server failed to start within ${HEALTH_CHECK_TIMEOUT}s timeout."
  echo "Check log file for details: $LOG_FILE"
  tail -20 "$LOG_FILE" 2>/dev/null || true
  return 1
}

# Check if same model is already running
RUNNING_MODEL=$(get_running_model)
if [ "$RUNNING_MODEL" = "$MODEL_NAME" ]; then
  # Verify server is actually responding
  if [ "$(check_health)" = "200" ]; then
    echo "Model '$MODEL_NAME' is already running and healthy."
    echo "Endpoint: http://$HOST:$PORT/v1"
    echo ""
    echo "To test:"
    echo "  curl http://localhost:$PORT/v1/models"
    exit 0
  else
    echo "Model '$MODEL_NAME' was running but is not responding. Restarting..."
    stop_vllm_server
  fi
elif [ -n "$RUNNING_MODEL" ]; then
  echo "Different model '$RUNNING_MODEL' is currently running."
  stop_vllm_server
fi

# Export HuggingFace token for gated model downloads
if [ -n "$HF_TOKEN" ]; then
  export HF_TOKEN
  export HUGGING_FACE_HUB_TOKEN="$HF_TOKEN"
  echo "HuggingFace token configured."
fi

# Start vLLM server
echo "Starting vLLM server with model: $MODEL_NAME"
echo "Log file: $LOG_FILE"

# Clear old log file
> "$LOG_FILE"

# Build vLLM command dynamically to support optional arguments
VLLM_CMD_ARGS=(
  python -m vllm.entrypoints.openai.api_server
  --model "$MODEL_NAME"
  --host "$HOST"
  --port "$PORT"
  --trust-remote-code
)
[ -n "$GPU_UTIL" ] && VLLM_CMD_ARGS+=(--gpu-memory-utilization "$GPU_UTIL")
[ -n "$CTX_LEN" ]  && VLLM_CMD_ARGS+=(--max-model-len "$CTX_LEN")
[ -n "$API_KEY" ]  && VLLM_CMD_ARGS+=(--api-key "$API_KEY")

# Start server in background
nohup "${VLLM_CMD_ARGS[@]}" >> "$LOG_FILE" 2>&1 &

# Save PID and model name
SERVER_PID=$!
echo $SERVER_PID > "$PID_FILE"
echo "$MODEL_NAME" > "$MODEL_FILE"

echo "Server started with PID: $SERVER_PID"

# Wait for server to be ready
if wait_for_server; then
  echo ""
  echo "=========================================="
  echo "vLLM Server Started Successfully!"
  echo "=========================================="
  echo ""
  echo "Model: $MODEL_NAME"
  echo "Endpoint: http://$HOST:$PORT/v1"
  echo "PID: $SERVER_PID"
  echo "Log: $LOG_FILE"
  echo ""
  echo "Test commands:"
  echo "  curl http://localhost:$PORT/v1/models"
  echo "  curl http://localhost:$PORT/v1/completions -H 'Content-Type: application/json' -d '{\"model\": \"$MODEL_NAME\", \"prompt\": \"Hello\", \"max_tokens\": 50}'"
  echo ""
  exit 0
else
  # Cleanup on failure
  rm -f "$PID_FILE" "$MODEL_FILE"
  exit 1
fi
