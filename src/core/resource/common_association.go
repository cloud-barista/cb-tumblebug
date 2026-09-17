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
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"sync"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/rs/zerolog/log"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var assocMutex sync.Mutex

// GetAssociatedObjectCount returns the number of Resource's associated Tumblebug objects
func GetAssociatedObjectCount(nsId string, resourceType string, resourceId string) (int, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return -1, err
	}

	err = common.CheckString(resourceId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return -1, err
	}
	check, err := CheckResource(nsId, resourceType, resourceId)

	if !check {
		errString := "The " + resourceType + " " + resourceId + " does not exist."
		err := fmt.Errorf("%s", errString)
		return -1, err
	}

	if err != nil {
		log.Error().Err(err).Msg("")
		return -1, err
	}

	key := common.GenResourceKey(nsId, resourceType, resourceId)

	keyValue, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return -1, err
	}
	if exists {
		inUseCount := int(gjson.Get(keyValue.Value, "associatedObjectList.#").Int())
		return inUseCount, nil
	}
	errString := "Cannot get " + resourceType + " " + resourceId + "."
	err = fmt.Errorf("%s", errString)
	return -1, err
}

// GetAssociatedObjectList returns the list of Resource's associated Tumblebug objects
func GetAssociatedObjectList(nsId string, resourceType string, resourceId string) ([]string, error) {

	var result []string

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	err = common.CheckString(resourceId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	check, err := CheckResource(nsId, resourceType, resourceId)

	if !check {
		errString := "The " + resourceType + " " + resourceId + " does not exist."
		err := fmt.Errorf("%s", errString)
		return nil, err
	}

	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	// Image, CustomImage, and Spec are stored in PostgreSQL, not in kvstore.
	// They do not maintain an associatedObjectList in ETCD, so return empty here.
	switch resourceType {
	case model.StrImage, model.StrCustomImage, model.StrSpec:
		return []string{}, nil
	}

	key := common.GenResourceKey(nsId, resourceType, resourceId)

	keyValue, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	if exists {

		type stringList struct {
			AssociatedObjectList []string `json:"associatedObjectList"`
		}
		res := stringList{}
		err = json.Unmarshal([]byte(keyValue.Value), &res)
		if err != nil {
			log.Error().Err(err).Msg("")
			return nil, err
		}
		result = res.AssociatedObjectList

		return result, nil
	}
	errString := "Cannot get " + resourceType + " " + resourceId + "."
	err = fmt.Errorf("%s", errString)
	return nil, err
}

// UpdateAssociatedObjectList adds or deletes the objectKey (currently, nodeKey) to/from TB object's associatedObjectList
func UpdateAssociatedObjectList(nsId string, resourceType string, resourceId string, cmd string, objectKey string) ([]string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	// err = common.CheckString(resourceId)
	// if err != nil {
	// 	log.Error().Err(err).Msg("")
	// 	return nil, err
	// }
	/*
		check, err := CheckResource(nsId, resourceType, resourceId)

		if !check {
			errString := "The " + resourceType + " " + resourceId + " does not exist."
			err := fmt.Errorf(errString)
			return -1, err
		}

		if err != nil {
			log.Error().Err(err).Msg("")
			return -1, err
		}
	*/
	log.Trace().Msg("[Set count] " + resourceType + ", " + resourceId)

	key := common.GenResourceKey(nsId, resourceType, resourceId)

	keyValue, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	if exists {
		objList, _ := GetAssociatedObjectList(nsId, resourceType, resourceId)
		switch cmd {
		case model.StrAdd:
			if slices.Contains(objList, objectKey) {
				errString := objectKey + " is already associated with " + resourceType + " " + resourceId + "."
				err = fmt.Errorf("%s", errString)
				return nil, err
			}
			var anyJson map[string]any
			json.Unmarshal([]byte(keyValue.Value), &anyJson)
			if anyJson["associatedObjectList"] == nil {
				arrayToBe := []string{objectKey}

				anyJson["associatedObjectList"] = arrayToBe
			} else { // anyJson["associatedObjectList"] != nil
				arrayAsIs := anyJson["associatedObjectList"].([]any)

				arrayToBe := append(arrayAsIs, objectKey)

				anyJson["associatedObjectList"] = arrayToBe
			}
			updatedJson, _ := json.Marshal(anyJson)

			keyValue.Value = string(updatedJson)
		case model.StrDelete:
			var foundKey int
			var foundVal string
			for k, v := range objList {
				if v == objectKey {
					foundKey = k
					foundVal = v
					break
				}
			}
			if foundVal == "" {
				errString := "Cannot find the associated object " + objectKey + "."
				err = fmt.Errorf("%s", errString)
				return nil, err
			} else {
				keyValue.Value, err = sjson.Delete(keyValue.Value, "associatedObjectList."+strconv.Itoa(foundKey))
				if err != nil {
					log.Error().Err(err).Msg("")
					return nil, err
				}
			}
		}

		if err != nil {
			log.Error().Err(err).Msg("")
			return nil, err
		}
		err = kvstore.Put(key, keyValue.Value)
		if err != nil {
			log.Error().Err(err).Msg("")
			return nil, err
		}

		result, _ := GetAssociatedObjectList(nsId, resourceType, resourceId)
		return result, nil
	}
	errString := "Cannot get " + resourceType + " " + resourceId + "."
	err = fmt.Errorf("%s", errString)
	return nil, err
}

// BatchRemoveFromAssociatedObjectList removes multiple objectKeys from a resource's
// associatedObjectList in a single read-modify-write, avoiding N round-trips for N nodes.
// Keys not present in the list are silently skipped. Returns nil if the resource does not exist.
func BatchRemoveFromAssociatedObjectList(nsId string, resourceType string, resourceId string, objectKeys []string) error {
	if len(objectKeys) == 0 {
		return nil
	}

	assocMutex.Lock()
	defer assocMutex.Unlock()

	key := common.GenResourceKey(nsId, resourceType, resourceId)
	keyValue, exists, err := kvstore.GetKv(key)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	toRemove := make(map[string]struct{}, len(objectKeys))
	for _, k := range objectKeys {
		toRemove[k] = struct{}{}
	}

	var anyJson map[string]any
	if err := json.Unmarshal([]byte(keyValue.Value), &anyJson); err != nil {
		return err
	}

	raw, ok := anyJson["associatedObjectList"]
	if !ok || raw == nil {
		return nil
	}
	existing, ok := raw.([]any)
	if !ok {
		return nil
	}

	filtered := make([]any, 0, len(existing))
	for _, v := range existing {
		s, ok := v.(string)
		if ok {
			if _, remove := toRemove[s]; !remove {
				filtered = append(filtered, v)
			}
		}
	}
	anyJson["associatedObjectList"] = filtered

	updated, err := json.Marshal(anyJson)
	if err != nil {
		return err
	}
	return kvstore.Put(key, string(updated))
}

// BatchAddToAssociatedObjectList adds multiple objectKeys to a resource's
// associatedObjectList in a single read-modify-write, avoiding N round-trips for N nodes.
// Duplicate keys are skipped. Returns nil if the resource does not exist.
func BatchAddToAssociatedObjectList(nsId string, resourceType string, resourceId string, objectKeys []string) error {
	if len(objectKeys) == 0 {
		return nil
	}

	assocMutex.Lock()
	defer assocMutex.Unlock()

	key := common.GenResourceKey(nsId, resourceType, resourceId)
	keyValue, exists, err := kvstore.GetKv(key)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	var anyJson map[string]any
	if err := json.Unmarshal([]byte(keyValue.Value), &anyJson); err != nil {
		return err
	}

	existingSet := make(map[string]struct{})
	existingSlice := make([]any, 0)
	if raw, ok := anyJson["associatedObjectList"]; ok && raw != nil {
		if arr, ok := raw.([]any); ok {
			existingSlice = arr
			for _, v := range arr {
				if s, ok := v.(string); ok {
					existingSet[s] = struct{}{}
				}
			}
		}
	}

	changed := false
	for _, k := range objectKeys {
		if k == "" {
			continue
		}
		if _, exists := existingSet[k]; !exists {
			existingSet[k] = struct{}{}
			existingSlice = append(existingSlice, k)
			changed = true
		}
	}

	if !changed {
		return nil
	}

	anyJson["associatedObjectList"] = existingSlice
	updated, err := json.Marshal(anyJson)
	if err != nil {
		return err
	}
	return kvstore.Put(key, string(updated))
}


// preserveAssociatedObjectList carries the stored associatedObjectList over into the
// value about to be written, so a whole-object write cannot drop associations.
func preserveAssociatedObjectList(storedValue string, newValue []byte) []byte {
	var stored map[string]any
	if err := json.Unmarshal([]byte(storedValue), &stored); err != nil {
		return newValue
	}
	assoc, ok := stored["associatedObjectList"]
	if !ok {
		return newValue
	}
	var updated map[string]any
	if err := json.Unmarshal(newValue, &updated); err != nil {
		return newValue
	}
	updated["associatedObjectList"] = assoc
	merged, err := json.Marshal(updated)
	if err != nil {
		return newValue
	}
	return merged
}

