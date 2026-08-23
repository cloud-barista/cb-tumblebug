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

package alibaba

import (
	"context"
	"fmt"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/vpc"
	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
)

func init() {
	csp.RegisterInventoryHandlers(csptypes.Alibaba, csp.InventoryHandlers{
		ListVMs:         ListVMs,
		ListResiduals:   ListResiduals,
		DeleteResiduals: DeleteResiduals,
	})
	// Remediation only: normal node control keeps using CB-Spider; direct terminate is for audit cleanup.
	csp.RegisterRemediationTerminateHandler(csptypes.Alibaba, BatchTerminateInstances)
}

// pageSize is the max page size accepted by Alibaba Describe* APIs (the default is only 10).
const pageSize = 100

// ListVMs lists every instance in region via DescribeInstances with explicit paging.
func ListVMs(ctx context.Context, region, _ string) ([]csp.VMRecord, error) {
	ak, sk, err := getAlibabaCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("Alibaba inventory: cannot get credentials: %w", err)
	}
	client, err := newECSClient(region, ak, sk)
	if err != nil {
		return nil, err
	}
	var out []csp.VMRecord
	for page := 1; ; page++ {
		req := ecs.CreateDescribeInstancesRequest()
		req.RegionId = region
		req.PageSize = requests.NewInteger(pageSize)
		req.PageNumber = requests.NewInteger(page)
		resp, err := client.DescribeInstances(req)
		if err != nil {
			return nil, fmt.Errorf("Alibaba DescribeInstances failed (region=%s, page=%d): %s", region, page, csp.RedactErr(err))
		}
		for _, inst := range resp.Instances.Instance {
			tags := make(map[string]string, len(inst.Tags.Tag))
			for _, t := range inst.Tags.Tag {
				tags[t.TagKey] = t.TagValue
			}
			rec := csp.VMRecord{CspResourceId: inst.InstanceId, Name: inst.InstanceName,
				Status: alibabaStateToTBStatus(inst.Status), Zone: inst.ZoneId, Tags: tags}
			if len(inst.PublicIpAddress.IpAddress) > 0 {
				rec.PublicIP = inst.PublicIpAddress.IpAddress[0]
			} else if inst.EipAddress.IpAddress != "" {
				rec.PublicIP = inst.EipAddress.IpAddress
			}
			out = append(out, rec)
		}
		if page*pageSize >= resp.TotalCount || len(resp.Instances.Instance) == 0 {
			break
		}
	}
	return out, nil
}

// BatchTerminateInstances force-deletes instances in batches of 100.
func BatchTerminateInstances(ctx context.Context, region string, instanceIds []string) (map[string]string, error) {
	result := make(map[string]string, len(instanceIds))
	if len(instanceIds) == 0 {
		return result, nil
	}
	ak, sk, err := getAlibabaCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("Alibaba terminate: cannot get credentials: %w", err)
	}
	client, err := newECSClient(region, ak, sk)
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(instanceIds); i += pageSize {
		batch := instanceIds[i:min(i+pageSize, len(instanceIds))]
		req := ecs.CreateDeleteInstancesRequest()
		req.RegionId = region
		req.InstanceId = &batch
		req.Force = requests.NewBoolean(true)
		if _, err := client.DeleteInstances(req); err != nil {
			return result, fmt.Errorf("Alibaba DeleteInstances failed (region=%s): %s", region, csp.RedactErr(err))
		}
		for _, id := range batch {
			result[id] = model.StatusTerminating
		}
	}
	return result, nil
}

