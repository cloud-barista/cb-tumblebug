#!/bin/bash

# This script should be run as root for full access to systemd logs and service management
if [ "$EUID" -ne 0 ]; then 
  echo "Please run as root"
  exit
fi


echo "[Status Elastic Stack]"
# Replace with your actual log directory if different
LATEST_LOG=$(ls -t /var/log/filebeat/filebeat-*.ndjson | head -n 1)

echo "[Filebeat] Checking logs..." 
sudo tail -n 30 "$LATEST_LOG"
echo ""
echo "[Filebeat] Checking status..."
sudo systemctl status filebeat.service --no-pager
