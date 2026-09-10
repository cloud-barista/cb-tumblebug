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

// Package common is to include common methods for managing multi-cloud infra
package common

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
)

func GetK8sClusterInfo() (model.K8sClusterAssetInfo, error) {
	return RuntimeK8sClusterInfo, nil
}

func getK8sClusterDetail(providerName string) *model.K8sClusterDetail {
	// Get model.K8sClusterDetail for providerName
	var k8sClusterDetail *model.K8sClusterDetail = nil
	for provider, detail := range RuntimeK8sClusterInfo.CSPs {
		provider = strings.ToLower(provider)
		if provider == providerName {
			k8sClusterDetail = &detail
			break
		}
	}

	return k8sClusterDetail
}

// GetAvailableK8sVersion is func to get available kubernetes cluster versions for provider and region from model.K8sClusterInfo
func GetAvailableK8sVersion(providerName string, regionName string) (*[]model.K8sClusterVersionDetailAvailable, error) {
	//
	// Check available K8sCluster version in k8sclusterinfo.yaml
	//

	providerName = strings.ToLower(providerName)
	regionName = strings.ToLower(regionName)

	// Get model.K8sClusterDetail for providerName
	k8sClusterDetail := getK8sClusterDetail(providerName)
	if k8sClusterDetail == nil {
		return nil, fmt.Errorf("unsupported provider(%s) for kubernetes cluster", providerName)
	}

	// Check if 'regionName' exists
	var availableVersion *[]model.K8sClusterVersionDetailAvailable = nil
	for _, versionDetail := range k8sClusterDetail.Version {
		for _, region := range versionDetail.Region {
			region = strings.ToLower(region)
			if strings.EqualFold(region, regionName) {
				if len(versionDetail.Available) == 0 {
					availableVersion = &[]model.K8sClusterVersionDetailAvailable{{Name: model.StrEmpty, Id: model.StrEmpty}}
				} else {
					availableVersion = &versionDetail.Available
				}
				return availableVersion, nil
			}
		}
	}

	// Check if 'common' exists
	for _, versionDetail := range k8sClusterDetail.Version {
		for _, region := range versionDetail.Region {
			region = strings.ToLower(region)
			if strings.EqualFold(region, model.StrCommon) {
				if len(versionDetail.Available) == 0 {
					availableVersion = &[]model.K8sClusterVersionDetailAvailable{{Name: model.StrEmpty, Id: model.StrEmpty}}
				} else {
					availableVersion = &versionDetail.Available
				}
				return availableVersion, nil
			}
		}
	}

	return nil, fmt.Errorf("no entry for provider(%s):region(%s)", providerName, regionName)
}

// GetAvailableK8sNodeImage is func to get available kubernetes cluster node images for provider and region from model.K8sClusterInfo
func GetAvailableK8sNodeImage(providerName string, regionName string) (*[]model.K8sClusterNodeImageDetailAvailable, error) {
	//
	// Check available K8sCluster node image in k8sclusterinfo.yaml
	//

	providerName = strings.ToLower(providerName)
	regionName = strings.ToLower(regionName)

	// Get model.K8sClusterDetail for providerName
	k8sClusterDetail := getK8sClusterDetail(providerName)
	if k8sClusterDetail == nil {
		return nil, fmt.Errorf("unsupported provider(%s) for kubernetes cluster", providerName)
	}

	// Check if 'regionName' exists
	var availableNodeImage *[]model.K8sClusterNodeImageDetailAvailable = nil
	for _, nodeImageDetail := range k8sClusterDetail.NodeImage {
		for _, region := range nodeImageDetail.Region {
			region = strings.ToLower(region)
			if strings.EqualFold(region, regionName) {
				if len(nodeImageDetail.Available) == 0 {
					availableNodeImage = &[]model.K8sClusterNodeImageDetailAvailable{{Name: model.StrEmpty, Id: model.StrEmpty}}
					break
				} else {
					availableNodeImage = &nodeImageDetail.Available
				}
				return availableNodeImage, nil
			}
		}
	}

	// Check if 'common' exists
	for _, nodeImageDetail := range k8sClusterDetail.NodeImage {
		for _, region := range nodeImageDetail.Region {
			region = strings.ToLower(region)
			if strings.EqualFold(region, model.StrCommon) {
				if len(nodeImageDetail.Available) == 0 {
					availableNodeImage = &[]model.K8sClusterNodeImageDetailAvailable{{Name: model.StrEmpty, Id: model.StrEmpty}}
					break
				} else {
					availableNodeImage = &nodeImageDetail.Available
				}
				return availableNodeImage, nil
			}
		}
	}

	return nil, fmt.Errorf("no available kubernetes cluster node image for region(%s) of provider(%s)", regionName, providerName)
}

