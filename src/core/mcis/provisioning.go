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

	// Label is for describing the mcis in a keyword (any string can be used)
	Label string `json:"label" example:"User custom label"`

	// SystemLabel is for describing the mcis in a keyword (any string can be used) for special System purpose
	SystemLabel string `json:"systemLabel" example:"Managed by CB-Tumblebug" default:""`

	PlacementAlgo string     `json:"placementAlgo,omitempty"`
	Description   string     `json:"description"`
	Vm            []TbVmInfo `json:"vm"`
}

// TbVmReq is struct to get requirements to create a new server instance
type TbVmReq struct {
	// VM name or VM group name if is (not empty) && (> 0). If it is a group, actual VM name will be generated with -N postfix.
	Name string `json:"name" validate:"required" example:"vm01"`

	// if vmGroupSize is (not empty) && (> 0), VM group will be gernetad. VMs will be created accordingly.
	VmGroupSize string `json:"vmGroupSize" example:"3" default:""`

	Label string `json:"label"`

	Description string `json:"description" example:"Description"`

	ConnectionName   string   `json:"connectionName" validate:"required" example:"testcloud01-seoul"`
	SpecId           string   `json:"specId" validate:"required"`
	ImageId          string   `json:"imageId" validate:"required"`
	VNetId           string   `json:"vNetId" validate:"required"`
	SubnetId         string   `json:"subnetId"`
	SecurityGroupIds []string `json:"securityGroupIds" validate:"required"`
	SshKeyId         string   `json:"sshKeyId" validate:"required"`
	VmUserAccount    string   `json:"vmUserAccount,omitempty"`
	VmUserPassword   string   `json:"vmUserPassword,omitempty"`
}

// TbVmDynamicReq is struct to get requirements to create a new server instance dynamically (with default resource option)
type TbVmDynamicReq struct {
	// VM name or VM group name if is (not empty) && (> 0). If it is a group, actual VM name will be generated with -N postfix.
	Name string `json:"name" example:"vm01"`

	// if vmGroupSize is (not empty) && (> 0), VM group will be gernetad. VMs will be created accordingly.
	VmGroupSize string `json:"vmGroupSize" example:"3" default:""`

	Label string `json:"label" example:"DynamicVM"`

	Description string `json:"description" example:"Description"`

	// CommonSpec is field for id of a spec in common namespace
	CommonSpec string `json:"commonSpec" validate:"required" example:"aws-ap-northeast-2-t2-small"`
	// CommonImage is field for id of a image in common namespace
	CommonImage string `json:"commonImage" validate:"required" example:"ubuntu18.04"`
}

// SpiderVMReqInfoWrapper is struct from CB-Spider (VMHandler.go) for wrapping SpiderVMInfo
type SpiderVMReqInfoWrapper struct { // Spider
	ConnectionName string
	ReqInfo        SpiderVMInfo
}

