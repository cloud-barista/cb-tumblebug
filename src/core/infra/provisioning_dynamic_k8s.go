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
	"strconv"
	"strings"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/rs/zerolog/log"
)

func filterCheckInfraDynamicReqInfoToCheckK8sClusterDynamicReqInfo(infraDReqInfo *model.CheckInfraDynamicReqInfo) *model.CheckK8sClusterDynamicReqInfo {
	k8sDReqInfo := model.CheckK8sClusterDynamicReqInfo{}

	if infraDReqInfo != nil {
		for _, k := range infraDReqInfo.ReqCheck {
			// Note: InfraType field is deprecated.
			// K8s minimum requirements (vCPU >= 2, Memory >= 4GB) are validated separately.

			imageListForK8s := []model.ImageInfo{}

			// Priority 1: Filter and prioritize K8s-optimized images
			for _, i := range k.Image {
				if i.IsKubernetesImage {
					imageListForK8s = append(imageListForK8s, i)
				}
			}

			// Priority 2: Fallback to all images if no K8s-optimized images available
			// This handles CSPs like Azure AKS where no dedicated K8s images exist
			if len(imageListForK8s) == 0 {
				log.Debug().Msg("No K8s-optimized images found, using all available images as fallback")
				imageListForK8s = k.Image
			}

			nodeDReqInfo := model.CheckNodeDynamicReqInfo{
				ConnectionConfigCandidates: k.ConnectionConfigCandidates,
				Spec:                       k.Spec,
				Region:                     k.Region,
				SystemMessage:              k.SystemMessage,
			}

			if len(imageListForK8s) > 0 {
				nodeDReqInfo.Image = imageListForK8s
			} else {
				// No available image because some CSP(ex. azure) can not specify an image
				nodeDReqInfo.Image = []model.ImageInfo{{Id: "default", Name: "default"}}
			}

			k8sDReqInfo.ReqCheck = append(k8sDReqInfo.ReqCheck, nodeDReqInfo)
		}
	}

	return &k8sDReqInfo
}

// CheckK8sClusterDynamicReq is func to check request info to create K8sCluster obeject and deploy requested Nodes in a dynamic way
func CheckK8sClusterDynamicReq(req *model.K8sClusterConnectionConfigCandidatesReq) (*model.CheckK8sClusterDynamicReqInfo, error) {
	if len(req.SpecIds) != 1 {
		err := fmt.Errorf("Only one SpecId should be defined.")
		log.Error().Err(err).Msg("")
		return &model.CheckK8sClusterDynamicReqInfo{}, err
	}

	infraCCCReq := model.InfraConnectionConfigCandidatesReq{
		SpecIds: req.SpecIds,
	}
	infraDReqInfo, err := CheckInfraDynamicReq(common.NewDefaultContext(), &infraCCCReq)

	k8sDReqInfo := filterCheckInfraDynamicReqInfoToCheckK8sClusterDynamicReqInfo(infraDReqInfo)

	return k8sDReqInfo, err
}

func getK8sRecommendVersion(providerName, regionName, reqVersion string) (string, error) {
	availableVersion, err := common.GetAvailableK8sVersion(providerName, regionName)
	if err != nil {
		err := fmt.Errorf("No available K8sCluster version.")
		log.Error().Err(err).Msg("")
		return "", err
	}

	recVersion := model.StrEmpty
	versionIdList := []string{}

	if reqVersion == "" {
		for _, verDetail := range *availableVersion {
			versionIdList = append(versionIdList, verDetail.Id)
			filteredRecVersion := common.FilterDigitsAndDots(recVersion)
			filteredAvailVersion := common.FilterDigitsAndDots(verDetail.Id)
			if common.CompareVersions(filteredRecVersion, filteredAvailVersion) < 0 {
				recVersion = verDetail.Id
			}
		}
	} else {
		for _, verDetail := range *availableVersion {
			versionIdList = append(versionIdList, verDetail.Id)
			if strings.EqualFold(reqVersion, verDetail.Id) {
				recVersion = verDetail.Id
				break
			} else {
				availVersion := common.FilterDigitsAndDots(verDetail.Id)
				filteredReqVersion := common.FilterDigitsAndDots(reqVersion)
				if strings.HasPrefix(availVersion, filteredReqVersion) {
					recVersion = availVersion
					break
				}
			}
		}
	}

	if strings.EqualFold(recVersion, model.StrEmpty) {
		return "", fmt.Errorf("Available K8sCluster Version(k8sclusterinfo.yaml) for Provider/Region(%s/%s): %s",
			providerName, regionName, strings.Join(versionIdList, ", "))
	}

	return recVersion, nil
}

