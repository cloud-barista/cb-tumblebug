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
	"reflect"

	//"log"
	"strconv"
	"strings"
	"time"

	//csv file handling

	"os"

	"math/rand"
	"sort"

	// REST API (echo)
	"net/http"

	"sync"

	"github.com/cloud-barista/cb-spider/interface/api"
	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
)

// [MCIS and VM object information managemenet]

// ListMcisId is func to list MCIS ID
func ListMcisId(nsId string) ([]string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	var mcisList []string

	// Check MCIS exists
	key := common.GenMcisKey(nsId, "", "")
	key += "/"

	keyValue, err := common.CBStore.GetList(key, true)

	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	for _, v := range keyValue {
		if strings.Contains(v.Key, "/mcis/") {
			trimmedString := strings.TrimPrefix(v.Key, (key + "mcis/"))
			// prevent malformed key (if key for mcis id includes '/', the key does not represent MCIS ID)
			if !strings.Contains(trimmedString, "/") {
				mcisList = append(mcisList, trimmedString)
			}
		}
	}

	return mcisList, nil
}

// ListVmId is func to list VM IDs
func ListVmId(nsId string, mcisId string) ([]string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	var vmList []string

	// Check MCIS exists
	key := common.GenMcisKey(nsId, mcisId, "")
	key += "/"

	_, err = common.CBStore.Get(key)
	if err != nil {
		fmt.Println("[Not found] " + mcisId)
		common.CBLog.Error(err)
		return vmList, err
	}

	keyValue, err := common.CBStore.GetList(key, true)

	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	for _, v := range keyValue {
		if strings.Contains(v.Key, "/vm/") {
			trimmedString := strings.TrimPrefix(v.Key, (key + "vm/"))
			// prevent malformed key (if key for vm id includes '/', the key does not represent VM ID)
			if !strings.Contains(trimmedString, "/") {
				vmList = append(vmList, trimmedString)
			}
		}
	}

	return vmList, nil

}

// GetVmListByLabel is func to list VM by label
func GetVmListByLabel(nsId string, mcisId string, label string) ([]string, error) {

	fmt.Println("[GetVmListByLabel]" + mcisId + " by " + label)

	var vmListByLabel []string

	vmList, err := ListVmId(nsId, mcisId)
	fmt.Println(vmList)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}
	if len(vmList) == 0 {
		return nil, nil
	}

	// delete vms info
	for _, v := range vmList {
		vmObj, vmErr := GetVmObject(nsId, mcisId, v)
		if vmErr != nil {
			common.CBLog.Error(err)
			return nil, vmErr
		}

		if vmObj.Label == label {
			fmt.Println("Found VM with " + vmObj.Label + ", VM ID: " + vmObj.Id)
			vmListByLabel = append(vmListByLabel, vmObj.Id)
		}
	}
	return vmListByLabel, nil

}

// ListVmGroupId is func to return list of VmGroups in a given MCIS
func ListVmGroupId(nsId string, mcisId string) ([]string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	fmt.Println("[ListVmGroupId]")
	key := common.GenMcisKey(nsId, mcisId, "")
	key += "/"

	keyValue, err := common.CBStore.GetList(key, true)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}
	var vmGroupList []string
	for _, v := range keyValue {
		if strings.Contains(v.Key, "/vmgroup/") {
			trimmedString := strings.TrimPrefix(v.Key, (key + "vmgroup/"))
			// prevent malformed key (if key for vm id includes '/', the key does not represent VM ID)
			if !strings.Contains(trimmedString, "/") {
				vmGroupList = append(vmGroupList, trimmedString)
			}
		}
	}
	return vmGroupList, nil
}

