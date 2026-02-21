#!/bin/bash
MODEL_NAME="${1:-}"
HOST="${2:-0.0.0.0}"
PORT="${3:-8000}"
EXTRA_ARGS="${@:4}" 

if [ -z "$MODEL_NAME" ]; then
  echo "Error: Model name is required."
  exit 1
fi

echo "=========================================="
echo "vLLM Model Serving on AMD GPU"
echo "Model: $MODEL_NAME"
echo "Port: $PORT"
echo "Extra Args: $EXTRA_ARGS"
echo "=========================================="

CONTAINER_NAME="vllm-serve-container"
HEALTH_CHECK_TIMEOUT=300
HEALTH_CHECK_INTERVAL=5

# Stop and remove existing vLLM container if running
if [ "$(sudo docker ps -aq -f name=^${CONTAINER_NAME}$)" ]; then
    echo "Stopping existing vLLM container..."
    sudo docker stop ${CONTAINER_NAME} >/dev/null
    sudo docker rm ${CONTAINER_NAME} >/dev/null
fi

# Start vLLM server in Docker container with GPU access
echo "Starting vLLM server with model: $MODEL_NAME in Docker..."

sudo docker run -d \
  --name ${CONTAINER_NAME} \
  --network=host \
  --group-add=video \
  --ipc=host \
  --cap-add=SYS_PTRACE \
  --security-opt seccomp=unconfined \
  --privileged \
  --device /dev/kfd \
  --device /dev/dri \
  -v ~/.cache/huggingface:/root/.cache/huggingface \
  rocm/vllm:latest \
  python3 -m vllm.entrypoints.openai.api_server \
  --model "$MODEL_NAME" \
  --host "$HOST" \
  --port "$PORT" \
  --trust-remote-code \
  $EXTRA_ARGS

# Wait for vLLM server to initialize and respond to health checks
echo "Waiting for server to be ready..."
elapsed=0
while [ $elapsed -lt $HEALTH_CHECK_TIMEOUT ]; do
  status=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:$PORT/v1/models 2>/dev/null)
  
  if [ "$status" = "200" ]; then
    echo -e "\n=========================================="
    echo "vLLM Server Started Successfully!"
    echo "=========================================="
    exit 0
  fi
  
  if [ ! "$(sudo docker ps -q -f name=^${CONTAINER_NAME}$)" ]; then
     echo -e "\nError: vLLM container crashed. Check logs with: sudo docker logs ${CONTAINER_NAME}"
     exit 1
  fi

  printf "."
  sleep $HEALTH_CHECK_INTERVAL
  elapsed=$((elapsed + HEALTH_CHECK_INTERVAL))
done

echo -e "\nError: Server failed to start."
exit 1