package failuretest

// Verifies classification against real CSP messages captured from a live
// deployment's /log/provision/ records.

import (
	"testing"

	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/alibaba"
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/aws"
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/gcp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
)

const (
	awsCapacity = "InsufficientInstanceCapacity: We currently do not have sufficient g6e.2xlarge capacity in the Availability Zone you requested (us-west-2a). Our system will be working on provisioning additional capacity. You can currently get g6e.2xlarge capacity by not specifying an Availability Zone in your request or choosing us-west-2b, us-west-2c, us-west-2d. status code: 500, request id: 2c2fa8a8-449c-4da9-a53c-414deb4b49e2 (from cb-spider:1024/spider/vm (500 Internal Server Error))"

	gcpStockout = "Operation errors: The zone 'projects/seokho-etri/zones/europe-west4-b' does not have enough resources available to fulfill the request. 'NULL:0/NULL:0/NULL:0 (state:STOCKOUT, sub-state:STOCKOUT, resource type:compute)'. (from cb-spider:1024/spider/vm (500 Internal Server Error))"

	aliNoStock = "SDK.ServerError ErrorCode: OperationDenied.NoStock Recommend: https://api.alibabacloud.com/troubleshoot?intl_lang=EN_US&q=OperationDenied.NoStock&product=Ecs&requestId=01A046FB-ED6B-504B-870C-544AB63F497B RequestId: 01A046FB-ED6B-504B-870C-544AB63F497B Message: The resource is out of stock in the specified zone. Please try other types, or choose other regions and zones. RespHeaders: map[Access-Control-Allow-Origin:[*] Access-Control-Expose-Headers:[*] Content-Length:[276]]"
)

func TestAWSCapacity(t *testing.T) {
	f := csp.ClassifyProvisioningFailure("aws", "us-west-2", "us-west-2a", awsCapacity)

	if f.Class != model.FailureZoneCapacity {
		t.Errorf("class = %q, want %q", f.Class, model.FailureZoneCapacity)
	}
	if !f.Retryable || f.RetryHint != model.RetryHintDifferentZone {
		t.Errorf("retryable=%v hint=%q", f.Retryable, f.RetryHint)
	}
	if f.CspErrorCode != "InsufficientInstanceCapacity" {
		t.Errorf("code = %q", f.CspErrorCode)
	}
	if f.AttemptedZone != "us-west-2a" || f.ReportedZone != "us-west-2a" {
		t.Errorf("attempted=%q reported=%q", f.AttemptedZone, f.ReportedZone)
	}
	want := []string{"us-west-2b", "us-west-2c", "us-west-2d"}
	if len(f.SuggestedZones) != len(want) {
		t.Fatalf("suggestedZones = %v, want %v", f.SuggestedZones, want)
	}
	for i := range want {
		if f.SuggestedZones[i] != want[i] {
			t.Errorf("suggestedZones[%d] = %q, want %q", i, f.SuggestedZones[i], want[i])
		}
	}
	if f.HttpStatus != 500 {
		t.Errorf("httpStatus = %d", f.HttpStatus)
	}
	if f.RequestId != "2c2fa8a8-449c-4da9-a53c-414deb4b49e2" {
		t.Errorf("requestId = %q", f.RequestId)
	}
}

// The failing zone must never survive in the suggestion list, even when the
// CSP repeats it.
func TestAWSSuggestionsExcludeAttemptedZone(t *testing.T) {
	msg := "InsufficientInstanceCapacity: no capacity in the Availability Zone you requested (us-west-2b). ... choosing us-west-2a, us-west-2b, us-west-2c. status code: 500"
	f := csp.ClassifyProvisioningFailure("aws", "us-west-2", "us-west-2b", msg)
	for _, z := range f.SuggestedZones {
		if z == "us-west-2b" {
			t.Fatalf("attempted zone leaked into suggestions: %v", f.SuggestedZones)
		}
	}
}

// Account quota must not be classified as a zone shortage: another zone in the
// same account hits the same ceiling.
func TestAWSQuotaIsNotZoneCapacity(t *testing.T) {
	msg := "VcpuLimitExceeded: You have requested more vCPU capacity than your current vCPU limit of 64 allows for the instance bucket. status code: 400, request id: abc12345-0000"
	f := csp.ClassifyProvisioningFailure("aws", "us-west-2", "us-west-2a", msg)
	if f.Class != model.FailureAccountQuota {
		t.Errorf("class = %q, want %q", f.Class, model.FailureAccountQuota)
	}
	if f.Retryable {
		t.Error("account quota must not be retryable")
	}
}

func TestGCPStockout(t *testing.T) {
	f := csp.ClassifyProvisioningFailure("gcp", "europe-west4", "europe-west4-b", gcpStockout)

	if f.Class != model.FailureZoneCapacity {
		t.Errorf("class = %q, want %q", f.Class, model.FailureZoneCapacity)
	}
	if f.CspErrorCode != "STOCKOUT" {
		t.Errorf("code = %q, want STOCKOUT", f.CspErrorCode)
	}
	if f.ReportedZone != "europe-west4-b" {
		t.Errorf("reportedZone = %q", f.ReportedZone)
	}
	// GCP offers no alternatives; the retry path must fall back to the region's
	// zone list rather than expecting suggestions here.
	if len(f.SuggestedZones) != 0 {
		t.Errorf("suggestedZones = %v, want empty", f.SuggestedZones)
	}
	if f.RetryHint != model.RetryHintDifferentZone {
		t.Errorf("hint = %q", f.RetryHint)
	}
}

