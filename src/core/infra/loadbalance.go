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
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvutil"
	validator "github.com/go-playground/validator/v10"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
)

const nlbPostfix = "-nlb"

// TbNLBReqStructLevelValidation is a function to validate 'model.TbNLBReq' object.
func TbNLBReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(model.TbNLBReq)

	err := common.CheckString(u.TargetGroup.SubGroupId)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.TargetGroup.SubGroupId, "name", "Name", err.Error(), "")
	}
}

// CreateMcSwNlb func create a special purpose MCI for NLB and depoly and setting SW NLB
func CreateMcSwNlb(nsId string, mciId string, req *model.TbNLBReq, option string) (model.McNlbInfo, error) {
	log.Info().Msg("CreateMcSwNlb")

	emptyObj := model.McNlbInfo{}

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	nlbMciId := mciId + nlbPostfix

	// create a special MCI for (SW)NLB
	labels := map[string]string{
		model.LabelDescription: "MCI for Global-NLB",
	}
	mciDynamicReq := model.TbMciDynamicReq{Name: nlbMciId, InstallMonAgent: "no", Label: labels}

	// get vm requst from cloud_conf.yaml
	subGroupName := "nlb"
	// default specId
	specId := common.RuntimeConf.Nlbsw.NlbMciSpecId
	imageId := common.RuntimeConf.Nlbsw.NlbMciImageId
	subGroupSize := common.RuntimeConf.Nlbsw.NlbMciSubGroupSize

	// Option can be applied
	// get recommended location and spec for the NLB host based on existing MCI
	recommendSpecReq := model.RecommendSpecReq{}
	recommendSpecReq.Priority.Policy = append(recommendSpecReq.Priority.Policy, model.PriorityCondition{Metric: "latency"})
	recommendSpecReq.Priority.Policy[0].Parameter = append(recommendSpecReq.Priority.Policy[0].Parameter, model.ParameterKeyVal{Key: "latencyMinimal"})

	mci, err := GetMciObject(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}
	for _, vm := range mci.Vm {
		regionOfVm := vm.ConnectionConfig.RegionZoneInfoName
		recommendSpecReq.Priority.Policy[0].Parameter[0].Val = append(recommendSpecReq.Priority.Policy[0].Parameter[0].Val, regionOfVm)
	}

	specList, err := RecommendSpec(model.SystemCommonNs, recommendSpecReq)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}
	if len(specList) != 0 {
		recommendedSpec := specList[0].Id
		specId = recommendedSpec
	}

	subGroupDynamicReq := model.TbCreateSubGroupDynamicReq{Name: subGroupName, SpecId: specId, ImageId: imageId, SubGroupSize: subGroupSize}
	mciDynamicReq.SubGroups = append(mciDynamicReq.SubGroups, subGroupDynamicReq)

	mciInfo, err := CreateMciDynamic("", nsId, &mciDynamicReq, "")
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	// Sleep for 60 seconds for a safe NLB installation.
	fmt.Printf("\n\n[Info] Sleep for 30 seconds for safe NLB installation.\n\n")
	time.Sleep(30 * time.Second)

	// Deploy SW NLB
	var cmds []string
	cmd := common.RuntimeConf.Nlbsw.CommandNlbPrepare
	cmds = append(cmds, cmd)
	cmd = common.RuntimeConf.Nlbsw.CommandNlbDeploy + " " + mciId + " " + common.ToLower(req.Listener.Protocol) + " " + req.Listener.Port
	cmds = append(cmds, cmd)

	// nodeId=${1:-vm}
	// nodeIp=${2:-127.0.0.1}
	// targetPort=${3:-80}
	accessList, err := GetMciAccessInfo(nsId, mciId, "")
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}
	for _, v := range accessList.MciSubGroupAccessInfo {
		for _, k := range v.MciVmAccessInfo {
			cmd = common.RuntimeConf.Nlbsw.CommandNlbAddTargetNode + " " + k.VmId + " " + k.PublicIP + " " + req.TargetGroup.Port
			cmds = append(cmds, cmd)
		}
	}

	cmd = common.RuntimeConf.Nlbsw.CommandNlbApplyConfig
	cmds = append(cmds, cmd)
	output, err := RemoteCommandToMci(nsId, nlbMciId, "", "", "", &model.MciCmdReq{Command: cmds})
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}
	result := model.MciSshCmdResult{Results: output}
	mcNlbInfo := model.McNlbInfo{MciAccessInfo: accessList, McNlbHostInfo: mciInfo, DeploymentLog: result}

	return mcNlbInfo, err

}

