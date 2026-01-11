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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	validator "github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/ssh"
)

// TbMciCmdReqStructLevelValidation is func to validate fields in model.MciCmdReq
func TbMciCmdReqStructLevelValidation(sl validator.StructLevel) {

	// u := sl.Current().Interface().(model.MciCmdReq)

	// err := common.CheckString(u.Command)
	// if err != nil {
	// 	// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
	// 	sl.ReportError(u.Command, "command", "Command", err.Error(), "")
	// }
}

// RemoteCommandToMci is func to command to all VMs in MCI by SSH
func RemoteCommandToMci(nsId string, mciId string, subGroupId string, vmId string, labelSelector string, req *model.MciCmdReq, xRequestId string) ([]model.SshCmdResult, error) {

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

	// returns InvalidValidationError for bad validation input, nil or ValidationErrors ( []FieldError )
	err = validate.Struct(req)
	if err != nil {

		// this check is only needed when your code could produce
		// an invalid value for validation such as interface with nil
		// value most including myself do not usually have code like this.
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Err(err).Msg("")
			temp := []model.SshCmdResult{}
			return temp, err
		}

		temp := []model.SshCmdResult{}
		return temp, err
	}

	check, _ := CheckMci(nsId, mciId)

	if !check {
		temp := []model.SshCmdResult{}
		err := fmt.Errorf("The mci " + mciId + " does not exist.")
		return temp, err
	}

	vmList, err := ListVmId(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	if len(vmList) == 0 {
		err := fmt.Errorf("MCI %s has no VMs to execute commands (status: Empty)", mciId)
		return nil, err
	}
	if subGroupId != "" {
		vmListInGroup, err := ListVmBySubGroup(nsId, mciId, subGroupId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return nil, err
		}
		if vmListInGroup == nil {
			err := fmt.Errorf("there is no %s subGroup or VM in the subGroup ", subGroupId)
			return nil, err
		}
		vmList = vmListInGroup
	}

	if vmId != "" {
		vmList = []string{vmId}
	}

	// Apply label-based filtering if labelSelector is specified
	if labelSelector != "" {
		log.Info().Str("labelSelector", labelSelector).Msg("Filtering VMs by label selector")

		// Add system label conditions
		systemLabelConditions := fmt.Sprintf("sys.mciId=%s", mciId)

		// Also add subGroupId condition if specified
		if subGroupId != "" {
			systemLabelConditions += fmt.Sprintf(",sys.subGroupId=%s", subGroupId)
		}

		labelSelector = systemLabelConditions + "," + labelSelector

		log.Debug().Str("combinedLabelSelector", labelSelector).Msg("Combined label selector")

		// Query resources using label selector
		matchedResources, err := label.GetResourcesByLabelSelector(model.StrVM, labelSelector)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get resources by label selector")
			return nil, fmt.Errorf("label selector error: %v", err)
		}

		if len(matchedResources) == 0 {
			log.Warn().Msg("No VMs matched the label selector criteria")
			return nil, fmt.Errorf("no VMs matched the label selector: %s", labelSelector)
		}

		// Extract matching VM IDs only
		filteredVmIds := make([]string, 0, len(matchedResources))
		for _, resource := range matchedResources {
			if vmInfo, ok := resource.(*model.VmInfo); ok {
				filteredVmIds = append(filteredVmIds, vmInfo.Id)
			}
		}

		log.Info().
			Int("matchedVMsCount", len(filteredVmIds)).
			Str("labelSelector", labelSelector).
			Msg("VMs filtered by label selector")

		// Replace VM list with label selector filtered VMs
		vmList = filteredVmIds
	}

	// goroutine sync wg
	var wg sync.WaitGroup

	var resultArray []model.SshCmdResult

	// Preprocess commands for each VM and add command status info
	vmCommands := make(map[string][]string)
	vmCommandIndices := make(map[string]int) // Track command index for each VM

	for i, targetVmId := range vmList {
		processedCommands := make([]string, len(req.Command))
		for j, cmd := range req.Command {
			processedCmd, err := processCommand(cmd, nsId, mciId, targetVmId, i)
			if err != nil {
				return nil, err
			}
			processedCommands[j] = processedCmd
		}
		vmCommands[targetVmId] = processedCommands

		// Add command status info for this VM
		combinedCommand := strings.Join(req.Command, " && ")
		combinedProcessedCommand := strings.Join(processedCommands, " && ")

		cmdIndex, err := AddCommandStatusInfo(nsId, mciId, targetVmId, xRequestId, combinedCommand, combinedProcessedCommand)
		if err != nil {
			log.Error().Err(err).Str("vmId", targetVmId).Msg("Failed to add command status info")
			// Continue with execution even if status tracking fails
		} else {
			vmCommandIndices[targetVmId] = cmdIndex
		}
	}

	// Execute commands in parallel using goroutines
	for targetVmId, commands := range vmCommands {
		wg.Add(1)
		go RunRemoteCommandAsyncWithStatus(&wg, nsId, mciId, targetVmId, req.UserName, commands, vmCommandIndices[targetVmId], &resultArray)
	}
	wg.Wait() // goroutine sync wg

	return resultArray, nil
}

// RunRemoteCommand is func to execute a SSH command to a VM (sync call)
func RunRemoteCommand(nsId string, mciId string, vmId string, givenUserName string, cmds []string) (map[int]string, map[int]string, error) {

	// use privagte IP of the target VM
	_, targetVmIP, targetSshPort, err := GetVmIp(nsId, mciId, vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return map[int]string{}, map[int]string{}, err
	}
	targetUserName, targetPrivateKey, err := VerifySshUserName(nsId, mciId, vmId, targetVmIP, targetSshPort, givenUserName)
	if err != nil {
		log.Error().Err(err).Msg("")
		return map[int]string{}, map[int]string{}, err
	}

	// Set Bastion SSH config (bastionEndpoint, userName, Private Key)
	bastionNodes, err := GetBastionNodes(nsId, mciId, vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return map[int]string{}, map[int]string{}, err
	}

	if len(bastionNodes) == 0 {
		err = fmt.Errorf("no bastion nodes available for VM (ID: %s) in MCI (ID: %s)", vmId, mciId)
		log.Error().Err(err).Msg("")

		// Assign a Bastion if none (randomly)
		_, err = SetBastionNodes(nsId, mciId, vmId, "")
		if err != nil {
			log.Error().Err(err).Msg("no bastion nodes available")
			return map[int]string{}, map[int]string{}, err
		}
		bastionNodes, err = GetBastionNodes(nsId, mciId, vmId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return map[int]string{}, map[int]string{}, err
		}
		if len(bastionNodes) == 0 {
			err = fmt.Errorf("still no bastion nodes available after attempted assignment")
			log.Error().Err(err).Msg("")
			return map[int]string{}, map[int]string{}, err
		}
	}

	bastionNode := bastionNodes[0]

	// Validate bastion node has valid VM ID
	if bastionNode.VmId == "" {
		err = fmt.Errorf("bastion node has empty VM ID")
		log.Error().Err(err).Msg("")
		return map[int]string{}, map[int]string{}, err
	}

	// use public IP of the bastion VM
	bastionIp, _, bastionSshPort, err := GetVmIp(nsId, bastionNode.MciId, bastionNode.VmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return map[int]string{}, map[int]string{}, err
	}

	// Validate bastion IP before proceeding
	if bastionIp == "" {
		err = fmt.Errorf("bastion VM (ID: %s) does not have a public IP address", bastionNode.VmId)
		log.Error().Err(err).Msg("")
		return map[int]string{}, map[int]string{}, err
	}

	// Validate IP address format
	if net.ParseIP(bastionIp) == nil {
		err = fmt.Errorf("bastion VM (ID: %s) has invalid IP address: %s", bastionNode.VmId, bastionIp)
		log.Error().Err(err).Msg("")
		return map[int]string{}, map[int]string{}, err
	}

	bastionUserName, bastionSshKey, err := VerifySshUserName(nsId, bastionNode.MciId, bastionNode.VmId, bastionIp, bastionSshPort, givenUserName)
	if err != nil {
		log.Error().Err(err).Msg("")
		return map[int]string{}, map[int]string{}, err
	}

	bastionEndpoint := fmt.Sprintf("%s:%s", bastionIp, bastionSshPort)

	// Log bastion connection details for debugging
	log.Debug().
		Str("bastionVmId", bastionNode.VmId).
		Str("bastionIp", bastionIp).
		Str("bastionPort", bastionSshPort).
		Str("bastionEndpoint", bastionEndpoint).
		Str("bastionUserName", bastionUserName).
		Msg("Bastion connection details")

	bastionSshInfo := model.SshInfo{
		EndPoint:   bastionEndpoint,
		UserName:   bastionUserName,
		PrivateKey: []byte(bastionSshKey),
	}

	log.Debug().Msg("[SSH] " + mciId + "." + vmId + "(" + targetVmIP + ")" + " with userName: " + targetUserName)
	for i, v := range cmds {
		log.Debug().Msg("[SSH] cmd[" + fmt.Sprint(i) + "]: " + v)
	}

	// Set VM SSH config (targetEndpoint, userName, Private Key)
	targetEndpoint := fmt.Sprintf("%s:%s", targetVmIP, targetSshPort)
	targetSshInfo := model.SshInfo{
		EndPoint:   targetEndpoint,
		UserName:   targetUserName,
		PrivateKey: []byte(targetPrivateKey),
	}

	// Set TOFU context for bastion and target VMs
	bastionCtx := tofuContext{
		NsId:  nsId,
		MciId: bastionNode.MciId,
		VmId:  bastionNode.VmId,
	}
	targetCtx := tofuContext{
		NsId:  nsId,
		MciId: mciId,
		VmId:  vmId,
	}

	// Execute SSH with TOFU host key verification
	stdoutResults, stderrResults, err := runSSH(bastionSshInfo, targetSshInfo, cmds, bastionCtx, targetCtx)
	if err != nil {
		log.Err(err).Msg("Error executing commands")
		return stdoutResults, stderrResults, err
	}
	return stdoutResults, stderrResults, nil

}

