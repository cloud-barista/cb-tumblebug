/*
Copyright 2019 The Cloud-Barista Authors.
<!-- SPDX-License-Identifier: Apache-2.0 -->
*/

package resource

import (
	"testing"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
)

func TestConvertSpiderClusterStatusToK8sClusterStatus(t *testing.T) {
	tests := []struct {
		name     string
		input    model.SpiderClusterStatus
		expected model.K8sClusterStatus
	}{
		{"Creating", model.SpiderClusterCreating, model.K8sClusterCreating},
		{"Active", model.SpiderClusterActive, model.K8sClusterActive},
		{"Inactive", model.SpiderClusterInactive, model.K8sClusterInactive},
		{"Updating", model.SpiderClusterUpdating, model.K8sClusterUpdating},
		{"Deleting", model.SpiderClusterDeleting, model.K8sClusterDeleting},
		{"Unknown", "SomethingUnknown", model.K8sClusterInactive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertSpiderClusterStatusToK8sClusterStatus(tt.input)
			if got != tt.expected {
				t.Errorf("convertSpiderClusterStatusToK8sClusterStatus(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestConvertSpiderNodeGroupStatusToK8sNodeGroupStatus(t *testing.T) {
	tests := []struct {
		name     string
		input    model.SpiderNodeGroupStatus
		expected model.K8sNodeGroupStatus
	}{
		{"Creating", model.SpiderNodeGroupCreating, model.K8sNodeGroupCreating},
		{"Active", model.SpiderNodeGroupActive, model.K8sNodeGroupActive},
		{"Inactive", model.SpiderNodeGroupInactive, model.K8sNodeGroupInactive},
		{"Updating", model.SpiderNodeGroupUpdating, model.K8sNodeGroupUpdating},
		{"Deleting", model.SpiderNodeGroupDeleting, model.K8sNodeGroupDeleting},
		{"Unknown", "SomethingUnknown", model.K8sNodeGroupInactive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertSpiderNodeGroupStatusToK8sNodeGroupStatus(tt.input)
			if got != tt.expected {
				t.Errorf("convertSpiderNodeGroupStatusToK8sNodeGroupStatus(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFillK8sNodeGroupInfoFromK8sNodeGroupReq(t *testing.T) {
	req := model.K8sNodeGroupReq{
		Name:            "ng-1",
		ImageId:         "ami-12345",
		SpecId:          "t3.medium",
		RootDiskType:    "gp3",
		RootDiskSize:    50,
		SshKeyId:        "my-key",
		OnAutoScaling:   "true",
		DesiredNodeSize: 3,
		MinNodeSize:     1,
		MaxNodeSize:     5,
		Description:     "Test node group",
	}

	var info model.K8sNodeGroupInfo
	fillK8sNodeGroupInfoFromK8sNodeGroupReq(&info, &req)

	if info.Id != req.Name || info.Name != req.Name {
		t.Errorf("Expected Id and Name %q, got %q and %q", req.Name, info.Id, info.Name)
	}
	if info.ImageId != req.ImageId {
		t.Errorf("Expected ImageId %q, got %q", req.ImageId, info.ImageId)
	}
	if info.SpecId != req.SpecId {
		t.Errorf("Expected SpecId %q, got %q", req.SpecId, info.SpecId)
	}
	if info.DesiredNodeSize != 3 || info.MinNodeSize != 1 || info.MaxNodeSize != 5 {
		t.Errorf("Node size mismatch: got desired=%d, min=%d, max=%d", info.DesiredNodeSize, info.MinNodeSize, info.MaxNodeSize)
	}
	if !info.OnAutoScaling {
		t.Errorf("Expected OnAutoScaling to be true, got %v", info.OnAutoScaling)
	}
}

func TestHandleK8sClusterAction(t *testing.T) {
	nsId := "test-ns-k8s"
	clusterId := "cluster-01"

	// 1. Validation errors for empty inputs
	if _, err := HandleK8sClusterAction("", clusterId, "continue"); err == nil {
		t.Error("expected error for empty nsId, got nil")
	}
	if _, err := HandleK8sClusterAction(nsId, "", "continue"); err == nil {
		t.Error("expected error for empty clusterId, got nil")
	}

	// 2. Non-existent cluster error
	if _, err := HandleK8sClusterAction(nsId, "non-existent", "continue"); err == nil {
		t.Error("expected error for non-existent cluster, got nil")
	}

	// 3. Create cluster in KVStore
	clusterInfo := model.K8sClusterInfo{
		Id:   clusterId,
		Name: clusterId,
	}
	err := createK8sClusterInfo(nsId, clusterInfo)
	if err != nil {
		t.Fatalf("failed to create k8s cluster info: %v", err)
	}
	defer deleteK8sClusterInfo(nsId, clusterId)

	// Verify CheckK8sCluster returns true
	exists, err := CheckK8sCluster(nsId, clusterId)
	if err != nil || !exists {
		t.Fatalf("expected cluster to exist: exists=%v, err=%v", exists, err)
	}

	// 4. Test "continue" action
	resp, err := HandleK8sClusterAction(nsId, clusterId, "continue")
	if err != nil {
		t.Errorf("unexpected error on continue action: %v", err)
	}
	if resp != "Continue the holding K8sCluster" {
		t.Errorf("unexpected response: %q", resp)
	}

	// Check holding map
	key := common.GenK8sClusterKey(nsId, clusterId)
	val, ok := holdingK8sClusterMap.Load(key)
	if !ok || val != "continue" {
		t.Errorf("expected holdingK8sClusterMap to have continue, got %v", val)
	}

	// 5. Test "withdraw" action
	resp, err = HandleK8sClusterAction(nsId, clusterId, "withdraw")
	if err != nil {
		t.Errorf("unexpected error on withdraw action: %v", err)
	}
	if resp != "Withdraw the holding K8sCluster" {
		t.Errorf("unexpected response: %q", resp)
	}

	// 6. Test unsupported action
	_, err = HandleK8sClusterAction(nsId, clusterId, "invalid-action")
	if err == nil {
		t.Error("expected error on unsupported action, got nil")
	}
}

func TestK8sClusterCRUD_InMemory(t *testing.T) {
	nsId := "test-ns-k8s-crud"
	clusterId := "my-k8s-cluster"

	// Initially should not exist
	exists, _ := CheckK8sCluster(nsId, clusterId)
	if exists {
		t.Fatalf("cluster %s should not exist initially", clusterId)
	}

	// Store cluster
	cluster := model.K8sClusterInfo{
		Id:             clusterId,
		Name:           clusterId,
		ConnectionName: "aws-test-conn",
		Status:         model.K8sClusterActive,
	}
	err := createK8sClusterInfo(nsId, cluster)
	if err != nil {
		t.Fatalf("failed to store cluster: %v", err)
	}

	// Check existence
	exists, err = CheckK8sCluster(nsId, clusterId)
	if err != nil || !exists {
		t.Fatalf("cluster should exist: exists=%v, err=%v", exists, err)
	}

	// Retrieve cluster
	retrieved, err := getK8sClusterInfo(nsId, clusterId)
	if err != nil {
		t.Fatalf("failed to get cluster: %v", err)
	}
	if retrieved.Id != clusterId || retrieved.Status != model.K8sClusterActive {
		t.Errorf("retrieved cluster mismatch: %+v", retrieved)
	}

	// Delete cluster
	err = deleteK8sClusterInfo(nsId, clusterId)
	if err != nil {
		t.Fatalf("failed to delete cluster: %v", err)
	}

	// Confirm deletion
	exists, _ = CheckK8sCluster(nsId, clusterId)
	if exists {
		t.Errorf("cluster should not exist after deletion")
	}
}