// CreateNLB accepts nlb creation request, creates and returns an TB nlb object
func CreateNLB(nsId string, mciId string, u *model.TbNLBReq, option string) (model.TbNLBInfo, error) {
	log.Info().Msg("CreateNLB")

	emptyObj := model.TbNLBInfo{}

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	err = common.CheckString(u.TargetGroup.SubGroupId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	err = validate.Struct(u)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Err(err).Msg("")
			return emptyObj, err
		}

		return emptyObj, err
	}

	check, err := CheckNLB(nsId, mciId, u.TargetGroup.SubGroupId)

	if check {
		err := fmt.Errorf("The nlb " + u.TargetGroup.SubGroupId + " already exists.")
		return emptyObj, err
	}

	if err != nil {
		err := fmt.Errorf("Failed to check the existence of the nlb " + u.TargetGroup.SubGroupId + ".")
		return emptyObj, err
	}

	vmIDs, err := ListVmBySubGroup(nsId, mciId, u.TargetGroup.SubGroupId)
	if err != nil {
		err := fmt.Errorf("Failed to get VMs in the SubGroup " + u.TargetGroup.SubGroupId + ".")
		return emptyObj, err
	}
	if len(vmIDs) == 0 {
		err := fmt.Errorf("There is no VMs in the SubGroup " + u.TargetGroup.SubGroupId + ".")
		return emptyObj, err
	}

	vm, err := GetVmObject(nsId, mciId, vmIDs[0])
	if err != nil {
		err := fmt.Errorf("Failed to get VM " + vmIDs[0] + ".")
		return emptyObj, err
	}

	vNetInfo := model.TbVNetInfo{}
	tempInterface, err := resource.GetResource(nsId, model.StrVNet, vm.VNetId)
	if err != nil {
		err := fmt.Errorf("Failed to get the TbVNetInfo " + vm.VNetId + ".")
		return emptyObj, err
	}
	err = common.CopySrcToDest(&tempInterface, &vNetInfo)
	if err != nil {
		err := fmt.Errorf("Failed to get the TbVNetInfo-CopySrcToDest() " + vm.VNetId + ".")
		return emptyObj, err
	}

	requestBody := model.SpiderNLBReqInfoWrapper{
		ConnectionName: vm.ConnectionName,
		ReqInfo: model.SpiderNLBReqInfo{
			Name:     common.GenUid(),
			VPCName:  vNetInfo.CspResourceName,
			Type:     u.Type,
			Scope:    u.Scope,
			Listener: u.Listener,
			HealthChecker: model.SpiderNLBHealthCheckerReq{
				Protocol:  u.TargetGroup.Protocol,
				Port:      u.TargetGroup.Port,
				Interval:  u.HealthChecker.Interval,
				Timeout:   u.HealthChecker.Timeout,
				Threshold: u.HealthChecker.Threshold,
			},
			VMGroup: model.SpiderNLBSubGroupReq{
				Protocol: u.TargetGroup.Protocol,
				Port:     u.TargetGroup.Port,
			},
		},
	}

	connConfig, err := common.GetConnConfig(vm.ConnectionName)
	if err != nil {
		err := fmt.Errorf("Failed to get the connConfig " + vm.ConnectionName + ".")
		return emptyObj, err
	}

	cloudType := connConfig.ProviderName

	// Convert cloud type to field name (e.g., AWS to Aws, OPENSTACK to Openstack)
	lowercase := strings.ToLower(cloudType)
	fieldName := strings.ToUpper(string(lowercase[0])) + lowercase[1:]

	// Get cloud setting with field name
	cloudSetting := model.CloudSetting{}

	getCloudSetting := func() {
		// cloudSetting := model.CloudSetting{}

		defer func() {
			if err := recover(); err != nil {
				log.Error().Msgf("%v", err)
				cloudSetting = reflect.ValueOf(&common.RuntimeConf.Cloud).Elem().FieldByName("Common").Interface().(model.CloudSetting)
			}
		}()

		cloudSetting = reflect.ValueOf(&common.RuntimeConf.Cloud).Elem().FieldByName(fieldName).Interface().(model.CloudSetting)

		// return cloudSetting
	}

	getCloudSetting()

	// Set nlb health checker info
	valuesFromYaml := model.TbNLBHealthCheckerInfo{}
	valuesFromYaml.Interval, _ = strconv.Atoi(cloudSetting.Nlb.Interval)
	valuesFromYaml.Timeout, _ = strconv.Atoi(cloudSetting.Nlb.Timeout)
	valuesFromYaml.Threshold, _ = strconv.Atoi(cloudSetting.Nlb.Threshold)

	if u.HealthChecker.Interval == "default" || u.HealthChecker.Interval == "" {
		requestBody.ReqInfo.HealthChecker.Interval = strconv.Itoa(valuesFromYaml.Interval)
	}
	if u.HealthChecker.Timeout == "default" || u.HealthChecker.Timeout == "" {
		requestBody.ReqInfo.HealthChecker.Timeout = strconv.Itoa(valuesFromYaml.Timeout)
	}
	if u.HealthChecker.Threshold == "default" || u.HealthChecker.Threshold == "" {
		requestBody.ReqInfo.HealthChecker.Threshold = strconv.Itoa(valuesFromYaml.Threshold)
	}

	for _, v := range vmIDs {
		vm, err := GetVmObject(nsId, mciId, v)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyObj, err
		}

		requestBody.ReqInfo.VMGroup.VMs = append(requestBody.ReqInfo.VMGroup.VMs, vm.CspResourceName)
	}

	var tempSpiderNLBInfo *model.SpiderNLBInfo

	client := resty.New().SetCloseConnection(true)
	client.SetAllowGetMethodPayload(true)

	req := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(requestBody).
		SetResult(&model.SpiderNLBInfo{}) // or SetResult(AuthSuccess{}).
		//SetError(&AuthError{}).       // or SetError(AuthError{}).

	var resp *resty.Response

	var url string
	if option == "register" && u.CspResourceId == "" {
		url = fmt.Sprintf("%s/nlb/%s", model.SpiderRestUrl, u.TargetGroup.SubGroupId)
		resp, err = req.Get(url)
	} else if option == "register" && u.CspResourceId != "" {
		url = fmt.Sprintf("%s/regnlb", model.SpiderRestUrl)
		resp, err = req.Post(url)
	} else { // option != "register"
		url = fmt.Sprintf("%s/nlb", model.SpiderRestUrl)
		resp, err = req.Post(url)
	}

	if err != nil {
		log.Error().Err(err).Msg("")
		err := fmt.Errorf("an error occurred while requesting to CB-Spider")
		return emptyObj, err
	}

	// fmt.Println("HTTP Status code: " + strconv.Itoa(resp.StatusCode()))
	switch {
	case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
		err := fmt.Errorf(string(resp.Body()))
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	tempSpiderNLBInfo = resp.Result().(*model.SpiderNLBInfo)
	location := connConfig.RegionDetail.Location

	content := model.TbNLBInfo{
		Id:             u.TargetGroup.SubGroupId,
		Name:           u.TargetGroup.SubGroupId,
		ConnectionName: vm.ConnectionName,
		Type:           tempSpiderNLBInfo.Type,
		Scope:          tempSpiderNLBInfo.Scope,
		Listener: model.TbNLBListenerInfo{
			Protocol:     tempSpiderNLBInfo.Listener.Protocol,
			IP:           tempSpiderNLBInfo.Listener.IP,
			Port:         tempSpiderNLBInfo.Listener.Port,
			DNSName:      tempSpiderNLBInfo.Listener.DNSName,
			KeyValueList: tempSpiderNLBInfo.Listener.KeyValueList,
		},
		HealthChecker: model.TbNLBHealthCheckerInfo{
			Protocol:     tempSpiderNLBInfo.HealthChecker.Protocol,
			Port:         tempSpiderNLBInfo.HealthChecker.Port,
			Interval:     tempSpiderNLBInfo.HealthChecker.Interval,
			Timeout:      tempSpiderNLBInfo.HealthChecker.Timeout,
			Threshold:    tempSpiderNLBInfo.HealthChecker.Threshold,
			KeyValueList: tempSpiderNLBInfo.HealthChecker.KeyValueList,
		},
		CspResourceId:        tempSpiderNLBInfo.IId.SystemId,
		CspResourceName:      tempSpiderNLBInfo.IId.NameId,
		CreatedTime:          tempSpiderNLBInfo.CreatedTime,
		Description:          u.Description,
		KeyValueList:         tempSpiderNLBInfo.KeyValueList,
		AssociatedObjectList: []string{},
		TargetGroup: model.TbNLBTargetGroupInfo{
			Protocol:     tempSpiderNLBInfo.VMGroup.Protocol,
			Port:         tempSpiderNLBInfo.VMGroup.Port,
			SubGroupId:   u.TargetGroup.SubGroupId,
			VMs:          vmIDs,
			KeyValueList: tempSpiderNLBInfo.VMGroup.KeyValueList,
		},
		Location: location,
	}

	if option == "register" && u.CspResourceId == "" {
		content.SystemLabel = "Registered from CB-Spider resource"
	} else if option == "register" && u.CspResourceId != "" {
		content.SystemLabel = "Registered from CSP resource"
	}

	// kvstore
	// Key := common.GenResourceKey(nsId, model.StrNLB, content.Id)
	Key := GenNLBKey(nsId, mciId, content.Id)
	Val, _ := json.Marshal(content)

	err = kvstore.Put(Key, string(Val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return content, err
	}

	keyValue, err := kvstore.GetKv(Key)
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("In CreateNLB(); kvstore.GetKv() returned an error.")
		log.Error().Err(err).Msg("")
		// return nil, err
	}

	result := model.TbNLBInfo{}
	err = json.Unmarshal([]byte(keyValue.Value), &result)
	if err != nil {
		log.Error().Err(err).Msg("")
	}
	return result, nil
}

