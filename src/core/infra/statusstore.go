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

package infra

import (
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
)

// PollPriority determines how frequently NodeStatusAgent polls a node.
type PollPriority int

const (
	PollSkip    PollPriority = iota // no polling (Terminated, Failed, stable Suspended, operation-locked)
	PollRecover                     // Undefined → 10 min
	PollNormal                      // Running → 5 min
	PollHigh                        // Transitional (Suspending/Resuming/Rebooting/Terminating) → 15 s
	PollUrgent                      // Operation lock TTL expired → 5 s
)

// pollIntervals maps each PollPriority to its polling interval.
var pollIntervals = map[PollPriority]time.Duration{
	PollUrgent:  5 * time.Second,
	PollHigh:    15 * time.Second,
	PollNormal:  5 * time.Minute,
	PollRecover: 10 * time.Minute,
}

// operationLockTTL is the maximum validity of an operation lock.
// Spider POST /vm times out after 20 min; 25 min gives a safe margin.
const operationLockTTL = 25 * time.Minute

// storeMaxStaleness returns the maximum acceptable age of a cached StatusEntry
// for the given node status and TargetAction.
// Returns 0 when the entry must always be re-fetched live (active operation or transient state).
func storeMaxStaleness(status, targetAction string) time.Duration {
	// Active operation in progress → always fetch live
	if targetAction != model.ActionComplete {
		return 0
	}
	switch status {
	case model.StatusTerminated, model.StatusFailed:
		return 60 * time.Minute // final states never change without user action
	case model.StatusSuspended:
		return 30 * time.Minute
	case model.StatusRunning:
		return pollIntervals[PollNormal] // matches daemon polling interval
	default:
		return 0 // all other states (Creating, Suspending, …): always live
	}
}

// StatusEntry is the in-memory representation of a Node's current state
// maintained by NodeStatusAgent. Fields are safe to read via StatusStore.Get.
type StatusEntry struct {
	// Status fields — updated on each CSP poll
	Status        string
	NativeStatus  string
	PublicIP      string
	TargetStatus  string
	TargetAction  string
	SystemMessage string
	LastUpdated   time.Time // last time Status was refreshed from CSP; zero = never

	// Daemon scheduling
	NextPollAt time.Time
	Priority   PollPriority

	// Operation lock.
	// Zero value = not locked.
	// When set and within operationLockTTL, the daemon skips this node because
	// a blocking lifecycle operation (Create, Suspend, …) owns its state.
	OperationLockedAt time.Time

	// Node identity — needed by the polling goroutine
	NsId            string
	InfraId         string
	NodeId          string
	ConnectionName  string
	CspResourceName string
	CspResourceId   string // CSP-native instance ID (e.g., "i-014fa6ede6ada0b2c"), used by batch SDK sweeper
	ProviderName    string
	Region          string
	CredentialHolder string // credential owner, used as context key for SDK calls

	// Node metadata — static fields from NodeInfo stored here so that
	// fetchNodeStatusWithCache can serve cache hits without a KV round-trip.
	Name           string
	PrivateIP      string
	SSHPort        int
	CreatedTime    string
	Location       model.Location
	MonAgentStatus string
}

// IsOperationLocked reports whether a live operation lock holds this node.
func (e *StatusEntry) IsOperationLocked() bool {
	return !e.OperationLockedAt.IsZero() &&
		time.Since(e.OperationLockedAt) < operationLockTTL
}

// IsLockTTLExpired reports a potential goroutine leak: the lock TTL has
// elapsed but TargetAction is still non-complete.
func (e *StatusEntry) IsLockTTLExpired() bool {
	return !e.OperationLockedAt.IsZero() &&
		time.Since(e.OperationLockedAt) >= operationLockTTL &&
		e.TargetAction != model.ActionComplete
}

// IsFresh reports whether the entry is still within its acceptable staleness
// window and can be served without a live CSP call.
func (e *StatusEntry) IsFresh() bool {
	maxAge := storeMaxStaleness(e.Status, e.TargetAction)
	return maxAge > 0 && !e.LastUpdated.IsZero() && time.Since(e.LastUpdated) < maxAge
}

// StatusStore is the in-memory status cache for all Nodes.
// It is safe for concurrent use.
type StatusStore struct {
	mu      sync.RWMutex
	entries map[string]*StatusEntry // key: "nsId/infraId/nodeId"
}

// globalStatusStore is the process-wide singleton.
var globalStatusStore = &StatusStore{
	entries: make(map[string]*StatusEntry),
}

func storeKey(nsId, infraId, nodeId string) string {
	return nsId + "/" + infraId + "/" + nodeId
}

// Get returns a copy of the StatusEntry for the node. ok is false if not found.
func (s *StatusStore) Get(nsId, infraId, nodeId string) (StatusEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.entries[storeKey(nsId, infraId, nodeId)]
	if !ok {
		return StatusEntry{}, false
	}
	return *e, true
}

// Set stores a StatusEntry for a node, replacing any existing entry.
func (s *StatusStore) Set(nsId, infraId, nodeId string, e StatusEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[storeKey(nsId, infraId, nodeId)] = &e
}

// Update applies fn to the StatusEntry for the node in place, creating it if absent.
func (s *StatusStore) Update(nsId, infraId, nodeId string, fn func(*StatusEntry)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := storeKey(nsId, infraId, nodeId)
	e, ok := s.entries[key]
	if !ok {
		e = &StatusEntry{NsId: nsId, InfraId: infraId, NodeId: nodeId}
		s.entries[key] = e
	}
	fn(e)
}

// Delete removes the StatusEntry for the node.
func (s *StatusStore) Delete(nsId, infraId, nodeId string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, storeKey(nsId, infraId, nodeId))
}

// Snapshot returns a copy of all current entries.
func (s *StatusStore) Snapshot() []StatusEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]StatusEntry, 0, len(s.entries))
	for _, e := range s.entries {
		out = append(out, *e)
	}
	return out
}
