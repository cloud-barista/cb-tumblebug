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
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/rs/zerolog/log"
)

func generateProvisioningLogKey(specId string) string {
	// URL encode the specId to handle special characters like "+" in "gcp+europe-north1+f1-micro"
	encodedSpecId := url.QueryEscape(specId)
	return fmt.Sprintf("/log/provision/%s", encodedSpecId)
}

// GetProvisioningLog retrieves provisioning log for a specific spec ID
func GetProvisioningLog(specId string) (*model.ProvisioningLog, error) {
	log.Debug().Msgf("Getting provisioning log for spec: %s", specId)

	key := generateProvisioningLogKey(specId)
	keyValue, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to get provisioning log for spec: %s", specId)
		return nil, fmt.Errorf("failed to get provisioning log: %w", err)
	}
	if !exists {
		log.Debug().Msgf("No provisioning log found for spec: %s", specId)
		return nil, nil // No log exists yet
	}
	// Check if the value is empty or invalid
	if keyValue.Value == "" {
		log.Debug().Msgf("Empty value found for provisioning log spec: %s, treating as no log exists", specId)
		return nil, nil
	}

	// Check if the value is valid JSON by trying to parse it
	var rawJson json.RawMessage
	if err := json.Unmarshal([]byte(keyValue.Value), &rawJson); err != nil {
		log.Warn().Err(err).Msgf("Invalid JSON found for provisioning log spec: %s, deleting corrupted entry", specId)
		// Delete the corrupted entry
		if deleteErr := kvstore.Delete(key); deleteErr != nil {
			log.Error().Err(deleteErr).Msgf("Failed to delete corrupted provisioning log for spec: %s", specId)
		}
		return nil, nil // Treat as no log exists
	}

	var provisioningLog model.ProvisioningLog
	err = json.Unmarshal([]byte(keyValue.Value), &provisioningLog)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to unmarshal provisioning log for spec: %s", specId)
		// Delete the corrupted entry as a fallback
		if deleteErr := kvstore.Delete(key); deleteErr != nil {
			log.Error().Err(deleteErr).Msgf("Failed to delete corrupted provisioning log for spec: %s", specId)
		}
		return nil, nil // Treat as no log exists instead of returning error
	}

	log.Debug().Msgf("Successfully retrieved provisioning log for spec: %s (failures: %d, successes: %d)",
		specId, provisioningLog.FailureCount, provisioningLog.SuccessCount)
	return &provisioningLog, nil
}

// SaveProvisioningLog saves or updates provisioning log for a specific spec ID
func SaveProvisioningLog(provisioningLog *model.ProvisioningLog) error {
	log.Debug().Msgf("Saving provisioning log for spec: %s", provisioningLog.SpecId)

	provisioningLog.LastUpdated = time.Now()

	key := generateProvisioningLogKey(provisioningLog.SpecId)
	value, err := json.Marshal(provisioningLog)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to marshal provisioning log for spec: %s", provisioningLog.SpecId)
		return fmt.Errorf("failed to marshal provisioning log: %w", err)
	}

	err = kvstore.Put(key, string(value))
	if err != nil {
		log.Error().Err(err).Msgf("Failed to save provisioning log for spec: %s", provisioningLog.SpecId)
		return fmt.Errorf("failed to save provisioning log: %w", err)
	}

	log.Debug().Msgf("Successfully saved provisioning log for spec: %s", provisioningLog.SpecId)
	return nil
}

// DeleteProvisioningLog deletes provisioning log for a specific spec ID
func DeleteProvisioningLog(specId string) error {
	log.Debug().Msgf("Deleting provisioning log for spec: %s", specId)

	key := generateProvisioningLogKey(specId)
	err := kvstore.Delete(key)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to delete provisioning log for spec: %s", specId)
		return fmt.Errorf("failed to delete provisioning log: %w", err)
	}

	log.Debug().Msgf("Successfully deleted provisioning log for spec: %s", specId)
	return nil
}

