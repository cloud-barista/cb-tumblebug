#!/bin/bash

# llm-d Deployment Script for Kubernetes
# This script deploys llm-d (distributed LLM inference) on a K8s cluster
# following the official llm-d deployment method using helmfile.
#
# llm-d deploys 3 Helm charts via helmfile:
#   1. llm-d-infra:       Gateway infrastructure (Istio/kgateway)
#   2. inferencepool:     Gateway API Inference Extension (EPP)
#   3. llm-d-modelservice: vLLM model server (GPU pods)
#
# Reference: https://llm-d.ai/docs/guide/Installation/inference-scheduling
#
# Prerequisites (installed by k8s-control-plane-setup.sh --llm-d):
#   - Kubernetes 1.29+ cluster with kubectl access
#   - Gateway API CRDs v1.4.0+
#   - Gateway API Inference Extension CRDs v1.3.0+
#   - Gateway control plane (Istio or kgateway)
#   - NVIDIA GPU Operator (for GPU nodes)
#   - Helm v3.12+, helmfile v1.1+, yq v4+
#   - GPU worker nodes with NVIDIA drivers
#
# Usage:
#   ./deploy-llm-d.sh                              # Deploy with defaults (Qwen3-32B, 8 replicas)
#   ./deploy-llm-d.sh --replicas 1 --tp 1          # Minimal: 1 replica, 1 GPU
#   ./deploy-llm-d.sh --hf-token YOUR_TOKEN         # Provide HuggingFace token
#   ./deploy-llm-d.sh --check                       # Check prerequisites only
#   ./deploy-llm-d.sh --uninstall                   # Remove deployment
#
# Remote execution (CB-MapUI / CB-Tumblebug API):
#   This script is designed for non-interactive SSH execution.

set -e

# ============================================================
# Non-interactive mode for SSH remote execution
# ============================================================
export DEBIAN_FRONTEND=noninteractive

# ============================================================
# Configuration (defaults match llm-d inference-scheduling guide)
# ============================================================
LLM_D_NAMESPACE="llm-d"
LLM_D_VERSION="main"           # Git branch/tag for llm-d repo (main or release tag)
LLM_D_GUIDE="inference-scheduling"  # Guide to deploy
MODEL_NAME=""                   # Override model (default: Qwen/Qwen3-32B from guide)
REPLICAS=""                     # Override replica count (default: 8 from guide)
TENSOR_PARALLEL=""              # Override tensor parallelism (default: 2 from guide)
HF_TOKEN=""                     # HuggingFace token for model download
HF_TOKEN_NAME="llm-d-hf-token" # Secret name for HF token
GATEWAY_PROVIDER=""             # Gateway provider: istio (default), kgateway, gke, etc.
HARDWARE=""                     # Hardware backend: cuda (default), amd, xpu, cpu, etc.
NODEPORT=""                     # NodePort number to expose gateway (e.g., 30080). Empty = ClusterIP.
CHECK_ONLY=false
UNINSTALL=false
LLM_D_REPO_DIR=""               # Will be set to cloned repo path

