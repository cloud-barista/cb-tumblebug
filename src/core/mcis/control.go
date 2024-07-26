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

	//"log"

	"strings"
	"time"

	//csv file handling

	// REST API (echo)

	"sync"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
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
		log.Error().Err(err).Msg("")
		return "", err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}
	check, _ := CheckMcis(nsId, mcisId)

	if !check {
		err := fmt.Errorf("The mcis " + mcisId + " does not exist.")
		return err.Error(), err
	}

	log.Debug().Msg("[Get MCIS requested action: " + action)
	if action == "suspend" {
		log.Debug().Msg("[suspend MCIS]")

		err := ControlMcisAsync(nsId, mcisId, ActionSuspend, force)
		if err != nil {
			return "", err
		}

		return "Suspending the MCIS", nil

	} else if action == "resume" {
		log.Debug().Msg("[resume MCIS]")

		err := ControlMcisAsync(nsId, mcisId, ActionResume, force)
		if err != nil {
			return "", err
		}

		return "Resuming the MCIS", nil

	} else if action == "reboot" {
		log.Debug().Msg("[reboot MCIS]")

		err := ControlMcisAsync(nsId, mcisId, ActionReboot, force)
		if err != nil {
			return "", err
		}

		return "Rebooting the MCIS", nil

	} else if action == "terminate" {
		log.Debug().Msg("[terminate MCIS]")

		vmList, err := ListVmId(nsId, mcisId)
		if err != nil {
			log.Error().Err(err).Msg("")
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

	} else if action == "continue" {
		log.Debug().Msg("[continue MCIS provisioning]")
		key := common.GenMcisKey(nsId, mcisId, "")
		holdingMcisMap.Store(key, action)

		return "Continue the holding MCIS", nil

	} else if action == "withdraw" {
		log.Debug().Msg("[withdraw MCIS provisioning]")
		key := common.GenMcisKey(nsId, mcisId, "")
		holdingMcisMap.Store(key, action)

		return "Withdraw the holding MCIS", nil

	} else if action == "refine" { // refine delete VMs in StatusFailed or StatusUndefined
		log.Debug().Msg("[refine MCIS]")

		vmList, err := ListVmId(nsId, mcisId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return "", err
		}

		if len(vmList) == 0 {
			return "No VM in the MCIS", nil
		}

		mcisStatus, err := GetMcisStatus(nsId, mcisId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return "", err
		}

		for _, v := range mcisStatus.Vm {

			// Remove VMs in StatusFailed or StatusUndefined
			log.Debug().Msgf("[vmInfo.Status] %v", v.Status)
			if v.Status == StatusFailed || v.Status == StatusUndefined {
				// Delete VM sequentially for safety (for performance, need to use goroutine)
				err := DelMcisVm(nsId, mcisId, v.Id, "force")
				if err != nil {
					log.Error().Err(err).Msg("")
					return "", err
				}
			}
		}

		return "Refined the MCIS", nil

	} else {
		return "", fmt.Errorf(action + " not supported")
	}
}

// HandleMcisVmAction is func to Get McisVm Action
func HandleMcisVmAction(nsId string, mcisId string, vmId string, action string, force bool) (string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}

	err = common.CheckString(vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}
	check, _ := CheckVm(nsId, mcisId, vmId)

	if !check {
		err := fmt.Errorf("The vm " + vmId + " does not exist.")
		return err.Error(), err
	}

	log.Debug().Msg("[VM action: " + action)

	mcis, err := GetMcisStatus(nsId, mcisId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}

	// Check if MCIS is under an action (individual VM action cannot be executed while MCIS is under an action)
	if mcis.TargetAction != "" && mcis.TargetAction != ActionComplete {
		err = fmt.Errorf("MCIS %s is under %s, please try later", mcisId, mcis.TargetAction)
		if !force {
			log.Info().Msg(err.Error())
			return "", err
		}
	}

	err = CheckAllowedTransition(nsId, mcisId, common.OptionalParameter{Set: true, Value: vmId}, action)
	if err != nil {
		if !force {
			log.Info().Msg(err.Error())
			return "", err
		}
	}

	var wg sync.WaitGroup
	results := make(chan ControlVmResult, 1)
	wg.Add(1)
	if strings.EqualFold(action, ActionSuspend) {
		go ControlVmAsync(&wg, nsId, mcisId, vmId, ActionSuspend, results)
	} else if strings.EqualFold(action, ActionResume) {
		go ControlVmAsync(&wg, nsId, mcisId, vmId, ActionResume, results)
	} else if strings.EqualFold(action, ActionReboot) {
		go ControlVmAsync(&wg, nsId, mcisId, vmId, ActionReboot, results)
	} else if strings.EqualFold(action, ActionTerminate) {
		go ControlVmAsync(&wg, nsId, mcisId, vmId, ActionTerminate, results)
	} else {
		close(results)
		wg.Done()
		return "", fmt.Errorf("not supported action: " + action)
	}
	checkErr := <-results
	if checkErr.Error != nil {
		return checkErr.Error.Error(), checkErr.Error
	}
	close(results)
	return "Working on " + action, nil
}

