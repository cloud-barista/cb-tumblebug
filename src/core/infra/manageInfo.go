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
	cspdirect "github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/rs/zerolog/log"
)

var infraInfoMutex sync.Mutex

// [Infra and Node object information managemenet]

// ListInfraId is func to list Infra ID
func ListInfraId(nsId string) ([]string, error) {

	var infraList []string

	// Check Infra exists
	key := common.GenInfraKey(nsId, "", "")
	key += "/"

	keys, err := kvstore.GetKeyList(key)

	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	for _, k := range keys {
		if strings.Contains(k, "/infra/") {
			trimmedString := strings.TrimPrefix(k, (key + "infra/"))
			// prevent malformed key (if key for infra id includes '/', the key does not represent Infra ID)
			if !strings.Contains(trimmedString, "/") {
				infraList = append(infraList, trimmedString)
			}
		}
	}

	return infraList, nil
}

// ListNodeId is func to list Node IDs
func ListNodeId(nsId string, infraId string) ([]string, error) {

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

	var nodeList []string

	// Check Infra exists
	key := common.GenInfraKey(nsId, infraId, "")
	key += "/"

	_, _, err := kvstore.GetKv(key)
	if err != nil {
		log.Debug().Msg("[Not found] " + infraId)
		log.Error().Err(err).Msg("")
		return nodeList, err
	}

	// WithKeysOnly: etcd returns only key bytes, not values.
	// For large infras this avoids transferring 10,000+ NodeInfo JSON objects
	// (~50 MB) that would exceed the gRPC default MaxCallRecvMsgSize (2 MB).
	keys, err := kvstore.GetKeyList(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	nodePrefix := key + model.StrNode + "/"
	for _, k := range keys {
		if strings.HasPrefix(k, nodePrefix) {
			trimmedString := strings.TrimPrefix(k, nodePrefix)
			// prevent malformed key (if key for node id includes '/', the key does not represent Node ID)
			if !strings.Contains(trimmedString, "/") {
				nodeList = append(nodeList, trimmedString)
			}
		}
	}

	return nodeList, nil

}

// ListNodeByLabel is a function to list Node IDs by label
func ListNodeByLabel(nsId string, infraId string, labelKey string) ([]string, error) {
	// Construct the label selector
	labelSelector := labelKey + " exists" + "," + model.LabelNamespace + "=" + nsId + "," + model.LabelInfraId + "=" + infraId

	// Call GetResourcesByLabelSelector (returns []interface{})
	resources, err := label.GetResourcesByLabelSelector(model.StrNode, labelSelector)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get resources by label selector")
		return nil, err
	}

	// Slice to store the list of Node IDs
	var nodeListByLabel []string

	// Convert []interface{} to NodeInfo and extract IDs
	for _, resource := range resources {
		if nodeInfo, ok := resource.(*model.NodeInfo); ok {
			nodeListByLabel = append(nodeListByLabel, nodeInfo.Id)
		} else {
			log.Warn().Msg("Resource is not of type NodeInfo")
		}
	}

	// Return the list of Node IDs
	return nodeListByLabel, nil
}

