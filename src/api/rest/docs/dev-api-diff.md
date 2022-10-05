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
* `PUT` /ns/{nsId}/mcis/{mcisId}/vm/{vmId}/{command} Attach/Detach data disk to/from VM
* `POST` /ns/{nsId}/mcis/{mcisId}/vm/{vmId}/{command} Create VM snapshot
* `GET` /ns/{nsId}/mcis/{mcisId}/vmgroup/{vmgroupId} List VMs with a VMGroup label in a specified MCIS
* `POST` /ns/{nsId}/mcis/{mcisId}/vmgroup/{vmgroupId} ScaleOut VM group in specified MCIS
* `POST` /ns/{nsId}/resources/customImage Create Custom Image
* `GET` /ns/{nsId}/resources/dataDisk List all Data Disks or Data Disks' ID
* `POST` /ns/{nsId}/resources/dataDisk Create Data Disk
* `DELETE` /ns/{nsId}/resources/dataDisk Delete all Data Disks
* `GET` /ns/{nsId}/resources/dataDisk/{dataDiskId} Get Data Disk
* `PUT` /ns/{nsId}/resources/dataDisk/{dataDiskId} Upsize Data Disk
* `DELETE` /ns/{nsId}/resources/dataDisk/{dataDiskId} Delete Data Disk
* `GET` /ns/{nsId}/mcis/{mcisId}/vmgroup List VMGroup IDs in a specified MCIS

### What's Deprecated
---

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
`POST` /ns/{nsId}/mcis Create MCIS  
    Parameters

        Insert mcisReq.vm.dataDiskIds
        Modify mcisReq.vm.rootDiskType //"", "default", "TYPE1", AWS: ["standard", "gp2", "gp3"], Azure: ["PremiumSSD", "StandardSSD", "StandardHHD"], GCP: ["pd-standard", "pd-balanced", "pd-ssd", "pd-extreme"], ALIBABA: ["cloud_efficiency", "cloud", "cloud_ssd"], TENCENT: ["CLOUD_PREMIUM", "CLOUD_SSD"]
    Return Type

        Insert vm.dataDiskIds
        Insert vm.cspViewVmDetail.dataDiskIIDs
        Insert vm.cspViewVmDetail.dataDiskNames
        Delete vm.vmBlockDisk
        Delete vm.vmBootDisk //ex) /dev/sda1
        Delete vm.cspViewVmDetail.vmblockDisk //ex)
        Delete vm.cspViewVmDetail.vmbootDisk //Deprecated soon // ex) /dev/sda1
        Modify vm.cspViewVmDetail.startTime //Timezone: based on cloud-barista server location.
`POST` /ns/{nsId}/mcis/{mcisId}/vm Create VM in specified MCIS  
    Parameters

        Insert vmReq.dataDiskIds
        Modify vmReq.rootDiskType //"", "default", "TYPE1", AWS: ["standard", "gp2", "gp3"], Azure: ["PremiumSSD", "StandardSSD", "StandardHHD"], GCP: ["pd-standard", "pd-balanced", "pd-ssd", "pd-extreme"], ALIBABA: ["cloud_efficiency", "cloud", "cloud_ssd"], TENCENT: ["CLOUD_PREMIUM", "CLOUD_SSD"]
    Return Type

        Insert dataDiskIds
        Insert cspViewVmDetail.dataDiskIIDs
        Insert cspViewVmDetail.dataDiskNames
        Delete vmBlockDisk
        Delete vmBootDisk //ex) /dev/sda1
        Delete cspViewVmDetail.vmblockDisk //ex)
        Delete cspViewVmDetail.vmbootDisk //Deprecated soon // ex) /dev/sda1
        Modify cspViewVmDetail.startTime //Timezone: based on cloud-barista server location.
`GET` /ns/{nsId}/mcis/{mcisId}/vm/{vmId} Get VM in specified MCIS  
    Parameters

        Modify option //Option for MCIS
`POST` /ns/{nsId}/mcis/{mcisId}/vmgroup Create multiple VMs by VM group in specified MCIS  
    Parameters

        Insert vmReq.dataDiskIds
        Modify vmReq.rootDiskType //"", "default", "TYPE1", AWS: ["standard", "gp2", "gp3"], Azure: ["PremiumSSD", "StandardSSD", "StandardHHD"], GCP: ["pd-standard", "pd-balanced", "pd-ssd", "pd-extreme"], ALIBABA: ["cloud_efficiency", "cloud", "cloud_ssd"], TENCENT: ["CLOUD_PREMIUM", "CLOUD_SSD"]
    Return Type

        Insert vm.dataDiskIds
        Insert vm.cspViewVmDetail.dataDiskIIDs
        Insert vm.cspViewVmDetail.dataDiskNames
        Delete vm.vmBlockDisk
        Delete vm.vmBootDisk //ex) /dev/sda1
        Delete vm.cspViewVmDetail.vmblockDisk //ex)
        Delete vm.cspViewVmDetail.vmbootDisk //Deprecated soon // ex) /dev/sda1
        Modify vm.cspViewVmDetail.startTime //Timezone: based on cloud-barista server location.