// provisioningLogHistoryLimit caps the number of samples retained in each of
// ProvisioningLog's FailureTimestamps/FailureMessages/SuccessTimestamps
// slices. These are keyed per spec (e.g. "aws+eu-west-3+t3a.nano") and shared
// across the whole system, so a spec/AMI combination that keeps failing the
// same way can otherwise append forever and grow this etcd record without
// bound (the same failure mode as unbounded per-VM CommandStatus history).
// FailureCount/SuccessCount remain true lifetime totals; only the sample
// slices are bounded. Override with TB_PROVISIONING_LOG_HISTORY_LIMIT.
var provisioningLogHistoryLimit = func() int {
	if v := os.Getenv("TB_PROVISIONING_LOG_HISTORY_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 30
}()

// trimTimeSliceHistory drops the oldest entries once s grows past limit,
// keeping only the most recent ones.
func trimTimeSliceHistory(s []time.Time, limit int) []time.Time {
	if len(s) > limit {
		return s[len(s)-limit:]
	}
	return s
}

// trimStringSliceHistory drops the oldest entries once s grows past limit,
// keeping only the most recent ones.
func trimStringSliceHistory(s []string, limit int) []string {
	if len(s) > limit {
		return s[len(s)-limit:]
	}
	return s
}

// recordFailureMessage appends newMessage (whitespace-trimmed) to messages
// unless it is an exact repeat of the immediately preceding message, then
// caps the result to the most recent limit entries. A spec that keeps
// failing with the exact same error only grows ProvisioningLog.FailureCount,
// not this sample slice. An empty/blank newMessage is a no-op.
func recordFailureMessage(messages []string, newMessage string, limit int) []string {
	trimmed := strings.TrimSpace(newMessage)
	if trimmed == "" {
		return messages
	}
	if len(messages) > 0 && messages[len(messages)-1] == trimmed {
		return messages
	}
	return trimStringSliceHistory(append(messages, trimmed), limit)
}

// RecordProvisioningEvent records a provisioning event (success or failure) to the log
func RecordProvisioningEvent(event *model.ProvisioningEvent) error {
	log.Debug().Msgf("Recording provisioning event for spec: %s, success: %t", event.SpecId, event.IsSuccess)

	// Get existing log or create new one
	existingLog, err := GetProvisioningLog(event.SpecId)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to get existing provisioning log for spec: %s", event.SpecId)
		return fmt.Errorf("failed to get existing provisioning log: %w", err)
	}

	var provisioningLog *model.ProvisioningLog
	if existingLog == nil {
		// Create new log if it doesn't exist
		log.Debug().Msgf("Creating new provisioning log for spec: %s", event.SpecId)

		// Get spec info to populate connection details
		specInfo, err := resource.GetSpec(model.SystemCommonNs, event.SpecId)
		if err != nil {
			log.Error().Err(err).Msgf("Failed to get spec info for: %s", event.SpecId)
			return fmt.Errorf("failed to get spec info: %w", err)
		}

		provisioningLog = &model.ProvisioningLog{
			SpecId:            event.SpecId,
			ConnectionName:    specInfo.ConnectionName,
			ProviderName:      specInfo.ProviderName,
			RegionName:        specInfo.RegionName,
			FailureCount:      0,
			SuccessCount:      0,
			FailureTimestamps: make([]time.Time, 0),
			SuccessTimestamps: make([]time.Time, 0),
			FailureMessages:   make([]string, 0),
			FailureImages:     make([]string, 0),
			SuccessImages:     make([]string, 0),
			AdditionalInfo:    make(map[string]string),
		}
	} else {
		provisioningLog = existingLog
	}

	// Record the event. FailureCount/SuccessCount are true lifetime totals and
	// always increment; the sample slices below are bounded (trimmed to
	// provisioningLogHistoryLimit) and, for failure messages, deduplicated
	// against the immediately preceding entry so a spec that keeps failing
	// with the exact same error does not grow this record without bound.
	if event.IsSuccess {
		// Only record success if there were previous failures
		if provisioningLog.FailureCount > 0 {
			log.Debug().Msgf("Recording success event for spec: %s (previous failures exist)", event.SpecId)
			provisioningLog.SuccessCount++
			provisioningLog.SuccessTimestamps = trimTimeSliceHistory(
				append(provisioningLog.SuccessTimestamps, event.Timestamp), provisioningLogHistoryLimit)
			if event.CspImageName != "" && !contains(provisioningLog.SuccessImages, event.CspImageName) {
				provisioningLog.SuccessImages = append(provisioningLog.SuccessImages, event.CspImageName)
			}
		} else {
			log.Debug().Msgf("Skipping success event recording for spec: %s (no previous failures)", event.SpecId)
			return nil // Don't record success if no previous failures
		}
	} else {
		// Always record failures
		log.Debug().Msgf("Recording failure event for spec: %s", event.SpecId)
		provisioningLog.FailureCount++
		provisioningLog.FailureTimestamps = trimTimeSliceHistory(
			append(provisioningLog.FailureTimestamps, event.Timestamp), provisioningLogHistoryLimit)
		provisioningLog.FailureMessages = recordFailureMessage(
			provisioningLog.FailureMessages, event.ErrorMessage, provisioningLogHistoryLimit)

		if event.CspImageName != "" && !contains(provisioningLog.FailureImages, event.CspImageName) {
			provisioningLog.FailureImages = append(provisioningLog.FailureImages, event.CspImageName)
		}
	}

	// Add additional context information
	if event.InfraId != "" {
		if provisioningLog.AdditionalInfo == nil {
			provisioningLog.AdditionalInfo = make(map[string]string)
		}
		provisioningLog.AdditionalInfo["lastInfraId"] = event.InfraId
	}

	// Save the updated log
	err = SaveProvisioningLog(provisioningLog)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to save provisioning log for spec: %s", event.SpecId)
		return fmt.Errorf("failed to save provisioning log: %w", err)
	}

	log.Debug().Msgf("Successfully recorded provisioning event for spec: %s (total failures: %d, successes: %d)",
		event.SpecId, provisioningLog.FailureCount, provisioningLog.SuccessCount)
	return nil
}

