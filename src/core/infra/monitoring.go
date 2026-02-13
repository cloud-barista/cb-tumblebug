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
		Port:        strconv.Itoa(sshPort),
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

		// fmt.Println("HTTP Status code: " + strconv.Itoa(res.StatusCode))
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
	if len(vmList) == 0 {
		err := fmt.Errorf("MCI %s has no VMs to install monitoring agent (status: Empty)", mciId)
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

// GetMonitoringData retrieves monitoring data from CB-Dragonfly for all VMs in an MCI
// Returns a consolidated response with metrics for each VM
func GetMonitoringData(nsId string, mciId string, metric string) (model.MonResultSimpleResponse, error) {
	// Initialize response object
	content := model.MonResultSimpleResponse{
		NsId:  nsId,
		MciId: mciId,
	}

	// Validate namespace ID
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("Invalid namespace ID format")
		return content, fmt.Errorf("invalid namespace ID: %w", err)
	}

	// Validate MCI ID
	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("Invalid MCI ID format")
		return content, fmt.Errorf("invalid MCI ID: %w", err)
	}

	// Check if MCI exists
	check, err := CheckMci(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msgf("Error checking MCI existence: %s/%s", nsId, mciId)
		return content, fmt.Errorf("error checking MCI existence: %w", err)
	}
	if !check {
		log.Error().Msgf("MCI does not exist: %s/%s", nsId, mciId)
		return content, fmt.Errorf("MCI %s does not exist in namespace %s", mciId, nsId)
	}

	// Get the list of VMs in the MCI
	vmList, err := ListVmId(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to list VMs for MCI: %s/%s", nsId, mciId)
		return content, fmt.Errorf("failed to list VMs: %w", err)
	}

	// If no VMs found, return empty result
	if len(vmList) == 0 {
		log.Warn().Msgf("No VMs found in MCI: %s/%s", nsId, mciId)
		return content, nil
	}

	log.Info().Msgf("Retrieving %s metrics for %d VMs in MCI %s/%s", metric, len(vmList), nsId, mciId)

	// Setup for concurrent monitoring requests
	var wg sync.WaitGroup
	var resultArray []model.MonResultSimple
	method := "GET"

	// Process each VM concurrently
	for _, vmId := range vmList {
		wg.Add(1)

		// Get VM IP address
		vmIp, _, _, err := GetVmIp(nsId, mciId, vmId)
		if err != nil {
			log.Error().Err(err).Msgf("Failed to get IP for VM: %s/%s/%s", nsId, mciId, vmId)

			// Create a result for this VM with error information
			errResult := model.MonResultSimple{
				VmId:   vmId,
				Metric: metric,
				Value:  "Error",
				Err:    fmt.Sprintf("Failed to get VM IP: %v", err),
			}
			resultArray = append(resultArray, errResult)

			wg.Done() // Decrement counter for this VM
			continue  // Continue to next VM
		}

		// Construct the API path for this VM's monitoring data
		cmd := fmt.Sprintf("/ns/%s/mci/%s/vm/%s/agent_ip/%s/metric/%s/ondemand-monitoring-info",
			nsId, mciId, vmId, vmIp, metric)

		// Make asynchronous call to CB-Dragonfly
		go CallGetMonitoringAsync(&wg, nsId, mciId, vmId, vmIp, method, metric, cmd, &resultArray)
	}

	// Wait for all monitoring requests to complete
	wg.Wait()

	// Add results to response object
	content.MciMonitoring = resultArray

	// Log summary of results
	successCount := 0
	errorCount := 0
	for _, result := range resultArray {
		if result.Err != "" {
			errorCount++
		} else {
			successCount++
		}
	}

	log.Info().Msgf("Monitoring data collection complete: %d successful, %d failed",
		successCount, errorCount)
	if errorCount > 0 {
		return content, fmt.Errorf("%d VMs failed to retrieve monitoring data", errorCount)
	}

	return content, nil
}

