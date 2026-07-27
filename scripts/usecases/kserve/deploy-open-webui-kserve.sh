#!/bin/bash

# Open WebUI for KServe (run on K8s control plane)
# Deploys Open WebUI connected to KServe OpenAI-compatible endpoints via NodePort.
# By default it auto-discovers ALL vLLM InferenceServices, so every served LLM
# appears in the model selector; re-run after adding a model to reconnect.
# Prerequisites: serve-vllm-model.sh completed (or any OpenAI-compatible backend URL)
# Designed for unattended execution via SSH or pipe (curl | bash)

set -e

ISVC_NAME="llm"
BACKEND_URL=""
NODE_PORT="30080"

usage() {
    echo "Usage: $0 [-n|--name <isvc-name>] [--backend-url <url[;url2;...]>] [--nodeport <port>]"
    echo "Default: auto-discovers every vLLM InferenceService and connects them all"
    echo "Example: $0 --nodeport 30080"
    exit 1
}

while [[ $# -gt 0 ]]; do
    case $1 in
        -n|--name) ISVC_NAME="$2"; shift 2 ;;
        --backend-url) BACKEND_URL="$2"; shift 2 ;;
        --nodeport) NODE_PORT="$2"; shift 2 ;;
        -h|--help) usage ;;
        *) echo "Unknown option: $1"; usage ;;
    esac
done

# Default: connect every vLLM (huggingface-format) InferenceService
if [ -z "$BACKEND_URL" ]; then
    BACKEND_URL=$(kubectl get isvc -o jsonpath='{range .items[?(@.spec.predictor.model.modelFormat.name=="huggingface")]}{.metadata.name}{"\n"}{end}' 2>/dev/null \
        | awk 'NF {printf "%s%s", sep, "http://"$1"-predictor.default.svc.cluster.local/openai/v1"; sep=";"}')
    [ -z "$BACKEND_URL" ] && BACKEND_URL="http://${ISVC_NAME}-predictor.default.svc.cluster.local/openai/v1"
fi
# One "none" API key per backend URL
API_KEYS=$(echo "$BACKEND_URL" | sed 's/[^;][^;]*/none/g')

echo "==== Open WebUI Setup (KServe) ===="
echo "  Backends: ${BACKEND_URL}"
echo "  NodePort: ${NODE_PORT}"

# The PVC below needs a default StorageClass; fail early with a clear message
if ! kubectl get storageclass 2>/dev/null | grep -q "(default)"; then
    echo "ERROR: No default StorageClass found (the data PVC would stay Pending forever)."
    echo "       Run deploy-kserve-stack.sh first, or set a default StorageClass manually."
    exit 1
fi

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: open-webui-data
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: 5Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: open-webui
spec:
  replicas: 1
  selector:
    matchLabels: { app: open-webui }
  template:
    metadata:
      labels: { app: open-webui }
    spec:
      containers:
        - name: open-webui
          image: ghcr.io/open-webui/open-webui:main
          ports:
            - containerPort: 8080
          env:
            - name: OPENAI_API_BASE_URLS
              value: "${BACKEND_URL}"
            - name: OPENAI_API_KEYS
              value: "${API_KEYS}"
            - name: ENABLE_OLLAMA_API
              value: "false"
          volumeMounts:
            - name: data
              mountPath: /app/backend/data
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: open-webui-data
---
apiVersion: v1
kind: Service
metadata:
  name: open-webui
spec:
  type: NodePort
  selector: { app: open-webui }
  ports:
    - port: 8080
      targetPort: 8080
      nodePort: ${NODE_PORT}
EOF

echo ""
echo "Waiting for Open WebUI to start (image pull; typically 2-5 min)..."
kubectl rollout status deployment/open-webui --timeout=10m > /dev/null

echo ""
echo "========================================"
echo "SUCCESS: Open WebUI is running"
echo "========================================"
echo ""
echo "[OPEN_WEBUI_NODEPORT]"
echo "${NODE_PORT}"
echo ""
echo "Access: http://<node-public-ip>:${NODE_PORT}"
echo "  (1) Allow inbound ${NODE_PORT}/tcp in the Security Group"
echo "  (2) First signup becomes the admin account"
echo "  (3) Select the served model in the model selector and chat"
echo ""
echo "\$\$CMD[Check Open WebUI](kubectl get pods -l app=open-webui)"
exit 0
