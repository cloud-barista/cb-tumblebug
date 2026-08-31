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

// Retrying nodes that failed to provision.
//
// A CSP capacity refusal is transient: g6e.2xlarge was refused twice in
// us-west-2 and accepted in the same zone minutes later. So the retry re-creates
// the node with the identical configuration — same zone, subnet, VNet, security
// group and key — and can repeat that as often as the caller asks.
//
// Moving a node to another zone is deliberately NOT part of this: passing a zone
// to a dynamic request builds a zone-scoped shared VNet, which would put the
// replacement in a different VPC from its siblings. That is a new NodeGroup, not
// a retry, and the review only mentions it as advice.
//
// Two ordering rules matter:
//   - The failed node is the configuration template (ScaleOutInfraNodeGroup
//     copies from it), so it must not be deleted before the replacement exists.
//     A single-node NodeGroup would otherwise lose its configuration entirely.
//   - Failures are re-classified from the stored raw CSP message instead of the
//     stored class, so parser improvements apply to older failures too.

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	cspcheck "github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/rs/zerolog/log"
)

const (
	retryDefaultAttempts = 1
	retryMaxAttempts     = 10
	retryDefaultInterval = 30 * time.Second
	retryMaxInterval     = 600 * time.Second
)

// activeRetryRuns guards against two concurrent retries on the same Infra, which
// would both scale out the same NodeGroup and double-provision.
var activeRetryRuns sync.Map // key: "{nsId}/{infraId}" -> struct{}

// ReviewRetryFailedNodes reports, for every failed node in the Infra, whether
// re-creating it can plausibly succeed and why. It creates nothing.
func ReviewRetryFailedNodes(nsId, infraId string, req *model.RetryFailedNodesReq) (*model.RetryFailedNodesReview, error) {
	if req == nil {
		req = &model.RetryFailedNodesReq{}
	}
	failedNodes, err := listFailedNodes(nsId, infraId, req.NodeIds)
	if err != nil {
		return nil, err
	}

	review := &model.RetryFailedNodesReview{
		InfraId: infraId,
		Plans:   make([]model.RetryNodePlan, 0, len(failedNodes)),
	}
	for _, node := range failedNodes {
		plan := planRetryForNode(nsId, infraId, node, req.Force)
		if plan.Action == model.RetryActionInPlace {
			review.RetriableCount++
			review.CostPerHourIfAll += plan.CostPerHour
		} else {
			review.NotRetriableCount++
		}
		review.Plans = append(review.Plans, plan)
	}

	switch {
	case len(failedNodes) == 0:
		review.Message = "no failed node in this infra"
	case review.RetriableCount == 0:
		review.Message = "none of the failed nodes can be fixed by retrying; see each plan's reason"
	default:
		review.Message = fmt.Sprintf("%d of %d failed node(s) can be retried in place",
			review.RetriableCount, len(failedNodes))
	}
	return review, nil
}

