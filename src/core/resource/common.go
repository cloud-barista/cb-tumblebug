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
	"hash/fnv"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/apierr"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
	directcsp "github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/secret"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvutil"

	"github.com/tidwall/sjson"
	gorm "gorm.io/gorm"

	"reflect"

	validator "github.com/go-playground/validator/v10"

	"github.com/rs/zerolog/log"
)

// use a single instance of Validate, it caches struct info
var validate *validator.Validate

// getResourceConnectionName extracts the connection name for a given resource.
// This function is used to group resources by their CSP connection for semaphore-based processing.
func getResourceConnectionName(nsId, resourceType, resourceId string) (string, error) {
	// Resolve the real connection name from the resource record. Do NOT guess it from the
	// resourceId: connection names themselves contain hyphens (e.g. "aws-ap-northeast-2")
	// and IDs may be prefixed by an infra/namespace name, so a "{connectionName}-..." split
	// mis-groups resources. `parts` is kept only as a last-resort fallback if the lookups fail.
	parts := strings.Split(resourceId, "-")

	// For Image, CustomImage, and Spec, use PostgreSQL (GORM)
	switch resourceType {
	case model.StrImage, model.StrCustomImage:
		var resource model.ImageInfo
		var result *gorm.DB

		if resourceType == model.StrImage {
			result = model.ORM.Select("connection_name").Where("namespace = ? AND id = ? AND (resource_type = ? OR resource_type IS NULL OR resource_type = '')",
				nsId, resourceId, model.StrImage).First(&resource)
		} else {
			result = model.ORM.Select("connection_name").Where("namespace = ? AND id = ? AND resource_type = ?",
				nsId, resourceId, model.StrCustomImage).First(&resource)
		}

		if result.Error == nil {
			return resource.ConnectionName, nil
		}
		// If DB lookup fails, use pattern-based fallback
		if len(parts) >= 2 {
			return parts[0], nil
		}
		return "unknown", result.Error

	case model.StrSpec:
		var resource model.SpecInfo
		result := model.ORM.Select("connection_name").Where("namespace = ? AND id = ?", nsId, resourceId).First(&resource)
		if result.Error == nil {
			return resource.ConnectionName, nil
		}
		// If DB lookup fails, use pattern-based fallback
		if len(parts) >= 2 {
			return parts[0], nil
		}
		return "unknown", result.Error
	}

	// For other resources, fall back to KV store lookup
	key := common.GenResourceKey(nsId, resourceType, resourceId)
	keyValue, _, err := kvstore.GetKv(key)
	if err != nil {
		// If KV lookup fails, use pattern-based fallback
		if len(parts) >= 2 {
			return parts[0], nil
		}
		return "unknown", err
	}

	// Parse the JSON value to extract connection name
	switch resourceType {

	case model.StrSSHKey:
		var resource model.SshKeyInfo
		err = json.Unmarshal([]byte(keyValue.Value), &resource)
		if err != nil {
			return "unknown", err
		}
		return resource.ConnectionName, nil

	case model.StrSecurityGroup:
		var resource model.SecurityGroupInfo
		err = json.Unmarshal([]byte(keyValue.Value), &resource)
		if err != nil {
			return "unknown", err
		}
		return resource.ConnectionName, nil

	case model.StrVNet:
		var resource model.VNetInfo
		err = json.Unmarshal([]byte(keyValue.Value), &resource)
		if err != nil {
			return "unknown", err
		}
		return resource.ConnectionName, nil

	case model.StrSubnet:
		var resource model.SubnetInfo
		err = json.Unmarshal([]byte(keyValue.Value), &resource)
		if err != nil {
			return "unknown", err
		}
		return resource.ConnectionName, nil

	case model.StrDataDisk:
		var resource model.DataDiskInfo
		err = json.Unmarshal([]byte(keyValue.Value), &resource)
		if err != nil {
			return "unknown", err
		}
		return resource.ConnectionName, nil

	default:
		// For unsupported resource types, use pattern-based extraction
		if len(parts) >= 2 {
			return parts[0], nil
		}
		return "unknown", fmt.Errorf("unsupported resource type: %s", resourceType)
	}
}

func init() {

	validate = validator.New()

	// register function to get tag name from json tags.
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	// register validation for 'Tb*Req'
	// NOTE: only have to register a non-pointer type for 'Tb*Req', validator
	// internally dereferences during it's type checks.
	validate.RegisterStructValidation(DataDiskReqStructLevelValidation, model.DataDiskReq{})
	validate.RegisterStructValidation(ImageReqStructLevelValidation, model.ImageReq{})
	validate.RegisterStructValidation(CustomImageReqStructLevelValidation, model.CustomImageReq{})
	validate.RegisterStructValidation(SecurityGroupReqStructLevelValidation, model.SecurityGroupReq{})
	validate.RegisterStructValidation(SpecReqStructLevelValidation, model.SpecReq{})
	validate.RegisterStructValidation(SshKeyReqStructLevelValidation, model.SshKeyReq{})
	validate.RegisterStructValidation(VNetReqStructLevelValidation, model.VNetReq{})
}

// DelAllResources deletes all TB Resource objects of the given resourceType.
// deleteResourceIdsParallel deletes the given resource IDs of one type in parallel,
// grouped per CSP connection under a bounded semaphore, reusing DelResource (which
// includes the post-deletion CSP verification). Callers do their own selection/guards
// and hand the final delete set here, so the parallel engine lives in one place.
func deleteResourceIdsParallel(nsId string, resourceType string, ids []string, forceFlag string) []model.ResourceDeleteResult {
	var resultList []model.ResourceDeleteResult
	if len(ids) == 0 {
		return resultList
	}
	var mutex sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, len(ids))
	var errChanClosed int32

	// Group resources by CSP connection to apply a per-CSP concurrency limit.
	connectionGroups := make(map[string][]string)
	for _, resourceId := range ids {
		connectionName, err := getResourceConnectionName(nsId, resourceType, resourceId)
		if err != nil {
			log.Warn().Err(err).Str("resourceId", resourceId).Msg("Failed to get connection name, using default group")
			connectionName = "unknown"
		}
		connectionGroups[connectionName] = append(connectionGroups[connectionName], resourceId)
	}

	const maxConcurrentPerCSP = 20
	connectionSemaphores := make(map[string]chan struct{})
	totalResources := 0
	for connectionName := range connectionGroups {
		connectionSemaphores[connectionName] = make(chan struct{}, maxConcurrentPerCSP)
		totalResources += len(connectionGroups[connectionName])
		log.Info().Msgf("Connection %s: %d %s resources", connectionName, len(connectionGroups[connectionName]), resourceType)
	}
	log.Info().Msgf("Starting deletion of %d %s resources across %d connections", totalResources, resourceType, len(connectionGroups))

	// Process ALL connection groups in parallel; within each, bounded by its semaphore.
	for connectionName, resourceIds := range connectionGroups {
		for range resourceIds {
			wg.Add(1)
		}
		go func(connName string, resourceList []string, semaphore chan struct{}) {
			for _, resourceId := range resourceList {
				go func(resourceId string) {
					defer wg.Done()
					semaphore <- struct{}{}
					defer func() { <-semaphore }()

					startTime := time.Now()
					common.RandomSleep(0, 1*1000) // avoid thundering herd

					success := true
					errMessage := ""
					if err := DelResource(nsId, resourceType, resourceId, forceFlag); err != nil {
						success = false
						errMessage = err.Error()
						if atomic.LoadInt32(&errChanClosed) == 0 {
							select {
							case errChan <- err:
							case <-time.After(10 * time.Millisecond):
							default:
							}
						}
					}

					mutex.Lock()
					resultList = append(resultList, model.ResourceDeleteResult{
						ResourceType: resourceType, ResourceId: resourceId, Success: success, Message: errMessage,
					})
					mutex.Unlock()

					deleteStatus := "[Done]"
					if !success {
						deleteStatus = "[Failed]"
					}
					log.Debug().Str("connectionName", connName).Str("resourceId", resourceId).
						Str("status", deleteStatus).Dur("elapsed", time.Since(startTime)).Msg("Resource deletion completed")
				}(resourceId)
			}
		}(connectionName, resourceIds, connectionSemaphores[connectionName])
	}

	wg.Wait()
	if atomic.CompareAndSwapInt32(&errChanClosed, 0, 1) {
		close(errChan)
	}
	for err := range errChan {
		if err != nil {
			log.Info().Err(err).Msg("error deleting resource")
		}
	}
	return resultList
}

func DelAllResources(nsId string, resourceType string, subString string, forceFlag string) (model.ResourceDeleteResults, error) {
	var resultList []model.ResourceDeleteResult

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.ResourceDeleteResults{Results: resultList}, err
	}

	resourceIdList, err := ListResourceId(nsId, resourceType)
	if err != nil {
		return model.ResourceDeleteResults{Results: resultList}, err
	}

	if len(resourceIdList) == 0 {
		errString := fmt.Sprintf("There is no %s resource in %s", resourceType, nsId)
		err := fmt.Errorf("%s", errString)
		log.Error().Err(err).Msg("")
		return model.ResourceDeleteResults{Results: resultList}, err
	}

	// Filter by subString, then delete the selected resources in parallel per CSP.
	var ids []string
	for _, resourceId := range resourceIdList {
		if subString != "" && !strings.Contains(resourceId, subString) {
			continue
		}
		ids = append(ids, resourceId)
	}
	resultList = deleteResourceIdsParallel(nsId, resourceType, ids, forceFlag)

	log.Info().Msgf("DelAllResources completed. Total results: %d", len(resultList))
	for i, result := range resultList {
		status := "[Done]"
		if !result.Success {
			status = "[Failed]"
		}
		log.Debug().Msgf("Result %d: %s %s: %s %s", i, status, result.ResourceType, result.ResourceId, result.Message)
	}

	// Sort the results for consistent output ordering (by ResourceType then ResourceId)
	sort.Slice(resultList, func(i, j int) bool {
		if resultList[i].ResourceType != resultList[j].ResourceType {
			return resultList[i].ResourceType < resultList[j].ResourceType
		}
		return resultList[i].ResourceId < resultList[j].ResourceId
	})

	// Build summary counts
	successCount := 0
	failedCount := 0
	for _, r := range resultList {
		if r.Success {
			successCount++
		} else {
			failedCount++
		}
	}

	response := model.ResourceDeleteResults{
		Total:        len(resultList),
		SuccessCount: successCount,
		FailedCount:  failedCount,
		Results:      resultList,
	}

	return response, nil
}

