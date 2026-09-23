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

	armcompute "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"

	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
)

func init() {
	csp.RegisterBatchVMStatusHandler(csptypes.Azure, BatchDescribeInstanceStatuses)
}

// azureArmIDParts holds the components parsed from an Azure ARM resource ID.
// Format: /subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Compute/virtualMachines/{name}
type azureArmIDParts struct {
	subscriptionID string
	resourceGroup  string
	vmName         string
}

// parseAzureArmID parses an Azure ARM resource ID into its components.
func parseAzureArmID(armID string) (azureArmIDParts, error) {
	// Normalize slashes and split.
	parts := strings.Split(strings.TrimPrefix(armID, "/"), "/")
	// Expected: subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Compute/virtualMachines/{name}
	// Index:    0             1    2              3   4         5                   6               7
	if len(parts) < 8 {
		return azureArmIDParts{}, fmt.Errorf("invalid Azure ARM resource ID (too few segments): %q", armID)
	}
	if !strings.EqualFold(parts[0], "subscriptions") ||
		!strings.EqualFold(parts[2], "resourceGroups") {
		return azureArmIDParts{}, fmt.Errorf("invalid Azure ARM resource ID (unexpected segments): %q", armID)
	}
	return azureArmIDParts{
		subscriptionID: parts[1],
		resourceGroup:  parts[3],
		vmName:         parts[len(parts)-1],
	}, nil
}

// BatchDescribeInstanceStatuses returns a map of the requested VM identifiers → TB status.
//
// It queries each VM's InstanceView directly from the regional Compute Resource Provider
// (bounded by azureControlConcurrency), returning real-time authoritative power state
// without relying on Azure ARM's subscription-wide cached ListAll snapshot.
func BatchDescribeInstanceStatuses(ctx context.Context, region string, instanceIds []string) (map[string]string, error) {
	if len(instanceIds) == 0 {
		return map[string]string{}, nil
	}

	creds, err := getCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("Azure vmstatus: cannot get credentials: %w", err)
	}

	vmClient, err := newVMClient(creds)
	if err != nil {
		return nil, fmt.Errorf("Azure vmstatus: failed to get VM client: %w", err)
	}

	type statusResult struct {
		id     string
		status string
		err    error
	}

	ch := make(chan statusResult, len(instanceIds))
	sem := make(chan struct{}, azureControlConcurrency)

	var wg sync.WaitGroup
	for _, instID := range instanceIds {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			rg := region
			vmName := id
			if parts, perr := parseAzureArmID(id); perr == nil {
				rg = parts.resourceGroup
				vmName = parts.vmName
			}

			resp, ierr := vmClient.InstanceView(ctx, rg, vmName, nil)
			if ierr != nil {
				ch <- statusResult{id: id, err: ierr}
				return
			}
			st := azureInstanceViewToTBStatus(&resp.VirtualMachineInstanceView)
			ch <- statusResult{id: id, status: st}
		}(instID)
	}
	wg.Wait()
	close(ch)

	result := make(map[string]string, len(instanceIds))
	for r := range ch {
		if r.err == nil && r.status != "" && r.status != model.StatusUndefined {
			result[r.id] = r.status
		}
	}

	log.Debug().
		Str("region", region).
		Int("queried", len(instanceIds)).
		Int("found", len(result)).
		Msg("[Azure] BatchDescribeInstanceStatuses completed")

	return result, nil
}

// azureInstanceViewToTBStatus extracts the VM state from an Azure VirtualMachineInstanceView
// and maps it to a TB status string.
func azureInstanceViewToTBStatus(iv *armcompute.VirtualMachineInstanceView) string {
	if iv == nil {
		return model.StatusUndefined
	}
	for _, status := range iv.Statuses {
		if status.Code == nil {
			continue
		}
		code := strings.ToLower(*status.Code)
		if strings.EqualFold(code, "provisioningstate/deleting") {
			return model.StatusTerminating
		}
		if strings.EqualFold(code, "provisioningstate/creating") {
			return model.StatusCreating
		}
	}
	for _, status := range iv.Statuses {
		if status.Code == nil {
			continue
		}
		code := strings.ToLower(*status.Code)
		if !strings.HasPrefix(code, "powerstate/") {
			continue
		}
		powerState := strings.TrimPrefix(code, "powerstate/")
		switch powerState {
		case "starting":
			return model.StatusResuming
		case "running":
			return model.StatusRunning
		case "stopping", "deallocating":
			return model.StatusSuspending
		case "stopped", "deallocated":
			// Spider's action=suspend calls Azure Stop (not Deallocate), so the VM
			// reaches PowerState "stopped" as its final resting state — never "deallocated".
			// Map both to Suspended to match Spider's own status reporting and AWS parity.
			return model.StatusSuspended
		default:
			return model.StatusUndefined
		}
	}
	return model.StatusUndefined
}

// azurePowerStateToTBStatus delegates to azureInstanceViewToTBStatus.
func azurePowerStateToTBStatus(props *armcompute.VirtualMachineProperties) string {
	if props == nil {
		return model.StatusUndefined
	}
	return azureInstanceViewToTBStatus(props.InstanceView)
}

