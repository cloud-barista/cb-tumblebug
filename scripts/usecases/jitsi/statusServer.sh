#!/bin/bash

echo "[Status Jitsi]"

PS=$(ps -ef | head -1; ps -ef | grep jitsi)
echo "[Process status of Jitsi]"
echo "$PS"

echo ""
