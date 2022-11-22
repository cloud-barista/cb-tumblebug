## Version latest to latest
---
### What's New
---
* `GET` /ns/{nsId}/benchmarkLatency/mcis/{mcisId} Run MCIS benchmark for network latency
* `POST` /ns/{nsId}/mcis/{mcisId}/mcSwNlb Create a special purpose MCIS for NLB and depoly and setting SW NLB
* `GET` /ns/{nsId}/mcis/{mcisId}/nlb List all NLBs or NLBs' ID
* `POST` /ns/{nsId}/mcis/{mcisId}/nlb Create NLB
* `DELETE` /ns/{nsId}/mcis/{mcisId}/nlb Delete all NLBs
* `GET` /ns/{nsId}/mcis/{mcisId}/nlb/{nlbId} Get NLB
* `DELETE` /ns/{nsId}/mcis/{mcisId}/nlb/{nlbId} Delete NLB
* `GET` /ns/{nsId}/mcis/{mcisId}/nlb/{nlbId}/healthz Get NLB Health
* `POST` /ns/{nsId}/mcis/{mcisId}/nlb/{nlbId}/vm Add VMs to NLB
* `DELETE` /ns/{nsId}/mcis/{mcisId}/nlb/{nlbId}/vm Delete VMs from NLB
* `GET` /ns/{nsId}/mcis/{mcisId}/subgroup List SubGroup IDs in a specified MCIS
* `GET` /ns/{nsId}/mcis/{mcisId}/subgroup/{subgroupId} List VMs with a SubGroup label in a specified MCIS
* `POST` /ns/{nsId}/mcis/{mcisId}/subgroup/{subgroupId} ScaleOut subGroup in specified MCIS
* `GET` /ns/{nsId}/mcis/{mcisId}/vm/{vmId}/dataDisk Get available dataDisks for a VM
* `PUT` /ns/{nsId}/mcis/{mcisId}/vm/{vmId}/dataDisk Attach/Detach available dataDisk
* `POST` /ns/{nsId}/mcis/{mcisId}/vm/{vmId}/snapshot Snapshot VM and create a Custom Image Object using the Snapshot
* `POST` /ns/{nsId}/mcis/{mcisId}/vmDynamic Create VM Dynamically and add it to MCIS
* `GET` /ns/{nsId}/resources/customImage List all customImages or customImages' ID
* `POST` /ns/{nsId}/resources/customImage Register existing Custom Image in a CSP
* `DELETE` /ns/{nsId}/resources/customImage Delete all customImages
* `GET` /ns/{nsId}/resources/customImage/{customImageId} Get customImage
* `DELETE` /ns/{nsId}/resources/customImage/{customImageId} Delete customImage
* `GET` /ns/{nsId}/resources/dataDisk List all Data Disks or Data Disks' ID
* `POST` /ns/{nsId}/resources/dataDisk Create Data Disk
* `DELETE` /ns/{nsId}/resources/dataDisk Delete all Data Disks
* `GET` /ns/{nsId}/resources/dataDisk/{dataDiskId} Get Data Disk
* `PUT` /ns/{nsId}/resources/dataDisk/{dataDiskId} Upsize Data Disk
* `DELETE` /ns/{nsId}/resources/dataDisk/{dataDiskId} Delete Data Disk
* `POST` /systemMcis Create System MCIS Dynamically for Special Purpose in NS:system-purpose-common-ns

### What's Deprecated
---
* `POST` /ns/{nsId}/mcis/{mcisId}/vmgroup Create multiple VMs by VM group in specified MCIS

### What's Changed
---
`GET` /inspectResourcesOverview Inspect Resources Overview (vNet, securityGroup, sshKey, vm) registered in CB-Tumblebug and CSP for all connections  
    Return Type

        Insert cspOnlyOverview.customImage
        Insert cspOnlyOverview.dataDisk
        Insert cspOnlyOverview.nlb
        Insert inspectResult.cspOnlyOverview.customImage
        Insert inspectResult.cspOnlyOverview.dataDisk
        Insert inspectResult.cspOnlyOverview.nlb
        Insert inspectResult.tumblebugOverview.customImage
        Insert inspectResult.tumblebugOverview.dataDisk
        Insert inspectResult.tumblebugOverview.nlb
        Insert tumblebugOverview.customImage
        Insert tumblebugOverview.dataDisk
        Insert tumblebugOverview.nlb
`POST` /mcisRecommendVm Recommend MCIS plan (filter and priority)  
    Parameters

        Modify deploymentPlan.priority.policy.metric //location,cost,latency
        Modify deploymentPlan.priority.policy.weight //0.3