func TestAlibabaNoStock(t *testing.T) {
	f := csp.ClassifyProvisioningFailure("alibaba", "cn-hangzhou", "cn-hangzhou-i", aliNoStock)

	if f.Class != model.FailureZoneCapacity {
		t.Errorf("class = %q, want %q", f.Class, model.FailureZoneCapacity)
	}
	if f.CspErrorCode != "OperationDenied.NoStock" {
		t.Errorf("code = %q", f.CspErrorCode)
	}
	// Alibaba never names the zone; the attempted zone recorded by CB-Tumblebug
	// is the only zone information available.
	if f.ReportedZone != "" {
		t.Errorf("reportedZone = %q, want empty", f.ReportedZone)
	}
	if f.AttemptedZone != "cn-hangzhou-i" {
		t.Errorf("attemptedZone = %q", f.AttemptedZone)
	}
	if f.RequestId != "01A046FB-ED6B-504B-870C-544AB63F497B" {
		t.Errorf("requestId = %q", f.RequestId)
	}
	// The RespHeaders map dump must be cut from the stored text.
	if len(f.RawMessage) == 0 || contains(f.RawMessage, "RespHeaders") {
		t.Errorf("rawMessage not trimmed: %q", f.RawMessage)
	}
}

// A provider with no registered parser must still produce a usable record.
func TestUnknownProviderFallsBackToKeywords(t *testing.T) {
	f := csp.ClassifyProvisioningFailure("tencent", "ap-seoul", "ap-seoul-1",
		"[TencentCloudSDKError] Code=ResourceInsufficient.SpecifiedInstanceType, Message=the specified instance type is insufficient")
	if f.Class != model.FailureZoneCapacity {
		t.Errorf("class = %q, want %q", f.Class, model.FailureZoneCapacity)
	}
	if f.AttemptedZone != "ap-seoul-1" {
		t.Errorf("attemptedZone = %q", f.AttemptedZone)
	}
}

// An unparseable message must never be fatal and must keep its text.
func TestUnrecognizedMessageStaysUsable(t *testing.T) {
	f := csp.ClassifyProvisioningFailure("nhn", "kr1", "kr-p1", "something nobody has taught us yet")
	if f.Class != model.FailureUnknown {
		t.Errorf("class = %q, want %q", f.Class, model.FailureUnknown)
	}
	if !f.Retryable || f.RetryHint != model.RetryHintSameConfig {
		t.Errorf("unknown should allow one same-config retry: retryable=%v hint=%q", f.Retryable, f.RetryHint)
	}
	if f.RawMessage != "something nobody has taught us yet" {
		t.Errorf("rawMessage = %q", f.RawMessage)
	}
	if f.Message == "" {
		t.Error("message must not be empty")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// Every keyword the previous infra.isAvailabilityFailure matched must still
// classify as a zone shortage, and every keyword it deliberately excluded must
// still be excluded.
func TestGenericKeywordParity(t *testing.T) {
	zoneCapacity := []string{
		"The zone does not have enough resources available",
		"(state:STOCKOUT, sub-state:STOCKOUT, resource type:compute)",
		"ZONE_RESOURCE_POOL_EXHAUSTED",
		"ResourceInsufficient.SpecifiedInstanceType",
		"The resource is out of stock in the specified zone",
		"INSUFFICIENT_RESOURCES in this zone",
		"RESOURCESEXHAUSTED",
		"InsufficientInstanceCapacity",
		"we do not have sufficient capacity",
		"UnfulfillableCapacity",
		"SkuNotAvailable",
	}
	for _, msg := range zoneCapacity {
		if got := csp.ClassifyProvisioningFailure("", "", "", msg).Class; got != model.FailureZoneCapacity {
			t.Errorf("%q classified as %q, want %q", msg, got, model.FailureZoneCapacity)
		}
	}

	// Account limits were explicitly excluded from zone-retry: another zone in
	// the same account hits the same ceiling.
	notZoneCapacity := []string{
		"InstanceLimitExceeded",
		"VcpuLimitExceeded",
		"OperationDenied.QuotaExceed",
		"QuotaExceedLimit",
		"Instance quota exceeded",
	}
	for _, msg := range notZoneCapacity {
		if got := csp.ClassifyProvisioningFailure("", "", "", msg).Class; got == model.FailureZoneCapacity {
			t.Errorf("%q classified as zone capacity; account quota is region-wide", msg)
		}
	}
}

// NHN/NCP/KT public-IP exhaustion is region-wide, and its text also contains the
// generic "no available" capacity keyword.
func TestPublicIpExhaustionIsRegionWide(t *testing.T) {
	for _, msg := range []string{
		"Failed to Create Public IP: no available public ip",
		"사용 가능한 공인 IP가 없습니다",
	} {
		f := csp.ClassifyProvisioningFailure("nhn", "kr1", "kr-p1", msg)
		if f.Class != model.FailureRegionCapacity {
			t.Errorf("%q classified as %q, want %q", msg, f.Class, model.FailureRegionCapacity)
		}
	}
}
