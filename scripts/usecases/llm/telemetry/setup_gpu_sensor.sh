#!/bin/bash
set -e

echo "Starting GPU VM Monitoring Setup (All-in-Docker Gateway)..."

# 1. Check Docker dependency
if ! command -v docker >/dev/null 2>&1; then
  echo "Error: Docker is required but not installed."
  exit 1
fi

# 2. Clean up existing containers
echo "Cleaning up old containers..."
sudo docker rm -f node-exporter dcgm-exporter rocm-exporter telegraf 2>/dev/null || true

# 2.5. Detect GPU type
echo "Detecting GPU type..."
GPU_TYPE="none"
GPU_PORT=""

if command -v nvidia-smi >/dev/null 2>&1 && nvidia-smi >/dev/null 2>&1; then
  GPU_TYPE="nvidia"
  GPU_PORT="9400"
  echo "✓ NVIDIA GPU detected"
elif [ -e /dev/kfd ] && [ -e /dev/dri ]; then
  GPU_TYPE="amd"
  GPU_PORT="9400"
  echo "✓ AMD GPU detected"
else
  echo "⚠ No GPU detected - skipping GPU monitoring"
fi

# 3. Start Node Exporter (System metrics)
echo "Starting Node Exporter..."
sudo docker run -d \
  --name node-exporter \
  --network host \
  --pid="host" \
  --restart always \
  -v "/:/host:ro,rslave" \
  prom/node-exporter:latest \
  --path.rootfs=/host > /dev/null

# 4. Start GPU-specific Exporter
if [ "$GPU_TYPE" = "nvidia" ]; then
  echo "Starting NVIDIA DCGM Exporter..."
  sudo docker run -d \
    --name dcgm-exporter \
    --gpus all \
    --cap-add SYS_ADMIN \
    -p 9400:9400 \
    --restart always \
    nvcr.io/nvidia/k8s/dcgm-exporter:4.5.2-4.8.1-distroless > /dev/null
  echo "✓ DCGM Exporter started on port 9400"
  
elif [ "$GPU_TYPE" = "amd" ]; then
  echo "Starting AMD Device Metrics Exporter (Official)..."
  sudo docker run -d \
    --name rocm-exporter \
    --device=/dev/kfd \
    --device=/dev/dri \
    --security-opt seccomp=unconfined \
    -v /sys:/sys:ro \
    -p 9400:5000 \
    --restart always \
    rocm/device-metrics-exporter:v1.4.2 > /dev/null
  echo "✓ ROCm Exporter started on port 9400"
fi

# 5. Create Telegraf configuration
echo "Configuring Telegraf Gateway..."
mkdir -p ~/telegraf_config

# Build Telegraf scrape URLs dynamically
SCRAPE_URLS='    "http://localhost:9100/metrics"'
if [ "$GPU_TYPE" != "none" ]; then
  SCRAPE_URLS="$SCRAPE_URLS,\n    \"http://localhost:9400/metrics\""
fi
SCRAPE_URLS="$SCRAPE_URLS,\n    \"http://localhost:8000/metrics\""

cat <<EOF > ~/telegraf_config/telegraf.conf
[agent]
  interval = "5s"
  round_interval = true
  metric_batch_size = 1000
  metric_buffer_limit = 10000
  collection_jitter = "0s"
  flush_interval = "5s"
  flush_jitter = "0s"
  omit_hostname = false

# Expose aggregated metrics on Port 9101
[[outputs.prometheus_client]]
  listen = ":9101"

# Scrape from Node(9100), GPU(9400 if detected), and vLLM(8000)
[[inputs.prometheus]]
  urls = [
$(echo -e "$SCRAPE_URLS")
  ]
EOF

# 6. Start Telegraf Gateway (Docker)
echo "Starting Telegraf Gateway..."
sudo docker run -d \
  --name telegraf \
  --network host \
  --restart always \
  -v $HOME/telegraf_config/telegraf.conf:/etc/telegraf/telegraf.conf:ro \
  telegraf > /dev/null

echo "Setup completed successfully. Gateway is running on Port 9101."
echo "Active containers:"
if [ "$GPU_TYPE" = "nvidia" ]; then
  sudo docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep -E "node-exporter|dcgm-exporter|telegraf"
elif [ "$GPU_TYPE" = "amd" ]; then
  sudo docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep -E "node-exporter|rocm-exporter|telegraf"
else
  sudo docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep -E "node-exporter|telegraf"
fi