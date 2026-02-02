#!/bin/bash

# NVIDIA GPU Driver and Container Toolkit Installation Script
# Supports Ubuntu 22.04/24.04 on bare-metal, VM, and Kubernetes GPU nodes
#
# Features:
#   - NVIDIA Driver installation (required for all GPU workloads)
#   - CUDA Toolkit installation (optional, for direct CUDA development)
#   - NVIDIA Container Toolkit (for Docker/Podman/Kubernetes GPU containers)
#   - containerd/Docker integration (auto-configured if present)
#   - NVSwitch/Fabric Manager support (for multi-GPU systems like H100/A100 HGX)
#
# Usage:
#   ./installCudaDriver.sh                    # Driver + Container Toolkit (recommended)
#   ./installCudaDriver.sh --with-toolkit     # + CUDA Toolkit for development
#   ./installCudaDriver.sh --no-reboot        # Skip automatic reboot
#   ./installCudaDriver.sh --driver-only      # Driver only, no container support
#
# Typical use cases:
#   - Kubernetes GPU node:  ./installCudaDriver.sh
#   - vLLM/Ollama on VM:    ./installCudaDriver.sh
#   - CUDA development:     ./installCudaDriver.sh --with-toolkit
#
# Remote execution (CB-MapUI / CB-Tumblebug API):
#   This script is designed for non-interactive SSH execution.
#   All prompts are suppressed using DEBIAN_FRONTEND, needrestart config, etc.
#   Use --no-reboot for remote execution to prevent SSH connection drop.
#
# References:
#   - CUDA: https://developer.nvidia.com/cuda-downloads
#   - Container Toolkit: https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/

set -e

# ============================================================
# Non-interactive mode for SSH remote execution
# ============================================================
# Prevent all interactive prompts during package installation
export DEBIAN_FRONTEND=noninteractive
export NEEDRESTART_MODE=a
export NEEDRESTART_SUSPEND=1

# Disable needrestart interactive prompts (Ubuntu 22.04+)
# This prevents "Which services should be restarted?" dialog
if [ -d /etc/needrestart/conf.d ]; then
    echo "\$nrconf{restart} = 'a';" | sudo tee /etc/needrestart/conf.d/99-autorestart.conf > /dev/null 2>&1 || true
fi

# ============================================================
# Configuration
# ============================================================
AUTO_REBOOT=true
INSTALL_TOOLKIT=false
INSTALL_CONTAINER_TOOLKIT=true
CUDA_VERSION=""  # Empty = latest, or specify like "12-6"

# ============================================================
# Parse arguments
# ============================================================
while [[ $# -gt 0 ]]; do
    case $1 in
        --no-reboot)
            AUTO_REBOOT=false
            shift
            ;;
        --with-toolkit)
            INSTALL_TOOLKIT=true
            shift
            ;;
        --driver-only)
            INSTALL_CONTAINER_TOOLKIT=false
            shift
            ;;
        --cuda-version)
            CUDA_VERSION="$2"
            shift 2
            ;;
        -h|--help)
            echo "NVIDIA GPU Driver Installation Script"
            echo ""
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --with-toolkit     Install CUDA Toolkit for development (nvcc, libraries)"
            echo "  --driver-only      Install driver only, skip Container Toolkit"
            echo "  --cuda-version VER Specify CUDA version (e.g., 12-6). Default: latest"
            echo "  --no-reboot        Skip automatic reboot after installation"
            echo "  -h, --help         Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                          # Recommended for K8s/vLLM/Ollama"
            echo "  $0 --with-toolkit           # For CUDA C++ development"
            echo "  $0 --cuda-version 12-6      # Specific CUDA version"
            echo ""
            echo "What gets installed:"
            echo "  Default:        NVIDIA Driver + Container Toolkit (~500MB)"
            echo "  --with-toolkit: + CUDA Toolkit (~2-3GB additional)"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use -h or --help for usage information"
            exit 1
            ;;
    esac
done

