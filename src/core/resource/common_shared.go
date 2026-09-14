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

package resource

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/rs/zerolog/log"
)

// SharedResourceOptions contains optional parameters for creating shared resources
type SharedResourceOptions struct {
	// Zone specifies the availability zone for subnet placement.
	// If specified, the subnet will be created in this zone.
	// Useful for GPU VMs or other resources only available in specific zones.
	// If empty, auto-selection based on connection config applies.
	Zone string

	// CredentialHolder specifies the credential holder for filtering connection configs.
	// If empty, defaults to model.DefaultCredentialHolder ("admin").
	CredentialHolder string

	// VNetTemplateId specifies the vNet template ID (from system namespace) to use instead
	// of the default hard-coded CIDR generation. If the template is not found, falls back
	// to the default behavior.
	VNetTemplateId string

	// SgTemplateId specifies the SecurityGroup template ID (from system namespace) to use
	// instead of the default all-open firewall rules. If the template is not found, falls
	// back to the default behavior.
	SgTemplateId string

	// PreferredZones, when set, tells VNet creation to place the initial subnets in these zones
	// (typically the zones where the requested spec is offered) instead of the first-N zones of
	// the region. Ignored when a zone is explicitly requested, or for CSPs without zonal subnets.
	PreferredZones []string

	// InfraId, when set, makes the SecurityGroup dedicated to a specific Infra
	// (named "{infraId}-{connection}[-zone][-sgTemplateId]") instead of shared across the
	// connection, and labels it (sys.infraId, sys.purpose) so the unused-resource release
	// operation can reclaim it once no VMs reference it. VNet and SSHKey remain shared
	// regardless of this field.
	InfraId string

	// NodeGroupName, when set (together with InfraId), scopes the SecurityGroup to a single
	// NodeGroup and names it "{infraId}-{nodeGroupName}" (unique within the Infra). Applications
	// are deployed per NodeGroup, so their firewall rules can then be opened per NodeGroup without
	// affecting sibling NodeGroups on the same connection. It also labels the SG with
	// sys.nodeGroupId for traceability. Only affects SecurityGroup naming; SSHKey stays per-Infra.
	NodeGroupName string
}

