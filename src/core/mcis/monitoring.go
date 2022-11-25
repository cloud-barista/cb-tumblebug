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
	"os"
	"time"

	validator "github.com/go-playground/validator/v10"
	"github.com/tidwall/gjson"

	"fmt"
	"io/ioutil"

	//"log"

	"strconv"
	"strings"

	// REST API (echo)
	"net/http"

	"sync"

	df_pb "github.com/cloud-barista/cb-dragonfly/pkg/api/grpc/protobuf/cbdragonfly"
	df_api "github.com/cloud-barista/cb-dragonfly/pkg/api/grpc/request"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
)

const (
	monMetricAll     string = "all"
	monMetricCpu     string = "cpu"
	monMetricCpufreq string = "cpufreq"
	monMetricMem     string = "mem"
	monMetricNet     string = "net"
	monMetricSwap    string = "swap"
	monMetricDisk    string = "disk"
	monMetricDiskio  string = "diskio"
)

// MonAgentInstallReq struct
type MonAgentInstallReq struct {
	NsId     string `json:"nsId,omitempty"`
	McisId   string `json:"mcisId,omitempty"`
	VmId     string `json:"vmId,omitempty"`
	PublicIp string `json:"publicIp,omitempty"`
	Port     string `json:"port,omitempty"`
	UserName string `json:"userName,omitempty"`
	SshKey   string `json:"sshKey,omitempty"`
	CspType  string `json:"cspType,omitempty"`
}

func DFMonAgentInstallReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(MonAgentInstallReq)

	err := common.CheckString(u.NsId)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.NsId, "nsId", "NsId", err.Error(), "")
	}

	err = common.CheckString(u.McisId)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.McisId, "mcisId", "McisId", err.Error(), "")
	}

	err = common.CheckString(u.VmId)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.VmId, "vmId", "VmId", err.Error(), "")
	}
}

/*
type DfTelegrafMetric struct {
	Name      string                 `json:"name"`
	Tags      map[string]interface{} `json:"tags"`
	Fields    map[string]interface{} `json:"fields"`
	Timestamp int64                  `json:"timestamp"`
	TagInfo   map[string]interface{} `json:"tagInfo"`
}
*/

// MonResultSimple struct is for containing vm monitoring results
type MonResultSimple struct {
	Metric string `json:"metric"`
	VmId   string `json:"vmId"`
	Value  string `json:"value"`
	Err    string `json:"err"`
}

// MonResultSimpleResponse struct is for containing Mcis monitoring results
type MonResultSimpleResponse struct {
	NsId           string            `json:"nsId"`
	McisId         string            `json:"mcisId"`
	McisMonitoring []MonResultSimple `json:"mcisMonitoring"`
}

// Module for checking CB-Dragonfly endpoint (call get config)
func CheckDragonflyEndpoint() error {
	if os.Getenv("DRAGONFLY_CALL_METHOD") == "REST" {
		cmd := "/config"

		url := common.DragonflyRestUrl + cmd
		method := "GET"

		client := &http.Client{}
		req, err := http.NewRequest(method, url, nil)

		if err != nil {
			fmt.Println(err)
			return err
		}
		res, err := client.Do(req)
		if err != nil {
			fmt.Println(err)
			return err
		}

		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			fmt.Println(err)
			return err
		}
		defer res.Body.Close()

		fmt.Println(string(body))
		return nil
	} else {
		monApi := df_api.InitMonitoringAPI()
		err := monApi.Open()
		if err != nil {
			common.CBLog.Error("failed to initialize grpc client, %s", err.Error())
		}
		defer monApi.Close()

		result, err := monApi.GetMonitoringConfig()
		if err != nil {
			return err
		}
		fmt.Println(result)
		return nil
	}
}