// DelResource deletes the TB Resource object
func DelResource(nsId string, resourceType string, resourceId string, forceFlag string) error {

	InvalidateVerifyCache(nsId, resourceType, resourceId)

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	err = common.CheckString(resourceId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	check, err := CheckResource(nsId, resourceType, resourceId)

	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	if !check {
		errString := "The " + resourceType + " " + resourceId + " does not exist."
		err := fmt.Errorf("%s", errString)
		return err
	}

	key := common.GenResourceKey(nsId, resourceType, resourceId)
	keyValue, _, _ := kvstore.GetKv(key)
	// In CheckResource() above, calling 'kvstore.GetKv()' and checking err parts exist.
	// So, in here, we don't need to check whether keyValue == nil or err != nil.

	// Pre-deletion dependency check: verify the resource is not still referenced by any VMs or other objects.
	// This prevents unnecessary CSP API calls that would fail with DependencyViolation errors.
	// Skipped when forceFlag is set, allowing forced cleanup of orphaned resources.
	if forceFlag != "true" {
		associatedList, assocErr := GetAssociatedObjectList(nsId, resourceType, resourceId)
		if assocErr != nil {
			log.Error().Err(assocErr).Msgf("Failed to check associated objects for %s '%s'; blocking deletion for safety", resourceType, resourceId)
			return fmt.Errorf("cannot delete %s '%s': failed to verify dependencies: %w", resourceType, resourceId, assocErr)
		}
		if len(associatedList) > 0 {
			err := fmt.Errorf("cannot delete %s '%s': still referenced by %d object(s): %v",
				resourceType, resourceId, len(associatedList), associatedList)
			log.Warn().Err(err).Msg("Resource has active associations, deletion blocked")
			return err
		}
	}

	//cspType := common.GetResourcesCspType(nsId, resourceType, resourceId)

	var childResources any

	var url string
	uid := ""

	// vNetInfoForMark is populated in the StrVNet switch case so that
	// markVNetDeleteFailed can be called from the shared error paths below.
	var vNetInfoForMark *model.VNetInfo

	// CSP identifiers captured for the tombstone purge gate / Spider repair
	var tsCspId, tsCspName string

	// Create Req body
	type JsonTemplate struct {
		ConnectionName string
	}
	requestBody := JsonTemplate{}

	switch resourceType {
	case model.StrImage:
		// Delete image from database
		result := model.ORM.Delete(&model.ImageInfo{}, "namespace = ? AND id = ? AND (resource_type = ? OR resource_type IS NULL OR resource_type = '')",
			model.SystemCommonNs, resourceId, model.StrImage)
		if result.Error != nil {
			fmt.Println(result.Error.Error())
			return result.Error
		} else {
			log.Debug().Msg("Image deleted successfully from database")
		}
		return nil

	case model.StrCustomImage:
		// Get custom image info from database
		var temp model.ImageInfo
		result := model.ORM.Where("namespace = ? AND id = ? AND resource_type = ?",
			nsId, resourceId, model.StrCustomImage).First(&temp)
		if result.Error != nil {
			log.Error().Err(result.Error).Msg("")
			return result.Error
		}

		requestBody.ConnectionName = temp.ConnectionName
		url = model.SpiderRestUrl + "/myimage/" + temp.CspImageName
		uid = temp.Uid
		tsCspId, tsCspName = temp.CspImageId, temp.CspImageName
	case model.StrSpec:
		// delete spec info

		//get related recommend spec
		//keyValue, err := kvstore.GetKv(key)
		// content := SpecInfo{}
		// err := json.Unmarshal([]byte(keyValue.Value), &content)
		// if err != nil {
		// 	log.Error().Err(err).Msg("")
		// 	return err
		// }

		// err = kvstore.Delete(key)
		// if err != nil {
		// 	log.Error().Err(err).Msg("")
		// 	return err
		// }

		// "DELETE FROM `spec` WHERE `id` = '" + resourceId + "';"
		result := model.ORM.Delete(&model.SpecInfo{}, "namespace = ? AND id = ?", nsId, resourceId)
		if result.Error != nil {
			fmt.Println(result.Error.Error())
		} else {
			log.Debug().Msg("Data deleted successfully..")
		}

		return nil
	case model.StrSSHKey:
		temp := model.SshKeyInfo{}
		err = json.Unmarshal([]byte(keyValue.Value), &temp)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
		requestBody.ConnectionName = temp.ConnectionName
		url = model.SpiderRestUrl + "/keypair/" + temp.CspResourceName
		uid = temp.Uid
		tsCspId, tsCspName = temp.CspResourceId, temp.CspResourceName

	case model.StrVNet:
		temp := model.VNetInfo{}
		err = json.Unmarshal([]byte(keyValue.Value), &temp)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
		requestBody.ConnectionName = temp.ConnectionName
		url = model.SpiderRestUrl + "/vpc/" + temp.CspResourceName
		childResources = temp.SubnetInfoList
		uid = temp.Uid
		vNetInfoForMark = &temp
		tsCspId, tsCspName = temp.CspResourceId, temp.CspResourceName

	case model.StrSecurityGroup:
		temp := model.SecurityGroupInfo{}
		err = json.Unmarshal([]byte(keyValue.Value), &temp)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
		requestBody.ConnectionName = temp.ConnectionName
		url = model.SpiderRestUrl + "/securitygroup/" + temp.CspResourceName
		uid = temp.Uid
		tsCspId, tsCspName = temp.CspResourceId, temp.CspResourceName

		// Pre-deletion CSP dependency check for Security Groups (skipped when forceFlag == "true")
		if forceFlag != "true" {
			if connConfig, connErr := common.GetConnConfig(temp.ConnectionName); connErr == nil {
				provider := connConfig.ProviderName
				region := connConfig.RegionDetail.RegionName
				sgIdentifier := temp.CspResourceId
				if sgIdentifier == "" {
					sgIdentifier = temp.CspResourceName
				}
				inUseInstances, checkErr := directcsp.CheckSecurityGroupInUse(context.Background(), provider, region, sgIdentifier)
				if checkErr != nil {
					log.Warn().Err(checkErr).Msgf("Failed to verify if SecurityGroup '%s' is in use on %s; proceeding with deletion", resourceId, provider)
				} else if len(inUseInstances) > 0 {
					return fmt.Errorf("cannot delete %s '%s' (CSP ID: %s): in use by %d active %s instance(s): %v",
						resourceType, resourceId, sgIdentifier, len(inUseInstances), strings.ToUpper(provider), inUseInstances)
				}
			}
		}

	case model.StrDataDisk:
		temp := model.DataDiskInfo{}
		err = json.Unmarshal([]byte(keyValue.Value), &temp)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
		requestBody.ConnectionName = temp.ConnectionName
		url = model.SpiderRestUrl + "/disk/" + temp.CspResourceName
		uid = temp.Uid
		tsCspId, tsCspName = temp.CspResourceId, temp.CspResourceName
	/*
		case "subnet":
			temp := subnetInfo{}
			json.Unmarshal([]byte(keyValue.Value), &content)
			return content.CspResourceId
		case "publicIp":
			temp := publicIpInfo{}
			json.Unmarshal([]byte(keyValue.Value), &temp)
			requestBody.ConnectionName = temp.ConnectionName
			url = common.TB_SPIDER_REST_URL + "/publicip/" + temp.CspPublicIpName
		case "vNic":
			temp := vNicInfo{}
			json.Unmarshal([]byte(keyValue.Value), &temp)
			requestBody.ConnectionName = temp.ConnectionName
			url = common.TB_SPIDER_REST_URL + "/vnic/" + temp.CspVNicName
	*/
	default:
		err := fmt.Errorf("invalid resourceType")
		return err
	}

	if forceFlag == "true" {
		url += "?force=true"
	}

	// Fail-closed tombstone deletion
	tombstoneEnabled := tombstoneSupported(resourceType)
	if tombstoneEnabled {
		inflightKey := deletionSlotKey(nsId, resourceType, resourceId)
		if _, busy := deletionInFlight.LoadOrStore(inflightKey, struct{}{}); busy {
			return fmt.Errorf("deletion of %s '%s' is already in progress (%w)", resourceType, resourceId, ErrDeletionInProgress)
		}
		defer deletionInFlight.Delete(inflightKey)

		// A vNet's subnets are removed as children by cleanupLocalResourceRecord (no
		// per-subnet Spider call), so surface their Deleting state here — otherwise
		// only the vNet shows Deleting while its subnets jump Available -> gone. Run
		// before markResourceDeleting: this rewrites the whole record, which would drop
		// the sjson-only tombstone fields markResourceDeleting adds next.
		if resourceType == model.StrVNet {
			markVNetSubnetsDeleting(nsId, resourceId)
		}

		// Persist the tombstone before calling Spider (crash-safe; retry keeps it)
		if err := markResourceDeleting(nsId, resourceType, resourceId); err != nil {
			return err
		}
	}

	client := clientManager.NewHttpClient()
	log.Debug().Msg("Sending DELETE request to " + url)

	execDelete := func() (model.SpiderBooleanInfo, error) {
		var cr model.SpiderBooleanInfo
		_, e := clientManager.ExecuteHttpRequest(
			client,
			"DELETE",
			url,
			nil,
			clientManager.SetUseBody(requestBody),
			&requestBody,
			&cr,
			clientManager.VeryShortDuration,
		)
		return cr, e
	}
	callResult, err := execDelete()

	// KT vNet: the CSP "VPC" is the account's undeletable default network, so the generic
	// presence checks below always say it exists. Decide by its tiers instead.
	if err != nil && resourceType == model.StrVNet && vNetInfoForMark != nil &&
		strings.Contains(strings.ToLower(err.Error()), "does not exist in connection") {
		if present, perr := VNetPresentOnCsp(*vNetInfoForMark); perr == nil && !present {
			log.Warn().Msgf("vNet '%s': Spider has no record and no CSP network remains; removing stale CB-TB record", resourceId)
			if cleanupErr := cleanupLocalResourceRecord(nsId, resourceType, resourceId, key, uid, childResources); cleanupErr != nil {
				return cleanupErr
			}
			return nil
		}
	}

	// Spider forgot the resource but the CSP still has it: retrying the DELETE alone
	// can never succeed, so re-register and retry once
	if err != nil && tombstoneEnabled &&
		strings.Contains(strings.ToLower(err.Error()), "does not exist in connection") {
		if present, gateErr := ResourcePresentOnCsp(requestBody.ConnectionName, resourceType, tsCspId, tsCspName); gateErr == nil && present {
			log.Warn().Msgf("%s '%s' exists on CSP but Spider lost its registration; repairing and retrying DELETE", resourceType, resourceId)
			if regErr := repairSpiderRegistration(resourceType, requestBody.ConnectionName, tsCspName, tsCspId); regErr == nil {
				callResult, err = execDelete()
			} else {
				log.Warn().Err(regErr).Msg("Spider re-registration failed")
			}
		}
	}

	if err != nil {
		// Spider's own IID registry (not just the CSP) has forgotten this resource —
		// e.g. an earlier DELETE was ambiguously interrupted mid-flight, Spider later
		// confirmed not-found on the CSP and dropped its own record, but returned an
		// error instead of success. Retrying this DELETE can never succeed. If the CSP
		// also no longer has it, clean up CB-TB's own record instead of failing forever;
		// if the CSP still has it (Spider unregistered without deleting), this is left
		// for the operator — CheckAssociatedCspResourceExistence's onCsp=true case.
		if strings.Contains(strings.ToLower(err.Error()), "does not exist in connection") {
			if onCsp, onSpider, checkErr := CheckAssociatedCspResourceExistence(nsId, resourceType, resourceId, requestBody.ConnectionName); checkErr == nil && !onCsp && !onSpider {
				log.Warn().Str("resourceType", resourceType).Str("resourceId", resourceId).
					Msg("Resource confirmed gone from both Spider and CSP; removing stale CB-TB record")
				if cleanupErr := cleanupLocalResourceRecord(nsId, resourceType, resourceId, key, uid, childResources); cleanupErr != nil {
					log.Error().Err(cleanupErr).Str("resourceType", resourceType).Str("resourceId", resourceId).
						Msg("Failed to remove stale CB-TB record")
					return cleanupErr
				}
				return nil
			}
		}
		if resourceType == model.StrVNet && vNetInfoForMark != nil {
			markVNetDeleteFailed(nsId, resourceId, key, vNetInfoForMark, err)
		}
		if tombstoneEnabled {
			if forceFlag == "true" {
				log.Warn().Err(err).Msgf("Force deletion of %s '%s': Spider DELETE failed; removing the record anyway (the CSP resource may remain as an orphan)", resourceType, resourceId)
				return cleanupLocalResourceRecord(nsId, resourceType, resourceId, key, uid, childResources)
			}
			markResourceDeleteFailed(nsId, resourceType, resourceId, err)
			err = fmt.Errorf("%v (%w)", err, ErrDeletionUnconfirmed)
		}
		log.Error().Err(err).Msg("")
		return err
	}

	// Validate Spider's response Result field
	// Spider returns {"Result": "true"} on successful deletion, {"Result": "false"} when deletion was not performed.
	// Previously this was not checked, causing CB-TB to delete its kvstore entry even when Spider didn't actually delete the resource.
	if !strings.EqualFold(callResult.Result, "true") {
		// Spider answers Result="false" (HTTP 200) when the driver reported not-found: it then
		// drops its own IID record without deleting anything, so a retry can never succeed.
		// If the CSP enumeration confirms the resource is gone, the record is stale, not
		// undeleted: purge it instead of failing forever (e.g. a global GCP machine image
		// registered once per region, deleted through one of them).
		if tombstoneEnabled && forceFlag != "true" {
			if present, gateErr := presentOnCspAfterDelete(requestBody.ConnectionName, resourceType, tsCspId, tsCspName, vNetInfoForMark); gateErr == nil && !present {
				log.Warn().Msgf("%s '%s': Spider reported Result=false and the CSP no longer has it; removing stale CB-TB record", resourceType, resourceId)
				return cleanupLocalResourceRecord(nsId, resourceType, resourceId, key, uid, childResources)
			}
		}
		resultErr := fmt.Errorf("Spider returned Result=%q for DELETE %s/%s (resource may still exist on Spider/CSP)", callResult.Result, resourceType, resourceId)
		if resourceType == model.StrVNet && vNetInfoForMark != nil {
			markVNetDeleteFailed(nsId, resourceId, key, vNetInfoForMark, resultErr)
		}
		if tombstoneEnabled {
			if forceFlag == "true" {
				log.Warn().Err(resultErr).Msgf("Force deletion of %s '%s': removing the record anyway (the CSP resource may remain as an orphan)", resourceType, resourceId)
				return cleanupLocalResourceRecord(nsId, resourceType, resourceId, key, uid, childResources)
			}
			markResourceDeleteFailed(nsId, resourceType, resourceId, resultErr)
			resultErr = fmt.Errorf("%v (%w)", resultErr, ErrDeletionUnconfirmed)
		}
		log.Error().Err(resultErr).Msg("Resource deletion not confirmed by Spider")
		return resultErr
	}
	log.Debug().Msgf("Spider confirmed deletion (Result=%s) for %s/%s", callResult.Result, resourceType, resourceId)

	if tombstoneEnabled {
		// Eventual-consistency wait; a "gone" GET alone never authorizes purge
		// (Spider may answer from its own registry while the CSP resource survives)
		for attempt := 0; attempt < deleteVerifyPollAttempts; attempt++ {
			if attempt > 0 {
				time.Sleep(deleteVerifyPollInterval)
			}
			if verifyResourceDeletedOnSpider(url, requestBody.ConnectionName, resourceType, resourceId) == nil {
				break
			}
		}

		// Only confirmed absence from the CSP enumeration may purge the record
		present, gateErr := presentOnCspAfterDelete(requestBody.ConnectionName, resourceType, tsCspId, tsCspName, vNetInfoForMark)
		if gateErr == nil && !present {
			return cleanupLocalResourceRecord(nsId, resourceType, resourceId, key, uid, childResources)
		}
		if forceFlag == "true" {
			log.Warn().Msgf("Force deletion of %s '%s': deletion unconfirmed (present=%v, gateErr=%v); removing the record anyway (the CSP resource may remain as an orphan)", resourceType, resourceId, present, gateErr)
			return cleanupLocalResourceRecord(nsId, resourceType, resourceId, key, uid, childResources)
		}
		var cause error
		sentinel := ErrDeletionUnconfirmed
		if gateErr != nil {
			cause = fmt.Errorf("deletion of %s '%s' unconfirmed: existence check failed: %v", resourceType, resourceId, gateErr)
			markResourceDeleteFailed(nsId, resourceType, resourceId, cause)
		} else {
			cause = fmt.Errorf("%s '%s' still exists on the CSP although Spider reported successful deletion", resourceType, resourceId)
			if markResourceStillOnCsp(nsId, resourceType, resourceId, cause) {
				sentinel = ErrDeletionInProgress
			}
		}
		err := fmt.Errorf("%v; the record is retained for retry — retry DELETE, or use force to discard it (%w)", cause, sentinel)
		log.Warn().Err(err).Msg("Fail-closed deletion: record retained")
		return err
	}

	// Fail-closed gate for own-path types routed through here (e.g. vNet via provisioning
	// rollback): purge only once the CSP enumeration confirms the resource is gone (issue #2685).
	if _, gateable := spiderAllListPath[resourceType]; gateable && forceFlag != "true" && tsCspName != "" {
		if present, gateErr := presentOnCspAfterDelete(requestBody.ConnectionName, resourceType, tsCspId, tsCspName, vNetInfoForMark); gateErr != nil || present {
			cause := fmt.Errorf("%s '%s' still exists on the CSP after DELETE; record retained — retry, or use force to discard it (%w)", resourceType, resourceId, ErrDeletionUnconfirmed)
			if gateErr != nil {
				cause = fmt.Errorf("%s '%s' deletion unconfirmed: CSP existence check failed: %v (%w)", resourceType, resourceId, gateErr, ErrDeletionUnconfirmed)
			}
			if resourceType == model.StrVNet && vNetInfoForMark != nil {
				markVNetDeleteFailed(nsId, resourceId, key, vNetInfoForMark, cause)
			}
			log.Warn().Err(cause).Msg("Fail-closed deletion: record retained")
			return cause
		}
	}

	// Defense-in-depth for non-gate-capable types: Spider's DELETE response is authoritative.
	if err := verifyResourceDeletedOnSpider(url, requestBody.ConnectionName, resourceType, resourceId); err != nil {
		log.Warn().Err(err).Msgf("Resource %s/%s may still exist on Spider after deletion was reported as successful", resourceType, resourceId)
	}

	return cleanupLocalResourceRecord(nsId, resourceType, resourceId, key, uid, childResources)
}

func DeregisterResource(nsId string, resourceType string, resourceId string) error {

	InvalidateVerifyCache(nsId, resourceType, resourceId)

	check, err := CheckResource(nsId, resourceType, resourceId)

	if err != nil || !check {
		log.Error().Err(err).Msg("")
		return err
	}

	key := common.GenResourceKey(nsId, resourceType, resourceId)
	keyValue, _, _ := kvstore.GetKv(key)

	var url string
	uid := ""

	// Create Req body
	type JsonTemplate struct {
		ConnectionName string
	}
	requestBody := JsonTemplate{}

	switch resourceType {

	case model.StrCustomImage:
		// Get custom image info from database
		resourceObj, err := GetResource(nsId, model.StrCustomImage, resourceId)
		if err != nil {
			return err
		}
		temp := resourceObj.(model.ImageInfo)
		requestBody.ConnectionName = temp.ConnectionName
		// Use deregister API: /regmyimage/{Name}
		url = model.SpiderRestUrl + "/regmyimage/" + temp.CspImageName
		uid = temp.Uid

	case model.StrSSHKey:
		temp := model.SshKeyInfo{}
		err = json.Unmarshal([]byte(keyValue.Value), &temp)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
		requestBody.ConnectionName = temp.ConnectionName
		// Use deregister API: /regkeypair/{Name}
		url = model.SpiderRestUrl + "/regkeypair/" + temp.CspResourceName
		uid = temp.Uid

	case model.StrSecurityGroup:
		temp := model.SecurityGroupInfo{}
		err = json.Unmarshal([]byte(keyValue.Value), &temp)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
		requestBody.ConnectionName = temp.ConnectionName
		// Use deregister API: /regsecuritygroup/{Name}
		url = model.SpiderRestUrl + "/regsecuritygroup/" + temp.CspResourceName
		uid = temp.Uid

	case model.StrDataDisk:
		temp := model.DataDiskInfo{}
		err = json.Unmarshal([]byte(keyValue.Value), &temp)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
		requestBody.ConnectionName = temp.ConnectionName
		// Use deregister API: /regdisk/{Name}
		url = model.SpiderRestUrl + "/regdisk/" + temp.CspResourceName
		uid = temp.Uid

	default:
		err := fmt.Errorf("invalid resourceType for deregistration: %s", resourceType)
		return err
	}

	var callResult any
	client := clientManager.NewHttpClient()
	method := "DELETE"

	log.Debug().Msg("Sending deregister DELETE request to " + url)

	restyResp, err := clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&callResult,
		clientManager.VeryShortDuration,
	)
	err = clientManager.HandleHttpResponse(restyResp, err)

	if err != nil {
		// If cb-spider reports the resource doesn't exist, it was never registered in Spider
		// (e.g. imported via registerCspResources). Treat as already deregistered from Spider
		// and proceed to clean up the TB registry.
		if apierr.IsNotFound(err) {
			log.Warn().Err(err).Msg("Resource not found in cb-spider IID store; proceeding with TB registry cleanup")
		} else {
			log.Error().Err(err).Msg("")
			return err
		}
	} else {
		log.Debug().Msg("Deregister request finished from " + url)
	}

	if strings.EqualFold(resourceType, model.StrCustomImage) {
		// Delete custom image from database
		result := model.ORM.Delete(&model.ImageInfo{}, "namespace = ? AND id = ? AND resource_type = ?",
			nsId, resourceId, model.StrCustomImage)
		if result.Error != nil {
			log.Error().Err(result.Error).Msg("")
		} else {
			log.Debug().Msg("Custom image deregistered successfully from database")
			imageInfoCache.Delete(strings.ToLower(nsId) + "/" + strings.ToLower(resourceId))
		}
	} else {
		// Delete from kvstore (for non-DB resources)
		err = kvstore.Delete(key)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
		if strings.EqualFold(resourceType, model.StrSSHKey) {
			_ = secret.DeleteSshKey(context.Background(), nsId, resourceId)
		}
	}

	err = label.DeleteLabelObject(resourceType, uid)
	if err != nil {
		log.Error().Err(err).Msg("")
	}

	return nil
}

