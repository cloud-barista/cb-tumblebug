# CB-Tumblebug (Multi-Cloud Infra Service Management)

[![Go Report Card](https://goreportcard.com/badge/github.com/cloud-barista/cb-tumblebug)](https://goreportcard.com/report/github.com/cloud-barista/cb-tumblebug)
[![Build](https://img.shields.io/github/workflow/status/cloud-barista/cb-tumblebug/Build%20amd64%20container%20image)](https://github.com/cloud-barista/cb-tumblebug/actions?query=workflow%3A%22Build+amd64+container+image%22)
[![Top Language](https://img.shields.io/github/languages/top/cloud-barista/cb-tumblebug)](https://github.com/cloud-barista/cb-tumblebug/search?l=go)
[![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/cloud-barista/cb-tumblebug?label=go.mod)](https://github.com/cloud-barista/cb-tumblebug/blob/main/go.mod)
[![Repo Size](https://img.shields.io/github/repo-size/cloud-barista/cb-tumblebug)](#)
[![GoDoc](https://godoc.org/github.com/cloud-barista/cb-tumblebug?status.svg)](https://pkg.go.dev/github.com/cloud-barista/cb-tumblebug@main)
[![Swagger API Doc](https://img.shields.io/badge/API%20Doc-Swagger-brightgreen)](https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml)

[![Release Version](https://img.shields.io/github/v/release/cloud-barista/cb-tumblebug?color=blue)](https://github.com/cloud-barista/cb-tumblebug/releases/latest)
[![Pre Release Version](https://img.shields.io/github/v/release/cloud-barista/cb-tumblebug?color=brightgreen&include_prereleases&label=release%28dev%29)](https://github.com/cloud-barista/cb-tumblebug/releases)
[![License](https://img.shields.io/github/license/cloud-barista/cb-tumblebug?color=blue)](https://github.com/cloud-barista/cb-tumblebug/blob/main/LICENSE)
[![Slack](https://img.shields.io/badge/Slack-SIG--TB-brightgreen)](https://cloud-barista.slack.com/archives/CJQ7575PU)

<!-- ALL-CONTRIBUTORS-BADGE:START - Do not remove or modify this section -->
[![All Contributors](https://img.shields.io/badge/all_contributors-22-orange.svg?style=flat-square)](#contributors-)
<!-- ALL-CONTRIBUTORS-BADGE:END -->

A sub-system of Cloud-Barista Platform to Deploy and Manage Multi-Cloud Infrastructure.

<details>
<summary>Note for developing and using Cloud-Barista</summary>

### Development stage of Cloud-Barista
```
Cloud-Barista is currently under development. (not v1.0 yet)
We welcome any new suggestions, issues, opinions, and controbutors !
Please note that the functionalities of Cloud-Barista are not stable and secure yet.
Becareful if you plan to use the current release in production.
If you have any difficulties in using Cloud-Barista, please let us know.
(Open an issue or Join the Cloud-Barista Slack)
```

### Localization and Globalization of CB-Tumblebug (CB-Tumblebug의 현지화 및 세계화)
```
[English] As an opensource project initiated by Korean members, 
we would like to promote participation of Korean contributors during initial stage of this project. 
So, CB-Tumblebug Repo will accept use of Korean language in its early stages.
On the other hand, we hope this project flourishes regardless of contributor's country eventually.
So, the maintainers recommend using English at least for the title of Issues, Pull Requests, and Commits, 
while CB-Tumblebug Repo accommodates local languages in the contents of them.
```

```
[한국어] CB-Tumblebug은 한국에서 시작된 오픈 소스 프로젝트로서 
프로젝트의 초기 단계에는 한국 기여자들의 참여를 촉진하고자 합니다. 
따라서 초기 단계의 CB-Tumblebug는 한국어 사용을 받아 들일 것입니다.
다른 한편으로, 이 프로젝트가 국가에 관계없이 번성하기를 희망합니다.
따라서 개발 히스토리 관리를 위해 이슈, 풀 요청, 커밋 등의 
제목에 대해서는 영어 사용을 권장하며, 내용에 대한 한국어 사용은 수용할 것입니다.
```

</details>



***
***

## Table of Contents

1. [CB-Tumblebug Execution and Development Environment](#cb-tumblebug-execution-and-development-environment)
2. [How to contribute on CB-Tumblebug](#How-to-contribute-on-CB-Tumblebug)
3. [How To Run CB-Tumblebug ](#how-to-run-cb-tumblebug)
4. [CB-Tumblebug build and Execution based on Source Code in detail](#cb-tumblebug-build-and-execution-based-on-source-code-in-detail)
5. [How to use CB-Tumblebug functions](#how-to-use-cb-tumblebug-functions)

***
***

## CB-Tumblebug Execution and Development Environment
- Linux (Recommended: Ubuntu 18.04)
- Go (Recommended: v1.19)

***
***

## How to contribute on CB-Tumblebug

CB-Tumblebug welcomes improvements from both new and experienced contributors!

Check out [CONTRIBUTING](https://github.com/cloud-barista/cb-tumblebug/blob/main/CONTRIBUTING.md).

***
***

## How To Run CB-Tumblebug 

### (1) Source Code based Installation and Execution

- Table of Contents
  - Install tools and packages required
  - Clone CB-Tumblebug 
  - Set CB-Tumblebug Environmental Variable
  - Build CB-Tumblebug and Execute (`make` and `make run`)
- [CB-Tumblebug build and Execution based on Source Code in detail](#cb-tumblebug-build-and-execution-based-on-source-code-in-detail)
  
### (2) Container based Installation and Execution

- Check out CB-Tumblebug image from (https://hub.docker.com/r/cloudbaristaorg/cb-tumblebug/tags)
- Execute CB-Tumblebug Container

  ```
  # docker run -p 1323:1323 -p 50252:50252 \
  -v ${HOME}/go/src/github.com/cloud-barista/cb-tumblebug/meta_db:/app/meta_db \
  --name cb-tumblebug \
  cloudbaristaorg/cb-tumblebug:x.x.x
  ```

### (3) cb-operator based Cloud-Barista Combined Execution

- Through [cb-operator](https://github.com/cloud-barista/cb-operator), we can run Cloud-Barista's entire FW including CB-TB at once.

  ```
  $ git clone https://github.com/cloud-barista/cb-operator.git
  $ cd cb-operator/src
  cb-operator/src$ make
  cb-operator/src$ ./operator
  ```

***
***

## CB-Tumblebug build and Execution based on Source Code in detail

### (1) Configure CB-Tumblebug Build Environment

- Install tools and packages required
  - Install Git, gcc and make 
    - `# apt update`
    - `# apt install make gcc git`

  - Install Go 
    - Install Go by referencing https://golang.org/dl/ (Recommended: v1.19 or higher)
    - Installation Example
      - `wget https://go.dev/dl/go1.19.linux-amd64.tar.gz`
      - `sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.19.linux-amd64.tar.gz`
      - add followings on the bottom of `.bashrc`
      ```
      export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin
      export GOPATH=$HOME/go
      ```
      - `source ~/.bashrc` (apply the changes to `.bashrc`)

- Clone CB-Tumblebug 
  - `# git clone https://github.com/cloud-barista/cb-tumblebug.git $HOME/go/src/github.com/cloud-barista/cb-tumblebug`

- Setting Environmental Variable for executing CB-Tumblebug
  - check and configure `cb-tumblebug/conf/setup.env`  (CB-Tumblebug environment variable, changes may be required)
    - run `source setup.env` to apply on the system.
  - check and configure `store_conf.yaml` under `cb-tumblebug/conf` (cb-store environment variable, changes may be required)
    - Set store type (NUTSDB or ETCD)
    - Address with DB data should be set when configuring NUTSDB(local DB) i (file will be added under `cb-tumblebug/meta_db/dat` as default)
  - set and configure`log_conf.yaml` under `cb-tumblebug/conf`  (cb-log environment variable, changes may be required)


### (2) Build CB-Tumblebug 

- Build Command
  ```Shell
  # cd ~/go/src/github.com/cloud-barista/cb-tumblebug/src
  # export GO111MODULE=on
  # make
  ```

- If Swagger API Document needs to be updated run `make swag` at `cb-tumblebug/src/` directory.
  - API document file is created at  `cb-tumblebug/src/api/rest/docs/swagger.yaml` directory.
  - Following API document can be checked on http://localhost:1323/tumblebug/swagger/index.html through web browser. (Automatically provided when CB-Tumblebug is executed)

### (3) Run CB-Tumblebug 
- Run [CB-Spider](https://github.com/cloud-barista/cb-spider) in another tab
- `# cd ~/go/src/github.com/cloud-barista/cb-tumblebug/src`
- `# make run` (or `# go run *.go`) 

  CB-Tumblebug server execution screen
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

   Multi-cloud infrastructure management framework
   ________________________________________________

   https://github.com/cloud-barista/cb-tumblebug


   Access to API dashboard (username: default / password: default)
   http://xxx.xxx.xxx.xxx:1323/tumblebug/swagger/index.html

  ⇨ http server started on [::]:1323
  ⇨ grpc server started on [::]:50252
  ```

<details>
<summary>Known Errors and Troubleshooting</summary>

- Errors related to `golang.org/x/net/trace`
  ``` 
  panic: /debug/requests is already registered. 
  You may have two independent copies of golang.org/x/net/trace in your binary, 
  trying to maintain separate state. 
  This may involve a vendor copy of golang.org/x/net/trace.
  ```

  Solution: Run following to resolve this issue by removing duplicated files.
  ```Shell
  # rm -rf $GOPATH/src/go.etcd.io/etcd/vendor/golang.org/x/net/trace
  # make
  ```
</details>

***
***

## How to use CB-Tumblebug functions

1. [Using CB-Tumblebug Script](#using-cb-tumblebug-script)
2. [Use CB-Tumblebug REST API](#use-cb-tumblebug-rest-api)


### Using CB-Tumblebug Script
[`src/testclient/scripts/`](https://github.com/cloud-barista/cb-tumblebug/blob/main/src/testclient/scripts/) provides a Bashshell-based script that simplifies and automates Complicated MCIS (MC-Infra) provisioning procedures.
 - Step 1: [Enter cloud authentication information and test basic information](#enter-cloud-authentication-information and test basic information)
 - Step 2: Provisioning Namespace, MCIR, MCIS .etc (Choose between Integrated control test/Individual controls test)
   - [Individual Control Test](#Individual-Control-Test) (Objected Dependencies should be Considered when testing individuals of Namespace, MCIR, MCIS .etc)
   - [Integrated Control Test](#Integrated-Control-Test) (Recommended) `src/testclient/scripts/sequentialFullTest/`
 - Step 3: [Multi Cloud Infrastructure usecase](#multi-cloud-infrastructure-usecase)

#### Enter cloud credentials and test basic information
1. Go to [`src/testclient/scripts/`](https://github.com/cloud-barista/cb-tumblebug/tree/main/src/testclient/scripts)
2. Create `credentials.conf`
   - `credentials.conf` provides basic credentials template of CSPs (AWS, GCP, AZURE, ALIBABA .etc)
   - Reference [`credentials.conf.example`](https://github.com/cloud-barista/cb-tumblebug/blob/main/src/testclient/scripts/credentials.conf.example)to enter the user's cloud credentials.
   - [How to get CSP credentials](https://github.com/cloud-barista/cb-tumblebug/wiki/How-to-get-public-cloud-credentials)
3. Configure [`conf.env`](https://github.com/cloud-barista/cb-tumblebug/blob/main/src/testclient/scripts/conf.env)
   - Provides basic test information (server endpoint, cloud vision, image name for test, and spec name for test .etc) of CB-Spider and CB-Tumblebug
   - Since information on many cloud types has already been investigated and entered, it can be used without modification. (However, it is necessary to check since charging may occur depending on the designated spec.)
     - Way to change VM image for test: [`IMAGE_NAME[$IX,$IY]=ami-061eb2b23f9f8839c`](https://github.com/cloud-barista/cb-tumblebug/blob/553c4884943916b3287ec17501c6f639e8667897/src/testclient/scripts/conf.env#L49)
     - Way to change VM spec for test: [`SPEC_NAME[$IX,$IY]=m4.4xlarge`](https://github.com/cloud-barista/cb-tumblebug/blob/553c4884943916b3287ec17501c6f639e8667897/src/testclient/scripts/conf.env#L50)   
4. [`testSet.env`](https://github.com/cloud-barista/cb-tumblebug/blob/main/src/testclient/scripts/testSet.env) 설정
   - Set cloud and Region configurations to be used for MCIS provisioning to files ( copy and utilize 'testSet.env')
   - Set Combination of CSP 
     - Set numbers of CSP combination (change numbers at [NumCSP=](https://github.com/cloud-barista/cb-tumblebug/blob/553c4884943916b3287ec17501c6f639e8667897/src/testclient/scripts/testSet.env#L9))
     - Combination of CSP can be set by rearranging the orders at [L15-L24](https://github.com/cloud-barista/cb-tumblebug/blob/553c4884943916b3287ec17501c6f639e8667897/src/testclient/scripts/testSet.env#L15) (until the numbers configured on NumCSP)
     - ex): Combination of  aws, alibaba : Change [NumCSP=](https://github.com/cloud-barista/cb-tumblebug/blob/553c4884943916b3287ec17501c6f639e8667897/src/testclient/scripts/testSet.env#L9)) to [NumCSP=2](https://github.com/cloud-barista/cb-tumblebug/blob/553c4884943916b3287ec17501c6f639e8667897/src/testclient/scripts/testSet.env#L9) , and rearrange those `IndexAWS=$((++IX))`, `IndexAlibaba=$((++IX))` accordingly.
   - Set the region of combination of CSPs
     - Go to each CSP setting section [`# AWS (Total: 21 Regions)`](https://github.com/cloud-barista/cb-tumblebug/blob/553c4884943916b3287ec17501c6f639e8667897/src/testclient/scripts/testSet.env#L44) 
     - Set the numbers of region desired`NumRegion[$IndexAWS]=2`  ( 2 was set for demo)
     - Rearrange the orders of the region to selected region desired (The top 2 region will be selected in case `NumRegion[$IndexAWS]=2`)
   - **Be aware!** 
     - Be aware that creating VMs on public CSPs such as AWS, GCP, Azure, etc. may be billed.
     - With the default setting of [`testSet.env`](https://github.com/cloud-barista/cb-tumblebug/blob/main/src/testclient/scripts/testSet.env), TestClouds (`TestCloud01`, `TestCloud02`, `TestCloud03`) will be used to create mock VMs.
     - `TestCloud01`, `TestCloud02`, `TestCloud03` are not real CSPs. It is used for testing purpose. (not support SSH into VM)
     - Anyway, please be aware cloud usage cost when you use public CSPs.

#### Individual Control Test
- For resource that you want to control, go to following directory to run the test needed
  - Since the objects depend on each other, it is desirable to perform them in ascending order with reference to numbers.
    - `1.configureSpider`  # Collection of scripts related to cloud information registration
    - `2.configureTumblebug`  # Collection of scripts related to namespace and dynamic environment settings
    - `3.vNet`  # Collection of scripts related to creation of MCIR vNet
    - `4.securityGroup`  # Collection of scripts related to creation of MCIR securityGroup
    - `5.sshKey`  # Collection of scripts related to creation of MCIR sshKey
    - `6.image`  # Collection of scripts related to registration of MCIR image
    - `7.spec`  # Collection of scripts related to registration of MCIR spec
    - `8.mcis`  # Collection of scripts related of MCIS creation and control, MCIS remote command, etc.
    - `9.monitoring`  # Collection of scripts of CB-DF Monitoring Agent Installation and Monitoring Test Script through CB-TB

#### Integrated Control Test
- Executing `create-all.sh` and `clean-all.sh` under `src/testclient/scripts/sequentialFullTest/`You can test the entire process can be tested at once.
- By running `create-all.sh` and `clean-all.sh` in `src/testclient/scripts/sequentialFullTest/` directory, 
  ```
  └── sequentialFullTest  # Automatic testing of Cloud information registration, NS generation, MCIR generation, and MCIS generation at once.
      ├── check-test-config.sh  # Check the configuration of the multi-cloud infrastructure specified in the current testSet.
      ├── create-all.sh  # Automatic testing of Cloud information registration, NS generation, MCIR generation, and MCIS generation at once.
      ├── gen-sshKey.sh  # Test log that has been executed (generating an SSH key file accessible to MCIS) 
      ├── command-mcis.sh  # Execute a remote command to the generated MCIS (multiple VM)
      ├── deploy-nginx-mcis.sh  # Automatically distribute Nginx to the generated MCIS (multiple VM)
      ├── create-mcis-for-df.sh  # Create MCIS for CB-Dragonfly hosting        
      ├── deploy-dragonfly-docker.sh  # Automatic distribution of CB-Dragonfly to MCIS and automatic environment configuration.
      ├── clean-all.sh  # Delete all objects in reverse order of creation.
      └── executionStatus  # Test log (information is added when performing testAll, and information is removed when performing cleanAll). You can check the work in progress

  ```
- Usage Example
  - MCIS creation test
    - `./create-all.sh -n shson -f ../testSetCustom.env`   # Performs creation with the cloud combination configured in ../testSetCustom.env 
    - The procedure for confirming the generation of MCIS configured in ../testSetCustom.env proceeds automatically.
    - Execution result example
      ```
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
  - MCIS removal test (needs to be deleted with input parameters used in creation)
    - `./clean-all.sh -n shson -f ../testSetCustom.env`   # Performs removal with the cloud combination configured in ../testSetCustom.env 
    - **Be aware!** 
      - If you created MCIS (VMs) for testing in public clouds, the VMs may be charged.
      - You need to terminate MCIS by using `clean-all` to avoid unexpected billing.
      - Anyway, please be aware cloud usage cost when you use public CSPs.
  - Create MCIS SSH access key and connect to each VM.
    - `./gen-sshKey.sh -n shson -f ../testSetCustom.env`  # Return all VM's access keys configured in MCIS.
    - Execution Result Example
      ```
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

<details>
<summary>I/O Examples</summary>

```
~/go/src/github.com/cloud-barista/cb-tumblebug/src/testclient/scripts/sequentialFullTest$ `./create-all.sh -n shson -f ../testSetCustom.env`
####################################################################
## Create MCIS from Zero Base
####################################################################
[Test for AWS]
####################################################################
## 0. Create Cloud Connection Config
####################################################################
[Test for AWS]
{
   "ProviderName" : "AWS",
   "DriverLibFileName" : "aws-driver-v1.0.so",
   "DriverName" : "aws-driver01"
}
..........
   "RegionName" : "aws-us-east-1"
}
{
   "CredentialName" : "aws-credential01",
   "RegionName" : "aws-us-east-1",
   "DriverName" : "aws-driver01",
   "ConfigName" : "aws-us-east-1",
   "ProviderName" : "AWS"
}
####################################################################
## 0. Namespace: Create
####################################################################
{
   "message" : "The namespace NS-01 already exists."
}
####################################################################
## 1. vpc: Create
####################################################################
[Test for AWS]
{
   "subnetInfoList" : [
      {
         "IId" : {
            "SystemId" : "subnet-0ab25b7090afa97b7",
            "NameId" : "aws-us-east-1-shson"
         },
................
   "status" : "",
   "name" : "aws-us-east-1-shson",
   "keyValueList" : null,
   "connectionName" : "aws-us-east-1",
   "cspVNetId" : "vpc-0e3004f28e8a89057"
}
Dozing for 10 : 1 2 3 4 5 6 7 8 9 10 (Back to work)
####################################################################
## 2. SecurityGroup: Create
####################################################################
[Test for AWS]
{
   "keyValueList" : [
      {
         "Value" : "aws-us-east-1-shson-delimiter-aws-us-east-1-shson",
         "Key" : "GroupName"
      },
      {
         "Key" : "VpcID",
...........
   "name" : "aws-us-east-1-shson",
   "description" : "test description",
   "cspSecurityGroupId" : "sg-033e4b7c42671873c",
   "id" : "aws-us-east-1-shson"
}
Dozing for 10 : 1 2 3 4 5 6 7 8 9 10 (Back to work)
####################################################################
## 3. sshKey: Create
####################################################################
[Test for AWS]
{
   "name" : "aws-us-east-1-shson",
   "fingerprint" : "d2:1a:a0:6d:b3:f7:8e:b7:44:9f:13:9c:d6:e3:a8:c3:58:8c:de:27",
..............
   "id" : "aws-us-east-1-shson",
   "description" : "",
   "privateKey" : "-----BEGIN RSA PRIVATE KEY-----\nMIIEogIBAAKCAQ\ KEY-----",
   "username" : ""
}
####################################################################
## 4. image: Register
####################################################################
[Test for AWS]
{
   "keyValueList" : [
      {
         "Key" : "",
         "Value" : ""
      },
      {
         "Value" : "",
         "Key" : ""
      }
   ],
   "description" : "Canonical, Ubuntu, 18.04 LTS, amd64 bionic",
   "cspImageName" : "",
   "connectionName" : "aws-us-east-1",
   "status" : "",
   "creationDate" : "",
   "cspImageId" : "ami-085925f297f89fce1",
   "name" : "aws-us-east-1-shson",
   "guestOS" : "Ubuntu",
   "id" : "aws-us-east-1-shson"
}
####################################################################
## 5. spec: Register
####################################################################
[Test for AWS]
{
   "mem_MiB" : "1024",
   "max_num_storage" : "",
........
   "mem_GiB" : "1",
   "id" : "aws-us-east-1-shson",
   "num_core" : "",
   "cspSpecName" : "t2.micro",
   "storage_GiB" : "",
   "ebs_bw_Mbps" : "",
   "connectionName" : "aws-us-east-1",
   "net_bw_Gbps" : "",
   "gpu_model" : "",
   "cost_per_hour" : "",
   "name" : "aws-us-east-1-shson"
}
####################################################################
## 6. vm: Create MCIS
####################################################################
[Test for AWS]
{
   "targetAction" : "Create",
   "status" : "Running-(3/3)",
   "id" : "aws-us-east-1-shson",
   "name" : "aws-us-east-1-shson",
   "description" : "Tumblebug Demo",
   "targetStatus" : "Running",
   "placementAlgo" : "",
   "vm" : [
      {
         "vmUserId" : "",
         "targetStatus" : "None",
         "subnetId" : "aws-us-east-1-shson",
         "location" : {
            "nativeRegion" : "us-east-1",
            "cloudType" : "aws",
            "latitude" : "38.1300",
            "briefAddr" : "Virginia",
            "longitude" : "-78.4500"
         },
         "vm_accessId" : "",
         "region" : {
            "Region" : "us-east-1",
            "Zone" : "us-east-1f"
         },
         "imageId" : "aws-us-east-1-shson",
         "privateDNS" : "ip-192-168-1-108.ec2.internal",
         "status" : "Running",
         "security_groupIds" : [
            "aws-us-east-1-shson"
         ],
         "vm_access_passwd" : "",
 .........
            "VMUserId" : "",
            "SecurityGroupIIds" : [
               {
                  "SystemId" : "sg-033e4b7c42671873c",
                  "NameId" : "aws-us-east-1-shson"
               }
            ],
            "PrivateDNS" : "ip-192-168-1-108.ec2.internal",
            "StartTime" : "2020-05-30T18:33:42Z",
            "VMBlockDisk" : "/dev/sda1",
            "ImageIId" : {
               "SystemId" : "ami-085925f297f89fce1",
               "NameId" : "ami-085925f297f89fce1"
            }
         },
         "publicIP" : "35.173.215.4",
         "name" : "aws-us-east-1-shson-01",
         "id" : "aws-us-east-1-shson-01",
         "vnetId" : "aws-us-east-1-shson",
         "sshKeyId" : "aws-us-east-1-shson",
         "privateIP" : "192.168.1.108",
         "config_name" : "aws-us-east-1",
         "vmBlockDisk" : "/dev/sda1",
         "targetAction" : "None",
         "description" : "description",
         "specId" : "aws-us-east-1-shson",
         "publicDNS" : "",
         "vmUserPasswd" : ""
      },
      {
         "vmBlockDisk" : "/dev/sda1",
         "targetAction" : "None",
         "description" : "description",
         "specId" : "aws-us-east-1-shson",
         "vmUserPasswd" : "",
         ..........
      }
   ]
}
Dozing for 1 : 1 (Back to work)
####################################################################
## 6. VM: Status MCIS
####################################################################
[Test for AWS]
{
   "targetStatus" : "None",
   "id" : "aws-us-east-1-shson",
   "targetAction" : "None",
   "vm" : [
      {
         "publicIp" : "35.173.215.4",
         "nativeStatus" : "Running",
         "cspId" : "aws-us-east-1-shson-01",
         "name" : "aws-us-east-1-shson-01",
         "status" : "Running",
         "targetAction" : "None",
         "targetStatus" : "None",
         "id" : "aws-us-east-1-shson-01"
      },
      {
         "name" : "aws-us-east-1-shson-02",
         "status" : "Running",
         "targetAction" : "None",
         "targetStatus" : "None",
         "id" : "aws-us-east-1-shson-02",
         "publicIp" : "18.206.13.233",
         "cspId" : "aws-us-east-1-shson-02",
         "nativeStatus" : "Running"
      },
      {
         "targetAction" : "None",
         "id" : "aws-us-east-1-shson-03",
         "targetStatus" : "None",
         "name" : "aws-us-east-1-shson-03",
         "status" : "Running",
         "cspId" : "aws-us-east-1-shson-03",
         "nativeStatus" : "Running",
         "publicIp" : "18.232.53.134"
      }
   ],
   "status" : "Running-(3/3)",
   "name" : "aws-us-east-1-shson"
}

[Logging to notify latest command history]

[Executed Command List]
[CMD] create-all.sh gcp 1 shson
[CMD] create-all.sh alibaba 1 shson
[CMD] create-all.sh aws 1 shson
```

[Executed Command List] contains the history of the command performed.
(Can be checked by following command cat ./executionStatus )
      
</details>

#### Multi Cloud Infrastructure usecase

##### MCIS SSH Remote Commands
  - Access status can be checked through SSH remote command execution.
    - command-mcis.sh  # execute command on MCIS(multiple VM) created.
    - Execution Example
      - `./create-all.sh -n shson -f ../testSet.env`  # Create VM based on information in testSet.env
      - `./command-mcis.sh -n shson -f ../testSet.env`  # Check IP and Hostname of all VMs in MCIS

##### MCIS Nginx Distributed Deployment
- Distribute Nginx to test web server access.
    - deploy-nginx-mcis.sh  # Automatic deployment of Nginx on MCIS(multiple VM) created.
    - Execution example
      - deploy-nginx-mcis.sh -n shson -f ../testSetAws.env # Install Ngnix and webpages based on information in testSet.env on all VMs of MCIS.

##### MCIS Weave Scope Cluster Monitoring Distributed Deployment
  - [Deploying Weave Scope Cluster on MCIS through Scripts](https://github.com/cloud-barista/cb-tumblebug/wiki/MCIS-WeaveScope-deployment)

##### MCIS Jitsi Videoconferencing Deployment
  - [Deploying Jitsi Videoconferencing on MCIS throgh Scripts](https://github.com/cloud-barista/cb-tumblebug/wiki/MCIS-Jitsi-deployment)

##### MCIS Ansible Execution Envioronement Atutomatic Configuration
  - [Ansible Execution Envioronement Atutomatic Configuration on MCIS through Scripts](https://github.com/cloud-barista/cb-tumblebug/wiki/MCIS-Ansible-deployment)

##### MCIS Toy Game Server Deployment
  - [Deploying Toy Game on MCIS Through Scripts](https://github.com/cloud-barista/cb-tumblebug/wiki/MCIS-toy-game-deployment)



### Use CB-Tumblebug REST API
1. Create required VM resources(MCIR) Through CB-Tumblebug Multi Cloud Interface Resource(MCIR) management API
   - [Create VM spec object](https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BMCIR%5D%20Spec%20management/post_ns__nsId__resources_spec)
   - [Create VM image object](https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BMCIR%5D%20Image%20management/post_ns__nsId__resources_image)
   - [Create Virtual network object](https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BMCIR%5D%20Network%20management/post_ns__nsId__resources_vNet)
   - [Create Security group object](https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BMCIR%5D%20Security%20group%20management/post_ns__nsId__resources_securityGroup)
   - [Create VM ssh key object](https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BMCIR%5D%20Access%20key%20management/post_ns__nsId__resources_sshKey)
2. Through CB-Tumblebug Multi Cloud Infra Service(MCIS) Management API, create, check, control, send command, termination, and deletion of MCIS.
   - [Create MCIS](https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BMCIS%5D%20Provisioning%20management/post_ns__nsId__mcis)
   - [Send Commands to MCIS](https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BMCIS%5D%20Remote%20command/post_ns__nsId__cmd_mcis__mcisId_)
   - [Check and Control MCIS](https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BMCIS%5D%20Provisioning%20management/get_ns__nsId__mcis__mcisId_)
   - [Delete MCIS(only when MCIS is terminated)](https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BMCIS%5D%20Provisioning%20management/delete_ns__nsId__mcis)

***
***

## Contributors ✨

Thanks goes to these wonderful people ([emoji key](https://allcontributors.org/docs/en/emoji-key)):
<!-- ALL-CONTRIBUTORS-LIST:START - Do not remove or modify this section -->
<!-- prettier-ignore-start -->
<!-- markdownlint-disable -->
<table>
  <tr>
    <td align="center"><a href="https://seokho-son.github.io/"><img src="https://avatars3.githubusercontent.com/u/5966944?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Seokho Son</b></sub></a><br /><a href="#maintenance-seokho-son" title="Maintenance">🚧</a> <a href="#ideas-seokho-son" title="Ideas, Planning, & Feedback">🤔</a> <a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=seokho-son" title="Code">💻</a> <a href="https://github.com/cloud-barista/cb-tumblebug/pulls?q=is%3Apr+reviewed-by%3Aseokho-son" title="Reviewed Pull Requests">👀</a></td>
    <td align="center"><a href="https://jihoon-seo.github.io"><img src="https://avatars1.githubusercontent.com/u/46767780?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Jihoon Seo</b></sub></a><br /><a href="#maintenance-jihoon-seo" title="Maintenance">🚧</a> <a href="#ideas-jihoon-seo" title="Ideas, Planning, & Feedback">🤔</a> <a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=jihoon-seo" title="Code">💻</a> <a href="https://github.com/cloud-barista/cb-tumblebug/pulls?q=is%3Apr+reviewed-by%3Ajihoon-seo" title="Reviewed Pull Requests">👀</a></td>
    <td align="center"><a href="https://github.com/hermitkim1"><img src="https://avatars2.githubusercontent.com/u/7975459?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Yunkon (Alvin) Kim </b></sub></a><br /><a href="#ideas-hermitkim1" title="Ideas, Planning, & Feedback">🤔</a> <a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=hermitkim1" title="Code">💻</a> <a href="https://github.com/cloud-barista/cb-tumblebug/pulls?q=is%3Apr+reviewed-by%3Ahermitkim1" title="Reviewed Pull Requests">👀</a></td>
    <td align="center"><a href="https://github.com/jmleefree"><img src="https://avatars3.githubusercontent.com/u/64775292?v=4?s=100" width="100px;" alt=""/><br /><sub><b>jmleefree</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=jmleefree" title="Code">💻</a> <a href="https://github.com/cloud-barista/cb-tumblebug/pulls?q=is%3Apr+reviewed-by%3Ajmleefree" title="Reviewed Pull Requests">👀</a></td>
    <td align="center"><a href="http://www.powerkim.co.kr"><img src="https://avatars2.githubusercontent.com/u/46367962?v=4?s=100" width="100px;" alt=""/><br /><sub><b>ByoungSeob Kim</b></sub></a><br /><a href="#ideas-powerkimhub" title="Ideas, Planning, & Feedback">🤔</a></td>
    <td align="center"><a href="https://github.com/sykim-etri"><img src="https://avatars3.githubusercontent.com/u/25163268?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Sooyoung Kim</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/issues?q=author%3Asykim-etri" title="Bug reports">🐛</a> <a href="#ideas-sykim-etri" title="Ideas, Planning, & Feedback">🤔</a></td>
    <td align="center"><a href="https://github.com/dongjae"><img src="https://avatars.githubusercontent.com/u/5770239?v=4?s=100" width="100px;" alt=""/><br /><sub><b>KANG DONG JAE</b></sub></a><br /><a href="#ideas-dongjae" title="Ideas, Planning, & Feedback">🤔</a></td>
  </tr>
  <tr>
    <td align="center"><a href="http://www.etri.re.kr"><img src="https://avatars.githubusercontent.com/u/5266479?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Youngwoo-Jung</b></sub></a><br /><a href="#ideas-Youngwoo-Jung" title="Ideas, Planning, & Feedback">🤔</a></td>
    <td align="center"><a href="https://github.com/innodreamer"><img src="https://avatars.githubusercontent.com/u/51111668?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Sean Oh</b></sub></a><br /><a href="#ideas-innodreamer" title="Ideas, Planning, & Feedback">🤔</a></td>
    <td align="center"><a href="https://github.com/MZC-CSC"><img src="https://avatars.githubusercontent.com/u/78469943?v=4?s=100" width="100px;" alt=""/><br /><sub><b>MZC-CSC</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/issues?q=author%3AMZC-CSC" title="Bug reports">🐛</a> <a href="#ideas-MZC-CSC" title="Ideas, Planning, & Feedback">🤔</a></td>
    <td align="center"><a href="https://github.com/itnpeople"><img src="https://avatars.githubusercontent.com/u/35829386?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Eunsang</b></sub></a><br /><a href="#userTesting-itnpeople" title="User Testing">📓</a></td>
    <td align="center"><a href="https://github.com/hyokyungk"><img src="https://avatars.githubusercontent.com/u/51115778?v=4?s=100" width="100px;" alt=""/><br /><sub><b>hyokyungk</b></sub></a><br /><a href="#userTesting-hyokyungk" title="User Testing">📓</a></td>
    <td align="center"><a href="https://github.com/pjini"><img src="https://avatars.githubusercontent.com/u/64886639?v=4?s=100" width="100px;" alt=""/><br /><sub><b>pjini</b></sub></a><br /><a href="#userTesting-pjini" title="User Testing">📓</a></td>
    <td align="center"><a href="https://github.com/vlatte"><img src="https://avatars.githubusercontent.com/u/21170063?v=4?s=100" width="100px;" alt=""/><br /><sub><b>sunmi</b></sub></a><br /><a href="#userTesting-vlatte" title="User Testing">📓</a></td>
  </tr>
  <tr>
    <td align="center"><a href="https://github.com/limsg1234"><img src="https://avatars.githubusercontent.com/u/53066410?v=4?s=100" width="100px;" alt=""/><br /><sub><b>sglim</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=limsg1234" title="Documentation">📖</a></td>
    <td align="center"><a href="https://github.com/jangh-lee"><img src="https://avatars.githubusercontent.com/u/72970232?v=4?s=100" width="100px;" alt=""/><br /><sub><b>jangh-lee</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=jangh-lee" title="Documentation">📖</a></td>
    <td align="center"><a href="https://github.com/leedohun"><img src="https://avatars.githubusercontent.com/u/33706689?v=4?s=100" width="100px;" alt=""/><br /><sub><b>이도훈</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=leedohun" title="Documentation">📖</a></td>
    <td align="center"><a href="https://velog.io/@skynet"><img src="https://avatars.githubusercontent.com/u/26251856?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Park Beomsu</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=computerphilosopher" title="Code">💻</a></td>
    <td align="center"><a href="https://github.com/HassanAlsamahi"><img src="https://avatars.githubusercontent.com/u/42076287?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Hassan Alsamahi</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=HassanAlsamahi" title="Code">💻</a></td>
    <td align="center"><a href="https://github.com/atg0831"><img src="https://avatars.githubusercontent.com/u/44899448?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Taegeon An</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=atg0831" title="Code">💻</a></td>
    <td align="center"><a href="http://ihp001.tistory.com"><img src="https://avatars.githubusercontent.com/u/47745785?v=4?s=100" width="100px;" alt=""/><br /><sub><b>INHYO</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=PARKINHYO" title="Code">💻</a></td>
  </tr>
  <tr>
    <td align="center"><a href="https://github.com/Modney"><img src="https://avatars.githubusercontent.com/u/46340193?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Modney</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=Modney" title="Documentation">📖</a></td>
  </tr>
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


***
***

## License
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fcloud-barista%2Fcb-tumblebug.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2Fcloud-barista%2Fcb-tumblebug?ref=badge_large)


***
