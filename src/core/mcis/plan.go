package mcis

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
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
	Metric    string      `json:"metric" example:"num_vCPU" enums:"num_vCPU,mem_GiB,Cost_per_hour"`
	Condition []Operation `json:"condition"`
}

// Operation is struct for .
type Operation struct {
	Operator string `json:"operator" example:">=" enums:">=,<=,=="` // >=, <=, ==
	Operand  string `json:"operand" example:"4" enums:"4,8,.."`     // 10, 70, 80, 98, ...
}

// PriorityInfo is struct for .
type PriorityInfo struct {
	Policy []PriorityCondition `json:"policy"`
}

// FilterCondition is struct for .
type PriorityCondition struct {
	Metric    string            `json:"metric" example:"location" enums:"location,latency,cost"` // location,latency,cost
	Weight    string            `json:"weight" example:"0.3" enums:"0.1,0.2,..."`                // 0.3
	Parameter []ParameterKeyVal `json:"parameter"`
}

// Operation is struct for .
type ParameterKeyVal struct {
	Key string   `json:"key" example:"coordinateClose" enums:"coordinateClose,coordinateWithin,coordinateFair"` // coordinate
	Val []string `json:"val" example:"46.3772/2.3730"`                                                          // ["Latitude,Longitude","12,543",..,"31,433"]
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
		metric := v.Metric
		conditions := v.Condition
		for _, condition := range conditions {

			operand64, err := strconv.ParseFloat(condition.Operand, 32)
			operand := float32(operand64)
			if err != nil {
				common.CBLog.Error(err)
				return []mcir.TbSpecInfo{}, err
			}

			switch condition.Operator {
			case "<=":

				switch metric {
				case "num_vCPU":
					u.Num_vCPU.Max = operand
				case "mem_GiB":
					u.Mem_GiB.Max = operand
				case "Cost_per_hour":
					u.Cost_per_hour.Max = operand
				default:
					fmt.Println("[Checking] Not available metric " + metric)
				}

			case ">=":

				switch metric {
				case "num_vCPU":
					u.Num_vCPU.Min = operand
				case "mem_GiB":
					u.Mem_GiB.Min = operand
				case "Cost_per_hour":
					u.Cost_per_hour.Min = operand
				default:
					fmt.Println("[Checking] Not available metric " + metric)
				}

			case "==":

				switch metric {
				case "num_vCPU":
					u.Num_vCPU.Max = operand
					u.Num_vCPU.Min = operand
				case "mem_GiB":
					u.Mem_GiB.Max = operand
					u.Mem_GiB.Min = operand
				case "Cost_per_hour":
					u.Cost_per_hour.Max = operand
					u.Cost_per_hour.Min = operand
				default:
					fmt.Println("[Checking] Not available metric " + metric)
				}
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
		case "latency":
			//
		case "cost":
			//
		default:
			// fmt.Println("[Checking] Not available metric " + metric)
		}

	}

	return prioritySpecs, nil

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
			latitude, err := strconv.ParseFloat(slice[0], 32)
			if err != nil {
				common.CBLog.Error(err)
				return []mcir.TbSpecInfo{}, err
			}
			longitude, err := strconv.ParseFloat(slice[1], 32)
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

			for i := range *specList {
				// update OrderInFilteredResult based on calculated priorityIndex
				(*specList)[distances[i].index].OrderInFilteredResult = uint16(distances[i].priorityIndex)
				// assign nomalized priorityIdex value to EvaluationScore_01
				(*specList)[distances[i].index].EvaluationScore_01 = float32(1 - (float32(distances[i].priorityIndex) / float32(len(*specList))))
				(*specList)[distances[i].index].EvaluationScore_02 = float32(distances[i].distance)
			}
			fmt.Printf("\n distances : %v \n", distances)

			//fmt.Printf("\n distances : %v \n", *specList)

		case "coordinateWithin":
			//
		case "coordinateFair":
			//
		default:
			// fmt.Println("[Checking] Not available metric " + metric)
		}

	}

	for i := range *specList {
		result = append(result, (*specList)[i])
		//result[i].OrderInFilteredResult = uint16(i + 1)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].OrderInFilteredResult < result[j].OrderInFilteredResult
	})
	fmt.Printf("\n result : %v \n", result)

	// updatedSpec, err := mcir.UpdateSpec(nsId, *result)
	// content, err = mcir.SortSpecs(*specList, "mem_GiB", "descending")
	return result, nil
}

// getDistance func get geographical distance between given coordinate and connectionConfig
func getDistance(latitude float64, longitude float64, ConnectionName string) (float64, error) {
	configTmp, _ := common.GetConnConfig(ConnectionName)
	regionTmp, _ := common.GetRegionInfo(configTmp.RegionName)

	nativeRegion := ""
	for _, v := range regionTmp.KeyValueInfoList {
		if strings.ToLower(v.Key) == "region" || strings.ToLower(v.Key) == "location" {
			nativeRegion = v.Value
			break
		}
	}
	Location := GetCloudLocation(strings.ToLower(configTmp.ProviderName), strings.ToLower(nativeRegion))

	cloudLatitude, err := strconv.ParseFloat(Location.Latitude, 32)
	if err != nil {
		common.CBLog.Error(err)
		return 0, err
	}
	cloudLongitude, err := strconv.ParseFloat(Location.Longitude, 32)
	if err != nil {
		common.CBLog.Error(err)
		return 0, err
	}

	// first := math.Pow(float64(cloudLatitude-latitude), 2)
	// second := math.Pow(float64(cloudLongitude-longitude), 2)
	// return math.Sqrt(first + second), nil
	return getHaversineDistance(cloudLatitude, cloudLongitude, latitude, longitude), nil

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
