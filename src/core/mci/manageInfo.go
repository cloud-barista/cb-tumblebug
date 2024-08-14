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

// Package mci is to manage multi-cloud infra service
package mci

import (
	"encoding/json"
	"fmt"
	"reflect"

	//"log"
	"strconv"
	"strings"
	"time"

	//csv file handling

	"math/rand"
	"sort"

	// REST API (echo)

	"sync"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
)

// [MCI and VM object information managemenet]

// MciStatusInfo is struct to define simple information of MCI with updated status of all VMs
type MciStatusInfo struct {
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

	// Label is for describing the mci in a keyword (any string can be used)
	Label string `json:"label" example:"User custom label"`

	// SystemLabel is for describing the mci in a keyword (any string can be used) for special System purpose
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

	Location common.Location `json:"location"`
}

// ListMciId is func to list MCI ID
func ListMciId(nsId string) ([]string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	var mciList []string

	// Check MCI exists
	key := common.GenMciKey(nsId, "", "")
	key += "/"

	keyValue, err := kvstore.GetKvList(key)

	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	for _, v := range keyValue {
		if strings.Contains(v.Key, "/mci/") {
			trimmedString := strings.TrimPrefix(v.Key, (key + "mci/"))
			// prevent malformed key (if key for mci id includes '/', the key does not represent MCI ID)
			if !strings.Contains(trimmedString, "/") {
				mciList = append(mciList, trimmedString)
			}
		}
	}

	return mciList, nil
}

