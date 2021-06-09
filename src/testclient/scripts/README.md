# CB-Tumblebug (Multi-Cloud Infra Service Management)

이 디렉토리는 CB-Tumblebug의 기본적인 테스트 수행을 위한 테스트 스크립트를 포함

***

## [목차]

1. [실행 환경](#실행-환경)
2. [실행 방법](#실행-방법)

***

## [실행 환경]
- Linux (검증시험: Ubuntu 18.04)


## [실행 방법]

### (0) 클라우드 인증 정보, 테스트 기본 정보 입력
- credentials.conf 파일 생성 # Cloud 정보 등록을 위한 CSP별 인증정보 파일 생성 (사용자에 맞게 수정 필요)
  - 기본적인 클라우드 타입 (AWS, GCP, AZURE, ALIBABA)에 대해 템플릿 제공
    - credentials.conf.example 템플릿 파일을 참고하여 실재 정보가 포함된 credentials.conf 생성 또는 수정
  - 주의: credentials.conf 에는 개인의 중요 정보가 포함되므로 Github에 업로드되지 않도록 주의 필요    
    - credentials.conf가 gitignore에 등록되어 있으므로 git의 추적 대상에서는 제외되어 있음
- conf.env  # CB-Spider 및 Tumblebug 서버 위치, 클라우드 리젼, 테스트용 이미지명, 테스트용 스팩명 등 테스트 기본 정보 제공
  - 특별한 상황이 아니면 수정이 불필요함. (CB-Spider와 CB-TB의 위치가 localhost가 아닌 경우 수정 필요)
  - 클라우드 타입(CSP)별 약 1~3개의 기본 리전이 입력되어 있음
    - 이미지와 스팩은 리전에 의존성이 있는 경우가 많으므로, 리전별로 지정이 필요

### (1) 클라우드정보, Namespace, MCIR, MCIS 등 개별 제어 시험
- 제어하고 싶은 리소스 오브젝트에 대해, 해당 디렉토리로 이동하여 필요한 시험 수행
  - 오브젝트는 서로 의존성이 있으므로, 번호를 참고하여 오름차순으로 수행하는 것이 바람직함
    - 1.configureSpider  # 클라우드 정보 등록 관련 스크립트 모음
    - 2.configureTumblebug  # 네임스페이스 관련 스크립트 모음
    - 3.vNet  # MCIR vNet 생성 관련 스크립트 모음
    - 4.securityGroup  # MCIR securityGroup 생성 관련 스크립트 모음
    - 5.sshKey  # MCIR sshKey 생성 관련 스크립트 모음
    - 6.image  # MCIR image 등록 관련 스크립트 모음
    - 7.spec  # MCIR spec 등록 관련 스크립트 모음
    - 8.mcis  # MCIS 생성 및 제어 관련 스크립트 모음

### (2) 한꺼번에 통합 시험 
- sequentialFullTest 에 포함된 cleanAll-mcis-mcir-ns-cloud.sh 을 수행하면 모든 것을 한번에 테스트 가능
```
└── sequentialFullTest  # Cloud 정보 등록, NS 생성, MCIR 생성, MCIS 생성까지 한번에 자동 테스트
    ├── cleanAll-mcis-mcir-ns-cloud.sh  # 모든 오브젝트 역으로 제어
    ├── command-mcis.sh  # 생성된 MCIS(다중VM)에 원격 명령 수행
    ├── deploy-nginx-mcis.sh  # 생성된 MCIS(다중VM)에 Nginx 자동 배포
    ├── executionStatus  # 수행이 진행된 테스트 로그 (testAll 수행시 정보가 추가되며, cleanAll 수행시 정보가 제거됨)
    ├── testAll-mcis-mcir-ns-cloud.sh  # Cloud 정보 등록, NS 생성, MCIR 생성, MCIS 생성까지 한번에 자동 테스트
    ├── test-cloud.sh
    ├── test-mcir-ns-cloud.sh
    └── test-ns-cloud.sh
```
- 사용 예시
  - 생성 테스트 (./testAll-mcis-mcir-ns-cloud.sh "CloudType" "Region" "DeveloperName" "NumOfVMs")
    - ./testAll-mcis-mcir-ns-cloud.sh all 1 shson 3      # 등록된 CSP 및 리전들을 활용한 MCIS 생성 (conf.env의 NumCSP, NumRegion 에 따라 VM 생성) shson이라는 개발자명으로 테스트 수행
    - ./testAll-mcis-mcir-ns-cloud.sh aws 1 shson 3      # aws의 1번 리전에 shson이라는 개발자명으로 테스트 수행
    - ./testAll-mcis-mcir-ns-cloud.sh aws 2 shson 3      # aws의 2번 리전에 shson이라는 개발자명으로 테스트 수행
    - ./testAll-mcis-mcir-ns-cloud.sh aws 3 shson 3      # aws의 3번 리전에 shson이라는 개발자명으로 테스트 수행
    - ./testAll-mcis-mcir-ns-cloud.sh gcp 1 shson 3      # gcp의 1번 리전에 shson이라는 개발자명으로 테스트 수행
    - ./testAll-mcis-mcir-ns-cloud.sh gcp 2 shson 3      # gcp의 2번 리전에 shson이라는 개발자명으로 테스트 수행
    - ./testAll-mcis-mcir-ns-cloud.sh azure 1 shson 3    # azure의 1번 리전에 shson이라는 개발자명으로 테스트 수행
    - ./testAll-mcis-mcir-ns-cloud.sh alibaba 1 shson 3  # alibaba의 1번 리전에 shson이라는 개발자명으로 테스트 수행
  - 제거 테스트 (이미 수행이 진행된 클라우드타입/리전/개발자명 으로만 삭제 진행이 필요)
    - ./cleanAll-mcis-mcir-ns-cloud.sh all 1 shson       # all로 수행된 shson이라는 개발자명으로 제거 수행
    - ./cleanAll-mcis-mcir-ns-cloud.sh aws 1 shson       # aws의 1번 리전에 shson이라는 개발자명으로 제거 수행
    - ./cleanAll-mcis-mcir-ns-cloud.sh aws 2 shson       # aws의 2번 리전에 shson이라는 개발자명으로 제거 수행
    - ./cleanAll-mcis-mcir-ns-cloud.sh aws 3 shson       # aws의 3번 리전에 shson이라는 개발자명으로 제거 수행
    - ./cleanAll-mcis-mcir-ns-cloud.sh gcp 1 shson       # gcp의 1번 리전에 shson이라는 개발자명으로 제거 수행
    - ./cleanAll-mcis-mcir-ns-cloud.sh gcp 2 shson       # gcp의 2번 리전에 shson이라는 개발자명으로 제거 수행
    - ./cleanAll-mcis-mcir-ns-cloud.sh azure 1 shson     # azure의 1번 리전에 shson이라는 개발자명으로 제거 수행
    - ./cleanAll-mcis-mcir-ns-cloud.sh alibaba 1 shson   # alibaba의 1번 리전에 shson이라는 개발자명으로 제거 수행

```
~/go/src/github.com/cloud-barista/cb-tumblebug/src/testclient/scripts/sequentialFullTest$ ./testAll-mcis-mcir-ns-cloud.sh aws 1 shson 3
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
   "message" : "The namespace ns-01 already exists."
}
####################################################################
## 3. vNet: Create
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
## 4. SecurityGroup: Create
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
## 5. sshKey: Create
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
## 6. image: Register
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
## 7. spec: Register
####################################################################
[Test for AWS]
{
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
## 8. vm: Create MCIS
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
         "targetStatus" : "None",
         "subnetId" : "aws-us-east-1-shson",
         "location" : {
            "nativeRegion" : "us-east-1",
            "cloudType" : "aws",
            "latitude" : "38.1300",
            "briefAddr" : "Virginia",
            "longitude" : "-78.4500"
         },
         "vmUserAccount" : "",
         "region" : {
            "Region" : "us-east-1",
            "Zone" : "us-east-1f"
         },
         "imageId" : "aws-us-east-1-shson",
         "privateDNS" : "ip-192-168-1-108.ec2.internal",
         "vmBootDisk" : "/dev/sda1",
         "status" : "Running",
         "securityGroupIds" : [
            "aws-us-east-1-shson"
         ],
         "vmUserPassword" : "",
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
         "vNetId" : "aws-us-east-1-shson",
         "sshKeyId" : "aws-us-east-1-shson",
         "privateIP" : "192.168.1.108",
         "connectionName" : "aws-us-east-1",
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
## 8. VM: Status MCIS
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
         "cspVmId" : "aws-us-east-1-shson-01",
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
         "cspVmId" : "aws-us-east-1-shson-02",
         "nativeStatus" : "Running"
      },
      {
         "targetAction" : "None",
         "id" : "aws-us-east-1-shson-03",
         "targetStatus" : "None",
         "name" : "aws-us-east-1-shson-03",
         "status" : "Running",
         "cspVmId" : "aws-us-east-1-shson-03",
         "nativeStatus" : "Running",
         "publicIp" : "18.232.53.134"
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


- MCIS 응용 기반 최종 검증
  - SSH 원격 커맨드 실행을 통해서 접속 여부 등을 확인 가능
    - command-mcis.sh  # 생성된 MCIS(다중VM)에 원격 명령 수행
    - 예시: command-mcis.sh aws 1 shson # aws의 1번 리전에 배치된 MCIS의 모든 VM에 IP 및 Hostname 조회를 수행
  - Nginx를 분산 배치하여, 웹서버 접속 시험이 가능
    - deploy-nginx-mcis.sh  # 생성된 MCIS(다중VM)에 Nginx 자동 배포
    - 예시: command-mcis.sh aws 1 shson # aws의 1번 리전에 배치된 MCIS의 모든 VM에 Nginx 및 웹페이지 설치 (접속 테스트 가능)
      ```
      ~/go/src/github.com/cloud-barista/cb-tumblebug/src/testclient/scripts/sequentialFullTest$ ./deploy-nginx-mcis.sh aws 1 shson
      ####################################################################
      ## Command (SSH) to MCIS 
      ####################################################################
      [Test for AWS]
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


## [테스트 코드 파일 트리 설명]
```
son@son:~/go/src/github.com/cloud-barista/cb-tumblebug/src/testclient/scripts$ tree
.
├── 1.configureSpider                  # 클라우드 정보 등록 관련 스크립트 모음
│   ├── get-cloud.sh
│   ├── list-cloud.sh
│   ├── register-cloud.sh
│   └── unregister-cloud.sh
├── 2.configureTumblebug               # 네임스페이스 및 TB 설정 관련 스크립트 모음
│   ├── check-ns.sh
│   ├── create-ns.sh
│   ├── delete-all-ns.sh
│   ├── delete-ns.sh
│   ├── get-config.sh
│   ├── get-ns.sh
│   ├── init-all-config.sh
│   ├── init-config.sh
│   ├── list-config.sh
│   ├── list-ns.sh
│   └── update-config.sh
├── 3.vNet                             # MCIR vNet 생성 관련 스크립트 모음
│   ├── create-vNet.sh
│   ├── delete-vNet.sh
│   ├── get-vNet.sh
│   ├── list-vNet.sh
│   ├── spider-create-vNet.sh
│   ├── spider-delete-vNet.sh
│   ├── spider-get-vNet.sh
│   ├── testAddAsso.sh
│   ├── testDeleteAsso.sh
│   └── testGetAssoCount.sh
├── 4.securityGroup                    # MCIR securityGroup 생성 관련 스크립트 모음
│   ├── create-securityGroup.sh
│   ├── delete-securityGroup.sh
│   ├── get-securityGroup.sh
│   ├── list-securityGroup.sh
│   ├── spider-get-securityGroup.sh
│   ├── testAddAsso.sh
│   ├── testDeleteAsso.sh
│   └── testGetAssoCount.sh
├── 5.sshKey                           # MCIR sshKey 생성 관련 스크립트 모음
│   ├── create-sshKey.sh
│   ├── delete-sshKey.sh
│   ├── force-delete-sshKey.sh
│   ├── get-sshKey.sh
│   ├── list-sshKey.sh
│   ├── spider-delete-sshKey.sh
│   ├── spider-get-sshKey.sh
│   ├── testAddAsso.sh
│   ├── testDeleteAsso.sh
│   └── testGetAssoCount.sh
├── 6.image                            # MCIR image 등록 관련 스크립트 모음
│   ├── fetch-images.sh
│   ├── get-image.sh
│   ├── list-image.sh
│   ├── lookupImageList.sh
│   ├── lookupImage.sh
│   ├── obsolete_registerImageWithInfo.sh
│   ├── registerImageWithId.sh
│   ├── spider-get-imagelist.sh
│   ├── spider-get-image.sh
│   ├── testAddAsso.sh
│   ├── testDeleteAsso.sh
│   ├── testGetAssoCount.sh
│   ├── test-search-image.sh
│   ├── unregister-all-images.sh
│   └── unregister-image.sh
├── 7.spec                             # MCIR spec 등록 관련 스크립트 모음
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
│   ├── testAddAsso.sh
│   ├── testDeleteAsso.sh
│   ├── testGetAssoCount.sh
│   ├── test-sort-specs.sh
│   ├── unregister-all-specs.sh
│   ├── unregister-spec.sh
│   └── update-spec.sh
├── 8.mcis                             # MCIS 생성 및 제어 관련 스크립트 모음
│   ├── add-vmgroup-to-mcis.sh
│   ├── add-vm-to-mcis.sh
│   ├── create-mcis-no-agent.sh
│   ├── create-mcis-policy.sh
│   ├── create-mcis.sh
│   ├── create-single-vm-mcis.sh
│   ├── delete-mcis-policy-all.sh
│   ├── delete-mcis-policy.sh
│   ├── get-mcis-policy.sh
│   ├── get-mcis.sh
│   ├── terminate-mcis.sh
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
│   └── delete-mcis.sh
├── 9.monitoring                       # 모니터링 에이전트 설치 및 테스트 관련 스크립트
│   ├── get-monitoring-data.sh
│   └── install-agent.sh
├── common-functions.sh                # 스크립트 공통 함수
├── conf.env   # 클라우드 리젼, 테스트용 이미지명, 테스트용 스팩명 등 테스트 기본 정보 제공
├── credentials.conf # Cloud 정보 등록을 위한 CSP별 인증정보 (사용자에 맞게 수정 필요)
├── credentials.conf.example
├── misc
│   ├── get-conn-config.sh
│   ├── get-region.sh
│   ├── list-conn-config.sh
│   └── list-region.sh
├── README.md
└── sequentialFullTest  # Cloud 정보 등록, NS 생성, MCIR 생성, MCIS 생성까지 한번에 자동 테스트
    ├── cb-demo-support
    ├── cleanAll-mcis-mcir-ns-cloud.sh # 지정된 MCIS관련 모든 오브젝트 역으로 중지 및 삭제하여 정리
    ├── command-mcis-custom.sh
    ├── command-mcis.sh          # MCIS에 SSH 커맨드를 테스트
    ├── create-mcis-for-df.sh    # MCIS를 생성하고, CB-Dragonfly를 자동 배포
    ├── delete-objects-becareful.sh
    ├── delete-object.sh
    ├── deploy-dragonfly-docker.sh    # 지정된 MCIS에 CB-Dragonfly를 자동 배포
    ├── deploy-loadMaker-to-mcis.sh
    ├── deploy-nginx-mcis.sh     # 생성된 MCIS(다중VM)에 Nginx 자동 배포
    ├── deploy-nginx-mcis-vm-withGivenName.sh
    ├── deploy-nginx-mcis-with-loadmaker.sh
    ├── deploy-spider-docker.sh
    ├── deploy-tumblebug.sh
    ├── deploy-weavescope-to-mcis.sh   # 생성된 MCIS(다중VM)에 WeaveScope 자동 배포
    ├── deploy-weavescope-to-mcis-total.sh
    ├── executionStatus  # 수행이 진행된 테스트 로그 (testAll 수행시 정보가 추가되며, cleanAll 수행시 정보가 제거됨)
    ├── expand-mcis.sh
    ├── gen-sshKey.sh      # MCIS에 대한 sshKey 생성 (sshkey-tmp 에 *.PEM 으로 저장됨)
    ├── gen-sshKey-withGivenMcisName.sh
    ├── get-object.sh      # CB-TB 오브젝트 데이터를 직접 조회
    ├── list-object.sh     # CB-TB 오브젝트 리스트 데이터를 직접 조회
    ├── sshkey-tmp         # sshKey 가 임시로 저장되는 디렉토리 (gitignore)
    ├── testAll-mcis-mcir-ns-cloud.sh  # Cloud 정보 등록, NS 생성, MCIR 생성, MCIS 생성까지 한번에 자동 테스트
    ├── test-cloud.sh
    ├── test-mcir-ns-cloud.sh
    └── test-ns-cloud.sh
```
