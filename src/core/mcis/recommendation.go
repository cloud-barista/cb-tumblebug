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

// Package mcis is to manage multi-cloud infra service
package mcis

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"

	cbstore_utils "github.com/cloud-barista/cb-store/utils"
)

// DeploymentPlan is struct for .
type DeploymentPlan struct {
	Filter   FilterInfo   `json:"filter"`
	Priority PriorityInfo `json:"priority"`
	Limit    string       `json:"limit" example:"5" enums:"1,2,...,30,..."`
}

// FilterInfo is struct for .
type FilterInfo struct {
	Policy []FilterCondition `json:"policy"`
}

// FilterCondition is struct for .
type FilterCondition struct {
	Metric    string      `json:"metric" example:"cpu" enums:"cpu,memory,cost"`
	Condition []Operation `json:"condition"`
}

// Operation is struct for .
type Operation struct {
	Operator string `json:"operator" example:"<=" enums:">=,<=,=="` // >=, <=, ==
	Operand  string `json:"operand" example:"4" enums:"4,8,.."`     // 10, 70, 80, 98, ...
}

// PriorityInfo is struct for .
type PriorityInfo struct {
	Policy []PriorityCondition `json:"policy"`
}

// FilterCondition is struct for .
type PriorityCondition struct {
	Metric    string            `json:"metric" example:"location" enums:"location,cost,random,performance,latency"`
	Weight    string            `json:"weight" example:"0.3" enums:"0.1,0.2,..."`
	Parameter []ParameterKeyVal `json:"parameter,omitempty"`
}

// Operation is struct for .
type ParameterKeyVal struct {
	Key string   `json:"key" example:"coordinateClose" enums:"coordinateClose,coordinateWithin,coordinateFair"` // coordinate
	Val []string `json:"val" example:"44.146838/-116.411403"`                                                   // ["Latitude,Longitude","12,543",..,"31,433"]
}

///

//// Info manage for MCIS recommendation
func RecommendVm(nsId string, plan DeploymentPlan) ([]mcir.TbSpecInfo, error) {

	fmt.Println("RecommendVm")

	// Filtering first

	u := &mcir.FilterSpecsByRangeRequest{}

	// veryLargeValue := float32(math.MaxFloat32)
	// verySmallValue := float32(0)

	// Filtering
	fmt.Println("[Filtering specs]")

	for _, v := range plan.Filter.Policy {
		metric := mcir.ToNamingRuleCompatible(v.Metric)
		conditions := v.Condition
		for _, condition := range conditions {

			var operand64 float64
			var operand float32
			var err error
			if metric == "cpu" || metric == "memory" || metric == "cost" {
				operand64, err = strconv.ParseFloat(strings.ReplaceAll(condition.Operand, " ", ""), 32)
				operand = float32(operand64)
				if err != nil {
					common.CBLog.Error(err)
					return []mcir.TbSpecInfo{}, err
				}
			}

			switch metric {
			case "cpu":
				switch condition.Operator {
				case "<=":
					u.NumvCPU.Max = operand
				case ">=":
					u.NumvCPU.Min = operand
				case "==":
					u.NumvCPU.Max = operand
					u.NumvCPU.Min = operand
				}
			case "memory":
				switch condition.Operator {
				case "<=":
					u.MemGiB.Max = operand
				case ">=":
					u.MemGiB.Min = operand
				case "==":
					u.MemGiB.Max = operand
					u.MemGiB.Min = operand
				}
			case "cost":
				switch condition.Operator {
				case "<=":
					u.CostPerHour.Max = operand
				case ">=":
					u.CostPerHour.Min = operand
				case "==":
					u.CostPerHour.Max = operand
					u.CostPerHour.Min = operand
				}
			case "region":
				u.RegionName = condition.Operand
			case "provider":
				u.ProviderName = condition.Operand
			case "specname":
				u.CspSpecName = condition.Operand
			default:
				fmt.Println("[Checking] Not available metric " + metric)
			}
		}
	}

	filteredSpecs, err := mcir.FilterSpecsByRange(nsId, *u)

	if err != nil {
		common.CBLog.Error(err)
		return []mcir.TbSpecInfo{}, err
	}
	if len(filteredSpecs) == 0 {
		return []mcir.TbSpecInfo{}, nil
	}

	// Prioritizing
	fmt.Println("[Prioritizing specs]")
	prioritySpecs := []mcir.TbSpecInfo{}

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

	// limit the number of items in result list
	result := []mcir.TbSpecInfo{}
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

// RecommendVmLatency func prioritize specs by latency based on given MCIS (fair)
func RecommendVmLatency(nsId string, specList *[]mcir.TbSpecInfo, param *[]ParameterKeyVal) ([]mcir.TbSpecInfo, error) {

	result := []mcir.TbSpecInfo{}

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
					l, _ := GetLatency(region, k.RegionName)
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
			fmt.Printf("\n[Latency]\n %v \n", distances)

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
			// fmt.Println("[Checking] Not available metric " + metric)
		}

	}

	for i := range *specList {
		result = append(result, (*specList)[i])
		//result[i].OrderInFilteredResult = uint16(i + 1)
	}

	// if evaluations for distance are same, low cost will have priolity
	sort.Slice(result, func(i, j int) bool {
		if result[i].OrderInFilteredResult < result[j].OrderInFilteredResult {
			return true
		} else if result[i].OrderInFilteredResult > result[j].OrderInFilteredResult {
			return false
		} else {
			return result[i].CostPerHour < result[j].CostPerHour
		}
		//return result[i].OrderInFilteredResult < result[j].OrderInFilteredResult
	})
	// fmt.Printf("\n result : %v \n", result)

	// updatedSpec, err := mcir.UpdateSpec(nsId, *result)
	// content, err = mcir.SortSpecs(*specList, "memGiB", "descending")
	return result, nil
}

