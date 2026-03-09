#!/bin/bash
set -e

GO_VERSION=1.25.0
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
# Set up .env with default credentials
# ──────────────
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
  echo "✅ .env created by using default / default for API username and password."
  echo "   → Edit $CB_DIR/.env to use custom credentials."
else
  echo "✅ .env already exists at $CB_DIR/.env. Skipping."
fi

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
  DOCKER_NEEDS_SUDO="false"
else
  echo "👥 Adding user to docker group..."
  sudo groupadd docker 2>/dev/null || true
  sudo usermod -aG docker $USER
  echo "   → User added to docker group"
  echo "   → Note: Group membership will be active after re-login"
  DOCKER_NEEDS_SUDO="true"
fi

# ──────────────
# Test Docker access
# ──────────────
echo
echo "🐳 Testing Docker access..."
if docker ps &>/dev/null; then
  echo "✅ Docker access confirmed (no sudo required)"
  DOCKER_NEEDS_SUDO="false"
elif sudo docker ps &>/dev/null; then
  echo "⚠️  Docker requires sudo (group membership not yet active)"
  echo "   → This is normal for first-time setup"
  DOCKER_NEEDS_SUDO="true"
else
  echo "❌ Docker is not accessible. Please check Docker installation."
  exit 1
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
echo "👉 1. (Optional) Verify or change API credentials:"
echo
echo "   cat $CB_DIR/.env"
echo "   → Default API username and password: default / default"
echo "   → Edit .env to use custom credentials before starting."
echo
echo "👉 2. Start all services using pre-built images:"
echo
echo "   # Option A: Run without building (faster, uses pre-built images)"
echo "   cd $CB_DIR && make up"
echo
echo "   # Option B: Build from source and run"
echo "   cd $CB_DIR && make compose"

if [ "$DOCKER_NEEDS_SUDO" = "true" ]; then
  echo
  echo "   ⚠️  Note: If you see Docker permission errors:"
  echo "   → Use 'sudo docker compose up' temporarily"
  echo "   → Or log out and log back in to activate docker group membership"
fi
echo
echo "👉 3. Create your cloud credentials:"
echo
echo "   ./init/genCredential.sh"
echo
echo "   → Then edit ~/.cloud-barista/credentials.yaml with your CSP info."
echo
echo "👉 4. Encrypt the credentials file:"
echo
echo "   ./init/encCredential.sh"
echo
echo "👉 5. Initialize CB-Tumblebug with all connection and resource info:"
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
