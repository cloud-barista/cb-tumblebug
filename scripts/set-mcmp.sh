#!/bin/bash
set -e

# ══════════════════════════════════════════════════════════════════
#  set-mcmp.sh — M-CMP (mc-admin-cli) install & operation script
#
#  Multi-stage workflow (optimized for CB-Tumblebug / CB-MapUI SSH):
#    Stage 1:  bash set-mcmp.sh install   ← clone, configure env/certs
#    Stage 2:  bash set-mcmp.sh pull      ← pre-pull Docker images quietly
#    Stage 3:  bash set-mcmp.sh run       ← start containers in waves
#    Status:   bash set-mcmp.sh info      ← show container status
#
#  All actions:
#    install   Clone repo, configure (does NOT start containers)  [default]
#    pull      Pre-pull Docker images without starting
#    run       Start all M-CMP containers in staged waves (detached)
#    info      Show container and image status
#    stop      Stop all M-CMP containers
#
#  Install options:
#    --mode dev|prod         IAM mode (default: dev)
#    --domain <IP|FQDN>      Public IP or domain for IAM HTTPS
#                            (default: localhost — plain HTTP;
#                             for CB-TB remote cmd: --domain $$Func(GetPublicIP(target=this)))
#
#  Required Security Group Inbound TCP Ports (for remote VM access):
#    - Web Console & IAM : 80, 443, 3000, 3001
#    - Service Iframes   : 3300 (Data), 7780/9000/9090 (Cost),
#                          18081/33002 (Observability/Grafana),
#                          18083 (Workflow), 18084 (App/SW Catalog)
#    - Infra & MapUI     : 1024 (Spider), 1323 (Tumblebug), 1324 (MapUI)
#    - Agent Telemetry   : 1883, 3100, 4317, 4318, 5672, 8086, 8087, 18080
# ══════════════════════════════════════════════════════════════════

# ──────────────────────────────────────────────
# Parse action + options
# ──────────────────────────────────────────────
ACTION="${1:-install}"
shift || true

MCMP_MODE="dev"
DOMAIN="localhost"

while [[ $# -gt 0 ]]; do
    case "$1" in
        -m|--mode)     MCMP_MODE="${2:?}"; shift 2 ;;
        -d|--domain)   DOMAIN="${2:?}"; shift 2 ;;
        -h|--help)
            cat <<HELP
Usage: set-mcmp.sh [ACTION] [OPTIONS]

Actions:
  install   Clone, configure env files, prepare  (default — does NOT start)
  pull      Pre-pull Docker images without starting
  run       Start all M-CMP containers in staged waves (detached)
  info      Show container and image status
  stop      Stop all M-CMP containers

Options (for 'install'):
  -m, --mode dev|prod       IAM mode (default: dev)
  -d, --domain <IP|FQDN>    Public IP or domain for IAM:
                              localhost  — plain HTTP [default]
                              <IP|FQDN>  — public IP or domain (HTTPS)
                              (CB-TB remote command: --domain \$\$Func(GetPublicIP(target=this)))

Required Security Group Inbound TCP Ports:
  - Web Console & IAM : 80, 443, 3000, 3001
  - Service Iframes   : 3300, 7780, 9000, 9090, 18081, 18083, 18084, 33002
  - Infra & MapUI     : 1024, 1323, 1324
  - Agent Telemetry   : 1883, 3100, 4317, 4318, 5672, 8086, 8087, 18080

Multi-stage example:
  bash set-mcmp.sh install --domain <VM_PUBLIC_IP>
  bash set-mcmp.sh pull
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
        echo "   Valid: install | pull | run | info | stop"
        exit 1 ;;
esac

# ──────────────────────────────────────────────
# Common: target user / paths / helpers
# ──────────────────────────────────────────────
TARGET_USER="${SUDO_USER:-$USER}"
TARGET_HOME=$(eval echo ~"$TARGET_USER")
MCMP_DIR="$TARGET_HOME/mc-admin-cli"
MCC="$MCMP_DIR/bin/mcc"
COMPOSE_FILE="$MCMP_DIR/conf/docker/docker-compose.yaml"
ENV_FILE="$MCMP_DIR/conf/docker/.env"

