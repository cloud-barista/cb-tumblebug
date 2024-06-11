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
fi

# Download and install CUDA repository pin file
echo "Downloading and installing CUDA repository pin file..."
if ! sudo wget -q https://developer.download.nvidia.com/compute/cuda/repos/ubuntu2204/x86_64/cuda-ubuntu2204.pin; then
  echo "Failed to download CUDA repository pin file. Exiting..."
  exit 1
fi
sudo mv cuda-ubuntu2204.pin /etc/apt/preferences.d/cuda-repository-pin-600

# Download and install CUDA repository package
echo "Downloading and installing CUDA repository package..."
if ! sudo wget -q https://developer.download.nvidia.com/compute/cuda/12.5.0/local_installers/cuda-repo-ubuntu2204-12-5-local_12.5.0-555.42.02-1_amd64.deb; then
  echo "Failed to download CUDA repository package. Exiting..."
  exit 1
fi
sudo dpkg -i cuda-repo-ubuntu2204-12-5-local_12.5.0-555.42.02-1_amd64.deb
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
. ~/.bashrc && echo $LD_LIBRARY_PATH

# Reboot the system
echo "Rebooting the system..."
echo "You need to check [nvcc --version] [nvidia-smi] after rebooting"
sudo reboot
