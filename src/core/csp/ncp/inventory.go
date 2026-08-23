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

// Package ncp provides direct Naver Cloud Platform (VPC) SDK calls (truth surface).
package ncp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/NaverCloudPlatform/ncloud-sdk-go-v2/ncloud"
	"github.com/NaverCloudPlatform/ncloud-sdk-go-v2/services/vserver"
	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
)

func init() {
	csp.RegisterInventoryHandlers(csptypes.NCP, csp.InventoryHandlers{
		ListVMs:         ListVMs,
		ListResiduals:   ListResiduals,
		DeleteResiduals: DeleteResiduals,
	})
	csp.RegisterRemediationTerminateHandler(csptypes.NCP, BatchTerminateInstances)
	csp.RegisterBatchVMStatusHandler(csptypes.NCP, BatchDescribeInstanceStatuses)
}

const (
	pageSize        = int32(100)
	stopWaitTimeout = 3 * time.Minute
	stopPollEvery   = 10 * time.Second
)

func newClient(ctx context.Context) (*vserver.APIClient, error) {
	path := csp.BuildSecretPath(ctx, csptypes.NCP)
	data, err := csp.ReadOpenBaoSecret(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("NCP: cannot get credentials: %w", err)
	}
	ak, sk := csp.GetString(data, "NCLOUD_ACCESS_KEY"), csp.GetString(data, "NCLOUD_SECRET_KEY")
	if ak == "" || sk == "" {
		return nil, fmt.Errorf("NCP credentials incomplete at %s", path)
	}
	return vserver.NewAPIClient(vserver.NewConfiguration(&ncloud.APIKey{AccessKey: ak, SecretKey: sk})), nil
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ncpStateToTBStatus mirrors cb-spider's NCP mapping, which keys on ServerInstanceStatusName
// ("running", "stopped", ...); the status code is only a fallback.
func ncpStateToTBStatus(name, code string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "creating", "init", "booting", "setting up":
		return model.StatusCreating
	case "running":
		return model.StatusRunning
	case "shutting down":
		return model.StatusSuspending
	case "stopped":
		return model.StatusSuspended
	case "rebooting", "hard rebooting":
		return model.StatusRebooting
	case "hard shutting down", "terminating":
		return model.StatusTerminating
	}
	switch strings.ToUpper(code) {
	case "RUN":
		return model.StatusRunning
	case "NSTOP":
		return model.StatusSuspended
	case "INIT", "CREAT":
		return model.StatusCreating
	}
	return model.StatusUndefined
}

func listInstances(ctx context.Context, region string, ids []string) ([]*vserver.ServerInstance, error) {
	region = strings.ToUpper(region)
	client, err := newClient(ctx)
	if err != nil {
		return nil, err
	}
	var out []*vserver.ServerInstance
	for page := int32(1); ; page++ {
		req := &vserver.GetServerInstanceListRequest{RegionCode: ncloud.String(region), PageNo: ncloud.Int32(page), PageSize: ncloud.Int32(pageSize)}
		if len(ids) > 0 {
			req.ServerInstanceNoList = ncloud.StringList(ids)
		}
		resp, err := client.V2Api.GetServerInstanceList(req)
		if err != nil {
			return nil, fmt.Errorf("NCP GetServerInstanceList failed (region=%s, page=%d): %w", region, page, err)
		}
		out = append(out, resp.ServerInstanceList...)
		total := int32(0)
		if resp.TotalRows != nil {
			total = *resp.TotalRows
		}
		if page*pageSize >= total || len(resp.ServerInstanceList) == 0 {
			break
		}
	}
	return out, nil
}

// ListVMs lists every server instance in region directly from NCP.
func ListVMs(ctx context.Context, region, _ string) ([]csp.VMRecord, error) {
	list, err := listInstances(ctx, region, nil)
	if err != nil {
		return nil, err
	}
	out := make([]csp.VMRecord, 0, len(list))
	for _, s := range list {
		code := ""
		if s.ServerInstanceStatus != nil {
			code = str(s.ServerInstanceStatus.Code)
		}
		out = append(out, csp.VMRecord{CspResourceId: str(s.ServerInstanceNo), Name: str(s.ServerName),
			Status: ncpStateToTBStatus(str(s.ServerInstanceStatusName), code), Zone: str(s.ZoneCode), PublicIP: str(s.PublicIp)})
	}
	return out, nil
}

