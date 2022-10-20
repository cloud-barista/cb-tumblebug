## Version latest to latest
---
### What's New
---
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

### What's Deprecated
---
* `PUT` /ns/{nsId}/mcis/{mcisId}/vm/{vmId}/{command} Attach/Detach data disk to/from VM
* `GET` /ns/{nsId}/mcis/{mcisId}/vmgroup List VMGroup IDs in a specified MCIS
* `POST` /ns/{nsId}/mcis/{mcisId}/vmgroup Create multiple VMs by VM group in specified MCIS
* `GET` /ns/{nsId}/mcis/{mcisId}/vmgroup/{vmgroupId} List VMs with a VMGroup label in a specified MCIS
* `POST` /ns/{nsId}/mcis/{mcisId}/vmgroup/{vmgroupId} ScaleOut VM group in specified MCIS

### What's Changed
---
`GET` /inspectResourcesOverview Inspect Resources Overview (vNet, securityGroup, sshKey, vm) registered in CB-Tumblebug and CSP for all connections  
    Return Type

        Insert cspOnlyOverview.customImage
        Insert inspectResult.cspOnlyOverview.customImage
        Insert inspectResult.tumblebugOverview.customImage
        Insert tumblebugOverview.customImage
`POST` /ns/{nsId}/cmd/mcis/{mcisId}/vm/{vmId} Send a command to specified VM  
    Parameters

        Modify vmId //VM ID
`GET` /ns/{nsId}/control/mcis/{mcisId}/vm/{vmId} Control the lifecycle of VM (suspend, resume, reboot, terminate)  
    Parameters

        Modify vmId //VM ID
`POST` /ns/{nsId}/mcis Create MCIS  
    Parameters

        Insert mcisReq.vm.subGroupSize //if subGroupSize is (not empty) && (> 0), subGroup will be gernetad. VMs will be created accordingly.
        Delete mcisReq.vm.vmGroupSize //if vmGroupSize is (not empty) && (> 0), VM group will be gernetad. VMs will be created accordingly.
        Modify mcisReq.vm.imageId
        Modify mcisReq.vm.name //VM name or VM group name if is (not empty) && (> 0). If it is a group, actual VM name will be generated with -N postfix.
    Return Type

        Insert vm.subGroupId //defined if the VM is in a group
        Insert vm.cspViewVmDetail.imageType
        Delete vm.vmGroupId //defined if the VM is in a group
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
`POST` /ns/{nsId}/mcis/{mcisId}/nlb Create NLB  
    Parameters

        Delete nlbReq.connectionName
        Delete nlbReq.name
        Delete nlbReq.vNetId
        Delete nlbReq.healthChecker.interval //secs, Interval time between health checks.
        Delete nlbReq.healthChecker.port //Listener Port or 1-65535
        Delete nlbReq.healthChecker.protocol //TCP|HTTP|HTTPS
        Delete nlbReq.healthChecker.threshold //num, The number of continuous health checks to change the VM status.
        Delete nlbReq.healthChecker.timeout //secs, Waiting time to decide an unhealthy VM when no response.
        Delete nlbReq.listener.cspID //Optional, May be Used by Driver.
        Delete nlbReq.listener.dnsName //Optional, Auto Generated and attached
        Delete nlbReq.listener.ip //Auto Generated and attached
        Delete nlbReq.listener.keyValueList
        Delete nlbReq.listener.port //1-65535
        Delete nlbReq.listener.protocol //TCP|UDP
        Delete nlbReq.targetGroup.cspID //Optional, May be Used by Driver.
        Delete nlbReq.targetGroup.keyValueList
        Delete nlbReq.targetGroup.port //Listener Port or 1-65535
        Delete nlbReq.targetGroup.protocol //TCP|HTTP|HTTPS
        Delete nlbReq.targetGroup.vmGroupId
        Delete nlbReq.targetGroup.vms
        Modify nlbReq.cspNLBId
    Return Type

        Delete healthChecker.cspID //Optional, May be Used by Driver.
        Delete healthChecker.interval //secs, Interval time between health checks.
        Delete healthChecker.keyValueList
        Delete healthChecker.port //Listener Port or 1-65535
        Delete healthChecker.protocol //TCP|HTTP|HTTPS
        Delete healthChecker.threshold //num, The number of continuous health checks to change the VM status.
        Delete healthChecker.timeout //secs, Waiting time to decide an unhealthy VM when no response.
        Delete listener.cspID //Optional, May be Used by Driver.
        Delete listener.dnsName //Optional, Auto Generated and attached
        Delete listener.ip //Auto Generated and attached
        Delete listener.keyValueList
        Delete listener.port //1-65535
        Delete listener.protocol //TCP|UDP
        Delete targetGroup.cspID //Optional, May be Used by Driver.
        Delete targetGroup.keyValueList
        Delete targetGroup.port //Listener Port or 1-65535
        Delete targetGroup.protocol //TCP|HTTP|HTTPS
        Delete targetGroup.vmGroupId
        Delete targetGroup.vms