// CheckSubnetInUseByNodes checks if a subnet is being used by any VMs
// It retrieves the VNet's associatedObjectList and checks each VM's subnetId field
func CheckSubnetInUseByNodes(nsId string, vNetId string, subnetId string) (bool, error) {
	resources, err := label.GetResourcesByLabelSelector(model.StrNode, "sys.subnetId="+subnetId+",sys.vNetId="+vNetId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get VMs by subnetId and vNetId labels")
		return false, err
	}

	if len(resources) > 0 {
		return true, nil
	}

	return false, nil
}

// DelEleInSlice delete an element from slice by index
//   - arr: the reference of slice
//   - index: the index of element will be deleted
func DelEleInSlice(arr any, index int) {
	vField := reflect.ValueOf(arr)
	value := vField.Elem()
	if value.Kind() == reflect.Slice || value.Kind() == reflect.Array {
		result := reflect.AppendSlice(value.Slice(0, index), value.Slice(index+1, value.Len()))
		value.Set(result)
	}
}

// ListResourceId returns the list of TB Resource object IDs of given resourceType
func ListResourceId(nsId string, resourceType string) ([]string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	if strings.EqualFold(resourceType, model.StrImage) ||
		strings.EqualFold(resourceType, model.StrCustomImage) ||
		strings.EqualFold(resourceType, model.StrSSHKey) ||
		strings.EqualFold(resourceType, model.StrSpec) ||
		strings.EqualFold(resourceType, model.StrVNet) ||
		strings.EqualFold(resourceType, model.StrSecurityGroup) ||
		strings.EqualFold(resourceType, model.StrDataDisk) ||
		strings.EqualFold(resourceType, model.StrObjectStorage) {
		// continue
	} else {
		err = fmt.Errorf("invalid resource type")
		log.Error().Err(err).Msg("")
		return nil, err
	}

	// Handle Image, CustomImage, and Spec using PostgreSQL (GORM)
	var resourceList []string
	switch resourceType {
	case model.StrImage:
		var images []model.ImageInfo
		result := model.ORM.Select("id").Where("namespace = ? AND (resource_type = ? OR resource_type IS NULL OR resource_type = '')",
			nsId, model.StrImage).Find(&images)
		if result.Error != nil {
			log.Error().Err(result.Error).Msg("Failed to list image IDs from database")
			return nil, result.Error
		}
		for _, img := range images {
			resourceList = append(resourceList, img.Id)
		}
		return resourceList, nil

	case model.StrCustomImage:
		var images []model.ImageInfo
		result := model.ORM.Select("id").Where("namespace = ? AND resource_type = ?",
			nsId, model.StrCustomImage).Find(&images)
		if result.Error != nil {
			log.Error().Err(result.Error).Msg("Failed to list custom image IDs from database")
			return nil, result.Error
		}
		for _, img := range images {
			resourceList = append(resourceList, img.Id)
		}
		return resourceList, nil

	case model.StrSpec:
		var specs []model.SpecInfo
		result := model.ORM.Select("id").Where("namespace = ?", nsId).Find(&specs)
		if result.Error != nil {
			log.Error().Err(result.Error).Msg("Failed to list spec IDs from database")
			return nil, result.Error
		}
		for _, spec := range specs {
			resourceList = append(resourceList, spec.Id)
		}
		return resourceList, nil
	}

	// For other resource types, use kvstore (existing code)
	key := "/ns/" + nsId + "/resources/"
	keyValue, err := kvstore.GetKvList(key)

	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	/* if keyValue == nil, then for-loop below will not be executed, and the empty array will be returned in `resourceList` placeholder.
	if keyValue == nil {
		err = fmt.Errorf("ListResourceId(); %s is empty.", key)
		log.Error().Err(err).Msg("")
		return nil, err
	}
	*/

	for _, v := range keyValue {
		trimmedString := strings.TrimPrefix(v.Key, (key + resourceType + "/"))
		// prevent malformed key (if key for resource id includes '/', the key does not represent resource ID)
		if !strings.Contains(trimmedString, "/") {
			resourceList = append(resourceList, trimmedString)
		}
	}

	return resourceList, nil

}

