#!/bin/bash

echo "[Start Xonotic FPS Game Server Installation]"

SECONDS=0

serverName=${1:-Xonotic-0.8.6-Cloud-Barista}
serverPort=${2:-26000}
numBot=${3:-2}
numMaxUser=32
FILE="xonotic-0.8.6.zip"
DOWNLOADLINK="https://dl.unvanquished.net/share/xonotic/release/xonotic-0.8.6.zip"
DIR="$HOME/Xonotic"
INSTALL_PATH="$HOME"
SERVICE_FILE="$HOME/.config/systemd/user/xonotic.service"
LOG_DIR="$HOME/.xonotic/logs"
XONOTIC_BINARY="xonotic-linux64-dedicated"
CONFIG_DIR="$HOME/.xonotic/data"

# Create necessary directories
mkdir -p "$INSTALL_PATH" "$DIR" "$LOG_DIR" "$CONFIG_DIR"

# Download and unzip Xonotic if it's not already present
if [ ! -f "$INSTALL_PATH/$FILE" ]; then
  sudo apt-get update > /dev/null
  sudo apt install unzip -y
  wget -O "$INSTALL_PATH/$FILE" "$DOWNLOADLINK"
  chmod +x "$INSTALL_PATH/$FILE"
fi

unzip "$INSTALL_PATH/$FILE"


# Ensure the binary exists and is executable
if [ ! -f "$DIR/$XONOTIC_BINARY" ]; then
  echo "Xonotic binary not found at $DIR/$XONOTIC_BINARY"
  exit 1
fi
chmod +x "$DIR/$XONOTIC_BINARY"

# Create configuration file with user inputs
appendConfig="port $serverPort\nhostname \"$serverName\"\nmaxplayers $numMaxUser\nbot_number $numBot"
if [ -f "$DIR/server/server.cfg" ]; then
  cp "$DIR/server/server.cfg" "$CONFIG_DIR"
else
  echo "server.cfg not found at $DIR/server/server.cfg"
  exit 1
fi
echo -e "${appendConfig}" >> "$CONFIG_DIR/server.cfg"

# Create systemd service file
echo "Creating systemd service file for Xonotic Server"
mkdir -p "$(dirname $SERVICE_FILE)"
cat <<EOF > $SERVICE_FILE
[Unit]
Description=Xonotic Dedicated Server
After=network.target

[Service]
Type=simple
ExecStart=$DIR/$XONOTIC_BINARY -dedicated
WorkingDirectory=$DIR
StandardOutput=file:$LOG_DIR/server.log
StandardError=file:$LOG_DIR/error.log
SyslogIdentifier=xonotic

[Install]
WantedBy=default.target
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
