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
	"sigs.k8s.io/yaml"
)

// ParseK8sStorageInfo parses the K8s storage catalog YAML (root key `k8sstorage`).
// Provider keys are normalized to lowercase; field values keep their case
// (StorageClass parameter keys such as Azure `skuName` are case-sensitive).
func ParseK8sStorageInfo(data []byte) (model.K8sStorageAssetInfo, error) {
	root := model.K8sStorageInfoConfig{}
	if err := yaml.Unmarshal(data, &root); err != nil {
		return model.K8sStorageAssetInfo{}, err
	}
	cfg := root.K8sStorage
	providers := make(map[string]model.K8sStorageProviderInfo, len(cfg.Providers))
	for k, v := range cfg.Providers {
		providers[strings.ToLower(k)] = v
	}
	cfg.Providers = providers
	for i := range cfg.StorageTypes {
		for j := range cfg.StorageTypes[i].Providers {
			cfg.StorageTypes[i].Providers[j].Provider = strings.ToLower(cfg.StorageTypes[i].Providers[j].Provider)
		}
	}
	return cfg, nil
}

// GetK8sStorageInfo returns the K8s storage catalog. With a non-empty provider,
// only that provider and its storage type entries are returned.
func GetK8sStorageInfo(provider string) (model.K8sStorageAssetInfo, error) {
	return FilterK8sStorageInfo(RuntimeK8sStorageInfo, provider)
}

// FilterK8sStorageInfo narrows the catalog to one provider (no-op for an empty provider).
func FilterK8sStorageInfo(catalog model.K8sStorageAssetInfo, provider string) (model.K8sStorageAssetInfo, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		return catalog, nil
	}
	p, ok := catalog.Providers[provider]
	if !ok {
		return model.K8sStorageAssetInfo{}, fmt.Errorf("provider(%s) not found in k8sstorageinfo", provider)
	}
	out := model.K8sStorageAssetInfo{
		Providers:    map[string]model.K8sStorageProviderInfo{provider: p},
		StorageTypes: make([]model.K8sStorageTypeInfo, 0, len(catalog.StorageTypes)),
	}
	for _, st := range catalog.StorageTypes {
		filtered := st
		filtered.Providers = []model.K8sStorageTypeProviderInfo{}
		for _, sp := range st.Providers {
			if sp.Provider == provider {
				filtered.Providers = append(filtered.Providers, sp)
			}
		}
		out.StorageTypes = append(out.StorageTypes, filtered)
	}
	return out, nil
}

// FindK8sStorageType returns the abstract storage type named name.
func FindK8sStorageType(catalog *model.K8sStorageAssetInfo, name string) (*model.K8sStorageTypeInfo, bool) {
	for i := range catalog.StorageTypes {
		if catalog.StorageTypes[i].Name == name {
			return &catalog.StorageTypes[i], true
		}
	}
	return nil, false
}

// ResolveK8sStorageDriverKind resolves an abstract storage type to the CB-Spider storage driver Kind
// (Block or File). It uses `driverKind` of the catalog and falls back to the access modes
// (ReadWriteMany needs a File driver).
func ResolveK8sStorageDriverKind(catalog *model.K8sStorageAssetInfo, storageType string) (string, error) {
	st, ok := FindK8sStorageType(catalog, storageType)
	if !ok {
		return "", fmt.Errorf("unknown storageType(%s)", storageType)
	}
	if kind, ok := NormalizeK8sStorageDriverKind(st.DriverKind); ok {
		return kind, nil
	}
	for _, m := range st.AccessModes {
		if m == "ReadWriteMany" {
			return model.K8sStorageDriverKindFile, nil
		}
	}
	return model.K8sStorageDriverKindBlock, nil
}

// NormalizeK8sStorageDriverKind normalizes a driver kind to Block or File (case-insensitive).
func NormalizeK8sStorageDriverKind(kind string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "block":
		return model.K8sStorageDriverKindBlock, true
	case "file":
		return model.K8sStorageDriverKindFile, true
	}
	return "", false
}

// K8sStorageKindInfoOf returns the block or file section of a provider for a driver kind.
func K8sStorageKindInfoOf(p model.K8sStorageProviderInfo, kind string) model.K8sStorageKindInfo {
	if kind == model.K8sStorageDriverKindFile {
		return p.FileStorage
	}
	return p.BlockStorage
}

// NormalizeK8sStorageTypes trims, de-duplicates and validates storage types against the catalog.
// An empty list means ["ssd-general"].
func NormalizeK8sStorageTypes(catalog *model.K8sStorageAssetInfo, storageTypes []string) ([]string, error) {
	out := []string{}
	seen := map[string]bool{}
	for _, t := range storageTypes {
		for _, part := range strings.Split(t, ",") {
			part = strings.TrimSpace(part)
			if part == "" || seen[part] {
				continue
			}
			if _, ok := FindK8sStorageType(catalog, part); !ok {
				return nil, fmt.Errorf("unknown storageType(%s): available storage types are %s", part, strings.Join(K8sStorageTypeNames(catalog), ", "))
			}
			seen[part] = true
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		if _, ok := FindK8sStorageType(catalog, model.K8sStorageTypeSsdGeneral); !ok {
			return nil, fmt.Errorf("default storageType(%s) not found in k8sstorageinfo", model.K8sStorageTypeSsdGeneral)
		}
		out = append(out, model.K8sStorageTypeSsdGeneral)
	}
	return out, nil
}

// K8sStorageTypeNames returns the names of all abstract storage types in the catalog.
func K8sStorageTypeNames(catalog *model.K8sStorageAssetInfo) []string {
	names := make([]string, 0, len(catalog.StorageTypes))
	for _, st := range catalog.StorageTypes {
		names = append(names, st.Name)
	}
	return names
}
