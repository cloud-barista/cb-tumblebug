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

// Package resource is to manage multi-cloud infra resource
package resource

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"sigs.k8s.io/yaml"
)

// GenerateK8sStorageClassManifests builds the common abstract StorageClasses (cb-<storageType>)
// for one provider. Provisioner and parameters come from the catalog only (TB never reads
// the cluster); file system parameters come from the file system TB recorded in state.
//
// Errors carry an HTTP status (see K8sStorageErrorCode):
//   - 400: unknown storage type
//   - 422: no catalog entry, or the entry/provider is not `verified: true`,
//     or the catalog needs a file system that was not recorded
//   - 412: the storage type is not Ready in the recorded storage state
func GenerateK8sStorageClassManifests(catalog *model.K8sStorageAssetInfo, provider string, storageTypes []string, setDefault bool, state *model.K8sClusterStorageInfo) ([]model.K8sStorageClassManifest, error) {
	provider = strings.ToLower(provider)
	types, err := common.NormalizeK8sStorageTypes(catalog, storageTypes)
	if err != nil {
		return nil, newK8sStorageError(http.StatusBadRequest, "%v", err)
	}

	manifests := make([]model.K8sStorageClassManifest, 0, len(types))
	for _, storageType := range types {
		kind, err := common.ResolveK8sStorageDriverKind(catalog, storageType)
		if err != nil {
			return nil, newK8sStorageError(http.StatusBadRequest, "%v", err)
		}

		providerInfo, ok := catalog.Providers[provider]
		if !ok {
			return nil, newK8sStorageError(http.StatusUnprocessableEntity, "provider(%s) not found in k8sstorageinfo", provider)
		}
		if !providerInfo.Verified {
			return nil, newK8sStorageError(http.StatusUnprocessableEntity, "provider(%s) is not verified in k8sstorageinfo; refusing to generate a StorageClass", provider)
		}

		var typeState *model.K8sStorageTypeState
		if state != nil {
			for i := range state.StorageTypes {
				if state.StorageTypes[i].StorageType == storageType {
					typeState = &state.StorageTypes[i]
					break
				}
			}
		}

		preferredProvisioner := ""
		if typeState != nil && typeState.Driver != nil {
			preferredProvisioner = typeState.Driver.CsiDriverName
		}
		entry, err := selectK8sStorageTypeProvider(catalog, storageType, provider, preferredProvisioner)
		if err != nil {
			return nil, err
		}

		if typeState == nil || typeState.Status != model.K8sStorageReady {
			status := "not prepared"
			reason := "call PUT /ns/{nsId}/k8sCluster/{k8sClusterId}/storage first"
			if typeState != nil {
				status = string(typeState.Status)
				if typeState.Reason != "" {
					reason = typeState.Reason
				}
			}
			return nil, newK8sStorageError(http.StatusPreconditionFailed, "storageType(%s) is not Ready (%s): %s", storageType, status, reason)
		}

		kindInfo := common.K8sStorageKindInfoOf(providerInfo, kind)
		var network k8sStorageClusterNetwork
		if state != nil {
			network = k8sStorageClusterNetwork{VpcCspId: state.ClusterVpcCspId, SubnetCspIds: state.ClusterSubnetCspIds}
		}
		m, err := buildK8sStorageClassManifest(storageType, kind, entry, kindInfo, typeState.FileSystem, network, setDefault)
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, m)
	}
	return manifests, nil
}

// selectK8sStorageTypeProvider picks the catalog entry of a provider for a storage type.
// The entry whose provisioner equals preferredProvisioner wins; otherwise the first entry.
// The selected entry must be `verified: true`.
func selectK8sStorageTypeProvider(catalog *model.K8sStorageAssetInfo, storageType, provider, preferredProvisioner string) (model.K8sStorageTypeProviderInfo, error) {
	st, ok := common.FindK8sStorageType(catalog, storageType)
	if !ok {
		return model.K8sStorageTypeProviderInfo{}, newK8sStorageError(http.StatusBadRequest, "unknown storageType(%s)", storageType)
	}
	var candidates []model.K8sStorageTypeProviderInfo
	for _, p := range st.Providers {
		if p.Provider == provider {
			candidates = append(candidates, p)
		}
	}
	if len(candidates) == 0 {
		return model.K8sStorageTypeProviderInfo{}, newK8sStorageError(http.StatusUnprocessableEntity, "storageType(%s) has no catalog entry for provider(%s)", storageType, provider)
	}
	selected := candidates[0]
	if preferredProvisioner != "" {
		for _, c := range candidates {
			if c.Provisioner == preferredProvisioner {
				selected = c
				break
			}
		}
	}
	if !selected.Verified {
		return model.K8sStorageTypeProviderInfo{}, newK8sStorageError(http.StatusUnprocessableEntity, "catalog entry of storageType(%s) for provider(%s) (provisioner %s) is not verified; refusing to generate a StorageClass", storageType, provider, selected.Provisioner)
	}
	return selected, nil
}

