#!/bin/bash

echo "[Stop Xonotic FPS Game]"

echo ""
echo "[Current server.log]"
cat ~/Xonotic/server.log
echo ""

PID=$(ps -ef | grep [x]onotic | awk '{print $2}')
kill $PID
echo ""
echo "[Stop Xonotic] PID=$PID"

ps -ef | grep [x]onotic

echo ""
