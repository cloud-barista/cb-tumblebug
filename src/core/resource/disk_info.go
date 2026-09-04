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
	"path"
	"slices"
	"sort"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
)

const (
	DiskViewAvailable = "available"
	DiskViewAll       = "all"
)

// DiskScopeFilter narrows the static reference to a region/zone/spec; empty fields mean no filtering.
type DiskScopeFilter struct {
	Region      string
	Zone        string
	CspSpecName string
	// OSType (e.g. "windows", "ubuntu") resolves byOS root size overrides into rootDiskSizeGB.
	OSType string
	// MinRootDiskSizeGB (e.g. the image's OS disk size) raises the root size minimum of every type.
	MinRootDiskSizeGB int
}

// ToCBSpiderDiskType translates a CSP-native disk type to the identifier CB-Spider expects, when
// assets/diskinfo.yaml declares one (cbSpiderName). Anything else ("", "default", already a
// CB-Spider name, or an unknown value) is returned unchanged so CB-Spider stays the final judge.
func ToCBSpiderDiskType(providerName, diskType string) string {
	provider, ok := common.RuntimeDiskInfo.Disk[csp.ResolveCloudPlatform(providerName)]
	if !ok || diskType == "" {
		return diskType
	}
	if t, ok := provider.DiskTypes[diskType]; ok && t.CBSpiderName != "" {
		return t.CBSpiderName
	}
	for k, t := range provider.DiskTypes {
		if t.CBSpiderName == "" && strings.EqualFold(k, diskType) {
			return k
		}
		if t.CBSpiderName != "" && strings.EqualFold(k, diskType) {
			return t.CBSpiderName
		}
	}
	return diskType
}

// FromCBSpiderDiskType is the reverse of ToCBSpiderDiskType (for presenting CB-Spider responses in CSP terms).
func FromCBSpiderDiskType(providerName, spiderDiskType string) string {
	provider, ok := common.RuntimeDiskInfo.Disk[csp.ResolveCloudPlatform(providerName)]
	if !ok || spiderDiskType == "" {
		return spiderDiskType
	}
	for k, t := range provider.DiskTypes {
		if t.CBSpiderName != "" && strings.EqualFold(t.CBSpiderName, spiderDiskType) {
			return k
		}
	}
	return spiderDiskType
}

// GetDiskSupport returns the static, CSP-wide root/data disk reference from assets/diskinfo.yaml (no CB-Spider call).
// view "available" (default) lists only types in scope for the filter; "all" includes out-of-scope ones with the reason.
func GetDiskSupport(providerName, view string, scope DiskScopeFilter) (model.DiskSupportResponse, error) {
	view, err := normalizeDiskView(view)
	if err != nil {
		return model.DiskSupportResponse{}, err
	}
	response := model.DiskSupportResponse{
		ResourceType: model.StrDataDisk,
		View:         view,
		RegionName:   scope.Region,
		Supports:     map[string]model.DiskCSPSupportInfo{},
	}

	providerName = csp.ResolveCloudPlatform(strings.TrimSpace(providerName))
	if providerName != "" {
		if _, inYaml := common.RuntimeDiskInfo.Disk[providerName]; !inYaml && !slices.Contains(csp.AllCSPs, providerName) {
			return response, fmt.Errorf("unknown provider '%s'", providerName)
		}
		response.Supports[providerName] = BuildDiskSupportInfo(providerName, view, scope)
		return response, nil
	}
	for _, cspKey := range csp.AllCSPs {
		response.Supports[cspKey] = BuildDiskSupportInfo(cspKey, view, scope)
	}
	for cspKey := range common.RuntimeDiskInfo.Disk {
		if _, done := response.Supports[cspKey]; !done {
			response.Supports[cspKey] = BuildDiskSupportInfo(cspKey, view, scope)
		}
	}
	return response, nil
}

// GetSpecDiskOptions resolves a spec's provider/region/cspSpecName and returns the disk options usable with it.
// imageId (optional) supplies the OS for byOS rules and the image's minimum root disk size; osType is a
// fallback/override for the OS when no image is given.
func GetSpecDiskOptions(nsId, specId, imageId, osType, view string) (model.SpecDiskOptionsResponse, error) {
	view, err := normalizeDiskView(view)
	if err != nil {
		return model.SpecDiskOptionsResponse{}, err
	}
	spec, err := GetSpec(nsId, specId)
	if err != nil {
		return model.SpecDiskOptionsResponse{}, err
	}
	provider := csp.ResolveCloudPlatform(spec.ProviderName)
	scope := DiskScopeFilter{Region: spec.RegionName, CspSpecName: spec.CspSpecName, OSType: osType}
	resp := model.SpecDiskOptionsResponse{
		SpecId:           spec.Id,
		ProviderName:     provider,
		RegionName:       spec.RegionName,
		CspSpecName:      spec.CspSpecName,
		SpecRootDiskType: spec.RootDiskType,
		SpecRootDiskSize: spec.RootDiskSize,
		LiveCheckHint:    "Static reference. For live stock per zone, use POST /ns/{nsId}/mci/reviewSpecImagePair with rootDiskType.",
	}
	if imageId != "" {
		img, err := GetImage(nsId, imageId)
		if err != nil {
			return model.SpecDiskOptionsResponse{}, fmt.Errorf("image '%s' not found in namespace '%s': %w", imageId, nsId, err)
		}
		resp.ImageId = img.CspImageName
		resp.ImageOSPlatform = string(img.OSPlatform)
		resp.ImageOSType = img.OSType
		if img.OSDiskSizeGB > 0 { // unknown sizes are stored as -1
			resp.ImageMinRootDiskSizeGB = int(img.OSDiskSizeGB)
			scope.MinRootDiskSizeGB = resp.ImageMinRootDiskSizeGB
		}
		if scope.OSType == "" {
			scope.OSType = strings.TrimSpace(string(img.OSPlatform) + " " + img.OSType)
		}
	}
	resp.DiskCSPSupportInfo = BuildDiskSupportInfo(provider, view, scope)
	return resp, nil
}

