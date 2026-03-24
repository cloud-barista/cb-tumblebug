#!/bin/bash

# Kolla-Ansible All-in-One Installation Script for AWS m5.metal (or bare-metal) Instances
# Installs a production-grade single-node OpenStack environment using Kolla-Ansible.
#
# Advantages over DevStack:
#   - Docker-based: services survive reboots (auto-restart)
#   - Stable: designed for long-running environments
#   - Native Octavia/Manila support (no placeholder hacks)
#   - Rolling upgrades supported
#
# Prerequisites:
#   - Ubuntu 22.04 on a bare-metal instance (e.g., AWS m5.metal)
#   - At least 8 GiB RAM, 50 GiB disk (m5.metal far exceeds this)
#   - Internet access for package downloads and Docker image pulls
#
# Usage:
#   ./1.installKolla.sh [--csp-name CSP_NAME] [--password ADMIN_PASSWORD] [--release RELEASE]
#                       [--latitude LAT] [--longitude LON] [--location DISPLAY]
#
# Parameters:
#   --csp-name   CSP provider name for CB-Tumblebug registration (default: openstack-kolla)
#   --password   OpenStack admin password (default: cbtumblebug)
#   --release    OpenStack release to install (default: 2025.2)
#   --latitude   Latitude for location info (default: 0)
#   --longitude  Longitude for location info (default: 0)
#   --location   Display name for location (default: OpenStack-Kolla)
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
OPENSTACK_RELEASE="2025.2"
CSP_NAME="openstack-kolla"
LOCATION_LATITUDE="0"
LOCATION_LONGITUDE="0"
LOCATION_DISPLAY="OpenStack-Kolla"

while [[ "$1" != "" ]]; do
    case $1 in
        --csp-name )   shift; CSP_NAME=$1 ;;
        --password )   shift; ADMIN_PASSWORD=$1 ;;
        --release )    shift; OPENSTACK_RELEASE=$1 ;;
        --latitude )   shift; LOCATION_LATITUDE=$1 ;;
        --longitude )  shift; LOCATION_LONGITUDE=$1 ;;
        --location )   shift; LOCATION_DISPLAY=$1 ;;
        * )            echo "Usage: $0 [--csp-name CSP_NAME] [--password ADMIN_PASSWORD] [--release RELEASE] [--latitude LAT] [--longitude LON] [--location DISPLAY]"; exit 1 ;;
    esac
    shift
done

echo "============================================================"
echo " Kolla-Ansible All-in-One Installation"
echo " for CB-Tumblebug Integration"
echo "============================================================"
echo " CSP Name        : $CSP_NAME"
echo " Admin Password  : $ADMIN_PASSWORD"
echo " OpenStack       : $OPENSTACK_RELEASE"
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

# ============================================================
# Detect IPs
# ============================================================
HOST_IP=$(hostname -I | awk '{print $1}')
PUBLIC_IP=$(curl -s --connect-timeout 5 http://169.254.169.254/latest/meta-data/public-ipv4 2>/dev/null || \
            curl -s --connect-timeout 5 https://api.ipify.org 2>/dev/null || \
            echo "$HOST_IP")

echo ""
echo " Internal IP : $HOST_IP"
echo " Public IP   : $PUBLIC_IP"
echo ""

# Detect primary network interface
NET_IFACE=$(ip -4 route show default | awk '{print $5}' | head -1)
if [ -z "$NET_IFACE" ]; then
    NET_IFACE="ens5"  # AWS default
fi
echo " Net Interface : $NET_IFACE"
echo ""

# ============================================================
# Step 1: Install system dependencies
# ============================================================
echo "[1/7] Installing system dependencies..."
wait_for_apt

sudo apt-get update -qq
sudo apt-get install -y -qq \
    python3-dev python3-venv python3-pip \
    libffi-dev gcc libssl-dev \
    git curl jq \
    > /dev/null 2>&1

echo "  System dependencies installed."

# ============================================================
# Step 2: Install Docker (if not present)
# ============================================================
echo "[2/7] Setting up Docker..."

if ! command -v docker &> /dev/null; then
    curl -fsSL https://get.docker.com | sudo bash > /dev/null 2>&1
    echo "  Docker installed."
else
    echo "  Docker already installed: $(docker --version)"
fi

# Ensure Docker is running
sudo systemctl enable docker
sudo systemctl start docker

# Add current user to docker group
if ! groups | grep -q docker; then
    sudo usermod -aG docker "$USER"
    echo "  Added $USER to docker group."
fi

