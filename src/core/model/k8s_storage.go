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

// Package model is to handle object of CB-Tumblebug
package model

import "time"

/*
 * K8s storage catalog (assets/k8sstorageinfo.yaml)
 */

// K8sStorageInfoConfig is the root of assets/k8sstorageinfo.yaml.
type K8sStorageInfoConfig struct {
	K8sStorage K8sStorageAssetInfo `json:"k8sstorage"`
}

// K8sStorageAssetInfo is the K8s storage catalog (the "k8sstorage" section of assets/k8sstorageinfo.yaml).
type K8sStorageAssetInfo struct {
	// Providers maps a CSP name (lowercase) to its StorageClass details
	Providers map[string]K8sStorageProviderInfo `json:"providers"`
	// StorageTypes is the list of CB-Tumblebug abstract storage types
	StorageTypes []K8sStorageTypeInfo `json:"storageTypes"`
}

// K8sStorageProviderInfo holds the StorageClass details of one CSP.
type K8sStorageProviderInfo struct {
	Verified     bool               `json:"verified"`
	BlockStorage K8sStorageKindInfo `json:"blockStorage"`
}

// K8sStorageKindInfo holds the StorageClass details of one storage kind (block) of a CSP.
type K8sStorageKindInfo struct {
	// Provisioners recognized for this kind (classification)
	Provisioners []string `json:"provisioners,omitempty"`
	// SkuKeys is the priority-ordered list of parameter keys holding the SKU (classification)
	SkuKeys []string `json:"skuKeys,omitempty"`
	// IgnoreValues are sentinel SKU values to skip (classification)
	IgnoreValues []string `json:"ignoreValues,omitempty"`
	// InstalledByDefault tells whether the CSI driver ships enabled
	InstalledByDefault *bool `json:"installedByDefault,omitempty"`
	// MinVolumeSizeGi is the smallest volume the CSP accepts for this kind (0 = no known minimum)
	MinVolumeSizeGi int `json:"minVolumeSizeGi,omitempty" example:"20"`

	// ScParameterKey is the exact parameter key written for the SKU in a generated StorageClass
	ScParameterKey string `json:"scParameterKey,omitempty" example:"type"`
	// ScParameters are fixed extra parameters written to a generated StorageClass
	ScParameters map[string]string `json:"scParameters,omitempty"`
	// VolumeBindingMode of a generated StorageClass (default: WaitForFirstConsumer)
	VolumeBindingMode string `json:"volumeBindingMode,omitempty" example:"WaitForFirstConsumer"`
	// AllowVolumeExpansion of a generated StorageClass (default: true)
	AllowVolumeExpansion *bool `json:"allowVolumeExpansion,omitempty"`
}

// K8sStorageTypeInfo is a CB-Tumblebug abstract storage type.
type K8sStorageTypeInfo struct {
	Name        string `json:"name" example:"ssd-general"`
	Description string `json:"description,omitempty"`
	ResolveBy   string `json:"resolveBy,omitempty" example:"defaultStorageClass"`
	// DriverKind is the CB-Spider storage driver Kind this type needs (Block)
	DriverKind  string                       `json:"driverKind,omitempty" example:"Block"`
	AccessModes []string                     `json:"accessModes,omitempty"`
	Providers   []K8sStorageTypeProviderInfo `json:"providers"`
}

// K8sStorageTypeProviderInfo is the CSP-specific realization of an abstract storage type.
type K8sStorageTypeProviderInfo struct {
	Provider    string `json:"provider" example:"aws"`
	Provisioner string `json:"provisioner" example:"ebs.csi.aws.com"`
	// Id is the CSP SKU value (e.g., gp3). Written as parameters[scParameterKey].
	Id       string `json:"id,omitempty" example:"gp3"`
	Verified bool   `json:"verified"`
}

// Abstract storage type names and defaults
const (
	K8sStorageTypeSsdGeneral = "ssd-general"
)

/*
 * CB-Spider storage driver (cluster CSI driver) API
 */

