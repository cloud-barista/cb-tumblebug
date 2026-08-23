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

// Package nhn provides direct NHN Cloud (OpenStack) SDK calls (truth surface).
package nhn

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	oscommon "github.com/cloud-barista/cb-tumblebug/src/core/csp/openstackcommon"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	nhnsdk "github.com/cloud-barista/nhncloud-sdk-go"
	nhnostack "github.com/cloud-barista/nhncloud-sdk-go/openstack"
	"github.com/cloud-barista/nhncloud-sdk-go/openstack/compute/v2/extensions/floatingips"
	"github.com/cloud-barista/nhncloud-sdk-go/openstack/compute/v2/servers"
	"github.com/rs/zerolog/log"
)

func init() {
	csp.RegisterInventoryHandlers(csptypes.NHN, csp.InventoryHandlers{
		ListVMs:         ListVMs,
		ListResiduals:   ListResiduals,
		DeleteResiduals: DeleteResiduals,
	})
	csp.RegisterRemediationTerminateHandler(csptypes.NHN, BatchTerminateInstances)
	csp.RegisterBatchVMStatusHandler(csptypes.NHN, BatchDescribeInstanceStatuses)
}

const concurrency = 5

type creds struct {
	IdentityEndpoint, Username, Password, DomainName, TenantID string
}

func getCreds(ctx context.Context) (*creds, error) {
	path := csp.BuildSecretPath(ctx, csptypes.NHN)
	data, err := csp.ReadOpenBaoSecret(ctx, path)
	if err != nil {
		return nil, err
	}
	c := &creds{
		IdentityEndpoint: csp.GetString(data, "NHN_IDENTITY_ENDPOINT"),
		Username:         csp.GetString(data, "NHN_USERNAME"),
		Password:         csp.GetString(data, "NHN_PASSWORD"),
		DomainName:       csp.GetString(data, "NHN_DOMAIN_NAME"),
		TenantID:         csp.GetString(data, "NHN_TENANT_ID"),
	}
	if c.IdentityEndpoint == "" || c.Username == "" || c.Password == "" || c.TenantID == "" {
		return nil, fmt.Errorf("NHN credentials incomplete at %s", path)
	}
	return c, nil
}

// newComputeClient authenticates and returns a Nova client for region (e.g. "KR1").
func newComputeClient(ctx context.Context, region string) (*nhnsdk.ServiceClient, error) {
	c, err := getCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("NHN: cannot get credentials: %w", err)
	}
	provider, err := nhnostack.AuthenticatedClient(nhnsdk.AuthOptions{
		IdentityEndpoint: c.IdentityEndpoint, Username: c.Username, Password: c.Password,
		DomainName: c.DomainName, TenantID: c.TenantID,
	})
	if err != nil {
		return nil, fmt.Errorf("NHN: authentication failed: %w", err)
	}
	client, err := nhnostack.NewComputeV2(provider, nhnsdk.EndpointOpts{Region: region})
	if err != nil && region != strings.ToUpper(region) {
		client, err = nhnostack.NewComputeV2(provider, nhnsdk.EndpointOpts{Region: strings.ToUpper(region)})
	}
	if err != nil {
		return nil, fmt.Errorf("NHN: compute endpoint not found (region=%s): %w", region, err)
	}
	return client, nil
}

func listServers(ctx context.Context, region string) ([]servers.Server, error) {
	client, err := newComputeClient(ctx, region)
	if err != nil {
		return nil, err
	}
	pages, err := servers.List(client, servers.ListOpts{}).AllPages()
	if err != nil {
		return nil, fmt.Errorf("NHN servers.List failed (region=%s): %w", region, err)
	}
	return servers.ExtractServers(pages)
}

// ListVMs lists every server in region directly from NHN Cloud.
func ListVMs(ctx context.Context, region, _ string) ([]csp.VMRecord, error) {
	list, err := listServers(ctx, region)
	if err != nil {
		return nil, err
	}
	out := make([]csp.VMRecord, 0, len(list))
	for _, s := range list {
		out = append(out, csp.VMRecord{CspResourceId: s.ID, Name: s.Name, Status: oscommon.StateToTBStatus(s.Status),
			PublicIP: oscommon.PublicIPOf(s.Addresses), Tags: s.Metadata})
	}
	return out, nil
}

// BatchDescribeInstanceStatuses returns TB statuses for the given server IDs (missing = not found).
func BatchDescribeInstanceStatuses(ctx context.Context, region string, instanceIds []string) (map[string]string, error) {
	result := make(map[string]string, len(instanceIds))
	if len(instanceIds) == 0 {
		return result, nil
	}
	list, err := listServers(ctx, region)
	if err != nil {
		return nil, err
	}
	want := make(map[string]struct{}, len(instanceIds))
	for _, id := range instanceIds {
		want[id] = struct{}{}
	}
	for _, s := range list {
		if _, ok := want[s.ID]; ok {
			result[s.ID] = oscommon.StateToTBStatus(s.Status)
		}
	}
	return result, nil
}

// BatchTerminateInstances deletes the given servers (floating IPs are handled as residuals).
func BatchTerminateInstances(ctx context.Context, region string, instanceIds []string) (map[string]string, error) {
	result := make(map[string]string, len(instanceIds))
	if len(instanceIds) == 0 {
		return result, nil
	}
	client, err := newComputeClient(ctx, region)
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
			if derr := servers.Delete(client, id).ExtractErr(); derr != nil {
				log.Warn().Err(derr).Msgf("[NHN] servers.Delete failed for %s", id)
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

// ListResiduals lists floating IPs not associated with any server.
func ListResiduals(ctx context.Context, region, _ string) ([]csp.ResidualResource, error) {
	client, err := newComputeClient(ctx, region)
	if err != nil {
		return nil, err
	}
	pages, err := floatingips.List(client).AllPages()
	if err != nil {
		return nil, fmt.Errorf("NHN floatingips.List failed (region=%s): %w", region, err)
	}
	fips, err := floatingips.ExtractFloatingIPs(pages)
	if err != nil {
		return nil, err
	}
	var out []csp.ResidualResource
	for _, f := range fips {
		if f.InstanceID != "" {
			continue
		}
		// Floating IPs carry no name/tag; an unassociated one in a TB-managed tenant is a residual.
		out = append(out, csp.ResidualResource{Type: "floatingIp", Id: f.ID, Detail: f.IP})
	}
	return out, nil
}

// DeleteResiduals releases the given floating IPs.
func DeleteResiduals(ctx context.Context, region, _ string, items []csp.ResidualResource) map[string]error {
	result := make(map[string]error, len(items))
	client, err := newComputeClient(ctx, region)
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
		result[it.Key()] = floatingips.Delete(client, it.Id).ExtractErr()
	}
	return result
}
