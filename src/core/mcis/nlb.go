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
	"reflect"
	"strconv"
	"strings"
	"time"

	cbstore_utils "github.com/cloud-barista/cb-store/utils"
	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
	validator "github.com/go-playground/validator/v10"
	"github.com/go-resty/resty/v2"
)

const nlbPostfix = "-nlb"

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

	Listener NLBListenerReq

	//------ Backend

	VMGroup       SpiderNLBSubGroupReq
	HealthChecker SpiderNLBHealthCheckerReq
}

type SpiderNLBHealthCheckerReq struct {
	Protocol  string `json:"protocol" example:"TCP"`      // TCP|HTTP|HTTPS
	Port      string `json:"port" example:"22"`           // Listener Port or 1-65535
	Interval  string `json:"interval" example:"default"`  // secs, Interval time between health checks.
	Timeout   string `json:"timeout" example:"default"`   // secs, Waiting time to decide an unhealthy VM when no response.
	Threshold string `json:"threshold" example:"default"` // num, The number of continuous health checks to change the VM status.
}

type TbNLBHealthCheckerReq struct {
	// Protocol  string `json:"protocol" example:"TCP"`      // TCP|HTTP|HTTPS
	// Port      string `json:"port" example:"22"`           // Listener Port or 1-65535

	Interval  string `json:"interval" example:"default"`  // secs, Interval time between health checks.
	Timeout   string `json:"timeout" example:"default"`   // secs, Waiting time to decide an unhealthy VM when no response.
	Threshold string `json:"threshold" example:"default"` // num, The number of continuous health checks to change the VM status.
}

type SpiderNLBSubGroupReq struct {
	Protocol string // TCP|HTTP|HTTPS
	Port     string // Listener Port or 1-65535
	VMs      []string
}

// SpiderNLBAddRemoveVMReqInfoWrapper is a wrapper struct to create JSON body of 'Add/Remove VMs to/from NLB' request
type SpiderNLBAddRemoveVMReqInfoWrapper struct {
	ConnectionName string
	ReqInfo        SpiderNLBSubGroupReq
}

// SpiderNLBInfo is a struct to handle NLB information from the CB-Spider's REST API response
type SpiderNLBInfo struct {
	IId    common.IID // {NameId, SystemId}
	VpcIID common.IID // {NameId, SystemId}

	Type  string // PUBLIC(V) | INTERNAL
	Scope string // REGION(V) | GLOBAL

	//------ Frontend
	Listener SpiderNLBListenerInfo

	//------ Backend
	VMGroup       SpiderNLBSubGroupInfo
	HealthChecker SpiderNLBHealthCheckerInfo

	CreatedTime  time.Time
	KeyValueList []common.KeyValue
}

// NLBListenerReq is a struct to handle NLB Listener information of the CB-Spider's & CB-Tumblebug's REST API request
type NLBListenerReq struct { // for both Spider and Tumblebug
	Protocol string `json:"protocol" example:"TCP"` // TCP|UDP
	Port     string `json:"port" example:"80"`      // 1-65535
}

// SpiderNLBListenerInfo is a struct to handle NLB Listener information from the CB-Spider's REST API response
type SpiderNLBListenerInfo struct {
	Protocol string `json:"protocol" example:"TCP"` // TCP|UDP
	IP       string `json:"ip" example:""`          // Auto Generated and attached
	Port     string `json:"port" example:"80"`      // 1-65535
	DNSName  string `json:"dnsName" example:""`     // Optional, Auto Generated and attached

	CspID        string            `json:"cspID"` // Optional, May be Used by Driver.
	KeyValueList []common.KeyValue `json:"keyValueList"`
}

// TbNLBListenerInfo is a struct to handle NLB Listener information from the CB-Tumblebug's REST API response
type TbNLBListenerInfo struct {
	Protocol string `json:"protocol" example:"TCP"`                                            // TCP|UDP
	IP       string `json:"ip" example:"x.x.x.x"`                                              // Auto Generated and attached
	Port     string `json:"port" example:"80"`                                                 // 1-65535
	DNSName  string `json:"dnsName" example:"ns01-group-cd3.elb.ap-northeast-2.amazonaws.com"` // Optional, Auto Generated and attached

	KeyValueList []common.KeyValue `json:"keyValueList"`
}

// SpiderNLBSubGroupInfo is a struct from NLBSubGroupInfo from Spider
type SpiderNLBSubGroupInfo struct {
	Protocol string // TCP|UDP|HTTP|HTTPS
	Port     string // 1-65535
	VMs      *[]common.IID

	CspID        string            `json:"cspID"` // Optional, May be Used by Driver.
	KeyValueList []common.KeyValue `json:"keyValueList"`
}

