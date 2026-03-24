#!/bin/bash

# Get OpenStack (Kolla-Ansible) Registration Info for CB-Tumblebug
# Run this AFTER Kolla-Ansible installation to extract the information needed
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
CSP_NAME="openstack-kolla"
LATITUDE=""
LONGITUDE=""
LOCATION_DISPLAY=""

while [[ "$1" != "" ]]; do
    case $1 in
        --csp-name )  shift; CSP_NAME=$1 ;;
        --latitude )  shift; LATITUDE=$1 ;;
        --longitude ) shift; LONGITUDE=$1 ;;
        --location )  shift; LOCATION_DISPLAY=$1 ;;
        * )           echo "Usage: $0 [--csp-name CSP_NAME] [--latitude LAT] [--longitude LON] [--location DISPLAY_NAME]"; exit 1 ;;
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
LOCATION_DISPLAY="${LOCATION_DISPLAY:-OpenStack-Kolla}"

# ============================================================
# Activate Kolla virtual environment
# ============================================================
KOLLA_VENV="/opt/kolla-venv"
if [ ! -d "$KOLLA_VENV" ]; then
    echo "ERROR: Kolla-Ansible virtual environment not found at $KOLLA_VENV"
    echo "       Run 1.installKolla.sh first."
    exit 1
fi
source "$KOLLA_VENV/bin/activate"

# Source admin credentials
OPENRC="/etc/kolla/admin-openrc.sh"
if [ ! -f "$OPENRC" ]; then
    echo "ERROR: admin-openrc.sh not found. Kolla-Ansible may not be fully deployed."
    echo "       Run: source $KOLLA_VENV/bin/activate && kolla-ansible -i /opt/kolla-config/all-in-one post-deploy"
    exit 1
fi
source "$OPENRC"

# Extract admin password from passwords.yml
ADMIN_PASSWORD=$(grep "^keystone_admin_password:" /etc/kolla/passwords.yml 2>/dev/null | awk '{print $2}')
ADMIN_PASSWORD=${ADMIN_PASSWORD:-cbtumblebug}

# Quick health check
if ! openstack token issue -f value -c id > /dev/null 2>&1; then
    echo "ERROR: Cannot authenticate to Keystone."
    echo "       Check if Kolla services are running: docker ps | grep kolla"
    exit 1
fi

# ============================================================
# Gather OpenStack info
# ============================================================
echo "Gathering OpenStack information..."

# Identity endpoint (Keystone)
# Kolla-Ansible uses standard port 5000 for Keystone
IDENTITY_ENDPOINT="http://${PUBLIC_IP}:5000/v3"

# Project ID
PROJECT_ID=$(openstack project show admin -f value -c id 2>/dev/null || echo "UNKNOWN")

# Domain
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
AZ=$(openstack availability zone list --compute -f value -c "Zone Name" 2>/dev/null | grep -v "^internal$" | head -1 || echo "nova")

# Service catalog
echo ""
echo "Service Catalog:"
openstack endpoint list -f table 2>/dev/null || echo "  (could not list endpoints)"

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
# Ensure placeholder services exist for CB-Spider compatibility
# CB-Spider's gophercloud v2 uses ServiceTypeAliases:
#   "block-storage" -> ["volumev3", "volumev2", "volume", "block-store"]
#   "shared-file-system" -> ["sharev2", "share"]
#   "load-balancer" -> (no aliases, must be exact)
# These services are not deployed in minimal Kolla setup.
# ============================================================
REGION_FOR_PLACEHOLDER=$(openstack region list -f value -c Region 2>/dev/null | head -1 || echo "RegionOne")

# Cinder (Block Storage) - gophercloud v2 matches "block-storage" directly
# No volumev3 alias needed.
if openstack service list -f value -c Type 2>/dev/null | grep -qE "^(block-storage|volumev3)$"; then
    echo "  ✓ block-storage (Cinder) - installed"
else
    echo "  ✗ block-storage (Cinder) - NOT found"
    echo "    WARNING: Cinder is not available. Disk operations will fail."
fi

# Octavia (Load Balancer) - type must be exactly "load-balancer"
if ! openstack service list -f value -c Type 2>/dev/null | grep -q "^load-balancer$"; then
    openstack service create --name octavia --description "Load Balancer (placeholder for CB-Spider)" load-balancer 2>/dev/null && \
    openstack endpoint create --region "$REGION_FOR_PLACEHOLDER" load-balancer public "http://${PUBLIC_IP}:9876/placeholder/v2.0" 2>/dev/null && \
    echo "  Created placeholder: load-balancer (octavia)"
fi

# Manila (Shared File System) - gophercloud v2 matches "shared-file-system" or alias "sharev2"
if ! openstack service list -f value -c Type 2>/dev/null | grep -qE "^(shared-file-system|sharev2)$"; then
    openstack service create --name manilav2 --description "Shared File System (placeholder for CB-Spider)" shared-file-system 2>/dev/null && \
    openstack endpoint create --region "$REGION_FOR_PLACEHOLDER" shared-file-system public "http://${PUBLIC_IP}:8786/placeholder/v2" 2>/dev/null && \
    echo "  Created placeholder: shared-file-system (manilav2)"
fi

# ============================================================
# Docker service status
# ============================================================
echo ""
echo "Kolla Service Containers:"
docker ps --filter "name=kolla" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null | head -20 || echo "  (could not list containers)"

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
echo " Deploy Method      : Kolla-Ansible (Docker containers)"
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
    description: OpenStack Kolla-Ansible (${PUBLIC_IP})
    cloudPlatform: openstack
    driver: openstack-driver-v1.0.so
    region:
      ${REGION}:
        id: ${REGION}
        description: Kolla-Ansible ${REGION}
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

echo "============================================================"
echo " Next Steps"
echo "============================================================"
echo ""
echo " 1. Add the snippets above to the respective files"
echo " 2. Open AWS Security Group ports: 5000, 8774, 9292, 9696, 8776"
echo " 3. Run: make enc-cred && make init"
echo ""
echo " API connectivity test:"
echo "   curl -s http://${PUBLIC_IP}:5000/v3 | jq ."
echo ""
