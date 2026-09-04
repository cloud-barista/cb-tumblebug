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
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/apierr"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
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
// splitNodeKey returns the Infra id and Node id of a key shaped
// "/ns/{nsId}/infra/{infraId}/node/{nodeId}", or false for any other key.
func splitNodeKey(key, nsPrefix string) (infraId, nodeId string, ok bool) {
	parts := strings.Split(strings.TrimPrefix(key, nsPrefix), "/")
	if len(parts) != 3 || parts[1] != model.StrNode {
		return "", "", false
	}
	return parts[0], parts[2], true
}

// ListNodeAllInNs returns every Node in the namespace, each tagged with the Infra it
// belongs to. Nodes live under their Infra, so this scans the Infra subtree once
// instead of listing Infras and querying each of them.
func ListNodeAllInNs(nsId string, filterKey string, filterVal string) ([]model.NodeInfoInNs, error) {
	if err := common.CheckString(nsId); err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	nsPrefix := fmt.Sprintf("/"+model.StrNamespace+"/%s/"+model.StrInfra+"/", nsId)
	keyValue, err := kvstore.GetKvList(nsPrefix)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	res := []model.NodeInfoInNs{}
	for _, v := range keyValue {
		infraId, _, ok := splitNodeKey(v.Key, nsPrefix)
		if !ok {
			continue
		}
		// Same filter semantics as the other listings: both terms must appear
		if filterKey != "" {
			value := strings.ToLower(v.Value)
			if !(strings.Contains(value, strings.ToLower(filterKey)) && strings.Contains(value, strings.ToLower(filterVal))) {
				continue
			}
		}
		tempObj := model.NodeInfo{}
		if err := json.Unmarshal([]byte(v.Value), &tempObj); err != nil {
			log.Error().Err(err).Str("key", v.Key).Msg("Cannot read the Node object")
			return nil, err
		}
		res = append(res, model.NodeInfoInNs{InfraId: infraId, NodeInfo: tempObj})
	}
	return res, nil
}

