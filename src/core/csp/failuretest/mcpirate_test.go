package failuretest

// Messages captured from infra "mc-pirate", where 10 GPU nodes across 8 CSPs
// produced 5 failures with 5 different causes. Only one of the five is worth
// retrying; the classifier has to keep the other four out of the retry path.

import (
	"strings"
	"testing"

	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/alibaba"
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/aws"
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/gcp"
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/ncp"
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/nhn"
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/tencent"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
)

func TestMcPirateFailures(t *testing.T) {
	tests := []struct {
		node          string
		provider      string
		region        string
		attemptedZone string
		raw           string
		wantClass     string
		wantRetryable bool
		wantHint      string
	}{
		{
			node: "nvidial40s-1", provider: "aws", region: "us-west-2", attemptedZone: "us-west-2d",
			raw:       "InsufficientInstanceCapacity: We currently do not have sufficient g6e.2xlarge capacity in the Availability Zone you requested (us-west-2d). Our system will be working on provisioning additional capacity. You can currently get g6e.2xlarge capacity by not specifying an Availability Zone in your request or choosing us-west-2a, us-west-2b, us-west-2c. status code: 500, request id: 9ad764b4-abd5-436d-bdd8-bb40b5a09a02 (from cb-spider:1024/spider/vm (500 Internal Server Error))",
			wantClass: model.FailureZoneCapacity, wantRetryable: true, wantHint: model.RetryHintDifferentZone,
		},
		{
			// The spec accepts only certain images; a different image is needed,
			// not another attempt.
			node: "g4-1", provider: "alibaba", region: "us-east-1", attemptedZone: "us-east-1b",
			raw:       "SDK.ServerError ErrorCode: InvalidParameter.NotMatch Recommend: https://api.aliyun.com/troubleshoot?q=InvalidParameter.NotMatch&product=Ecs&requestId=01A0553F-46CF-86A6-B881-2683774FABB0 RequestId: 01A0553F-46CF-86A6-B881-2683774FABB0 Message: The specified instance type ecs.gn6i-c4g1.xlarge only supports some specific images. You can use the DescribeImages API to query the available images.",
			wantClass: model.FailureImageSpecMismatch, wantRetryable: false, wantHint: model.RetryHintDifferentImage,
		},
		{
			// "Creation Limit:0" — the account is not entitled to this GPU type.
			node: "g9-1", provider: "ncp", region: "kr", attemptedZone: "KR-1",
			raw:       "Failed to Create VM instance : [Status: 400 Bad Request, Body: { responseError: { returnCode: 1153027, returnMessage: Server (VPC) product generation limit exceeded. Product Type: GPU - T4 - G1 Creation Limit:0 / Usage:0 / Creation Request:1 } }] (from cb-spider:1024/spider/vm (500 Internal Server Error))",
			wantClass: model.FailureAccountQuota, wantRetryable: false, wantHint: model.RetryHintNotRetryable,
		},
		{
			// Root disk too small for the flavor: the request needs correcting.
			node: "g10-1", provider: "nhn", region: "kr1", attemptedZone: "kr-pub-b",
			raw:       "Failed to Create a VM with the Block Storage Volume!! [Bad request with: [POST https://kr1-api-instance-infrastructure.nhncloudservice.com/v2/a77e25da7cc04a388716a7dc10dc9340/servers], error message: {badRequest:{message:Volume size is too small.,code:400}}] (from cb-spider:1024/spider/vm (500 Internal Server Error))",
			wantClass: model.FailureInvalidRequest, wantRetryable: false, wantHint: model.RetryHintAdjustRequest,
		},
		{
			node: "g11-1", provider: "tencent", region: "ap-seoul", attemptedZone: "ap-seoul-2",
			raw:       "[TencentCloudSDKError] Code=InvalidImageId.NotFound, Message=ImageId img-7rotv4ux is not found, RequestId=ac88e205-1490-4a9e-8dee-8bb754876cde (from cb-spider:1024/spider/vm (500 Internal Server Error))",
			wantClass: model.FailureImageSpecMismatch, wantRetryable: false, wantHint: model.RetryHintDifferentImage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.node, func(t *testing.T) {
			f := csp.ClassifyProvisioningFailure(tt.provider, tt.region, tt.attemptedZone, tt.raw)
			if f.Class != tt.wantClass {
				t.Errorf("class = %q, want %q", f.Class, tt.wantClass)
			}
			if f.Retryable != tt.wantRetryable {
				t.Errorf("retryable = %v, want %v", f.Retryable, tt.wantRetryable)
			}
			if f.RetryHint != tt.wantHint {
				t.Errorf("retryHint = %q, want %q", f.RetryHint, tt.wantHint)
			}
			// Every provider must get its attempted zone recorded, whether or not
			// the message names one.
			if f.AttemptedZone != tt.attemptedZone {
				t.Errorf("attemptedZone = %q, want %q", f.AttemptedZone, tt.attemptedZone)
			}
		})
	}
}