// CallGetMonitoringAsync makes asynchronous HTTP call to CB-Dragonfly for monitoring data
// and appends the result to the provided result array
func CallGetMonitoringAsync(wg *sync.WaitGroup, nsID string, mciID string, vmID string, vmIP string, method string, metric string, cmd string, returnResult *[]model.MonResultSimple) {
	defer wg.Done() // Ensure WaitGroup counter is decremented when function exits

	log.Info().Msg("[Call CB-DF] " + mciID + "/" + vmID + "(" + vmIP + ")")

	// Initialize result object
	resultTmp := model.MonResultSimple{
		VmId:   vmID,
		Metric: metric,
	}

	// Prepare HTTP request
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
		// Handle request creation error
		log.Error().Err(err).Msg("Failed to create HTTP request")
		resultTmp.Value = "Error"
		resultTmp.Err = fmt.Sprintf("HTTP request creation error: %v", err)
		*returnResult = append(*returnResult, resultTmp)
		return
	}

	// Execute HTTP request

	log.Debug().Msg("Call CB-DF Result (" + mciID + "," + vmID + ") ")
	res, err := client.Do(req)
	if err != nil {
		// Handle request execution error
		log.Error().Err(err).Msg("Failed to execute HTTP request")
		resultTmp.Value = "Error"
		resultTmp.Err = fmt.Sprintf("HTTP request execution error: %v", err)
		*returnResult = append(*returnResult, resultTmp)
		return
	}

	// Check status code
	if res.StatusCode < 200 || res.StatusCode >= 400 {
		errMsg := fmt.Sprintf("HTTP request failed with status code: %d", res.StatusCode)
		log.Error().Msg(errMsg)
		resultTmp.Value = "Error"
		resultTmp.Err = errMsg
		*returnResult = append(*returnResult, resultTmp)
		return
	}

	// Read response body
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		// Handle response reading error
		log.Error().Err(err).Msg("Failed to read response body")
		resultTmp.Value = "Error"
		resultTmp.Err = fmt.Sprintf("Response body read error: %v", err)
		*returnResult = append(*returnResult, resultTmp)
		return
	}

	response := string(body)

	// Validate JSON response
	if !gjson.Valid(response) {
		// Handle invalid JSON response
		log.Error().Msg("Invalid JSON response from monitoring server")
		log.Debug().Msgf("Raw response: %s", response)
		resultTmp.Value = "Invalid JSON response"
		resultTmp.Err = "Response validation error: Invalid JSON format"
		*returnResult = append(*returnResult, resultTmp)
		return
	}

	// Extract metric value based on metric type
	var result string
	var metricError string

	switch metric {
	case model.MonMetricCpu:
		value := gjson.Get(response, "values.cpu_utilization")
		if !value.Exists() {
			metricError = "CPU utilization data not found in response"
			result = "N/A"
		} else {
			result = value.String()
		}
	case model.MonMetricMem:
		value := gjson.Get(response, "values.mem_utilization")
		if !value.Exists() {
			metricError = "Memory utilization data not found in response"
			result = "N/A"
		} else {
			result = value.String()
		}
	case model.MonMetricDisk:
		value := gjson.Get(response, "values.disk_utilization")
		if !value.Exists() {
			metricError = "Disk utilization data not found in response"
			result = "N/A"
		} else {
			result = value.String()
		}
	case model.MonMetricNet:
		value := gjson.Get(response, "values.bytes_out")
		if !value.Exists() {
			metricError = "Network bytes out data not found in response"
			result = "N/A"
		} else {
			result = value.String()
		}
	default:
		// For unknown metrics, return the entire response
		result = response
	}

	// Set result value
	resultTmp.Value = result

	// Set error if metric data was not found
	if metricError != "" {
		resultTmp.Err = metricError
		log.Debug().Msgf("Monitoring data issue: %s", metricError)
	} else {
		log.Debug().Msgf("Successfully retrieved monitoring data: %s", result)
	}

	// Append result to return array
	*returnResult = append(*returnResult, resultTmp)
}
