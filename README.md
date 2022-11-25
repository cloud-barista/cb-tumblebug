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
[![All Contributors](https://img.shields.io/badge/all_contributors-40-orange.svg?style=flat-square)](#contributors-)
<!-- ALL-CONTRIBUTORS-BADGE:END -->

A sub-system of Cloud-Barista Platform to Deploy and Manage Multi-Cloud Infrastructure.
 - [CB-Tumblebug Overview (Korean)](https://github.com/cloud-barista/cb-tumblebug/wiki/CB-Tumblebug-Overview) 
 - [CB-Tumblebug Features (Korean)](https://github.com/cloud-barista/cb-tumblebug/wiki/CB-Tumblebug-Features)
 - [CB-Tumblebug Architecture (Korean)](https://github.com/cloud-barista/cb-tumblebug/wiki/CB-Tumblebug-Architecture)
 - [CB-Tumblebug Operation Sequence](https://github.com/cloud-barista/cb-tumblebug/tree/main/docs/designUML)

<details>
<summary>[Note] CB-Tumblebug is currently under development</summary>

```
CB-Tumblebug is currently under development. (not v1.0 yet)
We welcome any new suggestions, issues, opinions, and contributors !
Please note that the functionalities of Cloud-Barista are not stable and secure yet.
Be careful if you plan to use the current release in production.
If you have any difficulties in using Cloud-Barista, please let us know.
(Open an issue or Join the Cloud-Barista Slack)
```
  
</details>

<details>
<summary>[Note] Localization and Globalization of CB-Tumblebug</summary>
    
```
Since CB-Tumblebug was initiated by Korean members, 
we would like to promote participation of Korean contributors during initial stage of this project. 
So, CB-Tumblebug Repo will accept use of Korean language in its early stages.
On the other hand, we hope this project flourishes regardless of contributor's country eventually.
So, the maintainers recommend using English at least for the title of Issues, Pull Requests, and Commits, 
while CB-Tumblebug Repo accommodates local languages in the contents of them.
```

```
CB-Tumblebug은 한국에서 시작된 오픈 소스 프로젝트로서 
프로젝트의 초기 단계에는 한국 기여자들의 참여를 촉진하고자 합니다. 
따라서 초기 단계의 CB-Tumblebug는 한국어 사용을 받아 들일 것입니다.
다른 한편으로, 이 프로젝트가 국가에 관계없이 번성하기를 희망합니다.
따라서 개발 히스토리 관리를 위해 이슈, 풀 요청, 커밋 등의 
제목에 대해서는 영어 사용을 권장하며, 내용에 대한 한국어 사용은 수용할 것입니다.
```

</details>


[[한국어](README.md), [English](i18n/README-EN.md)]

***
***


## 목차

1. [CB-Tumblebug 실행 및 개발 환경](#cb-tumblebug-실행-및-개발-환경)
2. [CB-Tumblebug 기여 방법](#cb-tumblebug-기여-방법)
3. [CB-Tumblebug 실행 방법](#cb-tumblebug-실행-방법)
4. [CB-Tumblebug 소스 빌드 및 실행 방법 상세](#cb-tumblebug-소스-빌드-및-실행-방법-상세)
5. [CB-Tumblebug 기능 사용 방법](#cb-tumblebug-기능-사용-방법)

***
***

## CB-Tumblebug 실행 및 개발 환경
- Linux (추천: Ubuntu 18.04)
- Go (추천: v1.19)

***
***

## CB-Tumblebug 기여 방법

CB-Tumblebug welcomes improvements from both new and experienced contributors!

Check out [CONTRIBUTING](https://github.com/cloud-barista/cb-tumblebug/blob/main/CONTRIBUTING.md).

***
***

## CB-Tumblebug 실행 방법

### (1) 소스 코드 기반 설치 및 실행

- 개요
  - 필요 패키지/도구 설치
  - CB-Tumblebug 소스 다운로드 (Git clone CB-Tumblebug)
  - CB-Tumblebug 환경 변수 설정
  - CB-Tumblebug 빌드 및 실행 (`make` 및 `make run`)
- [소스 빌드 및 실행 방법 상세](#cb-tumblebug-소스-빌드-및-실행-방법-상세)
  
### (2) 컨테이너 기반 실행

- CB-Tumblebug 이미지 확인(https://hub.docker.com/r/cloudbaristaorg/cb-tumblebug/tags)
- CB-Tumblebug 컨테이너 실행

  ```bash
  ./scripts/runTumblebug.sh
  ```

  or

  ```bash
  docker run -p 1323:1323 -p 50252:50252 \
  -v ${HOME}/go/src/github.com/cloud-barista/cb-tumblebug/meta_db:/app/meta_db \
  --name cb-tumblebug \
  cloudbaristaorg/cb-tumblebug:x.x.x
  ```

### (3) cb-operator 기반 Cloud-Barista 통합 실행

- [cb-operator](https://github.com/cloud-barista/cb-operator)를 통해 CB-TB를 포함한 Cloud-Barista 전체 FW를 통합 실행 가능

  ```
  $ git clone https://github.com/cloud-barista/cb-operator.git
  $ cd cb-operator/src
  cb-operator/src$ make
  cb-operator/src$ ./operator
  ```

***
***

## CB-Tumblebug 소스 빌드 및 실행 방법 상세

### (1) CB-Tumblebug 빌드 환경 구성

- 필요 패키지 또는 도구 설치
  - Git, gcc, make 설치
    ```bash
    sudo apt update
    sudo apt install make gcc git
    ```
  - Go 설치
    - https://golang.org/dl/ 를 참고하여 Go 설치 (추천 개발 환경: Go v1.19 이상)
    - 설치 예시
      - Go 다운로드 및 압축 해제 
        ```bash
        wget https://go.dev/dl/go1.19.linux-amd64.tar.gz
        sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.19.linux-amd64.tar.gz
        ```
      - `.bashrc` 파일 하단에 다음을 추가 
        ```bash
        echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin' >> ~/.bashrc
        echo 'export GOPATH=$HOME/go' >> ~/.bashrc
        ```
      - `.bashrc` 변경 내용을 적용
        ```bash
        source ~/.bashrc
        echo $GOPATH
        ```

- CB-Tumblebug 소스 다운로드
  - CB-Tumblebug 저장소 클론
    ```bash
    git clone https://github.com/cloud-barista/cb-tumblebug.git $HOME/go/src/github.com/cloud-barista/cb-tumblebug
    cd ~/go/src/github.com/cloud-barista/cb-tumblebug
    ```
  - CB-Tumblebug 디렉토리 이동 alias 등록 (편의를 위한 선택 사항. cdtb, cbtbsrc, cdtbtest 키워드로 디렉토리 이동)
    ```bash
    echo "alias cdtb='cd $HOME/go/src/github.com/cloud-barista/cb-tumblebug'" >> ~/.bashrc
    echo "alias cdtbsrc='cd $HOME/go/src/github.com/cloud-barista/cb-tumblebug/src'" >> ~/.bashrc
    echo "alias cdtbtest='cd $HOME/go/src/github.com/cloud-barista/cb-tumblebug/src/testclient/scripts'" >> ~/.bashrc
    source ~/.bashrc
    ```

### (2) CB-Tumblebug 빌드

- 빌드 명령어
  ```bash
  cd ~/go/src/github.com/cloud-barista/cb-tumblebug/src
  export GO111MODULE=on
  make
  ```

- Swagger API 문서 업데이트 필요 시 `cb-tumblebug/src/` 에서 `make swag` 실행
  - API 문서 파일은 `cb-tumblebug/src/api/rest/docs/swagger.yaml` 에 생성됨
  - 해당 API 문서는 http://localhost:1323/tumblebug/swagger/index.html 로컬에서 웹브라우저로 확인 가능 (CB-Tumblebug 구동 시 자동으로 제공)
  - Swagger 기반 [API 문서 업데이트 방법 상세 정보](https://github.com/cloud-barista/cb-tumblebug/wiki/API-Document-Update)

### (3) CB-Tumblebug 실행

- CB-Spider 실행 
  - CB-Tumblebug은 클라우드 제어를 위해서 CB-Spider를 활용(필수 구동)
  - (추천 실행 방법) CB-TB 스크립트를 통한 CB-Spider 컨테이너 실행
    ```bash
    cd ~/go/src/github.com/cloud-barista/cb-tumblebug
    export CBTUMBLEBUG_ROOT=$HOME/go/src/github.com/cloud-barista/cb-tumblebug
    ./scripts/runSpider.sh
    ```
  - 상세 설치 방법은 [CB-Spider](https://github.com/cloud-barista/cb-spider) 참고
 
- CB-Tumblebug 실행에 필요한 환경변수 설정 (다른 탭에서)
  - `cb-tumblebug/conf/setup.env` 내용 확인 및 설정 (CB-Tumblebug 환경변수, 필요에 따라 변경)
    - 환경변수를 시스템에 반영 
      ```bash
      cd ~/go/src/github.com/cloud-barista/cb-tumblebug
      cat conf/setup.env
      source conf/setup.env
      ```
    - 필요에 따라 SELF_ENDPOINT 환경변수(외부에서 접속 가능한 주소)를 스크립트를 통해 자동으로 지정 
      - CB-Tumblebug을 실행하면 Swagger API Dashboard가 활성화되며, 외부에서 Dashboard에 접속 및 제어하려는 경우에 필요
      ```bash
      cd ~/go/src/github.com/cloud-barista/cb-tumblebug
      source ./scripts/setPublicIP.sh
      ```
  - `cb-tumblebug/conf` 의 `store_conf.yaml` 내용 확인 및 설정 (cb-store 환경변수, 필요에 따라 변경)
      - storetype 지정 (NUTSDB 또는 ETCD 지정)
      - NUTSDB(local DB) 설정시 DB 데이터가 포함된 주소 지정이 필요 (기본은 `cb-tumblebug/meta_db/dat` 에 파일로 추가됨)
  - `cb-tumblebug/conf` 의 `log_conf.yaml` 내용 확인 및 설정 (cb-log 환경변수, 필요에 따라 변경)
 
- CB-Tumblebug 실행
    ```bash
    cd ~/go/src/github.com/cloud-barista/cb-tumblebug/src
    make run
    ```

  CB-Tumblebug 서버 실행 화면
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
   http://xxx.xxx.xxx.xxx:1323/tumblebug/swagger/index.html

  ⇨ http server started on [::]:1323
  ⇨ grpc server started on [::]:50252
  ```

- 알려진 에러 및 해결 방법 
  ``` 
  panic: /debug/requests is already registered. 
  You may have two independent copies of golang.org/x/net/trace in your binary, 
  trying to maintain separate state. 
  This may involve a vendored copy of golang.org/x/net/trace.
  ```

  에러 발생 시, 다음을 실행하여 해결
  ```bash
  rm -rf $GOPATH/src/go.etcd.io/etcd/vendor/golang.org/x/net/trace
  make
  ```

### (4) CB-Tumblebug 멀티 클라우드 환경 설정

- 클라우드 credential 등록을 위한 `credentials.conf` 생성 및 정보 입력
   - 개요
     - `credentials.conf` 는 CB-TB가 지원하는 클라우드 타입 (AWS, GCP, AZURE, ALIBABA 등)에 대해 사용자 인증 정보를 입력 및 보관하는 파일
     - [`conf/template.credentials.conf`](https://github.com/cloud-barista/cb-tumblebug/blob/main/conf/template.credentials.conf)를 참조하여 `credentials.conf` 파일 생성 및 내용 입력 필요
   - 파일 생성 방법: CB-TB 스크립트를 통해 `credentials.conf` 파일 자동 생성
     ```bash
     cd ~/go/src/github.com/cloud-barista/cb-tumblebug
     ./scripts/genCredencialFile.sh
     ```
   - 정보 입력 방법: `conf/credentials.conf`에 사용자 정보 입력 ([참고: CSP별 인증 정보 획득 방법](https://github.com/cloud-barista/cb-tumblebug/wiki/How-to-get-public-cloud-credentials))

- 모든 멀티 클라우드 연결 정보 및 공통 자원 등록 
   - 개요
     - CB-TB의 멀티클라우드 인프라를 생성하기 위해서 클라우드에 대한 연결 정보 (크리덴셜, 클라우드 종류, 클라우드 리젼 등), 공통 활용 이미지 및 스펙 등의 등록이 필요
     - [`conf.env`](https://github.com/cloud-barista/cb-tumblebug/blob/main/src/testclient/scripts/conf.env): 클라우드 리젼 등 기본 정보 제공 (수정없이 사용 가능)
   - 등록 방법: `initMultiCloudEnv.sh` 스크립트 실행 (모든 확인 메시지에 대해 'y' 입력)
       ```bash
       cd ~/go/src/github.com/cloud-barista/cb-tumblebug
       ./scripts/initMultiCloudEnv.sh
       ```
     - [`conf.env`](https://github.com/cloud-barista/cb-tumblebug/blob/main/src/testclient/scripts/conf.env)의 연결 정보 자동 등록됨
     - [`assets`](https://github.com/cloud-barista/cb-tumblebug/tree/main/assets)의 파일에 기록된 공통 이미지 및 스펙 자동 등록됨

***
***

## CB-Tumblebug 기능 사용 방법

1. [CB-Tumblebug 스크립트 사용](#cb-tumblebug-스크립트-사용)
1. [CB-Tumblebug REST API 사용](#cb-tumblebug-rest-api-사용)


### CB-Tumblebug 스크립트 사용
[`src/testclient/scripts/`](https://github.com/cloud-barista/cb-tumblebug/blob/main/src/testclient/scripts/)는 복잡한 단계가 필요한 MCIS (MC-Infra) 프로비저닝 절차를 간소화 및 자동화시킨 Bash shell 기반 스크립트를 제공.
 - 1 단계: [클라우드 인증 정보 및 테스트 기본 정보 입력](#클라우드-인증-정보-및-테스트-기본-정보-입력)
 - 2 단계: Namespace, MCIR, MCIS 등 프로비저닝 (통합 제어 / 개별 제어 중 선택)
   - [개별 제어 시험](#개별-제어-시험) (Namespace, MCIR, MCIS 등 개별 시험시, 오브젝트들의 의존성 고려 필수)
   - [통합 제어 시험](#통합-제어-시험) (추천 테스트 방법) `src/testclient/scripts/sequentialFullTest/`
 - 3 단계: [멀티 클라우드 인프라 유스케이스 자동 배포](#멀티-클라우드-인프라-유스케이스)

#### 클라우드 인증 정보 및 테스트 기본 정보 입력
1. [`src/testclient/scripts/`](https://github.com/cloud-barista/cb-tumblebug/tree/main/src/testclient/scripts) 이동
2. [`conf.env`](https://github.com/cloud-barista/cb-tumblebug/blob/main/src/testclient/scripts/conf.env) 설정
   - CB-Spider 및 CB-Tumblebug 서버 엔드포인트, 클라우드 리젼, 테스트용 이미지명, 테스트용 스팩명 등 테스트 기본 정보 제공
   - 이미 많은 클라우드 타입에 대한 정보가 조사 및 입력되어 있으므로, 수정없이 사용 가능. (단, 지정된 Spec에 따라 과금이 발생할 수 있으므로 확인 필요)
     - 테스트용 VM 이미지 수정 방식: [`IMAGE_NAME[$IX,$IY]=ami-061eb2b23f9f8839c`](https://github.com/cloud-barista/cb-tumblebug/blob/553c4884943916b3287ec17501c6f639e8667897/src/testclient/scripts/conf.env#L49)
     - 테스트용 VM 스펙 수정 방식: [`SPEC_NAME[$IX,$IY]=m4.4xlarge`](https://github.com/cloud-barista/cb-tumblebug/blob/553c4884943916b3287ec17501c6f639e8667897/src/testclient/scripts/conf.env#L50)   
3. [`testSet.env`](https://github.com/cloud-barista/cb-tumblebug/blob/main/src/testclient/scripts/testSet.env) 설정
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

#### 개별 제어 시험
- 제어하고 싶은 리소스 오브젝트에 대해, 해당 디렉토리로 이동하여 필요한 시험 수행
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

#### 통합 제어 시험
- `src/testclient/scripts/sequentialFullTest/` 에 포함된 `create-all.sh` 및 `clean-all.sh` 을 수행하면 전체 과정을 한번에 테스트 가능
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
- 사용 방식
  - MCIS 생성 테스트
    - `./create-all.sh -n shson -f ../testSetCustom.env`   # ../testSetCustom.env 에 구성된 클라우드 조합으로 MCIS 생성 수행
    - ../testSetCustom.env에 구성된 MCIS 생성 형상을 확인하는 절차 자동으로 진행
    - 실행 결과 예시
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
      
  - MCIS 제거 테스트 (생성에서 활용한 입력 파라미터로 삭제 필요)
    - `./clean-all.sh -n shson -f ../testSetCustom.env`   # ../testSetCustom.env 에 구성된 클라우드 조합으로 제거 수행
    - **Be aware!** 
      - If you created MCIS (VMs) for testing in public clouds, the VMs may be charged.
      - You need to termiate MCIS by using `clean-all` to avoid unexpected billing.
      - Anyway, please be aware cloud usage cost when you use public CSPs.
      
  - MCIS SSH 접속키 생성 및 각 VM에 접속
    - `./gen-sshKey.sh -n shson -f ../testSetCustom.env`  # MCIS에 구성된 모든 VM의 접속키 리턴
    - 실행 결과 예시
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

  - MCIS SSH 원격 커맨드 실행을 통해 VM 통합 커맨드 확인
    - `./command-mcis.sh -n shson -f ../testSetCustom.env`  # MCIS의 모든 VM에 IP 및 Hostname 조회를 수행


- CB-MapUI 를 통해 MCIS 형상 확인 및 제어 가능
  - CB-Tumblebug은 지도 형태로 MCIS 배포 형상 확인을 위해 [CB-MapUI](https://github.com/cloud-barista/cb-mapui)를 활용
  - (추천 실행 방법) CB-TB 스크립트를 통한 CB-MapUI 컨테이너 실행
    ```bash
    cd ~/go/src/github.com/cloud-barista/cb-tumblebug
    export CBTUMBLEBUG_ROOT=$HOME/go/src/github.com/cloud-barista/cb-tumblebug
    ./scripts/runMapUI.sh
    ```
  - 웹브라우저에서 http://{HostIP}:1324 주소 접속
    
<details>
<summary>입출력 예시 보기</summary>

```bash
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

마지막의 [Executed Command List] 에는 수행한 커맨드의 히스토리가 포함됨. 
(cat ./executionStatus 를 통해 다시 확인 가능)
      
</details>

#### 멀티 클라우드 인프라 유스케이스

##### MCIS에 Nginx 분산 배치
  - deploy-nginx-mcis.sh  # 생성된 MCIS(다중VM)에 Nginx 자동 배포

##### MCIS Weave Scope 클러스터 모니터링 분산 배치
  - [스크립트를 통해 MCIS에 Weave Scope 클러스터 설치](https://github.com/cloud-barista/cb-tumblebug/wiki/MCIS-WeaveScope-deployment)

##### MCIS Jitsi 영상 회의 배치
  - [스크립트를 통해 MCIS에 Jitsi 영상회의 설치](https://github.com/cloud-barista/cb-tumblebug/wiki/MCIS-Jitsi-deployment)

##### MCIS Ansible 실행 환경 자동 구성
  - [스크립트를 통해 MCIS에 Ansible 실행 환경 자동 구성](https://github.com/cloud-barista/cb-tumblebug/wiki/MCIS-Ansible-deployment)

##### MCIS 토이 게임 서버 배치
  - [스크립트를 통해 MCIS에 토이 게임 서버 배치](https://github.com/cloud-barista/cb-tumblebug/wiki/MCIS-toy-game-deployment)

##### MCIS Xonotic(3D FPS) 게임 서버 배치
  - [스크립트를 통해 MCIS에 Xonotic 게임 서버 배치](https://github.com/cloud-barista/cb-tumblebug/wiki/Deploy-Xonotic-game-sever-in-a-Cloud-via-CB-Tumblebug)


### CB-Tumblebug REST API 사용
1. CB-Tumblebug 멀티 클라우드 네임스페이스 관리 API를 통해서 Namespace 생성
   - [Namespace 생성](https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BNamespace%5D%20Namespace%20management/post_ns)
2. CB-Tumblebug 멀티 클라우드 인프라 자원(MCIR) 관리 API를 통해서 VM 생성을 위한 자원 (MCIR) 생성
   - [VM spec object 생성](https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BMCIR%5D%20Spec%20management/post_ns__nsId__resources_spec)
   - [VM image object 생성](https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BMCIR%5D%20Image%20management/post_ns__nsId__resources_image)
   - [Virtual network object 생성](https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BMCIR%5D%20Network%20management/post_ns__nsId__resources_vNet)
   - [Security group object 생성](https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BMCIR%5D%20Security%20group%20management/post_ns__nsId__resources_securityGroup)
   - [VM 접속 ssh key object 생성](https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BMCIR%5D%20Access%20key%20management/post_ns__nsId__resources_sshKey)
3. CB-Tumblebug 멀티 클라우드 인프라 서비스(MCIS) 관리 API를 통해서 MCIS 생성, 조회, 제어, 원격명령수행, 종료 및 삭제
   - [MCIS 생성](https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BMCIS%5D%20Provisioning%20management/post_ns__nsId__mcis)
   - [MCIS 원격 커맨드](https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BMCIS%5D%20Remote%20command/post_ns__nsId__cmd_mcis__mcisId_)
   - [MCIS 조회 및 제어](https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BMCIS%5D%20Provisioning%20management/get_ns__nsId__mcis__mcisId_)
   - [MCIS 삭제(MCIS 종료 상태에서만 동작 가능)](https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/src/api/rest/docs/swagger.yaml#/%5BMCIS%5D%20Provisioning%20management/delete_ns__nsId__mcis)
4. CB-Tumblebug 최적 배치 및 동적 프로비저닝
   - [CB-Tumblebug 최적 배치 및 동적 프로비저닝](https://github.com/cloud-barista/cb-tumblebug/wiki/Dynamic-and-optimal-mcis-provisioning-guide)

  
***
***

## Contributors ✨

Thanks goes to these wonderful people ([emoji key](https://allcontributors.org/docs/en/emoji-key)):
<!-- ALL-CONTRIBUTORS-LIST:START - Do not remove or modify this section -->
<!-- prettier-ignore-start -->
<!-- markdownlint-disable -->
<table>
  <tbody>
    <tr>
      <td align="center"><a href="https://seokho-son.github.io/"><img src="https://avatars3.githubusercontent.com/u/5966944?v=4?s=100" width="100px;" alt="Seokho Son"/><br /><sub><b>Seokho Son</b></sub></a><br /><a href="#maintenance-seokho-son" title="Maintenance">🚧</a> <a href="#ideas-seokho-son" title="Ideas, Planning, & Feedback">🤔</a> <a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=seokho-son" title="Code">💻</a> <a href="https://github.com/cloud-barista/cb-tumblebug/pulls?q=is%3Apr+reviewed-by%3Aseokho-son" title="Reviewed Pull Requests">👀</a></td>
      <td align="center"><a href="https://jihoon-seo.github.io"><img src="https://avatars1.githubusercontent.com/u/46767780?v=4?s=100" width="100px;" alt="Jihoon Seo"/><br /><sub><b>Jihoon Seo</b></sub></a><br /><a href="#maintenance-jihoon-seo" title="Maintenance">🚧</a> <a href="#ideas-jihoon-seo" title="Ideas, Planning, & Feedback">🤔</a> <a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=jihoon-seo" title="Code">💻</a> <a href="https://github.com/cloud-barista/cb-tumblebug/pulls?q=is%3Apr+reviewed-by%3Ajihoon-seo" title="Reviewed Pull Requests">👀</a></td>
      <td align="center"><a href="https://github.com/yunkon-kim"><img src="https://avatars2.githubusercontent.com/u/7975459?v=4?s=100" width="100px;" alt="Yunkon (Alvin) Kim "/><br /><sub><b>Yunkon (Alvin) Kim </b></sub></a><br /><a href="#ideas-yunkon-kim" title="Ideas, Planning, & Feedback">🤔</a> <a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=yunkon-kim" title="Code">💻</a> <a href="https://github.com/cloud-barista/cb-tumblebug/pulls?q=is%3Apr+reviewed-by%3Ayunkon-kim" title="Reviewed Pull Requests">👀</a></td>
      <td align="center"><a href="https://github.com/jmleefree"><img src="https://avatars3.githubusercontent.com/u/64775292?v=4?s=100" width="100px;" alt="jmleefree"/><br /><sub><b>jmleefree</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=jmleefree" title="Code">💻</a> <a href="https://github.com/cloud-barista/cb-tumblebug/pulls?q=is%3Apr+reviewed-by%3Ajmleefree" title="Reviewed Pull Requests">👀</a></td>
      <td align="center"><a href="http://www.powerkim.co.kr"><img src="https://avatars2.githubusercontent.com/u/46367962?v=4?s=100" width="100px;" alt="ByoungSeob Kim"/><br /><sub><b>ByoungSeob Kim</b></sub></a><br /><a href="#ideas-powerkimhub" title="Ideas, Planning, & Feedback">🤔</a></td>
      <td align="center"><a href="https://github.com/sykim-etri"><img src="https://avatars3.githubusercontent.com/u/25163268?v=4?s=100" width="100px;" alt="Sooyoung Kim"/><br /><sub><b>Sooyoung Kim</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/issues?q=author%3Asykim-etri" title="Bug reports">🐛</a> <a href="#ideas-sykim-etri" title="Ideas, Planning, & Feedback">🤔</a></td>
      <td align="center"><a href="https://github.com/dongjae"><img src="https://avatars.githubusercontent.com/u/5770239?v=4?s=100" width="100px;" alt="KANG DONG JAE"/><br /><sub><b>KANG DONG JAE</b></sub></a><br /><a href="#ideas-dongjae" title="Ideas, Planning, & Feedback">🤔</a></td>
    </tr>
    <tr>
      <td align="center"><a href="http://www.etri.re.kr"><img src="https://avatars.githubusercontent.com/u/5266479?v=4?s=100" width="100px;" alt="Youngwoo-Jung"/><br /><sub><b>Youngwoo-Jung</b></sub></a><br /><a href="#ideas-Youngwoo-Jung" title="Ideas, Planning, & Feedback">🤔</a></td>
      <td align="center"><a href="https://github.com/innodreamer"><img src="https://avatars.githubusercontent.com/u/51111668?v=4?s=100" width="100px;" alt="Sean Oh"/><br /><sub><b>Sean Oh</b></sub></a><br /><a href="#ideas-innodreamer" title="Ideas, Planning, & Feedback">🤔</a></td>
      <td align="center"><a href="https://github.com/MZC-CSC"><img src="https://avatars.githubusercontent.com/u/78469943?v=4?s=100" width="100px;" alt="MZC-CSC"/><br /><sub><b>MZC-CSC</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/issues?q=author%3AMZC-CSC" title="Bug reports">🐛</a> <a href="#ideas-MZC-CSC" title="Ideas, Planning, & Feedback">🤔</a></td>
      <td align="center"><a href="https://github.com/itnpeople"><img src="https://avatars.githubusercontent.com/u/35829386?v=4?s=100" width="100px;" alt="Eunsang"/><br /><sub><b>Eunsang</b></sub></a><br /><a href="#userTesting-itnpeople" title="User Testing">📓</a></td>
      <td align="center"><a href="https://github.com/hyokyungk"><img src="https://avatars.githubusercontent.com/u/51115778?v=4?s=100" width="100px;" alt="hyokyungk"/><br /><sub><b>hyokyungk</b></sub></a><br /><a href="#userTesting-hyokyungk" title="User Testing">📓</a></td>
      <td align="center"><a href="https://github.com/pjini"><img src="https://avatars.githubusercontent.com/u/64886639?v=4?s=100" width="100px;" alt="pjini"/><br /><sub><b>pjini</b></sub></a><br /><a href="#userTesting-pjini" title="User Testing">📓</a></td>
      <td align="center"><a href="https://github.com/vlatte"><img src="https://avatars.githubusercontent.com/u/21170063?v=4?s=100" width="100px;" alt="sunmi"/><br /><sub><b>sunmi</b></sub></a><br /><a href="#userTesting-vlatte" title="User Testing">📓</a></td>
    </tr>
    <tr>
      <td align="center"><a href="https://github.com/limsg1234"><img src="https://avatars.githubusercontent.com/u/53066410?v=4?s=100" width="100px;" alt="sglim"/><br /><sub><b>sglim</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=limsg1234" title="Documentation">📖</a> <a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=limsg1234" title="Code">💻</a></td>
      <td align="center"><a href="https://github.com/jangh-lee"><img src="https://avatars.githubusercontent.com/u/72970232?v=4?s=100" width="100px;" alt="jangh-lee"/><br /><sub><b>jangh-lee</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=jangh-lee" title="Documentation">📖</a> <a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=jangh-lee" title="Code">💻</a></td>
      <td align="center"><a href="https://github.com/leedohun"><img src="https://avatars.githubusercontent.com/u/33706689?v=4?s=100" width="100px;" alt="이도훈"/><br /><sub><b>이도훈</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=leedohun" title="Documentation">📖</a> <a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=leedohun" title="Code">💻</a></td>
      <td align="center"><a href="https://velog.io/@skynet"><img src="https://avatars.githubusercontent.com/u/26251856?v=4?s=100" width="100px;" alt="Park Beomsu"/><br /><sub><b>Park Beomsu</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=computerphilosopher" title="Code">💻</a></td>
      <td align="center"><a href="https://github.com/HassanAlsamahi"><img src="https://avatars.githubusercontent.com/u/42076287?v=4?s=100" width="100px;" alt="Hassan Alsamahi"/><br /><sub><b>Hassan Alsamahi</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=HassanAlsamahi" title="Code">💻</a></td>
      <td align="center"><a href="https://github.com/atg0831"><img src="https://avatars.githubusercontent.com/u/44899448?v=4?s=100" width="100px;" alt="Taegeon An"/><br /><sub><b>Taegeon An</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=atg0831" title="Code">💻</a></td>
      <td align="center"><a href="http://ihp001.tistory.com"><img src="https://avatars.githubusercontent.com/u/47745785?v=4?s=100" width="100px;" alt="INHYO"/><br /><sub><b>INHYO</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=PARKINHYO" title="Code">💻</a></td>
    </tr>
    <tr>
      <td align="center"><a href="https://github.com/Modney"><img src="https://avatars.githubusercontent.com/u/46340193?v=4?s=100" width="100px;" alt="Modney"/><br /><sub><b>Modney</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=Modney" title="Documentation">📖</a> <a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=Modney" title="Code">💻</a></td>
      <td align="center"><a href="https://github.com/ChobobDev"><img src="https://avatars.githubusercontent.com/u/32432141?v=4?s=100" width="100px;" alt="Seongbin Bernie Cho"/><br /><sub><b>Seongbin Bernie Cho</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=ChobobDev" title="Code">💻</a> <a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=ChobobDev" title="Documentation">📖</a></td>
      <td align="center"><a href="https://github.com/gbnam"><img src="https://avatars.githubusercontent.com/u/17192707?v=4?s=100" width="100px;" alt="Gibaek Nam"/><br /><sub><b>Gibaek Nam</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=gbnam" title="Code">💻</a></td>
      <td align="center"><a href="https://github.com/betelgeuse-7"><img src="https://avatars.githubusercontent.com/u/71967052?v=4?s=100" width="100px;" alt="Abidin Durdu"/><br /><sub><b>Abidin Durdu</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=betelgeuse-7" title="Code">💻</a></td>
      <td align="center"><a href="https://sysgongbu.tistory.com/"><img src="https://avatars.githubusercontent.com/u/46469385?v=4?s=100" width="100px;" alt="soyeon Park"/><br /><sub><b>soyeon Park</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=sypark9646" title="Code">💻</a></td>
      <td align="center"><a href="https://github.com/Jayita10"><img src="https://avatars.githubusercontent.com/u/85472715?v=4?s=100" width="100px;" alt="Jayita Pramanik"/><br /><sub><b>Jayita Pramanik</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=Jayita10" title="Documentation">📖</a></td>
      <td align="center"><a href="https://github.com/MukulKolpe"><img src="https://avatars.githubusercontent.com/u/78664749?v=4?s=100" width="100px;" alt="Mukul Kolpe"/><br /><sub><b>Mukul Kolpe</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=MukulKolpe" title="Documentation">📖</a></td>
    </tr>
    <tr>
      <td align="center"><a href="https://github.com/EmmanuelMarianMat"><img src="https://avatars.githubusercontent.com/u/75481347?v=4?s=100" width="100px;" alt="EmmanuelMarianMat"/><br /><sub><b>EmmanuelMarianMat</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=EmmanuelMarianMat" title="Code">💻</a></td>
      <td align="center"><a href="http://carlosfelix.pythonanywhere.com/"><img src="https://avatars.githubusercontent.com/u/18339454?v=4?s=100" width="100px;" alt="Carlos Felix"/><br /><sub><b>Carlos Felix</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=carlosfrodrigues" title="Code">💻</a></td>
      <td align="center"><a href="https://github.com/Stuie"><img src="https://avatars.githubusercontent.com/u/389169?v=4?s=100" width="100px;" alt="Stuart Gilbert"/><br /><sub><b>Stuart Gilbert</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=Stuie" title="Code">💻</a></td>
      <td align="center"><a href="https://github.com/ketan40"><img src="https://avatars.githubusercontent.com/u/15875215?v=4?s=100" width="100px;" alt="Ketan Deshmukh"/><br /><sub><b>Ketan Deshmukh</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=ketan40" title="Code">💻</a></td>
      <td align="center"><a href="https://ie.linkedin.com/in/trionabarrow"><img src="https://avatars.githubusercontent.com/u/2207006?v=4?s=100" width="100px;" alt="Tríona Barrow"/><br /><sub><b>Tríona Barrow</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=polkabunny" title="Code">💻</a></td>
      <td align="center"><a href="https://www.bambutz.dev"><img src="https://avatars.githubusercontent.com/u/7022144?v=4?s=100" width="100px;" alt="BamButz"/><br /><sub><b>BamButz</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=BamButz" title="Code">💻</a></td>
      <td align="center"><a href="https://github.com/dogfootman"><img src="https://avatars.githubusercontent.com/u/80231499?v=4?s=100" width="100px;" alt="dogfootman"/><br /><sub><b>dogfootman</b></sub></a><br /><a href="#userTesting-dogfootman" title="User Testing">📓</a></td>
    </tr>
    <tr>
      <td align="center"><a href="https://github.com/choryang"><img src="https://avatars.githubusercontent.com/u/47209678?v=4?s=100" width="100px;" alt="Okhee Lee"/><br /><sub><b>Okhee Lee</b></sub></a><br /><a href="#userTesting-choryang" title="User Testing">📓</a></td>
      <td align="center"><a href="https://github.com/joowons"><img src="https://avatars.githubusercontent.com/u/85204858?v=4?s=100" width="100px;" alt="joowon"/><br /><sub><b>joowon</b></sub></a><br /><a href="#userTesting-joowons" title="User Testing">📓</a></td>
      <td align="center"><a href="https://github.com/bconfiden2"><img src="https://avatars.githubusercontent.com/u/58922834?v=4?s=100" width="100px;" alt="Sanghong Kim"/><br /><sub><b>Sanghong Kim</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=bconfiden2" title="Code">💻</a></td>
      <td align="center"><a href="https://github.com/Rohit-R2000"><img src="https://avatars.githubusercontent.com/u/83547290?v=4?s=100" width="100px;" alt="Rohit Rajput"/><br /><sub><b>Rohit Rajput</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=Rohit-R2000" title="Code">💻</a></td>
      <td align="center"><a href="https://github.com/arshad-k7"><img src="https://avatars.githubusercontent.com/u/49522121?v=4?s=100" width="100px;" alt="Arshad"/><br /><sub><b>Arshad</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=arshad-k7" title="Code">💻</a></td>
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


***
***

## License
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fcloud-barista%2Fcb-tumblebug.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2Fcloud-barista%2Fcb-tumblebug?ref=badge_large)


***
