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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
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
func RemoteCommandToMci(nsId string, mciId string, subGroupId string, vmId string, req *model.MciCmdReq) ([]model.SshCmdResult, error) {

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

		// for _, err := range err.(validator.ValidationErrors) {

		// 	fmt.Println(err.Namespace()) // can differ when a custom TagNameFunc is registered or
		// 	fmt.Println(err.Field())     // by passing alt name to ReportError like below
		// 	fmt.Println(err.StructNamespace())
		// 	fmt.Println(err.StructField())
		// 	fmt.Println(err.Tag())
		// 	fmt.Println(err.ActualTag())
		// 	fmt.Println(err.Kind())
		// 	fmt.Println(err.Type())
		// 	fmt.Println(err.Value())
		// 	fmt.Println(err.Param())
		// 	fmt.Println()
		// }

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
	if subGroupId != "" {
		vmListInGroup, err := ListVmBySubGroup(nsId, mciId, subGroupId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return nil, err
		}
		if vmListInGroup == nil {
			err := fmt.Errorf("No VM in " + subGroupId)
			return nil, err
		}
		vmList = vmListInGroup
	}

	if vmId != "" {
		vmList = []string{vmId}
	}

	// goroutine sync wg
	var wg sync.WaitGroup

	var resultArray []model.SshCmdResult

	// Preprocess commands for each VM
	vmCommands := make(map[string][]string)
	for i, vmId := range vmList {
		processedCommands := make([]string, len(req.Command))
		for j, cmd := range req.Command {
			processedCmd, err := processCommand(cmd, nsId, mciId, vmId, i)
			if err != nil {
				return nil, err
			}
			processedCommands[j] = processedCmd
		}
		vmCommands[vmId] = processedCommands
	}

	// Execute commands in parallel using goroutines
	for vmId, commands := range vmCommands {
		wg.Add(1)
		go RunRemoteCommandAsync(&wg, nsId, mciId, vmId, req.UserName, commands, &resultArray)
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
	bastionNode := bastionNodes[0]
	// use public IP of the bastion VM
	bastionIp, _, bastionSshPort, err := GetVmIp(nsId, bastionNode.MciId, bastionNode.VmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return map[int]string{}, map[int]string{}, err
	}
	bastionUserName, bastionSshKey, err := VerifySshUserName(nsId, bastionNode.MciId, bastionNode.VmId, bastionIp, bastionSshPort, givenUserName)
	bastionEndpoint := fmt.Sprintf("%s:%s", bastionIp, bastionSshPort)

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

	// Execute SSH
	stdoutResults, stderrResults, err := runSSH(bastionSshInfo, targetSshInfo, cmds)
	if err != nil {
		fmt.Printf("Error executing commands: %s\n", err)
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
	}

	// RunRemoteCommand
	stdoutResults, stderrResults, err := RunRemoteCommand(nsId, mciId, vmId, givenUserName, cmd)

	if err != nil {
		sshResultTmp.Stdout = stdoutResults
		sshResultTmp.Stderr = stderrResults
		sshResultTmp.Err = err
		*returnResult = append(*returnResult, sshResultTmp)
	} else {
		log.Debug().Msg("[Begin] SSH Output")
		fmt.Println(stdoutResults)
		log.Debug().Msg("[End] SSH Output")

		sshResultTmp.Stdout = stdoutResults
		sshResultTmp.Stderr = stderrResults
		sshResultTmp.Err = nil
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

	// retry: 5 times, sleep: 5 seconds. timeout for each Dial: 20 seconds
	retrycheck := 5
	timeout := time.Second * time.Duration(20)
	for i := 0; i < retrycheck; i++ {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), timeout)
		conn.Close()

		log.Debug().Msgf("[Check SSH Port] %v:%v", host, port)

		if err != nil {
			log.Err(err).Msg("SSH Port is NOT accessible yet. retry after 5 seconds sleep")
		} else {
			log.Debug().Msg("SSH Port is accessible")
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("SSH Port is NOT not accessible (5 trials)")
}

// GetVmSshKey is func to get VM SShKey. Returns username, verifiedUsername, privateKey
func GetVmSshKey(nsId string, mciId string, vmId string) (string, string, string, error) {

	var content struct {
		SshKeyId string `json:"sshKeyId"`
	}

	key := common.GenMciKey(nsId, mciId, vmId)

	keyValue, err := kvstore.GetKv(key)
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
	keyValue, err = kvstore.GetKv(sshKey)
	if err != nil || keyValue == (kvstore.KeyValue{}) {
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

	return keyContent.Username, keyContent.VerifiedUsername, keyContent.PrivateKey, nil
}

// UpdateVmSshKey is func to update VM SShKey
func UpdateVmSshKey(nsId string, mciId string, vmId string, verifiedUserName string) error {

	var content struct {
		SshKeyId string `json:"sshKeyId"`
	}

	key := common.GenMciKey(nsId, mciId, vmId)
	keyValue, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("In UpdateVmSshKey(); kvstore.GetKv() returned an error.")
		log.Error().Err(err).Msg("")
		// return nil, err
	}

	json.Unmarshal([]byte(keyValue.Value), &content)

	sshKey := common.GenResourceKey(nsId, model.StrSSHKey, content.SshKeyId)
	keyValue, _ = kvstore.GetKv(sshKey)

	tmpSshKeyInfo := model.TbSshKeyInfo{}
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

// runSSH func execute a command by SSH
func runSSH(bastionInfo model.SshInfo, targetInfo model.SshInfo, cmds []string) (map[int]string, map[int]string, error) {

	stdoutMap := make(map[int]string)
	stderrMap := make(map[int]string)

	// Parse the private key for the bastion host
	bastionSigner, err := ssh.ParsePrivateKey(bastionInfo.PrivateKey)
	if err != nil {
		return stdoutMap, stderrMap, err
	}

	// Create an SSH client configuration for the bastion host
	bastionConfig := &ssh.ClientConfig{
		User: bastionInfo.UserName,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(bastionSigner),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Parse the private key for the target host
	targetSigner, err := ssh.ParsePrivateKey(targetInfo.PrivateKey)
	if err != nil {
		return stdoutMap, stderrMap, err
	}

	// Create an SSH client configuration for the target host
	targetConfig := &ssh.ClientConfig{
		User: targetInfo.UserName,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(targetSigner),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Setup the bastion host connection
	bastionClient, err := ssh.Dial("tcp", bastionInfo.EndPoint, bastionConfig)
	if err != nil {
		return stdoutMap, stderrMap, err
	}
	defer bastionClient.Close()

	// Setup the actual SSH client through the bastion host
	conn, err := bastionClient.Dial("tcp", targetInfo.EndPoint)
	if err != nil {
		return stdoutMap, stderrMap, err
	}

	ncc, chans, reqs, err := ssh.NewClientConn(conn, targetInfo.EndPoint, targetConfig)
	if err != nil {
		return stdoutMap, stderrMap, err
	}
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

			_, targetVmIP, targetSshPort, _ := GetVmIp(nsId, mciId, vmId)
			targetUserName, targetPrivateKey, _ := VerifySshUserName(nsId, mciId, vmId, targetVmIP, targetSshPort, "")
			// error will be handled in the next step

			targetSshInfo := model.SshInfo{
				EndPoint:   fmt.Sprintf("%s:%s", targetVmIP, targetSshPort),
				UserName:   targetUserName,
				PrivateKey: []byte(targetPrivateKey),
			}

			// Transfer file to the VM via bastion
			err := transferFileToVmViaBastion(nsId, mciId, vmId, targetSshInfo, fileData, fileName, targetPath)

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

	err = runSCPWithBastion(bastionSshInfo, targetSshInfo, fileData, fileName, targetPath)
	if err != nil {
		return fmt.Errorf("failed to transfer file to VM via bastion: %v", err)
	}

	log.Info().Msgf("File successfully transferred to VM %s via bastion", vmId)
	return nil
}

// runSCPWithBastion is func to send a file using SCP over SSH via a Bastion host
func runSCPWithBastion(bastionInfo model.SshInfo, targetInfo model.SshInfo, fileData []byte, fileName string, targetPath string) error {
	log.Info().Msg("Setting up SCP connection via Bastion Host")

	// Parse the private key for the bastion host
	bastionSigner, err := ssh.ParsePrivateKey(bastionInfo.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to parse bastion private key: %v", err)
	}

	// Create an SSH client configuration for the bastion host
	bastionConfig := &ssh.ClientConfig{
		User: bastionInfo.UserName,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(bastionSigner),
		},
		HostKeyCallback: ssh.FixedHostKey(bastionPublicKey),
	}

	// Read the allowed host key for the bastion host
	bastionPublicKeyBytes, err := ioutil.ReadFile("bastion_hostkey.pub")
	if err != nil {
		return fmt.Errorf("failed to read bastion host key: %v", err)
	}
	bastionPublicKey, err := ssh.ParsePublicKey(bastionPublicKeyBytes)
	if err != nil {
		return fmt.Errorf("failed to parse bastion host key: %v", err)
	}

	// Parse the private key for the target host
	targetSigner, err := ssh.ParsePrivateKey(targetInfo.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to parse target private key: %v", err)
	}

	// Create an SSH client configuration for the target host
	targetConfig := &ssh.ClientConfig{
		User: targetInfo.UserName,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(targetSigner),
		},
		HostKeyCallback: ssh.FixedHostKey(targetPublicKey),
	}

	// Read the allowed host key for the target host
	targetPublicKeyBytes, err := ioutil.ReadFile("target_hostkey.pub")
	if err != nil {
		return fmt.Errorf("failed to read target host key: %v", err)
	}
	targetPublicKey, err := ssh.ParsePublicKey(targetPublicKeyBytes)
	if err != nil {
		return fmt.Errorf("failed to parse target host key: %v", err)
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

	tempVNetInfo, ok := res.(model.TbVNetInfo)
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
				}
				for _, v := range vmIdsInSubnet {
					tmpPublicIp, _, _, err := GetVmIp(nsId, mciId, v)
					if err != nil {
						log.Error().Err(err).Msg("")
					}
					if tmpPublicIp != "" {
						bastionVmId = v
						break
					}
				}
			} else {
				for _, existingId := range subnetInfo.BastionNodes {
					if existingId.VmId == bastionVmId {
						return fmt.Sprintf("Bastion (ID: %s) already exists in subnet (ID: %s) in VNet (ID: %s).",
							bastionVmId, subnetInfo.Id, vmObj.VNetId), nil
					}
				}
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
		vNets := resourceListInNs.([]model.TbVNetInfo) // type assertion
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
	tempVNetInfo, ok := res.(model.TbVNetInfo)
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
		return "", nil, errors.New("Built-in function error in command: no function found in command")
	}

	funcName := matches[1]
	paramsPart := matches[2]
	params := make(map[string]string)

	paramPairs := splitParams(paramsPart)

	for _, pair := range paramPairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			key := strings.TrimSpace(kv[0])
			value := strings.TrimSpace(kv[1])
			if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
				value = strings.Trim(value, "'")
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

// extractFunctionAndParams is a helper function to find matching parenthesis
func findMatchingParenthesis(command string, start int) int {
	count := 1
	for i := start; i < len(command); i++ {
		switch command[i] {
		case '(':
			count++
		case ')':
			count--
			if count == 0 {
				return i
			}
		}
	}
	return -1
}

// processCommand is function to replace the keywords with actual values
func processCommand(command, nsId, mciId, vmId string, vmIndex int) (string, error) {
	start := 0
	for {
		start = strings.Index(command[start:], "$$Func(")
		if start == -1 {
			break
		}
		start += 7 // Move past "$$Func("
		end := findMatchingParenthesis(command, start)
		if end == -1 {
			return "", errors.New("Built-in function error in command: no matching parenthesis found")
		}

		funcCall := command[start:end]

		funcName, params, err := extractFunctionAndParams(funcCall)
		if err != nil {
			return "", err
		}

		var replacement string
		if strings.EqualFold(funcName, "GetPublicIP") {
			targetMciId := mciId
			targetVmId := vmId
			if val, ok := params["target"]; ok {
				parts := strings.Split(val, ".")
				if len(parts) == 2 {
					targetMciId = parts[0]
					targetVmId = parts[1]
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
			replacement, err = getPublicIP(nsId, targetMciId, targetVmId, prefix, postfix)

			if err != nil {
				return "", fmt.Errorf("Built-in function getPublicIP error: %s", err.Error())
			}

		} else if strings.EqualFold(funcName, "GetPublicIPs") {
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
			replacement, err = getPublicIPs(nsId, targetMciId, separator, prefix, postfix)

			if err != nil {
				return "", fmt.Errorf("Built-in function getPublicIPs error: %s", err.Error())
			}

		} else if strings.EqualFold(funcName, "AssignTask") {
			taskListParam, ok := params["task"]
			if !ok {
				return "", fmt.Errorf("Built-in function AssignTask error: no task list provided")
			}
			tasks := splitParams(taskListParam)
			replacement = tasks[vmIndex%len(tasks)]
		} else {
			return "", fmt.Errorf("Built-in function error in command: Unknown function: %s", funcName)
		}

		// Replace the entire $$Func(...) expression with the result
		command = command[:start-7] + replacement + command[end+1:]
		start = start - 7 + len(replacement) // Adjust start for the next iteration
	}

	return command, nil
}

// Built-in functions for remote command
// getPublicIP function to get and replace string with the public IP of the target
func getPublicIP(nsId, mciId, vmId, prefix, postfix string) (string, error) {
	vmStatus, err := GetVmCurrentPublicIp(nsId, mciId, vmId)
	if err != nil {
		return "", err
	}
	ip := vmStatus.PublicIp
	return prefix + ip + postfix, err
}

// getPublicIP function to get and replace string with the public IP list of the target
func getPublicIPs(nsId, mciId, separator, prefix, postfix string) (string, error) {
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
