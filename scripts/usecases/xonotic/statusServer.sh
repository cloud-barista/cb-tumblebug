#!/bin/bash

# This script should be run as root for full access to systemd logs and service management
if [ "$EUID" -ne 0 ]; then 
  echo "Please run as root"
  exit
fi

echo "[Status Xonotic FPS Game]"

# Replace with your actual log directory if different
LOG_DIR="/var/log/xonotic"

echo ""
echo "[Xonotic Server.log so far]"
# Display the last few lines of the game server log
tail -n 30 "$LOG_DIR/server.log"
echo ""

# Check the status of the xonotic service
echo "[Systemd service status of Xonotic]"
systemctl status xonotic.service --no-pager -l

echo ""