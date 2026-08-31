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

package infra

// Re-creating a NodeGroup whose Nodes all failed, with corrected settings.
//
// Some failures cannot be retried at all: an image the CSP does not have, a root
// disk too small for the flavor. Re-sending the same request fails identically —
// the request itself has to change. Until now that meant deleting the NodeGroup
// by hand and adding another under a different name, because creating one rejects
// a name that is already taken.
//
// Nothing new is provisioned here. Clearing the last Node of a NodeGroup already
// removes the NodeGroup record, which frees the name, so this clears the failed
// Nodes and calls the ordinary dynamic creation path. Going through that path is
// what makes a corrected root disk stick: the NodeGroup record is rebuilt from the
// new request, where overwriting fields on the existing record would leave the
// next scale-out reading the old value.

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/rs/zerolog/log"
)

var (
	// ErrNodeGroupInUse means the NodeGroup still has Nodes that are not failed.
	// Replacing it would destroy working machines.
	ErrNodeGroupInUse = errors.New("node group still has nodes that are not failed")
	// ErrNodeGroupHasCspResource means a failed Node may still own a CSP resource.
	// CB-Tumblebug cannot tell from the record alone, and reconcile/refine exist
	// precisely to settle that.
	ErrNodeGroupHasCspResource = errors.New("a failed node may still hold a CSP resource")
	// ErrNodeGroupNameMismatch means the body names a different NodeGroup than the
	// path does — a client mistake, not a state conflict.
	ErrNodeGroupNameMismatch = errors.New("the request names a different node group than the path")
	// ErrNodeGroupNotFound means there is no such NodeGroup to replace.
	ErrNodeGroupNotFound = errors.New("node group not found")
	// ErrSpecChanged means the request carries a different spec. Instance type
	// decides cost, performance and availability at once; changing it is a new
	// NodeGroup, not a correction of this one.
	ErrSpecChanged = errors.New("the spec of an existing node group cannot be changed")
)

// ReplaceFailedNodeGroup clears a NodeGroup whose Nodes have all failed and
// re-creates it under the same name with the given request.
//
// It refuses rather than guessing whenever clearing could destroy something: a
// Node that is not failed, or a failed Node that may still own a CSP resource.
// The removed Nodes' failures are returned so the reason for the correction is
// not lost with the records.
func ReplaceFailedNodeGroup(ctx context.Context, nsId, infraId, nodeGroupId string, req *model.AddNodeGroupDynamicReq) (*model.ReplaceNodeGroupResult, error) {
	if req == nil {
		return nil, fmt.Errorf("%w: a node group request is required", ErrNodeGroupNameMismatch)
	}
	if req.Name == "" {
		req.Name = nodeGroupId
	}
	if !strings.EqualFold(req.Name, nodeGroupId) {
		return nil, fmt.Errorf("%w: body says '%s', path says '%s'", ErrNodeGroupNameMismatch, req.Name, nodeGroupId)
	}

	nodeIds, err := ListNodeByNodeGroup(nsId, infraId, nodeGroupId)
	if err != nil {
		return nil, fmt.Errorf("cannot list nodes of node group '%s': %w", nodeGroupId, err)
	}
	if len(nodeIds) == 0 {
		return nil, fmt.Errorf("%w: '%s' has no node in infra '%s'; create it instead", ErrNodeGroupNotFound, nodeGroupId, infraId)
	}

	removed := make([]model.RemovedFailedNode, 0, len(nodeIds))
	existingSpecId := ""
	for _, nodeId := range nodeIds {
		node, err := GetNodeObject(nsId, infraId, nodeId)
		if err != nil {
			return nil, fmt.Errorf("cannot read node '%s': %w", nodeId, err)
		}
		if !strings.EqualFold(node.Status, model.StatusFailed) {
			return nil, fmt.Errorf("%w: '%s' is %s. To change the configuration of a running group, add a new node group instead",
				ErrNodeGroupInUse, node.Id, node.Status)
		}
		if node.CspResourceName != "" || node.CspResourceId != "" {
			return nil, fmt.Errorf("%w: '%s' still names a CSP resource. Run action=reconcile to rescue it or action=refine to remove it, then retry",
				ErrNodeGroupHasCspResource, node.Id)
		}
		if existingSpecId == "" {
			existingSpecId = node.SpecId
		}
		removed = append(removed, model.RemovedFailedNode{NodeId: node.Id, Failure: reclassify(node)})
	}

	if req.SpecId != "" && existingSpecId != "" && req.SpecId != existingSpecId {
		return nil, fmt.Errorf("%w: '%s' is on '%s'. Add a new node group for '%s' instead",
			ErrSpecChanged, nodeGroupId, existingSpecId, req.SpecId)
	}
	if req.SpecId == "" {
		req.SpecId = existingSpecId
	}
	// A zone on a dynamic request derives a zone-scoped shared VNet, which would
	// put the new nodes in a separate VPC from the rest of the Infra.
	if req.Zone != "" {
		log.Warn().Msgf("ReplaceFailedNodeGroup: ignoring zone '%s' for '%s'; a zone-pinned request would build a separate VNet", req.Zone, nodeGroupId)
		req.Zone = ""
	}
	if req.NodeGroupSize < 1 {
		req.NodeGroupSize = len(nodeIds)
	}

	// Clearing the last node also removes the NodeGroup record, which is what
	// frees the name for the creation below.
	for _, r := range removed {
		if err := DelInfraNode(nsId, infraId, r.NodeId, "force"); err != nil {
			return nil, fmt.Errorf("cannot clear failed node '%s': %w", r.NodeId, err)
		}
	}
	log.Info().Msgf("ReplaceFailedNodeGroup: cleared %d failed node(s) of '%s' in infra '%s'", len(removed), nodeGroupId, infraId)

	infraInfo, err := CreateInfraNodeGroupDynamic(ctx, nsId, infraId, req)
	result := &model.ReplaceNodeGroupResult{
		InfraId:      infraId,
		NodeGroupId:  nodeGroupId,
		RemovedNodes: removed,
		Infra:        infraInfo,
	}
	if err != nil {
		// The failed records are gone and the new nodes did not come up. Nothing
		// existed on the CSP to lose, but say so plainly: the caller's next move
		// is to fix the request and create the node group again.
		result.Message = fmt.Sprintf("cleared %d failed node(s), but re-creating '%s' failed: %v",
			len(removed), nodeGroupId, err)
		return result, err
	}
	result.Message = fmt.Sprintf("replaced %d failed node(s) of '%s'", len(removed), nodeGroupId)
	return result, nil
}
