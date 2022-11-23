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
	"errors"

	"encoding/json"
	"fmt"
	"io/ioutil"

	//"log"
	"strconv"
	"strings"
	"time"

	//csv file handling

	"os"

	// REST API (echo)
	"net/http"

	"sync"

	"github.com/cloud-barista/cb-spider/interface/api"
	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
)

// MCIS Control

// ControlVmResult is struct for result of VM control
type ControlVmResult struct {
	VmId   string `json:"vmId"`
	Status string `json:"Status"`
	Error  error  `json:"Error"`
}

// ControlVmResultWrapper is struct for array of results of VM control
type ControlVmResultWrapper struct {
	ResultArray []ControlVmResult `json:"resultarray"`
}

// HandleMcisAction is func to handle actions to MCIS
func HandleMcisAction(nsId string, mcisId string, action string, force bool) (string, error) {
	action = common.ToLower(action)

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return "", err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return "", err
	}
	check, _ := CheckMcis(nsId, mcisId)

	if !check {
		err := fmt.Errorf("The mcis " + mcisId + " does not exist.")
		return err.Error(), err
	}

	fmt.Println("[Get MCIS requested action: " + action)
	if action == "suspend" {
		fmt.Println("[suspend MCIS]")

		err := ControlMcisAsync(nsId, mcisId, ActionSuspend, force)
		if err != nil {
			return "", err
		}

		return "Suspending the MCIS", nil

	} else if action == "resume" {
		fmt.Println("[resume MCIS]")

		err := ControlMcisAsync(nsId, mcisId, ActionResume, force)
		if err != nil {
			return "", err
		}

		return "Resuming the MCIS", nil

	} else if action == "reboot" {
		fmt.Println("[reboot MCIS]")

		err := ControlMcisAsync(nsId, mcisId, ActionReboot, force)
		if err != nil {
			return "", err
		}

		return "Rebooting the MCIS", nil

	} else if action == "terminate" {
		fmt.Println("[terminate MCIS]")

		vmList, err := ListVmId(nsId, mcisId)
		if err != nil {
			common.CBLog.Error(err)
			return "", err
		}

		if len(vmList) == 0 {
			return "No VM to terminate in the MCIS", nil
		}

		err = ControlMcisAsync(nsId, mcisId, ActionTerminate, force)
		if err != nil {
			return "", err
		}

		return "Terminated the MCIS", nil

	} else if action == "refine" { // refine delete VMs in StatusFailed or StatusUndefined
		fmt.Println("[refine MCIS]")

		vmList, err := ListVmId(nsId, mcisId)
		if err != nil {
			common.CBLog.Error(err)
			return "", err
		}

		if len(vmList) == 0 {
			return "No VM in the MCIS", nil
		}

		mcisStatus, err := GetMcisStatus(nsId, mcisId)
		if err != nil {
			common.CBLog.Error(err)
			return "", err
		}

		for _, v := range mcisStatus.Vm {

			// Remove VMs in StatusFailed or StatusUndefined
			fmt.Println("[vmInfo.Status]", v.Status)
			if v.Status == StatusFailed || v.Status == StatusUndefined {
				// Delete VM sequentially for safety (for performance, need to use goroutine)
				err := DelMcisVm(nsId, mcisId, v.Id, "force")
				if err != nil {
					common.CBLog.Error(err)
					return "", err
				}
			}
		}

		return "Refined the MCIS", nil

	} else {
		return "", fmt.Errorf(action + " not supported")
	}
}

// CoreGetMcisVmAction is func to Get McisVm Action
func CoreGetMcisVmAction(nsId string, mcisId string, vmId string, action string) (string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return "", err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return "", err
	}

	err = common.CheckString(vmId)
	if err != nil {
		common.CBLog.Error(err)
		return "", err
	}
	check, _ := CheckVm(nsId, mcisId, vmId)

	if !check {
		err := fmt.Errorf("The vm " + vmId + " does not exist.")
		return err.Error(), err
	}

	fmt.Println("[Get VM requested action: " + action)
	if action == "suspend" {
		fmt.Println("[suspend VM]")
		ControlVm(nsId, mcisId, vmId, ActionSuspend)
		return "Suspending the VM", nil

	} else if action == "resume" {
		fmt.Println("[resume VM]")
		ControlVm(nsId, mcisId, vmId, ActionResume)
		return "Resuming the VM", nil

	} else if action == "reboot" {
		fmt.Println("[reboot VM]")
		ControlVm(nsId, mcisId, vmId, ActionReboot)
		return "Rebooting the VM", nil

	} else if action == "terminate" {
		fmt.Println("[terminate VM]")
		ControlVm(nsId, mcisId, vmId, ActionTerminate)
		return "Terminated the VM", nil
	} else {
		return "", fmt.Errorf(action + " not supported")
	}
}

