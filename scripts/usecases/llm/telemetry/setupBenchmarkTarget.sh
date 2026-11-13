#!/bin/bash
# GPU VM LLM Setup: vLLM + Model + Telemetry
# Assumes GPU driver is already installed (use installGpuDriver.sh separately)
# Usage: bash setupBenchmarkTarget.sh --model MODEL [--hf-token TOKEN] [--port PORT]

if [ -z "$BASH_VERSION" ]; then
  [ ! -t 0 ] && echo "Error: Use 'bash' not 'sh'" && exit 1
  exec /bin/bash "$0" "$@"
fi

set -e

# =========================================================
# Argument Parsing
# =========================================================
MODEL_NAME=""
HF_TOKEN="${HF_TOKEN:-}"  # inherit from env if already set (avoids CLI arg exposure)
VLLM_PORT="8000"
TP_SIZE=""
GPU_UTIL=""
CTX_LEN=""
TOOL_PARSER=""
ENABLE_TOOLS="true"
VLLM_VERSION=""

while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --model)       MODEL_NAME="${2:?Error: --model requires a value}";        shift 2 ;;
    --hf-token)    HF_TOKEN="${2?Error: --hf-token requires an argument}";    shift 2 ;;
    --port)        VLLM_PORT="${2:?Error: --port requires a value}";          shift 2 ;;
    --tp)          TP_SIZE="${2:?Error: --tp requires a value}";              shift 2 ;;
    --gpu-util)    GPU_UTIL="${2?Error: --gpu-util requires an argument}";    shift 2 ;;
    --ctx-len)     CTX_LEN="${2?Error: --ctx-len requires an argument}";      shift 2 ;;
    --tool-parser) TOOL_PARSER="${2?Error: --tool-parser requires an argument}"; shift 2 ;;
    --no-tools)    ENABLE_TOOLS="false";                                      shift 1 ;;
    --version)     VLLM_VERSION="${2:?Error: --version requires a value}";    shift 2 ;;
    -h|--help)
      echo "Usage: bash setupBenchmarkTarget.sh [--model MODEL] [--hf-token TOKEN] [--port PORT]"
      echo "       [--tp N] [--gpu-util FLOAT] [--ctx-len N] [--tool-parser NAME] [--no-tools] [--version VER]"
      exit 0 ;;
    *)
      # Backward compatibility: treat first non-flag arg as model name
      if [ -z "$MODEL_NAME" ] && [[ "$1" != --* ]]; then
        MODEL_NAME="$1"; shift
      else
        echo "Unknown parameter: $1"; exit 1
      fi
      ;;
  esac
done

MODEL_NAME="${MODEL_NAME:-Qwen/Qwen2.5-7B-Instruct}"

GITHUB_BASE="${CB_TB_SCRIPT_BASE:-https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm}"
VLLM_INSTALL_SCRIPT_URL="${GITHUB_BASE}/deployvLLM.sh"
VLLM_SERVE_SCRIPT_URL="${GITHUB_BASE}/servevLLM.sh"
TELEMETRY_SCRIPT_URL="${GITHUB_BASE}/telemetry/setup_gpu_sensor.sh"

fetch_llm_script() {
  local src="$1" dst="$2"
  if [[ "$src" == http://* || "$src" == https://* ]]; then
    curl -fsSL "$src" -o "$dst"
  elif [ -f "$src" ]; then
    cp "$src" "$dst"
  else
    echo "Error: Script source not found: $src"
    return 1
  fi
}

echo "=========================================="
echo "LLM Setup: $MODEL_NAME"
echo "=========================================="
echo ""

# Pass HF token via env var (not CLI arg) so it doesn't appear in ps aux output
[ -n "$HF_TOKEN" ] && export HF_TOKEN

tmp_serve="$(mktemp -t servevLLM.XXXXXX.sh)"
fetch_llm_script "$VLLM_SERVE_SCRIPT_URL" "$tmp_serve"
# Verify model access before the multi-minute install so a missing token fails immediately
bash "$tmp_serve" --model "$MODEL_NAME" --check-access || { rm -f "$tmp_serve"; exit 1; }

# Step 1: vLLM Installation
echo "[1/3] Installing vLLM..."
tmp_deploy="$(mktemp -t deployvLLM.XXXXXX.sh)"
fetch_llm_script "$VLLM_INSTALL_SCRIPT_URL" "$tmp_deploy"
DEPLOY_ARGS=()
[ -n "$VLLM_VERSION" ] && DEPLOY_ARGS+=(--version "$VLLM_VERSION")
bash "$tmp_deploy" "${DEPLOY_ARGS[@]}" || { echo "✗ vLLM install failed"; rm -f "$tmp_deploy" "$tmp_serve"; exit 1; }
rm -f "$tmp_deploy"
echo "✓ vLLM installed"

# Step 2: Model Serving (servevLLM.sh already waits for HTTP 200 on /v1/models)
echo "[2/3] Starting model: $MODEL_NAME"
SERVE_ARGS=(--model "$MODEL_NAME" --port "$VLLM_PORT")
[ -n "$TP_SIZE" ]     && SERVE_ARGS+=(--tp "$TP_SIZE")
[ -n "$GPU_UTIL" ]    && SERVE_ARGS+=(--gpu-util "$GPU_UTIL")
[ -n "$CTX_LEN" ]     && SERVE_ARGS+=(--ctx-len "$CTX_LEN")
[ -n "$TOOL_PARSER" ] && SERVE_ARGS+=(--tool-parser "$TOOL_PARSER")
[ "$ENABLE_TOOLS" = "false" ] && SERVE_ARGS+=(--no-tools)
bash "$tmp_serve" "${SERVE_ARGS[@]}" || { echo "✗ Model start failed"; rm -f "$tmp_serve"; exit 1; }
rm -f "$tmp_serve"
echo "✓ vLLM server healthy on port ${VLLM_PORT}"

# Step 3: GPU Telemetry
echo "[3/3] Setting up GPU telemetry..."
tmp_telem="$(mktemp -t setup_gpu_sensor.XXXXXX.sh)"
if fetch_llm_script "$TELEMETRY_SCRIPT_URL" "$tmp_telem"; then
  bash "$tmp_telem" || echo "⚠ Telemetry setup failed (optional)"
else
  echo "⚠ Telemetry script unavailable (optional)"
fi
rm -f "$tmp_telem"
echo "✓ Telemetry configured"

# Complete
echo ""
echo "✓ Setup Complete!"
echo "  Model: $MODEL_NAME"
echo "\$\$ENDPOINT[vLLM API](http://0.0.0.0:${VLLM_PORT}/v1)"
echo "\$\$ENDPOINT[GPU Metrics](http://0.0.0.0:9101/metrics)"