// RecordProvisioningEventsFromInfra analyzes Infra creation result and records provisioning events
func RecordProvisioningEventsFromInfra(nsId string, infraInfo *model.InfraInfo) error {
	log.Debug().Msgf("Recording provisioning events from Infra: %s", infraInfo.Id)

	if infraInfo.CreationErrors == nil {
		log.Debug().Msgf("No creation errors found in Infra: %s, checking for individual VM failures", infraInfo.Id)
	}

	eventCount := 0

	// Process VMs to record events
	for _, node := range infraInfo.Node {
		log.Debug().Msgf("Processing VM: %s, status: %s", node.Id, node.Status)

		// Determine if this VM failed or succeeded based on status
		isSuccess := node.Status == model.StatusRunning
		errorMessage := ""

		if !isSuccess {
			// Look for specific error message in creation errors
			if infraInfo.CreationErrors != nil {
				for _, nodeError := range infraInfo.CreationErrors.NodeCreationErrors {
					if nodeError.NodeName == node.Id || strings.Contains(nodeError.NodeName, node.Id) {
						errorMessage = nodeError.Error
						break
					}
				}
				// Also check VM object creation errors
				for _, nodeError := range infraInfo.CreationErrors.NodeObjectCreationErrors {
					if nodeError.NodeName == node.Id || strings.Contains(nodeError.NodeName, node.Id) {
						errorMessage = nodeError.Error
						break
					}
				}
			}
			// Check VM's SystemMessage for additional error details
			if errorMessage == "" && node.SystemMessage != "" {
				errorMessage = node.SystemMessage
			}
			// If no specific error message found, provide a clearer message
			if errorMessage == "" {
				errorMessage = fmt.Sprintf("VM provisioning failed (status: %s) - check CSP console for details", node.Status)
			}
		}

		// Create provisioning event
		event := &model.ProvisioningEvent{
			SpecId:       node.SpecId,
			CspImageName: node.CspImageName,
			IsSuccess:    isSuccess,
			ErrorMessage: errorMessage,
			Timestamp:    time.Now(),
			NodeName:     node.Id,
			InfraId:      infraInfo.Id,
		}

		// Record the event
		err := RecordProvisioningEvent(event)
		if err != nil {
			log.Error().Err(err).Msgf("Failed to record provisioning event for VM: %s", node.Id)
			continue
		}

		eventCount++
		log.Debug().Msgf("Recorded provisioning event for VM: %s, spec: %s, success: %t",
			node.Id, node.SpecId, isSuccess)
	}

	log.Debug().Msgf("Successfully recorded %d provisioning events from Infra: %s", eventCount, infraInfo.Id)
	return nil
}

