#!/bin/bash

# Need to be executed with sudo
# Param: serverName, serverPort, numBot
echo "[Start Xonotic FPS Game Server]"

SECONDS=0

serverName=${1:-Xonotic-0.8.5-by-Cloud-Barista}
serverPort=${2:-26000}
numBot=${3:-2}
numMaxUser=32

echo "Installing Xonotic to instance..."
FILE="xonotic-0.8.5.zip"

#InstallFilePath="https://.../$FILE"
InstallFilePath="https://github.com/garymoon/xonotic/releases/download/xonotic-v0.8.5/$FILE"

if test -f "$FILE"; then
        echo "$FILE exists."
else
        sudo apt-get update > /dev/null; wget $InstallFilePath
fi

DIR="Xonotic"

if test -d "$DIR"; then
        echo "$DIR directory exists."
else
        sudo apt install unzip -y; unzip $FILE
fi

appendConfig="port $serverPort\nhostname \"$serverName\"\nmaxplayers $numMaxUser\nbot_number $numBot"
sudo cp ~/Xonotic/server/server.cfg ~/.xonotic/data
sudo echo -e "${appendConfig}" >> ~/.xonotic/data/server.cfg

echo "Launching Xonotic dedicated server"

cd Xonotic/; nohup ./xonotic-linux64-dedicated 1>server.log 2>&1 &

echo "Done! elapsed time: $SECONDS"

IP=$(curl https://api.ipify.org)

PID=$(ps -ef | grep [x]onotic | awk '{print $2}')

cat ~/Xonotic/server.log

echo ""
echo "[Start Xonotic: complete] PID=$PID"
echo "Access to $IP:$serverPort by using your Xonotic Client"
echo "Hostname: $serverName"
