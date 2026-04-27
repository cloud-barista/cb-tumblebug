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
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
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

// BatchDescribeInstanceStatuses queries Azure Compute for the given VM ARM resource IDs
// and returns a map of armResourceID → TB status string.
//
// VMs are grouped by resource group. For each resource group, the VMs are fetched
// in parallel using individual Get calls with InstanceView expansion (which returns
// the power state). Azure has no batch-status API in the standard SDK.
func BatchDescribeInstanceStatuses(ctx context.Context, region string, instanceIds []string) (map[string]string, error) {
	if len(instanceIds) == 0 {
		return map[string]string{}, nil
	}

	creds, err := getCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("Azure vmstatus: cannot get credentials: %w", err)
	}

	credential, err := azidentity.NewClientSecretCredential(
		creds.TenantID, creds.ClientID, creds.ClientSecret, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("Azure vmstatus: failed to create credential: %w", err)
	}

	vmClient, err := armcompute.NewVirtualMachinesClient(creds.SubscriptionID, credential, nil)
	if err != nil {
		return nil, fmt.Errorf("Azure vmstatus: failed to create VM client: %w", err)
	}

	// Fetch all VMs in parallel — Azure has no batch status API.
	type vmResult struct {
		armID  string
		status string
		err    error
	}
	ch := make(chan vmResult, len(instanceIds))

	var wg sync.WaitGroup
	for _, armID := range instanceIds {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			parts, perr := parseAzureArmID(id)
			if perr != nil {
				ch <- vmResult{armID: id, err: perr}
				return
			}
			expand := armcompute.InstanceViewTypesInstanceView
			resp, gerr := vmClient.Get(ctx, parts.resourceGroup, parts.vmName,
				&armcompute.VirtualMachinesClientGetOptions{Expand: &expand})
			if gerr != nil {
				var respErr *azcore.ResponseError
				if errors.As(gerr, &respErr) && respErr.StatusCode == http.StatusNotFound {
					ch <- vmResult{armID: id, status: model.StatusTerminated}
					return
				}
				ch <- vmResult{armID: id, err: gerr}
				return
			}
			ch <- vmResult{armID: id, status: azurePowerStateToTBStatus(resp.Properties)}
		}(armID)
	}

	wg.Wait()
	close(ch)

	result := make(map[string]string, len(instanceIds))
	var firstErr error
	for r := range ch {
		if r.err != nil {
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		result[r.armID] = r.status
	}
	if firstErr != nil && len(result) == 0 {
		return nil, fmt.Errorf("Azure vmstatus: all requests failed; first error: %w", firstErr)
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
		case "stopping":
			return model.StatusSuspending
		case "stopped", "deallocating":
			return model.StatusSuspending
		case "deallocated":
			return model.StatusSuspended
		default:
			return model.StatusUndefined
		}
	}
	return model.StatusUndefined
}