// RetryFailedNodes re-creates the retriable failed nodes, one node at a time so
// each outcome is visible before the next is attempted and a partial success is
// kept. Nodes the review excluded are skipped with their reason.
func RetryFailedNodes(ctx context.Context, nsId, infraId string, req *model.RetryFailedNodesReq) (*model.RetryFailedNodesResult, error) {
	if req == nil {
		req = &model.RetryFailedNodesReq{}
	}
	attempts := req.AttemptsPerNode
	if attempts < 1 {
		attempts = retryDefaultAttempts
	}
	if attempts > retryMaxAttempts {
		attempts = retryMaxAttempts
	}
	interval := time.Duration(req.IntervalSeconds) * time.Second
	if req.IntervalSeconds <= 0 {
		interval = retryDefaultInterval
	}
	if interval > retryMaxInterval {
		interval = retryMaxInterval
	}

	runKey := nsId + "/" + infraId
	if _, busy := activeRetryRuns.LoadOrStore(runKey, struct{}{}); busy {
		return nil, fmt.Errorf("a retry is already running for infra '%s'", infraId)
	}
	defer activeRetryRuns.Delete(runKey)

	failedNodes, err := listFailedNodes(nsId, infraId, req.NodeIds)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	result := &model.RetryFailedNodesResult{
		InfraId:         infraId,
		Results:         make([]model.RetryNodeResult, 0, len(failedNodes)),
		ParallelismUsed: map[string]int{},
	}

	// Plan every node up front so skips are reported without occupying a worker.
	type job struct {
		node model.NodeInfo
		plan model.RetryNodePlan
	}
	byConnection := map[string][]job{}
	for _, node := range failedNodes {
		plan := planRetryForNode(nsId, infraId, node, req.Force)
		if plan.Action != model.RetryActionInPlace {
			result.Results = append(result.Results, model.RetryNodeResult{
				NodeId: node.Id, NodeGroupId: node.NodeGroupId,
				Skipped: true, Reason: plan.Reason,
				LastFailure: plan.Failure,
			})
			result.SkippedCount++
			continue
		}
		byConnection[node.ConnectionName] = append(byConnection[node.ConnectionName], job{node: node, plan: plan})
	}

	// Nodes of one connection share a CSP region, so its per-region VM-creation
	// limit is the right ceiling — the same one infra provisioning obeys. Distinct
	// connections run concurrently under the global connection cap.
	var mu sync.Mutex
	var connWg sync.WaitGroup
	connSlots := make(chan struct{}, maxConcurrentConnections(len(byConnection)))

	for connectionName, jobs := range byConnection {
		limit := effectiveParallelism(req.Parallelism, connectionName, len(jobs))
		result.ParallelismUsed[connectionName] = limit

		connWg.Add(1)
		go func(connectionName string, jobs []job, limit int) {
			defer connWg.Done()
			connSlots <- struct{}{}
			defer func() { <-connSlots }()

			nodeSlots := make(chan struct{}, limit)
			var nodeWg sync.WaitGroup
			for _, j := range jobs {
				if ctx.Err() != nil {
					break
				}
				nodeWg.Add(1)
				go func(j job) {
					defer nodeWg.Done()
					nodeSlots <- struct{}{}
					defer func() { <-nodeSlots }()

					subnetOverride := ""
					if req.PreferAvailableSubnet && j.plan.SiblingSubnetId != "" {
						subnetOverride = j.plan.SiblingSubnetId
						log.Info().Msgf("retry: placing the replacement for '%s' in '%s' (zone %s), where %d sibling node(s) are running",
							j.node.Id, j.plan.SiblingSubnetId, j.plan.SiblingZone, j.plan.SiblingRunningCount)
					}
					r := retryOneNode(ctx, nsId, infraId, j.node, attempts, interval, req.KeepFailedNodes, subnetOverride)

					mu.Lock()
					result.Results = append(result.Results, r)
					if r.Succeeded {
						result.SucceededCount++
					} else {
						result.FailedCount++
					}
					mu.Unlock()
				}(j)
			}
			nodeWg.Wait()
		}(connectionName, jobs, limit)
	}
	connWg.Wait()

	result.ElapsedSeconds = int64(time.Since(start).Seconds())
	if infraStatus, err := GetInfraStatus(nsId, infraId); err == nil && infraStatus != nil {
		result.InfraStatus = infraStatus.Status
	}
	result.Message = fmt.Sprintf("%d succeeded, %d still failing, %d skipped",
		result.SucceededCount, result.FailedCount, result.SkippedCount)
	return result, nil
}

