#!/bin/bash
set -e

GO_VERSION=1.23.0
GO_TAR=go${GO_VERSION}.linux-amd64.tar.gz
CB_DIR=$HOME/go/src/github.com/cloud-barista/cb-tumblebug
SPEC_WARNING="false"

# ──────────────
# System Requirement Check (Non-interactive)
# ──────────────
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📋 CB-Tumblebug Setup Prerequisites"

echo
echo "✅ Recommended:"
echo "   - OS: Ubuntu 22.04 (Jammy)"
echo "   - vCPU: 4 or more"
echo "   - RAM: 6 GiB or more"
echo "   - Example: AWS c5a.xlarge"
echo
echo "🧪 Checking your system..."

VCPU=$(nproc)
MEM_GB=$(free -g | awk '/^Mem:/{print $2}')

echo "   → Detected: $VCPU vCPU, $MEM_GB GiB memory"

if [ "$VCPU" -lt 4 ] || [ "$MEM_GB" -lt 6 ]; then
  SPEC_WARNING="true"
  echo
  echo "⚠️  WARNING: Your system does not meet the recommended minimum spec (4 vCPU, 6 GiB RAM)"
  echo "   → Proceeding anyway (non-interactive mode)"
else
  echo "✅ Spec check passed."
fi

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# ──────────────
# Install prerequisites
# ──────────────
echo
echo "📦 Installing prerequisites..."
sudo apt update
sudo apt install -y make gcc git curl wget

# ──────────────
# Install Go (if needed)
# ──────────────
echo
if ! go version | grep -q "$GO_VERSION"; then
  echo "⬇️ Installing Go $GO_VERSION..."
  wget -q https://go.dev/dl/${GO_TAR}
  sudo rm -rf /usr/local/go
  sudo tar -C /usr/local -xzf ${GO_TAR}
  rm -f ${GO_TAR}
else
  echo "✅ Go $GO_VERSION already installed. Skipping."
fi

# ──────────────
# Set Go environment
# ──────────────
echo
if ! grep -q 'export GOPATH=' ~/.bashrc; then
  echo "🔧 Setting up Go environment..."
  echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin' >> ~/.bashrc
  echo 'export GOPATH=$HOME/go' >> ~/.bashrc
fi

source ~/.bashrc
export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin
export GOPATH=$HOME/go

go version

# ──────────────
# Clone CB-Tumblebug
# ──────────────
echo
if [ -d "$CB_DIR" ]; then
  echo "✅ CB-Tumblebug already cloned at $CB_DIR. Skipping clone."
else
  echo "🐳 Cloning CB-Tumblebug..."
  mkdir -p "$(dirname "$CB_DIR")"
  git clone https://github.com/cloud-barista/cb-tumblebug.git "$CB_DIR"
fi
cd "$CB_DIR"

# ──────────────
# Create ~/.cloud-barista
# ──────────────
mkdir -p ~/.cloud-barista

# ──────────────
# Register aliases
# ──────────────
echo
if ! grep -q "alias cdtb=" ~/.bashrc; then
  echo "🔗 Registering aliases in ~/.bashrc..."
  {
    echo "alias cdtb='cd $CB_DIR'"
    echo "alias cdtbsrc='cd $CB_DIR/src'"
    echo "alias cdtbtest='cd $CB_DIR/src/testclient/scripts'"
  } >> ~/.bashrc
  echo "ℹ️  Aliases added to ~/.bashrc"
  echo "   → To use them now, run: source ~/.bashrc"
fi

# ──────────────
# Install Docker (if needed)
# ──────────────
echo
if ! command -v docker &> /dev/null; then
  echo "🐳 Installing Docker..."
  curl -fsSL https://get.docker.com | sh
else
  echo "✅ Docker already installed. Skipping."
fi

# ──────────────
# Add user to docker group
# ──────────────
echo
if groups $USER | grep -q '\bdocker\b'; then
  echo "✅ User already in 'docker' group."
else
  echo "👥 Adding user to docker group..."
  sudo groupadd docker 2>/dev/null || true
  sudo usermod -aG docker $USER
fi

# ──────────────
# Install uv for init.py
# ──────────────
echo
echo "🧩 Checking for Python package manager 'uv'..."

if ! command -v uv &> /dev/null; then
    echo "→ 'uv' not found. Installing now..."
    curl -LsSf https://astral.sh/uv/install.sh | sh

    if [ -f "$HOME/.cargo/env" ]; then
        source "$HOME/.cargo/env"
    fi

    export PATH="$HOME/.cargo/bin:$PATH"
    echo 'export PATH="$HOME/.cargo/bin:$PATH"' >> ~/.bashrc

    echo "✅ uv installed successfully!"
else
    echo "✅ uv is already installed."
fi


# ──────────────
# Done
# ──────────────
echo
echo "✅ CB-Tumblebug setup is complete."

echo
echo "📁 Your CB-Tumblebug directory:"
echo "   $CB_DIR"

echo
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📌 NEXT STEPS"
echo
echo "👉 1. Run CB-Tumblebug (Docker Compose):"
echo
echo "   # Option A: Run without building (faster)"
echo "   docker compose up"
echo
echo "   # Option B: Build and run everything (docker --build)"
echo "   make compose"
echo
echo "👉 2. Create your cloud credentials:"
echo
echo "   ./init/genCredential.sh"
echo
echo "   → Then edit ~/.cloud-barista/credentials.yaml with your CSP info."
echo
echo "👉 3. Encrypt the credentials file:"
echo
echo "   ./init/encCredential.sh"
echo
echo "👉 4. Initialize CB-Tumblebug with all connection and resource info:"
echo
echo "   ./init/init.sh"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# ──────────────
# Final Notes
# ──────────────
if [ "$SPEC_WARNING" = "true" ]; then
  echo
  echo "⚠️  Final Warning: Your system is running below recommended spec (4 vCPU, 6 GiB RAM)."
  echo "   CB-Tumblebug may not perform optimally under load."
fi

echo
echo "🚪 To complete the setup:"
echo "   → Please log out and log back in (or open a new SSH session) to:"
echo "     - Apply Docker group permissions (to avoid using sudo)"
echo "     - Activate shell aliases like 'cdtb'"
echo
