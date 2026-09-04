package infra

// Plans are checked against the five real mc-pirate failures: only the AWS
// capacity refusal may be retried in place.

import (
	"os"
	"strings"
	"testing"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
)

func nodeWith(id, group, provider, region, zone, msg string) model.NodeInfo {
	n := model.NodeInfo{
		Id: id, NodeGroupId: group, Status: model.StatusFailed,
		SystemMessage: msg,
	}
	n.ConnectionConfig.ProviderName = provider
	n.Region.Region = region
	n.Region.Zone = zone
	return n
}

func TestPlanRetryForNode(t *testing.T) {
	tests := []struct {
		name       string
		node       model.NodeInfo
		wantAction string
	}{
		{
			"aws capacity is transient",
			nodeWith("nvidial40s-1", "nvidial40s", "aws", "us-west-2", "us-west-2d",
				"InsufficientInstanceCapacity: We currently do not have sufficient g6e.2xlarge capacity in the Availability Zone you requested (us-west-2d). status code: 500"),
			model.RetryActionInPlace,
		},
		{
			"ncp entitlement cannot be retried",
			nodeWith("g9-1", "g9", "ncp", "kr", "KR-1",
				"Failed to Create VM instance : [Status: 400 Bad Request, Body: { responseError: { returnCode: 1153027, returnMessage: Server (VPC) product generation limit exceeded. Product Type: GPU - T4 - G1 Creation Limit:0 / Usage:0 / Creation Request:1 } }]"),
			model.RetryActionNone,
		},
		{
			"tencent missing image cannot be retried",
			nodeWith("g11-1", "g11", "tencent", "ap-seoul", "ap-seoul-2",
				"[TencentCloudSDKError] Code=InvalidImageId.NotFound, Message=ImageId img-7rotv4ux is not found, RequestId=ac88e205"),
			model.RetryActionNone,
		},
		{
			"alibaba incompatible image cannot be retried",
			nodeWith("g4-1", "g4", "alibaba", "us-east-1", "us-east-1b",
				"SDK.ServerError ErrorCode: InvalidParameter.NotMatch RequestId: 01A0553F Message: The specified instance type ecs.gn6i-c4g1.xlarge only supports some specific images."),
			model.RetryActionNone,
		},
		{
			"nhn disk size cannot be retried",
			nodeWith("g10-1", "g10", "nhn", "kr1", "kr-pub-b",
				"Failed to Create a VM with the Block Storage Volume!! [Bad request with: [POST https://kr1-api.example/servers], error message: {badRequest:{message:Volume size is too small.,code:400}}]"),
			model.RetryActionNone,
		},
		{
			"an unrecognized failure earns one attempt",
			nodeWith("x-1", "x", "kt", "kr", "",
				"something nobody has taught us yet"),
			model.RetryActionInPlace,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := planRetryForNode("default", "infra01", tt.node, false)
			if plan.Action != tt.wantAction {
				t.Errorf("action = %q, want %q (reason: %s)", plan.Action, tt.wantAction, plan.Reason)
			}
			if plan.Reason == "" {
				t.Error("every plan must carry a reason the user can act on")
			}
		})
	}
}

// assumeResolved exists for blocks lifted outside CB-Tumblebug (a quota granted).
func TestPlanRetryForNodeAssumeResolved(t *testing.T) {
	node := nodeWith("g9-1", "g9", "ncp", "kr", "KR-1",
		"returnCode: 1153027, returnMessage: Server (VPC) product generation limit exceeded. Product Type: GPU - T4 - G1 Creation Limit:0 / Usage:0 / Creation Request:1 }")
	if got := planRetryForNode("default", "infra01", node, true).Action; got != model.RetryActionInPlace {
		t.Errorf("action = %q, want %q with assumeResolved", got, model.RetryActionInPlace)
	}
}

// assumeResolved can only help a block that something outside CB-Tumblebug can
// lift. A wrong request stays wrong however often it is re-sent, and the review
// has to say which of the two a Node hit.
func TestAssumeResolvedHelpsOnlyExternalBlocks(t *testing.T) {
	tests := []struct {
		name string
		node model.NodeInfo
		want bool
	}{
		{"ncp quota may be granted later", nodeWith("g9-1", "g9", "ncp", "kr", "KR-1",
			"returnCode: 1153027, returnMessage: Server (VPC) product generation limit exceeded. Product Type: GPU - T4 - G1 Creation Limit:0 / Usage:0 / Creation Request:1 }"), true},
		{"a missing image needs a different image", nodeWith("g11-1", "g11", "tencent", "ap-seoul", "ap-seoul-2",
			"[TencentCloudSDKError] Code=InvalidImageId.NotFound, Message=ImageId img-7rotv4ux is not found, RequestId=ac88e205"), false},
		{"a disk too small needs a corrected request", nodeWith("g10-1", "g10", "nhn", "kr1", "kr-pub-b",
			"Failed to Create a VM with the Block Storage Volume!! [Bad request with: [POST https://kr1-api.example/servers], error message: {badRequest:{message:Volume size is too small.,code:400}}]"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := planRetryForNode("default", "infra01", tt.node, false).AssumeResolvedHelps; got != tt.want {
				t.Errorf("assumeResolvedHelps = %v, want %v", got, tt.want)
			}
		})
	}
}

