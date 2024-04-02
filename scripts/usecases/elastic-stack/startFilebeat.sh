#!/bin/bash

# This script should be run as root, so we check for root privileges
if [ "$EUID" -ne 0 ]; then 
  echo "Please run as root"
  exit
fi

##############################################################################################
echo "[Start to install Filebeat]"

SECONDS=0

# Setup variables
# See FAQ on 2021 License change https://www.elastic.co/pricing/faq/licensing
# serverName=${1:-Filebeat-8.3.0-by-Cloud-Barista}     # Specify the server name
LOGSTASH_IP=${1:-localhost}                           # Specify Logstash IP
LOGSTASH_PORT="5044"                                  # Specify the port number for Logstash (default: 5044)
INSTALL_PATH="${HOME}"                                # The path where the Filebeat will be installed
ELASTIC_STACK_VERSION="8.3.0"                         # Specify the version you want to install

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

# Install prerequisites
echo "Installing prerequisites..."
eval "${PRE_INSTALL_CMD}"

# Change to the home directory
cd "${INSTALL_PATH}"

FILE_NAME="filebeat-${ELASTIC_STACK_VERSION}-${PACKAGE_FORMAT}"
FILE_URL="${BASE_URL}/beats/filebeat/${FILE_NAME}"

# Check if the file already exists
if [ ! -f "${FILE_NAME}" ]; then
    echo "Downloading ${FILE_NAME}..."
    sudo wget "${FILE_URL}"
else
    echo "${FILE_NAME} already exists."
fi

# Install the package
echo "Installing ${FILE_NAME}..."
eval "${INSTALL_CMD} ${FILE_NAME} > /dev/null 2>&1"

echo "Done to install Filebeat"


##############################################################################################
echo ""
echo "[Start to configure Filebeat]"
echo "[Note] Location of configuration files:"
echo "/etc/filebeat/filebeat.yml"

# Backup the original configuration files
echo "Backing up the original configuration files..."
sudo cp /etc/filebeat/filebeat.yml /etc/filebeat/filebeat.yml.orig

# Configure Filebeat
echo "Configuring Filebeat..."

# Set input for collecting log messages from files
sudo sed -i -e 's/- type: filestream/- type: log/' \
            -e 's/  enabled: false/  enabled: true/' \
            -e 's/  id: my-filestream-id/  #id: my-filestream-id/' \
            -e 's/    - \/var\/log\/\*.log/    - \/var\/log\/tumblebug\/\*.log/' /etc/filebeat/filebeat.yml

# Disable output to Elasticsearch
sudo sed -i '/output.elasticsearch:/,/hosts: \["localhost:9200"\]/s/^/#/' /etc/filebeat/filebeat.yml

# Uncomment 'output.logstash:'' and 'hosts: ["localhost:5044"]'
sudo sed -i -e 's/#output.logstash:/output.logstash:/' \
            -e 's/#hosts: \["localhost:5044"\]/hosts: ["localhost:5044"]/' /etc/filebeat/filebeat.yml

# Replace 'localhost:5044' with '${LOGSTASH_IP}:${LOGSTASH_PORT}'
sudo sed -i "s/localhost:5044/${LOGSTASH_IP}:${LOGSTASH_PORT}/" /etc/filebeat/filebeat.yml

# Enable logging to a file
if ! sudo grep -q 'path: /var/log/filebeat' /etc/filebeat/filebeat.yml; then
    echo '
logging.level: debug
logging.to_stderr: false
logging.to_syslog: false
logging.to_files: true
logging.files:
  path: /var/log/filebeat
  name: filebeat
  keepfiles: 7
  permissions: 0644
    ' | sudo tee -a /etc/filebeat/filebeat.yml > /dev/null
fi


##############################################################################################
echo "Enabling start-on-boot..."
sudo systemctl enable filebeat.service


##############################################################################################
echo "Starting Filebeat..."
sudo systemctl start filebeat.service

echo "Done! Filebeat installed and started as a service. Elapsed time: $SECONDS seconds."

# Display Filebeat status
sudo systemctl status filebeat.service --no-pager


##############################################################################################
echo "Displaying information about Filebeat..."

# # Get the public IP address
# IP=$(curl -s https://api.ipify.org)

# Get the process IDs of Filebeat
FILEBEAT_PID_LIST=($(pgrep -f filebeat))

# Display the PID list
echo "FILEBEAT_PID List: ${FILEBEAT_PID_LIST[@]}"

echo "Location of the log files collected by Filebeat:"
echo "/var/log/tumblebug/*.log"
