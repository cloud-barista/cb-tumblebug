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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/netutil"
	cspcheck "github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/rs/zerolog/log"
)

func CheckInfraDynamicReq(ctx context.Context, req *model.InfraConnectionConfigCandidatesReq) (*model.CheckInfraDynamicReqInfo, error) {

	credentialHolder := common.CredentialHolderFromContext(ctx)
	infraReqInfo := model.CheckInfraDynamicReqInfo{}

	connectionConfigList, err := common.GetConnConfigList(credentialHolder, true, true)
	if err != nil {
		err := fmt.Errorf("cannot load ConnectionConfigList in Infra dynamic request check")
		log.Error().Err(err).Msg("")
		return &infraReqInfo, err
	}

	// Find detail info and ConnectionConfigCandidates
	for _, k := range req.SpecIds {
		errMessage := ""

		nodeReqInfo := model.CheckNodeGroupDynamicReqInfo{}

		specInfo, err := resource.GetSpec(model.SystemCommonNs, k)
		if err != nil {
			log.Error().Err(err).Msg("")
			errMessage += "//Failed to get Spec (" + k + ")."
		}

		regionInfo, err := common.GetRegion(specInfo.ProviderName, specInfo.RegionName)
		if err != nil {
			errMessage += "//Failed to get Region (" + specInfo.RegionName + ") for Spec (" + k + ") is not found."
		}

		for _, connectionConfig := range connectionConfigList.Connectionconfig {
			if connectionConfig.ProviderName == specInfo.ProviderName && strings.Contains(connectionConfig.RegionDetail.RegionName, specInfo.RegionName) {
				nodeReqInfo.ConnectionConfigCandidates = append(nodeReqInfo.ConnectionConfigCandidates, connectionConfig.ConfigName)
			}
		}

		nodeReqInfo.Spec = specInfo
		availableImageList, err := resource.GetImagesByRegion(model.SystemCommonNs, specInfo.ProviderName, specInfo.RegionName)
		if err != nil {
			errMessage += "//Failed to search images for Spec (" + k + ")"
		}
		nodeReqInfo.Image = availableImageList
		nodeReqInfo.Region = regionInfo
		nodeReqInfo.SystemMessage = errMessage
		infraReqInfo.ReqCheck = append(infraReqInfo.ReqCheck, nodeReqInfo)
	}

	return &infraReqInfo, err
}

// CreateInfraDynamic is func to create Infra obeject and deploy requested VMs in a dynamic way
func CreateInfraDynamic(ctx context.Context, nsId string, req *model.InfraDynamicReq, deployOption string) (*model.InfraInfo, error) {

	reqID := common.RequestIDFromContext(ctx)
	credentialHolder := common.CredentialHolderFromContext(ctx)

	// Initialize comprehensive error tracking
	var errorHistory []string

	// Helper function to add errors to history
	addErrorToHistory := func(phase, details string) {
		timestamp := time.Now().Format("15:04:05")
		errorHistory = append(errorHistory, fmt.Sprintf("[%s] %s: %s", timestamp, phase, details))
	}

	infraReq := model.InfraReq{}
	infraReq.Name = req.Name
	infraReq.Label = req.Label
	infraReq.SystemLabel = req.SystemLabel
	infraReq.InstallMonAgent = req.InstallMonAgent
	infraReq.Description = req.Description
	infraReq.PostCommands = req.PostCommands
	infraReq.PostCommandAsync = req.PostCommandAsync
	infraReq.PolicyOnPartialFailure = req.PolicyOnPartialFailure

	// Validate post-deployment command request shape early (before any provisioning)
	if err := ValidatePostCommandRequest(req.PostCommands); err != nil {
		log.Error().Err(err).Msg("")
		return &model.InfraInfo{}, err
	}

	emptyInfra := &model.InfraInfo{}
	err := common.CheckString(nsId)
	if err != nil {
		err := fmt.Errorf("invalid namespace. %w", err)
		log.Error().Err(err).Msg("")
		addErrorToHistory("Namespace Validation", err.Error())
		return emptyInfra, err
	}
	check, err := CheckInfra(nsId, req.Name)
	if err != nil {
		err := fmt.Errorf("invalid infra name. %w", err)
		log.Error().Err(err).Msg("")
		addErrorToHistory("Infra Name Validation", err.Error())
		return emptyInfra, err
	}
	if check {
		err := fmt.Errorf("The infra %s already exists.", req.Name)
		addErrorToHistory("Infra Existence Check", err.Error())
		return emptyInfra, err
	}

	// Initialize Infra
	uid := common.GenUid()
	infraId := req.Name

	if err := createInfraObject(ctx, nsId, infraId, &infraReq, uid, ""); err != nil {
		addErrorToHistory("Infra Object Creation", err.Error())
		return emptyInfra, err
	}
	// Get Infra object
	infraTmp, _, err := GetInfraObject(nsId, infraId)
	if err != nil {
		addErrorToHistory("Infra Object Retrieval", err.Error())
		return emptyInfra, err
	}
	// start infra provisioning with StatusPreparing
	infraTmp.Status = model.StatusPreparing
	UpdateInfraInfo(nsId, infraTmp)

	nodeGroupReqs := req.NodeGroups

	// Propagate Infra-level template IDs to NodeGroups that don't specify their own
	for i := range nodeGroupReqs {
		if nodeGroupReqs[i].VNetTemplateId == "" && req.VNetTemplateId != "" {
			nodeGroupReqs[i].VNetTemplateId = req.VNetTemplateId
		}
		if nodeGroupReqs[i].SgTemplateId == "" && req.SgTemplateId != "" {
			nodeGroupReqs[i].SgTemplateId = req.SgTemplateId
		}
	}

	// Check whether VM names meet requirement.
	// Use semaphore for parallel processing with concurrency limit
	const maxConcurrency = 10
	semaphore := make(chan struct{}, maxConcurrency)

	var wg sync.WaitGroup
	var mutex sync.Mutex
	var validationErrors []string

	for i, k := range nodeGroupReqs {
		wg.Add(1)
		go func(index int, nodeGroupReq model.CreateNodeGroupDynamicReq) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }() // Release semaphore

			// log VM request details
			log.Debug().Msgf("[%d] VM Request: %+v", index, nodeGroupReq)

			err := checkCommonResAvailableForNodeGroupDynamicReq(ctx, &nodeGroupReq, nsId)
			if err != nil {
				log.Error().Err(err).Msgf("[%d] Failed to find common resource for Infra provision", index)
				mutex.Lock()
				validationErrors = append(validationErrors, fmt.Sprintf("NodeGroup[%d] '%s': %s",
					index+1, nodeGroupReq.Name, err.Error()))
				// Add to error history with more context
				addErrorToHistory("Resource Validation",
					fmt.Sprintf("NodeGroup '%s' (Index: %d) failed validation: %s",
						nodeGroupReq.Name, index+1, err.Error()))
				mutex.Unlock()
			}
		}(i, k)
	}

	wg.Wait()

	if len(validationErrors) > 0 {
		// Clean up Infra object on validation failure
		DelInfra(nsId, infraId, "force")

		// Build comprehensive error message with history
		var errorMsg strings.Builder
		errorMsg.WriteString(fmt.Sprintf("Infra '%s' validation failed due to resource availability errors.\n\n", req.Name))

		// Add error history if available
		if len(errorHistory) > 0 {
			errorMsg.WriteString("Error Timeline:\n")
			for i, errEntry := range errorHistory {
				errorMsg.WriteString(fmt.Sprintf(" %d. %s\n", i+1, errEntry))
			}
			errorMsg.WriteString("\n")
		}

		// Add validation error details
		errorMsg.WriteString("Resource Validation Failures:\n")
		for _, errStr := range validationErrors {
			errorMsg.WriteString(fmt.Sprintf(" • %s\n", errStr))
		}
		errorMsg.WriteString(fmt.Sprintf("\nSummary: %d out of %d NodeGroups failed validation", len(validationErrors), len(nodeGroupReqs)))

		return emptyInfra, errors.New(errorMsg.String())
	}

	// Check if nodeRequest has elements
	if len(nodeGroupReqs) > 0 {
		// allCreatedResources tracks ALL resources created during the preparation phase,
		// including those from failed NodeGroups. This enables cleanup under rollback policy.
		var allCreatedResources []CreatedResource
		var wg sync.WaitGroup
		var mutex sync.Mutex

		type nodeResult struct {
			result *NodeReqWithCreatedResources
			err    error
		}
		resultChan := make(chan nodeResult, len(nodeGroupReqs))

		// Group nodeGroupReqs by connectionName for sequential processing
		connectionGroups := make(map[string][]model.CreateNodeGroupDynamicReq)

		// First, determine the connection name for each nodeGroup
		for _, nodeGroupReq := range nodeGroupReqs {
			// Get spec info to determine connection
			specInfo, err := resource.GetSpec(model.SystemCommonNs, nodeGroupReq.SpecId)
			if err != nil {
				log.Error().Err(err).Msgf("Failed to get spec info for grouping: %s", nodeGroupReq.SpecId)
				// Add error to result channel instead of continuing
				resultChan <- nodeResult{
					result: nil,
					err:    fmt.Errorf("failed to get spec info for NodeGroup '%s': %w", nodeGroupReq.Name, err),
				}
				continue
			}

			connectionName := common.ResolveConnectionName(specInfo.ConnectionName, credentialHolder)
			// credentialHolder already extracted from ctx above
			if nodeGroupReq.ConnectionName != "" {
				connectionName = nodeGroupReq.ConnectionName
			}

			// Group by connection name
			connectionGroups[connectionName] = append(connectionGroups[connectionName], nodeGroupReq)
		}

		// Warn when the same connection has NodeGroups with different VNetTemplateIds.
		// Different templates result in separate VPCs within the same CSP region, so VMs
		// in those NodeGroups cannot communicate directly without VPC peering.
		for connName, nodeGroups := range connectionGroups {
			if len(nodeGroups) < 2 {
				continue
			}
			firstTemplate := nodeGroups[0].VNetTemplateId
			for _, sg := range nodeGroups[1:] {
				if sg.VNetTemplateId != firstTemplate {
					log.Warn().Msgf(
						"Connection '%s' has NodeGroups with different VNetTemplateIds ('%s' vs '%s'). "+
							"Each template creates an independent VPC; VMs across these NodeGroups cannot communicate directly without VPC peering.",
						connName, firstTemplate, sg.VNetTemplateId,
					)
					break
				}
			}
		}

		log.Info().Msgf("Grouped %d NodeGroups into %d connection groups", len(nodeGroupReqs), len(connectionGroups))

		// Process each connection group in parallel, but VMs within each group sequentially
		for connectionName, nodeGroupsInConnection := range connectionGroups {
			wg.Add(1)
			go func(connName string, nodeGroups []model.CreateNodeGroupDynamicReq) {
				defer wg.Done()

				log.Info().Msgf("Processing %d NodeGroups for connection '%s' sequentially", len(nodeGroups), connName)

				// Process NodeGroups in this connection sequentially
				for i, nodeGroupDynamicReq := range nodeGroups {
					log.Debug().Msgf("[%s][%d/%d] Processing NodeGroup '%s' sequentially",
						connName, i+1, len(nodeGroups), nodeGroupDynamicReq.Name)

					// Add small delay between sequential requests to avoid rate limiting
					if i > 0 {
						time.Sleep(2 * time.Second)
					}

					result, err := getNodeGroupReqFromDynamicReq(ctx, nsId, infraId, &nodeGroupDynamicReq)
					resultChan <- nodeResult{result: result, err: err}
				}

				log.Info().Msgf("Completed processing NodeGroups for connection '%s'", connName)
			}(connectionName, nodeGroupsInConnection)
		}

		// Wait for all goroutines to complete
		wg.Wait()
		close(resultChan)

		// Collect results and check for errors
		var hasError bool
		var failedNodeGroups []string
		var errorDetails []string
		var successfulNodeGroups []string

		for nodeRes := range resultChan {
			if nodeRes.err != nil {
				log.Error().Err(nodeRes.err).Msg("Failed to prepare resources for dynamic Infra creation")
				hasError = true

				// Extract NodeGroup details from error context
				nodeGroupName := "unknown"
				if nodeRes.result != nil && nodeRes.result.VmReq != nil {
					nodeGroupName = nodeRes.result.VmReq.Name
				}
				failedNodeGroups = append(failedNodeGroups, nodeGroupName)
				errorDetails = append(errorDetails, fmt.Sprintf("NodeGroup '%s': %s", nodeGroupName, nodeRes.err.Error()))

				// Add to error history
				addErrorToHistory("NodeGroup Resource Preparation",
					fmt.Sprintf("Failed to prepare resources for NodeGroup '%s': %s", nodeGroupName, nodeRes.err.Error()))

				// Track resources that were partially created before the failure so they can
				// be cleaned up if rollback policy is in effect.
				mutex.Lock()
				if nodeRes.result != nil && len(nodeRes.result.CreatedResources) > 0 {
					log.Info().Msgf("NodeGroup '%s' failed after creating %d resource(s); tracking for potential rollback",
						nodeGroupName, len(nodeRes.result.CreatedResources))
					allCreatedResources = append(allCreatedResources, nodeRes.result.CreatedResources...)
				}
				mutex.Unlock()
			} else {
				// Safely append to the shared infraReq.NodeGroups slice
				mutex.Lock()
				infraReq.NodeGroups = append(infraReq.NodeGroups, *nodeRes.result.VmReq)
				allCreatedResources = append(allCreatedResources, nodeRes.result.CreatedResources...)
				successfulNodeGroups = append(successfulNodeGroups, nodeRes.result.VmReq.Name)
				mutex.Unlock()
			}
		}

		// Handle resource preparation failures
		if hasError {
			// Get updated Infra object
			infraTmp, _, err := GetInfraObject(nsId, infraId)
			if err == nil {
				// Add general error summary to both SystemMessage and error history
				errorSummary := fmt.Sprintf("Resource preparation failed for %d NodeGroup(s) out of %d total NodeGroups", len(failedNodeGroups), len(failedNodeGroups)+len(successfulNodeGroups))
				infraTmp.SystemMessage = append(infraTmp.SystemMessage, errorSummary)
				addErrorToHistory("Resource Preparation Summary", errorSummary)

				// Add detailed error messages for each failed NodeGroup to both SystemMessage and error history
				for _, detail := range errorDetails {
					infraTmp.SystemMessage = append(infraTmp.SystemMessage, detail)
					addErrorToHistory("NodeGroup Resource Failure", detail)
				}

				// Check if ALL NodeGroups failed - if so, set status to Failed and return immediately
				if len(successfulNodeGroups) == 0 {
					addErrorToHistory("Infra Status Decision", "All NodeGroups failed resource preparation - marking Infra as Failed")
					infraTmp.SystemMessage = append(infraTmp.SystemMessage, "Infra creation aborted: All NodeGroups failed resource preparation")
					infraTmp.Status = model.StatusFailed
					UpdateInfraInfo(nsId, infraTmp)

					// Rollback any shared resources (VNet/SshKey/SG) that were partially created
					// before the failures. These resources are shared-namespace resources so they
					// will not be automatically cleaned up by Infra deletion.
					if len(allCreatedResources) > 0 {
						log.Info().Msgf("All NodeGroups failed — rolling back %d partially created shared resource(s)", len(allCreatedResources))
						if rollbackErr := rollbackCreatedResources(nsId, allCreatedResources); rollbackErr != nil {
							log.Warn().Err(rollbackErr).Msg("Partial rollback failure during all-NodeGroups-failed cleanup; some shared resources may remain")
							addErrorToHistory("Shared Resource Rollback", fmt.Sprintf("Rollback encountered errors: %s", rollbackErr.Error()))
						} else {
							addErrorToHistory("Shared Resource Rollback", fmt.Sprintf("Successfully rolled back %d shared resource(s)", len(allCreatedResources)))
						}
					}

					// Build comprehensive error message with complete history
					var errorMsg strings.Builder
					errorMsg.WriteString(fmt.Sprintf("Infra '%s' creation failed - all NodeGroups failed resource preparation.\n\n", req.Name))

					// Add full error history
					if len(errorHistory) > 0 {
						errorMsg.WriteString("Complete Error Timeline:\n")
						for i, errEntry := range errorHistory {
							errorMsg.WriteString(fmt.Sprintf("  %d. %s\n", i+1, errEntry))
						}
						errorMsg.WriteString("\n")
					}

					errorMsg.WriteString("Summary: All NodeGroups failed during resource preparation phase.\n")
					errorMsg.WriteString("Common causes: VPC/subnet limits, insufficient permissions, region capacity issues, or network configuration problems.\n")
					errorMsg.WriteString("Check the error timeline above for specific failure details.")

					return emptyInfra, fmt.Errorf("%s", errorMsg.String())
				}

				// Partial failure: some NodeGroups succeeded, some failed.
				// Apply PolicyOnPartialFailure to decide whether to rollback or continue.
				switch req.PolicyOnPartialFailure {
				case model.PolicyRollback:
					// Roll back ALL created shared resources (from both successful and failed NodeGroups)
					// because the user requested all-or-nothing semantics.
					addErrorToHistory("Infra Status Decision",
						fmt.Sprintf("Partial failure with policy=rollback: rolling back all %d created shared resource(s)", len(allCreatedResources)))
					log.Warn().Msgf("Partial NodeGroup failure with policy=rollback: rolling back %d shared resource(s)", len(allCreatedResources))
					if len(allCreatedResources) > 0 {
						if rollbackErr := rollbackCreatedResources(nsId, allCreatedResources); rollbackErr != nil {
							log.Warn().Err(rollbackErr).Msg("Partial rollback failure; some shared resources may remain")
						}
					}
					if cleanupErr := cleanupPartialInfra(nsId, infraId); cleanupErr != nil {
						log.Error().Err(cleanupErr).Msg("Failed to cleanup partial Infra during rollback")
					}
					return emptyInfra, fmt.Errorf("Infra '%s' creation aborted: %d NodeGroup(s) failed resource preparation and policy=rollback; all created resources have been cleaned up",
						req.Name, len(failedNodeGroups))
				default:
					// continue or refine: proceed with the successfully prepared NodeGroups
					addErrorToHistory("Infra Status Decision",
						fmt.Sprintf("Partial success: %d NodeGroups succeeded, %d failed - continuing with partial Infra creation (policy=%s)",
							len(successfulNodeGroups), len(failedNodeGroups), req.PolicyOnPartialFailure))
				}
				UpdateInfraInfo(nsId, infraTmp)
			}
		}

		// After processing all NodeGroups, check final state
		// Get updated Infra object for final status determination
		infraTmp, _, err := GetInfraObject(nsId, infraId)
		if err != nil {
			addErrorToHistory("Infra Object Retrieval for Final Status Check", err.Error())
			return emptyInfra, err
		}

		// Final check: if no NodeGroups were successfully prepared, mark as Failed
		if len(infraReq.NodeGroups) == 0 {
			addErrorToHistory("Final Status Decision", "No NodeGroups were successfully prepared - marking Infra as Failed")
			infraTmp.SystemMessage = append(infraTmp.SystemMessage, "Infra creation failed: No NodeGroups were successfully prepared")
			infraTmp.Status = model.StatusFailed
			UpdateInfraInfo(nsId, infraTmp)

			// Build comprehensive error message
			var errorMsg strings.Builder
			errorMsg.WriteString(fmt.Sprintf("Infra '%s' creation failed - no NodeGroups were successfully prepared.\n\n", req.Name))

			// Add full error history
			if len(errorHistory) > 0 {
				errorMsg.WriteString("Complete Error Timeline:\n")
				for i, errEntry := range errorHistory {
					errorMsg.WriteString(fmt.Sprintf("  %d. %s\n", i+1, errEntry))
				}
				errorMsg.WriteString("\n")
			}

			errorMsg.WriteString("Summary: All NodeGroups failed during resource preparation phase.\n")
			errorMsg.WriteString("This indicates that no VM NodeGroups could be prepared for provisioning.\n")
			errorMsg.WriteString("Check the error timeline above for specific failure details.")

			return emptyInfra, fmt.Errorf("%s", errorMsg.String())
		}
	}

	// Only proceed to StatusPrepared if we have successful NodeGroups
	infraTmp, _, err = GetInfraObject(nsId, infraId)
	if err != nil {
		addErrorToHistory("Infra Object Retrieval for Status Update", err.Error())
		return emptyInfra, err
	}

	// marking the infra is in StatusPrepared
	infraTmp.Status = model.StatusPrepared
	addErrorToHistory("Infra Status Update", fmt.Sprintf("Infra marked as Prepared with %d successful NodeGroups", len(infraReq.NodeGroups)))
	UpdateInfraInfo(nsId, infraTmp)

	// Log the prepared Infra request and update the progress
	common.PrintJsonPretty(infraReq)
	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{
		Title: fmt.Sprintf("Prepared %d resources for provisioning Infra: %s", len(infraReq.NodeGroups), infraReq.Name),
		Info:  infraReq, Time: time.Now(),
	})
	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{
		Title: "Start instance provisioning", Time: time.Now(),
	})

	// Run create Infra with the generated Infra request
	option := "create"
	if deployOption == "hold" {
		option = "hold"
	}
	result, err := CreateInfra(ctx, nsId, &infraReq, option, true)

	// If CreateInfra fails, build comprehensive error message with history
	if err != nil {
		// Do NOT add the full CreateInfra error to history — it will be shown once in Detail below.
		// Only record a brief note in the timeline.
		addErrorToHistory("Infra Creation", fmt.Sprintf("Infra '%s' creation failed (see Detail below)", req.Name))

		// Build comprehensive error message
		var errorMsg strings.Builder
		errorMsg.WriteString(fmt.Sprintf("Infra '%s' creation failed in final provisioning stage.\n\n", req.Name))

		// Add error history (timeline events only, no full error duplication)
		if len(errorHistory) > 0 {
			errorMsg.WriteString("Complete Error Timeline:\n")
			for i, errEntry := range errorHistory {
				errorMsg.WriteString(fmt.Sprintf("  %d. %s\n", i+1, errEntry))
			}
			errorMsg.WriteString("\n")
		}

		// Full error appears only once here
		errorMsg.WriteString(fmt.Sprintf("Detail: %s\n", err.Error()))

		// Check if NodeGroups is empty (which causes the validation error in CreateInfra)
		if len(infraReq.NodeGroups) == 0 {
			errorMsg.WriteString("\nRoot Cause: No VM NodeGroups were successfully prepared for provisioning.\n")
			errorMsg.WriteString("This typically indicates that all VM resource preparation failed during the earlier stages.\n")
			errorMsg.WriteString("Please check the error timeline above for specific resource creation failures (e.g., VPC limits, permissions, etc.).")
		}

		return result, fmt.Errorf("%s", errorMsg.String())
	}

	return result, err
}

