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

// Package label is to handle label selector for resources
package label

import (
	"encoding/json"
	"fmt"
	"strings"

	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvutil"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
)

// CreateOrUpdateLabel adds a new label or updates an existing label for the given resource,
// and then persists the updated label information in the Key-Value store.
func CreateOrUpdateLabel(labelType, uid string, resourceKey string, labels map[string]string) error {
	// Construct the labelKey
	labelKey := fmt.Sprintf("/label/%s/%s", labelType, uid)

	// Fetch the existing model.LabelInfo if it exists
	labelData, err := kvstore.Get(labelKey)
	if err != nil {
		log.Error().Err(err).Msg("failed to get label data from kvstore")
	}

	// log.Debug().Str("labelData", string(labelData)).Msg("Fetched label data")

	// if len(labelData) == 0 {
	// 	log.Debug().Msg("labelData is empty")
	// }
	var labelInfo model.LabelInfo

	if err == nil && len(labelData) > 0 {
		// If label info exists, unmarshal and update it
		err = json.Unmarshal([]byte(labelData), &labelInfo)
		if err != nil {
			return fmt.Errorf("failed to unmarshal existing label data: %w", err)
		}
		for key, value := range labels {
			labelInfo.Labels[key] = value
		}
	} else {
		// If label info does not exist or is empty, create a new one
		labelInfo = model.LabelInfo{
			ResourceKey: resourceKey,
			Labels:      labels,
		}
	}

	if labelType != model.StrVPN { // Skip for VPN
		// if kvstore key has LabelConnectionName, try ListCSPResourceLabel
		if connectionName, exists := labelInfo.Labels[model.LabelConnectionName]; exists && connectionName != "" {
			lbs := ListCSPResourceLabel(labelType, uid, connectionName)
			log.Info().Msgf("ListCSPResourceLabel: %v", lbs)

			// Merge CSP labels with existing labels (existing labels have priority)
			for key, value := range lbs {
				// Only add if key doesn't exist in labelInfo.Labels
				if _, exists := labelInfo.Labels[key]; !exists {
					labelInfo.Labels[key] = value
				}
			}
		}
	}

	// Save the updated model.LabelInfo back to the Key-Value store
	updatedLabelData, err := json.Marshal(labelInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal updated label info: %w", err)
	}

	err = kvstore.Put(labelKey, string(updatedLabelData))
	if err != nil {
		return fmt.Errorf("failed to put label info into kvstore: %w", err)
	}

	if labelType != model.StrVPN { // Skip for VPN
		// if kvstore key has LabelConnectionName, try UpdateCSPResourceLabel
		if connectionName, exists := labelInfo.Labels[model.LabelConnectionName]; exists && connectionName != "" {
			UpdateCSPResourceLabel(labelType, uid, labels, connectionName)
		}
	}
	return nil
}

// DeleteLabelObject deletes the entire label object for a given resource identified by its labelType and uid.
func DeleteLabelObject(labelType, uid string) error {
	// Construct the labelKey
	labelKey := fmt.Sprintf("/label/%s/%s", labelType, uid)

	// Delete the entire label object from the Key-Value store
	err := kvstore.Delete(labelKey)
	if err != nil {
		log.Error().Err(err).Str("labelKey", labelKey).Msg("Failed to delete label object from kvstore")
		return fmt.Errorf("failed to delete label object: %w", err)
	}

	log.Info().Str("labelKey", labelKey).Msg("Label object successfully deleted from kvstore")
	return nil
}