// RunRemoteCommandAsync is func to execute a SSH command to a VM (async call)
func RunRemoteCommandAsync(wg *sync.WaitGroup, nsId string, mciId string, vmId string, givenUserName string, cmd []string, returnResult *[]model.SshCmdResult) {

	defer wg.Done() //goroutine sync done

	vmIP, _, _, err := GetVmIp(nsId, mciId, vmId)

	sshResultTmp := model.SshCmdResult{}
	sshResultTmp.MciId = mciId
	sshResultTmp.VmId = vmId
	sshResultTmp.VmIp = vmIP
	sshResultTmp.Command = make(map[int]string)
	for i, c := range cmd {
		sshResultTmp.Command[i] = c
	}

	if err != nil {
		sshResultTmp.Err = err
		*returnResult = append(*returnResult, sshResultTmp)
		return
	}

	// Check VM status before executing SSH command
	vmInfo, err := GetVmObject(nsId, mciId, vmId)
	if err != nil {
		sshResultTmp.Err = fmt.Errorf("failed to get VM status: %v", err)
		*returnResult = append(*returnResult, sshResultTmp)
		return
	}

	// Validate VM status for SSH execution
	if vmInfo.Status != model.StatusRunning {
		var errorMsg string
		if vmInfo.Status == model.StatusTerminated {
			errorMsg = fmt.Sprintf("VM '%s' is in '%s' status. SSH connection is impossible for terminated VMs", vmId, vmInfo.Status)
		} else {
			errorMsg = fmt.Sprintf("VM '%s' is in '%s' status (not Running). Please change the VM status to Running and try again", vmId, vmInfo.Status)
		}
		sshResultTmp.Err = fmt.Errorf(errorMsg)
		*returnResult = append(*returnResult, sshResultTmp)
		return
	}

	// RunRemoteCommand
	stdoutResults, stderrResults, err := RunRemoteCommand(nsId, mciId, vmId, givenUserName, cmd)

	if err != nil {
		sshResultTmp.Stdout = stdoutResults
		sshResultTmp.Stderr = stderrResults
		sshResultTmp.Err = err
		*returnResult = append(*returnResult, sshResultTmp)
		return
	}

	log.Debug().Msg("[Begin] SSH Output")
	fmt.Println(stdoutResults)
	log.Debug().Msg("[End] SSH Output")

	sshResultTmp.Stdout = stdoutResults
	sshResultTmp.Stderr = stderrResults
	sshResultTmp.Err = nil
	*returnResult = append(*returnResult, sshResultTmp)
}

// RunRemoteCommandAsyncWithStatus is func to execute a SSH command to a VM (async call) with command status tracking
func RunRemoteCommandAsyncWithStatus(wg *sync.WaitGroup, nsId string, mciId string, vmId string, givenUserName string, cmd []string, cmdIndex int, returnResult *[]model.SshCmdResult) {

	defer wg.Done() //goroutine sync done

	vmIP, _, _, err := GetVmIp(nsId, mciId, vmId)

	sshResultTmp := model.SshCmdResult{}
	sshResultTmp.MciId = mciId
	sshResultTmp.VmId = vmId
	sshResultTmp.VmIp = vmIP
	sshResultTmp.Command = make(map[int]string)
	for i, c := range cmd {
		sshResultTmp.Command[i] = c
	}

	// Update status to Handling
	if cmdIndex > 0 {
		err := UpdateCommandStatusInfo(nsId, mciId, vmId, cmdIndex, model.CommandStatusHandling, "", "", "", "")
		if err != nil {
			log.Error().Err(err).Int("cmdIndex", cmdIndex).Msg("Failed to update command status to Handling")
		}
	}

	if err != nil {
		sshResultTmp.Err = err
		// Update status to Failed
		if cmdIndex > 0 {
			UpdateCommandStatusInfo(nsId, mciId, vmId, cmdIndex, model.CommandStatusFailed, "Failed to get VM IP", err.Error(), "", "")
		}
		*returnResult = append(*returnResult, sshResultTmp)
		return
	}

	// Check VM status before executing SSH command
	vmInfo, err := GetVmObject(nsId, mciId, vmId)
	if err != nil {
		sshResultTmp.Err = fmt.Errorf("failed to get VM status: %v", err)
		// Update status to Failed
		if cmdIndex > 0 {
			UpdateCommandStatusInfo(nsId, mciId, vmId, cmdIndex, model.CommandStatusFailed, "Failed to get VM status", err.Error(), "", "")
		}
		*returnResult = append(*returnResult, sshResultTmp)
		return
	}

	// Validate VM status for SSH execution
	if vmInfo.Status != model.StatusRunning {
		var errorMsg string
		if vmInfo.Status == model.StatusTerminated {
			errorMsg = fmt.Sprintf("VM '%s' is in '%s' status. SSH connection is impossible for terminated VMs", vmId, vmInfo.Status)
		} else {
			errorMsg = fmt.Sprintf("VM '%s' is in '%s' status (not Running). Please change the VM status to Running and try again", vmId, vmInfo.Status)
		}
		sshResultTmp.Err = fmt.Errorf(errorMsg)
		// Update status to Failed
		if cmdIndex > 0 {
			UpdateCommandStatusInfo(nsId, mciId, vmId, cmdIndex, model.CommandStatusFailed, "VM not in running status", errorMsg, "", "")
		}
		*returnResult = append(*returnResult, sshResultTmp)
		return
	}

	// Create context with timeout for long-running commands
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute) // 30 minute timeout
	defer cancel()

	// Channel to receive command execution results
	resultChan := make(chan struct {
		stdout map[int]string
		stderr map[int]string
		err    error
	}, 1)

	// Execute command in a separate goroutine
	go func() {
		stdout, stderr, err := RunRemoteCommand(nsId, mciId, vmId, givenUserName, cmd)
		resultChan <- struct {
			stdout map[int]string
			stderr map[int]string
			err    error
		}{stdout, stderr, err}
	}()

	// Wait for either completion or timeout
	select {
	case result := <-resultChan:
		// Command completed
		if result.err != nil {
			sshResultTmp.Stdout = result.stdout
			sshResultTmp.Stderr = result.stderr
			sshResultTmp.Err = result.err

			// Update status to Failed
			if cmdIndex > 0 {
				// Convert map to string for storage
				stdoutStr := ""
				stderrStr := ""
				for _, v := range result.stdout {
					stdoutStr += v + "\n"
				}
				for _, v := range result.stderr {
					stderrStr += v + "\n"
				}
				UpdateCommandStatusInfo(nsId, mciId, vmId, cmdIndex, model.CommandStatusFailed, "Command execution failed", result.err.Error(), stdoutStr, stderrStr)
			}
			*returnResult = append(*returnResult, sshResultTmp)
			return
		}

		log.Debug().Msg("[Begin] SSH Output")
		fmt.Println(result.stdout)
		log.Debug().Msg("[End] SSH Output")

		sshResultTmp.Stdout = result.stdout
		sshResultTmp.Stderr = result.stderr
		sshResultTmp.Err = nil

		// Update status to Completed
		if cmdIndex > 0 {
			// Convert map to string for storage
			stdoutStr := ""
			stderrStr := ""
			for _, v := range result.stdout {
				stdoutStr += v + "\n"
			}
			for _, v := range result.stderr {
				stderrStr += v + "\n"
			}
			UpdateCommandStatusInfo(nsId, mciId, vmId, cmdIndex, model.CommandStatusCompleted, "Command executed successfully", "", stdoutStr, stderrStr)
		}
		*returnResult = append(*returnResult, sshResultTmp)

	case <-ctx.Done():
		// Command timed out
		timeoutErr := fmt.Errorf("command execution timed out after 30 minutes")
		sshResultTmp.Err = timeoutErr

		// Update status to Timeout
		if cmdIndex > 0 {
			UpdateCommandStatusInfo(nsId, mciId, vmId, cmdIndex, model.CommandStatusTimeout, "Command execution timed out", timeoutErr.Error(), "", "")
		}

		log.Error().
			Str("nsId", nsId).
			Str("mciId", mciId).
			Str("vmId", vmId).
			Int("cmdIndex", cmdIndex).
			Msg("Command execution timed out")

		*returnResult = append(*returnResult, sshResultTmp)
	}
}

// VerifySshUserName is func to verify SSH username
func VerifySshUserName(nsId string, mciId string, vmId string, vmIp string, sshPort string, givenUserName string) (string, string, error) {

	// Disable the verification of SSH username (until bastion host is supported)

	// // find vaild username
	// userName, verifiedUserName, privateKey := GetVmSshKey(nsId, mciId, vmId)
	// userNames := []string{
	// 	model.SshDefaultUserName[0],
	// 	userName,
	// 	givenUserName,
	// 	model.SshDefaultUserName[1],
	// 	model.SshDefaultUserName[2],
	// 	model.SshDefaultUserName[3],
	// }

	// theUserName := ""
	// cmd := "sudo ls"

	// if verifiedUserName != "" {
	// 	/* Code for strict check in advance with real SSH (but slow down speed)
	// 	fmt.Printf("\n[Check SSH] (%s) with userName: %s\n", vmIp, verifiedUserName)
	// 	_, err := RunRemoteCommand(vmIp, sshPort, verifiedUserName, privateKey, cmd)
	// 	if err != nil {
	// 		return "", "", fmt.Errorf("Cannot do ssh, with %s, %s", verifiedUserName, err.Error())
	// 	}*/
	// 	theUserName = verifiedUserName
	// 	fmt.Printf("[%s] is a valid UserName\n", theUserName)
	// 	return theUserName, privateKey, nil
	// }

	// // If we have a varified username, Retrieve ssh username from the given list will not be executed
	// log.Debug().Msg("[Retrieve ssh username from the given list]")
	// for _, v := range userNames {
	// 	if v != "" {
	// 		fmt.Printf("[Check SSH] (%s) with userName: %s\n", vmIp, v)
	// 		_, err := RunRemoteCommand(vmIp, sshPort, v, privateKey, cmd)
	// 		if err != nil {
	// 			fmt.Printf("Cannot do ssh, with %s, %s", verifiedUserName, err.Error())
	// 		} else {
	// 			theUserName = v
	// 			fmt.Printf("[%s] is a valid UserName\n", theUserName)
	// 			break
	// 		}
	// 		time.Sleep(3 * time.Second)
	// 	}
	// }

	userName, _, privateKey, err := GetVmSshKey(nsId, mciId, vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", "", err
	}

	theUserName := ""
	if givenUserName != "" {
		theUserName = givenUserName
	} else if userName != "" {
		theUserName = userName
	} else {
		theUserName = model.SshDefaultUserName[0] // default username: cb-user
	}

	if theUserName == "" {
		err := fmt.Errorf("Could not find a valid username")
		log.Error().Err(err).Msg("")
		return "", "", err
	}

	// Disable the verification of SSH username (until bastion host is supported)

	// if theUserName != "" {
	// 	err := UpdateVmSshKey(nsId, mciId, vmId, theUserName)
	// 	if err != nil {
	// 		log.Error().Err(err).Msg("")
	// 		return "", "", err
	// 	}
	// } else {
	// 	return "", "", fmt.Errorf("Could not find a valid username")
	// }

	return theUserName, privateKey, nil
}