type SpiderNLBHealthCheckerInfo struct {
	Protocol  string `json:"protocol" example:"TCP"` // TCP|HTTP|HTTPS
	Port      string `json:"port" example:"22"`      // Listener Port or 1-65535
	Interval  int    `json:"interval" example:"10"`  // secs, Interval time between health checks.
	Timeout   int    `json:"timeout" example:"10"`   // secs, Waiting time to decide an unhealthy VM when no response.
	Threshold int    `json:"threshold" example:"3"`  // num, The number of continuous health checks to change the VM status.

	CspID        string            `json:"cspID"` // Optional, May be Used by Driver.
	KeyValueList []common.KeyValue `json:"keyValueList"`
}

type TbNLBHealthCheckerInfo struct {
	Protocol  string `json:"protocol" example:"TCP"` // TCP|HTTP|HTTPS
	Port      string `json:"port" example:"22"`      // Listener Port or 1-65535
	Interval  int    `json:"interval" example:"10"`  // secs, Interval time between health checks.
	Timeout   int    `json:"timeout" example:"10"`   // secs, Waiting time to decide an unhealthy VM when no response.
	Threshold int    `json:"threshold" example:"3"`  // num, The number of continuous health checks to change the VM status.

	KeyValueList []common.KeyValue `json:"keyValueList"`
}

type SpiderNLBHealthInfoWrapper struct {
	Healthinfo SpiderNLBHealthInfo
}

type SpiderNLBHealthInfo struct {
	AllVMs       *[]common.IID
	HealthyVMs   *[]common.IID
	UnHealthyVMs *[]common.IID
}

type TbNLBTargetGroupReq struct {
	Protocol   string `json:"protocol" example:"TCP"` // TCP|HTTP|HTTPS
	Port       string `json:"port" example:"80"`      // Listener Port or 1-65535
	SubGroupId string `json:"subGroupId" example:"g1"`
}

type TbNLBTargetGroupInfo struct {
	Protocol string `json:"protocol" example:"TCP"` // TCP|HTTP|HTTPS
	Port     string `json:"port" example:"80"`      // Listener Port or 1-65535

	SubGroupId string   `json:"subGroupId" example:"g1"`
	VMs        []string `json:"vms"`

	KeyValueList []common.KeyValue
}

// TbNLBReq is a struct to handle 'Create nlb' request toward CB-Tumblebug.
type TbNLBReq struct { // Tumblebug
	// Name           string `json:"name" validate:"required" example:"mc"`
	// ConnectionName string `json:"connectionName" validate:"required" example:"aws-ap-northeast-2"`
	// VNetId         string `json:"vNetId" validate:"required" example:"ns01-systemdefault-aws-ap-northeast-2"`

	Description string `json:"description"`
	// Existing NLB (used only for option=register)
	CspNLBId string `json:"cspNLBId"`

	Type  string `json:"type" validate:"required" enums:"PUBLIC,INTERNAL" example:"PUBLIC"` // PUBLIC(V) | INTERNAL
	Scope string `json:"scope" validate:"required" enums:"REGION,GLOBAL" example:"REGION"`  // REGION(V) | GLOBAL

	// Frontend
	Listener NLBListenerReq `json:"listener" validate:"required"`
	// Backend
	TargetGroup TbNLBTargetGroupReq `json:"targetGroup" validate:"required"`
	// HealthChecker
	HealthChecker TbNLBHealthCheckerReq `json:"healthChecker" validate:"required"`
}

// TbNLBReqStructLevelValidation is a function to validate 'TbNLBReq' object.
func TbNLBReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(TbNLBReq)

	err := common.CheckString(u.TargetGroup.SubGroupId)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.TargetGroup.SubGroupId, "name", "Name", err.Error(), "")
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

	Listener TbNLBListenerInfo `json:"listener"`

	//------ Backend

	TargetGroup   TbNLBTargetGroupInfo   `json:"targetGroup"`
	HealthChecker TbNLBHealthCheckerInfo `json:"healthChecker"`

	CreatedTime time.Time

	Description          string             `json:"description"`
	CspNLBId             string             `json:"cspNLBId"`
	CspNLBName           string             `json:"cspNLBName"`
	Status               string             `json:"status"`
	KeyValueList         []common.KeyValue  `json:"keyValueList"`
	AssociatedObjectList []string           `json:"associatedObjectList"`
	IsAutoGenerated      bool               `json:"isAutoGenerated"`
	Location             common.GeoLocation `json:"location"`

	// SystemLabel is for describing the MCIR in a keyword (any string can be used) for special System purpose
	SystemLabel string `json:"systemLabel" example:"Managed by CB-Tumblebug" default:""`

	// Disabled for now
	//Region         string `json:"region"`
	//ResourceGroupName string `json:"resourceGroupName"`
}

