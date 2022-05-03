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
	//"encoding/json"
	//uuid "github.com/google/uuid"
	"fmt"
	"strconv"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"

	"github.com/go-resty/resty/v2"

	"math"
	"reflect"
	"sync"
	"time"

	validator "github.com/go-playground/validator/v10"
)

// CB-Store
//var cblog *logrus.Logger
//var store icbs.Store

//var SPIDER_REST_URL string

// use a single instance of Validate, it caches struct info
var validate *validator.Validate

func init() {
	//cblog = config.Cblogger
	//store = cbstore.GetStore()
	//SPIDER_REST_URL = os.Getenv("SPIDER_REST_URL")

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

	validate.RegisterStructValidation(TbMcisReqStructLevelValidation, TbMcisReq{})
	validate.RegisterStructValidation(TbVmReqStructLevelValidation, TbVmReq{})
	validate.RegisterStructValidation(TbMcisCmdReqStructLevelValidation, McisCmdReq{})
	// validate.RegisterStructValidation(TbMcisRecommendReqStructLevelValidation, McisRecommendReq{})
	// validate.RegisterStructValidation(TbVmRecommendReqStructLevelValidation, TbVmRecommendReq{})
	// validate.RegisterStructValidation(TbBenchmarkReqStructLevelValidation, BenchmarkReq{})
	// validate.RegisterStructValidation(TbMultihostBenchmarkReqStructLevelValidation, MultihostBenchmarkReq{})

	validate.RegisterStructValidation(DFMonAgentInstallReqStructLevelValidation, MonAgentInstallReq{})

}

/*
func GenUid() string {
	return uuid.New().String()
}
*/

