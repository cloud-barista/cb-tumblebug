#!/bin/bash
set -e

MCMP_DIR=$HOME/mc-admin-cli
SPEC_WARNING="false"

# ──────────────
# System Requirement Check (Non-interactive)
# ──────────────
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📋 MC-Admin-CLI Setup Prerequisites"

echo
echo "✅ Recommended:"
echo "   - OS: Ubuntu 22.04 (Jammy)"
echo "   - vCPU: 4 or more"
echo "   - RAM: 8 GiB or more"
echo "   - Example: AWS c5a.xlarge or larger"
echo
echo "🧪 Checking your system..."

VCPU=$(nproc)
MEM_GB=$(free -g | awk '/^Mem:/{print $2}')

echo "   → Detected: $VCPU vCPU, $MEM_GB GiB memory"

if [ "$VCPU" -lt 4 ] || [ "$MEM_GB" -lt 8 ]; then
  SPEC_WARNING="true"
  echo
  echo "⚠️  WARNING: Your system does not meet the recommended minimum spec (4 vCPU, 8 GiB RAM)"
  echo "   → Proceeding anyway (non-interactive mode)"
else
  echo "✅ Spec check passed."
fi

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# ──────────────
# Detect actual user
# ──────────────
TARGET_USER="${SUDO_USER:-$USER}"
echo
echo "👤 Target user: $TARGET_USER"

# ──────────────
# Install prerequisites
# ──────────────
echo
echo "📦 Installing prerequisites..."
sudo apt update
sudo apt install -y curl git

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
# Enable Docker service
# ──────────────
echo
if command -v systemctl &> /dev/null; then
  echo "🔧 Enabling Docker service..."
  sudo systemctl enable --now docker || true
fi

# ──────────────
# Add user to docker group
# ──────────────
echo
if groups $TARGET_USER | grep -q '\bdocker\b'; then
  echo "✅ User '$TARGET_USER' already in 'docker' group."
else
  echo "👥 Adding user '$TARGET_USER' to docker group..."
  sudo groupadd docker 2>/dev/null || true
  sudo usermod -aG docker $TARGET_USER
fi

# ──────────────
# Test Docker with sudo
# ──────────────
echo
echo "🐳 Testing Docker (using sudo, group membership not required yet):"
sudo docker ps || true

# ──────────────
# Clone mc-admin-cli
# ──────────────
echo
if [ -d "$MCMP_DIR" ]; then
  echo "✅ mc-admin-cli already cloned at $MCMP_DIR. Skipping clone."
else
  echo "📥 Cloning mc-admin-cli..."
  git clone https://github.com/m-cmp/mc-admin-cli.git "$MCMP_DIR"
fi

# ──────────────
# Install mc-admin-cli
# ──────────────
echo
echo "🚀 Installing mc-admin-cli (mode=dev, background)..."
cd "$MCMP_DIR/bin"
./installAll.sh --mode dev --run background

# ──────────────
# Register aliases
# ──────────────
echo
if ! grep -q "alias cdmcmp=" ~/.bashrc; then
  echo "🔗 Registering aliases in ~/.bashrc..."
  {
    echo "alias cdmcmp='cd $MCMP_DIR'"
    echo "alias cdmcmpbin='cd $MCMP_DIR/bin'"
  } >> ~/.bashrc
  echo "ℹ️  Aliases added to ~/.bashrc"
  echo "   → To use them now, run: source ~/.bashrc"
fi

# ──────────────
# Done
# ──────────────
echo
echo "✅ MC-Admin-CLI setup is complete."

echo
echo "📁 Your mc-admin-cli directory:"
echo "   $MCMP_DIR"

echo
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📌 NEXT STEPS"
echo
echo "👉 1. Access the Web Console:"
echo
echo "   URL: http://<Your-VM-IP>:3001"
echo "   Username: mcmp"
echo "   Password: mcmp_password"
echo
echo "👉 2. Verify infrastructure status:"
echo
echo "   cd $MCMP_DIR/bin"
echo "   ./mcc infra info"
echo
echo "   ⚠️  IMPORTANT:"
echo "   - There are many Docker images and containers involved."
echo "   - Even after 3 minutes, not all containers may be fully healthy."
echo
echo "👉 3. If some containers are not healthy or you want to restart:"
echo
echo "   ./mcc infra stop    # stop all infra containers"
echo "   ./mcc infra run     # start infra containers again"
echo "   ./mcc infra info    # re-check status"
echo
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# ──────────────
# Final Notes
# ──────────────
if [ "$SPEC_WARNING" = "true" ]; then
  echo
  echo "⚠️  Final Warning: Your system is running below recommended spec (4 vCPU, 8 GiB RAM)."
  echo "   mc-admin-cli may not perform optimally under load."
fi

echo
echo "🚪 To complete the setup:"
echo "   → Please log out and log back in (or open a new SSH session) to:"
echo "     - Apply Docker group permissions (to avoid using sudo)"
echo "     - Activate shell aliases like 'cdmcmp'"
echo
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📦 OPTIONAL: Install CB-Tumblebug"
echo
echo "   MC-Admin-CLI works with CB-Tumblebug for multi-cloud orchestration."
echo "   To install CB-Tumblebug, run:"
echo
echo "   curl -sSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/set-tb.sh | bash"
echo
echo "   After installation, initialize CB-Tumblebug with:"
echo
echo "   cd ~/go/src/github.com/cloud-barista/cb-tumblebug"
echo "   make init"
echo
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo
