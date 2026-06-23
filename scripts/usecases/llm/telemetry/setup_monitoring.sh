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

# Prepare Grafana provisioning directories
mkdir -p grafana/provisioning/datasources
mkdir -p grafana/provisioning/dashboards
mkdir -p grafana/dashboards

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

# 3. Generate Grafana datasource provisioning (Prometheus as default)
echo "Generating Grafana datasource provisioning..."
cat <<EOF > grafana/provisioning/datasources/prometheus.yml
apiVersion: 1

deleteDatasources:
  - name: Prometheus
    orgId: 1

datasources:
  - name: Prometheus
    uid: prometheus
    type: prometheus
    access: proxy
    orgId: 1
    url: http://prometheus:9090
    isDefault: true
    editable: false
    jsonData:
      httpMethod: POST
EOF

# 4. Generate Grafana dashboard provisioning
echo "Generating Grafana dashboard provisioning..."
cat <<EOF > grafana/provisioning/dashboards/llm-monitoring.yml
apiVersion: 1

providers:
  - name: 'LLM Monitoring'
    orgId: 1
    folder: 'LLM Monitoring'
    type: file
    disableDeletion: false
    editable: true
    options:
      path: /var/lib/grafana/dashboards
EOF

cat <<'EOF' > grafana/dashboards/llm-monitoring-overview.json
{
  "uid": "llm-monitoring-overview",
  "title": "LLM Monitoring Overview",
  "schemaVersion": 39,
  "version": 1,
  "refresh": "10s",
  "time": {
    "from": "now-15m",
    "to": "now"
  },
  "tags": ["llm", "prometheus", "telegraf"],
  "templating": {
    "list": [
      {
        "name": "instance",
        "type": "query",
        "label": "Instance",
        "datasource": {
          "type": "prometheus",
          "uid": "prometheus"
        },
        "query": "label_values(up{job=\"gpu_vm_telegraf_gateway\"}, instance)",
        "includeAll": true,
        "multi": true,
        "current": {
          "selected": true,
          "text": "All",
          "value": "$__all"
        }
      }
    ]
  },
  "panels": [
    {
      "id": 1,
      "type": "stat",
      "title": "Healthy Targets",
      "gridPos": {
        "h": 5,
        "w": 6,
        "x": 0,
        "y": 0
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "sum(up{job=\"gpu_vm_telegraf_gateway\",instance=~\"$instance\"})",
          "refId": "A"
        }
      ]
    },
    {
      "id": 2,
      "type": "stat",
      "title": "Total Targets",
      "gridPos": {
        "h": 5,
        "w": 6,
        "x": 6,
        "y": 0
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "count(up{job=\"gpu_vm_telegraf_gateway\",instance=~\"$instance\"})",
          "refId": "A"
        }
      ]
    },
    {
      "id": 3,
      "type": "timeseries",
      "title": "Target Availability (%)",
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 12,
        "y": 0
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "100 * avg(up{job=\"gpu_vm_telegraf_gateway\",instance=~\"$instance\"})",
          "refId": "A"
        }
      ]
    },
    {
      "id": 4,
      "type": "timeseries",
      "title": "Scrape Samples (per target)",
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 0,
        "y": 8
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "scrape_samples_post_metric_relabeling{job=\"gpu_vm_telegraf_gateway\",instance=~\"$instance\"}",
          "legendFormat": "{{instance}}",
          "refId": "A"
        }
      ]
    },
    {
      "id": 5,
      "type": "timeseries",
      "title": "Scrape Duration (s)",
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 12,
        "y": 8
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "scrape_duration_seconds{job=\"gpu_vm_telegraf_gateway\",instance=~\"$instance\"}",
          "legendFormat": "{{instance}}",
          "refId": "A"
        }
      ]
    }
  ]
}
EOF

# 5. Generate docker-compose.yml
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
    volumes:
      - ./grafana/provisioning:/etc/grafana/provisioning
      - ./grafana/dashboards:/var/lib/grafana/dashboards
      - grafana-storage:/var/lib/grafana
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    restart: always

volumes:
  grafana-storage:
EOF

# 6. Run containers
echo "🚀 Starting Prometheus & Grafana..."
sudo docker compose up -d --force-recreate

echo ""
echo "=========================================="
echo "🎉 Monitoring Stack Setup Completed!"
echo "🌐 Check Targets: http://<MONITORING_VM_IP>:9090/targets"
echo "📈 Grafana: http://<MONITORING_VM_IP>:3000 (admin/admin)"
echo "📊 Dashboard: LLM Monitoring Overview (auto-provisioned)"
echo "=========================================="
echo "\$\$ENDPOINT[Prometheus](http://0.0.0.0:9090/targets)"
echo "\$\$ENDPOINT[Grafana](http://0.0.0.0:3000)"
echo "\$\$CREDENTIAL[Grafana login](admin / admin)"