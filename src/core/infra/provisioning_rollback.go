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

package infra

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	cspcheck "github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/rs/zerolog/log"
)

// isQuotaOrCapacityError reports whether err is a definitive CSP rejection meaning
// the VM was never created — quota exhausted or no available capacity in the region.
// These are not transient errors: retrying in the same region will keep failing until
// the quota is increased or capacity becomes available.
//
// Covered patterns (case-insensitive):
//
//	Azure:   "quota" (standardXxxFamily Cores quota), "operationnotallowed" (policy/quota),
//	         "insufficientcapacity", "skunotavailable"
//	AWS:     "vcpulimitexceeded", "instancelimitexceeded" (InsufficientInstanceCapacity is
//	         deliberately NOT matched: it is per-AZ and transient, cancelling a region for it is wrong)
//	GCP:     "quota_exceeded", "zone_resource_pool_exhausted", "rateLimitExceeded"
//	Alibaba: "instancequotaexceed", "operationdenied.quotaexceed"
//	NHN:     "overlimit"
//	Tencent: "quotaexceedlimit", "resourceinsufficient"
func isQuotaOrCapacityError(err error) bool {
	if err == nil {
		return false
	}
	// Transport failures and API throttling are transient: never treat them as a
	// region-wide quota rejection (they would cancel every remaining VM in the region).
	if isTransientNetworkError(err) || isApiThrottlingError(err) {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, pattern := range []string{
		"quota",
		"overlimit",
		"insufficientcapacity",
		"skunotavailable",
		"resource_pool_exhausted",
		"resourceinsufficient",
		"limitexceeded",
		"operationnotallowed",
		"ratelimitexceeded",
		"operationdenied",
		"사용 가능한 공인 ip가 없습니다",         // KT: 422 with a Korean title.
		"no available public ip",     // NHN: 500 "Failed to Create Public IP
		"failed to create public ip", // NCP: server creation limit reached
		"creation limit",
	} {
		if strings.Contains(msg, pattern) {
			return true
		}
	}
	return false
}

// recordNodeFailure classifies a CSP rejection and stores it on the node, so a
// later retry can act on the failure class and the zone that was attempted
// instead of re-parsing the message. SystemMessage keeps the one-line summary
// for display; the structured record lives in NodeInfo.Failure.
//
// Region/Zone are also filled in here: on the success path they come from
// CB-Spider's response, so without this a failed node carries no placement
// information at all and nothing can tell where it was tried.
func recordNodeFailure(nodeInfoData *model.NodeInfo, provider, attemptedZone string, err error) {
	if nodeInfoData == nil || err == nil {
		return
	}
	region := nodeInfoData.ConnectionConfig.RegionDetail.RegionName
	if region == "" {
		region = nodeInfoData.Region.Region
	}

	failure := cspcheck.ClassifyProvisioningFailure(provider, region, attemptedZone, err.Error())
	nodeInfoData.Failure = &failure
	// Keep the full CSP text here — it is what the UI renders today, and it is
	// now redacted and trimmed of provider debris. failure.Message holds the
	// one-line form for callers that want it.
	nodeInfoData.SystemMessage = failure.RawMessage

	if nodeInfoData.Region.Region == "" {
		nodeInfoData.Region.Region = region
	}
	if nodeInfoData.Region.Zone == "" {
		nodeInfoData.Region.Zone = failure.AttemptedZone
	}
}

// handleHoldOption handles the hold option logic
func handleHoldOption(nsId, infraId string) error {
	key := common.GenInfraKey(nsId, infraId, "")
	holdingInfraMap.Store(key, "holding")

	for {
		value, ok := holdingInfraMap.Load(key)
		if !ok {
			break
		}
		if value == "continue" {
			holdingInfraMap.Delete(key)
			break
		} else if value == "withdraw" {
			holdingInfraMap.Delete(key)
			DelInfra(nsId, infraId, "force")
			return fmt.Errorf("Infra creation was withdrawn by user")
		}

		log.Info().Msgf("Infra: %s (holding)", key)
		time.Sleep(5 * time.Second)
	}

	return nil
}

// cleanupPartialInfra cleans up partially created Infra resources.
// Uses option=terminate (refine → terminate all nodes → wait for CSP propagation →
// delete records), NOT force: some VMs may have been created successfully on the CSP,
// and force would delete only the CB-TB records, leaving those VMs as orphans.
func cleanupPartialInfra(nsId, infraId string) error {
	log.Warn().Msgf("Cleaning up partial Infra: %s/%s", nsId, infraId)

	_, err := DelInfra(nsId, infraId, model.ActionTerminate)
	if err != nil {
		return fmt.Errorf("failed to cleanup partial Infra: %w", err)
	}

	return nil
}

// CreatedResource represents a resource created during dynamic Infra provisioning
type CreatedResource struct {
	Type string `json:"type"` // "vnet", "sshkey", "securitygroup"
	Id   string `json:"id"`   // Resource ID
}

// NodeReqWithCreatedResources contains Node request and list of created resources for rollback
type NodeReqWithCreatedResources struct {
	VmReq            *model.CreateNodeGroupReq `json:"nodeReq"`
	CreatedResources []CreatedResource         `json:"createdResources"`
}

// rollbackCreatedResources deletes only the resources that were created during this Infra creation
func rollbackCreatedResources(nsId string, createdResources []CreatedResource) error {
	var errors []string
	var successes []string

	vNetIds := make([]string, 0)
	sshKeyIds := make([]string, 0)
	securityGroupIds := make([]string, 0)

	log.Info().Msgf("Starting rollback process for %d resources in namespace '%s'", len(createdResources), nsId)

	// Group resources by type for logging
	for _, res := range createdResources {
		switch res.Type {
		case model.StrVNet:
			vNetIds = append(vNetIds, res.Id)
		case model.StrSSHKey:
			sshKeyIds = append(sshKeyIds, res.Id)
		case model.StrSecurityGroup:
			securityGroupIds = append(securityGroupIds, res.Id)
		}
	}

	log.Info().Msgf("Resources to rollback: VNet(%d): %v, SSHKey(%d): %v, SecurityGroup(%d): %v",
		len(vNetIds), vNetIds, len(sshKeyIds), sshKeyIds, len(securityGroupIds), securityGroupIds)

	// Use semaphore for parallel processing with concurrency limit
	const maxConcurrency = 10
	semaphore := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	var mutex sync.Mutex

	// Delete SSHKeys first (usually least dependent) in parallel
	for _, res := range sshKeyIds {
		wg.Add(1)
		go func(resourceId string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }() // Release semaphore

			if err := resource.DelResource(nsId, model.StrSSHKey, resourceId, "false"); err != nil {
				errorMsg := fmt.Sprintf("Failed to delete SSHKey '%s' in namespace '%s': %v", resourceId, nsId, err)
				mutex.Lock()
				errors = append(errors, errorMsg)
				mutex.Unlock()
				log.Error().Err(err).Msgf("Rollback failed for SSHKey: %s", resourceId)
			} else {
				successMsg := fmt.Sprintf("SSHKey '%s'", resourceId)
				mutex.Lock()
				successes = append(successes, successMsg)
				mutex.Unlock()
				log.Info().Msgf("Successfully rolled back SSHKey: %s", resourceId)
			}
		}(res)
	}

	// Wait for all SSHKey deletions to complete
	wg.Wait()
	log.Info().Msgf("Completed SSHKey deletions: %d successful, %d failed", len(sshKeyIds), len(errors))

	// Delete SecurityGroups second in parallel
	for _, res := range securityGroupIds {
		wg.Add(1)
		go func(resourceId string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }() // Release semaphore

			if err := resource.DelResource(nsId, model.StrSecurityGroup, resourceId, "false"); err != nil {
				errorMsg := fmt.Sprintf("Failed to delete SecurityGroup '%s' in namespace '%s': %v", resourceId, nsId, err)
				mutex.Lock()
				errors = append(errors, errorMsg)
				mutex.Unlock()
				log.Error().Err(err).Msgf("Rollback failed for SecurityGroup: %s", resourceId)
			} else {
				successMsg := fmt.Sprintf("SecurityGroup '%s'", resourceId)
				mutex.Lock()
				successes = append(successes, successMsg)
				mutex.Unlock()
				log.Info().Msgf("Successfully rolled back SecurityGroup: %s", resourceId)
			}
		}(res)
	}

	// Wait for all SecurityGroup deletions to complete
	wg.Wait()
	log.Info().Msgf("Completed SecurityGroup deletions: %d total attempted", len(securityGroupIds))

	// wait for 5 secs for safe rollback
	time.Sleep(5 * time.Second)

	// Delete VNets last (most dependent) in parallel
	for _, res := range vNetIds {
		wg.Add(1)
		go func(resourceId string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }() // Release semaphore

			if err := resource.DelResource(nsId, model.StrVNet, resourceId, "false"); err != nil {
				errorMsg := fmt.Sprintf("Failed to delete VNet '%s' in namespace '%s': %v", resourceId, nsId, err)
				mutex.Lock()
				errors = append(errors, errorMsg)
				mutex.Unlock()
				log.Error().Err(err).Msgf("Rollback failed for VNet: %s", resourceId)
			} else {
				successMsg := fmt.Sprintf("VNet '%s'", resourceId)
				mutex.Lock()
				successes = append(successes, successMsg)
				mutex.Unlock()
				log.Info().Msgf("Successfully rolled back VNet: %s", resourceId)
			}
		}(res)
	}

	// Wait for all VNet deletions to complete
	wg.Wait()
	log.Info().Msgf("Completed VNet deletions: %d total attempted", len(vNetIds))

	// Log rollback summary
	log.Info().Msgf("Rollback summary: Success(%d): %v, Failed(%d): %d errors",
		len(successes), successes, len(errors), len(errors))

	if len(errors) > 0 {
		return fmt.Errorf("rollback completed with %d errors: %s", len(errors), strings.Join(errors, "; "))
	}

	log.Info().Msgf("All %d resources successfully rolled back in namespace '%s'", len(createdResources), nsId)
	return nil
}