// CheckConnectivity func checks if given port is open and ready
func CheckConnectivity(host string, port string) error {
	retrycheck := 5
	initialTimeout := 20 * time.Second
	maxTimeout := 60 * time.Second

	var lastErr error
	for i := 0; i < retrycheck; i++ {
		// Fix timeout calculation: start with initialTimeout for first attempt (i=0)
		// then progressively increase for subsequent attempts
		timeout := time.Duration(float64(initialTimeout) * (1.0 + 0.5*float64(i)))
		if timeout > maxTimeout {
			timeout = maxTimeout
		}

		log.Debug().Msgf("[Check SSH Port] %v:%v (Attempt %d/%d, Timeout: %v)",
			host, port, i+1, retrycheck, timeout)

		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), timeout)
		if err != nil {
			lastErr = err
			waitTime := time.Duration(5*(i+1)) * time.Second
			log.Warn().Err(err).Msgf("SSH Port is NOT accessible yet. Attempt %d/%d. Retrying in %v...",
				i+1, retrycheck, waitTime)
			time.Sleep(waitTime)
			continue
		}

		if conn != nil {
			conn.Close()
		}

		log.Info().Msgf("SSH Port is accessible after %d attempt(s)", i+1)
		return nil
	}

	return fmt.Errorf("SSH Port is NOT accessible after %d attempts: %v", retrycheck, lastErr)
}

// GetVmSshKey is func to get VM SShKey. Returns username, verifiedUsername, privateKey
func GetVmSshKey(nsId string, mciId string, vmId string) (string, string, string, error) {

	var content struct {
		SshKeyId string `json:"sshKeyId"`
	}

	key := common.GenMciKey(nsId, mciId, vmId)

	keyValue, _, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("Cannot find the key from DB. key: " + key)
		return "", "", "", err
	}

	err = json.Unmarshal([]byte(keyValue.Value), &content)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", "", "", err
	}

	sshKey := common.GenResourceKey(nsId, model.StrSSHKey, content.SshKeyId)
	keyValue, _, err = kvstore.GetKv(sshKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", "", "", err
	}

	var keyContent struct {
		Username         string `json:"username"`
		VerifiedUsername string `json:"verifiedUsername"`
		PrivateKey       string `json:"privateKey"`
	}
	err = json.Unmarshal([]byte(keyValue.Value), &keyContent)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", "", "", err
	}

	// Private key should already be normalized at storage time
	privateKey := keyContent.PrivateKey

	if privateKey == "" {
		err = fmt.Errorf("private key not found in SSH key resource")
		log.Error().Err(err).Msg("")
		return "", "", "", err
	}

	return keyContent.Username, keyContent.VerifiedUsername, privateKey, nil
}

// UpdateVmSshKey is func to update VM SShKey
func UpdateVmSshKey(nsId string, mciId string, vmId string, verifiedUserName string) error {

	var content struct {
		SshKeyId string `json:"sshKeyId"`
	}

	key := common.GenMciKey(nsId, mciId, vmId)
	keyValue, _, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("In UpdateVmSshKey(); kvstore.GetKv() returned an error.")
		log.Error().Err(err).Msg("")
		// return nil, err
	}

	json.Unmarshal([]byte(keyValue.Value), &content)

	sshKey := common.GenResourceKey(nsId, model.StrSSHKey, content.SshKeyId)
	keyValue, _, _ = kvstore.GetKv(sshKey)

	tmpSshKeyInfo := model.SshKeyInfo{}
	json.Unmarshal([]byte(keyValue.Value), &tmpSshKeyInfo)

	tmpSshKeyInfo.VerifiedUsername = verifiedUserName

	val, _ := json.Marshal(tmpSshKeyInfo)
	err = kvstore.Put(keyValue.Key, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	return nil
}

// Internal functions for SSH
func init() {

}

// SshHostKeyMismatchError represents an SSH host key verification failure
// This error occurs when the stored host key doesn't match the server's current host key
type SshHostKeyMismatchError struct {
	VmId                string
	StoredKeyType       string
	StoredFingerprint   string
	ReceivedKeyType     string
	ReceivedFingerprint string
}

func (e *SshHostKeyMismatchError) Error() string {
	return fmt.Sprintf("SSH host key verification failed for VM '%s': stored key fingerprint (%s %s) does not match received key (%s %s). "+
		"This could indicate a man-in-the-middle attack or the VM's host key has changed. "+
		"If you trust the new key, use the SSH host key reset API to update it.",
		e.VmId, e.StoredKeyType, e.StoredFingerprint, e.ReceivedKeyType, e.ReceivedFingerprint)
}

// calculateHostKeyFingerprint calculates SHA256 fingerprint of an SSH public key
// Returns standard SSH fingerprint format: "SHA256:" prefix with base64-encoded hash
func calculateHostKeyFingerprint(publicKey ssh.PublicKey) string {
	hash := sha256.Sum256(publicKey.Marshal())
	encoded := base64.StdEncoding.EncodeToString(hash[:])
	// Standard SSH fingerprint format: "SHA256:" prefix with base64-encoded hash without padding
	encoded = strings.TrimRight(encoded, "=")
	return "SHA256:" + encoded
}

// tofuContext contains VM identification info for TOFU host key verification (internal use only)
type tofuContext struct {
	NsId  string
	MciId string
	VmId  string
}

// createTOFUHostKeyCallback creates a HostKeyCallback that implements TOFU (Trust On First Use)
// - On first use: stores the host key and allows connection
// - On subsequent uses: verifies the host key matches the stored one
func createTOFUHostKeyCallback(ctx tofuContext) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		keyType := key.Type()
		keyData := base64.StdEncoding.EncodeToString(key.Marshal())
		fingerprint := calculateHostKeyFingerprint(key)

		log.Debug().
			Str("vmId", ctx.VmId).
			Str("hostname", hostname).
			Str("keyType", keyType).
			Str("fingerprint", fingerprint).
			Msg("SSH host key verification")

		// Get current VM info
		vmInfo, err := GetVmObject(ctx.NsId, ctx.MciId, ctx.VmId)
		if err != nil {
			// If VM info cannot be retrieved, reject connection for security
			log.Warn().
				Err(err).
				Str("vmId", ctx.VmId).
				Msg("Cannot retrieve VM info for TOFU verification, rejecting connection")
			return fmt.Errorf("cannot retrieve VM info for TOFU verification: %w", err)
		}

		// First connection (TOFU): store the host key
		if vmInfo.SshHostKeyInfo == nil || vmInfo.SshHostKeyInfo.HostKey == "" {
			log.Info().
				Str("vmId", ctx.VmId).
				Str("keyType", keyType).
				Str("fingerprint", fingerprint).
				Msg("First SSH connection - storing host key (TOFU)")

			vmInfo.SshHostKeyInfo = &model.SshHostKeyInfo{
				HostKey:     keyData,
				KeyType:     keyType,
				Fingerprint: fingerprint,
				FirstUsedAt: time.Now().Format(time.RFC3339),
			}

			UpdateVmInfo(ctx.NsId, ctx.MciId, vmInfo)

			return nil
		}

		// Subsequent connections: verify the host key
		if vmInfo.SshHostKeyInfo.HostKey != keyData {
			log.Warn().
				Str("vmId", ctx.VmId).
				Str("storedKeyType", vmInfo.SshHostKeyInfo.KeyType).
				Str("storedFingerprint", vmInfo.SshHostKeyInfo.Fingerprint).
				Str("receivedKeyType", keyType).
				Str("receivedFingerprint", fingerprint).
				Msg("SSH host key mismatch detected")

			return &SshHostKeyMismatchError{
				VmId:                ctx.VmId,
				StoredKeyType:       vmInfo.SshHostKeyInfo.KeyType,
				StoredFingerprint:   vmInfo.SshHostKeyInfo.Fingerprint,
				ReceivedKeyType:     keyType,
				ReceivedFingerprint: fingerprint,
			}
		}

		log.Debug().
			Str("vmId", ctx.VmId).
			Str("fingerprint", fingerprint).
			Msg("SSH host key verified successfully")

		return nil
	}
}

// ResetVmSshHostKey resets the stored SSH host key for a VM
// This should be called when the user trusts a new host key after verification failure
func ResetVmSshHostKey(nsId string, mciId string, vmId string) error {
	err := common.CheckString(nsId)
	if err != nil {
		return fmt.Errorf("invalid nsId: %w", err)
	}
	err = common.CheckString(mciId)
	if err != nil {
		return fmt.Errorf("invalid mciId: %w", err)
	}
	err = common.CheckString(vmId)
	if err != nil {
		return fmt.Errorf("invalid vmId: %w", err)
	}

	vmInfo, err := GetVmObject(nsId, mciId, vmId)
	if err != nil {
		return fmt.Errorf("failed to get VM info: %w", err)
	}

	log.Info().
		Str("vmId", vmId).
		Str("previousKeyType", func() string {
			if vmInfo.SshHostKeyInfo != nil {
				return vmInfo.SshHostKeyInfo.KeyType
			}
			return ""
		}()).
		Str("previousFingerprint", func() string {
			if vmInfo.SshHostKeyInfo != nil {
				return vmInfo.SshHostKeyInfo.Fingerprint
			}
			return ""
		}()).
		Msg("Resetting SSH host key for VM")

	vmInfo.SshHostKeyInfo = nil

	UpdateVmInfo(nsId, mciId, vmInfo)

	return nil
}

