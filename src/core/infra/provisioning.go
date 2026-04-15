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
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
	azure "github.com/cloud-barista/cb-tumblebug/src/core/csp/azure"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	validator "github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"
)

// getVmCreateRateLimitsForCSP returns rate limiting configuration for VM creation.
// Uses centralized CSP config from csp.GetRateLimitConfig() with built-in fallback for unknown CSPs.
func getVmCreateRateLimitsForCSP(cspName string) (int, int) {
	config := csp.GetRateLimitConfig(cspName)
	return config.MaxConcurrentRegions, config.MaxVMsPerRegion
}

// InfraReqStructLevelValidation is func to validate fields in InfraReqStruct
func InfraReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(model.InfraReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

// CreateNodeGroupReqStructLevelValidation is func to validate fields in model.CreateNodeGroupReqStruct
func CreateNodeGroupReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(model.CreateNodeGroupReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

var holdingInfraMap sync.Map

// createVmObjectSafe creates VM object without WaitGroup management
func createVmObjectSafe(nsId, infraId string, vmInfoData *model.VmInfo) error {
	var wg sync.WaitGroup
	wg.Add(1)
	return CreateVmObject(&wg, nsId, infraId, vmInfoData)
}

// // createVmSafe creates VM without WaitGroup management
// func createVmSafe(nsId, infraId string, vmInfoData *model.VmInfo, option string) error {
// 	var wg sync.WaitGroup
// 	wg.Add(1)
// 	err := CreateVm(&wg, nsId, infraId, vmInfoData, option)
// 	wg.Wait()
// 	return err
// }

// Helper functions for CreateInfra

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// createNodeGroup creates a nodeGroup with proper error handling
func createNodeGroup(ctx context.Context, nsId, infraId string, vmRequest *model.CreateNodeGroupReq, nodeGroupSize, vmStartIndex int, uid string, req *model.InfraReq) error {
	log.Info().Msgf("Creating Infra nodeGroup object for '%s'", vmRequest.Name)
	key := common.GenInfraNodeGroupKey(nsId, infraId, vmRequest.Name)

	nodeGroupInfoData := model.NodeGroupInfo{
		ResourceType:  model.StrNodeGroup,
		Id:            common.ToLower(vmRequest.Name),
		Name:          common.ToLower(vmRequest.Name),
		Uid:           common.GenUid(),
		NodeGroupSize: vmRequest.NodeGroupSize,
	}

	// Build VM ID list
	for i := vmStartIndex; i < nodeGroupSize+vmStartIndex; i++ {
		nodeGroupInfoData.VmId = append(nodeGroupInfoData.VmId, nodeGroupInfoData.Id+"-"+strconv.Itoa(i))
	}

	// Marshal with error handling
	val, err := json.Marshal(nodeGroupInfoData)
	if err != nil {
		return fmt.Errorf("failed to marshal nodeGroup data: %w", err)
	}

	if err := kvstore.Put(key, string(val)); err != nil {
		return fmt.Errorf("failed to store nodeGroup data: %w", err)
	}

	// Store label info
	labels := map[string]string{
		model.LabelManager:          model.StrManager,
		model.LabelNamespace:        nsId,
		model.LabelLabelType:        model.StrNodeGroup,
		model.LabelId:               nodeGroupInfoData.Id,
		model.LabelName:             nodeGroupInfoData.Name,
		model.LabelUid:              nodeGroupInfoData.Uid,
		model.LabelInfraId:          infraId,
		model.LabelInfraName:        req.Name,
		model.LabelInfraUid:         uid,
		model.LabelInfraDescription: req.Description,
	}

	return label.CreateOrUpdateLabel(ctx, model.StrNodeGroup, uid, key, labels)
}

// createInfraObject creates the Infra object with proper error handling
func createInfraObject(ctx context.Context, nsId, infraId string, req *model.InfraReq, uid string) error {
	log.Info().Msg("Creating Infra object")
	key := common.GenInfraKey(nsId, infraId, "")

	infraInfo := model.InfraInfo{
		ResourceType:    model.StrInfra,
		Id:              infraId,
		Name:            req.Name,
		Uid:             uid,
		Description:     req.Description,
		Status:          model.StatusCreating,
		TargetAction:    model.ActionCreate,
		TargetStatus:    model.StatusRunning,
		InstallMonAgent: req.InstallMonAgent,
		SystemLabel:     req.SystemLabel,
		PostCommand:     req.PostCommand,
	}

	val, err := json.Marshal(infraInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal Infra info: %w", err)
	}

	if err := kvstore.Put(key, string(val)); err != nil {
		return fmt.Errorf("failed to store Infra object: %w", err)
	}

	// Store label info
	labels := map[string]string{
		model.LabelManager:     model.StrManager,
		model.LabelNamespace:   nsId,
		model.LabelLabelType:   model.StrInfra,
		model.LabelId:          infraId,
		model.LabelName:        req.Name,
		model.LabelUid:         uid,
		model.LabelDescription: req.Description,
	}
	for key, value := range req.Label {
		labels[key] = value
	}

	return label.CreateOrUpdateLabel(ctx, model.StrInfra, uid, key, labels)
}

// handleHoldOption handles the hold option logic
func handleHoldOption(nsId, infraId string) error {
	key := common.GenInfraKey(nsId, infraId, "")
	holdingInfraMap.Store(key, "holding")

	for {
		value, ok := holdingInfraMap.Load(key)
		if !ok {
			break
		}
		if value == "continue" {
			holdingInfraMap.Delete(key)
			break
		} else if value == "withdraw" {
			holdingInfraMap.Delete(key)
			DelInfra(nsId, infraId, "force")
			return fmt.Errorf("Infra creation was withdrawn by user")
		}

		log.Info().Msgf("Infra: %s (holding)", key)
		time.Sleep(5 * time.Second)
	}

	return nil
}

// cleanupPartialInfra cleans up partially created Infra resources
func cleanupPartialInfra(nsId, infraId string) error {
	log.Warn().Msgf("Cleaning up partial Infra: %s/%s", nsId, infraId)

	// Attempt to delete Infra - this will handle cleanup of VMs and other resources
	_, err := DelInfra(nsId, infraId, "force")
	if err != nil {
		return fmt.Errorf("failed to cleanup partial Infra: %w", err)
	}

	return nil
}

// handleMonitoringAgent handles CB-Dragonfly monitoring agent installation
func handleMonitoringAgent(nsId, infraId string, infraTmp model.InfraInfo, option string) error {
	if !strings.Contains(infraTmp.InstallMonAgent, "yes") || option == "register" {
		return nil
	}

	log.Info().Msg("Installing CB-Dragonfly monitoring agent")

	if err := CheckDragonflyEndpoint(); err != nil {
		log.Warn().Msg("CB-Dragonfly is not available, skipping agent installation")
		return nil
	}

	reqToMon := &model.InfraCmdReq{
		UserName: "cb-user", // TODO: Make this configurable
	}

	// Intelligent wait time based on VM count
	waitTime := 30 * time.Second
	if len(infraTmp.Vm) > 5 {
		waitTime = 60 * time.Second
	}

	log.Info().Msgf("Waiting %v for safe CB-Dragonfly Agent installation", waitTime)
	time.Sleep(waitTime)

	content, err := InstallMonitorAgentToInfra(nsId, infraId, model.StrInfra, reqToMon)
	if err != nil {
		return fmt.Errorf("failed to install monitoring agent: %w", err)
	}

	log.Info().Msg("CB-Dragonfly monitoring agent installed successfully")
	common.PrintJsonPretty(content)
	return nil
}

// handlePostCommands handles post-deployment command execution
func handlePostCommands(nsId, infraId string, infraTmp model.InfraInfo) error {
	if len(infraTmp.PostCommand.Command) == 0 {
		return nil
	}

	log.Info().Msg("Executing post-deployment commands")
	log.Info().Msgf("Waiting 5 seconds for safe bootstrapping")
	time.Sleep(5 * time.Second)

	log.Info().Msgf("Executing commands: %+v", infraTmp.PostCommand)
	output, err := RemoteCommandToInfra(nsId, infraId, "", "", "", &infraTmp.PostCommand, "")
	if err != nil {
		return fmt.Errorf("failed to execute post-deployment commands: %w", err)
	}

	result := model.InfraSshCmdResult{
		Results: output,
	}

	common.PrintJsonPretty(result)
	infraTmp.PostCommandResult = result
	UpdateInfraInfo(nsId, infraTmp)

	log.Info().Msg("Post-deployment commands executed successfully")
	return nil
}

// CreatedResource represents a resource created during dynamic Infra provisioning
type CreatedResource struct {
	Type string `json:"type"` // "vnet", "sshkey", "securitygroup"
	Id   string `json:"id"`   // Resource ID
}

// VmReqWithCreatedResources contains VM request and list of created resources for rollback
type VmReqWithCreatedResources struct {
	VmReq            *model.CreateNodeGroupReq `json:"vmReq"`
	CreatedResources []CreatedResource         `json:"createdResources"`
}

// rollbackCreatedResources deletes only the resources that were created during this Infra creation
func rollbackCreatedResources(nsId string, createdResources []CreatedResource) error {
	var errors []string
	var successes []string

	vNetIds := make([]string, 0)
	sshKeyIds := make([]string, 0)
	securityGroupIds := make([]string, 0)

	log.Info().Msgf("Starting rollback process for %d resources in namespace '%s'", len(createdResources), nsId)

	// Group resources by type for logging
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

	log.Info().Msgf("Resources to rollback: VNet(%d): %v, SSHKey(%d): %v, SecurityGroup(%d): %v",
		len(vNetIds), vNetIds, len(sshKeyIds), sshKeyIds, len(securityGroupIds), securityGroupIds)

	// Use semaphore for parallel processing with concurrency limit
	const maxConcurrency = 10
	semaphore := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	var mutex sync.Mutex

	// Delete SSHKeys first (usually least dependent) in parallel
	for _, res := range sshKeyIds {
		wg.Add(1)
		go func(resourceId string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }() // Release semaphore

			if err := resource.DelResource(nsId, model.StrSSHKey, resourceId, "false"); err != nil {
				errorMsg := fmt.Sprintf("Failed to delete SSHKey '%s' in namespace '%s': %v", resourceId, nsId, err)
				mutex.Lock()
				errors = append(errors, errorMsg)
				mutex.Unlock()
				log.Error().Err(err).Msgf("Rollback failed for SSHKey: %s", resourceId)
			} else {
				successMsg := fmt.Sprintf("SSHKey '%s'", resourceId)
				mutex.Lock()
				successes = append(successes, successMsg)
				mutex.Unlock()
				log.Info().Msgf("Successfully rolled back SSHKey: %s", resourceId)
			}
		}(res)
	}

	// Wait for all SSHKey deletions to complete
	wg.Wait()
	log.Info().Msgf("Completed SSHKey deletions: %d successful, %d failed", len(sshKeyIds), len(errors))

	// Delete SecurityGroups second in parallel
	for _, res := range securityGroupIds {
		wg.Add(1)
		go func(resourceId string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }() // Release semaphore

			if err := resource.DelResource(nsId, model.StrSecurityGroup, resourceId, "false"); err != nil {
				errorMsg := fmt.Sprintf("Failed to delete SecurityGroup '%s' in namespace '%s': %v", resourceId, nsId, err)
				mutex.Lock()
				errors = append(errors, errorMsg)
				mutex.Unlock()
				log.Error().Err(err).Msgf("Rollback failed for SecurityGroup: %s", resourceId)
			} else {
				successMsg := fmt.Sprintf("SecurityGroup '%s'", resourceId)
				mutex.Lock()
				successes = append(successes, successMsg)
				mutex.Unlock()
				log.Info().Msgf("Successfully rolled back SecurityGroup: %s", resourceId)
			}
		}(res)
	}

	// Wait for all SecurityGroup deletions to complete
	wg.Wait()
	log.Info().Msgf("Completed SecurityGroup deletions: %d total attempted", len(securityGroupIds))

	// wait for 5 secs for safe rollback
	time.Sleep(5 * time.Second)

	// Delete VNets last (most dependent) in parallel
	for _, res := range vNetIds {
		wg.Add(1)
		go func(resourceId string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }() // Release semaphore

			if err := resource.DelResource(nsId, model.StrVNet, resourceId, "false"); err != nil {
				errorMsg := fmt.Sprintf("Failed to delete VNet '%s' in namespace '%s': %v", resourceId, nsId, err)
				mutex.Lock()
				errors = append(errors, errorMsg)
				mutex.Unlock()
				log.Error().Err(err).Msgf("Rollback failed for VNet: %s", resourceId)
			} else {
				successMsg := fmt.Sprintf("VNet '%s'", resourceId)
				mutex.Lock()
				successes = append(successes, successMsg)
				mutex.Unlock()
				log.Info().Msgf("Successfully rolled back VNet: %s", resourceId)
			}
		}(res)
	}

	// Wait for all VNet deletions to complete
	wg.Wait()
	log.Info().Msgf("Completed VNet deletions: %d total attempted", len(vNetIds))

	// Log rollback summary
	log.Info().Msgf("Rollback summary: Success(%d): %v, Failed(%d): %d errors",
		len(successes), successes, len(errors), len(errors))

	if len(errors) > 0 {
		return fmt.Errorf("rollback completed with %d errors: %s", len(errors), strings.Join(errors, "; "))
	}

	log.Info().Msgf("All %d resources successfully rolled back in namespace '%s'", len(createdResources), nsId)
	return nil
}

// Infra and VM Provisioning

// ScaleOutInfraNodeGroup is func to create Infra groupVM
func ScaleOutInfraNodeGroup(ctx context.Context, nsId string, infraId string, nodeGroupId string, numVMsToAdd int) (*model.InfraInfo, error) {
	vmIdList, err := ListVmByNodeGroup(nsId, infraId, nodeGroupId)
	if err != nil {
		temp := &model.InfraInfo{}
		return temp, err
	}
	vmObj, err := GetVmObject(nsId, infraId, vmIdList[0])
	if err != nil {
		temp := &model.InfraInfo{}
		return temp, err
	}

	vmNodeGroupReqTemplate := &model.CreateNodeGroupReq{}

	// only take template required to create VM
	vmNodeGroupReqTemplate.Name = vmObj.NodeGroupId
	vmNodeGroupReqTemplate.ConnectionName = vmObj.ConnectionName
	vmNodeGroupReqTemplate.ImageId = vmObj.ImageId
	vmNodeGroupReqTemplate.SpecId = vmObj.SpecId
	vmNodeGroupReqTemplate.VNetId = vmObj.VNetId
	vmNodeGroupReqTemplate.SubnetId = vmObj.SubnetId
	vmNodeGroupReqTemplate.SecurityGroupIds = vmObj.SecurityGroupIds
	vmNodeGroupReqTemplate.SshKeyId = vmObj.SshKeyId
	vmNodeGroupReqTemplate.VmUserName = vmObj.VmUserName
	vmNodeGroupReqTemplate.VmUserPassword = vmObj.VmUserPassword
	vmNodeGroupReqTemplate.RootDiskType = vmObj.RootDiskType
	vmNodeGroupReqTemplate.RootDiskSize = vmObj.RootDiskSize
	vmNodeGroupReqTemplate.Description = vmObj.Description

	vmNodeGroupReqTemplate.NodeGroupSize = numVMsToAdd

	result, err := CreateInfraGroupVm(ctx, nsId, infraId, vmNodeGroupReqTemplate, true)
	if err != nil {
		temp := &model.InfraInfo{}
		return temp, err
	}
	return result, nil

}

// CreateInfraGroupVm is func to create Infra groupVM
func CreateInfraGroupVm(ctx context.Context, nsId string, infraId string, vmRequest *model.CreateNodeGroupReq, newNodeGroup bool) (*model.InfraInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := &model.InfraInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = common.CheckString(infraId)
	if err != nil {
		temp := &model.InfraInfo{}
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

	infraTmp, _, err := GetInfraObject(nsId, infraId)

	if err != nil {
		temp := &model.InfraInfo{}
		return temp, err
	}

	//vmRequest := req

	targetAction := model.ActionCreate
	targetStatus := model.StatusRunning

	//goroutin
	var wg sync.WaitGroup

	// nodeGroup handling
	nodeGroupSize := vmRequest.NodeGroupSize
	fmt.Printf("nodeGroupSize: %v\n", nodeGroupSize)

	// make nodeGroup default (any VM going to be in a nodeGroup)
	if nodeGroupSize < 1 {
		nodeGroupSize = 1
	}

	vmStartIndex := 1

	tentativeVmId := common.ToLower(vmRequest.Name)

	err = common.CheckString(tentativeVmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return &model.InfraInfo{}, err
	}

	// Create or update nodeGroup object (nodeGroupSize is always >= 1)
	log.Info().Msg("Create Infra nodeGroup object")

	nodeGroupInfoData := model.NodeGroupInfo{}
	nodeGroupInfoData.ResourceType = model.StrNodeGroup
	nodeGroupInfoData.Id = tentativeVmId
	nodeGroupInfoData.Name = tentativeVmId
	nodeGroupInfoData.Uid = common.GenUid()
	nodeGroupInfoData.NodeGroupSize = nodeGroupSize

	key := common.GenInfraNodeGroupKey(nsId, infraId, vmRequest.Name)
	keyValue, exists, err := kvstore.GetKv(key)
	if err != nil {
		err = fmt.Errorf("In CreateInfraGroupVm(); kvstore.GetKv(): " + err.Error())
		log.Error().Err(err).Msg("")
	}
	if exists {
		if newNodeGroup {
			json.Unmarshal([]byte(keyValue.Value), &nodeGroupInfoData)
			existingVmSize := nodeGroupInfoData.NodeGroupSize
			// add the number of existing VMs in the NodeGroup with requested number for additions
			nodeGroupInfoData.NodeGroupSize = existingVmSize + nodeGroupSize
			vmStartIndex = existingVmSize + 1
		} else {
			err = fmt.Errorf("Duplicated NodeGroup ID")
			log.Error().Err(err).Msg("")
			return nil, err
		}
	}

	for i := vmStartIndex; i < nodeGroupSize+vmStartIndex; i++ {
		nodeGroupInfoData.VmId = append(nodeGroupInfoData.VmId, nodeGroupInfoData.Id+"-"+strconv.Itoa(i))
	}

	val, _ := json.Marshal(nodeGroupInfoData)
	err = kvstore.Put(key, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
	}
	// check stored nodeGroup object
	_, _, err = kvstore.GetKv(key)
	if err != nil {
		err = fmt.Errorf("In CreateInfraGroupVm(); kvstore.GetKv(): " + err.Error())
		log.Error().Err(err).Msg("")
		// return nil, err
	}

	// Create VM objects for all VMs in the nodeGroup
	for i := vmStartIndex; i < nodeGroupSize+vmStartIndex; i++ {
		vmInfoData := model.VmInfo{}

		vmInfoData.NodeGroupId = common.ToLower(vmRequest.Name)
		vmInfoData.Name = common.ToLower(vmRequest.Name) + "-" + strconv.Itoa(i)

		log.Debug().Msg("vmInfoData.Name: " + vmInfoData.Name)

		vmInfoData.ResourceType = model.StrVM
		vmInfoData.Id = vmInfoData.Name
		vmInfoData.Uid = common.GenUid()

		vmInfoData.PublicIP = ""
		vmInfoData.PublicDNS = ""

		// Set initial status based on whether this is a registration (CspResourceId is set)
		if vmRequest.CspResourceId != "" {
			vmInfoData.Status = model.StatusRegistering
		} else {
			vmInfoData.Status = model.StatusCreating
		}
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
		go CreateVmObject(&wg, nsId, infraId, &vmInfoData)
	}
	wg.Wait()

	// Set option based on whether this is a registration (CspResourceId is set)
	option := "create"
	if vmRequest.CspResourceId != "" {
		option = "register"
	}

	// Collect all VM info for rate-limited parallel processing
	var vmInfoList []*model.VmInfo
	for i := vmStartIndex; i <= nodeGroupSize+vmStartIndex; i++ {
		vmInfoData := model.VmInfo{}

		if nodeGroupSize == 0 { // for VM (not in a group)
			vmInfoData.Name = common.ToLower(vmRequest.Name)
		} else { // for VM (in a group)
			if i == nodeGroupSize+vmStartIndex {
				break
			}
			vmInfoData.NodeGroupId = common.ToLower(vmRequest.Name)
			vmInfoData.Name = common.ToLower(vmRequest.Name) + "-" + strconv.Itoa(i)
		}
		vmInfoData.Id = vmInfoData.Name
		vmId := vmInfoData.Id
		vmInfo, err := GetVmObject(nsId, infraId, vmId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return nil, err
		}
		vmInfoList = append(vmInfoList, &vmInfo)
	}

	// Create VMs with hierarchical rate limiting
	log.Info().Msgf("Creating %d VMs with rate limiting", len(vmInfoList))
	err = CreateVmsInParallel(ctx, nsId, infraId, vmInfoList, option)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create VMs in parallel")
		return nil, err
	}

	//Update Infra status

	infraTmp, _, err = GetInfraObject(nsId, infraId)
	if err != nil {
		temp := &model.InfraInfo{}
		return temp, err
	}

	infraStatusTmp, _ := GetInfraStatus(nsId, infraId)

	infraTmp.Status = infraStatusTmp.Status

	// More robust completion check for Create action
	isCreateCompleted := false
	if infraTmp.TargetAction == model.ActionCreate {
		// For Create action, check if all VMs are in final states (including Failed)
		// Final states: Running, Failed, Terminated, Suspended
		// Transitional states: Creating, Undefined, empty string
		allVmsInFinalState := true
		pendingCount := 0
		runningCount := 0
		failedCount := 0
		totalVmCount := len(infraStatusTmp.Vm)

		for _, vm := range infraStatusTmp.Vm {
			// Check if VM is still in transitional/pending state
			if vm.Status == model.StatusCreating || vm.Status == model.StatusRegistering || vm.Status == model.StatusUndefined || vm.Status == "" {
				allVmsInFinalState = false
				pendingCount++
			} else {
				// VM is in final state, count by type for logging
				switch vm.Status {
				case model.StatusRunning:
					runningCount++
				case model.StatusFailed:
					failedCount++
					// Other final states (Terminated, Suspended) are also acceptable
				}
			}
		}

		if allVmsInFinalState && totalVmCount > 0 {
			isCreateCompleted = true
			if failedCount > 0 {
				log.Info().Msgf("Infra %s Create action completed with partial success: %d running, %d failed, %d total VMs",
					infraId, runningCount, failedCount, totalVmCount)
			} else {
				log.Info().Msgf("Infra %s Create action completed successfully: all %d VMs reached final state",
					infraId, totalVmCount)
			}
		} else {
			log.Debug().Msgf("Infra %s Create action pending: %d/%d VMs still in transitional state",
				infraId, pendingCount, totalVmCount)
		}
	} else {
		// For other actions, use the original simple check
		isCreateCompleted = (infraTmp.TargetStatus == infraTmp.Status)
	}

	if isCreateCompleted {
		infraTmp.TargetStatus = model.StatusComplete
		infraTmp.TargetAction = model.ActionComplete
		log.Info().Msgf("Infra %s action completed, setting TargetAction/TargetStatus to Complete", infraId)
	}
	UpdateInfraInfo(nsId, infraTmp)

	// Install CB-Dragonfly monitoring agent

	if strings.Contains(infraTmp.InstallMonAgent, "yes") {

		// Sleep for 60 seconds for a safe DF agent installation.
		fmt.Printf("\n\n[Info] Sleep for 60 seconds for safe CB-Dragonfly Agent installation.\n\n")
		time.Sleep(60 * time.Second)

		check := CheckDragonflyEndpoint()
		if check != nil {
			fmt.Printf("\n\n[Warning] CB-Dragonfly is not available\n\n")
		} else {
			reqToMon := &model.InfraCmdReq{}
			reqToMon.UserName = "cb-user" // this Infra user name is temporal code. Need to improve.

			fmt.Printf("\n[InstallMonitorAgentToInfra]\n\n")
			content, err := InstallMonitorAgentToInfra(nsId, infraId, model.StrInfra, reqToMon)
			if err != nil {
				log.Error().Err(err).Msg("")
				//infraTmp.InstallMonAgent = "no"
			}
			common.PrintJsonPretty(content)
			//infraTmp.InstallMonAgent = "yes"
		}
	}

	vmList, err := ListVmByNodeGroup(nsId, infraId, tentativeVmId)

	if err != nil {
		infraTmp.SystemMessage = append(infraTmp.SystemMessage, err.Error())
	}
	if vmList != nil {
		infraTmp.NewVmList = vmList
	}

	return &infraTmp, nil

}