// ListVmId is func to list VM IDs
func ListVmId(nsId string, mciId string) ([]string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	var vmList []string

	// Check MCI exists
	key := common.GenMciKey(nsId, mciId, "")
	key += "/"

	_, err = kvstore.GetKv(key)
	if err != nil {
		log.Debug().Msg("[Not found] " + mciId)
		log.Error().Err(err).Msg("")
		return vmList, err
	}

	keyValue, err := kvstore.GetKvList(key)

	if err != nil {
		log.Error().Err(err).Msg("")
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

// ListVmByLabel is func to list VM by label
func ListVmByLabel(nsId string, mciId string, label string) ([]string, error) {

	log.Debug().Msg("[GetVmListByLabel]" + mciId + " by " + label)

	var vmListByLabel []string

	vmList, err := ListVmId(nsId, mciId)
	fmt.Println(vmList)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	if len(vmList) == 0 {
		return nil, nil
	}

	// delete vms info
	for _, v := range vmList {
		vmObj, vmErr := GetVmObject(nsId, mciId, v)
		if vmErr != nil {
			log.Error().Err(err).Msg("")
			return nil, vmErr
		}

		if vmObj.Label == label {
			log.Debug().Msg("Found VM with " + vmObj.Label + ", VM ID: " + vmObj.Id)
			vmListByLabel = append(vmListByLabel, vmObj.Id)
		}
	}
	return vmListByLabel, nil

}

// ListVmByFilter is func to get list VMs in a MCI by a filter consist of Key and Value
func ListVmByFilter(nsId string, mciId string, filterKey string, filterVal string) ([]string, error) {

	check, err := CheckMci(nsId, mciId)
	if !check {
		err := fmt.Errorf("Not found the MCI: " + mciId + " from the NS: " + nsId)
		return nil, err
	}

	vmList, err := ListVmId(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	if len(vmList) == 0 {
		return nil, nil
	}
	if filterKey == "" {
		return vmList, nil
	}

	var groupVmList []string

	for _, v := range vmList {
		vmObj, vmErr := GetVmObject(nsId, mciId, v)
		if vmErr != nil {
			log.Error().Err(err).Msg("")
			return nil, vmErr
		}
		vmObjReflect := reflect.ValueOf(&vmObj)
		elements := vmObjReflect.Elem()
		for i := 0; i < elements.NumField(); i++ {
			key := elements.Type().Field(i).Name
			if strings.EqualFold(filterKey, key) {
				//fmt.Println(key)

				val := elements.Field(i).Interface().(string)
				//fmt.Println(val)
				if strings.EqualFold(filterVal, val) {

					groupVmList = append(groupVmList, vmObj.Id)
					//fmt.Println(groupVmList)
				}

				break
			}
		}
	}
	return groupVmList, nil
}

// ListVmBySubGroup is func to get VM list with a SubGroup label in a specified MCI
func ListVmBySubGroup(nsId string, mciId string, groupId string) ([]string, error) {
	// SubGroupId is the Key for SubGroupId in TbVmInfo struct
	filterKey := "SubGroupId"
	return ListVmByFilter(nsId, mciId, filterKey, groupId)
}

// ListSubGroupId is func to return list of SubGroups in a given MCI
func ListSubGroupId(nsId string, mciId string) ([]string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	log.Debug().Msg("[ListSubGroupId]")
	key := common.GenMciKey(nsId, mciId, "")
	key += "/"

	keyValue, err := kvstore.GetKvList(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	var subGroupList []string
	for _, v := range keyValue {
		if strings.Contains(v.Key, "/subgroup/") {
			trimmedString := strings.TrimPrefix(v.Key, (key + "subgroup/"))
			// prevent malformed key (if key for vm id includes '/', the key does not represent VM ID)
			if !strings.Contains(trimmedString, "/") {
				subGroupList = append(subGroupList, trimmedString)
			}
		}
	}
	return subGroupList, nil
}

// GetMciInfo is func to return MCI information with the current status update
func GetMciInfo(nsId string, mciId string) (*TbMciInfo, error) {

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
	check, _ := CheckMci(nsId, mciId)

	if !check {
		temp := &TbMciInfo{}
		err := fmt.Errorf("The mci " + mciId + " does not exist.")
		return temp, err
	}

	mciObj, err := GetMciObject(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	// common.PrintJsonPretty(mciObj)

	mciStatus, err := GetMciStatus(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	// common.PrintJsonPretty(mciStatus)

	mciObj.Status = mciStatus.Status
	mciObj.StatusCount = mciStatus.StatusCount

	vmList, err := ListVmId(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	sort.Slice(mciObj.Vm, func(i, j int) bool {
		return mciObj.Vm[i].Id < mciObj.Vm[j].Id
	})

	for vmInfoIndex := range vmList {
		for vmStatusInfoIndex := range mciStatus.Vm {
			if mciObj.Vm[vmInfoIndex].Id == mciStatus.Vm[vmStatusInfoIndex].Id {
				mciObj.Vm[vmInfoIndex].Status = mciStatus.Vm[vmStatusInfoIndex].Status
				mciObj.Vm[vmInfoIndex].TargetStatus = mciStatus.Vm[vmStatusInfoIndex].TargetStatus
				mciObj.Vm[vmInfoIndex].TargetAction = mciStatus.Vm[vmStatusInfoIndex].TargetAction
				break
			}
		}
	}

	return &mciObj, nil
}

// GetMciAccessInfo is func to retrieve MCI Access information
func GetMciAccessInfo(nsId string, mciId string, option string) (*MciAccessInfo, error) {

	output := &MciAccessInfo{}
	temp := &MciAccessInfo{}
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return temp, err
	}
	check, _ := CheckMci(nsId, mciId)

	if !check {
		err := fmt.Errorf("The mci " + mciId + " does not exist.")
		return temp, err
	}

	output.MciId = mciId

	mcNlbAccess, err := GetMcNlbAccess(nsId, mciId)
	if err == nil {
		output.MciNlbListener = mcNlbAccess
	}

	subGroupList, err := ListSubGroupId(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return temp, err
	}
	// TODO: make in parallel

	for _, groupId := range subGroupList {
		subGroupAccessInfo := MciSubGroupAccessInfo{}
		subGroupAccessInfo.SubGroupId = groupId
		nlb, err := GetNLB(nsId, mciId, groupId)
		if err == nil {
			subGroupAccessInfo.NlbListener = &nlb.Listener
		}
		vmList, err := ListVmBySubGroup(nsId, mciId, groupId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return temp, err
		}
		var wg sync.WaitGroup
		chanResults := make(chan MciVmAccessInfo)

		for _, vmId := range vmList {
			wg.Add(1)
			go func(nsId string, mciId string, vmId string, option string, chanResults chan MciVmAccessInfo) {
				defer wg.Done()
				vmInfo, err := GetVmCurrentPublicIp(nsId, mciId, vmId)
				vmAccessInfo := MciVmAccessInfo{}
				if err != nil {
					log.Info().Err(err).Msg("")
					vmAccessInfo.PublicIP = ""
					vmAccessInfo.PrivateIP = ""
					vmAccessInfo.SSHPort = ""
				} else {
					vmAccessInfo.PublicIP = vmInfo.PublicIp
					vmAccessInfo.PrivateIP = vmInfo.PrivateIp
					vmAccessInfo.SSHPort = vmInfo.SSHPort
				}
				vmAccessInfo.VmId = vmId

				_, verifiedUserName, privateKey, err := GetVmSshKey(nsId, mciId, vmId)
				if err != nil {
					log.Error().Err(err).Msg("")
					vmAccessInfo.PrivateKey = ""
					vmAccessInfo.VmUserAccount = ""
				} else {
					if strings.EqualFold(option, "showSshKey") {
						vmAccessInfo.PrivateKey = privateKey
					}
					vmAccessInfo.VmUserAccount = verifiedUserName
				}

				//vmAccessInfo.VmUserPassword
				chanResults <- vmAccessInfo
			}(nsId, mciId, vmId, option, chanResults)
		}
		go func() {
			wg.Wait()
			close(chanResults)
		}()
		for result := range chanResults {
			subGroupAccessInfo.MciVmAccessInfo = append(subGroupAccessInfo.MciVmAccessInfo, result)
		}

		output.MciSubGroupAccessInfo = append(output.MciSubGroupAccessInfo, subGroupAccessInfo)
	}

	return output, nil
}

// ListMciInfo is func to get all MCI objects
func ListMciInfo(nsId string, option string) ([]TbMciInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	/*
		var content struct {
			//Name string     `json:"name"`
			Mci []mci.TbMciInfo `json:"mci"`
		}
	*/
	// content := RestGetAllMciResponse{}

	Mci := []TbMciInfo{}

	mciList, err := ListMciId(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	for _, v := range mciList {

		key := common.GenMciKey(nsId, v, "")
		keyValue, err := kvstore.GetKv(key)
		if err != nil {
			log.Error().Err(err).Msg("")
			err = fmt.Errorf("In CoreGetAllMci(); kvstore.GetKv() returned an error.")
			log.Error().Err(err).Msg("")
			// return nil, err
		}

		if keyValue == (kvstore.KeyValue{}) {
			return nil, fmt.Errorf("in CoreGetAllMci() mci loop; Cannot find " + key)
		}
		mciTmp := TbMciInfo{}
		json.Unmarshal([]byte(keyValue.Value), &mciTmp)
		mciId := v
		mciTmp.Id = mciId

		if option == "status" || option == "simple" {
			//get current mci status
			mciStatus, err := GetMciStatus(nsId, mciId)
			if err != nil {
				log.Error().Err(err).Msg("")
				return nil, err
			}
			mciTmp.Status = mciStatus.Status
		} else {
			//Set current mci status with NullStr
			mciTmp.Status = ""
		}

		// The cases with id, status, or others. except simple

		vmList, err := ListVmId(nsId, mciId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return nil, err
		}

		for _, v1 := range vmList {
			vmKey := common.GenMciKey(nsId, mciId, v1)
			vmKeyValue, err := kvstore.GetKv(key)
			if err != nil {
				err = fmt.Errorf("In CoreGetAllMci(); kvstore.GetKv() returned an error")
				log.Error().Err(err).Msg("")
				// return nil, err
			}

			if vmKeyValue == (kvstore.KeyValue{}) {
				return nil, fmt.Errorf("in CoreGetAllMci() vm loop; Cannot find " + vmKey)
			}
			vmTmp := TbVmInfo{}
			json.Unmarshal([]byte(vmKeyValue.Value), &vmTmp)
			vmTmp.Id = v1

			if option == "status" {
				//get current vm status
				vmStatusInfoTmp, err := FetchVmStatus(nsId, mciId, v1)
				if err != nil {
					log.Error().Err(err).Msg("")
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

			mciTmp.Vm = append(mciTmp.Vm, vmTmp)
		}

		Mci = append(Mci, mciTmp)
	}

	return Mci, nil
}

// ListVmInfo is func to Get MciVm Info
func ListVmInfo(nsId string, mciId string, vmId string) (*TbVmInfo, error) {

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

	err = common.CheckString(vmId)
	if err != nil {
		temp := &TbVmInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}
	check, _ := CheckVm(nsId, mciId, vmId)

	if !check {
		temp := &TbVmInfo{}
		err := fmt.Errorf("The vm " + vmId + " does not exist.")
		return temp, err
	}

	log.Debug().Msg("[Get MCI-VM info for id]" + vmId)

	key := common.GenMciKey(nsId, mciId, "")

	vmKey := common.GenMciKey(nsId, mciId, vmId)
	vmKeyValue, err := kvstore.GetKv(vmKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("In CoreGetMciVmInfo(); kvstore.GetKv() returned an error.")
		log.Error().Err(err).Msg("")
		// return nil, err
	}

	if vmKeyValue == (kvstore.KeyValue{}) {
		return nil, fmt.Errorf("Cannot find " + key)
	}
	vmTmp := TbVmInfo{}
	json.Unmarshal([]byte(vmKeyValue.Value), &vmTmp)
	vmTmp.Id = vmId

	//get current vm status
	vmStatusInfoTmp, err := FetchVmStatus(nsId, mciId, vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
	}

	vmTmp.Status = vmStatusInfoTmp.Status
	vmTmp.TargetStatus = vmStatusInfoTmp.TargetStatus
	vmTmp.TargetAction = vmStatusInfoTmp.TargetAction

	return &vmTmp, nil
}

// GetMciObject is func to retrieve MCI object from database (no current status update)
func GetMciObject(nsId string, mciId string) (TbMciInfo, error) {
	log.Debug().Msg("[GetMciObject]" + mciId)
	key := common.GenMciKey(nsId, mciId, "")
	keyValue, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return TbMciInfo{}, err
	}
	mciTmp := TbMciInfo{}
	json.Unmarshal([]byte(keyValue.Value), &mciTmp)

	vmList, err := ListVmId(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return TbMciInfo{}, err
	}

	for _, vmID := range vmList {
		vmtmp, err := GetVmObject(nsId, mciId, vmID)
		if err != nil {
			log.Error().Err(err).Msg("")
			return TbMciInfo{}, err
		}
		mciTmp.Vm = append(mciTmp.Vm, vmtmp)
	}

	return mciTmp, nil
}

// GetVmObject is func to get VM object
func GetVmObject(nsId string, mciId string, vmId string) (TbVmInfo, error) {
	key := common.GenMciKey(nsId, mciId, vmId)
	keyValue, err := kvstore.GetKv(key)
	if keyValue == (kvstore.KeyValue{}) || err != nil {
		err = fmt.Errorf("failed to get GetVmObject (ID: %s)", key)
		log.Error().Err(err).Msg("")
		return TbVmInfo{}, err
	}
	vmTmp := TbVmInfo{}
	err = json.Unmarshal([]byte(keyValue.Value), &vmTmp)
	if err != nil {
		err = fmt.Errorf("failed to get GetVmObject (ID: %s), message: failed to unmarshal", key)
		log.Error().Err(err).Msg("")
		return TbVmInfo{}, err
	}
	return vmTmp, nil
}

// GetVmIdNameInDetail is func to get ID and Name details
func GetVmIdNameInDetail(nsId string, mciId string, vmId string) (*TbIdNameInDetailInfo, error) {
	key := common.GenMciKey(nsId, mciId, vmId)
	keyValue, err := kvstore.GetKv(key)
	if keyValue == (kvstore.KeyValue{}) || err != nil {
		log.Error().Err(err).Msg("")
		return &TbIdNameInDetailInfo{}, err
	}
	vmTmp := TbVmInfo{}
	json.Unmarshal([]byte(keyValue.Value), &vmTmp)

	var idDetails TbIdNameInDetailInfo

	idDetails.IdInTb = vmTmp.Id
	idDetails.IdInSp = vmTmp.CspViewVmDetail.IId.NameId
	idDetails.IdInCsp = vmTmp.CspViewVmDetail.IId.SystemId
	idDetails.NameInCsp = "TBD"

	type spiderReqTmp struct {
		ConnectionName string `json:"ConnectionName"`
		ResourceType   string `json:"ResourceType"`
	}
	type spiderResTmp struct {
		Name string `json:"Name"`
	}

	var requestBody spiderReqTmp
	requestBody.ConnectionName = vmTmp.ConnectionName
	requestBody.ResourceType = "vm"

	callResult := spiderResTmp{}

	client := resty.New()
	url := fmt.Sprintf("%s/cspresourcename/%s", common.SpiderRestUrl, idDetails.IdInSp)
	method := "GET"
	client.SetTimeout(5 * time.Minute)

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
		log.Error().Err(err).Msg("")
		return &TbIdNameInDetailInfo{}, err
	}

	idDetails.NameInCsp = callResult.Name

	return &idDetails, nil
}

// [MCI and VM status management]

// GetMciStatus is func to Get Mci Status
func GetMciStatus(nsId string, mciId string) (*MciStatusInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return &MciStatusInfo{}, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return &MciStatusInfo{}, err
	}

	key := common.GenMciKey(nsId, mciId, "")

	keyValue, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return &MciStatusInfo{}, err
	}
	if keyValue == (kvstore.KeyValue{}) {
		err := fmt.Errorf("Not found [" + key + "]")
		log.Error().Err(err).Msg("")
		return &MciStatusInfo{}, err
	}

	mciStatus := MciStatusInfo{}
	json.Unmarshal([]byte(keyValue.Value), &mciStatus)

	mciTmp := TbMciInfo{}
	json.Unmarshal([]byte(keyValue.Value), &mciTmp)

	vmList, err := ListVmId(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return &MciStatusInfo{}, err
	}
	if len(vmList) == 0 {
		return &MciStatusInfo{}, nil
	}

	//goroutin sync wg
	var wg sync.WaitGroup
	for _, v := range vmList {
		wg.Add(1)
		go FetchVmStatusAsync(&wg, nsId, mciId, v, &mciStatus)
	}
	wg.Wait() //goroutine sync wg

	for _, v := range vmList {
		// set master IP of MCI (Default rule: select 1st Running VM as master)
		vmtmp, err := GetVmObject(nsId, mciId, v)
		if err == nil {
			if vmtmp.Status == StatusRunning {
				mciStatus.MasterVmId = vmtmp.Id
				mciStatus.MasterIp = vmtmp.PublicIP
				mciStatus.MasterSSHPort = vmtmp.SSHPort
				break
			}
		}
	}

	sort.Slice(mciStatus.Vm, func(i, j int) bool {
		return mciStatus.Vm[i].Id < mciStatus.Vm[j].Id
	})

	statusFlag := []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	statusFlagStr := []string{StatusFailed, StatusSuspended, StatusRunning, StatusTerminated, StatusCreating, StatusSuspending, StatusResuming, StatusRebooting, StatusTerminating, StatusUndefined}
	for _, v := range mciStatus.Vm {

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

	numVm := len(mciStatus.Vm)
	//numUnNormalStatus := statusFlag[0] + statusFlag[9]
	//numNormalStatus := numVm - numUnNormalStatus
	runningStatus := statusFlag[2]

	proportionStr := ":" + strconv.Itoa(tmpMax) + " (R:" + strconv.Itoa(runningStatus) + "/" + strconv.Itoa(numVm) + ")"
	if tmpMax == numVm {
		mciStatus.Status = statusFlagStr[tmpMaxIndex] + proportionStr
	} else if tmpMax < numVm {
		mciStatus.Status = "Partial-" + statusFlagStr[tmpMaxIndex] + proportionStr
	} else {
		mciStatus.Status = statusFlagStr[9] + proportionStr
	}
	// for representing Failed status in front.

	proportionStr = ":" + strconv.Itoa(statusFlag[0]) + " (R:" + strconv.Itoa(runningStatus) + "/" + strconv.Itoa(numVm) + ")"
	if statusFlag[0] > 0 {
		mciStatus.Status = "Partial-" + statusFlagStr[0] + proportionStr
		if statusFlag[0] == numVm {
			mciStatus.Status = statusFlagStr[0] + proportionStr
		}
	}

	// proportionStr = "-(" + strconv.Itoa(statusFlag[9]) + "/" + strconv.Itoa(numVm) + ")"
	// if statusFlag[9] > 0 {
	// 	mciStatus.Status = statusFlagStr[9] + proportionStr
	// }

	// Set mciStatus.StatusCount
	mciStatus.StatusCount.CountTotal = numVm
	mciStatus.StatusCount.CountFailed = statusFlag[0]
	mciStatus.StatusCount.CountSuspended = statusFlag[1]
	mciStatus.StatusCount.CountRunning = statusFlag[2]
	mciStatus.StatusCount.CountTerminated = statusFlag[3]
	mciStatus.StatusCount.CountCreating = statusFlag[4]
	mciStatus.StatusCount.CountSuspending = statusFlag[5]
	mciStatus.StatusCount.CountResuming = statusFlag[6]
	mciStatus.StatusCount.CountRebooting = statusFlag[7]
	mciStatus.StatusCount.CountTerminating = statusFlag[8]
	mciStatus.StatusCount.CountUndefined = statusFlag[9]

	isDone := true
	for _, v := range mciStatus.Vm {
		if v.TargetStatus != StatusComplete {
			if v.Status != StatusTerminated {
				isDone = false
			}
		}
	}
	if isDone {
		mciStatus.TargetAction = ActionComplete
		mciStatus.TargetStatus = StatusComplete
		mciTmp.TargetAction = ActionComplete
		mciTmp.TargetStatus = StatusComplete
		mciTmp.StatusCount = mciStatus.StatusCount
		UpdateMciInfo(nsId, mciTmp)
	}

	return &mciStatus, nil

	//need to change status

}

// ListMciStatus is func to get MCI status all
func ListMciStatus(nsId string) ([]MciStatusInfo, error) {

	//mciStatuslist := []MciStatusInfo{}
	mciList, err := ListMciId(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return []MciStatusInfo{}, err
	}

	var wg sync.WaitGroup
	chanResults := make(chan MciStatusInfo)
	var mciStatuslist []MciStatusInfo

	for _, mciId := range mciList {
		wg.Add(1)
		go func(nsId string, mciId string, chanResults chan MciStatusInfo) {
			defer wg.Done()
			mciStatus, err := GetMciStatus(nsId, mciId)
			if err != nil {
				log.Error().Err(err).Msg("")
			}
			chanResults <- *mciStatus
		}(nsId, mciId, chanResults)
	}

	go func() {
		wg.Wait()
		close(chanResults)
	}()
	for result := range chanResults {
		mciStatuslist = append(mciStatuslist, result)
	}

	return mciStatuslist, nil

	//need to change status

}

// GetVmCurrentPublicIp is func to get VM public IP
func GetVmCurrentPublicIp(nsId string, mciId string, vmId string) (TbVmStatusInfo, error) {
	errorInfo := TbVmStatusInfo{}
	errorInfo.Status = StatusFailed

	key := common.GenMciKey(nsId, mciId, vmId)
	keyValue, err := kvstore.GetKv(key)
	if err != nil || keyValue == (kvstore.KeyValue{}) {
		if keyValue == (kvstore.KeyValue{}) {
			log.Error().Err(err).Msgf("Not found: %s keyValue is nil", key)
			return errorInfo, fmt.Errorf("Not found: %s keyValue is nil", key)
		}
		log.Error().Err(err).Msg("")
		return errorInfo, err
	}

	temp := TbVmInfo{}
	err = json.Unmarshal([]byte(keyValue.Value), &temp)
	if err != nil {
		log.Error().Err(err).Msg("")
		return errorInfo, err
	}

	cspVmId := temp.CspViewVmDetail.IId.NameId
	if cspVmId == "" {
		err = fmt.Errorf("cspVmId is empty (VmId: %s)", vmId)
		log.Error().Err(err).Msg("")
		return errorInfo, err
	}

	type statusResponse struct {
		Status         string
		PublicIP       string
		PublicDNS      string
		PrivateIP      string
		PrivateDNS     string
		SSHAccessPoint string
	}

	client := resty.New()
	client.SetTimeout(2 * time.Minute)
	url := common.SpiderRestUrl + "/vm/" + cspVmId
	method := "GET"
	requestBody := common.SpiderConnectionName{}
	requestBody.ConnectionName = temp.ConnectionName
	callResult := statusResponse{}

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
		log.Trace().Err(err).Msg("")
		return errorInfo, err
	}

	vmStatusTmp := TbVmStatusInfo{}
	vmStatusTmp.PublicIp = callResult.PublicIP
	vmStatusTmp.PrivateIp = callResult.PrivateIP
	vmStatusTmp.SSHPort, _ = TrimIP(callResult.SSHAccessPoint)

	return vmStatusTmp, nil
}

// GetVmIp is func to get VM IP to return PublicIP, PrivateIP, SSHPort
func GetVmIp(nsId string, mciId string, vmId string) (string, string, string, error) {

	vmObject, err := GetVmObject(nsId, mciId, vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", "", "", err
	}

	return vmObject.PublicIP, vmObject.PrivateIP, vmObject.SSHPort, nil
}

// GetVmSpecId is func to get VM SpecId
func GetVmSpecId(nsId string, mciId string, vmId string) string {

	var content struct {
		SpecId string `json:"specId"`
	}

	log.Debug().Msg("[getVmSpecID]" + vmId)
	key := common.GenMciKey(nsId, mciId, vmId)

	keyValue, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("In GetVmSpecId(); kvstore.GetKv() returned an error.")
		log.Error().Err(err).Msg("")
		// return nil, err
	}

	json.Unmarshal([]byte(keyValue.Value), &content)

	fmt.Printf("%+v\n", content.SpecId)

	return content.SpecId
}

// FetchVmStatusAsync is func to get VM status async
func FetchVmStatusAsync(wg *sync.WaitGroup, nsId string, mciId string, vmId string, results *MciStatusInfo) error {
	defer wg.Done() //goroutine sync done

	if nsId != "" && mciId != "" && vmId != "" {
		vmStatusTmp, err := FetchVmStatus(nsId, mciId, vmId)
		if err != nil {
			log.Error().Err(err).Msg("")
			vmStatusTmp.Status = StatusFailed
			vmStatusTmp.SystemMessage = err.Error()
		}
		if vmStatusTmp != (TbVmStatusInfo{}) {
			results.Vm = append(results.Vm, vmStatusTmp)
		}
	}
	return nil
}

// FetchVmStatus is func to fetch VM status (call to CSPs)
func FetchVmStatus(nsId string, mciId string, vmId string) (TbVmStatusInfo, error) {

	errorInfo := TbVmStatusInfo{}

	temp, err := GetVmObject(nsId, mciId, vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
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
	errorInfo.SystemMessage = "Error in FetchVmStatus"

	cspVmId := temp.CspViewVmDetail.IId.NameId

	if (temp.TargetAction != ActionCreate && temp.TargetAction != ActionTerminate) && cspVmId == "" {
		err = fmt.Errorf("cspVmId is empty (VmId: %s)", vmId)
		log.Error().Err(err).Msg("")
		return errorInfo, err
	}

	type statusResponse struct {
		Status string
	}
	callResult := statusResponse{}
	callResult.Status = ""

	if temp.Status != StatusTerminated && cspVmId != "" {
		client := resty.New()
		url := common.SpiderRestUrl + "/vmstatus/" + cspVmId
		method := "GET"
		client.SetTimeout(60 * time.Second)

		type VMStatusReqInfo struct {
			ConnectionName string
		}
		requestBody := VMStatusReqInfo{}
		requestBody.ConnectionName = temp.ConnectionName

		// Retry to get right VM status from cb-spider. Sometimes cb-spider returns not approriate status.
		retrycheck := 2
		for i := 0; i < retrycheck; i++ {
			errorInfo.Status = StatusFailed
			err := common.ExecuteHttpRequest(
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
				errorInfo.SystemMessage = err.Error()
				callResult.Status = StatusUndefined
				break
			}
			if callResult.Status != "" {
				break
			}
			time.Sleep(5 * time.Second)
		}

	} else {
		callResult.Status = StatusUndefined
	}

	nativeStatus := callResult.Status

	// Define a map to validate nativeStatus
	var validStatuses = map[string]bool{
		StatusCreating:    true,
		StatusRunning:     true,
		StatusSuspending:  true,
		StatusSuspended:   true,
		StatusResuming:    true,
		StatusRebooting:   true,
		StatusTerminating: true,
		StatusTerminated:  true,
	}

	// Check if nativeStatus is a valid status, otherwise set to StatusUndefined
	if _, ok := validStatuses[nativeStatus]; ok {
		callResult.Status = nativeStatus
	} else {
		callResult.Status = StatusUndefined
	}

	temp, err = GetVmObject(nsId, mciId, vmId)
	if err != nil {
		log.Err(err).Msg("")
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
		if callResult.Status == StatusUndefined {
			callResult.Status = StatusCreating
		}
		if temp.Status == StatusFailed {
			callResult.Status = StatusFailed
		}
	}
	if vmStatusTmp.TargetAction == ActionTerminate {
		if callResult.Status == StatusUndefined {
			callResult.Status = StatusTerminated
		}
		if callResult.Status == StatusSuspending {
			callResult.Status = StatusTerminating
		}
	}
	if vmStatusTmp.TargetAction == ActionResume {
		if callResult.Status == StatusUndefined {
			callResult.Status = StatusResuming
		}
		if callResult.Status == StatusCreating {
			callResult.Status = StatusResuming
		}
	}
	// for action reboot, some csp's native status are suspending, suspended, creating, resuming
	if vmStatusTmp.TargetAction == ActionReboot {
		if callResult.Status == StatusUndefined {
			callResult.Status = StatusRebooting
		}
		if callResult.Status == StatusSuspending || callResult.Status == StatusSuspended || callResult.Status == StatusCreating || callResult.Status == StatusResuming {
			callResult.Status = StatusRebooting
		}
	}

	if vmStatusTmp.Status == StatusTerminated {
		callResult.Status = StatusTerminated
	}

	vmStatusTmp.Status = callResult.Status

	// TODO: Alibaba Undefined status error is not resolved yet.
	// (After Terminate action. "status": "Undefined", "targetStatus": "None", "targetAction": "None")

	//if TargetStatus == CurrentStatus, record to finialize the control operation
	if vmStatusTmp.TargetStatus == vmStatusTmp.Status {
		if vmStatusTmp.TargetStatus != StatusTerminated {
			vmStatusTmp.SystemMessage = vmStatusTmp.TargetStatus + "==" + vmStatusTmp.Status
			vmStatusTmp.TargetStatus = StatusComplete
			vmStatusTmp.TargetAction = ActionComplete

			//Get current public IP when status has been changed.
			vmInfoTmp, err := GetVmCurrentPublicIp(nsId, mciId, temp.Id)
			if err != nil {
				log.Error().Err(err).Msg("")
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
			vmStatusTmp.SystemMessage = "terminated VM. No action is acceptable except deletion"
		}
	}

	vmStatusTmp.PublicIp = temp.PublicIP
	vmStatusTmp.SSHPort = temp.SSHPort

	// Apply current status to vmInfo
	temp.Status = vmStatusTmp.Status
	temp.TargetAction = vmStatusTmp.TargetAction
	temp.TargetStatus = vmStatusTmp.TargetStatus
	temp.SystemMessage = vmStatusTmp.SystemMessage

	if cspVmId != "" {
		// don't update VM info, if cspVmId is empty
		UpdateVmInfo(nsId, mciId, temp)
	}

	return vmStatusTmp, nil
}

// GetMciVmStatus is func to Get MciVm Status
func GetMciVmStatus(nsId string, mciId string, vmId string) (*TbVmStatusInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := &TbVmStatusInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		temp := &TbVmStatusInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = common.CheckString(vmId)
	if err != nil {
		temp := &TbVmStatusInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	check, _ := CheckVm(nsId, mciId, vmId)

	if !check {
		temp := &TbVmStatusInfo{}
		err := fmt.Errorf("The vm " + vmId + " does not exist.")
		return temp, err
	}

	vmStatusResponse, err := FetchVmStatus(nsId, mciId, vmId)

	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	return &vmStatusResponse, nil
}

// [Update MCI and VM object]

// UpdateMciInfo is func to update MCI Info (without VM info in MCI)
func UpdateMciInfo(nsId string, mciInfoData TbMciInfo) {

	mciInfoData.Vm = nil

	key := common.GenMciKey(nsId, mciInfoData.Id, "")

	// Check existence of the key. If no key, no update.
	keyValue, err := kvstore.GetKv(key)
	if keyValue == (kvstore.KeyValue{}) || err != nil {
		return
	}

	mciTmp := TbMciInfo{}
	json.Unmarshal([]byte(keyValue.Value), &mciTmp)

	if !reflect.DeepEqual(mciTmp, mciInfoData) {
		val, _ := json.Marshal(mciInfoData)
		err = kvstore.Put(key, string(val))
		if err != nil {
			log.Error().Err(err).Msg("")
		}
	}
}

// UpdateVmInfo is func to update VM Info
func UpdateVmInfo(nsId string, mciId string, vmInfoData TbVmInfo) {
	key := common.GenMciKey(nsId, mciId, vmInfoData.Id)

	// Check existence of the key. If no key, no update.
	keyValue, err := kvstore.GetKv(key)
	if keyValue == (kvstore.KeyValue{}) || err != nil {
		return
	}

	vmTmp := TbVmInfo{}
	json.Unmarshal([]byte(keyValue.Value), &vmTmp)

	if !reflect.DeepEqual(vmTmp, vmInfoData) {
		val, _ := json.Marshal(vmInfoData)
		err = kvstore.Put(key, string(val))
		if err != nil {
			log.Error().Err(err).Msg("")
		}
	}
}

// ProvisionDataDisk is func to provision DataDisk to VM (create and attach to VM)
func ProvisionDataDisk(nsId string, mciId string, vmId string, u *mcir.TbDataDiskVmReq) (TbVmInfo, error) {
	vm, err := GetVmObject(nsId, mciId, vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return TbVmInfo{}, err
	}

	createDiskReq := mcir.TbDataDiskReq{
		Name:           u.Name,
		ConnectionName: vm.ConnectionName,
		DiskType:       u.DiskType,
		DiskSize:       u.DiskSize,
		Description:    u.Description,
	}

	newDataDisk, err := mcir.CreateDataDisk(nsId, &createDiskReq, "")
	if err != nil {
		log.Error().Err(err).Msg("")
		return TbVmInfo{}, err
	}
	retry := 3
	for i := 0; i < retry; i++ {
		vmInfo, err := AttachDetachDataDisk(nsId, mciId, vmId, common.AttachDataDisk, newDataDisk.Id, false)
		if err != nil {
			log.Error().Err(err).Msg("")
		} else {
			return vmInfo, nil
		}
		time.Sleep(5 * time.Second)
	}
	return TbVmInfo{}, err
}

// AttachDetachDataDisk is func to attach/detach DataDisk to/from VM
func AttachDetachDataDisk(nsId string, mciId string, vmId string, command string, dataDiskId string, force bool) (TbVmInfo, error) {
	vmKey := common.GenMciKey(nsId, mciId, vmId)

	// Check existence of the key. If no key, no update.
	keyValue, err := kvstore.GetKv(vmKey)
	if keyValue == (kvstore.KeyValue{}) || err != nil {
		err := fmt.Errorf("Failed to find 'ns/mci/vm': %s/%s/%s \n", nsId, mciId, vmId)
		log.Error().Err(err).Msg("")
		return TbVmInfo{}, err
	}

	vm := TbVmInfo{}
	json.Unmarshal([]byte(keyValue.Value), &vm)

	isInList := common.CheckElement(dataDiskId, vm.DataDiskIds)
	if command == common.DetachDataDisk && !isInList && !force {
		err := fmt.Errorf("Failed to find the dataDisk %s in the attached dataDisk list %v", dataDiskId, vm.DataDiskIds)
		log.Error().Err(err).Msg("")
		return TbVmInfo{}, err
	} else if command == common.AttachDataDisk && isInList && !force {
		err := fmt.Errorf("The dataDisk %s is already in the attached dataDisk list %v", dataDiskId, vm.DataDiskIds)
		log.Error().Err(err).Msg("")
		return TbVmInfo{}, err
	}

	dataDiskKey := common.GenResourceKey(nsId, common.StrDataDisk, dataDiskId)

	// Check existence of the key. If no key, no update.
	keyValue, err = kvstore.GetKv(dataDiskKey)
	if keyValue == (kvstore.KeyValue{}) || err != nil {
		return TbVmInfo{}, err
	}

	dataDisk := mcir.TbDataDiskInfo{}
	json.Unmarshal([]byte(keyValue.Value), &dataDisk)

	client := resty.New()
	method := "PUT"
	var callResult interface{}
	//var requestBody interface{}

	requestBody := mcir.SpiderDiskAttachDetachReqWrapper{
		ConnectionName: vm.ConnectionName,
		ReqInfo: mcir.SpiderDiskAttachDetachReq{
			VMName: vm.CspViewVmDetail.IId.NameId,
		},
	}

	var url string
	var cmdToUpdateAsso string

	switch command {
	case common.AttachDataDisk:
		//req = req.SetResult(&mcir.SpiderDiskInfo{})
		url = fmt.Sprintf("%s/disk/%s/attach", common.SpiderRestUrl, dataDisk.CspDataDiskName)

		cmdToUpdateAsso = common.StrAdd

	case common.DetachDataDisk:
		// req = req.SetResult(&bool)
		url = fmt.Sprintf("%s/disk/%s/detach", common.SpiderRestUrl, dataDisk.CspDataDiskName)

		cmdToUpdateAsso = common.StrDelete

	default:

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
		log.Error().Err(err).Msg("")
		return TbVmInfo{}, err
	}

	switch command {
	case common.AttachDataDisk:
		vm.DataDiskIds = append(vm.DataDiskIds, dataDiskId)
		// mcir.UpdateAssociatedObjectList(nsId, common.StrDataDisk, dataDiskId, common.StrAdd, vmKey)
	case common.DetachDataDisk:
		oldDataDiskIds := vm.DataDiskIds
		newDataDiskIds := oldDataDiskIds

		flag := false

		for i, oldDataDisk := range oldDataDiskIds {
			if oldDataDisk == dataDiskId {
				flag = true
				newDataDiskIds = append(oldDataDiskIds[:i], oldDataDiskIds[i+1:]...)
				break
			}
		}

		// Actually, in here, 'flag' cannot be false,
		// since isDataDiskAttached is confirmed to be 'true' in the beginning of this function.
		// Below is just a code snippet of 'defensive programming'.
		if !flag && !force {
			err := fmt.Errorf("Failed to find the dataDisk %s in the attached dataDisk list.", dataDiskId)
			log.Error().Err(err).Msg("")
			return TbVmInfo{}, err
		} else {
			vm.DataDiskIds = newDataDiskIds
		}
	}

	time.Sleep(8 * time.Second)
	method = "GET"
	url = fmt.Sprintf("%s/vm/%s", common.SpiderRestUrl, vm.CspViewVmDetail.IId.NameId)
	requestBodyConnection := common.SpiderConnectionName{
		ConnectionName: vm.ConnectionName,
	}
	var callResultSpiderVMInfo SpiderVMInfo

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBodyConnection),
		&requestBodyConnection,
		&callResultSpiderVMInfo,
		common.MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return vm, err
	}

	// fmt.Printf("in AttachDetachDataDisk(), updatedSpiderVM.DataDiskIIDs: %s", updatedSpiderVM.DataDiskIIDs) // for debug
	vm.CspViewVmDetail = callResultSpiderVMInfo

	UpdateVmInfo(nsId, mciId, vm)

	// Update TB DataDisk object's 'associatedObjects' field
	mcir.UpdateAssociatedObjectList(nsId, common.StrDataDisk, dataDiskId, cmdToUpdateAsso, vmKey)

	// Update TB DataDisk object's 'status' field
	// Just calling GetResource(dataDisk) once will update TB DataDisk object's 'status' field
	mcir.GetResource(nsId, common.StrDataDisk, dataDiskId)
	/*
		url = fmt.Sprintf("%s/disk/%s", common.SpiderRestUrl, dataDisk.CspDataDiskName)

		req = client.R().
			SetHeader("Content-Type", "application/json").
			SetBody(connectionName).
			SetResult(&mcir.SpiderDiskInfo{}) // or SetResult(AuthSuccess{}).
			//SetError(&AuthError{}).       // or SetError(AuthError{}).

		resp, err = req.Get(url)

		fmt.Printf("HTTP Status code: %d \n", resp.StatusCode())
		switch {
		case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
			err := fmt.Errorf(string(resp.Body()))
			fmt.Println("body: ", string(resp.Body()))
			log.Error().Err(err).Msg("")
			return vm, err
		}

		updatedSpiderDisk := resp.Result().(*mcir.SpiderDiskInfo)
		dataDisk.Status = updatedSpiderDisk.Status
		fmt.Printf("dataDisk.Status: %s \n", dataDisk.Status) // for debug
		mcir.UpdateResourceObject(nsId, common.StrDataDisk, dataDisk)
	*/

	return vm, nil
}

func GetAvailableDataDisks(nsId string, mciId string, vmId string, option string) (interface{}, error) {
	vmKey := common.GenMciKey(nsId, mciId, vmId)

	// Check existence of the key. If no key, no update.
	keyValue, err := kvstore.GetKv(vmKey)
	if keyValue == (kvstore.KeyValue{}) || err != nil {
		err := fmt.Errorf("Failed to find 'ns/mci/vm': %s/%s/%s \n", nsId, mciId, vmId)
		log.Error().Err(err).Msg("")
		return nil, err
	}

	vm := TbVmInfo{}
	json.Unmarshal([]byte(keyValue.Value), &vm)

	tbDataDisksInterface, err := mcir.ListResource(nsId, common.StrDataDisk, "", "")
	if err != nil {
		err := fmt.Errorf("Failed to get dataDisk List. \n")
		log.Error().Err(err).Msg("")
		return nil, err
	}

	jsonString, err := json.Marshal(tbDataDisksInterface)
	if err != nil {
		err := fmt.Errorf("Failed to marshal dataDisk list into JSON string. \n")
		log.Error().Err(err).Msg("")
		return nil, err
	}

	tbDataDisks := []mcir.TbDataDiskInfo{}
	json.Unmarshal(jsonString, &tbDataDisks)

	if option != "id" {
		return tbDataDisks, nil
	} else { // option == "id"
		idList := []string{}

		for _, v := range tbDataDisks {
			// Update Tb dataDisk object's status
			newObj, err := mcir.GetResource(nsId, common.StrDataDisk, v.Id)
			if err != nil {
				log.Error().Err(err).Msg("")
				return nil, err
			}
			tempObj := newObj.(mcir.TbDataDiskInfo)

			if v.ConnectionName == vm.ConnectionName && tempObj.Status == "Available" {
				idList = append(idList, v.Id)
			}
		}

		return idList, nil
	}
}

// [Delete MCI and VM object]

// DelMci is func to delete MCI object
func DelMci(nsId string, mciId string, option string) (common.IdList, error) {

	option = common.ToLower(option)
	deletedResources := common.IdList{}
	deleteStatus := "[Done] "

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return deletedResources, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return deletedResources, err
	}
	check, _ := CheckMci(nsId, mciId)

	if !check {
		err := fmt.Errorf("The mci " + mciId + " does not exist.")
		return deletedResources, err
	}

	log.Debug().Msg("[Delete MCI] " + mciId)

	// Check MCI status is Terminated so that approve deletion
	mciStatus, _ := GetMciStatus(nsId, mciId)
	if mciStatus == nil {
		err := fmt.Errorf("MCI " + mciId + " status nil, Deletion is not allowed (use option=force for force deletion)")
		log.Error().Err(err).Msg("")
		if option != "force" {
			return deletedResources, err
		}
	}

	if !(!strings.Contains(mciStatus.Status, "Partial-") && strings.Contains(mciStatus.Status, StatusTerminated)) {

		// with terminate option, do MCI refine and terminate in advance (skip if already StatusTerminated)
		if strings.EqualFold(option, ActionTerminate) {

			// ActionRefine
			_, err := HandleMciAction(nsId, mciId, ActionRefine, true)
			if err != nil {
				log.Error().Err(err).Msg("")
				return deletedResources, err
			}

			// ActionTerminate
			_, err = HandleMciAction(nsId, mciId, ActionTerminate, true)
			if err != nil {
				log.Error().Err(err).Msg("")
				return deletedResources, err
			}
			// for deletion, need to wait until termination is finished
			// Sleep for 5 seconds
			fmt.Printf("\n\n[Info] Sleep for 5 seconds for safe MCI-VMs termination.\n\n")
			time.Sleep(5 * time.Second)
			mciStatus, _ = GetMciStatus(nsId, mciId)
		}

	}

	// Check MCI status is Terminated (not Partial)
	if mciStatus.Id != "" && !(!strings.Contains(mciStatus.Status, "Partial-") && (strings.Contains(mciStatus.Status, StatusTerminated) || strings.Contains(mciStatus.Status, StatusUndefined) || strings.Contains(mciStatus.Status, StatusFailed))) {
		err := fmt.Errorf("MCI " + mciId + " is " + mciStatus.Status + " and not " + StatusTerminated + "/" + StatusUndefined + "/" + StatusFailed + ", Deletion is not allowed (use option=force for force deletion)")
		log.Error().Err(err).Msg("")
		if option != "force" {
			return deletedResources, err
		}
	}

	key := common.GenMciKey(nsId, mciId, "")

	// delete associated MCI Policy
	check, _ = CheckMciPolicy(nsId, mciId)
	if check {
		err = DelMciPolicy(nsId, mciId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return deletedResources, err
		}
		deletedResources.IdList = append(deletedResources.IdList, deleteStatus+"Policy: "+mciId)
	}

	vmList, err := ListVmId(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return deletedResources, err
	}

	// delete vms info
	for _, v := range vmList {
		vmKey := common.GenMciKey(nsId, mciId, v)
		fmt.Println(vmKey)

		// get vm info
		vmInfo, err := GetVmObject(nsId, mciId, v)
		if err != nil {
			log.Error().Err(err).Msg("")
			return deletedResources, err
		}

		err = kvstore.Delete(vmKey)
		if err != nil {
			log.Error().Err(err).Msg("")
			return deletedResources, err
		}

		_, err = mcir.UpdateAssociatedObjectList(nsId, common.StrImage, vmInfo.ImageId, common.StrDelete, vmKey)
		if err != nil {
			mcir.UpdateAssociatedObjectList(nsId, common.StrCustomImage, vmInfo.ImageId, common.StrDelete, vmKey)
		}

		//mcir.UpdateAssociatedObjectList(nsId, common.StrSpec, vmInfo.SpecId, common.StrDelete, vmKey)
		mcir.UpdateAssociatedObjectList(nsId, common.StrSSHKey, vmInfo.SshKeyId, common.StrDelete, vmKey)
		mcir.UpdateAssociatedObjectList(nsId, common.StrVNet, vmInfo.VNetId, common.StrDelete, vmKey)

		for _, v2 := range vmInfo.SecurityGroupIds {
			mcir.UpdateAssociatedObjectList(nsId, common.StrSecurityGroup, v2, common.StrDelete, vmKey)
		}

		for _, v2 := range vmInfo.DataDiskIds {
			mcir.UpdateAssociatedObjectList(nsId, common.StrDataDisk, v2, common.StrDelete, vmKey)
		}
		deletedResources.IdList = append(deletedResources.IdList, deleteStatus+"VM: "+v)
	}

	// delete subGroup info
	subGroupList, err := ListSubGroupId(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return deletedResources, err
	}
	for _, v := range subGroupList {
		subGroupKey := common.GenMciSubGroupKey(nsId, mciId, v)
		err := kvstore.Delete(subGroupKey)
		if err != nil {
			log.Error().Err(err).Msg("")
			return deletedResources, err
		}
		deletedResources.IdList = append(deletedResources.IdList, deleteStatus+"SubGroup: "+v)
	}

	// delete associated CSP NLBs
	forceFlag := "false"
	if option == "force" {
		forceFlag = "true"
	}
	output, err := DelAllNLB(nsId, mciId, "", forceFlag)
	if err != nil {
		log.Error().Err(err).Msg("")
		return deletedResources, err
	}
	deletedResources.IdList = append(deletedResources.IdList, output.IdList...)

	// delete associated MCI NLBs
	mciNlbId := mciId + "-nlb"
	check, _ = CheckMci(nsId, mciNlbId)
	if check {
		mciNlbDeleteResult, err := DelMci(nsId, mciNlbId, option)
		if err != nil {
			log.Error().Err(err).Msg("")
			return deletedResources, err
		}
		deletedResources.IdList = append(deletedResources.IdList, mciNlbDeleteResult.IdList...)
	}

	// delete mci info
	err = kvstore.Delete(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return deletedResources, err
	}
	deletedResources.IdList = append(deletedResources.IdList, deleteStatus+"MCI: "+mciId)

	return deletedResources, nil
}

// DelMciVm is func to delete VM object
func DelMciVm(nsId string, mciId string, vmId string, option string) error {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	err = common.CheckString(vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	check, _ := CheckVm(nsId, mciId, vmId)

	if !check {
		err := fmt.Errorf("The vm " + vmId + " does not exist.")
		return err
	}

	log.Debug().Msg("[Delete VM] " + vmId)

	// skip termination if option is force
	if option != "force" {
		// ControlVm first
		var wg sync.WaitGroup
		results := make(chan ControlVmResult, 1)
		wg.Add(1)
		go ControlVmAsync(&wg, nsId, mciId, vmId, ActionTerminate, results)
		checkErr := <-results
		wg.Wait()
		close(results)
		if checkErr.Error != nil {
			log.Info().Msg(checkErr.Error.Error())
			if option != "force" {
				return checkErr.Error
			}
		}
		// for deletion, need to wait until termination is finished
		// Sleep for 5 seconds
		fmt.Printf("\n\n[Info] Sleep for 20 seconds for safe VM termination.\n\n")
		time.Sleep(5 * time.Second)

	}

	// get vm info
	vmInfo, _ := GetVmObject(nsId, mciId, vmId)

	// delete vms info
	key := common.GenMciKey(nsId, mciId, vmId)
	err = kvstore.Delete(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	// remove empty SubGroups
	subGroup, err := ListSubGroupId(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list subGroup to remove")
		return err
	}
	for _, v := range subGroup {
		vmListInSubGroup, err := ListVmBySubGroup(nsId, mciId, v)
		if err != nil {
			log.Error().Err(err).Msg("Failed to list vm in subGroup to remove")
			return err
		}
		if len(vmListInSubGroup) == 0 {
			subGroupKey := common.GenMciSubGroupKey(nsId, mciId, v)
			err := kvstore.Delete(subGroupKey)
			if err != nil {
				log.Error().Err(err).Msg("Failed to remove the empty subGroup")
				return err
			}
		}
	}

	_, err = mcir.UpdateAssociatedObjectList(nsId, common.StrImage, vmInfo.ImageId, common.StrDelete, key)
	if err != nil {
		mcir.UpdateAssociatedObjectList(nsId, common.StrCustomImage, vmInfo.ImageId, common.StrDelete, key)
	}

	//mcir.UpdateAssociatedObjectList(nsId, common.StrSpec, vmInfo.SpecId, common.StrDelete, key)
	mcir.UpdateAssociatedObjectList(nsId, common.StrSSHKey, vmInfo.SshKeyId, common.StrDelete, key)
	mcir.UpdateAssociatedObjectList(nsId, common.StrVNet, vmInfo.VNetId, common.StrDelete, key)

	for _, v := range vmInfo.SecurityGroupIds {
		mcir.UpdateAssociatedObjectList(nsId, common.StrSecurityGroup, v, common.StrDelete, key)
	}

	for _, v := range vmInfo.DataDiskIds {
		mcir.UpdateAssociatedObjectList(nsId, common.StrDataDisk, v, common.StrDelete, key)
	}

	return nil
}

// DelAllMci is func to delete all MCI objects
func DelAllMci(nsId string, option string) (string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}

	mciList, err := ListMciId(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}

	if len(mciList) == 0 {
		return "No MCI to delete", nil
	}

	for _, v := range mciList {
		_, err := DelMci(nsId, v, option)
		if err != nil {
			log.Error().Err(err).Msg("")
			return "", fmt.Errorf("Failed to delete All MCIs")
		}
	}

	return "All MCIs has been deleted", nil
}

// UpdateVmPublicIp is func to update VM public IP
func UpdateVmPublicIp(nsId string, mciId string, vmInfoData TbVmInfo) error {

	vmInfoTmp, err := GetVmCurrentPublicIp(nsId, mciId, vmInfoData.Id)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	if vmInfoData.PublicIP != vmInfoTmp.PublicIp || vmInfoData.SSHPort != vmInfoTmp.SSHPort {
		vmInfoData.PublicIP = vmInfoTmp.PublicIp
		vmInfoData.SSHPort = vmInfoTmp.SSHPort
		UpdateVmInfo(nsId, mciId, vmInfoData)
	}
	return nil
}

// GetVmTemplate is func to get VM template
func GetVmTemplate(nsId string, mciId string, algo string) (TbVmInfo, error) {

	log.Debug().Msg("[GetVmTemplate]" + mciId + " by algo: " + algo)

	vmList, err := ListVmId(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return TbVmInfo{}, err
	}
	if len(vmList) == 0 {
		return TbVmInfo{}, nil
	}

	rand.Seed(time.Now().UnixNano())
	index := rand.Intn(len(vmList))
	vmObj, vmErr := GetVmObject(nsId, mciId, vmList[index])
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
		log.Error().Err(err).Msg("")
		return TbVmInfo{}, vmErr
	}

	return vmTemplate, nil

}
