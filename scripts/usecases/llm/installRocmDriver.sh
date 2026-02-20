#!/bin/bash
set -e

# ROCm version to install
ROCM_VERSION="7.0.1"
ROCM_BUILD="7.0.1.70001-1"

# APT options for silent installation
APT_OPTS="-y -q -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-confold"

# Remove any existing AMD GPU drivers and ROCm packages
echo "Cleaning previous AMD installations..."
sudo env DEBIAN_FRONTEND=noninteractive apt-get remove --purge $APT_OPTS amdgpu-install amdgpu-dkms amdgpu rocm-dev rocm-libs rocm-core > /dev/null 2>&1 || true
sudo env DEBIAN_FRONTEND=noninteractive apt-get autoremove $APT_OPTS > /dev/null 2>&1 || true
sudo rm -f /etc/apt/sources.list.d/amdgpu.list /etc/apt/sources.list.d/rocm.list

# Install kernel headers and build tools for DKMS driver compilation
echo "Installing required dependencies..."
sudo env DEBIAN_FRONTEND=noninteractive apt-get update -q
sudo env DEBIAN_FRONTEND=noninteractive apt-get install $APT_OPTS "linux-headers-$(uname -r)" "linux-modules-extra-$(uname -r)" python3-setuptools python3-wheel > /dev/null

# Add user to render and video groups for GPU access
TARGET_USER=${SUDO_USER:-$USER}
sudo usermod -a -G render,video $TARGET_USER

# Download ROCm installer package from AMD repository
echo "Downloading ROCm ${ROCM_VERSION}..."
cd /tmp
wget -q "https://repo.radeon.com/amdgpu-install/${ROCM_VERSION}/ubuntu/jammy/amdgpu-install_${ROCM_BUILD}_all.deb"

if [ ! -f "amdgpu-install_${ROCM_BUILD}_all.deb" ]; then
    echo "Error: Download failed. Version $ROCM_VERSION not found."
    exit 1
fi

# Install amdgpu-install tool and add AMD repositories
sudo env DEBIAN_FRONTEND=noninteractive apt-get install $APT_OPTS "./amdgpu-install_${ROCM_BUILD}_all.deb" > /dev/null
sudo env DEBIAN_FRONTEND=noninteractive apt-get update -q

# Install DKMS driver and ROCm packages
echo "Building AMD driver and ROCm (This may take 10-15 mins)..."
sudo env DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a apt-get install $APT_OPTS amdgpu-dkms rocm

# Configure library paths and update initramfs for driver loading
echo "Configuring environment and initramfs..."
echo "/opt/rocm/lib" | sudo tee /etc/ld.so.conf.d/rocm.conf > /dev/null
echo "/opt/rocm/lib64" | sudo tee -a /etc/ld.so.conf.d/rocm.conf > /dev/null
sudo ldconfig

if ! grep -q "/opt/rocm/bin" /home/$TARGET_USER/.bashrc; then
    echo 'export PATH=$PATH:/opt/rocm/bin:/opt/rocm/opencl/bin' >> /home/$TARGET_USER/.bashrc
fi

sudo sed -i '/blacklist amdgpu/d' /etc/modprobe.d/*.conf 2>/dev/null || true
# Update initramfs to load amdgpu driver on boot
sudo update-initramfs -uk all > /dev/null 2>&1

# Reboot system to load the new driver
if [ "$AUTO_REBOOT" = true ]; then
    echo "Rebooting in 5 seconds... (SSH will disconnect, verify with nvidia-smi after ~60s)"
    nohup bash -c 'sleep 5 && sudo reboot' > /dev/null 2>&1 &
    exit 0
else
    echo "REBOOT REQUIRED: run 'sudo reboot' to complete installation."
fi