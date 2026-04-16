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

// Package infra is to manage multi-cloud infra
package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/rs/zerolog/log"

	"math"
	"reflect"
	"sync"
	"time"

	validator "github.com/go-playground/validator/v10"
)

// use a single instance of Validate, it caches struct info
var validate *validator.Validate

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

	validate.RegisterStructValidation(InfraReqStructLevelValidation, model.InfraReq{})
	validate.RegisterStructValidation(CreateNodeGroupReqStructLevelValidation, model.CreateNodeGroupReq{})
	validate.RegisterStructValidation(TbInfraCmdReqStructLevelValidation, model.InfraCmdReq{})
	// validate.RegisterStructValidation(TbInfraRecommendReqStructLevelValidation, InfraRecommendReq{})
	// validate.RegisterStructValidation(VmRecommendReqStructLevelValidation, VmRecommendReq{})
	// validate.RegisterStructValidation(TbBenchmarkReqStructLevelValidation, BenchmarkReq{})
	// validate.RegisterStructValidation(TbMultihostBenchmarkReqStructLevelValidation, MultihostBenchmarkReq{})

	validate.RegisterStructValidation(DFMonAgentInstallReqStructLevelValidation, model.MonAgentInstallReq{})

}

// CheckInfra func is to check given infraId is duplicated with existing
func CheckInfra(nsId string, infraId string) (bool, error) {

	// Check parameters' emptiness
	if nsId == "" {
		err := fmt.Errorf("CheckInfra failed; nsId given is empty.")
		return false, err
	} else if infraId == "" {
		err := fmt.Errorf("CheckInfra failed; infraId given is empty.")
		return false, err
	}

	key := common.GenInfraKey(nsId, infraId, "")

	_, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("In CheckInfra(); kvstore.GetKv() returned an error.")
		log.Error().Err(err).Msg("")
		// return nil, err
	}

	if exists {
		return true, nil
	}
	return false, nil

}

// CheckNodeGroup func is to check given nodeGroupId is duplicated with existing
func CheckNodeGroup(nsId string, infraId string, nodeGroupId string) (bool, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	err = common.CheckString(infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	nodeGroupList, err := ListNodeGroupId(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}
	for _, v := range nodeGroupList {
		if strings.EqualFold(v, nodeGroupId) {
			return true, nil
		}
	}
	return false, nil
}

func CheckNode(nsId string, infraId string, nodeId string) (bool, error) {

	// Check parameters' emptiness
	if nsId == "" {
		err := fmt.Errorf("CheckNode failed; nsId given is null.")
		return false, err
	} else if infraId == "" {
		err := fmt.Errorf("CheckNode failed; infraId given is null.")
		return false, err
	} else if nodeId == "" {
		err := fmt.Errorf("CheckNode failed; nodeId given is null.")
		return false, err
	}

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	err = common.CheckString(infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}
	err = common.CheckString(nodeId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}
	//log.Debug().Msg("[Check node] " + infraId + ", " + nodeId)

	key := common.GenInfraKey(nsId, infraId, nodeId)

	_, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("In CheckNode(); kvstore.GetKv() returned an error.")
		log.Error().Err(err).Msg("")
		// return nil, err
	}

	if exists {
		return true, nil
	}
	return false, nil

}

func CheckInfraPolicy(nsId string, infraId string) (bool, error) {

	// Check parameters' emptiness
	if nsId == "" {
		err := fmt.Errorf("CheckInfra failed; nsId given is null.")
		return false, err
	} else if infraId == "" {
		err := fmt.Errorf("CheckInfra failed; infraId given is null.")
		return false, err
	}

	// err := common.CheckString(nsId)
	// if err != nil {
	// 	log.Error().Err(err).Msg("")
	// 	return false, err
	// }

	// err = common.CheckString(infraId)
	// if err != nil {
	// 	log.Error().Err(err).Msg("")
	// 	return false, err
	// }
	log.Debug().Msg("[Check InfraPolicy] " + infraId)

	key := common.GenInfraPolicyKey(nsId, infraId, "")

	_, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("In CheckInfraPolicy(); kvstore.GetKv() returned an error.")
		log.Error().Err(err).Msg("")
		// return nil, err
	}

	if exists {
		return true, nil
	}
	return false, nil

}

func TrimIP(sshAccessPoint string) (string, error) {
	splitted := strings.Split(sshAccessPoint, ":")
	if len(splitted) != 2 {
		err := fmt.Errorf("In TrimIP(), sshAccessPoint does not seem 8.8.8.8:22 form.")
		return strconv.Itoa(0), err
	}
	portString := splitted[1]
	port, err := strconv.Atoi(portString)
	if err != nil {
		err := fmt.Errorf("In TrimIP(), strconv.Atoi returned an error.")
		return strconv.Itoa(0), err
	}
	if port >= 1 && port <= 65535 { // valid port number
		return portString, nil
	} else {
		err := fmt.Errorf("In TrimIP(), detected port number seems wrong: " + portString)
		return strconv.Itoa(0), err
	}
}

