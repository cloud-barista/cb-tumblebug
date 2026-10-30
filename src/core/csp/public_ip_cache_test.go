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
	"fmt"
	"sync"
	"testing"
)

func TestPublicIPCacheStoreGetAndDelete(t *testing.T) {
	StoreBatchPublicIPs("AWS", "eu-central-1", map[string]string{
		"i-111": "18.199.1.1",
		"i-222": "18.199.1.2",
	})

	if got := GetCachedPublicIP("aws", "eu-central-1", "i-111"); got != "18.199.1.1" {
		t.Fatalf("expected 18.199.1.1, got %q", got)
	}
	if got := GetCachedPublicIP("AWS", "eu-central-1", "i-222"); got != "18.199.1.2" {
		t.Fatalf("expected 18.199.1.2, got %q", got)
	}

	// Empty IP on stop/terminate must evict the cached entry.
	StoreBatchPublicIPs("aws", "eu-central-1", map[string]string{
		"i-111": "",
		"i-222": "3.71.4.80",
	})

	if got := GetCachedPublicIP("aws", "eu-central-1", "i-111"); got != "" {
		t.Fatalf("expected i-111 to be evicted (empty), got %q", got)
	}
	if got := GetCachedPublicIP("aws", "eu-central-1", "i-222"); got != "3.71.4.80" {
		t.Fatalf("expected updated IP 3.71.4.80, got %q", got)
	}

	// Clean up test keys.
	StoreBatchPublicIPs("aws", "eu-central-1", map[string]string{"i-222": ""})
}

func TestPublicIPCacheConcurrentAccess(t *testing.T) {
	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			inst := fmt.Sprintf("i-conc-%d", id)
			for i := 0; i < 50; i++ {
				StoreBatchPublicIPs("aws", "us-east-1", map[string]string{inst: "54.0.0.1"})
				_ = GetCachedPublicIP("aws", "us-east-1", inst)
				StoreBatchPublicIPs("aws", "us-east-1", map[string]string{inst: ""})
			}
		}(g)
	}
	wg.Wait()
}

