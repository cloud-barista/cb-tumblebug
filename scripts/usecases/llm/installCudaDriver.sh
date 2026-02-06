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
set -o pipefail

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
# Fix dpkg/apt state (cleanup from previous failed installations)
# ============================================================
# Wait for any existing apt/dpkg locks
echo "Checking for apt/dpkg locks..."
for i in $(seq 1 12); do
    if ! sudo fuser /var/lib/dpkg/lock-frontend >/dev/null 2>&1 && \
       ! sudo fuser /var/lib/apt/lists/lock >/dev/null 2>&1; then
        break
    fi
    echo "  Waiting for apt lock... (${i}/12)"
    sleep 5
done

# Remove stale lock files (only if no process holds them)
sudo rm -f /var/lib/dpkg/lock-frontend /var/lib/dpkg/lock \
    /var/cache/apt/archives/lock /var/lib/apt/lists/lock 2>/dev/null || true

# Check for ANY nvidia/cuda packages NOT in clean 'installed' state.
# These broken packages poison ALL apt-get commands (including unrelated ones
# like linux-headers), because dpkg tries to re-configure them first and fails.
BROKEN_PKGS=$(dpkg -l 2>/dev/null | grep -E "nvidia|cuda|libnvidia" | grep -v "^ii " | grep -v "^un " | awk '{print $2}' || true)

if [ -n "$BROKEN_PKGS" ]; then
    echo ""
    echo "Found broken NVIDIA/CUDA packages from previous failed install."
    echo "Purging them to restore clean apt state..."
    echo "  Packages: $(echo $BROKEN_PKGS | tr '\n' ' ')"
    
    # Use --force-all: the nuclear option that removes even if scripts fail
    sudo dpkg --force-all --purge $BROKEN_PKGS 2>&1 | tail -5 || true
    
    # Clean up any DKMS leftovers
    sudo rm -rf /var/lib/dkms/nvidia 2>/dev/null || true
    sudo rm -f /var/crash/nvidia-*.crash 2>/dev/null || true
    
    echo "  Done. Reconfiguring remaining packages..."
    sudo dpkg --configure -a 2>/dev/null || true
    echo ""
fi

# ============================================================
# Configuration
# ============================================================
AUTO_REBOOT=true
INSTALL_TOOLKIT=false
INSTALL_CONTAINER_TOOLKIT=true
CUDA_VERSION=""  # Empty = latest, or specify like "12-6"