`POST` /ns/{nsId}/benchmark/mcis/{mcisId} Run MCIS benchmark for a single performance metric and return results  
    Return Type

        Insert resultarray.regionName
`POST` /ns/{nsId}/benchmarkAll/mcis/{mcisId} Run MCIS benchmark for all performance metrics and return results  
    Return Type

        Insert resultarray.regionName
`POST` /ns/{nsId}/cmd/mcis/{mcisId} Send a command to specified MCIS  
    Parameters

        Add subGroupId //subGroupId to apply the command only for VMs in subGroup of MCIS
`POST` /ns/{nsId}/cmd/mcis/{mcisId}/vm/{vmId} Send a command to specified VM  
    Parameters

        Modify vmId //VM ID
`GET` /ns/{nsId}/control/mcis/{mcisId}/vm/{vmId} Control the lifecycle of VM (suspend, resume, reboot, terminate)  
    Parameters

        Modify vmId //VM ID
`POST` /ns/{nsId}/installBenchmarkAgent/mcis/{mcisId} Install the benchmark agent to specified MCIS  
    Parameters

        Add option //Option for checking update
`POST` /ns/{nsId}/mcis Create MCIS  
    Parameters

        Insert mcisReq.vm.dataDiskIds
        Insert mcisReq.vm.subGroupSize //if subGroupSize is (not empty) && (> 0), subGroup will be gernetad. VMs will be created accordingly.
        Delete mcisReq.vm.vmGroupSize //if vmGroupSize is (not empty) && (> 0), VM group will be gernetad. VMs will be created accordingly.
        Modify mcisReq.vm.imageId
        Modify mcisReq.vm.name //VM name or VM group name if is (not empty) && (> 0). If it is a group, actual VM name will be generated with -N postfix.
        Modify mcisReq.vm.rootDiskType //"", "default", "TYPE1", AWS: ["standard", "gp2", "gp3"], Azure: ["PremiumSSD", "StandardSSD", "StandardHHD"], GCP: ["pd-standard", "pd-balanced", "pd-ssd", "pd-extreme"], ALIBABA: ["cloud_efficiency", "cloud", "cloud_ssd"], TENCENT: ["CLOUD_PREMIUM", "CLOUD_SSD"]
    Return Type

        Insert newVmList //List of IDs for new VMs. Return IDs if the VMs are newly added. This field should be used for return body only.
        Insert systemMessage //Latest system message such as error message
        Insert vm.connectionConfig
        Insert vm.dataDiskIds
        Insert vm.subGroupId //defined if the VM is in a group
        Insert vm.cspViewVmDetail.dataDiskIIDs
        Insert vm.cspViewVmDetail.dataDiskNames
        Insert vm.cspViewVmDetail.imageType
        Delete vm.vmBlockDisk
        Delete vm.vmBootDisk //ex) /dev/sda1
        Delete vm.vmGroupId //defined if the VM is in a group
        Delete vm.cspViewVmDetail.vmblockDisk //ex)
        Delete vm.cspViewVmDetail.vmbootDisk //Deprecated soon // ex) /dev/sda1
        Modify vm.cspViewVmDetail.startTime //Timezone: based on cloud-barista server location.
`GET` /ns/{nsId}/mcis/{mcisId} Get MCIS object (option: status, accessInfo, vmId)  
    Parameters

        Add filterKey //(For option=id) Field key for filtering (ex: connectionName)
        Add filterVal //(For option=id) Field value for filtering (ex: aws-ap-northeast-2)
        Add accessInfoOption //(For option=accessinfo) accessInfoOption (showSshKey)
        Modify option //Option
`DELETE` /ns/{nsId}/mcis/{mcisId} Delete MCIS  
    Return Type

        Insert output
        Delete message