/* Deprecated
// ControlMcis is func to control MCIS
func ControlMcis(nsId string, mcisId string, action string) error {

	key := common.GenMcisKey(nsId, mcisId, "")
	fmt.Println("[ControlMcis] " + key + " to " + action)
	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)

	vmList, err := ListVmId(nsId, mcisId)

	if err != nil {
		common.CBLog.Error(err)
		return err
	}
	if len(vmList) == 0 {
		return nil
	}
	fmt.Println("vmList ", vmList)

	for _, v := range vmList {
		ControlVm(nsId, mcisId, v, action)
	}
	return nil

	//need to change status

}
*/

// ControlMcisAsync is func to control MCIS async
func ControlMcisAsync(nsId string, mcisId string, action string, force bool) error {

	key := common.GenMcisKey(nsId, mcisId, "")
	fmt.Println("[ControlMcisAsync] " + key + " to " + action)
	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	checkError := CheckAllowedTransition(nsId, mcisId, action)
	if checkError != nil {
		if !force {
			return checkError
		}
	}

	mcisTmp := TbMcisInfo{}
	unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &mcisTmp)
	if unmarshalErr != nil {
		fmt.Println("unmarshalErr:", unmarshalErr)
	}

	vmList, err := ListVmId(nsId, mcisId)
	fmt.Println("=============================================== ", vmList)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}
	if len(vmList) == 0 {
		return errors.New("VM list is empty")
	}

	switch action {
	case ActionTerminate:

		mcisTmp.TargetAction = ActionTerminate
		mcisTmp.TargetStatus = StatusTerminated
		mcisTmp.Status = StatusTerminating

	case ActionReboot:

		mcisTmp.TargetAction = ActionReboot
		mcisTmp.TargetStatus = StatusRunning
		mcisTmp.Status = StatusRebooting

	case ActionSuspend:

		mcisTmp.TargetAction = ActionSuspend
		mcisTmp.TargetStatus = StatusSuspended
		mcisTmp.Status = StatusSuspending

	case ActionResume:

		mcisTmp.TargetAction = ActionResume
		mcisTmp.TargetStatus = StatusRunning
		mcisTmp.Status = StatusResuming

	default:
		return errors.New(action + " is invalid actionType")
	}
	UpdateMcisInfo(nsId, mcisTmp)

	//goroutin sync wg
	var wg sync.WaitGroup
	var results ControlVmResultWrapper

	for _, v := range vmList {
		wg.Add(1)

		// Avoid concurrent requests to CSP.
		time.Sleep(time.Duration(3) * time.Second)

		go ControlVmAsync(&wg, nsId, mcisId, v, action, &results)
	}
	wg.Wait() //goroutine sync wg

	checkErrFlag := ""
	for _, v := range results.ResultArray {
		if v.Error != nil {
			checkErrFlag += "["
			checkErrFlag += v.Error.Error()
			checkErrFlag += "]"
		}
	}
	if checkErrFlag != "" {
		return fmt.Errorf(checkErrFlag)
	}

	return nil

	//need to change status

}

