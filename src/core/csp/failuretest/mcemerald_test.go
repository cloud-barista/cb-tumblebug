package failuretest

// Messages captured from infra "mc-emerald", where 689 nodes across 8 CSPs
// produced 9 failures due to account quotas (Azure, IBM, Tencent) and transient
// allocation failure (NHN).
//
// These tests verify that provider-specific failure parsers accurately extract
// error codes, HTTP statuses, and assign appropriate failure classes and retry hints.

import (
	"strings"
	"testing"

	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/alibaba"
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/aws"
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/azure"
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/gcp"
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/ibm"
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/ncp"
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/nhn"
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/tencent"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
)

const (
	// Azure standardBSFamily core quota reached (409 Conflict)
	azureCoreQuota = "Failed to Start VM. err = PUT https://management.azure.com/subscriptions/a20fed83-96bd-4480-92a9-140b8e3b7c3a/resourceGroups/koreacentral/providers/Microsoft.Compute/virtualMachines/tbjd8i7lviks62p56b6i -------------------------------------------------------------------------------- RESPONSE 409: 409 Conflict ERROR CODE: OperationNotAllowed -------------------------------------------------------------------------------- { error: { code: OperationNotAllowed, message: Operation could not be completed as it results in exceeding approved standardBSFamily Cores quota. Additional details - Deployment Model: Resource Manager, Location: KoreaCentral, Current Limit: 50, Current Usage: 50, Additional Required: 1, (Minimum) New Limit Required: 51. Setup Alerts when Quota reaches threshold. Learn more at https://aka.ms/quotamonitoringalerting . Submit a request for Quota increase at https://aka.ms/ProdportalCRP/#blade/Microsoft_Azure_Capacity/UsageAndQuota.ReactView/Parameters/%7B%22subscriptionId%22:%22a20fed83-96bd-4480-92a9-140b8e3b7c3a%22,%22command%22:%22openQuotaApprovalBlade%22,%22quotas%22:[%7B%22location%22:%22KoreaCentral%22,%22providerId%22:%22Microsoft.Compute%22,%22resourceName%22:%22standardBSFamily%22,%22quotaRequest%22:%7B%22properties%22:%7B%22limit%22:51,%22unit%22:%22Count%22,%22name%22:%7B%22value%22:%22standardBSFamily%22%7D%7D%7D%7D]%7D by specifying parameters listed in the ‘Details’ section for deployment to succeed. Please read more about quota limits at https://docs.microsoft.com/en-us/azure/azure-supportability/per-vm-quota-requests } } -------------------------------------------------------------------------------- (from cb-spider:1024/spider/vm (500 Internal Server Error))"

	// Azure SKU capacity shortage
	azureSkuNotAvailable = "RESPONSE 400: 400 Bad Request ERROR CODE: SkuNotAvailable -------------------------------------------------------------------------------- { error: { code: SkuNotAvailable, message: The requested VM size Standard_D4s_v3 is currently not available in location eastus2 for subscription. } }"

	// IBM Cloud VPC Floating IP quota limit reached (40/40)
	ibmFloatingIpQuota = "Failed to Create VM. err = Creating a new floating IP address will put the user over quota. Allocated: 40, Requested: 1, Quota: 40 (from cb-spider:1024/spider/vm (500 Internal Server Error))"

	// NHN Cloud Public IP 500 Internal Server Error
	nhnPublicIp500 = "Failed to Start VM. Failed to Associate PublicIP : Failed to Create Public IP!! : [Internal Server Error] (from cb-spider:1024/spider/vm (500 Internal Server Error))"

	// NHN Cloud Public IP exhausted
	nhnPublicIpExhausted = "Failed to Start VM. Failed to Associate PublicIP : Failed to Create Public IP!! : [no available public ip] (from cb-spider:1024/spider/vm (500 Internal Server Error))"

	// Tencent Cloud Instance Quota reached
	tencentInstanceQuota = "[TencentCloudSDKError] Code=LimitExceeded.InstanceQuota, Message=Your current quota does not allow you to specified the value `1` in the parameter `InstanceCount`., RequestId=0811f29e-de5c-425c-b09a-15f51ea5bab1 (from cb-spider:1024/spider/vm (500 Internal Server Error))"
)

