#!/bin/bash

# DevStack Endpoint Update Script
# Updates OpenStack service catalog endpoints when the VM's public IP changes
# (e.g., after suspend/resume, stop/start, or IP reassignment).
#
# This script:
#   1. Detects the current public IP
#   2. Updates all service catalog endpoints from old IP to new public IP
#   3. Outputs updated credentials.yaml snippet for CB-Tumblebug re-registration
#
# Usage:
#   ./3.updateEndpoints.sh [--csp-name CSP_NAME]
#
# After running, update credentials.yaml with the new IdentityEndpoint and run:
#   make enc-cred && make init

set -e

# ============================================================
# Parse arguments
# ============================================================
CSP_NAME="openstack-devstack"

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
echo " DevStack Endpoint Update"
echo "============================================================"
echo ""
echo " Internal IP : $HOST_IP"
echo " Public IP   : $PUBLIC_IP"
echo ""

# ============================================================
# Verify DevStack is running
# ============================================================
DEVSTACK_DIR="/opt/stack/devstack"
if [ ! -f "$DEVSTACK_DIR/openrc" ]; then
    echo "ERROR: DevStack not found at $DEVSTACK_DIR"
    exit 1
fi

# Extract admin password
ADMIN_PASSWORD=$(grep "^ADMIN_PASSWORD=" "$DEVSTACK_DIR/local.conf" 2>/dev/null | cut -d'=' -f2)
ADMIN_PASSWORD=${ADMIN_PASSWORD:-cbtumblebug}

# Source openrc
source "$DEVSTACK_DIR/openrc" admin admin 2>/dev/null

# Quick health check
if ! openstack token issue -f value -c id > /dev/null 2>&1; then
    echo "ERROR: Cannot authenticate to Keystone. DevStack may not be running."
    echo "       Try: cd /opt/stack/devstack && ./stack.sh"
    exit 1
fi

# ============================================================
# Detect old IP from existing endpoints
# ============================================================
# Get the first endpoint URL and extract the IP/hostname
SAMPLE_URL=$(openstack endpoint list -f value -c URL 2>/dev/null | head -1)
if [ -z "$SAMPLE_URL" ]; then
    echo "ERROR: No endpoints found in service catalog."
    exit 1
fi

# Extract IP from URL (e.g., http://10.33.51.121:8774/v2.1 -> 10.33.51.121)
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
# ============================================================
REGION=$(openstack region list -f value -c Region 2>/dev/null | head -1 || echo "RegionOne")

if ! openstack service list -f value -c Type 2>/dev/null | grep -q "^load-balancer$"; then
    openstack service create --name octavia --description "Load Balancer (placeholder for CB-Spider)" load-balancer 2>/dev/null && \
    openstack endpoint create --region "$REGION" load-balancer public "http://${PUBLIC_IP}/placeholder/load-balancer/v2.0" 2>/dev/null && \
    echo " Created placeholder: load-balancer (octavia)"
fi

if ! openstack service list -f value -c Type 2>/dev/null | grep -q "^shared-file-system$"; then
    openstack service create --name manilav2 --description "Shared File System (placeholder for CB-Spider)" shared-file-system 2>/dev/null && \
    openstack endpoint create --region "$REGION" shared-file-system public "http://${PUBLIC_IP}/placeholder/shared-file-system/v2" 2>/dev/null && \
    echo " Created placeholder: shared-file-system (manilav2)"
fi

# ============================================================
# Output updated credentials snippet
# ============================================================
DOMAIN_NAME="Default"
PROJECT_ID=$(openstack project show admin -f value -c id 2>/dev/null || echo "UNKNOWN")
IDENTITY_ENDPOINT="http://${PUBLIC_IP}/identity/v3"

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
echo "    Old: http://${OLD_IP}/identity/v3"
echo "    New: ${IDENTITY_ENDPOINT}"
echo ""
echo " 2. Re-initialize CB-Tumblebug:"
echo "    make enc-cred && make init"
echo ""
echo " 3. Verify connectivity:"
echo "    curl -s http://${PUBLIC_IP}/identity/v3"
echo ""