`GET` /ns/{nsId}/mcis/{mcisId}/nlb/{nlbId} Get NLB  
    Parameters

        Modify nlbId //NLB ID
    Return Type

        Delete healthChecker.cspID //Optional, May be Used by Driver.
        Delete healthChecker.interval //secs, Interval time between health checks.
        Delete healthChecker.keyValueList
        Delete healthChecker.port //Listener Port or 1-65535
        Delete healthChecker.protocol //TCP|HTTP|HTTPS
        Delete healthChecker.threshold //num, The number of continuous health checks to change the VM status.
        Delete healthChecker.timeout //secs, Waiting time to decide an unhealthy VM when no response.
        Delete listener.cspID //Optional, May be Used by Driver.
        Delete listener.dnsName //Optional, Auto Generated and attached
        Delete listener.ip //Auto Generated and attached
        Delete listener.keyValueList
        Delete listener.port //1-65535
        Delete listener.protocol //TCP|UDP
        Delete targetGroup.cspID //Optional, May be Used by Driver.
        Delete targetGroup.keyValueList
        Delete targetGroup.port //Listener Port or 1-65535
        Delete targetGroup.protocol //TCP|HTTP|HTTPS
        Delete targetGroup.vmGroupId
        Delete targetGroup.vms
`GET` /ns/{nsId}/mcis/{mcisId}/nlb/{nlbId}/healthz Get NLB Health  
    Parameters

        Modify nlbId //NLB ID
    Return Type

        Delete healthChecker.cspID //Optional, May be Used by Driver.
        Delete healthChecker.interval //secs, Interval time between health checks.
        Delete healthChecker.keyValueList
        Delete healthChecker.port //Listener Port or 1-65535
        Delete healthChecker.protocol //TCP|HTTP|HTTPS
        Delete healthChecker.threshold //num, The number of continuous health checks to change the VM status.
        Delete healthChecker.timeout //secs, Waiting time to decide an unhealthy VM when no response.
        Delete listener.cspID //Optional, May be Used by Driver.
        Delete listener.dnsName //Optional, Auto Generated and attached
        Delete listener.ip //Auto Generated and attached
        Delete listener.keyValueList
        Delete listener.port //1-65535
        Delete listener.protocol //TCP|UDP
        Delete targetGroup.cspID //Optional, May be Used by Driver.
        Delete targetGroup.keyValueList
        Delete targetGroup.port //Listener Port or 1-65535
        Delete targetGroup.protocol //TCP|HTTP|HTTPS
        Delete targetGroup.vmGroupId
        Delete targetGroup.vms
