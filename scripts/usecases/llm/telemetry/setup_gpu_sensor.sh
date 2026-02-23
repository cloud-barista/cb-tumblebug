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
sudo docker rm -f node-exporter dcgm-exporter telegraf 2>/dev/null || true

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

# 4. Start NVIDIA DCGM Exporter (GPU metrics, latest distroless)
echo "Starting NVIDIA DCGM Exporter..."
sudo docker run -d \
  --name dcgm-exporter \
  --gpus all \
  --cap-add SYS_ADMIN \
  -p 9400:9400 \
  --restart always \
  nvcr.io/nvidia/k8s/dcgm-exporter:4.5.2-4.8.1-distroless > /dev/null

# 5. Create Telegraf configuration
echo "Configuring Telegraf Gateway..."
mkdir -p ~/telegraf_config
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

# Scrape from Node(9100), DCGM(9400), and vLLM(8000)
[[inputs.prometheus]]
  urls = [
    "http://localhost:9100/metrics",
    "http://localhost:9400/metrics",
    "http://localhost:8000/metrics"
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
sudo docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep -E "node-exporter|dcgm-exporter|telegraf"