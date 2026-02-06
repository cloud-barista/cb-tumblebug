#!/bin/bash

# llm-d Deployment Script for Kubernetes
# This script installs llm-d (distributed LLM inference) on a K8s cluster
# Designed for unattended execution via SSH
#
# Prerequisites:
#   - Kubernetes cluster with Gateway API, LeaderWorkerSet CRDs
#   - NVIDIA GPU Operator installed
#   - GPU worker nodes with NVIDIA drivers
#   - kubectl configured with cluster access
#
# Usage:
#   ./deploy-llm-d.sh                    # Deploy llm-d with default settings
#   ./deploy-llm-d.sh --model MODEL      # Deploy with specific model
#   ./deploy-llm-d.sh --replicas N       # Set number of replicas
#   ./deploy-llm-d.sh --check            # Check prerequisites only
#
# Remote execution (CB-MapUI / CB-Tumblebug API):
#   This script is designed for non-interactive SSH execution.

set -e

# ============================================================
# Non-interactive mode for SSH remote execution
# ============================================================
export DEBIAN_FRONTEND=noninteractive

# ============================================================
# Configuration
# ============================================================
LLM_D_VERSION="latest"
LLM_D_NAMESPACE="llm-d"
MODEL_NAME=""                 # Default: let llm-d use its default
REPLICAS="1"
GPU_MEMORY="40Gi"            # GPU memory limit per replica
CHECK_ONLY=false
INSTALL_GATEWAY=false        # Install Istio Gateway (for external access)

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
        --version|-v)
            LLM_D_VERSION="$2"
            shift 2
            ;;
        --namespace|-n)
            LLM_D_NAMESPACE="$2"
            shift 2
            ;;
        --gpu-memory)
            GPU_MEMORY="$2"
            shift 2
            ;;
        --with-gateway)
            INSTALL_GATEWAY=true
            shift
            ;;
        --check)
            CHECK_ONLY=true
            shift
            ;;
        -h|--help)
            echo "llm-d Deployment Script"
            echo ""
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  -m, --model MODEL    Model to serve (e.g., meta-llama/Llama-3.1-8B-Instruct)"
            echo "  -r, --replicas N     Number of replicas (default: 1)"
            echo "  -n, --namespace NS   Kubernetes namespace (default: llm-d)"
            echo "  --gpu-memory SIZE    GPU memory limit per replica (default: 40Gi)"
            echo "  --with-gateway       Install Istio Gateway for external access"
            echo "  --check              Check prerequisites only, don't install"
            echo "  -h, --help           Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                                    # Default deployment"
            echo "  $0 --model meta-llama/Llama-3.1-8B-Instruct"
            echo "  $0 --replicas 2 --gpu-memory 80Gi"
            echo "  $0 --check                            # Verify prerequisites"
            echo ""
            echo "Prerequisites:"
            echo "  - Kubernetes 1.29+ with kubectl access"
            echo "  - Gateway API CRDs installed"
            echo "  - LeaderWorkerSet CRD installed"
            echo "  - NVIDIA GPU Operator or device plugin"
            echo "  - GPU worker nodes with nvidia.com/gpu resources"
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
if [ -n "$MODEL_NAME" ]; then
    echo "Model: $MODEL_NAME"
fi
echo "Replicas: $REPLICAS"
echo "GPU Memory: $GPU_MEMORY"
echo "=========================================="
echo ""

# ============================================================
# Check Prerequisites
# ============================================================
echo "Checking prerequisites..."
PREREQ_FAILED=false

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
    K8S_VERSION=$(kubectl version --short 2>/dev/null | grep Server | awk '{print $3}' || kubectl version -o json 2>/dev/null | grep -o '"gitVersion": "[^"]*"' | head -1 | cut -d'"' -f4)
    echo "    Kubernetes version: ${K8S_VERSION:-unknown}"
fi

# Check Gateway API CRDs
if kubectl get crd gateways.gateway.networking.k8s.io &>/dev/null; then
    echo "  ✓ Gateway API CRDs installed"
else
    echo "  ✗ Gateway API CRDs not found"
    echo "    Install with: kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.0/standard-install.yaml"
    PREREQ_FAILED=true
fi

# Check LeaderWorkerSet CRD
if kubectl get crd leaderworkersets.leaderworkerset.x-k8s.io &>/dev/null; then
    echo "  ✓ LeaderWorkerSet CRD installed"
else
    echo "  ✗ LeaderWorkerSet CRD not found"
    echo "    Install with: kubectl apply --server-side -f https://github.com/kubernetes-sigs/lws/releases/download/v0.5.1/manifests.yaml"
    PREREQ_FAILED=true
fi

# Check GPU resources
GPU_NODES=$(kubectl get nodes -o jsonpath='{.items[*].status.allocatable.nvidia\.com/gpu}' 2>/dev/null | tr ' ' '\n' | grep -v '^$' | awk '{sum+=$1} END {print sum}')
if [ -n "$GPU_NODES" ] && [ "$GPU_NODES" -gt 0 ]; then
    echo "  ✓ GPU resources available: ${GPU_NODES} GPU(s)"
else
    echo "  ⚠ No GPU resources detected"
    echo "    Ensure GPU workers have joined and GPU Operator is running"
    echo "    Check with: kubectl describe nodes | grep nvidia.com/gpu"
fi

