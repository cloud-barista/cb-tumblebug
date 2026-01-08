#!/bin/bash

# This script installs vLLM and sets up the environment for LLM serving.
# vLLM is a high-throughput and memory-efficient inference engine for LLMs.
# https://docs.vllm.ai/

# Ensure script runs with bash even when executed via SSH
if [ -z "$BASH_VERSION" ]; then
  exec /bin/bash "$0" "$@"
fi

# Set strict mode for better error handling
set -e

echo "=========================================="
echo "vLLM Installation and Setup"
echo "=========================================="

# Check for NVIDIA GPU (vLLM requires CUDA)
echo "Checking for NVIDIA GPU..."
if command -v nvidia-smi >/dev/null 2>&1; then
  nvidia-smi --query-gpu=name,memory.total --format=csv,noheader
  GPU_COUNT=$(nvidia-smi --query-gpu=name --format=csv,noheader | wc -l)
  echo "Detected $GPU_COUNT GPU(s)"
else
  echo "Warning: nvidia-smi not found. vLLM requires NVIDIA GPU with CUDA support."
  echo "Please install NVIDIA drivers first using installCudaDriver.sh"
  exit 1
fi

# Check system resources
echo "Checking system resources..."
DISK_AVAIL=$(df -BG / | awk 'NR==2 {gsub("G","",$4); print $4}')
df -h / | awk '$NF=="/" {print "Disk - Total: "$2, "Available: "$4}'
free -h | awk '/Mem:/ {print "Memory - Total: "$2, "Available: "$7}'

# Warn if disk space is low (vLLM + models can require 20GB+)
if [ "$DISK_AVAIL" -lt 20 ] 2>/dev/null; then
  echo "Warning: Low disk space (${DISK_AVAIL}GB available). vLLM and models may require 20GB+."
  read -r -t 10 -p "Continue anyway? [y/N]: " CONTINUE || CONTINUE="n"
  if [[ ! "$CONTINUE" =~ ^[Yy]$ ]]; then
    echo "Installation cancelled."
    exit 1
  fi
fi

# Install system dependencies
echo "Installing system dependencies..."
export DEBIAN_FRONTEND=noninteractive
sudo apt-get update -qq
sudo apt-get install -y python3-pip python3-venv curl jq > /dev/null 2>&1

# Setup virtual environment
VENV_PATH="$HOME/venv_vllm"
echo "Setting up Python virtual environment at $VENV_PATH..."

if [ ! -d "$VENV_PATH" ]; then
  python3 -m venv "$VENV_PATH"
  echo "Created new virtual environment."
else
  echo "Virtual environment already exists."
fi

# Activate virtual environment
# shellcheck disable=SC1091
. "$VENV_PATH/bin/activate"

# Upgrade pip
echo "Upgrading pip..."
pip install --upgrade pip > /dev/null 2>&1

# Install vLLM
echo "Installing vLLM (this may take a few minutes)..."
LOG_FILE="$HOME/vllm_install.log"
echo "Logging vLLM installation details to $LOG_FILE"

set +e  # Temporarily disable exit on error for pip install
pip install -U vllm > "$LOG_FILE" 2>&1
INSTALL_RESULT=$?
set -e

if [ $INSTALL_RESULT -ne 0 ]; then
  echo "Failed to install vLLM with default wheels. Trying with CUDA 12.1 wheels..."
  set +e
  pip install vllm --extra-index-url https://download.pytorch.org/whl/cu121 >> "$LOG_FILE" 2>&1
  INSTALL_RESULT=$?
  set -e
  if [ $INSTALL_RESULT -ne 0 ]; then
    echo "vLLM installation failed. See $LOG_FILE for detailed error messages."
    exit 1
  fi
fi

# Verify installation
echo "Verifying vLLM installation..."
if python -c "import vllm; print(f'vLLM version: {vllm.__version__}')" 2>/dev/null; then
  echo "vLLM installed successfully!"
else
  echo "Warning: vLLM import failed. Installation may be incomplete."
  exit 1
fi

# Install additional useful packages
echo "Installing additional packages..."
pip install -U openai transformers huggingface_hub > /dev/null 2>&1

# Create a helper script for serving models
SERVE_SCRIPT="$HOME/vllm-serve.sh"
echo "Creating helper script at $SERVE_SCRIPT..."

cat > "$SERVE_SCRIPT" << 'EOF'
#!/bin/bash

# vLLM Model Serving Helper Script
# Usage: ./vllm-serve.sh <model_name> [options]

# Ensure script runs with bash
if [ -z "$BASH_VERSION" ]; then
  exec /bin/bash "$0" "$@"
fi

VENV_PATH="$HOME/venv_vllm"
# shellcheck disable=SC1091
. "$VENV_PATH/bin/activate"

MODEL=${1:-"Qwen/Qwen2.5-1.5B-Instruct"}
HOST=${2:-"0.0.0.0"}
PORT=${3:-"8000"}

# Validate PORT is a number within valid range
case "$PORT" in
  ''|*[!0-9]*) 
    echo "Error: PORT must be a number between 1 and 65535. Got: $PORT"
    exit 1
    ;;
esac
if [ "$PORT" -lt 1 ] || [ "$PORT" -gt 65535 ]; then
  echo "Error: PORT must be a number between 1 and 65535. Got: $PORT"
  exit 1
fi

# Validate HOST format (basic check for IP or hostname)
# Allow 0.0.0.0, localhost, valid IPs, and hostnames
if [[ ! "$HOST" =~ ^[0-9a-zA-Z.:_-]+$ ]]; then
  echo "Error: HOST contains invalid characters. Got: $HOST"
  exit 1
fi

echo "Starting vLLM server..."
echo "  Model: $MODEL"
echo "  Host: $HOST"
echo "  Port: $PORT"
echo ""
echo "API Endpoints:"
echo "  OpenAI Compatible: http://$HOST:$PORT/v1"
echo "  Models: http://$HOST:$PORT/v1/models"
echo "  Completions: http://$HOST:$PORT/v1/completions"
echo "  Chat: http://$HOST:$PORT/v1/chat/completions"
echo ""

# Run vLLM with OpenAI-compatible API server
python -m vllm.entrypoints.openai.api_server \
  --model "$MODEL" \
  --host "$HOST" \
  --port "$PORT" \
  --trust-remote-code
EOF

chmod +x "$SERVE_SCRIPT"

# Display completion message
echo "=========================================="
echo "vLLM Installation Complete!"
echo "=========================================="
echo ""
echo "To serve a model, run:"
echo "  source ~/venv_vllm/bin/activate"
echo "  python -m vllm.entrypoints.openai.api_server --model <model_name> --host 0.0.0.0 --port 8000"
echo ""
echo "Or use the helper script:"
echo "  ~/vllm-serve.sh <model_name> [host] [port]"
echo ""
echo "Example models:"
echo "  - Qwen/Qwen2.5-1.5B-Instruct (small, fast)"
echo "  - meta-llama/Llama-3.2-3B-Instruct"
echo "  - mistralai/Mistral-7B-Instruct-v0.3"
echo "  - deepseek-ai/DeepSeek-R1-Distill-Qwen-7B"
echo ""
echo "API will be available at: http://<your-ip>:8000/v1"
echo ""