// InspectResources returns the state list of TB Resource objects of given connConfig and resourceType
func InspectResources(connConfig string, resourceType string) (model.InspectResource, error) {
	nullObj := model.InspectResource{}

	// get providerName from connection
	providerName, err := common.GetProviderNameFromConnConfig(connConfig)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nullObj, err
	}

	nsList, err := common.ListNsId()
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("an error occurred while getting namespaces' list: " + err.Error())
		return nullObj, err
	}
	TbResourceList := model.ResourceOnTumblebug{}
	for _, ns := range nsList {

		// Bring TB resources
		switch resourceType {
		case model.StrNLB:
			infraListinNs, _ := ListInfraId(ns)
			if infraListinNs == nil {
				continue
			}
			for _, infra := range infraListinNs {
				nlbListInInfra, err := ListNLBId(ns, infra)
				if err != nil {
					log.Error().Err(err).Msg("")
					err := fmt.Errorf("an error occurred while getting resource list")
					return nullObj, err
				}
				if nlbListInInfra == nil {
					continue
				}

				for _, nlbId := range nlbListInInfra {
					nlb, err := GetNLB(ns, infra, nlbId)
					if err != nil {
						log.Error().Err(err).Msg("")
						err := fmt.Errorf("an error occurred while getting resource list")
						return nullObj, err
					}

					if nlb.ConnectionName == connConfig { // filtering
						temp := model.ResourceOnTumblebugInfo{}
						temp.IdByTb = nlb.Id
						temp.CspResourceId = nlb.CspResourceId
						temp.NsId = ns
						temp.InfraId = infra
						temp.ObjectKey = GenNLBKey(ns, infra, nlb.Id)

						TbResourceList.Info = append(TbResourceList.Info, temp)
					}
				}
			}
		case model.StrNode:
			infraListinNs, _ := ListInfraId(ns)
			if infraListinNs == nil {
				continue
			}
			for _, infra := range infraListinNs {
				nodeListInInfra, err := ListNodeId(ns, infra)
				if err != nil {
					log.Error().Err(err).Msg("")
					err := fmt.Errorf("an error occurred while getting resource list")
					return nullObj, err
				}
				if nodeListInInfra == nil {
					continue
				}

				for _, nodeId := range nodeListInInfra {
					node, err := GetNodeObject(ns, infra, nodeId)
					if err != nil {
						log.Error().Err(err).Msg("")
						err := fmt.Errorf("an error occurred while getting resource list")
						return nullObj, err
					}

					if node.ConnectionName == connConfig { // filtering
						temp := model.ResourceOnTumblebugInfo{}
						temp.IdByTb = node.Id
						temp.CspResourceId = node.CspResourceId
						temp.NsId = ns
						temp.InfraId = infra
						temp.ObjectKey = common.GenInfraKey(ns, infra, node.Id)

						TbResourceList.Info = append(TbResourceList.Info, temp)
					}
				}
			}
		case model.StrVNet:
			resourceListInNs, err := resource.ListResource(ns, resourceType, "", "")
			if err != nil {
				log.Error().Err(err).Msg("")
				err := fmt.Errorf("an error occurred while getting resource list")
				return nullObj, err
			}
			resourcesInNs := resourceListInNs.([]model.VNetInfo) // type assertion
			if len(resourcesInNs) == 0 {
				continue
			}
			for _, resource := range resourcesInNs {
				if resource.ConnectionName == connConfig { // filtering
					temp := model.ResourceOnTumblebugInfo{}
					temp.IdByTb = resource.Id
					temp.CspResourceId = resource.CspResourceId
					temp.NsId = ns
					temp.ObjectKey = common.GenResourceKey(ns, resourceType, resource.Id)

					TbResourceList.Info = append(TbResourceList.Info, temp)
				}
			}
		case model.StrSubnet:
			// Subnet resources are managed as child resources of VNet
			// We need to iterate through VNets and then their subnets
			vnetListInNs, err := resource.ListResource(ns, model.StrVNet, "", "")
			if err != nil {
				log.Error().Err(err).Msg("")
				err := fmt.Errorf("an error occurred while getting VNet list for subnet inspection")
				return nullObj, err
			}
			vnetsInNs := vnetListInNs.([]model.VNetInfo) // type assertion
			if len(vnetsInNs) == 0 {
				continue
			}
			for _, vnet := range vnetsInNs {
				if vnet.ConnectionName == connConfig { // filtering by connection
					// Get subnets for this VNet
					for _, subnet := range vnet.SubnetInfoList {
						temp := model.ResourceOnTumblebugInfo{}
						temp.IdByTb = subnet.Id
						temp.CspResourceId = subnet.CspResourceId
						temp.NsId = ns
						temp.ObjectKey = common.GenResourceKey(ns, model.StrVNet, vnet.Id) + "/" + model.StrSubnet + "/" + subnet.Id

						TbResourceList.Info = append(TbResourceList.Info, temp)
					}
				}
			}
		case model.StrSecurityGroup:
			resourceListInNs, err := resource.ListResource(ns, resourceType, "", "")
			if err != nil {
				log.Error().Err(err).Msg("")
				err := fmt.Errorf("an error occurred while getting resource list")
				return nullObj, err
			}
			resourcesInNs := resourceListInNs.([]model.SecurityGroupInfo) // type assertion
			if len(resourcesInNs) == 0 {
				continue
			}
			for _, resource := range resourcesInNs {
				if resource.ConnectionName == connConfig { // filtering
					temp := model.ResourceOnTumblebugInfo{}
					temp.IdByTb = resource.Id
					temp.CspResourceId = resource.CspResourceId
					temp.NsId = ns
					temp.ObjectKey = common.GenResourceKey(ns, resourceType, resource.Id)

					TbResourceList.Info = append(TbResourceList.Info, temp)
				}
			}
		case model.StrSSHKey:
			resourceListInNs, err := resource.ListResource(ns, resourceType, "", "")
			if err != nil {
				log.Error().Err(err).Msg("")
				err := fmt.Errorf("an error occurred while getting resource list")
				return nullObj, err
			}
			resourcesInNs := resourceListInNs.([]model.SshKeyInfo) // type assertion
			if len(resourcesInNs) == 0 {
				continue
			}
			for _, resource := range resourcesInNs {
				if resource.ConnectionName == connConfig { // filtering
					temp := model.ResourceOnTumblebugInfo{}
					temp.IdByTb = resource.Id
					temp.CspResourceId = resource.CspResourceId
					temp.NsId = ns
					temp.ObjectKey = common.GenResourceKey(ns, resourceType, resource.Id)

					TbResourceList.Info = append(TbResourceList.Info, temp)
				}
			}
		case model.StrDataDisk:
			resourceListInNs, err := resource.ListResource(ns, resourceType, "", "")
			if err != nil {
				log.Error().Err(err).Msg("")
				err := fmt.Errorf("an error occurred while getting resource list")
				return nullObj, err
			}
			resourcesInNs := resourceListInNs.([]model.DataDiskInfo) // type assertion
			if len(resourcesInNs) == 0 {
				continue
			}
			for _, resource := range resourcesInNs {
				if resource.ConnectionName == connConfig { // filtering
					temp := model.ResourceOnTumblebugInfo{}
					temp.IdByTb = resource.Id
					temp.CspResourceId = resource.CspResourceId
					temp.NsId = ns
					temp.ObjectKey = common.GenResourceKey(ns, resourceType, resource.Id)

					TbResourceList.Info = append(TbResourceList.Info, temp)
				}
			}
		case model.StrCustomImage:
			resourceListInNs, err := resource.ListResource(ns, resourceType, "", "")
			if err != nil {
				log.Error().Err(err).Msg("")
				err := fmt.Errorf("an error occurred while getting resource list")
				return nullObj, err
			}
			resourcesInNs := resourceListInNs.([]model.ImageInfo) // type assertion
			if len(resourcesInNs) == 0 {
				continue
			}
			for _, resource := range resourcesInNs {
				if resource.ConnectionName == connConfig { // filtering
					temp := model.ResourceOnTumblebugInfo{}
					temp.IdByTb = resource.Id
					temp.CspResourceId = resource.CspImageId
					temp.NsId = ns
					temp.ObjectKey = common.GenResourceKey(ns, resourceType, resource.Id)

					TbResourceList.Info = append(TbResourceList.Info, temp)
				}
			}
		default:
			err = fmt.Errorf("Invalid resourceType: " + resourceType)
			return nullObj, err
		}
	}

	// Use helper function to get CSP resource status from CB-Spider
	cspResourceStatus, err := resource.GetCspResourceStatus(connConfig, resourceType)
	if err != nil {
		log.Error().Err(err).Str("connection", connConfig).Str("resourceType", resourceType).
			Msg("Failed to get CSP resource status")
		result := model.InspectResource{}
		result.ConnectionName = connConfig
		result.ResourceType = resourceType
		result.SystemMessage = fmt.Sprintf("Failed to get CSP resource status: %v", err)
		result.Resources.OnTumblebug = TbResourceList
		return result, err
	}

	result := model.InspectResource{}

	/*
		// Implementation style 1
		if len(TbResourceList) > 0 {
			result.ResourcesOnTumblebug = TbResourceList
		} else {
			result.ResourcesOnTumblebug = []resourceOnTumblebug{}
		}
	*/
	// Implementation style 2
	result.ConnectionName = connConfig
	result.ResourceType = resourceType
	result.SystemMessage = cspResourceStatus.SystemMessage

	result.Resources.OnTumblebug = TbResourceList
	//result.ResourcesOnTumblebug.Info = append(result.ResourcesOnTumblebug.Info, TbResourceList...)

	// Use data from helper function instead of direct Spider call
	result.Resources.OnSpider = model.ResourceOnSpider{}
	result.Resources.OnCspTotal = model.ResourceOnCsp{}
	result.Resources.OnCspOnly = model.ResourceOnCsp{}

	tmpResourceOnSpider := model.ResourceOnSpiderInfo{}
	tmpResourceOnCsp := model.ResourceOnCspInfo{}

	for _, v := range cspResourceStatus.AllList.MappedList {
		tmpResourceOnSpider.IdBySp = v.NameId
		tmpResourceOnSpider.CspResourceId = v.SystemId
		result.Resources.OnSpider.Info = append(result.Resources.OnSpider.Info, tmpResourceOnSpider)

		tmpResourceOnCsp.CspResourceId = v.SystemId
		tmpResourceOnCsp.RefNameOrId = v.NameId
		result.Resources.OnCspTotal.Info = append(result.Resources.OnCspTotal.Info, tmpResourceOnCsp)
	}

	for _, v := range cspResourceStatus.AllList.OnlySpiderList {
		tmpResourceOnSpider.IdBySp = v.NameId
		tmpResourceOnSpider.CspResourceId = v.SystemId
		result.Resources.OnSpider.Info = append(result.Resources.OnSpider.Info, tmpResourceOnSpider)
	}

	for _, v := range cspResourceStatus.AllList.OnlyCSPList {
		tmpResourceOnCsp.CspResourceId = v.SystemId
		// Azure has different ID for NameId and SystemId
		if csp.ResolveCloudPlatform(providerName) == csp.Azure {
			if resourceType != model.StrDataDisk {
				tmpResourceOnCsp.CspResourceId = v.NameId
			}
		}
		tmpResourceOnCsp.RefNameOrId = v.NameId

		result.Resources.OnCspTotal.Info = append(result.Resources.OnCspTotal.Info, tmpResourceOnCsp)
		result.Resources.OnCspOnly.Info = append(result.Resources.OnCspOnly.Info, tmpResourceOnCsp)
	}

	// Count resources
	result.Resources.OnTumblebug.Count = len(result.Resources.OnTumblebug.Info)
	result.Resources.OnSpider.Count = len(result.Resources.OnSpider.Info)
	result.Resources.OnCspTotal.Count = len(result.Resources.OnCspTotal.Info)
	result.Resources.OnCspOnly.Count = len(result.Resources.OnCspOnly.Info)
	result.ResourceOverview.OnTumblebug = result.Resources.OnTumblebug.Count
	result.ResourceOverview.OnSpider = result.Resources.OnSpider.Count
	result.ResourceOverview.OnCspTotal = result.Resources.OnCspTotal.Count
	result.ResourceOverview.OnCspOnly = result.Resources.OnCspOnly.Count

	return result, nil
}