type TbNLBHealthInfo struct { // Tumblebug
	AllVMs       []string
	HealthyVMs   []string
	UnHealthyVMs []string
}

// TbNLBAddRemoveVMReq is a struct to handle 'Add/Remove VMs to/from NLB' request toward CB-Tumblebug.
type TbNLBAddRemoveVMReq struct { // Tumblebug
	TargetGroup TbNLBTargetGroupInfo `json:"targetGroup"`
}

// CreateMcSwNlb func create a special purpose MCIS for NLB and depoly and setting SW NLB
func CreateMcSwNlb(nsId string, mcisId string, req *TbNLBReq, option string) (TbMcisInfo, error) {
	fmt.Println("=========================== CreateMcSwNlb")

	emptyObj := TbMcisInfo{}

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return emptyObj, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return emptyObj, err
	}

	nlbMcisId := mcisId + nlbPostfix

	// create a special MCIS for (SW)NLB

	mcisDynamicReq := TbMcisDynamicReq{Name: nlbMcisId, InstallMonAgent: "no", Label: "McSwNlb"}

	// get vm requst from cloud_conf.yaml
	vmGroupName := "nlb"
	// default commonSpec
	commonSpec := common.RuntimeConf.Nlbsw.NlbMcisCommonSpec
	commonImage := common.RuntimeConf.Nlbsw.NlbMcisCommonImage
	subGroupSize := common.RuntimeConf.Nlbsw.NlbMcisSubGroupSize

	// Option can be applied
	// get recommended location and spec for the NLB host based on existing MCIS
	deploymentPlan := DeploymentPlan{}
	deploymentPlan.Priority.Policy = append(deploymentPlan.Priority.Policy, PriorityCondition{Metric: "latency"})
	deploymentPlan.Priority.Policy[0].Parameter = append(deploymentPlan.Priority.Policy[0].Parameter, ParameterKeyVal{Key: "latencyMinimal"})

	mcis, err := GetMcisObject(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return emptyObj, err
	}
	for _, vm := range mcis.Vm {
		regionOfVm := vm.ConnectionConfig.RegionName
		deploymentPlan.Priority.Policy[0].Parameter[0].Val = append(deploymentPlan.Priority.Policy[0].Parameter[0].Val, regionOfVm)
	}

	specList, err := RecommendVm(common.SystemCommonNs, deploymentPlan)
	if err != nil {
		common.CBLog.Error(err)
		return emptyObj, err
	}
	if len(specList) != 0 {
		recommendedSpec := specList[0].Id
		commonSpec = recommendedSpec
	}

	vmDynamicReq := TbVmDynamicReq{Name: vmGroupName, CommonSpec: commonSpec, CommonImage: commonImage, SubGroupSize: subGroupSize}
	mcisDynamicReq.Vm = append(mcisDynamicReq.Vm, vmDynamicReq)

	mcisInfo, err := CreateMcisDynamic(nsId, &mcisDynamicReq)
	if err != nil {
		common.CBLog.Error(err)
		return emptyObj, err
	}

	// Sleep for 60 seconds for a safe NLB installation.
	fmt.Printf("\n\n[Info] Sleep for 30 seconds for safe NLB installation.\n\n")
	time.Sleep(30 * time.Second)

	// Deploy SW NLB
	cmd := common.RuntimeConf.Nlbsw.CommandNlbPrepare
	_, err = RemoteCommandToMcis(nsId, nlbMcisId, "", &McisCmdReq{Command: cmd})
	if err != nil {
		common.CBLog.Error(err)
		return emptyObj, err
	}
	cmd = common.RuntimeConf.Nlbsw.CommandNlbDeploy + " " + mcisId + " " + common.ToLower(req.Listener.Protocol) + " " + req.Listener.Port
	_, err = RemoteCommandToMcis(nsId, nlbMcisId, "", &McisCmdReq{Command: cmd})
	if err != nil {
		common.CBLog.Error(err)
		return emptyObj, err
	}

	// nodeId=${1:-vm}
	// nodeIp=${2:-127.0.0.1}
	// targetPort=${3:-80}
	accessList, err := GetMcisAccessInfo(nsId, mcisId, "")
	if err != nil {
		common.CBLog.Error(err)
		return emptyObj, err
	}
	for _, v := range accessList.McisSubGroupAccessInfo {
		for _, k := range v.McisVmAccessInfo {

			cmd = common.RuntimeConf.Nlbsw.CommandNlbAddTargetNode + " " + k.VmId + " " + k.PublicIP + " " + req.TargetGroup.Port
			_, err = RemoteCommandToMcis(nsId, nlbMcisId, "", &McisCmdReq{Command: cmd})
			if err != nil {
				common.CBLog.Error(err)
				return emptyObj, err
			}

		}
	}

	cmd = common.RuntimeConf.Nlbsw.CommandNlbApplyConfig
	_, err = RemoteCommandToMcis(nsId, nlbMcisId, "", &McisCmdReq{Command: cmd})
	if err != nil {
		common.CBLog.Error(err)
		return emptyObj, err
	}

	return *mcisInfo, err

}

