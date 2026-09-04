/*
Copyright 2019 The Cloud-Barista Authors.
<!-- SPDX-License-Identifier: Apache-2.0 -->
*/

package infra

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/rs/zerolog/log"
)

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