// k8sLabelValue converts a CSP SKU into a valid Kubernetes label value: at most 63 characters of
// [A-Za-z0-9-_.], starting and ending with an alphanumeric. Invalid characters become "-"
// (e.g., NHN Cinder type "General SSD" -> "General-SSD"). The StorageClass parameter keeps the raw value.
func k8sLabelValue(v string) string {
	b := []byte(v)
	for i, c := range b {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
			b[i] = '-'
		}
	}
	s := string(b)
	if len(s) > 63 {
		s = s[:63]
	}
	return strings.Trim(s, "-_.")
}

// Cluster network placeholders in catalog scParameters: {{cluster.vpcId}} is replaced with the cluster
// VPC CSP ID, {{cluster.subnetId}} with the first cluster subnet CSP ID.
const (
	k8sStorageClusterVpcPlaceholder    = "{{cluster.vpcId}}"
	k8sStorageClusterSubnetPlaceholder = "{{cluster.subnetId}}"
)

// k8sStorageClusterNetwork is the recorded cluster network the placeholders resolve to.
type k8sStorageClusterNetwork struct {
	VpcCspId     string
	SubnetCspIds []string
}

func buildK8sStorageClassManifest(storageType, kind string, entry model.K8sStorageTypeProviderInfo, kindInfo model.K8sStorageKindInfo, fs *model.K8sStorageFileSystemState, network k8sStorageClusterNetwork, setDefault bool) (model.K8sStorageClassManifest, error) {
	labels := map[string]string{
		model.K8sStorageLabelManagedBy: model.K8sStorageLabelManagedByValue,
		model.K8sStorageLabelType:      storageType,
	}
	if sku := k8sLabelValue(entry.Id); sku != "" {
		labels[model.K8sStorageLabelSku] = sku
	}

	params := map[string]string{}
	for k, v := range kindInfo.ScParameters {
		if strings.Contains(v, k8sStorageClusterSubnetPlaceholder) {
			if len(network.SubnetCspIds) == 0 {
				return model.K8sStorageClassManifest{}, newK8sStorageError(http.StatusUnprocessableEntity, "storageType(%s) needs a cluster subnet ID but none is recorded; prepare it again with PUT .../storage", storageType)
			}
			v = strings.ReplaceAll(v, k8sStorageClusterSubnetPlaceholder, network.SubnetCspIds[0])
		}
		if strings.Contains(v, k8sStorageClusterVpcPlaceholder) {
			if network.VpcCspId == "" {
				return model.K8sStorageClassManifest{}, newK8sStorageError(http.StatusUnprocessableEntity, "storageType(%s) needs the cluster VPC ID but none is recorded; prepare it again with PUT .../storage", storageType)
			}
			v = strings.ReplaceAll(v, k8sStorageClusterVpcPlaceholder, network.VpcCspId)
		}
		params[k] = v
	}
	if kindInfo.ScParameterKey != "" && entry.Id != "" {
		params[kindInfo.ScParameterKey] = entry.Id
	}
	if len(kindInfo.FileSystemParameters) > 0 && kind == model.K8sStorageDriverKindFile {
		if fs == nil {
			return model.K8sStorageClassManifest{}, newK8sStorageError(http.StatusUnprocessableEntity, "storageType(%s) needs a file system but none is recorded; prepare it with PUT .../storage and fileSystem", storageType)
		}
		for k, v := range kindInfo.FileSystemParameters {
			params[k] = expandK8sStorageFileSystemPlaceholders(v, fs)
		}
	}
	if len(params) == 0 {
		params = nil
	}

	bindingMode := kindInfo.VolumeBindingMode
	if bindingMode == "" {
		bindingMode = model.K8sStorageBindingWaitForConsume
		if kind == model.K8sStorageDriverKindFile {
			bindingMode = model.K8sStorageBindingImmediate
		}
	}
	allowExpansion := true
	if kindInfo.AllowVolumeExpansion != nil {
		allowExpansion = *kindInfo.AllowVolumeExpansion
	}

	m := model.K8sStorageClassManifest{
		APIVersion: model.K8sStorageClassAPIVersion,
		Kind:       model.K8sStorageClassKind,
		Metadata: model.K8sStorageClassManifestMetadata{
			Name:   model.K8sStorageClassNamePrefix + storageType,
			Labels: labels,
		},
		Provisioner:          entry.Provisioner,
		Parameters:           params,
		ReclaimPolicy:        model.K8sStorageClassReclaimDelete,
		VolumeBindingMode:    bindingMode,
		AllowVolumeExpansion: &allowExpansion,
	}
	if setDefault {
		m.Metadata.Annotations = map[string]string{model.K8sStorageAnnotationDefaultSC: "true"}
	}
	return m, nil
}

