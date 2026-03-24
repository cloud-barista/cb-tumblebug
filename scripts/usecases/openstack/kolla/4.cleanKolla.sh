#!/bin/bash

# Kolla-Ansible Cleanup Script
# Removes the Kolla-Ansible deployment and cleans up all resources.
#
# Usage:
#   ./4.cleanKolla.sh          # Remove deployment, keep virtual environment
#   ./4.cleanKolla.sh --full   # Remove everything including venv and Docker images

set -e

FULL_CLEAN=false

while [[ "$1" != "" ]]; do
    case $1 in
        --full ) FULL_CLEAN=true ;;
        * )      echo "Usage: $0 [--full]"; exit 1 ;;
    esac
    shift
done

echo "============================================================"
echo " Kolla-Ansible Cleanup"
if $FULL_CLEAN; then
    echo " Mode: FULL (remove everything)"
else
    echo " Mode: Standard (keep venv for re-deploy)"
fi
echo "============================================================"
echo ""

KOLLA_VENV="/opt/kolla-venv"
INVENTORY="/opt/kolla-config/all-in-one"

# ============================================================
# Step 1: Kolla-Ansible destroy (if venv exists)
# ============================================================
if [ -d "$KOLLA_VENV" ] && [ -f "$INVENTORY" ]; then
    echo "[1/5] Running kolla-ansible destroy..."
    source "$KOLLA_VENV/bin/activate"

    set +e
    kolla-ansible -i "$INVENTORY" destroy --yes-i-really-really-mean-it 2>&1 | tail -10
    set -e

    deactivate 2>/dev/null || true
    echo "  Kolla services destroyed."
else
    echo "[1/5] Skipping kolla-ansible destroy (venv or inventory not found)."
fi

# ============================================================
# Step 2: Clean up remaining Docker containers
# ============================================================
echo "[2/5] Cleaning up Kolla Docker containers..."

KOLLA_CONTAINERS=$(docker ps -a --filter "name=kolla" -q 2>/dev/null)
if [ -n "$KOLLA_CONTAINERS" ]; then
    docker stop $KOLLA_CONTAINERS 2>/dev/null || true
    docker rm -f $KOLLA_CONTAINERS 2>/dev/null || true
    echo "  Removed $(echo "$KOLLA_CONTAINERS" | wc -w) Kolla container(s)."
else
    echo "  No Kolla containers found."
fi

# ============================================================
# Step 3: Clean up Cinder LVM
# ============================================================
echo "[3/5] Cleaning up Cinder LVM..."

if sudo vgdisplay cinder-volumes > /dev/null 2>&1; then
    sudo vgremove -f cinder-volumes 2>/dev/null || true
    echo "  Removed cinder-volumes VG."
fi

# Remove loopback
CINDER_LOOP="/var/lib/cinder/cinder-volumes.img"
if [ -f "$CINDER_LOOP" ]; then
    # Find and detach any loop device using this file
    for loop in $(sudo losetup -j "$CINDER_LOOP" 2>/dev/null | cut -d: -f1); do
        sudo losetup -d "$loop" 2>/dev/null || true
    done
    sudo rm -f "$CINDER_LOOP"
    echo "  Removed Cinder loopback file."
fi

# Remove systemd service
if [ -f /etc/systemd/system/cinder-loop.service ]; then
    sudo systemctl disable cinder-loop.service 2>/dev/null || true
    sudo rm -f /etc/systemd/system/cinder-loop.service
    sudo systemctl daemon-reload
    echo "  Removed cinder-loop systemd service."
fi

# ============================================================
# Step 4: Clean up configuration
# ============================================================
echo "[4/5] Cleaning up configuration..."

sudo rm -rf /etc/kolla
sudo rm -rf /opt/kolla-config
echo "  Removed /etc/kolla and /opt/kolla-config."

# ============================================================
# Step 5: Full cleanup (optional)
# ============================================================
if $FULL_CLEAN; then
    echo "[5/5] Full cleanup..."

    # Remove virtual environment
    sudo rm -rf "$KOLLA_VENV"
    echo "  Removed $KOLLA_VENV."

    # Remove Kolla Docker images
    KOLLA_IMAGES=$(docker images --filter "reference=kolla/*" -q 2>/dev/null)
    if [ -n "$KOLLA_IMAGES" ]; then
        docker rmi -f $KOLLA_IMAGES 2>/dev/null || true
        echo "  Removed Kolla Docker images."
    fi

    # Clean up docker volumes
    docker volume prune -f 2>/dev/null || true
    echo "  Pruned unused Docker volumes."

    # Remove Cinder data directory
    sudo rm -rf /var/lib/cinder
    echo "  Removed /var/lib/cinder."
else
    echo "[5/5] Skipping full cleanup (use --full to remove venv and Docker images)."
fi

echo ""
echo "============================================================"
echo " Cleanup complete!"
echo "============================================================"
echo ""
if $FULL_CLEAN; then
    echo " All Kolla-Ansible resources have been removed."
    echo " To re-install: ./1.installKolla.sh"
else
    echo " Deployment removed. Virtual environment preserved for faster re-deploy."
    echo " To re-deploy:"
    echo "   source $KOLLA_VENV/bin/activate"
    echo "   kolla-ansible -i /opt/kolla-config/all-in-one deploy"
    echo ""
    echo " To fully remove everything: ./4.cleanKolla.sh --full"
fi
echo ""
