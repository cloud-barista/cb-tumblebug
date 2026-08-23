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

package tencent

import (
	"context"
	"fmt"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	tccommon "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	cvm "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cvm/v20170312"
	tcvpc "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/vpc/v20170312"
)

func init() {
	csp.RegisterInventoryHandlers(csptypes.Tencent, csp.InventoryHandlers{
		ListVMs:         ListVMs,
		ListResiduals:   ListResiduals,
		DeleteResiduals: DeleteResiduals,
	})
	// Remediation only: normal node control keeps using CB-Spider; direct terminate is for audit cleanup.
	csp.RegisterRemediationTerminateHandler(csptypes.Tencent, BatchTerminateInstances)
}

// requestInterval keeps the call rate under Tencent's 10 req/s per-API limit.
const requestInterval = 150 * time.Millisecond

func newVPCClient(region, secretID, secretKey string) (*tcvpc.Client, error) {
	credential := tccommon.NewCredential(secretID, secretKey)
	cpf := profile.NewClientProfile()
	cpf.NetworkFailureMaxRetries = 2
	cpf.NetworkFailureRetryDuration = profile.ExponentialBackoff
	return tcvpc.NewClient(credential, region, cpf)
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ListVMs lists every instance in region via DescribeInstances (offset paging by 100).
func ListVMs(ctx context.Context, region, _ string) ([]csp.VMRecord, error) {
	id, key, err := getTencentCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("Tencent inventory: cannot get credentials: %w", err)
	}
	client, err := newCVMClient(region, id, key)
	if err != nil {
		return nil, err
	}
	var out []csp.VMRecord
	for offset := int64(0); ; offset += tencentBatchSize {
		req := cvm.NewDescribeInstancesRequest()
		req.Offset = tccommon.Int64Ptr(offset)
		req.Limit = tccommon.Int64Ptr(tencentBatchSize)
		resp, err := client.DescribeInstances(req)
		if err != nil {
			return nil, fmt.Errorf("Tencent DescribeInstances failed (region=%s, offset=%d): %w", region, offset, err)
		}
		for _, inst := range resp.Response.InstanceSet {
			if inst == nil || inst.InstanceId == nil {
				continue
			}
			tags := make(map[string]string, len(inst.Tags))
			for _, t := range inst.Tags {
				if t != nil && t.Key != nil && t.Value != nil {
					tags[*t.Key] = *t.Value
				}
			}
			rec := csp.VMRecord{CspResourceId: *inst.InstanceId, Name: str(inst.InstanceName),
				Status: tencentStateToTBStatus(str(inst.InstanceState)), Tags: tags}
			if inst.Placement != nil {
				rec.Zone = str(inst.Placement.Zone)
			}
			if len(inst.PublicIpAddresses) > 0 {
				rec.PublicIP = str(inst.PublicIpAddresses[0])
			}
			out = append(out, rec)
		}
		total := int64(0)
		if resp.Response.TotalCount != nil {
			total = *resp.Response.TotalCount
		}
		if offset+tencentBatchSize >= total || len(resp.Response.InstanceSet) == 0 {
			break
		}
		time.Sleep(requestInterval)
	}
	return out, nil
}

// BatchTerminateInstances terminates instances in batches of 100.
func BatchTerminateInstances(ctx context.Context, region string, instanceIds []string) (map[string]string, error) {
	result := make(map[string]string, len(instanceIds))
	if len(instanceIds) == 0 {
		return result, nil
	}
	id, key, err := getTencentCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("Tencent terminate: cannot get credentials: %w", err)
	}
	client, err := newCVMClient(region, id, key)
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(instanceIds); i += tencentBatchSize {
		batch := instanceIds[i:min(i+tencentBatchSize, len(instanceIds))]
		req := cvm.NewTerminateInstancesRequest()
		req.InstanceIds = tccommon.StringPtrs(batch)
		if _, err := client.TerminateInstances(req); err != nil {
			return result, fmt.Errorf("Tencent TerminateInstances failed (region=%s): %w", region, err)
		}
		for _, b := range batch {
			result[b] = model.StatusTerminating
		}
		time.Sleep(requestInterval)
	}
	return result, nil
}