// CreateInfra is func to create Infra object and deploy requested VMs (register CSP native VM with option=register)
func CreateInfra(ctx context.Context, nsId string, req *model.InfraReq, option string, isReqFromDynamic bool) (*model.InfraInfo, error) {
	// Input validation
	if err := common.CheckString(nsId); err != nil {
		log.Error().Err(err).Msg("Invalid namespace ID")
		return &model.InfraInfo{}, fmt.Errorf("invalid namespace ID: %w", err)
	}

	if err := validate.Struct(req); err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Error().Err(err).Msg("Invalid validation error")
			return nil, fmt.Errorf("validation failed: %w", err)
		}
		log.Error().Err(err).Msg("Request validation failed")
		return nil, fmt.Errorf("request validation failed: %w", err)
	}

	// Initialize failure tracking
	var (
		vmObjectErrors []model.VmCreationError
		vmCreateErrors []model.VmCreationError
		totalVmCount   int
		errorMu        sync.Mutex
	)

	// Count total VMs to be created (minimum 1 per nodeGroup)
	for _, nodeGroupReq := range req.NodeGroups {
		vmCount := nodeGroupReq.NodeGroupSize
		if vmCount < 1 {
			vmCount = 1
		}
		totalVmCount += vmCount
	}

	// Helper function to add VM creation error (with mutex for standalone use)
	addVmError := func(errors *[]model.VmCreationError, vmName, errorMsg, phase string) {
		errorMu.Lock()
		defer errorMu.Unlock()
		*errors = append(*errors, model.VmCreationError{
			VmName:    vmName,
			Error:     errorMsg,
			Phase:     phase,
			Timestamp: time.Now().Format(time.RFC3339),
		})
	}

	// Early validation of VM requests
	if len(req.NodeGroups) == 0 {
		return nil, fmt.Errorf("no VM requests provided")
	}

	for i, nodeGroupReq := range req.NodeGroups {
		if err := common.CheckString(nodeGroupReq.Name); err != nil {
			return nil, fmt.Errorf("invalid VM name at index %d: %w", i, err)
		}

		// Validate connection config early
		if _, err := common.GetConnConfig(nodeGroupReq.ConnectionName); err != nil {
			return nil, fmt.Errorf("invalid connection config '%s' for VM '%s': %w",
				nodeGroupReq.ConnectionName, nodeGroupReq.Name, err)
		}
	}

	// Initialize Infra
	uid := common.GenUid()
	infraId := req.Name

	// Pre-calculate VM configurations to avoid duplication
	type vmConfig struct {
		vmInfo        model.VmInfo
		nodeGroupSize int
		vmIndex       int
	}

	var vmConfigs []vmConfig
	var nodeGroupsCreated []string
	vmStartIndex := 1

	// Get infra object
	// Note: return 'an empty Infra object', 'nil' if Infra doesn't exist
	infraTmp, exists, err := GetInfraObject(nsId, infraId)
	log.Debug().Msgf("Fetched Infra object: %+v, error: %v", infraTmp, err)

	if isReqFromDynamic {
		// isReqFromDynamic. Do not create Infra object. Reuse the existing one.
		if err != nil {
			log.Error().Err(err).Msgf("Infra '%s' does not exist in namespace '%s' should be prepared by dynamic request", infraId, nsId)
		} else {
			infraTmp.Status = model.StatusCreating
			infraTmp.TargetAction = model.ActionCreate
			infraTmp.TargetStatus = model.StatusRunning
			UpdateInfraInfo(nsId, infraTmp)
		}
	} else {
		// fallback for manual infra create. not from isReqFromDynamic.
		if !exists {
			log.Debug().Msgf("Infra '%s' does not exist, creating new one", infraId)
			// Create Infra object first
			if err := createInfraObject(ctx, nsId, infraId, req, uid); err != nil {
				return nil, fmt.Errorf("failed to create Infra object: %w", err)
			}
		} else {
			// Check Infra existence (skip for register option)
			if option != "register" {
				log.Debug().Msgf("Infra '%s' already exists in namespace '%s'", infraId, nsId)
				return nil, fmt.Errorf("Infra '%s' already exists in namespace '%s'", infraId, nsId)
			} else {
				req.SystemLabel = "Registered from CSP"
			}
		}
	}

	// Process VM requests and build configurations
	for _, nodeGroupReq := range req.NodeGroups {
		nodeGroupSize := nodeGroupReq.NodeGroupSize
		if nodeGroupSize < 1 {
			nodeGroupSize = 1
		}

		log.Debug().Msgf("Processing VM request '%s' with nodeGroupSize: %d", nodeGroupReq.Name, nodeGroupSize)

		// Get connection config once and validate
		connectionConfig, err := common.GetConnConfig(nodeGroupReq.ConnectionName)
		if err != nil {
			return nil, fmt.Errorf("cannot retrieve connection config for VM '%s': %w", nodeGroupReq.Name, err)
		}

		// Create nodeGroup if needed
		if nodeGroupSize > 0 {
			nodeGroupName := common.ToLower(nodeGroupReq.Name)
			if !contains(nodeGroupsCreated, nodeGroupName) {
				if err := createNodeGroup(ctx, nsId, infraId, &nodeGroupReq, nodeGroupSize, vmStartIndex, uid, req); err != nil {
					return nil, fmt.Errorf("failed to create nodeGroup '%s': %w", nodeGroupName, err)
				}
				nodeGroupsCreated = append(nodeGroupsCreated, nodeGroupName)
			}
		}

		// Build VM configurations
		for i := vmStartIndex; i <= nodeGroupSize+vmStartIndex; i++ {
			if nodeGroupSize > 0 && i == nodeGroupSize+vmStartIndex {
				break
			}

			// Set initial status based on option (create vs register)
			initialStatus := model.StatusCreating
			if option == "register" {
				initialStatus = model.StatusRegistering
			}

			vmInfo := model.VmInfo{
				ResourceType:     model.StrVM,
				Uid:              common.GenUid(),
				PublicIP:         "",
				PublicDNS:        "",
				Status:           initialStatus,
				TargetAction:     model.ActionCreate,
				TargetStatus:     model.StatusRunning,
				ConnectionName:   nodeGroupReq.ConnectionName,
				ConnectionConfig: connectionConfig,
				Location:         connectionConfig.RegionDetail.Location,
				SpecId:           nodeGroupReq.SpecId,
				ImageId:          nodeGroupReq.ImageId,
				VNetId:           nodeGroupReq.VNetId,
				SubnetId:         nodeGroupReq.SubnetId,
				SecurityGroupIds: nodeGroupReq.SecurityGroupIds,
				DataDiskIds:      nodeGroupReq.DataDiskIds,
				SshKeyId:         nodeGroupReq.SshKeyId,
				Description:      nodeGroupReq.Description,
				VmUserName:       nodeGroupReq.VmUserName,
				VmUserPassword:   nodeGroupReq.VmUserPassword,
				RootDiskType:     nodeGroupReq.RootDiskType,
				RootDiskSize:     nodeGroupReq.RootDiskSize,
				Label:            nodeGroupReq.Label,
				CspResourceId:    nodeGroupReq.CspResourceId,
			}

			if nodeGroupSize == 0 {
				vmInfo.Name = common.ToLower(nodeGroupReq.Name)
			} else {
				vmInfo.NodeGroupId = common.ToLower(nodeGroupReq.Name)
				vmInfo.Name = common.ToLower(nodeGroupReq.Name) + "-" + strconv.Itoa(i)
			}
			vmInfo.Id = vmInfo.Name

			vmConfigs = append(vmConfigs, vmConfig{
				vmInfo:        vmInfo,
				nodeGroupSize: nodeGroupSize,
				vmIndex:       i,
			})
		}
	}

	// Handle hold option
	if option == "hold" {
		if err := handleHoldOption(nsId, infraId); err != nil {
			return nil, fmt.Errorf("hold option failed: %w", err)
		}
		option = "create"
	}

	// Create VM objects with error collection
	var wg sync.WaitGroup
	var createErrors []error

	log.Info().Msgf("Creating %d VM objects", len(vmConfigs))

	for _, config := range vmConfigs {
		wg.Add(1)
		go func(cfg vmConfig) {
			defer wg.Done()
			if err := createVmObjectSafe(nsId, infraId, &cfg.vmInfo); err != nil {
				errorMu.Lock()
				createErrors = append(createErrors, fmt.Errorf("VM object creation failed for '%s': %w", cfg.vmInfo.Name, err))
				addVmError(&vmObjectErrors, cfg.vmInfo.Name, err.Error(), "object_creation")
				errorMu.Unlock()
			}
		}(config)
	}
	wg.Wait()

	// Check for VM object creation errors
	if len(createErrors) > 0 {
		// Add VM object creation errors to Infra SystemMessage
		infraTmp, _, err := GetInfraObject(nsId, infraId)
		if err == nil {
			// Add VM object creation error summary
			errorSummary := fmt.Sprintf("VM object creation failed for %d out of %d VMs", len(createErrors), len(vmConfigs))
			infraTmp.SystemMessage = append(infraTmp.SystemMessage, errorSummary)

			// Add each VM object creation error
			for _, vmError := range vmObjectErrors {
				errorDetail := fmt.Sprintf("VM '%s' object creation failed: %s", vmError.VmName, vmError.Error)
				infraTmp.SystemMessage = append(infraTmp.SystemMessage, errorDetail)
			}

			// Add policy information
			policyMsg := fmt.Sprintf("Failure handling policy: %s", req.PolicyOnPartialFailure)
			infraTmp.SystemMessage = append(infraTmp.SystemMessage, policyMsg)

			UpdateInfraInfo(nsId, infraTmp)
			log.Info().Msgf("Added %d VM object creation errors to Infra SystemMessage", len(createErrors)+2)
		}

		switch req.PolicyOnPartialFailure {
		case model.PolicyRollback:
			log.Warn().Msgf("VM object creation failed for %d VMs, rolling back entire Infra due to policy=rollback", len(createErrors))
			if cleanupErr := cleanupPartialInfra(nsId, infraId); cleanupErr != nil {
				log.Error().Err(cleanupErr).Msg("Failed to cleanup partial Infra")
			}
			return nil, fmt.Errorf("VM object creation failed, Infra rolled back: %v", createErrors)
		case model.PolicyRefine:
			log.Warn().Msgf("VM object creation failed for %d VMs, failed VMs will be refined after Infra creation due to policy=refine", len(createErrors))
			// Refine will be executed after Infra creation is completed
		default: // model.PolicyContinue or empty
			log.Warn().Msgf("VM object creation failed for %d VMs, continuing with partial provisioning due to policy=continue", len(createErrors))
		}

		// Log detailed error information
		for i, err := range createErrors {
			log.Error().Msgf("VM object creation error %d: %v", i+1, err)
		}
	}

	// Create actual VMs with hierarchical rate limiting
	log.Info().Msgf("Creating %d VMs with rate limiting", len(vmConfigs))
	createErrors = createErrors[:0] // Reset error slice

	// Collect all VM info for rate-limited parallel processing
	var vmInfoList []*model.VmInfo
	for _, config := range vmConfigs {
		vmInfoData, err := GetVmObject(nsId, infraId, config.vmInfo.Id)
		if err != nil {
			return nil, fmt.Errorf("failed to get VM object '%s': %w", config.vmInfo.Id, err)
		}
		vmInfoList = append(vmInfoList, &vmInfoData)
	}

	// Create VMs with hierarchical rate limiting
	err = CreateVmsInParallel(ctx, nsId, infraId, vmInfoList, option)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create VMs in parallel")

		// CRITICAL: If CreateVmsInParallel returns error, it means ALL VMs failed
		// Check total VM count and immediately terminate if all failed
		totalVmsInParallel := len(vmInfoList)

		log.Error().Msgf("EARLY TERMINATION: CreateVmsInParallel returned error - all %d VMs failed", totalVmsInParallel)

		// Force update all VM statuses to Failed since CreateVmsInParallel failed completely
		log.Debug().Msg("Force updating all VM statuses to Failed since no VMs were actually created")
		for _, vmInfo := range vmInfoList {
			vmInfo.Status = model.StatusFailed
			if vmInfo.SystemMessage == "" {
				vmInfo.SystemMessage = fmt.Sprintf("VM creation failed: %s", err.Error())
			}

			UpdateVmInfo(nsId, infraId, *vmInfo)
			log.Debug().Msgf("Force updated VM %s to Failed status (no actual CSP VM created)", vmInfo.Name)
		}

		// Get Infra info and mark as failed immediately
		infraResult, infraErr := GetInfraInfo(nsId, infraId)
		if infraErr != nil {
			return nil, fmt.Errorf("failed to get Infra info after all VMs failed: %w", infraErr)
		}

		// Mark Infra as Failed with complete finalization
		infraResult.Status = model.StatusFailed
		infraResult.TargetStatus = model.StatusComplete
		infraResult.TargetAction = model.ActionComplete
		UpdateInfraInfo(nsId, *infraResult)

		log.Error().Msgf("Infra %s marked as Failed - all VM and Infra status updates completed", infraId)

		// Record provisioning failure events even when all VMs failed
		if err := RecordProvisioningEventsFromInfra(nsId, infraResult); err != nil {
			log.Error().Err(err).Msgf("Failed to record provisioning events for failed Infra '%s'", infraId)
		}

		// Return detailed error message
		errorMsg := fmt.Sprintf("Infra '%s' creation failed: all %d VMs failed to create.\n\nError: %s",
			infraId, totalVmsInParallel, err.Error())

		return infraResult, fmt.Errorf("%s", errorMsg)
	}

	// Continue with normal processing for successful or partial VM creation
	// Note: If CreateVmsInParallel returns error, we already handled it above and returned early
	// This code block is only reached when VM creation was successful or partially successful

	// Check for VM creation errors (this applies to partial failures only)
	if len(createErrors) > 0 {
		// Add VM creation errors to Infra SystemMessage
		infraTmp, _, err := GetInfraObject(nsId, infraId)
		if err == nil {
			// Add VM creation error summary
			errorSummary := fmt.Sprintf("VM creation failed for %d out of %d VMs", len(createErrors), len(vmConfigs))
			infraTmp.SystemMessage = append(infraTmp.SystemMessage, errorSummary)

			// Add each VM creation error - use vmObjectErrors if vmCreateErrors is empty
			errorList := vmCreateErrors
			if len(errorList) == 0 {
				errorList = vmObjectErrors
			}
			for _, vmError := range errorList {
				errorDetail := fmt.Sprintf("VM '%s' creation failed: %s", vmError.VmName, vmError.Error)
				infraTmp.SystemMessage = append(infraTmp.SystemMessage, errorDetail)
			}

			// Add policy information
			policyMsg := fmt.Sprintf("Failure handling policy: %s", req.PolicyOnPartialFailure)
			infraTmp.SystemMessage = append(infraTmp.SystemMessage, policyMsg)

			UpdateInfraInfo(nsId, infraTmp)
			log.Info().Msgf("Added %d VM creation errors to Infra SystemMessage", len(createErrors)+2)
		}

		switch req.PolicyOnPartialFailure {
		case model.PolicyRollback:
			log.Error().Msgf("VM creation failed for %d VMs, rolling back entire Infra due to policy=rollback", len(createErrors))
			// Record provisioning failure events before rollback
			if infraInfo, infraErr := GetInfraInfo(nsId, infraId); infraErr == nil {
				if err := RecordProvisioningEventsFromInfra(nsId, infraInfo); err != nil {
					log.Error().Err(err).Msgf("Failed to record provisioning events before rollback for Infra '%s'", infraId)
				}
			}
			if cleanupErr := cleanupPartialInfra(nsId, infraId); cleanupErr != nil {
				log.Error().Err(cleanupErr).Msg("Failed to cleanup partial Infra")
			}
			return nil, fmt.Errorf("VM creation failed, Infra rolled back: %v", createErrors)
		case model.PolicyRefine:
			log.Warn().Msgf("VM creation failed for %d VMs, failed VMs will be refined after Infra creation due to policy=refine", len(createErrors))
			// Refine will be executed after Infra creation is completed
		default: // model.PolicyContinue or empty
			log.Warn().Msgf("VM creation failed for %d VMs, continuing with partial Infra due to policy=continue", len(createErrors))
		}

		// Log detailed error information
		for i, err := range createErrors {
			log.Error().Msgf("VM creation error %d: %v", i+1, err)
		}

		// Continue with partial Infra unless rollback was requested
		log.Info().Msg("Continuing with partial Infra provisioning")
	}

	// Update Infra status - ensure completion status is set regardless of VM failures
	infraTmp, _, err = GetInfraObject(nsId, infraId)
	if err != nil {
		return nil, fmt.Errorf("failed to get Infra object after VM creation: %w", err)
	}

	// Set completion status first to prevent infinite status loops
	infraTmp.TargetStatus = model.StatusComplete
	infraTmp.TargetAction = model.ActionComplete
	UpdateInfraInfo(nsId, infraTmp)

	// Then get current status from CSP
	// Note: GetInfraStatus internally updates Infra info via UpdateInfraInfo
	infraStatusTmp, err := GetInfraStatus(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get Infra status, but continuing with Infra creation completion")
		// GetInfraStatus failed, but infraTmp still has the completion status we set above
		// No need to manually update status since GetInfraStatus failure means CSP status is unknown
		// The completion status (TargetAction=Complete, TargetStatus=Complete) remains valid
	} else {
		// GetInfraStatus succeeded and already updated Infra info internally
		// Update our local copy with the latest status from CSP
		infraTmp.Status = infraStatusTmp.Status
		// Final update to ensure our local changes are persisted
		UpdateInfraInfo(nsId, infraTmp)
	}

	log.Info().Msgf("Infra '%s' has been successfully created with %d VMs", infraId, len(vmConfigs))

	// Install monitoring agent if requested
	if err := handleMonitoringAgent(nsId, infraId, infraTmp, option); err != nil {
		log.Error().Err(err).Msg("Failed to install monitoring agent, but continuing")
		// Add monitoring agent error to SystemMessage
		infraTmp, _, infraErr := GetInfraObject(nsId, infraId)
		if infraErr == nil {
			errorMsg := fmt.Sprintf("Monitoring agent installation failed: %s", err.Error())
			infraTmp.SystemMessage = append(infraTmp.SystemMessage, errorMsg)
			UpdateInfraInfo(nsId, infraTmp)
		}
	}

	// Execute post-deployment commands
	if err := handlePostCommands(nsId, infraId, infraTmp); err != nil {
		log.Error().Err(err).Msg("Failed to execute post-deployment commands, but continuing")
		// Add post-command error to SystemMessage
		infraTmp, _, infraErr := GetInfraObject(nsId, infraId)
		if infraErr == nil {
			errorMsg := fmt.Sprintf("Post-deployment commands failed: %s", err.Error())
			infraTmp.SystemMessage = append(infraTmp.SystemMessage, errorMsg)
			UpdateInfraInfo(nsId, infraTmp)
		}
	}

	// Execute refine action if policy is set to refine and there were failures
	var shouldRefine bool
	if req.PolicyOnPartialFailure == model.PolicyRefine && (len(vmObjectErrors) > 0 || len(vmCreateErrors) > 0) {
		log.Info().Msgf("Executing refine action to cleanup failed VMs in Infra '%s'", infraId)
		if refineResult, err := HandleInfraAction(nsId, infraId, model.ActionRefine, true); err != nil {
			log.Error().Err(err).Msg("Failed to execute refine action, but continuing")
		} else {
			log.Info().Msgf("Refine action completed: %s", refineResult)
			shouldRefine = true
		}
	}

	// Get final Infra information
	infraResult, err := GetInfraInfo(nsId, infraId)
	if err != nil {
		return nil, fmt.Errorf("failed to get final Infra information: %w", err)
	}

	// Note: All VM failure case is already handled earlier when CreateVmsInParallel returns error
	// This section only handles partial failures or successful cases

	// Add creation error information if there were any failures
	if len(vmObjectErrors) > 0 || len(vmCreateErrors) > 0 {
		successfulVmCount := totalVmCount - len(vmObjectErrors) - len(vmCreateErrors)
		failedVmCount := len(vmObjectErrors) + len(vmCreateErrors)

		var failureStrategy string
		switch req.PolicyOnPartialFailure {
		case model.PolicyRollback:
			failureStrategy = model.PolicyRollback
		case model.PolicyRefine:
			failureStrategy = model.PolicyRefine
		default: // model.PolicyContinue or empty
			failureStrategy = model.PolicyContinue
		}

		infraResult.CreationErrors = &model.InfraCreationErrors{
			VmObjectCreationErrors:  vmObjectErrors,
			VmCreationErrors:        vmCreateErrors,
			TotalVmCount:            totalVmCount,
			SuccessfulVmCount:       successfulVmCount,
			FailedVmCount:           failedVmCount,
			FailureHandlingStrategy: failureStrategy,
		}

		log.Info().Msgf("Infra '%s' creation completed with %d successful VMs out of %d total (strategy: %s, refined: %t)",
			infraId, successfulVmCount, totalVmCount, failureStrategy, shouldRefine)
	} else {
		log.Info().Msgf("Infra '%s' has been successfully created with all %d VMs", infraId, totalVmCount)
	}

	// Record provisioning events to history if there were any failures or if specs have previous failure history
	if err := RecordProvisioningEventsFromInfra(nsId, infraResult); err != nil {
		log.Error().Err(err).Msgf("Failed to record provisioning events for Infra '%s', but continuing", infraId)
	}

	// Update DB for the final status of Infra
	infraResult.TargetStatus = model.StatusComplete
	infraResult.TargetAction = model.ActionComplete
	UpdateInfraInfo(nsId, *infraResult)

	// Re-read with labels properly loaded from the label store
	infraResult, err = GetInfraInfo(nsId, infraId)
	if err != nil {
		return nil, fmt.Errorf("failed to get Infra info after VM creation: %w", err)
	}
	return infraResult, nil
}

// CheckInfraDynamicReq is func to check request info to create Infra obeject and deploy requested VMs in a dynamic way
func CheckInfraDynamicReq(ctx context.Context, req *model.InfraConnectionConfigCandidatesReq) (*model.CheckInfraDynamicReqInfo, error) {

	credentialHolder := common.CredentialHolderFromContext(ctx)
	infraReqInfo := model.CheckInfraDynamicReqInfo{}

	connectionConfigList, err := common.GetConnConfigList(credentialHolder, true, true)
	if err != nil {
		err := fmt.Errorf("cannot load ConnectionConfigList in Infra dynamic request check")
		log.Error().Err(err).Msg("")
		return &infraReqInfo, err
	}

	// Find detail info and ConnectionConfigCandidates
	for _, k := range req.SpecIds {
		errMessage := ""

		vmReqInfo := model.CheckNodeGroupDynamicReqInfo{}

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
		infraReqInfo.ReqCheck = append(infraReqInfo.ReqCheck, vmReqInfo)
	}

	return &infraReqInfo, err
}