// GetK8sNodeGroupsOnK8sCreation is func to get whether nodegroups are required during the k8scluster creation
func GetK8sNodeGroupsOnK8sCreation(providerName string) (bool, error) {
	//
	// Get nodeGroupsOnCreation field in k8sclusterinfo.yaml
	//

	providerName = strings.ToLower(providerName)

	// Get model.K8sClusterDetail for providerName
	k8sClusterDetail := getK8sClusterDetail(providerName)
	if k8sClusterDetail == nil {
		return false, fmt.Errorf("unsupported provider(%s) for kubernetes cluster", providerName)
	}

	return k8sClusterDetail.NodeGroupsOnCreation, nil
}

// GetModelK8sNodeGroupsOnK8sCreation is to convert a NodeGroupsOnK8sCreation value to model.K8sClusterNodeGroupsOnK8sCreation
func GetModelK8sNodeGroupsOnK8sCreation(providerName string) (*model.K8sClusterNodeGroupsOnCreation, error) {
	k8sNodeGroupsOnK8sCreation, err := GetK8sNodeGroupsOnK8sCreation(providerName)
	if err != nil {
		return nil, err
	}

	return &model.K8sClusterNodeGroupsOnCreation{
		Result: strconv.FormatBool(k8sNodeGroupsOnK8sCreation),
	}, nil
}

// GetK8sNodeImageDesignation is func to get whether node image designation is possible to create a k8scluster
func GetK8sNodeImageDesignation(providerName string) (bool, error) {
	//
	// Get nodeGroupsOnCreation field in k8sclusterinfo.yaml
	//

	providerName = strings.ToLower(providerName)

	// Get model.K8sClusterDetail for providerName
	k8sClusterDetail := getK8sClusterDetail(providerName)
	if k8sClusterDetail == nil {
		return false, fmt.Errorf("unsupported provider(%s) for kubernetes cluster", providerName)
	}

	return k8sClusterDetail.NodeImageDesignation, nil
}

// GetModelK8sNodeImageDesignation is to convert a NodeImageDesignation value to model.K8sClusterNodeImageDesignation
func GetModelK8sNodeImageDesignation(providerName string) (*model.K8sClusterNodeImageDesignation, error) {
	k8sNodeImageDesignation, err := GetK8sNodeImageDesignation(providerName)
	if err != nil {
		return nil, err
	}

	return &model.K8sClusterNodeImageDesignation{
		Result: strconv.FormatBool(k8sNodeImageDesignation),
	}, nil
}

// GetK8sRequiredSubnetCount is func to get the required subnet count to create a k8scluster
func GetK8sRequiredSubnetCount(providerName string) (int, error) {
	//
	// Get requiredSubnetCount field in k8sclusterinfo.yaml
	//

	providerName = strings.ToLower(providerName)

	// Get model.K8sClusterDetail for providerName
	k8sClusterDetail := getK8sClusterDetail(providerName)
	if k8sClusterDetail == nil {
		return 0, fmt.Errorf("unsupported provider(%s) for kubernetes cluster", providerName)
	}

	// Set default value is 1
	requiredSubnetCount := max(k8sClusterDetail.RequiredSubnetCount, 1)

	return requiredSubnetCount, nil
}

// GetModelK8sRequiredSubnetCount is func to get the required subnet count to create a k8scluster
func GetModelK8sRequiredSubnetCount(providerName string) (*model.K8sClusterRequiredSubnetCount, error) {
	k8sRequiredSubnetCount, err := GetK8sRequiredSubnetCount(providerName)
	if err != nil {
		return nil, err
	}

	return &model.K8sClusterRequiredSubnetCount{
		Result: strconv.FormatInt(int64(k8sRequiredSubnetCount), 10),
	}, nil
}