# ============================================================
# Parse arguments
# ============================================================
while [[ $# -gt 0 ]]; do
    case $1 in
        --model|-m)
            MODEL_NAME="$2"
            shift 2
            ;;
        --replicas|-r)
            REPLICAS="$2"
            shift 2
            ;;
        --tp|--tensor-parallel)
            TENSOR_PARALLEL="$2"
            shift 2
            ;;
        --version|-v)
            LLM_D_VERSION="$2"
            shift 2
            ;;
        --namespace|-n)
            LLM_D_NAMESPACE="$2"
            shift 2
            ;;
        --hf-token)
            HF_TOKEN="$2"
            shift 2
            ;;
        --gateway)
            GATEWAY_PROVIDER="$2"
            shift 2
            ;;
        --hardware)
            HARDWARE="$2"
            shift 2
            ;;
        --guide)
            LLM_D_GUIDE="$2"
            shift 2
            ;;
        --nodeport)
            # If next arg looks like another flag or is missing, use default
            if [ -z "$2" ] || [[ "$2" == -* ]]; then
                NODEPORT="30080"
                shift
            else
                NODEPORT="$2"
                shift 2
            fi
            ;;
        --check)
            CHECK_ONLY=true
            shift
            ;;
        --uninstall)
            UNINSTALL=true
            shift
            ;;
        -h|--help)
            echo "llm-d Deployment Script (helmfile-based)"
            echo ""
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  -m, --model MODEL        Override model (default: Qwen/Qwen3-32B from guide)"
            echo "  -r, --replicas N          Override decode replica count (default: 8)"
            echo "      --tp N                Override tensor parallelism (default: 2)"
            echo "  -n, --namespace NS        Kubernetes namespace (default: llm-d)"
            echo "  -v, --version VER         llm-d repo version/branch (default: main)"
            echo "      --hf-token TOKEN      HuggingFace token for model download"
            echo "      --gateway PROVIDER    Gateway: istio (default), kgateway, gke, digitalocean"
            echo "      --hardware HW         Hardware: cuda (default), amd, xpu, cpu, hpu, tpu"
            echo "      --guide GUIDE         Guide to deploy (default: inference-scheduling)"
            echo "      --nodeport [PORT]     Expose gateway via NodePort (default: 30080)"
            echo "      --check               Check prerequisites only, don't deploy"
            echo "      --uninstall           Remove llm-d deployment"
            echo "  -h, --help                Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                                         # Deploy with guide defaults"
            echo "  $0 --replicas 1 --tp 1                     # Minimal 1-GPU deployment"
            echo "  $0 --hf-token hf_xxxxx                     # Provide HF token"
            echo "  $0 --model Qwen/Qwen3-0.6B --replicas 1 --tp 1  # Small model, 1 GPU"
            echo "  $0 --gateway kgateway                      # Use kgateway instead of istio"
            echo "  $0 --hardware amd                          # Deploy on AMD GPUs"
            echo "  $0 --nodeport 30080                          # Expose via NodePort on port 30080"
            echo "  $0 --check                                 # Verify prerequisites"
            echo "  $0 --uninstall                             # Remove deployment"
            echo ""
            echo "Architecture (3 Helm charts deployed via helmfile):"
            echo "  1. llm-d-infra:        Gateway + infrastructure"
            echo "  2. inferencepool:      Inference scheduler (EPP)"
            echo "  3. llm-d-modelservice: vLLM model server pods"
            echo ""
            echo "Prerequisites (install on control plane first):"
            echo "  k8s-control-plane-setup.sh --llm-d"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use -h or --help for usage information"
            exit 1
            ;;
    esac
done

echo "=========================================="
echo "llm-d Deployment"
echo "=========================================="
echo "Namespace: $LLM_D_NAMESPACE"
echo "Version: $LLM_D_VERSION"
echo "Guide: $LLM_D_GUIDE"
if [ -n "$MODEL_NAME" ]; then
    echo "Model Override: $MODEL_NAME"
fi
if [ -n "$REPLICAS" ]; then
    echo "Replicas Override: $REPLICAS"
fi
if [ -n "$TENSOR_PARALLEL" ]; then
    echo "Tensor Parallel Override: $TENSOR_PARALLEL"
fi
if [ -n "$GATEWAY_PROVIDER" ]; then
    echo "Gateway: $GATEWAY_PROVIDER"
fi
if [ -n "$HARDWARE" ]; then
    echo "Hardware: $HARDWARE"
fi
if [ -n "$NODEPORT" ]; then
    echo "Gateway Exposure: NodePort ($NODEPORT)"
fi
echo "=========================================="
echo ""

# ============================================================
# Check Prerequisites
# ============================================================
echo "Checking prerequisites..."
PREREQ_FAILED=false
PREREQ_WARNINGS=0

# Check kubectl
if ! command -v kubectl &>/dev/null; then
    echo "  ✗ kubectl not found"
    PREREQ_FAILED=true
else
    echo "  ✓ kubectl available"
fi

# Check cluster connectivity
if ! kubectl cluster-info &>/dev/null; then
    echo "  ✗ Cannot connect to Kubernetes cluster"
    PREREQ_FAILED=true
else
    echo "  ✓ Kubernetes cluster accessible"
    K8S_VERSION=$(kubectl version -o json 2>/dev/null | grep -o '"gitVersion": "[^"]*"' | tail -1 | cut -d'"' -f4 || echo "unknown")
    echo "    Kubernetes version: ${K8S_VERSION}"
fi

# Check Helm
if ! command -v helm &>/dev/null; then
    echo "  ✗ Helm not found (v3.12+ required)"
    PREREQ_FAILED=true
else
    echo "  ✓ Helm available: $(helm version --short 2>/dev/null | head -1)"
fi

# Check helmfile
if ! command -v helmfile &>/dev/null; then
    echo "  ✗ helmfile not found (v1.1+ required)"
    echo "    Install: https://helmfile.readthedocs.io/en/latest/#installation"
    PREREQ_FAILED=true
else
    HELMFILE_VER=$(helmfile version 2>/dev/null | head -1 || echo "unknown")
    echo "  ✓ helmfile available: ${HELMFILE_VER}"
fi

