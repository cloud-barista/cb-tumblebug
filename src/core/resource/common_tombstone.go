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

package resource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/secret"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/rs/zerolog/log"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// presentOnCspAfterDelete is the post-DELETE purge gate. vNets use the KT-aware check
// (KT's VPC is the undeletable account default network); async CSP deletions (e.g. NCP VPC)
// are polled for a while before the record is retained as "still exists".
func presentOnCspAfterDelete(connName, resourceType, cspId, cspName string, vNetInfo *model.VNetInfo) (bool, error) {
	check := func() (bool, error) {
		if resourceType == model.StrVNet && vNetInfo != nil {
			return VNetPresentOnCsp(*vNetInfo)
		}
		return ResourcePresentOnCsp(connName, resourceType, cspId, cspName)
	}
	var present bool
	var err error
	for attempt := 0; attempt < deleteGatePollAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(deleteGatePollInterval)
		}
		present, err = check()
		if err == nil && !present {
			return false, nil
		}
	}
	return present, err
}

// deleteGatePollAttempts/Interval bound how long the purge gate waits for an asynchronous CSP deletion.
const (
	deleteGatePollAttempts = 10
	deleteGatePollInterval = 6 * time.Second
)

// cleanupLocalResourceRecord removes CB-TB's own local record for a resource
// (kvstore entry, or DB row for DB-backed types like CustomImage), plus its
// label object and any child records (e.g. VNet's subnets). Used both after a
// Spider-confirmed successful DELETE and when Spider/CSP have confirmed the
// resource is already gone (see the "does not exist in connection" handling
// above), so both paths clean up the exact same backing stores.
func cleanupLocalResourceRecord(nsId, resourceType, resourceId, key, uid string, childResources any) error {
	if strings.EqualFold(resourceType, model.StrVNet) {
		subnets := childResources.([]model.SubnetInfo)
		for _, v := range subnets {
			subnetKey := common.GenChildResourceKey(nsId, model.StrSubnet, resourceId, v.Id)
			if err := kvstore.Delete(subnetKey); err != nil {
				log.Error().Err(err).Msg("")
			}
			if err := label.DeleteLabelObject(resourceType, v.Uid); err != nil {
				log.Error().Err(err).Msg("")
			}
		}
	} else if strings.EqualFold(resourceType, model.StrCustomImage) {
		// Delete custom image from database
		result := model.ORM.Delete(&model.ImageInfo{}, "namespace = ? AND id = ? AND resource_type = ?",
			nsId, resourceId, model.StrCustomImage)
		if result.Error != nil {
			fmt.Println(result.Error.Error())
		} else {
			log.Debug().Msg("Custom image deleted successfully from database")
			imageInfoCache.Delete(strings.ToLower(nsId) + "/" + strings.ToLower(resourceId))
		}
	}

	// Delete from kvstore for backward compatibility (only for non-DB resources)
	if !strings.EqualFold(resourceType, model.StrImage) &&
		!strings.EqualFold(resourceType, model.StrCustomImage) &&
		!strings.EqualFold(resourceType, model.StrSpec) {
		if err := kvstore.Delete(key); err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
		if strings.EqualFold(resourceType, model.StrSSHKey) {
			_ = secret.DeleteSshKey(context.Background(), nsId, resourceId)
		}
	}

	if err := label.DeleteLabelObject(resourceType, uid); err != nil {
		log.Error().Err(err).Msg("")
	}

	return nil
}

// Sentinel errors so the REST layer can map deletion outcomes to HTTP codes
// (in-progress → 202, unconfirmed/conflict → 409) via errors.Is
var (
	ErrDeletionInProgress    = errors.New("deletion in progress")
	ErrDeletionUnconfirmed   = errors.New("deletion unconfirmed; record retained")
	ErrTombstoneNameConflict = errors.New("name held by unconfirmed deletion")
	ErrNoPendingDeletion     = errors.New("no pending deletion")
)

// deletionInFlight dedups concurrent deletions per resource; in-memory only so a
// persisted Deleting status stays retryable after a crash
var deletionInFlight sync.Map

var (
	deleteVerifyPollAttempts = DefaultPollMaxAttempts
	deleteVerifyPollInterval = DefaultPollInterval
	// Within this window a still-visible resource stays Deleting (CSP async deletion)
	deleteFailGraceWindow = 2 * time.Minute
)