// CreateInfraDynamic is func to create Infra obeject and deploy requested VMs in a dynamic way
func CreateInfraDynamic(ctx context.Context, nsId string, req *model.InfraDynamicReq, deployOption string) (*model.InfraInfo, error) {

	reqID := common.RequestIDFromContext(ctx)
	credentialHolder := common.CredentialHolderFromContext(ctx)

	// Initialize comprehensive error tracking
	var errorHistory []string

	// Helper function to add errors to history
	addErrorToHistory := func(phase, details string) {
		timestamp := time.Now().Format("15:04:05")
		errorHistory = append(errorHistory, fmt.Sprintf("[%s] %s: %s", timestamp, phase, details))
	}

	infraReq := model.InfraReq{}
	infraReq.Name = req.Name
	infraReq.Label = req.Label
	infraReq.SystemLabel = req.SystemLabel
	infraReq.InstallMonAgent = req.InstallMonAgent
	infraReq.Description = req.Description
	infraReq.PostCommand = req.PostCommand
	infraReq.PolicyOnPartialFailure = req.PolicyOnPartialFailure

	emptyInfra := &model.InfraInfo{}
	err := common.CheckString(nsId)
	if err != nil {
		err := fmt.Errorf("invalid namespace. %w", err)
		log.Error().Err(err).Msg("")
		addErrorToHistory("Namespace Validation", err.Error())
		return emptyInfra, err
	}
	check, err := CheckInfra(nsId, req.Name)
	if err != nil {
		err := fmt.Errorf("invalid infra name. %w", err)
		log.Error().Err(err).Msg("")
		addErrorToHistory("Infra Name Validation", err.Error())
		return emptyInfra, err
	}
	if check {
		err := fmt.Errorf("The infra " + req.Name + " already exists.")
		addErrorToHistory("Infra Existence Check", err.Error())
		return emptyInfra, err
	}

	// Initialize Infra
	uid := common.GenUid()
	infraId := req.Name

	if err := createInfraObject(ctx, nsId, infraId, &infraReq, uid); err != nil {
		addErrorToHistory("Infra Object Creation", err.Error())
		return emptyInfra, err
	}
	// Get Infra object
	infraTmp, _, err := GetInfraObject(nsId, infraId)
	if err != nil {
		addErrorToHistory("Infra Object Retrieval", err.Error())
		return emptyInfra, err
	}
	// start infra provisioning with StatusPreparing
	infraTmp.Status = model.StatusPreparing
	UpdateInfraInfo(nsId, infraTmp)

	nodeGroupReqs := req.NodeGroups

	// Propagate Infra-level template IDs to NodeGroups that don't specify their own
	for i := range nodeGroupReqs {
		if nodeGroupReqs[i].VNetTemplateId == "" && req.VNetTemplateId != "" {
			nodeGroupReqs[i].VNetTemplateId = req.VNetTemplateId
		}
		if nodeGroupReqs[i].SgTemplateId == "" && req.SgTemplateId != "" {
			nodeGroupReqs[i].SgTemplateId = req.SgTemplateId
		}
	}

	// Check whether VM names meet requirement.
	// Use semaphore for parallel processing with concurrency limit
	const maxConcurrency = 10
	semaphore := make(chan struct{}, maxConcurrency)

	var wg sync.WaitGroup
	var mutex sync.Mutex
	var validationErrors []string

	for i, k := range nodeGroupReqs {
		wg.Add(1)
		go func(index int, nodeGroupReq model.CreateNodeGroupDynamicReq) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }() // Release semaphore

			// log VM request details
			log.Debug().Msgf("[%d] VM Request: %+v", index, nodeGroupReq)

			err := checkCommonResAvailableForNodeGroupDynamicReq(ctx, &nodeGroupReq, nsId)
			if err != nil {
				log.Error().Err(err).Msgf("[%d] Failed to find common resource for Infra provision", index)
				mutex.Lock()
				validationErrors = append(validationErrors, fmt.Sprintf("NodeGroup[%d] '%s': %s",
					index+1, nodeGroupReq.Name, err.Error()))
				// Add to error history with more context
				addErrorToHistory("Resource Validation",
					fmt.Sprintf("NodeGroup '%s' (Index: %d) failed validation: %s",
						nodeGroupReq.Name, index+1, err.Error()))
				mutex.Unlock()
			}
		}(i, k)
	}

	wg.Wait()

	if len(validationErrors) > 0 {
		// Clean up Infra object on validation failure
		DelInfra(nsId, infraId, "force")

		// Build comprehensive error message with history
		errorMsg := fmt.Sprintf("Infra '%s' validation failed due to resource availability errors.\n\n", req.Name)

		// Add error history if available
		if len(errorHistory) > 0 {
			errorMsg += "Error Timeline:\n"
			for i, errEntry := range errorHistory {
				errorMsg += fmt.Sprintf(" %d. %s\n", i+1, errEntry)
			}
			errorMsg += "\n"
		}

		// Add validation error details
		errorMsg += "Resource Validation Failures:\n"
		for _, errStr := range validationErrors {
			errorMsg += fmt.Sprintf(" • %s\n", errStr)
		}
		errorMsg += fmt.Sprintf("\nSummary: %d out of %d NodeGroups failed validation", len(validationErrors), len(nodeGroupReqs))

		return emptyInfra, errors.New(errorMsg)
	}

	// Check if vmRequest has elements
	if len(nodeGroupReqs) > 0 {
		// allCreatedResources tracks ALL resources created during the preparation phase,
		// including those from failed NodeGroups. This enables cleanup under rollback policy.
		var allCreatedResources []CreatedResource
		var wg sync.WaitGroup
		var mutex sync.Mutex

		type vmResult struct {
			result *VmReqWithCreatedResources
			err    error
		}
		resultChan := make(chan vmResult, len(nodeGroupReqs))

		// Group nodeGroupReqs by connectionName for sequential processing
		connectionGroups := make(map[string][]model.CreateNodeGroupDynamicReq)

		// First, determine the connection name for each nodeGroup
		for _, nodeGroupReq := range nodeGroupReqs {
			// Get spec info to determine connection
			specInfo, err := resource.GetSpec(model.SystemCommonNs, nodeGroupReq.SpecId)
			if err != nil {
				log.Error().Err(err).Msgf("Failed to get spec info for grouping: %s", nodeGroupReq.SpecId)
				// Add error to result channel instead of continuing
				resultChan <- vmResult{
					result: nil,
					err:    fmt.Errorf("failed to get spec info for NodeGroup '%s': %w", nodeGroupReq.Name, err),
				}
				continue
			}

			connectionName := common.ResolveConnectionName(specInfo.ConnectionName, credentialHolder)
			// credentialHolder already extracted from ctx above
			if nodeGroupReq.ConnectionName != "" {
				connectionName = nodeGroupReq.ConnectionName
			}

			// Group by connection name
			connectionGroups[connectionName] = append(connectionGroups[connectionName], nodeGroupReq)
		}

		// Warn when the same connection has NodeGroups with different VNetTemplateIds.
		// Different templates result in separate VPCs within the same CSP region, so VMs
		// in those NodeGroups cannot communicate directly without VPC peering.
		for connName, nodeGroups := range connectionGroups {
			if len(nodeGroups) < 2 {
				continue
			}
			firstTemplate := nodeGroups[0].VNetTemplateId
			for _, sg := range nodeGroups[1:] {
				if sg.VNetTemplateId != firstTemplate {
					log.Warn().Msgf(
						"Connection '%s' has NodeGroups with different VNetTemplateIds ('%s' vs '%s'). "+
							"Each template creates an independent VPC; VMs across these NodeGroups cannot communicate directly without VPC peering.",
						connName, firstTemplate, sg.VNetTemplateId,
					)
					break
				}
			}
		}

		log.Info().Msgf("Grouped %d NodeGroups into %d connection groups", len(nodeGroupReqs), len(connectionGroups))

		// Process each connection group in parallel, but VMs within each group sequentially
		for connectionName, nodeGroupsInConnection := range connectionGroups {
			wg.Add(1)
			go func(connName string, nodeGroups []model.CreateNodeGroupDynamicReq) {
				defer wg.Done()

				log.Info().Msgf("Processing %d NodeGroups for connection '%s' sequentially", len(nodeGroups), connName)

				// Process NodeGroups in this connection sequentially
				for i, nodeGroupDynamicReq := range nodeGroups {
					log.Debug().Msgf("[%s][%d/%d] Processing NodeGroup '%s' sequentially",
						connName, i+1, len(nodeGroups), nodeGroupDynamicReq.Name)

					// Add small delay between sequential requests to avoid rate limiting
					if i > 0 {
						time.Sleep(2 * time.Second)
					}

					result, err := getNodeGroupReqFromDynamicReq(ctx, nsId, &nodeGroupDynamicReq)
					resultChan <- vmResult{result: result, err: err}
				}

				log.Info().Msgf("Completed processing NodeGroups for connection '%s'", connName)
			}(connectionName, nodeGroupsInConnection)
		}

		// Wait for all goroutines to complete
		wg.Wait()
		close(resultChan)

		// Collect results and check for errors
		var hasError bool
		var failedNodeGroups []string
		var errorDetails []string
		var successfulNodeGroups []string

		for vmRes := range resultChan {
			if vmRes.err != nil {
				log.Error().Err(vmRes.err).Msg("Failed to prepare resources for dynamic Infra creation")
				hasError = true

				// Extract NodeGroup details from error context
				nodeGroupName := "unknown"
				if vmRes.result != nil && vmRes.result.VmReq != nil {
					nodeGroupName = vmRes.result.VmReq.Name
				}
				failedNodeGroups = append(failedNodeGroups, nodeGroupName)
				errorDetails = append(errorDetails, fmt.Sprintf("NodeGroup '%s': %s", nodeGroupName, vmRes.err.Error()))

				// Add to error history
				addErrorToHistory("NodeGroup Resource Preparation",
					fmt.Sprintf("Failed to prepare resources for NodeGroup '%s': %s", nodeGroupName, vmRes.err.Error()))

				// Track resources that were partially created before the failure so they can
				// be cleaned up if rollback policy is in effect.
				mutex.Lock()
				if vmRes.result != nil && len(vmRes.result.CreatedResources) > 0 {
					log.Info().Msgf("NodeGroup '%s' failed after creating %d resource(s); tracking for potential rollback",
						nodeGroupName, len(vmRes.result.CreatedResources))
					allCreatedResources = append(allCreatedResources, vmRes.result.CreatedResources...)
				}
				mutex.Unlock()
			} else {
				// Safely append to the shared infraReq.NodeGroups slice
				mutex.Lock()
				infraReq.NodeGroups = append(infraReq.NodeGroups, *vmRes.result.VmReq)
				allCreatedResources = append(allCreatedResources, vmRes.result.CreatedResources...)
				successfulNodeGroups = append(successfulNodeGroups, vmRes.result.VmReq.Name)
				mutex.Unlock()
			}
		}

		// Handle resource preparation failures
		if hasError {
			// Get updated Infra object
			infraTmp, _, err := GetInfraObject(nsId, infraId)
			if err == nil {
				// Add general error summary to both SystemMessage and error history
				errorSummary := fmt.Sprintf("Resource preparation failed for %d NodeGroup(s) out of %d total NodeGroups", len(failedNodeGroups), len(failedNodeGroups)+len(successfulNodeGroups))
				infraTmp.SystemMessage = append(infraTmp.SystemMessage, errorSummary)
				addErrorToHistory("Resource Preparation Summary", errorSummary)

				// Add detailed error messages for each failed NodeGroup to both SystemMessage and error history
				for _, detail := range errorDetails {
					infraTmp.SystemMessage = append(infraTmp.SystemMessage, detail)
					addErrorToHistory("NodeGroup Resource Failure", detail)
				}

				// Check if ALL NodeGroups failed - if so, set status to Failed and return immediately
				if len(successfulNodeGroups) == 0 {
					addErrorToHistory("Infra Status Decision", "All NodeGroups failed resource preparation - marking Infra as Failed")
					infraTmp.SystemMessage = append(infraTmp.SystemMessage, "Infra creation aborted: All NodeGroups failed resource preparation")
					infraTmp.Status = model.StatusFailed
					UpdateInfraInfo(nsId, infraTmp)

					// Rollback any shared resources (VNet/SshKey/SG) that were partially created
					// before the failures. These resources are shared-namespace resources so they
					// will not be automatically cleaned up by Infra deletion.
					if len(allCreatedResources) > 0 {
						log.Info().Msgf("All NodeGroups failed — rolling back %d partially created shared resource(s)", len(allCreatedResources))
						if rollbackErr := rollbackCreatedResources(nsId, allCreatedResources); rollbackErr != nil {
							log.Warn().Err(rollbackErr).Msg("Partial rollback failure during all-NodeGroups-failed cleanup; some shared resources may remain")
							addErrorToHistory("Shared Resource Rollback", fmt.Sprintf("Rollback encountered errors: %s", rollbackErr.Error()))
						} else {
							addErrorToHistory("Shared Resource Rollback", fmt.Sprintf("Successfully rolled back %d shared resource(s)", len(allCreatedResources)))
						}
					}

					// Build comprehensive error message with complete history
					errorMsg := fmt.Sprintf("Infra '%s' creation failed - all NodeGroups failed resource preparation.\n\n", req.Name)

					// Add full error history
					if len(errorHistory) > 0 {
						errorMsg += "Complete Error Timeline:\n"
						for i, errEntry := range errorHistory {
							errorMsg += fmt.Sprintf("  %d. %s\n", i+1, errEntry)
						}
						errorMsg += "\n"
					}

					errorMsg += "Summary: All NodeGroups failed during resource preparation phase.\n"
					errorMsg += "Common causes: VPC/subnet limits, insufficient permissions, region capacity issues, or network configuration problems.\n"
					errorMsg += "Check the error timeline above for specific failure details."

					return emptyInfra, fmt.Errorf("%s", errorMsg)
				}

				// Partial failure: some NodeGroups succeeded, some failed.
				// Apply PolicyOnPartialFailure to decide whether to rollback or continue.
				switch req.PolicyOnPartialFailure {
				case model.PolicyRollback:
					// Roll back ALL created shared resources (from both successful and failed NodeGroups)
					// because the user requested all-or-nothing semantics.
					addErrorToHistory("Infra Status Decision",
						fmt.Sprintf("Partial failure with policy=rollback: rolling back all %d created shared resource(s)", len(allCreatedResources)))
					log.Warn().Msgf("Partial NodeGroup failure with policy=rollback: rolling back %d shared resource(s)", len(allCreatedResources))
					if len(allCreatedResources) > 0 {
						if rollbackErr := rollbackCreatedResources(nsId, allCreatedResources); rollbackErr != nil {
							log.Warn().Err(rollbackErr).Msg("Partial rollback failure; some shared resources may remain")
						}
					}
					if cleanupErr := cleanupPartialInfra(nsId, infraId); cleanupErr != nil {
						log.Error().Err(cleanupErr).Msg("Failed to cleanup partial Infra during rollback")
					}
					return emptyInfra, fmt.Errorf("Infra '%s' creation aborted: %d NodeGroup(s) failed resource preparation and policy=rollback; all created resources have been cleaned up",
						req.Name, len(failedNodeGroups))
				default:
					// continue or refine: proceed with the successfully prepared NodeGroups
					addErrorToHistory("Infra Status Decision",
						fmt.Sprintf("Partial success: %d NodeGroups succeeded, %d failed - continuing with partial Infra creation (policy=%s)",
							len(successfulNodeGroups), len(failedNodeGroups), req.PolicyOnPartialFailure))
				}
				UpdateInfraInfo(nsId, infraTmp)
			}
		}

		// After processing all NodeGroups, check final state
		// Get updated Infra object for final status determination
		infraTmp, _, err := GetInfraObject(nsId, infraId)
		if err != nil {
			addErrorToHistory("Infra Object Retrieval for Final Status Check", err.Error())
			return emptyInfra, err
		}

		// Final check: if no NodeGroups were successfully prepared, mark as Failed
		if len(infraReq.NodeGroups) == 0 {
			addErrorToHistory("Final Status Decision", "No NodeGroups were successfully prepared - marking Infra as Failed")
			infraTmp.SystemMessage = append(infraTmp.SystemMessage, "Infra creation failed: No NodeGroups were successfully prepared")
			infraTmp.Status = model.StatusFailed
			UpdateInfraInfo(nsId, infraTmp)

			// Build comprehensive error message
			errorMsg := fmt.Sprintf("Infra '%s' creation failed - no NodeGroups were successfully prepared.\n\n", req.Name)

			// Add full error history
			if len(errorHistory) > 0 {
				errorMsg += "Complete Error Timeline:\n"
				for i, errEntry := range errorHistory {
					errorMsg += fmt.Sprintf("  %d. %s\n", i+1, errEntry)
				}
				errorMsg += "\n"
			}

			errorMsg += "Summary: All NodeGroups failed during resource preparation phase.\n"
			errorMsg += "This indicates that no VM NodeGroups could be prepared for provisioning.\n"
			errorMsg += "Check the error timeline above for specific failure details."

			return emptyInfra, fmt.Errorf("%s", errorMsg)
		}
	}

	// Only proceed to StatusPrepared if we have successful NodeGroups
	infraTmp, _, err = GetInfraObject(nsId, infraId)
	if err != nil {
		addErrorToHistory("Infra Object Retrieval for Status Update", err.Error())
		return emptyInfra, err
	}

	// marking the infra is in StatusPrepared
	infraTmp.Status = model.StatusPrepared
	addErrorToHistory("Infra Status Update", fmt.Sprintf("Infra marked as Prepared with %d successful NodeGroups", len(infraReq.NodeGroups)))
	UpdateInfraInfo(nsId, infraTmp)

	// Log the prepared Infra request and update the progress
	common.PrintJsonPretty(infraReq)
	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{
		Title: fmt.Sprintf("Prepared %d resources for provisioning Infra: %s", len(infraReq.NodeGroups), infraReq.Name),
		Info:  infraReq, Time: time.Now(),
	})
	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{
		Title: "Start instance provisioning", Time: time.Now(),
	})

	// Run create Infra with the generated Infra request
	option := "create"
	if deployOption == "hold" {
		option = "hold"
	}
	result, err := CreateInfra(ctx, nsId, &infraReq, option, true)

	// If CreateInfra fails, build comprehensive error message with history
	if err != nil {
		// Do NOT add the full CreateInfra error to history — it will be shown once in Detail below.
		// Only record a brief note in the timeline.
		addErrorToHistory("Infra Creation", fmt.Sprintf("Infra '%s' creation failed (see Detail below)", req.Name))

		// Build comprehensive error message
		errorMsg := fmt.Sprintf("Infra '%s' creation failed in final provisioning stage.\n\n", req.Name)

		// Add error history (timeline events only, no full error duplication)
		if len(errorHistory) > 0 {
			errorMsg += "Complete Error Timeline:\n"
			for i, errEntry := range errorHistory {
				errorMsg += fmt.Sprintf("  %d. %s\n", i+1, errEntry)
			}
			errorMsg += "\n"
		}

		// Full error appears only once here
		errorMsg += fmt.Sprintf("Detail: %s\n", err.Error())

		// Check if NodeGroups is empty (which causes the validation error in CreateInfra)
		if len(infraReq.NodeGroups) == 0 {
			errorMsg += "\nRoot Cause: No VM NodeGroups were successfully prepared for provisioning.\n"
			errorMsg += "This typically indicates that all VM resource preparation failed during the earlier stages.\n"
			errorMsg += "Please check the error timeline above for specific resource creation failures (e.g., VPC limits, permissions, etc.)."
		}

		return result, fmt.Errorf("%s", errorMsg)
	}

	return result, err
}

// ValidateInfraDynamicReq is func to validate Infra dynamic request before actual provisioning
func ValidateInfraDynamicReq(ctx context.Context, nsId string, req *model.InfraDynamicReq, deployOption string) (*model.ReviewInfraDynamicReqInfo, error) {
	return ReviewInfraDynamicReq(ctx, nsId, req, deployOption)
}