// ListNodeIdAllInNs returns "{infraId}/{nodeId}" for every Node in the namespace.
// The Infra id is part of the id because a Node id is only unique within its Infra.
func ListNodeIdAllInNs(nsId string) ([]string, error) {
	if err := common.CheckString(nsId); err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	nsPrefix := fmt.Sprintf("/"+model.StrNamespace+"/%s/"+model.StrInfra+"/", nsId)
	keyValue, err := kvstore.GetKvList(nsPrefix)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	idList := []string{}
	for _, v := range keyValue {
		if infraId, nodeId, ok := splitNodeKey(v.Key, nsPrefix); ok {
			idList = append(idList, infraId+"/"+nodeId)
		}
	}
	return idList, nil
}

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
		if after, ok := strings.CutPrefix(k, nodePrefix); ok {
			trimmedString := after
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
		err := fmt.Errorf("Not found the Infra: %s from the NS: %s", infraId, nsId)
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

// removeNodeFromNodeGroupRecord drops a deleted Node from its NodeGroup record, so the
// record does not keep over-counting. Only that Node is removed: names reserved for
// Nodes still being created must stay listed, or they could be handed out again.
func removeNodeFromNodeGroupRecord(nsId, infraId, nodeGroupId, nodeId string, remainingNodeIds []string) {
	nodeGroupInfo, err := GetNodeGroup(nsId, infraId, nodeGroupId)
	if err != nil {
		return
	}
	kept := []string{}
	for _, id := range nodeGroupInfo.NodeId {
		if id != nodeId {
			kept = append(kept, id)
		}
	}
	for _, id := range remainingNodeIds {
		if !contains(kept, id) {
			kept = append(kept, id)
		}
	}
	nodeGroupInfo.NodeId = kept
	nodeGroupInfo.NodeGroupSize = len(kept)
	val, err := json.Marshal(nodeGroupInfo)
	if err != nil {
		return
	}
	if err := kvstore.Put(common.GenInfraNodeGroupKey(nsId, infraId, nodeGroupId), string(val)); err != nil {
		log.Warn().Err(err).Msgf("Failed to update the NodeGroup record of %s", nodeGroupId)
	}
}

// GetNodeGroup is func to return list of NodeGroups in a given Infra
func GetNodeGroup(nsId string, infraId string, nodeGroupId string) (model.NodeGroupInfo, error) {
	nodeGroupInfo := model.NodeGroupInfo{}

	key := common.GenInfraNodeGroupKey(nsId, infraId, nodeGroupId)
	keyValue, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nodeGroupInfo, err
	}
	// A missing key returns an empty value; without this check json.Unmarshal("")
	// below fails and a not-found NodeGroup is misreported as a corrupted one.
	if !exists {
		return nodeGroupInfo, fmt.Errorf("no NodeGroup found (Key: %s)", key)
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

// GetInfraInfo is func to return Infra information with the current status update
// nodeSummaryFromEntry builds a NodeSummary (status + immutable config, no
// label/commandStatus/details) from a StatusStore entry, for the list view.
func nodeSummaryFromEntry(e StatusEntry) model.NodeSummary {
	return model.NodeSummary{
		Id:              e.NodeId,
		Name:            e.Name,
		CspResourceName: e.CspResourceName,
		CspResourceId:   e.CspResourceId,
		ConnectionName:  e.ConnectionName,
		// Full connection config, region and labels carried from the store's cached
		// static fields. Heavy/derived/sensitive fields (commandStatus,
		// sshHostKeyInfo, nodeUserPassword, addtionalDetails) are absent from
		// NodeSummary by design — see its doc.
		ConnectionConfig: e.ConnectionConfig,
		Region:           e.RegionInfo,
		Label:            e.Label,
		SpecId:           e.SpecId,
		ImageId:          e.ImageId,
		Spec:             e.Spec,
		Image:            e.Image,
		CspSpecName:      e.CspSpecName,
		CspImageName:     e.CspImageName,
		VNetId:           e.VNetId,
		CspVNetId:        e.CspVNetId,
		SubnetId:         e.SubnetId,
		CspSubnetId:      e.CspSubnetId,
		NetworkInterface: e.NetworkInterface,
		SecurityGroupIds: e.SecurityGroupIds,
		SshKeyId:         e.SshKeyId,
		CspSshKeyId:      e.CspSshKeyId,
		DataDiskIds:      e.DataDiskIds,
		RootDiskType:     e.RootDiskType,
		RootDiskSize:     e.RootDiskSize,
		NodeGroupId:      e.NodeGroupId,
		Uid:              e.Uid,
		ResourceType:     e.ResourceType,
		PublicDNS:        e.PublicDNS,
		PrivateDNS:       e.PrivateDNS,
		Status:           e.Status,
		TargetStatus:     e.TargetStatus,
		TargetAction:     e.TargetAction,
		PublicIP:         e.PublicIP,
		PrivateIP:        e.PrivateIP,
		SSHPort:          e.SSHPort,
		Location:         e.Location,
		MonAgentStatus:   e.MonAgentStatus,
		CreatedTime:      e.CreatedTime,
		SystemMessage:    e.SystemMessage,
	}
}

// GetInfraInfoBrief returns an InfraInfoSummary served from the StatusAgent's
// in-memory store: the Infra record (read once, no per-Node object reads) plus
// per-Node status/config from the store. It avoids GetInfraInfo's ~2N etcd reads
// (a full Node object read + a label read per Node), which flood etcd when clients
// poll the Infra list. Nodes are NodeSummary (status + immutable config, no per-Node
// label/commandStatus/details); the full per-Node object is on the single-Node API.
func GetInfraInfoBrief(nsId string, infraId string) (*model.InfraInfoSummary, error) {
	keyValue, exists, err := kvstore.GetKv(common.GenInfraKey(nsId, infraId, ""))
	if err != nil || !exists {
		return nil, fmt.Errorf("the infra %s does not exist", infraId)
	}
	// Infra-level fields share NodeSummary-independent json tags with the stored
	// Infra object, so unmarshalling straight into the summary fills them; the
	// object's empty embedded node array is then replaced with store-backed nodes.
	summary := model.InfraInfoSummary{}
	if err := json.Unmarshal([]byte(keyValue.Value), &summary); err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	if status, serr := GetInfraStatus(nsId, infraId); serr == nil {
		summary.Status = status.Status
		summary.StatusCount = status.StatusCount
	}

	var nodes []model.NodeSummary
	for _, e := range globalStatusStore.Snapshot() {
		if e.NsId == nsId && e.InfraId == infraId {
			nodes = append(nodes, nodeSummaryFromEntry(e))
		}
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Id < nodes[j].Id })
	summary.Node = nodes
	return &summary, nil
}

func GetInfraInfo(nsId string, infraId string) (*model.InfraInfo, error) {

	check, _ := CheckInfra(nsId, infraId)

	if !check {
		temp := &model.InfraInfo{}
		err := fmt.Errorf("The infra %s does not exist.", infraId)
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

	sort.Slice(infraObj.Node, func(i, j int) bool {
		return infraObj.Node[i].Id < infraObj.Node[j].Id
	})

	// Build a lookup map from node ID to status info.
	// Avoids index-out-of-range when autopilot goroutines are concurrently appending
	// nodes: a separate ListNodeId call could return N+k IDs while infraObj.Node still
	// has only N entries, causing infraObj.Node[N] to panic.
	nodeStatusById := make(map[string]model.NodeStatusInfo, len(infraStatus.Node))
	for _, ns := range infraStatus.Node {
		nodeStatusById[ns.Id] = ns
	}
	for i := range infraObj.Node {
		if ns, ok := nodeStatusById[infraObj.Node[i].Id]; ok {
			infraObj.Node[i].Status = ns.Status
			infraObj.Node[i].TargetStatus = ns.TargetStatus
			infraObj.Node[i].TargetAction = ns.TargetAction
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
		PostCommands:    infraInfo.PostCommands,
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
		err := fmt.Errorf("The infra %s does not exist.", infraId)
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
	// Groups are gathered concurrently; each node in them costs a live CSP round trip.
	groupResults := make([]model.InfraNodeGroupAccessInfo, len(nodeGroupList))
	groupErrs := make([]error, len(nodeGroupList))
	var groupWg sync.WaitGroup

	for groupIndex, groupId := range nodeGroupList {
		groupWg.Add(1)
		go func(groupIndex int, groupId string) {
			defer groupWg.Done()

			nodeGroupAccessInfo := model.InfraNodeGroupAccessInfo{}
			nodeGroupAccessInfo.NodeGroupId = groupId
			nlb, err := GetNLB(nsId, infraId, groupId)
			if err == nil {
				nodeGroupAccessInfo.NlbListener = &nlb.Listener
			}
			nodeList, err := ListNodeByNodeGroup(nsId, infraId, groupId)
			if err != nil {
				log.Error().Err(err).Msg("")
				groupErrs[groupIndex] = err
				return
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

					userName, verifiedUserName, privateKey, err := GetNodeSshKey(nsId, infraId, nodeId)
					if err != nil {
						log.Error().Err(err).Msg("")
						nodeAccessInfo.PrivateKey = ""
						nodeAccessInfo.NodeUserName = ""
					} else {
						if strings.EqualFold(option, "showSshKey") {
							nodeAccessInfo.PrivateKey = privateKey
						}
						nodeAccessInfo.NodeUserName = ResolveSshUserName(verifiedUserName, userName)
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

			groupResults[groupIndex] = nodeGroupAccessInfo
		}(groupIndex, groupId)
	}
	groupWg.Wait()

	for i, groupErr := range groupErrs {
		if groupErr != nil {
			return temp, groupErr
		}
		output.InfraNodeGroupAccessInfo = append(output.InfraNodeGroupAccessInfo, groupResults[i])
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

	userName, verifiedUserName, privateKey, err := GetNodeSshKey(nsId, infraId, nodeId)
	if err != nil {
		log.Info().Err(err).Msg("")
		return output, err
	} else {
		if strings.EqualFold(option, "showSshKey") {
			nodeAccessInfo.PrivateKey = privateKey
		}
		nodeAccessInfo.NodeUserName = ResolveSshUserName(verifiedUserName, userName)
	}

	output = nodeAccessInfo

	return output, nil
}

// ListInfraInfo is func to get all Infra objects as list-view summaries
func ListInfraInfo(nsId string, option string) ([]model.InfraInfoSummary, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	Infra := []model.InfraInfoSummary{}

	infraList, err := ListInfraId(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	for _, v := range infraList {

		// Serve the list from the in-memory store (status-level nodes, no per-Node
		// etcd object/label reads) so polling clients (e.g. mapui) do not flood etcd.
		infraTmp, err := GetInfraInfoBrief(nsId, v)
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

// [Update Infra and Node object]

// UpdateInfraInfo is func to update Infra Info (without Node info in Infra)
func UpdateInfraInfo(nsId string, infraInfoData model.InfraInfo) {
	// An empty Id collapses GenInfraKey onto the namespace key; reject to avoid
	// writing a stray/empty infra object.
	if infraInfoData.Id == "" {
		log.Warn().Msgf("UpdateInfraInfo skipped: empty Infra Id (ns %s)", nsId)
		return
	}
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

	// Note: Using reflect.DeepEqual for performance optimization to avoid unnecessary kvstore writes
	// The static analysis warning about errors is acceptable in this context
	if !reflect.DeepEqual(infraTmp, infraInfoData) {
		val, _ := json.Marshal(infraInfoData)
		err = kvstore.Put(key, string(val))
		if err != nil {
			log.Error().Err(err).Msg("")
		}
	}
}

// putNodeDetails stores a Node's auxiliary details (CSP raw metadata) under a
// separate key so they are never carried by status/bulk Node reads and writes.
func putNodeDetails(nsId, infraId, nodeId string, details []model.KeyValue) {
	if nodeId == "" || len(details) == 0 {
		return
	}
	val, err := json.Marshal(details)
	if err != nil {
		log.Error().Err(err).Msg("")
		return
	}
	if err := kvstore.Put(common.GenInfraNodeDetailsKey(nsId, infraId, nodeId), string(val)); err != nil {
		log.Error().Err(err).Msg("")
	}
}

// GetNodeDetails returns a Node's auxiliary details from the separate details
// key, or nil when none are stored.
func GetNodeDetails(nsId, infraId, nodeId string) []model.KeyValue {
	keyValue, exists, err := kvstore.GetKv(common.GenInfraNodeDetailsKey(nsId, infraId, nodeId))
	if err != nil || !exists {
		return nil
	}
	var details []model.KeyValue
	if err := json.Unmarshal([]byte(keyValue.Value), &details); err != nil {
		log.Error().Err(err).Msg("")
		return nil
	}
	return details
}

// AttachNodeDetails fills AddtionalDetails for each Node from the separate
// details key. Read APIs call this only when details are explicitly requested,
// so default/bulk responses stay small.
func AttachNodeDetails(nsId, infraId string, nodes []model.NodeInfo) {
	for i := range nodes {
		if d := GetNodeDetails(nsId, infraId, nodes[i].Id); d != nil {
			nodes[i].AddtionalDetails = d
		}
	}
}

// UpdateNodeInfo is func to update Node Info
func UpdateNodeInfo(nsId string, infraId string, nodeInfoData model.NodeInfo) {
	// An empty node Id collapses GenInfraKey onto the parent infra key, so this
	// write would overwrite the infra object with node data. Refuse it.
	if nodeInfoData.Id == "" {
		log.Warn().Msgf("UpdateNodeInfo skipped: empty node Id (Infra %s/%s)", nsId, infraId)
		return
	}
	// Store auxiliary details separately and strip them from the Node record.
	// nodeInfoData is a value copy, so this does not affect the caller.
	if len(nodeInfoData.AddtionalDetails) > 0 {
		putNodeDetails(nsId, infraId, nodeInfoData.Id, nodeInfoData.AddtionalDetails)
		nodeInfoData.AddtionalDetails = nil
	}
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

// [Delete Infra and Node object]

// DelInfra is func to delete Infra object
// describePotentialOrphans lists nodes that are not Terminated yet, i.e. CSP
// resources that force deletion would leave behind (billing + dependency locks).
func describePotentialOrphans(infraInfo *model.InfraInfo) string {
	if infraInfo == nil {
		return "CB-TB metadata was removed without confirming CSP termination"
	}
	alive := []string{}
	for _, node := range infraInfo.Node {
		if !strings.Contains(node.Status, model.StatusTerminated) && node.CspResourceId != "" {
			alive = append(alive, fmt.Sprintf("%s(%s,%s)", node.Id, node.CspResourceId, node.Status))
		}
	}
	if len(alive) == 0 {
		return ""
	}
	return fmt.Sprintf("%d node(s) may remain on the CSP: %s", len(alive), strings.Join(alive, ", "))
}

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
		err := fmt.Errorf("Infra %s status is unavailable, so deletion is not allowed. Use option=terminate to refine and terminate it before deletion; avoid option=force unless you must — it can leave orphaned (billable) CSP resources", infraId)
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
			// Scale the timeout with the number of nodes: allow ~6 s per node,
			// with a floor of 10 min and a ceiling of 60 min.
			// Example: 100 nodes → 10 min, 600 nodes → 60 min, 975 nodes → 60 min.
			nodeCount := infraStatus.StatusCount.CountTotal
			terminateWaitTimeout := time.Duration(max(10, nodeCount/10)) * time.Minute
			if terminateWaitTimeout > 60*time.Minute {
				terminateWaitTimeout = 60 * time.Minute
			}
			log.Info().Msgf("[DelInfra] Waiting for Infra %s termination to propagate (polling StatusStore every %s, timeout %s, nodes %d)",
				infraId, terminateWaitInterval, terminateWaitTimeout, nodeCount)
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
			err = fmt.Errorf("Infra %s is %s, which is not directly deletable. Use option=terminate to safely clean it up: it refines Failed/Undefined nodes (terminating any that still exist on the CSP) and terminates the rest before deletion. Avoid option=force unless you must — it drops the CB-TB records without terminating the CSP resources and can leave billable orphans", infraId, infraStatus.Status)
		}
		log.Error().Err(err).Msg("")
		if option != "force" {
			return deletedResources, err
		}
		// option=force removes CB-TB metadata without waiting for CSP termination:
		// anything still alive on the CSP becomes an orphan (keeps billing and blocks
		// VNet/SecurityGroup deletion). Warn loudly and report it to the caller.
		if orphanWarning := describePotentialOrphans(infraInfo); orphanWarning != "" {
			log.Warn().Msgf("Force deletion of Infra '%s': %s", infraId, orphanWarning)
			deletedResources.IdList = append(deletedResources.IdList,
				"[WARNING] "+orphanWarning+" — verify with POST /tumblebug/inspectResources (resourceType: node) and terminate leftovers")
		}
	}

	key := common.GenInfraKey(nsId, infraId, "")

	// CSP truth guard: never drop records while VMs of this infra are still alive at the CSP
	if gerr := guardOrphansBeforeDelete(nsId, infraId, option); gerr != nil {
		log.Error().Err(gerr).Msg("")
		return deletedResources, gerr
	}

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
			kvstore.Delete(common.GenInfraNodeDetailsKey(nsId, infraId, e.id))
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
		// Read only to recover the Uid for label cleanup. A NodeGroup we cannot read
		// (already gone or corrupted) must still be deleted — that is the goal here —
		// so never let a read failure block the Infra deletion.
		nodeGroupInfo, gerr := GetNodeGroup(nsId, infraId, v)

		if err = kvstore.Delete(nodeGroupKey); err != nil {
			log.Error().Err(err).Msg("")
			return deletedResources, err
		}
		deletedResources.IdList = append(deletedResources.IdList, deleteStatus+"NodeGroup: "+v)

		if gerr != nil {
			log.Warn().Err(gerr).Msgf("NodeGroup %s unreadable during delete; deleted its key, skipping label cleanup", v)
			continue
		}
		if err = label.DeleteLabelObject(model.StrNodeGroup, nodeGroupInfo.Uid); err != nil {
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
		err := fmt.Errorf("The node %s does not exist.", nodeId)
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

		// Re-verify the node actually reached a terminal state before removing its
		// metadata, instead of assuming a fixed sleep is enough.
		const maxDeleteWaitAttempts = 3
		const deleteWaitInterval = 3 * time.Second
		var status model.NodeStatusInfo
		safeToDelete := false
		for attempt := 0; attempt < maxDeleteWaitAttempts; attempt++ {
			if attempt > 0 {
				time.Sleep(deleteWaitInterval)
			}
			fetched, fetchErr := FetchNodeStatus(nsId, infraId, nodeId)
			if fetchErr != nil {
				if strings.Contains(fetchErr.Error(), "temporarily blocked") {
					continue
				}
				safeToDelete = true
				break
			}
			status = fetched
			if status.Status == model.StatusTerminated || status.Status == model.StatusUndefined || status.Status == model.StatusFailed {
				safeToDelete = true
				break
			}
		}
		if !safeToDelete {
			return fmt.Errorf("node %s not yet confirmed terminated (status=%s); retry shortly", nodeId, status.Status)
		}
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
	kvstore.Delete(common.GenInfraNodeDetailsKey(nsId, infraId, nodeId))

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
		nodeGroupKey := common.GenInfraNodeGroupKey(nsId, infraId, v)
		if len(nodeListInNodeGroup) == 0 {
			err := kvstore.Delete(nodeGroupKey)
			if err != nil {
				log.Error().Err(err).Msg("Failed to remove the empty nodeGroup")
				return err
			}
			continue
		}
		if v == nodeInfo.NodeGroupId {
			removeNodeFromNodeGroupRecord(nsId, infraId, v, nodeId, nodeListInNodeGroup)
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

	// Call Spider deregister API
	var callResult any
	client := clientManager.NewHttpClient()
	method := "DELETE"

	// Create request body
	type JsonTemplate struct {
		ConnectionName string
	}
	requestBody := JsonTemplate{
		ConnectionName: nodeInfo.ConnectionName,
	}

	if nodeInfo.CspResourceName == "" {
		// CspResourceName is not set — the VM was never registered in Spider's IID store
		// (e.g. imported via registerCspResources without Spider registration).
		// Skip the Spider call and proceed directly to TB registry cleanup.
		log.Warn().Msgf("CspResourceName is empty for node '%s'; skipping Spider deregister call", nodeId)
	} else {
		url := model.SpiderRestUrl + "/regvm/" + nodeInfo.CspResourceName
		log.Debug().Msg("Sending deregister DELETE request to " + url)

		restyResp, err := clientManager.ExecuteHttpRequest(
			client,
			method,
			url,
			nil,
			clientManager.SetUseBody(requestBody),
			&requestBody,
			&callResult,
			clientManager.VeryShortDuration,
		)
		err = clientManager.HandleHttpResponse(restyResp, err)

		if err != nil {
			if apierr.IsNotFound(err) {
				log.Warn().Err(err).Msg("VM not found in cb-spider IID store; proceeding with TB registry cleanup")
			} else {
				log.Error().Err(err).Msg("")
				return err
			}
		} else {
			log.Debug().Msg("Deregister request finished from " + url)
		}
	}

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
	nodeGroupKey := common.GenInfraNodeGroupKey(nsId, infraId, nodeInfo.NodeGroupId)
	if len(nodeListInNodeGroup) == 0 {
		err := kvstore.Delete(nodeGroupKey)
		if err != nil {
			log.Error().Err(err).Msg("Failed to remove the empty nodeGroup")
			return err
		}
	} else {
		removeNodeFromNodeGroupRecord(nsId, infraId, nodeInfo.NodeGroupId, nodeId, nodeListInNodeGroup)
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