// RemoveLabel removes a label from a resource identified by its uid.
func RemoveLabel(labelType, uid, key string) error {
	// Construct the labelKey
	labelKey := fmt.Sprintf("/label/%s/%s", labelType, uid)

	// Fetch the existing model.LabelInfo
	labelData, err := kvstore.Get(labelKey)
	if err != nil {
		log.Error().Err(err).Msgf("labelData: %v", labelData)
		return err
	}

	if labelData == "" {
		err = fmt.Errorf("does not exist, label object for %s", labelKey)
		log.Warn().Msg(err.Error())
		return err
	}

	var labelInfo model.LabelInfo
	err = json.Unmarshal([]byte(labelData), &labelInfo)
	if err != nil {
		log.Error().Err(err).Msgf("labelInfo: %v", labelInfo)
		return err
	}

	// Remove the label
	delete(labelInfo.Labels, key)

	// if kvstore key has LabelConnectionName, try UpdateCSPResourceLabel
	if connectionName, exists := labelInfo.Labels[model.LabelConnectionName]; exists && connectionName != "" {
		RemoveCSPResourceLabel(labelType, uid, key, connectionName)
	}

	// Save the updated model.LabelInfo back to the Key-Value store
	updatedLabelData, err := json.Marshal(labelInfo)
	if err != nil {
		log.Error().Err(err).Msgf("updatedLabelData: %v", updatedLabelData)
		return err
	}

	err = kvstore.Put(labelKey, string(updatedLabelData))
	if err != nil {
		log.Error().Err(err).Msgf("")
		return err
	}

	return nil
}

// GetLabels retrieves the labels for a resource identified by its uid.
func GetLabels(labelType, uid string) (label model.LabelInfo, err error) {
	labelInfo := model.LabelInfo{}

	// Construct the labelKey
	labelKey := fmt.Sprintf("/label/%s/%s", labelType, uid)

	// Fetch the existing model.LabelInfo
	labelData, err := kvstore.Get(labelKey)
	if err != nil {
		log.Error().Err(err).Msg("failed to get label data from kvstore")
		return labelInfo, err
	}
	if len(labelData) == 0 {
		log.Debug().Msg("labelData is empty")
		return labelInfo, nil
	}

	err = json.Unmarshal([]byte(labelData), &labelInfo)
	if err != nil {
		log.Error().Err(err).Msg("failed to unmarshal label data")
		return labelInfo, err
	}

	return labelInfo, nil
}

// MatchesLabelSelector checks if the labels match the given label selector.
func MatchesLabelSelector(labels map[string]string, labelSelector string) bool {
	// Split the labelSelector into individual selectors
	selectors := strings.Split(labelSelector, ",")

	for _, selector := range selectors {
		selector = strings.TrimSpace(selector)

		switch {
		case strings.Contains(selector, "!="):
			parts := strings.SplitN(selector, "!=", 2)
			key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
			if val, ok := labels[key]; !ok || val == value {
				return false
			}

		case strings.Contains(selector, "="):
			parts := strings.SplitN(selector, "=", 2)
			key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
			if val, ok := labels[key]; !ok || val != value {
				return false
			}

		case strings.Contains(selector, " in "):
			parts := strings.SplitN(selector, " in ", 2)
			key := strings.TrimSpace(parts[0])
			values := strings.Split(strings.Trim(parts[1], "()"), ",")
			found := false
			if val, ok := labels[key]; ok {
				for _, v := range values {
					if strings.TrimSpace(v) == val {
						found = true
						break
					}
				}
			}
			if !found {
				return false
			}

		case strings.Contains(selector, " notin "):
			parts := strings.SplitN(selector, " notin ", 2)
			key := strings.TrimSpace(parts[0])
			values := strings.Split(strings.Trim(parts[1], "()"), ",")
			if val, ok := labels[key]; ok {
				for _, v := range values {
					if strings.TrimSpace(v) == val {
						return false
					}
				}
			}

		case strings.HasSuffix(selector, " exists"):
			key := strings.TrimSpace(strings.TrimSuffix(selector, " exists"))
			if _, ok := labels[key]; !ok {
				return false
			}

		case strings.HasSuffix(selector, " !exists"):
			key := strings.TrimSpace(strings.TrimSuffix(selector, " !exists"))
			if _, ok := labels[key]; ok {
				return false
			}

		default:
			return false
		}
	}

	return true
}

