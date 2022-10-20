## Version latest to latest
---
### What's New
---
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
`POST` /ns/{nsId}/cmd/mcis/{mcisId}/vm/{vmId} Send a command to specified VM  
    Parameters

        Modify vmId //VM ID
`GET` /ns/{nsId}/control/mcis/{mcisId}/vm/{vmId} Control the lifecycle of VM (suspend, resume, reboot, terminate)  
    Parameters

        Modify vmId //VM ID
`POST` /ns/{nsId}/mcis Create MCIS  
    Parameters

        Insert mcisReq.vm.dataDiskIds
        Insert mcisReq.vm.subGroupSize //if subGroupSize is (not empty) && (> 0), subGroup will be gernetad. VMs will be created accordingly.
        Delete mcisReq.vm.vmGroupSize //if vmGroupSize is (not empty) && (> 0), VM group will be gernetad. VMs will be created accordingly.
        Modify mcisReq.vm.imageId
        Modify mcisReq.vm.name //VM name or VM group name if is (not empty) && (> 0). If it is a group, actual VM name will be generated with -N postfix.
        Modify mcisReq.vm.rootDiskType //"", "default", "TYPE1", AWS: ["standard", "gp2", "gp3"], Azure: ["PremiumSSD", "StandardSSD", "StandardHHD"], GCP: ["pd-standard", "pd-balanced", "pd-ssd", "pd-extreme"], ALIBABA: ["cloud_efficiency", "cloud", "cloud_ssd"], TENCENT: ["CLOUD_PREMIUM", "CLOUD_SSD"]
    Return Type

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
        Delete systemMessage //Latest system message such as error message
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

        Insert mcisPolicy.policy.autoAction.vm.dataDiskIds
        Insert mcisPolicy.policy.autoAction.vm.subGroupId //defined if the VM is in a group
        Insert mcisPolicy.policy.autoAction.vm.cspViewVmDetail.dataDiskIIDs
        Insert mcisPolicy.policy.autoAction.vm.cspViewVmDetail.dataDiskNames
        Insert mcisPolicy.policy.autoAction.vm.cspViewVmDetail.imageType
        Delete mcisPolicy.policy.autoAction.vm.vmBlockDisk
        Delete mcisPolicy.policy.autoAction.vm.vmBootDisk //ex) /dev/sda1
        Delete mcisPolicy.policy.autoAction.vm.vmGroupId //defined if the VM is in a group
        Delete mcisPolicy.policy.autoAction.vm.cspViewVmDetail.vmblockDisk //ex)
        Delete mcisPolicy.policy.autoAction.vm.cspViewVmDetail.vmbootDisk //Deprecated soon // ex) /dev/sda1
        Modify mcisPolicy.policy.autoAction.vm.cspViewVmDetail.startTime //Timezone: based on cloud-barista server location.
`GET` /ns/{nsId}/policy/mcis/{mcisId} Get MCIS Policy  
    Return Type

        Insert policy.autoAction.vm.dataDiskIds
        Insert policy.autoAction.vm.subGroupId //defined if the VM is in a group
        Insert policy.autoAction.vm.cspViewVmDetail.dataDiskIIDs
        Insert policy.autoAction.vm.cspViewVmDetail.dataDiskNames
        Insert policy.autoAction.vm.cspViewVmDetail.imageType
        Delete policy.autoAction.vm.vmBlockDisk
        Delete policy.autoAction.vm.vmBootDisk //ex) /dev/sda1
        Delete policy.autoAction.vm.vmGroupId //defined if the VM is in a group
        Delete policy.autoAction.vm.cspViewVmDetail.vmblockDisk //ex)
        Delete policy.autoAction.vm.cspViewVmDetail.vmbootDisk //Deprecated soon // ex) /dev/sda1
        Modify policy.autoAction.vm.cspViewVmDetail.startTime //Timezone: based on cloud-barista server location.
`POST` /ns/{nsId}/policy/mcis/{mcisId} Create MCIS Automation policy  
    Parameters

        Insert mcisInfo.policy.autoAction.vm.dataDiskIds
        Insert mcisInfo.policy.autoAction.vm.subGroupId //defined if the VM is in a group
        Insert mcisInfo.policy.autoAction.vm.cspViewVmDetail.dataDiskIIDs
        Insert mcisInfo.policy.autoAction.vm.cspViewVmDetail.dataDiskNames
        Insert mcisInfo.policy.autoAction.vm.cspViewVmDetail.imageType
        Delete mcisInfo.policy.autoAction.vm.vmBlockDisk
        Delete mcisInfo.policy.autoAction.vm.vmBootDisk //ex) /dev/sda1
        Delete mcisInfo.policy.autoAction.vm.vmGroupId //defined if the VM is in a group
        Delete mcisInfo.policy.autoAction.vm.cspViewVmDetail.vmblockDisk //ex)
        Delete mcisInfo.policy.autoAction.vm.cspViewVmDetail.vmbootDisk //Deprecated soon // ex) /dev/sda1
        Modify mcisInfo.policy.autoAction.vm.cspViewVmDetail.startTime //Timezone: based on cloud-barista server location.
    Return Type

        Insert policy.autoAction.vm.dataDiskIds
        Insert policy.autoAction.vm.subGroupId //defined if the VM is in a group
        Insert policy.autoAction.vm.cspViewVmDetail.dataDiskIIDs
        Insert policy.autoAction.vm.cspViewVmDetail.dataDiskNames
        Insert policy.autoAction.vm.cspViewVmDetail.imageType
        Delete policy.autoAction.vm.vmBlockDisk
        Delete policy.autoAction.vm.vmBootDisk //ex) /dev/sda1
        Delete policy.autoAction.vm.vmGroupId //defined if the VM is in a group
        Delete policy.autoAction.vm.cspViewVmDetail.vmblockDisk //ex)
        Delete policy.autoAction.vm.cspViewVmDetail.vmbootDisk //Deprecated soon // ex) /dev/sda1
        Modify policy.autoAction.vm.cspViewVmDetail.startTime //Timezone: based on cloud-barista server location.
`POST` /ns/{nsId}/registerCspVm Register existing VM in a CSP to Cloud-Barista MCIS  
    Parameters

        Insert mcisReq.vm.dataDiskIds
        Insert mcisReq.vm.subGroupSize //if subGroupSize is (not empty) && (> 0), subGroup will be gernetad. VMs will be created accordingly.
        Delete mcisReq.vm.vmGroupSize //if vmGroupSize is (not empty) && (> 0), VM group will be gernetad. VMs will be created accordingly.
        Modify mcisReq.vm.imageId
        Modify mcisReq.vm.name //VM name or VM group name if is (not empty) && (> 0). If it is a group, actual VM name will be generated with -N postfix.
        Modify mcisReq.vm.rootDiskType //"", "default", "TYPE1", AWS: ["standard", "gp2", "gp3"], Azure: ["PremiumSSD", "StandardSSD", "StandardHHD"], GCP: ["pd-standard", "pd-balanced", "pd-ssd", "pd-extreme"], ALIBABA: ["cloud_efficiency", "cloud", "cloud_ssd"], TENCENT: ["CLOUD_PREMIUM", "CLOUD_SSD"]
    Return Type

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