// ValidateInfraDynamicReq is func to validate Infra dynamic request before actual provisioning

func CreateSystemInfraDynamic(option string) (*model.InfraInfo, error) {
	nsId := model.SystemCommonNs
	req := &model.InfraDynamicReq{}

	// special purpose Infra
	req.Name = option
	labels := map[string]string{
		model.LabelPurpose: option,
	}
	req.Label = labels
	req.SystemLabel = option
	req.Description = option
	req.InstallMonAgent = "no"

	switch option {
	case "probe":
		connections, err := common.GetConnConfigList(model.DefaultCredentialHolder, true, true)
		if err != nil {
			log.Error().Err(err).Msg("")
			return nil, err
		}
		for _, v := range connections.Connectionconfig {

			nodeGroupDynamicReq := &model.CreateNodeGroupDynamicReq{}
			nodeGroupDynamicReq.ImageId = "ubuntu22.04"                // temporal default value. will be changed
			nodeGroupDynamicReq.SpecId = "aws-ap-northeast-2-t2-small" // temporal default value. will be changed

			recommendSpecReq := model.RecommendSpecReq{}
			condition := []model.Operation{}
			condition = append(condition, model.Operation{Operand: v.RegionZoneInfoName})

			log.Debug().Msg(" - v.RegionName: " + v.RegionZoneInfoName)

			recommendSpecReq.Filter.Policy = append(recommendSpecReq.Filter.Policy, model.FilterCondition{Metric: "region", Condition: condition})
			recommendSpecReq.Limit = 1
			common.PrintJsonPretty(recommendSpecReq)

			specList, err := RecommendSpec(common.NewDefaultContext(), model.SystemCommonNs, recommendSpecReq)
			if err != nil {
				log.Error().Err(err).Msg("")
				return nil, err
			}
			if len(specList) != 0 {
				recommendedSpec := specList[0].Id
				nodeGroupDynamicReq.SpecId = recommendedSpec

				nodeGroupDynamicReq.Label = labels
				nodeGroupDynamicReq.Name = nodeGroupDynamicReq.SpecId

				nodeGroupDynamicReq.RootDiskType = specList[0].RootDiskType
				nodeGroupDynamicReq.RootDiskSize = specList[0].RootDiskSize
				req.NodeGroups = append(req.NodeGroups, *nodeGroupDynamicReq)
			}
		}

	default:
		err := fmt.Errorf("Not available option. Try (option=probe)")
		return nil, err
	}
	if req.NodeGroups == nil {
		err := fmt.Errorf("No VM is defined")
		return nil, err
	}

	return CreateInfraDynamic(common.NewDefaultContext(), nsId, req, "")
}

// CreateInfraNodeGroupDynamic is func to create requested VM in a dynamic way and add it to Infra
func CreateInfraNodeGroupDynamic(ctx context.Context, nsId string, infraId string, req *model.AddNodeGroupDynamicReq) (*model.InfraInfo, error) {

	emptyInfra := &model.InfraInfo{}
	nodeGroupId := req.Name
	check, err := CheckNodeGroup(nsId, infraId, nodeGroupId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyInfra, err
	}
	if check {
		err := fmt.Errorf("The name for NodeGroup (prefix of VM Id) %s already exists.", req.Name)
		return emptyInfra, err
	}

	err = checkCommonResAvailableForNodeGroupDynamicReq(ctx, &req.CreateNodeGroupDynamicReq, nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyInfra, err
	}

	nodeReqResult, err := getNodeGroupReqFromDynamicReq(ctx, nsId, infraId, &req.CreateNodeGroupDynamicReq)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyInfra, err
	}

	infraResult, err := CreateInfraGroupNode(ctx, nsId, infraId, nodeReqResult.VmReq, true)
	if err != nil {
		return infraResult, err
	}

	// Bootstrap the newly added nodeGroup (phases without an explicit target
	// are scoped to this group so existing nodes are untouched)
	phases := req.PostCommands
	if len(phases) > 0 {
		xRequestId := newPostCommandRequestId(infraId)
		markPostCommandRunning(nsId, infraId, xRequestId)
		if req.PostCommandAsync {
			log.Info().Msgf("Bootstrapping added NodeGroup in background (xRequestId: %s)", xRequestId)
			go func() {
				if _, cmdErr := executePostCommands(nsId, infraId, nodeGroupId, phases, xRequestId); cmdErr != nil {
					log.Error().Err(cmdErr).Str("xRequestId", xRequestId).Msg("Background NodeGroup bootstrap failed")
				}
			}()
		} else if _, cmdErr := executePostCommands(nsId, infraId, nodeGroupId, phases, xRequestId); cmdErr != nil {
			log.Error().Err(cmdErr).Msg("Post-deployment commands failed for the added NodeGroup, but continuing")
		}
		if refreshed, refErr := GetInfraInfo(nsId, infraId); refErr == nil {
			infraResult = refreshed
		}
	}
	return infraResult, nil
}