// GetNLB returns the requested TB NLB object
func GetNLB(nsId string, mciId string, resourceId string) (model.TbNLBInfo, error) {

	emptyObj := model.TbNLBInfo{}

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	err = common.CheckString(resourceId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}
	check, err := CheckNLB(nsId, mciId, resourceId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	if !check {
		errString := "The NLB " + resourceId + " does not exist."
		err := fmt.Errorf(errString)
		return emptyObj, err
	}

	log.Debug().Msg("[Get NLB] " + resourceId)

	// key := common.GenResourceKey(nsId, resourceType, resourceId)
	key := GenNLBKey(nsId, mciId, resourceId)

	keyValue, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	res := model.TbNLBInfo{}

	if keyValue != (kvstore.KeyValue{}) {
		err = json.Unmarshal([]byte(keyValue.Value), &res)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyObj, err
		}
		return res, nil
	}
	errString := "Cannot get the NLB " + resourceId + "."
	err = fmt.Errorf(errString)
	return res, err
}

// GetMcNlbAccess returns the requested TB G-NLB access info (currenly MCI)
func GetMcNlbAccess(nsId string, mciId string) (*model.MciAccessInfo, error) {
	nlbMciId := mciId + nlbPostfix
	return GetMciAccessInfo(nsId, nlbMciId, "")
}

