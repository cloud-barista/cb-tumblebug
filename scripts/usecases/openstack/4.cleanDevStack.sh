#!/bin/bash

# DevStack Cleanup Script
# Removes a failed or stale DevStack installation so you can re-install cleanly.
#
# This script:
#   1. Runs DevStack's own unstack.sh + clean.sh (if available)
#   2. Stops and removes all DevStack systemd services
#   3. Cleans up leftover directories and processes
#   4. Prepares the system for a fresh ./1.installDevStack.sh run
#
# Usage:
#   ./4.cleanDevStack.sh [--full]
#
# Options:
#   --full    Also remove the DevStack source directory (/opt/stack/devstack)
#             Without this flag, the source is kept for faster re-install.

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
echo " DevStack Cleanup"
echo "============================================================"
echo ""

DEVSTACK_DIR="/opt/stack/devstack"

# ============================================================
# Step 1: Run DevStack's own cleanup scripts
# ============================================================
if [ -f "$DEVSTACK_DIR/unstack.sh" ]; then
    echo "[1/4] Running unstack.sh (stops all services)..."
    # unstack.sh may fail on partially installed systems; don't exit
    set +e
    sudo -u stack bash -c "cd $DEVSTACK_DIR && ./unstack.sh" 2>/dev/null
    set -e
    echo "      Done."
else
    echo "[1/4] unstack.sh not found, skipping..."
fi

if [ -f "$DEVSTACK_DIR/clean.sh" ]; then
    echo "[2/4] Running clean.sh (removes data and configs)..."
    set +e
    sudo -u stack bash -c "cd $DEVSTACK_DIR && ./clean.sh" 2>/dev/null
    set -e
    echo "      Done."
else
    echo "[2/4] clean.sh not found, skipping..."
fi

# ============================================================
# Step 2: Stop and disable DevStack systemd services
# ============================================================
echo "[3/4] Cleaning up systemd services..."
set +e
# Stop all devstack@ services
for svc in $(systemctl list-units --type=service --no-legend 2>/dev/null | grep 'devstack@' | awk '{print $1}'); do
    sudo systemctl stop "$svc" 2>/dev/null
    sudo systemctl disable "$svc" 2>/dev/null
    echo "      Stopped: $svc"
done
# Remove service files
sudo rm -f /etc/systemd/system/devstack@*.service 2>/dev/null
sudo systemctl daemon-reload 2>/dev/null
set -e
echo "      Done."

# ============================================================
# Step 3: Clean up data directories
# ============================================================
echo "[4/4] Cleaning up data and logs..."

# Remove runtime data (but keep source code unless --full)
sudo rm -rf /opt/stack/data 2>/dev/null || true
sudo rm -rf /opt/stack/logs 2>/dev/null || true
sudo rm -rf /opt/stack/status 2>/dev/null || true
sudo rm -rf /opt/stack/.cache 2>/dev/null || true
sudo rm -rf /opt/stack/.config 2>/dev/null || true

# Remove virtual environments
sudo rm -rf /opt/stack/requirements 2>/dev/null || true
sudo rm -rf /opt/stack/bin 2>/dev/null || true

# Remove cloned OpenStack service repos (nova, glance, etc.)
# Keep devstack source itself unless --full
for dir in /opt/stack/*/; do
    dirname=$(basename "$dir")
    if [ "$dirname" != "devstack" ]; then
        sudo rm -rf "$dir" 2>/dev/null || true
    fi
done

if [ "$FULL_CLEAN" = true ]; then
    echo "      Removing DevStack source directory (--full mode)..."
    sudo rm -rf "$DEVSTACK_DIR" 2>/dev/null || true
fi

echo "      Done."

# ============================================================
# Summary
# ============================================================
echo ""
echo "============================================================"
echo " Cleanup Complete"
echo "============================================================"
echo ""
if [ "$FULL_CLEAN" = true ]; then
    echo " All DevStack files removed."
else
    echo " DevStack source preserved at: $DEVSTACK_DIR"
    echo " local.conf preserved (your previous configuration)."
fi
echo ""
echo " To re-install DevStack, run:"
echo "   ./1.installDevStack.sh [--csp-name NAME]"
echo ""