// checkCommonResAvailableForNodeGroupDynamicReq is func to check common resources availability for NodeGroupDynamicReq
func checkCommonResAvailableForNodeGroupDynamicReq(ctx context.Context, req *model.CreateNodeGroupDynamicReq, nsId string) error {

	credentialHolder := common.CredentialHolderFromContext(ctx)

	log.Debug().Msgf("Checking common resources for VM Dynamic Request: %+v", req)
	log.Debug().Msgf("Namespace ID: %s", nsId)

	// Get spec info first (required for both spec and image validation)
	specInfo, err := resource.GetSpec(model.SystemCommonNs, req.SpecId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get spec info")
		return fmt.Errorf("failed to get VM specification '%s': %w", req.SpecId, err)
	}

	// Resolve connection name based on credential holder
	resolvedConnectionName := common.ResolveConnectionName(specInfo.ConnectionName, credentialHolder)

	// Channel to collect errors from parallel goroutines
	errorChan := make(chan error, 2)

	// Check spec availability in parallel
	go func() {
		var specAvailable bool
		var specCheckErr error

		// Use the provider-agnostic availability checker; fall back to
		// CB-Spider LookupSpec when no checker is registered for the provider.
		availability := cspcheck.CheckAvailability(ctx, model.AvailabilityQuery{
			Provider:     csp.ResolveCloudPlatform(specInfo.ProviderName),
			Region:       specInfo.RegionName,
			InstanceType: specInfo.CspSpecName,
		})
		if availability.Source == "none" {
			_, specCheckErr = resource.LookupSpec(resolvedConnectionName, specInfo.CspSpecName)
			if specCheckErr == nil {
				specAvailable = true
			}
		} else {
			specAvailable = availability.Available
			if !specAvailable {
				specCheckErr = fmt.Errorf("%s", availability.Reason)
			}
		}

		if specCheckErr != nil || !specAvailable {
			errMsg := "spec not available in CSP"
			if specCheckErr != nil {
				errMsg = specCheckErr.Error()
			}
			log.Error().Msgf("Spec validation failed for %s: %s", specInfo.CspSpecName, errMsg)
			errorChan <- fmt.Errorf("spec '%s' is not available in connection '%s': %s",
				specInfo.CspSpecName, resolvedConnectionName, errMsg)
		} else {
			log.Debug().Msgf("Spec validation successful: %s", specInfo.CspSpecName)
			errorChan <- nil
		}
	}()

	// Check image availability in parallel (with auto-registration if found in CSP but not in DB)
	go func() {
		_, isAutoRegistered, err := resource.EnsureImageAvailable(ctx, model.SystemCommonNs, resolvedConnectionName, req.ImageId)
		if err != nil {
			log.Error().Err(err).Msgf("Image validation failed for %s", req.ImageId)
			errorChan <- fmt.Errorf("image '%s' is not available in connection '%s': %w",
				req.ImageId, resolvedConnectionName, err)
		} else {
			if isAutoRegistered {
				log.Info().Msgf("Image '%s' was auto-registered from CSP", req.ImageId)
			}
			log.Debug().Msgf("Image validation successful: %s", req.ImageId)
			errorChan <- nil
		}
	}()

	// Collect errors from both goroutines
	var errorMessages []string
	for range 2 {

		if err := <-errorChan; err != nil {
			errorMessages = append(errorMessages, err.Error())
		}
	}

	// Return combined error if any validation failed
	if len(errorMessages) > 0 {
		combinedError := fmt.Errorf("validation failed for VM '%s': %s",
			req.Name, strings.Join(errorMessages, "; "))
		log.Error().Err(combinedError).Msg("Resource validation failures")
		return combinedError
	}

	log.Debug().Msgf("All resource validations passed for VM: %s", req.Name)
	return nil
}

// waitForVNetReady waits for VNet to be in a ready state with timeout and retry mechanism
func waitForVNetReady(ctx context.Context, nsId string, vNetId string) error {
	reqID := common.RequestIDFromContext(ctx)

	const (
		maxRetries             = 200
		retryInterval          = 5 * time.Second
		progressUpdateInterval = 10 // Update progress every 10 attempts (50 seconds)
	)
	// 1000 Secs

	log.Debug().Msgf("Waiting for VNet '%s' to be ready", vNetId)

	// Initial progress update
	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{
		Title: fmt.Sprintf("Waiting for VNet ready: %s", vNetId),
		Time:  time.Now(),
	})

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Update progress less frequently (only on first attempt and every progressUpdateInterval attempts)
		if attempt == 1 || attempt%progressUpdateInterval == 0 {
			clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{
				Title: fmt.Sprintf("Waiting for VNet ready: %s (attempt %d/%d)", vNetId, attempt, maxRetries),
				Time:  time.Now(),
			})
		}

		// Get VNet info using the dedicated function
		vNetInfo, err := resource.GetVNet(nsId, vNetId)
		if err != nil {
			log.Warn().Err(err).Msgf("Failed to get VNet '%s' on attempt %d", vNetId, attempt)
			time.Sleep(retryInterval)
			continue
		}

		// Check if VNet is ready
		if vNetInfo.Status == model.NetworkStatusAvailable {
			log.Info().Msgf("VNet '%s' is ready with status: %s", vNetId, vNetInfo.Status)
			// Final success progress update
			clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{
				Title: fmt.Sprintf("VNet ready: %s (status: %s)", vNetId, vNetInfo.Status),
				Time:  time.Now(),
			})
			return nil
		}

		// Check for error states
		if strings.Contains(strings.ToLower(vNetInfo.Status), "error") {
			return fmt.Errorf("VNet '%s' is in error state: %s", vNetId, vNetInfo.Status)
		}

		log.Debug().Msgf("VNet '%s' not ready yet, status: %s (attempt %d/%d)", vNetId, vNetInfo.Status, attempt, maxRetries)
		time.Sleep(retryInterval)
	}

	return fmt.Errorf("timeout waiting for VNet '%s' to be ready after %d minutes", vNetId, (maxRetries*int(retryInterval.Seconds()))/60)
}

// getNodeGroupReqFromDynamicReq is func to getNodeGroupReqFromDynamicReq with created resource tracking
// vnetSubnetEnsureMu serializes on-demand subnet creation per VNet so concurrent NodeGroups
// (same Infra, same connection) don't race on subnet CIDR allocation / naming.
var vnetSubnetEnsureMu sync.Map // key: "{nsId}/{vNetId}" -> *sync.Mutex

// nextSubnetCidr derives the next non-overlapping subnet CIDR inside the VNet's CIDR block,
// based on the last existing subnet. Returns an error when the VNet is out of address space.
func nextSubnetCidr(vNetInfo model.VNetInfo) (string, error) {
	if vNetInfo.CidrBlock == "" {
		return "", fmt.Errorf("VNet '%s' has no CIDR block", vNetInfo.Id)
	}
	if len(vNetInfo.SubnetInfoList) == 0 {
		return "", fmt.Errorf("VNet '%s' has no existing subnet to derive the next CIDR from", vNetInfo.Id)
	}
	last := vNetInfo.SubnetInfoList[len(vNetInfo.SubnetInfoList)-1].IPv4_CIDR
	return netutil.NextSubnet(last, vNetInfo.CidrBlock)
}

// specAvailableZonesOrdered returns the zones (in checker order) where the spec is offered, via the
// CSP per-zone availability checker (e.g. AWS DescribeInstanceTypeOfferings). Empty when there is
// no checker or no per-zone data — callers then keep default (first-N) zone behavior.
func specAvailableZonesOrdered(ctx context.Context, specInfo model.SpecInfo) []string {
	avail := cspcheck.CheckAvailability(ctx, model.AvailabilityQuery{
		Provider:         csp.ResolveCloudPlatform(specInfo.ProviderName),
		Region:           specInfo.RegionName,
		InstanceType:     specInfo.CspSpecName,
		AcceleratorModel: specInfo.AcceleratorModel,
		AcceleratorCount: int(specInfo.AcceleratorCount),
	})
	if avail.Source == "none" || len(avail.Zones) == 0 {
		return nil
	}
	zones := make([]string, 0, len(avail.Zones))
	for _, z := range avail.Zones {
		if z.Available {
			zones = append(zones, z.ZoneId)
		}
	}
	return zones
}

// ensureSpecAvailableSubnets makes sure the (shared) VNet has subnets in zones where the spec is
// actually offered, creating them on demand (up to desiredCount spec-available subnets). This lets
// both a freshly created and a reused shared VNet cover the zones a given spec needs — instead of
// only the first-N zones picked at VNet creation, which may not offer the spec (observed failure:
// t2.nano has no subnet in an offering zone).
//
// It returns the set of zones where the spec is offered (empty when unknown). Callers use it to
// select spec-available subnets. It is a safe no-op for:
//   - CSPs without a per-zone availability checker (Source "none") — nothing to base decisions on,
//   - CSPs whose subnets are regional, not zonal (e.g. GCP/Azure) — per-zone subnets don't apply,
//   - IBM/NCP-like CSPs where extra/other-zone subnets aren't supported — CreateSubnet just fails
//     and is skipped (best-effort).
func ensureSpecAvailableSubnets(ctx context.Context, nsId, vNetId string, specInfo model.SpecInfo, desiredCount int) map[string]bool {
	avail := cspcheck.CheckAvailability(ctx, model.AvailabilityQuery{
		Provider:         csp.ResolveCloudPlatform(specInfo.ProviderName),
		Region:           specInfo.RegionName,
		InstanceType:     specInfo.CspSpecName,
		AcceleratorModel: specInfo.AcceleratorModel,
		AcceleratorCount: int(specInfo.AcceleratorCount),
	})
	if avail.Source == "none" || len(avail.Zones) == 0 {
		return nil // no per-zone data; best-effort, nothing to ensure
	}
	availSet := make(map[string]bool, len(avail.Zones))
	availOrder := make([]string, 0, len(avail.Zones))
	for _, z := range avail.Zones {
		if z.Available {
			availSet[z.ZoneId] = true
			availOrder = append(availOrder, z.ZoneId)
		}
	}
	if len(availSet) == 0 {
		return availSet // spec offered in no zone; caller falls back
	}
	if desiredCount < 1 {
		desiredCount = 1
	}

	// Serialize subnet mutations for this VNet.
	lk, _ := vnetSubnetEnsureMu.LoadOrStore(nsId+"/"+vNetId, &sync.Mutex{})
	mu := lk.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	vNetInfo, err := resource.GetVNet(nsId, vNetId)
	if err != nil {
		log.Warn().Err(err).Msgf("ensureSpecAvailableSubnets: cannot read VNet '%s'", vNetId)
		return availSet
	}

	covered := make(map[string]bool)
	zonedSubnets := 0
	for _, s := range vNetInfo.SubnetInfoList {
		if s.Zone != "" {
			covered[s.Zone] = true
			zonedSubnets++
		}
	}
	// Regional-subnet CSP (subnets carry no zone): per-zone subnet coverage doesn't apply.
	if zonedSubnets == 0 && len(vNetInfo.SubnetInfoList) > 0 {
		return availSet
	}

	availCovered := 0
	for z := range covered {
		if availSet[z] {
			availCovered++
		}
	}

	for _, z := range availOrder {
		if availCovered >= desiredCount {
			break
		}
		if covered[z] {
			continue
		}
		newCidr, cErr := nextSubnetCidr(vNetInfo)
		if cErr != nil {
			log.Warn().Err(cErr).Msgf("ensureSpecAvailableSubnets: no CIDR space in VNet '%s' to add subnet for zone '%s'", vNetId, z)
			break
		}
		subReq := &model.SubnetReq{
			Name:      fmt.Sprintf("%s-%02d", vNetId, len(vNetInfo.SubnetInfoList)),
			IPv4_CIDR: newCidr,
			Zone:      z,
		}
		if _, sErr := resource.CreateSubnet(ctx, nsId, vNetId, subReq); sErr != nil {
			log.Warn().Err(sErr).Msgf("ensureSpecAvailableSubnets: could not add subnet in zone '%s' to VNet '%s' (CSP may not support it)", z, vNetId)
			continue
		}
		log.Info().Msgf("ensureSpecAvailableSubnets: added subnet '%s' (zone %s, cidr %s) to VNet '%s' for spec '%s'", subReq.Name, z, newCidr, vNetId, specInfo.Id)
		covered[z] = true
		availCovered++
		if vi, e := resource.GetVNet(nsId, vNetId); e == nil {
			vNetInfo = vi
		}
	}
	return availSet
}