// CheckNLB returns the existence of the TB NLB object in bool form.
func CheckNLB(nsId string, mciId string, resourceId string) (bool, error) {

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
		log.Error().Err(err).Msg("")
		return false, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	err = common.CheckString(resourceId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	// key := common.GenResourceKey(nsId, resourceType, resourceId)
	key := GenNLBKey(nsId, mciId, resourceId)

	keyValue, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}
	if keyValue != (kvstore.KeyValue{}) {
		return true, nil
	}
	return false, nil

}

// GenNLBKey is func to generate a key from NLB id
func GenNLBKey(nsId string, mciId string, resourceId string) string {
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "/invalidKey"
	}

	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "/invalidKey"
	}

	err = common.CheckString(resourceId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "/invalidKey"
	}

	return fmt.Sprintf("/ns/%s/mci/%s/nlb/%s", nsId, mciId, resourceId)
}

// ListNLBId returns the list of TB NLB object IDs of given nsId
func ListNLBId(nsId string, mciId string) ([]string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	log.Debug().Msg("[ListNLBId] ns: " + nsId)
	// key := "/ns/" + nsId + "/"
	key := fmt.Sprintf("/ns/%s/mci/%s/", nsId, mciId)
	fmt.Println(key)

	keyValue, err := kvstore.GetKvList(key)

	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	/* if keyValue == nil, then for-loop below will not be executed, and the empty array will be returned in `resourceList` placeholder.
	if keyValue == nil {
		err = fmt.Errorf("ListResourceId(); %s is empty.", key)
		log.Error().Err(err).Msg("")
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
func ListNLB(nsId string, mciId string, filterKey string, filterVal string) (interface{}, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	log.Debug().Msg("[Get] NLB list")
	key := fmt.Sprintf("/ns/%s/mci/%s/nlb", nsId, mciId)
	fmt.Println(key)

	keyValue, err := kvstore.GetKvList(key)
	keyValue = kvutil.FilterKvListBy(keyValue, key, 1)

	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	if keyValue != nil {
		res := []model.TbNLBInfo{}
		for _, v := range keyValue {

			tempObj := model.TbNLBInfo{}
			err = json.Unmarshal([]byte(v.Value), &tempObj)
			if err != nil {
				log.Error().Err(err).Msg("")
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
		res := []model.TbNLBInfo{}
		return res, nil

	}

	err = fmt.Errorf("Some exceptional case happened. Please check the references of " + common.GetFuncName())
	return nil, err // if interface{} == nil, make err be returned. Should not come this part if there is no err.
}

// DelNLB deletes the TB NLB object
func DelNLB(nsId string, mciId string, resourceId string, forceFlag string) error {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	err = common.CheckString(resourceId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	check, err := CheckNLB(nsId, mciId, resourceId)

	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	if !check {
		errString := "The NLB " + resourceId + " does not exist."
		err := fmt.Errorf(errString)
		return err
	}

	key := GenNLBKey(nsId, mciId, resourceId)
	fmt.Println("key: " + key)

	keyValue, _ := kvstore.GetKv(key)
	// In CheckResource() above, calling 'kvstore.GetKv()' and checking err parts exist.
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
			log.Error().Err(err).Msg("")
			return err
		}
	*/

	//cspType := common.GetResourcesCspType(nsId, resourceType, resourceId)

	// NLB has no childResources, so below line is commented.
	// var childResources interface{}

	var url string

	// Create Req body
	type JsonTemplate struct {
		ConnectionName string
	}
	requestBody := JsonTemplate{}

	temp := model.TbNLBInfo{}
	err = json.Unmarshal([]byte(keyValue.Value), &temp)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	requestBody.ConnectionName = temp.ConnectionName
	url = model.SpiderRestUrl + "/nlb/" + temp.CspResourceName

	fmt.Println("url: " + url)

	client := resty.New().SetCloseConnection(true)

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(requestBody).
		//SetResult(&SpiderSpecInfo{}). // or SetResult(AuthSuccess{}).
		//SetError(&AuthError{}).       // or SetError(AuthError{}).
		Delete(url)

	if err != nil {
		log.Error().Err(err).Msg("")
		err := fmt.Errorf("an error occurred while requesting to CB-Spider")
		return err
	}

	// fmt.Println("HTTP Status code: " + strconv.Itoa(resp.StatusCode()))
	switch {
	case forceFlag == "true":
		url += "?force=true"
		log.Debug().Msg("forceFlag == true; url: " + url)

		_, err := client.R().
			SetHeader("Content-Type", "application/json").
			SetBody(requestBody).
			//SetResult(&SpiderSpecInfo{}). // or SetResult(AuthSuccess{}).
			//SetError(&AuthError{}).       // or SetError(AuthError{}).
			Delete(url)

		if err != nil {
			log.Error().Err(err).Msg("")
			err := fmt.Errorf("an error occurred while requesting to CB-Spider")
			return err
		}

	case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
		err := fmt.Errorf(string(resp.Body()))
		log.Error().Err(err).Msg("")
		return err
	default:

	}

	err = kvstore.Delete(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	return nil
}

// DelAllNLB deletes all TB NLB object of given nsId
func DelAllNLB(nsId string, mciId string, subString string, forceFlag string) (model.IdList, error) {

	deletedResources := model.IdList{}
	deleteStatus := ""

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return deletedResources, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return deletedResources, err
	}

	resourceIdList, err := ListNLBId(nsId, mciId)
	if err != nil {
		return deletedResources, err
	}

	// Do not return error when len(resourceIdList) == 0
	// if len(resourceIdList) == 0 {
	// 	errString := "There is no NLB in " + nsId
	// 	err := fmt.Errorf(errString)
	// 	log.Error().Err(err).Msg("")
	// 	return deletedResources, err
	// }

	for _, v := range resourceIdList {
		// if subString is provided, check the resourceId contains the subString.
		if subString == "" || strings.Contains(v, subString) {

			deleteStatus = "[Done] "
			errString := ""

			err := DelNLB(nsId, mciId, v, forceFlag)
			if err != nil {
				deleteStatus = "[Failed] "
				errString = " (" + err.Error() + ")"
			}

			deletedResources.IdList = append(deletedResources.IdList, deleteStatus+"NLB: "+v+errString)
		}
	}
	return deletedResources, nil
}