const DefaultNamingRule = "^.*$" // Wildcard Pattern

// GetK8sInitialNodeGroupManagedByCluster returns whether the initial node group
// created during cluster creation is lifecycle-bound to the cluster (i.e., cannot
// be deleted independently via the node group API).
// This applies to CSPs such as Alibaba ACK and Tencent TKE.
func GetK8sInitialNodeGroupManagedByCluster(providerName string) (bool, error) {
	providerName = strings.ToLower(providerName)

	k8sClusterDetail := getK8sClusterDetail(providerName)
	if k8sClusterDetail == nil {
		return false, fmt.Errorf("unsupported provider(%s) for kubernetes cluster", providerName)
	}

	return k8sClusterDetail.InitialNodeGroupManagedByCluster, nil
}

// GetK8sAutoScalingOffNodeSize returns the node-size floor the CSP still demands while
// autoscaling is disabled. Zero fields mean the CSP ignores Min/Max in that state, which is
// the common case; see model.K8sClusterAutoScalingOffNodeSize for the CSPs that do not.
func GetK8sAutoScalingOffNodeSize(providerName string) (model.K8sClusterAutoScalingOffNodeSize, error) {
	providerName = strings.ToLower(providerName)

	k8sClusterDetail := getK8sClusterDetail(providerName)
	if k8sClusterDetail == nil {
		return model.K8sClusterAutoScalingOffNodeSize{}, fmt.Errorf("unsupported provider(%s) for kubernetes cluster", providerName)
	}

	return k8sClusterDetail.AutoScalingOffNodeSize, nil
}

// GetK8sNodeGroupNamingRule is func to get nodegroup's naming rule
func GetK8sNodeGroupNamingRule(providerName string) (string, error) {
	//
	// Get nodeGroupNamingRule field in k8sclusterinfo.yaml
	//

	providerName = strings.ToLower(providerName)

	// Get model.K8sClusterDetail for providerName
	k8sClusterDetail := getK8sClusterDetail(providerName)
	if k8sClusterDetail == nil {
		return "", fmt.Errorf("unsupported provider(%s) for kubernetes cluster", providerName)
	}

	namingRule := k8sClusterDetail.NodeGroupNamingRule
	if strings.EqualFold(namingRule, "") {
		namingRule = DefaultNamingRule
	}

	return namingRule, nil
}

/*
// GetModelK8sK8sNodeGroupNamingRule is to convert a K8sNodeGroupNamingRule value to model.K8sClusterK8sNodeGroupNamingRule
func GetModelK8sNodeGroupNamingRule(providerName string) (*model.K8sClusterNodeGroupsOnCreation, error) {
	k8sNodeGroupNamingRule, err := GetK8sNodeGroupNamingRule(providerName)
	if err != nil {
		return nil, err
	}

	return &model.K8sClusterNodeGroupsOnCreation{
		Result: k8sNodeGroupNamingRule,
	}, nil
}
*/

func FilterDigitsAndDots(input string) string {
	re := regexp.MustCompile(`[^0-9.]`)
	return re.ReplaceAllString(input, "")
}

func CompareVersions(version1, version2 string) int {
	v1Parts := strings.Split(version1, ".")
	v2Parts := strings.Split(version2, ".")

	// Adjust length by appending 0 if necessary
	maxLength := max(len(v2Parts), len(v1Parts))

	for i := 0; i < maxLength; i++ {
		var v1, v2 int

		// If a part is missing, treat it as 0
		if i < len(v1Parts) {
			v1, _ = strconv.Atoi(v1Parts[i])
		}
		if i < len(v2Parts) {
			v2, _ = strconv.Atoi(v2Parts[i])
		}

		// Compare each part
		if v1 > v2 {
			return 1
		} else if v1 < v2 {
			return -1
		}
	}

	return 0
}

// ExtractOSInfo extracts OS name and version from string
// and returns formatted string like "Ubuntu 22.04"
