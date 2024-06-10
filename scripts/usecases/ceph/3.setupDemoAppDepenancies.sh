#!/bin/bash

# This script should be run as root, so we check for root privileges
# if [ "$EUID" -ne 0 ]; then 
#   echo "Please run as root"
#   exit
# fi

##############################################################################################
echo "[Start to setup demo app dependency]"

SECONDS=0

# Setup variables

# Determine OS ID and Version
OS_ID=$(grep '^ID=' /etc/os-release | cut -d= -f2)
OS_VERSION=$(grep '^VERSION_ID=' /etc/os-release | cut -d= -f2 | tr -d '"')

# Check if the OS is Ubuntu 22.04
if [ "${OS_ID}" != "ubuntu" ] || [ "${OS_VERSION}" != "22.04" ]; then
  echo "This script has been confirmed to work on Ubuntu 22.04."
  exit 1
fi

# Install dependencies
sudo apt update
sudo apt install python3-pip
sudo pip3 install Faker
sudo pip3 install flask

# Display the result
echo "[Demo app dependency setup: complete]"

# Display notice
echo ""
echo "[NOTICE]"
echo "Execute below command"
echo "sudo python3 app.py"