// Exactly one of the five mc-pirate failures should reach the zone-shift path.
func TestMcPirateOnlyOneIsWorthRetrying(t *testing.T) {
	raws := map[string][2]string{
		"aws":     {"us-west-2d", "InsufficientInstanceCapacity: We currently do not have sufficient g6e.2xlarge capacity in the Availability Zone you requested (us-west-2d). You can currently get g6e.2xlarge capacity by not specifying an Availability Zone in your request or choosing us-west-2a, us-west-2b, us-west-2c. status code: 500"},
		"alibaba": {"us-east-1b", "SDK.ServerError ErrorCode: InvalidParameter.NotMatch RequestId: 01A0553F Message: The specified instance type ecs.gn6i-c4g1.xlarge only supports some specific images."},
		"ncp":     {"KR-1", "Failed to Create VM instance : [Status: 400 Bad Request, Body: { responseError: { returnCode: 1153027, returnMessage: Server (VPC) product generation limit exceeded. Product Type: GPU - T4 - G1 Creation Limit:0 / Usage:0 / Creation Request:1 } }]"},
		"nhn":     {"kr-pub-b", "Failed to Create a VM with the Block Storage Volume!! [Bad request with: [POST https://kr1-api.example/servers], error message: {badRequest:{message:Volume size is too small.,code:400}}]"},
		"tencent": {"ap-seoul-2", "[TencentCloudSDKError] Code=InvalidImageId.NotFound, Message=ImageId img-7rotv4ux is not found, RequestId=ac88e205"},
	}

	var retryable []string
	for provider, v := range raws {
		if csp.ClassifyProvisioningFailure(provider, "", v[0], v[1]).Retryable {
			retryable = append(retryable, provider)
		}
	}
	if len(retryable) != 1 || retryable[0] != "aws" {
		t.Fatalf("retryable providers = %v, want [aws] only", retryable)
	}
}

// notRetryable must stay reserved for failures no request change can fix, so a
// UI can tell "nothing you can do" apart from "change the image / the disk".
func TestMcPirateHintsAreActionable(t *testing.T) {
	tests := []struct{ node, provider, raw, wantHint string }{
		{"g9-1 (ncp quota)", "ncp",
			"Failed to Create VM instance : [Status: 400 Bad Request, Body: { responseError: { returnCode: 1153027, returnMessage: Server (VPC) product generation limit exceeded. Product Type: GPU - T4 - G1 Creation Limit:0 / Usage:0 / Creation Request:1 } }]",
			model.RetryHintNotRetryable},
		{"g11-1 (tencent image)", "tencent",
			"[TencentCloudSDKError] Code=InvalidImageId.NotFound, Message=ImageId img-7rotv4ux is not found, RequestId=ac88e205",
			model.RetryHintDifferentImage},
		{"g4-1 (alibaba image)", "alibaba",
			"SDK.ServerError ErrorCode: InvalidParameter.NotMatch RequestId: 01A0553F Message: The specified instance type ecs.gn6i-c4g1.xlarge only supports some specific images.",
			model.RetryHintDifferentImage},
		{"g10-1 (nhn disk)", "nhn",
			"Failed to Create a VM with the Block Storage Volume!! [Bad request with: [POST https://kr1-api.example/servers], error message: {badRequest:{message:Volume size is too small.,code:400}}]",
			model.RetryHintAdjustRequest},
	}
	for _, tt := range tests {
		if got := csp.ClassifyProvisioningFailure(tt.provider, "", "", tt.raw).RetryHint; got != tt.wantHint {
			t.Errorf("%s: retryHint = %q, want %q", tt.node, got, tt.wantHint)
		}
	}
}