// BatchDescribeInstanceStatuses returns TB statuses for the given instance numbers (missing = not found).
func BatchDescribeInstanceStatuses(ctx context.Context, region string, instanceIds []string) (map[string]string, error) {
	result := make(map[string]string, len(instanceIds))
	if len(instanceIds) == 0 {
		return result, nil
	}
	list, err := listInstances(ctx, region, instanceIds)
	if err != nil {
		return nil, err
	}
	for _, s := range list {
		code := ""
		if s.ServerInstanceStatus != nil {
			code = str(s.ServerInstanceStatus.Code)
		}
		result[str(s.ServerInstanceNo)] = ncpStateToTBStatus(str(s.ServerInstanceStatusName), code)
	}
	return result, nil
}

// BatchTerminateInstances stops running instances, waits for NSTOP, then terminates them.
func BatchTerminateInstances(ctx context.Context, region string, instanceIds []string) (map[string]string, error) {
	region = strings.ToUpper(region)
	result := make(map[string]string, len(instanceIds))
	if len(instanceIds) == 0 {
		return result, nil
	}
	client, err := newClient(ctx)
	if err != nil {
		return nil, err
	}
	statuses, err := BatchDescribeInstanceStatuses(ctx, region, instanceIds)
	if err != nil {
		return nil, err
	}
	var toStop, present []string
	for _, id := range instanceIds {
		st, ok := statuses[id]
		if !ok {
			continue
		}
		present = append(present, id)
		if st != model.StatusSuspended {
			toStop = append(toStop, id)
		}
	}
	if len(toStop) > 0 {
		if _, err := client.V2Api.StopServerInstances(&vserver.StopServerInstancesRequest{
			RegionCode: ncloud.String(region), ServerInstanceNoList: ncloud.StringList(toStop)}); err != nil {
			log.Warn().Err(err).Msg("[NCP] StopServerInstances failed; continuing to terminate what is stopped")
		}
		deadline := time.Now().Add(stopWaitTimeout)
		for time.Now().Before(deadline) {
			time.Sleep(stopPollEvery)
			cur, err := BatchDescribeInstanceStatuses(ctx, region, toStop)
			if err != nil {
				continue
			}
			pending := 0
			for _, id := range toStop {
				if cur[id] != model.StatusSuspended {
					pending++
				}
			}
			if pending == 0 {
				break
			}
		}
	}
	if len(present) == 0 {
		return result, nil
	}
	if _, err := client.V2Api.TerminateServerInstances(&vserver.TerminateServerInstancesRequest{
		RegionCode: ncloud.String(region), ServerInstanceNoList: ncloud.StringList(present)}); err != nil {
		return result, fmt.Errorf("NCP TerminateServerInstances failed (region=%s): %w", region, err)
	}
	for _, id := range present {
		result[id] = model.StatusTerminating
	}
	return result, nil
}

// ListResiduals lists public IPs not associated with any server instance.
func ListResiduals(ctx context.Context, region, _ string) ([]csp.ResidualResource, error) {
	region = strings.ToUpper(region)
	client, err := newClient(ctx)
	if err != nil {
		return nil, err
	}
	var out []csp.ResidualResource
	for page := int32(1); ; page++ {
		resp, err := client.V2Api.GetPublicIpInstanceList(&vserver.GetPublicIpInstanceListRequest{
			RegionCode: ncloud.String(region), IsAssociated: ncloud.Bool(false),
			PageNo: ncloud.Int32(page), PageSize: ncloud.Int32(pageSize)})
		if err != nil {
			return nil, fmt.Errorf("NCP GetPublicIpInstanceList failed (region=%s): %w", region, err)
		}
		for _, p := range resp.PublicIpInstanceList {
			if p == nil || p.PublicIpInstanceNo == nil || str(p.ServerInstanceNo) != "" {
				continue
			}
			out = append(out, csp.ResidualResource{Type: "publicIp", Id: *p.PublicIpInstanceNo, Detail: str(p.PublicIp)})
		}
		total := int32(0)
		if resp.TotalRows != nil {
			total = *resp.TotalRows
		}
		if page*pageSize >= total || len(resp.PublicIpInstanceList) == 0 {
			break
		}
	}
	return out, nil
}

// DeleteResiduals releases the given public IPs.
func DeleteResiduals(ctx context.Context, region, _ string, items []csp.ResidualResource) map[string]error {
	region = strings.ToUpper(region)
	result := make(map[string]error, len(items))
	client, err := newClient(ctx)
	if err != nil {
		for _, it := range items {
			result[it.Key()] = err
		}
		return result
	}
	for _, it := range items {
		if it.Type != "publicIp" {
			result[it.Key()] = fmt.Errorf("unsupported residual type %q", it.Type)
			continue
		}
		_, derr := client.V2Api.DeletePublicIpInstance(&vserver.DeletePublicIpInstanceRequest{
			RegionCode: ncloud.String(region), PublicIpInstanceNo: ncloud.String(it.Id)})
		result[it.Key()] = derr
	}
	return result
}
