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
	"errors"

	"encoding/json"
	"fmt"

	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
)

// MCI Control

// HandleMciAction is func to handle actions to MCI
func HandleMciAction(nsId string, mciId string, action string, force bool) (string, error) {
	action = common.ToLower(action)

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}

	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}
	check, _ := CheckMci(nsId, mciId)

	if !check {
		err := fmt.Errorf("The mci " + mciId + " does not exist.")
		return err.Error(), err
	}

	log.Debug().Msg("[Get MCI requested action: " + action)
	if action == "suspend" {
		log.Debug().Msg("[suspend MCI]")

		err := ControlMciAsync(nsId, mciId, model.ActionSuspend, force)
		if err != nil {
			return "", err
		}

		return "Suspending the MCI", nil

	} else if action == "resume" {
		log.Debug().Msg("[resume MCI]")

		err := ControlMciAsync(nsId, mciId, model.ActionResume, force)
		if err != nil {
			return "", err
		}

		return "Resuming the MCI", nil

	} else if action == "reboot" {
		log.Debug().Msg("[reboot MCI]")

		err := ControlMciAsync(nsId, mciId, model.ActionReboot, force)
		if err != nil {
			return "", err
		}

		return "Rebooting the MCI", nil

	} else if action == "terminate" {
		log.Debug().Msg("[terminate MCI]")

		vmList, err := ListVmId(nsId, mciId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return "", err
		}

		if len(vmList) == 0 {
			return "No VM to terminate in the MCI", nil
		}

		err = ControlMciAsync(nsId, mciId, model.ActionTerminate, force)
		if err != nil {
			return "", err
		}

		return "Terminated the MCI", nil

	} else if action == "continue" {
		log.Debug().Msg("[continue MCI provisioning]")
		key := common.GenMciKey(nsId, mciId, "")
		holdingMciMap.Store(key, action)

		return "Continue the holding MCI", nil

	} else if action == "withdraw" {
		log.Debug().Msg("[withdraw MCI provisioning]")
		key := common.GenMciKey(nsId, mciId, "")
		holdingMciMap.Store(key, action)

		return "Withdraw the holding MCI", nil

	} else if action == "refine" { // refine delete VMs in model.StatusFailed or model.StatusUndefined
		log.Debug().Msg("[refine MCI]")

		vmList, err := ListVmId(nsId, mciId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return "", err
		}

		if len(vmList) == 0 {
			return "No VM in the MCI", nil
		}

		mciStatus, err := GetMciStatus(nsId, mciId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return "", err
		}

		for _, v := range mciStatus.Vm {

			// Remove VMs in model.StatusFailed or model.StatusUndefined
			log.Debug().Msgf("[vmInfo.Status] %v", v.Status)
			if strings.EqualFold(v.Status, model.StatusFailed) || strings.EqualFold(v.Status, model.StatusUndefined) {
				// Delete VM sequentially for safety (for performance, need to use goroutine)
				err := DelMciVm(nsId, mciId, v.Id, "force")
				if err != nil {
					log.Error().Err(err).Msg("")
					return "", err
				}
			}
		}

		return "Refined the MCI", nil

	} else {
		return "", fmt.Errorf(action + " not supported")
	}
}