// ListResource returns the list of TB Resource objects of given resourceType
func ListResource(nsId string, resourceType string, filterKey string, filterVal string) (any, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	if strings.EqualFold(resourceType, model.StrImage) ||
		strings.EqualFold(resourceType, model.StrCustomImage) ||
		strings.EqualFold(resourceType, model.StrSSHKey) ||
		strings.EqualFold(resourceType, model.StrSpec) ||
		strings.EqualFold(resourceType, model.StrVNet) ||
		strings.EqualFold(resourceType, model.StrSecurityGroup) ||
		strings.EqualFold(resourceType, model.StrDataDisk) ||
		strings.EqualFold(resourceType, model.StrObjectStorage) ||
		strings.EqualFold(resourceType, model.StrRDBMS) {
		// continue
	} else {
		errString := "Cannot list " + resourceType + "s."
		err := fmt.Errorf("%s", errString)
		return nil, err
	}

	//log.Debug().Msg("[Get] " + resourceType + " list")

	// Handle Image, CustomImage, and Spec using PostgreSQL (GORM)
	switch resourceType {
	case model.StrImage:
		var res []model.ImageInfo
		query := model.ORM.Where("namespace = ? AND (resource_type = ? OR resource_type IS NULL OR resource_type = '')",
			nsId, model.StrImage)

		// Apply filter if provided
		if filterKey != "" && filterVal != "" {
			// Use GORM's ability to filter by JSON/struct fields
			query = query.Where(filterKey+" LIKE ?", "%"+filterVal+"%")
		}

		result := query.Find(&res)
		if result.Error != nil {
			log.Error().Err(result.Error).Msg("")
			return nil, result.Error
		}
		return res, nil

	case model.StrCustomImage:
		var res []model.ImageInfo
		query := model.ORM.Where("namespace = ? AND resource_type = ?", nsId, model.StrCustomImage)

		// Apply filter if provided
		if filterKey != "" && filterVal != "" {
			query = query.Where(filterKey+" LIKE ?", "%"+filterVal+"%")
		}

		result := query.Find(&res)
		if result.Error != nil {
			log.Error().Err(result.Error).Msg("")
			return nil, result.Error
		}

		// Update status for each custom image
		// log.Debug().Msg("Updating status for custom images...")
		for i := range res {
			newObj, err := GetResource(nsId, model.StrCustomImage, res[i].Id)
			if err != nil {
				log.Error().Err(err).Msg("")
				res[i].Description = err.Error()
				res[i].ImageStatus = "Error"
			} else if newObj != nil {
				res[i] = newObj.(model.ImageInfo)
			}
			// log.Debug().Msgf("Custom Image %s status: %s", res[i].Id, res[i].ImageStatus)
		}
		return res, nil

	case model.StrSpec:
		var res []model.SpecInfo
		query := model.ORM.Where("namespace = ?", nsId)

		// Apply filter if provided
		if filterKey != "" && filterVal != "" {
			query = query.Where(filterKey+" LIKE ?", "%"+filterVal+"%")
		}

		result := query.Find(&res)
		if result.Error != nil {
			log.Error().Err(result.Error).Msg("")
			return nil, result.Error
		}
		return res, nil
	}

	// For other resource types, use kvstore (existing code)
	key := "/ns/" + nsId + "/resources/" + resourceType
	//log.Debug().Msg(key)

	keyValue, err := kvstore.GetKvList(key)
	keyValue = kvutil.FilterKvListBy(keyValue, key, 1)

	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	if keyValue != nil {
		switch resourceType {
		case model.StrSecurityGroup:
			res := []model.SecurityGroupInfo{}
			for _, v := range keyValue {
				tempObj := model.SecurityGroupInfo{}
				err = json.Unmarshal([]byte(v.Value), &tempObj)
				if err != nil {
					log.Error().Err(err).Msg("")
					return nil, err
				}
				// Check the JSON body inclues both filterKey and filterVal strings. (assume key and value)
				if filterKey != "" {
					// If not inclues both, do not append current item to the list result.
					itemValueForCompare := strings.ToLower(v.Value)
					if !(strings.Contains(itemValueForCompare, strings.ToLower(filterKey)) && strings.Contains(itemValueForCompare, strings.ToLower(filterVal))) {
						continue
					}
				}
				res = append(res, tempObj)
			}
			return res, nil
		// case model.StrSpec:
		// 	res := []model.SpecInfo{}
		// 	for _, v := range keyValue {
		// 		tempObj := model.SpecInfo{}
		// 		err = json.Unmarshal([]byte(v.Value), &tempObj)
		// 		if err != nil {
		// 			log.Error().Err(err).Msg("")
		// 			return nil, err
		// 		}
		// 		// Check the JSON body inclues both filterKey and filterVal strings. (assume key and value)
		// 		if filterKey != "" {
		// 			// If not inclues both, do not append current item to the list result.
		// 			itemValueForCompare := strings.ToLower(v.Value)
		// 			if !(strings.Contains(itemValueForCompare, strings.ToLower(filterKey)) && strings.Contains(itemValueForCompare, strings.ToLower(filterVal))) {
		// 				continue
		// 			}
		// 		}
		// 		res = append(res, tempObj)
		// 	}
		// 	return res, nil
		case model.StrSSHKey:
			res := []model.SshKeyInfo{}
			for _, v := range keyValue {
				tempObj := model.SshKeyInfo{}
				err = json.Unmarshal([]byte(v.Value), &tempObj)
				if err != nil {
					log.Error().Err(err).Msg("")
					return nil, err
				}
				// Check the JSON body inclues both filterKey and filterVal strings. (assume key and value)
				if filterKey != "" {
					// If not inclues both, do not append current item to the list result.
					itemValueForCompare := strings.ToLower(v.Value)
					if !(strings.Contains(itemValueForCompare, strings.ToLower(filterKey)) && strings.Contains(itemValueForCompare, strings.ToLower(filterVal))) {
						continue
					}
				}
				res = append(res, tempObj)
			}
			return res, nil
		case model.StrVNet:
			res := []model.VNetInfo{}
			for _, v := range keyValue {
				tempObj := model.VNetInfo{}
				err = json.Unmarshal([]byte(v.Value), &tempObj)
				if err != nil {
					log.Error().Err(err).Msg("")
					return nil, err
				}
				// Check the JSON body inclues both filterKey and filterVal strings. (assume key and value)
				if filterKey != "" {
					// If not inclues both, do not append current item to the list result.
					itemValueForCompare := strings.ToLower(v.Value)
					if !(strings.Contains(itemValueForCompare, strings.ToLower(filterKey)) && strings.Contains(itemValueForCompare, strings.ToLower(filterVal))) {
						continue
					}
				}
				res = append(res, tempObj)
			}
			return res, nil
		case model.StrDataDisk:
			res := []model.DataDiskInfo{}
			for _, v := range keyValue {
				tempObj := model.DataDiskInfo{}
				err = json.Unmarshal([]byte(v.Value), &tempObj)
				if err != nil {
					log.Error().Err(err).Msg("")
					return nil, err
				}

				// Update TB DataDisk object's 'status' field
				// Just calling GetResource(dataDisk) once will update TB DataDisk object's 'status' field
				newObj, err := GetResource(nsId, model.StrDataDisk, tempObj.Id)
				if err != nil {
					log.Error().Err(err).Msg("")
					tempObj.Status = "Failed"
					tempObj.SystemMessage = err.Error()
				} else {
					tempObj = newObj.(model.DataDiskInfo)
				}

				// Check the JSON body inclues both filterKey and filterVal strings. (assume key and value)
				if filterKey != "" {
					// If not inclues both, do not append current item to the list result.
					itemValueForCompare := strings.ToLower(v.Value)
					if !(strings.Contains(itemValueForCompare, strings.ToLower(filterKey)) && strings.Contains(itemValueForCompare, strings.ToLower(filterVal))) {
						continue
					}
				}
				res = append(res, tempObj)
			}
			return res, nil
		case model.StrObjectStorage:
			res := []model.ObjectStorageInfo{}
			for _, v := range keyValue {
				tempObj := model.ObjectStorageInfo{}
				err = json.Unmarshal([]byte(v.Value), &tempObj)
				if err != nil {
					log.Error().Err(err).Msg("")
					return nil, err
				}
				// Check the JSON body inclues both filterKey and filterVal strings. (assume key and value)
				if filterKey != "" {
					// If not inclues both, do not append current item to the list result.
					itemValueForCompare := strings.ToLower(v.Value)
					if !(strings.Contains(itemValueForCompare, strings.ToLower(filterKey)) && strings.Contains(itemValueForCompare, strings.ToLower(filterVal))) {
						continue
					}
				}
				res = append(res, tempObj)
			}
			return res, nil
		case model.StrRDBMS:
			res := []model.RDBMSInfo{}
			for _, v := range keyValue {
				tempObj := model.RDBMSInfo{}
				err = json.Unmarshal([]byte(v.Value), &tempObj)
				if err != nil {
					log.Error().Err(err).Msg("")
					return nil, err
				}
				// Check the JSON body inclues both filterKey and filterVal strings. (assume key and value)
				if filterKey != "" {
					// If not inclues both, do not append current item to the list result.
					itemValueForCompare := strings.ToLower(v.Value)
					if !(strings.Contains(itemValueForCompare, strings.ToLower(filterKey)) && strings.Contains(itemValueForCompare, strings.ToLower(filterVal))) {
						continue
					}
				}
				res = append(res, tempObj)
			}
			return res, nil
		}

	} else { //return empty object according to resourceType
		switch resourceType {
		case model.StrImage:
			return []model.ImageInfo{}, nil
		case model.StrCustomImage:
			return []model.ImageInfo{}, nil
		case model.StrSecurityGroup:
			return []model.SecurityGroupInfo{}, nil
		case model.StrSpec:
			return []model.SpecInfo{}, nil
		case model.StrSSHKey:
			return []model.SshKeyInfo{}, nil
		case model.StrVNet:
			return []model.VNetInfo{}, nil
		case model.StrDataDisk:
			return []model.DataDiskInfo{}, nil
		case model.StrObjectStorage:
			return []model.ObjectStorageInfo{}, nil
		case model.StrRDBMS:
			return []model.RDBMSInfo{}, nil
		}
	}

	err = fmt.Errorf("Some exceptional case happened. Please check the references of %s", common.GetFuncName())
	return nil, err // if interface{} == nil, make err be returned. Should not come this part if there is no err.
}

