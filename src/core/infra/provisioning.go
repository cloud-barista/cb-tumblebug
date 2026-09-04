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
	"maps"
	"math/rand"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/alibaba" // register Alibaba handlers (availability, vmstatus)
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/aws"     // register AWS handlers (vmstatus)
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/azure"   // register Azure handlers (availability, vmstatus)
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/gcp"     // register GCP handlers (vmstatus)
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/tencent" // register Tencent handlers (availability, vmstatus)
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	validator "github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"
)

// isApiThrottlingError reports whether err is a CSP API rate-limit rejection (retryable).
func isApiThrottlingError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, p := range []string{"requestlimitexceeded", "throttling", "toomanyrequests", "too many requests", "frequency limit", "reduce the frequency", "429"} {
		if strings.Contains(msg, p) {
			return true
		}
	}
	return false
}

// createSharedResourceWithRetry retries shared-resource creation on transient network errors and
// API throttling (a single IBM API timeout once dropped a whole 40-node group before any VM was
// created). A retry that finds the resource already created by the earlier attempt is a success.
func createSharedResourceWithRetry(ctx context.Context, nsId, resType, connectionName string, opts *resource.SharedResourceOptions) error {
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		err = resource.CreateSharedResourceWithOptions(ctx, nsId, resType, connectionName, opts)
		if err == nil {
			return nil
		}
		if attempt > 1 && strings.Contains(strings.ToLower(err.Error()), "already exist") {
			log.Info().Msgf("[SharedResource] %s on %s exists after a retried attempt; continuing", resType, connectionName)
			return nil
		}
		if !(isTransientNetworkError(err) || isApiThrottlingError(err)) || ctx.Err() != nil {
			return err
		}
		wait := time.Duration(10*attempt) * time.Second
		log.Warn().Err(err).Msgf("[SharedResource] transient error creating %s on %s (attempt %d/3); retrying in %s", resType, connectionName, attempt, wait)
		time.Sleep(wait)
	}
	return err
}

// createThrottleMaxAttempts bounds Spider create retries on CSP API throttling.
const createThrottleMaxAttempts = 6

// createThrottleBackoff returns an exponential backoff with jitter: ~5s, 10s, 20s, 40s, 60s (cap).
func createThrottleBackoff(attempt int) time.Duration {
	base := 5 * time.Second << uint(attempt-1)
	if base > 60*time.Second {
		base = 60 * time.Second
	}
	return base + time.Duration(rand.Intn(3000))*time.Millisecond
}

// getNodeCreateRateLimitsForCSP returns rate limiting configuration for Node creation.
// Uses centralized CSP config from csp.GetRateLimitConfig() with built-in fallback for unknown CSPs.
func getNodeCreateRateLimitsForCSP(cspName string) (int, int) {
	config := csp.GetRateLimitConfig(cspName)
	return config.MaxConcurrentRegions, config.MaxNodesPerRegion
}

// pickSuggestedSystemDisk returns the first currently-available system-disk
// category from an AvailabilityResult, or "" when no suggestion is available
// (no checker, no available zone, or no supported disk listed).
//
// NOTE: This is intended for REVIEW/DISPLAY purposes only (e.g. mapui hint).
// Do NOT auto-apply the returned value to a VM creation request: the suggestion
// is picked from the first available zone of the region, which may differ from
// the actual VM zone (vnet/subnet are bound to a representative zone during
// infra dynamic provisioning), so silently switching the disk type can still
// cause "No AvailableSystemDisk" failures and breaks the user's mental model.
func pickSuggestedSystemDisk(r model.AvailabilityResult) string {
	if !r.Available {
		return ""
	}
	for _, z := range r.Zones {
		if !z.Available {
			continue
		}
		for _, d := range z.SupportedDisks {
			if d != "" {
				return d
			}
		}
	}
	return ""
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

// createNodeObjectSafe creates Node object without WaitGroup management
func createNodeObjectSafe(nsId, infraId string, nodeInfoData *model.NodeInfo) error {
	var wg sync.WaitGroup
	wg.Add(1)
	return CreateNodeObject(&wg, nsId, infraId, nodeInfoData)
}

// // createNodeSafe creates Node without WaitGroup management
// func createNodeSafe(nsId, infraId string, nodeInfoData *model.NodeInfo, option string) error {
// 	var wg sync.WaitGroup
// 	wg.Add(1)
// 	err := CreateNode(&wg, nsId, infraId, nodeInfoData, option)
// 	wg.Wait()
// 	return err
// }

// Helper functions for CreateInfra

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	return slices.Contains(slice, item)
}

// nodeGroupNameLocks serializes name assignment per NodeGroup so two concurrent
// additions cannot reserve the same index. Single-process scope, which matches the
// single-replica deployment; the kvstore checks below still guard the rest.
var nodeGroupNameLocks sync.Map