// GetMcisInfo is func to return MCIS information with the current status update
func GetMcisInfo(nsId string, mcisId string) (*TbMcisInfo, error) {

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
	check, _ := CheckMcis(nsId, mcisId)

	if !check {
		temp := &TbMcisInfo{}
		err := fmt.Errorf("The mcis " + mcisId + " does not exist.")
		return temp, err
	}

	mcisObj, err := GetMcisObject(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	// common.PrintJsonPretty(mcisObj)

	mcisStatus, err := GetMcisStatus(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}
	// common.PrintJsonPretty(mcisStatus)

	mcisObj.Status = mcisStatus.Status
	mcisObj.StatusCount = mcisStatus.StatusCount

	vmList, err := ListVmId(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	sort.Slice(mcisObj.Vm, func(i, j int) bool {
		return mcisObj.Vm[i].Id < mcisObj.Vm[j].Id
	})

	for vmInfoIndex := range vmList {
		for vmStatusInfoIndex := range mcisStatus.Vm {
			if mcisObj.Vm[vmInfoIndex].Id == mcisStatus.Vm[vmStatusInfoIndex].Id {
				mcisObj.Vm[vmInfoIndex].Status = mcisStatus.Vm[vmStatusInfoIndex].Status
				mcisObj.Vm[vmInfoIndex].TargetStatus = mcisStatus.Vm[vmStatusInfoIndex].TargetStatus
				mcisObj.Vm[vmInfoIndex].TargetAction = mcisStatus.Vm[vmStatusInfoIndex].TargetAction
				break
			}
		}
	}

	return &mcisObj, nil
}

// CoreGetAllMcis is func to get all MCIS objects
func CoreGetAllMcis(nsId string, option string) ([]TbMcisInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	/*
		var content struct {
			//Name string     `json:"name"`
			Mcis []mcis.TbMcisInfo `json:"mcis"`
		}
	*/
	// content := RestGetAllMcisResponse{}

	Mcis := []TbMcisInfo{}

	mcisList, err := ListMcisId(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	for _, v := range mcisList {

		key := common.GenMcisKey(nsId, v, "")
		keyValue, err := common.CBStore.Get(key)
		if err != nil {
			common.CBLog.Error(err)
			err = fmt.Errorf("In CoreGetAllMcis(); CBStore.Get() returned an error.")
			common.CBLog.Error(err)
			// return nil, err
		}

		if keyValue == nil {
			return nil, fmt.Errorf("in CoreGetAllMcis() mcis loop; Cannot find " + key)
		}
		mcisTmp := TbMcisInfo{}
		json.Unmarshal([]byte(keyValue.Value), &mcisTmp)
		mcisId := v
		mcisTmp.Id = mcisId

		if option == "status" || option == "simple" {
			//get current mcis status
			mcisStatus, err := GetMcisStatus(nsId, mcisId)
			if err != nil {
				common.CBLog.Error(err)
				return nil, err
			}
			mcisTmp.Status = mcisStatus.Status
		} else {
			//Set current mcis status with NullStr
			mcisTmp.Status = ""
		}

		// The cases with id, status, or others. except simple

		vmList, err := ListVmId(nsId, mcisId)
		if err != nil {
			common.CBLog.Error(err)
			return nil, err
		}

		for _, v1 := range vmList {
			vmKey := common.GenMcisKey(nsId, mcisId, v1)
			vmKeyValue, err := common.CBStore.Get(vmKey)
			if err != nil {
				err = fmt.Errorf("In CoreGetAllMcis(); CBStore.Get() returned an error")
				common.CBLog.Error(err)
				// return nil, err
			}

			if vmKeyValue == nil {
				return nil, fmt.Errorf("in CoreGetAllMcis() vm loop; Cannot find " + vmKey)
			}
			vmTmp := TbVmInfo{}
			json.Unmarshal([]byte(vmKeyValue.Value), &vmTmp)
			vmTmp.Id = v1

			if option == "status" {
				//get current vm status
				vmStatusInfoTmp, err := GetVmStatus(nsId, mcisId, v1)
				if err != nil {
					common.CBLog.Error(err)
				}
				vmTmp.Status = vmStatusInfoTmp.Status
			} else if option == "simple" {
				vmSimpleTmp := TbVmInfo{}
				vmSimpleTmp.Id = vmTmp.Id
				vmSimpleTmp.Location = vmTmp.Location
				vmTmp = vmSimpleTmp
			} else {
				//Set current vm status with NullStr
				vmTmp.Status = ""
			}

			mcisTmp.Vm = append(mcisTmp.Vm, vmTmp)
		}

		Mcis = append(Mcis, mcisTmp)
	}

	return Mcis, nil
}

// CoreGetMcisVmInfo is func to Get McisVm Info
func CoreGetMcisVmInfo(nsId string, mcisId string, vmId string) (*TbVmInfo, error) {

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

	err = common.CheckString(vmId)
	if err != nil {
		temp := &TbVmInfo{}
		common.CBLog.Error(err)
		return temp, err
	}
	check, _ := CheckVm(nsId, mcisId, vmId)

	if !check {
		temp := &TbVmInfo{}
		err := fmt.Errorf("The vm " + vmId + " does not exist.")
		return temp, err
	}

	fmt.Println("[Get MCIS-VM info for id]" + vmId)

	key := common.GenMcisKey(nsId, mcisId, "")

	vmKey := common.GenMcisKey(nsId, mcisId, vmId)
	vmKeyValue, err := common.CBStore.Get(vmKey)
	if err != nil {
		common.CBLog.Error(err)
		err = fmt.Errorf("In CoreGetMcisVmInfo(); CBStore.Get() returned an error.")
		common.CBLog.Error(err)
		// return nil, err
	}

	if vmKeyValue == nil {
		return nil, fmt.Errorf("Cannot find " + key)
	}
	vmTmp := TbVmInfo{}
	json.Unmarshal([]byte(vmKeyValue.Value), &vmTmp)
	vmTmp.Id = vmId

	//get current vm status
	vmStatusInfoTmp, err := GetVmStatus(nsId, mcisId, vmId)
	if err != nil {
		common.CBLog.Error(err)
	}

	vmTmp.Status = vmStatusInfoTmp.Status
	vmTmp.TargetStatus = vmStatusInfoTmp.TargetStatus
	vmTmp.TargetAction = vmStatusInfoTmp.TargetAction

	return &vmTmp, nil
}

// GetMcisObject is func to retrieve MCIS object from database (no current status update)
func GetMcisObject(nsId string, mcisId string) (TbMcisInfo, error) {
	fmt.Println("[GetMcisObject]" + mcisId)
	key := common.GenMcisKey(nsId, mcisId, "")
	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
		return TbMcisInfo{}, err
	}
	mcisTmp := TbMcisInfo{}
	json.Unmarshal([]byte(keyValue.Value), &mcisTmp)

	vmList, err := ListVmId(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return TbMcisInfo{}, err
	}

	for _, vmID := range vmList {
		vmtmp, err := GetVmObject(nsId, mcisId, vmID)
		if err != nil {
			common.CBLog.Error(err)
			return TbMcisInfo{}, err
		}
		mcisTmp.Vm = append(mcisTmp.Vm, vmtmp)
	}

	return mcisTmp, nil
}

// GetVmObject is func to get VM object
func GetVmObject(nsId string, mcisId string, vmId string) (TbVmInfo, error) {
	key := common.GenMcisKey(nsId, mcisId, vmId)
	keyValue, err := common.CBStore.Get(key)
	if keyValue == nil || err != nil {
		common.CBLog.Error(err)
		return TbVmInfo{}, err
	}
	vmTmp := TbVmInfo{}
	json.Unmarshal([]byte(keyValue.Value), &vmTmp)
	return vmTmp, nil
}

// [MCIS and VM status management]

// GetMcisStatus is func to Get Mcis Status
func GetMcisStatus(nsId string, mcisId string) (*McisStatusInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return &McisStatusInfo{}, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return &McisStatusInfo{}, err
	}

	fmt.Println("[GetMcisStatus]" + mcisId)

	key := common.GenMcisKey(nsId, mcisId, "")

	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
		return &McisStatusInfo{}, err
	}
	if keyValue == nil {
		err := fmt.Errorf("Not found [" + key + "]")
		common.CBLog.Error(err)
		return &McisStatusInfo{}, err
	}

	mcisStatus := McisStatusInfo{}
	json.Unmarshal([]byte(keyValue.Value), &mcisStatus)

	mcisTmp := TbMcisInfo{}
	json.Unmarshal([]byte(keyValue.Value), &mcisTmp)

	vmList, err := ListVmId(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return &McisStatusInfo{}, err
	}
	if len(vmList) == 0 {
		return &McisStatusInfo{}, nil
	}

	//goroutin sync wg
	var wg sync.WaitGroup
	for _, v := range vmList {
		wg.Add(1)
		go GetVmStatusAsync(&wg, nsId, mcisId, v, &mcisStatus)
	}
	wg.Wait() //goroutine sync wg

	for _, v := range vmList {
		// set master IP of MCIS (Default rule: select 1st Running VM as master)
		vmtmp, _ := GetVmObject(nsId, mcisId, v)
		if vmtmp.Status == StatusRunning {
			mcisStatus.MasterVmId = vmtmp.Id
			mcisStatus.MasterIp = vmtmp.PublicIP
			mcisStatus.MasterSSHPort = vmtmp.SSHPort
			break
		}
	}

	sort.Slice(mcisStatus.Vm, func(i, j int) bool {
		return mcisStatus.Vm[i].Id < mcisStatus.Vm[j].Id
	})

	statusFlag := []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	statusFlagStr := []string{StatusFailed, StatusSuspended, StatusRunning, StatusTerminated, StatusCreating, StatusSuspending, StatusResuming, StatusRebooting, StatusTerminating, StatusUndefined}
	for _, v := range mcisStatus.Vm {

		switch v.Status {
		case StatusFailed:
			statusFlag[0]++
		case StatusSuspended:
			statusFlag[1]++
		case StatusRunning:
			statusFlag[2]++
		case StatusTerminated:
			statusFlag[3]++
		case StatusCreating:
			statusFlag[4]++
		case StatusSuspending:
			statusFlag[5]++
		case StatusResuming:
			statusFlag[6]++
		case StatusRebooting:
			statusFlag[7]++
		case StatusTerminating:
			statusFlag[8]++
		default:
			statusFlag[9]++
		}
	}

	tmpMax := 0
	tmpMaxIndex := 0
	for i, v := range statusFlag {
		if v > tmpMax {
			tmpMax = v
			tmpMaxIndex = i
		}
	}

	numVm := len(mcisStatus.Vm)
	//numUnNormalStatus := statusFlag[0] + statusFlag[9]
	//numNormalStatus := numVm - numUnNormalStatus
	runningStatus := statusFlag[2]

	proportionStr := ":" + strconv.Itoa(tmpMax) + " (R:" + strconv.Itoa(runningStatus) + "/" + strconv.Itoa(numVm) + ")"
	if tmpMax == numVm {
		mcisStatus.Status = statusFlagStr[tmpMaxIndex] + proportionStr
	} else if tmpMax < numVm {
		mcisStatus.Status = "Partial-" + statusFlagStr[tmpMaxIndex] + proportionStr
	} else {
		mcisStatus.Status = statusFlagStr[9] + proportionStr
	}
	// for representing Failed status in front.

	proportionStr = ":" + strconv.Itoa(statusFlag[0]) + " (R:" + strconv.Itoa(runningStatus) + "/" + strconv.Itoa(numVm) + ")"
	if statusFlag[0] > 0 {
		mcisStatus.Status = "Partial-" + statusFlagStr[0] + proportionStr
		if statusFlag[0] == numVm {
			mcisStatus.Status = statusFlagStr[0] + proportionStr
		}
	}

	// proportionStr = "-(" + strconv.Itoa(statusFlag[9]) + "/" + strconv.Itoa(numVm) + ")"
	// if statusFlag[9] > 0 {
	// 	mcisStatus.Status = statusFlagStr[9] + proportionStr
	// }

	// Set mcisStatus.StatusCount
	mcisStatus.StatusCount.CountTotal = numVm
	mcisStatus.StatusCount.CountFailed = statusFlag[0]
	mcisStatus.StatusCount.CountSuspended = statusFlag[1]
	mcisStatus.StatusCount.CountRunning = statusFlag[2]
	mcisStatus.StatusCount.CountTerminated = statusFlag[3]
	mcisStatus.StatusCount.CountCreating = statusFlag[4]
	mcisStatus.StatusCount.CountSuspending = statusFlag[5]
	mcisStatus.StatusCount.CountResuming = statusFlag[6]
	mcisStatus.StatusCount.CountRebooting = statusFlag[7]
	mcisStatus.StatusCount.CountTerminating = statusFlag[8]
	mcisStatus.StatusCount.CountUndefined = statusFlag[9]

	var isDone bool
	isDone = true
	for _, v := range mcisStatus.Vm {
		if v.TargetStatus != StatusComplete {
			isDone = false
		}
	}
	if isDone {
		mcisStatus.TargetAction = ActionComplete
		mcisStatus.TargetStatus = StatusComplete
		mcisTmp.TargetAction = ActionComplete
		mcisTmp.TargetStatus = StatusComplete
		mcisTmp.StatusCount = mcisStatus.StatusCount
		UpdateMcisInfo(nsId, mcisTmp)
	}

	return &mcisStatus, nil

	//need to change status

}