# Startup Waves (aligned with mc-admin-cli/bin/installAll.sh)
STARTUP_WAVES=(
    "mc-infra-connector mc-infra-manager mc-iam-manager-nginx mc-iam-manager mc-iam-manager-post-initial"
    "mc-data-manager mc-web-console-api mc-web-console-front"
    "mc-observability-manager mc-observability-front mc-observability-insight mc-observability-insight-scheduler mc-observability-mcp-grafana mc-observability-mcp-maria mc-observability-mcp-influx mc-observability-log-collector"
    "mc-application-manager mc-workflow-manager mc-cost-optimizer-fe"
)

get_configured_domain() {
    if [ -f "$ENV_FILE" ]; then
        grep -E '^MC_IAM_MANAGER_PUBLIC_DOMAIN=' "$ENV_FILE" | tail -n1 | cut -d'=' -f2- | tr -d '[:space:]'
    fi
}

print_sg_ports() {
    echo "🔒 Required Security Group Inbound TCP Ports:"
    echo "   • Web Console & IAM : 80, 443, 3000, 3001"
    echo "   • Service UIs/APIs  : 3300 (Data), 7780/9000/9090 (Cost), 18081/33002 (Obs/Grafana), 18083 (Workflow), 18084 (SW Catalog)"
    echo "   • Infra & MapUI     : 1024 (Spider), 1323 (Tumblebug), 1324 (MapUI)"
    echo "   • Agent Telemetry   : 1883, 3100, 4317, 4318, 5672, 8086, 8087, 18080 (when monitoring target VMs)"
}

print_access_markers() {
    local cfg_domain
    cfg_domain=$(get_configured_domain)
    cfg_domain="${cfg_domain:-localhost}"

    local scheme="https"
    case "$cfg_domain" in
        localhost|127.0.0.1|mciam.local) scheme="http" ;;
    esac

    echo ""
    print_sg_ports
    echo ""
    if [ "$scheme" = "https" ]; then
        echo "ℹ️  Remote HTTPS mode (self-signed certificate for $cfg_domain):"
        echo "   1. Click 'M-CMP IAM Auth' below first and accept the self-signed certificate warning in your browser."
        echo "   2. Then open 'M-CMP Web Console' and accept its certificate warning to log in."
        echo "   (Note: When opening sub-menus for the first time, if an iframe shows a cert error, open its port once to accept the cert.)"
        echo ""
        echo "\$\$ENDPOINT[M-CMP IAM Auth (1. Accept Cert)](https://0.0.0.0:443/auth/)"
        echo "\$\$ENDPOINT[M-CMP Web Console (2. Open Console)](https://0.0.0.0:3001)"
    else
        echo "\$\$ENDPOINT[M-CMP Web Console](http://0.0.0.0:3001)"
    fi
    echo "\$\$ENDPOINT[M-CMP Tumblebug MapUI](http://0.0.0.0:1324)"
    echo "\$\$CREDENTIAL[M-CMP login](mcmp / mcmp_password)"
}

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
            echo "🚀 Starting M-CMP containers in staged waves (detached)..."
            echo "   Use 'bash set-mcmp.sh info' to monitor status."

            wave_num=0
            for wave_services in "${STARTUP_WAVES[@]}"; do
                wave_num=$((wave_num + 1))
                echo ""
                echo "---- Wave ${wave_num}/${#STARTUP_WAVES[@]}: $wave_services ----"
                $SUDO_PREFIX ./mcc infra run -d -s "$wave_services"
            done

            echo ""
            echo "---- Starting remaining M-CMP services ----"
            $SUDO_PREFIX ./mcc infra run -d

            # Verify mc-iam-manager-post-initial completion
            pi_exit=$($SUDO_PREFIX docker inspect --format='{{.State.ExitCode}}' mc-iam-manager-post-initial 2>/dev/null || echo "1")
            if [ "$pi_exit" != "0" ]; then
                echo ""
                echo "🔄 Re-running mc-iam-manager-post-initial to ensure IAM initialization..."
                $SUDO_PREFIX ./mcc infra run -s mc-iam-manager-post-initial || true
            fi

            # Reclaim ownership if sudo was used
            if [ -n "$SUDO_PREFIX" ]; then
                sudo chown -R "$TARGET_USER":"$TARGET_USER" "$MCMP_DIR" 2>/dev/null || true
            fi

            echo ""
            echo "✅ All M-CMP startup waves initiated."
            echo "   → Run 'bash set-mcmp.sh info' to check container health."
            print_access_markers
            ;;
        pull)
            echo ""
            echo "📥 Pre-pulling M-CMP Docker images (quiet mode to prevent log flooding)..."
            echo "   This may take several minutes for ~60 images."
            if $SUDO_PREFIX docker compose --help 2>&1 | grep -q -- "--progress"; then
                $SUDO_PREFIX env COMPOSE_PROJECT_NAME=mcc docker compose -f "$COMPOSE_FILE" --progress quiet pull
            else
                $SUDO_PREFIX ./mcc infra pull
            fi
            echo ""
            echo "✅ Image pull completed."
            echo "   → Run 'bash set-mcmp.sh run' to start containers."
            ;;
        info)
            echo ""
            $SUDO_PREFIX ./mcc infra info
            print_access_markers
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
MIN_VCPU=8
MIN_RAM_GB=16
SPEC_WARNING="false"