// InspectResourcesOverview func is to check all resources in CB-TB and CSPs
func InspectResourcesOverview() (model.InspectResourceAllResult, error) {
	startTime := time.Now()

	connectionConfigList, err := common.GetConnConfigList(model.DefaultCredentialHolder, true, true)
	if err != nil {
		err := fmt.Errorf("Cannot load ConnectionConfigList")
		log.Error().Err(err).Msg("")
		return model.InspectResourceAllResult{}, err
	}

	totalConnections := len(connectionConfigList.Connectionconfig)
	output := model.InspectResourceAllResult{}

	if totalConnections == 0 {
		return output, nil
	}

	// Use channel to collect results safely (no data race on append)
	resultChan := make(chan model.InspectResourceResult, totalConnections)

	// Use global semaphore to limit concurrent inspect operations
	inspectSemaphore := make(chan struct{}, csp.GlobalMaxConcurrentConnections)

	var wait sync.WaitGroup
	for _, k := range connectionConfigList.Connectionconfig {
		wait.Add(1)
		go func(k model.ConnConfig) {
			defer wait.Done()

			// Acquire semaphore to limit concurrency
			inspectSemaphore <- struct{}{}
			defer func() { <-inspectSemaphore }()

			common.RandomSleep(0, 60*1000)
			temp := model.InspectResourceResult{}
			temp.ConnectionName = k.ConfigName
			startTimeForConnection := time.Now()

			inspectResult, err := InspectResources(k.ConfigName, model.StrVNet)
			if err != nil {
				log.Error().Err(err).Msg("")
				temp.SystemMessage = err.Error()
			}
			// retry if request rateLimitExceeded occurs. (GCP has ratelimiting)
			rateLimitMessage := "limit"
			maxTrials := 5
			if strings.Contains(temp.SystemMessage, rateLimitMessage) {
				for i := 0; i < maxTrials; i++ {
					common.RandomSleep(40*1000, 80*1000)
					inspectResult, err = InspectResources(k.ConfigName, model.StrVNet)
					if err != nil {
						log.Error().Err(err).Msg("")
						temp.SystemMessage = err.Error()
					} else {
						temp.SystemMessage = ""
						break
					}
				}
			}
			temp.TumblebugOverview.VNet = inspectResult.ResourceOverview.OnTumblebug
			temp.CspOnlyOverview.VNet = inspectResult.ResourceOverview.OnCspOnly

			inspectResult, err = InspectResources(k.ConfigName, model.StrSecurityGroup)
			if err != nil {
				log.Error().Err(err).Msg("")
				temp.SystemMessage += err.Error()
			}
			temp.TumblebugOverview.SecurityGroup = inspectResult.ResourceOverview.OnTumblebug
			temp.CspOnlyOverview.SecurityGroup = inspectResult.ResourceOverview.OnCspOnly

			inspectResult, err = InspectResources(k.ConfigName, model.StrSSHKey)
			if err != nil {
				log.Error().Err(err).Msg("")
				temp.SystemMessage += err.Error()
			}
			temp.TumblebugOverview.SshKey = inspectResult.ResourceOverview.OnTumblebug
			temp.CspOnlyOverview.SshKey = inspectResult.ResourceOverview.OnCspOnly

			inspectResult, err = InspectResources(k.ConfigName, model.StrDataDisk)
			if err != nil {
				log.Error().Err(err).Msg("")
				temp.SystemMessage += err.Error()
			}
			temp.TumblebugOverview.DataDisk = inspectResult.ResourceOverview.OnTumblebug
			temp.CspOnlyOverview.DataDisk = inspectResult.ResourceOverview.OnCspOnly

			inspectResult, err = InspectResources(k.ConfigName, model.StrCustomImage)
			if err != nil {
				log.Error().Err(err).Msg("")
				temp.SystemMessage += err.Error()
			}
			temp.TumblebugOverview.CustomImage = inspectResult.ResourceOverview.OnTumblebug
			temp.CspOnlyOverview.CustomImage = inspectResult.ResourceOverview.OnCspOnly

			inspectResult, err = InspectResources(k.ConfigName, model.StrNode)
			if err != nil {
				log.Error().Err(err).Msg("")
				temp.SystemMessage += err.Error()
			}
			temp.TumblebugOverview.Node = inspectResult.ResourceOverview.OnTumblebug
			temp.CspOnlyOverview.Node = inspectResult.ResourceOverview.OnCspOnly

			inspectResult, err = InspectResources(k.ConfigName, model.StrNLB)
			if err != nil {
				log.Error().Err(err).Msg("")
				temp.SystemMessage += err.Error()
			}
			temp.TumblebugOverview.NLB = inspectResult.ResourceOverview.OnTumblebug
			temp.CspOnlyOverview.NLB = inspectResult.ResourceOverview.OnCspOnly

			temp.ElapsedTime = int(math.Round(time.Since(startTimeForConnection).Seconds()))

			// Send result to channel (safe for concurrent sends)
			resultChan <- temp

		}(k)
	}

	// Close channel after all goroutines complete
	go func() {
		wait.Wait()
		close(resultChan)
	}()

	// Collect results from channel (single consumer — no race)
	for temp := range resultChan {
		output.InspectResult = append(output.InspectResult, temp)
	}

	errorConnectionCnt := 0
	for _, k := range output.InspectResult {
		output.TumblebugOverview.VNet += k.TumblebugOverview.VNet
		output.TumblebugOverview.SecurityGroup += k.TumblebugOverview.SecurityGroup
		output.TumblebugOverview.SshKey += k.TumblebugOverview.SshKey
		output.TumblebugOverview.DataDisk += k.TumblebugOverview.DataDisk
		output.TumblebugOverview.CustomImage += k.TumblebugOverview.CustomImage
		output.TumblebugOverview.Node += k.TumblebugOverview.Node
		output.TumblebugOverview.NLB += k.TumblebugOverview.NLB

		output.CspOnlyOverview.VNet += k.CspOnlyOverview.VNet
		output.CspOnlyOverview.SecurityGroup += k.CspOnlyOverview.SecurityGroup
		output.CspOnlyOverview.SshKey += k.CspOnlyOverview.SshKey
		output.CspOnlyOverview.DataDisk += k.CspOnlyOverview.DataDisk
		output.CspOnlyOverview.CustomImage += k.CspOnlyOverview.CustomImage
		output.CspOnlyOverview.Node += k.CspOnlyOverview.Node
		output.CspOnlyOverview.NLB += k.CspOnlyOverview.NLB

		if k.SystemMessage != "" {
			errorConnectionCnt++
		}
	}

	sort.SliceStable(output.InspectResult, func(i, j int) bool {
		return output.InspectResult[i].ConnectionName < output.InspectResult[j].ConnectionName
	})

	output.ElapsedTime = int(math.Round(time.Since(startTime).Seconds()))
	output.RegisteredConnection = totalConnections
	output.AvailableConnection = totalConnections - errorConnectionCnt

	return output, err
}