// checkCommonResAvailableForK8sClusterDynamicReq is func to check common resources availability for K8sClusterDynamicReq
func checkCommonResAvailableForK8sClusterDynamicReq(ctx context.Context, dReq *model.K8sClusterDynamicReq) error {
	specInfo, err := resource.GetSpec(model.SystemCommonNs, dReq.SpecId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	connName := specInfo.ConnectionName
	// If ConnectionName is specified by the request, Use ConnectionName from the request
	if dReq.ConnectionName != "" {
		connName = dReq.ConnectionName
	}

	// validate the GetConnConfig for spec
	connConfig, err := common.GetConnConfig(connName)
	if err != nil {
		err := fmt.Errorf("Failed to get ConnectionName (%s) for Spec (%s) is not found.", connName, dReq.SpecId)
		log.Error().Err(err).Msg("")
		return err
	}

	niDesignation, err := common.GetK8sNodeImageDesignation(connConfig.ProviderName)
	if err != nil {
		log.Error().Err(err).Msg("")
	}

	if niDesignation == false {
		// if node image designation is not supported by CSP, auto-correct ImageId to "default"
		if !(strings.EqualFold(dReq.ImageId, "default") || strings.EqualFold(dReq.ImageId, "")) {
			log.Warn().Msgf("NodeImageDesignation is not supported by CSP(%s). ImageId '%s' will be replaced with 'default'", connConfig.ProviderName, dReq.ImageId)
			dReq.ImageId = "default"
		}
	}

	// In K8sCluster, allows dReq.ImageId to be set to "default" or ""
	if strings.EqualFold(dReq.ImageId, "default") ||
		strings.EqualFold(dReq.ImageId, "") {
		// do nothing
	} else {
		// Check if the image is available (DB or CSP) and auto-register if needed
		_, isAutoRegistered, err := resource.EnsureImageAvailable(ctx, model.SystemCommonNs, connName, dReq.ImageId)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get the Image from the CSP")
			return err
		}
		if isAutoRegistered {
			log.Info().Msgf("Image '%s' was auto-registered from CSP for K8sCluster", dReq.ImageId)
		}
	}

	return nil
}

// checkCommonResAvailableForK8sNodeGroupDynamicReq is func to check common resources availability for K8sNodeGroupDynamicReq
func checkCommonResAvailableForK8sNodeGroupDynamicReq(ctx context.Context, connName string, dReq *model.K8sNodeGroupDynamicReq) error {
	k8sClusterDReq := &model.K8sClusterDynamicReq{
		SpecId:         dReq.SpecId,
		ImageId:        dReq.ImageId,
		ConnectionName: connName,
	}

	err := checkCommonResAvailableForK8sClusterDynamicReq(ctx, k8sClusterDReq)
	if err != nil {
		return err
	}

	return nil
}

// applyK8sNodeGroupSizeDefaults is func to resolve the autoscale fields for the dynamic creation paths
func applyK8sNodeGroupSizeDefaults(ngReq *model.K8sNodeGroupReq, dReqOnAutoScaling string,
	desiredNodeSize, minNodeSize, maxNodeSize int,
	offNodeSize model.K8sClusterAutoScalingOffNodeSize) error {

	if desiredNodeSize < 0 || minNodeSize < 0 || maxNodeSize < 0 {
		return fmt.Errorf("node sizes must not be negative (desiredNodeSize=%d, minNodeSize=%d, maxNodeSize=%d)",
			desiredNodeSize, minNodeSize, maxNodeSize)
	}

	onAutoScaling := true
	autoScalingExplicit := dReqOnAutoScaling != ""
	if autoScalingExplicit {
		parsed, err := strconv.ParseBool(dReqOnAutoScaling)
		if err != nil {
			return fmt.Errorf("invalid onAutoScaling value %q: must be a boolean such as \"true\" or \"false\"", dReqOnAutoScaling)
		}
		onAutoScaling = parsed
	}
	ngReq.OnAutoScaling = strconv.FormatBool(onAutoScaling)

	ngReq.DesiredNodeSize = desiredNodeSize
	if ngReq.DesiredNodeSize <= 0 {
		ngReq.DesiredNodeSize = 1
	}

	ngReq.MinNodeSize = minNodeSize
	ngReq.MaxNodeSize = maxNodeSize

	switch {
	case onAutoScaling && autoScalingExplicit:
		if ngReq.MinNodeSize <= 0 || ngReq.MaxNodeSize <= 0 {
			return fmt.Errorf("minNodeSize and maxNodeSize are required when onAutoScaling is set to \"true\" explicitly "+
				"(got minNodeSize=%d, maxNodeSize=%d); omit onAutoScaling to keep the defaults, or set it to \"false\"",
				ngReq.MinNodeSize, ngReq.MaxNodeSize)
		}
	case onAutoScaling:
		// onAutoScaling omitted: keep the historical defaults.
		if ngReq.MinNodeSize <= 0 {
			ngReq.MinNodeSize = 1
		}
		if ngReq.MaxNodeSize <= 0 {
			ngReq.MaxNodeSize = 2
		}
	default:
		// Autoscaling off: Min/Max carry no meaning, so raise them only to the floor the CSP
		// insists on. A zero field in offNodeSize means the CSP has no such requirement.
		if ngReq.MinNodeSize < offNodeSize.Min {
			ngReq.MinNodeSize = offNodeSize.Min
		}
		if ngReq.MaxNodeSize < offNodeSize.Max {
			ngReq.MaxNodeSize = offNodeSize.Max
		}
	}

	if ngReq.MinNodeSize > 0 && ngReq.MinNodeSize > ngReq.MaxNodeSize {
		return fmt.Errorf("minNodeSize(%d) must not be greater than maxNodeSize(%d)",
			ngReq.MinNodeSize, ngReq.MaxNodeSize)
	}

	return nil
}

