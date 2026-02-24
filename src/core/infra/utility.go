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

	validate.RegisterStructValidation(MciReqStructLevelValidation, model.MciReq{})
	validate.RegisterStructValidation(CreateSubGroupReqStructLevelValidation, model.CreateSubGroupReq{})
	validate.RegisterStructValidation(TbMciCmdReqStructLevelValidation, model.MciCmdReq{})
	// validate.RegisterStructValidation(TbMciRecommendReqStructLevelValidation, MciRecommendReq{})
	// validate.RegisterStructValidation(VmRecommendReqStructLevelValidation, VmRecommendReq{})
	// validate.RegisterStructValidation(TbBenchmarkReqStructLevelValidation, BenchmarkReq{})
	// validate.RegisterStructValidation(TbMultihostBenchmarkReqStructLevelValidation, MultihostBenchmarkReq{})

	validate.RegisterStructValidation(DFMonAgentInstallReqStructLevelValidation, model.MonAgentInstallReq{})

}

// CheckMci func is to check given mciId is duplicated with existing
func CheckMci(nsId string, mciId string) (bool, error) {

	// Check parameters' emptiness
	if nsId == "" {
		err := fmt.Errorf("CheckMci failed; nsId given is empty.")
		return false, err
	} else if mciId == "" {
		err := fmt.Errorf("CheckMci failed; mciId given is empty.")
		return false, err
	}

	key := common.GenMciKey(nsId, mciId, "")

	_, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("In CheckMci(); kvstore.GetKv() returned an error.")
		log.Error().Err(err).Msg("")
		// return nil, err
	}

	if exists {
		return true, nil
	}
	return false, nil

}

// CheckSubGroup func is to check given subGroupId is duplicated with existing
func CheckSubGroup(nsId string, mciId string, subGroupId string) (bool, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	subGroupList, err := ListSubGroupId(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}
	for _, v := range subGroupList {
		if strings.EqualFold(v, subGroupId) {
			return true, nil
		}
	}
	return false, nil
}

func CheckVm(nsId string, mciId string, vmId string) (bool, error) {

	// Check parameters' emptiness
	if nsId == "" {
		err := fmt.Errorf("CheckVm failed; nsId given is null.")
		return false, err
	} else if mciId == "" {
		err := fmt.Errorf("CheckVm failed; mciId given is null.")
		return false, err
	} else if vmId == "" {
		err := fmt.Errorf("CheckVm failed; vmId given is null.")
		return false, err
	}

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}
	err = common.CheckString(vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}
	//log.Debug().Msg("[Check vm] " + mciId + ", " + vmId)

	key := common.GenMciKey(nsId, mciId, vmId)

	_, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("In CheckVm(); kvstore.GetKv() returned an error.")
		log.Error().Err(err).Msg("")
		// return nil, err
	}

	if exists {
		return true, nil
	}
	return false, nil

}