// GetAssetsSummary returns provider-level summary of spec/image assets for a namespace.
func GetAssetsSummary(nsId string) (model.AssetsSummaryResponse, error) {
	nsId = strings.TrimSpace(nsId)
	if nsId == "" {
		nsId = model.SystemCommonNs
	}

	result := model.AssetsSummaryResponse{
		NamespaceID: nsId,
		Providers:   make([]model.ProviderAssetSummary, 0),
	}

	type specAggRow struct {
		ProviderName      string
		SpecCount         int64
		PricedSpecCount   int64
		UnpricedSpecCount int64
	}

	var specAgg []specAggRow
	err := model.ORM.Model(&model.SpecInfo{}).
		Select(`provider_name as provider_name,
			COUNT(*) as spec_count,
			SUM(CASE WHEN cost_per_hour <> -1 THEN 1 ELSE 0 END) as priced_spec_count,
			SUM(CASE WHEN cost_per_hour = -1 THEN 1 ELSE 0 END) as unpriced_spec_count`).
		Where("namespace = ?", nsId).
		Group("provider_name").
		Scan(&specAgg).Error
	if err != nil {
		return model.AssetsSummaryResponse{}, fmt.Errorf("failed to summarize specs for namespace %s: %w", nsId, err)
	}

	type imageAggRow struct {
		ProviderName string
		ImageCount   int64
	}

	var imageAgg []imageAggRow
	err = model.ORM.Model(&model.ImageInfo{}).
		Select("provider_name as provider_name, COUNT(*) as image_count").
		Where("namespace = ? AND resource_type = ?", nsId, model.StrImage).
		Group("provider_name").
		Scan(&imageAgg).Error
	if err != nil {
		return model.AssetsSummaryResponse{}, fmt.Errorf("failed to summarize images for namespace %s: %w", nsId, err)
	}

	providerMap := make(map[string]*model.ProviderAssetSummary)

	for _, row := range specAgg {
		providerName := strings.TrimSpace(row.ProviderName)
		if providerName == "" {
			providerName = "unknown"
		}

		providerMap[providerName] = &model.ProviderAssetSummary{
			ProviderName:      providerName,
			SpecCount:         row.SpecCount,
			PricedSpecCount:   row.PricedSpecCount,
			UnpricedSpecCount: row.UnpricedSpecCount,
		}

		result.TotalSpecCount += row.SpecCount
		result.PricedSpecCount += row.PricedSpecCount
		result.UnpricedSpecCount += row.UnpricedSpecCount
	}

	for _, row := range imageAgg {
		providerName := strings.TrimSpace(row.ProviderName)
		if providerName == "" {
			providerName = "unknown"
		}

		if providerMap[providerName] == nil {
			providerMap[providerName] = &model.ProviderAssetSummary{ProviderName: providerName}
		}
		providerMap[providerName].ImageCount = row.ImageCount
		result.TotalImageCount += row.ImageCount
	}

	providerNames := make([]string, 0, len(providerMap))
	for name := range providerMap {
		providerNames = append(providerNames, name)
	}
	sort.Strings(providerNames)

	for _, name := range providerNames {
		result.Providers = append(result.Providers, *providerMap[name])
	}

	return result, nil
}