// GetMcisStatusAll is func to get MCIS status all
func GetMcisStatusAll(nsId string) ([]McisStatusInfo, error) {

	mcisStatuslist := []McisStatusInfo{}
	mcisList, err := ListMcisId(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return mcisStatuslist, err
	}

	for _, mcisId := range mcisList {
		mcisStatus, err := GetMcisStatus(nsId, mcisId)
		if err != nil {
			common.CBLog.Error(err)
			return mcisStatuslist, err
		}
		mcisStatuslist = append(mcisStatuslist, *mcisStatus)
	}
	return mcisStatuslist, nil

	//need to change status

}

// McisStatusInfo is struct to define simple information of MCIS with updated status of all VMs
type McisStatusInfo struct {
	Id   string `json:"id"`
	Name string `json:"name"`

	Status       string          `json:"status"`
	StatusCount  StatusCountInfo `json:"statusCount"`
	TargetStatus string          `json:"targetStatus"`
	TargetAction string          `json:"targetAction"`

	// InstallMonAgent Option for CB-Dragonfly agent installation ([yes/no] default:yes)
	InstallMonAgent string `json:"installMonAgent" example:"[yes, no]"` // yes or no

	MasterVmId    string `json:"masterVmId" example:"vm-asiaeast1-cb-01"`
	MasterIp      string `json:"masterIp" example:"32.201.134.113"`
	MasterSSHPort string `json:"masterSSHPort"`

	// Label is for describing the mcis in a keyword (any string can be used)
	Label string `json:"label" example:"User custom label"`

	// SystemLabel is for describing the mcis in a keyword (any string can be used) for special System purpose
	SystemLabel string `json:"systemLabel" example:"Managed by CB-Tumblebug" default:""`

	Vm []TbVmStatusInfo `json:"vm"`
}

