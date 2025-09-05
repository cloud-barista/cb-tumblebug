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
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
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

	// Use existing ListMciVmInfo function instead of individual GetVmObject calls
	vmInfoList, err := ListMciVmInfo(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	var groupVmList []string

	for _, vmObj := range vmInfoList {
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

	// Get MCI information to check if it's being terminated
	mciInfo, err := GetMciInfo(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("failed to get MCI info")
		return temp, err
	}

	// Check if MCI is being terminated or terminate action
	if strings.EqualFold(mciInfo.Status, model.StatusTerminated) ||
		mciInfo.TargetAction == model.ActionTerminate {
		err := fmt.Errorf("MCI %s is currently being terminated or in terminate action (Status: %s, TargetAction: %s)",
			mciId, mciInfo.Status, mciInfo.TargetAction)
		log.Info().Msg(err.Error())
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
			// Check if VM is terminated before processing
			vmObject, err := GetVmObject(nsId, mciId, vmId)
			if err != nil {
				log.Debug().Err(err).Msgf("Failed to get VM object for %s, skipping", vmId)
				continue
			}

			// Skip terminated VMs as they don't have meaningful access info
			if strings.EqualFold(vmObject.Status, model.StatusTerminated) {
				log.Debug().Msgf("VM %s is terminated, skipping access info collection", vmId)
				continue
			}

			wg.Add(1)
			go func(nsId string, mciId string, vmId string, option string, chanResults chan model.MciVmAccessInfo) {
				defer wg.Done()
				common.RandomSleep(0, len(vmList)/2*1000)
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

// ListMciVmInfo is func to Get all VM Info objects in MCI
func ListMciVmInfo(nsId string, mciId string) ([]model.VmInfo, error) {

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

	// Check if MCI exists
	check, err := CheckMci(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msgf("Cannot check MCI %s exist", mciId)
		return nil, err
	}
	if !check {
		err := fmt.Errorf("MCI %s does not exist", mciId)
		return nil, err
	}

	// Get VM ID list using existing function
	vmIdList, err := ListVmId(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to list VM IDs for MCI %s", mciId)
		return nil, err
	}

	if len(vmIdList) == 0 {
		return []model.VmInfo{}, nil
	}

	// Use parallel processing for better performance when dealing with multiple VMs
	var wg sync.WaitGroup
	chanResults := make(chan model.VmInfo, len(vmIdList))

	// Process each VM in parallel, with existence validation
	for _, vmId := range vmIdList {
		wg.Add(1)
		go func(vmId string) {
			defer wg.Done()

			// Check if VM exists first to avoid race conditions during deletion
			vmKey := common.GenMciKey(nsId, mciId, vmId)
			_, exists, err := kvstore.GetKv(vmKey)
			if err != nil || !exists {
				// VM might be deleted by concurrent operations (e.g., DelMci)
				// This is normal during MCI deletion process, so use Debug level
				log.Debug().Msgf("VM object not found for vmId: %s (possibly deleted concurrently)", vmId)
				return // Skip this VM
			}

			vmInfo, err := GetVmObject(nsId, mciId, vmId)
			if err != nil {
				// Secondary check - VM might have been deleted between existence check and retrieval
				log.Debug().Err(err).Msgf("VM object retrieval failed for vmId: %s (possibly deleted concurrently)", vmId)
				return // Skip this VM
			}

			chanResults <- vmInfo
		}(vmId)
	}

	// Wait for all goroutines to complete and close the channel
	go func() {
		wg.Wait()
		close(chanResults)
	}()

	// Collect results from the channel
	var vmInfoList []model.VmInfo
	for vmInfo := range chanResults {
		vmInfoList = append(vmInfoList, vmInfo)
	}

	return vmInfoList, nil
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

	// Use existing ListMciVmInfo function instead of manually iterating through VMs
	vmInfoList, err := ListMciVmInfo(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.MciInfo{}, false, err
	}

	mciTmp.Vm = vmInfoList

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

// ConvertVmInfoToVmStatusInfo converts VmInfo to VmStatusInfo for MCI status operations
func ConvertVmInfoToVmStatusInfo(vmInfo model.VmInfo) model.VmStatusInfo {
	return model.VmStatusInfo{
		Id:              vmInfo.Id,
		Uid:             vmInfo.Uid,
		CspResourceName: vmInfo.CspResourceName,
		CspResourceId:   vmInfo.CspResourceId,
		Name:            vmInfo.Name,
		Status:          vmInfo.Status,
		TargetStatus:    vmInfo.TargetStatus,
		TargetAction:    vmInfo.TargetAction,
		NativeStatus:    "", // VmInfo doesn't have NativeStatus, will be updated by status fetch
		MonAgentStatus:  vmInfo.MonAgentStatus,
		SystemMessage:   vmInfo.SystemMessage,
		CreatedTime:     vmInfo.CreatedTime,
		PublicIp:        vmInfo.PublicIP,
		PrivateIp:       vmInfo.PrivateIP,
		SSHPort:         vmInfo.SSHPort,
		Location:        vmInfo.Location,
	}
}

// ConvertVmInfoListToVmStatusInfoList converts a slice of VmInfo to VmStatusInfo for MCI status operations
func ConvertVmInfoListToVmStatusInfoList(vmInfoList []model.VmInfo) []model.VmStatusInfo {
	vmStatusInfoList := make([]model.VmStatusInfo, len(vmInfoList))
	for i, vmInfo := range vmInfoList {
		vmStatusInfoList[i] = ConvertVmInfoToVmStatusInfo(vmInfo)
	}
	return vmStatusInfoList
}

// ensureVmStatusInfoComplete ensures all VMs from VmInfo are represented in MciStatus.Vm
// This handles cases where VM status fetch might have failed or VM is newly created
// ConvertMciInfoToMciStatusInfo converts MciInfo to MciStatusInfo (partial conversion for basic fields)
func ConvertMciInfoToMciStatusInfo(mciInfo model.MciInfo) model.MciStatusInfo {
	return model.MciStatusInfo{
		Id:              mciInfo.Id,
		Name:            mciInfo.Name,
		Status:          mciInfo.Status,
		StatusCount:     mciInfo.StatusCount,
		TargetStatus:    mciInfo.TargetStatus,
		TargetAction:    mciInfo.TargetAction,
		InstallMonAgent: mciInfo.InstallMonAgent,
		Label:           mciInfo.Label,
		SystemLabel:     mciInfo.SystemLabel,
		Vm:              ConvertVmInfoListToVmStatusInfoList(mciInfo.Vm),
		// MasterVmId, MasterIp, MasterSSHPort will be set by status determination logic
	}
}

// ConvertVmInfoFieldsToVmStatusInfo converts VmInfo fields into existing VmStatusInfo
// VmInfo is considered the trusted source, so all relevant fields are converted
func ConvertVmInfoFieldsToVmStatusInfo(vmStatus *model.VmStatusInfo, vmInfo model.VmInfo) {
	// Always convert from VmInfo as it's the trusted source
	vmStatus.CreatedTime = vmInfo.CreatedTime
	vmStatus.SystemMessage = vmInfo.SystemMessage
	vmStatus.MonAgentStatus = vmInfo.MonAgentStatus
	vmStatus.TargetStatus = vmInfo.TargetStatus
	vmStatus.TargetAction = vmInfo.TargetAction

	// Convert network information - VmInfo is authoritative
	vmStatus.PublicIp = vmInfo.PublicIP
	vmStatus.PrivateIp = vmInfo.PrivateIP
	vmStatus.SSHPort = vmInfo.SSHPort

	// Convert Status only if vmStatus doesn't have real-time CSP status
	// Keep NativeStatus from CSP calls, but convert Status from VmInfo if no real-time data
	if vmStatus.NativeStatus == "" {
		vmStatus.Status = vmInfo.Status
	}
	// If we have real-time CSP status (NativeStatus), keep the current Status
}

// ConvertVmInfoFieldsToVmStatusInfoList converts VmInfo fields into corresponding VmStatusInfo list
func ConvertVmInfoFieldsToVmStatusInfoList(vmStatusList []model.VmStatusInfo, vmInfoList []model.VmInfo) {
	// Create a map for efficient lookup
	vmInfoMap := make(map[string]model.VmInfo)
	for _, vmInfo := range vmInfoList {
		vmInfoMap[vmInfo.Id] = vmInfo
	}

	// Convert each VM status if corresponding VmInfo exists
	for i := range vmStatusList {
		if vmInfo, exists := vmInfoMap[vmStatusList[i].Id]; exists {
			ConvertVmInfoFieldsToVmStatusInfo(&vmStatusList[i], vmInfo)
		}
	}
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

	// Fetch VM statuses with rate limiting by CSP and region
	vmStatusList, err := fetchVmStatusesWithRateLimiting(nsId, mciId, vmList)
	if err != nil {
		log.Error().Err(err).Msg("")
		return &model.MciStatusInfo{}, err
	}

	// Copy results to mciStatus
	mciStatus.Vm = vmStatusList

	vmInfos, err := ListMciVmInfo(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return &model.MciStatusInfo{}, err
	}

	// If VM status fetch didn't populate all VMs, use VmInfo as fallback
	if len(mciStatus.Vm) == 0 && len(vmInfos) > 0 {
		log.Debug().Msgf("No VM status info found, converting from VmInfo for MCI: %s", mciId)
		mciStatus.Vm = ConvertVmInfoListToVmStatusInfoList(vmInfos)
	}

	for _, v := range vmInfos {
		if strings.EqualFold(v.Status, model.StatusRunning) {
			mciStatus.MasterVmId = v.Id
			mciStatus.MasterIp = v.PublicIP
			mciStatus.MasterSSHPort = v.SSHPort
			break
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
			log.Warn().Msgf("Undefined status (%s) found in VM %s of MCI %s", v.Status, v.Id, mciId)
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

	// Use the maximum of actual VM count and status VM count to handle race conditions during creation
	// During MCI creation, len(vmList) might be smaller than len(mciStatus.Vm) due to timing issues
	actualVmCount := len(vmList)
	statusVmCount := len(mciStatus.Vm)
	vmInfoCount := len(vmInfos)

	// Check if MCI is still being created to use more stable VM count calculation
	isCreating := strings.Contains(mciTmp.Status, model.StatusCreating) ||
		strings.Contains(mciTmp.TargetAction, model.ActionCreate) ||
		strings.Contains(mciTmp.TargetStatus, model.StatusRunning)

	// Check if MCI is in a stable state (all VMs have same stable status)
	isStableState := tmpMax == statusVmCount && tmpMax > 0
	stableStatusName := ""
	if isStableState && tmpMaxIndex < len(statusFlagStr) {
		stableStatusName = statusFlagStr[tmpMaxIndex]
	}

	var numVm int
	if isCreating {
		// During creation, use the larger of the two counts to avoid showing decreasing VM counts
		numVm = actualVmCount
		if statusVmCount > actualVmCount {
			numVm = statusVmCount
		}
		// Additionally, ensure we don't show a VM count smaller than the previous maximum
		if numVm < mciStatus.StatusCount.CountTotal && mciStatus.StatusCount.CountTotal > 0 {
			numVm = mciStatus.StatusCount.CountTotal
		}

		// If we still have inconsistent counts, use the MCI's stored VM information as fallback
		if len(mciTmp.Vm) > numVm {
			numVm = len(mciTmp.Vm)
		}

		log.Debug().Msgf("MCI %s is creating: using stable VM count (%d) - actual: %d, status: %d, previous: %d, stored: %d",
			mciId, numVm, actualVmCount, statusVmCount, mciStatus.StatusCount.CountTotal, len(mciTmp.Vm))
	} else if isStableState {
		// For stable MCI states (all VMs in same state), use the most reliable source to avoid count fluctuation
		// This applies to Terminated, Suspended, Failed, Running, etc.
		// Use the maximum of available counts, prioritizing vmInfos as they are stored persistently
		numVm = vmInfoCount
		if actualVmCount > numVm {
			numVm = actualVmCount
		}
		if len(mciTmp.Vm) > numVm {
			numVm = len(mciTmp.Vm)
		}
		// Ensure we don't show a count smaller than the actual VMs found in dominant status
		if tmpMax > numVm {
			numVm = tmpMax
		}

		log.Debug().Msgf("MCI %s is in stable state (%s): using stable VM count (%d) - actual: %d, status: %d, vmInfos: %d, stored: %d, dominant: %d",
			mciId, stableStatusName, numVm, actualVmCount, statusVmCount, vmInfoCount, len(mciTmp.Vm), tmpMax)
	} else {
		// MCI creation completed, use actual VM count from status
		numVm = statusVmCount
		// log.Debug().Msgf("MCI %s creation completed: using status VM count (%d)", mciId, numVm)
	}

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

	// Recovery/fallback handling for TargetAction completion
	// Primary completion should happen in actual control actions (control.go, provisioning.go)
	// This serves as a safety net for cases where the primary completion was missed
	isDone := true
	pendingVmsCount := 0

	// Check MCI target action to determine completion criteria
	mciTargetAction := mciTmp.TargetAction

	// Only perform recovery completion if TargetAction is not already Complete
	if mciTargetAction != model.ActionComplete && mciTargetAction != "" {
		for _, v := range mciStatus.Vm {
			// Check completion based on action type
			switch mciTargetAction {
			case model.ActionCreate:
				// For Create action, completion means all VMs reach final states (Running/Failed/Terminated/Suspended)
				// VM is considered pending if it's still in transitional states (Creating/Undefined/empty)
				// Failed state is considered a final state - provisioning attempt was completed even if unsuccessful
				if v.Status == model.StatusCreating || v.Status == model.StatusUndefined || v.Status == "" {
					isDone = false
					pendingVmsCount++
				}
				// All other states (Running, Failed, Terminated, Suspended) are considered final states

			case model.ActionTerminate:
				// For Terminate action, completion means all VMs reach Terminated state or non-recoverable states
				// Failed, Undefined, empty states are also considered "complete" as they can't proceed further
				if v.Status != model.StatusTerminated && v.Status != model.StatusFailed &&
					v.Status != model.StatusUndefined && v.Status != "" {
					isDone = false
					pendingVmsCount++
				}

			case model.ActionSuspend:
				// For Suspend action, completion means all VMs reach Suspended state or non-recoverable states
				// Failed, Terminated, Undefined, empty states are considered "complete"
				if v.Status != model.StatusSuspended && v.Status != model.StatusFailed &&
					v.Status != model.StatusTerminated && v.Status != model.StatusUndefined && v.Status != "" {
					isDone = false
					pendingVmsCount++
				}

			case model.ActionResume:
				// For Resume action, completion means all VMs reach Running state or non-recoverable states
				// Failed, Terminated, Undefined, empty states are considered "complete"
				if v.Status != model.StatusRunning && v.Status != model.StatusFailed &&
					v.Status != model.StatusTerminated && v.Status != model.StatusUndefined && v.Status != "" {
					isDone = false
					pendingVmsCount++
				}

			case model.ActionReboot:
				// For Reboot action, completion means all VMs reach Running state or non-recoverable states
				// Failed, Terminated, Undefined, empty states are considered "complete"
				if v.Status != model.StatusRunning && v.Status != model.StatusFailed &&
					v.Status != model.StatusTerminated && v.Status != model.StatusUndefined && v.Status != "" {
					isDone = false
					pendingVmsCount++
				}

			default:
				// For unknown actions, use the existing logic
				if v.TargetStatus != model.StatusComplete {
					if v.Status != model.StatusTerminated {
						isDone = false
						pendingVmsCount++
					}
				}
			}
		}

		// Log completion status for debugging
		log.Debug().Msgf("MCI %s %s recovery completion check: %d VMs total, %d pending, isDone=%t",
			mciId, mciTargetAction, len(mciStatus.Vm), pendingVmsCount, isDone)

		if isDone {
			log.Warn().Msgf("MCI %s action %s completed via RECOVERY PATH (primary completion in control.go/provisioning.go was missed) - VM states: %d total, %d pending",
				mciId, mciTargetAction, len(mciStatus.Vm), pendingVmsCount)

			// Add more detailed logging for debugging
			statusBreakdown := make(map[string]int)
			for _, v := range mciStatus.Vm {
				statusBreakdown[v.Status]++
			}
			log.Debug().Msgf("MCI %s recovery completion - VM status breakdown: %+v", mciId, statusBreakdown)

			mciStatus.TargetAction = model.ActionComplete
			mciStatus.TargetStatus = model.StatusComplete
			mciTmp.TargetAction = model.ActionComplete
			mciTmp.TargetStatus = model.StatusComplete
			mciTmp.StatusCount = mciStatus.StatusCount
			UpdateMciInfo(nsId, mciTmp)
		}
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

// Rate limiting constants for different levels
const (
	defaultMaxConcurrentRegionsPerCSP = 10 // Default maximum concurrent regions per CSP
	defaultMaxConcurrentVMsPerRegion  = 30 // Default maximum concurrent VMs per region
)

// CSP-specific rate limiting configurations
var cspRateLimits = map[string]struct {
	maxRegions      int
	maxVMsPerRegion int
}{
	csp.AWS:       {maxRegions: 10, maxVMsPerRegion: 30},
	csp.Azure:     {maxRegions: 8, maxVMsPerRegion: 25},
	csp.GCP:       {maxRegions: 12, maxVMsPerRegion: 35},
	csp.Alibaba:   {maxRegions: 6, maxVMsPerRegion: 20},
	csp.Tencent:   {maxRegions: 6, maxVMsPerRegion: 20},
	csp.NCP:       {maxRegions: 3, maxVMsPerRegion: 15}, // NCP has stricter limits
	csp.NHN:       {maxRegions: 5, maxVMsPerRegion: 20},
	csp.OpenStack: {maxRegions: 5, maxVMsPerRegion: 15},
}

// getRateLimitsForCSP returns rate limiting configuration for a specific CSP
func getRateLimitsForCSP(cspName string) (int, int) {
	// Normalize CSP name to lowercase for lookup
	normalizedCSP := strings.ToLower(cspName)

	if limits, exists := cspRateLimits[normalizedCSP]; exists {
		return limits.maxRegions, limits.maxVMsPerRegion
	}

	// Return default values for unknown CSPs
	return defaultMaxConcurrentRegionsPerCSP, defaultMaxConcurrentVMsPerRegion
}

// VmGroupInfo represents VM grouping information for rate limiting
type VmGroupInfo struct {
	VmId         string
	ProviderName string
	RegionName   string
}

// fetchVmStatusesWithRateLimiting fetches VM statuses with hierarchical rate limiting
// Level 1: CSPs are processed in parallel
// Level 2: Within each CSP, regions are processed with semaphore (maxConcurrentRegionsPerCSP)
// Level 3: Within each region, VMs are processed with semaphore (maxConcurrentVMsPerRegion)
func fetchVmStatusesWithRateLimiting(nsId, mciId string, vmList []string) ([]model.VmStatusInfo, error) {
	if len(vmList) == 0 {
		return []model.VmStatusInfo{}, nil
	}

	// Step 1: Group VMs by CSP and region
	vmGroups := make(map[string]map[string][]string) // CSP -> Region -> VmIds
	vmGroupInfos := make(map[string]VmGroupInfo)     // VmId -> GroupInfo

	for _, vmId := range vmList {
		vmInfo, err := GetVmObject(nsId, mciId, vmId)
		if err != nil {
			log.Warn().Err(err).Msgf("Failed to get VM object for %s, skipping", vmId)
			continue
		}

		providerName := vmInfo.ConnectionConfig.ProviderName
		regionName := vmInfo.Region.Region

		// Initialize CSP map if not exists
		if vmGroups[providerName] == nil {
			vmGroups[providerName] = make(map[string][]string)
		}

		// Add VM to the appropriate group
		vmGroups[providerName][regionName] = append(vmGroups[providerName][regionName], vmId)
		vmGroupInfos[vmId] = VmGroupInfo{
			VmId:         vmId,
			ProviderName: providerName,
			RegionName:   regionName,
		}
	}

	// Step 2: Process CSPs in parallel
	var wg sync.WaitGroup
	var mutex sync.Mutex
	var allVmStatuses []model.VmStatusInfo

	for csp, regions := range vmGroups {
		wg.Add(1)
		go func(providerName string, regionMap map[string][]string) {
			defer wg.Done()

			// Get rate limits for this specific CSP
			maxRegionsForCSP, maxVMsForRegion := getRateLimitsForCSP(providerName)

			// log.Debug().Msgf("Processing CSP: %s with %d regions (limits: %d regions, %d VMs/region)",
			// 	providerName, len(regionMap), maxRegionsForCSP, maxVMsForRegion)

			// Step 3: Process regions within CSP with rate limiting
			regionSemaphore := make(chan struct{}, maxRegionsForCSP)
			var regionWg sync.WaitGroup
			var regionMutex sync.Mutex
			var cspVmStatuses []model.VmStatusInfo

			for region, vmIds := range regionMap {
				regionWg.Add(1)
				go func(regionName string, vmIdList []string) {
					defer regionWg.Done()

					// Acquire region semaphore
					regionSemaphore <- struct{}{}
					defer func() { <-regionSemaphore }()

					// log.Debug().Msgf("Processing region: %s/%s with %d VMs (in parallel: %d VMs/region)",
					// 	providerName, regionName, len(vmIdList), maxVMsForRegion)

					// Step 4: Process VMs within region with rate limiting
					vmSemaphore := make(chan struct{}, maxVMsForRegion)
					var vmWg sync.WaitGroup
					var vmMutex sync.Mutex
					var regionVmStatuses []model.VmStatusInfo

					for _, vmId := range vmIdList {
						vmWg.Add(1)
						go func(vmId string) {
							defer vmWg.Done()

							// Acquire VM semaphore
							vmSemaphore <- struct{}{}
							defer func() { <-vmSemaphore }()

							// Fetch VM status
							vmStatusTmp, err := FetchVmStatus(nsId, mciId, vmId)
							if err != nil {
								log.Error().Err(err).Msgf("Failed to fetch status for VM %s", vmId)
								vmStatusTmp.Status = model.StatusFailed
								vmStatusTmp.SystemMessage = err.Error()
							}

							if vmStatusTmp != (model.VmStatusInfo{}) {
								vmMutex.Lock()
								regionVmStatuses = append(regionVmStatuses, vmStatusTmp)
								vmMutex.Unlock()
							}
						}(vmId)
					}
					vmWg.Wait()

					// Merge region results to CSP results
					regionMutex.Lock()
					cspVmStatuses = append(cspVmStatuses, regionVmStatuses...)
					regionMutex.Unlock()

				}(region, vmIds)
			}
			regionWg.Wait()

			// Merge CSP results to global results
			mutex.Lock()
			allVmStatuses = append(allVmStatuses, cspVmStatuses...)
			mutex.Unlock()

			// log.Debug().Msgf("Completed CSP: %s, processed %d VMs", providerName, len(cspVmStatuses))

		}(csp, regions)
	}

	wg.Wait()

	// Summary logging
	cspCount := len(vmGroups)
	totalRegions := 0
	for _, regions := range vmGroups {
		totalRegions += len(regions)
	}

	log.Debug().Msgf("Rate-limited VM status fetch completed: %d CSPs, %d regions, %d VMs processed",
		cspCount, totalRegions, len(allVmStatuses))
	return allVmStatuses, nil
}

// // FetchVmStatusAsync is func to get VM status async
// func FetchVmStatusAsync(wg *sync.WaitGroup, nsId string, mciId string, vmId string, results *model.MciStatusInfo) error {
// 	defer wg.Done() //goroutine sync done

// 	if nsId != "" && mciId != "" && vmId != "" {
// 		vmStatusTmp, err := FetchVmStatus(nsId, mciId, vmId)
// 		if err != nil {
// 			log.Error().Err(err).Msg("")
// 			vmStatusTmp.Status = model.StatusFailed
// 			vmStatusTmp.SystemMessage = err.Error()
// 		}
// 		if vmStatusTmp != (model.VmStatusInfo{}) {
// 			results.Vm = append(results.Vm, vmStatusTmp)
// 		}
// 	}
// 	return nil
// }

// populateVmStatusInfoFromVmInfo fills VmStatusInfo with data from VmInfo
// This is a helper function to avoid code duplication in FetchVmStatus
func populateVmStatusInfoFromVmInfo(statusInfo *model.VmStatusInfo, vmInfo model.VmInfo) {
	statusInfo.Id = vmInfo.Id
	statusInfo.Name = vmInfo.Name
	statusInfo.CspResourceName = vmInfo.CspResourceName
	statusInfo.PublicIp = vmInfo.PublicIP
	statusInfo.SSHPort = vmInfo.SSHPort
	statusInfo.PrivateIp = vmInfo.PrivateIP
	statusInfo.Status = vmInfo.Status
	statusInfo.TargetAction = vmInfo.TargetAction
	statusInfo.TargetStatus = vmInfo.TargetStatus
	statusInfo.Location = vmInfo.Location
	statusInfo.MonAgentStatus = vmInfo.MonAgentStatus
	statusInfo.CreatedTime = vmInfo.CreatedTime
	statusInfo.SystemMessage = vmInfo.SystemMessage
}

// FetchVmStatus is func to fetch VM status (call to CSPs)
func FetchVmStatus(nsId string, mciId string, vmId string) (model.VmStatusInfo, error) {

	statusInfo := model.VmStatusInfo{}

	vmInfo, err := GetVmObject(nsId, mciId, vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return statusInfo, err
	}

	// Check if we should skip CSP API call based on VM state
	// Skip API calls for stable final states or when CSP resource doesn't exist
	shouldSkipCSPCall := false

	// Define stable states that don't require frequent CSP API calls
	// These states are relatively stable and don't change frequently
	stableStates := map[string]bool{
		model.StatusTerminated: true,
		model.StatusFailed:     true,
		model.StatusSuspended:  true, // Suspended VMs are stable until explicitly resumed
	}

	// Skip CSP API call for stable states
	if stableStates[vmInfo.Status] {
		shouldSkipCSPCall = true
	}

	// Skip CSP API call if cspResourceName is empty (VM not properly created)
	if vmInfo.CspResourceName == "" && vmInfo.TargetAction != model.ActionCreate {
		shouldSkipCSPCall = true
	}

	if shouldSkipCSPCall {
		// log.Debug().Msgf("VM %s: %s, skipping CSP status fetch", vmId, skipReason)
		// Return complete status info using stored VM info
		populateVmStatusInfoFromVmInfo(&statusInfo, vmInfo)
		statusInfo.NativeStatus = vmInfo.Status
		return statusInfo, nil
	}

	populateVmStatusInfoFromVmInfo(&statusInfo, vmInfo)
	statusInfo.NativeStatus = model.StatusUndefined

	cspResourceName := vmInfo.CspResourceName

	if (vmInfo.TargetAction != model.ActionCreate && vmInfo.TargetAction != model.ActionTerminate) && cspResourceName == "" {
		err = fmt.Errorf("cspResourceName is empty (VmId: %s)", vmId)
		log.Error().Err(err).Msg("")
		return statusInfo, err
	}

	type statusResponse struct {
		Status string
	}
	callResult := statusResponse{}
	callResult.Status = ""

	if vmInfo.Status != model.StatusTerminated && cspResourceName != "" {
		client := resty.New()
		url := model.SpiderRestUrl + "/vmstatus/" + cspResourceName
		method := "GET"
		client.SetTimeout(60 * time.Second)

		type VMStatusReqInfo struct {
			ConnectionName string
		}
		requestBody := VMStatusReqInfo{}
		requestBody.ConnectionName = vmInfo.ConnectionName

		// Retry to get right VM status from cb-spider. Sometimes cb-spider returns not approriate status.
		retrycheck := 2
		for i := 0; i < retrycheck; i++ {
			statusInfo.Status = model.StatusFailed
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
				statusInfo.SystemMessage = err.Error()

				// check if VM is already Terminated
				if vmInfo.Status == model.StatusTerminated {
					// VM was already terminated, maintain the status instead of marking as Undefined
					log.Debug().Msgf("VM %s does not exist in CSP but is already Terminated, maintaining status", vmId)
					callResult.Status = model.StatusTerminated
				} else {
					callResult.Status = model.StatusUndefined
				}
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

	vmInfo, err = GetVmObject(nsId, mciId, vmId)
	if err != nil {
		log.Err(err).Msg("")
		return statusInfo, err
	}
	vmStatusTmp := model.VmStatusInfo{}
	vmStatusTmp.Id = vmInfo.Id
	vmStatusTmp.Name = vmInfo.Name
	vmStatusTmp.CspResourceName = vmInfo.CspResourceName
	vmStatusTmp.Status = vmInfo.Status // Set the current status first
	vmStatusTmp.PrivateIp = vmInfo.PrivateIP
	vmStatusTmp.NativeStatus = nativeStatus
	vmStatusTmp.TargetAction = vmInfo.TargetAction
	vmStatusTmp.TargetStatus = vmInfo.TargetStatus
	vmStatusTmp.Location = vmInfo.Location
	vmStatusTmp.MonAgentStatus = vmInfo.MonAgentStatus
	vmStatusTmp.CreatedTime = vmInfo.CreatedTime
	vmStatusTmp.SystemMessage = vmInfo.SystemMessage

	//Correct undefined status using TargetAction
	if strings.EqualFold(vmStatusTmp.TargetAction, model.ActionCreate) {
		if strings.EqualFold(callResult.Status, model.StatusUndefined) {
			callResult.Status = model.StatusCreating
		}
		if strings.EqualFold(vmInfo.Status, model.StatusFailed) {
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
			vmInfoTmp, err := GetVmCurrentPublicIp(nsId, mciId, vmInfo.Id)
			if err != nil {
				log.Error().Err(err).Msg("")
				statusInfo.SystemMessage = err.Error()
				return statusInfo, err
			}
			vmInfo.PublicIP = vmInfoTmp.PublicIp
			vmInfo.SSHPort = vmInfoTmp.SSHPort

		} else {
			// Don't init TargetStatus if the TargetStatus is model.StatusTerminated. It is to finalize VM lifecycle if model.StatusTerminated.
			vmStatusTmp.TargetStatus = model.StatusTerminated
			vmStatusTmp.TargetAction = model.ActionTerminate
			vmStatusTmp.Status = model.StatusTerminated
			vmStatusTmp.SystemMessage = "terminated VM. No action is acceptable except deletion"
		}
	}

	vmStatusTmp.PublicIp = vmInfo.PublicIP
	vmStatusTmp.SSHPort = vmInfo.SSHPort

	// Apply current status to vmInfo only if VM is not already terminated
	// Prevent overwriting Terminated status with empty or other states
	originalVmInfo, _ := GetVmObject(nsId, mciId, vmId)
	if originalVmInfo.Status != model.StatusTerminated {
		vmInfo.Status = vmStatusTmp.Status
		vmInfo.TargetAction = vmStatusTmp.TargetAction
		vmInfo.TargetStatus = vmStatusTmp.TargetStatus
		vmInfo.SystemMessage = vmStatusTmp.SystemMessage

		if cspResourceName != "" {
			// don't update VM info, if cspResourceName is empty
			UpdateVmInfo(nsId, mciId, vmInfo)
		}
	} else {
		log.Debug().Msgf("VM %s is already terminated, skipping status update", vmId)
	}

	return vmStatusTmp, nil
}

// GetMciVmStatus is func to Get MciVm Status with option to control CSP API fetch
func GetMciVmStatus(nsId string, mciId string, vmId string, fetchFromCSP bool) (*model.VmStatusInfo, error) {

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

	var vmStatusResponse model.VmStatusInfo

	if fetchFromCSP {
		// Fetch current status from CSP API
		vmStatusResponse, err = FetchVmStatus(nsId, mciId, vmId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return nil, err
		}
	} else {
		// Use cached status from database (faster response)
		vmObject, err := GetVmObject(nsId, mciId, vmId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return nil, err
		}

		// Convert VmInfo to VmStatusInfo
		vmStatusResponse = model.VmStatusInfo{
			Id:              vmObject.Id,
			Name:            vmObject.Name,
			CspResourceName: vmObject.CspResourceName,
			Status:          vmObject.Status,
			TargetStatus:    vmObject.TargetStatus,
			TargetAction:    vmObject.TargetAction,
			PublicIp:        vmObject.PublicIP,
			PrivateIp:       vmObject.PrivateIP,
			SSHPort:         vmObject.SSHPort,
			Location:        vmObject.Location,
			MonAgentStatus:  vmObject.MonAgentStatus,
			CreatedTime:     vmObject.CreatedTime,
			SystemMessage:   vmObject.SystemMessage,
		}
	}

	return &vmStatusResponse, nil
}

// GetMciVmCurrentStatus is func to Get MciVm Current Status from CSP API (real-time)
func GetMciVmCurrentStatus(nsId string, mciId string, vmId string) (*model.VmStatusInfo, error) {
	// Simply delegate to GetMciVmStatus with fetchFromCSP=true
	return GetMciVmStatus(nsId, mciId, vmId, true)
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
		_, err := HandleMciVmAction(nsId, mciId, vmId, model.ActionTerminate, false)
		if err != nil {
			log.Info().Msg(err.Error())
			return err
		}
		// for deletion, need to wait until termination is finished
		// Sleep for 5 seconds
		log.Info().Msg("Wait for VM termination in 5 seconds")
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
