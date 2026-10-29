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
	"fmt"
	"strings"
	"sync"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	cspcheck "github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/rs/zerolog/log"
)

func ValidateInfraDynamicReq(ctx context.Context, nsId string, req *model.InfraDynamicReq, deployOption string) (*model.ReviewInfraDynamicReqInfo, error) {
	return ReviewInfraDynamicReq(ctx, nsId, req, deployOption)
}

// reviewSingleNodeGroupDynamicReq reviews and validates a single VM dynamic request
func reviewSingleNodeGroupDynamicReq(ctx context.Context, nsId string, infraSgTemplateId string, infraVNetTemplateId string, nodeGroupDynamicReq model.CreateNodeGroupDynamicReq, deployOption string) (model.ReviewNodeGroupDynamicReqInfo, *model.SpecInfo, bool, bool, float64) {

	credentialHolder := common.CredentialHolderFromContext(ctx)
	nodeReview := model.ReviewNodeGroupDynamicReqInfo{
		NodeName:      nodeGroupDynamicReq.Name,
		NodeGroupSize: nodeGroupDynamicReq.NodeGroupSize,
		CanCreate:     true,
		Status:        "Ready",
		Info:          make([]string, 0),
		Warnings:      make([]string, 0),
		Errors:        make([]string, 0),
	}

	viable := true
	hasNodeWarning := false
	var specInfoPtr *model.SpecInfo
	nodeCost := 0.0

	// Validate VM name
	if nodeGroupDynamicReq.Name == "" {
		nodeReview.Warnings = append(nodeReview.Warnings, "VM NodeGroup name not specified, will be auto-generated")
		hasNodeWarning = true
	}

	// Validate NodeGroupSize
	if nodeGroupDynamicReq.NodeGroupSize <= 0 {
		nodeGroupDynamicReq.NodeGroupSize = 1
		nodeReview.Warnings = append(nodeReview.Warnings, "NodeGroupSize not specified, defaulting to 1")
		hasNodeWarning = true
	}

	// Validate SpecId
	specInfo, err := resource.GetSpec(model.SystemCommonNs, nodeGroupDynamicReq.SpecId)
	if err != nil {
		nodeReview.Errors = append(nodeReview.Errors, fmt.Sprintf("Failed to get spec '%s': %v", nodeGroupDynamicReq.SpecId, err))
		nodeReview.SpecValidation = model.ReviewResourceValidation{
			ResourceId:  nodeGroupDynamicReq.SpecId,
			IsAvailable: false,
			Status:      "Unavailable",
			Message:     err.Error(),
		}
		nodeReview.CanCreate = false
		viable = false
	} else {
		specInfoPtr = &specInfo
		// Resolve connection name based on credential holder
		resolvedConnectionName := common.ResolveConnectionName(specInfo.ConnectionName, credentialHolder)
		nodeReview.ConnectionName = resolvedConnectionName
		nodeReview.ProviderName = specInfo.ProviderName
		nodeReview.RegionName = specInfo.RegionName

		// Check that the resolved connection exists and is verified.
		// Provisioning uses GetConnConfigList with filterVerified=true, so an
		// unverified connection will cause a "cannot find the connection config"
		// error at runtime — catch it here in the review stage instead.
		connConfig, connErr := common.GetConnConfig(resolvedConnectionName)
		if connErr != nil {
			nodeReview.Errors = append(nodeReview.Errors, fmt.Sprintf(
				"Connection '%s' (derived from spec '%s') not found: %v",
				resolvedConnectionName, nodeGroupDynamicReq.SpecId, connErr))
			nodeReview.CanCreate = false
			viable = false
		} else if !connConfig.Verified {
			nodeReview.Errors = append(nodeReview.Errors, fmt.Sprintf(
				"Connection '%s' is not verified. Complete connection verification before provisioning",
				resolvedConnectionName))
			nodeReview.CanCreate = false
			viable = false
		}

		// Check if spec is available in CSP using the provider-agnostic
		// availability checker (Alibaba: DescribeAvailableResource, Azure:
		// Resource SKU + quota, ...). Falls back to CB-Spider LookupSpec for
		// providers that have no registered checker.
		specAvailable := false
		var specCheckErr error
		cspSpecName := specInfo.CspSpecName

		availability := cspcheck.CheckAvailability(ctx, model.AvailabilityQuery{
			Provider:           csp.ResolveCloudPlatform(specInfo.ProviderName),
			Region:             specInfo.RegionName,
			InstanceType:       specInfo.CspSpecName,
			SystemDiskCategory: model.NormalizeDiskTypeForQuery(nodeGroupDynamicReq.RootDiskType),
			PreferredZone:      strings.TrimSpace(nodeGroupDynamicReq.Zone),
			AcceleratorModel:   specInfo.AcceleratorModel,
			AcceleratorCount:   int(specInfo.AcceleratorCount),
			VCPU:               int(specInfo.VCPU),
			MemoryGiB:          specInfo.MemoryGiB,
		})

		if availability.Source == "none" {
			// No checker registered for this provider: fall back to CB-Spider LookupSpec.
			cspSpec, lookupErr := resource.LookupSpec(resolvedConnectionName, specInfo.CspSpecName)
			if lookupErr == nil {
				specAvailable = true
				cspSpecName = cspSpec.Name
			} else {
				specCheckErr = lookupErr
			}
		} else {
			// Checker available: trust its result.
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
			nodeReview.Errors = append(nodeReview.Errors, fmt.Sprintf("Spec '%s' not available in CSP: %s", nodeGroupDynamicReq.SpecId, errMsg))
			nodeReview.SpecValidation = model.ReviewResourceValidation{
				ResourceId:    nodeGroupDynamicReq.SpecId,
				ResourceName:  specInfo.CspSpecName,
				IsAvailable:   false,
				Status:        "Unavailable",
				Message:       errMsg,
				CspResourceId: specInfo.CspSpecName,
			}
			nodeReview.CanCreate = false
			viable = false
		} else {
			nodeReview.SpecValidation = model.ReviewResourceValidation{
				ResourceId:    nodeGroupDynamicReq.SpecId,
				ResourceName:  specInfo.CspSpecName,
				IsAvailable:   true,
				Status:        "Available",
				Message:       availability.Reason,
				CspResourceId: cspSpecName,
			}
			if availability.Reason != "" {
				nodeReview.Warnings = append(nodeReview.Warnings, fmt.Sprintf("Zone availability notice for '%s': %s", specInfo.CspSpecName, availability.Reason))
				hasNodeWarning = true
			}

			// Add cost estimation if available
			if specInfo.CostPerHour > 0 {
				nodeGroupSizeInt := max(nodeGroupDynamicReq.NodeGroupSize, 1)
				nodeReview.EstimatedCost = fmt.Sprintf("$%.4f/hour", float64(specInfo.CostPerHour)*float64(nodeGroupSizeInt))
				nodeCost = float64(specInfo.CostPerHour) * float64(nodeGroupSizeInt)
			} else {
				nodeReview.EstimatedCost = "Cost estimation unavailable"
			}
		}
	}

	// Validate ImageId (with auto-registration if found in CSP but not in DB)
	if specInfoPtr != nil {
		resolvedConnName := common.ResolveConnectionName(specInfoPtr.ConnectionName, credentialHolder)
		imageInfo, isAutoRegistered, err := resource.EnsureImageAvailable(ctx, model.SystemCommonNs, resolvedConnName, nodeGroupDynamicReq.ImageId)
		if err != nil {
			nodeReview.Errors = append(nodeReview.Errors, fmt.Sprintf("Image '%s' not available: %v", nodeGroupDynamicReq.ImageId, err))
			nodeReview.ImageValidation = model.ReviewResourceValidation{
				ResourceId:    nodeGroupDynamicReq.ImageId,
				IsAvailable:   false,
				Status:        "Unavailable",
				Message:       err.Error(),
				CspResourceId: nodeGroupDynamicReq.ImageId,
			}
			nodeReview.CanCreate = false
			viable = false
		} else {
			status := "Available"
			if isAutoRegistered {
				status = "Available (Auto-registered)"
				nodeReview.Info = append(nodeReview.Info, fmt.Sprintf("Image '%s' was auto-registered from CSP", nodeGroupDynamicReq.ImageId))
			}
			nodeReview.ImageValidation = model.ReviewResourceValidation{
				ResourceId:    nodeGroupDynamicReq.ImageId,
				ResourceName:  imageInfo.Name,
				IsAvailable:   true,
				Status:        status,
				CspResourceId: imageInfo.CspImageName,
			}
			// Surface recoverable image-region mismatch (e.g. Alibaba family auto-resolution).
			// Use the pre-resolution record from DB so the warning reports both the
			// original (region-mismatched) CSP id and the resolved replacement.
			origImageInfo, origErr := resource.GetImage(model.SystemCommonNs, nodeGroupDynamicReq.ImageId)
			if origErr == nil {
				if warn, _ := resource.CheckImageRegionCompatibility(resolvedConnName, origImageInfo); warn != "" {
					if imageInfo.CspImageName != "" && imageInfo.CspImageName != origImageInfo.CspImageName {
						warn = fmt.Sprintf("%s (resolved CSP image id: %s)", warn, imageInfo.CspImageName)
					}
					nodeReview.Warnings = append(nodeReview.Warnings, warn)
					hasNodeWarning = true
				}
			}

			// Validate Spec ↔ Image compatibility (architecture, hypervisor, boot mode, disk controller, etc.)
			if compatErr := resource.CheckSpecImageCompatibility(*specInfoPtr, imageInfo); compatErr != nil {
				nodeReview.Errors = append(nodeReview.Errors, compatErr.Error())
				nodeReview.ImageValidation.IsAvailable = false
				nodeReview.ImageValidation.Status = "Incompatible"
				nodeReview.ImageValidation.Message = compatErr.Error()
				nodeReview.CanCreate = false
				viable = false
			}
		}
	}

	// Validate ConnectionName if explicitly specified in the request.
	// Treat missing or unverified connections as hard errors: provisioning uses
	// GetConnConfigList(filterVerified=true) so an unverified connection is
	// silently excluded at runtime, producing a confusing "cannot find the
	// connection config" failure instead of an actionable message.
	if nodeGroupDynamicReq.ConnectionName != "" {
		explicitConn, err := common.GetConnConfig(nodeGroupDynamicReq.ConnectionName)
		if err != nil {
			nodeReview.Errors = append(nodeReview.Errors, fmt.Sprintf(
				"Specified connection '%s' not found: %v",
				nodeGroupDynamicReq.ConnectionName, err))
			nodeReview.CanCreate = false
			viable = false
		} else if !explicitConn.Verified {
			nodeReview.Errors = append(nodeReview.Errors, fmt.Sprintf(
				"Specified connection '%s' is not verified. Complete connection verification before provisioning",
				nodeGroupDynamicReq.ConnectionName))
			nodeReview.CanCreate = false
			viable = false
		} else {
			nodeReview.ConnectionName = nodeGroupDynamicReq.ConnectionName
		}
	}

	// Validate RootDisk settings
	if nodeGroupDynamicReq.RootDiskType != "" && nodeGroupDynamicReq.RootDiskType != "default" {
		nodeReview.Info = append(nodeReview.Info, fmt.Sprintf("Root disk type configured: %s, be sure it's supported by the provider", nodeGroupDynamicReq.RootDiskType))
	}
	if nodeGroupDynamicReq.RootDiskSize > 0 {
		nodeReview.Info = append(nodeReview.Info, fmt.Sprintf("Root disk size configured: %d GB, be sure it meets minimum requirements", nodeGroupDynamicReq.RootDiskSize))
	}

	// Check provisioning history and risk analysis
	if specInfoPtr != nil {
		riskAnalysis, err := AnalyzeProvisioningRiskDetailed(nodeGroupDynamicReq.SpecId, nodeGroupDynamicReq.ImageId)
		if err != nil {
			log.Warn().Err(err).Msgf("Failed to analyze provisioning risk for VM: %s", nodeGroupDynamicReq.Name)
			nodeReview.Warnings = append(nodeReview.Warnings, "Failed to analyze provisioning history")
		} else {
			riskLevel := riskAnalysis.OverallRisk.Level
			riskMessage := riskAnalysis.OverallRisk.Message

			// Include recent failure messages if available
			var fullRiskMessage string
			if len(riskAnalysis.RecentFailureMessages) > 0 {
				fullRiskMessage = fmt.Sprintf("%s. Recent failure examples: %s",
					riskMessage, strings.Join(riskAnalysis.RecentFailureMessages, "; "))
			} else {
				fullRiskMessage = riskMessage
			}

			switch riskLevel {
			case "high":
				nodeReview.Errors = append(nodeReview.Errors, fmt.Sprintf("High provisioning failure risk: %s", fullRiskMessage))
				nodeReview.CanCreate = false
				viable = false
				log.Debug().Msgf("High risk detected for spec %s with image %s: %s", nodeGroupDynamicReq.SpecId, nodeGroupDynamicReq.ImageId, riskMessage)
			case "medium":
				nodeReview.Warnings = append(nodeReview.Warnings, fmt.Sprintf("Moderate provisioning failure risk: %s", fullRiskMessage))
				hasNodeWarning = true
				log.Debug().Msgf("Medium risk detected for spec %s with image %s: %s", nodeGroupDynamicReq.SpecId, nodeGroupDynamicReq.ImageId, riskMessage)
			case "low":
				if riskMessage != "No previous provisioning history available" && riskMessage != "No provisioning attempts recorded" {
					nodeReview.Info = append(nodeReview.Info, fmt.Sprintf("Provisioning history: %s", riskMessage))
				}
				log.Debug().Msgf("Low risk for spec %s with image %s: %s", nodeGroupDynamicReq.SpecId, nodeGroupDynamicReq.ImageId, riskMessage)
			default:
				log.Debug().Msgf("Unknown risk level for spec %s: %s", nodeGroupDynamicReq.SpecId, riskLevel)
			}
		}
	}

	// Check for provider-specific limitations
	if specInfoPtr != nil {
		providerName := specInfoPtr.ProviderName

		// Check KT Cloud limitations - temporary restriction to .itl specs only
		if csp.ResolveCloudPlatform(providerName) == csp.KT {
			if !strings.Contains(nodeGroupDynamicReq.SpecId, ".itl") {
				// Only show warning when spec does not contain '.itl'
				nodeReview.Warnings = append(nodeReview.Warnings, "KT Cloud provisioning is currently limited to '.itl' specs only (temporary limitation). This spec may fail to provision.")
				hasNodeWarning = true
				log.Debug().Msgf("KT Cloud warning for VM: %s (spec: %s does not contain '.itl')", nodeGroupDynamicReq.Name, nodeGroupDynamicReq.SpecId)
			} else {
				// '.itl' spec is valid, no warning needed
				log.Debug().Msgf("KT Cloud '.itl' spec detected for VM: %s (spec: %s)", nodeGroupDynamicReq.Name, nodeGroupDynamicReq.SpecId)
			}
		}

		// Check Alibaba China Local Region authorization.
		// Local Regions (cn-*-lr) require explicit account activation in the Alibaba Cloud Console.
		// No read-only API (DescribeAvailableResource, DescribeAccountAttributes) can detect this
		// restriction upfront — RunInstances returns RegionUnauthorized at creation time even when
		// stock is available and quota is positive.
		if csp.ResolveCloudPlatform(providerName) == csp.Alibaba &&
			strings.HasPrefix(specInfoPtr.RegionName, "cn-") &&
			strings.HasSuffix(specInfoPtr.RegionName, "-lr") {
			nodeReview.Warnings = append(nodeReview.Warnings, fmt.Sprintf(
				"Alibaba China Local Region %q requires explicit account activation in the Alibaba Cloud Console "+
					"(ECS > Local Regions > Activate) before instances can be created. "+
					"Without activation, RunInstances returns RegionUnauthorized even if stock is available. "+
					"Verify that your account has Local Region VM creation enabled.",
				specInfoPtr.RegionName))
			hasNodeWarning = true
			log.Debug().Msgf("Alibaba China Local Region warning for VM: %s (region: %s)", nodeGroupDynamicReq.Name, specInfoPtr.RegionName)
		}

		// // Check NHN Cloud limitations
		// if providerName == csp.NHN {
		// 	if deployOption != "hold" {
		// 		nodeReview.Errors = append(nodeReview.Errors, "NHN Cloud can only be provisioned with deployOption 'hold' (manual deployment required)")
		// 		nodeReview.CanCreate = false
		// 		viable = false
		// 		log.Debug().Msgf("NHN Cloud requires 'hold' deployOption for VM: %s", nodeGroupDynamicReq.Name)
		// 	} else {
		// 		nodeReview.Warnings = append(nodeReview.Warnings, "NHN Cloud requires manual deployment completion after 'hold' - automatic provisioning is not fully supported")
		// 		hasNodeWarning = true
		// 		log.Debug().Msgf("NHN Cloud 'hold' mode warning for VM: %s", nodeGroupDynamicReq.Name)
		// 	}
		// }
	}

	// Validate the effective SecurityGroup template exists before provisioning.
	// A missing template now aborts creation (no silent all-open fallback), so surface
	// it here — including the default 'sg-default' — to let users load it first.
	// Precedence: NodeGroup-level SgTemplateId > Infra-level > default template id.
	effectiveSgTemplateId := nodeGroupDynamicReq.SgTemplateId
	if effectiveSgTemplateId == "" {
		effectiveSgTemplateId = infraSgTemplateId
	}
	if effectiveSgTemplateId == "" {
		effectiveSgTemplateId = model.DefaultSecurityGroupTemplateId
	}
	sgTmplExists := false
	if nsId != model.SystemCommonNs {
		if _, err := common.GetSecurityGroupTemplate(nsId, effectiveSgTemplateId); err == nil {
			sgTmplExists = true
		}
	}
	if !sgTmplExists {
		if _, err := common.GetSecurityGroupTemplate(model.SystemCommonNs, effectiveSgTemplateId); err == nil {
			sgTmplExists = true
		}
	}
	if !sgTmplExists {
		nodeReview.Errors = append(nodeReview.Errors, fmt.Sprintf(
			"SecurityGroup template '%s' not found in namespace '%s' or system namespace; "+
				"load it before provisioning (e.g. run 'make init' to load init/templates, or register it via the template API)",
			effectiveSgTemplateId, nsId))
		nodeReview.CanCreate = false
		viable = false
	}

	// Validate the effective VNet template exists before provisioning. A missing template now
	// aborts creation (no silent hard-coded fallback), so surface it here — including the
	// default 'vnet-default'. Precedence: NodeGroup-level VNetTemplateId > Infra-level > default.
	effectiveVNetTemplateId := nodeGroupDynamicReq.VNetTemplateId
	if effectiveVNetTemplateId == "" {
		effectiveVNetTemplateId = infraVNetTemplateId
	}
	if effectiveVNetTemplateId == "" {
		effectiveVNetTemplateId = model.DefaultVNetTemplateId
	}
	vNetTmplExists := false
	if nsId != model.SystemCommonNs {
		if _, err := common.GetVNetTemplate(nsId, effectiveVNetTemplateId); err == nil {
			vNetTmplExists = true
		}
	}
	if !vNetTmplExists {
		if _, err := common.GetVNetTemplate(model.SystemCommonNs, effectiveVNetTemplateId); err == nil {
			vNetTmplExists = true
		}
	}
	if !vNetTmplExists {
		nodeReview.Errors = append(nodeReview.Errors, fmt.Sprintf(
			"VNet template '%s' not found in namespace '%s' or system namespace; "+
				"load it before provisioning (e.g. run 'make init' to load init/templates, or register it via the template API)",
			effectiveVNetTemplateId, nsId))
		nodeReview.CanCreate = false
		viable = false
	}

	// Set VM review status
	if len(nodeReview.Errors) > 0 {
		nodeReview.Status = "Error"
		nodeReview.Message = fmt.Sprintf("VM has %d error(s) that prevent creation", len(nodeReview.Errors))
	} else if len(nodeReview.Warnings) > 0 {
		nodeReview.Status = "Warning"
		nodeReview.Message = fmt.Sprintf("VM can be created but has %d warning(s)", len(nodeReview.Warnings))
	} else {
		nodeReview.Status = "Ready"
		nodeReview.Message = "VM can be created successfully"
	}

	log.Debug().Msgf("VM '%s' review completed: %s", nodeGroupDynamicReq.Name, nodeReview.Status)
	return nodeReview, specInfoPtr, viable, hasNodeWarning, nodeCost
}

