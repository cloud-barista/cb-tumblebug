#!/bin/bash

# Ray Worker Node Setup Script
# This script installs and configures a Ray worker node on Ubuntu
# Designed for unattended execution via SSH

# Default values
RAY_COMPONENT="minimal"   # Ray component to install (usually 'minimal' is sufficient for workers)
PUBLIC_IP=""              # IP address for this server (will be auto-detected if not provided)
HEAD_ADDRESS=""           # Address of the Ray head node (required)
HEAD_PORT="6379"          # Default port for Ray head node (can be overridden in HEAD_ADDRESS)

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -c|--component)
            RAY_COMPONENT="$2"
            shift 2
            ;;
        -i|--ip)
            PUBLIC_IP="$2"
            shift 2
            ;;            
        -h|--head)
            HEAD_ADDRESS="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [-c|--component COMPONENT] [-i|--ip IP_ADDRESS] [-h|--head HEAD_ADDRESS]"
            echo "Install component: minimal, all, etc. (default: minimal)"
            echo "HEAD_ADDRESS: Required, format should be IP:PORT (e.g., 10.0.0.1:6379)"
            exit 1
            ;;
    esac
done

# If PUBLIC_IP is not defined, get public IP for output information
if [ -z "$PUBLIC_IP" ]; then
    PUBLIC_IP=$(curl -s http://checkip.amazonaws.com || curl -s https://ipinfo.io/ip || curl -s https://api.ipify.org)
    if [ -z "$PUBLIC_IP" ]; then
    echo "Warning: Could not detect IP address, defaulting to localhost"
    PUBLIC_IP="localhost"
    fi
fi

# Check if head address is provided
if [ -z "$HEAD_ADDRESS" ]; then
    echo "ERROR: Head node address is required. Use -h or --head option."
    echo "Usage: $0 [-c|--component COMPONENT] [-h|--head HEAD_ADDRESS]"
    exit 1
fi

# Check if HEAD_ADDRESS is in the correct format
# If HEAD_ADDRESS does not contain a port, append the default port
if [[ ! "$HEAD_ADDRESS" =~ : ]]; then
    HEAD_ADDRESS="${HEAD_ADDRESS}:${HEAD_PORT}"
fi

echo "==== Ray Worker Node Setup ===="
echo "Ray Install Component: $RAY_COMPONENT"
echo "Head Node Address: $HEAD_ADDRESS"
echo "=========================="
echo

# Update package lists
echo "Updating package lists..."
sudo DEBIAN_FRONTEND=noninteractive apt update

# Install Python and pip
echo "Installing Python and pip..."
sudo DEBIAN_FRONTEND=noninteractive apt install python3-pip -y
if command -v needrestart &> /dev/null; then
    sudo NEEDRESTART_MODE=a sudo needrestart -r a
fi

echo "Installing Python-is-Python3..."
sudo DEBIAN_FRONTEND=noninteractive apt install python-is-python3 -y
if command -v needrestart &> /dev/null; then
    sudo NEEDRESTART_MODE=a sudo needrestart -r a
fi

# Add pip bin directory to PATH
echo "Updating PATH..."
if ! grep -q 'export PATH="$HOME/.local/bin:$PATH"' ~/.bashrc; then
    echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
fi
export PATH="$HOME/.local/bin:$PATH"

# Install Ray with specified component
echo "Installing Ray[$RAY_COMPONENT]..."
pip install -U "ray[$RAY_COMPONENT]"

# Start Ray worker
echo "Starting Ray worker node..."
echo "Connecting to head node at: $HEAD_ADDRESS"
echo "Self node IP address: $PUBLIC_IP"
echo "Excute command: ray start --address=\"$HEAD_ADDRESS\" --node-ip-address=\"$PUBLIC_IP\""
ray start --address="$HEAD_ADDRESS" --node-ip-address="$PUBLIC_IP"

# Check Ray status (give it a moment to start)
sleep 5

# Check Ray status
if ray status > /dev/null 2>&1; then
    echo "SUCCESS: Ray worker node setup complete!"
    echo "Connected to head node at: $HEAD_ADDRESS"
    echo "Run 'ray status' to see the cluster status."
    exit 0
else
    echo "ERROR: Ray worker node failed to connect to head node."
    echo "Please check the following:"
    echo "- Head node address is correct"
    echo "- Head node is running"
    echo "- Network connectivity between worker and head node"
    exit 1
fi
