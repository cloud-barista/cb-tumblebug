/*
Copyright 2019 The Cloud-Barista Authors.
<!-- SPDX-License-Identifier: Apache-2.0 -->
*/

package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	cspdirect "github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/rs/zerolog/log"
)

// ConvertNodeInfoToNodeStatusInfo converts NodeInfo to NodeStatusInfo for Infra status operations
func ConvertNodeInfoToNodeStatusInfo(nodeInfo model.NodeInfo) model.NodeStatusInfo {
	return model.NodeStatusInfo{
		Id:              nodeInfo.Id,
		Uid:             nodeInfo.Uid,
		CspResourceName: nodeInfo.CspResourceName,
		CspResourceId:   nodeInfo.CspResourceId,
		Name:            nodeInfo.Name,
		Status:          nodeInfo.Status,
		TargetStatus:    nodeInfo.TargetStatus,
		TargetAction:    nodeInfo.TargetAction,
		NativeStatus:    "", // NodeInfo doesn't have NativeStatus, will be updated by status fetch
		MonAgentStatus:  nodeInfo.MonAgentStatus,
		SystemMessage:   nodeInfo.SystemMessage,
		CreatedTime:     nodeInfo.CreatedTime,
		PublicIp:        nodeInfo.PublicIP,
		PrivateIp:       nodeInfo.PrivateIP,
		SSHPort:         nodeInfo.SSHPort,
		Location:        nodeInfo.Location,
	}
}

// ConvertNodeInfoListToNodeStatusInfoList converts a slice of NodeInfo to NodeStatusInfo for Infra status operations
func ConvertNodeInfoListToNodeStatusInfoList(nodeInfoList []model.NodeInfo) []model.NodeStatusInfo {
	nodeStatusInfoList := make([]model.NodeStatusInfo, len(nodeInfoList))
	for i, nodeInfo := range nodeInfoList {
		nodeStatusInfoList[i] = ConvertNodeInfoToNodeStatusInfo(nodeInfo)
	}
	return nodeStatusInfoList
}

// ensureNodeStatusInfoComplete ensures all Nodes from NodeInfo are represented in InfraStatus.Node
// This handles cases where Node status fetch might have failed or Node is newly created
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
		Node:            ConvertNodeInfoListToNodeStatusInfoList(infraInfo.Node),
		// MasterNodeId, MasterIp, MasterSSHPort will be set by status determination logic
	}
}

// ConvertNodeInfoFieldsToNodeStatusInfo converts NodeInfo fields into existing NodeStatusInfo
// NodeInfo is considered the trusted source, so all relevant fields are converted
func ConvertNodeInfoFieldsToNodeStatusInfo(nodeStatus *model.NodeStatusInfo, nodeInfo model.NodeInfo) {
	// Always convert from NodeInfo as it's the trusted source
	nodeStatus.CreatedTime = nodeInfo.CreatedTime
	nodeStatus.SystemMessage = nodeInfo.SystemMessage
	nodeStatus.MonAgentStatus = nodeInfo.MonAgentStatus
	nodeStatus.TargetStatus = nodeInfo.TargetStatus
	nodeStatus.TargetAction = nodeInfo.TargetAction

	// Convert network information - NodeInfo is authoritative
	nodeStatus.PublicIp = nodeInfo.PublicIP
	nodeStatus.PrivateIp = nodeInfo.PrivateIP
	nodeStatus.SSHPort = nodeInfo.SSHPort

	// Convert Status only if nodeStatus doesn't have real-time CSP status
	// Keep NativeStatus from CSP calls, but convert Status from NodeInfo if no real-time data
	if nodeStatus.NativeStatus == "" {
		nodeStatus.Status = nodeInfo.Status
	}
	// If we have real-time CSP status (NativeStatus), keep the current Status
}

// ConvertNodeInfoFieldsToNodeStatusInfoList converts NodeInfo fields into corresponding NodeStatusInfo list
func ConvertNodeInfoFieldsToNodeStatusInfoList(nodeStatusList []model.NodeStatusInfo, nodeInfoList []model.NodeInfo) {
	// Create a map for efficient lookup
	nodeInfoMap := make(map[string]model.NodeInfo)
	for _, nodeInfo := range nodeInfoList {
		nodeInfoMap[nodeInfo.Id] = nodeInfo
	}

	// Convert each Node status if corresponding NodeInfo exists
	for i := range nodeStatusList {
		if nodeInfo, exists := nodeInfoMap[nodeStatusList[i].Id]; exists {
			ConvertNodeInfoFieldsToNodeStatusInfo(&nodeStatusList[i], nodeInfo)
		}
	}
}

// GetNodeIdNameInDetail is func to get ID and Name details
func GetNodeIdNameInDetail(nsId string, infraId string, nodeId string) (*model.IdNameInDetailInfo, error) {
	key := common.GenInfraKey(nsId, infraId, nodeId)
	keyValue, _, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return &model.IdNameInDetailInfo{}, err
	}
	nodeTmp := model.NodeInfo{}
	json.Unmarshal([]byte(keyValue.Value), &nodeTmp)

	var idDetails model.IdNameInDetailInfo

	idDetails.IdInTb = nodeTmp.Id
	idDetails.IdInSp = nodeTmp.CspResourceName
	idDetails.IdInCsp = nodeTmp.CspResourceId
	idDetails.NameInCsp = nodeTmp.CspResourceName

	type spiderReqTmp struct {
		ConnectionName string `json:"ConnectionName"`
		ResourceType   string `json:"ResourceType"`
	}
	type spiderResTmp struct {
		Name string `json:"Name"`
	}

	var requestBody spiderReqTmp
	requestBody.ConnectionName = nodeTmp.ConnectionName
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

// [Infra and Node status management]

// nodeStatusInfoFromEntry builds a NodeStatusInfo from a StatusStore entry.
func nodeStatusInfoFromEntry(e StatusEntry) model.NodeStatusInfo {
	return model.NodeStatusInfo{
		Id:              e.NodeId,
		Name:            e.Name,
		CspResourceName: e.CspResourceName,
		Status:          e.Status,
		NativeStatus:    e.NativeStatus,
		TargetStatus:    e.TargetStatus,
		TargetAction:    e.TargetAction,
		PublicIp:        e.PublicIP,
		PrivateIp:       e.PrivateIP,
		SSHPort:         e.SSHPort,
		Location:        e.Location,
		MonAgentStatus:  e.MonAgentStatus,
		CreatedTime:     e.CreatedTime,
		SystemMessage:   e.SystemMessage,
	}
}