# Check yq
if ! command -v yq &>/dev/null; then
    echo "  ✗ yq not found (v4+ required)"
    echo "    Install: https://github.com/mikefarah/yq#install"
    PREREQ_FAILED=true
else
    echo "  ✓ yq available: $(yq --version 2>/dev/null | head -1)"
fi

# Check git
if ! command -v git &>/dev/null; then
    echo "  ✗ git not found"
    PREREQ_FAILED=true
else
    echo "  ✓ git available"
fi

# Check Gateway API CRDs (v1.4.0+)
if kubectl get crd gateways.gateway.networking.k8s.io &>/dev/null; then
    echo "  ✓ Gateway API CRDs installed"
else
    echo "  ✗ Gateway API CRDs not found"
    echo "    Run: k8s-control-plane-setup.sh --llm-d"
    PREREQ_FAILED=true
fi

# Check Gateway API Inference Extension CRDs (InferencePool)
if kubectl get crd inferencepools.inference.networking.k8s.io &>/dev/null; then
    echo "  ✓ Gateway API Inference Extension CRDs installed"
else
    echo "  ✗ Gateway API Inference Extension CRDs not found (InferencePool)"
    echo "    Run: k8s-control-plane-setup.sh --llm-d"
    PREREQ_FAILED=true
fi

# Check Prometheus Operator CRDs (PodMonitor, ServiceMonitor — required by llm-d charts)
if kubectl get crd podmonitors.monitoring.coreos.com &>/dev/null && \
   kubectl get crd servicemonitors.monitoring.coreos.com &>/dev/null; then
    echo "  ✓ Prometheus Operator CRDs installed (PodMonitor, ServiceMonitor)"
else
    echo "  ⚠ Prometheus Operator CRDs not found (PodMonitor/ServiceMonitor)"
    echo "    Auto-installing lightweight CRDs..."
    PROM_OP_VERSION="v0.82.2"
    PROM_CRD_BASE="https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/${PROM_OP_VERSION}/example/prometheus-operator-crd"
    kubectl apply --server-side -f "${PROM_CRD_BASE}/monitoring.coreos.com_podmonitors.yaml" 2>&1 || true
    kubectl apply --server-side -f "${PROM_CRD_BASE}/monitoring.coreos.com_servicemonitors.yaml" 2>&1 || true
    if kubectl get crd podmonitors.monitoring.coreos.com &>/dev/null && \
       kubectl get crd servicemonitors.monitoring.coreos.com &>/dev/null; then
        echo "  ✓ Prometheus Operator CRDs installed successfully"
    else
        echo "  ✗ Failed to install Prometheus Operator CRDs"
        echo "    Run: k8s-control-plane-setup.sh --llm-d"
        PREREQ_FAILED=true
    fi
fi

# Check Gateway control plane (Istio or kgateway)
GATEWAY_DETECTED=""
if kubectl get pods -n istio-system 2>/dev/null | grep -q "Running"; then
    GATEWAY_DETECTED="istio"
    echo "  ✓ Gateway control plane: Istio"
elif kubectl get pods -n kgateway 2>/dev/null | grep -q "Running"; then
    GATEWAY_DETECTED="kgateway"
    echo "  ✓ Gateway control plane: kgateway"
else
    echo "  ✗ No Gateway control plane detected (Istio or kgateway)"
    echo "    Run: k8s-control-plane-setup.sh --llm-d"
    PREREQ_FAILED=true
fi

# Check GPU resources
GPU_COUNT=$(kubectl get nodes -o jsonpath='{.items[*].status.allocatable.nvidia\.com/gpu}' 2>/dev/null | tr ' ' '\n' | grep -v '^$' | awk '{sum+=$1} END {print sum}')
if [ -n "$GPU_COUNT" ] && [ "$GPU_COUNT" -gt 0 ] 2>/dev/null; then
    echo "  ✓ GPU resources available: ${GPU_COUNT} GPU(s)"
else
    echo "  ⚠ No NVIDIA GPU resources detected (may still work if GPUs are pending)"
    echo "    Check: kubectl describe nodes | grep nvidia.com/gpu"
    PREREQ_WARNINGS=$((PREREQ_WARNINGS + 1))
fi

# Check HuggingFace token secret
if kubectl get secret "$HF_TOKEN_NAME" -n "$LLM_D_NAMESPACE" &>/dev/null; then
    echo "  ✓ HuggingFace token secret exists in namespace $LLM_D_NAMESPACE"
elif [ -n "$HF_TOKEN" ]; then
    echo "  ✓ HuggingFace token provided via --hf-token (will create secret)"
