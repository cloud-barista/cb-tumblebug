#!/bin/bash

# vLLM Model Serving via KServe InferenceService (run on K8s control plane)
# Serves a Hugging Face model with the KServe HuggingFace runtime (vLLM backend),
# exposing an OpenAI-compatible API (/openai/v1/chat/completions).
# Re-run with a different --model to swap the served model in place.
# Multiple LLMs: run once per model with a unique --name and --nodeport
#   (each takes one GPU; with GPU time-slicing also set --gpu-mem-util, e.g. 0.45)
# Prerequisites: deploy-kserve-stack.sh completed, GPU resources available
# Designed for unattended execution via SSH or pipe (curl | bash)

set -e

# Defaults
MODEL_ID="Qwen/Qwen2.5-7B-Instruct"
ISVC_NAME="llm"
CTX_LEN=""          # empty = model default (larger = more KV cache VRAM)
GPU_COUNT="1"
MEMORY_LIMIT="24Gi"
HF_TOKEN="${HF_TOKEN:-}"
NODE_PORT=""        # empty = ClusterIP only; set (e.g. 30800) to expose the API externally
TOOL_PARSER="auto"  # auto-detect from model family; needed for clients sending tool_choice:"auto"
SERVED_NAME=""      # model id shown to API clients (default: basename of --model)
GPU_MEM_UTIL=""     # vLLM --gpu-memory-utilization; set a fraction (e.g. 0.45) on shared GPUs
TARGET_NODE=""      # pin to a node (useful with mixed GPU types, e.g. L40S vs A100)

usage() {
    echo "Usage: bash serve-vllm-model.sh [OPTIONS]"
    echo "  -m, --model MODEL   HuggingFace model name (default: ${MODEL_ID})"
    echo "  -n, --name NAME     InferenceService name (default: ${ISVC_NAME})"
    echo "  --served-name NAME  Model name shown to API clients (default: basename of --model)"
    echo "  --hf-token TOKEN    HuggingFace token for gated models (Llama, Mistral, ...)"
    echo "  --ctx-len N         Max context length / --max-model-len (default: model default)"
    echo "  --tool-parser P     auto|hermes|llama3_json|mistral (default: auto)"
    echo "  --nodeport PORT     Expose the OpenAI API on a NodePort (e.g. 30800)"
    echo "  --gpu N             GPU count; N>1 enables tensor parallelism (default: ${GPU_COUNT})"
    echo "  --gpu-mem-util F    vLLM GPU memory fraction 0.0-1.0 (use e.g. 0.45 on time-sliced GPUs)"
    echo "  --node NAME         Pin the model to a node (useful with mixed GPU types)"
    echo "  --memory LIMIT      Memory limit (default: ${MEMORY_LIMIT})"
    echo "Multiple LLMs: serve each with a unique --name and --nodeport, e.g."
    echo "  bash serve-vllm-model.sh --model Qwen/Qwen2.5-14B-Instruct --name llm --nodeport 30800"
    echo "  bash serve-vllm-model.sh --model google/gemma-3-12b-it --name llm2 --nodeport 30801"
    exit 1
}

while [[ $# -gt 0 ]]; do
    case $1 in
        -m|--model) MODEL_ID="$2"; shift 2 ;;
        -n|--name) ISVC_NAME="$2"; shift 2 ;;
        --served-name) SERVED_NAME="$2"; shift 2 ;;
        --hf-token) HF_TOKEN="$2"; shift 2 ;;
        --ctx-len|--max-len) CTX_LEN="$2"; shift 2 ;;
        --tool-parser) TOOL_PARSER="$2"; shift 2 ;;
        --nodeport) NODE_PORT="$2"; shift 2 ;;
        --gpu) GPU_COUNT="$2"; shift 2 ;;
        --gpu-mem-util) GPU_MEM_UTIL="$2"; shift 2 ;;
        --node) TARGET_NODE="$2"; shift 2 ;;
        --memory) MEMORY_LIMIT="$2"; shift 2 ;;
        -h|--help) usage ;;
        *) echo "Unknown option: $1"; usage ;;
    esac
done

# API clients (e.g., Open WebUI model selector) see this name
[ -z "$SERVED_NAME" ] && SERVED_NAME="${MODEL_ID##*/}"

# Tool parser auto-detection by model family
if [ "$TOOL_PARSER" = "auto" ]; then
    case "$(echo "$MODEL_ID" | tr '[:upper:]' '[:lower:]')" in
        *llama*) TOOL_PARSER="llama3_json" ;;
        *mistral*|*mixtral*) TOOL_PARSER="mistral" ;;
        *) TOOL_PARSER="hermes" ;;  # Qwen and most others
    esac
fi

