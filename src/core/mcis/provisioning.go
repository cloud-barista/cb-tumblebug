/*
Copyright 2019 The Cloud-Barista Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package mcis is to manage multi-cloud infra service
package mcis

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"sync"

	//"log"

	"strings"
	"time"

	//csv file handling

	// REST API (echo)

	"github.com/cloud-barista/cb-spider/interface/api"
	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
	validator "github.com/go-playground/validator/v10"
)

const (
	// ActionCreate is const for Create
	ActionCreate string = "Create"

	// ActionTerminate is const for Terminate
	ActionTerminate string = "Terminate"

	// ActionSuspend is const for Suspend
	ActionSuspend string = "Suspend"

	// ActionResume is const for Resume
	ActionResume string = "Resume"

	// ActionReboot is const for Reboot
	ActionReboot string = "Reboot"

	// ActionRefine is const for Refine
	ActionRefine string = "Refine"

	// ActionComplete is const for Complete
	ActionComplete string = "None"
)
const (
	// StatusRunning is const for Running
	StatusRunning string = "Running"

	// StatusSuspended is const for Suspended
	StatusSuspended string = "Suspended"

	// StatusFailed is const for Failed
	StatusFailed string = "Failed"

	// StatusTerminated is const for Terminated
	StatusTerminated string = "Terminated"

	// StatusCreating is const for Creating
	StatusCreating string = "Creating"

	// StatusSuspending is const for Suspending
	StatusSuspending string = "Suspending"

	// StatusResuming is const for Resuming
	StatusResuming string = "Resuming"

	// StatusRebooting is const for Rebooting
	StatusRebooting string = "Rebooting"

	// StatusTerminating is const for Terminating
	StatusTerminating string = "Terminating"

	// StatusUndefined is const for Undefined
	StatusUndefined string = "Undefined"

	// StatusComplete is const for Complete
	StatusComplete string = "None"
)

const labelAutoGen string = "AutoGen"

// DefaultSystemLabel is const for string to specify the Default System Label
const DefaultSystemLabel string = "Managed by CB-Tumblebug"

// RegionInfo is struct for region information
type RegionInfo struct {
	Region string
	Zone   string
}

// TbMcisReq is sturct for requirements to create MCIS
type TbMcisReq struct {
	Name string `json:"name" validate:"required" example:"mcis01"`

	// InstallMonAgent Option for CB-Dragonfly agent installation ([yes/no] default:yes)
	InstallMonAgent string `json:"installMonAgent" example:"no" default:"yes" enums:"yes,no"` // yes or no

	// Label is for describing the mcis in a keyword (any string can be used)
	Label string `json:"label" example:"custom tag" default:""`

	// SystemLabel is for describing the mcis in a keyword (any string can be used) for special System purpose
	SystemLabel string `json:"systemLabel" example:"" default:""`

	PlacementAlgo string `json:"placementAlgo,omitempty"`
	Description   string `json:"description" example:"Made in CB-TB"`

	Vm []TbVmReq `json:"vm" validate:"required"`
}

// TbMcisReqStructLevelValidation is func to validate fields in TbMcisReqStruct
func TbMcisReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(TbMcisReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

// TbMcisInfo is struct for MCIS info
type TbMcisInfo struct {
	Id           string          `json:"id"`
	Name         string          `json:"name"`
	Status       string          `json:"status"`
	StatusCount  StatusCountInfo `json:"statusCount"`
	TargetStatus string          `json:"targetStatus"`
	TargetAction string          `json:"targetAction"`

	// InstallMonAgent Option for CB-Dragonfly agent installation ([yes/no] default:yes)
	InstallMonAgent string `json:"installMonAgent" example:"yes" default:"yes" enums:"yes,no"` // yes or no

	// ConfigureCloudAdaptiveNetwork is an option to configure Cloud Adaptive Network (CLADNet) ([yes/no] default:yes)
	ConfigureCloudAdaptiveNetwork string `json:"configureCloudAdaptiveNetwork" example:"yes" default:"no" enums:"yes,no"` // yes or no

	// Label is for describing the mcis in a keyword (any string can be used)
	Label string `json:"label" example:"User custom label"`

	// SystemLabel is for describing the mcis in a keyword (any string can be used) for special System purpose
	SystemLabel string `json:"systemLabel" example:"Managed by CB-Tumblebug" default:""`

	// Latest system message such as error message
	SystemMessage string `json:"systemMessage" example:"Failed because ..." default:""` // systeam-given string message

	PlacementAlgo string     `json:"placementAlgo,omitempty"`
	Description   string     `json:"description"`
	Vm            []TbVmInfo `json:"vm"`

	// List of IDs for new VMs. Return IDs if the VMs are newly added. This field should be used for return body only.
	NewVmList []string `json:"newVmList"`
}

// TbVmReq is struct to get requirements to create a new server instance
type TbVmReq struct {
	// VM name or subGroup name if is (not empty) && (> 0). If it is a group, actual VM name will be generated with -N postfix.
	Name string `json:"name" validate:"required" example:"g1-1"`

	// CSP managed ID or Name (required for option=register)
	IdByCSP string `json:"idByCsp,omitempty" example:"i-014fa6ede6ada0b2c"`

	// if subGroupSize is (not empty) && (> 0), subGroup will be gernetad. VMs will be created accordingly.
	SubGroupSize string `json:"subGroupSize" example:"3" default:""`

	Label string `json:"label"`

	Description string `json:"description" example:"Description"`

	ConnectionName string `json:"connectionName" validate:"required" example:"testcloud01-seoul"`
	SpecId         string `json:"specId" validate:"required"`
	// ImageType        string   `json:"imageType"`
	ImageId          string   `json:"imageId" validate:"required"`
	VNetId           string   `json:"vNetId" validate:"required"`
	SubnetId         string   `json:"subnetId" validate:"required"`
	SecurityGroupIds []string `json:"securityGroupIds" validate:"required"`
	SshKeyId         string   `json:"sshKeyId" validate:"required"`
	VmUserAccount    string   `json:"vmUserAccount,omitempty"`
	VmUserPassword   string   `json:"vmUserPassword,omitempty"`
	RootDiskType     string   `json:"rootDiskType,omitempty" example:"default, TYPE1, ..."`  // "", "default", "TYPE1", AWS: ["standard", "gp2", "gp3"], Azure: ["PremiumSSD", "StandardSSD", "StandardHDD"], GCP: ["pd-standard", "pd-balanced", "pd-ssd", "pd-extreme"], ALIBABA: ["cloud_efficiency", "cloud", "cloud_ssd"], TENCENT: ["CLOUD_PREMIUM", "CLOUD_SSD"]
	RootDiskSize     string   `json:"rootDiskSize,omitempty" example:"default, 30, 42, ..."` // "default", Integer (GB): ["50", ..., "1000"]
	DataDiskIds      []string `json:"dataDiskIds"`
}

// TbVmReq is struct to get requirements to create a new server instance
type TbScaleOutSubGroupReq struct {
	// Define addtional VMs to scaleOut
	NumVMsToAdd string `json:"numVMsToAdd" validate:"required" example:"2"`

	//tobe added accoring to new future capability
}

// TbMcisDynamicReq is sturct for requirements to create MCIS dynamically (with default resource option)
type TbMcisDynamicReq struct {
	Name string `json:"name" validate:"required" example:"mcis01"`

	// InstallMonAgent Option for CB-Dragonfly agent installation ([yes/no] default:yes)
	InstallMonAgent string `json:"installMonAgent" example:"no" default:"yes" enums:"yes,no"` // yes or no

	// Label is for describing the mcis in a keyword (any string can be used)
	Label string `json:"label" example:"DynamicVM" default:""`

	// SystemLabel is for describing the mcis in a keyword (any string can be used) for special System purpose
	SystemLabel string `json:"systemLabel" example:"" default:""`

	Description string `json:"description" example:"Made in CB-TB"`

	Vm []TbVmDynamicReq `json:"vm" validate:"required"`
}

// TbVmDynamicReq is struct to get requirements to create a new server instance dynamically (with default resource option)
type TbVmDynamicReq struct {
	// VM name or subGroup name if is (not empty) && (> 0). If it is a group, actual VM name will be generated with -N postfix.
	Name string `json:"name" example:"g1-1"`

	// if subGroupSize is (not empty) && (> 0), subGroup will be gernetad. VMs will be created accordingly.
	SubGroupSize string `json:"subGroupSize" example:"3" default:""`

	Label string `json:"label" example:"DynamicVM"`

	Description string `json:"description" example:"Description"`

	// CommonSpec is field for id of a spec in common namespace
	CommonSpec string `json:"commonSpec" validate:"required" example:"aws-ap-northeast-2-t2-small"`
	// CommonImage is field for id of a image in common namespace
	CommonImage string `json:"commonImage" validate:"required" example:"ubuntu18.04"`

	RootDiskType string `json:"rootDiskType,omitempty" example:"default, TYPE1, ..."`  // "", "default", "TYPE1", AWS: ["standard", "gp2", "gp3"], Azure: ["PremiumSSD", "StandardSSD", "StandardHDD"], GCP: ["pd-standard", "pd-balanced", "pd-ssd", "pd-extreme"], ALIBABA: ["cloud_efficiency", "cloud", "cloud_essd"], TENCENT: ["CLOUD_PREMIUM", "CLOUD_SSD"]
	RootDiskSize string `json:"rootDiskSize,omitempty" example:"default, 30, 42, ..."` // "default", Integer (GB): ["50", ..., "1000"]

	VmUserPassword string `json:"vmUserPassword default:""`
	// if ConnectionName is given, the VM tries to use associtated credential.
	// if not, it will use predefined ConnectionName in Spec objects
	ConnectionName string `json:"connectionName,omitempty" default:""`
}

// McisConnectionConfigCandidatesReq is struct for a request to check requirements to create a new MCIS instance dynamically (with default resource option)
type McisConnectionConfigCandidatesReq struct {
	// CommonSpec is field for id of a spec in common namespace
	CommonSpecs []string `json:"commonSpec" validate:"required" example:"aws-ap-northeast-2-t2-small,gcp-us-west1-g1-small"`
}

// CheckMcisDynamicReqInfo is struct to check requirements to create a new MCIS instance dynamically (with default resource option)
type CheckMcisDynamicReqInfo struct {
	ReqCheck []CheckVmDynamicReqInfo `json:"reqCheck" validate:"required"`
}

// CheckVmDynamicReqInfo is struct to check requirements to create a new server instance dynamically (with default resource option)
type CheckVmDynamicReqInfo struct {

	// ConnectionConfigCandidates will provide ConnectionConfig options
	ConnectionConfigCandidates []string `json:"connectionConfigCandidates" default:""`

	// CommonImage is field for id of a image in common namespace
	// CommonImage		string `json:"commonImage" validate:"required" example:"ubuntu18.04"`
	//RootDiskSize string `json:"rootDiskSize,omitempty" example:"default, 30, 42, ..."` // "default", Integer (GB): ["50", ..., "1000"]

	VmSpec mcir.TbSpecInfo `json:"vmSpec" default:""`
	Region common.Region   `json:"region" default:""`

	// Latest system message such as error message
	SystemMessage string `json:"systemMessage" example:"Failed because ..." default:""` // systeam-given string message

}

//

// SpiderVMReqInfoWrapper is struct from CB-Spider (VMHandler.go) for wrapping SpiderVMInfo
type SpiderVMReqInfoWrapper struct {
	ConnectionName string
	ReqInfo        SpiderVMInfo
}

type SpiderImageType string

const (
	PublicImage SpiderImageType = "PublicImage"
	MyImage     SpiderImageType = "MyImage"
)

// Ref: cb-spider/cloud-control-manager/cloud-driver/interfaces/resources/VMHandler.go
// SpiderVMInfo is struct from CB-Spider for VM information
type SpiderVMInfo struct {
	// Fields for request
	Name               string
	ImageName          string
	VPCName            string
	SubnetName         string
	SecurityGroupNames []string
	KeyPairName        string
	CSPid              string // VM ID given by CSP (required for registering VM)
	DataDiskNames      []string

	// Fields for both request and response
	VMSpecName   string // instance type or flavour, etc... ex) t2.micro or f1.micro
	VMUserId     string // ex) user1
	VMUserPasswd string
	RootDiskType string // "SSD(gp2)", "Premium SSD", ...
	RootDiskSize string // "default", "50", "1000" (GB)
	ImageType    SpiderImageType

	// Fields for response
	IId               common.IID // {NameId, SystemId}
	ImageIId          common.IID
	VpcIID            common.IID
	SubnetIID         common.IID   // AWS, ex) subnet-8c4a53e4
	SecurityGroupIIds []common.IID // AWS, ex) sg-0b7452563e1121bb6
	KeyPairIId        common.IID
	DataDiskIIDs      []common.IID
	StartTime         time.Time
	Region            RegionInfo //  ex) {us-east1, us-east1-c} or {ap-northeast-2}
	NetworkInterface  string     // ex) eth0
	PublicIP          string
	PublicDNS         string
	PrivateIP         string
	PrivateDNS        string
	RootDeviceName    string // "/dev/sda1", ...
	SSHAccessPoint    string
	KeyValueList      []common.KeyValue
}

// TbVmReqStructLevelValidation is func to validate fields in TbVmReqStruct
func TbVmReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(TbVmReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

// TbSubGroupInfo is struct to define an object that includes homogeneous VMs
type TbSubGroupInfo struct {
	Id           string   `json:"id"`
	Name         string   `json:"name"`
	VmId         []string `json:"vmId"`
	SubGroupSize string   `json:"subGroupSize"`
}

// TbVmInfo is struct to define a server instance object
type TbVmInfo struct {
	Id      string `json:"id"`
	Name    string `json:"name"`
	IdByCSP string `json:"idByCSP"` // CSP managed ID or Name

	// defined if the VM is in a group
	SubGroupId string `json:"subGroupId"`

	Location common.GeoLocation `json:"location"`

	// Required by CB-Tumblebug
	Status       string `json:"status"`
	TargetStatus string `json:"targetStatus"`
	TargetAction string `json:"targetAction"`

	// Montoring agent status
	MonAgentStatus string `json:"monAgentStatus" example:"[installed, notInstalled, failed]"` // yes or no// installed, notInstalled, failed

	// NetworkAgent status
	NetworkAgentStatus string `json:"networkAgentStatus" example:"[notInstalled, installing, installed, failed]"` // notInstalled, installing, installed, failed

	// Latest system message such as error message
	SystemMessage string `json:"systemMessage" example:"Failed because ..." default:""` // systeam-given string message

	// Created time
	CreatedTime string `json:"createdTime" example:"2022-11-10 23:00:00" default:""`

	Label       string `json:"label"`
	Description string `json:"description"`

	Region         RegionInfo `json:"region"` // AWS, ex) {us-east1, us-east1-c} or {ap-northeast-2}
	PublicIP       string     `json:"publicIP"`
	SSHPort        string     `json:"sshPort"`
	PublicDNS      string     `json:"publicDNS"`
	PrivateIP      string     `json:"privateIP"`
	PrivateDNS     string     `json:"privateDNS"`
	RootDiskType   string     `json:"rootDiskType"`
	RootDiskSize   string     `json:"rootDiskSize"`
	RootDeviceName string     `json:"rootDeviceName"`

	ConnectionName   string            `json:"connectionName"`
	ConnectionConfig common.ConnConfig `json:"connectionConfig"`
	SpecId           string            `json:"specId"`
	ImageId          string            `json:"imageId"`
	VNetId           string            `json:"vNetId"`
	SubnetId         string            `json:"subnetId"`
	SecurityGroupIds []string          `json:"securityGroupIds"`
	DataDiskIds      []string          `json:"dataDiskIds"`
	SshKeyId         string            `json:"sshKeyId"`
	VmUserAccount    string            `json:"vmUserAccount,omitempty"`
	VmUserPassword   string            `json:"vmUserPassword,omitempty"`

	CspViewVmDetail SpiderVMInfo `json:"cspViewVmDetail,omitempty"`
}

// McisAccessInfo is struct to retrieve overall access information of a MCIS
type McisAccessInfo struct {
	McisId                 string
	McisNlbListener        *McisAccessInfo `json:"mcisNlbListener,omitempty"`
	McisSubGroupAccessInfo []McisSubGroupAccessInfo
}

// McisSubGroupAccessInfo is struct for McisSubGroupAccessInfo
type McisSubGroupAccessInfo struct {
	SubGroupId       string
	NlbListener      *TbNLBListenerInfo `json:"nlbListener,omitempty"`
	McisVmAccessInfo []McisVmAccessInfo
}

// McisVmAccessInfo is struct for McisVmAccessInfo
type McisVmAccessInfo struct {
	VmId           string `json:"vmId"`
	PublicIP       string `json:"publicIP"`
	PrivateIP      string `json:"privateIP"`
	SSHPort        string `json:"sshPort"`
	PrivateKey     string `json:"privateKey,omitempty"`
	VmUserAccount  string `json:"vmUserAccount,omitempty"`
	VmUserPassword string `json:"vmUserPassword,omitempty"`
}

// TbVmIdNameInDetailInfo is struct for details related with ID and Name
type TbIdNameInDetailInfo struct {
	IdInTb    string `json:"idInTb"`
	IdInSp    string `json:"idInSp"`
	IdInCsp   string `json:"idInCsp"`
	NameInCsp string `json:"nameInCsp"`
}

// StatusCountInfo is struct to count the number of VMs in each status. ex: Running=4, Suspended=8.
type StatusCountInfo struct {

	// CountTotal is for Total VMs
	CountTotal int `json:"countTotal"`

	// CountCreating is for counting Creating
	CountCreating int `json:"countCreating"`

	// CountRunning is for counting Running
	CountRunning int `json:"countRunning"`

	// CountFailed is for counting Failed
	CountFailed int `json:"countFailed"`

	// CountSuspended is for counting Suspended
	CountSuspended int `json:"countSuspended"`

	// CountRebooting is for counting Rebooting
	CountRebooting int `json:"countRebooting"`

	// CountTerminated is for counting Terminated
	CountTerminated int `json:"countTerminated"`

	// CountSuspending is for counting Suspending
	CountSuspending int `json:"countSuspending"`

	// CountResuming is for counting Resuming
	CountResuming int `json:"countResuming"`

	// CountTerminating is for counting Terminating
	CountTerminating int `json:"countTerminating"`

	// CountUndefined is for counting Undefined
	CountUndefined int `json:"countUndefined"`
}

// McisRecommendReq is struct for McisRecommendReq
type McisRecommendReq struct {
	VmReq          []TbVmRecommendReq `json:"vmReq"`
	PlacementAlgo  string             `json:"placementAlgo"`
	PlacementParam []common.KeyValue  `json:"placementParam"`
	MaxResultNum   string             `json:"maxResultNum"`
}

// TbVmRecommendReq is struct for TbVmRecommendReq
type TbVmRecommendReq struct {
	RequestName  string `json:"requestName"`
	MaxResultNum string `json:"maxResultNum"`

	VcpuSize   string `json:"vcpuSize"`
	MemorySize string `json:"memorySize"`
	DiskSize   string `json:"diskSize"`
	//Disk_type   string `json:"disk_type"`

	PlacementAlgo  string            `json:"placementAlgo"`
	PlacementParam []common.KeyValue `json:"placementParam"`
}

// TbVmPriority is struct for TbVmPriority
type TbVmPriority struct {
	Priority string          `json:"priority"`
	VmSpec   mcir.TbSpecInfo `json:"vmSpec"`
}

// TbVmRecommendInfo is struct for TbVmRecommendInfo
type TbVmRecommendInfo struct {
	VmReq          TbVmRecommendReq  `json:"vmReq"`
	VmPriority     []TbVmPriority    `json:"vmPriority"`
	PlacementAlgo  string            `json:"placementAlgo"`
	PlacementParam []common.KeyValue `json:"placementParam"`
}

// MCIS and VM Provisioning

// CreateMcisVm is func to post (create) McisVm
func CreateMcisVm(nsId string, mcisId string, vmInfoData *TbVmInfo) (*TbVmInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := &TbVmInfo{}
		common.CBLog.Error(err)
		return temp, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		temp := &TbVmInfo{}
		common.CBLog.Error(err)
		return temp, err
	}
	err = common.CheckString(vmInfoData.Name)
	if err != nil {
		temp := &TbVmInfo{}
		common.CBLog.Error(err)
		return temp, err
	}
	check, _ := CheckVm(nsId, mcisId, vmInfoData.Name)

	if check {
		temp := &TbVmInfo{}
		err := fmt.Errorf("The vm " + vmInfoData.Name + " already exists.")
		return temp, err
	}

	targetAction := ActionCreate
	targetStatus := StatusRunning

	vmInfoData.Id = vmInfoData.Name
	vmInfoData.PublicIP = "empty"
	vmInfoData.PublicDNS = "empty"
	vmInfoData.TargetAction = targetAction
	vmInfoData.TargetStatus = targetStatus
	vmInfoData.Status = StatusCreating

	//goroutin
	var wg sync.WaitGroup
	wg.Add(1)

	option := "create"
	go AddVmToMcis(&wg, nsId, mcisId, vmInfoData, option)

	wg.Wait()

	vmStatus, err := GetVmStatus(nsId, mcisId, vmInfoData.Id)
	if err != nil {
		return nil, fmt.Errorf("Cannot find " + common.GenMcisKey(nsId, mcisId, vmInfoData.Id))
	}

	vmInfoData.Status = vmStatus.Status
	vmInfoData.TargetStatus = vmStatus.TargetStatus
	vmInfoData.TargetAction = vmStatus.TargetAction

	// Install CB-Dragonfly monitoring agent

	mcisTmp, _ := GetMcisObject(nsId, mcisId)

	fmt.Printf("\n[Init monitoring agent] for %+v\n - req.InstallMonAgent: %+v\n\n", mcisId, mcisTmp.InstallMonAgent)

	if !strings.Contains(mcisTmp.InstallMonAgent, "no") {

		// Sleep for 20 seconds for a safe DF agent installation.
		fmt.Printf("\n\n[Info] Sleep for 20 seconds for safe CB-Dragonfly Agent installation.\n\n")
		time.Sleep(20 * time.Second)

		check := CheckDragonflyEndpoint()
		if check != nil {
			fmt.Printf("\n\n[Warning] CB-Dragonfly is not available\n\n")
		} else {
			reqToMon := &McisCmdReq{}
			reqToMon.UserName = "cb-user" // this MCIS user name is temporal code. Need to improve.

			fmt.Printf("\n[InstallMonitorAgentToMcis]\n\n")
			content, err := InstallMonitorAgentToMcis(nsId, mcisId, common.StrMCIS, reqToMon)
			if err != nil {
				common.CBLog.Error(err)
				//mcisTmp.InstallMonAgent = "no"
			}
			common.PrintJsonPretty(content)
			//mcisTmp.InstallMonAgent = "yes"
		}
	}

	return vmInfoData, nil
}

// ScaleOutMcisSubGroup is func to create MCIS groupVM
func ScaleOutMcisSubGroup(nsId string, mcisId string, subGroupId string, numVMsToAdd string) (*TbMcisInfo, error) {
	vmIdList, err := ListMcisGroupVms(nsId, mcisId, subGroupId)
	if err != nil {
		temp := &TbMcisInfo{}
		return temp, err
	}
	vmObj, err := GetVmObject(nsId, mcisId, vmIdList[0])

	vmTemplate := &TbVmReq{}

	// only take template required to create VM
	vmTemplate.Name = vmObj.SubGroupId
	vmTemplate.ConnectionName = vmObj.ConnectionName
	vmTemplate.ImageId = vmObj.ImageId
	vmTemplate.SpecId = vmObj.SpecId
	vmTemplate.VNetId = vmObj.VNetId
	vmTemplate.SubnetId = vmObj.SubnetId
	vmTemplate.SecurityGroupIds = vmObj.SecurityGroupIds
	vmTemplate.SshKeyId = vmObj.SshKeyId
	vmTemplate.VmUserAccount = vmObj.VmUserAccount
	vmTemplate.VmUserPassword = vmObj.VmUserPassword
	vmTemplate.RootDiskType = vmObj.RootDiskType
	vmTemplate.RootDiskSize = vmObj.RootDiskSize
	vmTemplate.Description = vmObj.Description

	vmTemplate.SubGroupSize = numVMsToAdd

	result, err := CreateMcisGroupVm(nsId, mcisId, vmTemplate, true)
	if err != nil {
		temp := &TbMcisInfo{}
		return temp, err
	}
	return result, nil

}

// CreateMcisGroupVm is func to create MCIS groupVM
func CreateMcisGroupVm(nsId string, mcisId string, vmRequest *TbVmReq, newSubGroup bool) (*TbMcisInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := &TbMcisInfo{}
		common.CBLog.Error(err)
		return temp, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		temp := &TbMcisInfo{}
		common.CBLog.Error(err)
		return temp, err
	}

	// returns InvalidValidationError for bad validation input, nil or ValidationErrors ( []FieldError )
	err = validate.Struct(vmRequest)
	if err != nil {

		// this check is only needed when your code could produce
		// an invalid value for validation such as interface with nil
		// value most including myself do not usually have code like this.
		if _, ok := err.(*validator.InvalidValidationError); ok {
			fmt.Println(err)
			return nil, err
		}

		// for _, err := range err.(validator.ValidationErrors) {

		// 	fmt.Println(err.Namespace()) // can differ when a custom TagNameFunc is registered or
		// 	fmt.Println(err.Field())     // by passing alt name to ReportError like below
		// 	fmt.Println(err.StructNamespace())
		// 	fmt.Println(err.StructField())
		// 	fmt.Println(err.Tag())
		// 	fmt.Println(err.ActualTag())
		// 	fmt.Println(err.Kind())
		// 	fmt.Println(err.Type())
		// 	fmt.Println(err.Value())
		// 	fmt.Println(err.Param())
		// 	fmt.Println()
		// }

		return nil, err
	}

	mcisTmp, err := GetMcisObject(nsId, mcisId)

	if err != nil {
		temp := &TbMcisInfo{}
		return temp, err
	}

	//vmRequest := req

	targetAction := ActionCreate
	targetStatus := StatusRunning

	//goroutin
	var wg sync.WaitGroup

	// subGroup handling
	subGroupSize, err := strconv.Atoi(vmRequest.SubGroupSize)
	fmt.Printf("subGroupSize: %v\n", subGroupSize)

	// make subGroup default (any VM going to be in a subGroup)
	if subGroupSize < 1 || err != nil {
		subGroupSize = 1
	}

	vmStartIndex := 1

	tentativeVmId := common.ToLower(vmRequest.Name)

	err = common.CheckString(tentativeVmId)
	if err != nil {
		common.CBLog.Error(err)
		return &TbMcisInfo{}, err
	}

	if subGroupSize > 0 {

		fmt.Println("=========================== Create MCIS subGroup object")

		subGroupInfoData := TbSubGroupInfo{}
		subGroupInfoData.Id = tentativeVmId
		subGroupInfoData.Name = tentativeVmId
		subGroupInfoData.SubGroupSize = vmRequest.SubGroupSize

		key := common.GenMcisSubGroupKey(nsId, mcisId, vmRequest.Name)
		keyValue, err := common.CBStore.Get(key)
		if err != nil {
			err = fmt.Errorf("In CreateMcisGroupVm(); CBStore.Get(): " + err.Error())
			common.CBLog.Error(err)
		}
		if keyValue != nil {
			if newSubGroup {
				json.Unmarshal([]byte(keyValue.Value), &subGroupInfoData)
				existingVmSize, err := strconv.Atoi(subGroupInfoData.SubGroupSize)
				if err != nil {
					err = fmt.Errorf("In CreateMcisGroupVm(); CBStore.Get(): " + err.Error())
					common.CBLog.Error(err)
				}
				// add the number of existing VMs in the SubGroup with requested number for additions
				subGroupInfoData.SubGroupSize = strconv.Itoa(existingVmSize + subGroupSize)
				vmStartIndex = existingVmSize + 1
			} else {
				err = fmt.Errorf("Duplicated SubGroup ID")
				common.CBLog.Error(err)
				return nil, err
			}
		}

		for i := vmStartIndex; i < subGroupSize+vmStartIndex; i++ {
			subGroupInfoData.VmId = append(subGroupInfoData.VmId, subGroupInfoData.Id+"-"+strconv.Itoa(i))
		}

		val, _ := json.Marshal(subGroupInfoData)
		err = common.CBStore.Put(key, string(val))
		if err != nil {
			common.CBLog.Error(err)
		}
		// check stored subGroup object
		keyValue, err = common.CBStore.Get(key)
		if err != nil {
			err = fmt.Errorf("In CreateMcisGroupVm(); CBStore.Get(): " + err.Error())
			common.CBLog.Error(err)
			// return nil, err
		}

		fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		fmt.Println("===========================")

	}

	for i := vmStartIndex; i <= subGroupSize+vmStartIndex; i++ {
		vmInfoData := TbVmInfo{}

		if subGroupSize == 0 { // for VM (not in a group)
			vmInfoData.Name = vmRequest.Name
		} else { // for VM (in a group)
			if i == subGroupSize+vmStartIndex {
				break
			}
			vmInfoData.SubGroupId = vmRequest.Name
			// TODO: Enhancement Required. Need to check existing subGroup. Need to update it if exist.
			vmInfoData.Name = vmRequest.Name + "-" + strconv.Itoa(i)
			fmt.Println("===========================")
			fmt.Println("vmInfoData.Name: " + vmInfoData.Name)
			fmt.Println("===========================")
		}
		vmInfoData.Id = vmInfoData.Name

		vmInfoData.Description = vmRequest.Description
		vmInfoData.PublicIP = "empty"
		vmInfoData.PublicDNS = "empty"

		vmInfoData.Status = StatusCreating
		vmInfoData.TargetAction = targetAction
		vmInfoData.TargetStatus = targetStatus

		vmInfoData.ConnectionName = vmRequest.ConnectionName
		vmInfoData.ConnectionConfig, err = common.GetConnConfig(vmRequest.ConnectionName)
		if err != nil {
			err = fmt.Errorf("Cannot retrieve ConnectionConfig" + err.Error())
			common.CBLog.Error(err)
		}
		vmInfoData.SpecId = vmRequest.SpecId
		vmInfoData.ImageId = vmRequest.ImageId
		vmInfoData.VNetId = vmRequest.VNetId
		vmInfoData.SubnetId = vmRequest.SubnetId
		//vmInfoData.VnicId = vmRequest.VnicId
		//vmInfoData.PublicIpId = vmRequest.PublicIpId
		vmInfoData.SecurityGroupIds = vmRequest.SecurityGroupIds
		vmInfoData.DataDiskIds = vmRequest.DataDiskIds
		vmInfoData.SshKeyId = vmRequest.SshKeyId
		vmInfoData.Description = vmRequest.Description

		vmInfoData.RootDiskType = vmRequest.RootDiskType
		vmInfoData.RootDiskSize = vmRequest.RootDiskSize

		vmInfoData.VmUserAccount = vmRequest.VmUserAccount
		vmInfoData.VmUserPassword = vmRequest.VmUserPassword

		wg.Add(1)
		// option != register
		go AddVmToMcis(&wg, nsId, mcisId, &vmInfoData, "")

	}

	wg.Wait()

	//Update MCIS status

	mcisTmp, err = GetMcisObject(nsId, mcisId)
	if err != nil {
		temp := &TbMcisInfo{}
		return temp, err
	}

	mcisStatusTmp, _ := GetMcisStatus(nsId, mcisId)

	mcisTmp.Status = mcisStatusTmp.Status

	if mcisTmp.TargetStatus == mcisTmp.Status {
		mcisTmp.TargetStatus = StatusComplete
		mcisTmp.TargetAction = ActionComplete
	}
	UpdateMcisInfo(nsId, mcisTmp)

	// Install CB-Dragonfly monitoring agent

	fmt.Printf("\n[Init monitoring agent] for %+v\n - req.InstallMonAgent: %+v\n\n", mcisId, mcisTmp.InstallMonAgent)
	if !strings.Contains(mcisTmp.InstallMonAgent, "no") {

		// Sleep for 60 seconds for a safe DF agent installation.
		fmt.Printf("\n\n[Info] Sleep for 60 seconds for safe CB-Dragonfly Agent installation.\n\n")
		time.Sleep(60 * time.Second)

		check := CheckDragonflyEndpoint()
		if check != nil {
			fmt.Printf("\n\n[Warning] CB-Dragonfly is not available\n\n")
		} else {
			reqToMon := &McisCmdReq{}
			reqToMon.UserName = "cb-user" // this MCIS user name is temporal code. Need to improve.

			fmt.Printf("\n[InstallMonitorAgentToMcis]\n\n")
			content, err := InstallMonitorAgentToMcis(nsId, mcisId, common.StrMCIS, reqToMon)
			if err != nil {
				common.CBLog.Error(err)
				//mcisTmp.InstallMonAgent = "no"
			}
			common.PrintJsonPretty(content)
			//mcisTmp.InstallMonAgent = "yes"
		}
	}

	vmList, err := ListMcisGroupVms(nsId, mcisId, tentativeVmId)

	if err != nil {
		mcisTmp.SystemMessage = err.Error()
	}
	if vmList != nil {
		mcisTmp.NewVmList = vmList
	}

	return &mcisTmp, nil

}

// CreateMcis is func to create MCIS obeject and deploy requested VMs (register CSP native VM with option=register)
func CreateMcis(nsId string, req *TbMcisReq, option string) (*TbMcisInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := &TbMcisInfo{}
		common.CBLog.Error(err)
		return temp, err
	}

	// returns InvalidValidationError for bad validation input, nil or ValidationErrors ( []FieldError )
	err = validate.Struct(req)
	if err != nil {

		// this check is only needed when your code could produce
		// an invalid value for validation such as interface with nil
		// value most including myself do not usually have code like this.
		if _, ok := err.(*validator.InvalidValidationError); ok {
			fmt.Println(err)
			return nil, err
		}

		// for _, err := range err.(validator.ValidationErrors) {

		// 	fmt.Println(err.Namespace()) // can differ when a custom TagNameFunc is registered or
		// 	fmt.Println(err.Field())     // by passing alt name to ReportError like below
		// 	fmt.Println(err.StructNamespace())
		// 	fmt.Println(err.StructField())
		// 	fmt.Println(err.Tag())
		// 	fmt.Println(err.ActualTag())
		// 	fmt.Println(err.Kind())
		// 	fmt.Println(err.Type())
		// 	fmt.Println(err.Value())
		// 	fmt.Println(err.Param())
		// 	fmt.Println()
		// }

		return nil, err
	}

	// skip mcis id checking for option=register
	if option != "register" {
		check, _ := CheckMcis(nsId, req.Name)
		if check {
			err := fmt.Errorf("The mcis " + req.Name + " already exists.")
			return nil, err
		}
	} else {
		req.SystemLabel = "Registered from CSP resource"
	}

	targetAction := ActionCreate
	targetStatus := StatusRunning

	mcisId := req.Name
	vmRequest := req.Vm

	fmt.Println("=========================== Create MCIS object")
	key := common.GenMcisKey(nsId, mcisId, "")
	mapA := map[string]string{
		"id":              mcisId,
		"name":            mcisId,
		"description":     req.Description,
		"status":          StatusCreating,
		"targetAction":    targetAction,
		"targetStatus":    targetStatus,
		"installMonAgent": req.InstallMonAgent,
		"label":           req.Label,
		"systemLabel":     req.SystemLabel,
	}
	val, err := json.Marshal(mapA)
	if err != nil {
		err := fmt.Errorf("System Error: CreateMcis json.Marshal(mapA) Error")
		common.CBLog.Error(err)
		return nil, err
	}

	err = common.CBStore.Put(key, string(val))
	if err != nil {
		err := fmt.Errorf("System Error: CreateMcis CBStore.Put Error")
		common.CBLog.Error(err)
		return nil, err
	}

	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
		err = fmt.Errorf("In CreateMcis(); CBStore.Get() returned an error.")
		common.CBLog.Error(err)
		// return nil, err
	}

	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")

	// Check whether VM names meet requirement.
	for _, k := range vmRequest {
		err = common.CheckString(k.Name)
		if err != nil {
			common.CBLog.Error(err)
			return &TbMcisInfo{}, err
		}
	}

	//goroutin
	var wg sync.WaitGroup

	vmStartIndex := 1

	for _, k := range vmRequest {

		// subGroup handling
		subGroupSize, err := strconv.Atoi(k.SubGroupSize)
		if err != nil {
			subGroupSize = 1
		}
		fmt.Printf("subGroupSize: %v\n", subGroupSize)

		if subGroupSize > 0 {

			fmt.Println("=========================== Create MCIS subGroup object")
			key := common.GenMcisSubGroupKey(nsId, mcisId, k.Name)

			subGroupInfoData := TbSubGroupInfo{}
			subGroupInfoData.Id = common.ToLower(k.Name)
			subGroupInfoData.Name = common.ToLower(k.Name)
			subGroupInfoData.SubGroupSize = k.SubGroupSize

			for i := vmStartIndex; i < subGroupSize+vmStartIndex; i++ {
				subGroupInfoData.VmId = append(subGroupInfoData.VmId, subGroupInfoData.Id+"-"+strconv.Itoa(i))
			}

			val, _ := json.Marshal(subGroupInfoData)
			err := common.CBStore.Put(key, string(val))
			if err != nil {
				common.CBLog.Error(err)
			}
			keyValue, err := common.CBStore.Get(key)
			if err != nil {
				common.CBLog.Error(err)
				err = fmt.Errorf("In CreateMcis(); CBStore.Get() returned an error.")
				common.CBLog.Error(err)
				// return nil, err
			}

			fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
			fmt.Println("===========================")

		}

		for i := vmStartIndex; i <= subGroupSize+vmStartIndex; i++ {
			vmInfoData := TbVmInfo{}

			if subGroupSize == 0 { // for VM (not in a group)
				vmInfoData.Name = common.ToLower(k.Name)
			} else { // for VM (in a group)
				if i == subGroupSize+vmStartIndex {
					break
				}
				vmInfoData.SubGroupId = common.ToLower(k.Name)
				vmInfoData.Name = common.ToLower(k.Name) + "-" + strconv.Itoa(i)
				fmt.Println("===========================")
				fmt.Println("vmInfoData.Name: " + vmInfoData.Name)
				fmt.Println("===========================")

			}
			vmInfoData.Id = vmInfoData.Name

			vmInfoData.PublicIP = "empty"
			vmInfoData.PublicDNS = "empty"

			vmInfoData.Status = StatusCreating
			vmInfoData.TargetAction = targetAction
			vmInfoData.TargetStatus = targetStatus

			vmInfoData.ConnectionName = k.ConnectionName
			vmInfoData.ConnectionConfig, err = common.GetConnConfig(k.ConnectionName)
			if err != nil {
				err = fmt.Errorf("Cannot retrieve ConnectionConfig" + err.Error())
				common.CBLog.Error(err)
			}
			vmInfoData.SpecId = k.SpecId
			vmInfoData.ImageId = k.ImageId
			vmInfoData.VNetId = k.VNetId
			vmInfoData.SubnetId = k.SubnetId
			vmInfoData.SecurityGroupIds = k.SecurityGroupIds
			vmInfoData.DataDiskIds = k.DataDiskIds
			vmInfoData.SshKeyId = k.SshKeyId
			vmInfoData.Description = k.Description
			vmInfoData.VmUserAccount = k.VmUserAccount
			vmInfoData.VmUserPassword = k.VmUserPassword
			vmInfoData.RootDiskType = k.RootDiskType
			vmInfoData.RootDiskSize = k.RootDiskSize

			vmInfoData.Label = k.Label

			vmInfoData.IdByCSP = k.IdByCSP

			// Avoid concurrent requests to CSP.
			time.Sleep(time.Duration(i) * time.Second)

			wg.Add(1)
			go AddVmToMcis(&wg, nsId, mcisId, &vmInfoData, option)
			//AddVmToMcis(nsId, req.Id, vmInfoData)

		}
	}
	wg.Wait()

	mcisTmp, err := GetMcisObject(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	mcisStatusTmp, err := GetMcisStatus(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	mcisTmp.Status = mcisStatusTmp.Status

	if mcisTmp.TargetStatus == mcisTmp.Status {
		mcisTmp.TargetStatus = StatusComplete
		mcisTmp.TargetAction = ActionComplete
	}
	UpdateMcisInfo(nsId, mcisTmp)

	fmt.Println("[MCIS has been created]" + mcisId)

	// Install CB-Dragonfly monitoring agent

	fmt.Printf("[Init monitoring agent] for %+v\n - req.InstallMonAgent: %+v\n\n", mcisTmp.Id, req.InstallMonAgent)

	mcisTmp.InstallMonAgent = req.InstallMonAgent
	UpdateMcisInfo(nsId, mcisTmp)

	if !strings.Contains(mcisTmp.InstallMonAgent, "no") && option != "register" {

		check := CheckDragonflyEndpoint()
		if check != nil {
			fmt.Printf("\n\n[Warning] CB-Dragonfly is not available\n\n")
		} else {
			reqToMon := &McisCmdReq{}
			reqToMon.UserName = "cb-user" // this MCIS user name is temporal code. Need to improve.

			fmt.Printf("\n===========================\n")
			// Sleep for 60 seconds for a safe DF agent installation.
			fmt.Printf("\n\n[Info] Sleep for 60 seconds for safe CB-Dragonfly Agent installation.\n")
			time.Sleep(60 * time.Second)

			fmt.Printf("\n[InstallMonitorAgentToMcis]\n\n")
			content, err := InstallMonitorAgentToMcis(nsId, mcisId, common.StrMCIS, reqToMon)
			if err != nil {
				common.CBLog.Error(err)
				//mcisTmp.InstallMonAgent = "no"
			}
			common.PrintJsonPretty(content)
			//mcisTmp.InstallMonAgent = "yes"
		}
	}

	mcisResult, err := GetMcisInfo(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}
	return mcisResult, nil
}

// CheckMcisDynamicReq is func to check request info to create MCIS obeject and deploy requested VMs in a dynamic way
func CheckMcisDynamicReq(req *McisConnectionConfigCandidatesReq) (*CheckMcisDynamicReqInfo, error) {

	mcisReqInfo := CheckMcisDynamicReqInfo{}

	connectionConfigList, err := common.GetConnConfigList()
	if err != nil {
		err := fmt.Errorf("Cannot load ConnectionConfigList in MCIS dynamic request check.")
		common.CBLog.Error(err)
		return &mcisReqInfo, err
	}

	// Find detail info and ConnectionConfigCandidates
	for _, k := range req.CommonSpecs {
		errMessage := ""

		vmReqInfo := CheckVmDynamicReqInfo{}

		tempInterface, err := mcir.GetResource(common.SystemCommonNs, common.StrSpec, k)
		if err != nil {
			errMessage += "//Failed to get the spec " + k
		}

		specInfo := mcir.TbSpecInfo{}
		err = common.CopySrcToDest(&tempInterface, &specInfo)
		if err != nil {
			errMessage += "//Failed to CopySrcToDest() " + k
		}

		regionInfo, err := common.GetRegion(specInfo.RegionName)
		if err != nil {
			errMessage += "//Failed to get Region (" + specInfo.RegionName + ") for Spec (" + k + ") is not found."
		}

		for _, connectionConfig := range connectionConfigList.Connectionconfig {
			if connectionConfig.RegionName == specInfo.RegionName {
				vmReqInfo.ConnectionConfigCandidates = append(vmReqInfo.ConnectionConfigCandidates, connectionConfig.ConfigName)
			}
		}

		vmReqInfo.VmSpec = specInfo
		vmReqInfo.Region = regionInfo
		vmReqInfo.SystemMessage = errMessage
		mcisReqInfo.ReqCheck = append(mcisReqInfo.ReqCheck, vmReqInfo)
	}

	return &mcisReqInfo, err
}

// CreateSystemMcisDynamic is func to create MCIS obeject and deploy requested VMs in a dynamic way
func CreateSystemMcisDynamic(option string) (*TbMcisInfo, error) {
	nsId := common.SystemCommonNs
	req := &TbMcisDynamicReq{}

	// special purpose MCIS
	req.Name = option
	req.Label = option
	req.SystemLabel = option
	req.Description = option
	req.InstallMonAgent = "no"

	switch option {
	case "probe":
		connections, err := common.GetConnConfigList()
		if err != nil {
			common.CBLog.Error(err)
			return nil, err
		}
		for _, v := range connections.Connectionconfig {

			vmReq := &TbVmDynamicReq{}
			vmReq.CommonImage = "ubuntu18.04"                // temporal default value. will be changed
			vmReq.CommonSpec = "aws-ap-northeast-2-t2-small" // temporal default value. will be changed

			deploymentPlan := DeploymentPlan{}
			condition := []Operation{}
			condition = append(condition, Operation{Operand: v.RegionName})

			fmt.Println(" - v.RegionName: " + v.RegionName)

			deploymentPlan.Filter.Policy = append(deploymentPlan.Filter.Policy, FilterCondition{Metric: "region", Condition: condition})
			deploymentPlan.Limit = "1"
			common.PrintJsonPretty(deploymentPlan)

			specList, err := RecommendVm(common.SystemCommonNs, deploymentPlan)
			if err != nil {
				common.CBLog.Error(err)
				return nil, err
			}
			if len(specList) != 0 {
				recommendedSpec := specList[0].Id
				vmReq.CommonSpec = recommendedSpec

				vmReq.Label = vmReq.CommonSpec
				vmReq.Name = vmReq.CommonSpec

				vmReq.RootDiskType = specList[0].RootDiskType
				vmReq.RootDiskSize = specList[0].RootDiskSize
				req.Vm = append(req.Vm, *vmReq)
			}
		}

	default:
		err := fmt.Errorf("Not available option. Try (option=probe)")
		return nil, err
	}
	if req.Vm == nil {
		err := fmt.Errorf("No VM is defined")
		return nil, err
	}

	return CreateMcisDynamic(nsId, req)
}

// CreateMcisDynamic is func to create MCIS obeject and deploy requested VMs in a dynamic way
func CreateMcisDynamic(nsId string, req *TbMcisDynamicReq) (*TbMcisInfo, error) {

	mcisReq := TbMcisReq{}
	mcisReq.Name = req.Name
	mcisReq.Label = req.Label
	mcisReq.SystemLabel = req.SystemLabel
	mcisReq.InstallMonAgent = req.InstallMonAgent
	mcisReq.Description = req.Description

	emptyMcis := &TbMcisInfo{}
	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return emptyMcis, err
	}
	check, err := CheckMcis(nsId, req.Name)
	if err != nil {
		common.CBLog.Error(err)
		return emptyMcis, err
	}
	if check {
		err := fmt.Errorf("The mcis " + req.Name + " already exists.")
		return emptyMcis, err
	}

	vmRequest := req.Vm
	// Check whether VM names meet requirement.
	for _, k := range vmRequest {
		vmReq, err := getVmReqFromDynamicReq(nsId, &k)
		if err != nil {
			common.CBLog.Error(err)
			return emptyMcis, err
		}
		mcisReq.Vm = append(mcisReq.Vm, *vmReq)
	}

	common.PrintJsonPretty(mcisReq)

	// Run create MCIS with the generated MCIS request (option != register)
	option := "create"
	return CreateMcis(nsId, &mcisReq, option)
}

// CreateMcisVmDynamic is func to create requested VM in a dynamic way and add it to MCIS
func CreateMcisVmDynamic(nsId string, mcisId string, req *TbVmDynamicReq) (*TbMcisInfo, error) {

	emptyMcis := &TbMcisInfo{}
	subGroupId := req.Name
	check, err := CheckSubGroup(nsId, mcisId, subGroupId)
	if err != nil {
		common.CBLog.Error(err)
		return emptyMcis, err
	}
	if check {
		err := fmt.Errorf("The name for SubGroup (prefix of VM Id) " + req.Name + " already exists.")
		return emptyMcis, err
	}

	vmReq, err := getVmReqFromDynamicReq(nsId, req)
	if err != nil {
		common.CBLog.Error(err)
		return emptyMcis, err
	}

	return CreateMcisGroupVm(nsId, mcisId, vmReq, true)
}

// getVmReqForDynamicMcis is func to getVmReqFromDynamicReq
func getVmReqFromDynamicReq(nsId string, req *TbVmDynamicReq) (*TbVmReq, error) {

	onDemand := true

	vmRequest := req
	// Check whether VM names meet requirement.
	k := vmRequest

	vmReq := &TbVmReq{}
	tempInterface, err := mcir.GetResource(common.SystemCommonNs, common.StrSpec, k.CommonSpec)
	if err != nil {
		err := fmt.Errorf("Failed to get the spec " + k.CommonSpec)
		common.CBLog.Error(err)
		return &TbVmReq{}, err
	}
	specInfo := mcir.TbSpecInfo{}
	err = common.CopySrcToDest(&tempInterface, &specInfo)
	if err != nil {
		err := fmt.Errorf("Failed to CopySrcToDest() " + k.CommonSpec)
		common.CBLog.Error(err)
		return &TbVmReq{}, err
	}

	// remake vmReqest from given input and check resource availability
	vmReq.ConnectionName = specInfo.ConnectionName

	// If ConnectionName is specified by the request, Use ConnectionName from the request
	if k.ConnectionName != "" {
		vmReq.ConnectionName = k.ConnectionName
	}
	// validate the region for spec
	_, err = common.GetConnConfig(specInfo.RegionName)
	if err != nil {
		err := fmt.Errorf("Failed to get RegionName (" + specInfo.RegionName + ") for Spec (" + k.CommonSpec + ") is not found.")
		common.CBLog.Error(err)
		return &TbVmReq{}, err
	}
	// validate the GetConnConfig for spec
	_, err = common.GetConnConfig(vmReq.ConnectionName)
	if err != nil {
		err := fmt.Errorf("Failed to get ConnectionName (" + vmReq.ConnectionName + ") for Spec (" + k.CommonSpec + ") is not found.")
		common.CBLog.Error(err)
		return &TbVmReq{}, err
	}

	// Default resource name has this pattern (nsId + "-systemdefault-" + vmReq.ConnectionName)
	resourceName := nsId + common.StrDefaultResourceName + vmReq.ConnectionName

	vmReq.SpecId = specInfo.Id
	vmReq.ImageId = mcir.ToNamingRuleCompatible(vmReq.ConnectionName + "-" + k.CommonImage)
	tempInterface, err = mcir.GetResource(common.SystemCommonNs, common.StrImage, vmReq.ImageId)
	if err != nil {
		err := fmt.Errorf("Failed to get the Image " + vmReq.ImageId + " from " + vmReq.ConnectionName)
		common.CBLog.Error(err)
		return &TbVmReq{}, err
	}

	vmReq.VNetId = resourceName
	tempInterface, err = mcir.GetResource(nsId, common.StrVNet, vmReq.VNetId)
	if err != nil {
		err := fmt.Errorf("Failed to get the vNet " + vmReq.VNetId + " from " + vmReq.ConnectionName)
		common.CBLog.Info(err)
		if !onDemand {
			return &TbVmReq{}, err
		}
		err2 := mcir.LoadDefaultResource(nsId, common.StrVNet, vmReq.ConnectionName)
		if err2 != nil {
			common.CBLog.Error(err2)
			err2 = fmt.Errorf("[1]" + err.Error() + " [2]" + err2.Error())
			return &TbVmReq{}, err2
		}
	}
	vmReq.SubnetId = resourceName

	vmReq.SshKeyId = resourceName
	tempInterface, err = mcir.GetResource(nsId, common.StrSSHKey, vmReq.SshKeyId)
	if err != nil {
		err := fmt.Errorf("Failed to get the SshKey " + vmReq.SshKeyId + " from " + vmReq.ConnectionName)
		common.CBLog.Info(err)
		if !onDemand {
			return &TbVmReq{}, err
		}
		err2 := mcir.LoadDefaultResource(nsId, common.StrSSHKey, vmReq.ConnectionName)
		if err2 != nil {
			common.CBLog.Error(err2)
			err2 = fmt.Errorf("[1]" + err.Error() + " [2]" + err2.Error())
			return &TbVmReq{}, err2
		}
	}
	securityGroup := resourceName
	vmReq.SecurityGroupIds = append(vmReq.SecurityGroupIds, securityGroup)
	tempInterface, err = mcir.GetResource(nsId, common.StrSecurityGroup, securityGroup)
	if err != nil {
		err := fmt.Errorf("Failed to get the SecurityGroup " + securityGroup + " from " + vmReq.ConnectionName)
		common.CBLog.Info(err)
		if !onDemand {
			return &TbVmReq{}, err
		}
		err2 := mcir.LoadDefaultResource(nsId, common.StrSecurityGroup, vmReq.ConnectionName)
		if err2 != nil {
			common.CBLog.Error(err2)
			err2 = fmt.Errorf("[1]" + err.Error() + " [2]" + err2.Error())
			return &TbVmReq{}, err2
		}
	}

	vmReq.Name = k.Name
	if vmReq.Name == "" {
		vmReq.Name = common.GenUid()
	}
	vmReq.Label = k.Label
	vmReq.SubGroupSize = k.SubGroupSize
	vmReq.Description = k.Description
	vmReq.RootDiskType = k.RootDiskType
	vmReq.RootDiskSize = k.RootDiskSize
	vmReq.VmUserPassword = k.VmUserPassword

	common.PrintJsonPretty(vmReq)

	return vmReq, nil
}

// AddVmToMcis is func to add VM to MCIS
func AddVmToMcis(wg *sync.WaitGroup, nsId string, mcisId string, vmInfoData *TbVmInfo, option string) error {
	fmt.Printf("\n[AddVmToMcis]\n")
	//goroutin
	defer wg.Done()

	key := common.GenMcisKey(nsId, mcisId, "")
	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
		err = fmt.Errorf("In AddVmToMcis(); CBStore.Get() returned an error.")
		common.CBLog.Error(err)
		// return nil, err
	}

	if keyValue == nil {
		return fmt.Errorf("AddVmToMcis: Cannot find mcisId. Key: %s", key)
	}

	configTmp, _ := common.GetConnConfig(vmInfoData.ConnectionName)
	regionTmp, _ := common.GetRegion(configTmp.RegionName)

	nativeRegion := ""
	for _, v := range regionTmp.KeyValueInfoList {
		if strings.ToLower(v.Key) == "region" || strings.ToLower(v.Key) == "location" {
			nativeRegion = v.Value
			break
		}
	}

	vmInfoData.Location = common.GetCloudLocation(strings.ToLower(configTmp.ProviderName), strings.ToLower(nativeRegion))

	//AddVmInfoToMcis(nsId, mcisId, *vmInfoData)
	// Make VM object
	key = common.GenMcisKey(nsId, mcisId, vmInfoData.Id)
	val, _ := json.Marshal(vmInfoData)
	err = common.CBStore.Put(key, string(val))
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	fmt.Printf("\n[AddVmToMcis Befor request vmInfoData]\n")

	//instanceIds, publicIPs := CreateVm(&vmInfoData)
	err = CreateVm(nsId, mcisId, vmInfoData, option)

	fmt.Printf("\n[AddVmToMcis After request vmInfoData]\n")

	if err != nil {
		vmInfoData.Status = StatusFailed
		vmInfoData.SystemMessage = err.Error()
		UpdateVmInfo(nsId, mcisId, *vmInfoData)
		common.CBLog.Error(err)
		return err
	}

	// set initial TargetAction, TargetStatus
	vmInfoData.TargetAction = ActionComplete
	vmInfoData.TargetStatus = StatusComplete

	// get and set current vm status
	vmStatusInfoTmp, err := GetVmStatus(nsId, mcisId, vmInfoData.Id)

	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	fmt.Printf("\n[AddVmToMcis vmStatusInfoTmp]\n")
	common.PrintJsonPretty(vmStatusInfoTmp)

	vmInfoData.Status = vmStatusInfoTmp.Status

	// Monitoring Agent Installation Status (init: notInstalled)
	vmInfoData.MonAgentStatus = "notInstalled"
	vmInfoData.NetworkAgentStatus = "notInstalled"

	// set CreatedTime
	t := time.Now()
	vmInfoData.CreatedTime = t.Format("2006-01-02 15:04:05")
	fmt.Println(vmInfoData.CreatedTime)

	UpdateVmInfo(nsId, mcisId, *vmInfoData)

	return nil

}

// CreateVm is func to create VM (option = "register" for register existing VM)
func CreateVm(nsId string, mcisId string, vmInfoData *TbVmInfo, option string) error {

	fmt.Printf("\n\n[CreateVm(vmInfoData *TbVmInfo)]\n\n")

	switch {
	case vmInfoData.Name == "":
		err := fmt.Errorf("vmInfoData.Name is empty")
		common.CBLog.Error(err)
		return err
	case vmInfoData.ImageId == "":
		err := fmt.Errorf("vmInfoData.ImageId is empty")
		common.CBLog.Error(err)
		return err
	case vmInfoData.ConnectionName == "":
		err := fmt.Errorf("vmInfoData.ConnectionName is empty")
		common.CBLog.Error(err)
		return err
	case vmInfoData.SshKeyId == "":
		err := fmt.Errorf("vmInfoData.SshKeyId is empty")
		common.CBLog.Error(err)
		return err
	case vmInfoData.SpecId == "":
		err := fmt.Errorf("vmInfoData.SpecId is empty")
		common.CBLog.Error(err)
		return err
	case vmInfoData.SecurityGroupIds == nil:
		err := fmt.Errorf("vmInfoData.SecurityGroupIds is empty")
		common.CBLog.Error(err)
		return err
	case vmInfoData.VNetId == "":
		err := fmt.Errorf("vmInfoData.VNetId is empty")
		common.CBLog.Error(err)
		return err
	case vmInfoData.SubnetId == "":
		err := fmt.Errorf("vmInfoData.SubnetId is empty")
		common.CBLog.Error(err)
		return err
	default:

	}

	// in case of registering existing CSP VM
	if option == "register" {
		// IdByCSP is required
		if vmInfoData.IdByCSP == "" {
			err := fmt.Errorf("vmInfoData.IdByCSP is empty (required for register VM)")
			common.CBLog.Error(err)
			return err
		}
	}

	var tempSpiderVMInfo SpiderVMInfo

	// Fill VM creation reqest (request to cb-spider)
	tempReq := SpiderVMReqInfoWrapper{}
	tempReq.ConnectionName = vmInfoData.ConnectionName

	//generate VM ID(Name) to request to CSP(Spider)
	//combination of nsId, mcidId, and vmName reqested from user
	tempReq.ReqInfo.Name = fmt.Sprintf("%s-%s-%s", nsId, mcisId, vmInfoData.Name)

	err := fmt.Errorf("")
	customImageFlag := false

	tempReq.ReqInfo.VMUserId = vmInfoData.VmUserAccount
	tempReq.ReqInfo.VMUserPasswd = vmInfoData.VmUserPassword
	// provide a random passwd, if it is not provided by user (the passwd required for Windows)
	if tempReq.ReqInfo.VMUserPasswd == "" {
		// assign random string (mixed Uid style)
		tempReq.ReqInfo.VMUserPasswd = common.GenRandomPassword()
	}

	tempReq.ReqInfo.RootDiskType = vmInfoData.RootDiskType
	tempReq.ReqInfo.RootDiskSize = vmInfoData.RootDiskSize

	if option == "register" {
		tempReq.ReqInfo.CSPid = vmInfoData.IdByCSP

	} else {
		// Try lookup customImage
		tempReq.ReqInfo.ImageName, err = common.GetCspResourceId(nsId, common.StrCustomImage, vmInfoData.ImageId)
		if tempReq.ReqInfo.ImageName == "" || err != nil {
			errAgg := err.Error()
			// If customImage doesn't exist, then try lookup image
			tempReq.ReqInfo.ImageName, err = common.GetCspResourceId(nsId, common.StrImage, vmInfoData.ImageId)
			if tempReq.ReqInfo.ImageName == "" || err != nil {
				errAgg += err.Error()
				// If cannot find the resource, use common resource
				tempReq.ReqInfo.ImageName, err = common.GetCspResourceId(common.SystemCommonNs, common.StrImage, vmInfoData.ImageId)
				if tempReq.ReqInfo.ImageName == "" || err != nil {
					errAgg += err.Error()
					err = fmt.Errorf(errAgg)
					common.CBLog.Error(err)
					return err
				}
			}
		} else {
			customImageFlag = true
			tempReq.ReqInfo.ImageType = MyImage
			// If the requested image is a custom image (generated by VM snapshot), RootDiskType should be empty.
			// TB ignore inputs for RootDiskType, RootDiskSize
			tempReq.ReqInfo.RootDiskType = ""
			tempReq.ReqInfo.RootDiskSize = ""
		}

		tempReq.ReqInfo.VMSpecName, err = common.GetCspResourceId(nsId, common.StrSpec, vmInfoData.SpecId)
		if tempReq.ReqInfo.VMSpecName == "" || err != nil {
			common.CBLog.Info(err)
			errAgg := err.Error()
			// If cannot find the resource, use common resource
			tempReq.ReqInfo.VMSpecName, err = common.GetCspResourceId(common.SystemCommonNs, common.StrSpec, vmInfoData.SpecId)
			if tempReq.ReqInfo.ImageName == "" || err != nil {
				errAgg += err.Error()
				err = fmt.Errorf(errAgg)
				common.CBLog.Error(err)
				return err
			}
		}

		tempReq.ReqInfo.VPCName, err = common.GetCspResourceId(nsId, common.StrVNet, vmInfoData.VNetId)
		if tempReq.ReqInfo.VPCName == "" {
			common.CBLog.Error(err)
			return err
		}

		// TODO: needs to be enhnaced to use GetCspResourceId (GetCspResourceId needs to be updated as well)
		tempReq.ReqInfo.SubnetName = vmInfoData.SubnetId //common.GetCspResourceId(nsId, common.StrVNet, vmInfoData.SubnetId)
		if tempReq.ReqInfo.SubnetName == "" {
			common.CBLog.Error(err)
			return err
		}

		var SecurityGroupIdsTmp []string
		for _, v := range vmInfoData.SecurityGroupIds {
			CspSgId, err := common.GetCspResourceId(nsId, common.StrSecurityGroup, v)
			if CspSgId == "" {
				common.CBLog.Error(err)
				return err
			}

			SecurityGroupIdsTmp = append(SecurityGroupIdsTmp, CspSgId)
		}
		tempReq.ReqInfo.SecurityGroupNames = SecurityGroupIdsTmp

		var DataDiskIdsTmp []string
		for _, v := range vmInfoData.DataDiskIds {
			// ignore DataDiskIds == "", assume it is ignorable mistake
			if v != "" {
				CspDataDiskId, err := common.GetCspResourceId(nsId, common.StrDataDisk, v)
				if err != nil || CspDataDiskId == "" {
					common.CBLog.Error(err)
					return err
				}
				DataDiskIdsTmp = append(DataDiskIdsTmp, CspDataDiskId)
			}
		}
		tempReq.ReqInfo.DataDiskNames = DataDiskIdsTmp

		tempReq.ReqInfo.KeyPairName, err = common.GetCspResourceId(nsId, common.StrSSHKey, vmInfoData.SshKeyId)
		if tempReq.ReqInfo.KeyPairName == "" {
			common.CBLog.Error(err)
			return err
		}
	}

	fmt.Printf("\n[Request body to CB-Spider for Creating VM]\n")
	common.PrintJsonPretty(tempReq)

	payload, _ := json.Marshal(tempReq)

	// Randomly sleep within 30 Secs to avoid rateLimit from CSP
	common.RandomSleep(0, 30)

	// Call CB-Spider API by REST or gRPC
	if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

		url := common.SpiderRestUrl + "/vm"
		method := "POST"
		if option == "register" {
			url = common.SpiderRestUrl + "/regvm"
			method = "POST"
		}

		fmt.Println("\n[Calling CB-Spider]")
		fmt.Println("url: " + url + " method: " + method)

		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		req, err := http.NewRequest(method, url, strings.NewReader(string(payload)))

		if err != nil {
			common.CBLog.Error(err)
			return err
		}

		req.Header.Add("Content-Type", "application/json")

		res, err := client.Do(req)
		if err != nil {
			common.CBLog.Error(err)
			return err
		}

		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			common.CBLog.Error(err)
			return err
		}
		defer res.Body.Close()

		// tempSpiderVMInfo = SpiderVMInfo{} // FYI; SpiderVMInfo: the struct in CB-Spider
		err = json.Unmarshal(body, &tempSpiderVMInfo)

		if err != nil {
			common.PrintJsonPretty(err)
			common.CBLog.Error(err)
			return err
		}

		fmt.Println("HTTP Status code: " + strconv.Itoa(res.StatusCode))
		switch {
		case res.StatusCode >= 400 || res.StatusCode < 200:
			err := fmt.Errorf(string(body))
			fmt.Println("body: ", string(body))
			common.CBLog.Error(err)
			return err
		}

	} else {

		// Set CCM gRPC API
		ccm := api.NewCloudResourceHandler()
		err := ccm.SetConfigPath(os.Getenv("CBTUMBLEBUG_ROOT") + "/conf/grpc_conf.yaml")
		if err != nil {
			common.CBLog.Error("ccm failed to set config : ", err)
			return err
		}
		err = ccm.Open()
		if err != nil {
			common.CBLog.Error("ccm api open failed : ", err)
			return err
		}
		defer ccm.Close()

		fmt.Println("\n[Calling CB-Spider]")

		result, err := ccm.StartVM(string(payload))
		if err != nil {
			common.CBLog.Error(err)
			return err
		}

		tempSpiderVMInfo = SpiderVMInfo{} // FYI; SpiderVMInfo: the struct in CB-Spider
		err2 := json.Unmarshal([]byte(result), &tempSpiderVMInfo)
		if err2 != nil {
			fmt.Println(err)
			common.CBLog.Error(err)
			return err
		}
	}
	fmt.Println("[Response from CB-Spider]")
	common.PrintJsonPretty(tempSpiderVMInfo)
	fmt.Println("[Finished calling CB-Spider]")

	vmInfoData.CspViewVmDetail = tempSpiderVMInfo
	vmInfoData.VmUserAccount = tempSpiderVMInfo.VMUserId
	vmInfoData.VmUserPassword = tempSpiderVMInfo.VMUserPasswd

	//vmInfoData.Location = vmInfoData.Location

	//vmInfoData.PlacementAlgo = vmInfoData.PlacementAlgo

	//vmInfoData.CspVmId = temp.Id
	//vmInfoData.StartTime = temp.StartTime
	vmInfoData.Region = tempSpiderVMInfo.Region
	vmInfoData.PublicIP = tempSpiderVMInfo.PublicIP
	vmInfoData.SSHPort, _ = TrimIP(tempSpiderVMInfo.SSHAccessPoint)
	vmInfoData.PublicDNS = tempSpiderVMInfo.PublicDNS
	vmInfoData.PrivateIP = tempSpiderVMInfo.PrivateIP
	vmInfoData.PrivateDNS = tempSpiderVMInfo.PrivateDNS
	vmInfoData.RootDiskType = tempSpiderVMInfo.RootDiskType
	vmInfoData.RootDiskSize = tempSpiderVMInfo.RootDiskSize
	vmInfoData.RootDeviceName = tempSpiderVMInfo.RootDeviceName
	//vmInfoData.KeyValueList = temp.KeyValueList

	/* Dummy code
	if customImageFlag == true {
		vmInfoData.ImageType = "custom"
	}
	*/

	//configTmp, _ := common.GetConnConfig(vmInfoData.ConnectionName)
	//vmInfoData.Location = GetCloudLocation(strings.ToLower(configTmp.ProviderName), strings.ToLower(tempSpiderVMInfo.Region.Region))

	if option == "register" {

		// Reconstuct resource IDs
		// vNet
		resourceListInNs, err := mcir.ListResource(nsId, common.StrVNet, "cspVNetName", tempSpiderVMInfo.VpcIID.NameId)
		if err != nil {
			common.CBLog.Error(err)
		} else {
			resourcesInNs := resourceListInNs.([]mcir.TbVNetInfo) // type assertion
			for _, resource := range resourcesInNs {
				if resource.ConnectionName == tempReq.ConnectionName {
					vmInfoData.VNetId = resource.Id
					//vmInfoData.SubnetId = resource.SubnetInfoList
				}
			}
		}

		// access Key
		resourceListInNs, err = mcir.ListResource(nsId, common.StrSSHKey, "cspSshKeyName", tempSpiderVMInfo.KeyPairIId.NameId)
		if err != nil {
			common.CBLog.Error(err)
		} else {
			resourcesInNs := resourceListInNs.([]mcir.TbSshKeyInfo) // type assertion
			for _, resource := range resourcesInNs {
				if resource.ConnectionName == tempReq.ConnectionName {
					vmInfoData.SshKeyId = resource.Id
				}
			}
		}

	} else {
		vmKey := common.GenMcisKey(nsId, mcisId, vmInfoData.Id)

		if customImageFlag == false {
			mcir.UpdateAssociatedObjectList(nsId, common.StrImage, vmInfoData.ImageId, common.StrAdd, vmKey)
		} else {
			mcir.UpdateAssociatedObjectList(nsId, common.StrCustomImage, vmInfoData.ImageId, common.StrAdd, vmKey)
		}

		mcir.UpdateAssociatedObjectList(nsId, common.StrSpec, vmInfoData.SpecId, common.StrAdd, vmKey)
		mcir.UpdateAssociatedObjectList(nsId, common.StrSSHKey, vmInfoData.SshKeyId, common.StrAdd, vmKey)
		mcir.UpdateAssociatedObjectList(nsId, common.StrVNet, vmInfoData.VNetId, common.StrAdd, vmKey)

		for _, v := range vmInfoData.SecurityGroupIds {
			mcir.UpdateAssociatedObjectList(nsId, common.StrSecurityGroup, v, common.StrAdd, vmKey)
		}

		for _, v := range vmInfoData.DataDiskIds {
			mcir.UpdateAssociatedObjectList(nsId, common.StrDataDisk, v, common.StrAdd, vmKey)
		}
	}

	// Register dataDisks which are created with the creation of VM
	for _, v := range tempSpiderVMInfo.DataDiskIIDs {
		tbDataDiskReq := mcir.TbDataDiskReq{
			Name:           v.NameId,
			ConnectionName: vmInfoData.ConnectionName,
			// CspDataDiskId:  v.NameId, // v.SystemId ? IdByCsp ?
		}

		dataDisk, err := mcir.CreateDataDisk(nsId, &tbDataDiskReq, "register")
		if err != nil {
			err = fmt.Errorf("After starting VM %s, failed to register dataDisk %s. \n", vmInfoData.Name, v.NameId)
			// continue
		}

		vmInfoData.DataDiskIds = append(vmInfoData.DataDiskIds, dataDisk.Id)

		vmKey := common.GenMcisKey(nsId, mcisId, vmInfoData.Id)
		mcir.UpdateAssociatedObjectList(nsId, common.StrDataDisk, dataDisk.Id, common.StrAdd, vmKey)
	}

	UpdateVmInfo(nsId, mcisId, *vmInfoData)

	return nil
}
