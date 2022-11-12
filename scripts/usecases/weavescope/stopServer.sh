#!/bin/bash

echo "[Stop Weave Scope Cluster Monitoring]"

PID=$(ps -ef | grep scope | awk '{print $2}')
sudo scope stop
echo "[Stop scope] PID=$PID"
echo "[Check scope Process]"
sleep 2
ps -ef | grep scope

echo ""