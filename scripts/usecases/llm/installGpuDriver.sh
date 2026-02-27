#!/bin/bash

# GPU Driver Installation Script (NVIDIA / AMD ROCm)
# Supports Ubuntu 22.04/24.04 on bare-metal, VM, and Kubernetes GPU nodes
#
# Features (NVIDIA):
#   - NVIDIA Driver installation (required for all GPU workloads)
#   - CUDA Toolkit installation (optional, for direct CUDA development)
#   - NVIDIA Container Toolkit (for Docker/Podman/Kubernetes GPU containers)
#   - containerd/Docker integration (auto-configured if present)
#   - NVSwitch/Fabric Manager support (for multi-GPU systems like H100/A100 HGX)
#
# Features (AMD):
#   - ROCm driver and kernel module (amdgpu-dkms) installation
#   - User group assignment (render, video) for GPU access
#   - Library path and environment variable configuration
#
# Usage:
#   ./installGpuDriver.sh                    # Auto-detect GPU type
#   ./installGpuDriver.sh --gpu nvidia       # Force NVIDIA path
#   ./installGpuDriver.sh --gpu amd          # Force AMD path
#   ./installGpuDriver.sh --no-reboot        # Skip automatic reboot
#
# NVIDIA-specific options:
#   --with-toolkit        Install CUDA Toolkit for CUDA C++ development
#   --driver-only         Driver only, no Container Toolkit
#   --cuda-version VER    Specific CUDA version (e.g., 12-6)
#
# AMD-specific options:
#   --rocm-version VER    ROCm version to install (default: 7.0.1)
#   --rocm-build BUILD    Full build string (default: 7.0.1.70001-1)
#
# Remote execution (CB-MapUI / CB-Tumblebug API):
#   This script is designed for non-interactive SSH execution.
#   All prompts are suppressed using DEBIAN_FRONTEND, needrestart config, etc.
#   Use --no-reboot for remote execution to prevent SSH connection drop.
#
# References:
#   - CUDA: https://developer.nvidia.com/cuda-downloads
#   - Container Toolkit: https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/
#   - ROCm: https://rocm.docs.amd.com/

set -e
set -o pipefail

# ============================================================
# Non-interactive mode for SSH remote execution
# ============================================================
export DEBIAN_FRONTEND=noninteractive
export NEEDRESTART_MODE=a
export NEEDRESTART_SUSPEND=1

if [ -d /etc/needrestart/conf.d ]; then
    echo "\$nrconf{restart} = 'a';" | sudo tee /etc/needrestart/conf.d/99-autorestart.conf > /dev/null 2>&1 || true
fi

# ============================================================
# Fix dpkg/apt state (cleanup from previous failed installations)
# ============================================================
echo "Checking for apt/dpkg locks..."
for i in $(seq 1 12); do
    if ! sudo fuser /var/lib/dpkg/lock-frontend >/dev/null 2>&1 && \
       ! sudo fuser /var/lib/apt/lists/lock >/dev/null 2>&1; then
        break
    fi
    echo "  Waiting for apt lock... (${i}/12)"
    sleep 5
done

sudo rm -f /var/lib/dpkg/lock-frontend /var/lib/dpkg/lock \
    /var/cache/apt/archives/lock /var/lib/apt/lists/lock 2>/dev/null || true

# ============================================================
# Configuration
# ============================================================
AUTO_REBOOT=true
GPU_TYPE=""        # empty = auto-detect

# NVIDIA config
INSTALL_TOOLKIT=false
INSTALL_CONTAINER_TOOLKIT=true
CUDA_VERSION=""    # empty = latest

# AMD config
ROCM_VERSION="7.0.1"
ROCM_BUILD="7.0.1.70001-1"

