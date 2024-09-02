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
	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	validator "github.com/go-playground/validator/v10"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
)

// TbMciReqStructLevelValidation is func to validate fields in TbMciReqStruct
func TbMciReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(model.TbMciReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

// TbVmReqStructLevelValidation is func to validate fields in model.TbVmReqStruct
func TbVmReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(model.TbVmReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

var holdingMciMap sync.Map

// MCI and VM Provisioning

// CreateMciVm is func to post (create) MciVm
func CreateMciVm(nsId string, mciId string, vmInfoData *model.TbVmInfo) (*model.TbVmInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := &model.TbVmInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		temp := &model.TbVmInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}
	err = common.CheckString(vmInfoData.Name)
	if err != nil {
		temp := &model.TbVmInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}
	check, _ := CheckVm(nsId, mciId, vmInfoData.Name)

	if check {
		temp := &model.TbVmInfo{}
		err := fmt.Errorf("The vm " + vmInfoData.Name + " already exists.")
		return temp, err
	}

	vmInfoData.Id = vmInfoData.Name
	vmInfoData.PublicIP = "empty"
	vmInfoData.PublicDNS = "empty"
	vmInfoData.TargetAction = model.ActionCreate
	vmInfoData.TargetStatus = model.StatusRunning
	vmInfoData.Status = model.StatusCreating

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
			reqToMon := &model.MciCmdReq{}
			reqToMon.UserName = "cb-user" // this MCI user name is temporal code. Need to improve.

			fmt.Printf("\n[InstallMonitorAgentToMci]\n\n")
			content, err := InstallMonitorAgentToMci(nsId, mciId, model.StrMCI, reqToMon)
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
func ScaleOutMciSubGroup(nsId string, mciId string, subGroupId string, numVMsToAdd string) (*model.TbMciInfo, error) {
	vmIdList, err := ListVmBySubGroup(nsId, mciId, subGroupId)
	if err != nil {
		temp := &model.TbMciInfo{}
		return temp, err
	}
	vmObj, err := GetVmObject(nsId, mciId, vmIdList[0])

	vmTemplate := &model.TbVmReq{}

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
		temp := &model.TbMciInfo{}
		return temp, err
	}
	return result, nil

}

// CreateMciGroupVm is func to create MCI groupVM
func CreateMciGroupVm(nsId string, mciId string, vmRequest *model.TbVmReq, newSubGroup bool) (*model.TbMciInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := &model.TbMciInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		temp := &model.TbMciInfo{}
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
		temp := &model.TbMciInfo{}
		return temp, err
	}

	//vmRequest := req

	targetAction := model.ActionCreate
	targetStatus := model.StatusRunning

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
		return &model.TbMciInfo{}, err
	}

	if subGroupSize > 0 {

		log.Info().Msg("Create MCI subGroup object")

		subGroupInfoData := model.TbSubGroupInfo{}
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
		vmInfoData := model.TbVmInfo{}

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

		vmInfoData.Status = model.StatusCreating
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
		temp := &model.TbMciInfo{}
		return temp, err
	}

	mciStatusTmp, _ := GetMciStatus(nsId, mciId)

	mciTmp.Status = mciStatusTmp.Status

	if mciTmp.TargetStatus == mciTmp.Status {
		mciTmp.TargetStatus = model.StatusComplete
		mciTmp.TargetAction = model.ActionComplete
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
			reqToMon := &model.MciCmdReq{}
			reqToMon.UserName = "cb-user" // this MCI user name is temporal code. Need to improve.

			fmt.Printf("\n[InstallMonitorAgentToMci]\n\n")
			content, err := InstallMonitorAgentToMci(nsId, mciId, model.StrMCI, reqToMon)
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
func CreateMci(nsId string, req *model.TbMciReq, option string) (*model.TbMciInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := &model.TbMciInfo{}
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

	uuid := common.GenUid()

	targetAction := model.ActionCreate
	targetStatus := model.StatusRunning

	mciId := req.Name
	vmRequest := req.Vm

	log.Info().Msg("Create MCI object")
	key := common.GenMciKey(nsId, mciId, "")
	mapA := map[string]string{
		"id":              mciId,
		"name":            mciId,
		"uuid":            uuid,
		"description":     req.Description,
		"status":          model.StatusCreating,
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

	// Store label info using CreateOrUpdateLabel
	labels := map[string]string{
		"provider":  "cb-tumblebug",
		"namespace": nsId,
	}
	err = label.CreateOrUpdateLabel(model.StrMCI, uuid, key, labels)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	// Check whether VM names meet requirement.
	for _, k := range vmRequest {
		err = common.CheckString(k.Name)
		if err != nil {
			log.Error().Err(err).Msg("")
			return &model.TbMciInfo{}, err
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

			uuidSubGroup := common.GenUid()

			subGroupInfoData := model.TbSubGroupInfo{}
			subGroupInfoData.Id = common.ToLower(k.Name)
			subGroupInfoData.Name = common.ToLower(k.Name)
			subGroupInfoData.Uuid = uuidSubGroup
			subGroupInfoData.SubGroupSize = k.SubGroupSize

			for i := vmStartIndex; i < subGroupSize+vmStartIndex; i++ {
				subGroupInfoData.VmId = append(subGroupInfoData.VmId, subGroupInfoData.Id+"-"+strconv.Itoa(i))
			}

			val, _ := json.Marshal(subGroupInfoData)
			err := kvstore.Put(key, string(val))
			if err != nil {
				log.Error().Err(err).Msg("")
			}

			// Store label info using CreateOrUpdateLabel
			labels := map[string]string{
				"provider":  "cb-tumblebug",
				"namespace": nsId,
			}
			err = label.CreateOrUpdateLabel(model.StrSubGroup, uuid, key, labels)
			if err != nil {
				log.Error().Err(err).Msg("")
				return nil, err
			}

		}

		for i := vmStartIndex; i <= subGroupSize+vmStartIndex; i++ {
			vmInfoData := model.TbVmInfo{}

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
			uuidVm := common.GenUid()

			vmInfoData.Id = vmInfoData.Name
			vmInfoData.Uuid = uuidVm

			vmInfoData.PublicIP = "empty"
			vmInfoData.PublicDNS = "empty"

			vmInfoData.Status = model.StatusCreating
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
		mciTmp.TargetStatus = model.StatusComplete
		mciTmp.TargetAction = model.ActionComplete
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
			reqToMon := &model.MciCmdReq{}
			reqToMon.UserName = "cb-user" // this MCI user name is temporal code. Need to improve.

			fmt.Printf("\n===========================\n")
			// Sleep for 60 seconds for a safe DF agent installation.
			fmt.Printf("\n\n[Info] Sleep for 60 seconds for safe CB-Dragonfly Agent installation.\n")
			time.Sleep(60 * time.Second)

			fmt.Printf("\n[InstallMonitorAgentToMci]\n\n")
			content, err := InstallMonitorAgentToMci(nsId, mciId, model.StrMCI, reqToMon)
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
func CheckMciDynamicReq(req *model.MciConnectionConfigCandidatesReq) (*model.CheckMciDynamicReqInfo, error) {

	mciReqInfo := model.CheckMciDynamicReqInfo{}

	connectionConfigList, err := common.GetConnConfigList(model.DefaultCredentialHolder, true, true)
	if err != nil {
		err := fmt.Errorf("Cannot load ConnectionConfigList in MCI dynamic request check.")
		log.Error().Err(err).Msg("")
		return &mciReqInfo, err
	}

	// Find detail info and ConnectionConfigCandidates
	for _, k := range req.CommonSpecs {
		errMessage := ""

		vmReqInfo := model.CheckVmDynamicReqInfo{}

		specInfo, err := resource.GetSpec(model.SystemCommonNs, k)
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
		availableImageList, err := resource.SearchImage(model.SystemCommonNs, imageSearchKey)
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
func CreateSystemMciDynamic(option string) (*model.TbMciInfo, error) {
	nsId := model.SystemCommonNs
	req := &model.TbMciDynamicReq{}

	// special purpose MCI
	req.Name = option
	req.Label = option
	req.SystemLabel = option
	req.Description = option
	req.InstallMonAgent = "no"

	switch option {
	case "probe":
		connections, err := common.GetConnConfigList(model.DefaultCredentialHolder, true, true)
		if err != nil {
			log.Error().Err(err).Msg("")
			return nil, err
		}
		for _, v := range connections.Connectionconfig {

			vmReq := &model.TbVmDynamicReq{}
			vmReq.CommonImage = "ubuntu18.04"                // temporal default value. will be changed
			vmReq.CommonSpec = "aws-ap-northeast-2-t2-small" // temporal default value. will be changed

			deploymentPlan := model.DeploymentPlan{}
			condition := []model.Operation{}
			condition = append(condition, model.Operation{Operand: v.RegionZoneInfoName})

			log.Debug().Msg(" - v.RegionName: " + v.RegionZoneInfoName)

			deploymentPlan.Filter.Policy = append(deploymentPlan.Filter.Policy, model.FilterCondition{Metric: "region", Condition: condition})
			deploymentPlan.Limit = "1"
			common.PrintJsonPretty(deploymentPlan)

			specList, err := RecommendVm(model.SystemCommonNs, deploymentPlan)
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
func CreateMciDynamic(reqID string, nsId string, req *model.TbMciDynamicReq, deployOption string) (*model.TbMciInfo, error) {

	mciReq := model.TbMciReq{}
	mciReq.Name = req.Name
	mciReq.Label = req.Label
	mciReq.SystemLabel = req.SystemLabel
	mciReq.InstallMonAgent = req.InstallMonAgent
	mciReq.Description = req.Description

	emptyMci := &model.TbMciInfo{}
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
func CreateMciVmDynamic(nsId string, mciId string, req *model.TbVmDynamicReq) (*model.TbMciInfo, error) {

	emptyMci := &model.TbMciInfo{}
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
func checkCommonResAvailable(req *model.TbVmDynamicReq) error {

	vmRequest := req
	// Check whether VM names meet requirement.
	k := vmRequest

	vmReq := &model.TbVmReq{}

	specInfo, err := resource.GetSpec(model.SystemCommonNs, req.CommonSpec)
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
	_, err = resource.GetImage(model.SystemCommonNs, vmReq.ImageId)
	if err != nil {
		err := fmt.Errorf("Failed to get Image " + k.CommonImage + " from " + vmReq.ConnectionName)
		log.Error().Err(err).Msg("")
		return err
	}

	return nil
}

// getVmReqForDynamicMci is func to getVmReqFromDynamicReq
func getVmReqFromDynamicReq(reqID string, nsId string, req *model.TbVmDynamicReq) (*model.TbVmReq, error) {

	onDemand := true

	vmRequest := req
	// Check whether VM names meet requirement.
	k := vmRequest

	vmReq := &model.TbVmReq{}

	specInfo, err := resource.GetSpec(model.SystemCommonNs, req.CommonSpec)
	if err != nil {
		log.Error().Err(err).Msg("")
		return &model.TbVmReq{}, err
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
		return &model.TbVmReq{}, err
	}

	// Default resource name has this pattern (nsId + "-shared-" + vmReq.ConnectionName)
	resourceName := nsId + model.StrSharedResourceName + vmReq.ConnectionName

	vmReq.SpecId = specInfo.Id
	osType := strings.ReplaceAll(k.CommonImage, " ", "")
	vmReq.ImageId = resource.GetProviderRegionZoneResourceKey(connection.ProviderName, connection.RegionDetail.RegionName, "", osType)
	// incase of user provided image id completely (e.g. aws+ap-northeast-2+ubuntu22.04)
	if strings.Contains(k.CommonImage, "+") {
		vmReq.ImageId = k.CommonImage
	}
	_, err = resource.GetImage(model.SystemCommonNs, vmReq.ImageId)
	if err != nil {
		err := fmt.Errorf("Failed to get the Image " + vmReq.ImageId + " from " + vmReq.ConnectionName)
		log.Error().Err(err).Msg("")
		return &model.TbVmReq{}, err
	}

	common.UpdateRequestProgress(reqID, common.ProgressInfo{Title: "Setting vNet:" + resourceName, Time: time.Now()})

	vmReq.VNetId = resourceName
	_, err = resource.GetResource(nsId, model.StrVNet, vmReq.VNetId)
	if err != nil {
		if !onDemand {
			err := fmt.Errorf("Failed to get the vNet " + vmReq.VNetId + " from " + vmReq.ConnectionName)
			log.Error().Err(err).Msg("Failed to get the vNet")
			return &model.TbVmReq{}, err
		}
		common.UpdateRequestProgress(reqID, common.ProgressInfo{Title: "Loading default vNet:" + resourceName, Time: time.Now()})
		err2 := resource.LoadSharedResource(nsId, model.StrVNet, vmReq.ConnectionName)
		if err2 != nil {
			log.Error().Err(err2).Msg("Failed to create new default vNet " + vmReq.VNetId + " from " + vmReq.ConnectionName)
			return &model.TbVmReq{}, err2
		} else {
			log.Info().Msg("Created new default vNet: " + vmReq.VNetId)
		}
	} else {
		log.Info().Msg("Found and utilize default vNet: " + vmReq.VNetId)
	}
	vmReq.SubnetId = resourceName

	common.UpdateRequestProgress(reqID, common.ProgressInfo{Title: "Setting SSHKey:" + resourceName, Time: time.Now()})
	vmReq.SshKeyId = resourceName
	_, err = resource.GetResource(nsId, model.StrSSHKey, vmReq.SshKeyId)
	if err != nil {
		if !onDemand {
			err := fmt.Errorf("Failed to get the SSHKey " + vmReq.SshKeyId + " from " + vmReq.ConnectionName)
			log.Error().Err(err).Msg("Failed to get the SSHKey")
			return &model.TbVmReq{}, err
		}
		common.UpdateRequestProgress(reqID, common.ProgressInfo{Title: "Loading default SSHKey:" + resourceName, Time: time.Now()})
		err2 := resource.LoadSharedResource(nsId, model.StrSSHKey, vmReq.ConnectionName)
		if err2 != nil {
			log.Error().Err(err2).Msg("Failed to create new default SSHKey " + vmReq.SshKeyId + " from " + vmReq.ConnectionName)
			return &model.TbVmReq{}, err2
		} else {
			log.Info().Msg("Created new default SSHKey: " + vmReq.VNetId)
		}
	} else {
		log.Info().Msg("Found and utilize default SSHKey: " + vmReq.VNetId)
	}

	common.UpdateRequestProgress(reqID, common.ProgressInfo{Title: "Setting securityGroup:" + resourceName, Time: time.Now()})
	securityGroup := resourceName
	vmReq.SecurityGroupIds = append(vmReq.SecurityGroupIds, securityGroup)
	_, err = resource.GetResource(nsId, model.StrSecurityGroup, securityGroup)
	if err != nil {
		if !onDemand {
			err := fmt.Errorf("Failed to get the securityGroup " + securityGroup + " from " + vmReq.ConnectionName)
			log.Error().Err(err).Msg("Failed to get the securityGroup")
			return &model.TbVmReq{}, err
		}
		common.UpdateRequestProgress(reqID, common.ProgressInfo{Title: "Loading default securityGroup:" + resourceName, Time: time.Now()})
		err2 := resource.LoadSharedResource(nsId, model.StrSecurityGroup, vmReq.ConnectionName)
		if err2 != nil {
			log.Error().Err(err2).Msg("Failed to create new default securityGroup " + securityGroup + " from " + vmReq.ConnectionName)
			return &model.TbVmReq{}, err2
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
func AddVmToMci(wg *sync.WaitGroup, nsId string, mciId string, vmInfoData *model.TbVmInfo, option string) error {
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
		vmInfoData.Status = model.StatusFailed
		vmInfoData.SystemMessage = err.Error()
		UpdateVmInfo(nsId, mciId, *vmInfoData)
		log.Error().Err(err).Msg("")
		return err
	}

	// set initial TargetAction, TargetStatus
	vmInfoData.TargetAction = model.ActionComplete
	vmInfoData.TargetStatus = model.StatusComplete

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

	// Store label info using CreateOrUpdateLabel
	labels := map[string]string{
		"provider":  "cb-tumblebug",
		"namespace": nsId,
	}
	err = label.CreateOrUpdateLabel(model.StrVM, vmInfoData.Uuid, key, labels)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	return nil

}

// CreateVm is func to create VM (option = "register" for register existing VM)
func CreateVm(nsId string, mciId string, vmInfoData *model.TbVmInfo, option string) error {

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

	var callResult model.SpiderVMInfo

	// Fill VM creation reqest (request to cb-spider)
	requestBody := model.SpiderVMReqInfoWrapper{}
	requestBody.ConnectionName = vmInfoData.ConnectionName

	//generate VM ID(Name) to request to CSP(Spider)
	requestBody.ReqInfo.Name = vmInfoData.Uuid

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
		requestBody.ReqInfo.ImageName, err = resource.GetCspResourceId(nsId, model.StrCustomImage, vmInfoData.ImageId)
		if requestBody.ReqInfo.ImageName == "" || err != nil {
			log.Warn().Msgf("Not found %s from CustomImage in ns: %s, find it from UserImage", vmInfoData.ImageId, nsId)
			errAgg := err.Error()
			// If customImage doesn't exist, then try lookup image
			requestBody.ReqInfo.ImageName, err = resource.GetCspResourceId(nsId, model.StrImage, vmInfoData.ImageId)
			if requestBody.ReqInfo.ImageName == "" || err != nil {
				log.Warn().Msgf("Not found %s from UserImage in ns: %s, find CommonImage from SystemCommonNs", vmInfoData.ImageId, nsId)
				errAgg += err.Error()
				// If cannot find the resource, use common resource
				requestBody.ReqInfo.ImageName, err = resource.GetCspResourceId(model.SystemCommonNs, model.StrImage, vmInfoData.ImageId)
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
			requestBody.ReqInfo.ImageType = model.MyImage
			// If the requested image is a custom image (generated by VM snapshot), RootDiskType should be empty.
			// TB ignore inputs for RootDiskType, RootDiskSize
			requestBody.ReqInfo.RootDiskType = ""
			requestBody.ReqInfo.RootDiskSize = ""
		}

		requestBody.ReqInfo.VMSpecName, err = resource.GetCspResourceId(nsId, model.StrSpec, vmInfoData.SpecId)
		if requestBody.ReqInfo.VMSpecName == "" || err != nil {
			log.Warn().Msgf("Not found the Spec: %s in nsId: %s, find it from SystemCommonNs", vmInfoData.SpecId, nsId)
			errAgg := err.Error()
			// If cannot find the resource, use common resource
			requestBody.ReqInfo.VMSpecName, err = resource.GetCspResourceId(model.SystemCommonNs, model.StrSpec, vmInfoData.SpecId)
			log.Info().Msgf("Use the common VMSpecName: %s", requestBody.ReqInfo.VMSpecName)

			if requestBody.ReqInfo.ImageName == "" || err != nil {
				errAgg += err.Error()
				err = fmt.Errorf(errAgg)
				log.Error().Err(err).Msg("")
				return err
			}
		}

		requestBody.ReqInfo.VPCName, err = resource.GetCspResourceId(nsId, model.StrVNet, vmInfoData.VNetId)
		if requestBody.ReqInfo.VPCName == "" {
			log.Error().Err(err).Msg("")
			return err
		}

		// retrieve csp subnet id
		subnetInfo, err := resource.GetSubnet(nsId, vmInfoData.VNetId, vmInfoData.SubnetId)
		if err != nil {
			log.Error().Err(err).Msg("Cannot find the Subnet ID: " + vmInfoData.SubnetId)
			return err
		}

		requestBody.ReqInfo.SubnetName = subnetInfo.CspSubnetName
		if requestBody.ReqInfo.SubnetName == "" {
			log.Error().Err(err).Msg("")
			return err
		}

		var SecurityGroupIdsTmp []string
		for _, v := range vmInfoData.SecurityGroupIds {
			CspSgId, err := resource.GetCspResourceId(nsId, model.StrSecurityGroup, v)
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
				CspDataDiskId, err := resource.GetCspResourceId(nsId, model.StrDataDisk, v)
				if err != nil || CspDataDiskId == "" {
					log.Error().Err(err).Msg("")
					return err
				}
				DataDiskIdsTmp = append(DataDiskIdsTmp, CspDataDiskId)
			}
		}
		requestBody.ReqInfo.DataDiskNames = DataDiskIdsTmp

		requestBody.ReqInfo.KeyPairName, err = resource.GetCspResourceId(nsId, model.StrSSHKey, vmInfoData.SshKeyId)
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

	url := model.SpiderRestUrl + "/vm"
	if option == "register" {
		url = model.SpiderRestUrl + "/regvm"
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
		resourceListInNs, err := resource.ListResource(nsId, model.StrVNet, "cspVNetName", callResult.VpcIID.NameId)
		if err != nil {
			log.Error().Err(err).Msg("")
		} else {
			resourcesInNs := resourceListInNs.([]model.TbVNetInfo) // type assertion
			for _, resource := range resourcesInNs {
				if resource.ConnectionName == requestBody.ConnectionName {
					vmInfoData.VNetId = resource.Id
					//vmInfoData.SubnetId = resource.SubnetInfoList
				}
			}
		}

		// access Key
		resourceListInNs, err = resource.ListResource(nsId, model.StrSSHKey, "cspSshKeyName", callResult.KeyPairIId.NameId)
		if err != nil {
			log.Error().Err(err).Msg("")
		} else {
			resourcesInNs := resourceListInNs.([]model.TbSshKeyInfo) // type assertion
			for _, resource := range resourcesInNs {
				if resource.ConnectionName == requestBody.ConnectionName {
					vmInfoData.SshKeyId = resource.Id
				}
			}
		}

	} else {
		vmKey := common.GenMciKey(nsId, mciId, vmInfoData.Id)

		if customImageFlag == false {
			resource.UpdateAssociatedObjectList(nsId, model.StrImage, vmInfoData.ImageId, model.StrAdd, vmKey)
		} else {
			resource.UpdateAssociatedObjectList(nsId, model.StrCustomImage, vmInfoData.ImageId, model.StrAdd, vmKey)
		}

		//resource.UpdateAssociatedObjectList(nsId, model.StrSpec, vmInfoData.SpecId, model.StrAdd, vmKey)
		resource.UpdateAssociatedObjectList(nsId, model.StrSSHKey, vmInfoData.SshKeyId, model.StrAdd, vmKey)
		resource.UpdateAssociatedObjectList(nsId, model.StrVNet, vmInfoData.VNetId, model.StrAdd, vmKey)

		for _, v := range vmInfoData.SecurityGroupIds {
			resource.UpdateAssociatedObjectList(nsId, model.StrSecurityGroup, v, model.StrAdd, vmKey)
		}

		for _, v := range vmInfoData.DataDiskIds {
			resource.UpdateAssociatedObjectList(nsId, model.StrDataDisk, v, model.StrAdd, vmKey)
		}
	}

	// Register dataDisks which are created with the creation of VM
	for _, v := range callResult.DataDiskIIDs {
		tbDataDiskReq := model.TbDataDiskReq{
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
		resource.UpdateAssociatedObjectList(nsId, model.StrDataDisk, dataDisk.Id, model.StrAdd, vmKey)
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
