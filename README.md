# CB-Tumblebug: Multi-Cloud Infra Management System (of Cloud-Barista)

[![Go Report Card](https://goreportcard.com/badge/github.com/cloud-barista/cb-tumblebug)](https://goreportcard.com/report/github.com/cloud-barista/cb-tumblebug)
[![Build](https://img.shields.io/github/actions/workflow/status/cloud-barista/cb-tumblebug/continuous-integration.yaml)](https://github.com/cloud-barista/cb-tumblebug/actions/workflows/continuous-integration.yaml?query=workflow%3AContinuous-Integration-%28CI%29)
[![Top Language](https://img.shields.io/github/languages/top/cloud-barista/cb-tumblebug)](https://github.com/cloud-barista/cb-tumblebug/search?l=go)
[![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/cloud-barista/cb-tumblebug?label=go.mod)](https://github.com/cloud-barista/cb-tumblebug/blob/main/go.mod)
[![Repo Size](https://img.shields.io/github/repo-size/cloud-barista/cb-tumblebug)](#)
[![GoDoc](https://godoc.org/github.com/cloud-barista/cb-tumblebug?status.svg)](https://pkg.go.dev/github.com/cloud-barista/cb-tumblebug@main)
[![Swagger API Doc](https://img.shields.io/badge/API%20Doc-Swagger-brightgreen)](https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml)

[![Release Version](https://img.shields.io/github/v/release/cloud-barista/cb-tumblebug?color=blue)](https://github.com/cloud-barista/cb-tumblebug/releases/latest)
[![Pre Release Version](https://img.shields.io/github/v/release/cloud-barista/cb-tumblebug?color=brightgreen&include_prereleases&label=release%28dev%29)](https://github.com/cloud-barista/cb-tumblebug/releases)
[![License](https://img.shields.io/github/license/cloud-barista/cb-tumblebug?color=blue)](https://github.com/cloud-barista/cb-tumblebug/blob/main/LICENSE)
[![Slack](https://img.shields.io/badge/Slack-SIG--TB-brightgreen)](https://cloud-barista.slack.com/archives/CJQ7575PU)

<!-- ALL-CONTRIBUTORS-BADGE:START - Do not remove or modify this section -->
[![All Contributors](https://img.shields.io/badge/all_contributors-43-orange.svg?style=flat-square)](#contributors-)
<!-- ALL-CONTRIBUTORS-BADGE:END -->

CB-Tumblebug (CB-TB for short) is a system for managing multi-cloud infrastructure consisting of resources from multiple cloud service providers. (Cloud-Barista)

- [CB-Tumblebug Overview](https://github.com/cloud-barista/cb-tumblebug/wiki/CB‐Tumblebug-Overview)
- [CB-Tumblebug Features](https://github.com/cloud-barista/cb-tumblebug/wiki/CB‐Tumblebug-Features)
- [CB-Tumblebug Architecture](https://github.com/cloud-barista/cb-tumblebug/wiki/CB‐Tumblebug-Architecture)
- [CB-Tumblebug Operation Sequence](https://github.com/cloud-barista/cb-tumblebug/tree/main/docs/designUML)
- **Hot usecase of CB-Tumblebug**
  - **Deploy a Multi-Cloud Infra with GPUs and Enjoy muiltple LLMs in parallel (YouTube)**
    [![Multi-Cloud LLMs in parallel](https://github.com/cloud-barista/cb-tumblebug/assets/5966944/e15feb67-ba02-4066-af62-d9f8e8330a63)](https://www.youtube.com/watch?v=SD9ZoT_OZpQ)
  - [LLM-related scripts](https://github.com/cloud-barista/cb-tumblebug/tree/main/scripts/usecases/llm)


<details>
<summary>[Note] Development of CB-Tumblebug is ongoing </summary>

```
CB-TB is not v1.0 yet.
We welcome any new suggestions, issues, opinions, and contributors!
Please note that the functionalities of Cloud-Barista are not stable and secure yet.
Be careful if you plan to use the current release in production.
If you have any difficulties in using Cloud-Barista, please let us know.
(Open an issue or join the Cloud-Barista Slack)
```

</details>

<details>
<summary>[Note] Localization and Globalization of CB-Tumblebug</summary>
    
```
As an open-source project initiated by Korean members, 
we would like to promote participation of Korean contributors during the initial stage of this project. 
So, the CB-TB repo will accept the use of the Korean language in its early stages.
However, we hope this project flourishes regardless of the contributor's country eventually.
So, the maintainers recommend using English at least for the titles of Issues, Pull Requests, and Commits, 
while the CB-TB repo accommodates local languages in their contents.
```

</details>

---

## Index

1. [Prerequisites](#prerequisites)
1. [How to Download](#how-to-download)
1. [How to Run](#3-run-cb-tb-system)
1. [How to Build](#how-to-build-and-run)
1. [How to Use](#how-to-use-cb-tb-features)
1. [How to Contribute](#how-to-contribute)


---

## Prerequisites

### Envionment
- Linux (recommended: `Ubuntu 22.04`)
- Golang (recommended: `v1.21.6`)

### Dependency
Open source packages used in this project

- [Dependencies](https://github.com/cloud-barista/cb-tumblebug/network/dependencies) 
- [SBOM](https://github.com/cloud-barista/cb-tumblebug/dependency-graph/sbom)


---

## How to Contribute

CB-TB welcomes improvements from both new and experienced contributors!

Check out [CONTRIBUTING](https://github.com/cloud-barista/cb-tumblebug/blob/main/CONTRIBUTING.md).

---

---

## How to Download

- Clone CB-TB repository

  ```bash
  git clone --depth 1 https://github.com/cloud-barista/cb-tumblebug.git $HOME/go/src/github.com/cloud-barista/cb-tumblebug
  cd ~/go/src/github.com/cloud-barista/cb-tumblebug
  ```

  The `--depth 1` option reduces the size by limiting the commit history download.

  For contributing, it is recommended not to specify this option or to restore the commit history using the following command.
  

  ```bash
  git fetch --unshallow
  ```

  Register alias for the CB-TB directory (optional action for convenience: `cdtb`, `cbtbsrc`, `cdtbtest`).
  ```bash
  echo "alias cdtb='cd $HOME/go/src/github.com/cloud-barista/cb-tumblebug'" >> ~/.bashrc
  echo "alias cdtbsrc='cd $HOME/go/src/github.com/cloud-barista/cb-tumblebug/src'" >> ~/.bashrc
  echo "alias cdtbtest='cd $HOME/go/src/github.com/cloud-barista/cb-tumblebug/src/testclient/scripts'" >> ~/.bashrc
  source ~/.bashrc
  ```

---

---

## How to Build and Run

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
        wget https://go.dev/dl/go1.21.6.linux-amd64.tar.gz;
        sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz
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

### (2) Build CB-TB

- Build the Golang source code using the Makefile
  ```bash
  cd ~/go/src/github.com/cloud-barista/cb-tumblebug/src
  make
  ```
  
  All dependencies will be downloaded automatically by Go.
  
  The initial build will take some time, but subsequent builds will be faster by the Go build cache.
  
  **Note** To update the Swagger API documentation, run `make swag` in `cb-tumblebug/src/`
    - API documentation file will be generated at `cb-tumblebug/src/api/rest/docs/swagger.yaml`
    - API documentation can be viewed in a web browser at http://localhost:1323/tumblebug/swagger/ (provided when CB-TB is running)
    - Detailed information on [how to update the API](https://github.com/cloud-barista/cb-tumblebug/wiki/API-Document-Update)


### (3) Run CB-TB system

#### (3-1) Run dependant sub-project
- Run CB-Spider
  
  CB-Tumblebug requires [CB-Spider](https://github.com/cloud-barista/cb-spider) to control multiple cloud service providers.

  - (Recommended method) Run the CB-Spider container using the CB-TB script (preferably use the specified version)

    ```bash
    cd ~/go/src/github.com/cloud-barista/cb-tumblebug
    ./scripts/runSpider.sh
    ```

    Docker must be installed. If it is not installed, you can use the following script (not for production setup)

    ```
    cd ~/go/src/github.com/cloud-barista/cb-tumblebug
    ./scripts/installDocker.sh
    ```

    For installation methods other than the container, refer to [CB-Spider](https://github.com/cloud-barista/cb-spider)

#### (3-2: option 1) Run CB-TB from the source code (recommended)
- [Clone the repository](#how-to-download)
- [Build and Setup](#how-to-build-and-setup)
- Set environment variables required to run CB-TB (in another tab)
  - Check and configure the contents of `cb-tumblebug/conf/setup.env` (CB-TB environment variables, modify as needed)
    - Apply the environment variables to the system
      ```bash
      cd ~/go/src/github.com/cloud-barista/cb-tumblebug
      source conf/setup.env
      ```
    - (Note) Automatically set the SELF_ENDPOINT environment variable (an externally accessible address) using a script if needed
      - This is necessary if you want to access and control the Swagger API Dashboard from outside when CB-TB is running
      ```bash
      cd ~/go/src/github.com/cloud-barista/cb-tumblebug
      source ./scripts/setPublicIP.sh
      ```
  - Check and configure the contents of `store_conf.yaml` in `cb-tumblebug/conf` (cb-store environment variables, modify as needed)
    - Specify storetype (NUTSDB or ETCD)
    - When setting NUTSDB (local DB), it is necessary to specify the path (by default, `cb-tumblebug/meta_db/dat`)
- Execute the built cb-tumblebug binary by using `make run`
  ```bash
  cd ~/go/src/github.com/cloud-barista/cb-tumblebug/src
  make run
  ```

#### (3-2: option 2) Run CB-TB from container images

- Check CB-TB available docker image tags(https://hub.docker.com/r/cloudbaristaorg/cb-tumblebug/tags)
- Run the container image (two options)
  - Run a script to excute CB-TB docker image (recommended)

    ```bash
    ./scripts/runTumblebug.sh
    ```

  - Run docker direclty
    ```bash
    docker run -p 1323:1323 \
    -v ${HOME}/go/src/github.com/cloud-barista/cb-tumblebug/meta_db:/app/meta_db \
    --name cb-tumblebug \
    cloudbaristaorg/cb-tumblebug:x.x.x
    ```

#### (3-3) Check the system is ready

  You will see the following messages..

  ```
    ██████╗██████╗    ████████╗██████╗
   ██╔════╝██╔══██╗   ╚══██╔══╝██╔══██╗
   ██║     ██████╔╝█████╗██║   ██████╔╝
   ██║     ██╔══██╗╚════╝██║   ██╔══██╗
   ╚██████╗██████╔╝      ██║   ██████╔╝
    ╚═════╝╚═════╝       ╚═╝   ╚═════╝

   ██████╗ ███████╗ █████╗ ██████╗ ██╗   ██╗
   ██╔══██╗██╔════╝██╔══██╗██╔══██╗╚██╗ ██╔╝
   ██████╔╝█████╗  ███████║██║  ██║ ╚████╔╝
   ██╔══██╗██╔══╝  ██╔══██║██║  ██║  ╚██╔╝
   ██║  ██║███████╗██║  ██║██████╔╝   ██║
   ╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝╚═════╝    ╚═╝

   Multi-cloud infrastructure managemenet framework
   ________________________________________________

   https://github.com/cloud-barista/cb-tumblebug


   Access to API dashboard (username: default / password: default)
   http://xxx.xxx.xxx.xxx:1323/tumblebug/api

  ⇨ http server started on [::]:1323
  ```

  - In default (`cb-tumblebug/conf/setup.env`),
    you can find the system log in `cb-tumblebug/log/tumblebug.log` (log is based on `zerolog`)


### (4) Configure Multi-Cloud info
To provisioning multi-cloud infrastructures with CB-TB, it is necessary to register the connection information (credentials) for clouds, as well as commonly used images and specifications.

- Create `credentials.yaml` file and input your cloud credentials
  - Overview
    - `credentials.yaml` is a file that includes multiple credentials to use API of Clouds supported by CB-TB (AWS, GCP, AZURE, ALIBABA, etc.)
    - It should be located in the `~/.cloud-barista/` directory and securely managed.
    - Refer to the [`template.credentials.yaml`](https://github.com/cloud-barista/cb-tumblebug/blob/main/scripts/init/template.credentials.yaml) for the template
  - Create `credentials.yaml` the file
    
    Automatically generate the `credentials.yaml` file in the `~/.cloud-barista/` directory using the CB-TB script
    
    ```bash
    cd ~/go/src/github.com/cloud-barista/cb-tumblebug
    ./scripts/init/genCredential.sh
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
- Register all multi-cloud connection information and common resources
  - How to register
  
    Refer to [README.md for init.py](https://github.com/cloud-barista/cb-tumblebug/blob/main/scripts/init/README.md), and execute the [`init.py`](https://github.com/cloud-barista/cb-tumblebug/blob/main/scripts/init/init.py) script. (enter 'y' for confirmation prompts)
    
    ```bash
    cd ~/go/src/github.com/cloud-barista/cb-tumblebug
    ./scripts/init/init.sh
    ```
    
    - The credentials in `~/.cloud-barista/credentials.yaml` will be automatically registered (all CSP and region information recorded in [`cloudinfo.yaml`](https://github.com/cloud-barista/cb-tumblebug/blob/main/assets/cloudinfo.yaml) will be automatically registered in the system)
      - Note: You can check the latest regions and zones of CSP using [`update-cloudinfo.py`](https://github.com/cloud-barista/cb-tumblebug/blob/main/scripts/misc/update-cloudinfo.py) and review the file for updates. (contributions to updates are welcome)
    - Common images and specifications recorded in the [`cloudimage.csv`](https://github.com/cloud-barista/cb-tumblebug/blob/main/assets/cloudimage.csv) and [`cloudspec.csv`](https://github.com/cloud-barista/cb-tumblebug/blob/main/assets/cloudspec.csv) files in the [`assets`](https://github.com/cloud-barista/cb-tumblebug/tree/main/assets) directory will be automatically registered.


### (5) Shutting down and Version Upgrade

- Shutting down the CB-TB & CB-Spider servers

  - CB-Spider: Shut down the server using `ctrl` + `c`
  - CB-TB: Shut down the server using `ctrl` + `c` (When a shutdown event occurs, the system will be shutting down gracefully: API requests that can be processed within 10 seconds will be completed)
  - In case of cleanup is needed due to internal system errors
    - Check and delete resources created through CB-TB
    - Delete CB-TB & CB-Spider metadata using the provided script
      ```bash
      cd ~/go/src/github.com/cloud-barista/cb-tumblebug
      ./scripts/cleanDB.sh
      ```

- Upgrading the CB-TB & CB-Spider versions
  
  The following cleanup steps are unnecessary if you clearly understand the impact of the upgrade
  
  - Check and delete resources created through CB-TB
  - Delete CB-TB & CB-Spider metadata
    ```bash
    cd ~/go/src/github.com/cloud-barista/cb-tumblebug
    ./scripts/cleanDB.sh
    ```
  - Restart with the upgraded version


---

---

## How to Use CB-TB Features

1. [Using CB-TB MapUI](#using-cb-tb-mapui) (recommended)
2. [Using CB-TB REST API](#using-cb-tb-rest-api) (recommended)
3. [Using CB-TB Test Scripts](#using-cb-tb-scripts)

### Using CB-TB MapUI

- With CB-MapUI, you can create, view, and control Mutli-Cloud infra.
  - [CB-MapUI](https://github.com/cloud-barista/cb-mapui) is a project to visualize the deployment of MCIS in a map GUI.
  - Run the CB-MapUI container using the CB-TB script
    ```bash
    cd ~/go/src/github.com/cloud-barista/cb-tumblebug
    ./scripts/runMapUI.sh
    ```
  - Access via web browser at http://{HostIP}:1324
    ![image](https://github.com/cloud-barista/cb-mapui/assets/5966944/2423fbcd-0fdb-4511-85e2-488ba15ae8c0)

### Using CB-TB REST API

- Access to REST API dashboard 
  - http://[IP]:1323/tumblebug/api
    - Upsteam online API document: [![Swagger API Doc](https://img.shields.io/badge/API%20Doc-Swagger-brightgreen)](https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml)
  - REST API AUTH
    
    CB-TB API is encoded with `basic access authentication` by default. (not fully secured yet!)
    
    You need to encode the `Username` and `Password` entered during server startup in Base64 and include it in the API header.

- [A guide to quickly create a Multi-Cloud Infra](https://github.com/cloud-barista/cb-tumblebug/discussions/1570)

- Using individual APIs
  - Create resources required for VM provisioning by using MCIR(multi-cloud infrastructure resources) management APIs
    - [Create VM spec object](https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BInfra%20resource%5D%20MCIR%20Spec%20management/post_ns__nsId__resources_spec)
    - [Create VM image object](https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BInfra%20resource%5D%20MCIR%20Image%20management/post_ns__nsId__resources_image)
    - [Create network object](https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BInfra%20resource%5D%20MCIR%20Network%20management/post_ns__nsId__resources_vNet)
    - [Create security group object](https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BInfra%20resource%5D%20MCIR%20Security%20group%20management/post_ns__nsId__resources_securityGroup)
    - [Create access key object](https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BInfra%20resource%5D%20MCIR%20Access%20key%20management/post_ns__nsId__resources_sshKey)
  - Create, view, control, execute remote commands, shut down, and delete MCIS using the MCIS(multi-cloud infrastructure service) management APIs
    - [Create MCIS](https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BInfra%20service%5D%20MCIS%20Provisioning%20management/post_ns__nsId__mcis)
    - [MCIS remote command](https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BInfra%20service%5D%20MCIS%20Remote%20command/post_ns__nsId__cmd_mcis__mcisId_)
    - [View and control MCIS](https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BInfra%20service%5D%20MCIS%20Provisioning%20management/get_ns__nsId__mcis__mcisId_)
    - [Terminate and Delete MCIS](https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BInfra%20service%5D%20MCIS%20Provisioning%20management/delete_ns__nsId__mcis__mcisId_)
  - CB-TB optimal and dynamic provisioning
    - [CB-TB optimal and dynamic provisioning](https://github.com/cloud-barista/cb-tumblebug/wiki/Dynamic-and-optimal-mcis-provisioning-guide)

### Using CB-TB Scripts

[`src/testclient/scripts/`](https://github.com/cloud-barista/cb-tumblebug/blob/main/src/testclient/scripts/) provides Bash shell-based scripts that simplify and automate the MCIS (MC-Infra) provisioning procedures, which require complex steps.

- Step 1: [Setup Test Environment](#setup-test-environment)
- Step 2: [Integrated Tests](#integrated-tests)
- Step 3: [Experience Use Cases](#multi-cloud-infrastructure-use-cases)

#### Setup Test Environment

1. Go to [`src/testclient/scripts/`](https://github.com/cloud-barista/cb-tumblebug/tree/main/src/testclient/scripts)
2. Configure [`conf.env`](https://github.com/cloud-barista/cb-tumblebug/blob/main/src/testclient/scripts/conf.env)
   - Provide basic test information such as CB-Spider and CB-TB server endpoints, cloud regions, test image names, test spec names, etc.
   - Much information for various cloud types has already been investigated and input, so it can be used without modification. (However, check for charges based on the specified spec)
     - How to modify test VM image: [`IMAGE_NAME[$IX,$IY]=ami-061eb2b23f9f8839c`](https://github.com/cloud-barista/cb-tumblebug/blob/553c4884943916b3287ec17501c6f639e8667897/src/testclient/scripts/conf.env#L49)
     - How to modify test VM spec: [`SPEC_NAME[$IX,$IY]=m4.4xlarge`](https://github.com/cloud-barista/cb-tumblebug/blob/553c4884943916b3287ec17501c6f639e8667897/src/testclient/scripts/conf.env#L50)
3. Configure [`testSet.env`](https://github.com/cloud-barista/cb-tumblebug/blob/main/src/testclient/scripts/testSet.env)
   - Set the cloud and region configurations to be used for MCIS provisioning in a file (you can change the existing `testSet.env` or copy and use it)
   - Specify the types of CSPs to combine
     - Change the number in [NumCSP=](https://github.com/cloud-barista/cb-tumblebug/blob/553c4884943916b3287ec17501c6f639e8667897/src/testclient/scripts/testSet.env#L9) to specify the total number of CSPs to combine
     - Specify the types of CSPs to combine by rearranging the lines in [L15-L24](https://github.com/cloud-barista/cb-tumblebug/blob/553c4884943916b3287ec17501c6f639e8667897/src/testclient/scripts/testSet.env#L15) (use up to the number specified in NumCSP)
     - Example: To combine aws and alibaba, change [NumCSP=2](https://github.com/cloud-barista/cb-tumblebug/blob/553c4884943916b3287ec17501c6f639e8667897/src/testclient/scripts/testSet.env#L9) and rearrange `IndexAWS=$((++IX))`, `IndexAlibaba=$((++IX))`
   - Specify the regions of the CSPs to combine
     - Go to each CSP setting item [`# AWS (Total: 21 Regions)`](https://github.com/cloud-barista/cb-tumblebug/blob/553c4884943916b3287ec17501c6f639e8667897/src/testclient/scripts/testSet.env#L44)
     - Specify the number of regions to configure in `NumRegion[$IndexAWS]=2` (in the example, it is set to 2)
     - Set the desired regions by rearranging the lines of the region list (if `NumRegion[$IndexAWS]=2`, the top 2 listed regions will be selected)
   - **Be aware!**
     - Be aware that creating VMs on public CSPs such as AWS, GCP, Azure, etc. may incur charges.
     - With the default setting of [`testSet.env`](https://github.com/cloud-barista/cb-tumblebug/blob/main/src/testclient/scripts/testSet.env), TestClouds (`TestCloud01`, `TestCloud02`, `TestCloud03`) will be used to create mock VMs.
     - `TestCloud01`, `TestCloud02`, `TestCloud03` are not real CSPs. They are used for testing purposes (do not support SSH into VM).
     - Anyway, please be aware of cloud usage costs when using public CSPs.

#### Integrated Tests

- You can test the entire process at once by executing `create-all.sh` and `clean-all.sh` included in `src/testclient/scripts/sequentialFullTest/`

  ```bash
  └── sequentialFullTest # Automatic testing from cloud information registration to NS creation, MCIR creation, and MCIS creation
      ├── check-test-config.sh # Check the multi-cloud infrastructure configuration specified in the current testSet
      ├── create-all.sh # Automatic testing from cloud information registration to NS creation, MCIR creation, and MCIS creation
      ├── gen-sshKey.sh # Generate SSH key files to access MCIS
      ├── command-mcis.sh # Execute remote commands on the created MCIS (multiple VMs)
      ├── deploy-nginx-mcis.sh # Automatically deploy Nginx on the created MCIS (multiple VMs)
      ├── create-mcis-for-df.sh # Create MCIS for hosting CB-Dragonfly
      ├── deploy-dragonfly-docker.sh # Automatically deploy CB-Dragonfly on MCIS and set up the environment
      ├── clean-all.sh # Delete all objects in reverse order of creation
      ├── create-k8scluster-only.sh # Create a K8s cluster for the multi-cloud infrastructure specified in the testSet
      ├── get-k8scluster.sh # Get K8s cluster information for the multi-cloud infrastructure specified in the testSet
      ├── clean-k8scluster-only.sh # Delete the K8s cluster for the multi-cloud infrastructure specified in the testSet
      ├── force-clean-k8scluster-only.sh # Force delete the K8s cluster for the multi-cloud infrastructure specified in the testSet if deletion fails
      ├── add-k8snodegroup.sh # Add a new K8s node group to the created K8s cluster
      ├── remove-k8snodegroup.sh # Delete the newly created K8s node group in the K8s cluster
      ├── set-k8snodegroup-autoscaling.sh # Change the autoscaling setting of the created K8s node group to off
      ├── change-k8snodegroup-autoscalesize.sh # Change the autoscale size of the created K8s node group
      ├── deploy-weavescope-to-k8scluster.sh # Deploy weavescope to the created K8s cluster
      └── executionStatus # Logs of the tests performed (information is added when testAll is executed and removed when cleanAll is executed. You can check the ongoing tasks)
  ```

- MCIS Creation Test
  - `./create-all.sh -n shson -f ../testSetCustom.env` # Create MCIS with the cloud combination configured in ../testSetCustom.env
  - Automatically proceed with the process to check the MCIS creation configuration specified in `../testSetCustom.env`
  - Example of execution result

    ```bash
    Table: All VMs in the MCIS : cb-shson

    ID              Status   PublicIP       PrivateIP      CloudType  CloudRegion     CreatedTime
    --              ------   --------       ---------      ---------  -----------     -----------
    aws-ap-southeast-1-0   Running  xx.250.xx.73   192.168.2.180  aws        ap-southeast-1  2021-09-17   14:59:30
    aws-ca-central-1-0   Running  x.97.xx.230    192.168.4.98   aws        ca-central-1    2021-09-17   14:59:58
    gcp-asia-east1-0  Running  xx.229.xxx.26  192.168.3.2    gcp        asia-east1      2021-09-17   14:59:42

    [DATE: 17/09/2021 15:00:00] [ElapsedTime: 49s (0m:49s)] [Command: ./create-mcis-only.sh all 1 shson ../testSetCustom.env 1]

    [Executed Command List]
    [MCIR:aws-ap-southeast-1(28s)] create-mcir-ns-cloud.sh (MCIR) aws 1 shson ../testSetCustom.env
    [MCIR:aws-ca-central-1(34s)] create-mcir-ns-cloud.sh (MCIR) aws 2 shson ../testSetCustom.env
    [MCIR:gcp-asia-east1(93s)] create-mcir-ns-cloud.sh (MCIR) gcp 1 shson ../testSetCustom.env
    [MCIS:cb-shsonvm4(19s+More)] create-mcis-only.sh (MCIS) all 1 shson ../testSetCustom.env

    [DATE: 17/09/2021 15:00:00] [ElapsedTime: 149s (2m:29s)] [Command: ./create-all.sh -n shson -f ../testSetCustom.env -x 1]
    ```

- MCIS Removal Test (Use the input parameters used in creation for deletion)
  - `./clean-all.sh -n shson -f ../testSetCustom.env` # Perform removal of created resources according to `../testSetCustom.env`
  - **Be aware!**
    - If you created MCIS (VMs) for testing in public clouds, the VMs may incur charges.
    - You need to terminate MCIS by using `clean-all` to avoid unexpected billing.
    - Anyway, please be aware of cloud usage costs when using public CSPs.

- Generate MCIS SSH access keys and access each VM
  - `./gen-sshKey.sh -n shson -f ../testSetCustom.env` # Return access keys for all VMs configured in MCIS
  - Example of execution result
    ```bash
    ...
    [GENERATED PRIVATE KEY (PEM, PPK)]
    [MCIS INFO: mc-shson]
     [VMIP]: 13.212.254.59   [MCISID]: mc-shson   [VMID]: aws-ap-southeast-1-0
     ./sshkey-tmp/aws-ap-southeast-1-shson.pem
     ./sshkey-tmp/aws-ap-southeast-1-shson.ppk
     ...

    [SSH COMMAND EXAMPLE]
     [VMIP]: 13.212.254.59   [MCISID]: mc-shson   [VMID]: aws-ap-southeast-1-0
     ssh -i ./sshkey-tmp/aws-ap-southeast-1-shson.pem cb-user@13.212.254.59 -o StrictHostKeyChecking=no
     ...
     [VMIP]: 35.182.30.37   [MCISID]: mc-shson   [VMID]: aws-ca-central-1-0
     ssh -i ./sshkey-tmp/aws-ca-central-1-shson.pem cb-user@35.182.30.37 -o StrictHostKeyChecking=no
    ```

- Verify MCIS via SSH remote command execution

  - `./command-mcis.sh -n shson -f ../testSetCustom.env` # Execute IP and hostname retrieval for all VMs in MCIS

- K8s Cluster Test (WIP: Stability work in progress for each CSP)  

  ```bash
  ./create-mcir-ns-cloud.sh -n tb -f ../testSet.env` # Create MCIR required for K8s cluster creation
  ./create-k8scluster-only.sh -n tb -f ../testSet.env -x 1 -z 1` # Create K8s cluster (-x maximum number of nodes, -z additional name for K8s node group and K8s cluster)
  ./get-k8scluster.sh -n tb -f ../testSet.env -z 1` # Get K8s cluster information
  ./add-k8snodegroup.sh -n tb -f ../testSet.env -x 1 -z 1` # Add a new K8s node group to the K8s cluster
  ./change-k8snodegroup-autoscalesize.sh -n tb -f ../testSet.env -x 1 -z 1` # Change the autoscale size of the specified K8s node group
  ./deploy-weavescope-to-k8scluster.sh -n tb -f ../testSet.env -y n` # Deploy weavescope to the created K8s cluster
  ./set-k8snodegroup-autoscaling.sh -n tb -f ../testSet.env -z 1` # Change the autoscaling setting of the new K8s node group to off
  ./remove-k8snodegroup.sh -n tb -f ../testSet.env -z 1` # Delete the newly created K8s node group
  ./clean-k8scluster-only.sh -n tb -f ../testSet.env -z 1` # Delete the created K8s cluster
  ./force-clean-k8scluster-only.sh -n tb -f ../testSet.env -z 1` # Force delete the created K8s cluster if deletion fails
  ./clean-mcir-ns-cloud.h -n tb -f ../testSet.env` # Delete the created MCIR
  ```

#### Multi-Cloud Infrastructure Use Cases

##### Deploying an MCIS Xonotic (3D FPS) Game Server

- [Deploy Xonotic game servers on MCIS](https://github.com/cloud-barista/cb-tumblebug/wiki/Deploy-Xonotic-game-sever-in-a-Cloud-via-CB-Tumblebug)

##### Distributed Deployment of MCIS Weave Scope Cluster Monitoring

- [Install Weave Scope cluster on MCIS](https://github.com/cloud-barista/cb-tumblebug/wiki/MCIS-WeaveScope-deployment)

##### Deploying MCIS Jitsi Video Conferencing

- [Install Jitsi video conferencing on MCIS](https://github.com/cloud-barista/cb-tumblebug/wiki/MCIS-Jitsi-deployment)

##### Automatic Configuration of MCIS Ansible Execution Environment

- [Automatically configure Ansible execution environment on MCIS](https://github.com/cloud-barista/cb-tumblebug/wiki/MCIS-Ansible-deployment)

---

---

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
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/yunkon-kim"><img src="https://avatars2.githubusercontent.com/u/7975459?v=4?s=100" width="100px;" alt="Yunkon Kim "/><br /><sub><b>Yunkon Kim </b></sub></a><br /><a href="#ideas-yunkon-kim" title="Ideas, Planning, & Feedback">🤔</a> <a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=yunkon-kim" title="Code">💻</a> <a href="https://github.com/cloud-barista/cb-tumblebug/pulls?q=is%3Apr+reviewed-by%3Ayunkon-kim" title="Reviewed Pull Requests">👀</a> <a href="#maintenance-yunkon-kim" title="Maintenance">🚧</a></td>
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