// TbVmStatusInfo is to define simple information of VM with updated status
type TbVmStatusInfo struct {
	Id      string `json:"id"`
	Name    string `json:"name"`
	CspVmId string `json:"cspVmId"`

	Status       string `json:"status"`
	TargetStatus string `json:"targetStatus"`
	TargetAction string `json:"targetAction"`
	NativeStatus string `json:"nativeStatus"`

	// Montoring agent status
	MonAgentStatus string `json:"monAgentStatus" example:"[installed, notInstalled, failed]"` // yes or no// installed, notInstalled, failed

	// Latest system message such as error message
	SystemMessage string `json:"systemMessage" example:"Failed because ..." default:""` // systeam-given string message

	// Created time
	CreatedTime string `json:"createdTime" example:"2022-11-10 23:00:00" default:""`

	PublicIp  string `json:"publicIp"`
	PrivateIp string `json:"privateIp"`
	SSHPort   string `json:"sshPort"`

	Location common.GeoLocation `json:"location"`
}

// GetVmCurrentPublicIp is func to get VM public IP
func GetVmCurrentPublicIp(nsId string, mcisId string, vmId string) (TbVmStatusInfo, error) {

	fmt.Println("[GetVmStatus]" + vmId)
	key := common.GenMcisKey(nsId, mcisId, vmId)
	errorInfo := TbVmStatusInfo{}

	keyValue, err := common.CBStore.Get(key)
	if err != nil || keyValue == nil {
		fmt.Println(err)
		return errorInfo, err
	}

	temp := TbVmInfo{}
	unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &temp)
	if unmarshalErr != nil {
		fmt.Println("unmarshalErr:", unmarshalErr)
	}
	fmt.Println("\n[Calling SPIDER] START")
	fmt.Println("CspVmId: " + temp.CspViewVmDetail.IId.NameId)

	cspVmId := temp.CspViewVmDetail.IId.NameId

	type statusResponse struct {
		Status         string
		PublicIP       string
		SSHAccessPoint string
	}
	var statusResponseTmp statusResponse

	if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

		url := common.SpiderRestUrl + "/vm/" + cspVmId
		method := "GET"

		type VMStatusReqInfo struct {
			ConnectionName string
		}
		tempReq := VMStatusReqInfo{}
		tempReq.ConnectionName = temp.ConnectionName
		payload, _ := json.MarshalIndent(tempReq, "", "  ")

		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		req, err := http.NewRequest(method, url, strings.NewReader(string(payload)))

		errorInfo.Status = StatusFailed

		if err != nil {
			fmt.Println(err)
			return errorInfo, err
		}
		req.Header.Add("Content-Type", "application/json")

		res, err := client.Do(req)

		if err != nil {
			fmt.Println(err)
			return errorInfo, err
		}

		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)

		statusResponseTmp = statusResponse{}

		err2 := json.Unmarshal(body, &statusResponseTmp)
		if err2 != nil {
			fmt.Println(err2)
			return errorInfo, err2
		}

	} else {

		// Set CCM gRPC API
		ccm := api.NewCloudResourceHandler()
		err := ccm.SetConfigPath(os.Getenv("CBTUMBLEBUG_ROOT") + "/conf/grpc_conf.yaml")
		if err != nil {
			common.CBLog.Error("ccm failed to set config : ", err)
			return errorInfo, err
		}
		err = ccm.Open()
		if err != nil {
			common.CBLog.Error("ccm api open failed : ", err)
			return errorInfo, err
		}
		defer ccm.Close()

		result, err := ccm.GetVMByParam(temp.ConnectionName, cspVmId)
		if err != nil {
			common.CBLog.Error(err)
			return errorInfo, err
		}

		statusResponseTmp = statusResponse{}
		err2 := json.Unmarshal([]byte(result), &statusResponseTmp)
		if err2 != nil {
			common.CBLog.Error(err2)
			return errorInfo, err2
		}

	}

	fmt.Println(statusResponseTmp)

	vmStatusTmp := TbVmStatusInfo{}
	vmStatusTmp.PublicIp = statusResponseTmp.PublicIP
	vmStatusTmp.SSHPort, _ = TrimIP(statusResponseTmp.SSHAccessPoint)

	return vmStatusTmp, nil

}

// GetVmIp is func to get VM IP
func GetVmIp(nsId string, mcisId string, vmId string) (string, string) {

	var content struct {
		PublicIP string `json:"publicIP"`
		SSHPort  string `json:"sshPort"`
	}

	fmt.Printf("[GetVmIp] " + vmId)
	key := common.GenMcisKey(nsId, mcisId, vmId)

	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
		err = fmt.Errorf("In GetVmIp(); CBStore.Get() returned an error.")
		common.CBLog.Error(err)
		// return nil, err
	}

	json.Unmarshal([]byte(keyValue.Value), &content)

	fmt.Printf(" %+v\n", content.PublicIP)

	return content.PublicIP, content.SSHPort
}