// AnalyzeProvisioningRisk analyzes the risk of provisioning failure based on historical data
func AnalyzeProvisioningRisk(specId string, cspImageName string) (riskLevel string, riskMessage string, err error) {
	log.Debug().Msgf("Analyzing provisioning risk for spec: %s, image: %s", specId, cspImageName)

	// Get detailed risk analysis
	riskAnalysis, err := AnalyzeProvisioningRiskDetailed(specId, cspImageName)
	if err != nil {
		return "low", "Unable to analyze provisioning risk", err
	}

	// Return overall risk for backward compatibility
	return riskAnalysis.OverallRisk.Level, riskAnalysis.OverallRisk.Message, nil
}

// AnalyzeProvisioningRiskDetailed provides comprehensive risk analysis with separate spec and image risk assessment
func AnalyzeProvisioningRiskDetailed(specId string, cspImageName string) (*model.RiskAnalysis, error) {
	log.Debug().Msgf("Analyzing detailed provisioning risk for spec: %s, image: %s", specId, cspImageName)

	// Get provisioning log - now handles corrupted data gracefully
	provisioningLog, err := GetProvisioningLog(specId)
	if err != nil {
		log.Warn().Err(err).Msgf("Failed to get provisioning log for spec: %s, treating as no history", specId)
		// Return default low risk analysis
		return &model.RiskAnalysis{
			SpecRisk: model.SpecRiskInfo{
				Level:   "low",
				Message: "Unable to analyze spec history, assuming low risk",
			},
			ImageRisk: model.ImageRiskInfo{
				Level:            "low",
				Message:          "Unable to analyze image history, assuming low risk",
				IsNewCombination: true,
			},
			OverallRisk: model.OverallRiskInfo{
				Level:             "low",
				Message:           "Unable to analyze provisioning history, assuming low risk",
				PrimaryRiskFactor: "none",
			},
			Recommendations: []string{"Monitor this deployment for any issues"},
		}, nil
	}

	// If no log exists, assume low risk
	if provisioningLog == nil {
		log.Debug().Msgf("No provisioning history found for spec: %s", specId)
		return &model.RiskAnalysis{
			SpecRisk: model.SpecRiskInfo{
				Level:   "low",
				Message: "No previous provisioning history available for this spec",
			},
			ImageRisk: model.ImageRiskInfo{
				Level:            "low",
				Message:          "No previous history for this image with this spec",
				IsNewCombination: true,
			},
			OverallRisk: model.OverallRiskInfo{
				Level:             "low",
				Message:           "No previous provisioning history available",
				PrimaryRiskFactor: "none",
			},
			Recommendations: []string{"This is a new spec, monitor deployment closely"},
		}, nil
	}

	totalAttempts := provisioningLog.FailureCount + provisioningLog.SuccessCount
	if totalAttempts == 0 {
		log.Debug().Msgf("No provisioning attempts recorded for spec: %s", specId)
		return &model.RiskAnalysis{
			SpecRisk: model.SpecRiskInfo{
				Level:   "low",
				Message: "No provisioning attempts recorded for this spec",
			},
			ImageRisk: model.ImageRiskInfo{
				Level:            "low",
				Message:          "No attempts with this image on this spec",
				IsNewCombination: true,
			},
			OverallRisk: model.OverallRiskInfo{
				Level:             "low",
				Message:           "No provisioning attempts recorded",
				PrimaryRiskFactor: "none",
			},
			Recommendations: []string{"First deployment with this configuration, proceed with monitoring"},
		}, nil
	}

	failureRate := float64(provisioningLog.FailureCount) / float64(totalAttempts)

	// Check image-specific history
	imageHasFailed := contains(provisioningLog.FailureImages, cspImageName)
	imageHasSucceeded := contains(provisioningLog.SuccessImages, cspImageName)
	isNewCombination := !imageHasFailed && !imageHasSucceeded

	// Count the number of different images that have failed/succeeded with this spec
	failedImageCount := len(provisioningLog.FailureImages)
	succeededImageCount := len(provisioningLog.SuccessImages)

	log.Debug().Msgf("Provisioning analysis for spec %s: failures=%d, successes=%d, rate=%.2f, image_failed=%t, image_succeeded=%t, failed_images=%d, succeeded_images=%d",
		specId, provisioningLog.FailureCount, provisioningLog.SuccessCount, failureRate, imageHasFailed, imageHasSucceeded, failedImageCount, succeededImageCount)

	// Classify failure history to differentiate transient capacity/quota limits from permanent incompatibilities
	hasQuotaCapacity, hasIncompatibility, failureClass := analyzeFailureMessages(
		provisioningLog.ProviderName, provisioningLog.RegionName, provisioningLog.FailureMessages)

	// Analyze spec-specific risk
	specRisk := analyzeSpecRisk(specId, failedImageCount, succeededImageCount, provisioningLog.FailureCount, provisioningLog.SuccessCount, failureRate, hasQuotaCapacity, hasIncompatibility, failureClass)

	// Analyze image-specific risk
	imageRisk := analyzeImageRisk(specId, cspImageName, imageHasFailed, imageHasSucceeded, isNewCombination, hasQuotaCapacity, hasIncompatibility, failureClass)

	// Determine overall risk and primary factor
	overallRisk := determineOverallRisk(specRisk, imageRisk)

	// Generate recommendations
	recommendations := generateRecommendations(specRisk, imageRisk, overallRisk)

	// Get recent unique failure messages for context
	recentFailureMessages := getRecentUniqueFailureMessages(provisioningLog, 5)

	return &model.RiskAnalysis{
		SpecRisk:              specRisk,
		ImageRisk:             imageRisk,
		OverallRisk:           overallRisk,
		Recommendations:       recommendations,
		RecentFailureMessages: recentFailureMessages,
	}, nil
}

