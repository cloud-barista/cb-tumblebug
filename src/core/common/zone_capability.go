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

package common

// Zone placement in CB-Tumblebug is expressed only through the subnet: neither
// SpiderVMReqInfo nor the driver-level VMReqInfo carries a zone, and CB-Spider
// derives the target zone from the subnet's ZoneId when starting a VM. Moving a
// node to another zone therefore requires two independent things to hold, and
// this file answers whether they do.

import (
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
)

// zoneCapabilityTTL bounds how long a resolved capability is reused.
const zoneCapabilityTTL = 30 * time.Minute

type zoneCapabilityEntry struct {
	value     model.ZoneCapability
	expiresAt time.Time
}

var zoneCapabilityCache sync.Map

// ResolveZoneCapability reports whether nodes on this connection can be placed
// in a chosen zone, and which zones are candidates, using CB-Tumblebug's
// authoritative cloudinfo.yaml and CSP platform metadata.
//
// Both gates must pass:
//   - the CSP platform supports zone-based subnet/VM control (false only for KT and KT Classic)
//   - the region has at least two zones in cloudinfo.yaml (e.g. some Azure regions have none)
func ResolveZoneCapability(connectionName string) model.ZoneCapability {
	connectionName = strings.TrimSpace(connectionName)
	if connectionName == "" {
		return model.ZoneCapability{Reason: "connection name is empty"}
	}

	if v, ok := zoneCapabilityCache.Load(connectionName); ok {
		if e, ok := v.(*zoneCapabilityEntry); ok && time.Now().Before(e.expiresAt) {
			return e.value
		}
	}

	zc := model.ZoneCapability{}

	connConfig, err := GetConnConfig(connectionName)
	if err != nil {
		zc.Reason = "connection config not found: " + err.Error()
		return zc
	}

	regionName := connConfig.RegionDetail.RegionName
	if regionName == "" {
		regionName = connConfig.RegionZoneInfo.AssignedRegion
	}
	zones := connConfig.RegionDetail.Zones
	if len(zones) == 0 && connConfig.ProviderName != "" && regionName != "" {
		if regionDetail, rErr := GetRegion(connConfig.ProviderName, regionName); rErr == nil && len(regionDetail.Zones) > 0 {
			zones = regionDetail.Zones
		}
	}

	zoneControl := SupportsZoneBasedControl(connConfig.ProviderName)
	zc = decideZoneCapability(zoneControl, zones, regionName)

	zoneCapabilityCache.Store(connectionName, &zoneCapabilityEntry{
		value:     zc,
		expiresAt: time.Now().Add(zoneCapabilityTTL),
	})
	return zc
}

// SupportsZoneBasedControl reports whether the given CSP supports zone-based
// subnet/VM placement. Only KT and KT Classic lack zone-based control.
func SupportsZoneBasedControl(providerName string) bool {
	platform := csp.ResolveCloudPlatform(providerName)
	return platform != csp.KT && platform != "ktclassic"
}

// decideZoneCapability applies the two gates. Split out from the lookup so the
// decision is testable without CB-Spider or a KV store.
func decideZoneCapability(zoneControl bool, zones []string, regionName string) model.ZoneCapability {
	zc := model.ZoneCapability{ZoneControl: zoneControl, Zones: zones}
	switch {
	case !zoneControl:
		zc.Reason = "the CSP driver does not support zone-based control"
	case len(zones) == 0:
		zc.Reason = "region '" + regionName + "' has no availability zones"
	case len(zones) == 1:
		zc.Reason = "region '" + regionName + "' has only one availability zone"
	default:
		zc.Shiftable = true
	}
	return zc
}