// reviewSingleNodeGroupDynamicReq reviews and validates a single VM dynamic request
func reviewSingleNodeGroupDynamicReq(ctx context.Context, nodeGroupDynamicReq model.CreateNodeGroupDynamicReq, deployOption string) (model.ReviewNodeGroupDynamicReqInfo, *model.SpecInfo, bool, bool, float64) {

	credentialHolder := common.CredentialHolderFromContext(ctx)
	vmReview := model.ReviewNodeGroupDynamicReqInfo{
		VmName:        nodeGroupDynamicReq.Name,
		NodeGroupSize: nodeGroupDynamicReq.NodeGroupSize,
		CanCreate:     true,
		Status:        "Ready",
		Info:          make([]string, 0),
		Warnings:      make([]string, 0),
		Errors:        make([]string, 0),
	}

	viable := true
	hasVmWarning := false
	var specInfoPtr *model.SpecInfo
	vmCost := 0.0

	// Validate VM name
	if nodeGroupDynamicReq.Name == "" {
		vmReview.Warnings = append(vmReview.Warnings, "VM NodeGroup name not specified, will be auto-generated")
		hasVmWarning = true
	}

	// Validate NodeGroupSize
	if nodeGroupDynamicReq.NodeGroupSize <= 0 {
		nodeGroupDynamicReq.NodeGroupSize = 1
		vmReview.Warnings = append(vmReview.Warnings, "NodeGroupSize not specified, defaulting to 1")
		hasVmWarning = true
	}

	// Validate SpecId
	specInfo, err := resource.GetSpec(model.SystemCommonNs, nodeGroupDynamicReq.SpecId)
	if err != nil {
		vmReview.Errors = append(vmReview.Errors, fmt.Sprintf("Failed to get spec '%s': %v", nodeGroupDynamicReq.SpecId, err))
		vmReview.SpecValidation = model.ReviewResourceValidation{
			ResourceId:  nodeGroupDynamicReq.SpecId,
			IsAvailable: false,
			Status:      "Unavailable",
			Message:     err.Error(),
		}
		vmReview.CanCreate = false
		viable = false
	} else {
		specInfoPtr = &specInfo
		// Resolve connection name based on credential holder
		resolvedConnectionName := common.ResolveConnectionName(specInfo.ConnectionName, credentialHolder)
		vmReview.ConnectionName = resolvedConnectionName
		vmReview.ProviderName = specInfo.ProviderName
		vmReview.RegionName = specInfo.RegionName

		// Check if spec is available in CSP
		specAvailable := false
		var specCheckErr error
		cspSpecName := specInfo.CspSpecName

		if csp.ResolveCloudPlatform(specInfo.ProviderName) == csp.Azure {
			// Azure: use direct Azure Resource SKU API first (fast, bypasses CB-Spider)
			log.Debug().Str("provider", "azure").Str("region", specInfo.RegionName).Str("spec", specInfo.CspSpecName).Msg("Using direct Azure spec check for Infra review")
			specResult, azErr := azure.CheckSpecAvailability(ctx, specInfo.RegionName, specInfo.CspSpecName)
			if azErr == nil {
				specAvailable = specResult.Available
				if !specAvailable {
					specCheckErr = fmt.Errorf("%s", specResult.Reason)
				}
			} else {
				// Fall back to CB-Spider LookupSpec on Azure check errors
				log.Warn().Err(azErr).Str("provider", "azure").Str("region", specInfo.RegionName).Str("spec", specInfo.CspSpecName).Msg("Direct Azure spec check failed; falling back to CB-Spider LookupSpec")
				cspSpec, lookupErr := resource.LookupSpec(resolvedConnectionName, specInfo.CspSpecName)
				if lookupErr == nil {
					specAvailable = true
					cspSpecName = cspSpec.Name
				} else {
					specCheckErr = lookupErr
				}
			}
		} else {
			// Other providers: use CB-Spider LookupSpec
			cspSpec, lookupErr := resource.LookupSpec(resolvedConnectionName, specInfo.CspSpecName)
			if lookupErr == nil {
				specAvailable = true
				cspSpecName = cspSpec.Name
			} else {
				specCheckErr = lookupErr
			}
		}

		if specCheckErr != nil || !specAvailable {
			errMsg := "spec not available in CSP"
			if specCheckErr != nil {
				errMsg = specCheckErr.Error()
			}
			vmReview.Errors = append(vmReview.Errors, fmt.Sprintf("Spec '%s' not available in CSP: %s", nodeGroupDynamicReq.SpecId, errMsg))
			vmReview.SpecValidation = model.ReviewResourceValidation{
				ResourceId:    nodeGroupDynamicReq.SpecId,
				ResourceName:  specInfo.CspSpecName,
				IsAvailable:   false,
				Status:        "Unavailable",
				Message:       errMsg,
				CspResourceId: specInfo.CspSpecName,
			}
			vmReview.CanCreate = false
			viable = false
		} else {
			vmReview.SpecValidation = model.ReviewResourceValidation{
				ResourceId:    nodeGroupDynamicReq.SpecId,
				ResourceName:  specInfo.CspSpecName,
				IsAvailable:   true,
				Status:        "Available",
				CspResourceId: cspSpecName,
			}

			// Add cost estimation if available
			if specInfo.CostPerHour > 0 {
				nodeGroupSizeInt := nodeGroupDynamicReq.NodeGroupSize
				if nodeGroupSizeInt < 1 {
					nodeGroupSizeInt = 1
				}
				vmReview.EstimatedCost = fmt.Sprintf("$%.4f/hour", float64(specInfo.CostPerHour)*float64(nodeGroupSizeInt))
				vmCost = float64(specInfo.CostPerHour) * float64(nodeGroupSizeInt)
			} else {
				vmReview.EstimatedCost = "Cost estimation unavailable"
			}
		}
	}

	// Validate ImageId (with auto-registration if found in CSP but not in DB)
	if specInfoPtr != nil {
		resolvedConnName := common.ResolveConnectionName(specInfoPtr.ConnectionName, credentialHolder)
		imageInfo, isAutoRegistered, err := resource.EnsureImageAvailable(model.SystemCommonNs, resolvedConnName, nodeGroupDynamicReq.ImageId)
		if err != nil {
			vmReview.Errors = append(vmReview.Errors, fmt.Sprintf("Image '%s' not available: %v", nodeGroupDynamicReq.ImageId, err))
			vmReview.ImageValidation = model.ReviewResourceValidation{
				ResourceId:    nodeGroupDynamicReq.ImageId,
				IsAvailable:   false,
				Status:        "Unavailable",
				Message:       err.Error(),
				CspResourceId: nodeGroupDynamicReq.ImageId,
			}
			vmReview.CanCreate = false
			viable = false
		} else {
			status := "Available"
			if isAutoRegistered {
				status = "Available (Auto-registered)"
				vmReview.Info = append(vmReview.Info, fmt.Sprintf("Image '%s' was auto-registered from CSP", nodeGroupDynamicReq.ImageId))
			}
			vmReview.ImageValidation = model.ReviewResourceValidation{
				ResourceId:    nodeGroupDynamicReq.ImageId,
				ResourceName:  imageInfo.Name,
				IsAvailable:   true,
				Status:        status,
				CspResourceId: imageInfo.CspImageName,
			}
		}
	}

	// Validate ConnectionName if specified
	if nodeGroupDynamicReq.ConnectionName != "" {
		_, err := common.GetConnConfig(nodeGroupDynamicReq.ConnectionName)
		if err != nil {
			vmReview.Warnings = append(vmReview.Warnings, fmt.Sprintf("Specified connection '%s' not found, will use default from spec", nodeGroupDynamicReq.ConnectionName))
			hasVmWarning = true
		} else {
			vmReview.ConnectionName = nodeGroupDynamicReq.ConnectionName
		}
	}

	// Validate RootDisk settings
	if nodeGroupDynamicReq.RootDiskType != "" && nodeGroupDynamicReq.RootDiskType != "default" {
		vmReview.Info = append(vmReview.Info, fmt.Sprintf("Root disk type configured: %s, be sure it's supported by the provider", nodeGroupDynamicReq.RootDiskType))
	}
	if nodeGroupDynamicReq.RootDiskSize > 0 {
		vmReview.Info = append(vmReview.Info, fmt.Sprintf("Root disk size configured: %d GB, be sure it meets minimum requirements", nodeGroupDynamicReq.RootDiskSize))
	}

	// Check provisioning history and risk analysis
	if specInfoPtr != nil {
		riskAnalysis, err := AnalyzeProvisioningRiskDetailed(nodeGroupDynamicReq.SpecId, nodeGroupDynamicReq.ImageId)
		if err != nil {
			log.Warn().Err(err).Msgf("Failed to analyze provisioning risk for VM: %s", nodeGroupDynamicReq.Name)
			vmReview.Warnings = append(vmReview.Warnings, "Failed to analyze provisioning history")
		} else {
			riskLevel := riskAnalysis.OverallRisk.Level
			riskMessage := riskAnalysis.OverallRisk.Message

			// Include recent failure messages if available
			var fullRiskMessage string
			if len(riskAnalysis.RecentFailureMessages) > 0 {
				fullRiskMessage = fmt.Sprintf("%s. Recent failure examples: %s",
					riskMessage, strings.Join(riskAnalysis.RecentFailureMessages, "; "))
			} else {
				fullRiskMessage = riskMessage
			}

			switch riskLevel {
			case "high":
				vmReview.Errors = append(vmReview.Errors, fmt.Sprintf("High provisioning failure risk: %s", fullRiskMessage))
				vmReview.CanCreate = false
				viable = false
				log.Debug().Msgf("High risk detected for spec %s with image %s: %s", nodeGroupDynamicReq.SpecId, nodeGroupDynamicReq.ImageId, riskMessage)
			case "medium":
				vmReview.Warnings = append(vmReview.Warnings, fmt.Sprintf("Moderate provisioning failure risk: %s", fullRiskMessage))
				hasVmWarning = true
				log.Debug().Msgf("Medium risk detected for spec %s with image %s: %s", nodeGroupDynamicReq.SpecId, nodeGroupDynamicReq.ImageId, riskMessage)
			case "low":
				if riskMessage != "No previous provisioning history available" && riskMessage != "No provisioning attempts recorded" {
					vmReview.Info = append(vmReview.Info, fmt.Sprintf("Provisioning history: %s", riskMessage))
				}
				log.Debug().Msgf("Low risk for spec %s with image %s: %s", nodeGroupDynamicReq.SpecId, nodeGroupDynamicReq.ImageId, riskMessage)
			default:
				log.Debug().Msgf("Unknown risk level for spec %s: %s", nodeGroupDynamicReq.SpecId, riskLevel)
			}
		}
	}

	// Check for provider-specific limitations
	if specInfoPtr != nil {
		providerName := specInfoPtr.ProviderName

		// Check KT Cloud limitations - temporary restriction to .itl specs only
		if csp.ResolveCloudPlatform(providerName) == csp.KT {
			if !strings.Contains(nodeGroupDynamicReq.SpecId, ".itl") {
				// Only show warning when spec does not contain '.itl'
				vmReview.Warnings = append(vmReview.Warnings, "KT Cloud provisioning is currently limited to '.itl' specs only (temporary limitation). This spec may fail to provision.")
				hasVmWarning = true
				log.Debug().Msgf("KT Cloud warning for VM: %s (spec: %s does not contain '.itl')", nodeGroupDynamicReq.Name, nodeGroupDynamicReq.SpecId)
			} else {
				// '.itl' spec is valid, no warning needed
				log.Debug().Msgf("KT Cloud '.itl' spec detected for VM: %s (spec: %s)", nodeGroupDynamicReq.Name, nodeGroupDynamicReq.SpecId)
			}
		}

		// // Check NHN Cloud limitations
		// if providerName == csp.NHN {
		// 	if deployOption != "hold" {
		// 		vmReview.Errors = append(vmReview.Errors, "NHN Cloud can only be provisioned with deployOption 'hold' (manual deployment required)")
		// 		vmReview.CanCreate = false
		// 		viable = false
		// 		log.Debug().Msgf("NHN Cloud requires 'hold' deployOption for VM: %s", nodeGroupDynamicReq.Name)
		// 	} else {
		// 		vmReview.Warnings = append(vmReview.Warnings, "NHN Cloud requires manual deployment completion after 'hold' - automatic provisioning is not fully supported")
		// 		hasVmWarning = true
		// 		log.Debug().Msgf("NHN Cloud 'hold' mode warning for VM: %s", nodeGroupDynamicReq.Name)
		// 	}
		// }
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

	log.Debug().Msgf("VM '%s' review completed: %s", nodeGroupDynamicReq.Name, vmReview.Status)
	return vmReview, specInfoPtr, viable, hasVmWarning, vmCost
}

// ReviewSpecImagePair reviews spec and image pair compatibility for provisioning
func ReviewSpecImagePair(ctx context.Context, specId, imageId string) (*model.SpecImagePairReviewResult, error) {
	log.Debug().Msgf("Reviewing spec-image pair: spec=%s, image=%s", specId, imageId)

	result := &model.SpecImagePairReviewResult{
		SpecId:   specId,
		ImageId:  imageId,
		IsValid:  true,
		Status:   "OK",
		Info:     make([]string, 0),
		Warnings: make([]string, 0),
		Errors:   make([]string, 0),
	}

	// Validate SpecId
	specInfo, err := resource.GetSpec(model.SystemCommonNs, specId)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to get spec '%s': %v", specId, err))
		result.SpecValidation = model.ReviewResourceValidation{
			ResourceId:  specId,
			IsAvailable: false,
			Status:      "Unavailable",
			Message:     err.Error(),
		}
		result.IsValid = false
		result.Status = "Error"
		result.Message = fmt.Sprintf("Spec '%s' is not available", specId)
	} else {
		result.SpecDetails = &specInfo
		result.ConnectionName = specInfo.ConnectionName
		result.ProviderName = specInfo.ProviderName
		result.RegionName = specInfo.RegionName

		// Check if spec is available in CSP
		specAvailable := false
		var specCheckErr error

		if csp.ResolveCloudPlatform(specInfo.ProviderName) == csp.Azure {
			// Azure: use direct Azure Resource SKU API first (fast, bypasses CB-Spider)
			log.Debug().Str("provider", "azure").Str("region", specInfo.RegionName).Str("spec", specInfo.CspSpecName).Msg("Using direct Azure spec check")
			specResult, azErr := azure.CheckSpecAvailability(ctx, specInfo.RegionName, specInfo.CspSpecName)
			if azErr == nil {
				specAvailable = specResult.Available
				if !specAvailable {
					// Azure positively reported the spec as unavailable; treat as authoritative.
					specCheckErr = fmt.Errorf("%s", specResult.Reason)
				}
			} else {
				// Fall back to CB-Spider LookupSpec on Azure check errors to preserve existing behavior.
				log.Warn().Err(azErr).Str("provider", "azure").Str("region", specInfo.RegionName).Str("spec", specInfo.CspSpecName).Msg("Direct Azure spec check failed; falling back to CB-Spider LookupSpec")
				_, specCheckErr = resource.LookupSpec(specInfo.ConnectionName, specInfo.CspSpecName)
				if specCheckErr == nil {
					specAvailable = true
				}
			}
		} else {
			// Other providers: use CB-Spider LookupSpec
			_, specCheckErr = resource.LookupSpec(specInfo.ConnectionName, specInfo.CspSpecName)
			if specCheckErr == nil {
				specAvailable = true
			}
		}

		if specCheckErr != nil || !specAvailable {
			errMsg := "spec not available in CSP"
			if specCheckErr != nil {
				errMsg = specCheckErr.Error()
			}
			result.Errors = append(result.Errors, fmt.Sprintf("Spec '%s' not available in CSP: %s", specId, errMsg))
			result.SpecValidation = model.ReviewResourceValidation{
				ResourceId:    specId,
				ResourceName:  specInfo.CspSpecName,
				IsAvailable:   false,
				Status:        "Unavailable",
				Message:       errMsg,
				CspResourceId: specInfo.CspSpecName,
			}
			result.IsValid = false
			result.Status = "Error"
			result.Message = fmt.Sprintf("Spec '%s' is not available in CSP", specId)
		} else {
			result.SpecValidation = model.ReviewResourceValidation{
				ResourceId:    specId,
				ResourceName:  specInfo.CspSpecName,
				IsAvailable:   true,
				Status:        "Available",
				CspResourceId: specInfo.CspSpecName,
			}

			// Add cost estimation if available
			if specInfo.CostPerHour > 0 {
				result.EstimatedCost = fmt.Sprintf("$%.4f/hour", specInfo.CostPerHour)
			}
		}
	}

	// Validate ImageId (with auto-registration if found in CSP but not in DB)
	if result.ConnectionName != "" {
		imageInfo, isAutoRegistered, err := resource.EnsureImageAvailable(model.SystemCommonNs, result.ConnectionName, imageId)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Image '%s' not available: %v", imageId, err))
			result.ImageValidation = model.ReviewResourceValidation{
				ResourceId:    imageId,
				IsAvailable:   false,
				Status:        "Unavailable",
				Message:       err.Error(),
				CspResourceId: imageId,
			}
			result.IsValid = false
			result.Status = "Error"
			if result.Message == "" {
				result.Message = fmt.Sprintf("Image '%s' is not available", imageId)
			} else {
				result.Message += fmt.Sprintf("; Image '%s' is not available", imageId)
			}
		} else {
			result.ImageDetails = &imageInfo
			status := "Available"
			if isAutoRegistered {
				status = "Available (Auto-registered)"
				result.Info = append(result.Info, fmt.Sprintf("Image '%s' was auto-registered from CSP", imageId))
			}
			result.ImageValidation = model.ReviewResourceValidation{
				ResourceId:    imageId,
				ResourceName:  imageInfo.Name,
				IsAvailable:   true,
				Status:        status,
				CspResourceId: imageInfo.CspImageName,
			}
		}
	} else {
		// Cannot validate image without connection info from spec
		result.ImageValidation = model.ReviewResourceValidation{
			ResourceId:  imageId,
			IsAvailable: false,
			Status:      "Unknown",
			Message:     "Cannot validate image without valid spec",
		}
	}

	// Check provisioning history and risk analysis
	if result.SpecValidation.IsAvailable {
		riskAnalysis, err := AnalyzeProvisioningRiskDetailed(specId, imageId)
		if err != nil {
			log.Warn().Err(err).Msgf("Failed to analyze provisioning risk for spec-image pair: %s / %s", specId, imageId)
			result.Warnings = append(result.Warnings, "Failed to analyze provisioning history")
		} else {
			riskLevel := riskAnalysis.OverallRisk.Level
			riskMessage := riskAnalysis.OverallRisk.Message

			// Include recent failure messages if available
			var fullRiskMessage string
			if len(riskAnalysis.RecentFailureMessages) > 0 {
				fullRiskMessage = fmt.Sprintf("%s. Recent failures: %s",
					riskMessage, strings.Join(riskAnalysis.RecentFailureMessages, "; "))
			} else {
				fullRiskMessage = riskMessage
			}

			switch riskLevel {
			case "high":
				result.Errors = append(result.Errors, fmt.Sprintf("High provisioning failure risk: %s", fullRiskMessage))
				result.IsValid = false
				result.Status = "Error"
				if result.Message == "" {
					result.Message = "High provisioning failure risk detected"
				} else {
					result.Message += "; High provisioning failure risk detected"
				}
				log.Debug().Msgf("High risk detected for spec %s with image %s: %s", specId, imageId, riskMessage)
			case "medium":
				result.Warnings = append(result.Warnings, fmt.Sprintf("Moderate provisioning failure risk: %s", fullRiskMessage))
				if result.Status == "OK" {
					result.Status = "Warning"
				}
				log.Debug().Msgf("Medium risk detected for spec %s with image %s: %s", specId, imageId, riskMessage)
			case "low":
				if riskMessage != "No previous provisioning history available" && riskMessage != "No provisioning attempts recorded" {
					result.Info = append(result.Info, fmt.Sprintf("Provisioning history: %s", riskMessage))
				}
				log.Debug().Msgf("Low risk for spec %s with image %s: %s", specId, imageId, riskMessage)
			}
		}
	}

	// Set final message if valid
	if result.IsValid {
		if result.Status == "Warning" {
			result.Message = "Spec and image pair is valid but has warnings"
		} else {
			result.Message = "Spec and image pair is valid for provisioning"
		}
	}

	log.Debug().Msgf("Spec-image pair review completed: %s - %s", result.Status, result.Message)
	return result, nil
}

// ReviewSingleNodeGroupDynamicReq reviews and validates a single VM dynamic request and returns comprehensive review information
func ReviewSingleNodeGroupDynamicReq(ctx context.Context, nsId string, req *model.CreateNodeGroupDynamicReq) (*model.ReviewNodeGroupDynamicReqInfo, error) {
	log.Debug().Msgf("Starting single VM dynamic request review for: %s", req.Name)

	// Basic validation
	err := common.CheckString(nsId)
	if err != nil {
		return nil, fmt.Errorf("invalid namespace: %w", err)
	}

	// Use the common VM review function with empty deployOption
	vmReview, _, _, _, _ := reviewSingleNodeGroupDynamicReq(ctx, *req, "")

	log.Debug().Msgf("Single VM review completed: %s - %s", vmReview.Status, vmReview.Message)
	return &vmReview, nil
}

// ReviewInfraDynamicReq is func to review and validate Infra dynamic request comprehensively
func ReviewInfraDynamicReq(ctx context.Context, nsId string, req *model.InfraDynamicReq, deployOption string) (*model.ReviewInfraDynamicReqInfo, error) {

	log.Debug().Msgf("Starting Infra dynamic request review for: %s", req.Name)

	reviewResult := &model.ReviewInfraDynamicReqInfo{
		InfraName:    req.Name,
		TotalVmCount: len(req.NodeGroups),
		VmReviews:    make([]model.ReviewNodeGroupDynamicReqInfo, 0),
		ResourceSummary: model.ReviewResourceSummary{
			UniqueSpecs:     make([]string, 0),
			UniqueImages:    make([]string, 0),
			ConnectionNames: make([]string, 0),
			ProviderNames:   make([]string, 0),
			RegionNames:     make([]string, 0),
		},
		Recommendations:        make([]string, 0),
		PolicyOnPartialFailure: req.PolicyOnPartialFailure,
	}

	// Basic validation
	err := common.CheckString(nsId)
	if err != nil {
		return nil, fmt.Errorf("invalid namespace: %w", err)
	}

	// Check if Infra name is valid and doesn't exist
	check, err := CheckInfra(nsId, req.Name)
	if err != nil {
		return nil, fmt.Errorf("invalid infra name: %w", err)
	}
	if check {
		reviewResult.OverallStatus = "Error"
		reviewResult.OverallMessage = fmt.Sprintf("Infra '%s' already exists in namespace '%s'", req.Name, nsId)
		reviewResult.CreationViable = false
		return reviewResult, nil
	}

	if len(req.NodeGroups) == 0 {
		reviewResult.OverallStatus = "Error"
		reviewResult.OverallMessage = "No VM requests provided"
		reviewResult.CreationViable = false
		return reviewResult, nil
	}

	// Track resource summary with thread-safe maps
	specMap := make(map[string]bool)
	imageMap := make(map[string]bool)
	connectionMap := make(map[string]bool)
	providerMap := make(map[string]bool)
	regionMap := make(map[string]bool)

	// Use semaphore for parallel processing with concurrency limit
	const maxConcurrency = 10
	semaphore := make(chan struct{}, maxConcurrency)

	// Channel to collect VM review results
	vmReviewChan := make(chan struct {
		index    int
		vmReview model.ReviewNodeGroupDynamicReqInfo
		specInfo *model.SpecInfo
		viable   bool
		warning  bool
		cost     float64
	}, len(req.NodeGroups))

	// WaitGroup to wait for all goroutines to complete
	var wg sync.WaitGroup

	// Validate each VM request in parallel
	for i, nodeGroupReq := range req.NodeGroups {
		wg.Add(1)
		go func(index int, nodeGroupDynamicReq model.CreateNodeGroupDynamicReq) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Use the common VM review function
			vmReview, specInfoPtr, viable, hasVmWarning, vmCost := reviewSingleNodeGroupDynamicReq(ctx, nodeGroupDynamicReq, deployOption)

			// Send result to channel
			vmReviewChan <- struct {
				index    int
				vmReview model.ReviewNodeGroupDynamicReqInfo
				specInfo *model.SpecInfo
				viable   bool
				warning  bool
				cost     float64
			}{
				index:    index,
				vmReview: vmReview,
				specInfo: specInfoPtr,
				viable:   viable,
				warning:  hasVmWarning,
				cost:     vmCost,
			}

			log.Debug().Msgf("[%d] VM '%s' review completed: %s", index, nodeGroupDynamicReq.Name, vmReview.Status)
		}(i, nodeGroupReq)
	}

	// Close channel when all goroutines are done
	go func() {
		wg.Wait()
		close(vmReviewChan)
	}()

	// Collect results and maintain order
	vmReviews := make([]model.ReviewNodeGroupDynamicReqInfo, len(req.NodeGroups))
	allViable := true
	hasWarnings := false
	totalEstimatedCost := 0.0
	vmWithUnknownCost := 0

	// Process results from channel
	for result := range vmReviewChan {
		// Store VM review result in correct order
		vmReviews[result.index] = result.vmReview

		// Update overall status flags
		if !result.viable {
			allViable = false
		}
		if result.warning {
			hasWarnings = true
		}

		// Update cost calculation
		if result.cost > 0 {
			totalEstimatedCost += result.cost
		} else if result.vmReview.EstimatedCost == "Cost estimation unavailable" {
			vmWithUnknownCost++
		}

		// Update resource summary maps (thread-safe since we're processing sequentially here)
		if result.specInfo != nil {
			specMap[req.NodeGroups[result.index].SpecId] = true
			connectionMap[result.specInfo.ConnectionName] = true
			providerMap[result.specInfo.ProviderName] = true
			regionMap[result.specInfo.RegionName] = true
		}

		if req.NodeGroups[result.index].ImageId != "" {
			imageMap[req.NodeGroups[result.index].ImageId] = true
		}
	}

	// Store VM reviews in result
	reviewResult.VmReviews = vmReviews

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
		reviewResult.OverallMessage = fmt.Sprintf("Infra cannot be created due to critical errors in VM configurations (Providers: %v, Regions: %v)",
			reviewResult.ResourceSummary.ProviderNames, reviewResult.ResourceSummary.RegionNames)
		reviewResult.Recommendations = append(reviewResult.Recommendations, "Fix all VM configuration errors before attempting to create Infra")
	} else if hasWarnings {
		reviewResult.OverallStatus = "Warning"
		reviewResult.OverallMessage = fmt.Sprintf("Infra can be created but has some configuration warnings (Providers: %v, Regions: %v)",
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

	// Add PolicyOnPartialFailure analysis and recommendations
	policy := req.PolicyOnPartialFailure
	if policy == "" {
		policy = model.PolicyContinue // default value
		reviewResult.PolicyOnPartialFailure = model.PolicyContinue
	}

	var policyDescription, policyRecommendation string

	switch policy {
	case model.PolicyContinue:
		policyDescription = "If some VMs fail during creation, Infra will be created with successfully provisioned VMs only. Failed VMs will remain in 'StatusFailed' state and can be fixed later using 'refine' action."
		reviewResult.Recommendations = append(reviewResult.Recommendations,
			"Failure Policy: 'continue' - Partial deployment allowed, failed VMs can be refined later")
		if reviewResult.TotalVmCount > 1 {
			policyRecommendation = "With multiple VMs, consider 'rollback' policy for all-or-nothing deployment, or 'refine' policy for automatic cleanup"
			reviewResult.Recommendations = append(reviewResult.Recommendations,
				"With multiple VMs, partial failures are possible. Consider using 'rollback' policy if you need all-or-nothing deployment, or 'refine' policy for automatic cleanup of failed VMs.")
		}
	case model.PolicyRollback:
		policyDescription = "If any VM fails during creation, the entire Infra will be deleted automatically. This ensures all-or-nothing deployment but may waste resources if only a few VMs fail."
		reviewResult.Recommendations = append(reviewResult.Recommendations,
			"Failure Policy: 'rollback' - All-or-nothing deployment, entire Infra deleted on any failure")
		if reviewResult.TotalVmCount > 5 {
			policyRecommendation = "With many VMs, rollback policy increases risk of complete deployment failure. Consider 'continue' or 'refine' policy for better reliability"
			reviewResult.Recommendations = append(reviewResult.Recommendations,
				"WARNING: With many VMs, rollback policy increases risk of complete deployment failure. Consider 'continue' or 'refine' policy for better reliability.")
		}
		if reviewResult.ResourceSummary.TotalProviders > 2 {
			reviewResult.Recommendations = append(reviewResult.Recommendations,
				"WARNING: Multiple cloud providers increase failure probability. Rollback policy may cause complete deployment failure due to single provider issues.")
		}
	case model.PolicyRefine:
		policyDescription = "If some VMs fail during creation, Infra will be created with successful VMs, and failed VMs will be automatically cleaned up using refine action. This provides the best balance between reliability and resource efficiency."
		reviewResult.Recommendations = append(reviewResult.Recommendations,
			"Failure Policy: 'refine' - Automatic cleanup of failed VMs, optimal balance of reliability and efficiency")
		if reviewResult.TotalVmCount > 10 {
			policyRecommendation = "With many VMs, 'refine' policy provides optimal balance between reliability and resource efficiency"
			reviewResult.Recommendations = append(reviewResult.Recommendations,
				"RECOMMENDED: With many VMs, 'refine' policy provides optimal balance between reliability and resource efficiency.")
		}
	default:
		policyDescription = fmt.Sprintf("Unknown failure policy '%s'. Will default to 'continue'. Valid options: continue, rollback, refine", policy)
		policyRecommendation = "Use one of the valid failure policies: continue, rollback, refine"
		reviewResult.Recommendations = append(reviewResult.Recommendations,
			fmt.Sprintf("WARNING: Unknown failure policy '%s'. Will default to 'continue'. Valid options: continue, rollback, refine", policy))
	}

	reviewResult.PolicyDescription = policyDescription
	reviewResult.PolicyRecommendation = policyRecommendation

	// Add policy-specific warnings based on deployment context
	if reviewResult.OverallStatus == "Warning" && policy == model.PolicyRollback {
		reviewResult.Recommendations = append(reviewResult.Recommendations,
			"CAUTION: Configuration warnings detected with 'rollback' policy. Address warnings to prevent complete deployment failure.")
	}

	if len(reviewResult.ResourceSummary.ProviderNames) > 1 && policy == model.PolicyRollback {
		reviewResult.Recommendations = append(reviewResult.Recommendations,
			"TIP: Multi-cloud deployment with 'rollback' policy is risky. Consider 'refine' policy for better fault tolerance across providers.")
	}

	if deployOption == "hold" {
		reviewResult.Recommendations = append(reviewResult.Recommendations,
			fmt.Sprintf("DEPLOYMENT HOLD: Infra creation will be held for review. Failure policy '%s' will apply when deployment is resumed with control continue.", policy))
	}

	// Add provider-specific global recommendations
	for _, providerName := range reviewResult.ResourceSummary.ProviderNames {
		switch csp.ResolveCloudPlatform(providerName) {
		case csp.KT:
			reviewResult.Recommendations = append(reviewResult.Recommendations,
				"NOTICE: KT Cloud provisioning is currently limited to specs with '.itl' in the name (temporary limitation)")
			// case csp.NHN:
			// 	if deployOption != "hold" {
			// 		reviewResult.Recommendations = append(reviewResult.Recommendations,
			// 			"CRITICAL: NHN Cloud requires deployOption 'hold' for manual deployment - automatic provisioning will fail")
			// 	} else {
			// 		reviewResult.Recommendations = append(reviewResult.Recommendations,
			// 			"INFO: NHN Cloud deployment will be held for manual completion - automatic provisioning is not fully supported")
			// 	}
		}
	}

	log.Debug().Msgf("Infra review completed: %s - %s (Policy: %s)", reviewResult.OverallStatus, reviewResult.OverallMessage, policy)
	return reviewResult, nil
}

// CreateSystemInfraDynamic is func to create Infra obeject and deploy requested VMs in a dynamic way
func CreateSystemInfraDynamic(option string) (*model.InfraInfo, error) {
	nsId := model.SystemCommonNs
	req := &model.InfraDynamicReq{}

	// special purpose Infra
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

			nodeGroupDynamicReq := &model.CreateNodeGroupDynamicReq{}
			nodeGroupDynamicReq.ImageId = "ubuntu22.04"                // temporal default value. will be changed
			nodeGroupDynamicReq.SpecId = "aws-ap-northeast-2-t2-small" // temporal default value. will be changed

			recommendSpecReq := model.RecommendSpecReq{}
			condition := []model.Operation{}
			condition = append(condition, model.Operation{Operand: v.RegionZoneInfoName})

			log.Debug().Msg(" - v.RegionName: " + v.RegionZoneInfoName)

			recommendSpecReq.Filter.Policy = append(recommendSpecReq.Filter.Policy, model.FilterCondition{Metric: "region", Condition: condition})
			recommendSpecReq.Limit = 1
			common.PrintJsonPretty(recommendSpecReq)

			specList, err := RecommendSpec(common.NewDefaultContext(), model.SystemCommonNs, recommendSpecReq)
			if err != nil {
				log.Error().Err(err).Msg("")
				return nil, err
			}
			if len(specList) != 0 {
				recommendedSpec := specList[0].Id
				nodeGroupDynamicReq.SpecId = recommendedSpec

				nodeGroupDynamicReq.Label = labels
				nodeGroupDynamicReq.Name = nodeGroupDynamicReq.SpecId

				nodeGroupDynamicReq.RootDiskType = specList[0].RootDiskType
				nodeGroupDynamicReq.RootDiskSize = specList[0].RootDiskSize
				req.NodeGroups = append(req.NodeGroups, *nodeGroupDynamicReq)
			}
		}

	default:
		err := fmt.Errorf("Not available option. Try (option=probe)")
		return nil, err
	}
	if req.NodeGroups == nil {
		err := fmt.Errorf("No VM is defined")
		return nil, err
	}

	return CreateInfraDynamic(common.NewDefaultContext(), nsId, req, "")
}

