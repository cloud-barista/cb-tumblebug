#!/bin/bash

# This script installs vLLM and sets up the environment for LLM serving.
# vLLM is a high-throughput and memory-efficient inference engine for LLMs.
# https://docs.vllm.ai/
#
# Usage:
#   curl -fsSL <url> | bash        (pipe to bash, not sh)
#   bash deployvLLM.sh             (direct execution)

# Ensure script runs with bash
if [ -z "$BASH_VERSION" ]; then
  # Check if running from pipe (stdin is not a terminal)
  if [ ! -t 0 ]; then
    echo "Error: This script requires bash. Please use:"
    echo "  curl -fsSL <url> | bash"
    exit 1
  else
    # Running as a file, re-execute with bash
    exec /bin/bash "$0" "$@"
  fi
fi

# Set strict mode for better error handling
set -e

echo "=========================================="
echo "vLLM Installation and Setup"
echo "=========================================="

# Detect GPU type
echo "Detecting GPU hardware..."

if command -v nvidia-smi >/dev/null 2>&1; then
  echo "Found NVIDIA GPU(s):"
  nvidia-smi --query-gpu=name,memory.total --format=csv,noheader
  GPU_COUNT=$(nvidia-smi --query-gpu=name --format=csv,noheader | wc -l)
  GPU_TYPE="nvidia"
  echo "Detected $GPU_COUNT NVIDIA GPU(s)"
elif command -v rocm-smi >/dev/null 2>&1; then
  echo "Found AMD GPU(s):"
  rocm-smi --showproductname
  GPU_COUNT=$(rocm-smi -i | grep -c "GPU\[")
  GPU_TYPE="amd"
  echo "Detected $GPU_COUNT AMD GPU(s)"
else
  echo "Error: No supported GPU found. vLLM requires either:"
  echo "  - NVIDIA GPU with CUDA (nvidia-smi must be available)"
  echo "  - AMD GPU with ROCm (rocm-smi must be available)"
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

# AMD ROCm pre-built vllm wheels are Python 3.12 only (cp312).
# Install python3.12 for AMD; use system python3 for NVIDIA.
if [ "$GPU_TYPE" = "amd" ]; then
  sudo apt-get install -y python3-pip curl jq > /dev/null 2>&1
  # python3.12 may not be in default repos on Ubuntu 22.04 — add deadsnakes PPA
  if ! apt-cache show python3.12 > /dev/null 2>&1; then
    echo "  Adding deadsnakes PPA for python3.12..."
    sudo apt-get install -y software-properties-common > /dev/null 2>&1
    sudo add-apt-repository -y ppa:deadsnakes/ppa > /dev/null 2>&1
    sudo apt-get update -qq
  fi
  sudo apt-get install -y python3.12 python3.12-venv python3.12-dev > /dev/null 2>&1
  PYTHON_BIN="python3.12"
else
  sudo apt-get install -y python3-pip python3-venv curl jq > /dev/null 2>&1
  PYTHON_BIN="python3"
fi
echo "Using $($PYTHON_BIN --version)"

# Setup virtual environment
VENV_PATH="$HOME/venv_vllm"
echo "Setting up Python virtual environment at $VENV_PATH..."

if [ ! -d "$VENV_PATH" ]; then
  "$PYTHON_BIN" -m venv "$VENV_PATH"
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
echo "Installing vLLM for $GPU_TYPE GPU(s) (this may take a few minutes)..."
LOG_FILE="$HOME/vllm_install.log"
echo "Logging vLLM installation details to $LOG_FILE"

set +e  # Temporarily disable exit on error for pip install

if [ "$GPU_TYPE" = "nvidia" ]; then
  pip install -U vllm > "$LOG_FILE" 2>&1
  INSTALL_RESULT=$?
  if [ $INSTALL_RESULT -ne 0 ]; then
    echo "Failed with default wheels. Trying CUDA 12.1 wheels..."
    pip install vllm --extra-index-url https://download.pytorch.org/whl/cu121 >> "$LOG_FILE" 2>&1
    INSTALL_RESULT=$?
  fi
else
  # === AMD ROCm: install pre-built ROCm vllm wheel via uv ===
  # Pre-built wheels available at wheels.vllm.ai/rocm/ (Python 3.12 / ROCm 7.0+).
  # uv must be used: it gives --extra-index-url higher priority than PyPI,
  # preventing pip from selecting the CUDA wheel from PyPI instead.
  ROCM_VERSION=$(cat /opt/rocm/.info/version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+' | head -1)
  ROCM_VERSION=${ROCM_VERSION:-$(rocm-smi --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+' | head -1)}
  echo "Installed ROCm version: ${ROCM_VERSION:-unknown}"

  # Install uv if not already available
  if ! command -v uv &>/dev/null; then
    echo "Installing uv (fast Python package manager)..."
    pip install uv >> "$LOG_FILE" 2>&1
  fi

  echo "Installing pre-built ROCm vllm wheel from https://wheels.vllm.ai/rocm/ ..."
  uv pip install vllm --extra-index-url "https://wheels.vllm.ai/rocm/" >> "$LOG_FILE" 2>&1
  INSTALL_RESULT=$?

  if [ $INSTALL_RESULT -eq 0 ]; then
    HIP_VER=$(python -c "import torch; print(torch.version.hip or '')" 2>/dev/null || true)
    if [ -z "$HIP_VER" ]; then
      echo "ERROR: vllm installed but torch has no HIP support (CUDA wheel was selected)."
      echo "  This should not happen with uv. Check $LOG_FILE for details."
      INSTALL_RESULT=1
    else
      echo "  Pre-built ROCm vllm installed (torch HIP: $HIP_VER)"
    fi
  else
    echo "ERROR: Pre-built ROCm vllm wheel installation failed."
    echo "  Possible reasons:"
    echo "    - No wheel available for ROCm ${ROCM_VERSION} / Python 3.12"
    echo "    - Check available wheels: https://wheels.vllm.ai/rocm/"
    echo "  See $LOG_FILE for details."
  fi
fi

set -e

if [ $INSTALL_RESULT -ne 0 ]; then
  echo "vLLM installation failed. See $LOG_FILE for detailed error messages."
  exit 1
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

# Display completion message
echo "=========================================="
echo "vLLM Installation Complete! (GPU: $GPU_TYPE, Count: $GPU_COUNT)"
echo "=========================================="
echo ""
echo "To serve a model, use servevLLM.sh script:"
echo "  curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/servevLLM.sh | bash -s -- <model_name>"
echo ""
echo "Or manually:"
echo "  source ~/venv_vllm/bin/activate"
echo "  python -m vllm.entrypoints.openai.api_server --model <model_name> --host 0.0.0.0 --port 8000"
echo ""
echo "Recommended models:"
echo "  - Qwen/Qwen2.5-1.5B-Instruct (small, fast, ~3GB VRAM)"
echo "  - meta-llama/Llama-3.2-3B-Instruct (~7GB VRAM)"
echo "  - mistralai/Mistral-7B-Instruct-v0.3 (~15GB VRAM)"
echo "  - deepseek-ai/DeepSeek-R1-Distill-Qwen-7B (~15GB VRAM)"
echo ""
