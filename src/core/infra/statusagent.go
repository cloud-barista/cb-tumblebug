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
	"context"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

// cspRateLimits defines the default CB-Spider API call rates per second per CSP.
// These limit concurrent polling towards Spider (and transitively towards the CSP).
var cspRateLimits = map[string]float64{
	"aws":     10,
	"azure":   8,
	"gcp":     12,
	"alibaba": 6,
	"ncp":     3,
	"nhn":     5,
	"ibm":     5,
}

const defaultRateLimit = 5.0 // req/s for unknown CSPs

// NodeStatusAgent is the background daemon that periodically refreshes Node
// statuses from CB-Spider and keeps StatusStore up to date.
//
// Design:
//   - A scan loop (1 s tick) finds nodes whose NextPollAt has passed and sends
//     them to a worker pool via a buffered channel.
//   - Workers call FetchNodeStatus (which writes results through to StatusStore).
//   - Per-CSP+region rate.Limiters prevent Spider / CSP API throttling.
//   - Nodes locked by an active lifecycle operation (AcquireLock) are skipped
//     until the operation completes (ReleaseLock) or the lock TTL expires.
type NodeStatusAgent struct {
	workerCh chan StatusEntry // items ready to be polled
	limiters sync.Map        // map[string]*rate.Limiter, keyed "provider/region"
	workers  int
}

// GlobalAgent is the process-wide NodeStatusAgent singleton.
var GlobalAgent = &NodeStatusAgent{
	workerCh: make(chan StatusEntry, 200),
	workers:  20,
}

// Start launches the scan loop and worker pool.
// Blocks until ctx is cancelled (call in a goroutine).
func (a *NodeStatusAgent) Start(ctx context.Context) {
	log.Info().Msg("[NodeStatusAgent] Starting")

	for i := 0; i < a.workers; i++ {
		go a.worker(ctx)
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("[NodeStatusAgent] Stopped")
			return
		case <-ticker.C:
			a.dispatchEligible()
		}
	}
}

// dispatchEligible scans StatusStore and sends eligible nodes to the worker pool.
func (a *NodeStatusAgent) dispatchEligible() {
	now := time.Now()
	for _, e := range globalStatusStore.Snapshot() {
		if e.Priority == PollSkip {
			continue
		}

		// Operation lock check: two distinct cases depending on whether TTL is still valid.
		if !e.OperationLockedAt.IsZero() {
			if e.IsOperationLocked() {
				// Lock is active and within TTL — daemon must not poll.
				continue
			}
			// Lock TTL has elapsed.
			if e.TargetAction != model.ActionComplete {
				// Potential goroutine leak: a blocking operation started but never
				// released the lock. Warn and promote so the daemon re-checks CSP.
				log.Warn().
					Str("nodeId", e.NodeId).
					Str("infraId", e.InfraId).
					Str("targetAction", e.TargetAction).
					Msg("[NodeStatusAgent] Operation lock TTL expired with pending TargetAction; clearing lock and promoting to URGENT. Run action=reconcile if node is stuck.")
				globalStatusStore.Update(e.NsId, e.InfraId, e.NodeId, func(ent *StatusEntry) {
					ent.OperationLockedAt = time.Time{}
					ent.Priority = PollUrgent
					ent.NextPollAt = now
				})
				continue // picked up as URGENT on next tick
			}
			// TTL elapsed but TargetAction is already complete — lock was never cleared
			// (shouldn't happen in normal flow). Clear it silently and fall through.
			globalStatusStore.Update(e.NsId, e.InfraId, e.NodeId, func(ent *StatusEntry) {
				ent.OperationLockedAt = time.Time{}
			})
		}

		if now.Before(e.NextPollAt) {
			continue
		}

		// Tentatively push NextPollAt to prevent re-dispatch while worker runs.
		interval := pollIntervals[e.Priority]
		if interval == 0 {
			interval = pollIntervals[PollNormal]
		}
		globalStatusStore.Update(e.NsId, e.InfraId, e.NodeId, func(ent *StatusEntry) {
			ent.NextPollAt = now.Add(interval)
		})

		select {
		case a.workerCh <- e:
		default:
			// Pool saturated; revert so this node is retried next tick.
			globalStatusStore.Update(e.NsId, e.InfraId, e.NodeId, func(ent *StatusEntry) {
				ent.NextPollAt = now
			})
		}
	}
}

