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
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	validator "github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"
	"github.com/tidwall/gjson"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
)

func DFMonAgentInstallReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(model.MonAgentInstallReq)

	err := common.CheckString(u.NsId)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.NsId, "nsId", "NsId", err.Error(), "")
	}

	err = common.CheckString(u.MciId)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.MciId, "mciId", "MciId", err.Error(), "")
	}

	err = common.CheckString(u.VmId)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.VmId, "vmId", "VmId", err.Error(), "")
	}
}

// Module for checking CB-Dragonfly endpoint (call get config)
func CheckDragonflyEndpoint() error {

	cmd := "/config"

	url := model.DragonflyRestUrl + cmd
	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)

	if err != nil {
		log.Err(err).Msg("")
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		log.Err(err).Msg("")
		return err
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Err(err).Msg("")
		return err
	}
	defer res.Body.Close()

	log.Debug().Msg(string(body))
	return nil

}

// CallMonitoringAsync is func to call CB-Dragonfly monitoring framework
func CallMonitoringAsync(wg *sync.WaitGroup, nsID string, mciID string, mciServiceType string, vmID string, givenUserName string, method string, cmd string, returnResult *[]model.SshCmdResult) {

	defer wg.Done() //goroutin sync done

	vmIP, _, sshPort, err := GetVmIp(nsID, mciID, vmID)
	errStr := ""
	if err != nil {
		log.Error().Err(err).Msg("")
		errStr += "/ " + err.Error()
	}
	userName, privateKey, err := VerifySshUserName(nsID, mciID, vmID, vmIP, sshPort, givenUserName)
	if err != nil {
		log.Error().Err(err).Msg("")
		errStr += "/ " + err.Error()
	}
	log.Debug().Msg("[CallMonitoringAsync] " + mciID + "/" + vmID + "(" + vmIP + ")" + "with userName:" + userName)

	// set vm MonAgentStatus = "installing" (to avoid duplicated requests)
	vmInfoTmp, _ := GetVmObject(nsID, mciID, vmID)
	vmInfoTmp.MonAgentStatus = "installing"
	UpdateVmInfo(nsID, mciID, vmInfoTmp)

	if mciServiceType == "" {
		mciServiceType = model.StrMCI
	}

	url := model.DragonflyRestUrl + cmd
	log.Debug().Msg("\n[Calling DRAGONFLY] START")
	log.Debug().Msg("VM:" + nsID + "/" + mciID + "/" + vmID + ", URL:" + url + ", userName:" + userName + ", cspType:" + vmInfoTmp.ConnectionConfig.ProviderName + ", service_type:" + mciServiceType)

	requestBody := model.DfAgentInstallReq{
		NsId:        nsID,
		MciId:       mciID,
		VmId:        vmID,
		PublicIp:    vmIP,
		Port:        sshPort,
		UserName:    userName,
		SshKey:      privateKey,
		CspType:     vmInfoTmp.ConnectionConfig.ProviderName,
		ServiceType: mciServiceType,
	}
	if requestBody.SshKey == "" {
		common.PrintJsonPretty(requestBody)
		err = fmt.Errorf("/request body to install monitoring agent: privateKey is empty/")
		log.Error().Err(err).Msg("")
		errStr += "/ " + err.Error()
	}

	payload, err := json.Marshal(requestBody)
	if err != nil {
		log.Error().Err(err).Msg("")
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
		log.Error().Err(err).Msg("")
		errStr += "/ " + err.Error()
	}

	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)

	result := ""

	log.Debug().Msg("Called CB-DRAGONFLY API")
	if err != nil {
		log.Error().Err(err).Msg("")
		errStr += "/ " + err.Error()
	} else {

		body, err := io.ReadAll(res.Body)
		if err != nil {
			log.Error().Err(err).Msg("")
			errStr += "/ " + err.Error()
		}
		defer res.Body.Close()

		fmt.Println("HTTP Status code: " + strconv.Itoa(res.StatusCode))
		switch {
		case res.StatusCode >= 400 || res.StatusCode < 200:
			err = fmt.Errorf("CB-DF HTTP Status: " + strconv.Itoa(res.StatusCode) + " / " + string(body))
			log.Error().Err(err).Msg("")
			errStr += "/ " + err.Error()
		}

		result = string(body)
	}

	//wg.Done() //goroutin sync done

	//vmInfoTmp, _ := GetVmObject(nsID, mciID, vmID)

	sshResultTmp := model.SshCmdResult{}
	sshResultTmp.MciId = mciID
	sshResultTmp.VmId = vmID
	sshResultTmp.VmIp = vmIP

	sshResultTmp.Stdout = make(map[int]string)
	sshResultTmp.Stderr = make(map[int]string)

	if err != nil || errStr != "" {
		log.Error().Err(err).Msgf("[Monitoring Agent deployment errors] %s", errStr)
		sshResultTmp.Stderr[0] = errStr
		sshResultTmp.Err = err
		*returnResult = append(*returnResult, sshResultTmp)
		vmInfoTmp.MonAgentStatus = "failed"
	} else {
		fmt.Println("Result: " + result)
		sshResultTmp.Stdout[0] = result
		sshResultTmp.Err = nil
		*returnResult = append(*returnResult, sshResultTmp)
		vmInfoTmp.MonAgentStatus = "installed"
	}

	UpdateVmInfo(nsID, mciID, vmInfoTmp)

}

