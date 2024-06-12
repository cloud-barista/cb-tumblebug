#!/bin/bash

# Check NVIDIA driver
echo "Checking NVIDIA driver with nvidia-smi"
nvidia-smi

# Install Ollama
echo_message "Installing Ollama"
curl -fsSL https://ollama.com/install.sh | sh

# Modify Ollama service file
echo_message "Modifying Ollama service file"
sudo sed -i '/\[Service\]/a Environment="OLLAMA_HOST=0.0.0.0:3000"' /etc/systemd/system/ollama.service
sudo sed -i 's/User=ollama/User=root/' /etc/systemd/system/ollama.service
sudo sed -i 's/Group=ollama/Group=root/' /etc/systemd/system/ollama.service

# Reload and restart Ollama service
echo_message "Reloading and restarting Ollama service"
sudo systemctl daemon-reload
sudo systemctl restart ollama

# Check Ollama service status
echo_message "Checking Ollama service status"
sudo systemctl status ollama --no-pager

# List models on Ollama
echo_message "Listing models on Ollama"
OLLAMA_HOST=0.0.0.0:3000 ollama list