// errIfNameHeldByTombstone returns ErrTombstoneNameConflict when a deletion
// tombstone still holds the name (kvstore-backed types)
func errIfNameHeldByTombstone(nsId, resourceType, name string) error {
	kv, exists, _ := kvstore.GetKv(common.GenResourceKey(nsId, resourceType, name))
	if !exists || gjson.Get(kv.Value, "deletionRequestedAt").String() == "" {
		return nil
	}
	return fmt.Errorf("previous deletion of %s '%s' is unconfirmed (status=%s); retry DELETE to complete it, or DELETE with force to discard the record (%w)",
		resourceType, name, gjson.Get(kv.Value, "status").String(), ErrTombstoneNameConflict)
}

// tombstoneSupported reports whether the type uses fail-closed tombstone deletion
func tombstoneSupported(resourceType string) bool {
	switch resourceType {
	case model.StrDataDisk, model.StrSSHKey, model.StrSecurityGroup, model.StrCustomImage, model.StrVNet:
		return true
	}
	return false
}

// IsAutoManagedResource reports whether a resource is auto-managed: created on demand for
// provisioning and released when unused (deletion means "release if unused" — restorable),
// versus user-owned (deletion is sticky). Detected by the shared-name prefix or purpose label.
func IsAutoManagedResource(nsId, resourceId, labelType, uid string) bool {
	if strings.HasPrefix(resourceId, nsId+model.StrSharedResourceName) {
		return true
	}
	if labelType == "" || uid == "" {
		return false
	}
	labelInfo, err := label.GetLabels(labelType, uid)
	if err != nil {
		log.Warn().Err(err).Msgf("label lookup failed for %s/%s; treating %s as user-owned", labelType, uid, resourceId)
		return false
	}
	return labelInfo.Labels[model.LabelPurpose] == model.PurposeInfraDynamic
}

// tombstoneStatus returns the per-type status vocabulary for a tombstone state
func tombstoneStatus(resourceType string, failed bool) string {
	if failed {
		return model.ResourceStatusFailed
	}
	return model.ResourceStatusDeleting
}

// deletionSlotKey is the mutual-exclusion key shared by delete and restore
func deletionSlotKey(nsId, resourceType, resourceId string) string {
	return nsId + "/" + resourceType + "/" + resourceId
}

// patchTombstoneFields updates only the tombstone fields on the raw stored JSON, so
// fields unknown to this code path are never dropped
func patchTombstoneFields(val, status, message string) string {
	val, _ = sjson.Set(val, "status", status)
	val, _ = sjson.Set(val, "systemMessage", message)
	if gjson.Get(val, "deletionRequestedAt").String() == "" {
		val, _ = sjson.Set(val, "deletionRequestedAt", time.Now().UTC().Format(time.RFC3339))
	}
	return val
}

// MarkTombstoneByKey marks a kvstore record (by raw key) as a deletion tombstone;
// for records whose key is not GenResourceKey-shaped (e.g. NLB under its Infra)
func MarkTombstoneByKey(key, status, message string) error {
	kv, exists, err := kvstore.GetKv(key)
	if err != nil || !exists {
		return fmt.Errorf("cannot load record %s: %v", key, err)
	}
	return kvstore.Put(key, patchTombstoneFields(kv.Value, status, message))
}

func patchKvTombstone(nsId, resourceType, resourceId, status, reason, message string) error {
	key := common.GenResourceKey(nsId, resourceType, resourceId)
	kv, exists, err := kvstore.GetKv(key)
	if err != nil || !exists {
		return fmt.Errorf("cannot load %s '%s' record: %v", resourceType, resourceId, err)
	}
	val := patchTombstoneFields(kv.Value, status, message)
	var conds []model.Condition
	if raw := gjson.Get(val, "conditions").Raw; raw != "" {
		json.Unmarshal([]byte(raw), &conds)
	}
	model.SetCondition(&conds, model.ConditionReady, model.ConditionFalse, reason, message)
	if b, err := json.Marshal(conds); err == nil {
		val, _ = sjson.SetRaw(val, "conditions", string(b))
	}
	return kvstore.Put(key, val)
}