else
    echo "  ⚠ HuggingFace token not found (secret: $HF_TOKEN_NAME in ns: $LLM_D_NAMESPACE)"
    echo "    Provide with --hf-token or create manually:"
    echo "    kubectl create secret generic $HF_TOKEN_NAME --from-literal=HF_TOKEN=<token> -n $LLM_D_NAMESPACE"
    PREREQ_WARNINGS=$((PREREQ_WARNINGS + 1))
fi

echo ""

if [ "$PREREQ_FAILED" = true ]; then
    echo "=========================================="
    echo "ERROR: Prerequisites not met"
    echo "=========================================="
    echo ""
    echo "Install prerequisites on the control plane first:"
    echo "  k8s-control-plane-setup.sh --llm-d"
    echo ""
    echo "Then install client tools on this machine:"
    echo "  # helmfile: https://helmfile.readthedocs.io/en/latest/#installation"
    echo "  # yq: https://github.com/mikefarah/yq#install"
    exit 1
fi

if [ "$PREREQ_WARNINGS" -gt 0 ]; then
    echo "  ($PREREQ_WARNINGS warning(s) - deployment may still proceed)"
    echo ""
fi

if [ "$CHECK_ONLY" = true ]; then
    echo "=========================================="
    echo "Prerequisites Check: PASSED"
    echo "=========================================="
    echo ""
    echo "All critical prerequisites are met. Run without --check to deploy."
    exit 0
fi

# ============================================================
# Uninstall mode
# ============================================================
if [ "$UNINSTALL" = true ]; then
    echo "Uninstalling llm-d from namespace $LLM_D_NAMESPACE..."

    # Try helmfile destroy first
    if [ -d ~/llm-d/guides/"$LLM_D_GUIDE" ]; then
        cd ~/llm-d/guides/"$LLM_D_GUIDE"
        helmfile destroy -n "$LLM_D_NAMESPACE" 2>&1 || true
    else
        # Manual helm uninstall
        echo "  llm-d repo not found, uninstalling helm releases manually..."
        for release in $(helm list -n "$LLM_D_NAMESPACE" -q 2>/dev/null); do
            echo "  Removing: $release"
            helm uninstall "$release" -n "$LLM_D_NAMESPACE" 2>&1 || true
        done
    fi

    echo ""
    echo "=========================================="
    echo "llm-d uninstalled from namespace $LLM_D_NAMESPACE"
    echo "=========================================="
    echo ""
    echo "To also delete the namespace:"
    echo "  kubectl delete namespace $LLM_D_NAMESPACE"
    exit 0
fi

# ============================================================
# Clone llm-d repo
# ============================================================
echo "Setting up llm-d repository..."
LLM_D_REPO_DIR=~/llm-d

if [ -d "$LLM_D_REPO_DIR/.git" ]; then
    echo "  Existing repo found, updating..."
    cd "$LLM_D_REPO_DIR"
    git fetch origin 2>/dev/null || true
    git checkout "$LLM_D_VERSION" 2>/dev/null || git checkout "origin/$LLM_D_VERSION" 2>/dev/null || true
    git pull origin "$LLM_D_VERSION" 2>/dev/null || true
else
    echo "  Cloning llm-d repo (branch: $LLM_D_VERSION)..."
    rm -rf "$LLM_D_REPO_DIR"
    git clone --depth 1 --branch "$LLM_D_VERSION" https://github.com/llm-d/llm-d.git "$LLM_D_REPO_DIR" 2>&1 || {
        # If branch doesn't exist, clone main and checkout
        git clone --depth 1 https://github.com/llm-d/llm-d.git "$LLM_D_REPO_DIR" 2>&1
        cd "$LLM_D_REPO_DIR"
        git checkout "$LLM_D_VERSION" 2>/dev/null || true
    }
fi

# Verify guide exists
GUIDE_DIR="$LLM_D_REPO_DIR/guides/$LLM_D_GUIDE"
if [ ! -d "$GUIDE_DIR" ]; then
    echo "ERROR: Guide directory not found: $GUIDE_DIR"
    echo ""
    echo "Available guides:"
    ls -1 "$LLM_D_REPO_DIR/guides/" 2>/dev/null | grep -v "prereq\|benchmark\|README\|\." || echo "  (none found)"
    exit 1
fi

echo "  ✓ Guide directory: $GUIDE_DIR"

# ============================================================
# Create namespace and HuggingFace token secret
# ============================================================
echo ""
echo "Setting up namespace and secrets..."

kubectl create namespace "$LLM_D_NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - > /dev/null 2>&1

