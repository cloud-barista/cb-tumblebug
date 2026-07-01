#!/bin/bash
set -e

# ══════════════════════════════════════════════════════════════════
#  set-mcmp.sh — M-CMP (mc-admin-cli) install & operation script
#
#  Two-stage workflow:
#    Stage 1:  bash set-mcmp.sh install   ← clone, configure env/certs
#    Stage 2:  bash set-mcmp.sh run       ← start containers (detached)
#    Status:   bash set-mcmp.sh info      ← show container status
#
#  All actions:
#    install   Clone repo, configure (does NOT start containers)  [default]
#    run       Start all M-CMP containers in background
#    pull      Pre-pull Docker images without starting
#    info      Show container and image status
#    stop      Stop all M-CMP containers
#
#  Install options:
#    --mode dev|prod         IAM mode (default: dev)
#    --domain <IP|FQDN>      Public IP or domain for IAM HTTPS
#                            (default: localhost — plain HTTP)
# ══════════════════════════════════════════════════════════════════

# ──────────────────────────────────────────────
# Parse action + options
# ──────────────────────────────────────────────
ACTION="${1:-install}"
shift || true

MCMP_MODE="dev"
DOMAIN=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        -m|--mode)     MCMP_MODE="${2:?}"; shift 2 ;;
        -d|--domain)   DOMAIN="${2:?}"; shift 2 ;;
        -h|--help)
            cat <<HELP
Usage: set-mcmp.sh [ACTION] [OPTIONS]

Actions:
  install   Clone, configure env files, prepare  (default — does NOT start)
  run       Start all M-CMP containers in background
  pull      Pre-pull Docker images without starting
  info      Show container and image status
  stop      Stop all M-CMP containers

Options (for 'install'):
  -m, --mode dev|prod       IAM mode (default: dev)
  -d, --domain <IP|FQDN>    Public IP or domain for IAM HTTPS
                            (default: localhost — plain HTTP)

Two-stage example:
  bash set-mcmp.sh install
  bash set-mcmp.sh run
  bash set-mcmp.sh info
HELP
            exit 0 ;;
        *) echo "❌ Unknown option: $1. Use --help for usage."; exit 1 ;;
    esac
done

# Validate action
case "$ACTION" in
    install|run|pull|info|stop) ;;
    *)
        echo "❌ Unknown action: '$ACTION'"
        echo "   Valid: install | run | pull | info | stop"
        exit 1 ;;
esac

