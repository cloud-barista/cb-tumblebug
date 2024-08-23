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

// Package mci is to manage multi-cloud infra
package infra

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	validator "github.com/go-playground/validator/v10"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
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

// TbMciReq is struct for requirements to create MCI
type TbMciReq struct {
	Name string `json:"name" validate:"required" example:"mci01"`

	// InstallMonAgent Option for CB-Dragonfly agent installation ([yes/no] default:yes)
	InstallMonAgent string `json:"installMonAgent" example:"no" default:"yes" enums:"yes,no"` // yes or no

	// Label is for describing the mci in a keyword (any string can be used)
	Label string `json:"label" example:"custom tag" default:""`

	// SystemLabel is for describing the mci in a keyword (any string can be used) for special System purpose
	SystemLabel string `json:"systemLabel" example:"" default:""`

	PlacementAlgo string `json:"placementAlgo,omitempty"`
	Description   string `json:"description" example:"Made in CB-TB"`

	Vm []TbVmReq `json:"vm" validate:"required"`
}

// TbMciReqStructLevelValidation is func to validate fields in TbMciReqStruct
func TbMciReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(TbMciReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

// TbMciInfo is struct for MCI info
type TbMciInfo struct {
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

	// Label is for describing the mci in a keyword (any string can be used)
	Label string `json:"label" example:"User custom label"`

	// SystemLabel is for describing the mci in a keyword (any string can be used) for special System purpose
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

	// if subGroupSize is (not empty) && (> 0), subGroup will be generated. VMs will be created accordingly.
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

// TbMciDynamicReq is struct for requirements to create MCI dynamically (with default resource option)
type TbMciDynamicReq struct {
	Name string `json:"name" validate:"required" example:"mci01"`

	// InstallMonAgent Option for CB-Dragonfly agent installation ([yes/no] default:yes)
	InstallMonAgent string `json:"installMonAgent" example:"no" default:"no" enums:"yes,no"` // yes or no

	// Label is for describing the mci in a keyword (any string can be used)
	Label string `json:"label" example:"DynamicVM" default:""`

	// SystemLabel is for describing the mci in a keyword (any string can be used) for special System purpose
	SystemLabel string `json:"systemLabel" example:"" default:""`

	Description string `json:"description" example:"Made in CB-TB"`

	Vm []TbVmDynamicReq `json:"vm" validate:"required"`
}

// TbVmDynamicReq is struct to get requirements to create a new server instance dynamically (with default resource option)
type TbVmDynamicReq struct {
	// VM name or subGroup name if is (not empty) && (> 0). If it is a group, actual VM name will be generated with -N postfix.
	Name string `json:"name" example:"g1-1"`

	// if subGroupSize is (not empty) && (> 0), subGroup will be generated. VMs will be created accordingly.
	SubGroupSize string `json:"subGroupSize" example:"3" default:"1"`

	Label string `json:"label" example:"DynamicVM"`

	Description string `json:"description" example:"Description"`

	// CommonSpec is field for id of a spec in common namespace
	CommonSpec string `json:"commonSpec" validate:"required" example:"aws+ap-northeast-2+t2.small"`
	// CommonImage is field for id of a image in common namespace
	CommonImage string `json:"commonImage" validate:"required" example:"ubuntu18.04"`

	RootDiskType string `json:"rootDiskType,omitempty" example:"default, TYPE1, ..." default:"default"`  // "", "default", "TYPE1", AWS: ["standard", "gp2", "gp3"], Azure: ["PremiumSSD", "StandardSSD", "StandardHDD"], GCP: ["pd-standard", "pd-balanced", "pd-ssd", "pd-extreme"], ALIBABA: ["cloud_efficiency", "cloud", "cloud_essd"], TENCENT: ["CLOUD_PREMIUM", "CLOUD_SSD"]
	RootDiskSize string `json:"rootDiskSize,omitempty" example:"default, 30, 42, ..." default:"default"` // "default", Integer (GB): ["50", ..., "1000"]

	VmUserPassword string `json:"vmUserPassword,omitempty" default:""`
	// if ConnectionName is given, the VM tries to use associtated credential.
	// if not, it will use predefined ConnectionName in Spec objects
	ConnectionName string `json:"connectionName,omitempty" default:""`
}

// MciConnectionConfigCandidatesReq is struct for a request to check requirements to create a new MCI instance dynamically (with default resource option)
type MciConnectionConfigCandidatesReq struct {
	// CommonSpec is field for id of a spec in common namespace
	CommonSpecs []string `json:"commonSpec" validate:"required" example:"aws+ap-northeast-2+t2.small,gcp+us-west1+g1-small"`
}

// CheckMciDynamicReqInfo is struct to check requirements to create a new MCI instance dynamically (with default resource option)
type CheckMciDynamicReqInfo struct {
	ReqCheck []CheckVmDynamicReqInfo `json:"reqCheck" validate:"required"`
}

// CheckVmDynamicReqInfo is struct to check requirements to create a new server instance dynamically (with default resource option)
type CheckVmDynamicReqInfo struct {

	// ConnectionConfigCandidates will provide ConnectionConfig options
	ConnectionConfigCandidates []string `json:"connectionConfigCandidates" default:""`

	//RootDiskSize string `json:"rootDiskSize,omitempty" example:"default, 30, 42, ..."` // "default", Integer (GB): ["50", ..., "1000"]

	Spec   resource.TbSpecInfo    `json:"spec" default:""`
	Image  []resource.TbImageInfo `json:"image" default:""`
	Region common.RegionDetail    `json:"region" default:""`

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

	Location common.Location `json:"location"`

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

// MciAccessInfo is struct to retrieve overall access information of a MCI
type MciAccessInfo struct {
	MciId                 string
	MciNlbListener        *MciAccessInfo `json:"mciNlbListener,omitempty"`
	MciSubGroupAccessInfo []MciSubGroupAccessInfo
}

// MciSubGroupAccessInfo is struct for MciSubGroupAccessInfo
type MciSubGroupAccessInfo struct {
	SubGroupId      string
	NlbListener     *TbNLBListenerInfo `json:"nlbListener,omitempty"`
	BastionVmId     string
	MciVmAccessInfo []MciVmAccessInfo
}

// MciVmAccessInfo is struct for MciVmAccessInfo
type MciVmAccessInfo struct {
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

// MciRecommendReq is struct for MciRecommendReq
type MciRecommendReq struct {
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
	Priority string              `json:"priority"`
	VmSpec   resource.TbSpecInfo `json:"vmSpec"`
}

// TbVmRecommendInfo is struct for TbVmRecommendInfo
type TbVmRecommendInfo struct {
	VmReq          TbVmRecommendReq  `json:"vmReq"`
	VmPriority     []TbVmPriority    `json:"vmPriority"`
	PlacementAlgo  string            `json:"placementAlgo"`
	PlacementParam []common.KeyValue `json:"placementParam"`
}

var holdingMciMap sync.Map

// MCI and VM Provisioning

// CreateMciVm is func to post (create) MciVm
func CreateMciVm(nsId string, mciId string, vmInfoData *TbVmInfo) (*TbVmInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := &TbVmInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		temp := &TbVmInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}
	err = common.CheckString(vmInfoData.Name)
	if err != nil {
		temp := &TbVmInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}
	check, _ := CheckVm(nsId, mciId, vmInfoData.Name)

	if check {
		temp := &TbVmInfo{}
		err := fmt.Errorf("The vm " + vmInfoData.Name + " already exists.")
		return temp, err
	}

	vmInfoData.Id = vmInfoData.Name
	vmInfoData.PublicIP = "empty"
	vmInfoData.PublicDNS = "empty"
	vmInfoData.TargetAction = ActionCreate
	vmInfoData.TargetStatus = StatusRunning
	vmInfoData.Status = StatusCreating

	//goroutin
	var wg sync.WaitGroup
	wg.Add(1)

	option := "create"
	go AddVmToMci(&wg, nsId, mciId, vmInfoData, option)

	wg.Wait()

	vmStatus, err := FetchVmStatus(nsId, mciId, vmInfoData.Id)
	if err != nil {
		return nil, fmt.Errorf("Cannot find " + common.GenMciKey(nsId, mciId, vmInfoData.Id))
	}

	vmInfoData.Status = vmStatus.Status
	vmInfoData.TargetStatus = vmStatus.TargetStatus
	vmInfoData.TargetAction = vmStatus.TargetAction

	// Install CB-Dragonfly monitoring agent

	mciTmp, _ := GetMciObject(nsId, mciId)

	fmt.Printf("\n[Init monitoring agent] for %+v\n - req.InstallMonAgent: %+v\n\n", mciId, mciTmp.InstallMonAgent)

	if !strings.Contains(mciTmp.InstallMonAgent, "no") {

		// Sleep for 20 seconds for a safe DF agent installation.
		fmt.Printf("\n\n[Info] Sleep for 20 seconds for safe CB-Dragonfly Agent installation.\n\n")
		time.Sleep(20 * time.Second)

		check := CheckDragonflyEndpoint()
		if check != nil {
			fmt.Printf("\n\n[Warning] CB-Dragonfly is not available\n\n")
		} else {
			reqToMon := &MciCmdReq{}
			reqToMon.UserName = "cb-user" // this MCI user name is temporal code. Need to improve.

			fmt.Printf("\n[InstallMonitorAgentToMci]\n\n")
			content, err := InstallMonitorAgentToMci(nsId, mciId, common.StrMCI, reqToMon)
			if err != nil {
				log.Error().Err(err).Msg("")
				//mciTmp.InstallMonAgent = "no"
			}
			common.PrintJsonPretty(content)
			//mciTmp.InstallMonAgent = "yes"
		}
	}

	return vmInfoData, nil
}

// ScaleOutMciSubGroup is func to create MCI groupVM
func ScaleOutMciSubGroup(nsId string, mciId string, subGroupId string, numVMsToAdd string) (*TbMciInfo, error) {
	vmIdList, err := ListVmBySubGroup(nsId, mciId, subGroupId)
	if err != nil {
		temp := &TbMciInfo{}
		return temp, err
	}
	vmObj, err := GetVmObject(nsId, mciId, vmIdList[0])

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

	result, err := CreateMciGroupVm(nsId, mciId, vmTemplate, true)
	if err != nil {
		temp := &TbMciInfo{}
		return temp, err
	}
	return result, nil

}

// CreateMciGroupVm is func to create MCI groupVM
func CreateMciGroupVm(nsId string, mciId string, vmRequest *TbVmReq, newSubGroup bool) (*TbMciInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := &TbMciInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		temp := &TbMciInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	// returns InvalidValidationError for bad validation input, nil or ValidationErrors ( []FieldError )
	err = validate.Struct(vmRequest)
	if err != nil {

		// this check is only needed when your code could produce
		// an invalid value for validation such as interface with nil
		// value most including myself do not usually have code like this.
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Err(err).Msg("")
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

	mciTmp, err := GetMciObject(nsId, mciId)

	if err != nil {
		temp := &TbMciInfo{}
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
		log.Error().Err(err).Msg("")
		return &TbMciInfo{}, err
	}

	if subGroupSize > 0 {

		log.Info().Msg("Create MCI subGroup object")

		subGroupInfoData := TbSubGroupInfo{}
		subGroupInfoData.Id = tentativeVmId
		subGroupInfoData.Name = tentativeVmId
		subGroupInfoData.SubGroupSize = vmRequest.SubGroupSize

		key := common.GenMciSubGroupKey(nsId, mciId, vmRequest.Name)
		keyValue, err := kvstore.GetKv(key)
		if err != nil {
			err = fmt.Errorf("In CreateMciGroupVm(); kvstore.GetKv(): " + err.Error())
			log.Error().Err(err).Msg("")
		}
		if keyValue != (kvstore.KeyValue{}) {
			if newSubGroup {
				json.Unmarshal([]byte(keyValue.Value), &subGroupInfoData)
				existingVmSize, err := strconv.Atoi(subGroupInfoData.SubGroupSize)
				if err != nil {
					err = fmt.Errorf("In CreateMciGroupVm(); kvstore.GetKv(): " + err.Error())
					log.Error().Err(err).Msg("")
				}
				// add the number of existing VMs in the SubGroup with requested number for additions
				subGroupInfoData.SubGroupSize = strconv.Itoa(existingVmSize + subGroupSize)
				vmStartIndex = existingVmSize + 1
			} else {
				err = fmt.Errorf("Duplicated SubGroup ID")
				log.Error().Err(err).Msg("")
				return nil, err
			}
		}

		for i := vmStartIndex; i < subGroupSize+vmStartIndex; i++ {
			subGroupInfoData.VmId = append(subGroupInfoData.VmId, subGroupInfoData.Id+"-"+strconv.Itoa(i))
		}

		val, _ := json.Marshal(subGroupInfoData)
		err = kvstore.Put(key, string(val))
		if err != nil {
			log.Error().Err(err).Msg("")
		}
		// check stored subGroup object
		keyValue, err = kvstore.GetKv(key)
		if err != nil {
			err = fmt.Errorf("In CreateMciGroupVm(); kvstore.GetKv(): " + err.Error())
			log.Error().Err(err).Msg("")
			// return nil, err
		}

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

			log.Debug().Msg("vmInfoData.Name: " + vmInfoData.Name)

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
			log.Error().Err(err).Msg("")
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
		go AddVmToMci(&wg, nsId, mciId, &vmInfoData, "")

	}

	wg.Wait()

	//Update MCI status

	mciTmp, err = GetMciObject(nsId, mciId)
	if err != nil {
		temp := &TbMciInfo{}
		return temp, err
	}

	mciStatusTmp, _ := GetMciStatus(nsId, mciId)

	mciTmp.Status = mciStatusTmp.Status

	if mciTmp.TargetStatus == mciTmp.Status {
		mciTmp.TargetStatus = StatusComplete
		mciTmp.TargetAction = ActionComplete
	}
	UpdateMciInfo(nsId, mciTmp)

	// Install CB-Dragonfly monitoring agent

	if !strings.Contains(mciTmp.InstallMonAgent, "no") {

		// Sleep for 60 seconds for a safe DF agent installation.
		fmt.Printf("\n\n[Info] Sleep for 60 seconds for safe CB-Dragonfly Agent installation.\n\n")
		time.Sleep(60 * time.Second)

		check := CheckDragonflyEndpoint()
		if check != nil {
			fmt.Printf("\n\n[Warning] CB-Dragonfly is not available\n\n")
		} else {
			reqToMon := &MciCmdReq{}
			reqToMon.UserName = "cb-user" // this MCI user name is temporal code. Need to improve.

			fmt.Printf("\n[InstallMonitorAgentToMci]\n\n")
			content, err := InstallMonitorAgentToMci(nsId, mciId, common.StrMCI, reqToMon)
			if err != nil {
				log.Error().Err(err).Msg("")
				//mciTmp.InstallMonAgent = "no"
			}
			common.PrintJsonPretty(content)
			//mciTmp.InstallMonAgent = "yes"
		}
	}

	vmList, err := ListVmBySubGroup(nsId, mciId, tentativeVmId)

	if err != nil {
		mciTmp.SystemMessage = err.Error()
	}
	if vmList != nil {
		mciTmp.NewVmList = vmList
	}

	return &mciTmp, nil

}

// CreateMci is func to create MCI obeject and deploy requested VMs (register CSP native VM with option=register)
func CreateMci(nsId string, req *TbMciReq, option string) (*TbMciInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := &TbMciInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	// returns InvalidValidationError for bad validation input, nil or ValidationErrors ( []FieldError )
	err = validate.Struct(req)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Err(err).Msg("")
			return nil, err
		}
		return nil, err
	}

	// skip mci id checking for option=register
	if option != "register" {
		check, _ := CheckMci(nsId, req.Name)
		if check {
			err := fmt.Errorf("The mci " + req.Name + " already exists.")
			return nil, err
		}
	} else {
		req.SystemLabel = "Registered from CSP resource"
	}

	targetAction := ActionCreate
	targetStatus := StatusRunning

	mciId := req.Name
	vmRequest := req.Vm

	log.Info().Msg("Create MCI object")
	key := common.GenMciKey(nsId, mciId, "")
	mapA := map[string]string{
		"id":              mciId,
		"name":            mciId,
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
		err := fmt.Errorf("System Error: CreateMci json.Marshal(mapA) Error")
		log.Error().Err(err).Msg("")
		return nil, err
	}

	err = kvstore.Put(key, string(val))
	if err != nil {
		err := fmt.Errorf("System Error: CreateMci kvstore.Put Error")
		log.Error().Err(err).Msg("")
		return nil, err
	}

	// Check whether VM names meet requirement.
	for _, k := range vmRequest {
		err = common.CheckString(k.Name)
		if err != nil {
			log.Error().Err(err).Msg("")
			return &TbMciInfo{}, err
		}
	}

	// hold option will hold the MCI creation process until the user releases it.
	if option == "hold" {
		key := common.GenMciKey(nsId, mciId, "")
		holdingMciMap.Store(key, "holding")
		for {
			value, ok := holdingMciMap.Load(key)
			if !ok {
				break
			}
			if value == "continue" {
				holdingMciMap.Delete(key)
				break
			} else if value == "withdraw" {
				holdingMciMap.Delete(key)
				DelMci(nsId, mciId, "force")
				err := fmt.Errorf("Withdrawed MCI creation")
				log.Error().Err(err).Msg("")
				return nil, err
			}

			log.Info().Msgf("MCI: %s (holding)", key)
			time.Sleep(5 * time.Second)
		}
		option = "create"
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

			log.Info().Msg("Create MCI subGroup object")
			key := common.GenMciSubGroupKey(nsId, mciId, k.Name)

			subGroupInfoData := TbSubGroupInfo{}
			subGroupInfoData.Id = common.ToLower(k.Name)
			subGroupInfoData.Name = common.ToLower(k.Name)
			subGroupInfoData.SubGroupSize = k.SubGroupSize

			for i := vmStartIndex; i < subGroupSize+vmStartIndex; i++ {
				subGroupInfoData.VmId = append(subGroupInfoData.VmId, subGroupInfoData.Id+"-"+strconv.Itoa(i))
			}

			val, _ := json.Marshal(subGroupInfoData)
			err := kvstore.Put(key, string(val))
			if err != nil {
				log.Error().Err(err).Msg("")
			}

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

				log.Debug().Msg("vmInfoData.Name: " + vmInfoData.Name)

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
				log.Error().Err(err).Msg("")
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
			go AddVmToMci(&wg, nsId, mciId, &vmInfoData, option)
			//AddVmToMci(nsId, req.Id, vmInfoData)

		}
	}
	wg.Wait()

	mciTmp, err := GetMciObject(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	mciStatusTmp, err := GetMciStatus(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	mciTmp.Status = mciStatusTmp.Status

	if mciTmp.TargetStatus == mciTmp.Status {
		mciTmp.TargetStatus = StatusComplete
		mciTmp.TargetAction = ActionComplete
	}
	UpdateMciInfo(nsId, mciTmp)

	log.Debug().Msg("[MCI has been created]" + mciId)

	// Install CB-Dragonfly monitoring agent

	mciTmp.InstallMonAgent = req.InstallMonAgent
	UpdateMciInfo(nsId, mciTmp)

	if !strings.Contains(mciTmp.InstallMonAgent, "no") && option != "register" {

		check := CheckDragonflyEndpoint()
		if check != nil {
			fmt.Printf("\n\n[Warning] CB-Dragonfly is not available\n\n")
		} else {
			reqToMon := &MciCmdReq{}
			reqToMon.UserName = "cb-user" // this MCI user name is temporal code. Need to improve.

			fmt.Printf("\n===========================\n")
			// Sleep for 60 seconds for a safe DF agent installation.
			fmt.Printf("\n\n[Info] Sleep for 60 seconds for safe CB-Dragonfly Agent installation.\n")
			time.Sleep(60 * time.Second)

			fmt.Printf("\n[InstallMonitorAgentToMci]\n\n")
			content, err := InstallMonitorAgentToMci(nsId, mciId, common.StrMCI, reqToMon)
			if err != nil {
				log.Error().Err(err).Msg("")
				//mciTmp.InstallMonAgent = "no"
			}
			common.PrintJsonPretty(content)
			//mciTmp.InstallMonAgent = "yes"
		}
	}

	mciResult, err := GetMciInfo(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	return mciResult, nil
}

// CheckMciDynamicReq is func to check request info to create MCI obeject and deploy requested VMs in a dynamic way
func CheckMciDynamicReq(req *MciConnectionConfigCandidatesReq) (*CheckMciDynamicReqInfo, error) {

	mciReqInfo := CheckMciDynamicReqInfo{}

	connectionConfigList, err := common.GetConnConfigList(common.DefaultCredentialHolder, true, true)
	if err != nil {
		err := fmt.Errorf("Cannot load ConnectionConfigList in MCI dynamic request check.")
		log.Error().Err(err).Msg("")
		return &mciReqInfo, err
	}

	// Find detail info and ConnectionConfigCandidates
	for _, k := range req.CommonSpecs {
		errMessage := ""

		vmReqInfo := CheckVmDynamicReqInfo{}

		specInfo, err := resource.GetSpec(common.SystemCommonNs, k)
		if err != nil {
			log.Error().Err(err).Msg("")
			errMessage += "//Failed to get Spec (" + k + ")."
		}

		regionInfo, err := common.GetRegion(specInfo.ProviderName, specInfo.RegionName)
		if err != nil {
			errMessage += "//Failed to get Region (" + specInfo.RegionName + ") for Spec (" + k + ") is not found."
		}

		for _, connectionConfig := range connectionConfigList.Connectionconfig {
			if connectionConfig.ProviderName == specInfo.ProviderName && strings.Contains(connectionConfig.RegionDetail.RegionName, specInfo.RegionName) {
				vmReqInfo.ConnectionConfigCandidates = append(vmReqInfo.ConnectionConfigCandidates, connectionConfig.ConfigName)
			}
		}

		vmReqInfo.Spec = specInfo
		imageSearchKey := specInfo.ProviderName + "+" + specInfo.RegionName
		availableImageList, err := resource.SearchImage(common.SystemCommonNs, imageSearchKey)
		if err != nil {
			errMessage += "//Failed to search images for Spec (" + k + ")"
		}
		vmReqInfo.Image = availableImageList
		vmReqInfo.Region = regionInfo
		vmReqInfo.SystemMessage = errMessage
		mciReqInfo.ReqCheck = append(mciReqInfo.ReqCheck, vmReqInfo)
	}

	return &mciReqInfo, err
}

// CreateSystemMciDynamic is func to create MCI obeject and deploy requested VMs in a dynamic way
func CreateSystemMciDynamic(option string) (*TbMciInfo, error) {
	nsId := common.SystemCommonNs
	req := &TbMciDynamicReq{}

	// special purpose MCI
	req.Name = option
	req.Label = option
	req.SystemLabel = option
	req.Description = option
	req.InstallMonAgent = "no"

	switch option {
	case "probe":
		connections, err := common.GetConnConfigList(common.DefaultCredentialHolder, true, true)
		if err != nil {
			log.Error().Err(err).Msg("")
			return nil, err
		}
		for _, v := range connections.Connectionconfig {

			vmReq := &TbVmDynamicReq{}
			vmReq.CommonImage = "ubuntu18.04"                // temporal default value. will be changed
			vmReq.CommonSpec = "aws-ap-northeast-2-t2-small" // temporal default value. will be changed

			deploymentPlan := DeploymentPlan{}
			condition := []Operation{}
			condition = append(condition, Operation{Operand: v.RegionZoneInfoName})

			log.Debug().Msg(" - v.RegionName: " + v.RegionZoneInfoName)

			deploymentPlan.Filter.Policy = append(deploymentPlan.Filter.Policy, FilterCondition{Metric: "region", Condition: condition})
			deploymentPlan.Limit = "1"
			common.PrintJsonPretty(deploymentPlan)

			specList, err := RecommendVm(common.SystemCommonNs, deploymentPlan)
			if err != nil {
				log.Error().Err(err).Msg("")
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

	return CreateMciDynamic("", nsId, req, "")
}

// CreateMciDynamic is func to create MCI obeject and deploy requested VMs in a dynamic way
func CreateMciDynamic(reqID string, nsId string, req *TbMciDynamicReq, deployOption string) (*TbMciInfo, error) {

	mciReq := TbMciReq{}
	mciReq.Name = req.Name
	mciReq.Label = req.Label
	mciReq.SystemLabel = req.SystemLabel
	mciReq.InstallMonAgent = req.InstallMonAgent
	mciReq.Description = req.Description

	emptyMci := &TbMciInfo{}
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyMci, err
	}
	check, err := CheckMci(nsId, req.Name)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyMci, err
	}
	if check {
		err := fmt.Errorf("The mci " + req.Name + " already exists.")
		return emptyMci, err
	}

	vmRequest := req.Vm
	// Check whether VM names meet requirement.
	errStr := ""
	for i, k := range vmRequest {
		err = checkCommonResAvailable(&k)
		if err != nil {
			log.Error().Err(err).Msgf("[%d] Failed to find common resource for MCI provision", i)
			errStr += "{[" + strconv.Itoa(i+1) + "] " + err.Error() + "} "
		}
	}
	if errStr != "" {
		err = fmt.Errorf(errStr)
		return emptyMci, err
	}

	//If not, generate default resources dynamically.
	for _, k := range vmRequest {
		vmReq, err := getVmReqFromDynamicReq(reqID, nsId, &k)
		if err != nil {
			log.Error().Err(err).Msg("Failed to prefare resources for dynamic MCI creation")
			// Rollback created default resources
			time.Sleep(5 * time.Second)
			log.Info().Msg("Try rollback created default resources")
			rollbackResult, rollbackErr := resource.DelAllSharedResources(nsId)
			if rollbackErr != nil {
				err = fmt.Errorf("Failed in rollback operation: %w", rollbackErr)
			} else {
				ids := strings.Join(rollbackResult.IdList, ", ")
				err = fmt.Errorf("Rollback results [%s]: %w", ids, err)
			}
			return emptyMci, err
		}
		mciReq.Vm = append(mciReq.Vm, *vmReq)
	}

	common.PrintJsonPretty(mciReq)
	common.UpdateRequestProgress(reqID, common.ProgressInfo{Title: "Prepared all resources for provisioning MCI:" + mciReq.Name, Info: mciReq, Time: time.Now()})
	common.UpdateRequestProgress(reqID, common.ProgressInfo{Title: "Start provisioning", Time: time.Now()})

	// Run create MCI with the generated MCI request (option != register)
	option := "create"
	if deployOption == "hold" {
		option = "hold"
	}
	return CreateMci(nsId, &mciReq, option)
}

// CreateMciVmDynamic is func to create requested VM in a dynamic way and add it to MCI
func CreateMciVmDynamic(nsId string, mciId string, req *TbVmDynamicReq) (*TbMciInfo, error) {

	emptyMci := &TbMciInfo{}
	subGroupId := req.Name
	check, err := CheckSubGroup(nsId, mciId, subGroupId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyMci, err
	}
	if check {
		err := fmt.Errorf("The name for SubGroup (prefix of VM Id) " + req.Name + " already exists.")
		return emptyMci, err
	}

	vmReq, err := getVmReqFromDynamicReq("", nsId, req)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyMci, err
	}

	return CreateMciGroupVm(nsId, mciId, vmReq, true)
}

// checkCommonResAvailable is func to check common resources availability
func checkCommonResAvailable(req *TbVmDynamicReq) error {

	vmRequest := req
	// Check whether VM names meet requirement.
	k := vmRequest

	vmReq := &TbVmReq{}

	specInfo, err := resource.GetSpec(common.SystemCommonNs, req.CommonSpec)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	// remake vmReqest from given input and check resource availability
	vmReq.ConnectionName = specInfo.ConnectionName

	// If ConnectionName is specified by the request, Use ConnectionName from the request
	if k.ConnectionName != "" {
		vmReq.ConnectionName = k.ConnectionName
	}

	// validate the GetConnConfig for spec
	connection, err := common.GetConnConfig(vmReq.ConnectionName)
	if err != nil {
		err := fmt.Errorf("Failed to get ConnectionName (" + vmReq.ConnectionName + ") for Spec (" + k.CommonSpec + ") is not found.")
		log.Error().Err(err).Msg("")
		return err
	}

	osType := strings.ReplaceAll(k.CommonImage, " ", "")
	vmReq.ImageId = resource.GetProviderRegionZoneResourceKey(connection.ProviderName, connection.RegionDetail.RegionName, "", osType)
	// incase of user provided image id completely (e.g. aws+ap-northeast-2+ubuntu22.04)
	if strings.Contains(k.CommonImage, "+") {
		vmReq.ImageId = k.CommonImage
	}
	_, err = resource.GetImage(common.SystemCommonNs, vmReq.ImageId)
	if err != nil {
		err := fmt.Errorf("Failed to get Image " + k.CommonImage + " from " + vmReq.ConnectionName)
		log.Error().Err(err).Msg("")
		return err
	}

	return nil
}

// getVmReqForDynamicMci is func to getVmReqFromDynamicReq
func getVmReqFromDynamicReq(reqID string, nsId string, req *TbVmDynamicReq) (*TbVmReq, error) {

	onDemand := true

	vmRequest := req
	// Check whether VM names meet requirement.
	k := vmRequest

	vmReq := &TbVmReq{}

	specInfo, err := resource.GetSpec(common.SystemCommonNs, req.CommonSpec)
	if err != nil {
		log.Error().Err(err).Msg("")
		return &TbVmReq{}, err
	}

	// remake vmReqest from given input and check resource availability
	vmReq.ConnectionName = specInfo.ConnectionName

	// If ConnectionName is specified by the request, Use ConnectionName from the request
	if k.ConnectionName != "" {
		vmReq.ConnectionName = k.ConnectionName
	}

	// validate the GetConnConfig for spec
	connection, err := common.GetConnConfig(vmReq.ConnectionName)
	if err != nil {
		err := fmt.Errorf("Failed to get ConnectionName (" + vmReq.ConnectionName + ") for Spec (" + k.CommonSpec + ") is not found.")
		log.Error().Err(err).Msg("")
		return &TbVmReq{}, err
	}

	// Default resource name has this pattern (nsId + "-shared-" + vmReq.ConnectionName)
	resourceName := nsId + common.StrSharedResourceName + vmReq.ConnectionName

	vmReq.SpecId = specInfo.Id
	osType := strings.ReplaceAll(k.CommonImage, " ", "")
	vmReq.ImageId = resource.GetProviderRegionZoneResourceKey(connection.ProviderName, connection.RegionDetail.RegionName, "", osType)
	// incase of user provided image id completely (e.g. aws+ap-northeast-2+ubuntu22.04)
	if strings.Contains(k.CommonImage, "+") {
		vmReq.ImageId = k.CommonImage
	}
	_, err = resource.GetImage(common.SystemCommonNs, vmReq.ImageId)
	if err != nil {
		err := fmt.Errorf("Failed to get the Image " + vmReq.ImageId + " from " + vmReq.ConnectionName)
		log.Error().Err(err).Msg("")
		return &TbVmReq{}, err
	}

	common.UpdateRequestProgress(reqID, common.ProgressInfo{Title: "Setting vNet:" + resourceName, Time: time.Now()})

	vmReq.VNetId = resourceName
	_, err = resource.GetResource(nsId, common.StrVNet, vmReq.VNetId)
	if err != nil {
		if !onDemand {
			err := fmt.Errorf("Failed to get the vNet " + vmReq.VNetId + " from " + vmReq.ConnectionName)
			log.Error().Err(err).Msg("Failed to get the vNet")
			return &TbVmReq{}, err
		}
		common.UpdateRequestProgress(reqID, common.ProgressInfo{Title: "Loading default vNet:" + resourceName, Time: time.Now()})
		err2 := resource.LoadSharedResource(nsId, common.StrVNet, vmReq.ConnectionName)
		if err2 != nil {
			log.Error().Err(err2).Msg("Failed to create new default vNet " + vmReq.VNetId + " from " + vmReq.ConnectionName)
			return &TbVmReq{}, err2
		} else {
			log.Info().Msg("Created new default vNet: " + vmReq.VNetId)
		}
	} else {
		log.Info().Msg("Found and utilize default vNet: " + vmReq.VNetId)
	}
	vmReq.SubnetId = resourceName

	common.UpdateRequestProgress(reqID, common.ProgressInfo{Title: "Setting SSHKey:" + resourceName, Time: time.Now()})
	vmReq.SshKeyId = resourceName
	_, err = resource.GetResource(nsId, common.StrSSHKey, vmReq.SshKeyId)
	if err != nil {
		if !onDemand {
			err := fmt.Errorf("Failed to get the SSHKey " + vmReq.SshKeyId + " from " + vmReq.ConnectionName)
			log.Error().Err(err).Msg("Failed to get the SSHKey")
			return &TbVmReq{}, err
		}
		common.UpdateRequestProgress(reqID, common.ProgressInfo{Title: "Loading default SSHKey:" + resourceName, Time: time.Now()})
		err2 := resource.LoadSharedResource(nsId, common.StrSSHKey, vmReq.ConnectionName)
		if err2 != nil {
			log.Error().Err(err2).Msg("Failed to create new default SSHKey " + vmReq.SshKeyId + " from " + vmReq.ConnectionName)
			return &TbVmReq{}, err2
		} else {
			log.Info().Msg("Created new default SSHKey: " + vmReq.VNetId)
		}
	} else {
		log.Info().Msg("Found and utilize default SSHKey: " + vmReq.VNetId)
	}

	common.UpdateRequestProgress(reqID, common.ProgressInfo{Title: "Setting securityGroup:" + resourceName, Time: time.Now()})
	securityGroup := resourceName
	vmReq.SecurityGroupIds = append(vmReq.SecurityGroupIds, securityGroup)
	_, err = resource.GetResource(nsId, common.StrSecurityGroup, securityGroup)
	if err != nil {
		if !onDemand {
			err := fmt.Errorf("Failed to get the securityGroup " + securityGroup + " from " + vmReq.ConnectionName)
			log.Error().Err(err).Msg("Failed to get the securityGroup")
			return &TbVmReq{}, err
		}
		common.UpdateRequestProgress(reqID, common.ProgressInfo{Title: "Loading default securityGroup:" + resourceName, Time: time.Now()})
		err2 := resource.LoadSharedResource(nsId, common.StrSecurityGroup, vmReq.ConnectionName)
		if err2 != nil {
			log.Error().Err(err2).Msg("Failed to create new default securityGroup " + securityGroup + " from " + vmReq.ConnectionName)
			return &TbVmReq{}, err2
		} else {
			log.Info().Msg("Created new default securityGroup: " + securityGroup)
		}
	} else {
		log.Info().Msg("Found and utilize default securityGroup: " + securityGroup)
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
	common.UpdateRequestProgress(reqID, common.ProgressInfo{Title: "Prepared resources for VM:" + vmReq.Name, Info: vmReq, Time: time.Now()})

	return vmReq, nil
}

// AddVmToMci is func to add VM to MCI
func AddVmToMci(wg *sync.WaitGroup, nsId string, mciId string, vmInfoData *TbVmInfo, option string) error {
	log.Debug().Msg("Start to add VM To MCI")
	//goroutin
	defer wg.Done()

	key := common.GenMciKey(nsId, mciId, "")
	keyValue, err := kvstore.GetKv(key)
	if err != nil {
		log.Fatal().Err(err).Msg("AddVmToMci(); kvstore.GetKv() returned an error.")
		return err
	}
	if keyValue == (kvstore.KeyValue{}) {
		return fmt.Errorf("AddVmToMci: Cannot find mciId. Key: %s", key)
	}

	// Make VM object
	key = common.GenMciKey(nsId, mciId, vmInfoData.Id)
	val, _ := json.Marshal(vmInfoData)
	err = kvstore.Put(key, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	configTmp, err := common.GetConnConfig(vmInfoData.ConnectionName)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	vmInfoData.Location = configTmp.RegionDetail.Location

	//AddVmInfoToMci(nsId, mciId, *vmInfoData)
	// Update VM object
	val, _ = json.Marshal(vmInfoData)
	err = kvstore.Put(key, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	//instanceIds, publicIPs := CreateVm(&vmInfoData)
	err = CreateVm(nsId, mciId, vmInfoData, option)

	if err != nil {
		vmInfoData.Status = StatusFailed
		vmInfoData.SystemMessage = err.Error()
		UpdateVmInfo(nsId, mciId, *vmInfoData)
		log.Error().Err(err).Msg("")
		return err
	}

	// set initial TargetAction, TargetStatus
	vmInfoData.TargetAction = ActionComplete
	vmInfoData.TargetStatus = StatusComplete

	// get and set current vm status
	vmStatusInfoTmp, err := FetchVmStatus(nsId, mciId, vmInfoData.Id)

	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	vmInfoData.Status = vmStatusInfoTmp.Status

	// Monitoring Agent Installation Status (init: notInstalled)
	vmInfoData.MonAgentStatus = "notInstalled"
	vmInfoData.NetworkAgentStatus = "notInstalled"

	// set CreatedTime
	t := time.Now()
	vmInfoData.CreatedTime = t.Format("2006-01-02 15:04:05")
	log.Debug().Msg(vmInfoData.CreatedTime)

	UpdateVmInfo(nsId, mciId, *vmInfoData)

	return nil

}

// CreateVm is func to create VM (option = "register" for register existing VM)
func CreateVm(nsId string, mciId string, vmInfoData *TbVmInfo, option string) error {

	var err error = nil
	switch {
	case vmInfoData.Name == "":
		err = fmt.Errorf("vmInfoData.Name is empty")
	case vmInfoData.ImageId == "":
		err = fmt.Errorf("vmInfoData.ImageId is empty")
	case vmInfoData.ConnectionName == "":
		err = fmt.Errorf("vmInfoData.ConnectionName is empty")
	case vmInfoData.SshKeyId == "":
		err = fmt.Errorf("vmInfoData.SshKeyId is empty")
	case vmInfoData.SpecId == "":
		err = fmt.Errorf("vmInfoData.SpecId is empty")
	case vmInfoData.SecurityGroupIds == nil:
		err = fmt.Errorf("vmInfoData.SecurityGroupIds is empty")
	case vmInfoData.VNetId == "":
		err = fmt.Errorf("vmInfoData.VNetId is empty")
	case vmInfoData.SubnetId == "":
		err = fmt.Errorf("vmInfoData.SubnetId is empty")
	default:
	}
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	// in case of registering existing CSP VM
	if option == "register" {
		// IdByCSP is required
		if vmInfoData.IdByCSP == "" {
			err := fmt.Errorf("vmInfoData.IdByCSP is empty (required for register VM)")
			log.Error().Err(err).Msg("")
			return err
		}
	}

	var callResult SpiderVMInfo

	// Fill VM creation reqest (request to cb-spider)
	requestBody := SpiderVMReqInfoWrapper{}
	requestBody.ConnectionName = vmInfoData.ConnectionName

	//generate VM ID(Name) to request to CSP(Spider)
	requestBody.ReqInfo.Name = common.GenUid()

	customImageFlag := false

	requestBody.ReqInfo.VMUserId = vmInfoData.VmUserAccount
	requestBody.ReqInfo.VMUserPasswd = vmInfoData.VmUserPassword
	// provide a random passwd, if it is not provided by user (the passwd required for Windows)
	if requestBody.ReqInfo.VMUserPasswd == "" {
		// assign random string (mixed Uid style)
		requestBody.ReqInfo.VMUserPasswd = common.GenRandomPassword(14)
	}

	requestBody.ReqInfo.RootDiskType = vmInfoData.RootDiskType
	requestBody.ReqInfo.RootDiskSize = vmInfoData.RootDiskSize

	if option == "register" {
		requestBody.ReqInfo.CSPid = vmInfoData.IdByCSP

	} else {
		// Try lookup customImage
		requestBody.ReqInfo.ImageName, err = resource.GetCspResourceId(nsId, common.StrCustomImage, vmInfoData.ImageId)
		if requestBody.ReqInfo.ImageName == "" || err != nil {
			log.Warn().Msgf("Not found %s from CustomImage in ns: %s, find it from UserImage", vmInfoData.ImageId, nsId)
			errAgg := err.Error()
			// If customImage doesn't exist, then try lookup image
			requestBody.ReqInfo.ImageName, err = resource.GetCspResourceId(nsId, common.StrImage, vmInfoData.ImageId)
			if requestBody.ReqInfo.ImageName == "" || err != nil {
				log.Warn().Msgf("Not found %s from UserImage in ns: %s, find CommonImage from SystemCommonNs", vmInfoData.ImageId, nsId)
				errAgg += err.Error()
				// If cannot find the resource, use common resource
				requestBody.ReqInfo.ImageName, err = resource.GetCspResourceId(common.SystemCommonNs, common.StrImage, vmInfoData.ImageId)
				if requestBody.ReqInfo.ImageName == "" || err != nil {
					errAgg += err.Error()
					err = fmt.Errorf(errAgg)
					log.Error().Err(err).Msgf("Not found %s both from ns %s and SystemCommonNs", vmInfoData.ImageId, nsId)
					return err
				} else {
					log.Info().Msgf("Use the CommonImage: %s in SystemCommonNs", requestBody.ReqInfo.ImageName)
				}
			} else {
				log.Info().Msgf("Use the UserImage: %s in ns: %s", requestBody.ReqInfo.ImageName, nsId)
			}
		} else {
			customImageFlag = true
			requestBody.ReqInfo.ImageType = MyImage
			// If the requested image is a custom image (generated by VM snapshot), RootDiskType should be empty.
			// TB ignore inputs for RootDiskType, RootDiskSize
			requestBody.ReqInfo.RootDiskType = ""
			requestBody.ReqInfo.RootDiskSize = ""
		}

		requestBody.ReqInfo.VMSpecName, err = resource.GetCspResourceId(nsId, common.StrSpec, vmInfoData.SpecId)
		if requestBody.ReqInfo.VMSpecName == "" || err != nil {
			log.Warn().Msgf("Not found the Spec: %s in nsId: %s, find it from SystemCommonNs", vmInfoData.SpecId, nsId)
			errAgg := err.Error()
			// If cannot find the resource, use common resource
			requestBody.ReqInfo.VMSpecName, err = resource.GetCspResourceId(common.SystemCommonNs, common.StrSpec, vmInfoData.SpecId)
			log.Info().Msgf("Use the common VMSpecName: %s", requestBody.ReqInfo.VMSpecName)

			if requestBody.ReqInfo.ImageName == "" || err != nil {
				errAgg += err.Error()
				err = fmt.Errorf(errAgg)
				log.Error().Err(err).Msg("")
				return err
			}
		}

		requestBody.ReqInfo.VPCName, err = resource.GetCspResourceId(nsId, common.StrVNet, vmInfoData.VNetId)
		if requestBody.ReqInfo.VPCName == "" {
			log.Error().Err(err).Msg("")
			return err
		}

		// TODO: needs to be enhnaced to use GetCspResourceId (GetCspResourceId needs to be updated as well)
		requestBody.ReqInfo.SubnetName = vmInfoData.SubnetId //resource.GetCspResourceId(nsId, common.StrVNet, vmInfoData.SubnetId)
		if requestBody.ReqInfo.SubnetName == "" {
			log.Error().Err(err).Msg("")
			return err
		}

		var SecurityGroupIdsTmp []string
		for _, v := range vmInfoData.SecurityGroupIds {
			CspSgId, err := resource.GetCspResourceId(nsId, common.StrSecurityGroup, v)
			if CspSgId == "" {
				log.Error().Err(err).Msg("")
				return err
			}

			SecurityGroupIdsTmp = append(SecurityGroupIdsTmp, CspSgId)
		}
		requestBody.ReqInfo.SecurityGroupNames = SecurityGroupIdsTmp

		var DataDiskIdsTmp []string
		for _, v := range vmInfoData.DataDiskIds {
			// ignore DataDiskIds == "", assume it is ignorable mistake
			if v != "" {
				CspDataDiskId, err := resource.GetCspResourceId(nsId, common.StrDataDisk, v)
				if err != nil || CspDataDiskId == "" {
					log.Error().Err(err).Msg("")
					return err
				}
				DataDiskIdsTmp = append(DataDiskIdsTmp, CspDataDiskId)
			}
		}
		requestBody.ReqInfo.DataDiskNames = DataDiskIdsTmp

		requestBody.ReqInfo.KeyPairName, err = resource.GetCspResourceId(nsId, common.StrSSHKey, vmInfoData.SshKeyId)
		if requestBody.ReqInfo.KeyPairName == "" {
			log.Error().Err(err).Msg("")
			return err
		}
	}

	log.Info().Msg("VM request body to CB-Spider")
	common.PrintJsonPretty(requestBody)

	// Randomly sleep within 20 Secs to avoid rateLimit from CSP
	common.RandomSleep(0, 20)
	client := resty.New()
	method := "POST"
	client.SetTimeout(20 * time.Minute)

	url := common.SpiderRestUrl + "/vm"
	if option == "register" {
		url = common.SpiderRestUrl + "/regvm"
	}

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		&callResult,
		common.MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("Spider returned an error")
		return err
	}

	vmInfoData.CspViewVmDetail = callResult
	vmInfoData.VmUserAccount = callResult.VMUserId
	vmInfoData.VmUserPassword = callResult.VMUserPasswd
	//vmInfoData.Location = vmInfoData.Location
	//vmInfoData.PlacementAlgo = vmInfoData.PlacementAlgo
	//vmInfoData.CspVmId = temp.Id
	//vmInfoData.StartTime = temp.StartTime
	vmInfoData.Region = callResult.Region
	vmInfoData.PublicIP = callResult.PublicIP
	vmInfoData.SSHPort, _ = TrimIP(callResult.SSHAccessPoint)
	vmInfoData.PublicDNS = callResult.PublicDNS
	vmInfoData.PrivateIP = callResult.PrivateIP
	vmInfoData.PrivateDNS = callResult.PrivateDNS
	vmInfoData.RootDiskType = callResult.RootDiskType
	vmInfoData.RootDiskSize = callResult.RootDiskSize
	vmInfoData.RootDeviceName = callResult.RootDeviceName
	//configTmp, _ := common.GetConnConfig(vmInfoData.ConnectionName)

	if option == "register" {

		// Reconstuct resource IDs
		// vNet
		resourceListInNs, err := resource.ListResource(nsId, common.StrVNet, "cspVNetName", callResult.VpcIID.NameId)
		if err != nil {
			log.Error().Err(err).Msg("")
		} else {
			resourcesInNs := resourceListInNs.([]resource.TbVNetInfo) // type assertion
			for _, resource := range resourcesInNs {
				if resource.ConnectionName == requestBody.ConnectionName {
					vmInfoData.VNetId = resource.Id
					//vmInfoData.SubnetId = resource.SubnetInfoList
				}
			}
		}

		// access Key
		resourceListInNs, err = resource.ListResource(nsId, common.StrSSHKey, "cspSshKeyName", callResult.KeyPairIId.NameId)
		if err != nil {
			log.Error().Err(err).Msg("")
		} else {
			resourcesInNs := resourceListInNs.([]resource.TbSshKeyInfo) // type assertion
			for _, resource := range resourcesInNs {
				if resource.ConnectionName == requestBody.ConnectionName {
					vmInfoData.SshKeyId = resource.Id
				}
			}
		}

	} else {
		vmKey := common.GenMciKey(nsId, mciId, vmInfoData.Id)

		if customImageFlag == false {
			resource.UpdateAssociatedObjectList(nsId, common.StrImage, vmInfoData.ImageId, common.StrAdd, vmKey)
		} else {
			resource.UpdateAssociatedObjectList(nsId, common.StrCustomImage, vmInfoData.ImageId, common.StrAdd, vmKey)
		}

		//resource.UpdateAssociatedObjectList(nsId, common.StrSpec, vmInfoData.SpecId, common.StrAdd, vmKey)
		resource.UpdateAssociatedObjectList(nsId, common.StrSSHKey, vmInfoData.SshKeyId, common.StrAdd, vmKey)
		resource.UpdateAssociatedObjectList(nsId, common.StrVNet, vmInfoData.VNetId, common.StrAdd, vmKey)

		for _, v := range vmInfoData.SecurityGroupIds {
			resource.UpdateAssociatedObjectList(nsId, common.StrSecurityGroup, v, common.StrAdd, vmKey)
		}

		for _, v := range vmInfoData.DataDiskIds {
			resource.UpdateAssociatedObjectList(nsId, common.StrDataDisk, v, common.StrAdd, vmKey)
		}
	}

	// Register dataDisks which are created with the creation of VM
	for _, v := range callResult.DataDiskIIDs {
		tbDataDiskReq := resource.TbDataDiskReq{
			Name:           v.NameId,
			ConnectionName: vmInfoData.ConnectionName,
			// CspDataDiskId:  v.NameId, // v.SystemId ? IdByCsp ?
		}

		dataDisk, err := resource.CreateDataDisk(nsId, &tbDataDiskReq, "register")
		if err != nil {
			err = fmt.Errorf("After starting VM %s, failed to register dataDisk %s. \n", vmInfoData.Name, v.NameId)
			// continue
		}

		vmInfoData.DataDiskIds = append(vmInfoData.DataDiskIds, dataDisk.Id)

		vmKey := common.GenMciKey(nsId, mciId, vmInfoData.Id)
		resource.UpdateAssociatedObjectList(nsId, common.StrDataDisk, dataDisk.Id, common.StrAdd, vmKey)
	}

	UpdateVmInfo(nsId, mciId, *vmInfoData)

	// Assign a Bastion if none (randomly)
	_, err = SetBastionNodes(nsId, mciId, vmInfoData.Id, "")
	if err != nil {
		// just log error and continue
		log.Info().Err(err).Msg("")
	}

	return nil
}
