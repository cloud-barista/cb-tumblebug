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

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
)

func init() {
	csp.RegisterInventoryHandlers(csptypes.AWS, csp.InventoryHandlers{
		ListVMs:         ListVMs,
		ListResiduals:   ListResiduals,
		DeleteResiduals: DeleteResiduals,
	})
}

func tagMap(tags []ec2types.Tag) map[string]string {
	m := make(map[string]string, len(tags))
	for _, t := range tags {
		if t.Key != nil && t.Value != nil {
			m[*t.Key] = *t.Value
		}
	}
	return m
}

// ListVMs lists all non-terminated instances in the region directly from EC2.
func ListVMs(ctx context.Context, region, _ string) ([]csp.VMRecord, error) {
	client, err := newEC2Client(ctx, region)
	if err != nil {
		return nil, err
	}
	var out []csp.VMRecord
	var token *string
	for {
		resp, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
			MaxResults: aws.Int32(1000),
			NextToken:  token,
			Filters: []ec2types.Filter{{Name: aws.String("instance-state-name"),
				Values: []string{"pending", "running", "stopping", "stopped", "shutting-down"}}},
		})
		if err != nil {
			return nil, fmt.Errorf("AWS DescribeInstances failed (region=%s): %w", region, err)
		}
		for _, r := range resp.Reservations {
			for _, i := range r.Instances {
				if i.InstanceId == nil || i.State == nil {
					continue
				}
				tags := tagMap(i.Tags)
				rec := csp.VMRecord{CspResourceId: *i.InstanceId, Name: tags["Name"],
					Status: awsStateToTBStatus(string(i.State.Name)), Tags: tags}
				if i.Placement != nil && i.Placement.AvailabilityZone != nil {
					rec.Zone = *i.Placement.AvailabilityZone
				}
				if i.PublicIpAddress != nil {
					rec.PublicIP = *i.PublicIpAddress
				}
				out = append(out, rec)
			}
		}
		if resp.NextToken == nil || *resp.NextToken == "" {
			break
		}
		token = resp.NextToken
	}
	return out, nil
}

// ListResiduals lists TB-managed ENIs, Elastic IPs, and EBS volumes that are not attached.
func ListResiduals(ctx context.Context, region, _ string) ([]csp.ResidualResource, error) {
	client, err := newEC2Client(ctx, region)
	if err != nil {
		return nil, err
	}
	var out []csp.ResidualResource

	var token *string
	for {
		resp, err := client.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
			NextToken: token, MaxResults: aws.Int32(1000),
			Filters: []ec2types.Filter{{Name: aws.String("status"), Values: []string{"available"}}},
		})
		if err != nil {
			return nil, fmt.Errorf("AWS DescribeNetworkInterfaces failed (region=%s): %w", region, err)
		}
		for _, n := range resp.NetworkInterfaces {
			if n.NetworkInterfaceId == nil {
				continue
			}
			tags := tagMap(n.TagSet)
			name := tags["Name"]
			if !csp.IsManagedByTB(name, tags) && !csp.IsTBUid(aws.ToString(n.Description)) {
				continue
			}
			out = append(out, csp.ResidualResource{Type: "eni", Id: *n.NetworkInterfaceId, Name: name})
		}
		if resp.NextToken == nil || *resp.NextToken == "" {
			break
		}
		token = resp.NextToken
	}

	addrs, err := client.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{})
	if err != nil {
		return nil, fmt.Errorf("AWS DescribeAddresses failed (region=%s): %w", region, err)
	}
	for _, a := range addrs.Addresses {
		if a.AllocationId == nil || a.AssociationId != nil {
			continue
		}
		tags := tagMap(a.Tags)
		if !csp.IsManagedByTB(tags["Name"], tags) {
			continue
		}
		out = append(out, csp.ResidualResource{Type: "eip", Id: *a.AllocationId, Name: tags["Name"], Detail: aws.ToString(a.PublicIp)})
	}

	token = nil
	for {
		resp, err := client.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{
			NextToken: token, MaxResults: aws.Int32(500),
			Filters: []ec2types.Filter{{Name: aws.String("status"), Values: []string{"available"}}},
		})
		if err != nil {
			return nil, fmt.Errorf("AWS DescribeVolumes failed (region=%s): %w", region, err)
		}
		for _, v := range resp.Volumes {
			if v.VolumeId == nil {
				continue
			}
			tags := tagMap(v.Tags)
			if !csp.IsManagedByTB(tags["Name"], tags) {
				continue
			}
			out = append(out, csp.ResidualResource{Type: "volume", Id: *v.VolumeId, Name: tags["Name"]})
		}
		if resp.NextToken == nil || *resp.NextToken == "" {
			break
		}
		token = resp.NextToken
	}
	return out, nil
}

// DeleteResiduals deletes the given ENIs, Elastic IPs, and volumes.
func DeleteResiduals(ctx context.Context, region, _ string, items []csp.ResidualResource) map[string]error {
	result := make(map[string]error, len(items))
	client, err := newEC2Client(ctx, region)
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
			_, derr = client.DeleteNetworkInterface(ctx, &ec2.DeleteNetworkInterfaceInput{NetworkInterfaceId: aws.String(it.Id)})
		case "eip":
			_, derr = client.ReleaseAddress(ctx, &ec2.ReleaseAddressInput{AllocationId: aws.String(it.Id)})
		case "volume":
			_, derr = client.DeleteVolume(ctx, &ec2.DeleteVolumeInput{VolumeId: aws.String(it.Id)})
		default:
			derr = fmt.Errorf("unsupported residual type %q", it.Type)
		}
		result[it.Key()] = derr
	}
	return result
}