// getRegisterRateLimitsForCSP returns rate limiting config for resource registration.
// Uses centralized CSP config from csp.GetRateLimitConfig() with built-in fallback for unknown CSPs.
func getRegisterRateLimitsForCSP(providerName string) (maxConns int, delayMinMs int, delayMaxMs int) {
	config := csp.GetRateLimitConfig(providerName)
	return config.MaxConcurrentRegistrations, config.RegistrationDelayMinMs, config.RegistrationDelayMaxMs
}

// RegisterCspNativeResourcesAll registers all CSP-native resources into CB-TB
// using hierarchical rate limiting: global cap → per-CSP cap → per-connection processing.
// Results are collected via a channel to avoid data races.
func RegisterCspNativeResourcesAll(ctx context.Context, nsId string, infraNamePrefix string, option string, infraFlag string) (model.RegisterResourceAllResult, error) {
	startTime := time.Now()

	if _, err := getValidatedOptionMap(option); err != nil {
		log.Error().Err(err).Msg("Invalid registration options")
		return model.RegisterResourceAllResult{}, err
	}

	connectionConfigList, err := common.GetConnConfigList(model.DefaultCredentialHolder, true, true)
	if err != nil {
		err := fmt.Errorf("Cannot load ConnectionConfigList")
		log.Error().Err(err).Msg("")
		return model.RegisterResourceAllResult{}, err
	}

	totalConnections := len(connectionConfigList.Connectionconfig)
	if totalConnections == 0 {
		return model.RegisterResourceAllResult{}, nil
	}

	// Step 1: Group connections by CSP (ProviderName)
	cspGroups := make(map[string][]model.ConnConfig) // providerName -> []ConnConfig
	for _, k := range connectionConfigList.Connectionconfig {
		provider := strings.ToLower(k.ProviderName)
		cspGroups[provider] = append(cspGroups[provider], k)
	}

	log.Info().Msgf("RegisterCspNativeResourcesAll: %d connections grouped into %d CSPs (global concurrency limit: %d)",
		totalConnections, len(cspGroups), csp.GlobalMaxConcurrentConnections)
	for provider, conns := range cspGroups {
		maxConns, _, _ := getRegisterRateLimitsForCSP(provider)
		log.Info().Msgf("  CSP %s: %d connections (max concurrent: %d)", provider, len(conns), maxConns)
	}

	// Step 2: Create channel to collect results safely (Fix #3: avoid data race on append)
	resultChan := make(chan model.RegisterResourceResult, totalConnections)

	// Step 3: Global semaphore to cap total concurrent goroutines
	globalSemaphore := make(chan struct{}, csp.GlobalMaxConcurrentConnections)

	// Step 4: Process CSPs in parallel, each with its own per-CSP semaphore
	var cspWg sync.WaitGroup
	for provider, connections := range cspGroups {
		cspWg.Add(1)
		go func(providerName string, connConfigs []model.ConnConfig) {
			defer cspWg.Done()

			maxConns, delayMinMs, delayMaxMs := getRegisterRateLimitsForCSP(providerName)
			cspSemaphore := make(chan struct{}, maxConns)

			log.Info().Msgf("Starting resource registration for CSP %s: %d connections (max concurrent: %d, delay: %d-%dms)",
				providerName, len(connConfigs), maxConns, delayMinMs, delayMaxMs)

			var connWg sync.WaitGroup
			for _, connConfig := range connConfigs {
				connWg.Add(1)
				go func(k model.ConnConfig) {
					defer connWg.Done()

					// Acquire global semaphore (total concurrency cap)
					globalSemaphore <- struct{}{}
					defer func() { <-globalSemaphore }()

					// Acquire per-CSP semaphore (CSP-specific concurrency cap)
					cspSemaphore <- struct{}{}
					defer func() { <-cspSemaphore }()

					// Stagger start with CSP-specific delay to avoid API rate limit bursts
					common.RandomSleep(delayMinMs, delayMaxMs)

					infraNameForRegister := infraNamePrefix + "-" + k.ConfigName

					log.Debug().Msgf("Registering resources for connection %s (CSP: %s)", k.ConfigName, providerName)
					registerResult, err := RegisterCspNativeResources(ctx, nsId, k.ConfigName, infraNameForRegister, option, infraFlag)
					if err != nil {
						log.Error().Err(err).Msgf("Failed to register resources for connection %s", k.ConfigName)
					}
					log.Debug().Msgf("Completed registration for connection %s (CSP: %s, elapsed: %ds)",
						k.ConfigName, providerName, registerResult.ElapsedTime)

					// Send result to channel (no race condition — channel is safe for concurrent sends)
					resultChan <- registerResult
				}(connConfig)
			}
			connWg.Wait()

			log.Info().Msgf("Completed resource registration for CSP %s (%d connections)", providerName, len(connConfigs))
		}(provider, connections)
	}

	// Step 5: Close channel after all goroutines complete
	go func() {
		cspWg.Wait()
		close(resultChan)
	}()

	// Step 6: Collect results from channel
	output := model.RegisterResourceAllResult{}
	for result := range resultChan {
		output.RegisterationResult = append(output.RegisterationResult, result)
	}

	// Step 7: Aggregate overview counts
	errorConnectionCnt := 0
	for _, k := range output.RegisterationResult {
		output.RegisterationOverview.VNet += k.RegisterationOverview.VNet
		output.RegisterationOverview.SecurityGroup += k.RegisterationOverview.SecurityGroup
		output.RegisterationOverview.SshKey += k.RegisterationOverview.SshKey
		output.RegisterationOverview.DataDisk += k.RegisterationOverview.DataDisk
		output.RegisterationOverview.CustomImage += k.RegisterationOverview.CustomImage
		output.RegisterationOverview.Node += k.RegisterationOverview.Node
		output.RegisterationOverview.NLB += k.RegisterationOverview.NLB
		output.RegisterationOverview.Failed += k.RegisterationOverview.Failed

		if k.SystemMessage != "" {
			errorConnectionCnt++
		}
	}

	output.ElapsedTime = int(math.Round(time.Since(startTime).Seconds()))
	output.RegisteredConnection = totalConnections
	output.AvailableConnection = totalConnections - errorConnectionCnt

	sort.SliceStable(output.RegisterationResult, func(i, j int) bool {
		return output.RegisterationResult[i].ConnectionName < output.RegisterationResult[j].ConnectionName
	})

	log.Info().Msgf("RegisterCspNativeResourcesAll completed: %d connections, %d errors, %ds elapsed",
		totalConnections, errorConnectionCnt, output.ElapsedTime)

	return output, err
}

