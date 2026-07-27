#!/bin/bash

# GPU time-slicing via GPU Operator (run on K8s control plane)
# Lets N pods share one physical GPU (e.g. two vLLM models on a single L40S).
# Time-slicing has NO memory isolation: give each vLLM model a fraction via
# serve-vllm-model.sh --gpu-mem-util (e.g. 0.45 each for 2 models per GPU).
# Note: L40S/L4/A10 do not support MIG; time-slicing is the sharing option there
# (on A100/H100 prefer config-gpu-mig.sh for hardware isolation; for software
#  VRAM isolation on non-MIG GPUs see the HAMi project — not included here).
# Prerequisites: deploy-kserve-stack.sh completed (GPU Operator installed)

set -e

REPLICAS="2"
DISABLE=false
TARGET_NODES=""   # empty = every GPU node; set on mixed clusters (e.g. slice L40S, MIG the A100)

usage() {
    echo "Usage: $0 [--replicas N] [--node NAME]... [--disable]"
    echo "  --replicas N   How many pods can share one GPU (default: ${REPLICAS})"
    echo "  --node NAME    Apply only to this node (repeatable; default: all GPU nodes)"
    echo "  --disable      Remove time-slicing (back to 1 pod per GPU)"
    exit 1
}

while [[ $# -gt 0 ]]; do
    case $1 in
        --replicas) REPLICAS="$2"; shift 2 ;;
        --node) TARGET_NODES="${TARGET_NODES} $2"; shift 2 ;;
        --disable) DISABLE=true; shift ;;
        -h|--help) usage ;;
        *) echo "Unknown option: $1"; usage ;;
    esac
done

NS="gpu-operator"
kubectl get ns ${NS} > /dev/null 2>&1 || { echo "ERROR: gpu-operator namespace not found. Run deploy-kserve-stack.sh first."; exit 1; }

if [ "$DISABLE" = true ]; then
    echo "==== Disabling GPU time-slicing ===="
    kubectl patch clusterpolicies.nvidia.com cluster-policy --type merge \
        -p '{"spec": {"devicePlugin": {"config": null}}}'
    kubectl label nodes -l nvidia.com/gpu.present=true nvidia.com/device-plugin.config- 2>/dev/null || true
else
    echo "==== Enabling GPU time-slicing (${REPLICAS} pods per GPU) ===="
    cat <<EOF | kubectl -n ${NS} apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: time-slicing-config
data:
  time-sliced: |-
    version: v1
    flags:
      migStrategy: none
    sharing:
      timeSlicing:
        renameByDefault: false
        failRequestsGreaterThanOne: false
        resources:
        - name: nvidia.com/gpu
          replicas: ${REPLICAS}
EOF
    if [ -n "$(echo ${TARGET_NODES} | tr -d ' ')" ]; then
        # Per-node: only labeled nodes pick up the config (others keep full GPUs)
        kubectl patch clusterpolicies.nvidia.com cluster-policy --type merge \
            -p '{"spec": {"devicePlugin": {"config": {"name": "time-slicing-config", "default": null}}}}'
        for NODE in ${TARGET_NODES}; do
            kubectl label node "${NODE}" nvidia.com/device-plugin.config=time-sliced --overwrite
        done
        echo "  Applied to node(s):${TARGET_NODES} (other GPU nodes keep 1 pod per GPU)"
    else
        kubectl patch clusterpolicies.nvidia.com cluster-policy --type merge \
            -p '{"spec": {"devicePlugin": {"config": {"name": "time-slicing-config", "default": "time-sliced"}}}}'
    fi
fi

echo ""
echo "Waiting for the device plugin to restart and re-advertise GPUs..."
sleep 10
kubectl -n ${NS} rollout status daemonset/nvidia-device-plugin-daemonset --timeout=3m > /dev/null 2>&1 || true

# Allocatable count settles a few seconds after the plugin restarts
for i in $(seq 1 18); do
    TOTAL=$(kubectl get nodes -o jsonpath='{range .items[*]}{.status.allocatable.nvidia\.com/gpu}{"\n"}{end}' 2>/dev/null | awk '{s+=$1} END {print s+0}')
    [ "${TOTAL:-0}" -gt 0 ] && break
    sleep 10
done

echo ""
echo "=== Allocatable nvidia.com/gpu per node ==="
kubectl get nodes -o custom-columns='NAME:.metadata.name,GPU:.status.allocatable.nvidia\.com/gpu'

echo ""
echo "========================================"
if [ "$DISABLE" = true ]; then
    echo "SUCCESS: time-slicing disabled (1 pod per physical GPU)"
else
    echo "SUCCESS: each physical GPU now accepts ${REPLICAS} pods"
    echo "========================================"
    echo ""
    echo "IMPORTANT: time-slicing does not partition VRAM. When serving multiple"
    echo "vLLM models on one GPU, cap each one, e.g. for ${REPLICAS} models per GPU:"
    echo "  bash serve-vllm-model.sh --model <m1> --name llm  --nodeport 30800 --gpu-mem-util 0.45"
    echo "  bash serve-vllm-model.sh --model <m2> --name llm2 --nodeport 30801 --gpu-mem-util 0.45"
fi
echo ""
echo "\$\$CMD[Check GPU allocatable](kubectl get nodes -o custom-columns='NAME:.metadata.name,GPU:.status.allocatable.nvidia\\.com/gpu')"
exit 0