# ── System requirement check ──
echo ""
echo "🧪 Checking system requirements..."
echo "   Recommended: ${MIN_VCPU}+ vCPU / 32 GiB RAM (min ${MIN_RAM_GB} GiB RAM, e.g. m6i.2xlarge)"

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
echo "📦 Installing prerequisites (curl, git, openssl, make)..."
sudo env DEBIAN_FRONTEND=noninteractive apt-get update -qq
sudo env DEBIAN_FRONTEND=noninteractive apt-get install -y -qq curl git openssl make

# ── Docker ──
echo ""
if ! command -v docker &>/dev/null; then
    echo "🐳 Installing Docker..."
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

# ── Clone or update CB-Tumblebug repository (required for 'make init') ──
CB_DIR="$TARGET_HOME/go/src/github.com/cloud-barista/cb-tumblebug"
echo ""
if [ -d "$CB_DIR/.git" ]; then
    echo "✅ cb-tumblebug already cloned at $CB_DIR."
else
    echo "📥 Cloning cb-tumblebug (for 'make init' credential & asset setup)..."
    sudo -u "$TARGET_USER" mkdir -p "$(dirname "$CB_DIR")"
    sudo -u "$TARGET_USER" git clone https://github.com/cloud-barista/cb-tumblebug.git "$CB_DIR"
fi

sudo -u "$TARGET_USER" mkdir -p "$TARGET_HOME/.cloud-barista"
if [ ! -f "$CB_DIR/.env" ] && [ -f "$CB_DIR/.env.example" ]; then
    echo "📋 Creating $CB_DIR/.env from .env.example with default credentials..."
    sudo -u "$TARGET_USER" cp "$CB_DIR/.env.example" "$CB_DIR/.env"
    sed -i 's/^TB_API_USERNAME=$/TB_API_USERNAME=default/' "$CB_DIR/.env"
    sed -i 's/^TB_API_PASSWORD=$/TB_API_PASSWORD=default/' "$CB_DIR/.env"
    sed -i 's/^SPIDER_USERNAME=$/SPIDER_USERNAME=default/' "$CB_DIR/.env"
    sed -i 's/^SPIDER_PASSWORD=$/SPIDER_PASSWORD=default/' "$CB_DIR/.env"
    sed -i 's/^TERRARIUM_API_USERNAME=$/TERRARIUM_API_USERNAME=default/' "$CB_DIR/.env"
    sed -i 's/^TERRARIUM_API_PASSWORD=$/TERRARIUM_API_PASSWORD=default/' "$CB_DIR/.env"
    sudo chown "$TARGET_USER":"$TARGET_USER" "$CB_DIR/.env"
fi

# Ensure 'uv' is installed for TARGET_USER (required by cb-tumblebug 'make init')
if ! sudo -u "$TARGET_USER" bash -lc 'export PATH="$HOME/.local/bin:$HOME/.cargo/bin:$PATH"; command -v uv' &>/dev/null; then
    echo "🧩 Installing Python package manager 'uv' (for 'make init')..."
    sudo -u "$TARGET_USER" bash -c 'curl -LsSf https://astral.sh/uv/install.sh | sh'
    if ! grep -q '\.local/bin' "$TARGET_HOME/.bashrc" 2>/dev/null; then
        echo 'export PATH="$HOME/.local/bin:$HOME/.cargo/bin:$PATH"' >> "$TARGET_HOME/.bashrc"
    fi