// ControlVmAsync is func to control VM async
func ControlVmAsync(wg *sync.WaitGroup, nsId string, mcisId string, vmId string, action string, results *ControlVmResultWrapper) error {
	defer wg.Done() //goroutine sync done

	var errTmp error
	var err error
	var err2 error
	resultTmp := ControlVmResult{}
	resultTmp.VmId = vmId
	resultTmp.Status = ""
	temp := TbVmInfo{}

	key := common.GenMcisKey(nsId, mcisId, vmId)
	fmt.Println("[ControlVmAsync] " + key)

	keyValue, err := common.CBStore.Get(key)

	if keyValue == nil || err != nil {

		resultTmp.Error = fmt.Errorf("CBStoreGetErr. keyValue == nil || err != nil. key[" + key + "]")
		results.ResultArray = append(results.ResultArray, resultTmp)
		common.PrintJsonPretty(resultTmp)
		return resultTmp.Error

	} else {
		fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		fmt.Println("===============================================")

		unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &temp)
		if unmarshalErr != nil {
			fmt.Println("Unmarshal error:", unmarshalErr)
		}

		fmt.Println("\n[Calling SPIDER]START vmControl")

		cspVmId := temp.CspViewVmDetail.IId.NameId
		common.PrintJsonPretty(temp.CspViewVmDetail)

		// Prevent malformed cspVmId
		if cspVmId == "" || common.CheckString(cspVmId) != nil {
			resultTmp.Error = fmt.Errorf("Not valid requested CSPNativeVmId: [" + cspVmId + "]")
			temp.Status = StatusFailed
			temp.SystemMessage = resultTmp.Error.Error()
			UpdateVmInfo(nsId, mcisId, temp)
			//return err
		} else {
			if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

				url := ""
				method := ""
				switch action {
				case ActionTerminate:

					temp.TargetAction = ActionTerminate
					temp.TargetStatus = StatusTerminated
					temp.Status = StatusTerminating

					url = common.SpiderRestUrl + "/vm/" + cspVmId
					method = "DELETE"
				case ActionReboot:

					temp.TargetAction = ActionReboot
					temp.TargetStatus = StatusRunning
					temp.Status = StatusRebooting

					url = common.SpiderRestUrl + "/controlvm/" + cspVmId + "?action=reboot"
					method = "GET"
				case ActionSuspend:

					temp.TargetAction = ActionSuspend
					temp.TargetStatus = StatusSuspended
					temp.Status = StatusSuspending

					url = common.SpiderRestUrl + "/controlvm/" + cspVmId + "?action=suspend"
					method = "GET"
				case ActionResume:

					temp.TargetAction = ActionResume
					temp.TargetStatus = StatusRunning
					temp.Status = StatusResuming

					url = common.SpiderRestUrl + "/controlvm/" + cspVmId + "?action=resume"
					method = "GET"
				default:
					return errors.New(action + " is invalid actionType")
				}

				UpdateVmInfo(nsId, mcisId, temp)

				type ControlVMReqInfo struct {
					ConnectionName string
				}
				tempReq := ControlVMReqInfo{}
				tempReq.ConnectionName = temp.ConnectionName
				payload, _ := json.MarshalIndent(tempReq, "", "  ")

				client := &http.Client{
					CheckRedirect: func(req *http.Request, via []*http.Request) error {
						return http.ErrUseLastResponse
					},
				}
				req, err := http.NewRequest(method, url, strings.NewReader(string(payload)))

				if err != nil {
					common.CBLog.Error(err)
					return err
				}
				req.Header.Add("Content-Type", "application/json")

				res, err := client.Do(req)
				if err != nil {
					common.CBLog.Error(err)
					return err
				}
				body, err := ioutil.ReadAll(res.Body)
				if err != nil {
					common.CBLog.Error(err)
					return err
				}
				defer res.Body.Close()

				fmt.Println("HTTP Status code: " + strconv.Itoa(res.StatusCode))
				switch {
				case res.StatusCode >= 400 || res.StatusCode < 200:
					err := fmt.Errorf(string(body))
					common.CBLog.Error(err)
					errTmp = err
				}

				err2 = json.Unmarshal(body, &resultTmp)

			} else {

				// Set CCM gRPC API
				ccm := api.NewCloudResourceHandler()
				err := ccm.SetConfigPath(os.Getenv("CBTUMBLEBUG_ROOT") + "/conf/grpc_conf.yaml")
				if err != nil {
					common.CBLog.Error("ccm failed to set config : ", err)
					temp.Status = StatusFailed
					UpdateVmInfo(nsId, mcisId, temp)
					return err
				}
				err = ccm.Open()
				if err != nil {
					common.CBLog.Error("ccm api open failed : ", err)
					temp.Status = StatusFailed
					UpdateVmInfo(nsId, mcisId, temp)
					return err
				}
				defer ccm.Close()

				var result string

				switch action {
				case ActionTerminate:

					temp.TargetAction = ActionTerminate
					temp.TargetStatus = StatusTerminated
					temp.Status = StatusTerminating

					UpdateVmInfo(nsId, mcisId, temp)

					result, err = ccm.TerminateVMByParam(temp.ConnectionName, cspVmId, "false")

				case ActionReboot:

					temp.TargetAction = ActionReboot
					temp.TargetStatus = StatusRunning
					temp.Status = StatusRebooting

					UpdateVmInfo(nsId, mcisId, temp)

					result, err = ccm.ControlVMByParam(temp.ConnectionName, cspVmId, "reboot")

				case ActionSuspend:

					temp.TargetAction = ActionSuspend
					temp.TargetStatus = StatusSuspended
					temp.Status = StatusSuspending

					UpdateVmInfo(nsId, mcisId, temp)

					result, err = ccm.ControlVMByParam(temp.ConnectionName, cspVmId, "suspend")

				case ActionResume:

					temp.TargetAction = ActionResume
					temp.TargetStatus = StatusRunning
					temp.Status = StatusResuming

					UpdateVmInfo(nsId, mcisId, temp)

					result, err = ccm.ControlVMByParam(temp.ConnectionName, cspVmId, "resume")

				default:
					return errors.New(action + " is invalid actionType")
				}

				err2 = json.Unmarshal([]byte(result), &resultTmp)

			}

			if err2 != nil {
				fmt.Println(err2)
				common.CBLog.Error(err)
				errTmp = err
			}
			if errTmp != nil {
				resultTmp.Error = errTmp

				temp.Status = StatusFailed
				temp.SystemMessage = errTmp.Error()
				UpdateVmInfo(nsId, mcisId, temp)
			}
			results.ResultArray = append(results.ResultArray, resultTmp)

			common.PrintJsonPretty(resultTmp)

			fmt.Println("[Calling SPIDER]END vmControl")

			if action != ActionTerminate {
				//When VM is restared, temporal PublicIP will be chanaged. Need update.
				UpdateVmPublicIp(nsId, mcisId, temp)
			} else { // if action == ActionTerminate
				_, err = mcir.UpdateAssociatedObjectList(nsId, common.StrImage, temp.ImageId, common.StrDelete, key)
				if err != nil {
					mcir.UpdateAssociatedObjectList(nsId, common.StrCustomImage, temp.ImageId, common.StrDelete, key)
				}

				mcir.UpdateAssociatedObjectList(nsId, common.StrSpec, temp.SpecId, common.StrDelete, key)
				mcir.UpdateAssociatedObjectList(nsId, common.StrSSHKey, temp.SshKeyId, common.StrDelete, key)
				mcir.UpdateAssociatedObjectList(nsId, common.StrVNet, temp.VNetId, common.StrDelete, key)

				for _, v := range temp.SecurityGroupIds {
					mcir.UpdateAssociatedObjectList(nsId, common.StrSecurityGroup, v, common.StrDelete, key)
				}

				for _, v := range temp.DataDiskIds {
					mcir.UpdateAssociatedObjectList(nsId, common.StrDataDisk, v, common.StrDelete, key)
				}
			}
		}

	}

	return nil

}