// getK8sClusterReqFromDynamicReq is func to get K8sClusterReq from K8sClusterDynamicReq
func getK8sClusterReqFromDynamicReq(ctx context.Context, nsId string, dReq *model.K8sClusterDynamicReq, skipVersionCheck bool) (*model.K8sClusterReq, error) {
	reqID := common.RequestIDFromContext(ctx)
	onDemand := true

	emptyK8sReq := &model.K8sClusterReq{}
	k8sReq := &model.K8sClusterReq{}
	k8sngReq := &model.K8sNodeGroupReq{}

	specInfo, err := resource.GetSpec(model.SystemCommonNs, dReq.SpecId)
	if err != nil {
		log.Err(err).Msg("")
		return emptyK8sReq, err
	}
	k8sngReq.SpecId = specInfo.Id

	var k8sRecVersion string
	if skipVersionCheck {
		// Use the requested version directly without validation
		k8sRecVersion = dReq.Version
		if k8sRecVersion == "" {
			// If skipVersionCheck is true, an explicit version must be provided
			err := fmt.Errorf("skipVersionCheck is true but no version is specified; an explicit version must be provided")
			log.Err(err).Msg("")
			return emptyK8sReq, err
		}
		log.Warn().Msgf("K8sCluster version validation skipped for version: %s (dynamic)", k8sRecVersion)
	} else {
		// Normal validation path
		k8sRecVersion, err = getK8sRecommendVersion(specInfo.ProviderName, specInfo.RegionName, dReq.Version)
		if err != nil {
			log.Err(err).Msg("")
			return emptyK8sReq, err
		}
	}

	// If ConnectionName is specified by the request, Use ConnectionName from the request
	k8sReq.ConnectionName = specInfo.ConnectionName
	if dReq.ConnectionName != "" {
		k8sReq.ConnectionName = dReq.ConnectionName
	}

	// validate the GetConnConfig for spec
	connection, err := common.GetConnConfig(k8sReq.ConnectionName)
	if err != nil {
		err := fmt.Errorf("Failed to Get ConnectionName (%s) for Spec (%s) is not found.", k8sReq.ConnectionName, dReq.SpecId)
		log.Err(err).Msg("")
		return emptyK8sReq, err
	}

	k8sNgOnCreation, err := common.GetK8sNodeGroupsOnK8sCreation(connection.ProviderName)
	if err != nil {
		log.Err(err).Msgf("Failed to Get Nodegroups on K8sCluster Creation")
		return emptyK8sReq, err
	}

	// In K8sCluster, allows dReq.ImageId to be set to "default" or ""
	if strings.EqualFold(dReq.ImageId, "default") ||
		strings.EqualFold(dReq.ImageId, "") {
		// do nothing
	} else {
		// Check if the image is available (DB or CSP) and auto-register if needed
		_, isAutoRegistered, err := resource.EnsureImageAvailable(ctx, nsId, k8sReq.ConnectionName, dReq.ImageId)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get the Image from the CSP")
			return emptyK8sReq, err
		}
		if isAutoRegistered {
			log.Info().Msgf("Image '%s' was auto-registered from CSP for K8sCluster", dReq.ImageId)
		}
	}

	// Default resource name has this pattern (nsId + "-shared-" + nodeReq.ConnectionName)
	resourceName := nsId + model.StrSharedResourceName + k8sReq.ConnectionName

	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Setting vNet:" + resourceName, Time: time.Now()})

	k8sReq.VNetId = resourceName
	_, err = resource.GetResource(nsId, model.StrVNet, k8sReq.VNetId)
	if err != nil {
		if !onDemand {
			err := fmt.Errorf("Failed to get the vNet %s from %s", k8sReq.VNetId, k8sReq.ConnectionName)
			log.Err(err).Msg("Failed to get the vNet")
			return emptyK8sReq, err
		}

		clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Loading default vNet:" + resourceName, Time: time.Now()})

		err2 := resource.CreateSharedResource(ctx, nsId, model.StrVNet, k8sReq.ConnectionName)
		if err2 != nil {
			log.Err(err2).Msg("Failed to create new default vNet " + k8sReq.VNetId + " from " + k8sReq.ConnectionName)
			return emptyK8sReq, err2
		} else {
			log.Info().Msg("Created new default vNet: " + k8sReq.VNetId)
		}
	} else {
		log.Info().Msg("Found and utilize default vNet: " + k8sReq.VNetId)

		// Fail fast if the vNet was deleted out-of-band on the CSP.
		if exists, indet := resource.VerifySharedResourceOnCsp(nsId, model.StrVNet, k8sReq.VNetId); indet == nil && !exists {
			err := fmt.Errorf("vNet '%s' is recorded in Tumblebug but missing on the CSP (deleted out-of-band?). "+
				"Clear the stale record with DELETE /ns/%s/deregisterResource/vNet/%s?withSubnets=true and retry; it will be recreated on demand",
				k8sReq.VNetId, nsId, k8sReq.VNetId)
			log.Error().Err(err).Msg("VNet drift detected for K8sCluster")
			return emptyK8sReq, err
		}
	}
	k8sReq.SubnetIds = append(k8sReq.SubnetIds, resourceName)
	k8sReq.SubnetIds = append(k8sReq.SubnetIds, resourceName+"-01")

	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Setting SSHKey:" + resourceName, Time: time.Now()})

	k8sngReq.SshKeyId = resourceName
	_, err = resource.GetResource(nsId, model.StrSSHKey, k8sngReq.SshKeyId)
	if err != nil {
		if !onDemand {
			err := fmt.Errorf("Failed to get the SSHKey %s from %s", k8sngReq.SshKeyId, k8sReq.ConnectionName)
			log.Err(err).Msg("Failed to get the SSHKey")
			return emptyK8sReq, err
		}

		clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Loading default SSHKey:" + resourceName, Time: time.Now()})

		err2 := resource.CreateSharedResource(ctx, nsId, model.StrSSHKey, k8sReq.ConnectionName)
		if err2 != nil {
			log.Err(err2).Msg("Failed to create new default SSHKey " + k8sngReq.SshKeyId + " from " + k8sReq.ConnectionName)
			return emptyK8sReq, err2
		} else {
			log.Info().Msg("Created new default SSHKey: " + k8sngReq.SshKeyId)
		}
	} else {
		log.Info().Msg("Found and utilize default SSHKey: " + k8sngReq.SshKeyId)

		// If the keypair was deleted out-of-band on the CSP, replace the stale record.
		if exists, indet := resource.VerifySharedResourceOnCsp(nsId, model.StrSSHKey, k8sngReq.SshKeyId); indet == nil && !exists {
			log.Warn().Msgf("SSHKey drift detected for '%s'; deregistering stale record and recreating", k8sngReq.SshKeyId)
			if derr := resource.DeregisterResource(nsId, model.StrSSHKey, k8sngReq.SshKeyId); derr != nil {
				log.Warn().Err(derr).Msgf("failed to deregister drifted SSHKey '%s'", k8sngReq.SshKeyId)
			}
			if err2 := resource.CreateSharedResource(ctx, nsId, model.StrSSHKey, k8sReq.ConnectionName); err2 != nil {
				log.Err(err2).Msg("Failed to recreate drifted SSHKey " + k8sngReq.SshKeyId + " from " + k8sReq.ConnectionName)
				return emptyK8sReq, err2
			}
			log.Info().Msg("Recreated drifted SSHKey: " + k8sngReq.SshKeyId)
		}
	}

	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Setting securityGroup:" + resourceName, Time: time.Now()})

	// K8s uses a shared SG per connection, but sourced from the dedicated "sg-k8s" template
	// (intentionally permissive; required ports vary by CSP). The template suffix keeps this
	// SG distinct from other shared SGs (e.g. an SSH-only "{ns}-shared-{conn}").
	securityGroup := resourceName + "-" + model.K8sSecurityGroupTemplateId
	k8sReq.SecurityGroupIds = append(k8sReq.SecurityGroupIds, securityGroup)
	_, err = resource.GetResource(nsId, model.StrSecurityGroup, securityGroup)
	if err != nil {
		if !onDemand {
			err := fmt.Errorf("Failed to get the securityGroup %s from %s", securityGroup, k8sReq.ConnectionName)
			log.Err(err).Msg("Failed to get the securityGroup")
			return emptyK8sReq, err
		}

		clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Loading default securityGroup:" + securityGroup, Time: time.Now()})

		err2 := createSharedResourceWithRetry(ctx, nsId, model.StrSecurityGroup, k8sReq.ConnectionName,
			&resource.SharedResourceOptions{SgTemplateId: model.K8sSecurityGroupTemplateId})
		if err2 != nil {
			log.Err(err2).Msg("Failed to create new default securityGroup " + securityGroup + " from " + k8sReq.ConnectionName)
			return emptyK8sReq, err2
		} else {
			log.Info().Msg("Created new default securityGroup: " + securityGroup)
		}
	} else {
		log.Info().Msg("Found and utilize default securityGroup: " + securityGroup)

		// Fail fast if the securityGroup was deleted out-of-band on the CSP.
		if exists, indet := resource.VerifySharedResourceOnCsp(nsId, model.StrSecurityGroup, securityGroup); indet == nil && !exists {
			err := fmt.Errorf("securityGroup '%s' is recorded in Tumblebug but missing on the CSP (deleted out-of-band?). "+
				"Clear the stale record with DELETE /ns/%s/deregisterResource/securityGroup/%s and retry; it will be recreated on demand",
				securityGroup, nsId, securityGroup)
			log.Error().Err(err).Msg("SecurityGroup drift detected for K8sCluster")
			return emptyK8sReq, err
		}
	}

	k8sngReq.Name = dReq.NodeGroupName
	if k8sngReq.Name == "" {
		k8sngReq.Name = common.GenUid()
	}
	k8sngReq.RootDiskType = dReq.RootDiskType
	k8sngReq.RootDiskSize = dReq.RootDiskSize
	offNodeSize, err := common.GetK8sAutoScalingOffNodeSize(connection.ProviderName)
	if err != nil {
		log.Warn().Err(err).Msgf("Failed to get AutoScalingOffNodeSize for provider(%s); assuming no floor", connection.ProviderName)
		offNodeSize = model.K8sClusterAutoScalingOffNodeSize{}
	}
	if err := applyK8sNodeGroupSizeDefaults(k8sngReq, dReq.OnAutoScaling,
		dReq.DesiredNodeSize, dReq.MinNodeSize, dReq.MaxNodeSize, offNodeSize); err != nil {
		log.Err(err).Msg("Failed to apply K8sNodeGroup size defaults")
		return emptyK8sReq, err
	}
	k8sReq.Description = dReq.Description
	k8sReq.Name = dReq.Name
	if k8sReq.Name == "" {
		k8sReq.Name = common.GenUid()
	}
	k8sReq.Version = k8sRecVersion
	if k8sNgOnCreation {
		k8sReq.K8sNodeGroupList = append(k8sReq.K8sNodeGroupList, *k8sngReq)
	} else {
		log.Info().Msg("Need to Add NodeGroups To Use This K8sCluster")
	}
	k8sReq.Label = dReq.Label

	common.PrintJsonPretty(k8sReq)
	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Prepared resources for K8sCluster:" + k8sReq.Name, Info: k8sReq, Time: time.Now()})

	return k8sReq, nil
}

