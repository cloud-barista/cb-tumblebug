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
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/rs/zerolog/log"
)

func GenInfraKey(nsId string, infraId string, nodeId string) string {

	if nodeId != "" {
		return "/" + model.StrNamespace + "/" + nsId + "/" + model.StrInfra + "/" + infraId + "/" + model.StrNode + "/" + nodeId
	} else if infraId != "" {
		return "/" + model.StrNamespace + "/" + nsId + "/" + model.StrInfra + "/" + infraId
	} else if nsId != "" {
		return "/" + model.StrNamespace + "/" + nsId
	} else {
		return ""
	}

}

// GenInfraNodeGroupKey is func to generate a key from nodeGroupId used in keyValue store.
// The group id is canonicalized the same way Node ids are (ToLower), so a record written
// from a mixed-case request name stays reachable by the id Nodes actually carry.
func GenInfraNodeGroupKey(nsId string, infraId string, groupId string) string {

	return "/" + model.StrNamespace + "/" + nsId + "/" + model.StrInfra + "/" + infraId + "/" + model.StrNodeGroup + "/" + ToLower(groupId)

}

// GenInfraNodeDetailsKey generates the key for a Node's auxiliary details
// (CSP raw metadata), stored separately from the Node record so status and bulk
// reads/writes do not carry it.
func GenInfraNodeDetailsKey(nsId string, infraId string, nodeId string) string {

	return "/" + model.StrNamespace + "/" + nsId + "/" + model.StrInfra + "/" + infraId + "/" + model.StrNodeDetails + "/" + nodeId

}

// GenInfraNodeInitScriptKey generates the key for a Node's init script result (written by the notifier handler)
func GenInfraNodeInitScriptKey(nsId string, infraId string, nodeId string) string {
	return "/" + model.StrNamespace + "/" + nsId + "/" + model.StrInfra + "/" + infraId + "/" + model.StrInitScript + "/" + nodeId
}

// GenInfraPolicyKey is func to generate Infra policy key
func GenInfraPolicyKey(nsId string, infraId string, nodeId string) string {
	if nodeId != "" {
		return "/" + model.StrNamespace + "/" + nsId + "/policy/" + model.StrInfra + "/" + infraId + "/" + model.StrNode + "/" + nodeId
	} else if infraId != "" {
		return "/" + model.StrNamespace + "/" + nsId + "/policy/" + model.StrInfra + "/" + infraId
	} else if nsId != "" {
		return "/" + model.StrNamespace + "/" + nsId
	} else {
		return ""
	}
}

// GenConnectionKey is func to generate a key for connection info
func GenConnectionKey(connectionId string) string {
	return "/connection/" + connectionId
}

// GenTemplateKey is func to generate a key for template stored in ETCD
// Key format: /ns/{nsId}/template/{targetType}/{templateId}
func GenTemplateKey(nsId string, targetType string, templateId string) string {
	if nsId == "" {
		return ""
	}
	if targetType == "" {
		return "/ns/" + nsId + "/template"
	}
	if templateId == "" {
		return "/ns/" + nsId + "/template/" + targetType
	}
	return "/ns/" + nsId + "/template/" + targetType + "/" + templateId
}

// GetCredentialHolderList derives distinct credential holders from registered connection configs

func GenResourceKey(nsId string, resourceType string, resourceId string) string {

	if strings.EqualFold(resourceType, model.StrImage) ||
		strings.EqualFold(resourceType, model.StrCustomImage) ||
		strings.EqualFold(resourceType, model.StrSSHKey) ||
		strings.EqualFold(resourceType, model.StrSpec) ||
		strings.EqualFold(resourceType, model.StrVNet) ||
		strings.EqualFold(resourceType, model.StrVPN) ||
		strings.EqualFold(resourceType, model.StrRDBMS) ||
		strings.EqualFold(resourceType, model.StrObjectStorage) ||
		strings.EqualFold(resourceType, model.StrSecurityGroup) ||
		strings.EqualFold(resourceType, model.StrDataDisk) {
		//resourceType == "publicIp" ||
		//resourceType == "vNic" {
		return "/ns/" + nsId + "/resources/" + resourceType + "/" + resourceId
	} else {
		return "/invalidKey"
	}
}

// GenK8sClusterKey is func to generate a key from K8sCluster ID
func GenK8sClusterKey(nsId string, k8sClusterId string) string {
	err := CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "/invalidKey"
	}

	err = CheckString(k8sClusterId)
	if err != nil {
		log.Err(err).Msg("Failed to Generate K8sCluster Key")
		return "/invalidKey"
	}

	return fmt.Sprintf("/ns/%s/k8scluster/%s", nsId, k8sClusterId)
}

// GenChildResourceKey is func to generate a key from resource type and id
func GenChildResourceKey(nsId string, resourceType string, parentResourceId string, resourceId string) string {

	if strings.EqualFold(resourceType, model.StrSubnet) {
		parentResourceType := model.StrVNet
		// return "/ns/" + nsId + "/resources/" + resourceType + "/" + resourceId
		return fmt.Sprintf("/ns/%s/resources/%s/%s/%s/%s", nsId, parentResourceType, parentResourceId, resourceType, resourceId)
	} else {
		return "/invalidKey"
	}
}

// GetConnConfig is func to get connection config