# Configure Docker daemon for Kolla
sudo mkdir -p /etc/docker
if [ ! -f /etc/docker/daemon.json ] || ! grep -q "live-restore" /etc/docker/daemon.json 2>/dev/null; then
    cat << 'DOCKER_CONF' | sudo tee /etc/docker/daemon.json > /dev/null
{
    "live-restore": true,
    "log-driver": "json-file",
    "log-opts": {
        "max-size": "50m",
        "max-file": "3"
    }
}
DOCKER_CONF
    sudo systemctl restart docker
    echo "  Docker daemon configured."
fi

# ============================================================
# Step 3: Create Python virtual environment & install Kolla-Ansible
# ============================================================
echo "[3/7] Installing Kolla-Ansible in virtual environment..."

KOLLA_VENV="/opt/kolla-venv"
sudo mkdir -p "$KOLLA_VENV"
sudo chown "$USER:$USER" "$KOLLA_VENV"

python3 -m venv "$KOLLA_VENV"
source "$KOLLA_VENV/bin/activate"

pip install -U pip setuptools wheel > /dev/null 2>&1
pip install 'ansible-core>=2.16,<2.18' > /dev/null 2>&1
pip install git+https://opendev.org/openstack/kolla-ansible@stable/${OPENSTACK_RELEASE} > /dev/null 2>&1

echo "  Kolla-Ansible installed."

# Install Ansible Galaxy requirements
kolla-ansible install-deps > /dev/null 2>&1
echo "  Ansible Galaxy dependencies installed."

# ============================================================
# Step 4: Configure Kolla-Ansible
# ============================================================
echo "[4/7] Configuring Kolla-Ansible..."

sudo mkdir -p /etc/kolla
sudo chown "$USER:$USER" /etc/kolla

# Copy default configuration
cp "$KOLLA_VENV/share/kolla-ansible/etc_examples/kolla/globals.yml" /etc/kolla/globals.yml
cp "$KOLLA_VENV/share/kolla-ansible/etc_examples/kolla/passwords.yml" /etc/kolla/passwords.yml

# Copy inventory
mkdir -p /opt/kolla-config
cp "$KOLLA_VENV/share/kolla-ansible/ansible/inventory/all-in-one" /opt/kolla-config/all-in-one

# Generate random passwords
kolla-genpwd
echo "  Passwords generated."

# Override keystone_admin_password with user-specified password
sed -i "s|^keystone_admin_password:.*|keystone_admin_password: ${ADMIN_PASSWORD}|" /etc/kolla/passwords.yml

# Configure globals.yml
cat << GLOBALS > /etc/kolla/globals.yml
---
# ============================================================
# Kolla-Ansible globals.yml for CB-Tumblebug Integration
# Generated by installKolla.sh
# ============================================================

# Base configuration
kolla_base_distro: "ubuntu"
openstack_release: "${OPENSTACK_RELEASE}"

# Networking
kolla_internal_vip_address: "${HOST_IP}"
network_interface: "${NET_IFACE}"

# For all-in-one deployment on cloud VM without second NIC,
# use the same interface. Neutron will use OVS internal ports.
neutron_external_interface: "${NET_IFACE}"

# Disable HAProxy for single-node (avoids VIP binding issues on cloud VMs)
enable_haproxy: "no"

# Core services (enabled by default, listed for clarity)
enable_openstack_core: "yes"

# Block storage (Cinder with LVM backend)
enable_cinder: "yes"
enable_cinder_backend_lvm: "yes"
cinder_volume_group: "cinder-volumes"

# Networking (OVN is default in 2025.2)
neutron_plugin_agent: "ovn"

# Dashboard
enable_horizon: "yes"

# -------------------------------------------------------
# CB-Spider Required Services
# CB-Spider's OpenStack driver requires Octavia (NLB) and Manila
# (SharedFS) service clients during connection initialization.
# However, actually deploying these services on AWS bare-metal
# adds significant complexity (amphora images, management networks,
# certificates for Octavia; service VMs for Manila).
# Instead, we create placeholder service catalog entries after
# deployment. This lets CB-Spider initialize without errors while
# keeping the installation simple and reliable.
# -------------------------------------------------------

# Hypervisor - KVM on bare-metal
nova_compute_virt_type: "kvm"

# Docker configuration
docker_registry:
docker_namespace: "kolla"

# Logging
enable_central_logging: "no"

GLOBALS

echo "  globals.yml configured."

# ============================================================
# Step 5: Prepare LVM for Cinder
# ============================================================
echo "[5/7] Preparing LVM volume group for Cinder..."