// analyzeFailureMessages categorizes past failure messages using the multi-CSP failure classifier.
func analyzeFailureMessages(provider, region string, failureMessages []string) (hasQuotaCapacity bool, hasIncompatibility bool, primaryClass string) {
	if len(failureMessages) == 0 {
		return false, false, ""
	}

	for _, msg := range failureMessages {
		failure := csp.ClassifyProvisioningFailure(provider, region, "", msg)
		switch failure.Class {
		case model.FailureAccountQuota, model.FailureZoneCapacity, model.FailureRegionCapacity, model.FailureThrottling:
			hasQuotaCapacity = true
			if primaryClass == "" {
				primaryClass = failure.Class
			}
		case model.FailureImageSpecMismatch, model.FailureInvalidRequest, model.FailureAuth:
			hasIncompatibility = true
			if primaryClass == "" || !hasIncompatibility {
				primaryClass = failure.Class
			}
		}
	}
	return hasQuotaCapacity, hasIncompatibility, primaryClass
}

// analyzeSpecRisk analyzes risk factors specific to the VM specification
func analyzeSpecRisk(specId string, failedImageCount, succeededImageCount, totalFailures, totalSuccesses int, failureRate float64, hasQuotaCapacity, hasIncompatibility bool, failureClass string) model.SpecRiskInfo {
	var level, message string

	if failedImageCount >= 10 && !hasQuotaCapacity {
		// Very likely spec-level issue: 10+ different images failed
		level = "high"
		message = fmt.Sprintf("Spec '%s': %d different images failed (%.0f%% failure rate) - spec itself may be problematic",
			specId, failedImageCount, failureRate*100)
	} else if failedImageCount >= 5 && !hasQuotaCapacity {
		// Likely spec-level issue: 5+ different images failed
		level = "medium"
		message = fmt.Sprintf("Spec '%s': %d different images failed (%.0f%% failure rate) - check spec compatibility",
			specId, failedImageCount, failureRate*100)
	} else if failedImageCount >= 3 && succeededImageCount == 0 && !hasQuotaCapacity {
		// Potential spec-level issue: 3+ different images failed with no successes
		level = "medium"
		message = fmt.Sprintf("Spec '%s': %d images failed, none succeeded (%.0f%% failure rate)",
			specId, failedImageCount, failureRate*100)
	} else if failureRate >= 0.8 {
		if hasQuotaCapacity && !hasIncompatibility {
			level = "medium"
			message = fmt.Sprintf("Spec '%s' has %.0f%% failure rate (%d failures, %d successes), mainly due to %s",
				specId, failureRate*100, totalFailures, totalSuccesses, failureClass)
		} else {
			level = "high"
			message = fmt.Sprintf("Spec '%s' has %.0f%% failure rate (%d failures out of %d attempts)",
				specId, failureRate*100, totalFailures, totalFailures+totalSuccesses)
		}
	} else if failureRate >= 0.5 {
		level = "medium"
		message = fmt.Sprintf("Spec '%s' has %.0f%% failure rate (%d failures, %d successes)",
			specId, failureRate*100, totalFailures, totalSuccesses)
	} else if failureRate > 0 {
		level = "low"
		message = fmt.Sprintf("Spec '%s' has %.0f%% failure rate (%d failures, %d successes) - mostly stable",
			specId, failureRate*100, totalFailures, totalSuccesses)
	} else {
		level = "low"
		message = fmt.Sprintf("Spec '%s' has 100%% success rate (%d successes, no failures)", specId, totalSuccesses)
	}

	return model.SpecRiskInfo{
		Level:               level,
		Message:             message,
		FailedImageCount:    failedImageCount,
		SucceededImageCount: succeededImageCount,
		TotalFailures:       totalFailures,
		TotalSuccesses:      totalSuccesses,
		FailureRate:         failureRate,
	}
}

