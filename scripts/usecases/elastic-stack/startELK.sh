#!/bin/bash

# This script should be run as root, so we check for root privileges
if [ "$EUID" -ne 0 ]; then 
  echo "Please run as root"
  exit
fi

##############################################################################################
echo "[Start to install ELK Stack (i.e., Elasticsearch, Logstash, and Kibana)]"

SECONDS=0

# Setup variables
# See FAQ on 2021 License change https://www.elastic.co/pricing/faq/licensing
serverName=${1:-ELK-Stack-8.3.0-by-Cloud-Barista} # Specify the server name
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

# Install prerequisites
echo "Installing prerequisites..."
eval "${PRE_INSTALL_CMD}"

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
    if [ ! -f "$file" ]; then
        echo "Downloading $file..."
        sudo wget "$url"
    else
        echo "$file already exists."
    fi

    # Install the package
    echo "Installing $file..."
    eval "${INSTALL_CMD} $file > /dev/null 2>&1"
done

echo "Done to install ELK Stack"


##############################################################################################
echo ""
echo "[Start to configure ELK Stack]"
echo "[Note] Location of configuration files:"
echo "/etc/elasticsearch/elasticsearch.yml"
echo "/etc/logstash/logstash.yml"
echo "/etc/kibana/kibana.yml"

# Backup the original configuration files
echo "Backing up the original configuration files..."
sudo cp /etc/elasticsearch/elasticsearch.yml /etc/elasticsearch/elasticsearch.yml.orig
sudo cp /etc/logstash/logstash.yml /etc/logstash/logstash.yml.orig
sudo cp /etc/kibana/kibana.yml /etc/kibana/kibana.yml.orig

# Configure Elasticsearch (security features: disabled)
echo "Configuring Elasticsearch..."
sudo sed -i 's/^xpack.security.enabled: true/xpack.security.enabled: false/' /etc/elasticsearch/elasticsearch.yml
sudo sed -i 's/^xpack.security.enrollment.enabled: true/xpack.security.enrollment.enabled: false/' /etc/elasticsearch/elasticsearch.yml
sudo sed -i '/xpack.security.http.ssl:/!b;n;s/enabled: true/enabled: false/' /etc/elasticsearch/elasticsearch.yml
sudo sed -i '/xpack.security.transport.ssl:/!b;n;s/enabled: true/enabled: false/' /etc/elasticsearch/elasticsearch.yml

# Configure Logstash
echo "Configuring Logstash..."
# Nothing changed in logstash.yml
# Add logstash.conf
sudo bash -c 'cat << EOF > /etc/logstash/conf.d/logstash-filebeat.conf
# Sample Logstash configuration for creating a simple
# Beats -> Logstash -> Elasticsearch pipeline.

input {
  beats {
    port => 5044
    host => "0.0.0.0"
  }
}

output {
  elasticsearch {
    hosts => ["http://localhost:9200"]
    index => "%{[@metadata][beat]}-%{[@metadata][version]}-%{+YYYY.MM.dd}"
    #user => "elastic"
    #password => "changeme"
  }
}
EOF'

# Configure Kibana
echo "Configuring Kibana..."
# Allow connections from remote users
sudo sed -i 's/^#server.host: "localhost"/server.host: "0.0.0.0"/' /etc/kibana/kibana.yml


##############################################################################################
echo "Enabling start-on-boot..."
sudo systemctl enable elasticsearch.service
sudo systemctl enable logstash.service
sudo systemctl enable kibana.service

##############################################################################################
echo "Starting Elasticsearch, Logstash, and kibana services..."
sudo systemctl start elasticsearch.service
sleep 3
sudo systemctl start logstash.service
sleep 3
sudo systemctl start kibana.service

echo "Done! ELK Stack installed and started as a service. Elapsed time: $SECONDS seconds."

# Display ELK Stack status
sudo systemctl status elasticsearch.service --no-pager
sudo systemctl status logstash.service --no-pager
sudo systemctl status kibana.service --no-pager


##############################################################################################
echo "Displaying information about the ELK Stack..."

# Get the public IP address
IP=$(curl -s https://api.ipify.org)

# Get the process IDs of ELK Stack
ELASTICSEARCH_PID_LIST=($(pgrep -f elasticsearch))
LOGSTASH_PID_LIST=($(pgrep -f logstash))
KIBANA_PID_LIST=($(pgrep -f kibana))

# Display the PID list
echo "ELASTICSEARCH_PID List: ${ELASTICSEARCH_PID_LIST[@]}"
echo "LOGSTASH_PID List: ${LOGSTASH_PID_LIST[@]}"
echo "KIBANA_PID List: ${KIBANA_PID_LIST[@]}"

# Display the access information
echo "[Start ELK Stack: complete]"
echo "Access to $IP:$KIBANA_PORT by using your Kibana"
echo "Hostname: $serverName"

# Display notice
echo ""
echo "[NOTICE]"
echo "The Kibana server may take some time to start up. Please wait for a moment."
