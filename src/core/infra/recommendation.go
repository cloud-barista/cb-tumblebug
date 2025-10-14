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

// Package mci is to manage multi-cloud infra
package infra

import (
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/rs/zerolog/log"
)

// toUpperFirst converts the first letter of a string to uppercase
func toUpperFirst(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// applyFilterPolicies dynamically sets filters on the request based on the policies.
func applyFilterPolicies(request *model.FilterSpecsByRangeRequest, plan *model.RecommendSpecReq) error {
	val := reflect.ValueOf(request).Elem()

	for _, policy := range plan.Filter.Policy {
		for _, condition := range policy.Condition {
			fieldName := toUpperFirst(policy.Metric) // Correctly capitalize the first letter
			field := val.FieldByName(fieldName)
			if !field.IsValid() {
				return fmt.Errorf("invalid metric: %s", policy.Metric)
			}
			if err := setFieldCondition(field, condition); err != nil {
				return fmt.Errorf("setting condition failed: %v", err)
			}
		}
	}
	return nil
}

// setFieldCondition applies the specified condition to the field.
func setFieldCondition(field reflect.Value, condition model.Operation) error {
	if field.Kind() == reflect.Struct && (field.Type().Name() == "Range" || field.Type().Name() == "range") {
		operand, err := strconv.ParseFloat(condition.Operand, 32)
		if err != nil {
			return err
		}
		return applyRange(field, condition.Operator, float32(operand))
	} else if field.Kind() == reflect.String {
		// Directly set the string value without checking operator.
		field.SetString(condition.Operand)
	}
	return nil
}

// applyRange sets min and max on Range type struct fields based on operator.
func applyRange(field reflect.Value, operator string, operand float32) error {
	min := field.FieldByName("Min")
	max := field.FieldByName("Max")
	switch operator {
	case "<=":
		max.SetFloat(float64(operand))
	case ">=":
		min.SetFloat(float64(operand))
	case "==":
		min.SetFloat(float64(operand))
		max.SetFloat(float64(operand))
	default:
		return fmt.Errorf("unsupported operator: %s", operator)
	}
	return nil
}

// RecommendSpec is func to recommend a VM
func RecommendSpec(nsId string, plan model.RecommendSpecReq) ([]model.SpecInfo, error) {
	// Filtering and sorting with DB query

	u := &model.FilterSpecsByRangeRequest{}
	// Apply filter policies dynamically.
	if err := applyFilterPolicies(u, &plan); err != nil {
		log.Error().Err(err).Msg("Failed to apply filter policies")
		return nil, err
	}

	// Set final limit
	finalLimitNum, err := strconv.Atoi(plan.Limit)
	if err != nil {
		finalLimitNum = 0 // Default to no limit if parsing fails
	}

	// Apply final limit to the filter request
	if finalLimitNum > 0 {
		u.Limit = finalLimitNum
		log.Info().Msgf("Setting limit to %d", finalLimitNum)
	} else {
		u.Limit = 0 // No limit
		log.Info().Msg("No limit applied - returning all filtered results")
	}

	// Build ORDER BY clause based on priority policy
	orderBy, err := buildOrderByClause(plan.Priority.Policy)
	if err != nil {
		log.Error().Err(err).Msg("Failed to build ORDER BY clause")
		return nil, err
	}

	// log.Debug().Msgf("Using ORDER BY: %s", orderBy)

	// Filtering and sorting in one DB query
	log.Debug().Msg("[Filtering and sorting specs with DB query]")

	startTime := time.Now()
	filteredSpecs, err := resource.FilterSpecsByRange(nsId, *u, orderBy)

	if err != nil {
		log.Error().Err(err).Msg("")
		return []model.SpecInfo{}, err
	}
	elapsedTime := time.Since(startTime)
	log.Info().
		Int("resultCount", len(filteredSpecs)).
		Dur("elapsedTime", elapsedTime).
		Msg("Filtering and sorting complete")

	if len(filteredSpecs) == 0 {
		return []model.SpecInfo{}, nil
	}

	return filteredSpecs, nil

}

// buildOrderByClause builds the ORDER BY clause based on priority policies
func buildOrderByClause(policies []model.PriorityCondition) (string, error) {
	if len(policies) == 0 {
		// Default to cost ordering (ascending - cheaper first), -1 means unknown cost (lowest priority)
		return "CASE WHEN cost_per_hour > 0 THEN cost_per_hour ELSE 999999 END ASC", nil
	}

	orderParts := []string{}

	for _, policy := range policies {
		switch policy.Metric {
		case "cost":
			// Cost: ascending (cheaper first), -1 means unknown cost (lowest priority)
			orderParts = append(orderParts, "CASE WHEN cost_per_hour > 0 THEN cost_per_hour ELSE 999999 END ASC")
		case "performance":
			// Performance: descending (higher performance first), -1 means unknown performance (lowest priority)
			orderParts = append(orderParts, "CASE WHEN evaluation_score01 > 0 THEN evaluation_score01 ELSE -999999 END DESC")
		case "random":
			// Random: use RANDOM() function
			orderParts = append(orderParts, "RANDOM()")
		case "location":
			// Location: build distance-based ORDER BY
			locationOrderBy, err := BuildLocationOrderByClause(&policy.Parameter)
			if err != nil {
				log.Warn().Err(err).Msg("Failed to build location ORDER BY, falling back to cost")
				orderParts = append(orderParts, "CASE WHEN cost_per_hour > 0 THEN cost_per_hour ELSE 999999 END ASC")
			} else {
				orderParts = append(orderParts, locationOrderBy)
			}
		case "latency":
			// Latency: build latency-based ORDER BY using LatencyInfo table
			latencyOrderBy, err := BuildLatencyOrderByClause(&policy.Parameter)
			if err != nil {
				log.Warn().Err(err).Msg("Failed to build latency ORDER BY, falling back to cost")
				orderParts = append(orderParts, "CASE WHEN cost_per_hour > 0 THEN cost_per_hour ELSE 999999 END ASC")
			} else {
				orderParts = append(orderParts, latencyOrderBy)
			}
		default:
			// Default to cost ordering
			orderParts = append(orderParts, "CASE WHEN cost_per_hour > 0 THEN cost_per_hour ELSE 999999 END ASC")
		}
	}

	if len(orderParts) == 0 {
		return "CASE WHEN cost_per_hour > 0 THEN cost_per_hour ELSE 999999 END ASC", nil
	}

	return strings.Join(orderParts, ", "), nil
}

// RecommendVmLatency func prioritize specs by latency based on given MCI (fair)
func RecommendVmLatency(nsId string, specList *[]model.SpecInfo, param *[]model.ParameterKeyVal) ([]model.SpecInfo, error) {

	result := []model.SpecInfo{}

	for _, v := range *param {

		switch v.Key {
		case "latencyMinimal":

			// distance (in terms of latency)
			type distanceType struct {
				distance      float64
				index         int
				priorityIndex int
			}
			distances := []distanceType{}

			// Evaluate
			for i, k := range *specList {
				sumLatancy := 0.0
				for _, region := range v.Val {
					l, _ := GetLatency(region, k.ProviderName+"-"+k.RegionName)
					sumLatancy += l
				}

				distances = append(distances, distanceType{})
				distances[i].distance = sumLatancy
				distances[i].index = i
			}

			// Sort
			sort.Slice(distances, func(i, j int) bool {
				// Handle negative cost values - positive values have higher priority
				if (*specList)[i].CostPerHour >= 0 && (*specList)[j].CostPerHour >= 0 {
					// Both are positive, compare normally
					return (*specList)[i].CostPerHour < (*specList)[j].CostPerHour
				}
				// Otherwise, positive value has higher priority
				return (*specList)[i].CostPerHour >= 0
			})
			sort.Slice(distances, func(i, j int) bool {
				return distances[i].distance < distances[j].distance
			})
			//fmt.Printf("\n[Latency]\n %v \n", distances)

			priorityCnt := 1
			for i := range distances {

				// priorityIndex++ if two distances are not equal (give the same priorityIndex if two variables are same)
				if i != 0 {
					if distances[i].distance > distances[i-1].distance {
						priorityCnt++
					}
				}
				distances[i].priorityIndex = priorityCnt

			}

			max := float32(distances[len(*specList)-1].distance)
			min := float32(distances[0].distance)

			for i := range *specList {
				// update OrderInFilteredResult based on calculated priorityIndex
				(*specList)[distances[i].index].OrderInFilteredResult = uint16(distances[i].priorityIndex)
				// assign nomalized priorityIdex value to EvaluationScore09
				(*specList)[distances[i].index].EvaluationScore09 = float32((max - float32(distances[i].distance)) / (max - min + 0.0000001)) // Add small value to avoid NaN by division
				(*specList)[distances[i].index].EvaluationScore10 = float32(distances[i].distance)
				// fmt.Printf("\n [%v] OrderInFilteredResult:%v, max:%v, min:%v, distance:%v, eval:%v \n", i, (*specList)[distances[i].index].OrderInFilteredResult, max, min, float32(distances[i].distance), (*specList)[distances[i].index].EvaluationScore09)
			}
		default:
			// log.Debug().Msg("[Checking] Not available metric " + metric)
		}

	}

	result = append(result, *specList...)
	//result[i].OrderInFilteredResult = uint16(i + 1)

	// Sorting result based on multiple criteria: OrderInFilteredResult, CostPerHour, VCPU, MemoryGiB
	sort.Slice(result, func(i, j int) bool {
		// 1st priority: OrderInFilteredResult
		if result[i].OrderInFilteredResult != result[j].OrderInFilteredResult {
			return result[i].OrderInFilteredResult < result[j].OrderInFilteredResult
		}
		// 2nd priority: CostPerHour
		if result[i].CostPerHour != result[j].CostPerHour {
			// negative value has lower priority (negative means cost is not specified)
			if result[i].CostPerHour >= 0 && result[j].CostPerHour >= 0 {
				// both are positive, compare normally
				return result[i].CostPerHour < result[j].CostPerHour
			}
			return result[i].CostPerHour >= 0
		}
		// 3rd priority: VCPU
		if result[i].VCPU != result[j].VCPU {
			return float32(result[i].VCPU) < float32(result[j].VCPU)
		}
		// 4th priority: MemoryGiB
		return float32(result[i].MemoryGiB) < float32(result[j].MemoryGiB)
	})

	// updatedSpec, err := resource.UpdateSpec(nsId, *result)
	// content, err = resource.SortSpecs(*specList, "memoryGiB", "descending")
	return result, nil
}

// BuildLocationOrderByClause generates ORDER BY clause for location-based sorting
func BuildLocationOrderByClause(param *[]model.ParameterKeyVal) (string, error) {
	if param == nil || len(*param) == 0 {
		return "", fmt.Errorf("no location parameters provided")
	}

	for _, v := range *param {
		switch v.Key {
		case "coordinateClose":
			if len(v.Val) == 0 {
				return "", fmt.Errorf("coordinateClose requires coordinate value")
			}

			coordinateStr := v.Val[0]
			slice := strings.Split(coordinateStr, "/")
			if len(slice) != 2 {
				return "", fmt.Errorf("invalid coordinate format, expected 'latitude/longitude'")
			}

			latitude, err := strconv.ParseFloat(strings.ReplaceAll(slice[0], " ", ""), 64)
			if err != nil {
				return "", fmt.Errorf("invalid latitude: %v", err)
			}
			longitude, err := strconv.ParseFloat(strings.ReplaceAll(slice[1], " ", ""), 64)
			if err != nil {
				return "", fmt.Errorf("invalid longitude: %v", err)
			}

			// Generate distance-based sorting with 50km grouping and cost as secondary sort
			// Primary: 50km distance groups (0-49km, 50-99km, 100-149km, etc.)
			// Secondary: Cost within same distance group
			// Tertiary: Exact distance for tie-breaking
			// Use calculated distance to avoid duplication
			haversineDistance := fmt.Sprintf(`(6371 * acos(
				cos(radians(%f)) * cos(radians(region_latitude)) * 
				cos(radians(region_longitude) - radians(%f)) + 
				sin(radians(%f)) * sin(radians(region_latitude))
			))`, latitude, longitude, latitude)

			orderBy := fmt.Sprintf(`
                ROUND(%s / 50) * 50 ASC,
                CASE WHEN cost_per_hour > 0 THEN cost_per_hour ELSE 999999 END ASC,
                %s ASC`,
				haversineDistance, haversineDistance)

			return orderBy, nil

		case "coordinateFair":
			if len(v.Val) == 0 {
				return "", fmt.Errorf("coordinateFair requires coordinate values")
			}

			// Calculate centroid of coordinate clusters
			latitudeSum := 0.0
			longitudeSum := 0.0
			for _, coordinateStr := range v.Val {
				slice := strings.Split(coordinateStr, "/")
				if len(slice) != 2 {
					return "", fmt.Errorf("invalid coordinate format, expected 'latitude/longitude'")
				}

				latitudeEach, err := strconv.ParseFloat(strings.ReplaceAll(slice[0], " ", ""), 64)
				if err != nil {
					return "", fmt.Errorf("invalid latitude: %v", err)
				}
				longitudeEach, err := strconv.ParseFloat(strings.ReplaceAll(slice[1], " ", ""), 64)
				if err != nil {
					return "", fmt.Errorf("invalid longitude: %v", err)
				}
				latitudeSum += latitudeEach
				longitudeSum += longitudeEach
			}

			latitude := latitudeSum / float64(len(v.Val))
			longitude := longitudeSum / float64(len(v.Val))

			// Generate Haversine distance calculation for centroid
			orderBy := fmt.Sprintf(`(
				6371 * acos(
					cos(radians(%f)) * cos(radians(region_latitude)) * 
					cos(radians(region_longitude) - radians(%f)) + 
					sin(radians(%f)) * sin(radians(region_latitude))
				)
			) ASC`, latitude, longitude, latitude)

			return orderBy, nil

		case "coordinateWithin":
			// For coordinateWithin, we can use the same distance calculation as coordinateClose
			// The filtering by radius would be handled in WHERE clause, not ORDER BY
			if len(v.Val) == 0 {
				return "", fmt.Errorf("coordinateWithin requires coordinate value")
			}

			coordinateStr := v.Val[0]
			parts := strings.Split(coordinateStr, "/")
			if len(parts) < 2 {
				return "", fmt.Errorf("invalid coordinate format for coordinateWithin")
			}

			latitude, err := strconv.ParseFloat(strings.ReplaceAll(parts[0], " ", ""), 64)
			if err != nil {
				return "", fmt.Errorf("invalid latitude: %v", err)
			}
			longitude, err := strconv.ParseFloat(strings.ReplaceAll(parts[1], " ", ""), 64)
			if err != nil {
				return "", fmt.Errorf("invalid longitude: %v", err)
			}

			orderBy := fmt.Sprintf(`(
				6371 * acos(
					cos(radians(%f)) * cos(radians(region_latitude)) * 
					cos(radians(region_longitude) - radians(%f)) + 
					sin(radians(%f)) * sin(radians(region_latitude))
				)
			) ASC`, latitude, longitude, latitude)

			return orderBy, nil
		}
	}

	return "", fmt.Errorf("unsupported location parameter")
}

// BuildLatencyOrderByClause generates ORDER BY clause for latency-based sorting
func BuildLatencyOrderByClause(param *[]model.ParameterKeyVal) (string, error) {
	if param == nil || len(*param) == 0 {
		return "", fmt.Errorf("no latency parameters provided")
	}

	for _, v := range *param {
		switch v.Key {
		case "latencyMinimal":
			if len(v.Val) == 0 {
				return "", fmt.Errorf("latencyMinimal requires target region values")
			}

			// Build subquery to calculate sum of latencies for each spec
			// We'll use COALESCE to handle missing latency data with a high penalty value
			latencyParts := []string{}

			for _, targetRegion := range v.Val {
				// Create a subquery that joins with LatencyInfo table
				// The source region is constructed as "provider_name+region_name"
				latencySubquery := fmt.Sprintf(`
					COALESCE((
						SELECT latency_ms 
						FROM tb_latency_infos 
						WHERE source_region = '%s' 
						AND target_region = tb_spec_infos.provider_name || '+' || tb_spec_infos.region_name
						LIMIT 1
					), 999999)`, targetRegion)

				latencyParts = append(latencyParts, latencySubquery)
			}

			// Sum all latencies for multi-target latency minimization
			if len(latencyParts) == 1 {
				orderBy := fmt.Sprintf("(%s) ASC", latencyParts[0])
				return orderBy, nil
			} else {
				// Sum multiple latencies
				orderBy := fmt.Sprintf("(%s) ASC", strings.Join(latencyParts, " + "))
				return orderBy, nil
			}
		}
	}

	return "", fmt.Errorf("unsupported latency parameter")
}

// RecommendVmLocation func prioritize specs based on given location
func RecommendVmLocation(nsId string, specList *[]model.SpecInfo, param *[]model.ParameterKeyVal) ([]model.SpecInfo, error) {

	for _, v := range *param {

		switch v.Key {
		case "coordinateClose":
			//
			coordinateStr := v.Val[0]

			slice := strings.Split(coordinateStr, "/")
			latitude, err := strconv.ParseFloat(strings.ReplaceAll(slice[0], " ", ""), 32)
			if err != nil {
				log.Error().Err(err).Msg("")
				return []model.SpecInfo{}, err
			}
			longitude, err := strconv.ParseFloat(strings.ReplaceAll(slice[1], " ", ""), 32)
			if err != nil {
				log.Error().Err(err).Msg("")
				return []model.SpecInfo{}, err
			}

			type distanceType struct {
				distance      float64
				index         int
				priorityIndex int
			}
			distances := make([]distanceType, len(*specList))

			var wg sync.WaitGroup // WaitGroup to wait for all goroutines to finish
			var mu sync.Mutex     // Mutex to protect shared data
			var once sync.Once    // Once ensures that certain actions are performed only once
			var globalErr error   // Global error variable to capture any error from goroutines

			for i := range *specList {
				wg.Add(1)
				go func(i int) {
					defer wg.Done() // Decrement the counter when the goroutine completes

					var distance float64
					distance, err = getDistance(latitude, longitude, (*specList)[i].ProviderName, (*specList)[i].RegionName)
					if err != nil {
						log.Error().Err(err).Msg("")
						mu.Lock()
						globalErr = err // Capture the error in globalErr
						mu.Unlock()

						once.Do(func() {
							// If an error occurs, stop all operations (this block is executed only once)
							distance = 99999999 // Set a very large value to avoid using this value in the calculation
						})
					}

					mu.Lock() // Lock to protect the shared data
					distances[i].distance = distance
					distances[i].index = i
					mu.Unlock() // Unlock after updating

				}(i)
			}

			wg.Wait() // Wait for all goroutines to finish

			if globalErr != nil { // If there's an error from any goroutine, return it
				// log.Error().Err(globalErr).Msg("")
				// return []model.SpecInfo{}, globalErr
			}

			sort.Slice(distances, func(i, j int) bool {
				// Handle negative cost values - positive values have higher priority
				if (*specList)[i].CostPerHour >= 0 && (*specList)[j].CostPerHour >= 0 {
					// Both are positive, compare normally
					return (*specList)[i].CostPerHour < (*specList)[j].CostPerHour
				}
				// Otherwise, positive value has higher priority
				return (*specList)[i].CostPerHour >= 0
			})

			// Sort distances based on calculated distance
			sort.Slice(distances, func(i, j int) bool {
				return distances[i].distance < distances[j].distance
			})

			// Calculate priorityIndex based on distances
			minDistance := distances[0].distance
			// Set a tolerance threshold as 10% of the minimum distance
			toleranceThreshold := minDistance * 0.1

			priorityCnt := 1
			for i := range distances {
				// priorityIndex++ if two distances are not equal (give the same priorityIndex if two variables are same)
				if i != 0 {
					currentDiff := distances[i].distance - distances[i-1].distance

					if currentDiff > toleranceThreshold {
						priorityCnt++
					}
				}
				distances[i].priorityIndex = priorityCnt
			}

			max := float32(distances[len(*specList)-1].distance)
			min := float32(distances[0].distance)

			for i := range *specList {
				// update OrderInFilteredResult based on calculated priorityIndex
				(*specList)[distances[i].index].OrderInFilteredResult = uint16(distances[i].priorityIndex)
				// assign nomalized priorityIdex value to EvaluationScore09
				(*specList)[distances[i].index].EvaluationScore09 = float32((max - float32(distances[i].distance)) / (max - min + 0.0000001)) // Add small value to avoid NaN by division
				(*specList)[distances[i].index].EvaluationScore10 = float32(distances[i].distance)
				// fmt.Printf("\n [%v] OrderInFilteredResult:%v, max:%v, min:%v, distance:%v, eval:%v \n", i, (*specList)[distances[i].index].OrderInFilteredResult, max, min, float32(distances[i].distance), (*specList)[distances[i].index].EvaluationScore09)
			}

		case "coordinateWithin":
			//
		case "coordinateFair":

			// Calculate centroid of coordinate clusters
			latitudeSum := 0.0
			longitudeSum := 0.0
			for _, coordinateStr := range v.Val {
				slice := strings.Split(coordinateStr, "/")
				latitudeEach, err := strconv.ParseFloat(strings.ReplaceAll(slice[0], " ", ""), 32)
				if err != nil {
					log.Error().Err(err).Msg("")
					return []model.SpecInfo{}, err
				}
				longitudeEach, err := strconv.ParseFloat(strings.ReplaceAll(slice[1], " ", ""), 32)
				if err != nil {
					log.Error().Err(err).Msg("")
					return []model.SpecInfo{}, err
				}
				latitudeSum += latitudeEach
				longitudeSum += longitudeEach
			}
			latitude := latitudeSum / (float64)(len(v.Val))
			longitude := longitudeSum / (float64)(len(v.Val))

			// Sorting, closes to the centroid.

			type distanceType struct {
				distance      float64
				index         int
				priorityIndex int
			}
			distances := make([]distanceType, len(*specList))

			var wg sync.WaitGroup // WaitGroup to wait for all goroutines to finish
			var mu sync.Mutex     // Mutex to protect shared data
			var once sync.Once    // Once ensures that certain actions are performed only once
			var globalErr error   // Global error variable to capture any error from goroutines

			for i := range *specList {
				wg.Add(1)
				go func(i int) {
					defer wg.Done() // Decrement the counter when the goroutine completes

					distance, err := getDistance(latitude, longitude, (*specList)[i].ProviderName, (*specList)[i].RegionName)
					if err != nil {
						log.Error().Err(err).Msg("")
						mu.Lock()
						globalErr = err // Capture the error in globalErr
						mu.Unlock()

						once.Do(func() {
							// If an error occurs, stop all operations (this block is executed only once)
							distance = 99999999 // Set a very large value to avoid using this value in the calculation
						})
					}

					mu.Lock() // Lock to protect the shared data
					distances[i].distance = distance
					distances[i].index = i
					mu.Unlock() // Unlock after updating

				}(i)
			}

			wg.Wait() // Wait for all goroutines to finish

			if globalErr != nil { // If there's an error from any goroutine, return it
				return []model.SpecInfo{}, globalErr
			}

			sort.Slice(distances, func(i, j int) bool {
				// Handle negative cost values - positive values have higher priority
				if (*specList)[i].CostPerHour >= 0 && (*specList)[j].CostPerHour >= 0 {
					// Both are positive, compare normally
					return (*specList)[i].CostPerHour < (*specList)[j].CostPerHour
				}
				// Otherwise, positive value has higher priority
				return (*specList)[i].CostPerHour >= 0
			})
			sort.Slice(distances, func(i, j int) bool {
				return distances[i].distance < distances[j].distance
			})

			priorityCnt := 1
			for i := range distances {

				// priorityIndex++ if two distances are not equal (give the same priorityIndex if two variables are same)
				if i != 0 {
					if distances[i].distance > distances[i-1].distance {
						priorityCnt++
					}
				}
				distances[i].priorityIndex = priorityCnt

			}

			max := float32(distances[len(*specList)-1].distance)
			min := float32(distances[0].distance)

			for i := range *specList {
				// update OrderInFilteredResult based on calculated priorityIndex
				(*specList)[distances[i].index].OrderInFilteredResult = uint16(distances[i].priorityIndex)
				// assign nomalized priorityIdex value to EvaluationScore09
				(*specList)[distances[i].index].EvaluationScore09 = float32((max - float32(distances[i].distance)) / (max - min + 0.0000001)) // Add small value to avoid NaN by division
				(*specList)[distances[i].index].EvaluationScore10 = float32(distances[i].distance)
				// fmt.Printf("\n [%v] OrderInFilteredResult:%v, max:%v, min:%v, distance:%v, eval:%v \n", i, (*specList)[distances[i].index].OrderInFilteredResult, max, min, float32(distances[i].distance), (*specList)[distances[i].index].EvaluationScore09)
			}
		default:
			// log.Debug().Msg("[Checking] Not available metric " + metric)
		}

	}

	result := append([]model.SpecInfo{}, (*specList)...)

	// Sorting result based on multiple criteria: OrderInFilteredResult, CostPerHour, VCPU, MemoryGiB
	sort.Slice(result, func(i, j int) bool {
		// 1st priority: OrderInFilteredResult
		if result[i].OrderInFilteredResult != result[j].OrderInFilteredResult {
			return result[i].OrderInFilteredResult < result[j].OrderInFilteredResult
		}
		// 2nd priority: CostPerHour
		if result[i].CostPerHour != result[j].CostPerHour {
			// negative value has lower priority (negative means cost is not specified)
			if result[i].CostPerHour >= 0 && result[j].CostPerHour >= 0 {
				// both are positive, compare normally
				return result[i].CostPerHour < result[j].CostPerHour
			}
			return result[i].CostPerHour >= 0
		}
		// 3rd priority: VCPU
		if result[i].VCPU != result[j].VCPU {
			return float32(result[i].VCPU) < float32(result[j].VCPU)
		}
		// 4th priority: MemoryGiB
		return float32(result[i].MemoryGiB) < float32(result[j].MemoryGiB)
	})
	// fmt.Printf("\n result : %v \n", result)

	// updatedSpec, err := resource.UpdateSpec(nsId, *result)
	// content, err = resource.SortSpecs(*specList, "memoryGiB", "descending")
	return result, nil
}

// RecommendSpecOptions returns available options for RecommendSpec API
func RecommendSpecOptions(nsId string) (*model.RecommendSpecRequestOptions, error) {
	log.Info().Str("nsId", nsId).Msg("Getting RecommendSpec options")

	// Initialize options with static configuration
	options := &model.RecommendSpecRequestOptions{
		Filter: model.FilterOptionsInfo{
			AvailableMetrics: []string{
				// String fields
				"id",
				"providerName",
				"regionName",
				"cspSpecName",
				"architecture",
				"acceleratorModel",
				"acceleratorType",
				"description",
				// Range fields (numeric)
				"vCPU",
				"memoryGiB",
				// "diskSizeGB",
				// "maxTotalStorageTiB",
				// "netBwGbps",
				"acceleratorCount",
				"acceleratorMemoryGB",
				"costPerHour",
				"evaluationScore01",
				// "evaluationScore02",
				// "evaluationScore03",
				// "evaluationScore04",
				// "evaluationScore05",
				// "evaluationScore06",
				// "evaluationScore07",
				// "evaluationScore08",
				// "evaluationScore09",
				// "evaluationScore10",
			},
			ExamplePolicies: []model.FilterConditionExample{
				{
					Metric:      "vCPU",
					Description: "Filter specs with 2-8 vCPUs",
					Condition: []model.OperationExample{
						{Operator: ">=", Operand: "2"},
						{Operator: "<=", Operand: "8"},
					},
				},
				{
					Metric:      "memoryGiB",
					Description: "Filter specs with at least 4GB memory",
					Condition: []model.OperationExample{
						{Operator: ">=", Operand: "4"},
					},
				},
				{
					Metric:      "costPerHour",
					Description: "Filter specs with cost under $0.50/hour",
					Condition: []model.OperationExample{
						{Operator: "<=", Operand: "0.50"},
					},
				},
				{
					Metric:      "providerName",
					Description: "Filter specs from specific provider",
					Condition: []model.OperationExample{
						{Operator: "=", Operand: csp.AWS}, {Operator: "=", Operand: strings.Join([]string{csp.AWS, csp.GCP}, ",")},
					},
				},
				{
					Metric:      "architecture",
					Description: "Filter specs by architecture",
					Condition: []model.OperationExample{
						{Operator: "=", Operand: "x86_64"},
					},
				},
				{
					Metric:      "infraType",
					Description: "Filter specs by infrastructure type",
					Condition: []model.OperationExample{
						{Operator: "=", Operand: "vm"},
					},
				},
				{
					Metric:      "acceleratorType",
					Description: "Filter specs with GPU accelerator",
					Condition: []model.OperationExample{
						{Operator: "=", Operand: "GPU"},
					},
				},
				{
					Metric:      "diskSizeGB",
					Description: "Filter specs with disk size between 100-500GB",
					Condition: []model.OperationExample{
						{Operator: ">=", Operand: "100"},
						{Operator: "<=", Operand: "500"},
					},
				},
			},
			AvailableValues: model.FilterAvailableValues{},
		},
		Priority: model.PriorityOptionsInfo{
			AvailableMetrics: []string{
				"cost",
				"performance",
				"location",
				"latency",
				"random",
			},
			ExamplePolicies: []model.PriorityConditionExample{
				{
					Metric:      "cost",
					Description: "Prioritize by lowest cost",
					Weight:      "1.0",
				},
				{
					Metric:      "performance",
					Description: "Prioritize by highest performance",
					Weight:      "1.0",
				},
				{
					Metric:      "location",
					Description: "Prioritize by proximity to coordinates",
					Weight:      "1.0",
					Parameter: []model.ParameterKeyValExample{
						{
							Key:         "coordinateClose",
							Description: "Find specs closest to given coordinate",
							Val:         []string{"37.5665/126.9780"},
						},
					},
				},
				{
					Metric:      "latency",
					Description: "Prioritize by minimal network latency",
					Weight:      "1.0",
					Parameter: []model.ParameterKeyValExample{
						{
							Key:         "latencyMinimal",
							Description: "Find specs with minimal latency to target",
							Val:         []string{"aws+us-east-1"},
						},
					},
				},
				{
					Metric:      "random",
					Description: "Random prioritization for testing",
					Weight:      "1.0",
				},
			},
			ParameterOptions: model.ParameterOptionsInfo{
				LocationParameters: []model.ParameterOptionDetail{
					{
						Key:         "coordinateClose",
						Description: "Find specs closest to given coordinate (latitude/longitude)",
						Format:      "latitude/longitude",
						Example:     []string{"37.5665/126.9780", "35.6762/139.6503", "40.7128/-74.0060"},
					},
					{
						Key:         "coordinateWithin",
						Description: "Find specs within radius of coordinate (latitude/longitude/radius_km)",
						Format:      "latitude/longitude/radius_km",
						Example:     []string{"37.5665/126.9780/100", "35.6762/139.6503/50"},
					},
					{
						Key:         "coordinateFair",
						Description: "Fair distribution around coordinate with radius (latitude/longitude/radius_km)",
						Format:      "latitude/longitude/radius_km",
						Example:     []string{"37.5665/126.9780/200", "35.6762/139.6503/150"},
					},
				},
				LatencyParameters: []model.ParameterOptionDetail{
					{
						Key:         "latencyMinimal",
						Description: "Find specs with minimal network latency to target region",
						Format:      "provider+region",
						Example:     []string{"aws+us-east-1", "azure+eastus", "gcp+us-central1-a"},
					},
				},
			},
		},
		Limit: []string{"5", "10", "20", "50"},
	}

	// Efficiently get distinct values from DB using ORM queries with fallback to default examples
	// Get distinct provider names (non-empty only)
	if err := model.ORM.Model(&model.SpecInfo{}).
		Where("namespace = ? AND provider_name != ''", nsId).
		Distinct("provider_name").
		Order("provider_name").
		Pluck("provider_name", &options.Filter.AvailableValues.ProviderName).Error; err != nil {
		log.Warn().Err(err).Msg("Failed to get distinct provider names, using default examples")
		// Fallback to default examples if query fails
		options.Filter.AvailableValues.ProviderName = []string{csp.AWS, csp.Azure, csp.GCP, csp.NCP, csp.Alibaba, csp.Tencent, csp.NHN}
	}

	// Get distinct region names (non-empty only)
	if err := model.ORM.Model(&model.SpecInfo{}).
		Where("namespace = ? AND region_name != ''", nsId).
		Distinct("region_name").
		Order("region_name").
		Pluck("region_name", &options.Filter.AvailableValues.RegionName).Error; err != nil {
		log.Warn().Err(err).Msg("Failed to get distinct region names, using default examples")
		// Fallback to default examples if query fails
		options.Filter.AvailableValues.RegionName = []string{
			"us-east-1", "us-west-2", "eu-west-1", "ap-northeast-2",
			"ap-southeast-1", "koreacentral", "eastus", "asia-northeast3",
		}
	}

	// Get distinct architectures (non-empty only)
	if err := model.ORM.Model(&model.SpecInfo{}).
		Where("namespace = ? AND architecture != ''", nsId).
		Distinct("architecture").
		Order("architecture").
		Pluck("architecture", &options.Filter.AvailableValues.Architecture).Error; err != nil {
		log.Warn().Err(err).Msg("Failed to get distinct architectures, using default examples")
		// Fallback to default examples if query fails
		options.Filter.AvailableValues.Architecture = []string{"x86_64", "arm64"}
	}

	// Get distinct infra types (non-empty only)
	// Note: infraType is commented out in AvailableMetrics - uncomment when needed
	// if err := model.ORM.Model(&model.SpecInfo{}).
	// 	Where("namespace = ? AND infra_type != ''", nsId).
	// 	Distinct("infra_type").
	// 	Order("infra_type").
	// 	Pluck("infra_type", &options.Filter.AvailableValues.InfraType).Error; err != nil {
	// 	log.Warn().Err(err).Msg("Failed to get distinct infra types, using default examples")
	// 	// Fallback to default examples if query fails
	// 	options.Filter.AvailableValues.InfraType = []string{"vm", "k8s", "container"}
	// }

	// // Get distinct connection names (non-empty only)
	// This parameter is used to filter the available connection names for the user

	// if err := model.ORM.Model(&model.SpecInfo{}).
	// 	Where("namespace = ? AND connection_name != ''", nsId).
	// 	Distinct("connection_name").
	// 	Order("connection_name").
	// 	Pluck("connection_name", &options.Filter.AvailableValues.ConnectionName).Error; err != nil {
	// 	log.Warn().Err(err).Msg("Failed to get distinct connection names, using default examples")
	// 	// Fallback to default examples if query fails
	// 	options.Filter.AvailableValues.ConnectionName = []string{
	// 		"aws-ap-northeast-2", "azure-koreacentral", "gcp-asia-northeast3", "ncp-kr",
	// 	}
	// }

	// Get distinct CSP spec names (grouped by provider for better representation)
	var cspSpecsByProvider []struct {
		ProviderName string `json:"provider_name"`
		CspSpecName  string `json:"csp_spec_name"`
	}

	if err := model.ORM.Model(&model.SpecInfo{}).
		Select("provider_name, csp_spec_name").
		Where("namespace = ? AND csp_spec_name != ''", nsId).
		Order("provider_name, csp_spec_name").
		Find(&cspSpecsByProvider).Error; err != nil {
		log.Warn().Err(err).Msg("Failed to get CSP spec names by provider, using default examples")
		// Fallback to default examples if query fails
		options.Filter.AvailableValues.CspSpecName = []string{
			"t2.micro", "t2.small", "t3.medium", "Standard_B1s", "Standard_B2s",
			"e2-micro", "e2-small", "n1-standard-1", "m8-g3a", "m8-l3a",
		}
	} else {
		// Group CSP spec names by provider and take diverse examples from each
		providerCspSpecs := make(map[string][]string)
		for _, spec := range cspSpecsByProvider {
			providerCspSpecs[spec.ProviderName] = append(providerCspSpecs[spec.ProviderName], spec.CspSpecName)
		}

		var sampleCspSpecs []string
		// Collect diverse examples (max 3 per provider, total max 40)
		maxPerProvider := 3
		totalLimit := 40
		for _, specs := range providerCspSpecs {
			taken := 0
			for _, cspSpecName := range specs {
				if taken < maxPerProvider && len(sampleCspSpecs) < totalLimit {
					sampleCspSpecs = append(sampleCspSpecs, cspSpecName)
					taken++
				}
			}
			if len(sampleCspSpecs) >= totalLimit {
				break
			}
		}

		// If no specs found in DB, use fallback examples
		if len(sampleCspSpecs) == 0 {
			sampleCspSpecs = []string{
				"t2.micro", "t2.small", "t3.medium", "Standard_B1s", "Standard_B2s",
				"e2-micro", "e2-small", "n1-standard-1", "m8-g3a", "m8-l3a",
			}
		}

		options.Filter.AvailableValues.CspSpecName = sampleCspSpecs
	}

	// Get distinct OS types (non-empty only)
	// Note: osType is commented out in AvailableMetrics - uncomment when needed
	// if err := model.ORM.Model(&model.SpecInfo{}).
	// 	Where("namespace = ? AND os_type != ''", nsId).
	// 	Distinct("os_type").
	// 	Order("os_type").
	// 	Pluck("os_type", &options.Filter.AvailableValues.OsType).Error; err != nil {
	// 	log.Warn().Err(err).Msg("Failed to get distinct OS types, using default examples")
	// 	// Fallback to default examples if query fails
	// 	options.Filter.AvailableValues.OsType = []string{"linux", "windows"}
	// }

	// Get distinct accelerator models (non-empty only)
	if err := model.ORM.Model(&model.SpecInfo{}).
		Where("namespace = ? AND accelerator_model != ''", nsId).
		Distinct("accelerator_model").
		Order("accelerator_model").
		Pluck("accelerator_model", &options.Filter.AvailableValues.AcceleratorModel).Error; err != nil {
		log.Warn().Err(err).Msg("Failed to get distinct accelerator models, using default examples")
		// Fallback to default examples if query fails
		options.Filter.AvailableValues.AcceleratorModel = []string{
			"NVIDIA", "Tesla K80", "Tesla V100", "Tesla T4", "A100", "H100",
		}
	}

	// Get distinct accelerator types (non-empty only)
	if err := model.ORM.Model(&model.SpecInfo{}).
		Where("namespace = ? AND accelerator_type != ''", nsId).
		Distinct("accelerator_type").
		Order("accelerator_type").
		Pluck("accelerator_type", &options.Filter.AvailableValues.AcceleratorType).Error; err != nil {
		log.Warn().Err(err).Msg("Failed to get distinct accelerator types, using default examples")
		// Fallback to default examples if query fails
		options.Filter.AvailableValues.AcceleratorType = []string{"gpu"}
	}

	// Get distinct evaluation statuses (non-empty only)
	// Note: evaluationStatus is commented out in AvailableMetrics - uncomment when needed
	// if err := model.ORM.Model(&model.SpecInfo{}).
	// 	Where("namespace = ? AND evaluation_status != ''", nsId).
	// 	Distinct("evaluation_status").
	// 	Order("evaluation_status").
	// 	Pluck("evaluation_status", &options.Filter.AvailableValues.EvaluationStatus).Error; err != nil {
	// 	log.Warn().Err(err).Msg("Failed to get distinct evaluation statuses, using default examples")
	// 	// Fallback to default examples if query fails
	// 	options.Filter.AvailableValues.EvaluationStatus = []string{"evaluated", "pending", "not-evaluated"}
	// }

	// Apply fallback logic: if DB returns empty results, use default examples
	if len(options.Filter.AvailableValues.ProviderName) == 0 {
		log.Info().Msg("No provider names found in DB, using default examples")
		options.Filter.AvailableValues.ProviderName = []string{csp.AWS, csp.Azure, csp.GCP, csp.NCP, csp.Alibaba, csp.Tencent}
	}

	if len(options.Filter.AvailableValues.RegionName) == 0 {
		log.Info().Msg("No region names found in DB, using default examples")
		options.Filter.AvailableValues.RegionName = []string{
			"us-east-1", "us-west-2", "eu-west-1", "ap-northeast-2",
			"ap-southeast-1", "koreacentral", "eastus", "asia-northeast3",
		}
	}

	if len(options.Filter.AvailableValues.Architecture) == 0 {
		log.Info().Msg("No architectures found in DB, using default examples")
		options.Filter.AvailableValues.Architecture = []string{"x86_64", "arm64"}
	}

	// Note: Following fallback logic is commented out as corresponding fields are disabled in AvailableMetrics
	// Uncomment when those fields are re-enabled

	// if len(options.Filter.AvailableValues.InfraType) == 0 {
	// 	log.Info().Msg("No infra types found in DB, using default examples")
	// 	options.Filter.AvailableValues.InfraType = []string{"vm", "k8s", "container"}
	// }

	return options, nil
}

// getDistance func get geographical distance between given coordinate and region
func getDistance(latitude float64, longitude float64, providerName string, regionName string) (float64, error) {

	regionInfo, err := common.GetRegion(providerName, regionName)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 999999, err
	}
	cloudLatitude := regionInfo.Location.Latitude
	cloudLongitude := regionInfo.Location.Longitude

	// first := math.Pow(float64(cloudLatitude-latitude), 2)
	// second := math.Pow(float64(cloudLongitude-longitude), 2)
	// return math.Sqrt(first + second), nil
	return getHaversineDistance(cloudLatitude, cloudLongitude, latitude, longitude), nil

}

