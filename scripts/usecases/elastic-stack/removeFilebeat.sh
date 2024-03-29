#!/bin/bash

# This script should be run as root, so we check for root privileges
if [ "$EUID" -ne 0 ]; then 
  echo "Please run as root"
  exit
fi

##############################################################################################
echo "[Start to remove Filebeat]"

SECONDS=0

##############################################################################################
echo "Stop Filebeat"
sudo systemctl stop filebeat.service

##############################################################################################
echo "Disable start-on-boot"
sudo systemctl disable filebeat.service

echo "[Start to install Filebeat]"

SECONDS=0

# Setup variables
# See FAQ on 2021 License change https://www.elastic.co/pricing/faq/licensing
INSTALL_PATH="${HOME}"                                # The path where the Filebeat will be installed
ELASTIC_STACK_VERSION="8.3.0"                         # Specify the version you want to install

# Determine OS ID
OS_ID=$(grep '^ID=' /etc/os-release | cut -d= -f2)
echo "OS ID: ${OS_ID}"

# Setup variables and commands for installing and starting a server based on the operating system
case "${OS_ID}" in
  ubuntu* | debian*) 
    PACKAGE_FORMAT="amd64.deb"
    ;;

  centos* | rocky* | rhel* | fedora*)    
    PACKAGE_FORMAT="x86_64.rpm"
    ;;

  *) 
    echo "Unsupported distribution: ${OS_ID}"
    exit 1
    ;;
esac

# Change to the home directory
cd "${INSTALL_PATH}"

FILE_NAME="filebeat-${ELASTIC_STACK_VERSION}-${PACKAGE_FORMAT}"

# Check if the file exists
if [ -f "${FILE_NAME}" ]; then
    # Remove the file
    echo "Removing $FILE_NAME..."
    eval "sudo rm $FILE_NAME"
fi

# Remove Filebeat package
echo "Removing the installed Filebeat..."
case "${OS_ID}" in
  ubuntu* | debian*) 
    sudo apt remove --purge -y filebeat
    ;;

  centos* | rocky* | rhel* | fedora*)
    sudo rpm -e filebeat
    ;;

  *) 
    echo "Unsupported distribution: ${OS_ID}"
    exit 1
    ;;
esac

# Remove the Filebeat directories and configurations
echo "Removing the Filebeat directories and configurations..."
sudo rm -rf /etc/filebeat
sudo rm -rf /var/lib/filebeat