// CreateK8sClusterDynamic is func to create K8sCluster obeject and deploy requested K8sCluster and NodeGroup in a dynamic way
func CreateK8sClusterDynamic(ctx context.Context, nsId string, dReq *model.K8sClusterDynamicReq, deployOption string, skipVersionCheck bool) (*model.K8sClusterInfo, error) {
	reqID := common.RequestIDFromContext(ctx)
	emptyK8sCluster := &model.K8sClusterInfo{}
	err := common.CheckString(nsId)
	if err != nil {
		log.Err(err).Msg("")
		return emptyK8sCluster, err
	}

	// Validate that name is provided for single cluster creation
	if dReq.Name == "" {
		err := fmt.Errorf("cluster name is required")
		log.Err(err).Msg("")
		return emptyK8sCluster, err
	}

	check, err := resource.CheckK8sCluster(nsId, dReq.Name)
	if err != nil {
		log.Err(err).Msg("")
		return emptyK8sCluster, err
	}
	if check {
		err := fmt.Errorf("already exists")
		log.Err(err).Msgf("Failed to Create K8sCluster(%s) Dynamically", dReq.Name)
		return emptyK8sCluster, err
	}

	err = checkCommonResAvailableForK8sClusterDynamicReq(ctx, dReq)
	if err != nil {
		log.Err(err).Msgf("Failed to find common resource for K8sCluster provision")
		return emptyK8sCluster, err
	}

	//If not, generate default resources dynamically.
	k8sReq, err := getK8sClusterReqFromDynamicReq(ctx, nsId, dReq, skipVersionCheck)
	if err != nil {
		log.Err(err).Msg("Failed to get shared resources for dynamic K8sCluster creation")
		return emptyK8sCluster, err
	}
	/*
		  FIXME: need to improve a rollback process
			if err != nil {
				log.Err(err).Msg("Failed to prefare resources for dynamic K8sCluster creation")
				// Rollback created default resources
				time.Sleep(5 * time.Second)
				log.Info().Msg("Try rollback created default resources")
				rollbackResult, rollbackErr := resource.DelAllSharedResources(nsId)
				if rollbackErr != nil {
					err = fmt.Errorf("Failed in rollback operation: %w", rollbackErr)
				} else {
					ids := strings.Join(rollbackResult.IdList, ", ")
					err = fmt.Errorf("Rollback results [%s]: %w", ids, err)
				}
				return emptyK8sCluster, err
			}
	*/

	common.PrintJsonPretty(k8sReq)
	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Prepared all resources for provisioning K8sCluster:" + k8sReq.Name, Info: k8sReq, Time: time.Now()})
	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Start provisioning", Time: time.Now()})

	// Run create K8sCluster with the generated K8sCluster request (option != register)
	option := "create"
	if deployOption == "hold" {
		option = "hold"
	}

	// skipVersionCheck parameter is passed from function argument
	return resource.CreateK8sCluster(ctx, nsId, k8sReq, option, skipVersionCheck)
}

