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

// Package model defines data structures for Tumblebug
package model

// ---- assets/diskinfo.yaml (source of truth, CSP-native terms) ----

// DiskInfoConfig mirrors assets/diskinfo.yaml, the static per-CSP root/data disk reference.
type DiskInfoConfig struct {
	Disk map[string]DiskProviderConfig `yaml:"disk"`
}

// DiskProviderConfig is one CSP's entry under "disk" in assets/diskinfo.yaml.
type DiskProviderConfig struct {
	Description string            `yaml:"description"`
	Links       map[string]string `yaml:"links,omitempty"`
	Note        string            `yaml:"note,omitempty"`
	// RootDiskSelectable/DataDiskSelectable: whether the type can be chosen at creation at all.
	RootDiskSelectable bool `yaml:"rootDiskSelectable"`
	DataDiskSelectable bool `yaml:"dataDiskSelectable"`
	// DefaultRootDiskType/DefaultDataDiskType: what "default" (or empty) resolves to.
	DefaultRootDiskType string `yaml:"defaultRootDiskType,omitempty"`
	DefaultDataDiskType string `yaml:"defaultDataDiskType,omitempty"`
	// DiskTypeOrder is the display order (map keys are unordered).
	DiskTypeOrder []string                  `yaml:"diskTypeOrder,omitempty"`
	DiskTypes     map[string]DiskTypeConfig `yaml:"diskTypes"`
}

// DiskTypeConfig is one disk type, keyed by the CSP-native identifier.
type DiskTypeConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	Note        string `yaml:"note,omitempty"`
	// CBSpiderName is set only when CB-Spider expects a different identifier than the CSP-native one;
	// CB-Tumblebug translates the user's CSP-native value to it before calling CB-Spider.
	CBSpiderName string `yaml:"cbSpiderName,omitempty"`
	DiskTypeRule `yaml:",inline"`
	// Availability scopes the type to regions/zones/spec patterns; empty means everywhere.
	Availability *DiskAvailability `yaml:"availability,omitempty"`
}

// DiskTypeRule is the root/data applicability and size constraints of a disk type.
type DiskTypeRule struct {
	RootDisk       bool                          `yaml:"rootDisk" json:"rootDisk"`
	DataDisk       bool                          `yaml:"dataDisk" json:"dataDisk"`
	RootDiskSizeGB *DiskSizeConstraint           `yaml:"rootDiskSizeGB,omitempty" json:"rootDiskSizeGB,omitempty"`
	DataDiskSizeGB *DiskSizeConstraint           `yaml:"dataDiskSizeGB,omitempty" json:"dataDiskSizeGB,omitempty"`
	ByOS           map[string]DiskSizeConstraint `yaml:"byOS,omitempty" json:"byOS,omitempty"` // root disk size override per OS family (e.g. windows)
}

// DiskSizeConstraint is a size rule in GB: a min/max range, an optional step, or a discrete list.
type DiskSizeConstraint struct {
	Min     int   `yaml:"min,omitempty" json:"min,omitempty" example:"10"`
	Max     int   `yaml:"max,omitempty" json:"max,omitempty" example:"16384"`
	Step    int   `yaml:"step,omitempty" json:"step,omitempty" example:"10"`
	Allowed []int `yaml:"allowed,omitempty" json:"allowed,omitempty" example:"50,100"`
}

// DiskAvailability scopes a disk type. only/except per axis; patterns are case-insensitive globs.
type DiskAvailability struct {
	Regions      *DiskScope `yaml:"regions,omitempty" json:"regions,omitempty"`
	Zones        *DiskScope `yaml:"zones,omitempty" json:"zones,omitempty"`
	SpecPatterns *DiskScope `yaml:"specPatterns,omitempty" json:"specPatterns,omitempty"`
	Note         string     `yaml:"note,omitempty" json:"note,omitempty"`
}

// DiskScope is an include-only or exclude list.
type DiskScope struct {
	Only   []string `yaml:"only,omitempty" json:"only,omitempty"`
	Except []string `yaml:"except,omitempty" json:"except,omitempty"`
}

// ---- API responses ----

// DiskSupportResponse wraps GET /disk/support (static reference matrix).
type DiskSupportResponse struct {
	ResourceType string                        `json:"resourceType" example:"disk"`
	View         string                        `json:"view" example:"available" enums:"available,all"`
	RegionName   string                        `json:"regionName,omitempty" example:"ap-northeast-2"`
	Supports     map[string]DiskCSPSupportInfo `json:"supports"`
}

// DiskCSPSupportInfo is one CSP's entry in GET /disk/support.
type DiskCSPSupportInfo struct {
	Supported           bool           `json:"supported" example:"true"`
	RootDiskSelectable  bool           `json:"rootDiskSelectable" example:"true"`
	DataDiskSelectable  bool           `json:"dataDiskSelectable" example:"true"`
	DefaultRootDiskType string         `json:"defaultRootDiskType,omitempty" example:"gp3"`
	DefaultDataDiskType string         `json:"defaultDataDiskType,omitempty" example:"gp3"`
	DiskTypes           []DiskTypeInfo `json:"diskTypes,omitempty"`
	Note                string         `json:"note,omitempty"`
}

// DiskTypeInfo is one disk type option in CSP-native terms (the value to pass as rootDiskType / diskType).
type DiskTypeInfo struct {
	DiskType    string `json:"diskType" example:"gp3"`
	DisplayName string `json:"displayName,omitempty" example:"General Purpose SSD (gp3)"`
	Description string `json:"description,omitempty"`
	// Available is false when the type is out of scope for the requested region/zone/spec (see unavailableReason).
	Available         bool   `json:"available" example:"true"`
	UnavailableReason string `json:"unavailableReason,omitempty" example:"not compatible with spec 'c3-standard-4'"`
	DiskTypeRule
	// CBSpiderName is the identifier CB-Tumblebug sends to CB-Spider when it differs from diskType (informational).
	CBSpiderName string            `json:"cbSpiderName,omitempty" example:"PremiumSSD"`
	Availability *DiskAvailability `json:"availability,omitempty"`
	Note         string            `json:"note,omitempty"`
}

// SpecDiskOptionsResponse wraps GET /ns/{nsId}/resources/spec/{specId}/diskOptions.
type SpecDiskOptionsResponse struct {
	SpecId       string `json:"specId" example:"aws+ap-northeast-2+t3.medium"`
	ProviderName string `json:"providerName" example:"aws"`
	RegionName   string `json:"regionName" example:"ap-northeast-2"`
	CspSpecName  string `json:"cspSpecName" example:"t3.medium"`
	// SpecRootDiskType/Size are the spec's own root disk hints (from cloudspec.csv), if any.
	SpecRootDiskType string `json:"specRootDiskType,omitempty"`
	SpecRootDiskSize int    `json:"specRootDiskSize,omitempty"`
	// Image context (when imageId is given): OS used for byOS rules and the image's minimum root disk size,
	// which raises rootDiskSizeGB.min of every root-capable type.
	ImageId                string `json:"imageId,omitempty" example:"ami-0123456789abcdef0"`
	ImageOSPlatform        string `json:"imageOSPlatform,omitempty" example:"Linux/UNIX"`
	ImageOSType            string `json:"imageOSType,omitempty" example:"ubuntu 22.04"`
	ImageMinRootDiskSizeGB int    `json:"imageMinRootDiskSizeGB,omitempty" example:"8"`
	DiskCSPSupportInfo
	// LiveCheckHint points to the live stock check (POST .../reviewSpecImagePair with rootDiskType).
	LiveCheckHint string `json:"liveCheckHint,omitempty"`
}