// CreateInfraNodeGroupDynamic is func to create requested VM in a dynamic way and add it to Infra
func CreateInfraNodeGroupDynamic(ctx context.Context, nsId string, infraId string, req *model.CreateNodeGroupDynamicReq) (*model.InfraInfo, error) {

	emptyInfra := &model.InfraInfo{}
	nodeGroupId := req.Name
	check, err := CheckNodeGroup(nsId, infraId, nodeGroupId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyInfra, err
	}
	if check {
		err := fmt.Errorf("The name for NodeGroup (prefix of VM Id) " + req.Name + " already exists.")
		return emptyInfra, err
	}

	err = checkCommonResAvailableForNodeGroupDynamicReq(ctx, req, nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyInfra, err
	}

	vmReqResult, err := getNodeGroupReqFromDynamicReq(ctx, nsId, req)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyInfra, err
	}

	return CreateInfraGroupVm(ctx, nsId, infraId, vmReqResult.VmReq, true)
}

// checkCommonResAvailableForNodeGroupDynamicReq is func to check common resources availability for NodeGroupDynamicReq
func checkCommonResAvailableForNodeGroupDynamicReq(ctx context.Context, req *model.CreateNodeGroupDynamicReq, nsId string) error {

	credentialHolder := common.CredentialHolderFromContext(ctx)

	log.Debug().Msgf("Checking common resources for VM Dynamic Request: %+v", req)
	log.Debug().Msgf("Namespace ID: %s", nsId)

	// Get spec info first (required for both spec and image validation)
	specInfo, err := resource.GetSpec(model.SystemCommonNs, req.SpecId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get spec info")
		return fmt.Errorf("failed to get VM specification '%s': %w", req.SpecId, err)
	}

	// Resolve connection name based on credential holder
	resolvedConnectionName := common.ResolveConnectionName(specInfo.ConnectionName, credentialHolder)

	// Channel to collect errors from parallel goroutines
	errorChan := make(chan error, 2)

	// Check spec availability in parallel
	go func() {
		var specAvailable bool
		var specCheckErr error

		if csp.ResolveCloudPlatform(specInfo.ProviderName) == csp.Azure {
			// Azure: use direct Azure Resource SKU API first (fast, bypasses CB-Spider)
			log.Debug().Str("provider", "azure").Str("region", specInfo.RegionName).Str("spec", specInfo.CspSpecName).Msg("Using direct Azure spec check")
			specResult, azErr := azure.CheckSpecAvailability(ctx, specInfo.RegionName, specInfo.CspSpecName)
			if azErr == nil {
				specAvailable = specResult.Available
				if !specAvailable {
					specCheckErr = fmt.Errorf("%s", specResult.Reason)
				}
			} else {
				// Fall back to CB-Spider LookupSpec on Azure check errors
				log.Warn().Err(azErr).Str("provider", "azure").Str("region", specInfo.RegionName).Str("spec", specInfo.CspSpecName).Msg("Direct Azure spec check failed; falling back to CB-Spider LookupSpec")
				_, specCheckErr = resource.LookupSpec(resolvedConnectionName, specInfo.CspSpecName)
				if specCheckErr == nil {
					specAvailable = true
				}
			}
		} else {
			// Other providers: use CB-Spider LookupSpec
			_, specCheckErr = resource.LookupSpec(resolvedConnectionName, specInfo.CspSpecName)
			if specCheckErr == nil {
				specAvailable = true
			}
		}

		if specCheckErr != nil || !specAvailable {
			errMsg := "spec not available in CSP"
			if specCheckErr != nil {
				errMsg = specCheckErr.Error()
			}
			log.Error().Msgf("Spec validation failed for %s: %s", specInfo.CspSpecName, errMsg)
			errorChan <- fmt.Errorf("spec '%s' is not available in connection '%s': %s",
				specInfo.CspSpecName, resolvedConnectionName, errMsg)
		} else {
			log.Debug().Msgf("Spec validation successful: %s", specInfo.CspSpecName)
			errorChan <- nil
		}
	}()

	// Check image availability in parallel (with auto-registration if found in CSP but not in DB)
	go func() {
		_, isAutoRegistered, err := resource.EnsureImageAvailable(model.SystemCommonNs, resolvedConnectionName, req.ImageId)
		if err != nil {
			log.Error().Err(err).Msgf("Image validation failed for %s", req.ImageId)
			errorChan <- fmt.Errorf("image '%s' is not available in connection '%s': %w",
				req.ImageId, resolvedConnectionName, err)
		} else {
			if isAutoRegistered {
				log.Info().Msgf("Image '%s' was auto-registered from CSP", req.ImageId)
			}
			log.Debug().Msgf("Image validation successful: %s", req.ImageId)
			errorChan <- nil
		}
	}()

	// Collect errors from both goroutines
	var errorMessages []string
	for i := 0; i < 2; i++ {

		if err := <-errorChan; err != nil {
			errorMessages = append(errorMessages, err.Error())
		}
	}

	// Return combined error if any validation failed
	if len(errorMessages) > 0 {
		combinedError := fmt.Errorf("validation failed for VM '%s': %s",
			req.Name, strings.Join(errorMessages, "; "))
		log.Error().Err(combinedError).Msg("Resource validation failures")
		return combinedError
	}

	log.Debug().Msgf("All resource validations passed for VM: %s", req.Name)
	return nil
}

// waitForVNetReady waits for VNet to be in a ready state with timeout and retry mechanism
func waitForVNetReady(ctx context.Context, nsId string, vNetId string) error {
	reqID := common.RequestIDFromContext(ctx)

	const (
		maxRetries             = 200
		retryInterval          = 5 * time.Second
		progressUpdateInterval = 10 // Update progress every 10 attempts (50 seconds)
	)
	// 1000 Secs

	log.Debug().Msgf("Waiting for VNet '%s' to be ready", vNetId)

	// Initial progress update
	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{
		Title: fmt.Sprintf("Waiting for VNet ready: %s", vNetId),
		Time:  time.Now(),
	})

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Update progress less frequently (only on first attempt and every progressUpdateInterval attempts)
		if attempt == 1 || attempt%progressUpdateInterval == 0 {
			clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{
				Title: fmt.Sprintf("Waiting for VNet ready: %s (attempt %d/%d)", vNetId, attempt, maxRetries),
				Time:  time.Now(),
			})
		}

		// Get VNet info using the dedicated function
		vNetInfo, err := resource.GetVNet(nsId, vNetId)
		if err != nil {
			log.Warn().Err(err).Msgf("Failed to get VNet '%s' on attempt %d", vNetId, attempt)
			time.Sleep(retryInterval)
			continue
		}

		// Check if VNet is ready
		if vNetInfo.Status == model.NetworkStatusAvailable {
			log.Info().Msgf("VNet '%s' is ready with status: %s", vNetId, vNetInfo.Status)
			// Final success progress update
			clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{
				Title: fmt.Sprintf("VNet ready: %s (status: %s)", vNetId, vNetInfo.Status),
				Time:  time.Now(),
			})
			return nil
		}

		// Check for error states
		if strings.Contains(strings.ToLower(vNetInfo.Status), "error") {
			return fmt.Errorf("VNet '%s' is in error state: %s", vNetId, vNetInfo.Status)
		}

		log.Debug().Msgf("VNet '%s' not ready yet, status: %s (attempt %d/%d)", vNetId, vNetInfo.Status, attempt, maxRetries)
		time.Sleep(retryInterval)
	}

	return fmt.Errorf("timeout waiting for VNet '%s' to be ready after %d minutes", vNetId, (maxRetries*int(retryInterval.Seconds()))/60)
}

// getNodeGroupReqFromDynamicReq is func to getNodeGroupReqFromDynamicReq with created resource tracking
func getNodeGroupReqFromDynamicReq(ctx context.Context, nsId string, req *model.CreateNodeGroupDynamicReq) (*VmReqWithCreatedResources, error) {

	reqID := common.RequestIDFromContext(ctx)
	credentialHolder := common.CredentialHolderFromContext(ctx)

	onDemand := true
	var createdResources []CreatedResource

	vmRequest := req
	// Check whether VM names meet requirement.
	k := vmRequest

	nodeGroupReq := &model.CreateNodeGroupReq{}

	specInfo, err := resource.GetSpec(model.SystemCommonNs, req.SpecId)
	if err != nil {
		detailedErr := fmt.Errorf("failed to find VM specification '%s': %w. Please verify the spec exists and is properly configured", req.SpecId, err)
		log.Error().Err(err).Msgf("Spec lookup failed for VM '%s' with SpecId '%s'", req.Name, req.SpecId)
		return &VmReqWithCreatedResources{VmReq: &model.CreateNodeGroupReq{Name: req.Name}, CreatedResources: createdResources}, detailedErr
	}

	// remake vmReqest from given input and check resource availability
	// Resolve connection name based on credential holder
	nodeGroupReq.ConnectionName = common.ResolveConnectionName(specInfo.ConnectionName, credentialHolder)

	// If ConnectionName is specified by the request, Use ConnectionName from the request
	if k.ConnectionName != "" {
		nodeGroupReq.ConnectionName = k.ConnectionName
	}

	// validate the GetConnConfig for spec
	connection, err := common.GetConnConfig(nodeGroupReq.ConnectionName)
	if err != nil {
		detailedErr := fmt.Errorf("failed to get connection configuration '%s' for VM '%s' with spec '%s': %w. Please verify the connection exists and is properly configured",
			nodeGroupReq.ConnectionName, req.Name, k.SpecId, err)
		log.Error().Err(err).Msgf("Connection config lookup failed for VM '%s', ConnectionName '%s', Spec '%s'", req.Name, nodeGroupReq.ConnectionName, k.SpecId)
		return &VmReqWithCreatedResources{VmReq: &model.CreateNodeGroupReq{Name: req.Name, ConnectionName: nodeGroupReq.ConnectionName}, CreatedResources: createdResources}, detailedErr
	}

	// Base shared resource name pattern: nsId + "-shared-" + connectionName [+ "-" + zone]
	baseResourceName := nsId + model.StrSharedResourceName + nodeGroupReq.ConnectionName
	if req.Zone != "" {
		baseResourceName = baseResourceName + "-" + req.Zone
		log.Info().Msgf("Using zone-specific shared resource name: %s (zone: %s) for VM '%s'", baseResourceName, req.Zone, req.Name)
	}

	// VNet resource name: append templateId suffix when a specific template is requested,
	// so that different templates result in independent VNets within the same connection.
	vNetResourceName := baseResourceName
	if req.VNetTemplateId != "" {
		vNetResourceName = baseResourceName + "-" + req.VNetTemplateId
		log.Info().Msgf("Using template-specific VNet resource name: %s (template: %s) for VM '%s'", vNetResourceName, req.VNetTemplateId, req.Name)
	}

	// SG resource name: append templateId suffix so different NodeGroups on the same
	// connection can independently use different SecurityGroup policies.
	sgResourceName := baseResourceName
	if req.SgTemplateId != "" {
		sgResourceName = baseResourceName + "-" + req.SgTemplateId
		log.Info().Msgf("Using template-specific SG resource name: %s (template: %s) for VM '%s'", sgResourceName, req.SgTemplateId, req.Name)
	}

	// SSHKey shares the base resource name (connection-scoped, no template support)
	resourceName := baseResourceName

	nodeGroupReq.SpecId = specInfo.Id
	nodeGroupReq.ImageId = k.ImageId

	// Check if the image is available (DB or CSP) and auto-register if needed
	imageInfo, isAutoRegistered, err := resource.EnsureImageAvailable(nsId, connection.ConfigName, nodeGroupReq.ImageId)
	if err != nil {
		detailedErr := fmt.Errorf("failed to find image '%s' for VM '%s' in CSP '%s' (connection: %s): %w. Please verify the image exists and is accessible in the target region",
			nodeGroupReq.ImageId, req.Name, connection.ProviderName, connection.ConfigName, err)
		log.Error().Err(err).Msgf("Image lookup failed for VM '%s', ImageId '%s', Provider '%s', Connection '%s'",
			req.Name, nodeGroupReq.ImageId, connection.ProviderName, connection.ConfigName)
		return &VmReqWithCreatedResources{VmReq: &model.CreateNodeGroupReq{Name: req.Name, ConnectionName: nodeGroupReq.ConnectionName, ImageId: nodeGroupReq.ImageId}, CreatedResources: createdResources}, detailedErr
	}
	if isAutoRegistered {
		log.Info().Msgf("Image '%s' was auto-registered from CSP for VM '%s'", nodeGroupReq.ImageId, req.Name)
	}
	// Update ImageId with the registered image ID (handles both regular and custom images)
	nodeGroupReq.ImageId = imageInfo.Id

	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Setting vNet:" + vNetResourceName, Time: time.Now()})

	nodeGroupReq.VNetId = vNetResourceName
	_, err = resource.GetResource(nsId, model.StrVNet, nodeGroupReq.VNetId)
	if err != nil {
		if !onDemand {
			detailedErr := fmt.Errorf("failed to get required VNet '%s' for VM '%s' from connection '%s': %w. VNet must exist when onDemand is disabled",
				nodeGroupReq.VNetId, req.Name, nodeGroupReq.ConnectionName, err)
			log.Error().Err(err).Msgf("VNet lookup failed for VM '%s', VNetId '%s', Connection '%s' (onDemand disabled)",
				req.Name, nodeGroupReq.VNetId, nodeGroupReq.ConnectionName)
			return &VmReqWithCreatedResources{VmReq: &model.CreateNodeGroupReq{Name: req.Name, ConnectionName: nodeGroupReq.ConnectionName, VNetId: nodeGroupReq.VNetId}, CreatedResources: createdResources}, detailedErr
		}
		clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Loading default vNet:" + vNetResourceName, Time: time.Now()})

		// Check if the target vNet (template-specific or base) already exists (e.g. created
		// by a concurrent NodeGroup for the same connection). Using vNetResourceName here
		// ensures we check the exact resource we intend to use, not a legacy ID.
		_, err := resource.GetResource(nsId, model.StrVNet, vNetResourceName)
		log.Debug().Msg("checked if the default vNet does NOT exist")
		// Create a new default vNet if it does not exist
		if err != nil {
			log.Debug().Msg("Not found default vNet: " + err.Error())
			// Pass Zone, CredentialHolder, and template options
			sharedResourceOpts := &resource.SharedResourceOptions{
				CredentialHolder: credentialHolder,
				VNetTemplateId:   req.VNetTemplateId,
			}
			if req.Zone != "" {
				sharedResourceOpts.Zone = req.Zone
				log.Info().Msgf("Creating VNet with explicit zone '%s' for VM '%s'", req.Zone, req.Name)
			}
			err2 := resource.CreateSharedResourceWithOptions(ctx, nsId, model.StrVNet, nodeGroupReq.ConnectionName, sharedResourceOpts)
			if err2 != nil {
				detailedErr := fmt.Errorf("failed to create default VNet for VM '%s' in namespace '%s' using connection '%s': %w. This may be due to CSP quotas, permissions, or network configuration issues",
					req.Name, nsId, nodeGroupReq.ConnectionName, err2)
				log.Error().Err(err2).Msgf("VNet creation failed for VM '%s', VNetId '%s', Namespace '%s', Connection '%s'",
					req.Name, nodeGroupReq.VNetId, nsId, nodeGroupReq.ConnectionName)
				return &VmReqWithCreatedResources{VmReq: &model.CreateNodeGroupReq{Name: req.Name, ConnectionName: nodeGroupReq.ConnectionName, VNetId: nodeGroupReq.VNetId}, CreatedResources: createdResources}, detailedErr
			} else {
				log.Info().Msg("Created new default vNet: " + nodeGroupReq.VNetId)
				// Track the newly created VNet
				createdResources = append(createdResources, CreatedResource{Type: model.StrVNet, Id: nodeGroupReq.VNetId})
			}
		}
		// Wait for the VNet to be ready after creation
		err = waitForVNetReady(ctx, nsId, nodeGroupReq.VNetId)
		if err != nil {
			detailedErr := fmt.Errorf("VNet '%s' is not ready for use after creation: %w", nodeGroupReq.VNetId, err)
			log.Error().Err(err).Msgf("VNet ready check failed for VM '%s', VNetId '%s'", req.Name, nodeGroupReq.VNetId)
			return &VmReqWithCreatedResources{VmReq: &model.CreateNodeGroupReq{Name: req.Name, ConnectionName: nodeGroupReq.ConnectionName, VNetId: nodeGroupReq.VNetId}, CreatedResources: createdResources}, detailedErr
		}
	} else {
		log.Info().Msg("Found and utilize default vNet: " + nodeGroupReq.VNetId)

		// Even if VNet exists, ensure it's ready for use
		vNetInfo, err := resource.GetVNet(nsId, nodeGroupReq.VNetId)
		if err != nil {
			detailedErr := fmt.Errorf("failed to get VNet info for '%s': %w", nodeGroupReq.VNetId, err)
			log.Error().Err(err).Msg(detailedErr.Error())
			return &VmReqWithCreatedResources{VmReq: &model.CreateNodeGroupReq{Name: req.Name, ConnectionName: nodeGroupReq.ConnectionName, VNetId: nodeGroupReq.VNetId}, CreatedResources: createdResources}, detailedErr
		}

		// Check if VNet is ready, if not wait for it
		if vNetInfo.Status != model.NetworkStatusAvailable {
			log.Info().Msgf("VNet '%s' exists but not ready (status: %s), waiting for ready state", nodeGroupReq.VNetId, vNetInfo.Status)
			err = waitForVNetReady(ctx, nsId, nodeGroupReq.VNetId)
			if err != nil {
				detailedErr := fmt.Errorf("existing VNet '%s' is not ready for use: %w", nodeGroupReq.VNetId, err)
				log.Error().Err(err).Msgf("VNet ready check failed for VM '%s', VNetId '%s'", req.Name, nodeGroupReq.VNetId)
				return &VmReqWithCreatedResources{VmReq: &model.CreateNodeGroupReq{Name: req.Name, ConnectionName: nodeGroupReq.ConnectionName, VNetId: nodeGroupReq.VNetId}, CreatedResources: createdResources}, detailedErr
			}
		}
	}

	// Select subnet based on user-specified zone or VNet template.
	// - Zone specified: find a subnet matching that zone via FindSubnetByZone
	// - Template used (no zone): look up VNet to get first subnet's actual ID
	//   (template subnets may have custom names, not matching vNetResourceName)
	// - Default (no zone, no template): subnet has same name as VNet (hard-coded convention)
	if req.Zone != "" {
		subnetId, subnetZone, err := resource.FindSubnetByZone(nsId, nodeGroupReq.VNetId, req.Zone)
		if err != nil {
			log.Warn().Err(err).Msgf("Failed to find subnet by zone '%s', using default subnet", req.Zone)
			nodeGroupReq.SubnetId = vNetResourceName
		} else {
			nodeGroupReq.SubnetId = subnetId
			log.Info().Msgf("Selected subnet '%s' (zone: '%s') for VM '%s' based on requested zone '%s'",
				subnetId, subnetZone, req.Name, req.Zone)
		}
	} else if req.VNetTemplateId != "" {
		// Template-based VNet: subnets have custom names defined in the template.
		// Look up the VNet to find a subnet. When multiple subnets exist (e.g. multiZone),
		// distribute VMs across subnets using the NodeGroup name as a hash key so placement
		// is deterministic but not always concentrated on the first subnet.
		vNetInfo, err := resource.GetVNet(nsId, nodeGroupReq.VNetId)
		if err == nil && len(vNetInfo.SubnetInfoList) > 0 {
			subnetCount := len(vNetInfo.SubnetInfoList)
			subnetIdx := 0
			if subnetCount > 1 {
				// Simple hash over NodeGroup name bytes for deterministic distribution
				var nameHash int
				for _, c := range req.Name {
					nameHash += int(c)
				}
				subnetIdx = nameHash % subnetCount
			}
			selectedSubnet := vNetInfo.SubnetInfoList[subnetIdx]
			nodeGroupReq.SubnetId = selectedSubnet.Id
			log.Info().Msgf("Selected subnet [%d/%d] '%s' (zone: '%s') from template-based VNet '%s' for VM '%s'",
				subnetIdx+1, subnetCount, selectedSubnet.Id, selectedSubnet.Zone, nodeGroupReq.VNetId, req.Name)
		} else {
			log.Warn().Msgf("Could not retrieve subnets from template-based VNet '%s', falling back to VNet name as SubnetId", nodeGroupReq.VNetId)
			nodeGroupReq.SubnetId = vNetResourceName
		}
	} else {
		// Default (hard-coded) path: first subnet is named identically to the VNet
		nodeGroupReq.SubnetId = vNetResourceName
	}

	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Setting SSHKey:" + resourceName, Time: time.Now()})
	nodeGroupReq.SshKeyId = resourceName
	_, err = resource.GetResource(nsId, model.StrSSHKey, nodeGroupReq.SshKeyId)
	if err != nil {
		if !onDemand {
			detailedErr := fmt.Errorf("failed to get required SSHKey '%s' for VM '%s' from connection '%s': %w. SSHKey must exist when onDemand is disabled",
				nodeGroupReq.SshKeyId, req.Name, nodeGroupReq.ConnectionName, err)
			log.Error().Err(err).Msgf("SSHKey lookup failed for VM '%s', SshKeyId '%s', Connection '%s' (onDemand disabled)",
				req.Name, nodeGroupReq.SshKeyId, nodeGroupReq.ConnectionName)
			return &VmReqWithCreatedResources{VmReq: &model.CreateNodeGroupReq{Name: req.Name, ConnectionName: nodeGroupReq.ConnectionName, SshKeyId: nodeGroupReq.SshKeyId}, CreatedResources: createdResources}, detailedErr
		}
		clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Loading default SSHKey:" + resourceName, Time: time.Now()})

		// Check if the default SSHKey exists
		_, err := resource.GetResource(nsId, model.StrSSHKey, nodeGroupReq.ConnectionName)
		log.Debug().Msg("checked if the default SSHKey does NOT exist")
		// Create a new default SSHKey if it does not exist
		if err != nil {
			log.Debug().Msg("Not found default SSHKey: " + err.Error())
			// Pass Zone and CredentialHolder options (SSHKey has no template support)
			sharedResourceOpts := &resource.SharedResourceOptions{CredentialHolder: credentialHolder}
			if req.Zone != "" {
				sharedResourceOpts.Zone = req.Zone
				log.Info().Msgf("Creating SSHKey with explicit zone '%s' for VM '%s'", req.Zone, req.Name)
			}
			err2 := resource.CreateSharedResourceWithOptions(ctx, nsId, model.StrSSHKey, nodeGroupReq.ConnectionName, sharedResourceOpts)
			if err2 != nil {
				detailedErr := fmt.Errorf("failed to create default SSHKey for VM '%s' in namespace '%s' using connection '%s': %w. This may be due to CSP quotas, permissions, or key generation issues",
					req.Name, nsId, nodeGroupReq.ConnectionName, err2)
				log.Error().Err(err2).Msgf("SSHKey creation failed for VM '%s', SshKeyId '%s', Namespace '%s', Connection '%s'",
					req.Name, nodeGroupReq.SshKeyId, nsId, nodeGroupReq.ConnectionName)
				return &VmReqWithCreatedResources{VmReq: &model.CreateNodeGroupReq{Name: req.Name, ConnectionName: nodeGroupReq.ConnectionName, SshKeyId: nodeGroupReq.SshKeyId}, CreatedResources: createdResources}, detailedErr
			} else {
				log.Info().Msg("Created new default SSHKey: " + nodeGroupReq.SshKeyId)
				// Track the newly created SSHKey
				createdResources = append(createdResources, CreatedResource{Type: model.StrSSHKey, Id: nodeGroupReq.SshKeyId})
			}
		}
	} else {
		log.Info().Msg("Found and utilize default SSHKey: " + nodeGroupReq.SshKeyId)
	}

	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Setting securityGroup:" + sgResourceName, Time: time.Now()})
	securityGroup := sgResourceName
	nodeGroupReq.SecurityGroupIds = append(nodeGroupReq.SecurityGroupIds, securityGroup)
	_, err = resource.GetResource(nsId, model.StrSecurityGroup, securityGroup)
	if err != nil {
		if !onDemand {
			detailedErr := fmt.Errorf("failed to get required SecurityGroup '%s' for VM '%s' from connection '%s': %w. SecurityGroup must exist when onDemand is disabled",
				securityGroup, req.Name, nodeGroupReq.ConnectionName, err)
			log.Error().Err(err).Msgf("SecurityGroup lookup failed for VM '%s', SecurityGroup '%s', Connection '%s' (onDemand disabled)",
				req.Name, securityGroup, nodeGroupReq.ConnectionName)
			return &VmReqWithCreatedResources{VmReq: &model.CreateNodeGroupReq{Name: req.Name, ConnectionName: nodeGroupReq.ConnectionName, SecurityGroupIds: []string{securityGroup}}, CreatedResources: createdResources}, detailedErr
		}
		clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Loading default securityGroup:" + sgResourceName, Time: time.Now()})

		// Check if the target SecurityGroup (template-specific or base) already exists.
		// Using sgResourceName ensures we check the exact resource we intend to use.
		_, err := resource.GetResource(nsId, model.StrSecurityGroup, sgResourceName)
		// Create a new default security group if it does not exist
		log.Debug().Msg("checked if the default security group does NOT exist")
		if err != nil {
			log.Debug().Msg("Not found default security group: " + err.Error())
			// Pass Zone, CredentialHolder, and template options
			// VNetTemplateId is needed so the SG's VNetId points to the template-specific VNet name
			sharedResourceOpts := &resource.SharedResourceOptions{
				CredentialHolder: credentialHolder,
				VNetTemplateId:   req.VNetTemplateId,
				SgTemplateId:     req.SgTemplateId,
			}
			if req.Zone != "" {
				sharedResourceOpts.Zone = req.Zone
				log.Info().Msgf("Creating SecurityGroup with explicit zone '%s' for VM '%s'", req.Zone, req.Name)
			}
			err2 := resource.CreateSharedResourceWithOptions(ctx, nsId, model.StrSecurityGroup, nodeGroupReq.ConnectionName, sharedResourceOpts)
			if err2 != nil {
				detailedErr := fmt.Errorf("failed to create default SecurityGroup for VM '%s' in namespace '%s' using connection '%s': %w. This may be due to CSP quotas, permissions, or firewall rule configuration issues",
					req.Name, nsId, nodeGroupReq.ConnectionName, err2)
				log.Error().Err(err2).Msgf("SecurityGroup creation failed for VM '%s', SecurityGroup '%s', Namespace '%s', Connection '%s'",
					req.Name, securityGroup, nsId, nodeGroupReq.ConnectionName)
				return &VmReqWithCreatedResources{VmReq: &model.CreateNodeGroupReq{Name: req.Name, ConnectionName: nodeGroupReq.ConnectionName, SecurityGroupIds: []string{securityGroup}}, CreatedResources: createdResources}, detailedErr
			} else {
				log.Info().Msg("Created new default securityGroup: " + securityGroup)
				// Track the newly created SecurityGroup
				createdResources = append(createdResources, CreatedResource{Type: model.StrSecurityGroup, Id: securityGroup})
			}
		}
	} else {
		log.Info().Msg("Found and utilize default securityGroup: " + securityGroup)
	}

	nodeGroupReq.Name = k.Name
	if nodeGroupReq.Name == "" {
		nodeGroupReq.Name = common.GenUid()
	}
	nodeGroupReq.Label = k.Label
	nodeGroupReq.NodeGroupSize = k.NodeGroupSize
	nodeGroupReq.Description = k.Description
	nodeGroupReq.RootDiskType = k.RootDiskType
	nodeGroupReq.RootDiskSize = k.RootDiskSize
	nodeGroupReq.VmUserPassword = k.VmUserPassword

	common.PrintJsonPretty(nodeGroupReq)
	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Prepared resources for VM:" + nodeGroupReq.Name, Info: nodeGroupReq, Time: time.Now()})

	return &VmReqWithCreatedResources{VmReq: nodeGroupReq, CreatedResources: createdResources}, nil
}

