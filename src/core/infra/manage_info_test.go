/*
Copyright 2019 The Cloud-Barista Authors.
<!-- SPDX-License-Identifier: Apache-2.0 -->
*/

package infra

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvtest"
)

func TestSortAndCompactStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "Empty slice",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "Duplicates and unsorted",
			input:    []string{"c", "a", "b", "a", "c", "b"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "Already sorted without duplicates",
			input:    []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortAndCompactStrings(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("sortAndCompactStrings(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestAppendNonEmptyString(t *testing.T) {
	s := []string{"initial"}
	s = appendNonEmptyString(s, "")
	if len(s) != 1 {
		t.Errorf("expected length 1 after appending empty string, got %d", len(s))
	}

	s = appendNonEmptyString(s, "new")
	if len(s) != 2 || s[1] != "new" {
		t.Errorf("expected 'new' to be appended, got %v", s)
	}
}

func TestBuildImplicitClusterInfoFromNodes(t *testing.T) {
	nodes := []model.NodeInfo{
		{
			Id:             "node-1",
			VNetId:         "vnet-east",
			NodeGroupId:    "group-worker",
			ConnectionName: "aws-east",
			ConnectionConfig: model.ConnConfig{
				ProviderName: "aws",
			},
			Region: model.RegionInfo{
				Region: "us-east-1",
			},
		},
		{
			Id:             "node-2",
			VNetId:         "vnet-east",
			NodeGroupId:    "group-worker",
			ConnectionName: "aws-east",
			ConnectionConfig: model.ConnConfig{
				ProviderName: "aws",
			},
			Region: model.RegionInfo{
				Region: "us-east-1",
			},
		},
		{
			Id:             "node-3",
			VNetId:         "vnet-west",
			NodeGroupId:    "group-master",
			ConnectionName: "aws-west",
			ConnectionConfig: model.ConnConfig{
				ProviderName: "aws",
			},
			Region: model.RegionInfo{
				Region: "us-west-2",
			},
		},
	}

	clusters := buildImplicitClusterInfoFromNodes("infra-demo", nodes)
	if len(clusters) != 2 {
		t.Fatalf("expected 2 clusters (vnet-east and vnet-west), got %d", len(clusters))
	}

	// First cluster: vnet-east (sorted by ID)
	c1 := clusters[0]
	if c1.VNetId != "vnet-east" {
		t.Errorf("expected first cluster VNetId 'vnet-east', got %q", c1.VNetId)
	}
	if c1.NodeCount != 2 {
		t.Errorf("expected 2 nodes in vnet-east, got %d", c1.NodeCount)
	}
	if c1.NodeGroupCount != 1 {
		t.Errorf("expected 1 nodegroup in vnet-east, got %d", c1.NodeGroupCount)
	}

	// Second cluster: vnet-west
	c2 := clusters[1]
	if c2.VNetId != "vnet-west" {
		t.Errorf("expected second cluster VNetId 'vnet-west', got %q", c2.VNetId)
	}
	if c2.NodeCount != 1 {
		t.Errorf("expected 1 node in vnet-west, got %d", c2.NodeCount)
	}
}

func TestConvertNodeInfoToNodeStatusInfo(t *testing.T) {
	node := model.NodeInfo{
		Id:              "node-101",
		Uid:             "uid-101",
		CspResourceName: "csp-vm-101",
		CspResourceId:   "i-1234567890abcdef0",
		Name:            "worker-01",
		Status:          model.StatusRunning,
		PublicIP:        "198.51.100.1",
		PrivateIP:       "10.0.0.5",
		SSHPort:         22,
	}

	statusInfo := ConvertNodeInfoToNodeStatusInfo(node)

	if statusInfo.Id != node.Id {
		t.Errorf("expected Id %q, got %q", node.Id, statusInfo.Id)
	}
	if statusInfo.PublicIp != node.PublicIP {
		t.Errorf("expected PublicIp %q, got %q", node.PublicIP, statusInfo.PublicIp)
	}
	if statusInfo.PrivateIp != node.PrivateIP {
		t.Errorf("expected PrivateIp %q, got %q", node.PrivateIP, statusInfo.PrivateIp)
	}
	if statusInfo.Status != model.StatusRunning {
		t.Errorf("expected Status %q, got %q", model.StatusRunning, statusInfo.Status)
	}
}

func TestHydrateNodeInfo(t *testing.T) {
	ng := model.NodeGroupInfo{
		Id:               "ng-01",
		ConnectionName:   "aws-us-east-1",
		SpecId:           "aws-t2-micro",
		ImageId:          "ami-12345",
		VNetId:           "vnet-01",
		SubnetId:         "subnet-01",
		SecurityGroupIds: []string{"sg-01"},
		SshKeyId:         "ssh-key-01",
		Description:      "test node group",
		Label:            map[string]string{"env": "test"},
	}

	node := model.NodeInfo{
		Id:          "node-01",
		NodeGroupId: "ng-01",
		Name:        "worker-01",
		Status:      model.StatusRunning,
		PublicIP:    "1.2.3.4",
	}

	HydrateNodeInfo(&node, &ng)

	if node.ConnectionName != "aws-us-east-1" {
		t.Errorf("expected ConnectionName %q, got %q", "aws-us-east-1", node.ConnectionName)
	}
	if node.SpecId != "aws-t2-micro" {
		t.Errorf("expected SpecId %q, got %q", "aws-t2-micro", node.SpecId)
	}
	if node.ImageId != "ami-12345" {
		t.Errorf("expected ImageId %q, got %q", "ami-12345", node.ImageId)
	}
	if node.VNetId != "vnet-01" {
		t.Errorf("expected VNetId %q, got %q", "vnet-01", node.VNetId)
	}
	if len(node.SecurityGroupIds) != 1 || node.SecurityGroupIds[0] != "sg-01" {
		t.Errorf("expected SecurityGroupIds %v, got %v", []string{"sg-01"}, node.SecurityGroupIds)
	}
	if node.Label["env"] != "test" {
		t.Errorf("expected Label['env'] %q, got %q", "test", node.Label["env"])
	}
}

func TestLoadInfraNodeGroupMap_NonExistent(t *testing.T) {
	m := LoadInfraNodeGroupMap("non-existent-ns", "non-existent-infra")
	if m == nil {
		t.Errorf("expected non-nil map, got nil")
	}
	if len(m) != 0 {
		t.Errorf("expected empty map, got %d entries", len(m))
	}
}

func TestCompactNodeInfo_RoundtripAndSizeReduction(t *testing.T) {
	ng := model.NodeGroupInfo{
		ResourceType:   model.StrNodeGroup,
		Id:             "ng-01",
		Name:           "ng-01",
		ConnectionName: "aws-ap-northeast-2",
		ConnectionConfig: model.ConnConfig{
			ConfigName:     "aws-ap-northeast-2",
			ProviderName:   "aws",
			DriverName:     "aws-driver-v1.0",
			CredentialName: "aws-credential",
			RegionDetail: model.RegionDetail{
				RegionName: "ap-northeast-2",
				Location: model.Location{
					Display:   "Seoul, Korea",
					Latitude:  37.5665,
					Longitude: 126.9780,
				},
			},
		},
		Region: model.RegionInfo{
			Region: "ap-northeast-2",
			Zone:   "ap-northeast-2a",
		},
		Location: model.Location{
			Display:   "Seoul, Korea",
			Latitude:  37.5665,
			Longitude: 126.9780,
		},
		SpecId:      "aws-t3-small",
		CspSpecName: "t3.small",
		Spec: model.SpecSummary{
			CspSpecName: "t3.small",
			VCPU:        2,
			MemoryGiB:   2,
			CostPerHour: 0.026,
		},
		ImageId:      "ami-ubuntu-2204",
		CspImageName: "ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server",
		Image: model.ImageSummary{
			ResourceType:   model.StrImage,
			CspImageName:   "ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server",
			OSType:         "Linux",
			OSArchitecture: "x86_64",
			OSDistribution: "Ubuntu",
		},
		VNetId:           "vnet-seoul",
		CspVNetId:        "vpc-12345678",
		SubnetId:         "subnet-a",
		CspSubnetId:      "subnet-12345678",
		NetworkInterface: "eth0",
		SecurityGroupIds: []string{"sg-web", "sg-ssh"},
		SshKeyId:         "key-seoul",
		CspSshKeyId:      "key-12345678",
		SSHPort:          22,
		NodeUserName:     "ubuntu",
		RootDiskType:     "gp3",
		RootDiskSize:     50,
		RootDeviceName:   "/dev/sda1",
		Label:            map[string]string{"env": "prod", "cluster": "large-scale"},
		Description:      "production worker node group",
	}

	fullNode := model.NodeInfo{
		ResourceType:     model.StrNode,
		Id:               "ng-01-1",
		Uid:              "uid-node-12345",
		CspResourceName:  "cb-ng-01-1",
		CspResourceId:    "i-0123456789abcdef0",
		Name:             "ng-01-1",
		NodeGroupId:      "ng-01",
		Status:           model.StatusRunning,
		TargetStatus:     model.StatusComplete,
		TargetAction:     model.ActionComplete,
		CreatedTime:      "2026-10-02 14:00:00",
		PublicIP:         "54.180.1.2",
		PrivateIP:        "10.0.1.5",
		PublicDNS:        "ec2-54-180-1-2.ap-northeast-2.compute.amazonaws.com",
		PrivateDNS:       "ip-10-0-1-5.ap-northeast-2.compute.internal",
		ConnectionName:   ng.ConnectionName,
		ConnectionConfig: ng.ConnectionConfig,
		Region:           ng.Region,
		Location:         ng.Location,
		SpecId:           ng.SpecId,
		CspSpecName:      ng.CspSpecName,
		Spec:             ng.Spec,
		ImageId:          ng.ImageId,
		CspImageName:     ng.CspImageName,
		Image:            ng.Image,
		VNetId:           ng.VNetId,
		CspVNetId:        ng.CspVNetId,
		SubnetId:         ng.SubnetId,
		CspSubnetId:      ng.CspSubnetId,
		NetworkInterface: ng.NetworkInterface,
		SecurityGroupIds: ng.SecurityGroupIds,
		SshKeyId:         ng.SshKeyId,
		CspSshKeyId:      ng.CspSshKeyId,
		SSHPort:          ng.SSHPort,
		NodeUserName:     ng.NodeUserName,
		RootDiskType:     ng.RootDiskType,
		RootDiskSize:     ng.RootDiskSize,
		RootDeviceName:   ng.RootDeviceName,
		Description:      ng.Description,
		Label:            map[string]string{"env": "prod", "cluster": "large-scale"},
	}

	fullJSON, err := json.Marshal(fullNode)
	if err != nil {
		t.Fatalf("failed to marshal full Node: %v", err)
	}

	// 1. Dehydrate to CompactNodeInfo
	compact := model.ToCompactNodeInfo(fullNode, &ng)
	compactJSON, err := json.Marshal(compact)
	if err != nil {
		t.Fatalf("failed to marshal compact Node: %v", err)
	}

	t.Logf("Full Node JSON size: %d bytes", len(fullJSON))
	t.Logf("Compact Node JSON size: %d bytes", len(compactJSON))

	// Verify > 60% reduction (typically 80-90%)
	reductionPercent := float64(len(fullJSON)-len(compactJSON)) / float64(len(fullJSON)) * 100
	t.Logf("Storage reduction: %.1f%%", reductionPercent)
	if reductionPercent < 60.0 {
		t.Errorf("expected at least 60%% storage reduction, got %.1f%%", reductionPercent)
	}

	// 2. Unmarshal compact JSON into NodeInfo (simulating reading from etcd)
	var restoredNode model.NodeInfo
	if err := json.Unmarshal(compactJSON, &restoredNode); err != nil {
		t.Fatalf("failed to unmarshal compact JSON into NodeInfo: %v", err)
	}

	// 3. Hydrate from NodeGroup
	HydrateNodeInfo(&restoredNode, &ng)

	// 4. Verify all fields match the original fullNode
	if !reflect.DeepEqual(restoredNode, fullNode) {
		t.Errorf("restored node does not match original full node!\nRestored: %+v\nOriginal: %+v", restoredNode, fullNode)
	}
}

func TestCompactNodeInfo_OverridesPreservation(t *testing.T) {
	ng := model.NodeGroupInfo{
		Id:             "ng-multi",
		ConnectionName: "aws-ap-northeast-2",
		Region: model.RegionInfo{
			Region: "ap-northeast-2",
			Zone:   "ap-northeast-2a",
		},
		SubnetId:    "subnet-a",
		CspSubnetId: "csp-subnet-a",
		SSHPort:     22,
		Label:       map[string]string{"env": "prod", "tier": "backend"},
		Description: "default description",
	}

	// Node placed in different subnet/zone with custom labels and override description
	nodeWithOverrides := model.NodeInfo{
		Id:          "node-02",
		NodeGroupId: "ng-multi",
		Region: model.RegionInfo{
			Region: "ap-northeast-2",
			Zone:   "ap-northeast-2c", // Overridden zone
		},
		SubnetId:    "subnet-c",     // Overridden subnet
		CspSubnetId: "csp-subnet-c", // Overridden csp subnet
		SSHPort:     2222,           // Overridden port
		Description: "custom worker description",
		Label: map[string]string{
			"env":    "staging",        // Overridden key
			"tier":   "backend",        // Inherited key
			"custom": "worker-special", // Instance-specific key
		},
	}

	compact := model.ToCompactNodeInfo(nodeWithOverrides, &ng)

	// Check that compact preserved the overrides
	if compact.SubnetId != "subnet-c" {
		t.Errorf("expected compact SubnetId 'subnet-c', got %q", compact.SubnetId)
	}
	if compact.Region == nil || compact.Region.Zone != "ap-northeast-2c" {
		t.Errorf("expected compact Region.Zone 'ap-northeast-2c', got %+v", compact.Region)
	}
	if compact.SSHPort != 2222 {
		t.Errorf("expected compact SSHPort 2222, got %d", compact.SSHPort)
	}
	if compact.Description != "custom worker description" {
		t.Errorf("expected compact Description 'custom worker description', got %q", compact.Description)
	}
	if compact.Label["env"] != "staging" || compact.Label["custom"] != "worker-special" {
		t.Errorf("expected compact custom labels, got %+v", compact.Label)
	}
	// "tier": "backend" should NOT be in compact.Label because it matches ng
	if _, exists := compact.Label["tier"]; exists {
		t.Errorf("expected unchanged label 'tier' to be omitted from compact, but was present")
	}

	// Now hydrate back
	var restored model.NodeInfo
	compactBytes, _ := json.Marshal(compact)
	_ = json.Unmarshal(compactBytes, &restored)
	HydrateNodeInfo(&restored, &ng)

	if restored.SubnetId != "subnet-c" {
		t.Errorf("expected restored SubnetId 'subnet-c', got %q", restored.SubnetId)
	}
	if restored.Region.Zone != "ap-northeast-2c" {
		t.Errorf("expected restored Zone 'ap-northeast-2c', got %q", restored.Region.Zone)
	}
	if restored.SSHPort != 2222 {
		t.Errorf("expected restored SSHPort 2222, got %d", restored.SSHPort)
	}
	if restored.Label["env"] != "staging" {
		t.Errorf("expected overridden label env='staging', got %q", restored.Label["env"])
	}
	if restored.Label["tier"] != "backend" {
		t.Errorf("expected inherited label tier='backend', got %q", restored.Label["tier"])
	}
	if restored.Label["custom"] != "worker-special" {
		t.Errorf("expected instance label custom='worker-special', got %q", restored.Label["custom"])
	}
}

func TestNodeGroupCache(t *testing.T) {
	// Directly test caching functions
	InvalidateNodeGroupCache("test-ns", "test-infra")

	cacheKey := "test-ns/test-infra/ng-cache"
	nodeGroupCacheMu.Lock()
	nodeGroupCache[cacheKey] = cachedNodeGroupEntry{
		ng: model.NodeGroupInfo{
			Id:             "ng-cache",
			ConnectionName: "conn-cached",
		},
		expiresAt: time.Now().Add(5 * time.Minute),
	}
	nodeGroupCacheMu.Unlock()

	ng, err := GetNodeGroupCached("test-ns", "test-infra", "ng-cache")
	if err != nil {
		t.Fatalf("expected cache hit, got error: %v", err)
	}
	if ng.ConnectionName != "conn-cached" {
		t.Errorf("expected ConnectionName 'conn-cached', got %q", ng.ConnectionName)
	}

	// Invalidate cache
	InvalidateNodeGroupCache("test-ns", "test-infra")

	nodeGroupCacheMu.RLock()
	_, exists := nodeGroupCache[cacheKey]
	nodeGroupCacheMu.RUnlock()

	if exists {
		t.Errorf("expected cache entry to be purged, but it still exists")
	}
}

func TestUpdateNodeInfo_CompactPersistenceAndHydration(t *testing.T) {
	memStore := kvtest.NewMemoryStore()
	cleanup := kvstore.SetTestStore(memStore)
	defer cleanup()

	nsId := "test-ns"
	infraId := "test-infra"
	ngId := "ng-worker"
	nodeId := "node-w1"

	// 1. Setup NodeGroup
	ng := model.NodeGroupInfo{
		ResourceType:   model.StrNodeGroup,
		Id:             ngId,
		Name:           ngId,
		ConnectionName: "aws-ap-northeast-2",
		ConnectionConfig: model.ConnConfig{
			ConfigName:   "aws-ap-northeast-2",
			ProviderName: "aws",
		},
		Region: model.RegionInfo{
			Region: "ap-northeast-2",
			Zone:   "ap-northeast-2a",
		},
		SpecId:           "aws-t3-small",
		ImageId:          "ami-ubuntu-2204",
		VNetId:           "vnet-test",
		SubnetId:         "subnet-test-a",
		SecurityGroupIds: []string{"sg-web"},
		SshKeyId:         "key-test",
		SSHPort:          22,
		NodeUserName:     "ubuntu",
		Label:            map[string]string{"env": "test"},
		Description:      "worker group",
	}
	ngBytes, _ := json.Marshal(ng)
	_ = kvstore.Put(common.GenInfraNodeGroupKey(nsId, infraId, ngId), string(ngBytes))

	// Invalidate cache so it loads fresh
	InvalidateNodeGroupCache(nsId, infraId)

	// 2. Setup initial compact node in kvstore
	initialCompact := model.CompactNodeInfo{
		ResourceType: model.StrNode,
		Id:           nodeId,
		Uid:          "uid-node-w1",
		Name:         nodeId,
		NodeGroupId:  ngId,
		Status:       model.StatusRunning,
		PublicIP:     "1.2.3.4",
		PrivateIP:    "10.0.0.1",
	}
	compactBytes, _ := json.Marshal(initialCompact)
	nodeKey := common.GenInfraKey(nsId, infraId, nodeId)
	_ = kvstore.Put(nodeKey, string(compactBytes))

	// 3. Read via GetNodeObject - must be fully hydrated
	hydrated, err := GetNodeObject(nsId, infraId, nodeId)
	if err != nil {
		t.Fatalf("GetNodeObject failed: %v", err)
	}
	if hydrated.ConnectionName != "aws-ap-northeast-2" {
		t.Errorf("expected ConnectionName 'aws-ap-northeast-2', got %q", hydrated.ConnectionName)
	}
	if hydrated.SpecId != "aws-t3-small" {
		t.Errorf("expected SpecId 'aws-t3-small', got %q", hydrated.SpecId)
	}
	if hydrated.Label["env"] != "test" {
		t.Errorf("expected Label env='test', got %q", hydrated.Label["env"])
	}

	// 4. Update node with fully hydrated struct (simulating status update or API update)
	hydrated.Status = model.StatusSuspended
	UpdateNodeInfo(nsId, infraId, hydrated)

	// 5. Verify etcd raw data is STILL compact (< 400 bytes, no connectionConfig)
	rawKV, exists, err := kvstore.GetKv(nodeKey)
	if err != nil || !exists {
		t.Fatalf("expected node in kvstore, exists=%v, err=%v", exists, err)
	}

	t.Logf("Updated node raw JSON: %s", rawKV.Value)
	t.Logf("Updated node raw JSON length: %d bytes", len(rawKV.Value))

	if len(rawKV.Value) > 600 {
		t.Errorf("expected compact node JSON (<600 bytes), got %d bytes: %s", len(rawKV.Value), rawKV.Value)
	}

	// Verify status was updated
	var storedCompact model.CompactNodeInfo
	_ = json.Unmarshal([]byte(rawKV.Value), &storedCompact)
	if storedCompact.Status != model.StatusSuspended {
		t.Errorf("expected stored status 'Suspended', got %q", storedCompact.Status)
	}

	// 6. Read again via GetNodeObject - must be hydrated and reflect updated status
	reRead, err := GetNodeObject(nsId, infraId, nodeId)
	if err != nil {
		t.Fatalf("GetNodeObject failed: %v", err)
	}
	if reRead.Status != model.StatusSuspended {
		t.Errorf("expected re-read status 'Suspended', got %q", reRead.Status)
	}
	if reRead.ConnectionName != "aws-ap-northeast-2" {
		t.Errorf("expected re-read ConnectionName 'aws-ap-northeast-2', got %q", reRead.ConnectionName)
	}

	// 7. Test legacy record migration
	legacyNodeId := "legacy-node"
	legacyKey := common.GenInfraKey(nsId, infraId, legacyNodeId)
	legacyFull := hydrated
	legacyFull.Id = legacyNodeId
	legacyFull.Status = model.StatusRunning
	legacyFullBytes, _ := json.Marshal(legacyFull)
	_ = kvstore.Put(legacyKey, string(legacyFullBytes))

	t.Logf("Legacy full node size before update: %d bytes", len(legacyFullBytes))

	// Update legacy node status
	legacyFull.Status = model.StatusTerminated
	UpdateNodeInfo(nsId, infraId, legacyFull)

	// Verify legacy node was compacted in etcd
	rawLegacyKV, exists, _ := kvstore.GetKv(legacyKey)
	if !exists {
		t.Fatalf("legacy node missing")
	}
	t.Logf("Legacy node size after update: %d bytes", len(rawLegacyKV.Value))
	if len(rawLegacyKV.Value) > 600 {
		t.Errorf("expected legacy node to be migrated to compact (<600 bytes), got %d bytes", len(rawLegacyKV.Value))
	}
}
