#!/bin/bash

# Get OpenStack Registration Info for CB-Tumblebug
# Run this AFTER DevStack installation to extract the information needed
# to register this OpenStack instance as a new CSP in CB-Tumblebug.
#
# Usage:
#   ./2.getRegistrationInfo.sh [--csp-name CSP_NAME] [--latitude LAT] [--longitude LON] [--location DISPLAY_NAME]
#
# Output:
#   - Prints credential and registration info
#   - Generates cloudinfo snippet and credentials snippet for copy-paste

set -e

# ============================================================
# Parse arguments
# ============================================================
CSP_NAME="openstack-devstack"
LATITUDE=""
LONGITUDE=""
LOCATION_DISPLAY=""

while [[ "$1" != "" ]]; do
    case $1 in
        --csp-name ) shift; CSP_NAME=$1 ;;
        --latitude ) shift; LATITUDE=$1 ;;
        --longitude ) shift; LONGITUDE=$1 ;;
        --location ) shift; LOCATION_DISPLAY=$1 ;;
        * )          echo "Usage: $0 [--csp-name CSP_NAME] [--latitude LAT] [--longitude LON] [--location DISPLAY_NAME]"; exit 1 ;;
    esac
    shift
done

# ============================================================
# Detect environment
# ============================================================
HOST_IP=$(hostname -I | awk '{print $1}')
PUBLIC_IP=$(curl -s --connect-timeout 5 http://169.254.169.254/latest/meta-data/public-ipv4 2>/dev/null || \
            curl -s --connect-timeout 5 https://api.ipify.org 2>/dev/null || \
            echo "$HOST_IP")

# Set defaults for optional location parameters
LATITUDE="${LATITUDE:-0}"
LONGITUDE="${LONGITUDE:-0}"
LOCATION_DISPLAY="${LOCATION_DISPLAY:-DevStack}"

# Source DevStack openrc for admin credentials
DEVSTACK_DIR="/opt/stack/devstack"
if [ ! -f "$DEVSTACK_DIR/openrc" ]; then
    echo "ERROR: DevStack not found at $DEVSTACK_DIR"
    echo "       Run 1.installDevStack.sh first."
    exit 1
fi

# Extract admin password from local.conf
ADMIN_PASSWORD=$(grep "^ADMIN_PASSWORD=" "$DEVSTACK_DIR/local.conf" 2>/dev/null | cut -d'=' -f2)
ADMIN_PASSWORD=${ADMIN_PASSWORD:-cbtumblebug}

# Source openrc to get environment variables
source "$DEVSTACK_DIR/openrc" admin admin 2>/dev/null

# ============================================================
# Gather OpenStack info
# ============================================================
echo "Gathering OpenStack information..."

# Identity endpoint (Keystone)
# DevStack 2025.2 uses Apache reverse proxy: /identity/v3 on port 80
IDENTITY_ENDPOINT="http://${PUBLIC_IP}/identity/v3"
IDENTITY_ENDPOINT_INTERNAL="http://${HOST_IP}/identity/v3"

# Project ID
PROJECT_ID=$(openstack project show admin -f value -c id 2>/dev/null || echo "UNKNOWN")

# Domain (capital 'D' — Keystone accepts both but CB-Tumblebug convention uses 'Default')
DOMAIN_NAME="Default"

# Region
REGION=$(openstack region list -f value -c Region 2>/dev/null | head -1 || echo "RegionOne")

# Available flavors
echo ""
echo "Available Flavors (Specs):"
openstack flavor list -f table 2>/dev/null || echo "  (could not list flavors)"

# Available images
echo ""
echo "Available Images:"
openstack image list -f table 2>/dev/null || echo "  (could not list images)"

# Available networks
echo ""
echo "Available Networks:"
openstack network list -f table 2>/dev/null || echo "  (could not list networks)"

# Availability zones
echo ""
echo "Availability Zones (Compute):"
openstack availability zone list --compute -f table 2>/dev/null || echo "  (could not list AZs)"
# Filter out 'internal' AZ (Nova internal services only); select the actual compute AZ
AZ=$(openstack availability zone list --compute -f value -c "Zone Name" 2>/dev/null | grep -v "^internal$" | head -1 || echo "nova")