// SpiderVMInfo is struct from CB-Spider for VM information
type SpiderVMInfo struct { // Spider
	// Fields for request
	Name               string
	ImageName          string
	VPCName            string
	SubnetName         string
	SecurityGroupNames []string
	KeyPairName        string

	// Fields for both request and response
	VMSpecName   string //  instance type or flavour, etc... ex) t2.micro or f1.micro
	VMUserId     string // ex) user1
	VMUserPasswd string

	// Fields for response
	IId               common.IID // {NameId, SystemId}
	ImageIId          common.IID
	VpcIID            common.IID
	SubnetIID         common.IID   // AWS, ex) subnet-8c4a53e4
	SecurityGroupIIds []common.IID // AWS, ex) sg-0b7452563e1121bb6
	KeyPairIId        common.IID
	StartTime         time.Time  // Timezone: based on cloud-barista server location.
	Region            RegionInfo //  ex) {us-east1, us-east1-c} or {ap-northeast-2}
	NetworkInterface  string     // ex) eth0
	PublicIP          string
	PublicDNS         string
	PrivateIP         string
	PrivateDNS        string
	VMBootDisk        string // ex) /dev/sda1
	VMBlockDisk       string // ex)
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

// TbVmGroupInfo is struct to define an object that includes homogeneous VMs
type TbVmGroupInfo struct {
	Id          string   `json:"id"`
	Name        string   `json:"name"`
	VmId        []string `json:"vmId"`
	VmGroupSize string   `json:"vmGroupSize"`
}

// TbVmInfo is struct to define a server instance object
type TbVmInfo struct {
	Id   string `json:"id"`
	Name string `json:"name"`

	// defined if the VM is in a group
	VmGroupId string `json:"vmGroupId"`

	Location common.GeoLocation `json:"location"`

	// Required by CB-Tumblebug
	Status       string `json:"status"`
	TargetStatus string `json:"targetStatus"`
	TargetAction string `json:"targetAction"`

	// Montoring agent status
	MonAgentStatus string `json:"monAgentStatus" example:"[installed, notInstalled, failed]"` // yes or no// installed, notInstalled, failed

	// Latest system message such as error message
	SystemMessage string `json:"systemMessage" example:"Failed because ..." default:""` // systeam-given string message

	// Created time
	CreatedTime string `json:"createdTime" example:"2022-11-10 23:00:00" default:""`

	Label       string `json:"label"`
	Description string `json:"description"`

	Region      RegionInfo `json:"region"` // AWS, ex) {us-east1, us-east1-c} or {ap-northeast-2}
	PublicIP    string     `json:"publicIP"`
	SSHPort     string     `json:"sshPort"`
	PublicDNS   string     `json:"publicDNS"`
	PrivateIP   string     `json:"privateIP"`
	PrivateDNS  string     `json:"privateDNS"`
	VMBootDisk  string     `json:"vmBootDisk"` // ex) /dev/sda1
	VMBlockDisk string     `json:"vmBlockDisk"`

	ConnectionName   string   `json:"connectionName"`
	SpecId           string   `json:"specId"`
	ImageId          string   `json:"imageId"`
	VNetId           string   `json:"vNetId"`
	SubnetId         string   `json:"subnetId"`
	SecurityGroupIds []string `json:"securityGroupIds"`
	SshKeyId         string   `json:"sshKeyId"`
	VmUserAccount    string   `json:"vmUserAccount,omitempty"`
	VmUserPassword   string   `json:"vmUserPassword,omitempty"`

	CspViewVmDetail SpiderVMInfo `json:"cspViewVmDetail,omitempty"`
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

// CorePostMcisVm is func to post (create) McisVm
func CorePostMcisVm(nsId string, mcisId string, vmInfoData *TbVmInfo) (*TbVmInfo, error) {

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

	go AddVmToMcis(&wg, nsId, mcisId, vmInfoData)

	wg.Wait()

	vmStatus, err := GetVmStatus(nsId, mcisId, vmInfoData.Id)
	if err != nil {
		//mapA := map[string]string{"message": "Cannot find " + common.GenMcisKey(nsId, mcisId, "")}
		//return c.JSON(http.StatusOK, &mapA)
		return nil, fmt.Errorf("Cannot find " + common.GenMcisKey(nsId, mcisId, vmInfoData.Id))
	}

	vmInfoData.Status = vmStatus.Status
	vmInfoData.TargetStatus = vmStatus.TargetStatus
	vmInfoData.TargetAction = vmStatus.TargetAction

	// Install CB-Dragonfly monitoring agent

	mcisTmp, _ := GetMcisObject(nsId, mcisId)

	fmt.Printf("\n[Init monitoring agent] for %+v\n - req.InstallMonAgent: %+v\n\n", mcisId, mcisTmp.InstallMonAgent)

	if mcisTmp.InstallMonAgent != "no" {

		// Sleep for 20 seconds for a safe DF agent installation.
		fmt.Printf("\n\n[Info] Sleep for 20 seconds for safe CB-Dragonfly Agent installation.\n\n")
		time.Sleep(20 * time.Second)

		check := CheckDragonflyEndpoint()
		if check != nil {
			fmt.Printf("\n\n[Warring] CB-Dragonfly is not available\n\n")
		} else {
			reqToMon := &McisCmdReq{}
			reqToMon.UserName = "cb-user" // this MCIS user name is temporal code. Need to improve.

			fmt.Printf("\n[InstallMonitorAgentToMcis]\n\n")
			content, err := InstallMonitorAgentToMcis(nsId, mcisId, mcisTmp.SystemLabel, reqToMon)
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

// CreateMcisGroupVm is func to create MCIS groupVM
func CreateMcisGroupVm(nsId string, mcisId string, vmRequest *TbVmReq) (*TbMcisInfo, error) {

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

	// VM Group handling
	vmGroupSize, _ := strconv.Atoi(vmRequest.VmGroupSize)
	fmt.Printf("vmGroupSize: %v\n", vmGroupSize)

	if vmGroupSize > 0 {

		fmt.Println("=========================== Create MCIS VM Group object")
		key := common.GenMcisVmGroupKey(nsId, mcisId, vmRequest.Name)

		// TODO: Enhancement Required. Need to check existing VM Group. Need to update it if exist.
		vmGroupInfoData := TbVmGroupInfo{}
		vmGroupInfoData.Id = vmRequest.Name
		vmGroupInfoData.Name = vmRequest.Name
		vmGroupInfoData.VmGroupSize = vmRequest.VmGroupSize

		for i := 0; i < vmGroupSize; i++ {
			vmGroupInfoData.VmId = append(vmGroupInfoData.VmId, vmGroupInfoData.Id+"-"+strconv.Itoa(i))
		}

		val, _ := json.Marshal(vmGroupInfoData)
		err := common.CBStore.Put(key, string(val))
		if err != nil {
			common.CBLog.Error(err)
		}
		keyValue, err := common.CBStore.Get(key)
		if err != nil {
			common.CBLog.Error(err)
			err = fmt.Errorf("In CreateMcisGroupVm(); CBStore.Get() returned an error.")
			common.CBLog.Error(err)
			// return nil, err
		}

		fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		fmt.Println("===========================")

	}

	for i := 0; i <= vmGroupSize; i++ {
		vmInfoData := TbVmInfo{}

		if vmGroupSize == 0 { // for VM (not in a group)
			vmInfoData.Name = vmRequest.Name
		} else { // for VM (in a group)
			if i == vmGroupSize {
				break // if vmGroupSize != 0 && vmGroupSize == i, skip the final loop
			}
			vmInfoData.VmGroupId = vmRequest.Name
			// TODO: Enhancement Required. Need to check existing VM Group. Need to update it if exist.
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
		vmInfoData.SpecId = vmRequest.SpecId
		vmInfoData.ImageId = vmRequest.ImageId
		vmInfoData.VNetId = vmRequest.VNetId
		vmInfoData.SubnetId = vmRequest.SubnetId
		//vmInfoData.VnicId = vmRequest.VnicId
		//vmInfoData.PublicIpId = vmRequest.PublicIpId
		vmInfoData.SecurityGroupIds = vmRequest.SecurityGroupIds
		vmInfoData.SshKeyId = vmRequest.SshKeyId
		vmInfoData.Description = vmRequest.Description

		vmInfoData.VmUserAccount = vmRequest.VmUserAccount
		vmInfoData.VmUserPassword = vmRequest.VmUserPassword

		wg.Add(1)
		go AddVmToMcis(&wg, nsId, mcisId, &vmInfoData)

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
	if mcisTmp.InstallMonAgent != "no" {

		// Sleep for 60 seconds for a safe DF agent installation.
		fmt.Printf("\n\n[Info] Sleep for 60 seconds for safe CB-Dragonfly Agent installation.\n\n")
		time.Sleep(60 * time.Second)

		check := CheckDragonflyEndpoint()
		if check != nil {
			fmt.Printf("\n\n[Warring] CB-Dragonfly is not available\n\n")
		} else {
			reqToMon := &McisCmdReq{}
			reqToMon.UserName = "cb-user" // this MCIS user name is temporal code. Need to improve.

			fmt.Printf("\n[InstallMonitorAgentToMcis]\n\n")
			content, err := InstallMonitorAgentToMcis(nsId, mcisId, mcisTmp.SystemLabel, reqToMon)
			if err != nil {
				common.CBLog.Error(err)
				//mcisTmp.InstallMonAgent = "no"
			}
			common.PrintJsonPretty(content)
			//mcisTmp.InstallMonAgent = "yes"
		}
	}
	return &mcisTmp, nil

}

// CreateMcis is func to create MCIS obeject and deploy requested VMs
func CreateMcis(nsId string, req *TbMcisReq) (*TbMcisInfo, error) {

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

	check, _ := CheckMcis(nsId, req.Name)
	if check {
		err := fmt.Errorf("The mcis " + req.Name + " already exists.")
		return nil, err
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
			temp := &TbMcisInfo{}
			common.CBLog.Error(err)
			return temp, err
		}
	}

	//goroutin
	var wg sync.WaitGroup

	for _, k := range vmRequest {

		// VM Group handling
		vmGroupSize, _ := strconv.Atoi(k.VmGroupSize)
		fmt.Printf("vmGroupSize: %v\n", vmGroupSize)

		if vmGroupSize > 0 {

			fmt.Println("=========================== Create MCIS VM Group object")
			key := common.GenMcisVmGroupKey(nsId, mcisId, k.Name)

			vmGroupInfoData := TbVmGroupInfo{}
			vmGroupInfoData.Id = common.ToLower(k.Name)
			vmGroupInfoData.Name = common.ToLower(k.Name)
			vmGroupInfoData.VmGroupSize = k.VmGroupSize

			for i := 0; i < vmGroupSize; i++ {
				vmGroupInfoData.VmId = append(vmGroupInfoData.VmId, vmGroupInfoData.Id+"-"+strconv.Itoa(i))
			}

			val, _ := json.Marshal(vmGroupInfoData)
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

		for i := 0; i <= vmGroupSize; i++ {
			vmInfoData := TbVmInfo{}

			if vmGroupSize == 0 { // for VM (not in a group)
				vmInfoData.Name = common.ToLower(k.Name)
			} else { // for VM (in a group)
				if i == vmGroupSize {
					break // if vmGroupSize != 0 && vmGroupSize == i, skip the final loop
				}
				vmInfoData.VmGroupId = common.ToLower(k.Name)
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
			vmInfoData.SpecId = k.SpecId
			vmInfoData.ImageId = k.ImageId
			vmInfoData.VNetId = k.VNetId
			vmInfoData.SubnetId = k.SubnetId
			vmInfoData.SecurityGroupIds = k.SecurityGroupIds
			vmInfoData.SshKeyId = k.SshKeyId
			vmInfoData.Description = k.Description
			vmInfoData.VmUserAccount = k.VmUserAccount
			vmInfoData.VmUserPassword = k.VmUserPassword

			vmInfoData.Label = k.Label

			// Avoid concurrent requests to CSP.
			time.Sleep(time.Duration(i) * time.Second)

			wg.Add(1)
			go AddVmToMcis(&wg, nsId, mcisId, &vmInfoData)
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
	//common.PrintJsonPretty(mcisTmp)

	// Install CB-Dragonfly monitoring agent

	fmt.Printf("[Init monitoring agent] for %+v\n - req.InstallMonAgent: %+v\n\n", mcisTmp.Id, req.InstallMonAgent)

	mcisTmp.InstallMonAgent = req.InstallMonAgent
	UpdateMcisInfo(nsId, mcisTmp)

	if req.InstallMonAgent != "no" {

		check := CheckDragonflyEndpoint()
		if check != nil {
			fmt.Printf("\n\n[Warring] CB-Dragonfly is not available\n\n")
		} else {
			reqToMon := &McisCmdReq{}
			reqToMon.UserName = "cb-user" // this MCIS user name is temporal code. Need to improve.

			fmt.Printf("\n===========================\n")
			// Sleep for 60 seconds for a safe DF agent installation.
			fmt.Printf("\n\n[Info] Sleep for 60 seconds for safe CB-Dragonfly Agent installation.\n")
			time.Sleep(60 * time.Second)

			fmt.Printf("\n[InstallMonitorAgentToMcis]\n\n")
			content, err := InstallMonitorAgentToMcis(nsId, mcisId, mcisTmp.SystemLabel, reqToMon)
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

// CreateMcisDynamic is func to create MCIS obeject and deploy requested VMs in a dynamic way
func CreateMcisDynamic(nsId string, req *TbMcisDynamicReq) (*TbMcisInfo, error) {

	commonNS := "common"
	onDemand := true

	mcisReq := TbMcisReq{}
	mcisReq.Name = req.Name
	mcisReq.Label = req.Label
	mcisReq.SystemLabel = req.SystemLabel
	mcisReq.InstallMonAgent = req.InstallMonAgent
	mcisReq.Description = req.Description

	vmRequest := req.Vm
	// Check whether VM names meet requirement.
	for _, k := range vmRequest {

		vmReq := TbVmReq{}
		tempInterface, err := mcir.GetResource(commonNS, common.StrSpec, k.CommonSpec)
		if err != nil {
			err := fmt.Errorf("Failed to get the spec " + k.CommonSpec)
			common.CBLog.Error(err)
			return &TbMcisInfo{}, err
		}
		specInfo := mcir.TbSpecInfo{}
		err = common.CopySrcToDest(&tempInterface, &specInfo)
		if err != nil {
			err := fmt.Errorf("Failed to CopySrcToDest() " + k.CommonSpec)
			common.CBLog.Error(err)
			return &TbMcisInfo{}, err
		}

		// remake vmReqest from given input and check resource availability
		vmReq.ConnectionName = specInfo.ConnectionName

		// Default resource name has this pattern (nsId + "-systemdefault-" + vmReq.ConnectionName)
		resourceName := nsId + common.StrDefaultResourceName + vmReq.ConnectionName

		vmReq.SpecId = specInfo.Id
		vmReq.ImageId = mcir.ToNamingRuleCompatible(vmReq.ConnectionName + "-" + k.CommonImage)
		tempInterface, err = mcir.GetResource(commonNS, common.StrImage, vmReq.ImageId)
		if err != nil {
			err := fmt.Errorf("Failed to get the Image " + vmReq.ImageId + " from " + vmReq.ConnectionName)
			common.CBLog.Error(err)
			return &TbMcisInfo{}, err
		}

		vmReq.VNetId = resourceName
		tempInterface, err = mcir.GetResource(nsId, common.StrVNet, vmReq.VNetId)
		if err != nil {
			err := fmt.Errorf("Failed to get the vNet " + vmReq.VNetId + " from " + vmReq.ConnectionName)
			common.CBLog.Info(err)
			if !onDemand {
				return &TbMcisInfo{}, err
			}
			err2 := mcir.LoadDefaultResource(nsId, common.StrVNet, vmReq.ConnectionName)
			if err2 != nil {
				common.CBLog.Error(err2)
				err2 = fmt.Errorf("[1]" + err.Error() + " [2]" + err2.Error())
				return &TbMcisInfo{}, err2
			}
		}
		vmReq.SubnetId = resourceName

		vmReq.SshKeyId = resourceName
		tempInterface, err = mcir.GetResource(nsId, common.StrSSHKey, vmReq.SshKeyId)
		if err != nil {
			err := fmt.Errorf("Failed to get the SshKey " + vmReq.SshKeyId + " from " + vmReq.ConnectionName)
			common.CBLog.Info(err)
			if !onDemand {
				return &TbMcisInfo{}, err
			}
			err2 := mcir.LoadDefaultResource(nsId, common.StrSSHKey, vmReq.ConnectionName)
			if err2 != nil {
				common.CBLog.Error(err2)
				err2 = fmt.Errorf("[1]" + err.Error() + " [2]" + err2.Error())
				return &TbMcisInfo{}, err2
			}
		}
		securityGroup := resourceName
		vmReq.SecurityGroupIds = append(vmReq.SecurityGroupIds, securityGroup)
		tempInterface, err = mcir.GetResource(nsId, common.StrSecurityGroup, securityGroup)
		if err != nil {
			err := fmt.Errorf("Failed to get the SecurityGroup " + securityGroup + " from " + vmReq.ConnectionName)
			common.CBLog.Info(err)
			if !onDemand {
				return &TbMcisInfo{}, err
			}
			err2 := mcir.LoadDefaultResource(nsId, common.StrSecurityGroup, vmReq.ConnectionName)
			if err2 != nil {
				common.CBLog.Error(err2)
				err2 = fmt.Errorf("[1]" + err.Error() + " [2]" + err2.Error())
				return &TbMcisInfo{}, err2
			}
		}

		vmReq.Name = k.Name
		if vmReq.Name == "" {
			vmReq.Name = common.GenUid()
		}
		vmReq.Label = k.Label
		vmReq.VmGroupSize = k.VmGroupSize
		vmReq.Description = k.Description

		mcisReq.Vm = append(mcisReq.Vm, vmReq)

	}

	common.PrintJsonPretty(mcisReq)

	// Run create MCIS with the generated MCIS request
	return CreateMcis(nsId, &mcisReq)
}

// AddVmToMcis is func to add VM to MCIS
func AddVmToMcis(wg *sync.WaitGroup, nsId string, mcisId string, vmInfoData *TbVmInfo) error {
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

	//fmt.Printf("\n[configTmp]\n %+v regionTmp %+v \n", configTmp, regionTmp)
	//fmt.Printf("\n[vmInfoData.Location]\n %+v\n", vmInfoData.Location)

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
	//common.PrintJsonPretty(vmInfoData)

	//instanceIds, publicIPs := CreateVm(&vmInfoData)
	err = CreateVm(nsId, mcisId, vmInfoData)

	fmt.Printf("\n[AddVmToMcis After request vmInfoData]\n")
	//common.PrintJsonPretty(vmInfoData)

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

	// set CreatedTime
	t := time.Now()
	vmInfoData.CreatedTime = t.Format("2006-01-02 15:04:05")
	fmt.Println(vmInfoData.CreatedTime)

	UpdateVmInfo(nsId, mcisId, *vmInfoData)

	return nil

}

// CreateVm is func to create VM
func CreateVm(nsId string, mcisId string, vmInfoData *TbVmInfo) error {

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

	var tempSpiderVMInfo SpiderVMInfo

	// Fill VM creation reqest (request to cb-spider)
	tempReq := SpiderVMReqInfoWrapper{}
	tempReq.ConnectionName = vmInfoData.ConnectionName

	//generate VM ID(Name) to request to CSP(Spider)
	//combination of nsId, mcidId, and vmName reqested from user
	tempReq.ReqInfo.Name = nsId + "-" + mcisId + "-" + vmInfoData.Name

	err := fmt.Errorf("")

	commonNs := "common"

	tempReq.ReqInfo.ImageName, err = common.GetCspResourceId(nsId, common.StrImage, vmInfoData.ImageId)
	if tempReq.ReqInfo.ImageName == "" || err != nil {
		common.CBLog.Error(err)
		// If cannot find the resource, use common resource
		tempReq.ReqInfo.ImageName, err = common.GetCspResourceId(commonNs, common.StrImage, vmInfoData.ImageId)
		if tempReq.ReqInfo.ImageName == "" || err != nil {
			common.CBLog.Error(err)
			return err
		}
	}

	tempReq.ReqInfo.VMSpecName, err = common.GetCspResourceId(nsId, common.StrSpec, vmInfoData.SpecId)
	if tempReq.ReqInfo.VMSpecName == "" || err != nil {
		common.CBLog.Error(err)
		// If cannot find the resource, use common resource
		tempReq.ReqInfo.VMSpecName, err = common.GetCspResourceId(commonNs, common.StrSpec, vmInfoData.SpecId)
		if tempReq.ReqInfo.ImageName == "" || err != nil {
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
		CspSgId := v //common.GetCspResourceId(nsId, common.StrSecurityGroup, v)
		if CspSgId == "" {
			common.CBLog.Error(err)
			return err
		}

		SecurityGroupIdsTmp = append(SecurityGroupIdsTmp, CspSgId)
	}
	tempReq.ReqInfo.SecurityGroupNames = SecurityGroupIdsTmp

	tempReq.ReqInfo.KeyPairName, err = common.GetCspResourceId(nsId, common.StrSSHKey, vmInfoData.SshKeyId)
	if tempReq.ReqInfo.KeyPairName == "" {
		common.CBLog.Error(err)
		return err
	}

	tempReq.ReqInfo.VMUserId = vmInfoData.VmUserAccount
	tempReq.ReqInfo.VMUserPasswd = vmInfoData.VmUserPassword

	fmt.Printf("\n[Request body to CB-SPIDER for Creating VM]\n")
	common.PrintJsonPretty(tempReq)

	payload, _ := json.Marshal(tempReq)

	// Randomly sleep within 30 Secs to avoid rateLimit from CSP
	common.RandomSleep(30)

	// Call cb-spider API by REST or gRPC
	if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

		url := common.SpiderRestUrl + "/vm"
		method := "POST"

		fmt.Println("\n[Calling SPIDER]START")
		fmt.Println("url: " + url + " method: " + method)

		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		req, err := http.NewRequest(method, url, strings.NewReader(string(payload)))

		if err != nil {
			//fmt.Println(err)
			common.CBLog.Error(err)
			return err
		}

		req.Header.Add("Content-Type", "application/json")

		res, err := client.Do(req)
		if err != nil {
			//common.PrintJsonPretty(err)
			common.CBLog.Error(err)
			return err
		}

		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)

		if err != nil {
			//common.PrintJsonPretty(err)
			common.CBLog.Error(err)
			return err
		}

		tempSpiderVMInfo = SpiderVMInfo{} // FYI; SpiderVMInfo: the struct in CB-Spider
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

		fmt.Println("\n[Calling SPIDER]START")

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
	fmt.Println("[Response from SPIDER]")
	common.PrintJsonPretty(tempSpiderVMInfo)
	fmt.Println("[Calling SPIDER]END")

	// Fill vmInfoData from the cb-spider response
	vmInfoData.CspViewVmDetail = tempSpiderVMInfo
	vmInfoData.VmUserAccount = tempSpiderVMInfo.VMUserId
	vmInfoData.VmUserPassword = tempSpiderVMInfo.VMUserPasswd

	//vmInfoData.Location = vmInfoData.Location

	//vmInfoData.VcpuSize = vmInfoData.VcpuSize
	//vmInfoData.MemorySize = vmInfoData.MemorySize
	//vmInfoData.DiskSize = vmInfoData.DiskSize
	//vmInfoData.Disk_type = vmInfoData.Disk_type

	//vmInfoData.PlacementAlgo = vmInfoData.PlacementAlgo

	// 2. Provided by CB-Spider
	//vmInfoData.CspVmId = temp.Id
	//vmInfoData.StartTime = temp.StartTime
	vmInfoData.Region = tempSpiderVMInfo.Region
	vmInfoData.PublicIP = tempSpiderVMInfo.PublicIP
	vmInfoData.SSHPort, _ = TrimIP(tempSpiderVMInfo.SSHAccessPoint)
	vmInfoData.PublicDNS = tempSpiderVMInfo.PublicDNS
	vmInfoData.PrivateIP = tempSpiderVMInfo.PrivateIP
	vmInfoData.PrivateDNS = tempSpiderVMInfo.PrivateDNS
	vmInfoData.VMBootDisk = tempSpiderVMInfo.VMBootDisk
	vmInfoData.VMBlockDisk = tempSpiderVMInfo.VMBlockDisk
	//vmInfoData.KeyValueList = temp.KeyValueList

	//configTmp, _ := common.GetConnConfig(vmInfoData.ConnectionName)
	//vmInfoData.Location = GetCloudLocation(strings.ToLower(configTmp.ProviderName), strings.ToLower(tempSpiderVMInfo.Region.Region))

	vmKey := common.GenMcisKey(nsId, mcisId, vmInfoData.Id)
	//mcir.UpdateAssociatedObjectList(nsId, common.StrSSHKey, vmInfoData.SshKeyId, common.StrAdd, vmKey)
	mcir.UpdateAssociatedObjectList(nsId, common.StrImage, vmInfoData.ImageId, common.StrAdd, vmKey)
	mcir.UpdateAssociatedObjectList(nsId, common.StrSpec, vmInfoData.SpecId, common.StrAdd, vmKey)
	mcir.UpdateAssociatedObjectList(nsId, common.StrSSHKey, vmInfoData.SshKeyId, common.StrAdd, vmKey)
	mcir.UpdateAssociatedObjectList(nsId, common.StrVNet, vmInfoData.VNetId, common.StrAdd, vmKey)

	for _, v2 := range vmInfoData.SecurityGroupIds {
		mcir.UpdateAssociatedObjectList(nsId, common.StrSecurityGroup, v2, common.StrAdd, vmKey)
	}

	UpdateVmInfo(nsId, mcisId, *vmInfoData)

	return nil
}

// [Etc used in provisioning]