// reserveNodeNames picks names for the Nodes about to be created and records them in
// the NodeGroup object, so a concurrent caller sees them as taken. Names follow the
// Nodes that actually exist rather than a size counter: a record that under-counts
// would name a live Node, whose record would then be overwritten and its CSP resource
// left running untracked (issue #2652).
func reserveNodeNames(nsId, infraId, nodeGroupId string, nodeRequest *model.CreateNodeGroupReq, count int, newNodeGroup bool) ([]string, error) {
	registerMode := nodeRequest.CspResourceId != ""
	mutex, _ := nodeGroupNameLocks.LoadOrStore(nsId+"/"+infraId+"/"+nodeGroupId, &sync.Mutex{})
	mutex.(*sync.Mutex).Lock()
	defer mutex.(*sync.Mutex).Unlock()

	migrateLegacyNodeGroupRecord(nsId, infraId, nodeGroupId)

	record := model.NodeGroupInfo{
		ResourceType: model.StrNodeGroup,
		Id:           nodeGroupId,
		Name:         nodeGroupId,
		Uid:          common.GenUid(),
	}
	key := common.GenInfraNodeGroupKey(nsId, infraId, nodeGroupId)
	keyValue, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Warn().Err(err).Msgf("Cannot read the NodeGroup record of %s", nodeGroupId)
	}
	if exists {
		if !newNodeGroup {
			return nil, fmt.Errorf("Duplicated NodeGroup ID")
		}
		json.Unmarshal([]byte(keyValue.Value), &record)
	} else {
		record.RootDiskType = nodeRequest.RootDiskType
		record.RootDiskSize = nodeRequest.RootDiskSize
	}

	// Names in use: the Nodes that exist plus the ones the record already reserved
	// (a reservation is recorded before its Node object exists)
	inUse := append([]string{}, record.NodeId...)
	liveNodeIds, listErr := ListNodeByNodeGroup(nsId, infraId, nodeGroupId)
	if listErr != nil {
		log.Warn().Err(listErr).Msgf("Cannot list Nodes of NodeGroup %s; relying on its record", nodeGroupId)
	}
	for _, liveNodeId := range liveNodeIds {
		if !contains(inUse, liveNodeId) {
			inUse = append(inUse, liveNodeId)
		}
	}
	startIndex := maxNodeIndex(nodeGroupId, inUse) + 1
	if fromSize := record.NodeGroupSize + 1; fromSize > startIndex {
		startIndex = fromSize
	}

	newNodeIds := []string{}
	if registerMode {
		// Register mode: one Node per registration, named after the NodeGroup
		newNodeIds = append(newNodeIds, nodeGroupId)
	} else {
		for i := startIndex; i < count+startIndex; i++ {
			newNodeIds = append(newNodeIds, nodeGroupId+"-"+strconv.Itoa(i))
		}
	}

	for _, newNodeId := range newNodeIds {
		if _, nodeExists, _ := kvstore.GetKv(common.GenInfraKey(nsId, infraId, newNodeId)); nodeExists {
			return nil, fmt.Errorf("Node %s already exists in Infra %s; aborting to avoid overwriting it and orphaning its CSP resource", newNodeId, infraId)
		}
	}

	record.NodeId = append(inUse, newNodeIds...)
	record.NodeGroupSize = len(record.NodeId)
	val, _ := json.Marshal(record)
	if err := kvstore.Put(key, string(val)); err != nil {
		return nil, fmt.Errorf("cannot store the NodeGroup record of %s: %w", nodeGroupId, err)
	}
	return newNodeIds, nil
}

// migrateLegacyNodeGroupRecord moves a record stored under a non-canonical (mixed-case)
// key to the canonical one. Such a record was unreachable from the lower-cased id that
// Nodes carry, which is one way the size bookkeeping went missing (issue #2652).
func migrateLegacyNodeGroupRecord(nsId, infraId, nodeGroupId string) {
	groupIds, err := ListNodeGroupId(nsId, infraId)
	if err != nil {
		return
	}
	canonicalKey := common.GenInfraNodeGroupKey(nsId, infraId, nodeGroupId)
	prefix := common.GenInfraKey(nsId, infraId, "") + "/" + model.StrNodeGroup + "/"
	for _, groupId := range groupIds {
		if groupId == nodeGroupId || common.ToLower(groupId) != nodeGroupId {
			continue
		}
		legacyKey := prefix + groupId
		legacyValue, exists, err := kvstore.GetKv(legacyKey)
		if err != nil || !exists {
			continue
		}
		if _, canonicalExists, _ := kvstore.GetKv(canonicalKey); !canonicalExists {
			if err := kvstore.Put(canonicalKey, legacyValue.Value); err != nil {
				continue
			}
		}
		kvstore.Delete(legacyKey)
		log.Info().Msgf("Migrated NodeGroup record %s to %s", legacyKey, canonicalKey)
	}
}

// maxNodeIndex returns the highest "<nodeGroupId>-<n>" suffix among the given Node IDs,
// or 0 when none matches. Node names are derived from it so that a new Node never takes
// the name of an existing one.
func maxNodeIndex(nodeGroupId string, nodeIds []string) int {
	highest := 0
	prefix := nodeGroupId + "-"
	for _, nodeId := range nodeIds {
		suffix, found := strings.CutPrefix(nodeId, prefix)
		if !found {
			continue
		}
		if index, err := strconv.Atoi(suffix); err == nil && index > highest {
			highest = index
		}
	}
	return highest
}

