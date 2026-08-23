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

package gcp

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
	compute "google.golang.org/api/compute/v1"
)

func init() {
	csp.RegisterInventoryHandlers(csptypes.GCP, csp.InventoryHandlers{
		ListVMs:         ListVMs,
		ListResiduals:   ListResiduals,
		DeleteResiduals: DeleteResiduals,
	})
	// Remediation only: normal node control keeps using CB-Spider; direct terminate is for audit cleanup.
	csp.RegisterRemediationTerminateHandler(csptypes.GCP, BatchTerminateInstances)
}

const gcpConcurrency = 10

// listInstancesInRegion returns every instance whose zone belongs to region, plus the project ID.
func listInstancesInRegion(ctx context.Context, region string) ([]*compute.Instance, string, error) {
	creds, err := getGCPCreds(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("GCP inventory: cannot get credentials: %w", err)
	}
	svc, err := newComputeService(ctx, creds)
	if err != nil {
		return nil, "", err
	}
	var out []*compute.Instance
	pageToken := ""
	for {
		call := svc.Instances.AggregatedList(creds.ProjectID).Context(ctx)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		resp, err := call.Do()
		if err != nil {
			return nil, "", fmt.Errorf("GCP AggregatedList failed (project=%s): %w", creds.ProjectID, err)
		}
		for zoneKey, items := range resp.Items {
			if !strings.HasPrefix(strings.TrimPrefix(zoneKey, "zones/"), region+"-") {
				continue
			}
			out = append(out, items.Instances...)
		}
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}
	return out, creds.ProjectID, nil
}

func zoneOf(inst *compute.Instance) string {
	return inst.Zone[strings.LastIndex(inst.Zone, "/")+1:]
}

// ListVMs lists every instance in region directly from Compute Engine.
func ListVMs(ctx context.Context, region, _ string) ([]csp.VMRecord, error) {
	insts, _, err := listInstancesInRegion(ctx, region)
	if err != nil {
		return nil, err
	}
	out := make([]csp.VMRecord, 0, len(insts))
	for _, inst := range insts {
		rec := csp.VMRecord{CspResourceId: inst.Name, Name: inst.Name, Status: gcpStateToTBStatus(inst.Status),
			Zone: zoneOf(inst), Tags: inst.Labels}
		for _, ni := range inst.NetworkInterfaces {
			for _, ac := range ni.AccessConfigs {
				if ac.NatIP != "" {
					rec.PublicIP = ac.NatIP
				}
			}
		}
		out = append(out, rec)
	}
	return out, nil
}

// BatchTerminateInstances deletes the named instances; zones are resolved from the regional inventory.
func BatchTerminateInstances(ctx context.Context, region string, instanceIds []string) (map[string]string, error) {
	result := make(map[string]string, len(instanceIds))
	if len(instanceIds) == 0 {
		return result, nil
	}
	insts, project, err := listInstancesInRegion(ctx, region)
	if err != nil {
		return nil, err
	}
	creds, err := getGCPCreds(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := newComputeService(ctx, creds)
	if err != nil {
		return nil, err
	}
	zones := make(map[string]string, len(insts))
	for _, inst := range insts {
		zones[inst.Name] = zoneOf(inst)
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, gcpConcurrency)
	for _, name := range instanceIds {
		zone, ok := zones[name]
		if !ok {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(name, zone string) {
			defer wg.Done()
			defer func() { <-sem }()
			if _, derr := svc.Instances.Delete(project, zone, name).Context(ctx).Do(); derr != nil {
				log.Warn().Err(derr).Msgf("[GCP] Instances.Delete failed for %s/%s", zone, name)
				return
			}
			mu.Lock()
			result[name] = model.StatusTerminating
			mu.Unlock()
		}(name, zone)
	}
	wg.Wait()
	return result, nil
}

// ListResiduals lists TB-managed unattached disks and unused external addresses in region.
func ListResiduals(ctx context.Context, region, _ string) ([]csp.ResidualResource, error) {
	creds, err := getGCPCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("GCP residuals: cannot get credentials: %w", err)
	}
	svc, err := newComputeService(ctx, creds)
	if err != nil {
		return nil, err
	}
	var out []csp.ResidualResource
	pageToken := ""
	for {
		call := svc.Disks.AggregatedList(creds.ProjectID).Context(ctx)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		resp, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("GCP Disks.AggregatedList failed: %w", err)
		}
		for zoneKey, items := range resp.Items {
			zone := strings.TrimPrefix(zoneKey, "zones/")
			if !strings.HasPrefix(zone, region+"-") {
				continue
			}
			for _, d := range items.Disks {
				if len(d.Users) > 0 || !csp.IsManagedByTB(d.Name, d.Labels) {
					continue
				}
				out = append(out, csp.ResidualResource{Type: "disk", Id: d.Name, Name: d.Name, Zone: zone})
			}
		}
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}
	addrs, err := svc.Addresses.List(creds.ProjectID, region).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("GCP Addresses.List failed (region=%s): %w", region, err)
	}
	for _, a := range addrs.Items {
		if a.Status == "IN_USE" || !csp.IsManagedByTB(a.Name, a.Labels) {
			continue
		}
		out = append(out, csp.ResidualResource{Type: "publicIp", Id: a.Name, Name: a.Name, Detail: a.Address})
	}
	return out, nil
}

// DeleteResiduals deletes the given disks and addresses.
func DeleteResiduals(ctx context.Context, region, _ string, items []csp.ResidualResource) map[string]error {
	result := make(map[string]error, len(items))
	fail := func(err error) map[string]error {
		for _, it := range items {
			result[it.Key()] = err
		}
		return result
	}
	creds, err := getGCPCreds(ctx)
	if err != nil {
		return fail(err)
	}
	svc, err := newComputeService(ctx, creds)
	if err != nil {
		return fail(err)
	}
	for _, it := range items {
		var derr error
		switch it.Type {
		case "disk":
			_, derr = svc.Disks.Delete(creds.ProjectID, it.Zone, it.Id).Context(ctx).Do()
		case "publicIp":
			_, derr = svc.Addresses.Delete(creds.ProjectID, region, it.Id).Context(ctx).Do()
		default:
			derr = fmt.Errorf("unsupported residual type %q", it.Type)
		}
		result[it.Key()] = derr
	}
	return result
}
