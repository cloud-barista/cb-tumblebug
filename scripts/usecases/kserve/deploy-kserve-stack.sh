#!/bin/bash

# KServe Stack Setup Script (run on K8s control plane)
# Installs: default StorageClass (local-path), Helm, NVIDIA GPU Operator,
#           cert-manager, KServe (RawDeployment mode; no Istio/Knative required)
# Prerequisites: K8s cluster deployed via k8s-control-plane-setup.sh / k8s-worker-setup.sh,
#                NVIDIA driver pre-installed on GPU workers (installGpuDriver.sh or GPU-ready image)
# Designed for unattended execution via SSH or pipe (curl | bash)

set -e

CERT_MANAGER_VERSION="v1.16.2"
KSERVE_VERSION="v0.19.0"
LOCAL_PATH_VERSION="v0.0.30"

echo "==== KServe Stack Setup ===="

if ! kubectl get nodes > /dev/null 2>&1; then
    echo "ERROR: kubectl cannot reach the cluster. Run this on the control plane."
    exit 1
fi

# ============================================================
# Default StorageClass (required by Open WebUI and model caches)
# ============================================================
if ! kubectl get storageclass 2>/dev/null | grep -q "(default)"; then
    echo "Installing local-path-provisioner as default StorageClass..."
    kubectl apply -f "https://raw.githubusercontent.com/rancher/local-path-provisioner/${LOCAL_PATH_VERSION}/deploy/local-path-storage.yaml" > /dev/null
    kubectl patch storageclass local-path -p '{"metadata":{"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}' > /dev/null
    kubectl rollout status deploy local-path-provisioner -n local-path-storage --timeout=3m > /dev/null
    echo "  ✓ local-path set as default StorageClass"
else
    echo "  ✓ Default StorageClass already present"
fi

# ============================================================
# Helm
# ============================================================
if ! command -v helm > /dev/null 2>&1; then
    echo "Installing Helm..."
    curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash > /dev/null
fi
echo "  ✓ Helm $(helm version --short 2>/dev/null)"

# ============================================================
# NVIDIA GPU Operator
# driver.enabled=false: NVIDIA driver is managed externally (pre-installed on GPU nodes)
# ============================================================
echo "Installing NVIDIA GPU Operator (this may take 5-10 minutes)..."
helm repo add nvidia https://helm.ngc.nvidia.com/nvidia > /dev/null 2>&1 || true
helm repo update > /dev/null 2>&1 || true
if helm upgrade --install gpu-operator nvidia/gpu-operator \
    --namespace gpu-operator --create-namespace \
    --set driver.enabled=false \
    --set toolkit.enabled=true \
    --set devicePlugin.enabled=true \
    --set mig.strategy=single \
    --wait --timeout 15m 2>&1 | tail -1; then
    echo "  ✓ NVIDIA GPU Operator installed"
else
    echo "  ⚠ GPU Operator install timed out or had warnings; check: kubectl get pods -n gpu-operator"
fi

# ============================================================
# cert-manager (KServe dependency)
# ============================================================
echo "Installing cert-manager ${CERT_MANAGER_VERSION}..."
kubectl apply -f "https://github.com/cert-manager/cert-manager/releases/download/${CERT_MANAGER_VERSION}/cert-manager.yaml" > /dev/null
kubectl wait --for=condition=Available deploy -n cert-manager --all --timeout=5m > /dev/null
echo "  ✓ cert-manager ready"

# ============================================================
# KServe (RawDeployment mode)
# ============================================================
echo "Installing KServe ${KSERVE_VERSION}..."
# kserve.yaml does not include the Namespace object — create it first
kubectl create namespace kserve --dry-run=client -o yaml | kubectl apply -f - > /dev/null
# kserve.yaml bundles CRDs and CRs using them (ClusterStorageContainer) — the CR
# can race CRD establishment ("no matches for kind"); apply, wait, re-apply
KSERVE_URL="https://github.com/kserve/kserve/releases/download/${KSERVE_VERSION}/kserve.yaml"
kubectl apply --server-side -f "${KSERVE_URL}" > /dev/null 2>&1 || true
kubectl wait --for=condition=Established crd --all --timeout=60s > /dev/null 2>&1 || true
kubectl apply --server-side -f "${KSERVE_URL}" > /dev/null
kubectl wait --for=condition=Available deploy -n kserve --all --timeout=5m > /dev/null
kubectl apply --server-side -f "https://github.com/kserve/kserve/releases/download/${KSERVE_VERSION}/kserve-cluster-resources.yaml" > /dev/null

# RawDeployment: plain Deployments/Services instead of Knative/Istio
kubectl patch configmap inferenceservice-config -n kserve --type merge \
    -p '{"data":{"deploy":"{\"defaultDeploymentMode\":\"RawDeployment\"}"}}' > /dev/null
kubectl rollout restart deploy kserve-controller-manager -n kserve > /dev/null
kubectl rollout status deploy kserve-controller-manager -n kserve --timeout=3m > /dev/null
echo "  ✓ KServe ready (RawDeployment mode)"

# ============================================================
# Summary
# ============================================================
echo ""
echo "========================================"
echo "SUCCESS: KServe stack is ready"
echo "========================================"
echo ""
echo "[KSERVE_GPU_RESOURCES]"
kubectl get nodes -o custom-columns='NAME:.metadata.name,GPU:.status.allocatable.nvidia\.com/gpu' 2>/dev/null || true
echo ""
echo "Note: GPU shows <none> until GPU Operator finishes configuring GPU workers (~2-5 min)."
echo "Next step: serve a model with serve-vllm-model.sh"
echo ""
echo "\$\$CMD[Check GPU Operator](kubectl get pods -n gpu-operator)"
echo "\$\$CMD[Check KServe](kubectl get pods -n kserve)"
exit 0