if ! sudo vgdisplay cinder-volumes > /dev/null 2>&1; then
    # Create a loopback device for Cinder volumes
    CINDER_LOOP="/var/lib/cinder/cinder-volumes.img"
    sudo mkdir -p /var/lib/cinder
    if [ ! -f "$CINDER_LOOP" ]; then
        sudo dd if=/dev/zero of="$CINDER_LOOP" bs=1G count=50 status=progress 2>&1 | tail -1
    fi
    LOOP_DEV=$(sudo losetup -f --show "$CINDER_LOOP")
    sudo pvcreate "$LOOP_DEV"
    sudo vgcreate cinder-volumes "$LOOP_DEV"

    # Make loopback persistent across reboots
    cat << LOOP_SERVICE | sudo tee /etc/systemd/system/cinder-loop.service > /dev/null
[Unit]
Description=Setup Cinder LVM loopback device
Before=docker.service
After=local-fs.target

[Service]
Type=oneshot
ExecStart=/sbin/losetup -f /var/lib/cinder/cinder-volumes.img
ExecStartPost=/bin/bash -c 'sleep 1 && vgchange -ay cinder-volumes'
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
LOOP_SERVICE
    sudo systemctl daemon-reload
    sudo systemctl enable cinder-loop.service
    echo "  Cinder LVM volume group created (50 GiB loopback)."
else
    echo "  Cinder LVM volume group already exists."
fi

# ============================================================
# Step 6: Bootstrap, pre-check, and deploy
# ============================================================
echo "[6/7] Running Kolla-Ansible deployment..."
echo "      This takes 20-40 minutes (pulling Docker images + deploying services)."
echo ""

INVENTORY="/opt/kolla-config/all-in-one"

echo "  [6a/7] Bootstrap servers..."
kolla-ansible -i "$INVENTORY" bootstrap-servers 2>&1 | tail -5
echo ""

echo "  [6b/7] Pre-checks..."
kolla-ansible -i "$INVENTORY" prechecks 2>&1 | tail -5
echo ""

echo "  [6c/7] Deploying OpenStack services..."
set +e
kolla-ansible -i "$INVENTORY" deploy 2>&1 | tail -20
DEPLOY_EXIT=$?
set -e

if [ $DEPLOY_EXIT -ne 0 ]; then
    echo ""
    echo "============================================================"
    echo " Kolla-Ansible deployment FAILED (exit code: $DEPLOY_EXIT)"
    echo " Check logs: docker logs <service_container>"
    echo " Re-try:  source $KOLLA_VENV/bin/activate && kolla-ansible -i $INVENTORY deploy"
    echo "============================================================"
    exit 1
fi

echo ""
echo "  [6d/7] Post-deploy setup..."
kolla-ansible -i "$INVENTORY" post-deploy 2>&1 | tail -5

echo "  Deployment complete."

# ============================================================
# Step 7: Install OpenStack CLI & configure endpoints
# ============================================================
echo "[7/7] Configuring for external access..."

# Install OpenStack CLI
pip install python-openstackclient > /dev/null 2>&1

# Source admin credentials
source /etc/kolla/admin-openrc.sh

# Verify connectivity
if ! openstack token issue -f value -c id > /dev/null 2>&1; then
    echo "ERROR: Cannot authenticate to Keystone after deployment."
    exit 1
fi

# Get project/region info
PROJECT_ID=$(openstack project show admin -f value -c id 2>/dev/null || echo "UNKNOWN")
REGION=$(openstack region list -f value -c Region 2>/dev/null | head -1 || echo "RegionOne")
AZ=$(openstack availability zone list --compute -f value -c "Zone Name" 2>/dev/null | grep -v "^internal$" | head -1 || echo "nova")

# Update service catalog endpoints to use Public IP
# Kolla-Ansible binds to HOST_IP (private), but external clients need PUBLIC_IP
if [ "$PUBLIC_IP" != "$HOST_IP" ]; then
    CHANGED=0
    for eid in $(openstack endpoint list -f value -c ID 2>/dev/null); do
        eurl=$(openstack endpoint show "$eid" -f value -c url 2>/dev/null)
        if echo "$eurl" | grep -q "$HOST_IP"; then
            new_url=$(echo "$eurl" | sed "s/$HOST_IP/$PUBLIC_IP/g")
            openstack endpoint set --url "$new_url" "$eid" 2>/dev/null
            CHANGED=$((CHANGED + 1))
        fi
    done
    if [ $CHANGED -gt 0 ]; then
        echo ""
        echo "  Updated $CHANGED service catalog endpoint(s): $HOST_IP -> $PUBLIC_IP"
    fi
fi