// retryOneNode re-creates a single failed node by scaling its NodeGroup out by
// one, which copies the configuration from the failed node itself.
func retryOneNode(ctx context.Context, nsId, infraId string, failed model.NodeInfo,
	attempts int, interval time.Duration, keepFailed bool, subnetOverride string) model.RetryNodeResult {

	start := time.Now()
	r := model.RetryNodeResult{NodeId: failed.Id, NodeGroupId: failed.NodeGroupId}
	r.PlacedInSubnetId = failed.SubnetId
	if subnetOverride != "" {
		r.PlacedInSubnetId = subnetOverride
	}

	for attempt := 1; attempt <= attempts; attempt++ {
		if ctx.Err() != nil {
			r.Reason = "cancelled"
			break
		}
		r.Attempts = attempt

		// The failed node is the template, not whichever node the KV scan happens to
		// return first: in a NodeGroup spread across subnets the copied SubnetId is
		// what decides the zone, so an arbitrary source would place the replacement
		// unpredictably.
		_, createdIds, err := ScaleOutInfraNodeGroupFrom(ctx, nsId, infraId, failed.NodeGroupId, 1, failed.Id, subnetOverride)
		newNodeId := ""
		if len(createdIds) > 0 {
			newNodeId = createdIds[0]
		}

		if err == nil && newNodeId != "" {
			if node, getErr := GetNodeObject(nsId, infraId, newNodeId); getErr == nil &&
				!strings.EqualFold(node.Status, model.StatusFailed) {
				r.Succeeded = true
				r.NewNodeId = newNodeId
				break
			}
		}

		// The attempt produced a node record that also failed; classify it and
		// clear it away so the next attempt starts from a clean group.
		if newNodeId != "" {
			if node, getErr := GetNodeObject(nsId, infraId, newNodeId); getErr == nil {
				r.LastFailure = reclassify(node)
				if delErr := DelInfraNode(nsId, infraId, newNodeId, "force"); delErr != nil {
					log.Warn().Err(delErr).Msgf("retry: could not remove the failed replacement node '%s'", newNodeId)
				}
			}
		}
		if r.LastFailure == nil && err != nil {
			f := cspcheck.ClassifyProvisioningFailure(providerOf(failed), failed.Region.Region, failed.Region.Zone, err.Error())
			r.LastFailure = &f
		}

		// A failure that retrying cannot fix ends the loop early rather than
		// burning the remaining attempts on the same rejection.
		if r.LastFailure != nil && !r.LastFailure.Retryable {
			r.Reason = "stopped early: " + r.LastFailure.Message
			break
		}
		if attempt < attempts {
			log.Info().Msgf("retry: node '%s' attempt %d/%d failed; waiting %s", failed.Id, attempt, attempts, interval)
			select {
			case <-ctx.Done():
			case <-time.After(interval):
			}
		}
	}

	if r.Succeeded && !keepFailed {
		// Only now is the failed record expendable: it was the configuration
		// template for the replacement that just came up.
		if err := DelInfraNode(nsId, infraId, failed.Id, "force"); err != nil {
			log.Warn().Err(err).Msgf("retry: replacement '%s' is up but the failed node '%s' could not be removed", r.NewNodeId, failed.Id)
		} else {
			r.FailedNodeRemoved = true
		}
	}
	r.ElapsedSeconds = int64(time.Since(start).Seconds())
	return r
}

// planRetryForNode decides what can be done about one failed node.
func planRetryForNode(nsId, infraId string, node model.NodeInfo, force bool) model.RetryNodePlan {
	failure := reclassify(node)
	plan := model.RetryNodePlan{
		NodeId:      node.Id,
		NodeGroupId: node.NodeGroupId,
		SpecId:      node.SpecId,
		ImageId:     node.ImageId,
		Provider:    providerOf(node),
		Region:      node.Region.Region,
		Zone:        node.Region.Zone,
		Failure:     failure,
	}

	plan.SubnetId = node.SubnetId
	if sib, ok := findRunningSiblingSubnet(nsId, infraId, node); ok {
		plan.SiblingSubnetId = sib.subnetId
		plan.SiblingZone = sib.zone
		plan.SiblingRunningCount = sib.runningCount
	}

	// Cost is informational; never look up an empty spec key.
	if node.SpecId != "" {
		if spec, err := resource.GetSpec(model.SystemCommonNs, node.SpecId); err == nil {
			plan.CostPerHour = float64(spec.CostPerHour)
		}
	}

	switch {
	case failure == nil:
		plan.Action = model.RetryActionInPlace
		plan.Reason = "the failure was not recorded; a single attempt will show whether it recurs"

	case failure.Retryable:
		plan.Action = model.RetryActionInPlace
		plan.Reason = retriableReason(failure)
		if plan.SiblingSubnetId != "" && failure.Class == model.FailureZoneCapacity {
			plan.Reason += fmt.Sprintf(
				"; %d node(s) of this NodeGroup are running in %s (subnet %s) of the same VNet — set preferAvailableSubnet to place the replacement there instead, which keeps the same VPC, security group and key",
				plan.SiblingRunningCount, plan.SiblingZone, plan.SiblingSubnetId)
		}

	case force:
		plan.Action = model.RetryActionInPlace
		plan.Reason = "forced by request despite: " + failure.Message

	default:
		plan.Action = model.RetryActionNone
		plan.Reason = notRetriableReason(failure)
	}

	// Zone advice belongs only to a zone-specific shortage, and only where the
	// CSP and the region actually allow choosing a zone.
	if failure != nil && failure.Class == model.FailureZoneCapacity {
		zc := common.ResolveZoneCapability(node.ConnectionName)
		plan.ZoneCapability = &zc
		plan.Escalation = zoneEscalationText(plan.Region, zc)
	}
	return plan
}