// worker processes poll items from workerCh.
func (a *NodeStatusAgent) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case entry := <-a.workerCh:
			a.poll(ctx, entry)
		}
	}
}

// poll rate-limits then calls FetchNodeStatus.
// FetchNodeStatus writes the result through to StatusStore (via writeStatusToStore).
func (a *NodeStatusAgent) poll(ctx context.Context, entry StatusEntry) {
	limiter := a.getLimiter(entry.ProviderName, entry.Region)
	if err := limiter.Wait(ctx); err != nil {
		return // ctx cancelled
	}

	_, err := FetchNodeStatus(entry.NsId, entry.InfraId, entry.NodeId)
	if err != nil {
		log.Debug().Err(err).
			Str("nodeId", entry.NodeId).
			Msg("[NodeStatusAgent] FetchNodeStatus failed; will retry at next scheduled poll")
	}
}

// AcquireLock marks a node as operation-locked so the daemon skips polling it.
// status is the transitional state to record (e.g. StatusCreating, StatusSuspending).
// targetAction is the operation being performed (e.g. ActionCreate, ActionSuspend).
// Call before issuing any blocking Spider operation (POST /vm, suspend, resume, …).
func (a *NodeStatusAgent) AcquireLock(nsId, infraId, nodeId, status, targetAction string) {
	globalStatusStore.Update(nsId, infraId, nodeId, func(e *StatusEntry) {
		e.Status = status
		e.TargetAction = targetAction
		e.OperationLockedAt = time.Now()
		e.Priority = PollSkip
		e.LastUpdated = time.Time{} // mark stale so first post-op fetch is live
		e.NsId = nsId
		e.InfraId = infraId
		e.NodeId = nodeId
	})
}

// ReleaseLock clears the operation lock so the daemon resumes normal polling.
// Call after the blocking Spider operation returns (success or error).
func (a *NodeStatusAgent) ReleaseLock(nsId, infraId, nodeId string) {
	globalStatusStore.Update(nsId, infraId, nodeId, func(e *StatusEntry) {
		e.OperationLockedAt = time.Time{}
	})
}

// Promote bumps a node's priority and schedules it for an immediate re-poll.
// Useful after an operation completes to confirm the final state quickly.
func (a *NodeStatusAgent) Promote(nsId, infraId, nodeId string, priority PollPriority) {
	globalStatusStore.Update(nsId, infraId, nodeId, func(e *StatusEntry) {
		if priority > e.Priority {
			e.Priority = priority
		}
		e.NextPollAt = time.Now()
	})
}

// getLimiter returns the rate.Limiter for the given CSP+region, creating it lazily.
func (a *NodeStatusAgent) getLimiter(provider, region string) *rate.Limiter {
	key := strings.ToLower(provider) + "/" + region
	if v, ok := a.limiters.Load(key); ok {
		return v.(*rate.Limiter)
	}
	r, ok := cspRateLimits[strings.ToLower(provider)]
	if !ok {
		r = defaultRateLimit
	}
	l := rate.NewLimiter(rate.Limit(r), int(r))
	actual, _ := a.limiters.LoadOrStore(key, l)
	return actual.(*rate.Limiter)
}

