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

		nodeList, err := ListNodeId(nsId, infraId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return "", err
		}

		if len(nodeList) == 0 {
			return "No Node to terminate in the Infra", nil
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

	} else if action == "refine" { // refine delete Nodes in model.StatusFailed or model.StatusUndefined

		nodeList, err := ListNodeId(nsId, infraId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return "", err
		}

		if len(nodeList) == 0 {
			return "No Node in the Infra", nil
		}

		infraStatus, err := GetInfraStatus(nsId, infraId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return "", err
		}

		var deletedCount int
		var remainingNodeIds []string

		for _, v := range infraStatus.Node {
			// Remove Nodes in model.StatusFailed or model.StatusUndefined
			log.Debug().Msgf("[nodeInfo.Status] %v", v.Status)
			if strings.EqualFold(v.Status, model.StatusFailed) || strings.EqualFold(v.Status, model.StatusUndefined) {
				// Delete Node sequentially for safety (for performance, need to use goroutine)
				err := DelInfraNode(nsId, infraId, v.Id, "force")
				if err != nil {
					log.Error().Err(err).Msg("")
					return "", err
				}
				deletedCount++
			} else {
				remainingNodeIds = append(remainingNodeIds, v.Id)
			}
		}

		// Update Infra object to reflect the current Node list after refine
		if deletedCount > 0 {
			infraTmp, _, err := GetInfraObject(nsId, infraId)
			if err != nil {
				log.Error().Err(err).Msg("")
				return "", err
			}

			// Rebuild Node list with only remaining Nodes
			var remainingNodes []model.NodeInfo
			for _, nodeId := range remainingNodeIds {
				nodeInfo, err := GetNodeObject(nsId, infraId, nodeId)
				if err != nil {
					log.Warn().Err(err).Msgf("Failed to get VM info for %s during refine update", nodeId)
					continue
				}
				remainingNodes = append(remainingNodes, nodeInfo)
			}

			infraTmp.Node = remainingNodes
			UpdateInfraInfo(nsId, infraTmp)

			log.Info().Msgf("Refine completed: deleted %d Nodes, %d Nodes remaining", deletedCount, len(remainingNodeIds))
		}

		return "Refined the Infra", nil

	} else {
		return "", fmt.Errorf(action + " not supported")
	}
}