# Create HF token secret if token is provided
if [ -n "$HF_TOKEN" ]; then
    echo "  Creating HuggingFace token secret..."
    kubectl create secret generic "$HF_TOKEN_NAME" \
        --from-literal="HF_TOKEN=${HF_TOKEN}" \
        --namespace "$LLM_D_NAMESPACE" \
        --dry-run=client -o yaml | kubectl apply -f - > /dev/null 2>&1
    echo "  ✓ HuggingFace token secret created: $HF_TOKEN_NAME"
elif ! kubectl get secret "$HF_TOKEN_NAME" -n "$LLM_D_NAMESPACE" &>/dev/null; then
    echo "  ⚠ No HuggingFace token - model download may fail for gated models"
fi

# ============================================================
# Apply value overrides
# ============================================================
echo ""
echo "Preparing deployment values..."

cd "$GUIDE_DIR"

# Determine values file based on hardware
VALUES_FILE="ms-${LLM_D_GUIDE}/values.yaml"
if [ -n "$HARDWARE" ] && [ "$HARDWARE" != "cuda" ]; then
    HW_VALUES="ms-${LLM_D_GUIDE}/values_${HARDWARE}.yaml"
    if [ -f "$HW_VALUES" ]; then
        VALUES_FILE="$HW_VALUES"
        echo "  Using hardware values: $HW_VALUES"
    else
        echo "  ⚠ Hardware values file not found: $HW_VALUES, using default"
    fi
fi

# Apply overrides to values file
OVERRIDES_APPLIED=false

if [ -n "$REPLICAS" ] || [ -n "$TENSOR_PARALLEL" ] || [ -n "$MODEL_NAME" ]; then
    OVERRIDES_APPLIED=true
fi

if [ "$OVERRIDES_APPLIED" = true ] && [ -f "$VALUES_FILE" ]; then
    echo "  Applying overrides to $VALUES_FILE..."
    # Backup original
    cp "$VALUES_FILE" "${VALUES_FILE}.bak"

    if [ -n "$MODEL_NAME" ]; then
        yq eval -i ".modelArtifacts.uri = \"hf://${MODEL_NAME}\"" "$VALUES_FILE" 2>/dev/null || true
        yq eval -i ".modelArtifacts.name = \"${MODEL_NAME}\"" "$VALUES_FILE" 2>/dev/null || true
        echo "  Model override: $MODEL_NAME"
    fi

    if [ -n "$REPLICAS" ]; then
        yq eval -i ".decode.replicas = ${REPLICAS}" "$VALUES_FILE" 2>/dev/null || true
        echo "  Replicas override: $REPLICAS"
    fi

    if [ -n "$TENSOR_PARALLEL" ]; then
        yq eval -i ".decode.parallelism.tensor = ${TENSOR_PARALLEL}" "$VALUES_FILE" 2>/dev/null || true
        echo "  Tensor parallelism override: $TENSOR_PARALLEL"
    fi
else
    echo "  Using guide defaults (no overrides)"
fi

# ============================================================
# Deploy with helmfile
# ============================================================
echo ""
echo "Deploying llm-d via helmfile..."
echo "  Directory: $GUIDE_DIR"

# Build helmfile command as array (avoid eval for safety)
HELMFILE_CMD=(helmfile apply -n "$LLM_D_NAMESPACE")

# Add environment flag for gateway/hardware
if [ -n "$GATEWAY_PROVIDER" ]; then
    HELMFILE_CMD+=(-e "$GATEWAY_PROVIDER")
    echo "  Environment: $GATEWAY_PROVIDER"
elif [ -n "$HARDWARE" ] && [ "$HARDWARE" != "cuda" ]; then
    HELMFILE_CMD+=(-e "$HARDWARE")
    echo "  Environment: $HARDWARE"
fi

echo ""
printf 'Running: '
printf '%s ' "${HELMFILE_CMD[@]}"
echo
echo "---"

# Execute helmfile apply
DEPLOY_SUCCESS=false
if "${HELMFILE_CMD[@]}" 2>&1; then
    DEPLOY_SUCCESS=true
fi

# Restore original values if we modified them
if [ "$OVERRIDES_APPLIED" = true ] && [ -f "${VALUES_FILE}.bak" ]; then
    mv "${VALUES_FILE}.bak" "$VALUES_FILE"
fi

echo "---"
echo ""

if [ "$DEPLOY_SUCCESS" = false ]; then
    echo "=========================================="
    echo "ERROR: helmfile apply failed"
    echo "=========================================="
    echo ""
    echo "Troubleshooting:"
    echo "  1. Check prerequisites: $0 --check"
    echo "  2. Check helm releases: helm list -n $LLM_D_NAMESPACE"
    echo "  3. Check pods: kubectl get pods -n $LLM_D_NAMESPACE"
    echo "  4. Check events: kubectl get events -n $LLM_D_NAMESPACE --sort-by='.lastTimestamp'"
    exit 1
