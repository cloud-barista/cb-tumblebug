#!/bin/bash

# This script should be run as root, so we check for root privileges
# if [ "$EUID" -ne 0 ]; then 
#   echo "Please run as root"
#   exit
# fi

##############################################################################################
echo "[Start to setup Ceph member node]"

SECONDS=0

# Setup variables
PUBKEY=${1:?"the ceph public key is required"} # public key

# Determine OS ID and Version
OS_ID=$(grep '^ID=' /etc/os-release | cut -d= -f2)
OS_VERSION=$(grep '^VERSION_ID=' /etc/os-release | cut -d= -f2 | tr -d '"')

# Check if the OS is Ubuntu 22.04
if [ "${OS_ID}" != "ubuntu" ] || [ "${OS_VERSION}" != "22.04" ]; then
  echo "This script has been confirmed to work on Ubuntu 22.04."
  exit 1
fi
# Install docker
sudo apt update
sudo apt install -y docker.io
sudo systemctl start docker
sudo systemctl enable docker
docker --version

echo ${PUBKEY} >> ~/.ssh/authorized_keys
chmod 600 ~/.ssh/authorized_keys

# Display the result
echo "[Ceph member node setup: complete]"

# Display notice
IP=$(hostname -I | awk '{print $1}')
HOSTNAME=$(hostname)
echo ""
echo "[NOTICE]"
echo "On the master node, execute below command "
echo "sudo ceph orch host add $HOSTNAME $IP"
