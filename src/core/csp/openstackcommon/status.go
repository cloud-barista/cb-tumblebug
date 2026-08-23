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

// Package openstackcommon holds helpers shared by OpenStack-based direct clients (NHN, KT).
package openstackcommon

import (
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
)

// StateToTBStatus maps a Nova server status to a TB status string.
func StateToTBStatus(state string) string {
	switch strings.ToUpper(state) {
	case "ACTIVE":
		return model.StatusRunning
	case "SHUTOFF", "PAUSED", "SUSPENDED":
		return model.StatusSuspended
	case "BUILD", "BUILDING":
		return model.StatusCreating
	case "DELETED", "SOFT_DELETED":
		return model.StatusTerminated
	case "ERROR":
		return model.StatusFailed
	case "REBOOT", "HARD_REBOOT":
		return model.StatusRebooting
	default:
		return model.StatusUndefined
	}
}

// PublicIPOf picks the first floating/public address from a Nova addresses map.
func PublicIPOf(addresses map[string]interface{}) string {
	for _, v := range addresses {
		list, ok := v.([]interface{})
		if !ok {
			continue
		}
		for _, a := range list {
			m, ok := a.(map[string]interface{})
			if !ok {
				continue
			}
			if t, _ := m["OS-EXT-IPS:type"].(string); t == "floating" {
				if addr, _ := m["addr"].(string); addr != "" {
					return addr
				}
			}
		}
	}
	return ""
}