// GetVmSpecId is func to get VM SpecId
func GetVmSpecId(nsId string, mcisId string, vmId string) string {

	var content struct {
		SpecId string `json:"specId"`
	}

	fmt.Println("[getVmSpecID]" + vmId)
	key := common.GenMcisKey(nsId, mcisId, vmId)

	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
		err = fmt.Errorf("In GetVmSpecId(); CBStore.Get() returned an error.")
		common.CBLog.Error(err)
		// return nil, err
	}

	json.Unmarshal([]byte(keyValue.Value), &content)

	fmt.Printf("%+v\n", content.SpecId)

	return content.SpecId
}

// GetVmStatusAsync is func to get VM status async
func GetVmStatusAsync(wg *sync.WaitGroup, nsId string, mcisId string, vmId string, results *McisStatusInfo) error {
	defer wg.Done() //goroutine sync done

	vmStatusTmp, err := GetVmStatus(nsId, mcisId, vmId)
	if err != nil {
		common.CBLog.Error(err)
		vmStatusTmp.Status = StatusFailed
		vmStatusTmp.SystemMessage = err.Error()
	}

	results.Vm = append(results.Vm, vmStatusTmp)
	return nil
}

// GetVmStatus is func to get VM status
func GetVmStatus(nsId string, mcisId string, vmId string) (TbVmStatusInfo, error) {

	// defer func() {
	// 	if runtimeErr := recover(); runtimeErr != nil {
	// 		myErr := fmt.Errorf("in GetVmStatus; mcisId: " + mcisId + ", vmId: " + vmId)
	// 		common.CBLog.Error(myErr)
	// 		common.CBLog.Error(runtimeErr)
	// 	}
	// }()

	key := common.GenMcisKey(nsId, mcisId, vmId)

	errorInfo := TbVmStatusInfo{}

	keyValue, err := common.CBStore.Get(key)
	if keyValue == nil || err != nil {
		fmt.Println("CBStoreGetErr. keyValue == nil || err != nil", err)
		fmt.Println(err)
		return errorInfo, err
	}

	temp := TbVmInfo{}
	unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &temp)
	if unmarshalErr != nil {
		fmt.Println("unmarshalErr:", unmarshalErr)
		fmt.Println(err)
		return errorInfo, err
	}

	errorInfo.Id = temp.Id
	errorInfo.Name = temp.Name
	errorInfo.CspVmId = temp.CspViewVmDetail.IId.NameId
	errorInfo.PublicIp = temp.PublicIP
	errorInfo.SSHPort = temp.SSHPort
	errorInfo.PrivateIp = temp.PrivateIP
	errorInfo.NativeStatus = StatusUndefined
	errorInfo.TargetAction = temp.TargetAction
	errorInfo.TargetStatus = temp.TargetStatus
	errorInfo.Location = temp.Location
	errorInfo.MonAgentStatus = temp.MonAgentStatus
	errorInfo.CreatedTime = temp.CreatedTime
	errorInfo.SystemMessage = "Error in GetVmStatus"

	cspVmId := temp.CspViewVmDetail.IId.NameId

	type statusResponse struct {
		Status string
	}
	statusResponseTmp := statusResponse{}
	statusResponseTmp.Status = ""

	if cspVmId != "" && temp.Status != StatusTerminated {
		fmt.Print("[Calling SPIDER] vmstatus, ")
		fmt.Println("CspVmId: " + cspVmId)
		if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

			url := common.SpiderRestUrl + "/vmstatus/" + cspVmId
			method := "GET"

			type VMStatusReqInfo struct {
				ConnectionName string
			}
			tempReq := VMStatusReqInfo{}
			tempReq.ConnectionName = temp.ConnectionName
			payload, _ := json.MarshalIndent(tempReq, "", "  ")

			client := &http.Client{
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
			}

			// Retry to get right VM status from cb-spider. Sometimes cb-spider returns not approriate status.
			retrycheck := 2
			for i := 0; i < retrycheck; i++ {

				req, err := http.NewRequest(method, url, strings.NewReader(string(payload)))
				errorInfo.Status = StatusFailed
				if err != nil {
					fmt.Println(err)
					return errorInfo, err
				}
				req.Header.Add("Content-Type", "application/json")

				res, err := client.Do(req)
				if err != nil {
					fmt.Println(err)
					errorInfo.SystemMessage = err.Error()
					//return errorInfo, err
				} else {
					body, err := ioutil.ReadAll(res.Body)
					if err != nil {
						fmt.Println(err)
						errorInfo.SystemMessage = err.Error()
						return errorInfo, err
					}
					err = json.Unmarshal(body, &statusResponseTmp)
					if err != nil {
						fmt.Println(err)
						errorInfo.SystemMessage = err.Error()
						return errorInfo, err
					}
					defer res.Body.Close()
				}

				if statusResponseTmp.Status != "" {
					break
				}
				time.Sleep(1 * time.Second)
			}

		} else {

			// Set CCM gRPC API
			ccm := api.NewCloudResourceHandler()
			err := ccm.SetConfigPath(os.Getenv("CBTUMBLEBUG_ROOT") + "/conf/grpc_conf.yaml")
			if err != nil {
				common.CBLog.Error("ccm failed to set config : ", err)
				return errorInfo, err
			}
			err = ccm.Open()
			if err != nil {
				common.CBLog.Error("ccm api open failed : ", err)
				return errorInfo, err
			}
			defer ccm.Close()

			// Retry to get right VM status from cb-spider. Sometimes cb-spider returns not approriate status.
			retrycheck := 2
			for i := 0; i < retrycheck; i++ {
				result, err := ccm.GetVMStatusByParam(temp.ConnectionName, cspVmId)
				if err != nil {
					common.CBLog.Error(err)
					errorInfo.SystemMessage = err.Error()
					//return errorInfo, err
				} else {
					err = json.Unmarshal([]byte(result), &statusResponseTmp)
					if err != nil {
						common.CBLog.Error(err)
						errorInfo.SystemMessage = err.Error()
						return errorInfo, err
					}
				}

				if statusResponseTmp.Status != "" {
					break
				}
				time.Sleep(1 * time.Second)
			}
		}

	} else {
		statusResponseTmp.Status = ""
	}

	nativeStatus := statusResponseTmp.Status
	// Temporal CODE. This should be changed after CB-Spider fixes status types and strings/
	if nativeStatus == "Creating" {
		statusResponseTmp.Status = StatusCreating
	} else if nativeStatus == "Running" {
		statusResponseTmp.Status = StatusRunning
	} else if nativeStatus == "Suspending" {
		statusResponseTmp.Status = StatusSuspending
	} else if nativeStatus == "Suspended" {
		statusResponseTmp.Status = StatusSuspended
	} else if nativeStatus == "Resuming" {
		statusResponseTmp.Status = StatusResuming
	} else if nativeStatus == "Rebooting" {
		statusResponseTmp.Status = StatusRebooting
	} else if nativeStatus == "Terminating" {
		statusResponseTmp.Status = StatusTerminating
	} else if nativeStatus == "Terminated" {
		statusResponseTmp.Status = StatusTerminated
	} else {
		statusResponseTmp.Status = StatusUndefined
	}
	// End of Temporal CODE.
	temp, err = GetVmObject(nsId, mcisId, vmId)
	if keyValue == nil || err != nil {
		fmt.Println("CBStoreGetErr. keyValue == nil || err != nil", err)
		fmt.Println(err)
		return errorInfo, err
	}
	vmStatusTmp := TbVmStatusInfo{}
	vmStatusTmp.Id = temp.Id
	vmStatusTmp.Name = temp.Name
	vmStatusTmp.CspVmId = temp.CspViewVmDetail.IId.NameId

	vmStatusTmp.PrivateIp = temp.PrivateIP
	vmStatusTmp.NativeStatus = nativeStatus
	vmStatusTmp.TargetAction = temp.TargetAction
	vmStatusTmp.TargetStatus = temp.TargetStatus
	vmStatusTmp.Location = temp.Location
	vmStatusTmp.MonAgentStatus = temp.MonAgentStatus
	vmStatusTmp.CreatedTime = temp.CreatedTime
	vmStatusTmp.SystemMessage = temp.SystemMessage

	//Correct undefined status using TargetAction
	if vmStatusTmp.TargetAction == ActionCreate {
		if statusResponseTmp.Status == StatusUndefined {
			statusResponseTmp.Status = StatusCreating
		}
		if temp.Status == StatusFailed {
			statusResponseTmp.Status = StatusFailed
		}
	}
	if vmStatusTmp.TargetAction == ActionTerminate {
		if statusResponseTmp.Status == StatusUndefined {
			statusResponseTmp.Status = StatusTerminated
		}
		if statusResponseTmp.Status == StatusSuspending {
			statusResponseTmp.Status = StatusTerminated
		}
	}
	if vmStatusTmp.TargetAction == ActionResume {
		if statusResponseTmp.Status == StatusUndefined {
			statusResponseTmp.Status = StatusResuming
		}
		if statusResponseTmp.Status == StatusCreating {
			statusResponseTmp.Status = StatusResuming
		}
	}
	// for action reboot, some csp's native status are suspending, suspended, creating, resuming
	if vmStatusTmp.TargetAction == ActionReboot {
		if statusResponseTmp.Status == StatusUndefined {
			statusResponseTmp.Status = StatusRebooting
		}
		if statusResponseTmp.Status == StatusSuspending || statusResponseTmp.Status == StatusSuspended || statusResponseTmp.Status == StatusCreating || statusResponseTmp.Status == StatusResuming {
			statusResponseTmp.Status = StatusRebooting
		}
	}

	if vmStatusTmp.Status == StatusTerminated {
		statusResponseTmp.Status = StatusTerminated
	}

	vmStatusTmp.Status = statusResponseTmp.Status

	// TODO: Alibaba Undefined status error is not resolved yet.
	// (After Terminate action. "status": "Undefined", "targetStatus": "None", "targetAction": "None")

	//if TargetStatus == CurrentStatus, record to finialize the control operation
	if vmStatusTmp.TargetStatus == vmStatusTmp.Status {
		if vmStatusTmp.TargetStatus != StatusTerminated {
			vmStatusTmp.SystemMessage = vmStatusTmp.TargetStatus + "==" + vmStatusTmp.Status
			vmStatusTmp.TargetStatus = StatusComplete
			vmStatusTmp.TargetAction = ActionComplete

			//Get current public IP when status has been changed.
			//UpdateVmPublicIp(nsId, mcisId, temp)
			vmInfoTmp, err := GetVmCurrentPublicIp(nsId, mcisId, temp.Id)
			if err != nil {
				common.CBLog.Error(err)
				errorInfo.SystemMessage = err.Error()
				return errorInfo, err
			}
			temp.PublicIP = vmInfoTmp.PublicIp
			temp.SSHPort = vmInfoTmp.SSHPort

		} else {
			// Don't init TargetStatus if the TargetStatus is StatusTerminated. It is to finalize VM lifecycle if StatusTerminated.
			vmStatusTmp.TargetStatus = StatusTerminated
			vmStatusTmp.TargetAction = ActionTerminate
			vmStatusTmp.Status = StatusTerminated
			vmStatusTmp.SystemMessage = "This VM has been terminated. No action is acceptable except deletion"
		}
	}

	vmStatusTmp.PublicIp = temp.PublicIP
	vmStatusTmp.SSHPort = temp.SSHPort

	// Apply current status to vmInfo
	temp.Status = vmStatusTmp.Status
	temp.SystemMessage = vmStatusTmp.SystemMessage
	temp.TargetAction = vmStatusTmp.TargetAction
	temp.TargetStatus = vmStatusTmp.TargetStatus

	if cspVmId != "" {
		// don't update VM info, if cspVmId is empty
		UpdateVmInfo(nsId, mcisId, temp)
	}

	return vmStatusTmp, nil
}

