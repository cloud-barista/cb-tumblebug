#!/bin/bash

# Enable Hubble UI (Cilium service map) — run on the control plane.
# Optional observability add-on for clusters deployed with --cni cilium.
# Exposes the UI on NodePort 30012 (open the SG port for your IP only).

set -e

NODEPORT="${NODEPORT:-30012}"

echo "==== Hubble UI Setup (NodePort: ${NODEPORT}) ===="

if ! kubectl get nodes > /dev/null 2>&1; then
    echo "ERROR: kubectl cannot reach the cluster. Run this on the control plane."
    exit 1
fi

if ! kubectl -n kube-system get daemonset cilium > /dev/null 2>&1; then
    echo "ERROR: this cluster is not running Cilium."
    echo "Deploy the control plane with: k8s-control-plane-setup.sh --cni cilium"
    exit 1
fi

if ! command -v cilium > /dev/null 2>&1; then
    echo "Installing cilium CLI..."
    CILIUM_CLI_VERSION=$(curl -fsSL https://raw.githubusercontent.com/cilium/cilium-cli/main/stable.txt)
    curl -fsSL "https://github.com/cilium/cilium-cli/releases/download/${CILIUM_CLI_VERSION}/cilium-linux-amd64.tar.gz" | sudo tar -xz -C /usr/local/bin
fi

echo "Enabling Hubble (+ UI)..."
cilium hubble enable --ui
kubectl -n kube-system rollout status deploy/hubble-relay --timeout=3m > /dev/null || true
kubectl -n kube-system rollout status deploy/hubble-ui --timeout=3m > /dev/null

echo "Exposing Hubble UI on NodePort ${NODEPORT}..."
kubectl -n kube-system patch svc hubble-ui --type merge \
    -p "{\"spec\":{\"type\":\"NodePort\",\"ports\":[{\"name\":\"http\",\"port\":80,\"targetPort\":8081,\"nodePort\":${NODEPORT}}]}}" > /dev/null

NODE_IP=$(hostname -I | awk '{print $1}')
echo ""
echo "=== Smoke test via NodePort ==="
curl -fsS -o /dev/null -w "HTTP %{http_code}\n" "http://${NODE_IP}:${NODEPORT}/" || true

echo ""
echo "========================================"
echo "SUCCESS: Hubble UI is ready"
echo "========================================"
echo ""
echo "SECURITY: open NodePort ${NODEPORT} in the security group for YOUR IP only."
echo "Disable: cilium hubble disable"
echo ""
echo "\$\$CMD[Check Hubble pods](kubectl get pods -n kube-system -l k8s-app=hubble-ui)"
exit 0
