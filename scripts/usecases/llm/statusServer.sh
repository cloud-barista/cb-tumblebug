#!/bin/bash

# Define script variables
SERVICE_NAME="llmServer"
SOURCE_FILE="$SERVICE_NAME".py
LOG_FILE="$SERVICE_NAME".log
VENV_PATH=venv_"$SERVICE_NAME" 

echo "[$SERVICE_NAME] Checking status of the LLM service..."

# Check if the LLM service is running
PID=$(ps aux | grep "$SERVICE_NAME" | grep -v grep | awk '{print $2}')

if [ -z "$PID" ]; then
    echo "[$SERVICE_NAME] LLM service is not running."
else
    echo "[$SERVICE_NAME] LLM service is running. PID: $PID"
    echo ""
    echo "[$SERVICE_NAME] Showing the last 20 lines of the log file ($LOG_FILE):"
    echo ""
    tail -n 20 "$LOG_FILE"
fi