#!/bin/bash

# MIG partitioning via GPU Operator mig-manager (run on K8s control plane)
# Splits an A100/H100-class GPU into hardware-isolated slices, each visible as
# one nvidia.com/gpu (single strategy) — serve-vllm-model.sh works unchanged.
# Not supported on L40S/L4/A10 (use config-gpu-timeslicing.sh there instead).
# WARNING: applying a profile resets the GPU; model pods on the node restart.
# Prerequisites: deploy-kserve-stack.sh completed (GPU Operator installed)

set -e

PROFILE="all-3g.40gb"
TARGET_NODE=""   # empty = all MIG-capable GPU nodes

usage() {
    echo "Usage: $0 [--profile P] [--node NAME] [--disable]"
    echo "  --profile P   MIG profile (default: ${PROFILE})"
    echo "                A100/H100 80GB: all-1g.10gb (7x 10GB) | all-2g.20gb (3x 20GB) | all-3g.40gb (2x 40GB)"
    echo "                A100 40GB:      all-1g.5gb  (7x 5GB)  | all-2g.10gb (3x 10GB) | all-3g.20gb (2x 20GB)"
    echo "  --node NAME   Apply to one node only (default: all GPU nodes)"
    echo "  --disable     Back to full GPUs (profile all-disabled)"
    echo "VRAM guide per slice (FP8 ≈ 1 GB/B): 3g.40gb ≤32B | 2g.20gb ≤14B | 1g.10gb ≤8B"
    exit 1
}

while [[ $# -gt 0 ]]; do
    case $1 in
        --profile) PROFILE="$2"; shift 2 ;;
        --node) TARGET_NODE="$2"; shift 2 ;;
        --disable) PROFILE="all-disabled"; shift ;;
        -h|--help) usage ;;
        *) echo "Unknown option: $1"; usage ;;
    esac
done

kubectl get ns gpu-operator > /dev/null 2>&1 || { echo "ERROR: gpu-operator namespace not found. Run deploy-kserve-stack.sh first."; exit 1; }

if [ -n "$TARGET_NODE" ]; then
    NODES="$TARGET_NODE"
else
    NODES=$(kubectl get nodes -l nvidia.com/gpu.present=true -o jsonpath='{.items[*].metadata.name}')
fi
[ -n "$NODES" ] || { echo "ERROR: no GPU nodes found (label nvidia.com/gpu.present=true missing)"; exit 1; }

# MIG capability check via GPU Feature Discovery labels
for NODE in $NODES; do
    CAPABLE=$(kubectl get node "$NODE" -o jsonpath='{.metadata.labels.nvidia\.com/mig\.capable}' 2>/dev/null)
    if [ "$CAPABLE" != "true" ] && [ "$PROFILE" != "all-disabled" ]; then
        echo "WARNING: node ${NODE} is not MIG-capable (mig.capable=${CAPABLE:-unset}); skipping"
        NODES=$(echo "$NODES" | tr ' ' '\n' | grep -v "^${NODE}$" | tr '\n' ' ')
    fi
done
[ -n "$(echo $NODES | tr -d ' ')" ] || { echo "ERROR: no MIG-capable nodes to configure (A100/H100-class required)"; exit 1; }

echo "==== MIG configuration: ${PROFILE} on: ${NODES} ===="
for NODE in $NODES; do
    kubectl label node "$NODE" nvidia.com/mig.config="${PROFILE}" --overwrite
done

echo ""
echo "Waiting for mig-manager to apply the profile (GPU reset; typically 1-3 min)..."
for i in $(seq 1 30); do
    PENDING=""
    for NODE in $NODES; do
        STATE=$(kubectl get node "$NODE" -o jsonpath='{.metadata.labels.nvidia\.com/mig\.config\.state}' 2>/dev/null)
        [ "$STATE" = "success" ] || PENDING="${PENDING} ${NODE}(${STATE:-pending})"
    done
    [ -z "$PENDING" ] && break
    echo "  [$(date +%H:%M:%S)] applying:${PENDING}"
    sleep 10
done
if [ -n "$PENDING" ]; then
    echo "ERROR: MIG config did not reach 'success' on:${PENDING}"
    echo "Check: kubectl -n gpu-operator logs -l app=nvidia-mig-manager --tail=30"
    exit 1
fi

# Allocatable count settles after the device plugin re-advertises
sleep 15
echo ""
echo "=== Allocatable nvidia.com/gpu per node (each MIG slice counts as 1) ==="
kubectl get nodes -o custom-columns='NAME:.metadata.name,GPU:.status.allocatable.nvidia\.com/gpu'

echo ""
echo "========================================"
if [ "$PROFILE" = "all-disabled" ]; then
    echo "SUCCESS: MIG disabled (full GPUs restored)"
else
    echo "SUCCESS: MIG profile ${PROFILE} active"
    echo "========================================"
    echo ""
    echo "Each slice is hardware-isolated (own VRAM) — no --gpu-mem-util needed."
    echo "Serve one model per slice, e.g. with all-3g.40gb (2 slices per GPU):"
    echo "  bash serve-vllm-model.sh --model <m1> --name llm  --nodeport 30800"
    echo "  bash serve-vllm-model.sh --model <m2> --name llm2 --nodeport 30801"
fi
echo ""
echo "\$\$CMD[Check GPU allocatable](kubectl get nodes -o custom-columns='NAME:.metadata.name,GPU:.status.allocatable.nvidia\\.com/gpu')"
exit 0