// createNodeGroup creates a nodeGroup with proper error handling
func createNodeGroup(ctx context.Context, nsId, infraId string, nodeRequest *model.CreateNodeGroupReq, nodeGroupSize, nodeStartIndex int, uid string, req *model.InfraReq) error {
	log.Info().Msgf("Creating Infra nodeGroup object for '%s'", nodeRequest.Name)
	key := common.GenInfraNodeGroupKey(nsId, infraId, nodeRequest.Name)

	nodeGroupInfoData := model.NodeGroupInfo{
		ResourceType: model.StrNodeGroup,
		Id:           common.ToLower(nodeRequest.Name),
		Name:         common.ToLower(nodeRequest.Name),
		Uid:          common.GenUid(),
		// Record the number of Nodes actually created, not the requested value:
		// a request may omit it (0) while one Node is still created, and a later
		// ScaleOut would then reuse that Node's name (issue #2652)
		NodeGroupSize: nodeGroupSize,
		RootDiskType:  nodeRequest.RootDiskType,
		RootDiskSize:  nodeRequest.RootDiskSize,
	}

	// Build Node ID list
	for i := nodeStartIndex; i < nodeGroupSize+nodeStartIndex; i++ {
		if nodeRequest.CspResourceId != "" {
			// Register mode: one node per registration, no index suffix
			nodeGroupInfoData.NodeId = append(nodeGroupInfoData.NodeId, nodeGroupInfoData.Id)
			break
		} else {
			nodeGroupInfoData.NodeId = append(nodeGroupInfoData.NodeId, nodeGroupInfoData.Id+"-"+strconv.Itoa(i))
		}
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
func createInfraObject(ctx context.Context, nsId, infraId string, req *model.InfraReq, uid string, option string) error {
	log.Info().Msg("Creating Infra object")
	key := common.GenInfraKey(nsId, infraId, "")

	// Register is a discovery-type action: keep the Infra-level TargetAction consistent
	// with its Nodes (which use ActionRegister) so discovery-aware Infra logic triggers.
	initStatus := model.StatusCreating
	initTargetAction := model.ActionCreate
	if option == "register" {
		initStatus = model.StatusRegistering
		initTargetAction = model.ActionRegister
	}

	infraInfo := model.InfraInfo{
		ResourceType:     model.StrInfra,
		Id:               infraId,
		Name:             req.Name,
		Uid:              uid,
		Description:      req.Description,
		Status:           initStatus,
		TargetAction:     initTargetAction,
		TargetStatus:     model.StatusRunning,
		InstallMonAgent:  req.InstallMonAgent,
		SystemLabel:      req.SystemLabel,
		PostCommands:     req.PostCommands,
		PostCommandAsync: req.PostCommandAsync,
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
	maps.Copy(labels, req.Label)

	return label.CreateOrUpdateLabel(ctx, model.StrInfra, uid, key, labels)
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

	// Intelligent wait time based on Node count
	waitTime := 30 * time.Second
	if len(infraTmp.Node) > 5 {
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

// Infra and Node Provisioning

// ScaleOutInfraNodeGroup is func to create Infra groupNode
func ScaleOutInfraNodeGroup(ctx context.Context, nsId string, infraId string, nodeGroupId string, numNodesToAdd int) (*model.InfraInfo, error) {
	result, _, err := ScaleOutInfraNodeGroupFrom(ctx, nsId, infraId, nodeGroupId, numNodesToAdd, "", "")
	return result, err
}

// ScaleOutInfraNodeGroupFrom is ScaleOutInfraNodeGroup with explicit control over
// where the new Nodes land.
//
// templateNodeId names the Node to copy the configuration from. ListNodeByNodeGroup
// returns Nodes in KV scan order, so leaving it empty makes the choice arbitrary —
// which matters once a NodeGroup spans several subnets (DistributeSubnets), because
// the copied SubnetId then decides the zone. Callers that need a predictable
// placement must name the template Node.
//
// subnetIdOverride places the new Nodes in another subnet of the same VNet. This is
// not the same as pinning a zone on a dynamic request: that builds a zone-scoped
// VNet, whereas this keeps the VPC, security group and key, so the new Nodes stay
// on the Infra's private network.
// It returns the ids of the Nodes it created alongside the updated Infra.
func ScaleOutInfraNodeGroupFrom(ctx context.Context, nsId string, infraId string, nodeGroupId string, numNodesToAdd int, templateNodeId string, subnetIdOverride string) (*model.InfraInfo, []string, error) {
	if numNodesToAdd < 1 {
		return &model.InfraInfo{}, nil, fmt.Errorf("numNodesToAdd must be 1 or more (got %d)", numNodesToAdd)
	}

	nodeIdList, err := ListNodeByNodeGroup(nsId, infraId, nodeGroupId)
	if err != nil {
		temp := &model.InfraInfo{}
		return temp, nil, err
	}
	if len(nodeIdList) == 0 {
		return &model.InfraInfo{}, nil, fmt.Errorf("NodeGroup '%s' has no Node in Infra '%s'; scale-out needs an existing Node to copy the configuration from", nodeGroupId, infraId)
	}
	sourceNodeId := nodeIdList[0]
	if templateNodeId != "" {
		if !contains(nodeIdList, templateNodeId) {
			return &model.InfraInfo{}, nil, fmt.Errorf("template Node '%s' is not in NodeGroup '%s'", templateNodeId, nodeGroupId)
		}
		sourceNodeId = templateNodeId
	}
	nodeObj, err := GetNodeObject(nsId, infraId, sourceNodeId)
	if err != nil {
		temp := &model.InfraInfo{}
		return temp, nil, err
	}

	nodeGroupReqTemplate := &model.CreateNodeGroupReq{}

	// only take template required to create Node
	nodeGroupReqTemplate.Name = nodeObj.NodeGroupId
	nodeGroupReqTemplate.ConnectionName = nodeObj.ConnectionName
	nodeGroupReqTemplate.ImageId = nodeObj.ImageId
	// Carry the resolved CSP image name over: without it CreateNode falls back to a
	// namespace image lookup that fails for CSPs whose imageId is not a TB resource id
	nodeGroupReqTemplate.CspImageName = nodeObj.CspImageName
	nodeGroupReqTemplate.SpecId = nodeObj.SpecId
	nodeGroupReqTemplate.Label = filterOutSystemLabels(nodeObj.Label)
	nodeGroupReqTemplate.VNetId = nodeObj.VNetId
	nodeGroupReqTemplate.SubnetId = nodeObj.SubnetId
	if subnetIdOverride != "" {
		nodeGroupReqTemplate.SubnetId = subnetIdOverride
	}
	nodeGroupReqTemplate.SecurityGroupIds = nodeObj.SecurityGroupIds
	nodeGroupReqTemplate.SshKeyId = nodeObj.SshKeyId
	nodeGroupReqTemplate.NodeUserName = nodeObj.NodeUserName
	nodeGroupReqTemplate.NodeUserPassword = nodeObj.NodeUserPassword
	// Root disk config comes from the NodeGroup record (what was requested). The Node
	// holds the CSP-reported type, which some CSPs reject when sent back as a request.
	if nodeGroupInfo, err := GetNodeGroup(nsId, infraId, nodeGroupId); err == nil {
		nodeGroupReqTemplate.RootDiskType = nodeGroupInfo.RootDiskType
		nodeGroupReqTemplate.RootDiskSize = nodeGroupInfo.RootDiskSize
	}
	if nodeGroupReqTemplate.RootDiskSize == 0 {
		nodeGroupReqTemplate.RootDiskSize = nodeObj.RootDiskSize
	}
	nodeGroupReqTemplate.Description = nodeObj.Description

	nodeGroupReqTemplate.NodeGroupSize = numNodesToAdd

	result, newNodeIds, err := createInfraGroupNodeWithIds(ctx, nsId, infraId, nodeGroupReqTemplate, true)
	if err != nil {
		// The ids are still reported on failure: a Node record may exist and need
		// cleaning up even though creation did not succeed.
		return &model.InfraInfo{}, newNodeIds, err
	}
	return result, newNodeIds, nil
}

// CreateInfraGroupNode is func to create Infra groupNode
func CreateInfraGroupNode(ctx context.Context, nsId string, infraId string, nodeRequest *model.CreateNodeGroupReq, newNodeGroup bool) (*model.InfraInfo, error) {
	result, _, err := createInfraGroupNodeWithIds(ctx, nsId, infraId, nodeRequest, newNodeGroup)
	return result, err
}

// createInfraGroupNodeWithIds is CreateInfraGroupNode that also returns the Node
// ids it reserved. Identifying new Nodes by diffing the NodeGroup listing before
// and after is unreliable once several creations run concurrently on the same
// NodeGroup — each would see the others' Nodes appear.
func createInfraGroupNodeWithIds(ctx context.Context, nsId string, infraId string, nodeRequest *model.CreateNodeGroupReq, newNodeGroup bool) (*model.InfraInfo, []string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := &model.InfraInfo{}
		log.Error().Err(err).Msg("")
		return temp, nil, err
	}

	err = common.CheckString(infraId)
	if err != nil {
		temp := &model.InfraInfo{}
		log.Error().Err(err).Msg("")
		return temp, nil, err
	}

	// returns InvalidValidationError for bad validation input, nil or ValidationErrors ( []FieldError )
	err = validate.Struct(nodeRequest)
	if err != nil {

		// this check is only needed when your code could produce
		// an invalid value for validation such as interface with nil
		// value most including myself do not usually have code like this.
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Err(err).Msg("")
			return nil, nil, err
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

		return nil, nil, err
	}

	infraTmp, _, err := GetInfraObject(nsId, infraId)

	if err != nil {
		temp := &model.InfraInfo{}
		return temp, nil, err
	}

	//nodeRequest := req

	targetAction := model.ActionCreate
	targetStatus := model.StatusRunning

	//goroutin
	var wg sync.WaitGroup

	// nodeGroup handling
	nodeGroupSize := nodeRequest.NodeGroupSize
	fmt.Printf("nodeGroupSize: %v\n", nodeGroupSize)

	// make nodeGroup default (any Node going to be in a nodeGroup)
	if nodeGroupSize < 1 {
		nodeGroupSize = 1
	}

	tentativeNodeId := common.ToLower(nodeRequest.Name)

	err = common.CheckString(tentativeNodeId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return &model.InfraInfo{}, nil, err
	}

	// Create or update nodeGroup object (nodeGroupSize is always >= 1)
	log.Info().Msg("Create Infra nodeGroup object")

	newNodeIds, err := reserveNodeNames(nsId, infraId, tentativeNodeId, nodeRequest, nodeGroupSize, newNodeGroup)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, nil, err
	}
	log.Info().Msgf("Reserved Node names for NodeGroup %s: %v", tentativeNodeId, newNodeIds)

	var objectErrs []error
	var objectErrMu sync.Mutex

	// Create Node objects for the reserved names
	for i, reservedNodeId := range newNodeIds {
		nodeInfoData := model.NodeInfo{}

		nodeInfoData.NodeGroupId = tentativeNodeId
		nodeInfoData.Name = reservedNodeId

		log.Debug().Msg("nodeInfoData.Name: " + nodeInfoData.Name)

		nodeInfoData.ResourceType = model.StrNode
		nodeInfoData.Id = nodeInfoData.Name
		nodeInfoData.Uid = common.GenUid()

		nodeInfoData.PublicIP = ""
		nodeInfoData.PublicDNS = ""

		// Set initial status based on whether this is a registration (CspResourceId is set).
		// Register is a discovery-type action (ActionRegister): its target is the resource's
		// actual CSP state, resolved via late-binding in FetchNodeStatus.
		if nodeRequest.CspResourceId != "" {
			nodeInfoData.Status = model.StatusRegistering
			nodeInfoData.TargetAction = model.ActionRegister
		} else {
			nodeInfoData.Status = model.StatusCreating
			nodeInfoData.TargetAction = targetAction
		}
		nodeInfoData.TargetStatus = targetStatus

		nodeInfoData.ConnectionName = nodeRequest.ConnectionName
		nodeInfoData.ConnectionConfig, err = common.GetConnConfig(nodeRequest.ConnectionName)
		if err != nil {
			err = fmt.Errorf("Cannot retrieve ConnectionConfig: %s", err.Error())
			log.Error().Err(err).Msg("")
		}
		nodeInfoData.Location = nodeInfoData.ConnectionConfig.RegionDetail.Location
		nodeInfoData.SpecId = nodeRequest.SpecId
		nodeInfoData.ImageId = nodeRequest.ImageId
		nodeInfoData.CspImageName = nodeRequest.CspImageName // pre-resolved at nodegroup level; empty for custom images
		nodeInfoData.VNetId = nodeRequest.VNetId
		// Distribute VMs across subnets when a subnet list is provided (round-robin by VM index),
		// otherwise all VMs use the single SubnetId. The index-based mapping is deterministic, so
		// nodes added later to the same NodeGroup keep a consistent VM-to-subnet assignment.
		nodeInfoData.SubnetId = nodeRequest.SubnetId
		if len(nodeRequest.SubnetIds) > 0 {
			nodeInfoData.SubnetId = nodeRequest.SubnetIds[i%len(nodeRequest.SubnetIds)]
		}
		nodeInfoData.SecurityGroupIds = nodeRequest.SecurityGroupIds
		nodeInfoData.DataDiskIds = nodeRequest.DataDiskIds
		nodeInfoData.SshKeyId = nodeRequest.SshKeyId
		nodeInfoData.Description = nodeRequest.Description
		nodeInfoData.NodeUserName = nodeRequest.NodeUserName
		nodeInfoData.NodeUserPassword = nodeRequest.NodeUserPassword
		nodeInfoData.RootDiskType = nodeRequest.RootDiskType
		nodeInfoData.RootDiskSize = nodeRequest.RootDiskSize

		nodeInfoData.Label = nodeRequest.Label

		nodeInfoData.CspResourceId = nodeRequest.CspResourceId

		wg.Add(1)
		go func(node model.NodeInfo) {
			if err := CreateNodeObject(&wg, nsId, infraId, &node); err != nil {
				objectErrMu.Lock()
				objectErrs = append(objectErrs, err)
				objectErrMu.Unlock()
			}
		}(nodeInfoData)
	}
	wg.Wait()
	if len(objectErrs) > 0 {
		err := fmt.Errorf("failed to create Node objects for NodeGroup %s: %v", tentativeNodeId, objectErrs)
		log.Error().Err(err).Msg("")
		return nil, newNodeIds, err
	}

	// Set option based on whether this is a registration (CspResourceId is set)
	option := "create"
	if nodeRequest.CspResourceId != "" {
		option = "register"
	}

	// Collect all Node info for rate-limited parallel processing
	var nodeInfoList []*model.NodeInfo
	for _, nodeId := range newNodeIds {
		nodeInfo, err := GetNodeObject(nsId, infraId, nodeId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return nil, newNodeIds, err
		}
		nodeInfoList = append(nodeInfoList, &nodeInfo)
	}

	// Create VMs with hierarchical rate limiting
	log.Info().Msgf("Creating %d VMs with rate limiting", len(nodeInfoList))
	err = CreateNodesInParallel(ctx, nsId, infraId, nodeInfoList, option)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create VMs in parallel")
		return nil, newNodeIds, err
	}

	//Update Infra status

	infraTmp, _, err = GetInfraObject(nsId, infraId)
	if err != nil {
		temp := &model.InfraInfo{}
		return temp, newNodeIds, err
	}

	infraStatusTmp, _ := GetInfraStatus(nsId, infraId)

	infraTmp.Status = infraStatusTmp.Status

	// More robust completion check for Create/Register action.
	// Register (discovery-type) shares this path: it also completes when every Node
	// reaches a final state — but that final state may be Running, Suspended, etc.
	isCreateCompleted := false
	if infraTmp.TargetAction == model.ActionCreate || infraTmp.TargetAction == model.ActionRegister {
		// For Create action, check if all VMs are in final states (including Failed)
		// Final states: Running, Failed, Terminated, Suspended
		// Transitional states: Creating, Undefined, empty string
		allNodesInFinalState := true
		pendingCount := 0
		runningCount := 0
		failedCount := 0
		totalNodeCount := len(infraStatusTmp.Node)

		for _, node := range infraStatusTmp.Node {
			// Check if VM is still in transitional/pending state.
			// StatusUndefined is NOT treated as pending here — it means the creation
			// attempt is done (Spider returned 500 with no VM identity). The node is
			// an orphan candidate; run action=reconcile to rescue or action=refine to remove.
			if node.Status == model.StatusCreating || node.Status == model.StatusRegistering || node.Status == model.StatusReconciling || node.Status == "" {
				allNodesInFinalState = false
				pendingCount++
			} else {
				// VM is in final state, count by type for logging
				switch node.Status {
				case model.StatusRunning:
					runningCount++
				case model.StatusFailed:
					failedCount++
				case model.StatusUndefined:
					failedCount++ // treat as non-running final state for summary purposes
				}
			}
		}

		if allNodesInFinalState && totalNodeCount > 0 {
			isCreateCompleted = true
			if failedCount > 0 {
				log.Info().Msgf("Infra %s Create action completed with partial success: %d running, %d failed, %d total VMs",
					infraId, runningCount, failedCount, totalNodeCount)
			} else {
				log.Info().Msgf("Infra %s Create action completed successfully: all %d VMs reached final state",
					infraId, totalNodeCount)
			}
		} else {
			log.Debug().Msgf("Infra %s Create action pending: %d/%d VMs still in transitional state",
				infraId, pendingCount, totalNodeCount)
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

	// Only the Nodes added by this call: callers (and the registration flow) read this
	// to identify what was just created, so the whole NodeGroup must not be reported
	infraTmp.NewNodeList = newNodeIds

	return &infraTmp, newNodeIds, nil

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
		nodeObjectErrors []model.NodeCreationError
		nodeCreateErrors []model.NodeCreationError
		totalNodeCount   int
		errorMu          sync.Mutex
	)

	// Count total VMs to be created (minimum 1 per nodeGroup)
	for _, nodeGroupReq := range req.NodeGroups {
		nodeCount := max(nodeGroupReq.NodeGroupSize, 1)
		totalNodeCount += nodeCount
	}

	// Helper function to add VM creation error (with mutex for standalone use)
	addNodeError := func(errors *[]model.NodeCreationError, nodeName, errorMsg, phase string) {
		errorMu.Lock()
		defer errorMu.Unlock()
		*errors = append(*errors, model.NodeCreationError{
			NodeName:  nodeName,
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
	type nodeConfig struct {
		nodeInfo      model.NodeInfo
		nodeGroupSize int
		nodeIndex     int
	}

	var nodeConfigs []nodeConfig
	var nodeGroupsCreated []string
	nodeStartIndex := 1

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
			if option == "register" {
				infraTmp.Status = model.StatusRegistering
				infraTmp.TargetAction = model.ActionRegister
			}
			UpdateInfraInfo(nsId, infraTmp)
		}
	} else {
		// fallback for manual infra create. not from isReqFromDynamic.
		if !exists {
			log.Debug().Msgf("Infra '%s' does not exist, creating new one", infraId)
			// Create Infra object first
			if err := createInfraObject(ctx, nsId, infraId, req, uid, option); err != nil {
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
		nodeGroupSize := max(nodeGroupReq.NodeGroupSize, 1)

		log.Debug().Msgf("Processing VM request '%s' with nodeGroupSize: %d", nodeGroupReq.Name, nodeGroupSize)

		// Get connection config once and validate
		connectionConfig, err := common.GetConnConfig(nodeGroupReq.ConnectionName)
		if err != nil {
			return nil, fmt.Errorf("cannot retrieve connection config for VM '%s': %w", nodeGroupReq.Name, err)
		}

		// Pre-resolve CspImageName once per nodeGroup so each per-VM CreateNode call can skip
		// the redundant GetImage DB query.  Custom images stay empty and go through the full path.
		if nodeGroupReq.CspImageName == "" && nodeGroupReq.ImageId != "" {
			if imgInfo, err := resource.GetImage(nsId, nodeGroupReq.ImageId); err == nil &&
				imgInfo.ResourceType != model.StrCustomImage {
				nodeGroupReq.CspImageName = imgInfo.CspImageName
			}
		}

		// Create nodeGroup if needed
		if nodeGroupSize > 0 {
			nodeGroupName := common.ToLower(nodeGroupReq.Name)
			if !contains(nodeGroupsCreated, nodeGroupName) {
				if err := createNodeGroup(ctx, nsId, infraId, &nodeGroupReq, nodeGroupSize, nodeStartIndex, uid, req); err != nil {
					return nil, fmt.Errorf("failed to create nodeGroup '%s': %w", nodeGroupName, err)
				}
				nodeGroupsCreated = append(nodeGroupsCreated, nodeGroupName)
			}
		}

		// Build VM configurations
		for i := nodeStartIndex; i <= nodeGroupSize+nodeStartIndex; i++ {
			if nodeGroupSize > 0 && i == nodeGroupSize+nodeStartIndex {
				break
			}

			// Set initial status and action based on option (create vs register).
			// Register is a discovery-type action (ActionRegister): its target is the
			// resource's actual CSP state, resolved via late-binding in FetchNodeStatus.
			initialStatus := model.StatusCreating
			initialTargetAction := model.ActionCreate
			if option == "register" {
				initialStatus = model.StatusRegistering
				initialTargetAction = model.ActionRegister
			}

			nodeInfo := model.NodeInfo{
				ResourceType:     model.StrNode,
				Uid:              common.GenUid(),
				PublicIP:         "",
				PublicDNS:        "",
				Status:           initialStatus,
				TargetAction:     initialTargetAction,
				TargetStatus:     model.StatusRunning,
				ConnectionName:   nodeGroupReq.ConnectionName,
				ConnectionConfig: connectionConfig,
				Location:         connectionConfig.RegionDetail.Location,
				SpecId:           nodeGroupReq.SpecId,
				ImageId:          nodeGroupReq.ImageId,
				CspImageName:     nodeGroupReq.CspImageName,
				VNetId:           nodeGroupReq.VNetId,
				SubnetId:         nodeGroupReq.SubnetId,
				SecurityGroupIds: nodeGroupReq.SecurityGroupIds,
				DataDiskIds:      nodeGroupReq.DataDiskIds,
				SshKeyId:         nodeGroupReq.SshKeyId,
				Description:      nodeGroupReq.Description,
				NodeUserName:     nodeGroupReq.NodeUserName,
				NodeUserPassword: nodeGroupReq.NodeUserPassword,
				RootDiskType:     nodeGroupReq.RootDiskType,
				RootDiskSize:     nodeGroupReq.RootDiskSize,
				Label:            nodeGroupReq.Label,
				CspResourceId:    nodeGroupReq.CspResourceId,
			}

			if nodeGroupSize == 0 {
				nodeInfo.Name = common.ToLower(nodeGroupReq.Name)
			} else {
				nodeInfo.NodeGroupId = common.ToLower(nodeGroupReq.Name)
				if nodeGroupReq.CspResourceId != "" {
					// Register mode: node name is the nodeGroup name itself (no index suffix)
					nodeInfo.Name = common.ToLower(nodeGroupReq.Name)
				} else {
					nodeInfo.Name = common.ToLower(nodeGroupReq.Name) + "-" + strconv.Itoa(i)
				}
			}
			nodeInfo.Id = nodeInfo.Name

			// Distribute VMs across subnets when a subnet list is provided (round-robin by VM
			// index), otherwise all VMs use the single SubnetId. Deterministic per VM index so
			// nodes added later keep a consistent VM-to-subnet assignment.
			if len(nodeGroupReq.SubnetIds) > 0 {
				nodeInfo.SubnetId = nodeGroupReq.SubnetIds[i%len(nodeGroupReq.SubnetIds)]
			}

			nodeConfigs = append(nodeConfigs, nodeConfig{
				nodeInfo:      nodeInfo,
				nodeGroupSize: nodeGroupSize,
				nodeIndex:     i,
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

	log.Info().Msgf("Creating %d VM objects", len(nodeConfigs))

	for _, config := range nodeConfigs {
		wg.Add(1)
		go func(cfg nodeConfig) {
			defer wg.Done()
			if err := createNodeObjectSafe(nsId, infraId, &cfg.nodeInfo); err != nil {
				errorMu.Lock()
				createErrors = append(createErrors, fmt.Errorf("VM object creation failed for '%s': %w", cfg.nodeInfo.Name, err))
				addNodeError(&nodeObjectErrors, cfg.nodeInfo.Name, err.Error(), "object_creation")
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
			errorSummary := fmt.Sprintf("VM object creation failed for %d out of %d VMs", len(createErrors), len(nodeConfigs))
			infraTmp.SystemMessage = append(infraTmp.SystemMessage, errorSummary)

			// Add each VM object creation error
			for _, nodeError := range nodeObjectErrors {
				errorDetail := fmt.Sprintf("VM '%s' object creation failed: %s", nodeError.NodeName, nodeError.Error)
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
	log.Info().Msgf("Creating %d VMs with rate limiting", len(nodeConfigs))
	createErrors = createErrors[:0] // Reset error slice

	// Collect all Node info for rate-limited parallel processing
	var nodeInfoList []*model.NodeInfo
	for _, config := range nodeConfigs {
		nodeInfoData, err := GetNodeObject(nsId, infraId, config.nodeInfo.Id)
		if err != nil {
			return nil, fmt.Errorf("failed to get VM object '%s': %w", config.nodeInfo.Id, err)
		}
		nodeInfoList = append(nodeInfoList, &nodeInfoData)
	}

	// Create VMs with hierarchical rate limiting
	err = CreateNodesInParallel(ctx, nsId, infraId, nodeInfoList, option)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create VMs in parallel")

		// CRITICAL: If CreateNodesInParallel returns error, it means ALL VMs failed
		// Check total VM count and immediately terminate if all failed
		totalNodesInParallel := len(nodeInfoList)

		log.Error().Msgf("EARLY TERMINATION: CreateNodesInParallel returned error - all %d VMs failed", totalNodesInParallel)

		// Force update all VM statuses to Failed since CreateNodesInParallel failed completely
		log.Debug().Msg("Force updating all VM statuses to Failed since no VMs were actually created")
		for _, nodeInfo := range nodeInfoList {
			nodeInfo.Status = model.StatusFailed
			if nodeInfo.SystemMessage == "" {
				nodeInfo.SystemMessage = fmt.Sprintf("VM creation failed: %s", err.Error())
			}

			UpdateNodeInfo(nsId, infraId, *nodeInfo)
			log.Debug().Msgf("Force updated VM %s to Failed status (no actual CSP VM created)", nodeInfo.Name)
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
			infraId, totalNodesInParallel, err.Error())

		return infraResult, fmt.Errorf("%s", errorMsg)
	}

	// Continue with normal processing for successful or partial VM creation
	// Note: If CreateNodesInParallel returns error, we already handled it above and returned early
	// This code block is only reached when VM creation was successful or partially successful

	// Check for VM creation errors (this applies to partial failures only)
	if len(createErrors) > 0 {
		// Add VM creation errors to Infra SystemMessage
		infraTmp, _, err := GetInfraObject(nsId, infraId)
		if err == nil {
			// Add VM creation error summary
			errorSummary := fmt.Sprintf("VM creation failed for %d out of %d VMs", len(createErrors), len(nodeConfigs))
			infraTmp.SystemMessage = append(infraTmp.SystemMessage, errorSummary)

			// Add each VM creation error - use nodeObjectErrors if nodeCreateErrors is empty
			errorList := nodeCreateErrors
			if len(errorList) == 0 {
				errorList = nodeObjectErrors
			}
			for _, nodeError := range errorList {
				errorDetail := fmt.Sprintf("VM '%s' creation failed: %s", nodeError.NodeName, nodeError.Error)
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

	log.Info().Msgf("Infra '%s' has been successfully created with %d VMs", infraId, len(nodeConfigs))

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
	if req.PolicyOnPartialFailure == model.PolicyRefine && (len(nodeObjectErrors) > 0 || len(nodeCreateErrors) > 0) {
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

	// Note: All VM failure case is already handled earlier when CreateNodesInParallel returns error
	// This section only handles partial failures or successful cases

	// Add creation error information if there were any failures
	if len(nodeObjectErrors) > 0 || len(nodeCreateErrors) > 0 {
		successfulNodeCount := totalNodeCount - len(nodeObjectErrors) - len(nodeCreateErrors)
		failedNodeCount := len(nodeObjectErrors) + len(nodeCreateErrors)

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
			NodeObjectCreationErrors: nodeObjectErrors,
			NodeCreationErrors:       nodeCreateErrors,
			TotalNodeCount:           totalNodeCount,
			SuccessfulNodeCount:      successfulNodeCount,
			FailedNodeCount:          failedNodeCount,
			FailureHandlingStrategy:  failureStrategy,
		}

		log.Info().Msgf("Infra '%s' creation completed with %d successful VMs out of %d total (strategy: %s, refined: %t)",
			infraId, successfulNodeCount, totalNodeCount, failureStrategy, shouldRefine)
	} else {
		log.Info().Msgf("Infra '%s' has been successfully created with all %d VMs", infraId, totalNodeCount)
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
