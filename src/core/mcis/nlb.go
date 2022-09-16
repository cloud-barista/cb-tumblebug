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
	"os"
	"strconv"
	"strings"
	"time"

	cbstore_utils "github.com/cloud-barista/cb-store/utils"
	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
	validator "github.com/go-playground/validator/v10"
	"github.com/go-resty/resty/v2"
)

// 2022-07-15 https://github.com/cloud-barista/cb-spider/blob/master/cloud-control-manager/cloud-driver/interfaces/resources/NLBHandler.go

// SpiderNLBReqInfoWrapper is a wrapper struct to create JSON body of 'Create NLB request'
type SpiderNLBReqInfoWrapper struct {
	ConnectionName string
	ReqInfo        SpiderNLBReqInfo
}

// 2022-07-15 https://github.com/cloud-barista/cb-spider/blob/a3ee3030e4b956bcb27691203f286265ab3a926e/api-runtime/rest-runtime/CCMRest.go#L1895

// SpiderNLBReqInfo is a struct to create JSON body of 'Create NLB request'
type SpiderNLBReqInfo struct {
	Name    string
	VPCName string
	Type    string // PUBLIC(V) | INTERNAL
	Scope   string // REGION(V) | GLOBAL

	//------ Frontend

	Listener NLBListenerInfo

	//------ Backend

	VMGroup       SpiderNLBVMGroupReq
	HealthChecker NLBHealthCheckerReq
}

type NLBHealthCheckerReq struct {
	Protocol  string // TCP|HTTP|HTTPS
	Port      string // Listener Port or 1-65535
	Interval  string // secs, Interval time between health checks.
	Timeout   string // secs, Waiting time to decide an unhealthy VM when no response.
	Threshold string // num, The number of continuous health checks to change the VM status.
}

type SpiderNLBVMGroupReq struct {
	Protocol string // TCP|HTTP|HTTPS
	Port     string // Listener Port or 1-65535
	VMs      []string
}

// SpiderNLBAddRemoveVMReqInfoWrapper is a wrapper struct to create JSON body of 'Add/Remove VMs to/from NLB' request
type SpiderNLBAddRemoveVMReqInfoWrapper struct {
	ConnectionName string
	ReqInfo        SpiderNLBVMGroupReq
}

// SpiderNLBInfo is a struct to handle NLB information from the CB-Spider's REST API response
type SpiderNLBInfo struct {
	IId    common.IID // {NameId, SystemId}
	VpcIID common.IID // {NameId, SystemId}

	Type  string // PUBLIC(V) | INTERNAL
	Scope string // REGION(V) | GLOBAL

	//------ Frontend
	Listener NLBListenerInfo

	//------ Backend
	VMGroup       NLBVMGroupInfo
	HealthChecker NLBHealthCheckerInfo

	CreatedTime  time.Time
	KeyValueList []common.KeyValue
}

// NLBListenerInfo is a struct to handle NLB Listener information from the CB-Spider's REST API response
type NLBListenerInfo struct { // for both Spider and Tumblebug
	Protocol string `json:"protocol" example:"TCP"` // TCP|UDP
	IP       string `json:"ip" example:""`          // Auto Generated and attached
	Port     string `json:"port" example:"22"`      // 1-65535
	DNSName  string `json:"dnsName" example:""`     // Optional, Auto Generated and attached

	CspID        string            `json:"cspID"` // Optional, May be Used by Driver.
	KeyValueList []common.KeyValue `json:"keyValueList"`
}

type NLBVMGroupInfo struct { // Spider
	Protocol string // TCP|UDP|HTTP|HTTPS
	Port     string // 1-65535
	VMs      *[]common.IID

	CspID        string            `json:"cspID"` // Optional, May be Used by Driver.
	KeyValueList []common.KeyValue `json:"keyValueList"`
}

type NLBHealthCheckerInfo struct {
	Protocol  string `json:"protocol" example:"TCP"` // TCP|HTTP|HTTPS
	Port      string `json:"port" example:"22"`      // Listener Port or 1-65535
	Interval  int    `json:"interval" example:"10"`  // secs, Interval time between health checks.
	Timeout   int    `json:"timeout" example:"10"`   // secs, Waiting time to decide an unhealthy VM when no response.
	Threshold int    `json:"threshold" example:"3"`  // num, The number of continuous health checks to change the VM status.

	CspID        string            `json:"cspID"` // Optional, May be Used by Driver.
	KeyValueList []common.KeyValue `json:"keyValueList"`
}

type SpiderHealthInfo struct {
	AllVMs       *[]common.IID
	HealthyVMs   *[]common.IID
	UnHealthyVMs *[]common.IID
}

type TBNLBTargetGroup struct {
	Protocol string   `json:"protocol" example:"TCP"` // TCP|HTTP|HTTPS
	Port     string   `json:"port" example:"22"`      // Listener Port or 1-65535
	MCIS     string   `json:"mcis" example:"mc"`
	VMs      []string `json:"vms"`

	CspID        string // Optional, May be Used by Driver.
	KeyValueList []common.KeyValue
}