# ============================================================
# Pre-flight checks
# ============================================================
echo "=========================================="
echo "NVIDIA GPU Driver Installation"
echo "=========================================="
echo ""

# Check for NVIDIA GPU
echo "Checking for NVIDIA GPU..."
GPU_INFO=$(sudo lspci | grep -i nvidia || true)
if [ -z "$GPU_INFO" ]; then
    echo "ERROR: No NVIDIA GPU detected!"
    echo ""
    echo "PCI devices found:"
    sudo lspci | grep -i -E "vga|3d|display" || true
    exit 1
fi

echo "NVIDIA GPU detected:"
echo "$GPU_INFO"
echo ""

# Check disk space
echo "Checking disk space..."
AVAILABLE_GB=$(df -BG / | awk 'NR==2 {gsub("G",""); print $4}')
REQUIRED_GB=5
if [ "$INSTALL_TOOLKIT" = true ]; then
    REQUIRED_GB=10
fi

if [ "$AVAILABLE_GB" -lt "$REQUIRED_GB" ]; then
    echo "WARNING: Low disk space. Available: ${AVAILABLE_GB}GB, Recommended: ${REQUIRED_GB}GB"
fi
df -h / | awk 'NR==1 || NR==2'
echo ""

# Display installation plan
echo "Installation plan:"
echo "  ✓ NVIDIA Driver (latest)"
if [ "$INSTALL_CONTAINER_TOOLKIT" = true ]; then
    echo "  ✓ NVIDIA Container Toolkit"
fi
if [ "$INSTALL_TOOLKIT" = true ]; then
    echo "  ✓ CUDA Toolkit ${CUDA_VERSION:-latest}"
fi
echo ""

# ============================================================
# Add NVIDIA CUDA Repository (Network Repo - lightweight)
# ============================================================
echo "=========================================="
echo "Setting up NVIDIA CUDA Repository..."
echo "=========================================="

# Detect Ubuntu version
UBUNTU_VERSION=$(lsb_release -rs | tr -d '.')
if [[ "$UBUNTU_VERSION" == "2404" ]]; then
    REPO_URL="https://developer.download.nvidia.com/compute/cuda/repos/ubuntu2404/x86_64"
    KEYRING_URL="${REPO_URL}/cuda-keyring_1.1-1_all.deb"
elif [[ "$UBUNTU_VERSION" == "2204" ]]; then
    REPO_URL="https://developer.download.nvidia.com/compute/cuda/repos/ubuntu2204/x86_64"
    KEYRING_URL="${REPO_URL}/cuda-keyring_1.1-1_all.deb"
else
    echo "WARNING: Ubuntu $UBUNTU_VERSION may not be fully supported. Trying Ubuntu 22.04 repo..."
    REPO_URL="https://developer.download.nvidia.com/compute/cuda/repos/ubuntu2204/x86_64"
    KEYRING_URL="${REPO_URL}/cuda-keyring_1.1-1_all.deb"
fi

echo "Repository: $REPO_URL"
echo ""

# Download and install keyring (small file, ~10KB)
echo "Installing CUDA repository keyring..."
KEYRING_FILE=$(mktemp)
if ! wget -q "$KEYRING_URL" -O "$KEYRING_FILE"; then
    echo "ERROR: Failed to download CUDA keyring"
    exit 1
fi

sudo dpkg -i --force-confdef --force-confold "$KEYRING_FILE" 2>/dev/null || sudo dpkg -i "$KEYRING_FILE"
rm -f "$KEYRING_FILE"

# Update package lists
echo "Updating package lists..."
sudo apt-get update -qq

# ============================================================
# Install NVIDIA Driver
# ============================================================
echo ""
echo "=========================================="
echo "Installing NVIDIA Driver..."
echo "=========================================="

# Install driver (this will install the latest compatible driver)
echo "Installing cuda-drivers package..."
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y \
    -o Dpkg::Options::="--force-confdef" \
    -o Dpkg::Options::="--force-confold" \
    cuda-drivers 2>&1 | \
    grep -E "^(Get:|Unpacking|Setting up|cuda-drivers|nvidia)" || true

