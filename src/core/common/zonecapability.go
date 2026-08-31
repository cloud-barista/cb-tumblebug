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

	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/rs/zerolog/log"
)

// spiderDriverCapability is the subset of CB-Spider's driver capability
// response that zone placement depends on.
type spiderDriverCapability struct {
	ZoneBasedControl bool
}

// zoneCapabilityTTL bounds how long a resolved capability is reused. Driver
// capability never changes at runtime and a region's zone list changes rarely,
// so this only exists to keep repeated retry planning off CB-Spider.
const zoneCapabilityTTL = 30 * time.Minute

type zoneCapabilityEntry struct {
	value     model.ZoneCapability
	expiresAt time.Time
}

var zoneCapabilityCache sync.Map

// ResolveZoneCapability reports whether nodes on this connection can be placed
// in a chosen zone, and which zones are candidates.
//
// Both gates must pass:
//   - the CSP driver declares ZoneBasedControl (false only for KT and KT Classic)
//   - the region has at least two zones (10 of Azure's 48 regions have none)
//
// A CB-Spider lookup failure is not fatal: the capability falls back to the
// region's zone list alone, since a wrong "not shiftable" would silently
// disable a retry that would have worked.
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
	zc.Zones = connConfig.RegionDetail.Zones

	zoneControl, err := getSpiderZoneBasedControl(connectionName)
	if err != nil {
		// Assume supported: all providers but KT declare it, and wrongly
		// reporting "not shiftable" would disable a viable retry.
		log.Debug().Err(err).Msgf("zone capability: CB-Spider capability lookup failed for '%s'; assuming zone-based control", connectionName)
		zoneControl = true
	}
	zc = decideZoneCapability(zoneControl, zc.Zones, connConfig.RegionDetail.RegionName)

	zoneCapabilityCache.Store(connectionName, &zoneCapabilityEntry{
		value:     zc,
		expiresAt: time.Now().Add(zoneCapabilityTTL),
	})
	return zc
}

func getSpiderZoneBasedControl(connectionName string) (bool, error) {
	var callResult spiderDriverCapability
	client := clientManager.NewHttpClient()
	client.SetTimeout(clientManager.AvailabilityCheckTimeout)
	url := model.SpiderRestUrl + "/driver/capability?ConnectionName=" + connectionName
	requestBody := clientManager.NoBody

	_, err := clientManager.ExecuteHttpRequest(
		client,
		"GET",
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&callResult,
		clientManager.MediumDuration,
	)
	if err != nil {
		return false, err
	}
	return callResult.ZoneBasedControl, nil
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
