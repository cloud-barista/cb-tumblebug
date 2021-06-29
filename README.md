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
[![All Contributors](https://img.shields.io/badge/all_contributors-14-orange.svg?style=flat-square)](#contributors-)
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

## 목차

1. [CB-Tumblebug 실행 및 개발 환경](#cb-tumblebug-실행-및-개발-환경)
2. [CB-Tumblebug 실행 방법](#cb-tumblebug-실행-방법)
3. [CB-Tumblebug 소스 빌드 및 실행 방법 상세](#cb-tumblebug-소스-빌드-및-실행-방법-상세)
4. [CB-Tumblebug 기능 사용 방법](#cb-tumblebug-기능-사용-방법)

***
***

## CB-Tumblebug 실행 및 개발 환경
- Linux (추천: Ubuntu v18.04)
- Go (추천: v1.16)

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

```
# docker run -p 1323:1323 -p 50252:50252 \
-v /root/go/src/github.com/cloud-barista/cb-tumblebug/meta_db:/app/meta_db \
--name cb-tumblebug \
cloudbaristaorg/cb-tumblebug:0.3.xx
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
    - `# apt update`
    - `# apt install make gcc git`

  - Go 설치
    - https://golang.org/dl/ 를 참고하여 Go 설치 (버전 v1.16 이상: 추천 개발 환경)
    - 설치 예시
      - `wget https://golang.org/dl/go1.16.4.linux-amd64.tar.gz`
      - `tar -C /usr/local -xzf go1.16.4.linux-amd64.tar.gz`
      - `.bashrc` 파일 하단에 다음을 추가 
      ```
      export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin
      export GOPATH=$HOME/go
      ```
      - `source ~/.bashrc` (`.bashrc` 변경 내용을 적용)

- CB-Tumblebug 소스 다운로드
  - `# git clone https://github.com/cloud-barista/cb-tumblebug.git $HOME/go/src/github.com/cloud-barista/cb-tumblebug`

- CB-Tumblebug 실행에 필요한 환경변수 설정
  - `cb-tumblebug/conf/setup.env` 내용 확인 및 설정 (CB-Tumblebug 환경변수, 필요에 따라 변경)
    - `source setup.env` 실행으로 시스템에 반영
  - `cb-tumblebug/conf` 의 `store_conf.yaml` 내용 확인 및 설정 (cb-store 환경변수, 필요에 따라 변경)
    - storetype 지정 (NUTSDB 또는 ETCD 지정)
    - NUTSDB(local DB) 설정시 DB 데이터가 포함된 주소 지정이 필요 (기본은 `cb-tumblebug/meta_db/dat` 에 파일로 추가됨)
  - `cb-tumblebug/conf` 의 `log_conf.yaml` 내용 확인 및 설정 (cb-log 환경변수, 필요에 따라 변경)


### (2) CB-Tumblebug 빌드

- 빌드 명령어
```Shell
# cd ~/go/src/github.com/cloud-barista/cb-tumblebug/src
# export GO111MODULE=on
# ./make
```

- Swagger API 문서 업데이트 필요 시 `cb-tumblebug/src/` 에서 `make swag` 실행
  - API 문서 파일은 `cb-tumblebug/src/api/rest/docs/swagger.yaml` 에 생성됨
  - 해당 API 문서는 http://localhost:1323/tumblebug/swagger/index.html 로컬에서 웹브라우저로 확인 가능 (CB-Tumblebug 구동 시 자동으로 제공)

### (3) CB-Tumblebug 실행

- `# ./make run` (또는 `# go run *.go`) 

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
   http://xxx.xxx.xxx.xxx:1323/tumblebug/swagger/index.html?url=http://xxx.xxx.xxx.xxx:1323/tumblebug/swaggerActive

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
  ```Shell
  # rm -rf $GOPATH/src/go.etcd.io/etcd/vendor/golang.org/x/net/trace
  # ./make
  ```

***
***

## CB-Tumblebug 기능 사용 방법

### (1) CB-Tumblebug 및 CB-Spider의 REST API를 사용하여 테스트
- CB-Spider API를 통해 클라우드 인프라 연동 정보 등록
   - https://cloud-barista.github.io/rest-api/v0.3.0/spider/ccim/
- CB-Tumblebug 멀티 클라우드 네임스페이스 관리 API를 통해서 Namespace 생성
   - https://cloud-barista.github.io/cb-tumblebug-api-web/#/Namespace/post_ns
- CB-Tumblebug 멀티 클라우드 인프라 자원(MCIR) 관리 API를 통해서 VM 생성을 위한 자원 (MCIR) 생성
   - https://cloud-barista.github.io/cb-tumblebug-api-web/#/VNet/post_ns__nsId__resources_vNet
   - https://cloud-barista.github.io/cb-tumblebug-api-web/#/SSH%20Key/post_ns__nsId__resources_sshKey
   - https://cloud-barista.github.io/cb-tumblebug-api-web/#/Security%20Group
- CB-Tumblebug 멀티 클라우드 인프라 서비스(MCIS) 관리 API를 통해서 MCIS 생성, 조회, 제어, 원격명령수행, 종료 및 삭제
   - https://cloud-barista.github.io/cb-tumblebug-api-web/#/MCIS/post_ns__nsId__mcis
   - https://cloud-barista.github.io/cb-tumblebug-api-web/#/MCIS/get_ns__nsId__mcis
   - https://cloud-barista.github.io/cb-tumblebug-api-web/#/MCIS/post_ns__nsId__cmd_mcis__mcisId_
   - https://cloud-barista.github.io/cb-tumblebug-api-web/#/MCIS/delete_ns__nsId__mcis__mcisId_

### (2) CB-Tumblebug 스크립트를 통한 테스트
- `src/testclient/scripts/`
   - 클라우드 인증 정보, 테스트 기본 정보 입력
   - 클라우드정보, Namespace, MCIR, MCIS 등 개별 제어 시험 (개별 시험시, 오브젝트들의 의존성 고려 필요))
   - 한꺼번에 통합 시험 (추천 테스트 방법)
     - `src/testclient/scripts/sequentialFullTest/`

#### 0) 클라우드 인증 정보, 테스트 기본 정보 입력
- `src/testclient/scripts/` 이동
- [`credentials.conf.example`](https://github.com/cloud-barista/cb-tumblebug/blob/main/src/testclient/scripts/credentials.conf.example)을 복사하여 `credentials.conf` 를 생성하고, `credentials.conf` 항목에 사용자의 클라우드 인증 정보 입력
   - `credentials.conf` 는 기본적인 클라우드 타입 (AWS, GCP, AZURE, ALIBABA 등)에 대해 인증 정보 템플릿 제공
   - [CSP별 인증 정보 획득 방법 참고](https://github.com/cloud-barista/cb-tumblebug/wiki/How-to-get-public-cloud-credentials)
- `conf.env` 설정
   - CB-Spider 및 CB-Tumblebug 서버 엔드포인트, 클라우드 리젼, 테스트용 이미지명, 테스트용 스팩명 등 테스트 기본 정보 제공
   - 이미 많은 클라우드 타입에 대한 정보가 조사 및 입력되어 있으므로, 특별한 경우가 아니면 수정없이 사용 가능. 

#### 1) 클라우드정보, Namespace, MCIR, MCIS 등 개별 제어 시험
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

#### 2) 한꺼번에 통합 시험 (추천 테스트 방법)
- `src/testclient/scripts/sequentialFullTest/` 에 포함된 `create-all.sh` 및 `clean-all.sh` 을 수행하면 모든 것을 한번에 테스트 가능
```
└── sequentialFullTest  # Cloud 정보 등록, NS 생성, MCIR 생성, MCIS 생성까지 한번에 자동 테스트
    ├── create-all.sh  # Cloud 정보 등록, NS 생성, MCIR 생성, MCIS 생성까지 한번에 자동 테스트
    ├── gen-sshKey.sh  # 수행이 진행된 테스트 로그 (MCIS에 접속 가능한 SSH키 파일 생성)  
    ├── command-mcis.sh  # 생성된 MCIS(다중VM)에 원격 명령 수행
    ├── deploy-nginx-mcis.sh  # 생성된 MCIS(다중VM)에 Nginx 자동 배포  
    ├── create-mcis-for-df.sh  # CB-Dragonfly 호스팅을 위한 MCIS 생성        
    ├── deploy-dragonfly-docker.sh  # MCIS에 CB-Dragonfly 자동 배포 및 환경 자동 설정      
    ├── clean-all.sh  # 모든 오브젝트를 생성의 역순으로 삭제
    └── executionStatus  # 수행이 진행된 테스트 로그 (testAll 수행시 정보가 추가되며, cleanAll 수행시 정보가 제거됨. 진행중인 작업 확인 가능)

```
- 사용 예시
  - 생성 테스트
    - `./create-all.sh aws 1 shson`       # aws의 1번 리전에 shson이라는 개발자명으로 생성 수행
    - `./create-all.sh aws 2 shson`       # aws의 2번 리전에 shson이라는 개발자명으로 생성 수행
    - `./create-all.sh aws 3 shson`       # aws의 3번 리전에 shson이라는 개발자명으로 생성 수행
    - `./create-all.sh gcp 1 shson`       # gcp의 1번 리전에 shson이라는 개발자명으로 생성 수행
    - `./create-all.sh gcp 2 shson`       # gcp의 2번 리전에 shson이라는 개발자명으로 생성 수행
    - `./create-all.sh azure 1 shson`     # azure의 1번 리전에 shson이라는 개발자명으로 생성 수행
    - `./create-all.sh alibaba 1 shson`   # alibaba의 1번 리전에 shson이라는 개발자명으로 생성 수행
    - `./create-all.sh all 1 shson ../testSetCustom.env`   # ../testSetCustom.env 에 구성된 클라우드 조합으로 MCIS 생성 수행 (`all 1` all 선택시 리전은 의미가 없으며, 임의로 지정하면 됨. 향후 개선 예정)
  - 제거 테스트 (이미 수행이 진행된 클라우드타입/리전/개발자명 으로만 삭제 진행이 필요)
    - `./clean-all.sh aws 1 shson`       # aws의 1번 리전에 shson이라는 개발자명으로 제거 수행
    - `./clean-all.sh aws 2 shson`       # aws의 2번 리전에 shson이라는 개발자명으로 제거 수행
    - `./clean-all.sh aws 3 shson`       # aws의 3번 리전에 shson이라는 개발자명으로 제거 수행
    - `./clean-all.sh gcp 1 shson`       # gcp의 1번 리전에 shson이라는 개발자명으로 제거 수행
    - `./clean-all.sh gcp 2 shson`       # gcp의 2번 리전에 shson이라는 개발자명으로 제거 수행
    - `./clean-all.sh azure 1 shson`     # azure의 1번 리전에 shson이라는 개발자명으로 제거 수행
    - `./clean-all.sh alibaba 1 shson`   # alibaba의 1번 리전에 shson이라는 개발자명으로 제거 수행
    - `./create-all.sh all 1 shson ../testSetCustom.env`   # ../testSetCustom.env 에 구성된 클라우드 조합으로 제거 수행

<details>
<summary>입출력 예시 보기</summary>

```
~/go/src/github.com/cloud-barista/cb-tumblebug/src/testclient/scripts/sequentialFullTest$ ./create-all.sh aws 1 shson
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
   "privateKey" : "-----BEGIN RSA PRIVATE KEY-----\nMIIEogIBAAKCAQEAopGlO3dUrB4AMcBr4XZg0OVrveecA9Hv0/a9GmxgXU5dx42YV4DwW7oq/+Dq\nPaCSXvGGtdVHuL0hoOKdGYOx89qzi+nUgNQup+pKLbQw4aU2gVbV1/3/ejt7tYRUeWNU9c4b7m7E\nfs3A0xgfmak90eoQen+TJYhkfdWcSwkmJSH61bEFRbdeyijEODCu0TAGDrtRZzdCRUzbA/N7FjsC\ns0a1C...LpszE9J0bfhLOqgmkYNGSw4oR+gPRIsipUK6SzaRH7nFnOSw=\n-----END RSA PRIVATE KEY-----",
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

#### 3) 멀티 클라우드 인프라 유스케이스

##### MCIS SSH 원격 커맨드
  - SSH 원격 커맨드 실행을 통해서 접속 여부 등을 확인 가능
    - command-mcis.sh  # 생성된 MCIS(다중VM)에 원격 명령 수행
    - 실행 예시
      - `./create-all.sh all 1 shson ../testSet.env`  # testSet.env 에 구성된 정보를 기준으로 MCIS 생성
      - `./command-mcis.sh all 1 shson ../testSet.env`  # MCIS의 모든 VM에 IP 및 Hostname 조회를 수행

##### MCIS SSH 접속키 생성 및 접속
  - 스크립트를 통해 MCIS의 VM에 각각 접속할 수 있는 SSH Key 및 주소를 생성
    - gen-sshKey.sh  # MCIS에 구성된 모든 VM의 접속키 리턴
    - 실행 예시
      - `./create-all.sh all 1 shson ../testSetAws.env`  # testSetAws.env 에 구성된 정보를 기준으로 MCIS 생성
      - `./gen-sshKey.sh all 1 shson ../testSetAws.env` # MCIS에 구성된 모든 VM의 접속키 리턴 
        ```
        son@son:~/go/src/github.com/cloud-barista/cb-tumblebug/src/testclient/scripts/sequentialFullTest$ ./gen-sshKey.sh all 1 shson ../testSetAws.env 
        ####################################################################
        ## Generate SSH KEY (PEM, PPK)
        ####################################################################
        ...
        [GENERATED PRIVATE KEY (PEM, PPK)]
        [MCIS INFO: mc-shson]
         [VMIP]: 13.212.254.59   [MCISID]: mc-shson   [VMID]: aws-ap-se-1-0
         ./sshkey-tmp/aws-ap-se-1-shson.pem 
         ./sshkey-tmp/aws-ap-se-1-shson.ppk
         [VMIP]: 54.177.115.174   [MCISID]: mc-shson   [VMID]: aws-us-west-1-0
         ./sshkey-tmp/aws-us-west-1-shson.pem 
         ./sshkey-tmp/aws-us-west-1-shson.ppk
         [VMIP]: 35.182.30.37   [MCISID]: mc-shson   [VMID]: aws-ca-ct-1-0
         ./sshkey-tmp/aws-ca-ct-1-shson.pem 
         ./sshkey-tmp/aws-ca-ct-1-shson.ppk

        [SSH COMMAND EXAMPLE]
         [VMIP]: 13.212.254.59   [MCISID]: mc-shson   [VMID]: aws-ap-se-1-0
         ssh -i ./sshkey-tmp/aws-ap-se-1-shson.pem cb-user@13.212.254.59 -o StrictHostKeyChecking=no
         [VMIP]: 54.177.115.174   [MCISID]: mc-shson   [VMID]: aws-us-west-1-0
         ssh -i ./sshkey-tmp/aws-us-west-1-shson.pem cb-user@54.177.115.174 -o StrictHostKeyChecking=no
         [VMIP]: 35.182.30.37   [MCISID]: mc-shson   [VMID]: aws-ca-ct-1-0
         ssh -i ./sshkey-tmp/aws-ca-ct-1-shson.pem cb-user@35.182.30.37 -o StrictHostKeyChecking=no
        ```


##### MCIS Nginx 분산 배치
- Nginx를 분산 배치하여, 웹서버 접속 시험 가능
    - deploy-nginx-mcis.sh  # 생성된 MCIS(다중VM)에 Nginx 자동 배포
    - 실행 예시
      - command-mcis.sh aws 1 shson # aws의 1번 리전에 배치된 MCIS의 모든 VM에 Nginx 및 웹페이지 설치 (접속 테스트 가능)
        ```
        ~/go/src/github.com/cloud-barista/cb-tumblebug/src/testclient/scripts/sequentialFullTest$ ./deploy-nginx-mcis.sh aws 1 shson
        {
          "result_array" : [
              {
                "vmIp" : "35.173.215.4",
                "vmId" : "aws-us-east-1-shson-01",
                "result" : "WebServer is ready. Access http://35.173.215.4",
                "mcisId" : "aws-us-east-1-shson"
              },
              {
                "vmIp" : "18.206.13.233",
                "vmId" : "aws-us-east-1-shson-02",
                "result" : "WebServer is ready. Access http://18.206.13.233",
                "mcisId" : "aws-us-east-1-shson"
              },
              {
                "mcisId" : "aws-us-east-1-shson",
                "result" : "WebServer is ready. Access http://18.232.53.134",
                "vmId" : "aws-us-east-1-shson-03",
                "vmIp" : "18.232.53.134"
              }
          ]
        }
        ```

##### MCIS Weave Scope 클러스터 모니터링 분산 배치
  - [스크립트를 통해 MCIS에 Weave Scope 클러스터 설치](https://github.com/cloud-barista/cb-tumblebug/wiki/MCIS-WeaveScope-deployment)

##### MCIS Jitsi 영상 회의 배치
  - [스크립트를 통해 MCIS에 Jitsi 영상회의 설치](https://github.com/cloud-barista/cb-tumblebug/wiki/MCIS-Jitsi-deployment)

##### MCIS Ansible 실행 환경 자동 구성
  - [스크립트를 통해 MCIS에 Ansible 실행 환경 자동 구성](https://github.com/cloud-barista/cb-tumblebug/wiki/MCIS-Ansible-deployment)

##### MCIS 토이 게임 서버 배치
  - [스크립트를 통해 MCIS에 토이 게임 서버 설치](https://github.com/cloud-barista/cb-tumblebug/wiki/MCIS-toy-game-deployment)


***
***

## Contributors ✨

Thanks goes to these wonderful people ([emoji key](https://allcontributors.org/docs/en/emoji-key)):
<!-- ALL-CONTRIBUTORS-LIST:START - Do not remove or modify this section -->
<!-- prettier-ignore-start -->
<!-- markdownlint-disable -->
<table>
  <tr>
    <td align="center"><a href="https://jihoon-seo.github.io"><img src="https://avatars1.githubusercontent.com/u/46767780?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Jihoon Seo</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=jihoon-seo" title="Code">💻</a> <a href="https://github.com/cloud-barista/cb-tumblebug/pulls?q=is%3Apr+reviewed-by%3Ajihoon-seo" title="Reviewed Pull Requests">👀</a></td>
    <td align="center"><a href="https://github.com/hermitkim1"><img src="https://avatars2.githubusercontent.com/u/7975459?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Yunkon (Alvin) Kim </b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=hermitkim1" title="Code">💻</a> <a href="https://github.com/cloud-barista/cb-tumblebug/pulls?q=is%3Apr+reviewed-by%3Ahermitkim1" title="Reviewed Pull Requests">👀</a></td>
    <td align="center"><a href="https://seokho-son.github.io/"><img src="https://avatars3.githubusercontent.com/u/5966944?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Seokho Son</b></sub></a><br /><a href="#maintenance-seokho-son" title="Maintenance">🚧</a></td>
    <td align="center"><a href="https://github.com/jmleefree"><img src="https://avatars3.githubusercontent.com/u/64775292?v=4?s=100" width="100px;" alt=""/><br /><sub><b>jmleefree</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/commits?author=jmleefree" title="Code">💻</a></td>
    <td align="center"><a href="http://www.powerkim.co.kr"><img src="https://avatars2.githubusercontent.com/u/46367962?v=4?s=100" width="100px;" alt=""/><br /><sub><b>ByoungSeob Kim</b></sub></a><br /><a href="#ideas-powerkimhub" title="Ideas, Planning, & Feedback">🤔</a></td>
    <td align="center"><a href="https://github.com/sykim-etri"><img src="https://avatars3.githubusercontent.com/u/25163268?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Sooyoung Kim</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/issues?q=author%3Asykim-etri" title="Bug reports">🐛</a></td>
    <td align="center"><a href="https://github.com/dongjae"><img src="https://avatars.githubusercontent.com/u/5770239?v=4?s=100" width="100px;" alt=""/><br /><sub><b>KANG DONG JAE</b></sub></a><br /><a href="#ideas-dongjae" title="Ideas, Planning, & Feedback">🤔</a></td>
  </tr>
  <tr>
    <td align="center"><a href="http://www.etri.re.kr"><img src="https://avatars.githubusercontent.com/u/5266479?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Youngwoo-Jung</b></sub></a><br /><a href="#ideas-Youngwoo-Jung" title="Ideas, Planning, & Feedback">🤔</a></td>
    <td align="center"><a href="https://github.com/innodreamer"><img src="https://avatars.githubusercontent.com/u/51111668?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Sean Oh</b></sub></a><br /><a href="#ideas-innodreamer" title="Ideas, Planning, & Feedback">🤔</a></td>
    <td align="center"><a href="https://github.com/MZC-CSC"><img src="https://avatars.githubusercontent.com/u/78469943?v=4?s=100" width="100px;" alt=""/><br /><sub><b>MZC-CSC</b></sub></a><br /><a href="https://github.com/cloud-barista/cb-tumblebug/issues?q=author%3AMZC-CSC" title="Bug reports">🐛</a></td>
    <td align="center"><a href="https://github.com/itnpeople"><img src="https://avatars.githubusercontent.com/u/35829386?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Eunsang</b></sub></a><br /><a href="#userTesting-itnpeople" title="User Testing">📓</a></td>
    <td align="center"><a href="https://github.com/hyokyungk"><img src="https://avatars.githubusercontent.com/u/51115778?v=4?s=100" width="100px;" alt=""/><br /><sub><b>hyokyungk</b></sub></a><br /><a href="#userTesting-hyokyungk" title="User Testing">📓</a></td>
    <td align="center"><a href="https://github.com/pjini"><img src="https://avatars.githubusercontent.com/u/64886639?v=4?s=100" width="100px;" alt=""/><br /><sub><b>pjini</b></sub></a><br /><a href="#userTesting-pjini" title="User Testing">📓</a></td>
    <td align="center"><a href="https://github.com/vlatte"><img src="https://avatars.githubusercontent.com/u/21170063?v=4?s=100" width="100px;" alt=""/><br /><sub><b>sunmi</b></sub></a><br /><a href="#userTesting-vlatte" title="User Testing">📓</a></td>
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
