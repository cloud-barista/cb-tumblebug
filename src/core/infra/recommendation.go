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
func applyFilterPolicies(request *model.FilterSpecsByRangeRequest, plan *model.DeploymentPlan) error {
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

// RecommendVm is func to recommend a VM
func RecommendVm(nsId string, plan model.DeploymentPlan) ([]model.TbSpecInfo, error) {
	// Filtering first

	u := &model.FilterSpecsByRangeRequest{}
	// Apply filter policies dynamically.
	if err := applyFilterPolicies(u, &plan); err != nil {
		log.Error().Err(err).Msg("Failed to apply filter policies")
		return nil, err
	}

	// veryLargeValue := float32(math.MaxFloat32)
	// verySmallValue := float32(0)

	// Filtering
	log.Debug().Msg("[Filtering specs]")

	startTime := time.Now()
	filteredSpecs, err := resource.FilterSpecsByRange(nsId, *u)

	if err != nil {
		log.Error().Err(err).Msg("")
		return []model.TbSpecInfo{}, err
	}
	elapsedTime := time.Since(startTime)
	log.Info().
		Int("filteredItemCount", len(filteredSpecs)).
		Dur("elapsedTime", elapsedTime).
		Msg("Filtering complete")

	if len(filteredSpecs) == 0 {
		return []model.TbSpecInfo{}, nil
	}

	// // sorting based on VCPU and MemoryGiB
	// sort.Slice(filteredSpecs, func(i, j int) bool {
	// 	// sort based on VCPU first
	// 	if filteredSpecs[i].VCPU != filteredSpecs[j].VCPU {
	// 		return float32(filteredSpecs[i].VCPU) < float32(filteredSpecs[j].VCPU)
	// 	}
	// 	// if VCPU is same, sort based on MemoryGiB
	// 	return float32(filteredSpecs[i].MemoryGiB) < float32(filteredSpecs[j].MemoryGiB)
	// })

	// Prioritizing
	log.Debug().Msg("[Prioritizing specs]")
	prioritySpecs := []model.TbSpecInfo{}

	startTime = time.Now()
	for _, v := range plan.Priority.Policy {
		metric := v.Metric

		switch metric {
		case "location":
			prioritySpecs, err = RecommendVmLocation(nsId, &filteredSpecs, &v.Parameter)
		case "performance":
			prioritySpecs, err = RecommendVmPerformance(nsId, &filteredSpecs)
		case "cost":
			prioritySpecs, err = RecommendVmCost(nsId, &filteredSpecs)
		case "random":
			prioritySpecs, err = RecommendVmRandom(nsId, &filteredSpecs)
		case "latency":
			prioritySpecs, err = RecommendVmLatency(nsId, &filteredSpecs, &v.Parameter)
		default:
			prioritySpecs, err = RecommendVmCost(nsId, &filteredSpecs)
		}

	}
	if plan.Priority.Policy == nil {
		prioritySpecs, err = RecommendVmCost(nsId, &filteredSpecs)
	}

	elapsedTime = time.Since(startTime)
	log.Info().
		Dur("elapsedTime", elapsedTime).
		Msg("Sorting complete")

	// limit the number of items in result list
	result := []model.TbSpecInfo{}
	limitNum, err := strconv.Atoi(plan.Limit)
	if err != nil {
		limitNum = math.MaxInt
	}
	for i, v := range prioritySpecs {
		result = append(result, v)
		if i == (limitNum - 1) {
			break
		}
	}

	return result, nil

}

// RecommendVmLatency func prioritize specs by latency based on given MCI (fair)
func RecommendVmLatency(nsId string, specList *[]model.TbSpecInfo, param *[]model.ParameterKeyVal) ([]model.TbSpecInfo, error) {

	result := []model.TbSpecInfo{}

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
				return (*specList)[i].CostPerHour < (*specList)[j].CostPerHour
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

	for i := range *specList {
		result = append(result, (*specList)[i])
		//result[i].OrderInFilteredResult = uint16(i + 1)
	}

	// Sorting result based on multiple criteria: OrderInFilteredResult, CostPerHour, VCPU, MemoryGiB
	sort.Slice(result, func(i, j int) bool {
		// 1st priority: OrderInFilteredResult
		if result[i].OrderInFilteredResult != result[j].OrderInFilteredResult {
			return result[i].OrderInFilteredResult < result[j].OrderInFilteredResult
		}
		// 2nd priority: CostPerHour
		if result[i].CostPerHour != result[j].CostPerHour {
			return result[i].CostPerHour < result[j].CostPerHour
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

// RecommendVmLocation func prioritize specs based on given location
func RecommendVmLocation(nsId string, specList *[]model.TbSpecInfo, param *[]model.ParameterKeyVal) ([]model.TbSpecInfo, error) {

	for _, v := range *param {

		switch v.Key {
		case "coordinateClose":
			//
			coordinateStr := v.Val[0]

			slice := strings.Split(coordinateStr, "/")
			latitude, err := strconv.ParseFloat(strings.ReplaceAll(slice[0], " ", ""), 32)
			if err != nil {
				log.Error().Err(err).Msg("")
				return []model.TbSpecInfo{}, err
			}
			longitude, err := strconv.ParseFloat(strings.ReplaceAll(slice[1], " ", ""), 32)
			if err != nil {
				log.Error().Err(err).Msg("")
				return []model.TbSpecInfo{}, err
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
				// return []model.TbSpecInfo{}, globalErr
			}

			sort.Slice(distances, func(i, j int) bool {
				return (*specList)[i].CostPerHour < (*specList)[j].CostPerHour
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
					return []model.TbSpecInfo{}, err
				}
				longitudeEach, err := strconv.ParseFloat(strings.ReplaceAll(slice[1], " ", ""), 32)
				if err != nil {
					log.Error().Err(err).Msg("")
					return []model.TbSpecInfo{}, err
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
							return
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
				return []model.TbSpecInfo{}, globalErr
			}

			sort.Slice(distances, func(i, j int) bool {
				return (*specList)[i].CostPerHour < (*specList)[j].CostPerHour
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

	result := append([]model.TbSpecInfo{}, (*specList)...)

	// Sorting result based on multiple criteria: OrderInFilteredResult, CostPerHour, VCPU, MemoryGiB
	sort.Slice(result, func(i, j int) bool {
		// 1st priority: OrderInFilteredResult
		if result[i].OrderInFilteredResult != result[j].OrderInFilteredResult {
			return result[i].OrderInFilteredResult < result[j].OrderInFilteredResult
		}
		// 2nd priority: CostPerHour
		if result[i].CostPerHour != result[j].CostPerHour {
			return result[i].CostPerHour < result[j].CostPerHour
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

	latencyString := common.RuntimeLatancyMap[common.RuntimeLatancyMapIndex[src]][common.RuntimeLatancyMapIndex[dest]]
	latency, err := strconv.ParseFloat(strings.ReplaceAll(latencyString, " ", ""), 32)
	if err != nil {
		log.Info().Err(err).Msgf("Cannot get GetLatency between src: %v, dest: %v (check assets)", src, dest)
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
func RecommendVmRandom(nsId string, specList *[]model.TbSpecInfo) ([]model.TbSpecInfo, error) {

	result := append([]model.TbSpecInfo{}, (*specList)...)

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
func RecommendVmCost(nsId string, specList *[]model.TbSpecInfo) ([]model.TbSpecInfo, error) {

	result := append([]model.TbSpecInfo{}, (*specList)...)

	sort.Slice(result, func(i, j int) bool { return result[i].CostPerHour < result[j].CostPerHour })

	Max := float32(result[len(result)-1].CostPerHour)
	Min := float32(result[0].CostPerHour)

	for i := range result {
		result[i].OrderInFilteredResult = uint16(i + 1)
		result[i].EvaluationScore09 = float32((Max - result[i].CostPerHour) / (Max - Min + 0.0000001)) // Add small value to avoid NaN by division
	}

	return result, nil
}

// RecommendVmPerformance func prioritize specs based on given Performance condition
func RecommendVmPerformance(nsId string, specList *[]model.TbSpecInfo) ([]model.TbSpecInfo, error) {

	result := append([]model.TbSpecInfo{}, (*specList)...)

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
func RecommendK8sNode(nsId string, plan model.DeploymentPlan) ([]model.TbSpecInfo, error) {
	emptyObjList := []model.TbSpecInfo{}

	limitOrig := plan.Limit
	plan.Limit = strconv.Itoa(math.MaxInt)

	tbSpecInfoListForVm, err := RecommendVm(nsId, plan)
	if err != nil {
		return emptyObjList, err
	}

	limitNum, err := strconv.Atoi(limitOrig)
	if err != nil {
		limitNum = math.MaxInt
	}

	tbSpecInfoListForK8s := []model.TbSpecInfo{}
	count := 0
	for _, tbSpecInfo := range tbSpecInfoListForVm {
		if strings.Contains(tbSpecInfo.InfraType, model.StrK8s) ||
			strings.Contains(tbSpecInfo.InfraType, model.StrKubernetes) {
			tbSpecInfoListForK8s = append(tbSpecInfoListForK8s, tbSpecInfo)
			count++
			if count == limitNum {
				break
			}
		}
	}

	return tbSpecInfoListForK8s, nil
}

// // GetRecommendList is func to get recommendation list
// func GetRecommendList(nsId string, cpuSize string, memSize string, diskSize string) ([]TbVmPriority, error) {

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
// 		return []TbVmPriority{}, err
// 	}

// 	var vmPriorityList []TbVmPriority

// 	for cnt, v := range keyValue {
// 		log.Debug().Msg("getRecommendList1: " + v.Key)
// 		err = json.Unmarshal([]byte(v.Value), &content)
// 		if err != nil {
// 			log.Error().Err(err).Msg("")
// 			return []TbVmPriority{}, err
// 		}

// 		content2 := model.TbSpecInfo{}
// 		key2 := common.GenResourceKey(nsId, model.StrSpec, content.Id)

// 		keyValue2, err := kvstore.GetKv(key2)
// 		if err != nil {
// 			log.Error().Err(err).Msg("")
// 			return []TbVmPriority{}, err
// 		}
// 		json.Unmarshal([]byte(keyValue2.Value), &content2)
// 		content2.Id = content.Id

// 		vmPriorityTmp := TbVmPriority{}
// 		vmPriorityTmp.Priority = strconv.Itoa(cnt)
// 		vmPriorityTmp.VmSpec = content2
// 		vmPriorityList = append(vmPriorityList, vmPriorityTmp)
// 	}

// 	return vmPriorityList, err

// 	//requires error handling

// }

// // CorePostMciRecommend is func to command to all VMs in MCI with SSH
// func CorePostMciRecommend(nsId string, req *MciRecommendReq) ([]TbVmRecommendInfo, error) {

// 	err := common.CheckString(nsId)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return nil, err
// 	}

// 	/*
// 		var content struct {
// 			//VmReq          []TbVmRecommendReq    `json:"vmReq"`
// 			VmRecommend    []infra.TbVmRecommendInfo `json:"vmRecommend"`
// 			PlacementAlgo  string                   `json:"placementAlgo"`
// 			PlacementParam []common.KeyValue        `json:"placementParam"`
// 		}
// 	*/
// 	//content := RestPostMciRecommendResponse{}
// 	//content.VmReq = req.VmReq
// 	//content.PlacementAlgo = req.PlacementAlgo
// 	//content.PlacementParam = req.PlacementParam

// 	VmRecommend := []TbVmRecommendInfo{}

// 	vmList := req.VmReq

// 	for i, v := range vmList {
// 		vmTmp := TbVmRecommendInfo{}
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