// ControlMcisAsync is func to control MCIS async
func ControlMcisAsync(nsId string, mcisId string, action string, force bool) error {

	mcis, err := GetMcisObject(nsId, mcisId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	// Check if MCIS is under an action (new action cannot be executed while MCIS is under an action)
	if mcis.TargetAction != "" && mcis.TargetAction != ActionComplete {
		err = fmt.Errorf("MCIS %s is under %s, please try later", mcisId, mcis.TargetAction)
		if !force {
			log.Info().Msg(err.Error())
			return err
		}
	}

	err = CheckAllowedTransition(nsId, mcisId, common.OptionalParameter{Set: false}, action)
	if err != nil {
		if !force {
			return err
		}
	}

	vmList, err := ListVmId(nsId, mcisId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	if len(vmList) == 0 {
		return errors.New("VM list is empty")
	}

	switch action {
	case ActionTerminate:

		mcis.TargetAction = ActionTerminate
		mcis.TargetStatus = StatusTerminated
		mcis.Status = StatusTerminating

	case ActionReboot:

		mcis.TargetAction = ActionReboot
		mcis.TargetStatus = StatusRunning
		mcis.Status = StatusRebooting

	case ActionSuspend:

		mcis.TargetAction = ActionSuspend
		mcis.TargetStatus = StatusSuspended
		mcis.Status = StatusSuspending

	case ActionResume:

		mcis.TargetAction = ActionResume
		mcis.TargetStatus = StatusRunning
		mcis.Status = StatusResuming

	default:
		return errors.New(action + " is invalid actionType")
	}
	UpdateMcisInfo(nsId, mcis)

	//goroutin sync wg
	var wg sync.WaitGroup
	results := make(chan ControlVmResult, len(vmList))

	for _, vmId := range vmList {
		// skip if control is not needed
		err = CheckAllowedTransition(nsId, mcisId, common.OptionalParameter{Set: true, Value: vmId}, action)
		if err == nil || force {
			wg.Add(1)

			// Avoid concurrent requests to CSP.
			time.Sleep(time.Duration(3) * time.Second)

			go ControlVmAsync(&wg, nsId, mcisId, vmId, action, results)
		}
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	checkErrFlag := ""
	for result := range results {
		fmt.Println("Result:", result)
		if result.Error != nil {
			checkErrFlag += "["
			checkErrFlag += result.Error.Error()
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
func ControlVmAsync(wg *sync.WaitGroup, nsId string, mcisId string, vmId string, action string, results chan<- ControlVmResult) {
	defer wg.Done() //goroutine sync done

	var err error

	callResult := ControlVmResult{}
	callResult.VmId = vmId
	callResult.Status = ""
	temp := TbVmInfo{}

	key := common.GenMcisKey(nsId, mcisId, vmId)
	log.Debug().Msg("[ControlVmAsync] " + key)

	keyValue, err := kvstore.GetKv(key)

	if keyValue == (kvstore.KeyValue{}) || err != nil {
		callResult.Error = fmt.Errorf("kvstore.Get() Err in ControlVmAsync. key[" + key + "]")
		log.Fatal().Err(callResult.Error).Msg("Error in ControlVmAsync")

		results <- callResult
		return
	} else {

		unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &temp)
		if unmarshalErr != nil {
			log.Fatal().Err(unmarshalErr).Msg("Unmarshal error")
		}

		cspVmId := temp.CspViewVmDetail.IId.NameId
		//common.PrintJsonPretty(temp.CspViewVmDetail)

		// Prevent malformed cspVmId
		if cspVmId == "" || common.CheckString(cspVmId) != nil {
			callResult.Error = fmt.Errorf("Not valid requested CSPNativeVmId: [" + cspVmId + "]")
			temp.Status = StatusFailed
			temp.SystemMessage = callResult.Error.Error()
			UpdateVmInfo(nsId, mcisId, temp)
			return
		} else {

			url := ""
			method := ""
			switch action {
			case ActionTerminate:

				temp.TargetAction = ActionTerminate
				temp.TargetStatus = StatusTerminated
				temp.Status = StatusTerminating

				url = common.SpiderRestUrl + "/vm/" + cspVmId
				method = "DELETE"

				// Remove Bastion Info from all vNets if the terminating VM is a Bastion
				_, err := RemoveBastionNodes(nsId, mcisId, vmId)
				if err != nil {
					log.Info().Msg(err.Error())
				}

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
				callResult.Error = fmt.Errorf(action + " is invalid actionType")
				results <- callResult
				return
			}

			UpdateVmInfo(nsId, mcisId, temp)

			client := resty.New()
			client.SetTimeout(10 * time.Minute)

			requestBody := common.SpiderConnectionName{}
			requestBody.ConnectionName = temp.ConnectionName

			err = common.ExecuteHttpRequest(
				client,
				method,
				url,
				nil,
				common.SetUseBody(requestBody),
				&requestBody,
				&callResult,
				common.MediumDuration,
			)
			if err != nil {
				log.Error().Err(err).Msg("")
				temp.Status = StatusFailed
				temp.SystemMessage = err.Error()
				UpdateVmInfo(nsId, mcisId, temp)

				callResult.Error = err
				results <- callResult
				return
			}

			common.PrintJsonPretty(callResult)

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

			results <- callResult
		}

	}
	return
}

// CheckAllowedTransition is func to check status transition is acceptable
func CheckAllowedTransition(nsId string, mcisId string, vmId common.OptionalParameter, action string) error {

	targetStatus := ""
	switch {
	case strings.EqualFold(action, ActionTerminate):
		targetStatus = StatusTerminated
	case strings.EqualFold(action, ActionReboot):
		targetStatus = StatusRunning
	case strings.EqualFold(action, ActionSuspend):
		targetStatus = StatusSuspended
	case strings.EqualFold(action, ActionResume):
		targetStatus = StatusRunning
	default:
		return fmt.Errorf("requested action %s is not matched with available actions", action)
	}

	if vmId.Set {
		vm, err := GetMcisVmStatus(nsId, mcisId, vmId.Value)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}

		// duplicated action
		if strings.EqualFold(vm.Status, targetStatus) {
			return errors.New(action + " is not allowed for VM under " + vm.Status)
		}
		// redundant action
		if strings.EqualFold(vm.Status, StatusTerminated) {
			return errors.New(action + " is not allowed for VM under " + vm.Status)
		}
		// under transitional status
		if strings.EqualFold(vm.Status, StatusCreating) ||
			strings.EqualFold(vm.Status, StatusTerminating) ||
			strings.EqualFold(vm.Status, StatusResuming) ||
			strings.EqualFold(vm.Status, StatusSuspending) ||
			strings.EqualFold(vm.Status, StatusRebooting) {

			return errors.New(action + " is not allowed for VM under " + vm.Status)
		}
		// under conditional status
		if strings.EqualFold(vm.Status, StatusSuspended) {
			if strings.EqualFold(action, ActionResume) || strings.EqualFold(action, ActionTerminate) {
				return nil
			} else {
				return errors.New(action + " is not allowed for VM under " + vm.Status)
			}
		}
	} else {
		mcis, err := GetMcisStatus(nsId, mcisId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}

		// duplicated action
		if strings.EqualFold(mcis.Status, targetStatus) {
			return errors.New(action + " is not allowed for MCIS under " + mcis.Status)
		}
		// redundant action
		if strings.EqualFold(mcis.Status, StatusTerminated) {
			return errors.New(action + " is not allowed for MCIS under " + mcis.Status)
		}
		// under transitional status
		if strings.Contains(mcis.Status, StatusCreating) ||
			strings.Contains(mcis.Status, StatusTerminating) ||
			strings.Contains(mcis.Status, StatusResuming) ||
			strings.Contains(mcis.Status, StatusSuspending) ||
			strings.Contains(mcis.Status, StatusRebooting) {

			return errors.New(action + " is not allowed for MCIS under " + mcis.Status)
		}
		// under conditional status
		if strings.EqualFold(mcis.Status, StatusSuspended) {
			if strings.EqualFold(action, ActionResume) || strings.EqualFold(action, ActionTerminate) {
				return nil
			} else {
				return errors.New(action + " is not allowed for MCIS under " + mcis.Status)
			}
		}
	}
	return nil
}