// GetNLBHealth queries the health status of NLB to CB-Spider, and returns it to user
func GetNLBHealth(nsId string, mciId string, nlbId string) (model.TbNLBHealthInfo, error) {
	log.Info().Msg("GetNLBHealth")

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.TbNLBHealthInfo{}, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.TbNLBHealthInfo{}, err
	}

	err = common.CheckString(nlbId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.TbNLBHealthInfo{}, err
	}

	check, err := CheckNLB(nsId, mciId, nlbId)

	if !check {
		err := fmt.Errorf("The nlb " + nlbId + " does not exist.")
		return model.TbNLBHealthInfo{}, err
	}

	if err != nil {
		err := fmt.Errorf("Failed to check the existence of the nlb " + nlbId + ".")
		return model.TbNLBHealthInfo{}, err
	}

	nlb, err := GetNLB(nsId, mciId, nlbId)
	if err != nil {
		err := fmt.Errorf("Failed to get the NLB " + nlbId + ".")
		return model.TbNLBHealthInfo{}, err
	}

	requestBody := model.SpiderConnectionName{}
	requestBody.ConnectionName = nlb.ConnectionName

	var tempSpiderNLBHealthInfo *model.SpiderNLBHealthInfoWrapper

	client := resty.New().SetCloseConnection(true)
	client.SetAllowGetMethodPayload(true)

	req := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(requestBody).
		SetResult(&model.SpiderNLBHealthInfoWrapper{}) // or SetResult(AuthSuccess{}).
		//SetError(&AuthError{}).       // or SetError(AuthError{}).

	var resp *resty.Response

	var url string
	url = fmt.Sprintf("%s/nlb/%s/health", model.SpiderRestUrl, nlb.CspResourceName)
	resp, err = req.Get(url)

	if err != nil {
		log.Error().Err(err).Msg("")
		err := fmt.Errorf("an error occurred while requesting to CB-Spider")
		return model.TbNLBHealthInfo{}, err
	}

	// fmt.Println("HTTP Status code: " + strconv.Itoa(resp.StatusCode()))
	switch {
	case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
		err := fmt.Errorf(string(resp.Body()))
		log.Error().Err(err).Msg("")
		return model.TbNLBHealthInfo{}, err
	}

	tempSpiderNLBHealthInfo = resp.Result().(*model.SpiderNLBHealthInfoWrapper)

	result := model.TbNLBHealthInfo{}

	if tempSpiderNLBHealthInfo.Healthinfo.HealthyVMs != nil {
		for _, v := range *tempSpiderNLBHealthInfo.Healthinfo.HealthyVMs {
			vm, err := FindTbVmByCspId(nsId, mciId, v.NameId)
			if err != nil {
				return model.TbNLBHealthInfo{}, err
			}

			result.HealthyVMs = append(result.HealthyVMs, vm.Id)
		}
	}

	if tempSpiderNLBHealthInfo.Healthinfo.UnHealthyVMs != nil {
		for _, v := range *tempSpiderNLBHealthInfo.Healthinfo.UnHealthyVMs {
			vm, err := FindTbVmByCspId(nsId, mciId, v.NameId)
			if err != nil {
				return model.TbNLBHealthInfo{}, err
			}

			result.UnHealthyVMs = append(result.UnHealthyVMs, vm.Id)
		}
	}

	result.AllVMs = append(result.AllVMs, result.HealthyVMs...)
	result.AllVMs = append(result.AllVMs, result.UnHealthyVMs...)
	/*
		// kvstore
		// Key := common.GenResourceKey(nsId, model.StrNLB, content.Id)
		Key := GenNLBKey(nsId, mciId, content.Id)
		Val, _ := json.Marshal(content)

		err = kvstore.Put(Key, string(Val))
		if err != nil {
			log.Error().Err(err).Msg("")
			return content, err
		}

		keyValue, err := kvstore.GetKv(key)
		if err != nil {
			log.Error().Err(err).Msg("")
			err = fmt.Errorf("In CreateNLB(); kvstore.GetKv() returned an error.")
			log.Error().Err(err).Msg("")
			// return nil, err
		}




		result := model.TbNLBInfo{}
		err = json.Unmarshal([]byte(keyValue.Value), &result)
		if err != nil {
			log.Error().Err(err).Msg("")
		}
	*/

	return result, nil
}

