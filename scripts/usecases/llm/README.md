

# CB-Tumblebug LLM Deployment Scripts

This repository contains scripts for deploying Large Language Models (LLMs) on multiple cloud environments using the CB-Tumblebug open-source project. The scripts focus on deploying Ollama, an open-source LLM, as well as setting up the necessary CUDA drivers and the OpenWebUI for LLM management.

- Setup NVIDIA GPU driver and CUDA toolkit (installCudaDriver.sh)
- Install Ollama cluster (deployOllama.sh)
- Pull LLM models
- Install Open WebUI for Ollama cluster

These scripts provide a straightforward way to deploy and manage LLMs on multiple cloud environments using CB-Tumblebug. By automating the installation of necessary drivers and the deployment of both the LLM and management interface, these scripts significantly reduce the complexity involved in setting up and maintaining LLM services in a multi-cloud setup.

**Deploy a Multi-Cloud Infra with GPUs and Enjoy muiltple LLMs in parallel (YouTube)**
[![Multi-Cloud LLMs in parallel](https://github.com/cloud-barista/cb-tumblebug/assets/5966944/e15feb67-ba02-4066-af62-d9f8e8330a63)](https://www.youtube.com/watch?v=SD9ZoT_OZpQ)

For more information, please refer to the [CB-Tumblebug documentation](https://github.com/cloud-barista/cb-tumblebug).

If you are looking for LLM serving with LangChain and vLLM, go to the [Alternative section](#alternative).


## Prerequisites

- CB-Tumblebug installed and configured (recommended)
- Access to cloud instances with GPUs
- Bash shell environment
- Ubuntu 22.04 for CUDA driver installation

## Scripts Overview

### 1. installCudaDriver.sh

This script installs the CUDA driver required for running LLMs on GPU instances. **Note:** This script works only on Ubuntu 22.04.

#### Usage

```bash
curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/installCudaDriver.sh | sh
```

#### Description

The `installCudaDriver.sh` script automates the installation of the NVIDIA CUDA driver. This is essential for leveraging GPU acceleration when running LLMs, providing significant performance improvements.

### 2. deployOllama.sh

This script deploys the Ollama LLM on the specified cloud environment.

#### Usage

```bash
curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/deployOllama.sh | sh
```

#### Description

The `deployOllama.sh` script handles the deployment of the Ollama LLM. It sets up the necessary environment, downloads the required files, and configures the service to run in a multi-cloud setup using CB-Tumblebug. The endpoint for accessing Ollama will be provided in the format `$$Func(GetPublicIP(target=this, prefix=http://, postfix=:3000))`.

#### Example for Downloading and Setting Up LLM Models

Using CB-Tumblebug remote commands, you can download and set up LLM models with the following command:

```bash
OLLAMA_HOST=0.0.0.0:3000 ollama pull $$Func(AssignTask(task='llama3, solar, mistral, phi3, gemma, mixtral, llava, yi, falcon2, llama2'))
```

Without CB-Tumblebug, you can use following command directly.

```bash
OLLAMA_HOST=0.0.0.0:3000 ollama pull solar
```

### 3. deployOpenWebUI.sh

This script sets up the OpenWebUI for managing and interacting with the deployed LLMs.

#### Usage

Using CB-Tumblebug remote commands:

```bash
wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/deployOpenWebUI.sh; chmod +x ~/deployOpenWebUI.sh; sudo ~/deployOpenWebUI.sh $$Func(GetPublicIPs(target=this, separator=;, prefix=http://, postfix=:3000))
```

Without CB-Tumblebug:

```bash
wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/deployOpenWebUI.sh; chmod +x ~/deployOpenWebUI.sh; sudo ~/deployOpenWebUI.sh http://<Ollama_Server_Endpoint1>:3000;http://<Ollama_Server_Endpoint2>:3000
```

Replace `<Ollama_Server_Endpoint1>` and `<Ollama_Server_Endpoint2>` with the actual server endpoints.

#### Description

The `deployOpenWebUI.sh` script installs and configures the OpenWebUI, providing a user-friendly interface for managing the deployed LLMs. This interface facilitates easy interaction with the models, making it simpler to perform tasks such as querying the models and monitoring their performance.

## How to Use

1. **Install the CUDA Driver**

   Run the `installCudaDriver.sh` script to install the necessary CUDA drivers. Ensure you are running this on an Ubuntu 22.04 system.

   ```bash
   curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/installCudaDriver.sh | sh
   ```

2. **Deploy Ollama LLM**

   After installing the CUDA drivers, deploy the Ollama LLM by running the `deployOllama.sh` script.

   ```bash
   curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/deployOllama.sh | sh
   ```

3. **Set Up OpenWebUI**

   Set up the OpenWebUI for easier management of your LLMs. Use the appropriate command based on whether you are using CB-Tumblebug remote commands or not.

   With CB-Tumblebug:

   ```bash
   wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/deployOpenWebUI.sh; chmod +x ~/deployOpenWebUI.sh; sudo ~/deployOpenWebUI.sh $$Func(GetPublicIPs(target=this, separator=;, prefix=http://, postfix=:3000))
   ```

   Without CB-Tumblebug:

   ```bash
   wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/deployOpenWebUI.sh; chmod +x ~/deployOpenWebUI.sh; sudo ~/deployOpenWebUI.sh http://<Ollama_Server_Endpoint1>:3000;http://<Ollama_Server_Endpoint2>:3000
   ```

---


## Alternative

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