// ReviewSpecImagePair reviews spec and image pair compatibility for provisioning.
//
// rootDiskType and zone are OPTIONAL refinements for the availability check:
//   - rootDiskType: empty or "default" means "no specific disk category"; the
//     checker reports stock across all categories supported in the region.
//     A specific value (e.g., "cloud_essd") narrows the check to that exact
//     disk so the UI can flag a combination that will fail at provisioning.
//   - zone: empty means "all zones in the region"; a specific zone scopes
//     the result to that single zone (so SuggestedSystemDisk reflects it).
func ReviewSpecImagePair(ctx context.Context, specId, imageId, rootDiskType, zone string) (*model.SpecImagePairReviewResult, error) {
	log.Debug().Msgf("Reviewing spec-image pair: spec=%s, image=%s, rootDiskType=%q, zone=%q",
		specId, imageId, rootDiskType, zone)

	// Normalize "default"/empty to "" for the availability query so checkers
	// don't send "default" as a literal disk category to the CSP API.
	normalizedDisk := model.NormalizeDiskTypeForQuery(rootDiskType)
	normalizedZone := strings.TrimSpace(zone)

	result := &model.SpecImagePairReviewResult{
		SpecId:                specId,
		ImageId:               imageId,
		IsValid:               true,
		Status:                "OK",
		Info:                  make([]string, 0),
		Warnings:              make([]string, 0),
		Errors:                make([]string, 0),
		RequestedRootDiskType: normalizedDisk,
		RequestedZone:         normalizedZone,
	}

	// Validate SpecId
	specInfo, err := resource.GetSpec(model.SystemCommonNs, specId)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to get spec '%s': %v", specId, err))
		result.SpecValidation = model.ReviewResourceValidation{
			ResourceId:  specId,
			IsAvailable: false,
			Status:      "Unavailable",
			Message:     err.Error(),
		}
		result.IsValid = false
		result.Status = "Error"
		result.Message = fmt.Sprintf("Spec '%s' is not available", specId)
	} else {
		result.SpecDetails = &specInfo
		// The spec record stores the default-tenant connection name. Resolve it
		// against the credential holder carried by ctx so that all subsequent
		// Spider calls (LookupSpec, EnsureImageAvailable) and the returned
		// ConnectionName use the requesting tenant's credentials.
		credentialHolder := common.CredentialHolderFromContext(ctx)
		resolvedConnectionName := common.ResolveConnectionName(specInfo.ConnectionName, credentialHolder)
		result.ConnectionName = resolvedConnectionName
		result.ProviderName = specInfo.ProviderName
		result.RegionName = specInfo.RegionName

		// Verify the resolved connection exists and is verified before proceeding.
		// An unverified connection passes GetConnConfig but is filtered out by
		// GetConnConfigList(filterVerified=true) used during actual provisioning,
		// which produces a confusing runtime error. Surface it here instead.
		pairConn, pairConnErr := common.GetConnConfig(resolvedConnectionName)
		if pairConnErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf(
				"Connection '%s' not found: %v", resolvedConnectionName, pairConnErr))
			result.SpecValidation = model.ReviewResourceValidation{
				ResourceId:  specId,
				IsAvailable: false,
				Status:      "Unavailable",
				Message:     fmt.Sprintf("Connection '%s' not found", resolvedConnectionName),
			}
			result.IsValid = false
			result.Status = "Error"
			result.Message = fmt.Sprintf("Connection '%s' not found", resolvedConnectionName)
			return result, nil
		} else if !pairConn.Verified {
			result.Errors = append(result.Errors, fmt.Sprintf(
				"Connection '%s' is not verified. Complete connection verification before provisioning",
				resolvedConnectionName))
			result.SpecValidation = model.ReviewResourceValidation{
				ResourceId:  specId,
				IsAvailable: false,
				Status:      "Unavailable",
				Message:     fmt.Sprintf("Connection '%s' is not verified", resolvedConnectionName),
			}
			result.IsValid = false
			result.Status = "Error"
			result.Message = fmt.Sprintf("Connection '%s' is not verified", resolvedConnectionName)
			return result, nil
		}

		// Check if spec is available in CSP using the provider-agnostic
		// availability checker (Alibaba: DescribeAvailableResource, Azure:
		// Resource SKU + quota, ...). Falls back to CB-Spider LookupSpec for
		// providers that have no registered checker.
		specAvailable := false
		var specCheckErr error

		availability := cspcheck.CheckAvailability(ctx, model.AvailabilityQuery{
			Provider:           csp.ResolveCloudPlatform(specInfo.ProviderName),
			Region:             specInfo.RegionName,
			InstanceType:       specInfo.CspSpecName,
			SystemDiskCategory: normalizedDisk,
			PreferredZone:      normalizedZone,
			AcceleratorModel:   specInfo.AcceleratorModel,
			AcceleratorCount:   int(specInfo.AcceleratorCount),
			VCPU:               int(specInfo.VCPU),
			MemoryGiB:          specInfo.MemoryGiB,
		})

		switch availability.Source {
		case "none":
			// No checker registered for this provider: fall back to CB-Spider LookupSpec.
			_, specCheckErr = resource.LookupSpec(resolvedConnectionName, specInfo.CspSpecName)
			if specCheckErr == nil {
				specAvailable = true
			}
		default:
			// Checker available (Alibaba, Azure, ...): trust its result.
			result.Availability = &availability
			specAvailable = availability.Available
			if !specAvailable {
				specCheckErr = fmt.Errorf("%s", availability.Reason)
			}
			// Pick suggested zone + disk from the matrix when present.
			// When the user specified a zone, prefer that zone's disks first.
			pickFrom := func(targetZone string) bool {
				for _, z := range availability.Zones {
					if !z.Available {
						continue
					}
					if targetZone != "" && !strings.EqualFold(z.ZoneId, targetZone) {
						continue
					}
					result.SuggestedZone = z.ZoneId
					if len(z.SupportedDisks) > 0 {
						// Prefer the requested disk type if it is in the list,
						// otherwise the first available one.
						chosen := z.SupportedDisks[0]
						if normalizedDisk != "" {
							for _, d := range z.SupportedDisks {
								if strings.EqualFold(d, normalizedDisk) {
									chosen = d
									break
								}
							}
						}
						result.SuggestedSystemDisk = chosen
					}
					return true
				}
				return false
			}
			if !pickFrom(normalizedZone) {
				// Fall back to any available zone when the requested zone has no stock.
				pickFrom("")
			}

			// If the user specified a disk type, warn when the suggested zone
			// doesn't actually support it (or when no zone supports it).
			if normalizedDisk != "" {
				diskSupportedSomewhere := false
				for _, z := range availability.Zones {
					if !z.Available {
						continue
					}
					for _, d := range z.SupportedDisks {
						if strings.EqualFold(d, normalizedDisk) {
							diskSupportedSomewhere = true
							break
						}
					}
					if diskSupportedSomewhere {
						break
					}
				}
				if !diskSupportedSomewhere {
					result.Warnings = append(result.Warnings, fmt.Sprintf(
						"requested rootDiskType '%s' is not currently available in region '%s' for spec '%s'; "+
							"VM creation may fail. Consider 'default' or '%s'.",
						normalizedDisk, specInfo.RegionName, specInfo.CspSpecName, result.SuggestedSystemDisk))
				}
			}
		}

		if specCheckErr != nil || !specAvailable {
			errMsg := "spec not available in CSP"
			if specCheckErr != nil {
				errMsg = specCheckErr.Error()
			}
			result.Errors = append(result.Errors, fmt.Sprintf("Spec '%s' not available in CSP: %s", specId, errMsg))
			result.SpecValidation = model.ReviewResourceValidation{
				ResourceId:    specId,
				ResourceName:  specInfo.CspSpecName,
				IsAvailable:   false,
				Status:        "Unavailable",
				Message:       errMsg,
				CspResourceId: specInfo.CspSpecName,
			}
			result.IsValid = false
			result.Status = "Error"
			result.Message = fmt.Sprintf("Spec '%s' is not available in CSP", specId)
		} else {
			result.SpecValidation = model.ReviewResourceValidation{
				ResourceId:    specId,
				ResourceName:  specInfo.CspSpecName,
				IsAvailable:   true,
				Status:        "Available",
				Message:       availability.Reason,
				CspResourceId: specInfo.CspSpecName,
			}
			if availability.Reason != "" {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Zone availability notice for '%s': %s", specInfo.CspSpecName, availability.Reason))
				if result.Status == "OK" {
					result.Status = "Warning"
				}
			}

			// Add cost estimation if available
			if specInfo.CostPerHour > 0 {
				result.EstimatedCost = fmt.Sprintf("$%.4f/hour", specInfo.CostPerHour)
			}

			// Warn for Alibaba China Local Regions that require explicit account activation.
			if csp.ResolveCloudPlatform(specInfo.ProviderName) == csp.Alibaba &&
				strings.HasPrefix(specInfo.RegionName, "cn-") &&
				strings.HasSuffix(specInfo.RegionName, "-lr") {
				result.Warnings = append(result.Warnings, fmt.Sprintf(
					"Alibaba China Local Region %q requires explicit account activation in the Alibaba Cloud Console "+
						"(ECS > Local Regions > Activate) before instances can be created. "+
						"Without activation, RunInstances returns RegionUnauthorized even if stock is available. "+
						"Verify that your account has Local Region VM creation enabled.",
					specInfo.RegionName))
				result.Status = "Warning"
				log.Debug().Msgf("Alibaba China Local Region warning for spec-image pair: %s (region: %s)", specId, specInfo.RegionName)
			}
		}
	}

	// Validate ImageId (with auto-registration if found in CSP but not in DB)
	if result.ConnectionName != "" {
		imageInfo, isAutoRegistered, err := resource.EnsureImageAvailable(ctx, model.SystemCommonNs, result.ConnectionName, imageId)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Image '%s' not available: %v", imageId, err))
			result.ImageValidation = model.ReviewResourceValidation{
				ResourceId:    imageId,
				IsAvailable:   false,
				Status:        "Unavailable",
				Message:       err.Error(),
				CspResourceId: imageId,
			}
			result.IsValid = false
			result.Status = "Error"
			if result.Message == "" {
				result.Message = fmt.Sprintf("Image '%s' is not available", imageId)
			} else {
				result.Message += fmt.Sprintf("; Image '%s' is not available", imageId)
			}
		} else {
			result.ImageDetails = &imageInfo
			status := "Available"
			if isAutoRegistered {
				status = "Available (Auto-registered)"
				result.Info = append(result.Info, fmt.Sprintf("Image '%s' was auto-registered from CSP", imageId))
			}
			result.ImageValidation = model.ReviewResourceValidation{
				ResourceId:    imageId,
				ResourceName:  imageInfo.Name,
				IsAvailable:   true,
				Status:        status,
				CspResourceId: imageInfo.CspImageName,
			}

			// Validate Spec ↔ Image compatibility (architecture, hypervisor, boot mode, disk controller, etc.)
			if result.SpecDetails != nil {
				if compatErr := resource.CheckSpecImageCompatibility(*result.SpecDetails, imageInfo); compatErr != nil {
					result.Errors = append(result.Errors, compatErr.Error())
					result.ImageValidation.IsAvailable = false
					result.ImageValidation.Status = "Incompatible"
					result.ImageValidation.Message = compatErr.Error()
					result.IsValid = false
					result.Status = "Error"
					if result.Message == "" {
						result.Message = compatErr.Error()
					} else {
						result.Message += "; " + compatErr.Error()
					}
				}
			}
		}
	} else {
		// Cannot validate image without connection info from spec
		result.ImageValidation = model.ReviewResourceValidation{
			ResourceId:  imageId,
			IsAvailable: false,
			Status:      "Unknown",
			Message:     "Cannot validate image without valid spec",
		}
	}

	// Check provisioning history and risk analysis
	if result.SpecValidation.IsAvailable {
		riskAnalysis, err := AnalyzeProvisioningRiskDetailed(specId, imageId)
		if err != nil {
			log.Warn().Err(err).Msgf("Failed to analyze provisioning risk for spec-image pair: %s / %s", specId, imageId)
			result.Warnings = append(result.Warnings, "Failed to analyze provisioning history")
		} else {
			riskLevel := riskAnalysis.OverallRisk.Level
			riskMessage := riskAnalysis.OverallRisk.Message

			// Include recent failure messages if available
			var fullRiskMessage string
			if len(riskAnalysis.RecentFailureMessages) > 0 {
				fullRiskMessage = fmt.Sprintf("%s. Recent failures: %s",
					riskMessage, strings.Join(riskAnalysis.RecentFailureMessages, "; "))
			} else {
				fullRiskMessage = riskMessage
			}

			switch riskLevel {
			case "high":
				result.Errors = append(result.Errors, fmt.Sprintf("High provisioning failure risk: %s", fullRiskMessage))
				result.IsValid = false
				result.Status = "Error"
				if result.Message == "" {
					result.Message = "High provisioning failure risk detected"
				} else {
					result.Message += "; High provisioning failure risk detected"
				}
				log.Debug().Msgf("High risk detected for spec %s with image %s: %s", specId, imageId, riskMessage)
			case "medium":
				result.Warnings = append(result.Warnings, fmt.Sprintf("Moderate provisioning failure risk: %s", fullRiskMessage))
				if result.Status == "OK" {
					result.Status = "Warning"
				}
				log.Debug().Msgf("Medium risk detected for spec %s with image %s: %s", specId, imageId, riskMessage)
			case "low":
				if riskMessage != "No previous provisioning history available" && riskMessage != "No provisioning attempts recorded" {
					result.Info = append(result.Info, fmt.Sprintf("Provisioning history: %s", riskMessage))
				}
				log.Debug().Msgf("Low risk for spec %s with image %s: %s", specId, imageId, riskMessage)
			}
		}
	}

	// Set final message if valid
	if result.IsValid {
		if result.Status == "Warning" {
			result.Message = "Spec and image pair is valid but has warnings"
		} else {
			result.Message = "Spec and image pair is valid for provisioning"
		}
	}

	log.Debug().Msgf("Spec-image pair review completed: %s - %s", result.Status, result.Message)
	return result, nil
}