// RegisterCspNativeResources registers specified CSP native resources from a target connection.
func RegisterCspNativeResources(ctx context.Context, nsId string, connConfig string, infraNamePrefix string, option string, infraFlag string) (model.RegisterResourceResult, error) {
	startTime := time.Now()
	optionFlag := "register"
	result := model.RegisterResourceResult{}

	// 1. Option Parsing & Validation
	doMap, err := getValidatedOptionMap(option)
	if err != nil {
		log.Error().Err(err).Msgf("Invalid registration options for connection: %s", connConfig)
		return result, err
	}

	// 2. Execution (Best Effort)
	genName := func(cspId string) string {
		return common.ChangeIdString(fmt.Sprintf("%s-%s", connConfig, cspId))
	}

	// [1] CustomImage
	if doMap[model.StrCustomImage] {
		if res, err := InspectResources(connConfig, model.StrCustomImage); err != nil {
			result.SystemMessage += "// CustomImage Inspect Failed: " + err.Error()
		} else {
			for _, r := range res.Resources.OnCspOnly.Info {
				req := model.CustomImageReq{
					ConnectionName: connConfig, CspResourceId: r.CspResourceId, Name: genName(r.CspResourceId),
				}
				_, err = resource.RegisterCustomImageWithId(nsId, &req)
				appendResult(&result, model.StrCustomImage, req.Name, err, &result.RegisterationOverview.CustomImage)
			}
		}
	}

	// [2] VNet
	if doMap[model.StrVNet] {
		if res, err := InspectResources(connConfig, model.StrVNet); err != nil {
			result.SystemMessage += "// VNet Inspect Failed: " + err.Error()
		} else {
			for _, r := range res.Resources.OnCspOnly.Info {
				req := model.RegisterVNetReq{
					ConnectionName: connConfig, CspResourceId: r.CspResourceId, Name: genName(r.CspResourceId),
					Description: "Ref name: " + r.RefNameOrId + ". CSP managed VNet (registered to CB-TB)",
				}
				_, err = resource.RegisterVNet(ctx, nsId, &req)
				appendResult(&result, model.StrVNet, req.Name, err, &result.RegisterationOverview.VNet)
			}
		}
	}

	// [3] SecurityGroup
	if doMap[model.StrSecurityGroup] {
		if res, err := InspectResources(connConfig, model.StrSecurityGroup); err != nil {
			result.SystemMessage += "// SG Inspect Failed: " + err.Error()
		} else {
			for _, r := range res.Resources.OnCspOnly.Info {
				req := model.SecurityGroupReq{
					ConnectionName: connConfig, CspResourceId: r.CspResourceId, Name: genName(r.CspResourceId),
					VNetId: "unknown", Description: "Ref name: " + r.RefNameOrId + ". CSP managed Security Group (registered to CB-TB)",
				}
				_, err = resource.CreateSecurityGroup(ctx, nsId, &req, optionFlag)
				appendResult(&result, model.StrSecurityGroup, req.Name, err, &result.RegisterationOverview.SecurityGroup)
			}
		}
	}

	// [4] SSHKey
	if doMap[model.StrSSHKey] {
		if res, err := InspectResources(connConfig, model.StrSSHKey); err != nil {
			result.SystemMessage += "// SSHKey Inspect Failed: " + err.Error()
		} else {
			for _, r := range res.Resources.OnCspOnly.Info {
				req := model.SshKeyReq{
					ConnectionName: connConfig, CspResourceId: r.CspResourceId, Name: genName(r.CspResourceId),
					Username: "unknown", Fingerprint: "unknown", PublicKey: "unknown", PrivateKey: "unknown",
					Description: "Ref name: " + r.RefNameOrId + ". CSP managed SSH Key (registered to CB-TB)",
				}
				_, err = resource.CreateSshKey(ctx, nsId, &req, optionFlag)
				appendResult(&result, model.StrSSHKey, req.Name, err, &result.RegisterationOverview.SshKey)
			}
		}
	}

	// [5] Node (VM)
	if doMap[model.StrNode] {
		if res, err := InspectResources(connConfig, model.StrNode); err != nil {
			result.SystemMessage += "// Node Inspect Failed: " + err.Error()
		} else {
			// Determine Infra creation strategy based on infraFlag
			useSingleInfra := strings.ToLower(infraFlag) == "y"
			var singleInfraName string
			var singleInfraCreated bool

			// Track registered Infras for status sync
			registeredInfras := make(map[string]bool)

			// Track network-based nodegroups: key = "vnetId_subnetId", value = nodegroup name
			networkNodeGroupMap := make(map[string]string)
			// Track nodegroup Node counts for naming: key = nodegroup name, value = Node count
			nodegroupNodeCount := make(map[string]int)

			if useSingleInfra {
				// Single Infra mode: all Nodes go into one Infra
				singleInfraName = common.ChangeIdString(infraNamePrefix)
			}

			// Phase 1: Register all Nodes and collect network info
			// We'll use temporary nodegroup names first, then reorganize
			type registeredNodeInfo struct {
				nodeId       string
				vnetId     string
				subnetId   string
				networkKey string
			}
			var registeredNodes []registeredNodeInfo

			for idx, r := range res.Resources.OnCspOnly.Info {
				// Generate a temporary unique nodegroup name for initial registration
				tempNodeGroupName := common.ChangeIdString(fmt.Sprintf("reg-%s-%d", connConfig, idx))

				var infraName string
				if useSingleInfra {
					// Use the same Infra name for all Nodes
					infraName = singleInfraName
				} else {
					// Create separate Infra for each Node (use shorter name)
					infraName = common.ChangeIdString(fmt.Sprintf("%s-%s", infraNamePrefix, r.RefNameOrId))
				}

				var nodeId string
				if useSingleInfra && singleInfraCreated {
					// Add Node to existing Infra
					nodeGroupReq := &model.CreateNodeGroupReq{
						ConnectionName: connConfig, CspResourceId: r.CspResourceId, Name: tempNodeGroupName,
						Description: "Ref name: " + r.RefNameOrId + ". CSP managed Node (registered to CB-TB)",
						Label:       map[string]string{model.LabelRegistered: "true"},
						// Placeholders
						ImageId: "unknown", SpecId: "unknown", SshKeyId: "unknown",
						SubnetId: "unknown", VNetId: "unknown", SecurityGroupIds: []string{"unknown"},
					}
					infraInfo, err := CreateInfraGroupNode(ctx, nsId, infraName, nodeGroupReq, true)
					appendResult(&result, model.StrNode, tempNodeGroupName, err, &result.RegisterationOverview.Node)
					if err == nil {
						registeredInfras[infraName] = true
						// Get the Node ID from the newly added Node
						if infraInfo != nil && len(infraInfo.NewNodeList) > 0 {
							nodeId = infraInfo.NewNodeList[0]
						}
					}
				} else {
					// Create new Infra (either first Node in single Infra mode, or each Node in separate Infra mode)
					req := model.InfraReq{
						Name: infraName, Description: "Infra for CSP managed Nodes", InstallMonAgent: "no",
						NodeGroups: []model.CreateNodeGroupReq{{
							ConnectionName: connConfig, CspResourceId: r.CspResourceId, Name: tempNodeGroupName,
							Description: "Ref name: " + r.RefNameOrId + ". CSP managed Node (registered to CB-TB)",
							Label:       map[string]string{model.LabelRegistered: "true"},
							// Placeholders
							ImageId: "unknown", SpecId: "unknown", SshKeyId: "unknown",
							SubnetId: "unknown", VNetId: "unknown", SecurityGroupIds: []string{"unknown"},
						}},
					}
					infraInfo, err := CreateInfra(ctx, nsId, &req, optionFlag, false)
					appendResult(&result, model.StrNode, tempNodeGroupName, err, &result.RegisterationOverview.Node)

					if err == nil {
						registeredInfras[infraName] = true
						if useSingleInfra {
							singleInfraCreated = true
						}
						// Get the Node ID from the newly created Node
						if infraInfo != nil && len(infraInfo.NewNodeList) > 0 {
							nodeId = infraInfo.NewNodeList[0]
						} else if infraInfo != nil && len(infraInfo.Node) > 0 {
							nodeId = infraInfo.Node[0].Id
						}
					}
				}

				// If Node was registered successfully, collect its network info
				if nodeId != "" && useSingleInfra {
					nodeInfo, err := GetNodeObject(nsId, singleInfraName, nodeId)
					if err == nil && nodeInfo.VNetId != "" {
						networkKey := fmt.Sprintf("%s_%s", nodeInfo.VNetId, nodeInfo.SubnetId)
						registeredNodes = append(registeredNodes, registeredNodeInfo{
							nodeId:       nodeId,
							vnetId:     nodeInfo.VNetId,
							subnetId:   nodeInfo.SubnetId,
							networkKey: networkKey,
						})
					}
				}
			}

			// Phase 2: Reorganize nodegroups by network configuration (only for single Infra mode)
			if useSingleInfra && len(registeredNodes) > 1 {
				log.Info().Msgf("Reorganizing %d VMs into network-based nodegroups in Infra %s", len(registeredNodes), singleInfraName)

				// Group VMs by network configuration
				networkGroups := make(map[string][]string) // networkKey -> []nodeId
				for _, node := range registeredNodes {
					networkGroups[node.networkKey] = append(networkGroups[node.networkKey], node.nodeId)
				}

				// Generate meaningful nodegroup names and update VMs
				nodegroupIndex := 1
				for networkKey, nodeIds := range networkGroups {
					var newNodeGroupName string
					if len(networkGroups) == 1 {
						// All VMs in same network - use simple name
						newNodeGroupName = fmt.Sprintf("reg-group")
					} else {
						// Multiple networks - use indexed name
						newNodeGroupName = fmt.Sprintf("reg-group%d", nodegroupIndex)
						nodegroupIndex++
					}

					// Track for logging
					networkNodeGroupMap[networkKey] = newNodeGroupName
					nodegroupNodeCount[newNodeGroupName] = len(nodeIds)

					// Update each VM's NodeGroupId
					for _, nodeId := range nodeIds {
						nodeInfo, err := GetNodeObject(nsId, singleInfraName, nodeId)
						if err != nil {
							log.Warn().Err(err).Msgf("Failed to get VM %s for nodegroup update", nodeId)
							continue
						}

						oldNodeGroupId := nodeInfo.NodeGroupId
						nodeInfo.NodeGroupId = newNodeGroupName
						UpdateNodeInfo(nsId, singleInfraName, nodeInfo)

						// Delete old nodegroup if it was temporary (starts with "reg-")
						if oldNodeGroupId != "" && strings.HasPrefix(oldNodeGroupId, "reg-") && oldNodeGroupId != newNodeGroupName {
							oldNodeGroupKey := common.GenInfraNodeGroupKey(nsId, singleInfraName, oldNodeGroupId)
							kvstore.Delete(oldNodeGroupKey)
						}
					}

					// Create or update the new nodegroup
					nodegroupKey := common.GenInfraNodeGroupKey(nsId, singleInfraName, newNodeGroupName)
					nodegroupInfo := model.NodeGroupInfo{
						ResourceType:  model.StrNodeGroup,
						Id:            newNodeGroupName,
						Name:          newNodeGroupName,
						Uid:           common.GenUid(),
						NodeGroupSize: len(nodeIds),
						NodeId:        nodeIds,
					}
					nodegroupVal, _ := json.Marshal(nodegroupInfo)
					kvstore.Put(nodegroupKey, string(nodegroupVal))

					log.Info().Msgf("Created nodegroup '%s' with %d VMs (network: %s)", newNodeGroupName, len(nodeIds), networkKey)
				}
			}

			// Sync Infra status after all VM registrations and reorganization
			for infraName := range registeredInfras {
				if _, err := GetInfraStatus(nsId, infraName); err != nil {
					log.Warn().Err(err).Msgf("Failed to sync Infra status for %s after registration", infraName)
				} else {
					log.Info().Msgf("Infra %s status synced after VM registration", infraName)
				}
			}
		}
	}

	// [6] DataDisk
	if doMap[model.StrDataDisk] {
		if res, err := InspectResources(connConfig, model.StrDataDisk); err != nil {
			result.SystemMessage += "// DataDisk Inspect Failed: " + err.Error()
		} else {
			for _, r := range res.Resources.OnCspOnly.Info {
				req := model.DataDiskReq{
					ConnectionName: connConfig, CspResourceId: r.CspResourceId, Name: genName(r.CspResourceId),
					Description: "Ref name: " + r.RefNameOrId + ". CSP managed Data Disk (registered to CB-TB)",
				}
				_, err = resource.CreateDataDisk(ctx, nsId, &req, optionFlag)
				appendResult(&result, model.StrDataDisk, req.Name, err, &result.RegisterationOverview.DataDisk)
			}
		}
	}

	result.ConnectionName = connConfig
	result.ElapsedTime = int(math.Round(time.Since(startTime).Seconds()))
	fmt.Printf("\n\n%s [Elapsed]Total %d \n\n", connConfig, result.ElapsedTime)

	return result, nil
}

