#!/bin/bash

# Kolla-Ansible Endpoint Update Script
# Updates OpenStack service catalog endpoints when the VM's public IP changes
# (e.g., after suspend/resume, stop/start, or IP reassignment).
#
# Unlike DevStack, Kolla-Ansible services auto-restart on reboot.
# This script only needs to update the service catalog endpoints.
#
# Usage:
#   ./3.updateEndpoints.sh [--csp-name CSP_NAME]

set -e

# ============================================================
# Parse arguments
# ============================================================
CSP_NAME="openstack-kolla"

while [[ "$1" != "" ]]; do
    case $1 in
        --csp-name ) shift; CSP_NAME=$1 ;;
        * )          echo "Usage: $0 [--csp-name CSP_NAME]"; exit 1 ;;
    esac
    shift
done

# ============================================================
# Detect IPs
# ============================================================
HOST_IP=$(hostname -I | awk '{print $1}')
PUBLIC_IP=$(curl -s --connect-timeout 5 http://169.254.169.254/latest/meta-data/public-ipv4 2>/dev/null || \
            curl -s --connect-timeout 5 https://api.ipify.org 2>/dev/null || \
            echo "$HOST_IP")

echo "============================================================"
echo " Kolla-Ansible Endpoint Update"
echo "============================================================"
echo ""
echo " Internal IP : $HOST_IP"
echo " Public IP   : $PUBLIC_IP"
echo ""

# ============================================================
# Activate Kolla virtual environment
# ============================================================
KOLLA_VENV="/opt/kolla-venv"
if [ ! -d "$KOLLA_VENV" ]; then
    echo "ERROR: Kolla-Ansible virtual environment not found at $KOLLA_VENV"
    exit 1
fi
source "$KOLLA_VENV/bin/activate"

# Source admin credentials
OPENRC="/etc/kolla/admin-openrc.sh"
if [ ! -f "$OPENRC" ]; then
    echo "ERROR: admin-openrc.sh not found."
    exit 1
fi
source "$OPENRC"

# Extract admin password
ADMIN_PASSWORD=$(grep "^keystone_admin_password:" /etc/kolla/passwords.yml 2>/dev/null | awk '{print $2}')
ADMIN_PASSWORD=${ADMIN_PASSWORD:-cbtumblebug}

# Quick health check
if ! openstack token issue -f value -c id > /dev/null 2>&1; then
    echo "ERROR: Cannot authenticate to Keystone."
    echo "       Check if Kolla services are running: docker ps | grep keystone"
    exit 1
fi

# ============================================================
# Detect old IP from existing endpoints
# ============================================================
SAMPLE_URL=$(openstack endpoint list -f value -c URL 2>/dev/null | head -1)
if [ -z "$SAMPLE_URL" ]; then
    echo "ERROR: No endpoints found in service catalog."
    exit 1
fi

# Extract IP from URL (e.g., http://10.33.51.121:5000/v3 -> 10.33.51.121)
OLD_IP=$(echo "$SAMPLE_URL" | sed -E 's|https?://([^:/]+).*|\1|')
echo " Old endpoint IP : $OLD_IP"

if [ "$OLD_IP" = "$PUBLIC_IP" ]; then
    echo ""
    echo " Endpoints already use the current Public IP. Nothing to update."
    echo ""
    exit 0
fi

# ============================================================
# Update all service catalog endpoints
# ============================================================
echo ""
echo "Updating service catalog endpoints: $OLD_IP -> $PUBLIC_IP"
echo ""

CHANGED=0
TOTAL=0
for eid in $(openstack endpoint list -f value -c ID 2>/dev/null); do
    TOTAL=$((TOTAL + 1))
    eurl=$(openstack endpoint show "$eid" -f value -c url 2>/dev/null)
    if echo "$eurl" | grep -q "$OLD_IP"; then
        new_url=$(echo "$eurl" | sed "s/$OLD_IP/$PUBLIC_IP/g")
        openstack endpoint set --url "$new_url" "$eid" 2>/dev/null
        CHANGED=$((CHANGED + 1))
        echo "  ✓ $eurl -> $new_url"
    fi
