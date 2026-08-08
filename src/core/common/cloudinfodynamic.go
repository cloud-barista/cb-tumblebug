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
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	modelcsp "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"

	"github.com/rs/zerolog/log"
)

// Dynamic CSP registration.
//
// cloudinfo.yaml is read once at startup, so a CSP that comes into existence while the
// server is running — most obviously an OpenStack that CB-Tumblebug itself just deployed
// onto a VM it created — could not be used without editing the file and restarting. On
// Kubernetes that is worse still: the file the process reads comes from the container
// image, so editing the local copy does nothing at all.
//
// The definitions registered here are kept in the same RuntimeCloudInfo the file
// populates, and persisted to the kvstore so they survive a restart.

// cloudInfoMu guards RuntimeCloudInfo. Registration replaces the CSPs map wholesale
// (copy-on-write) rather than mutating it in place: GetCloudInfo hands callers a shallow
// copy that still aliases the live map, and several of them iterate it. Swapping the map
// leaves those callers reading a consistent snapshot instead of racing a concurrent write.
var cloudInfoMu sync.RWMutex

// DynamicCspKeyPrefix is where runtime-registered CSP definitions live in the kvstore.
const DynamicCspKeyPrefix = "/cloudinfo/dynamicCsp"

// dynamicCspKey returns the kvstore key holding one runtime-registered CSP definition.
func dynamicCspKey(providerName string) string {
	return DynamicCspKeyPrefix + "/" + strings.ToLower(providerName)
}

// GetCloudInfo is func to get all cloud info
func GetCloudInfo() (model.CloudInfo, error) {
	cloudInfoMu.RLock()
	defer cloudInfoMu.RUnlock()
	return RuntimeCloudInfo, nil
}

// getCspDetail returns one CSP definition and whether it exists.
func getCspDetail(providerName string) (model.CSPDetail, bool) {
	cloudInfoMu.RLock()
	defer cloudInfoMu.RUnlock()
	detail, ok := RuntimeCloudInfo.CSPs[strings.ToLower(providerName)]
	return detail, ok
}

// listCspNames returns the registered provider names.
func listCspNames() []string {
	cloudInfoMu.RLock()
	defer cloudInfoMu.RUnlock()
	names := make([]string, 0, len(RuntimeCloudInfo.CSPs))
	for name := range RuntimeCloudInfo.CSPs {
		names = append(names, name)
	}
	return names
}

// snapshotCspDetails returns a copy of the provider→detail map for safe iteration.
func snapshotCspDetails() map[string]model.CSPDetail {
	cloudInfoMu.RLock()
	defer cloudInfoMu.RUnlock()
	out := make(map[string]model.CSPDetail, len(RuntimeCloudInfo.CSPs))
	for name, detail := range RuntimeCloudInfo.CSPs {
		out[name] = detail
	}
	return out
}

// putCspDetail installs (or replaces) one CSP definition under the write lock.
func putCspDetail(providerName string, detail model.CSPDetail) {
	cloudInfoMu.Lock()
	defer cloudInfoMu.Unlock()

	newCSPs := make(map[string]model.CSPDetail, len(RuntimeCloudInfo.CSPs)+1)
	for name, existing := range RuntimeCloudInfo.CSPs {
		newCSPs[name] = existing
	}
	newCSPs[strings.ToLower(providerName)] = detail
	RuntimeCloudInfo.CSPs = newCSPs
}

// dropCspDetail removes one CSP definition under the write lock.
func dropCspDetail(providerName string) bool {
	cloudInfoMu.Lock()
	defer cloudInfoMu.Unlock()

	key := strings.ToLower(providerName)
	if _, ok := RuntimeCloudInfo.CSPs[key]; !ok {
		return false
	}
	newCSPs := make(map[string]model.CSPDetail, len(RuntimeCloudInfo.CSPs))
	for name, existing := range RuntimeCloudInfo.CSPs {
		if name != key {
			newCSPs[name] = existing
		}
	}
	RuntimeCloudInfo.CSPs = newCSPs
	return true
}

// normalizeCspDetail lowercases region keys and fills the derived fields the startup
// loader fills, so a definition registered at runtime is shaped exactly like one read
// from cloudinfo.yaml.
func normalizeCspDetail(detail model.CSPDetail) model.CSPDetail {
	newRegions := make(map[string]model.RegionDetail, len(detail.Regions))
	for regionKey, regionDetail := range detail.Regions {
		lowerKey := strings.ToLower(regionKey)
		regionDetail.RegionName = lowerKey
		if regionDetail.RegionId == "" {
			regionDetail.RegionId = regionKey
		}
		newRegions[lowerKey] = regionDetail
	}
	detail.Regions = newRegions
	return detail
}

// validateCspDetail rejects definitions that would register but never work.
func validateCspDetail(providerName string, detail model.CSPDetail) error {
	if err := CheckString(providerName); err != nil {
		return fmt.Errorf("invalid provider name %q: %w", providerName, err)
	}
	if detail.Driver == "" {
		return fmt.Errorf("driver is required (ex: openstack-driver-v1.0.so)")
	}
	if len(detail.Regions) == 0 {
		return fmt.Errorf("at least one region is required")
	}
	for regionKey, region := range detail.Regions {
		if len(region.Zones) == 0 {
			return fmt.Errorf("region %q has no zone; at least one is required", regionKey)
		}
	}
	// CloudPlatform is what selects the CB-Spider driver. A derived CSP (an OpenStack
	// instance named openstack-site01) must declare the platform it is an instance of,
	// otherwise Spider is asked for a driver named after the instance and finds none.
	if detail.CloudPlatform != "" {
		if _, known := getCspDetail(detail.CloudPlatform); !known {
			return fmt.Errorf("cloudPlatform %q is not a known CSP; it must name a base platform such as openstack", detail.CloudPlatform)
		}
	}
	return nil
}