// Parse, Set Defaults, and Validate Options
func getValidatedOptionMap(option string) (map[string]bool, error) {
	doMap := make(map[string]bool)

	if len(option) == 0 {
		allResources := []string{
			model.StrCustomImage,
			model.StrVNet,
			model.StrSecurityGroup,
			model.StrSSHKey,
			model.StrNode,
			model.StrDataDisk,
		}
		for _, op := range allResources {
			doMap[op] = true
		}
	} else {
		reqOptions := strings.Split(strings.ReplaceAll(option, " ", ""), ",")
		for _, op := range reqOptions {
			if op == "" {
				continue
			}
			doMap[op] = true
		}
	}

	if err := validateReqOptions(doMap); err != nil {
		return nil, err
	}

	return doMap, nil
}

func validateReqOptions(doMap map[string]bool) error {
	var valErrs []string

	allowed := map[string]bool{
		model.StrCustomImage:   true,
		model.StrVNet:          true,
		model.StrSecurityGroup: true,
		model.StrSSHKey:        true,
		model.StrNode:            true,
		model.StrDataDisk:      true,
	}

	for op := range doMap {
		if !allowed[op] {
			valErrs = append(valErrs, "unsupported option: '"+op+"'")
		}
	}

	requiredDeps := map[string][]string{
		model.StrDataDisk:      {model.StrNode},
		model.StrNode:            {model.StrSecurityGroup, model.StrSSHKey},
		model.StrSecurityGroup: {model.StrVNet},
	}

	for key, required := range requiredDeps {
		if !doMap[key] {
			continue
		}
		for _, dep := range required {
			if !doMap[dep] {
				valErrs = append(valErrs, fmt.Sprintf("'%s' requires '%s'", key, dep))
			}
		}
	}

	if len(valErrs) > 0 {
		return fmt.Errorf("Validation Failed: %s", strings.Join(valErrs, ", "))
	}
	return nil
}

