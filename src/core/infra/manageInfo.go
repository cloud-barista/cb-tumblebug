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
	"reflect"

	"strconv"
	"strings"
	"time"

	"math/rand"
	"sort"
	"sync"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
)

var mciInfoMutex sync.Mutex

// [MCI and VM object information managemenet]

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

	_, _, err = kvstore.GetKv(key)
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

// ListVmByLabel is a function to list VM IDs by label
func ListVmByLabel(nsId string, mciId string, labelKey string) ([]string, error) {
	// Construct the label selector
	labelSelector := labelKey + " exists" + "," + model.LabelNamespace + "=" + nsId + "," + model.LabelMciId + "=" + mciId

	// Call GetResourcesByLabelSelector (returns []interface{})
	resources, err := label.GetResourcesByLabelSelector(model.StrVM, labelSelector)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get resources by label selector")
		return nil, err
	}

	// Slice to store the list of VM IDs
	var vmListByLabel []string

	// Convert []interface{} to VmInfo and extract IDs
	for _, resource := range resources {
		if vmInfo, ok := resource.(*model.VmInfo); ok {
			vmListByLabel = append(vmListByLabel, vmInfo.Id)
		} else {
			log.Warn().Msg("Resource is not of type VmInfo")
		}
	}

	// Return the list of VM IDs
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
	// SubGroupId is the Key for SubGroupId in model.VmInfo struct
	filterKey := "SubGroupId"
	return ListVmByFilter(nsId, mciId, filterKey, groupId)
}

// GetSubGroup is func to return list of SubGroups in a given MCI
func GetSubGroup(nsId string, mciId string, subGroupId string) (model.SubGroupInfo, error) {
	subGroupInfo := model.SubGroupInfo{}
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return subGroupInfo, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return subGroupInfo, err
	}

	key := common.GenMciSubGroupKey(nsId, mciId, subGroupId)
	keyValue, _, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return subGroupInfo, err
	}
	err = json.Unmarshal([]byte(keyValue.Value), &subGroupInfo)
	if err != nil {
		err = fmt.Errorf("failed to get subGroupInfo (Key: %s), message: failed to unmarshal", key)
		log.Error().Err(err).Msg("")
		return subGroupInfo, err
	}
	return subGroupInfo, nil
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

	//log.Debug().Msg("[ListSubGroupId]")
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
func GetMciInfo(nsId string, mciId string) (*model.MciInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := &model.MciInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		temp := &model.MciInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}
	check, _ := CheckMci(nsId, mciId)

	if !check {
		temp := &model.MciInfo{}
		err := fmt.Errorf("The mci " + mciId + " does not exist.")
		return temp, err
	}

	mciObj, _, err := GetMciObject(nsId, mciId)
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

	// add label info for VM
	for i := range mciObj.Vm {
		labelInfo, err := label.GetLabels(model.StrVM, mciObj.Vm[i].Uid)
		if err != nil {
			log.Error().Err(err).Msg("Cannot get the label info")
			return nil, err
		}
		mciObj.Vm[i].Label = labelInfo.Labels
	}

	// add label info
	labelInfo, err := label.GetLabels(model.StrMCI, mciObj.Uid)
	if err != nil {
		log.Error().Err(err).Msg("Cannot get the label info")
		return nil, err
	}
	mciObj.Label = labelInfo.Labels

	return &mciObj, nil
}