echo "NVIDIA Driver installation completed."

# ============================================================
# Install CUDA Toolkit (optional)
# ============================================================
if [ "$INSTALL_TOOLKIT" = true ]; then
    echo ""
    echo "=========================================="
    echo "Installing CUDA Toolkit..."
    echo "=========================================="
    
    if [ -n "$CUDA_VERSION" ]; then
        TOOLKIT_PACKAGE="cuda-toolkit-${CUDA_VERSION}"
    else
        TOOLKIT_PACKAGE="cuda-toolkit"
    fi
    
    echo "Installing ${TOOLKIT_PACKAGE}..."
    sudo DEBIAN_FRONTEND=noninteractive apt-get install -y \
        -o Dpkg::Options::="--force-confdef" \
        -o Dpkg::Options::="--force-confold" \
        "$TOOLKIT_PACKAGE" 2>&1 | \
        grep -E "^(Get:|Unpacking|Setting up|cuda)" || true
    
    # Set environment variables for CUDA
    echo "Setting up CUDA environment variables..."
    
    # Find installed CUDA version
    CUDA_PATH=$(ls -d /usr/local/cuda-* 2>/dev/null | sort -V | tail -1)
    if [ -n "$CUDA_PATH" ]; then
        CUDA_VER=$(basename "$CUDA_PATH")
        
        # Add to bashrc if not already present
        if ! grep -q "CUDA_HOME" ~/.bashrc 2>/dev/null; then
            cat >> ~/.bashrc << CUDA_ENV

# CUDA Environment
export CUDA_HOME=${CUDA_PATH}
export PATH=\${CUDA_HOME}/bin\${PATH:+:\${PATH}}
export LD_LIBRARY_PATH=\${CUDA_HOME}/lib64\${LD_LIBRARY_PATH:+:\${LD_LIBRARY_PATH}}
CUDA_ENV
            echo "CUDA environment added to ~/.bashrc"
        fi
    fi
    
    echo "CUDA Toolkit installation completed."
fi

# ============================================================
# Check for NVSwitch and install Fabric Manager
# ============================================================
echo ""
echo "Checking for NVSwitch topology..."
NVSWITCH_PCI=$(sudo lspci | grep -i -E "nvswitch|nvlink" 2>/dev/null || true)
NVSWITCH_DEV=$(ls /dev/nvidia-nvswitch* 2>/dev/null || true)

if [ -n "$NVSWITCH_PCI" ] || [ -n "$NVSWITCH_DEV" ]; then
    echo "NVSwitch detected. Installing NVIDIA Fabric Manager..."
    if [ -n "$NVSWITCH_PCI" ]; then
        echo "  PCI devices: $NVSWITCH_PCI"
    fi
    
    # Install Fabric Manager (version auto-matched with driver)
    sudo DEBIAN_FRONTEND=noninteractive apt-get install -y \
        -o Dpkg::Options::="--force-confdef" \
        -o Dpkg::Options::="--force-confold" \
        nvidia-fabricmanager 2>&1 | \
        grep -E "^(Get:|Setting up|nvidia-fabric)" || true
    
    if [ $? -eq 0 ]; then
        echo "Enabling nvidia-fabricmanager service..."
        sudo systemctl enable nvidia-fabricmanager
        echo "Fabric Manager installed and enabled."
    else
        echo "WARNING: Failed to install nvidia-fabricmanager."
    fi
else
    echo "No NVSwitch detected. Skipping Fabric Manager."
    echo "  (Fabric Manager is only needed for multi-GPU systems with NVSwitch)"
fi

