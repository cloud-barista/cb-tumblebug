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
	"errors"

	"fmt"

	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/rs/zerolog/log"
)

// Infra Control

// HandleInfraAction is func to handle actions to Infra
func HandleInfraAction(nsId string, infraId string, action string, force bool) (string, error) {
	action = common.ToLower(action)

	// err := common.CheckString(nsId)
	// if err != nil {
	// 	log.Error().Err(err).Msg("")
	// 	return "", err
	// }

	// err = common.CheckString(infraId)
	// if err != nil {
	// 	log.Error().Err(err).Msg("")
	// 	return "", err
	// }
	check, _ := CheckInfra(nsId, infraId)

	if !check {
		err := fmt.Errorf("The infra " + infraId + " does not exist.")
		return err.Error(), err
	}

	log.Info().Msgf("Action requested for Infra %s: Action=%s", infraId, action)

	if action == "suspend" {

		err := ControlInfraAsync(nsId, infraId, model.ActionSuspend, force)
		if err != nil {
			return "", err
		}

		return "Suspending the Infra", nil

	} else if action == "resume" {

		err := ControlInfraAsync(nsId, infraId, model.ActionResume, force)
		if err != nil {
			return "", err
		}

		return "Resuming the Infra", nil

	} else if action == "reboot" {

		err := ControlInfraAsync(nsId, infraId, model.ActionReboot, force)
		if err != nil {
			return "", err
		}

		return "Rebooting the Infra", nil

	} else if action == "terminate" {

		vmList, err := ListVmId(nsId, infraId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return "", err
		}

		if len(vmList) == 0 {
			return "No VM to terminate in the Infra", nil
		}

		err = ControlInfraAsync(nsId, infraId, model.ActionTerminate, force)
		if err != nil {
			return "", err
		}

		return "Terminated the Infra", nil

	} else if action == "continue" {
		key := common.GenInfraKey(nsId, infraId, "")
		holdingInfraMap.Store(key, action)

		return "Continue the holding Infra", nil

	} else if action == "withdraw" {
		key := common.GenInfraKey(nsId, infraId, "")
		holdingInfraMap.Store(key, action)

		return "Withdraw the holding Infra", nil

	} else if action == "refine" { // refine delete VMs in model.StatusFailed or model.StatusUndefined

		vmList, err := ListVmId(nsId, infraId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return "", err
		}

		if len(vmList) == 0 {
			return "No VM in the Infra", nil
		}

		infraStatus, err := GetInfraStatus(nsId, infraId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return "", err
		}

		var deletedCount int
		var remainingVmIds []string

		for _, v := range infraStatus.Vm {
			// Remove VMs in model.StatusFailed or model.StatusUndefined
			log.Debug().Msgf("[vmInfo.Status] %v", v.Status)
			if strings.EqualFold(v.Status, model.StatusFailed) || strings.EqualFold(v.Status, model.StatusUndefined) {
				// Delete VM sequentially for safety (for performance, need to use goroutine)
				err := DelInfraVm(nsId, infraId, v.Id, "force")
				if err != nil {
					log.Error().Err(err).Msg("")
					return "", err
				}
				deletedCount++
			} else {
				remainingVmIds = append(remainingVmIds, v.Id)
			}
		}

		// Update Infra object to reflect the current VM list after refine
		if deletedCount > 0 {
			infraTmp, _, err := GetInfraObject(nsId, infraId)
			if err != nil {
				log.Error().Err(err).Msg("")
				return "", err
			}

			// Rebuild VM list with only remaining VMs
			var remainingVms []model.VmInfo
			for _, vmId := range remainingVmIds {
				vmInfo, err := GetVmObject(nsId, infraId, vmId)
				if err != nil {
					log.Warn().Err(err).Msgf("Failed to get VM info for %s during refine update", vmId)
					continue
				}
				remainingVms = append(remainingVms, vmInfo)
			}

			infraTmp.Vm = remainingVms
			UpdateInfraInfo(nsId, infraTmp)

			log.Info().Msgf("Refine completed: deleted %d VMs, %d VMs remaining", deletedCount, len(remainingVmIds))
		}

		return "Refined the Infra", nil

	} else {
		return "", fmt.Errorf(action + " not supported")
	}
}