// CustomImageCreationTimeout is the maximum time to wait for a custom image to become available
// If an image stays in "Creating" state longer than this, it will be marked as "Failed"
const CustomImageCreationTimeout = 30 * time.Minute

// isStableImageStatus checks if CustomImage status is stable (no need to update from Spider)
// Stable states: Available, Failed, Deprecated - these won't change without explicit user action
// Unstable states: Creating, Deleting, Unavailable, empty - need to check Spider for updates
func isStableImageStatus(status model.ImageStatus) bool {
	return status == model.ImageAvailable ||
		status == model.ImageFailed ||
		status == model.ImageDeprecated
}

// MapSpiderToTumblebugImageStatus maps CB-Spider's image status to CB-Tumblebug's enhanced status
// CB-Spider only returns "Available" or "Unavailable", but CB-Tumblebug needs more granular states
// Exported for use by other packages (e.g., infra/snapshot.go)
func MapSpiderToTumblebugImageStatus(spiderStatus string) model.ImageStatus {
	switch strings.ToLower(strings.TrimSpace(spiderStatus)) {
	case "available":
		return model.ImageAvailable
	case "unavailable":
		// Spider returns "Unavailable" for "creating" state
		// CB-Tumblebug treats this as "Creating" and will check timeout
		return model.ImageCreating
	case "":
		return model.ImageCreating
	default:
		return model.ImageUnavailable
	}
}

// isStableDiskStatus checks if DataDisk status is stable (no need to update from Spider)
// Stable states: Available, Attached, Error, Failed - won't change without explicit user action
// Unstable states: empty, Creating, Deleting - need to check Spider for updates
func isStableDiskStatus(status model.DiskStatus) bool {
	return status == model.DiskAvailable || status == model.DiskAttached ||
		status == model.DiskError || status == model.DiskFailed
}