// HandleMciVmAction is func to Get MciVm Action
func HandleMciVmAction(nsId string, mciId string, vmId string, action string, force bool) (string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}

	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}

	err = common.CheckString(vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}
	check, _ := CheckVm(nsId, mciId, vmId)

	if !check {
		err := fmt.Errorf("The vm " + vmId + " does not exist.")
		return err.Error(), err
	}

	log.Info().Msg("[VM control request] " + action)

	mci, err := GetMciStatus(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}

	// Check if MCI is under an action (individual VM action cannot be executed while MCI is under an action)
	if mci.TargetAction != "" && mci.TargetAction != model.ActionComplete {
		err = fmt.Errorf("MCI %s is under %s, please try later", mciId, mci.TargetAction)
		if !force {
			log.Info().Msg(err.Error())
			return "", err
		}
	}

	err = CheckAllowedTransition(nsId, mciId, model.OptionalParameter{Set: true, Value: vmId}, action)
	if err != nil {
		if !force {
			log.Info().Msg(err.Error())
			return "", err
		}
	}

	var wg sync.WaitGroup
	results := make(chan model.ControlVmResult, 1)
	wg.Add(1)
	if strings.EqualFold(action, model.ActionSuspend) {
		go ControlVmAsync(&wg, nsId, mciId, vmId, model.ActionSuspend, results)
	} else if strings.EqualFold(action, model.ActionResume) {
		go ControlVmAsync(&wg, nsId, mciId, vmId, model.ActionResume, results)
	} else if strings.EqualFold(action, model.ActionReboot) {
		go ControlVmAsync(&wg, nsId, mciId, vmId, model.ActionReboot, results)
	} else if strings.EqualFold(action, model.ActionTerminate) {
		go ControlVmAsync(&wg, nsId, mciId, vmId, model.ActionTerminate, results)
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

// ControlMciAsync is func to control MCI async
func ControlMciAsync(nsId string, mciId string, action string, force bool) error {

	mci, _, err := GetMciObject(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	// Check if MCI is under an action (new action cannot be executed while MCI is under an action)
	if mci.TargetAction != "" && mci.TargetAction != model.ActionComplete {
		err = fmt.Errorf("MCI %s is under %s, please try later", mciId, mci.TargetAction)
		if !force {
			log.Info().Msg(err.Error())
			return err
		}
	}

	err = CheckAllowedTransition(nsId, mciId, model.OptionalParameter{Set: false}, action)
	if err != nil {
		if !force {
			return err
		}
	}

	vmList, err := ListVmId(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	if len(vmList) == 0 {
		return errors.New("VM list is empty")
	}

	switch action {
	case model.ActionTerminate:

		mci.TargetAction = model.ActionTerminate
		mci.TargetStatus = model.StatusTerminated
		mci.Status = model.StatusTerminating

	case model.ActionReboot:

		mci.TargetAction = model.ActionReboot
		mci.TargetStatus = model.StatusRunning
		mci.Status = model.StatusRebooting

	case model.ActionSuspend:

		mci.TargetAction = model.ActionSuspend
		mci.TargetStatus = model.StatusSuspended
		mci.Status = model.StatusSuspending

	case model.ActionResume:

		mci.TargetAction = model.ActionResume
		mci.TargetStatus = model.StatusRunning
		mci.Status = model.StatusResuming

	default:
		return errors.New(action + " is invalid actionType")
	}
	UpdateMciInfo(nsId, mci)

	//goroutin sync wg
	var wg sync.WaitGroup
	results := make(chan model.ControlVmResult, len(vmList))

	for _, vmId := range vmList {
		// skip if control is not needed
		err = CheckAllowedTransition(nsId, mciId, model.OptionalParameter{Set: true, Value: vmId}, action)
		if err == nil || force {
			wg.Add(1)

			// Avoid concurrent requests to CSP.
			time.Sleep(time.Millisecond * 1000)

			go ControlVmAsync(&wg, nsId, mciId, vmId, action, results)
		}
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	// Update MCI TargetAction to None. Even if there are errors, we want to mark it as complete.
	mci, _, err = GetMciObject(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	mci.TargetAction = model.ActionComplete
	mci.TargetStatus = model.StatusComplete
	UpdateMciInfo(nsId, mci)

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
		return errors.New(checkErrFlag)
	}

	return nil
}

// ControlVmAsync is func to control VM async
func ControlVmAsync(wg *sync.WaitGroup, nsId string, mciId string, vmId string, action string, results chan<- model.ControlVmResult) {
	defer wg.Done() //goroutine sync done

	var err error

	callResult := model.ControlVmResult{}
	callResult.VmId = vmId
	callResult.Status = ""
	temp := model.VmInfo{}

	key := common.GenMciKey(nsId, mciId, vmId)
	log.Debug().Msg("[ControlVmAsync] " + key)

	keyValue, exists, err := kvstore.GetKv(key)

	if !exists || err != nil {
		callResult.Error = fmt.Errorf("kvstore.Get() Err in ControlVmAsync. key[" + key + "]")
		log.Fatal().Err(callResult.Error).Msg("Error in ControlVmAsync")

		results <- callResult
		return
	} else {

		unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &temp)
		if unmarshalErr != nil {
			log.Fatal().Err(unmarshalErr).Msg("Unmarshal error")
		}

		cspResourceName := temp.CspResourceName
		//common.PrintJsonPretty(temp.AddtionalDetails)

		// Prevent malformed cspResourceName
		if cspResourceName == "" || common.CheckString(cspResourceName) != nil {
			callResult.Error = fmt.Errorf("Not valid requested CSPNativeVmId: [" + cspResourceName + "]")
			temp.Status = model.StatusFailed
			temp.SystemMessage = callResult.Error.Error()
			UpdateVmInfo(nsId, mciId, temp)
			return
		} else {
			currentStatusBeforeUpdating := temp.Status

			url := ""
			method := ""
			switch action {
			case model.ActionTerminate:

				temp.TargetAction = model.ActionTerminate
				temp.TargetStatus = model.StatusTerminated
				temp.Status = model.StatusTerminating

				url = model.SpiderRestUrl + "/vm/" + cspResourceName
				method = "DELETE"

				// Remove Bastion Info from all vNets if the terminating VM is a Bastion
				_, err := RemoveBastionNodes(nsId, mciId, vmId)
				if err != nil {
					log.Info().Msg(err.Error())
				}

			case model.ActionReboot:

				temp.TargetAction = model.ActionReboot
				temp.TargetStatus = model.StatusRunning
				temp.Status = model.StatusRebooting

				url = model.SpiderRestUrl + "/controlvm/" + cspResourceName + "?action=reboot"
				method = "GET"
			case model.ActionSuspend:

				temp.TargetAction = model.ActionSuspend
				temp.TargetStatus = model.StatusSuspended
				temp.Status = model.StatusSuspending

				url = model.SpiderRestUrl + "/controlvm/" + cspResourceName + "?action=suspend"
				method = "GET"
			case model.ActionResume:

				temp.TargetAction = model.ActionResume
				temp.TargetStatus = model.StatusRunning
				temp.Status = model.StatusResuming

				url = model.SpiderRestUrl + "/controlvm/" + cspResourceName + "?action=resume"
				method = "GET"
			default:
				callResult.Error = fmt.Errorf(action + " is invalid actionType")
				results <- callResult
				return
			}

			// Check current VM status before making CB-Spider API call
			// If VM is already in target status, skip the operation
			if currentStatusBeforeUpdating == temp.TargetStatus {
				log.Debug().Msgf("[ControlVmAsync] VM [%s] is already in target status [%s], skipping CB-Spider call", vmId, temp.TargetStatus)
				callResult.Status = temp.Status
				results <- callResult
				return
			}

			UpdateVmInfo(nsId, mciId, temp)

			client := resty.New()
			client.SetTimeout(10 * time.Minute)

			// Set longer timeout for NCP (VPC)
			if strings.Contains(strings.ToLower(temp.ConnectionConfig.ProviderName), csp.NCP) {
				log.Debug().Msgf("Setting longer API request timeout (15m) for %s", csp.NCP)
				client.SetTimeout(15 * time.Minute)
			}

			requestBody := model.SpiderConnectionName{}
			requestBody.ConnectionName = temp.ConnectionName

			err = clientManager.ExecuteHttpRequest(
				client,
				method,
				url,
				nil,
				clientManager.SetUseBody(requestBody),
				&requestBody,
				&callResult,
				clientManager.MediumDuration,
			)
			if err != nil {
				log.Error().Err(err).Msg("")
				temp.Status = model.StatusFailed
				temp.SystemMessage = err.Error()
				UpdateVmInfo(nsId, mciId, temp)

				callResult.Error = err
				results <- callResult
				return
			}

			common.PrintJsonPretty(callResult)

			if action != model.ActionTerminate {
				//When VM is restared, temporal PublicIP will be chanaged. Need update.
				UpdateVmPublicIp(nsId, mciId, temp)
			} else { // if action == model.ActionTerminate
				_, err = resource.UpdateAssociatedObjectList(nsId, model.StrImage, temp.ImageId, model.StrDelete, key)
				if err != nil {
					resource.UpdateAssociatedObjectList(nsId, model.StrCustomImage, temp.ImageId, model.StrDelete, key)
				}

				//resource.UpdateAssociatedObjectList(nsId, model.StrSpec, temp.SpecId, model.StrDelete, key)
				resource.UpdateAssociatedObjectList(nsId, model.StrSSHKey, temp.SshKeyId, model.StrDelete, key)
				resource.UpdateAssociatedObjectList(nsId, model.StrVNet, temp.VNetId, model.StrDelete, key)

				for _, v := range temp.SecurityGroupIds {
					resource.UpdateAssociatedObjectList(nsId, model.StrSecurityGroup, v, model.StrDelete, key)
				}

				for _, v := range temp.DataDiskIds {
					resource.UpdateAssociatedObjectList(nsId, model.StrDataDisk, v, model.StrDelete, key)
				}
			}

			results <- callResult
		}

	}
	return
}

// CheckAllowedTransition is func to check status transition is acceptable
func CheckAllowedTransition(nsId string, mciId string, vmId model.OptionalParameter, action string) error {

	targetStatus := ""
	switch {
	case strings.EqualFold(action, model.ActionTerminate):
		targetStatus = model.StatusTerminated
	case strings.EqualFold(action, model.ActionReboot):
		targetStatus = model.StatusRunning
	case strings.EqualFold(action, model.ActionSuspend):
		targetStatus = model.StatusSuspended
	case strings.EqualFold(action, model.ActionResume):
		targetStatus = model.StatusRunning
	default:
		return fmt.Errorf("requested action %s is not matched with available actions", action)
	}

	if vmId.Set {
		vm, err := GetMciVmStatus(nsId, mciId, vmId.Value)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}

		// duplicated action
		if strings.EqualFold(vm.Status, targetStatus) {
			if !strings.EqualFold(action, model.ActionReboot) {
				return errors.New(action + " is not allowed for VM under " + vm.Status)
			}
		}
		// redundant action
		if strings.EqualFold(vm.Status, model.StatusTerminated) {
			return errors.New(action + " is not allowed for VM under " + vm.Status)
		}
		// under transitional status
		if strings.EqualFold(vm.Status, model.StatusCreating) ||
			strings.EqualFold(vm.Status, model.StatusTerminating) ||
			strings.EqualFold(vm.Status, model.StatusResuming) ||
			strings.EqualFold(vm.Status, model.StatusSuspending) ||
			strings.EqualFold(vm.Status, model.StatusRebooting) {

			return errors.New(action + " is not allowed for VM under " + vm.Status)
		}
		// under conditional status
		if strings.EqualFold(vm.Status, model.StatusSuspended) {
			if strings.EqualFold(action, model.ActionResume) || strings.EqualFold(action, model.ActionTerminate) {
				return nil
			} else {
				return errors.New(action + " is not allowed for VM under " + vm.Status)
			}
		}
	} else {
		mci, err := GetMciStatus(nsId, mciId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}

		// duplicated action
		if strings.EqualFold(mci.Status, targetStatus) {
			return errors.New(action + " is not allowed for MCI under " + mci.Status)
		}
		// redundant action
		if strings.EqualFold(mci.Status, model.StatusTerminated) {
			return errors.New(action + " is not allowed for MCI under " + mci.Status)
		}
		// under transitional status
		if strings.Contains(mci.Status, model.StatusCreating) ||
			strings.Contains(mci.Status, model.StatusTerminating) ||
			strings.Contains(mci.Status, model.StatusResuming) ||
			strings.Contains(mci.Status, model.StatusSuspending) ||
			strings.Contains(mci.Status, model.StatusRebooting) {

			return errors.New(action + " is not allowed for MCI under " + mci.Status)
		}
		// under conditional status
		if strings.EqualFold(mci.Status, model.StatusSuspended) {
			if strings.EqualFold(action, model.ActionResume) || strings.EqualFold(action, model.ActionTerminate) {
				return nil
			} else {
				return errors.New(action + " is not allowed for MCI under " + mci.Status)
			}
		}
	}
	return nil
}