// TbNLBReq is a struct to handle 'Create nlb' request toward CB-Tumblebug.
type TbNLBReq struct { // Tumblebug
	Name           string `json:"name" validate:"required" example:"mc"`
	ConnectionName string `json:"connectionName" validate:"required" example:"aws-ap-northeast-2"`
	VNetId         string `json:"vNetId" validate:"required" example:"ns01-systemdefault-aws-ap-northeast-2"`
	Description    string `json:"description"`
	CspNLBId       string `json:"cspNLBId"`

	Type  string `json:"type" validate:"required" enums:"PUBLIC,INTERNAL" example:"PUBLIC"` // PUBLIC(V) | INTERNAL
	Scope string `json:"scope" validate:"required" enums:"REGION,GLOBAL" example:"REGION"`  // REGION(V) | GLOBAL

	//------ Frontend

	Listener NLBListenerInfo `json:"listener" validate:"required"`

	//------ Backend

	TargetGroup   TBNLBTargetGroup    `json:"targetGroup"`
	HealthChecker NLBHealthCheckerReq `json:"healthChecker" validate:"required"`
}

// TbNLBReqStructLevelValidation is a function to validate 'TbNLBReq' object.
func TbNLBReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(TbNLBReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

// TbNLBInfo is a struct that represents TB nlb object.
type TbNLBInfo struct { // Tumblebug
	Id             string `json:"id"`
	Name           string `json:"name"`
	ConnectionName string `json:"connectionName"`

	Type  string // PUBLIC(V) | INTERNAL
	Scope string // REGION(V) | GLOBAL

	//------ Frontend

	Listener NLBListenerInfo

	//------ Backend

	TargetGroup   TBNLBTargetGroup `json:"targetGroup"`
	HealthChecker NLBHealthCheckerInfo

	CreatedTime time.Time

	Description          string            `json:"description"`
	CspNLBId             string            `json:"cspNLBId"`
	CspNLBName           string            `json:"cspNLBName"`
	Status               string            `json:"status"`
	KeyValueList         []common.KeyValue `json:"keyValueList"`
	AssociatedObjectList []string          `json:"associatedObjectList"`
	IsAutoGenerated      bool              `json:"isAutoGenerated"`

	// SystemLabel is for describing the MCIR in a keyword (any string can be used) for special System purpose
	SystemLabel string `json:"systemLabel" example:"Managed by CB-Tumblebug" default:""`

	// Disabled for now
	//Region         string `json:"region"`
	//ResourceGroupName string `json:"resourceGroupName"`
}

// TbNLBAddRemoveVMReq is a struct to handle 'Add/Remove VMs to/from NLB' request toward CB-Tumblebug.
type TbNLBAddRemoveVMReq struct { // Tumblebug
	TargetGroup TBNLBTargetGroup `json:"targetGroup"`
}

// CreateNLB accepts nlb creation request, creates and returns an TB nlb object
func CreateNLB(nsId string, u *TbNLBReq, option string) (TbNLBInfo, error) {
	fmt.Println("=========================== CreateNLB")

	err := common.CheckString(nsId)
	if err != nil {
		temp := TbNLBInfo{}
		common.CBLog.Error(err)
		return temp, err
	}

	err = validate.Struct(u)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			fmt.Println(err)
			temp := TbNLBInfo{}
			return temp, err
		}

		temp := TbNLBInfo{}
		return temp, err
	}

	check, err := CheckNLB(nsId, u.Name)

	if check {
		temp := TbNLBInfo{}
		err := fmt.Errorf("The nlb " + u.Name + " already exists.")
		return temp, err
	}

	if err != nil {
		temp := TbNLBInfo{}
		err := fmt.Errorf("Failed to check the existence of the nlb " + u.Name + ".")
		return temp, err
	}

	vNetInfo := mcir.TbVNetInfo{}
	tempInterface, err := mcir.GetResource(nsId, common.StrVNet, u.VNetId)
	if err != nil {
		err := fmt.Errorf("Failed to get the TbVNetInfo " + u.VNetId + ".")
		return TbNLBInfo{}, err
	}
	err = common.CopySrcToDest(&tempInterface, &vNetInfo)
	if err != nil {
		err := fmt.Errorf("Failed to get the TbVNetInfo-CopySrcToDest() " + u.VNetId + ".")
		return TbNLBInfo{}, err
	}

	tempReq := SpiderNLBReqInfoWrapper{}
	tempReq.ConnectionName = u.ConnectionName
	tempReq.ReqInfo.Name = fmt.Sprintf("%s-%s", nsId, u.Name)
	tempReq.ReqInfo.VPCName = vNetInfo.CspVNetName
	tempReq.ReqInfo.Type = u.Type
	tempReq.ReqInfo.Scope = u.Scope

	tempReq.ReqInfo.Listener = u.Listener

	tempReq.ReqInfo.HealthChecker = u.HealthChecker

	tempReq.ReqInfo.VMGroup.Port = u.TargetGroup.Port
	tempReq.ReqInfo.VMGroup.Protocol = u.TargetGroup.Protocol

	for _, v := range u.TargetGroup.VMs {
		vm, err := GetVmObject(nsId, u.TargetGroup.MCIS, v)
		if err != nil {
			common.CBLog.Error(err)
			return TbNLBInfo{}, err
		}
		// fmt.Println("vm:")                             // for debug
		// payload, _ := json.MarshalIndent(vm, "", "  ") // for debug
		// fmt.Print(string(payload))                     // for debug
		tempReq.ReqInfo.VMGroup.VMs = append(tempReq.ReqInfo.VMGroup.VMs, vm.CspViewVmDetail.IId.NameId)
	}

	// fmt.Printf("u.TargetGroup.VMs: %s \n", u.TargetGroup.VMs)                             // for debug
	// fmt.Printf("tempReq.ReqInfo.VMGroup.VMs: %s \n", tempReq.ReqInfo.VMGroup.VMs) // for debug
	/*
		for _, v := range u.VMIDList {
			mcisId_vmId := strings.Split(v, "/")
			if len(mcisId_vmId) != 2 {
				err := fmt.Errorf("Cannot retrieve VM info: " + v)
				common.CBLog.Error(err)
				return TbNLBInfo{}, err
			}

			vm, err := mcis.GetVmObject(nsId, mcisId_vmId[0], mcisId_vmId[1])
			if err != nil {
				common.CBLog.Error(err)
				return TbNLBInfo{}, err
			}

			tempReq.ReqInfo.VMGroup = append(tempReq.ReqInfo.VMGroup, vm.IdByCSP)
		}
	*/

	var tempSpiderNLBInfo *SpiderNLBInfo

	if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

		client := resty.New().SetCloseConnection(true)
		client.SetAllowGetMethodPayload(true)

		// fmt.Println("tempReq:")                             // for debug
		// payload, _ := json.MarshalIndent(tempReq, "", "  ") // for debug
		// fmt.Print(string(payload))                          // for debug

		req := client.R().
			SetHeader("Content-Type", "application/json").
			SetBody(tempReq).
			SetResult(&SpiderNLBInfo{}) // or SetResult(AuthSuccess{}).
			//SetError(&AuthError{}).       // or SetError(AuthError{}).

		var resp *resty.Response
		var err error

		var url string
		if option == "register" && u.CspNLBId == "" {
			url = fmt.Sprintf("%s/nlb/%s", common.SpiderRestUrl, u.Name)
			resp, err = req.Get(url)
		} else if option == "register" && u.CspNLBId != "" {
			url = fmt.Sprintf("%s/regnlb", common.SpiderRestUrl)
			resp, err = req.Post(url)
		} else { // option != "register"
			url = fmt.Sprintf("%s/nlb", common.SpiderRestUrl)
			resp, err = req.Post(url)
		}

		if err != nil {
			common.CBLog.Error(err)
			content := TbNLBInfo{}
			err := fmt.Errorf("an error occurred while requesting to CB-Spider")
			return content, err
		}

		fmt.Println("HTTP Status code: " + strconv.Itoa(resp.StatusCode()))
		switch {
		case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
			err := fmt.Errorf(string(resp.Body()))
			common.CBLog.Error(err)
			content := TbNLBInfo{}
			return content, err
		}

		tempSpiderNLBInfo = resp.Result().(*SpiderNLBInfo)

	}
	/*
		else {

			// Set CCM API
			ccm := api.NewCloudResourceHandler()
			err := ccm.SetConfigPath(os.Getenv("CBTUMBLEBUG_ROOT") + "/conf/grpc_conf.yaml")
			if err != nil {
				common.CBLog.Error("ccm failed to set config : ", err)
				return TbNLBInfo{}, err
			}
			err = ccm.Open()
			if err != nil {
				common.CBLog.Error("ccm api open failed : ", err)
				return TbNLBInfo{}, err
			}
			defer ccm.Close()

			payload, _ := json.MarshalIndent(tempReq, "", "  ")

			var result string

			if option == "register" {
				result, err = ccm.CreateNLB(string(payload))
			} else {
				result, err = ccm.GetNLB(string(payload))
			}

			if err != nil {
				common.CBLog.Error(err)
				return TbNLBInfo{}, err
			}

			tempSpiderNLBInfo = &SpiderNLBInfo{}
			err = json.Unmarshal([]byte(result), &tempSpiderNLBInfo)
			if err != nil {
				common.CBLog.Error(err)
				return TbNLBInfo{}, err
			}

		}
	*/

	content := TbNLBInfo{}
	//content.Id = common.GenUid()
	content.Id = u.Name
	content.Name = u.Name
	content.ConnectionName = u.ConnectionName
	content.Type = tempSpiderNLBInfo.Type
	content.Scope = tempSpiderNLBInfo.Scope
	content.Listener = tempSpiderNLBInfo.Listener
	content.HealthChecker = tempSpiderNLBInfo.HealthChecker
	content.CspNLBId = tempSpiderNLBInfo.IId.SystemId
	content.CspNLBName = tempSpiderNLBInfo.IId.NameId
	content.Description = u.Description
	content.KeyValueList = tempSpiderNLBInfo.KeyValueList
	content.AssociatedObjectList = []string{}

	content.TargetGroup.Port = tempSpiderNLBInfo.VMGroup.Port
	content.TargetGroup.Protocol = tempSpiderNLBInfo.VMGroup.Protocol
	content.TargetGroup.MCIS = u.TargetGroup.MCIS
	content.TargetGroup.VMs = u.TargetGroup.VMs
	content.TargetGroup.CspID = u.TargetGroup.CspID
	content.TargetGroup.KeyValueList = u.TargetGroup.KeyValueList

	if option == "register" && u.CspNLBId == "" {
		content.SystemLabel = "Registered from CB-Spider resource"
	} else if option == "register" && u.CspNLBId != "" {
		content.SystemLabel = "Registered from CSP resource"
	}

	// cb-store
	// Key := common.GenResourceKey(nsId, common.StrNLB, content.Id)
	Key := GenNLBKey(nsId, content.Id)
	Val, _ := json.Marshal(content)

	err = common.CBStore.Put(Key, string(Val))
	if err != nil {
		common.CBLog.Error(err)
		return content, err
	}

	keyValue, err := common.CBStore.Get(Key)
	if err != nil {
		common.CBLog.Error(err)
		err = fmt.Errorf("In CreateNLB(); CBStore.Get() returned an error.")
		common.CBLog.Error(err)
		// return nil, err
	}

	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")

	result := TbNLBInfo{}
	err = json.Unmarshal([]byte(keyValue.Value), &result)
	if err != nil {
		common.CBLog.Error(err)
	}
	return result, nil
}