// CreateNLB accepts nlb creation request, creates and returns an TB nlb object
func CreateNLB(nsId string, mcisId string, u *TbNLBReq, option string) (TbNLBInfo, error) {
	fmt.Println("=========================== CreateNLB")

	emptyObj := TbNLBInfo{}

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return emptyObj, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return emptyObj, err
	}

	err = common.CheckString(u.TargetGroup.SubGroupId)
	if err != nil {
		common.CBLog.Error(err)
		return emptyObj, err
	}

	err = validate.Struct(u)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			fmt.Println(err)
			return emptyObj, err
		}

		return emptyObj, err
	}

	check, err := CheckNLB(nsId, mcisId, u.TargetGroup.SubGroupId)

	if check {
		err := fmt.Errorf("The nlb " + u.TargetGroup.SubGroupId + " already exists.")
		return emptyObj, err
	}

	if err != nil {
		err := fmt.Errorf("Failed to check the existence of the nlb " + u.TargetGroup.SubGroupId + ".")
		return emptyObj, err
	}

	vmIDs, err := ListMcisGroupVms(nsId, mcisId, u.TargetGroup.SubGroupId)
	if err != nil {
		err := fmt.Errorf("Failed to get VMs in the SubGroup " + u.TargetGroup.SubGroupId + ".")
		return emptyObj, err
	}
	if len(vmIDs) == 0 {
		err := fmt.Errorf("There is no VMs in the SubGroup " + u.TargetGroup.SubGroupId + ".")
		return emptyObj, err
	}

	vm, err := GetVmObject(nsId, mcisId, vmIDs[0])
	if err != nil {
		err := fmt.Errorf("Failed to get VM " + vmIDs[0] + ".")
		return emptyObj, err
	}

	vNetInfo := mcir.TbVNetInfo{}
	tempInterface, err := mcir.GetResource(nsId, common.StrVNet, vm.VNetId)
	if err != nil {
		err := fmt.Errorf("Failed to get the TbVNetInfo " + vm.VNetId + ".")
		return emptyObj, err
	}
	err = common.CopySrcToDest(&tempInterface, &vNetInfo)
	if err != nil {
		err := fmt.Errorf("Failed to get the TbVNetInfo-CopySrcToDest() " + vm.VNetId + ".")
		return emptyObj, err
	}

	tempReq := SpiderNLBReqInfoWrapper{
		ConnectionName: vm.ConnectionName,
		ReqInfo: SpiderNLBReqInfo{
			Name:     fmt.Sprintf("%s-%s", nsId, u.TargetGroup.SubGroupId),
			VPCName:  vNetInfo.CspVNetName,
			Type:     u.Type,
			Scope:    u.Scope,
			Listener: u.Listener,
			HealthChecker: SpiderNLBHealthCheckerReq{
				Protocol:  u.TargetGroup.Protocol,
				Port:      u.TargetGroup.Port,
				Interval:  u.HealthChecker.Interval,
				Timeout:   u.HealthChecker.Timeout,
				Threshold: u.HealthChecker.Threshold,
			},
			VMGroup: SpiderNLBSubGroupReq{
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
	cloudSetting := common.CloudSetting{}

	getCloudSetting := func() {
		// cloudSetting := common.CloudSetting{}

		defer func() {
			if err := recover(); err != nil {
				fmt.Println(err)
				cloudSetting = reflect.ValueOf(&common.RuntimeConf.Cloud).Elem().FieldByName("Common").Interface().(common.CloudSetting)
			}
		}()

		cloudSetting = reflect.ValueOf(&common.RuntimeConf.Cloud).Elem().FieldByName(fieldName).Interface().(common.CloudSetting)

		// return cloudSetting
	}

	getCloudSetting()

	// Set nlb health checker info
	valuesFromYaml := TbNLBHealthCheckerInfo{}
	valuesFromYaml.Interval, _ = strconv.Atoi(cloudSetting.Nlb.Interval)
	valuesFromYaml.Timeout, _ = strconv.Atoi(cloudSetting.Nlb.Timeout)
	valuesFromYaml.Threshold, _ = strconv.Atoi(cloudSetting.Nlb.Threshold)

	if u.HealthChecker.Interval == "default" || u.HealthChecker.Interval == "" {
		tempReq.ReqInfo.HealthChecker.Interval = strconv.Itoa(valuesFromYaml.Interval)
	}
	if u.HealthChecker.Timeout == "default" || u.HealthChecker.Timeout == "" {
		tempReq.ReqInfo.HealthChecker.Timeout = strconv.Itoa(valuesFromYaml.Timeout)
	}
	if u.HealthChecker.Threshold == "default" || u.HealthChecker.Threshold == "" {
		tempReq.ReqInfo.HealthChecker.Threshold = strconv.Itoa(valuesFromYaml.Threshold)
	}

	for _, v := range vmIDs {
		vm, err := GetVmObject(nsId, mcisId, v)
		if err != nil {
			common.CBLog.Error(err)
			return emptyObj, err
		}
		// fmt.Println("vm:")                             // for debug
		// payload, _ := json.MarshalIndent(vm, "", "  ") // for debug
		// fmt.Print(string(payload))                     // for debug
		// fmt.Print("vm.CspViewVmDetail.IId.NameId: " + vm.CspViewVmDetail.IId.NameId) // for debug
		tempReq.ReqInfo.VMGroup.VMs = append(tempReq.ReqInfo.VMGroup.VMs, vm.CspViewVmDetail.IId.NameId)
	}

	// fmt.Printf("u.TargetGroup.VMs: %s \n", u.TargetGroup.VMs)                     // for debug
	// fmt.Printf("tempReq.ReqInfo.SubGroup.VMs: %s \n", tempReq.ReqInfo.SubGroup.VMs) // for debug

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
			url = fmt.Sprintf("%s/nlb/%s", common.SpiderRestUrl, u.TargetGroup.SubGroupId)
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
			err := fmt.Errorf("an error occurred while requesting to CB-Spider")
			return emptyObj, err
		}

		fmt.Println("HTTP Status code: " + strconv.Itoa(resp.StatusCode()))
		switch {
		case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
			err := fmt.Errorf(string(resp.Body()))
			common.CBLog.Error(err)
			return emptyObj, err
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
				return emptyObj, err
			}
			err = ccm.Open()
			if err != nil {
				common.CBLog.Error("ccm api open failed : ", err)
				return emptyObj, err
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
				return emptyObj, err
			}

			tempSpiderNLBInfo = &SpiderNLBInfo{}
			err = json.Unmarshal([]byte(result), &tempSpiderNLBInfo)
			if err != nil {
				common.CBLog.Error(err)
				return emptyObj, err
			}

		}
	*/

	regionTmp, _ := common.GetRegion(connConfig.RegionName)
	nativeRegion := ""
	for _, v := range regionTmp.KeyValueInfoList {
		if strings.ToLower(v.Key) == "region" || strings.ToLower(v.Key) == "location" {
			nativeRegion = v.Value
			break
		}
	}

	content := TbNLBInfo{
		Id:             u.TargetGroup.SubGroupId,
		Name:           u.TargetGroup.SubGroupId,
		ConnectionName: vm.ConnectionName,
		Type:           tempSpiderNLBInfo.Type,
		Scope:          tempSpiderNLBInfo.Scope,
		Listener: TbNLBListenerInfo{
			Protocol:     tempSpiderNLBInfo.Listener.Protocol,
			IP:           tempSpiderNLBInfo.Listener.IP,
			Port:         tempSpiderNLBInfo.Listener.Port,
			DNSName:      tempSpiderNLBInfo.Listener.DNSName,
			KeyValueList: tempSpiderNLBInfo.Listener.KeyValueList,
		},
		HealthChecker: TbNLBHealthCheckerInfo{
			Protocol:     tempSpiderNLBInfo.HealthChecker.Protocol,
			Port:         tempSpiderNLBInfo.HealthChecker.Port,
			Interval:     tempSpiderNLBInfo.HealthChecker.Interval,
			Timeout:      tempSpiderNLBInfo.HealthChecker.Timeout,
			Threshold:    tempSpiderNLBInfo.HealthChecker.Threshold,
			KeyValueList: tempSpiderNLBInfo.HealthChecker.KeyValueList,
		},
		CspNLBId:             tempSpiderNLBInfo.IId.SystemId,
		CspNLBName:           tempSpiderNLBInfo.IId.NameId,
		CreatedTime:          tempSpiderNLBInfo.CreatedTime,
		Description:          u.Description,
		KeyValueList:         tempSpiderNLBInfo.KeyValueList,
		AssociatedObjectList: []string{},
		TargetGroup: TbNLBTargetGroupInfo{
			Protocol:     tempSpiderNLBInfo.VMGroup.Protocol,
			Port:         tempSpiderNLBInfo.VMGroup.Port,
			SubGroupId:   u.TargetGroup.SubGroupId,
			VMs:          vmIDs,
			KeyValueList: tempSpiderNLBInfo.VMGroup.KeyValueList,
		},
		Location: common.GetCloudLocation(strings.ToLower(connConfig.ProviderName), strings.ToLower(nativeRegion)),
	}

	if option == "register" && u.CspNLBId == "" {
		content.SystemLabel = "Registered from CB-Spider resource"
	} else if option == "register" && u.CspNLBId != "" {
		content.SystemLabel = "Registered from CSP resource"
	}

	// cb-store
	// Key := common.GenResourceKey(nsId, common.StrNLB, content.Id)
	Key := GenNLBKey(nsId, mcisId, content.Id)
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
func GetNLB(nsId string, mcisId string, resourceId string) (TbNLBInfo, error) {

	emptyObj := TbNLBInfo{}

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return emptyObj, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return emptyObj, err
	}

	err = common.CheckString(resourceId)
	if err != nil {
		common.CBLog.Error(err)
		return emptyObj, err
	}
	check, err := CheckNLB(nsId, mcisId, resourceId)
	if err != nil {
		common.CBLog.Error(err)
		return emptyObj, err
	}

	if !check {
		errString := "The NLB " + resourceId + " does not exist."
		err := fmt.Errorf(errString)
		return emptyObj, err
	}

	fmt.Println("[Get NLB] " + resourceId)

	// key := common.GenResourceKey(nsId, resourceType, resourceId)
	key := GenNLBKey(nsId, mcisId, resourceId)

	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
		return emptyObj, err
	}

	res := TbNLBInfo{}

	if keyValue != nil {
		err = json.Unmarshal([]byte(keyValue.Value), &res)
		if err != nil {
			common.CBLog.Error(err)
			return emptyObj, err
		}
		return res, nil
	}
	errString := "Cannot get the NLB " + resourceId + "."
	err = fmt.Errorf(errString)
	return res, err
}