# ============================================================
# Install NVIDIA Container Toolkit
# ============================================================
if [ "$INSTALL_CONTAINER_TOOLKIT" = true ]; then
    echo ""
    echo "=========================================="
    echo "Installing NVIDIA Container Toolkit..."
    echo "=========================================="
    
    # Add NVIDIA Container Toolkit repository
    echo "Adding Container Toolkit repository..."
    curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | \
        sudo gpg --dearmor --yes -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
    
    curl -s -L https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list | \
        sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | \
        sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list > /dev/null
    
    sudo apt-get update -qq
    
    # Install Container Toolkit
    echo "Installing nvidia-container-toolkit..."
    sudo DEBIAN_FRONTEND=noninteractive apt-get install -y \
        -o Dpkg::Options::="--force-confdef" \
        -o Dpkg::Options::="--force-confold" \
        nvidia-container-toolkit 2>&1 | \
        grep -E "^(Get:|Setting up|nvidia-container)" || true
    
    echo "NVIDIA Container Toolkit installed."
    
    # Configure container runtimes
    echo ""
    echo "Configuring container runtimes..."
    
    # Configure containerd (for Kubernetes)
    if command -v containerd &>/dev/null; then
        echo "  Configuring containerd for NVIDIA runtime..."
        sudo nvidia-ctk runtime configure --runtime=containerd 2>/dev/null || true
        
        if systemctl is-active --quiet containerd; then
            sudo systemctl restart containerd
            echo "  ✓ containerd configured and restarted"
        else
            echo "  ✓ containerd configured (will apply on next start)"
        fi
    else
        echo "  - containerd not found (install later for Kubernetes)"
    fi
    
    # Configure Docker (if present)
    if command -v docker &>/dev/null; then
        echo "  Configuring Docker for NVIDIA runtime..."
        sudo nvidia-ctk runtime configure --runtime=docker 2>/dev/null || true
        
        if systemctl is-active --quiet docker; then
            sudo systemctl restart docker
            echo "  ✓ Docker configured and restarted"
        else
            echo "  ✓ Docker configured (will apply on next start)"
        fi
    else
        echo "  - Docker not found (optional)"
    fi
fi

# ============================================================
# Installation Summary
# ============================================================
echo ""
echo "=========================================="
echo "Installation Complete!"
echo "=========================================="
echo ""
echo "Installed components:"
echo "  ✓ NVIDIA Driver"
if [ "$INSTALL_CONTAINER_TOOLKIT" = true ]; then
    echo "  ✓ NVIDIA Container Toolkit"
fi
if [ "$INSTALL_TOOLKIT" = true ]; then
    echo "  ✓ CUDA Toolkit"
fi
if [ -n "$NVSWITCH_PCI" ] || [ -n "$NVSWITCH_DEV" ]; then
    echo "  ✓ NVIDIA Fabric Manager"
fi
echo ""
echo "Supported workloads:"
echo "  • Ollama, vLLM, PyTorch, TensorFlow"
if [ "$INSTALL_CONTAINER_TOOLKIT" = true ]; then
    echo "  • Docker/Podman GPU containers"
    echo "  • Kubernetes GPU pods"
fi
if [ "$INSTALL_TOOLKIT" = true ]; then
    echo "  • CUDA C/C++ development (nvcc)"
fi
echo ""
echo "Verification commands (after reboot):"
echo "  nvidia-smi                    # GPU status"
if [ "$INSTALL_CONTAINER_TOOLKIT" = true ]; then
    echo "  nvidia-ctk --version          # Container Toolkit"
fi
if [ "$INSTALL_TOOLKIT" = true ]; then
    echo "  nvcc --version                # CUDA compiler"
fi
echo ""

# ============================================================
# Reboot handling
# ============================================================
if [ "$AUTO_REBOOT" = true ]; then
    echo "Rebooting in 5 seconds to load the driver..."
    echo "(Use --no-reboot to skip)"
    echo ""
    echo "NOTE: SSH connection will be terminated."
    echo "      Wait ~60 seconds, then reconnect and verify with 'nvidia-smi'"
    # Use nohup to prevent SSH hangup from blocking reboot
    # The sleep allows this script to complete and return success before reboot
    nohup bash -c 'sleep 5 && sudo reboot' > /dev/null 2>&1 &
    exit 0
else
    echo "=========================================="
    echo "REBOOT REQUIRED"
    echo "=========================================="
    echo "Run 'sudo reboot' to complete installation."
fi