// ListResiduals lists TB-managed available ENIs, disks, and EIPs in region.
func ListResiduals(ctx context.Context, region, _ string) ([]csp.ResidualResource, error) {
	ak, sk, err := getAlibabaCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("Alibaba residuals: cannot get credentials: %w", err)
	}
	client, err := newECSClient(region, ak, sk)
	if err != nil {
		return nil, err
	}
	var out []csp.ResidualResource

	for page := 1; ; page++ {
		req := ecs.CreateDescribeNetworkInterfacesRequest()
		req.RegionId = region
		req.Status = "Available"
		req.PageSize = requests.NewInteger(pageSize)
		req.PageNumber = requests.NewInteger(page)
		resp, err := client.DescribeNetworkInterfaces(req)
		if err != nil {
			return nil, fmt.Errorf("Alibaba DescribeNetworkInterfaces failed: %s", csp.RedactErr(err))
		}
		for _, n := range resp.NetworkInterfaceSets.NetworkInterfaceSet {
			tags := map[string]string{}
			for _, t := range n.Tags.Tag {
				tags[t.TagKey] = t.TagValue
			}
			if !csp.IsManagedByTB(n.NetworkInterfaceName, tags) {
				continue
			}
			out = append(out, csp.ResidualResource{Type: "eni", Id: n.NetworkInterfaceId, Name: n.NetworkInterfaceName})
		}
		if page*pageSize >= resp.TotalCount || len(resp.NetworkInterfaceSets.NetworkInterfaceSet) == 0 {
			break
		}
	}

	for page := 1; ; page++ {
		req := ecs.CreateDescribeDisksRequest()
		req.RegionId = region
		req.Status = "Available"
		req.PageSize = requests.NewInteger(pageSize)
		req.PageNumber = requests.NewInteger(page)
		resp, err := client.DescribeDisks(req)
		if err != nil {
			return nil, fmt.Errorf("Alibaba DescribeDisks failed: %s", csp.RedactErr(err))
		}
		for _, d := range resp.Disks.Disk {
			tags := map[string]string{}
			for _, t := range d.Tags.Tag {
				tags[t.TagKey] = t.TagValue
			}
			if !csp.IsManagedByTB(d.DiskName, tags) {
				continue
			}
			out = append(out, csp.ResidualResource{Type: "disk", Id: d.DiskId, Name: d.DiskName, Zone: d.ZoneId})
		}
		if page*pageSize >= resp.TotalCount || len(resp.Disks.Disk) == 0 {
			break
		}
	}

	vpcClient, err := vpc.NewClientWithAccessKey(region, ak, sk)
	if err != nil {
		return nil, err
	}
	for page := 1; ; page++ {
		req := vpc.CreateDescribeEipAddressesRequest()
		req.RegionId = region
		req.Status = "Available"
		req.PageSize = requests.NewInteger(pageSize)
		req.PageNumber = requests.NewInteger(page)
		resp, err := vpcClient.DescribeEipAddresses(req)
		if err != nil {
			return nil, fmt.Errorf("Alibaba DescribeEipAddresses failed: %s", csp.RedactErr(err))
		}
		for _, e := range resp.EipAddresses.EipAddress {
			tags := map[string]string{}
			for _, t := range e.Tags.Tag {
				tags[t.Key] = t.Value
			}
			if !csp.IsManagedByTB(e.Name, tags) {
				continue
			}
			out = append(out, csp.ResidualResource{Type: "eip", Id: e.AllocationId, Name: e.Name, Detail: e.IpAddress})
		}
		if page*pageSize >= resp.TotalCount || len(resp.EipAddresses.EipAddress) == 0 {
			break
		}
	}
	return out, nil
}

// DeleteResiduals deletes the given ENIs, disks, and EIPs.
func DeleteResiduals(ctx context.Context, region, _ string, items []csp.ResidualResource) map[string]error {
	result := make(map[string]error, len(items))
	fail := func(err error) map[string]error {
		for _, it := range items {
			result[it.Key()] = err
		}
		return result
	}
	ak, sk, err := getAlibabaCreds(ctx)
	if err != nil {
		return fail(err)
	}
	client, err := newECSClient(region, ak, sk)
	if err != nil {
		return fail(err)
	}
	vpcClient, err := vpc.NewClientWithAccessKey(region, ak, sk)
	if err != nil {
		return fail(err)
	}
	for _, it := range items {
		var derr error
		switch it.Type {
		case "eni":
			req := ecs.CreateDeleteNetworkInterfaceRequest()
			req.RegionId = region
			req.NetworkInterfaceId = it.Id
			_, derr = client.DeleteNetworkInterface(req)
		case "disk":
			req := ecs.CreateDeleteDiskRequest()
			req.DiskId = it.Id
			_, derr = client.DeleteDisk(req)
		case "eip":
			req := vpc.CreateReleaseEipAddressRequest()
			req.RegionId = region
			req.AllocationId = it.Id
			_, derr = vpcClient.ReleaseEipAddress(req)
		default:
			derr = fmt.Errorf("unsupported residual type %q", it.Type)
		}
		if derr != nil {
			derr = fmt.Errorf("%s", csp.RedactErr(derr))
		}
		result[it.Key()] = derr
	}
	return result
}