# ──────────────────────────────────────────────
# Common: target user / paths
# ──────────────────────────────────────────────
TARGET_USER="${SUDO_USER:-$USER}"
TARGET_HOME=$(eval echo ~"$TARGET_USER")
MCMP_DIR="$TARGET_HOME/mc-admin-cli"
MCC="$MCMP_DIR/bin/mcc"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📋 M-CMP (mc-admin-cli) — Action: $ACTION"
echo "   User: $TARGET_USER   Home: $TARGET_HOME"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# ══════════════════════════════════════════════
#  Non-install actions (run / pull / info / stop)
# ══════════════════════════════════════════════
if [ "$ACTION" != "install" ]; then
    if [ ! -f "$MCC" ]; then
        echo "❌ mc-admin-cli not found at $MCMP_DIR."
        echo "   Run 'bash set-mcmp.sh install' first."
        exit 1
    fi

    # Detect whether docker requires sudo
    SUDO_PREFIX=""
    if ! sudo -u "$TARGET_USER" docker ps &>/dev/null 2>&1; then
        if sudo docker ps &>/dev/null 2>&1; then
            SUDO_PREFIX="sudo"
        else
            echo "❌ Docker is not accessible. Check Docker installation."
            exit 1
        fi
    fi

    cd "$MCMP_DIR/bin"

    case "$ACTION" in
        run)
            echo ""
            echo "🚀 Starting M-CMP containers (detached)..."
            echo "   This pulls any missing images and starts all containers."
            echo "   Use 'bash set-mcmp.sh info' to monitor status."
            echo ""
            $SUDO_PREFIX ./mcc infra run --detach
            echo ""
            echo "✅ M-CMP containers are starting in the background."
            echo ""
            echo "   ⚠️  Many containers are involved — it may take several minutes"
            echo "      for all services to become healthy."
            echo "   → Run 'bash set-mcmp.sh info' to check status."
            echo ""
            echo "\$\$ENDPOINT[M-CMP Web Console](http://0.0.0.0:3001)"
            echo "\$\$CREDENTIAL[M-CMP login](mcmp / mcmp_password)"
            ;;
        pull)
            echo ""
            echo "📥 Pulling M-CMP Docker images..."
            echo "   This may take a while for the initial pull."
            $SUDO_PREFIX ./mcc infra pull
            echo ""
            echo "✅ Image pull completed."
            echo "   → Run 'bash set-mcmp.sh run' to start containers."
            ;;
        info)
            echo ""
            $SUDO_PREFIX ./mcc infra info
            echo ""
            echo "\$\$ENDPOINT[M-CMP Web Console](http://0.0.0.0:3001)"
            echo "\$\$CREDENTIAL[M-CMP login](mcmp / mcmp_password)"
            ;;
        stop)
            echo ""
            echo "🛑 Stopping M-CMP containers..."
            $SUDO_PREFIX ./mcc infra stop
            echo ""
            echo "✅ Containers stopped."
            echo "   → Run 'bash set-mcmp.sh run' to restart."
            ;;
    esac
    exit 0
fi

# ══════════════════════════════════════════════
#  INSTALL action
# ══════════════════════════════════════════════
MIN_VCPU=4
MIN_RAM_GB=8
SPEC_WARNING="false"

# ── System requirement check ──
echo ""
echo "🧪 Checking system requirements..."
echo "   Recommended: ${MIN_VCPU} vCPU / ${MIN_RAM_GB} GiB RAM (e.g. AWS c5a.xlarge)"

VCPU=$(nproc)
MEM_GB=$(free -g | awk '/^Mem:/{print $2}')
echo "   Detected:    ${VCPU} vCPU / ${MEM_GB} GiB RAM"

if [ "$VCPU" -lt "$MIN_VCPU" ] || [ "$MEM_GB" -lt "$MIN_RAM_GB" ]; then
    SPEC_WARNING="true"
    echo "   ⚠️  Below recommended spec — proceeding anyway (non-interactive)"
else
    echo "   ✅ Spec check passed"
fi

# ── Prerequisites ──
echo ""
echo "📦 Installing prerequisites (curl, git)..."
sudo apt-get update -qq
sudo apt-get install -y curl git

# ── Docker ──
echo ""
if ! command -v docker &>/dev/null; then
    echo "🐳 Installing Docker..."
    echo "   (See https://docs.docker.com/engine/install/ for manual install)"
    TMP=$(mktemp)
    curl -fsSL https://get.docker.com -o "$TMP"
    sh "$TMP"
    rm -f "$TMP"
else
    echo "✅ Docker already installed."
fi

if command -v systemctl &>/dev/null; then
    sudo systemctl enable --now docker 2>/dev/null || true
fi

# ── Docker group ──
echo ""
if groups "$TARGET_USER" | grep -q '\bdocker\b'; then
    echo "✅ User '$TARGET_USER' already in docker group."
else
    echo "👥 Adding '$TARGET_USER' to docker group..."
    sudo groupadd docker 2>/dev/null || true
    sudo usermod -aG docker "$TARGET_USER"
    echo "   → Note: log out and back in to activate group membership"
fi

# ── Docker access ──
echo ""
echo "🐳 Testing Docker access..."
DOCKER_NEEDS_SUDO="false"
if sudo -u "$TARGET_USER" docker ps &>/dev/null 2>&1; then
    echo "✅ Docker access confirmed (no sudo required)"
