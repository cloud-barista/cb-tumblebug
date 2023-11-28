#!/bin/bash

# This script should be run as root for full access to systemd logs and service management
if [ "$EUID" -ne 0 ]; then 
  echo "Please run as root"
  exit
fi


echo "[Status ELK Stack]"
# Replace with your actual log directory if different
LOG_DIR="/var/log"

echo "[Elasticsearch] Checking logs..." 
tail -n 30 "$LOG_DIR/elasticsearch/elasticsearch.log"
echo ""
echo "[Elasticsearch] Checking status..."
sudo systemctl status elasticsearch.service --no-pager

echo ""
echo "[Logstash] Checking logs..." 
tail -n 30 "$LOG_DIR/logstash/logstash-plain.log"
echo ""
echo "[Logstash] Checking status..." 
sudo systemctl status logstash.service --no-pager

echo ""
echo "[Kibana] Checking logs..." 
# Replace with your actual log directory if different
tail -n 30 "$LOG_DIR/kibana/kibana.log"
echo ""
echo "[Kibana] Checking status..." 
sudo systemctl status kibana.service --no-pager