// AddNLBVMs accepts VM addition request, adds VM to NLB, and returns an updated TB NLB object
func AddNLBVMs(nsId string, mciId string, resourceId string, u *model.TbNLBAddRemoveVMReq) (model.TbNLBInfo, error) {
	log.Info().Msg("AddNLBVMs")

	err := common.CheckString(nsId)
	if err != nil {
		temp := model.TbNLBInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		temp := model.TbNLBInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = validate.Struct(u)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Err(err).Msg("")
			temp := model.TbNLBInfo{}
			return temp, err
		}

		temp := model.TbNLBInfo{}
		return temp, err
	}

	check, err := CheckNLB(nsId, mciId, resourceId)

	if !check {
		temp := model.TbNLBInfo{}
		err := fmt.Errorf("The nlb " + resourceId + " does not exist.")
		return temp, err
	}

	if err != nil {
		temp := model.TbNLBInfo{}
		err := fmt.Errorf("Failed to check the existence of the nlb " + resourceId + ".")
		return temp, err
	}

	/*
		vNetInfo := model.TbVNetInfo{}
		tempInterface, err := resource.GetResource(nsId, model.StrVNet, u.VNetId)
		if err != nil {
			err := fmt.Errorf("Failed to get the TbVNetInfo " + u.VNetId + ".")
			return model.TbNLBInfo{}, err
		}
		err = common.CopySrcToDest(&tempInterface, &vNetInfo)
		if err != nil {
			err := fmt.Errorf("Failed to get the TbVNetInfo-CopySrcToDest() " + u.VNetId + ".")
			return model.TbNLBInfo{}, err
		}
	*/

	nlb, err := GetNLB(nsId, mciId, resourceId)
	if err != nil {
		temp := model.TbNLBInfo{}
		err := fmt.Errorf("Failed to get the nlb object " + resourceId + ".")
		return temp, err
	}

	requestBody := model.SpiderNLBAddRemoveVMReqInfoWrapper{}
	requestBody.ConnectionName = nlb.ConnectionName

	for _, v := range u.TargetGroup.VMs {
		vm, err := GetVmObject(nsId, mciId, v)
		if err != nil {
			log.Error().Err(err).Msg("")
			return model.TbNLBInfo{}, err
		}

		requestBody.ReqInfo.VMs = append(requestBody.ReqInfo.VMs, vm.CspResourceName)
	}

	var tempSpiderNLBInfo *model.SpiderNLBInfo

	client := resty.New().SetCloseConnection(true)
	client.SetAllowGetMethodPayload(true)

	req := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(requestBody).
		SetResult(&model.SpiderNLBInfo{}) // or SetResult(AuthSuccess{}).
		//SetError(&AuthError{}).       // or SetError(AuthError{}).

	var resp *resty.Response

	var url string
	url = fmt.Sprintf("%s/nlb/%s/vms", model.SpiderRestUrl, nlb.CspResourceName)
	resp, err = req.Post(url)

	if err != nil {
		log.Error().Err(err).Msg("")
		content := model.TbNLBInfo{}
		err := fmt.Errorf("an error occurred while requesting to CB-Spider")
		return content, err
	}

	// fmt.Println("HTTP Status code: " + strconv.Itoa(resp.StatusCode()))
	switch {
	case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
		err := fmt.Errorf(string(resp.Body()))
		log.Error().Err(err).Msg("")
		content := model.TbNLBInfo{}
		return content, err
	}

	tempSpiderNLBInfo = resp.Result().(*model.SpiderNLBInfo)

	content := model.TbNLBInfo{
		Id:             nlb.Name,
		Name:           nlb.Name,
		ConnectionName: nlb.ConnectionName,
		Type:           tempSpiderNLBInfo.Type,
		Scope:          tempSpiderNLBInfo.Scope,
		Listener: model.TbNLBListenerInfo{
			Protocol:     tempSpiderNLBInfo.Listener.Protocol,
			IP:           tempSpiderNLBInfo.Listener.IP,
			Port:         tempSpiderNLBInfo.Listener.Port,
			DNSName:      tempSpiderNLBInfo.Listener.DNSName,
			KeyValueList: tempSpiderNLBInfo.Listener.KeyValueList,
		},
		HealthChecker: model.TbNLBHealthCheckerInfo{
			Protocol:     tempSpiderNLBInfo.HealthChecker.Protocol,
			Port:         tempSpiderNLBInfo.HealthChecker.Port,
			Interval:     tempSpiderNLBInfo.HealthChecker.Interval,
			Timeout:      tempSpiderNLBInfo.HealthChecker.Timeout,
			Threshold:    tempSpiderNLBInfo.HealthChecker.Threshold,
			KeyValueList: tempSpiderNLBInfo.HealthChecker.KeyValueList,
		},
		CspResourceId:        tempSpiderNLBInfo.IId.SystemId,
		CspResourceName:      tempSpiderNLBInfo.IId.NameId,
		CreatedTime:          tempSpiderNLBInfo.CreatedTime,
		Description:          nlb.Description,
		KeyValueList:         tempSpiderNLBInfo.KeyValueList,
		AssociatedObjectList: []string{},
		TargetGroup: model.TbNLBTargetGroupInfo{
			Protocol:   tempSpiderNLBInfo.VMGroup.Protocol,
			Port:       tempSpiderNLBInfo.VMGroup.Port,
			SubGroupId: nlb.TargetGroup.SubGroupId,
			// VMs:          vmIDs,
			KeyValueList: tempSpiderNLBInfo.VMGroup.KeyValueList,
		},
	}
	content.TargetGroup.VMs = append(content.TargetGroup.VMs, nlb.TargetGroup.VMs...)
	content.TargetGroup.VMs = append(content.TargetGroup.VMs, u.TargetGroup.VMs...)

	// kvstore
	// Key := common.GenResourceKey(nsId, model.StrNLB, content.Id)
	Key := GenNLBKey(nsId, mciId, content.Id)
	Val, _ := json.Marshal(content)

	err = kvstore.Put(Key, string(Val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return content, err
	}

	keyValue, err := kvstore.GetKv(Key)
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("In CreateNLB(); kvstore.GetKv() returned an error.")
		log.Error().Err(err).Msg("")
		// return nil, err
	}

	result := model.TbNLBInfo{}
	err = json.Unmarshal([]byte(keyValue.Value), &result)
	if err != nil {
		log.Error().Err(err).Msg("")
	}
	return result, nil
}