// GetResource returns the requested TB Resource object
func GetResource(nsId string, resourceType string, resourceId string) (any, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	log.Trace().Msg("[Get resource] " + resourceType + ", " + resourceId)

	// Handle Image, CustomImage, and Spec using PostgreSQL (GORM)
	switch resourceType {
	case model.StrImage, model.StrCustomImage:
		var res model.ImageInfo
		var result *gorm.DB

		if resourceType == model.StrCustomImage {
			// Get custom image (resource_type = customImage)
			result = model.ORM.Where("namespace = ? AND id = ? AND resource_type = ?",
				nsId, resourceId, model.StrCustomImage).First(&res)
		} else {
			// Get regular image (resource_type != customImage, namespace = system-common-ns)
			result = model.ORM.Where("namespace = ? AND id = ? AND resource_type != ?",
				model.SystemCommonNs, resourceId, model.StrCustomImage).First(&res)
		}

		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				errString := fmt.Sprintf("The %s %s does not exist.", resourceType, resourceId)
				return nil, fmt.Errorf("%s", errString)
			}
			log.Error().Err(result.Error).Msg("")
			return nil, result.Error
		}

		// For CustomImage, update status from Spider only if not in stable state
		if resourceType == model.StrCustomImage {
			// Deletion tombstones are TB-owned: a Spider refresh would resurrect the
			// status or error once the CSP resource is gone
			if res.DeletionRequestedAt != "" {
				return res, nil
			}

			// Skip Spider API call if status is already stable (Available)
			// CustomImage status changes: Creating -> Available (one-way transition)
			// Once Available, it remains Available until deleted
			if isStableImageStatus(res.ImageStatus) {
				log.Trace().Msgf("Skipping Spider status update for custom image %s (already stable: %s)", res.Id, res.ImageStatus)
				return res, nil
			}

			log.Debug().Msgf("Updating status for custom image ID:%s CspImageName:%s CspImageId:%s (current: %s)", res.Id, res.CspImageName, res.CspImageId, res.ImageStatus)

			url := fmt.Sprintf("%s/myimage/%s", model.SpiderRestUrl, res.CspImageName)
			// Note: CB-Spider has internal error. Log not useful error message like below:
			// Not effective to CB-TB logic, but need to be aware of it. since cb-spider log may confuse operator.
			// cb-tumblebug| 4:13PM DBG src/core/resource/common.go:1188 > Updating status for custom image ID:custom-image-g1 CspImageName:custom-image-g1 CspImageId:ami-09e8eaf264b0f76ab
			// cb-spider| [CB-SPIDER].[ERROR]: 2025-10-14 16:13:19 MyImageManager.go:471, github.com/cloud-barista/cb-spider/api-runtime/common-runtime.GetMyImage() - aws-ap-northeast-2, i-0c4405a99cb146221: does not exist!
			client := clientManager.NewHttpClient()
			client.SetAllowGetMethodPayload(true)

			requestBody := model.SpiderConnectionName{
				ConnectionName: res.ConnectionName,
			}
			method := "GET"
			var callResult model.SpiderMyImageInfo

			_, err := clientManager.ExecuteHttpRequest(
				client,
				method,
				url,
				nil,
				clientManager.SetUseBody(requestBody),
				&requestBody,
				&callResult,
				clientManager.MediumDuration,
			)

			if err != nil {
				log.Error().Err(err).Msg("")
				return nil, err
			}

			// Map Spider's status to CB-Tumblebug's enhanced status
			newStatus := MapSpiderToTumblebugImageStatus(string(callResult.Status))

			// Check for creation timeout - if image has been in "Creating" state too long, mark as Failed
			if newStatus == model.ImageCreating {
				creationTime, parseErr := time.Parse(time.RFC3339, res.CreationDate)
				if parseErr == nil {
					elapsed := time.Since(creationTime)
					if elapsed > CustomImageCreationTimeout {
						log.Warn().Msgf("Custom image %s has been in Creating state for %v (timeout: %v), marking as Failed",
							res.Id, elapsed.Round(time.Minute), CustomImageCreationTimeout)
						newStatus = model.ImageFailed
					} else {
						log.Debug().Msgf("Custom image %s still creating (elapsed: %v, timeout: %v)",
							res.Id, elapsed.Round(time.Second), CustomImageCreationTimeout)
					}
				}
			}

			res.ImageStatus = newStatus

			// Update the database with new status
			model.ORM.Model(&res).Where("namespace = ? AND id = ?", nsId, resourceId).
				Update("image_status", res.ImageStatus)
		}

		return res, nil

	case model.StrSpec:
		var res model.SpecInfo
		// Spec is always in system-common-ns
		result := model.ORM.Where("namespace = ? AND id = ?", model.SystemCommonNs, resourceId).First(&res)

		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				errString := fmt.Sprintf("The %s %s does not exist.", resourceType, resourceId)
				return nil, fmt.Errorf("%s", errString)
			}
			log.Error().Err(result.Error).Msg("")
			return nil, result.Error
		}

		return res, nil
	}

	// For other resource types, use kvstore (existing code)
	key := common.GenResourceKey(nsId, resourceType, resourceId)
	keyValue, exists, err := kvstore.GetKv(key)

	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	if exists {
		switch resourceType {
		case model.StrSecurityGroup:
			res := model.SecurityGroupInfo{}
			err = json.Unmarshal([]byte(keyValue.Value), &res)
			if err != nil {
				log.Error().Err(err).Msg("")
				return nil, err
			}
			return res, nil
		case model.StrSSHKey:
			res := model.SshKeyInfo{}
			err = json.Unmarshal([]byte(keyValue.Value), &res)
			if err != nil {
				log.Error().Err(err).Msg("")
				return nil, err
			}
			// Retrieve private key from OpenBao
			if privKey, err := secret.GetSshKey(context.Background(), nsId, resourceId); err == nil && privKey != "" {
				res.PrivateKey = privKey
			} else if res.PrivateKey == "" && len(res.KeyValueList) > 0 {
				// Fallback: check KeyValueList for older keys
				for _, kv := range res.KeyValueList {
					if strings.EqualFold(kv.Key, "PrivateKey") || strings.EqualFold(kv.Key, "KeyMaterial") {
						res.PrivateKey = kv.Value
						break
					}
				}
			}
			if strings.Contains(res.PrivateKey, "\\n") {
				res.PrivateKey = strings.ReplaceAll(res.PrivateKey, "\\n", "\n")
			}
			return res, nil
		case model.StrVNet:
			res := model.VNetInfo{}
			err = json.Unmarshal([]byte(keyValue.Value), &res)
			if err != nil {
				log.Error().Err(err).Msg("")
				return nil, err
			}
			return res, nil
		case model.StrDataDisk:
			res := model.DataDiskInfo{}
			err = json.Unmarshal([]byte(keyValue.Value), &res)
			if err != nil {
				log.Error().Err(err).Msg("")
				return res, err
			}

			// Deletion tombstones are TB-owned: a Spider refresh would resurrect the
			// status or error once the CSP resource is gone
			if res.DeletionRequestedAt != "" {
				return res, nil
			}

			// Skip Spider API call if status is already stable (Available, Attached, Error)
			// DataDisk status changes:
			//   Creating -> Available (after creation completes)
			//   Available <-> Attached (only via explicit Attach/Detach actions)
			//   Deleting -> (resource deleted)
			// Once in stable state, it won't change without user action
			if isStableDiskStatus(res.Status) {
				log.Trace().Msgf("Skipping Spider status update for dataDisk %s (already stable: %s)", res.Id, res.Status)
				return res, nil
			}

			// If status is "Unknown" (e.g., due to Spider API error), do not attempt to update and just return current status
			// This prevents infinite loops of API calls when Spider is unreachable or returns errors
			// TODO: Consider adding a retry mechanism or alerting for "Unknown" status if it persists for too long
			if res.Status == "Unknown" {
				log.Trace().Msgf("DataDisk %s has Unknown status. It cannot be updated.", res.Id)
				return res, nil
			}

			// Update TB DataDisk object's 'status' field (only for unstable states)
			log.Debug().Msgf("Updating status for dataDisk ID:%s (current: %s)", res.Id, res.Status)
			url := fmt.Sprintf("%s/disk/%s", model.SpiderRestUrl, res.CspResourceName)

			client := clientManager.NewHttpClient()
			client.SetAllowGetMethodPayload(true)

			requestBody := model.SpiderConnectionName{
				ConnectionName: res.ConnectionName,
			}
			method := "GET"
			var callResult model.SpiderDiskInfo

			_, err = clientManager.ExecuteHttpRequest(
				client,
				method,
				url,
				nil,
				clientManager.SetUseBody(requestBody),
				&requestBody,
				&callResult,
				clientManager.MediumDuration,
			)

			if err != nil {
				log.Error().Err(err).Msg("")
				return res, err
			}

			if res.Status != callResult.Status {
				log.Debug().Msgf("DataDisk %s status changed from %s to %s", res.Id, res.Status, callResult.Status)
				res.Status = callResult.Status
				// Only the status is known to have changed here; res was read before the
				// Spider call, so writing it wholesale could revert concurrent updates
				if err := UpdateResourceStatus(nsId, model.StrDataDisk, res.Id, string(callResult.Status)); err != nil {
					log.Warn().Err(err).Msgf("Failed to persist status of dataDisk %s", res.Id)
				}
			}

			return res, nil
		case model.StrObjectStorage:
			res := model.ObjectStorageInfo{}
			err = json.Unmarshal([]byte(keyValue.Value), &res)
			if err != nil {
				log.Error().Err(err).Msg("")
				return nil, err
			}
			return res, nil
		case model.StrRDBMS:
			res := model.RDBMSInfo{}
			err = json.Unmarshal([]byte(keyValue.Value), &res)
			if err != nil {
				log.Error().Err(err).Msg("")
				return nil, err
			}
			return res, nil
		}

		//return true, nil
	}
	errString := "Cannot get " + resourceType + " " + resourceId + "."
	err = fmt.Errorf("%s", errString)
	return nil, err
}

// GenSpecMapKey generates a SpecMap key for storing or accessing data in a map
func GenSpecMapKey(region, specName string) string {
	return strings.ToLower(fmt.Sprintf("%s-%s", region, specName))
}

// GenResourceKey generates a Resource key for concatenating providerName, regionName, zoneName, resourceName
func GetProviderRegionZoneResourceKey(providerName, regionName, zoneName, resourceName string) string {

	div := "+"

	if regionName == "" && zoneName == "" {
		return strings.ToLower(fmt.Sprintf("%s%s%s", providerName, div, resourceName))
	}

	if zoneName == "" {
		return strings.ToLower(fmt.Sprintf("%s%s%s%s%s", providerName, div, regionName, div, resourceName))
	}

	return strings.ToLower(fmt.Sprintf("%s%s%s%s%s%s%s", providerName, div, regionName, div, zoneName, div, resourceName))
}

// ResolveProviderRegionZoneResourceKey resolves the Resource key into providerName, regionName, zoneName, resourceName
func ResolveProviderRegionZoneResourceKey(key string) (providerName string, regionName string, zoneName string, resourceName string, err error) {

	div := "+"

	split := strings.Split(key, div)

	if len(split) == 1 {
		return "", "", "", "", fmt.Errorf("ResourceKey dose not contain div(%s)", div)
	}

	if len(split) == 2 {
		return split[0], "", "", split[1], nil
	}

	if len(split) == 3 {
		return split[0], split[1], "", split[2], nil
	}

	return split[0], split[1], split[2], split[3], nil
}

// creationLockStripes is a fixed-size set of mutexes hashed by resource key, serializing
// check-then-create sequences to avoid racing duplicate creates without growing unbounded
// like a sync.Map keyed per resource would over a long-running server's lifetime.
const creationLockStripes = 256

var creationLocks [creationLockStripes]sync.Mutex

// LockResourceCreation serializes creation of a given (nsId, resourceType, resourceId).
// Call before the existence check in a Create* function; release via defer.
func LockResourceCreation(nsId, resourceType, resourceId string) func() {
	key := common.GenResourceKey(nsId, resourceType, resourceId)
	h := fnv.New32a()
	h.Write([]byte(key))
	mu := &creationLocks[h.Sum32()%creationLockStripes]
	mu.Lock()
	return mu.Unlock
}

