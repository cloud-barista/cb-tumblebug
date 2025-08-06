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
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
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

// CreatedResource represents a resource created during dynamic MCI provisioning
type CreatedResource struct {
	Type string `json:"type"` // "vnet", "sshkey", "securitygroup"
	Id   string `json:"id"`   // Resource ID
}

// VmReqWithCreatedResources contains VM request and list of created resources for rollback
type VmReqWithCreatedResources struct {
	VmReq            *model.TbVmReq    `json:"vmReq"`
	CreatedResources []CreatedResource `json:"createdResources"`
}

// rollbackCreatedResources deletes only the resources that were created during this MCI creation
func rollbackCreatedResources(nsId string, createdResources []CreatedResource) error {
	var errors []string

	vNetIds := make([]string, 0)
	sshKeyIds := make([]string, 0)
	securityGroupIds := make([]string, 0)

	for _, res := range createdResources {
		switch res.Type {
		case model.StrVNet:
			vNetIds = append(vNetIds, res.Id)
		case model.StrSSHKey:
			sshKeyIds = append(sshKeyIds, res.Id)
		case model.StrSecurityGroup:
			securityGroupIds = append(securityGroupIds, res.Id)
		}
	}

	for _, res := range sshKeyIds {
		if err := resource.DelResource(nsId, model.StrSSHKey, res, "false"); err != nil {
			errors = append(errors, fmt.Sprintf("Failed to delete SSHKey %s: %v", res, err))
			log.Error().Err(err).Msgf("Failed to rollback SSHKey: %s", res)
		} else {
			log.Info().Msgf("Successfully rolled back SSHKey: %s", res)
		}
	}

	for _, res := range securityGroupIds {
		if err := resource.DelResource(nsId, model.StrSecurityGroup, res, "false"); err != nil {
			errors = append(errors, fmt.Sprintf("Failed to delete SecurityGroup %s: %v", res, err))
			log.Error().Err(err).Msgf("Failed to rollback SecurityGroup: %s", res)
		} else {
			log.Info().Msgf("Successfully rolled back SecurityGroup: %s", res)
		}
	}

	for _, res := range vNetIds {
		if err := resource.DelResource(nsId, model.StrVNet, res, "false"); err != nil {
			errors = append(errors, fmt.Sprintf("Failed to delete VNet %s: %v", res, err))
			log.Error().Err(err).Msgf("Failed to rollback VNet: %s", res)
		} else {
			log.Info().Msgf("Successfully rolled back VNet: %s", res)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("rollback completed with errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

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
	go CreateVmObject(&wg, nsId, mciId, vmInfoData)
	wg.Wait()

	wg.Add(1)
	go CreateVm(&wg, nsId, mciId, vmInfoData, option)
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

	if strings.Contains(mciTmp.InstallMonAgent, "yes") {

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
	vmTemplate.VmUserName = vmObj.VmUserName
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
		subGroupInfoData.ResourceType = model.StrSubGroup
		subGroupInfoData.Id = tentativeVmId
		subGroupInfoData.Name = tentativeVmId
		subGroupInfoData.Uid = common.GenUid()
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
			vmInfoData.Name = common.ToLower(vmRequest.Name)
		} else { // for VM (in a group)
			if i == subGroupSize+vmStartIndex {
				break
			}
			vmInfoData.SubGroupId = common.ToLower(vmRequest.Name)
			vmInfoData.Name = common.ToLower(vmRequest.Name) + "-" + strconv.Itoa(i)

			log.Debug().Msg("vmInfoData.Name: " + vmInfoData.Name)

		}
		vmInfoData.ResourceType = model.StrVM
		vmInfoData.Id = vmInfoData.Name
		vmInfoData.Uid = common.GenUid()

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
		vmInfoData.Location = vmInfoData.ConnectionConfig.RegionDetail.Location
		vmInfoData.SpecId = vmRequest.SpecId
		vmInfoData.ImageId = vmRequest.ImageId
		vmInfoData.VNetId = vmRequest.VNetId
		vmInfoData.SubnetId = vmRequest.SubnetId
		vmInfoData.SecurityGroupIds = vmRequest.SecurityGroupIds
		vmInfoData.DataDiskIds = vmRequest.DataDiskIds
		vmInfoData.SshKeyId = vmRequest.SshKeyId
		vmInfoData.Description = vmRequest.Description
		vmInfoData.VmUserName = vmRequest.VmUserName
		vmInfoData.VmUserPassword = vmRequest.VmUserPassword
		vmInfoData.RootDiskType = vmRequest.RootDiskType
		vmInfoData.RootDiskSize = vmRequest.RootDiskSize

		vmInfoData.Label = vmRequest.Label

		vmInfoData.CspResourceId = vmRequest.CspResourceId

		wg.Add(1)
		go CreateVmObject(&wg, nsId, mciId, &vmInfoData)
	}
	wg.Wait()

	option := "create"

	for i := vmStartIndex; i <= subGroupSize+vmStartIndex; i++ {
		vmInfoData := model.TbVmInfo{}

		if subGroupSize == 0 { // for VM (not in a group)
			vmInfoData.Name = common.ToLower(vmRequest.Name)
		} else { // for VM (in a group)
			if i == subGroupSize+vmStartIndex {
				break
			}
			vmInfoData.SubGroupId = common.ToLower(vmRequest.Name)
			vmInfoData.Name = common.ToLower(vmRequest.Name) + "-" + strconv.Itoa(i)
		}
		vmInfoData.Id = vmInfoData.Name
		vmId := vmInfoData.Id
		vmInfoData, err := GetVmObject(nsId, mciId, vmId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return nil, err
		}

		// Avoid concurrent requests to CSP.
		time.Sleep(time.Millisecond * 1000)

		wg.Add(1)
		go CreateVm(&wg, nsId, mciId, &vmInfoData, option)
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

	if strings.Contains(mciTmp.InstallMonAgent, "yes") {

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

	uid := common.GenUid()

	targetAction := model.ActionCreate
	targetStatus := model.StatusRunning

	mciId := req.Name
	vmRequests := req.Vm

	log.Info().Msg("Create MCI object")
	key := common.GenMciKey(nsId, mciId, "")

	mciInfo := model.TbMciInfo{
		ResourceType:    model.StrMCI,
		Id:              mciId,
		Name:            req.Name,
		Uid:             uid,
		Description:     req.Description,
		Status:          model.StatusCreating,
		TargetAction:    targetAction,
		TargetStatus:    targetStatus,
		InstallMonAgent: req.InstallMonAgent,
		SystemLabel:     req.SystemLabel,
		PostCommand:     req.PostCommand,
	}

	val, err := json.Marshal(mciInfo)
	if err != nil {
		err := fmt.Errorf("System Error: CreateMci json.Marshal(mciInfo) Error")
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
		model.LabelManager:     model.StrManager,
		model.LabelNamespace:   nsId,
		model.LabelLabelType:   model.StrMCI,
		model.LabelId:          mciId,
		model.LabelName:        req.Name,
		model.LabelUid:         uid,
		model.LabelDescription: req.Description,
	}
	for key, value := range req.Label {
		labels[key] = value
	}

	err = label.CreateOrUpdateLabel(model.StrMCI, uid, key, labels)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	// Check whether VM names meet requirement.
	for _, k := range vmRequests {
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

	for _, vmRequest := range vmRequests {

		// subGroup handling
		subGroupSize, err := strconv.Atoi(vmRequest.SubGroupSize)
		if err != nil {
			subGroupSize = 1
		}
		fmt.Printf("subGroupSize: %v\n", subGroupSize)

		if subGroupSize > 0 {

			log.Info().Msg("Create MCI subGroup object")
			key := common.GenMciSubGroupKey(nsId, mciId, vmRequest.Name)

			subGroupInfoData := model.TbSubGroupInfo{}
			subGroupInfoData.ResourceType = model.StrSubGroup
			subGroupInfoData.Id = common.ToLower(vmRequest.Name)
			subGroupInfoData.Name = common.ToLower(vmRequest.Name)
			subGroupInfoData.Uid = common.GenUid()
			subGroupInfoData.SubGroupSize = vmRequest.SubGroupSize

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
				model.LabelManager:        model.StrManager,
				model.LabelNamespace:      nsId,
				model.LabelLabelType:      model.StrSubGroup,
				model.LabelId:             subGroupInfoData.Id,
				model.LabelName:           subGroupInfoData.Name,
				model.LabelUid:            subGroupInfoData.Uid,
				model.LabelMciId:          mciId,
				model.LabelMciName:        req.Name,
				model.LabelMciUid:         uid,
				model.LabelMciDescription: req.Description,
			}
			err = label.CreateOrUpdateLabel(model.StrSubGroup, uid, key, labels)
			if err != nil {
				log.Error().Err(err).Msg("")
				return nil, err
			}

		}

		for i := vmStartIndex; i <= subGroupSize+vmStartIndex; i++ {
			vmInfoData := model.TbVmInfo{}

			if subGroupSize == 0 { // for VM (not in a group)
				vmInfoData.Name = common.ToLower(vmRequest.Name)
			} else { // for VM (in a group)
				if i == subGroupSize+vmStartIndex {
					break
				}
				vmInfoData.SubGroupId = common.ToLower(vmRequest.Name)
				vmInfoData.Name = common.ToLower(vmRequest.Name) + "-" + strconv.Itoa(i)

				log.Debug().Msg("vmInfoData.Name: " + vmInfoData.Name)

			}
			vmInfoData.ResourceType = model.StrVM
			vmInfoData.Id = vmInfoData.Name
			vmInfoData.Uid = common.GenUid()

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
			vmInfoData.Location = vmInfoData.ConnectionConfig.RegionDetail.Location
			vmInfoData.SpecId = vmRequest.SpecId
			vmInfoData.ImageId = vmRequest.ImageId
			vmInfoData.VNetId = vmRequest.VNetId
			vmInfoData.SubnetId = vmRequest.SubnetId
			vmInfoData.SecurityGroupIds = vmRequest.SecurityGroupIds
			vmInfoData.DataDiskIds = vmRequest.DataDiskIds
			vmInfoData.SshKeyId = vmRequest.SshKeyId
			vmInfoData.Description = vmRequest.Description
			vmInfoData.VmUserName = vmRequest.VmUserName
			vmInfoData.VmUserPassword = vmRequest.VmUserPassword
			vmInfoData.RootDiskType = vmRequest.RootDiskType
			vmInfoData.RootDiskSize = vmRequest.RootDiskSize

			vmInfoData.Label = vmRequest.Label

			vmInfoData.CspResourceId = vmRequest.CspResourceId

			wg.Add(1)
			go CreateVmObject(&wg, nsId, mciId, &vmInfoData)
		}
	}
	wg.Wait()

	for _, vmRequest := range vmRequests {
		// subGroup handling
		subGroupSize, err := strconv.Atoi(vmRequest.SubGroupSize)
		if err != nil {
			subGroupSize = 1
		}

		for i := vmStartIndex; i <= subGroupSize+vmStartIndex; i++ {
			vmInfoData := model.TbVmInfo{}

			if subGroupSize == 0 { // for VM (not in a group)
				vmInfoData.Name = common.ToLower(vmRequest.Name)
			} else { // for VM (in a group)
				if i == subGroupSize+vmStartIndex {
					break
				}
				vmInfoData.SubGroupId = common.ToLower(vmRequest.Name)
				vmInfoData.Name = common.ToLower(vmRequest.Name) + "-" + strconv.Itoa(i)
			}
			vmInfoData.Id = vmInfoData.Name
			vmId := vmInfoData.Id
			vmInfoData, err := GetVmObject(nsId, mciId, vmId)
			if err != nil {
				log.Error().Err(err).Msg("")
				return nil, err
			}

			// Avoid concurrent requests to CSP.
			time.Sleep(time.Millisecond * 1000)

			wg.Add(1)
			go CreateVm(&wg, nsId, mciId, &vmInfoData, option)
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
	if strings.Contains(mciTmp.InstallMonAgent, "yes") && option != "register" {

		check := CheckDragonflyEndpoint()
		if check != nil {
			fmt.Printf("\n\n[Warning] CB-Dragonfly is not available\n\n")
		} else {
			reqToMon := &model.MciCmdReq{}
			reqToMon.UserName = "cb-user" // this MCI user name is temporal code. Need to improve.
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

	if len(mciTmp.PostCommand.Command) > 0 {
		log.Info().Msgf("Wait for 5 seconds for a safe bootstrapping.")
		time.Sleep(5 * time.Second)
		log.Info().Msgf("BootstrappingCommand: %+v", mciTmp.PostCommand)
		output, err := RemoteCommandToMci(nsId, mciId, "", "", "", &mciTmp.PostCommand)
		if err != nil {
			log.Error().Err(err).Msg("")
		}
		result := model.MciSshCmdResult{}
		for _, v := range output {
			result.Results = append(result.Results, v)
		}
		common.PrintJsonPretty(result)

		mciTmp.PostCommandResult = result
		UpdateMciInfo(nsId, mciTmp)
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
		err := fmt.Errorf("cannot load ConnectionConfigList in MCI dynamic request check")
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
		availableImageList, err := resource.GetImagesByRegion(model.SystemCommonNs, specInfo.ProviderName, specInfo.RegionName)
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
	labels := map[string]string{
		model.LabelPurpose: option,
	}
	req.Label = labels
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
			vmReq.CommonImage = "ubuntu22.04"                // temporal default value. will be changed
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

				vmReq.Label = labels
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
	mciReq.PostCommand = req.PostCommand

	emptyMci := &model.TbMciInfo{}
	err := common.CheckString(nsId)
	if err != nil {
		err := fmt.Errorf("invalid namespace. %w", err)
		log.Error().Err(err).Msg("")
		return emptyMci, err
	}
	check, err := CheckMci(nsId, req.Name)
	if err != nil {
		err := fmt.Errorf("invalid mci name. %w", err)
		log.Error().Err(err).Msg("")
		return emptyMci, err
	}
	if check {
		err := fmt.Errorf("The mci " + req.Name + " already exists.")
		return emptyMci, err
	}

	vmRequest := req.Vm
	// Check whether VM names meet requirement.
	// Use semaphore for parallel processing with concurrency limit
	const maxConcurrency = 10
	semaphore := make(chan struct{}, maxConcurrency)

	var wg sync.WaitGroup
	var mutex sync.Mutex
	errStr := ""

	for i, k := range vmRequest {
		wg.Add(1)
		go func(index int, vmReq model.TbVmDynamicReq) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }() // Release semaphore

			// log VM request details
			log.Debug().Msgf("[%d] VM Request: %+v", index, vmReq)

			err := checkCommonResAvailableForVmDynamicReq(&vmReq, nsId)
			if err != nil {
				log.Error().Err(err).Msgf("[%d] Failed to find common resource for MCI provision", index)
				mutex.Lock()
				errStr += "{[" + strconv.Itoa(index+1) + "] " + err.Error() + "} "
				mutex.Unlock()
			}
		}(i, k)
	}

	wg.Wait()

	if errStr != "" {
		err = fmt.Errorf(errStr)
		return emptyMci, err
	}

	/*
	 * [NOTE]
	 * 1. Generate default resources first
	 * 2. And then, parallel processing of VM requests
	 */

	// Check if vmRequest has elements
	if len(vmRequest) > 0 {
		var allCreatedResources []CreatedResource
		var wg sync.WaitGroup
		var mutex sync.Mutex

		type vmResult struct {
			result *VmReqWithCreatedResources
			err    error
		}
		resultChan := make(chan vmResult, len(vmRequest))

		// Process all vmRequests in parallel
		for _, k := range vmRequest {
			wg.Add(1)
			go func(vmReq model.TbVmDynamicReq) {
				defer wg.Done()
				result, err := getVmReqFromDynamicReq(reqID, nsId, &vmReq)
				resultChan <- vmResult{result: result, err: err}
			}(k)
		}

		// Wait for all goroutines to complete
		wg.Wait()
		close(resultChan)

		// Collect results and check for errors
		var hasError bool
		for vmRes := range resultChan {
			if vmRes.err != nil {
				log.Error().Err(vmRes.err).Msg("Failed to prepare resources for dynamic MCI creation")
				hasError = true
			} else {
				// Safely append to the shared mciReq.Vm slice
				mutex.Lock()
				mciReq.Vm = append(mciReq.Vm, *vmRes.result.VmReq)
				allCreatedResources = append(allCreatedResources, vmRes.result.CreatedResources...)
				mutex.Unlock()
			}
		}

		// If there were any errors, rollback all created resources
		if hasError {
			log.Info().Msg("Rolling back all created default resources due to errors")
			time.Sleep(5 * time.Second)
			rollbackErr := rollbackCreatedResources(nsId, allCreatedResources)
			if rollbackErr != nil {
				return emptyMci, fmt.Errorf("failed in rollback operation: %w", rollbackErr)
			} else {
				return emptyMci, fmt.Errorf("rollback completed successfully after errors in resource preparation")
			}
		}
	}

	// Log the prepared MCI request and update the progress
	common.PrintJsonPretty(mciReq)
	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{
		Title: "Prepared all resources for provisioning MCI: " + mciReq.Name,
		Info:  mciReq, Time: time.Now(),
	})
	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{
		Title: "Start instance provisioning", Time: time.Now(),
	})

	// Run create MCI with the generated MCI request
	option := "create"
	if deployOption == "hold" {
		option = "hold"
	}
	return CreateMci(nsId, &mciReq, option)
}

// ValidateMciDynamicReq is func to validate MCI dynamic request before actual provisioning
func ValidateMciDynamicReq(reqID string, nsId string, req *model.TbMciDynamicReq, deployOption string) (*model.ReviewMciDynamicReqInfo, error) {
	return ReviewMciDynamicReq(reqID, nsId, req, deployOption)
}

// ReviewMciDynamicReq is func to review and validate MCI dynamic request comprehensively
func ReviewMciDynamicReq(reqID string, nsId string, req *model.TbMciDynamicReq, deployOption string) (*model.ReviewMciDynamicReqInfo, error) {

	log.Debug().Msgf("Starting MCI dynamic request review for: %s", req.Name)

	reviewResult := &model.ReviewMciDynamicReqInfo{
		MciName:      req.Name,
		TotalVmCount: len(req.Vm),
		VmReviews:    make([]model.ReviewVmDynamicReqInfo, 0),
		ResourceSummary: model.ReviewResourceSummary{
			UniqueSpecs:     make([]string, 0),
			UniqueImages:    make([]string, 0),
			ConnectionNames: make([]string, 0),
			ProviderNames:   make([]string, 0),
			RegionNames:     make([]string, 0),
		},
		Recommendations: make([]string, 0),
	}

	// Basic validation
	err := common.CheckString(nsId)
	if err != nil {
		return nil, fmt.Errorf("invalid namespace: %w", err)
	}

	// Check if MCI name is valid and doesn't exist
	check, err := CheckMci(nsId, req.Name)
	if err != nil {
		return nil, fmt.Errorf("invalid mci name: %w", err)
	}
	if check {
		reviewResult.OverallStatus = "Error"
		reviewResult.OverallMessage = fmt.Sprintf("MCI '%s' already exists in namespace '%s'", req.Name, nsId)
		reviewResult.CreationViable = false
		return reviewResult, nil
	}

	if len(req.Vm) == 0 {
		reviewResult.OverallStatus = "Error"
		reviewResult.OverallMessage = "No VM requests provided"
		reviewResult.CreationViable = false
		return reviewResult, nil
	}

	// Track resource summary
	specMap := make(map[string]bool)
	imageMap := make(map[string]bool)
	connectionMap := make(map[string]bool)
	providerMap := make(map[string]bool)
	regionMap := make(map[string]bool)

	// Validate each VM request
	allViable := true
	hasWarnings := false
	totalEstimatedCost := 0.0
	vmWithUnknownCost := 0

	for i, vmReq := range req.Vm {
		vmReview := model.ReviewVmDynamicReqInfo{
			VmName:       vmReq.Name,
			SubGroupSize: vmReq.SubGroupSize,
			CanCreate:    true,
			Status:       "Ready",
			Info:         make([]string, 0),
			Warnings:     make([]string, 0),
			Errors:       make([]string, 0),
		}

		// Validate VM name
		if vmReq.Name == "" {
			vmReview.Warnings = append(vmReview.Warnings, "VM SubGroup name not specified, will be auto-generated")
			hasWarnings = true
		}

		// Validate SubGroupSize
		if vmReq.SubGroupSize == "" {
			vmReq.SubGroupSize = "1"
			vmReview.Warnings = append(vmReview.Warnings, "SubGroupSize not specified, defaulting to 1")
			hasWarnings = true
		}

		// Validate CommonSpec
		specInfo, err := resource.GetSpec(model.SystemCommonNs, vmReq.CommonSpec)
		var specInfoPtr *model.TbSpecInfo
		if err != nil {
			vmReview.Errors = append(vmReview.Errors, fmt.Sprintf("Failed to get spec '%s': %v", vmReq.CommonSpec, err))
			vmReview.SpecValidation = model.ReviewResourceValidation{
				ResourceId:  vmReq.CommonSpec,
				IsAvailable: false,
				Status:      "Unavailable",
				Message:     err.Error(),
			}
			vmReview.CanCreate = false
			allViable = false
		} else {
			specInfoPtr = &specInfo
			vmReview.ConnectionName = specInfo.ConnectionName
			vmReview.ProviderName = specInfo.ProviderName
			vmReview.RegionName = specInfo.RegionName

			// Check if spec is available in CSP
			cspSpec, err := resource.LookupSpec(specInfo.ConnectionName, specInfo.CspSpecName)
			if err != nil {
				vmReview.Errors = append(vmReview.Errors, fmt.Sprintf("Spec '%s' not available in CSP: %v", vmReq.CommonSpec, err))
				vmReview.SpecValidation = model.ReviewResourceValidation{
					ResourceId:    vmReq.CommonSpec,
					ResourceName:  specInfo.CspSpecName,
					IsAvailable:   false,
					Status:        "Unavailable",
					Message:       err.Error(),
					CspResourceId: specInfo.CspSpecName,
				}
				vmReview.CanCreate = false
				allViable = false
			} else {
				vmReview.SpecValidation = model.ReviewResourceValidation{
					ResourceId:    vmReq.CommonSpec,
					ResourceName:  specInfo.CspSpecName,
					IsAvailable:   true,
					Status:        "Available",
					CspResourceId: cspSpec.Name,
				}

				// Add cost estimation if available
				if specInfo.CostPerHour > 0 {
					vmReview.EstimatedCost = fmt.Sprintf("$%.4f/hour", specInfo.CostPerHour)
					totalEstimatedCost += float64(specInfo.CostPerHour)
				} else {
					vmReview.EstimatedCost = "Cost estimation unavailable"
					vmReview.Warnings = append(vmReview.Warnings, "Cost estimation not available for this spec")
					hasWarnings = true
					vmWithUnknownCost++
				}
			}

			// Track resource summary
			specMap[vmReq.CommonSpec] = true
			connectionMap[specInfo.ConnectionName] = true
			providerMap[specInfo.ProviderName] = true
			regionMap[specInfo.RegionName] = true
		}

		// Validate CommonImage
		if specInfoPtr != nil {
			cspImage, err := resource.LookupImage(specInfoPtr.ConnectionName, vmReq.CommonImage)
			if err != nil {
				vmReview.Errors = append(vmReview.Errors, fmt.Sprintf("Image '%s' not available in CSP: %v", vmReq.CommonImage, err))
				vmReview.ImageValidation = model.ReviewResourceValidation{
					ResourceId:    vmReq.CommonImage,
					IsAvailable:   false,
					Status:        "Unavailable",
					Message:       err.Error(),
					CspResourceId: vmReq.CommonImage,
				}
				vmReview.CanCreate = false
				allViable = false
			} else {
				vmReview.ImageValidation = model.ReviewResourceValidation{
					ResourceId:    vmReq.CommonImage,
					ResourceName:  cspImage.Name,
					IsAvailable:   true,
					Status:        "Available",
					CspResourceId: cspImage.IId.SystemId,
				}
			}

			imageMap[vmReq.CommonImage] = true
		}

		// Validate ConnectionName if specified
		if vmReq.ConnectionName != "" {
			_, err := common.GetConnConfig(vmReq.ConnectionName)
			if err != nil {
				vmReview.Warnings = append(vmReview.Warnings, fmt.Sprintf("Specified connection '%s' not found, will use default from spec", vmReq.ConnectionName))
				hasWarnings = true
			} else {
				vmReview.ConnectionName = vmReq.ConnectionName
			}
		}

		// Validate RootDisk settings
		if vmReq.RootDiskType != "" && vmReq.RootDiskType != "default" {
			vmReview.Info = append(vmReview.Info, fmt.Sprintf("Root disk type configured: %s, be sure it's supported by the provider", vmReq.RootDiskType))
		}
		if vmReq.RootDiskSize != "" && vmReq.RootDiskSize != "default" {
			vmReview.Info = append(vmReview.Info, fmt.Sprintf("Root disk size configured: %s GB, be sure it meets minimum requirements", vmReq.RootDiskSize))
		}

		// Set VM review status
		if len(vmReview.Errors) > 0 {
			vmReview.Status = "Error"
			vmReview.Message = fmt.Sprintf("VM has %d error(s) that prevent creation", len(vmReview.Errors))
		} else if len(vmReview.Warnings) > 0 {
			vmReview.Status = "Warning"
			vmReview.Message = fmt.Sprintf("VM can be created but has %d warning(s)", len(vmReview.Warnings))
		} else {
			vmReview.Status = "Ready"
			vmReview.Message = "VM can be created successfully"
		}

		reviewResult.VmReviews = append(reviewResult.VmReviews, vmReview)
		log.Debug().Msgf("[%d] VM '%s' review completed: %s", i, vmReq.Name, vmReview.Status)
	}

	// Build resource summary
	for spec := range specMap {
		reviewResult.ResourceSummary.UniqueSpecs = append(reviewResult.ResourceSummary.UniqueSpecs, spec)
	}
	for image := range imageMap {
		reviewResult.ResourceSummary.UniqueImages = append(reviewResult.ResourceSummary.UniqueImages, image)
	}
	for conn := range connectionMap {
		reviewResult.ResourceSummary.ConnectionNames = append(reviewResult.ResourceSummary.ConnectionNames, conn)
	}
	for provider := range providerMap {
		reviewResult.ResourceSummary.ProviderNames = append(reviewResult.ResourceSummary.ProviderNames, provider)
	}
	for region := range regionMap {
		reviewResult.ResourceSummary.RegionNames = append(reviewResult.ResourceSummary.RegionNames, region)
	}

	reviewResult.ResourceSummary.TotalProviders = len(providerMap)
	reviewResult.ResourceSummary.TotalRegions = len(regionMap)

	// Count available/unavailable resources
	for _, vmReview := range reviewResult.VmReviews {
		if vmReview.SpecValidation.IsAvailable {
			reviewResult.ResourceSummary.AvailableSpecs++
		} else {
			reviewResult.ResourceSummary.UnavailableSpecs++
		}
		if vmReview.ImageValidation.IsAvailable {
			reviewResult.ResourceSummary.AvailableImages++
		} else {
			reviewResult.ResourceSummary.UnavailableImages++
		}
	}

	// Set overall status and cost estimation
	if totalEstimatedCost > 0 {
		if vmWithUnknownCost > 0 {
			reviewResult.EstimatedCost = fmt.Sprintf("$%.4f/hour (partial - %d VMs have unknown costs)", totalEstimatedCost, vmWithUnknownCost)
		} else {
			reviewResult.EstimatedCost = fmt.Sprintf("$%.4f/hour", totalEstimatedCost)
		}
	} else if vmWithUnknownCost > 0 {
		reviewResult.EstimatedCost = fmt.Sprintf("Cost estimation unavailable for all %d VMs", vmWithUnknownCost)
	}

	reviewResult.CreationViable = allViable

	if !allViable {
		reviewResult.OverallStatus = "Error"
		reviewResult.OverallMessage = fmt.Sprintf("MCI cannot be created due to critical errors in VM configurations (Providers: %v, Regions: %v)",
			reviewResult.ResourceSummary.ProviderNames, reviewResult.ResourceSummary.RegionNames)
		reviewResult.Recommendations = append(reviewResult.Recommendations, "Fix all VM configuration errors before attempting to create MCI")
	} else if hasWarnings {
		reviewResult.OverallStatus = "Warning"
		reviewResult.OverallMessage = fmt.Sprintf("MCI can be created but has some configuration warnings (Providers: %v, Regions: %v)",
			reviewResult.ResourceSummary.ProviderNames, reviewResult.ResourceSummary.RegionNames)
		reviewResult.Recommendations = append(reviewResult.Recommendations, "Review and address warnings for optimal configuration")
	} else {
		reviewResult.OverallStatus = "Ready"
		reviewResult.OverallMessage = fmt.Sprintf("All VMs can be created successfully (Providers: %v, Regions: %v)",
			reviewResult.ResourceSummary.ProviderNames, reviewResult.ResourceSummary.RegionNames)
	}

	// Add specific recommendations
	if reviewResult.ResourceSummary.TotalProviders > 3 {
		reviewResult.Recommendations = append(reviewResult.Recommendations, "Consider consolidating to fewer cloud providers to simplify management")
	}
	if reviewResult.ResourceSummary.TotalRegions > 5 {
		reviewResult.Recommendations = append(reviewResult.Recommendations, "Large number of regions may increase latency between VMs")
	}
	if totalEstimatedCost > 10.0 {
		reviewResult.Recommendations = append(reviewResult.Recommendations, "High estimated cost - consider using smaller instance types if appropriate")
	}
	if vmWithUnknownCost > 0 {
		reviewResult.Recommendations = append(reviewResult.Recommendations, fmt.Sprintf("Cost estimation unavailable for %d VMs - actual costs may be higher than shown", vmWithUnknownCost))
	}

	log.Debug().Msgf("MCI review completed: %s - %s", reviewResult.OverallStatus, reviewResult.OverallMessage)
	return reviewResult, nil
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

	err = checkCommonResAvailableForVmDynamicReq(req, nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyMci, err
	}

	vmReqResult, err := getVmReqFromDynamicReq("", nsId, req)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyMci, err
	}

	return CreateMciGroupVm(nsId, mciId, vmReqResult.VmReq, true)
}

// checkCommonResAvailableForVmDynamicReq is func to check common resources availability for VmDynamicReq
func checkCommonResAvailableForVmDynamicReq(req *model.TbVmDynamicReq, nsId string) error {

	log.Debug().Msgf("Checking common resources for VM Dynamic Request: %+v", req)
	log.Debug().Msgf("Namespace ID: %s", nsId)

	specInfo, err := resource.GetSpec(model.SystemCommonNs, req.CommonSpec)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	// check if the spec is available in the CSP
	_, err = resource.LookupSpec(specInfo.ConnectionName, specInfo.CspSpecName)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get the Spec from the CSP")
		return err
	}

	// check if the image is available in the CSP
	_, err = resource.LookupImage(specInfo.ConnectionName, req.CommonImage)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get the Image from the CSP")
		return err
	}

	return err
}

// getVmReqFromDynamicReq is func to getVmReqFromDynamicReq with created resource tracking
func getVmReqFromDynamicReq(reqID string, nsId string, req *model.TbVmDynamicReq) (*VmReqWithCreatedResources, error) {

	onDemand := true
	var createdResources []CreatedResource

	vmRequest := req
	// Check whether VM names meet requirement.
	k := vmRequest

	vmReq := &model.TbVmReq{}

	specInfo, err := resource.GetSpec(model.SystemCommonNs, req.CommonSpec)
	if err != nil {
		log.Error().Err(err).Msg("")
		return &VmReqWithCreatedResources{VmReq: &model.TbVmReq{}, CreatedResources: createdResources}, err
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
		return &VmReqWithCreatedResources{VmReq: &model.TbVmReq{}, CreatedResources: createdResources}, err
	}

	// Default resource name has this pattern (nsId + "-shared-" + vmReq.ConnectionName)
	resourceName := nsId + model.StrSharedResourceName + vmReq.ConnectionName

	vmReq.SpecId = specInfo.Id
	vmReq.ImageId = k.CommonImage

	// check if the image is available in the CSP
	_, err = resource.LookupImage(connection.ConfigName, vmReq.ImageId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get the Image from database as well as the CSP")
		return &VmReqWithCreatedResources{VmReq: &model.TbVmReq{}, CreatedResources: createdResources}, err
	}
	// Need enhancement to handle custom image request

	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Setting vNet:" + resourceName, Time: time.Now()})

	vmReq.VNetId = resourceName
	_, err = resource.GetResource(nsId, model.StrVNet, vmReq.VNetId)
	if err != nil {
		if !onDemand {
			err := fmt.Errorf("Failed to get the vNet " + vmReq.VNetId + " from " + vmReq.ConnectionName)
			log.Error().Err(err).Msg("Failed to get the vNet")
			return &VmReqWithCreatedResources{VmReq: &model.TbVmReq{}, CreatedResources: createdResources}, err
		}
		clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Loading default vNet:" + resourceName, Time: time.Now()})

		// Check if the default vNet exists
		_, err := resource.GetResource(nsId, model.StrVNet, vmReq.ConnectionName)
		log.Debug().Msg("checked if the default vNet does NOT exist")
		// Create a new default vNet if it does not exist
		if err != nil && strings.Contains(err.Error(), "does not exist") {
			err2 := resource.CreateSharedResource(nsId, model.StrVNet, vmReq.ConnectionName)
			if err2 != nil {
				log.Error().Err(err2).Msg("Failed to create new default vNet " + vmReq.VNetId + " from " + vmReq.ConnectionName)
				return &VmReqWithCreatedResources{VmReq: &model.TbVmReq{}, CreatedResources: createdResources}, err2
			} else {
				log.Info().Msg("Created new default vNet: " + vmReq.VNetId)
				// Track the newly created VNet
				createdResources = append(createdResources, CreatedResource{Type: model.StrVNet, Id: vmReq.VNetId})
			}
		}
	} else {
		log.Info().Msg("Found and utilize default vNet: " + vmReq.VNetId)
	}
	vmReq.SubnetId = resourceName

	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Setting SSHKey:" + resourceName, Time: time.Now()})
	vmReq.SshKeyId = resourceName
	_, err = resource.GetResource(nsId, model.StrSSHKey, vmReq.SshKeyId)
	if err != nil {
		if !onDemand {
			err := fmt.Errorf("Failed to get the SSHKey " + vmReq.SshKeyId + " from " + vmReq.ConnectionName)
			log.Error().Err(err).Msg("Failed to get the SSHKey")
			return &VmReqWithCreatedResources{VmReq: &model.TbVmReq{}, CreatedResources: createdResources}, err
		}
		clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Loading default SSHKey:" + resourceName, Time: time.Now()})

		// Check if the default SSHKey exists
		_, err := resource.GetResource(nsId, model.StrSSHKey, vmReq.ConnectionName)
		log.Debug().Msg("checked if the default SSHKey does NOT exist")
		// Create a new default SSHKey if it does not exist
		if err != nil && strings.Contains(err.Error(), "does not exist") {
			err2 := resource.CreateSharedResource(nsId, model.StrSSHKey, vmReq.ConnectionName)
			if err2 != nil {
				log.Error().Err(err2).Msg("Failed to create new default SSHKey " + vmReq.SshKeyId + " from " + vmReq.ConnectionName)
				return &VmReqWithCreatedResources{VmReq: &model.TbVmReq{}, CreatedResources: createdResources}, err2
			} else {
				log.Info().Msg("Created new default SSHKey: " + vmReq.SshKeyId)
				// Track the newly created SSHKey
				createdResources = append(createdResources, CreatedResource{Type: model.StrSSHKey, Id: vmReq.SshKeyId})
			}
		}
	} else {
		log.Info().Msg("Found and utilize default SSHKey: " + vmReq.SshKeyId)
	}

	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Setting securityGroup:" + resourceName, Time: time.Now()})
	securityGroup := resourceName
	vmReq.SecurityGroupIds = append(vmReq.SecurityGroupIds, securityGroup)
	_, err = resource.GetResource(nsId, model.StrSecurityGroup, securityGroup)
	if err != nil {
		if !onDemand {
			err := fmt.Errorf("Failed to get the securityGroup " + securityGroup + " from " + vmReq.ConnectionName)
			log.Error().Err(err).Msg("Failed to get the securityGroup")
			return &VmReqWithCreatedResources{VmReq: &model.TbVmReq{}, CreatedResources: createdResources}, err
		}
		clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Loading default securityGroup:" + resourceName, Time: time.Now()})

		// Check if the default security group exists
		_, err := resource.GetResource(nsId, model.StrSecurityGroup, vmReq.ConnectionName)
		// Create a new default security group if it does not exist
		log.Debug().Msg("checked if the default security group does NOT exist")
		if err != nil && strings.Contains(err.Error(), "does not exist") {
			err2 := resource.CreateSharedResource(nsId, model.StrSecurityGroup, vmReq.ConnectionName)
			if err2 != nil {
				log.Error().Err(err2).Msg("Failed to create new default securityGroup " + securityGroup + " from " + vmReq.ConnectionName)
				return &VmReqWithCreatedResources{VmReq: &model.TbVmReq{}, CreatedResources: createdResources}, err2
			} else {
				log.Info().Msg("Created new default securityGroup: " + securityGroup)
				// Track the newly created SecurityGroup
				createdResources = append(createdResources, CreatedResource{Type: model.StrSecurityGroup, Id: securityGroup})
			}
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
	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Prepared resources for VM:" + vmReq.Name, Info: vmReq, Time: time.Now()})

	return &VmReqWithCreatedResources{VmReq: vmReq, CreatedResources: createdResources}, nil
}

// CreateVmObject is func to add VM to MCI
func CreateVmObject(wg *sync.WaitGroup, nsId string, mciId string, vmInfoData *model.TbVmInfo) error {
	log.Debug().Msg("Start to add VM To MCI")
	//goroutin
	defer wg.Done()

	key := common.GenMciKey(nsId, mciId, "")
	keyValue, err := kvstore.GetKv(key)
	if err != nil {
		log.Fatal().Err(err).Msg("AddVmToMci kvstore.GetKv() returned an error.")
		return err
	}
	if keyValue == (kvstore.KeyValue{}) {
		return fmt.Errorf("AddVmToMci Cannot find mciId. Key: %s", key)
	}

	configTmp, err := common.GetConnConfig(vmInfoData.ConnectionName)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	vmInfoData.Location = configTmp.RegionDetail.Location

	// Make VM object
	key = common.GenMciKey(nsId, mciId, vmInfoData.Id)
	val, _ := json.Marshal(vmInfoData)
	err = kvstore.Put(key, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	return nil
}

// CreateVm is func to create VM (option = "register" for register existing VM)
func CreateVm(wg *sync.WaitGroup, nsId string, mciId string, vmInfoData *model.TbVmInfo, option string) error {
	log.Info().Msgf("Start to create VM: %s", vmInfoData.Name)
	//goroutin
	defer wg.Done()

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
		vmInfoData.Status = model.StatusFailed
		vmInfoData.SystemMessage = err.Error()
		UpdateVmInfo(nsId, mciId, *vmInfoData)
		log.Error().Err(err).Msg("")
		return err
	}

	vmKey := common.GenMciKey(nsId, mciId, vmInfoData.Id)

	// in case of registering existing CSP VM
	if option == "register" {
		// CspResourceId is required
		if vmInfoData.CspResourceId == "" {
			err := fmt.Errorf("vmInfoData.CspResourceId is empty (required for register VM)")
			vmInfoData.Status = model.StatusFailed
			vmInfoData.SystemMessage = err.Error()
			UpdateVmInfo(nsId, mciId, *vmInfoData)
			log.Error().Err(err).Msg("")
			return err
		}
	}

	var callResult model.SpiderVMInfo

	// Fill VM creation reqest (request to cb-spider)
	requestBody := model.SpiderVMReqInfoWrapper{}
	requestBody.ConnectionName = vmInfoData.ConnectionName

	//generate VM ID(Name) to request to CSP(Spider)
	requestBody.ReqInfo.Name = vmInfoData.Uid

	customImageFlag := false

	requestBody.ReqInfo.VMUserId = vmInfoData.VmUserName
	requestBody.ReqInfo.VMUserPasswd = vmInfoData.VmUserPassword
	// provide a random passwd, if it is not provided by user (the passwd required for Windows)
	if requestBody.ReqInfo.VMUserPasswd == "" {
		// assign random string (mixed Uid style)
		requestBody.ReqInfo.VMUserPasswd = common.GenRandomPassword(14)
	}

	requestBody.ReqInfo.RootDiskType = vmInfoData.RootDiskType
	requestBody.ReqInfo.RootDiskSize = vmInfoData.RootDiskSize

	if option == "register" {
		requestBody.ReqInfo.CSPid = vmInfoData.CspResourceId

	} else {
		// Try lookup customImage
		requestBody.ReqInfo.ImageName, err = resource.GetCspResourceName(nsId, model.StrCustomImage, vmInfoData.ImageId)
		if requestBody.ReqInfo.ImageName == "" || err != nil {
			log.Debug().Msgf("Not found %s from CustomImage in ns: %s, Use the ImageName directly", vmInfoData.ImageId, nsId)
			// If the image is not a custom image, use the requested image name directly
			requestBody.ReqInfo.ImageName = vmInfoData.ImageId
		} else {
			customImageFlag = true
			requestBody.ReqInfo.ImageType = model.MyImage
			// If the requested image is a custom image (generated by VM snapshot), RootDiskType should be empty.
			// TB ignore inputs for RootDiskType, RootDiskSize
			requestBody.ReqInfo.RootDiskType = ""
			requestBody.ReqInfo.RootDiskSize = ""
		}

		requestBody.ReqInfo.VMSpecName, err = resource.GetCspResourceName(nsId, model.StrSpec, vmInfoData.SpecId)
		if requestBody.ReqInfo.VMSpecName == "" || err != nil {
			log.Warn().Msgf("Not found the Spec: %s in nsId: %s, find it from SystemCommonNs", vmInfoData.SpecId, nsId)
			errAgg := err.Error()
			// If cannot find the resource, use common resource
			requestBody.ReqInfo.VMSpecName, err = resource.GetCspResourceName(model.SystemCommonNs, model.StrSpec, vmInfoData.SpecId)
			log.Info().Msgf("Use the common VMSpecName: %s", requestBody.ReqInfo.VMSpecName)

			if requestBody.ReqInfo.VMSpecName == "" || err != nil {
				errAgg += err.Error()
				err = fmt.Errorf(errAgg)

				vmInfoData.Status = model.StatusFailed
				vmInfoData.SystemMessage = err.Error()
				UpdateVmInfo(nsId, mciId, *vmInfoData)
				log.Error().Err(err).Msg("")

				return err
			}
		}

		requestBody.ReqInfo.VPCName, err = resource.GetCspResourceName(nsId, model.StrVNet, vmInfoData.VNetId)
		if requestBody.ReqInfo.VPCName == "" {
			log.Error().Err(err).Msg("")
			return err
		}

		// retrieve csp subnet id
		subnetInfo, err := resource.GetSubnet(nsId, vmInfoData.VNetId, vmInfoData.SubnetId)
		if err != nil {
			log.Error().Err(err).Msg("Cannot find the Subnet ID: " + vmInfoData.SubnetId)
			vmInfoData.Status = model.StatusFailed
			vmInfoData.SystemMessage = err.Error()
			UpdateVmInfo(nsId, mciId, *vmInfoData)
			return err
		}

		requestBody.ReqInfo.SubnetName = subnetInfo.CspResourceName
		if requestBody.ReqInfo.SubnetName == "" {
			vmInfoData.Status = model.StatusFailed
			vmInfoData.SystemMessage = err.Error()
			UpdateVmInfo(nsId, mciId, *vmInfoData)
			log.Error().Err(err).Msg("")
			return err
		}

		var SecurityGroupIdsTmp []string
		for _, v := range vmInfoData.SecurityGroupIds {
			CspResourceId, err := resource.GetCspResourceName(nsId, model.StrSecurityGroup, v)
			if CspResourceId == "" {
				vmInfoData.Status = model.StatusFailed
				vmInfoData.SystemMessage = err.Error()
				UpdateVmInfo(nsId, mciId, *vmInfoData)
				log.Error().Err(err).Msg("")
				return err
			}

			SecurityGroupIdsTmp = append(SecurityGroupIdsTmp, CspResourceId)
		}
		requestBody.ReqInfo.SecurityGroupNames = SecurityGroupIdsTmp

		var DataDiskIdsTmp []string
		for _, v := range vmInfoData.DataDiskIds {
			// ignore DataDiskIds == "", assume it is ignorable mistake
			if v != "" {
				CspResourceId, err := resource.GetCspResourceName(nsId, model.StrDataDisk, v)
				if err != nil || CspResourceId == "" {
					vmInfoData.Status = model.StatusFailed
					vmInfoData.SystemMessage = err.Error()
					UpdateVmInfo(nsId, mciId, *vmInfoData)
					log.Error().Err(err).Msg("")
					return err
				}
				DataDiskIdsTmp = append(DataDiskIdsTmp, CspResourceId)
			}
		}
		requestBody.ReqInfo.DataDiskNames = DataDiskIdsTmp

		requestBody.ReqInfo.KeyPairName, err = resource.GetCspResourceName(nsId, model.StrSSHKey, vmInfoData.SshKeyId)
		if requestBody.ReqInfo.KeyPairName == "" {
			vmInfoData.Status = model.StatusFailed
			vmInfoData.SystemMessage = err.Error()
			UpdateVmInfo(nsId, mciId, *vmInfoData)
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
		err = fmt.Errorf("%v", err)
		vmInfoData.Status = model.StatusFailed
		vmInfoData.SystemMessage = err.Error()
		UpdateVmInfo(nsId, mciId, *vmInfoData)
		log.Error().Err(err).Msg("")
		return err
	}

	vmInfoData.AddtionalDetails = callResult.KeyValueList
	vmInfoData.VmUserName = callResult.VMUserId
	vmInfoData.VmUserPassword = callResult.VMUserPasswd
	vmInfoData.CspResourceName = callResult.IId.NameId
	vmInfoData.CspResourceId = callResult.IId.SystemId
	vmInfoData.Region = callResult.Region
	vmInfoData.PublicIP = callResult.PublicIP
	vmInfoData.SSHPort, _ = TrimIP(callResult.SSHAccessPoint)
	vmInfoData.PublicDNS = callResult.PublicDNS
	vmInfoData.PrivateIP = callResult.PrivateIP
	vmInfoData.PrivateDNS = callResult.PrivateDNS
	vmInfoData.RootDiskType = callResult.RootDiskType
	vmInfoData.RootDiskSize = callResult.RootDiskSize
	vmInfoData.RootDiskName = callResult.RootDiskName
	vmInfoData.NetworkInterface = callResult.NetworkInterface

	vmInfoData.CspSpecName = callResult.VMSpecName
	vmInfoData.CspImageName = callResult.ImageIId.SystemId
	vmInfoData.CspVNetId = callResult.VpcIID.SystemId
	vmInfoData.CspSubnetId = callResult.SubnetIID.SystemId
	vmInfoData.CspSshKeyId = callResult.KeyPairIId.SystemId

	if option == "register" {

		// Reconstuct resource IDs
		// vNet
		resourceListInNs, err := resource.ListResource(nsId, model.StrVNet, "cspResourceName", callResult.VpcIID.NameId)
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
		resourceListInNs, err = resource.ListResource(nsId, model.StrSSHKey, "cspResourceName", callResult.KeyPairIId.NameId)
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
			CspResourceId:  v.SystemId,
		}

		dataDisk, err := resource.CreateDataDisk(nsId, &tbDataDiskReq, "register")
		if err != nil {
			err = fmt.Errorf("after starting VM %s, failed to register dataDisk %s. \n", vmInfoData.Name, v.NameId)
			log.Err(err).Msg("")
		}

		vmInfoData.DataDiskIds = append(vmInfoData.DataDiskIds, dataDisk.Id)

		resource.UpdateAssociatedObjectList(nsId, model.StrDataDisk, dataDisk.Id, model.StrAdd, vmKey)
	}

	// Assign a Bastion if none (randomly)
	_, err = SetBastionNodes(nsId, mciId, vmInfoData.Id, "")
	if err != nil {
		// just log error and continue
		log.Debug().Msg(err.Error())
	}

	// set initial TargetAction, TargetStatus
	vmInfoData.TargetAction = model.ActionComplete
	vmInfoData.TargetStatus = model.StatusComplete

	// get and set current vm status
	vmStatusInfoTmp, err := FetchVmStatus(nsId, mciId, vmInfoData.Id)

	if err != nil {
		err = fmt.Errorf("cannot Fetch Vm Status from CSP: %v", err)
		vmInfoData.Status = model.StatusFailed
		vmInfoData.SystemMessage = err.Error()
		UpdateVmInfo(nsId, mciId, *vmInfoData)

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
		model.LabelManager:         model.StrManager,
		model.LabelNamespace:       nsId,
		model.LabelLabelType:       model.StrVM,
		model.LabelId:              vmInfoData.Id,
		model.LabelName:            vmInfoData.Name,
		model.LabelUid:             vmInfoData.Uid,
		model.LabelCspResourceId:   vmInfoData.CspResourceId,
		model.LabelCspResourceName: vmInfoData.CspResourceName,
		model.LabelSubGroupId:      vmInfoData.SubGroupId,
		model.LabelMciId:           mciId,
		model.LabelCreatedTime:     vmInfoData.CreatedTime,
		model.LabelConnectionName:  vmInfoData.ConnectionName,
	}
	for key, value := range vmInfoData.Label {
		labels[key] = value
	}
	err = label.CreateOrUpdateLabel(model.StrVM, vmInfoData.Uid, vmKey, labels)
	if err != nil {
		err = fmt.Errorf("cannot create label object: %v", err)
		vmInfoData.Status = model.StatusFailed
		vmInfoData.SystemMessage = err.Error()
		UpdateVmInfo(nsId, mciId, *vmInfoData)

		log.Error().Err(err).Msg("")
	}

	return nil
}

func filterCheckMciDynamicReqInfoToCheckK8sClusterDynamicReqInfo(mciDReqInfo *model.CheckMciDynamicReqInfo) *model.CheckK8sClusterDynamicReqInfo {
	k8sDReqInfo := model.CheckK8sClusterDynamicReqInfo{}

	if mciDReqInfo != nil {
		for _, k := range mciDReqInfo.ReqCheck {
			if strings.Contains(k.Spec.InfraType, model.StrK8s) ||
				strings.Contains(k.Spec.InfraType, model.StrKubernetes) {

				imageListForK8s := []model.TbImageInfo{}
				for _, i := range k.Image {
					if strings.Contains(i.InfraType, model.StrK8s) ||
						strings.Contains(i.InfraType, model.StrKubernetes) {
						imageListForK8s = append(imageListForK8s, i)
					}
				}

				nodeDReqInfo := model.CheckNodeDynamicReqInfo{
					ConnectionConfigCandidates: k.ConnectionConfigCandidates,
					Spec:                       k.Spec,
					Region:                     k.Region,
					SystemMessage:              k.SystemMessage,
				}

				if len(imageListForK8s) > 0 {
					nodeDReqInfo.Image = imageListForK8s
				} else {
					// No available image because some CSP(ex. azure) can not specify an image
					nodeDReqInfo.Image = []model.TbImageInfo{{Id: "default", Name: "default"}}
				}

				k8sDReqInfo.ReqCheck = append(k8sDReqInfo.ReqCheck, nodeDReqInfo)
			}
		}
	}

	return &k8sDReqInfo
}

// CheckK8sClusterDynamicReq is func to check request info to create K8sCluster obeject and deploy requested Nodes in a dynamic way
func CheckK8sClusterDynamicReq(req *model.K8sClusterConnectionConfigCandidatesReq) (*model.CheckK8sClusterDynamicReqInfo, error) {
	if len(req.CommonSpecs) != 1 {
		err := fmt.Errorf("Only one CommonSpec should be defined.")
		log.Error().Err(err).Msg("")
		return &model.CheckK8sClusterDynamicReqInfo{}, err
	}

	mciCCCReq := model.MciConnectionConfigCandidatesReq{
		CommonSpecs: req.CommonSpecs,
	}
	mciDReqInfo, err := CheckMciDynamicReq(&mciCCCReq)

	k8sDReqInfo := filterCheckMciDynamicReqInfoToCheckK8sClusterDynamicReqInfo(mciDReqInfo)

	return k8sDReqInfo, err
}

func getK8sRecommendVersion(providerName, regionName, reqVersion string) (string, error) {
	availableVersion, err := common.GetAvailableK8sVersion(providerName, regionName)
	if err != nil {
		err := fmt.Errorf("No available K8sCluster version.")
		log.Error().Err(err).Msg("")
		return "", err
	}

	recVersion := model.StrEmpty
	versionIdList := []string{}

	if reqVersion == "" {
		for _, verDetail := range *availableVersion {
			versionIdList = append(versionIdList, verDetail.Id)
			filteredRecVersion := common.FilterDigitsAndDots(recVersion)
			filteredAvailVersion := common.FilterDigitsAndDots(verDetail.Id)
			if common.CompareVersions(filteredRecVersion, filteredAvailVersion) < 0 {
				recVersion = verDetail.Id
			}
		}
	} else {
		for _, verDetail := range *availableVersion {
			versionIdList = append(versionIdList, verDetail.Id)
			if strings.EqualFold(reqVersion, verDetail.Id) {
				recVersion = verDetail.Id
				break
			} else {
				availVersion := common.FilterDigitsAndDots(verDetail.Id)
				filteredReqVersion := common.FilterDigitsAndDots(reqVersion)
				if strings.HasPrefix(availVersion, filteredReqVersion) {
					recVersion = availVersion
					break
				}
			}
		}
	}

	if strings.EqualFold(recVersion, model.StrEmpty) {
		return "", fmt.Errorf("Available K8sCluster Version(k8sclusterinfo.yaml) for Provider/Region(%s/%s): %s",
			providerName, regionName, strings.Join(versionIdList, ", "))
	}

	return recVersion, nil
}

// checkCommonResAvailableForK8sClusterDynamicReq is func to check common resources availability for K8sClusterDynamicReq
func checkCommonResAvailableForK8sClusterDynamicReq(dReq *model.TbK8sClusterDynamicReq) error {
	specInfo, err := resource.GetSpec(model.SystemCommonNs, dReq.CommonSpec)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	connName := specInfo.ConnectionName
	// If ConnectionName is specified by the request, Use ConnectionName from the request
	if dReq.ConnectionName != "" {
		connName = dReq.ConnectionName
	}

	// validate the GetConnConfig for spec
	connConfig, err := common.GetConnConfig(connName)
	if err != nil {
		err := fmt.Errorf("Failed to get ConnectionName (" + connName + ") for Spec (" + dReq.CommonSpec + ") is not found.")
		log.Error().Err(err).Msg("")
		return err
	}

	niDesignation, err := common.GetK8sNodeImageDesignation(connConfig.ProviderName)
	if err != nil {
		log.Error().Err(err).Msg("")
	}

	if niDesignation == false {
		// if node image designation is not supported by CSP, CommonImage should be "default" or ""(blank)
		if !(strings.EqualFold(dReq.CommonImage, "default") || strings.EqualFold(dReq.CommonImage, "")) {
			err := fmt.Errorf("The NodeImageDesignation is not supported by CSP(%s). CommonImage's value should be \"default\" or \"\"", connConfig.ProviderName)
			log.Error().Err(err).Msg("")
			return err
		}
	}

	// In K8sCluster, allows dReq.CommonImage to be set to "default" or ""
	if strings.EqualFold(dReq.CommonImage, "default") ||
		strings.EqualFold(dReq.CommonImage, "") {
		// do nothing
	} else {

		// check if the image is available in the CSP
		_, err = resource.LookupImage(dReq.ConnectionName, dReq.CommonImage)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get the Image from the CSP")
			return err
		}

	}

	return nil
}

// checkCommonResAvailableForK8sNodeGroupDynamicReq is func to check common resources availability for K8sNodeGroupDynamicReq
func checkCommonResAvailableForK8sNodeGroupDynamicReq(connName string, dReq *model.TbK8sNodeGroupDynamicReq) error {
	k8sClusterDReq := &model.TbK8sClusterDynamicReq{
		CommonSpec:     dReq.CommonSpec,
		CommonImage:    dReq.CommonImage,
		ConnectionName: connName,
	}

	err := checkCommonResAvailableForK8sClusterDynamicReq(k8sClusterDReq)
	if err != nil {
		return err
	}

	return nil
}

// getK8sClusterReqFromDynamicReq is func to get TbK8sClusterReq from TbK8sClusterDynamicReq
func getK8sClusterReqFromDynamicReq(reqID string, nsId string, dReq *model.TbK8sClusterDynamicReq) (*model.TbK8sClusterReq, error) {
	onDemand := true

	emptyK8sReq := &model.TbK8sClusterReq{}
	k8sReq := &model.TbK8sClusterReq{}
	k8sngReq := &model.TbK8sNodeGroupReq{}

	specInfo, err := resource.GetSpec(model.SystemCommonNs, dReq.CommonSpec)
	if err != nil {
		log.Err(err).Msg("")
		return emptyK8sReq, err
	}
	k8sngReq.SpecId = specInfo.Id

	k8sRecVersion, err := getK8sRecommendVersion(specInfo.ProviderName, specInfo.RegionName, dReq.Version)
	if err != nil {
		log.Err(err).Msg("")
		return emptyK8sReq, err
	}

	// If ConnectionName is specified by the request, Use ConnectionName from the request
	k8sReq.ConnectionName = specInfo.ConnectionName
	if dReq.ConnectionName != "" {
		k8sReq.ConnectionName = dReq.ConnectionName
	}

	// validate the GetConnConfig for spec
	connection, err := common.GetConnConfig(k8sReq.ConnectionName)
	if err != nil {
		err := fmt.Errorf("Failed to Get ConnectionName (" + k8sReq.ConnectionName + ") for Spec (" + dReq.CommonSpec + ") is not found.")
		log.Err(err).Msg("")
		return emptyK8sReq, err
	}

	k8sNgOnCreation, err := common.GetK8sNodeGroupsOnK8sCreation(connection.ProviderName)
	if err != nil {
		log.Err(err).Msgf("Failed to Get Nodegroups on K8sCluster Creation")
		return emptyK8sReq, err
	}

	// In K8sCluster, allows dReq.CommonImage to be set to "default" or ""
	if strings.EqualFold(dReq.CommonImage, "default") ||
		strings.EqualFold(dReq.CommonImage, "") {
		// do nothing
	} else {

		// check if the image is available in the CSP
		_, err = resource.LookupImage(dReq.ConnectionName, dReq.CommonImage)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get the Image from the CSP")
			return emptyK8sReq, err
		}

	}

	// Default resource name has this pattern (nsId + "-shared-" + vmReq.ConnectionName)
	resourceName := nsId + model.StrSharedResourceName + k8sReq.ConnectionName

	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Setting vNet:" + resourceName, Time: time.Now()})

	k8sReq.VNetId = resourceName
	_, err = resource.GetResource(nsId, model.StrVNet, k8sReq.VNetId)
	if err != nil {
		if !onDemand {
			err := fmt.Errorf("Failed to get the vNet " + k8sReq.VNetId + " from " + k8sReq.ConnectionName)
			log.Err(err).Msg("Failed to get the vNet")
			return emptyK8sReq, err
		}

		clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Loading default vNet:" + resourceName, Time: time.Now()})

		err2 := resource.CreateSharedResource(nsId, model.StrVNet, k8sReq.ConnectionName)
		if err2 != nil {
			log.Err(err2).Msg("Failed to create new default vNet " + k8sReq.VNetId + " from " + k8sReq.ConnectionName)
			return emptyK8sReq, err2
		} else {
			log.Info().Msg("Created new default vNet: " + k8sReq.VNetId)
		}
	} else {
		log.Info().Msg("Found and utilize default vNet: " + k8sReq.VNetId)
	}
	k8sReq.SubnetIds = append(k8sReq.SubnetIds, resourceName)
	k8sReq.SubnetIds = append(k8sReq.SubnetIds, resourceName+"-01")

	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Setting SSHKey:" + resourceName, Time: time.Now()})

	k8sngReq.SshKeyId = resourceName
	_, err = resource.GetResource(nsId, model.StrSSHKey, k8sngReq.SshKeyId)
	if err != nil {
		if !onDemand {
			err := fmt.Errorf("Failed to get the SSHKey " + k8sngReq.SshKeyId + " from " + k8sReq.ConnectionName)
			log.Err(err).Msg("Failed to get the SSHKey")
			return emptyK8sReq, err
		}

		clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Loading default SSHKey:" + resourceName, Time: time.Now()})

		err2 := resource.CreateSharedResource(nsId, model.StrSSHKey, k8sReq.ConnectionName)
		if err2 != nil {
			log.Err(err2).Msg("Failed to create new default SSHKey " + k8sngReq.SshKeyId + " from " + k8sReq.ConnectionName)
			return emptyK8sReq, err2
		} else {
			log.Info().Msg("Created new default SSHKey: " + k8sngReq.SshKeyId)
		}
	} else {
		log.Info().Msg("Found and utilize default SSHKey: " + k8sngReq.SshKeyId)
	}

	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Setting securityGroup:" + resourceName, Time: time.Now()})

	securityGroup := resourceName
	k8sReq.SecurityGroupIds = append(k8sReq.SecurityGroupIds, securityGroup)
	_, err = resource.GetResource(nsId, model.StrSecurityGroup, securityGroup)
	if err != nil {
		if !onDemand {
			err := fmt.Errorf("Failed to get the securityGroup " + securityGroup + " from " + k8sReq.ConnectionName)
			log.Err(err).Msg("Failed to get the securityGroup")
			return emptyK8sReq, err
		}

		clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Loading default securityGroup:" + resourceName, Time: time.Now()})

		err2 := resource.CreateSharedResource(nsId, model.StrSecurityGroup, k8sReq.ConnectionName)
		if err2 != nil {
			log.Err(err2).Msg("Failed to create new default securityGroup " + securityGroup + " from " + k8sReq.ConnectionName)
			return emptyK8sReq, err2
		} else {
			log.Info().Msg("Created new default securityGroup: " + securityGroup)
		}
	} else {
		log.Info().Msg("Found and utilize default securityGroup: " + securityGroup)
	}

	k8sngReq.Name = dReq.NodeGroupName
	if k8sngReq.Name == "" {
		k8sngReq.Name = common.GenUid()
	}
	k8sngReq.RootDiskType = dReq.RootDiskType
	k8sngReq.RootDiskSize = dReq.RootDiskSize
	k8sngReq.OnAutoScaling = dReq.OnAutoScaling
	if k8sngReq.OnAutoScaling == "" {
		k8sngReq.OnAutoScaling = "true"
	}
	k8sngReq.DesiredNodeSize = dReq.DesiredNodeSize
	if k8sngReq.DesiredNodeSize == "" {
		k8sngReq.DesiredNodeSize = "1"
	}
	k8sngReq.MinNodeSize = dReq.MinNodeSize
	if k8sngReq.MinNodeSize == "" {
		k8sngReq.MinNodeSize = "1"
	}
	k8sngReq.MaxNodeSize = dReq.MaxNodeSize
	if k8sngReq.MaxNodeSize == "" {
		k8sngReq.MaxNodeSize = "2"
	}
	k8sReq.Description = dReq.Description
	k8sReq.Name = dReq.Name
	if k8sReq.Name == "" {
		k8sReq.Name = common.GenUid()
	}
	k8sReq.Version = k8sRecVersion
	if k8sNgOnCreation {
		k8sReq.K8sNodeGroupList = append(k8sReq.K8sNodeGroupList, *k8sngReq)
	} else {
		log.Info().Msg("Need to Add NodeGroups To Use This K8sCluster")
	}
	k8sReq.Label = dReq.Label

	common.PrintJsonPretty(k8sReq)
	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Prepared resources for K8sCluster:" + k8sReq.Name, Info: k8sReq, Time: time.Now()})

	return k8sReq, nil
}

// CreateK8sClusterDynamic is func to create K8sCluster obeject and deploy requested K8sCluster and NodeGroup in a dynamic way
func CreateK8sClusterDynamic(reqID string, nsId string, dReq *model.TbK8sClusterDynamicReq, deployOption string) (*model.TbK8sClusterInfo, error) {
	emptyK8sCluster := &model.TbK8sClusterInfo{}
	err := common.CheckString(nsId)
	if err != nil {
		log.Err(err).Msg("")
		return emptyK8sCluster, err
	}
	check, err := resource.CheckK8sCluster(nsId, dReq.Name)
	if err != nil {
		log.Err(err).Msg("")
		return emptyK8sCluster, err
	}
	if check {
		err := fmt.Errorf("already exists")
		log.Err(err).Msgf("Failed to Create K8sCluster(%s) Dynamically", dReq.Name)
		return emptyK8sCluster, err
	}

	err = checkCommonResAvailableForK8sClusterDynamicReq(dReq)
	if err != nil {
		log.Err(err).Msgf("Failed to find common resource for K8sCluster provision")
		return emptyK8sCluster, err
	}

	//If not, generate default resources dynamically.
	k8sReq, err := getK8sClusterReqFromDynamicReq(reqID, nsId, dReq)
	if err != nil {
		log.Err(err).Msg("Failed to get shared resources for dynamic K8sCluster creation")
		return emptyK8sCluster, err
	}
	/*
		  FIXME: need to improve a rollback process
			if err != nil {
				log.Err(err).Msg("Failed to prefare resources for dynamic K8sCluster creation")
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
				return emptyK8sCluster, err
			}
	*/

	common.PrintJsonPretty(k8sReq)
	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Prepared all resources for provisioning K8sCluster:" + k8sReq.Name, Info: k8sReq, Time: time.Now()})
	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Start provisioning", Time: time.Now()})

	// Run create K8sCluster with the generated K8sCluster request (option != register)
	option := "create"
	if deployOption == "hold" {
		option = "hold"
	}

	return resource.CreateK8sCluster(nsId, k8sReq, option)
}

// getK8sNodeGroupReqFromDynamicReq is func to get TbK8sNodeGroupReq from TbK8sNodeGroupDynamicReq
func getK8sNodeGroupReqFromDynamicReq(reqID string, nsId string, k8sClusterInfo *model.TbK8sClusterInfo, dReq *model.TbK8sNodeGroupDynamicReq) (*model.TbK8sNodeGroupReq, error) {
	emptyK8sNgReq := &model.TbK8sNodeGroupReq{}
	k8sNgReq := &model.TbK8sNodeGroupReq{}

	specInfo, err := resource.GetSpec(model.SystemCommonNs, dReq.CommonSpec)
	if err != nil {
		log.Err(err).Msg("")
		return emptyK8sNgReq, err
	}
	k8sNgReq.SpecId = specInfo.Id

	// If ConnectionName for K8sNodeGroup must be same as ConnectionName for K8sCluster
	if specInfo.ConnectionName != k8sClusterInfo.ConnectionName {
		err := fmt.Errorf("ConnectionName(" + specInfo.ConnectionName + ") of K8sNodeGroup Must Match ConnectionName(" + k8sClusterInfo.ConnectionName + ") of K8sCluster")
		log.Err(err).Msg("")
		return emptyK8sNgReq, err
	}

	// In K8sNodeGroup, allows dReq.CommonImage to be set to "default" or ""
	if strings.EqualFold(dReq.CommonImage, "default") ||
		strings.EqualFold(dReq.CommonImage, "") {
		// do nothing
	} else {
		// check if the image is available in the CSP
		_, err = resource.LookupImage(k8sClusterInfo.ConnectionName, dReq.CommonImage)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get the Image from the CSP")
			return emptyK8sNgReq, err
		}
	}

	// Default resource name has this pattern (nsId + "-shared-" + vmReq.ConnectionName)
	resourceName := nsId + model.StrSharedResourceName + k8sClusterInfo.ConnectionName

	k8sNgReq.SshKeyId = resourceName
	_, err = resource.GetResource(nsId, model.StrSSHKey, k8sNgReq.SshKeyId)
	if err != nil {
		err := fmt.Errorf("Failed to get the SSHKey " + k8sNgReq.SshKeyId + " from " + k8sClusterInfo.ConnectionName)
		log.Err(err).Msg("Failed to get the SSHKey")
		return emptyK8sNgReq, err
	} else {
		log.Info().Msg("Found and utilize default SSHKey: " + k8sNgReq.SshKeyId)
	}

	k8sNgReq.Name = dReq.Name
	if k8sNgReq.Name == "" {
		k8sNgReq.Name = common.GenUid()
	}
	k8sNgReq.RootDiskType = dReq.RootDiskType
	k8sNgReq.RootDiskSize = dReq.RootDiskSize
	k8sNgReq.OnAutoScaling = dReq.OnAutoScaling
	if k8sNgReq.OnAutoScaling == "" {
		k8sNgReq.OnAutoScaling = "true"
	}
	k8sNgReq.DesiredNodeSize = dReq.DesiredNodeSize
	if k8sNgReq.DesiredNodeSize == "" {
		k8sNgReq.DesiredNodeSize = "1"
	}
	k8sNgReq.MinNodeSize = dReq.MinNodeSize
	if k8sNgReq.MinNodeSize == "" {
		k8sNgReq.MinNodeSize = "1"
	}
	k8sNgReq.MaxNodeSize = dReq.MaxNodeSize
	if k8sNgReq.MaxNodeSize == "" {
		k8sNgReq.MaxNodeSize = "2"
	}
	k8sNgReq.Description = dReq.Description
	k8sNgReq.Label = dReq.Label

	common.PrintJsonPretty(k8sNgReq)
	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Prepared resources for K8sNodeGroup:" + k8sNgReq.Name, Info: k8sNgReq, Time: time.Now()})

	return k8sNgReq, nil
}

// CreateK8sNodeGroupDynamic is func to create K8sNodeGroup obeject and deploy requested K8sNodeGroup in a dynamic way
func CreateK8sNodeGroupDynamic(reqID string, nsId string, k8sClusterId string, dReq *model.TbK8sNodeGroupDynamicReq) (*model.TbK8sClusterInfo, error) {
	log.Debug().Msgf("reqID: %s, nsId: %s, k8sClusterId: %s, dReq: %v\n", reqID, nsId, k8sClusterId, dReq)

	emptyK8sCluster := &model.TbK8sClusterInfo{}

	check, err := resource.CheckK8sCluster(nsId, k8sClusterId)
	if err != nil {
		log.Err(err).Msg("")
		return emptyK8sCluster, err
	}
	if !check {
		err := fmt.Errorf("K8sCluster(%s) is not existed", k8sClusterId)
		log.Err(err).Msgf("Failed to Create K8sNodeGroup(%s) in K8sCluster(%s) Dynamically", dReq.Name, k8sClusterId)
		return emptyK8sCluster, err
	}

	tbK8sCInfo, err := resource.GetK8sCluster(nsId, k8sClusterId)
	if err != nil {
		log.Err(err).Msgf("Failed to Create K8sNodeGroup(%s) in K8sCluster(%s) Dynamically", dReq.Name, k8sClusterId)
		return emptyK8sCluster, err
	}

	if tbK8sCInfo.Status != model.TbK8sClusterActive {
		err := fmt.Errorf("K8sCluster(%s) is not active status", k8sClusterId)
		log.Err(err).Msgf("Failed to Create K8sNodeGroup(%s) in K8sCluster(%s) Dynamically", dReq.Name, k8sClusterId)
		return emptyK8sCluster, err
	}

	for _, ngi := range tbK8sCInfo.K8sNodeGroupList {
		if ngi.Name == dReq.Name {
			err := fmt.Errorf("K8sNodeGroup(%s) already exists", dReq.Name)
			log.Err(err).Msgf("Failed to Create K8sNodeGroup(%s) in K8sCluster(%s) Dynamically", dReq.Name, k8sClusterId)
			return emptyK8sCluster, err
		}
	}

	err = checkCommonResAvailableForK8sNodeGroupDynamicReq(tbK8sCInfo.ConnectionName, dReq)
	if err != nil {
		log.Err(err).Msgf("Failed to find common resource for K8sNodeGroup provision")
		return emptyK8sCluster, err
	}

	k8sNgReq, err := getK8sNodeGroupReqFromDynamicReq(reqID, nsId, tbK8sCInfo, dReq)
	if err != nil {
		log.Err(err).Msg("Failed to get shared resources for dynamic K8sNodeGroup creation")
		return emptyK8sCluster, err
	}

	common.PrintJsonPretty(k8sNgReq)
	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Prepared all resources for provisioning K8sNodeGroup:" + k8sNgReq.Name, Info: k8sNgReq, Time: time.Now()})
	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Start provisioning", Time: time.Now()})

	return resource.AddK8sNodeGroup(nsId, k8sClusterId, k8sNgReq)
}
