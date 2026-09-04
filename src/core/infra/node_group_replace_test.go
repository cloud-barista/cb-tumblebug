package infra

// The guards matter more than the happy path: every refusal here prevents
// destroying something the caller did not mean to destroy.

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvtest"
)

func TestReplaceFailedNodeGroupRejectsMismatchedName(t *testing.T) {
	req := &model.AddNodeGroupDynamicReq{}
	req.Name = "other"
	_, err := ReplaceFailedNodeGroup(t.Context(), "default", "infra01", "g11", req)
	if err == nil {
		t.Fatal("a request naming a different node group must be refused")
	}
	if !errors.Is(err, ErrNodeGroupNameMismatch) {
		t.Errorf("got %v, want ErrNodeGroupNameMismatch", err)
	}
}

func TestReplaceFailedNodeGroupRefusesWhenNotFound(t *testing.T) {
	memStore := kvtest.NewMemoryStore()
	cleanup := kvstore.SetTestStore(memStore)
	defer cleanup()

	// Infra exists but has no nodes
	infraObj := model.InfraInfo{Id: "infra01"}
	infraBytes, _ := json.Marshal(infraObj)
	_ = kvstore.Put(common.GenInfraKey("default", "infra01", ""), string(infraBytes))

	req := &model.AddNodeGroupDynamicReq{}
	req.Name = "g11"
	_, err := ReplaceFailedNodeGroup(t.Context(), "default", "infra01", "g11", req)
	if err == nil {
		t.Fatal("empty node group should return error")
	}
	if !errors.Is(err, ErrNodeGroupNotFound) {
		t.Errorf("got %v, want ErrNodeGroupNotFound", err)
	}
}

func TestReplaceFailedNodeGroupRefusesWhenNodeRunning(t *testing.T) {
	memStore := kvtest.NewMemoryStore()
	cleanup := kvstore.SetTestStore(memStore)
	defer cleanup()

	nsId := "default"
	infraId := "infra01"
	groupId := "g11"
	nodeId := "node01"

	infraObj := model.InfraInfo{Id: infraId}
	infraBytes, _ := json.Marshal(infraObj)
	_ = kvstore.Put(common.GenInfraKey(nsId, infraId, ""), string(infraBytes))

	nodeObj := model.NodeInfo{
		Id:          nodeId,
		NodeGroupId: groupId,
		Status:      model.StatusRunning, // not failed!
		SpecId:      "aws-t3-small",
	}
	nodeBytes, _ := json.Marshal(nodeObj)
	_ = kvstore.Put(common.GenInfraKey(nsId, infraId, nodeId), string(nodeBytes))

	req := &model.AddNodeGroupDynamicReq{}
	req.Name = groupId
	_, err := ReplaceFailedNodeGroup(t.Context(), nsId, infraId, groupId, req)
	if err == nil {
		t.Fatal("running node group must be refused")
	}
	if !errors.Is(err, ErrNodeGroupInUse) {
		t.Errorf("got %v, want ErrNodeGroupInUse", err)
	}
}

func TestReplaceFailedNodeGroupRefusesWhenCspResourceRemains(t *testing.T) {
	memStore := kvtest.NewMemoryStore()
	cleanup := kvstore.SetTestStore(memStore)
	defer cleanup()

	nsId := "default"
	infraId := "infra01"
	groupId := "g11"
	nodeId := "node01"

	infraObj := model.InfraInfo{Id: infraId}
	infraBytes, _ := json.Marshal(infraObj)
	_ = kvstore.Put(common.GenInfraKey(nsId, infraId, ""), string(infraBytes))

	nodeObj := model.NodeInfo{
		Id:              nodeId,
		NodeGroupId:     groupId,
		Status:          model.StatusFailed,
		CspResourceName: "i-0123456789abcdef0", // CSP resource still holds
		SpecId:          "aws-t3-small",
	}
	nodeBytes, _ := json.Marshal(nodeObj)
	_ = kvstore.Put(common.GenInfraKey(nsId, infraId, nodeId), string(nodeBytes))

	req := &model.AddNodeGroupDynamicReq{}
	req.Name = groupId
	_, err := ReplaceFailedNodeGroup(t.Context(), nsId, infraId, groupId, req)
	if err == nil {
		t.Fatal("node with remaining CSP resource must be refused")
	}
	if !errors.Is(err, ErrNodeGroupHasCspResource) {
		t.Errorf("got %v, want ErrNodeGroupHasCspResource", err)
	}
}

func TestReplaceFailedNodeGroupRefusesWhenSpecChanged(t *testing.T) {
	memStore := kvtest.NewMemoryStore()
	cleanup := kvstore.SetTestStore(memStore)
	defer cleanup()

	nsId := "default"
	infraId := "infra01"
	groupId := "g11"
	nodeId := "node01"

	infraObj := model.InfraInfo{Id: infraId}
	infraBytes, _ := json.Marshal(infraObj)
	_ = kvstore.Put(common.GenInfraKey(nsId, infraId, ""), string(infraBytes))

	nodeObj := model.NodeInfo{
		Id:          nodeId,
		NodeGroupId: groupId,
		Status:      model.StatusFailed,
		SpecId:      "aws-t3-small",
	}
	nodeBytes, _ := json.Marshal(nodeObj)
	_ = kvstore.Put(common.GenInfraKey(nsId, infraId, nodeId), string(nodeBytes))

	req := &model.AddNodeGroupDynamicReq{}
	req.Name = groupId
	req.SpecId = "aws-t3-large" // different spec!

	_, err := ReplaceFailedNodeGroup(t.Context(), nsId, infraId, groupId, req)
	if err == nil {
		t.Fatal("changing spec on failed node group must be refused")
	}
	if !errors.Is(err, ErrSpecChanged) {
		t.Errorf("got %v, want ErrSpecChanged", err)
	}
}

func TestReplaceErrorsAreDistinguishable(t *testing.T) {
	// The handler maps these to 409 and 400, so they have to stay distinct from
	// each other and from a plain error.
	for _, pair := range [][2]error{
		{ErrNodeGroupInUse, ErrNodeGroupHasCspResource},
		{ErrNodeGroupInUse, ErrSpecChanged},
		{ErrNodeGroupHasCspResource, ErrSpecChanged},
	} {
		if errors.Is(pair[0], pair[1]) {
			t.Errorf("%v and %v must not match each other", pair[0], pair[1])
		}
	}
	wrapped := errors.New("wrapped: " + ErrSpecChanged.Error())
	if errors.Is(wrapped, ErrSpecChanged) {
		t.Error("a merely similar message must not be mistaken for the sentinel")
	}
}