func getNodeGroupReqFromDynamicReq(ctx context.Context, nsId string, infraId string, req *model.CreateNodeGroupDynamicReq) (*NodeReqWithCreatedResources, error) {

	reqID := common.RequestIDFromContext(ctx)
	credentialHolder := common.CredentialHolderFromContext(ctx)

	onDemand := true
	var createdResources []CreatedResource

	nodeRequest := req
	// Check whether VM names meet requirement.
	k := nodeRequest

	nodeGroupReq := &model.CreateNodeGroupReq{}

	specInfo, err := resource.GetSpec(model.SystemCommonNs, req.SpecId)
	if err != nil {
		detailedErr := fmt.Errorf("failed to find VM specification '%s': %w. Please verify the spec exists and is properly configured", req.SpecId, err)
		log.Error().Err(err).Msgf("Spec lookup failed for VM '%s' with SpecId '%s'", req.Name, req.SpecId)
		return &NodeReqWithCreatedResources{VmReq: &model.CreateNodeGroupReq{Name: req.Name}, CreatedResources: createdResources}, detailedErr
	}

	// remake nodeRequest from given input and check resource availability
	// Resolve connection name based on credential holder
	nodeGroupReq.ConnectionName = common.ResolveConnectionName(specInfo.ConnectionName, credentialHolder)

	// If ConnectionName is specified by the request, Use ConnectionName from the request
	if k.ConnectionName != "" {
		nodeGroupReq.ConnectionName = k.ConnectionName
	}

	// validate the GetConnConfig for spec
	connection, err := common.GetConnConfig(nodeGroupReq.ConnectionName)
	if err != nil {
		detailedErr := fmt.Errorf("failed to get connection configuration '%s' for VM '%s' with spec '%s': %w. Please verify the connection exists and is properly configured",
			nodeGroupReq.ConnectionName, req.Name, k.SpecId, err)
		log.Error().Err(err).Msgf("Connection config lookup failed for VM '%s', ConnectionName '%s', Spec '%s'", req.Name, nodeGroupReq.ConnectionName, k.SpecId)
		return &NodeReqWithCreatedResources{VmReq: &model.CreateNodeGroupReq{Name: req.Name, ConnectionName: nodeGroupReq.ConnectionName}, CreatedResources: createdResources}, detailedErr
	}

	// Base shared resource name pattern: nsId + "-shared-" + connectionName [+ "-" + zone]
	baseResourceName := nsId + model.StrSharedResourceName + nodeGroupReq.ConnectionName
	if req.Zone != "" {
		baseResourceName = baseResourceName + "-" + req.Zone
		log.Info().Msgf("Using zone-specific shared resource name: %s (zone: %s) for VM '%s'", baseResourceName, req.Zone, req.Name)
	}

	// VNet resource name: shared per-connection by default, or dedicated per-Infra when the
	// resolved VNet template requests it (vNetPolicy.Dedicated). Centralized in
	// GenVNetResourceName so this name and the one used when actually creating the VNet
	// (and the SG's VNetId reference) always agree.
	vNetResourceName := resource.GenVNetResourceName(nsId, infraId, nodeGroupReq.ConnectionName, req.Zone, req.VNetTemplateId)
	if req.VNetTemplateId != "" {
		log.Info().Msgf("Using template-specific VNet resource name: %s (template: %s) for VM '%s'", vNetResourceName, req.VNetTemplateId, req.Name)
	}

	// SG resource name: dedicated per NodeGroup ("{infraId}-{nodeGroupName}"). Applications are
	// deployed per NodeGroup, so their firewall ports can be opened per NodeGroup without affecting
	// sibling NodeGroups on the same connection. The NodeGroup name is unique within an Infra, so no
	// connection/zone/template suffix is needed. Reclaimed by the unused-resource release operation
	// via its sys.infraId/sys.purpose labels once no VMs reference it. VNet/SSHKey stay per-Infra.
	// NOTE: must match the name computed in resource.CreateSharedResourceWithOptions (NodeGroupName).
	sgResourceName := infraId + "-" + req.Name

	nodeGroupReq.SpecId = specInfo.Id
	nodeGroupReq.ImageId = k.ImageId

	// Check if the image is available (DB or CSP) and auto-register if needed
	imageInfo, isAutoRegistered, err := resource.EnsureImageAvailable(ctx, nsId, connection.ConfigName, nodeGroupReq.ImageId)
	if err != nil {
		detailedErr := fmt.Errorf("failed to find image '%s' for VM '%s' in CSP '%s' (connection: %s): %w. Please verify the image exists and is accessible in the target region",
			nodeGroupReq.ImageId, req.Name, connection.ProviderName, connection.ConfigName, err)
		log.Error().Err(err).Msgf("Image lookup failed for VM '%s', ImageId '%s', Provider '%s', Connection '%s'",
			req.Name, nodeGroupReq.ImageId, connection.ProviderName, connection.ConfigName)
		return &NodeReqWithCreatedResources{VmReq: &model.CreateNodeGroupReq{Name: req.Name, ConnectionName: nodeGroupReq.ConnectionName, ImageId: nodeGroupReq.ImageId}, CreatedResources: createdResources}, detailedErr
	}
	if isAutoRegistered {
		log.Info().Msgf("Image '%s' was auto-registered from CSP for VM '%s'", nodeGroupReq.ImageId, req.Name)
	}
	// Update ImageId with the registered image ID (handles both regular and custom images)
	nodeGroupReq.ImageId = imageInfo.Id
	// Pre-populate CspImageName for non-custom images so per-VM CreateNode calls can skip
	// the redundant GetImage DB query. Custom images go through the full path (empty = fallback).
	if imageInfo.ResourceType != model.StrCustomImage {
		nodeGroupReq.CspImageName = imageInfo.CspImageName
	}

	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Setting vNet:" + vNetResourceName, Time: time.Now()})

	nodeGroupReq.VNetId = vNetResourceName
	_, err = resource.GetResource(nsId, model.StrVNet, nodeGroupReq.VNetId)
	if err != nil {
		if !onDemand {
			detailedErr := fmt.Errorf("failed to get required VNet '%s' for VM '%s' from connection '%s': %w. VNet must exist when onDemand is disabled",
				nodeGroupReq.VNetId, req.Name, nodeGroupReq.ConnectionName, err)
			log.Error().Err(err).Msgf("VNet lookup failed for VM '%s', VNetId '%s', Connection '%s' (onDemand disabled)",
				req.Name, nodeGroupReq.VNetId, nodeGroupReq.ConnectionName)
			return &NodeReqWithCreatedResources{VmReq: &model.CreateNodeGroupReq{Name: req.Name, ConnectionName: nodeGroupReq.ConnectionName, VNetId: nodeGroupReq.VNetId}, CreatedResources: createdResources}, detailedErr
		}
		clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Loading default vNet:" + vNetResourceName, Time: time.Now()})

		// Check if the target vNet (template-specific or base) already exists (e.g. created
		// by a concurrent NodeGroup for the same connection). Using vNetResourceName here
		// ensures we check the exact resource we intend to use, not a legacy ID.
		_, err := resource.GetResource(nsId, model.StrVNet, vNetResourceName)
		log.Debug().Msg("checked if the default vNet does NOT exist")
		// Create a new default vNet if it does not exist
		if err != nil {
			log.Debug().Msg("Not found default vNet: " + err.Error())
			// Pass Zone, CredentialHolder, and template options
			sharedResourceOpts := &resource.SharedResourceOptions{
				CredentialHolder: credentialHolder,
				VNetTemplateId:   req.VNetTemplateId,
				// InfraId enables per-Infra dedicated VNet naming/labeling when the resolved
				// template sets vNetPolicy.Dedicated; ignored (VNet stays shared) otherwise.
				InfraId: infraId,
			}
			if req.Zone != "" {
				sharedResourceOpts.Zone = req.Zone
				log.Info().Msgf("Creating VNet with explicit zone '%s' for VM '%s'", req.Zone, req.Name)
			} else if p := csp.ResolveCloudPlatform(specInfo.ProviderName); p != csp.GCP && p != csp.Azure {
				// Place the initial subnets in the spec's offering zones (layer 1) so we don't create
				// subnets in zones the spec can't use — avoids leaving unused subnets. Restricted to
				// zonal-subnet CSPs; GCP/Azure subnets are regional (no per-zone placement), and CSPs
				// without a per-zone checker return no zones and keep default placement.
				sharedResourceOpts.PreferredZones = specAvailableZonesOrdered(ctx, specInfo)
			}
			err2 := createSharedResourceWithRetry(ctx, nsId, model.StrVNet, nodeGroupReq.ConnectionName, sharedResourceOpts)
			if err2 != nil {
				detailedErr := fmt.Errorf("failed to create default VNet for VM '%s' in namespace '%s' using connection '%s': %w. This may be due to CSP quotas, permissions, or network configuration issues",
					req.Name, nsId, nodeGroupReq.ConnectionName, err2)
				log.Error().Err(err2).Msgf("VNet creation failed for VM '%s', VNetId '%s', Namespace '%s', Connection '%s'",
					req.Name, nodeGroupReq.VNetId, nsId, nodeGroupReq.ConnectionName)
				return &NodeReqWithCreatedResources{VmReq: &model.CreateNodeGroupReq{Name: req.Name, ConnectionName: nodeGroupReq.ConnectionName, VNetId: nodeGroupReq.VNetId}, CreatedResources: createdResources}, detailedErr
			} else {
				log.Info().Msg("Created new default vNet: " + nodeGroupReq.VNetId)
				// Track the newly created VNet
				createdResources = append(createdResources, CreatedResource{Type: model.StrVNet, Id: nodeGroupReq.VNetId})
			}
		}
		// Wait for the VNet to be ready after creation
		err = waitForVNetReady(ctx, nsId, nodeGroupReq.VNetId)
		if err != nil {
			detailedErr := fmt.Errorf("VNet '%s' is not ready for use after creation: %w", nodeGroupReq.VNetId, err)
			log.Error().Err(err).Msgf("VNet ready check failed for VM '%s', VNetId '%s'", req.Name, nodeGroupReq.VNetId)
			return &NodeReqWithCreatedResources{VmReq: &model.CreateNodeGroupReq{Name: req.Name, ConnectionName: nodeGroupReq.ConnectionName, VNetId: nodeGroupReq.VNetId}, CreatedResources: createdResources}, detailedErr
		}
	} else {
		log.Info().Msg("Found and utilize default vNet: " + nodeGroupReq.VNetId)

		// Fail fast if the vNet was deleted out-of-band on the CSP.
		if exists, indet := resource.VerifySharedResourceOnCsp(nsId, model.StrVNet, vNetResourceName); indet == nil && !exists {
			detailedErr := fmt.Errorf("vNet '%s' is recorded in Tumblebug but missing on the CSP (deleted out-of-band?). "+
				"Clear the stale record with DELETE /ns/%s/deregisterResource/vNet/%s?withSubnets=true and retry; it will be recreated on demand",
				vNetResourceName, nsId, vNetResourceName)
			log.Error().Err(detailedErr).Msgf("VNet drift detected for VM '%s', Connection '%s'", req.Name, nodeGroupReq.ConnectionName)
			return &NodeReqWithCreatedResources{VmReq: &model.CreateNodeGroupReq{Name: req.Name, ConnectionName: nodeGroupReq.ConnectionName, VNetId: nodeGroupReq.VNetId}, CreatedResources: createdResources}, detailedErr
		}

		// Even if VNet exists, ensure it's ready for use
		vNetInfo, err := resource.GetVNet(nsId, nodeGroupReq.VNetId)
		if err != nil {
			detailedErr := fmt.Errorf("failed to get VNet info for '%s': %w", nodeGroupReq.VNetId, err)
			log.Error().Err(err).Msg(detailedErr.Error())
			return &NodeReqWithCreatedResources{VmReq: &model.CreateNodeGroupReq{Name: req.Name, ConnectionName: nodeGroupReq.ConnectionName, VNetId: nodeGroupReq.VNetId}, CreatedResources: createdResources}, detailedErr
		}

		// Check if VNet is ready, if not wait for it
		if vNetInfo.Status != model.NetworkStatusAvailable {
			log.Info().Msgf("VNet '%s' exists but not ready (status: %s), waiting for ready state", nodeGroupReq.VNetId, vNetInfo.Status)
			err = waitForVNetReady(ctx, nsId, nodeGroupReq.VNetId)
			if err != nil {
				detailedErr := fmt.Errorf("existing VNet '%s' is not ready for use: %w", nodeGroupReq.VNetId, err)
				log.Error().Err(err).Msgf("VNet ready check failed for VM '%s', VNetId '%s'", req.Name, nodeGroupReq.VNetId)
				return &NodeReqWithCreatedResources{VmReq: &model.CreateNodeGroupReq{Name: req.Name, ConnectionName: nodeGroupReq.ConnectionName, VNetId: nodeGroupReq.VNetId}, CreatedResources: createdResources}, detailedErr
			}
		}
	}

	// Select subnet based on user-specified zone or VNet template.
	// - Zone specified: find a subnet matching that zone via FindSubnetByZone
	// - Template used (no zone): look up VNet to get first subnet's actual ID
	//   (template subnets may have custom names, not matching vNetResourceName)
	// - Default (no zone, no template): subnet has same name as VNet (hard-coded convention)
	if req.Zone != "" {
		subnetId, subnetZone, err := resource.FindSubnetByZone(nsId, nodeGroupReq.VNetId, req.Zone)
		if err != nil {
			log.Warn().Err(err).Msgf("Failed to find subnet by zone '%s', using default subnet", req.Zone)
			nodeGroupReq.SubnetId = vNetResourceName
		} else {
			nodeGroupReq.SubnetId = subnetId
			log.Info().Msgf("Selected subnet '%s' (zone: '%s') for VM '%s' based on requested zone '%s'",
				subnetId, subnetZone, req.Name, req.Zone)
		}
	} else if req.DistributeSubnets {
		// Distribute this NodeGroup's VMs across subnets in zones where the spec is offered.
		// Step 1: ensure such subnets exist — creating them on demand so a fresh or reused shared
		// VNet covers the spec's offering zones (not just the first-N zones chosen at VNet creation).
		// Step 2: round-robin VMs across the spec-available subnets. availSet is the CSP's per-AZ
		// offering set (empty when unknown → best-effort: use all subnets).
		availSet := ensureSpecAvailableSubnets(ctx, nsId, nodeGroupReq.VNetId, specInfo, 2)
		vNetInfo, vnetErr := resource.GetVNet(nsId, nodeGroupReq.VNetId)
		if vnetErr != nil || len(vNetInfo.SubnetInfoList) == 0 {
			log.Warn().Err(vnetErr).Msgf("DistributeSubnets: could not read subnets of VNet '%s'; using VNet name as SubnetId", nodeGroupReq.VNetId)
			nodeGroupReq.SubnetId = vNetResourceName
		} else {
			candidates := vNetInfo.SubnetInfoList
			if len(availSet) > 0 {
				var filtered []model.SubnetInfo
				for _, s := range vNetInfo.SubnetInfoList {
					if s.Zone == "" || availSet[s.Zone] {
						filtered = append(filtered, s)
					}
				}
				if len(filtered) > 0 {
					candidates = filtered
				}
			}
			subnetIds := make([]string, 0, len(candidates))
			zonesForLog := make([]string, 0, len(candidates))
			for _, s := range candidates {
				subnetIds = append(subnetIds, s.Id)
				zonesForLog = append(zonesForLog, s.Zone)
			}
			nodeGroupReq.SubnetIds = subnetIds
			nodeGroupReq.SubnetId = subnetIds[0]
			log.Info().Msgf("DistributeSubnets: NodeGroup '%s' VMs will spread across %d subnet(s) %v (zones %v) of VNet '%s'",
				req.Name, len(subnetIds), subnetIds, zonesForLog, nodeGroupReq.VNetId)
		}
	} else if req.VNetTemplateId != "" {
		// Template-based VNet: subnets have custom names defined in the template.
		// Look up the VNet to find a subnet. When multiple subnets exist (e.g. multiZone),
		// distribute VMs across subnets using the NodeGroup name as a hash key so placement
		// is deterministic but not always concentrated on the first subnet.
		vNetInfo, err := resource.GetVNet(nsId, nodeGroupReq.VNetId)
		if err == nil && len(vNetInfo.SubnetInfoList) > 0 {
			subnetCount := len(vNetInfo.SubnetInfoList)
			subnetIdx := 0
			if subnetCount > 1 {
				// Simple hash over NodeGroup name bytes for deterministic distribution
				var nameHash int
				for _, c := range req.Name {
					nameHash += int(c)
				}
				subnetIdx = nameHash % subnetCount
			}
			selectedSubnet := vNetInfo.SubnetInfoList[subnetIdx]
			nodeGroupReq.SubnetId = selectedSubnet.Id
			log.Info().Msgf("Selected subnet [%d/%d] '%s' (zone: '%s') from template-based VNet '%s' for VM '%s'",
				subnetIdx+1, subnetCount, selectedSubnet.Id, selectedSubnet.Zone, nodeGroupReq.VNetId, req.Name)
		} else {
			log.Warn().Msgf("Could not retrieve subnets from template-based VNet '%s', falling back to VNet name as SubnetId", nodeGroupReq.VNetId)
			nodeGroupReq.SubnetId = vNetResourceName
		}
	} else {
		// Default: place all Nodes in a subnet whose zone offers the spec. Ensure at least one such
		// subnet exists (creating it on demand for a fresh/reused shared VNet), then pick it —
		// keeping the base subnet when it already offers the spec (the common case). Falls back to
		// the base subnet when per-zone data is unknown.
		nodeGroupReq.SubnetId = vNetResourceName
		if availSet := ensureSpecAvailableSubnets(ctx, nsId, nodeGroupReq.VNetId, specInfo, 1); len(availSet) > 0 {
			if vNetInfo, err := resource.GetVNet(nsId, nodeGroupReq.VNetId); err == nil {
				baseOk := false
				firstAvail := ""
				for _, s := range vNetInfo.SubnetInfoList {
					if s.Zone != "" && !availSet[s.Zone] {
						continue // subnet's zone does not offer the spec
					}
					if firstAvail == "" {
						firstAvail = s.Id
					}
					if s.Id == vNetResourceName {
						baseOk = true
					}
				}
				if !baseOk && firstAvail != "" {
					nodeGroupReq.SubnetId = firstAvail
					log.Info().Msgf("Default placement: base subnet not in a spec-available zone; using '%s' for NodeGroup '%s'", firstAvail, req.Name)
				}
			}
		}
	}

	// SSHKey is dedicated per Infra ("{infraId}-{conn}[-zone]") for credential isolation, so a
	// compromised key is scoped to a single Infra instead of every Infra on the connection.
	sshKeyResourceName := infraId + "-" + nodeGroupReq.ConnectionName
	if req.Zone != "" {
		sshKeyResourceName = sshKeyResourceName + "-" + req.Zone
	}
	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Setting SSHKey:" + sshKeyResourceName, Time: time.Now()})
	nodeGroupReq.SshKeyId = sshKeyResourceName
	_, err = resource.GetResource(nsId, model.StrSSHKey, nodeGroupReq.SshKeyId)
	if err != nil {
		if !onDemand {
			detailedErr := fmt.Errorf("failed to get required SSHKey '%s' for VM '%s' from connection '%s': %w. SSHKey must exist when onDemand is disabled",
				nodeGroupReq.SshKeyId, req.Name, nodeGroupReq.ConnectionName, err)
			log.Error().Err(err).Msgf("SSHKey lookup failed for VM '%s', SshKeyId '%s', Connection '%s' (onDemand disabled)",
				req.Name, nodeGroupReq.SshKeyId, nodeGroupReq.ConnectionName)
			return &NodeReqWithCreatedResources{VmReq: &model.CreateNodeGroupReq{Name: req.Name, ConnectionName: nodeGroupReq.ConnectionName, SshKeyId: nodeGroupReq.SshKeyId}, CreatedResources: createdResources}, detailedErr
		}
		clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Loading default SSHKey:" + sshKeyResourceName, Time: time.Now()})

		// Check if the dedicated SSHKey already exists (e.g. created by a concurrent
		// NodeGroup of the same Infra+connection); create it only if it does not.
		_, err := resource.GetResource(nsId, model.StrSSHKey, sshKeyResourceName)
		log.Debug().Msg("checked if the dedicated SSHKey does NOT exist")
		// Create a new dedicated SSHKey if it does not exist
		if err != nil {
			log.Debug().Msg("Not found dedicated SSHKey: " + err.Error())
			// Pass Zone, CredentialHolder, and InfraId (dedicated per-Infra) options. No template support.
			sharedResourceOpts := &resource.SharedResourceOptions{
				CredentialHolder: credentialHolder,
				InfraId:          infraId,
			}
			if req.Zone != "" {
				sharedResourceOpts.Zone = req.Zone
				log.Info().Msgf("Creating SSHKey with explicit zone '%s' for VM '%s'", req.Zone, req.Name)
			}
			err2 := createSharedResourceWithRetry(ctx, nsId, model.StrSSHKey, nodeGroupReq.ConnectionName, sharedResourceOpts)
			if err2 != nil {
				detailedErr := fmt.Errorf("failed to create default SSHKey for VM '%s' in namespace '%s' using connection '%s': %w. This may be due to CSP quotas, permissions, or key generation issues",
					req.Name, nsId, nodeGroupReq.ConnectionName, err2)
				log.Error().Err(err2).Msgf("SSHKey creation failed for VM '%s', SshKeyId '%s', Namespace '%s', Connection '%s'",
					req.Name, nodeGroupReq.SshKeyId, nsId, nodeGroupReq.ConnectionName)
				return &NodeReqWithCreatedResources{VmReq: &model.CreateNodeGroupReq{Name: req.Name, ConnectionName: nodeGroupReq.ConnectionName, SshKeyId: nodeGroupReq.SshKeyId}, CreatedResources: createdResources}, detailedErr
			} else {
				log.Info().Msg("Created new default SSHKey: " + nodeGroupReq.SshKeyId)
				// Track the newly created SSHKey
				createdResources = append(createdResources, CreatedResource{Type: model.StrSSHKey, Id: nodeGroupReq.SshKeyId})
			}
		}
	} else {
		log.Info().Msg("Found and utilize default SSHKey: " + nodeGroupReq.SshKeyId)

		// If the keypair was deleted out-of-band on the CSP, replace the stale record
		// (rotates key material; existing VMs keep the old key).
		if exists, indet := resource.VerifySharedResourceOnCsp(nsId, model.StrSSHKey, sshKeyResourceName); indet == nil && !exists {
			log.Warn().Msgf("SSHKey drift detected for '%s'; deregistering stale record and recreating", sshKeyResourceName)
			if derr := resource.DeregisterResource(nsId, model.StrSSHKey, sshKeyResourceName); derr != nil {
				log.Warn().Err(derr).Msgf("failed to deregister drifted SSHKey '%s'", sshKeyResourceName)
			}
			sharedResourceOpts := &resource.SharedResourceOptions{
				CredentialHolder: credentialHolder,
				InfraId:          infraId,
			}
			if req.Zone != "" {
				sharedResourceOpts.Zone = req.Zone
			}
			if err2 := createSharedResourceWithRetry(ctx, nsId, model.StrSSHKey, nodeGroupReq.ConnectionName, sharedResourceOpts); err2 != nil {
				detailedErr := fmt.Errorf("failed to recreate drifted SSHKey '%s' on connection '%s': %w", sshKeyResourceName, nodeGroupReq.ConnectionName, err2)
				log.Error().Err(err2).Msgf("SSHKey recreation failed for VM '%s'", req.Name)
				return &NodeReqWithCreatedResources{VmReq: &model.CreateNodeGroupReq{Name: req.Name, ConnectionName: nodeGroupReq.ConnectionName, SshKeyId: nodeGroupReq.SshKeyId}, CreatedResources: createdResources}, detailedErr
			}
			createdResources = append(createdResources, CreatedResource{Type: model.StrSSHKey, Id: sshKeyResourceName})
			log.Info().Msg("Recreated drifted SSHKey: " + sshKeyResourceName)
		}
	}

	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Setting securityGroup:" + sgResourceName, Time: time.Now()})
	securityGroup := sgResourceName
	nodeGroupReq.SecurityGroupIds = append(nodeGroupReq.SecurityGroupIds, securityGroup)
	_, err = resource.GetResource(nsId, model.StrSecurityGroup, securityGroup)
	if err != nil {
		if !onDemand {
			detailedErr := fmt.Errorf("failed to get required SecurityGroup '%s' for VM '%s' from connection '%s': %w. SecurityGroup must exist when onDemand is disabled",
				securityGroup, req.Name, nodeGroupReq.ConnectionName, err)
			log.Error().Err(err).Msgf("SecurityGroup lookup failed for VM '%s', SecurityGroup '%s', Connection '%s' (onDemand disabled)",
				req.Name, securityGroup, nodeGroupReq.ConnectionName)
			return &NodeReqWithCreatedResources{VmReq: &model.CreateNodeGroupReq{Name: req.Name, ConnectionName: nodeGroupReq.ConnectionName, SecurityGroupIds: []string{securityGroup}}, CreatedResources: createdResources}, detailedErr
		}
		clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Loading default securityGroup:" + sgResourceName, Time: time.Now()})

		// Check if the target SecurityGroup (template-specific or base) already exists.
		// Using sgResourceName ensures we check the exact resource we intend to use.
		_, err := resource.GetResource(nsId, model.StrSecurityGroup, sgResourceName)
		// Create a new default security group if it does not exist
		log.Debug().Msg("checked if the default security group does NOT exist")
		if err != nil {
			log.Debug().Msg("Not found default security group: " + err.Error())
			// Pass Zone, CredentialHolder, and template options
			// VNetTemplateId is needed so the SG's VNetId points to the template-specific VNet name
			sharedResourceOpts := &resource.SharedResourceOptions{
				CredentialHolder: credentialHolder,
				VNetTemplateId:   req.VNetTemplateId,
				SgTemplateId:     req.SgTemplateId,
				InfraId:          infraId,
				// Per-NodeGroup SG naming ("{infraId}-{nodeGroupName}"): must agree with
				// sgResourceName computed above so the created resource matches the referenced ID.
				NodeGroupName: req.Name,
			}
			if req.Zone != "" {
				sharedResourceOpts.Zone = req.Zone
				log.Info().Msgf("Creating SecurityGroup with explicit zone '%s' for VM '%s'", req.Zone, req.Name)
			}
			err2 := createSharedResourceWithRetry(ctx, nsId, model.StrSecurityGroup, nodeGroupReq.ConnectionName, sharedResourceOpts)
			if err2 != nil {
				detailedErr := fmt.Errorf("failed to create default SecurityGroup for VM '%s' in namespace '%s' using connection '%s': %w. This may be due to CSP quotas, permissions, or firewall rule configuration issues",
					req.Name, nsId, nodeGroupReq.ConnectionName, err2)
				log.Error().Err(err2).Msgf("SecurityGroup creation failed for VM '%s', SecurityGroup '%s', Namespace '%s', Connection '%s'",
					req.Name, securityGroup, nsId, nodeGroupReq.ConnectionName)
				return &NodeReqWithCreatedResources{VmReq: &model.CreateNodeGroupReq{Name: req.Name, ConnectionName: nodeGroupReq.ConnectionName, SecurityGroupIds: []string{securityGroup}}, CreatedResources: createdResources}, detailedErr
			} else {
				log.Info().Msg("Created new default securityGroup: " + securityGroup)
				// Track the newly created SecurityGroup
				createdResources = append(createdResources, CreatedResource{Type: model.StrSecurityGroup, Id: securityGroup})
			}
		}
	} else {
		log.Info().Msg("Found and utilize default securityGroup: " + securityGroup)

		// Fail fast if the securityGroup was deleted out-of-band on the CSP.
		if exists, indet := resource.VerifySharedResourceOnCsp(nsId, model.StrSecurityGroup, sgResourceName); indet == nil && !exists {
			detailedErr := fmt.Errorf("securityGroup '%s' is recorded in Tumblebug but missing on the CSP (deleted out-of-band?). "+
				"Clear the stale record with DELETE /ns/%s/deregisterResource/securityGroup/%s and retry; it will be recreated on demand",
				sgResourceName, nsId, sgResourceName)
			log.Error().Err(detailedErr).Msgf("SecurityGroup drift detected for VM '%s', Connection '%s'", req.Name, nodeGroupReq.ConnectionName)
			return &NodeReqWithCreatedResources{VmReq: &model.CreateNodeGroupReq{Name: req.Name, ConnectionName: nodeGroupReq.ConnectionName, SecurityGroupIds: []string{securityGroup}}, CreatedResources: createdResources}, detailedErr
		}
	}

	nodeGroupReq.Name = k.Name
	if nodeGroupReq.Name == "" {
		nodeGroupReq.Name = common.GenUid()
	}
	nodeGroupReq.Label = k.Label
	nodeGroupReq.NodeGroupSize = k.NodeGroupSize
	nodeGroupReq.Description = k.Description
	nodeGroupReq.RootDiskType = k.RootDiskType
	nodeGroupReq.RootDiskSize = k.RootDiskSize
	// NodeUserPassword is not taken from the request; CreateNode generates a random
	// password internally for the CSP-side requirement (Windows).

	common.PrintJsonPretty(nodeGroupReq)
	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Prepared resources for VM:" + nodeGroupReq.Name, Info: nodeGroupReq, Time: time.Now()})

	return &NodeReqWithCreatedResources{VmReq: nodeGroupReq, CreatedResources: createdResources}, nil
}