// monAgentInstallReq is struct for CB-Dragonfly monitoring agent installation request
type monAgentInstallReq struct {
	NsId        string `json:"ns_id"`
	McisId      string `json:"mcis_id"`
	VmId        string `json:"vm_id"`
	PublicIp    string `json:"public_ip"`
	UserName    string `json:"user_name"`
	SshKey      string `json:"ssh_key"`
	CspType     string `json:"cspType"`
	ServiceType string `json:"service_type"`
	Port        string `json:"port"`
}

// CallMonitoringAsync is func to call CB-Dragonfly monitoring framework
func CallMonitoringAsync(wg *sync.WaitGroup, nsID string, mcisID string, mcisServiceType string, vmID string, givenUserName string, method string, cmd string, returnResult *[]SshCmdResult) {

	defer wg.Done() //goroutin sync done

	vmIP, sshPort := GetVmIp(nsID, mcisID, vmID)
	userName, privateKey, err := VerifySshUserName(nsID, mcisID, vmID, vmIP, sshPort, givenUserName)
	errStr := ""
	if err != nil {
		common.CBLog.Error(err)
		errStr += "/ " + err.Error()
	}
	fmt.Println("[CallMonitoringAsync] " + mcisID + "/" + vmID + "(" + vmIP + ")" + "with userName:" + userName)

	// set vm MonAgentStatus = "installing" (to avoid duplicated requests)
	vmInfoTmp, _ := GetVmObject(nsID, mcisID, vmID)
	vmInfoTmp.MonAgentStatus = "installing"
	UpdateVmInfo(nsID, mcisID, vmInfoTmp)

	if mcisServiceType == "" {
		mcisServiceType = common.StrMCIS
	}

	url := common.DragonflyRestUrl + cmd
	fmt.Println("\n[Calling DRAGONFLY] START")
	fmt.Println("VM:" + nsID + "/" + mcisID + "/" + vmID + ", URL:" + url + ", userName:" + userName + ", cspType:" + vmInfoTmp.Location.CloudType + ", service_type:" + mcisServiceType)

	tempReq := monAgentInstallReq{
		NsId:        nsID,
		McisId:      mcisID,
		VmId:        vmID,
		PublicIp:    vmIP,
		Port:        sshPort,
		UserName:    userName,
		SshKey:      privateKey,
		CspType:     vmInfoTmp.Location.CloudType,
		ServiceType: mcisServiceType,
	}
	if tempReq.SshKey == "" {
		common.PrintJsonPretty(tempReq)
		err = fmt.Errorf("/request body to install monitoring agent: privateKey is empty/")
		common.CBLog.Error(err)
		errStr += "/ " + err.Error()
	}

	payload, err := json.Marshal(tempReq)
	if err != nil {
		common.CBLog.Error(err)
		errStr += "/ " + err.Error()
	}

	responseLimit := 8
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: time.Duration(responseLimit) * time.Minute,
	}
	req, err := http.NewRequest(method, url, strings.NewReader(string(payload)))

	if err != nil {
		common.CBLog.Error(err)
		errStr += "/ " + err.Error()
	}

	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)

	result := ""

	fmt.Println("Called CB-DRAGONFLY API")
	if err != nil {
		common.CBLog.Error(err)
		errStr += "/ " + err.Error()
	} else {

		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			common.CBLog.Error(err)
			errStr += "/ " + err.Error()
		}
		defer res.Body.Close()

		fmt.Println("HTTP Status code: " + strconv.Itoa(res.StatusCode))
		switch {
		case res.StatusCode >= 400 || res.StatusCode < 200:
			err = fmt.Errorf("CB-DF HTTP Status: " + strconv.Itoa(res.StatusCode) + " / " + string(body))
			common.CBLog.Error(err)
			errStr += "/ " + err.Error()
		}

		result = string(body)
	}

	//wg.Done() //goroutin sync done

	//vmInfoTmp, _ := GetVmObject(nsID, mcisID, vmID)

	sshResultTmp := SshCmdResult{}
	sshResultTmp.McisId = mcisID
	sshResultTmp.VmId = vmID
	sshResultTmp.VmIp = vmIP

	if err != nil || errStr != "" {
		common.CBLog.Error("[Monitoring Agent deployment errors] " + errStr)
		sshResultTmp.Result = errStr
		sshResultTmp.Err = err
		*returnResult = append(*returnResult, sshResultTmp)
		vmInfoTmp.MonAgentStatus = "failed"
	} else {
		fmt.Println("Result: " + result)
		sshResultTmp.Result = result
		sshResultTmp.Err = nil
		*returnResult = append(*returnResult, sshResultTmp)
		vmInfoTmp.MonAgentStatus = "installed"
	}

	UpdateVmInfo(nsID, mcisID, vmInfoTmp)

}

