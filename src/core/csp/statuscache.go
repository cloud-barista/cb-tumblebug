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
	"context"
	"strings"
	"sync"
	"time"
)

// Per-node status polls used to issue one batch call per node per poll (for GCP a whole-project
// AggregatedList), which burned API quotas under polling amplification. This cache coalesces
// polls per (provider, region): one fetch refreshes every id asked recently, concurrent callers
// wait for the in-flight fetch, and results are reused for BatchStatusCacheTTL.

// BatchStatusCacheTTL is how long a batch status result is reused.
const BatchStatusCacheTTL = 10 * time.Second

// batchStatusIdleTTL drops ids nobody asked about for this long (terminated/removed nodes).
const batchStatusIdleTTL = 2 * time.Minute

type batchStatusEntry struct {
	mu        sync.Mutex
	gen       uint64 // incremented per fetch
	fetched   time.Time
	requested map[string]bool   // ids covered by the last fetch
	statuses  map[string]string // id -> TB status (absent = not found at the CSP)
	lastAsked map[string]time.Time
}

// batchStatusCoalesceWindow lets concurrent first-time pollers register their ids before one fetch
// serves them all (a status sweep over N new nodes must not cost N batch calls).
const batchStatusCoalesceWindow = 200 * time.Millisecond

var (
	batchStatusMu      sync.Mutex
	batchStatusEntries = map[string]*batchStatusEntry{}
)

func batchStatusKey(provider, region string) string {
	return strings.ToLower(provider) + "|" + strings.ToLower(region)
}

func getBatchStatusEntry(provider, region string) *batchStatusEntry {
	batchStatusMu.Lock()
	defer batchStatusMu.Unlock()
	k := batchStatusKey(provider, region)
	e, ok := batchStatusEntries[k]
	if !ok {
		e = &batchStatusEntry{requested: map[string]bool{}, statuses: map[string]string{}, lastAsked: map[string]time.Time{}}
		batchStatusEntries[k] = e
	}
	return e
}

// lookupLocked returns the cached status when the last fetch is fresh and covered instanceId.
func (e *batchStatusEntry) lookupLocked(instanceId string, now time.Time) (string, bool, bool) {
	if now.Sub(e.fetched) < BatchStatusCacheTTL && e.requested[instanceId] {
		s, ok := e.statuses[instanceId]
		return s, ok, true
	}
	return "", false, false
}

// BatchVMStatusCached returns the status of one instance through the shared per-region cache.
// found=false with err=nil means the CSP's batch response did not include the id (instance gone).
func BatchVMStatusCached(ctx context.Context, provider, region string, fn BatchVMStatusFunc, instanceId string) (status string, found bool, err error) {
	e := getBatchStatusEntry(provider, region)
	e.mu.Lock()
	now := time.Now()
	e.lastAsked[instanceId] = now
	if s, ok, hit := e.lookupLocked(instanceId, now); hit {
		e.mu.Unlock()
		return s, ok, nil
	}
	gen := e.gen
	e.mu.Unlock()

	time.Sleep(batchStatusCoalesceWindow) // let concurrent pollers register their ids

	e.mu.Lock() // serializes fetches per region; waiters then read the fresh result
	defer e.mu.Unlock()
	now = time.Now()
	if e.gen != gen {
		if s, ok, hit := e.lookupLocked(instanceId, now); hit {
			return s, ok, nil
		}
	}
	ids := make([]string, 0, len(e.lastAsked))
	for id, t := range e.lastAsked {
		if now.Sub(t) > batchStatusIdleTTL {
			delete(e.lastAsked, id)
			continue
		}
		ids = append(ids, id)
	}
	statuses, ferr := fn(ctx, region, ids)
	if ferr != nil {
		return "", false, ferr
	}
	e.gen++
	e.fetched = time.Now()
	e.statuses = statuses
	e.requested = make(map[string]bool, len(ids))
	for _, id := range ids {
		e.requested[id] = true
	}
	s, ok := statuses[instanceId]
	return s, ok, nil
}

// StoreBatchStatuses seeds the cache from a batch fetch made elsewhere (e.g. the BatchSweeper).
func StoreBatchStatuses(provider, region string, requested []string, statuses map[string]string) {
	e := getBatchStatusEntry(provider, region)
	e.mu.Lock()
	defer e.mu.Unlock()
	now := time.Now()
	e.gen++
	e.fetched = now
	e.statuses = statuses
	e.requested = make(map[string]bool, len(requested))
	for _, id := range requested {
		e.requested[id] = true
		e.lastAsked[id] = now
	}
}