// ControlVm is func to control VM
func ControlVm(nsId string, mcisId string, vmId string, action string) error {

	var content struct {
		CloudId string `json:"cloudId"`
		CspVmId string `json:"cspVmId"`
	}

	key := common.GenMcisKey(nsId, mcisId, vmId)
	fmt.Println("[ControlVm] " + key)

	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		err = fmt.Errorf("In ControlVm(); CBStore.Get() returned an error.")
		common.CBLog.Error(err)
		return err
	}

	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)

	temp := TbVmInfo{}
	unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &temp)
	if unmarshalErr != nil {
		fmt.Println("unmarshalErr:", unmarshalErr)
	}

	fmt.Println("\n[Calling SPIDER]START vmControl")

	cspVmId := temp.CspViewVmDetail.IId.NameId
	common.PrintJsonPretty(temp.CspViewVmDetail)

	if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

		url := ""
		method := ""
		switch action {
		case ActionTerminate:

			temp.TargetAction = ActionTerminate
			temp.TargetStatus = StatusTerminated
			temp.Status = StatusTerminating

			url = common.SpiderRestUrl + "/vm/" + cspVmId
			method = "DELETE"
		case ActionReboot:

			temp.TargetAction = ActionReboot
			temp.TargetStatus = StatusRunning
			temp.Status = StatusRebooting

			url = common.SpiderRestUrl + "/controlvm/" + cspVmId + "?action=reboot"
			method = "GET"
		case ActionSuspend:

			temp.TargetAction = ActionSuspend
			temp.TargetStatus = StatusSuspended
			temp.Status = StatusSuspending

			url = common.SpiderRestUrl + "/controlvm/" + cspVmId + "?action=suspend"
			method = "GET"
		case ActionResume:

			temp.TargetAction = ActionResume
			temp.TargetStatus = StatusRunning
			temp.Status = StatusResuming

			url = common.SpiderRestUrl + "/controlvm/" + cspVmId + "?action=resume"
			method = "GET"
		default:
			return errors.New(action + "is invalid actionType")
		}

		UpdateVmInfo(nsId, mcisId, temp)

		type ControlVMReqInfo struct {
			ConnectionName string
		}
		tempReq := ControlVMReqInfo{}
		tempReq.ConnectionName = temp.ConnectionName
		payload, _ := json.MarshalIndent(tempReq, "", "  ")

		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		req, err := http.NewRequest(method, url, strings.NewReader(string(payload)))

		if err != nil {
			fmt.Println(err)
			return err
		}
		req.Header.Add("Content-Type", "application/json")

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

		fmt.Println("[Calling SPIDER] END vmControl\n")
		/*
			if strings.Compare(content.CspVmId, "Not assigned yet") == 0 {
				return nil
			}
			if strings.Compare(content.CloudId, "aws") == 0 {
				controlVmAws(content.CspVmId)
			} else if strings.Compare(content.CloudId, "gcp") == 0 {
				controlVmGcp(content.CspVmId)
			} else if strings.Compare(content.CloudId, "azure") == 0 {
				controlVmAzure(content.CspVmId)
			} else {
				fmt.Println("==============ERROR=no matched providerId=================")
			}
		*/

		return nil

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

		var result string

		switch action {
		case ActionTerminate:

			result, err = ccm.TerminateVMByParam(temp.ConnectionName, cspVmId, "false")
			if err != nil {
				common.CBLog.Error(err)
				return err
			}

		case ActionReboot:

			result, err = ccm.ControlVMByParam(temp.ConnectionName, cspVmId, "reboot")
			if err != nil {
				common.CBLog.Error(err)
				return err
			}

		case ActionSuspend:

			result, err = ccm.ControlVMByParam(temp.ConnectionName, cspVmId, "suspend")
			if err != nil {
				common.CBLog.Error(err)
				return err
			}

		case ActionResume:

			result, err = ccm.ControlVMByParam(temp.ConnectionName, cspVmId, "resume")
			if err != nil {
				common.CBLog.Error(err)
				return err
			}

		default:
			return errors.New(action + "is invalid actionType")
		}

		fmt.Println(result)
		fmt.Println("[Calling SPIDER]END vmControl\n")

		return nil
	}
}