`POST` /ns/{nsId}/mcis/{mcisId}/nlb/{nlbId}/vm Add VMs to NLB  
    Parameters

        Delete nlbAddRemoveVMReq.targetGroup.cspID //Optional, May be Used by Driver.
        Delete nlbAddRemoveVMReq.targetGroup.keyValueList
        Delete nlbAddRemoveVMReq.targetGroup.port //Listener Port or 1-65535
        Delete nlbAddRemoveVMReq.targetGroup.protocol //TCP|HTTP|HTTPS
        Delete nlbAddRemoveVMReq.targetGroup.vmGroupId
        Delete nlbAddRemoveVMReq.targetGroup.vms
        Modify nlbId //NLB ID
    Return Type

        Delete healthChecker.cspID //Optional, May be Used by Driver.
        Delete healthChecker.interval //secs, Interval time between health checks.
        Delete healthChecker.keyValueList
        Delete healthChecker.port //Listener Port or 1-65535
        Delete healthChecker.protocol //TCP|HTTP|HTTPS
        Delete healthChecker.threshold //num, The number of continuous health checks to change the VM status.
        Delete healthChecker.timeout //secs, Waiting time to decide an unhealthy VM when no response.
        Delete listener.cspID //Optional, May be Used by Driver.
        Delete listener.dnsName //Optional, Auto Generated and attached
        Delete listener.ip //Auto Generated and attached
        Delete listener.keyValueList
        Delete listener.port //1-65535
        Delete listener.protocol //TCP|UDP
        Delete targetGroup.cspID //Optional, May be Used by Driver.
        Delete targetGroup.keyValueList
        Delete targetGroup.port //Listener Port or 1-65535
        Delete targetGroup.protocol //TCP|HTTP|HTTPS
        Delete targetGroup.vmGroupId
        Delete targetGroup.vms
`DELETE` /ns/{nsId}/mcis/{mcisId}/nlb/{nlbId}/vm Delete VMs from NLB  
    Parameters

        Delete nlbAddRemoveVMReq.targetGroup.cspID //Optional, May be Used by Driver.
        Delete nlbAddRemoveVMReq.targetGroup.keyValueList
        Delete nlbAddRemoveVMReq.targetGroup.port //Listener Port or 1-65535
        Delete nlbAddRemoveVMReq.targetGroup.protocol //TCP|HTTP|HTTPS
        Delete nlbAddRemoveVMReq.targetGroup.vmGroupId
        Delete nlbAddRemoveVMReq.targetGroup.vms
        Modify nlbId //NLB ID
`POST` /ns/{nsId}/mcis/{mcisId}/vm Create and add homogeneous VMs(subGroup) to a specified MCIS (Set subGroupSize for multiple VMs)  
    Parameters

        Insert vmReq.subGroupSize //if subGroupSize is (not empty) && (> 0), subGroup will be gernetad. VMs will be created accordingly.
        vmReq Notes Details for an VM object change into Details for VMs(subGroup)
        Delete vmReq.vmGroupSize //if vmGroupSize is (not empty) && (> 0), VM group will be gernetad. VMs will be created accordingly.
        Modify vmReq.imageId
        Modify vmReq.name //VM name or VM group name if is (not empty) && (> 0). If it is a group, actual VM name will be generated with -N postfix.
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
        Delete dataDiskIds
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
        Delete vmGroupId //defined if the VM is in a group
        Delete vmUserAccount
        Delete vmUserPassword
        Modify label
        Modify status //Required by CB-Tumblebug
`GET` /ns/{nsId}/mcis/{mcisId}/vm/{vmId} Get VM in specified MCIS  
    Parameters

        Modify vmId //VM ID
`DELETE` /ns/{nsId}/mcis/{mcisId}/vm/{vmId} Delete VM in specified MCIS  
    Parameters

        Modify vmId //VM ID