echo "==== vLLM Model Serving (KServe) ===="
echo "  Model: ${MODEL_ID}"
echo "  InferenceService: ${ISVC_NAME}"
echo "  Served model name: ${SERVED_NAME}"
echo "  Context length: ${CTX_LEN:-model default}"
echo "  Tool parser: ${TOOL_PARSER}"

if ! kubectl get crd inferenceservices.serving.kserve.io > /dev/null 2>&1; then
    echo "ERROR: KServe is not installed. Run deploy-kserve-stack.sh first."
    exit 1
fi

# Fail early if the NodePort is already taken by a different service
if [ -n "$NODE_PORT" ]; then
    IN_USE=$(kubectl get svc -A -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.spec.ports[*].nodePort}{"\n"}{end}' 2>/dev/null \
        | awk -v p="$NODE_PORT" -v me="${ISVC_NAME}-api" '$0 ~ " "p"( |$)" && $1 != me {print $1}' | head -1)
    if [ -n "$IN_USE" ]; then
        echo "ERROR: NodePort ${NODE_PORT} is already used by service '${IN_USE}'."
        echo "       Pick another port for this model, e.g. --nodeport $((NODE_PORT + 1))"
        exit 1
    fi
fi

# Optional args assembled only when set
EXTRA_ARGS=""
if [ -n "$CTX_LEN" ]; then
    EXTRA_ARGS="${EXTRA_ARGS}
        - --max-model-len=${CTX_LEN}"
fi
if [ -n "$GPU_MEM_UTIL" ]; then
    EXTRA_ARGS="${EXTRA_ARGS}
        - --gpu-memory-utilization=${GPU_MEM_UTIL}"
fi
if [ "$GPU_COUNT" -gt 1 ] 2>/dev/null; then
    EXTRA_ARGS="${EXTRA_ARGS}
        - --tensor-parallel-size=${GPU_COUNT}"
fi

# VLLM_USE_DEEP_GEMM=0: DeepGEMM JIT needs nvcc, absent from the runtime image
ENV_BLOCK='
      env:
        - name: VLLM_USE_DEEP_GEMM
          value: "0"'

# HF token secret for gated models
if [ -n "$HF_TOKEN" ]; then
    kubectl create secret generic hf-token --from-literal=HF_TOKEN="$HF_TOKEN" \
        --dry-run=client -o yaml | kubectl apply -f - > /dev/null
    ENV_BLOCK="${ENV_BLOCK}
        - name: HF_TOKEN
          valueFrom:
            secretKeyRef:
              name: hf-token
              key: HF_TOKEN"
fi

NODE_BLOCK=""
if [ -n "$TARGET_NODE" ]; then
    NODE_BLOCK="
    nodeSelector:
      kubernetes.io/hostname: ${TARGET_NODE}"
fi

# deploymentStrategy Recreate: with a single GPU, RollingUpdate deadlocks
# (the new pod cannot schedule until the old pod releases the GPU)
cat <<EOF | kubectl apply -f -
apiVersion: serving.kserve.io/v1beta1
kind: InferenceService
metadata:
  name: ${ISVC_NAME}
spec:
  predictor:
    deploymentStrategy:
      type: Recreate${NODE_BLOCK}
    model:
      modelFormat:
        name: huggingface
      args:
        - --model_name=${SERVED_NAME}
        - --model_id=${MODEL_ID}
        - --backend=vllm
        - --enable-auto-tool-choice
        - --tool-call-parser=${TOOL_PARSER}${EXTRA_ARGS}${ENV_BLOCK}
      resources:
        requests:
          cpu: "2"
          memory: 10Gi
          nvidia.com/gpu: "${GPU_COUNT}"
        limits:
          cpu: "4"
          memory: ${MEMORY_LIMIT}
          nvidia.com/gpu: "${GPU_COUNT}"
EOF

# Optional NodePort for external API access (extra Service; the predictor
# Service is managed by KServe and would be reverted if patched directly)
if [ -n "$NODE_PORT" ]; then
    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  name: ${ISVC_NAME}-api
spec:
  type: NodePort
  selector:
    serving.kserve.io/inferenceservice: ${ISVC_NAME}
  ports:
    - port: 80
      targetPort: 8080
      nodePort: ${NODE_PORT}
EOF
fi

