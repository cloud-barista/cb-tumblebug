# Cloud-Barista 👋
# CB-Tumblebug (Multi-Cloud Infra Management)

[![Go Report Card](https://goreportcard.com/badge/github.com/cloud-barista/cb-tumblebug)](https://goreportcard.com/report/github.com/cloud-barista/cb-tumblebug)
[![Top Language](https://img.shields.io/github/languages/top/cloud-barista/cb-tumblebug)](https://github.com/cloud-barista/cb-tumblebug/search?l=go)
[![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/cloud-barista/cb-tumblebug?label=go.mod)](https://github.com/cloud-barista/cb-tumblebug/blob/main/go.mod)
[![Repo Size](https://img.shields.io/github/repo-size/cloud-barista/cb-tumblebug)](#)
[![GoDoc](https://godoc.org/github.com/cloud-barista/cb-tumblebug?status.svg)](https://pkg.go.dev/github.com/cloud-barista/cb-tumblebug@main)
[![Codebase](https://img.shields.io/badge/Visual-Codebase-blue)](https://mango-dune-07a8b7110.1.azurestaticapps.net/?repo=cloud-barista%2Fcb-tumblebug)
[![Swagger API Doc](https://img.shields.io/badge/API%20Doc-Swagger-brightgreen)](https://cloud-barista.github.io/api/?url=https://converter.swagger.io/api/convert?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/interface/rest/docs/swagger.json#/)

[![License](https://img.shields.io/github/license/cloud-barista/cb-tumblebug?color=blue)](https://github.com/cloud-barista/cb-tumblebug/blob/main/LICENSE)
[![Release Version](https://img.shields.io/github/v/release/cloud-barista/cb-tumblebug?color=blue)](https://github.com/cloud-barista/cb-tumblebug/releases/latest)
[![Pre Release Version](https://img.shields.io/github/v/release/cloud-barista/cb-tumblebug?color=brightgreen&include_prereleases&label=release%28dev%29)](https://github.com/cloud-barista/cb-tumblebug/releases)
[![Build](https://img.shields.io/github/actions/workflow/status/cloud-barista/cb-tumblebug/continuous-integration.yaml)](https://github.com/cloud-barista/cb-tumblebug/actions/workflows/continuous-integration.yaml?query=workflow%3AContinuous-Integration-%28CI%29)
[![Slack](https://img.shields.io/badge/Slack-SIG--TB-brightgreen)](https://cloud-barista.slack.com/archives/CJQ7575PU)

<!-- ALL-CONTRIBUTORS-BADGE:START - Do not remove or modify this section -->
[![All Contributors](https://img.shields.io/badge/all_contributors-50-orange.svg?style=flat-square)](#contributors-)
<!-- ALL-CONTRIBUTORS-BADGE:END -->

## What is CB-Tumblebug? ✨

**CB-Tumblebug (CB-TB)** is an advanced **multi-cloud infrastructure management system** that enables seamless provisioning, management, and orchestration of resources across multiple cloud service providers. Part of the Cloud-Barista project, CB-TB abstracts the complexity of multi-cloud environments into a unified, intuitive interface.

### 🎯 Key Capabilities
- **🌐 Multi-Cloud Orchestration**: Manage AWS, Azure, GCP, Alibaba Cloud, and more from a single platform
- **⚡ Auto-provisioning**: Intelligent resource recommendations and automated deployment
- **🔐 Secure Operations**: Encrypted credential management and hybrid encryption protocols
- **🗺️ Visual Infrastructure Map**: Interactive GUI for infrastructure visualization and management  
- **🤖 AI-Powered Management**: NEW! Control infrastructure using natural language via our MCP Server

### 📚 Documentation & Resources
- [📖 Overview](https://github.com/cloud-barista/cb-tumblebug/wiki/CB‐Tumblebug-Overview) | [✨ Features](https://github.com/cloud-barista/cb-tumblebug/wiki/CB‐Tumblebug-Features) | [🏗️ Architecture](https://github.com/cloud-barista/cb-tumblebug/wiki/CB‐Tumblebug-Architecture)
- [☁️ Supported Cloud Providers & Resources](https://docs.google.com/spreadsheets/d/1idBoaTxEMzuVACKUIMIE9OY1rPO-7yZ0y7Rs1dBG0og/edit?usp=sharing)

  ![Multi-Cloud Support Matrix](https://github.com/user-attachments/assets/35efa629-e864-4092-abb0-b455df4fd3c4)
  
  > 📌 **Note**: Reference only - functionality not guaranteed. Regular updates are made.  
  > Kubernetes support is currently WIP with limited features available.

---

<details>
<summary>📋 Development Status & Contributing Notes</summary>

### 🚧 Ongoing Development 
CB-TB has not reached version 1.0 yet. We welcome any new suggestions, issues, opinions, and contributors!
Please note that the functionalities of Cloud-Barista are not yet stable or secure.
Be cautious if you plan to use the current release in a production environment.
If you encounter any difficulties using Cloud-Barista,
please let us know by opening an issue or joining the Cloud-Barista Slack.

### 🌍 Localization & Globalization
As an open-source project initiated by Korean members,
we aim to encourage participation from Korean contributors during the initial stages of this project.
Therefore, the CB-TB repository will accept the use of the Korean language in its early stages.
However, we hope this project will thrive regardless of contributors' countries in the long run.
To facilitate this, the maintainers recommend using English at least for
the titles of Issues, Pull Requests, and Commits, while accommodating local languages in the contents.

</details>

### 🌟 Featured Use Cases

**🤖 NEW: AI-Powered Multi-Cloud Management**
- Control CB-Tumblebug through AI assistants like Claude and VS Code
- Natural language interface for infrastructure provisioning and management using MCP (Model Context Protocol)
- Streamable HTTP transport for modern MCP compatibility
- [📖 MCP Server Guide](src/interface/mcp/README.md) | [🚀 Quick Start](src/interface/mcp/README.md#-quick-start-with-docker-compose-recommended)

**🎮 GPU-Powered Multi-Cloud LLM Deployment**
[![Multi-Cloud LLMs in parallel](https://github.com/cloud-barista/cb-tumblebug/assets/5966944/e15feb67-ba02-4066-af62-d9f8e8330a63)](https://www.youtube.com/watch?v=SD9ZoT_OZpQ)
- Deploy GPU instances across multiple clouds for AI/ML workloads
- [🧠 LLM Scripts & Examples](https://github.com/cloud-barista/cb-tumblebug/tree/main/scripts/usecases/llm)


---

## Table of Contents

1. [⚡ Quick Start](#quick-start-)
2. [🔧 Prerequisites](#prerequisites-)
3. [🚀 Installation & Setup](#installation--setup-)
4. [🌟 How to Use](#how-to-use-cb-tb-features-)
5. [🛠️ Development](#development-%EF%B8%8F)
6. [🤝 Contributing](#how-to-contribute-)

---

## Quick Start ⚡

Get CB-Tumblebug running in under 5 minutes:

```bash
# 1. Automated setup (recommended for new users)
curl -sSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/set-tb.sh | bash

# 2. Start all services
cd ~/go/src/github.com/cloud-barista/cb-tumblebug
make compose

# 3. Configure credentials (see detailed setup below)
./init/genCredential.sh
# Edit ~/.cloud-barista/credentials.yaml with your cloud credentials
./init/encCredential.sh
./init/init.sh

# 4. Access services
# - API: http://localhost:1323/tumblebug/api
# - MapUI: http://localhost:1324
# - MCP Server: http://localhost:8000/mcp (if enabled)
```

> 💡 **New to CB-Tumblebug?** Follow the [detailed setup guide](#installation--setup-) below for comprehensive instructions.

---

## Prerequisites 🔧

### System Requirements

| Component | Minimum Specification | Recommended |
|-----------|----------------------|-------------|
| **OS** | Linux (Ubuntu 22.04+) | Ubuntu 22.04 LTS |
| **CPU** | 4 cores | 8+ cores |
| **Memory** | 6 GiB | 16+ GiB |
| **Storage** | 20 GiB free space | 50+ GiB SSD |
| **Example** | AWS `c5a.xlarge` | AWS `c5a.2xlarge` |

> ⚠️ **Performance Note**: Lower specifications may cause initialization failures or performance degradation.

### Required Software

- **Docker & Docker Compose** (latest stable)
- **Go 1.25.0+** (for building from source)
- **Git** (for cloning repository)

### Dependencies & Security

- 📦 [View Dependencies](https://github.com/cloud-barista/cb-tumblebug/network/dependencies)
- 🛡️ [Software Bill of Materials (SBOM)](https://github.com/cloud-barista/cb-tumblebug/dependency-graph/sbom)


---

## Installation & Setup 🚀

### Option 1: Automated Setup (Recommended)

For new users on clean Linux systems:

```bash
# Download and run automated setup script
curl -sSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/set-tb.sh | bash
```

> ℹ️ **Post-installation**: Log out and back in to activate Docker permissions and aliases.
- https://github.com/cloud-barista/cb-tumblebug/blob/main/scripts/set-tb.sh

```bash
curl -sSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/set-tb.sh | bash
```

> ℹ️ After the script finishes, you may need to **log out and back in** to activate Docker permissions and aliases.
> If you'd prefer to install dependencies and clone the repository manually, follow the steps below. 👇


### (1) Download CB-Tumblebug

- Clone the CB-Tumblebug repository:

  ```bash
  git clone https://github.com/cloud-barista/cb-tumblebug.git $HOME/go/src/github.com/cloud-barista/cb-tumblebug
  cd ~/go/src/github.com/cloud-barista/cb-tumblebug
  ```

  Optionally, you can register aliases for the CB-Tumblebug directory to simplify navigation:

  ```bash
  echo "alias cdtb='cd $HOME/go/src/github.com/cloud-barista/cb-tumblebug'" >> ~/.bashrc
  echo "alias cdtbsrc='cd $HOME/go/src/github.com/cloud-barista/cb-tumblebug/src'" >> ~/.bashrc
  echo "alias cdtbtest='cd $HOME/go/src/github.com/cloud-barista/cb-tumblebug/src/testclient/scripts'" >> ~/.bashrc
  source ~/.bashrc
  ```

---

### (2) Run CB-TB and All Related Components

- Check Docker Compose Installation:
  
  Ensure that Docker Engine and Docker Compose are installed on your system.
  If not, you can use the following script to install them (note: this script is not intended for production environments):

  ```bash
  # download and install docker with docker compose
  curl -sSL get.docker.com | sh
  
  # optional: add user to docker groupd
  sudo groupadd docker
  sudo usermod -aG docker ${USER}
  newgrp docker

  # test the docker works
  docker run hello-world
  ```
    
- Start All Components Using Docker Compose:

  To run all components, use the following command:
  
  ```bash
  cd ~/go/src/github.com/cloud-barista/cb-tumblebug
  docker compose up
  ```
  
  This command will start all components as defined in the preconfigured [docker-compose.yaml](https://github.com/cloud-barista/cb-tumblebug/blob/main/docker-compose.yaml) file. For configuration customization, please refer to the [guide](https://github.com/cloud-barista/cb-tumblebug/discussions/1669).

  The following components will be started:
  - ETCD: CB-Tumblebug KeyValue DB
  - CB-Spider: a Cloud API controller
  - CB-MapUI: a simple Map-based GUI web server
  - CB-Tumblebug: the system with API server
  - CB-Tumblebug MCP Server: AI assistant interface (if enabled)
  - PostgreSQL: Specs and Images storage
  - Traefik: Reverse proxy for secure access

  **Container Architecture Overview:**
  ```mermaid
  graph TB
      subgraph "External Access"
          User[👤 User]
          AI[🤖 AI Assistant<br/>Claude/VS Code]
      end
      
      subgraph "Docker Compose Environment"
          subgraph "Frontend & Interfaces"
              UI[CB-MapUI<br/>:1324]
              MCP[TB-MCP Server<br/>:8000]
              Proxy[Traefik Proxy<br/>:80/:443]
          end
          
          subgraph "Backend Services"
              TB[CB-Tumblebug<br/>:1323<br/>Multi-Cloud Management]
              Spider[CB-Spider<br/>:1024<br/>Cloud API Abstraction]
              ETCD[ETCD<br/>:2379<br/>Metadata Store]
              PG[PostgreSQL<br/>:5432<br/>Specs/Images DB]
          end
      end
      
      subgraph "Cloud Providers"
          AWS[AWS]
          Azure[Azure] 
          GCP[GCP]
          Others[Others...]
      end
      
      %% User connections
      User -->|HTTP/HTTPS| Proxy
      User -->|HTTP| UI
      User -->|HTTP| TB
      AI -->|MCP HTTP| MCP
      
      %% Proxy routing
      Proxy -->|Route| UI
      
      %% Internal service connections
      UI -.->|API calls| TB
      MCP -->|REST API| TB
      TB -->|REST API| Spider
      TB -->|gRPC| ETCD
      TB -->|SQL| PG
      
      %% Cloud connections
      Spider -->|Cloud APIs| AWS
      Spider -->|Cloud APIs| Azure
      Spider -->|Cloud APIs| GCP
      Spider -->|Cloud APIs| Others
      
      %% Styling
      classDef frontend fill:#e3f2fd,stroke:#1976d2
      classDef backend fill:#f3e5f5,stroke:#7b1fa2
      classDef storage fill:#e8f5e8,stroke:#388e3c
      classDef cloud fill:#fff3e0,stroke:#f57c00
      
      class UI,MCP,Proxy frontend
      class TB,Spider,ETCD,PG backend
      class AWS,Azure,GCP,Others cloud
  ```

  ![image](https://github.com/user-attachments/assets/4466b6ff-6566-4ee0-ae60-d57e3d152821)
  
  After running the command, you should see output similar to the following:
  ![image](https://github.com/user-attachments/assets/1861edfd-411f-4c43-ab62-fa3658b8a1e9)

  **Service Endpoints:**
  - **CB-Tumblebug API**: http://localhost:1323/tumblebug/api
  - **CB-MapUI**: http://localhost:1324 (direct) or https://cb-mapui.localhost (via Traefik with SSL)
  - **MCP Server**: http://localhost:8000/mcp (if enabled)
  - **Traefik Dashboard**: http://localhost:8080 (reverse proxy monitoring)

  **Note**: Before using CB-Tumblebug, you need to initialize it.

---
  
### (3) Initialize CB-Tumblebug to configure Multi-Cloud info

To provisioning multi-cloud infrastructures with CB-TB, it is necessary to register the connection information (credentials) for clouds, as well as commonly used images and specifications.

- Create `credentials.yaml` file and input your cloud credentials
  - Overview
    - `credentials.yaml` is a file that includes multiple credentials to use API of Clouds supported by CB-TB (AWS, GCP, AZURE, ALIBABA, etc.)
    - It should be located in the `~/.cloud-barista/` directory and securely managed.
    - Refer to the [`template.credentials.yaml`](https://github.com/cloud-barista/cb-tumblebug/blob/main/init/template.credentials.yaml) for the template
  - Create `credentials.yaml` the file

    Automatically generate the `credentials.yaml` file in the `~/.cloud-barista/` directory using the CB-TB script

    ```bash
    cd ~/go/src/github.com/cloud-barista/cb-tumblebug
    ./init/genCredential.sh
    ```

  - Input credential data

    Put credential data to `~/.cloud-barista/credentials.yaml` ([Reference: How to obtain a credential for each CSP](https://github.com/cloud-barista/cb-tumblebug/wiki/How-to-get-public-cloud-credentials))

    ```example
    ### Cloud credentials for credential holders (default: admin)
    credentialholder:
      admin:
        alibaba:
          # ClientId(ClientId): client ID of the EIAM application
          # Example: app_mkv7rgt4d7i4u7zqtzev2mxxxx
          ClientId:
          # ClientSecret(ClientSecret): client secret of the EIAM application
          # Example: CSEHDcHcrUKHw1CuxkJEHPveWRXBGqVqRsxxxx
          ClientSecret:
        aws:
          # ClientId(aws_access_key_id)
          # ex: AKIASSSSSSSSSSS56DJH
          ClientId:
          # ClientSecret(aws_secret_access_key)
          # ex: jrcy9y0Psejjfeosifj3/yxYcgadklwihjdljMIQ0
          ClientSecret:
        ...
    ```
- Encrypt `credentials.yaml` into `credentials.yaml.enc`

  To protect sensitive information, `credentials.yaml` is not used directly. Instead, it must be encrypted using `encCredential.sh`. The encrypted file `credentials.yaml.enc` is then used by `init.py`. This approach ensures that sensitive credentials are not stored in plain text.

  - Encrypting Credentials
    ```bash
    init/encCredential.sh
    ``` 
    When executing the script, you have two options: 1) enter your password or 2) let the system generate a random passkey.

    Option 1: Entering your password:

    ![Image](https://github.com/user-attachments/assets/8f051ce8-9282-4e6d-a8ae-af5c831622c7)
    
    Option 2: Letting the system generate a random passkey, which MUST be securely stored in a safe location:

    ![Image](https://github.com/user-attachments/assets/807511ee-05d9-481e-a191-d1aad2e9aeb2)

  If you need to update your credentials, decrypt the encrypted file using `decCredential.sh`, make the necessary changes to `credentials.yaml`, and then re-encrypt it.

  - Decrypting Credentials
    ```bash
    init/decCredential.sh
    ```
    Option 1: If encrypted using option 1, please use the same password to decrypt the file:

    ![Image](https://github.com/user-attachments/assets/600921fb-cdff-4313-ae4d-266ddd31809b)
    
    Option 2: If encrypted using option 2, enter the passkey to decrypt the file:

    ![Image](https://github.com/user-attachments/assets/2bb029a4-8dd9-4e1a-8cad-af70ca72e9fd)


- (INIT) Register all multi-cloud connection information and common resources

  - How to register

    Refer to [README.md for init.py](https://github.com/cloud-barista/cb-tumblebug/blob/main/init/README.md), and execute the [`init.py`](https://github.com/cloud-barista/cb-tumblebug/blob/main/init/init.py) script. (enter 'y' for confirmation prompts)

    ```bash
    cd ~/go/src/github.com/cloud-barista/cb-tumblebug
    ./init/init.sh
    ```

    - The credentials in `~/.cloud-barista/credentials.yaml.enc` (encrypted file from the `credentials.yaml`) will be automatically registered (all CSP and region information recorded in [`cloudinfo.yaml`](https://github.com/cloud-barista/cb-tumblebug/blob/main/assets/cloudinfo.yaml) will be automatically registered in the system)
      - Note: You can check the latest regions and zones of CSP using [`update-cloudinfo.py`](https://github.com/cloud-barista/cb-tumblebug/blob/main/scripts/misc/update-cloudinfo.py) and review the file for updates. (contributions to updates are welcome)
    - Common images and specifications recorded in the [`cloudimage.csv`](https://github.com/cloud-barista/cb-tumblebug/blob/main/assets/cloudimage.csv) and [`cloudspec.csv`](https://github.com/cloud-barista/cb-tumblebug/blob/main/assets/cloudspec.csv) files in the [`assets`](https://github.com/cloud-barista/cb-tumblebug/tree/main/assets) directory will be automatically registered.
   
    - **`init.py` will apply the hybrid encryption for secure transmission of credentials**
      1. Retrieve RSA Public Key: Use the `/credential/publicKey` API to get the public key.
      2. Encrypt Credentials: Encrypt credentials with a randomly generated `AES` key, then encrypt the `AES` key with the `RSA public key`.
      3. Transmit Encrypted Data: Send `the encrypted credentials` and `AES key` to the server. The server decrypts the AES key and uses it to decrypt the credentials.

      This method ensures your credentials are securely transmitted and protected during registration. See [init.py](https://github.com/cloud-barista/cb-tumblebug/blob/main/init/init.py#L150) for a Python implementation.
      In detail, check out [Secure Credential Registration Guide (How to use the credential APIs)](https://github.com/cloud-barista/cb-tumblebug/discussions/1773)

---

### (4) Shutting down and Version Upgrade

- Shutting down CB-TB and related components

  - Stop all containers by `ctrl` + `c` or type the command `sudo docker compose stop` / `sudo docker compose down`
    (When a shutdown event occurs to CB-TB, the system will be shutting down gracefully: API requests that can be processed within 10 seconds will be completed)
    
    ![image](https://github.com/user-attachments/assets/009e5df6-93cb-4ff7-93c0-62458341c78b)

  - In case of cleanup is needed due to internal system errors
    - Check and delete resources created through CB-TB
    - Delete CB-TB & CB-Spider metadata using the provided script
      ```bash
      cd ~/go/src/github.com/cloud-barista/cb-tumblebug
      ./init/cleanDB.sh
      ```

- Upgrading the CB-TB & CB-Spider versions

  The following cleanup steps are unnecessary if you clearly understand the impact of the upgrade

  - Check and delete resources created through CB-TB
  - Delete CB-TB & CB-Spider metadata
    ```bash
    cd ~/go/src/github.com/cloud-barista/cb-tumblebug
    ./init/cleanDB.sh
    ```
  - Restart with the upgraded version


---

## How to Use CB-TB Features 🌟

1. [🤖 Using CB-TB MCP Server (AI Assistant Interface)](#using-cb-tb-mcp-server) (NEW!)
2. [Using CB-TB MapUI](#using-cb-tb-mapui) (recommended)
3. [Using CB-TB REST API](#using-cb-tb-rest-api) (recommended)

### Using CB-TB MCP Server

**🚀 NEW: Control CB-Tumblebug with AI assistants like Claude!**

The Model Context Protocol (MCP) Server enables natural language interaction with CB-Tumblebug through AI assistants:

- **🧠 AI-Powered Infrastructure Management**: Deploy and manage multi-cloud resources using natural language commands
- **🔗 Seamless Integration**: Works with Claude Desktop (via proxy), VS Code (direct), and other MCP-compatible clients  
- **📡 Modern Protocol**: Uses Streamable HTTP transport (current MCP standard)
- **⚡ Quick Start**: Enable with `make compose` and uncomment MCP service in `docker-compose.yaml`

```bash
# Enable MCP Server (Proof of Concept)
# 1. Uncomment cb-tumblebug-mcp-server in docker-compose.yaml
# 2. Launch with Docker Compose
make compose

# Access MCP server at http://localhost:8000/mcp
```

**📖 [Complete MCP Server Guide →](src/interface/mcp/README.md)**

⚠️ **Note**: MCP Server is a Proof of Concept. Review code thoroughly before production use.

---

### Using CB-TB MapUI 🗺️

**Visual Infrastructure Management with Interactive Maps**

CB-MapUI provides an intuitive, map-based interface for managing multi-cloud infrastructure:

- **🗺️ Geographic Visualization**: See your infrastructure deployed across the globe
- **📊 Real-time Monitoring**: Monitor resource status and performance  
- **🎮 Interactive Control**: Create, manage, and control resources visually
- **🌐 Multi-Cloud View**: Unified view across all cloud providers

```bash
# Access CB-MapUI (auto-started with Docker Compose)
open http://localhost:1324

# Or run standalone MapUI container
./scripts/runMapUI.sh
```

![CB-MapUI Interface](https://github.com/cloud-barista/cb-mapui/assets/5966944/2423fbcd-0fdb-4511-85e2-488ba15ae8c0)

**Features:**
- Drag-and-drop resource creation
- Real-time infrastructure mapping
- Cross-cloud resource relationships
- Performance metrics overlay

> 📖 **Learn More**: [CB-MapUI Repository](https://github.com/cloud-barista/cb-mapui)

---

### Using CB-TB REST API 🔌

**Programmatic Multi-Cloud Infrastructure Management**

CB-Tumblebug provides a comprehensive REST API for automated infrastructure management:

**🌐 API Dashboard & Documentation**
- **Interactive API Explorer**: [http://localhost:1323/tumblebug/api](http://localhost:1323/tumblebug/api)
- **Live Documentation**: [![Swagger API Doc](https://img.shields.io/badge/API%20Doc-Swagger-brightgreen)](https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/interface/rest/docs/swagger.yaml)

**🔐 Authentication**
CB-TB uses Basic Authentication (development phase - not production-ready):
```bash
# Include base64 encoded credentials in request headers
Authorization: Basic <base64(username:password)>
```

**🚀 Quick Infrastructure Creation**
Following the [Quick MCI Creation Guide](https://github.com/cloud-barista/cb-tumblebug/discussions/1570):

```bash
# 1. Create VM specification
curl -X POST "http://localhost:1323/tumblebug/ns/default/resources/spec" \
  -H "Authorization: Basic <credentials>" \
  -d '{"name": "web-spec", "connectionName": "aws-ap-northeast-2"}'

# 2. Create VM image  
curl -X POST "http://localhost:1323/tumblebug/ns/default/resources/image" \
  -H "Authorization: Basic <credentials>" \
  -d '{"name": "ubuntu-image", "connectionName": "aws-ap-northeast-2"}'

# 3. Create Multi-Cloud Infrastructure
curl -X POST "http://localhost:1323/tumblebug/ns/default/mci" \
  -H "Authorization: Basic <credentials>" \
  -d @mci-config.json
```

**🛠️ Core API Categories**
- **Infrastructure Resources**: VM specs, images, networks, security groups
- **Multi-Cloud Infrastructure (MCI)**: Provision and manage distributed infrastructure
- **Monitoring & Control**: Performance metrics, scaling, lifecycle management
- **Credentials & Connections**: Secure cloud provider configuration
    - [Create access key object](https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/interface/rest/docs/swagger.yaml#/%5BInfra%20resource%5D%20Resource%20Access%20key%20management/post_ns__nsId__resources_sshKey)
  - Create, view, control, execute remote commands, shut down, and delete MCI using the MCI(multi-cloud infrastructure service) management APIs
    - [Create MCI](https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/interface/rest/docs/swagger.yaml#/%5BInfra%20service%5D%20MCI%20Provisioning%20management/post_ns__nsId__mci)
    - [MCI remote command](https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/interface/rest/docs/swagger.yaml#/%5BInfra%20service%5D%20MCI%20Remote%20command/post_ns__nsId__cmd_mci__mciId_)
    - [View and control MCI](https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/interface/rest/docs/swagger.yaml#/%5BInfra%20service%5D%20MCI%20Provisioning%20management/get_ns__nsId__mci__mciId_)
    - [Terminate and Delete MCI](https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/interface/rest/docs/swagger.yaml#/%5BInfra%20service%5D%20MCI%20Provisioning%20management/delete_ns__nsId__mci__mciId_)
  - CB-TB optimal and dynamic provisioning
    - [CB-TB optimal and dynamic provisioning](https://github.com/cloud-barista/cb-tumblebug/wiki/Dynamic-and-optimal-mci-provisioning-guide)

---

### Multi-Cloud Infrastructure Use Cases

#### Deploying an MCI Xonotic (3D FPS) Game Server

- [Deploy Xonotic game servers on MCI](https://github.com/cloud-barista/cb-tumblebug/wiki/Deploy-Xonotic-game-sever-in-a-Cloud-via-CB-Tumblebug)

#### Distributed Deployment of MCI Weave Scope Cluster Monitoring

- [Install Weave Scope cluster on MCI](https://github.com/cloud-barista/cb-tumblebug/wiki/MCI-WeaveScope-deployment)

#### Deploying MCI Jitsi Video Conferencing

- [Install Jitsi video conferencing on MCI](https://github.com/cloud-barista/cb-tumblebug/wiki/MCI-Jitsi-deployment)

#### Automatic Configuration of MCI Ansible Execution Environment

- [Automatically configure Ansible execution environment on MCI](https://github.com/cloud-barista/cb-tumblebug/wiki/MCI-Ansible-deployment)


---

## How to Build 🛠️

### (1) Setup Prerequisites

- Setup required tools

  - Install: git, gcc, make
    ```bash
    sudo apt update
    sudo apt install make gcc git
    ```
  - Install: Golang

    - Check https://golang.org/dl/ and setup Go

      - Download
        ```bash
        wget https://go.dev/dl/go1.25.0.linux-amd64.tar.gz;
        sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.25.0.linux-amd64.tar.gz
        ```
      - Setup environment

        ```bash
        echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin' >> ~/.bashrc
        echo 'export GOPATH=$HOME/go' >> ~/.bashrc
        ```

        ```bash
        source ~/.bashrc
        echo $GOPATH
        go env
        go version
        ```

---

### (2) Build and Run CB-Tumblebug

#### (2-1) Option 1: Run CB-Tumblebug with Docker Compose (Recommended)

- Run Docker Compose with the build option

  To build the current CB-Tumblebug source code into a container image and run it along with the other containers, use the following command:
  
  ```bash
  cd ~/go/src/github.com/cloud-barista/cb-tumblebug
  sudo DOCKER_BUILDKIT=1 docker compose up --build
  ```

  This command will automatically build the CB-Tumblebug from the local source code
  and start it within a Docker container, along with any other necessary services as defined in the `docker-compose.yml` file. `DOCKER_BUILDKIT=1` setting is used to speed up the build by using the go build cache technique.

#### (2-2) Option 2: Run CB-Tumblebug from the Makefile

- Build the Golang source code using the Makefile

  ```bash
  cd ~/go/src/github.com/cloud-barista/cb-tumblebug/src
  make
  ```

  All dependencies will be downloaded automatically by Go.

  The initial build will take some time, but subsequent builds will be faster by the Go build cache.

  **Note** To update the Swagger API documentation, run `make swag`

  - API documentation file will be generated at `cb-tumblebug/src/interface/rest/docs/swagger.yaml`
  - API documentation can be viewed in a web browser at http://localhost:1323/tumblebug/api (provided when CB-TB is running)
  - Detailed information on [how to update the API](https://github.com/cloud-barista/cb-tumblebug/wiki/API-Document-Update)

- Set environment variables required to run CB-TB (in another tab)

  - Check and configure the contents of `cb-tumblebug/conf/setup.env` (CB-TB environment variables, modify as needed)
    - Apply the environment variables to the system
      ```bash
      cd ~/go/src/github.com/cloud-barista/cb-tumblebug
      source conf/setup.env
      ```
    - (Optional) Automatically set the TB_SELF_ENDPOINT environment variable (an externally accessible address) using a script if needed
      - This is necessary if you want to access and control the Swagger API Dashboard from outside when CB-TB is running
      ```bash
      cd ~/go/src/github.com/cloud-barista/cb-tumblebug
      source ./scripts/setPublicIP.sh
      ```

- Execute the built cb-tumblebug binary by using `make run`
  ```bash
  cd ~/go/src/github.com/cloud-barista/cb-tumblebug/src
  make run
  ```


---

## How to Contribute 🙏

CB-TB welcomes improvements from both new and experienced contributors!

Check out [CONTRIBUTING](https://github.com/cloud-barista/cb-tumblebug/blob/main/CONTRIBUTING.md).


## Contributors ✨

Thanks goes to these wonderful people ([emoji key](https://allcontributors.org/docs/en/emoji-key)):

<!-- ALL-CONTRIBUTORS-LIST:START - Do not remove or modify this section -->
<!-- prettier-ignore-start -->
<!-- markdownlint-disable -->
<table>
  <tbody>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="https://seokho-son.github.io/"><img src="https://avatars3.githubusercontent.com/u/5966944?v=4?s=100" width="100px;" alt="Seokho Son"/><br /><sub><b>Seokho Son</b></sub></a><br /><a href="#maintenance-seokho-son" title="Maintenance">🚧</a> <a href="#ideas-seokho-son" title="Ideas, Planning, & Feedback">🤔</a> <a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=seokho-son" title="Code">💻</a> <a href="https://github.com/cloud-barista/cb-tumblebug/pulls?q=is%3Apr+reviewed-by%3Aseokho-son" title="Reviewed Pull Requests">👀</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://jihoon-seo.github.io"><img src="https://avatars1.githubusercontent.com/u/46767780?v=4?s=100" width="100px;" alt="Jihoon Seo"/><br /><sub><b>Jihoon Seo</b></sub></a><br /><a href="#maintenance-jihoon-seo" title="Maintenance">🚧</a> <a href="#ideas-jihoon-seo" title="Ideas, Planning, & Feedback">🤔</a> <a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=jihoon-seo" title="Code">💻</a> <a href="https://github.com/cloud-barista/cb-tumblebug/pulls?q=is%3Apr+reviewed-by%3Ajihoon-seo" title="Reviewed Pull Requests">👀</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/yunkon-kim"><img src="https://avatars2.githubusercontent.com/u/7975459?v=4?s=100" width="100px;" alt="Yunkon (Alvin) Kim "/><br /><sub><b>Yunkon (Alvin) Kim </b></sub></a><br /><a href="#ideas-yunkon-kim" title="Ideas, Planning, & Feedback">🤔</a> <a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=yunkon-kim" title="Code">💻</a> <a href="https://github.com/cloud-barista/cb-tumblebug/pulls?q=is%3Apr+reviewed-by%3Ayunkon-kim" title="Reviewed Pull Requests">👀</a> <a href="#maintenance-yunkon-kim" title="Maintenance">🚧</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/jmleefree"><img src="https://avatars3.githubusercontent.com/u/64775292?v=4?s=100" width="100px;" alt="jmleefree"/><br /><sub><b>jmleefree</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=jmleefree" title="Code">💻</a> <a href="https://github.com/cloud-barista/cb-tumblebug/pulls?q=is%3Apr+reviewed-by%3Ajmleefree" title="Reviewed Pull Requests">👀</a></td>
      <td align="center" valign="top" width="14.28%"><a href="http://www.powerkim.co.kr"><img src="https://avatars2.githubusercontent.com/u/46367962?v=4?s=100" width="100px;" alt="ByoungSeob Kim"/><br /><sub><b>ByoungSeob Kim</b></sub></a><br /><a href="#ideas-powerkimhub" title="Ideas, Planning, & Feedback">🤔</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/sykim-etri"><img src="https://avatars3.githubusercontent.com/u/25163268?v=4?s=100" width="100px;" alt="Sooyoung Kim"/><br /><sub><b>Sooyoung Kim</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/issues?q=author%3Asykim-etri" title="Bug reports">🐛</a> <a href="#ideas-sykim-etri" title="Ideas, Planning, & Feedback">🤔</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/dongjae"><img src="https://avatars.githubusercontent.com/u/5770239?v=4?s=100" width="100px;" alt="KANG DONG JAE"/><br /><sub><b>KANG DONG JAE</b></sub></a><br /><a href="#ideas-dongjae" title="Ideas, Planning, & Feedback">🤔</a></td>
    </tr>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="http://www.etri.re.kr"><img src="https://avatars.githubusercontent.com/u/5266479?v=4?s=100" width="100px;" alt="Youngwoo-Jung"/><br /><sub><b>Youngwoo-Jung</b></sub></a><br /><a href="#ideas-Youngwoo-Jung" title="Ideas, Planning, & Feedback">🤔</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/innodreamer"><img src="https://avatars.githubusercontent.com/u/51111668?v=4?s=100" width="100px;" alt="Sean Oh"/><br /><sub><b>Sean Oh</b></sub></a><br /><a href="#ideas-innodreamer" title="Ideas, Planning, & Feedback">🤔</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/MZC-CSC"><img src="https://avatars.githubusercontent.com/u/78469943?v=4?s=100" width="100px;" alt="MZC-CSC"/><br /><sub><b>MZC-CSC</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/issues?q=author%3AMZC-CSC" title="Bug reports">🐛</a> <a href="#ideas-MZC-CSC" title="Ideas, Planning, & Feedback">🤔</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/itnpeople"><img src="https://avatars.githubusercontent.com/u/35829386?v=4?s=100" width="100px;" alt="Eunsang"/><br /><sub><b>Eunsang</b></sub></a><br /><a href="#userTesting-itnpeople" title="User Testing">📓</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/hyokyungk"><img src="https://avatars.githubusercontent.com/u/51115778?v=4?s=100" width="100px;" alt="hyokyungk"/><br /><sub><b>hyokyungk</b></sub></a><br /><a href="#userTesting-hyokyungk" title="User Testing">📓</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/pjini"><img src="https://avatars.githubusercontent.com/u/64886639?v=4?s=100" width="100px;" alt="pjini"/><br /><sub><b>pjini</b></sub></a><br /><a href="#userTesting-pjini" title="User Testing">📓</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/vlatte"><img src="https://avatars.githubusercontent.com/u/21170063?v=4?s=100" width="100px;" alt="sunmi"/><br /><sub><b>sunmi</b></sub></a><br /><a href="#userTesting-vlatte" title="User Testing">📓</a></td>
    </tr>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/limsg1234"><img src="https://avatars.githubusercontent.com/u/53066410?v=4?s=100" width="100px;" alt="sglim"/><br /><sub><b>sglim</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=limsg1234" title="Documentation">📖</a> <a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=limsg1234" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/jangh-lee"><img src="https://avatars.githubusercontent.com/u/72970232?v=4?s=100" width="100px;" alt="jangh-lee"/><br /><sub><b>jangh-lee</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=jangh-lee" title="Documentation">📖</a> <a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=jangh-lee" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/leedohun"><img src="https://avatars.githubusercontent.com/u/33706689?v=4?s=100" width="100px;" alt="이도훈"/><br /><sub><b>이도훈</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=leedohun" title="Documentation">📖</a> <a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=leedohun" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://velog.io/@skynet"><img src="https://avatars.githubusercontent.com/u/26251856?v=4?s=100" width="100px;" alt="Park Beomsu"/><br /><sub><b>Park Beomsu</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=computerphilosopher" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/HassanAlsamahi"><img src="https://avatars.githubusercontent.com/u/42076287?v=4?s=100" width="100px;" alt="Hassan Alsamahi"/><br /><sub><b>Hassan Alsamahi</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=HassanAlsamahi" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/atg0831"><img src="https://avatars.githubusercontent.com/u/44899448?v=4?s=100" width="100px;" alt="Taegeon An"/><br /><sub><b>Taegeon An</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=atg0831" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="http://ihp001.tistory.com"><img src="https://avatars.githubusercontent.com/u/47745785?v=4?s=100" width="100px;" alt="INHYO"/><br /><sub><b>INHYO</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=PARKINHYO" title="Code">💻</a></td>
    </tr>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/Modney"><img src="https://avatars.githubusercontent.com/u/46340193?v=4?s=100" width="100px;" alt="Modney"/><br /><sub><b>Modney</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=Modney" title="Documentation">📖</a> <a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=Modney" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/ChobobDev"><img src="https://avatars.githubusercontent.com/u/32432141?v=4?s=100" width="100px;" alt="Seongbin Bernie Cho"/><br /><sub><b>Seongbin Bernie Cho</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=ChobobDev" title="Code">💻</a> <a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=ChobobDev" title="Documentation">📖</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/gbnam"><img src="https://avatars.githubusercontent.com/u/17192707?v=4?s=100" width="100px;" alt="Gibaek Nam"/><br /><sub><b>Gibaek Nam</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=gbnam" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/betelgeuse-7"><img src="https://avatars.githubusercontent.com/u/71967052?v=4?s=100" width="100px;" alt="Abidin Durdu"/><br /><sub><b>Abidin Durdu</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=betelgeuse-7" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://sysgongbu.tistory.com/"><img src="https://avatars.githubusercontent.com/u/46469385?v=4?s=100" width="100px;" alt="soyeon Park"/><br /><sub><b>soyeon Park</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=sypark9646" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/Jayita10"><img src="https://avatars.githubusercontent.com/u/85472715?v=4?s=100" width="100px;" alt="Jayita Pramanik"/><br /><sub><b>Jayita Pramanik</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=Jayita10" title="Documentation">📖</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/MukulKolpe"><img src="https://avatars.githubusercontent.com/u/78664749?v=4?s=100" width="100px;" alt="Mukul Kolpe"/><br /><sub><b>Mukul Kolpe</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=MukulKolpe" title="Documentation">📖</a></td>
    </tr>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/EmmanuelMarianMat"><img src="https://avatars.githubusercontent.com/u/75481347?v=4?s=100" width="100px;" alt="EmmanuelMarianMat"/><br /><sub><b>EmmanuelMarianMat</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=EmmanuelMarianMat" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="http://carlosfelix.pythonanywhere.com/"><img src="https://avatars.githubusercontent.com/u/18339454?v=4?s=100" width="100px;" alt="Carlos Felix"/><br /><sub><b>Carlos Felix</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=carlosfrodrigues" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/Stuie"><img src="https://avatars.githubusercontent.com/u/389169?v=4?s=100" width="100px;" alt="Stuart Gilbert"/><br /><sub><b>Stuart Gilbert</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=Stuie" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/ketan40"><img src="https://avatars.githubusercontent.com/u/15875215?v=4?s=100" width="100px;" alt="Ketan Deshmukh"/><br /><sub><b>Ketan Deshmukh</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=ketan40" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://ie.linkedin.com/in/trionabarrow"><img src="https://avatars.githubusercontent.com/u/2207006?v=4?s=100" width="100px;" alt="Tríona Barrow"/><br /><sub><b>Tríona Barrow</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=polkabunny" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://www.bambutz.dev"><img src="https://avatars.githubusercontent.com/u/7022144?v=4?s=100" width="100px;" alt="BamButz"/><br /><sub><b>BamButz</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=BamButz" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/dogfootman"><img src="https://avatars.githubusercontent.com/u/80231499?v=4?s=100" width="100px;" alt="dogfootman"/><br /><sub><b>dogfootman</b></sub></a><br /><a href="#userTesting-dogfootman" title="User Testing">📓</a></td>
    </tr>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/choryang"><img src="https://avatars.githubusercontent.com/u/47209678?v=4?s=100" width="100px;" alt="Okhee Lee"/><br /><sub><b>Okhee Lee</b></sub></a><br /><a href="#userTesting-choryang" title="User Testing">📓</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/joowons"><img src="https://avatars.githubusercontent.com/u/85204858?v=4?s=100" width="100px;" alt="joowon"/><br /><sub><b>joowon</b></sub></a><br /><a href="#userTesting-joowons" title="User Testing">📓</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/bconfiden2"><img src="https://avatars.githubusercontent.com/u/58922834?v=4?s=100" width="100px;" alt="Sanghong Kim"/><br /><sub><b>Sanghong Kim</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=bconfiden2" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/Rohit-R2000"><img src="https://avatars.githubusercontent.com/u/83547290?v=4?s=100" width="100px;" alt="Rohit Rajput"/><br /><sub><b>Rohit Rajput</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=Rohit-R2000" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/arshad-k7"><img src="https://avatars.githubusercontent.com/u/49522121?v=4?s=100" width="100px;" alt="Arshad"/><br /><sub><b>Arshad</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=arshad-k7" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="http://thearchivelog.dev"><img src="https://avatars.githubusercontent.com/u/44025432?v=4?s=100" width="100px;" alt="Jongwoo Han"/><br /><sub><b>Jongwoo Han</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=jongwooo" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="http://blog.naver.com/dev4unet"><img src="https://avatars.githubusercontent.com/u/8984372?v=4?s=100" width="100px;" alt="Yoo Jae-Sung"/><br /><sub><b>Yoo Jae-Sung</b></sub></a><br /><a href="#userTesting-dev4unet" title="User Testing">📓</a></td>
    </tr>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/raccoon-mh"><img src="https://avatars.githubusercontent.com/u/130422754?v=4?s=100" width="100px;" alt="Minhyeok LEE"/><br /><sub><b>Minhyeok LEE</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/pulls?q=is%3Apr+reviewed-by%3Araccoon-mh" title="Reviewed Pull Requests">👀</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/SungWoongz"><img src="https://avatars.githubusercontent.com/u/99646153?v=4?s=100" width="100px;" alt="SungWoongz"/><br /><sub><b>SungWoongz</b></sub></a><br /><a href="#userTesting-SungWoongz" title="User Testing">📓</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/gabrilito1"><img src="https://avatars.githubusercontent.com/u/105322029?v=4?s=100" width="100px;" alt="Gabriel lima"/><br /><sub><b>Gabriel lima</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=gabrilito1" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/tharun634"><img src="https://avatars.githubusercontent.com/u/53267275?v=4?s=100" width="100px;" alt="Tharun K"/><br /><sub><b>Tharun K</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=tharun634" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/ShehzadAhm"><img src="https://avatars.githubusercontent.com/u/55528726?v=4?s=100" width="100px;" alt="Shehzad"/><br /><sub><b>Shehzad</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=ShehzadAhm" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://wavee.world/invitation/b96d00e6-b802-4a1b-8a66-2e3854a01ffd"><img src="https://avatars.githubusercontent.com/u/22633385?v=4?s=100" width="100px;" alt="Ikko Eltociear Ashimine"/><br /><sub><b>Ikko Eltociear Ashimine</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=eltociear" title="Documentation">📖</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/MohitKambli"><img src="https://avatars.githubusercontent.com/u/31406633?v=4?s=100" width="100px;" alt="Mohit Kambli"/><br /><sub><b>Mohit Kambli</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=MohitKambli" title="Code">💻</a></td>
    </tr>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/hanizang77"><img src="https://avatars.githubusercontent.com/u/194071819?v=4?s=100" width="100px;" alt="Hannie Zang"/><br /><sub><b>Hannie Zang</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=hanizang77" title="Code">💻</a></td>
    </tr>
  </tbody>
</table>

<!-- markdownlint-restore -->
<!-- prettier-ignore-end -->

<!-- ALL-CONTRIBUTORS-LIST:END -->

<!-- ALL-CONTRIBUTORS-LIST:START - Do not remove or modify this section -->
<!-- prettier-ignore-start -->
<!-- markdownlint-disable -->

<!-- markdownlint-restore -->
<!-- prettier-ignore-end -->

<!-- ALL-CONTRIBUTORS-LIST:END -->

---

---

## License

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fcloud-barista%2Fcb-tumblebug.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2Fcloud-barista%2Fcb-tumblebug?ref=badge_large)

---
