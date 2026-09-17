/*
Copyright 2019 The Cloud-Barista Authors.
<!-- SPDX-License-Identifier: Apache-2.0 -->
*/

package infra

import (
	"reflect"
	"testing"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
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
			Id:          "node-1",
			VNetId:      "vnet-east",
			NodeGroupId: "group-worker",
			ConnectionName: "aws-east",
			ConnectionConfig: model.ConnConfig{
				ProviderName: "aws",
			},
			Region: model.RegionInfo{
				Region: "us-east-1",
			},
		},
		{
			Id:          "node-2",
			VNetId:      "vnet-east",
			NodeGroupId: "group-worker",
			ConnectionName: "aws-east",
			ConnectionConfig: model.ConnConfig{
				ProviderName: "aws",
			},
			Region: model.RegionInfo{
				Region: "us-east-1",
			},
		},
		{
			Id:          "node-3",
			VNetId:      "vnet-west",
			NodeGroupId: "group-master",
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
		Id:             "ng-01",
		ConnectionName: "aws-us-east-1",
		SpecId:         "aws-t2-micro",
		ImageId:        "ami-12345",
		VNetId:         "vnet-01",
		SubnetId:       "subnet-01",
		SecurityGroupIds: []string{"sg-01"},
		SshKeyId:       "ssh-key-01",
		Description:    "test node group",
		Label:          map[string]string{"env": "test"},
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

