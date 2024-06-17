#!/bin/bash

# Install Docker
echo "Installing Docker"
curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/installDocker.sh | sh

# Run Open WebUI container
echo "Running Open WebUI container"

# Check if OLLAMA_BASE_URLS is provided
if [ -z "$1" ]; then
    echo "OLLAMA_BASE_URLS is not provided. Using default value: --add-host=host.docker.internal:host-gateway"
    sudo docker run -d -p 80:8080 -e --add-host=host.docker.internal:host-gateway -v open-webui:/app/backend/data --name open-webui --restart always ghcr.io/open-webui/open-webui:main
else
    echo "OLLAMA_BASE_URLS=$1"
    sudo docker run -d -p 80:8080 -e OLLAMA_BASE_URLS="$1" -v open-webui:/app/backend/data --name open-webui --restart always ghcr.io/open-webui/open-webui:main
fi

# Display the status of the Open WebUI container
echo "Displaying the status of the Open WebUI container"
sudo docker ps -f name=open-webui