`POST` /ns/{nsId}/mcis/{mcisId}/vm Create and add homogeneous VMs(subGroup) to a specified MCIS (Set subGroupSize for multiple VMs)  
    Parameters

        Insert vmReq.dataDiskIds
        Insert vmReq.subGroupSize //if subGroupSize is (not empty) && (> 0), subGroup will be gernetad. VMs will be created accordingly.
        vmReq Notes Details for an VM object change into Details for VMs(subGroup)
        Delete vmReq.vmGroupSize //if vmGroupSize is (not empty) && (> 0), VM group will be gernetad. VMs will be created accordingly.
        Modify vmReq.imageId
        Modify vmReq.name //VM name or VM group name if is (not empty) && (> 0). If it is a group, actual VM name will be generated with -N postfix.
        Modify vmReq.rootDiskType //"", "default", "TYPE1", AWS: ["standard", "gp2", "gp3"], Azure: ["PremiumSSD", "StandardSSD", "StandardHHD"], GCP: ["pd-standard", "pd-balanced", "pd-ssd", "pd-extreme"], ALIBABA: ["cloud_efficiency", "cloud", "cloud_ssd"], TENCENT: ["CLOUD_PREMIUM", "CLOUD_SSD"]
    Return Type

        Insert configureCloudAdaptiveNetwork //ConfigureCloudAdaptiveNetwork is an option to configure Cloud Adaptive Network (CLADNet) ([yes/no] default:yes)
        Insert installMonAgent //InstallMonAgent Option for CB-Dragonfly agent installation ([yes/no] default:yes)
        Insert newVmList //List of IDs for new VMs. Return IDs if the VMs are newly added. This field should be used for return body only.
        Insert placementAlgo
        Insert statusCount
        Insert systemLabel //SystemLabel is for describing the mcis in a keyword (any string can be used) for special System purpose
        Insert vm
        Delete connectionName
        Delete createdTime //Created time
        Delete cspViewVmDetail
        Delete idByCSP //CSP managed ID or Name
        Delete imageId
        Delete location
        Delete monAgentStatus //Montoring agent status
        Delete networkAgentStatus //NetworkAgent status
        Delete privateDNS
        Delete privateIP
        Delete publicDNS
        Delete publicIP
        Delete region //AWS, ex) {us-east1, us-east1-c} or {ap-northeast-2}
        Delete rootDeviceName
        Delete rootDiskSize
        Delete rootDiskType
        Delete securityGroupIds
        Delete specId
        Delete sshKeyId
        Delete sshPort
        Delete subnetId
        Delete vNetId
        Delete vmBlockDisk
        Delete vmBootDisk //ex) /dev/sda1
        Delete vmGroupId //defined if the VM is in a group
        Delete vmUserAccount
        Delete vmUserPassword
        Modify label
        Modify status //Required by CB-Tumblebug
`GET` /ns/{nsId}/mcis/{mcisId}/vm/{vmId} Get VM in specified MCIS  
    Parameters

        Modify vmId //VM ID
        Modify option //Option for MCIS
`DELETE` /ns/{nsId}/mcis/{mcisId}/vm/{vmId} Delete VM in specified MCIS  
    Parameters

        Modify vmId //VM ID
`POST` /ns/{nsId}/mcisDynamic Create MCIS Dynamically  
    Parameters

        Insert mcisReq.vm.subGroupSize //if subGroupSize is (not empty) && (> 0), subGroup will be gernetad. VMs will be created accordingly.
        Delete mcisReq.vm.vmGroupSize //if vmGroupSize is (not empty) && (> 0), VM group will be gernetad. VMs will be created accordingly.
        Modify mcisReq.vm.name //VM name or VM group name if is (not empty) && (> 0). If it is a group, actual VM name will be generated with -N postfix.
        Modify mcisReq.vm.rootDiskType //"", "default", "TYPE1", AWS: ["standard", "gp2", "gp3"], Azure: ["PremiumSSD", "StandardSSD", "StandardHHD"], GCP: ["pd-standard", "pd-balanced", "pd-ssd", "pd-extreme"], ALIBABA: ["cloud_efficiency", "cloud", "cloud_essd"], TENCENT: ["CLOUD_PREMIUM", "CLOUD_SSD"]
    Return Type

        Insert newVmList //List of IDs for new VMs. Return IDs if the VMs are newly added. This field should be used for return body only.
        Insert systemMessage //Latest system message such as error message
        Insert vm.connectionConfig
        Insert vm.dataDiskIds
        Insert vm.subGroupId //defined if the VM is in a group
        Insert vm.cspViewVmDetail.dataDiskIIDs
        Insert vm.cspViewVmDetail.dataDiskNames
        Insert vm.cspViewVmDetail.imageType
        Delete vm.vmBlockDisk
        Delete vm.vmBootDisk //ex) /dev/sda1
        Delete vm.vmGroupId //defined if the VM is in a group
        Delete vm.cspViewVmDetail.vmblockDisk //ex)
        Delete vm.cspViewVmDetail.vmbootDisk //Deprecated soon // ex) /dev/sda1
        Modify vm.cspViewVmDetail.startTime //Timezone: based on cloud-barista server location.
