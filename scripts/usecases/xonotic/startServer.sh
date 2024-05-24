#!/bin/bash

# This script should be run as root, so we check for root privileges
if [ "$EUID" -ne 0 ]; then
  echo "Please run as root"
  exit
fi

echo "[Start Xonotic FPS Game Server Installation]"

SECONDS=0

serverName=${1:-Xonotic-0.8.6-Cloud-Barista}
serverPort=${2:-26000}
numBot=${3:-2}
numMaxUser=32
FILE="xonotic-0.8.6.zip"
DIR="/root/Xonotic"
INSTALL_PATH="/root"
SERVICE_FILE="/etc/systemd/system/xonotic.service"
LOG_DIR="/var/log/xonotic"
XONOTIC_BINARY="xonotic-linux64-dedicated"
CONFIG_DIR="$INSTALL_PATH/.xonotic/data"

# Download and unzip Xonotic if it's not already present
if [ ! -f "$INSTALL_PATH/$FILE" ]; then
  apt-get update > /dev/null
  wget -O "$INSTALL_PATH/$FILE" "https://dl.unvanquished.net/share/xonotic/release/xonotic-0.8.6.zip"
fi

if [ ! -d "$DIR" ]; then
  apt install unzip -y
  unzip "$INSTALL_PATH/$FILE" -d "$INSTALL_PATH"
fi

# Create configuration file with user inputs
appendConfig="sv_public 1\nport $serverPort\nhostname \"$serverName\"\nmaxplayers $numMaxUser\nbot_number $numBot"
mkdir -p "$CONFIG_DIR"
cp "$DIR/server/server.cfg" "$CONFIG_DIR"
echo -e "${appendConfig}" >> "$CONFIG_DIR/server.cfg"

# Create systemd service file
echo "Creating systemd service file for Xonotic Server"
cat <<EOF > $SERVICE_FILE
[Unit]
Description=Xonotic Dedicated Server
After=network.target

[Service]
Type=simple
User=root
Group=root
ExecStart=$DIR/$XONOTIC_BINARY -dedicated
WorkingDirectory=$DIR
StandardOutput=file:$LOG_DIR/server.log
StandardError=file:$LOG_DIR/error.log
SyslogIdentifier=xonotic

[Install]
WantedBy=multi-user.target
EOF

# Create log directory
mkdir -p "$LOG_DIR"

# Reload systemd to recognize new service
systemctl daemon-reload

# Enable and start the service
systemctl enable xonotic.service
systemctl start xonotic.service

echo "Done! Xonotic server installed and started as a service. Elapsed time: $SECONDS seconds."

# Display status
systemctl status xonotic.service

# Get the public IP address
IP=$(curl -s https://api.ipify.org)

# Get the process ID of the Xonotic server
PID=$(pgrep -f xonotic-linux64-dedicated)

# Display the server information
echo "[Start Xonotic: complete] PID=$PID"
echo "Access to $IP:$serverPort by using your Xonotic Client"
echo "Hostname: $serverName"