// CreateVmObject is func to add VM to Infra
func CreateVmObject(wg *sync.WaitGroup, nsId string, infraId string, vmInfoData *model.VmInfo) error {
	log.Debug().Msg("Start to add VM To Infra")
	//goroutin
	defer wg.Done()

	key := common.GenInfraKey(nsId, infraId, "")
	_, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Fatal().Err(err).Msg("AddVmToInfra kvstore.GetKv() returned an error.")
		return err
	}
	if !exists {
		return fmt.Errorf("AddVmToInfra Cannot find infraId. Key: %s", key)
	}

	configTmp, err := common.GetConnConfig(vmInfoData.ConnectionName)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	vmInfoData.Location = configTmp.RegionDetail.Location

	// Make VM object
	key = common.GenInfraKey(nsId, infraId, vmInfoData.Id)
	val, _ := json.Marshal(vmInfoData)
	err = kvstore.Put(key, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	return nil
}

// VmCreateInfo represents VM creation information with grouping details
type VmCreateInfo struct {
	VmInfo       *model.VmInfo
	ProviderName string
	RegionName   string
}

// CreateVmsInParallel creates VMs with hierarchical rate limiting
// Level 1: CSPs are processed in parallel
// Level 2: Within each CSP, regions are processed with semaphore (maxConcurrentRegionsPerCSP)
// Level 3: Within each region, VMs are processed with semaphore (maxConcurrentVMsPerRegion)
func CreateVmsInParallel(ctx context.Context, nsId, infraId string, vmInfoList []*model.VmInfo, option string) error {
	if len(vmInfoList) == 0 {
		return nil
	}

	// Step 1: Group VMs by CSP and region
	vmGroups := make(map[string]map[string][]*model.VmInfo) // CSP -> Region -> VmInfos
	vmGroupInfos := make(map[string]VmCreateInfo)           // VmId -> CreateInfo

	for _, vmInfo := range vmInfoList {
		providerName := vmInfo.ConnectionConfig.ProviderName
		regionName := vmInfo.Region.Region

		// Initialize CSP map if not exists
		if vmGroups[providerName] == nil {
			vmGroups[providerName] = make(map[string][]*model.VmInfo)
		}

		// Add VM to the appropriate group
		vmGroups[providerName][regionName] = append(vmGroups[providerName][regionName], vmInfo)
		vmGroupInfos[vmInfo.Id] = VmCreateInfo{
			VmInfo:       vmInfo,
			ProviderName: providerName,
			RegionName:   regionName,
		}
	}

	// Step 2: Process CSPs in parallel
	var wg sync.WaitGroup
	var mutex sync.Mutex
	var allErrors []error

	for csp, regions := range vmGroups {
		wg.Add(1)
		go func(providerName string, regionMap map[string][]*model.VmInfo) {
			defer wg.Done()

			// Get rate limits for this specific CSP
			maxRegionsForCSP, maxVMsForRegion := getVmCreateRateLimitsForCSP(providerName)

			log.Debug().Msgf("Creating VMs for CSP: %s with %d regions (limits: %d regions, %d VMs/region)",
				providerName, len(regionMap), maxRegionsForCSP, maxVMsForRegion)

			// Step 3: Process regions within CSP with rate limiting
			regionSemaphore := make(chan struct{}, maxRegionsForCSP)
			var regionWg sync.WaitGroup
			var regionMutex sync.Mutex
			var cspErrors []error

			for region, vmInfos := range regionMap {
				regionWg.Add(1)
				go func(regionName string, vmInfoList []*model.VmInfo) {
					defer regionWg.Done()

					// Acquire region semaphore
					regionSemaphore <- struct{}{}
					defer func() { <-regionSemaphore }()

					log.Debug().Msgf("Creating VMs in region: %s/%s with %d VMs (limit: %d VMs/region)",
						providerName, regionName, len(vmInfoList), maxVMsForRegion)

					// Step 4: Process VMs within region with rate limiting
					vmSemaphore := make(chan struct{}, maxVMsForRegion)
					var vmWg sync.WaitGroup
					var vmMutex sync.Mutex
					var regionErrors []error

					for _, vmInfo := range vmInfoList {
						vmWg.Add(1)
						go func(vmInfo *model.VmInfo) {
							defer vmWg.Done()

							// Acquire VM semaphore
							vmSemaphore <- struct{}{}
							defer func() { <-vmSemaphore }()

							// Create VM using the existing CreateVm function
							var createWg sync.WaitGroup
							createWg.Add(1)
							err := CreateVm(ctx, &createWg, nsId, infraId, vmInfo, option)
							if err != nil {
								log.Error().Err(err).Msgf("Failed to create VM %s", vmInfo.Name)
								vmMutex.Lock()
								regionErrors = append(regionErrors, fmt.Errorf("VM %s: %w", vmInfo.Name, err))
								vmMutex.Unlock()
							}

						}(vmInfo)
					}
					vmWg.Wait()

					// Merge region errors to CSP errors
					if len(regionErrors) > 0 {
						regionMutex.Lock()
						cspErrors = append(cspErrors, regionErrors...)
						regionMutex.Unlock()
					}

				}(region, vmInfos)
			}
			regionWg.Wait()

			// Merge CSP errors to global errors
			if len(cspErrors) > 0 {
				mutex.Lock()
				allErrors = append(allErrors, cspErrors...)
				mutex.Unlock()
			}

			log.Debug().Msgf("Completed VM creation for CSP: %s", providerName)

		}(csp, regions)
	}

	wg.Wait()

	// Summary logging
	cspCount := len(vmGroups)
	totalRegions := 0
	for _, regions := range vmGroups {
		totalRegions += len(regions)
	}

	if len(allErrors) > 0 {
		log.Warn().Msgf("Rate-limited VM creation completed with errors: %d CSPs, %d regions, %d VMs total, %d errors",
			cspCount, totalRegions, len(vmInfoList), len(allErrors))
		// Don't return error for partial failures - let the caller handle individual VM status checks
		// Return first error for compatibility only if ALL VMs failed
		if len(allErrors) >= len(vmInfoList) {
			return allErrors[0]
		}
		log.Info().Msgf("Partial VM creation success: %d out of %d VMs may have failed, but continuing",
			len(allErrors), len(vmInfoList))
	}

	log.Debug().Msgf("Rate-limited VM creation completed successfully: %d CSPs, %d regions, %d VMs processed",
		cspCount, totalRegions, len(vmInfoList))
	return nil
}