// RecommendVmLocation func prioritize specs based on given location
func RecommendVmLocation(nsId string, specList *[]mcir.TbSpecInfo, param *[]ParameterKeyVal) ([]mcir.TbSpecInfo, error) {

	result := []mcir.TbSpecInfo{}

	for _, v := range *param {

		switch v.Key {
		case "coordinateClose":
			//
			coordinateStr := v.Val[0]

			slice := strings.Split(coordinateStr, "/")
			latitude, err := strconv.ParseFloat(strings.ReplaceAll(slice[0], " ", ""), 32)
			if err != nil {
				common.CBLog.Error(err)
				return []mcir.TbSpecInfo{}, err
			}
			longitude, err := strconv.ParseFloat(strings.ReplaceAll(slice[1], " ", ""), 32)
			if err != nil {
				common.CBLog.Error(err)
				return []mcir.TbSpecInfo{}, err
			}

			type distanceType struct {
				distance      float64
				index         int
				priorityIndex int
			}
			distances := []distanceType{}

			for i := range *specList {
				distances = append(distances, distanceType{})
				distances[i].distance, err = getDistance(latitude, longitude, (*specList)[i].ConnectionName)
				if err != nil {
					common.CBLog.Error(err)
					return []mcir.TbSpecInfo{}, err
				}
				distances[i].index = i
			}

			sort.Slice(distances, func(i, j int) bool {
				return (*specList)[i].CostPerHour < (*specList)[j].CostPerHour
			})
			sort.Slice(distances, func(i, j int) bool {
				return distances[i].distance < distances[j].distance
			})
			fmt.Printf("\n distances : %v \n", distances)

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
			var err error

			// Calculate centroid of coordinate clusters
			latitudeSum := 0.0
			longitudeSum := 0.0
			for _, coordinateStr := range v.Val {
				slice := strings.Split(coordinateStr, "/")
				latitudeEach, err := strconv.ParseFloat(strings.ReplaceAll(slice[0], " ", ""), 32)
				if err != nil {
					common.CBLog.Error(err)
					return []mcir.TbSpecInfo{}, err
				}
				longitudeEach, err := strconv.ParseFloat(strings.ReplaceAll(slice[1], " ", ""), 32)
				if err != nil {
					common.CBLog.Error(err)
					return []mcir.TbSpecInfo{}, err
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
			distances := []distanceType{}

			for i := range *specList {
				distances = append(distances, distanceType{})
				distances[i].distance, err = getDistance(latitude, longitude, (*specList)[i].ConnectionName)
				if err != nil {
					common.CBLog.Error(err)
					return []mcir.TbSpecInfo{}, err
				}
				distances[i].index = i
			}

			sort.Slice(distances, func(i, j int) bool {
				return (*specList)[i].CostPerHour < (*specList)[j].CostPerHour
			})
			sort.Slice(distances, func(i, j int) bool {
				return distances[i].distance < distances[j].distance
			})
			fmt.Printf("\n distances : %v \n", distances)

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
			// fmt.Println("[Checking] Not available metric " + metric)
		}

	}

	for i := range *specList {
		result = append(result, (*specList)[i])
		//result[i].OrderInFilteredResult = uint16(i + 1)
	}

	// if evaluations for distance are same, low cost will have priolity
	sort.Slice(result, func(i, j int) bool {
		if result[i].OrderInFilteredResult < result[j].OrderInFilteredResult {
			return true
		} else if result[i].OrderInFilteredResult > result[j].OrderInFilteredResult {
			return false
		} else {
			return result[i].CostPerHour < result[j].CostPerHour
		}
		//return result[i].OrderInFilteredResult < result[j].OrderInFilteredResult
	})
	// fmt.Printf("\n result : %v \n", result)

	// updatedSpec, err := mcir.UpdateSpec(nsId, *result)
	// content, err = mcir.SortSpecs(*specList, "memGiB", "descending")
	return result, nil
}

// getDistance func get geographical distance between given coordinate and connectionConfig
func getDistance(latitude float64, longitude float64, ConnectionName string) (float64, error) {
	configTmp, _ := common.GetConnConfig(ConnectionName)
	regionTmp, _ := common.GetRegion(configTmp.RegionName)

	nativeRegion := ""
	for _, v := range regionTmp.KeyValueInfoList {
		if strings.ToLower(v.Key) == "region" || strings.ToLower(v.Key) == "location" {
			nativeRegion = v.Value
			break
		}
	}
	Location := common.GetCloudLocation(strings.ToLower(configTmp.ProviderName), strings.ToLower(nativeRegion))

	cloudLatitude, err := strconv.ParseFloat(strings.ReplaceAll(Location.Latitude, " ", ""), 32)
	if err != nil {
		common.CBLog.Error(err)
		return 0, err
	}
	cloudLongitude, err := strconv.ParseFloat(strings.ReplaceAll(Location.Longitude, " ", ""), 32)
	if err != nil {
		common.CBLog.Error(err)
		return 0, err
	}

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
		common.CBLog.Error(err)
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
func RecommendVmRandom(nsId string, specList *[]mcir.TbSpecInfo) ([]mcir.TbSpecInfo, error) {

	result := []mcir.TbSpecInfo{}

	for i := range *specList {
		result = append(result, (*specList)[i])
	}

	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(result), func(i, j int) { result[i], result[j] = result[j], result[i] })

	Max := float32(result[len(result)-1].CostPerHour)
	Min := float32(result[0].CostPerHour)

	for i := range result {
		result[i].OrderInFilteredResult = uint16(i + 1)
		result[i].EvaluationScore09 = float32((Max - result[i].CostPerHour) / (Max - Min + 0.0000001)) // Add small value to avoid NaN by division
	}

	fmt.Printf("\n result : %v \n", result)

	return result, nil
}

// RecommendVmCost func prioritize specs based on given Cost
func RecommendVmCost(nsId string, specList *[]mcir.TbSpecInfo) ([]mcir.TbSpecInfo, error) {

	result := []mcir.TbSpecInfo{}

	for i := range *specList {
		result = append(result, (*specList)[i])
	}

	sort.Slice(result, func(i, j int) bool { return result[i].CostPerHour < result[j].CostPerHour })

	Max := float32(result[len(result)-1].CostPerHour)
	Min := float32(result[0].CostPerHour)

	for i := range result {
		result[i].OrderInFilteredResult = uint16(i + 1)
		result[i].EvaluationScore09 = float32((Max - result[i].CostPerHour) / (Max - Min + 0.0000001)) // Add small value to avoid NaN by division
	}

	fmt.Printf("\n result : %v \n", result)

	return result, nil
}

// RecommendVmPerformance func prioritize specs based on given Performance condition
func RecommendVmPerformance(nsId string, specList *[]mcir.TbSpecInfo) ([]mcir.TbSpecInfo, error) {

	result := []mcir.TbSpecInfo{}

	for i := range *specList {
		result = append(result, (*specList)[i])
	}

	sort.Slice(result, func(i, j int) bool { return result[i].EvaluationScore01 > result[j].EvaluationScore01 })

	Max := float32(result[0].EvaluationScore01)
	Min := float32(result[len(result)-1].EvaluationScore01)

	for i := range result {
		result[i].OrderInFilteredResult = uint16(i + 1)
		result[i].EvaluationScore09 = float32((result[i].EvaluationScore01 - Min) / (Max - Min + 0.0000001)) // Add small value to avoid NaN by division
	}
	fmt.Printf("\n result : %v \n", result)

	return result, nil
}

// GetRecommendList is func to get recommendation list
func GetRecommendList(nsId string, cpuSize string, memSize string, diskSize string) ([]TbVmPriority, error) {

	fmt.Println("GetRecommendList")

	var content struct {
		Id             string
		Price          string
		ConnectionName string
	}

	key := common.GenMcisKey(nsId, "", "") + "/cpuSize/" + cpuSize + "/memSize/" + memSize + "/diskSize/" + diskSize
	fmt.Println(key)
	keyValue, err := common.CBStore.GetList(key, true)
	keyValue = cbstore_utils.GetChildList(keyValue, key)
	if err != nil {
		common.CBLog.Error(err)
		return []TbVmPriority{}, err
	}

	var vmPriorityList []TbVmPriority

	for cnt, v := range keyValue {
		fmt.Println("getRecommendList1: " + v.Key)
		err = json.Unmarshal([]byte(v.Value), &content)
		if err != nil {
			common.CBLog.Error(err)
			return []TbVmPriority{}, err
		}

		content2 := mcir.TbSpecInfo{}
		key2 := common.GenResourceKey(nsId, common.StrSpec, content.Id)

		keyValue2, err := common.CBStore.Get(key2)
		if err != nil {
			common.CBLog.Error(err)
			return []TbVmPriority{}, err
		}
		json.Unmarshal([]byte(keyValue2.Value), &content2)
		content2.Id = content.Id

		vmPriorityTmp := TbVmPriority{}
		vmPriorityTmp.Priority = strconv.Itoa(cnt)
		vmPriorityTmp.VmSpec = content2
		vmPriorityList = append(vmPriorityList, vmPriorityTmp)
	}

	fmt.Println("===============================================")
	return vmPriorityList, err

	//requires error handling

}

// CorePostMcisRecommend is func to command to all VMs in MCIS with SSH
func CorePostMcisRecommend(nsId string, req *McisRecommendReq) ([]TbVmRecommendInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	/*
		var content struct {
			//VmReq          []TbVmRecommendReq    `json:"vmReq"`
			VmRecommend    []mcis.TbVmRecommendInfo `json:"vmRecommend"`
			PlacementAlgo  string                   `json:"placementAlgo"`
			PlacementParam []common.KeyValue        `json:"placementParam"`
		}
	*/
	//content := RestPostMcisRecommendResponse{}
	//content.VmReq = req.VmReq
	//content.PlacementAlgo = req.PlacementAlgo
	//content.PlacementParam = req.PlacementParam

	VmRecommend := []TbVmRecommendInfo{}

	vmList := req.VmReq

	for i, v := range vmList {
		vmTmp := TbVmRecommendInfo{}
		//vmTmp.RequestName = v.RequestName
		vmTmp.VmReq = req.VmReq[i]
		vmTmp.PlacementAlgo = v.PlacementAlgo
		vmTmp.PlacementParam = v.PlacementParam

		var err error
		vmTmp.VmPriority, err = GetRecommendList(nsId, v.VcpuSize, v.MemorySize, v.DiskSize)

		if err != nil {
			common.CBLog.Error(err)
			return nil, fmt.Errorf("Failed to recommend MCIS")
		}

		VmRecommend = append(VmRecommend, vmTmp)
	}

	return VmRecommend, nil
}