// CoreGetMcisVmStatus is func to Get McisVm Status
func CoreGetMcisVmStatus(nsId string, mcisId string, vmId string) (*TbVmStatusInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := &TbVmStatusInfo{}
		common.CBLog.Error(err)
		return temp, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		temp := &TbVmStatusInfo{}
		common.CBLog.Error(err)
		return temp, err
	}

	err = common.CheckString(vmId)
	if err != nil {
		temp := &TbVmStatusInfo{}
		common.CBLog.Error(err)
		return temp, err
	}

	check, _ := CheckVm(nsId, mcisId, vmId)

	if !check {
		temp := &TbVmStatusInfo{}
		err := fmt.Errorf("The vm " + vmId + " does not exist.")
		return temp, err
	}

	fmt.Println("[status VM]")

	vmKey := common.GenMcisKey(nsId, mcisId, vmId)
	vmKeyValue, err := common.CBStore.Get(vmKey)
	if err != nil {
		err = fmt.Errorf("in CoreGetMcisVmStatus(); CBStore.Get() returned an error")
		common.CBLog.Error(err)
		// return nil, err
	}

	if vmKeyValue == nil {
		return nil, fmt.Errorf("Cannot find " + vmKey)
	}

	vmStatusResponse, err := GetVmStatus(nsId, mcisId, vmId)

	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	return &vmStatusResponse, nil
}