// K8s storage driver kinds (CB-Spider StorageDriverKind)
const (
	K8sStorageDriverKindBlock = "Block"
)

// K8s storage driver management modes (CB-Spider StorageDriverManagementMode)
const (
	K8sStorageDriverModeAddon        = "Addon"
	K8sStorageDriverModeBuiltIn      = "BuiltIn"
	K8sStorageDriverModeNotSupported = "NotSupported"
)

// K8s storage driver statuses (CB-Spider StorageDriverStatus)
const (
	K8sStorageDriverEnabled  = "Enabled"
	K8sStorageDriverDisabled = "Disabled"
	K8sStorageDriverUpdating = "Updating"
	K8sStorageDriverFailed   = "Failed"
	K8sStorageDriverUnknown  = "Unknown"
)

// SpiderStorageDriverInfo is the CB-Spider StorageDriverInfo
type SpiderStorageDriverInfo struct {
	Kind           string
	CSPDriverName  string
	CSIDriverName  string
	ManagementMode string
	Status         string
	Version        string
	KeyValueList   []KeyValue
}

// SpiderStorageDriverListRes is the CB-Spider response of 'GET /cluster/{Name}/storagedriver'
type SpiderStorageDriverListRes struct {
	Result []SpiderStorageDriverInfo `json:"storagedriver"`
}

// SpiderStorageDriverEnableReq is the CB-Spider request of 'PUT /cluster/{Name}/storagedriver'
type SpiderStorageDriverEnableReq struct {
	ConnectionName string
	ReqInfo        SpiderStorageDriverEnableReqInfo
}

// SpiderStorageDriverEnableReqInfo is the ReqInfo of SpiderStorageDriverEnableReq
type SpiderStorageDriverEnableReqInfo struct {
	Kind    string
	Version string `json:"Version,omitempty"`
}

// K8sStorageDriverInfo is a CSP-managed CSI storage driver of a K8sCluster (CB-Tumblebug view of CB-Spider StorageDriverInfo)
type K8sStorageDriverInfo struct {
	// Kind is Block
	Kind string `json:"kind" example:"Block"`
	// CspDriverName is the CSP add-on name as-is
	CspDriverName string `json:"cspDriverName,omitempty" example:"aws-ebs-csi-driver"`
	// CsiDriverName is the name of the Kubernetes CSIDriver object
	CsiDriverName string `json:"csiDriverName,omitempty" example:"ebs.csi.aws.com"`
	// ManagementMode is Addon, BuiltIn or NotSupported
	ManagementMode string `json:"managementMode" example:"Addon"`
	// Status is Enabled, Disabled, Updating, Failed or Unknown
	Status       string     `json:"status" example:"Enabled"`
	Version      string     `json:"version,omitempty" example:"v1.37.0-eksbuild.1"`
	KeyValueList []KeyValue `json:"keyValueList,omitempty"`
}

// K8sStorageDriverListResponse is the response of 'GET /ns/{nsId}/k8sCluster/{k8sClusterId}/storageDriver'
type K8sStorageDriverListResponse struct {
	K8sClusterId   string                 `json:"k8sClusterId" example:"k8scluster01"`
	StorageDrivers []K8sStorageDriverInfo `json:"storageDrivers"`
}

// K8sStorageDriverEnableReq is the request of 'PUT /ns/{nsId}/k8sCluster/{k8sClusterId}/storageDriver'
type K8sStorageDriverEnableReq struct {
	// Kind is Block (the only supported kind)
	Kind string `json:"kind" validate:"required" example:"Block" enums:"Block"`
	// Version of the driver. Empty means the CSP default version.
	Version string `json:"version,omitempty" example:""`
}

/*
 * Storage preparation (abstract storage types)
 */

// K8sStorageStatus is the CSP-side readiness of an abstract storage type on a K8sCluster
type K8sStorageStatus string

const (
	K8sStorageReady     K8sStorageStatus = "Ready"
	K8sStorageNotReady  K8sStorageStatus = "NotReady"
	K8sStoragePreparing K8sStorageStatus = "Preparing"
)

