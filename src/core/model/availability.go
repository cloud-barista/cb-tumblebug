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

package model

import (
	"strings"
	"time"
)

// IsDefaultDiskType reports whether the given root-disk-type value should
// be treated as "let the CSP/Spider pick its default" (i.e. no explicit
// disk category). cb-tumblebug accepts both "" and "default" (case-insensitive)
// as the special "use default" sentinel; everything else is a CSP-native
// disk category that should be honored as-is.
func IsDefaultDiskType(rootDiskType string) bool {
	v := strings.TrimSpace(rootDiskType)
	if v == "" {
		return true
	}
	return strings.EqualFold(v, "default")
}

// NormalizeDiskTypeForQuery returns a value suitable for AvailabilityQuery.
// SystemDiskCategory: "" when the input is the "default" sentinel (so the
// checker queries availability across all categories), otherwise the
// trimmed value as-is.
func NormalizeDiskTypeForQuery(rootDiskType string) string {
	if IsDefaultDiskType(rootDiskType) {
		return ""
	}
	return strings.TrimSpace(rootDiskType)
}

// AvailabilityQuery is the provider-agnostic input for a pre-flight
// availability check against a CSP. Implementations of the underlying
// checker may use a subset of these fields depending on what their CSP
// API supports.
type AvailabilityQuery struct {
	Provider           string // CSP identifier (e.g., "alibaba", "azure")
	Region             string // CSP-native region (e.g., "cn-qingdao")
	InstanceType       string // CSP-native instance type (e.g., "ecs.gn6i-c4g1.xlarge")
	SystemDiskCategory string // CSP-native disk category (optional, e.g., "cloud_essd")
	PreferredZone      string // optional zone hint
	ImageId            string // optional image id (some CSPs validate compatibility)
}

// ZoneAvailability describes per-zone availability for the queried
// instance type, including which system-disk categories are currently
// available in that zone.
type ZoneAvailability struct {
	ZoneId         string   `json:"zoneId"`
	Available      bool     `json:"available"`
	SupportedDisks []string `json:"supportedDisks,omitempty"` // disk categories currently available in this zone
	Status         string   `json:"status,omitempty"`         // CSP-native status (e.g., "WithStock", "ClosedWithStock")
	Reason         string   `json:"reason,omitempty"`         // why not available, when applicable
}

// AvailabilityResult is the provider-agnostic output of a pre-flight
// availability check. The Available field is the OR of all zones; callers
// can inspect Zones to choose a specific zone or disk category.
type AvailabilityResult struct {
	Provider     string             `json:"provider"`
	Region       string             `json:"region"`
	InstanceType string             `json:"instanceType"`
	Available    bool               `json:"available"`
	Zones        []ZoneAvailability `json:"zones,omitempty"`
	Reason       string             `json:"reason,omitempty"`    // explanation when Available is false (or when no checker)
	Source       string             `json:"source,omitempty"`    // checker identifier for tracing (e.g., "alibaba:DescribeAvailableResource")
	Cached       bool               `json:"cached,omitempty"`    // true if served from cache
	QueriedAt    time.Time          `json:"queriedAt,omitempty"` // time of original (uncached) query
}