// StartupScan loads all Nodes from the KV store into StatusStore and schedules
// them for polling. Running (stable) nodes are spread over the first poll interval
// to avoid a burst of Spider calls immediately at startup.
// Nodes with a pending TargetAction (stuck since the last restart) are logged as
// warnings and queued for immediate polling; operators can run action=reconcile
// if they remain stuck.
func (a *NodeStatusAgent) StartupScan() {
	log.Info().Msg("[NodeStatusAgent] Starting startup scan")

	nsList, err := common.ListNsId()
	if err != nil {
		log.Error().Err(err).Msg("[NodeStatusAgent] StartupScan: cannot list namespaces")
		return
	}

	type nodeRef struct {
		nsId    string
		infraId string
		info    model.NodeInfo
	}

	var urgent []nodeRef
	var normal []nodeRef

	for _, nsId := range nsList {
		infraIds, err := ListInfraId(nsId)
		if err != nil {
			log.Warn().Err(err).Str("ns", nsId).Msg("[NodeStatusAgent] StartupScan: cannot list infras")
			continue
		}
		for _, infraId := range infraIds {
			nodeIds, err := ListNodeId(nsId, infraId)
			if err != nil {
				log.Warn().Err(err).Str("infra", infraId).Msg("[NodeStatusAgent] StartupScan: cannot list nodes")
				continue
			}
			for _, nodeId := range nodeIds {
				nodeInfo, err := GetNodeObject(nsId, infraId, nodeId)
				if err != nil {
					log.Warn().Err(err).Str("node", nodeId).Msg("[NodeStatusAgent] StartupScan: cannot read node")
					continue
				}
				ref := nodeRef{nsId: nsId, infraId: infraId, info: nodeInfo}
				if nodeInfo.TargetAction != model.ActionComplete {
					urgent = append(urgent, ref)
				} else {
					normal = append(normal, ref)
				}
			}
		}
	}

	log.Info().
		Int("urgent", len(urgent)).
		Int("normal", len(normal)).
		Msg("[NodeStatusAgent] StartupScan: scan complete")

	// Urgent: nodes with a pending TargetAction — schedule immediately and warn.
	for _, ref := range urgent {
		log.Warn().
			Str("nsId", ref.nsId).
			Str("infraId", ref.infraId).
			Str("nodeId", ref.info.Id).
			Str("status", ref.info.Status).
			Str("targetAction", ref.info.TargetAction).
			Msg("[NodeStatusAgent] StartupScan: node has pending TargetAction; scheduling URGENT poll. Run action=reconcile if stuck.")
		entry := buildStatusEntry(ref.nsId, ref.infraId, ref.info)
		entry.Priority = PollUrgent
		entry.NextPollAt = time.Now()
		globalStatusStore.Set(ref.nsId, ref.infraId, ref.info.Id, entry)
	}

	// Normal: spread over the first poll interval to prevent burst.
	var spreadInterval time.Duration
	if len(normal) > 0 {
		spreadInterval = pollIntervals[PollNormal] / time.Duration(len(normal))
	}
	for i, ref := range normal {
		entry := buildStatusEntry(ref.nsId, ref.infraId, ref.info)
		entry.NextPollAt = time.Now().Add(time.Duration(i) * spreadInterval)
		globalStatusStore.Set(ref.nsId, ref.infraId, ref.info.Id, entry)
	}
}

// buildStatusEntry creates a StatusEntry from a NodeInfo.
// LastUpdated is zero (never freshly fetched from CSP), so IsFresh() returns false
// until the daemon or a direct API call polls the node and writes through.
func buildStatusEntry(nsId, infraId string, nodeInfo model.NodeInfo) StatusEntry {
	providerName := nodeInfo.ConnectionConfig.ProviderName
	if providerName == "" {
		// Fallback: extract provider prefix from connection name (e.g. "aws-ap-northeast-2" → "aws")
		if parts := strings.SplitN(nodeInfo.ConnectionName, "-", 2); len(parts) > 0 {
			providerName = strings.ToLower(parts[0])
		}
	}
	priority := priorityForStatus(nodeInfo.Status, nodeInfo.TargetAction)
	return StatusEntry{
		Status:          nodeInfo.Status,
		NativeStatus:    nodeInfo.Status,
		PublicIP:        nodeInfo.PublicIP,
		TargetStatus:    nodeInfo.TargetStatus,
		TargetAction:    nodeInfo.TargetAction,
		SystemMessage:   nodeInfo.SystemMessage,
		LastUpdated:     time.Time{}, // never freshly fetched
		Priority:        priority,
		NextPollAt:      time.Now(), // poll as soon as scheduled
		NsId:            nsId,
		InfraId:         infraId,
		NodeId:          nodeInfo.Id,
		ConnectionName:  nodeInfo.ConnectionName,
		CspResourceName: nodeInfo.CspResourceName,
		ProviderName:    providerName,
		Region:          nodeInfo.Region.Region,
	}
}

