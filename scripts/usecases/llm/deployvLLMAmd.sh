#!/bin/bash
set -e

echo "=========================================="
echo "vLLM Setup for AMD GPU (ROCm)"
echo "=========================================="
# Verify AMD GPU and ROCm installationecho "Checking for AMD GPU..."
if command -v rocm-smi >/dev/null 2>&1; then
  rocm-smi --showproductname
else
  echo "Warning: rocm-smi not found. Please install AMD ROCm drivers first."
  exit 1
fi

# Install Docker if not present
echo "Checking for Docker..."
if ! command -v docker >/dev/null 2>&1; then
  echo "Docker is not installed. Installing Docker..."
  curl -fsSL https://get.docker.com -o get-docker.sh
  sudo sh get-docker.sh
  sudo usermod -aG docker $USER
fi

# Create HuggingFace cache directory for model storage
HF_CACHE_DIR="$HOME/.cache/huggingface"
mkdir -p "$HF_CACHE_DIR"
echo "HuggingFace cache directory ready at $HF_CACHE_DIR"

# Pull official vLLM Docker image with ROCm support
echo "Pulling official ROCm vLLM Docker image..."
sudo docker pull rocm/vllm:latest

echo "=========================================="
echo "vLLM Readiness Complete for AMD!"
echo "=========================================="