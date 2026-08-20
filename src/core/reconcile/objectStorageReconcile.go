package reconcile

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/rs/zerolog/log"
)

// ObjectStorageReconciler implements the Reconciler interface for ObjectStorage resources.
type ObjectStorageReconciler struct{}

func init() {
	GetManager().RegisterReconciler(model.StrObjectStorage, &ObjectStorageReconciler{})
}

// Reconcile performs diagnosis and self-healing for ObjectStorage resources.
func (r *ObjectStorageReconciler) Reconcile(ctx context.Context, nsId string, resourceId string, optPreloadedStatus *model.CspResourceStatusResponse) (any, error) {
	log.Info().Msgf("Reconcile started for ObjectStorage: %s/%s", nsId, resourceId)

	// 1. Retrieve Expected State from DB
	osKey := common.GenResourceKey(nsId, model.StrObjectStorage, resourceId)
	keyValue, exists, err := kvstore.GetKv(osKey)
	if err != nil {
		return nil, fmt.Errorf("failed to read ObjectStorage from DB: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("does not exist, ObjectStorage: %s", resourceId)
	}

	var osInfo model.ObjectStorageInfo
	if err := json.Unmarshal([]byte(keyValue.Value), &osInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ObjectStorage info: %w", err)
	}

	// 2. Resolve CSP status once
	var statusResp model.CspResourceStatusResponse
	if optPreloadedStatus != nil {
		statusResp = *optPreloadedStatus
		log.Debug().Msgf("Using preloaded ObjectStorage status (connection: %s)", osInfo.ConnectionName)
	} else {
		log.Debug().Msgf("[Request to Spider] Listing all ObjectStorages for connection: %s", osInfo.ConnectionName)
		var fetchErr error
		statusResp, fetchErr = resource.GetCspResourceStatus(osInfo.ConnectionName, model.StrObjectStorage)
		if fetchErr != nil {
			log.Error().Err(fetchErr).Msg("failed to get ObjectStorage status from Spider, skipping reconciliation")
			return model.SimpleMsg{}, fmt.Errorf("failed to reconcile ObjectStorage '%s': %w", resourceId, fetchErr)
		}
	}

	// 3. State Machine Handling based on Current DB Status
	switch osInfo.Status {
	case model.StorageStatusAvailable:
		return r.reconcileAvailable(nsId, &osInfo, &statusResp)

	case model.StorageStatusFailed:
		return r.reconcileFailed(nsId, &osInfo, &statusResp)

	case model.StorageStatusCreating:
		log.Warn().Msgf("ObjectStorage (%s) is stuck in Creating. Verifying CSP status...", resourceId)
		return r.reconcileCreating(nsId, &osInfo, &statusResp)

	case model.StorageStatusDeleting:
		log.Warn().Msgf("ObjectStorage (%s) is stuck in Deleting. Re-triggering deletion logic...", resourceId)
		return r.reconcileDeleting(nsId, &osInfo, &statusResp)

	default:
		return model.SimpleMsg{}, fmt.Errorf("invalid resource status: %s", osInfo.Status)
	}
}

// reconcileAvailable reconciles an ObjectStorage resource in Available status.
func (r *ObjectStorageReconciler) reconcileAvailable(nsId string, osInfo *model.ObjectStorageInfo, statusResp *model.CspResourceStatusResponse) (model.SimpleMsg, error) {
	osKey := common.GenResourceKey(nsId, model.StrObjectStorage, osInfo.Id)
	syncState := resource.GetResourceSyncState(osInfo.CspResourceName, osInfo.CspResourceId, *statusResp)
	switch syncState {
	case model.SyncStateInSync:
		model.SetCondition(&osInfo.Conditions, model.ConditionSynced, model.ConditionTrue, model.ReasonAvailable, "Resource is in sync across all layers")
	case model.SyncStateSpMetaMissing:
		model.SetCondition(&osInfo.Conditions, model.ConditionSynced, model.ConditionFalse, string(syncState), "Spider metadata missing; TB metadata preserved")
	case model.SyncStateCspResourceMissing:
		model.SetCondition(&osInfo.Conditions, model.ConditionReady, model.ConditionFalse, string(syncState), "Resource missing on CSP provider")
		model.SetCondition(&osInfo.Conditions, model.ConditionSynced, model.ConditionFalse, string(syncState), "Resource missing on CSP provider")
		osInfo.SystemMessage = "Reconcile Diagnostic: CSP resource missing."
	case model.SyncStateTbMetaOnly:
		model.SetCondition(&osInfo.Conditions, model.ConditionReady, model.ConditionFalse, string(syncState), "Ghost metadata: resource absent on Spider and CSP")
		model.SetCondition(&osInfo.Conditions, model.ConditionSynced, model.ConditionFalse, string(syncState), "Ghost metadata: resource absent on Spider and CSP")
		osInfo.SystemMessage = "Reconcile Diagnostic: Ghost metadata detected."
	}
	osInfo.Status = model.DeriveObjectStorageStatus(osInfo.Conditions)

	val, err := json.Marshal(osInfo)
	if err != nil {
		return model.SimpleMsg{}, err
	}
	if putErr := kvstore.Put(osKey, string(val)); putErr != nil {
		return model.SimpleMsg{}, putErr
	}
	return model.SimpleMsg{Message: fmt.Sprintf("ObjectStorage (%s) reconciled", osInfo.Id)}, nil
}

// reconcileFailed handles self-healing for ObjectStorage in Failed status if CSP resource still exists.
func (r *ObjectStorageReconciler) reconcileFailed(nsId string, osInfo *model.ObjectStorageInfo, statusResp *model.CspResourceStatusResponse) (model.SimpleMsg, error) {
	osKey := common.GenResourceKey(nsId, model.StrObjectStorage, osInfo.Id)
	syncState := resource.GetResourceSyncState(osInfo.CspResourceName, osInfo.CspResourceId, *statusResp)

	// A user-owned deletion tombstone is sticky: never self-heal it back to Available.
	// The label lookup does I/O, so evaluate it only when a restore is on the table.
	restoreOk := (syncState == model.SyncStateInSync || syncState == model.SyncStateSpMetaMissing) &&
		model.ShouldRestoreToAvailable(osInfo.Conditions)
	if restoreOk {
		if cond := model.GetCondition(osInfo.Conditions, model.ConditionReady); cond != nil &&
			cond.Reason == model.ReasonDeletionFailed && !resource.IsAutoManagedResource(nsId, osInfo.Id, model.StrObjectStorage, osInfo.Uid) {
			restoreOk = false
		}
	}

	switch {
	case restoreOk:
		prevReason, prevMessage := "", ""
		if cond := model.GetCondition(osInfo.Conditions, model.ConditionReady); cond != nil {
			prevReason = cond.Reason
			prevMessage = cond.Message
		}
		log.Info().Msgf("ObjectStorage (%s) restored from %s to Available; CSP resource exists", osInfo.Id, prevReason)

		restoredMsg := fmt.Sprintf("Restored from %s; CSP resource exists", prevReason)
		if prevMessage != "" {
			restoredMsg = fmt.Sprintf("%s (previous failure: %s)", restoredMsg, prevMessage)
		}
		model.SetCondition(&osInfo.Conditions, model.ConditionReady, model.ConditionTrue, model.ReasonRestored, restoredMsg)
		model.SetCondition(&osInfo.Conditions, model.ConditionSynced, model.ConditionTrue, model.ReasonAvailable, "Resource is in sync across all layers")
		osInfo.SystemMessage = ""

	case syncState == model.SyncStateCspResourceMissing:
		model.SetCondition(&osInfo.Conditions, model.ConditionReady, model.ConditionFalse, string(syncState), "Resource missing on CSP provider")
		model.SetCondition(&osInfo.Conditions, model.ConditionSynced, model.ConditionFalse, string(syncState), "Resource missing on CSP provider")
		osInfo.SystemMessage = "Reconcile Diagnostic: CSP resource missing."

	case syncState == model.SyncStateTbMetaOnly:
		model.SetCondition(&osInfo.Conditions, model.ConditionReady, model.ConditionFalse, string(syncState), "Ghost metadata: resource absent on Spider and CSP")
		model.SetCondition(&osInfo.Conditions, model.ConditionSynced, model.ConditionFalse, string(syncState), "Ghost metadata: resource absent on Spider and CSP")
		osInfo.SystemMessage = "Reconcile Diagnostic: Ghost metadata detected."

	case syncState == model.SyncStateSpMetaMissing:
		// Not authorized to restore — record the diagnosis only; Ready/Status/SystemMessage stay untouched.
		model.SetCondition(&osInfo.Conditions, model.ConditionSynced, model.ConditionFalse, string(syncState), "Spider metadata missing; TB metadata preserved")

	default: // SyncStateInSync, not authorized to restore (sticky tombstone)
		model.SetCondition(&osInfo.Conditions, model.ConditionSynced, model.ConditionTrue, model.ReasonAvailable, "Resource is in sync across all layers")
	}
	osInfo.Status = model.DeriveObjectStorageStatus(osInfo.Conditions)

	val, err := json.Marshal(osInfo)
	if err != nil {
		return model.SimpleMsg{}, err
	}
	if putErr := kvstore.Put(osKey, string(val)); putErr != nil {
		return model.SimpleMsg{}, putErr
	}
	return model.SimpleMsg{Message: fmt.Sprintf("ObjectStorage (%s) reconciled", osInfo.Id)}, nil
}

// reconcileCreating handles stuck creation status for ObjectStorage (skeleton for future implementation).
// TODO: Implement creation recovery after detailed verification:
// 1. If resource exists on CSP -> promote status to Available.
// 2. If resource missing on CSP -> mark status as Failed (Reason: CreationFailed).
func (r *ObjectStorageReconciler) reconcileCreating(nsId string, osInfo *model.ObjectStorageInfo, statusResp *model.CspResourceStatusResponse) (model.SimpleMsg, error) {
	log.Info().Msgf("reconcileCreating called for ObjectStorage (%s); logic is under construction", osInfo.Id)
	return model.SimpleMsg{Message: fmt.Sprintf("ObjectStorage (%s) creation recovery logic is under construction (skeleton)", osInfo.Id)}, nil
}

// reconcileDeleting retries the fail-closed delete for an ObjectStorage stuck in Deleting:
// it purges the record if the CSP resource is now gone, or keeps it if still present.
func (r *ObjectStorageReconciler) reconcileDeleting(nsId string, osInfo *model.ObjectStorageInfo, statusResp *model.CspResourceStatusResponse) (model.SimpleMsg, error) {
	if err := resource.DeleteObjectStorage(nsId, osInfo.Id, false, false); err != nil {
		log.Warn().Err(err).Msgf("ObjectStorage (%s) deletion still unconfirmed; record retained for retry", osInfo.Id)
		return model.SimpleMsg{Message: fmt.Sprintf("ObjectStorage (%s) deletion retried; still present, retained", osInfo.Id)}, nil
	}
	return model.SimpleMsg{Message: fmt.Sprintf("ObjectStorage (%s) deletion completed (record purged)", osInfo.Id)}, nil
}

// ReconcileAll reconciles all ObjectStorages in the namespace.
func (r *ObjectStorageReconciler) ReconcileAll(ctx context.Context, nsId string, maxConcurrent int) (model.ResourceReconcileResults, error) {
	startTime := time.Now()
	log.Info().Msgf("ReconcileAll ObjectStorages started for namespace: %s (maxConcurrent: %d)", nsId, maxConcurrent)

	// 1. List all ObjectStorages in the namespace
	result, err := resource.ListResource(nsId, model.StrObjectStorage, "", "")
	if err != nil {
		return model.ResourceReconcileResults{}, fmt.Errorf("failed to list ObjectStorages: %w", err)
	}

	osList, ok := result.([]model.ObjectStorageInfo)
	if !ok {
		return model.ResourceReconcileResults{}, fmt.Errorf("unexpected type from ListResource: expected []model.ObjectStorageInfo")
	}

	if len(osList) == 0 {
		log.Info().Msg("No ObjectStorages found in namespace")
		return model.ResourceReconcileResults{
			Total:        0,
			SuccessCount: 0,
			FailedCount:  0,
			Results:      []model.ResourceReconcileResult{},
		}, nil
	}

	// 2. Group ObjectStorages by connection
	connectionGroups := make(map[string][]model.ObjectStorageInfo)
	for _, osItem := range osList {
		connectionGroups[osItem.ConnectionName] = append(connectionGroups[osItem.ConnectionName], osItem)
	}

	log.Info().Msgf("Grouped %d ObjectStorages across %d connections", len(osList), len(connectionGroups))
	log.Info().Msg("Starting pipeline: fetch status and reconcile per connection")

	// 3. Pipeline approach
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var results []model.ResourceReconcileResult
	var reconciledCount int32
	var fetchedConnCount int32
	totalConnections := int32(len(connectionGroups))

	for connName, storages := range connectionGroups {
		wg.Add(1)
		go func(conn string, osItems []model.ObjectStorageInfo) {
			defer wg.Done()
			connStartTime := time.Now()

			log.Info().Msgf("[%s] Fetching ObjectStorage status (%d items)...", conn, len(osItems))
			fetchStartTime := time.Now()
			status, fetchErr := resource.GetCspResourceStatus(conn, model.StrObjectStorage)
			fetchElapsed := time.Since(fetchStartTime).Seconds()
			completed := atomic.AddInt32(&fetchedConnCount, 1)
			log.Info().Msgf("[%s] Status fetch complete (%d/%d connections, %.2fs)", conn, completed, totalConnections, fetchElapsed)

			if fetchErr != nil {
				log.Warn().Err(fetchErr).Msgf("[%s] Failed to fetch ObjectStorage status; skipping %d items", conn, len(osItems))
				mu.Lock()
				for _, osItem := range osItems {
					fetchElapsedRounded := roundTo2Decimals(fetchElapsed)
					results = append(results, model.ResourceReconcileResult{
						ResourceType:   model.StrObjectStorage,
						ResourceId:     osItem.Id,
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

			log.Info().Msgf("[%s] Starting reconciliation for %d ObjectStorages...", conn, len(osItems))
			var connWg sync.WaitGroup
			for _, osItem := range osItems {
				connWg.Add(1)
				go func(item model.ObjectStorageInfo) {
					defer connWg.Done()
					itemStartTime := time.Now()

					sem <- struct{}{}
					defer func() { <-sem }()

					select {
					case <-ctx.Done():
						cancelElapsed := roundTo2Decimals(time.Since(itemStartTime).Seconds())
						mu.Lock()
						results = append(results, model.ResourceReconcileResult{
							ResourceType:   model.StrObjectStorage,
							ResourceId:     item.Id,
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

					resp, recErr := r.Reconcile(ctx, nsId, item.Id, &status)
					itemElapsed := roundTo2Decimals(time.Since(itemStartTime).Seconds())

					recResult := model.ResourceReconcileResult{
						ResourceType:   model.StrObjectStorage,
						ResourceId:     item.Id,
						ConnectionName: conn,
						Success:        recErr == nil,
						ElapsedSeconds: itemElapsed,
						Elapsed:        formatDuration(itemElapsed),
					}

					if recErr != nil {
						recResult.Error = recErr.Error()
						log.Warn().Err(recErr).Msgf("[%s] Failed to reconcile ObjectStorage: %s (%.2fs)", conn, item.Id, itemElapsed)
					} else if msg, ok := resp.(model.SimpleMsg); ok {
						recResult.Message = msg.Message
						log.Debug().Msgf("[%s] Reconciled ObjectStorage %s: %s (%.2fs)", conn, item.Id, msg.Message, itemElapsed)
					}

					mu.Lock()
					results = append(results, recResult)
					mu.Unlock()

					doneCount := atomic.AddInt32(&reconciledCount, 1)
					if len(osItems) > 10 && (doneCount%10 == 0 || doneCount == int32(len(osItems))) {
						log.Info().Msgf("Reconciliation progress: %d/%d ObjectStorages complete", doneCount, len(osItems))
					}
				}(osItem)
			}

			connWg.Wait()
			connElapsed := time.Since(connStartTime).Seconds()
			log.Info().Msgf("[%s] Connection reconciliation complete (%.2fs total)", conn, connElapsed)
		}(connName, storages)
	}

	wg.Wait()

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

	log.Info().Msgf("ReconcileAll ObjectStorages completed for namespace %s in %s: %d total, %d success, %d failed",
		nsId, formatDuration(totalElapsed), response.Total, response.SuccessCount, response.FailedCount)

	return response, nil
}