// A node whose failure predates its provider's parser must be classified with
// today's rules, not with whatever was stored at the time.
func TestReclassifyUsesCurrentParsers(t *testing.T) {
	node := nodeWith("g11-1", "g11", "tencent", "ap-seoul", "ap-seoul-2",
		"[TencentCloudSDKError] Code=InvalidImageId.NotFound, Message=ImageId img-7rotv4ux is not found, RequestId=ac88e205")
	node.Failure = &model.ProvisioningFailure{
		Class:      model.FailureUnknown, // what was stored before the parser existed
		Retryable:  true,
		RetryHint:  model.RetryHintSameConfig,
		RawMessage: node.SystemMessage,
	}
	f := reclassify(node)
	if f.Class != model.FailureImageSpecMismatch {
		t.Errorf("class = %q, want %q — the stored class must not win over the raw message", f.Class, model.FailureImageSpecMismatch)
	}
	if f.Retryable {
		t.Error("re-classification must also correct retryability")
	}
}

// The escalation text has to warn that a zone-pinned NodeGroup lands in its own
// VNet, since that costs the node its private path to the rest of the Infra.
func TestZoneEscalationText(t *testing.T) {
	shiftable := zoneEscalationText("us-west-2", model.ZoneCapability{
		ZoneControl: true, Shiftable: true,
		Zones: []string{"us-west-2a", "us-west-2b", "us-west-2c", "us-west-2d"},
	})
	if !strings.Contains(shiftable, "retry target") {
		t.Errorf("escalation should point at the target's zone: %q", shiftable)
	}
	if !strings.Contains(shiftable, "same VNet") {
		t.Errorf("escalation should say the replacement stays in the same VNet: %q", shiftable)
	}
	if !strings.Contains(shiftable, "us-west-2c") {
		t.Errorf("escalation should list the candidate zones: %q", shiftable)
	}

	// Azure westus has no zones at all; the advice must say so rather than
	// suggesting a move that cannot happen.
	blocked := zoneEscalationText("westus", model.ZoneCapability{
		ZoneControl: true, Shiftable: false,
		Reason: "region 'westus' has no availability zones",
	})
	if !strings.Contains(blocked, "no availability zones") {
		t.Errorf("escalation must explain why a zone move is impossible: %q", blocked)
	}
}

// Parallelism must stay inside the per-CSP creation limits that infra
// provisioning already respects, so a retry burst cannot trip a rate limit that
// normal provisioning avoids.
func TestEffectiveParallelism(t *testing.T) {
	tests := []struct {
		name      string
		requested int
		jobCount  int
		want      int
	}{
		{"unset means as many as the CSP allows", 0, 5, 5},
		{"negative is treated as unset", -3, 5, 5},
		{"never more workers than jobs", 10, 3, 3},
		{"honours an explicit value", 3, 5, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// An unknown connection falls back to the default CSP config
			// (MaxNodesPerRegion 20), so these cases are not clipped by it.
			if got := effectiveParallelism(tt.requested, "no-such-connection", tt.jobCount); got != tt.want {
				t.Errorf("effectiveParallelism(%d, jobs=%d) = %d, want %d", tt.requested, tt.jobCount, got, tt.want)
			}
		})
	}

	// The default config caps at 20 regardless of what was asked for.
	if got := effectiveParallelism(50, "no-such-connection", 50); got != 20 {
		t.Errorf("a request above the CSP limit should be capped to 20, got %d", got)
	}
}

func TestMaxConcurrentConnections(t *testing.T) {
	if got := maxConcurrentConnections(0); got != 1 {
		t.Errorf("no connections should still yield one worker, got %d", got)
	}
	if got := maxConcurrentConnections(3); got != 3 {
		t.Errorf("got %d, want 3", got)
	}
	if got := maxConcurrentConnections(99); got != csp.GlobalMaxConcurrentConnections {
		t.Errorf("got %d, want the global cap %d", got, csp.GlobalMaxConcurrentConnections)
	}
}

// A creation that fails after names were reserved still leaves Node records
// behind, so the ids must reach the caller — otherwise a retry cannot clean up
// its own failed attempt and the NodeGroup accumulates dead records.
func TestScaleOutReportsIdsOnFailurePaths(t *testing.T) {
	src, err := os.ReadFile("provisioning.go")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(src), "\n")

	reserve := -1
	for i, l := range lines {
		if strings.Contains(l, "newNodeIds, err := reserveNodeNames(") {
			reserve = i
			break
		}
	}
	if reserve < 0 {
		t.Fatal("cannot find the reserveNodeNames call")
	}
	end := reserve
	for i := reserve; i < len(lines); i++ {
		if lines[i] == "}" {
			end = i
			break
		}
	}

	// Skip the reservation's own error return: nothing was reserved there.
	for i := reserve + 4; i < end; i++ {
		l := strings.TrimSpace(lines[i])
		if strings.HasPrefix(l, "return ") && strings.HasSuffix(l, ", nil, err") {
			t.Errorf("provisioning.go:%d discards the reserved node ids: %s", i+1, l)
		}
	}
}
