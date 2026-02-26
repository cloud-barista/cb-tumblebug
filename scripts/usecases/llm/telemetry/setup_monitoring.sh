#!/bin/bash
set -e

if [ $# -eq 0 ]; then
  echo "❌ Error: At least one GPU VM IP is required."
  echo "👉 Usage: bash setup_monitoring.sh <GPU_VM_IP1> <GPU_VM_IP2> ..."
  echo "👉 Example: bash setup_monitoring.sh 104.42.74.157 3.96.201.235"
  exit 1
fi

GPU_VM_IPS=("$@")

echo "=========================================="
echo "📊 Installing LLM Monitoring Stack"
echo "Target GPU VMs: ${#GPU_VM_IPS[@]} nodes"
for ip in "${GPU_VM_IPS[@]}"; do
  echo "  - $ip"
done
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

# Build targets array
TARGETS=""
for ip in "${GPU_VM_IPS[@]}"; do
  if [ -z "$TARGETS" ]; then
    TARGETS="'${ip}:9101'"
  else
    TARGETS="${TARGETS}, '${ip}:9101'"
  fi
done

cat <<EOF > prometheus.yml
global:
  scrape_interval: 5s

scrape_configs:
  # Single Gateway Target: Telegraf automatically provides Node, DCGM, and vLLM data
  - job_name: 'gpu_vm_telegraf_gateway'
    static_configs:
      - targets: [${TARGETS}]
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