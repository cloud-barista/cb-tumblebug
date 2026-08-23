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

package azure

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	armcompute "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	armnetwork "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
)

func init() {
	csp.RegisterInventoryHandlers(csptypes.Azure, csp.InventoryHandlers{
		ListVMs:         ListVMs,
		ListResiduals:   ListResiduals,
		DeleteResiduals: DeleteResiduals,
	})
	// Remediation only: Spider's Azure terminate also removes NIC/PIP/disk; ours relies on residual cleanup.
	csp.RegisterRemediationTerminateHandler(csptypes.Azure, BatchTerminateInstances)
}

// deleteConcurrency bounds parallel ARM delete pollers.
const deleteConcurrency = 10

// resourceGroupOf extracts the resource group from an ARM resource ID.
func resourceGroupOf(armID string) string {
	parts := strings.Split(strings.Trim(armID, "/"), "/")
	for i := 0; i+1 < len(parts); i++ {
		if strings.EqualFold(parts[i], "resourceGroups") {
			return parts[i+1]
		}
	}
	return ""
}

func strTags(tags map[string]*string) map[string]string {
	m := make(map[string]string, len(tags))
	for k, v := range tags {
		if v != nil {
			m[k] = *v
		}
	}
	return m
}

func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ListVMs lists every VM located in region directly from ARM.
func ListVMs(ctx context.Context, region, _ string) ([]csp.VMRecord, error) {
	creds, err := getCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("Azure inventory: cannot get credentials: %w", err)
	}
	vmClient, err := newVMClient(creds)
	if err != nil {
		return nil, err
	}
	statusOnly := "true"
	pager := vmClient.NewListAllPager(&armcompute.VirtualMachinesClientListAllOptions{StatusOnly: &statusOnly})
	var out []csp.VMRecord
	for pager.More() {
		page, perr := pager.NextPage(ctx)
		if perr != nil {
			return nil, fmt.Errorf("Azure ListAll VMs failed: %w", perr)
		}
		for _, vm := range page.Value {
			if vm == nil || vm.ID == nil || !strings.EqualFold(ptrStr(vm.Location), region) {
				continue
			}
			out = append(out, csp.VMRecord{CspResourceId: *vm.ID, Name: ptrStr(vm.Name),
				Status: azurePowerStateToTBStatus(vm.Properties), Tags: strTags(vm.Tags)})
		}
	}
	return out, nil
}

// BatchTerminateInstances force-deletes the given VMs (ARM IDs or bare names) and waits for completion.
// NIC/public IP/disk left behind are handled by ListResiduals/DeleteResiduals.
func BatchTerminateInstances(ctx context.Context, region string, instanceIds []string) (map[string]string, error) {
	result := make(map[string]string, len(instanceIds))
	if len(instanceIds) == 0 {
		return result, nil
	}
	creds, err := getCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("Azure terminate: cannot get credentials: %w", err)
	}
	vmClient, err := newVMClient(creds)
	if err != nil {
		return nil, err
	}
	var byName map[string]string
	resolve := func(id string) (rg, name string, ok bool) {
		if parts, perr := parseAzureArmID(id); perr == nil {
			return parts.resourceGroup, parts.vmName, true
		}
		if byName == nil {
			byName = map[string]string{}
			if vms, lerr := ListVMs(ctx, region, ""); lerr == nil {
				for _, v := range vms {
					byName[strings.ToLower(v.Name)] = v.CspResourceId
				}
			}
		}
		if armID, found := byName[strings.ToLower(id)]; found {
			if parts, perr := parseAzureArmID(armID); perr == nil {
				return parts.resourceGroup, parts.vmName, true
			}
		}
		return "", "", false
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, deleteConcurrency)
	for _, id := range instanceIds {
		rg, name, ok := resolve(id)
		if !ok {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(id, rg, name string) {
			defer wg.Done()
			defer func() { <-sem }()
			poller, perr := vmClient.BeginDelete(ctx, rg, name, &armcompute.VirtualMachinesClientBeginDeleteOptions{ForceDeletion: to.Ptr(true)})
			if perr != nil {
				log.Warn().Err(perr).Msgf("[Azure] BeginDelete failed for %s", name)
				return
			}
			if _, perr = poller.PollUntilDone(ctx, nil); perr != nil {
				log.Warn().Err(perr).Msgf("[Azure] delete polling failed for %s", name)
				return
			}
			mu.Lock()
			result[id] = model.StatusTerminated
			mu.Unlock()
		}(id, rg, name)
	}
	wg.Wait()
	return result, nil
}

