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
	//"encoding/json"
	//uuid "github.com/google/uuid"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/rs/zerolog/log"

	"github.com/go-resty/resty/v2"

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

	validate.RegisterStructValidation(TbMciReqStructLevelValidation, model.TbMciReq{})
	validate.RegisterStructValidation(TbVmReqStructLevelValidation, model.TbVmReq{})
	validate.RegisterStructValidation(TbMciCmdReqStructLevelValidation, model.MciCmdReq{})
	// validate.RegisterStructValidation(TbMciRecommendReqStructLevelValidation, MciRecommendReq{})
	// validate.RegisterStructValidation(TbVmRecommendReqStructLevelValidation, TbVmRecommendReq{})
	// validate.RegisterStructValidation(TbBenchmarkReqStructLevelValidation, BenchmarkReq{})
	// validate.RegisterStructValidation(TbMultihostBenchmarkReqStructLevelValidation, MultihostBenchmarkReq{})

	validate.RegisterStructValidation(DFMonAgentInstallReqStructLevelValidation, model.MonAgentInstallReq{})

}

/*
func GenUid() string {
	return uuid.New().String()
}
*/

/*
type resourceIds struct {
	CspImageId           string
	CspImageName         string
	CspSshKeyName        string
	Name                 string // Spec
	CspVNetId            string
	CspVNetName          string
	CspSecurityGroupId   string
	CspSecurityGroupName string
	CspPublicIpId        string
	CspPublicIpName      string
	CspVNicId            string
	CspVNicName          string

	ConnectionName string
}
*/

