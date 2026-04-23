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
		// continue resumes a Provisioning that was held with option=hold.
		// It only signals an in-memory holding goroutine; if no goroutine is
		// waiting (e.g. server restart), the caller should use `reconcile`
		// instead. This action is purely an "intent gate" for hold mode.
		key := common.GenInfraKey(nsId, infraId, "")
		if _, holding := holdingInfraMap.Load(key); holding {
			holdingInfraMap.Store(key, action)
			log.Info().Msgf("Continue: signalled holding Infra %s/%s", nsId, infraId)
			return "Continue the holding Infra", nil
		}
		err := fmt.Errorf("no holding goroutine for Infra %s; if the Infra is stuck after a server restart, use action=reconcile (forward) or action=abort (backward)", infraId)
		log.Warn().Msg(err.Error())
		return "", err

	} else if action == "withdraw" {
		// withdraw cancels a Provisioning that was held with option=hold.
		// Like continue, it only signals an in-memory holding goroutine.
		// For crash-recovery teardown, use `abort` instead.
		key := common.GenInfraKey(nsId, infraId, "")
		if _, holding := holdingInfraMap.Load(key); holding {
			holdingInfraMap.Store(key, action)
			log.Info().Msgf("Withdraw: signalled holding Infra %s/%s", nsId, infraId)
			return "Withdraw the holding Infra", nil
		}
		err := fmt.Errorf("no holding goroutine for Infra %s; to tear down a stuck Infra after a server restart, use action=abort", infraId)
		log.Warn().Msg(err.Error())
		return "", err

	} else if action == "reconcile" {
		// reconcile drives the Infra forward toward its desired Running state
		// by querying Spider for each transient Node and absorbing CSP-side
		// orphan VMs (created before the server crashed but never recorded
		// in TB). Nodes that cannot be reconciled are marked Failed so a
		// subsequent `refine` can remove them. Used to recover Infras stuck
		// after a server restart. No new Spider create calls are issued.
		log.Info().Msgf("Reconcile: forward-reconciling Infra %s/%s", nsId, infraId)
		return reconcileInfraForward(nsId, infraId)

	} else if action == "abort" {
		// abort drives the Infra backward toward Terminated by force-
		// terminating every non-final Node in parallel (with orphan rescue
		// for Nodes missing cspResourceName) and sweeping any Failed remnants
		// via `refine`. Used to give up on a stuck Infra after a server
		// restart or a partial provisioning failure. The final DELETE call
		// is left to the operator.
		log.Info().Msgf("Abort: backward-reconciling Infra %s/%s", nsId, infraId)
		return reconcileInfraBackward(nsId, infraId)

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
			// Reset stale aggregates so that the next GetInfraStatus call
			// recomputes the proportion ("R:x/y") from scratch instead of
			// being clamped by the previous CountTotal (monotonic-up logic
			// in GetInfraStatus would otherwise keep the larger pre-refine
			// total even though Nodes were removed).
			infraTmp.StatusCount = model.StatusCountInfo{}
			infraTmp.Status = ""
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

// transientNodeStatus reports whether status represents an in-flight provisioning
// operation that should be reconciled against the CSP. These are the states a
// crashed/restarted server typically leaves behind.
func transientNodeStatus(status string) bool {
	return strings.EqualFold(status, model.StatusCreating) ||
		strings.EqualFold(status, model.StatusTerminating) ||
		strings.EqualFold(status, model.StatusSuspending) ||
		strings.EqualFold(status, model.StatusResuming) ||
		strings.EqualFold(status, model.StatusRebooting) ||
		strings.EqualFold(status, model.StatusUndefined) ||
		status == ""
}

// settleInfraTargetAction clears Infra-level TargetAction/TargetStatus once
// every Node has reached a final (non-transient) status. Called at the tail of
// reconcileInfraForward / reconcileInfraBackward so that subsequent control
// actions (refine, terminate, delete) are no longer blocked by lingering
// Create/Terminate intent.
func settleInfraTargetAction(nsId, infraId string) {
	infraTmp, _, err := GetInfraObject(nsId, infraId)
	if err != nil {
		log.Warn().Err(err).Msgf("settleInfraTargetAction: cannot load Infra %s/%s", nsId, infraId)
		return
	}
	allSettled := true
	for _, n := range infraTmp.Node {
		if transientNodeStatus(n.Status) {
			allSettled = false
			break
		}
	}
	if allSettled {
		infraTmp.TargetAction = model.ActionComplete
		infraTmp.TargetStatus = model.StatusComplete
		UpdateInfraInfo(nsId, infraTmp)
		log.Info().Msgf("settleInfraTargetAction: Infra %s/%s targetAction cleared (all Nodes settled)", nsId, infraId)
	}
}

// reconcileInfraForward implements the `reconcile` action (forward crash
// recovery). For each Node currently in a transient state it:
//
//  1. Calls FetchNodeStatus, which queries Spider when a cspResourceName is
//     known and persists the corrected status to KV automatically.
//  2. For Nodes still transient AND lacking a cspResourceName (the typical
//     "server died before Spider returned VM IID" pattern), tries to rescue
//     the orphan by querying Spider /allvm for the Node's connection and
//     matching IID.NameId == Node.Uid. Matched orphans are absorbed via
//     Spider /regvm so the Node becomes manageable again, then
//     FetchNodeStatus runs once more to commit the real CSP status.
//  3. Nodes still transient and not rescuable (no CSP record matches the
//     Node.Uid) are marked Failed so refine can clean them up.
//
// After all Nodes are processed, the Infra-level TargetAction is settled when
// possible. The caller can then run `refine` to remove Failed Nodes.
func reconcileInfraForward(nsId, infraId string) (string, error) {
	infraStatus, err := GetInfraStatus(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}
	if infraStatus == nil {
		return "", fmt.Errorf("Infra %s/%s not found", nsId, infraId)
	}

	// Preflight: handle stuck-in-preparation Infra.
	// Provisioning sets Status="Preparing" before creating shared resources
	// (vNet/SG/SSHKey/...) and "Prepared" right before dispatching Nodes. If
	// the server crashes in either window, the Infra has 0 (or very few)
	// Nodes and the original CreateInfraDynamic request cannot be replayed
	// safely from KV (it isn't stored verbatim, and shared resources may be
	// half-provisioned). Reconcile cannot meaningfully resume here, so we
	// surface the situation explicitly and let the operator choose
	// abort → re-run create.
	if infraTmp, _, ierr := GetInfraObject(nsId, infraId); ierr == nil {
		stuckInPrep := strings.Contains(infraTmp.Status, model.StatusPreparing) ||
			strings.Contains(infraTmp.Status, model.StatusPrepared)
		if stuckInPrep && len(infraStatus.Node) == 0 {
			origStatus := infraTmp.Status
			infraTmp.Status = model.StatusFailed
			infraTmp.StatusCount = model.StatusCountInfo{}
			infraTmp.TargetAction = model.ActionComplete
			infraTmp.TargetStatus = model.StatusComplete
			infraTmp.SystemMessage = append(infraTmp.SystemMessage, fmt.Sprintf(
				"Infra was stuck in %s with no Nodes (server likely crashed during resource preparation). "+
					"Reconcile cannot resume provisioning safely. Run abort to clean up, then re-create.",
				origStatus))
			UpdateInfraInfo(nsId, infraTmp)
			msg := fmt.Sprintf("Reconcile Infra %s: was %s with 0 Nodes; marked Failed (cannot auto-resume preparation phase). Run abort, then re-create.",
				infraId, origStatus)
			log.Info().Msg(msg)
			return msg, nil
		}
		if stuckInPrep && len(infraStatus.Node) > 0 {
			// Some Nodes exist (provisioning crashed shortly after Status moved
			// past Preparing/Prepared). Clear the stale top-level Status so
			// GetInfraStatus recomputes it fresh from per-Node states; the
			// regular per-Node reconcile loop below handles the rest.
			infraTmp.Status = ""
			infraTmp.StatusCount = model.StatusCountInfo{}
			UpdateInfraInfo(nsId, infraTmp)
		}
	}

	var (
		reconciledRunning int
		rescued           int
		markedFailed      int
		untouched         int
	)

	// Pass 1: ask Spider for truth on Nodes that already have cspResourceName.
	// Collect orphan candidates (transient + no cspResourceName) for Pass 2.
	var orphanCands []orphanCandidate
	for _, n := range infraStatus.Node {
		if !transientNodeStatus(n.Status) {
			untouched++
			continue
		}

		fetched, ferr := FetchNodeStatus(nsId, infraId, n.Id)
		if ferr != nil {
			log.Warn().Err(ferr).Msgf("reconcileInfraForward: FetchNodeStatus failed for %s; will fall back to KV", n.Id)
		} else if !transientNodeStatus(fetched.Status) {
			if strings.EqualFold(fetched.Status, model.StatusRunning) {
				reconciledRunning++
			}
			log.Info().Msgf("reconcileInfraForward: Node %s reconciled to %s via Spider", n.Id, fetched.Status)
			continue
		}

		nodeObj, gerr := GetNodeObject(nsId, infraId, n.Id)
		if gerr != nil {
			log.Warn().Err(gerr).Msgf("reconcileInfraForward: cannot read Node %s; skipping", n.Id)
			continue
		}
		if strings.TrimSpace(nodeObj.CspResourceName) == "" {
			orphanCands = append(orphanCands, orphanCandidate{
				NodeId:         nodeObj.Id,
				Uid:            nodeObj.Uid,
				ConnectionName: nodeObj.ConnectionName,
			})
			continue
		}

		// cspResourceName exists but Spider gave no final answer. Leave
		// for operator retry; do not silently mark Failed.
		log.Warn().Msgf("reconcileInfraForward: Node %s remains %s after Spider query (cspResourceName=%s); requires retry",
			n.Id, n.Status, nodeObj.CspResourceName)
	}

	// Pass 2: orphan rescue via Spider /allvm + /regvm (one /allvm per
	// distinct connection). For each rescued Node, run FetchNodeStatus once
	// more so its real CSP status is persisted.
	if len(orphanCands) > 0 {
		rescuedIds, notFoundIds := rescueOrphanNodes(nsId, infraId, orphanCands)
		for _, id := range rescuedIds {
			if fetched, ferr := FetchNodeStatus(nsId, infraId, id); ferr == nil &&
				strings.EqualFold(fetched.Status, model.StatusRunning) {
				reconciledRunning++
			}
			rescued++
		}
		for _, id := range notFoundIds {
			nodeObj, gerr := GetNodeObject(nsId, infraId, id)
			if gerr != nil {
				continue
			}
			nodeObj.Status = model.StatusFailed
			nodeObj.TargetAction = model.ActionComplete
			nodeObj.TargetStatus = model.StatusComplete
			nodeObj.SystemMessage = "presumed not created (no cspResourceName, no CSP record matched Uid); marked Failed by reconcileInfraForward"
			UpdateNodeInfo(nsId, infraId, nodeObj)
			markedFailed++
			log.Info().Msgf("reconcileInfraForward: Node %s marked Failed (orphan rescue found no CSP match)", id)
		}
	}

	settleInfraTargetAction(nsId, infraId)

	msg := fmt.Sprintf("Reconciled Infra %s: running=%d, rescued=%d, marked-failed=%d, untouched=%d. Run refine to remove Failed Nodes.",
		infraId, reconciledRunning, rescued, markedFailed, untouched)
	log.Info().Msg(msg)
	return msg, nil
}

// reconcileInfraBackward implements the `abort` action (backward crash
// recovery). The intent is to abandon the entire Infra, so every Node
// (regardless of current status) is driven toward Terminated. To avoid
// per-Node sequential latency on large Infras (1000+ VMs), termination is
// dispatched through ControlNodesInParallel which already implements the
// hierarchical CSP→region→VM rate-limited fan-out used by the regular
// terminate action.
//
// Steps:
//
//  1. Classify Nodes into:
//     - skipped:    Terminated / Terminating (terminate is idempotent)
//     - uncertain:  cspResourceName == "" (might be a CSP-side orphan)
//     - ready:      cspResourceName != "" (terminatable directly)
//  2. For uncertain Nodes only, run rescueOrphanNodes (one Spider /allvm per
//     distinct connection). Matched orphans are absorbed via /regvm and join
//     the ready set; unmatched ones are marked Failed locally.
//  3. ControlNodesInParallel terminates every ready Node concurrently with
//     force=true so per-Node transient guards and the Infra-level
//     TargetAction guard are bypassed.
//
// The Infra-level TargetAction is set to Terminate up-front so concurrent
// status pollers observe the new intent. The final DelInfra is deliberately
// NOT issued — the operator runs DELETE explicitly when teardown completes.
func reconcileInfraBackward(nsId, infraId string) (string, error) {
	infraStatus, err := GetInfraStatus(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}
	if infraStatus == nil {
		return "", fmt.Errorf("Infra %s/%s not found", nsId, infraId)
	}

	if infraTmp, _, ierr := GetInfraObject(nsId, infraId); ierr == nil {
		// Abort/backward recovery drives the entire Infra toward Terminated.
		// Update the intent fields AND clear stale top-level Status /
		// StatusCount that may have been frozen at "Creating" / pre-crash
		// totals — otherwise GetInfraStatus's monotonic-up logic keeps
		// reporting the old CountTotal forever (e.g.
		// "Partial-Terminated:26 (R:0/28)" when only 26 Nodes actually
		// exist).
		infraTmp.TargetAction = model.ActionTerminate
		infraTmp.TargetStatus = model.StatusTerminated
		infraTmp.Status = ""
		infraTmp.StatusCount = model.StatusCountInfo{}
		UpdateInfraInfo(nsId, infraTmp)
	}

	var (
		readyIds          []string
		uncertain         []orphanCandidate
		skippedTerminated int
		markedFailed      int
	)

	for _, n := range infraStatus.Node {
		if strings.EqualFold(n.Status, model.StatusTerminated) ||
			strings.EqualFold(n.Status, model.StatusTerminating) {
			skippedTerminated++
			continue
		}

		nodeObj, gerr := GetNodeObject(nsId, infraId, n.Id)
		if gerr != nil {
			log.Warn().Err(gerr).Msgf("reconcileInfraBackward: cannot read Node %s; skipping", n.Id)
			continue
		}

		if strings.TrimSpace(nodeObj.CspResourceName) == "" {
			uncertain = append(uncertain, orphanCandidate{
				NodeId:         nodeObj.Id,
				Uid:            nodeObj.Uid,
				ConnectionName: nodeObj.ConnectionName,
			})
			continue
		}
		readyIds = append(readyIds, nodeObj.Id)
	}

	// Bounded /allvm calls: one per distinct connection in the uncertain set.
	// Matched orphans get absorbed into Spider and join the ready set.
	rescuedCount := 0
	if len(uncertain) > 0 {
		rescuedIds, notFoundIds := rescueOrphanNodes(nsId, infraId, uncertain)
		readyIds = append(readyIds, rescuedIds...)
		rescuedCount = len(rescuedIds)
		for _, id := range notFoundIds {
			nodeObj, gerr := GetNodeObject(nsId, infraId, id)
			if gerr != nil {
				continue
			}
			nodeObj.Status = model.StatusFailed
			nodeObj.TargetAction = model.ActionComplete
			nodeObj.TargetStatus = model.StatusComplete
			nodeObj.SystemMessage = "presumed not created (no cspResourceName, no CSP record matched Uid); marked Failed by reconcileInfraBackward"
			UpdateNodeInfo(nsId, infraId, nodeObj)
			markedFailed++
		}
	}

	if len(readyIds) > 0 {
		// force=true bypasses both per-Node transient guard and the Infra
		// TargetAction guard inside the parallel control path.
		if cerr := ControlNodesInParallel(nsId, infraId, readyIds, model.ActionTerminate, true); cerr != nil {
			log.Warn().Err(cerr).Msgf("reconcileInfraBackward: parallel terminate reported errors")
		}
	}

	// Sweep markedFailed Nodes by reusing the existing `refine` action.
	// Refine deletes any Node whose status is Failed/Undefined via
	// DelInfraNode(..., "force") and rebuilds the Infra.Node slice. We only
	// invoke it when there is something to remove so rescue-and-terminate
	// Nodes (still Terminating) are never touched.
	cleanedCount := 0
	if markedFailed > 0 {
		if _, rerr := HandleInfraAction(nsId, infraId, model.ActionRefine, true); rerr != nil {
			log.Warn().Err(rerr).Msgf("reconcileInfraBackward: refine cleanup during abort failed")
		} else {
			cleanedCount = markedFailed
		}
	}

	msg := fmt.Sprintf("Aborted Infra %s: terminate-requested=%d (incl. orphan-rescued=%d), marked-failed=%d, refine-cleaned=%d, already-terminated=%d. Run DELETE after termination completes.",
		infraId, len(readyIds), rescuedCount, markedFailed, cleanedCount, skippedTerminated)
	log.Info().Msg(msg)
	return msg, nil
}

// orphanCandidate identifies a TB Node whose cspResourceName is missing —
// possibly because the server crashed before Spider returned the VM IID.
// rescueOrphanNodes resolves whether the VM actually exists on the CSP.
type orphanCandidate struct {
	NodeId         string
	Uid            string // matched against IID.NameId from Spider /allvm
	ConnectionName string
}

// rescueOrphanNodes attempts to absorb CSP-side VMs that exist without a TB
// cspResourceName mapping. For each distinct ConnectionName it queries Spider
// /allvm exactly once and matches both MappedList and OnlyCSPList by
// NameId == Node.Uid (Spider always uses Node.Uid as the CSP-side NameId
// during create).
//
//   - MappedList match: Spider already has the VM registered (the crash
//     happened after Spider stored it but before TB persisted the response).
//     Just fill in the TB Node's cspResourceName/cspResourceId in place —
//     calling /regvm here would fail with "already exists".
//   - OnlyCSPList match: VM exists on the CSP but Spider does not know about
//     it. Import via Spider /regvm, then fill in the TB Node fields.
//
// Returns rescued and not-found node IDs.
func rescueOrphanNodes(nsId, infraId string, candidates []orphanCandidate) (rescued, notFound []string) {
	if len(candidates) == 0 {
		return nil, nil
	}
	byConn := make(map[string][]orphanCandidate)
	for _, c := range candidates {
		if c.ConnectionName == "" || c.Uid == "" {
			notFound = append(notFound, c.NodeId)
			continue
		}
		byConn[c.ConnectionName] = append(byConn[c.ConnectionName], c)
	}

	for connName, group := range byConn {
		statusResp, err := resource.GetCspResourceStatus(connName, model.StrNode)
		if err != nil {
			log.Warn().Err(err).Str("connection", connName).
				Msg("rescueOrphanNodes: /allvm failed; treating group as not-found")
			for _, c := range group {
				notFound = append(notFound, c.NodeId)
			}
			continue
		}
		mapped := make(map[string]string, len(statusResp.AllList.MappedList))
		for _, iid := range statusResp.AllList.MappedList {
			mapped[iid.NameId] = iid.SystemId
		}
		cspOnly := make(map[string]string, len(statusResp.AllList.OnlyCSPList))
		for _, iid := range statusResp.AllList.OnlyCSPList {
			cspOnly[iid.NameId] = iid.SystemId
		}
		log.Info().Str("connection", connName).
			Int("candidates", len(group)).
			Int("mappedVMs", len(mapped)).
			Int("cspOnlyVMs", len(cspOnly)).
			Msg("rescueOrphanNodes: scanning Spider for orphan matches")

		for _, c := range group {
			// 1) Already mapped in Spider — just heal TB metadata.
			if sysId, ok := mapped[c.Uid]; ok {
				nodeObj, gerr := GetNodeObject(nsId, infraId, c.NodeId)
				if gerr != nil {
					log.Warn().Err(gerr).Str("nodeId", c.NodeId).
						Msg("rescueOrphanNodes: cannot load Node for mapped rescue")
					notFound = append(notFound, c.NodeId)
					continue
				}
				nodeObj.CspResourceName = c.Uid
				nodeObj.CspResourceId = sysId
				nodeObj.SystemMessage = "Healed from Spider mapping via reconcile (orphan rescue)"
				UpdateNodeInfo(nsId, infraId, nodeObj)
				rescued = append(rescued, c.NodeId)
				continue
			}
			// 2) Exists only on CSP — import via /regvm.
			if sysId, ok := cspOnly[c.Uid]; ok {
				if err := importNodeFromCsp(nsId, infraId, c.NodeId, connName, c.Uid, sysId); err != nil {
					log.Warn().Err(err).Str("nodeId", c.NodeId).
						Msg("rescueOrphanNodes: import via /regvm failed")
					notFound = append(notFound, c.NodeId)
					continue
				}
				rescued = append(rescued, c.NodeId)
				log.Info().Str("nodeId", c.NodeId).Str("connection", connName).
					Str("cspName", c.Uid).Str("cspSystemId", sysId).
					Msg("rescueOrphanNodes: orphan VM imported into Spider")
				continue
			}
			// 3) No match anywhere — Node never made it to the CSP.
			notFound = append(notFound, c.NodeId)
		}
	}
	return rescued, notFound
}

// importNodeFromCsp invokes Spider /regvm to absorb an existing CSP VM into
// Spider's metadata, then updates the TB Node with cspResourceName /
// cspResourceId so subsequent control actions (status fetch, terminate, ...)
// work normally.
func importNodeFromCsp(nsId, infraId, nodeId, connectionName, name, cspSystemId string) error {
	type regReqInfo struct {
		Name  string `json:"Name"`
		CSPId string `json:"CSPId"`
	}
	type regReq struct {
		ConnectionName string     `json:"ConnectionName"`
		ReqInfo        regReqInfo `json:"ReqInfo"`
	}
	type regRespIID struct {
		NameId   string `json:"NameId"`
		SystemId string `json:"SystemId"`
	}
	type regResp struct {
		IId regRespIID `json:"IId"`
	}

	body := regReq{
		ConnectionName: connectionName,
		ReqInfo:        regReqInfo{Name: name, CSPId: cspSystemId},
	}
	var resp regResp

	client := clientManager.NewHttpClient()
	client.SetTimeout(2 * time.Minute)
	if _, err := clientManager.ExecuteHttpRequest(
		client, "POST", model.SpiderRestUrl+"/regvm",
		nil,
		clientManager.SetUseBody(body),
		&body, &resp,
		clientManager.MediumDuration,
	); err != nil {
		return fmt.Errorf("Spider /regvm failed: %w", err)
	}

	nodeObj, gerr := GetNodeObject(nsId, infraId, nodeId)
	if gerr != nil {
		return fmt.Errorf("GetNodeObject failed after /regvm: %w", gerr)
	}
	if resp.IId.NameId != "" {
		nodeObj.CspResourceName = resp.IId.NameId
	} else {
		nodeObj.CspResourceName = name
	}
	if resp.IId.SystemId != "" {
		nodeObj.CspResourceId = resp.IId.SystemId
	} else {
		nodeObj.CspResourceId = cspSystemId
	}
	nodeObj.SystemMessage = "Imported from CSP via reconcile (orphan rescue)"
	UpdateNodeInfo(nsId, infraId, nodeObj)
	return nil
}
