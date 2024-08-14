#!/bin/bash

# README
# This script sets up an nginx web server, updates the default web page, and optionally uses a provided IP address.
#
# Usage:
#   ./script.sh [--ip IP_ADDRESS]
#
# Options:
#   --ip IP_ADDRESS  Optional: Specify the IP address to be displayed on the web page. If not provided, the external IP will be used.
#
# Steps:
# 1. Update package lists and install nginx.
# 2. Disable IPv6 settings in nginx configuration if necessary.
# 3. Start the nginx service.
# 4. Fetch the IP address (either from the parameter or from the external service).
# 5. Update the default nginx web page with custom content including the IP address.
# 6. Display the URL to access the web server.

# Function to display usage
usage() {
    echo "Usage: $0 [--ip IP_ADDRESS]"
    exit 1
}

# Parse command-line options
while [[ "$1" != "" ]]; do
    case $1 in
        --ip ) shift
               HOST_IP=$1
               ;;
        * )    usage
               exit 1
    esac
    shift
done

# If no IP address is provided, use the external IP address
if [ -z "${HOST_IP}" ]; then
    HOST_IP=$(curl -s https://api.ipify.org)
fi

# Update and install nginx
echo "Updating package lists..."
sudo apt-get update > /dev/null

echo "Installing nginx..."
sudo apt-get -y install nginx > /dev/null

# Check nginx version
nginx -v

# Disable IPv6 settings in nginx configuration
FILE="/etc/nginx/sites-enabled/default"

if [ -f "$FILE" ]; then
    echo "Disabling IPv6 settings in $FILE..."
    sudo sed -i '/listen \[::\]:80 default_server;/ s/^/#/' "$FILE"
else
    echo "Error: $FILE does not exist."
    exit 1
fi

# Start nginx service
echo "Starting nginx service..."
sudo service nginx start

# Function to update the HTML file
update_html_file() {
    local file=$1
    local host_ip=$2
    
    sudo sed -i "s/<\/title>/<\/title><meta http-equiv=\"refresh\" content=\"1\">/g" $file
    sudo sed -i "s/<h1>Welcome to nginx!/<h1><br><br>Welcome to Cloud-Barista<br><br>Host IP is<br>$host_ip<br><br>/g" $file
    sudo sed -i "s/Commercial support is available at/<h2>Check CB-Tumblebug MCI VM Location<\/h2>/g" $file
    sudo sed -i "s/Thank you for using nginx/Thank you for using Cloud-Barista and CB-Tumblebug/g" $file
    sudo sed -i "s|<a href=\"http://nginx.com/\">nginx.com</a>.</p>|<a href=\"https://www.geolocation.com/?ip=$host_ip#ipresult\">Check the Location of NGINX HOST</a>.</p>|g" $file
}

# HTML file path
HTML_FILE="/var/www/html/index.nginx-debian.html"

# Update the HTML file
echo "Updating HTML file at $HTML_FILE..."
update_html_file $HTML_FILE $HOST_IP

# Print the access URL
echo "WebServer is ready."
echo "http://$HOST_IP"