// RemoveNLBVMs accepts VM removal request, removes VMs from NLB, and returns an error if occurs.
func RemoveNLBVMs(nsId string, mciId string, resourceId string, u *model.TbNLBAddRemoveVMReq) error {
	log.Info().Msg("RemoveNLBVMs")

	err := common.CheckString(nsId)
	if err != nil {
		// temp := model.TbNLBInfo{}
		log.Error().Err(err).Msg("")
		return err
	}

	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	err = validate.Struct(u)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Err(err).Msg("")
			// temp := model.TbNLBInfo{}
			return err
		}

		// temp := model.TbNLBInfo{}
		return err
	}

	check, err := CheckNLB(nsId, mciId, resourceId)

	if !check {
		// temp := model.TbNLBInfo{}
		err := fmt.Errorf("The nlb " + resourceId + " does not exist.")
		return err
	}

	if err != nil {
		// temp := model.TbNLBInfo{}
		err := fmt.Errorf("Failed to check the existence of the nlb " + resourceId + ".")
		return err
	}

	/*
		vNetInfo := model.TbVNetInfo{}
		tempInterface, err := resource.GetResource(nsId, model.StrVNet, u.VNetId)
		if err != nil {
			err := fmt.Errorf("Failed to get the TbVNetInfo " + u.VNetId + ".")
			return model.TbNLBInfo{}, err
		}
		err = common.CopySrcToDest(&tempInterface, &vNetInfo)
		if err != nil {
			err := fmt.Errorf("Failed to get the TbVNetInfo-CopySrcToDest() " + u.VNetId + ".")
			return model.TbNLBInfo{}, err
		}
	*/

	nlb, err := GetNLB(nsId, mciId, resourceId)
	if err != nil {
		// temp := model.TbNLBInfo{}
		err := fmt.Errorf("Failed to get the nlb object " + resourceId + ".")
		return err
	}

	requestBody := model.SpiderNLBAddRemoveVMReqInfoWrapper{}
	requestBody.ConnectionName = nlb.ConnectionName

	// fmt.Printf("u.TargetGroup.VMs: %s \n", u.TargetGroup.VMs) // for debug

	for _, v := range u.TargetGroup.VMs {
		vm, err := GetVmObject(nsId, mciId, v)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
		// log.Debug().Msg("vm:")                             // for debug
		// payload, _ := json.MarshalIndent(vm, "", "  ") // for debug
		// fmt.Print(string(payload))                     // for debug
		if vm.CspResourceName == "" {
			fmt.Printf("Failed to get %s; skipping;", v)
		} else {
			requestBody.ReqInfo.VMs = append(requestBody.ReqInfo.VMs, vm.CspResourceName)
		}
	}

	// fmt.Printf("requestBody.ReqInfo.SubGroup.VMs: %s \n", requestBody.ReqInfo.VMs) // for debug
	/*
		for _, v := range u.VMIDList {
			mciId_vmId := strings.Split(v, "/")
			if len(mciId_vmId) != 2 {
				err := fmt.Errorf("Cannot retrieve VM info: " + v)
				log.Error().Err(err).Msg("")
				return model.TbNLBInfo{}, err
			}

			vm, err := infra.GetVmObject(nsId, mciId_vmId[0], mciId_vmId[1])
			if err != nil {
				log.Error().Err(err).Msg("")
				return model.TbNLBInfo{}, err
			}

			requestBody.ReqInfo.SubGroup = append(requestBody.ReqInfo.SubGroup, vm.CspResourceId)
		}
	*/

	// var tempSpiderNLBInfo *model.SpiderNLBInfo

	client := resty.New().SetCloseConnection(true)
	client.SetAllowGetMethodPayload(true)

	req := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(requestBody)
		// SetResult(&model.SpiderNLBInfo{}) // or SetResult(AuthSuccess{}).
		//SetError(&AuthError{}).       // or SetError(AuthError{}).

	var resp *resty.Response

	var url string
	url = fmt.Sprintf("%s/nlb/%s/vms", model.SpiderRestUrl, nlb.CspResourceName)
	resp, err = req.Delete(url)

	if err != nil {
		log.Error().Err(err).Msg("")
		// content := model.TbNLBInfo{}
		err := fmt.Errorf("an error occurred while requesting to CB-Spider")
		return err
	}

	// fmt.Println("HTTP Status code: " + strconv.Itoa(resp.StatusCode()))
	switch {
	case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
		err := fmt.Errorf(string(resp.Body()))
		log.Error().Err(err).Msg("")
		// content := model.TbNLBInfo{}
		return err
	}

	// result := resp.Result().(bool)

	oldVMList := nlb.TargetGroup.VMs
	for _, vmToDelete := range u.TargetGroup.VMs {
		oldVMList = remove(oldVMList, vmToDelete)
	}
	newVMList := oldVMList

	/*
		content := model.TbNLBInfo{}
		//content.Id = common.GenUid()
		content.Id = nlb.Id
		content.Name = nlb.Name
		content.ConnectionName = nlb.ConnectionName
		content.Type = tempSpiderNLBInfo.Type
		content.Scope = tempSpiderNLBInfo.Scope
		content.Listener = tempSpiderNLBInfo.Listener
		content.HealthChecker = tempSpiderNLBInfo.HealthChecker
		content.CspResourceId = tempSpiderNLBInfo.IId.SystemId
		content.CspResourceName = tempSpiderNLBInfo.IId.NameId
		content.Description = nlb.Description
		content.KeyValueList = tempSpiderNLBInfo.KeyValueList
		content.AssociatedObjectList = []string{}

		content.TargetGroup.Port = tempSpiderNLBInfo.SubGroup.Port
		content.TargetGroup.Protocol = tempSpiderNLBInfo.SubGroup.Protocol
		content.TargetGroup.MCI = u.TargetGroup.MCI // What if oldNlb.TargetGroup.MCI != newNlb.TargetGroup.MCI
		content.TargetGroup.CspID = u.TargetGroup.CspID
		content.TargetGroup.KeyValueList = u.TargetGroup.KeyValueList

		// content.TargetGroup.VMs = u.TargetGroup.VMs
		content.TargetGroup.VMs = append(content.TargetGroup.VMs, nlb.TargetGroup.VMs...)
		content.TargetGroup.VMs = append(content.TargetGroup.VMs, u.TargetGroup.VMs...)
	*/

	nlb.TargetGroup.VMs = newVMList

	// kvstore
	// Key := common.GenResourceKey(nsId, model.StrNLB, content.Id)
	Key := GenNLBKey(nsId, mciId, nlb.Id)
	Val, _ := json.Marshal(nlb)

	err = kvstore.Put(Key, string(Val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

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
