#!/bin/bash

# Ray Head Node Setup Script
# This script installs and configures a Ray head node on Ubuntu
# Designed for unattended execution via SSH
# Based on https://docs.ray.io/en/latest/ray-overview/installation.html

# Default values
RAY_COMPONENT="all"       # Ray component to install (core/air/tune/rllib/serve/all)
PUBLIC_IP=""              # IP address for this server (will be auto-detected if not provided)
INSTALL_METRICS="yes"     # Install and configure Prometheus and Grafana metrics

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
        -m|--metrics)
            INSTALL_METRICS="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [-c|--component COMPONENT] [-i|--ip IP_ADDRESS] [-m|--metrics yes|no]"
            echo "Install component: core, air, tune, rllib, serve, all (default setting: all)"
            echo "Metrics: Install Prometheus and Grafana metrics (default: yes)"
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

# Add pip bin directory to PATH
echo "Updating PATH..."
if ! grep -q 'export PATH="$HOME/.local/bin:$PATH"' ~/.bashrc; then
    echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
fi
export PATH="$HOME/.local/bin:$PATH"

echo "==== Ray Head Node Setup ===="
echo "Ray Install Component: $RAY_COMPONENT"
echo "Server IP: $PUBLIC_IP"
echo "=========================="
echo

# Update package lists
echo "Updating package lists..."
sudo DEBIAN_FRONTEND=noninteractive apt update

# Install Python and pip
echo "Installing Python and pip..."
sudo DEBIAN_FRONTEND=noninteractive apt install python3-pip python-is-python3 -y

# Install Ray with specified component
echo "Installing Ray[$RAY_COMPONENT]..."
pip install -U "ray[$RAY_COMPONENT]"

# Configure Grafana iframe host
echo "Configuring Grafana iframe host..."
export RAY_GRAFANA_IFRAME_HOST="http://${PUBLIC_IP}:3000"
echo "export RAY_GRAFANA_IFRAME_HOST=\"http://${PUBLIC_IP}:3000\"" >> ~/.bashrc


# Start Ray head node
echo "Starting Ray head node..."
ray start --head --port=6379 --dashboard-host=0.0.0.0 --dashboard-port=8265

# Check Ray status (give it a moment to start)
sleep 5

# Check Ray status
if ray status > /dev/null 2>&1; then
    echo "SUCCESS: Ray head node setup complete!"
    
    # Launch Prometheus and install Grafana if metrics are enabled
    if [ "$INSTALL_METRICS" = "yes" ]; then
        echo "Setting up metrics (Prometheus and Grafana)..."
        
        # Launch Prometheus using Ray's built-in command
        echo "Launching Prometheus..."
        ray metrics launch-prometheus

        # Create needed directories for Grafana
        echo "Setting up Grafana directories..."
        mkdir -p /tmp/ray/session_latest/metrics/grafana/provisioning/plugins
        mkdir -p /tmp/ray/session_latest/metrics/grafana/provisioning/alerting
        
        # Install Grafana
        echo "Installing Grafana..."
        sudo DEBIAN_FRONTEND=noninteractive apt-get install -y adduser libfontconfig1 musl
        wget -q https://dl.grafana.com/enterprise/release/grafana-enterprise_11.5.2_amd64.deb
        sudo dpkg -i grafana-enterprise_11.5.2_amd64.deb
        rm -f grafana-enterprise_11.5.2_amd64.deb
        
        # Set permissions for Grafana
        USER=$(whoami)
        sudo chown -R $USER:$USER /usr/share/grafana
        
        # Start Grafana server
        echo "Starting Grafana server..."
        nohup grafana-server --homepath /usr/share/grafana --config /tmp/ray/session_latest/metrics/grafana/grafana.ini web > grafana.log 2>&1 &
        
        # Give Grafana a moment to start
        sleep 5
        
        echo "Metrics setup complete!"
        echo "grafana.log available at: $(pwd)/grafana.log"
    fi
    
    # Print connection information
    echo
    echo "===== Ray Cluster Information ====="
    echo "[Add worker - Public IP] ray start --address='${PUBLIC_IP}:6379'"
    PRIVATE_IP=$(hostname -I | awk '{print $1}')    
    echo "[Add worker - Private IP] ray start --address='${PRIVATE_IP}:6379'"
    echo "[Dashboard] http://${PUBLIC_IP}:8265"
    
    if [ "$INSTALL_METRICS" = "yes" ]; then
        echo "[Prometheus] http://${PUBLIC_IP}:9090"
        echo "[Grafana] http://${PUBLIC_IP}:3000"
    fi
    
    echo "Ray logs available at: /tmp/ray/session_latest/logs/"
    exit 0
else
    echo "ERROR: Ray head node failed to start properly."
    echo "Check logs at: /tmp/ray/session_latest/logs/"
    exit 1
fi

exit 0