// GetNLB returns the requested TB NLB object
func GetNLB(nsId string, resourceId string) (TbNLBInfo, error) {
	res := TbNLBInfo{}

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return res, err
	}

	err = common.CheckString(resourceId)
	if err != nil {
		common.CBLog.Error(err)
		return res, err
	}
	check, err := CheckNLB(nsId, resourceId)
	if err != nil {
		common.CBLog.Error(err)
		return res, err
	}

	if !check {
		errString := "The NLB " + resourceId + " does not exist."
		err := fmt.Errorf(errString)
		return res, err
	}

	fmt.Println("[Get NLB] " + resourceId)

	// key := common.GenResourceKey(nsId, resourceType, resourceId)
	key := GenNLBKey(nsId, resourceId)

	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
		return res, err
	}
	if keyValue != nil {

		err = json.Unmarshal([]byte(keyValue.Value), &res)
		if err != nil {
			common.CBLog.Error(err)
			return res, err
		}
		return res, nil
	}
	errString := "Cannot get the NLB " + resourceId + "."
	err = fmt.Errorf(errString)
	return res, err
}

// CheckNLB returns the existence of the TB NLB object in bool form.
func CheckNLB(nsId string, resourceId string) (bool, error) {

	// Check parameters' emptiness
	if nsId == "" {
		err := fmt.Errorf("CheckResource failed; nsId given is null.")
		return false, err
	} else if resourceId == "" {
		err := fmt.Errorf("CheckResource failed; resourceId given is null.")
		return false, err
	}

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return false, err
	}

	err = common.CheckString(resourceId)
	if err != nil {
		common.CBLog.Error(err)
		return false, err
	}

	fmt.Println("[Check NLB] " + resourceId)

	// key := common.GenResourceKey(nsId, resourceType, resourceId)
	key := GenNLBKey(nsId, resourceId)

	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
		return false, err
	}
	if keyValue != nil {
		return true, nil
	}
	return false, nil

}