// analyzeImageRisk analyzes risk factors specific to the image
func analyzeImageRisk(specId, imageId string, imageHasFailed, imageHasSucceeded, isNewCombination, hasQuotaCapacity, hasIncompatibility bool, failureClass string) model.ImageRiskInfo {
	var level, message string

	if imageHasFailed {
		if !imageHasSucceeded {
			// This specific image has failed before and never succeeded with this spec
			if hasQuotaCapacity && !hasIncompatibility {
				level = "medium"
				message = fmt.Sprintf("Image '%s' with spec '%s' previously hit %s (never succeeded yet, but may succeed if capacity/quota allows)", imageId, specId, failureClass)
			} else {
				level = "high"
				message = fmt.Sprintf("Image '%s' has FAILED with spec '%s' before (never succeeded)", imageId, specId)
			}
		} else {
			// This image has both failed and succeeded with this spec
			level = "medium"
			if hasQuotaCapacity && !hasIncompatibility {
				message = fmt.Sprintf("Image '%s' with spec '%s' has succeeded before (past failures were due to %s)", imageId, specId, failureClass)
			} else {
				message = fmt.Sprintf("Image '%s' with spec '%s' has succeeded before, but also had past failures", imageId, specId)
			}
		}
	} else if imageHasSucceeded && !imageHasFailed {
		// This image has only succeeded with this spec - safest option
		level = "low"
		message = fmt.Sprintf("Image '%s' has succeeded with spec '%s' before (no failures)", imageId, specId)
	} else if isNewCombination {
		// This is a new combination - unknown risk
		level = "low"
		message = fmt.Sprintf("Image '%s' + Spec '%s' is a new combination (no history)", imageId, specId)
	} else {
		// Fallback case
		level = "low"
		message = "No specific image risk identified"
	}

	return model.ImageRiskInfo{
		Level:                level,
		Message:              message,
		HasFailedWithSpec:    imageHasFailed,
		HasSucceededWithSpec: imageHasSucceeded,
		IsNewCombination:     isNewCombination,
	}
}

// determineOverallRisk determines the overall risk based on spec and image risks
func determineOverallRisk(specRisk model.SpecRiskInfo, imageRisk model.ImageRiskInfo) model.OverallRiskInfo {
	var level, message, primaryRiskFactor string

	// Determine the highest risk level
	specRiskValue := getRiskValue(specRisk.Level)
	imageRiskValue := getRiskValue(imageRisk.Level)

	if specRiskValue >= imageRiskValue {
		level = specRisk.Level
		primaryRiskFactor = "spec"
		if specRiskValue > imageRiskValue {
			message = specRisk.Message
		} else {
			message = fmt.Sprintf("%s | %s", specRisk.Message, imageRisk.Message)
		}
	} else {
		level = imageRisk.Level
		primaryRiskFactor = "image"
		message = imageRisk.Message
	}

	// Special case handling
	if specRisk.Level == "low" && imageRisk.Level == "low" {
		primaryRiskFactor = "none"
		message = imageRisk.Message // Use the detailed image message which includes spec+image info
	} else if imageRisk.IsNewCombination && specRisk.Level != "low" {
		primaryRiskFactor = "combination"
		message = fmt.Sprintf("%s (image untested with this spec)", specRisk.Message)
	}

	return model.OverallRiskInfo{
		Level:             level,
		Message:           message,
		PrimaryRiskFactor: primaryRiskFactor,
	}
}

