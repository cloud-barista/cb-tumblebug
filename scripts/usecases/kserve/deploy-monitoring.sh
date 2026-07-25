#!/bin/bash

# Monitoring Stack for GPU/LLM serving (run on K8s control plane)
# Installs kube-prometheus-stack (Prometheus + Grafana + Alertmanager) and wires up
# the metrics already exposed by the cluster: DCGM (GPU) and vLLM/KServe (LLM).
# Grafana is exposed on NodePort 30300 (open the port in the Security Group).

set -e

NS="monitoring"
GRAFANA_NODEPORT="30300"
GRAFANA_PASSWORD="admin"

while [[ $# -gt 0 ]]; do
    case $1 in
        --nodeport) GRAFANA_NODEPORT="$2"; shift 2 ;;
        --password) GRAFANA_PASSWORD="$2"; shift 2 ;;
        *) echo "Usage: $0 [--nodeport 30300] [--password admin]"; exit 1 ;;
    esac
done

echo "==== Monitoring Stack Setup (Prometheus + Grafana) ===="

if ! command -v helm > /dev/null 2>&1; then
    curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash > /dev/null
fi

helm repo add prometheus-community https://prometheus-community.github.io/helm-charts > /dev/null 2>&1 || true
helm repo update > /dev/null 2>&1 || true

echo "Installing kube-prometheus-stack (this may take 3-5 minutes)..."
helm upgrade --install monitoring prometheus-community/kube-prometheus-stack \
    --namespace ${NS} --create-namespace \
    --set grafana.service.type=NodePort \
    --set grafana.service.nodePort=${GRAFANA_NODEPORT} \
    --set grafana.adminPassword="${GRAFANA_PASSWORD}" \
    --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false \
    --set prometheus.prometheusSpec.podMonitorSelectorNilUsesHelmValues=false \
    --wait --timeout 10m 2>&1 | tail -1
echo "  ✓ kube-prometheus-stack installed"

# Scrape DCGM exporter (deployed by GPU Operator) for GPU metrics
cat <<EOF | kubectl apply -f -
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: nvidia-dcgm-exporter
  namespace: ${NS}
spec:
  namespaceSelector:
    matchNames: ["gpu-operator"]
  selector:
    matchLabels:
      app: nvidia-dcgm-exporter
  endpoints:
    - targetPort: 9400
      interval: 15s
EOF

# Scrape KServe predictor pods (vLLM /metrics: TTFT, KV cache usage, tokens/s, ...)
cat <<EOF | kubectl apply -f -
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: kserve-predictors
  namespace: ${NS}
spec:
  namespaceSelector:
    any: true
  selector:
    matchExpressions:
      - { key: serving.kserve.io/inferenceservice, operator: Exists }
  # No port specified: predictor Services have one dynamically-named port and
  # pods declare no containerPort, so numeric targetPort matching fails
  endpoints:
    - path: /metrics
      interval: 15s
EOF
echo "  ✓ GPU (DCGM) and KServe/vLLM scrape targets configured"

# Grafana dashboards via sidecar (ConfigMaps labeled grafana_dashboard)
WORKDIR=$(mktemp -d)
if curl -fsSL "https://grafana.com/api/dashboards/12239/revisions/latest/download" -o "${WORKDIR}/dcgm.json" 2>/dev/null; then
    sed -i 's/${DS_PROMETHEUS}/Prometheus/g' "${WORKDIR}/dcgm.json"
    kubectl create configmap grafana-dashboard-dcgm -n ${NS} --from-file="${WORKDIR}/dcgm.json" \
        --dry-run=client -o yaml | kubectl apply -f - > /dev/null
    kubectl label configmap grafana-dashboard-dcgm -n ${NS} grafana_dashboard=1 --overwrite > /dev/null
    echo "  ✓ NVIDIA DCGM dashboard imported"
fi
if curl -fsSL "https://raw.githubusercontent.com/vllm-project/vllm/main/examples/observability/prometheus_grafana/grafana.json" -o "${WORKDIR}/vllm.json" 2>/dev/null; then
    sed -i 's/${DS_PROMETHEUS}/Prometheus/g' "${WORKDIR}/vllm.json"
    kubectl create configmap grafana-dashboard-vllm -n ${NS} --from-file="${WORKDIR}/vllm.json" \
        --dry-run=client -o yaml | kubectl apply -f - > /dev/null
    kubectl label configmap grafana-dashboard-vllm -n ${NS} grafana_dashboard=1 --overwrite > /dev/null
    echo "  ✓ vLLM dashboard imported"
else
    echo "  ⚠ vLLM dashboard download failed (import manually in Grafana if needed)"
fi

echo ""
echo "========================================"
echo "SUCCESS: Monitoring stack is running"
echo "========================================"
echo ""
echo "[GRAFANA_NODEPORT]"
echo "${GRAFANA_NODEPORT}"
echo ""
echo "Access: http://<node-public-ip>:${GRAFANA_NODEPORT} (allow ${GRAFANA_NODEPORT}/tcp in the Security Group)"
echo "  Login: admin / ${GRAFANA_PASSWORD}"
echo "  Dashboards: NVIDIA DCGM Exporter (GPU), vLLM (LLM serving)"
echo ""
echo "\$\$CMD[Check Monitoring](kubectl get pods -n ${NS})"
exit 0