func normalizeDiskView(view string) (string, error) {
	view = strings.ToLower(strings.TrimSpace(view))
	switch view {
	case "":
		return DiskViewAvailable, nil
	case DiskViewAvailable, DiskViewAll:
		return view, nil
	}
	return "", fmt.Errorf("invalid view '%s' (available|all)", view)
}

// BuildDiskSupportInfo builds one CSP's entry; Supported: false when assets/diskinfo.yaml has no entry.
func BuildDiskSupportInfo(cspKey, view string, scope DiskScopeFilter) model.DiskCSPSupportInfo {
	provider, exists := common.RuntimeDiskInfo.Disk[cspKey]
	if !exists {
		return model.DiskCSPSupportInfo{Supported: false, Note: "No disk type reference is available for this CSP."}
	}
	info := model.DiskCSPSupportInfo{
		Supported:           true,
		RootDiskSelectable:  provider.RootDiskSelectable,
		DataDiskSelectable:  provider.DataDiskSelectable,
		DefaultRootDiskType: provider.DefaultRootDiskType,
		DefaultDataDiskType: provider.DefaultDataDiskType,
		Note:                provider.Note,
	}
	for _, k := range orderedDiskTypeKeys(provider) {
		t := provider.DiskTypes[k]
		item := model.DiskTypeInfo{
			DiskType:     k,
			DisplayName:  t.Name,
			Description:  t.Description,
			Available:    true,
			DiskTypeRule: t.DiskTypeRule,
			CBSpiderName: t.CBSpiderName,
			Availability: t.Availability,
			Note:         t.Note,
		}
		if !item.RootDisk {
			item.RootDiskSizeGB, item.ByOS = nil, nil
		} else {
			if scope.OSType != "" {
				item.RootDiskSizeGB = resolveRootSizeByOS(item.DiskTypeRule, scope.OSType)
				item.ByOS = nil
			}
			if scope.MinRootDiskSizeGB > 0 {
				item.RootDiskSizeGB = raiseMinSize(item.RootDiskSizeGB, scope.MinRootDiskSizeGB)
			}
		}
		if !item.DataDisk {
			item.DataDiskSizeGB = nil
		}
		if reason := availabilityMismatch(t.Availability, scope); reason != "" {
			item.Available, item.UnavailableReason = false, reason
			if view == DiskViewAvailable {
				continue
			}
		}
		info.DiskTypes = append(info.DiskTypes, item)
	}
	return info
}

// resolveRootSizeByOS picks the byOS override whose family name appears in osType (case-insensitive), else the base rule.
func resolveRootSizeByOS(rule model.DiskTypeRule, osType string) *model.DiskSizeConstraint {
	os := strings.ToLower(osType)
	for family, override := range rule.ByOS {
		if strings.Contains(os, strings.ToLower(family)) {
			c := override
			return &c
		}
	}
	return rule.RootDiskSizeGB
}

// raiseMinSize returns a copy of c whose minimum is at least floor (allowed lists drop smaller values).
func raiseMinSize(c *model.DiskSizeConstraint, floor int) *model.DiskSizeConstraint {
	out := model.DiskSizeConstraint{}
	if c != nil {
		out = *c
	}
	if len(out.Allowed) > 0 {
		kept := []int{}
		for _, v := range out.Allowed {
			if v >= floor {
				kept = append(kept, v)
			}
		}
		out.Allowed = kept
		return &out
	}
	if out.Min < floor {
		out.Min = floor
	}
	return &out
}

func orderedDiskTypeKeys(p model.DiskProviderConfig) []string {
	keys := []string{}
	for _, k := range p.DiskTypeOrder {
		if _, ok := p.DiskTypes[k]; ok && !slices.Contains(keys, k) {
			keys = append(keys, k)
		}
	}
	rest := []string{}
	for k := range p.DiskTypes {
		if !slices.Contains(keys, k) {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	return append(keys, rest...)
}

// availabilityMismatch returns why the type is out of scope for the filter, or "" if in scope / unscoped.
func availabilityMismatch(a *model.DiskAvailability, scope DiskScopeFilter) string {
	if a == nil {
		return ""
	}
	if scope.Region != "" && !inScope(a.Regions, scope.Region) {
		return fmt.Sprintf("not available in region '%s'", scope.Region)
	}
	if scope.Zone != "" && !inScope(a.Zones, scope.Zone) {
		return fmt.Sprintf("not available in zone '%s'", scope.Zone)
	}
	if scope.CspSpecName != "" && !inScope(a.SpecPatterns, scope.CspSpecName) {
		return fmt.Sprintf("not compatible with spec '%s'", scope.CspSpecName)
	}
	return ""
}

func inScope(s *model.DiskScope, value string) bool {
	if s == nil {
		return true
	}
	v := strings.ToLower(value)
	if len(s.Only) > 0 {
		return globAny(s.Only, v)
	}
	return !globAny(s.Except, v)
}

func globAny(patterns []string, v string) bool {
	for _, p := range patterns {
		if ok, _ := path.Match(strings.ToLower(p), v); ok {
			return true
		}
	}
	return false
}
