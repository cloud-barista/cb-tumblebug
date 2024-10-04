# CB-Tumblebug (Multi-Cloud Infra Management) 👋

[![Go Report Card](https://goreportcard.com/badge/github.com/cloud-barista/cb-tumblebug)](https://goreportcard.com/report/github.com/cloud-barista/cb-tumblebug)
[![Top Language](https://img.shields.io/github/languages/top/cloud-barista/cb-tumblebug)](https://github.com/cloud-barista/cb-tumblebug/search?l=go)
[![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/cloud-barista/cb-tumblebug?label=go.mod)](https://github.com/cloud-barista/cb-tumblebug/blob/main/go.mod)
[![Repo Size](https://img.shields.io/github/repo-size/cloud-barista/cb-tumblebug)](#)
[![GoDoc](https://godoc.org/github.com/cloud-barista/cb-tumblebug?status.svg)](https://pkg.go.dev/github.com/cloud-barista/cb-tumblebug@main)
[![Codebase](https://img.shields.io/badge/Visual-Codebase-blue)](https://mango-dune-07a8b7110.1.azurestaticapps.net/?repo=cloud-barista%2Fcb-tumblebug)
[![Swagger API Doc](https://img.shields.io/badge/API%20Doc-Swagger-brightgreen)](https://cloud-barista.github.io/api/?url=https://converter.swagger.io/api/convert?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.json#/)

[![License](https://img.shields.io/github/license/cloud-barista/cb-tumblebug?color=blue)](https://github.com/cloud-barista/cb-tumblebug/blob/main/LICENSE)
[![Release Version](https://img.shields.io/github/v/release/cloud-barista/cb-tumblebug?color=blue)](https://github.com/cloud-barista/cb-tumblebug/releases/latest)
[![Pre Release Version](https://img.shields.io/github/v/release/cloud-barista/cb-tumblebug?color=brightgreen&include_prereleases&label=release%28dev%29)](https://github.com/cloud-barista/cb-tumblebug/releases)
[![Build](https://img.shields.io/github/actions/workflow/status/cloud-barista/cb-tumblebug/continuous-integration.yaml)](https://github.com/cloud-barista/cb-tumblebug/actions/workflows/continuous-integration.yaml?query=workflow%3AContinuous-Integration-%28CI%29)
[![Slack](https://img.shields.io/badge/Slack-SIG--TB-brightgreen)](https://cloud-barista.slack.com/archives/CJQ7575PU)

<!-- ALL-CONTRIBUTORS-BADGE:START - Do not remove or modify this section -->
[![All Contributors](https://img.shields.io/badge/all_contributors-46-orange.svg?style=flat-square)](#contributors-)
<!-- ALL-CONTRIBUTORS-BADGE:END -->

## CB-TB? ✨

CB-Tumblebug (CB-TB for short) is a system for managing multi-cloud infrastructure consisting of resources from multiple cloud service providers. (Cloud-Barista)

- [Overview](https://github.com/cloud-barista/cb-tumblebug/wiki/CB‐Tumblebug-Overview), [Features](https://github.com/cloud-barista/cb-tumblebug/wiki/CB‐Tumblebug-Features), [Architecture](https://github.com/cloud-barista/cb-tumblebug/wiki/CB‐Tumblebug-Architecture)
- [Supported Cloud Providers and Resource Types](https://docs.google.com/spreadsheets/d/1idBoaTxEMzuVACKUIMIE9OY1rPO-7yZ0y7Rs1dBG0og/edit?usp=sharing)
  ![image](https://github.com/user-attachments/assets/7c3a5961-ffbe-4ed1-bc50-f3f445417a96)
  - This is for reference only, and we do not guarantee its functionality. Regular updates are made.
  - The support for Kubernetes is currently mostly a WIP, and even when available, it offers only a limited set of features.

---

<details>
<summary>Note: Ongoing Development of CB-Tumblebug </summary>

```
CB-TB has not reached version 1.0 yet. We welcome any new suggestions, issues, opinions, and contributors!
Please note that the functionalities of Cloud-Barista are not yet stable or secure.
Be cautious if you plan to use the current release in a production environment.
If you encounter any difficulties using Cloud-Barista,
please let us know by opening an issue or joining the Cloud-Barista Slack.
```

</details>

<details>
<summary>Note: Localization and Globalization of CB-Tumblebug </summary>
    
```
As an open-source project initiated by Korean members,
we aim to encourage participation from Korean contributors during the initial stages of this project.
Therefore, the CB-TB repository will accept the use of the Korean language in its early stages.
However, we hope this project will thrive regardless of contributors' countries in the long run.
To facilitate this, the maintainers recommend using English at least for
the titles of Issues, Pull Requests, and Commits, while accommodating local languages in the contents.
```

</details>

### Popular Use Case 🌟

 - **Deploy a Multi-Cloud Infra with GPUs and Enjoy muiltple LLMs in parallel (YouTube)**
   [![Multi-Cloud LLMs in parallel](https://github.com/cloud-barista/cb-tumblebug/assets/5966944/e15feb67-ba02-4066-af62-d9f8e8330a63)](https://www.youtube.com/watch?v=SD9ZoT_OZpQ)
 - [LLM-related scripts](https://github.com/cloud-barista/cb-tumblebug/tree/main/scripts/usecases/llm)


---

## Index 🔗

1. [Prerequisites](#prerequisites-)
1. [How to Run](#how-to-run-)
1. [How to Use](#how-to-use-cb-tb-features-)
1. [How to Build](#how-to-build-%EF%B8%8F)
1. [How to Contribute](#how-to-contribute-)

---

## Prerequisites 🌍

### Envionment

- Linux (recommend: `Ubuntu 22.04`)
- Docker and Docker Compose 
- Golang (recommend: `v1.23.0`) to build the source

---

### Dependency

Open source packages used in this project

- [Dependencies](https://github.com/cloud-barista/cb-tumblebug/network/dependencies)
- [SBOM](https://github.com/cloud-barista/cb-tumblebug/dependency-graph/sbom)


---

## How to Run 🚀

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
  cd ~/go/src/github.com/cloud-barista/cb-tumblebug
  ./scripts/installDocker.sh
  ```
    
- Start All Components Using Docker Compose:

  To run all components, use the following command:
  
  ```bash
  cd ~/go/src/github.com/cloud-barista/cb-tumblebug
  sudo docker compose up
  ```
  
  This command will start all components as defined in the preconfigured [docker-compose.yaml](https://github.com/cloud-barista/cb-tumblebug/blob/main/docker-compose.yaml) file. For configuration customization, please refer to the [guide](https://github.com/cloud-barista/cb-tumblebug/discussions/1669).

  The following components will be started:
  - ETCD: CB-Tumblebug KeyValue DB
  - CB-Spider: a Cloud API controller
  - CB-MapUI: a simple Map-based GUI web server
  - CB-Tumblebug: the system with API server

  ![image](https://github.com/user-attachments/assets/4466b6ff-6566-4ee0-ae60-d57e3d152821)
  
  After running the command, you should see output similar to the following:
  ![image](https://github.com/user-attachments/assets/1861edfd-411f-4c43-ab62-fa3658b8a1e9)

  Now, the CB-Tumblebug API server is accessible at: http://localhost:1323/tumblebug/api
  Additionally, CB-MapUI is accessible at: http://localhost:1324

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
    ![image](https://github.com/user-attachments/assets/5e7a73a6-8746-4be3-9a74-50b051f345bb)

  If you need to update your credentials, decrypt the encrypted file using `decCredential.sh`, make the necessary changes to `credentials.yaml`, and then re-encrypt it.

  - Decrypting Credentials
    ```bash
    init/decCredential.sh
    ```
    ![image](https://github.com/user-attachments/assets/85c91124-317d-4877-a025-a53cfdf2725e)

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

1. [Using CB-TB MapUI](#using-cb-tb-mapui) (recommended)
2. [Using CB-TB REST API](#using-cb-tb-rest-api) (recommended)
3. [Using CB-TB Test Scripts](#using-cb-tb-scripts)

### Using CB-TB MapUI

- With CB-MapUI, you can create, view, and control Mutli-Cloud infra.
  - [CB-MapUI](https://github.com/cloud-barista/cb-mapui) is a project to visualize the deployment of MCI in a map GUI.
  - CB-MapUI also run with CB-Tumblebug by default (edit `dockercompose.yaml` to disable)
    - If you run the CB-MapUI container using the CB-TB script, excute
      ```bash
      cd ~/go/src/github.com/cloud-barista/cb-tumblebug
      ./scripts/runMapUI.sh
      ```
  - Access via web browser at http://{HostIP}:1324
    ![image](https://github.com/cloud-barista/cb-mapui/assets/5966944/2423fbcd-0fdb-4511-85e2-488ba15ae8c0)

---

### Using CB-TB REST API

- Access to REST API dashboard

  - http://[IP]:1323/tumblebug/api
    - Upsteam online API document: [![Swagger API Doc](https://img.shields.io/badge/API%20Doc-Swagger-brightgreen)](https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml)
  - REST API AUTH

    CB-TB API is encoded with `basic access authentication` by default. (not fully secured yet!)

    You need to encode the `Username` and `Password` entered during server startup in Base64 and include it in the API header.

- [A guide to quickly create a Multi-Cloud Infra](https://github.com/cloud-barista/cb-tumblebug/discussions/1570)

- Using individual APIs
  - Create resources required for VM provisioning by using Resource(multi-cloud infrastructure resources) management APIs
    - [Create VM spec object](https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BInfra%20resource%5D%20Resource%20Spec%20management/post_ns__nsId__resources_spec)
    - [Create VM image object](https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BInfra%20resource%5D%20Resource%20Image%20management/post_ns__nsId__resources_image)
    - [Create network object](https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BInfra%20resource%5D%20Resource%20Network%20management/post_ns__nsId__resources_vNet)
    - [Create security group object](https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BInfra%20resource%5D%20Resource%20Security%20group%20management/post_ns__nsId__resources_securityGroup)
    - [Create access key object](https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BInfra%20resource%5D%20Resource%20Access%20key%20management/post_ns__nsId__resources_sshKey)
  - Create, view, control, execute remote commands, shut down, and delete MCI using the MCI(multi-cloud infrastructure service) management APIs
    - [Create MCI](https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BInfra%20service%5D%20MCI%20Provisioning%20management/post_ns__nsId__mci)
    - [MCI remote command](https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BInfra%20service%5D%20MCI%20Remote%20command/post_ns__nsId__cmd_mci__mciId_)
    - [View and control MCI](https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BInfra%20service%5D%20MCI%20Provisioning%20management/get_ns__nsId__mci__mciId_)
    - [Terminate and Delete MCI](https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BInfra%20service%5D%20MCI%20Provisioning%20management/delete_ns__nsId__mci__mciId_)
  - CB-TB optimal and dynamic provisioning
    - [CB-TB optimal and dynamic provisioning](https://github.com/cloud-barista/cb-tumblebug/wiki/Dynamic-and-optimal-mci-provisioning-guide)

---

### Using CB-TB Scripts

[`src/testclient/scripts/`](https://github.com/cloud-barista/cb-tumblebug/blob/main/src/testclient/scripts/) provides Bash shell-based scripts that simplify and automate the MCI (MC-Infra) provisioning procedures, which require complex steps.

<details>
<summary>[Note] Details </summary>
  
- Step 1: [Setup Test Environment](#setup-test-environment)
- Step 2: [Integrated Tests](#integrated-tests)


#### Setup Test Environment

1. Go to [`src/testclient/scripts/`](https://github.com/cloud-barista/cb-tumblebug/tree/main/src/testclient/scripts)
2. Configure [`conf.env`](https://github.com/cloud-barista/cb-tumblebug/blob/main/src/testclient/scripts/conf.env)
   - Provide basic test information such as CB-Spider and CB-TB server endpoints, cloud regions, test image names, test spec names, etc.
   - Much information for various cloud types has already been investigated and input, so it can be used without modification. (However, check for charges based on the specified spec)
     - How to modify test VM image: [`IMAGE_NAME[$IX,$IY]=ami-061eb2b23f9f8839c`](https://github.com/cloud-barista/cb-tumblebug/blob/553c4884943916b3287ec17501c6f639e8667897/src/testclient/scripts/conf.env#L49)
     - How to modify test VM spec: [`SPEC_NAME[$IX,$IY]=m4.4xlarge`](https://github.com/cloud-barista/cb-tumblebug/blob/553c4884943916b3287ec17501c6f639e8667897/src/testclient/scripts/conf.env#L50)
3. Configure [`testSet.env`](https://github.com/cloud-barista/cb-tumblebug/blob/main/src/testclient/scripts/testSet.env)
   - Set the cloud and region configurations to be used for MCI provisioning in a file (you can change the existing `testSet.env` or copy and use it)
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
  └── sequentialFullTest # Automatic testing from cloud information registration to NS creation, Resource creation, and MCI creation
      ├── check-test-config.sh # Check the multi-cloud infrastructure configuration specified in the current testSet
      ├── create-all.sh # Automatic testing from cloud information registration to NS creation, Resource creation, and MCI creation
      ├── gen-sshKey.sh # Generate SSH key files to access MCI
      ├── command-mci.sh # Execute remote commands on the created MCI (multiple VMs)
      ├── deploy-nginx-mci.sh # Automatically deploy Nginx on the created MCI (multiple VMs)
      ├── create-mci-for-df.sh # Create MCI for hosting CB-Dragonfly
      ├── deploy-dragonfly-docker.sh # Automatically deploy CB-Dragonfly on MCI and set up the environment
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

- MCI Creation Test

  - `./create-all.sh -n shson -f ../testSetCustom.env` # Create MCI with the cloud combination configured in ../testSetCustom.env
  - Automatically proceed with the process to check the MCI creation configuration specified in `../testSetCustom.env`
  - Example of execution result

    ```bash
    Table: All VMs in the MCI : cb-shson

    ID              Status   PublicIP       PrivateIP      CloudType  CloudRegion     CreatedTime
    --              ------   --------       ---------      ---------  -----------     -----------
    aws-ap-southeast-1-0   Running  xx.250.xx.73   192.168.2.180  aws        ap-southeast-1  2021-09-17   14:59:30
    aws-ca-central-1-0   Running  x.97.xx.230    192.168.4.98   aws        ca-central-1    2021-09-17   14:59:58
    gcp-asia-east1-0  Running  xx.229.xxx.26  192.168.3.2    gcp        asia-east1      2021-09-17   14:59:42

    [DATE: 17/09/2021 15:00:00] [ElapsedTime: 49s (0m:49s)] [Command: ./create-mci-only.sh all 1 shson ../testSetCustom.env 1]

    [Executed Command List]
    [Resource:aws-ap-southeast-1(28s)] create-resource-ns-cloud.sh (Resource) aws 1 shson ../testSetCustom.env
    [Resource:aws-ca-central-1(34s)] create-resource-ns-cloud.sh (Resource) aws 2 shson ../testSetCustom.env
    [Resource:gcp-asia-east1(93s)] create-resource-ns-cloud.sh (Resource) gcp 1 shson ../testSetCustom.env
    [MCI:cb-shsonvm4(19s+More)] create-mci-only.sh (MCI) all 1 shson ../testSetCustom.env

    [DATE: 17/09/2021 15:00:00] [ElapsedTime: 149s (2m:29s)] [Command: ./create-all.sh -n shson -f ../testSetCustom.env -x 1]
    ```

- MCI Removal Test (Use the input parameters used in creation for deletion)

  - `./clean-all.sh -n shson -f ../testSetCustom.env` # Perform removal of created resources according to `../testSetCustom.env`
  - **Be aware!**
    - If you created MCI (VMs) for testing in public clouds, the VMs may incur charges.
    - You need to terminate MCI by using `clean-all` to avoid unexpected billing.
    - Anyway, please be aware of cloud usage costs when using public CSPs.

- Generate MCI SSH access keys and access each VM

  - `./gen-sshKey.sh -n shson -f ../testSetCustom.env` # Return access keys for all VMs configured in MCI
  - Example of execution result

    ```bash
    ...
    [GENERATED PRIVATE KEY (PEM, PPK)]
    [MCI INFO: mc-shson]
     [VMIP]: 13.212.254.59   [MCIID]: mc-shson   [VMID]: aws-ap-southeast-1-0
     ./sshkey-tmp/aws-ap-southeast-1-shson.pem
     ./sshkey-tmp/aws-ap-southeast-1-shson.ppk
     ...

    [SSH COMMAND EXAMPLE]
     [VMIP]: 13.212.254.59   [MCIID]: mc-shson   [VMID]: aws-ap-southeast-1-0
     ssh -i ./sshkey-tmp/aws-ap-southeast-1-shson.pem cb-user@13.212.254.59 -o StrictHostKeyChecking=no
     ...
     [VMIP]: 35.182.30.37   [MCIID]: mc-shson   [VMID]: aws-ca-central-1-0
     ssh -i ./sshkey-tmp/aws-ca-central-1-shson.pem cb-user@35.182.30.37 -o StrictHostKeyChecking=no
    ```

- Verify MCI via SSH remote command execution

  - `./command-mci.sh -n shson -f ../testSetCustom.env` # Execute IP and hostname retrieval for all VMs in MCI

- K8s Cluster Test (WIP: Stability work in progress for each CSP)

  ```bash
  ./create-resource-ns-cloud.sh -n tb -f ../testSet.env` # Create Resource required for K8s cluster creation
  ./create-k8scluster-only.sh -n tb -f ../testSet.env -x 1 -z 1` # Create K8s cluster (-x maximum number of nodes, -z additional name for K8s node group and K8s cluster)
  ./get-k8scluster.sh -n tb -f ../testSet.env -z 1` # Get K8s cluster information
  ./add-k8snodegroup.sh -n tb -f ../testSet.env -x 1 -z 1` # Add a new K8s node group to the K8s cluster
  ./change-k8snodegroup-autoscalesize.sh -n tb -f ../testSet.env -x 1 -z 1` # Change the autoscale size of the specified K8s node group
  ./deploy-weavescope-to-k8scluster.sh -n tb -f ../testSet.env -y n` # Deploy weavescope to the created K8s cluster
  ./set-k8snodegroup-autoscaling.sh -n tb -f ../testSet.env -z 1` # Change the autoscaling setting of the new K8s node group to off
  ./remove-k8snodegroup.sh -n tb -f ../testSet.env -z 1` # Delete the newly created K8s node group
  ./clean-k8scluster-only.sh -n tb -f ../testSet.env -z 1` # Delete the created K8s cluster
  ./force-clean-k8scluster-only.sh -n tb -f ../testSet.env -z 1` # Force delete the created K8s cluster if deletion fails
  ./clean-resource-ns-cloud.h -n tb -f ../testSet.env` # Delete the created Resource
  ```


</details>

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
        wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz;
        sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz
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

  - API documentation file will be generated at `cb-tumblebug/src/api/rest/docs/swagger.yaml`
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
