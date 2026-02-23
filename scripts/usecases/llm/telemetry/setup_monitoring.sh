#!/bin/bash
set -e

GPU_VM_IP="${1}"

if [ -z "$GPU_VM_IP" ]; then
  echo "❌ Error: Target GPU VM IP is required."
  echo "👉 Usage: bash setup_monitoring.sh <GPU_VM_IP>"
  exit 1
fi

echo "=========================================="
echo "📊 Installing LLM Monitoring Stack"
echo "Target GPU VM IP: $GPU_VM_IP"
echo "=========================================="

# 1. Install Docker if missing
if ! command -v docker >/dev/null 2>&1; then
  curl -fsSL https://get.docker.com -o get-docker.sh
  sudo sh get-docker.sh
  sudo apt-get install -y docker-compose-plugin
fi

WORK_DIR="$HOME/llm_monitor"
mkdir -p "$WORK_DIR"
cd "$WORK_DIR"

# 2. Generate prometheus.yml (Scraping ONLY Telegraf 9101)
echo "Generating prometheus.yml..."
cat <<EOF > prometheus.yml
global:
  scrape_interval: 5s

scrape_configs:
  # Single Gateway Target: Telegraf automatically provides Node, DCGM, and vLLM data
  - job_name: 'gpu_vm_telegraf_gateway'
    static_configs:
      - targets: ['${GPU_VM_IP}:9101']
EOF

# 3. Generate docker-compose.yml
echo "Generating docker-compose.yml..."
cat <<EOF > docker-compose.yml
version: '3.8'
services:
  prometheus:
    image: prom/prometheus:latest
    container_name: prometheus
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
    restart: always

  grafana:
    image: grafana/grafana:latest
    container_name: grafana
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    restart: always
EOF

# 4. Run containers
echo "🚀 Starting Prometheus & Grafana..."
sudo docker compose up -d --force-recreate

echo ""
echo "=========================================="
echo "🎉 Monitoring Stack Setup Completed!"
echo "🌐 Check Targets: http://<MONITORING_VM_IP>:9090/targets"
echo "📈 Grafana: http://<MONITORING_VM_IP>:3000 (admin/admin)"
echo "=========================================="