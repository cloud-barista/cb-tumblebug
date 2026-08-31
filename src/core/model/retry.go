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

package model

// Retry actions reported per failed node.
const (
	// RetryActionInPlace re-creates the node with the identical configuration —
	// same zone, subnet, VNet, security group and key. CSP capacity fluctuates,
	// so the zone that just refused can accept the same request minutes later.
	RetryActionInPlace = "retryInPlace"
	// RetryActionNone means retrying cannot help: the account lacks quota or
	// permission, or the request itself has to be corrected first.
	RetryActionNone = "none"
)

// RetryTarget is one failed Node to retry, with the settings that apply to it.
// Per-Node settings live here rather than in maps keyed by Node id alongside a
// separate id list, so a setting can never name a Node that is not being retried.
type RetryTarget struct {
	NodeId string `json:"nodeId" validate:"required" example:"nvidial40s-1"`

	// Zone places this replacement in a chosen zone. The zone is resolved to a
	// subnet of the Node's own VNet — one is added there if the VNet has none yet
	// and has CIDR space left — so the replacement keeps the VPC, security group
	// and key. Empty keeps the zone the Node failed in.
	//
	// The review reports which zones the region offers, and which of them already
	// hold Running Nodes of the same NodeGroup.
	Zone string `json:"zone,omitempty" example:"us-west-2c"`

	// AssumeResolved retries a Node the classifier ruled out, asserting that the
	// condition blocking it was resolved outside CB-Tumblebug: a quota granted, a
	// permission fixed. The stored failure still says otherwise and nothing in
	// CB-Tumblebug can observe the change, so the caller has to say so.
	//
	// It cannot help a request that is simply wrong — an image the CSP does not
	// have, a disk too small for the flavor. Re-sending that unchanged fails
	// identically; correct the NodeGroup instead. The review reports, per Node,
	// which of the two a failure is.
	AssumeResolved bool `json:"assumeResolved,omitempty" example:"false"`
}

// RetryFailedNodesReq is the request body for the retry endpoints.
type RetryFailedNodesReq struct {
	// Targets are the failed Nodes to retry. Empty means every failed Node of the
	// Infra that the classifier considers retriable.
	Targets []RetryTarget `json:"targets,omitempty"`

	// AttemptsPerNode is how many times to re-create each Node before giving up.
	// Capacity comes and goes, so repeating the same request is the point.
	// Default 1, maximum 10.
	AttemptsPerNode int `json:"attemptsPerNode,omitempty" example:"3"`

	// IntervalSeconds is the pause between attempts on the same Node.
	// Default 30, maximum 600.
	IntervalSeconds int `json:"intervalSeconds,omitempty" example:"30"`

	// Parallelism is how many Nodes to retry at the same time.
	//
	// Unset retries as many at once as the CSP allows: the cap is the per-region
	// VM-creation limit infra provisioning already obeys, applied per connection,
	// so a burst stays inside an envelope the provider handles. An explicit value
	// is capped the same way rather than rejected. Ask for 1 to retry one Node at
	// a time, so each outcome is visible before the next starts.
	Parallelism int `json:"parallelism,omitempty" example:"0"`

	// KeepFailedNodes leaves the original failed Node records in place after a
	// successful replacement. They are deleted by default, since the replacement
	// takes over their role and the failure is preserved in the retry result.
	KeepFailedNodes bool `json:"keepFailedNodes,omitempty" example:"false"`
}