`POST` /ns/{nsId}/mcisDynamic Create MCIS Dynamically  
    Parameters

        Modify mcisReq.vm.rootDiskType //"", "default", "TYPE1", AWS: ["standard", "gp2", "gp3"], Azure: ["PremiumSSD", "StandardSSD", "StandardHHD"], GCP: ["pd-standard", "pd-balanced", "pd-ssd", "pd-extreme"], ALIBABA: ["cloud_efficiency", "cloud", "cloud_essd"], TENCENT: ["CLOUD_PREMIUM", "CLOUD_SSD"]
    Return Type

        Insert vm.dataDiskIds
        Insert vm.cspViewVmDetail.dataDiskIIDs
        Insert vm.cspViewVmDetail.dataDiskNames
        Delete vm.vmBlockDisk
        Delete vm.vmBootDisk //ex) /dev/sda1
        Delete vm.cspViewVmDetail.vmblockDisk //ex)
        Delete vm.cspViewVmDetail.vmbootDisk //Deprecated soon // ex) /dev/sda1
        Modify vm.cspViewVmDetail.startTime //Timezone: based on cloud-barista server location.
`GET` /ns/{nsId}/policy/mcis List all MCIS policies  
    Return Type

        Insert mcisPolicy.policy.autoAction.vm.dataDiskIds
        Insert mcisPolicy.policy.autoAction.vm.cspViewVmDetail.dataDiskIIDs
        Insert mcisPolicy.policy.autoAction.vm.cspViewVmDetail.dataDiskNames
        Delete mcisPolicy.policy.autoAction.vm.vmBlockDisk
        Delete mcisPolicy.policy.autoAction.vm.vmBootDisk //ex) /dev/sda1
        Delete mcisPolicy.policy.autoAction.vm.cspViewVmDetail.vmblockDisk //ex)
        Delete mcisPolicy.policy.autoAction.vm.cspViewVmDetail.vmbootDisk //Deprecated soon // ex) /dev/sda1
        Modify mcisPolicy.policy.autoAction.vm.cspViewVmDetail.startTime //Timezone: based on cloud-barista server location.
`GET` /ns/{nsId}/policy/mcis/{mcisId} Get MCIS Policy  
    Return Type

        Insert policy.autoAction.vm.dataDiskIds
        Insert policy.autoAction.vm.cspViewVmDetail.dataDiskIIDs
        Insert policy.autoAction.vm.cspViewVmDetail.dataDiskNames
        Delete policy.autoAction.vm.vmBlockDisk
        Delete policy.autoAction.vm.vmBootDisk //ex) /dev/sda1
        Delete policy.autoAction.vm.cspViewVmDetail.vmblockDisk //ex)
        Delete policy.autoAction.vm.cspViewVmDetail.vmbootDisk //Deprecated soon // ex) /dev/sda1
        Modify policy.autoAction.vm.cspViewVmDetail.startTime //Timezone: based on cloud-barista server location.
`POST` /ns/{nsId}/policy/mcis/{mcisId} Create MCIS Automation policy  
    Parameters

        Insert mcisInfo.policy.autoAction.vm.dataDiskIds
        Insert mcisInfo.policy.autoAction.vm.cspViewVmDetail.dataDiskIIDs
        Insert mcisInfo.policy.autoAction.vm.cspViewVmDetail.dataDiskNames
        Delete mcisInfo.policy.autoAction.vm.vmBlockDisk
        Delete mcisInfo.policy.autoAction.vm.vmBootDisk //ex) /dev/sda1
        Delete mcisInfo.policy.autoAction.vm.cspViewVmDetail.vmblockDisk //ex)
        Delete mcisInfo.policy.autoAction.vm.cspViewVmDetail.vmbootDisk //Deprecated soon // ex) /dev/sda1
        Modify mcisInfo.policy.autoAction.vm.cspViewVmDetail.startTime //Timezone: based on cloud-barista server location.
    Return Type

        Insert policy.autoAction.vm.dataDiskIds
        Insert policy.autoAction.vm.cspViewVmDetail.dataDiskIIDs
        Insert policy.autoAction.vm.cspViewVmDetail.dataDiskNames
        Delete policy.autoAction.vm.vmBlockDisk
        Delete policy.autoAction.vm.vmBootDisk //ex) /dev/sda1
        Delete policy.autoAction.vm.cspViewVmDetail.vmblockDisk //ex)
        Delete policy.autoAction.vm.cspViewVmDetail.vmbootDisk //Deprecated soon // ex) /dev/sda1
        Modify policy.autoAction.vm.cspViewVmDetail.startTime //Timezone: based on cloud-barista server location.
`POST` /ns/{nsId}/registerCspVm Register existing VM in a CSP to Cloud-Barista MCIS  
    Parameters

        Insert mcisReq.vm.dataDiskIds
        Modify mcisReq.vm.rootDiskType //"", "default", "TYPE1", AWS: ["standard", "gp2", "gp3"], Azure: ["PremiumSSD", "StandardSSD", "StandardHHD"], GCP: ["pd-standard", "pd-balanced", "pd-ssd", "pd-extreme"], ALIBABA: ["cloud_efficiency", "cloud", "cloud_ssd"], TENCENT: ["CLOUD_PREMIUM", "CLOUD_SSD"]
    Return Type

        Insert vm.dataDiskIds
        Insert vm.cspViewVmDetail.dataDiskIIDs
        Insert vm.cspViewVmDetail.dataDiskNames
        Delete vm.vmBlockDisk
        Delete vm.vmBootDisk //ex) /dev/sda1
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