// CheckResource returns the existence of the TB Resource resource in bool form.
func CheckResource(nsId string, resourceType string, resourceId string) (bool, error) {

	// Check parameters' emptiness
	if nsId == "" {
		err := fmt.Errorf("failed to check resource, the given nsId is null")
		return false, err
	} else if resourceType == "" {
		err := fmt.Errorf("failed to check resource, the given resourceType is null")
		return false, err
	} else if resourceId == "" {
		err := fmt.Errorf("failed to check resource, the given resourceId is null")
		return false, err
	}

	// Check resourceType's validity
	if strings.EqualFold(resourceType, model.StrImage) ||
		strings.EqualFold(resourceType, model.StrCustomImage) ||
		strings.EqualFold(resourceType, model.StrSSHKey) ||
		strings.EqualFold(resourceType, model.StrSpec) ||
		strings.EqualFold(resourceType, model.StrVNet) ||
		strings.EqualFold(resourceType, model.StrVPN) ||
		strings.EqualFold(resourceType, model.StrRDBMS) ||
		strings.EqualFold(resourceType, model.StrObjectStorage) ||
		strings.EqualFold(resourceType, model.StrSecurityGroup) ||
		strings.EqualFold(resourceType, model.StrDataDisk) {
		//resourceType == "subnet" ||
		//resourceType == "publicIp" ||
		//resourceType == "vNic" {
		// continue
	} else {
		err := fmt.Errorf("invalid resource type")
		return false, err
	}

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	// Handle Image, CustomImage, and Spec using PostgreSQL (GORM)
	switch resourceType {
	case model.StrImage:
		var count int64
		result := model.ORM.Model(&model.ImageInfo{}).Where("namespace = ? AND id = ? AND (resource_type = ? OR resource_type IS NULL OR resource_type = '')",
			nsId, resourceId, model.StrImage).Count(&count)
		if result.Error != nil {
			log.Error().Err(result.Error).Msg("")
			return false, result.Error
		}
		return count > 0, nil

	case model.StrCustomImage:
		var count int64
		result := model.ORM.Model(&model.ImageInfo{}).Where("namespace = ? AND id = ? AND resource_type = ?",
			nsId, resourceId, model.StrCustomImage).Count(&count)
		if result.Error != nil {
			log.Error().Err(result.Error).Msg("")
			return false, result.Error
		}
		return count > 0, nil

	case model.StrSpec:
		var count int64
		result := model.ORM.Model(&model.SpecInfo{}).Where("namespace = ? AND id = ?",
			nsId, resourceId).Count(&count)
		if result.Error != nil {
			log.Error().Err(result.Error).Msg("")
			return false, result.Error
		}
		return count > 0, nil
	}

	// For other resource types, use kvstore (existing code)
	key := common.GenResourceKey(nsId, resourceType, resourceId)

	_, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}
	if exists {
		return true, nil
	}
	return false, nil
}