func CheckMciPolicy(nsId string, mciId string) (bool, error) {

	// Check parameters' emptiness
	if nsId == "" {
		err := fmt.Errorf("CheckMci failed; nsId given is null.")
		return false, err
	} else if mciId == "" {
		err := fmt.Errorf("CheckMci failed; mciId given is null.")
		return false, err
	}

	// err := common.CheckString(nsId)
	// if err != nil {
	// 	log.Error().Err(err).Msg("")
	// 	return false, err
	// }

	// err = common.CheckString(mciId)
	// if err != nil {
	// 	log.Error().Err(err).Msg("")
	// 	return false, err
	// }
	log.Debug().Msg("[Check MciPolicy] " + mciId)

	key := common.GenMciPolicyKey(nsId, mciId, "")

	_, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("In CheckMciPolicy(); kvstore.GetKv() returned an error.")
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
			mciListinNs, _ := ListMciId(ns)
			if mciListinNs == nil {
				continue
			}
			for _, mci := range mciListinNs {
				nlbListInMci, err := ListNLBId(ns, mci)
				if err != nil {
					log.Error().Err(err).Msg("")
					err := fmt.Errorf("an error occurred while getting resource list")
					return nullObj, err
				}
				if nlbListInMci == nil {
					continue
				}

				for _, nlbId := range nlbListInMci {
					nlb, err := GetNLB(ns, mci, nlbId)
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
						temp.MciId = mci
						temp.ObjectKey = GenNLBKey(ns, mci, nlb.Id)

						TbResourceList.Info = append(TbResourceList.Info, temp)
					}
				}
			}
		case model.StrVM:
			mciListinNs, _ := ListMciId(ns)
			if mciListinNs == nil {
				continue
			}
			for _, mci := range mciListinNs {
				vmListInMci, err := ListVmId(ns, mci)
				if err != nil {
					log.Error().Err(err).Msg("")
					err := fmt.Errorf("an error occurred while getting resource list")
					return nullObj, err
				}
				if vmListInMci == nil {
					continue
				}

				for _, vmId := range vmListInMci {
					vm, err := GetVmObject(ns, mci, vmId)
					if err != nil {
						log.Error().Err(err).Msg("")
						err := fmt.Errorf("an error occurred while getting resource list")
						return nullObj, err
					}

					if vm.ConnectionName == connConfig { // filtering
						temp := model.ResourceOnTumblebugInfo{}
						temp.IdByTb = vm.Id
						temp.CspResourceId = vm.CspResourceId
						temp.NsId = ns
						temp.MciId = mci
						temp.ObjectKey = common.GenMciKey(ns, mci, vm.Id)

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
		if providerName == csp.Azure {
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

			inspectResult, err = InspectResources(k.ConfigName, model.StrVM)
			if err != nil {
				log.Error().Err(err).Msg("")
				temp.SystemMessage += err.Error()
			}
			temp.TumblebugOverview.Vm = inspectResult.ResourceOverview.OnTumblebug
			temp.CspOnlyOverview.Vm = inspectResult.ResourceOverview.OnCspOnly

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
		output.TumblebugOverview.Vm += k.TumblebugOverview.Vm
		output.TumblebugOverview.NLB += k.TumblebugOverview.NLB

		output.CspOnlyOverview.VNet += k.CspOnlyOverview.VNet
		output.CspOnlyOverview.SecurityGroup += k.CspOnlyOverview.SecurityGroup
		output.CspOnlyOverview.SshKey += k.CspOnlyOverview.SshKey
		output.CspOnlyOverview.DataDisk += k.CspOnlyOverview.DataDisk
		output.CspOnlyOverview.CustomImage += k.CspOnlyOverview.CustomImage
		output.CspOnlyOverview.Vm += k.CspOnlyOverview.Vm
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

// getRegisterRateLimitsForCSP returns rate limiting config for resource registration.
// Uses centralized CSP config from csp.GetRateLimitConfig() with built-in fallback for unknown CSPs.
func getRegisterRateLimitsForCSP(providerName string) (maxConns int, delayMinMs int, delayMaxMs int) {
	config := csp.GetRateLimitConfig(providerName)
	return config.MaxConcurrentRegistrations, config.RegistrationDelayMinMs, config.RegistrationDelayMaxMs
}

// RegisterCspNativeResourcesAll registers all CSP-native resources into CB-TB
// using hierarchical rate limiting: global cap → per-CSP cap → per-connection processing.
// Results are collected via a channel to avoid data races.
func RegisterCspNativeResourcesAll(nsId string, mciNamePrefix string, option string, mciFlag string) (model.RegisterResourceAllResult, error) {
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

					mciNameForRegister := mciNamePrefix + "-" + k.ConfigName

					log.Debug().Msgf("Registering resources for connection %s (CSP: %s)", k.ConfigName, providerName)
					registerResult, err := RegisterCspNativeResources(nsId, k.ConfigName, mciNameForRegister, option, mciFlag)
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
		output.RegisterationOverview.Vm += k.RegisterationOverview.Vm
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
func RegisterCspNativeResources(nsId string, connConfig string, mciNamePrefix string, option string, mciFlag string) (model.RegisterResourceResult, error) {
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
				_, err = resource.RegisterVNet(nsId, &req)
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
				_, err = resource.CreateSecurityGroup(nsId, &req, optionFlag)
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
				_, err = resource.CreateSshKey(nsId, &req, optionFlag)
				appendResult(&result, model.StrSSHKey, req.Name, err, &result.RegisterationOverview.SshKey)
			}
		}
	}

	// [5] VM
	if doMap[model.StrVM] {
		if res, err := InspectResources(connConfig, model.StrVM); err != nil {
			result.SystemMessage += "// VM Inspect Failed: " + err.Error()
		} else {
			// Determine MCI creation strategy based on mciFlag
			useSingleMci := strings.ToLower(mciFlag) == "y"
			var singleMciName string
			var singleMciCreated bool

			// Track registered MCIs for status sync
			registeredMcis := make(map[string]bool)

			// Track network-based subgroups: key = "vnetId_subnetId", value = subgroup name
			networkSubgroupMap := make(map[string]string)
			// Track subgroup VM counts for naming: key = subgroup name, value = VM count
			subgroupVmCount := make(map[string]int)

			if useSingleMci {
				// Single MCI mode: all VMs go into one MCI
				singleMciName = common.ChangeIdString(mciNamePrefix)
			}

			// Phase 1: Register all VMs and collect network info
			// We'll use temporary subgroup names first, then reorganize
			type registeredVmInfo struct {
				vmId       string
				vnetId     string
				subnetId   string
				networkKey string
			}
			var registeredVms []registeredVmInfo

			for idx, r := range res.Resources.OnCspOnly.Info {
				// Generate a temporary unique subgroup name for initial registration
				tempSubGroupName := common.ChangeIdString(fmt.Sprintf("reg-%s-%d", connConfig, idx))

				var mciName string
				if useSingleMci {
					// Use the same MCI name for all VMs
					mciName = singleMciName
				} else {
					// Create separate MCI for each VM (use shorter name)
					mciName = common.ChangeIdString(fmt.Sprintf("%s-%s", mciNamePrefix, r.RefNameOrId))
				}

				var vmId string
				if useSingleMci && singleMciCreated {
					// Add VM to existing MCI
					subGroupReq := &model.CreateSubGroupReq{
						ConnectionName: connConfig, CspResourceId: r.CspResourceId, Name: tempSubGroupName,
						Description: "Ref name: " + r.RefNameOrId + ". CSP managed VM (registered to CB-TB)",
						Label:       map[string]string{model.LabelRegistered: "true"},
						// Placeholders
						ImageId: "unknown", SpecId: "unknown", SshKeyId: "unknown",
						SubnetId: "unknown", VNetId: "unknown", SecurityGroupIds: []string{"unknown"},
					}
					mciInfo, err := CreateMciGroupVm(nsId, mciName, subGroupReq, true)
					appendResult(&result, model.StrVM, tempSubGroupName, err, &result.RegisterationOverview.Vm)
					if err == nil {
						registeredMcis[mciName] = true
						// Get the VM ID from the newly added VM
						if mciInfo != nil && len(mciInfo.NewVmList) > 0 {
							vmId = mciInfo.NewVmList[0]
						}
					}
				} else {
					// Create new MCI (either first VM in single MCI mode, or each VM in separate MCI mode)
					req := model.MciReq{
						Name: mciName, Description: "MCI for CSP managed VMs", InstallMonAgent: "no",
						SubGroups: []model.CreateSubGroupReq{{
							ConnectionName: connConfig, CspResourceId: r.CspResourceId, Name: tempSubGroupName,
							Description: "Ref name: " + r.RefNameOrId + ". CSP managed VM (registered to CB-TB)",
							Label:       map[string]string{model.LabelRegistered: "true"},
							// Placeholders
							ImageId: "unknown", SpecId: "unknown", SshKeyId: "unknown",
							SubnetId: "unknown", VNetId: "unknown", SecurityGroupIds: []string{"unknown"},
						}},
					}
					mciInfo, err := CreateMci(nsId, &req, optionFlag, false)
					appendResult(&result, model.StrVM, tempSubGroupName, err, &result.RegisterationOverview.Vm)

					if err == nil {
						registeredMcis[mciName] = true
						if useSingleMci {
							singleMciCreated = true
						}
						// Get the VM ID from the newly created VM
						if mciInfo != nil && len(mciInfo.NewVmList) > 0 {
							vmId = mciInfo.NewVmList[0]
						} else if mciInfo != nil && len(mciInfo.Vm) > 0 {
							vmId = mciInfo.Vm[0].Id
						}
					}
				}

				// If VM was registered successfully, collect its network info
				if vmId != "" && useSingleMci {
					vmInfo, err := GetVmObject(nsId, singleMciName, vmId)
					if err == nil && vmInfo.VNetId != "" {
						networkKey := fmt.Sprintf("%s_%s", vmInfo.VNetId, vmInfo.SubnetId)
						registeredVms = append(registeredVms, registeredVmInfo{
							vmId:       vmId,
							vnetId:     vmInfo.VNetId,
							subnetId:   vmInfo.SubnetId,
							networkKey: networkKey,
						})
					}
				}
			}

			// Phase 2: Reorganize subgroups by network configuration (only for single MCI mode)
			if useSingleMci && len(registeredVms) > 1 {
				log.Info().Msgf("Reorganizing %d VMs into network-based subgroups in MCI %s", len(registeredVms), singleMciName)

				// Group VMs by network configuration
				networkGroups := make(map[string][]string) // networkKey -> []vmId
				for _, vm := range registeredVms {
					networkGroups[vm.networkKey] = append(networkGroups[vm.networkKey], vm.vmId)
				}

				// Generate meaningful subgroup names and update VMs
				subgroupIndex := 1
				for networkKey, vmIds := range networkGroups {
					var newSubgroupName string
					if len(networkGroups) == 1 {
						// All VMs in same network - use simple name
						newSubgroupName = fmt.Sprintf("reg-group")
					} else {
						// Multiple networks - use indexed name
						newSubgroupName = fmt.Sprintf("reg-group%d", subgroupIndex)
						subgroupIndex++
					}

					// Track for logging
					networkSubgroupMap[networkKey] = newSubgroupName
					subgroupVmCount[newSubgroupName] = len(vmIds)

					// Update each VM's SubGroupId
					for _, vmId := range vmIds {
						vmInfo, err := GetVmObject(nsId, singleMciName, vmId)
						if err != nil {
							log.Warn().Err(err).Msgf("Failed to get VM %s for subgroup update", vmId)
							continue
						}

						oldSubgroupId := vmInfo.SubGroupId
						vmInfo.SubGroupId = newSubgroupName
						UpdateVmInfo(nsId, singleMciName, vmInfo)

						// Delete old subgroup if it was temporary (starts with "reg-")
						if oldSubgroupId != "" && strings.HasPrefix(oldSubgroupId, "reg-") && oldSubgroupId != newSubgroupName {
							oldSubgroupKey := common.GenMciSubGroupKey(nsId, singleMciName, oldSubgroupId)
							kvstore.Delete(oldSubgroupKey)
						}
					}

					// Create or update the new subgroup
					subgroupKey := common.GenMciSubGroupKey(nsId, singleMciName, newSubgroupName)
					subgroupInfo := model.SubGroupInfo{
						ResourceType: model.StrSubGroup,
						Id:           newSubgroupName,
						Name:         newSubgroupName,
						Uid:          common.GenUid(),
						SubGroupSize: len(vmIds),
						VmId:         vmIds,
					}
					subgroupVal, _ := json.Marshal(subgroupInfo)
					kvstore.Put(subgroupKey, string(subgroupVal))

					log.Info().Msgf("Created subgroup '%s' with %d VMs (network: %s)", newSubgroupName, len(vmIds), networkKey)
				}
			}

			// Sync MCI status after all VM registrations and reorganization
			for mciName := range registeredMcis {
				if _, err := GetMciStatus(nsId, mciName); err != nil {
					log.Warn().Err(err).Msgf("Failed to sync MCI status for %s after registration", mciName)
				} else {
					log.Info().Msgf("MCI %s status synced after VM registration", mciName)
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
				_, err = resource.CreateDataDisk(nsId, &req, optionFlag)
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
			model.StrVM,
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
		model.StrVM:            true,
		model.StrDataDisk:      true,
	}

	for op := range doMap {
		if !allowed[op] {
			valErrs = append(valErrs, "unsupported option: '"+op+"'")
		}
	}

	requiredDeps := map[string][]string{
		model.StrDataDisk:      {model.StrVM},
		model.StrVM:            {model.StrSecurityGroup, model.StrSSHKey},
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

func FindTbVmByCspId(nsId string, mciId string, vmCspResourceId string) (model.VmInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.VmInfo{}, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.VmInfo{}, err
	}

	err = common.CheckString(vmCspResourceId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.VmInfo{}, err
	}

	check, err := CheckMci(nsId, mciId)

	if !check {
		err := fmt.Errorf("The MCI " + mciId + " does not exist.")
		return model.VmInfo{}, err
	}

	if err != nil {
		err := fmt.Errorf("Failed to check the existence of the MCI " + mciId + ".")
		return model.VmInfo{}, err
	}

	mci, _, err := GetMciObject(nsId, mciId)
	if err != nil {
		err := fmt.Errorf("Failed to get the MCI " + mciId + ".")
		return model.VmInfo{}, err
	}

	vms := mci.Vm
	for _, v := range vms {
		if v.CspResourceId == vmCspResourceId || v.CspResourceName == vmCspResourceId {
			return v, nil
		}
	}

	err = fmt.Errorf("Cannot find the VM %s in %s/%s", vmCspResourceId, nsId, mciId)
	return model.VmInfo{}, err
}
