#!/bin/bash

# Define script variables
SCRIPT_NAME=$(basename "$0")
SERVICE_NAME="runCloudLLM.py"

echo "[$SCRIPT_NAME] Attempting to stop the LLM service..."

# Find the PID of the LLM service
PIDS=$(ps aux | grep "$SERVICE_NAME" | grep -v grep | awk '{print $2}')

if [ -z "$PIDS" ]; then
    echo "[$SCRIPT_NAME] No LLM service is currently running."
else
    # Kill the LLM service processes
    for PID in $PIDS; do
        kill $PID
        if [ $? -eq 0 ]; then
            echo "[$SCRIPT_NAME] Successfully stopped the LLM service (PID: $PID)."
        else
            echo "[$SCRIPT_NAME] Failed to stop the LLM service (PID: $PID)."
        fi
    done
fi
