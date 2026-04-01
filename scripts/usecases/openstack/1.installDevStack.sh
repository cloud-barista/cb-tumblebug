#!/bin/bash

# DevStack Installation Script for AWS m5.metal (or bare-metal) Instances
# Installs a single-node OpenStack (DevStack) environment for CB-Tumblebug integration testing.
#
# Prerequisites:
#   - Ubuntu 22.04 or 24.04 on a bare-metal instance (e.g., AWS m5.metal)
#   - At least 8 GiB RAM, 50 GiB disk (m5.metal far exceeds this)
#   - Internet access for package downloads
#
# Usage:
#   ./1.installDevStack.sh [--csp-name CSP_NAME] [--password ADMIN_PASSWORD] [--branch OPENSTACK_BRANCH] [--latitude LAT] [--longitude LON] [--location DISPLAY]
#
# Parameters:
#   --csp-name  CSP provider name for CB-Tumblebug registration (default: openstack-devstack)
#   --password  OpenStack admin password (default: cbtumblebug)
#   --branch    DevStack branch to install (default: stable/2025.2)
#   --latitude  Latitude for location info (default: 0)
#   --longitude Longitude for location info (default: 0)
#   --location  Display name for location (default: DevStack)
#
# After completion, run ./2.getRegistrationInfo.sh to get CB-Tumblebug registration details.

set -e

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
# Parse arguments
# ============================================================
ADMIN_PASSWORD="cbtumblebug"
OPENSTACK_BRANCH="stable/2025.2"
CSP_NAME="openstack-devstack"
LOCATION_LATITUDE="0"
LOCATION_LONGITUDE="0"
LOCATION_DISPLAY="DevStack"

while [[ "$1" != "" ]]; do
    case $1 in
        --csp-name )  shift; CSP_NAME=$1 ;;
        --password )  shift; ADMIN_PASSWORD=$1 ;;
        --branch )    shift; OPENSTACK_BRANCH=$1 ;;
        --latitude )  shift; LOCATION_LATITUDE=$1 ;;
        --longitude ) shift; LOCATION_LONGITUDE=$1 ;;
        --location )  shift; LOCATION_DISPLAY=$1 ;;
        * )           echo "Usage: $0 [--csp-name CSP_NAME] [--password ADMIN_PASSWORD] [--branch OPENSTACK_BRANCH] [--latitude LAT] [--longitude LON] [--location DISPLAY]"; exit 1 ;;
    esac
    shift
done

echo "============================================================"
echo " DevStack Installation for CB-Tumblebug Integration"
echo "============================================================"
echo " CSP Name       : $CSP_NAME"
echo " Admin Password : $ADMIN_PASSWORD"
echo " Branch         : $OPENSTACK_BRANCH"
echo "============================================================"

# ============================================================
# Wait for apt locks (in case cloud-init is still running)
# ============================================================
wait_for_apt() {
    local max_wait=120
    local waited=0
    while sudo fuser /var/lib/dpkg/lock-frontend >/dev/null 2>&1 || \
          sudo fuser /var/lib/apt/lists/lock >/dev/null 2>&1; do
        if [ $waited -ge $max_wait ]; then
            echo "Warning: apt lock still held after ${max_wait}s, proceeding..."
            break
        fi
        echo "Waiting for apt lock... ($waited/${max_wait}s)"
        sleep 5
        waited=$((waited + 5))
    done
}

wait_for_apt

# ============================================================
# Retry helper for transient network failures
# ============================================================
retry() {
    local max_attempts=3
    local delay=15
    local attempt=1
    while [ $attempt -le $max_attempts ]; do
        if "$@"; then
            return 0
        fi
        echo "Attempt $attempt/$max_attempts failed. Retrying in ${delay}s..."
        sleep $delay
        attempt=$((attempt + 1))
    done
    echo "ERROR: Command failed after $max_attempts attempts: $*"
    return 1
}

# ============================================================
# Pre-flight checks
# ============================================================
echo ""
echo "[Pre-flight] Checking system requirements..."

# Check available disk space (minimum 50 GiB recommended for DevStack)
MIN_DISK_GB=50
AVAILABLE_GB=$(df --output=avail / | tail -1 | awk '{printf "%d", $1/1024/1024}')
echo "  Disk available: ${AVAILABLE_GB} GiB (minimum: ${MIN_DISK_GB} GiB)"
if [ "$AVAILABLE_GB" -lt "$MIN_DISK_GB" ]; then
    echo "ERROR: Insufficient disk space. DevStack requires at least ${MIN_DISK_GB} GiB free."
    echo "       Current available: ${AVAILABLE_GB} GiB on /"
    echo "       Consider using a larger root volume (100 GiB+ recommended)."
    exit 1