# ============================================================
# Update service catalog endpoints to use Public IP
# ============================================================
if [ "$PUBLIC_IP" != "$HOST_IP" ]; then
    echo ""
    echo "Checking service catalog endpoints..."
    CHANGED=0
    for eid in $(openstack endpoint list -f value -c ID 2>/dev/null); do
        eurl=$(openstack endpoint show "$eid" -f value -c url 2>/dev/null)
        if echo "$eurl" | grep -Fq "$HOST_IP"; then
            new_url=${eurl//$HOST_IP/$PUBLIC_IP}
            openstack endpoint set --url "$new_url" "$eid" 2>/dev/null
            CHANGED=$((CHANGED + 1))
        fi
    done
    if [ $CHANGED -gt 0 ]; then
        echo "  Updated $CHANGED endpoint(s): $HOST_IP -> $PUBLIC_IP"
    else
        echo "  All endpoints already use Public IP."
    fi
fi

# ============================================================
# Verify CB-Spider required services in service catalog
# CB-Spider's OpenStack driver (gophercloud v2) requires:
#   "load-balancer", "block-storage" (aliases: volumev3 etc.), "shared-file-system" (alias: sharev2)
# gophercloud v2 ServiceTypeAliases handles matching automatically.
# If missing, create placeholders as fallback.
# ============================================================
echo ""
echo "Verifying CB-Spider required services..."
PLACEHOLDER_CREATED=0

# Cinder (Block Storage) - gophercloud v2 type: "block-storage"
# gophercloud v2 ServiceTypeAliases: "block-storage" -> ["volumev3", "volumev2", "volume", "block-store"]
# No alias entry needed; "block-storage" is matched directly.
if openstack service list -f value -c Type 2>/dev/null | grep -qE "^(block-storage|volumev3)$"; then
    echo "  ✓ block-storage (Cinder) - installed"
else
    echo "  ✗ block-storage (Cinder) - NOT found"
    echo "    WARNING: Cinder is not available. Disk operations will fail."
fi

# Octavia (Load Balancer) - gophercloud v2 type: "load-balancer"
if openstack service list -f value -c Type 2>/dev/null | grep -q "^load-balancer$"; then
    echo "  ✓ load-balancer (Octavia) - installed"
else
    echo "  ✗ load-balancer (Octavia) - NOT found, creating placeholder..."
    openstack service create --name octavia --description "Load Balancer (placeholder for CB-Spider)" load-balancer && \
    openstack endpoint create --region "$REGION" load-balancer public "http://${PUBLIC_IP}/placeholder/load-balancer/v2.0" && \
    PLACEHOLDER_CREATED=$((PLACEHOLDER_CREATED + 1))
fi

# Manila (Shared File System) - gophercloud v2 type: "shared-file-system" (alias: "sharev2")
if openstack service list -f value -c Type 2>/dev/null | grep -qE "^(shared-file-system|sharev2)$"; then
    echo "  ✓ shared-file-system (Manila) - installed"
else
    echo "  ✗ shared-file-system (Manila) - NOT found, creating placeholder..."
    openstack service create --name manilav2 --description "Shared File System (placeholder for CB-Spider)" shared-file-system && \
    openstack endpoint create --region "$REGION" shared-file-system public "http://${PUBLIC_IP}/placeholder/shared-file-system/v2" && \
    PLACEHOLDER_CREATED=$((PLACEHOLDER_CREATED + 1))
fi

if [ $PLACEHOLDER_CREATED -gt 0 ]; then
    echo "  ⚠ Created $PLACEHOLDER_CREATED placeholder(s) - consider re-running install with plugins"
fi

# ============================================================
# Output registration information
# ============================================================
echo ""
echo "============================================================"
echo " CB-Tumblebug Registration Information"
echo "============================================================"
echo ""
echo " CSP Name           : $CSP_NAME"
echo " Cloud Platform     : openstack"
echo " Identity Endpoint  : $IDENTITY_ENDPOINT"
echo " Username           : admin"
echo " Password           : $ADMIN_PASSWORD"
echo " Domain Name        : $DOMAIN_NAME"
echo " Project ID         : $PROJECT_ID"
echo " Region             : $REGION"
echo " Availability Zone  : $AZ"
echo " Public IP          : $PUBLIC_IP"
echo " Internal IP        : $HOST_IP"
echo ""

# ============================================================
# Generate cloudinfo.yaml snippet
# ============================================================
echo "============================================================"
echo " cloudinfo.yaml snippet (add to assets/cloudinfo.yaml)"
echo "============================================================"
cat << EOF

  ${CSP_NAME}:
    description: DevStack on AWS (${PUBLIC_IP})
    cloudPlatform: openstack
    driver: openstack-driver-v1.0.so
    region:
      ${REGION}:
        id: ${REGION}
        description: DevStack ${REGION}
        location:
          display: ${LOCATION_DISPLAY}
          latitude: ${LATITUDE}
          longitude: ${LONGITUDE}
        zone:
        - ${AZ}

EOF

# ============================================================
# Generate credentials.yaml snippet
# ============================================================
echo "============================================================"
echo " credentials.yaml snippet (add to ~/.cloud-barista/credentials.yaml)"
echo "============================================================"
cat << EOF

    ${CSP_NAME}:
      IdentityEndpoint: ${IDENTITY_ENDPOINT}
      Username: admin
      Password: ${ADMIN_PASSWORD}
      DomainName: ${DOMAIN_NAME}
      ProjectID: ${PROJECT_ID}

EOF

# ============================================================
# API connectivity test
# ============================================================
echo "============================================================"
echo " API Connectivity Test"
echo "============================================================"
echo ""

# Test Keystone token
echo -n "Keystone Auth (internal) ... "
TOKEN_RESP=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Content-Type: application/json" \
    -d "{\"auth\":{\"identity\":{\"methods\":[\"password\"],\"password\":{\"user\":{\"name\":\"admin\",\"domain\":{\"name\":\"$DOMAIN_NAME\"},\"password\":\"$ADMIN_PASSWORD\"}}},\"scope\":{\"project\":{\"name\":\"admin\",\"domain\":{\"name\":\"$DOMAIN_NAME\"}}}}}" \
    "${IDENTITY_ENDPOINT_INTERNAL}/auth/tokens" 2>/dev/null)
if [ "$TOKEN_RESP" = "201" ]; then
    echo "OK (HTTP 201)"
else
    echo "FAILED (HTTP $TOKEN_RESP)"
fi

echo -n "Keystone Auth (public)   ... "
TOKEN_RESP_PUB=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Content-Type: application/json" \
    -d "{\"auth\":{\"identity\":{\"methods\":[\"password\"],\"password\":{\"user\":{\"name\":\"admin\",\"domain\":{\"name\":\"$DOMAIN_NAME\"},\"password\":\"$ADMIN_PASSWORD\"}}},\"scope\":{\"project\":{\"name\":\"admin\",\"domain\":{\"name\":\"$DOMAIN_NAME\"}}}}}" \
    "${IDENTITY_ENDPOINT}/auth/tokens" 2>/dev/null)
if [ "$TOKEN_RESP_PUB" = "201" ]; then
    echo "OK (HTTP 201)"
else
    echo "FAILED (HTTP $TOKEN_RESP_PUB)"
    echo ""
    echo "  ⚠ Public endpoint test failed. Ensure:"
    echo "    1. AWS Security Group allows inbound on port 80 (HTTP) and 443 (HTTPS)"
    echo "    2. DevStack services are bound to 0.0.0.0 or the public IP"
    echo "    3. CB-Spider can reach ${PUBLIC_IP} from its network"
fi

echo ""
echo "============================================================"
echo " Next Steps"
echo "============================================================"
echo ""
echo " 1. Add the snippets above to:"
echo "    - ~/.cloud-barista/credentials.yaml  (credentials)"
echo "    - cb-tumblebug/assets/cloudinfo.yaml  (cloud info)"
echo ""
echo " 2. Open AWS Security Group for this VM:"
echo "    - Port 80   (Keystone/Horizon via Apache reverse proxy)"
echo "    - Port 9696 (Neutron network)"
echo "    Source: CB-Spider/CB-Tumblebug server IP"
echo ""
echo " 3. Re-initialize CB-Tumblebug:"
echo "    make enc-cred && make init"
echo ""
echo " 4. The new CSP '${CSP_NAME}' should appear in connection list."
echo ""