// GenNLBKey is func to generate a key from NLB id
func GenNLBKey(nsId string, resourceId string) string {
	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return "/invalidKey"
	}

	err = common.CheckString(resourceId)
	if err != nil {
		common.CBLog.Error(err)
		return "/invalidKey"
	}

	return fmt.Sprintf("/ns/%s/nlb/%s", nsId, resourceId)
}

// ListNLBId returns the list of TB NLB object IDs of given nsId
func ListNLBId(nsId string) ([]string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	fmt.Println("[ListNLBId] ns: " + nsId)
	key := "/ns/" + nsId + "/"
	fmt.Println(key)

	keyValue, err := common.CBStore.GetList(key, true)

	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	/* if keyValue == nil, then for-loop below will not be executed, and the empty array will be returned in `resourceList` placeholder.
	if keyValue == nil {
		err = fmt.Errorf("ListResourceId(); %s is empty.", key)
		common.CBLog.Error(err)
		return nil, err
	}
	*/

	var resourceList []string
	for _, v := range keyValue {
		trimmedString := strings.TrimPrefix(v.Key, (key + "nlb/"))
		// prevent malformed key (if key for resource id includes '/', the key does not represent resource ID)
		if !strings.Contains(trimmedString, "/") {
			resourceList = append(resourceList, trimmedString)
		}
	}

	return resourceList, nil

}

// ListNLB returns the list of TB NLB objects of given nsId
func ListNLB(nsId string, filterKey string, filterVal string) (interface{}, error) {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	fmt.Println("[Get] NLB list")
	key := "/ns/" + nsId + "/nlb"
	fmt.Println(key)

	keyValue, err := common.CBStore.GetList(key, true)
	keyValue = cbstore_utils.GetChildList(keyValue, key)

	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}
	if keyValue != nil {
		res := []TbNLBInfo{}
		for _, v := range keyValue {

			tempObj := TbNLBInfo{}
			err = json.Unmarshal([]byte(v.Value), &tempObj)
			if err != nil {
				common.CBLog.Error(err)
				return nil, err
			}
			// Check the JSON body inclues both filterKey and filterVal strings. (assume key and value)
			if filterKey != "" {
				// If not inclues both, do not append current item to the list result.
				itemValueForCompare := strings.ToLower(v.Value)
				if !(strings.Contains(itemValueForCompare, strings.ToLower(filterKey)) && strings.Contains(itemValueForCompare, strings.ToLower(filterVal))) {
					continue
				}
			}
			res = append(res, tempObj)
		}
		return res, nil

	} else { //return empty object according to resourceType
		res := []TbNLBInfo{}
		return res, nil

	}

	err = fmt.Errorf("Some exceptional case happened. Please check the references of " + common.GetFuncName())
	return nil, err // if interface{} == nil, make err be returned. Should not come this part if there is no err.
}

