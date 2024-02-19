
# LLM Service Management Scripts

This document describes a set of scripts designed to manage an LLM (Large Language Model) service. These scripts facilitate starting the service (`startServer.sh`), checking its status (`statusServer.sh`), and stopping the service (`stopServer.sh`).

## Prerequisites

- A Linux-based system
- Python 3 installed
- pip (Python package manager)
- sudo privileges or root access

## Installation & Setup

### 1. Start Server

Starts the LLM service by installing necessary Python packages and running a FastAPI-based service in the background.

```bash
sudo ./startServer.sh
```

To download and prepare `startServer.sh` script for execution:

```bash
wget https://example.com/path/to/startServer.sh
chmod +x startServer.sh
```

### 2. Check Server Status

Checks the status of the currently running LLM service. If the service is running, it outputs the contents of recent logs.

```bash
./statusServer.sh
```

To download and prepare `statusServer.sh` script for execution:

```bash
wget https://example.com/path/to/statusServer.sh
chmod +x statusServer.sh
```

### 3. Stop Server

Stops the running LLM service by safely terminating all related processes.

```bash
./stopServer.sh
```

To download and prepare `stopServer.sh` script for execution:

```bash
wget https://example.com/path/to/stopServer.sh
chmod +x stopServer.sh
```

## Testing the Server

Once the server is running, you can test the LLM service with the following `curl` command. This command sends a text generation request to the service, testing its operational status.

```bash
curl -X POST http://{PUBLICIP}:5001/v1/generateText \
     -H "Content-Type: application/json" \
     -d '{"prompt": "Who is president of US?"}'
```

Replace `{PUBLICIP}` with the public IP address of the server where the LLM service is running.

## Notes

- These scripts operate a Python-based LLM service using FastAPI and Uvicorn.
- Service logs are saved to `~/llm_nohup.out` by default.
- Server testing utilizes the service's public IP address and port number `5001`.
