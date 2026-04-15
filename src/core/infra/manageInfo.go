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

// Package infra is to manage multi-cloud infra
package infra

import (
	"context"
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
	"github.com/rs/zerolog/log"
)

var infraInfoMutex sync.Mutex

// [Infra and VM object information managemenet]

// ListInfraId is func to list Infra ID
func ListInfraId(nsId string) ([]string, error) {

	var infraList []string

	// Check Infra exists
	key := common.GenInfraKey(nsId, "", "")
	key += "/"

	keyValue, err := kvstore.GetKvList(key)

	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	for _, v := range keyValue {
		if strings.Contains(v.Key, "/infra/") {
			trimmedString := strings.TrimPrefix(v.Key, (key + "infra/"))
			// prevent malformed key (if key for infra id includes '/', the key does not represent Infra ID)
			if !strings.Contains(trimmedString, "/") {
				infraList = append(infraList, trimmedString)
			}
		}
	}

	return infraList, nil
}

// ListVmId is func to list VM IDs
func ListVmId(nsId string, infraId string) ([]string, error) {

	// err := common.CheckString(nsId)
	// if err != nil {
	// 	log.Error().Err(err).Msg("")
	// 	return nil, err
	// }

	// err = common.CheckString(infraId)
	// if err != nil {
	// 	log.Error().Err(err).Msg("")
	// 	return nil, err
	// }

	var vmList []string

	// Check Infra exists
	key := common.GenInfraKey(nsId, infraId, "")
	key += "/"

	_, _, err := kvstore.GetKv(key)
	if err != nil {
		log.Debug().Msg("[Not found] " + infraId)
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
func ListVmByLabel(nsId string, infraId string, labelKey string) ([]string, error) {
	// Construct the label selector
	labelSelector := labelKey + " exists" + "," + model.LabelNamespace + "=" + nsId + "," + model.LabelInfraId + "=" + infraId

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

// ListVmByFilter is func to get list VMs in a Infra by a filter consist of Key and Value
func ListVmByFilter(nsId string, infraId string, filterKey string, filterVal string) ([]string, error) {

	check, err := CheckInfra(nsId, infraId)
	if !check {
		err := fmt.Errorf("Not found the Infra: " + infraId + " from the NS: " + nsId)
		return nil, err
	}

	vmList, err := ListVmId(nsId, infraId)
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

	// Use existing ListInfraVmInfo function instead of individual GetVmObject calls
	vmInfoList, err := ListInfraVmInfo(nsId, infraId)
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

// ListVmByNodeGroup is func to get VM list with a NodeGroup label in a specified Infra
func ListVmByNodeGroup(nsId string, infraId string, groupId string) ([]string, error) {
	// NodeGroupId is the Key for NodeGroupId in model.VmInfo struct
	filterKey := "NodeGroupId"
	return ListVmByFilter(nsId, infraId, filterKey, groupId)
}

// GetNodeGroup is func to return list of NodeGroups in a given Infra
func GetNodeGroup(nsId string, infraId string, nodeGroupId string) (model.NodeGroupInfo, error) {
	nodeGroupInfo := model.NodeGroupInfo{}

	key := common.GenInfraNodeGroupKey(nsId, infraId, nodeGroupId)
	keyValue, _, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nodeGroupInfo, err
	}
	err = json.Unmarshal([]byte(keyValue.Value), &nodeGroupInfo)
	if err != nil {
		err = fmt.Errorf("failed to get nodeGroupInfo (Key: %s), message: failed to unmarshal", key)
		log.Error().Err(err).Msg("")
		return nodeGroupInfo, err
	}
	return nodeGroupInfo, nil
}

// ListNodeGroupId is func to return list of NodeGroups in a given Infra
func ListNodeGroupId(nsId string, infraId string) ([]string, error) {

	//log.Debug().Msg("[ListNodeGroupId]")
	key := common.GenInfraKey(nsId, infraId, "")
	key += "/"

	keyValue, err := kvstore.GetKvList(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	var nodeGroupList []string
	for _, v := range keyValue {
		if strings.Contains(v.Key, "/nodegroup/") {
			trimmedString := strings.TrimPrefix(v.Key, (key + "nodegroup/"))
			// prevent malformed key (if key for vm id includes '/', the key does not represent VM ID)
			if !strings.Contains(trimmedString, "/") {
				nodeGroupList = append(nodeGroupList, trimmedString)
			}
		}
	}
	return nodeGroupList, nil
}

// GetInfraInfo is func to return Infra information with the current status update
func GetInfraInfo(nsId string, infraId string) (*model.InfraInfo, error) {

	check, _ := CheckInfra(nsId, infraId)

	if !check {
		temp := &model.InfraInfo{}
		err := fmt.Errorf("The infra " + infraId + " does not exist.")
		return temp, err
	}

	infraObj, _, err := GetInfraObject(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	// common.PrintJsonPretty(infraObj)

	infraStatus, err := GetInfraStatus(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	// common.PrintJsonPretty(infraStatus)

	infraObj.Status = infraStatus.Status
	infraObj.StatusCount = infraStatus.StatusCount

	vmList, err := ListVmId(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	sort.Slice(infraObj.Vm, func(i, j int) bool {
		return infraObj.Vm[i].Id < infraObj.Vm[j].Id
	})

	for vmInfoIndex := range vmList {
		for vmStatusInfoIndex := range infraStatus.Vm {
			if infraObj.Vm[vmInfoIndex].Id == infraStatus.Vm[vmStatusInfoIndex].Id {
				infraObj.Vm[vmInfoIndex].Status = infraStatus.Vm[vmStatusInfoIndex].Status
				infraObj.Vm[vmInfoIndex].TargetStatus = infraStatus.Vm[vmStatusInfoIndex].TargetStatus
				infraObj.Vm[vmInfoIndex].TargetAction = infraStatus.Vm[vmStatusInfoIndex].TargetAction
				break
			}
		}
	}

	// add label info for VM
	for i := range infraObj.Vm {
		labelInfo, err := label.GetLabels(model.StrVM, infraObj.Vm[i].Uid)
		if err != nil {
			log.Error().Err(err).Msg("Cannot get the label info")
			return nil, err
		}
		infraObj.Vm[i].Label = labelInfo.Labels
	}

	// add label info
	labelInfo, err := label.GetLabels(model.StrInfra, infraObj.Uid)
	if err != nil {
		log.Error().Err(err).Msg("Cannot get the label info")
		return nil, err
	}
	infraObj.Label = labelInfo.Labels

	return &infraObj, nil
}

// filterOutSystemLabels returns a copy of labels excluding system-managed keys (prefixed with "sys.").
func filterOutSystemLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return labels
	}
	filtered := make(map[string]string)
	for k, v := range labels {
		if !strings.HasPrefix(k, model.LabelSystemPrefix) {
			filtered[k] = v
		}
	}
	return filtered
}

// ExtractInfraDynamicReqFromInfraInfo reconstructs an InfraDynamicReq from a running Infra's info.
// This returns a dynamic creation request (resources like vNet, subnet, SG, sshKey are auto-created)
// so that users can easily clone or recreate a similar Infra configuration.
func ExtractInfraDynamicReqFromInfraInfo(nsId string, infraId string) (*model.InfraDynamicReq, error) {

	infraInfo, err := GetInfraInfo(nsId, infraId)
	if err != nil {
		return nil, err
	}

	if len(infraInfo.Vm) == 0 {
		return nil, fmt.Errorf("Infra '%s' has no VMs to extract configuration from", infraId)
	}

	// Group VMs by NodeGroupId to reconstruct NodeGroup requests
	nodeGroupMap := make(map[string][]model.VmInfo)
	var nodeGroupOrder []string
	for _, vm := range infraInfo.Vm {
		sgId := vm.NodeGroupId
		if sgId == "" {
			sgId = vm.Id // fallback: treat each VM as its own group
		}
		if _, exists := nodeGroupMap[sgId]; !exists {
			nodeGroupOrder = append(nodeGroupOrder, sgId)
		}
		nodeGroupMap[sgId] = append(nodeGroupMap[sgId], vm)
	}

	var nodeGroups []model.CreateNodeGroupDynamicReq
	for _, sgId := range nodeGroupOrder {
		vms := nodeGroupMap[sgId]
		// Use the first VM in each nodegroup as the representative spec
		rep := vms[0]
		sg := model.CreateNodeGroupDynamicReq{
			Name:           sgId,
			NodeGroupSize:  len(vms),
			Label:          filterOutSystemLabels(rep.Label),
			Description:    rep.Description,
			ConnectionName: rep.ConnectionName,
			SpecId:         rep.SpecId,
			ImageId:        rep.ImageId,
			RootDiskType:   rep.RootDiskType,
			RootDiskSize:   rep.RootDiskSize,
			Zone:           rep.Region.Zone,
		}
		nodeGroups = append(nodeGroups, sg)
	}

	infraDynamicReq := &model.InfraDynamicReq{
		Name:            infraInfo.Name,
		InstallMonAgent: infraInfo.InstallMonAgent,
		Label:           filterOutSystemLabels(infraInfo.Label),
		SystemLabel:     infraInfo.SystemLabel,
		Description:     infraInfo.Description,
		NodeGroups:      nodeGroups,
		PostCommand:     infraInfo.PostCommand,
	}

	return infraDynamicReq, nil
}

// GetInfraAccessInfo is func to retrieve Infra Access information
func GetInfraAccessInfo(nsId string, infraId string, option string) (*model.InfraAccessInfo, error) {

	output := &model.InfraAccessInfo{}
	temp := &model.InfraAccessInfo{}
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = common.CheckString(infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return temp, err
	}
	check, _ := CheckInfra(nsId, infraId)

	if !check {
		err := fmt.Errorf("The infra " + infraId + " does not exist.")
		return temp, err
	}

	// Get Infra information to check if it's being terminated
	infraInfo, err := GetInfraInfo(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("failed to get Infra info")
		return temp, err
	}

	// Check if Infra is being terminated or terminate action
	if strings.EqualFold(infraInfo.Status, model.StatusTerminated) ||
		infraInfo.TargetAction == model.ActionTerminate {
		err := fmt.Errorf("Infra %s is currently being terminated or in terminate action (Status: %s, TargetAction: %s)",
			infraId, infraInfo.Status, infraInfo.TargetAction)
		log.Info().Msg(err.Error())
		return temp, err
	}

	output.InfraId = infraId

	mcNlbAccess, err := GetMcNlbAccess(nsId, infraId)
	if err == nil {
		output.InfraNlbListener = mcNlbAccess
	}

	nodeGroupList, err := ListNodeGroupId(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return temp, err
	}
	// TODO: make in parallel

	for _, groupId := range nodeGroupList {
		nodeGroupAccessInfo := model.InfraNodeGroupAccessInfo{}
		nodeGroupAccessInfo.NodeGroupId = groupId
		nlb, err := GetNLB(nsId, infraId, groupId)
		if err == nil {
			nodeGroupAccessInfo.NlbListener = &nlb.Listener
		}
		vmList, err := ListVmByNodeGroup(nsId, infraId, groupId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return temp, err
		}
		var wg sync.WaitGroup
		chanResults := make(chan model.InfraVmAccessInfo)

		for _, vmId := range vmList {
			// Check if VM is terminated before processing
			vmObject, err := GetVmObject(nsId, infraId, vmId)
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
			go func(nsId string, infraId string, vmId string, option string, chanResults chan model.InfraVmAccessInfo) {
				defer wg.Done()
				common.RandomSleep(0, len(vmList)/2*1000)
				vmInfo, err := GetVmCurrentPublicIp(nsId, infraId, vmId)

				vmAccessInfo := model.InfraVmAccessInfo{}
				if err != nil {
					log.Info().Err(err).Msg("")
					vmAccessInfo.PublicIP = ""
					vmAccessInfo.PrivateIP = ""
					vmAccessInfo.SSHPort = 0
				} else {
					vmAccessInfo.PublicIP = vmInfo.PublicIp
					vmAccessInfo.PrivateIP = vmInfo.PrivateIp
					vmAccessInfo.SSHPort = vmInfo.SSHPort
				}
				vmAccessInfo.VmId = vmId

				vmObject, err := GetVmObject(nsId, infraId, vmId)
				if err != nil {
					log.Info().Err(err).Msg("")
				} else {
					vmAccessInfo.ConnectionConfig = vmObject.ConnectionConfig
				}

				_, verifiedUserName, privateKey, err := GetVmSshKey(nsId, infraId, vmId)
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
			}(nsId, infraId, vmId, option, chanResults)
		}
		go func() {
			wg.Wait()
			close(chanResults)
		}()
		for result := range chanResults {
			nodeGroupAccessInfo.InfraVmAccessInfo = append(nodeGroupAccessInfo.InfraVmAccessInfo, result)
		}

		output.InfraNodeGroupAccessInfo = append(output.InfraNodeGroupAccessInfo, nodeGroupAccessInfo)
	}

	return output, nil
}

// GetInfraVmAccessInfo is func to retrieve Infra Access information
func GetInfraVmAccessInfo(nsId string, infraId string, vmId string, option string) (*model.InfraVmAccessInfo, error) {

	output := &model.InfraVmAccessInfo{}

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return output, err
	}

	err = common.CheckString(infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return output, err
	}
	check, _ := CheckInfra(nsId, infraId)

	if !check {
		err := fmt.Errorf("The infra %s does not exist.", infraId)
		return output, err
	}

	output.VmId = vmId

	vmInfo, err := GetVmCurrentPublicIp(nsId, infraId, vmId)

	vmAccessInfo := &model.InfraVmAccessInfo{}
	if err != nil {
		log.Info().Err(err).Msg("")
		return output, err
	} else {
		vmAccessInfo.PublicIP = vmInfo.PublicIp
		vmAccessInfo.PrivateIP = vmInfo.PrivateIp
		vmAccessInfo.SSHPort = vmInfo.SSHPort
	}
	vmAccessInfo.VmId = vmId

	vmObject, err := GetVmObject(nsId, infraId, vmId)
	if err != nil {
		log.Info().Err(err).Msg("")
		return output, err
	} else {
		vmAccessInfo.ConnectionConfig = vmObject.ConnectionConfig
	}

	_, verifiedUserName, privateKey, err := GetVmSshKey(nsId, infraId, vmId)
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

// ListInfraInfo is func to get all Infra objects
func ListInfraInfo(nsId string, option string) ([]model.InfraInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	Infra := []model.InfraInfo{}

	infraList, err := ListInfraId(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	for _, v := range infraList {

		infraTmp, err := GetInfraInfo(nsId, v)
		if err != nil {
			log.Error().Err(err).Msg("")
			return nil, err
		}

		Infra = append(Infra, *infraTmp)
	}

	return Infra, nil
}

// ListInfraVmInfo is func to Get all VM Info objects in Infra
func ListInfraVmInfo(nsId string, infraId string) ([]model.VmInfo, error) {

	// Check if Infra exists
	check, err := CheckInfra(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msgf("Cannot check Infra %s exist", infraId)
		return nil, err
	}
	if !check {
		err := fmt.Errorf("Infra %s does not exist", infraId)
		return nil, err
	}

	// Get VM ID list using existing function
	vmIdList, err := ListVmId(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to list VM IDs for Infra %s", infraId)
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
			vmKey := common.GenInfraKey(nsId, infraId, vmId)
			_, exists, err := kvstore.GetKv(vmKey)
			if err != nil || !exists {
				// VM might be deleted by concurrent operations (e.g., DelInfra)
				// This is normal during Infra deletion process, so use Debug level
				log.Debug().Msgf("VM object not found for vmId: %s (possibly deleted concurrently)", vmId)
				return // Skip this VM
			}

			vmInfo, err := GetVmObject(nsId, infraId, vmId)
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

// GetInfraObject is func to retrieve Infra object from database (no current status update)
func GetInfraObject(nsId string, infraId string) (model.InfraInfo, bool, error) {
	//log.Debug().Msg("[GetInfraObject]" + infraId)
	key := common.GenInfraKey(nsId, infraId, "")
	keyValue, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.InfraInfo{}, false, err
	}
	if !exists {
		log.Warn().Msgf("no Infra found (ID: %s)", key)
		return model.InfraInfo{}, false, err
	}

	infraTmp := model.InfraInfo{}
	json.Unmarshal([]byte(keyValue.Value), &infraTmp)

	// Use existing ListInfraVmInfo function instead of manually iterating through VMs
	vmInfoList, err := ListInfraVmInfo(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.InfraInfo{}, false, err
	}

	infraTmp.Vm = vmInfoList

	return infraTmp, true, nil
}

// GetVmObject is func to get VM object
func GetVmObject(nsId string, infraId string, vmId string) (model.VmInfo, error) {

	vmTmp := model.VmInfo{}
	key := common.GenInfraKey(nsId, infraId, vmId)
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

// ConvertVmInfoToVmStatusInfo converts VmInfo to VmStatusInfo for Infra status operations
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

// ConvertVmInfoListToVmStatusInfoList converts a slice of VmInfo to VmStatusInfo for Infra status operations
func ConvertVmInfoListToVmStatusInfoList(vmInfoList []model.VmInfo) []model.VmStatusInfo {
	vmStatusInfoList := make([]model.VmStatusInfo, len(vmInfoList))
	for i, vmInfo := range vmInfoList {
		vmStatusInfoList[i] = ConvertVmInfoToVmStatusInfo(vmInfo)
	}
	return vmStatusInfoList
}

// ensureVmStatusInfoComplete ensures all VMs from VmInfo are represented in InfraStatus.Vm
// This handles cases where VM status fetch might have failed or VM is newly created
// ConvertInfraInfoToInfraStatusInfo converts InfraInfo to InfraStatusInfo (partial conversion for basic fields)
func ConvertInfraInfoToInfraStatusInfo(infraInfo model.InfraInfo) model.InfraStatusInfo {
	return model.InfraStatusInfo{
		Id:              infraInfo.Id,
		Name:            infraInfo.Name,
		Status:          infraInfo.Status,
		StatusCount:     infraInfo.StatusCount,
		TargetStatus:    infraInfo.TargetStatus,
		TargetAction:    infraInfo.TargetAction,
		InstallMonAgent: infraInfo.InstallMonAgent,
		Label:           infraInfo.Label,
		SystemLabel:     infraInfo.SystemLabel,
		Vm:              ConvertVmInfoListToVmStatusInfoList(infraInfo.Vm),
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
func GetVmIdNameInDetail(nsId string, infraId string, vmId string) (*model.IdNameInDetailInfo, error) {
	key := common.GenInfraKey(nsId, infraId, vmId)
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

	client := clientManager.NewHttpClient()
	url := fmt.Sprintf("%s/cspresourcename/%s", model.SpiderRestUrl, idDetails.IdInSp)
	method := "GET"
	client.SetTimeout(5 * time.Minute)

	_, err = clientManager.ExecuteHttpRequest(
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

// [Infra and VM status management]

// GetInfraStatus is func to Get Infra Status
func GetInfraStatus(nsId string, infraId string) (*model.InfraStatusInfo, error) {

	// err := common.CheckString(nsId)
	// if err != nil {
	// 	log.Error().Err(err).Msg("")
	// 	return &model.InfraStatusInfo{}, err
	// }

	// err = common.CheckString(infraId)
	// if err != nil {
	// 	log.Error().Err(err).Msg("")
	// 	return &model.InfraStatusInfo{}, err
	// }

	key := common.GenInfraKey(nsId, infraId, "")

	keyValue, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return &model.InfraStatusInfo{}, err
	}
	if !exists {
		err := fmt.Errorf("%s", "Not found ["+key+"]")
		log.Error().Err(err).Msg("")
		return &model.InfraStatusInfo{}, err
	}

	infraStatus := model.InfraStatusInfo{}
	json.Unmarshal([]byte(keyValue.Value), &infraStatus)

	infraTmp := model.InfraInfo{}
	json.Unmarshal([]byte(keyValue.Value), &infraTmp)

	vmList, err := ListVmId(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return &model.InfraStatusInfo{}, err
	}
	if len(vmList) == 0 {
		// Infra has no VMs - check if it's in provisioning phase or truly empty
		currentStatus := infraTmp.Status
		if strings.Contains(currentStatus, model.StatusPreparing) || strings.Contains(currentStatus, model.StatusPrepared) ||
			strings.Contains(currentStatus, model.StatusCreating) || strings.Contains(currentStatus, model.StatusFailed) {
			// Infra is in provisioning phase or failed - keep current status
			infraStatus.Status = currentStatus
		} else {
			// Infra was already running/completed but now has no VMs - set to Empty
			infraStatus.Status = model.StatusEmpty
		}
		infraStatus.StatusCount = model.StatusCountInfo{}
		infraStatus.Vm = []model.VmStatusInfo{}
		return &infraStatus, nil
	}

	// Fetch VM statuses with rate limiting by CSP and region
	vmStatusList, err := fetchVmStatusesWithRateLimiting(nsId, infraId, vmList)
	if err != nil {
		log.Error().Err(err).Msg("")
		return &model.InfraStatusInfo{}, err
	}
	// log.Debug().Msgf("Fetched %d VM statuses for Infra %s", len(vmStatusList), infraId)
	// log.Debug().Msgf("VM Status List: %+v", vmStatusList)

	// Copy results to infraStatus
	infraStatus.Vm = vmStatusList

	vmInfos, err := ListInfraVmInfo(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return &model.InfraStatusInfo{}, err
	}

	// If VM status fetch didn't populate all VMs, use VmInfo as fallback
	if len(infraStatus.Vm) == 0 && len(vmInfos) > 0 {
		// log.Debug().Msgf("No VM status info found, converting from VmInfo for Infra: %s", infraId)
		infraStatus.Vm = ConvertVmInfoListToVmStatusInfoList(vmInfos)
	}

	for _, v := range vmInfos {
		if strings.EqualFold(v.Status, model.StatusRunning) {
			infraStatus.MasterVmId = v.Id
			infraStatus.MasterIp = v.PublicIP
			infraStatus.MasterSSHPort = v.SSHPort
			break
		}
	}

	sort.Slice(infraStatus.Vm, func(i, j int) bool {
		return infraStatus.Vm[i].Id < infraStatus.Vm[j].Id
	})

	statusFlag := []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	statusFlagStr := []string{model.StatusFailed, model.StatusSuspended, model.StatusRunning, model.StatusTerminated, model.StatusCreating, model.StatusSuspending, model.StatusResuming, model.StatusRebooting, model.StatusTerminating, model.StatusRegistering, model.StatusUndefined}
	for _, v := range infraStatus.Vm {

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
		case model.StatusRegistering:
			statusFlag[9]++
		default:
			statusFlag[10]++
			log.Warn().Msgf("Undefined status (%s) found in VM %s of Infra %s", v.Status, v.Id, infraId)
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
	// During Infra creation, len(vmList) might be smaller than len(infraStatus.Vm) due to timing issues
	actualVmCount := len(vmList)
	statusVmCount := len(infraStatus.Vm)
	vmInfoCount := len(vmInfos)

	// Check if Infra is still being created/registered to use more stable VM count calculation
	isCreating := strings.Contains(infraTmp.Status, model.StatusCreating) ||
		strings.Contains(infraTmp.Status, model.StatusRegistering) ||
		strings.Contains(infraTmp.TargetAction, model.ActionCreate) ||
		strings.Contains(infraTmp.TargetStatus, model.StatusRunning)

	// Check if Infra is in a stable state (all VMs have same stable status)
	isStableState := tmpMax == statusVmCount && tmpMax > 0
	// stableStatusName := ""
	// if isStableState && tmpMaxIndex < len(statusFlagStr) {
	// 	stableStatusName = statusFlagStr[tmpMaxIndex]
	// }

	var numVm int
	if isCreating {
		// During creation, use the larger of the two counts to avoid showing decreasing VM counts
		numVm = actualVmCount
		if statusVmCount > actualVmCount {
			numVm = statusVmCount
		}
		// Additionally, ensure we don't show a VM count smaller than the previous maximum
		if numVm < infraStatus.StatusCount.CountTotal && infraStatus.StatusCount.CountTotal > 0 {
			numVm = infraStatus.StatusCount.CountTotal
		}

		// If we still have inconsistent counts, use the Infra's stored VM information as fallback
		if len(infraTmp.Vm) > numVm {
			numVm = len(infraTmp.Vm)
		}

		// log.Debug().Msgf("Infra %s is creating: using stable VM count (%d) - actual: %d, status: %d, previous: %d, stored: %d",
		// 	infraId, numVm, actualVmCount, statusVmCount, infraStatus.StatusCount.CountTotal, len(infraTmp.Vm))
	} else if isStableState {
		// For stable Infra states (all VMs in same state), use the most reliable source to avoid count fluctuation
		// This applies to Terminated, Suspended, Failed, Running, etc.
		// Use the maximum of available counts, prioritizing vmInfos as they are stored persistently
		numVm = vmInfoCount
		if actualVmCount > numVm {
			numVm = actualVmCount
		}
		if len(infraTmp.Vm) > numVm {
			numVm = len(infraTmp.Vm)
		}
		// Ensure we don't show a count smaller than the actual VMs found in dominant status
		if tmpMax > numVm {
			numVm = tmpMax
		}

		// log.Debug().Msgf("Infra %s is in stable state (%s): using stable VM count (%d) - actual: %d, status: %d, vmInfos: %d, stored: %d, dominant: %d",
		// 	infraId, stableStatusName, numVm, actualVmCount, statusVmCount, vmInfoCount, len(infraTmp.Vm), tmpMax)
	} else {
		// Infra creation completed, use actual VM count from status
		numVm = statusVmCount
		// log.Debug().Msgf("Infra %s creation completed: using status VM count (%d)", infraId, numVm)
	}

	//numUnNormalStatus := statusFlag[0] + statusFlag[9]
	//numNormalStatus := numVm - numUnNormalStatus
	runningStatus := statusFlag[2]

	proportionStr := ":" + strconv.Itoa(tmpMax) + " (R:" + strconv.Itoa(runningStatus) + "/" + strconv.Itoa(numVm) + ")"
	if tmpMax == numVm {
		infraStatus.Status = statusFlagStr[tmpMaxIndex] + proportionStr
	} else if tmpMax < numVm {
		infraStatus.Status = "Partial-" + statusFlagStr[tmpMaxIndex] + proportionStr
	} else {
		infraStatus.Status = statusFlagStr[9] + proportionStr
	}
	// // for representing Failed status in front.

	// proportionStr = ":" + strconv.Itoa(statusFlag[0]) + " (R:" + strconv.Itoa(runningStatus) + "/" + strconv.Itoa(numVm) + ")"
	// if statusFlag[0] > 0 {
	// 	infraStatus.Status = "Partial-" + statusFlagStr[0] + proportionStr
	// 	if statusFlag[0] == numVm {
	// 		infraStatus.Status = statusFlagStr[0] + proportionStr
	// 	}
	// }

	// proportionStr = "-(" + strconv.Itoa(statusFlag[9]) + "/" + strconv.Itoa(numVm) + ")"
	// if statusFlag[9] > 0 {
	// 	infraStatus.Status = statusFlagStr[9] + proportionStr
	// }

	// Set infraStatus.StatusCount
	infraStatus.StatusCount.CountTotal = numVm
	infraStatus.StatusCount.CountFailed = statusFlag[0]
	infraStatus.StatusCount.CountSuspended = statusFlag[1]
	infraStatus.StatusCount.CountRunning = statusFlag[2]
	infraStatus.StatusCount.CountTerminated = statusFlag[3]
	infraStatus.StatusCount.CountCreating = statusFlag[4]
	infraStatus.StatusCount.CountSuspending = statusFlag[5]
	infraStatus.StatusCount.CountResuming = statusFlag[6]
	infraStatus.StatusCount.CountRebooting = statusFlag[7]
	infraStatus.StatusCount.CountTerminating = statusFlag[8]
	infraStatus.StatusCount.CountRegistering = statusFlag[9]
	infraStatus.StatusCount.CountUndefined = statusFlag[10]

	// Recovery/fallback handling for TargetAction completion
	// Primary completion should happen in actual control actions (control.go, provisioning.go)
	// This serves as a safety net for cases where the primary completion was missed
	isDone := true
	pendingVmsCount := 0

	// Check Infra target action to determine completion criteria
	infraTargetAction := infraTmp.TargetAction

	// Only perform recovery completion if TargetAction is not already Complete
	if infraTargetAction != model.ActionComplete && infraTargetAction != "" {
		for _, v := range infraStatus.Vm {
			// Check completion based on action type
			switch infraTargetAction {
			case model.ActionCreate:
				// For Create action, completion means all VMs reach final states (Running/Failed/Terminated/Suspended)
				// VM is considered pending if it's still in transitional states (Creating/Registering/Undefined/empty)
				// Failed state is considered a final state - provisioning attempt was completed even if unsuccessful
				if v.Status == model.StatusCreating || v.Status == model.StatusRegistering || v.Status == model.StatusUndefined || v.Status == "" {
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
		// log.Debug().Msgf("Infra %s %s recovery completion check: %d VMs total, %d pending, isDone=%t",
		// 	infraId, infraTargetAction, len(infraStatus.Vm), pendingVmsCount, isDone)

		if isDone {
			log.Warn().Msgf("Infra %s action %s completed via RECOVERY PATH (primary completion in control.go/provisioning.go was missed) - VM states: %d total, %d pending",
				infraId, infraTargetAction, len(infraStatus.Vm), pendingVmsCount)

			// Add more detailed logging for debugging
			statusBreakdown := make(map[string]int)
			for _, v := range infraStatus.Vm {
				statusBreakdown[v.Status]++
			}
			// log.Debug().Msgf("Infra %s recovery completion - VM status breakdown: %+v", infraId, statusBreakdown)

			// Check if all VMs are in failed state
			// If there are no VMs, consider it as all VMs failed for creation context
			allVmsFailed := len(infraStatus.Vm) == 0
			if len(infraStatus.Vm) > 0 {
				allVmsFailed = true
				for _, v := range infraStatus.Vm {
					if v.Status != model.StatusFailed && v.Status != model.StatusTerminated {
						allVmsFailed = false
						break
					}
				}
			}

			if allVmsFailed && infraTargetAction == model.ActionCreate {
				// All VMs failed during creation - mark Infra as Failed
				log.Error().Msgf("Infra %s: All VMs failed during creation - setting Infra status to Failed", infraId)
				infraStatus.TargetAction = model.ActionComplete
				infraStatus.TargetStatus = model.StatusComplete // Target was to complete the creation process
				infraStatus.Status = model.StatusFailed         // Actual status is Failed due to VM failures
				infraTmp.TargetAction = model.ActionComplete
				infraTmp.TargetStatus = model.StatusComplete // Target was to complete the creation process
				infraTmp.Status = model.StatusFailed         // Actual status is Failed due to VM failures
			} else {
				// Normal completion
				infraStatus.TargetAction = model.ActionComplete
				infraStatus.TargetStatus = model.StatusComplete
				infraTmp.TargetAction = model.ActionComplete
				infraTmp.TargetStatus = model.StatusComplete
			}

			infraTmp.StatusCount = infraStatus.StatusCount
			UpdateInfraInfo(nsId, infraTmp)
		}
	}

	return &infraStatus, nil

	//need to change status

}

// ListInfraStatus is func to get Infra status all
func ListInfraStatus(nsId string) ([]model.InfraStatusInfo, error) {

	//infraStatuslist := []model.InfraStatusInfo{}
	infraList, err := ListInfraId(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return []model.InfraStatusInfo{}, err
	}

	var wg sync.WaitGroup
	chanResults := make(chan model.InfraStatusInfo)
	var infraStatuslist []model.InfraStatusInfo

	for _, infraId := range infraList {
		wg.Add(1)
		go func(nsId string, infraId string, chanResults chan model.InfraStatusInfo) {
			defer wg.Done()
			infraStatus, err := GetInfraStatus(nsId, infraId)
			if err != nil {
				log.Error().Err(err).Msg("")
			}
			chanResults <- *infraStatus
		}(nsId, infraId, chanResults)
	}

	go func() {
		wg.Wait()
		close(chanResults)
	}()
	for result := range chanResults {
		infraStatuslist = append(infraStatuslist, result)
	}

	return infraStatuslist, nil

	//need to change status

}

// GetVmCurrentPublicIp is func to get VM public IP
func GetVmCurrentPublicIp(nsId string, infraId string, vmId string) (model.VmStatusInfo, error) {
	errorInfo := model.VmStatusInfo{}
	errorInfo.Status = model.StatusFailed

	temp, err := GetVmObject(nsId, infraId, vmId) // to check if the VM exists
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

	client := clientManager.NewHttpClient()
	client.SetTimeout(2 * time.Minute)
	url := model.SpiderRestUrl + "/vm/" + cspResourceName
	method := "GET"
	requestBody := model.SpiderConnectionName{}
	requestBody.ConnectionName = temp.ConnectionName
	callResult := statusResponse{}

	_, err = clientManager.ExecuteHttpRequest(
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
	// Convert port string from Spider to int
	if portStr, err := TrimIP(callResult.SSHAccessPoint); err == nil {
		if port, err := strconv.Atoi(portStr); err == nil {
			vmStatusTmp.SSHPort = port
		}
	}

	return vmStatusTmp, nil
}

// GetVmIp is func to get VM IP to return PublicIP, PrivateIP, SSHPort
func GetVmIp(nsId string, infraId string, vmId string) (string, string, int, error) {

	vmObject, err := GetVmObject(nsId, infraId, vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", "", 0, err
	}

	return vmObject.PublicIP, vmObject.PrivateIP, vmObject.SSHPort, nil
}

// GetVmSpecId is func to get VM SpecId
func GetVmSpecId(nsId string, infraId string, vmId string) string {

	var content struct {
		SpecId string `json:"specId"`
	}

	log.Debug().Msg("[getVmSpecID]" + vmId)
	key := common.GenInfraKey(nsId, infraId, vmId)

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

// getRateLimitsForCSP returns rate limiting configuration for VM status fetching
// for a specific CSP, using the centralized configuration in csp package.
func getRateLimitsForCSP(cspName string) (int, int) {
	config := csp.GetRateLimitConfig(cspName)
	return config.MaxConcurrentRegionsForStatus, config.MaxVMsPerRegionForStatus
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
func fetchVmStatusesWithRateLimiting(nsId, infraId string, vmList []string) ([]model.VmStatusInfo, error) {
	if len(vmList) == 0 {
		return []model.VmStatusInfo{}, nil
	}

	// Step 1: Group VMs by CSP and region
	vmGroups := make(map[string]map[string][]string) // CSP -> Region -> VmIds
	vmGroupInfos := make(map[string]VmGroupInfo)     // VmId -> GroupInfo

	for _, vmId := range vmList {
		vmInfo, err := GetVmObject(nsId, infraId, vmId)
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
							vmStatusTmp, err := FetchVmStatus(nsId, infraId, vmId)
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

	// // Summary logging
	// cspCount := len(vmGroups)
	// totalRegions := 0
	// for _, regions := range vmGroups {
	// 	totalRegions += len(regions)
	// }

	// log.Debug().Msgf("Rate-limited VM status fetch completed: %d CSPs, %d regions, %d VMs processed",
	// 	cspCount, totalRegions, len(allVmStatuses))
	return allVmStatuses, nil
}

// // FetchVmStatusAsync is func to get VM status async
// func FetchVmStatusAsync(wg *sync.WaitGroup, nsId string, infraId string, vmId string, results *model.InfraStatusInfo) error {
// 	defer wg.Done() //goroutine sync done

// 	if nsId != "" && infraId != "" && vmId != "" {
// 		vmStatusTmp, err := FetchVmStatus(nsId, infraId, vmId)
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
func FetchVmStatus(nsId string, infraId string, vmId string) (model.VmStatusInfo, error) {

	statusInfo := model.VmStatusInfo{}

	vmInfo, err := GetVmObject(nsId, infraId, vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return statusInfo, err
	}

	// log.Debug().Msgf("[FetchVmStatus] VM %s - Initial state from DB: Status=%s, TargetAction=%s, TargetStatus=%s, ConnectionName=%s",
	// 	vmId, vmInfo.Status, vmInfo.TargetAction, vmInfo.TargetStatus, vmInfo.ConnectionName)

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

	// Skip CSP API call for stable states ONLY if there's no active action in progress
	// If TargetAction is set (Resume, Reboot, etc.), we must fetch from CSP to track progress
	if stableStates[vmInfo.Status] && vmInfo.TargetAction == model.ActionComplete {
		shouldSkipCSPCall = true
	}

	// Skip CSP API call if cspResourceName is empty (VM not properly created)
	if vmInfo.CspResourceName == "" && vmInfo.TargetAction != model.ActionCreate {
		shouldSkipCSPCall = true
	}

	if shouldSkipCSPCall {
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
		client := clientManager.NewHttpClient()
		url := model.SpiderRestUrl + "/vmstatus/" + cspResourceName
		method := "GET"
		client.SetTimeout(60 * time.Second)

		type VMStatusReqInfo struct {
			ConnectionName string
		}
		requestBody := VMStatusReqInfo{}
		requestBody.ConnectionName = vmInfo.ConnectionName

		// log.Debug().Msgf("[FetchVmStatus] VM %s: Calling CB-Spider API - URL: %s, ConnectionName: %s",
		// 	vmId, url, vmInfo.ConnectionName)

		// Retry to get right VM status from cb-spider. Sometimes cb-spider returns not approriate status.
		retrycheck := 2
		for range retrycheck {
			statusInfo.Status = model.StatusFailed
			_, err := clientManager.ExecuteHttpRequest(
				client,
				method,
				url,
				nil,
				clientManager.SetUseBody(requestBody),
				&requestBody,
				&callResult,
				clientManager.MediumDuration,
			)

			// log.Debug().Msgf("[FetchVmStatus] VM %s: CB-Spider response (attempt %d/%d) - Status: %s, Error: %v",
			// 	vmId, i+1, retrycheck, callResult.Status, err)

			if err != nil {
				statusInfo.SystemMessage = err.Error()

				// check if VM is already Terminated
				if vmInfo.Status == model.StatusTerminated {
					// VM was already terminated, maintain the status instead of marking as Undefined
					// log.Debug().Msgf("VM %s does not exist in CSP but is already Terminated, maintaining status", vmId)
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

	// log.Debug().Msgf("[FetchVmStatus] VM %s: Raw NativeStatus from CSP: %s", vmId, nativeStatus)

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
		// log.Debug().Msgf("[FetchVmStatus] VM %s: NativeStatus '%s' is not valid, setting to Undefined", vmId, nativeStatus)
		callResult.Status = model.StatusUndefined
	}

	vmInfo, err = GetVmObject(nsId, infraId, vmId)
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

	// log.Debug().Msgf("[FetchVmStatus] VM %s: Before TargetAction correction - Status=%s, NativeStatus=%s, TargetAction=%s, TargetStatus=%s",
	// 	vmId, vmStatusTmp.Status, vmStatusTmp.NativeStatus, vmStatusTmp.TargetAction, vmStatusTmp.TargetStatus)

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
		// NCP may return Creating status during Resume operation instead of Resuming status.
		if strings.EqualFold(callResult.Status, model.StatusCreating) {
			log.Debug().Msgf("[FetchVmStatus] VM %s: CSP returned Creating during Resume action, correcting to Resuming", vmId)
			callResult.Status = model.StatusResuming
		}
		// Some CSPs (e.g., KT Cloud) may return Suspended status during Resume operation
		// instead of returning Resuming status. Correct it to Resuming.
		if strings.EqualFold(callResult.Status, model.StatusSuspended) {
			log.Debug().Msgf("[FetchVmStatus] VM %s: CSP returned Suspended during Resume action, correcting to Resuming", vmId)
			callResult.Status = model.StatusResuming
		}
	}
	// Some CSPs may return Running or Resuming status during Suspend operation instead of Suspending status.
	if strings.EqualFold(vmStatusTmp.TargetAction, model.ActionSuspend) {
		if strings.EqualFold(callResult.Status, model.StatusUndefined) {
			callResult.Status = model.StatusSuspending
		}
		if strings.EqualFold(callResult.Status, model.StatusRunning) {
			log.Debug().Msgf("[FetchVmStatus] VM %s: CSP returned Running during Suspend action, correcting to Suspending", vmId)
			callResult.Status = model.StatusSuspending
		}
		// Tencent may temporarily return Resuming status during Suspend operation
		if strings.EqualFold(callResult.Status, model.StatusResuming) {
			log.Debug().Msgf("[FetchVmStatus] VM %s: CSP returned Resuming during Suspend action, correcting to Suspending", vmId)
			callResult.Status = model.StatusSuspending
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

	// Log status change if status actually changed
	previousStatus := vmStatusTmp.Status
	vmStatusTmp.Status = callResult.Status
	if previousStatus != vmStatusTmp.Status {
		log.Debug().Msgf("[FetchVmStatus] VM %s: Status changed - %s -> %s (NativeStatus: %s, TargetAction: %s)",
			vmId, previousStatus, vmStatusTmp.Status, vmStatusTmp.NativeStatus, vmStatusTmp.TargetAction)
	}

	// TODO: Alibaba Undefined status error is not resolved yet.
	// (After Terminate action. "status": "Undefined", "targetStatus": "None", "targetAction": "None")

	//if TargetStatus == CurrentStatus, record to finialize the control operation
	if vmStatusTmp.TargetStatus == vmStatusTmp.Status {
		if vmStatusTmp.TargetStatus != model.StatusTerminated {
			log.Debug().Msgf("[FetchVmStatus] VM %s: Action completed - TargetStatus(%s) reached",
				vmId, vmStatusTmp.TargetStatus)
			vmStatusTmp.SystemMessage = vmStatusTmp.TargetStatus + "==" + vmStatusTmp.Status
			vmStatusTmp.TargetStatus = model.StatusComplete
			vmStatusTmp.TargetAction = model.ActionComplete

			//Get current public IP when status has been changed.
			vmInfoTmp, err := GetVmCurrentPublicIp(nsId, infraId, vmInfo.Id)
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
	originalVmInfo, _ := GetVmObject(nsId, infraId, vmId)
	if originalVmInfo.Status != model.StatusTerminated {
		vmInfo.Status = vmStatusTmp.Status
		vmInfo.TargetAction = vmStatusTmp.TargetAction
		vmInfo.TargetStatus = vmStatusTmp.TargetStatus
		vmInfo.SystemMessage = vmStatusTmp.SystemMessage

		if cspResourceName != "" {
			// don't update VM info, if cspResourceName is empty
			UpdateVmInfo(nsId, infraId, vmInfo)
		}
	}
	// else: VM is already terminated, skip status update

	return vmStatusTmp, nil
}

// GetInfraVmStatus is func to Get InfraVm Status with option to control CSP API fetch
func GetInfraVmStatus(nsId string, infraId string, vmId string, fetchFromCSP bool) (*model.VmStatusInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := &model.VmStatusInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = common.CheckString(infraId)
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

	check, _ := CheckVm(nsId, infraId, vmId)

	if !check {
		temp := &model.VmStatusInfo{}
		err := fmt.Errorf("The vm " + vmId + " does not exist.")
		return temp, err
	}

	var vmStatusResponse model.VmStatusInfo

	if fetchFromCSP {
		// Fetch current status from CSP API
		vmStatusResponse, err = FetchVmStatus(nsId, infraId, vmId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return nil, err
		}
	} else {
		// Use cached status from database (faster response)
		vmObject, err := GetVmObject(nsId, infraId, vmId)
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

// GetInfraVmCurrentStatus is func to Get InfraVm Current Status from CSP API (real-time)
func GetInfraVmCurrentStatus(nsId string, infraId string, vmId string) (*model.VmStatusInfo, error) {
	// Simply delegate to GetInfraVmStatus with fetchFromCSP=true
	return GetInfraVmStatus(nsId, infraId, vmId, true)
}

// [Update Infra and VM object]

// UpdateInfraInfo is func to update Infra Info (without VM info in Infra)
func UpdateInfraInfo(nsId string, infraInfoData model.InfraInfo) {
	infraInfoMutex.Lock()
	defer infraInfoMutex.Unlock()

	infraInfoData.Vm = nil

	key := common.GenInfraKey(nsId, infraInfoData.Id, "")

	// Check existence of the key. If no key, no update.
	keyValue, exists, err := kvstore.GetKv(key)
	if !exists || err != nil {
		return
	}

	infraTmp := model.InfraInfo{}
	json.Unmarshal([]byte(keyValue.Value), &infraTmp)

	if !reflect.DeepEqual(infraTmp, infraInfoData) {
		val, _ := json.Marshal(infraInfoData)
		err = kvstore.Put(key, string(val))
		if err != nil {
			log.Error().Err(err).Msg("")
		}
	}
}

// UpdateVmInfo is func to update VM Info
func UpdateVmInfo(nsId string, infraId string, vmInfoData model.VmInfo) {
	infraInfoMutex.Lock()
	defer func() {
		infraInfoMutex.Unlock()
	}()

	key := common.GenInfraKey(nsId, infraId, vmInfoData.Id)

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

// GetInfraAssociatedResources returns a list of associated resource IDs for given Infra info
func GetInfraAssociatedResources(nsId string, infraId string) (model.InfraAssociatedResourceList, error) {

	infraInfo, _, err := GetInfraObject(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.InfraAssociatedResourceList{}, err
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
	nodeGroupIdSet := make(map[string]struct{})
	cspVmNameSet := make(map[string]struct{})
	cspVmIdSet := make(map[string]struct{})

	for _, vm := range infraInfo.Vm {
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
		if vm.NodeGroupId != "" {
			nodeGroupIdSet[vm.NodeGroupId] = struct{}{}
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

	return model.InfraAssociatedResourceList{
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
		NodeGroupIds:     toSlice(nodeGroupIdSet),
		CspVmNames:       toSlice(cspVmNameSet),
		CspVmIds:         toSlice(cspVmIdSet),
	}, nil
}

// ProvisionDataDisk is func to provision DataDisk to VM (create and attach to VM)
func ProvisionDataDisk(ctx context.Context, nsId string, infraId string, vmId string, u *model.DataDiskVmReq) (model.VmInfo, error) {
	vm, err := GetVmObject(nsId, infraId, vmId)
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

	newDataDisk, err := resource.CreateDataDisk(ctx, nsId, &createDiskReq, "")
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.VmInfo{}, err
	}
	retry := 3
	for i := 0; i < retry; i++ {
		vmInfo, err := AttachDetachDataDisk(nsId, infraId, vmId, model.AttachDataDisk, newDataDisk.Id, false)
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
func AttachDetachDataDisk(nsId string, infraId string, vmId string, command string, dataDiskId string, force bool) (model.VmInfo, error) {
	vmKey := common.GenInfraKey(nsId, infraId, vmId)

	// Check existence of the key. If no key, no update.
	keyValue, exists, err := kvstore.GetKv(vmKey)
	if !exists || err != nil {
		err := fmt.Errorf("Failed to find 'ns/infra/vm': %s/%s/%s \n", nsId, infraId, vmId)
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

	client := clientManager.NewHttpClient()
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

	_, err = clientManager.ExecuteHttpRequest(
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

	_, err = clientManager.ExecuteHttpRequest(
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

	UpdateVmInfo(nsId, infraId, vm)

	// Update TB DataDisk object's 'associatedObjects' field
	resource.UpdateAssociatedObjectList(nsId, model.StrDataDisk, dataDiskId, cmdToUpdateAsso, vmKey)

	// Update TB DataDisk object's 'status' field
	// Directly set the final expected status after successful attach/detach operation
	// (no need to query Spider again since the operation was successful)
	switch command {
	case model.AttachDataDisk:
		dataDisk.Status = model.DiskAttached
	case model.DetachDataDisk:
		dataDisk.Status = model.DiskAvailable
	}
	resource.UpdateResourceObject(nsId, model.StrDataDisk, dataDisk)
	log.Debug().Msgf("Updated DataDisk %s status to %s after %s operation", dataDiskId, dataDisk.Status, command)
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

func GetAvailableDataDisks(nsId string, infraId string, vmId string, option string) (interface{}, error) {
	vmKey := common.GenInfraKey(nsId, infraId, vmId)

	// Check existence of the key. If no key, no update.
	keyValue, exists, err := kvstore.GetKv(vmKey)
	if !exists || err != nil {
		err := fmt.Errorf("Failed to find 'ns/infra/vm': %s/%s/%s \n", nsId, infraId, vmId)
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

// [Delete Infra and VM object]

// DelInfra is func to delete Infra object
func DelInfra(nsId string, infraId string, option string) (model.IdList, error) {

	option = common.ToLower(option)
	deletedResources := model.IdList{}
	deleteStatus := "[Done] "

	infraInfo, err := GetInfraInfo(nsId, infraId)

	if err != nil {
		log.Error().Err(err).Msg("Cannot Delete Infra")
		return deletedResources, err
	}

	log.Debug().Msg("[Delete Infra] " + infraId)

	// Check Infra status is Terminated so that approve deletion
	infraStatus, _ := GetInfraStatus(nsId, infraId)
	if infraStatus == nil {
		err := fmt.Errorf("Infra " + infraId + " status nil, Deletion is not allowed (use option=force for force deletion)")
		log.Error().Err(err).Msg("")
		if option != "force" {
			return deletedResources, err
		}
	}

	if !(!strings.Contains(infraStatus.Status, "Partial-") && strings.Contains(infraStatus.Status, model.StatusTerminated)) {

		// with terminate option, do Infra refine and terminate in advance (skip if already model.StatusTerminated)
		if strings.EqualFold(option, model.ActionTerminate) {

			// ActionRefine
			_, err := HandleInfraAction(nsId, infraId, model.ActionRefine, true)
			if err != nil {
				log.Error().Err(err).Msg("")
				return deletedResources, err
			}

			// model.ActionTerminate
			_, err = HandleInfraAction(nsId, infraId, model.ActionTerminate, true)
			if err != nil {
				log.Error().Err(err).Msg("")
				return deletedResources, err
			}
			// Poll until all VMs reach Terminated (or a non-Terminating state),
			// because ControlInfraAsync returns immediately while the CSP processes
			// the termination request in the background (can take several minutes
			// for bare-metal instances, but typically 10-30 s for regular VMs).
			const terminateWaitInterval = 5 * time.Second
			const terminateWaitTimeout = 10 * time.Minute
			log.Info().Msgf("Waiting for Infra %s to become Terminated (polling every %s, timeout %s)", infraId, terminateWaitInterval, terminateWaitTimeout)
			deadline := time.Now().Add(terminateWaitTimeout)
			for time.Now().Before(deadline) {
				time.Sleep(terminateWaitInterval)
				infraStatus, _ = GetInfraStatus(nsId, infraId)
				if infraStatus == nil {
					break
				}
				log.Info().Msgf("Infra %s status: %s", infraId, infraStatus.Status)
				// Exit the loop once every VM has left the Terminating state
				if !strings.Contains(infraStatus.Status, model.StatusTerminating) {
					break
				}
			}
			if infraStatus != nil && strings.Contains(infraStatus.Status, model.StatusTerminating) {
				log.Warn().Msgf("Infra %s is still %s after %s — proceeding with deletion anyway", infraId, infraStatus.Status, terminateWaitTimeout)
			}
		}

	}

	// Check Infra status is Terminated (not Partial)
	// Allow deletion for: Terminated, Undefined, Failed, Preparing, Prepared, Empty
	if infraStatus.Id != "" && !(!strings.Contains(infraStatus.Status, "Partial-") && (strings.Contains(infraStatus.Status, model.StatusTerminated) || strings.Contains(infraStatus.Status, model.StatusUndefined) || strings.Contains(infraStatus.Status, model.StatusFailed) || strings.Contains(infraStatus.Status, model.StatusPreparing) || strings.Contains(infraStatus.Status, model.StatusPrepared) || strings.Contains(infraStatus.Status, model.StatusEmpty))) {
		var err error
		if strings.Contains(infraStatus.Status, model.StatusTerminating) {
			// Termination is still in progress (e.g. bare-metal instances take several minutes).
			// The caller should retry deletion after a while.
			err = fmt.Errorf("Infra %s is still %s — termination is in progress. Please retry deletion in a few minutes", infraId, infraStatus.Status)
		} else {
			err = fmt.Errorf("Infra %s is %s, which is not a deletable status (Terminated/Undefined/Failed/Preparing/Prepared/Empty). Use option=force for forced deletion", infraId, infraStatus.Status)
		}
		log.Error().Err(err).Msg("")
		if option != "force" {
			return deletedResources, err
		}
	}

	key := common.GenInfraKey(nsId, infraId, "")

	// delete associated Infra Policy
	check, _ := CheckInfraPolicy(nsId, infraId)
	if check {
		err = DelInfraPolicy(nsId, infraId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return deletedResources, err
		}
		deletedResources.IdList = append(deletedResources.IdList, deleteStatus+"Policy: "+infraId)
	}

	vmList, err := ListVmId(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return deletedResources, err
	}

	// delete vms info
	for _, v := range vmList {
		vmKey := common.GenInfraKey(nsId, infraId, v)
		fmt.Println(vmKey)

		// get vm info
		vmInfo, err := GetVmObject(nsId, infraId, v)
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

	// delete nodeGroup info
	nodeGroupList, err := ListNodeGroupId(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return deletedResources, err
	}
	for _, v := range nodeGroupList {
		nodeGroupKey := common.GenInfraNodeGroupKey(nsId, infraId, v)
		nodeGroupInfo, err := GetNodeGroup(nsId, infraId, v)
		if err != nil {
			log.Error().Err(err).Msg("Cannot get NodeGroup")
			return deletedResources, err
		}

		err = kvstore.Delete(nodeGroupKey)
		if err != nil {
			log.Error().Err(err).Msg("")
			return deletedResources, err
		}
		deletedResources.IdList = append(deletedResources.IdList, deleteStatus+"NodeGroup: "+v)

		err = label.DeleteLabelObject(model.StrNodeGroup, nodeGroupInfo.Uid)
		if err != nil {
			log.Error().Err(err).Msg("")
		}
	}

	// delete associated CSP NLBs
	forceFlag := "false"
	if option == "force" {
		forceFlag = "true"
	}
	output, err := DelAllNLB(nsId, infraId, "", forceFlag)
	if err != nil {
		log.Error().Err(err).Msg("")
		return deletedResources, err
	}
	deletedResources.IdList = append(deletedResources.IdList, output.IdList...)

	// delete associated Infra NLBs
	infraNlbId := infraId + "-nlb"
	check, _ = CheckInfra(nsId, infraNlbId)
	if check {
		infraNlbDeleteResult, err := DelInfra(nsId, infraNlbId, option)
		if err != nil {
			log.Error().Err(err).Msg("")
			return deletedResources, err
		}
		deletedResources.IdList = append(deletedResources.IdList, infraNlbDeleteResult.IdList...)
	}

	// delete infra info
	err = kvstore.Delete(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return deletedResources, err
	}
	deletedResources.IdList = append(deletedResources.IdList, deleteStatus+"Infra: "+infraId)

	err = label.DeleteLabelObject(model.StrInfra, infraInfo.Uid)
	if err != nil {
		log.Error().Err(err).Msg("")
	}

	return deletedResources, nil
}

// DelInfraVm is func to delete VM object
func DelInfraVm(nsId string, infraId string, vmId string, option string) error {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	err = common.CheckString(infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	err = common.CheckString(vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	check, _ := CheckVm(nsId, infraId, vmId)

	if !check {
		err := fmt.Errorf("The vm " + vmId + " does not exist.")
		return err
	}

	log.Debug().Msg("Deleting VM " + vmId)

	// skip termination if option is force
	if option != "force" {
		// ControlVm first
		_, err := HandleInfraVmAction(nsId, infraId, vmId, model.ActionTerminate, false)
		if err != nil {
			log.Info().Msg(err.Error())
			return err
		}
		// for deletion, need to wait until termination is finished
		log.Info().Msg("Wait for VM termination in 1 second")
		time.Sleep(1 * time.Second)

	}

	// get vm info
	vmInfo, _ := GetVmObject(nsId, infraId, vmId)

	// delete vms info
	key := common.GenInfraKey(nsId, infraId, vmId)
	err = kvstore.Delete(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	// remove empty NodeGroups
	nodeGroup, err := ListNodeGroupId(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list nodeGroup to remove")
		return err
	}
	for _, v := range nodeGroup {
		vmListInNodeGroup, err := ListVmByNodeGroup(nsId, infraId, v)
		if err != nil {
			log.Error().Err(err).Msg("Failed to list vm in nodeGroup to remove")
			return err
		}
		if len(vmListInNodeGroup) == 0 {
			nodeGroupKey := common.GenInfraNodeGroupKey(nsId, infraId, v)
			err := kvstore.Delete(nodeGroupKey)
			if err != nil {
				log.Error().Err(err).Msg("Failed to remove the empty nodeGroup")
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

// DeregisterInfraVm deregisters VM from Spider and TB without deleting the actual CSP resource
// This function only removes the VM mapping from Spider and TB internal storage
// The actual CSP VM resource remains intact and can be re-registered later
func DeregisterInfraVm(nsId string, infraId string, vmId string) error {

	log.Debug().Msg("[Deregister VM] " + vmId)

	// get vm info
	vmInfo, err := GetVmObject(nsId, infraId, vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	// Check if associated resources exist before deregistration
	var relatedResources []string

	// Check DataDisks
	for _, dataDiskId := range vmInfo.DataDiskIds {
		exists, _ := resource.CheckResource(nsId, model.StrDataDisk, dataDiskId)
		if exists {
			relatedResources = append(relatedResources, fmt.Sprintf("DataDisk: %s", dataDiskId))
		}
	}

	// Check SecurityGroups
	for _, sgId := range vmInfo.SecurityGroupIds {
		exists, _ := resource.CheckResource(nsId, model.StrSecurityGroup, sgId)
		if exists {
			relatedResources = append(relatedResources, fmt.Sprintf("SecurityGroup: %s", sgId))
		}
	}

	// Check SSHKey
	if vmInfo.SshKeyId != "" {
		exists, _ := resource.CheckResource(nsId, model.StrSSHKey, vmInfo.SshKeyId)
		if exists {
			relatedResources = append(relatedResources, fmt.Sprintf("SSHKey: %s", vmInfo.SshKeyId))
		}
	}

	// If any resources are missing, return error
	if len(relatedResources) > 0 {
		err := fmt.Errorf("cannot deregister VM '%s': the following associated resources do not exist: %v", vmId, relatedResources)
		return err
	}

	// Call Spider deregister API
	var callResult interface{}
	client := clientManager.NewHttpClient()
	method := "DELETE"

	// Create request body
	type JsonTemplate struct {
		ConnectionName string
	}
	requestBody := JsonTemplate{
		ConnectionName: vmInfo.ConnectionName,
	}

	url := model.SpiderRestUrl + "/regvm/" + vmInfo.CspResourceName
	log.Debug().Msg("Sending deregister DELETE request to " + url)

	_, err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&callResult,
		clientManager.VeryShortDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	log.Debug().Msg("Deregister request finished from " + url)

	// delete the VM info from TB
	key := common.GenInfraKey(nsId, infraId, vmId)
	err = kvstore.Delete(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	// remove empty NodeGroups
	vmListInNodeGroup, err := ListVmByNodeGroup(nsId, infraId, vmInfo.NodeGroupId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list vm in nodeGroup to remove")
		return err
	}
	if len(vmListInNodeGroup) == 0 {
		nodeGroupKey := common.GenInfraNodeGroupKey(nsId, infraId, vmInfo.NodeGroupId)
		err := kvstore.Delete(nodeGroupKey)
		if err != nil {
			log.Error().Err(err).Msg("Failed to remove the empty nodeGroup")
			return err
		}
	}

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

// DelAllInfra is func to delete all Infra objects in parallel
func DelAllInfra(nsId string, option string) (string, error) {

	infraList, err := ListInfraId(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}

	if len(infraList) == 0 {
		return "No Infra to delete", nil
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(infraList))
	defer close(errCh)

	for _, v := range infraList {
		wg.Add(1)
		go func(infraId string) {
			defer wg.Done()
			_, err := DelInfra(nsId, infraId, option)
			if err != nil {
				log.Error().Err(err).Str("infraId", infraId).Msg("Failed to delete Infra")
				errCh <- err
			}
		}(v)
	}

	wg.Wait()

	select {
	case err := <-errCh:
		return "", fmt.Errorf("failed to delete all Infras: %v", err)
	default:
		return "All Infras have been deleted", nil
	}
}

// UpdateVmPublicIp is func to update VM public IP
func UpdateVmPublicIp(nsId string, infraId string, vmInfoData model.VmInfo) error {

	vmInfoTmp, err := GetVmCurrentPublicIp(nsId, infraId, vmInfoData.Id)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	if vmInfoData.PublicIP != vmInfoTmp.PublicIp || vmInfoData.SSHPort != vmInfoTmp.SSHPort {
		vmInfoData.PublicIP = vmInfoTmp.PublicIp
		vmInfoData.SSHPort = vmInfoTmp.SSHPort
		UpdateVmInfo(nsId, infraId, vmInfoData)
	}
	return nil
}

// GetVmTemplate is func to get VM template
func GetVmTemplate(nsId string, infraId string, algo string) (model.VmInfo, error) {

	log.Debug().Msg("[GetVmTemplate]" + infraId + " by algo: " + algo)

	vmList, err := ListVmId(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.VmInfo{}, err
	}
	if len(vmList) == 0 {
		return model.VmInfo{}, nil
	}

	rand.Seed(time.Now().UnixNano())
	index := rand.Intn(len(vmList))
	vmObj, vmErr := GetVmObject(nsId, infraId, vmList[index])
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