// GetVmSshHostKey returns the stored SSH host key information for a VM
func GetVmSshHostKey(nsId string, mciId string, vmId string) (model.SshHostKeyInfo, error) {
	err := common.CheckString(nsId)
	if err != nil {
		return model.SshHostKeyInfo{}, fmt.Errorf("invalid nsId: %w", err)
	}
	err = common.CheckString(mciId)
	if err != nil {
		return model.SshHostKeyInfo{}, fmt.Errorf("invalid mciId: %w", err)
	}
	err = common.CheckString(vmId)
	if err != nil {
		return model.SshHostKeyInfo{}, fmt.Errorf("invalid vmId: %w", err)
	}

	vmInfo, err := GetVmObject(nsId, mciId, vmId)
	if err != nil {
		return model.SshHostKeyInfo{}, fmt.Errorf("failed to get VM info: %w", err)
	}

	if vmInfo.SshHostKeyInfo == nil {
		return model.SshHostKeyInfo{}, nil
	}

	return *vmInfo.SshHostKeyInfo, nil
}

// runSSH func execute a command by SSH
// bastionCtx and targetCtx are used for TOFU host key verification
func runSSH(bastionInfo model.SshInfo, targetInfo model.SshInfo, cmds []string, bastionCtx tofuContext, targetCtx tofuContext) (map[int]string, map[int]string, error) {
	stdoutMap := make(map[int]string)
	stderrMap := make(map[int]string)

	// Log connection details for debugging DNS issues
	log.Debug().
		Str("bastionEndpoint", bastionInfo.EndPoint).
		Str("bastionUserName", bastionInfo.UserName).
		Str("targetEndpoint", targetInfo.EndPoint).
		Str("targetUserName", targetInfo.UserName).
		Msg("SSH connection attempt details")

	// Parse the private key for the bastion host
	bastionSigner, err := ssh.ParsePrivateKey(bastionInfo.PrivateKey)
	if err != nil {
		return stdoutMap, stderrMap, fmt.Errorf("failed to parse bastion private key: %v", err)
	}

	// Create an SSH client configuration for the bastion host with TOFU host key verification
	bastionConfig := &ssh.ClientConfig{
		User: bastionInfo.UserName,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(bastionSigner),
		},
		HostKeyCallback: createTOFUHostKeyCallback(bastionCtx),
		Timeout:         30 * time.Second,
	}

	// Parse the private key for the target host
	targetSigner, err := ssh.ParsePrivateKey(targetInfo.PrivateKey)
	if err != nil {
		return stdoutMap, stderrMap, err
	}

	// Create an SSH client configuration for the target host with TOFU host key verification
	targetConfig := &ssh.ClientConfig{
		User: targetInfo.UserName,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(targetSigner),
		},
		HostKeyCallback: createTOFUHostKeyCallback(targetCtx),
		Timeout:         30 * time.Second,
	}

	targetHost, targetPort, err := net.SplitHostPort(targetInfo.EndPoint)
	if err != nil {
		return stdoutMap, stderrMap, fmt.Errorf("invalid target endpoint format: %v", err)
	}

	log.Info().Msgf("Attempting to connect to target host %s:%s via bastion", targetHost, targetPort)

	retryCount := 3
	initialTimeout := 20 * time.Second
	maxTimeout := 60 * time.Second
	var bastionClient *ssh.Client
	var conn net.Conn
	var lastErr error

	for i := range retryCount {
		// Fix timeout calculation: start with initialTimeout for first attempt (i=0)
		// then progressively increase for subsequent attempts
		timeout := min(time.Duration(float64(initialTimeout)*(1.0+0.5*float64(i))), maxTimeout)

		log.Debug().Msgf("[Check Target via Bastion] %v:%v (Attempt %d/%d, Timeout: %v)",
			targetHost, targetPort, i+1, retryCount, timeout)

		ctx, cancel := context.WithTimeout(context.Background(), timeout)

		connCh := make(chan net.Conn, 1)
		errCh := make(chan error, 1)
		sshClientCh := make(chan *ssh.Client, 1)

		go func() {
			// Setup the bastion host connection
			log.Debug().Str("bastionEndpoint", bastionInfo.EndPoint).Msg("Attempting to dial bastion host")
			client, err := ssh.Dial("tcp", bastionInfo.EndPoint, bastionConfig)
			if err != nil {
				err = fmt.Errorf("failed to establish SSH connection to bastion host: %v", err)
				log.Error().
					Str("bastionEndpoint", bastionInfo.EndPoint).
					Str("bastionUserName", bastionInfo.UserName).
					Err(err).
					Msg("Bastion SSH connection failed")
				errCh <- err
				return
			}
			log.Debug().Str("bastionEndpoint", bastionInfo.EndPoint).Msg("Successfully connected to bastion host")

			sshClientCh <- client

			log.Debug().Str("targetEndpoint", targetInfo.EndPoint).Msg("Attempting to dial target host via bastion")
			targetConn, err := client.Dial("tcp", targetInfo.EndPoint)
			if err != nil {
				client.Close()
				log.Error().
					Str("targetEndpoint", targetInfo.EndPoint).
					Err(err).
					Msg("Target connection via bastion failed")
				errCh <- err
				return
			}
			log.Debug().Str("targetEndpoint", targetInfo.EndPoint).Msg("Successfully connected to target host via bastion")

			connCh <- targetConn
		}()

		select {
		case conn = <-connCh:
			bastionClient = <-sshClientCh
			cancel()
			log.Info().Msgf("Successfully connected to target host on attempt %d", i+1)
			goto CONNECTION_ESTABLISHED
		case err := <-errCh:
			cancel()
			lastErr = err
			waitTime := time.Duration(3) * time.Second
			log.Warn().Err(err).Msgf("Failed to connect to target host. Attempt %d/%d. Retrying in %v...",
				i+1, retryCount, waitTime)
			time.Sleep(waitTime)
		case <-ctx.Done():
			cancel()
			lastErr = ctx.Err()
			waitTime := time.Duration(3) * time.Second
			log.Warn().Err(lastErr).Msgf("Connection timeout. Attempt %d/%d. Retrying in %v...",
				i+1, retryCount, waitTime)
			time.Sleep(waitTime)
		}
	}

	return stdoutMap, stderrMap, fmt.Errorf("failed to connect to target host via bastion after %d attempts: %v", retryCount, lastErr)

CONNECTION_ESTABLISHED:
	defer bastionClient.Close()
	defer conn.Close()

	log.Debug().Msgf("Establishing SSH connection to target host with user: %s", targetInfo.UserName)

	if len(targetInfo.PrivateKey) == 0 {
		return stdoutMap, stderrMap, fmt.Errorf("empty private key for target host")
	}

	var ncc ssh.Conn
	var chans <-chan ssh.NewChannel
	var reqs <-chan *ssh.Request
	sshRetryCount := 3
	var lastSSHErr error

	for i := 0; i < sshRetryCount; i++ {
		ncc, chans, reqs, err = ssh.NewClientConn(conn, targetInfo.EndPoint, targetConfig)
		if err == nil {
			break
		}

		lastSSHErr = err
		log.Warn().Err(err).Msgf("SSH authentication failed. Attempt %d/%d", i+1, sshRetryCount)

		if strings.Contains(err.Error(), "handshake failed") ||
			strings.Contains(err.Error(), "no supported methods remain") {
			waitTime := time.Duration(3*(i+1)) * time.Second
			log.Info().Msgf("Waiting for SSH daemon to initialize. Retrying in %v...", waitTime)
			time.Sleep(waitTime)
		} else {
			break
		}
	}

	if err != nil {
		log.Error().Str("user", targetInfo.UserName).
			Str("endpoint", targetInfo.EndPoint).
			Err(lastSSHErr).Msg("SSH authentication failed")

		if strings.Contains(lastSSHErr.Error(), "no supported methods remain") {
			return stdoutMap, stderrMap, fmt.Errorf("SSH authentication failed. Please check: 1) private key is valid 2) user '%s' exists on target 3) authorized_keys is properly configured", targetInfo.UserName)
		}

		return stdoutMap, stderrMap, fmt.Errorf("failed to establish SSH connection to target host: %v", lastSSHErr)
	}

	log.Info().Msgf("SSH connection established successfully to %s as user %s", targetInfo.EndPoint, targetInfo.UserName)
	client := ssh.NewClient(ncc, chans, reqs)
	defer client.Close()

	// Run the commands
	for i, cmd := range cmds {
		// Create a new SSH session for each command
		session, err := client.NewSession()
		if err != nil {
			return stdoutMap, stderrMap, err
		}
		defer session.Close() // Ensure session is closed

		// Get pipes for stdout and stderr
		stdoutPipe, err := session.StdoutPipe()
		if err != nil {
			return stdoutMap, stderrMap, err
		}

		stderrPipe, err := session.StderrPipe()
		if err != nil {
			return stdoutMap, stderrMap, err
		}

		// Start the command
		if err := session.Start(cmd); err != nil {
			return stdoutMap, stderrMap, err
		}

		// Read stdout and stderr
		var stdoutBuf, stderrBuf bytes.Buffer
		stdoutDone := make(chan struct{})
		stderrDone := make(chan struct{})

		go func() {
			io.Copy(io.MultiWriter(os.Stdout, &stdoutBuf), stdoutPipe)
			close(stdoutDone)
		}()

		go func() {
			io.Copy(io.MultiWriter(os.Stderr, &stderrBuf), stderrPipe)
			close(stderrDone)
		}()

		// Wait for the command to finish
		err = session.Wait()
		<-stdoutDone
		<-stderrDone

		if err != nil {
			stderrMap[i] = fmt.Sprintf("(%s)\nStderr: %s", err, stderrBuf.String())
			stdoutMap[i] = stdoutBuf.String()
			break
		}

		stdoutMap[i] = stdoutBuf.String()
		stderrMap[i] = stderrBuf.String()
	}

	return stdoutMap, stderrMap, nil
}