// GetMciAccessInfo is func to retrieve MCI Access information
func GetMciAccessInfo(nsId string, mciId string, option string) (*model.MciAccessInfo, error) {

	output := &model.MciAccessInfo{}
	temp := &model.MciAccessInfo{}
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
		subGroupAccessInfo := model.MciSubGroupAccessInfo{}
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
		chanResults := make(chan model.MciVmAccessInfo)

		for _, vmId := range vmList {
			wg.Add(1)
			go func(nsId string, mciId string, vmId string, option string, chanResults chan model.MciVmAccessInfo) {
				defer wg.Done()
				common.RandomSleep(0, len(vmList)/2)
				vmInfo, err := GetVmCurrentPublicIp(nsId, mciId, vmId)

				vmAccessInfo := model.MciVmAccessInfo{}
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

				vmObject, err := GetVmObject(nsId, mciId, vmId)
				if err != nil {
					log.Info().Err(err).Msg("")
				} else {
					vmAccessInfo.ConnectionConfig = vmObject.ConnectionConfig
				}

				_, verifiedUserName, privateKey, err := GetVmSshKey(nsId, mciId, vmId)
				if err != nil {
					log.Error().Err(err).Msg("")
					vmAccessInfo.PrivateKey = ""
					vmAccessInfo.VmUserName = ""
				} else {
					if strings.EqualFold(option, "showSshKey") {
						vmAccessInfo.PrivateKey = privateKey
					}
					vmAccessInfo.VmUserName = verifiedUserName
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

// GetMciVmAccessInfo is func to retrieve MCI Access information
func GetMciVmAccessInfo(nsId string, mciId string, vmId string, option string) (*model.MciVmAccessInfo, error) {

	output := &model.MciVmAccessInfo{}

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return output, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return output, err
	}
	check, _ := CheckMci(nsId, mciId)

	if !check {
		err := fmt.Errorf("The mci %s does not exist.", mciId)
		return output, err
	}

	output.VmId = vmId

	vmInfo, err := GetVmCurrentPublicIp(nsId, mciId, vmId)

	vmAccessInfo := &model.MciVmAccessInfo{}
	if err != nil {
		log.Info().Err(err).Msg("")
		return output, err
	} else {
		vmAccessInfo.PublicIP = vmInfo.PublicIp
		vmAccessInfo.PrivateIP = vmInfo.PrivateIp
		vmAccessInfo.SSHPort = vmInfo.SSHPort
	}
	vmAccessInfo.VmId = vmId

	vmObject, err := GetVmObject(nsId, mciId, vmId)
	if err != nil {
		log.Info().Err(err).Msg("")
		return output, err
	} else {
		vmAccessInfo.ConnectionConfig = vmObject.ConnectionConfig
	}

	_, verifiedUserName, privateKey, err := GetVmSshKey(nsId, mciId, vmId)
	if err != nil {
		log.Info().Err(err).Msg("")
		return output, err
	} else {
		if strings.EqualFold(option, "showSshKey") {
			vmAccessInfo.PrivateKey = privateKey
		}
		vmAccessInfo.VmUserName = verifiedUserName
	}

	output = vmAccessInfo

	return output, nil
}

// ListMciInfo is func to get all MCI objects
func ListMciInfo(nsId string, option string) ([]model.MciInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	Mci := []model.MciInfo{}

	mciList, err := ListMciId(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	for _, v := range mciList {

		mciTmp, err := GetMciInfo(nsId, v)
		if err != nil {
			log.Error().Err(err).Msg("")
			return nil, err
		}

		Mci = append(Mci, *mciTmp)
	}

	return Mci, nil
}

// ListVmInfo is func to Get MciVm Info
func ListVmInfo(nsId string, mciId string, vmId string) (*model.VmInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := &model.VmInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		temp := &model.VmInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = common.CheckString(vmId)
	if err != nil {
		temp := &model.VmInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}
	check, _ := CheckVm(nsId, mciId, vmId)

	if !check {
		temp := &model.VmInfo{}
		err := fmt.Errorf("The vm " + vmId + " does not exist.")
		return temp, err
	}

	log.Debug().Msg("[Get MCI-VM info for id]" + vmId)

	key := common.GenMciKey(nsId, mciId, "")

	vmKey := common.GenMciKey(nsId, mciId, vmId)
	vmKeyValue, exists, err := kvstore.GetKv(vmKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("kvstore.GetKv() returned an error.")
		log.Error().Err(err).Msg("")
		// return nil, err
	}

	if !exists {
		return nil, fmt.Errorf("Cannot find " + key)
	}
	vmTmp := model.VmInfo{}
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
func GetMciObject(nsId string, mciId string) (model.MciInfo, bool, error) {
	//log.Debug().Msg("[GetMciObject]" + mciId)
	key := common.GenMciKey(nsId, mciId, "")
	keyValue, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.MciInfo{}, false, err
	}
	if !exists {
		log.Warn().Msgf("no MCI found (ID: %s)", key)
		return model.MciInfo{}, false, err
	}

	mciTmp := model.MciInfo{}
	json.Unmarshal([]byte(keyValue.Value), &mciTmp)

	vmList, err := ListVmId(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.MciInfo{}, false, err
	}

	for _, vmID := range vmList {
		vmtmp, err := GetVmObject(nsId, mciId, vmID)
		if err != nil {
			log.Error().Err(err).Msg("")
			return model.MciInfo{}, false, err
		}
		mciTmp.Vm = append(mciTmp.Vm, vmtmp)
	}

	return mciTmp, true, nil
}

// GetVmObject is func to get VM object
func GetVmObject(nsId string, mciId string, vmId string) (model.VmInfo, error) {

	vmTmp := model.VmInfo{}
	key := common.GenMciKey(nsId, mciId, vmId)
	keyValue, exists, err := kvstore.GetKv(key)
	if err != nil {
		err = fmt.Errorf("failed to get GetVmObject (ID: %s)", key)
		log.Error().Err(err).Msg("")
		return model.VmInfo{}, err
	}
	if !exists {
		log.Warn().Msgf("no VM found (ID: %s)", key)
		return model.VmInfo{}, fmt.Errorf("no VM found (ID: %s)", key)
	}

	err = json.Unmarshal([]byte(keyValue.Value), &vmTmp)
	if err != nil {
		err = fmt.Errorf("failed to get GetVmObject (ID: %s), message: failed to unmarshal", key)
		log.Error().Err(err).Msg("")
		return model.VmInfo{}, err
	}
	return vmTmp, nil
}

// GetVmIdNameInDetail is func to get ID and Name details
func GetVmIdNameInDetail(nsId string, mciId string, vmId string) (*model.IdNameInDetailInfo, error) {
	key := common.GenMciKey(nsId, mciId, vmId)
	keyValue, _, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return &model.IdNameInDetailInfo{}, err
	}
	vmTmp := model.VmInfo{}
	json.Unmarshal([]byte(keyValue.Value), &vmTmp)

	var idDetails model.IdNameInDetailInfo

	idDetails.IdInTb = vmTmp.Id
	idDetails.IdInSp = vmTmp.CspResourceName
	idDetails.IdInCsp = vmTmp.CspResourceId
	idDetails.NameInCsp = vmTmp.CspResourceName

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
	url := fmt.Sprintf("%s/cspresourcename/%s", model.SpiderRestUrl, idDetails.IdInSp)
	method := "GET"
	client.SetTimeout(5 * time.Minute)

	err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&callResult,
		clientManager.MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return &model.IdNameInDetailInfo{}, err
	}

	idDetails.NameInCsp = callResult.Name

	return &idDetails, nil
}

// [MCI and VM status management]

// GetMciStatus is func to Get Mci Status
func GetMciStatus(nsId string, mciId string) (*model.MciStatusInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return &model.MciStatusInfo{}, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return &model.MciStatusInfo{}, err
	}

	key := common.GenMciKey(nsId, mciId, "")

	keyValue, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return &model.MciStatusInfo{}, err
	}
	if !exists {
		err := fmt.Errorf("Not found [" + key + "]")
		log.Error().Err(err).Msg("")
		return &model.MciStatusInfo{}, err
	}

	mciStatus := model.MciStatusInfo{}
	json.Unmarshal([]byte(keyValue.Value), &mciStatus)

	mciTmp := model.MciInfo{}
	json.Unmarshal([]byte(keyValue.Value), &mciTmp)

	vmList, err := ListVmId(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return &model.MciStatusInfo{}, err
	}
	if len(vmList) == 0 {
		return &mciStatus, nil
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
			if strings.EqualFold(vmtmp.Status, model.StatusRunning) {
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
	statusFlagStr := []string{model.StatusFailed, model.StatusSuspended, model.StatusRunning, model.StatusTerminated, model.StatusCreating, model.StatusSuspending, model.StatusResuming, model.StatusRebooting, model.StatusTerminating, model.StatusUndefined}
	for _, v := range mciStatus.Vm {

		switch v.Status {
		case model.StatusFailed:
			statusFlag[0]++
		case model.StatusSuspended:
			statusFlag[1]++
		case model.StatusRunning:
			statusFlag[2]++
		case model.StatusTerminated:
			statusFlag[3]++
		case model.StatusCreating:
			statusFlag[4]++
		case model.StatusSuspending:
			statusFlag[5]++
		case model.StatusResuming:
			statusFlag[6]++
		case model.StatusRebooting:
			statusFlag[7]++
		case model.StatusTerminating:
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
	// // for representing Failed status in front.

	// proportionStr = ":" + strconv.Itoa(statusFlag[0]) + " (R:" + strconv.Itoa(runningStatus) + "/" + strconv.Itoa(numVm) + ")"
	// if statusFlag[0] > 0 {
	// 	mciStatus.Status = "Partial-" + statusFlagStr[0] + proportionStr
	// 	if statusFlag[0] == numVm {
	// 		mciStatus.Status = statusFlagStr[0] + proportionStr
	// 	}
	// }

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

	// additional handling is required for TargetAction in under the Termination action
	isDone := true
	for _, v := range mciStatus.Vm {
		if v.TargetStatus != model.StatusComplete {
			if v.Status != model.StatusTerminated {
				isDone = false
			}
		}
	}
	if isDone {
		mciStatus.TargetAction = model.ActionComplete
		mciStatus.TargetStatus = model.StatusComplete
		// mciTmp.TargetAction = model.ActionComplete
		// mciTmp.TargetStatus = model.StatusComplete
		mciTmp.StatusCount = mciStatus.StatusCount
		UpdateMciInfo(nsId, mciTmp)
	}

	return &mciStatus, nil

	//need to change status

}

// ListMciStatus is func to get MCI status all
func ListMciStatus(nsId string) ([]model.MciStatusInfo, error) {

	//mciStatuslist := []model.MciStatusInfo{}
	mciList, err := ListMciId(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return []model.MciStatusInfo{}, err
	}

	var wg sync.WaitGroup
	chanResults := make(chan model.MciStatusInfo)
	var mciStatuslist []model.MciStatusInfo

	for _, mciId := range mciList {
		wg.Add(1)
		go func(nsId string, mciId string, chanResults chan model.MciStatusInfo) {
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
func GetVmCurrentPublicIp(nsId string, mciId string, vmId string) (model.VmStatusInfo, error) {
	errorInfo := model.VmStatusInfo{}
	errorInfo.Status = model.StatusFailed

	temp, err := GetVmObject(nsId, mciId, vmId) // to check if the VM exists
	if err != nil {
		log.Error().Err(err).Msg("")
		return errorInfo, err
	}

	cspResourceName := temp.CspResourceName
	if cspResourceName == "" {
		err = fmt.Errorf("cspResourceName is empty (VmId: %s)", vmId)
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
	url := model.SpiderRestUrl + "/vm/" + cspResourceName
	method := "GET"
	requestBody := model.SpiderConnectionName{}
	requestBody.ConnectionName = temp.ConnectionName
	callResult := statusResponse{}

	err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&callResult,
		clientManager.MediumDuration,
	)

	if err != nil {
		log.Trace().Err(err).Msg("")
		return errorInfo, err
	}

	vmStatusTmp := model.VmStatusInfo{}
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

	keyValue, _, err := kvstore.GetKv(key)
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
func FetchVmStatusAsync(wg *sync.WaitGroup, nsId string, mciId string, vmId string, results *model.MciStatusInfo) error {
	defer wg.Done() //goroutine sync done

	if nsId != "" && mciId != "" && vmId != "" {
		vmStatusTmp, err := FetchVmStatus(nsId, mciId, vmId)
		if err != nil {
			log.Error().Err(err).Msg("")
			vmStatusTmp.Status = model.StatusFailed
			vmStatusTmp.SystemMessage = err.Error()
		}
		if vmStatusTmp != (model.VmStatusInfo{}) {
			results.Vm = append(results.Vm, vmStatusTmp)
		}
	}
	return nil
}

// FetchVmStatus is func to fetch VM status (call to CSPs)
func FetchVmStatus(nsId string, mciId string, vmId string) (model.VmStatusInfo, error) {

	errorInfo := model.VmStatusInfo{}

	temp, err := GetVmObject(nsId, mciId, vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return errorInfo, err
	}

	errorInfo.Id = temp.Id
	errorInfo.Name = temp.Name
	errorInfo.CspResourceName = temp.CspResourceName
	errorInfo.PublicIp = temp.PublicIP
	errorInfo.SSHPort = temp.SSHPort
	errorInfo.PrivateIp = temp.PrivateIP
	errorInfo.NativeStatus = model.StatusUndefined
	errorInfo.TargetAction = temp.TargetAction
	errorInfo.TargetStatus = temp.TargetStatus
	errorInfo.Location = temp.Location
	errorInfo.MonAgentStatus = temp.MonAgentStatus
	errorInfo.CreatedTime = temp.CreatedTime
	errorInfo.SystemMessage = "Error in FetchVmStatus"

	cspResourceName := temp.CspResourceName

	if (temp.TargetAction != model.ActionCreate && temp.TargetAction != model.ActionTerminate) && cspResourceName == "" {
		err = fmt.Errorf("cspResourceName is empty (VmId: %s)", vmId)
		log.Error().Err(err).Msg("")
		return errorInfo, err
	}

	type statusResponse struct {
		Status string
	}
	callResult := statusResponse{}
	callResult.Status = ""

	if temp.Status != model.StatusTerminated && cspResourceName != "" {
		client := resty.New()
		url := model.SpiderRestUrl + "/vmstatus/" + cspResourceName
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
			errorInfo.Status = model.StatusFailed
			err := clientManager.ExecuteHttpRequest(
				client,
				method,
				url,
				nil,
				clientManager.SetUseBody(requestBody),
				&requestBody,
				&callResult,
				clientManager.MediumDuration,
			)
			if err != nil {
				errorInfo.SystemMessage = err.Error()
				callResult.Status = model.StatusUndefined
				break
			}
			if callResult.Status != "" {
				break
			}
			time.Sleep(5 * time.Second)
		}

	} else {
		callResult.Status = model.StatusUndefined
	}

	nativeStatus := callResult.Status

	// Define a map to validate nativeStatus
	var validStatuses = map[string]bool{
		model.StatusCreating:    true,
		model.StatusRunning:     true,
		model.StatusSuspending:  true,
		model.StatusSuspended:   true,
		model.StatusResuming:    true,
		model.StatusRebooting:   true,
		model.StatusTerminating: true,
		model.StatusTerminated:  true,
	}

	// Check if nativeStatus is a valid status, otherwise set to model.StatusUndefined
	if _, ok := validStatuses[nativeStatus]; ok {
		callResult.Status = nativeStatus
	} else {
		callResult.Status = model.StatusUndefined
	}

	temp, err = GetVmObject(nsId, mciId, vmId)
	if err != nil {
		log.Err(err).Msg("")
		return errorInfo, err
	}
	vmStatusTmp := model.VmStatusInfo{}
	vmStatusTmp.Id = temp.Id
	vmStatusTmp.Name = temp.Name
	vmStatusTmp.CspResourceName = temp.CspResourceName

	vmStatusTmp.PrivateIp = temp.PrivateIP
	vmStatusTmp.NativeStatus = nativeStatus
	vmStatusTmp.TargetAction = temp.TargetAction
	vmStatusTmp.TargetStatus = temp.TargetStatus
	vmStatusTmp.Location = temp.Location
	vmStatusTmp.MonAgentStatus = temp.MonAgentStatus
	vmStatusTmp.CreatedTime = temp.CreatedTime
	vmStatusTmp.SystemMessage = temp.SystemMessage

	//Correct undefined status using TargetAction
	if strings.EqualFold(vmStatusTmp.TargetAction, model.ActionCreate) {
		if strings.EqualFold(callResult.Status, model.StatusUndefined) {
			callResult.Status = model.StatusCreating
		}
		if strings.EqualFold(temp.Status, model.StatusFailed) {
			callResult.Status = model.StatusFailed
		}
	}
	if strings.EqualFold(vmStatusTmp.TargetAction, model.ActionTerminate) {
		if strings.EqualFold(callResult.Status, model.StatusUndefined) {
			callResult.Status = model.StatusTerminated
		}
		if strings.EqualFold(callResult.Status, model.StatusSuspending) {
			callResult.Status = model.StatusTerminating
		}
	}
	if strings.EqualFold(vmStatusTmp.TargetAction, model.ActionResume) {
		if strings.EqualFold(callResult.Status, model.StatusUndefined) {
			callResult.Status = model.StatusResuming
		}
		if strings.EqualFold(callResult.Status, model.StatusCreating) {
			callResult.Status = model.StatusResuming
		}
	}
	// for action reboot, some csp's native status are suspending, suspended, creating, resuming
	if strings.EqualFold(vmStatusTmp.TargetAction, model.ActionReboot) {
		if strings.EqualFold(callResult.Status, model.StatusUndefined) {
			callResult.Status = model.StatusRebooting
		}
		if strings.EqualFold(callResult.Status, model.StatusSuspending) || strings.EqualFold(callResult.Status, model.StatusSuspended) || strings.EqualFold(callResult.Status, model.StatusCreating) || strings.EqualFold(callResult.Status, model.StatusResuming) {
			callResult.Status = model.StatusRebooting
		}
	}

	if strings.EqualFold(vmStatusTmp.Status, model.StatusTerminated) {
		callResult.Status = model.StatusTerminated
	}

	vmStatusTmp.Status = callResult.Status

	// TODO: Alibaba Undefined status error is not resolved yet.
	// (After Terminate action. "status": "Undefined", "targetStatus": "None", "targetAction": "None")

	//if TargetStatus == CurrentStatus, record to finialize the control operation
	if vmStatusTmp.TargetStatus == vmStatusTmp.Status {
		if vmStatusTmp.TargetStatus != model.StatusTerminated {
			vmStatusTmp.SystemMessage = vmStatusTmp.TargetStatus + "==" + vmStatusTmp.Status
			vmStatusTmp.TargetStatus = model.StatusComplete
			vmStatusTmp.TargetAction = model.ActionComplete

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
			// Don't init TargetStatus if the TargetStatus is model.StatusTerminated. It is to finalize VM lifecycle if model.StatusTerminated.
			vmStatusTmp.TargetStatus = model.StatusTerminated
			vmStatusTmp.TargetAction = model.ActionTerminate
			vmStatusTmp.Status = model.StatusTerminated
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

	if cspResourceName != "" {
		// don't update VM info, if cspResourceName is empty
		UpdateVmInfo(nsId, mciId, temp)
	}

	return vmStatusTmp, nil
}

// GetMciVmStatus is func to Get MciVm Status
func GetMciVmStatus(nsId string, mciId string, vmId string) (*model.VmStatusInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := &model.VmStatusInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		temp := &model.VmStatusInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = common.CheckString(vmId)
	if err != nil {
		temp := &model.VmStatusInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	check, _ := CheckVm(nsId, mciId, vmId)

	if !check {
		temp := &model.VmStatusInfo{}
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
func UpdateMciInfo(nsId string, mciInfoData model.MciInfo) {
	mciInfoMutex.Lock()
	defer mciInfoMutex.Unlock()

	mciInfoData.Vm = nil

	key := common.GenMciKey(nsId, mciInfoData.Id, "")

	// Check existence of the key. If no key, no update.
	keyValue, exists, err := kvstore.GetKv(key)
	if !exists || err != nil {
		return
	}

	mciTmp := model.MciInfo{}
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
func UpdateVmInfo(nsId string, mciId string, vmInfoData model.VmInfo) {
	mciInfoMutex.Lock()
	defer func() {
		mciInfoMutex.Unlock()
	}()

	key := common.GenMciKey(nsId, mciId, vmInfoData.Id)

	// Check existence of the key. If no key, no update.
	keyValue, exists, err := kvstore.GetKv(key)
	if !exists || err != nil {
		return
	}

	vmTmp := model.VmInfo{}
	json.Unmarshal([]byte(keyValue.Value), &vmTmp)

	if !reflect.DeepEqual(vmTmp, vmInfoData) {
		val, _ := json.Marshal(vmInfoData)
		err = kvstore.Put(key, string(val))
		if err != nil {
			log.Error().Err(err).Msg("")
		}
	}
}

// GetMciAssociatedResources returns a list of associated resource IDs for given MCI info
func GetMciAssociatedResources(nsId string, mciId string) (model.MciAssociatedResourceList, error) {

	mciInfo, _, err := GetMciObject(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.MciAssociatedResourceList{}, err
	}

	vNetSet := make(map[string]struct{})
	cspVNetSet := make(map[string]struct{})
	subnetSet := make(map[string]struct{})
	cspSubnetSet := make(map[string]struct{})
	sgSet := make(map[string]struct{})
	dataDiskSet := make(map[string]struct{})
	sshKeySet := make(map[string]struct{})
	imageSet := make(map[string]struct{})
	specSet := make(map[string]struct{})
	connNameSet := make(map[string]struct{})
	providerNameSet := make(map[string]struct{})
	vmIdSet := make(map[string]struct{})
	subGroupIdSet := make(map[string]struct{})
	cspVmNameSet := make(map[string]struct{})
	cspVmIdSet := make(map[string]struct{})

	for _, vm := range mciInfo.Vm {
		if vm.VNetId != "" {
			vNetSet[vm.VNetId] = struct{}{}
		}
		if vm.CspVNetId != "" {
			cspVNetSet[vm.CspVNetId] = struct{}{}
		}
		if vm.SubnetId != "" {
			subnetSet[vm.SubnetId] = struct{}{}
		}
		if vm.CspSubnetId != "" {
			cspSubnetSet[vm.CspSubnetId] = struct{}{}
		}
		for _, sg := range vm.SecurityGroupIds {
			if sg != "" {
				sgSet[sg] = struct{}{}
			}
		}
		for _, dd := range vm.DataDiskIds {
			if dd != "" {
				dataDiskSet[dd] = struct{}{}
			}
		}
		if vm.SshKeyId != "" {
			sshKeySet[vm.SshKeyId] = struct{}{}
		}
		if vm.ImageId != "" {
			imageSet[vm.ImageId] = struct{}{}
		}
		if vm.SpecId != "" {
			specSet[vm.SpecId] = struct{}{}
		}
		if vm.ConnectionName != "" {
			connNameSet[vm.ConnectionName] = struct{}{}
		}
		if vm.ConnectionConfig.ProviderName != "" {
			providerNameSet[vm.ConnectionConfig.ProviderName] = struct{}{}
		}
		if vm.Id != "" {
			vmIdSet[vm.Id] = struct{}{}
		}
		if vm.SubGroupId != "" {
			subGroupIdSet[vm.SubGroupId] = struct{}{}
		}
		if vm.CspResourceName != "" {
			cspVmNameSet[vm.CspResourceName] = struct{}{}
		}
		if vm.CspResourceId != "" {
			cspVmIdSet[vm.CspResourceId] = struct{}{}
		}
	}

	toSlice := func(m map[string]struct{}) []string {
		s := make([]string, 0, len(m))
		for k := range m {
			s = append(s, k)
		}
		return s
	}

	return model.MciAssociatedResourceList{
		VNetIds:          toSlice(vNetSet),
		CspVNetIds:       toSlice(cspVNetSet),
		SubnetIds:        toSlice(subnetSet),
		CspSubnetIds:     toSlice(cspSubnetSet),
		SecurityGroupIds: toSlice(sgSet),
		DataDiskIds:      toSlice(dataDiskSet),
		SSHKeyIds:        toSlice(sshKeySet),
		ImageIds:         toSlice(imageSet),
		SpecIds:          toSlice(specSet),
		ConnectionNames:  toSlice(connNameSet),
		ProviderNames:    toSlice(providerNameSet),
		VmIds:            toSlice(vmIdSet),
		SubGroupIds:      toSlice(subGroupIdSet),
		CspVmNames:       toSlice(cspVmNameSet),
		CspVmIds:         toSlice(cspVmIdSet),
	}, nil
}

// ProvisionDataDisk is func to provision DataDisk to VM (create and attach to VM)
func ProvisionDataDisk(nsId string, mciId string, vmId string, u *model.DataDiskVmReq) (model.VmInfo, error) {
	vm, err := GetVmObject(nsId, mciId, vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.VmInfo{}, err
	}

	createDiskReq := model.DataDiskReq{
		Name:           u.Name,
		ConnectionName: vm.ConnectionName,
		DiskType:       u.DiskType,
		DiskSize:       u.DiskSize,
		Description:    u.Description,
	}

	newDataDisk, err := resource.CreateDataDisk(nsId, &createDiskReq, "")
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.VmInfo{}, err
	}
	retry := 3
	for i := 0; i < retry; i++ {
		vmInfo, err := AttachDetachDataDisk(nsId, mciId, vmId, model.AttachDataDisk, newDataDisk.Id, false)
		if err != nil {
			log.Error().Err(err).Msg("")
		} else {
			return vmInfo, nil
		}
		time.Sleep(5 * time.Second)
	}
	return model.VmInfo{}, err
}

// AttachDetachDataDisk is func to attach/detach DataDisk to/from VM
func AttachDetachDataDisk(nsId string, mciId string, vmId string, command string, dataDiskId string, force bool) (model.VmInfo, error) {
	vmKey := common.GenMciKey(nsId, mciId, vmId)

	// Check existence of the key. If no key, no update.
	keyValue, exists, err := kvstore.GetKv(vmKey)
	if !exists || err != nil {
		err := fmt.Errorf("Failed to find 'ns/mci/vm': %s/%s/%s \n", nsId, mciId, vmId)
		log.Error().Err(err).Msg("")
		return model.VmInfo{}, err
	}

	vm := model.VmInfo{}
	json.Unmarshal([]byte(keyValue.Value), &vm)

	isInList := common.CheckElement(dataDiskId, vm.DataDiskIds)
	if strings.EqualFold(command, model.DetachDataDisk) && !isInList && !force {
		err := fmt.Errorf("Failed to find the dataDisk %s in the attached dataDisk list %v", dataDiskId, vm.DataDiskIds)
		log.Error().Err(err).Msg("")
		return model.VmInfo{}, err
	} else if strings.EqualFold(command, model.AttachDataDisk) && isInList && !force {
		err := fmt.Errorf("The dataDisk %s is already in the attached dataDisk list %v", dataDiskId, vm.DataDiskIds)
		log.Error().Err(err).Msg("")
		return model.VmInfo{}, err
	}

	dataDiskKey := common.GenResourceKey(nsId, model.StrDataDisk, dataDiskId)

	// Check existence of the key. If no key, no update.
	keyValue, exists, err = kvstore.GetKv(dataDiskKey)
	if !exists || err != nil {
		return model.VmInfo{}, err
	}

	dataDisk := model.DataDiskInfo{}
	json.Unmarshal([]byte(keyValue.Value), &dataDisk)

	client := resty.New()
	method := "PUT"
	var callResult interface{}
	//var requestBody interface{}

	requestBody := model.SpiderDiskAttachDetachReqWrapper{
		ConnectionName: vm.ConnectionName,
		ReqInfo: model.SpiderDiskAttachDetachReq{
			VMName: vm.CspResourceName,
		},
	}

	var url string
	var cmdToUpdateAsso string

	switch command {
	case model.AttachDataDisk:
		//req = req.SetResult(&model.SpiderDiskInfo{})
		url = fmt.Sprintf("%s/disk/%s/attach", model.SpiderRestUrl, dataDisk.CspResourceName)

		cmdToUpdateAsso = model.StrAdd

	case model.DetachDataDisk:
		// req = req.SetResult(&bool)
		url = fmt.Sprintf("%s/disk/%s/detach", model.SpiderRestUrl, dataDisk.CspResourceName)

		cmdToUpdateAsso = model.StrDelete

	default:

	}

	err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&callResult,
		clientManager.MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return model.VmInfo{}, err
	}

	switch command {
	case model.AttachDataDisk:
		vm.DataDiskIds = append(vm.DataDiskIds, dataDiskId)
		// resource.UpdateAssociatedObjectList(nsId, model.StrDataDisk, dataDiskId, model.StrAdd, vmKey)
	case model.DetachDataDisk:
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
			return model.VmInfo{}, err
		} else {
			vm.DataDiskIds = newDataDiskIds
		}
	}

	time.Sleep(8 * time.Second)
	method = "GET"
	url = fmt.Sprintf("%s/vm/%s", model.SpiderRestUrl, vm.CspResourceName)
	requestBodyConnection := model.SpiderConnectionName{
		ConnectionName: vm.ConnectionName,
	}
	var callResultSpiderVMInfo model.SpiderVMInfo

	err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBodyConnection),
		&requestBodyConnection,
		&callResultSpiderVMInfo,
		clientManager.MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return vm, err
	}

	// fmt.Printf("in AttachDetachDataDisk(), updatedSpiderVM.DataDiskIIDs: %s", updatedSpiderVM.DataDiskIIDs) // for debug
	vm.AddtionalDetails = callResultSpiderVMInfo.KeyValueList

	UpdateVmInfo(nsId, mciId, vm)

	// Update TB DataDisk object's 'associatedObjects' field
	resource.UpdateAssociatedObjectList(nsId, model.StrDataDisk, dataDiskId, cmdToUpdateAsso, vmKey)

	// Update TB DataDisk object's 'status' field
	// Just calling GetResource(dataDisk) once will update TB DataDisk object's 'status' field
	resource.GetResource(nsId, model.StrDataDisk, dataDiskId)
	/*
		url = fmt.Sprintf("%s/disk/%s", model.SpiderRestUrl, dataDisk.CspResourceName)

		req = client.R().
			SetHeader("Content-Type", "application/json").
			SetBody(connectionName).
			SetResult(&resource.SpiderDiskInfo{}) // or SetResult(AuthSuccess{}).
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

		updatedSpiderDisk := resp.Result().(*resource.SpiderDiskInfo)
		dataDisk.Status = updatedSpiderDisk.Status
		fmt.Printf("dataDisk.Status: %s \n", dataDisk.Status) // for debug
		resource.UpdateResourceObject(nsId, model.StrDataDisk, dataDisk)
	*/

	return vm, nil
}

func GetAvailableDataDisks(nsId string, mciId string, vmId string, option string) (interface{}, error) {
	vmKey := common.GenMciKey(nsId, mciId, vmId)

	// Check existence of the key. If no key, no update.
	keyValue, exists, err := kvstore.GetKv(vmKey)
	if !exists || err != nil {
		err := fmt.Errorf("Failed to find 'ns/mci/vm': %s/%s/%s \n", nsId, mciId, vmId)
		log.Error().Err(err).Msg("")
		return nil, err
	}

	vm := model.VmInfo{}
	json.Unmarshal([]byte(keyValue.Value), &vm)

	tbDataDisksInterface, err := resource.ListResource(nsId, model.StrDataDisk, "", "")
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

	tbDataDisks := []model.DataDiskInfo{}
	json.Unmarshal(jsonString, &tbDataDisks)

	if option != "id" {
		return tbDataDisks, nil
	} else { // option == "id"
		idList := []string{}

		for _, v := range tbDataDisks {
			// Update Tb dataDisk object's status
			newObj, err := resource.GetResource(nsId, model.StrDataDisk, v.Id)
			if err != nil {
				log.Error().Err(err).Msg("")
				return nil, err
			}
			tempObj := newObj.(model.DataDiskInfo)

			if v.ConnectionName == vm.ConnectionName && tempObj.Status == "Available" {
				idList = append(idList, v.Id)
			}
		}

		return idList, nil
	}
}

// [Delete MCI and VM object]

// DelMci is func to delete MCI object
func DelMci(nsId string, mciId string, option string) (model.IdList, error) {

	option = common.ToLower(option)
	deletedResources := model.IdList{}
	deleteStatus := "[Done] "

	mciInfo, err := GetMciInfo(nsId, mciId)

	if err != nil {
		log.Error().Err(err).Msg("Cannot Delete Mci")
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

	if !(!strings.Contains(mciStatus.Status, "Partial-") && strings.Contains(mciStatus.Status, model.StatusTerminated)) {

		// with terminate option, do MCI refine and terminate in advance (skip if already model.StatusTerminated)
		if strings.EqualFold(option, model.ActionTerminate) {

			// ActionRefine
			_, err := HandleMciAction(nsId, mciId, model.ActionRefine, true)
			if err != nil {
				log.Error().Err(err).Msg("")
				return deletedResources, err
			}

			// model.ActionTerminate
			_, err = HandleMciAction(nsId, mciId, model.ActionTerminate, true)
			if err != nil {
				log.Error().Err(err).Msg("")
				return deletedResources, err
			}
			// for deletion, need to wait until termination is finished
			// Sleep for 5 seconds

			log.Info().Msg("Wait for MCI-VMs termination in 5 seconds")

			time.Sleep(5 * time.Second)
			mciStatus, _ = GetMciStatus(nsId, mciId)
		}

	}

	// Check MCI status is Terminated (not Partial)
	if mciStatus.Id != "" && !(!strings.Contains(mciStatus.Status, "Partial-") && (strings.Contains(mciStatus.Status, model.StatusTerminated) || strings.Contains(mciStatus.Status, model.StatusUndefined) || strings.Contains(mciStatus.Status, model.StatusFailed) || strings.Contains(mciStatus.Status, model.StatusPreparing) || strings.Contains(mciStatus.Status, model.StatusPrepared))) {
		err := fmt.Errorf("MCI " + mciId + " is " + mciStatus.Status + " and not " + model.StatusTerminated + "/" + model.StatusUndefined + "/" + model.StatusFailed + ", Deletion is not allowed (use option=force for force deletion)")
		log.Error().Err(err).Msg("")
		if option != "force" {
			return deletedResources, err
		}
	}

	key := common.GenMciKey(nsId, mciId, "")

	// delete associated MCI Policy
	check, _ := CheckMciPolicy(nsId, mciId)
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

		_, err = resource.UpdateAssociatedObjectList(nsId, model.StrImage, vmInfo.ImageId, model.StrDelete, vmKey)
		if err != nil {
			resource.UpdateAssociatedObjectList(nsId, model.StrCustomImage, vmInfo.ImageId, model.StrDelete, vmKey)
		}

		//resource.UpdateAssociatedObjectList(nsId, model.StrSpec, vmInfo.SpecId, model.StrDelete, vmKey)
		resource.UpdateAssociatedObjectList(nsId, model.StrSSHKey, vmInfo.SshKeyId, model.StrDelete, vmKey)
		resource.UpdateAssociatedObjectList(nsId, model.StrVNet, vmInfo.VNetId, model.StrDelete, vmKey)

		for _, v2 := range vmInfo.SecurityGroupIds {
			resource.UpdateAssociatedObjectList(nsId, model.StrSecurityGroup, v2, model.StrDelete, vmKey)
		}

		for _, v2 := range vmInfo.DataDiskIds {
			resource.UpdateAssociatedObjectList(nsId, model.StrDataDisk, v2, model.StrDelete, vmKey)
		}
		deletedResources.IdList = append(deletedResources.IdList, deleteStatus+"VM: "+v)

		err = label.DeleteLabelObject(model.StrVM, vmInfo.Uid)
		if err != nil {
			log.Error().Err(err).Msg("")
		}

	}

	// delete subGroup info
	subGroupList, err := ListSubGroupId(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return deletedResources, err
	}
	for _, v := range subGroupList {
		subGroupKey := common.GenMciSubGroupKey(nsId, mciId, v)
		subGroupInfo, err := GetSubGroup(nsId, mciId, v)
		if err != nil {
			log.Error().Err(err).Msg("Cannot get SubGroup")
			return deletedResources, err
		}

		err = kvstore.Delete(subGroupKey)
		if err != nil {
			log.Error().Err(err).Msg("")
			return deletedResources, err
		}
		deletedResources.IdList = append(deletedResources.IdList, deleteStatus+"SubGroup: "+v)

		err = label.DeleteLabelObject(model.StrSubGroup, subGroupInfo.Uid)
		if err != nil {
			log.Error().Err(err).Msg("")
		}
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

	err = label.DeleteLabelObject(model.StrMCI, mciInfo.Uid)
	if err != nil {
		log.Error().Err(err).Msg("")
	}

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
		results := make(chan model.ControlVmResult, 1)
		wg.Add(1)
		go ControlVmAsync(&wg, nsId, mciId, vmId, model.ActionTerminate, results)
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

	_, err = resource.UpdateAssociatedObjectList(nsId, model.StrImage, vmInfo.ImageId, model.StrDelete, key)
	if err != nil {
		resource.UpdateAssociatedObjectList(nsId, model.StrCustomImage, vmInfo.ImageId, model.StrDelete, key)
	}

	//resource.UpdateAssociatedObjectList(nsId, model.StrSpec, vmInfo.SpecId, model.StrDelete, key)
	resource.UpdateAssociatedObjectList(nsId, model.StrSSHKey, vmInfo.SshKeyId, model.StrDelete, key)
	resource.UpdateAssociatedObjectList(nsId, model.StrVNet, vmInfo.VNetId, model.StrDelete, key)

	for _, v := range vmInfo.SecurityGroupIds {
		resource.UpdateAssociatedObjectList(nsId, model.StrSecurityGroup, v, model.StrDelete, key)
	}

	for _, v := range vmInfo.DataDiskIds {
		resource.UpdateAssociatedObjectList(nsId, model.StrDataDisk, v, model.StrDelete, key)
	}

	err = label.DeleteLabelObject(model.StrVM, vmInfo.Uid)
	if err != nil {
		log.Error().Err(err).Msg("")
	}

	return nil
}

// DelAllMci is func to delete all MCI objects in parallel
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

	var wg sync.WaitGroup
	errCh := make(chan error, len(mciList))
	defer close(errCh)

	for _, v := range mciList {
		wg.Add(1)
		go func(mciId string) {
			defer wg.Done()
			_, err := DelMci(nsId, mciId, option)
			if err != nil {
				log.Error().Err(err).Str("mciId", mciId).Msg("Failed to delete MCI")
				errCh <- err
			}
		}(v)
	}

	wg.Wait()

	select {
	case err := <-errCh:
		return "", fmt.Errorf("failed to delete all MCIs: %v", err)
	default:
		return "All MCIs have been deleted", nil
	}
}

// UpdateVmPublicIp is func to update VM public IP
func UpdateVmPublicIp(nsId string, mciId string, vmInfoData model.VmInfo) error {

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
func GetVmTemplate(nsId string, mciId string, algo string) (model.VmInfo, error) {

	log.Debug().Msg("[GetVmTemplate]" + mciId + " by algo: " + algo)

	vmList, err := ListVmId(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.VmInfo{}, err
	}
	if len(vmList) == 0 {
		return model.VmInfo{}, nil
	}

	rand.Seed(time.Now().UnixNano())
	index := rand.Intn(len(vmList))
	vmObj, vmErr := GetVmObject(nsId, mciId, vmList[index])
	var vmTemplate model.VmInfo

	// only take template required to create VM
	vmTemplate.Name = vmObj.Name
	vmTemplate.ConnectionName = vmObj.ConnectionName
	vmTemplate.ImageId = vmObj.ImageId
	vmTemplate.SpecId = vmObj.SpecId
	vmTemplate.VNetId = vmObj.VNetId
	vmTemplate.SubnetId = vmObj.SubnetId
	vmTemplate.SecurityGroupIds = vmObj.SecurityGroupIds
	vmTemplate.SshKeyId = vmObj.SshKeyId
	vmTemplate.VmUserName = vmObj.VmUserName
	vmTemplate.VmUserPassword = vmObj.VmUserPassword

	if vmErr != nil {
		log.Error().Err(err).Msg("")
		return model.VmInfo{}, vmErr
	}

	return vmTemplate, nil

}
