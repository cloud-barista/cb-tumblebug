#!/bin/bash

# This script should be run as root, so we check for root privileges
if [ "$EUID" -ne 0 ]; then 
  echo "Please run as root"
  exit
fi

##############################################################################################
echo "[Start to remove ELK Stack]"

SECONDS=0

##############################################################################################
echo "Stop Elasticsearch, Logstash, and kibana services"
sudo systemctl stop elasticsearch.service
sudo systemctl stop logstash.service
sudo systemctl stop kibana.service

##############################################################################################
echo "Disable start-on-boot"
sudo systemctl disable elasticsearch.service
sudo systemctl disable logstash.service
sudo systemctl disable kibana.service

# Setup variables
# See FAQ on 2021 License change https://www.elastic.co/pricing/faq/licensing
serverName=${1:-Elastic-Stack-8.3.0-by-Cloud-Barista} # Specify the server name
INSTALL_PATH="${HOME}"                                # The path where the ELK Stack will be installed
ELASTIC_STACK_VERSION="8.3.0"                         # Specify the version you want to install
KIBANA_PORT="5601"                                    # Specify the port number for Kibana (default: 5601)

# Determine OS ID
OS_ID=$(grep '^ID=' /etc/os-release | cut -d= -f2)
echo "OS ID: ${OS_ID}"

# Setup variables and commands for installing and starting a server based on the operating system
case "${OS_ID}" in
  ubuntu* | debian*) 
    
    BASE_URL="https://artifacts.elastic.co/downloads"
    PACKAGE_FORMAT="amd64.deb"
    INSTALL_CMD="sudo dpkg -i"
    PRE_INSTALL_CMD="sudo DEBIAN_FRONTEND=noninteractive apt update -qq && sudo DEBIAN_FRONTEND=noninteractive apt install -qq -y openjdk-11-jdk < /dev/null > /dev/null"
    ;;

  centos* | rocky* | rhel* | fedora*)    
    BASE_URL="https://artifacts.elastic.co/downloads"
    PACKAGE_FORMAT="x86_64.rpm"
    INSTALL_CMD="sudo rpm -i"
    PRE_INSTALL_CMD="sudo yum -q -y update && sudo yum -q -y install java-11-openjdk"
    ;;

  *) 
    echo "Unsupported distribution: ${OS_ID}"
    exit 1
    ;;
esac

# Define file names and URLs in an associative array
declare -A FILES=(
    ["elasticsearch-${ELASTIC_STACK_VERSION}-${PACKAGE_FORMAT}"]="${BASE_URL}/elasticsearch/elasticsearch-${ELASTIC_STACK_VERSION}-${PACKAGE_FORMAT}"
    ["logstash-${ELASTIC_STACK_VERSION}-${PACKAGE_FORMAT}"]="${BASE_URL}/logstash/logstash-${ELASTIC_STACK_VERSION}-${PACKAGE_FORMAT}"
    ["kibana-${ELASTIC_STACK_VERSION}-${PACKAGE_FORMAT}"]="${BASE_URL}/kibana/kibana-${ELASTIC_STACK_VERSION}-${PACKAGE_FORMAT}"
)

# Change to the home directory
cd "${INSTALL_PATH}"

# Loop through the array and process each file
for file in "${!FILES[@]}"; do
    url=${FILES[$file]}

    # Check if the file already exists
    if [ -f "$file" ]; then
        # Remove the file
        echo "Removing $file..."
        eval "sudo rm $file"
    fi
done

# Remove ELK Stack package
echo "Removing the installed ELK Stack..."
case "${OS_ID}" in
  ubuntu* | debian*) 
    sudo apt remove --purge -y elasticsearch logstash kibana    
    ;;

  centos* | rocky* | rhel* | fedora*)    
    sudo rpm -e elasticsearch logstash kibana
    ;;

  *) 
    echo "Unsupported distribution: ${OS_ID}"
    exit 1
    ;;
esac

# Remove the ELK Stack directories and configurations
echo "Removing the ELK Stack directories and configurations..."
sudo rm -rf /etc/elasticsearch
sudo rm -rf /etc/logstash
sudo rm -rf /etc/kibana
sudo rm -rf /var/lib/elasticsearch
sudo rm -rf /var/lib/logstash
sudo rm -rf /var/lib/kibana