fi

# ============================================================
# Install HTTPRoute
# ============================================================
echo "Installing HTTPRoute..."
HTTPROUTE_FILE="httproute.yaml"
if [ -n "$GATEWAY_PROVIDER" ] && [ "$GATEWAY_PROVIDER" = "gke" ]; then
    HTTPROUTE_FILE="httproute.gke.yaml"
fi

if [ -f "$HTTPROUTE_FILE" ]; then
    kubectl apply -f "$HTTPROUTE_FILE" -n "$LLM_D_NAMESPACE" 2>&1 || {
        echo "  ⚠ HTTPRoute installation failed (may need manual setup)"
    }
    echo "  ✓ HTTPRoute installed"
else
    echo "  ⚠ HTTPRoute file not found: $HTTPROUTE_FILE"
fi

# ============================================================
# Apply NodePort patch to gateway service (post-deploy)
# ============================================================
if [ -n "$NODEPORT" ]; then
    echo ""
    echo "Applying NodePort exposure to gateway service..."

    # Find the gateway service
    NP_SVC=$(kubectl get svc -n "$LLM_D_NAMESPACE" -o name 2>/dev/null | grep -i "gateway-istio\|gateway-kgateway\|gateway" | head -1 | sed 's|service/||')
    if [ -n "$NP_SVC" ]; then
        # Patch service type to NodePort and add nodePort to the first port
        # Use JSON patch to avoid overwriting the entire ports array
        kubectl patch svc "$NP_SVC" -n "$LLM_D_NAMESPACE" --type=json \
            -p "[{\"op\":\"replace\",\"path\":\"/spec/type\",\"value\":\"NodePort\"},{\"op\":\"add\",\"path\":\"/spec/ports/0/nodePort\",\"value\":${NODEPORT}}]" 2>&1 || {
            echo "  ⚠ JSON patch failed, trying merge patch on type only..."
            kubectl patch svc "$NP_SVC" -n "$LLM_D_NAMESPACE" --type=merge \
                -p "{\"spec\":{\"type\":\"NodePort\"}}" 2>&1 || true
        }

        # Verify
        ACTUAL_TYPE=$(kubectl get svc "$NP_SVC" -n "$LLM_D_NAMESPACE" -o jsonpath='{.spec.type}' 2>/dev/null)
        ACTUAL_NODEPORT=$(kubectl get svc "$NP_SVC" -n "$LLM_D_NAMESPACE" -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null)
        if [ "$ACTUAL_TYPE" = "NodePort" ]; then
            echo "  ✓ Gateway service exposed as NodePort (port: ${ACTUAL_NODEPORT:-$NODEPORT})"
        else
            echo "  ⚠ Gateway service type is still $ACTUAL_TYPE (NodePort patch may not have applied)"
        fi
    else
        echo "  ⚠ Gateway service not found yet (NodePort will need manual patching)"
    fi
fi

# ============================================================
# Wait for pods and verify
# ============================================================
echo ""
echo "Waiting for llm-d pods to be ready..."

DEPLOY_TIMEOUT=600  # 10 minutes
ELAPSED=0
INTERVAL=10

while [ $ELAPSED -lt $DEPLOY_TIMEOUT ]; do
    # Get pod status (exclude Completed jobs from totals)
    TOTAL_PODS=$(kubectl get pods -n "$LLM_D_NAMESPACE" --no-headers 2>/dev/null | grep -cv "Completed" 2>/dev/null) || TOTAL_PODS=0
    READY_PODS=$(kubectl get pods -n "$LLM_D_NAMESPACE" --no-headers 2>/dev/null | grep -v "Completed" | grep -c '\([0-9]\+\)/\1' 2>/dev/null) || READY_PODS=0
    PENDING_PODS=$(kubectl get pods -n "$LLM_D_NAMESPACE" --no-headers 2>/dev/null | grep -cE "Pending|ContainerCreating|Init" 2>/dev/null) || PENDING_PODS=0
    FAILED_PODS=$(kubectl get pods -n "$LLM_D_NAMESPACE" --no-headers 2>/dev/null | grep -cE "Error|CrashLoopBackOff|ImagePullBackOff" 2>/dev/null) || FAILED_PODS=0

    if [ "$TOTAL_PODS" -gt 0 ] && [ "$READY_PODS" -eq "$TOTAL_PODS" ]; then
        echo "  ✓ All pods ready ($READY_PODS/$TOTAL_PODS)"
        break
    fi

    if [ "$FAILED_PODS" -gt 0 ]; then
        echo "  ⚠ $FAILED_PODS pod(s) in error state"
        kubectl get pods -n "$LLM_D_NAMESPACE" --no-headers 2>/dev/null | grep -E "Error|CrashLoopBackOff|ImagePullBackOff" | head -3
    fi

    if [ $((ELAPSED % 30)) -eq 0 ]; then
        echo "  Waiting... ($READY_PODS/$TOTAL_PODS ready, $PENDING_PODS pending) [${ELAPSED}s/${DEPLOY_TIMEOUT}s]"
    fi

    sleep $INTERVAL
    ELAPSED=$((ELAPSED + INTERVAL))