// TransferFileToMci is a function to transfer a file to all VMs in MCI by SSH through bastion hosts
func TransferFileToMci(nsId string, mciId string, subGroupId string, vmId string, fileData []byte, fileName string, targetPath string) ([]model.SshCmdResult, error) {
	// Get the list of VMs in the MCI
	vmList, err := ListVmId(nsId, mciId)
	if err != nil {
		return nil, err
	}
	// If a subGroupId is provided, filter the VM list by subGroup
	if subGroupId != "" {
		vmListInGroup, err := ListVmBySubGroup(nsId, mciId, subGroupId)
		if err != nil {
			return nil, err
		}
		vmList = vmListInGroup
	}
	// If a specific vmId is provided, limit the transfer to that VM only
	if vmId != "" {
		vmList = []string{vmId}
	}

	// Create a wait group to sync goroutines
	var wg sync.WaitGroup
	var resultArray []model.SshCmdResult
	var resultMutex sync.Mutex // To safely append to resultArray in concurrent goroutines

	// Iterate over the VM list to transfer the file
	for _, vmId := range vmList {
		wg.Add(1)
		go func(vmId string) {
			defer wg.Done()
			log.Info().Msgf("Transferring file to VM: %s", vmId)

			_, targetVmIP, targetSshPort, err := GetVmIp(nsId, mciId, vmId)

			// Create the result for this VM
			result := model.SshCmdResult{
				MciId:   mciId,
				VmId:    vmId,
				VmIp:    targetVmIP,
				Command: map[int]string{0: fmt.Sprintf("scp %s to %s", fileName, targetPath)},
				Stdout:  map[int]string{},
				Stderr:  map[int]string{},
			}

			if err != nil {
				result.Err = err
				result.Stderr[0] = fmt.Sprintf("Failed to get VM IP: %v", err)
				resultMutex.Lock()
				resultArray = append(resultArray, result)
				resultMutex.Unlock()
				return
			}

			// Check VM status before executing file transfer
			vmInfo, err := GetVmObject(nsId, mciId, vmId)
			if err != nil {
				result.Err = fmt.Errorf("failed to get VM status: %v", err)
				result.Stderr[0] = fmt.Sprintf("Failed to get VM status: %v", err)
				resultMutex.Lock()
				resultArray = append(resultArray, result)
				resultMutex.Unlock()
				return
			}

			// Validate VM status for file transfer
			if vmInfo.Status != model.StatusRunning {
				var errorMsg string
				if vmInfo.Status == model.StatusTerminated {
					errorMsg = fmt.Sprintf("VM '%s' is in '%s' status. File transfer is impossible for terminated VMs", vmId, vmInfo.Status)
				} else {
					errorMsg = fmt.Sprintf("VM '%s' is in '%s' status (not Running). Please change the VM status to Running and try again", vmId, vmInfo.Status)
				}
				result.Err = fmt.Errorf(errorMsg)
				result.Stderr[0] = errorMsg
				resultMutex.Lock()
				resultArray = append(resultArray, result)
				resultMutex.Unlock()
				return
			}

			targetUserName, targetPrivateKey, err := VerifySshUserName(nsId, mciId, vmId, targetVmIP, targetSshPort, "")
			if err != nil {
				result.Err = fmt.Errorf("failed to verify SSH username: %v", err)
				result.Stderr[0] = fmt.Sprintf("Failed to verify SSH username: %v", err)
				resultMutex.Lock()
				resultArray = append(resultArray, result)
				resultMutex.Unlock()
				return
			}

			targetSshInfo := model.SshInfo{
				EndPoint:   fmt.Sprintf("%s:%s", targetVmIP, targetSshPort),
				UserName:   targetUserName,
				PrivateKey: []byte(targetPrivateKey),
			}

			// Transfer file to the VM via bastion
			err = transferFileToVmViaBastion(nsId, mciId, vmId, targetSshInfo, fileData, fileName, targetPath)

			if err != nil {
				result.Stderr[0] = fmt.Sprintf("Failed to transfer file: %v", err)
				result.Err = fmt.Errorf("file transfer failed: %v", err)
				log.Error().Err(err).Msgf("Failed to transfer file to VM: %s", vmId)
			} else {
				result.Stdout[0] = fmt.Sprintf("File transfer successful: %s%s", targetPath, fileName)
				log.Info().Msgf("Successfully transferred file to VM: %s", vmId)
			}

			// Safely append to resultArray
			resultMutex.Lock()
			resultArray = append(resultArray, result)
			resultMutex.Unlock()
		}(vmId)
	}
	wg.Wait()

	return resultArray, nil
}

// transferFileToVmViaBastion is a function to transfer a file to a specific VM via Bastion Host
func transferFileToVmViaBastion(nsId string, mciId string, vmId string, targetSshInfo model.SshInfo, fileData []byte, fileName string, targetPath string) error {

	bastionNodes, err := GetBastionNodes(nsId, mciId, vmId)
	if err != nil || len(bastionNodes) == 0 {
		return fmt.Errorf("failed to get bastion nodes: %v", err)
	}

	bastionNode := bastionNodes[0]
	bastionIp, _, bastionSshPort, err := GetVmIp(nsId, bastionNode.MciId, bastionNode.VmId)
	if err != nil {
		return fmt.Errorf("failed to get bastion VM IP and SSH port: %v", err)
	}

	bastionUserName, bastionPrivateKey, err := VerifySshUserName(nsId, bastionNode.MciId, bastionNode.VmId, bastionIp, bastionSshPort, "")
	if err != nil {
		return fmt.Errorf("failed to verify SSH username for bastion: %v", err)
	}

	bastionSshInfo := model.SshInfo{
		EndPoint:   fmt.Sprintf("%s:%s", bastionIp, bastionSshPort),
		UserName:   bastionUserName,
		PrivateKey: []byte(bastionPrivateKey),
	}

	// Set TOFU context for bastion and target VMs
	bastionCtx := tofuContext{
		NsId:  nsId,
		MciId: bastionNode.MciId,
		VmId:  bastionNode.VmId,
	}
	targetCtx := tofuContext{
		NsId:  nsId,
		MciId: mciId,
		VmId:  vmId,
	}

	err = runSCPWithBastion(bastionSshInfo, targetSshInfo, fileData, fileName, targetPath, bastionCtx, targetCtx)
	if err != nil {
		return fmt.Errorf("failed to transfer file to VM via bastion: %v", err)
	}

	log.Info().Msgf("File successfully transferred to VM %s via bastion", vmId)
	return nil
}

// runSCPWithBastion is func to send a file using SCP over SSH via a Bastion host
// bastionCtx and targetCtx are used for TOFU host key verification
func runSCPWithBastion(bastionInfo model.SshInfo, targetInfo model.SshInfo, fileData []byte, fileName string, targetPath string, bastionCtx tofuContext, targetCtx tofuContext) error {
	log.Info().Msg("Setting up SCP connection via Bastion Host")

	// Parse the private key for the bastion host
	bastionSigner, err := ssh.ParsePrivateKey(bastionInfo.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to parse bastion private key: %v", err)
	}

	// Create an SSH client configuration for the bastion host with TOFU host key verification
	bastionConfig := &ssh.ClientConfig{
		User: bastionInfo.UserName,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(bastionSigner),
		},
		HostKeyCallback: createTOFUHostKeyCallback(bastionCtx),
	}

	// Parse the private key for the target host
	targetSigner, err := ssh.ParsePrivateKey(targetInfo.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to parse target private key: %v", err)
	}

	// Create an SSH client configuration for the target host with TOFU host key verification
	targetConfig := &ssh.ClientConfig{
		User: targetInfo.UserName,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(targetSigner),
		},
		HostKeyCallback: createTOFUHostKeyCallback(targetCtx),
	}

	// Setup the bastion host connection
	bastionClient, err := ssh.Dial("tcp", bastionInfo.EndPoint, bastionConfig)
	if err != nil {
		return fmt.Errorf("failed to dial bastion: %v", err)
	}
	defer bastionClient.Close()

	// Setup the actual SSH client through the bastion host
	conn, err := bastionClient.Dial("tcp", targetInfo.EndPoint)
	if err != nil {
		return fmt.Errorf("failed to dial target via bastion: %v", err)
	}

	ncc, chans, reqs, err := ssh.NewClientConn(conn, targetInfo.EndPoint, targetConfig)
	if err != nil {
		return fmt.Errorf("failed to create target SSH connection: %v", err)
	}
	client := ssh.NewClient(ncc, chans, reqs)
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %v", err)
	}
	defer session.Close()

	// Set up pipes for capturing stdout and stderr
	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to set up stdout pipe: %v", err)
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to set up stderr pipe: %v", err)
	}

	// Set up stdin pipe for SCP data transfer
	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to set up stdin for SCP: %v", err)
	}

	// Construct the SCP command and log it
	targetFullPath := fmt.Sprintf("%s/%s", targetPath, fileName)
	cmd := fmt.Sprintf("scp -t '%s'", targetFullPath)
	log.Info().Msgf("Executing SCP command: %s", cmd)

	// Run the SCP command
	if err := session.Start(cmd); err != nil {
		stdin.Close() // Close stdin to signal error and exit early
		return fmt.Errorf("failed to start SCP command: %v", err)
	}

	// Send the file metadata (file size and permissions)
	fileSize := len(fileData)
	fmt.Fprintf(stdin, "C0644 %d %s\n", fileSize, fileName)

	// Log file data transfer initiation
	log.Info().Msgf("Sending file data: %s (size: %d)", fileName, fileSize)

	// Write the file data to the remote server
	_, err = stdin.Write(fileData)
	if err != nil {
		stdin.Close() // Close stdin to ensure resources are cleaned up
		return fmt.Errorf("failed to write file data: %v", err)
	}

	// End of file transmission (SCP protocol requires a 0-byte to signify EOF)
	fmt.Fprint(stdin, "\x00")

	// Close stdin explicitly before waiting for the session to complete
	stdin.Close()

	// Capture and log stdout and stderr
	stdoutBuf := new(bytes.Buffer)
	stderrBuf := new(bytes.Buffer)

	go io.Copy(stdoutBuf, stdout)
	go io.Copy(stderrBuf, stderr)

	// Wait for SCP session to complete and check for errors
	if err := session.Wait(); err != nil {
		// Log stdout and stderr for better error diagnostics
		log.Error().Msgf("SCP command failed with error: %v", err)
		log.Error().Msgf("SCP stdout: %s", stdoutBuf.String())
		log.Error().Msgf("SCP stderr: %s", stderrBuf.String())

		// Include stderr in the returned error
		return fmt.Errorf("SCP command failed: %v, stderr: %s", err, stderrBuf.String())
	}

	// Log success message after file transfer is complete
	log.Info().Msgf("File successfully transferred to %s via Bastion", targetFullPath)

	return nil
}