func InstallMonitorAgentToMcis(nsId string, mcisId string, mcisServiceType string, req *McisCmdReq) (AgentInstallContentWrapper, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := AgentInstallContentWrapper{}
		common.CBLog.Error(err)
		return temp, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		temp := AgentInstallContentWrapper{}
		common.CBLog.Error(err)
		return temp, err
	}
	check, _ := CheckMcis(nsId, mcisId)

	if !check {
		temp := AgentInstallContentWrapper{}
		err := fmt.Errorf("The mcis " + mcisId + " does not exist.")
		return temp, err
	}

	content := AgentInstallContentWrapper{}

	//install script
	cmd := "/agent"

	vmList, err := ListVmId(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return content, err
	}

	fmt.Println("[Install agent for each VM]")

	//goroutin sync wg
	var wg sync.WaitGroup

	var resultArray []SshCmdResult

	method := "POST"

	for _, v := range vmList {
		vmObjTmp, _ := GetVmObject(nsId, mcisId, v)
		fmt.Println("MonAgentStatus : " + vmObjTmp.MonAgentStatus)

		// Request agent installation (skip if in installing or installed status)
		if vmObjTmp.MonAgentStatus != "installed" && vmObjTmp.MonAgentStatus != "installing" {

			// Avoid RunRemoteCommand to not ready VM
			if err == nil {
				wg.Add(1)
				go CallMonitoringAsync(&wg, nsId, mcisId, mcisServiceType, v, req.UserName, method, cmd, &resultArray)
			} else {
				common.CBLog.Error(err)
			}

		}
	}
	wg.Wait() //goroutin sync wg

	for _, v := range resultArray {

		resultTmp := AgentInstallContent{}
		resultTmp.McisId = mcisId
		resultTmp.VmId = v.VmId
		resultTmp.VmIp = v.VmIp
		resultTmp.Result = v.Result
		content.ResultArray = append(content.ResultArray, resultTmp)
	}

	common.PrintJsonPretty(content)

	return content, nil

}

// GetMonitoringData func retrieves monitoring data from cb-dragonfly
func GetMonitoringData(nsId string, mcisId string, metric string) (MonResultSimpleResponse, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := MonResultSimpleResponse{}
		common.CBLog.Error(err)
		return temp, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		temp := MonResultSimpleResponse{}
		common.CBLog.Error(err)
		return temp, err
	}
	check, _ := CheckMcis(nsId, mcisId)

	if !check {
		temp := MonResultSimpleResponse{}
		err := fmt.Errorf("The mcis " + mcisId + " does not exist.")
		return temp, err
	}

	content := MonResultSimpleResponse{}

	vmList, err := ListVmId(nsId, mcisId)
	if err != nil {
		//common.CBLog.Error(err)
		return content, err
	}

	//goroutin sync wg
	var wg sync.WaitGroup

	var resultArray []MonResultSimple

	method := "GET"

	for _, v := range vmList {
		wg.Add(1)

		vmId := v
		vmIp, _ := GetVmIp(nsId, mcisId, vmId)

		// DF: Get vm on-demand monitoring metric info
		// Path Param: /ns/:nsId/mcis/:mcisId/vm/:vmId/agent_ip/:agent_ip/metric/:metric_name/ondemand-monitoring-info
		cmd := "/ns/" + nsId + "/mcis/" + mcisId + "/vm/" + vmId + "/agent_ip/" + vmIp + "/metric/" + metric + "/ondemand-monitoring-info"

		go CallGetMonitoringAsync(&wg, nsId, mcisId, vmId, vmIp, method, metric, cmd, &resultArray)

	}
	wg.Wait() //goroutin sync wg

	content.NsId = nsId
	content.McisId = mcisId
	for _, v := range resultArray {
		content.McisMonitoring = append(content.McisMonitoring, v)
	}

	fmt.Printf("%+v\n", content)

	return content, nil

}