// GetResourcesByLabelSelector retrieves resources based on a label selector.
func GetResourcesByLabelSelector(labelType, labelSelector string) ([]interface{}, error) {
	var matchedResources []interface{}

	// Fetch all label entries for the resourceType
	listKey := fmt.Sprintf("/label/%s", labelType)
	keyValue, err := kvstore.GetKvList(listKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	// Apply filtering if necessary (assuming kvutil.FilterKvListBy applies some filtering logic)
	keyValue = kvutil.FilterKvListBy(keyValue, listKey, 1)

	// Log the number of filtered label entries
	//log.Debug().Int("numLabelEntries", len(keyValue)).Str("listKey", listKey).Msg("Fetched and filtered list of label entries")

	// Get the appropriate resource type constructor
	resourceConstructor, exists := model.ResourceTypeRegistry[labelType]
	if !exists {
		log.Error().Str("labelType", labelType).Msg("Unsupported label type")
		return nil, fmt.Errorf("unsupported label type: %s", labelType)
	}

	// Iterate over each filtered label entry
	for _, kv := range keyValue {
		labelKey := kv.Key
		labelData := kv.Value

		log.Debug().Str("labelKey", labelKey).Msg("Processing label entry")
		//log.Debug().Str("labelKey", labelKey).Str("labelData", string(labelData)).Msg("Fetched label data")

		var labelInfo model.LabelInfo
		err = json.Unmarshal([]byte(labelData), &labelInfo)
		if err != nil {
			log.Error().Err(err).Str("labelData", string(labelData)).Msg("Failed to unmarshal label data")
			continue // Skip this entry and continue with the next one
		}

		if MatchesLabelSelector(labelInfo.Labels, labelSelector) {
			// Use the resource constructor to create a new resource instance
			resource := resourceConstructor()

			// Fetch the actual resource using the resourceKey
			resourceData, err := kvstore.Get(labelInfo.ResourceKey)
			if err != nil {
				log.Error().Err(err).Str("resourceKey", labelInfo.ResourceKey).Msg("Failed to get resource data")
				continue // Skip this entry and continue with the next one
			}
			if len(resourceData) == 0 {
				log.Debug().Str("resourceKey", labelInfo.ResourceKey).Msg("Resource data is empty")
				continue // Skip this entry and continue with the next
			}

			//log.Debug().Str("resourceKey", labelInfo.ResourceKey).Str("resourceData", string(resourceData)).Msg("Fetched resource data")

			err = json.Unmarshal([]byte(resourceData), resource)
			if err != nil {
				log.Error().Err(err).Str("resourceData", string(resourceData)).Msg("Failed to unmarshal resource data")
				continue // Skip this entry and continue with the next one
			}

			matchedResources = append(matchedResources, resource)
		}
	}

	//log.Debug().Int("numMatchedResources", len(matchedResources)).Str("labelType", labelType).Msg("Matched resources found")
	return matchedResources, nil
}

// UpdateCSPResourceLabel best-effort updates the labels of a resource in the CSP
func UpdateCSPResourceLabel(labelType, uid string, labels map[string]string, connectionName string) {

	client := resty.New()
	url := model.SpiderRestUrl + "/tag"
	method := "POST"
	var callResult model.KeyValue
	requestBody := model.SpiderTagAddRequest{}
	requestBody.ConnectionName = connectionName
	requestBody.ReqInfo.ResourceName = uid
	requestBody.ReqInfo.ResourceType = convertTermToSpider(labelType)

	for key, value := range labels {
		requestBody.ReqInfo.Tag = model.KeyValue{
			Key:   key,
			Value: value,
		}

		err := clientManager.ExecuteHttpRequest(
			client,
			method,
			url,
			nil,
			clientManager.SetUseBody(requestBody),
			&requestBody,
			&callResult,
			clientManager.MediumDuration,
		)

		// this is a best-effort operation, so we don't return an error if it fails
		// drop if we meet the first error
		if err != nil {
			log.Info().Err(err).Msg("Cannot update CSP label/tag")
			break
		}
	}
}

// RemoveCSPResourceLabel best-effort removes the labels of a resource in the CSP
func RemoveCSPResourceLabel(labelType, uid string, key string, connectionName string) {

	client := resty.New()
	url := fmt.Sprintf("%s/tag/%s", model.SpiderRestUrl, key)
	method := "DELETE"
	var callResult model.KeyValue
	requestBody := model.SpiderTagRemoveRequest{}
	requestBody.ConnectionName = connectionName
	requestBody.ReqInfo.ResourceName = uid
	requestBody.ReqInfo.ResourceType = convertTermToSpider(labelType)

	clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&callResult,
		clientManager.MediumDuration,
	)

}