// ReviewSingleNodeGroupDynamicReq reviews and validates a single VM dynamic request and returns comprehensive review information
func ReviewSingleNodeGroupDynamicReq(ctx context.Context, nsId string, req *model.CreateNodeGroupDynamicReq) (*model.ReviewNodeGroupDynamicReqInfo, error) {
	log.Debug().Msgf("Starting single VM dynamic request review for: %s", req.Name)

	// Basic validation
	err := common.CheckString(nsId)
	if err != nil {
		return nil, fmt.Errorf("invalid namespace: %w", err)
	}

	// Use the common VM review function with empty deployOption.
	// No infra-level SgTemplateId context here; the request's own SgTemplateId (if any) applies.
	nodeReview, _, _, _, _ := reviewSingleNodeGroupDynamicReq(ctx, nsId, "", "", *req, "")

	log.Debug().Msgf("Single VM review completed: %s - %s", nodeReview.Status, nodeReview.Message)
	return &nodeReview, nil
}

// ReviewInfraDynamicReq is func to review and validate Infra dynamic request comprehensively
func ReviewInfraDynamicReq(ctx context.Context, nsId string, req *model.InfraDynamicReq, deployOption string) (*model.ReviewInfraDynamicReqInfo, error) {

	log.Debug().Msgf("Starting Infra dynamic request review for: %s", req.Name)

	// Review is the dry run of creation: reject a malformed bootstrap request here
	// so the caller sees it before any provisioning attempt
	if err := ValidatePostCommandRequest(req.PostCommands); err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	reviewResult := &model.ReviewInfraDynamicReqInfo{
		InfraName:      req.Name,
		TotalNodeCount: len(req.NodeGroups),
		NodeReviews:    make([]model.ReviewNodeGroupDynamicReqInfo, 0),
		ResourceSummary: model.ReviewResourceSummary{
			UniqueSpecs:     make([]string, 0),
			UniqueImages:    make([]string, 0),
			ConnectionNames: make([]string, 0),
			ProviderNames:   make([]string, 0),
			RegionNames:     make([]string, 0),
		},
		Recommendations:        make([]string, 0),
		PolicyOnPartialFailure: req.PolicyOnPartialFailure,
	}

	// Basic validation
	err := common.CheckString(nsId)
	if err != nil {
		return nil, fmt.Errorf("invalid namespace: %w", err)
	}

	// Check if Infra name is valid and doesn't exist
	check, err := CheckInfra(nsId, req.Name)
	if err != nil {
		return nil, fmt.Errorf("invalid infra name: %w", err)
	}
	if check {
		reviewResult.OverallStatus = "Error"
		reviewResult.OverallMessage = fmt.Sprintf("Infra '%s' already exists in namespace '%s'", req.Name, nsId)
		reviewResult.CreationViable = false
		return reviewResult, nil
	}

	if len(req.NodeGroups) == 0 {
		reviewResult.OverallStatus = "Error"
		reviewResult.OverallMessage = "No VM requests provided"
		reviewResult.CreationViable = false
		return reviewResult, nil
	}

	// Track resource summary with thread-safe maps
	specMap := make(map[string]bool)
	imageMap := make(map[string]bool)
	connectionMap := make(map[string]bool)
	providerMap := make(map[string]bool)
	regionMap := make(map[string]bool)

	// Hierarchical concurrency for review:
	//   - Cross-CSP: parallel up to a global safety cap (prevents unbounded
	//     fanout when many NodeGroups are submitted at once).
	//   - Within a single CSP: capped per-CSP to avoid hitting CSP API rate
	//     limits on the read-only calls performed by reviewSingleNodeGroupDynamicReq
	//     (CheckAvailability, EnsureImageAvailable, LookupSpec, ...).
	// Defaults are conservative for read-heavy review traffic.
	const (
		maxReviewConcurrencyGlobal = 30
		maxReviewConcurrencyPerCSP = 3
	)
	globalSemaphore := make(chan struct{}, maxReviewConcurrencyGlobal)
	var cspSemMu sync.Mutex
	cspSemaphores := make(map[string]chan struct{})
	getCSPSemaphore := func(cspKey string) chan struct{} {
		cspSemMu.Lock()
		defer cspSemMu.Unlock()
		if s, ok := cspSemaphores[cspKey]; ok {
			return s
		}
		s := make(chan struct{}, maxReviewConcurrencyPerCSP)
		cspSemaphores[cspKey] = s
		return s
	}

	// Channel to collect VM review results
	nodeReviewChan := make(chan struct {
		index      int
		nodeReview model.ReviewNodeGroupDynamicReqInfo
		specInfo   *model.SpecInfo
		viable     bool
		warning    bool
		cost       float64
	}, len(req.NodeGroups))

	// WaitGroup to wait for all goroutines to complete
	var wg sync.WaitGroup

	// Validate each VM request in parallel
	for i, nodeGroupReq := range req.NodeGroups {
		wg.Add(1)
		go func(index int, nodeGroupDynamicReq model.CreateNodeGroupDynamicReq) {
			defer wg.Done()

			// Global cap (safety net across all CSPs).
			globalSemaphore <- struct{}{}
			defer func() { <-globalSemaphore }()

			// Determine the target CSP via a quick local spec lookup so we can
			// apply per-CSP rate limiting before the heavier CSP API calls
			// inside reviewSingleNodeGroupDynamicReq. GetSpec hits the local
			// cache/DB and is cheap relative to the gated work.
			cspKey := "_unknown"
			if specInfo, err := resource.GetSpec(model.SystemCommonNs, nodeGroupDynamicReq.SpecId); err == nil {
				cspKey = string(csp.ResolveCloudPlatform(specInfo.ProviderName))
			}
			cspSem := getCSPSemaphore(cspKey)
			cspSem <- struct{}{}
			defer func() { <-cspSem }()

			// Use the common VM review function
			nodeReview, specInfoPtr, viable, hasNodeWarning, nodeCost := reviewSingleNodeGroupDynamicReq(ctx, nsId, req.SgTemplateId, req.VNetTemplateId, nodeGroupDynamicReq, deployOption)

			// Send result to channel
			nodeReviewChan <- struct {
				index      int
				nodeReview model.ReviewNodeGroupDynamicReqInfo
				specInfo   *model.SpecInfo
				viable     bool
				warning    bool
				cost       float64
			}{
				index:      index,
				nodeReview: nodeReview,
				specInfo:   specInfoPtr,
				viable:     viable,
				warning:    hasNodeWarning,
				cost:       nodeCost,
			}

			log.Debug().Msgf("[%d] VM '%s' review completed: %s", index, nodeGroupDynamicReq.Name, nodeReview.Status)
		}(i, nodeGroupReq)
	}

	// Close channel when all goroutines are done
	go func() {
		wg.Wait()
		close(nodeReviewChan)
	}()

	// Collect results and maintain order
	nodeReviews := make([]model.ReviewNodeGroupDynamicReqInfo, len(req.NodeGroups))
	allViable := true
	hasWarnings := false
	totalEstimatedCost := 0.0
	nodeWithUnknownCost := 0

	// Process results from channel
	for result := range nodeReviewChan {
		// Store VM review result in correct order
		nodeReviews[result.index] = result.nodeReview

		// Update overall status flags
		if !result.viable {
			allViable = false
		}
		if result.warning {
			hasWarnings = true
		}

		// Update cost calculation
		if result.cost > 0 {
			totalEstimatedCost += result.cost
		} else if result.nodeReview.EstimatedCost == "Cost estimation unavailable" {
			nodeWithUnknownCost++
		}

		// Update resource summary maps (thread-safe since we're processing sequentially here)
		if result.specInfo != nil {
			specMap[req.NodeGroups[result.index].SpecId] = true
			connectionMap[result.specInfo.ConnectionName] = true
			providerMap[result.specInfo.ProviderName] = true
			regionMap[result.specInfo.RegionName] = true
		}

		if req.NodeGroups[result.index].ImageId != "" {
			imageMap[req.NodeGroups[result.index].ImageId] = true
		}
	}

	// Store VM reviews in result
	reviewResult.NodeReviews = nodeReviews

	// Build resource summary
	for spec := range specMap {
		reviewResult.ResourceSummary.UniqueSpecs = append(reviewResult.ResourceSummary.UniqueSpecs, spec)
	}
	for image := range imageMap {
		reviewResult.ResourceSummary.UniqueImages = append(reviewResult.ResourceSummary.UniqueImages, image)
	}
	for conn := range connectionMap {
		reviewResult.ResourceSummary.ConnectionNames = append(reviewResult.ResourceSummary.ConnectionNames, conn)
	}
	for provider := range providerMap {
		reviewResult.ResourceSummary.ProviderNames = append(reviewResult.ResourceSummary.ProviderNames, provider)
	}
	for region := range regionMap {
		reviewResult.ResourceSummary.RegionNames = append(reviewResult.ResourceSummary.RegionNames, region)
	}

	reviewResult.ResourceSummary.TotalProviders = len(providerMap)
	reviewResult.ResourceSummary.TotalRegions = len(regionMap)

	// Count available/unavailable resources
	for _, nodeReview := range reviewResult.NodeReviews {
		if nodeReview.SpecValidation.IsAvailable {
			reviewResult.ResourceSummary.AvailableSpecs++
		} else {
			reviewResult.ResourceSummary.UnavailableSpecs++
		}
		if nodeReview.ImageValidation.IsAvailable {
			reviewResult.ResourceSummary.AvailableImages++
		} else {
			reviewResult.ResourceSummary.UnavailableImages++
		}
	}

	// Set overall status and cost estimation
	if totalEstimatedCost > 0 {
		if nodeWithUnknownCost > 0 {
			reviewResult.EstimatedCost = fmt.Sprintf("$%.4f/hour (partial - %d VMs have unknown costs)", totalEstimatedCost, nodeWithUnknownCost)
		} else {
			reviewResult.EstimatedCost = fmt.Sprintf("$%.4f/hour", totalEstimatedCost)
		}
	} else if nodeWithUnknownCost > 0 {
		reviewResult.EstimatedCost = fmt.Sprintf("Cost estimation unavailable for all %d VMs", nodeWithUnknownCost)
	}

	reviewResult.CreationViable = allViable

	if !allViable {
		reviewResult.OverallStatus = "Error"
		reviewResult.OverallMessage = fmt.Sprintf("Infra cannot be created due to critical errors in VM configurations (Providers: %v, Regions: %v)",
			reviewResult.ResourceSummary.ProviderNames, reviewResult.ResourceSummary.RegionNames)
		reviewResult.Recommendations = append(reviewResult.Recommendations, "Fix all VM configuration errors before attempting to create Infra")
	} else if hasWarnings {
		reviewResult.OverallStatus = "Warning"
		reviewResult.OverallMessage = fmt.Sprintf("Infra can be created but has some configuration warnings (Providers: %v, Regions: %v)",
			reviewResult.ResourceSummary.ProviderNames, reviewResult.ResourceSummary.RegionNames)
		reviewResult.Recommendations = append(reviewResult.Recommendations, "Review and address warnings for optimal configuration")
	} else {
		reviewResult.OverallStatus = "Ready"
		reviewResult.OverallMessage = fmt.Sprintf("All VMs can be created successfully (Providers: %v, Regions: %v)",
			reviewResult.ResourceSummary.ProviderNames, reviewResult.ResourceSummary.RegionNames)
	}

	// Add specific recommendations
	if reviewResult.ResourceSummary.TotalProviders > 3 {
		reviewResult.Recommendations = append(reviewResult.Recommendations, "Consider consolidating to fewer cloud providers to simplify management")
	}
	if reviewResult.ResourceSummary.TotalRegions > 5 {
		reviewResult.Recommendations = append(reviewResult.Recommendations, "Large number of regions may increase latency between VMs")
	}
	if totalEstimatedCost > 10.0 {
		reviewResult.Recommendations = append(reviewResult.Recommendations, "High estimated cost - consider using smaller instance types if appropriate")
	}
	if nodeWithUnknownCost > 0 {
		reviewResult.Recommendations = append(reviewResult.Recommendations, fmt.Sprintf("Cost estimation unavailable for %d VMs - actual costs may be higher than shown", nodeWithUnknownCost))
	}

	// Add PolicyOnPartialFailure analysis and recommendations
	policy := req.PolicyOnPartialFailure
	if policy == "" {
		policy = model.PolicyContinue // default value
		reviewResult.PolicyOnPartialFailure = model.PolicyContinue
	}

	var policyDescription, policyRecommendation string

	switch policy {
	case model.PolicyContinue:
		policyDescription = "If some VMs fail during creation, Infra will be created with successfully provisioned VMs only. Failed VMs will remain in 'StatusFailed' state and can be fixed later using 'refine' action."
		reviewResult.Recommendations = append(reviewResult.Recommendations,
			"Failure Policy: 'continue' - Partial deployment allowed, failed VMs can be refined later")
		if reviewResult.TotalNodeCount > 1 {
			policyRecommendation = "With multiple VMs, consider 'rollback' policy for all-or-nothing deployment, or 'refine' policy for automatic cleanup"
			reviewResult.Recommendations = append(reviewResult.Recommendations,
				"With multiple VMs, partial failures are possible. Consider using 'rollback' policy if you need all-or-nothing deployment, or 'refine' policy for automatic cleanup of failed VMs.")
		}
	case model.PolicyRollback:
		policyDescription = "If any VM fails during creation, the entire Infra will be deleted automatically. This ensures all-or-nothing deployment but may waste resources if only a few VMs fail."
		reviewResult.Recommendations = append(reviewResult.Recommendations,
			"Failure Policy: 'rollback' - All-or-nothing deployment, entire Infra deleted on any failure")
		if reviewResult.TotalNodeCount > 5 {
			policyRecommendation = "With many VMs, rollback policy increases risk of complete deployment failure. Consider 'continue' or 'refine' policy for better reliability"
			reviewResult.Recommendations = append(reviewResult.Recommendations,
				"WARNING: With many VMs, rollback policy increases risk of complete deployment failure. Consider 'continue' or 'refine' policy for better reliability.")
		}
		if reviewResult.ResourceSummary.TotalProviders > 2 {
			reviewResult.Recommendations = append(reviewResult.Recommendations,
				"WARNING: Multiple cloud providers increase failure probability. Rollback policy may cause complete deployment failure due to single provider issues.")
		}
	case model.PolicyRefine:
		policyDescription = "If some VMs fail during creation, Infra will be created with successful VMs, and failed VMs will be automatically cleaned up using refine action. This provides the best balance between reliability and resource efficiency."
		reviewResult.Recommendations = append(reviewResult.Recommendations,
			"Failure Policy: 'refine' - Automatic cleanup of failed VMs, optimal balance of reliability and efficiency")
		if reviewResult.TotalNodeCount > 10 {
			policyRecommendation = "With many VMs, 'refine' policy provides optimal balance between reliability and resource efficiency"
			reviewResult.Recommendations = append(reviewResult.Recommendations,
				"RECOMMENDED: With many VMs, 'refine' policy provides optimal balance between reliability and resource efficiency.")
		}
	default:
		policyDescription = fmt.Sprintf("Unknown failure policy '%s'. Will default to 'continue'. Valid options: continue, rollback, refine", policy)
		policyRecommendation = "Use one of the valid failure policies: continue, rollback, refine"
		reviewResult.Recommendations = append(reviewResult.Recommendations,
			fmt.Sprintf("WARNING: Unknown failure policy '%s'. Will default to 'continue'. Valid options: continue, rollback, refine", policy))
	}

	reviewResult.PolicyDescription = policyDescription
	reviewResult.PolicyRecommendation = policyRecommendation

	// Add policy-specific warnings based on deployment context
	if reviewResult.OverallStatus == "Warning" && policy == model.PolicyRollback {
		reviewResult.Recommendations = append(reviewResult.Recommendations,
			"CAUTION: Configuration warnings detected with 'rollback' policy. Address warnings to prevent complete deployment failure.")
	}

	if len(reviewResult.ResourceSummary.ProviderNames) > 1 && policy == model.PolicyRollback {
		reviewResult.Recommendations = append(reviewResult.Recommendations,
			"TIP: Multi-cloud deployment with 'rollback' policy is risky. Consider 'refine' policy for better fault tolerance across providers.")
	}

	if deployOption == "hold" {
		reviewResult.Recommendations = append(reviewResult.Recommendations,
			fmt.Sprintf("DEPLOYMENT HOLD: Infra creation will be held for review. Failure policy '%s' will apply when deployment is resumed with control continue.", policy))
	}

	// Add provider-specific global recommendations
	for _, providerName := range reviewResult.ResourceSummary.ProviderNames {
		switch csp.ResolveCloudPlatform(providerName) {
		case csp.KT:
			reviewResult.Recommendations = append(reviewResult.Recommendations,
				"NOTICE: KT Cloud provisioning is currently limited to specs with '.itl' in the name (temporary limitation)")
			// case csp.NHN:
			// 	if deployOption != "hold" {
			// 		reviewResult.Recommendations = append(reviewResult.Recommendations,
			// 			"CRITICAL: NHN Cloud requires deployOption 'hold' for manual deployment - automatic provisioning will fail")
			// 	} else {
			// 		reviewResult.Recommendations = append(reviewResult.Recommendations,
			// 			"INFO: NHN Cloud deployment will be held for manual completion - automatic provisioning is not fully supported")
			// 	}
		}
	}

	log.Debug().Msgf("Infra review completed: %s - %s (Policy: %s)", reviewResult.OverallStatus, reviewResult.OverallMessage, policy)
	return reviewResult, nil
}

// CreateSystemInfraDynamic is func to create Infra obeject and deploy requested VMs in a dynamic way