// clearKvTombstone cancels a deletion tombstone: status back to Available, deletionRequestedAt
// cleared, Ready condition restored. Used by RestoreResource once the CSP resource is confirmed present.
func clearKvTombstone(nsId, resourceType, resourceId string) error {
	key := common.GenResourceKey(nsId, resourceType, resourceId)
	kv, exists, err := kvstore.GetKv(key)
	if err != nil || !exists {
		return fmt.Errorf("cannot load %s '%s' record: %v", resourceType, resourceId, err)
	}
	val := kv.Value
	val, _ = sjson.Set(val, "status", model.ResourceStatusAvailable)
	val, _ = sjson.Set(val, "systemMessage", "")
	val, _ = sjson.Set(val, "deletionRequestedAt", "")
	var conds []model.Condition
	if raw := gjson.Get(val, "conditions").Raw; raw != "" {
		json.Unmarshal([]byte(raw), &conds)
	}
	model.SetCondition(&conds, model.ConditionReady, model.ConditionTrue, model.ReasonRestored, "Deletion cancelled by user; CSP resource confirmed present")
	if b, err := json.Marshal(conds); err == nil {
		val, _ = sjson.SetRaw(val, "conditions", string(b))
	}
	return kvstore.Put(key, val)
}

// RestoreResource cancels a deletion tombstone and returns the resource to Available, but
// only when the CSP resource is confirmed to still exist — so a deletion mistakenly issued
// (or blocked by a live dependency) can be undone without resurrecting a ghost record.
func RestoreResource(nsId, resourceType, resourceId string) error {
	key := common.GenResourceKey(nsId, resourceType, resourceId)
	kv, exists, err := kvstore.GetKv(key)
	if err != nil || !exists {
		return fmt.Errorf("%s '%s' not found", resourceType, resourceId)
	}
	// Only a deletion tombstone is restorable; anything else would be forced to
	// Available. Some producers (e.g. vNet's own delete path) mark tombstones via
	// conditions only, so the Ready-condition reason is accepted as evidence too.
	isTombstone := gjson.Get(kv.Value, "deletionRequestedAt").String() != ""
	if !isTombstone {
		for _, c := range gjson.Get(kv.Value, "conditions").Array() {
			if c.Get("type").String() == string(model.ConditionReady) {
				r := c.Get("reason").String()
				isTombstone = r == model.ReasonDeleting || r == model.ReasonDeletionFailed
			}
		}
	}
	if !isTombstone {
		return fmt.Errorf("%s '%s' has no pending deletion to restore from (status=%s) (%w)",
			resourceType, resourceId, gjson.Get(kv.Value, "status").String(), ErrNoPendingDeletion)
	}
	// Claim the same slot as deletion so a restore cannot interleave with an in-flight
	// generic-path delete. Own-path deletes (vNet/NLB/RDBMS) do not claim this slot yet.
	inflightKey := deletionSlotKey(nsId, resourceType, resourceId)
	if _, busy := deletionInFlight.LoadOrStore(inflightKey, struct{}{}); busy {
		return fmt.Errorf("cannot restore %s '%s' while its deletion is in progress (%w)", resourceType, resourceId, ErrDeletionInProgress)
	}
	defer deletionInFlight.Delete(inflightKey)

	connName := gjson.Get(kv.Value, "connectionName").String()
	cspId := gjson.Get(kv.Value, "cspResourceId").String()
	cspName := gjson.Get(kv.Value, "cspResourceName").String()
	present, gateErr := ResourcePresentOnCsp(connName, resourceType, cspId, cspName)
	if gateErr != nil {
		return fmt.Errorf("cannot restore %s '%s': CSP existence check failed: %w", resourceType, resourceId, gateErr)
	}
	if !present {
		return fmt.Errorf("cannot restore %s '%s': it is not present on the CSP — purge the record instead", resourceType, resourceId)
	}
	return clearKvTombstone(nsId, resourceType, resourceId)
}

// patchCustomImageTombstone is the DB-backed equivalent for customImage rows
func patchCustomImageTombstone(nsId, resourceId, status, message string) error {
	var row model.ImageInfo
	if result := model.ORM.Where("namespace = ? AND id = ? AND resource_type = ?",
		nsId, resourceId, model.StrCustomImage).First(&row); result.Error != nil {
		return result.Error
	}
	updates := map[string]any{"image_status": status, "system_message": message}
	if row.DeletionRequestedAt == "" {
		updates["deletion_requested_at"] = time.Now().UTC().Format(time.RFC3339)
	}
	result := model.ORM.Model(&model.ImageInfo{}).Where("namespace = ? AND id = ? AND resource_type = ?",
		nsId, resourceId, model.StrCustomImage).Updates(updates)
	imageInfoCache.Delete(strings.ToLower(nsId) + "/" + strings.ToLower(resourceId))
	return result.Error
}