func InstallMonitorAgentToMci(nsId string, mciId string, mciServiceType string, req *model.MciCmdReq) (model.AgentInstallContentWrapper, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := model.AgentInstallContentWrapper{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		temp := model.AgentInstallContentWrapper{}
		log.Error().Err(err).Msg("")
		return temp, err
	}
	check, _ := CheckMci(nsId, mciId)

	if !check {
		temp := model.AgentInstallContentWrapper{}
		err := fmt.Errorf("The mci " + mciId + " does not exist.")
		return temp, err
	}

	content := model.AgentInstallContentWrapper{}

	//install script
	cmd := "/agent"

	vmList, err := ListVmId(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return content, err
	}

	log.Debug().Msg("[Install agent for each VM]")

	//goroutin sync wg
	var wg sync.WaitGroup

	var resultArray []model.SshCmdResult

	method := "POST"

	for _, v := range vmList {
		vmObjTmp, _ := GetVmObject(nsId, mciId, v)
		fmt.Println("MonAgentStatus : " + vmObjTmp.MonAgentStatus)

		// Request agent installation (skip if in installing or installed status)
		if vmObjTmp.MonAgentStatus != "installed" && vmObjTmp.MonAgentStatus != "installing" {

			// Avoid RunRemoteCommand to not ready VM
			if err == nil {
				wg.Add(1)
				go CallMonitoringAsync(&wg, nsId, mciId, mciServiceType, v, req.UserName, method, cmd, &resultArray)
			} else {
				log.Error().Err(err).Msg("")
			}

		}
	}
	wg.Wait() //goroutin sync wg

	for _, v := range resultArray {

		resultTmp := model.AgentInstallContent{}
		resultTmp.MciId = mciId
		resultTmp.VmId = v.VmId
		resultTmp.VmIp = v.VmIp
		resultTmp.Result = v.Stdout[0]
		content.ResultArray = append(content.ResultArray, resultTmp)
	}

	common.PrintJsonPretty(content)

	return content, nil

}

// SetMonitoringAgentStatusInstalled is func to Set Monitoring Agent Status Installed
func SetMonitoringAgentStatusInstalled(nsId string, mciId string, vmId string) error {
	targetStatus := "installed"
	return UpdateMonitoringAgentStatusManually(nsId, mciId, vmId, targetStatus)
}