// HandleInfraVmAction is func to Get InfraVm Action
func HandleInfraVmAction(nsId string, infraId string, vmId string, action string, force bool) (string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}

	err = common.CheckString(infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}

	err = common.CheckString(vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}
	check, _ := CheckVm(nsId, infraId, vmId)

	if !check {
		err := fmt.Errorf("The vm " + vmId + " does not exist.")
		return err.Error(), err
	}

	log.Info().Msg("[VM control request] " + action)

	infra, err := GetInfraStatus(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}

	// Check if Infra is under an action (individual VM action cannot be executed while Infra is under an action)
	if infra.TargetAction != "" && infra.TargetAction != model.ActionComplete {
		err = fmt.Errorf("Infra %s is under %s, please try later", infraId, infra.TargetAction)
		if !force {
			log.Info().Msg(err.Error())
			return "", err
		}
	}

	err = CheckAllowedTransition(nsId, infraId, model.OptionalParameter{Set: true, Value: vmId}, action)
	if err != nil {
		if !force {
			log.Info().Msg(err.Error())
			return "", err
		}
	}

	// If VM is already terminated, treat terminate as a completed no-op
	if strings.EqualFold(action, model.ActionTerminate) {
		vmStatus, statusErr := GetInfraVmStatus(nsId, infraId, vmId, false)
		if statusErr == nil && strings.EqualFold(vmStatus.Status, model.StatusTerminated) {
			log.Info().Msgf("[VM %s] already terminated, skipping", vmId)
			return "Already terminated", nil
		}
	}

	var wg sync.WaitGroup
	results := make(chan model.ControlVmResult, 1)
	wg.Add(1)
	if strings.EqualFold(action, model.ActionSuspend) {
		go ControlVmAsync(&wg, nsId, infraId, vmId, model.ActionSuspend, results)
	} else if strings.EqualFold(action, model.ActionResume) {
		go ControlVmAsync(&wg, nsId, infraId, vmId, model.ActionResume, results)
	} else if strings.EqualFold(action, model.ActionReboot) {
		go ControlVmAsync(&wg, nsId, infraId, vmId, model.ActionReboot, results)
	} else if strings.EqualFold(action, model.ActionTerminate) {
		go ControlVmAsync(&wg, nsId, infraId, vmId, model.ActionTerminate, results)
	} else {
		close(results)
		wg.Done()
		return "", fmt.Errorf("not supported action: " + action)
	}
	checkErr := <-results
	if checkErr.Error != nil {
		return checkErr.Error.Error(), checkErr.Error
	}
	close(results)
	return "Working on " + action, nil
}

