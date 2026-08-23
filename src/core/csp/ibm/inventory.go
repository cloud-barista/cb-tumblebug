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

// Package ibm provides direct IBM Cloud VPC SDK calls (truth surface: inventory, terminate, residuals).
package ibm

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
)

func init() {
	csp.RegisterInventoryHandlers(csptypes.IBM, csp.InventoryHandlers{
		ListVMs:         ListVMs,
		ListResiduals:   ListResiduals,
		DeleteResiduals: DeleteResiduals,
	})
	csp.RegisterRemediationTerminateHandler(csptypes.IBM, BatchTerminateInstances)
	csp.RegisterBatchVMStatusHandler(csptypes.IBM, BatchDescribeInstanceStatuses)
}

const (
	pageLimit   = int64(100)
	concurrency = 5
)

var serviceCache sync.Map

func getIBMApiKey(ctx context.Context) (string, error) {
	path := csp.BuildSecretPath(ctx, csptypes.IBM)
	data, err := csp.ReadOpenBaoSecret(ctx, path)
	if err != nil {
		return "", err
	}
	key := csp.GetString(data, "IC_API_KEY")
	if key == "" {
		return "", fmt.Errorf("IBM credentials incomplete at %s (need IC_API_KEY)", path)
	}
	return key, nil
}

// newVPCService returns a regional VPC service client (cached per key+region).
func newVPCService(ctx context.Context, region string) (*vpcv1.VpcV1, error) {
	apiKey, err := getIBMApiKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("IBM: cannot get credentials: %w", err)
	}
	credKey := csp.CredKey(apiKey)
	if v, ok := csp.LoadClient(&serviceCache, region, credKey); ok {
		return v.(*vpcv1.VpcV1), nil
	}
	svc, err := vpcv1.NewVpcV1(&vpcv1.VpcV1Options{
		Authenticator: &core.IamAuthenticator{ApiKey: apiKey},
		URL:           fmt.Sprintf("https://%s.iaas.cloud.ibm.com/v1", region),
	})
	if err != nil {
		return nil, fmt.Errorf("IBM: failed to create VPC client (region=%s): %w", region, err)
	}
	return csp.StoreClient(&serviceCache, region, credKey, svc).(*vpcv1.VpcV1), nil
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// nextStart extracts the "start" token from a collection's next href.
func nextStart(next *vpcv1.PageLink) string {
	if next == nil || next.Href == nil {
		return ""
	}
	u, err := url.Parse(*next.Href)
	if err != nil {
		return ""
	}
	return u.Query().Get("start")
}

// ibmStateToTBStatus mirrors cb-spider's IBM convertInstanceStatus mapping.
func ibmStateToTBStatus(state string) string {
	switch strings.ToLower(state) {
	case "running":
		return model.StatusRunning
	case "stopped", "paused":
		return model.StatusSuspended
	case "pausing", "pending", "stopping":
		return model.StatusSuspending
	case "starting", "resuming":
		return model.StatusResuming
	case "restarting":
		return model.StatusRebooting
	case "deleting":
		return model.StatusTerminating
	case "failed":
		return model.StatusFailed
	default:
		return model.StatusUndefined
	}
}

func listInstances(ctx context.Context, region string) ([]vpcv1.Instance, error) {
	svc, err := newVPCService(ctx, region)
	if err != nil {
		return nil, err
	}
	var out []vpcv1.Instance
	start := ""
	for {
		opts := &vpcv1.ListInstancesOptions{Limit: core.Int64Ptr(pageLimit)}
		if start != "" {
			opts.Start = core.StringPtr(start)
		}
		coll, _, err := svc.ListInstancesWithContext(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("IBM ListInstances failed (region=%s): %w", region, err)
		}
		out = append(out, coll.Instances...)
		start = nextStart(coll.Next)
		if start == "" {
			break
		}
	}
	return out, nil
}