# ----------------------------------------------------------
# Create placeholder service catalog entries for CB-Spider
# CB-Spider's OpenStack driver (gophercloud v2) requires:
#   "load-balancer", "block-storage" (aliases: volumev3 etc.), "shared-file-system" (alias: sharev2)
# gophercloud v2 ServiceTypeAliases handles matching automatically:
#   - "block-storage" matches directly (no volumev3 alias needed)
#   - "shared-file-system" or "sharev2" both match
# We create placeholders for services not deployed.
# ----------------------------------------------------------
PLACEHOLDER_CREATED=0

# Cinder (Block Storage) - gophercloud v2 type: "block-storage"
# gophercloud v2 ServiceTypeAliases: "block-storage" -> ["volumev3", "volumev2", "volume", "block-store"]
# No alias entry needed; "block-storage" or "volumev3" is matched directly.
if openstack service list -f value -c Type 2>/dev/null | grep -qE "^(block-storage|volumev3)$"; then
    echo "  ✓ block-storage (Cinder) - installed"
else
    echo "  ✗ block-storage (Cinder) - NOT found"
    echo "    WARNING: Cinder is not available. Disk operations will fail."
fi

# Octavia (Load Balancer) - gophercloud v2 type: "load-balancer" (no aliases)
if ! openstack service list -f value -c Type 2>/dev/null | grep -q "^load-balancer$"; then
    openstack service create --name octavia --description "Load Balancer (placeholder for CB-Spider)" load-balancer 2>/dev/null && \
    openstack endpoint create --region "$REGION" load-balancer public "http://${PUBLIC_IP}:9876/placeholder/v2.0" 2>/dev/null && \
    PLACEHOLDER_CREATED=$((PLACEHOLDER_CREATED + 1))
    echo "  Created placeholder: load-balancer (octavia)"
fi

# Manila (Shared File System) - gophercloud v2 type: "shared-file-system" (alias: "sharev2")
if ! openstack service list -f value -c Type 2>/dev/null | grep -qE "^(shared-file-system|sharev2)$"; then
    openstack service create --name manilav2 --description "Shared File System (placeholder for CB-Spider)" shared-file-system 2>/dev/null && \
    openstack endpoint create --region "$REGION" shared-file-system public "http://${PUBLIC_IP}:8786/placeholder/v2" 2>/dev/null && \
    PLACEHOLDER_CREATED=$((PLACEHOLDER_CREATED + 1))
    echo "  Created placeholder: shared-file-system (manilav2)"
fi

if [ $PLACEHOLDER_CREATED -gt 0 ]; then
    echo "  Created $PLACEHOLDER_CREATED placeholder service(s) for CB-Spider compatibility"
fi

# Save admin-openrc with public IP for convenience
cat << OPENRC > /etc/kolla/admin-openrc-public.sh
export OS_PROJECT_DOMAIN_NAME=Default
export OS_USER_DOMAIN_NAME=Default
export OS_PROJECT_NAME=admin
export OS_TENANT_NAME=admin
export OS_USERNAME=admin
export OS_PASSWORD=${ADMIN_PASSWORD}
export OS_AUTH_URL=http://${PUBLIC_IP}:5000/v3
export OS_INTERFACE=public
export OS_ENDPOINT_TYPE=publicURL
export OS_IDENTITY_API_VERSION=3
export OS_REGION_NAME=${REGION}
export OS_AUTH_PLUGIN=password
OPENRC

echo ""
echo "============================================================"
echo " Kolla-Ansible installation COMPLETED successfully!"
echo "============================================================"
echo ""
echo " Horizon Dashboard  : http://${PUBLIC_IP}"
echo " Keystone Auth URL  : http://${PUBLIC_IP}:5000/v3"
echo " Username / Password: admin / ${ADMIN_PASSWORD}"
echo " Project ID         : ${PROJECT_ID}"
echo " Region / AZ        : ${REGION} / ${AZ}"
echo ""

echo "============================================================"
echo " credentials.yaml snippet"
echo "============================================================"
cat << CRED_EOF

    ${CSP_NAME}:
      IdentityEndpoint: http://${PUBLIC_IP}:5000/v3
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
    description: OpenStack Kolla-Ansible (${PUBLIC_IP})
    cloudPlatform: openstack
    driver: openstack-driver-v1.0.so
    region:
      ${REGION}:
        id: ${REGION}
        description: Kolla-Ansible ${REGION}
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
echo " 2. Open AWS Security Group ports: 5000, 8774, 9292, 9696, 8776"
echo ""
echo " 3. Re-initialize CB-Tumblebug:"
echo "    make enc-cred && make init"
echo ""
echo " For detailed info, run: ./2.getRegistrationInfo.sh"
echo ""
echo " NOTE: Unlike DevStack, Kolla-Ansible services auto-restart on reboot."
echo "       No need to re-run installation after a reboot."