// RetryNodePlan is the verdict for one failed node.
type RetryNodePlan struct {
	NodeId      string `json:"nodeId" example:"nvidial40s-1"`
	NodeGroupId string `json:"nodeGroupId" example:"nvidial40s"`
	SpecId      string `json:"specId,omitempty"`
	ImageId     string `json:"imageId,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Region      string `json:"region,omitempty"`
	Zone        string `json:"zone,omitempty"`

	// Failure is re-derived from the stored raw CSP message rather than read
	// from the record, so parser improvements apply to failures recorded
	// before they existed.
	Failure *ProvisioningFailure `json:"failure,omitempty"`

	// AssumeResolvedHelps reports whether retrying this Node with assumeResolved
	// can succeed. True when the block is external and may have been lifted since
	// (quota, permission); false when the request itself has to change.
	AssumeResolvedHelps bool `json:"assumeResolvedHelps,omitempty"`

	// Action is one of the RetryAction* constants.
	Action string `json:"action" example:"retryInPlace"`
	// Reason explains the action in terms a user can act on.
	Reason string `json:"reason,omitempty"`
	// CostPerHour is what one replacement node will cost while it runs.
	CostPerHour float64 `json:"costPerHour,omitempty"`

	// SubnetId is the subnet the failed Node was placed in. A NodeGroup spread
	// across subnets (DistributeSubnets) has Nodes in different zones, so this is
	// what decides where a replacement lands.
	SubnetId string `json:"subnetId,omitempty"`
	// SiblingSubnetId is another subnet of the same VNet where Nodes of this
	// NodeGroup are Running — live evidence that its zone has capacity. Set
	// that zone as a target's zone to place the replacement there instead.
	SiblingSubnetId string `json:"siblingSubnetId,omitempty"`
	// SiblingZone is that subnet's zone.
	SiblingZone string `json:"siblingZone,omitempty"`
	// SiblingRunningCount is how many Nodes of this NodeGroup run there.
	SiblingRunningCount int `json:"siblingRunningCount,omitempty"`

	// Escalation describes what to do if repeated in-place retries keep failing.
	// It is advice only: moving a node to another zone puts it in a different
	// VNet, so it is a new NodeGroup rather than a retry, and the user has to
	// choose it deliberately.
	Escalation string `json:"escalation,omitempty"`
	// ZoneCapability reports whether another zone is even an option here.
	ZoneCapability *ZoneCapability `json:"zoneCapability,omitempty"`
}

// RetryFailedNodesReview is the response of the review endpoint.
type RetryFailedNodesReview struct {
	InfraId string `json:"infraId"`
	// Plans covers every failed node, retriable or not, so the caller can show
	// why the excluded ones were excluded.
	Plans []RetryNodePlan `json:"plans"`

	RetriableCount    int     `json:"retriableCount"`
	NotRetriableCount int     `json:"notRetriableCount"`
	CostPerHourIfAll  float64 `json:"costPerHourIfAll,omitempty"`
	Message           string  `json:"message,omitempty"`
}

// RetryNodeResult is the outcome of retrying one node.
type RetryNodeResult struct {
	NodeId      string `json:"nodeId"`
	NodeGroupId string `json:"nodeGroupId"`
	// NewNodeId is the replacement node. A replacement takes the next free index
	// in the NodeGroup; an attempt that fails releases its name again, so
	// repeated attempts reuse one index instead of climbing.
	NewNodeId string `json:"newNodeId,omitempty"`
	Succeeded bool   `json:"succeeded"`
	Attempts  int    `json:"attempts"`
	Skipped   bool   `json:"skipped,omitempty"`
	Reason    string `json:"reason,omitempty"`
	// LastFailure is the classified failure of the final unsuccessful attempt.
	LastFailure *ProvisioningFailure `json:"lastFailure,omitempty"`
	// FailedNodeRemoved reports whether the original failed record was deleted.
	FailedNodeRemoved bool `json:"failedNodeRemoved,omitempty"`
	// PlacedInSubnetId is the subnet the replacement was requested in, which
	// differs from the failed Node's when the target named another zone.
	PlacedInSubnetId string `json:"placedInSubnetId,omitempty"`
	// PlacedInZone is that subnet's zone.
	PlacedInZone   string `json:"placedInZone,omitempty"`
	ElapsedSeconds int64  `json:"elapsedSeconds,omitempty"`
}

// RetryFailedNodesResult is the response of the execute endpoint.
type RetryFailedNodesResult struct {
	InfraId        string            `json:"infraId"`
	Results        []RetryNodeResult `json:"results"`
	SucceededCount int               `json:"succeededCount"`
	FailedCount    int               `json:"failedCount"`
	SkippedCount   int               `json:"skippedCount"`
	// ParallelismUsed is the concurrency actually applied per CSP region after
	// the provider caps were enforced.
	ParallelismUsed map[string]int `json:"parallelismUsed,omitempty"`
	ElapsedSeconds  int64          `json:"elapsedSeconds"`
	InfraStatus     string         `json:"infraStatus,omitempty"`
	Message         string         `json:"message,omitempty"`
}
