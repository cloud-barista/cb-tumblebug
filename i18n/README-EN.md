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
- Linux (Recommended: Ubuntu v18.04)
- Go (Recommended: v1.16)

***
***

## How to contribute on CB-Tumblebug

CB-Tumblebug welcomes improvements from all contributors, new and experienced!

### (1) Types of Contribution

- Open an Issue
  - Bug report, Enhancement request, Feature request, ...
- Open PR (Pull-Request) 
  - Documentation, Source code, ...
  
### (2) Contribution Guide

- Overview
  - [Cloud-Barista Contribution Overview](https://github.com/cloud-barista/docs/blob/master/CONTRIBUTING.md#how-to-contribute)
  - [Cloud-Barista Code of Conduct](https://github.com/cloud-barista/docs/blob/master/contributing/CODE_OF_CONDUCT.md)
- In detail
  - [Open and update a PR](https://github.com/cloud-barista/docs/blob/master/contributing/how_to_open_a_pull_request-ko.md)
    - **Be careful!** 
      - Contributors should not push files related to their personal credencials (e.g., credentials.conf) to remote repository.
      - The credential file for CSPs (`src/testclient/scripts/credentials.conf`) is in the [.gitignore](https://github.com/cloud-barista/cb-tumblebug/blob/ed250835a1357784afd4c857d6bd56e0d78cd219/.gitignore#L36) condition.
      - So, `credentials.conf` will not be staged for a commit.
      - Anyway, please be careful.
  - [Test requirement for developers](https://github.com/cloud-barista/cb-tumblebug/wiki/Basic-testing-guide-before-a-contribution)
  - [Coding convention for developers](https://github.com/cloud-barista/cb-tumblebug/wiki/Coding-Convention)

***
***

## How To Run CB-Tumblebug 

### (1) Source Code based Installation and Execution

- 개요
  - Inatall tools and packages required
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
  cloudbaristaorg/cb-tumblebug:0.4.xx
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

- Inatall tools and packages required
  - Install Git, gcc and make 
    - `# apt update`
    - `# apt install make gcc git`

  - Inatall Go 
    - Install Go by referencing https://golang.org/dl/ (Version v1.16 or upper: recommened environment)
    - Installation Example
      - `wget https://golang.org/dl/go1.16.4.linux-amd64.tar.gz`
      - `tar -C /usr/local -xzf go1.16.4.linux-amd64.tar.gz`
      - add followings on the bottom of `.bashrc`
      ```
      export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin
      export GOPATH=$HOME/go
      ```
      - `source ~/.bashrc` (apply the changes to `.bashrc`)

- Clone CB-Tumblebug 
  - `# git clone https://github.com/cloud-barista/cb-tumblebug.git $HOME/go/src/github.com/cloud-barista/cb-tumblebug`

- Setting Environmentla Variable for executing CB-Tumblebug
  - `cb-tumblebug/conf/setup.env` 내용 확인 및 설정 (CB-Tumblebug 환경변수, 필요에 따라 변경)
    - `source setup.env` 실행으로 시스템에 반영
  - `cb-tumblebug/conf` 의 `store_conf.yaml` 내용 확인 및 설정 (cb-store 환경변수, 필요에 따라 변경)
    - storetype 지정 (NUTSDB 또는 ETCD 지정)
    - NUTSDB(local DB) 설정시 DB 데이터가 포함된 주소 지정이 필요 (기본은 `cb-tumblebug/meta_db/dat` 에 파일로 추가됨)
  - `cb-tumblebug/conf` 의 `log_conf.yaml` 내용 확인 및 설정 (cb-log 환경변수, 필요에 따라 변경)


### (2) Build CB-Tumblebug 

- Build Command
  ```Shell
  # cd ~/go/src/github.com/cloud-barista/cb-tumblebug/src
  # export GO111MODULE=on
  # make
  ```

- If Swagger API Document needs to be upadated run `make swag` at `cb-tumblebug/src/` directory.
  - API cdouemtn file is created at  `cb-tumblebug/src/api/rest/docs/swagger.yaml` directory.
  - Following API docunent can be checked on http://localhost:1323/tumblebug/swagger/index.html through web browser. (Automatically provided when CB-Tumblebug is executed)

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

   Multi-cloud infrastructure managemenet framework
   ________________________________________________

   https://github.com/cloud-barista/cb-tumblebug


   Access to API dashboard (username: default / password: default)
   http://xxx.xxx.xxx.xxx:1323/tumblebug/swagger/index.html?url=http://xxx.xxx.xxx.xxx:1323/tumblebug/swaggerActive

  ⇨ http server started on [::]:1323
  ⇨ grpc server started on [::]:50252
  ```

- Known Errors and Trubleshooting
  ``` 
  panic: /debug/requests is already registered. 
  You may have two independent copies of golang.org/x/net/trace in your binary, 
  trying to maintain separate state. 
  This may involve a vendored copy of golang.org/x/net/trace.
  ```

  run following to reslove if error occurs.
  ```Shell
  # rm -rf $GOPATH/src/go.etcd.io/etcd/vendor/golang.org/x/net/trace
  # make
  ```

***
***

## How to use CB-Tumblebug functions

1. [Using CB-Tumblebug Script](#using-cb-tumblebug-script)
2. [Use CB-Tumblebug REST API](#use-cb-tumblebug-rest-api)


### Using CB-Tumblebug Script
[`src/testclient/scripts/`](https://github.com/cloud-barista/cb-tumblebug/blob/main/src/testclient/scripts/)는 복잡한 단계가 필요한 MCIS (MC-Infra) 프로비저닝 절차를 간소화 및 자동화시킨 Bash shell 기반 스크립트를 제공.
 - 1 단계: [클라우드 인증 정보 및 테스트 기본 정보 입력](#클라우드-인증-정보-및-테스트-기본-정보-입력)
 - 2 단계: Namespace, MCIR, MCIS 등 프로비저닝 (통합 제어 / 개별 제어 중 선택)
   - [개별 제어 시험](개별-제어-시험) (Namespace, MCIR, MCIS 등 개별 시험시, 오브젝트들의 의존성 고려 필수)
   - [통합 제어 시험](통합-제어-시험) (추천 테스트 방법) `src/testclient/scripts/sequentialFullTest/`
 - 3 단계: [멀티 클라우드 인프라 유스케이스 자동 배포](#멀티-클라우드-인프라-유스케이스

#### 클라우드 인증 정보 및 테스트 기본 정보 입력
1. [`src/testclient/scripts/`](https://github.com/cloud-barista/cb-tumblebug/tree/main/src/testclient/scripts) 이동
2. `credentials.conf` 생성 
   - `credentials.conf` 는 기본적인 클라우드 타입 (AWS, GCP, AZURE, ALIBABA 등)에 대해 인증 정보 템플릿 제공
   - [`credentials.conf.example`](https://github.com/cloud-barista/cb-tumblebug/blob/main/src/testclient/scripts/credentials.conf.example)를 참조하여 항목에 사용자의 클라우드 인증 정보 입력
   - [CSP별 인증 정보 획득 방법 참고](https://github.com/cloud-barista/cb-tumblebug/wiki/How-to-get-public-cloud-credentials)
3. [`conf.env`](https://github.com/cloud-barista/cb-tumblebug/blob/main/src/testclient/scripts/conf.env) 설정
   - CB-Spider 및 CB-Tumblebug 서버 엔드포인트, 클라우드 리젼, 테스트용 이미지명, 테스트용 스팩명 등 테스트 기본 정보 제공
   - 이미 많은 클라우드 타입에 대한 정보가 조사 및 입력되어 있으므로, 수정없이 사용 가능. (단, 지정된 Spec에 따라 과금이 발생할 수 있으므로 확인 필요)
     - 테스트용 VM 이미지 수정 방식: [`IMAGE_NAME[$IX,$IY]=ami-061eb2b23f9f8839c`](https://github.com/cloud-barista/cb-tumblebug/blob/553c4884943916b3287ec17501c6f639e8667897/src/testclient/scripts/conf.env#L49)
     - 테스트용 VM 스펙 수정 방식: [`SPEC_NAME[$IX,$IY]=m4.4xlarge`](https://github.com/cloud-barista/cb-tumblebug/blob/553c4884943916b3287ec17501c6f639e8667897/src/testclient/scripts/conf.env#L50)   
4. [`testSet.env`](https://github.com/cloud-barista/cb-tumblebug/blob/main/src/testclient/scripts/testSet.env) 설정
   - MCIS 프로비저닝에 사용될, 클라우드 및 리전 구성을 파일로 설정 (기존의 `testSet.env` 를 변경해도 되고, 복사하여 활용도 가능)
   - 조합할 CSP 종류 지정
     - 조합할 총 CSP 개수 지정 ([NumCSP=](https://github.com/cloud-barista/cb-tumblebug/blob/553c4884943916b3287ec17501c6f639e8667897/src/testclient/scripts/testSet.env#L9) 에 숫자를 변경)
     - 조합할 CSP 종류는 [L15-L24](https://github.com/cloud-barista/cb-tumblebug/blob/553c4884943916b3287ec17501c6f639e8667897/src/testclient/scripts/testSet.env#L15)의 라인 상 순서를 변경하여 지정 (NumCSP에 지정된 개수까지 활용)
     - 예: aws, alibaba 로 조합하고 싶은 경우: [NumCSP=2](https://github.com/cloud-barista/cb-tumblebug/blob/553c4884943916b3287ec17501c6f639e8667897/src/testclient/scripts/testSet.env#L9) 로 변경하고, `IndexAWS=$((++IX))`, `IndexAlibaba=$((++IX))` 순으로 재정렬
   - 조합할 CSP의 리전 지정
     - 각 CSP 설정 항목으로 이동 [`# AWS (Total: 21 Regions)`](https://github.com/cloud-barista/cb-tumblebug/blob/553c4884943916b3287ec17501c6f639e8667897/src/testclient/scripts/testSet.env#L44) 
     - `NumRegion[$IndexAWS]=2` 에서 구성하고 싶은 리전의 수를 지정 (예시에서는 2로 지정)
     - 리전 리스트의 라인 순서를 변경하여 원하는 리전으로 셋팅 (`NumRegion[$IndexAWS]=2` 인 경우 가장 위에 나열된 2개의 리전이 선택)
   - **Be aware!** 
     - Be aware that creating VMs on public CSPs such as AWS, GCP, Azure, etc. may be billed.
     - With the default setting of [`testSet.env`](https://github.com/cloud-barista/cb-tumblebug/blob/main/src/testclient/scripts/testSet.env), TestClouds (`TestCloud01`, `TestCloud02`, `TestCloud03`) will be used to create mock VMs.
     - `TestCloud01`, `TestCloud02`, `TestCloud03` are not real CSPs. It is used for testing purpose. (not support SSH into VM)
     - Anyway, please be aware cloud usage cost when you use public CSPs.

#### Individual Control Test
- For resource that you want to control, go to following directory to run the test needed
  - 오브젝트는 서로 의존성이 있으므로, 번호를 참고하여 오름차순으로 수행하는 것이 바람직함
    - `1.configureSpider`  # 클라우드 정보 등록 관련 스크립트 모음
    - `2.configureTumblebug`  # 네임스페이스 및 동적 환경 설정 관련 스크립트 모음
    - `3.vNet`  # MCIR vNet 생성 관련 스크립트 모음
    - `4.securityGroup`  # MCIR securityGroup 생성 관련 스크립트 모음
    - `5.sshKey`  # MCIR sshKey 생성 관련 스크립트 모음
    - `6.image`  # MCIR image 등록 관련 스크립트 모음
    - `7.spec`  # MCIR spec 등록 관련 스크립트 모음
    - `8.mcis`  # MCIS 생성 및 제어, MCIS 원격 커맨드 등 스크립트 모음
    - `9.monitoring`  # CB-TB를 통해서 CB-DF 모니터링 에이전트 설치 및 모니터링 테스트 스크립트 모음

#### Integrated Control Test
- `src/testclient/scripts/sequentialFullTest/` 에 포함된 `create-all.sh` 및 `clean-all.sh` 을 수행하면 전체 과정을 한번에 테스트 가능
- By running `create-all.sh` and `clean-all.sh` in `src/testclient/scripts/sequentialFullTest/` dritectory, 
  ```
  └── sequentialFullTest  # Cloud 정보 등록, NS 생성, MCIR 생성, MCIS 생성까지 한번에 자동 테스트
      ├── check-test-config.sh  # 현재 testSet에 지정된 멀티 클라우드 인프라 구성을 확인
      ├── create-all.sh  # Cloud 정보 등록, NS 생성, MCIR 생성, MCIS 생성까지 한번에 자동 테스트
      ├── gen-sshKey.sh  # 수행이 진행된 테스트 로그 (MCIS에 접속 가능한 SSH키 파일 생성)  
      ├── command-mcis.sh  # 생성된 MCIS(다중VM)에 원격 명령 수행
      ├── deploy-nginx-mcis.sh  # 생성된 MCIS(다중VM)에 Nginx 자동 배포  
      ├── create-mcis-for-df.sh  # CB-Dragonfly 호스팅을 위한 MCIS 생성        
      ├── deploy-dragonfly-docker.sh  # MCIS에 CB-Dragonfly 자동 배포 및 환경 자동 설정      
      ├── clean-all.sh  # 모든 오브젝트를 생성의 역순으로 삭제
      └── executionStatus  # 수행이 진행된 테스트 로그 (testAll 수행시 정보가 추가되며, cleanAll 수행시 정보가 제거됨. 진행중인 작업 확인 가능)

  ```
- Usage Example
  - MCIS생성 테스트
    - `./create-all.sh -n shson -f ../testSetCustom.env`   # ../testSetCustom.env 에 구성된 클라우드 조합으로 MCIS 생성 수행
    - ../testSetCustom.env에 구성된 MCIS 생성 형상을 확인하는 절차가 자동으로 진행됨
    - 실행 결과 예시
      ```
      Table: All VMs in the MCIS : cb-shson

      ID              Status   PublicIP       PrivateIP      CloudType  CloudRegion     CreatedTime
      --              ------   --------       ---------      ---------  -----------     -----------
      aws-ap-se-1-0   Running  xx.250.xx.73   192.168.2.180  aws        ap-southeast-1  2021-09-17   14:59:30
      aws-ca-ct-1-0   Running  x.97.xx.230    192.168.4.98   aws        ca-central-1    2021-09-17   14:59:58
      gcp-as-east1-0  Running  xx.229.xxx.26  192.168.3.2    gcp        asia-east1      2021-09-17   14:59:42

      [DATE: 17/09/2021 15:00:00] [ElapsedTime: 49s (0m:49s)] [Command: ./create-mcis-only.sh all 1 shson ../testSetCustom.env 1]

      [Executed Command List]
      [MCIR:aws-ap-southeast-1(28s)] create-mcir-ns-cloud.sh (MCIR) aws 1 shson ../testSetCustom.env
      [MCIR:aws-ca-central-1(34s)] create-mcir-ns-cloud.sh (MCIR) aws 2 shson ../testSetCustom.env
      [MCIR:gcp-asia-east1(93s)] create-mcir-ns-cloud.sh (MCIR) gcp 1 shson ../testSetCustom.env
      [MCIS:cb-shsonvm4(19s+More)] create-mcis-only.sh (MCIS) all 1 shson ../testSetCustom.env

      [DATE: 17/09/2021 15:00:00] [ElapsedTime: 149s (2m:29s)] [Command: ./create-all.sh -n shson -f ../testSetCustom.env -x 1]
      ```
  - MCIS제거 테스트 (생성에서 활용한 입력 파라미터로 삭제 필요)
    - `./clean-all.sh -n shson -f ../testSetCustom.env`   # ../testSetCustom.env 에 구성된 클라우드 조합으로 제거 수행
    - **Be aware!** 
      - If you created MCIS (VMs) for testing in public clouds, the VMs may be charged.
      - You need to termiate MCIS by using `clean-all` to avoid unexpected billing.
      - Anyway, please be aware cloud usage cost when you use public CSPs.
  - MCIS SSH 접속키 생성 및 각 VM에 접속
    - `./gen-sshKey.sh -n shson -f ../testSetCustom.env`  # MCIS에 구성된 모든 VM의 접속키 리턴
    - Executuon Result Example
      ```
      ...
      [GENERATED PRIVATE KEY (PEM, PPK)]
      [MCIS INFO: mc-shson]
       [VMIP]: 13.212.254.59   [MCISID]: mc-shson   [VMID]: aws-ap-se-1-0
       ./sshkey-tmp/aws-ap-se-1-shson.pem 
       ./sshkey-tmp/aws-ap-se-1-shson.ppk
       ...
       
      [SSH COMMAND EXAMPLE]
       [VMIP]: 13.212.254.59   [MCISID]: mc-shson   [VMID]: aws-ap-se-1-0
       ssh -i ./sshkey-tmp/aws-ap-se-1-shson.pem cb-user@13.212.254.59 -o StrictHostKeyChecking=no
       ...
       [VMIP]: 35.182.30.37   [MCISID]: mc-shson   [VMID]: aws-ca-ct-1-0
       ssh -i ./sshkey-tmp/aws-ca-ct-1-shson.pem cb-user@35.182.30.37 -o StrictHostKeyChecking=no
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
## 0. Create Cloud Connction Config
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
         "vmBootDisk" : "/dev/sda1",
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
            "VMBootDisk" : "/dev/sda1",
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

마지막의 [Executed Command List] 에는 수행한 커맨드의 히스토리가 포함됨. 
(cat ./executionStatus 를 통해 다시 확인 가능)
      
</details>

#### Multi Cloud Infrastructure usecase

##### MCIS SSH Remote Commands
  - SSH 원격 커맨드 실행을 통해서 접속 여부 등을 확인 가능
    - command-mcis.sh  # 생성된 MCIS(다중VM)에 원격 명령 수행
    - Execution Example
      - `./create-all.sh -n shson -f ../testSet.env`  # testSet.env 에 구성된 정보를 기준으로 MCIS 생성
      - `./command-mcis.sh -n shson -f ../testSet.env`  # MCIS의 모든 VM에 IP 및 Hostname 조회를 수행

##### MCIS Nginx Distributed Deployment
- Nginx를 분산 배치하여, 웹서버 접속 시험 가능
    - deploy-nginx-mcis.sh  # 생성된 MCIS(다중VM)에 Nginx 자동 배포
    - 실행 예시
      - deploy-nginx-mcis.sh -n shson -f ../testSetAws.env # testSetAws.env 에 구성된 정보를 기준으로 MCIS의 모든 VM에 Nginx 및 웹페이지 설치

##### MCIS Weave Scope Cluster Monitoring Distributed Deployment
  - [Deploying Weave Scope Cluster on MCIS through Scripts](https://github.com/cloud-barista/cb-tumblebug/wiki/MCIS-WeaveScope-deployment)

##### MCIS Jitsi Videoconferencing Deployment
  - [Deploying Jitsi Videoconferencing on MCIS throgh Scripts](https://github.com/cloud-barista/cb-tumblebug/wiki/MCIS-Jitsi-deployment)

##### MCIS Ansible Execution Envioronement Atutomatic Configuration
  - [Ansible Execution Envioronement Atutomatic Configuration on MCIS through Scripts](https://github.com/cloud-barista/cb-tumblebug/wiki/MCIS-Ansible-deployment)

##### MCIS Toy Game Server Deployment
  - [Deploying Toy Game on MCIS Through Scripts](https://github.com/cloud-barista/cb-tumblebug/wiki/MCIS-toy-game-deployment)



### Use CB-Tumblebug REST API
1. CB-Spider API를 통해 클라우드 인프라 연동 정보 등록
   - https://cloud-barista.github.io/rest-api/v0.4.0/spider/ccim/
2. CB-Tumblebug 멀티 클라우드 네임스페이스 관리 API를 통해서 Namespace 생성
   - [Namespace 생성](https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BNamespace%5D%20Namespace%20management/post_ns)
3. CB-Tumblebug 멀티 클라우드 인프라 자원(MCIR) 관리 API를 통해서 VM 생성을 위한 자원 (MCIR) 생성
   - [VM spec object 생성](https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BMCIR%5D%20Spec%20management/post_ns__nsId__resources_spec)
   - [VM image object 생성](https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BMCIR%5D%20Image%20management/post_ns__nsId__resources_image)
   - [Virtual network object 생성](https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BMCIR%5D%20Network%20management/post_ns__nsId__resources_vNet)
   - [Security group object 생성](https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BMCIR%5D%20Security%20group%20management/post_ns__nsId__resources_securityGroup)
   - [VM 접속 ssh key object 생성](https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BMCIR%5D%20Access%20key%20management/post_ns__nsId__resources_sshKey)
4. CB-Tumblebug 멀티 클라우드 인프라 서비스(MCIS) 관리 API를 통해서 MCIS 생성, 조회, 제어, 원격명령수행, 종료 및 삭제
   - [MCIS 생성](https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BMCIS%5D%20Provisioning%20management/post_ns__nsId__mcis)
   - [MCIS 원격 커맨드](https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BMCIS%5D%20Remote%20command/post_ns__nsId__cmd_mcis__mcisId_)
   - [MCIS 조회 및 제어](https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BMCIS%5D%20Provisioning%20management/get_ns__nsId__mcis__mcisId_)
   - [MCIS 삭제(MCIS 종료 상태에서만 동작 가능)](https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BMCIS%5D%20Provisioning%20management/delete_ns__nsId__mcis)

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