// RegisterCspDefinition adds (or replaces) a CSP definition at runtime and pushes it to
// CB-Spider, so the provider becomes usable without editing cloudinfo.yaml or restarting.
//
// It performs the same three steps the startup path performs for every CSP in the file:
// record the platform mapping, install the definition, and register the driver plus its
// regions and zones with CB-Spider. Credentials are registered separately, afterwards,
// via RegisterCredential — this call only makes the provider known.
func RegisterCspDefinition(providerName string, detail model.CSPDetail, persist bool) error {
	providerName = strings.ToLower(providerName)
	detail = normalizeCspDetail(detail)

	if err := validateCspDetail(providerName, detail); err != nil {
		return err
	}

	// Record the platform mapping before installing the definition: RegisterCloudInfo
	// resolves the driver through it, and a missing entry would send the CSP instance
	// name to Spider as if it were a platform.
	if detail.CloudPlatform != "" {
		modelcsp.RegisterCloudPlatform(providerName, detail.CloudPlatform)
	} else {
		modelcsp.RegisterCloudPlatform(providerName, providerName)
	}

	putCspDetail(providerName, detail)

	if err := RegisterCloudInfo(providerName); err != nil {
		// Leave the definition in place: the usual cause is CB-Spider being briefly
		// unavailable, and a retry of this call then succeeds without the caller having
		// to re-send the whole definition.
		log.Error().Err(err).Str("provider", providerName).
			Msg("CSP definition installed but registering it with CB-Spider failed")
		return fmt.Errorf("failed to register provider %q with CB-Spider: %w", providerName, err)
	}

	if persist {
		if err := persistCspDefinition(providerName, detail); err != nil {
			// The provider works for this process; only surviving a restart is lost.
			log.Warn().Err(err).Str("provider", providerName).
				Msg("CSP registered but could not be persisted; it will be gone after a restart")
		}
	}

	log.Info().Str("provider", providerName).Str("driver", detail.Driver).
		Int("regions", len(detail.Regions)).Msg("Registered CSP definition at runtime")
	return nil
}

// UnregisterCspDefinition removes a runtime-registered CSP definition.
//
// Only definitions registered at runtime can be removed: the ones loaded from
// cloudinfo.yaml would come back on the next restart, so removing them would produce a
// server whose behaviour silently depends on how long ago it started.
func UnregisterCspDefinition(providerName string) error {
	providerName = strings.ToLower(providerName)

	_, found, err := kvstore.GetKv(dynamicCspKey(providerName))
	if err != nil {
		return fmt.Errorf("failed to look up the persisted definition for %q: %w", providerName, err)
	}
	if !found {
		return fmt.Errorf("provider %q was not registered at runtime; edit assets/cloudinfo.yaml to remove a built-in provider", providerName)
	}

	if err := kvstore.Delete(dynamicCspKey(providerName)); err != nil {
		return fmt.Errorf("failed to remove the persisted definition for %q: %w", providerName, err)
	}
	if !dropCspDetail(providerName) {
		log.Warn().Str("provider", providerName).Msg("persisted definition removed but the provider was not in memory")
	}

	log.Info().Str("provider", providerName).Msg("Unregistered runtime CSP definition")
	return nil
}

// persistCspDefinition stores one definition so it survives a restart.
func persistCspDefinition(providerName string, detail model.CSPDetail) error {
	value, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	return kvstore.Put(dynamicCspKey(providerName), string(value))
}

// LoadDynamicCspDefinitions restores runtime-registered CSPs from the kvstore. It runs at
// startup, after cloudinfo.yaml has been loaded, so a definition registered at runtime
// wins over a same-named entry in the file.
//
// Registration with CB-Spider is left to RegisterAllCloudInfo, which runs afterwards and
// walks everything in RuntimeCloudInfo — including what this restores.
func LoadDynamicCspDefinitions() error {
	entries, err := kvstore.GetKvList(DynamicCspKeyPrefix)
	if err != nil {
		return fmt.Errorf("failed to read persisted CSP definitions: %w", err)
	}

	restored := 0
	for _, entry := range entries {
		providerName := strings.TrimPrefix(entry.Key, DynamicCspKeyPrefix+"/")
		if providerName == "" {
			continue
		}
		var detail model.CSPDetail
		if err := json.Unmarshal([]byte(entry.Value), &detail); err != nil {
			log.Warn().Err(err).Str("key", entry.Key).Msg("skipping unreadable CSP definition")
			continue
		}
		putCspDetail(providerName, normalizeCspDetail(detail))
		restored++
		log.Info().Str("provider", providerName).Msg("Restored runtime-registered CSP definition")
	}

	if restored > 0 {
		log.Info().Int("count", restored).Msg("Restored runtime-registered CSP definitions")
	}
	return nil
}

// IsDynamicCspDefinition reports whether a provider was registered at runtime rather than
// read from cloudinfo.yaml.
func IsDynamicCspDefinition(providerName string) bool {
	_, found, err := kvstore.GetKv(dynamicCspKey(strings.ToLower(providerName)))
	return err == nil && found
}