func retriableReason(f *model.ProvisioningFailure) string {
	switch f.Class {
	case model.FailureZoneCapacity:
		zone := f.AttemptedZone
		if zone == "" {
			zone = "the requested zone"
		}
		return fmt.Sprintf("%s had no capacity for this spec at the time; capacity is released continuously, so the same request can be accepted later", zone)
	case model.FailureThrottling:
		return "the CSP was rate-limiting requests, not rejecting this one on its merits"
	case model.FailureNetwork:
		return "the request did not reach the CSP; it was never evaluated"
	case model.FailureUnknown:
		return "the cause is not recognized; one attempt will show whether it recurs"
	default:
		return f.Message
	}
}

func notRetriableReason(f *model.ProvisioningFailure) string {
	switch f.RetryHint {
	case model.RetryHintDifferentImage:
		return "this image cannot be used with this spec; choose another image and add a NodeGroup — " + f.Message
	case model.RetryHintAdjustRequest:
		return "the request itself was rejected; correct it and add a NodeGroup — " + f.Message
	case model.RetryHintDifferentRegion:
		return "the whole region is short of this resource; another region is needed — " + f.Message
	default:
		return "retrying cannot change the outcome — " + f.Message
	}
}

// reclassify derives the failure from the stored raw CSP text rather than the
// stored class, so a node that failed before its provider had a parser is
// classified with today's rules.
func reclassify(node model.NodeInfo) *model.ProvisioningFailure {
	raw := ""
	if node.Failure != nil && node.Failure.RawMessage != "" {
		raw = node.Failure.RawMessage
	} else if node.SystemMessage != "" {
		raw = node.SystemMessage
	}
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	zone := node.Region.Zone
	if zone == "" && node.Failure != nil {
		zone = node.Failure.AttemptedZone
	}
	f := cspcheck.ClassifyProvisioningFailure(providerOf(node), node.Region.Region, zone, raw)
	return &f
}

func providerOf(node model.NodeInfo) string {
	if node.ConnectionConfig.ProviderName != "" {
		return node.ConnectionConfig.ProviderName
	}
	if cc, err := common.GetConnConfig(node.ConnectionName); err == nil {
		return cc.ProviderName
	}
	return ""
}

// listFailedNodes returns the Infra's failed nodes, optionally narrowed to the
// requested ids.
func listFailedNodes(nsId, infraId string, only []string) ([]model.NodeInfo, error) {
	nodeIds, err := ListNodeId(nsId, infraId)
	if err != nil {
		return nil, fmt.Errorf("cannot list nodes of infra '%s': %w", infraId, err)
	}
	wanted := map[string]bool{}
	for _, id := range only {
		wanted[id] = true
	}

	failed := []model.NodeInfo{}
	for _, nodeId := range nodeIds {
		if len(wanted) > 0 && !wanted[nodeId] {
			continue
		}
		node, err := GetNodeObject(nsId, infraId, nodeId)
		if err != nil {
			log.Warn().Err(err).Msgf("retry: cannot read node '%s'", nodeId)
			continue
		}
		if strings.EqualFold(node.Status, model.StatusFailed) {
			failed = append(failed, node)
		}
	}
	return failed, nil
}

