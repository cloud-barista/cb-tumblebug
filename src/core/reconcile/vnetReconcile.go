package reconcile

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/rs/zerolog/log"
)

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
		log.Debug().Msgf("[Request to Spider] Listing all VPCs for connection: %s", vNetInfo.ConnectionName)
		var fetchErr error
		vpcStatusResp, fetchErr = resource.GetCspResourceStatus(vNetInfo.ConnectionName, model.StrVNet)
		if fetchErr != nil {
			log.Error().Err(fetchErr).Msg("failed to get VPC resource status from Spider, skipping reconciliation")
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