fi

# Check available memory (minimum 8 GiB recommended)
MIN_MEM_GB=8
AVAILABLE_MEM_GB=$(free -g | awk '/^Mem:/{print $2}')
echo "  Memory total  : ${AVAILABLE_MEM_GB} GiB (minimum: ${MIN_MEM_GB} GiB)"
if [ "$AVAILABLE_MEM_GB" -lt "$MIN_MEM_GB" ]; then
    echo "ERROR: Insufficient memory. DevStack requires at least ${MIN_MEM_GB} GiB RAM."
    echo "       Current total: ${AVAILABLE_MEM_GB} GiB"
    exit 1
fi

# Check OS version
OS_ID=$(. /etc/os-release && echo "$ID")
OS_VERSION=$(. /etc/os-release && echo "$VERSION_ID")
echo "  OS            : ${OS_ID} ${OS_VERSION}"
if [[ "$OS_ID" != "ubuntu" ]] || [[ "$OS_VERSION" != "22.04" && "$OS_VERSION" != "24.04" ]]; then
    echo "WARNING: DevStack is tested on Ubuntu 22.04/24.04. Current: ${OS_ID} ${OS_VERSION}"
    echo "         Proceeding, but issues may occur."
fi

echo "  All pre-flight checks passed."

# ============================================================
# Step 1: System preparation
# ============================================================
echo ""
echo "[1/5] Updating system packages..."
retry sudo apt-get update -qq
retry sudo DEBIAN_FRONTEND=noninteractive apt-get upgrade -y -qq \
    -o Dpkg::Options::="--force-confdef" \
    -o Dpkg::Options::="--force-confold"

echo "Installing prerequisites..."
retry sudo apt-get install -y -qq git python3-pip python3-venv net-tools curl jq

# ============================================================
# Step 2: Create stack user (DevStack requirement)
# ============================================================
echo ""
echo "[2/5] Setting up 'stack' user..."

if ! id "stack" &>/dev/null; then
    sudo useradd -s /bin/bash -d /opt/stack -m stack
    sudo chmod +x /opt/stack
fi

# Grant passwordless sudo
echo "stack ALL=(ALL) NOPASSWD: ALL" | sudo tee /etc/sudoers.d/stack > /dev/null

# ============================================================
# Step 3: Clone DevStack
# ============================================================
echo ""
echo "[3/5] Cloning DevStack ($OPENSTACK_BRANCH)..."

# Pre-configure git for the stack user to handle large repos over GnuTLS reliably.
# - http.version HTTP/1.1: avoids GnuTLS TLS-layer errors that occur with HTTP/2 GOAWAY
#   frames on large repo clones (the "TLS connection was non-properly terminated" error).
# - http.postBuffer: increases curl send buffer to 500 MiB for large pack transfers.
# - http.lowSpeedLimit/Time: prevents git from aborting slow-but-progressing clones.
sudo -u stack git config --global http.version HTTP/1.1
sudo -u stack git config --global http.postBuffer 524288000
sudo -u stack git config --global http.lowSpeedLimit 1000
sudo -u stack git config --global http.lowSpeedTime 60

retry sudo -u stack bash -c "OPENSTACK_BRANCH='$OPENSTACK_BRANCH'
    cd /opt/stack
    if [ -d devstack ]; then
        echo 'DevStack directory exists, pulling latest...'
        cd devstack && git checkout \"\$OPENSTACK_BRANCH\" && git pull
    else
        git clone https://opendev.org/openstack/devstack -b \"\$OPENSTACK_BRANCH\"
    fi
"

# ============================================================
# Step 4: Configure DevStack (local.conf)
# ============================================================
echo ""
echo "[4/5] Generating local.conf..."

# Detect host IP (private IP for AWS)
HOST_IP=$(hostname -I | awk '{print $1}')
# Detect public IP for external access
PUBLIC_IP=$(curl -s --connect-timeout 5 http://169.254.169.254/latest/meta-data/public-ipv4 2>/dev/null || \
            curl -s --connect-timeout 5 https://api.ipify.org 2>/dev/null || \
            echo "$HOST_IP")

sudo -u stack bash -c "cat > /opt/stack/devstack/local.conf << 'LOCALCONF'
[[local|localrc]]
# -------------------------------------------------------
# Credentials (use only alphanumeric characters)
# -------------------------------------------------------
ADMIN_PASSWORD=${ADMIN_PASSWORD}
DATABASE_PASSWORD=\${ADMIN_PASSWORD}
RABBIT_PASSWORD=\${ADMIN_PASSWORD}
SERVICE_PASSWORD=\${ADMIN_PASSWORD}