// SetBastionNodes func sets bastion nodes
func SetBastionNodes(nsId string, mciId string, targetVmId string, bastionVmId string) (string, error) {

	// Check if bastion node already exists for the target VM (for random assignment)
	currentBastion, err := GetBastionNodes(nsId, mciId, targetVmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}
	if len(currentBastion) > 0 && bastionVmId == "" {
		return "", fmt.Errorf("bastion node already exists for VM (ID: %s) in MCI (ID: %s) under namespace (ID: %s)",
			targetVmId, mciId, nsId)
	}

	vmObj, err := GetVmObject(nsId, mciId, targetVmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}

	res, err := resource.GetResource(nsId, model.StrVNet, vmObj.VNetId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}

	tempVNetInfo, ok := res.(model.VNetInfo)
	if !ok {
		log.Error().Err(err).Msg("")
		return "", err
	}

	// find subnet and append bastion node
	for i, subnetInfo := range tempVNetInfo.SubnetInfoList {
		if subnetInfo.Id == vmObj.SubnetId {

			if bastionVmId == "" {
				vmIdsInSubnet, err := ListVmByFilter(nsId, mciId, "SubnetId", subnetInfo.Id)
				if err != nil {
					log.Error().Err(err).Msg("")
					return "", fmt.Errorf("failed to list VMs in subnet (ID: %s): %w", subnetInfo.Id, err)
				}

				// Find a VM with public IP to use as bastion
				for _, v := range vmIdsInSubnet {
					tmpPublicIp, _, _, err := GetVmIp(nsId, mciId, v)
					if err != nil {
						log.Error().Err(err).Msgf("failed to get IP for VM %s", v)
						continue
					}
					if tmpPublicIp != "" {
						bastionVmId = v
						log.Info().Msgf("Selected VM %s as bastion (public IP: %s)", v, tmpPublicIp)
						break
					}
				}

				// If no suitable bastion VM found, return error
				if bastionVmId == "" {
					return "", fmt.Errorf("no VM with public IP found in subnet (ID: %s) to use as bastion", subnetInfo.Id)
				}
			} else {
				for _, existingId := range subnetInfo.BastionNodes {
					if existingId.VmId == bastionVmId {
						return fmt.Sprintf("Bastion (ID: %s) already exists in subnet (ID: %s) in VNet (ID: %s).",
							bastionVmId, subnetInfo.Id, vmObj.VNetId), nil
					}
				}
			}

			// Validate that we have a valid bastion VM ID before creating the node
			if bastionVmId == "" {
				return "", fmt.Errorf("failed to find a suitable bastion VM in subnet (ID: %s)", subnetInfo.Id)
			}

			bastionCandidate := model.BastionNode{MciId: mciId, VmId: bastionVmId}
			// Append bastionVmId only if it doesn't already exist.
			subnetInfo.BastionNodes = append(subnetInfo.BastionNodes, bastionCandidate)
			tempVNetInfo.SubnetInfoList[i] = subnetInfo
			resource.UpdateResourceObject(nsId, model.StrVNet, tempVNetInfo)

			return fmt.Sprintf("Successfully set the bastion (ID: %s) for subnet (ID: %s) in vNet (ID: %s) for VM (ID: %s) in MCI (ID: %s).",
				bastionVmId, subnetInfo.Id, vmObj.VNetId, targetVmId, mciId), nil
		}
	}
	return "", fmt.Errorf("failed to set bastion. Subnet (ID: %s) not found in VNet (ID: %s) for VM (ID: %s) in MCI (ID: %s) under namespace (ID: %s)",
		vmObj.SubnetId, vmObj.VNetId, targetVmId, mciId, nsId)
}

// RemoveBastionNodes func removes existing bastion nodes info
func RemoveBastionNodes(nsId string, mciId string, bastionVmId string) (string, error) {
	resourceListInNs, err := resource.ListResource(nsId, model.StrVNet, "mciId", mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	} else {
		vNets := resourceListInNs.([]model.VNetInfo) // type assertion
		for _, vNet := range vNets {
			removed := false
			for i, subnet := range vNet.SubnetInfoList {
				for j := len(subnet.BastionNodes) - 1; j >= 0; j-- {
					if subnet.BastionNodes[j].VmId == bastionVmId {
						subnet.BastionNodes = append(subnet.BastionNodes[:j], subnet.BastionNodes[j+1:]...)
						removed = true
					}
				}
				vNet.SubnetInfoList[i] = subnet
			}
			if removed {
				resource.UpdateResourceObject(nsId, model.StrVNet, vNet)
			}
		}
	}
	return fmt.Sprintf("Successfully removed the bastion (ID: %s) in MCI (ID: %s) from all subnets", bastionVmId, mciId), nil
}

// GetBastionNodes func retrieves bastion nodes for a given VM
func GetBastionNodes(nsId string, mciId string, targetVmId string) ([]model.BastionNode, error) {
	returnValue := []model.BastionNode{}
	// Fetch VM object based on nsId, mciId, and targetVmId
	vmObj, err := GetVmObject(nsId, mciId, targetVmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return returnValue, err
	}

	// Fetch VNet resource information
	res, err := resource.GetResource(nsId, model.StrVNet, vmObj.VNetId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return returnValue, err
	}

	// Type assertion for VNet information
	tempVNetInfo, ok := res.(model.VNetInfo)
	if !ok {
		log.Error().Err(err).Msg("")
		return returnValue, err
	}

	// Find the subnet corresponding to the VM and return the BastionNodeIds
	for _, subnetInfo := range tempVNetInfo.SubnetInfoList {
		if subnetInfo.Id == vmObj.SubnetId {
			if subnetInfo.BastionNodes == nil {
				return returnValue, nil
			}
			returnValue = subnetInfo.BastionNodes
			return returnValue, nil
		}
	}

	return returnValue, fmt.Errorf("failed to get bastion in Subnet (ID: %s) of VNet (ID: %s) for VM (ID: %s)",
		vmObj.SubnetId, vmObj.VNetId, targetVmId)
}

// Helper function to extract function name and parameters from the string
func extractFunctionAndParams(funcCall string) (string, map[string]string, error) {
	regex := regexp.MustCompile(`^\s*([a-zA-Z0-9]+)\((.*?)\)\s*$`)
	matches := regex.FindStringSubmatch(funcCall)
	if len(matches) < 3 {
		return "", nil, errors.New("built-in function error in command: no function found in command")
	}

	funcName := matches[1]
	paramsPart := matches[2]
	params := make(map[string]string)

	paramPairs := splitParams(paramsPart)

	for _, pair := range paramPairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			key := strings.TrimSpace(kv[0])
			value := kv[1]

			if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
				value = value[1 : len(value)-1]
			}

			params[key] = value
		}
	}

	return funcName, params, nil
}