done

if [ $ELAPSED -ge $DEPLOY_TIMEOUT ]; then
    echo "  ⚠ Timeout waiting for all pods (some may still be starting)"
fi

# ============================================================
# Verify Installation
# ============================================================
echo ""
echo "Verifying installation..."

# Check helm releases
echo ""
echo "Helm releases:"
helm list -n "$LLM_D_NAMESPACE" 2>/dev/null || echo "  (none found)"

# Check pods
echo ""
echo "Pods:"
kubectl get pods -n "$LLM_D_NAMESPACE" -o wide 2>/dev/null || echo "  (none found)"

# Check services
echo ""
echo "Services:"
kubectl get svc -n "$LLM_D_NAMESPACE" 2>/dev/null || echo "  (none found)"

# Check InferencePool
echo ""
echo "InferencePool:"
kubectl get inferencepool -n "$LLM_D_NAMESPACE" 2>/dev/null || echo "  (none found)"

# Check Gateway
echo ""
echo "Gateway:"
kubectl get gateway -n "$LLM_D_NAMESPACE" 2>/dev/null || echo "  (none found)"

# ============================================================
# Determine service info
# ============================================================
GATEWAY_SVC=$(kubectl get svc -n "$LLM_D_NAMESPACE" -o name 2>/dev/null | grep -i "gateway-istio\|gateway-kgateway\|gateway" | head -1 | sed 's|service/||')
GATEWAY_PORT=""
if [ -n "$GATEWAY_SVC" ]; then
    # Try port named "default" (Istio gateway uses this), then "http", then first port
    GATEWAY_PORT=$(kubectl get svc -n "$LLM_D_NAMESPACE" "$GATEWAY_SVC" -o jsonpath='{.spec.ports[?(@.name=="default")].port}' 2>/dev/null)
    if [ -z "$GATEWAY_PORT" ]; then
        GATEWAY_PORT=$(kubectl get svc -n "$LLM_D_NAMESPACE" "$GATEWAY_SVC" -o jsonpath='{.spec.ports[?(@.name=="http")].port}' 2>/dev/null)
    fi
    if [ -z "$GATEWAY_PORT" ]; then
        # Fallback: find port 80 among all ports, or use the first non-status port
        GATEWAY_PORT=$(kubectl get svc -n "$LLM_D_NAMESPACE" "$GATEWAY_SVC" -o jsonpath='{.spec.ports[*].port}' 2>/dev/null | tr ' ' '\n' | grep -x '80' | head -1)
    fi
    if [ -z "$GATEWAY_PORT" ]; then
        GATEWAY_PORT=$(kubectl get svc -n "$LLM_D_NAMESPACE" "$GATEWAY_SVC" -o jsonpath='{.spec.ports[0].port}' 2>/dev/null)
    fi
fi
GATEWAY_PORT=${GATEWAY_PORT:-80}

# Determine NodePort and external access info
GATEWAY_NODEPORT=""
GATEWAY_SVC_TYPE=""
if [ -n "$GATEWAY_SVC" ]; then
    GATEWAY_SVC_TYPE=$(kubectl get svc "$GATEWAY_SVC" -n "$LLM_D_NAMESPACE" -o jsonpath='{.spec.type}' 2>/dev/null)
    if [ "$GATEWAY_SVC_TYPE" = "NodePort" ]; then
        GATEWAY_NODEPORT=$(kubectl get svc "$GATEWAY_SVC" -n "$LLM_D_NAMESPACE" -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null)
    fi
fi

# Get node external IP for NodePort access
NODE_EXTERNAL_IP=""
if [ -n "$GATEWAY_NODEPORT" ]; then
    # Try ExternalIP first, then InternalIP
    NODE_EXTERNAL_IP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="ExternalIP")].address}' 2>/dev/null)
    if [ -z "$NODE_EXTERNAL_IP" ]; then
        NODE_EXTERNAL_IP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}' 2>/dev/null)
    fi
fi