else
    echo "✅ 'uv' is already installed."
fi

# ── Clone or update mc-admin-cli repository ──
echo ""
if [ -d "$MCMP_DIR/.git" ]; then
    echo "🔄 mc-admin-cli already cloned. Pulling latest changes..."
    git -C "$MCMP_DIR" pull --ff-only || git -C "$MCMP_DIR" fetch
else
    echo "📥 Cloning mc-admin-cli..."
    git clone https://github.com/m-cmp/mc-admin-cli.git "$MCMP_DIR"
fi

# ── Configure: installAll.sh --run skip (configure env/certs, do NOT start) ──
DOMAIN="${DOMAIN:-localhost}"
echo ""
echo "⚙️  Configuring mc-admin-cli..."
echo "   Mode: $MCMP_MODE   Domain: $DOMAIN"
echo "   Run mode: skip (containers will NOT be started)"

cd "$MCMP_DIR/bin"

INSTALL_ARGS="--mode $MCMP_MODE --domain $DOMAIN --run skip"

if [ "$DOCKER_NEEDS_SUDO" = "true" ]; then
    echo "   → Running installAll.sh with sudo (docker group not yet active)..."
    sudo ./installAll.sh $INSTALL_ARGS
else
    ./installAll.sh $INSTALL_ARGS
fi

# ── Patch nginx.conf for Software Catalog (mc-application-manager) remote access ──
# mc-application-manager:0.7.0 bundles a hardcoded 'http://localhost:18084' Axios baseURL.
# Injecting sub_filter into the nginx proxy block rewrites it dynamically to the public host.
NGINX_CONF="$MCMP_DIR/conf/docker/container-volume/mc-iam-manager/nginx/nginx.conf"
if [ -f "$NGINX_CONF" ] && ! grep -q "sub_filter 'http://localhost:18084'" "$NGINX_CONF"; then
    echo "🔧 Patching nginx.conf with sub_filter for remote Software Catalog compatibility..."
    sudo sed -i '/set \$upstream_am mc-application-manager;/a \            proxy_set_header Accept-Encoding "";\n            sub_filter '\''http://localhost:18084'\'' '\''$scheme://$http_host'\'';\n            sub_filter_types text/javascript application/javascript;\n            sub_filter_once off;' "$NGINX_CONF"
fi

echo "   → Ensuring file ownership in $MCMP_DIR for $TARGET_USER..."
sudo chown -R "$TARGET_USER":"$TARGET_USER" "$MCMP_DIR"

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
if ! grep -q "alias cdtb=" "$TARGET_BASHRC" 2>/dev/null; then
    {
        echo "alias cdtb='cd $CB_DIR'"
        echo "alias cdtbsrc='cd $CB_DIR/src'"
        echo "alias cdtbtest='cd $CB_DIR/src/testclient/scripts'"
    } >> "$TARGET_BASHRC"
fi

# ── Done ──
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ M-CMP install (Stage 1) complete."
echo "   M-CMP Directory:        $MCMP_DIR"
echo "   CB-Tumblebug Directory: $CB_DIR"
echo "   Configured Domain:      $DOMAIN"
echo ""
echo "📌 NEXT STEPS"
echo ""
echo "   [Stage 2] Pre-pull Docker images (recommended):"
echo "      bash set-mcmp.sh pull"
echo ""
echo "   [Stage 3] Start all M-CMP containers:"
echo "      bash set-mcmp.sh run"
echo ""
echo "   [Status]  Check container health:"
echo "      bash set-mcmp.sh info"
echo ""
print_sg_ports
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
echo "📦 Initialize CSP Credentials & Cloud Assets (via CB-Tumblebug)"
echo ""
echo "   # CB-Tumblebug is cloned at: $CB_DIR"
echo "   # After M-CMP containers are running (mc-infra-manager on :1323):"
echo "   cd ~/go/src/github.com/cloud-barista/cb-tumblebug"
echo "   make gen-cred   # edit ~/.cloud-barista/credentials.yaml"
echo "   make enc-cred   # encrypt credentials.yaml"
echo "   make init       # register credentials and cloud assets into M-CMP"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