// markVNetSubnetsDeleting patches a Deleting condition onto the vNet's embedded
// SubnetInfoList (what GetVNet/list derive subnet status from), so the bulk/release
// delete path surfaces the same subnet Deleting state as the dedicated DeleteSubnet
// path. Best-effort. Consolidate with DeleteSubnet if/when the dedicated and generic
// delete lines are unified.
func markVNetSubnetsDeleting(nsId, vNetId string) {
	key := common.GenResourceKey(nsId, model.StrVNet, vNetId)
	kv, exists, err := kvstore.GetKv(key)
	if err != nil || !exists {
		return
	}
	var v model.VNetInfo
	if err := json.Unmarshal([]byte(kv.Value), &v); err != nil || len(v.SubnetInfoList) == 0 {
		return
	}
	for i := range v.SubnetInfoList {
		model.SetCondition(&v.SubnetInfoList[i].Conditions, model.ConditionReady, model.ConditionFalse,
			model.ReasonDeleting, "Subnet deletion in progress")
		v.SubnetInfoList[i].Status = model.DeriveStatus(model.StrSubnet, v.SubnetInfoList[i].Conditions)
	}
	if b, err := json.Marshal(v); err == nil {
		if err := kvstore.Put(key, string(b)); err != nil {
			log.Warn().Err(err).Msgf("failed to mark subnets of vNet '%s' as Deleting", vNetId)
		}
	}
}

// markResourceDeleting persists the tombstone before Spider is called; the original
// request time is kept across retries
func markResourceDeleting(nsId, resourceType, resourceId string) error {
	if resourceType == model.StrCustomImage {
		return patchCustomImageTombstone(nsId, resourceId, string(model.ImageDeleting), "")
	}
	return patchKvTombstone(nsId, resourceType, resourceId,
		tombstoneStatus(resourceType, false), model.ReasonDeleting, "deletion in progress")
}

// markResourceDeleteFailed keeps the record visible as Failed; retrying DELETE resumes
func markResourceDeleteFailed(nsId, resourceType, resourceId string, cause error) {
	var err error
	if resourceType == model.StrCustomImage {
		err = patchCustomImageTombstone(nsId, resourceId, string(model.ImageFailed), cause.Error())
	} else {
		err = patchKvTombstone(nsId, resourceType, resourceId,
			tombstoneStatus(resourceType, true), model.ReasonDeletionFailed, cause.Error())
	}
	if err != nil {
		log.Error().Err(err).Msgf("Failed to mark %s '%s' as DeletionFailed", resourceType, resourceId)
	}
}

// tombstoneRequestedAt returns the stored deletionRequestedAt ("" if none)
func tombstoneRequestedAt(nsId, resourceType, resourceId string) string {
	if resourceType == model.StrCustomImage {
		var row model.ImageInfo
		if result := model.ORM.Where("namespace = ? AND id = ? AND resource_type = ?",
			nsId, resourceId, model.StrCustomImage).First(&row); result.Error == nil {
			return row.DeletionRequestedAt
		}
		return ""
	}
	kv, exists, err := kvstore.GetKv(common.GenResourceKey(nsId, resourceType, resourceId))
	if err != nil || !exists {
		return ""
	}
	return gjson.Get(kv.Value, "deletionRequestedAt").String()
}

// markResourceStillOnCsp keeps the record Deleting within the grace window
// (slow async CSP deletion), Failed after it; reports which state was chosen
func markResourceStillOnCsp(nsId, resourceType, resourceId string, cause error) (stillDeleting bool) {
	requestedAt, parseErr := time.Parse(time.RFC3339, tombstoneRequestedAt(nsId, resourceType, resourceId))
	if parseErr == nil && time.Since(requestedAt) <= deleteFailGraceWindow {
		var err error
		if resourceType == model.StrCustomImage {
			err = patchCustomImageTombstone(nsId, resourceId, string(model.ImageDeleting), cause.Error())
		} else {
			err = patchKvTombstone(nsId, resourceType, resourceId,
				tombstoneStatus(resourceType, false), model.ReasonDeleting, cause.Error())
		}
		if err != nil {
			log.Error().Err(err).Msgf("Failed to mark %s '%s' deletion outcome", resourceType, resourceId)
		}
		return true
	}
	markResourceDeleteFailed(nsId, resourceType, resourceId, cause)
	return false
}
