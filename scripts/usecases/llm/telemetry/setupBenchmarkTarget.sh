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

while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --model)    MODEL_NAME="${2:?Error: --model requires a value}";    shift 2 ;;
    --hf-token) HF_TOKEN="${2?Error: --hf-token requires an argument}";   shift 2 ;;
    --port)     VLLM_PORT="${2:?Error: --port requires a value}";      shift 2 ;;
    -h|--help)
      echo "Usage: bash setupBenchmarkTarget.sh [--model MODEL] [--hf-token TOKEN] [--port PORT]"
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

MODEL_NAME="${MODEL_NAME:-Qwen/Qwen2.5-1.5B-Instruct}"

GITHUB_BASE="https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm"
VLLM_INSTALL_SCRIPT_URL="${GITHUB_BASE}/deployvLLM.sh"
VLLM_SERVE_SCRIPT_URL="${GITHUB_BASE}/servevLLM.sh"
TELEMETRY_SCRIPT_URL="${GITHUB_BASE}/telemetry/setup_gpu_sensor.sh"

echo "=========================================="
echo "LLM Setup: $MODEL_NAME"
echo "=========================================="
echo ""

# Pass HF token via env var (not CLI arg) so it doesn't appear in ps aux output
[ -n "$HF_TOKEN" ] && export HF_TOKEN

# Step 1: vLLM Installation
echo "[1/3] Installing vLLM..."
tmp_deploy="$(mktemp -t deployvLLM.XXXXXX.sh)"
curl -fsSL "$VLLM_INSTALL_SCRIPT_URL" -o "$tmp_deploy"
bash "$tmp_deploy" || { echo "✗ vLLM install failed"; rm -f "$tmp_deploy"; exit 1; }
rm -f "$tmp_deploy"
echo "✓ vLLM installed"

# Step 2: Model Serving
echo "[2/3] Starting model: $MODEL_NAME"
tmp_serve="$(mktemp -t servevLLM.XXXXXX.sh)"
curl -fsSL "$VLLM_SERVE_SCRIPT_URL" -o "$tmp_serve"
bash "$tmp_serve" --model "$MODEL_NAME" --port "$VLLM_PORT" || { echo "✗ Model start failed"; rm -f "$tmp_serve"; exit 1; }
rm -f "$tmp_serve"

# Health check
echo "Checking server health..."
for i in {1..30}; do
  curl -s -o /dev/null -w "%{http_code}" "http://localhost:${VLLM_PORT}/v1/models" | grep -q "200" && { echo "✓ vLLM server healthy"; break; }
  [ $i -eq 30 ] && echo "⚠ Health check timeout (server may still be starting)"
  sleep 2
done

# Step 3: GPU Telemetry
echo "[3/3] Setting up GPU telemetry..."
curl -fsSL "$TELEMETRY_SCRIPT_URL" | bash || echo "⚠ Telemetry setup failed (optional)"
echo "✓ Telemetry configured"

# Complete
echo ""
echo "✓ Setup Complete!"
echo "  Model: $MODEL_NAME"
echo "\$\$ENDPOINT[vLLM API](http://0.0.0.0:${VLLM_PORT}/v1)"
echo "\$\$ENDPOINT[GPU Metrics](http://0.0.0.0:9101/metrics)"