// expandK8sStorageFileSystemPlaceholders fills {{fileSystem.*}} placeholders of a catalog value.
func expandK8sStorageFileSystemPlaceholders(v string, fs *model.K8sStorageFileSystemState) string {
	host, path := splitK8sStorageEndpoint(fs.Endpoint)
	r := strings.NewReplacer(
		"{{fileSystem.name}}", fs.Name,
		"{{fileSystem.cspId}}", fs.CspId,
		"{{fileSystem.endpointHost}}", host,
		"{{fileSystem.endpointPath}}", path,
		"{{fileSystem.endpoint}}", fs.Endpoint,
	)
	return r.Replace(v)
}

// splitK8sStorageEndpoint splits an NFS endpoint "host:/path" into host and path ("/" when absent).
func splitK8sStorageEndpoint(endpoint string) (string, string) {
	endpoint = strings.TrimSpace(endpoint)
	if i := strings.Index(endpoint, ":/"); i >= 0 {
		return endpoint[:i], endpoint[i+1:]
	}
	return endpoint, "/"
}

// MarshalK8sStorageClassManifestsYAML renders manifests as a multi-document YAML stream.
func MarshalK8sStorageClassManifestsYAML(manifests []model.K8sStorageClassManifest) (string, error) {
	docs := make([]string, 0, len(manifests))
	for _, m := range manifests {
		b, err := yaml.Marshal(m)
		if err != nil {
			return "", fmt.Errorf("failed to marshal StorageClass(%s): %w", m.Metadata.Name, err)
		}
		docs = append(docs, string(b))
	}
	return strings.Join(docs, "---\n"), nil
}

// NewK8sStorageClassManifestList wraps manifests into a Kubernetes List object.
func NewK8sStorageClassManifestList(manifests []model.K8sStorageClassManifest) model.K8sStorageClassManifestList {
	if manifests == nil {
		manifests = []model.K8sStorageClassManifest{}
	}
	return model.K8sStorageClassManifestList{APIVersion: "v1", Kind: "List", Items: manifests}
}

// summarizeK8sStorage counts cluster results by status.
func summarizeK8sStorage(results []model.K8sClusterStorageStatusResponse) model.K8sStorageSummary {
	s := model.K8sStorageSummary{Total: len(results)}
	for _, r := range results {
		switch r.Status {
		case model.K8sStorageReady:
			s.Ready++
		case model.K8sStoragePreparing:
			s.Preparing++
		default:
			s.NotReady++
		}
	}
	return s
}

// aggregateK8sStorageStatus derives the cluster-level status from the per-type statuses.
func aggregateK8sStorageStatus(types []model.K8sStorageTypeState) model.K8sStorageStatus {
	if len(types) == 0 {
		return model.K8sStorageNotReady
	}
	ready := true
	for _, t := range types {
		if t.Status == model.K8sStoragePreparing {
			return model.K8sStoragePreparing
		}
		if t.Status != model.K8sStorageReady {
			ready = false
		}
	}
	if ready {
		return model.K8sStorageReady
	}
	return model.K8sStorageNotReady
}
