#!/bin/bash

# This script should be run as root, so we check for root privileges
# if [ "$EUID" -ne 0 ]; then 
#   echo "Please run as root"
#   exit
# fi

##############################################################################################
echo "[Start to setup Ceph master node]"

SECONDS=0

# Setup variables
# serverName=${1:-ELK-Stack-8.3.0-by-Cloud-Barista}     # Specify the server name
CEPH_RELEASE_VERSION="18.2.0"                         # Specify the version you want to install
CEPH_DASHBOARD_PORT="8443"                            # Specify the port number for the dashboard (default: 8443)

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

# Install Ceph master node
curl --silent --remote-name --location https://download.ceph.com/rpm-${CEPH_RELEASE_VERSION}/el9/noarch/cephadm
chmod +x cephadm
sudo ./cephadm add-repo --release reef
sudo ./cephadm install
which cephadm

IP=$(hostname -I | awk '{print $1}')
echo $IP
sudo cephadm bootstrap --mon-ip $IP --ssh-user cb-user
sudo cephadm add-repo --release reef
sudo cephadm install ceph-common
sudo ceph status

sudo ceph telemetry on --license sharing-1-0
sudo ceph orch host ls --detail

##############################################################################################
echo "Displaying information about the Ceph dashboard..."

# Get the public IP address
EXTERNAL_IP=$(curl -s https://api.ipify.org)

# Display the result and the access information
echo "[Ceph master setup: complete]"
echo "Access to https://$EXTERNAL_IP:$CEPH_DASHBOARD_PORT to use the Ceph dashboard."

# Display notice
echo ""
echo "[NOTICE]"
echo "It may take some time to start up the dashboard. Please wait for a moment."
echo "The default username: admin"
echo "The default password: see the above, it was genereted during the installation process"