// CheckAllowedTransition is func to check status transition is acceptable
func CheckAllowedTransition(nsId string, mcisId string, action string) error {

	fmt.Println("[CheckAllowedTransition]" + mcisId + " to " + action)
	key := common.GenMcisKey(nsId, mcisId, "")
	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	mcisTmp := TbMcisInfo{}
	unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &mcisTmp)
	if unmarshalErr != nil {
		fmt.Println("Unmarshal Error:", unmarshalErr)
	}

	mcisStatusTmp, _ := GetMcisStatus(nsId, mcisId)

	if strings.Contains(mcisStatusTmp.Status, StatusCreating) ||
		strings.Contains(mcisStatusTmp.Status, StatusTerminating) ||
		strings.Contains(mcisStatusTmp.Status, StatusResuming) ||
		strings.Contains(mcisStatusTmp.Status, StatusSuspending) ||
		strings.Contains(mcisStatusTmp.Status, StatusRebooting) {

		return errors.New(action + " is not allowed for MCIS under " + mcisStatusTmp.Status)
	}
	if !strings.Contains(mcisStatusTmp.Status, "Partial-") && strings.Contains(mcisStatusTmp.Status, StatusTerminated) {
		return errors.New(action + " is not allowed for " + mcisStatusTmp.Status + " MCIS")
	}
	if strings.Contains(mcisStatusTmp.Status, StatusSuspended) {
		if strings.EqualFold(action, ActionResume) || strings.EqualFold(action, ActionSuspend) || strings.EqualFold(action, ActionTerminate) {
			return nil
		} else {
			return errors.New(action + " is not allowed for " + mcisStatusTmp.Status + " MCIS")
		}
	}
	return nil
}