// generateRecommendations provides actionable guidance based on risk analysis
func generateRecommendations(specRisk model.SpecRiskInfo, imageRisk model.ImageRiskInfo, overallRisk model.OverallRiskInfo) []string {
	var recommendations []string

	switch overallRisk.PrimaryRiskFactor {
	case "spec":
		if specRisk.Level == "high" {
			recommendations = append(recommendations, "Consider changing to a different VM specification")
			recommendations = append(recommendations, "Check if this spec is available and properly configured in the target region")
			if specRisk.FailedImageCount >= 5 {
				recommendations = append(recommendations, "Multiple images have failed with this spec - likely a spec-level compatibility issue")
			}
		} else if specRisk.Level == "medium" {
			recommendations = append(recommendations, "Monitor deployment closely - this spec has shown some issues")
			recommendations = append(recommendations, "Consider having a backup spec ready")
		}

	case "image":
		if imageRisk.Level == "high" {
			if imageRisk.HasFailedWithSpec && !imageRisk.HasSucceededWithSpec {
				recommendations = append(recommendations, "CRITICAL: This exact spec+image combination has failed before and NEVER succeeded")
				recommendations = append(recommendations, "STRONGLY RECOMMEND: Use a different image immediately")
				recommendations = append(recommendations, "Find alternative images with same OS/application requirements")
			} else if imageRisk.HasFailedWithSpec && imageRisk.HasSucceededWithSpec {
				recommendations = append(recommendations, "HIGH RISK: This exact combination has failed at least once before")
				recommendations = append(recommendations, "CAUTION: Even though it succeeded sometimes, failure history indicates instability")
				recommendations = append(recommendations, "Consider using a more reliable image or test extensively before production")
			}
		} else if imageRisk.Level == "medium" {
			recommendations = append(recommendations, "This image has mixed results with this spec - proceed with caution")
		}

	case "combination":
		recommendations = append(recommendations, "This is a new spec+image combination")
		recommendations = append(recommendations, "Monitor closely as there's no historical data for this combination")
		if specRisk.Level != "low" {
			recommendations = append(recommendations, "Consider that this spec has shown issues with other images")
		}

	case "none":
		recommendations = append(recommendations, "Both spec and image appear safe based on historical data")
		recommendations = append(recommendations, "Continue with standard monitoring")

	default:
		recommendations = append(recommendations, "Monitor deployment and record results for future analysis")
	}

	// Add critical warnings for any failure history
	if imageRisk.HasFailedWithSpec {
		recommendations = append(recommendations, "IMPORTANT: This exact spec+image combination has failure history - high caution advised")
	}

	// Add general recommendations based on risk levels
	if overallRisk.Level == "high" {
		recommendations = append(recommendations, "HIGH RISK DEPLOYMENT - Consider testing in development environment first")
		recommendations = append(recommendations, "Ensure robust rollback plans and monitoring are in place")
	} else if overallRisk.Level == "medium" {
		recommendations = append(recommendations, "Medium risk - ensure proper monitoring and rollback plans are in place")
	}

	return recommendations
}

// getRiskValue converts risk level to numeric value for comparison
func getRiskValue(riskLevel string) int {
	switch riskLevel {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
} // CleanupCorruptedProvisioningLogs removes all corrupted provisioning log entries from kvstore
func CleanupCorruptedProvisioningLogs() error {
	log.Debug().Msg("Starting cleanup of corrupted provisioning logs")

	// Get all keys with provisioning log prefix
	keyPattern := "/log/provision/"
	keys, err := kvstore.GetKvList(keyPattern)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list provisioning log keys")
		return fmt.Errorf("failed to list provisioning log keys: %w", err)
	}

	cleanupCount := 0
	for _, key := range keys {
		// Check if the value is empty or invalid JSON (key already contains Value from GetKvList)
		if key.Value == "" {
			log.Debug().Msgf("Deleting empty provisioning log: %s", key.Key)
			if deleteErr := kvstore.Delete(key.Key); deleteErr != nil {
				log.Error().Err(deleteErr).Msgf("Failed to delete empty log: %s", key.Key)
			} else {
				cleanupCount++
			}
			continue
		}

		// Test JSON validity
		var testLog model.ProvisioningLog
		if err := json.Unmarshal([]byte(key.Value), &testLog); err != nil {
			log.Debug().Msgf("Deleting corrupted provisioning log: %s", key.Key)
			if deleteErr := kvstore.Delete(key.Key); deleteErr != nil {
				log.Error().Err(deleteErr).Msgf("Failed to delete corrupted log: %s", key.Key)
			} else {
				cleanupCount++
			}
		}
	}

	log.Debug().Msgf("Cleanup completed. Removed %d corrupted provisioning logs", cleanupCount)
	return nil
}