# Common apt-get options
APT_OPTS=(-o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold")
APT_INSTALL=(sudo DEBIAN_FRONTEND=noninteractive apt-get install -y "${APT_OPTS[@]}")
APT_OPTS_STR="-y -q -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-confold"

# ============================================================
# Parse arguments
# ============================================================
while [[ $# -gt 0 ]]; do
    case $1 in
        --gpu)
            GPU_TYPE="$2"
            shift 2
            ;;
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
        --rocm-version)
            ROCM_VERSION="$2"
            shift 2
            ;;
        --rocm-build)
            ROCM_BUILD="$2"
            shift 2
            ;;
        -h|--help)
            echo "GPU Driver Installation Script (NVIDIA / AMD ROCm)"
            echo ""
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Common options:"
            echo "  --gpu TYPE         GPU type: nvidia or amd (default: auto-detect)"
            echo "  --no-reboot        Skip automatic reboot after installation"
            echo "  -h, --help         Show this help message"
            echo ""
            echo "NVIDIA-specific options:"
            echo "  --with-toolkit     Install CUDA Toolkit for development (nvcc, libraries)"
            echo "  --driver-only      Install driver only, skip Container Toolkit"
            echo "  --cuda-version VER Specify CUDA version (e.g., 12-6). Default: latest"
            echo ""
            echo "AMD-specific options:"
            echo "  --rocm-version VER ROCm version to install (default: 7.0.1)"
            echo "  --rocm-build BUILD Full build string (default: 7.0.1.70001-1)"
            echo ""
            echo "Examples:"
            echo "  $0                          # Auto-detect GPU, recommended defaults"
            echo "  $0 --gpu nvidia             # Force NVIDIA path"
            echo "  $0 --gpu amd                # Force AMD path"
            echo "  $0 --gpu nvidia --with-toolkit  # NVIDIA + CUDA Toolkit"
            echo "  $0 --gpu amd --rocm-version 6.2.4 --rocm-build 6.2.4.60204-1"
            echo "  $0 --no-reboot              # For remote/automated execution"
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
# GPU Auto-detection
# ============================================================
if [ -z "$GPU_TYPE" ]; then
    echo "Auto-detecting GPU type..."
    if sudo lspci 2>/dev/null | grep -qi nvidia; then
        GPU_TYPE="nvidia"
    elif sudo lspci 2>/dev/null | grep -qi -E "advanced micro devices|AMD/ATI|amd.*radeon|radeon"; then
        GPU_TYPE="amd"
    else
        echo "ERROR: No supported GPU detected via lspci."
        echo ""
        echo "PCI devices found:"
        sudo lspci | grep -i -E "vga|3d|display" || true
        echo ""
        echo "Specify GPU type manually with: --gpu nvidia  or  --gpu amd"
        exit 1
    fi
    echo "  Detected: $GPU_TYPE"
fi

if [ "$GPU_TYPE" != "nvidia" ] && [ "$GPU_TYPE" != "amd" ]; then
    echo "ERROR: Invalid --gpu value '$GPU_TYPE'. Must be 'nvidia' or 'amd'."
    exit 1
fi

echo "========== GPU Driver Installation (${GPU_TYPE^^}) =========="

# ============================================================
# ============================================================
#  NVIDIA PATH
# ============================================================
# ============================================================
if [ "$GPU_TYPE" = "nvidia" ]; then

    # ----------------------------------------------------------
    # Cleanup broken NVIDIA/CUDA packages from previous failures
    # ----------------------------------------------------------
    BROKEN_PKGS=$(dpkg -l 2>/dev/null | grep -E "nvidia|cuda|libnvidia" | grep -v "^ii " | grep -v "^un " | awk '{print $2}' || true)
    if [ -n "$BROKEN_PKGS" ]; then
        echo ""
        echo "Found broken NVIDIA/CUDA packages from previous failed install. Purging..."
        echo "  Packages: $(echo $BROKEN_PKGS | tr '\n' ' ')"
        sudo dpkg --force-all --purge $BROKEN_PKGS 2>&1 | tail -5 || true
        sudo rm -rf /var/lib/dkms/nvidia 2>/dev/null || true
        sudo rm -f /var/crash/nvidia-*.crash 2>/dev/null || true
        sudo dpkg --configure -a 2>/dev/null || true
        echo ""
    fi

    # ----------------------------------------------------------
    # Pre-flight: verify NVIDIA GPU present
    # ----------------------------------------------------------
    echo "Checking for NVIDIA GPU..."
    GPU_INFO=$(sudo lspci | grep -i nvidia || true)
    if [ -z "$GPU_INFO" ]; then
        echo "ERROR: No NVIDIA GPU detected!"
        sudo lspci | grep -i -E "vga|3d|display" || true
        exit 1
    fi
    echo "  $GPU_INFO"

    AVAILABLE_GB=$(df -BG / | awk 'NR==2 {gsub("G",""); print $4}')
    REQUIRED_GB=5
    [ "$INSTALL_TOOLKIT" = true ] && REQUIRED_GB=10
    if [ "$AVAILABLE_GB" -lt "$REQUIRED_GB" ]; then
        echo "WARNING: Low disk space. Available: ${AVAILABLE_GB}GB, Recommended: ${REQUIRED_GB}GB"
    fi

    # ----------------------------------------------------------
    # Pre-cleanup: Remove any existing NVIDIA driver packages to avoid version conflicts.
    # This is critical for re-runs where a previously installed (incompatible) driver
    # would block the installation of a different version branch (e.g., 590.x blocks 550.x).
    # Must run BEFORE DKMS prerequisites because broken packages poison ALL apt-get installs.
    # ----------------------------------------------------------
    EXISTING_NV=$(dpkg -l 2>/dev/null | grep "^ii" | awk '{print $2}' | grep -E "^(cuda-drivers|nvidia-driver|nvidia-dkms|nvidia-kernel|nvidia-firmware|libnvidia-|nvidia-modprobe|nvidia-settings|nvidia-compute|xserver-xorg-video-nvidia)" || true)
    if [ -n "$EXISTING_NV" ]; then
        echo ""
        echo "========== Pre-cleanup: Removing Existing NVIDIA Packages =========="
        echo "  Packages to remove: $(echo $EXISTING_NV | wc -w) packages"
        # Stop nvidia-persistenced to release module references
        sudo systemctl stop nvidia-persistenced 2>/dev/null || true
        # Purge all existing NVIDIA packages
        sudo dpkg --force-all --purge $EXISTING_NV 2>&1 | tail -5 || true
        sudo rm -rf /var/lib/dkms/nvidia 2>/dev/null || true
        sudo dpkg --configure -a 2>/dev/null || true
        sudo apt-get -f install -y 2>&1 | tail -3 || true
        echo "  ✓ Existing NVIDIA packages removed."
    fi

    # ----------------------------------------------------------
    # DKMS prerequisites
    # ----------------------------------------------------------
    echo ""
    echo "========== DKMS Prerequisites =========="

    sudo apt-get update -qq

    # Blacklist nouveau
    if ! grep -q "blacklist nouveau" /etc/modprobe.d/blacklist-nouveau.conf 2>/dev/null; then
        echo "Blacklisting nouveau driver..."
        sudo tee /etc/modprobe.d/blacklist-nouveau.conf > /dev/null << 'EOF'