// CreateNodeObject is func to add VM to Infra
func CreateNodeObject(wg *sync.WaitGroup, nsId string, infraId string, nodeInfoData *model.NodeInfo) error {
	log.Debug().Msg("Start to add VM To Infra")
	//goroutin
	defer wg.Done()

	key := common.GenInfraKey(nsId, infraId, "")
	_, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Fatal().Err(err).Msg("AddNodeToInfra kvstore.GetKv() returned an error.")
		return err
	}
	if !exists {
		return fmt.Errorf("AddNodeToInfra Cannot find infraId. Key: %s", key)
	}

	// Overwriting a Node record loses the identity of its CSP resource, which keeps
	// running and billing untracked (issue #2652)
	nodeKey := common.GenInfraKey(nsId, infraId, nodeInfoData.Id)
	if _, nodeExists, getErr := kvstore.GetKv(nodeKey); getErr == nil && nodeExists {
		return fmt.Errorf("Node %s already exists in Infra %s; refusing to overwrite its record", nodeInfoData.Id, infraId)
	}

	configTmp, err := common.GetConnConfig(nodeInfoData.ConnectionName)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	nodeInfoData.Location = configTmp.RegionDetail.Location

	// Store auxiliary details under a separate key; keep them out of the Node
	// record so status/bulk reads stay small. nodeInfoData is a pointer, so store
	// a stripped copy rather than mutating the caller's object.
	if len(nodeInfoData.AddtionalDetails) > 0 {
		putNodeDetails(nsId, infraId, nodeInfoData.Id, nodeInfoData.AddtionalDetails)
	}
	nodeToStore := *nodeInfoData
	nodeToStore.AddtionalDetails = nil

	// Make VM object
	val, _ := json.Marshal(nodeToStore)
	err = kvstore.Put(nodeKey, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	return nil
}

// NodeCreateInfo represents Node creation information with grouping details
type NodeCreateInfo struct {
	NodeInfo     *model.NodeInfo
	ProviderName string
	RegionName   string
}

type ctxKeySkipAssoc struct{}