// NCP's message must be read as a server-product entitlement, not as anything
// about VPC networking: "Server (VPC)" names the platform, and a Creation Limit
// of 0 means the account was never allowed to create the product.
func TestNcpQuotaMessageDistinguishesEntitlement(t *testing.T) {
	const notEntitled = "Failed to Create VM instance : [Status: 400 Bad Request, Body: { responseError: { returnCode: 1153027, returnMessage: Server (VPC) product generation limit exceeded. Product Type: GPU - T4 - G1 Creation Limit:0 / Usage:0 / Creation Request:1 } }] (from cb-spider:1024/spider/vm (500 Internal Server Error))"

	f := csp.ClassifyProvisioningFailure("ncp", "kr", "KR-1", notEntitled)
	if f.Class != model.FailureAccountQuota || f.Retryable {
		t.Fatalf("class=%q retryable=%v", f.Class, f.Retryable)
	}
	if f.CspErrorCode != "1153027" {
		t.Errorf("cspErrorCode = %q, want 1153027", f.CspErrorCode)
	}
	if !strings.Contains(f.Message, "GPU - T4 - G1") || !strings.Contains(f.Message, "not entitled") {
		t.Errorf("message should name the product and say it is not entitled: %q", f.Message)
	}

	// The same code with a non-zero limit is an exhausted allowance instead.
	exhausted := strings.Replace(notEntitled, "Creation Limit:0 / Usage:0", "Creation Limit:5 / Usage:5", 1)
	f2 := csp.ClassifyProvisioningFailure("ncp", "kr", "KR-1", exhausted)
	if strings.Contains(f2.Message, "not entitled") {
		t.Errorf("an exhausted limit must not read as missing entitlement: %q", f2.Message)
	}
	if !strings.Contains(f2.Message, "used up") {
		t.Errorf("message = %q, want it to say the limit is used up", f2.Message)
	}
}

// Messages captured from infra "mc-lavender", where 689 nodes across 8 CSPs
// produced 204 failures. Two of them were classified wrongly at the time.
func TestMcLavenderFailures(t *testing.T) {
	tests := []struct {
		name          string
		provider      string
		raw           string
		wantClass     string
		wantRetryable bool
		wantHint      string
	}{
		{
			// AWS throttles RunInstances under a burst and answers 503. The code
			// contains "LimitExceeded" but a rate limit clears by itself, so
			// reporting it as an account quota sends the user to raise a limit
			// that is not the problem.
			"a rate limit is not an account quota", "aws",
			"RequestLimitExceeded: Request limit exceeded. Account 635484366616 has been throttled on ec2:RunInstances because it exceeded its request rate limit. status code: 503, request id: a58a400a-fa5b-4152-8f01-9e7035a57586",
			model.FailureThrottling, true, model.RetryHintWaitAndRetry,
		},
		{
			// A real allowance, with the numbers in the message.
			"a floating IP allowance is an account quota", "ibm",
			"Failed to Create VM. err = Creating a new floating IP address will put the user over quota. Allocated: 42, Requested: 1, Quota: 40",
			model.FailureAccountQuota, false, model.RetryHintNotRetryable,
		},
		{
			// The CSP returned an internal error and nothing else. Claiming the
			// region is short of addresses — and that another region is the answer
			// — reads far more into it than it says.
			"a bare public IP failure is not a region shortage", "nhn",
			"Failed to Start VM. Failed to Associate PublicIP : Failed to Create Public IP!! : [Internal Server Error]",
			model.FailureUnknown, true, model.RetryHintSameConfig,
		},
		{
			"tencent understock is a zone shortage", "tencent",
			"[TencentCloudSDKError] Code=ResourceInsufficient.SpecifiedInstanceType, Message=The specified type of instance is understocked., RequestId=26df9a6d",
			model.FailureZoneCapacity, true, model.RetryHintDifferentZone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := csp.ClassifyProvisioningFailure(tt.provider, "", "", tt.raw)
			if f.Class != tt.wantClass {
				t.Errorf("class = %q, want %q", f.Class, tt.wantClass)
			}
			if f.Retryable != tt.wantRetryable {
				t.Errorf("retryable = %v, want %v", f.Retryable, tt.wantRetryable)
			}
			if f.RetryHint != tt.wantHint {
				t.Errorf("retryHint = %q, want %q", f.RetryHint, tt.wantHint)
			}
		})
	}
}

// An exhausted shared pool of addresses is a region-wide shortage; an exhausted
// allowance is an account quota. The two need different actions from the user, so
// they must not collapse into one class.
func TestPublicIpShortageVersusQuota(t *testing.T) {
	for _, raw := range []string{
		"Failed to Create Public IP: no available public ip",
		"사용 가능한 공인 IP가 없습니다",
	} {
		if got := csp.ClassifyProvisioningFailure("nhn", "kr1", "", raw).Class; got != model.FailureRegionCapacity {
			t.Errorf("%q classified as %q, want %q", raw, got, model.FailureRegionCapacity)
		}
	}
	quota := "Creating a new floating IP address will put the user over quota. Allocated: 42, Requested: 1, Quota: 40"
	if got := csp.ClassifyProvisioningFailure("ibm", "jp-osa", "", quota).Class; got != model.FailureAccountQuota {
		t.Errorf("an address allowance is an account quota, got %q", got)
	}
}
