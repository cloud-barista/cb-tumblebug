#!/bin/bash
  
# Define script variables
SERVICE_NAME="llmServer"
SOURCE_FILE="$SERVICE_NAME".py
LOG_FILE="$SERVICE_NAME".log
VENV_PATH=venv_"$SERVICE_NAME"  # virtual environment path

echo "Checking source file: $SOURCE_FILE"
if [ -f "$SOURCE_FILE" ]; then
  echo "Loading [$SOURCE_FILE] file."
else
  echo "Source [$SOURCE_FILE] does not exist. Exiting."
  exit 1
fi

# Step 2: Update system and install Python3-pip
echo "[$SERVICE_NAME] Updating system and installing Python3-pip..."
sudo apt-get update > /dev/null
sudo apt-get install -y python3-pip python3-venv jq > /dev/null


echo "[$SERVICE_NAME] Checking for virtual environment..."
if [ ! -d "$VENV_PATH" ]; then
  echo "Creating virtual environment..."
  python3 -m venv $VENV_PATH
else
  echo "Virtual environment already exists."
fi

source $VENV_PATH/bin/activate


# Step 3: Install required Python packages
echo "[$SERVICE_NAME] Installing required Python packages..."
pip install -U fastapi uvicorn 
pip install -U langchain langchain-community
pip install -U gptcache 
sudo $VENV_PATH/bin/python -m pip install vllm
pip install openai==0.28.1 

# Check if the pip install was successful
if [ $? -ne 0 ]; then
    echo "[$SERVICE_NAME] Failed to install Python packages. Exiting."
    exit 1
fi

# Step 4: Run the Python script in the background using nohup and virtual environment's python
echo "[$SERVICE_NAME] Starting LLM service in the background..."
nohup $VENV_PATH/bin/python $SOURCE_FILE > $LOG_FILE 2>&1 &

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

echo ""
echo "[Test: replace localhost with IP address of the server]"
echo "curl -X POST http://localhost:5001/query -H \"Content-Type: application/json\" -d '{\"prompt\": \"What is the Multi-Cloud?\"}'"
curl -s -X POST http://localhost:5001/query \
-H "Content-Type: application/json" \
-d '{"prompt": "What is the Multi-Cloud?"}' | jq .

echo "http://localhost:5001/query?prompt=What is the Multi-Cloud?"
curl -s "http://localhost:5001/query?prompt=What+is+the+Multi-Cloud?" | jq .


echo ""