echo ""
echo "Waiting for model to be ready (image pull + model download; typically 5-15 min)..."
# Poll instead of a blind wait: fail fast with logs when the model crashes
# (e.g., unsupported architecture or model too large for the GPU)
READY=false
for i in $(seq 1 180); do
    if [ "$(kubectl get isvc ${ISVC_NAME} -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null)" = "True" ]; then
        READY=true; break
    fi
    # Fail fast when no GPU is free (pod would stay Pending forever)
    PHASE=$(kubectl get pods -l serving.kserve.io/inferenceservice=${ISVC_NAME} -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "")
    if [ "$PHASE" = "Pending" ] && [ "$i" -ge 9 ] && \
       kubectl describe pods -l serving.kserve.io/inferenceservice=${ISVC_NAME} 2>/dev/null | grep -q "Insufficient nvidia.com/gpu"; then
        echo ""
        echo "ERROR: no free GPU for this model (all nvidia.com/gpu slots are taken)."
        echo "Options:"
        echo "  - free a GPU:        kubectl delete isvc <one-of-the-served-models>"
        echo "  - share GPUs:        config-gpu-timeslicing.sh (any GPU) or config-gpu-mig.sh (A100/H100),"
        echo "                       then re-run this script (add --gpu-mem-util on time-sliced GPUs)"
        echo "  - add a GPU node:    join another GPU worker to the cluster"
        echo "Currently served LLMs:"
        kubectl get isvc -o jsonpath='{range .items[?(@.spec.predictor.model.modelFormat.name=="huggingface")]}{"  "}{.metadata.name}{"\n"}{end}' 2>/dev/null
        echo "(removing the pending InferenceService '${ISVC_NAME}' to keep the cluster clean)"
        kubectl delete isvc ${ISVC_NAME} > /dev/null 2>&1 || true
        exit 1
    fi
    RESTARTS=$(kubectl get pods -l serving.kserve.io/inferenceservice=${ISVC_NAME} -o jsonpath='{.items[0].status.containerStatuses[0].restartCount}' 2>/dev/null || echo 0)
    WAITING=$(kubectl get pods -l serving.kserve.io/inferenceservice=${ISVC_NAME} -o jsonpath='{.items[0].status.containerStatuses[0].state.waiting.reason}' 2>/dev/null || echo "")
    if [ "${RESTARTS:-0}" -ge 5 ] && [ "$WAITING" = "CrashLoopBackOff" ]; then
        echo ""
        echo "ERROR: model server keeps crashing (${RESTARTS} restarts). Recent logs:"
        kubectl logs -l serving.kserve.io/inferenceservice=${ISVC_NAME} --previous --tail=15 2>/dev/null | tail -8 || \
        kubectl logs -l serving.kserve.io/inferenceservice=${ISVC_NAME} --tail=15 2>/dev/null | tail -8
        echo ""
        echo "Common causes: model architecture newer than the runtime supports,"
        echo "or model too large for GPU VRAM. Try a smaller/older model."
        exit 1
    fi
    sleep 10
done
if [ "$READY" = true ]; then
    echo "  ✓ InferenceService is ready"
else
    echo "  ⚠ Not ready within 30m. Check: kubectl logs -l serving.kserve.io/inferenceservice=${ISVC_NAME}"
    exit 1
fi

# Smoke test via the in-cluster service
CLUSTER_IP=$(kubectl get svc ${ISVC_NAME}-predictor -o jsonpath='{.spec.clusterIP}')
RESP=$(curl -s --max-time 60 "http://${CLUSTER_IP}/openai/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -d "{\"model\":\"${SERVED_NAME}\",\"messages\":[{\"role\":\"user\",\"content\":\"Say OK\"}],\"max_tokens\":10}" || true)

echo ""
echo "========================================"
if echo "$RESP" | grep -q '"content"'; then
    echo "SUCCESS: Model is serving"
else
    echo "WARNING: Ready but smoke test did not return content"
    echo "$RESP" | head -c 300
fi
echo "========================================"
echo ""
echo "[KSERVE_MODEL_ENDPOINT]"
echo "http://${ISVC_NAME}-predictor.default.svc.cluster.local/openai/v1"
if [ -n "$NODE_PORT" ]; then
    echo ""
    echo "[KSERVE_MODEL_NODEPORT]"
    echo "${NODE_PORT}"
    echo "External API: http://<node-public-ip>:${NODE_PORT}/openai/v1 (allow ${NODE_PORT}/tcp in the Security Group)"
fi
echo ""
echo "=== LLMs currently served by KServe ==="
kubectl get isvc -o jsonpath='{range .items[?(@.spec.predictor.model.modelFormat.name=="huggingface")]}{.metadata.name}{"  ready="}{.status.conditions[?(@.type=="Ready")].status}{"\n"}{end}' 2>/dev/null | sed 's/^/  /'
echo ""
echo "To change this model, re-run with a different --model."
echo "To ADD another model, re-run with a unique --name and --nodeport (e.g. --name llm2 --nodeport 30801),"
echo "then re-run deploy-open-webui-kserve.sh to connect all models to the WebUI."
echo ""
echo "\$\$CMD[Check InferenceService](kubectl get isvc ${ISVC_NAME})"
echo "\$\$CMD[Model Logs](kubectl logs -l serving.kserve.io/inferenceservice=${ISVC_NAME} --tail=30)"
exit 0
