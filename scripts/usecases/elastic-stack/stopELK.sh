#!/bin/bash

# This script should be run as root for full access to systemd logs and service management
if [ "$EUID" -ne 0 ]; then 
  echo "Please run as root"
  exit
fi

echo "[Stop ELK Stack]"
# Stop the systemd service
systemctl stop elasticsearch.service
systemctl stop logstash.service
systemctl stop kibana.service

echo "ELK Stack services have been stopped."

# Optionally, check to confirm the service is stopped
echo "[Systemd service status of ELK Stack]"
sudo systemctl status elasticsearch.service --no-pager
sudo systemctl status logstash.service --no-pager
sudo systemctl status kibana.service --no-pager
