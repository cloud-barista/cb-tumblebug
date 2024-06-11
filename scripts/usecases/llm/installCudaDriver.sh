#!/bin/bash

# Ensure the script is run as root
if [ "$(id -u)" -ne 0 ]; then
  echo "This script must be run as root" >&2
  exit 1
fi

# Check for NVIDIA GPU
echo "Checking for NVIDIA GPU..."
sudo lspci | grep -i nvidia

# Download and install CUDA repository pin file
echo "Downloading and installing CUDA repository pin file..."
sudo wget https://developer.download.nvidia.com/compute/cuda/repos/ubuntu2204/x86_64/cuda-ubuntu2204.pin
sudo mv cuda-ubuntu2204.pin /etc/apt/preferences.d/cuda-repository-pin-600

# Download and install CUDA repository package
echo "Downloading and installing CUDA repository package..."
sudo wget https://developer.download.nvidia.com/compute/cuda/12.5.0/local_installers/cuda-repo-ubuntu2204-12-5-local_12.5.0-555.42.02-1_amd64.deb
sudo dpkg -i cuda-repo-ubuntu2204-12-5-local_12.5.0-555.42.02-1_amd64.deb
sudo cp /var/cuda-repo-ubuntu2204-12-5-local/cuda-*-keyring.gpg /usr/share/keyrings/

# Update package lists
echo "Updating package lists..."
sudo apt-get update -qq

# Install GCC
echo "Installing GCC..."
sudo DEBIAN_FRONTEND=noninteractive apt-get -y install gcc
gcc --version

# Install CUDA
echo "Installing CUDA..."
sudo apt-get -y install cuda-12-5

# Set environment variables
echo "Setting environment variables..."
echo 'export PATH=/usr/local/cuda-12.5/bin${PATH:+:${PATH}}' >> ~/.bashrc
echo 'export LD_LIBRARY_PATH=/usr/local/cuda-12.5/lib64${LD_LIBRARY_PATH:+:${LD_LIBRARY_PATH}}' >> ~/.bashrc

# Apply environment variables
echo "Applying environment variables..."
source ~/.bashrc

# Print LD_LIBRARY_PATH to verify
echo "Verifying LD_LIBRARY_PATH..."
echo $LD_LIBRARY_PATH

# Reboot the system
echo "Rebooting the system..."
sudo reboot