// DelNLB deletes the TB NLB object
func DelNLB(nsId string, resourceId string, forceFlag string) error {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	err = common.CheckString(resourceId)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}
	check, err := CheckNLB(nsId, resourceId)

	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	if !check {
		errString := "The NLB " + resourceId + " does not exist."
		err := fmt.Errorf(errString)
		return err
	}

	key := GenNLBKey(nsId, resourceId)
	fmt.Println("key: " + key)

	keyValue, _ := common.CBStore.Get(key)
	// In CheckResource() above, calling 'CBStore.Get()' and checking err parts exist.
	// So, in here, we don't need to check whether keyValue == nil or err != nil.

	// Deleting NLB should be possible, even if backend VMs still exist.
	// So here 'associated object' codes are commented.
	/*
		associatedList, _ := GetAssociatedObjectList(nsId, resourceType, resourceId)
		if len(associatedList) == 0 {
			// continue
		} else {
			errString := " [Failed]" + " Associated with [" + strings.Join(associatedList[:], ", ") + "]"
			err := fmt.Errorf(errString)
			common.CBLog.Error(err)
			return err
		}
	*/

	//cspType := common.GetResourcesCspType(nsId, resourceType, resourceId)

	// NLB has no childResources, so below line is commented.
	// var childResources interface{}

	// Disable gRPC calling for NLB
	// if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

	var url string

	// Create Req body
	type JsonTemplate struct {
		ConnectionName string
	}
	tempReq := JsonTemplate{}

	temp := TbNLBInfo{}
	err = json.Unmarshal([]byte(keyValue.Value), &temp)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}
	tempReq.ConnectionName = temp.ConnectionName
	url = common.SpiderRestUrl + "/nlb/" + temp.CspNLBName

	fmt.Println("url: " + url)

	client := resty.New().SetCloseConnection(true)

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(tempReq).
		//SetResult(&SpiderSpecInfo{}). // or SetResult(AuthSuccess{}).
		//SetError(&AuthError{}).       // or SetError(AuthError{}).
		Delete(url)

	if err != nil {
		common.CBLog.Error(err)
		err := fmt.Errorf("an error occurred while requesting to CB-Spider")
		return err
	}

	fmt.Println("HTTP Status code: " + strconv.Itoa(resp.StatusCode()))
	switch {
	case forceFlag == "true":
		url += "?force=true"
		fmt.Println("forceFlag == true; url: " + url)

		_, err := client.R().
			SetHeader("Content-Type", "application/json").
			SetBody(tempReq).
			//SetResult(&SpiderSpecInfo{}). // or SetResult(AuthSuccess{}).
			//SetError(&AuthError{}).       // or SetError(AuthError{}).
			Delete(url)

		if err != nil {
			common.CBLog.Error(err)
			err := fmt.Errorf("an error occurred while requesting to CB-Spider")
			return err
		}

	case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
		err := fmt.Errorf(string(resp.Body()))
		common.CBLog.Error(err)
		return err
	default:

	}

	// Disable gRPC calling for NLB
	/*
		} else {

			// Set CCM gRPC API
			ccm := api.NewCloudResourceHandler()
			err := ccm.SetConfigPath(os.Getenv("CBTUMBLEBUG_ROOT") + "/conf/grpc_conf.yaml")
			if err != nil {
				common.CBLog.Error("ccm failed to set config : ", err)
				return err
			}
			err = ccm.Open()
			if err != nil {
				common.CBLog.Error("ccm api open failed : ", err)
				return err
			}
			defer ccm.Close()

			temp := TbNLBInfo{}
			err = json.Unmarshal([]byte(keyValue.Value), &temp)
			if err != nil {
				common.CBLog.Error(err)
				return err
			}

			_, err = ccm.DeleteNLBByParam(temp.ConnectionName, temp.Name, forceFlag)
			if err != nil {
				common.CBLog.Error(err)
				return err
			}

			// err = common.CBStore.Delete(key)
			// if err != nil {
			// 	common.CBLog.Error(err)
			// 	return err
			// }
			// return nil

		}
	*/

	err = common.CBStore.Delete(key)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}
	return nil
}

// DelAllNLB deletes all TB NLB object of given nsId
func DelAllNLB(nsId string, subString string, forceFlag string) (common.IdList, error) {

	deletedResources := common.IdList{}
	deleteStatus := ""

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return deletedResources, err
	}

	resourceIdList, err := ListNLBId(nsId)
	if err != nil {
		return deletedResources, err
	}

	if len(resourceIdList) == 0 {
		errString := "There is no NLB in " + nsId
		err := fmt.Errorf(errString)
		common.CBLog.Error(err)
		return deletedResources, err
	}

	for _, v := range resourceIdList {
		// if subString is provided, check the resourceId contains the subString.
		if subString == "" || strings.Contains(v, subString) {
			deleteStatus = ""

			err := DelNLB(nsId, v, forceFlag)

			if err != nil {
				deleteStatus = err.Error()
			} else {
				deleteStatus = " [Done]"
			}

			deletedResources.IdList = append(deletedResources.IdList, "NLB: "+v+deleteStatus)
		}
	}
	return deletedResources, nil
}

