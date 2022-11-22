# v0.7.0 (Cortado, 2022.11.25.)

### Tested with 
- CB-Spider [v0.7.0](https://github.com/cloud-barista/cb-spider/releases/tag/v0.7.0) 
- CB-Dragonfly [v0.7.0](https://github.com/cloud-barista/cb-dragonfly/releases/tag/v0.7.0)
- (for developers) CB-MapUI [v0.7.0](https://github.com/cloud-barista/cb-mapui/releases/tag/v0.7.0)
- (under integration) CB-Larva Network [v0.0.15](https://github.com/cloud-barista/cb-larva/releases/tag/v0.0.15)

### What's Changed
* Update outdated Alibaba Ubuntu images by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1155
* Update xonotic usecase release v0.8.5 by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1156
* Add NLB mgmt feature by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1158
* Bump to go 1.19 & Update `go.mod` by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1160
* Add test VM image set by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1162
* Update Xonotic 0.8.5 script for ubuntu 22.04 dist by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1163
* Fix image id for EC2 debian 10 by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1164
* Update and fix spec list by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1165
* Add cost priority for specs in same location by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1166
* Update OS and Go in workflows by @yunkon-kim in https://github.com/cloud-barista/cb-tumblebug/pull/1167
* Update import pkg versions by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1168
* Enhance error handling in MCIS Policy check by @bconfiden2 in https://github.com/cloud-barista/cb-tumblebug/pull/1170
* docs: add bconfiden2 as a contributor for code by @allcontributors in https://github.com/cloud-barista/cb-tumblebug/pull/1171
* Provide default values for NLB API by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1172
* [Workflow] Update Swagger REST API doc by @cloud-barista-hub in https://github.com/cloud-barista/cb-tumblebug/pull/1173
* Update MapUI version by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1174
* Add 'NLB VM addition/removal' feature by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1175
* Add get VM list in a VMGroup within a MCIS by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1176
* Add API to get VMGroup list in a MCIS by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1177
* Enhance NLB mgmt feature (2) by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1178
* [Workflow] Update Swagger REST API doc by @cloud-barista-hub in https://github.com/cloud-barista/cb-tumblebug/pull/1179
* Update NCP-VPC & NHN Cloud regions & zones by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1180
* Add scaleOut feature for VMGroup in a MCIS by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1181
* [Workflow] Update Swagger REST API doc by @cloud-barista-hub in https://github.com/cloud-barista/cb-tumblebug/pull/1183
* Add 'DataDisk mgmt' feature by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1182
* Fix Echo vulnerability CVE-2022-40083 by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1185
* Update CB-Spider version for runContainer by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1186
* Add get VM ID Name info in detail by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1187
* Add yaml config feature to handle different available values for each cloud by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1188
* Update runSpider version to 0.6.11 by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1189
* Add customImage mgmt and snapshot features by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1190
* Add dev-api-diff.html info by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1191
* Feat: StartVmWithSnapshot + RegisterConsequentVolumes by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1192
* Implement `cloud_conf.yaml` manifest handling feat. for NLB by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1193
* Add example values for NLB request by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1194
* Tidy `TbNLBReq` fields by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1197
* Set NLB health checker info by reflection by @yunkon-kim in https://github.com/cloud-barista/cb-tumblebug/pull/1196
* Provide one-stop values for nlb api by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1198
* Simplify docker install script by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1200
* Remove associated NLBs with MCIS del by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1201
* [Workflow] Update Swagger REST API doc by @cloud-barista-hub in https://github.com/cloud-barista/cb-tumblebug/pull/1202
* Add 'GetAvailableDataDisks' feature by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1195
* HotFix for delete mcis err due to nlb by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1203
* Provide defaults/examples for dataDisk & snapshot by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1205
* Change vmGroup to subGroup by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1206
* [Workflow] Update Swagger REST API doc by @cloud-barista-hub in https://github.com/cloud-barista/cb-tumblebug/pull/1207
* Test and update associated FWs by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1209
* Remove historic add VM way by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1210
* Ignore rootDiskType for creating VM with customIMG by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1211
* Add Swagger godoc for CustomImage REST APIs by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1212
* Enhance API for nlb disk customimg by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1213
* [Workflow] Update Swagger REST API doc by @cloud-barista-hub in https://github.com/cloud-barista/cb-tumblebug/pull/1214
* Remove old api documents by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1215
* Add vm (id) list filtering feature by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1216
* Add get mcis access info feature by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1218
* Apply omitempty for nlb obj in mcisAccessInfo by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1221
* Provide show or hide option for sshKey in access info by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1222
* [Workflow] Update Swagger REST API doc by @cloud-barista-hub in https://github.com/cloud-barista/cb-tumblebug/pull/1223
* Add scripts to command SW NLB-HAProxy by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1224
* Add SW NLB config for PoC by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1225
* Enable coordinateFair option for recommend by location by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1235
* feat: add VM to MCIS dynamically by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1237
* Add 'Location' field in NLB object by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1238
* Modify 'serviceType' value in InstallMonAgentReq by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1239
* fix:CoreDelAllMcis to  DelAllMcis by @arshad-k7 in https://github.com/cloud-barista/cb-tumblebug/pull/1236
* Add StrictHostKeyChecking=no for mcis file copy by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1240
* Update deprecated image IDs by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1241
* Fix orchestration scaleout err by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1242
* Add test-mcis-dynamic-all-for-one.sh by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1243
* Update `availableDataDisk` REST API by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1244
* Change func name `CorePostMcisVm` into `CreateMcisVm` by @Rohit-R2000 in https://github.com/cloud-barista/cb-tumblebug/pull/1219
* Hotfix for register-cloud-interactive.sh for CloudIt credential registration by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1245
* Enhance orchestration mechanism with various features by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1246
* [Workflow] Update Swagger REST API doc by @cloud-barista-hub in https://github.com/cloud-barista/cb-tumblebug/pull/1247
* Fix bugs on NLB mgmt feature by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1248
* Add MC NLB service feature poc by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1249
* Fix nil reference error by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1250
* Enhance error handling for DataDiskIds in mcis provision by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1251
* docs: add Rohit-R2000 as a contributor for code by @allcontributors in https://github.com/cloud-barista/cb-tumblebug/pull/1252
* docs: add arshad-k7 as a contributor for code by @allcontributors in https://github.com/cloud-barista/cb-tumblebug/pull/1253
* Add CreateSystemMcisDynamic for network probe by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1254
* Change default recommendation rule by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1255
* Change message for nginx index by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1256
* Enhance mRTT benchmark to gen latency Map by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1257
* [Workflow] Update Swagger REST API doc by @cloud-barista-hub in https://github.com/cloud-barista/cb-tumblebug/pull/1258
* Hotfix for runtime err in monitoring agent by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1259
* Validate and update assets data by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1261
* Enhance latency map creation by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1262
* Auto NLB deployment feature by global clouds latency evaluation by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1265
* Make nginx demo page refresh by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1266
* Fix error in refresh web demo by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1267
* Add weavescope script by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1268
* Show access info of Global-NLB by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1269
* Get access info in parallel by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1270
* Update assets (spec & image) by @jihoon-seo in https://github.com/cloud-barista/cb-tumblebug/pull/1263
* Update Spider version by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1271
* Fix not available image ids by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1272
* Enhance error handling for listing custom img by @seokho-son in https://github.com/cloud-barista/cb-tumblebug/pull/1273

### API
- Swagger UI URL: https://cloud-barista.github.io/cb-tumblebug-api-web/?url=https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/v0.7.0/src/docs/swagger.yaml

### What's Changed

**Full Changelog**: https://github.com/cloud-barista/cb-tumblebug/compare/v0.6.0...v0.7.0


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
