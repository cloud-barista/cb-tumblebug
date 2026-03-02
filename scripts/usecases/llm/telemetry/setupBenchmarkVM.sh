#!/bin/bash
# Benchmark VM Setup: Monitoring + Tools
# Usage: curl -fsSL <url> | bash -s -- <GPU_VM_IP1> [GPU_VM_IP2] ...

if [ -z "$BASH_VERSION" ]; then
  [ ! -t 0 ] && echo "Error: Use 'bash' not 'sh'" && exit 1
  exec /bin/bash "$0" "$@"
fi

set -e

# Validate GPU VM IPs
if [ $# -eq 0 ]; then
  echo "Error: At least one GPU VM IP is required"
  echo "Usage: bash setupBenchmarkVM.sh <GPU_VM_IP1> [GPU_VM_IP2] ..."
  echo "Example: bash setupBenchmarkVM.sh 1.1.1.1 2.2.2.2"
  exit 1
fi

GPU_VM_IPS=("$@")

GITHUB_BASE="https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/telemetry"
SETUP_MONITORING_URL="${GITHUB_BASE}/setup_monitoring.sh"
EXPORT_METRICS_URL="${GITHUB_BASE}/export_metrics.sh"
RUN_GUIDELLM_URL="${GITHUB_BASE}/run_guidellm.sh"

echo "=========================================="
echo "Benchmark VM Setup"
echo "Target GPU VMs: ${#GPU_VM_IPS[@]} nodes"
for ip in "${GPU_VM_IPS[@]}"; do
  echo "  - $ip"
done
echo "=========================================="
echo ""

# Step 1: Setup Monitoring (Prometheus + Grafana)
echo "[1/3] Setting up monitoring stack..."
curl -fsSL "$SETUP_MONITORING_URL" | bash -s -- "${GPU_VM_IPS[@]}" || { echo "✗ Monitoring setup failed"; exit 1; }
echo "✓ Monitoring configured"

# Step 2: Download export_metrics.sh
echo "[2/3] Downloading export_metrics.sh..."
curl -fsSL "$EXPORT_METRICS_URL" -o ~/export_metrics.sh
chmod +x ~/export_metrics.sh
echo "✓ export_metrics.sh ready (~/export_metrics.sh)"

# Step 3: Download run_guidellm.sh
echo "[3/3] Downloading run_guidellm.sh..."
curl -fsSL "$RUN_GUIDELLM_URL" -o ~/run_guidellm.sh
chmod +x ~/run_guidellm.sh
echo "✓ run_guidellm.sh ready (~/run_guidellm.sh)"

echo ""
echo "✓ Setup Complete!"
echo "  Prometheus: http://<YOUR_VM_IP>:9090"
echo "  Grafana: http://<YOUR_VM_IP>:3000 (admin/admin)"
echo ""