// ListNodeByFilter is func to get list Nodes in an Infra by a filter consist of Key and Value
func ListNodeByFilter(nsId string, infraId string, filterKey string, filterVal string) ([]string, error) {

	check, err := CheckInfra(nsId, infraId)
	if !check {
		err := fmt.Errorf("Not found the Infra: " + infraId + " from the NS: " + nsId)
		return nil, err
	}

	nodeList, err := ListNodeId(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	if len(nodeList) == 0 {
		return nil, nil
	}
	if filterKey == "" {
		return nodeList, nil
	}

	// Use existing ListInfraNodeInfo function instead of individual GetNodeObject calls
	nodeInfoList, err := ListInfraNodeInfo(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	var groupNodeList []string

	for _, nodeObj := range nodeInfoList {
		nodeObjReflect := reflect.ValueOf(&nodeObj)
		elements := nodeObjReflect.Elem()
		for i := 0; i < elements.NumField(); i++ {
			key := elements.Type().Field(i).Name
			if strings.EqualFold(filterKey, key) {
				//fmt.Println(key)

				val := elements.Field(i).Interface().(string)
				//fmt.Println(val)
				if strings.EqualFold(filterVal, val) {

					groupNodeList = append(groupNodeList, nodeObj.Id)
					//fmt.Println(groupNodeList)
				}

				break
			}
		}
	}
	return groupNodeList, nil
}

// ListNodeByNodeGroup is func to get Node list with a NodeGroup label in a specified Infra
func ListNodeByNodeGroup(nsId string, infraId string, groupId string) ([]string, error) {
	// NodeGroupId is the Key for NodeGroupId in model.NodeInfo struct
	filterKey := "NodeGroupId"
	return ListNodeByFilter(nsId, infraId, filterKey, groupId)
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

	keys, err := kvstore.GetKeyList(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	var nodeGroupList []string
	for _, k := range keys {
		if strings.Contains(k, "/"+model.StrNodeGroup+"/") {
			trimmedString := strings.TrimPrefix(k, (key + model.StrNodeGroup + "/"))
			// prevent malformed key (if key for node id includes '/', the key does not represent Node ID)
			if !strings.Contains(trimmedString, "/") {
				nodeGroupList = append(nodeGroupList, trimmedString)
			}
		}
	}
	return nodeGroupList, nil
}

func appendNonEmptyString(values []string, candidate string) []string {
	if candidate == "" {
		return values
	}
	return append(values, candidate)
}

func sortAndCompactStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	sort.Strings(values)
	compacted := values[:1]
	for i := 1; i < len(values); i++ {
		if values[i] != values[i-1] {
			compacted = append(compacted, values[i])
		}
	}
	return compacted
}

func buildImplicitClusterGroupKey(node model.NodeInfo) string {
	vNetId := common.ToLower(strings.TrimSpace(node.VNetId))
	nodeGroupId := common.ToLower(strings.TrimSpace(node.NodeGroupId))

	if vNetId != "" {
		// Primary grouping: same VNet within the Infra.
		return vNetId
	}

	// Fallback: keep each NodeGroup as an independent implicit cluster if VNet is not present.
	if nodeGroupId == "" {
		nodeGroupId = common.ToLower(strings.TrimSpace(node.Id))
	}
	return "nogroup|" + nodeGroupId
}

func buildImplicitClusterId(node model.NodeInfo) string {
	vNetId := common.ToLower(strings.TrimSpace(node.VNetId))
	nodeGroupId := common.ToLower(strings.TrimSpace(node.NodeGroupId))

	if vNetId != "" {
		return vNetId
	}

	if nodeGroupId == "" {
		nodeGroupId = common.ToLower(strings.TrimSpace(node.Id))
	}
	if nodeGroupId == "" {
		nodeGroupId = "unknown"
	}
	return fmt.Sprintf("nogroup--%s", nodeGroupId)
}

func normalizeImplicitClusterInfo(cluster *model.InfraClusterInfo) {
	cluster.ConnectionNames = sortAndCompactStrings(cluster.ConnectionNames)
	cluster.ProviderNames = sortAndCompactStrings(cluster.ProviderNames)
	cluster.RegionNames = sortAndCompactStrings(cluster.RegionNames)
	cluster.NodeGroupIds = sortAndCompactStrings(cluster.NodeGroupIds)
	cluster.NodeIds = sortAndCompactStrings(cluster.NodeIds)
	cluster.NodeGroupCount = len(cluster.NodeGroupIds)
	cluster.NodeCount = len(cluster.NodeIds)
	// Fix representative fields to be deterministic (use first sorted entry)
	if len(cluster.NodeGroupIds) > 0 {
		cluster.RepresentativeNodeGroupId = cluster.NodeGroupIds[0]
	}
	if len(cluster.NodeIds) > 0 {
		cluster.RepresentativeNodeId = cluster.NodeIds[0]
	}
}

func buildImplicitClusterInfoFromNodes(infraId string, nodeInfoList []model.NodeInfo) []model.InfraClusterInfo {
	if len(nodeInfoList) == 0 {
		return []model.InfraClusterInfo{}
	}

	clustersByKey := map[string]*model.InfraClusterInfo{}

	for _, node := range nodeInfoList {
		groupKey := buildImplicitClusterGroupKey(node)
		cluster, exists := clustersByKey[groupKey]
		if !exists {
			cluster = &model.InfraClusterInfo{
				Id:                        buildImplicitClusterId(node),
				Name:                      buildImplicitClusterId(node),
				InfraId:                   infraId,
				VNetId:                    node.VNetId,
				RepresentativeNodeGroupId: node.NodeGroupId,
				RepresentativeNodeId:      node.Id,
			}
			clustersByKey[groupKey] = cluster
		}

		nodeGroupId := node.NodeGroupId
		if nodeGroupId == "" {
			nodeGroupId = node.Id
		}

		cluster.NodeGroupIds = appendNonEmptyString(cluster.NodeGroupIds, nodeGroupId)
		cluster.NodeIds = appendNonEmptyString(cluster.NodeIds, node.Id)
		cluster.ConnectionNames = appendNonEmptyString(cluster.ConnectionNames, node.ConnectionName)
		cluster.ProviderNames = appendNonEmptyString(cluster.ProviderNames, node.ConnectionConfig.ProviderName)
		cluster.RegionNames = appendNonEmptyString(cluster.RegionNames, node.Region.Region)
	}

	clusterList := make([]model.InfraClusterInfo, 0, len(clustersByKey))
	for _, cluster := range clustersByKey {
		normalizeImplicitClusterInfo(cluster)
		clusterList = append(clusterList, *cluster)
	}

	sort.Slice(clusterList, func(i, j int) bool {
		return clusterList[i].Id < clusterList[j].Id
	})

	return clusterList
}

// ListInfraClusterInfo returns implicit cluster views synthesized at query-time from Infra Nodes.
// No persistent cluster object is created or maintained.
func ListInfraClusterInfo(nsId string, infraId string) ([]model.InfraClusterInfo, error) {
	check, err := CheckInfra(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("Cannot check Infra existence")
		return nil, err
	}
	if !check {
		return nil, fmt.Errorf("the infra %s does not exist", infraId)
	}

	nodeInfoList, err := ListInfraNodeInfo(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("failed to list Nodes for implicit cluster view")
		return nil, err
	}

	return buildImplicitClusterInfoFromNodes(infraId, nodeInfoList), nil
}

// GetInfraClusterInfo returns a single implicit cluster view synthesized at query-time.
func GetInfraClusterInfo(nsId string, infraId string, clusterId string) (*model.InfraClusterInfo, error) {
	if err := common.CheckString(clusterId); err != nil {
		log.Error().Err(err).Msg("invalid clusterId")
		return nil, err
	}

	clusters, err := ListInfraClusterInfo(nsId, infraId)
	if err != nil {
		return nil, err
	}

	for i := range clusters {
		if clusters[i].Id == clusterId {
			return &clusters[i], nil
		}
	}

	return nil, fmt.Errorf("the cluster %s does not exist in infra %s", clusterId, infraId)
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

	nodeList, err := ListNodeId(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	sort.Slice(infraObj.Node, func(i, j int) bool {
		return infraObj.Node[i].Id < infraObj.Node[j].Id
	})

	for nodeInfoIndex := range nodeList {
		for nodeStatusInfoIndex := range infraStatus.Node {
			if infraObj.Node[nodeInfoIndex].Id == infraStatus.Node[nodeStatusInfoIndex].Id {
				infraObj.Node[nodeInfoIndex].Status = infraStatus.Node[nodeStatusInfoIndex].Status
				infraObj.Node[nodeInfoIndex].TargetStatus = infraStatus.Node[nodeStatusInfoIndex].TargetStatus
				infraObj.Node[nodeInfoIndex].TargetAction = infraStatus.Node[nodeStatusInfoIndex].TargetAction
				break
			}
		}
	}

	// add label info for Node
	for i := range infraObj.Node {
		labelInfo, err := label.GetLabels(model.StrNode, infraObj.Node[i].Uid)
		if err != nil {
			log.Error().Err(err).Msg("Cannot get the label info")
			return nil, err
		}
		infraObj.Node[i].Label = labelInfo.Labels
	}

	// add label info
	labelInfo, err := label.GetLabels(model.StrInfra, infraObj.Uid)
	if err != nil {
		log.Error().Err(err).Msg("Cannot get the label info")
		return nil, err
	}
	infraObj.Label = labelInfo.Labels

	// add implicit cluster view synthesized from already-loaded Nodes
	infraObj.Cluster = buildImplicitClusterInfoFromNodes(infraId, infraObj.Node)

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

	if len(infraInfo.Node) == 0 {
		return nil, fmt.Errorf("Infra '%s' has no Nodes to extract configuration from", infraId)
	}

	// Group Nodes by NodeGroupId to reconstruct NodeGroup requests
	nodeGroupMap := make(map[string][]model.NodeInfo)
	var nodeGroupOrder []string
	for _, node := range infraInfo.Node {
		sgId := node.NodeGroupId
		if sgId == "" {
			sgId = node.Id // fallback: treat each Node as its own group
		}
		if _, exists := nodeGroupMap[sgId]; !exists {
			nodeGroupOrder = append(nodeGroupOrder, sgId)
		}
		nodeGroupMap[sgId] = append(nodeGroupMap[sgId], node)
	}

	var nodeGroups []model.CreateNodeGroupDynamicReq
	for _, sgId := range nodeGroupOrder {
		nodes := nodeGroupMap[sgId]
		// Use the first Node in each nodegroup as the representative spec
		rep := nodes[0]
		sg := model.CreateNodeGroupDynamicReq{
			Name:           sgId,
			NodeGroupSize:  len(nodes),
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
		nodeList, err := ListNodeByNodeGroup(nsId, infraId, groupId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return temp, err
		}
		var wg sync.WaitGroup
		chanResults := make(chan model.InfraNodeAccessInfo)

		for _, nodeId := range nodeList {
			// Check if Node is terminated before processing
			nodeObject, err := GetNodeObject(nsId, infraId, nodeId)
			if err != nil {
				log.Debug().Err(err).Msgf("Failed to get VM object for %s, skipping", nodeId)
				continue
			}

			// Skip terminated Nodes as they don't have meaningful access info
			if strings.EqualFold(nodeObject.Status, model.StatusTerminated) {
				log.Debug().Msgf("VM %s is terminated, skipping access info collection", nodeId)
				continue
			}

			wg.Add(1)
			go func(nsId string, infraId string, nodeId string, option string, chanResults chan model.InfraNodeAccessInfo) {
				defer wg.Done()
				common.RandomSleep(0, len(nodeList)/2*1000)
				nodeInfo, err := GetNodeCurrentPublicIp(nsId, infraId, nodeId)

				nodeAccessInfo := model.InfraNodeAccessInfo{}
				if err != nil {
					log.Info().Err(err).Msg("")
					nodeAccessInfo.PublicIP = ""
					nodeAccessInfo.PrivateIP = ""
					nodeAccessInfo.SSHPort = 0
				} else {
					nodeAccessInfo.PublicIP = nodeInfo.PublicIp
					nodeAccessInfo.PrivateIP = nodeInfo.PrivateIp
					nodeAccessInfo.SSHPort = nodeInfo.SSHPort
				}
				nodeAccessInfo.NodeId = nodeId

				nodeObject, err := GetNodeObject(nsId, infraId, nodeId)
				if err != nil {
					log.Info().Err(err).Msg("")
				} else {
					nodeAccessInfo.ConnectionConfig = nodeObject.ConnectionConfig
				}

				_, verifiedUserName, privateKey, err := GetNodeSshKey(nsId, infraId, nodeId)
				if err != nil {
					log.Error().Err(err).Msg("")
					nodeAccessInfo.PrivateKey = ""
					nodeAccessInfo.NodeUserName = ""
				} else {
					if strings.EqualFold(option, "showSshKey") {
						nodeAccessInfo.PrivateKey = privateKey
					}
					nodeAccessInfo.NodeUserName = verifiedUserName
				}

				//nodeAccessInfo.NodeUserPassword
				chanResults <- nodeAccessInfo
			}(nsId, infraId, nodeId, option, chanResults)
		}
		go func() {
			wg.Wait()
			close(chanResults)
		}()
		for result := range chanResults {
			nodeGroupAccessInfo.NodeAccessInfo = append(nodeGroupAccessInfo.NodeAccessInfo, result)
		}

		output.InfraNodeGroupAccessInfo = append(output.InfraNodeGroupAccessInfo, nodeGroupAccessInfo)
	}

	return output, nil
}

// GetInfraNodeAccessInfo is func to retrieve Infra Access information
func GetInfraNodeAccessInfo(nsId string, infraId string, nodeId string, option string) (*model.InfraNodeAccessInfo, error) {

	output := &model.InfraNodeAccessInfo{}

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

	output.NodeId = nodeId

	nodeInfo, err := GetNodeCurrentPublicIp(nsId, infraId, nodeId)

	nodeAccessInfo := &model.InfraNodeAccessInfo{}
	if err != nil {
		log.Info().Err(err).Msg("")
		return output, err
	} else {
		nodeAccessInfo.PublicIP = nodeInfo.PublicIp
		nodeAccessInfo.PrivateIP = nodeInfo.PrivateIp
		nodeAccessInfo.SSHPort = nodeInfo.SSHPort
	}
	nodeAccessInfo.NodeId = nodeId

	nodeObject, err := GetNodeObject(nsId, infraId, nodeId)
	if err != nil {
		log.Info().Err(err).Msg("")
		return output, err
	} else {
		nodeAccessInfo.ConnectionConfig = nodeObject.ConnectionConfig
	}

	_, verifiedUserName, privateKey, err := GetNodeSshKey(nsId, infraId, nodeId)
	if err != nil {
		log.Info().Err(err).Msg("")
		return output, err
	} else {
		if strings.EqualFold(option, "showSshKey") {
			nodeAccessInfo.PrivateKey = privateKey
		}
		nodeAccessInfo.NodeUserName = verifiedUserName
	}

	output = nodeAccessInfo

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

// ListInfraNodeInfo is func to Get all Node Info objects in Infra
func ListInfraNodeInfo(nsId string, infraId string) ([]model.NodeInfo, error) {

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

	// Get Node ID list using existing function
	nodeIdList, err := ListNodeId(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to list Node IDs for Infra %s", infraId)
		return nil, err
	}

	if len(nodeIdList) == 0 {
		return []model.NodeInfo{}, nil
	}

	// Use parallel processing for better performance when dealing with multiple Nodes
	var wg sync.WaitGroup
	chanResults := make(chan model.NodeInfo, len(nodeIdList))

	// Process each Node in parallel, with existence validation
	for _, nodeId := range nodeIdList {
		wg.Add(1)
		go func(nodeId string) {
			defer wg.Done()

			// Check if Node exists first to avoid race conditions during deletion
			nodeKey := common.GenInfraKey(nsId, infraId, nodeId)
			_, exists, err := kvstore.GetKv(nodeKey)
			if err != nil || !exists {
				// Node might be deleted by concurrent operations (e.g., DelInfra)
				// This is normal during Infra deletion process, so use Debug level
				log.Debug().Msgf("VM object not found for nodeId: %s (possibly deleted concurrently)", nodeId)
				return // Skip this Node
			}

			nodeInfo, err := GetNodeObject(nsId, infraId, nodeId)
			if err != nil {
				// Secondary check - Node might have been deleted between existence check and retrieval
				log.Debug().Err(err).Msgf("VM object retrieval failed for nodeId: %s (possibly deleted concurrently)", nodeId)
				return // Skip this Node
			}

			chanResults <- nodeInfo
		}(nodeId)
	}

	// Wait for all goroutines to complete and close the channel
	go func() {
		wg.Wait()
		close(chanResults)
	}()

	// Collect results from the channel
	var nodeInfoList []model.NodeInfo
	for nodeInfo := range chanResults {
		nodeInfoList = append(nodeInfoList, nodeInfo)
	}

	return nodeInfoList, nil
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

	// Use existing ListInfraNodeInfo function instead of manually iterating through Nodes
	nodeInfoList, err := ListInfraNodeInfo(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.InfraInfo{}, false, err
	}

	infraTmp.Node = nodeInfoList

	return infraTmp, true, nil
}

// GetNodeObject is func to get Node object
func GetNodeObject(nsId string, infraId string, nodeId string) (model.NodeInfo, error) {

	nodeTmp := model.NodeInfo{}
	key := common.GenInfraKey(nsId, infraId, nodeId)
	keyValue, exists, err := kvstore.GetKv(key)
	if err != nil {
		err = fmt.Errorf("failed to get GetNodeObject (ID: %s)", key)
		log.Error().Err(err).Msg("")
		return model.NodeInfo{}, err
	}
	if !exists {
		return model.NodeInfo{}, fmt.Errorf("no Node found (ID: %s)", key)
	}

	err = json.Unmarshal([]byte(keyValue.Value), &nodeTmp)
	if err != nil {
		err = fmt.Errorf("failed to get GetNodeObject (ID: %s), message: failed to unmarshal", key)
		log.Error().Err(err).Msg("")
		return model.NodeInfo{}, err
	}
	return nodeTmp, nil
}

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

	// Fetch Node statuses with rate limiting by CSP and region
	nodeStatusList, err := fetchNodeStatusesWithRateLimiting(nsId, infraId, nodeList)
	if err != nil {
		log.Error().Err(err).Msg("")
		return &model.InfraStatusInfo{}, err
	}
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

	statusFlag := []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	statusFlagStr := []string{model.StatusFailed, model.StatusSuspended, model.StatusRunning, model.StatusTerminated, model.StatusCreating, model.StatusSuspending, model.StatusResuming, model.StatusRebooting, model.StatusTerminating, model.StatusRegistering, model.StatusUndefined}
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
		numNode = actualNodeCount
		if statusNodeCount > actualNodeCount {
			numNode = statusNodeCount
		}
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
		numNode = nodeInfoCount
		if actualNodeCount > numNode {
			numNode = actualNodeCount
		}
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
			case model.ActionCreate:
				// Final states: Running, Failed, Terminated, Suspended, Undefined.
				// Undefined means the creation attempt ended without VM identity (Spider 500);
				// it is an orphan candidate handled by action=reconcile, not a pending state.
				if v.Status == model.StatusCreating || v.Status == model.StatusRegistering || v.Status == "" {
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

			if allNodesFailed && infraTargetAction == model.ActionCreate {
				// All Nodes failed during creation - mark Infra as Failed
				log.Error().Msgf("Infra %s: All Nodes failed during creation - setting Infra status to Failed", infraId)
				infraStatus.TargetAction = model.ActionComplete
				infraStatus.TargetStatus = model.StatusComplete // Target was to complete the creation process
				infraStatus.Status = model.StatusFailed         // Actual status is Failed due to Node failures
				infraTmp.TargetAction = model.ActionComplete
				infraTmp.TargetStatus = model.StatusComplete // Target was to complete the creation process
				infraTmp.Status = model.StatusFailed         // Actual status is Failed due to Node failures
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
		clientManager.MediumDuration,
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

const terminatingFailStreakMax = 3

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
								// Debug-level: node may have been deleted concurrently (e.g., by DelInfra).
								log.Debug().Err(err).Msgf("[fetchNodeStatuses] node %s not found (likely deleted concurrently); skipping", nodeId)
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

	// Skip CSP API call if cspResourceName is empty (Node not properly created)
	if nodeInfo.CspResourceName == "" && nodeInfo.TargetAction != model.ActionCreate {
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

	if (nodeInfo.TargetAction != model.ActionCreate && nodeInfo.TargetAction != model.ActionTerminate) && cspResourceName == "" {
		err = fmt.Errorf("cspResourceName is empty (NodeId: %s)", nodeId)
		log.Error().Err(err).Msg("")
		return statusInfo, err
	}

	type statusResponse struct {
		Status string
	}
	callResult := statusResponse{}
	callResult.Status = ""

	if nodeInfo.Status != model.StatusTerminated && cspResourceName != "" {
		// Direct SDK fast path: bypass Spider for CSPs with a registered BatchVMStatusFunc.
		// Benefits: connection pooling, proper retry/backoff, no extra Spider network hop.
		if handler, ok := cspdirect.GetBatchVMStatusHandler(nodeInfo.ConnectionConfig.ProviderName); ok && nodeInfo.CspResourceId != "" {
			sdkCtx := context.WithValue(context.Background(), model.CtxKeyCredentialHolder, nodeInfo.ConnectionConfig.CredentialHolder)
			statuses, sdkErr := handler(sdkCtx, nodeInfo.ConnectionConfig.RegionDetail.RegionName, []string{nodeInfo.CspResourceId})
			if sdkErr == nil {
				if s, ok := statuses[nodeInfo.CspResourceId]; ok {
					callResult.Status = s
				} else {
					callResult.Status = model.StatusUndefined
				}
				goto applyStatus
			}
			log.Warn().Err(sdkErr).Str("provider", nodeInfo.ConnectionConfig.ProviderName).
				Msgf("[FetchNodeStatus] direct SDK failed for %s; falling back to Spider", nodeId)
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
					if streak >= terminatingFailStreakMax {
						terminatingFailStreak.Delete(streakKey)
						log.Info().Msgf("[FetchNodeStatus] Node %s: Spider error for %d consecutive polls (Terminating); promoting to Terminated", nodeId, streak)
						callResult.Status = model.StatusTerminated
					} else {
						terminatingFailStreak.Store(streakKey, streak)
					}
				}
				break
			}
			if callResult.Status != "" {
				// Successful Spider response: reset consecutive failure streak for this node.
				terminatingFailStreak.Delete(nsId + "/" + infraId + "/" + nodeId)
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
	if strings.EqualFold(nodeStatusTmp.TargetAction, model.ActionTerminate) {
		if strings.EqualFold(callResult.Status, model.StatusUndefined) {
			callResult.Status = model.StatusTerminated
		}
		if strings.EqualFold(callResult.Status, model.StatusSuspending) {
			callResult.Status = model.StatusTerminating
		}
		// Terminate API was already issued; if local status is already Terminating
		// but CSP still reports Running (terminate not yet acknowledged), hold Terminating.
		if strings.EqualFold(nodeStatusTmp.Status, model.StatusTerminating) &&
			strings.EqualFold(callResult.Status, model.StatusRunning) {
			log.Debug().Msgf("[FetchNodeStatus] VM %s: CSP returned Running during Terminate (local=Terminating), holding Terminating", nodeId)
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

	// Log status change if status actually changed
	previousStatus := nodeStatusTmp.Status
	nodeStatusTmp.Status = callResult.Status
	if previousStatus != nodeStatusTmp.Status {
		log.Debug().Msgf("[FetchNodeStatus] Node %s: Status changed - %s -> %s (NativeStatus: %s, TargetAction: %s)",
			nodeId, previousStatus, nodeStatusTmp.Status, nodeStatusTmp.NativeStatus, nodeStatusTmp.TargetAction)
	}

	// TODO: Alibaba Undefined status error is not resolved yet.
	// (After Terminate action. "status": "Undefined", "targetStatus": "None", "targetAction": "None")

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
	originalNodeInfo, _ := GetNodeObject(nsId, infraId, nodeId)
	if originalNodeInfo.Status != model.StatusTerminated {
		nodeInfo.Status = nodeStatusTmp.Status
		nodeInfo.TargetAction = nodeStatusTmp.TargetAction
		nodeInfo.TargetStatus = nodeStatusTmp.TargetStatus
		nodeInfo.SystemMessage = nodeStatusTmp.SystemMessage

		if cspResourceName != "" {
			// don't update Node info, if cspResourceName is empty
			UpdateNodeInfo(nsId, infraId, nodeInfo)
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
		err := fmt.Errorf("The node " + nodeId + " does not exist.")
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

// [Update Infra and Node object]

// UpdateInfraInfo is func to update Infra Info (without Node info in Infra)
func UpdateInfraInfo(nsId string, infraInfoData model.InfraInfo) {
	infraInfoMutex.Lock()
	defer infraInfoMutex.Unlock()

	infraInfoData.Node = nil

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

// UpdateNodeInfo is func to update Node Info
func UpdateNodeInfo(nsId string, infraId string, nodeInfoData model.NodeInfo) {
	infraInfoMutex.Lock()
	defer func() {
		infraInfoMutex.Unlock()
	}()

	key := common.GenInfraKey(nsId, infraId, nodeInfoData.Id)

	// Check existence of the key. If no key, no update.
	keyValue, exists, err := kvstore.GetKv(key)
	if !exists || err != nil {
		return
	}

	nodeTmp := model.NodeInfo{}
	json.Unmarshal([]byte(keyValue.Value), &nodeTmp)

	if !reflect.DeepEqual(nodeTmp, nodeInfoData) {
		val, _ := json.Marshal(nodeInfoData)
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
	nodeIdSet := make(map[string]struct{})
	nodeGroupIdSet := make(map[string]struct{})
	cspNodeNameSet := make(map[string]struct{})
	cspNodeIdSet := make(map[string]struct{})

	for _, node := range infraInfo.Node {
		if node.VNetId != "" {
			vNetSet[node.VNetId] = struct{}{}
		}
		if node.CspVNetId != "" {
			cspVNetSet[node.CspVNetId] = struct{}{}
		}
		if node.SubnetId != "" {
			subnetSet[node.SubnetId] = struct{}{}
		}
		if node.CspSubnetId != "" {
			cspSubnetSet[node.CspSubnetId] = struct{}{}
		}
		for _, sg := range node.SecurityGroupIds {
			if sg != "" {
				sgSet[sg] = struct{}{}
			}
		}
		for _, dd := range node.DataDiskIds {
			if dd != "" {
				dataDiskSet[dd] = struct{}{}
			}
		}
		if node.SshKeyId != "" {
			sshKeySet[node.SshKeyId] = struct{}{}
		}
		if node.ImageId != "" {
			imageSet[node.ImageId] = struct{}{}
		}
		if node.SpecId != "" {
			specSet[node.SpecId] = struct{}{}
		}
		if node.ConnectionName != "" {
			connNameSet[node.ConnectionName] = struct{}{}
		}
		if node.ConnectionConfig.ProviderName != "" {
			providerNameSet[node.ConnectionConfig.ProviderName] = struct{}{}
		}
		if node.Id != "" {
			nodeIdSet[node.Id] = struct{}{}
		}
		if node.NodeGroupId != "" {
			nodeGroupIdSet[node.NodeGroupId] = struct{}{}
		}
		if node.CspResourceName != "" {
			cspNodeNameSet[node.CspResourceName] = struct{}{}
		}
		if node.CspResourceId != "" {
			cspNodeIdSet[node.CspResourceId] = struct{}{}
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
		NodeIds:          toSlice(nodeIdSet),
		NodeGroupIds:     toSlice(nodeGroupIdSet),
		CspNodeNames:     toSlice(cspNodeNameSet),
		CspNodeIds:       toSlice(cspNodeIdSet),
	}, nil
}

// ProvisionDataDisk is func to provision DataDisk to Node (create and attach to Node)
func ProvisionDataDisk(ctx context.Context, nsId string, infraId string, nodeId string, u *model.DataDiskNodeReq) (model.NodeInfo, error) {
	node, err := GetNodeObject(nsId, infraId, nodeId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.NodeInfo{}, err
	}

	createDiskReq := model.DataDiskReq{
		Name:           u.Name,
		ConnectionName: node.ConnectionName,
		DiskType:       u.DiskType,
		DiskSize:       u.DiskSize,
		Description:    u.Description,
	}

	newDataDisk, err := resource.CreateDataDisk(ctx, nsId, &createDiskReq, "")
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.NodeInfo{}, err
	}
	retry := 3
	for i := 0; i < retry; i++ {
		nodeInfo, err := AttachDetachDataDisk(nsId, infraId, nodeId, model.AttachDataDisk, newDataDisk.Id, false)
		if err != nil {
			log.Error().Err(err).Msg("")
		} else {
			return nodeInfo, nil
		}
		time.Sleep(5 * time.Second)
	}
	return model.NodeInfo{}, err
}

// AttachDetachDataDisk is func to attach/detach DataDisk to/from Node
func AttachDetachDataDisk(nsId string, infraId string, nodeId string, command string, dataDiskId string, force bool) (model.NodeInfo, error) {
	nodeKey := common.GenInfraKey(nsId, infraId, nodeId)

	// Check existence of the key. If no key, no update.
	keyValue, exists, err := kvstore.GetKv(nodeKey)
	if !exists || err != nil {
		err := fmt.Errorf("Failed to find 'ns/infra/node': %s/%s/%s \n", nsId, infraId, nodeId)
		log.Error().Err(err).Msg("")
		return model.NodeInfo{}, err
	}

	node := model.NodeInfo{}
	json.Unmarshal([]byte(keyValue.Value), &node)

	isInList := common.CheckElement(dataDiskId, node.DataDiskIds)
	if strings.EqualFold(command, model.DetachDataDisk) && !isInList && !force {
		err := fmt.Errorf("Failed to find the dataDisk %s in the attached dataDisk list %v", dataDiskId, node.DataDiskIds)
		log.Error().Err(err).Msg("")
		return model.NodeInfo{}, err
	} else if strings.EqualFold(command, model.AttachDataDisk) && isInList && !force {
		err := fmt.Errorf("The dataDisk %s is already in the attached dataDisk list %v", dataDiskId, node.DataDiskIds)
		log.Error().Err(err).Msg("")
		return model.NodeInfo{}, err
	}

	dataDiskKey := common.GenResourceKey(nsId, model.StrDataDisk, dataDiskId)

	// Check existence of the key. If no key, no update.
	keyValue, exists, err = kvstore.GetKv(dataDiskKey)
	if !exists || err != nil {
		return model.NodeInfo{}, err
	}

	dataDisk := model.DataDiskInfo{}
	json.Unmarshal([]byte(keyValue.Value), &dataDisk)

	client := clientManager.NewHttpClient()
	method := "PUT"
	var callResult interface{}
	//var requestBody interface{}

	requestBody := model.SpiderDiskAttachDetachReqWrapper{
		ConnectionName: node.ConnectionName,
		ReqInfo: model.SpiderDiskAttachDetachReq{
			VMName: node.CspResourceName,
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
		return model.NodeInfo{}, err
	}

	switch command {
	case model.AttachDataDisk:
		node.DataDiskIds = append(node.DataDiskIds, dataDiskId)
		// resource.UpdateAssociatedObjectList(nsId, model.StrDataDisk, dataDiskId, model.StrAdd, nodeKey)
	case model.DetachDataDisk:
		oldDataDiskIds := node.DataDiskIds
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
			return model.NodeInfo{}, err
		} else {
			node.DataDiskIds = newDataDiskIds
		}
	}

	time.Sleep(8 * time.Second)
	method = "GET"
	url = fmt.Sprintf("%s/node/%s", model.SpiderRestUrl, node.CspResourceName)
	requestBodyConnection := model.SpiderConnectionName{
		ConnectionName: node.ConnectionName,
	}
	var callResultSpiderNodeInfo model.SpiderVMInfo

	_, err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBodyConnection),
		&requestBodyConnection,
		&callResultSpiderNodeInfo,
		clientManager.MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return node, err
	}

	// fmt.Printf("in AttachDetachDataDisk(), updatedSpiderNode.DataDiskIIDs: %s", updatedSpiderNode.DataDiskIIDs) // for debug
	node.AddtionalDetails = callResultSpiderNodeInfo.KeyValueList

	UpdateNodeInfo(nsId, infraId, node)

	// Update TB DataDisk object's 'associatedObjects' field
	resource.UpdateAssociatedObjectList(nsId, model.StrDataDisk, dataDiskId, cmdToUpdateAsso, nodeKey)

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
			return node, err
		}

		updatedSpiderDisk := resp.Result().(*resource.SpiderDiskInfo)
		dataDisk.Status = updatedSpiderDisk.Status
		fmt.Printf("dataDisk.Status: %s \n", dataDisk.Status) // for debug
		resource.UpdateResourceObject(nsId, model.StrDataDisk, dataDisk)
	*/

	return node, nil
}

func GetAvailableDataDisks(nsId string, infraId string, nodeId string, option string) (interface{}, error) {
	nodeKey := common.GenInfraKey(nsId, infraId, nodeId)

	// Check existence of the key. If no key, no update.
	keyValue, exists, err := kvstore.GetKv(nodeKey)
	if !exists || err != nil {
		err := fmt.Errorf("Failed to find 'ns/infra/node': %s/%s/%s \n", nsId, infraId, nodeId)
		log.Error().Err(err).Msg("")
		return nil, err
	}

	node := model.NodeInfo{}
	json.Unmarshal([]byte(keyValue.Value), &node)

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

			if v.ConnectionName == node.ConnectionName && tempObj.Status == "Available" {
				idList = append(idList, v.Id)
			}
		}

		return idList, nil
	}
}

// [Delete Infra and Node object]

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
			// Wait until all Nodes leave the Terminating state.
			// StatusAgent (PollHigh = 15 s for Terminating nodes) updates StatusStore
			// as CSP propagates each termination. We read StatusStore directly instead
			// of calling GetInfraStatus (which fans out to 1300 CSP SDK calls every 5 s
			// and causes OOM at scale).
			const terminateWaitInterval = 5 * time.Second
			const terminateWaitTimeout = 10 * time.Minute
			log.Info().Msgf("[DelInfra] Waiting for Infra %s termination to propagate (polling StatusStore every %s, timeout %s)",
				infraId, terminateWaitInterval, terminateWaitTimeout)
			deadline := time.Now().Add(terminateWaitTimeout)
			for time.Now().Before(deadline) {
				time.Sleep(terminateWaitInterval)
				stillTerminating := false
				for _, e := range globalStatusStore.Snapshot() {
					if e.NsId != nsId || e.InfraId != infraId {
						continue
					}
					if strings.EqualFold(e.Status, model.StatusTerminating) {
						stillTerminating = true
						break
					}
				}
				if !stillTerminating {
					break
				}
				log.Debug().Msgf("[DelInfra] Infra %s: some nodes still Terminating — waiting", infraId)
			}
			// Re-read for the status-check below.
			infraStatus, _ = GetInfraStatus(nsId, infraId)
			if infraStatus != nil && strings.Contains(infraStatus.Status, model.StatusTerminating) {
				log.Warn().Msgf("[DelInfra] Infra %s still %s after %s — proceeding with deletion anyway",
					infraId, infraStatus.Status, terminateWaitTimeout)
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

	nodeList, err := ListNodeId(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return deletedResources, err
	}

	// delete nodes info
	type nodeEntry struct {
		id   string
		key  string
		info model.NodeInfo
	}

	// Step 1: fetch all node infos in parallel
	entries := make([]nodeEntry, len(nodeList))
	fetchErrs := make([]error, len(nodeList))
	var fetchWg sync.WaitGroup
	for i, v := range nodeList {
		fetchWg.Add(1)
		go func(i int, v string) {
			defer fetchWg.Done()
			nodeKey := common.GenInfraKey(nsId, infraId, v)
			nodeInfo, err := GetNodeObject(nsId, infraId, v)
			entries[i] = nodeEntry{id: v, key: nodeKey, info: nodeInfo}
			fetchErrs[i] = err
		}(i, v)
	}
	fetchWg.Wait()
	for _, err := range fetchErrs {
		if err != nil {
			log.Error().Err(err).Msg("")
			return deletedResources, err
		}
	}

	// Step 2: delete kvstore entries and status store in parallel
	// Remove from StatusStore before etcd so StatusAgent cannot dispatch a node
	// between the etcd deletion and the StatusStore cleanup.
	deleteErrs := make([]error, len(entries))
	var deleteWg sync.WaitGroup
	for i, e := range entries {
		deleteWg.Add(1)
		go func(i int, e nodeEntry) {
			defer deleteWg.Done()
			globalStatusStore.Delete(nsId, infraId, e.id)
			deleteErrs[i] = kvstore.Delete(e.key)
		}(i, e)
	}
	deleteWg.Wait()
	for _, err := range deleteErrs {
		if err != nil {
			log.Error().Err(err).Msg("")
			return deletedResources, err
		}
	}

	// Step 3: batch-remove associated object lists — one read-modify-write per resource
	// instead of N round-trips for N nodes sharing the same resource.
	type resourceRef struct {
		resourceType string
		resourceId   string
	}
	assocMap := make(map[resourceRef][]string)
	for _, e := range entries {
		add := func(rType, rId string) {
			if rId != "" {
				ref := resourceRef{rType, rId}
				assocMap[ref] = append(assocMap[ref], e.key)
			}
		}
		// Try both Image and CustomImage; BatchRemoveFromAssociatedObjectList silently
		// skips keys not present, so the non-matching type is a no-op.
		add(model.StrImage, e.info.ImageId)
		add(model.StrCustomImage, e.info.ImageId)
		add(model.StrSSHKey, e.info.SshKeyId)
		add(model.StrVNet, e.info.VNetId)
		for _, sgId := range e.info.SecurityGroupIds {
			add(model.StrSecurityGroup, sgId)
		}
		for _, ddId := range e.info.DataDiskIds {
			add(model.StrDataDisk, ddId)
		}
	}
	var batchWg sync.WaitGroup
	for ref, keys := range assocMap {
		batchWg.Add(1)
		go func(ref resourceRef, keys []string) {
			defer batchWg.Done()
			if err := resource.BatchRemoveFromAssociatedObjectList(nsId, ref.resourceType, ref.resourceId, keys); err != nil {
				log.Warn().Err(err).Msgf("BatchRemoveFromAssociatedObjectList failed for %s/%s", ref.resourceType, ref.resourceId)
			}
		}(ref, keys)
	}
	batchWg.Wait()

	// Step 4: delete labels in parallel
	var labelWg sync.WaitGroup
	for _, e := range entries {
		labelWg.Add(1)
		go func(e nodeEntry) {
			defer labelWg.Done()
			if err := label.DeleteLabelObject(model.StrNode, e.info.Uid); err != nil {
				log.Error().Err(err).Msg("")
			}
		}(e)
		deletedResources.IdList = append(deletedResources.IdList, deleteStatus+"Node: "+e.id)
	}
	labelWg.Wait()

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

// DelInfraNode is func to delete Node object
func DelInfraNode(nsId string, infraId string, nodeId string, option string) error {

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

	err = common.CheckString(nodeId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	check, _ := CheckNode(nsId, infraId, nodeId)

	if !check {
		err := fmt.Errorf("The node " + nodeId + " does not exist.")
		return err
	}

	log.Debug().Msg("Deleting VM " + nodeId)

	// skip termination if option is force
	if option != "force" {
		// ControlNode first
		_, err := HandleInfraNodeAction(nsId, infraId, nodeId, model.ActionTerminate, false)
		if err != nil {
			log.Info().Msg(err.Error())
			return err
		}
		// for deletion, need to wait until termination is finished
		log.Info().Msg("Wait for Node termination in 1 second")
		time.Sleep(1 * time.Second)

	}

	// get node info
	nodeInfo, _ := GetNodeObject(nsId, infraId, nodeId)

	// delete nodes info
	key := common.GenInfraKey(nsId, infraId, nodeId)
	err = kvstore.Delete(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	globalStatusStore.Delete(nsId, infraId, nodeId)

	// remove empty NodeGroups
	nodeGroup, err := ListNodeGroupId(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list nodeGroup to remove")
		return err
	}
	for _, v := range nodeGroup {
		nodeListInNodeGroup, err := ListNodeByNodeGroup(nsId, infraId, v)
		if err != nil {
			log.Error().Err(err).Msg("Failed to list node in nodeGroup to remove")
			return err
		}
		if len(nodeListInNodeGroup) == 0 {
			nodeGroupKey := common.GenInfraNodeGroupKey(nsId, infraId, v)
			err := kvstore.Delete(nodeGroupKey)
			if err != nil {
				log.Error().Err(err).Msg("Failed to remove the empty nodeGroup")
				return err
			}
		}
	}

	_, err = resource.UpdateAssociatedObjectList(nsId, model.StrImage, nodeInfo.ImageId, model.StrDelete, key)
	if err != nil {
		resource.UpdateAssociatedObjectList(nsId, model.StrCustomImage, nodeInfo.ImageId, model.StrDelete, key)
	}

	//resource.UpdateAssociatedObjectList(nsId, model.StrSpec, nodeInfo.SpecId, model.StrDelete, key)
	resource.UpdateAssociatedObjectList(nsId, model.StrSSHKey, nodeInfo.SshKeyId, model.StrDelete, key)
	resource.UpdateAssociatedObjectList(nsId, model.StrVNet, nodeInfo.VNetId, model.StrDelete, key)

	for _, v := range nodeInfo.SecurityGroupIds {
		resource.UpdateAssociatedObjectList(nsId, model.StrSecurityGroup, v, model.StrDelete, key)
	}

	for _, v := range nodeInfo.DataDiskIds {
		resource.UpdateAssociatedObjectList(nsId, model.StrDataDisk, v, model.StrDelete, key)
	}

	err = label.DeleteLabelObject(model.StrNode, nodeInfo.Uid)
	if err != nil {
		log.Error().Err(err).Msg("")
	}

	return nil
}

// DeregisterInfraNode deregisters Node from Spider and TB without deleting the actual CSP resource
// This function only removes the Node mapping from Spider and TB internal storage
// The actual CSP Node resource remains intact and can be re-registered later
func DeregisterInfraNode(nsId string, infraId string, nodeId string) error {

	log.Debug().Msg("[Deregister VM] " + nodeId)

	// get node info
	nodeInfo, err := GetNodeObject(nsId, infraId, nodeId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	// Check if associated resources exist before deregistration
	var relatedResources []string

	// Check DataDisks
	for _, dataDiskId := range nodeInfo.DataDiskIds {
		exists, _ := resource.CheckResource(nsId, model.StrDataDisk, dataDiskId)
		if exists {
			relatedResources = append(relatedResources, fmt.Sprintf("DataDisk: %s", dataDiskId))
		}
	}

	// Check SecurityGroups
	for _, sgId := range nodeInfo.SecurityGroupIds {
		exists, _ := resource.CheckResource(nsId, model.StrSecurityGroup, sgId)
		if exists {
			relatedResources = append(relatedResources, fmt.Sprintf("SecurityGroup: %s", sgId))
		}
	}

	// Check SSHKey
	if nodeInfo.SshKeyId != "" {
		exists, _ := resource.CheckResource(nsId, model.StrSSHKey, nodeInfo.SshKeyId)
		if exists {
			relatedResources = append(relatedResources, fmt.Sprintf("SSHKey: %s", nodeInfo.SshKeyId))
		}
	}

	// If any resources are missing, return error
	if len(relatedResources) > 0 {
		err := fmt.Errorf("cannot deregister VM '%s': the following associated resources do not exist: %v", nodeId, relatedResources)
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
		ConnectionName: nodeInfo.ConnectionName,
	}

	url := model.SpiderRestUrl + "/regvm/" + nodeInfo.CspResourceName
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

	// delete the Node info from TB
	key := common.GenInfraKey(nsId, infraId, nodeId)
	err = kvstore.Delete(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	globalStatusStore.Delete(nsId, infraId, nodeId)

	// remove empty NodeGroups
	nodeListInNodeGroup, err := ListNodeByNodeGroup(nsId, infraId, nodeInfo.NodeGroupId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list node in nodeGroup to remove")
		return err
	}
	if len(nodeListInNodeGroup) == 0 {
		nodeGroupKey := common.GenInfraNodeGroupKey(nsId, infraId, nodeInfo.NodeGroupId)
		err := kvstore.Delete(nodeGroupKey)
		if err != nil {
			log.Error().Err(err).Msg("Failed to remove the empty nodeGroup")
			return err
		}
	}

	resource.UpdateAssociatedObjectList(nsId, model.StrSSHKey, nodeInfo.SshKeyId, model.StrDelete, key)
	resource.UpdateAssociatedObjectList(nsId, model.StrVNet, nodeInfo.VNetId, model.StrDelete, key)

	for _, v := range nodeInfo.SecurityGroupIds {
		resource.UpdateAssociatedObjectList(nsId, model.StrSecurityGroup, v, model.StrDelete, key)
	}

	for _, v := range nodeInfo.DataDiskIds {
		resource.UpdateAssociatedObjectList(nsId, model.StrDataDisk, v, model.StrDelete, key)
	}

	err = label.DeleteLabelObject(model.StrNode, nodeInfo.Uid)
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

// UpdateNodePublicIp is func to update Node public IP
func UpdateNodePublicIp(nsId string, infraId string, nodeInfoData model.NodeInfo) error {

	nodeInfoTmp, err := GetNodeCurrentPublicIp(nsId, infraId, nodeInfoData.Id)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	if nodeInfoData.PublicIP != nodeInfoTmp.PublicIp || nodeInfoData.SSHPort != nodeInfoTmp.SSHPort {
		nodeInfoData.PublicIP = nodeInfoTmp.PublicIp
		nodeInfoData.SSHPort = nodeInfoTmp.SSHPort
		UpdateNodeInfo(nsId, infraId, nodeInfoData)
	}
	return nil
}

// GetNodeTemplate is func to get Node template
func GetNodeTemplate(nsId string, infraId string, algo string) (model.NodeInfo, error) {

	log.Debug().Msg("[GetNodeTemplate]" + infraId + " by algo: " + algo)

	nodeList, err := ListNodeId(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.NodeInfo{}, err
	}
	if len(nodeList) == 0 {
		return model.NodeInfo{}, nil
	}

	rand.Seed(time.Now().UnixNano())
	index := rand.Intn(len(nodeList))
	nodeObj, nodeErr := GetNodeObject(nsId, infraId, nodeList[index])
	var nodeTemplate model.NodeInfo

	// only take template required to create Node
	nodeTemplate.Name = nodeObj.Name
	nodeTemplate.ConnectionName = nodeObj.ConnectionName
	nodeTemplate.ImageId = nodeObj.ImageId
	nodeTemplate.SpecId = nodeObj.SpecId
	nodeTemplate.VNetId = nodeObj.VNetId
	nodeTemplate.SubnetId = nodeObj.SubnetId
	nodeTemplate.SecurityGroupIds = nodeObj.SecurityGroupIds
	nodeTemplate.SshKeyId = nodeObj.SshKeyId
	nodeTemplate.NodeUserName = nodeObj.NodeUserName
	nodeTemplate.NodeUserPassword = nodeObj.NodeUserPassword

	if nodeErr != nil {
		log.Error().Err(err).Msg("")
		return model.NodeInfo{}, nodeErr
	}

	return nodeTemplate, nil

}