// applyVNetPolicy converts a CSP-agnostic VNetPolicy into concrete VNetReq fields on reqTmp,
// applying CSP-specific constraints automatically:
//
//   - IBM:  always capped to 1 subnet (VPC Address Prefix limitation in CB-Spider)
//   - NCP:  all subnets forced to the same zone (K8s cluster requirement)
//   - GCP:  VPC-level CidrBlock left empty (GCP assigns CIDR at subnet level)
//   - Others: up to 2 subnets placed in different zones when multiZone=true and the region has ≥ 2 zones
//
// CIDR assignment:
//   - policy.CidrBlock == "auto" → 10.{sliceIndex}.0.0/16 (same algorithm as default hard-coded path)
//   - explicit CIDR              → used as-is
//
// explicitZone overrides zone selection for all subnets (e.g. when a GPU VM requires a specific zone).
func applyVNetPolicy(nsId string, reqTmp *model.VNetReq, policy *model.VNetPolicy, provider, connectionName string, sliceIndex int, explicitZone string, preferredZones []string) error {
	resolvedProvider := csp.ResolveCloudPlatform(provider)

	// Determine effective subnet count respecting CSP limits
	subnetCount := policy.SubnetCount
	if subnetCount <= 0 {
		subnetCount = 1
	}
	if resolvedProvider == csp.IBM {
		// IBM VPC requires zone-specific Address Prefix setup; CB-Spider uses the same CIDR for all zones
		// which causes conflicts when multiple subnets/zones are created.
		subnetCount = 1
		log.Info().Msg("IBM VPC: capping subnet count to 1 (Address Prefix limitation)")
	} else if subnetCount > 2 {
		subnetCount = 2
		log.Info().Msgf("Capping subnet count to 2 (requested %d)", policy.SubnetCount)
	}

	// GCP has no VPC-level CIDR: its subnets carry their own CIDRs, and CB-Spider rejects a
	// VPC CIDR for GCP (it returns "GCP VPC does not support IPv4_CIDR"). Omit it for GCP so
	// CB-Tumblebug does not store/display a CIDR that is never actually applied. Verified:
	// CB-Spider creates a GCP VPC successfully with an empty vNet CIDR. (The dynamic
	// provisioning path calls CreateVNet directly, which does not run the REST-only
	// ValidateVNetReq CIDR check, so the empty GCP CIDR reaches CB-Spider and succeeds.)
	if resolvedProvider != csp.GCP {
		if policy.CidrBlock == "auto" || policy.CidrBlock == "" {
			reqTmp.CidrBlock = "10." + strconv.Itoa(sliceIndex) + ".0.0/16"
		} else {
			reqTmp.CidrBlock = policy.CidrBlock
		}
	}

	// NCP: all subnets must reside in the same zone
	multiZone := policy.MultiZone
	if resolvedProvider == csp.NCP {
		multiZone = false
		log.Info().Msg("NCP: disabling multi-zone (all subnets must be in the same zone)")
	}

	// Resolve zone(s) for subnet placement.
	zones, zoneCount, _ := GetFirstNZones(connectionName, 2)

	// When the spec's offering zones are known (preferredZones) and no explicit zone is requested,
	// place subnets in those zones so we don't create subnets in zones the spec can't use. One
	// subnet per offering zone, capped by subnetCount. preferredZones is empty for CSPs without a
	// per-zone availability checker or with regional subnets, so those keep the default behavior.
	usePreferred := len(preferredZones) > 0 && explicitZone == ""
	if usePreferred {
		zones = preferredZones
		zoneCount = len(preferredZones)
		if subnetCount > zoneCount {
			subnetCount = zoneCount
		}
	}

	connConfig, err := common.GetConnConfig(connectionName)
	if err != nil {
		return fmt.Errorf("failed to get connection config for '%s': %w", connectionName, err)
	}
	assignedZone := connConfig.RegionZoneInfo.AssignedZone

	// NCP always needs an explicit zone assignment
	if resolvedProvider == csp.NCP && explicitZone == "" && assignedZone != "" {
		explicitZone = assignedZone
	}
	if resolvedProvider == csp.NCP && explicitZone == "" && zoneCount > 0 {
		explicitZone = zones[0]
	}

	// Build subnets
	subnetCidrs := []string{
		"10." + strconv.Itoa(sliceIndex) + ".0.0/18",
		"10." + strconv.Itoa(sliceIndex) + ".64.0/18",
	}
	subnetNames := []string{reqTmp.Name, reqTmp.Name + "-01"}

	for i := 0; i < subnetCount; i++ {
		subnet := model.SubnetReq{
			Name:      subnetNames[i],
			IPv4_CIDR: subnetCidrs[i],
		}
		// Zone assignment
		if usePreferred {
			// One subnet per spec-available zone (authoritative when known).
			subnet.Zone = zones[i]
		} else if explicitZone != "" {
			if i == 0 || !multiZone {
				subnet.Zone = explicitZone
			} else {
				// Find a zone different from explicitZone for second subnet
				secondZone := explicitZone
				for _, z := range zones {
					if z != explicitZone {
						secondZone = z
						break
					}
				}
				subnet.Zone = secondZone
			}
		} else if assignedZone != "" {
			if !multiZone || i == 0 {
				subnet.Zone = assignedZone
			} else if zoneCount > 1 {
				subnet.Zone = zones[1]
			} else {
				subnet.Zone = assignedZone
			}
		} else if multiZone && zoneCount > 1 {
			subnet.Zone = zones[i]
		}

		reqTmp.SubnetInfoList = append(reqTmp.SubnetInfoList, subnet)
	}

	return nil
}

// GenVNetResourceName returns the VNet resource name used by dynamic provisioning, honoring the
// resolved VNet template's isolation policy: dedicated per-Infra ("{infraId}-{conn}[-zone]") when
// the template's vNetPolicy.Dedicated is set (and infraId is non-empty), otherwise shared
// ("{ns}-shared-{conn}[-zone]"). A template-id suffix is appended when a non-default template is
// explicitly requested so different templates yield independent VNets. This is the single source
// of the VNet name so provisioning and shared-resource creation always agree (SG references the
// VNet by this name). Falls back to the shared name if the template cannot be read (creation then
// fails with a clear error).
func GenVNetResourceName(nsId, infraId, connectionName, zone, vNetTemplateId string) string {
	effectiveId := model.DefaultVNetTemplateId
	if vNetTemplateId != "" {
		effectiveId = vNetTemplateId
	}
	dedicated := false
	tmpl, err := common.GetVNetTemplate(nsId, effectiveId)
	if err != nil {
		tmpl, err = common.GetVNetTemplate(model.SystemCommonNs, effectiveId)
	}
	if err == nil && tmpl.VNetPolicy != nil && tmpl.VNetPolicy.Dedicated {
		dedicated = true
	}

	name := nsId + model.StrSharedResourceName + connectionName
	if dedicated && infraId != "" {
		name = infraId + "-" + connectionName
	}
	if zone != "" {
		name = name + "-" + zone
	}
	if vNetTemplateId != "" {
		name = name + "-" + vNetTemplateId
	}
	return name
}