# Determine deployed model name for reference
DEPLOYED_MODEL="${MODEL_NAME}"
if [ -z "$DEPLOYED_MODEL" ] && [ -f "$VALUES_FILE" ]; then
    DEPLOYED_MODEL=$(yq eval '.modelArtifacts.name // .modelArtifacts.uri' "$VALUES_FILE" 2>/dev/null | sed 's|^hf://||')
fi
DEPLOYED_MODEL=${DEPLOYED_MODEL:-"unknown"}

TOTAL_FINAL=$(kubectl get pods -n "$LLM_D_NAMESPACE" --no-headers 2>/dev/null | wc -l | tr -d ' ')
READY_FINAL=$(kubectl get pods -n "$LLM_D_NAMESPACE" --no-headers 2>/dev/null | grep -c "Running" 2>/dev/null) || READY_FINAL=0

# ============================================================
# Output structured results
# ============================================================
echo ""
if [ "$READY_FINAL" -gt 0 ] && [ "$READY_FINAL" -eq "$TOTAL_FINAL" ]; then
    echo "=========================================="
    echo "SUCCESS: llm-d deployment complete!"
    echo "=========================================="
else
    echo "=========================================="
    echo "llm-d deployment applied (pods still starting)"
    echo "=========================================="
fi

echo ""
echo "[LLM_D_NAMESPACE]"
echo "$LLM_D_NAMESPACE"
echo ""
echo "[LLM_D_GATEWAY_SERVICE]"
echo "${GATEWAY_SVC:-N/A}"
echo ""
echo "[LLM_D_ENDPOINT]"
if [ -n "$GATEWAY_SVC" ]; then
    echo "http://${GATEWAY_SVC}.${LLM_D_NAMESPACE}.svc.cluster.local:${GATEWAY_PORT}"
else
    echo "N/A (gateway service not found yet)"
fi
if [ -n "$GATEWAY_NODEPORT" ]; then
    echo ""
    echo "[LLM_D_NODEPORT]"
    echo "${GATEWAY_NODEPORT}"
    echo ""
    echo "[LLM_D_EXTERNAL_ENDPOINT]"
    if [ -n "$NODE_EXTERNAL_IP" ]; then
        echo "http://${NODE_EXTERNAL_IP}:${GATEWAY_NODEPORT}"
    else
        echo "http://<NODE_PUBLIC_IP>:${GATEWAY_NODEPORT}"
    fi
fi
echo ""
echo "[LLM_D_PODS_STATUS]"
echo "${READY_FINAL}/${TOTAL_FINAL} running"
echo ""
echo "[LLM_D_HELM_RELEASES]"
helm list -n "$LLM_D_NAMESPACE" --short 2>/dev/null || echo "none"
echo ""

# ============================================================
# Quick Reference
# ============================================================
echo "========================================"
echo "Quick Reference"
echo "========================================"
echo ""
echo "Check status:"
echo "  kubectl get pods -n $LLM_D_NAMESPACE"
echo "  kubectl get inferencepool -n $LLM_D_NAMESPACE"
echo "  helm list -n $LLM_D_NAMESPACE"
echo ""
if [ -n "$GATEWAY_SVC" ]; then
    if [ -n "$GATEWAY_NODEPORT" ]; then
        echo "External access (NodePort):"
        if [ -n "$NODE_EXTERNAL_IP" ]; then
            echo "  curl http://${NODE_EXTERNAL_IP}:${GATEWAY_NODEPORT}/v1/completions \\"
        else
            echo "  curl http://<NODE_PUBLIC_IP>:${GATEWAY_NODEPORT}/v1/completions \\"
        fi
        echo "    -H 'Content-Type: application/json' \\"
        echo "    -d '{\"model\": \"${DEPLOYED_MODEL}\", \"prompt\": \"Hello\", \"max_tokens\": 50}'"
        echo ""
    fi
    echo "Port forward for local access:"
    echo "  kubectl port-forward svc/$GATEWAY_SVC -n $LLM_D_NAMESPACE 8080:${GATEWAY_PORT}"
    echo ""
    echo "Test inference (via cluster internal or port-forward):"
    echo "  curl http://localhost:8080/v1/completions \\"
    echo "    -H 'Content-Type: application/json' \\"
    echo "    -d '{\"model\": \"${DEPLOYED_MODEL}\", \"prompt\": \"Hello\", \"max_tokens\": 50}'"
    echo ""
fi
echo "View logs:"
echo "  kubectl logs -n $LLM_D_NAMESPACE -l app.kubernetes.io/component=decode --tail=50"
echo ""
echo "Uninstall:"
echo "  deploy-llm-d.sh --uninstall"
echo "  # or: cd $GUIDE_DIR && helmfile destroy -n $LLM_D_NAMESPACE"
echo ""
