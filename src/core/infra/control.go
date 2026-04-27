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

	"context"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	cspdirect "github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/rs/zerolog/log"
)

// globalControlSem limits concurrent Spider control calls (DELETE/action) process-wide.
// Spider uses SetCloseConnection(true) with no TCP pooling; sending 1000+ concurrent
// connections causes RST replies at scale. 50 slots match the status-polling budget.
var globalControlSem = make(chan struct{}, 50)

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
		// Re-fetch and clear TargetAction so future operations are not permanently blocked.
		// ControlNodesInParallel only returns error on total failure; individual node
		// failures are surfaced per-node and do not block infra-level operations.
		if freshInfra, _, fetchErr := GetInfraObject(nsId, infraId); fetchErr == nil {
			freshInfra.TargetAction = model.ActionComplete
			freshInfra.TargetStatus = model.StatusComplete
			UpdateInfraInfo(nsId, freshInfra)
		}
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
	NodeId       string
	ProviderName string
	RegionName   string
}

// bulkControlEntry holds all data needed to send a bulk SDK control action for one node
// and perform post-action cleanup without a second GetNodeObject call.
type bulkControlEntry struct {
	nodeId           string
	cspResourceId    string // CSP-native instance ID (e.g., "i-0abc123def456")
	cspResourceName  string // Spider resource name, for post-terminate IID cleanup
	connectionName   string // Spider connection name, for post-terminate IID cleanup
	credentialHolder string // for SDK context key
	cspRegion        string // CSP-native region name for SDK calls
	nodeInfo         model.NodeInfo
}

// getNodeControlRateLimitsForCSP returns rate limits for non-create control operations
// (terminate, reboot, suspend, resume). These operations are far less constrained by
// CSP API limits than VM creation, so we allow much higher per-region concurrency.
func getNodeControlRateLimitsForCSP(cspName string) (maxRegions, maxNodesPerRegion int) {
	config := csp.GetRateLimitConfig(cspName)
	// Regions: same as create (all regions in parallel)
	maxRegions = config.MaxConcurrentRegions
	// Nodes: uncapped per region — TerminateInstances/StopInstances accept up to 1000 IDs
	// per call and don't share the RunInstances throttle bucket. Use 1000 to signal
	// "no effective limit" without changing the semaphore semantics.
	maxNodesPerRegion = 1000
	return
}