// CreateVm is func to create VM (option = "register" for register existing VM)
func CreateVm(ctx context.Context, wg *sync.WaitGroup, nsId string, infraId string, vmInfoData *model.VmInfo, option string) error {
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
		UpdateVmInfo(nsId, infraId, *vmInfoData)
		log.Error().Err(err).Msg("")
		return err
	}

	vmKey := common.GenInfraKey(nsId, infraId, vmInfoData.Id)

	// in case of registering existing CSP VM
	if option == "register" {
		// CspResourceId is required
		if vmInfoData.CspResourceId == "" {
			err := fmt.Errorf("vmInfoData.CspResourceId is empty (required for register VM)")
			vmInfoData.Status = model.StatusFailed
			vmInfoData.SystemMessage = err.Error()
			UpdateVmInfo(nsId, infraId, *vmInfoData)
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
	// Convert int to string for Spider API
	if vmInfoData.RootDiskSize > 0 {
		requestBody.ReqInfo.RootDiskSize = strconv.Itoa(vmInfoData.RootDiskSize)
	} else {
		requestBody.ReqInfo.RootDiskSize = ""
	}

	if option == "register" {
		requestBody.ReqInfo.CSPid = vmInfoData.CspResourceId

	} else {
		// Try lookup customImage
		imageInfo, err := resource.GetImage(nsId, vmInfoData.ImageId)
		if err != nil {
			log.Debug().Msgf("GetImage returned an error: %s", err.Error())
			return err
		}
		if imageInfo.ResourceType == model.StrCustomImage {
			// If the requested image is a custom image (generated by VM snapshot), RootDiskType should be empty.
			// TB ignore inputs for RootDiskType, RootDiskSize
			customImageFlag = true
			requestBody.ReqInfo.ImageType = model.MyImage
			requestBody.ReqInfo.RootDiskType = ""
			requestBody.ReqInfo.RootDiskSize = ""
			requestBody.ReqInfo.ImageName = imageInfo.CspImageName
			log.Debug().Msgf("CustomImage detected, set ImageName to CspImageId: %s", requestBody.ReqInfo.ImageName)
			log.Debug().Msgf("CustomImage detected, ignore RootDiskType and RootDiskSize")
		} else {
			requestBody.ReqInfo.ImageName = imageInfo.CspImageName
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
				UpdateVmInfo(nsId, infraId, *vmInfoData)
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
			UpdateVmInfo(nsId, infraId, *vmInfoData)
			return err
		}

		requestBody.ReqInfo.SubnetName = subnetInfo.CspResourceName
		if requestBody.ReqInfo.SubnetName == "" {
			vmInfoData.Status = model.StatusFailed
			vmInfoData.SystemMessage = err.Error()
			UpdateVmInfo(nsId, infraId, *vmInfoData)
			log.Error().Err(err).Msg("")
			return err
		}

		var SecurityGroupIdsTmp []string
		for _, v := range vmInfoData.SecurityGroupIds {
			CspResourceId, err := resource.GetCspResourceName(nsId, model.StrSecurityGroup, v)
			if CspResourceId == "" {
				vmInfoData.Status = model.StatusFailed
				vmInfoData.SystemMessage = err.Error()
				UpdateVmInfo(nsId, infraId, *vmInfoData)
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
					UpdateVmInfo(nsId, infraId, *vmInfoData)
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
			UpdateVmInfo(nsId, infraId, *vmInfoData)
			log.Error().Err(err).Msg("")
			return err
		}
	}

	common.RandomSleep(0, 5*1000)
	log.Info().Msg("VM request body to CB-Spider")
	common.PrintJsonPretty(requestBody)

	client := clientManager.NewHttpClient()
	method := "POST"
	client.SetTimeout(20 * time.Minute)

	url := model.SpiderRestUrl + "/vm"
	if option == "register" {
		url = model.SpiderRestUrl + "/regvm"
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
		err = fmt.Errorf("%v", err)
		vmInfoData.Status = model.StatusFailed
		vmInfoData.SystemMessage = err.Error()
		UpdateVmInfo(nsId, infraId, *vmInfoData)
		msg := fmt.Sprintf("Failed to create VM %s request body to Spider: %v", vmInfoData.Name, requestBody)
		log.Error().Err(err).Msg(msg)
		return err
	}

	vmInfoData.AddtionalDetails = callResult.KeyValueList
	vmInfoData.VmUserName = callResult.VMUserId
	vmInfoData.VmUserPassword = callResult.VMUserPasswd
	vmInfoData.CspResourceName = callResult.IId.NameId
	vmInfoData.CspResourceId = callResult.IId.SystemId
	vmInfoData.Region = callResult.Region
	vmInfoData.PublicIP = callResult.PublicIP
	// Convert port string from Spider to int
	if portStr, err := TrimIP(callResult.SSHAccessPoint); err == nil {
		if port, err := strconv.Atoi(portStr); err == nil {
			vmInfoData.SSHPort = port
		}
	}
	vmInfoData.PublicDNS = callResult.PublicDNS
	vmInfoData.PrivateIP = callResult.PrivateIP
	vmInfoData.PrivateDNS = callResult.PrivateDNS
	vmInfoData.RootDiskType = callResult.RootDiskType
	// Convert RootDiskSize string from Spider to int
	if rootDiskSize, err := strconv.Atoi(callResult.RootDiskSize); err == nil {
		vmInfoData.RootDiskSize = rootDiskSize
	}
	vmInfoData.RootDeviceName = callResult.RootDeviceName
	vmInfoData.NetworkInterface = callResult.NetworkInterface

	vmInfoData.CspSpecName = callResult.VMSpecName
	vmInfoData.CspImageName = callResult.ImageIId.SystemId
	vmInfoData.CspVNetId = callResult.VpcIID.SystemId
	vmInfoData.CspSubnetId = callResult.SubnetIID.SystemId
	vmInfoData.CspSshKeyId = callResult.KeyPairIId.SystemId

	if option == "register" {
		// Reconstuct resource IDs
		// Spec
		if callResult.VMSpecName != "" {
			resourceListInNs, err := resource.ListResource(model.SystemCommonNs, model.StrSpec, "csp_spec_name", callResult.VMSpecName)
			if err != nil {
				log.Error().Err(err).Msg("Failed to list Spec")
			} else {
				resourcesInNs := resourceListInNs.([]model.SpecInfo)
				for _, res := range resourcesInNs {
					if res.ConnectionName == requestBody.ConnectionName {
						vmInfoData.SpecId = res.Id
						break
					}
				}
			}
		}

		// Image
		targetImageName := callResult.ImageIId.SystemId
		if targetImageName == "" {
			targetImageName = callResult.ImageIId.NameId
		} else {
			// Try to use EnsureImageAvailable for consistent image handling
			imageInfo, isAutoRegistered, err := resource.EnsureImageAvailable(nsId, requestBody.ConnectionName, targetImageName)

			if err != nil {
				log.Error().Err(err).Msgf("Failed to ensure image availability: %s", targetImageName)
				errMsg := fmt.Sprintf("Dependency Missing: Cannot find or register Image (CSP ID: %s) in TB.", targetImageName)
				log.Error().Msg(errMsg)
			} else {
				vmInfoData.ImageId = imageInfo.Id

				// Determine if this is a custom image
				if imageInfo.ResourceType == model.StrCustomImage {
					customImageFlag = true
				}

				if !isAutoRegistered {
					log.Debug().Msgf("Image found in DB: %s (ID: %s)", targetImageName, imageInfo.Id)
				}
			}
		}

		// vNet
		resourceListInNs, err := resource.ListResource(nsId, model.StrVNet, "cspResourceName", callResult.VpcIID.SystemId)
		if err != nil {
			log.Error().Err(err).Msg("")
		} else {
			resourcesInNs := resourceListInNs.([]model.VNetInfo) // type assertion
			for _, resource := range resourcesInNs {
				if resource.ConnectionName == requestBody.ConnectionName {
					vmInfoData.VNetId = resource.Id

					// subnet
					targetSubnet := callResult.SubnetIID.SystemId

					if targetSubnet == "" {
						targetSubnet = callResult.SubnetIID.NameId
					}

					for _, subnet := range resource.SubnetInfoList {
						if subnet.CspResourceId == targetSubnet {
							vmInfoData.SubnetId = subnet.Id
							break
						}
					}
					break
				}
			}
		}

		// SecurityGroups
		var matchedSgIds []string
		for _, sgIID := range callResult.SecurityGroupIIds {
			resourceListInNs, err := resource.ListResource(nsId, model.StrSecurityGroup, "cspResourceName", sgIID.SystemId)
			if err != nil {
				log.Error().Err(err).Msg("")
			} else {
				resourcesInNs := resourceListInNs.([]model.SecurityGroupInfo)
				for _, resource := range resourcesInNs {
					if resource.ConnectionName == requestBody.ConnectionName {
						matchedSgIds = append(matchedSgIds, resource.Id)
						break
					}
				}
			}
		}
		vmInfoData.SecurityGroupIds = matchedSgIds

		// access Key
		sshKeyMatched := false
		if callResult.KeyPairIId.SystemId != "" {
			resourceListInNs, err = resource.ListResource(nsId, model.StrSSHKey, "cspResourceName", callResult.KeyPairIId.SystemId)
			if err != nil {
				log.Warn().Err(err).Msg("Failed to list SSH keys for matching")
			} else {
				resourcesInNs := resourceListInNs.([]model.SshKeyInfo) // type assertion
				for _, res := range resourcesInNs {
					if res.ConnectionName == requestBody.ConnectionName {
						vmInfoData.SshKeyId = res.Id
						sshKeyMatched = true
						break
					}
				}
			}
		}

		// GCP does not have SSH key as an independent resource object.
		// Create a placeholder SSH key so that VM registration can proceed.
		// The user can later update this SSH key via the ComplementSshKey API.
		if !sshKeyMatched {
			providerName := strings.ToLower(vmInfoData.ConnectionConfig.ProviderName)
			if csp.ResolveCloudPlatform(providerName) == csp.GCP {
				log.Info().Msgf("GCP detected: creating placeholder SSH key for VM '%s' (GCP does not manage SSH keys as independent resources)", vmInfoData.Name)
				placeholderSshKey, placeholderErr := resource.CreatePlaceholderSshKey(ctx, nsId, requestBody.ConnectionName, vmInfoData.Name, vmInfoData.Uid)
				if placeholderErr != nil {
					log.Error().Err(placeholderErr).Msgf("Failed to create placeholder SSH key for GCP VM '%s'", vmInfoData.Name)
				} else {
					vmInfoData.SshKeyId = placeholderSshKey.Id
					log.Info().Msgf("Successfully created placeholder SSH key '%s' for GCP VM '%s'", placeholderSshKey.Id, vmInfoData.Name)
				}
			} else {
				log.Warn().Msgf("No matching SSH key found for VM '%s' (provider: %s, cspKeyPairId: %s)", vmInfoData.Name, providerName, callResult.KeyPairIId.SystemId)
			}
		}

	}

	if customImageFlag == false {
		resource.UpdateAssociatedObjectList(nsId, model.StrImage, vmInfoData.ImageId, model.StrAdd, vmKey)
	} else {
		resource.UpdateAssociatedObjectList(nsId, model.StrCustomImage, vmInfoData.ImageId, model.StrAdd, vmKey)
	}

	//resource.UpdateAssociatedObjectList(nsId, model.StrSpec, vmInfoData.SpecId, model.StrAdd, vmKey)
	if vmInfoData.SshKeyId != "" {
		resource.UpdateAssociatedObjectList(nsId, model.StrSSHKey, vmInfoData.SshKeyId, model.StrAdd, vmKey)
	}
	resource.UpdateAssociatedObjectList(nsId, model.StrVNet, vmInfoData.VNetId, model.StrAdd, vmKey)

	for _, v := range vmInfoData.SecurityGroupIds {
		resource.UpdateAssociatedObjectList(nsId, model.StrSecurityGroup, v, model.StrAdd, vmKey)
	}

	for _, v := range vmInfoData.DataDiskIds {
		resource.UpdateAssociatedObjectList(nsId, model.StrDataDisk, v, model.StrAdd, vmKey)
	}

	// Register dataDisks which are created with the creation of VM
	for _, v := range callResult.DataDiskIIDs {
		tbDataDiskReq := model.DataDiskReq{
			Name:           v.NameId,
			ConnectionName: vmInfoData.ConnectionName,
			CspResourceId:  v.SystemId,
		}

		dataDisk, err := resource.CreateDataDisk(ctx, nsId, &tbDataDiskReq, "register")
		if err != nil {
			err = fmt.Errorf("after starting VM %s, failed to register dataDisk %s. \n", vmInfoData.Name, v.NameId)
			log.Err(err).Msg("")
		}

		vmInfoData.DataDiskIds = append(vmInfoData.DataDiskIds, dataDisk.Id)

		resource.UpdateAssociatedObjectList(nsId, model.StrDataDisk, dataDisk.Id, model.StrAdd, vmKey)
	}

	// Populate SpecSummary and ImageSummary for VmInfo
	if vmInfoData.SpecId != "" {
		specInfo, err := resource.GetSpec(model.SystemCommonNs, vmInfoData.SpecId)
		if err != nil {
			log.Warn().Err(err).Msgf("Failed to get spec info for SpecSummary: %s", vmInfoData.SpecId)
		} else {
			vmInfoData.Spec = model.SpecSummary{
				CspSpecName:         specInfo.CspSpecName,
				VCPU:                specInfo.VCPU,
				MemoryGiB:           specInfo.MemoryGiB,
				AcceleratorModel:    specInfo.AcceleratorModel,
				AcceleratorCount:    specInfo.AcceleratorCount,
				AcceleratorMemoryGB: specInfo.AcceleratorMemoryGB,
				AcceleratorType:     specInfo.AcceleratorType,
				CostPerHour:         specInfo.CostPerHour,
			}
		}
	}

	if vmInfoData.ImageId != "" {
		imageInfo, err := resource.GetImage(nsId, vmInfoData.ImageId)
		if err != nil {
			log.Warn().Err(err).Msgf("Failed to get image info for ImageSummary: %s", vmInfoData.ImageId)
		} else {
			vmInfoData.Image = model.ImageSummary{
				ResourceType:   imageInfo.ResourceType,
				CspImageName:   imageInfo.CspImageName,
				OSType:         imageInfo.OSType,
				OSArchitecture: imageInfo.OSArchitecture,
				OSDistribution: imageInfo.OSDistribution,
			}
		}
	}

	// Assign a Bastion if none (randomly)
	UpdateVmInfo(nsId, infraId, *vmInfoData)
	_, err = SetBastionNodes(nsId, infraId, vmInfoData.Id, "", "", "")
	if err != nil {
		// just log error and continue
		log.Debug().Msg(err.Error())
	}

	// set initial TargetAction, TargetStatus
	vmInfoData.TargetAction = model.ActionComplete
	vmInfoData.TargetStatus = model.StatusComplete

	// get and set current vm status
	vmStatusInfoTmp, err := FetchVmStatus(nsId, infraId, vmInfoData.Id)

	if err != nil {
		err = fmt.Errorf("cannot Fetch Vm Status from CSP: %v", err)
		vmInfoData.Status = model.StatusFailed
		vmInfoData.SystemMessage = err.Error()
		UpdateVmInfo(nsId, infraId, *vmInfoData)

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

	UpdateVmInfo(nsId, infraId, *vmInfoData)

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
		model.LabelNodeGroupId:     vmInfoData.NodeGroupId,
		model.LabelInfraId:         infraId,
		model.LabelCreatedTime:     vmInfoData.CreatedTime,
		model.LabelConnectionName:  vmInfoData.ConnectionName,
		model.LabelVNetId:          vmInfoData.VNetId,
		model.LabelSubnetId:        vmInfoData.SubnetId,
	}
	for key, value := range vmInfoData.Label {
		labels[key] = value
	}
	err = label.CreateOrUpdateLabel(ctx, model.StrVM, vmInfoData.Uid, vmKey, labels)
	if err != nil {
		err = fmt.Errorf("cannot create label object: %v", err)
		vmInfoData.Status = model.StatusFailed
		vmInfoData.SystemMessage = err.Error()
		UpdateVmInfo(nsId, infraId, *vmInfoData)

		log.Error().Err(err).Msg("")
		return err
	}

	return nil
}

func filterCheckInfraDynamicReqInfoToCheckK8sClusterDynamicReqInfo(infraDReqInfo *model.CheckInfraDynamicReqInfo) *model.CheckK8sClusterDynamicReqInfo {
	k8sDReqInfo := model.CheckK8sClusterDynamicReqInfo{}

	if infraDReqInfo != nil {
		for _, k := range infraDReqInfo.ReqCheck {
			// Note: InfraType field is deprecated.
			// K8s minimum requirements (vCPU >= 2, Memory >= 4GB) are validated separately.

			imageListForK8s := []model.ImageInfo{}

			// Priority 1: Filter and prioritize K8s-optimized images
			for _, i := range k.Image {
				if i.IsKubernetesImage {
					imageListForK8s = append(imageListForK8s, i)
				}
			}

			// Priority 2: Fallback to all images if no K8s-optimized images available
			// This handles CSPs like Azure AKS where no dedicated K8s images exist
			if len(imageListForK8s) == 0 {
				log.Debug().Msg("No K8s-optimized images found, using all available images as fallback")
				imageListForK8s = k.Image
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
				nodeDReqInfo.Image = []model.ImageInfo{{Id: "default", Name: "default"}}
			}

			k8sDReqInfo.ReqCheck = append(k8sDReqInfo.ReqCheck, nodeDReqInfo)
		}
	}

	return &k8sDReqInfo
}

// CheckK8sClusterDynamicReq is func to check request info to create K8sCluster obeject and deploy requested Nodes in a dynamic way
func CheckK8sClusterDynamicReq(req *model.K8sClusterConnectionConfigCandidatesReq) (*model.CheckK8sClusterDynamicReqInfo, error) {
	if len(req.SpecIds) != 1 {
		err := fmt.Errorf("Only one SpecId should be defined.")
		log.Error().Err(err).Msg("")
		return &model.CheckK8sClusterDynamicReqInfo{}, err
	}

	infraCCCReq := model.InfraConnectionConfigCandidatesReq{
		SpecIds: req.SpecIds,
	}
	infraDReqInfo, err := CheckInfraDynamicReq(common.NewDefaultContext(), &infraCCCReq)

	k8sDReqInfo := filterCheckInfraDynamicReqInfoToCheckK8sClusterDynamicReqInfo(infraDReqInfo)

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
func checkCommonResAvailableForK8sClusterDynamicReq(dReq *model.K8sClusterDynamicReq) error {
	specInfo, err := resource.GetSpec(model.SystemCommonNs, dReq.SpecId)
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
		err := fmt.Errorf("Failed to get ConnectionName (" + connName + ") for Spec (" + dReq.SpecId + ") is not found.")
		log.Error().Err(err).Msg("")
		return err
	}

	niDesignation, err := common.GetK8sNodeImageDesignation(connConfig.ProviderName)
	if err != nil {
		log.Error().Err(err).Msg("")
	}

	if niDesignation == false {
		// if node image designation is not supported by CSP, auto-correct ImageId to "default"
		if !(strings.EqualFold(dReq.ImageId, "default") || strings.EqualFold(dReq.ImageId, "")) {
			log.Warn().Msgf("NodeImageDesignation is not supported by CSP(%s). ImageId '%s' will be replaced with 'default'", connConfig.ProviderName, dReq.ImageId)
			dReq.ImageId = "default"
		}
	}

	// In K8sCluster, allows dReq.ImageId to be set to "default" or ""
	if strings.EqualFold(dReq.ImageId, "default") ||
		strings.EqualFold(dReq.ImageId, "") {
		// do nothing
	} else {
		// Check if the image is available (DB or CSP) and auto-register if needed
		_, isAutoRegistered, err := resource.EnsureImageAvailable(model.SystemCommonNs, connName, dReq.ImageId)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get the Image from the CSP")
			return err
		}
		if isAutoRegistered {
			log.Info().Msgf("Image '%s' was auto-registered from CSP for K8sCluster", dReq.ImageId)
		}
	}

	return nil
}

// checkCommonResAvailableForK8sNodeGroupDynamicReq is func to check common resources availability for K8sNodeGroupDynamicReq
func checkCommonResAvailableForK8sNodeGroupDynamicReq(connName string, dReq *model.K8sNodeGroupDynamicReq) error {
	k8sClusterDReq := &model.K8sClusterDynamicReq{
		SpecId:         dReq.SpecId,
		ImageId:        dReq.ImageId,
		ConnectionName: connName,
	}

	err := checkCommonResAvailableForK8sClusterDynamicReq(k8sClusterDReq)
	if err != nil {
		return err
	}

	return nil
}

// getK8sClusterReqFromDynamicReq is func to get K8sClusterReq from K8sClusterDynamicReq
func getK8sClusterReqFromDynamicReq(ctx context.Context, nsId string, dReq *model.K8sClusterDynamicReq, skipVersionCheck bool) (*model.K8sClusterReq, error) {
	reqID := common.RequestIDFromContext(ctx)
	onDemand := true

	emptyK8sReq := &model.K8sClusterReq{}
	k8sReq := &model.K8sClusterReq{}
	k8sngReq := &model.K8sNodeGroupReq{}

	specInfo, err := resource.GetSpec(model.SystemCommonNs, dReq.SpecId)
	if err != nil {
		log.Err(err).Msg("")
		return emptyK8sReq, err
	}
	k8sngReq.SpecId = specInfo.Id

	var k8sRecVersion string
	if skipVersionCheck {
		// Use the requested version directly without validation
		k8sRecVersion = dReq.Version
		if k8sRecVersion == "" {
			// If skipVersionCheck is true, an explicit version must be provided
			err := fmt.Errorf("skipVersionCheck is true but no version is specified; an explicit version must be provided")
			log.Err(err).Msg("")
			return emptyK8sReq, err
		}
		log.Warn().Msgf("K8sCluster version validation skipped for version: %s (dynamic)", k8sRecVersion)
	} else {
		// Normal validation path
		k8sRecVersion, err = getK8sRecommendVersion(specInfo.ProviderName, specInfo.RegionName, dReq.Version)
		if err != nil {
			log.Err(err).Msg("")
			return emptyK8sReq, err
		}
	}

	// If ConnectionName is specified by the request, Use ConnectionName from the request
	k8sReq.ConnectionName = specInfo.ConnectionName
	if dReq.ConnectionName != "" {
		k8sReq.ConnectionName = dReq.ConnectionName
	}

	// validate the GetConnConfig for spec
	connection, err := common.GetConnConfig(k8sReq.ConnectionName)
	if err != nil {
		err := fmt.Errorf("Failed to Get ConnectionName (" + k8sReq.ConnectionName + ") for Spec (" + dReq.SpecId + ") is not found.")
		log.Err(err).Msg("")
		return emptyK8sReq, err
	}

	k8sNgOnCreation, err := common.GetK8sNodeGroupsOnK8sCreation(connection.ProviderName)
	if err != nil {
		log.Err(err).Msgf("Failed to Get Nodegroups on K8sCluster Creation")
		return emptyK8sReq, err
	}

	// In K8sCluster, allows dReq.ImageId to be set to "default" or ""
	if strings.EqualFold(dReq.ImageId, "default") ||
		strings.EqualFold(dReq.ImageId, "") {
		// do nothing
	} else {
		// Check if the image is available (DB or CSP) and auto-register if needed
		_, isAutoRegistered, err := resource.EnsureImageAvailable(nsId, k8sReq.ConnectionName, dReq.ImageId)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get the Image from the CSP")
			return emptyK8sReq, err
		}
		if isAutoRegistered {
			log.Info().Msgf("Image '%s' was auto-registered from CSP for K8sCluster", dReq.ImageId)
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

		err2 := resource.CreateSharedResource(ctx, nsId, model.StrVNet, k8sReq.ConnectionName)
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

		err2 := resource.CreateSharedResource(ctx, nsId, model.StrSSHKey, k8sReq.ConnectionName)
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

		err2 := resource.CreateSharedResource(ctx, nsId, model.StrSecurityGroup, k8sReq.ConnectionName)
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
	if k8sngReq.DesiredNodeSize <= 0 {
		k8sngReq.DesiredNodeSize = 1
	}
	k8sngReq.MinNodeSize = dReq.MinNodeSize
	if k8sngReq.MinNodeSize <= 0 {
		k8sngReq.MinNodeSize = 1
	}
	k8sngReq.MaxNodeSize = dReq.MaxNodeSize
	if k8sngReq.MaxNodeSize <= 0 {
		k8sngReq.MaxNodeSize = 2
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
func CreateK8sClusterDynamic(ctx context.Context, nsId string, dReq *model.K8sClusterDynamicReq, deployOption string, skipVersionCheck bool) (*model.K8sClusterInfo, error) {
	reqID := common.RequestIDFromContext(ctx)
	emptyK8sCluster := &model.K8sClusterInfo{}
	err := common.CheckString(nsId)
	if err != nil {
		log.Err(err).Msg("")
		return emptyK8sCluster, err
	}

	// Validate that name is provided for single cluster creation
	if dReq.Name == "" {
		err := fmt.Errorf("cluster name is required")
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
	k8sReq, err := getK8sClusterReqFromDynamicReq(ctx, nsId, dReq, skipVersionCheck)
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

	// skipVersionCheck parameter is passed from function argument
	return resource.CreateK8sCluster(ctx, nsId, k8sReq, option, skipVersionCheck)
}

// getK8sNodeGroupReqFromDynamicReq is func to get K8sNodeGroupReq from K8sNodeGroupDynamicReq
func getK8sNodeGroupReqFromDynamicReq(ctx context.Context, nsId string, k8sClusterInfo *model.K8sClusterInfo, dReq *model.K8sNodeGroupDynamicReq) (*model.K8sNodeGroupReq, error) {
	reqID := common.RequestIDFromContext(ctx)
	emptyK8sNgReq := &model.K8sNodeGroupReq{}
	k8sNgReq := &model.K8sNodeGroupReq{}

	specInfo, err := resource.GetSpec(model.SystemCommonNs, dReq.SpecId)
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

	// In K8sNodeGroup, allows dReq.ImageId to be set to "default" or ""
	if strings.EqualFold(dReq.ImageId, "default") ||
		strings.EqualFold(dReq.ImageId, "") {
		// Use default - Spider will auto-map AMI Type based on VMSpec
		k8sNgReq.ImageId = ""
		log.Debug().Msg("ImageId is empty or default. Spider will auto-map AMI Type based on VMSpec.")
	} else {
		// Check if the image is available (DB or CSP) and auto-register if needed
		_, isAutoRegistered, err := resource.EnsureImageAvailable(nsId, k8sClusterInfo.ConnectionName, dReq.ImageId)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get the Image from the CSP")
			return emptyK8sNgReq, err
		}
		if isAutoRegistered {
			log.Info().Msgf("Image '%s' was auto-registered from CSP for K8sNodeGroup", dReq.ImageId)
		}
		k8sNgReq.ImageId = dReq.ImageId
		log.Debug().Msgf("Using user-specified imageId: %s", dReq.ImageId)
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
	if k8sNgReq.DesiredNodeSize <= 0 {
		k8sNgReq.DesiredNodeSize = 1
	}
	k8sNgReq.MinNodeSize = dReq.MinNodeSize
	if k8sNgReq.MinNodeSize <= 0 {
		k8sNgReq.MinNodeSize = 1
	}
	k8sNgReq.MaxNodeSize = dReq.MaxNodeSize
	if k8sNgReq.MaxNodeSize <= 0 {
		k8sNgReq.MaxNodeSize = 2
	}
	k8sNgReq.Description = dReq.Description
	k8sNgReq.Label = dReq.Label

	common.PrintJsonPretty(k8sNgReq)
	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Prepared resources for K8sNodeGroup:" + k8sNgReq.Name, Info: k8sNgReq, Time: time.Now()})

	return k8sNgReq, nil
}

// CreateK8sNodeGroupDynamic is func to create K8sNodeGroup obeject and deploy requested K8sNodeGroup in a dynamic way
func CreateK8sNodeGroupDynamic(ctx context.Context, nsId string, k8sClusterId string, dReq *model.K8sNodeGroupDynamicReq) (*model.K8sClusterInfo, error) {
	reqID := common.RequestIDFromContext(ctx)
	log.Debug().Msgf("reqID: %s, nsId: %s, k8sClusterId: %s, dReq: %v\n", reqID, nsId, k8sClusterId, dReq)

	emptyK8sCluster := &model.K8sClusterInfo{}

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

	if tbK8sCInfo.Status != model.K8sClusterActive {
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

	k8sNgReq, err := getK8sNodeGroupReqFromDynamicReq(ctx, nsId, tbK8sCInfo, dReq)
	if err != nil {
		log.Err(err).Msg("Failed to get shared resources for dynamic K8sNodeGroup creation")
		return emptyK8sCluster, err
	}

	common.PrintJsonPretty(k8sNgReq)
	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Prepared all resources for provisioning K8sNodeGroup:" + k8sNgReq.Name, Info: k8sNgReq, Time: time.Now()})
	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Start provisioning", Time: time.Now()})

	return resource.AddK8sNodeGroup(nsId, k8sClusterId, k8sNgReq)
}

// Provisioning History Management Functions

// generateProvisioningLogKey generates kvstore key for provisioning log
// It URL-encodes the specId to handle special characters like "+" safely
func generateProvisioningLogKey(specId string) string {
	// URL encode the specId to handle special characters like "+" in "gcp+europe-north1+f1-micro"
	encodedSpecId := url.QueryEscape(specId)
	return fmt.Sprintf("/log/provision/%s", encodedSpecId)
}

// GetProvisioningLog retrieves provisioning log for a specific spec ID
func GetProvisioningLog(specId string) (*model.ProvisioningLog, error) {
	log.Debug().Msgf("Getting provisioning log for spec: %s", specId)

	key := generateProvisioningLogKey(specId)
	keyValue, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to get provisioning log for spec: %s", specId)
		return nil, fmt.Errorf("failed to get provisioning log: %w", err)
	}
	if !exists {
		log.Debug().Msgf("No provisioning log found for spec: %s", specId)
		return nil, nil // No log exists yet
	}
	// Check if the value is empty or invalid
	if keyValue.Value == "" {
		log.Debug().Msgf("Empty value found for provisioning log spec: %s, treating as no log exists", specId)
		return nil, nil
	}

	// Check if the value is valid JSON by trying to parse it
	var rawJson json.RawMessage
	if err := json.Unmarshal([]byte(keyValue.Value), &rawJson); err != nil {
		log.Warn().Err(err).Msgf("Invalid JSON found for provisioning log spec: %s, deleting corrupted entry", specId)
		// Delete the corrupted entry
		if deleteErr := kvstore.Delete(key); deleteErr != nil {
			log.Error().Err(deleteErr).Msgf("Failed to delete corrupted provisioning log for spec: %s", specId)
		}
		return nil, nil // Treat as no log exists
	}

	var provisioningLog model.ProvisioningLog
	err = json.Unmarshal([]byte(keyValue.Value), &provisioningLog)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to unmarshal provisioning log for spec: %s", specId)
		// Delete the corrupted entry as a fallback
		if deleteErr := kvstore.Delete(key); deleteErr != nil {
			log.Error().Err(deleteErr).Msgf("Failed to delete corrupted provisioning log for spec: %s", specId)
		}
		return nil, nil // Treat as no log exists instead of returning error
	}

	log.Debug().Msgf("Successfully retrieved provisioning log for spec: %s (failures: %d, successes: %d)",
		specId, provisioningLog.FailureCount, provisioningLog.SuccessCount)
	return &provisioningLog, nil
}

// SaveProvisioningLog saves or updates provisioning log for a specific spec ID
func SaveProvisioningLog(provisioningLog *model.ProvisioningLog) error {
	log.Debug().Msgf("Saving provisioning log for spec: %s", provisioningLog.SpecId)

	provisioningLog.LastUpdated = time.Now()

	key := generateProvisioningLogKey(provisioningLog.SpecId)
	value, err := json.Marshal(provisioningLog)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to marshal provisioning log for spec: %s", provisioningLog.SpecId)
		return fmt.Errorf("failed to marshal provisioning log: %w", err)
	}

	err = kvstore.Put(key, string(value))
	if err != nil {
		log.Error().Err(err).Msgf("Failed to save provisioning log for spec: %s", provisioningLog.SpecId)
		return fmt.Errorf("failed to save provisioning log: %w", err)
	}

	log.Debug().Msgf("Successfully saved provisioning log for spec: %s", provisioningLog.SpecId)
	return nil
}

// DeleteProvisioningLog deletes provisioning log for a specific spec ID
func DeleteProvisioningLog(specId string) error {
	log.Debug().Msgf("Deleting provisioning log for spec: %s", specId)

	key := generateProvisioningLogKey(specId)
	err := kvstore.Delete(key)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to delete provisioning log for spec: %s", specId)
		return fmt.Errorf("failed to delete provisioning log: %w", err)
	}

	log.Debug().Msgf("Successfully deleted provisioning log for spec: %s", specId)
	return nil
}

// RecordProvisioningEvent records a provisioning event (success or failure) to the log
func RecordProvisioningEvent(event *model.ProvisioningEvent) error {
	log.Debug().Msgf("Recording provisioning event for spec: %s, success: %t", event.SpecId, event.IsSuccess)

	// Get existing log or create new one
	existingLog, err := GetProvisioningLog(event.SpecId)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to get existing provisioning log for spec: %s", event.SpecId)
		return fmt.Errorf("failed to get existing provisioning log: %w", err)
	}

	var provisioningLog *model.ProvisioningLog
	if existingLog == nil {
		// Create new log if it doesn't exist
		log.Debug().Msgf("Creating new provisioning log for spec: %s", event.SpecId)

		// Get spec info to populate connection details
		specInfo, err := resource.GetSpec(model.SystemCommonNs, event.SpecId)
		if err != nil {
			log.Error().Err(err).Msgf("Failed to get spec info for: %s", event.SpecId)
			return fmt.Errorf("failed to get spec info: %w", err)
		}

		provisioningLog = &model.ProvisioningLog{
			SpecId:            event.SpecId,
			ConnectionName:    specInfo.ConnectionName,
			ProviderName:      specInfo.ProviderName,
			RegionName:        specInfo.RegionName,
			FailureCount:      0,
			SuccessCount:      0,
			FailureTimestamps: make([]time.Time, 0),
			SuccessTimestamps: make([]time.Time, 0),
			FailureMessages:   make([]string, 0),
			FailureImages:     make([]string, 0),
			SuccessImages:     make([]string, 0),
			AdditionalInfo:    make(map[string]string),
		}
	} else {
		provisioningLog = existingLog
	}

	// Record the event
	if event.IsSuccess {
		// Only record success if there were previous failures
		if provisioningLog.FailureCount > 0 {
			log.Debug().Msgf("Recording success event for spec: %s (previous failures exist)", event.SpecId)
			provisioningLog.SuccessCount++
			provisioningLog.SuccessTimestamps = append(provisioningLog.SuccessTimestamps, event.Timestamp)
			if event.CspImageName != "" && !contains(provisioningLog.SuccessImages, event.CspImageName) {
				provisioningLog.SuccessImages = append(provisioningLog.SuccessImages, event.CspImageName)
			}
		} else {
			log.Debug().Msgf("Skipping success event recording for spec: %s (no previous failures)", event.SpecId)
			return nil // Don't record success if no previous failures
		}
	} else {
		// Always record failures
		log.Debug().Msgf("Recording failure event for spec: %s", event.SpecId)
		provisioningLog.FailureCount++
		provisioningLog.FailureTimestamps = append(provisioningLog.FailureTimestamps, event.Timestamp)
		if event.ErrorMessage != "" {
			provisioningLog.FailureMessages = append(provisioningLog.FailureMessages, event.ErrorMessage)
		}
		if event.CspImageName != "" && !contains(provisioningLog.FailureImages, event.CspImageName) {
			provisioningLog.FailureImages = append(provisioningLog.FailureImages, event.CspImageName)
		}
	}

	// Add additional context information
	if event.InfraId != "" {
		if provisioningLog.AdditionalInfo == nil {
			provisioningLog.AdditionalInfo = make(map[string]string)
		}
		provisioningLog.AdditionalInfo["lastInfraId"] = event.InfraId
	}

	// Save the updated log
	err = SaveProvisioningLog(provisioningLog)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to save provisioning log for spec: %s", event.SpecId)
		return fmt.Errorf("failed to save provisioning log: %w", err)
	}

	log.Debug().Msgf("Successfully recorded provisioning event for spec: %s (total failures: %d, successes: %d)",
		event.SpecId, provisioningLog.FailureCount, provisioningLog.SuccessCount)
	return nil
}