// AddNLBVMs accepts VM addition request, adds VM to NLB, and returns an updated TB NLB object
func AddNLBVMs(nsId string, resourceId string, u *TbNLBAddRemoveVMReq) (TbNLBInfo, error) {
	fmt.Println("=========================== AddNLBVMs")

	err := common.CheckString(nsId)
	if err != nil {
		temp := TbNLBInfo{}
		common.CBLog.Error(err)
		return temp, err
	}

	err = validate.Struct(u)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			fmt.Println(err)
			temp := TbNLBInfo{}
			return temp, err
		}

		temp := TbNLBInfo{}
		return temp, err
	}

	check, err := CheckNLB(nsId, resourceId)

	if !check {
		temp := TbNLBInfo{}
		err := fmt.Errorf("The nlb " + resourceId + " does not exist.")
		return temp, err
	}

	if err != nil {
		temp := TbNLBInfo{}
		err := fmt.Errorf("Failed to check the existence of the nlb " + resourceId + ".")
		return temp, err
	}

	/*
		vNetInfo := mcir.TbVNetInfo{}
		tempInterface, err := mcir.GetResource(nsId, common.StrVNet, u.VNetId)
		if err != nil {
			err := fmt.Errorf("Failed to get the TbVNetInfo " + u.VNetId + ".")
			return TbNLBInfo{}, err
		}
		err = common.CopySrcToDest(&tempInterface, &vNetInfo)
		if err != nil {
			err := fmt.Errorf("Failed to get the TbVNetInfo-CopySrcToDest() " + u.VNetId + ".")
			return TbNLBInfo{}, err
		}
	*/

	nlb, err := GetNLB(nsId, resourceId)
	if err != nil {
		temp := TbNLBInfo{}
		err := fmt.Errorf("Failed to get the nlb object " + resourceId + ".")
		return temp, err
	}

	tempReq := SpiderNLBAddRemoveVMReqInfoWrapper{}
	tempReq.ConnectionName = nlb.ConnectionName

	for _, v := range u.TargetGroup.VMs {
		vm, err := GetVmObject(nsId, u.TargetGroup.MCIS, v)
		if err != nil {
			common.CBLog.Error(err)
			return TbNLBInfo{}, err
		}
		// fmt.Println("vm:")                             // for debug
		// payload, _ := json.MarshalIndent(vm, "", "  ") // for debug
		// fmt.Print(string(payload))                     // for debug
		tempReq.ReqInfo.VMs = append(tempReq.ReqInfo.VMs, vm.CspViewVmDetail.IId.NameId)
	}

	// fmt.Printf("u.TargetGroup.VMs: %s \n", u.TargetGroup.VMs)                             // for debug
	// fmt.Printf("tempReq.ReqInfo.VMGroup.VMs: %s \n", tempReq.ReqInfo.VMGroup.VMs) // for debug
	/*
		for _, v := range u.VMIDList {
			mcisId_vmId := strings.Split(v, "/")
			if len(mcisId_vmId) != 2 {
				err := fmt.Errorf("Cannot retrieve VM info: " + v)
				common.CBLog.Error(err)
				return TbNLBInfo{}, err
			}

			vm, err := mcis.GetVmObject(nsId, mcisId_vmId[0], mcisId_vmId[1])
			if err != nil {
				common.CBLog.Error(err)
				return TbNLBInfo{}, err
			}

			tempReq.ReqInfo.VMGroup = append(tempReq.ReqInfo.VMGroup, vm.IdByCSP)
		}
	*/

	var tempSpiderNLBInfo *SpiderNLBInfo

	if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

		client := resty.New().SetCloseConnection(true)
		client.SetAllowGetMethodPayload(true)

		// fmt.Println("tempReq:")                             // for debug
		// payload, _ := json.MarshalIndent(tempReq, "", "  ") // for debug
		// fmt.Print(string(payload))                          // for debug

		req := client.R().
			SetHeader("Content-Type", "application/json").
			SetBody(tempReq).
			SetResult(&SpiderNLBInfo{}) // or SetResult(AuthSuccess{}).
			//SetError(&AuthError{}).       // or SetError(AuthError{}).

		var resp *resty.Response
		var err error

		var url string
		url = fmt.Sprintf("%s/nlb/%s/vms", common.SpiderRestUrl, nlb.CspNLBName)
		resp, err = req.Post(url)

		if err != nil {
			common.CBLog.Error(err)
			content := TbNLBInfo{}
			err := fmt.Errorf("an error occurred while requesting to CB-Spider")
			return content, err
		}

		fmt.Println("HTTP Status code: " + strconv.Itoa(resp.StatusCode()))
		switch {
		case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
			err := fmt.Errorf(string(resp.Body()))
			common.CBLog.Error(err)
			content := TbNLBInfo{}
			return content, err
		}

		tempSpiderNLBInfo = resp.Result().(*SpiderNLBInfo)

	}
	/*
		else {

			// Set CCM API
			ccm := api.NewCloudResourceHandler()
			err := ccm.SetConfigPath(os.Getenv("CBTUMBLEBUG_ROOT") + "/conf/grpc_conf.yaml")
			if err != nil {
				common.CBLog.Error("ccm failed to set config : ", err)
				return TbNLBInfo{}, err
			}
			err = ccm.Open()
			if err != nil {
				common.CBLog.Error("ccm api open failed : ", err)
				return TbNLBInfo{}, err
			}
			defer ccm.Close()

			payload, _ := json.MarshalIndent(tempReq, "", "  ")

			var result string

			if option == "register" {
				result, err = ccm.CreateNLB(string(payload))
			} else {
				result, err = ccm.GetNLB(string(payload))
			}

			if err != nil {
				common.CBLog.Error(err)
				return TbNLBInfo{}, err
			}

			tempSpiderNLBInfo = &SpiderNLBInfo{}
			err = json.Unmarshal([]byte(result), &tempSpiderNLBInfo)
			if err != nil {
				common.CBLog.Error(err)
				return TbNLBInfo{}, err
			}

		}
	*/

	content := TbNLBInfo{}
	//content.Id = common.GenUid()
	content.Id = nlb.Id
	content.Name = nlb.Name
	content.ConnectionName = nlb.ConnectionName
	content.Type = tempSpiderNLBInfo.Type
	content.Scope = tempSpiderNLBInfo.Scope
	content.Listener = tempSpiderNLBInfo.Listener
	content.HealthChecker = tempSpiderNLBInfo.HealthChecker
	content.CspNLBId = tempSpiderNLBInfo.IId.SystemId
	content.CspNLBName = tempSpiderNLBInfo.IId.NameId
	content.Description = nlb.Description
	content.KeyValueList = tempSpiderNLBInfo.KeyValueList
	content.AssociatedObjectList = []string{}

	content.TargetGroup.Port = tempSpiderNLBInfo.VMGroup.Port
	content.TargetGroup.Protocol = tempSpiderNLBInfo.VMGroup.Protocol
	content.TargetGroup.MCIS = u.TargetGroup.MCIS // What if oldNlb.TargetGroup.MCIS != newNlb.TargetGroup.MCIS
	content.TargetGroup.CspID = u.TargetGroup.CspID
	content.TargetGroup.KeyValueList = u.TargetGroup.KeyValueList

	// content.TargetGroup.VMs = u.TargetGroup.VMs
	content.TargetGroup.VMs = append(content.TargetGroup.VMs, nlb.TargetGroup.VMs...)
	content.TargetGroup.VMs = append(content.TargetGroup.VMs, u.TargetGroup.VMs...)

	// cb-store
	// Key := common.GenResourceKey(nsId, common.StrNLB, content.Id)
	Key := GenNLBKey(nsId, content.Id)
	Val, _ := json.Marshal(content)

	err = common.CBStore.Put(Key, string(Val))
	if err != nil {
		common.CBLog.Error(err)
		return content, err
	}

	keyValue, err := common.CBStore.Get(Key)
	if err != nil {
		common.CBLog.Error(err)
		err = fmt.Errorf("In CreateNLB(); CBStore.Get() returned an error.")
		common.CBLog.Error(err)
		// return nil, err
	}

	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")

	result := TbNLBInfo{}
	err = json.Unmarshal([]byte(keyValue.Value), &result)
	if err != nil {
		common.CBLog.Error(err)
	}
	return result, nil
}