// HandleInfraNodeAction is func to Get InfraNode Action
func HandleInfraNodeAction(nsId string, infraId string, nodeId string, action string, force bool) (string, error) {

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

	err = common.CheckString(nodeId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}
	check, _ := CheckNode(nsId, infraId, nodeId)

	if !check {
		err := fmt.Errorf("The vm " + nodeId + " does not exist.")
		return err.Error(), err
	}

	log.Info().Msg("[Node control request] " + action)

	infra, err := GetInfraStatus(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}

	// Check if Infra is under an action (individual Node action cannot be executed while Infra is under an action)
	if infra.TargetAction != "" && infra.TargetAction != model.ActionComplete {
		err = fmt.Errorf("Infra %s is under %s, please try later", infraId, infra.TargetAction)
		if !force {
			log.Info().Msg(err.Error())
			return "", err
		}
	}

	err = CheckAllowedTransition(nsId, infraId, model.OptionalParameter{Set: true, Value: nodeId}, action)
	if err != nil {
		if !force {
			log.Info().Msg(err.Error())
			return "", err
		}
	}

	// If Node is already terminated, treat terminate as a completed no-op
	if strings.EqualFold(action, model.ActionTerminate) {
		nodeStatus, statusErr := GetInfraNodeStatus(nsId, infraId, nodeId, false)
		if statusErr == nil && strings.EqualFold(nodeStatus.Status, model.StatusTerminated) {
			log.Info().Msgf("[VM %s] already terminated, skipping", nodeId)
			return "Already terminated", nil
		}
	}

	var wg sync.WaitGroup
	results := make(chan model.ControlNodeResult, 1)
	wg.Add(1)
	if strings.EqualFold(action, model.ActionSuspend) {
		go ControlNodeAsync(&wg, nsId, infraId, nodeId, model.ActionSuspend, results)
	} else if strings.EqualFold(action, model.ActionResume) {
		go ControlNodeAsync(&wg, nsId, infraId, nodeId, model.ActionResume, results)
	} else if strings.EqualFold(action, model.ActionReboot) {
		go ControlNodeAsync(&wg, nsId, infraId, nodeId, model.ActionReboot, results)
	} else if strings.EqualFold(action, model.ActionTerminate) {
		go ControlNodeAsync(&wg, nsId, infraId, nodeId, model.ActionTerminate, results)
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

	nodeList, err := ListNodeId(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	if len(nodeList) == 0 {
		return errors.New("Node list is empty")
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

	// Apply CSP-aware rate limiting for Node control operations
	err = ControlNodesInParallel(nsId, infraId, nodeList, action, force)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to control Nodes in parallel for action %s", action)
		return err
	}

	// Update Infra TargetAction to Complete after all Node operations are done
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

// NodeControlInfo represents Node control information with grouping details
type NodeControlInfo struct {
	NodeId         string
	ProviderName string
	RegionName   string
}

// ControlNodesInParallel controls VMs with hierarchical rate limiting
// Level 1: CSPs are processed in parallel
// Level 2: Within each CSP, regions are processed with semaphore (maxConcurrentRegionsPerCSP)
// Level 3: Within each region, VMs are processed with semaphore (maxConcurrentNodesPerRegion)
func ControlNodesInParallel(nsId, infraId string, nodeList []string, action string, force bool) error {
	if len(nodeList) == 0 {
		return nil
	}

	// Step 1: Group VMs by CSP and region
	nodeGroups := make(map[string]map[string][]string) // CSP -> Region -> NodeIds
	nodeGroupInfos := make(map[string]NodeControlInfo)   // NodeId -> ControlInfo

	for _, nodeId := range nodeList {
		// Skip if control is not needed
		err := CheckAllowedTransition(nsId, infraId, model.OptionalParameter{Set: true, Value: nodeId}, action)
		if err != nil && !force {
			log.Debug().Msgf("Skipping VM %s for action %s: %v", nodeId, action, err)
			continue
		}

		nodeInfo, err := GetNodeObject(nsId, infraId, nodeId)
		if err != nil {
			log.Warn().Err(err).Msgf("Failed to get VM %s info, skipping", nodeId)
			continue
		}

		providerName := nodeInfo.ConnectionConfig.ProviderName
		regionName := nodeInfo.Region.Region

		// Initialize CSP map if not exists
		if nodeGroups[providerName] == nil {
			nodeGroups[providerName] = make(map[string][]string)
		}

		// Add VM to the appropriate group
		nodeGroups[providerName][regionName] = append(nodeGroups[providerName][regionName], nodeId)
		nodeGroupInfos[nodeId] = NodeControlInfo{
			NodeId:       nodeId,
			ProviderName: providerName,
			RegionName:   regionName,
		}
	}

	// Step 2: Process CSPs in parallel
	var wg sync.WaitGroup
	var mutex sync.Mutex
	var allErrors []error
	var successCount int
	totalNodeCount := len(nodeList)

	for csp, regions := range nodeGroups {
		wg.Add(1)
		go func(providerName string, regionMap map[string][]string) {
			defer wg.Done()

			// Get rate limits for this specific CSP (use same limits as VM creation)
			maxRegionsForCSP, maxNodesForRegion := getNodeCreateRateLimitsForCSP(providerName)

			// log.Debug().Msgf("Controlling VMs for CSP: %s with %d regions (limits: %d regions, %d VMs/region)",
			// 	providerName, len(regionMap), maxRegionsForCSP, maxNodesForRegion)

			// Step 3: Process regions within CSP with rate limiting
			regionSemaphore := make(chan struct{}, maxRegionsForCSP)
			var regionWg sync.WaitGroup
			var regionMutex sync.Mutex
			var cspErrors []error
			var cspSuccessCount int

			for region, nodeIds := range regionMap {
				regionWg.Add(1)
				go func(regionName string, nodeIdList []string) {
					defer regionWg.Done()

					// Acquire region semaphore
					regionSemaphore <- struct{}{}
					defer func() { <-regionSemaphore }()

					// log.Debug().Msgf("Controlling VMs in region: %s/%s with %d VMs (limit: %d VMs/region)",
					// 	providerName, regionName, len(nodeIdList), maxNodesForRegion)

					// Step 4: Process VMs within region with rate limiting
					nodeSemaphore := make(chan struct{}, maxNodesForRegion)
					var nodeWg sync.WaitGroup
					var nodeMutex sync.Mutex
					var regionErrors []error
					var regionSuccessCount int

					for _, nodeId := range nodeIdList {
						nodeWg.Add(1)
						go func(nodeId string) {
							defer nodeWg.Done()

							// Acquire VM semaphore
							nodeSemaphore <- struct{}{}
							defer func() { <-nodeSemaphore }()

							// Control VM using the existing ControlNodeAsync function
							var controlWg sync.WaitGroup
							results := make(chan model.ControlNodeResult, 1)
							controlWg.Add(1)

							// Add delay to avoid overwhelming CSP APIs
							common.RandomSleep(0, 1000)

							go ControlNodeAsync(&controlWg, nsId, infraId, nodeId, action, results)

							result := <-results
							close(results)

							if result.Error != nil {
								log.Error().Err(result.Error).Msgf("Failed to control VM %s", nodeId)
								nodeMutex.Lock()
								regionErrors = append(regionErrors, fmt.Errorf("VM %s: %w", nodeId, result.Error))
								nodeMutex.Unlock()
							} else {
								nodeMutex.Lock()
								regionSuccessCount++
								nodeMutex.Unlock()
							}

						}(nodeId)
					}
					nodeWg.Wait()

					// Merge region results to CSP results
					regionMutex.Lock()
					cspErrors = append(cspErrors, regionErrors...)
					cspSuccessCount += regionSuccessCount
					regionMutex.Unlock()

					// log.Debug().Msgf("Completed VM control in region %s/%s: %d/%d VMs successful",
					// 	providerName, regionName, regionSuccessCount, len(nodeIdList))

				}(region, nodeIds)
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
	cspCount := len(nodeGroups)
	totalRegions := 0
	for _, regions := range nodeGroups {
		totalRegions += len(regions)
	}

	if len(allErrors) > 0 {
		log.Warn().Msgf("Rate-limited VM control completed with some errors: %d CSPs, %d regions, %d/%d VMs successful, %d errors",
			cspCount, totalRegions, successCount, totalNodeCount, len(allErrors))
		// Don't return error for partial failures, just log them
	}
	// else: Rate-limited VM control completed successfully

	return nil
}

// ControlNodeAsync is func to control VM async
func ControlNodeAsync(wg *sync.WaitGroup, nsId string, infraId string, nodeId string, action string, results chan<- model.ControlNodeResult) {
	defer wg.Done() //goroutine sync done

	var err error

	callResult := model.ControlNodeResult{}
	callResult.NodeId = nodeId
	callResult.Status = ""

	// Use GetNodeObject to get VM information
	temp, err := GetNodeObject(nsId, infraId, nodeId)
	if err != nil {
		callResult.Error = fmt.Errorf("GetNodeObject() Err in ControlNodeAsync: %v", err)
		log.Error().Err(callResult.Error).Msg("Error in ControlNodeAsync")
		results <- callResult
		return
	}

	// Generate key for resource updates
	key := common.GenInfraKey(nsId, infraId, nodeId)

	// If Node is already terminated, return early without UpdateNodeInfo
	if temp.Status == model.StatusTerminated {
		// log.Debug().Msgf("[ControlNodeAsync] VM [%s] is already terminated, skipping action [%s]", nodeId, action)
		callResult.Status = temp.Status
		results <- callResult
		return
	}

	cspResourceName := temp.CspResourceName
	//common.PrintJsonPretty(temp.AddtionalDetails)

	// Prevent malformed cspResourceName
	if cspResourceName == "" || common.CheckString(cspResourceName) != nil {
		callResult.Error = fmt.Errorf("Not valid requested CSPNativeNodeId: [" + cspResourceName + "]")
		// temp.Status = model.StatusFailed
		temp.SystemMessage = callResult.Error.Error()
		UpdateNodeInfo(nsId, infraId, temp)
		results <- callResult
		return
	}

	currentStatusBeforeUpdating := temp.Status

	// Log control request initiation
	log.Debug().Msgf("[ControlNode] VM %s: Control request received - Action: %s, CurrentStatus: %s",
		nodeId, action, currentStatusBeforeUpdating)

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
		CancelActiveCommandsForNode(nodeId)

		// Remove Bastion Info from all vNets if the terminating VM is a Bastion
		_, err := RemoveBastionNodes(nsId, infraId, "", "", nodeId)
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
		log.Debug().Msgf("[ControlNode] VM %s: Already in target status [%s], skipping", nodeId, temp.TargetStatus)
		callResult.Status = temp.Status
		results <- callResult
		return
	}

	// Log status transition
	log.Info().Msgf("[ControlNode] VM %s: Status transition - %s -> %s (Target: %s)",
		nodeId, currentStatusBeforeUpdating, temp.Status, temp.TargetStatus)

	UpdateNodeInfo(nsId, infraId, temp)

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

	// log.Debug().Msgf("[ControlNodeAsync] VM %s: CB-Spider control API response - Status: %s, Error: %v",
	// 	nodeId, callResult.Status, err)

	if err != nil {
		log.Error().Err(err).Msg("")
		callResult.Error = err
		results <- callResult
		return
	}

	// common.PrintJsonPretty(callResult)

	// Fetch actual VM status from CSP after successful control operation
	// This ensures we have the most accurate status in our database
	nodeStatusInfo, err := FetchNodeStatus(nsId, infraId, nodeId)
	if err != nil {
		log.Warn().Err(err).Msgf("Failed to fetch VM status after %s operation for VM %s", action, nodeId)
	} else {
		log.Debug().Msgf("[ControlNode] VM %s: After %s - Status: %s, NativeStatus: %s",
			nodeId, action, nodeStatusInfo.Status, nodeStatusInfo.NativeStatus)
	}

	if action != model.ActionTerminate {
		//When VM is restarted, temporal PublicIP will be changed. Need update.
		UpdateNodePublicIp(nsId, infraId, temp)
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
func CheckAllowedTransition(nsId string, infraId string, nodeId model.OptionalParameter, action string) error {

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

	if nodeId.Set {
		vm, err := GetInfraNodeStatus(nsId, infraId, nodeId.Value, false)
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
