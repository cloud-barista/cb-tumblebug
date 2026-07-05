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
	"sync"

	armcompute "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
)

func init() {
	csp.RegisterBatchVMControlHandlers(csptypes.Azure, csp.BatchVMControlHandlers{
		Suspend: BatchSuspendInstances,
		Resume:  BatchResumeInstances,
		Reboot:  BatchRebootInstances,
	})
}

// Terminate is intentionally not registered here: unlike the OS disk (created with
// DeleteOption=Delete), the NIC and Public IP are not cascade-deleted when the VM
// is deleted, so a correct Terminate needs the same follow-up NIC/PublicIP cleanup
// CB-Spider's driver already does. Suspend/Resume/Reboot have no such cleanup
// concern — they only change power state — so they are safe to bypass CB-Spider for.

// azureControlConcurrency bounds concurrent Suspend/Resume/Reboot calls per batch, mirroring
// azureStatusConcurrency in vmstatus.go: Azure has no batch control API, so each VM
// requires an individual REST call, and an unbounded fan-out risks HTTP 429s.
const azureControlConcurrency = 20

// runBatchControl issues fn for each instance ARM ID (bounded by azureControlConcurrency)
// and collects successes into a map of armID -> resultStatus. Failed instances are
// omitted from the result map, matching BatchVMControlFunc's documented contract.
func runBatchControl(ctx context.Context, instanceIds []string, resultStatus string,
	fn func(ctx context.Context, vmClient *armcompute.VirtualMachinesClient, parts azureArmIDParts) error) (map[string]string, error) {

	if len(instanceIds) == 0 {
		return map[string]string{}, nil
	}

	creds, err := getCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("Azure vmcontrol: cannot get credentials: %w", err)
	}
	vmClient, err := getOrCreateVMClient(creds)
	if err != nil {
		return nil, fmt.Errorf("Azure vmcontrol: failed to get VM client: %w", err)
	}

	type ctrlResult struct {
		armID string
		err   error
	}
	ch := make(chan ctrlResult, len(instanceIds))
	sem := make(chan struct{}, azureControlConcurrency)

	var wg sync.WaitGroup
	for _, armID := range instanceIds {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			parts, perr := parseAzureArmID(id)
			if perr != nil {
				ch <- ctrlResult{armID: id, err: perr}
				return
			}
			ch <- ctrlResult{armID: id, err: fn(ctx, vmClient, parts)}
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
		result[r.armID] = resultStatus
	}
	if firstErr != nil && len(result) == 0 {
		return nil, fmt.Errorf("Azure vmcontrol: all requests failed; first error: %w", firstErr)
	}
	return result, nil
}

// BatchSuspendInstances issues BeginPowerOff for each VM and returns immediately after
// Azure accepts the request — it does not wait for the power-off to complete (unlike
// CB-Spider's driver, which blocks on PollUntilDone). Completion is picked up by the
// existing status poller (BatchDescribeInstanceStatuses) on its normal cadence.
func BatchSuspendInstances(ctx context.Context, region string, instanceIds []string) (map[string]string, error) {
	result, err := runBatchControl(ctx, instanceIds, model.StatusSuspending,
		func(ctx context.Context, vmClient *armcompute.VirtualMachinesClient, parts azureArmIDParts) error {
			_, err := vmClient.BeginPowerOff(ctx, parts.resourceGroup, parts.vmName, nil)
			return err
		})
	if err == nil {
		log.Debug().Str("region", region).Int("sent", len(instanceIds)).Int("accepted", len(result)).
			Msg("[Azure] BatchSuspendInstances completed")
	}
	return result, err
}

// BatchResumeInstances issues BeginStart for each VM and returns immediately after
// Azure accepts the request, for the same reason described in BatchSuspendInstances.
func BatchResumeInstances(ctx context.Context, region string, instanceIds []string) (map[string]string, error) {
	result, err := runBatchControl(ctx, instanceIds, model.StatusResuming,
		func(ctx context.Context, vmClient *armcompute.VirtualMachinesClient, parts azureArmIDParts) error {
			_, err := vmClient.BeginStart(ctx, parts.resourceGroup, parts.vmName, nil)
			return err
		})
	if err == nil {
		log.Debug().Str("region", region).Int("sent", len(instanceIds)).Int("accepted", len(result)).
			Msg("[Azure] BatchResumeInstances completed")
	}
	return result, err
}

// BatchRebootInstances issues BeginRestart for each VM and returns immediately after
// Azure accepts the request, for the same reason described in BatchSuspendInstances.
// BeginRestart is Azure's native soft-reboot operation, not a Stop+Start cycle.
func BatchRebootInstances(ctx context.Context, region string, instanceIds []string) (map[string]string, error) {
	result, err := runBatchControl(ctx, instanceIds, model.StatusRebooting,
		func(ctx context.Context, vmClient *armcompute.VirtualMachinesClient, parts azureArmIDParts) error {
			_, err := vmClient.BeginRestart(ctx, parts.resourceGroup, parts.vmName, nil)
			return err
		})
	if err == nil {
		log.Debug().Str("region", region).Int("sent", len(instanceIds)).Int("accepted", len(result)).
			Msg("[Azure] BatchRebootInstances completed")
	}
	return result, err
}