// GetLatency func get latency between given two regions
func GetLatency(src string, dest string) (float64, error) {
	latency, err := model.GetLatencyValue(src, dest)
	if err != nil {
		log.Info().Err(err).Msgf("Cannot get GetLatency between src: %v, dest: %v (check database)", src, dest)
		return 999999, err
	}
	return latency, nil
}

// getHaversineDistance func return HaversineDistance
func getHaversineDistance(a1 float64, b1 float64, a2 float64, b2 float64) (distance float64) {
	deltaA := (a2 - a1) * (math.Pi / 180)
	deltaB := (b2 - b1) * (math.Pi / 180)

	a := math.Sin(deltaA/2)*math.Sin(deltaA/2) +
		math.Cos(a1*(math.Pi/180))*math.Cos(a2*(math.Pi/180))*math.Sin(deltaB/2)*math.Sin(deltaB/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	earthRadius := float64(6371)
	return (earthRadius * c)
}

// RecommendVmRandom func prioritize specs randomly
func RecommendVmRandom(nsId string, specList *[]model.SpecInfo) ([]model.SpecInfo, error) {

	result := append([]model.SpecInfo{}, (*specList)...)

	rand.Shuffle(len(result), func(i, j int) { result[i], result[j] = result[j], result[i] })

	Max := float32(result[len(result)-1].CostPerHour)
	Min := float32(result[0].CostPerHour)

	for i := range result {
		result[i].OrderInFilteredResult = uint16(i + 1)
		result[i].EvaluationScore09 = float32((Max - result[i].CostPerHour) / (Max - Min + 0.0000001)) // Add small value to avoid NaN by division
	}

	return result, nil
}

// RecommendVmCost func prioritize specs based on given Cost
func RecommendVmCost(nsId string, specList *[]model.SpecInfo) ([]model.SpecInfo, error) {

	result := append([]model.SpecInfo{}, (*specList)...)

	sort.Slice(result, func(i, j int) bool {
		// Handle negative cost values - positive values have higher priority
		if result[i].CostPerHour >= 0 && result[j].CostPerHour >= 0 {
			// Both are positive, compare normally
			return result[i].CostPerHour < result[j].CostPerHour
		}
		// Otherwise, positive value has higher priority
		return result[i].CostPerHour >= 0
	})

	Max := float32(result[len(result)-1].CostPerHour)
	Min := float32(result[0].CostPerHour)

	for i := range result {
		result[i].OrderInFilteredResult = uint16(i + 1)
		result[i].EvaluationScore09 = float32((Max - result[i].CostPerHour) / (Max - Min + 0.0000001)) // Add small value to avoid NaN by division
	}

	return result, nil
}

// RecommendVmPerformance func prioritize specs based on given Performance condition
func RecommendVmPerformance(nsId string, specList *[]model.SpecInfo) ([]model.SpecInfo, error) {

	result := append([]model.SpecInfo{}, (*specList)...)

	sort.Slice(result, func(i, j int) bool { return result[i].EvaluationScore01 > result[j].EvaluationScore01 })

	Max := float32(result[0].EvaluationScore01)
	Min := float32(result[len(result)-1].EvaluationScore01)

	for i := range result {
		result[i].OrderInFilteredResult = uint16(i + 1)
		result[i].EvaluationScore09 = float32((result[i].EvaluationScore01 - Min) / (Max - Min + 0.0000001)) // Add small value to avoid NaN by division
	}

	return result, nil
}

// RecommendK8sNode is func to recommend a node for K8sCluster
func RecommendK8sNode(nsId string, plan model.RecommendSpecReq) ([]model.SpecInfo, error) {
	emptyObjList := []model.SpecInfo{}

	// K8s node minimum requirements
	const minVCPU = 2
	const minMemoryGiB = 4.0

	limitOrig := plan.Limit
	plan.Limit = strconv.Itoa(math.MaxInt)

	SpecInfoListForVm, err := RecommendSpec(nsId, plan)
	if err != nil {
		return emptyObjList, err
	}

	limitNum, err := strconv.Atoi(limitOrig)
	if err != nil {
		limitNum = math.MaxInt
	}

	SpecInfoListForK8s := []model.SpecInfo{}
	count := 0
	for _, SpecInfo := range SpecInfoListForVm {
		// K8s node minimum hardware requirements: 2+ vCPU, 4+ GB RAM
		log.Debug().Msgf("Checking spec: %s (vCPU: %d, Memory: %.2fGB)",
			SpecInfo.Id, SpecInfo.VCPU, SpecInfo.MemoryGiB)

		if SpecInfo.VCPU >= minVCPU && SpecInfo.MemoryGiB >= minMemoryGiB {
			SpecInfoListForK8s = append(SpecInfoListForK8s, SpecInfo)
			count++
			if count == limitNum {
				break
			}
		} else {
			log.Debug().Msgf("Spec %s does not meet K8s minimum requirements (need: vCPU>=%d, RAM>=%.1fGB)",
				SpecInfo.Id, minVCPU, minMemoryGiB)
		}
	}

	log.Info().Msgf("K8s node recommendation complete: %d specs found (from %d total specs)",
		len(SpecInfoListForK8s), len(SpecInfoListForVm))

	return SpecInfoListForK8s, nil
}

// // GetRecommendList is func to get recommendation list
// func GetRecommendList(nsId string, cpuSize string, memSize string, diskSize string) ([]VmPriority, error) {

// 	log.Debug().Msg("GetRecommendList")

// 	var content struct {
// 		Id             string
// 		Price          string
// 		ConnectionName string
// 	}

// 	key := common.GenMciKey(nsId, "", "") + "/cpuSize/" + cpuSize + "/memSize/" + memSize + "/diskSize/" + diskSize
// 	log.Debug().Msg(key)
// 	keyValue, err := kvstore.GetKvList(key)
// 	keyValue = kvutil.FilterKvListBy(keyValue, key, 1)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return []VmPriority{}, err
// 	}

// 	var vmPriorityList []VmPriority

// 	for cnt, v := range keyValue {
// 		log.Debug().Msg("getRecommendList1: " + v.Key)
// 		err = json.Unmarshal([]byte(v.Value), &content)
// 		if err != nil {
// 			log.Error().Err(err).Msg("")
// 			return []VmPriority{}, err
// 		}

// 		content2 := model.SpecInfo{}
// 		key2 := common.GenResourceKey(nsId, model.StrSpec, content.Id)

// 		keyValue2, err := kvstore.GetKv(key2)
// 		if err != nil {
// 			log.Error().Err(err).Msg("")
// 			return []VmPriority{}, err
// 		}
// 		json.Unmarshal([]byte(keyValue2.Value), &content2)
// 		content2.Id = content.Id

// 		vmPriorityTmp := VmPriority{}
// 		vmPriorityTmp.Priority = strconv.Itoa(cnt)
// 		vmPriorityTmp.VmSpec = content2
// 		vmPriorityList = append(vmPriorityList, vmPriorityTmp)
// 	}

// 	return vmPriorityList, err

// 	//requires error handling

// }

// // CorePostMciRecommend is func to command to all VMs in MCI with SSH
// func CorePostMciRecommend(nsId string, req *MciRecommendReq) ([]VmRecommendInfo, error) {

// 	err := common.CheckString(nsId)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return nil, err
// 	}

// 	/*
// 		var content struct {
// 			//VmReq          []VmRecommendReq    `json:"vmReq"`
// 			VmRecommend    []infra.VmRecommendInfo `json:"vmRecommend"`
// 			PlacementAlgo  string                   `json:"placementAlgo"`
// 			PlacementParam []common.KeyValue        `json:"placementParam"`
// 		}
// 	*/
// 	//content := RestPostMciRecommendResponse{}
// 	//content.VmReq = req.VmReq
// 	//content.PlacementAlgo = req.PlacementAlgo
// 	//content.PlacementParam = req.PlacementParam

// 	VmRecommend := []VmRecommendInfo{}

// 	vmList := req.VmReq

// 	for i, v := range vmList {
// 		vmTmp := VmRecommendInfo{}
// 		//vmTmp.RequestName = v.RequestName
// 		vmTmp.VmReq = req.VmReq[i]
// 		vmTmp.PlacementAlgo = v.PlacementAlgo
// 		vmTmp.PlacementParam = v.PlacementParam

// 		var err error
// 		vmTmp.VmPriority, err = GetRecommendList(nsId, v.VcpuSize, v.MemorySize, v.DiskSize)

// 		if err != nil {
// 			log.Error().Err(err).Msg("")
// 			return nil, fmt.Errorf("Failed to recommend MCI")
// 		}

// 		VmRecommend = append(VmRecommend, vmTmp)
// 	}

// 	return VmRecommend, nil
// }
