package reconcile

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/rs/zerolog/log"
)

// formatDuration converts seconds to human-readable format
// Examples: 125.5s -> "2m 5s", 2.3s -> "2.3s", 0.234s -> "234ms"
func formatDuration(seconds float64) string {
	if seconds >= 60 {
		minutes := int(seconds / 60)
		secs := int(seconds) % 60
		return fmt.Sprintf("%dm %ds", minutes, secs)
	}
	if seconds >= 1 {
		return fmt.Sprintf("%.1fs", seconds)
	}
	return fmt.Sprintf("%dms", int(seconds*1000))
}

// roundTo2Decimals rounds a float to 2 decimal places
func roundTo2Decimals(val float64) float64 {
	return math.Round(val*100) / 100
}

// VNetReconciler implements the Reconciler interface for VNet resources.
type VNetReconciler struct{}

func init() {
	GetManager().RegisterReconciler(model.StrVNet, &VNetReconciler{})
}

// Reconcile performs the state machine logic for VNet and delegates the actual sync to SyncVNetState.
func (r *VNetReconciler) Reconcile(ctx context.Context, nsId string, resourceId string, optPreloadedVNetStatus *model.CspResourceStatusResponse) (any, error) {
	log.Info().Msgf("Reconcile started for VNet: %s/%s", nsId, resourceId)

	// 1. Retrieve the Expected State from DB
	vNetKey := common.GenResourceKey(nsId, model.StrVNet, resourceId)
	// TODO: distributed-lock [ACQUIRE] key=vNetKey
	// Lock must be acquired here — before the first kvstore read — so that the entire
	// read → routing decision → SyncVNetState execution is a single atomic critical section.
	// TODO: distributed-lock [RELEASE] defer unlock immediately after acquire (covers all return paths)
	keyValue, exists, err := kvstore.GetKv(vNetKey)
	if err != nil {
		return nil, fmt.Errorf("failed to read vNet from DB: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("does not exist, vNet: %s", resourceId)
	}

	var vNetInfo model.VNetInfo
	if err := json.Unmarshal([]byte(keyValue.Value), &vNetInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal vNet info: %w", err)
	}

	// 2. Resolve CSP status — Reconciler is responsible for fetching it once.
	//    If a preloaded cache is provided (e.g., from a batch reconcile), use it directly.
	//    Otherwise fetch from Spider here so that all downstream calls receive a non-nil status.
	var vpcStatusResp model.CspResourceStatusResponse
	if optPreloadedVNetStatus != nil {
		vpcStatusResp = *optPreloadedVNetStatus
		log.Debug().Msgf("Using preloaded VNet status (connection: %s)", vNetInfo.ConnectionName)
	} else {
		log.Debug().Msgf("[Request to Spider] Listing all vNets for connection: %s", vNetInfo.ConnectionName)
		var fetchErr error
		vpcStatusResp, fetchErr = resource.GetCspResourceStatus(vNetInfo.ConnectionName, model.StrVNet)
		if fetchErr != nil {
			log.Error().Err(fetchErr).Msg("failed to get vNet resource status from Spider, skipping reconciliation")
			return model.SimpleMsg{}, fmt.Errorf("failed to reconcile vNet '%s': %w", resourceId, fetchErr)
		}
	}

	// 3. State Machine Handling based on Current 	// 3. State Machine Handling based on Current DB Status
	switch vNetInfo.Status {
	case model.NetworkStatusAvailable:
		return r.reconcileAvailable(nsId, &vNetInfo, &vpcStatusResp)

	case model.NetworkStatusFailed:
		return r.reconcileFailed(nsId, &vNetInfo, &vpcStatusResp)

	case model.NetworkStatusCreating:
		log.Warn().Msgf("vNet (%s) is stuck in Creating. Verifying CSP status...", resourceId)
		return r.reconcileCreating(nsId, &vNetInfo, &vpcStatusResp)

	case model.NetworkStatusDeleting:
		log.Warn().Msgf("vNet (%s) is stuck in Deleting. Re-triggering deletion logic...", resourceId)
		return r.reconcileDeleting(nsId, &vNetInfo, &vpcStatusResp)

	default:
		return model.SimpleMsg{}, fmt.Errorf("invalid resource status: %s", vNetInfo.Status)
	}
}

// reconcileAvailable reconciles a VNet in Available status by syncing child subnets and checking CSP state.
func (r *VNetReconciler) reconcileAvailable(nsId string, vNetInfo *model.VNetInfo, vpcStatusResp *model.CspResourceStatusResponse) (model.SimpleMsg, error) {
	vNetKey := common.GenResourceKey(nsId, model.StrVNet, vNetInfo.Id)

	// Always reconcile child subnets first so child drift is diagnosed
	r.reconcileChildSubnets(nsId, vNetInfo, vpcStatusResp)

	syncState := resource.GetResourceSyncState(vNetInfo.CspResourceName, vNetInfo.CspResourceId, *vpcStatusResp)
	// KT: the VPC is the account default network and always "exists"; restore only if tiers remain.
	if syncState == model.SyncStateInSync || syncState == model.SyncStateSpMetaMissing {
		if conn, cerr := common.GetConnConfig(vNetInfo.ConnectionName); cerr == nil && strings.EqualFold(conn.ProviderName, "kt") {
			if present, perr := resource.VNetPresentOnCsp(*vNetInfo); perr == nil && !present {
				log.Info().Msgf("vNet (%s): KT default network present but no tiers remain; treating as CspResourceMissing", vNetInfo.Id)
				syncState = model.SyncStateCspResourceMissing
			}
		}
	}
	switch syncState {
	case model.SyncStateInSync:
		model.SetCondition(&vNetInfo.Conditions, model.ConditionSynced, model.ConditionTrue, model.ReasonAvailable, "Resource is in sync across all layers")
	case model.SyncStateSpMetaMissing:
		model.SetCondition(&vNetInfo.Conditions, model.ConditionSynced, model.ConditionFalse, string(syncState), "Spider metadata missing; TB metadata preserved")
	case model.SyncStateCspResourceMissing:
		model.SetCondition(&vNetInfo.Conditions, model.ConditionReady, model.ConditionFalse, string(syncState), "Resource missing on CSP provider")
		model.SetCondition(&vNetInfo.Conditions, model.ConditionSynced, model.ConditionFalse, string(syncState), "Resource missing on CSP provider")
		vNetInfo.SystemMessage = "Reconcile Diagnostic: CSP resource missing."
	case model.SyncStateTbMetaOnly:
		model.SetCondition(&vNetInfo.Conditions, model.ConditionReady, model.ConditionFalse, string(syncState), "Ghost metadata: resource absent on Spider and CSP")
		model.SetCondition(&vNetInfo.Conditions, model.ConditionSynced, model.ConditionFalse, string(syncState), "Ghost metadata: resource absent on Spider and CSP")
		vNetInfo.SystemMessage = "Reconcile Diagnostic: Ghost metadata detected."
	}
	vNetInfo.Status = model.DeriveVNetStatus(vNetInfo.Conditions)

	val, err := json.Marshal(vNetInfo)
	if err != nil {
		return model.SimpleMsg{}, err
	}
	// PutResourceObject (not a plain kvstore.Put) preserves AssociatedObjectList against a concurrent update.
	if putErr := resource.PutResourceObject(vNetKey, val); putErr != nil {
		return model.SimpleMsg{}, putErr
	}
	return model.SimpleMsg{Message: fmt.Sprintf("vNet (%s) reconciled", vNetInfo.Id)}, nil
}

// reconcileFailed handles self-healing for resources in Failed status if CSP resource still exists.
func (r *VNetReconciler) reconcileFailed(nsId string, vNetInfo *model.VNetInfo, vpcStatusResp *model.CspResourceStatusResponse) (model.SimpleMsg, error) {
	vNetKey := common.GenResourceKey(nsId, model.StrVNet, vNetInfo.Id)

	// Always reconcile child subnets first so child drift is diagnosed
	r.reconcileChildSubnets(nsId, vNetInfo, vpcStatusResp)

	syncState := resource.GetResourceSyncState(vNetInfo.CspResourceName, vNetInfo.CspResourceId, *vpcStatusResp)

	// A user-owned deletion tombstone is sticky: never self-heal it back to Available
	// (only auto-managed shared resources are restorable for reuse). The label lookup
	// does I/O, so evaluate it only when a restore is actually on the table.
	restoreOk := (syncState == model.SyncStateInSync || syncState == model.SyncStateSpMetaMissing) &&
		model.ShouldRestoreToAvailable(vNetInfo.Conditions)
	if restoreOk {
		if cond := model.GetCondition(vNetInfo.Conditions, model.ConditionReady); cond != nil &&
			cond.Reason == model.ReasonDeletionFailed && !resource.IsAutoManagedResource(nsId, vNetInfo.Id, model.StrVNet, vNetInfo.Uid) {
			restoreOk = false
		}
	}

	switch {
	case restoreOk:
		prevReason, prevMessage := "", ""
		if cond := model.GetCondition(vNetInfo.Conditions, model.ConditionReady); cond != nil {
			prevReason = cond.Reason
			prevMessage = cond.Message
		}
		log.Info().Msgf("vNet (%s) restored from %s to Available; CSP resource exists", vNetInfo.Id, prevReason)

		restoredMsg := fmt.Sprintf("Restored from %s; CSP resource exists", prevReason)
		if prevMessage != "" {
			restoredMsg = fmt.Sprintf("%s (previous failure: %s)", restoredMsg, prevMessage)
		}
		model.SetCondition(&vNetInfo.Conditions, model.ConditionReady, model.ConditionTrue, model.ReasonRestored, restoredMsg)
		model.SetCondition(&vNetInfo.Conditions, model.ConditionSynced, model.ConditionTrue, model.ReasonAvailable, "Resource is in sync across all layers")
		vNetInfo.SystemMessage = ""

	case syncState == model.SyncStateCspResourceMissing:
		model.SetCondition(&vNetInfo.Conditions, model.ConditionReady, model.ConditionFalse, string(syncState), "Resource missing on CSP provider")
		model.SetCondition(&vNetInfo.Conditions, model.ConditionSynced, model.ConditionFalse, string(syncState), "Resource missing on CSP provider")
		vNetInfo.SystemMessage = "Reconcile Diagnostic: CSP resource missing."

	case syncState == model.SyncStateTbMetaOnly:
		model.SetCondition(&vNetInfo.Conditions, model.ConditionReady, model.ConditionFalse, string(syncState), "Ghost metadata: resource absent on Spider and CSP")
		model.SetCondition(&vNetInfo.Conditions, model.ConditionSynced, model.ConditionFalse, string(syncState), "Ghost metadata: resource absent on Spider and CSP")
		vNetInfo.SystemMessage = "Reconcile Diagnostic: Ghost metadata detected."

	case syncState == model.SyncStateSpMetaMissing:
		// Not authorized to restore — record the diagnosis only; Ready/Status/SystemMessage stay untouched.
		model.SetCondition(&vNetInfo.Conditions, model.ConditionSynced, model.ConditionFalse, string(syncState), "Spider metadata missing; TB metadata preserved")

	default: // SyncStateInSync, not authorized to restore (sticky tombstone)
		model.SetCondition(&vNetInfo.Conditions, model.ConditionSynced, model.ConditionTrue, model.ReasonAvailable, "Resource is in sync across all layers")
	}
	vNetInfo.Status = model.DeriveVNetStatus(vNetInfo.Conditions)

	val, err := json.Marshal(vNetInfo)
	if err != nil {
		return model.SimpleMsg{}, err
	}
	// PutResourceObject (not a plain kvstore.Put) preserves AssociatedObjectList against a concurrent update.
	if putErr := resource.PutResourceObject(vNetKey, val); putErr != nil {
		return model.SimpleMsg{}, putErr
	}
	return model.SimpleMsg{Message: fmt.Sprintf("vNet (%s) reconciled", vNetInfo.Id)}, nil
}

// reconcileCreating handles stuck creation status (skeleton for future implementation).
// TODO: Implement creation recovery after detailed verification:
// 1. If resource exists on CSP -> promote status to Available.
// 2. If resource missing on CSP -> mark status as Failed (Reason: CreationFailed).
func (r *VNetReconciler) reconcileCreating(nsId string, vNetInfo *model.VNetInfo, vpcStatusResp *model.CspResourceStatusResponse) (model.SimpleMsg, error) {
	log.Info().Msgf("reconcileCreating called for vNet (%s); logic is under construction", vNetInfo.Id)
	return model.SimpleMsg{Message: fmt.Sprintf("vNet (%s) creation recovery logic is under construction (skeleton)", vNetInfo.Id)}, nil
}

// reconcileDeleting retries the fail-closed delete for a vNet stuck in Deleting: it purges
// the record if the CSP resource is now gone, or keeps it if still present. Idempotent.
func (r *VNetReconciler) reconcileDeleting(nsId string, vNetInfo *model.VNetInfo, vpcStatusResp *model.CspResourceStatusResponse) (model.SimpleMsg, error) {
	if _, err := resource.DeleteVNet(nsId, vNetInfo.Id, resource.ActionWithSubnets.String()); err != nil {
		log.Warn().Err(err).Msgf("vNet (%s) deletion still unconfirmed; record retained for retry", vNetInfo.Id)
		return model.SimpleMsg{Message: fmt.Sprintf("vNet (%s) deletion retried; still present, retained", vNetInfo.Id)}, nil
	}
	return model.SimpleMsg{Message: fmt.Sprintf("vNet (%s) deletion completed (record purged)", vNetInfo.Id)}, nil
}

// reconcileChildSubnets reconciles child subnets for a parent VNet.
func (r *VNetReconciler) reconcileChildSubnets(nsId string, vNetInfo *model.VNetInfo, vpcStatusResp *model.CspResourceStatusResponse) {
	vNetKey := common.GenResourceKey(nsId, model.StrVNet, vNetInfo.Id)
	subnetKvList, err := kvstore.GetKvList(vNetKey + "/subnet")
	if err != nil || len(subnetKvList) == 0 {
		return
	}

	subnetStatus, subnetErr := resource.GetCspResourceStatus(
		vNetInfo.ConnectionName,
		model.StrSubnet,
		resource.ResourceStatusFilter{ParentResourceId: vNetInfo.CspResourceName},
	)
	var optPreloadedSubnetStatus *model.CspResourceStatusResponse
	if subnetErr == nil {
		optPreloadedSubnetStatus = &subnetStatus
	}

	if summary, sErr := resource.SyncSubnetsForVNet(nsId, subnetKvList, vNetInfo, optPreloadedSubnetStatus); sErr == nil {
		log.Debug().Msgf("Subnet sync for vNet (%s): total %d, restored %d, cleaned %d", vNetInfo.Id, summary.Total, summary.Restored, summary.Cleaned)
	}

	latestKv, latestExists, latestErr := kvstore.GetKv(vNetKey)
	if latestErr == nil && latestExists {
		if uErr := json.Unmarshal([]byte(latestKv.Value), vNetInfo); uErr != nil {
			log.Warn().Err(uErr).Msg("failed to unmarshal latest vNet info; using in-memory copy")
		}
	}
}

// ReconcileAll reconciles all VNets in the namespace by comparing TB metadata with CSP state.
// This method batches reconciliation with optimized API calls (pre-fetch status per connection).
func (r *VNetReconciler) ReconcileAll(ctx context.Context, nsId string, maxConcurrent int) (model.ResourceReconcileResults, error) {
	startTime := time.Now()
	log.Info().Msgf("ReconcileAll VNets started for namespace: %s (maxConcurrent: %d)", nsId, maxConcurrent)

	// 1. List all VNets in the namespace
	result, err := resource.ListResource(nsId, model.StrVNet, "", "")
	if err != nil {
		return model.ResourceReconcileResults{}, fmt.Errorf("failed to list VNets: %w", err)
	}

	vnetList, ok := result.([]model.VNetInfo)
	if !ok {
		return model.ResourceReconcileResults{}, fmt.Errorf("unexpected type from ListResource: expected []model.VNetInfo")
	}

	if len(vnetList) == 0 {
		log.Info().Msg("No VNets found in namespace")
		return model.ResourceReconcileResults{
			Total:        0,
			SuccessCount: 0,
			FailedCount:  0,
			Results:      []model.ResourceReconcileResult{},
		}, nil
	}

	// 2. Group VNets by connection to optimize API calls
	connectionGroups := make(map[string][]model.VNetInfo)
	for _, vnet := range vnetList {
		connectionGroups[vnet.ConnectionName] = append(connectionGroups[vnet.ConnectionName], vnet)
	}

	log.Info().Msgf("Grouped %d VNets across %d connections", len(vnetList), len(connectionGroups))
	log.Info().Msg("Starting pipeline: fetch status and reconcile per connection (optimized for minimal wait time)")

	// 3. Pipeline approach: Fetch status → Immediately reconcile (per connection)
	//    This eliminates wait time between fetch and reconcile phases.
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var results []model.ResourceReconcileResult
	var reconciledCount int32
	var fetchedConnCount int32
	totalConnections := int32(len(connectionGroups))

	// Process each connection independently: fetch → reconcile pipeline
	for connName, vnets := range connectionGroups {
		wg.Add(1)
		go func(conn string, vnetList []model.VNetInfo) {
			defer wg.Done()
			connStartTime := time.Now()

			// Step 1: Fetch CSP status for this connection
			log.Info().Msgf("[%s] Fetching VNet status (%d VNets)...", conn, len(vnetList))
			fetchStartTime := time.Now()
			status, fetchErr := resource.GetCspResourceStatus(conn, model.StrVNet)
			fetchElapsed := time.Since(fetchStartTime).Seconds()
			completed := atomic.AddInt32(&fetchedConnCount, 1)
			log.Info().Msgf("[%s] Status fetch complete (%d/%d connections, %.2fs)", conn, completed, totalConnections, fetchElapsed)

			if fetchErr != nil {
				log.Warn().Err(fetchErr).Msgf("[%s] Failed to fetch VNet status; skipping %d VNets", conn, len(vnetList))
				// Record failures for all VNets in this connection
				mu.Lock()
				for _, vnet := range vnetList {
					fetchElapsedRounded := roundTo2Decimals(fetchElapsed)
					results = append(results, model.ResourceReconcileResult{
						ResourceType:   model.StrVNet,
						ResourceId:     vnet.Id,
						ConnectionName: conn,
						Success:        false,
						ElapsedSeconds: fetchElapsedRounded,
						Elapsed:        formatDuration(fetchElapsedRounded),
						Error:          fmt.Sprintf("failed to fetch CSP status for connection: %s", conn),
					})
				}
				mu.Unlock()
				return
			}
			// Step 2: Immediately start reconciling VNets (no wait!)
			log.Info().Msgf("[%s] Starting reconciliation for %d VNets...", conn, len(vnetList))
			var connWg sync.WaitGroup
			for _, vnet := range vnetList {
				connWg.Add(1)
				go func(v model.VNetInfo) {
					defer connWg.Done()
					vnetStartTime := time.Now()

					// Acquire semaphore
					sem <- struct{}{}
					defer func() { <-sem }()

					// Check context cancellation
					select {
					case <-ctx.Done():
						cancelElapsed := roundTo2Decimals(time.Since(vnetStartTime).Seconds())
						mu.Lock()
						results = append(results, model.ResourceReconcileResult{
							ResourceType:   model.StrVNet,
							ResourceId:     v.Id,
							ConnectionName: conn,
							Success:        false,
							ElapsedSeconds: cancelElapsed,
							Elapsed:        formatDuration(cancelElapsed),
							Error:          "reconciliation cancelled",
						})
						mu.Unlock()
						return
					default:
					}

					// Run reconcile with fetched status
					resp, recErr := r.Reconcile(ctx, nsId, v.Id, &status)
					vnetElapsed := roundTo2Decimals(time.Since(vnetStartTime).Seconds())

					// Build result
					result := model.ResourceReconcileResult{
						ResourceType:   model.StrVNet,
						ResourceId:     v.Id,
						ConnectionName: conn,
						Success:        recErr == nil,
						ElapsedSeconds: vnetElapsed,
						Elapsed:        formatDuration(vnetElapsed),
					}

					if recErr != nil {
						result.Error = recErr.Error()
						log.Warn().Err(recErr).Msgf("[%s] Failed to reconcile VNet: %s (%.2fs)", conn, v.Id, vnetElapsed)
					} else if msg, ok := resp.(model.SimpleMsg); ok {
						result.Message = msg.Message
						log.Debug().Msgf("[%s] Reconciled VNet %s: %s (%.2fs)", conn, v.Id, msg.Message, vnetElapsed)
					}

					mu.Lock()
					results = append(results, result)
					mu.Unlock()

					// Progress logging (only for batches)
					completed := atomic.AddInt32(&reconciledCount, 1)
					if len(vnetList) > 10 && (completed%10 == 0 || completed == int32(len(vnetList))) {
						log.Info().Msgf("Reconciliation progress: %d/%d VNets complete", completed, len(vnetList))
					}
				}(vnet)
			}

			// Wait for all VNets in this connection to complete
			connWg.Wait()
			connElapsed := time.Since(connStartTime).Seconds()
			log.Info().Msgf("[%s] Connection reconciliation complete (%.2fs total)", conn, connElapsed)
		}(connName, vnets)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// 5. Build aggregated response
	successCount := 0
	failedCount := 0
	for _, res := range results {
		if res.Success {
			successCount++
		} else {
			failedCount++
		}
	}

	totalElapsed := roundTo2Decimals(time.Since(startTime).Seconds())
	response := model.ResourceReconcileResults{
		Total:          len(results),
		SuccessCount:   successCount,
		FailedCount:    failedCount,
		ElapsedSeconds: totalElapsed,
		Elapsed:        formatDuration(totalElapsed),
		Results:        results,
	}

	log.Info().Msgf("ReconcileAll VNets completed for namespace %s: total=%d, success=%d, failed=%d, elapsed=%s",
		nsId, response.Total, response.SuccessCount, response.FailedCount, response.Elapsed)

	return response, nil
}