// ListResiduals lists TB-managed unattached secondary ENIs and unbound EIPs in region.
func ListResiduals(ctx context.Context, region, _ string) ([]csp.ResidualResource, error) {
	id, key, err := getTencentCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("Tencent residuals: cannot get credentials: %w", err)
	}
	client, err := newVPCClient(region, id, key)
	if err != nil {
		return nil, err
	}
	var out []csp.ResidualResource
	for offset := uint64(0); ; offset += tencentBatchSize {
		req := tcvpc.NewDescribeNetworkInterfacesRequest()
		req.Offset = tccommon.Uint64Ptr(offset)
		req.Limit = tccommon.Uint64Ptr(tencentBatchSize)
		resp, err := client.DescribeNetworkInterfaces(req)
		if err != nil {
			return nil, fmt.Errorf("Tencent DescribeNetworkInterfaces failed: %w", err)
		}
		for _, n := range resp.Response.NetworkInterfaceSet {
			if n == nil || n.NetworkInterfaceId == nil || n.Attachment != nil {
				continue
			}
			if n.Primary != nil && *n.Primary {
				continue
			}
			tags := map[string]string{}
			for _, t := range n.TagSet {
				if t != nil && t.Key != nil && t.Value != nil {
					tags[*t.Key] = *t.Value
				}
			}
			if !csp.IsManagedByTB(str(n.NetworkInterfaceName), tags) {
				continue
			}
			out = append(out, csp.ResidualResource{Type: "eni", Id: *n.NetworkInterfaceId, Name: str(n.NetworkInterfaceName)})
		}
		total := uint64(0)
		if resp.Response.TotalCount != nil {
			total = *resp.Response.TotalCount
		}
		if offset+tencentBatchSize >= total || len(resp.Response.NetworkInterfaceSet) == 0 {
			break
		}
		time.Sleep(requestInterval)
	}
	for offset := int64(0); ; offset += tencentBatchSize {
		req := tcvpc.NewDescribeAddressesRequest()
		req.Offset = tccommon.Int64Ptr(offset)
		req.Limit = tccommon.Int64Ptr(tencentBatchSize)
		resp, err := client.DescribeAddresses(req)
		if err != nil {
			return nil, fmt.Errorf("Tencent DescribeAddresses failed: %w", err)
		}
		for _, a := range resp.Response.AddressSet {
			if a == nil || a.AddressId == nil || str(a.AddressStatus) != "UNBIND" {
				continue
			}
			if !csp.IsManagedByTB(str(a.AddressName), nil) {
				continue
			}
			out = append(out, csp.ResidualResource{Type: "eip", Id: *a.AddressId, Name: str(a.AddressName), Detail: str(a.AddressIp)})
		}
		total := int64(0)
		if resp.Response.TotalCount != nil {
			total = *resp.Response.TotalCount
		}
		if offset+tencentBatchSize >= total || len(resp.Response.AddressSet) == 0 {
			break
		}
		time.Sleep(requestInterval)
	}
	return out, nil
}

// DeleteResiduals deletes the given ENIs and releases EIPs.
func DeleteResiduals(ctx context.Context, region, _ string, items []csp.ResidualResource) map[string]error {
	result := make(map[string]error, len(items))
	id, key, err := getTencentCreds(ctx)
	if err != nil {
		for _, it := range items {
			result[it.Key()] = err
		}
		return result
	}
	client, err := newVPCClient(region, id, key)
	if err != nil {
		for _, it := range items {
			result[it.Key()] = err
		}
		return result
	}
	for _, it := range items {
		var derr error
		switch it.Type {
		case "eni":
			req := tcvpc.NewDeleteNetworkInterfaceRequest()
			req.NetworkInterfaceId = tccommon.StringPtr(it.Id)
			_, derr = client.DeleteNetworkInterface(req)
		case "eip":
			req := tcvpc.NewReleaseAddressesRequest()
			req.AddressIds = tccommon.StringPtrs([]string{it.Id})
			_, derr = client.ReleaseAddresses(req)
		default:
			derr = fmt.Errorf("unsupported residual type %q", it.Type)
		}
		result[it.Key()] = derr
		time.Sleep(requestInterval)
	}
	return result
}
