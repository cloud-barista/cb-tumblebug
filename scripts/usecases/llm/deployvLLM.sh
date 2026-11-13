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

# =========================================================
# Argument Parsing
# =========================================================
HF_TOKEN="${HF_TOKEN:-}"   # inherit from env if already set (avoids CLI arg exposure)
VLLM_VERSION=""            # empty = install latest

while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --hf-token) HF_TOKEN="${2?Error: --hf-token requires an argument}";     shift 2 ;;
    --version)  VLLM_VERSION="${2:?Error: --version requires a value}"; shift 2 ;;
    -h|--help)
      echo "Usage: bash deployvLLM.sh [OPTIONS]"
      echo "  --hf-token TOKEN   HuggingFace token for gated models"
      echo "  --version VER      Pin vLLM version (e.g. 0.22.0). Default: latest"
      exit 0 ;;
    *) echo "Unknown parameter: $1"; exit 1 ;;
  esac
done

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
  GPU_COUNT=$(rocm-smi -i | grep -oE '^GPU\[[0-9]+\]' | sort -u | wc -l)
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

# Fail fast if disk space cannot even hold vLLM + PyTorch/CUDA runtime (~10GB),
# and warn if space is tight for 7B-8B BF16 model weights (~15-25GB).
if [ "$DISK_AVAIL" -lt 12 ] 2>/dev/null; then
  echo "Error: Insufficient disk space (${DISK_AVAIL}GB available)."
  echo "  vLLM + GPU runtime libraries require at least 12GB free (rootDiskSize >= 100GB recommended for LLM weights)."
  exit 1
elif [ "$DISK_AVAIL" -lt 35 ] 2>/dev/null; then
  echo "Warning: Low disk space (${DISK_AVAIL}GB available). vLLM installs with --no-cache (~10GB), so 7B-8B BF16 models (~15-25GB) may fill the disk."
  echo "  Recommended VM rootDiskSize: 100GB+."
fi

# Install system dependencies
echo "Installing system dependencies..."
export DEBIAN_FRONTEND=noninteractive
sudo apt-get -o DPkg::Lock::Timeout=600 update -qq

# Use Python 3.12 for both NVIDIA and AMD:
#   - AMD pre-built ROCm wheels are Python 3.12 only (cp312)
#   - vLLM 0.23+ docs quickstart recommends Python 3.12 for NVIDIA too
#   - build-essential provides gcc/g++ needed by Triton and FlashInfer JIT compilation
# Ubuntu 22.04 ships Python 3.10 by default; add deadsnakes PPA when needed.
sudo apt-get -o DPkg::Lock::Timeout=600 install -y python3-pip build-essential curl jq > /dev/null 2>&1
if ! apt-cache show python3.12 > /dev/null 2>&1; then
  echo "  Adding deadsnakes PPA for python3.12..."
  sudo apt-get -o DPkg::Lock::Timeout=600 install -y software-properties-common > /dev/null 2>&1
  sudo add-apt-repository -y ppa:deadsnakes/ppa > /dev/null 2>&1
  sudo apt-get -o DPkg::Lock::Timeout=600 update -qq
fi
sudo apt-get -o DPkg::Lock::Timeout=600 install -y python3.12 python3.12-venv python3.12-dev > /dev/null 2>&1
PYTHON_BIN="python3.12"
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

# Configure HuggingFace token (needed for gated models like meta-llama/*, mistralai/*)
if [ -n "$HF_TOKEN" ]; then
  export HF_TOKEN
  export HUGGING_FACE_HUB_TOKEN="$HF_TOKEN"
  echo "HuggingFace token configured."
fi

# Install vLLM
VLLM_SPEC="${VLLM_VERSION:+vllm==${VLLM_VERSION}}"
VLLM_SPEC="${VLLM_SPEC:-vllm}"
echo "Installing ${VLLM_SPEC} for $GPU_TYPE GPU(s) (this may take a few minutes)..."
LOG_FILE="$HOME/vllm_install.log"
echo "Logging vLLM installation details to $LOG_FILE"

set +e  # Temporarily disable exit on error for pip install