// ListVMs lists every instance in region directly from IBM VPC.
func ListVMs(ctx context.Context, region, _ string) ([]csp.VMRecord, error) {
	insts, err := listInstances(ctx, region)
	if err != nil {
		return nil, err
	}
	out := make([]csp.VMRecord, 0, len(insts))
	for _, inst := range insts {
		rec := csp.VMRecord{CspResourceId: str(inst.ID), Name: str(inst.Name), Status: ibmStateToTBStatus(str(inst.Status))}
		if inst.Zone != nil {
			rec.Zone = str(inst.Zone.Name)
		}
		out = append(out, rec)
	}
	return out, nil
}

// BatchDescribeInstanceStatuses returns TB statuses for the given instance IDs (missing = not found).
func BatchDescribeInstanceStatuses(ctx context.Context, region string, instanceIds []string) (map[string]string, error) {
	result := make(map[string]string, len(instanceIds))
	if len(instanceIds) == 0 {
		return result, nil
	}
	insts, err := listInstances(ctx, region)
	if err != nil {
		return nil, err
	}
	want := make(map[string]struct{}, len(instanceIds))
	for _, id := range instanceIds {
		want[id] = struct{}{}
	}
	for _, inst := range insts {
		if _, ok := want[str(inst.ID)]; ok {
			result[str(inst.ID)] = ibmStateToTBStatus(str(inst.Status))
		}
	}
	return result, nil
}

// BatchTerminateInstances deletes the given instances (their floating IPs are handled as residuals).
func BatchTerminateInstances(ctx context.Context, region string, instanceIds []string) (map[string]string, error) {
	result := make(map[string]string, len(instanceIds))
	if len(instanceIds) == 0 {
		return result, nil
	}
	svc, err := newVPCService(ctx, region)
	if err != nil {
		return nil, err
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	for _, id := range instanceIds {
		wg.Add(1)
		sem <- struct{}{}
		go func(id string) {
			defer wg.Done()
			defer func() { <-sem }()
			if _, derr := svc.DeleteInstanceWithContext(ctx, &vpcv1.DeleteInstanceOptions{ID: core.StringPtr(id)}); derr != nil {
				log.Warn().Err(derr).Msgf("[IBM] DeleteInstance failed for %s", id)
				return
			}
			mu.Lock()
			result[id] = model.StatusTerminating
			mu.Unlock()
		}(id)
	}
	wg.Wait()
	return result, nil
}

// ListResiduals lists TB-managed floating IPs that are not bound to any target.
func ListResiduals(ctx context.Context, region, _ string) ([]csp.ResidualResource, error) {
	svc, err := newVPCService(ctx, region)
	if err != nil {
		return nil, err
	}
	var out []csp.ResidualResource
	start := ""
	for {
		opts := &vpcv1.ListFloatingIpsOptions{Limit: core.Int64Ptr(pageLimit)}
		if start != "" {
			opts.Start = core.StringPtr(start)
		}
		coll, _, err := svc.ListFloatingIpsWithContext(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("IBM ListFloatingIps failed (region=%s): %w", region, err)
		}
		for _, fip := range coll.FloatingIps {
			if fip.Target != nil || !csp.IsManagedByTB(str(fip.Name), nil) {
				continue
			}
			out = append(out, csp.ResidualResource{Type: "floatingIp", Id: str(fip.ID), Name: str(fip.Name), Detail: str(fip.Address)})
		}
		start = nextStart(coll.Next)
		if start == "" {
			break
		}
	}
	return out, nil
}

// DeleteResiduals releases the given floating IPs.
func DeleteResiduals(ctx context.Context, region, _ string, items []csp.ResidualResource) map[string]error {
	result := make(map[string]error, len(items))
	svc, err := newVPCService(ctx, region)
	if err != nil {
		for _, it := range items {
			result[it.Key()] = err
		}
		return result
	}
	for _, it := range items {
		if it.Type != "floatingIp" {
			result[it.Key()] = fmt.Errorf("unsupported residual type %q", it.Type)
			continue
		}
		_, derr := svc.DeleteFloatingIPWithContext(ctx, &vpcv1.DeleteFloatingIPOptions{ID: core.StringPtr(it.Id)})
		result[it.Key()] = derr
	}
	return result
}