// RecordProvisioningEventsFromInfra analyzes Infra creation result and records provisioning events
func RecordProvisioningEventsFromInfra(nsId string, infraInfo *model.InfraInfo) error {
	log.Debug().Msgf("Recording provisioning events from Infra: %s", infraInfo.Id)

	if infraInfo.CreationErrors == nil {
		log.Debug().Msgf("No creation errors found in Infra: %s, checking for individual VM failures", infraInfo.Id)
	}

	eventCount := 0

	// Process VMs to record events
	for _, vm := range infraInfo.Vm {
		log.Debug().Msgf("Processing VM: %s, status: %s", vm.Id, vm.Status)

		// Determine if this VM failed or succeeded based on status
		isSuccess := vm.Status == model.StatusRunning
		errorMessage := ""

		if !isSuccess {
			// Look for specific error message in creation errors
			if infraInfo.CreationErrors != nil {
				for _, vmError := range infraInfo.CreationErrors.VmCreationErrors {
					if vmError.VmName == vm.Id || strings.Contains(vmError.VmName, vm.Id) {
						errorMessage = vmError.Error
						break
					}
				}
				// Also check VM object creation errors
				for _, vmError := range infraInfo.CreationErrors.VmObjectCreationErrors {
					if vmError.VmName == vm.Id || strings.Contains(vmError.VmName, vm.Id) {
						errorMessage = vmError.Error
						break
					}
				}
			}
			// Check VM's SystemMessage for additional error details
			if errorMessage == "" && vm.SystemMessage != "" {
				errorMessage = vm.SystemMessage
			}
			// If no specific error message found, provide a clearer message
			if errorMessage == "" {
				errorMessage = fmt.Sprintf("VM provisioning failed (status: %s) - check CSP console for details", vm.Status)
			}
		}

		// Create provisioning event
		event := &model.ProvisioningEvent{
			SpecId:       vm.SpecId,
			CspImageName: vm.CspImageName,
			IsSuccess:    isSuccess,
			ErrorMessage: errorMessage,
			Timestamp:    time.Now(),
			VmName:       vm.Id,
			InfraId:      infraInfo.Id,
		}

		// Record the event
		err := RecordProvisioningEvent(event)
		if err != nil {
			log.Error().Err(err).Msgf("Failed to record provisioning event for VM: %s", vm.Id)
			continue
		}

		eventCount++
		log.Debug().Msgf("Recorded provisioning event for VM: %s, spec: %s, success: %t",
			vm.Id, vm.SpecId, isSuccess)
	}

	log.Debug().Msgf("Successfully recorded %d provisioning events from Infra: %s", eventCount, infraInfo.Id)
	return nil
}

// AnalyzeProvisioningRisk analyzes the risk of provisioning failure based on historical data
func AnalyzeProvisioningRisk(specId string, cspImageName string) (riskLevel string, riskMessage string, err error) {
	log.Debug().Msgf("Analyzing provisioning risk for spec: %s, image: %s", specId, cspImageName)

	// Get detailed risk analysis
	riskAnalysis, err := AnalyzeProvisioningRiskDetailed(specId, cspImageName)
	if err != nil {
		return "low", "Unable to analyze provisioning risk", err
	}

	// Return overall risk for backward compatibility
	return riskAnalysis.OverallRisk.Level, riskAnalysis.OverallRisk.Message, nil
}

// AnalyzeProvisioningRiskDetailed provides comprehensive risk analysis with separate spec and image risk assessment
func AnalyzeProvisioningRiskDetailed(specId string, cspImageName string) (*model.RiskAnalysis, error) {
	log.Debug().Msgf("Analyzing detailed provisioning risk for spec: %s, image: %s", specId, cspImageName)

	// Get provisioning log - now handles corrupted data gracefully
	provisioningLog, err := GetProvisioningLog(specId)
	if err != nil {
		log.Warn().Err(err).Msgf("Failed to get provisioning log for spec: %s, treating as no history", specId)
		// Return default low risk analysis
		return &model.RiskAnalysis{
			SpecRisk: model.SpecRiskInfo{
				Level:   "low",
				Message: "Unable to analyze spec history, assuming low risk",
			},
			ImageRisk: model.ImageRiskInfo{
				Level:            "low",
				Message:          "Unable to analyze image history, assuming low risk",
				IsNewCombination: true,
			},
			OverallRisk: model.OverallRiskInfo{
				Level:             "low",
				Message:           "Unable to analyze provisioning history, assuming low risk",
				PrimaryRiskFactor: "none",
			},
			Recommendations: []string{"Monitor this deployment for any issues"},
		}, nil
	}

	// If no log exists, assume low risk
	if provisioningLog == nil {
		log.Debug().Msgf("No provisioning history found for spec: %s", specId)
		return &model.RiskAnalysis{
			SpecRisk: model.SpecRiskInfo{
				Level:   "low",
				Message: "No previous provisioning history available for this spec",
			},
			ImageRisk: model.ImageRiskInfo{
				Level:            "low",
				Message:          "No previous history for this image with this spec",
				IsNewCombination: true,
			},
			OverallRisk: model.OverallRiskInfo{
				Level:             "low",
				Message:           "No previous provisioning history available",
				PrimaryRiskFactor: "none",
			},
			Recommendations: []string{"This is a new spec, monitor deployment closely"},
		}, nil
	}

	totalAttempts := provisioningLog.FailureCount + provisioningLog.SuccessCount
	if totalAttempts == 0 {
		log.Debug().Msgf("No provisioning attempts recorded for spec: %s", specId)
		return &model.RiskAnalysis{
			SpecRisk: model.SpecRiskInfo{
				Level:   "low",
				Message: "No provisioning attempts recorded for this spec",
			},
			ImageRisk: model.ImageRiskInfo{
				Level:            "low",
				Message:          "No attempts with this image on this spec",
				IsNewCombination: true,
			},
			OverallRisk: model.OverallRiskInfo{
				Level:             "low",
				Message:           "No provisioning attempts recorded",
				PrimaryRiskFactor: "none",
			},
			Recommendations: []string{"First deployment with this configuration, proceed with monitoring"},
		}, nil
	}

	failureRate := float64(provisioningLog.FailureCount) / float64(totalAttempts)

	// Check image-specific history
	imageHasFailed := contains(provisioningLog.FailureImages, cspImageName)
	imageHasSucceeded := contains(provisioningLog.SuccessImages, cspImageName)
	isNewCombination := !imageHasFailed && !imageHasSucceeded

	// Count the number of different images that have failed/succeeded with this spec
	failedImageCount := len(provisioningLog.FailureImages)
	succeededImageCount := len(provisioningLog.SuccessImages)

	log.Debug().Msgf("Provisioning analysis for spec %s: failures=%d, successes=%d, rate=%.2f, image_failed=%t, image_succeeded=%t, failed_images=%d, succeeded_images=%d",
		specId, provisioningLog.FailureCount, provisioningLog.SuccessCount, failureRate, imageHasFailed, imageHasSucceeded, failedImageCount, succeededImageCount)

	// Analyze spec-specific risk
	specRisk := analyzeSpecRisk(specId, failedImageCount, succeededImageCount, provisioningLog.FailureCount, provisioningLog.SuccessCount, failureRate)

	// Analyze image-specific risk
	imageRisk := analyzeImageRisk(specId, cspImageName, imageHasFailed, imageHasSucceeded, isNewCombination)

	// Determine overall risk and primary factor
	overallRisk := determineOverallRisk(specRisk, imageRisk)

	// Generate recommendations
	recommendations := generateRecommendations(specRisk, imageRisk, overallRisk)

	// Get recent unique failure messages for context
	recentFailureMessages := getRecentUniqueFailureMessages(provisioningLog, 5)

	return &model.RiskAnalysis{
		SpecRisk:              specRisk,
		ImageRisk:             imageRisk,
		OverallRisk:           overallRisk,
		Recommendations:       recommendations,
		RecentFailureMessages: recentFailureMessages,
	}, nil
}

// analyzeSpecRisk analyzes risk factors specific to the VM specification
func analyzeSpecRisk(specId string, failedImageCount, succeededImageCount, totalFailures, totalSuccesses int, failureRate float64) model.SpecRiskInfo {
	var level, message string

	if failedImageCount >= 10 {
		// Very likely spec-level issue: 10+ different images failed
		level = "high"
		message = fmt.Sprintf("Spec '%s': %d different images failed (%.0f%% failure rate) - spec itself may be problematic",
			specId, failedImageCount, failureRate*100)
	} else if failedImageCount >= 5 {
		// Likely spec-level issue: 5+ different images failed
		level = "medium"
		message = fmt.Sprintf("Spec '%s': %d different images failed (%.0f%% failure rate) - check spec compatibility",
			specId, failedImageCount, failureRate*100)
	} else if failedImageCount >= 3 && succeededImageCount == 0 {
		// Potential spec-level issue: 3+ different images failed with no successes
		level = "medium"
		message = fmt.Sprintf("Spec '%s': %d images failed, none succeeded (%.0f%% failure rate)",
			specId, failedImageCount, failureRate*100)
	} else if failureRate >= 0.8 {
		level = "high"
		message = fmt.Sprintf("Spec '%s' has %.0f%% failure rate (%d failures out of %d attempts)",
			specId, failureRate*100, totalFailures, totalFailures+totalSuccesses)
	} else if failureRate >= 0.5 {
		level = "medium"
		message = fmt.Sprintf("Spec '%s' has %.0f%% failure rate (%d failures, %d successes)",
			specId, failureRate*100, totalFailures, totalSuccesses)
	} else if failureRate > 0 {
		level = "low"
		message = fmt.Sprintf("Spec '%s' has %.0f%% failure rate (%d failures, %d successes) - mostly stable",
			specId, failureRate*100, totalFailures, totalSuccesses)
	} else {
		level = "low"
		message = fmt.Sprintf("Spec '%s' has 100%% success rate (%d successes, no failures)", specId, totalSuccesses)
	}

	return model.SpecRiskInfo{
		Level:               level,
		Message:             message,
		FailedImageCount:    failedImageCount,
		SucceededImageCount: succeededImageCount,
		TotalFailures:       totalFailures,
		TotalSuccesses:      totalSuccesses,
		FailureRate:         failureRate,
	}
}

// analyzeImageRisk analyzes risk factors specific to the image
func analyzeImageRisk(specId, imageId string, imageHasFailed, imageHasSucceeded, isNewCombination bool) model.ImageRiskInfo {
	var level, message string

	if imageHasFailed {
		// CRITICAL: Any previous failure with this exact spec+image combination means high risk
		if !imageHasSucceeded {
			// This specific image has failed before and never succeeded with this spec
			level = "high"
			message = fmt.Sprintf("Image '%s' has FAILED with spec '%s' before (never succeeded)", imageId, specId)
		} else {
			// This image has both failed and succeeded with this spec - still high risk due to failure history
			level = "high"
			message = fmt.Sprintf("Image '%s' has FAILED with spec '%s' before (sometimes succeeds, but unreliable)", imageId, specId)
		}
	} else if imageHasSucceeded && !imageHasFailed {
		// This image has only succeeded with this spec - safest option
		level = "low"
		message = fmt.Sprintf("Image '%s' has succeeded with spec '%s' before (no failures)", imageId, specId)
	} else if isNewCombination {
		// This is a new combination - unknown risk
		level = "low"
		message = fmt.Sprintf("Image '%s' + Spec '%s' is a new combination (no history)", imageId, specId)
	} else {
		// Fallback case
		level = "low"
		message = "No specific image risk identified"
	}

	return model.ImageRiskInfo{
		Level:                level,
		Message:              message,
		HasFailedWithSpec:    imageHasFailed,
		HasSucceededWithSpec: imageHasSucceeded,
		IsNewCombination:     isNewCombination,
	}
}

// determineOverallRisk determines the overall risk based on spec and image risks
func determineOverallRisk(specRisk model.SpecRiskInfo, imageRisk model.ImageRiskInfo) model.OverallRiskInfo {
	var level, message, primaryRiskFactor string

	// Determine the highest risk level
	specRiskValue := getRiskValue(specRisk.Level)
	imageRiskValue := getRiskValue(imageRisk.Level)

	if specRiskValue >= imageRiskValue {
		level = specRisk.Level
		primaryRiskFactor = "spec"
		if specRiskValue > imageRiskValue {
			message = specRisk.Message
		} else {
			message = fmt.Sprintf("%s | %s", specRisk.Message, imageRisk.Message)
		}
	} else {
		level = imageRisk.Level
		primaryRiskFactor = "image"
		message = imageRisk.Message
	}

	// Special case handling
	if specRisk.Level == "low" && imageRisk.Level == "low" {
		primaryRiskFactor = "none"
		message = imageRisk.Message // Use the detailed image message which includes spec+image info
	} else if imageRisk.IsNewCombination && specRisk.Level != "low" {
		primaryRiskFactor = "combination"
		message = fmt.Sprintf("%s (image untested with this spec)", specRisk.Message)
	}

	return model.OverallRiskInfo{
		Level:             level,
		Message:           message,
		PrimaryRiskFactor: primaryRiskFactor,
	}
}

// generateRecommendations provides actionable guidance based on risk analysis
func generateRecommendations(specRisk model.SpecRiskInfo, imageRisk model.ImageRiskInfo, overallRisk model.OverallRiskInfo) []string {
	var recommendations []string

	switch overallRisk.PrimaryRiskFactor {
	case "spec":
		if specRisk.Level == "high" {
			recommendations = append(recommendations, "Consider changing to a different VM specification")
			recommendations = append(recommendations, "Check if this spec is available and properly configured in the target region")
			if specRisk.FailedImageCount >= 5 {
				recommendations = append(recommendations, "Multiple images have failed with this spec - likely a spec-level compatibility issue")
			}
		} else if specRisk.Level == "medium" {
			recommendations = append(recommendations, "Monitor deployment closely - this spec has shown some issues")
			recommendations = append(recommendations, "Consider having a backup spec ready")
		}

	case "image":
		if imageRisk.Level == "high" {
			if imageRisk.HasFailedWithSpec && !imageRisk.HasSucceededWithSpec {
				recommendations = append(recommendations, "CRITICAL: This exact spec+image combination has failed before and NEVER succeeded")
				recommendations = append(recommendations, "STRONGLY RECOMMEND: Use a different image immediately")
				recommendations = append(recommendations, "Find alternative images with same OS/application requirements")
			} else if imageRisk.HasFailedWithSpec && imageRisk.HasSucceededWithSpec {
				recommendations = append(recommendations, "HIGH RISK: This exact combination has failed at least once before")
				recommendations = append(recommendations, "CAUTION: Even though it succeeded sometimes, failure history indicates instability")
				recommendations = append(recommendations, "Consider using a more reliable image or test extensively before production")
			}
		} else if imageRisk.Level == "medium" {
			recommendations = append(recommendations, "This image has mixed results with this spec - proceed with caution")
		}

	case "combination":
		recommendations = append(recommendations, "This is a new spec+image combination")
		recommendations = append(recommendations, "Monitor closely as there's no historical data for this combination")
		if specRisk.Level != "low" {
			recommendations = append(recommendations, "Consider that this spec has shown issues with other images")
		}

	case "none":
		recommendations = append(recommendations, "Both spec and image appear safe based on historical data")
		recommendations = append(recommendations, "Continue with standard monitoring")

	default:
		recommendations = append(recommendations, "Monitor deployment and record results for future analysis")
	}

	// Add critical warnings for any failure history
	if imageRisk.HasFailedWithSpec {
		recommendations = append(recommendations, "IMPORTANT: This exact spec+image combination has failure history - high caution advised")
	}

	// Add general recommendations based on risk levels
	if overallRisk.Level == "high" {
		recommendations = append(recommendations, "HIGH RISK DEPLOYMENT - Consider testing in development environment first")
		recommendations = append(recommendations, "Ensure robust rollback plans and monitoring are in place")
	} else if overallRisk.Level == "medium" {
		recommendations = append(recommendations, "Medium risk - ensure proper monitoring and rollback plans are in place")
	}

	return recommendations
}

// getRiskValue converts risk level to numeric value for comparison
func getRiskValue(riskLevel string) int {
	switch riskLevel {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
} // CleanupCorruptedProvisioningLogs removes all corrupted provisioning log entries from kvstore
func CleanupCorruptedProvisioningLogs() error {
	log.Debug().Msg("Starting cleanup of corrupted provisioning logs")

	// Get all keys with provisioning log prefix
	keyPattern := "/log/provision/"
	keys, err := kvstore.GetKvList(keyPattern)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list provisioning log keys")
		return fmt.Errorf("failed to list provisioning log keys: %w", err)
	}

	cleanupCount := 0
	for _, key := range keys {
		keyValue, _, err := kvstore.GetKv(key.Key)
		if err != nil {
			log.Warn().Err(err).Msgf("Failed to get value for key: %s", key.Key)
			continue
		}

		// Check if the value is empty or invalid JSON
		if keyValue.Value == "" {
			log.Debug().Msgf("Deleting empty provisioning log: %s", key.Key)
			if deleteErr := kvstore.Delete(key.Key); deleteErr != nil {
				log.Error().Err(deleteErr).Msgf("Failed to delete empty log: %s", key.Key)
			} else {
				cleanupCount++
			}
			continue
		}

		// Test JSON validity
		var testLog model.ProvisioningLog
		if err := json.Unmarshal([]byte(keyValue.Value), &testLog); err != nil {
			log.Debug().Msgf("Deleting corrupted provisioning log: %s", key.Key)
			if deleteErr := kvstore.Delete(key.Key); deleteErr != nil {
				log.Error().Err(deleteErr).Msgf("Failed to delete corrupted log: %s", key.Key)
			} else {
				cleanupCount++
			}
		}
	}

	log.Debug().Msgf("Cleanup completed. Removed %d corrupted provisioning logs", cleanupCount)
	return nil
}

// ValidateProvisioningLogIntegrity checks and repairs provisioning log data integrity
func ValidateProvisioningLogIntegrity(specId string) error {
	log.Debug().Msgf("Validating provisioning log integrity for spec: %s", specId)

	key := generateProvisioningLogKey(specId)
	keyValue, _, err := kvstore.GetKv(key)
	if err != nil {
		if err.Error() == "key not found" {
			log.Debug().Msgf("No provisioning log found for spec: %s", specId)
			return nil // No log exists, nothing to validate
		}
		return fmt.Errorf("failed to get provisioning log: %w", err)
	}

	// Check if the value is empty
	if keyValue.Value == "" {
		log.Warn().Msgf("Empty provisioning log found for spec: %s, deleting", specId)
		return kvstore.Delete(key)
	}

	// Test JSON validity
	var testLog model.ProvisioningLog
	if err := json.Unmarshal([]byte(keyValue.Value), &testLog); err != nil {
		log.Warn().Msgf("Corrupted provisioning log found for spec: %s, deleting", specId)
		return kvstore.Delete(key)
	}

	// Validate data consistency
	totalAttempts := testLog.FailureCount + testLog.SuccessCount
	if totalAttempts != len(testLog.FailureTimestamps)+len(testLog.SuccessTimestamps) {
		log.Warn().Msgf("Inconsistent timestamp count for spec: %s, repairing", specId)

		// Repair by truncating arrays to match counts
		if len(testLog.FailureTimestamps) > testLog.FailureCount {
			testLog.FailureTimestamps = testLog.FailureTimestamps[:testLog.FailureCount]
		}
		if len(testLog.SuccessTimestamps) > testLog.SuccessCount {
			testLog.SuccessTimestamps = testLog.SuccessTimestamps[:testLog.SuccessCount]
		}

		// Save repaired log
		return SaveProvisioningLog(&testLog)
	}

	log.Debug().Msgf("Provisioning log integrity validated for spec: %s", specId)
	return nil
}

// getRecentUniqueFailureMessages extracts recent, unique failure messages from provisioning log
// Returns up to maxMessages most recent unique failure messages
func getRecentUniqueFailureMessages(provisioningLog *model.ProvisioningLog, maxMessages int) []string {
	if provisioningLog == nil || len(provisioningLog.FailureMessages) == 0 {
		return []string{}
	}

	// Use a map to track unique messages and a slice to maintain order
	uniqueMessages := make(map[string]bool)
	var recentMessages []string

	// Process messages from most recent to oldest (assuming they are stored in chronological order)
	// We'll take the last entries as the most recent ones
	messages := provisioningLog.FailureMessages
	startIdx := len(messages) - maxMessages*2 // Look at more messages to find unique ones
	if startIdx < 0 {
		startIdx = 0
	}

	// Process from the end (most recent) backwards
	for i := len(messages) - 1; i >= startIdx && len(recentMessages) < maxMessages; i-- {
		message := strings.TrimSpace(messages[i])
		if message != "" && !uniqueMessages[message] {
			uniqueMessages[message] = true
			recentMessages = append(recentMessages, message)
		}
	}

	// Reverse the slice to have most recent first
	for i, j := 0, len(recentMessages)-1; i < j; i, j = i+1, j-1 {
		recentMessages[i], recentMessages[j] = recentMessages[j], recentMessages[i]
	}

	return recentMessages
}

// CreateK8sMultiClusterDynamic creates multiple K8sClusters in parallel
func CreateK8sMultiClusterDynamic(ctx context.Context, nsId string, multiReq *model.K8sMultiClusterDynamicReq, deployOption string, skipVersionCheck bool) (*model.K8sMultiClusterInfo, error) {
	reqID := common.RequestIDFromContext(ctx)
	if len(multiReq.Clusters) == 0 {
		return nil, fmt.Errorf("no clusters specified in the request")
	}

	// Validate: Either namePrefix is provided OR all clusters have names
	if multiReq.NamePrefix == "" {
		for i, cluster := range multiReq.Clusters {
			if cluster.Name == "" {
				return nil, fmt.Errorf("cluster[%d] must have a name when namePrefix is not provided", i)
			}
		}
	}

	log.Info().Msgf("Creating %d K8sClusters in parallel", len(multiReq.Clusters))

	// Create channels for results
	type clusterResult struct {
		index          int
		name           string // Store actual cluster name for error reporting
		connectionName string // Connection name for error reporting
		specId         string // Spec ID for error reporting
		cluster        *model.K8sClusterInfo
		err            error
	}
	resultChan := make(chan clusterResult, len(multiReq.Clusters))

	// Capture namePrefix to avoid data race in goroutines
	namePrefix := multiReq.NamePrefix

	// Launch goroutines for parallel creation
	for i, clusterReq := range multiReq.Clusters {
		go func(index int, req model.K8sClusterDynamicReq) {
			// Generate unique request ID for each cluster
			clusterCtx := common.WithRequestID(ctx, fmt.Sprintf("%s-cluster-%d", reqID, index))

			// Auto-generate cluster name and inject clustergroup label if NamePrefix is provided
			if namePrefix != "" {
				// Auto-generate name if not provided
				if req.Name == "" {
					// Extract CSP name from specId (format: "provider+region+spec")
					cspName := "unknown"
					if req.SpecId != "" {
						parts := strings.Split(req.SpecId, "+")
						if len(parts) > 0 {
							cspName = parts[0]
						}
					}
					req.Name = fmt.Sprintf("%s-%s-%d", namePrefix, cspName, index+1)
					log.Debug().Msgf("Auto-generated cluster name: %s", req.Name)
				}

				// Inject clustergroup label for grouping clusters created together
				if req.Label == nil {
					req.Label = make(map[string]string)
				}
				req.Label["clustergroup"] = namePrefix
				log.Debug().Msgf("Injected clustergroup label: %s for cluster: %s", namePrefix, req.Name)
			}

			log.Info().Msgf("[%d/%d] Starting K8sCluster creation: %s", index+1, len(multiReq.Clusters), req.Name)

			cluster, err := CreateK8sClusterDynamic(clusterCtx, nsId, &req, deployOption, skipVersionCheck)

			if err != nil {
				log.Error().Err(err).Msgf("[%d/%d] Failed to create K8sCluster: %s", index+1, len(multiReq.Clusters), req.Name)
			} else {
				log.Info().Msgf("[%d/%d] Successfully created K8sCluster: %s", index+1, len(multiReq.Clusters), req.Name)
			}

			resultChan <- clusterResult{
				index:          index,
				name:           req.Name, // Store actual name for error reporting
				connectionName: req.ConnectionName,
				specId:         req.SpecId,
				cluster:        cluster,
				err:            err,
			}
		}(i, clusterReq)
	}

	// Collect results
	results := make([]*model.K8sClusterInfo, len(multiReq.Clusters))
	var errors []string
	var failedClusters []model.K8sClusterFailedInfo

	for i := 0; i < len(multiReq.Clusters); i++ {
		result := <-resultChan
		results[result.index] = result.cluster
		if result.err != nil {
			// Use actual cluster name (which may be auto-generated)
			errors = append(errors, fmt.Sprintf("Cluster[%d] %s: %v", result.index, result.name, result.err))
			// Add to failed clusters list for detailed error reporting
			failedClusters = append(failedClusters, model.K8sClusterFailedInfo{
				Name:           result.name,
				ConnectionName: result.connectionName,
				SpecId:         result.specId,
				Error:          result.err.Error(),
			})
		}
	}

	// Prepare response
	multiInfo := &model.K8sMultiClusterInfo{
		Clusters:       make([]model.K8sClusterInfo, 0, len(results)),
		FailedClusters: failedClusters,
	}

	for _, cluster := range results {
		// Skip nil or empty clusters (empty cluster has no Id/Name)
		if cluster != nil && cluster.Id != "" {
			multiInfo.Clusters = append(multiInfo.Clusters, *cluster)
		}
	}

	// Return error if any cluster failed
	if len(errors) > 0 {
		log.Warn().Msgf("Some clusters failed to create: %v", errors)
		return multiInfo, fmt.Errorf("failed to create %d cluster(s): %s", len(errors), strings.Join(errors, "; "))
	}

	log.Info().Msgf("Successfully created all %d K8sClusters", len(multiInfo.Clusters))
	return multiInfo, nil
}
