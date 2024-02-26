## Introduction

- LLM Deployment on Cloud VM

This document describes a set of scripts designed to deploy an 
LLM (Large Language Model) service. These scripts facilitate starting 
the service (`startServer.sh`), checking its status (`statusServer.sh`), 
and stopping the service (`stopServer.sh`).

## Installation & Setup

### Prerequisites

- A Linux-based system
- Python 3 installed
- pip (Python package manager)
- sudo privileges or root access (for apt-get)

### 1. Start Server

Starts the LLM service by installing necessary Python packages 
and running a FastAPI-based service in the background.

```bash
./startServer.sh
```

To download and prepare `startServer.sh` script for execution:

```bash
wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/llmServer.py
wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/startServer.sh
chmod +x startServer.sh
```

### 2. Check Server Status

Checks the status of the currently running LLM service. If the 
service is running, it outputs the contents of recent logs.

```bash
./statusServer.sh
```

To download and prepare `statusServer.sh` script for execution:

```bash
wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/statusServer.sh
chmod +x statusServer.sh
```

### 3. Stop Server

Stops the running LLM service by safely terminating all related processes.

```bash
./stopServer.sh
```

To download and prepare `stopServer.sh` script for execution:

```bash
wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/stopServer.sh
chmod +x stopServer.sh
```

## Testing the Server

Once the server is running, you can test the LLM service with 
the following `curl` command. This command sends a text generation 
request to the service, testing its operational status.

```bash
curl -s "http://{PUBLICIP}:5001/status" | jq .
```

```bash
curl -s -X POST http://{PUBLICIP}:5001/query \
     -H "Content-Type: application/json" \
     -d '{"prompt": "What is the Multi-Cloud?"}' | jq .
```

```bash
curl -s "http://{PUBLICIP}:5001/query?prompt=What+is+the+Multi-Cloud?" | jq .
```

Replace `{PUBLICIP}` with the public IP address of the server 
where the LLM service is running.

## Notes

- These scripts operate a Python-based LLM service using FastAPI and Uvicorn.
- Service logs are saved to llmServer.log by default, located in the same 
  directory as the startServer.sh script.
- Server testing utilizes the service's public IP address and port number 5001.
- Ensure you replace localhost with the actual public IP address of your server
  when testing from outside the server's local network.