// getK8sNodeGroupReqFromDynamicReq is func to get K8sNodeGroupReq from K8sNodeGroupDynamicReq
func getK8sNodeGroupReqFromDynamicReq(ctx context.Context, nsId string, k8sClusterInfo *model.K8sClusterInfo, dReq *model.K8sNodeGroupDynamicReq) (*model.K8sNodeGroupReq, error) {
	reqID := common.RequestIDFromContext(ctx)
	emptyK8sNgReq := &model.K8sNodeGroupReq{}
	k8sNgReq := &model.K8sNodeGroupReq{}

	specInfo, err := resource.GetSpec(model.SystemCommonNs, dReq.SpecId)
	if err != nil {
		log.Err(err).Msg("")
		return emptyK8sNgReq, err
	}
	k8sNgReq.SpecId = specInfo.Id

	// If ConnectionName for K8sNodeGroup must be same as ConnectionName for K8sCluster
	if specInfo.ConnectionName != k8sClusterInfo.ConnectionName {
		err := fmt.Errorf("ConnectionName(%s) of K8sNodeGroup Must Match ConnectionName(%s) of K8sCluster", specInfo.ConnectionName, k8sClusterInfo.ConnectionName)
		log.Err(err).Msg("")
		return emptyK8sNgReq, err
	}

	// In K8sNodeGroup, allows dReq.ImageId to be set to "default" or ""
	if strings.EqualFold(dReq.ImageId, "default") ||
		strings.EqualFold(dReq.ImageId, "") {
		// Use default - Spider will auto-map AMI Type based on VMSpec
		k8sNgReq.ImageId = ""
		log.Debug().Msg("ImageId is empty or default. Spider will auto-map AMI Type based on VMSpec.")
	} else {
		// Check if the image is available (DB or CSP) and auto-register if needed
		_, isAutoRegistered, err := resource.EnsureImageAvailable(ctx, nsId, k8sClusterInfo.ConnectionName, dReq.ImageId)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get the Image from the CSP")
			return emptyK8sNgReq, err
		}
		if isAutoRegistered {
			log.Info().Msgf("Image '%s' was auto-registered from CSP for K8sNodeGroup", dReq.ImageId)
		}
		k8sNgReq.ImageId = dReq.ImageId
		log.Debug().Msgf("Using user-specified imageId: %s", dReq.ImageId)
	}

	// Default resource name has this pattern (nsId + "-shared-" + nodeReq.ConnectionName)
	resourceName := nsId + model.StrSharedResourceName + k8sClusterInfo.ConnectionName

	k8sNgReq.SshKeyId = resourceName
	_, err = resource.GetResource(nsId, model.StrSSHKey, k8sNgReq.SshKeyId)
	if err != nil {
		err := fmt.Errorf("Failed to get the SSHKey %s from %s", k8sNgReq.SshKeyId, k8sClusterInfo.ConnectionName)
		log.Err(err).Msg("Failed to get the SSHKey")
		return emptyK8sNgReq, err
	} else {
		log.Info().Msg("Found and utilize default SSHKey: " + k8sNgReq.SshKeyId)

		// If the keypair was deleted out-of-band on the CSP, replace the stale record.
		if exists, indet := resource.VerifySharedResourceOnCsp(nsId, model.StrSSHKey, k8sNgReq.SshKeyId); indet == nil && !exists {
			log.Warn().Msgf("SSHKey drift detected for '%s'; deregistering stale record and recreating", k8sNgReq.SshKeyId)
			if derr := resource.DeregisterResource(nsId, model.StrSSHKey, k8sNgReq.SshKeyId); derr != nil {
				log.Warn().Err(derr).Msgf("failed to deregister drifted SSHKey '%s'", k8sNgReq.SshKeyId)
			}
			if err2 := resource.CreateSharedResource(ctx, nsId, model.StrSSHKey, k8sClusterInfo.ConnectionName); err2 != nil {
				log.Err(err2).Msg("Failed to recreate drifted SSHKey " + k8sNgReq.SshKeyId + " from " + k8sClusterInfo.ConnectionName)
				return emptyK8sNgReq, err2
			}
			log.Info().Msg("Recreated drifted SSHKey: " + k8sNgReq.SshKeyId)
		}
	}

	k8sNgReq.Name = dReq.Name
	if k8sNgReq.Name == "" {
		k8sNgReq.Name = common.GenUid()
	}
	k8sNgReq.RootDiskType = dReq.RootDiskType
	k8sNgReq.RootDiskSize = dReq.RootDiskSize
	// specInfo.ConnectionName is verified above to match the cluster's ConnectionName,
	// so its ProviderName is the cluster's provider.
	offNodeSize, err := common.GetK8sAutoScalingOffNodeSize(specInfo.ProviderName)
	if err != nil {
		log.Warn().Err(err).Msgf("Failed to get AutoScalingOffNodeSize for provider(%s); assuming no floor", specInfo.ProviderName)
		offNodeSize = model.K8sClusterAutoScalingOffNodeSize{}
	}
	if err := applyK8sNodeGroupSizeDefaults(k8sNgReq, dReq.OnAutoScaling,
		dReq.DesiredNodeSize, dReq.MinNodeSize, dReq.MaxNodeSize, offNodeSize); err != nil {
		log.Err(err).Msg("Failed to apply K8sNodeGroup size defaults")
		return emptyK8sNgReq, err
	}
	k8sNgReq.Description = dReq.Description
	k8sNgReq.Label = dReq.Label

	common.PrintJsonPretty(k8sNgReq)
	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Prepared resources for K8sNodeGroup:" + k8sNgReq.Name, Info: k8sNgReq, Time: time.Now()})

	return k8sNgReq, nil
}

