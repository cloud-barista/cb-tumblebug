#!/bin/bash
# GPU VM LLM Setup: vLLM + Model + Telemetry
# Assumes GPU driver is already installed (use installGpuDriver.sh separately)
# Usage: curl -fsSL <url> | bash -s -- [model_name]

if [ -z "$BASH_VERSION" ]; then
  [ ! -t 0 ] && echo "Error: Use 'bash' not 'sh'" && exit 1
  exec /bin/bash "$0" "$@"
fi

set -e

# Config
MODEL_NAME="${1:-Qwen/Qwen2.5-1.5B-Instruct}"
VLLM_PORT="8000"

GITHUB_BASE="https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm"
VLLM_INSTALL_SCRIPT_URL="${GITHUB_BASE}/deployvLLM.sh"
VLLM_SERVE_SCRIPT_URL="${GITHUB_BASE}/servevLLM.sh"
TELEMETRY_SCRIPT_URL="${GITHUB_BASE}/telemetry/setup_gpu_sensor.sh"

echo "=========================================="
echo "LLM Setup: $MODEL_NAME"
echo "=========================================="
echo ""

# Step 1: vLLM Installation
echo "[1/3] Installing vLLM..."
curl -fsSL "$VLLM_INSTALL_SCRIPT_URL" | bash || { echo "✗ vLLM install failed"; exit 1; }
echo "✓ vLLM installed"

# Step 2: Model Serving
echo "[2/3] Starting model: $MODEL_NAME"
curl -fsSL "$VLLM_SERVE_SCRIPT_URL" | bash -s -- "$MODEL_NAME" 0.0.0.0 "$VLLM_PORT" || { echo "✗ Model start failed"; exit 1; }

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
echo "  API: http://<YOUR_VM_IP>:${VLLM_PORT}/v1"
echo "  Metrics: http://<YOUR_VM_IP>:9101/metrics"
echo ""
echo "Test: curl http://<YOUR_VM_IP>:${VLLM_PORT}/v1/models"