func CallGetMonitoringAsync(wg *sync.WaitGroup, nsID string, mcisID string, vmID string, vmIP string, method string, metric string, cmd string, returnResult *[]MonResultSimple) {

	defer wg.Done() //goroutin sync done

	fmt.Print("[Call CB-DF] ")

	var response string
	var errStr string
	var result string
	var err error
	if os.Getenv("DRAGONFLY_CALL_METHOD") == "REST" {
		url := common.DragonflyRestUrl + cmd
		fmt.Println("URL: " + url)

		responseLimit := 8
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Timeout: time.Duration(responseLimit) * time.Minute,
		}
		req, err := http.NewRequest(method, url, nil)
		if err != nil {
			common.CBLog.Error(err)
			errStr = err.Error()
		}

		fmt.Print("[Call CB-DF Result (" + mcisID + "," + vmID + ")] ")
		res, err := client.Do(req)

		if err != nil {
			common.CBLog.Error(err)
			errStr = err.Error()
		} else {
			fmt.Println("HTTP Status code: " + strconv.Itoa(res.StatusCode))
			switch {
			case res.StatusCode >= 400 || res.StatusCode < 200:
				err1 := fmt.Errorf("HTTP Status: not in 200-399")
				common.CBLog.Error(err1)
				errStr = err1.Error()
			}

			body, err2 := ioutil.ReadAll(res.Body)
			if err2 != nil {
				common.CBLog.Error(err2)
				errStr = err2.Error()
			}
			defer res.Body.Close()
			response = string(body)
		}

	} else {
		reqParams := df_pb.VMOnDemandMonQryRequest{
			NsId:    nsID,
			McisId:  mcisID,
			VmId:    vmID,
			AgentIp: vmIP,
		}

		monApi := df_api.InitMonitoringAPI()
		err := monApi.Open()
		if err != nil {
			common.CBLog.Error("failed to initialize grpc client, %s", err.Error())
		}
		defer monApi.Close()

		result, err := monApi.GetVMOnDemandMonInfo(metric, reqParams)
		if err != nil {
			common.CBLog.Error(err)
		}
		response = result
	}

	if !gjson.Valid(response) {
		fmt.Println("!gjson.Valid(response)")
	}

	switch metric {
	case monMetricCpu:
		value := gjson.Get(response, "values.cpu_utilization")
		result = value.String()
	case monMetricMem:
		value := gjson.Get(response, "values.mem_utilization")
		result = value.String()
	case monMetricDisk:
		value := gjson.Get(response, "values.disk_utilization")
		result = value.String()
	case monMetricNet:
		value := gjson.Get(response, "values.bytes_out")
		result = value.String()
	default:
		result = response
	}

	//wg.Done() //goroutin sync done

	ResultTmp := MonResultSimple{}
	ResultTmp.VmId = vmID
	ResultTmp.Metric = metric

	if err != nil {
		fmt.Println("CB-DF Error message: " + errStr)
		ResultTmp.Value = errStr
		ResultTmp.Err = err.Error()
		*returnResult = append(*returnResult, ResultTmp)
	} else {
		fmt.Println("CB-DF Result: " + result)
		ResultTmp.Value = result
		*returnResult = append(*returnResult, ResultTmp)
	}

}
