#!/bin/bash

# This script should be run as root for full access to systemd logs and service management
if [ "$EUID" -ne 0 ]; then 
  echo "Please run as root"
  exit
fi

echo "[Stop Filebeat]"
# Stop the systemd service
systemctl stop filebeat.service
echo "Filebeat services have been stopped."

# Optionally, check to confirm the service is stopped
echo "[Systemd service status of Filebeat]"
sudo systemctl status filebeat.service --no-pager
