#!/bin/bash
SERVICE_NAME="llmServer"
SOURCE_FILE="$SERVICE_NAME".py
LOG_FILE="$SERVICE_NAME".log
VENV_PATH=venv_"$SERVICE_NAME" 

echo "[$SERVICE_NAME] Attempting to stop the LLM service..."

# Find the PID of the LLM service
PIDS=$(ps aux | grep "$SERVICE_NAME" | grep -v grep | awk '{print $2}')

if [ -z "$PIDS" ]; then
    echo "[$SERVICE_NAME] No LLM service is currently running."
else
    # Kill the LLM service processes
    for PID in $PIDS; do
        kill $PID
        if [ $? -eq 0 ]; then
            echo "[$SERVICE_NAME] Successfully stopped the LLM service (PID: $PID)."
        else
            echo "[$SERVICE_NAME] Failed to stop the LLM service (PID: $PID)."
        fi
    done
fi
