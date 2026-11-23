#!/bin/bash
set -e

GO_VERSION=1.26.2
KIND_VERSION=${KIND_VERSION:-v0.32.0}
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64)  GO_ARCH=amd64 ;;
  aarch64|arm64) GO_ARCH=arm64 ;;
  *)             GO_ARCH=amd64 ;;
esac
GO_TAR=go${GO_VERSION}.linux-${GO_ARCH}.tar.gz
CB_DIR=$HOME/go/src/github.com/cloud-barista/cb-tumblebug
SPEC_WARNING="false"

# ──────────────
# System Requirement Check (Non-interactive)
# ──────────────
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📋 CB-Tumblebug Setup Prerequisites (Kubernetes & Compose)"

echo
echo "✅ Recommended:"
echo "   - OS: Ubuntu 22.04 / 24.04 LTS"
echo "   - vCPU: 4 or more (8+ recommended for full K8s stack)"
echo "   - RAM: 8 GiB or more"
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
# Install base packages
# ──────────────
echo
echo "📦 Installing base packages..."
sudo DEBIAN_FRONTEND=noninteractive apt-get update -qq
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq make gcc git curl wget jq ca-certificates

# ──────────────
# Install Go (if needed)
# ──────────────
echo
export PATH="$PATH:/usr/local/go/bin:$HOME/go/bin:$HOME/.local/bin:$HOME/.cargo/bin"
export GOPATH="$HOME/go"

if ! command -v go >/dev/null 2>&1 || ! go version | grep -q "$GO_VERSION"; then
  echo "⬇️ Installing Go $GO_VERSION (${GO_ARCH})..."
  wget -q "https://go.dev/dl/${GO_TAR}" -O "/tmp/${GO_TAR}"
  sudo rm -rf /usr/local/go
  sudo tar -C /usr/local -xzf "/tmp/${GO_TAR}"
  rm -f "/tmp/${GO_TAR}"
else
  echo "✅ Go $GO_VERSION already installed. Skipping."
fi

if ! grep -q 'export GOPATH=' ~/.bashrc 2>/dev/null; then
  echo "🔧 Setting up Go environment in ~/.bashrc..."
  {
    echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin:$HOME/.local/bin'
    echo 'export GOPATH=$HOME/go'
  } >> ~/.bashrc
fi

go version

# ──────────────
# Clone CB-Tumblebug
# ──────────────
echo
if [ -d "$CB_DIR" ]; then
  echo "✅ CB-Tumblebug already cloned at $CB_DIR. Skipping clone."
else
  echo "📥 Cloning CB-Tumblebug..."
  mkdir -p "$(dirname "$CB_DIR")"
  git clone https://github.com/cloud-barista/cb-tumblebug.git "$CB_DIR"
fi
cd "$CB_DIR"

# ──────────────
# Create ~/.cloud-barista & .env
# ──────────────
mkdir -p ~/.cloud-barista

echo
if [ ! -f "$CB_DIR/.env" ]; then
  echo "📋 Creating .env from .env.example with default credentials..."
  cp "$CB_DIR/.env.example" "$CB_DIR/.env"
  sed -i 's/^TB_API_USERNAME=$/TB_API_USERNAME=default/' "$CB_DIR/.env"
  sed -i 's/^TB_API_PASSWORD=$/TB_API_PASSWORD=default/' "$CB_DIR/.env"
  sed -i 's/^SPIDER_USERNAME=$/SPIDER_USERNAME=default/' "$CB_DIR/.env"
  sed -i 's/^SPIDER_PASSWORD=$/SPIDER_PASSWORD=default/' "$CB_DIR/.env"
  sed -i 's/^TERRARIUM_API_USERNAME=$/TERRARIUM_API_USERNAME=default/' "$CB_DIR/.env"
  sed -i 's/^TERRARIUM_API_PASSWORD=$/TERRARIUM_API_PASSWORD=default/' "$CB_DIR/.env"
  echo "✅ .env created (default / default)."
else
  echo "✅ .env already exists at $CB_DIR/.env. Skipping."
fi

# ──────────────
# Register aliases
# ──────────────
echo
if ! grep -q "alias cdtb=" ~/.bashrc 2>/dev/null; then
  echo "🔗 Registering aliases in ~/.bashrc..."
  {
    echo "alias cdtb='cd $CB_DIR'"
    echo "alias cdtbsrc='cd $CB_DIR/src'"
    echo "alias cdtbtest='cd $CB_DIR/src/testclient/scripts'"
  } >> ~/.bashrc
fi

# ──────────────
# Install Docker + Kubernetes prerequisites (kubectl, helm, kind, inotify sysctls)
# ──────────────
echo
echo "☸️ Installing Docker & Kubernetes prerequisites (kubectl, helm, kind, sysctls)..."
YES=1 INSTALL_DOCKER=1 KIND_VERSION="$KIND_VERSION" bash "$CB_DIR/scripts/misc/k-prereqs.sh"

DOCKER_NEEDS_SUDO="false"
if ! docker ps >/dev/null 2>&1; then
  DOCKER_NEEDS_SUDO="true"
fi

# ──────────────
# Install uv for init.py
# ──────────────
echo
echo "🧩 Checking for Python package manager 'uv'..."
if ! command -v uv >/dev/null 2>&1; then
  echo "→ 'uv' not found. Installing now..."
  curl -LsSf https://astral.sh/uv/install.sh | sh
  export PATH="$HOME/.local/bin:$HOME/.cargo/bin:$PATH"
  if ! grep -q '\.local/bin' ~/.bashrc 2>/dev/null; then
    echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
  fi
  echo "✅ uv installed successfully!"
else
  echo "✅ uv is already installed."
fi

# ──────────────
# Done
# ──────────────
echo
echo "✅ CB-Tumblebug host setup is complete."
echo "📁 Directory: $CB_DIR"
echo
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📌 NEXT STEPS"
echo
echo "👉 Option A — Kubernetes Deployment (Recommended):"
echo "   1. (If this is your first login after setup) Apply docker group:"
echo "      newgrp docker   # or reconnect SSH"
echo "   2. Deploy K8s stack (auto-creates kind cluster + Envoy Gateway + OpenBao):"
echo "      cd $CB_DIR && make k-up"
echo "   3. Configure credentials & initialize:"
echo "      cd $CB_DIR && make gen-cred && make enc-cred && make k-init"
echo "      (Or initialize remotely from a bootstrapper PC: make k-init HOST=<this-vm-ip>)"
echo
echo "👉 Option B — Docker Compose Deployment:"
echo "   cd $CB_DIR && make up"
echo "   cd $CB_DIR && make gen-cred && make enc-cred && make init"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [ "$DOCKER_NEEDS_SUDO" = "true" ]; then
  echo
  echo "🚪 Reconnect SSH (or run 'newgrp docker') so Docker/kind can run without sudo."
fi
if [ "$SPEC_WARNING" = "true" ]; then
  echo
  echo "⚠️  Note: System is below recommended spec (4 vCPU, 6 GiB RAM)."
fi
echo