/*
type mcirIds struct {
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

func CheckMcis(nsId string, mcisId string) (bool, error) {

	// Check parameters' emptiness
	if nsId == "" {
		err := fmt.Errorf("CheckMcis failed; nsId given is null.")
		return false, err
	} else if mcisId == "" {
		err := fmt.Errorf("CheckMcis failed; mcisId given is null.")
		return false, err
	}

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return false, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return false, err
	}
	fmt.Println("[Check mcis] " + mcisId)

	key := common.GenMcisKey(nsId, mcisId, "")

	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
		err = fmt.Errorf("In CheckMcis(); CBStore.Get() returned an error.")
		common.CBLog.Error(err)
		// return nil, err
	}

	if keyValue != nil {
		return true, nil
	}
	return false, nil

}

func CheckVm(nsId string, mcisId string, vmId string) (bool, error) {

	// Check parameters' emptiness
	if nsId == "" {
		err := fmt.Errorf("CheckVm failed; nsId given is null.")
		return false, err
	} else if mcisId == "" {
		err := fmt.Errorf("CheckVm failed; mcisId given is null.")
		return false, err
	} else if vmId == "" {
		err := fmt.Errorf("CheckVm failed; vmId given is null.")
		return false, err
	}

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return false, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return false, err
	}
	err = common.CheckString(vmId)
	if err != nil {
		common.CBLog.Error(err)
		return false, err
	}
	fmt.Println("[Check vm] " + mcisId + ", " + vmId)

	key := common.GenMcisKey(nsId, mcisId, vmId)

	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
		err = fmt.Errorf("In CheckVm(); CBStore.Get() returned an error.")
		common.CBLog.Error(err)
		// return nil, err
	}

	if keyValue != nil {
		return true, nil
	}
	return false, nil

}

func CheckMcisPolicy(nsId string, mcisId string) (bool, error) {

	// Check parameters' emptiness
	if nsId == "" {
		err := fmt.Errorf("CheckMcis failed; nsId given is null.")
		return false, err
	} else if mcisId == "" {
		err := fmt.Errorf("CheckMcis failed; mcisId given is null.")
		return false, err
	}

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return false, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return false, err
	}
	fmt.Println("[Check McisPolicy] " + mcisId)

	key := common.GenMcisPolicyKey(nsId, mcisId, "")

	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
		err = fmt.Errorf("In CheckMcisPolicy(); CBStore.Get() returned an error.")
		common.CBLog.Error(err)
		// return nil, err
	}

	if keyValue != nil {
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

type SpiderNameIdSystemId struct {
	NameId   string
	SystemId string
}

type SpiderAllListWrapper struct {
	AllList SpiderAllList
}

type SpiderAllList struct {
	MappedList     []SpiderNameIdSystemId
	OnlySpiderList []SpiderNameIdSystemId
	OnlyCSPList    []SpiderNameIdSystemId
}

// TbInspectResourcesResponse is struct for response of InspectResources request
type TbInspectResourcesResponse struct {
	InspectResources []InspectResource `json:"inspectResources"`
}

// InspectResource is struct for InspectResource per Cloud Connection
type InspectResource struct {
	// ResourcesOnCsp       interface{} `json:"resourcesOnCsp"`
	// ResourcesOnSpider    interface{} `json:"resourcesOnSpider"`
	// ResourcesOnTumblebug interface{} `json:"resourcesOnTumblebug"`

	ConnectionName   string                `json:"connectionName"`
	ResourceType     string                `json:"resourceType"`
	SystemMessage    string                `json:"systemMessage"`
	ResourceOverview resourceCountOverview `json:"resourceOverview"`
	Resources        resourcesByManageType `json:"resources"`
}

type resourceCountOverview struct {
	OnTumblebug int `json:"onTumblebug"`
	OnSpider    int `json:"onSpider"`
	OnCspTotal  int `json:"onCspTotal"`
	OnCspOnly   int `json:"onCspOnly"`
}

type resourcesByManageType struct {
	OnTumblebug resourceOnTumblebug `json:"onTumblebug"`
	OnSpider    resourceOnSpider    `json:"onSpider"`
	OnCspTotal  resourceOnCsp       `json:"onCspTotal"`
	OnCspOnly   resourceOnCsp       `json:"onCspOnly"`
}

type resourceOnSpider struct {
	Count int                    `json:"count"`
	Info  []resourceOnSpiderInfo `json:"info"`
}

type resourceOnSpiderInfo struct {
	IdBySp  string `json:"idBySp"`
	IdByCsp string `json:"idByCsp"`
}

type resourceOnCsp struct {
	Count int                 `json:"count"`
	Info  []resourceOnCspInfo `json:"info"`
}

type resourceOnCspInfo struct {
	IdByCsp     string `json:"idByCsp"`
	RefNameOrId string `json:"refNameOrId"`
}

type resourceOnTumblebug struct {
	Count int                       `json:"count"`
	Info  []resourceOnTumblebugInfo `json:"info"`
}

type resourceOnTumblebugInfo struct {
	IdByTb    string `json:"idByTb"`
	IdByCsp   string `json:"idByCsp"`
	NsId      string `json:"nsId"`
	McisId    string `json:"mcisId,omitempty"`
	ObjectKey string `json:"objectKey"`
}

// InspectResources returns the state list of TB MCIR objects of given connConfig and resourceType
func InspectResources(connConfig string, resourceType string) (InspectResource, error) {

	nsList, err := common.ListNsId()
	nullObj := InspectResource{}
	if err != nil {
		common.CBLog.Error(err)
		err = fmt.Errorf("an error occurred while getting namespaces' list: " + err.Error())
		return nullObj, err
	}
	// var TbResourceList []string
	TbResourceList := resourceOnTumblebug{}
	for _, ns := range nsList {

		// Bring TB resources
		switch resourceType {
		case common.StrVM:
			mcisListinNs, _ := ListMcisId(ns)
			if mcisListinNs == nil {
				continue
			}
			for _, mcis := range mcisListinNs {
				vmListInMcis, err := ListVmId(ns, mcis)
				if err != nil {
					common.CBLog.Error(err)
					err := fmt.Errorf("an error occurred while getting resource list")
					return nullObj, err
				}
				if vmListInMcis == nil {
					continue
				}

				for _, vmId := range vmListInMcis {
					vm, err := GetVmObject(ns, mcis, vmId)
					if err != nil {
						common.CBLog.Error(err)
						err := fmt.Errorf("an error occurred while getting resource list")
						return nullObj, err
					}

					if vm.ConnectionName == connConfig { // filtering
						temp := resourceOnTumblebugInfo{}
						temp.IdByTb = vm.Id
						temp.IdByCsp = vm.CspViewVmDetail.IId.SystemId
						temp.NsId = ns
						temp.McisId = mcis
						temp.ObjectKey = common.GenMcisKey(ns, mcis, vm.Id)

						TbResourceList.Info = append(TbResourceList.Info, temp)
					}
				}
			}
		case common.StrVNet:
			resourceListInNs, err := mcir.ListResource(ns, resourceType)
			if err != nil {
				common.CBLog.Error(err)
				err := fmt.Errorf("an error occurred while getting resource list")
				return nullObj, err
			}
			if resourceListInNs == nil {
				continue
			}
			resourcesInNs := resourceListInNs.([]mcir.TbVNetInfo) // type assertion
			for _, resource := range resourcesInNs {
				if resource.ConnectionName == connConfig { // filtering
					temp := resourceOnTumblebugInfo{}
					temp.IdByTb = resource.Id
					temp.IdByCsp = resource.CspVNetId
					temp.NsId = ns
					temp.ObjectKey = common.GenResourceKey(ns, resourceType, resource.Id)

					TbResourceList.Info = append(TbResourceList.Info, temp)
				}
			}
		case common.StrSecurityGroup:
			resourceListInNs, err := mcir.ListResource(ns, resourceType)
			if err != nil {
				common.CBLog.Error(err)
				err := fmt.Errorf("an error occurred while getting resource list")
				return nullObj, err
			}
			if resourceListInNs == nil {
				continue
			}
			resourcesInNs := resourceListInNs.([]mcir.TbSecurityGroupInfo) // type assertion
			for _, resource := range resourcesInNs {
				if resource.ConnectionName == connConfig { // filtering
					temp := resourceOnTumblebugInfo{}
					temp.IdByTb = resource.Id
					temp.IdByCsp = resource.CspSecurityGroupId
					temp.NsId = ns
					temp.ObjectKey = common.GenResourceKey(ns, resourceType, resource.Id)

					TbResourceList.Info = append(TbResourceList.Info, temp)
				}
			}
		case common.StrSSHKey:
			resourceListInNs, err := mcir.ListResource(ns, resourceType)
			if err != nil {
				common.CBLog.Error(err)
				err := fmt.Errorf("an error occurred while getting resource list")
				return nullObj, err
			}
			if resourceListInNs == nil {
				continue
			}
			resourcesInNs := resourceListInNs.([]mcir.TbSshKeyInfo) // type assertion
			for _, resource := range resourcesInNs {
				if resource.ConnectionName == connConfig { // filtering
					temp := resourceOnTumblebugInfo{}
					temp.IdByTb = resource.Id
					temp.IdByCsp = resource.CspSshKeyName
					temp.NsId = ns
					temp.ObjectKey = common.GenResourceKey(ns, resourceType, resource.Id)

					TbResourceList.Info = append(TbResourceList.Info, temp)
				}
			}
		}
	}

	client := resty.New().SetCloseConnection(true)
	client.SetAllowGetMethodPayload(true)

	// Create Req body
	type JsonTemplate struct {
		ConnectionName string
	}
	tempReq := JsonTemplate{}
	tempReq.ConnectionName = connConfig

	var spiderRequestURL string
	switch resourceType {
	case common.StrVM:
		spiderRequestURL = common.SpiderRestUrl + "/allvm"
	case common.StrVNet:
		spiderRequestURL = common.SpiderRestUrl + "/allvpc"
	case common.StrSecurityGroup:
		spiderRequestURL = common.SpiderRestUrl + "/allsecuritygroup"
	case common.StrSSHKey:
		spiderRequestURL = common.SpiderRestUrl + "/allkeypair"
	}

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(tempReq).
		SetResult(&SpiderAllListWrapper{}). // or SetResult(AuthSuccess{}).
		//SetError(&AuthError{}).       // or SetError(AuthError{}).
		Get(spiderRequestURL)

	if err != nil {
		common.CBLog.Error(err)
		err := fmt.Errorf("an error occurred while requesting to CB-Spider")
		return nullObj, err
	}

	fmt.Println("HTTP Status code: " + strconv.Itoa(resp.StatusCode()))
	switch {
	case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
		err := fmt.Errorf(string(resp.Body()))
		common.CBLog.Error(err)
		return nullObj, err
	default:
	}

	temp, _ := resp.Result().(*SpiderAllListWrapper) // type assertion

	result := InspectResource{}

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
	result.Resources.OnSpider = resourceOnSpider{}
	result.Resources.OnCspTotal = resourceOnCsp{}
	result.Resources.OnCspOnly = resourceOnCsp{}

	tmpResourceOnSpider := resourceOnSpiderInfo{}
	tmpResourceOnCsp := resourceOnCspInfo{}

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

// RegisterCspNativeResourceResultAll is struct for Register Csp Native Resource Result for All Clouds
type RegisterResourceAllResult struct {
	RegisterationResult []RegisterResourceResult `json:"registerationResult"`
}

// RegisterCspNativeResourceResult is struct for Register Csp Native Resource Result
type RegisterResourceResult struct {
	ConnectionName        string                `json:"connectionName"`
	SystemMessage         string                `json:"systemMessage"`
	ElapsedTime           int                   `json:"elapsedTime"`
	RegisterationOverview registerationOverview `json:"registerationOverview"`
	RegisterationOutputs  common.IdList         `json:"registerationOutputs"`
}

type registerationOverview struct {
	VNet          int `json:"vNet"`
	SecurityGroup int `json:"securityGroup"`
	SshKey        int `json:"sshKey"`
	Vm            int `json:"vm"`
	Failed        int `json:"failed"`
}

// RegisterCspNativeResourcesAll func registers all CSP-native resources into CB-TB
func RegisterCspNativeResourcesAll(nsId string, mcisId string) (RegisterResourceAllResult, error) {

	connectionConfigList, err := common.GetConnConfigList()
	if err != nil {
		err := fmt.Errorf("Cannnot load ConnectionConfigList")
		common.CBLog.Error(err)
		return RegisterResourceAllResult{}, err
	}
	refinedConnectionConfigList := common.ConnConfigList{}
	for _, k := range connectionConfigList.Connectionconfig {
		if !strings.Contains(k.ConfigName, "gcp") {
			refinedConnectionConfigList.Connectionconfig = append(refinedConnectionConfigList.Connectionconfig, k)
		}
	}

	output := RegisterResourceAllResult{}

	var wait sync.WaitGroup
	for _, k := range refinedConnectionConfigList.Connectionconfig {
		wait.Add(1)
		go func(k common.ConnConfig) {
			defer wait.Done()

			mcisNameForRegister := mcisId + "-" + k.ConfigName
			//common.RandomSleep(300)
			registerResult, err := RegisterCspNativeResources(nsId, k.ConfigName, mcisNameForRegister)
			if err != nil {
				common.CBLog.Error(err)
			}

			output.RegisterationResult = append(output.RegisterationResult, registerResult)

		}(k)
	}
	wait.Wait()

	return output, err
}

// RegisterCspNativeResources func registers all CSP-native resources into CB-TB
func RegisterCspNativeResources(nsId string, connConfig string, mcisId string) (RegisterResourceResult, error) {
	startTime := time.Now()

	optionFlag := "register"
	registeredStatus := ""
	result := RegisterResourceResult{}

	startTime01 := time.Now() //tmp

	// bring vNet list and register all
	inspectedResources, err := InspectResources(connConfig, common.StrVNet)
	if err != nil {
		common.CBLog.Error(err)
		result.SystemMessage = err.Error()
	}
	for _, r := range inspectedResources.Resources.OnCspOnly.Info {
		req := mcir.TbVNetReq{}
		req.ConnectionName = connConfig
		req.CspVNetId = r.IdByCsp
		req.Description = "Ref name: " + r.RefNameOrId + ". CSP managed resource (registered to CB-TB)"
		req.Name = req.ConnectionName + "-" + req.CspVNetId
		req.Name = strings.ToLower(req.Name)

		_, err = mcir.CreateVNet(nsId, &req, optionFlag)

		registeredStatus = ""
		if err != nil {
			common.CBLog.Error(err)
			registeredStatus = "  [Failed] " + err.Error()
			result.RegisterationOverview.VNet--
			result.RegisterationOverview.Failed++
		}
		result.RegisterationOutputs.IdList = append(result.RegisterationOutputs.IdList, common.StrVNet+": "+req.Name+registeredStatus)
		result.RegisterationOverview.VNet++
	}

	fmt.Printf("\n\n%s [Elapsed]%s %d \n\n", connConfig, common.StrVNet, int(math.Round(time.Now().Sub(startTime01).Seconds()))) //tmp
	startTime02 := time.Now()                                                                                                    //tmp

	// bring SecurityGroup list and register all
	inspectedResources, err = InspectResources(connConfig, common.StrSecurityGroup)
	if err != nil {
		common.CBLog.Error(err)
		result.SystemMessage += "//" + err.Error()
	}
	for _, r := range inspectedResources.Resources.OnCspOnly.Info {
		req := mcir.TbSecurityGroupReq{}
		req.ConnectionName = connConfig
		req.VNetId = "not defined"
		req.CspSecurityGroupId = r.IdByCsp
		req.Description = "Ref name: " + r.RefNameOrId + ". CSP managed resource (registered to CB-TB)"
		req.Name = req.ConnectionName + "-" + req.CspSecurityGroupId
		req.Name = strings.ToLower(req.Name)

		_, err = mcir.CreateSecurityGroup(nsId, &req, optionFlag)

		registeredStatus = ""
		if err != nil {
			common.CBLog.Error(err)
			registeredStatus = "  [Failed] " + err.Error()
			result.RegisterationOverview.SecurityGroup--
			result.RegisterationOverview.Failed++
		}
		result.RegisterationOutputs.IdList = append(result.RegisterationOutputs.IdList, common.StrSecurityGroup+": "+req.Name+registeredStatus)
		result.RegisterationOverview.SecurityGroup++
	}

	fmt.Printf("\n\n%s [Elapsed]%s %d \n\n", connConfig, common.StrSecurityGroup, int(math.Round(time.Now().Sub(startTime02).Seconds()))) //tmp
	startTime03 := time.Now()                                                                                                             //tmp

	// bring SSHKey list and register all
	inspectedResources, err = InspectResources(connConfig, common.StrSSHKey)
	if err != nil {
		common.CBLog.Error(err)
		result.SystemMessage += "//" + err.Error()
	}
	for _, r := range inspectedResources.Resources.OnCspOnly.Info {
		req := mcir.TbSshKeyReq{}
		req.ConnectionName = connConfig
		req.CspSshKeyId = r.IdByCsp
		req.Description = "Ref name: " + r.RefNameOrId + ". CSP managed resource (registered to CB-TB)"
		req.Name = req.ConnectionName + "-" + req.CspSshKeyId
		req.Name = strings.ToLower(req.Name)

		req.Fingerprint = "cannot retrieve"
		req.PrivateKey = "cannot retrieve"
		req.PublicKey = "cannot retrieve"
		req.Username = "cannot retrieve"

		_, err = mcir.CreateSshKey(nsId, &req, optionFlag)

		registeredStatus = ""
		if err != nil {
			common.CBLog.Error(err)
			registeredStatus = "  [Failed] " + err.Error()
			result.RegisterationOverview.SshKey--
			result.RegisterationOverview.Failed++
		}
		result.RegisterationOutputs.IdList = append(result.RegisterationOutputs.IdList, common.StrSSHKey+": "+req.Name+registeredStatus)
		result.RegisterationOverview.SshKey++
	}

	fmt.Printf("\n\n%s [Elapsed]%s %d \n\n", connConfig, common.StrSSHKey, int(math.Round(time.Now().Sub(startTime03).Seconds()))) //tmp
	startTime04 := time.Now()                                                                                                      //tmp

	// bring VM list and register all
	inspectedResources, err = InspectResources(connConfig, common.StrVM)
	if err != nil {
		common.CBLog.Error(err)
		result.SystemMessage += "//" + err.Error()
	}
	for _, r := range inspectedResources.Resources.OnCspOnly.Info {
		req := TbMcisReq{}
		req.Description = "MCIS for CSP managed VMs (registered to CB-TB)"
		req.InstallMonAgent = "no"
		req.Name = mcisId
		req.Name = strings.ToLower(req.Name)

		vm := TbVmReq{}
		vm.ConnectionName = connConfig
		vm.IdByCSP = r.IdByCsp
		vm.Description = "Ref name: " + r.RefNameOrId + ". CSP managed VM (registered to CB-TB)"
		vm.Name = vm.ConnectionName + "-" + vm.IdByCSP
		vm.Name = strings.ToLower(vm.Name)
		vm.Label = "not defined"

		vm.ImageId = "cannot retrieve"
		vm.SpecId = "cannot retrieve"
		vm.SshKeyId = "cannot retrieve"
		vm.SubnetId = "cannot retrieve"
		vm.VNetId = "cannot retrieve"
		vm.SecurityGroupIds = append(vm.SecurityGroupIds, "cannot retrieve")

		req.Vm = append(req.Vm, vm)

		_, err = CreateMcis(nsId, &req, optionFlag)

		registeredStatus = ""
		if err != nil {
			common.CBLog.Error(err)
			registeredStatus = "  [Failed] " + err.Error()
			result.RegisterationOverview.Vm--
			result.RegisterationOverview.Failed++
		}
		result.RegisterationOutputs.IdList = append(result.RegisterationOutputs.IdList, common.StrVM+": "+vm.Name+registeredStatus)
		result.RegisterationOverview.Vm++
	}
	result.ConnectionName = connConfig
	result.ElapsedTime = int(math.Round(time.Now().Sub(startTime).Seconds()))

	fmt.Printf("\n\n%s [Elapsed]%s %d \n\n", connConfig, common.StrVM, int(math.Round(time.Now().Sub(startTime04).Seconds()))) //tmp

	fmt.Printf("\n\n%s [Elapsed]Total %d \n\n", connConfig, int(math.Round(time.Now().Sub(startTime).Seconds())))

	return result, err

}
