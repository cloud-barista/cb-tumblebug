#!/bin/bash

# Define script variables
SCRIPT_NAME=$(basename "$0")
SERVICE_NAME="runCloudLLM.py"
LOG_FILE=~/llm_nohup.out

echo "[$SCRIPT_NAME] Checking status of the LLM service..."

# Check if the LLM service is running
PID=$(ps aux | grep "$SERVICE_NAME" | grep -v grep | awk '{print $2}')

if [ -z "$PID" ]; then
    echo "[$SCRIPT_NAME] LLM service is not running."
else
    echo "[$SCRIPT_NAME] LLM service is running. PID: $PID"
    echo "[$SCRIPT_NAME] Showing the last 20 lines of the log file ($LOG_FILE):"
    tail -n 20 "$LOG_FILE"
fi