// K8sClusterStorageReq is the storage option of K8sCluster creation requests
// and the request of 'PUT /ns/{nsId}/k8sCluster/{k8sClusterId}/storage'
type K8sClusterStorageReq struct {
	// StorageTypes to prepare. Empty means ["ssd-general"].
	StorageTypes []string `json:"storageTypes,omitempty" example:"ssd-general"`
}

// K8sStorageDriverRef is the storage driver summary recorded for an abstract storage type
type K8sStorageDriverRef struct {
	Kind           string `json:"kind" example:"Block"`
	CsiDriverName  string `json:"csiDriverName,omitempty" example:"ebs.csi.aws.com"`
	Status         string `json:"status,omitempty" example:"Enabled"`
	ManagementMode string `json:"managementMode,omitempty" example:"Addon"`
}

// K8sStorageTypeState is the preparation state of one abstract storage type on a K8sCluster
type K8sStorageTypeState struct {
	StorageType string               `json:"storageType" example:"ssd-general"`
	Status      K8sStorageStatus     `json:"status" example:"Ready"`
	Reason      string               `json:"reason,omitempty" example:""`
	Driver      *K8sStorageDriverRef `json:"driver,omitempty"`
}

// K8sStorageAction is a CSP-side change made by CB-Tumblebug while preparing storage
type K8sStorageAction struct {
	Step   string    `json:"step" example:"enableStorageDriver"`
	Kind   string    `json:"kind,omitempty" example:"Block"`
	Target string    `json:"target,omitempty" example:""`
	From   string    `json:"from,omitempty" example:"Disabled"`
	To     string    `json:"to,omitempty" example:"Enabled"`
	Time   time.Time `json:"time,omitempty"`
}

// K8sClusterStorageInfo is the storage state recorded in the K8sCluster object (K8sClusterInfo.Storage)
type K8sClusterStorageInfo struct {
	Status       K8sStorageStatus      `json:"status" example:"Ready"`
	StorageTypes []K8sStorageTypeState `json:"storageTypes"`
	Actions      []K8sStorageAction    `json:"actions,omitempty"`
	UpdatedAt    time.Time             `json:"updatedAt"`
}

// K8sClusterStoragePrepareResponse is the 202 response of 'PUT /ns/{nsId}/k8sCluster/{k8sClusterId}/storage'
type K8sClusterStoragePrepareResponse struct {
	K8sClusterId string           `json:"k8sClusterId" example:"k8scluster01"`
	Status       K8sStorageStatus `json:"status" example:"Preparing"`
	StorageTypes []string         `json:"storageTypes" example:"ssd-general"`
	// Reason is set when the cluster was not accepted (multi-cluster request only)
	Reason string `json:"reason,omitempty"`
}

// K8sClusterStorageStatusResponse is the response of 'GET /ns/{nsId}/k8sCluster/{k8sClusterId}/storage'
type K8sClusterStorageStatusResponse struct {
	K8sClusterId string                `json:"k8sClusterId" example:"k8scluster01"`
	Provider     string                `json:"provider" example:"aws"`
	Status       K8sStorageStatus      `json:"status" example:"Ready"`
	Reason       string                `json:"reason,omitempty"`
	StorageTypes []K8sStorageTypeState `json:"storageTypes"`
	Actions      []K8sStorageAction    `json:"actions,omitempty"`
	UpdatedAt    *time.Time            `json:"updatedAt,omitempty"`
}

// K8sMultiClusterStorageReq is the request of 'PUT /ns/{nsId}/k8sStorage'
type K8sMultiClusterStorageReq struct {
	// K8sClusterIds to prepare. Empty means all K8sClusters in the namespace.
	K8sClusterIds []string `json:"k8sClusterIds,omitempty"`
	// StorageTypes to prepare. Empty means ["ssd-general"].
	StorageTypes []string `json:"storageTypes,omitempty" example:"ssd-general"`
}

// K8sMultiClusterStoragePrepareResponse is the 202 response of 'PUT /ns/{nsId}/k8sStorage'
type K8sMultiClusterStoragePrepareResponse struct {
	NsId    string                             `json:"nsId" example:"default"`
	Results []K8sClusterStoragePrepareResponse `json:"results"`
}