// GetMcNlbAccess returns the requested TB G-NLB access info (currenly MCIS)
func GetMcNlbAccess(nsId string, mcisId string) (*McisAccessInfo, error) {
	nlbMcisId := mcisId + nlbPostfix
	return GetMcisAccessInfo(nsId, nlbMcisId, "")
}

// CheckNLB returns the existence of the TB NLB object in bool form.
func CheckNLB(nsId string, mcisId string, resourceId string) (bool, error) {

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

	err = common.CheckString(mcisId)
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
	key := GenNLBKey(nsId, mcisId, resourceId)

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
func GenNLBKey(nsId string, mcisId string, resourceId string) string {
	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return "/invalidKey"
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return "/invalidKey"
	}

	err = common.CheckString(resourceId)
	if err != nil {
		common.CBLog.Error(err)
		return "/invalidKey"
	}

	return fmt.Sprintf("/ns/%s/mcis/%s/nlb/%s", nsId, mcisId, resourceId)
}

// ListNLBId returns the list of TB NLB object IDs of given nsId
func ListNLBId(nsId string, mcisId string) ([]string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	fmt.Println("[ListNLBId] ns: " + nsId)
	// key := "/ns/" + nsId + "/"
	key := fmt.Sprintf("/ns/%s/mcis/%s/", nsId, mcisId)
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
func ListNLB(nsId string, mcisId string, filterKey string, filterVal string) (interface{}, error) {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	fmt.Println("[Get] NLB list")
	key := fmt.Sprintf("/ns/%s/mcis/%s/nlb", nsId, mcisId)
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
func DelNLB(nsId string, mcisId string, resourceId string, forceFlag string) error {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	err = common.CheckString(resourceId)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}
	check, err := CheckNLB(nsId, mcisId, resourceId)

	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	if !check {
		errString := "The NLB " + resourceId + " does not exist."
		err := fmt.Errorf(errString)
		return err
	}

	key := GenNLBKey(nsId, mcisId, resourceId)
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
func DelAllNLB(nsId string, mcisId string, subString string, forceFlag string) (common.IdList, error) {

	deletedResources := common.IdList{}
	deleteStatus := ""

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return deletedResources, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return deletedResources, err
	}

	resourceIdList, err := ListNLBId(nsId, mcisId)
	if err != nil {
		return deletedResources, err
	}

	// Do not return error when len(resourceIdList) == 0
	// if len(resourceIdList) == 0 {
	// 	errString := "There is no NLB in " + nsId
	// 	err := fmt.Errorf(errString)
	// 	common.CBLog.Error(err)
	// 	return deletedResources, err
	// }

	for _, v := range resourceIdList {
		// if subString is provided, check the resourceId contains the subString.
		if subString == "" || strings.Contains(v, subString) {
			deleteStatus = ""

			err := DelNLB(nsId, mcisId, v, forceFlag)

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

// GetNLBHealth queries the health status of NLB to CB-Spider, and returns it to user
func GetNLBHealth(nsId string, mcisId string, nlbId string) (TbNLBHealthInfo, error) {
	fmt.Println("=========================== GetNLBHealth")

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return TbNLBHealthInfo{}, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return TbNLBHealthInfo{}, err
	}

	err = common.CheckString(nlbId)
	if err != nil {
		common.CBLog.Error(err)
		return TbNLBHealthInfo{}, err
	}

	check, err := CheckNLB(nsId, mcisId, nlbId)

	if !check {
		err := fmt.Errorf("The nlb " + nlbId + " does not exist.")
		return TbNLBHealthInfo{}, err
	}

	if err != nil {
		err := fmt.Errorf("Failed to check the existence of the nlb " + nlbId + ".")
		return TbNLBHealthInfo{}, err
	}

	nlb, err := GetNLB(nsId, mcisId, nlbId)
	if err != nil {
		err := fmt.Errorf("Failed to get the NLB " + nlbId + ".")
		return TbNLBHealthInfo{}, err
	}

	tempReq := common.SpiderConnectionName{}
	tempReq.ConnectionName = nlb.ConnectionName

	var tempSpiderNLBHealthInfo *SpiderNLBHealthInfoWrapper

	if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

		client := resty.New().SetCloseConnection(true)
		client.SetAllowGetMethodPayload(true)

		// fmt.Println("tempReq:")                             // for debug
		// payload, _ := json.MarshalIndent(tempReq, "", "  ") // for debug
		// fmt.Print(string(payload))                          // for debug

		req := client.R().
			SetHeader("Content-Type", "application/json").
			SetBody(tempReq).
			SetResult(&SpiderNLBHealthInfoWrapper{}) // or SetResult(AuthSuccess{}).
			//SetError(&AuthError{}).       // or SetError(AuthError{}).

		var resp *resty.Response
		var err error

		var url string
		url = fmt.Sprintf("%s/nlb/%s/health", common.SpiderRestUrl, nlb.CspNLBName)
		resp, err = req.Get(url)

		if err != nil {
			common.CBLog.Error(err)
			err := fmt.Errorf("an error occurred while requesting to CB-Spider")
			return TbNLBHealthInfo{}, err
		}

		fmt.Println("HTTP Status code: " + strconv.Itoa(resp.StatusCode()))
		switch {
		case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
			err := fmt.Errorf(string(resp.Body()))
			common.CBLog.Error(err)
			return TbNLBHealthInfo{}, err
		}

		tempSpiderNLBHealthInfo = resp.Result().(*SpiderNLBHealthInfoWrapper)

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

	result := TbNLBHealthInfo{}

	if tempSpiderNLBHealthInfo.Healthinfo.HealthyVMs != nil {
		for _, v := range *tempSpiderNLBHealthInfo.Healthinfo.HealthyVMs {
			vm, err := FindTbVmByCspId(nsId, mcisId, v.NameId)
			if err != nil {
				return TbNLBHealthInfo{}, err
			}

			result.HealthyVMs = append(result.HealthyVMs, vm.Id)
		}
	}

	if tempSpiderNLBHealthInfo.Healthinfo.UnHealthyVMs != nil {
		for _, v := range *tempSpiderNLBHealthInfo.Healthinfo.UnHealthyVMs {
			vm, err := FindTbVmByCspId(nsId, mcisId, v.NameId)
			if err != nil {
				return TbNLBHealthInfo{}, err
			}

			result.UnHealthyVMs = append(result.UnHealthyVMs, vm.Id)
		}
	}

	result.AllVMs = append(result.AllVMs, result.HealthyVMs...)
	result.AllVMs = append(result.AllVMs, result.UnHealthyVMs...)
	/*
		// cb-store
		// Key := common.GenResourceKey(nsId, common.StrNLB, content.Id)
		Key := GenNLBKey(nsId, mcisId, content.Id)
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
	*/

	return result, nil
}

// AddNLBVMs accepts VM addition request, adds VM to NLB, and returns an updated TB NLB object
func AddNLBVMs(nsId string, mcisId string, resourceId string, u *TbNLBAddRemoveVMReq) (TbNLBInfo, error) {
	fmt.Println("=========================== AddNLBVMs")

	err := common.CheckString(nsId)
	if err != nil {
		temp := TbNLBInfo{}
		common.CBLog.Error(err)
		return temp, err
	}

	err = common.CheckString(mcisId)
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

	check, err := CheckNLB(nsId, mcisId, resourceId)

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

	nlb, err := GetNLB(nsId, mcisId, resourceId)
	if err != nil {
		temp := TbNLBInfo{}
		err := fmt.Errorf("Failed to get the nlb object " + resourceId + ".")
		return temp, err
	}

	tempReq := SpiderNLBAddRemoveVMReqInfoWrapper{}
	tempReq.ConnectionName = nlb.ConnectionName

	for _, v := range u.TargetGroup.VMs {
		vm, err := GetVmObject(nsId, mcisId, v)
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
	// fmt.Printf("tempReq.ReqInfo.SubGroup.VMs: %s \n", tempReq.ReqInfo.SubGroup.VMs) // for debug
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

			tempReq.ReqInfo.SubGroup = append(tempReq.ReqInfo.SubGroup, vm.IdByCSP)
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

	content := TbNLBInfo{
		Id:             nlb.Name,
		Name:           nlb.Name,
		ConnectionName: nlb.ConnectionName,
		Type:           tempSpiderNLBInfo.Type,
		Scope:          tempSpiderNLBInfo.Scope,
		Listener: TbNLBListenerInfo{
			Protocol:     tempSpiderNLBInfo.Listener.Protocol,
			IP:           tempSpiderNLBInfo.Listener.IP,
			Port:         tempSpiderNLBInfo.Listener.Port,
			DNSName:      tempSpiderNLBInfo.Listener.DNSName,
			KeyValueList: tempSpiderNLBInfo.Listener.KeyValueList,
		},
		HealthChecker: TbNLBHealthCheckerInfo{
			Protocol:     tempSpiderNLBInfo.HealthChecker.Protocol,
			Port:         tempSpiderNLBInfo.HealthChecker.Port,
			Interval:     tempSpiderNLBInfo.HealthChecker.Interval,
			Timeout:      tempSpiderNLBInfo.HealthChecker.Timeout,
			Threshold:    tempSpiderNLBInfo.HealthChecker.Threshold,
			KeyValueList: tempSpiderNLBInfo.HealthChecker.KeyValueList,
		},
		CspNLBId:             tempSpiderNLBInfo.IId.SystemId,
		CspNLBName:           tempSpiderNLBInfo.IId.NameId,
		CreatedTime:          tempSpiderNLBInfo.CreatedTime,
		Description:          nlb.Description,
		KeyValueList:         tempSpiderNLBInfo.KeyValueList,
		AssociatedObjectList: []string{},
		TargetGroup: TbNLBTargetGroupInfo{
			Protocol:   tempSpiderNLBInfo.VMGroup.Protocol,
			Port:       tempSpiderNLBInfo.VMGroup.Port,
			SubGroupId: nlb.TargetGroup.SubGroupId,
			// VMs:          vmIDs,
			KeyValueList: tempSpiderNLBInfo.VMGroup.KeyValueList,
		},
	}
	content.TargetGroup.VMs = append(content.TargetGroup.VMs, nlb.TargetGroup.VMs...)
	content.TargetGroup.VMs = append(content.TargetGroup.VMs, u.TargetGroup.VMs...)

	// cb-store
	// Key := common.GenResourceKey(nsId, common.StrNLB, content.Id)
	Key := GenNLBKey(nsId, mcisId, content.Id)
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
func RemoveNLBVMs(nsId string, mcisId string, resourceId string, u *TbNLBAddRemoveVMReq) error {
	fmt.Println("=========================== RemoveNLBVMs")

	err := common.CheckString(nsId)
	if err != nil {
		// temp := TbNLBInfo{}
		common.CBLog.Error(err)
		return err
	}

	err = common.CheckString(mcisId)
	if err != nil {
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

	check, err := CheckNLB(nsId, mcisId, resourceId)

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

	nlb, err := GetNLB(nsId, mcisId, resourceId)
	if err != nil {
		// temp := TbNLBInfo{}
		err := fmt.Errorf("Failed to get the nlb object " + resourceId + ".")
		return err
	}

	tempReq := SpiderNLBAddRemoveVMReqInfoWrapper{}
	tempReq.ConnectionName = nlb.ConnectionName

	// fmt.Printf("u.TargetGroup.VMs: %s \n", u.TargetGroup.VMs) // for debug

	for _, v := range u.TargetGroup.VMs {
		vm, err := GetVmObject(nsId, mcisId, v)
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

	// fmt.Printf("tempReq.ReqInfo.SubGroup.VMs: %s \n", tempReq.ReqInfo.VMs) // for debug
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

			tempReq.ReqInfo.SubGroup = append(tempReq.ReqInfo.SubGroup, vm.IdByCSP)
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

		content.TargetGroup.Port = tempSpiderNLBInfo.SubGroup.Port
		content.TargetGroup.Protocol = tempSpiderNLBInfo.SubGroup.Protocol
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
	Key := GenNLBKey(nsId, mcisId, nlb.Id)
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