`GET` /ns/{nsId}/policy/mcis List all MCIS policies  
    Return Type

        Insert mcisPolicy.policy.autoAction.vmDynamicReq
        Delete mcisPolicy.policy.autoAction.vm
        Modify mcisPolicy.description
        Modify mcisPolicy.policy.autoAction.actionType
        Modify mcisPolicy.policy.autoAction.placementAlgo
        Modify mcisPolicy.policy.autoCondition.evaluationPeriod //evaluationPeriod
        Modify mcisPolicy.policy.autoCondition.metric
        Modify mcisPolicy.policy.autoCondition.operand //10, 70, 80, 98, ...
        Modify mcisPolicy.policy.autoCondition.operator //<, <=, >, >=, ...
`POST` /ns/{nsId}/policy/mcis/{mcisId} Create MCIS Automation policy  
    Parameters

        Add mcisPolicyReq //Details for an MCIS automation policy request
        Delete mcisInfo //Details for an MCIS object
    Return Type

        Insert policy.autoAction.vmDynamicReq
        Delete policy.autoAction.vm
        Modify description
        Modify policy.autoAction.actionType
        Modify policy.autoAction.placementAlgo
        Modify policy.autoCondition.evaluationPeriod //evaluationPeriod
        Modify policy.autoCondition.metric
        Modify policy.autoCondition.operand //10, 70, 80, 98, ...
        Modify policy.autoCondition.operator //<, <=, >, >=, ...
`GET` /ns/{nsId}/policy/mcis/{mcisId} Get MCIS Policy  
    Return Type

        Insert policy.autoAction.vmDynamicReq
        Delete policy.autoAction.vm
        Modify description
        Modify policy.autoAction.actionType
        Modify policy.autoAction.placementAlgo
        Modify policy.autoCondition.evaluationPeriod //evaluationPeriod
        Modify policy.autoCondition.metric
        Modify policy.autoCondition.operand //10, 70, 80, 98, ...
        Modify policy.autoCondition.operator //<, <=, >, >=, ...
`POST` /ns/{nsId}/registerCspVm Register existing VM in a CSP to Cloud-Barista MCIS  
    Parameters

        Insert mcisReq.vm.dataDiskIds
        Insert mcisReq.vm.subGroupSize //if subGroupSize is (not empty) && (> 0), subGroup will be gernetad. VMs will be created accordingly.
        Delete mcisReq.vm.vmGroupSize //if vmGroupSize is (not empty) && (> 0), VM group will be gernetad. VMs will be created accordingly.
        Modify mcisReq.vm.imageId
        Modify mcisReq.vm.name //VM name or VM group name if is (not empty) && (> 0). If it is a group, actual VM name will be generated with -N postfix.
        Modify mcisReq.vm.rootDiskType //"", "default", "TYPE1", AWS: ["standard", "gp2", "gp3"], Azure: ["PremiumSSD", "StandardSSD", "StandardHHD"], GCP: ["pd-standard", "pd-balanced", "pd-ssd", "pd-extreme"], ALIBABA: ["cloud_efficiency", "cloud", "cloud_ssd"], TENCENT: ["CLOUD_PREMIUM", "CLOUD_SSD"]
    Return Type

        Insert newVmList //List of IDs for new VMs. Return IDs if the VMs are newly added. This field should be used for return body only.
        Insert systemMessage //Latest system message such as error message
        Insert vm.connectionConfig
        Insert vm.dataDiskIds
        Insert vm.subGroupId //defined if the VM is in a group
        Insert vm.cspViewVmDetail.dataDiskIIDs
        Insert vm.cspViewVmDetail.dataDiskNames
        Insert vm.cspViewVmDetail.imageType
        Delete vm.vmBlockDisk
        Delete vm.vmBootDisk //ex) /dev/sda1
        Delete vm.vmGroupId //defined if the VM is in a group
        Delete vm.cspViewVmDetail.vmblockDisk //ex)
        Delete vm.cspViewVmDetail.vmbootDisk //Deprecated soon // ex) /dev/sda1
        Modify vm.cspViewVmDetail.startTime //Timezone: based on cloud-barista server location.
`POST` /registerCspResources Register CSP Native Resources (vNet, securityGroup, sshKey, vm) to CB-Tumblebug  
    Return Type

        Insert registerationOverview.customImage
        Insert registerationOverview.dataDisk
        Insert registerationOverview.nlb
`POST` /registerCspResourcesAll Register CSP Native Resources (vNet, securityGroup, sshKey, vm) from all Clouds to CB-Tumblebug  
    Return Type

        Insert registerationOverview.customImage
        Insert registerationOverview.dataDisk
        Insert registerationOverview.nlb
        Insert registerationResult.registerationOverview.customImage
        Insert registerationResult.registerationOverview.dataDisk
        Insert registerationResult.registerationOverview.nlb