// [Update MCIS and VM object]

// UpdateMcisInfo is func to update MCIS Info (without VM info in MCIS)
func UpdateMcisInfo(nsId string, mcisInfoData TbMcisInfo) {

	mcisInfoData.Vm = nil

	key := common.GenMcisKey(nsId, mcisInfoData.Id, "")

	// Check existence of the key. If no key, no update.
	keyValue, err := common.CBStore.Get(key)
	if keyValue == nil || err != nil {
		return
	}

	mcisTmp := TbMcisInfo{}
	json.Unmarshal([]byte(keyValue.Value), &mcisTmp)

	if !reflect.DeepEqual(mcisTmp, mcisInfoData) {
		val, _ := json.Marshal(mcisInfoData)
		err = common.CBStore.Put(key, string(val))
		if err != nil {
			common.CBLog.Error(err)
		}
	}
}

// UpdateVmInfo is func to update VM Info
func UpdateVmInfo(nsId string, mcisId string, vmInfoData TbVmInfo) {
	key := common.GenMcisKey(nsId, mcisId, vmInfoData.Id)

	// Check existence of the key. If no key, no update.
	keyValue, err := common.CBStore.Get(key)
	if keyValue == nil || err != nil {
		return
	}

	vmTmp := TbVmInfo{}
	json.Unmarshal([]byte(keyValue.Value), &vmTmp)

	if !reflect.DeepEqual(vmTmp, vmInfoData) {
		val, _ := json.Marshal(vmInfoData)
		err = common.CBStore.Put(key, string(val))
		if err != nil {
			common.CBLog.Error(err)
		}
	}
}

// [Delete MCIS and VM object]

// DelMcis is func to delete MCIS object
func DelMcis(nsId string, mcisId string, option string) error {

	option = common.ToLower(option)

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}
	check, _ := CheckMcis(nsId, mcisId)

	if !check {
		err := fmt.Errorf("The mcis " + mcisId + " does not exist.")
		return err
	}

	fmt.Println("[Delete MCIS] " + mcisId)

	// Check MCIS status is Terminated so that approve deletion
	mcisStatus, _ := GetMcisStatus(nsId, mcisId)
	if mcisStatus == nil {
		err := fmt.Errorf("MCIS " + mcisId + " status nil, Deletion is not allowed (use option=force for force deletion)")
		common.CBLog.Error(err)
		if option != "force" {
			return err
		}
	}

	if !(!strings.Contains(mcisStatus.Status, "Partial-") && strings.Contains(mcisStatus.Status, StatusTerminated)) {

		// with terminate option, do MCIS refine and terminate in advance (skip if already StatusTerminated)
		if strings.EqualFold(option, ActionTerminate) {

			// ActionRefine
			_, err := HandleMcisAction(nsId, mcisId, ActionRefine)
			if err != nil {
				common.CBLog.Error(err)
				return err
			}

			// ActionTerminate
			_, err = HandleMcisAction(nsId, mcisId, ActionTerminate)
			if err != nil {
				common.CBLog.Error(err)
				return err
			}
			// for deletion, need to wait until termination is finished
			// Sleep for 5 seconds
			fmt.Printf("\n\n[Info] Sleep for 5 seconds for safe MCIS-VMs termination.\n\n")
			time.Sleep(5 * time.Second)
			mcisStatus, _ = GetMcisStatus(nsId, mcisId)
		}

	}

	// Check MCIS status is Terminated (not Partial)
	if mcisStatus.Id != "" && !(!strings.Contains(mcisStatus.Status, "Partial-") && (strings.Contains(mcisStatus.Status, StatusTerminated) || strings.Contains(mcisStatus.Status, StatusUndefined) || strings.Contains(mcisStatus.Status, StatusFailed))) {
		err := fmt.Errorf("MCIS " + mcisId + " is " + mcisStatus.Status + " and not " + StatusTerminated + "/" + StatusUndefined + "/" + StatusFailed + ", Deletion is not allowed (use option=force for force deletion)")
		common.CBLog.Error(err)
		if option != "force" {
			return err
		}
	}

	key := common.GenMcisKey(nsId, mcisId, "")
	fmt.Println(key)

	vmList, err := ListVmId(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	// delete vms info
	for _, v := range vmList {
		vmKey := common.GenMcisKey(nsId, mcisId, v)
		fmt.Println(vmKey)

		// get vm info
		vmInfo, err := GetVmObject(nsId, mcisId, v)
		if err != nil {
			common.CBLog.Error(err)
			return err
		}

		err = common.CBStore.Delete(vmKey)
		if err != nil {
			common.CBLog.Error(err)
			return err
		}

		mcir.UpdateAssociatedObjectList(nsId, common.StrImage, vmInfo.ImageId, common.StrDelete, vmKey)
		mcir.UpdateAssociatedObjectList(nsId, common.StrSpec, vmInfo.SpecId, common.StrDelete, vmKey)
		mcir.UpdateAssociatedObjectList(nsId, common.StrSSHKey, vmInfo.SshKeyId, common.StrDelete, vmKey)
		mcir.UpdateAssociatedObjectList(nsId, common.StrVNet, vmInfo.VNetId, common.StrDelete, vmKey)

		for _, v2 := range vmInfo.SecurityGroupIds {
			mcir.UpdateAssociatedObjectList(nsId, common.StrSecurityGroup, v2, common.StrDelete, vmKey)
		}
	}

	// delete vm group info
	vmGroupList, err := ListVmGroupId(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}
	for _, v := range vmGroupList {
		vmGroupKey := common.GenMcisVmGroupKey(nsId, mcisId, v)
		fmt.Println(vmGroupKey)
		err := common.CBStore.Delete(vmGroupKey)
		if err != nil {
			common.CBLog.Error(err)
			return err
		}
	}

	// delete mcis info
	err = common.CBStore.Delete(key)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	return nil
}

