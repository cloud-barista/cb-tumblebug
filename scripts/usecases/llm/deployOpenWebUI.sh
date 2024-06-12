#!/bin/bash

# Check if OLLAMA_BASE_URLS is provided
if [ -z "$1" ]; then
    echo "Error: OLLAMA_BASE_URLS is not provided."
    echo "Usage: $0 <OLLAMA_BASE_URLS>"
    echo "Example: $0 http://3.144.94.41:3000,http://35.223.136.86:3000"
    exit 1
fi

OLLAMA_BASE_URLS=$1

# Install Docker
echo "Installing Docker"
curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/installDocker.sh | sh

# Run Open WebUI container
echo "Running Open WebUI container"
sudo docker run -d -p 80:8080 -e OLLAMA_BASE_URLS=$OLLAMA_BASE_URLS -v open-webui:/app/backend/data --name open-webui --restart always ghcr.io/open-webui/open-webui:main

# Display the status of the Open WebUI container
echo "Displaying the status of the Open WebUI container"
sudo docker ps -f name=open-webui