// CreateK8sNodeGroupDynamic is func to create K8sNodeGroup obeject and deploy requested K8sNodeGroup in a dynamic way
func CreateK8sNodeGroupDynamic(ctx context.Context, nsId string, k8sClusterId string, dReq *model.K8sNodeGroupDynamicReq) (*model.K8sClusterInfo, error) {
	reqID := common.RequestIDFromContext(ctx)
	log.Debug().Msgf("reqID: %s, nsId: %s, k8sClusterId: %s, dReq: %v\n", reqID, nsId, k8sClusterId, dReq)

	emptyK8sCluster := &model.K8sClusterInfo{}

	check, err := resource.CheckK8sCluster(nsId, k8sClusterId)
	if err != nil {
		log.Err(err).Msg("")
		return emptyK8sCluster, err
	}
	if !check {
		err := fmt.Errorf("K8sCluster(%s) is not existed", k8sClusterId)
		log.Err(err).Msgf("Failed to Create K8sNodeGroup(%s) in K8sCluster(%s) Dynamically", dReq.Name, k8sClusterId)
		return emptyK8sCluster, err
	}

	tbK8sCInfo, err := resource.GetK8sCluster(nsId, k8sClusterId)
	if err != nil {
		log.Err(err).Msgf("Failed to Create K8sNodeGroup(%s) in K8sCluster(%s) Dynamically", dReq.Name, k8sClusterId)
		return emptyK8sCluster, err
	}

	if tbK8sCInfo.Status != model.K8sClusterActive {
		err := fmt.Errorf("K8sCluster(%s) is not active status", k8sClusterId)
		log.Err(err).Msgf("Failed to Create K8sNodeGroup(%s) in K8sCluster(%s) Dynamically", dReq.Name, k8sClusterId)
		return emptyK8sCluster, err
	}

	for _, ngi := range tbK8sCInfo.K8sNodeGroupList {
		if ngi.Name == dReq.Name {
			err := fmt.Errorf("K8sNodeGroup(%s) already exists", dReq.Name)
			log.Err(err).Msgf("Failed to Create K8sNodeGroup(%s) in K8sCluster(%s) Dynamically", dReq.Name, k8sClusterId)
			return emptyK8sCluster, err
		}
	}

	err = checkCommonResAvailableForK8sNodeGroupDynamicReq(ctx, tbK8sCInfo.ConnectionName, dReq)
	if err != nil {
		log.Err(err).Msgf("Failed to find common resource for K8sNodeGroup provision")
		return emptyK8sCluster, err
	}

	k8sNgReq, err := getK8sNodeGroupReqFromDynamicReq(ctx, nsId, tbK8sCInfo, dReq)
	if err != nil {
		log.Err(err).Msg("Failed to get shared resources for dynamic K8sNodeGroup creation")
		return emptyK8sCluster, err
	}

	common.PrintJsonPretty(k8sNgReq)
	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Prepared all resources for provisioning K8sNodeGroup:" + k8sNgReq.Name, Info: k8sNgReq, Time: time.Now()})
	clientManager.UpdateRequestProgress(reqID, clientManager.ProgressInfo{Title: "Start provisioning", Time: time.Now()})

	return resource.AddK8sNodeGroup(ctx, nsId, k8sClusterId, k8sNgReq)
}