// nodeStatusesFromStore builds the node status list for an Infra from the
// StatusAgent's in-memory store, which the daemon keeps fresh (batch sweep +
// individual polls). This replaces a live per-node CSP fanout that materializes
// N statuses and spawns N goroutines — the source of OOM on large infras.
// Nodes not yet in the store (e.g. freshly provisioned before the first sweep)
// fall back to their stored object, with no CSP call.
func nodeStatusesFromStore(nsId, infraId string, nodeList []string) []model.NodeStatusInfo {
	byId := make(map[string]StatusEntry, len(nodeList))
	for _, e := range globalStatusStore.Snapshot() {
		if e.NsId == nsId && e.InfraId == infraId {
			byId[e.NodeId] = e
		}
	}

	result := make([]model.NodeStatusInfo, 0, len(nodeList))
	var missing []string
	for _, nodeId := range nodeList {
		if e, ok := byId[nodeId]; ok {
			result = append(result, nodeStatusInfoFromEntry(e))
		} else {
			missing = append(missing, nodeId)
		}
	}
	for _, nodeId := range missing {
		nodeInfo, err := GetNodeObject(nsId, infraId, nodeId)
		if err != nil {
			continue
		}
		result = append(result, ConvertNodeInfoListToNodeStatusInfoList([]model.NodeInfo{nodeInfo})...)
	}
	return result
}

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
		err := fmt.Errorf("Not found [%s]", key)
		log.Error().Err(err).Msg("")
		return &model.InfraStatusInfo{}, err
	}

	infraStatus := model.InfraStatusInfo{}
	json.Unmarshal([]byte(keyValue.Value), &infraStatus)

	infraTmp := model.InfraInfo{}
	json.Unmarshal([]byte(keyValue.Value), &infraTmp)

	nodeList, err := ListNodeId(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return &model.InfraStatusInfo{}, err
	}
	if len(nodeList) == 0 {
		// Infra has no Nodes - check if it's in provisioning phase or truly empty
		currentStatus := infraTmp.Status
		if strings.Contains(currentStatus, model.StatusPreparing) || strings.Contains(currentStatus, model.StatusPrepared) ||
			strings.Contains(currentStatus, model.StatusCreating) || strings.Contains(currentStatus, model.StatusFailed) {
			// Infra is in provisioning phase or failed - keep current status
			infraStatus.Status = currentStatus
		} else {
			// Infra was already running/completed but now has no Nodes - set to Empty
			infraStatus.Status = model.StatusEmpty
		}
		infraStatus.StatusCount = model.StatusCountInfo{}
		infraStatus.Node = []model.NodeStatusInfo{}
		return &infraStatus, nil
	}

	// Serve node statuses from the StatusAgent's in-memory store instead of a live
	// per-node CSP fanout: the daemon keeps the store fresh, and materializing N
	// statuses + goroutines here OOMs on large infras.
	nodeStatusList := nodeStatusesFromStore(nsId, infraId, nodeList)
	// log.Debug().Msgf("Fetched %d VM statuses for Infra %s", len(nodeStatusList), infraId)
	// log.Debug().Msgf("VM Status List: %+v", nodeStatusList)

	// Copy results to infraStatus
	infraStatus.Node = nodeStatusList

	// If status fetch unexpectedly returned nothing, fall back to NodeInfo from KV.
	if len(infraStatus.Node) == 0 {
		nodeInfos, err := ListInfraNodeInfo(nsId, infraId)
		if err == nil && len(nodeInfos) > 0 {
			infraStatus.Node = ConvertNodeInfoListToNodeStatusInfoList(nodeInfos)
		}
	}

	// Identify master node from the already-fetched node statuses (no extra KV reads).
	for _, v := range infraStatus.Node {
		if strings.EqualFold(v.Status, model.StatusRunning) {
			infraStatus.MasterNodeId = v.Id
			infraStatus.MasterIp = v.PublicIp
			infraStatus.MasterSSHPort = v.SSHPort
			break
		}
	}

	sort.Slice(infraStatus.Node, func(i, j int) bool {
		return infraStatus.Node[i].Id < infraStatus.Node[j].Id
	})

	statusFlag := []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	statusFlagStr := []string{model.StatusFailed, model.StatusSuspended, model.StatusRunning, model.StatusTerminated, model.StatusCreating, model.StatusSuspending, model.StatusResuming, model.StatusRebooting, model.StatusTerminating, model.StatusRegistering, model.StatusUndefined, model.StatusReconciling}
	for _, v := range infraStatus.Node {

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
		case model.StatusReconciling:
			statusFlag[11]++
		case model.StatusUndefined:
			statusFlag[10]++
			log.Debug().Msgf("Node %s in Infra %s has Undefined status (orphan candidate; run action=reconcile to rescue or action=refine to remove)", v.Id, infraId)
		default:
			statusFlag[10]++
			log.Warn().Msgf("Unexpected status (%s) found in Node %s of Infra %s", v.Status, v.Id, infraId)
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

	// Use the maximum of actual Node count and status Node count to handle race conditions during creation
	// During Infra creation, len(nodeList) might be smaller than len(infraStatus.Vm) due to timing issues
	actualNodeCount := len(nodeList)
	statusNodeCount := len(infraStatus.Node)
	nodeInfoCount := statusNodeCount

	// Check if Infra is still being created/registered to use more stable Node count calculation.
	// Note: infraTmp.Status (KV-stored) can become stale (e.g., "Creating" persisted before a
	// server crash) and is not authoritative here; rely on TargetAction/TargetStatus, which are
	// kept current by control actions (Create/Continue/Withdraw/Refine/etc.).
	isCreating := strings.Contains(infraTmp.TargetAction, model.ActionCreate) ||
		strings.Contains(infraTmp.TargetAction, model.ActionRegister) ||
		strings.Contains(infraTmp.TargetAction, model.ActionReconcile) ||
		strings.Contains(infraTmp.TargetStatus, model.StatusRunning)

	// Check if Infra is in a stable state (all Nodes have same stable status)
	isStableState := tmpMax == statusNodeCount && tmpMax > 0
	// stableStatusName := ""
	// if isStableState && tmpMaxIndex < len(statusFlagStr) {
	// 	stableStatusName = statusFlagStr[tmpMaxIndex]
	// }

	var numNode int
	if isCreating {
		// During creation, use the larger of the two counts to avoid showing decreasing Node counts
		numNode = max(statusNodeCount, actualNodeCount)
		// Additionally, ensure we don't show a Node count smaller than the previous maximum
		if numNode < infraStatus.StatusCount.CountTotal && infraStatus.StatusCount.CountTotal > 0 {
			numNode = infraStatus.StatusCount.CountTotal
		}

		// If we still have inconsistent counts, use the Infra's stored Node information as fallback
		if len(infraTmp.Node) > numNode {
			numNode = len(infraTmp.Node)
		}
	} else if isStableState {
		// For stable Infra states (all Nodes in same state), use the most reliable source to avoid count fluctuation
		// This applies to Terminated, Suspended, Failed, Running, etc.
		// Use the maximum of available counts, prioritizing nodeInfos as they are stored persistently
		numNode = max(actualNodeCount, nodeInfoCount)
		if len(infraTmp.Node) > numNode {
			numNode = len(infraTmp.Node)
		}
		// Ensure we don't show a count smaller than the actual Nodes found in dominant status
		if tmpMax > numNode {
			numNode = tmpMax
		}
	} else {
		// Infra creation completed, use actual Node count from status
		numNode = statusNodeCount
	}

	//numUnNormalStatus := statusFlag[0] + statusFlag[9]
	//numNormalStatus := numNode - numUnNormalStatus
	runningStatus := statusFlag[2]

	proportionStr := ":" + strconv.Itoa(tmpMax) + " (R:" + strconv.Itoa(runningStatus) + "/" + strconv.Itoa(numNode) + ")"
	if tmpMax == numNode {
		infraStatus.Status = statusFlagStr[tmpMaxIndex] + proportionStr
	} else if tmpMax < numNode {
		infraStatus.Status = "Partial-" + statusFlagStr[tmpMaxIndex] + proportionStr
	} else {
		infraStatus.Status = statusFlagStr[9] + proportionStr
	}
	// // for representing Failed status in front.

	// proportionStr = ":" + strconv.Itoa(statusFlag[0]) + " (R:" + strconv.Itoa(runningStatus) + "/" + strconv.Itoa(numNode) + ")"
	// if statusFlag[0] > 0 {
	// 	infraStatus.Status = "Partial-" + statusFlagStr[0] + proportionStr
	// 	if statusFlag[0] == numNode {
	// 		infraStatus.Status = statusFlagStr[0] + proportionStr
	// 	}
	// }

	// proportionStr = "-(" + strconv.Itoa(statusFlag[9]) + "/" + strconv.Itoa(numNode) + ")"
	// if statusFlag[9] > 0 {
	// 	infraStatus.Status = statusFlagStr[9] + proportionStr
	// }

	// Set infraStatus.StatusCount
	infraStatus.StatusCount.CountTotal = numNode
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
	infraStatus.StatusCount.CountReconciling = statusFlag[11]

	// Recovery/fallback handling for TargetAction completion
	// Primary completion should happen in actual control actions (control.go, provisioning.go)
	// This serves as a safety net for cases where the primary completion was missed
	isDone := true
	pendingNodesCount := 0

	// Re-read the infra object immediately before the recovery check to get the latest TargetAction.
	// GetInfraStatus is a slow function (fetches CSP status for all nodes); the primary completion
	// path in provisioning.go may have written TargetAction=Complete to the KV store while CSP
	// polling was in progress. Without this re-read, a stale in-memory TargetAction=Create would
	// cause a false recovery-path trigger (TOCTOU race condition).
	if freshKeyValue, freshExists, freshErr := kvstore.GetKv(key); freshErr == nil && freshExists {
		var freshInfraTmp model.InfraInfo
		if jsonErr := json.Unmarshal([]byte(freshKeyValue.Value), &freshInfraTmp); jsonErr == nil {
			infraTmp.TargetAction = freshInfraTmp.TargetAction
			infraTmp.TargetStatus = freshInfraTmp.TargetStatus
		}
	}

	// Check Infra target action to determine completion criteria
	infraTargetAction := infraTmp.TargetAction

	// Only perform recovery completion if TargetAction is not already Complete
	if infraTargetAction != model.ActionComplete && infraTargetAction != "" {
		for _, v := range infraStatus.Node {
			// Check completion based on action type
			switch infraTargetAction {
			case model.ActionCreate, model.ActionRegister, model.ActionReconcile:
				// Final states: Running, Failed, Terminated, Suspended, Undefined.
				// Undefined means the creation attempt ended without VM identity (Spider 500);
				// it is an orphan candidate handled by action=reconcile, not a pending state.
				// Register/Reconcile (discovery-type) share this shape: done once a node
				// leaves its operational transient state, whatever the observed end state.
				if v.Status == model.StatusCreating || v.Status == model.StatusRegistering ||
					v.Status == model.StatusReconciling || v.Status == "" {
					isDone = false
					pendingNodesCount++
				}

			case model.ActionTerminate:
				// For Terminate action, completion means all Nodes reach Terminated state or non-recoverable states
				// Failed, Undefined, empty states are also considered "complete" as they can't proceed further
				if v.Status != model.StatusTerminated && v.Status != model.StatusFailed &&
					v.Status != model.StatusUndefined && v.Status != "" {
					isDone = false
					pendingNodesCount++
				}

			case model.ActionSuspend:
				// For Suspend action, completion means all Nodes reach Suspended state or non-recoverable states
				// Failed, Terminated, Undefined, empty states are considered "complete"
				if v.Status != model.StatusSuspended && v.Status != model.StatusFailed &&
					v.Status != model.StatusTerminated && v.Status != model.StatusUndefined && v.Status != "" {
					isDone = false
					pendingNodesCount++
				}

			case model.ActionResume:
				// For Resume action, completion means all Nodes reach Running state or non-recoverable states
				// Failed, Terminated, Undefined, empty states are considered "complete"
				if v.Status != model.StatusRunning && v.Status != model.StatusFailed &&
					v.Status != model.StatusTerminated && v.Status != model.StatusUndefined && v.Status != "" {
					isDone = false
					pendingNodesCount++
				}

			case model.ActionReboot:
				// For Reboot action, completion means all Nodes reach Running state or non-recoverable states
				// Failed, Terminated, Undefined, empty states are considered "complete"
				if v.Status != model.StatusRunning && v.Status != model.StatusFailed &&
					v.Status != model.StatusTerminated && v.Status != model.StatusUndefined && v.Status != "" {
					isDone = false
					pendingNodesCount++
				}

			default:
				// For unknown actions, use the existing logic
				if v.TargetStatus != model.StatusComplete {
					if v.Status != model.StatusTerminated {
						isDone = false
						pendingNodesCount++
					}
				}
			}
		}

		// Log completion status for debugging
		// log.Debug().Msgf("Infra %s %s recovery completion check: %d Nodes total, %d pending, isDone=%t",
		// 	infraId, infraTargetAction, len(infraStatus.Vm), pendingNodesCount, isDone)

		if isDone {
			log.Warn().Msgf("Infra %s action %s completed via RECOVERY PATH (primary completion in control.go/provisioning.go was missed) - Node states: %d total, %d pending",
				infraId, infraTargetAction, len(infraStatus.Node), pendingNodesCount)

			// Add more detailed logging for debugging
			statusBreakdown := make(map[string]int)
			for _, v := range infraStatus.Node {
				statusBreakdown[v.Status]++
			}
			// log.Debug().Msgf("Infra %s recovery completion - Node status breakdown: %+v", infraId, statusBreakdown)

			// Check if all Nodes are in failed state
			// If there are no Nodes, consider it as all Nodes failed for creation context
			allNodesFailed := len(infraStatus.Node) == 0
			if len(infraStatus.Node) > 0 {
				allNodesFailed = true
				for _, v := range infraStatus.Node {
					if v.Status != model.StatusFailed && v.Status != model.StatusTerminated {
						allNodesFailed = false
						break
					}
				}
			}

			// Re-fetch the freshest Infra object right before writing. The
			// per-node CSP status calls above (fetchNodeStatusesWithRateLimiting)
			// can take a while on large Infras; without this, the infraTmp
			// snapshot captured at the top of this function — before those
			// calls ran — could clobber fields the primary completion path
			// (control.go/provisioning.go), or any other concurrent writer,
			// updated in the meantime (same TOCTOU concern as the TargetAction
			// re-read above, extended to cover the actual write). Falls back
			// to infraTmp if the re-read fails.
			target := infraTmp
			if freshKeyValue, freshExists, freshErr := kvstore.GetKv(key); freshErr == nil && freshExists {
				var latest model.InfraInfo
				if jsonErr := json.Unmarshal([]byte(freshKeyValue.Value), &latest); jsonErr == nil {
					target = latest
				}
			}

			if allNodesFailed && (infraTargetAction == model.ActionCreate || isDiscoveryAction(infraTargetAction)) {
				// All Nodes failed during creation - mark Infra as Failed
				log.Error().Msgf("Infra %s: All Nodes failed during creation - setting Infra status to Failed", infraId)
				infraStatus.TargetAction = model.ActionComplete
				infraStatus.TargetStatus = model.StatusComplete // Target was to complete the creation process
				infraStatus.Status = model.StatusFailed         // Actual status is Failed due to Node failures
				target.TargetAction = model.ActionComplete
				target.TargetStatus = model.StatusComplete // Target was to complete the creation process
				target.Status = model.StatusFailed         // Actual status is Failed due to Node failures
			} else {
				// Normal completion
				infraStatus.TargetAction = model.ActionComplete
				infraStatus.TargetStatus = model.StatusComplete
				target.TargetAction = model.ActionComplete
				target.TargetStatus = model.StatusComplete
			}

			target.StatusCount = infraStatus.StatusCount
			UpdateInfraInfo(nsId, target)
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

// GetNodeCurrentPublicIp is func to get Node public IP
func GetNodeCurrentPublicIp(nsId string, infraId string, nodeId string) (model.NodeStatusInfo, error) {
	errorInfo := model.NodeStatusInfo{}
	errorInfo.Status = model.StatusFailed

	temp, err := GetNodeObject(nsId, infraId, nodeId) // to check if the VM exists
	if err != nil {
		log.Error().Err(err).Msg("")
		return errorInfo, err
	}

	cspResourceName := temp.CspResourceName
	if cspResourceName == "" {
		err = fmt.Errorf("cspResourceName is empty (NodeId: %s)", nodeId)
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
		clientManager.AccessInfoDuration,
	)

	if err != nil {
		log.Trace().Err(err).Msg("")
		return errorInfo, err
	}

	nodeStatusTmp := model.NodeStatusInfo{}
	nodeStatusTmp.PublicIp = callResult.PublicIP
	nodeStatusTmp.PrivateIp = callResult.PrivateIP
	// Convert port string from Spider to int
	if portStr, err := TrimIP(callResult.SSHAccessPoint); err == nil {
		if port, err := strconv.Atoi(portStr); err == nil {
			nodeStatusTmp.SSHPort = port
		}
	}

	return nodeStatusTmp, nil
}

// GetNodeIp is func to get Node IP to return PublicIP, PrivateIP, SSHPort
func GetNodeIp(nsId string, infraId string, nodeId string) (string, string, int, error) {

	nodeObject, err := GetNodeObject(nsId, infraId, nodeId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", "", 0, err
	}

	return nodeObject.PublicIP, nodeObject.PrivateIP, nodeObject.SSHPort, nil
}

// GetNodeSpecId is func to get Node SpecId
func GetNodeSpecId(nsId string, infraId string, nodeId string) string {

	var content struct {
		SpecId string `json:"specId"`
	}

	log.Debug().Msg("[getNodeSpecID]" + nodeId)
	key := common.GenInfraKey(nsId, infraId, nodeId)

	keyValue, _, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("In GetNodeSpecId(); kvstore.GetKv() returned an error.")
		log.Error().Err(err).Msg("")
		// return nil, err
	}

	json.Unmarshal([]byte(keyValue.Value), &content)

	fmt.Printf("%+v\n", content.SpecId)

	return content.SpecId
}

// getRateLimitsForCSP returns rate limiting configuration for Node status fetching
// for a specific CSP, using the centralized configuration in csp package.
func getRateLimitsForCSP(cspName string) (int, int) {
	config := csp.GetRateLimitConfig(cspName)
	return config.MaxConcurrentRegionsForStatus, config.MaxNodesPerRegionForStatus
}

// NodeGroupStatusInfo represents Node grouping information for rate limiting
type NodeGroupStatusInfo struct {
	NodeId       string
	ProviderName string
	RegionName   string
}

// fetchNodeStatusesWithRateLimiting fetches Node statuses with hierarchical rate limiting
// Level 1: CSPs are processed in parallel
// Level 2: Within each CSP, regions are processed with semaphore (maxConcurrentRegionsPerCSP)
// Level 3: Within each region, Nodes are processed with semaphore (maxConcurrentNodesPerRegion)
// maxConcurrentSpiderCalls bounds the total number of concurrent Spider vmstatus
// HTTP calls across all CSPs and regions. Each Spider call holds an HTTP response
// buffer (~50 KB); at 1300 nodes this would otherwise allocate ~65 MB just in
// buffers, plus goroutine stacks, pushing the process past its memory limit.
const maxConcurrentSpiderCalls = 50

// globalSpiderSem is a process-wide semaphore for Spider status calls.
// Declared at package level so it is shared across concurrent infra status polls.
var globalSpiderSem = make(chan struct{}, maxConcurrentSpiderCalls)

// terminatingFailStreak counts consecutive Spider poll failures for Terminating nodes.
// When the streak reaches terminatingFailStreakMax across successive polling cycles,
// the node is promoted to Terminated — avoiding indefinite stalls when the VM is
// already gone from the CSP but Spider consistently returns errors.
// Key format: "nsId/infraId/nodeId"
var terminatingFailStreak sync.Map

// terminatingReTerminateSent tracks nodes for which a mid-streak re-terminate has
// already been fired, preventing duplicate background re-terminate goroutines.
// Key format: "nsId/infraId/nodeId"
var terminatingReTerminateSent sync.Map

// terminatingFailStreakMax is set to 10 (≈ 2.5 min at 15 s poll interval).
// A value of 3 was too small for CSPs with unstable APIs (e.g. Alibaba):
// a single successful Spider response in between resets the streak to zero,
// making it nearly impossible to ever reach the threshold when errors and
// successes alternate. 10 gives enough headroom while still auto-resolving
// within a few minutes once the VM is confirmed gone at the CSP side.
const terminatingFailStreakMax = 10

// isNodeGoneError reports whether a Spider error means the VM no longer exists at the CSP.
func isNodeGoneError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	for _, p := range []string{"does not exist", "not found", "notfound", "notexist", "no such", "could not be found", "has been deleted"} {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

// terminatingReTerminateAt is the streak count at which a background re-terminate
// is fired before the final promotion. This guards against the rare case where
// the original terminate request timed out before reaching the CSP.
// The vmstatus circuit breaker and the terminate (DELETE) circuit breaker are
// keyed separately, so re-terminate can succeed even when vmstatus is blocked.
const terminatingReTerminateAt = terminatingFailStreakMax / 2

// terminatingHoldSince records when a node with TargetAction=terminate was first seen still alive at
// the CSP. Key format: "nsId/infraId/nodeId". Time-based (not poll-count-based) so UI polling
// amplification cannot trigger re-terminates within seconds.
var terminatingHoldSince sync.Map

// terminatingHoldReTerminateAfter is how long a live status may persist before terminate is re-issued.
const terminatingHoldReTerminateAfter = 4 * time.Minute

// trackTerminatingHold re-issues terminate when the CSP keeps reporting a live status long after the
// terminate was dispatched (e.g. the DELETE never reached the CSP).
func trackTerminatingHold(nsId, infraId, nodeId, cspStatus string) {
	key := nsId + "/" + infraId + "/" + nodeId
	first, _ := terminatingHoldSince.LoadOrStore(key, time.Now())
	held := time.Since(first.(time.Time))
	if held < terminatingHoldReTerminateAfter {
		return
	}
	terminatingHoldSince.Store(key, time.Now())
	fireReTerminate(nsId, infraId, nodeId, fmt.Sprintf("CSP still reports %s %s after terminate", cspStatus, held.Round(time.Second)))
}

// reTerminateMinInterval bounds how often a background re-terminate may fire per node, and
// reTerminateSem bounds how many run at once — status polls can repeat every few seconds.
const reTerminateMinInterval = 3 * time.Minute

var (
	reTerminateLastSent sync.Map // "nsId/infraId/nodeId" -> time.Time
	reTerminateSem      = make(chan struct{}, 20)
)

// fireReTerminate re-issues terminate for a node unless one was sent recently or too many are in flight.
func fireReTerminate(nsId, infraId, nodeId, reason string) {
	key := nsId + "/" + infraId + "/" + nodeId
	if last, ok := reTerminateLastSent.Load(key); ok && time.Since(last.(time.Time)) < reTerminateMinInterval {
		return
	}
	select {
	case reTerminateSem <- struct{}{}:
	default:
		return // too many in flight; the next poll round retries
	}
	reTerminateLastSent.Store(key, time.Now())
	log.Info().Msgf("[FetchNodeStatus] Node %s: %s — re-issuing terminate to ensure CSP delivery", nodeId, reason)
	go func() {
		defer func() { <-reTerminateSem }()
		if _, rtErr := HandleInfraNodeAction(nsId, infraId, nodeId, model.ActionTerminate, true); rtErr != nil {
			log.Warn().Err(rtErr).Msgf("[FetchNodeStatus] Node %s: background re-terminate failed", nodeId)
		}
	}()
}

// trackTerminatingPollFailure advances the poll-failure streak of a Terminating node,
// re-issues terminate at terminatingReTerminateAt, and re-arms at terminatingFailStreakMax.
// It never promotes the node: a transport error cannot confirm the VM is gone.
func trackTerminatingPollFailure(nsId, infraId, nodeId, source string) {
	streakKey := nsId + "/" + infraId + "/" + nodeId
	prev, _ := terminatingFailStreak.LoadOrStore(streakKey, 0)
	streak := prev.(int) + 1
	if streak == terminatingReTerminateAt {
		fireReTerminate(nsId, infraId, nodeId, fmt.Sprintf("%s streak=%d", source, streak))
	}
	if streak >= terminatingFailStreakMax {
		terminatingFailStreak.Delete(streakKey)
		log.Debug().Msgf("[FetchNodeStatus] Node %s: %d consecutive %s errors (Terminating); CSP state unknown, keeping Terminating", nodeId, streak, source)
		return
	}
	terminatingFailStreak.Store(streakKey, streak)
}

// suspendResumeRebootFailStreak counts consecutive failed status polls for a
// node while it is Suspending, Resuming, or Rebooting. Without this, a node
// whose CSP/driver can never successfully report a recognized status (e.g. an
// unmapped native status string, or a persistently erroring vmstatus call)
// stays in the transitional status forever: the TargetAction-correction logic
// further down in FetchNodeStatus forces any status that isn't the confirmed
// target back to the transitional one on every single poll, with no way out.
// Reaching suspendResumeRebootFailStreakMax forces the node to Undefined
// instead of the requested target status — a poll failure alone can't confirm
// the VM actually reached Suspended/Running, so this surfaces the node for
// operator review (action=reconcile) rather than reporting an unverified
// state. Kept separate from terminatingFailStreak above: Terminating's give-up
// target (Terminated) and recovery nudge (re-issue terminate) don't apply here.
// Key format: "nsId/infraId/nodeId".
var suspendResumeRebootFailStreak sync.Map

// suspendResumeRebootFailStreakMax mirrors terminatingFailStreakMax's reasoning
// (~2.5 min at the 15s poll interval; a smaller value trips too easily on CSPs
// with unstable APIs where an occasional successful poll resets the streak).
const suspendResumeRebootFailStreakMax = 10

// recordSuspendResumeRebootFailure tracks one failed status poll for nodeId
// while currentStatus is Suspending, Resuming, or Rebooting. It returns
// model.StatusUndefined once suspendResumeRebootFailStreakMax consecutive
// failures have accumulated, or "" if the node should keep waiting (or
// currentStatus isn't one of the three covered here).
func recordSuspendResumeRebootFailure(nsId, infraId, nodeId, currentStatus string) string {
	switch currentStatus {
	case model.StatusSuspending, model.StatusResuming, model.StatusRebooting:
	default:
		return ""
	}

	streakKey := nsId + "/" + infraId + "/" + nodeId
	prev, _ := suspendResumeRebootFailStreak.LoadOrStore(streakKey, 0)
	streak := prev.(int) + 1
	if streak >= suspendResumeRebootFailStreakMax {
		suspendResumeRebootFailStreak.Delete(streakKey)
		log.Info().Msgf("[FetchNodeStatus] Node %s: poll failed %d consecutive times while %s; forcing to Undefined for operator review", nodeId, streak, currentStatus)
		return model.StatusUndefined
	}
	suspendResumeRebootFailStreak.Store(streakKey, streak)
	return ""
}

// resetSuspendResumeRebootFailure clears nodeId's failure streak after a
// successful status poll.
func resetSuspendResumeRebootFailure(nsId, infraId, nodeId string) {
	suspendResumeRebootFailStreak.Delete(nsId + "/" + infraId + "/" + nodeId)
}

func fetchNodeStatusesWithRateLimiting(nsId, infraId string, nodeList []string) ([]model.NodeStatusInfo, error) {
	if len(nodeList) == 0 {
		return []model.NodeStatusInfo{}, nil
	}

	// Step 1: Group Nodes by CSP and region.
	// GetNodeObject calls are parallelised (bounded semaphore) to avoid the
	// sequential ~10 s wall-clock cost for 10,000+ nodes.
	nodeGroups := make(map[string]map[string][]string) // CSP -> Region -> NodeIds

	const groupConcurrency = 50
	type groupResult struct {
		nodeId       string
		providerName string
		regionName   string
	}
	resultCh := make(chan groupResult, len(nodeList))
	groupSem := make(chan struct{}, groupConcurrency)
	var groupWg sync.WaitGroup

	for _, nodeId := range nodeList {
		groupWg.Add(1)
		go func(nodeId string) {
			defer groupWg.Done()
			groupSem <- struct{}{}
			defer func() { <-groupSem }()

			nodeInfo, err := GetNodeObject(nsId, infraId, nodeId)
			if err != nil {
				log.Warn().Err(err).Msgf("Failed to get VM object for %s, skipping", nodeId)
				return
			}
			resultCh <- groupResult{
				nodeId:       nodeId,
				providerName: nodeInfo.ConnectionConfig.ProviderName,
				regionName:   nodeInfo.Region.Region,
			}
		}(nodeId)
	}
	groupWg.Wait()
	close(resultCh)

	for r := range resultCh {
		if nodeGroups[r.providerName] == nil {
			nodeGroups[r.providerName] = make(map[string][]string)
		}
		nodeGroups[r.providerName][r.regionName] = append(nodeGroups[r.providerName][r.regionName], r.nodeId)
	}

	// Step 2: Process CSPs in parallel
	var wg sync.WaitGroup
	var mutex sync.Mutex
	var allNodeStatuses []model.NodeStatusInfo

	for csp, regions := range nodeGroups {
		wg.Add(1)
		go func(providerName string, regionMap map[string][]string) {
			defer wg.Done()

			// Get rate limits for this specific CSP
			maxRegionsForCSP, maxNodesForRegion := getRateLimitsForCSP(providerName)

			// log.Debug().Msgf("Processing CSP: %s with %d regions (limits: %d regions, %d Nodes/region)",
			// 	providerName, len(regionMap), maxRegionsForCSP, maxNodesForRegion)

			// Step 3: Process regions within CSP with rate limiting
			regionSemaphore := make(chan struct{}, maxRegionsForCSP)
			var regionWg sync.WaitGroup
			var regionMutex sync.Mutex
			var cspVmStatuses []model.NodeStatusInfo

			for region, nodeIds := range regionMap {
				regionWg.Add(1)
				go func(regionName string, nodeIdList []string) {
					defer regionWg.Done()

					// Acquire region semaphore
					regionSemaphore <- struct{}{}
					defer func() { <-regionSemaphore }()

					// log.Debug().Msgf("Processing region: %s/%s with %d Nodes (in parallel: %d Nodes/region)",
					// 	providerName, regionName, len(nodeIdList), maxNodesForRegion)

					// Step 4: Process Nodes within region with rate limiting.
					// Use the global semaphore instead of a per-region one so that the
					// total concurrent Spider calls across all regions stays bounded.
					_ = maxNodesForRegion // per-region limit superseded by globalSpiderSem
					var nodeWg sync.WaitGroup
					var nodeMutex sync.Mutex
					var regionNodeStatuses []model.NodeStatusInfo

					for _, nodeId := range nodeIdList {
						nodeWg.Add(1)
						go func(nodeId string) {
							defer nodeWg.Done()

							// Fetch Node status — uses StatusStore if fresh, falls back to Spider
							nodeStatusTmp, err := fetchNodeStatusWithCache(nsId, infraId, nodeId)
							if err != nil {
								if isTransientNetworkError(err) {
									log.Warn().Err(err).Msgf("[fetchNodeStatuses] node %s: transient error fetching status; skipping this cycle", nodeId)
								} else {
									// Node may have been deleted concurrently (e.g., by DelInfra).
									log.Debug().Err(err).Msgf("[fetchNodeStatuses] node %s not found; skipping", nodeId)
								}
								return
							}

							if nodeStatusTmp != (model.NodeStatusInfo{}) {
								nodeMutex.Lock()
								regionNodeStatuses = append(regionNodeStatuses, nodeStatusTmp)
								nodeMutex.Unlock()
							}
						}(nodeId)
					}
					nodeWg.Wait()

					// Merge region results to CSP results
					regionMutex.Lock()
					cspVmStatuses = append(cspVmStatuses, regionNodeStatuses...)
					regionMutex.Unlock()

				}(region, nodeIds)
			}
			regionWg.Wait()

			// Merge CSP results to global results
			mutex.Lock()
			allNodeStatuses = append(allNodeStatuses, cspVmStatuses...)
			mutex.Unlock()

			// log.Debug().Msgf("Completed CSP: %s, processed %d Nodes", providerName, len(cspVmStatuses))

		}(csp, regions)
	}

	wg.Wait()

	// // Summary logging
	// cspCount := len(nodeGroups)
	// totalRegions := 0
	// for _, regions := range nodeGroups {
	// 	totalRegions += len(regions)
	// }

	// log.Debug().Msgf("Rate-limited Node status fetch completed: %d CSPs, %d regions, %d Nodes processed",
	// 	cspCount, totalRegions, len(allNodeStatuses))
	return allNodeStatuses, nil
}

// // FetchNodeStatusAsync is func to get Node status async
// func FetchNodeStatusAsync(wg *sync.WaitGroup, nsId string, infraId string, nodeId string, results *model.InfraStatusInfo) error {
// 	defer wg.Done() //goroutine sync done

// 	if nsId != "" && infraId != "" && nodeId != "" {
// 		nodeStatusTmp, err := FetchNodeStatus(nsId, infraId, nodeId)
// 		if err != nil {
// 			log.Error().Err(err).Msg("")
// 			nodeStatusTmp.Status = model.StatusFailed
// 			nodeStatusTmp.SystemMessage = err.Error()
// 		}
// 		if nodeStatusTmp != (model.NodeStatusInfo{}) {
// 			results.Vm = append(results.Vm, nodeStatusTmp)
// 		}
// 	}
// 	return nil
// }

// populateNodeStatusInfoFromNodeInfo fills NodeStatusInfo with data from NodeInfo
// This is a helper function to avoid code duplication in FetchNodeStatus
func populateNodeStatusInfoFromNodeInfo(statusInfo *model.NodeStatusInfo, nodeInfo model.NodeInfo) {
	statusInfo.Id = nodeInfo.Id
	statusInfo.Name = nodeInfo.Name
	statusInfo.CspResourceName = nodeInfo.CspResourceName
	statusInfo.PublicIp = nodeInfo.PublicIP
	statusInfo.SSHPort = nodeInfo.SSHPort
	statusInfo.PrivateIp = nodeInfo.PrivateIP
	statusInfo.Status = nodeInfo.Status
	statusInfo.TargetAction = nodeInfo.TargetAction
	statusInfo.TargetStatus = nodeInfo.TargetStatus
	statusInfo.Location = nodeInfo.Location
	statusInfo.MonAgentStatus = nodeInfo.MonAgentStatus
	statusInfo.CreatedTime = nodeInfo.CreatedTime
	statusInfo.SystemMessage = nodeInfo.SystemMessage
}

// FetchNodeStatus is func to fetch Node status (call to CSPs)
func FetchNodeStatus(nsId string, infraId string, nodeId string) (model.NodeStatusInfo, error) {

	statusInfo := model.NodeStatusInfo{}

	nodeInfo, err := GetNodeObject(nsId, infraId, nodeId)
	if err != nil {
		// Debug-level: a concurrent DelInfra may delete the node between the
		// StatusAgent dispatch and the FetchNodeStatus call — this is a benign race.
		log.Debug().Err(err).Str("nodeId", nodeId).Msg("[FetchNodeStatus] node not found (likely deleted concurrently)")
		return statusInfo, err
	}

	// log.Debug().Msgf("[FetchNodeStatus] Node - Initial state from DB: Status=%s, TargetAction=%s, TargetStatus=%s, ConnectionName=%s",
	// 	nodeId, nodeInfo.Status, nodeInfo.TargetAction, nodeInfo.TargetStatus, nodeInfo.ConnectionName)

	// Check if we should skip CSP API call based on Node state
	// Skip API calls for stable final states or when CSP resource doesn't exist
	shouldSkipCSPCall := false

	// Define stable states that don't require frequent CSP API calls
	// These states are relatively stable and don't change frequently
	stableStates := map[string]bool{
		model.StatusTerminated: true,
		model.StatusFailed:     true,
		model.StatusSuspended:  true, // Suspended Nodes are stable until explicitly resumed
	}

	// Skip CSP API call for stable states ONLY if there's no active action in progress
	// If TargetAction is set (Resume, Reboot, etc.), we must fetch from CSP to track progress
	if stableStates[nodeInfo.Status] && nodeInfo.TargetAction == model.ActionComplete {
		shouldSkipCSPCall = true
	}

	// Skip CSP API call if cspResourceName is empty (Node not properly created).
	// Create and discovery-type actions (Register/Reconcile) are exempt: their nodes
	// may legitimately have an empty cspResourceName while still resolving.
	if nodeInfo.CspResourceName == "" && nodeInfo.TargetAction != model.ActionCreate &&
		!isDiscoveryAction(nodeInfo.TargetAction) {
		// A Terminate action on a node that was never created at CSP (empty cspResourceName)
		// has nothing to terminate — promote to Terminated immediately so the infra can proceed.
		if nodeInfo.TargetAction == model.ActionTerminate {
			nodeInfo.Status = model.StatusTerminated
			nodeInfo.TargetAction = model.ActionComplete
			nodeInfo.TargetStatus = model.StatusTerminated
			nodeInfo.SystemMessage = "terminated (VM was never created at CSP)"
			UpdateNodeInfo(nsId, infraId, nodeInfo)
			log.Info().Str("nodeId", nodeId).Msg("[FetchNodeStatus] never-created node promoted to Terminated (no CSP resource to terminate)")
		}
		shouldSkipCSPCall = true
	}

	if shouldSkipCSPCall {
		// Return complete status info using stored Node info
		populateNodeStatusInfoFromNodeInfo(&statusInfo, nodeInfo)
		statusInfo.NativeStatus = nodeInfo.Status
		writeStatusToStore(nsId, infraId, nodeId, statusInfo, nodeInfo)
		return statusInfo, nil
	}

	populateNodeStatusInfoFromNodeInfo(&statusInfo, nodeInfo)
	statusInfo.NativeStatus = model.StatusUndefined

	cspResourceName := nodeInfo.CspResourceName

	if (nodeInfo.TargetAction != model.ActionCreate && nodeInfo.TargetAction != model.ActionTerminate &&
		!isDiscoveryAction(nodeInfo.TargetAction)) && cspResourceName == "" {
		err = fmt.Errorf("cspResourceName is empty (NodeId: %s)", nodeId)
		log.Error().Err(err).Msg("")
		return statusInfo, err
	}

	type statusResponse struct {
		Status string
	}
	callResult := statusResponse{}
	callResult.Status = ""
	// pollFailed marks that this poll ended in an error (direct SDK or Spider),
	// as opposed to a successful call that simply returned an unrecognized
	// status. Checked after the TargetAction-correction blocks below to drive
	// the Suspend/Resume/Reboot circuit breaker (see suspendResumeRebootFailStreak).
	pollFailed := false

	// Enter on a CspResourceId even when cspResourceName is empty: a register/discovery
	// target on a batch-capable CSP resolves via the direct SDK by CspResourceId, so it
	// must reach the SDK path below rather than being skipped (the Spider sub-path, which
	// needs cspResourceName, is guarded separately).
	if nodeInfo.Status != model.StatusTerminated && (cspResourceName != "" || nodeInfo.CspResourceId != "") {
		// Direct SDK fast path: bypass Spider for CSPs with a registered BatchVMStatusFunc.
		// Benefits: connection pooling, cached OAuth tokens, no extra Spider network hop.
		// On failure we do NOT fall back to Spider: Spider calls the same CSP API, so a
		// transient CSP failure that breaks our SDK would break Spider too — adding only the
		// Spider round-trip latency (up to 60 s) with no reliability gain.
		if handler, ok := cspdirect.GetBatchVMStatusHandler(nodeInfo.ConnectionConfig.ProviderName); ok && nodeInfo.CspResourceId != "" {
			sdkCtx := context.WithValue(context.Background(), model.CtxKeyCredentialHolder, nodeInfo.ConnectionConfig.CredentialHolder)
			// Shared per-region cache: N node polls within BatchStatusCacheTTL cost one batch call.
			s, found, sdkErr := cspdirect.BatchVMStatusCached(sdkCtx, nodeInfo.ConnectionConfig.ProviderName,
				nodeInfo.ConnectionConfig.RegionDetail.RegionName, handler, nodeInfo.CspResourceId)
			if sdkErr == nil {
				sdkStreakKey := nsId + "/" + infraId + "/" + nodeId
				if found {
					callResult.Status = s
					terminatingFailStreak.Delete(sdkStreakKey)
					terminatingReTerminateSent.Delete(sdkStreakKey)
					resetBatchNotFound(nsId, infraId, nodeId)
					goto applyStatus
				}
				// Missing from a successful batch response: the CSP instance is gone. Advance
				// the shared not-found streak and settle as Terminated once reached — uniform
				// with the BatchSweeper, so the two paths agree instead of oscillating.
				terminatingFailStreak.Delete(sdkStreakKey)
				callResult.Status = recordBatchNotFound(nsId, infraId, nodeId)
				goto applyStatus
			}

			// Direct SDK failed — preserve stable status to avoid false flips, same logic
			// as Spider error handling.
			pollFailed = true
			log.Warn().Err(sdkErr).Str("provider", nodeInfo.ConnectionConfig.ProviderName).
				Msgf("[FetchNodeStatus] direct SDK failed for %s; preserving current status (skipping Spider fallback)", nodeId)
			switch nodeInfo.Status {
			case model.StatusRunning, model.StatusSuspended, model.StatusTerminated,
				model.StatusSuspending, model.StatusResuming, model.StatusRebooting,
				model.StatusTerminating:
				callResult.Status = nodeInfo.Status
			default:
				callResult.Status = model.StatusUndefined
			}
			// A direct SDK transport error says nothing about CSP state: a gone VM shows up
			// as a successful batch response without the id (handled above). Never promote
			// on errors; re-issue terminate mid-streak and re-arm at the max.
			if nodeInfo.Status == model.StatusTerminating {
				trackTerminatingPollFailure(nsId, infraId, nodeId, "direct SDK")
			}
			goto applyStatus
		}

		// The Spider sub-path needs a cspResourceName. A node that reached here on a
		// CspResourceId alone (batch-capable CSPs handled above; non-batch CSPs have no
		// direct SDK) has nothing to query via Spider — leave the status as-is.
		if cspResourceName == "" {
			goto applyStatus
		}

		// Rate-limit all Spider HTTP calls process-wide regardless of call path
		// (StatusAgent workers, reconcile goroutines, direct callers all share the cap).
		globalSpiderSem <- struct{}{}
		defer func() { <-globalSpiderSem }()

		client := clientManager.NewHttpClient()
		url := model.SpiderRestUrl + "/vmstatus/" + cspResourceName
		method := "GET"
		client.SetTimeout(60 * time.Second)

		type VMStatusReqInfo struct {
			ConnectionName string
		}
		requestBody := VMStatusReqInfo{}
		requestBody.ConnectionName = nodeInfo.ConnectionName

		// log.Debug().Msgf("[FetchNodeStatus] Node: Calling CB-Spider API - URL: %s, ConnectionName: %s",
		// 	nodeId, url, nodeInfo.ConnectionName)

		// Retry to get right Node status from cb-spider. Sometimes cb-spider returns not approriate status.
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

			// log.Debug().Msgf("[FetchNodeStatus] Node: CB-Spider response (attempt %d/%d) - Status: %s, Error: %v",
			// 	nodeId, i+1, retrycheck, callResult.Status, err)

			if err != nil {
				pollFailed = true
				statusInfo.SystemMessage = err.Error()
				log.Warn().Err(err).Msgf("[FetchNodeStatus] Node %s: Spider error (current status: %s); preserving stable status to avoid false Undefined flip", nodeId, nodeInfo.Status)

				// On transient errors (connection reset, timeout), preserve stable statuses.
				// Running/Suspended nodes are left as-is; the next successful poll will catch
				// real state changes (e.g. spot-instance reclaim).
				// Creating/Undefined stay Undefined since they're already uncertain.
				switch nodeInfo.Status {
				case model.StatusRunning, model.StatusSuspended, model.StatusTerminated,
					model.StatusSuspending, model.StatusResuming, model.StatusRebooting,
					model.StatusTerminating:
					callResult.Status = nodeInfo.Status
				default:
					callResult.Status = model.StatusUndefined
				}

				// For Terminating nodes, track consecutive poll failures across cycles.
				// A single transient Spider error should not flip status; but if Spider
				// consistently cannot find the VM over multiple polling cycles, the VM
				// is almost certainly gone from the CSP.
				if nodeInfo.Status == model.StatusTerminating {
					streakKey := nsId + "/" + infraId + "/" + nodeId
					prev, _ := terminatingFailStreak.LoadOrStore(streakKey, 0)
					streak := prev.(int) + 1

					// Mid-streak: re-issue the terminate request to guard against the rare
					// case where the original terminate timed out before reaching the CSP.
					// The terminate (DELETE) endpoint has its own circuit-breaker key,
					// independent of the vmstatus (GET) circuit-breaker, so this re-send
					// can succeed even when vmstatus polling is blocked.
					// Runs in a goroutine to avoid blocking the current status-poll cycle.
					if streak == terminatingReTerminateAt {
						fireReTerminate(nsId, infraId, nodeId, fmt.Sprintf("Spider streak=%d", streak))
					}

					if streak >= terminatingFailStreakMax {
						if isNodeGoneError(err) {
							terminatingFailStreak.Delete(streakKey)
							terminatingReTerminateSent.Delete(streakKey)
							log.Info().Msgf("[FetchNodeStatus] Node %s: Spider error for %d consecutive polls (Terminating); promoting to Terminated", nodeId, streak)
							callResult.Status = model.StatusTerminated
						} else {
							// Network/DNS failures say nothing about CSP state: keep Terminating and
							// restart the streak so the re-terminate fires again on the next round.
							terminatingFailStreak.Delete(streakKey)
							terminatingReTerminateSent.Delete(streakKey)
							log.Warn().Msgf("[FetchNodeStatus] Node %s: %d consecutive non-not-found Spider errors (Terminating); CSP state unknown, keeping Terminating and re-arming re-terminate", nodeId, streak)
						}
					} else {
						terminatingFailStreak.Store(streakKey, streak)
					}
				}
				break
			}
			if callResult.Status != "" {
				// Successful Spider response: reset consecutive failure streak for this node.
				streakKey := nsId + "/" + infraId + "/" + nodeId
				terminatingFailStreak.Delete(streakKey)
				terminatingReTerminateSent.Delete(streakKey)
				break
			}
			time.Sleep(5 * time.Second)
		}

	} else {
		callResult.Status = model.StatusUndefined
	}

applyStatus:
	nativeStatus := callResult.Status

	// log.Debug().Msgf("[FetchNodeStatus] VM %s: Raw NativeStatus from CSP: %s", nodeId, nativeStatus)

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
		// log.Debug().Msgf("[FetchNodeStatus] VM %s: NativeStatus '%s' is not valid, setting to Undefined", nodeId, nativeStatus)
		callResult.Status = model.StatusUndefined
	}

	nodeInfo, err = GetNodeObject(nsId, infraId, nodeId)
	if err != nil {
		log.Err(err).Msg("")
		return statusInfo, err
	}
	nodeStatusTmp := model.NodeStatusInfo{}
	nodeStatusTmp.Id = nodeInfo.Id
	nodeStatusTmp.Name = nodeInfo.Name
	nodeStatusTmp.CspResourceName = nodeInfo.CspResourceName
	nodeStatusTmp.Status = nodeInfo.Status // Set the current status first
	nodeStatusTmp.PrivateIp = nodeInfo.PrivateIP
	nodeStatusTmp.NativeStatus = nativeStatus
	nodeStatusTmp.TargetAction = nodeInfo.TargetAction
	nodeStatusTmp.TargetStatus = nodeInfo.TargetStatus
	nodeStatusTmp.Location = nodeInfo.Location
	nodeStatusTmp.MonAgentStatus = nodeInfo.MonAgentStatus
	nodeStatusTmp.CreatedTime = nodeInfo.CreatedTime
	nodeStatusTmp.SystemMessage = nodeInfo.SystemMessage

	// log.Debug().Msgf("[FetchNodeStatus] Node: Before TargetAction correction - Status=%s, NativeStatus=%s, TargetAction=%s, TargetStatus=%s",
	// 	nodeId, nodeStatusTmp.Status, nodeStatusTmp.NativeStatus, nodeStatusTmp.TargetAction, nodeStatusTmp.TargetStatus)

	//Correct undefined status using TargetAction
	if strings.EqualFold(nodeStatusTmp.TargetAction, model.ActionCreate) {
		if strings.EqualFold(callResult.Status, model.StatusUndefined) {
			callResult.Status = model.StatusCreating
		}
		if strings.EqualFold(nodeInfo.Status, model.StatusFailed) {
			callResult.Status = model.StatusFailed
		}
	}

	// Discovery-type actions (Register/Reconcile): while the CSP state is not yet
	// resolved (Undefined), hold the operational status (Registering/Reconciling)
	// instead of flipping to Creating. The actual state is late-bound below once a
	// recognized status is observed.
	if isDiscoveryAction(nodeStatusTmp.TargetAction) &&
		strings.EqualFold(callResult.Status, model.StatusUndefined) {
		callResult.Status = discoveryTransientStatus(nodeStatusTmp.TargetAction)
	}

	// Fallback: if the CSP (or Spider) returned Undefined but the node already has a
	// PublicIP, the CSP committed to creating the VM — preserve Creating to avoid a
	// false Undefined flip while the VM is still booting or the CSP API is slow.
	// This covers two gaps the TargetAction correction above cannot handle:
	//   1. TargetAction was already cleared to ActionComplete (e.g. IBM: VM briefly
	//      reached Running, action completed, then vmstatus API began timing out).
	//   2. TargetAction is empty due to a concurrent goroutine clearing it before
	//      the correction above is evaluated.
	if strings.EqualFold(callResult.Status, model.StatusUndefined) &&
		strings.EqualFold(nodeInfo.Status, model.StatusCreating) &&
		nodeInfo.PublicIP != "" {
		log.Debug().Msgf("[FetchNodeStatus] Node %s: PublicIP already assigned (%s), preserving Creating despite Undefined from CSP/Spider", nodeId, nodeInfo.PublicIP)
		callResult.Status = model.StatusCreating
	}

	if strings.EqualFold(nodeStatusTmp.TargetAction, model.ActionTerminate) {
		switch {
		case strings.EqualFold(callResult.Status, model.StatusTerminated):
			// confirmed terminated — pass through
			terminatingHoldSince.Delete(nsId + "/" + infraId + "/" + nodeId)
		case strings.EqualFold(callResult.Status, model.StatusTerminating):
			// deletion in progress — pass through
			terminatingHoldSince.Delete(nsId + "/" + infraId + "/" + nodeId)
		case strings.EqualFold(callResult.Status, model.StatusSuspending):
			// stopping phase of a delete (e.g. GCP STOPPING) — in progress, no re-terminate
			terminatingHoldSince.Delete(nsId + "/" + infraId + "/" + nodeId)
			callResult.Status = model.StatusTerminating
		case strings.EqualFold(callResult.Status, model.StatusUndefined):
			// VM no longer found at CSP — treat as confirmed terminated
			terminatingHoldSince.Delete(nsId + "/" + infraId + "/" + nodeId)
			callResult.Status = model.StatusTerminated
		default:
			// CSP returned Running, Suspended, Suspending, etc.
			// Usually a transient artifact (e.g. Azure briefly reports Running after
			// DELETE is accepted) — but if the terminate never reached the CSP the node
			// would hold forever, so re-issue terminate after repeated observations.
			log.Debug().Msgf("[FetchNodeStatus] VM %s: TargetAction=terminate but CSP returned %s; holding Terminating",
				nodeId, callResult.Status)
			trackTerminatingHold(nsId, infraId, nodeId, callResult.Status)
			callResult.Status = model.StatusTerminating
		}
	}
	if strings.EqualFold(nodeStatusTmp.TargetAction, model.ActionResume) {
		if strings.EqualFold(callResult.Status, model.StatusUndefined) {
			callResult.Status = model.StatusResuming
		}
		// NCP may return Creating status during Resume operation instead of Resuming status.
		if strings.EqualFold(callResult.Status, model.StatusCreating) {
			log.Debug().Msgf("[FetchNodeStatus] VM %s: CSP returned Creating during Resume action, correcting to Resuming", nodeId)
			callResult.Status = model.StatusResuming
		}
		// Some CSPs (e.g., KT Cloud) may return Suspended status during Resume operation
		// instead of returning Resuming status. Correct it to Resuming.
		if strings.EqualFold(callResult.Status, model.StatusSuspended) {
			log.Debug().Msgf("[FetchNodeStatus] VM %s: CSP returned Suspended during Resume action, correcting to Resuming", nodeId)
			callResult.Status = model.StatusResuming
		}
	}
	// Some CSPs may return Running or Resuming status during Suspend operation instead of Suspending status.
	if strings.EqualFold(nodeStatusTmp.TargetAction, model.ActionSuspend) {
		if strings.EqualFold(callResult.Status, model.StatusUndefined) {
			callResult.Status = model.StatusSuspending
		}
		if strings.EqualFold(callResult.Status, model.StatusRunning) {
			log.Debug().Msgf("[FetchNodeStatus] VM %s: CSP returned Running during Suspend action, correcting to Suspending", nodeId)
			callResult.Status = model.StatusSuspending
		}
		// Tencent may temporarily return Resuming status during Suspend operation
		if strings.EqualFold(callResult.Status, model.StatusResuming) {
			log.Debug().Msgf("[FetchNodeStatus] VM %s: CSP returned Resuming during Suspend action, correcting to Suspending", nodeId)
			callResult.Status = model.StatusSuspending
		}
	}
	// for action reboot, some csp's native status are suspending, suspended, creating, resuming
	if strings.EqualFold(nodeStatusTmp.TargetAction, model.ActionReboot) {
		if strings.EqualFold(callResult.Status, model.StatusUndefined) {
			callResult.Status = model.StatusRebooting
		}
		if strings.EqualFold(callResult.Status, model.StatusSuspending) || strings.EqualFold(callResult.Status, model.StatusSuspended) || strings.EqualFold(callResult.Status, model.StatusCreating) || strings.EqualFold(callResult.Status, model.StatusResuming) {
			callResult.Status = model.StatusRebooting
		}
	}

	if strings.EqualFold(nodeStatusTmp.Status, model.StatusTerminated) {
		callResult.Status = model.StatusTerminated
	}

	// Circuit breaker for Suspend/Resume/Reboot: the TargetAction-correction
	// blocks above force any non-target status back to the transitional one on
	// every poll, so a node whose status can never be confirmed would otherwise
	// stay Suspending/Resuming/Rebooting forever. This covers two distinct
	// "can't confirm" cases: (1) the SDK/Spider call itself errored (pollFailed),
	// and (2) the call succeeded but returned a native status this codebase
	// doesn't recognize (not in validStatuses, e.g. a CSP-specific value or a
	// legitimate "Failed" that validStatuses doesn't include) — that case does
	// NOT set pollFailed, since the RPC succeeded, but is just as stuck-forever
	// without this check. Applied last (after the correction blocks) so a
	// give-up decision isn't immediately overwritten by them. Streak resets on
	// any poll that isn't one of these two cases.
	_, nativeStatusRecognized := validStatuses[nativeStatus]
	pollUnconfirmed := pollFailed || !nativeStatusRecognized
	if pollUnconfirmed {
		if giveUp := recordSuspendResumeRebootFailure(nsId, infraId, nodeId, callResult.Status); giveUp != "" {
			callResult.Status = giveUp
		}
	} else {
		resetSuspendResumeRebootFailure(nsId, infraId, nodeId)
	}

	// Log status change if status actually changed
	previousStatus := nodeStatusTmp.Status
	nodeStatusTmp.Status = callResult.Status
	if previousStatus != nodeStatusTmp.Status {
		log.Debug().Msgf("[FetchNodeStatus] Node %s: Status changed - %s -> %s (NativeStatus: %s, TargetAction: %s)",
			nodeId, previousStatus, nodeStatusTmp.Status, nodeStatusTmp.NativeStatus, nodeStatusTmp.TargetAction)
	}

	// TODO: Alibaba Undefined status error is not resolved yet.
	// (After Terminate action. "status": "Undefined", "targetStatus": "None", "targetAction": "None")

	// Discovery-type actions (Register/Reconcile) have no fixed target. They complete
	// once the resource resolves to a real end state:
	//   - live (Running/Suspended): late-bind TargetStatus to it so the standard
	//     finalization below (TargetStatus==Status) records the actual state.
	//   - terminated/terminating: not a manageable target, but a real end state —
	//     settle as Terminated (final, no longer re-polled) so refine removes it,
	//     rather than mislabeling a gone resource as Failed.
	if isDiscoveryAction(nodeStatusTmp.TargetAction) {
		if isStableObservedStatus(nodeStatusTmp.Status) {
			nodeStatusTmp.TargetStatus = nodeStatusTmp.Status
		} else if strings.EqualFold(nodeStatusTmp.Status, model.StatusTerminated) ||
			strings.EqualFold(nodeStatusTmp.Status, model.StatusTerminating) {
			nodeStatusTmp.Status = model.StatusTerminated
			nodeStatusTmp.TargetStatus = model.StatusComplete
			nodeStatusTmp.TargetAction = model.ActionComplete
			nodeStatusTmp.SystemMessage = "target CSP resource is terminated; cannot be managed (run refine to remove)"
		}
	}

	//if TargetStatus == CurrentStatus, record to finialize the control operation
	if nodeStatusTmp.TargetStatus == nodeStatusTmp.Status {
		if nodeStatusTmp.TargetStatus != model.StatusTerminated {
			log.Debug().Msgf("[FetchNodeStatus] Node %s: Action completed - TargetStatus(%s) reached",
				nodeId, nodeStatusTmp.TargetStatus)
			nodeStatusTmp.SystemMessage = nodeStatusTmp.TargetStatus + "==" + nodeStatusTmp.Status
			nodeStatusTmp.TargetStatus = model.StatusComplete
			nodeStatusTmp.TargetAction = model.ActionComplete

			//Get current public IP when status has been changed.
			nodeInfoTmp, err := GetNodeCurrentPublicIp(nsId, infraId, nodeInfo.Id)
			if err != nil {
				log.Error().Err(err).Msg("")
				statusInfo.SystemMessage = err.Error()
				return statusInfo, err
			}
			nodeInfo.PublicIP = nodeInfoTmp.PublicIp
			nodeInfo.SSHPort = nodeInfoTmp.SSHPort

		} else {
			// Don't init TargetStatus if the TargetStatus is model.StatusTerminated. It is to finalize Node lifecycle if model.StatusTerminated.
			nodeStatusTmp.TargetStatus = model.StatusTerminated
			nodeStatusTmp.TargetAction = model.ActionTerminate
			nodeStatusTmp.Status = model.StatusTerminated
			nodeStatusTmp.SystemMessage = "terminated VM. No action is acceptable except deletion"
		}
	}

	nodeStatusTmp.PublicIp = nodeInfo.PublicIP
	nodeStatusTmp.SSHPort = nodeInfo.SSHPort

	// Apply current status to nodeInfo only if VM is not already terminated
	// Prevent overwriting Terminated status with empty or other states
	// A concurrent DelInfra may remove the node during the (slow) CSP status call
	// above; skip the write-back so we don't act on a not-found zero object.
	originalNodeInfo, getErr := GetNodeObject(nsId, infraId, nodeId)
	if getErr == nil && originalNodeInfo.Status != model.StatusTerminated {
		nodeInfo.Status = nodeStatusTmp.Status
		nodeInfo.TargetAction = nodeStatusTmp.TargetAction
		nodeInfo.TargetStatus = nodeStatusTmp.TargetStatus
		nodeInfo.SystemMessage = nodeStatusTmp.SystemMessage

		if cspResourceName != "" {
			// Write onto the freshly read object: nodeInfo is a snapshot taken before
			// the CSP status call, so writing it wholesale would revert fields changed
			// meanwhile (e.g. DataDiskIds set by AttachDetachDataDisk)
			originalNodeInfo.Status = nodeStatusTmp.Status
			originalNodeInfo.TargetAction = nodeStatusTmp.TargetAction
			originalNodeInfo.TargetStatus = nodeStatusTmp.TargetStatus
			originalNodeInfo.SystemMessage = nodeStatusTmp.SystemMessage
			originalNodeInfo.PublicIP = nodeInfo.PublicIP
			originalNodeInfo.PrivateIP = nodeInfo.PrivateIP
			originalNodeInfo.SSHPort = nodeInfo.SSHPort
			UpdateNodeInfo(nsId, infraId, originalNodeInfo)
		}
	}
	// else: Node is already terminated, skip status update

	writeStatusToStore(nsId, infraId, nodeId, nodeStatusTmp, nodeInfo)
	return nodeStatusTmp, nil
}

// GetInfraNodeStatus is func to Get InfraNode Status with option to control CSP API fetch
func GetInfraNodeStatus(nsId string, infraId string, nodeId string, fetchFromCSP bool) (*model.NodeStatusInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := &model.NodeStatusInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = common.CheckString(infraId)
	if err != nil {
		temp := &model.NodeStatusInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = common.CheckString(nodeId)
	if err != nil {
		temp := &model.NodeStatusInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	check, _ := CheckNode(nsId, infraId, nodeId)

	if !check {
		temp := &model.NodeStatusInfo{}
		err := fmt.Errorf("The node %s does not exist.", nodeId)
		return temp, err
	}

	var nodeStatusResponse model.NodeStatusInfo

	if fetchFromCSP {
		// Fetch current status from CSP API
		nodeStatusResponse, err = FetchNodeStatus(nsId, infraId, nodeId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return nil, err
		}
	} else {
		// Use cached status from database (faster response)
		nodeObject, err := GetNodeObject(nsId, infraId, nodeId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return nil, err
		}

		// Convert NodeInfo to NodeStatusInfo
		nodeStatusResponse = model.NodeStatusInfo{
			Id:              nodeObject.Id,
			Name:            nodeObject.Name,
			CspResourceName: nodeObject.CspResourceName,
			Status:          nodeObject.Status,
			TargetStatus:    nodeObject.TargetStatus,
			TargetAction:    nodeObject.TargetAction,
			PublicIp:        nodeObject.PublicIP,
			PrivateIp:       nodeObject.PrivateIP,
			SSHPort:         nodeObject.SSHPort,
			Location:        nodeObject.Location,
			MonAgentStatus:  nodeObject.MonAgentStatus,
			CreatedTime:     nodeObject.CreatedTime,
			SystemMessage:   nodeObject.SystemMessage,
		}
	}

	return &nodeStatusResponse, nil
}

// GetInfraNodeCurrentStatus is func to Get InfraNode Current Status from CSP API (real-time)
func GetInfraNodeCurrentStatus(nsId string, infraId string, nodeId string) (*model.NodeStatusInfo, error) {
	// Simply delegate to GetInfraNodeStatus with fetchFromCSP=true
	return GetInfraNodeStatus(nsId, infraId, nodeId, true)
}
