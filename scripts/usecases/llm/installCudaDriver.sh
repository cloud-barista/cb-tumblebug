#!/bin/bash

# This script installs NVIDIA drivers and CUDA on Ubuntu 22.04.
# It downloads necessary files from NVIDIA's official website,
# installs the drivers and CUDA, sets the required environment variables,
# and reboots the system to apply changes.
# https://developer.nvidia.com/cuda-downloads?target_os=Linux&target_arch=x86_64&Distribution=Ubuntu&target_version=22.04&target_type=deb_local

# Check for NVIDIA GPU
echo "Checking for NVIDIA GPU..."
GPU_INFO=$(sudo lspci | grep -i nvidia)
if [ -z "$GPU_INFO" ]; then
  echo "No NVIDIA GPU detected or an error occurred. Exiting..."
  sudo lspci
  exit 1
else
  echo "NVIDIA GPU detected:"
  echo "$GPU_INFO"
  echo "Check root disk size"
  df -h / | awk '$NF=="/" {print "Total: "$2, "Available: "$4}'
fi

# Download and install CUDA repository pin file
echo "Downloading and installing CUDA repository pin file..."
if ! sudo wget -q https://developer.download.nvidia.com/compute/cuda/repos/ubuntu2204/x86_64/cuda-ubuntu2204.pin; then
  echo "Failed to download CUDA repository pin file. Exiting..."
  exit 1
fi
ls -lh cuda-ubuntu2204.pin
sudo mv cuda-ubuntu2204.pin /etc/apt/preferences.d/cuda-repository-pin-600

# Download and install CUDA repository package
echo "Downloading and installing CUDA repository package..."
if ! sudo wget -q https://developer.download.nvidia.com/compute/cuda/12.5.0/local_installers/cuda-repo-ubuntu2204-12-5-local_12.5.0-555.42.02-1_amd64.deb; then
  echo "Failed to download CUDA repository package. Exiting..."
  exit 1
fi

echo "Check root disk size"
df -h / | awk '$NF=="/" {print "Total: "$2, "Available: "$4}'
du -h cuda-repo-ubuntu2204-12-5-local_12.5.0-555.42.02-1_amd64.deb

if ! sudo dpkg -i cuda-repo-ubuntu2204-12-5-local_12.5.0-555.42.02-1_amd64.deb; then
  echo "Failed to install CUDA repository package. Exiting..."
  exit 1
fi
sudo cp /var/cuda-repo-ubuntu2204-12-5-local/cuda-*-keyring.gpg /usr/share/keyrings/

# Update package lists
echo "Updating package lists..."
sudo apt-get update -qq

# Install GCC
echo "Installing GCC..."
sudo DEBIAN_FRONTEND=noninteractive apt-get -y install gcc > /dev/null 2>&1
gcc --version

# Install CUDA
echo "Installing CUDA..."
sudo DEBIAN_FRONTEND=noninteractive apt-get -y install cuda-12-5 > /dev/null 2>&1

# Set environment variables
echo "Setting environment variables..."
echo 'export PATH=/usr/local/cuda-12.5/bin${PATH:+:${PATH}}' >> ~/.bashrc
echo 'export LD_LIBRARY_PATH=/usr/local/cuda-12.5/lib64${LD_LIBRARY_PATH:+:${LD_LIBRARY_PATH}}' >> ~/.bashrc

# Print LD_LIBRARY_PATH to verify
echo "Verifying LD_LIBRARY_PATH..."
. ~/.bashrc && echo "$LD_LIBRARY_PATH"

# Check for NVSwitch and install Fabric Manager if needed (required for H100, A100 multi-GPU with NVSwitch)
echo "Checking for NVSwitch topology..."
NVSWITCH_PCI=$(sudo lspci | grep -i -E "nvswitch|nvlink" 2>/dev/null)
NVSWITCH_DEV=$(ls /dev/nvidia-nvswitch* 2>/dev/null)

if [ -n "$NVSWITCH_PCI" ] || [ -n "$NVSWITCH_DEV" ]; then
  echo "NVSwitch detected. Installing NVIDIA Fabric Manager..."
  if [ -n "$NVSWITCH_PCI" ]; then
    echo "  PCI devices: $NVSWITCH_PCI"
  fi
  if [ -n "$NVSWITCH_DEV" ]; then
    echo "  Device nodes: $NVSWITCH_DEV"
  fi
  
  # Install Fabric Manager (version must match CUDA driver version)
  sudo DEBIAN_FRONTEND=noninteractive apt-get -y install nvidia-fabricmanager-555 > /dev/null 2>&1
  if [ $? -eq 0 ]; then
    echo "Enabling and starting nvidia-fabricmanager service..."
    sudo systemctl enable nvidia-fabricmanager
    sudo systemctl start nvidia-fabricmanager
    echo "Fabric Manager installed and enabled successfully."
  else
    echo "Warning: Failed to install nvidia-fabricmanager. Manual installation may be required."
  fi
else
  echo "No NVSwitch detected. Skipping Fabric Manager installation."
  echo "  (Fabric Manager is only needed for multi-GPU systems with NVSwitch, e.g., H100/A100 HGX)"
fi

# Notify rebooting the system is required
echo "Going to reboot the system to make driver works. [sudo reboot]"
echo "You can verify the setup by using [nvidia-smi] and [nvcc --version] after rebooting"
echo "For NVSwitch systems, also check [sudo systemctl status nvidia-fabricmanager]"

sudo reboot