// CreateSharedResource is to register default resource from asset files (../assets/*.csv)
// This is a wrapper function that maintains backward compatibility
func CreateSharedResource(ctx context.Context, nsId string, resType string, connectionName string) error {
	return CreateSharedResourceWithOptions(ctx, nsId, resType, connectionName, nil)
}

// CreateSharedResourceWithOptions creates shared resources with optional parameters
func CreateSharedResourceWithOptions(ctx context.Context, nsId string, resType string, connectionName string, options *SharedResourceOptions) error {

	// Check 'nsId' namespace.
	_, err := common.GetNs(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	var resList []string
	if resType == "all" {
		// ORDER MATTERS: SecurityGroup references the VNet by name (reqTmp.VNetId = vNetResourceName).
		// VNet must be created before SecurityGroup. SSHKey is independent but placed between
		// them to allow the VNet-to-SecurityGroup dependency to always be satisfied.
		// Do NOT reorder without updating the SecurityGroup creation logic below.
		resList = append(resList, model.StrVNet)
		resList = append(resList, model.StrSSHKey)
		resList = append(resList, model.StrSecurityGroup)
	} else {
		resList = append(resList, resType)
	}

	// Determine credential holder from options, fallback to default
	credentialHolder := model.DefaultCredentialHolder
	if options != nil && options.CredentialHolder != "" {
		credentialHolder = options.CredentialHolder
	}

	// Read default resources from file and create objects

	connectionList, err := common.GetConnConfigList(credentialHolder, true, true)
	if err != nil {
		log.Error().Err(err).Msg("Cannot GetConnConfig")
		return err
	}
	sliceIndex := -1
	provider := ""
	for i, connConfig := range connectionList.Connectionconfig {
		if connConfig.ConfigName == connectionName {
			log.Info().Msgf("[%d] connectionName: %s", i, connectionName)
			sliceIndex = i
			provider = strings.ToLower(connConfig.ProviderName)
		}
	}
	if sliceIndex == -1 {
		err := fmt.Errorf("cannot find the connection config: %s", connectionName)
		log.Error().Err(err).Msg("Failed to CreateSharedResource")
		return err
	}
	sliceIndex = (sliceIndex % 254) + 1

	// Base shared resource name: nsId + "-shared-" + connectionName [+ "-" + zone]
	baseResourceName := nsId + model.StrSharedResourceName + connectionName
	if options != nil && options.Zone != "" {
		baseResourceName = baseResourceName + "-" + options.Zone
		log.Info().Msgf("Using zone-specific shared resource name: %s (zone: %s)", baseResourceName, options.Zone)
	}

	// VNet resource name: shared per-connection by default, or dedicated per-Infra when the
	// resolved VNet template requests it. Centralized in GenVNetResourceName so the SG's VNetId
	// reference (computed in the SG branch/another call) always matches the actual VNet name.
	optInfraId, optVNetTemplateId, optZone := "", "", ""
	if options != nil {
		optInfraId = options.InfraId
		optVNetTemplateId = options.VNetTemplateId
		optZone = options.Zone
	}
	vNetResourceName := GenVNetResourceName(nsId, optInfraId, connectionName, optZone, optVNetTemplateId)

	// SG resource name, most-specific first:
	//   - per-NodeGroup ("{infraId}-{nodeGroupName}") when NodeGroupName is set, so app-specific
	//     firewall rules stay scoped to the NodeGroup deploying the app. NodeGroup names are unique
	//     within an Infra, so no connection/zone/template suffix is needed for uniqueness.
	//   - per-Infra ("{infraId}-{conn}[-zone][-sgTemplateId]") when only InfraId is set.
	//   - shared per-connection ("{ns}-shared-{conn}[-zone][-sgTemplateId]") otherwise.
	sgResourceName := baseResourceName
	if options != nil && options.InfraId != "" && options.NodeGroupName != "" {
		sgResourceName = options.InfraId + "-" + options.NodeGroupName
	} else {
		sgBaseName := baseResourceName
		if options != nil && options.InfraId != "" {
			sgBaseName = options.InfraId + "-" + connectionName
			if options.Zone != "" {
				sgBaseName = sgBaseName + "-" + options.Zone
			}
		}
		sgResourceName = sgBaseName
		if options != nil && options.SgTemplateId != "" {
			sgResourceName = sgBaseName + "-" + options.SgTemplateId
		}
	}

	// SSHKey resource name: dedicated per-Infra when InfraId is set ("{infraId}-{conn}[-zone]")
	// for credential isolation, otherwise shared per-connection. No template support.
	resourceName := baseResourceName
	if options != nil && options.InfraId != "" {
		resourceName = options.InfraId + "-" + connectionName
		if options.Zone != "" {
			resourceName = resourceName + "-" + options.Zone
		}
	}

	description := "Generated Default Resource"

	for _, resType := range resList {
		if strings.EqualFold(resType, model.StrVNet) {
			log.Debug().Msg(model.StrVNet)

			reqTmp := model.VNetReq{}
			reqTmp.ConnectionName = connectionName
			reqTmp.Name = vNetResourceName
			reqTmp.Description = description

			// Resolve the VNet structure from a template (single source of the default policy).
			// When no template is requested, fall back to the default template id. A missing
			// template is a hard error (no silent hard-coded fallback); the InfraDynamic review
			// stage checks this up front. Lookup order: user namespace first, then system.
			effectiveVNetTemplateId := model.DefaultVNetTemplateId
			if options != nil && options.VNetTemplateId != "" {
				effectiveVNetTemplateId = options.VNetTemplateId
			}
			var explicitZone string
			if options != nil {
				explicitZone = options.Zone
			}

			var tmplFound bool
			var tmpl model.VNetTemplateInfo
			if nsId != model.SystemCommonNs {
				if t, err := common.GetVNetTemplate(nsId, effectiveVNetTemplateId); err == nil {
					tmpl = t
					tmplFound = true
					log.Info().Msgf("VNet template '%s' found in user namespace '%s'", effectiveVNetTemplateId, nsId)
				}
			}
			if !tmplFound {
				if t, err := common.GetVNetTemplate(model.SystemCommonNs, effectiveVNetTemplateId); err != nil {
					log.Warn().Err(err).Msgf("VNet template '%s' not found in namespace '%s' or system namespace", effectiveVNetTemplateId, nsId)
				} else {
					tmpl = t
					tmplFound = true
					log.Info().Msgf("VNet template '%s' found in system namespace", effectiveVNetTemplateId)
				}
			}
			if !tmplFound {
				return fmt.Errorf("VNet template '%s' not found in namespace '%s' or system namespace; "+
					"load it before provisioning (e.g. run 'make init' to load init/templates, or register it via the template API)", effectiveVNetTemplateId, nsId)
			}

			// Build the VNet request from the template (policy = CSP-agnostic intent auto-resolved
			// to CSP-specific details; raw = explicit structure as-is).
			if tmpl.VNetPolicy != nil {
				log.Info().Msgf("Using VNet policy template '%s' for connection '%s'", effectiveVNetTemplateId, connectionName)
				var preferredZones []string
				if options != nil {
					preferredZones = options.PreferredZones
				}
				if err := applyVNetPolicy(nsId, &reqTmp, tmpl.VNetPolicy, provider, connectionName, sliceIndex, explicitZone, preferredZones); err != nil {
					log.Error().Err(err).Msgf("Failed to apply VNet policy from template '%s'", effectiveVNetTemplateId)
					return err
				}
			} else if tmpl.VNetReq != nil {
				log.Info().Msgf("Using VNet raw template '%s' for connection '%s'", effectiveVNetTemplateId, connectionName)
				reqTmp.CidrBlock = tmpl.VNetReq.CidrBlock
				reqTmp.SubnetInfoList = tmpl.VNetReq.SubnetInfoList
				// Apply explicit zone to subnets that don't already have a zone assigned
				if explicitZone != "" {
					for i := range reqTmp.SubnetInfoList {
						if reqTmp.SubnetInfoList[i].Zone == "" {
							reqTmp.SubnetInfoList[i].Zone = explicitZone
						}
					}
				}
			} else {
				return fmt.Errorf("VNet template '%s' has neither vNetPolicy nor vNetReq defined", effectiveVNetTemplateId)
			}

			common.PrintJsonPretty(reqTmp)
			resultInfo, err := CreateVNet(ctx, nsId, &reqTmp)
			// KT has a single account-wide VPC whose tiers persist; a leftover tier with the same
			// auto-assigned CIDR yields 409 "duplicate network cidr" — retry with a different /16.
			for attempt := 1; err != nil && tmpl.VNetPolicy != nil && isDuplicateCidrError(err) && attempt <= 3; attempt++ {
				altIndex := (sliceIndex+attempt*61)%254 + 1
				log.Warn().Msgf("vNet CIDR conflict at the CSP for connection '%s' (attempt %d); retrying with 10.%d.0.0/16", connectionName, attempt, altIndex)
				reqTmp.SubnetInfoList = nil
				if perr := applyVNetPolicy(nsId, &reqTmp, tmpl.VNetPolicy, provider, connectionName, altIndex, explicitZone, preferredZonesOf(options)); perr != nil {
					break
				}
				resultInfo, err = CreateVNet(ctx, nsId, &reqTmp)
			}
			if err != nil {
				log.Error().Err(err).Msgf("Failed to create vNet from template '%s'", effectiveVNetTemplateId)
				return err
			}
			common.PrintJsonPretty(resultInfo)

			// Tag a dedicated per-Infra VNet (sys.infraId, sys.purpose) so the unused-resource
			// release operation can reclaim it once no VMs reference it.
			if options != nil && options.InfraId != "" && tmpl.VNetPolicy != nil && tmpl.VNetPolicy.Dedicated {
				vnetKey := common.GenResourceKey(nsId, model.StrVNet, resultInfo.Id)
				extraLabels := map[string]string{
					model.LabelInfraId: options.InfraId,
					model.LabelPurpose: model.PurposeInfraDynamic,
				}
				if lblErr := label.CreateOrUpdateLabel(ctx, model.StrVNet, resultInfo.Uid, vnetKey, extraLabels); lblErr != nil {
					log.Warn().Err(lblErr).Msgf("Failed to label dedicated VNet '%s' with infraId/purpose", resultInfo.Id)
				}
			}
		} else if strings.EqualFold(resType, model.StrSecurityGroup) {
			log.Debug().Msg(model.StrSecurityGroup)

			reqTmp := model.SecurityGroupReq{}
			reqTmp.ConnectionName = connectionName
			reqTmp.Name = sgResourceName
			reqTmp.Description = description
			reqTmp.VNetId = vNetResourceName

			// Resolve the firewall policy from a SecurityGroup template so that the
			// default policy lives in a single editable source (init/templates/*.json,
			// loaded into the system namespace by `make init`) instead of being
			// hard-coded here. When no template is explicitly requested, fall back to
			// the default template id.
			// Lookup order: user namespace (nsId) first, then system namespace.
			effectiveSgTemplateId := model.DefaultSecurityGroupTemplateId
			if options != nil && options.SgTemplateId != "" {
				effectiveSgTemplateId = options.SgTemplateId
			}

			var sgTmplFound bool
			var sgTmpl model.SecurityGroupTemplateInfo

			// Try user namespace first
			if nsId != model.SystemCommonNs {
				t, err := common.GetSecurityGroupTemplate(nsId, effectiveSgTemplateId)
				if err == nil {
					sgTmpl = t
					sgTmplFound = true
					log.Info().Msgf("SecurityGroup template '%s' found in user namespace '%s'", effectiveSgTemplateId, nsId)
				}
			}
			// Fallback to system namespace
			if !sgTmplFound {
				t, err := common.GetSecurityGroupTemplate(model.SystemCommonNs, effectiveSgTemplateId)
				if err != nil {
					log.Warn().Err(err).Msgf("SecurityGroup template '%s' not found in namespace '%s' or system namespace", effectiveSgTemplateId, nsId)
				} else {
					sgTmpl = t
					sgTmplFound = true
					log.Info().Msgf("SecurityGroup template '%s' found in system namespace", effectiveSgTemplateId)
				}
			}

			// A missing SecurityGroup template is a hard error: there is no silent
			// all-open fallback. The InfraDynamic review stage checks this up front so
			// users can load the template before provisioning.
			if !sgTmplFound {
				return fmt.Errorf("SecurityGroup template '%s' not found in namespace '%s' or system namespace; "+
					"load it before provisioning (e.g. run 'make init' to load init/templates, or register it via the template API)", effectiveSgTemplateId, nsId)
			}

			log.Info().Msgf("Using SecurityGroup template '%s' for connection '%s'", effectiveSgTemplateId, connectionName)
			// Copy rules into a fresh slice so we never mutate the cached template,
			// resolving the "internal" CIDR keyword to the target VNet's own CIDR block
			// (resolved lazily so templates without the keyword incur no lookup).
			internalCidr := ""
			var ruleList []model.FirewallRuleReq
			if sgTmpl.SecurityGroupReq.FirewallRules != nil {
				for _, r := range *sgTmpl.SecurityGroupReq.FirewallRules {
					if strings.EqualFold(r.CIDR, model.FirewallCidrKeywordInternal) {
						if internalCidr == "" {
							// Prefer the actual VNet CIDR (covers vNet-template custom ranges), but
							// only if it is a valid CIDR: some CSPs have no VPC-level CIDR (e.g. GCP,
							// whose stored CidrBlock is a placeholder like "GCP VPC does not support
							// IPv4_CIDR"), so fall back to the generated /16 range that covers the
							// auto-assigned subnets in that case.
							internalCidr = fmt.Sprintf("10.%d.0.0/16", sliceIndex)
							if vNetInfo, err := GetVNet(nsId, vNetResourceName); err != nil {
								log.Warn().Err(err).Msgf("Could not read VNet '%s' for 'internal' keyword; using generated range '%s'", vNetResourceName, internalCidr)
							} else if _, _, perr := net.ParseCIDR(vNetInfo.CidrBlock); perr == nil {
								internalCidr = vNetInfo.CidrBlock
							}
						}
						r.CIDR = internalCidr
					}
					ruleList = append(ruleList, r)
				}
			}
			reqTmp.FirewallRules = &ruleList

			common.PrintJsonPretty(reqTmp)
			resultInfo, err := CreateSecurityGroup(ctx, nsId, &reqTmp, "")
			if err != nil {
				log.Error().Err(err).Msgf("Failed to create SecurityGroup (template: '%s')", effectiveSgTemplateId)
				return err
			}
			common.PrintJsonPretty(resultInfo)

			// Tag a dedicated per-Infra SG (sys.infraId, sys.purpose) so the unused-resource
			// release operation can reclaim it once no VMs reference it. Labels merge with the
			// base labels set by CreateSecurityGroup.
			if options != nil && options.InfraId != "" {
				sgKey := common.GenResourceKey(nsId, model.StrSecurityGroup, resultInfo.Id)
				extraLabels := map[string]string{
					model.LabelInfraId: options.InfraId,
					model.LabelPurpose: model.PurposeInfraDynamic,
				}
				// Tag the owning NodeGroup for per-NodeGroup SGs (traceability / label queries).
				if options.NodeGroupName != "" {
					extraLabels[model.LabelNodeGroupId] = options.NodeGroupName
				}
				if lblErr := label.CreateOrUpdateLabel(ctx, model.StrSecurityGroup, resultInfo.Uid, sgKey, extraLabels); lblErr != nil {
					log.Warn().Err(lblErr).Msgf("Failed to label dedicated SG '%s' with infraId/purpose", resultInfo.Id)
				}
			}

		} else if strings.EqualFold(resType, model.StrSSHKey) {
			log.Debug().Msg(model.StrSSHKey)

			reqTmp := model.SshKeyReq{}

			reqTmp.ConnectionName = connectionName
			reqTmp.Name = resourceName
			reqTmp.Description = description

			common.PrintJsonPretty(reqTmp)

			resultInfo, err := CreateSshKey(ctx, nsId, &reqTmp, "")
			if err != nil {
				log.Error().Err(err).Msg("Failed to create SshKey")
				return err
			}

			// Tag a dedicated per-Infra SSHKey (sys.infraId, sys.purpose) so the unused-resource
			// release operation can reclaim it once no VMs reference it. Labels merge with the
			// base labels set by CreateSshKey.
			if options != nil && options.InfraId != "" {
				keyKey := common.GenResourceKey(nsId, model.StrSSHKey, resultInfo.Id)
				extraLabels := map[string]string{
					model.LabelInfraId: options.InfraId,
					model.LabelPurpose: model.PurposeInfraDynamic,
				}
				if lblErr := label.CreateOrUpdateLabel(ctx, model.StrSSHKey, resultInfo.Uid, keyKey, extraLabels); lblErr != nil {
					log.Warn().Err(lblErr).Msgf("Failed to label dedicated SSHKey '%s' with infraId/purpose", resultInfo.Id)
				}
			}
		} else {
			return errors.New("Not valid option (provide sg, sshkey, vnet, or all)")
		}
	}

	return nil
}

// DeleteSharedResources releases auto-generated resources that are no longer referenced:
//   - shared resources named "{nsId}-shared-...": SecurityGroup, SSHKey, vNet
//   - per-Infra dedicated SecurityGroups (label sys.purpose=infra-dynamic-sg), which become
//     orphaned once their Infra is deleted
//
// Only resources with NO associated objects (per CB-TB records) are removed, so user-created
// or still-in-use resources are preserved (forceFlag=false is used as an extra guard). When
// dryRun is true nothing is deleted and the result lists what WOULD be removed.
//
// Deletion order is SecurityGroup -> SSHKey -> vNet so that a shared vNet is only removed
// after the SGs referencing it are gone (avoids CSP DependencyViolation).
func DeleteSharedResources(nsId string, dryRun bool) (model.ResourceDeleteResults, error) {

	output := model.ResourceDeleteResults{}
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return output, err
	}

	matchedSubstring := nsId + model.StrSharedResourceName

	// filterShared returns resource IDs of a type whose name marks them as shared/auto-generated.
	filterShared := func(resourceType string) []string {
		ids, err := ListResourceId(nsId, resourceType)
		if err != nil {
			log.Warn().Err(err).Msgf("Failed to list %s for shared-resource release", resourceType)
			return nil
		}
		var matched []string
		for _, id := range ids {
			if strings.Contains(id, matchedSubstring) {
				matched = append(matched, id)
			}
		}
		return matched
	}

	// dedicatedSelector matches per-Infra auto-generated resources (SG / SSHKey).
	dedicatedSelector := model.LabelPurpose + "=" + model.PurposeInfraDynamic + "," + model.LabelNamespace + "=" + nsId

	// SecurityGroup candidates: shared-named + per-Infra dedicated (identified by label).
	sgIdSet := make(map[string]bool)
	for _, id := range filterShared(model.StrSecurityGroup) {
		sgIdSet[id] = true
	}
	if resources, lblErr := label.GetResourcesByLabelSelector(model.StrSecurityGroup, dedicatedSelector); lblErr != nil {
		log.Warn().Err(lblErr).Msg("Failed to list per-Infra dedicated SecurityGroups by label")
	} else {
		for _, r := range resources {
			if sg, ok := r.(*model.SecurityGroupInfo); ok {
				sgIdSet[sg.Id] = true
			}
		}
	}
	sgIds := make([]string, 0, len(sgIdSet))
	for id := range sgIdSet {
		sgIds = append(sgIds, id)
	}

	// SSHKey candidates: shared-named + per-Infra dedicated (identified by label).
	keyIdSet := make(map[string]bool)
	for _, id := range filterShared(model.StrSSHKey) {
		keyIdSet[id] = true
	}
	if resources, lblErr := label.GetResourcesByLabelSelector(model.StrSSHKey, dedicatedSelector); lblErr != nil {
		log.Warn().Err(lblErr).Msg("Failed to list per-Infra dedicated SSHKeys by label")
	} else {
		for _, r := range resources {
			if k, ok := r.(*model.SshKeyInfo); ok {
				keyIdSet[k.Id] = true
			}
		}
	}
	keyIds := make([]string, 0, len(keyIdSet))
	for id := range keyIdSet {
		keyIds = append(keyIds, id)
	}

	// VNet candidates: shared-named + per-Infra dedicated (identified by label).
	vnetIdSet := make(map[string]bool)
	for _, id := range filterShared(model.StrVNet) {
		vnetIdSet[id] = true
	}
	if resources, lblErr := label.GetResourcesByLabelSelector(model.StrVNet, dedicatedSelector); lblErr != nil {
		log.Warn().Err(lblErr).Msg("Failed to list per-Infra dedicated VNets by label")
	} else {
		for _, r := range resources {
			if v, ok := r.(*model.VNetInfo); ok {
				vnetIdSet[v.Id] = true
			}
		}
	}
	vnetIds := make([]string, 0, len(vnetIdSet))
	for id := range vnetIdSet {
		vnetIds = append(vnetIds, id)
	}

	// release deletes (or, in dryRun, reports) only resources with no associated objects.
	// release selects the deletable resources of one type (those with no associated objects),
	// then deletes them in parallel per CSP via the shared engine — while the caller keeps the
	// dependency-ordered type staging (SG -> SSHKey -> VNet) by calling release() per type.
	// The association check is done up front: types are staged, so by the time a type runs its
	// earlier-stage dependencies are already gone and the association state is stable.
	release := func(resourceType string, ids []string) {
		var deletable []string
		for _, id := range ids {
			assoc, assocErr := GetAssociatedObjectList(nsId, resourceType, id)
			if assocErr != nil {
				log.Warn().Err(assocErr).Msgf("Failed to check associations for %s '%s'; skipping", resourceType, id)
				continue
			}
			// Verify if the associated objects actually exist in kvstore.
			// Stale references from interrupted or deleted Infras must not block shared resource cleanup.
			var liveAssoc []string
			var deadAssoc []string
			for _, key := range assoc {
				if _, exists, _ := kvstore.Get(key); exists {
					liveAssoc = append(liveAssoc, key)
				} else {
					deadAssoc = append(deadAssoc, key)
				}
			}
			if len(liveAssoc) > 0 {
				// Still referenced by live resources -> preserve.
				if len(deadAssoc) > 0 && !dryRun {
					// Clean up the dead portion in the background
					_ = BatchRemoveFromAssociatedObjectList(nsId, resourceType, id, deadAssoc)
				}
				continue
			}
			if len(deadAssoc) > 0 && !dryRun {
				// All references were dead; prune them to keep metadata clean
				log.Info().Msgf("Pruning %d dead associations from %s '%s'", len(deadAssoc), resourceType, id)
				_ = BatchRemoveFromAssociatedObjectList(nsId, resourceType, id, deadAssoc)
			}
			if dryRun {
				output.Results = append(output.Results, model.ResourceDeleteResult{
					ResourceType: resourceType, ResourceId: id, Success: true,
					Message: "would be deleted (dry-run): no live associated objects",
				})
				continue
			}
			deletable = append(deletable, id)
		}
		if len(deletable) > 0 {
			output.Results = append(output.Results, deleteResourceIdsParallel(nsId, resourceType, deletable, "false")...)
		}
	}

	release(model.StrSecurityGroup, sgIds)
	release(model.StrSSHKey, keyIds)
	release(model.StrVNet, vnetIds)

	// Build summary counts
	successCount := 0
	failedCount := 0
	for _, r := range output.Results {
		if r.Success {
			successCount++
		} else {
			failedCount++
		}
	}
	output.Total = len(output.Results)
	output.SuccessCount = successCount
	output.FailedCount = failedCount

	return output, nil
}