done

echo ""
echo " Updated $CHANGED / $TOTAL endpoint(s)."

# ============================================================
# Ensure placeholder services exist for CB-Spider compatibility
# CB-Spider's gophercloud v2 uses ServiceTypeAliases:
#   "block-storage" -> ["volumev3", "volumev2", "volume", "block-store"]
#   "shared-file-system" -> ["sharev2", "share"]
#   "load-balancer" -> (no aliases, must be exact)
# ============================================================
REGION=$(openstack region list -f value -c Region 2>/dev/null | head -1 || echo "RegionOne")

# Cinder (Block Storage) - gophercloud v2 matches "block-storage" directly
# No volumev3 alias needed.
if ! openstack service list -f value -c Type 2>/dev/null | grep -qE "^(block-storage|volumev3)$"; then
    echo " WARNING: block-storage (Cinder) not found. Disk operations will fail."
fi

# Octavia (Load Balancer) - type must be exactly "load-balancer"
if ! openstack service list -f value -c Type 2>/dev/null | grep -q "^load-balancer$"; then
    openstack service create --name octavia --description "Load Balancer (placeholder for CB-Spider)" load-balancer 2>/dev/null && \
    openstack endpoint create --region "$REGION" load-balancer public "http://${PUBLIC_IP}:9876/placeholder/v2.0" 2>/dev/null && \
    echo " Created placeholder: load-balancer (octavia)"
fi

# Manila (Shared File System) - gophercloud v2 matches "shared-file-system" or alias "sharev2"
if ! openstack service list -f value -c Type 2>/dev/null | grep -qE "^(shared-file-system|sharev2)$"; then
    openstack service create --name manilav2 --description "Shared File System (placeholder for CB-Spider)" shared-file-system 2>/dev/null && \
    openstack endpoint create --region "$REGION" shared-file-system public "http://${PUBLIC_IP}:8786/placeholder/v2" 2>/dev/null && \
    echo " Created placeholder: shared-file-system (manilav2)"
fi

# ============================================================
# Update admin-openrc-public.sh
# ============================================================
REGION=$(openstack region list -f value -c Region 2>/dev/null | head -1 || echo "RegionOne")

cat << OPENRC_PUB > /etc/kolla/admin-openrc-public.sh
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
OPENRC_PUB

echo " Updated /etc/kolla/admin-openrc-public.sh"

# ============================================================
# Output updated credentials snippet
# ============================================================
DOMAIN_NAME="Default"
PROJECT_ID=$(openstack project show admin -f value -c id 2>/dev/null || echo "UNKNOWN")
IDENTITY_ENDPOINT="http://${PUBLIC_IP}:5000/v3"

echo ""
echo "============================================================"
echo " Updated credentials.yaml snippet"
echo "============================================================"
cat << EOF

    ${CSP_NAME}:
      IdentityEndpoint: ${IDENTITY_ENDPOINT}
      Username: admin
      Password: ${ADMIN_PASSWORD}
      DomainName: ${DOMAIN_NAME}
      ProjectID: ${PROJECT_ID}

EOF

echo "============================================================"
echo " Next Steps"
echo "============================================================"
echo ""
echo " 1. Update IdentityEndpoint in ~/.cloud-barista/credentials.yaml"
echo "    Old: http://${OLD_IP}:5000/v3"
echo "    New: ${IDENTITY_ENDPOINT}"
echo ""
echo " 2. Re-initialize CB-Tumblebug:"
echo "    make enc-cred && make init"
echo ""
echo " 3. Verify connectivity:"
echo "    curl -s http://${PUBLIC_IP}:5000/v3 | jq ."
echo ""
