#!/bin/bash
  
# Define script variables
SERVICE_NAME="llmServer"
SOURCE_FILE="$SERVICE_NAME".py
LOG_FILE="$SERVICE_NAME".log
VENV_PATH=venv_"$SERVICE_NAME"  # virtual environment path

IP="localhost"
PORT="5000"
MODEL="tiiuae/falcon-7b-instruct"
while [[ "$#" -gt 0 ]]; do
    case $1 in
        --ip) IP="$2"; shift ;;
        --port) PORT="$2"; shift ;;
        --model) MODEL="$2"; shift ;;
        *) echo "Unknown parameter passed: $1"; exit 1 ;;
    esac
    shift
done

echo "Using IP: $IP"
echo "Using PORT: $PORT"
echo "Using MODEL: $MODEL"

echo "Checking source file: $SOURCE_FILE"
if [ -f "$SOURCE_FILE" ]; then
  echo "Loading [$SOURCE_FILE] file."
else
  echo "Source [$SOURCE_FILE] does not exist. Exiting."
  exit 1
fi

# Step 2: Update system and install Python3-pip
echo "[$SERVICE_NAME] Updating system and installing Python3-pip..."
export DEBIAN_FRONTEND=noninteractive
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
pip install -U -q flask
pip install -U -q langchain langchain-community
pip install -U -q gptcache 
sudo $VENV_PATH/bin/python -m pip install -U -q vllm
pip install -q openai==0.28.1 

# Check if the pip install was successful
if [ $? -ne 0 ]; then
    echo "[$SERVICE_NAME] Failed to install Python packages. Exiting."
    exit 1
fi

# Step 4: Run the Python script in the background using nohup and virtual environment's python
echo "[$SERVICE_NAME] Starting LLM service in the background..."
nohup $VENV_PATH/bin/python $SOURCE_FILE $@ > $LOG_FILE 2>&1 &
# Sleep for 20 seconds to allow the service to start
sleep 20

echo "[$SERVICE_NAME] Checking status of the LLM service..."

# Check if the LLM service is running
echo "[$SERVICE_NAME] last 30 lines of the log file ($LOG_FILE):"
echo "cat $LOG_FILE"
tail -n 30 "$LOG_FILE"

PID=$(ps aux | grep "$SERVICE_NAME" | grep -v grep | awk '{print $2}')
if [ -z "$PID" ]; then
    echo "[$SERVICE_NAME] LLM service is not running."
    exit 1
else
    echo ""
    echo "[$SERVICE_NAME] LLM service is running. PID: $PID"
    echo ""
fi

echo ""
echo "[Test: use your IP address of the server]"

# Test the status endpoint
echo "Testing status endpoint:"
cmd="curl -s http://$IP:$PORT/status"
echo $cmd
response=$($cmd)
echo $response | jq -R 'fromjson? // .'
echo ""

# Test the prompt endpoint with a POST request
echo "Testing prompt endpoint with POST request:"
cmd="curl -s -X POST http://$IP:$PORT/prompt -H \"Content-Type: application/json\" -d '{\"input\": \"What is the Multi-Cloud?\"}'"
echo $cmd
response=$($cmd)
echo $response | jq -R 'fromjson? // .'
echo ""

# Test the prompt endpoint with a GET request
echo "Testing prompt endpoint with GET request:"
cmd="curl -s \"http://$IP:$PORT/prompt?input=What+is+the+Multi-Cloud?\""
echo $cmd
response=$($cmd)
echo $response | jq -R 'fromjson? // .'
echo ""