`POST` /ns/{nsId}/mcisDynamic Create MCIS Dynamically  
    Parameters

        Insert mcisReq.vm.subGroupSize //if subGroupSize is (not empty) && (> 0), subGroup will be gernetad. VMs will be created accordingly.
        Delete mcisReq.vm.vmGroupSize //if vmGroupSize is (not empty) && (> 0), VM group will be gernetad. VMs will be created accordingly.
        Modify mcisReq.vm.name //VM name or VM group name if is (not empty) && (> 0). If it is a group, actual VM name will be generated with -N postfix.
    Return Type

        Insert vm.subGroupId //defined if the VM is in a group
        Insert vm.cspViewVmDetail.imageType
        Delete vm.vmGroupId //defined if the VM is in a group
`GET` /ns/{nsId}/policy/mcis List all MCIS policies  
    Return Type

        Insert mcisPolicy.policy.autoAction.vm.subGroupId //defined if the VM is in a group
        Insert mcisPolicy.policy.autoAction.vm.cspViewVmDetail.imageType
        Delete mcisPolicy.policy.autoAction.vm.vmGroupId //defined if the VM is in a group
`GET` /ns/{nsId}/policy/mcis/{mcisId} Get MCIS Policy  
    Return Type

        Insert policy.autoAction.vm.subGroupId //defined if the VM is in a group
        Insert policy.autoAction.vm.cspViewVmDetail.imageType
        Delete policy.autoAction.vm.vmGroupId //defined if the VM is in a group
`POST` /ns/{nsId}/policy/mcis/{mcisId} Create MCIS Automation policy  
    Parameters

        Insert mcisInfo.policy.autoAction.vm.subGroupId //defined if the VM is in a group
        Insert mcisInfo.policy.autoAction.vm.cspViewVmDetail.imageType
        Delete mcisInfo.policy.autoAction.vm.vmGroupId //defined if the VM is in a group
    Return Type

        Insert policy.autoAction.vm.subGroupId //defined if the VM is in a group
        Insert policy.autoAction.vm.cspViewVmDetail.imageType
        Delete policy.autoAction.vm.vmGroupId //defined if the VM is in a group
`POST` /ns/{nsId}/registerCspVm Register existing VM in a CSP to Cloud-Barista MCIS  
    Parameters

        Insert mcisReq.vm.subGroupSize //if subGroupSize is (not empty) && (> 0), subGroup will be gernetad. VMs will be created accordingly.
        Delete mcisReq.vm.vmGroupSize //if vmGroupSize is (not empty) && (> 0), VM group will be gernetad. VMs will be created accordingly.
        Modify mcisReq.vm.imageId
        Modify mcisReq.vm.name //VM name or VM group name if is (not empty) && (> 0). If it is a group, actual VM name will be generated with -N postfix.
    Return Type

        Insert vm.subGroupId //defined if the VM is in a group
        Insert vm.cspViewVmDetail.imageType
        Delete vm.vmGroupId //defined if the VM is in a group
`POST` /ns/{nsId}/resources/dataDisk Create Data Disk  
    Parameters

        Modify dataDiskInfo.connectionName
        Modify dataDiskInfo.diskSize
        Modify dataDiskInfo.diskType
        Modify dataDiskInfo.name
    Return Type

        Modify connectionName
        Modify createdTime
        Modify cspDataDiskId
        Modify cspDataDiskName
        Modify description
        Modify diskSize
        Modify diskType
        Modify id
        Modify name
        Modify status //available, unavailable
`PUT` /ns/{nsId}/resources/dataDisk/{dataDiskId} Upsize Data Disk  
    Return Type

        Modify connectionName
        Modify createdTime
        Modify cspDataDiskId
        Modify cspDataDiskName
        Modify description
        Modify diskSize
        Modify diskType
        Modify id
        Modify name
        Modify status //available, unavailable
`GET` /ns/{nsId}/resources/dataDisk/{dataDiskId} Get Data Disk  
    Return Type

        Modify connectionName
        Modify createdTime
        Modify cspDataDiskId
        Modify cspDataDiskName
        Modify description
        Modify diskSize
        Modify diskType
        Modify id
        Modify name
        Modify status //available, unavailable
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