// CheckChildResource returns the existence of the TB Resource resource in bool form.
func CheckChildResource(nsId string, resourceType string, parentResourceId string, resourceId string) (bool, error) {

	// Check parameters' emptiness
	if nsId == "" {
		err := fmt.Errorf("CheckResource failed; nsId given is null.")
		return false, err
	} else if resourceType == "" {
		err := fmt.Errorf("CheckResource failed; resourceType given is null.")
		return false, err
	} else if parentResourceId == "" {
		err := fmt.Errorf("CheckResource failed; parentResourceId given is null.")
		return false, err
	} else if resourceId == "" {
		err := fmt.Errorf("CheckResource failed; resourceId given is null.")
		return false, err
	}

	var parentResourceType string
	// Check resourceType's validity
	if strings.EqualFold(resourceType, model.StrSubnet) {
		parentResourceType = model.StrVNet
		// continue
	} else {
		err := fmt.Errorf("invalid resource type")
		return false, err
	}

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	err = common.CheckString(parentResourceId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	err = common.CheckString(resourceId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	fmt.Printf("[Check child resource] %s, %s, %s", resourceType, parentResourceId, resourceId)

	key := common.GenResourceKey(nsId, parentResourceType, parentResourceId)
	key += "/" + resourceType + "/" + resourceId

	_, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}
	if exists {
		return true, nil
	}
	return false, nil

}

/*
func convertSpiderResourceToTumblebugResource(resourceType string, i interface{}) (interface{}, error) {
	if resourceType == "" {
		err := fmt.Errorf("CheckResource failed; resourceType given is null.")
		return nil, err
	}

	// Check resourceType's validity
	if resourceType == model.StrImage ||
		resourceType == model.StrSSHKey ||
		resourceType == model.StrSpec ||
		resourceType == model.StrVNet ||
		resourceType == model.StrSecurityGroup {
		//resourceType == "subnet" ||
		//resourceType == "publicIp" ||
		//resourceType == "vNic" {
		// continue
	} else {
		err := fmt.Errorf("invalid resource type")
		return nil, err
	}

}
*/

// https://stackoverflow.com/questions/45139954/dynamic-struct-as-parameter-golang

type IdNameOnly struct {
	Id   string
	Name string
}

// GetIdFromStruct accepts any struct for argument, and returns value of the field 'Id'
func GetIdFromStruct(u any) (string, error) {
	jsonInByteStream, err := json.Marshal(u)
	if err != nil {
		return "", err
	}

	idStruct := IdNameOnly{}
	json.Unmarshal(jsonInByteStream, &idStruct)

	return idStruct.Id, nil
}

// GetNameFromStruct accepts any struct for argument, and returns value of the field 'Name'
func GetNameFromStruct(u any) (string, error) {
	jsonInByteStream, err := json.Marshal(u)
	if err != nil {
		return "", err
	}

	idStruct := IdNameOnly{}
	json.Unmarshal(jsonInByteStream, &idStruct)

	return idStruct.Name, nil
}

// LoadAssets is to register common resources from asset files (../assets/*.csv)
// includeAzure: if true, Azure images will be fetched (may take 40+ minutes)
// LoadAssets fetches specs and images into the system namespace.
//
// targetProviders narrows the run to the named providers (as listed by GET /provider,
// i.e. cloudinfo keys, so a single OpenStack instance can be targeted rather than every
// OpenStack). Fetching every provider takes 10-40 minutes, which is a poor trade when
// only one newly registered CSP needs its catalog. When it is set, includeAzure no
// longer applies: the caller has already said exactly what to fetch.
func LoadAssets(includeAzure bool, targetProviders []string) (*model.IdList, error) {

	regiesteredIds := &model.IdList{}

	// Check common namespace. Create one if not.
	_, err := common.GetNs(model.SystemCommonNs)
	if err != nil {
		nsReq := model.NsReq{}
		nsReq.Name = model.SystemCommonNs
		nsReq.Description = "Namespace for common resources"
		_, nsErr := common.CreateNs(context.Background(), &nsReq)
		if nsErr != nil {
			log.Error().Err(nsErr).Msg("")
			return regiesteredIds, nsErr
		}
	}

	startTime := time.Now()

	reqBodySpecFetchOption := &model.SpecFetchOption{TargetProviders: targetProviders}
	if len(targetProviders) > 0 {
		log.Info().Strs("providers", targetProviders).Msg("Loading assets for the selected providers only")
	}

	resultFetchSpecsForAllConnConfigs, err := FetchSpecsForAllConnConfigs(model.SystemCommonNs, reqBodySpecFetchOption)
	if err != nil {
		log.Error().Err(err).Msg("FetchImagesForAllConnConfigs failed")
	}
	elapsedFetchSpec := time.Since(startTime)
	log.Debug().Msgf("resultFetchSpecsForAllConnConfigs.RegisteredSpecs: %+v elapsed: [%s]", resultFetchSpecsForAllConnConfigs.RegisteredSpecs, elapsedFetchSpec)

	startTime = time.Now()
	err = UpdateSpecsFromAsset(model.SystemCommonNs)
	if err != nil {
		log.Error().Err(err).Msg("UpdateSpecsFromAsset failed")
	}
	elapsedUpdateSpec := time.Since(startTime)
	log.Info().Msgf("UpdateSpecsFromAsset. Elapsed [%s]", elapsedUpdateSpec)

	// Skip spec cleanup for now (will examine later)
	// TODO: Re-enable UpdateExistingSpecListByAvailableRegionZones after examination
	log.Info().Msg("Skipping UpdateExistingSpecListByAvailableRegionZones for Alibaba (temporarily disabled for examination)")

	// Start image fetching (keeping this part running)
	startTime = time.Now()
	reqBodyImageFetchOption := &model.ImageFetchOption{}

	// Configure Azure inclusion based on parameter
	if len(targetProviders) > 0 {
		// An explicit provider list wins: excluding/including Azure is meaningless
		// once the caller has named the providers to fetch.
		reqBodyImageFetchOption.TargetProviders = targetProviders
		reqBodyImageFetchOption.RegionAgnosticProviders = []string{csp.GCP, csp.Azure}
	} else if includeAzure {
		log.Info().Msg("Azure images will be fetched (this may take 40+ minutes)")
		// When including Azure, add it to RegionAgnosticProviders
		reqBodyImageFetchOption.RegionAgnosticProviders = []string{csp.GCP, csp.Azure}
		reqBodyImageFetchOption.ExcludedProviders = []string{} // Don't exclude any providers
	} else {
		log.Info().Msg("Azure images will be excluded (default behavior for faster initialization)")
		// Default behavior: exclude Azure, use GCP as region-agnostic
		reqBodyImageFetchOption.ExcludedProviders = []string{csp.Azure}
		reqBodyImageFetchOption.RegionAgnosticProviders = []string{csp.GCP}
	}
	resultFetchImagesForAllConnConfigs, err := FetchImagesForAllConnConfigs(model.SystemCommonNs, reqBodyImageFetchOption)
	if err != nil {
		log.Error().Err(err).Msg("FetchImagesForAllConnConfigs failed")
	}
	elapsedFetchImg := time.Since(startTime)
	log.Debug().Msgf("resultFetchImagesForAllConnConfigs.RegisteredImages: %+v elapsed: [%s]", resultFetchImagesForAllConnConfigs.RegisteredImages, elapsedFetchImg)

	// Force garbage collection for large cleanup
	runtime.GC()

	startTime = time.Now()
	resultUpdateImagesFromAsset, err := UpdateImagesFromAsset(model.SystemCommonNs)
	if err != nil {
		log.Error().Err(err).Msg("UpdateImagesFromAsset failed")
	}
	log.Debug().Msgf("resultUpdateImagesFromAsset: %+v", resultUpdateImagesFromAsset)

	elapsedUpdateImg := time.Since(startTime)

	// Force garbage collection for large cleanup
	runtime.GC()

	// waitSpecImg.Wait()
	// sort.Strings(regiesteredIds.IdList)
	//log.Info().Msgf("Registered Common Resources %d", len(regiesteredIds.IdList))

	log.Info().Msgf("Fetched Spec List. Elapsed [%s]", elapsedFetchSpec)
	log.Info().Msgf("Updated Spec List. Elapsed [%s]", elapsedUpdateSpec)
	log.Info().Msgf("Image fetching completed. Elapsed [%s]", elapsedFetchImg)
	log.Info().Msgf("Updated Image List. Elapsed [%s]", elapsedUpdateImg)

	// FetchPriceForAllConnConfigs is called to update the prices of all specs
	log.Info().Msgf("FetchPriceForAllConnConfigs is called to update the prices of all specs")
	// FetchPriceForAllConnConfigs() will be called in the end of this function in background
	//go FetchPriceForAllConnConfigs()

	return regiesteredIds, nil
}

// ToNamingRuleCompatible function transforms a given string to match the regex pattern [a-z]([-a-z0-9]*[a-z0-9])?.
func ToNamingRuleCompatible(rawName string) string {
	// Convert all uppercase letters to lowercase
	rawName = strings.ToLower(rawName)

	// // Replace all non-alphanumeric characters with '-'
	// nonAlphanumericRegex := regexp.MustCompile(`[^a-z0-9]+`)
	// rawName = nonAlphanumericRegex.ReplaceAllString(rawName, "-")

	// // Remove leading and trailing '-' from the result string
	// trimLeadingTrailingDashRegex := regexp.MustCompile(`^-+|-+$`)
	// rawName = trimLeadingTrailingDashRegex.ReplaceAllString(rawName, "")

	return rawName
}

// UpdateResourceObject is func to update the resource object
func UpdateResourceObject(nsId string, resourceType string, resourceObject any) {
	resourceId, err := GetIdFromStruct(resourceObject)
	if resourceId == "" || err != nil {
		log.Debug().Msgf("in UpdateResourceObject; failed to extract resourceId") // for debug
		return
	}

	// Handle Image, CustomImage, and Spec using PostgreSQL (GORM)
	switch resourceType {
	case model.StrImage, model.StrCustomImage:
		imageInfo, ok := resourceObject.(model.ImageInfo)
		if !ok {
			log.Debug().Msgf("Failed to convert resourceObject to ImageInfo")
			return
		}

		var whereClause string
		if resourceType == model.StrImage {
			whereClause = "namespace = ? AND id = ? AND (resource_type = ? OR resource_type IS NULL OR resource_type = '')"
		} else {
			whereClause = "namespace = ? AND id = ? AND resource_type = ?"
		}

		result := model.ORM.Model(&model.ImageInfo{}).Where(whereClause, nsId, resourceId, resourceType).Updates(&imageInfo)
		if result.Error != nil {
			log.Error().Err(result.Error).Msgf("Failed to update %s in database", resourceType)
		}
		return

	case model.StrSpec:
		specInfo, ok := resourceObject.(model.SpecInfo)
		if !ok {
			log.Debug().Msgf("Failed to convert resourceObject to SpecInfo")
			return
		}

		result := model.ORM.Model(&model.SpecInfo{}).Where("namespace = ? AND id = ?", nsId, resourceId).Updates(&specInfo)
		if result.Error != nil {
			log.Error().Err(result.Error).Msg("Failed to update spec in database")
		}
		return
	}

	// For other resource types, use kvstore (existing code)
	key := common.GenResourceKey(nsId, resourceType, resourceId)

	// Check existence of the key. If no key, no update.
	keyValue, exists, err := kvstore.GetKv(key)
	if !exists || err != nil {
		return
	}

	// Implementation 2
	var oldObject any
	err = json.Unmarshal([]byte(keyValue.Value), &oldObject)
	if err != nil {
		log.Error().Err(err).Msg("")
	}

	if !reflect.DeepEqual(oldObject, resourceObject) {
		val, _ := json.Marshal(resourceObject)
		// Callers pass a snapshot taken earlier, so a whole-object write would revert
		// associatedObjectList changes made meanwhile by UpdateAssociatedObjectList
		// (the only writer of that field). Keep the stored value.
		val = preserveAssociatedObjectList(keyValue.Value, val)
		err = kvstore.Put(key, string(val))
		if err != nil {
			log.Error().Err(err).Msg("")
		}
	}

}

// PutResourceObject writes a resource record to the kvstore while keeping the stored
// associatedObjectList, which is maintained separately by UpdateAssociatedObjectList.
// Use it instead of kvstore.Put when writing a whole resource snapshot.
func PutResourceObject(key string, value []byte) error {
	if keyValue, exists, err := kvstore.GetKv(key); err == nil && exists {
		value = preserveAssociatedObjectList(keyValue.Value, value)
	}
	return kvstore.Put(key, string(value))
}

// UpdateResourceStatus updates only the status field of a stored resource, leaving
// every other field as stored. Use it instead of writing a whole snapshot back when
// only the status is known to have changed.
func UpdateResourceStatus(nsId string, resourceType string, resourceId string, status string) error {
	key := common.GenResourceKey(nsId, resourceType, resourceId)
	keyValue, exists, err := kvstore.GetKv(key)
	if err != nil || !exists {
		return err
	}
	updated, err := sjson.Set(keyValue.Value, "status", status)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	return kvstore.Put(key, updated)
}

func expandInfraType(infraType string) string {
	expInfraTypeList := []string{}
	lowerInfraType := strings.ToLower(infraType)

	if strings.Contains(lowerInfraType, model.StrNode) {
		expInfraTypeList = append(expInfraTypeList, model.StrNode)
	}
	if strings.Contains(lowerInfraType, model.StrK8s) ||
		strings.Contains(lowerInfraType, model.StrKubernetes) ||
		strings.Contains(lowerInfraType, model.StrContainer) {
		expInfraTypeList = append(expInfraTypeList, model.StrK8s)
		expInfraTypeList = append(expInfraTypeList, model.StrKubernetes)
		expInfraTypeList = append(expInfraTypeList, model.StrContainer)
	}

	return strings.Join(expInfraTypeList, "|")
}

// specNameCache caches successful (nsId, specId) → CspSpecName lookups.
// CspSpecName is immutable once stored, so process-lifetime caching is safe.
var specNameCache sync.Map // key: "nsId/specId", value: string

// WarmSpecNameCache pre-populates the cache for a given namespace/specId with a known CspSpecName.
// Used by provisioning callers that resolve via a fallback namespace to avoid repeated miss queries.
func WarmSpecNameCache(nsId, specId, cspSpecName string) {
	key := strings.ToLower(nsId) + "/" + strings.ToLower(specId)
	specNameCache.Store(key, cspSpecName)
}

// GetCspResourceName is func to retrieve CSP native resource ID
func GetCspResourceName(nsId string, resourceType string, resourceId string) (string, error) {

	if strings.EqualFold(resourceType, model.StrSpec) {
		cacheKey := strings.ToLower(nsId) + "/" + strings.ToLower(resourceId)
		if cached, ok := specNameCache.Load(cacheKey); ok {
			return cached.(string), nil
		}
		specInfo, err := GetSpec(nsId, resourceId)
		if err != nil {
			return "", err
		}
		specNameCache.Store(cacheKey, specInfo.CspSpecName)
		return specInfo.CspSpecName, nil
	}
	if strings.EqualFold(resourceType, model.StrImage) {
		imageInfo, err := GetImage(nsId, resourceId)
		if err != nil {
			return "", err
		}
		return imageInfo.CspImageName, nil
	}

	key := common.GenResourceKey(nsId, resourceType, resourceId)
	if key == "/invalidKey" {
		return "", fmt.Errorf("invalid nsId or resourceType or resourceId")
	}
	keyValue, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}

	if !exists {
		//log.Error().Err(err).Msg("")
		// if there is no matched value for the key, return empty string. Error will be handled in a parent function
		return "", fmt.Errorf("cannot find the key %s", key)
	}

	// A resource with pending/unconfirmed deletion must not be resolved for use
	var tombstoneProbe struct {
		Status              string `json:"status"`
		DeletionRequestedAt string `json:"deletionRequestedAt"`
	}
	if json.Unmarshal([]byte(keyValue.Value), &tombstoneProbe) == nil && tombstoneProbe.DeletionRequestedAt != "" {
		return "", fmt.Errorf("%s '%s' has a pending/unconfirmed deletion (status=%s); retry DELETE to complete it or recreate under another name",
			resourceType, resourceId, tombstoneProbe.Status)
	}

	switch resourceType {
	case model.StrCustomImage:
		content := model.ResourceIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspResourceName, nil
	case model.StrSSHKey:
		content := model.ResourceIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspResourceName, nil
	case model.StrVNet:
		content := model.ResourceIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspResourceName, nil // contains CspResourceId
	case model.StrSecurityGroup:
		content := model.ResourceIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspResourceName, nil
	case model.StrDataDisk:
		content := model.ResourceIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspResourceName, nil

	default:
		return "", fmt.Errorf("invalid resourceType")
	}
}

// GetCspResourceId is func to retrieve CSP native resource ID (SystemId)
func GetCspResourceId(nsId string, resourceType string, resourceId string) (string, error) {

	if strings.EqualFold(resourceType, model.StrSpec) {
		specInfo, err := GetSpec(nsId, resourceId)
		if err != nil {
			return "", err
		}
		return specInfo.CspSpecName, nil // For Spec, name and id are the same
	}
	if strings.EqualFold(resourceType, model.StrImage) || strings.EqualFold(resourceType, model.StrCustomImage) {
		imageInfo, err := GetImage(nsId, resourceId)
		if err != nil {
			return "", err
		}
		if imageInfo.ResourceType == model.StrCustomImage {
			return imageInfo.CspImageId, nil // For CustomImage, CspImageId should be used
		}
		return imageInfo.CspImageName, nil // For Image, CspImageName should be used
	}

	key := common.GenResourceKey(nsId, resourceType, resourceId)
	if key == "/invalidKey" {
		return "", fmt.Errorf("invalid nsId or resourceType or resourceId")
	}
	keyValue, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}

	if !exists {
		//log.Error().Err(err).Msg("")
		// if there is no matched value for the key, return empty string. Error will be handled in a parent function
		return "", fmt.Errorf("cannot find the key %s", key)
	}

	// need to handle subnet in a different way

	switch resourceType {
	case model.StrCustomImage:
		content := model.ResourceIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspResourceId, nil // Return CspResourceId instead of CspResourceName
	case model.StrSSHKey:
		content := model.ResourceIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspResourceId, nil
	case model.StrVNet:
		content := model.ResourceIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspResourceId, nil
	case model.StrSecurityGroup:
		content := model.ResourceIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspResourceId, nil
	case model.StrDataDisk:
		content := model.ResourceIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspResourceId, nil

	default:
		return "", fmt.Errorf("invalid resourceType")
	}
}
