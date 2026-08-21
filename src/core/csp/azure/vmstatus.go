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
// It issues a single subscription-wide VirtualMachines ListAll with statusOnly=true, which
// returns every VM's runtime InstanceView (power state) in one paged call, and matches each
// requested identifier against that snapshot. Identifiers may be a full ARM resource ID or a
// bare VM name — TB register records sometimes hold only the name — and both resolve because
// the snapshot is keyed by ARM ID and by name.
//
// A requested VM absent from the snapshot is omitted from the result: the clean "not found"
// signal callers treat as gone (the AWS handler follows the same contract). Using ListAll
// rather than a per-VM Get also lets a bare name resolve without knowing its resource group.
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

	statusOnly := "true"
	pager := vmClient.NewListAllPager(&armcompute.VirtualMachinesClientListAllOptions{StatusOnly: &statusOnly})
	byKey := make(map[string]string)
	for pager.More() {
		page, perr := pager.NextPage(ctx)
		if perr != nil {
			return nil, fmt.Errorf("Azure vmstatus: ListAll failed (region=%s): %w", region, perr)
		}
		for _, vm := range page.Value {
			if vm == nil {
				continue
			}
			status := azurePowerStateToTBStatus(vm.Properties)
			if vm.ID != nil {
				byKey[strings.ToLower(*vm.ID)] = status
			}
			if vm.Name != nil {
				byKey[strings.ToLower(*vm.Name)] = status
			}
		}
	}

	result := make(map[string]string, len(instanceIds))
	for _, id := range instanceIds {
		keys := []string{strings.ToLower(id)}
		if parts, perr := parseAzureArmID(id); perr == nil {
			keys = append(keys, strings.ToLower(parts.vmName))
		}
		for _, k := range keys {
			if s, ok := byKey[k]; ok {
				result[id] = s
				break
			}
		}
	}

	log.Trace().
		Str("region", region).
		Int("queried", len(instanceIds)).
		Int("found", len(result)).
		Msg("[Azure] BatchDescribeInstanceStatuses completed")

	return result, nil
}

// azurePowerStateToTBStatus extracts the VM state from an Azure VM instance view
// and maps it to a TB status string.
//
// Azure reports two relevant status categories in InstanceView.Statuses:
//   - PowerState/xxx  — actual power state of the VM
//   - ProvisioningState/xxx — ARM-level provisioning state (including "deleting")
//
// ProvisioningState/deleting is checked first because a VM being deleted may still
// report a stale PowerState (e.g. stopped) that would otherwise be misread as Suspended.
func azurePowerStateToTBStatus(props *armcompute.VirtualMachineProperties) string {
	if props == nil || props.InstanceView == nil {
		return model.StatusUndefined
	}
	for _, status := range props.InstanceView.Statuses {
		if status.Code == nil {
			continue
		}
		code := strings.ToLower(*status.Code)
		if strings.EqualFold(code, "provisioningstate/deleting") {
			return model.StatusTerminating
		}
	}
	for _, status := range props.InstanceView.Statuses {
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
			return model.StatusCreating
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
