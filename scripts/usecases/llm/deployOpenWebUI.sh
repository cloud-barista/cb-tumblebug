#!/bin/bash

# Deploy Open WebUI with support for Ollama and OpenAI-compatible APIs (vLLM, etc.)
#
# Usage:
#   curl -fsSL <url> | bash                           # Local Ollama (default)
#   curl -fsSL <url> | bash -s -- ollama <url>        # Remote Ollama
#   curl -fsSL <url> | bash -s -- vllm <url>          # vLLM (OpenAI compatible)
#   curl -fsSL <url> | bash -s -- openai <url> <key>  # OpenAI API
#
# Examples:
#   bash deployOpenWebUI.sh ollama http://10.0.0.5:11434
#   bash deployOpenWebUI.sh vllm http://10.0.0.5:8000/v1
#   bash deployOpenWebUI.sh openai https://api.openai.com/v1 sk-xxx
#
# Notes:
#   - For vLLM: API key is typically not required (uses placeholder if omitted)
#   - For OpenAI: API key is required
#   - Security: API keys are stored in container environment variables and
#     may be visible via 'docker inspect'. Use appropriate access controls.

set -e

# Install Docker
echo "Installing Docker..."
curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/installDocker.sh | bash

# Stop and remove existing container if exists
if sudo docker ps -a --format '{{.Names}}' | grep -q '^open-webui$'; then
    echo "Removing existing Open WebUI container..."
    sudo docker stop open-webui 2>/dev/null || true
    sudo docker rm open-webui 2>/dev/null || true
fi

# Parse arguments
BACKEND_TYPE="${1:-ollama}"
BACKEND_URL="$2"
API_KEY="${3:-}"

# Validate BACKEND_URL format if provided
if [ -n "$BACKEND_URL" ]; then
    if ! [[ "$BACKEND_URL" =~ ^https?:// ]]; then
        echo "Error: Backend URL must start with http:// or https://"
        echo "  Provided: $BACKEND_URL"
        exit 1
    fi
fi

echo "=========================================="
echo "Deploying Open WebUI"
echo "=========================================="
echo "Backend Type: $BACKEND_TYPE"

case "$BACKEND_TYPE" in
    ollama)
        if [ -z "$BACKEND_URL" ]; then
            echo "Using local Ollama with host.docker.internal"
            sudo docker run -d \
                -p 80:8080 \
                --add-host=host.docker.internal:host-gateway \
                -v open-webui:/app/backend/data \
                --name open-webui \
                --restart always \
                ghcr.io/open-webui/open-webui:main
        else
            echo "Ollama URL: $BACKEND_URL"
            sudo docker run -d \
                -p 80:8080 \
                -e OLLAMA_BASE_URLS="$BACKEND_URL" \
                -v open-webui:/app/backend/data \
                --name open-webui \
                --restart always \
                ghcr.io/open-webui/open-webui:main
        fi
        ;;
    vllm|openai)
        if [ -z "$BACKEND_URL" ]; then
            echo "Error: Backend URL is required for $BACKEND_TYPE"
            echo "Usage: bash deployOpenWebUI.sh $BACKEND_TYPE <api_base_url> [api_key]"
            exit 1
        fi
        echo "OpenAI-compatible API URL: $BACKEND_URL"
        if [ -n "$API_KEY" ]; then
            echo "API Key: ${API_KEY:0:8}***"
        else
            echo "API Key: (not provided)"
            API_KEY="sk-no-key-required"
        fi
        sudo docker run -d \
            -p 80:8080 \
            -e OPENAI_API_BASE_URLS="$BACKEND_URL" \
            -e OPENAI_API_KEYS="$API_KEY" \
            -e ENABLE_OLLAMA_API=false \
            -v open-webui:/app/backend/data \
            --name open-webui \
            --restart always \
            ghcr.io/open-webui/open-webui:main
        ;;
    *)
        echo "Error: Unknown backend type: $BACKEND_TYPE"
        echo "Supported types: ollama, vllm, openai"
        exit 1
        ;;
esac

# Wait for container to start (with retry)
echo "Waiting for container to start..."
for i in 1 2 3; do
    sleep 5
    if sudo docker ps -f name=open-webui --format '{{.Names}}' | grep -q '^open-webui$'; then
        break
    fi
    if [ "$i" -lt 3 ]; then
        echo "Container not ready yet, retrying... ($i/3)"
    fi
done

# Check if container started successfully
if ! sudo docker ps -f name=open-webui --format '{{.Names}}' | grep -q '^open-webui$'; then
    echo "Error: Open WebUI container failed to start"
    echo "Container logs:"
    sudo docker logs open-webui 2>&1 | tail -20
    exit 1
fi

# Display the status of the Open WebUI container
echo "=========================================="
echo "Open WebUI Container Status"
echo "=========================================="
sudo docker ps -f name=open-webui --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

echo ""
echo "Open WebUI is available at: http://<your-ip>:80"
echo ""
