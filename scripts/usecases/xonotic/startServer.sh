#!/bin/bash

echo "[Start Xonotic FPS Game Server]"

SECONDS=0

echo "Installing Xonotic to instance..."
FILE="xonotic-0.8.2.zip"

#InstallFilePath="https://.../$FILE"
InstallFilePath="https://z.xnz.me/xonotic/builds/$FILE"

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

echo ""

echo "Launching Xonotic dedicated server"

cd Xonotic/; nohup ./xonotic-linux64-dedicated 1>server.log 2>&1 &

echo "Done! elapsed time: $SECONDS"

IP=$(curl https://api.ipify.org)

PID=$(ps -ef | grep [x]onotic | awk '{print $2}')

cat ~/Xonotic/server.log

echo ""
echo "[Start Xonotic: complete] PID=$PID"
echo "Access to $IP:26000 by using your Xonotic Client"
echo ""