// ValidateProvisioningLogIntegrity checks and repairs provisioning log data integrity
func ValidateProvisioningLogIntegrity(specId string) error {
	log.Debug().Msgf("Validating provisioning log integrity for spec: %s", specId)

	key := generateProvisioningLogKey(specId)
	keyValue, _, err := kvstore.GetKv(key)
	if err != nil {
		if err.Error() == "key not found" {
			log.Debug().Msgf("No provisioning log found for spec: %s", specId)
			return nil // No log exists, nothing to validate
		}
		return fmt.Errorf("failed to get provisioning log: %w", err)
	}

	// Check if the value is empty
	if keyValue.Value == "" {
		log.Warn().Msgf("Empty provisioning log found for spec: %s, deleting", specId)
		return kvstore.Delete(key)
	}

	// Test JSON validity
	var testLog model.ProvisioningLog
	if err := json.Unmarshal([]byte(keyValue.Value), &testLog); err != nil {
		log.Warn().Msgf("Corrupted provisioning log found for spec: %s, deleting", specId)
		return kvstore.Delete(key)
	}

	// Validate data consistency. FailureCount/SuccessCount are true lifetime
	// totals while FailureTimestamps/SuccessTimestamps are bounded, most-recent
	// samples (see provisioningLogHistoryLimit), so the timestamp slices are
	// expected to be shorter than the counts once a spec has failed/succeeded
	// more times than the retention limit — that is not corruption. Only a
	// slice LONGER than its count is impossible under normal recording logic
	// and indicates real corruption worth repairing.
	if len(testLog.FailureTimestamps) > testLog.FailureCount || len(testLog.SuccessTimestamps) > testLog.SuccessCount {
		log.Warn().Msgf("Timestamp count exceeds recorded total for spec: %s, repairing", specId)

		// Repair by truncating arrays to match counts
		if len(testLog.FailureTimestamps) > testLog.FailureCount {
			testLog.FailureTimestamps = testLog.FailureTimestamps[:testLog.FailureCount]
		}
		if len(testLog.SuccessTimestamps) > testLog.SuccessCount {
			testLog.SuccessTimestamps = testLog.SuccessTimestamps[:testLog.SuccessCount]
		}

		// Save repaired log
		return SaveProvisioningLog(&testLog)
	}

	log.Debug().Msgf("Provisioning log integrity validated for spec: %s", specId)
	return nil
}

// getRecentUniqueFailureMessages extracts recent, unique failure messages from provisioning log
// Returns up to maxMessages most recent unique failure messages
func getRecentUniqueFailureMessages(provisioningLog *model.ProvisioningLog, maxMessages int) []string {
	if provisioningLog == nil || len(provisioningLog.FailureMessages) == 0 {
		return []string{}
	}

	// Use a map to track unique messages and a slice to maintain order
	uniqueMessages := make(map[string]bool)
	var recentMessages []string

	// Process messages from most recent to oldest (assuming they are stored in chronological order)
	// We'll take the last entries as the most recent ones
	messages := provisioningLog.FailureMessages
	startIdx := max(
		// Look at more messages to find unique ones
		len(messages)-maxMessages*2, 0)

	// Process from the end (most recent) backwards
	for i := len(messages) - 1; i >= startIdx && len(recentMessages) < maxMessages; i-- {
		message := strings.TrimSpace(messages[i])
		if message != "" && !uniqueMessages[message] {
			uniqueMessages[message] = true
			recentMessages = append(recentMessages, message)
		}
	}

	// Reverse the slice to have most recent first
	for i, j := 0, len(recentMessages)-1; i < j; i, j = i+1, j-1 {
		recentMessages[i], recentMessages[j] = recentMessages[j], recentMessages[i]
	}

	return recentMessages
}

// CreateK8sMultiClusterDynamic creates multiple K8sClusters in parallel
