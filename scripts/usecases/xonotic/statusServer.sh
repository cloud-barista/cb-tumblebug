#!/bin/bash

echo "[Status Xonotic FPS Game]"

echo ""
echo "[Xonotic Server.log so far]"
cat ~/Xonotic/server.log
echo ""

PS=$(ps -ef | head -1; ps -ef | grep [x]onotic)
echo "[Process status of Xonotic]"
echo "$PS"

echo ""
