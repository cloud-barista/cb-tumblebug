#!/bin/bash

# vLLM Model Serving via KServe InferenceService (run on K8s control plane)
# Serves a Hugging Face model with the KServe HuggingFace runtime (vLLM backend),
# exposing an OpenAI-compatible API (/openai/v1/chat/completions).
# Re-run with a different --model to swap the served model in place.
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

usage() {
    echo "Usage: bash serve-vllm-model.sh [OPTIONS]"
    echo "  -m, --model MODEL   HuggingFace model name (default: ${MODEL_ID})"
    echo "  -n, --name NAME     InferenceService name (default: ${ISVC_NAME})"
    echo "  --served-name NAME  Model name shown to API clients (default: basename of --model)"
    echo "  --hf-token TOKEN    HuggingFace token for gated models (Llama, Mistral, ...)"
    echo "  --ctx-len N         Max context length / --max-model-len (default: model default)"
    echo "  --tool-parser P     auto|hermes|llama3_json|mistral (default: auto)"
    echo "  --nodeport PORT     Expose the OpenAI API on a NodePort (e.g. 30800)"
    echo "  --gpu N             GPU count (default: ${GPU_COUNT})"
    echo "  --memory LIMIT      Memory limit (default: ${MEMORY_LIMIT})"
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

# Optional args assembled only when set
EXTRA_ARGS=""
if [ -n "$CTX_LEN" ]; then
    EXTRA_ARGS="${EXTRA_ARGS}
        - --max-model-len=${CTX_LEN}"
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
      type: Recreate
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
echo "To change the model, re-run this script with a different --model."
echo ""
echo "\$\$CMD[Check InferenceService](kubectl get isvc ${ISVC_NAME})"
echo "\$\$CMD[Model Logs](kubectl logs -l serving.kserve.io/inferenceservice=${ISVC_NAME} --tail=30)"
exit 0