// K8sStorageSummary counts K8sClusters by storage status
type K8sStorageSummary struct {
	Total     int `json:"total" example:"7"`
	Ready     int `json:"ready" example:"6"`
	Preparing int `json:"preparing" example:"0"`
	NotReady  int `json:"notReady" example:"1"`
}

// K8sMultiClusterStorageStatusResponse is the response of 'GET /ns/{nsId}/k8sStorage'
type K8sMultiClusterStorageStatusResponse struct {
	NsId    string                            `json:"nsId" example:"default"`
	Summary K8sStorageSummary                 `json:"summary"`
	Results []K8sClusterStorageStatusResponse `json:"results"`
}

/*
 * StorageClass manifest
 */

// Labels and annotations of generated StorageClasses
const (
	K8sStorageClassNamePrefix       = "cb-"
	K8sStorageLabelManagedBy        = "app.kubernetes.io/managed-by"
	K8sStorageLabelManagedByValue   = "cb-tumblebug"
	K8sStorageLabelType             = "storage.cloud-barista.org/type"
	K8sStorageLabelSku              = "storage.cloud-barista.org/sku"
	K8sStorageAnnotationDefaultSC   = "storageclass.kubernetes.io/is-default-class"
	K8sStorageClassAPIVersion       = "storage.k8s.io/v1"
	K8sStorageClassKind             = "StorageClass"
	K8sStorageClassReclaimDelete    = "Delete"
	K8sStorageBindingWaitForConsume = "WaitForFirstConsumer"
)

// K8sStorageClassManifestMetadata is the metadata of a generated StorageClass
type K8sStorageClassManifestMetadata struct {
	Name        string            `json:"name" example:"cb-ssd-general"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// K8sStorageClassManifest is a generated Kubernetes StorageClass (storage.k8s.io/v1)
type K8sStorageClassManifest struct {
	APIVersion           string                          `json:"apiVersion" example:"storage.k8s.io/v1"`
	Kind                 string                          `json:"kind" example:"StorageClass"`
	Metadata             K8sStorageClassManifestMetadata `json:"metadata"`
	Provisioner          string                          `json:"provisioner" example:"ebs.csi.aws.com"`
	Parameters           map[string]string               `json:"parameters,omitempty"`
	ReclaimPolicy        string                          `json:"reclaimPolicy,omitempty" example:"Delete"`
	VolumeBindingMode    string                          `json:"volumeBindingMode,omitempty" example:"WaitForFirstConsumer"`
	AllowVolumeExpansion *bool                           `json:"allowVolumeExpansion,omitempty"`
}

// K8sStorageClassManifestList is the JSON response of the StorageClass manifest API (a Kubernetes List)
type K8sStorageClassManifestList struct {
	APIVersion string                    `json:"apiVersion" example:"v1"`
	Kind       string                    `json:"kind" example:"List"`
	Items      []K8sStorageClassManifest `json:"items"`
}

// K8sClusterStorageClassManifestResult is the per-cluster result of 'GET /ns/{nsId}/k8sStorageClassManifest'
type K8sClusterStorageClassManifestResult struct {
	K8sClusterId string `json:"k8sClusterId" example:"k8scluster01"`
	Provider     string `json:"provider,omitempty" example:"aws"`
	// Status is Ready when a manifest was generated for all requested storage types, otherwise NotReady
	Status K8sStorageStatus `json:"status" example:"Ready"`
	Reason string           `json:"reason,omitempty"`
	// Manifest is the multi-document StorageClass YAML (empty when Status is NotReady)
	Manifest string `json:"manifest,omitempty"`
}

// K8sMultiClusterStorageClassManifestResponse is the response of 'GET /ns/{nsId}/k8sStorageClassManifest'
type K8sMultiClusterStorageClassManifestResponse struct {
	NsId    string                                 `json:"nsId" example:"default"`
	Results []K8sClusterStorageClassManifestResult `json:"results"`
}
