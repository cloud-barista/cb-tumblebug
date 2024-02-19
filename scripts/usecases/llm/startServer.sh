#!/bin/bash

# Define script variables
SCRIPT_NAME=$(basename "$0")
LLM_PYTHON_FILE=~/runCloudLLM.py
LOG_FILE=~/llm_nohup.out

# Step 1: Check for root/sudo
if [[ $EUID -ne 0 ]]; then
   echo "This script must be run as root or with sudo" 
   exit 1
fi

# Step 2: Update system and install Python3-pip
echo "[$SCRIPT_NAME] Updating system and installing Python3-pip..."
apt-get update
apt-get install -y python3-pip

# Step 3: Install required Python packages
echo "[$SCRIPT_NAME] Installing required Python packages..."
pip3 install openai==0.28.1 langchain -U langchain-community gptcache

# Check if the pip install was successful
if [ $? -ne 0 ]; then
    echo "[$SCRIPT_NAME] Failed to install Python packages. Exiting."
    exit 1
fi

# Step 4: Create the Python script for LLM
echo "[$SCRIPT_NAME] Creating the Python script for LLM..."
cat <<EOF > $LLM_PYTHON_FILE
from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse
import langchain
from langchain_community.llms import VLLM
import uvicorn

app = FastAPI()

llm = VLLM(model="tiiuae/falcon-7b-instruct",
           trust_remote_code=True,
           max_new_tokens=50,
           temperature=0.6)

@app.get("/")
def read_root():
    return {"Hello": "World"}

@app.post("/v1/generateText")
async def generateText(request: Request) -> JSONResponse:
    request_dict = await request.json()
    prompt = request_dict.get("prompt", "")
    output = llm(prompt)
    return JSONResponse(content={"text": output})

EOF

# Step 5: Run the Python script in the background
echo "[$SCRIPT_NAME] Starting LLM service in the background..."
nohup python3 $LLM_PYTHON_FILE > $LOG_FILE 2>&1 &

echo "[$SCRIPT_NAME] LLM service started. Logs are available at $LOG_FILE"