# Check Helm
if ! command -v helm &>/dev/null; then
    echo "  ✗ Helm not found"
    echo "    Installing Helm..."
    curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash > /dev/null 2>&1
    if command -v helm &>/dev/null; then
        echo "  ✓ Helm installed"
    else
        echo "  ✗ Failed to install Helm"
        PREREQ_FAILED=true
    fi
else
    echo "  ✓ Helm available: $(helm version --short 2>/dev/null | head -1)"
fi

echo ""

if [ "$PREREQ_FAILED" = true ]; then
    echo "=========================================="
    echo "ERROR: Prerequisites not met"
    echo "=========================================="
    echo ""
    echo "Please install missing components before running this script."
    echo ""
    echo "Quick setup (run on control plane with --llm-d mode):"
    echo "  ./k8s-control-plane-setup.sh --llm-d"
    exit 1
fi

if [ "$CHECK_ONLY" = true ]; then
    echo "=========================================="
    echo "Prerequisites Check: PASSED"
    echo "=========================================="
    echo ""
    echo "All prerequisites are met. Run without --check to deploy."
    exit 0
fi

# ============================================================
# Install llm-d
# ============================================================
echo "Installing llm-d..."

# Add llm-d Helm repository
echo "Adding llm-d Helm repository..."
helm repo add llm-d https://llm-d.github.io/llm-d/ 2>/dev/null || true
helm repo update > /dev/null 2>&1

# Create namespace
kubectl create namespace "$LLM_D_NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - > /dev/null

# Prepare Helm values
HELM_VALUES=""
if [ -n "$MODEL_NAME" ]; then
    HELM_VALUES="$HELM_VALUES --set model.name=$MODEL_NAME"
fi
HELM_VALUES="$HELM_VALUES --set replicaCount=$REPLICAS"
HELM_VALUES="$HELM_VALUES --set resources.limits.nvidia\\.com/gpu=1"

# Install llm-d
echo "Installing llm-d Helm chart..."
helm upgrade --install llm-d llm-d/llm-d \
    --namespace "$LLM_D_NAMESPACE" \
    $HELM_VALUES \
    --wait --timeout 10m 2>&1 | grep -E "^(NAME:|STATUS:|REVISION:)" || true

# Wait for deployment
echo ""
echo "Waiting for llm-d pods to be ready..."
for i in {1..60}; do
    READY_PODS=$(kubectl get pods -n "$LLM_D_NAMESPACE" -l app.kubernetes.io/name=llm-d -o jsonpath='{.items[*].status.containerStatuses[*].ready}' 2>/dev/null | tr ' ' '\n' | grep -c true 2>/dev/null) || READY_PODS=0
    TOTAL_PODS=$(kubectl get pods -n "$LLM_D_NAMESPACE" -l app.kubernetes.io/name=llm-d --no-headers 2>/dev/null | wc -l | tr -d ' ') || TOTAL_PODS=0
    
    if [ "$READY_PODS" -gt 0 ] && [ "$READY_PODS" -eq "$TOTAL_PODS" ]; then
        echo "  All pods ready ($READY_PODS/$TOTAL_PODS)"
        break
    fi
    
    if [ $((i % 10)) -eq 0 ]; then
        echo "  Waiting... ($READY_PODS/$TOTAL_PODS pods ready)"
    fi
    sleep 5
done

# ============================================================
# Get Service Information
# ============================================================
echo ""
echo "=========================================="
echo "llm-d Deployment Complete!"
echo "=========================================="
echo ""

# Get service details
SERVICE_NAME=$(kubectl get svc -n "$LLM_D_NAMESPACE" -l app.kubernetes.io/name=llm-d -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "llm-d")
SERVICE_PORT=$(kubectl get svc -n "$LLM_D_NAMESPACE" "$SERVICE_NAME" -o jsonpath='{.spec.ports[0].port}' 2>/dev/null || echo "8000")
CLUSTER_IP=$(kubectl get svc -n "$LLM_D_NAMESPACE" "$SERVICE_NAME" -o jsonpath='{.spec.clusterIP}' 2>/dev/null || echo "")

echo "[LLM_D_NAMESPACE]"
echo "$LLM_D_NAMESPACE"
echo ""
echo "[LLM_D_SERVICE]"
echo "$SERVICE_NAME"
echo ""
echo "[LLM_D_ENDPOINT]"
echo "http://${SERVICE_NAME}.${LLM_D_NAMESPACE}.svc.cluster.local:${SERVICE_PORT}"
echo ""

# Show pods
echo "Pods:"
kubectl get pods -n "$LLM_D_NAMESPACE" -o wide 2>/dev/null || true
echo ""

# Show services
echo "Services:"
kubectl get svc -n "$LLM_D_NAMESPACE" 2>/dev/null || true
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
echo "  kubectl logs -n $LLM_D_NAMESPACE -l app.kubernetes.io/name=llm-d"
echo ""
echo "Port forward for local access:"
echo "  kubectl port-forward svc/$SERVICE_NAME -n $LLM_D_NAMESPACE 8000:$SERVICE_PORT"
echo ""
echo "Test inference (from cluster):"
echo "  curl http://${SERVICE_NAME}.${LLM_D_NAMESPACE}:${SERVICE_PORT}/v1/completions \\"
echo "    -H 'Content-Type: application/json' \\"
echo "    -d '{\"prompt\": \"Hello\", \"max_tokens\": 50}'"
echo ""
echo "Scale replicas:"
echo "  kubectl scale deployment llm-d -n $LLM_D_NAMESPACE --replicas=<N>"
echo ""
