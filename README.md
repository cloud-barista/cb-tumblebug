# CB-Tumblebug (Multi-Cloud Infra Service Management)

[![Go Report Card](https://goreportcard.com/badge/github.com/cloud-barista/cb-tumblebug)](https://goreportcard.com/report/github.com/cloud-barista/cb-tumblebug)
![Top Language](https://img.shields.io/github/languages/top/cloud-barista/cb-tumblebug) 
![Repo Size](https://img.shields.io/github/repo-size/cloud-barista/cb-tumblebug)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/cloud-barista/cb-tumblebug?label=go.mod)
![Release Version](https://img.shields.io/github/v/release/cloud-barista/cb-tumblebug)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/cloud-barista/cb-tumblebug/blob/master/LICENSE)

<!-- ALL-CONTRIBUTORS-BADGE:START - Do not remove or modify this section -->
[![All Contributors](https://img.shields.io/badge/all_contributors-9-orange.svg?style=flat-square)](#contributors-)
<!-- ALL-CONTRIBUTORS-BADGE:END -->

A sub-system of Cloud-Barista Platform to Deploy and Manage Multi-Cloud Infrastructure.

```
[NOTE] Development stage of CB-Tumblebug (CB-Tumblebug 개발 단계)
CB-Tumblebug is currently under development. (the latest release is v0.3.0)
So, we do not recommend using the current release in production.
Please note that the functionalities of CB-Tumblebug are not stable and secure yet.
If you have any difficulties in using CB-Tumblebug, please let us know.
(Open an issue or Join the cloud-barista Slack)
```

```
[NOTE] Localization and Globalization of CB-Tumblebug (CB-Tumblebug의 현지화 및 세계화)
As an opensource project initiated by Korean members, 
we would like to promote participation of Korean contributors during initial stage of this project. 
So, CB-Tumblebug Repo will accept use of Korean language in its early stages.
On the other hand, we hope this project flourishes regardless of contributor's country eventually.
So, the maintainers recommend using English at least for the title of Issues, Pull Requests, and Commits, 
while CB-Tumblebug Repo accommodates local languages in the contents of them.

CB-Tumblebug은 한국에서 시작된 오픈 소스 프로젝트로서 
프로젝트의 초기 단계에는 한국 기여자들의 참여를 촉진하고자 합니다. 
따라서 초기 단계의 CB-Tumblebug는 한국어 사용을 받아 들일 것입니다.
다른 한편으로, 이 프로젝트가 국가에 관계없이 번성하기를 희망합니다.
따라서 개발 히스토리 관리를 위해 이슈, 풀 요청, 커밋 등의 
제목에 대해서는 영어 사용을 권장하며, 내용에 대한 한국어 사용은 수용할 것입니다.
```

***

## [목차]

1. [실행 환경](#실행-환경)
2. [실행 방법](#실행-방법)
3. [소스 기반 설치 & 실행 상세 정보](#소스-기반-설치--실행-상세-정보)

***

## [실행 환경]
- Linux (검증시험: Ubuntu 18.04)

## [실행 방법]

### (1) 컨테이너 기반 실행
- CB-Tumblebug 이미지 확인(https://hub.docker.com/r/cloudbaristaorg/cb-tumblebug/tags)
- CB-Tumblebug 컨테이너 실행

```
# docker run -p 1323:1323 \
-v /root/go/src/github.com/cloud-barista/cb-tumblebug/meta_db:/app/meta_db \
--name cb-tumblebug \
cloudbaristaorg/cb-tumblebug:v0.3.x-yyyymmdd
```

### (2) 소스 기반 실행

- Go 설치 & Git 설치
- 환경 변수 설정
- CB-Tumblebug 소스 다운로드 (Git clone CB-Tumblebug)
- 의존 라이브러리 다운로드
  - Cloud-Barista alliance 설치 (CB-Store, CB-Log, CB-Spider)
  - 기타 라이브러리
- CB-Tumblebug 빌드 (make) 및 실행 (make run)
  - `cb-tumblebug/src/` 에서 수행

- Swagger API 문서 업데이트 필요시 `~/go/bin/swag init` 실행
  - API 문서 파일은 `cb-tumblebug/src/docs/swagger.yaml` 에 생성됨
  - 해당 API 문서는 http://localhost:1323/tumblebug/swagger/index.html 로컬에서 웹브라우저로 확인 가능 (cb-tumblebug 구동 시 자동으로 제공)

### (3) Cloud-Barista 시스템 통합 실행 참고 (cb-operator)
```
https://github.com/cloud-barista/cb-operator 를 통해 Cloud-Barista 전체 FW를 통합 실행할 수 있음

$ git clone https://github.com/cloud-barista/cb-operator.git
$ cd cb-operator/src
cb-operator/src$ make
cb-operator/src$ ./operator
```

## [소스 기반 설치 & 실행 상세 정보]

- Git 설치
  - `# apt update`
  - `# apt install git`

- Go 설치
  - https://golang.org/doc/install (아래 링크를 통해 1.12 이상 버전으로 설치 필요)
  - `wget https://dl.google.com/go/go1.13.4.linux-amd64.tar.gz`
  - `tar -C /usr/local -xzf go1.13.4.linux-amd64.tar.gz`
  - `.bashrc` 파일 하단에 다음을 추가: 
  ```
  export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin
  export GOPATH=$HOME/go
  ```


- `.bashrc` 에 기재한 내용을 적용하기 위해, 다음 중 하나를 수행
  - bash 재기동
  - `source ~/.bashrc`
  - `. ~/.bashrc`

- CB-Tumblebug 소스 다운로드
  - `# go get -v github.com/cloud-barista/cb-tumblebug`

- CB-Tumblebug 실행에 필요한 환경변수 설정
  - `source setup.env` (cb-tumblebug/conf 에 setup.env)
  - cb-tumblebug/conf 에 store_conf.yaml 내용 확인 및 설정 (CB-Store 설정)
    - storetype 지정 (NUTSDB 또는 ETCD 지정)
    - NUTSDB(local DB) 설정시 DB 데이터가 포함된 주소 지정이 필요 (기본은 cb-tumblebug/meta_db/dat 에 파일로 추가됨)
  - cb-tumblebug/conf 에 log_conf.yaml 내용 확인 및 설정 (CB-Log 설정)


### CB-Tumblebug 빌드

```Shell
# cd ~/go/src/github.com/cloud-barista/cb-tumblebug/src
# export GO111MODULE=on
# ./make
```


### CB-Tumblebug 실행

- `# ./make run` (또는 `# go run *.go`)
  - CB-Tumblebug API server가 실행됨

``` 
[NOTE]
“panic: /debug/requests is already registered. 
You may have two independent copies of golang.org/x/net/trace in your binary, 
trying to maintain separate state. 
This may involve a vendored copy of golang.org/x/net/trace.”

에러 발생 시, 다음을 실행하여 해결
```

```Shell
# rm -rf $GOPATH/src/go.etcd.io/etcd/vendor/golang.org/x/net/trace
# ./make
```

## [활용 예시]

### CB-Tumblebug 및 CB-Spider의 REST API를 사용하여 테스트
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

### CB-Tumblebug 스크립트를 통한 테스트 개요
- cloud-barista/cb-tumblebug/test/official/
   - 클라우드 인증 정보, 테스트 기본 정보 입력
   - 클라우드정보, Namespace, MCIR, MCIS 등 개별 제어 시험 (개별 시험시, 오브젝트들의 의존성 고려 필요))
   - 한꺼번에 통합 시험 (추천 테스트 방법)
     - cloud-barista/cb-tumblebug/test/official/sequentialFullTest

#### 0) 클라우드 인증 정보, 테스트 기본 정보 입력
- cloud-barista/cb-tumblebug/test/official/ 이동
- credentials.conf  # Cloud 정보 등록을 위한 CSP별 인증정보 (사용자에 맞게 수정 필요)
   - 기본적인 클라우드 타입 (AWS, GCP, AZURE, ALIBABA)에 대해 템플릿 제공
- conf.env  # CB-Spider 및 Tumblebug 서버 위치, 클라우드 리젼, 테스트용 이미지명, 테스트용 스팩명 등 테스트 기본 정보 제공
   - 특별한 상황이 아니면 수정이 불필요함. (CB-Spider와 CB-TB의 위치가 localhost가 아닌 경우 수정 필요)
   - 클라우드 타입(CSP)별 약 1~3개의 기본 리전이 입력되어 있음
     - 이미지와 스팩은 리전에 의존성이 있는 경우가 많으므로, 리전별로 지정이 필요

#### 1) 클라우드정보, Namespace, MCIR, MCIS 등 개별 제어 시험
- 제어하고 싶은 리소스 오브젝트에 대해, 해당 디렉토리로 이동하여 필요한 시험 수행
  - 오브젝트는 서로 의존성이 있으므로, 번호를 참고하여 오름차순으로 수행하는 것이 바람직함
    - 1.configureSpider  # 클라우드 정보 등록 관련 스크립트 모음
    - 2.configureTumblebug  # 네임스페이스 및 동적 환경 설정 관련 스크립트 모음
    - 3.vNet  # MCIR vNet 생성 관련 스크립트 모음
    - 4.securityGroup  # MCIR securityGroup 생성 관련 스크립트 모음
    - 5.sshKey  # MCIR sshKey 생성 관련 스크립트 모음
    - 6.image  # MCIR image 등록 관련 스크립트 모음
    - 7.spec  # MCIR spec 등록 관련 스크립트 모음
    - 8.mcis  # MCIS 생성 및 제어, MCIS 원격 커맨드 등 스크립트 모음

#### 2) 한꺼번에 통합 시험 (추천 테스트 방법)
- sequentialFullTest 에 포함된 cleanAll-mcis-mcir-ns-cloud.sh 을 수행하면 모든 것을 한번에 테스트 가능
```
└── sequentialFullTest  # Cloud 정보 등록, NS 생성, MCIR 생성, MCIS 생성까지 한번에 자동 테스트
    ├── testAll-mcis-mcir-ns-cloud.sh  # Cloud 정보 등록, NS 생성, MCIR 생성, MCIS 생성까지 한번에 자동 테스트
    ├── gen-sshKey.sh  # 수행이 진행된 테스트 로그 (MCIS에 접속 가능한 SSH키 파일 생성)  
    ├── command-mcis.sh  # 생성된 MCIS(다중VM)에 원격 명령 수행
    ├── deploy-nginx-mcis.sh  # 생성된 MCIS(다중VM)에 Nginx 자동 배포  
    ├── create-mcis-for-df.sh  # CB-Dragonfly 호스팅을 위한 MCIS 생성        
    ├── deploy-dragonfly-docker.sh  # MCIS에 CB-Dragonfly 자동 배포 및 환경 자동 설정      
    ├── cleanAll-mcis-mcir-ns-cloud.sh  # 모든 오브젝트를 생성의 역순으로 삭제
    └── executionStatus  # 수행이 진행된 테스트 로그 (testAll 수행시 정보가 추가되며, cleanAll 수행시 정보가 제거됨. 진행중인 작업 확인 가능)

```
- 사용 예시
  - 생성 테스트
    - ./testAll-mcis-mcir-ns-cloud.sh aws 1 shson       # aws의 1번 리전에 shson이라는 개발자명으로 테스트 수행
    - ./testAll-mcis-mcir-ns-cloud.sh aws 2 shson       # aws의 2번 리전에 shson이라는 개발자명으로 테스트 수행
    - ./testAll-mcis-mcir-ns-cloud.sh aws 3 shson       # aws의 3번 리전에 shson이라는 개발자명으로 테스트 수행
    - ./testAll-mcis-mcir-ns-cloud.sh gcp 1 shson       # gcp의 1번 리전에 shson이라는 개발자명으로 테스트 수행
    - ./testAll-mcis-mcir-ns-cloud.sh gcp 2 shson       # gcp의 2번 리전에 shson이라는 개발자명으로 테스트 수행
    - ./testAll-mcis-mcir-ns-cloud.sh azure 1 shson     # azure의 1번 리전에 shson이라는 개발자명으로 테스트 수행
    - ./testAll-mcis-mcir-ns-cloud.sh alibaba 1 shson   # alibaba의 1번 리전에 shson이라는 개발자명으로 테스트 수행
  - 제거 테스트 (이미 수행이 진행된 클라우드타입/리전/개발자명 으로만 삭제 진행이 필요)
    - ./cleanAll-mcis-mcir-ns-cloud.sh aws 1 shson       # aws의 1번 리전에 shson이라는 개발자명으로 제거 테스트 수행
    - ./cleanAll-mcis-mcir-ns-cloud.sh aws 2 shson       # aws의 2번 리전에 shson이라는 개발자명으로 제거 테스트 수행
    - ./cleanAll-mcis-mcir-ns-cloud.sh aws 3 shson       # aws의 3번 리전에 shson이라는 개발자명으로 제거 테스트 수행
    - ./cleanAll-mcis-mcir-ns-cloud.sh gcp 1 shson       # gcp의 1번 리전에 shson이라는 개발자명으로 제거 테스트 수행
    - ./cleanAll-mcis-mcir-ns-cloud.sh gcp 2 shson       # gcp의 2번 리전에 shson이라는 개발자명으로 제거 테스트 수행
    - ./cleanAll-mcis-mcir-ns-cloud.sh azure 1 shson     # azure의 1번 리전에 shson이라는 개발자명으로 제거 테스트 수행
    - ./cleanAll-mcis-mcir-ns-cloud.sh alibaba 1 shson   # alibaba의 1번 리전에 shson이라는 개발자명으로 제거 테스트 수행

<details>
<summary>입출력 예시 보기</summary>

```
~/go/src/github.com/cloud-barista/cb-tumblebug/test/official/sequentialFullTest$ ./testAll-mcis-mcir-ns-cloud.sh aws 1 shson
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
   "placement_algo" : "",
   "vm" : [
      {
         "vmUserId" : "",
         "targetStatus" : "None",
         "subnet_id" : "aws-us-east-1-shson",
         "location" : {
            "nativeRegion" : "us-east-1",
            "cloudType" : "aws",
            "latitude" : "38.1300",
            "briefAddr" : "Virginia",
            "longitude" : "-78.4500"
         },
         "vm_access_id" : "",
         "region" : {
            "Region" : "us-east-1",
            "Zone" : "us-east-1f"
         },
         "image_id" : "aws-us-east-1-shson",
         "privateDNS" : "ip-192-168-1-108.ec2.internal",
         "vmBootDisk" : "/dev/sda1",
         "status" : "Running",
         "security_group_ids" : [
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
         "vnet_id" : "aws-us-east-1-shson",
         "ssh_key_id" : "aws-us-east-1-shson",
         "privateIP" : "192.168.1.108",
         "config_name" : "aws-us-east-1",
         "vmBlockDisk" : "/dev/sda1",
         "targetAction" : "None",
         "description" : "description",
         "spec_id" : "aws-us-east-1-shson",
         "publicDNS" : "",
         "vmUserPasswd" : ""
      },
      {
         "vmBlockDisk" : "/dev/sda1",
         "targetAction" : "None",
         "description" : "description",
         "spec_id" : "aws-us-east-1-shson",
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
         "public_ip" : "35.173.215.4",
         "native_status" : "Running",
         "csp_vm_id" : "aws-us-east-1-shson-01",
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
         "public_ip" : "18.206.13.233",
         "csp_vm_id" : "aws-us-east-1-shson-02",
         "native_status" : "Running"
      },
      {
         "targetAction" : "None",
         "id" : "aws-us-east-1-shson-03",
         "targetStatus" : "None",
         "name" : "aws-us-east-1-shson-03",
         "status" : "Running",
         "csp_vm_id" : "aws-us-east-1-shson-03",
         "native_status" : "Running",
         "public_ip" : "18.232.53.134"
      }
   ],
   "status" : "Running-(3/3)",
   "name" : "aws-us-east-1-shson"
}

[Logging to notify latest command history]

[Executed Command List]
[CMD] testAll-mcis-mcir-ns-cloud.sh gcp 1 shson
[CMD] testAll-mcis-mcir-ns-cloud.sh alibaba 1 shson
[CMD] testAll-mcis-mcir-ns-cloud.sh aws 1 shson
```

마지막의 [Executed Command List] 에는 수행한 커맨드의 히스토리가 포함됨. 
(cat ./executionStatus 를 통해 다시 확인 가능)
      
</details>

#### 3) MCIS 응용 기반 최종 검증

  - SSH 원격 커맨드 실행을 통해서 접속 여부 등을 확인 가능
    - command-mcis.sh  # 생성된 MCIS(다중VM)에 원격 명령 수행
    - 예시: command-mcis.sh aws 1 shson # aws의 1번 리전에 배치된 MCIS의 모든 VM에 IP 및 Hostname 조회를 수행
  - Nginx를 분산 배치하여, 웹서버 접속 시험이 가능
    - deploy-nginx-mcis.sh  # 생성된 MCIS(다중VM)에 Nginx 자동 배포
    - 예시: command-mcis.sh aws 1 shson # aws의 1번 리전에 배치된 MCIS의 모든 VM에 Nginx 및 웹페이지 설치 (접속 테스트 가능)
      ```
      ~/go/src/github.com/cloud-barista/cb-tumblebug/test/official/sequentialFullTest$ ./deploy-nginx-mcis.sh aws 1 shson
      ####################################################################
      ## Command (SSH) to MCIS 
      ####################################################################
      [Test for AWS]
      {
        "result_array" : [
            {
              "vm_ip" : "35.173.215.4",
              "vm_id" : "aws-us-east-1-shson-01",
              "result" : "WebServer is ready. Access http://35.173.215.4",
              "mcis_id" : "aws-us-east-1-shson"
            },
            {
              "vm_ip" : "18.206.13.233",
              "vm_id" : "aws-us-east-1-shson-02",
              "result" : "WebServer is ready. Access http://18.206.13.233",
              "mcis_id" : "aws-us-east-1-shson"
            },
            {
              "mcis_id" : "aws-us-east-1-shson",
              "result" : "WebServer is ready. Access http://18.232.53.134",
              "vm_id" : "aws-us-east-1-shson-03",
              "vm_ip" : "18.232.53.134"
            }
        ]
      }
      ```


#### [테스트 코드 파일 트리 설명]

<details>
<summary>테스트 스크립트 디렉토리 전체 Tree 보기</summary>

```
~/go/src/github.com/cloud-barista/cb-tumblebug/test/official$ tree
.
├── 1.configureSpider
│   ├── get-cloud.sh
│   ├── list-cloud.sh
│   ├── register-cloud.sh
│   └── unregister-cloud.sh
├── 2.configureTumblebug
│   ├── check-ns.sh
│   ├── create-ns.sh
│   ├── delete-all-config.sh
│   ├── delete-all-ns.sh
│   ├── delete-ns.sh
│   ├── get-config.sh
│   ├── get-ns.sh
│   ├── list-config.sh
│   ├── list-ns.sh
│   └── update-config.sh
├── 3.vNet
│   ├── create-vNet.sh
│   ├── delete-vNet.sh
│   ├── get-vNet.sh
│   ├── list-vNet.sh
│   └── spider-get-vNet.sh
├── 4.securityGroup
│   ├── create-securityGroup.sh
│   ├── delete-securityGroup.sh
│   ├── get-securityGroup.sh
│   ├── list-securityGroup.sh
│   └── spider-get-securityGroup.sh
├── 5.sshKey
│   ├── create-sshKey.sh
│   ├── delete-sshKey.sh
│   ├── force-delete-sshKey.sh
│   ├── get-sshKey.sh
│   ├── list-sshKey.sh
│   ├── spider-delete-sshKey.sh
│   └── spider-get-sshKey.sh
├── 6.image
│   ├── fetch-images.sh
│   ├── get-image.sh
│   ├── list-image.sh
│   ├── lookupImageList.sh
│   ├── lookupImage.sh
│   ├── registerImageWithId.sh
│   ├── registerImageWithInfo.sh
│   ├── spider-get-imagelist.sh
│   ├── spider-get-image.sh
│   ├── test-search-image.sh
│   ├── unregister-all-images.sh
│   └── unregister-image.sh
├── 7.spec
│   ├── fetch-specs.sh
│   ├── filter-specs.sh
│   ├── get-spec.sh
│   ├── list-spec.sh
│   ├── lookupSpecList.sh
│   ├── lookupSpec.sh
│   ├── range-filter-specs.sh
│   ├── register-spec.sh
│   ├── spider-get-speclist.sh
│   ├── spider-get-spec.sh
│   ├── test-sort-specs.sh
│   ├── test-update-spec.sh
│   ├── unregister-all-specs.sh
│   └── unregister-spec.sh
├── 8.mcis
│   ├── add-vm-to-mcis.sh
│   ├── create-mcis-no-agent.sh
│   ├── create-mcis-policy.sh
│   ├── create-mcis.sh
│   ├── create-single-vm-mcis.sh
│   ├── delete-mcis-policy-all.sh
│   ├── delete-mcis-policy.sh
│   ├── get-mcis-policy.sh
│   ├── get-mcis.sh
│   ├── just-terminate-mcis.sh
│   ├── list-mcis-policy.sh
│   ├── list-mcis.sh
│   ├── list-mcis-status.sh
│   ├── reboot-mcis.sh
│   ├── resume-mcis.sh
│   ├── spider-create-vm.sh
│   ├── spider-delete-vm.sh
│   ├── spider-get-vm.sh
│   ├── spider-get-vmstatus.sh
│   ├── status-mcis.sh
│   ├── suspend-mcis.sh
│   └── terminate-and-delete-mcis.sh
├── 9.monitoring
│   ├── get-monitoring-data.sh
│   └── install-agent.sh
├── conf.env
├── credentials.conf
├── credentials.conf.example
├── misc
│   ├── get-conn-config.sh
│   ├── get-region.sh
│   ├── list-conn-config.sh
│   └── list-region.sh
├── README.md
└── sequentialFullTest
    ├── cb-demo-support
    ├── cleanAll-mcis-mcir-ns-cloud.sh
    ├── command-mcis-custom.sh
    ├── command-mcis.sh
    ├── create-mcis-for-df.sh
    ├── deploy-dragonfly-docker.sh
    ├── deploy-loadMaker-to-mcis.sh
    ├── deploy-nginx-mcis.sh
    ├── deploy-nginx-mcis-vm-withGivenName.sh
    ├── deploy-nginx-mcis-with-loadmaker.sh
    ├── deploy-spider-docker.sh
    ├── deploy-tumblebug.sh
    ├── executionStatus
    ├── expand-mcis.sh
    ├── gen-sshKey.sh
    ├── gen-sshKey-withGivenMcisName.sh
    ├── sshkey-tmp
    │   ├── gcp-asia-east1-shson6.pem
    │   └── gcp-asia-east1-shson6.ppk
    ├── testAll-mcis-mcir-ns-cloud.sh
    ├── test-cloud.sh
    ├── test-mcir-ns-cloud.sh
    └── test-ns-cloud.sh

```

</details>

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
