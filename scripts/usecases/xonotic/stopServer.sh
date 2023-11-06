#!/bin/bash

# This script should be run as root for full access to systemd logs and service management
if [ "$EUID" -ne 0 ]; then 
  echo "Please run as root"
  exit
fi

echo "[Stop Xonotic FPS Game]"

# Stop the systemd service
systemctl stop xonotic.service

echo "Xonotic service has been stopped."

# Optionally, check to confirm the service is stopped
echo "[Check Xonotic Service Status]"
systemctl status xonotic.service

echo ""