// updates the result counters and logs the outcome.
func appendResult(result *model.RegisterResourceResult, resType, name string, err error, successCounter *int) {
	status := ""
	if err != nil {
		log.Error().Err(err).Msgf("Failed to register %s: %s", resType, name)
		status = " [Failed] " + err.Error()
		result.RegisterationOverview.Failed++
	} else {
		if successCounter != nil {
			*successCounter++
		}
	}
	result.RegisterationOutputs.IdList = append(result.RegisterationOutputs.IdList, fmt.Sprintf("%s: %s%s", resType, name, status))
}

func FindTbNodeByCspId(nsId string, infraId string, nodeCspResourceId string) (model.NodeInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.NodeInfo{}, err
	}

	err = common.CheckString(infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.NodeInfo{}, err
	}

	err = common.CheckString(nodeCspResourceId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.NodeInfo{}, err
	}

	check, err := CheckInfra(nsId, infraId)

	if !check {
		err := fmt.Errorf("The Infra " + infraId + " does not exist.")
		return model.NodeInfo{}, err
	}

	if err != nil {
		err := fmt.Errorf("Failed to check the existence of the Infra " + infraId + ".")
		return model.NodeInfo{}, err
	}

	infra, _, err := GetInfraObject(nsId, infraId)
	if err != nil {
		err := fmt.Errorf("Failed to get the Infra " + infraId + ".")
		return model.NodeInfo{}, err
	}

	nodes := infra.Node
	for _, v := range nodes {
		if v.CspResourceId == nodeCspResourceId || v.CspResourceName == nodeCspResourceId {
			return v, nil
		}
	}

	err = fmt.Errorf("Cannot find the VM %s in %s/%s", nodeCspResourceId, nsId, infraId)
	return model.NodeInfo{}, err
}