// UpdateMonitoringAgentStatusManually is func to Update Monitoring Agent Installation Status Manually
func UpdateMonitoringAgentStatusManually(nsId string, mciId string, vmId string, targetStatus string) error {

	vmInfoTmp, err := GetVmObject(nsId, mciId, vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	// set vm MonAgentStatus
	vmInfoTmp.MonAgentStatus = targetStatus
	UpdateVmInfo(nsId, mciId, vmInfoTmp)

	//TODO: add validation for monitoring

	return nil
}

// GetMonitoringData func retrieves monitoring data from cb-dragonfly
func GetMonitoringData(nsId string, mciId string, metric string) (model.MonResultSimpleResponse, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := model.MonResultSimpleResponse{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		temp := model.MonResultSimpleResponse{}
		log.Error().Err(err).Msg("")
		return temp, err
	}
	check, _ := CheckMci(nsId, mciId)

	if !check {
		temp := model.MonResultSimpleResponse{}
		err := fmt.Errorf("The mci " + mciId + " does not exist.")
		return temp, err
	}

	content := model.MonResultSimpleResponse{}

	vmList, err := ListVmId(nsId, mciId)
	if err != nil {
		//log.Error().Err(err).Msg("")
		return content, err
	}

	//goroutin sync wg
	var wg sync.WaitGroup

	var resultArray []model.MonResultSimple

	method := "GET"

	for _, vmId := range vmList {
		wg.Add(1)

		vmIp, _, _, err := GetVmIp(nsId, mciId, vmId)
		if err != nil {
			log.Error().Err(err).Msg("")
			wg.Done()
			// continue to next vm even if error occurs
		} else {
			// DF: Get vm on-demand monitoring metric info
			// Path Param: /ns/:nsId/mci/:mciId/vm/:vmId/agent_ip/:agent_ip/metric/:metric_name/ondemand-monitoring-info
			cmd := "/ns/" + nsId + "/mci/" + mciId + "/vm/" + vmId + "/agent_ip/" + vmIp + "/metric/" + metric + "/ondemand-monitoring-info"
			go CallGetMonitoringAsync(&wg, nsId, mciId, vmId, vmIp, method, metric, cmd, &resultArray)
		}
	}
	wg.Wait() //goroutin sync wg

	content.NsId = nsId
	content.MciId = mciId
	for _, v := range resultArray {
		content.MciMonitoring = append(content.MciMonitoring, v)
	}

	fmt.Printf("%+v\n", content)

	return content, nil

}

func CallGetMonitoringAsync(wg *sync.WaitGroup, nsID string, mciID string, vmID string, vmIP string, method string, metric string, cmd string, returnResult *[]model.MonResultSimple) {

	defer wg.Done() //goroutin sync done

	log.Info().Msg("[Call CB-DF] " + mciID + "/" + vmID + "(" + vmIP + ")")

	var response string
	var errStr string
	var result string
	var err error

	url := model.DragonflyRestUrl + cmd
	log.Debug().Msg("URL: " + url)

	responseLimit := 8
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: time.Duration(responseLimit) * time.Minute,
	}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		log.Error().Err(err).Msg("")
		errStr = err.Error()
	}

	fmt.Print("[Call CB-DF Result (" + mciID + "," + vmID + ")] ")
	res, err := client.Do(req)

	if err != nil {
		log.Error().Err(err).Msg("")
		errStr = err.Error()
	} else {
		fmt.Println("HTTP Status code: " + strconv.Itoa(res.StatusCode))
		switch {
		case res.StatusCode >= 400 || res.StatusCode < 200:
			err1 := fmt.Errorf("HTTP Status: not in 200-399")
			log.Error().Err(err1).Msg("")
			errStr = err1.Error()
		}

		body, err2 := io.ReadAll(res.Body)
		if err2 != nil {
			log.Error().Err(err2).Msg("")
			errStr = err2.Error()
		}
		defer res.Body.Close()
		response = string(body)
	}

	if !gjson.Valid(response) {
		log.Debug().Msg("!gjson.Valid(response)")
	}

	switch metric {
	case model.MonMetricCpu:
		value := gjson.Get(response, "values.cpu_utilization")
		result = value.String()
	case model.MonMetricMem:
		value := gjson.Get(response, "values.mem_utilization")
		result = value.String()
	case model.MonMetricDisk:
		value := gjson.Get(response, "values.disk_utilization")
		result = value.String()
	case model.MonMetricNet:
		value := gjson.Get(response, "values.bytes_out")
		result = value.String()
	default:
		result = response
	}

	//wg.Done() //goroutin sync done

	ResultTmp := model.MonResultSimple{}
	ResultTmp.VmId = vmID
	ResultTmp.Metric = metric

	if err != nil {
		fmt.Println("CB-DF Error message: " + errStr)
		ResultTmp.Value = errStr
		ResultTmp.Err = err.Error()
		*returnResult = append(*returnResult, ResultTmp)
	} else {
		log.Debug().Msg("CB-DF Result: " + result)
		ResultTmp.Value = result
		*returnResult = append(*returnResult, ResultTmp)
	}

}