# Common apt-get options to suppress all interactive prompts
APT_OPTS=(-o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold")
APT_INSTALL=(sudo DEBIAN_FRONTEND=noninteractive apt-get install -y "${APT_OPTS[@]}")

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
echo "========== NVIDIA GPU Driver Installation =========="

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

echo "  $GPU_INFO"

# Check disk space (only warn if low)
AVAILABLE_GB=$(df -BG / | awk 'NR==2 {gsub("G",""); print $4}')
REQUIRED_GB=5
if [ "$INSTALL_TOOLKIT" = true ]; then
    REQUIRED_GB=10
fi
if [ "$AVAILABLE_GB" -lt "$REQUIRED_GB" ]; then
    echo "WARNING: Low disk space. Available: ${AVAILABLE_GB}GB, Recommended: ${REQUIRED_GB}GB"
fi

# ============================================================
# Install DKMS prerequisites (before CUDA repo, from Ubuntu repos)
# ============================================================
echo "\n========== DKMS Prerequisites =========="

# Refresh package index (stale cache causes 404 on security.ubuntu.com)
sudo apt-get update -qq

# Blacklist nouveau driver (open-source driver that conflicts with NVIDIA)
if ! grep -q "blacklist nouveau" /etc/modprobe.d/blacklist-nouveau.conf 2>/dev/null; then
    echo "Blacklisting nouveau driver..."
    sudo tee /etc/modprobe.d/blacklist-nouveau.conf > /dev/null << 'EOF'
blacklist nouveau
options nouveau modeset=0
EOF
    sudo update-initramfs -u 2>/dev/null || true
fi

# Install linux headers (required for NVIDIA DKMS kernel module compilation)
KERNEL_VERSION=$(uname -r)
echo "Installing linux-headers for kernel ${KERNEL_VERSION}..."
"${APT_INSTALL[@]}" "linux-headers-${KERNEL_VERSION}" 2>&1 | tail -3 || {
    echo "  Exact headers not available. Trying linux-headers-generic..."
    "${APT_INSTALL[@]}" linux-headers-generic 2>&1 | tail -3 || true
}

# Verify kernel build directory
BUILD_DIR="/lib/modules/${KERNEL_VERSION}/build"
if [ ! -d "$BUILD_DIR" ]; then
    echo "ERROR: Kernel headers directory not found: $BUILD_DIR"
    echo "DKMS cannot compile the NVIDIA kernel module without this."
    echo "Try: sudo apt install linux-headers-${KERNEL_VERSION}"
    exit 1
fi
echo "  ✓ Kernel headers: $BUILD_DIR"

# Install build tools
echo "Installing build-essential and dkms..."
"${APT_INSTALL[@]}" build-essential dkms 2>&1 | tail -3

# Check if the kernel needs a newer GCC than the default.
# Ubuntu 22.04 HWE kernels (6.x) are compiled with GCC 12, but the default is GCC 11.
# DKMS uses the system default GCC, which fails if kernel headers require newer flags
# like '-ftrivial-auto-var-init=zero' (GCC 12+).
KERNEL_GCC_VER=""
if [ -f "$BUILD_DIR/.config" ]; then
    # Extract GCC major version from kernel config, e.g. CONFIG_CC_VERSION_TEXT="gcc (Ubuntu 12.3.0-...) 12.3.0"
    KERNEL_GCC_VER=$(grep 'CONFIG_CC_VERSION_TEXT=' "$BUILD_DIR/.config" 2>/dev/null | sed 's/.*gcc.*[ (]\([0-9]\+\)\..*/\1/' || true)
fi
SYSTEM_GCC_VER=$(gcc -dumpversion 2>/dev/null | cut -d. -f1 || true)

if [ -n "$KERNEL_GCC_VER" ] && [ -n "$SYSTEM_GCC_VER" ] && [ "$KERNEL_GCC_VER" -gt "$SYSTEM_GCC_VER" ] 2>/dev/null; then
    echo "  Kernel was built with GCC ${KERNEL_GCC_VER}, but system has GCC ${SYSTEM_GCC_VER}."
    echo "  Installing gcc-${KERNEL_GCC_VER} for DKMS compatibility..."
    "${APT_INSTALL[@]}" "gcc-${KERNEL_GCC_VER}" 2>&1 | tail -3
    
    # Tell DKMS to use the matching GCC version
    sudo mkdir -p /etc/dkms/framework.conf.d
    echo "export CC=/usr/bin/gcc-${KERNEL_GCC_VER}" | sudo tee /etc/dkms/framework.conf.d/cc.conf > /dev/null
    echo "  ✓ DKMS configured to use gcc-${KERNEL_GCC_VER}"
else
    echo "  ✓ GCC: $(gcc --version 2>/dev/null | head -1 || echo 'not found')"
fi

# ============================================================
# Add NVIDIA CUDA Repository (Network Repo - lightweight)
# ============================================================
echo "\n========== NVIDIA CUDA Repository =========="

# Detect Ubuntu version and architecture
# Use /etc/os-release (always available) instead of lsb_release (may not be installed on 24.04 minimal)
if [ -f /etc/os-release ]; then
    UBUNTU_VERSION=$(. /etc/os-release && echo "${VERSION_ID}" | tr -d '.')
else
    UBUNTU_VERSION=$(lsb_release -rs 2>/dev/null | tr -d '.' || echo "2204")
fi

ARCH=$(dpkg --print-architecture 2>/dev/null || echo "amd64")
if [ "$ARCH" = "arm64" ]; then
    ARCH_PATH="sbsa"  # ARM server (e.g., Grace Hopper)
else
    ARCH_PATH="x86_64"
fi

if [[ "$UBUNTU_VERSION" == "2404" ]]; then
    REPO_URL="https://developer.download.nvidia.com/compute/cuda/repos/ubuntu2404/${ARCH_PATH}"
    KEYRING_URL="${REPO_URL}/cuda-keyring_1.1-1_all.deb"
elif [[ "$UBUNTU_VERSION" == "2204" ]]; then
    REPO_URL="https://developer.download.nvidia.com/compute/cuda/repos/ubuntu2204/${ARCH_PATH}"
    KEYRING_URL="${REPO_URL}/cuda-keyring_1.1-1_all.deb"
else
    echo "WARNING: Ubuntu $UBUNTU_VERSION may not be fully supported. Trying Ubuntu 22.04 repo..."
    REPO_URL="https://developer.download.nvidia.com/compute/cuda/repos/ubuntu2204/${ARCH_PATH}"
    KEYRING_URL="${REPO_URL}/cuda-keyring_1.1-1_all.deb"
fi

# Download and install keyring (small file, ~10KB)
echo "Adding CUDA repo (${UBUNTU_VERSION}, ${ARCH_PATH})..."
KEYRING_FILE=$(mktemp)
if ! curl -fsSL "$KEYRING_URL" -o "$KEYRING_FILE" 2>/dev/null; then
    # Fallback to wget if curl fails
    if ! wget -q "$KEYRING_URL" -O "$KEYRING_FILE" 2>/dev/null; then
        echo "ERROR: Failed to download CUDA keyring from $KEYRING_URL"
        exit 1
    fi
fi

sudo dpkg -i --force-confdef --force-confold "$KEYRING_FILE" 2>/dev/null || sudo dpkg -i "$KEYRING_FILE"
rm -f "$KEYRING_FILE"

# Update package lists
sudo apt-get update -qq

# ============================================================
# Install NVIDIA Driver (open kernel modules)
# ============================================================
# Use open kernel modules (nvidia-open) instead of proprietary (nvidia-dkms).
# Open modules are REQUIRED for Blackwell+ GPUs and work on all Turing+ (2018+) GPUs,
# which covers all modern cloud GPU instances (T4, A10G, A100, L4, H100, H200, B200, etc.).
echo "\n========== NVIDIA Driver =========="

echo "Installing cuda-drivers-open (this may take several minutes)..."
INSTALL_LOG=$(mktemp)
set +e
"${APT_INSTALL[@]}" cuda-drivers-open 2>&1 | tee "$INSTALL_LOG"
INSTALL_EXIT_CODE=${PIPESTATUS[0]}
set -e  # Re-enable exit-on-error

# Check installation result
if [ $INSTALL_EXIT_CODE -ne 0 ]; then
    echo ""
    echo "ERROR: cuda-drivers-open installation failed (exit code: $INSTALL_EXIT_CODE)"
    
    # Show DKMS build log if it was a DKMS failure
    if grep -q "bad exit status\|Bad return status" "$INSTALL_LOG" 2>/dev/null; then
        echo ""
        echo "DKMS kernel module build failed."
        DKMS_LOG=$(find /var/lib/dkms/nvidia/*/build/make.log -type f 2>/dev/null | head -1)
        if [ -n "$DKMS_LOG" ]; then
            echo "--- Last 20 lines of $DKMS_LOG ---"
            tail -20 "$DKMS_LOG" 2>/dev/null || true
        fi
    fi
    
    echo ""
    echo "Attempting fallback: install nvidia-driver-550-open..."
    
    # First: purge ALL leftover nvidia packages from the failed attempt.
    # Version-less packages (e.g. libnvidia-cfg1) conflict with version-suffixed ones (libnvidia-cfg1-550).
    echo "  Cleaning up leftover packages from failed attempt..."
    LEFTOVER_PKGS=$(dpkg -l 2>/dev/null | grep -E "nvidia|cuda|libnvidia" | grep -v "^ii " | grep -v "^un " | awk '{print $2}' || true)
    if [ -n "$LEFTOVER_PKGS" ]; then
        sudo dpkg --force-all --purge $LEFTOVER_PKGS 2>&1 | tail -5 || true
        sudo rm -rf /var/lib/dkms/nvidia 2>/dev/null || true
        sudo dpkg --configure -a 2>/dev/null || true
    fi
    # Also remove any version-less nvidia libs that were auto-installed
    VERSIONLESS_PKGS=$(dpkg -l 2>/dev/null | grep "^ii" | awk '{print $2}' | grep -E "^libnvidia-|^nvidia-" | grep -v -- "-[0-9]" || true)
    if [ -n "$VERSIONLESS_PKGS" ]; then
        echo "  Removing version-less nvidia packages: $(echo $VERSIONLESS_PKGS | tr '\n' ' ')"
        sudo dpkg --force-all --purge $VERSIONLESS_PKGS 2>&1 | tail -3 || true
    fi
    
    set +e
    sudo DEBIAN_FRONTEND=noninteractive apt-get install -y \
        "${APT_OPTS[@]}" -o Dpkg::Options::="--force-overwrite" \
        nvidia-driver-550-open 2>&1 | tee "$INSTALL_LOG"
    FALLBACK_EXIT_CODE=${PIPESTATUS[0]}
    set -e
    
    if [ $FALLBACK_EXIT_CODE -ne 0 ]; then
        echo ""
        echo "ERROR: Fallback nvidia-driver-550-open also failed."
        rm -f "$INSTALL_LOG"
        exit 1
    fi
fi
rm -f "$INSTALL_LOG"

# Verify: check DKMS status (the critical gate for nvidia-smi after reboot)
echo ""
echo "Verifying installation..."
DKMS_STATUS=$(dkms status nvidia 2>/dev/null || true)
if echo "$DKMS_STATUS" | grep -q "installed"; then
    echo "✓ NVIDIA DKMS module: $DKMS_STATUS"
else
    echo "DKMS status: ${DKMS_STATUS:-not found}"
    # Check if packages are properly configured
    BROKEN_DRV=$(dpkg -l | grep -E "nvidia-dkms|nvidia-open|nvidia-driver|cuda-drivers" | grep -v "^ii " || true)
    if [ -n "$BROKEN_DRV" ]; then
        echo ""
        echo "WARNING: Driver packages not properly configured:"
        echo "$BROKEN_DRV"
        echo "nvidia-smi will likely NOT work after reboot."
        echo "Check: /var/lib/dkms/nvidia/*/build/make.log"
    fi
fi

if modinfo nvidia &>/dev/null; then
    echo "✓ Kernel module: $(modinfo nvidia 2>/dev/null | grep '^version:' | awk '{print $2}')"
fi

echo ""
echo "NVIDIA Driver installation completed."

# ============================================================
# Install CUDA Toolkit (optional)
# ============================================================
if [ "$INSTALL_TOOLKIT" = true ]; then
    echo "\n========== CUDA Toolkit =========="
    
    if [ -n "$CUDA_VERSION" ]; then
        TOOLKIT_PACKAGE="cuda-toolkit-${CUDA_VERSION}"
    else
        TOOLKIT_PACKAGE="cuda-toolkit"
    fi
    
    echo "Installing ${TOOLKIT_PACKAGE}..."
    set +e
    "${APT_INSTALL[@]}" "$TOOLKIT_PACKAGE" 2>&1 | tail -20
    TOOLKIT_EXIT=${PIPESTATUS[0]}
    set -e
    if [ $TOOLKIT_EXIT -ne 0 ]; then
        echo "WARNING: CUDA Toolkit installation failed (exit code: $TOOLKIT_EXIT)"
        echo "The NVIDIA driver is still installed. You can install the toolkit later."
    fi
    
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
# Fabric Manager is REQUIRED for multi-GPU HGX systems (A100/H100/H200 SXM)
# that use NVSwitch for GPU-to-GPU communication. Without it, only GPU 0 is accessible.
echo ""
echo "Checking for NVSwitch/multi-GPU topology..."

# Detect NVSwitch via multiple methods:
#  1. PCI devices: NVSwitch shows up as a separate PCI device
#  2. Device files: /dev/nvidia-nvswitch* exist after driver load
#  3. Multi-GPU count: 4+ GPUs typically means HGX with NVSwitch
NVSWITCH_PCI=$(sudo lspci 2>/dev/null | grep -i -E "nvswitch|nvlink" || true)
NVSWITCH_DEV=$(ls /dev/nvidia-nvswitch* 2>/dev/null || true)
GPU_COUNT=$(sudo lspci 2>/dev/null | grep -c -i "nvidia.*3d controller\|nvidia.*vga") || GPU_COUNT=0

NEED_FABRIC_MANAGER=false
if [ -n "$NVSWITCH_PCI" ] || [ -n "$NVSWITCH_DEV" ]; then
    NEED_FABRIC_MANAGER=true
    echo "  NVSwitch detected via PCI/device."
elif [ "$GPU_COUNT" -ge 4 ] 2>/dev/null; then
    # HGX systems with 4+ GPUs almost always have NVSwitch
    NEED_FABRIC_MANAGER=true
    echo "  ${GPU_COUNT} GPUs detected (likely HGX system with NVSwitch)."
fi

if [ "$NEED_FABRIC_MANAGER" = true ]; then
    # Fabric Manager version MUST match the installed driver major version.
    # Mismatch causes: "Version mismatch between FM (X) and driver (Y)" → only GPU 0 accessible.
    DRIVER_MAJOR=$(dpkg -l 2>/dev/null | grep "^ii" | awk '{print $2}' | grep -oP "^nvidia-driver-\K[0-9]+" | sort -rn | head -1 || true)
    if [ -n "$DRIVER_MAJOR" ]; then
        FM_PKG="nvidia-fabricmanager-${DRIVER_MAJOR}"
        echo "  Installing ${FM_PKG} (matching driver version ${DRIVER_MAJOR})..."
    else
        FM_PKG="nvidia-fabricmanager"
        echo "  Installing ${FM_PKG}..."
    fi
    
    set +e
    "${APT_INSTALL[@]}" "$FM_PKG" 2>&1 | tail -10
    FM_EXIT=${PIPESTATUS[0]}
    set -e
    
    if [ $FM_EXIT -eq 0 ]; then
        # Enable for auto-start on boot and start now if driver is loaded
        sudo systemctl enable nvidia-fabricmanager 2>/dev/null || true
        sudo systemctl start nvidia-fabricmanager 2>/dev/null || true
        echo "  ✓ Fabric Manager installed and enabled."
        echo "    (Will fully activate after reboot when all GPU modules are loaded)"
    else
        echo "  WARNING: Failed to install ${FM_PKG} (exit code: $FM_EXIT)."
        echo "  Multi-GPU communication may not work. Install manually after reboot:"
        echo "    sudo apt install ${FM_PKG} && sudo systemctl enable --now nvidia-fabricmanager"
    fi
else
    if [ "$GPU_COUNT" -gt 1 ] 2>/dev/null; then
        echo "  ${GPU_COUNT} GPUs detected (PCIe topology, no NVSwitch). Fabric Manager not needed."
    else
        echo "  Single GPU detected. Fabric Manager not needed."
    fi
fi

# Enable nvidia-persistenced for multi-GPU systems to avoid cold-start latency.
# Without this, the first GPU operation after idle may take ~2s to re-initialize.
if [ "$GPU_COUNT" -gt 1 ] 2>/dev/null; then
    if systemctl list-unit-files nvidia-persistenced.service &>/dev/null; then
        sudo systemctl enable nvidia-persistenced 2>/dev/null || true
        sudo systemctl start nvidia-persistenced 2>/dev/null || true
        echo "  ✓ nvidia-persistenced enabled (reduces GPU initialization latency)."
    fi
fi

# ============================================================
# Install NVIDIA Container Toolkit
# ============================================================
if [ "$INSTALL_CONTAINER_TOOLKIT" = true ]; then
    echo "\n========== NVIDIA Container Toolkit =========="
    
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
    set +e
    "${APT_INSTALL[@]}" nvidia-container-toolkit 2>&1 | tail -10
    CTK_EXIT=${PIPESTATUS[0]}
    set -e
    
    if [ $CTK_EXIT -ne 0 ]; then
        echo "WARNING: Container Toolkit installation failed (exit code: $CTK_EXIT)"
    else
        echo "NVIDIA Container Toolkit installed."
    fi
    
    # Configure container runtimes
    echo ""
    echo "Configuring container runtimes..."
    
    # Configure containerd (for Kubernetes)
    if command -v containerd &>/dev/null; then
        echo "  Configuring containerd for NVIDIA runtime..."
        # --set-as-default: makes nvidia the default runtime (required for GPU Operator validator pods)
        sudo nvidia-ctk runtime configure --runtime=containerd --set-as-default 2>/dev/null || true
        
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
# Summary
# ============================================================
echo ""
echo "========== Installation Complete =========="
COMPONENTS="Driver"
[ "$INSTALL_CONTAINER_TOOLKIT" = true ] && COMPONENTS="$COMPONENTS, Container-Toolkit"
[ "$INSTALL_TOOLKIT" = true ] && COMPONENTS="$COMPONENTS, CUDA-Toolkit"
[ "$NEED_FABRIC_MANAGER" = true ] && COMPONENTS="$COMPONENTS, Fabric-Manager"
echo "  Installed: $COMPONENTS"
echo "  Verify after reboot: nvidia-smi"

# ============================================================
# Reboot handling
# ============================================================
if [ "$AUTO_REBOOT" = true ]; then
    echo "Rebooting in 5 seconds... (SSH will disconnect, verify with nvidia-smi after ~60s)"
    nohup bash -c 'sleep 5 && sudo reboot' > /dev/null 2>&1 &
    exit 0
else
    echo "REBOOT REQUIRED: run 'sudo reboot' to complete installation."
fi
