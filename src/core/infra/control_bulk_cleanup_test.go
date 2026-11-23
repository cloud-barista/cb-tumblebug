package infra

import (
	"slices"
	"testing"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
)

func TestBulkAssociationKeys(t *testing.T) {
	node := func(id, image, key, vnet string, sgs, disks []string) bulkControlEntry {
		return bulkControlEntry{nodeId: id, nodeInfo: model.NodeInfo{
			ImageId: image, SshKeyId: key, VNetId: vnet, SecurityGroupIds: sgs, DataDiskIds: disks,
		}}
	}
	entries := []bulkControlEntry{
		node("g1-1", "img-a", "key-1", "vnet-1", []string{"sg-1"}, nil),
		node("g1-2", "img-a", "key-1", "vnet-1", []string{"sg-1", "sg-2"}, []string{"disk-1"}),
		node("g2-1", "", "key-2", "vnet-1", nil, nil),
	}
	got := bulkAssociationKeys("ns1", "infra1", entries)

	k := func(nodeId string) string { return common.GenInfraKey("ns1", "infra1", nodeId) }
	want := map[bulkAssocRef][]string{
		{model.StrImage, "img-a"}:        {k("g1-1"), k("g1-2")},
		{model.StrCustomImage, "img-a"}:  {k("g1-1"), k("g1-2")},
		{model.StrSSHKey, "key-1"}:       {k("g1-1"), k("g1-2")},
		{model.StrSSHKey, "key-2"}:       {k("g2-1")},
		{model.StrVNet, "vnet-1"}:        {k("g1-1"), k("g1-2"), k("g2-1")},
		{model.StrSecurityGroup, "sg-1"}: {k("g1-1"), k("g1-2")},
		{model.StrSecurityGroup, "sg-2"}: {k("g1-2")},
		{model.StrDataDisk, "disk-1"}:    {k("g1-2")},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d resources, want %d: %v", len(got), len(want), got)
	}
	for ref, keys := range want {
		if !slices.Equal(got[ref], keys) {
			t.Errorf("%s %s: got %v, want %v", ref.resourceType, ref.resourceId, got[ref], keys)
		}
	}
	if len(bulkAssociationKeys("ns1", "infra1", nil)) != 0 {
		t.Error("no entries should give no resources")
	}
}
