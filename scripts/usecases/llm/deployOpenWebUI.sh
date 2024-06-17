#!/bin/bash

# Check if OLLAMA_BASE_URLS is provided
if [ -z "$1" ]; then
    echo "OLLAMA_BASE_URLS is not provided. Using default value: --add-host=host.docker.internal:host-gateway"
    OLLAMA_BASE_URLS="--add-host=host.docker.internal:host-gateway"
else
    OLLAMA_BASE_URLS="-e \"OLLAMA_BASE_URLS=$1\""
fi

# Install Docker
echo "Installing Docker"
curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/installDocker.sh | sh

# Run Open WebUI container
echo "Running Open WebUI container"
sudo docker run -d -p 80:8080 $OLLAMA_BASE_URLS -v open-webui:/app/backend/data --name open-webui --restart always ghcr.io/open-webui/open-webui:main

# Display the status of the Open WebUI container
echo "Displaying the status of the Open WebUI container"
sudo docker ps -f name=open-webui