# -------------------------------------------------------
# Host configuration
# -------------------------------------------------------
HOST_IP=${HOST_IP}

# Note: Do NOT set SERVICE_HOST to Public IP on AWS.
# AWS public IPs are NAT'd and not bound to local interfaces,
# so services (e.g., etcd) cannot bind to them.
# The registration script (2.getRegistrationInfo.sh) handles
# rewriting internal IPs to public IPs in the output snippets.

# Always re-clone source repos (safe for re-installs after clean.sh)
RECLONE=yes

# -------------------------------------------------------
# Disable Tempest (test framework, not needed for operation)
# -------------------------------------------------------
disable_service tempest

# -------------------------------------------------------
# Hypervisor - use KVM on bare-metal
# -------------------------------------------------------
LIBVIRT_TYPE=kvm

# -------------------------------------------------------
# Guest image - Ubuntu cloud image for VM provisioning
# -------------------------------------------------------
IMAGE_URLS=\"https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-amd64.img\"

# -------------------------------------------------------
# Logging
# -------------------------------------------------------
LOGFILE=/opt/stack/logs/stack.sh.log
LOG_COLOR=False
LOGDAYS=1

# -------------------------------------------------------
# Cinder volume size
# -------------------------------------------------------
VOLUME_BACKING_FILE_SIZE=50G

# -------------------------------------------------------
# Octavia (Load Balancer) - required by CB-Spider NLBClient
# gophercloud v2 service catalog type: "load-balancer"
# -------------------------------------------------------
enable_plugin octavia https://opendev.org/openstack/octavia ${OPENSTACK_BRANCH}

# -------------------------------------------------------
# Manila (Shared File System) - required by CB-Spider SharedFileSystemClient
# gophercloud v2 service catalog type: "shared-file-system" (alias: "sharev2")
# -------------------------------------------------------
enable_plugin manila https://opendev.org/openstack/manila ${OPENSTACK_BRANCH}
LOCALCONF
"

echo "Generated local.conf with HOST_IP=$HOST_IP"

# ============================================================
# Step 5: Run DevStack installation
# ============================================================
echo ""
echo "[5/5] Running stack.sh (this takes 20-40 minutes with Octavia/Manila)..."
echo "      Logs: /opt/stack/logs/stack.sh.log"

# Temporarily disable 'exit on error' so we can capture stack.sh's exit code
set +e
sudo -u stack bash -c '
    cd /opt/stack/devstack && ./stack.sh
'
STACK_EXIT=$?
# Re-enable 'exit on error' for the remainder of the script
set -e