// ControlInfraAsync is func to control Infra async
func ControlInfraAsync(nsId string, infraId string, action string, force bool) error {

	infra, _, err := GetInfraObject(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	// Check if Infra is under an action (new action cannot be executed while Infra is under an action)
	if infra.TargetAction != "" && infra.TargetAction != model.ActionComplete {
		err = fmt.Errorf("Infra %s is under %s, please try later", infraId, infra.TargetAction)
		if !force {
			log.Info().Msg(err.Error())
			return err
		}
	}

	err = CheckAllowedTransition(nsId, infraId, model.OptionalParameter{Set: false}, action)
	if err != nil {
		if !force {
			return err
		}
	}

	vmList, err := ListVmId(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	if len(vmList) == 0 {
		return errors.New("VM list is empty")
	}

	switch action {
	case model.ActionTerminate:

		infra.TargetAction = model.ActionTerminate
		infra.TargetStatus = model.StatusTerminated
		infra.Status = model.StatusTerminating

	case model.ActionReboot:

		infra.TargetAction = model.ActionReboot
		infra.TargetStatus = model.StatusRunning
		infra.Status = model.StatusRebooting

	case model.ActionSuspend:

		infra.TargetAction = model.ActionSuspend
		infra.TargetStatus = model.StatusSuspended
		infra.Status = model.StatusSuspending

	case model.ActionResume:

		infra.TargetAction = model.ActionResume
		infra.TargetStatus = model.StatusRunning
		infra.Status = model.StatusResuming

	default:
		return errors.New(action + " is invalid actionType")
	}
	UpdateInfraInfo(nsId, infra)

	// Apply CSP-aware rate limiting for VM control operations
	err = ControlVmsInParallel(nsId, infraId, vmList, action, force)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to control VMs in parallel for action %s", action)
		return err
	}

	// Update Infra TargetAction to Complete after all VM operations are done
	// This ensures proper completion handling for large Infras
	infra, _, err = GetInfraObject(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	// Mark as complete regardless of individual VM failures
	// Similar to Create action, some VMs may fail but the action itself is complete
	infra.TargetAction = model.ActionComplete
	infra.TargetStatus = model.StatusComplete
	UpdateInfraInfo(nsId, infra)

	log.Info().Msgf("Infra %s action %s completed successfully", infraId, action)
	return nil
}

// VmControlInfo represents VM control information with grouping details
type VmControlInfo struct {
	VmId         string
	ProviderName string
	RegionName   string
}

// ControlVmsInParallel controls VMs with hierarchical rate limiting
// Level 1: CSPs are processed in parallel
// Level 2: Within each CSP, regions are processed with semaphore (maxConcurrentRegionsPerCSP)
// Level 3: Within each region, VMs are processed with semaphore (maxConcurrentVMsPerRegion)
func ControlVmsInParallel(nsId, infraId string, vmList []string, action string, force bool) error {
	if len(vmList) == 0 {
		return nil
	}

	// Step 1: Group VMs by CSP and region
	vmGroups := make(map[string]map[string][]string) // CSP -> Region -> VmIds
	vmGroupInfos := make(map[string]VmControlInfo)   // VmId -> ControlInfo

	for _, vmId := range vmList {
		// Skip if control is not needed
		err := CheckAllowedTransition(nsId, infraId, model.OptionalParameter{Set: true, Value: vmId}, action)
		if err != nil && !force {
			log.Debug().Msgf("Skipping VM %s for action %s: %v", vmId, action, err)
			continue
		}

		vmInfo, err := GetVmObject(nsId, infraId, vmId)
		if err != nil {
			log.Warn().Err(err).Msgf("Failed to get VM %s info, skipping", vmId)
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
		vmGroupInfos[vmId] = VmControlInfo{
			VmId:         vmId,
			ProviderName: providerName,
			RegionName:   regionName,
		}
	}

	// Step 2: Process CSPs in parallel
	var wg sync.WaitGroup
	var mutex sync.Mutex
	var allErrors []error
	var successCount int
	totalVmCount := len(vmList)

	for csp, regions := range vmGroups {
		wg.Add(1)
		go func(providerName string, regionMap map[string][]string) {
			defer wg.Done()

			// Get rate limits for this specific CSP (use same limits as VM creation)
			maxRegionsForCSP, maxVMsForRegion := getVmCreateRateLimitsForCSP(providerName)

			// log.Debug().Msgf("Controlling VMs for CSP: %s with %d regions (limits: %d regions, %d VMs/region)",
			// 	providerName, len(regionMap), maxRegionsForCSP, maxVMsForRegion)

			// Step 3: Process regions within CSP with rate limiting
			regionSemaphore := make(chan struct{}, maxRegionsForCSP)
			var regionWg sync.WaitGroup
			var regionMutex sync.Mutex
			var cspErrors []error
			var cspSuccessCount int

			for region, vmIds := range regionMap {
				regionWg.Add(1)
				go func(regionName string, vmIdList []string) {
					defer regionWg.Done()

					// Acquire region semaphore
					regionSemaphore <- struct{}{}
					defer func() { <-regionSemaphore }()

					// log.Debug().Msgf("Controlling VMs in region: %s/%s with %d VMs (limit: %d VMs/region)",
					// 	providerName, regionName, len(vmIdList), maxVMsForRegion)

					// Step 4: Process VMs within region with rate limiting
					vmSemaphore := make(chan struct{}, maxVMsForRegion)
					var vmWg sync.WaitGroup
					var vmMutex sync.Mutex
					var regionErrors []error
					var regionSuccessCount int

					for _, vmId := range vmIdList {
						vmWg.Add(1)
						go func(vmId string) {
							defer vmWg.Done()

							// Acquire VM semaphore
							vmSemaphore <- struct{}{}
							defer func() { <-vmSemaphore }()

							// Control VM using the existing ControlVmAsync function
							var controlWg sync.WaitGroup
							results := make(chan model.ControlVmResult, 1)
							controlWg.Add(1)

							// Add delay to avoid overwhelming CSP APIs
							common.RandomSleep(0, 1000)

							go ControlVmAsync(&controlWg, nsId, infraId, vmId, action, results)

							result := <-results
							close(results)

							if result.Error != nil {
								log.Error().Err(result.Error).Msgf("Failed to control VM %s", vmId)
								vmMutex.Lock()
								regionErrors = append(regionErrors, fmt.Errorf("VM %s: %w", vmId, result.Error))
								vmMutex.Unlock()
							} else {
								vmMutex.Lock()
								regionSuccessCount++
								vmMutex.Unlock()
							}

						}(vmId)
					}
					vmWg.Wait()

					// Merge region results to CSP results
					regionMutex.Lock()
					cspErrors = append(cspErrors, regionErrors...)
					cspSuccessCount += regionSuccessCount
					regionMutex.Unlock()

					// log.Debug().Msgf("Completed VM control in region %s/%s: %d/%d VMs successful",
					// 	providerName, regionName, regionSuccessCount, len(vmIdList))

				}(region, vmIds)
			}
			regionWg.Wait()

			// Merge CSP results to global results
			mutex.Lock()
			allErrors = append(allErrors, cspErrors...)
			successCount += cspSuccessCount
			mutex.Unlock()

			// log.Debug().Msgf("Completed VM control for CSP: %s, %d VMs successful", providerName, cspSuccessCount)

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
		log.Warn().Msgf("Rate-limited VM control completed with some errors: %d CSPs, %d regions, %d/%d VMs successful, %d errors",
			cspCount, totalRegions, successCount, totalVmCount, len(allErrors))
		// Don't return error for partial failures, just log them
	}
	// else: Rate-limited VM control completed successfully

	return nil
}

// ControlVmAsync is func to control VM async
func ControlVmAsync(wg *sync.WaitGroup, nsId string, infraId string, vmId string, action string, results chan<- model.ControlVmResult) {
	defer wg.Done() //goroutine sync done

	var err error

	callResult := model.ControlVmResult{}
	callResult.VmId = vmId
	callResult.Status = ""

	// Use GetVmObject to get VM information
	temp, err := GetVmObject(nsId, infraId, vmId)
	if err != nil {
		callResult.Error = fmt.Errorf("GetVmObject() Err in ControlVmAsync: %v", err)
		log.Error().Err(callResult.Error).Msg("Error in ControlVmAsync")
		results <- callResult
		return
	}

	// Generate key for resource updates
	key := common.GenInfraKey(nsId, infraId, vmId)

	// If VM is already terminated, return early without UpdateVmInfo
	if temp.Status == model.StatusTerminated {
		// log.Debug().Msgf("[ControlVmAsync] VM [%s] is already terminated, skipping action [%s]", vmId, action)
		callResult.Status = temp.Status
		results <- callResult
		return
	}

	cspResourceName := temp.CspResourceName
	//common.PrintJsonPretty(temp.AddtionalDetails)

	// Prevent malformed cspResourceName
	if cspResourceName == "" || common.CheckString(cspResourceName) != nil {
		callResult.Error = fmt.Errorf("Not valid requested CSPNativeVmId: [" + cspResourceName + "]")
		// temp.Status = model.StatusFailed
		temp.SystemMessage = callResult.Error.Error()
		UpdateVmInfo(nsId, infraId, temp)
		results <- callResult
		return
	}

	currentStatusBeforeUpdating := temp.Status

	// Log control request initiation
	log.Debug().Msgf("[ControlVm] VM %s: Control request received - Action: %s, CurrentStatus: %s",
		vmId, action, currentStatusBeforeUpdating)

	url := ""
	method := ""
	// timeout is set per-action below; terminate needs extra time for bare-metal instances
	timeout := 20 * time.Minute
	switch action {
	case model.ActionTerminate:

		temp.TargetAction = model.ActionTerminate
		temp.TargetStatus = model.StatusTerminated
		temp.Status = model.StatusTerminating

		url = model.SpiderRestUrl + "/vm/" + cspResourceName
		method = "DELETE"
		// Bare-metal instances (e.g. AWS m5.metal) can take significantly longer to terminate
		timeout = 40 * time.Minute

		// Cancel any active SSH commands for this VM to prevent hanging sessions
		CancelActiveCommandsForVm(vmId)

		// Remove Bastion Info from all vNets if the terminating VM is a Bastion
		_, err := RemoveBastionNodes(nsId, infraId, "", "", vmId)
		if err != nil {
			log.Info().Msg(err.Error())
		}

	case model.ActionReboot:

		temp.TargetAction = model.ActionReboot
		temp.TargetStatus = model.StatusRunning
		temp.Status = model.StatusRebooting

		url = model.SpiderRestUrl + "/controlvm/" + cspResourceName + "?action=reboot"
		method = "GET"
	case model.ActionSuspend:

		temp.TargetAction = model.ActionSuspend
		temp.TargetStatus = model.StatusSuspended
		temp.Status = model.StatusSuspending

		url = model.SpiderRestUrl + "/controlvm/" + cspResourceName + "?action=suspend"
		method = "GET"
	case model.ActionResume:

		temp.TargetAction = model.ActionResume
		temp.TargetStatus = model.StatusRunning
		temp.Status = model.StatusResuming

		url = model.SpiderRestUrl + "/controlvm/" + cspResourceName + "?action=resume"
		method = "GET"
	default:
		callResult.Error = fmt.Errorf("%s is invalid actionType", action)
		results <- callResult
		return
	}

	// Check current VM status before making CB-Spider API call
	// If VM is already in target status, skip the operation
	// Exception: Reboot action should always be executed even if current status equals target status (Running -> Running)
	if currentStatusBeforeUpdating == temp.TargetStatus && action != model.ActionReboot {
		log.Debug().Msgf("[ControlVm] VM %s: Already in target status [%s], skipping", vmId, temp.TargetStatus)
		callResult.Status = temp.Status
		results <- callResult
		return
	}

	// Log status transition
	log.Info().Msgf("[ControlVm] VM %s: Status transition - %s -> %s (Target: %s)",
		vmId, currentStatusBeforeUpdating, temp.Status, temp.TargetStatus)

	UpdateVmInfo(nsId, infraId, temp)

	client := clientManager.NewHttpClient()
	// NCP requires a slightly longer timeout due to its control plane characteristics
	if csp.ResolveCloudPlatform(temp.ConnectionConfig.ProviderName) == csp.NCP {
		client.SetTimeout(timeout + 10*time.Minute)
	} else {
		client.SetTimeout(timeout)
	}

	requestBody := model.SpiderConnectionName{}
	requestBody.ConnectionName = temp.ConnectionName

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

	// log.Debug().Msgf("[ControlVmAsync] VM %s: CB-Spider control API response - Status: %s, Error: %v",
	// 	vmId, callResult.Status, err)

	if err != nil {
		log.Error().Err(err).Msg("")
		callResult.Error = err
		results <- callResult
		return
	}

	// common.PrintJsonPretty(callResult)

	// Fetch actual VM status from CSP after successful control operation
	// This ensures we have the most accurate status in our database
	vmStatusInfo, err := FetchVmStatus(nsId, infraId, vmId)
	if err != nil {
		log.Warn().Err(err).Msgf("Failed to fetch VM status after %s operation for VM %s", action, vmId)
	} else {
		log.Debug().Msgf("[ControlVm] VM %s: After %s - Status: %s, NativeStatus: %s",
			vmId, action, vmStatusInfo.Status, vmStatusInfo.NativeStatus)
	}

	if action != model.ActionTerminate {
		//When VM is restarted, temporal PublicIP will be changed. Need update.
		UpdateVmPublicIp(nsId, infraId, temp)
	} else { // if action == model.ActionTerminate
		_, err = resource.UpdateAssociatedObjectList(nsId, model.StrImage, temp.ImageId, model.StrDelete, key)
		if err != nil {
			resource.UpdateAssociatedObjectList(nsId, model.StrCustomImage, temp.ImageId, model.StrDelete, key)
		}

		//resource.UpdateAssociatedObjectList(nsId, model.StrSpec, temp.SpecId, model.StrDelete, key)
		resource.UpdateAssociatedObjectList(nsId, model.StrSSHKey, temp.SshKeyId, model.StrDelete, key)
		resource.UpdateAssociatedObjectList(nsId, model.StrVNet, temp.VNetId, model.StrDelete, key)

		for _, v := range temp.SecurityGroupIds {
			resource.UpdateAssociatedObjectList(nsId, model.StrSecurityGroup, v, model.StrDelete, key)
		}

		for _, v := range temp.DataDiskIds {
			resource.UpdateAssociatedObjectList(nsId, model.StrDataDisk, v, model.StrDelete, key)
		}
	}

	results <- callResult
}

// CheckAllowedTransition is func to check status transition is acceptable
func CheckAllowedTransition(nsId string, infraId string, vmId model.OptionalParameter, action string) error {

	targetStatus := ""
	switch {
	case strings.EqualFold(action, model.ActionTerminate):
		targetStatus = model.StatusTerminated
	case strings.EqualFold(action, model.ActionReboot):
		targetStatus = model.StatusRunning
	case strings.EqualFold(action, model.ActionSuspend):
		targetStatus = model.StatusSuspended
	case strings.EqualFold(action, model.ActionResume):
		targetStatus = model.StatusRunning
	default:
		return fmt.Errorf("requested action %s is not matched with available actions", action)
	}

	if vmId.Set {
		vm, err := GetInfraVmStatus(nsId, infraId, vmId.Value, false)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}

		// duplicated action
		if strings.EqualFold(vm.Status, targetStatus) {
			if strings.EqualFold(action, model.ActionTerminate) {
				// Terminate is idempotent: already terminated is considered success
				return nil
			}
			if !strings.EqualFold(action, model.ActionReboot) {
				return errors.New(action + " is not allowed for VM under " + vm.Status)
			}
		}
		// redundant action
		if strings.EqualFold(vm.Status, model.StatusTerminated) {
			if strings.EqualFold(action, model.ActionTerminate) {
				return nil
			}
			return errors.New(action + " is not allowed for VM under " + vm.Status)
		}
		// under transitional status
		if strings.EqualFold(vm.Status, model.StatusCreating) ||
			strings.EqualFold(vm.Status, model.StatusTerminating) ||
			strings.EqualFold(vm.Status, model.StatusResuming) ||
			strings.EqualFold(vm.Status, model.StatusSuspending) ||
			strings.EqualFold(vm.Status, model.StatusRebooting) {

			return errors.New(action + " is not allowed for VM under " + vm.Status)
		}
		// under conditional status
		if strings.EqualFold(vm.Status, model.StatusSuspended) {
			if strings.EqualFold(action, model.ActionResume) || strings.EqualFold(action, model.ActionTerminate) {
				return nil
			} else {
				return errors.New(action + " is not allowed for VM under " + vm.Status)
			}
		}
	} else {
		infra, err := GetInfraStatus(nsId, infraId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}

		// duplicated action
		if strings.EqualFold(infra.Status, targetStatus) {
			if strings.EqualFold(action, model.ActionTerminate) {
				return nil
			}
			return errors.New(action + " is not allowed for Infra under " + infra.Status)
		}
		// redundant action
		if strings.EqualFold(infra.Status, model.StatusTerminated) {
			if strings.EqualFold(action, model.ActionTerminate) {
				return nil
			}
			return errors.New(action + " is not allowed for Infra under " + infra.Status)
		}
		// under transitional status
		if strings.Contains(infra.Status, model.StatusCreating) ||
			strings.Contains(infra.Status, model.StatusTerminating) ||
			strings.Contains(infra.Status, model.StatusResuming) ||
			strings.Contains(infra.Status, model.StatusSuspending) ||
			strings.Contains(infra.Status, model.StatusRebooting) {

			return errors.New(action + " is not allowed for Infra under " + infra.Status)
		}
		// under conditional status
		if strings.EqualFold(infra.Status, model.StatusSuspended) {
			if strings.EqualFold(action, model.ActionResume) || strings.EqualFold(action, model.ActionTerminate) {
				return nil
			} else {
				return errors.New(action + " is not allowed for Infra under " + infra.Status)
			}
		}
	}
	return nil
}