// ListResiduals lists TB-managed NICs without a VM, their public IPs, and unattached disks in region.
func ListResiduals(ctx context.Context, region, _ string) ([]csp.ResidualResource, error) {
	creds, err := getCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("Azure residuals: cannot get credentials: %w", err)
	}
	credential, err := getOrCreateCredential(creds)
	if err != nil {
		return nil, err
	}
	var out []csp.ResidualResource

	nicClient, err := armnetwork.NewInterfacesClient(creds.SubscriptionID, credential, nil)
	if err != nil {
		return nil, err
	}
	orphanNicIDs := map[string]bool{}
	nicPager := nicClient.NewListAllPager(nil)
	for nicPager.More() {
		page, perr := nicPager.NextPage(ctx)
		if perr != nil {
			return nil, fmt.Errorf("Azure list NICs failed: %w", perr)
		}
		for _, n := range page.Value {
			if n == nil || n.ID == nil || !strings.EqualFold(ptrStr(n.Location), region) {
				continue
			}
			if n.Properties != nil && n.Properties.VirtualMachine != nil {
				continue
			}
			if !csp.IsManagedByTB(ptrStr(n.Name), strTags(n.Tags)) {
				continue
			}
			orphanNicIDs[strings.ToLower(*n.ID)] = true
			out = append(out, csp.ResidualResource{Type: "nic", Id: *n.ID, Name: ptrStr(n.Name)})
		}
	}

	pipClient, err := armnetwork.NewPublicIPAddressesClient(creds.SubscriptionID, credential, nil)
	if err != nil {
		return nil, err
	}
	pipPager := pipClient.NewListAllPager(nil)
	for pipPager.More() {
		page, perr := pipPager.NextPage(ctx)
		if perr != nil {
			return nil, fmt.Errorf("Azure list public IPs failed: %w", perr)
		}
		for _, p := range page.Value {
			if p == nil || p.ID == nil || !strings.EqualFold(ptrStr(p.Location), region) {
				continue
			}
			if !csp.IsManagedByTB(ptrStr(p.Name), strTags(p.Tags)) {
				continue
			}
			detail := "unattached"
			if p.Properties != nil && p.Properties.IPConfiguration != nil && p.Properties.IPConfiguration.ID != nil {
				// Attached: only a residual when bound to one of the orphan NICs above.
				cfg := strings.ToLower(*p.Properties.IPConfiguration.ID)
				nicID := cfg[:strings.Index(cfg+"/ipconfigurations/", "/ipconfigurations/")]
				if !orphanNicIDs[nicID] {
					continue
				}
				detail = "attached to orphan NIC"
			}
			out = append(out, csp.ResidualResource{Type: "publicIp", Id: *p.ID, Name: ptrStr(p.Name), Detail: detail})
		}
	}

	diskClient, err := armcompute.NewDisksClient(creds.SubscriptionID, credential, nil)
	if err != nil {
		return nil, err
	}
	diskPager := diskClient.NewListPager(nil)
	for diskPager.More() {
		page, perr := diskPager.NextPage(ctx)
		if perr != nil {
			return nil, fmt.Errorf("Azure list disks failed: %w", perr)
		}
		for _, d := range page.Value {
			if d == nil || d.ID == nil || !strings.EqualFold(ptrStr(d.Location), region) || d.ManagedBy != nil {
				continue
			}
			if !csp.IsManagedByTB(ptrStr(d.Name), strTags(d.Tags)) {
				continue
			}
			out = append(out, csp.ResidualResource{Type: "disk", Id: *d.ID, Name: ptrStr(d.Name)})
		}
	}
	return out, nil
}

// DeleteResiduals deletes NICs first (freeing their public IPs), then public IPs, then disks.
func DeleteResiduals(ctx context.Context, region, _ string, items []csp.ResidualResource) map[string]error {
	result := make(map[string]error, len(items))
	fail := func(err error) map[string]error {
		for _, it := range items {
			result[it.Key()] = err
		}
		return result
	}
	creds, err := getCreds(ctx)
	if err != nil {
		return fail(err)
	}
	credential, err := getOrCreateCredential(creds)
	if err != nil {
		return fail(err)
	}
	nicClient, err := armnetwork.NewInterfacesClient(creds.SubscriptionID, credential, nil)
	if err != nil {
		return fail(err)
	}
	pipClient, err := armnetwork.NewPublicIPAddressesClient(creds.SubscriptionID, credential, nil)
	if err != nil {
		return fail(err)
	}
	diskClient, err := armcompute.NewDisksClient(creds.SubscriptionID, credential, nil)
	if err != nil {
		return fail(err)
	}

	del := func(it csp.ResidualResource) error {
		rg := resourceGroupOf(it.Id)
		name := it.Id[strings.LastIndex(it.Id, "/")+1:]
		switch it.Type {
		case "nic":
			p, e := nicClient.BeginDelete(ctx, rg, name, nil)
			if e != nil {
				return e
			}
			_, e = p.PollUntilDone(ctx, nil)
			return e
		case "publicIp":
			p, e := pipClient.BeginDelete(ctx, rg, name, nil)
			if e != nil {
				return e
			}
			_, e = p.PollUntilDone(ctx, nil)
			return e
		case "disk":
			p, e := diskClient.BeginDelete(ctx, rg, name, nil)
			if e != nil {
				return e
			}
			_, e = p.PollUntilDone(ctx, nil)
			return e
		}
		return fmt.Errorf("unsupported residual type %q", it.Type)
	}
	for _, phase := range []string{"nic", "publicIp", "disk"} {
		var wg sync.WaitGroup
		var mu sync.Mutex
		sem := make(chan struct{}, deleteConcurrency)
		for _, it := range items {
			if it.Type != phase {
				continue
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(it csp.ResidualResource) {
				defer wg.Done()
				defer func() { <-sem }()
				e := del(it)
				mu.Lock()
				result[it.Key()] = e
				mu.Unlock()
			}(it)
		}
		wg.Wait()
	}
	return result
}
