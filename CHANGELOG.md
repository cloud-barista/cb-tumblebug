# v0.6.0 (Caffè latte, 2022.07.08.)

### Tested with 
- CB-Spider (https://github.com/cloud-barista/cb-spider/releases/tag/v0.6.0)
- CB-Dragonfly (https://github.com/cloud-barista/cb-dragonfly/releases/tag/v0.6.0)
- (Optional: for developers) CB-MapUI (https://github.com/cloud-barista/cb-mapui/releases/tag/v0.6.0)
- (Optional: under integration) CB-Larva network (https://github.com/cloud-barista/cb-larva/releases/tag/v0.0.15)

### Note
* Add usecase for FPS Game Xonotic by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1006
* Fix deploy-fps-game for background mode by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1007
* Add scripts for FPS game usecase by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1008
* Fix wrong switched scripts by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1009
* Support Xonotic server configuration by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1010
* Fix response message for MCIS terminate by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1014
* Update 'registerExistingSG' function by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1015
* Add `registerExistingSSHKey` feature by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1016
* Enhance `registerExistingVNet` feature by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1018
* Enhance `registerExisting SG/SSHKey` feature by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1019
* Add comments for required params for register by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1020
* Add `UpdateSshKey` feature by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1023
* Organize REST API server Go files by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1026
* Unspecify GitHub Actions' versions by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1029
* Fix GitHub workflows by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1030
* Add script for remote copy file to MCIS VMs by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1031
* Update Swagger serving URL by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1032
* Bump actions/checkout from 2 to 3 by @dependabot in https://github.com/cloud-barista/cb-tumblebug/pull/1035
* Update README-EN.md by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1034
* Update cb-spider and cb-mapui version used in scripts by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1036
* Bump actions/cache from 2 to 3 by @dependabot in https://github.com/cloud-barista/cb-tumblebug/pull/1039
* Feat/graceful shutdown by @sypark9646 in https://github.com/cloud-barista/cb-tumblebug/pull/874
* Align shutdown message by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1041
* Bump peter-evans/create-pull-request from 3 to 4 by @dependabot in https://github.com/cloud-barista/cb-tumblebug/pull/1045
* Initial codes to register existing CSP VM by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1046
* Add support for NHN Cloud by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1048
* Update to prepend NS prefix to SG name by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1049
* Add RootDiskType, RootDiskSize handling feature by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/974
* Wait until the graceful shutdown is processed by @yunkon-kim in https://github.com/cloud-barista/cb-tumblebug/pull/1050
* Fix bug on NS prefix of SG name by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1052
* Enhance container run scripts by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1053
* Add Cloud Adaptive Network configuration when creating MCIS by @yunkon-kim in https://github.com/cloud-barista/cb-tumblebug/pull/1054
* Add Docker engine installation script by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1060
* Remove duplicated script create-single-vm-mcis.sh by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1058
* Bump actions/setup-go from 2 to 3 by @dependabot in https://github.com/cloud-barista/cb-tumblebug/pull/1061
* Update readme for Xonotic usecase by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1063
* Remove duplicated script codes in cbadm by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1066
* Update `testclient/scripts/conf.env` by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1067
* Hotfix cb-larva package by @yunkon-kim in https://github.com/cloud-barista/cb-tumblebug/pull/1068
* Support credential selection for dynamic MCIS creation by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1069
* Merge duplicated inspect functions for vm and mcir by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1071
* Update InspecResource struct with csp only and cleanup by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1073
* Fix misspelled word (#1057) by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1075
* Register all CSP resources to CB-TB objs by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1076
* Change struct for inspectResources and registerCspResources with Err fix by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1078
* Add overview to inspect resources and registerCspResources for all connectionConfigs by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1080
* Tidy redundant comments & prints by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1084
* Temporal removal of GCP from RegisterCspResAll and hotfix by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1086
* Add firewallRule mgmt feature by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1083
* Enhance RegisterCspNativeResourcesAll mechanism by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1092
* Add feat for Inspect Resources Overview by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1094
* Trials for inspectResourcesOverview to prevent rateLimitExceeded from CSP by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1096
* Enhance RegisterCspNativeResourcesAll usability by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1097
* Hotfix for runtime err in RegisterCspNativeRes by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1101
* Add and update cb-network APIs by @yunkon-kim in https://github.com/cloud-barista/cb-tumblebug/pull/1100
* Update CB-MapUI version to v0.5.3 by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1103
* Enhance `ListResource()` by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1105
* Update gomod gosum by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1106
* Fix unmatched info in grpc test code by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1107
* Fix create-ns.sh error (fix #1085) by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1108
* Fix region name typo (norwaywestest) by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1109
* Fix path issue in test scripts by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1110
* Update image and spec managed by tb (fix #958) by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1111
* Update GCP images to the latest by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1113
* Add support for NCP-VPC by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1114
* Enhance output for mcis status by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1116
* Update predefined spec, img, connection list by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1118
* Update configs for cloud resources by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1119
* Enable RootDiskType for dynamic provisioning to fix Alibaba provisioning failures by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1120
* Update to enable disk type settings by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1122
* Update NCP-VPC metadata by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1124
* Update NCP-VPC metadata by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1125
* Fix problematic configurations to prepare demo by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1127
* Provide naming rule checking for dynamic MCIS provisioning by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1128
* Fix firewallrule description in swagger API doc by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1129
* Add updating NS functionality by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1132
* Fix status check nil error in GetVmStatusAsync by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1135
* Add resource list filtering feature by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1137
* Reconstuct resource IDs for a registered MCIS by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1139
* Enhance stability and speed of TB by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1140
* Add script for test-mcis-dynamic-all by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1142
* Fix misspell by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1143
* Fix control mcis force option error by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1146
* Update alibaba image id by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1147
* Extend API rate limit from 1 to 2 in a sec by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1148
* Update Spider and MapUI container images default version by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1149
* Staging CB-TB v0.6.0 by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1150
* Update Cloud Adaptive Network API by @yunkon-kim in https://github.com/cloud-barista/cb-tumblebug/pull/1151


### API
- Swagger UI URL: https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/v0.6.0/src/docs/swagger.yaml

### What's Changed

**Full Changelog**: https://github.com/cloud-barista/cb-tumblebug/compare/v0.5.0...v0.6.0

***


# v0.5.0 (Affogato, 2021.12.16.)

### Tested with 
- CB-Spider (https://github.com/cloud-barista/cb-spider/releases/tag/v0.5.0)
- CB-Dragonfly (https://github.com/cloud-barista/cb-dragonfly/releases/tag/v0.5.0)

### Note
- Fix error regarding `OpenSQL()` (Issue: [#645](https://github.com/cloud-barista/cb-tumblebug/issues/645), PR: [#646](https://github.com/cloud-barista/cb-tumblebug/pull/646))
- Update grpc protobuf to sync with rest [#668](https://github.com/cloud-barista/cb-tumblebug/pull/668)
- Change method to input parameters for script [#677](https://github.com/cloud-barista/cb-tumblebug/pull/677)
- Refine source code (variable name in camelCase consistently)
- Add list MCIS simple option [#731](https://github.com/cloud-barista/cb-tumblebug/pull/731)
- Add MCIS status count feature and update MCIS response field [#732](https://github.com/cloud-barista/cb-tumblebug/pull/732)
- Apply colors to important messages in script [#798](https://github.com/cloud-barista/cb-tumblebug/pull/798)
- Add interactive scripts to run containers to support CB-Tumblebug [#764](https://github.com/cloud-barista/cb-tumblebug/pull/764)
- Fix some REST APIs methods from get to post [#742](https://github.com/cloud-barista/cb-tumblebug/pull/742)
- Verify cb-tb and cb-sp are ready [#741](https://github.com/cloud-barista/cb-tumblebug/pull/741)
- Enhance capability of mcis recommendation [#833](https://github.com/cloud-barista/cb-tumblebug/pull/833)
- Add omitted error handling [#828](https://github.com/cloud-barista/cb-tumblebug/pull/828)
- English README.md [#825](https://github.com/cloud-barista/cb-tumblebug/pull/825)
- Influencing cb-spider resource objects with namespace [#909](https://github.com/cloud-barista/cb-tumblebug/pull/909)
- Add delete all default resource feature (deleteAll output becomes list) [#942](https://github.com/cloud-barista/cb-tumblebug/pull/942)
- Remove control action parameters from get mcis [#928](https://github.com/cloud-barista/cb-tumblebug/pull/928)
- Add feature for connection with geolocation [#936](https://github.com/cloud-barista/cb-tumblebug/pull/936)
- Enable dynamic MCIS provisioning [#879](https://github.com/cloud-barista/cb-tumblebug/pull/879)
- Added and tested IBM(VPC) CSP and Tencent CSP [#969](https://github.com/cloud-barista/cb-tumblebug/discussions/969)
- Add option=terminate for delete mcis [#959](https://github.com/cloud-barista/cb-tumblebug/pull/959)
- Expedite speed of scripts
- Add SystemLabel field to MCIS info for CB-DF CB-MCKS CB-webtool integration [#977](https://github.com/cloud-barista/cb-tumblebug/pull/977)
- Update assets/cloudspec.csv [#975](https://github.com/cloud-barista/cb-tumblebug/pull/975)

### API
- Swagger UI URL: https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/v0.5.0/src/api/rest/docs/swagger.yaml
- Trace for API changes: diff between two API doc files like 
  - `git diff https://github.com/cloud-barista/cb-tumblebug/blob/v0.4.0/src/api/rest/docs/swagger.yaml https://github.com/cloud-barista/cb-tumblebug/blob/v0.5.0/src/api/rest/docs/swagger.yaml`

### What's Changed

**Full Changelog**: https://github.com/cloud-barista/cb-tumblebug/compare/v0.4.0...v0.5.0

***


# v0.4.0 (Cafe Mocha, 2021.06.30.)

### API Change 
Ref) [API ChangeLog](https://github.com/cloud-barista/cb-tumblebug/discussions/416)

- Add VMGroup parameter in create MCIS API
- Add Private IP parameter in get MCIS status API
- Add MCIS Refine option in MCIS action (get) API
- Add verifiedUserName parameter in get spec API
- Add API for ListResourceId, ListMcisId, ListVmId 
- Add TB object control API
- Add inspectResources API
- Change API style: snakeCase to camelCase


### Feature
Ref) [Supported cloud service providers](https://github.com/cloud-barista/cb-tumblebug/discussions/429)

- Add VM group feature to request multiple VMs simply [#413](https://github.com/cloud-barista/cb-tumblebug/pull/413)
- Provide SystemMessage to vm status object [#475](https://github.com/cloud-barista/cb-tumblebug/pull/475)
- Enhance and expedite mcis lifecycle handling [#625](https://github.com/cloud-barista/cb-tumblebug/pull/625)
- Add MCIS Refine feature [#572](https://github.com/cloud-barista/cb-tumblebug/pull/572)
- Add feature for general TB object retrieve [#417](https://github.com/cloud-barista/cb-tumblebug/pull/417)
- Add initial code for mcis and vm plan with location-based algo [#511](https://github.com/cloud-barista/cb-tumblebug/pull/511)
- Add inspectVMs function [#505](https://github.com/cloud-barista/cb-tumblebug/pull/505)
- Expedite auto agent installation [#448](https://github.com/cloud-barista/cb-tumblebug/pull/448)
- Enhance ssh username verification performance [#423](https://github.com/cloud-barista/cb-tumblebug/pull/423) 
- Add WeaveScope deployment script [#419](https://github.com/cloud-barista/cb-tumblebug/pull/419)
- Add jitsi video conference automation [#476](https://github.com/cloud-barista/cb-tumblebug/pull/476)
- Add script for deploying web game server [#609](https://github.com/cloud-barista/cb-tumblebug/pull/609)

### Bug Fix
- Enhance error handing for provisioning and cmd phases [#435](https://github.com/cloud-barista/cb-tumblebug/pull/435)
- Fix agent installation bug and script update [#437](https://github.com/cloud-barista/cb-tumblebug/pull/437)
- Fix initial failed status in MCIS provisioning [#467](https://github.com/cloud-barista/cb-tumblebug/pull/467)
- Fix list object key parsing bug [#607](https://github.com/cloud-barista/cb-tumblebug/pull/607)
- Patch gRPC API [#536](https://github.com/cloud-barista/cb-tumblebug/pull/536)

### Note
- Default development environment: Go v1.16 

***

# v0.3.0-espresso (2020.12.03.)

### API Change
- MCIS 자동 제어 기능 API 추가
- 동적 시스템 환경 설정 변경 기능 API 추가
- MCIS 생성 API의 모니터링 에이전트 자동 배치 옵션 제공

### Feature
- MCIS 생성시 모니터링 에이전트 자동 배치 기능 추가
- MCIS 자동 제어 기능 추가
- MCIS 시나리오 테스트 스크립트 추가
- MCIS 마스터 VM 및 VM IP 정보 제공 기능 추가
- MCIR VM 사양 패치 및 등록 기능 추가
- 동적 시스템 환경 설정 변경 기능 추가

### Bug Fix
- MCIS 종료시 런타임 오류 수정

***

# v0.2.0-cappuccino (2020.06.02.)

### API Change
- MCIS 통합 원격 커맨드 기능 API 추가
- 개별 VM 원격 커맨드 기능 API 추가
- MCIR Subnet 관리 API 제거
- MCIR VNic 관리 API 제거
- MCIR PublicIP 관리 API 제거
- 전체 Request 및 Response Body의 상세 항목 변경 (API 예시 참고)

### Feature
- MCIS 및 VM에 현재 수행 중인 제어 명령 정보를 관리
- 멀티 클라우드 동적 성능 밴치마킹 기능 일부 추가 (PoC 수준)
- MCIS VM 생성 및 제어시 Goroutine을 적용하여 속도 개선
- MCIS 및 VM 원격 커맨드 기능 추가
- MCIS 오브젝트 정보 보완 (VM의 위경도 정보 제공)

### Bug Fix
- MCIS 라이프사이클 오류 개선

***

# v0.1.0-americano (2019.12.23.)

### Feature
- Namespace, MCIR, MCIS 관리 기본 기능 제공