if [ "$GPU_TYPE" = "nvidia" ]; then
  # Install uv if not available — required for --torch-backend=auto (latest vLLM docs recommendation)
  if ! command -v uv &>/dev/null; then
    echo "Installing uv (fast Python package manager)..."
    pip install uv >> "$LOG_FILE" 2>&1
  fi
  # --torch-backend=auto: lets uv select the correct CUDA wheel for the installed driver.
  # --no-cache: avoids leaving ~5-6GB of duplicate PyTorch/CUDA wheel archives in ~/.cache/uv.
  echo "Installing vLLM with CUDA backend (uv --no-cache --torch-backend=auto)..."
  uv pip install --no-cache "$VLLM_SPEC" --torch-backend=auto >> "$LOG_FILE" 2>&1
  INSTALL_RESULT=$?
else
  # === AMD ROCm: install pre-built ROCm vllm wheel via uv ===
  # Each release publishes one wheel under wheels.vllm.ai/rocm/<version>/<rocmNNN>/
  # (Python 3.12 only). uv must be used: it gives --extra-index-url priority over PyPI,
  # otherwise pip picks the CUDA wheel.
  # The bare rocm/ index only carries the latest release, and its glibc requirement can
  # exceed the host's (0.28.0 needs glibc 2.39; Ubuntu 22.04 has 2.35), so the newest
  # release whose wheel tag fits the host glibc is selected instead.
  ROCM_VERSION=$(cat /opt/rocm/.info/version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+' | head -1)
  ROCM_VERSION=${ROCM_VERSION:-$(rocm-smi --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+' | head -1)}
  echo "Installed ROCm version: ${ROCM_VERSION:-unknown}"
  ROCM_WHEEL_INDEX="https://wheels.vllm.ai/rocm"
  GLIBC_MINOR=$(getconf GNU_LIBC_VERSION 2>/dev/null | grep -oE '[0-9]+$')

  # Install uv if not already available
  if ! command -v uv &>/dev/null; then
    echo "Installing uv (fast Python package manager)..."
    pip install uv >> "$LOG_FILE" 2>&1
  fi

  rocm_wheel_glibc() {  # $1=version $2=variant -> minimum glibc minor of the vllm wheel
    curl -fsSL -m 20 "$ROCM_WHEEL_INDEX/$1/$2/vllm/" 2>/dev/null | grep -oE 'manylinux_2_[0-9]+' | head -1 | grep -oE '[0-9]+$'
  }
  rocm_variant() {  # $1=version -> rocmNNN subdirectory, empty if no wheel for this release
    curl -fsSL -m 20 "$ROCM_WHEEL_INDEX/$1/" 2>/dev/null | grep -oE 'rocm[0-9]+' | head -1
  }

  ROCM_VLLM_VERSION=""; ROCM_VLLM_VARIANT=""
  if [ -n "$VLLM_VERSION" ]; then
    ROCM_VLLM_VARIANT=$(rocm_variant "$VLLM_VERSION")
    if [ -z "$ROCM_VLLM_VARIANT" ]; then
      echo "ERROR: no ROCm wheel published for vllm ${VLLM_VERSION} (see $ROCM_WHEEL_INDEX/)."
      INSTALL_RESULT=1
    else
      ROCM_VLLM_VERSION="$VLLM_VERSION"
      _req=$(rocm_wheel_glibc "$ROCM_VLLM_VERSION" "$ROCM_VLLM_VARIANT")
      if [ -n "$_req" ] && [ -n "$GLIBC_MINOR" ] && [ "$_req" -gt "$GLIBC_MINOR" ]; then
        echo "Warning: vllm ${VLLM_VERSION} ROCm wheel needs glibc 2.${_req}; host has 2.${GLIBC_MINOR}. Install will likely fail."
      fi
    fi
  else
    echo "Selecting the newest vLLM release with a ROCm wheel for glibc 2.${GLIBC_MINOR:-?}..."
    for _ver in $(curl -fsSL -m 20 https://pypi.org/pypi/vllm/json 2>/dev/null \
                  | jq -r '.releases | keys[]' | grep -E '^[0-9]+\.[0-9]+\.[0-9]+$' | sort -rV | head -15); do
      _variant=$(rocm_variant "$_ver"); [ -z "$_variant" ] && continue
      _req=$(rocm_wheel_glibc "$_ver" "$_variant")
      if [ -n "$_req" ] && [ -n "$GLIBC_MINOR" ] && [ "$_req" -le "$GLIBC_MINOR" ]; then
        ROCM_VLLM_VERSION="$_ver"; ROCM_VLLM_VARIANT="$_variant"; break
      fi
    done
    if [ -z "$ROCM_VLLM_VERSION" ]; then
      echo "Warning: could not match a ROCm wheel to host glibc; falling back to the latest index."
    fi
  fi

  if [ "${INSTALL_RESULT:-0}" -eq 0 ]; then
    if [ -n "$ROCM_VLLM_VERSION" ]; then
      # Each index rebuilds torchvision/torchaudio/flash-attn against its own torch but keeps
      # the same version strings, so switching vLLM versions in an existing venv leaves stale
      # binaries behind (undefined HIP symbols at runtime). Reinstall the whole set then.
      _cur=$(python -c "import importlib.metadata as m; print(m.version('vllm'))" 2>/dev/null || true)
      _reinstall=""
      if [ -n "$_cur" ] && [ "$_cur" != "${ROCM_VLLM_VERSION}+${ROCM_VLLM_VARIANT}" ]; then
        echo "  Existing vllm $_cur differs; reinstalling all packages."
        _reinstall="--reinstall"
      fi
      echo "Installing vllm ${ROCM_VLLM_VERSION}+${ROCM_VLLM_VARIANT} from $ROCM_WHEEL_INDEX/${ROCM_VLLM_VERSION}/${ROCM_VLLM_VARIANT}/ ..."
      uv pip install --no-cache $_reinstall "vllm==${ROCM_VLLM_VERSION}" --extra-index-url "$ROCM_WHEEL_INDEX/${ROCM_VLLM_VERSION}/${ROCM_VLLM_VARIANT}/" >> "$LOG_FILE" 2>&1
    else
      echo "Installing pre-built ROCm vllm wheel from $ROCM_WHEEL_INDEX/ ..."
      uv pip install --no-cache vllm --extra-index-url "$ROCM_WHEEL_INDEX/" >> "$LOG_FILE" 2>&1
    fi
    INSTALL_RESULT=$?
  fi

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
    echo "  Host: glibc 2.${GLIBC_MINOR:-?}, ROCm ${ROCM_VERSION:-unknown}, Python 3.12"
    echo "  Check available wheels: $ROCM_WHEEL_INDEX/<version>/"
    echo "  See $LOG_FILE for details."
  fi
fi

set -e

if [ $INSTALL_RESULT -ne 0 ]; then
  echo "vLLM installation failed (exit code: $INSTALL_RESULT)."
  echo "--- Last 40 lines of $LOG_FILE ---"
  tail -n 40 "$LOG_FILE" 2>/dev/null || true
  echo "----------------------------------"
  exit 1
fi

# FlashInfer (bundled with vLLM 0.23+) JIT-compiles CUDA sampling kernels at startup.
# This requires BOTH a compatible 'nvcc' AND 'curand.h' (libcurand-dev), matched to the
# installed PyTorch CUDA version rather than blindly installing a newer experimental toolkit.
if [ "$GPU_TYPE" = "nvidia" ]; then
  _torch_cuda=$(python -c "import torch; print(torch.version.cuda or '')" 2>/dev/null || true)
  _drv_cuda=$(nvidia-smi 2>/dev/null | sed -n 's/.*CUDA Version: \([0-9]\+\.[0-9]\+\).*/\1/p' | head -1 || true)
  _target_cuda="${_torch_cuda:-$_drv_cuda}"
  _target_dash="${_target_cuda//./-}"

  _cuda_ver_num() {
    echo "$1" | awk -F'[.-]' 'NF>=2 {print $1*100 + $2}'
  }

  _existing_nvcc=""
  if [ -x /usr/local/cuda/bin/nvcc ]; then
    _existing_nvcc="/usr/local/cuda/bin/nvcc"
  else
    _existing_nvcc=$(ls /usr/local/cuda-*/bin/nvcc 2>/dev/null | sort -V | tail -1 || true)
    [ -z "$_existing_nvcc" ] && command -v nvcc &>/dev/null && _existing_nvcc="$(command -v nvcc)"
  fi

  _need_nvcc=false
  if [ -z "$_existing_nvcc" ]; then
    _need_nvcc=true
  else
    # Ubuntu universe's /usr/bin/nvcc (CUDA 11.5) fails FlashInfer FP8/FP4 JIT flags (needs CUDA >= 12.8 when torch >= 12.8)
    _nvcc_rel=$("$_existing_nvcc" --version 2>/dev/null | sed -n 's/.*release \([0-9]\+\.[0-9]\+\).*/\1/p' | head -1 || true)
    _nvcc_num=$(_cuda_ver_num "${_nvcc_rel:-0.0}")
    _torch_num=$(_cuda_ver_num "${_torch_cuda:-0.0}")
    if [ "${_torch_num:-0}" -ge 1208 ] && [ "${_nvcc_num:-0}" -gt 0 ] && [ "${_nvcc_num:-0}" -lt 1208 ]; then
      echo "Existing nvcc ($_existing_nvcc, release $_nvcc_rel) is older than required for PyTorch CUDA $_torch_cuda; installing matching cuda-nvcc..."
      _need_nvcc=true
    fi
  fi

  ensure_cuda_apt_repo() {
    if ! apt-cache search --names-only '^cuda-nvcc-' 2>/dev/null | grep -q .; then
      echo "  CUDA apt repo not found — adding keyring..."
      _ubuntu_ver=$(. /etc/os-release 2>/dev/null && echo "${VERSION_ID//./}" || echo "2204")
      [[ "$_ubuntu_ver" == "2404" ]] && _cuda_repo="ubuntu2404" || _cuda_repo="ubuntu2204"
      _kring=$(mktemp)
      curl -fsSL "https://developer.download.nvidia.com/compute/cuda/repos/${_cuda_repo}/x86_64/cuda-keyring_1.1-1_all.deb" \
        -o "$_kring" 2>/dev/null && \
        sudo dpkg -i --force-confdef --force-confold "$_kring" 2>/dev/null || true
      rm -f "$_kring"
      sudo apt-get -o DPkg::Lock::Timeout=600 update -qq
    fi
  }

  _chosen_cuda_dash=""
  if [ "$_need_nvcc" = true ]; then
    echo "Installing cuda-nvcc (required for FlashInfer JIT compilation; target CUDA: ${_target_cuda:-latest})..."
    ensure_cuda_apt_repo
    _nvcc_pkg=""
    if [ -n "$_target_dash" ] && apt-cache show "cuda-nvcc-${_target_dash}" &>/dev/null; then
      _nvcc_pkg="cuda-nvcc-${_target_dash}"
    elif [ -n "$_target_cuda" ]; then
      _tgt_num=$(_cuda_ver_num "$_target_cuda")
      _nvcc_pkg=$(apt-cache search --names-only '^cuda-nvcc-[0-9]+-[0-9]+$' 2>/dev/null \
        | awk '{print $1}' | sort -V | awk -F'-' -v tgt="${_tgt_num:-9999}" '
          NF>=4 { n = $3*100 + $4; if (n <= tgt) best = $0 }
          END { print best }')
    fi
    if [ -z "$_nvcc_pkg" ]; then
      _nvcc_pkg=$(apt-cache search --names-only '^cuda-nvcc-[0-9]+-[0-9]+$' 2>/dev/null \
        | awk '{print $1}' | sort -V | tail -1)
    fi
    [ -z "$_nvcc_pkg" ] && _nvcc_pkg="cuda-nvcc"
    echo "  Installing $_nvcc_pkg..."
    set +e
    sudo apt-get -o DPkg::Lock::Timeout=600 install -y --no-install-recommends "$_nvcc_pkg" 2>&1 | tail -5
    _nvcc_exit=${PIPESTATUS[0]}
    set -e
    if [ $_nvcc_exit -ne 0 ]; then
      echo "  Warning: $_nvcc_pkg install failed — FlashInfer JIT will fail at runtime."
    fi
    _chosen_cuda_dash=$(echo "$_nvcc_pkg" | grep -oE '[0-9]+-[0-9]+$' 2>/dev/null || true)
  else
    _real_nvcc=$(readlink -f "$_existing_nvcc" 2>/dev/null || echo "$_existing_nvcc")
    _chosen_cuda_dash=$(echo "$_real_nvcc" | sed -n 's|.*/cuda-\([0-9]\+\)\.\([0-9]\+\)/.*|\1-\2|p' | head -1)
    if [ -z "$_chosen_cuda_dash" ]; then
      _nvcc_rel=$("$_existing_nvcc" --version 2>/dev/null | sed -n 's/.*release \([0-9]\+\.[0-9]\+\).*/\1/p' | head -1 || true)
      _chosen_cuda_dash="${_nvcc_rel//./-}"
    fi
    [ -z "$_chosen_cuda_dash" ] && _chosen_cuda_dash="$_target_dash"
    echo "Found compatible nvcc: $_existing_nvcc (CUDA ${_chosen_cuda_dash/-/.})"
  fi

  # Ensure /usr/local/cuda points to a valid CUDA toolkit containing bin/nvcc
  _preferred_cuda_dir=""
  if [ -n "$_chosen_cuda_dash" ] && [ -x "/usr/local/cuda-${_chosen_cuda_dash/-/.}/bin/nvcc" ]; then
    _preferred_cuda_dir="/usr/local/cuda-${_chosen_cuda_dash/-/.}"
  fi
  if [ ! -x /usr/local/cuda/bin/nvcc ] || [ "$_need_nvcc" = true ]; then
    [ -z "$_preferred_cuda_dir" ] && _preferred_cuda_dir=$(ls -d /usr/local/cuda-*/ 2>/dev/null | sort -V | tail -1 | sed 's|/$||')
    if [ -n "$_preferred_cuda_dir" ] && [ -x "$_preferred_cuda_dir/bin/nvcc" ]; then
      sudo ln -sfn "$_preferred_cuda_dir" /usr/local/cuda
      echo "  Configured /usr/local/cuda -> $_preferred_cuda_dir"
    fi
  fi

  # FlashInfer sampling.cuh #includes <curand.h>, which is NOT part of cuda-nvcc and may
  # also be missing on images where nvcc was pre-installed without full dev headers.
  _cuda_check_dir="${_preferred_cuda_dir:-/usr/local/cuda}"
  if ! [ -f "$_cuda_check_dir/include/curand.h" ] && \
     ! ls "$_cuda_check_dir"/targets/*/include/curand.h &>/dev/null && \
     ! [ -f /usr/include/curand.h ]; then
    ensure_cuda_apt_repo
    _curand_pkg=""
    if [ -n "$_chosen_cuda_dash" ] && apt-cache show "libcurand-dev-${_chosen_cuda_dash}" &>/dev/null; then
      _curand_pkg="libcurand-dev-${_chosen_cuda_dash}"
    else
      _curand_pkg=$(apt-cache search --names-only '^libcurand-dev-[0-9]+-[0-9]+$' 2>/dev/null \
        | awk '{print $1}' | sort -V | tail -1)
      [ -z "$_curand_pkg" ] && _curand_pkg="libcurand-dev"
    fi
    echo "  Installing $_curand_pkg (curand.h required by FlashInfer sampling JIT)..."
    set +e
    sudo apt-get -o DPkg::Lock::Timeout=600 install -y --no-install-recommends "$_curand_pkg" 2>&1 | tail -3
    _curand_exit=${PIPESTATUS[0]}
    set -e
    if [ $_curand_exit -ne 0 ]; then
      echo "  Warning: $_curand_pkg install failed — FlashInfer sampling JIT will fail at runtime."
    fi
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
uv pip install --no-cache openai huggingface_hub >> "$LOG_FILE" 2>&1 || pip install --no-cache-dir openai huggingface_hub > /dev/null 2>&1

# Patch prometheus_fastapi_instrumentator routing.py:
# All known versions have an unfixed bug — '_IncludedRouter' (Starlette ≥0.40,
# bundled with vLLM 0.23+) has no .path attribute and crashes every HTTP request.
# This affects both NVIDIA and AMD. Patch applies to all GPU types.
echo "Patching prometheus_fastapi_instrumentator routing.py..."
ROUTING_FILE="$(python -c "import os, prometheus_fastapi_instrumentator as p; print(os.path.join(os.path.dirname(p.__file__), 'routing.py'))" 2>/dev/null || echo "")"
if [ -f "$ROUTING_FILE" ]; then
  sed -i 's/route_name = route\.path$/route_name = getattr(route, "path", None)/g' "$ROUTING_FILE" && \
    echo "  Patched: route.path → getattr(route, 'path', None)" || \
    echo "Warning: Could not patch routing.py (non-fatal)"
else
  echo "Warning: routing.py not found, skipping patch (non-fatal)"
fi

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
echo "  vllm serve <model_name> --host 0.0.0.0 --port 8000"
echo ""
echo "Recommended models (24GB GPU such as NVIDIA L4):"
echo "  - Qwen/Qwen2.5-7B-Instruct           (~14GB VRAM, ungated)"
echo "  - microsoft/Phi-4-mini-instruct      (~8GB VRAM, ungated)"
echo "  - Qwen/Qwen2.5-1.5B-Instruct         (~3GB VRAM, ungated)"
echo "  - meta-llama/Llama-3.1-8B-Instruct   (~16GB VRAM, gated — requires --hf-token)"
echo ""
