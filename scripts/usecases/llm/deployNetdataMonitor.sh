#!/bin/bash

# This script installs Netdata monitoring agent on Ubuntu.
# Netdata provides real-time performance monitoring for systems and applications.
# https://www.netdata.cloud/

echo "=========================================="
echo "Netdata Monitoring Agent Installation"
echo "=========================================="

# Check if running on a supported OS
echo "Checking OS compatibility..."
if [ -f /etc/os-release ]; then
  . /etc/os-release
  echo "Detected OS: $NAME $VERSION"
else
  echo "Warning: Could not detect OS. Proceeding anyway..."
fi

# Check system resources
echo "Checking system resources..."
df -h / | awk '$NF=="/" {print "Disk - Total: "$2, "Available: "$4}'
free -h | awk '/Mem:/ {print "Memory - Total: "$2, "Available: "$7}'

# Download and install Netdata using official kickstart script
echo "Downloading and installing Netdata..."
if wget -O /tmp/netdata-kickstart.sh https://get.netdata.cloud/kickstart.sh; then
  echo "Running Netdata installation script..."
  sh /tmp/netdata-kickstart.sh --non-interactive --stable-channel
else
  echo "Failed to download Netdata kickstart script. Exiting..."
  exit 1
fi

# Verify installation
echo "Verifying Netdata installation..."
if command -v netdata &> /dev/null; then
  echo "Netdata installed successfully!"
  netdata -v
else
  echo "Warning: Netdata command not found. Installation may have failed."
  exit 1
fi

# Check Netdata service status
echo "Checking Netdata service status..."
sudo systemctl status netdata --no-pager || true

# Enable Netdata to start on boot
echo "Enabling Netdata service..."
sudo systemctl enable netdata

# Display access information
echo "=========================================="
echo "Netdata Installation Complete!"
echo "=========================================="
echo ""
echo "Access Netdata dashboard at:"
echo "  http://<your-server-ip>:19999"
echo ""
echo "Useful commands:"
echo "  sudo systemctl status netdata  - Check status"
echo "  sudo systemctl restart netdata - Restart service"
echo "  sudo systemctl stop netdata    - Stop service"
echo ""