// priorityForStatus maps a node status + TargetAction to a PollPriority.
func priorityForStatus(status, targetAction string) PollPriority {
	switch status {
	case model.StatusSuspending, model.StatusResuming,
		model.StatusRebooting, model.StatusTerminating:
		return PollHigh
	case model.StatusRunning:
		return PollNormal
	case model.StatusUndefined:
		return PollRecover
	case model.StatusTerminated, model.StatusFailed:
		return PollSkip
	case model.StatusSuspended:
		if targetAction == model.ActionComplete {
			return PollSkip // stable; no polling until user resumes
		}
		return PollHigh
	case model.StatusCreating:
		// AcquireLock sets PollSkip during the Spider POST /vm; dispatchEligible skips
		// locked nodes regardless of priority. PollHigh takes effect after ReleaseLock
		// so the daemon re-checks CSPs that return Creating before transitioning to Running.
		return PollHigh
	}
	return PollNormal
}

// writeStatusToStore updates StatusStore with a freshly fetched Node status.
// It always sets Priority and NextPollAt based on the returned status, even if
// the node is currently operation-locked — the lock's effect is enforced in
// dispatchEligible (which checks IsOperationLocked), not in scheduling data.
func writeStatusToStore(nsId, infraId, nodeId string, statusInfo model.NodeStatusInfo, nodeInfo model.NodeInfo) {
	providerName := nodeInfo.ConnectionConfig.ProviderName
	if providerName == "" {
		if parts := strings.SplitN(nodeInfo.ConnectionName, "-", 2); len(parts) > 0 {
			providerName = strings.ToLower(parts[0])
		}
	}

	globalStatusStore.Update(nsId, infraId, nodeId, func(e *StatusEntry) {
		e.Status = statusInfo.Status
		e.NativeStatus = statusInfo.NativeStatus
		e.PublicIP = statusInfo.PublicIp
		e.TargetStatus = statusInfo.TargetStatus
		e.TargetAction = statusInfo.TargetAction
		e.SystemMessage = statusInfo.SystemMessage
		e.LastUpdated = time.Now()

		// Identity fields (kept current in case they changed)
		e.NsId = nsId
		e.InfraId = infraId
		e.NodeId = nodeId
		e.ConnectionName = nodeInfo.ConnectionName
		e.CspResourceName = nodeInfo.CspResourceName
		e.ProviderName = providerName
		e.Region = nodeInfo.Region.Region

		// Recalculate polling schedule (lock enforcement is in dispatchEligible, not here)
		newPriority := priorityForStatus(statusInfo.Status, statusInfo.TargetAction)
		e.Priority = newPriority
		interval := pollIntervals[newPriority]
		if interval > 0 && (e.NextPollAt.IsZero() || !e.NextPollAt.After(time.Now())) {
			e.NextPollAt = time.Now().Add(interval)
		}
	})
}

// fetchNodeStatusWithCache checks StatusStore before calling FetchNodeStatus.
// If the stored entry is fresh (within its max-staleness window), it returns the
// cached status combined with current NodeInfo metadata — no Spider call is made.
// Otherwise it falls back to FetchNodeStatus which writes through to StatusStore.
func fetchNodeStatusWithCache(nsId, infraId, nodeId string) (model.NodeStatusInfo, error) {
	if entry, ok := globalStatusStore.Get(nsId, infraId, nodeId); ok && entry.IsFresh() {
		nodeInfo, err := GetNodeObject(nsId, infraId, nodeId)
		if err == nil {
			var cached model.NodeStatusInfo
			populateNodeStatusInfoFromNodeInfo(&cached, nodeInfo)
			// Override with fresh CSP-polled values from StatusStore
			cached.Status = entry.Status
			cached.NativeStatus = entry.NativeStatus
			cached.PublicIp = entry.PublicIP
			cached.TargetStatus = entry.TargetStatus
			cached.TargetAction = entry.TargetAction
			cached.SystemMessage = entry.SystemMessage
			return cached, nil
		}
	}
	return FetchNodeStatus(nsId, infraId, nodeId)
}