func TestAzureFailureParser(t *testing.T) {
	t.Run("CoreQuotaExceeded", func(t *testing.T) {
		f := csp.ClassifyProvisioningFailure("azure", "koreacentral", "1", azureCoreQuota)

		if f.Class != model.FailureAccountQuota {
			t.Errorf("class = %q, want %q", f.Class, model.FailureAccountQuota)
		}
		if f.Retryable {
			t.Error("account quota should not be retryable")
		}
		if f.RetryHint != model.RetryHintNotRetryable {
			t.Errorf("retryHint = %q, want %q", f.RetryHint, model.RetryHintNotRetryable)
		}
		if f.CspErrorCode != "OperationNotAllowed" {
			t.Errorf("code = %q, want %q", f.CspErrorCode, "OperationNotAllowed")
		}
		if f.HttpStatus != 409 {
			t.Errorf("httpStatus = %d, want 409", f.HttpStatus)
		}
		if !strings.Contains(f.Message, "exceeding approved standardBSFamily Cores quota") {
			t.Errorf("message = %q, expected quota explanation", f.Message)
		}
	})

	t.Run("SkuNotAvailable", func(t *testing.T) {
		f := csp.ClassifyProvisioningFailure("azure", "eastus2", "2", azureSkuNotAvailable)

		if f.Class != model.FailureZoneCapacity {
			t.Errorf("class = %q, want %q", f.Class, model.FailureZoneCapacity)
		}
		if !f.Retryable {
			t.Error("zone capacity shortage should be retryable in another zone")
		}
		if f.RetryHint != model.RetryHintDifferentZone {
			t.Errorf("retryHint = %q, want %q", f.RetryHint, model.RetryHintDifferentZone)
		}
		if f.CspErrorCode != "SkuNotAvailable" {
			t.Errorf("code = %q, want %q", f.CspErrorCode, "SkuNotAvailable")
		}
		if f.HttpStatus != 400 {
			t.Errorf("httpStatus = %d, want 400", f.HttpStatus)
		}
	})
}

func TestIBMFailureParser(t *testing.T) {
	t.Run("FloatingIpQuotaExceeded", func(t *testing.T) {
		f := csp.ClassifyProvisioningFailure("ibm", "jp-osa", "jp-osa-1", ibmFloatingIpQuota)

		if f.Class != model.FailureAccountQuota {
			t.Errorf("class = %q, want %q", f.Class, model.FailureAccountQuota)
		}
		if f.Retryable {
			t.Error("account quota should not be retryable")
		}
		if f.RetryHint != model.RetryHintNotRetryable {
			t.Errorf("retryHint = %q, want %q", f.RetryHint, model.RetryHintNotRetryable)
		}
		if f.Message != "Creating a new floating IP address will put the user over quota. Allocated: 40, Requested: 1, Quota: 40" {
			t.Errorf("message = %q", f.Message)
		}
	})
}

func TestNHNPublicIpFailures(t *testing.T) {
	t.Run("Transient500Error", func(t *testing.T) {
		f := csp.ClassifyProvisioningFailure("nhn", "kr1", "kr-pub-a", nhnPublicIp500)

		// Transient 500 error should not shut down the circuit breaker
		if f.Class != model.FailureUnknown {
			t.Errorf("class = %q, want %q", f.Class, model.FailureUnknown)
		}
		if !f.Retryable {
			t.Error("transient 500 error should be retryable")
		}
		if f.RetryHint != model.RetryHintSameConfig {
			t.Errorf("retryHint = %q, want %q", f.RetryHint, model.RetryHintSameConfig)
		}
		if f.HttpStatus != 500 {
			t.Errorf("httpStatus = %d, want 500", f.HttpStatus)
		}
	})

	t.Run("ExhaustedCapacity", func(t *testing.T) {
		f := csp.ClassifyProvisioningFailure("nhn", "kr1", "kr-pub-a", nhnPublicIpExhausted)

		// IP pool exhaustion is a region-wide capacity issue
		if f.Class != model.FailureRegionCapacity {
			t.Errorf("class = %q, want %q", f.Class, model.FailureRegionCapacity)
		}
		if f.Retryable {
			t.Error("exhausted capacity should not be retryable in same region")
		}
		if f.RetryHint != model.RetryHintDifferentRegion {
			t.Errorf("retryHint = %q, want %q", f.RetryHint, model.RetryHintDifferentRegion)
		}
	})
}

func TestTencentInstanceQuota(t *testing.T) {
	f := csp.ClassifyProvisioningFailure("tencent", "ap-seoul", "ap-seoul-1", tencentInstanceQuota)

	if f.Class != model.FailureAccountQuota {
		t.Errorf("class = %q, want %q", f.Class, model.FailureAccountQuota)
	}
	if f.Retryable {
		t.Error("account quota should not be retryable")
	}
	if f.CspErrorCode != "LimitExceeded.InstanceQuota" {
		t.Errorf("code = %q", f.CspErrorCode)
	}
	if f.RequestId != "0811f29e-de5c-425c-b09a-15f51ea5bab1" {
		t.Errorf("requestId = %q", f.RequestId)
	}
}
