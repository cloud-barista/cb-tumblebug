package reconcile

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
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

	// 3. State Machine Handling based on Current DB Status
	switch vNetInfo.Status {
	case model.NetworkStatusFailed:
		// Check condition if it's DeletionFailed
		cond := model.GetCondition(vNetInfo.Conditions, model.ConditionReady)
		if cond != nil && cond.Reason == model.ReasonDeletionFailed {
			log.Info().Msgf("vNet (%s) failed during deletion. Checking CSP status for self-healing...", resourceId)
			// Delegate the actual diffing to the lower-level Sync function
			return resource.SyncVNetState(nsId, &vNetInfo, &vpcStatusResp)
		}
		// TODO: Handle creation failures
		return resource.SyncVNetState(nsId, &vNetInfo, &vpcStatusResp)

	case model.NetworkStatusCreating:
		// Handle stuck creation
		log.Warn().Msgf("vNet (%s) is stuck in Creating. Verifying CSP status...", resourceId)
		return r.handleCreation(nsId, resourceId, &vNetInfo, &vpcStatusResp)

	case model.NetworkStatusDeleting:
		// Handle stuck deletion
		log.Warn().Msgf("vNet (%s) is stuck in Deleting. Re-triggering deletion logic...", resourceId)
		return r.handleDeletion(nsId, resourceId, &vNetInfo, &vpcStatusResp)

	case model.NetworkStatusAvailable:
		// Periodic sync: even in a normal state, detect differences between DB and CSP.
		return resource.SyncVNetState(nsId, &vNetInfo, &vpcStatusResp)

	default:
		// Unknown state
		return model.SimpleMsg{}, fmt.Errorf("invalid resource status: %s", vNetInfo.Status)
	}
}

func (r *VNetReconciler) handleCreation(nsId string, resourceId string, vNetInfo *model.VNetInfo, optPreloadedVNetStatus *model.CspResourceStatusResponse) (interface{}, error) {
	// TODO: Implement dedicated creation recovery logic here.
	// optPreloadedVNetStatus is already resolved by the caller (Reconcile).
	// Future Implementation Steps:
	// 1. If the resource exists and is fully provisioned on CSP → Update DB status to Available.
	// 2. If the resource does not exist → Mark DB status as Failed(CreationFailed).
	// 3. If the resource is still provisioning → Keep Creating state and wait for the next Reconcile cycle.
	return model.SimpleMsg{Message: "Creation recovery logic is under construction (skeleton)"}, nil
}

func (r *VNetReconciler) handleDeletion(nsId string, resourceId string, vNetInfo *model.VNetInfo, optPreloadedVNetStatus *model.CspResourceStatusResponse) (interface{}, error) {
	// TODO: Implement dedicated deletion recovery logic here.
	// optPreloadedVNetStatus is already resolved by the caller (Reconcile).
	// Future Implementation Steps:
	// 1. If the resource no longer exists on CSP → Clean up DB metadata (deletion complete).
	// 2. If the resource still exists on CSP → Attempt deletion again via CSP API (retry).
	// 3. If deletion fails permanently (e.g., due to dependencies) → Mark DB status as Failed(DeletionFailed).
	return model.SimpleMsg{Message: "Deletion recovery logic is under construction (skeleton)"}, nil
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