blacklist nouveau
options nouveau modeset=0
EOF
        sudo update-initramfs -u 2>/dev/null || true
    fi

    KERNEL_VERSION=$(uname -r)
    echo "Installing linux-headers for kernel ${KERNEL_VERSION}..."
    "${APT_INSTALL[@]}" "linux-headers-${KERNEL_VERSION}" 2>&1 | tail -3 || {
        echo "  Exact headers not available. Trying linux-headers-generic..."
        "${APT_INSTALL[@]}" linux-headers-generic 2>&1 | tail -3 || true
    }

    BUILD_DIR="/lib/modules/${KERNEL_VERSION}/build"
    if [ ! -d "$BUILD_DIR" ]; then
        echo "ERROR: Kernel headers directory not found: $BUILD_DIR"
        echo "Try: sudo apt install linux-headers-${KERNEL_VERSION}"
        exit 1
    fi
    echo "  ✓ Kernel headers: $BUILD_DIR"

    echo "Installing build-essential and dkms..."
    "${APT_INSTALL[@]}" build-essential dkms 2>&1 | tail -3

    # GCC version check for HWE kernels
    KERNEL_GCC_VER=""
    if [ -f "$BUILD_DIR/.config" ]; then
        KERNEL_GCC_VER=$(grep 'CONFIG_CC_VERSION_TEXT=' "$BUILD_DIR/.config" 2>/dev/null | sed 's/.*gcc.*[ (]\([0-9]\+\)\..*/\1/' || true)
    fi
    SYSTEM_GCC_VER=$(gcc -dumpversion 2>/dev/null | cut -d. -f1 || true)

    if [ -n "$KERNEL_GCC_VER" ] && [ -n "$SYSTEM_GCC_VER" ] && [ "$KERNEL_GCC_VER" -gt "$SYSTEM_GCC_VER" ] 2>/dev/null; then
        echo "  Kernel was built with GCC ${KERNEL_GCC_VER}, but system has GCC ${SYSTEM_GCC_VER}."
        echo "  Installing gcc-${KERNEL_GCC_VER} for DKMS compatibility..."
        "${APT_INSTALL[@]}" "gcc-${KERNEL_GCC_VER}" 2>&1 | tail -3
        sudo mkdir -p /etc/dkms/framework.conf.d
        echo "export CC=/usr/bin/gcc-${KERNEL_GCC_VER}" | sudo tee /etc/dkms/framework.conf.d/cc.conf > /dev/null
        echo "  ✓ DKMS configured to use gcc-${KERNEL_GCC_VER}"
    else
        echo "  ✓ GCC: $(gcc --version 2>/dev/null | head -1 || echo 'not found')"
    fi

    # ----------------------------------------------------------
    # NVIDIA CUDA Repository
    # ----------------------------------------------------------
    echo ""
    echo "========== NVIDIA CUDA Repository =========="

    if [ -f /etc/os-release ]; then
        UBUNTU_VERSION=$(. /etc/os-release && echo "${VERSION_ID}" | tr -d '.')
    else
        UBUNTU_VERSION=$(lsb_release -rs 2>/dev/null | tr -d '.' || echo "2204")
    fi

    ARCH=$(dpkg --print-architecture 2>/dev/null || echo "amd64")
    [ "$ARCH" = "arm64" ] && ARCH_PATH="sbsa" || ARCH_PATH="x86_64"

    if [[ "$UBUNTU_VERSION" == "2404" ]]; then
        REPO_URL="https://developer.download.nvidia.com/compute/cuda/repos/ubuntu2404/${ARCH_PATH}"
    elif [[ "$UBUNTU_VERSION" == "2204" ]]; then
        REPO_URL="https://developer.download.nvidia.com/compute/cuda/repos/ubuntu2204/${ARCH_PATH}"
    else
        echo "WARNING: Ubuntu $UBUNTU_VERSION may not be fully supported. Trying Ubuntu 22.04 repo..."
        REPO_URL="https://developer.download.nvidia.com/compute/cuda/repos/ubuntu2204/${ARCH_PATH}"
    fi
    KEYRING_URL="${REPO_URL}/cuda-keyring_1.1-1_all.deb"

    echo "Adding CUDA repo (${UBUNTU_VERSION}, ${ARCH_PATH})..."
    KEYRING_FILE=$(mktemp)
    if ! curl -fsSL "$KEYRING_URL" -o "$KEYRING_FILE" 2>/dev/null; then
        wget -q "$KEYRING_URL" -O "$KEYRING_FILE" 2>/dev/null || { echo "ERROR: Failed to download CUDA keyring."; exit 1; }
    fi
    sudo dpkg -i --force-confdef --force-confold "$KEYRING_FILE" 2>/dev/null || sudo dpkg -i "$KEYRING_FILE"
    rm -f "$KEYRING_FILE"
    sudo apt-get update -qq

    # ----------------------------------------------------------
    # Detect vGPU vs bare-metal/passthrough
    # ----------------------------------------------------------
    # NVIDIA open kernel modules do NOT support vGPU configurations.
    # Cloud instances (AWS g6, Azure NCas, etc.) often expose GPUs as vGPU,
    # which requires proprietary (closed-source) kernel modules.
    # Detect vGPU by checking PCI subsystem or kernel module signature.
    IS_VGPU=false
    # Method 1: Check for vGPU PCI subsystem class (0x0302 = 3D controller, common for vGPU)
    # vGPU guests typically show "3D controller" instead of "VGA compatible controller"
    if sudo lspci 2>/dev/null | grep -i nvidia | grep -qi "3d controller"; then
        IS_VGPU=true
    fi
    # Method 2: Check for NVIDIA GRID/vGPU device files or modules
    if [ -d /proc/driver/nvidia/gpus ] && grep -q -ri "vGPU\|GRID" /proc/driver/nvidia/ 2>/dev/null; then
        IS_VGPU=true
    fi

    if [ "$IS_VGPU" = true ]; then
        echo "  ⚠ vGPU detected (3D controller). Open kernel modules are NOT supported."
        echo "  → Using proprietary (closed-source) driver."
    fi

    # ----------------------------------------------------------
    # Install linux-modules-extra (required by some CSP kernels)
    # ----------------------------------------------------------
    # AWS/Azure/GCP custom kernels may have GPU-related kernel modules
    # (vfio-pci, i2c, etc.) in linux-modules-extra. Without it, GPU
    # initialization can fail on some cloud providers.
    echo "Installing linux-modules-extra for kernel ${KERNEL_VERSION}..."
    "${APT_INSTALL[@]}" "linux-modules-extra-${KERNEL_VERSION}" 2>&1 | tail -3 || true

    # ----------------------------------------------------------
    # NVIDIA Driver
    # ----------------------------------------------------------
    echo ""
    echo "========== NVIDIA Driver =========="

    INSTALL_LOG=$(mktemp)
    DRIVER_INSTALLED=false

    # Helper: purge broken/leftover nvidia packages before retrying
    cleanup_nvidia_packages() {
        LEFTOVER_PKGS=$(dpkg -l 2>/dev/null | grep -E "nvidia|cuda|libnvidia" | grep -v "^ii " | grep -v "^un " | awk '{print $2}' || true)
        if [ -n "$LEFTOVER_PKGS" ]; then
            echo "  Cleaning up leftover packages..."
            sudo dpkg --force-all --purge $LEFTOVER_PKGS 2>&1 | tail -5 || true
            sudo rm -rf /var/lib/dkms/nvidia 2>/dev/null || true
            sudo dpkg --configure -a 2>/dev/null || true
        fi
        VERSIONLESS_PKGS=$(dpkg -l 2>/dev/null | grep "^ii" | awk '{print $2}' | grep -E "^libnvidia-|^nvidia-" | grep -v -- "-[0-9]" || true)
        if [ -n "$VERSIONLESS_PKGS" ]; then
            sudo dpkg --force-all --purge $VERSIONLESS_PKGS 2>&1 | tail -3 || true
        fi
    }

    # Helper: show DKMS build log on failure
    show_dkms_log() {
        if grep -q "bad exit status\|Bad return status" "$INSTALL_LOG" 2>/dev/null; then
            echo "DKMS kernel module build failed."
            DKMS_LOG=$(find /var/lib/dkms/nvidia/*/build/make.log -type f 2>/dev/null | head -1)
            if [ -n "$DKMS_LOG" ]; then
                echo "--- Last 20 lines of $DKMS_LOG ---"
                tail -20 "$DKMS_LOG" 2>/dev/null || true
            fi
        fi
    }

    # Build driver candidate list based on vGPU detection.
    # - vGPU: ONLY proprietary drivers (open modules fail with "not supported by open nvidia.ko")
    # - Passthrough/bare-metal: try open first (required for Blackwell+), then proprietary fallback
    #
    # IMPORTANT: Use version-pinned packages (cuda-drivers-550) BEFORE unversioned (cuda-drivers).
    # The unversioned 'cuda-drivers' metapackage always pulls the LATEST driver from the CUDA repo.
    # Newer driver branches (e.g., 590.x) may drop support for certain GPU PCI IDs
    # (e.g., L4 GPU 10de:27b8 is "not supported by NVIDIA 590.48.01").
    # The 550.x branch is the current production/stable branch with broad GPU support.
    if [ "$IS_VGPU" = true ]; then
        DRIVER_CANDIDATES=("cuda-drivers-550" "nvidia-driver-550" "cuda-drivers")
    else
        DRIVER_CANDIDATES=("cuda-drivers-550-open" "cuda-drivers-open" "cuda-drivers-550" "nvidia-driver-550-open" "nvidia-driver-550" "cuda-drivers")
    fi

    for CANDIDATE in "${DRIVER_CANDIDATES[@]}"; do
        echo ""
        echo "Attempting: ${CANDIDATE}..."
        set +e
        if [[ "$CANDIDATE" == nvidia-driver-* ]]; then
            # Ubuntu repo packages may need --force-overwrite for file conflicts
            sudo DEBIAN_FRONTEND=noninteractive apt-get install -y \
                "${APT_OPTS[@]}" -o Dpkg::Options::="--force-overwrite" \
                "$CANDIDATE" 2>&1 | tee "$INSTALL_LOG"
        else
            "${APT_INSTALL[@]}" "$CANDIDATE" 2>&1 | tee "$INSTALL_LOG"
        fi
        CANDIDATE_EXIT=${PIPESTATUS[0]}
        set -e

        if [ $CANDIDATE_EXIT -eq 0 ]; then
            echo "✓ Successfully installed: ${CANDIDATE}"
            DRIVER_INSTALLED=true
            break
        else
            echo "✗ ${CANDIDATE} failed (exit code: $CANDIDATE_EXIT)"
            show_dkms_log
            cleanup_nvidia_packages
        fi
    done

    rm -f "$INSTALL_LOG"

    if [ "$DRIVER_INSTALLED" != true ]; then
        echo ""
        echo "ERROR: All driver installation attempts failed."
        echo "Tried: ${DRIVER_CANDIDATES[*]}"
        exit 1
    fi

    # Ensure nvidia-modprobe is installed (creates /dev/nvidia* device nodes).
    # cuda-drivers-open includes this as a dependency, but Ubuntu's
    # nvidia-driver-550-open does NOT. Without it, nvidia-smi fails with
    # "couldn't communicate with the NVIDIA driver" even though the kernel
    # module is loaded, because /dev/nvidia0 and /dev/nvidia-uvm are missing.
    if ! dpkg -l nvidia-modprobe 2>/dev/null | grep -q "^ii"; then
        echo "Installing nvidia-modprobe (required for /dev/nvidia* device nodes)..."
        "${APT_INSTALL[@]}" nvidia-modprobe 2>&1 | tail -3 || true
    fi

    echo ""
    echo "Verifying installation..."
    DKMS_STATUS=$(dkms status nvidia 2>/dev/null || true)
    if echo "$DKMS_STATUS" | grep -q "installed"; then
        echo "✓ NVIDIA DKMS module: $DKMS_STATUS"
    else
        echo "DKMS status: ${DKMS_STATUS:-not found}"
        BROKEN_DRV=$(dpkg -l | grep -E "nvidia-dkms|nvidia-open|nvidia-driver|cuda-drivers" | grep -v "^ii " || true)
        if [ -n "$BROKEN_DRV" ]; then
            echo "WARNING: Driver packages not properly configured. Check /var/lib/dkms/nvidia/*/build/make.log"
        fi
    fi
    modinfo nvidia &>/dev/null && echo "✓ Kernel module: $(modinfo nvidia 2>/dev/null | grep '^version:' | awk '{print $2}')" || true

    echo ""
    echo "NVIDIA Driver installation completed."

    # ----------------------------------------------------------
    # CUDA Toolkit (optional)
    # ----------------------------------------------------------
    if [ "$INSTALL_TOOLKIT" = true ]; then
        echo ""
        echo "========== CUDA Toolkit =========="

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
            echo "WARNING: CUDA Toolkit installation failed. The NVIDIA driver is still installed."
        fi

        CUDA_PATH=$(ls -d /usr/local/cuda-* 2>/dev/null | sort -V | tail -1)
        if [ -n "$CUDA_PATH" ] && ! grep -q "CUDA_HOME" ~/.bashrc 2>/dev/null; then
            cat >> ~/.bashrc << CUDA_ENV

# CUDA Environment
export CUDA_HOME=${CUDA_PATH}
export PATH=\${CUDA_HOME}/bin\${PATH:+:\${PATH}}
export LD_LIBRARY_PATH=\${CUDA_HOME}/lib64\${LD_LIBRARY_PATH:+:\${LD_LIBRARY_PATH}}
CUDA_ENV
            echo "CUDA environment added to ~/.bashrc"
        fi
        echo "CUDA Toolkit installation completed."
    fi

    # ----------------------------------------------------------
    # NVSwitch / Fabric Manager
    # ----------------------------------------------------------
    echo ""
    echo "Checking for NVSwitch/multi-GPU topology..."

    NVSWITCH_PCI=$(sudo lspci 2>/dev/null | grep -i -E "nvswitch|nvlink" || true)
    [ -z "$NVSWITCH_PCI" ] && NVSWITCH_PCI=$(sudo lspci -n 2>/dev/null | grep -i "10de:2[23]" || true)
    NVSWITCH_DEV=$(ls /dev/nvidia-nvswitch* 2>/dev/null || true)
    GPU_COUNT=$(sudo lspci 2>/dev/null | grep -i nvidia | grep -i -c "3d controller\|vga compatible") || GPU_COUNT=0
    [ "$GPU_COUNT" -eq 0 ] && command -v nvidia-smi &>/dev/null && GPU_COUNT=$(nvidia-smi -L 2>/dev/null | grep -c "^GPU") || true
    NVSWITCH_TOPO=""
    [ -z "$NVSWITCH_PCI" ] && [ -z "$NVSWITCH_DEV" ] && command -v nvidia-smi &>/dev/null && \
        NVSWITCH_TOPO=$(nvidia-smi topo -m 2>/dev/null | grep -i "nvswitch\|NV[0-9]" || true)

    NEED_FABRIC_MANAGER=false
    if [ -n "$NVSWITCH_PCI" ] || [ -n "$NVSWITCH_DEV" ]; then
        NEED_FABRIC_MANAGER=true
        echo "  NVSwitch detected via PCI/device."
    elif [ -n "$NVSWITCH_TOPO" ]; then
        NEED_FABRIC_MANAGER=true
        echo "  NVSwitch detected via nvidia-smi topology."
    elif [ "$GPU_COUNT" -ge 4 ] 2>/dev/null; then
        NEED_FABRIC_MANAGER=true
        echo "  ${GPU_COUNT} GPUs detected (likely HGX system with NVSwitch)."
    fi

    if [ "$NEED_FABRIC_MANAGER" = true ]; then
        DRIVER_MAJOR=$(dpkg -l 2>/dev/null | grep "^ii" | awk '{print $2}' | grep -oP "^nvidia-driver-\K[0-9]+" | sort -rn | head -1 || true)
        [ -z "$DRIVER_MAJOR" ] && command -v nvidia-smi &>/dev/null && \
            DRIVER_MAJOR=$(nvidia-smi --query-gpu=driver_version --format=csv,noheader 2>/dev/null | head -1 | cut -d. -f1 || true)
        if [ -n "$DRIVER_MAJOR" ]; then
            FM_PKG="nvidia-fabricmanager-${DRIVER_MAJOR}"
        else
            FM_PKG="nvidia-fabricmanager"
        fi
        echo "  Installing ${FM_PKG}..."
        set +e
        "${APT_INSTALL[@]}" "$FM_PKG" 2>&1 | tail -10
        FM_EXIT=${PIPESTATUS[0]}
        set -e
        if [ $FM_EXIT -eq 0 ]; then
            sudo systemctl enable nvidia-fabricmanager 2>/dev/null || true
            sudo systemctl start nvidia-fabricmanager 2>/dev/null || true
            echo "  ✓ Fabric Manager installed and enabled."
        else
            echo "  WARNING: Failed to install ${FM_PKG}. Install manually after reboot:"
            echo "    sudo apt install ${FM_PKG} && sudo systemctl enable --now nvidia-fabricmanager"
        fi
    else
        [ "$GPU_COUNT" -gt 1 ] 2>/dev/null && \
            echo "  ${GPU_COUNT} GPUs detected (PCIe, no NVSwitch). Fabric Manager not needed." || \
            echo "  Single GPU detected. Fabric Manager not needed."
    fi

    if [ "$GPU_COUNT" -gt 1 ] 2>/dev/null; then
        if systemctl list-unit-files nvidia-persistenced.service &>/dev/null; then
            sudo systemctl enable nvidia-persistenced 2>/dev/null || true
            sudo systemctl start nvidia-persistenced 2>/dev/null || true
            echo "  ✓ nvidia-persistenced enabled."
        fi
    fi

    # ----------------------------------------------------------
    # NVIDIA Container Toolkit
    # ----------------------------------------------------------
    if [ "$INSTALL_CONTAINER_TOOLKIT" = true ]; then
        echo ""
        echo "========== NVIDIA Container Toolkit =========="

        curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | \
            sudo gpg --dearmor --yes -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
        curl -s -L https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list | \
            sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | \
            sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list > /dev/null
        sudo apt-get update -qq

        echo "Installing nvidia-container-toolkit..."
        set +e
        "${APT_INSTALL[@]}" nvidia-container-toolkit 2>&1 | tail -10
        CTK_EXIT=${PIPESTATUS[0]}
        set -e
        [ $CTK_EXIT -ne 0 ] && echo "WARNING: Container Toolkit installation failed." || echo "NVIDIA Container Toolkit installed."

        echo ""
        echo "Configuring container runtimes..."
        if command -v containerd &>/dev/null; then
            sudo nvidia-ctk runtime configure --runtime=containerd --set-as-default 2>/dev/null || true
            systemctl is-active --quiet containerd && sudo systemctl restart containerd && \
                echo "  ✓ containerd configured and restarted" || echo "  ✓ containerd configured"
        else
            echo "  - containerd not found"
        fi
        if command -v docker &>/dev/null; then
            sudo nvidia-ctk runtime configure --runtime=docker 2>/dev/null || true
            systemctl is-active --quiet docker && sudo systemctl restart docker && \
                echo "  ✓ Docker configured and restarted" || echo "  ✓ Docker configured"
        else
            echo "  - Docker not found (optional)"
        fi
    fi

    # ----------------------------------------------------------
    # Summary
    # ----------------------------------------------------------
    echo ""
    echo "========== Installation Complete =========="
    COMPONENTS="Driver"
    [ "$INSTALL_CONTAINER_TOOLKIT" = true ] && COMPONENTS="$COMPONENTS, Container-Toolkit"
    [ "$INSTALL_TOOLKIT" = true ] && COMPONENTS="$COMPONENTS, CUDA-Toolkit"
    [ "$NEED_FABRIC_MANAGER" = true ] && COMPONENTS="$COMPONENTS, Fabric-Manager"
    echo "  Installed: $COMPONENTS"
    echo "  Verify after reboot: nvidia-smi"

# ============================================================
# ============================================================
#  AMD ROCm PATH
# ============================================================
# ============================================================
elif [ "$GPU_TYPE" = "amd" ]; then

    # ----------------------------------------------------------
    # Cleanup broken AMD/ROCm packages from previous failures
    # ----------------------------------------------------------
    BROKEN_AMD=$(dpkg -l 2>/dev/null | grep -E "amdgpu|rocm" | grep -v "^ii " | grep -v "^un " | awk '{print $2}' || true)
    if [ -n "$BROKEN_AMD" ]; then
        echo "Found broken AMD/ROCm packages. Purging..."
        echo "  Packages: $(echo $BROKEN_AMD | tr '\n' ' ')"
        sudo dpkg --force-all --purge $BROKEN_AMD 2>&1 | tail -5 || true
        sudo dpkg --configure -a 2>/dev/null || true
    fi

    # ----------------------------------------------------------
    # Pre-flight: show detected AMD GPU info
    # ----------------------------------------------------------
    echo "AMD GPU info:"
    sudo lspci | grep -i -E "vga|3d|display" || true
    echo ""
    AVAILABLE_GB=$(df -BG / | awk 'NR==2 {gsub("G",""); print $4}')
    if [ "$AVAILABLE_GB" -lt 10 ]; then
        echo "WARNING: Low disk space. Available: ${AVAILABLE_GB}GB, ROCm requires ~10GB."
    fi

    # ----------------------------------------------------------
    # Remove any existing AMD GPU drivers and ROCm packages
    # ----------------------------------------------------------
    echo ""
    echo "========== Cleaning Previous AMD Installations =========="
    sudo env DEBIAN_FRONTEND=noninteractive apt-get remove --purge $APT_OPTS_STR \
        amdgpu-install amdgpu-dkms amdgpu rocm-dev rocm-libs rocm-core > /dev/null 2>&1 || true
    sudo env DEBIAN_FRONTEND=noninteractive apt-get autoremove $APT_OPTS_STR > /dev/null 2>&1 || true
    sudo rm -f /etc/apt/sources.list.d/amdgpu.list /etc/apt/sources.list.d/rocm.list

    # ----------------------------------------------------------
    # Install dependencies
    # ----------------------------------------------------------
    echo ""
    echo "========== Installing Dependencies =========="
    sudo apt-get update -qq
    "${APT_INSTALL[@]}" \
        "linux-headers-$(uname -r)" \
        "linux-modules-extra-$(uname -r)" \
        python3-setuptools python3-wheel 2>&1 | tail -5

    # Verify kernel headers
    BUILD_DIR="/lib/modules/$(uname -r)/build"
    if [ ! -d "$BUILD_DIR" ]; then
        echo "WARNING: Kernel headers not found at $BUILD_DIR. DKMS may fail."
    else
        echo "  ✓ Kernel headers: $BUILD_DIR"
    fi

    # ----------------------------------------------------------
    # User group assignment for GPU access
    # ----------------------------------------------------------
    TARGET_USER=${SUDO_USER:-$USER}
    echo "  Adding $TARGET_USER to render and video groups..."
    sudo usermod -a -G render,video "$TARGET_USER"
    echo "  ✓ Groups: $(groups $TARGET_USER)"

    # ----------------------------------------------------------
    # Download and install amdgpu-install tool
    # ----------------------------------------------------------
    echo ""
    echo "========== Downloading ROCm ${ROCM_VERSION} =========="

    AMDGPU_DEB="amdgpu-install_${ROCM_BUILD}_all.deb"
    AMDGPU_URL="https://repo.radeon.com/amdgpu-install/${ROCM_VERSION}/ubuntu/jammy/${AMDGPU_DEB}"

    echo "  URL: $AMDGPU_URL"
    cd /tmp
    wget -q "$AMDGPU_URL"

    if [ ! -f "$AMDGPU_DEB" ]; then
        echo "ERROR: Download failed. Version ${ROCM_VERSION} not found."
        exit 1
    fi
    echo "  Download complete."

    echo "  Installing amdgpu-install tool..."
    sudo env DEBIAN_FRONTEND=noninteractive apt-get install $APT_OPTS_STR "./${AMDGPU_DEB}" > /dev/null
    sudo env DEBIAN_FRONTEND=noninteractive apt-get update -q

    # ----------------------------------------------------------
    # Build and install AMD driver + ROCm
    # ----------------------------------------------------------
    echo ""
    echo "========== Building AMD Driver and ROCm (this may take 10-15 mins) =========="

    sudo env DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a \
        apt-get install $APT_OPTS_STR amdgpu-dkms rocm

    # ----------------------------------------------------------
    # Configure library paths and environment
    # ----------------------------------------------------------
    echo ""
    echo "========== Configuring Environment =========="

    echo "/opt/rocm/lib" | sudo tee /etc/ld.so.conf.d/rocm.conf > /dev/null
    echo "/opt/rocm/lib64" | sudo tee -a /etc/ld.so.conf.d/rocm.conf > /dev/null
    sudo ldconfig

    if ! grep -q "/opt/rocm/bin" "/home/${TARGET_USER}/.bashrc" 2>/dev/null; then
        echo 'export PATH=$PATH:/opt/rocm/bin:/opt/rocm/opencl/bin' >> "/home/${TARGET_USER}/.bashrc"
        echo "  ROCm PATH added to ~/.bashrc"
    fi
    echo "  ✓ Library paths configured."

    # Ensure amdgpu is not blacklisted
    sudo sed -i '/blacklist amdgpu/d' /etc/modprobe.d/*.conf 2>/dev/null || true

    # Update initramfs to load amdgpu on boot
    echo "  Updating initramfs..."
    sudo update-initramfs -uk all > /dev/null 2>&1
    echo "  ✓ initramfs updated."

    # ----------------------------------------------------------
    # Summary
    # ----------------------------------------------------------
    echo ""
    echo "========== Installation Complete =========="
    echo "  Installed: ROCm ${ROCM_VERSION} (amdgpu-dkms + rocm)"
    echo "  Verify after reboot: rocm-smi"

fi  # end GPU_TYPE

# ============================================================
# Reboot
# ============================================================
echo ""
if [ "$AUTO_REBOOT" = true ]; then
    echo "Rebooting in 5 seconds... (SSH will disconnect, verify with ${GPU_TYPE}-smi after ~60s)"
    nohup bash -c 'sleep 5 && sudo reboot' > /dev/null 2>&1 &
    exit 0
else
    echo "REBOOT REQUIRED: run 'sudo reboot' to complete installation."
fi