// Helper function to split parameters by comma, considering quoted parts
func splitParams(paramsPart string) []string {
	var result []string
	var current strings.Builder
	inQuotes := false // Initialize inQuotes

	for i := 0; i < len(paramsPart); i++ {
		switch paramsPart[i] {
		case '\'':
			inQuotes = !inQuotes
			current.WriteByte(paramsPart[i])
		case ',':
			if inQuotes {
				current.WriteByte(paramsPart[i])
			} else {
				result = append(result, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(paramsPart[i])
		}
	}
	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result
}

// processCommand processes a command string and replaces all $$Func(...) occurrences with their computed values
func processCommand(command, nsId, mciId, vmId string, vmIndex int) (string, error) {
	// Keep track of the processed command throughout iterations
	processedCommand := command

	// Safety measure to prevent infinite loops
	maxIterations := 100
	iterCount := 0

	for iterCount < maxIterations {
		iterCount++

		// Look for the next function call pattern
		funcStartIndex := strings.Index(processedCommand, "$$Func(")
		if funcStartIndex == -1 {
			// No more function calls to process
			break
		}

		// Start position of the actual function content (after $$Func()
		contentStartIndex := funcStartIndex + 7

		// Match parentheses to find the correct ending position
		bracketCount := 1
		contentEndIndex := -1

		for i := contentStartIndex; i < len(processedCommand); i++ {
			if processedCommand[i] == '(' {
				bracketCount++
			} else if processedCommand[i] == ')' {
				bracketCount--
				if bracketCount == 0 {
					contentEndIndex = i
					break
				}
			}
		}

		if contentEndIndex == -1 {
			return "", errors.New("built-in function error in command: no matching parenthesis found")
		}

		// Extract the function call content
		funcCall := processedCommand[contentStartIndex:contentEndIndex]

		// Parse function name and parameters
		funcName, params, err := extractFunctionAndParams(funcCall)
		if err != nil {
			return "", err
		}

		// Process different built-in functions
		var replacement string
		if strings.EqualFold(funcName, "GetPublicIP") || strings.EqualFold(funcName, "GetPrivateIP") {
			targetMciId := mciId
			targetVmId := vmId
			if val, ok := params["target"]; ok {
				parts := strings.Split(val, ".")
				if len(parts) == 2 {
					targetMciId = parts[0]
					targetVmId = parts[1]
					if targetMciId == "this" {
						targetMciId = mciId
					}
					if targetVmId == "this" {
						targetVmId = vmId
					}
					// if targetVm or targetMci is not specified, return error
					if targetMciId == "" || targetVmId == "" {
						return "", fmt.Errorf("built-in function %s error: target MCI or VM %s is invalid", funcName, val)
					}

				} else if strings.EqualFold(val, "this") {
					targetMciId = mciId
					targetVmId = vmId
				}
			}
			prefix := ""
			if pre, ok := params["prefix"]; ok {
				prefix = pre
			}
			postfix := ""
			if post, ok := params["postfix"]; ok {
				postfix = post
			}
			if strings.EqualFold(funcName, "GetPublicIP") {
				// Logic for GetPublicIP function
				replacement, err = replaceWithPublicIP(nsId, targetMciId, targetVmId, prefix, postfix)
			} else {
				// Logic for GetPrivateIP function
				replacement, err = replaceWithPrivateIP(nsId, targetMciId, targetVmId, prefix, postfix)
			}
			if err != nil {
				return "", fmt.Errorf("built-in function GetPublicIP error: %s", err.Error())
			}
		} else if strings.EqualFold(funcName, "GetPublicIPs") || strings.EqualFold(funcName, "GetPrivateIPs") {
			// Logic for GetPublicIPs function
			targetMciId := mciId
			if val, ok := params["target"]; ok {
				if strings.EqualFold(val, "this") {
					targetMciId = mciId
				} else {
					targetMciId = val
				}
			}
			separator := ","
			if sep, ok := params["separator"]; ok {
				separator = sep
			}
			prefix := ""
			if pre, ok := params["prefix"]; ok {
				prefix = pre
			}
			postfix := ""
			if post, ok := params["postfix"]; ok {
				postfix = post
			}
			if strings.EqualFold(funcName, "GetPublicIPs") {
				replacement, err = replaceWithPublicIPs(nsId, targetMciId, separator, prefix, postfix)
			} else {
				replacement, err = replaceWithPrivateIPs(nsId, targetMciId, separator, prefix, postfix)
			}
			if err != nil {
				return "", fmt.Errorf("built-in function getPublicIPs error: %s", err.Error())
			}
		} else if strings.EqualFold(funcName, "AssignTask") {
			// Logic for AssignTask function
			taskListParam, ok := params["task"]
			if !ok {
				return "", fmt.Errorf("built-in function AssignTask error: no task list provided")
			}
			tasks := splitParams(taskListParam)
			replacement = tasks[vmIndex%len(tasks)]
		} else if strings.EqualFold(funcName, "GetNsId") {
			// Logic for getNsId function
			prefix := ""
			if pre, ok := params["prefix"]; ok {
				prefix = pre
			}
			postfix := ""
			if post, ok := params["postfix"]; ok {
				postfix = post
			}
			replacement = replaceWithId(nsId, prefix, postfix)
		} else if strings.EqualFold(funcName, "GetMciId") {
			// Logic for getMciId function
			prefix := ""
			if pre, ok := params["prefix"]; ok {
				prefix = pre
			}
			postfix := ""
			if post, ok := params["postfix"]; ok {
				postfix = post
			}
			replacement = replaceWithId(mciId, prefix, postfix)
		} else if strings.EqualFold(funcName, "GetVmId") {
			// Logic for getVmId function
			prefix := ""
			if pre, ok := params["prefix"]; ok {
				prefix = pre
			}
			postfix := ""
			if post, ok := params["postfix"]; ok {
				postfix = post
			}
			replacement = replaceWithId(vmId, prefix, postfix)
		} else {
			return "", fmt.Errorf("built-in function error in command: unknown function: %s", funcName)
		}

		// Replace the entire function call with its result in the processed command
		processedCommand = processedCommand[:funcStartIndex] + replacement + processedCommand[contentEndIndex+1:]
	}

	// Safety check for possible infinite loops
	if iterCount >= maxIterations {
		return "", errors.New("built-in function error: too many iterations, possible infinite loop")
	}

	return processedCommand, nil
}

// Built-in functions for remote command
// replaceWithPublicIP function to get and replace string with the public IP of the target
func replaceWithPublicIP(nsId, mciId, vmId, prefix, postfix string) (string, error) {
	vmStatus, err := GetVmCurrentPublicIp(nsId, mciId, vmId)
	if err != nil {
		return "", err
	}
	ip := vmStatus.PublicIp
	return prefix + ip + postfix, err
}

// replaceWithPrivateIP function to get and replace string with the private IP of the target
func replaceWithPrivateIP(nsId, mciId, vmId, prefix, postfix string) (string, error) {
	vmStatus, err := GetVmCurrentPublicIp(nsId, mciId, vmId)
	if err != nil {
		return "", err
	}
	ip := vmStatus.PrivateIp
	return prefix + ip + postfix, err
}

// replaceWithPublicIPs function to get and replace string with the public IP list of the target
func replaceWithPublicIPs(nsId, mciId, separator, prefix, postfix string) (string, error) {
	mciStatus, err := GetMciStatus(nsId, mciId)
	if err != nil {
		return "", err
	}
	ips := make([]string, len(mciStatus.Vm))
	for i, vmStatus := range mciStatus.Vm {
		ips[i] = prefix + vmStatus.PublicIp + postfix
	}
	return strings.Join(ips, separator), nil
}

// replaceWithPrivateIPs function to get and replace string with the Private IP list of the target
func replaceWithPrivateIPs(nsId, mciId, separator, prefix, postfix string) (string, error) {
	mciStatus, err := GetMciStatus(nsId, mciId)
	if err != nil {
		return "", err
	}
	ips := make([]string, len(mciStatus.Vm))
	for i, vmStatus := range mciStatus.Vm {
		ips[i] = prefix + vmStatus.PrivateIp + postfix
	}
	return strings.Join(ips, separator), nil
}

// replaceWithId function to replace string with the prefix and postfix
func replaceWithId(id, prefix, postfix string) string {
	return prefix + id + postfix
}

// Command Status Management Functions

// updateVmCommandStatusSafe safely updates only CommandStatus field of VM with proper locking
func updateVmCommandStatusSafe(nsId, mciId, vmId string, updateFunc func(*[]model.CommandStatusInfo) error) error {
	// Use the same mutex as UpdateVmInfo for consistency
	key := common.GenMciKey(nsId, mciId, vmId)

	// Retry mechanism for concurrent access
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Get current VM info
		keyValue, exists, err := kvstore.GetKv(key)
		if !exists || err != nil {
			return fmt.Errorf("failed to get VM info: %v", err)
		}

		vmInfo := model.VmInfo{}
		err = json.Unmarshal([]byte(keyValue.Value), &vmInfo)
		if err != nil {
			return fmt.Errorf("failed to unmarshal VM info: %v", err)
		}

		// Apply the update function to CommandStatus
		originalCommandStatus := make([]model.CommandStatusInfo, len(vmInfo.CommandStatus))
		copy(originalCommandStatus, vmInfo.CommandStatus)

		err = updateFunc(&vmInfo.CommandStatus)
		if err != nil {
			return err
		}

		// Only update if CommandStatus actually changed
		if reflect.DeepEqual(originalCommandStatus, vmInfo.CommandStatus) {
			return nil // No change needed
		}

		// Atomic update
		vmJson, err := json.Marshal(vmInfo)
		if err != nil {
			return fmt.Errorf("failed to marshal VM info: %v", err)
		}

		err = kvstore.Put(key, string(vmJson))
		if err != nil {
			if attempt < maxRetries-1 {
				// Retry on failure (might be concurrent update)
				time.Sleep(time.Millisecond * 100 * time.Duration(attempt+1))
				continue
			}
			return fmt.Errorf("failed to update VM info after %d attempts: %v", maxRetries, err)
		}

		return nil
	}

	return fmt.Errorf("failed to update VM CommandStatus after %d retries", maxRetries)
}

// Helper function to get next command index
func getNextCommandIndex(commandStatus []model.CommandStatusInfo) int {
	nextIndex := 1
	if len(commandStatus) > 0 {
		// Find the maximum index and increment
		maxIndex := 0
		for _, cmd := range commandStatus {
			if cmd.Index > maxIndex {
				maxIndex = cmd.Index
			}
		}
		nextIndex = maxIndex + 1
	}
	return nextIndex
}

// Helper function to find command by index
func findCommandByIndex(commandStatus []model.CommandStatusInfo, index int) (*model.CommandStatusInfo, int) {
	for i := range commandStatus {
		if commandStatus[i].Index == index {
			return &commandStatus[i], i
		}
	}
	return nil, -1
}

// Helper function to filter commands based on criteria
func filterCommands(commandStatus []model.CommandStatusInfo, filter *model.CommandStatusFilter) []model.CommandStatusInfo {
	if filter == nil {
		return commandStatus
	}

	var filtered []model.CommandStatusInfo

	for _, cmd := range commandStatus {
		// Apply status filter - check if command status is in the allowed list
		if len(filter.Status) > 0 {
			found := false
			for _, status := range filter.Status {
				if cmd.Status == status {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		if filter.XRequestId != "" && cmd.XRequestId != filter.XRequestId {
			continue
		}
		if filter.CommandContains != "" && !strings.Contains(cmd.CommandRequested, filter.CommandContains) {
			continue
		}
		if filter.StartTimeFrom != "" {
			startTime, err := time.Parse(time.RFC3339, cmd.StartedTime)
			if err != nil {
				continue
			}
			filterTime, err := time.Parse(time.RFC3339, filter.StartTimeFrom)
			if err != nil {
				continue
			}
			if startTime.Before(filterTime) {
				continue
			}
		}
		if filter.StartTimeTo != "" {
			startTime, err := time.Parse(time.RFC3339, cmd.StartedTime)
			if err != nil {
				continue
			}
			filterTime, err := time.Parse(time.RFC3339, filter.StartTimeTo)
			if err != nil {
				continue
			}
			if startTime.After(filterTime) {
				continue
			}
		}

		// Apply index range filters
		if filter.IndexFrom > 0 && cmd.Index < filter.IndexFrom {
			continue
		}
		if filter.IndexTo > 0 && cmd.Index > filter.IndexTo {
			continue
		}

		filtered = append(filtered, cmd)
	}

	return filtered
}

// Helper function to apply pagination
func applyPagination(commandStatus []model.CommandStatusInfo, offset, limit int) []model.CommandStatusInfo {
	if offset >= len(commandStatus) {
		return []model.CommandStatusInfo{}
	}

	end := offset + limit
	if end > len(commandStatus) {
		end = len(commandStatus)
	}

	return commandStatus[offset:end]
}

// AddCommandStatusInfo adds a new command status record to VM's command history
func AddCommandStatusInfo(nsId, mciId, vmId, xRequestId, commandRequested, commandExecuted string) (int, error) {
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}
	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}
	err = common.CheckString(vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}

	var nextIndex int

	err = updateVmCommandStatusSafe(nsId, mciId, vmId, func(commandStatus *[]model.CommandStatusInfo) error {
		// Generate next index using helper function
		nextIndex = getNextCommandIndex(*commandStatus)

		// Create new command status info
		newCommandStatus := model.CommandStatusInfo{
			Index:            nextIndex,
			XRequestId:       xRequestId,
			CommandRequested: commandRequested,
			CommandExecuted:  commandExecuted,
			Status:           model.CommandStatusQueued,
			StartedTime:      time.Now().Format(time.RFC3339),
		}

		// Add to command status list
		*commandStatus = append(*commandStatus, newCommandStatus)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}

	log.Info().
		Str("nsId", nsId).
		Str("mciId", mciId).
		Str("vmId", vmId).
		Int("index", nextIndex).
		Str("xRequestId", xRequestId).
		Msg("Command status added")

	return nextIndex, nil
}

// UpdateCommandStatusInfo updates an existing command status record
func UpdateCommandStatusInfo(nsId, mciId, vmId string, index int, status model.CommandExecutionStatus, resultSummary, errorMessage, stdout, stderr string) error {
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
	err = common.CheckString(vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	err = updateVmCommandStatusSafe(nsId, mciId, vmId, func(commandStatus *[]model.CommandStatusInfo) error {
		// Find the command status by index using helper function
		cmdStatus, cmdIndex := findCommandByIndex(*commandStatus, index)
		if cmdStatus == nil {
			return fmt.Errorf("command with index %d not found for VM (ID: %s)", index, vmId)
		}

		// Update status and completion time
		startTime, _ := time.Parse(time.RFC3339, cmdStatus.StartedTime)
		currentTime := time.Now()

		(*commandStatus)[cmdIndex].Status = status

		// Only set CompletedTime for final states (Completed, Failed, Timeout)
		if status == model.CommandStatusCompleted ||
			status == model.CommandStatusFailed ||
			status == model.CommandStatusTimeout {
			(*commandStatus)[cmdIndex].CompletedTime = currentTime.Format(time.RFC3339)
		}

		// Calculate elapsed time in seconds (not milliseconds)
		(*commandStatus)[cmdIndex].ElapsedTime = int64(currentTime.Sub(startTime).Seconds())
		(*commandStatus)[cmdIndex].ResultSummary = resultSummary
		(*commandStatus)[cmdIndex].ErrorMessage = errorMessage

		// Truncate output if too long (limit to 1000 characters for history)
		if len(stdout) > 1000 {
			(*commandStatus)[cmdIndex].Stdout = stdout[:1000] + "...(truncated)"
		} else {
			(*commandStatus)[cmdIndex].Stdout = stdout
		}

		if len(stderr) > 1000 {
			(*commandStatus)[cmdIndex].Stderr = stderr[:1000] + "...(truncated)"
		} else {
			(*commandStatus)[cmdIndex].Stderr = stderr
		}

		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	log.Info().
		Str("nsId", nsId).
		Str("mciId", mciId).
		Str("vmId", vmId).
		Int("index", index).
		Str("status", string(status)).
		Msg("Command status updated")

	return nil
}

// GetCommandStatusInfo retrieves a specific command status record
func GetCommandStatusInfo(nsId, mciId, vmId string, index int) (*model.CommandStatusInfo, error) {
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
	err = common.CheckString(vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	// Use existing GetVmObject function instead of direct kvstore access
	vmInfo, err := GetVmObject(nsId, mciId, vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	// Find the command status by index using helper function
	cmdStatus, _ := findCommandByIndex(vmInfo.CommandStatus, index)
	if cmdStatus == nil {
		return nil, fmt.Errorf("command with index %d not found for VM (ID: %s)", index, vmId)
	}

	// For "Handling" status, calculate real-time elapsed time
	if cmdStatus.Status == model.CommandStatusHandling && cmdStatus.StartedTime != "" {
		if startTime, err := time.Parse(time.RFC3339, cmdStatus.StartedTime); err == nil {
			// Create a copy of the command status to avoid modifying the original
			realtimeCmdStatus := *cmdStatus
			realtimeCmdStatus.ElapsedTime = int64(time.Since(startTime).Seconds())
			return &realtimeCmdStatus, nil
		}
	}

	return cmdStatus, nil
}

// ListCommandStatusInfo retrieves command status records with filtering
func ListCommandStatusInfo(nsId, mciId, vmId string, filter *model.CommandStatusFilter) (*model.CommandStatusListResponse, error) {
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
	err = common.CheckString(vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	// Use existing GetVmObject function instead of direct kvstore access
	vmInfo, err := GetVmObject(nsId, mciId, vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	// Apply filters using helper function
	filteredCommands := filterCommands(vmInfo.CommandStatus, filter)
	total := len(filteredCommands)

	// Apply pagination using helper function
	offset := 0
	limit := 50 // Default limit
	if filter != nil {
		if filter.Offset > 0 {
			offset = filter.Offset
		}
		if filter.Limit > 0 {
			limit = filter.Limit
		}
	}

	paginatedCommands := applyPagination(filteredCommands, offset, limit)

	// Apply real-time elapsed time calculation for "Handling" status commands
	for i := range paginatedCommands {
		if paginatedCommands[i].Status == model.CommandStatusHandling && paginatedCommands[i].StartedTime != "" {
			if startTime, err := time.Parse(time.RFC3339, paginatedCommands[i].StartedTime); err == nil {
				paginatedCommands[i].ElapsedTime = int64(time.Since(startTime).Seconds())
			}
		}
	}

	response := &model.CommandStatusListResponse{
		Commands: paginatedCommands,
		Total:    total,
		Offset:   offset,
		Limit:    limit,
	}

	return response, nil
}

// DeleteCommandStatusInfo deletes a specific command status record
func DeleteCommandStatusInfo(nsId, mciId, vmId string, index int) error {
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
	err = common.CheckString(vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	err = updateVmCommandStatusSafe(nsId, mciId, vmId, func(commandStatus *[]model.CommandStatusInfo) error {
		// Find and remove the command status by index
		_, cmdIndex := findCommandByIndex(*commandStatus, index)
		if cmdIndex == -1 {
			return fmt.Errorf("command with index %d not found for VM (ID: %s)", index, vmId)
		}

		// Remove the command from slice
		*commandStatus = append((*commandStatus)[:cmdIndex], (*commandStatus)[cmdIndex+1:]...)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	log.Info().
		Str("nsId", nsId).
		Str("mciId", mciId).
		Str("vmId", vmId).
		Int("index", index).
		Msg("Command status deleted")

	return nil
}

// DeleteCommandStatusInfoByCriteria deletes multiple command status records by criteria
func DeleteCommandStatusInfoByCriteria(nsId, mciId, vmId string, filter *model.CommandStatusFilter) (int, error) {
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}
	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}
	err = common.CheckString(vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}

	var deleteCount int

	err = updateVmCommandStatusSafe(nsId, mciId, vmId, func(commandStatus *[]model.CommandStatusInfo) error {
		// Find matching commands to delete using helper function
		commandsToDelete := filterCommands(*commandStatus, filter)
		deleteCount = len(commandsToDelete)

		if deleteCount == 0 {
			return nil // No commands to delete
		}

		// Create a new slice without the matching commands
		var remainingCommands []model.CommandStatusInfo
		for _, cmd := range *commandStatus {
			shouldDelete := false
			for _, delCmd := range commandsToDelete {
				if cmd.Index == delCmd.Index {
					shouldDelete = true
					break
				}
			}
			if !shouldDelete {
				remainingCommands = append(remainingCommands, cmd)
			}
		}

		*commandStatus = remainingCommands
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}

	log.Info().
		Str("nsId", nsId).
		Str("mciId", mciId).
		Str("vmId", vmId).
		Int("deleteCount", deleteCount).
		Msg("Command statuses deleted by criteria")

	return deleteCount, nil
}

// ClearAllCommandStatusInfo deletes all command status records for a VM
func ClearAllCommandStatusInfo(nsId, mciId, vmId string) (int, error) {
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}
	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}
	err = common.CheckString(vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}

	var clearCount int

	err = updateVmCommandStatusSafe(nsId, mciId, vmId, func(commandStatus *[]model.CommandStatusInfo) error {
		// Count and clear all command statuses
		clearCount = len(*commandStatus)
		*commandStatus = []model.CommandStatusInfo{}
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}

	log.Info().
		Str("nsId", nsId).
		Str("mciId", mciId).
		Str("vmId", vmId).
		Int("clearCount", clearCount).
		Msg("All command statuses cleared")

	return clearCount, nil
}

// GetHandlingCommandCount returns the count of currently handling commands for a VM
// This function is optimized for frequent polling and avoids unnecessary processing
func GetHandlingCommandCount(nsId, mciId, vmId string) (int, error) {
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}
	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}
	err = common.CheckString(vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}

	// Use existing GetVmObject function - optimized for performance
	vmInfo, err := GetVmObject(nsId, mciId, vmId)
	if err != nil {
		// Don't log errors for frequent polling calls to reduce noise
		return 0, err
	}

	// Count handling commands efficiently
	handlingCount := 0
	for _, cmdStatus := range vmInfo.CommandStatus {
		if cmdStatus.Status == model.CommandStatusHandling {
			handlingCount++
		}
	}

	return handlingCount, nil
}

// GetMciHandlingCommandCount returns the count of currently handling commands across all VMs in an MCI
// This function is optimized for MCI-level monitoring
func GetMciHandlingCommandCount(nsId, mciId string) (map[string]int, int, error) {
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, 0, err
	}
	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, 0, err
	}

	// Get VM list
	vmList, err := ListVmId(nsId, mciId)
	if err != nil {
		return nil, 0, err
	}

	vmHandlingCounts := make(map[string]int)
	totalHandlingCount := 0

	// Process each VM's handling commands
	for _, vmId := range vmList {
		handlingCount, err := GetHandlingCommandCount(nsId, mciId, vmId)
		if err != nil {
			// Continue processing other VMs even if one fails
			log.Debug().Err(err).Msgf("Failed to get handling count for VM %s", vmId)
			vmHandlingCounts[vmId] = 0
			continue
		}

		vmHandlingCounts[vmId] = handlingCount
		totalHandlingCount += handlingCount
	}

	return vmHandlingCounts, totalHandlingCount, nil
}