echo ""
echo "============================================================"
if [ $STACK_EXIT -eq 0 ]; then
    echo " DevStack installation COMPLETED successfully!"
    echo "============================================================"

    # ----------------------------------------------------------
    # Gather registration info for CB-Tumblebug
    # ----------------------------------------------------------
    source /opt/stack/devstack/openrc admin admin 2>/dev/null
    PROJECT_ID=$(openstack project show admin -f value -c id 2>/dev/null || echo "UNKNOWN")
    REGION=$(openstack region list -f value -c Region 2>/dev/null | head -1 || echo "RegionOne")
    AZ=$(openstack availability zone list --compute -f value -c "Zone Name" 2>/dev/null | grep -v "^internal$" | head -1 || echo "nova")

    # Update all service catalog endpoints to use Public IP
    # so external clients (CB-Spider) can reach them.
    INTERNAL_IP="$HOST_IP"
    CHANGED=0
    for eid in $(openstack endpoint list -f value -c ID 2>/dev/null); do
        eurl=$(openstack endpoint show "$eid" -f value -c url 2>/dev/null)
        if echo "$eurl" | grep -q "$INTERNAL_IP"; then
            new_url=$(echo "$eurl" | sed "s/$INTERNAL_IP/$PUBLIC_IP/g")
            openstack endpoint set --url "$new_url" "$eid" 2>/dev/null
            CHANGED=$((CHANGED + 1))
        fi
    done
    if [ $CHANGED -gt 0 ]; then
        echo ""
        echo " Updated $CHANGED service catalog endpoint(s): $INTERNAL_IP -> $PUBLIC_IP"
    fi

    # ----------------------------------------------------------
    # Verify CB-Spider required services in service catalog
    # CB-Spider's OpenStack driver (gophercloud v2) requires
    # these service types during connection initialization:
    #   NewLoadBalancerV2()      -> type "load-balancer"
    #   NewBlockStorageV3()      -> type "block-storage" (aliases: "volumev3", "volumev2", "volume")
    #   NewSharedFileSystemV2()  -> type "shared-file-system" (aliases: "sharev2", "share")
    #
    # gophercloud v2 uses ServiceTypeAliases, so:
    #   - Cinder "block-storage" is matched directly (no alias entry needed)
    #   - Manila "sharev2" or "shared-file-system" both work
    #   - Octavia/Manila are optional plugins; create placeholders if not installed
    # ----------------------------------------------------------
    echo ""
    echo " Verifying CB-Spider required services..."
    PLACEHOLDER_CREATED=0

    # Cinder (Block Storage) - gophercloud v2 type: "block-storage"
    # gophercloud v2 ServiceTypeAliases: "block-storage" -> ["volumev3", "volumev2", "volume", "block-store"]
    # No alias entry needed; "block-storage" is matched directly.
    if openstack service list -f value -c Type 2>/dev/null | grep -qE "^(block-storage|volumev3)$"; then
        echo "   ✓ block-storage (Cinder) - installed"
    else
        echo "   ✗ block-storage (Cinder) - NOT found"
        echo "     WARNING: Cinder is not available. Disk operations will fail."
    fi

    # Octavia (Load Balancer) - gophercloud v2 type: "load-balancer"
    if openstack service list -f value -c Type 2>/dev/null | grep -q "^load-balancer$"; then
        echo "   ✓ load-balancer (Octavia) - installed"
    else
        echo "   ✗ load-balancer (Octavia) - NOT found, creating placeholder..."
        openstack service create --name octavia --description "Load Balancer (placeholder for CB-Spider)" load-balancer && \
        openstack endpoint create --region "$REGION" load-balancer public "http://${PUBLIC_IP}/placeholder/load-balancer/v2.0" && \
        PLACEHOLDER_CREATED=$((PLACEHOLDER_CREATED + 1))
    fi

    # Manila (Shared File System) - gophercloud v2 type: "shared-file-system" (alias: "sharev2")
    if openstack service list -f value -c Type 2>/dev/null | grep -qE "^(shared-file-system|sharev2)$"; then
        echo "   ✓ shared-file-system (Manila) - installed"
    else
        echo "   ✗ shared-file-system (Manila) - NOT found, creating placeholder..."
        openstack service create --name manilav2 --description "Shared File System (placeholder for CB-Spider)" shared-file-system && \
        openstack endpoint create --region "$REGION" shared-file-system public "http://${PUBLIC_IP}/placeholder/shared-file-system/v2" && \
        PLACEHOLDER_CREATED=$((PLACEHOLDER_CREATED + 1))
    fi

    if [ $PLACEHOLDER_CREATED -gt 0 ]; then
        echo "   ⚠ Created $PLACEHOLDER_CREATED placeholder(s) - plugin install may have failed"
    fi

    echo ""
    echo " Horizon Dashboard  : http://${PUBLIC_IP}/dashboard"
    echo " Keystone Auth URL  : http://${PUBLIC_IP}/identity/v3"
    echo " Username / Password: admin / ${ADMIN_PASSWORD}"
    echo " Project ID         : ${PROJECT_ID}"
    echo " Region / AZ        : ${REGION} / ${AZ}"
    echo ""

    echo "============================================================"
    echo " credentials.yaml snippet"
    echo "============================================================"
    cat << CRED_EOF

    ${CSP_NAME}:
      IdentityEndpoint: http://${PUBLIC_IP}/identity/v3
      Username: admin
      Password: ${ADMIN_PASSWORD}
      DomainName: Default
      ProjectID: ${PROJECT_ID}

CRED_EOF

    echo "============================================================"
    echo " cloudinfo.yaml snippet"
    echo "============================================================"
    cat << CLOUD_EOF

  ${CSP_NAME}:
    description: DevStack (${PUBLIC_IP})
    cloudPlatform: openstack
    driver: openstack-driver-v1.0.so
    region:
      ${REGION}:
        id: ${REGION}
        description: DevStack ${REGION}
        location:
          display: ${LOCATION_DISPLAY}
          latitude: ${LOCATION_LATITUDE}
          longitude: ${LOCATION_LONGITUDE}
        zone:
        - ${AZ}

CLOUD_EOF

    echo "============================================================"
    echo " Next Steps"
    echo "============================================================"
    echo ""
    echo " 1. Add the snippets above to:"
    echo "    - ~/.cloud-barista/credentials.yaml  (credentials)"
    echo "    - cb-tumblebug/assets/cloudinfo.yaml  (cloud info)"
    echo ""
    echo " 2. Re-initialize CB-Tumblebug:"
    echo "    make enc-cred && make init"
    echo ""
    echo " For detailed info, run: ./2.getRegistrationInfo.sh"
else
    echo " DevStack installation FAILED (exit code: $STACK_EXIT)"
    echo " Check logs: /opt/stack/logs/stack.sh.log"
    echo "============================================================"
    exit 1
fi