func CheckMci(nsId string, mciId string) (bool, error) {

	// Check parameters' emptiness
	if nsId == "" {
		err := fmt.Errorf("CheckMci failed; nsId given is null.")
		return false, err
	} else if mciId == "" {
		err := fmt.Errorf("CheckMci failed; mciId given is null.")
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

	key := common.GenMciKey(nsId, mciId, "")

	keyValue, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("In CheckMci(); kvstore.GetKv() returned an error.")
		log.Error().Err(err).Msg("")
		// return nil, err
	}

	if keyValue != (kvstore.KeyValue{}) {
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
	log.Debug().Msg("[Check vm] " + mciId + ", " + vmId)

	key := common.GenMciKey(nsId, mciId, vmId)

	keyValue, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("In CheckVm(); kvstore.GetKv() returned an error.")
		log.Error().Err(err).Msg("")
		// return nil, err
	}

	if keyValue != (kvstore.KeyValue{}) {
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
	log.Debug().Msg("[Check MciPolicy] " + mciId)

	key := common.GenMciPolicyKey(nsId, mciId, "")

	keyValue, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("In CheckMciPolicy(); kvstore.GetKv() returned an error.")
		log.Error().Err(err).Msg("")
		// return nil, err
	}

	if keyValue != (kvstore.KeyValue{}) {
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

	nsList, err := common.ListNsId()
	nullObj := model.InspectResource{}
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
						temp.IdByCsp = nlb.CspNLBId
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
						temp.IdByCsp = vm.CspViewVmDetail.IId.SystemId
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
			resourcesInNs := resourceListInNs.([]model.TbVNetInfo) // type assertion
			if len(resourcesInNs) == 0 {
				continue
			}
			for _, resource := range resourcesInNs {
				if resource.ConnectionName == connConfig { // filtering
					temp := model.ResourceOnTumblebugInfo{}
					temp.IdByTb = resource.Id
					temp.IdByCsp = resource.CspVNetId
					temp.NsId = ns
					temp.ObjectKey = common.GenResourceKey(ns, resourceType, resource.Id)

					TbResourceList.Info = append(TbResourceList.Info, temp)
				}
			}
		case model.StrSecurityGroup:
			resourceListInNs, err := resource.ListResource(ns, resourceType, "", "")
			if err != nil {
				log.Error().Err(err).Msg("")
				err := fmt.Errorf("an error occurred while getting resource list")
				return nullObj, err
			}
			resourcesInNs := resourceListInNs.([]model.TbSecurityGroupInfo) // type assertion
			if len(resourcesInNs) == 0 {
				continue
			}
			for _, resource := range resourcesInNs {
				if resource.ConnectionName == connConfig { // filtering
					temp := model.ResourceOnTumblebugInfo{}
					temp.IdByTb = resource.Id
					temp.IdByCsp = resource.CspSecurityGroupId
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
			resourcesInNs := resourceListInNs.([]model.TbSshKeyInfo) // type assertion
			if len(resourcesInNs) == 0 {
				continue
			}
			for _, resource := range resourcesInNs {
				if resource.ConnectionName == connConfig { // filtering
					temp := model.ResourceOnTumblebugInfo{}
					temp.IdByTb = resource.Id
					temp.IdByCsp = resource.CspSshKeyId
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
			resourcesInNs := resourceListInNs.([]model.TbDataDiskInfo) // type assertion
			if len(resourcesInNs) == 0 {
				continue
			}
			for _, resource := range resourcesInNs {
				if resource.ConnectionName == connConfig { // filtering
					temp := model.ResourceOnTumblebugInfo{}
					temp.IdByTb = resource.Id
					temp.IdByCsp = resource.CspDataDiskId
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
			resourcesInNs := resourceListInNs.([]model.TbCustomImageInfo) // type assertion
			if len(resourcesInNs) == 0 {
				continue
			}
			for _, resource := range resourcesInNs {
				if resource.ConnectionName == connConfig { // filtering
					temp := model.ResourceOnTumblebugInfo{}
					temp.IdByTb = resource.Id
					temp.IdByCsp = resource.CspCustomImageId
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

	client := resty.New().SetCloseConnection(true)
	client.SetAllowGetMethodPayload(true)

	// Create Req body
	type JsonTemplate struct {
		ConnectionName string
	}
	requestBody := JsonTemplate{}
	requestBody.ConnectionName = connConfig

	var spiderRequestURL string
	switch resourceType {
	case model.StrNLB:
		spiderRequestURL = model.SpiderRestUrl + "/allnlb"
	case model.StrVM:
		spiderRequestURL = model.SpiderRestUrl + "/allvm"
	case model.StrVNet:
		spiderRequestURL = model.SpiderRestUrl + "/allvpc"
	case model.StrSecurityGroup:
		spiderRequestURL = model.SpiderRestUrl + "/allsecuritygroup"
	case model.StrSSHKey:
		spiderRequestURL = model.SpiderRestUrl + "/allkeypair"
	case model.StrDataDisk:
		spiderRequestURL = model.SpiderRestUrl + "/alldisk"
	case model.StrCustomImage:
		spiderRequestURL = model.SpiderRestUrl + "/allmyimage"
	default:
		err = fmt.Errorf("Invalid resourceType: " + resourceType)
		return nullObj, err
	}

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(requestBody).
		SetResult(&model.SpiderAllListWrapper{}). // or SetResult(AuthSuccess{}).
		//SetError(&AuthError{}).       // or SetError(AuthError{}).
		Get(spiderRequestURL)

	if err != nil {
		log.Error().Err(err).Msg("")
		err := fmt.Errorf("an error occurred while requesting to CB-Spider")
		return nullObj, err
	}

	fmt.Println("HTTP Status code: " + strconv.Itoa(resp.StatusCode()))
	switch {
	case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
		err := fmt.Errorf(string(resp.Body()))
		log.Error().Err(err).Msg("")
		return nullObj, err
	default:
	}

	temp, _ := resp.Result().(*model.SpiderAllListWrapper) // type assertion

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

	result.Resources.OnTumblebug = TbResourceList
	//result.ResourcesOnTumblebug.Info = append(result.ResourcesOnTumblebug.Info, TbResourceList...)

	// result.ResourcesOnCsp = append((*temp).AllList.MappedList, (*temp).AllList.OnlyCSPList...)
	// result.ResourcesOnSpider = append((*temp).AllList.MappedList, (*temp).AllList.OnlySpiderList...)
	result.Resources.OnSpider = model.ResourceOnSpider{}
	result.Resources.OnCspTotal = model.ResourceOnCsp{}
	result.Resources.OnCspOnly = model.ResourceOnCsp{}

	tmpResourceOnSpider := model.ResourceOnSpiderInfo{}
	tmpResourceOnCsp := model.ResourceOnCspInfo{}

	for _, v := range (*temp).AllList.MappedList {
		tmpResourceOnSpider.IdBySp = v.NameId
		tmpResourceOnSpider.IdByCsp = v.SystemId
		result.Resources.OnSpider.Info = append(result.Resources.OnSpider.Info, tmpResourceOnSpider)

		tmpResourceOnCsp.IdByCsp = v.SystemId
		tmpResourceOnCsp.RefNameOrId = v.NameId
		result.Resources.OnCspTotal.Info = append(result.Resources.OnCspTotal.Info, tmpResourceOnCsp)
	}

	for _, v := range (*temp).AllList.OnlySpiderList {
		tmpResourceOnSpider.IdBySp = v.NameId
		tmpResourceOnSpider.IdByCsp = v.SystemId
		result.Resources.OnSpider.Info = append(result.Resources.OnSpider.Info, tmpResourceOnSpider)
	}

	for _, v := range (*temp).AllList.OnlyCSPList {
		tmpResourceOnCsp.IdByCsp = v.SystemId
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

	output := model.InspectResourceAllResult{}

	var wait sync.WaitGroup
	for _, k := range connectionConfigList.Connectionconfig {
		wait.Add(1)
		go func(k model.ConnConfig) {
			defer wait.Done()

			common.RandomSleep(0, 60)
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
					common.RandomSleep(40, 80)
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

			temp.ElapsedTime = int(math.Round(time.Now().Sub(startTimeForConnection).Seconds()))

			output.InspectResult = append(output.InspectResult, temp)

		}(k)
	}
	wait.Wait()

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

	output.ElapsedTime = int(math.Round(time.Now().Sub(startTime).Seconds()))
	output.RegisteredConnection = len(connectionConfigList.Connectionconfig)
	output.AvailableConnection = output.RegisteredConnection - errorConnectionCnt

	return output, err
}

// RegisterCspNativeResourcesAll func registers all CSP-native resources into CB-TB
func RegisterCspNativeResourcesAll(nsId string, mciId string, option string, mciFlag string) (model.RegisterResourceAllResult, error) {
	startTime := time.Now()

	connectionConfigList, err := common.GetConnConfigList(model.DefaultCredentialHolder, true, true)
	if err != nil {
		err := fmt.Errorf("Cannot load ConnectionConfigList")
		log.Error().Err(err).Msg("")
		return model.RegisterResourceAllResult{}, err
	}

	output := model.RegisterResourceAllResult{}

	var wait sync.WaitGroup
	for _, k := range connectionConfigList.Connectionconfig {
		wait.Add(1)
		go func(k model.ConnConfig) {
			defer wait.Done()

			mciNameForRegister := mciId + "-" + k.ConfigName
			// Assign RandomSleep range by clouds
			// This code is temporal, CB-Spider needs to be enhnaced for locking mechanism.
			// CB-SP v0.5.9 will not help with rate limit issue.
			if option != "onlyVm" {
				if strings.Contains(k.ConfigName, "alibaba") {
					common.RandomSleep(100, 200)
				} else if strings.Contains(k.ConfigName, "aws") {
					common.RandomSleep(300, 500)
				} else if strings.Contains(k.ConfigName, "gcp") {
					common.RandomSleep(700, 900)
				} else {
				}
			}

			common.RandomSleep(0, 50)

			registerResult, err := RegisterCspNativeResources(nsId, k.ConfigName, mciNameForRegister, option, mciFlag)
			if err != nil {
				log.Error().Err(err).Msg("")
			}

			output.RegisterationResult = append(output.RegisterationResult, registerResult)

		}(k)
	}
	wait.Wait()

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

	output.ElapsedTime = int(math.Round(time.Now().Sub(startTime).Seconds()))
	output.RegisteredConnection = len(connectionConfigList.Connectionconfig)
	output.AvailableConnection = output.RegisteredConnection - errorConnectionCnt

	sort.SliceStable(output.RegisterationResult, func(i, j int) bool {
		return output.RegisterationResult[i].ConnectionName < output.RegisterationResult[j].ConnectionName
	})

	return output, err
}

// RegisterCspNativeResources func registers all CSP-native resources into CB-TB
func RegisterCspNativeResources(nsId string, connConfig string, mciId string, option string, mciFlag string) (model.RegisterResourceResult, error) {
	startTime := time.Now()

	optionFlag := "register"
	registeredStatus := ""
	result := model.RegisterResourceResult{}

	startTime01 := time.Now() //tmp
	var err error

	if option != "onlyVm" {
		// bring vNet list and register all
		inspectedResources, err := InspectResources(connConfig, model.StrVNet)
		if err != nil {
			log.Error().Err(err).Msg("")
			result.SystemMessage = err.Error()
		}
		for _, r := range inspectedResources.Resources.OnCspOnly.Info {
			req := model.TbRegisterVNetReq{}
			req.ConnectionName = connConfig
			req.CspVNetId = r.IdByCsp
			req.Description = "Ref name: " + r.RefNameOrId + ". CSP managed resource (registered to CB-TB)"
			req.Name = req.ConnectionName + "-" + req.CspVNetId
			req.Name = common.ChangeIdString(req.Name)

			_, err = resource.RegisterVNet(nsId, &req)

			registeredStatus = ""
			if err != nil {
				log.Error().Err(err).Msg("")
				registeredStatus = "  [Failed] " + err.Error()
				result.RegisterationOverview.VNet--
				result.RegisterationOverview.Failed++
			}
			result.RegisterationOutputs.IdList = append(result.RegisterationOutputs.IdList, model.StrVNet+": "+req.Name+registeredStatus)
			result.RegisterationOverview.VNet++
		}

		fmt.Printf("\n\n%s [Elapsed]%s %d \n\n", connConfig, model.StrVNet, int(math.Round(time.Now().Sub(startTime01).Seconds()))) //tmp
		startTime02 := time.Now()                                                                                                   //tmp

		// bring SecurityGroup list and register all
		inspectedResources, err = InspectResources(connConfig, model.StrSecurityGroup)
		if err != nil {
			log.Error().Err(err).Msg("")
			result.SystemMessage += "//" + err.Error()
		}
		for _, r := range inspectedResources.Resources.OnCspOnly.Info {
			req := model.TbSecurityGroupReq{}
			req.ConnectionName = connConfig
			req.VNetId = "not defined"
			req.CspSecurityGroupId = r.IdByCsp
			req.Description = "Ref name: " + r.RefNameOrId + ". CSP managed resource (registered to CB-TB)"
			req.Name = req.ConnectionName + "-" + req.CspSecurityGroupId
			req.Name = common.ChangeIdString(req.Name)

			_, err = resource.CreateSecurityGroup(nsId, &req, optionFlag)

			registeredStatus = ""
			if err != nil {
				log.Error().Err(err).Msg("")
				registeredStatus = "  [Failed] " + err.Error()
				result.RegisterationOverview.SecurityGroup--
				result.RegisterationOverview.Failed++
			}
			result.RegisterationOutputs.IdList = append(result.RegisterationOutputs.IdList, model.StrSecurityGroup+": "+req.Name+registeredStatus)
			result.RegisterationOverview.SecurityGroup++
		}

		fmt.Printf("\n\n%s [Elapsed]%s %d \n\n", connConfig, model.StrSecurityGroup, int(math.Round(time.Now().Sub(startTime02).Seconds()))) //tmp
		startTime03 := time.Now()                                                                                                            //tmp

		// bring SSHKey list and register all
		inspectedResources, err = InspectResources(connConfig, model.StrSSHKey)
		if err != nil {
			log.Error().Err(err).Msg("")
			result.SystemMessage += "//" + err.Error()
		}
		for _, r := range inspectedResources.Resources.OnCspOnly.Info {
			req := model.TbSshKeyReq{}
			req.ConnectionName = connConfig
			req.CspSshKeyId = r.IdByCsp
			req.Description = "Ref name: " + r.RefNameOrId + ". CSP managed resource (registered to CB-TB)"
			req.Name = req.ConnectionName + "-" + req.CspSshKeyId
			req.Name = common.ChangeIdString(req.Name)

			req.Fingerprint = "cannot retrieve"
			req.PrivateKey = "cannot retrieve"
			req.PublicKey = "cannot retrieve"
			req.Username = "cannot retrieve"

			_, err = resource.CreateSshKey(nsId, &req, optionFlag)

			registeredStatus = ""
			if err != nil {
				log.Error().Err(err).Msg("")
				registeredStatus = "  [Failed] " + err.Error()
				result.RegisterationOverview.SshKey--
				result.RegisterationOverview.Failed++
			}
			result.RegisterationOutputs.IdList = append(result.RegisterationOutputs.IdList, model.StrSSHKey+": "+req.Name+registeredStatus)
			result.RegisterationOverview.SshKey++
		}

		fmt.Printf("\n\n%s [Elapsed]%s %d \n\n", connConfig, model.StrSSHKey, int(math.Round(time.Now().Sub(startTime03).Seconds()))) //tmp

		startTime04 := time.Now() //tmp

		// bring DataDisk list and register all
		inspectedResources, err = InspectResources(connConfig, model.StrDataDisk)
		if err != nil {
			log.Error().Err(err).Msg("")
			result.SystemMessage += "//" + err.Error()
		}
		for _, r := range inspectedResources.Resources.OnCspOnly.Info {
			req := model.TbDataDiskReq{
				Name:           fmt.Sprintf("%s-%s", connConfig, r.IdByCsp),
				ConnectionName: connConfig,
				CspDataDiskId:  r.IdByCsp,
			}
			req.Name = common.ChangeIdString(req.Name)

			_, err = resource.CreateDataDisk(nsId, &req, optionFlag)

			registeredStatus = ""
			if err != nil {
				log.Error().Err(err).Msg("")
				registeredStatus = "  [Failed] " + err.Error()
				result.RegisterationOverview.DataDisk--
				result.RegisterationOverview.Failed++
			}
			result.RegisterationOutputs.IdList = append(result.RegisterationOutputs.IdList, model.StrDataDisk+": "+req.Name+registeredStatus)
			result.RegisterationOverview.DataDisk++
		}

		fmt.Printf("\n\n%s [Elapsed]%s %d \n\n", connConfig, model.StrDataDisk, int(math.Round(time.Now().Sub(startTime04).Seconds()))) //tmp

		startTime05 := time.Now() //tmp

		// bring CustomImage list and register all
		inspectedResources, err = InspectResources(connConfig, model.StrCustomImage)
		if err != nil {
			log.Error().Err(err).Msg("")
			result.SystemMessage += "//" + err.Error()
		}
		for _, r := range inspectedResources.Resources.OnCspOnly.Info {
			req := model.TbCustomImageReq{
				Name:             fmt.Sprintf("%s-%s", connConfig, r.IdByCsp),
				ConnectionName:   connConfig,
				CspCustomImageId: r.IdByCsp,
			}
			req.Name = common.ChangeIdString(req.Name)

			_, err = resource.RegisterCustomImageWithId(nsId, &req)

			registeredStatus = ""
			if err != nil {
				log.Error().Err(err).Msg("")
				registeredStatus = "  [Failed] " + err.Error()
				result.RegisterationOverview.CustomImage--
				result.RegisterationOverview.Failed++
			}
			result.RegisterationOutputs.IdList = append(result.RegisterationOutputs.IdList, model.StrCustomImage+": "+req.Name+registeredStatus)
			result.RegisterationOverview.CustomImage++
		}

		fmt.Printf("\n\n%s [Elapsed]%s %d \n\n", connConfig, model.StrCustomImage, int(math.Round(time.Now().Sub(startTime05).Seconds()))) //tmp
	}

	startTime06 := time.Now() //tmp

	if option != "exceptVm" {

		// bring VM list and register all
		inspectedResourcesVm, err := InspectResources(connConfig, model.StrVM)
		if err != nil {
			log.Error().Err(err).Msg("")
			result.SystemMessage += "//" + err.Error()
		}
		for _, r := range inspectedResourcesVm.Resources.OnCspOnly.Info {
			req := model.TbMciReq{}
			req.Description = "MCI for CSP managed VMs (registered to CB-TB)"
			req.InstallMonAgent = "no"
			req.Name = mciId
			req.Name = common.ChangeIdString(req.Name)

			vm := model.TbVmReq{}
			vm.ConnectionName = connConfig
			vm.IdByCSP = r.IdByCsp
			vm.Description = "Ref name: " + r.RefNameOrId + ". CSP managed VM (registered to CB-TB)"
			vm.Name = vm.ConnectionName + "-" + r.RefNameOrId + "-" + vm.IdByCSP
			vm.Name = common.ChangeIdString(vm.Name)
			if mciFlag == "n" {
				// (if mciFlag == "n") create a mci for each vm
				req.Name = vm.Name
			}
			vm.Label = "not defined"

			vm.ImageId = "cannot retrieve"
			vm.SpecId = "cannot retrieve"
			vm.SshKeyId = "cannot retrieve"
			vm.SubnetId = "cannot retrieve"
			vm.VNetId = "cannot retrieve"
			vm.SecurityGroupIds = append(vm.SecurityGroupIds, "cannot retrieve")

			req.Vm = append(req.Vm, vm)

			_, err = CreateMci(nsId, &req, optionFlag)

			registeredStatus = ""
			if err != nil {
				log.Error().Err(err).Msg("")
				registeredStatus = "  [Failed] " + err.Error()
				result.RegisterationOverview.Vm--
				result.RegisterationOverview.Failed++
			}
			result.RegisterationOutputs.IdList = append(result.RegisterationOutputs.IdList, model.StrVM+": "+vm.Name+registeredStatus)
			result.RegisterationOverview.Vm++
		}
	}

	result.ConnectionName = connConfig
	result.ElapsedTime = int(math.Round(time.Now().Sub(startTime).Seconds()))

	fmt.Printf("\n\n%s [Elapsed]%s %d \n\n", connConfig, model.StrVM, int(math.Round(time.Now().Sub(startTime06).Seconds()))) //tmp

	fmt.Printf("\n\n%s [Elapsed]Total %d \n\n", connConfig, int(math.Round(time.Now().Sub(startTime).Seconds())))

	return result, err

}

func FindTbVmByCspId(nsId string, mciId string, vmIdByCsp string) (model.TbVmInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.TbVmInfo{}, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.TbVmInfo{}, err
	}

	err = common.CheckString(vmIdByCsp)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.TbVmInfo{}, err
	}

	check, err := CheckMci(nsId, mciId)

	if !check {
		err := fmt.Errorf("The MCI " + mciId + " does not exist.")
		return model.TbVmInfo{}, err
	}

	if err != nil {
		err := fmt.Errorf("Failed to check the existence of the MCI " + mciId + ".")
		return model.TbVmInfo{}, err
	}

	mci, err := GetMciObject(nsId, mciId)
	if err != nil {
		err := fmt.Errorf("Failed to get the MCI " + mciId + ".")
		return model.TbVmInfo{}, err
	}

	vms := mci.Vm
	for _, v := range vms {
		if v.IdByCSP == vmIdByCsp || v.CspViewVmDetail.IId.NameId == vmIdByCsp {
			return v, nil
		}
	}

	err = fmt.Errorf("Cannot find the VM %s in %s/%s", vmIdByCsp, nsId, mciId)
	return model.TbVmInfo{}, err
}