// zoneEscalationText is the advice shown when in-place retries keep failing.
// It always states the VNet consequence: a zone-pinned NodeGroup is created in
// its own shared VNet, so its nodes cannot reach the rest of the Infra over the
// private network.
func zoneEscalationText(region string, zc model.ZoneCapability) string {
	if !zc.Shiftable {
		return "moving to another zone is not possible here: " + zc.Reason +
			"; a different region or spec is the remaining option"
	}
	return fmt.Sprintf(
		"if repeated attempts keep failing, add a NodeGroup in another zone of %s (%s); "+
			"note that a zone-pinned NodeGroup gets its own VNet, so it will not share a private network with this Infra's other nodes",
		region, strings.Join(zc.Zones, ", "))
}

// siblingSubnet describes a subnet of the same VNet where NodeGroup peers run.
type siblingSubnet struct {
	subnetId     string
	zone         string
	runningCount int
}

// findRunningSiblingSubnet looks for another subnet of the failed node's NodeGroup
// that currently holds Running nodes. A running peer is the strongest capacity
// signal available — stronger than a CSP's suggested-zone list, which was observed
// to name zones that refused the very next request — and staying inside the VNet
// keeps the replacement on the Infra's private network. It is still only evidence:
// a zone that accepted a node minutes ago can refuse the retry.
//
// The busiest such subnet wins, since more running peers is more evidence.
func findRunningSiblingSubnet(nsId, infraId string, failed model.NodeInfo) (siblingSubnet, bool) {
	if failed.NodeGroupId == "" {
		return siblingSubnet{}, false
	}
	peerIds, err := ListNodeByNodeGroup(nsId, infraId, failed.NodeGroupId)
	if err != nil {
		return siblingSubnet{}, false
	}

	counts := map[string]int{}
	zones := map[string]string{}
	for _, peerId := range peerIds {
		if peerId == failed.Id {
			continue
		}
		peer, err := GetNodeObject(nsId, infraId, peerId)
		if err != nil || !strings.EqualFold(peer.Status, model.StatusRunning) {
			continue
		}
		// Only a different subnet of the same VNet: another VNet would take the
		// replacement off the Infra's private network.
		if peer.SubnetId == "" || peer.SubnetId == failed.SubnetId || peer.VNetId != failed.VNetId {
			continue
		}
		counts[peer.SubnetId]++
		zones[peer.SubnetId] = peer.Region.Zone
	}

	best := siblingSubnet{}
	for subnetId, count := range counts {
		if count > best.runningCount {
			best = siblingSubnet{subnetId: subnetId, zone: zones[subnetId], runningCount: count}
		}
	}
	return best, best.runningCount > 0
}

// effectiveParallelism caps a requested concurrency by what the CSP tolerates.
// MaxNodesPerRegion is the limit infra provisioning already uses for creating VMs
// in one region, so a retry burst stays within the same envelope — Tencent, for
// instance, allows far fewer parallel creates than AWS.
func effectiveParallelism(requested int, connectionName string, jobCount int) int {
	if requested < 1 {
		requested = 1
	}
	if requested > jobCount {
		requested = jobCount
	}
	provider := ""
	if cc, err := common.GetConnConfig(connectionName); err == nil {
		provider = cc.ProviderName
	}
	if limit := csp.GetRateLimitConfig(provider).MaxNodesPerRegion; limit > 0 && requested > limit {
		log.Info().Msgf("retry: capping parallelism for '%s' from %d to %d (%s limit)",
			connectionName, requested, limit, provider)
		requested = limit
	}
	if requested < 1 {
		requested = 1
	}
	return requested
}

// maxConcurrentConnections bounds how many CSP connections are worked on at once,
// reusing the global cap that connection-wide operations already respect.
func maxConcurrentConnections(connectionCount int) int {
	if connectionCount < 1 {
		return 1
	}
	if connectionCount > csp.GlobalMaxConcurrentConnections {
		return csp.GlobalMaxConcurrentConnections
	}
	return connectionCount
}