elif sudo docker ps &>/dev/null 2>&1; then
    echo "⚠️  Docker requires sudo (group not yet active — normal for first-time setup)"
    DOCKER_NEEDS_SUDO="true"
else
    echo "❌ Docker is not accessible. Please check Docker installation."
    exit 1
fi

# ── Clone or update repository ──
echo ""
if [ -d "$MCMP_DIR/.git" ]; then
    echo "🔄 mc-admin-cli already cloned. Pulling latest changes..."
    git -C "$MCMP_DIR" pull --ff-only || git -C "$MCMP_DIR" fetch
else
    echo "📥 Cloning mc-admin-cli..."
    git clone https://github.com/m-cmp/mc-admin-cli.git "$MCMP_DIR"
fi

# ── Configure: installAll.sh --run skip (configure env/certs, do NOT start) ──
echo ""
echo "⚙️  Configuring mc-admin-cli..."
echo "   Mode: $MCMP_MODE   Domain: ${DOMAIN:-localhost (HTTP, no certs)}"
echo "   Run mode: skip (containers will NOT be started)"

cd "$MCMP_DIR/bin"

INSTALL_ARGS="--mode $MCMP_MODE --run skip"
[ -n "$DOMAIN" ] && INSTALL_ARGS="$INSTALL_ARGS --domain $DOMAIN"

if [ "$DOCKER_NEEDS_SUDO" = "true" ]; then
    echo "   → Running installAll.sh with sudo (docker group not yet active)..."
    sudo ./installAll.sh $INSTALL_ARGS
    echo "   → Fixing file ownership in $MCMP_DIR..."
    sudo chown -R "$TARGET_USER":"$TARGET_USER" "$MCMP_DIR"
else
    ./installAll.sh $INSTALL_ARGS
fi

# ── Register shell aliases ──
echo ""
TARGET_BASHRC="$TARGET_HOME/.bashrc"
if ! grep -q "alias cdmcmp=" "$TARGET_BASHRC" 2>/dev/null; then
    echo "🔗 Registering shell aliases in $TARGET_BASHRC..."
    {
        echo "alias cdmcmp='cd $MCMP_DIR'"
        echo "alias cdmcmpbin='cd $MCMP_DIR/bin'"
    } >> "$TARGET_BASHRC"
    echo "   → Aliases added (active after: source $TARGET_BASHRC)"
fi

# ── Done ──
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ M-CMP install (Stage 1) complete."
echo "   Directory: $MCMP_DIR"
echo ""
echo "📌 NEXT STEPS"
echo ""
echo "   [Stage 2] Pre-pull Docker images (optional, recommended):"
echo "      bash set-mcmp.sh pull"
echo ""
echo "   [Stage 2] Start all M-CMP containers:"
echo "      bash set-mcmp.sh run"
echo ""
echo "   [Status]  Check container health:"
echo "      bash set-mcmp.sh info"
echo ""
echo "   Web Console: http://<Your-VM-IP>:3001"
echo "   Username: mcmp   Password: mcmp_password"
echo ""
if [ "$DOCKER_NEEDS_SUDO" = "true" ]; then
    echo "   ⚠️  Log out and back in to activate docker group"
    echo "      (avoids needing sudo for docker commands)"
    echo ""
fi
if [ "$SPEC_WARNING" = "true" ]; then
    echo "   ⚠️  System is below recommended spec (${MIN_VCPU} vCPU / ${MIN_RAM_GB} GiB)."
    echo "      M-CMP may not perform optimally under load."
    echo ""
fi
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "   Other operations:"
echo "      bash set-mcmp.sh stop     # stop all containers"
echo "      bash set-mcmp.sh run      # restart containers"
echo "      bash set-mcmp.sh info     # check status"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📦 OPTIONAL: Install CB-Tumblebug"
echo ""
echo "   # Quick install"
echo "   curl -sSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/set-tb.sh | bash"
echo ""
echo "   # After install, initialize:"
echo "   cd ~/go/src/github.com/cloud-barista/cb-tumblebug && make init"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