// MergeCSPResourceLabel merges the labels of a resource in the CSP with the existing labels
func MergeCSPResourceLabel(labelType, uid string, resourceKey string) error {
	// Construct the labelKey
	labelKey := fmt.Sprintf("/label/%s/%s", labelType, uid)

	// Fetch the existing model.LabelInfo if it exists
	labelData, err := kvstore.Get(labelKey)
	if err != nil {
		log.Error().Err(err).Msg("failed to get label data from kvstore")
	}

	// log.Debug().Str("labelData", string(labelData)).Msg("Fetched label data")

	// if len(labelData) == 0 {
	// 	log.Debug().Msg("labelData is empty")
	// }
	var labelInfo model.LabelInfo
	err = json.Unmarshal([]byte(labelData), &labelInfo)
	if err != nil {
		return fmt.Errorf("failed to unmarshal existing label data: %w", err)
	}

	// if kvstore key has LabelConnectionName, try ListCSPResourceLabel
	if connectionName, exists := labelInfo.Labels[model.LabelConnectionName]; exists && connectionName != "" {
		lbs := ListCSPResourceLabel(labelType, uid, connectionName)
		log.Info().Msgf("ListCSPResourceLabel: %v", lbs)

		// Merge CSP labels with existing labels (CSP labels have priority)
		for key, value := range lbs {
			labelInfo.Labels[key] = value
		}
	}

	// Save the updated model.LabelInfo back to the Key-Value store
	updatedLabelData, err := json.Marshal(labelInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal updated label info: %w", err)
	}

	err = kvstore.Put(labelKey, string(updatedLabelData))
	if err != nil {
		return fmt.Errorf("failed to put label info into kvstore: %w", err)
	}

	// if kvstore key has LabelConnectionName, try UpdateCSPResourceLabel
	if connectionName, exists := labelInfo.Labels[model.LabelConnectionName]; exists && connectionName != "" {
		UpdateCSPResourceLabel(labelType, uid, labelInfo.Labels, connectionName)
	}
	return nil
}

// ListCSPResourceLabel best-effort lists the labels of a resource in the CSP
func ListCSPResourceLabel(labelType, uid string, connectionName string) (labels map[string]string) {

	type jsonResult struct {
		Result       []model.KeyValue `json:"tag"`
		ResourceType string           `json:"resourceType"`
	}
	resourceType := convertTermToSpider(labelType)
	resourceName := uid

	client := resty.New()
	url := fmt.Sprintf("%s/tag?ConnectionName=%s&ResourceType=%s&ResourceName=%s", model.SpiderRestUrl, connectionName, resourceType, resourceName)
	method := "GET"
	var callResult jsonResult
	requestBody := clientManager.NoBody

	err := clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&callResult,
		clientManager.MediumDuration,
	)
	labels = make(map[string]string)

	if err != nil {
		log.Info().Err(err).Msg("Failed to list CSP resource label")
		return labels
	}

	// Convert []model.KeyValue to map[string]string
	for _, tag := range callResult.Result {
		labels[tag.Key] = tag.Value
	}

	return labels

}

// convertTermToSpider converts internal label type to CSP resource type
func convertTermToSpider(labelType string) string {
	// Spider ResourceType Enum: all, image, vpc, subnet, sg, keypair, vm, nlb, disk, myimage, cluster, nodegroup
	if labelType == model.StrVNet {
		return model.StrVPC
	} else if labelType == model.StrSecurityGroup {
		return model.StrSG
	} else {
		return labelType
	}
}