// ControlNodesInParallel controls VMs with hierarchical rate limiting
// Level 1: CSPs are processed in parallel
// Level 2: Within each CSP, regions are processed with semaphore (maxConcurrentRegionsPerCSP)
// Level 3: Within each region, VMs are processed with semaphore (maxConcurrentNodesPerRegion)
func ControlNodesInParallel(nsId, infraId string, nodeList []string, action string, force bool) error {
	if len(nodeList) == 0 {
		return nil
	}

	// Step 1: Group VMs by CSP and region; also collect bulk-eligible entries.
	nodeGroups := make(map[string]map[string][]string) // CSP -> Region -> NodeIds
	nodeGroupInfos := make(map[string]NodeControlInfo)  // NodeId -> ControlInfo
	// bulkEntries holds extra per-node data for the bulk SDK fast-path.
	// Populated for nodes whose CSP has a registered BatchVMControlHandler and
	// whose CspResourceId is known. Reboot is always routed through Spider.
	bulkEntries := make(map[string]bulkControlEntry) // NodeId -> bulkControlEntry

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

		// Register as bulk-eligible if the CSP has a bulk handler and the node has a CSP resource ID.
		if action != model.ActionReboot && nodeInfo.CspResourceId != "" {
			if _, hasBulk := cspdirect.GetBatchVMControlHandler(providerName, action); hasBulk {
				bulkEntries[nodeId] = bulkControlEntry{
					nodeId:           nodeId,
					cspResourceId:    nodeInfo.CspResourceId,
					cspResourceName:  nodeInfo.CspResourceName,
					connectionName:   nodeInfo.ConnectionName,
					credentialHolder: nodeInfo.ConnectionConfig.CredentialHolder,
					cspRegion:        nodeInfo.ConnectionConfig.RegionDetail.RegionName,
					nodeInfo:         nodeInfo,
				}
			}
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

			// Use creation rate limits only for Create; all other control actions
			// (Terminate, Reboot, Suspend, Resume) are far less throttle-sensitive.
			var maxRegionsForCSP, maxNodesForRegion int
			if strings.EqualFold(action, model.ActionCreate) {
				maxRegionsForCSP, maxNodesForRegion = getNodeCreateRateLimitsForCSP(providerName)
			} else {
				maxRegionsForCSP, maxNodesForRegion = getNodeControlRateLimitsForCSP(providerName)
			}

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

					// ── Bulk SDK fast-path ──────────────────────────────────────────────
					// For Suspend/Resume/Terminate on CSPs with a registered bulk handler,
					// send all nodes in this region in one (or a few) SDK call(s) instead
					// of N individual Spider HTTP requests. Reboot always uses Spider.
					spiderNodeIds := runBulkControlForRegion(nsId, infraId, providerName, regionName, nodeIdList, action, bulkEntries)
					nodeIdList = spiderNodeIds
					// ── End bulk SDK fast-path ──────────────────────────────────────────

					// Step 4: Process remaining VMs via Spider with rate limiting
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

// runBulkControlForRegion sends a bulk SDK control action for all bulk-eligible nodes in one
// region, grouped further by credentialHolder. Returns the node IDs that were NOT handled by
// the bulk SDK (no handler registered, missing CspResourceId, or SDK call failed) so that
// the caller can route them through the Spider per-node path.
func runBulkControlForRegion(
	nsId, infraId, providerName, regionName string,
	nodeIdList []string,
	action string,
	bulkEntries map[string]bulkControlEntry,
) (spiderFallback []string) {
	handler, hasBulk := cspdirect.GetBatchVMControlHandler(providerName, action)
	if !hasBulk {
		return nodeIdList // no handler registered for this CSP+action
	}

	// Sub-group by (credentialHolder, cspRegion) — usually one group per region.
	type groupKey struct{ holder, cspRegion string }
	type holderGroup struct {
		key     groupKey
		entries []bulkControlEntry
	}
	holderMap := make(map[groupKey]*holderGroup)

	for _, nodeId := range nodeIdList {
		be, ok := bulkEntries[nodeId]
		if !ok || be.cspResourceId == "" {
			spiderFallback = append(spiderFallback, nodeId)
			continue
		}
		k := groupKey{be.credentialHolder, be.cspRegion}
		if holderMap[k] == nil {
			holderMap[k] = &holderGroup{key: k}
		}
		holderMap[k].entries = append(holderMap[k].entries, be)
	}

	for _, hg := range holderMap {
		// Pre-bulk: set transitional status in etcd + StatusStore.
		// For Terminate, also cancel SSH sessions and remove bastion references.
		for _, be := range hg.entries {
			applyBulkTransitionalStatus(nsId, infraId, be, action)
		}

		// Collect CSP resource IDs for the SDK call.
		ids := make([]string, len(hg.entries))
		for i, be := range hg.entries {
			ids[i] = be.cspResourceId
		}

		sdkCtx := context.WithValue(context.Background(), model.CtxKeyCredentialHolder, hg.key.holder)
		statuses, err := handler(sdkCtx, hg.key.cspRegion, ids)
		if err != nil {
			log.Warn().Err(err).
				Str("provider", providerName).
				Str("region", hg.key.cspRegion).
				Int("count", len(ids)).
				Msgf("[BulkControl] %s SDK call failed; routing %d nodes to Spider fallback", action, len(ids))
			for _, be := range hg.entries {
				spiderFallback = append(spiderFallback, be.nodeId)
			}
			continue
		}

		// Post-bulk: update StatusStore and launch per-node cleanup.
		idToEntry := make(map[string]bulkControlEntry, len(hg.entries))
		for _, be := range hg.entries {
			idToEntry[be.cspResourceId] = be
			newStatus, found := statuses[be.cspResourceId]
			if !found {
				newStatus = bulkTransitionalStatus(action)
			}
			globalStatusStore.Update(nsId, infraId, be.nodeId, func(e *StatusEntry) {
				e.Status = newStatus
				e.NativeStatus = newStatus
				e.LastUpdated = time.Now()
				e.Priority = PollHigh // trigger quick individual follow-up poll
				e.NextPollAt = time.Now()
			})
			go postBulkControlCleanup(nsId, infraId, be, action)
		}

		log.Debug().
			Str("provider", providerName).
			Str("region", hg.key.cspRegion).
			Str("action", action).
			Int("sent", len(ids)).
			Int("accepted", len(statuses)).
			Msg("[BulkControl] batch complete")

		// For Terminate: block until all accepted nodes actually reach Terminated.
		// This preserves the synchronous behavior that Spider's DELETE /vm provided —
		// callers expect the API to return only after the action is truly complete.
		if strings.EqualFold(action, model.ActionTerminate) {
			if statusFn, hasStatus := cspdirect.GetBatchVMStatusHandler(providerName); hasStatus {
				acceptedIds := make([]string, 0, len(statuses))
				for id := range statuses {
					acceptedIds = append(acceptedIds, id)
				}
				waitBulkTerminated(sdkCtx, nsId, infraId, hg.key.cspRegion, acceptedIds, idToEntry, statusFn)
			}
		}
	}

	return spiderFallback
}

// waitBulkTerminated polls the CSP until every instance in acceptedIds reports Terminated
// (or the 10-minute deadline passes). It updates the StatusStore for each node as it
// transitions, so the StatusAgent picks up the final state without an extra round-trip.
func waitBulkTerminated(
	ctx context.Context,
	nsId, infraId, region string,
	pendingIds []string,
	idToEntry map[string]bulkControlEntry,
	statusFn cspdirect.BatchVMStatusFunc,
) {
	const (
		pollInterval = 5 * time.Second
		maxWait      = 10 * time.Minute
	)
	deadline := time.Now().Add(maxWait)

	for len(pendingIds) > 0 && time.Now().Before(deadline) {
		time.Sleep(pollInterval)

		currentStatuses, err := statusFn(ctx, region, pendingIds)
		if err != nil {
			log.Warn().Err(err).Str("region", region).
				Msg("[BulkControl] status poll failed during terminate wait; skipping remaining wait")
			return
		}

		var remaining []string
		for _, id := range pendingIds {
			s, found := currentStatuses[id]
			if !found || strings.EqualFold(s, model.StatusTerminated) {
				if be, ok := idToEntry[id]; ok {
					globalStatusStore.Update(nsId, infraId, be.nodeId, func(e *StatusEntry) {
						e.Status = model.StatusTerminated
						e.NativeStatus = model.StatusTerminated
						e.LastUpdated = time.Now()
						e.Priority = PollHigh
						e.NextPollAt = time.Now()
					})
				}
			} else {
				remaining = append(remaining, id)
			}
		}
		pendingIds = remaining
	}

	if len(pendingIds) > 0 {
		log.Warn().Str("region", region).Int("remaining", len(pendingIds)).
			Msg("[BulkControl] some nodes did not reach Terminated within 10 min; StatusAgent will follow up")
	} else {
		log.Debug().Str("region", region).Msg("[BulkControl] all nodes confirmed Terminated")
	}
}

// applyBulkTransitionalStatus sets the in-flight status for a node in both etcd and StatusStore
// before the bulk SDK call so that StatusAgent and API callers see a consistent state.
func applyBulkTransitionalStatus(nsId, infraId string, be bulkControlEntry, action string) {
	temp := be.nodeInfo
	switch action {
	case model.ActionTerminate:
		temp.TargetAction = model.ActionTerminate
		temp.TargetStatus = model.StatusTerminated
		temp.Status = model.StatusTerminating
		CancelActiveCommandsForNode(be.nodeId)
		RemoveBastionNodes(nsId, infraId, "", "", be.nodeId) //nolint:errcheck
	case model.ActionSuspend:
		temp.TargetAction = model.ActionSuspend
		temp.TargetStatus = model.StatusSuspended
		temp.Status = model.StatusSuspending
	case model.ActionResume:
		temp.TargetAction = model.ActionResume
		temp.TargetStatus = model.StatusRunning
		temp.Status = model.StatusResuming
	}
	UpdateNodeInfo(nsId, infraId, temp)
	globalStatusStore.Update(nsId, infraId, be.nodeId, func(e *StatusEntry) {
		e.Status = temp.Status
		e.TargetAction = temp.TargetAction
		e.TargetStatus = temp.TargetStatus
		e.Priority = PollHigh
		e.NextPollAt = time.Now()
	})
}

// bulkTransitionalStatus returns the expected in-flight TB status for an action
// when the CSP response does not include a per-instance status.
func bulkTransitionalStatus(action string) string {
	switch action {
	case model.ActionSuspend:
		return model.StatusSuspending
	case model.ActionResume:
		return model.StatusResuming
	case model.ActionTerminate:
		return model.StatusTerminating
	default:
		return model.StatusUndefined
	}
}

// postBulkControlCleanup runs TB metadata cleanup after a successful bulk SDK control call.
// For Terminate: updates associated-object lists and schedules async Spider IID cleanup.
// For Suspend/Resume: no additional TB metadata work needed; StatusAgent handles IP refresh.
func postBulkControlCleanup(nsId, infraId string, be bulkControlEntry, action string) {
	if action != model.ActionTerminate {
		return
	}
	key := common.GenInfraKey(nsId, infraId, be.nodeId)
	ni := be.nodeInfo
	_, err := resource.UpdateAssociatedObjectList(nsId, model.StrImage, ni.ImageId, model.StrDelete, key)
	if err != nil {
		resource.UpdateAssociatedObjectList(nsId, model.StrCustomImage, ni.ImageId, model.StrDelete, key) //nolint:errcheck
	}
	resource.UpdateAssociatedObjectList(nsId, model.StrSSHKey, ni.SshKeyId, model.StrDelete, key)          //nolint:errcheck
	resource.UpdateAssociatedObjectList(nsId, model.StrVNet, ni.VNetId, model.StrDelete, key)              //nolint:errcheck
	for _, sg := range ni.SecurityGroupIds {
		resource.UpdateAssociatedObjectList(nsId, model.StrSecurityGroup, sg, model.StrDelete, key) //nolint:errcheck
	}
	for _, disk := range ni.DataDiskIds {
		resource.UpdateAssociatedObjectList(nsId, model.StrDataDisk, disk, model.StrDelete, key) //nolint:errcheck
	}
	// Spider IID cleanup: async, non-blocking.
	// The instance is already terminating in AWS; Spider force-delete just removes
	// the vm_iid_infos record. We fire-and-forget through globalControlSem to rate-limit.
	asyncSpiderForceDeleteVM(be.connectionName, be.cspResourceName)
}

// asyncSpiderForceDeleteVM deletes Spider's vm_iid_infos record for a VM that was terminated
// directly via the AWS SDK. The instance may still be in "shutting-down" state when called;
// Spider will poll until "terminated" (fast path since AWS completes in < 60 s typically).
// Errors are non-fatal: the instance is already gone from AWS, only Spider's DB record is stale.
func asyncSpiderForceDeleteVM(connectionName, cspResourceName string) {
	if cspResourceName == "" {
		return
	}
	go func() {
		globalControlSem <- struct{}{} // share the same rate-limit budget as Spider control calls
		defer func() { <-globalControlSem }()

		client := clientManager.NewHttpClient()
		client.SetTimeout(15 * time.Minute)
		url := model.SpiderRestUrl + "/vm/" + cspResourceName + "?force=true"
		requestBody := model.SpiderConnectionName{ConnectionName: connectionName}
		var ignored struct{}
		_, err := clientManager.ExecuteHttpRequest(
			client, "DELETE", url, nil,
			clientManager.SetUseBody(requestBody), &requestBody, &ignored,
			clientManager.MediumDuration,
		)
		if err != nil {
			log.Debug().Err(err).Str("vm", cspResourceName).
				Msg("[BulkControl] Spider IID cleanup failed (non-critical; instance already terminated in CSP)")
		} else {
			log.Debug().Str("vm", cspResourceName).Msg("[BulkControl] Spider IID cleanup completed")
		}
	}()
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

	// Rate-limit concurrent Spider control calls to prevent RST storms at scale.
	// Released immediately after the Spider call (before polling / cleanup) so the
	// slot is not held for the full per-node duration.
	globalControlSem <- struct{}{}

	// Retry on transient network errors (connection reset, EOF, broken pipe).
	// NHN and some other CSPs occasionally reset connections during Floating IP
	// operations; a single retry is usually enough for the TCP session to recover.
	const maxControlRetries = 2
	for attempt := 0; attempt <= maxControlRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt)) * time.Second // 2s, 4s
			log.Warn().Msgf("[ControlNodeAsync] VM %s: transient error on %s (attempt %d/%d), retrying in %v: %v",
				nodeId, action, attempt, maxControlRetries+1, backoff, err)
			time.Sleep(backoff)
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
		if err == nil || !isTransientNetworkError(err) {
			break
		}
	}

	// Release the control semaphore immediately — post-call work (FetchNodeStatus,
	// polling, cleanup) must not hold the slot or other goroutines starve.
	<-globalControlSem

	if err != nil {
		log.Error().Err(err).Msg("")
		callResult.Error = err
		// Sync actual node state from CSP even on failure — the operation may have
		// partially succeeded (e.g. VM terminated but Floating IP release failed).
		// Without this, the KV store retains the transitional status set before the
		// Spider call (e.g. Terminating) while the VM may still be Running.
		if syncInfo, syncErr := FetchNodeStatus(nsId, infraId, nodeId); syncErr != nil {
			log.Warn().Err(syncErr).Msgf("[ControlNodeAsync] VM %s: failed to sync status after %s error", nodeId, action)
		} else {
			log.Debug().Msgf("[ControlNodeAsync] VM %s: post-error status sync — Status: %s, NativeStatus: %s",
				nodeId, syncInfo.Status, syncInfo.NativeStatus)
		}
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

	// For terminate: some CSPs (e.g. IBM VPC) delete asynchronously — Spider returns
	// success with status=Terminating immediately, but the VM still holds subnet/VPC
	// resources until the CSP propagates the deletion. Poll until the VM actually leaves
	// Terminating state so that subsequent shared-resource cleanup (VPC, SG) does not fail.
	//
	// For CSPs with a registered batch SDK handler (AWS, GCP, Azure, Alibaba, Tencent),
	// skip this loop entirely: VPC resources are not locked by a terminating instance on
	// those platforms, and StatusAgent polls termination progress efficiently via batch
	// SDK calls. Holding goroutines here for up to 10 min at 1000+ scale would starve
	// waiting goroutines and cause cascading Spider connection resets.
	if action == model.ActionTerminate {
		initiallyTerminating := err != nil || strings.EqualFold(nodeStatusInfo.Status, model.StatusTerminating)
		_, hasSDKHandler := cspdirect.GetBatchVMStatusHandler(temp.ConnectionConfig.ProviderName)
		if initiallyTerminating && !hasSDKHandler {
			const pollInterval = 5 * time.Second
			const pollTimeout = 10 * time.Minute
			deadline := time.Now().Add(pollTimeout)
			log.Info().Msgf("[ControlNodeAsync] VM %s: CSP reports Terminating — polling every %v until deletion propagates (timeout %v)",
				nodeId, pollInterval, pollTimeout)
			for time.Now().Before(deadline) {
				time.Sleep(pollInterval)
				fetchedStatus, fetchErr := FetchNodeStatus(nsId, infraId, nodeId)
				if fetchErr != nil {
					if strings.Contains(fetchErr.Error(), "temporarily blocked") {
						// Circuit breaker is open; wait for it to reset (~30 s) and retry.
						continue
					}
					// VM is no longer accessible from CSP — deletion propagated.
					log.Debug().Msgf("[ControlNodeAsync] VM %s: CSP no longer reports VM, termination confirmed", nodeId)
					break
				}
				if !strings.EqualFold(fetchedStatus.Status, model.StatusTerminating) {
					log.Debug().Msgf("[ControlNodeAsync] VM %s: Termination polling done, status=%s", nodeId, fetchedStatus.Status)
					break
				}
			}
		}
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

// isTransientNetworkError returns true for errors that are safe to retry
// (connection reset by peer, unexpected EOF, broken pipe).
func isTransientNetworkError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "connection reset by peer") ||
		strings.Contains(s, "EOF") ||
		strings.Contains(s, "broken pipe") ||
		strings.Contains(s, "i/o timeout")
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
			strings.Contains(infra.Status, model.StatusResuming) ||
			strings.Contains(infra.Status, model.StatusSuspending) ||
			strings.Contains(infra.Status, model.StatusRebooting) {

			return errors.New(action + " is not allowed for Infra under " + infra.Status)
		}
		// Terminating is allowed to proceed for Terminate (idempotent — nodes in flight
		// will be confirmed Terminated by StatusAgent; any that failed can be retried).
		if strings.Contains(infra.Status, model.StatusTerminating) {
			if strings.EqualFold(action, model.ActionTerminate) {
				return nil
			}
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
	// Read node IDs directly from KV — avoids triggering a full GetInfraStatus
	// (which would call Spider for every stale Running node) when all we need
	// is to identify transient/failed candidates.
	nodeIds, err := ListNodeId(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}
	if len(nodeIds) == 0 {
		// Try the infra object for a structured "not found" message.
		if _, exists, _ := GetInfraObject(nsId, infraId); !exists {
			return "", fmt.Errorf("Infra %s/%s not found", nsId, infraId)
		}
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
		if stuckInPrep && len(nodeIds) == 0 {
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
		if stuckInPrep && len(nodeIds) > 0 {
			// Some Nodes exist (provisioning crashed shortly after Status moved
			// past Preparing/Prepared). Clear the stale top-level Status so it
			// is recomputed fresh; the regular per-Node loop below handles the rest.
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

	// Pass 1: read each Node's last-known status from KV (no Spider call for
	// healthy Running nodes).  Only transient/failed Nodes need Spider truth.
	//
	// Orphan candidates are Nodes that may have a VM on CSP without a TB
	// cspResourceName link. Two sources:
	//   (a) Transient status (Creating/Undefined/…) + no cspResourceName — the
	//       classic crash-recovery pattern: Spider returned the VM IID but TB
	//       never persisted it before the process died.
	//   (b) Failed status + no cspResourceName — Spider POST /vm returned 500
	//       AFTER the VM was created on CSP (e.g. NHN Floating IP assignment
	//       failed). These are potential billing orphans that must be rescued or
	//       confirmed absent before being treated as truly Failed.
	// Pass 1: classify each Node. Nodes without cspResourceName are deferred
	// directly to orphan rescue — Spider vmstatus cannot resolve them by name,
	// so calling FetchNodeStatus would always return Undefined (wasted round-trip).
	// Nodes with a known cspResourceName are queried individually via Spider.
	var orphanCands []orphanCandidate
	var knownCspCands []string // nodeIds with cspResourceName that are still transient
	for _, nodeId := range nodeIds {
		nodeObj, gerr := GetNodeObject(nsId, infraId, nodeId)
		if gerr != nil {
			log.Warn().Err(gerr).Msgf("reconcileInfraForward: cannot read Node %s; skipping", nodeId)
			untouched++
			continue
		}

		isTransient := transientNodeStatus(nodeObj.Status)
		isFailed := strings.EqualFold(nodeObj.Status, model.StatusFailed)

		if !isTransient && !isFailed {
			untouched++
			continue
		}

		// Failed nodes with a known CspResourceName are definitively done;
		// no orphan search needed.
		if isFailed && strings.TrimSpace(nodeObj.CspResourceName) != "" {
			untouched++
			continue
		}

		if strings.TrimSpace(nodeObj.CspResourceName) == "" {
			// No cspResourceName: Spider cannot resolve this VM by name.
			// Skip FetchNodeStatus (always returns Undefined without a name)
			// and go straight to orphan rescue via allVM.
			if isFailed {
				log.Info().Msgf("reconcileInfraForward: Node %s is Failed with no cspResourceName; "+
					"deferring to orphan rescue via /allvm", nodeId)
			}
			orphanCands = append(orphanCands, orphanCandidate{
				NodeId:         nodeObj.Id,
				Uid:            nodeObj.Uid,
				ConnectionName: nodeObj.ConnectionName,
			})
			continue
		}

		// cspResourceName known — Spider can look this up directly.
		knownCspCands = append(knownCspCands, nodeId)
	}

	// Resolve nodes with known cspResourceName.
	// AWS nodes are batched by (credentialHolder, region) into a single DescribeInstances call each.
	// Non-AWS nodes fall back to individual Spider vmstatus calls.
	type fetchResult struct {
		nodeId string
		status string
		err    error
	}

	// batchKey groups nodes that share a (provider, credentialHolder, region) triple
	// and have a registered direct-SDK handler — they can be queried in one batch call.
	type batchKey struct {
		provider         string
		credentialHolder string
		region           string
	}
	type batchItem struct {
		nodeId     string
		instanceId string // CspResourceId
	}
	batchGroups := make(map[batchKey][]batchItem)
	var spiderCands []string

	for _, nodeId := range knownCspCands {
		nodeObj, gerr := GetNodeObject(nsId, infraId, nodeId)
		if gerr != nil {
			spiderCands = append(spiderCands, nodeId)
			continue
		}
		provider := nodeObj.ConnectionConfig.ProviderName
		if _, hasHandler := cspdirect.GetBatchVMStatusHandler(provider); hasHandler && nodeObj.CspResourceId != "" {
			key := batchKey{
				provider:         provider,
				credentialHolder: nodeObj.ConnectionConfig.CredentialHolder,
				region:           nodeObj.ConnectionConfig.RegionDetail.RegionName,
			}
			batchGroups[key] = append(batchGroups[key], batchItem{nodeId: nodeId, instanceId: nodeObj.CspResourceId})
		} else {
			spiderCands = append(spiderCands, nodeId)
		}
	}

	// One goroutine per (provider, credentialHolder, region) group — one batch SDK call each.
	batchTotal := 0
	for _, grp := range batchGroups {
		batchTotal += len(grp)
	}
	batchCh := make(chan fetchResult, batchTotal)

	for key, group := range batchGroups {
		go func(k batchKey, grp []batchItem) {
			handler, _ := cspdirect.GetBatchVMStatusHandler(k.provider)
			ids := make([]string, len(grp))
			for i, item := range grp {
				ids[i] = item.instanceId
			}
			sdkCtx := context.WithValue(context.Background(), model.CtxKeyCredentialHolder, k.credentialHolder)
			statuses, berr := handler(sdkCtx, k.region, ids)
			for _, item := range grp {
				if berr != nil {
					batchCh <- fetchResult{nodeId: item.nodeId, err: berr}
					continue
				}
				s, ok := statuses[item.instanceId]
				if !ok {
					s = model.StatusUndefined
				}
				batchCh <- fetchResult{nodeId: item.nodeId, status: s}
			}
		}(key, group)
	}

	// Spider path: individual parallel FetchNodeStatus for non-AWS nodes.
	fetchCh := make(chan fetchResult, len(spiderCands))
	for _, nodeId := range spiderCands {
		go func(nid string) {
			fetched, ferr := FetchNodeStatus(nsId, infraId, nid)
			fetchCh <- fetchResult{nodeId: nid, status: fetched.Status, err: ferr}
		}(nodeId)
	}

	// Collect direct-SDK batch results and update node status directly.
	for i := 0; i < batchTotal; i++ {
		r := <-batchCh
		if r.err != nil {
			log.Warn().Err(r.err).Msgf("reconcileInfraForward: direct SDK batch failed for %s; leaving for retry", r.nodeId)
			continue
		}
		nodeObj, gerr := GetNodeObject(nsId, infraId, r.nodeId)
		if gerr != nil {
			log.Warn().Err(gerr).Msgf("reconcileInfraForward: cannot load Node %s after SDK batch", r.nodeId)
			continue
		}
		if !transientNodeStatus(r.status) {
			nodeObj.Status = r.status
			UpdateNodeInfo(nsId, infraId, nodeObj)
			if strings.EqualFold(r.status, model.StatusRunning) {
				reconciledRunning++
			}
			log.Info().Msgf("reconcileInfraForward: Node %s reconciled to %s via direct SDK batch", r.nodeId, r.status)
		} else {
			log.Warn().Msgf("reconcileInfraForward: Node %s remains %s after SDK batch; requires retry", r.nodeId, r.status)
		}
	}

	// Collect Spider path results.
	for range spiderCands {
		r := <-fetchCh
		if r.err != nil {
			log.Warn().Err(r.err).Msgf("reconcileInfraForward: FetchNodeStatus failed for %s; leaving for retry", r.nodeId)
			continue
		}
		if !transientNodeStatus(r.status) {
			if strings.EqualFold(r.status, model.StatusRunning) {
				reconciledRunning++
			}
			log.Info().Msgf("reconcileInfraForward: Node %s reconciled to %s via Spider", r.nodeId, r.status)
		} else {
			log.Warn().Msgf("reconcileInfraForward: Node %s remains %s after Spider query; requires retry", r.nodeId, r.status)
		}
	}

	// Pass 2: orphan rescue via Spider /allvm + /regvm (already parallelised
	// across distinct connections inside rescueOrphanNodes).
	// Post-rescue FetchNodeStatus calls are also parallelised here.
	if len(orphanCands) > 0 {
		rescuedIds, notFoundIds := rescueOrphanNodes(nsId, infraId, orphanCands)

		postCh := make(chan fetchResult, len(rescuedIds))
		for _, id := range rescuedIds {
			go func(nid string) {
				fetched, ferr := FetchNodeStatus(nsId, infraId, nid)
				postCh <- fetchResult{nodeId: nid, status: fetched.Status, err: ferr}
			}(id)
		}
		for range rescuedIds {
			r := <-postCh
			if r.err == nil && strings.EqualFold(r.status, model.StatusRunning) {
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
	// Read node IDs directly from KV — same rationale as reconcileInfraForward:
	// avoids Spider calls for the many Running nodes we only need to enumerate.
	nodeIds, err := ListNodeId(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}
	if len(nodeIds) == 0 {
		if _, exists, _ := GetInfraObject(nsId, infraId); !exists {
			return "", fmt.Errorf("Infra %s/%s not found", nsId, infraId)
		}
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

	for _, nodeId := range nodeIds {
		nodeObj, gerr := GetNodeObject(nsId, infraId, nodeId)
		if gerr != nil {
			log.Warn().Err(gerr).Msgf("reconcileInfraBackward: cannot read Node %s; skipping", nodeId)
			continue
		}

		if strings.EqualFold(nodeObj.Status, model.StatusTerminated) ||
			strings.EqualFold(nodeObj.Status, model.StatusTerminating) {
			skippedTerminated++
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

	// Fire all /allvm requests in parallel — each can take minutes on large CSPs.
	type connScan struct {
		connName   string
		group      []orphanCandidate
		statusResp model.CspResourceStatusResponse
		err        error
	}
	scanCh := make(chan connScan, len(byConn))
	for connName, group := range byConn {
		go func(cn string, grp []orphanCandidate) {
			resp, err := resource.GetCspResourceStatus(cn, model.StrNode)
			scanCh <- connScan{connName: cn, group: grp, statusResp: resp, err: err}
		}(connName, group)
	}

	for range byConn {
		scan := <-scanCh
		connName := scan.connName
		group := scan.group
		if scan.err != nil {
			log.Warn().Err(scan.err).Str("connection", connName).
				Msg("rescueOrphanNodes: /allvm failed; treating group as not-found")
			for _, c := range group {
				notFound = append(notFound, c.NodeId)
			}
			continue
		}
		mapped := make(map[string]string, len(scan.statusResp.AllList.MappedList))
		for _, iid := range scan.statusResp.AllList.MappedList {
			mapped[iid.NameId] = iid.SystemId
		}
		cspOnly := make(map[string]string, len(scan.statusResp.AllList.OnlyCSPList))
		for _, iid := range scan.statusResp.AllList.OnlyCSPList {
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