// CreateNodesInParallel creates VMs with hierarchical rate limiting
// Level 1: CSPs are processed in parallel
// Level 2: Within each CSP, regions are processed with semaphore (maxConcurrentRegionsPerCSP)
// Level 3: Within each region, VMs are processed with semaphore (maxConcurrentNodesPerRegion)
func CreateNodesInParallel(ctx context.Context, nsId, infraId string, nodeInfoList []*model.NodeInfo, option string) error {
	if len(nodeInfoList) == 0 {
		return nil
	}

	// Tell child CreateNode calls to skip individual association updates so we can
	// batch them efficiently at the end, eliminating tens of thousands of serial RMW loops.
	ctx = context.WithValue(ctx, ctxKeySkipAssoc{}, true)

	// Step 1: Group VMs by CSP and region
	nodeGroups := make(map[string]map[string][]*model.NodeInfo) // CSP -> Region -> NodeInfos
	nodeGroupInfos := make(map[string]NodeCreateInfo)           // NodeId -> CreateInfo

	for _, nodeInfo := range nodeInfoList {
		providerName := nodeInfo.ConnectionConfig.ProviderName
		// nodeInfo.Region is filled from Spider's create response, so it is empty here; key by
		// the connection's region or every node of a CSP lands in one "" bucket (one region's
		// concurrency limit for all regions, and one region's quota error cancels all of them).
		regionName := nodeInfo.ConnectionConfig.RegionDetail.RegionName
		if regionName == "" {
			regionName = nodeInfo.Region.Region
		}
		if regionName == "" {
			regionName = nodeInfo.ConnectionName
		}

		// Initialize CSP map if not exists
		if nodeGroups[providerName] == nil {
			nodeGroups[providerName] = make(map[string][]*model.NodeInfo)
		}

		// Add VM to the appropriate group
		nodeGroups[providerName][regionName] = append(nodeGroups[providerName][regionName], nodeInfo)
		nodeGroupInfos[nodeInfo.Id] = NodeCreateInfo{
			NodeInfo:     nodeInfo,
			ProviderName: providerName,
			RegionName:   regionName,
		}
	}

	// Step 2: Process CSPs in parallel
	var wg sync.WaitGroup
	var mutex sync.Mutex
	var allErrors []error

	for csp, regions := range nodeGroups {
		wg.Add(1)
		go func(providerName string, regionMap map[string][]*model.NodeInfo) {
			defer wg.Done()

			// Get rate limits for this specific CSP
			maxRegionsForCSP, maxNodesForRegion := getNodeCreateRateLimitsForCSP(providerName)

			log.Debug().Msgf("Creating VMs for CSP: %s with %d regions (limits: %d regions, %d VMs/region)",
				providerName, len(regionMap), maxRegionsForCSP, maxNodesForRegion)

			// Step 3: Process regions within CSP with rate limiting
			regionSemaphore := make(chan struct{}, maxRegionsForCSP)
			var regionWg sync.WaitGroup
			var regionMutex sync.Mutex
			var cspErrors []error

			for region, nodeInfos := range regionMap {
				regionWg.Add(1)
				go func(regionName string, nodeInfoList []*model.NodeInfo) {
					defer regionWg.Done()

					// Acquire region semaphore
					regionSemaphore <- struct{}{}
					defer func() { <-regionSemaphore }()

					log.Debug().Msgf("Creating VMs in region: %s/%s with %d VMs (limit: %d VMs/region)",
						providerName, regionName, len(nodeInfoList), maxNodesForRegion)

					// Step 4: Process VMs within region with rate limiting.
					// regionCtx is cancelled on the first quota/capacity error so that
					// goroutines still waiting for a semaphore slot exit early.
					regionCtx, cancelRegion := context.WithCancel(ctx)
					defer cancelRegion()

					var quotaOnce sync.Once
					var quotaErrMsg string
					skipReason := func() string {
						if quotaErrMsg != "" {
							return fmt.Sprintf("region quota/capacity exhausted (%s)", quotaErrMsg)
						}
						return fmt.Sprintf("creation cancelled (%v)", ctx.Err())
					}

					nodeSemaphore := make(chan struct{}, maxNodesForRegion)
					var nodeWg sync.WaitGroup
					var nodeMutex sync.Mutex
					var regionErrors []error

					for _, nodeInfo := range nodeInfoList {
						nodeWg.Add(1)
						go func(nodeInfo *model.NodeInfo) {
							defer nodeWg.Done()

							// Acquire VM semaphore — or bail out if region was quota-cancelled.
							select {
							case nodeSemaphore <- struct{}{}:
							case <-regionCtx.Done():
								reason := skipReason()
								nodeInfoData := *nodeInfo
								nodeInfoData.Status = model.StatusFailed
								nodeInfoData.TargetAction = model.ActionComplete
								nodeInfoData.TargetStatus = ""
								nodeInfoData.SystemMessage = "VM creation skipped: " + reason
								UpdateNodeInfo(nsId, infraId, nodeInfoData)
								log.Warn().Msgf("[CreateNode] VM %s skipped in %s/%s: %s", nodeInfo.Name, providerName, regionName, reason)
								nodeMutex.Lock()
								regionErrors = append(regionErrors, fmt.Errorf("VM %s: creation skipped: %s", nodeInfo.Name, reason))
								nodeMutex.Unlock()
								return
							}
							defer func() { <-nodeSemaphore }()

							// Re-check after acquiring slot (another goroutine may have just cancelled).
							if regionCtx.Err() != nil {
								reason := skipReason()
								nodeInfoData := *nodeInfo
								nodeInfoData.Status = model.StatusFailed
								nodeInfoData.TargetAction = model.ActionComplete
								nodeInfoData.TargetStatus = ""
								nodeInfoData.SystemMessage = "VM creation skipped: " + reason
								UpdateNodeInfo(nsId, infraId, nodeInfoData)
								log.Warn().Msgf("[CreateNode] VM %s skipped in %s/%s: %s", nodeInfo.Name, providerName, regionName, reason)
								nodeMutex.Lock()
								regionErrors = append(regionErrors, fmt.Errorf("VM %s: creation skipped: %s", nodeInfo.Name, reason))
								nodeMutex.Unlock()
								return
							}

							// Create VM using the existing CreateNode function
							var createWg sync.WaitGroup
							createWg.Add(1)
							err := CreateNode(regionCtx, &createWg, nsId, infraId, nodeInfo, option)
							if err != nil {
								log.Error().Err(err).Msgf("Failed to create VM %s", nodeInfo.Name)
								nodeMutex.Lock()
								regionErrors = append(regionErrors, fmt.Errorf("VM %s: %w", nodeInfo.Name, err))
								nodeMutex.Unlock()

								if isQuotaOrCapacityError(err) {
									quotaOnce.Do(func() {
										quotaErrMsg = err.Error()
										cancelRegion()
										log.Warn().Msgf("[CreateNode] Quota/capacity error in %s/%s; cancelling remaining VM creation in this region",
											providerName, regionName)
									})
								}
							}

						}(nodeInfo)
					}
					nodeWg.Wait()

					// Merge region errors to CSP errors
					if len(regionErrors) > 0 {
						regionMutex.Lock()
						cspErrors = append(cspErrors, regionErrors...)
						regionMutex.Unlock()
					}

				}(region, nodeInfos)
			}
			regionWg.Wait()

			// Merge CSP errors to global errors
			if len(cspErrors) > 0 {
				mutex.Lock()
				allErrors = append(allErrors, cspErrors...)
				mutex.Unlock()
			}

			log.Debug().Msgf("Completed VM creation for CSP: %s", providerName)

		}(csp, regions)
	}

	wg.Wait()

	// Batch update associated object lists for all successfully created nodes.
	// This collapses tens of thousands of serial RMW etcd transactions on shared
	// keys (VNet, SSHKey, Image, SecurityGroups) into one batch update per resource.
	type assocKey struct {
		resourceType string
		resourceId   string
	}
	assocMap := make(map[assocKey][]string)
	for _, nodeInfo := range nodeInfoList {
		if nodeInfo == nil || strings.EqualFold(nodeInfo.Status, model.StatusFailed) {
			continue
		}
		nodeKey := common.GenInfraKey(nsId, infraId, nodeInfo.Id)
		if nodeInfo.ImageId != "" {
			imgType := model.StrImage
			if _, exists, _ := kvstore.Get(common.GenResourceKey(nsId, model.StrCustomImage, nodeInfo.ImageId)); exists {
				imgType = model.StrCustomImage
			}
			k := assocKey{resourceType: imgType, resourceId: nodeInfo.ImageId}
			assocMap[k] = append(assocMap[k], nodeKey)
		}
		if nodeInfo.SshKeyId != "" {
			k := assocKey{resourceType: model.StrSSHKey, resourceId: nodeInfo.SshKeyId}
			assocMap[k] = append(assocMap[k], nodeKey)
		}
		if nodeInfo.VNetId != "" {
			k := assocKey{resourceType: model.StrVNet, resourceId: nodeInfo.VNetId}
			assocMap[k] = append(assocMap[k], nodeKey)
		}
		for _, sgId := range nodeInfo.SecurityGroupIds {
			if sgId != "" {
				k := assocKey{resourceType: model.StrSecurityGroup, resourceId: sgId}
				assocMap[k] = append(assocMap[k], nodeKey)
			}
		}
		for _, diskId := range nodeInfo.DataDiskIds {
			if diskId != "" {
				k := assocKey{resourceType: model.StrDataDisk, resourceId: diskId}
				assocMap[k] = append(assocMap[k], nodeKey)
			}
		}
	}

	for k, keys := range assocMap {
		if err := resource.BatchAddToAssociatedObjectList(nsId, k.resourceType, k.resourceId, keys); err != nil {
			log.Warn().Err(err).Msgf("Failed to batch update associated object list for %s %s", k.resourceType, k.resourceId)
		}
	}

	// Summary logging
	cspCount := len(nodeGroups)
	totalRegions := 0
	for _, regions := range nodeGroups {
		totalRegions += len(regions)
	}

	if len(allErrors) > 0 {
		log.Warn().Msgf("Rate-limited VM creation completed with errors: %d CSPs, %d regions, %d VMs total, %d errors",
			cspCount, totalRegions, len(nodeInfoList), len(allErrors))
		// Don't return error for partial failures - let the caller handle individual VM status checks
		// Return first error for compatibility only if ALL VMs failed
		if len(allErrors) >= len(nodeInfoList) {
			return allErrors[0]
		}
		log.Info().Msgf("Partial VM creation success: %d out of %d VMs may have failed, but continuing",
			len(allErrors), len(nodeInfoList))
	}

	log.Debug().Msgf("Rate-limited VM creation completed successfully: %d CSPs, %d regions, %d VMs processed",
		cspCount, totalRegions, len(nodeInfoList))
	return nil
}

