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
  "version": 2,
  "refresh": "10s",
  "time": {
    "from": "now-30m",
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
      },
      {
        "name": "gpu",
        "type": "query",
        "label": "GPU",
        "datasource": {
          "type": "prometheus",
          "uid": "prometheus"
        },
        "query": "label_values(DCGM_FI_DEV_GPU_UTIL{job=\"gpu_vm_telegraf_gateway\",instance=~\"$instance\"}, gpu)",
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
      "title": "Active Instances",
      "gridPos": {
        "h": 4,
        "w": 4,
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
      "title": "Avg CPU Usage (%)",
      "gridPos": {
        "h": 4,
        "w": 5,
        "x": 4,
        "y": 0
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "100 - (avg(rate(node_cpu_seconds_total{job=\"gpu_vm_telegraf_gateway\",mode=\"idle\",instance=~\"$instance\"}[2m])) * 100)",
          "refId": "A"
        }
      ]
    },
    {
      "id": 3,
      "type": "stat",
      "title": "Avg Memory Usage (%)",
      "gridPos": {
        "h": 4,
        "w": 5,
        "x": 9,
        "y": 0
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "100 * (1 - avg(node_memory_MemAvailable_bytes{job=\"gpu_vm_telegraf_gateway\",instance=~\"$instance\"} / node_memory_MemTotal_bytes{job=\"gpu_vm_telegraf_gateway\",instance=~\"$instance\"}))",
          "refId": "A"
        }
      ]
    },
    {
      "id": 4,
      "type": "stat",
      "title": "Avg GPU Utilization (%)",
      "gridPos": {
        "h": 4,
        "w": 5,
        "x": 14,
        "y": 0
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "avg(DCGM_FI_DEV_GPU_UTIL{job=\"gpu_vm_telegraf_gateway\",instance=~\"$instance\",gpu=~\"$gpu\"})",
          "refId": "A"
        }
      ]
    },
    {
      "id": 5,
      "type": "stat",
      "title": "Avg GPU Memory Usage (%)",
      "gridPos": {
        "h": 4,
        "w": 5,
        "x": 19,
        "y": 0
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "100 * avg(DCGM_FI_DEV_FB_USED{job=\"gpu_vm_telegraf_gateway\",instance=~\"$instance\",gpu=~\"$gpu\"} / DCGM_FI_DEV_FB_TOTAL{job=\"gpu_vm_telegraf_gateway\",instance=~\"$instance\",gpu=~\"$gpu\"})",
          "refId": "A"
        }
      ]
    },
    {
      "id": 6,
      "type": "timeseries",
      "title": "GPU Utilization by GPU (%)",
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 0,
        "y": 4
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "avg by (instance, gpu) (DCGM_FI_DEV_GPU_UTIL{job=\"gpu_vm_telegraf_gateway\",instance=~\"$instance\",gpu=~\"$gpu\"})",
          "legendFormat": "{{instance}} gpu{{gpu}}",
          "refId": "A"
        }
      ]
    },
    {
      "id": 7,
      "type": "timeseries",
      "title": "GPU Memory Used (MiB)",
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 12,
        "y": 4
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "DCGM_FI_DEV_FB_USED{job=\"gpu_vm_telegraf_gateway\",instance=~\"$instance\",gpu=~\"$gpu\"}",
          "legendFormat": "{{instance}} gpu{{gpu}}",
          "refId": "A"
        }
      ]
    },
    {
      "id": 8,
      "type": "timeseries",
      "title": "GPU Power Draw (W)",
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 0,
        "y": 12
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "DCGM_FI_DEV_POWER_USAGE{job=\"gpu_vm_telegraf_gateway\",instance=~\"$instance\",gpu=~\"$gpu\"}",
          "legendFormat": "{{instance}} gpu{{gpu}}",
          "refId": "A"
        }
      ]
    },
    {
      "id": 9,
      "type": "timeseries",
      "title": "CPU Usage by VM (%)",
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 12,
        "y": 12
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "100 - (avg by (instance) (rate(node_cpu_seconds_total{job=\"gpu_vm_telegraf_gateway\",mode=\"idle\",instance=~\"$instance\"}[2m])) * 100)",
          "legendFormat": "{{instance}}",
          "refId": "A"
        }
      ]
    },
    {
      "id": 10,
      "type": "timeseries",
      "title": "Memory Usage by VM (%)",
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 0,
        "y": 20
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "100 * (1 - (node_memory_MemAvailable_bytes{job=\"gpu_vm_telegraf_gateway\",instance=~\"$instance\"} / node_memory_MemTotal_bytes{job=\"gpu_vm_telegraf_gateway\",instance=~\"$instance\"}))",
          "legendFormat": "{{instance}}",
          "refId": "A"
        }
      ]
    },
    {
      "id": 11,
      "type": "timeseries",
      "title": "Network Throughput by VM (B/s)",
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 12,
        "y": 20
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "sum by (instance) (rate(node_network_receive_bytes_total{job=\"gpu_vm_telegraf_gateway\",device!~\"lo|docker.*|veth.*\",instance=~\"$instance\"}[1m]) + rate(node_network_transmit_bytes_total{job=\"gpu_vm_telegraf_gateway\",device!~\"lo|docker.*|veth.*\",instance=~\"$instance\"}[1m]))",
          "legendFormat": "{{instance}}",
          "refId": "A"
        }
      ]
    },
    {
      "id": 12,
      "type": "timeseries",
      "title": "vLLM Running Requests",
      "gridPos": {
        "h": 8,
        "w": 8,
        "x": 0,
        "y": 28
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "sum by (instance) ((vllm:num_requests_running{job=\"gpu_vm_telegraf_gateway\",instance=~\"$instance\"}) or (vllm_num_requests_running{job=\"gpu_vm_telegraf_gateway\",instance=~\"$instance\"}))",
          "legendFormat": "{{instance}}",
          "refId": "A"
        }
      ]
    },
    {
      "id": 13,
      "type": "timeseries",
      "title": "vLLM Prompt Throughput (tokens/s)",
      "gridPos": {
        "h": 8,
        "w": 8,
        "x": 8,
        "y": 28
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "sum by (instance) ((vllm:avg_prompt_throughput_toks_per_s{job=\"gpu_vm_telegraf_gateway\",instance=~\"$instance\"}) or (vllm_avg_prompt_throughput_toks_per_s{job=\"gpu_vm_telegraf_gateway\",instance=~\"$instance\"}))",
          "legendFormat": "{{instance}}",
          "refId": "A"
        }
      ]
    },
    {
      "id": 14,
      "type": "timeseries",
      "title": "vLLM Generation Throughput (tokens/s)",
      "gridPos": {
        "h": 8,
        "w": 8,
        "x": 16,
        "y": 28
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "sum by (instance) ((vllm:avg_generation_throughput_toks_per_s{job=\"gpu_vm_telegraf_gateway\",instance=~\"$instance\"}) or (vllm_avg_generation_throughput_toks_per_s{job=\"gpu_vm_telegraf_gateway\",instance=~\"$instance\"}))",
          "legendFormat": "{{instance}}",
          "refId": "A"
        }
      ]
    },
    {
      "id": 15,
      "type": "timeseries",
      "title": "vLLM Request Latency p95 (s)",
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 0,
        "y": 36
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "histogram_quantile(0.95, sum by (le, instance) (rate((vllm:request_latency_seconds_bucket{job=\"gpu_vm_telegraf_gateway\",instance=~\"$instance\"} or vllm_request_latency_seconds_bucket{job=\"gpu_vm_telegraf_gateway\",instance=~\"$instance\"})[5m])))",
          "legendFormat": "{{instance}}",
          "refId": "A"
        }
      ]
    },
    {
      "id": 16,
      "type": "timeseries",
      "title": "Target Availability (%)",
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 12,
        "y": 36
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "100 * avg(up{job=\"gpu_vm_telegraf_gateway\",instance=~\"$instance\"})",
          "legendFormat": "{{instance}}",
          "refId": "A"
        }
      ]
    },
    {
      "id": 17,
      "type": "stat",
      "title": "Loaded Ollama Models",
      "description": "Number of models currently loaded in Ollama memory (requires TelemetrySensor with Ollama running)",
      "gridPos": {
        "h": 4,
        "w": 6,
        "x": 0,
        "y": 44
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "count(ollama_model_size_vram{job=\"gpu_vm_telegraf_gateway\",instance=~\"$instance\"}) or vector(0)",
          "refId": "A"
        }
      ]
    },
    {
      "id": 18,
      "type": "stat",
      "title": "Total Ollama VRAM Used (MiB)",
      "description": "Sum of VRAM consumed by all loaded Ollama models across selected instances",
      "gridPos": {
        "h": 4,
        "w": 6,
        "x": 6,
        "y": 44
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "sum(ollama_model_size_vram{job=\"gpu_vm_telegraf_gateway\",instance=~\"$instance\"}) / 1048576 or vector(0)",
          "refId": "A"
        }
      ]
    },
    {
      "id": 19,
      "type": "timeseries",
      "title": "Ollama VRAM per Loaded Model (MiB)",
      "description": "VRAM used by each loaded Ollama model — label shows instance and model name",
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 0,
        "y": 48
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "ollama_model_size_vram{job=\"gpu_vm_telegraf_gateway\",instance=~\"$instance\"} / 1048576",
          "legendFormat": "{{instance}} · {{name}}",
          "refId": "A"
        }
      ]
    },
    {
      "id": 20,
      "type": "timeseries",
      "title": "Ollama Loaded Model Count",
      "description": "Number of models simultaneously loaded in Ollama per instance over time",
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 12,
        "y": 48
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "count by (instance) (ollama_model_size_vram{job=\"gpu_vm_telegraf_gateway\",instance=~\"$instance\"})",
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