// RemoveNLBVMs accepts VM removal request, removes VMs from NLB, and returns an error if occurs.
func RemoveNLBVMs(nsId string, resourceId string, u *TbNLBAddRemoveVMReq) error {
	fmt.Println("=========================== RemoveNLBVMs")

	err := common.CheckString(nsId)
	if err != nil {
		// temp := TbNLBInfo{}
		common.CBLog.Error(err)
		return err
	}

	err = validate.Struct(u)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			fmt.Println(err)
			// temp := TbNLBInfo{}
			return err
		}

		// temp := TbNLBInfo{}
		return err
	}

	check, err := CheckNLB(nsId, resourceId)

	if !check {
		// temp := TbNLBInfo{}
		err := fmt.Errorf("The nlb " + resourceId + " does not exist.")
		return err
	}

	if err != nil {
		// temp := TbNLBInfo{}
		err := fmt.Errorf("Failed to check the existence of the nlb " + resourceId + ".")
		return err
	}

	/*
		vNetInfo := mcir.TbVNetInfo{}
		tempInterface, err := mcir.GetResource(nsId, common.StrVNet, u.VNetId)
		if err != nil {
			err := fmt.Errorf("Failed to get the TbVNetInfo " + u.VNetId + ".")
			return TbNLBInfo{}, err
		}
		err = common.CopySrcToDest(&tempInterface, &vNetInfo)
		if err != nil {
			err := fmt.Errorf("Failed to get the TbVNetInfo-CopySrcToDest() " + u.VNetId + ".")
			return TbNLBInfo{}, err
		}
	*/

	nlb, err := GetNLB(nsId, resourceId)
	if err != nil {
		// temp := TbNLBInfo{}
		err := fmt.Errorf("Failed to get the nlb object " + resourceId + ".")
		return err
	}

	tempReq := SpiderNLBAddRemoveVMReqInfoWrapper{}
	tempReq.ConnectionName = nlb.ConnectionName

	// fmt.Printf("u.TargetGroup.VMs: %s \n", u.TargetGroup.VMs) // for debug

	for _, v := range u.TargetGroup.VMs {
		vm, err := GetVmObject(nsId, u.TargetGroup.MCIS, v)
		if err != nil {
			common.CBLog.Error(err)
			return err
		}
		// fmt.Println("vm:")                             // for debug
		// payload, _ := json.MarshalIndent(vm, "", "  ") // for debug
		// fmt.Print(string(payload))                     // for debug
		if vm.CspViewVmDetail.IId.NameId == "" {
			fmt.Printf("Failed to get %s; skipping;", v)
		} else {
			tempReq.ReqInfo.VMs = append(tempReq.ReqInfo.VMs, vm.CspViewVmDetail.IId.NameId)
		}
	}

	// fmt.Printf("tempReq.ReqInfo.VMGroup.VMs: %s \n", tempReq.ReqInfo.VMs) // for debug
	/*
		for _, v := range u.VMIDList {
			mcisId_vmId := strings.Split(v, "/")
			if len(mcisId_vmId) != 2 {
				err := fmt.Errorf("Cannot retrieve VM info: " + v)
				common.CBLog.Error(err)
				return TbNLBInfo{}, err
			}

			vm, err := mcis.GetVmObject(nsId, mcisId_vmId[0], mcisId_vmId[1])
			if err != nil {
				common.CBLog.Error(err)
				return TbNLBInfo{}, err
			}

			tempReq.ReqInfo.VMGroup = append(tempReq.ReqInfo.VMGroup, vm.IdByCSP)
		}
	*/

	// var tempSpiderNLBInfo *SpiderNLBInfo

	if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

		client := resty.New().SetCloseConnection(true)
		client.SetAllowGetMethodPayload(true)

		// fmt.Println("tempReq:")                             // for debug
		// payload, _ := json.MarshalIndent(tempReq, "", "  ") // for debug
		// fmt.Print(string(payload))                          // for debug

		req := client.R().
			SetHeader("Content-Type", "application/json").
			SetBody(tempReq)
			// SetResult(&SpiderNLBInfo{}) // or SetResult(AuthSuccess{}).
			//SetError(&AuthError{}).       // or SetError(AuthError{}).

		var resp *resty.Response
		var err error

		var url string
		url = fmt.Sprintf("%s/nlb/%s/vms", common.SpiderRestUrl, nlb.CspNLBName)
		resp, err = req.Delete(url)

		if err != nil {
			common.CBLog.Error(err)
			// content := TbNLBInfo{}
			err := fmt.Errorf("an error occurred while requesting to CB-Spider")
			return err
		}

		fmt.Println("HTTP Status code: " + strconv.Itoa(resp.StatusCode()))
		switch {
		case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
			err := fmt.Errorf(string(resp.Body()))
			common.CBLog.Error(err)
			// content := TbNLBInfo{}
			return err
		}

		// result := resp.Result().(bool)

	}
	/*
		else {

			// Set CCM API
			ccm := api.NewCloudResourceHandler()
			err := ccm.SetConfigPath(os.Getenv("CBTUMBLEBUG_ROOT") + "/conf/grpc_conf.yaml")
			if err != nil {
				common.CBLog.Error("ccm failed to set config : ", err)
				return TbNLBInfo{}, err
			}
			err = ccm.Open()
			if err != nil {
				common.CBLog.Error("ccm api open failed : ", err)
				return TbNLBInfo{}, err
			}
			defer ccm.Close()

			payload, _ := json.MarshalIndent(tempReq, "", "  ")

			var result string

			if option == "register" {
				result, err = ccm.CreateNLB(string(payload))
			} else {
				result, err = ccm.GetNLB(string(payload))
			}

			if err != nil {
				common.CBLog.Error(err)
				return TbNLBInfo{}, err
			}

			tempSpiderNLBInfo = &SpiderNLBInfo{}
			err = json.Unmarshal([]byte(result), &tempSpiderNLBInfo)
			if err != nil {
				common.CBLog.Error(err)
				return TbNLBInfo{}, err
			}

		}
	*/

	oldVMList := nlb.TargetGroup.VMs
	for _, vmToDelete := range u.TargetGroup.VMs {
		oldVMList = remove(oldVMList, vmToDelete)
	}
	newVMList := oldVMList

	/*
		content := TbNLBInfo{}
		//content.Id = common.GenUid()
		content.Id = nlb.Id
		content.Name = nlb.Name
		content.ConnectionName = nlb.ConnectionName
		content.Type = tempSpiderNLBInfo.Type
		content.Scope = tempSpiderNLBInfo.Scope
		content.Listener = tempSpiderNLBInfo.Listener
		content.HealthChecker = tempSpiderNLBInfo.HealthChecker
		content.CspNLBId = tempSpiderNLBInfo.IId.SystemId
		content.CspNLBName = tempSpiderNLBInfo.IId.NameId
		content.Description = nlb.Description
		content.KeyValueList = tempSpiderNLBInfo.KeyValueList
		content.AssociatedObjectList = []string{}

		content.TargetGroup.Port = tempSpiderNLBInfo.VMGroup.Port
		content.TargetGroup.Protocol = tempSpiderNLBInfo.VMGroup.Protocol
		content.TargetGroup.MCIS = u.TargetGroup.MCIS // What if oldNlb.TargetGroup.MCIS != newNlb.TargetGroup.MCIS
		content.TargetGroup.CspID = u.TargetGroup.CspID
		content.TargetGroup.KeyValueList = u.TargetGroup.KeyValueList

		// content.TargetGroup.VMs = u.TargetGroup.VMs
		content.TargetGroup.VMs = append(content.TargetGroup.VMs, nlb.TargetGroup.VMs...)
		content.TargetGroup.VMs = append(content.TargetGroup.VMs, u.TargetGroup.VMs...)
	*/

	nlb.TargetGroup.VMs = newVMList

	// cb-store
	// Key := common.GenResourceKey(nsId, common.StrNLB, content.Id)
	Key := GenNLBKey(nsId, nlb.Id)
	Val, _ := json.Marshal(nlb)

	err = common.CBStore.Put(Key, string(Val))
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	keyValue, err := common.CBStore.Get(Key)
	if err != nil {
		common.CBLog.Error(err)
		err = fmt.Errorf("In CreateNLB(); CBStore.Get() returned an error.")
		common.CBLog.Error(err)
		// return nil, err
	}

	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")

	/*
		result := TbNLBInfo{}
		err = json.Unmarshal([]byte(keyValue.Value), &result)
		if err != nil {
			common.CBLog.Error(err)
		}
	*/
	return nil
}

func remove(l []string, item string) []string {
	for i, other := range l {
		if other == item {
			return append(l[:i], l[i+1:]...)
		}
	}
	return l
}