// CreateNode is func to create VM (option = "register" for register existing VM)
func CreateNode(ctx context.Context, wg *sync.WaitGroup, nsId string, infraId string, nodeInfoData *model.NodeInfo, option string) error {
	log.Info().Msgf("Start to create VM: %s", nodeInfoData.Name)
	//goroutin
	defer wg.Done()

	var err error = nil
	switch {
	case nodeInfoData.Name == "":
		err = fmt.Errorf("nodeInfoData.Name is empty")
	case nodeInfoData.ImageId == "":
		err = fmt.Errorf("nodeInfoData.ImageId is empty")
	case nodeInfoData.ConnectionName == "":
		err = fmt.Errorf("nodeInfoData.ConnectionName is empty")
	case nodeInfoData.SshKeyId == "":
		err = fmt.Errorf("nodeInfoData.SshKeyId is empty")
	case nodeInfoData.SpecId == "":
		err = fmt.Errorf("nodeInfoData.SpecId is empty")
	case nodeInfoData.SecurityGroupIds == nil:
		err = fmt.Errorf("nodeInfoData.SecurityGroupIds is empty")
	case nodeInfoData.VNetId == "":
		err = fmt.Errorf("nodeInfoData.VNetId is empty")
	case nodeInfoData.SubnetId == "":
		err = fmt.Errorf("nodeInfoData.SubnetId is empty")
	default:
	}
	if err != nil {
		nodeInfoData.Status = model.StatusFailed
		nodeInfoData.SystemMessage = err.Error()
		UpdateNodeInfo(nsId, infraId, *nodeInfoData)
		log.Error().Err(err).Msg("")
		return err
	}

	nodeKey := common.GenInfraKey(nsId, infraId, nodeInfoData.Id)

	// Seed the status store with the node's known static config (location, spec,
	// network, …) before locking, so the list/map view can place and label the node
	// while it is Creating — before the first CSP poll. Without this the store entry
	// created by AcquireLock carries only status/ids, so the node arrives at the map
	// with an empty location and is mis-rendered as location-less. AcquireLock then
	// overlays the operation lock; a later poll refreshes the dynamic fields.
	globalStatusStore.Set(nsId, infraId, nodeInfoData.Id, buildStatusEntry(nsId, infraId, *nodeInfoData))

	// Acquire operation lock so NodeStatusAgent skips polling while Spider POST /vm blocks.
	// The lock is always released on return (defer), guaranteeing cleanup on error paths.
	GlobalAgent.AcquireLock(nsId, infraId, nodeInfoData.Id, model.StatusCreating, model.ActionCreate)
	defer GlobalAgent.ReleaseLock(nsId, infraId, nodeInfoData.Id)

	// On any failure exit, sync the seeded store entry to Failed. The many failure
	// paths below write Failed to the KV record but not to the status store, which
	// now serves the list/status views; without this the node stays stuck at
	// Creating (with a Create target) even though provisioning failed. The success
	// path updates the store live via FetchNodeStatus, so this only acts on failure.
	defer func() {
		if strings.EqualFold(nodeInfoData.Status, model.StatusFailed) {
			globalStatusStore.Update(nsId, infraId, nodeInfoData.Id, func(e *StatusEntry) {
				e.Status = model.StatusFailed
				e.TargetStatus = model.StatusFailed
				e.TargetAction = model.ActionComplete
				e.Priority = PollSkip
				e.SystemMessage = nodeInfoData.SystemMessage
			})
		}
	}()

	// in case of registering existing CSP VM
	if option == "register" {
		// CspResourceId is required
		if nodeInfoData.CspResourceId == "" {
			err := fmt.Errorf("nodeInfoData.CspResourceId is empty (required for register VM)")
			nodeInfoData.Status = model.StatusFailed
			nodeInfoData.SystemMessage = err.Error()
			UpdateNodeInfo(nsId, infraId, *nodeInfoData)
			log.Error().Err(err).Msg("")
			return err
		}
	}

	var callResult model.SpiderVMInfo

	// Fill VM creation reqest (request to cb-spider)
	requestBody := model.SpiderVMReqInfoWrapper{}
	requestBody.ConnectionName = nodeInfoData.ConnectionName

	// Zone this VM is being placed in, resolved from the selected subnet below.
	attemptedZone := ""

	//generate VM ID(Name) to request to CSP(Spider)
	requestBody.ReqInfo.Name = nodeInfoData.Uid

	customImageFlag := false

	requestBody.ReqInfo.VMUserId = nodeInfoData.NodeUserName
	requestBody.ReqInfo.VMUserPasswd = nodeInfoData.NodeUserPassword
	// provide a random passwd, if it is not provided by user (the passwd required for Windows)
	if requestBody.ReqInfo.VMUserPasswd == "" {
		// assign random string (mixed Uid style)
		requestBody.ReqInfo.VMUserPasswd = common.GenRandomPassword(14)
	}

	// Users give CSP-native disk types; translate to CB-Spider's identifier when assets/diskinfo.yaml declares one.
	providerName := nodeInfoData.ConnectionConfig.ProviderName
	if providerName == "" {
		if cc, err := common.GetConnConfig(nodeInfoData.ConnectionName); err == nil {
			providerName = cc.ProviderName
		}
	}
	requestBody.ReqInfo.RootDiskType = resource.ToCBSpiderDiskType(providerName, nodeInfoData.RootDiskType)
	// Convert int to string for Spider API
	if nodeInfoData.RootDiskSize > 0 {
		requestBody.ReqInfo.RootDiskSize = strconv.Itoa(nodeInfoData.RootDiskSize)
	} else {
		requestBody.ReqInfo.RootDiskSize = ""
	}

	// NOTE: We intentionally do NOT auto-apply a stock-aware system-disk
	// suggestion here. The infra dynamic flow binds vnet/subnet to a single
	// representative zone of the region, but availability suggestions are
	// computed across all zones; silently switching RootDiskType based on a
	// different zone can still cause "No AvailableSystemDisk" failures and
	// surprises the user. Suggestions are exposed via the review APIs
	// (SpecImagePairReviewResult.SuggestedSystemDisk) for UI display only.

	if option == "register" {
		// Pre-check: reject registration of instances that no longer exist or are already
		// terminated in the CSP. Uses the direct batch SDK (same path as BatchSweeper) so
		// no Spider round-trip is needed. CSPs without a registered handler skip the check.
		if handler, ok := cspcheck.GetBatchVMStatusHandler(nodeInfoData.ConnectionConfig.ProviderName); ok && nodeInfoData.CspResourceId != "" {
			sdkCtx := context.WithValue(ctx, model.CtxKeyCredentialHolder, nodeInfoData.ConnectionConfig.CredentialHolder)
			preCheckStatuses, preCheckErr := handler(sdkCtx, nodeInfoData.ConnectionConfig.RegionDetail.RegionName, []string{nodeInfoData.CspResourceId})
			if preCheckErr != nil {
				log.Warn().Err(preCheckErr).Msgf("[register] pre-check SDK call failed for %s; proceeding to let Spider decide", nodeInfoData.CspResourceId)
			} else {
				instanceStatus, found := preCheckStatuses[nodeInfoData.CspResourceId]
				if !found || strings.EqualFold(instanceStatus, model.StatusTerminated) {
					// The direct SDK confirmed the instance is gone from the CSP (clean
					// response, id absent) — a definitive not-found, so mark Terminated
					// (not Failed): the record reflects reality and is a clean GC target.
					msg := fmt.Sprintf("instance %s is not found or already terminated in CSP; skipping registration", nodeInfoData.CspResourceId)
					nodeInfoData.Status = model.StatusTerminated
					nodeInfoData.SystemMessage = msg
					UpdateNodeInfo(nsId, infraId, *nodeInfoData)
					log.Warn().Msgf("[register] %s", msg)
					return fmt.Errorf("%s", msg)
				}
			}
		}

		requestBody.ReqInfo.CSPid = nodeInfoData.CspResourceId
		// SecurityGroupNames is intentionally not set here:
		// Spider's /regvm auto-discovers security groups from the existing CSP instance.
		// Passing TB-resolved SG names can interfere with Spider's internal name mapping,
		// and the SecurityGroupIds field in the request may contain "unknown" placeholders
		// used by the auto-registration flow (utility.go) before resources are fully resolved.
	} else {
		if nodeInfoData.CspImageName != "" {
			// CspImageName was pre-resolved at nodegroup level (Alibaba/Azure latest-version
			// resolution already applied). Skip the redundant per-VM GetImage DB call.
			// This eliminates O(nodeCount) concurrent DB queries during large infra creation.
			// Custom images always have CspImageName empty here and go through the full path below.
			requestBody.ReqInfo.ImageName = nodeInfoData.CspImageName
		} else {
			// Full path: custom images and any node not going through the dynamic creation flow.
			imageInfo, err := resource.GetImage(nsId, nodeInfoData.ImageId)
			if err != nil {
				log.Debug().Msgf("GetImage returned an error: %s", err.Error())
				nodeInfoData.Status = model.StatusFailed
				nodeInfoData.SystemMessage = err.Error()
				UpdateNodeInfo(nsId, infraId, *nodeInfoData)
				return err
			}
			// A customImage with pending/unconfirmed deletion must not be used
			if imageInfo.DeletionRequestedAt != "" {
				err := fmt.Errorf("image '%s' has a pending/unconfirmed deletion (status=%s); retry DELETE to complete it or use another image",
					nodeInfoData.ImageId, imageInfo.ImageStatus)
				nodeInfoData.Status = model.StatusFailed
				nodeInfoData.SystemMessage = err.Error()
				UpdateNodeInfo(nsId, infraId, *nodeInfoData)
				return err
			}
			// Resolve provider-specific "latest image" (Alibaba, Azure) right before
			// handing the CSP image name to cb-spider.
			imageInfo = resource.ResolveLatestImageForVMCreation(ctx, nodeInfoData.ConnectionName, imageInfo)
			if imageInfo.ResourceType == model.StrCustomImage {
				customImageFlag = true
				requestBody.ReqInfo.ImageType = model.MyImage
				requestBody.ReqInfo.RootDiskType = ""
				requestBody.ReqInfo.RootDiskSize = ""
				requestBody.ReqInfo.ImageName = imageInfo.CspImageName
				log.Debug().Msgf("CustomImage detected, set ImageName to CspImageId: %s", requestBody.ReqInfo.ImageName)
				log.Debug().Msgf("CustomImage detected, ignore RootDiskType and RootDiskSize")
			} else {
				requestBody.ReqInfo.ImageName = imageInfo.CspImageName
			}
		}

		requestBody.ReqInfo.VMSpecName, err = resource.GetCspResourceName(nsId, model.StrSpec, nodeInfoData.SpecId)
		if requestBody.ReqInfo.VMSpecName == "" || err != nil {
			log.Warn().Msgf("Not found the Spec: %s in nsId: %s, find it from SystemCommonNs", nodeInfoData.SpecId, nsId)
			errAgg := err.Error()
			// If cannot find the resource, use common resource
			requestBody.ReqInfo.VMSpecName, err = resource.GetCspResourceName(model.SystemCommonNs, model.StrSpec, nodeInfoData.SpecId)
			log.Info().Msgf("Use the common VMSpecName: %s", requestBody.ReqInfo.VMSpecName)
			// Warm the user-namespace cache entry so subsequent VMs with the same spec skip the DB miss.
			if requestBody.ReqInfo.VMSpecName != "" && err == nil {
				resource.WarmSpecNameCache(nsId, nodeInfoData.SpecId, requestBody.ReqInfo.VMSpecName)
			}

			if requestBody.ReqInfo.VMSpecName == "" || err != nil {
				errAgg += err.Error()
				err = fmt.Errorf("%s", errAgg)

				nodeInfoData.Status = model.StatusFailed
				nodeInfoData.SystemMessage = err.Error()
				UpdateNodeInfo(nsId, infraId, *nodeInfoData)
				log.Error().Err(err).Msg("")

				return err
			}
		}

		requestBody.ReqInfo.VPCName, err = resource.GetCspResourceName(nsId, model.StrVNet, nodeInfoData.VNetId)
		if requestBody.ReqInfo.VPCName == "" {
			nodeInfoData.Status = model.StatusFailed
			nodeInfoData.SystemMessage = fmt.Sprintf("VPC lookup failed for VNetId %s: %v", nodeInfoData.VNetId, err)
			UpdateNodeInfo(nsId, infraId, *nodeInfoData)
			log.Error().Err(err).Msg("")
			return err
		}

		// retrieve csp subnet id
		subnetInfo, err := resource.GetSubnet(nsId, nodeInfoData.VNetId, nodeInfoData.SubnetId)
		if err != nil {
			log.Error().Err(err).Msg("Cannot find the Subnet ID: " + nodeInfoData.SubnetId)
			nodeInfoData.Status = model.StatusFailed
			nodeInfoData.SystemMessage = err.Error()
			UpdateNodeInfo(nsId, infraId, *nodeInfoData)
			return err
		}

		// The subnet is the only thing that pins a zone: neither SpiderVMReqInfo nor
		// the driver-level VMReqInfo carries one, and CB-Spider derives the target
		// zone from the subnet. Record it now so a failure can report where it was
		// attempted — several CSPs never name the zone in their error text.
		attemptedZone = subnetInfo.Zone

		requestBody.ReqInfo.SubnetName = subnetInfo.CspResourceName
		if requestBody.ReqInfo.SubnetName == "" {
			nodeInfoData.Status = model.StatusFailed
			nodeInfoData.SystemMessage = fmt.Sprintf("Empty SubnetName for SubnetId %s in VNetId %s", nodeInfoData.SubnetId, nodeInfoData.VNetId)
			UpdateNodeInfo(nsId, infraId, *nodeInfoData)
			log.Error().Msg(nodeInfoData.SystemMessage)
			return err
		}

		var SecurityGroupIdsTmp []string
		for _, v := range nodeInfoData.SecurityGroupIds {
			CspResourceId, err := resource.GetCspResourceName(nsId, model.StrSecurityGroup, v)
			if CspResourceId == "" {
				nodeInfoData.Status = model.StatusFailed
				nodeInfoData.SystemMessage = err.Error()
				UpdateNodeInfo(nsId, infraId, *nodeInfoData)
				log.Error().Err(err).Msg("")
				return err
			}

			SecurityGroupIdsTmp = append(SecurityGroupIdsTmp, CspResourceId)
		}
		requestBody.ReqInfo.SecurityGroupNames = SecurityGroupIdsTmp

		var DataDiskIdsTmp []string
		for _, v := range nodeInfoData.DataDiskIds {
			// ignore DataDiskIds == "", assume it is ignorable mistake
			if v != "" {
				CspResourceId, err := resource.GetCspResourceName(nsId, model.StrDataDisk, v)
				if err != nil || CspResourceId == "" {
					nodeInfoData.Status = model.StatusFailed
					nodeInfoData.SystemMessage = err.Error()
					UpdateNodeInfo(nsId, infraId, *nodeInfoData)
					log.Error().Err(err).Msg("")
					return err
				}
				DataDiskIdsTmp = append(DataDiskIdsTmp, CspResourceId)
			}
		}
		requestBody.ReqInfo.DataDiskNames = DataDiskIdsTmp

		requestBody.ReqInfo.KeyPairName, err = resource.GetCspResourceName(nsId, model.StrSSHKey, nodeInfoData.SshKeyId)
		if requestBody.ReqInfo.KeyPairName == "" {
			nodeInfoData.Status = model.StatusFailed
			nodeInfoData.SystemMessage = err.Error()
			UpdateNodeInfo(nsId, infraId, *nodeInfoData)
			log.Error().Err(err).Msg("")
			return err
		}
	}

	common.RandomSleep(0, 5*1000)
	// log.Info().Msg("VM request body to CB-Spider")
	// common.PrintJsonPretty(requestBody)

	client := clientManager.NewHttpClient()
	method := "POST"
	client.SetTimeout(20 * time.Minute)

	url := model.SpiderRestUrl + "/vm"
	if option == "register" {
		url = model.SpiderRestUrl + "/regvm"
	}

	// API throttling (e.g. AWS RequestLimitExceeded on RunInstances) rejects the request before
	// anything is created, so it is safe to retry with backoff instead of failing the node.
	for attempt := 1; ; attempt++ {
		_, err = clientManager.ExecuteHttpRequest(
			client,
			method,
			url,
			nil,
			clientManager.SetUseBody(requestBody),
			&requestBody,
			&callResult,
			clientManager.MediumDuration,
		)
		if err == nil || attempt >= createThrottleMaxAttempts || !isApiThrottlingError(err) || ctx.Err() != nil {
			break
		}
		wait := createThrottleBackoff(attempt)
		log.Warn().Msgf("[CreateNode] VM %s throttled by the CSP API (attempt %d/%d); retrying in %s", nodeInfoData.Name, attempt, createThrottleMaxAttempts, wait)
		time.Sleep(wait)
	}

	if err != nil {
		err = fmt.Errorf("%v", err)

		recordNodeFailure(nodeInfoData, providerName, attemptedZone, err)

		if isQuotaOrCapacityError(err) {
			// Definitive pre-create rejection: the CSP refused the request before any
			// resource was provisioned — no VM exists on the CSP side. Mark Failed
			// directly so the user sees the real cause without a misleading reconcile step.
			// Covers: quota/capacity exhaustion, invalid image/spec combinations, etc.
			nodeInfoData.Status = model.StatusFailed
			nodeInfoData.TargetAction = model.ActionComplete
			nodeInfoData.TargetStatus = ""
			UpdateNodeInfo(nsId, infraId, *nodeInfoData)
			log.Warn().Err(err).Msgf("[CreateNode] VM %s rejected by CSP before provisioning; marking Failed.", nodeInfoData.Name)
			return err
		}

		// Spider POST /vm returned an error without VM info (CspResourceName not yet set).
		// Mark Failed so the user sees a clear terminal state. If the VM was actually
		// created on the CSP before the error occurred (e.g. NHN: VM created then
		// Floating IP assignment failed), action=reconcile will rescue it via /allvm.
		// Nodes that are Failed with no cspResourceName are routed to orphan rescue
		// in reconcileInfraForward, so no state is lost.
		nodeInfoData.Status = model.StatusFailed
		nodeInfoData.TargetAction = model.ActionComplete
		nodeInfoData.TargetStatus = ""
		UpdateNodeInfo(nsId, infraId, *nodeInfoData)
		log.Warn().Err(err).Msgf("[CreateNode] Spider returned error for VM %s without VM identity info; "+
			"marking Failed. Run action=reconcile to rescue any orphaned CSP VM, or action=refine to remove.", nodeInfoData.Name)
		return err
	}

	nodeInfoData.AddtionalDetails = callResult.KeyValueList
	nodeInfoData.NodeUserName = callResult.VMUserId
	nodeInfoData.NodeUserPassword = callResult.VMUserPasswd
	nodeInfoData.CspResourceName = callResult.IId.NameId
	nodeInfoData.CspResourceId = callResult.IId.SystemId
	nodeInfoData.Region = callResult.Region
	nodeInfoData.PublicIP = callResult.PublicIP
	// Convert port string from Spider to int
	if portStr, err := TrimIP(callResult.SSHAccessPoint); err == nil {
		if port, err := strconv.Atoi(portStr); err == nil {
			nodeInfoData.SSHPort = port
		}
	}
	nodeInfoData.PublicDNS = callResult.PublicDNS
	nodeInfoData.PrivateIP = callResult.PrivateIP
	nodeInfoData.PrivateDNS = callResult.PrivateDNS
	nodeInfoData.RootDiskType = callResult.RootDiskType
	// Convert RootDiskSize string from Spider to int
	if rootDiskSize, err := strconv.Atoi(callResult.RootDiskSize); err == nil {
		nodeInfoData.RootDiskSize = rootDiskSize
	}
	nodeInfoData.RootDeviceName = callResult.RootDeviceName
	nodeInfoData.NetworkInterface = callResult.NetworkInterface

	nodeInfoData.CspSpecName = callResult.VMSpecName
	nodeInfoData.CspImageName = callResult.ImageIId.SystemId
	nodeInfoData.CspVNetId = callResult.VpcIID.SystemId
	nodeInfoData.CspSubnetId = callResult.SubnetIID.SystemId
	nodeInfoData.CspSshKeyId = callResult.KeyPairIId.SystemId

	if option == "register" {
		// Reconstuct resource IDs
		// Spec: resolve from DB, or fetch from the CSP and register on demand (as with Image).
		if callResult.VMSpecName != "" {
			specInfo, isAutoRegistered, err := resource.EnsureSpecAvailable(requestBody.ConnectionName, callResult.VMSpecName)
			if err != nil {
				log.Warn().Err(err).Msgf("Cannot resolve spec '%s' for registered VM; leaving spec unset", callResult.VMSpecName)
			} else {
				nodeInfoData.SpecId = specInfo.Id
				if isAutoRegistered {
					log.Info().Msgf("Auto-registered spec '%s' (ID: %s) from CSP during registration", callResult.VMSpecName, specInfo.Id)
				}
			}
		}

		// Image
		targetImageName := callResult.ImageIId.SystemId
		if targetImageName == "" {
			targetImageName = callResult.ImageIId.NameId
		} else {
			// Try to use EnsureImageAvailable for consistent image handling
			imageInfo, isAutoRegistered, err := resource.EnsureImageAvailable(ctx, nsId, requestBody.ConnectionName, targetImageName)

			if err != nil {
				// Best-effort: registration continues with the image left unset, so keep this a
				// warning rather than an error that reads like a failed registration.
				log.Warn().Err(err).Msgf("Cannot resolve image '%s' for registered VM; leaving image unset", targetImageName)
			} else {
				nodeInfoData.ImageId = imageInfo.Id

				// Determine if this is a custom image
				if imageInfo.ResourceType == model.StrCustomImage {
					customImageFlag = true
				}

				if !isAutoRegistered {
					log.Debug().Msgf("Image found in DB: %s (ID: %s)", targetImageName, imageInfo.Id)
				}
			}
		}

		// vNet
		resourceListInNs, err := resource.ListResource(nsId, model.StrVNet, "cspResourceName", callResult.VpcIID.SystemId)
		if err != nil {
			log.Error().Err(err).Msg("")
		} else {
			resourcesInNs := resourceListInNs.([]model.VNetInfo) // type assertion
			for _, resource := range resourcesInNs {
				if resource.ConnectionName == requestBody.ConnectionName {
					nodeInfoData.VNetId = resource.Id

					// subnet
					targetSubnet := callResult.SubnetIID.SystemId

					if targetSubnet == "" {
						targetSubnet = callResult.SubnetIID.NameId
					}

					for _, subnet := range resource.SubnetInfoList {
						if subnet.CspResourceId == targetSubnet {
							nodeInfoData.SubnetId = subnet.Id
							break
						}
					}
					break
				}
			}
		}

		// SecurityGroups
		var matchedSgIds []string
		for _, sgIID := range callResult.SecurityGroupIIds {
			resourceListInNs, err := resource.ListResource(nsId, model.StrSecurityGroup, "cspResourceName", sgIID.SystemId)
			if err != nil {
				log.Error().Err(err).Msg("")
			} else {
				resourcesInNs := resourceListInNs.([]model.SecurityGroupInfo)
				for _, resource := range resourcesInNs {
					if resource.ConnectionName == requestBody.ConnectionName {
						matchedSgIds = append(matchedSgIds, resource.Id)
						break
					}
				}
			}
		}
		nodeInfoData.SecurityGroupIds = matchedSgIds

		// access Key
		sshKeyMatched := false
		if callResult.KeyPairIId.SystemId != "" {
			resourceListInNs, err = resource.ListResource(nsId, model.StrSSHKey, "cspResourceName", callResult.KeyPairIId.SystemId)
			if err != nil {
				log.Warn().Err(err).Msg("Failed to list SSH keys for matching")
			} else {
				resourcesInNs := resourceListInNs.([]model.SshKeyInfo) // type assertion
				for _, res := range resourcesInNs {
					if res.ConnectionName == requestBody.ConnectionName {
						nodeInfoData.SshKeyId = res.Id
						sshKeyMatched = true
						break
					}
				}
			}
		}

		// GCP does not have SSH key as an independent resource object.
		// Create a placeholder SSH key so that VM registration can proceed.
		// The user can later update this SSH key via the ComplementSshKey API.
		if !sshKeyMatched {
			providerName := strings.ToLower(nodeInfoData.ConnectionConfig.ProviderName)
			if csp.ResolveCloudPlatform(providerName) == csp.GCP {
				log.Info().Msgf("GCP detected: creating placeholder SSH key for VM '%s' (GCP does not manage SSH keys as independent resources)", nodeInfoData.Name)
				placeholderSshKey, placeholderErr := resource.CreatePlaceholderSshKey(ctx, nsId, requestBody.ConnectionName, nodeInfoData.Name, nodeInfoData.Uid)
				if placeholderErr != nil {
					log.Error().Err(placeholderErr).Msgf("Failed to create placeholder SSH key for GCP VM '%s'", nodeInfoData.Name)
				} else {
					nodeInfoData.SshKeyId = placeholderSshKey.Id
					log.Info().Msgf("Successfully created placeholder SSH key '%s' for GCP VM '%s'", placeholderSshKey.Id, nodeInfoData.Name)
				}
			} else {
				log.Warn().Msgf("No matching SSH key found for VM '%s' (provider: %s, cspKeyPairId: %s)", nodeInfoData.Name, providerName, callResult.KeyPairIId.SystemId)
			}
		}

	}

	if ctx.Value(ctxKeySkipAssoc{}) != true {
		if customImageFlag == false {
			resource.UpdateAssociatedObjectList(nsId, model.StrImage, nodeInfoData.ImageId, model.StrAdd, nodeKey)
		} else {
			resource.UpdateAssociatedObjectList(nsId, model.StrCustomImage, nodeInfoData.ImageId, model.StrAdd, nodeKey)
		}

		//resource.UpdateAssociatedObjectList(nsId, model.StrSpec, nodeInfoData.SpecId, model.StrAdd, nodeKey)
		if nodeInfoData.SshKeyId != "" {
			resource.UpdateAssociatedObjectList(nsId, model.StrSSHKey, nodeInfoData.SshKeyId, model.StrAdd, nodeKey)
		}
		resource.UpdateAssociatedObjectList(nsId, model.StrVNet, nodeInfoData.VNetId, model.StrAdd, nodeKey)

		for _, v := range nodeInfoData.SecurityGroupIds {
			resource.UpdateAssociatedObjectList(nsId, model.StrSecurityGroup, v, model.StrAdd, nodeKey)
		}

		for _, v := range nodeInfoData.DataDiskIds {
			resource.UpdateAssociatedObjectList(nsId, model.StrDataDisk, v, model.StrAdd, nodeKey)
		}
	}

	// Register dataDisks which are created with the creation of VM
	for _, v := range callResult.DataDiskIIDs {
		tbDataDiskReq := model.DataDiskReq{
			Name:           v.NameId,
			ConnectionName: nodeInfoData.ConnectionName,
			CspResourceId:  v.SystemId,
		}

		dataDisk, err := resource.CreateDataDisk(ctx, nsId, &tbDataDiskReq, "register")
		if err != nil {
			err = fmt.Errorf("after starting VM %s, failed to register dataDisk %s. \n", nodeInfoData.Name, v.NameId)
			log.Err(err).Msg("")
		}

		nodeInfoData.DataDiskIds = append(nodeInfoData.DataDiskIds, dataDisk.Id)

		if ctx.Value(ctxKeySkipAssoc{}) != true {
			resource.UpdateAssociatedObjectList(nsId, model.StrDataDisk, dataDisk.Id, model.StrAdd, nodeKey)
		}
	}

	// Populate SpecSummary and ImageSummary for NodeInfo
	if nodeInfoData.SpecId != "" {
		specInfo, err := resource.GetSpec(model.SystemCommonNs, nodeInfoData.SpecId)
		if err != nil {
			log.Warn().Err(err).Msgf("Failed to get spec info for SpecSummary: %s", nodeInfoData.SpecId)
		} else {
			nodeInfoData.Spec = model.SpecSummary{
				CspSpecName:         specInfo.CspSpecName,
				VCPU:                specInfo.VCPU,
				MemoryGiB:           specInfo.MemoryGiB,
				AcceleratorModel:    specInfo.AcceleratorModel,
				AcceleratorCount:    specInfo.AcceleratorCount,
				AcceleratorMemoryGB: specInfo.AcceleratorMemoryGB,
				AcceleratorType:     specInfo.AcceleratorType,
				CostPerHour:         specInfo.CostPerHour,
			}
		}
	}

	if nodeInfoData.ImageId != "" {
		imageInfo, err := resource.GetImage(nsId, nodeInfoData.ImageId)
		if err != nil {
			log.Warn().Err(err).Msgf("Failed to get image info for ImageSummary: %s", nodeInfoData.ImageId)
		} else {
			nodeInfoData.Image = model.ImageSummary{
				ResourceType:   imageInfo.ResourceType,
				CspImageName:   imageInfo.CspImageName,
				OSType:         imageInfo.OSType,
				OSArchitecture: imageInfo.OSArchitecture,
				OSDistribution: imageInfo.OSDistribution,
			}
		}
	}

	UpdateNodeInfo(nsId, infraId, *nodeInfoData)

	// set initial TargetAction, TargetStatus
	nodeInfoData.TargetAction = model.ActionComplete
	nodeInfoData.TargetStatus = model.StatusComplete

	// get and set current node status (the VM exists now: a status-poll failure must not mark it Failed)
	var nodeStatusInfoTmp model.NodeStatusInfo
	for attempt := 1; attempt <= 3; attempt++ {
		nodeStatusInfoTmp, err = FetchNodeStatus(nsId, infraId, nodeInfoData.Id)
		if err == nil {
			break
		}
		log.Warn().Err(err).Msgf("[CreateNode] status fetch failed for %s after creation (attempt %d/3)", nodeInfoData.Id, attempt)
		time.Sleep(time.Duration(attempt*5) * time.Second)
	}
	if err != nil {
		// Keep Creating; the status poller converges once the CSP API is reachable again.
		nodeInfoData.Status = model.StatusCreating
		nodeInfoData.SystemMessage = fmt.Sprintf("VM created; status not yet confirmed: %v", err)
		UpdateNodeInfo(nsId, infraId, *nodeInfoData)
		log.Warn().Err(err).Msgf("[CreateNode] %s created but status unconfirmed; leaving Creating for the poller", nodeInfoData.Id)
		return nil
	}

	nodeInfoData.Status = nodeStatusInfoTmp.Status

	// Monitoring Agent Installation Status (init: notInstalled)
	nodeInfoData.MonAgentStatus = "notInstalled"
	nodeInfoData.NetworkAgentStatus = "notInstalled"

	// set CreatedTime
	t := time.Now()
	nodeInfoData.CreatedTime = t.Format("2006-01-02 15:04:05")
	log.Debug().Msg(nodeInfoData.CreatedTime)

	UpdateNodeInfo(nsId, infraId, *nodeInfoData)

	// Assign a Bastion if none (randomly). Runs after the fetched status is
	// persisted above: auto-selection only accepts Running candidates, so an
	// earlier call would see this node (and siblings) as still Creating.
	if _, bastionErr := SetBastionNodes(nsId, infraId, nodeInfoData.Id, "", "", ""); bastionErr != nil {
		// just log and continue (e.g. a bastion already exists for the subnet)
		log.Debug().Msg(bastionErr.Error())
	}

	// Store label info using CreateOrUpdateLabel
	labels := map[string]string{
		model.LabelManager:         model.StrManager,
		model.LabelNamespace:       nsId,
		model.LabelLabelType:       model.StrNode,
		model.LabelId:              nodeInfoData.Id,
		model.LabelName:            nodeInfoData.Name,
		model.LabelUid:             nodeInfoData.Uid,
		model.LabelCspResourceId:   nodeInfoData.CspResourceId,
		model.LabelCspResourceName: nodeInfoData.CspResourceName,
		model.LabelNodeGroupId:     nodeInfoData.NodeGroupId,
		model.LabelInfraId:         infraId,
		model.LabelCreatedTime:     nodeInfoData.CreatedTime,
		model.LabelConnectionName:  nodeInfoData.ConnectionName,
		model.LabelVNetId:          nodeInfoData.VNetId,
		model.LabelSubnetId:        nodeInfoData.SubnetId,
	}
	maps.Copy(labels, nodeInfoData.Label)
	err = label.CreateOrUpdateLabel(ctx, model.StrNode, nodeInfoData.Uid, nodeKey, labels)
	if err != nil {
		err = fmt.Errorf("cannot create label object: %v", err)
		nodeInfoData.Status = model.StatusFailed
		nodeInfoData.SystemMessage = err.Error()
		UpdateNodeInfo(nsId, infraId, *nodeInfoData)

		log.Error().Err(err).Msg("")
		return err
	}

	return nil
}

