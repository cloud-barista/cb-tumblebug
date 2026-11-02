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

package csp

import (
	"strings"
	"sync"
)

type publicIPCacheKey struct {
	provider   string
	region     string
	instanceId string
}

// publicIPCache is completely independent of regionStatusEntry / status_cache.go
// so batch status handlers can record observed public IPs without any lock ordering
// or re-entrancy interaction with BatchVMStatusCached.
var publicIPCache sync.Map // map[publicIPCacheKey]string

// StoreBatchPublicIPs records the observed public IP for each instance in a batch response.
// An empty IP string (e.g. when an instance is stopped/suspended/terminated or has no public IP)
// removes the entry from the map so stale IPs are never served.
func StoreBatchPublicIPs(provider, region string, ips map[string]string) {
	if len(ips) == 0 {
		return
	}
	normProvider := strings.ToLower(provider)
	for id, ip := range ips {
		if id == "" {
			continue
		}
		k := publicIPCacheKey{
			provider:   normProvider,
			region:     region,
			instanceId: id,
		}
		if ip == "" {
			publicIPCache.Delete(k)
		} else {
			publicIPCache.Store(k, ip)
		}
	}
}

// GetCachedPublicIP returns the last non-empty public IP observed for (provider, region, instanceId),
// or "" if none is present.
func GetCachedPublicIP(provider, region, instanceId string) string {
	if instanceId == "" {
		return ""
	}
	k := publicIPCacheKey{
		provider:   strings.ToLower(provider),
		region:     region,
		instanceId: instanceId,
	}
	if v, ok := publicIPCache.Load(k); ok {
		if ip, ok := v.(string); ok {
			return ip
		}
	}
	return ""
}

