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

// RDBMSReconciler implements the Reconciler interface for RDBMS resources.
type RDBMSReconciler struct{}

func init() {
	GetManager().RegisterReconciler(model.StrRDBMS, &RDBMSReconciler{})
}

// Reconcile performs diagnosis and self-healing for RDBMS resources.
func (r *RDBMSReconciler) Reconcile(ctx context.Context, nsId string, resourceId string, optPreloadedStatus *model.CspResourceStatusResponse) (any, error) {
	log.Info().Msgf("Reconcile started for RDBMS: %s/%s", nsId, resourceId)

	// 1. Retrieve Expected State from DB
	rdbmsKey := common.GenResourceKey(nsId, model.StrRDBMS, resourceId)
	keyValue, exists, err := kvstore.GetKv(rdbmsKey)
	if err != nil {
		return nil, fmt.Errorf("failed to read RDBMS from DB: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("does not exist, RDBMS: %s", resourceId)
	}

	var rdbmsInfo model.RDBMSInfo
	if err := json.Unmarshal([]byte(keyValue.Value), &rdbmsInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal RDBMS info: %w", err)
	}

	// 2. Resolve CSP status once
	var statusResp model.CspResourceStatusResponse
	if optPreloadedStatus != nil {
		statusResp = *optPreloadedStatus
		log.Debug().Msgf("Using preloaded RDBMS status (connection: %s)", rdbmsInfo.ConnectionName)
	} else {
		log.Debug().Msgf("[Request to Spider] Listing all RDBMS for connection: %s", rdbmsInfo.ConnectionName)
		var fetchErr error
		statusResp, fetchErr = resource.GetCspResourceStatus(rdbmsInfo.ConnectionName, model.StrRDBMS)
		if fetchErr != nil {
			log.Error().Err(fetchErr).Msg("failed to get RDBMS status from Spider, skipping reconciliation")
			return model.SimpleMsg{}, fmt.Errorf("failed to reconcile RDBMS '%s': %w", resourceId, fetchErr)
		}
	}

	// 3. State Machine Handling based on Current DB Status
	switch rdbmsInfo.Status {
	case model.StorageStatusAvailable:
		return r.reconcileAvailable(nsId, &rdbmsInfo, &statusResp)

	case model.StorageStatusFailed:
		return r.reconcileFailed(nsId, &rdbmsInfo, &statusResp)

	case model.StorageStatusCreating:
		log.Warn().Msgf("RDBMS (%s) is stuck in Creating. Verifying CSP status...", resourceId)
		return r.reconcileCreating(nsId, &rdbmsInfo, &statusResp)

	case model.StorageStatusDeleting:
		log.Warn().Msgf("RDBMS (%s) is stuck in Deleting. Re-triggering deletion logic...", resourceId)
		return r.reconcileDeleting(nsId, &rdbmsInfo, &statusResp)

	default:
		return model.SimpleMsg{}, fmt.Errorf("invalid resource status: %s", rdbmsInfo.Status)
	}
}

// reconcileAvailable reconciles an RDBMS resource in Available status.
func (r *RDBMSReconciler) reconcileAvailable(nsId string, rdbmsInfo *model.RDBMSInfo, statusResp *model.CspResourceStatusResponse) (model.SimpleMsg, error) {
	rdbmsKey := common.GenResourceKey(nsId, model.StrRDBMS, rdbmsInfo.Id)
	syncState := resource.GetResourceSyncState(rdbmsInfo.CspResourceName, rdbmsInfo.CspResourceId, *statusResp)

	// Never fold SpMetaMissing into InSync — ApplySyncState leaves Status untouched for it, so this stays accurate.
	resource.ApplySyncState(&rdbmsInfo.Conditions, &rdbmsInfo.Status, &rdbmsInfo.SystemMessage, syncState)

	val, err := json.Marshal(rdbmsInfo)
	if err != nil {
		return model.SimpleMsg{}, err
	}
	if putErr := kvstore.Put(rdbmsKey, string(val)); putErr != nil {
		return model.SimpleMsg{}, putErr
	}
	return model.SimpleMsg{Message: fmt.Sprintf("RDBMS (%s) reconciled", rdbmsInfo.Id)}, nil
}

// reconcileFailed handles self-healing for RDBMS in Failed status if CSP resource still exists.
func (r *RDBMSReconciler) reconcileFailed(nsId string, rdbmsInfo *model.RDBMSInfo, statusResp *model.CspResourceStatusResponse) (model.SimpleMsg, error) {
	rdbmsKey := common.GenResourceKey(nsId, model.StrRDBMS, rdbmsInfo.Id)
	syncState := resource.GetResourceSyncState(rdbmsInfo.CspResourceName, rdbmsInfo.CspResourceId, *statusResp)

	// A user-owned deletion tombstone is sticky: never self-heal it back to Available.
	// The label lookup does I/O, so evaluate it only when a restore is on the table.
	restoreOk := (syncState == model.SyncStateInSync || syncState == model.SyncStateSpMetaMissing) &&
		model.ShouldRestoreToAvailable(rdbmsInfo.Conditions)
	if restoreOk {
		if cond := model.GetCondition(rdbmsInfo.Conditions, model.ConditionReady); cond != nil &&
			cond.Reason == model.ReasonDeletionFailed && !resource.IsAutoManagedResource(nsId, rdbmsInfo.Id, model.StrRDBMS, rdbmsInfo.Uid) {
			restoreOk = false
		}
	}

	if restoreOk {
		prevReason, prevMessage := "", ""
		if cond := model.GetCondition(rdbmsInfo.Conditions, model.ConditionReady); cond != nil {
			prevReason = cond.Reason
			prevMessage = cond.Message
		}
		log.Info().Msgf("RDBMS (%s) restored from %s to Available; CSP resource exists", rdbmsInfo.Id, prevReason)

		restoredMsg := fmt.Sprintf("Restored from %s; CSP resource exists", prevReason)
		if prevMessage != "" {
			restoredMsg = fmt.Sprintf("%s (previous failure: %s)", restoredMsg, prevMessage)
		}
		model.SetCondition(&rdbmsInfo.Conditions, model.ConditionReady, model.ConditionTrue, model.ReasonRestored, restoredMsg)
		// Record Synced from the real syncState, not a hardcoded "Synchronized with CSP" — keeps SpMetaMissing visible during restore.
		resource.ApplySyncState(&rdbmsInfo.Conditions, &rdbmsInfo.Status, &rdbmsInfo.SystemMessage, syncState)
		// ApplySyncState won't set Status for SpMetaMissing, but CSP is confirmed alive here, so force Available.
		rdbmsInfo.Status = model.StorageStatusAvailable
	} else {
		resource.ApplySyncState(&rdbmsInfo.Conditions, &rdbmsInfo.Status, &rdbmsInfo.SystemMessage, syncState)
	}

	val, err := json.Marshal(rdbmsInfo)
	if err != nil {
		return model.SimpleMsg{}, err
	}
	if putErr := kvstore.Put(rdbmsKey, string(val)); putErr != nil {
		return model.SimpleMsg{}, putErr
	}
	return model.SimpleMsg{Message: fmt.Sprintf("RDBMS (%s) reconciled", rdbmsInfo.Id)}, nil
}

// reconcileCreating handles stuck creation status for RDBMS (skeleton for future implementation).
func (r *RDBMSReconciler) reconcileCreating(nsId string, rdbmsInfo *model.RDBMSInfo, statusResp *model.CspResourceStatusResponse) (model.SimpleMsg, error) {
	log.Info().Msgf("reconcileCreating called for RDBMS (%s); logic is under construction", rdbmsInfo.Id)
	return model.SimpleMsg{Message: fmt.Sprintf("RDBMS (%s) creation recovery logic is under construction (skeleton)", rdbmsInfo.Id)}, nil
}

// reconcileDeleting retries the fail-closed delete for an RDBMS stuck in Deleting: it purges
// the record if the CSP resource is now gone, or keeps it if still present.
func (r *RDBMSReconciler) reconcileDeleting(nsId string, rdbmsInfo *model.RDBMSInfo, statusResp *model.CspResourceStatusResponse) (model.SimpleMsg, error) {
	if err := resource.DeleteRDBMS(nsId, rdbmsInfo.Id, false); err != nil {
		log.Warn().Err(err).Msgf("RDBMS (%s) deletion still unconfirmed; record retained for retry", rdbmsInfo.Id)
		return model.SimpleMsg{Message: fmt.Sprintf("RDBMS (%s) deletion retried; still present, retained", rdbmsInfo.Id)}, nil
	}
	return model.SimpleMsg{Message: fmt.Sprintf("RDBMS (%s) deletion completed (record purged)", rdbmsInfo.Id)}, nil
}

// ReconcileAll reconciles all RDBMS instances in the namespace.
func (r *RDBMSReconciler) ReconcileAll(ctx context.Context, nsId string, maxConcurrent int) (model.ResourceReconcileResults, error) {
	startTime := time.Now()
	log.Info().Msgf("ReconcileAll RDBMS started for namespace: %s (maxConcurrent: %d)", nsId, maxConcurrent)

	// 1. List all RDBMS instances in the namespace
	result, err := resource.ListResource(nsId, model.StrRDBMS, "", "")
	if err != nil {
		return model.ResourceReconcileResults{}, fmt.Errorf("failed to list RDBMS: %w", err)
	}

	rdbmsList, ok := result.([]model.RDBMSInfo)
	if !ok {
		return model.ResourceReconcileResults{}, fmt.Errorf("unexpected type from ListResource: expected []model.RDBMSInfo")
	}

	if len(rdbmsList) == 0 {
		log.Info().Msg("No RDBMS instances found in namespace")
		return model.ResourceReconcileResults{
			Total:        0,
			SuccessCount: 0,
			FailedCount:  0,
			Results:      []model.ResourceReconcileResult{},
		}, nil
	}

	// 2. Group RDBMS instances by connection
	connectionGroups := make(map[string][]model.RDBMSInfo)
	for _, rdbmsItem := range rdbmsList {
		connectionGroups[rdbmsItem.ConnectionName] = append(connectionGroups[rdbmsItem.ConnectionName], rdbmsItem)
	}

	log.Info().Msgf("Grouped %d RDBMS instances across %d connections", len(rdbmsList), len(connectionGroups))
	log.Info().Msg("Starting pipeline: fetch status and reconcile per connection")

	// 3. Pipeline approach
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var results []model.ResourceReconcileResult
	var reconciledCount int32
	var fetchedConnCount int32
	totalConnections := int32(len(connectionGroups))

	for connName, instances := range connectionGroups {
		wg.Add(1)
		go func(conn string, rdbmsItems []model.RDBMSInfo) {
			defer wg.Done()
			connStartTime := time.Now()

			log.Info().Msgf("[%s] Fetching RDBMS status (%d items)...", conn, len(rdbmsItems))
			fetchStartTime := time.Now()
			status, fetchErr := resource.GetCspResourceStatus(conn, model.StrRDBMS)
			fetchElapsed := time.Since(fetchStartTime).Seconds()
			completed := atomic.AddInt32(&fetchedConnCount, 1)
			log.Info().Msgf("[%s] Status fetch complete (%d/%d connections, %.2fs)", conn, completed, totalConnections, fetchElapsed)

			if fetchErr != nil {
				log.Warn().Err(fetchErr).Msgf("[%s] Failed to fetch RDBMS status; skipping %d items", conn, len(rdbmsItems))
				mu.Lock()
				for _, rdbmsItem := range rdbmsItems {
					fetchElapsedRounded := roundTo2Decimals(fetchElapsed)
					results = append(results, model.ResourceReconcileResult{
						ResourceType:   model.StrRDBMS,
						ResourceId:     rdbmsItem.Id,
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

			log.Info().Msgf("[%s] Starting reconciliation for %d RDBMS instances...", conn, len(rdbmsItems))
			var connWg sync.WaitGroup
			for _, rdbmsItem := range rdbmsItems {
				connWg.Add(1)
				go func(item model.RDBMSInfo) {
					defer connWg.Done()
					itemStartTime := time.Now()

					sem <- struct{}{}
					defer func() { <-sem }()

					select {
					case <-ctx.Done():
						cancelElapsed := roundTo2Decimals(time.Since(itemStartTime).Seconds())
						mu.Lock()
						results = append(results, model.ResourceReconcileResult{
							ResourceType:   model.StrRDBMS,
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
						ResourceType:   model.StrRDBMS,
						ResourceId:     item.Id,
						ConnectionName: conn,
						Success:        recErr == nil,
						ElapsedSeconds: itemElapsed,
						Elapsed:        formatDuration(itemElapsed),
					}

					if recErr != nil {
						recResult.Error = recErr.Error()
						log.Warn().Err(recErr).Msgf("[%s] Failed to reconcile RDBMS: %s (%.2fs)", conn, item.Id, itemElapsed)
					} else if msg, ok := resp.(model.SimpleMsg); ok {
						recResult.Message = msg.Message
						log.Debug().Msgf("[%s] Reconciled RDBMS %s: %s (%.2fs)", conn, item.Id, msg.Message, itemElapsed)
					}

					mu.Lock()
					results = append(results, recResult)
					mu.Unlock()

					doneCount := atomic.AddInt32(&reconciledCount, 1)
					if len(rdbmsItems) > 10 && (doneCount%10 == 0 || doneCount == int32(len(rdbmsItems))) {
						log.Info().Msgf("Reconciliation progress: %d/%d RDBMS instances complete", doneCount, len(rdbmsItems))
					}
				}(rdbmsItem)
			}

			connWg.Wait()
			connElapsed := time.Since(connStartTime).Seconds()
			log.Info().Msgf("[%s] Connection reconciliation complete (%.2fs total)", conn, connElapsed)
		}(connName, instances)
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

	log.Info().Msgf("ReconcileAll RDBMS completed for namespace %s in %s: %d total, %d success, %d failed",
		nsId, formatDuration(totalElapsed), response.Total, response.SuccessCount, response.FailedCount)

	return response, nil
}