// Provisioning History Management Functions

// generateProvisioningLogKey generates kvstore key for provisioning log
// It URL-encodes the specId to handle special characters like "+" safely

func CreateK8sMultiClusterDynamic(ctx context.Context, nsId string, multiReq *model.K8sMultiClusterDynamicReq, deployOption string, skipVersionCheck bool) (*model.K8sMultiClusterInfo, error) {
	reqID := common.RequestIDFromContext(ctx)
	if len(multiReq.Clusters) == 0 {
		return nil, fmt.Errorf("no clusters specified in the request")
	}

	// Validate: Either namePrefix is provided OR all clusters have names
	if multiReq.NamePrefix == "" {
		for i, cluster := range multiReq.Clusters {
			if cluster.Name == "" {
				return nil, fmt.Errorf("cluster[%d] must have a name when namePrefix is not provided", i)
			}
		}
	}

	log.Info().Msgf("Creating %d K8sClusters in parallel", len(multiReq.Clusters))

	// Create channels for results
	type clusterResult struct {
		index          int
		name           string // Store actual cluster name for error reporting
		connectionName string // Connection name for error reporting
		specId         string // Spec ID for error reporting
		cluster        *model.K8sClusterInfo
		err            error
	}
	resultChan := make(chan clusterResult, len(multiReq.Clusters))

	// Capture namePrefix to avoid data race in goroutines
	namePrefix := multiReq.NamePrefix

	// Launch goroutines for parallel creation
	for i, clusterReq := range multiReq.Clusters {
		go func(index int, req model.K8sClusterDynamicReq) {
			// Generate unique request ID for each cluster
			clusterCtx := common.WithRequestID(ctx, fmt.Sprintf("%s-cluster-%d", reqID, index))

			// Auto-generate cluster name and inject clustergroup label if NamePrefix is provided
			if namePrefix != "" {
				// Auto-generate name if not provided
				if req.Name == "" {
					// Extract CSP name from specId (format: "provider+region+spec")
					cspName := "unknown"
					if req.SpecId != "" {
						parts := strings.Split(req.SpecId, "+")
						if len(parts) > 0 {
							cspName = parts[0]
						}
					}
					req.Name = fmt.Sprintf("%s-%s-%d", namePrefix, cspName, index+1)
					log.Debug().Msgf("Auto-generated cluster name: %s", req.Name)
				}

				// Inject clustergroup label for grouping clusters created together
				if req.Label == nil {
					req.Label = make(map[string]string)
				}
				req.Label["clustergroup"] = namePrefix
				log.Debug().Msgf("Injected clustergroup label: %s for cluster: %s", namePrefix, req.Name)
			}

			log.Info().Msgf("[%d/%d] Starting K8sCluster creation: %s", index+1, len(multiReq.Clusters), req.Name)

			cluster, err := CreateK8sClusterDynamic(clusterCtx, nsId, &req, deployOption, skipVersionCheck)

			if err != nil {
				log.Error().Err(err).Msgf("[%d/%d] Failed to create K8sCluster: %s", index+1, len(multiReq.Clusters), req.Name)
			} else {
				log.Info().Msgf("[%d/%d] Successfully created K8sCluster: %s", index+1, len(multiReq.Clusters), req.Name)
			}

			resultChan <- clusterResult{
				index:          index,
				name:           req.Name, // Store actual name for error reporting
				connectionName: req.ConnectionName,
				specId:         req.SpecId,
				cluster:        cluster,
				err:            err,
			}
		}(i, clusterReq)
	}

	// Collect results
	results := make([]*model.K8sClusterInfo, len(multiReq.Clusters))
	var errors []string
	var failedClusters []model.K8sClusterFailedInfo

	for i := 0; i < len(multiReq.Clusters); i++ {
		result := <-resultChan
		results[result.index] = result.cluster
		if result.err != nil {
			// Use actual cluster name (which may be auto-generated)
			errors = append(errors, fmt.Sprintf("Cluster[%d] %s: %v", result.index, result.name, result.err))
			// Add to failed clusters list for detailed error reporting
			failedClusters = append(failedClusters, model.K8sClusterFailedInfo{
				Name:           result.name,
				ConnectionName: result.connectionName,
				SpecId:         result.specId,
				Error:          result.err.Error(),
			})
		}
	}

	// Prepare response
	multiInfo := &model.K8sMultiClusterInfo{
		Clusters:       make([]model.K8sClusterInfo, 0, len(results)),
		FailedClusters: failedClusters,
	}

	for _, cluster := range results {
		// Skip nil or empty clusters (empty cluster has no Id/Name)
		if cluster != nil && cluster.Id != "" {
			multiInfo.Clusters = append(multiInfo.Clusters, *cluster)
		}
	}

	// Return error if any cluster failed
	if len(errors) > 0 {
		log.Warn().Msgf("Some clusters failed to create: %v", errors)
		return multiInfo, fmt.Errorf("failed to create %d cluster(s): %s", len(errors), strings.Join(errors, "; "))
	}

	log.Info().Msgf("Successfully created all %d K8sClusters", len(multiInfo.Clusters))
	return multiInfo, nil
}