// FindConnectionsWithSharedResources returns deduplicated connection names that
// have shared resources (VNet, SecurityGroup, SSHKey) in the given namespace.
// Shared resources follow the naming pattern: {nsId}-shared-{connectionName}.
func FindConnectionsWithSharedResources(nsId string) ([]string, error) {
	matchedSubstring := nsId + model.StrSharedResourceName
	connSet := make(map[string]bool)

	if rawList, err := ListResource(nsId, model.StrVNet, "", ""); err == nil {
		for _, item := range rawList.([]model.VNetInfo) {
			if strings.Contains(item.Name, matchedSubstring) && item.ConnectionName != "" {
				connSet[item.ConnectionName] = true
			}
		}
	}

	if rawList, err := ListResource(nsId, model.StrSecurityGroup, "", ""); err == nil {
		for _, item := range rawList.([]model.SecurityGroupInfo) {
			if strings.Contains(item.Name, matchedSubstring) && item.ConnectionName != "" {
				connSet[item.ConnectionName] = true
			}
		}
	}

	if rawList, err := ListResource(nsId, model.StrSSHKey, "", ""); err == nil {
		for _, item := range rawList.([]model.SshKeyInfo) {
			if strings.Contains(item.Name, matchedSubstring) && item.ConnectionName != "" {
				connSet[item.ConnectionName] = true
			}
		}
	}

	connections := make([]string, 0, len(connSet))
	for conn := range connSet {
		connections = append(connections, conn)
	}
	sort.Strings(connections)
	return connections, nil
}

// isDuplicateCidrError reports a CSP-side CIDR collision (e.g. KT "duplicate network cidr").
func isDuplicateCidrError(err error) bool {
	if err == nil {
		return false
	}
	m := strings.ToLower(err.Error())
	return strings.Contains(m, "duplicate network cidr") || strings.Contains(m, "cidr") && strings.Contains(m, "conflict")
}

// preferredZonesOf extracts preferred zones from shared-resource options (nil-safe).
func preferredZonesOf(options *SharedResourceOptions) []string {
	if options == nil {
		return nil
	}
	return options.PreferredZones
}