// DelMcisVm is func to delete VM object
func DelMcisVm(nsId string, mcisId string, vmId string, option string) error {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	err = common.CheckString(vmId)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}
	check, _ := CheckVm(nsId, mcisId, vmId)

	if !check {
		err := fmt.Errorf("The vm " + vmId + " does not exist.")
		return err
	}

	fmt.Println("[Delete VM] " + vmId)

	// ControlVm first
	err = ControlVm(nsId, mcisId, vmId, ActionTerminate)

	if err != nil {
		common.CBLog.Error(err)
		if option != "force" {
			return err
		}
	}
	// for deletion, need to wait until termination is finished
	// Sleep for 5 seconds
	fmt.Printf("\n\n[Info] Sleep for 20 seconds for safe VM termination.\n\n")
	time.Sleep(5 * time.Second)

	// get vm info
	vmInfo, _ := GetVmObject(nsId, mcisId, vmId)

	// delete vms info
	key := common.GenMcisKey(nsId, mcisId, vmId)
	err = common.CBStore.Delete(key)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	mcir.UpdateAssociatedObjectList(nsId, common.StrImage, vmInfo.ImageId, common.StrDelete, key)
	mcir.UpdateAssociatedObjectList(nsId, common.StrSpec, vmInfo.SpecId, common.StrDelete, key)
	mcir.UpdateAssociatedObjectList(nsId, common.StrSSHKey, vmInfo.SshKeyId, common.StrDelete, key)
	mcir.UpdateAssociatedObjectList(nsId, common.StrVNet, vmInfo.VNetId, common.StrDelete, key)

	for _, v2 := range vmInfo.SecurityGroupIds {
		mcir.UpdateAssociatedObjectList(nsId, common.StrSecurityGroup, v2, common.StrDelete, key)
	}

	return nil
}

// CoreDelAllMcis is func to delete all MCIS objects
func CoreDelAllMcis(nsId string, option string) (string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return "", err
	}

	mcisList, err := ListMcisId(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return "", err
	}

	if len(mcisList) == 0 {
		return "No MCIS to delete", nil
	}

	for _, v := range mcisList {
		err := DelMcis(nsId, v, option)
		if err != nil {
			common.CBLog.Error(err)
			return "", fmt.Errorf("Failed to delete All MCISs")
		}
	}

	return "All MCISs has been deleted", nil
}

// UpdateVmPublicIp is func to update VM public IP
func UpdateVmPublicIp(nsId string, mcisId string, vmInfoData TbVmInfo) error {

	vmInfoTmp, err := GetVmCurrentPublicIp(nsId, mcisId, vmInfoData.Id)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}
	if vmInfoData.PublicIP != vmInfoTmp.PublicIp || vmInfoData.SSHPort != vmInfoTmp.SSHPort {
		vmInfoData.PublicIP = vmInfoTmp.PublicIp
		vmInfoData.SSHPort = vmInfoTmp.SSHPort
		UpdateVmInfo(nsId, mcisId, vmInfoData)
	}
	return nil
}

// GetVmTemplate is func to get VM template
func GetVmTemplate(nsId string, mcisId string, algo string) (TbVmInfo, error) {

	fmt.Println("[GetVmTemplate]" + mcisId + " by algo: " + algo)

	vmList, err := ListVmId(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return TbVmInfo{}, err
	}
	if len(vmList) == 0 {
		return TbVmInfo{}, nil
	}

	rand.Seed(time.Now().UnixNano())
	index := rand.Intn(len(vmList))
	vmObj, vmErr := GetVmObject(nsId, mcisId, vmList[index])
	var vmTemplate TbVmInfo

	// only take template required to create VM
	vmTemplate.Name = vmObj.Name
	vmTemplate.ConnectionName = vmObj.ConnectionName
	vmTemplate.ImageId = vmObj.ImageId
	vmTemplate.SpecId = vmObj.SpecId
	vmTemplate.VNetId = vmObj.VNetId
	vmTemplate.SubnetId = vmObj.SubnetId
	vmTemplate.SecurityGroupIds = vmObj.SecurityGroupIds
	vmTemplate.SshKeyId = vmObj.SshKeyId
	vmTemplate.VmUserAccount = vmObj.VmUserAccount
	vmTemplate.VmUserPassword = vmObj.VmUserPassword

	if vmErr != nil {
		common.CBLog.Error(err)
		return TbVmInfo{}, vmErr
	}

	return vmTemplate, nil

}
