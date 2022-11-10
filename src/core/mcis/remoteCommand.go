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

	//"github.com/sirupsen/logrus"

	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bramvdbogaerde/go-scp"
	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
	validator "github.com/go-playground/validator/v10"
	"golang.org/x/crypto/ssh"
)

// sshDefaultUserName is array for temporal constants
var sshDefaultUserName = []string{"cb-user", "ubuntu", "root", "ec2-user"}

// McisCmdReq is struct for remote command
type McisCmdReq struct {
	UserName string `json:"userName" example:"cb-user" default:""`
	Command  string `json:"command" validate:"required" example:"sudo apt-get install ..."`
}

// TbMcisCmdReqStructLevelValidation is func to validate fields in McisCmdReq
func TbMcisCmdReqStructLevelValidation(sl validator.StructLevel) {

	// u := sl.Current().Interface().(McisCmdReq)

	// err := common.CheckString(u.Command)
	// if err != nil {
	// 	// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
	// 	sl.ReportError(u.Command, "command", "Command", err.Error(), "")
	// }
}

// SshCmdResult is struct for SshCmd Result
type SshCmdResult struct { // Tumblebug
	McisId string `json:"mcisId"`
	VmId   string `json:"vmId"`
	VmIp   string `json:"vmIp"`
	Result string `json:"result"`
	Err    error  `json:"err"`
}

// RemoteCommandToMcisVm is func to command to a VM in MCIS by SSH
func RemoteCommandToMcisVm(nsId string, mcisId string, vmId string, req *McisCmdReq) (string, error) {

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

	// returns InvalidValidationError for bad validation input, nil or ValidationErrors ( []FieldError )
	err = validate.Struct(req)
	if err != nil {

		// this check is only needed when your code could produce
		// an invalid value for validation such as interface with nil
		// value most including myself do not usually have code like this.
		if _, ok := err.(*validator.InvalidValidationError); ok {
			fmt.Println(err)
			return "", err
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

		return "", err
	}

	check, _ := CheckVm(nsId, mcisId, vmId)

	if !check {
		err := fmt.Errorf("The vm " + vmId + " does not exist.")
		return err.Error(), err
	}

	vmIp, sshPort := GetVmIp(nsId, mcisId, vmId)

	//sshKey := req.SshKey
	cmd := req.Command

	// find vaild username
	userName, sshKey, err := VerifySshUserName(nsId, mcisId, vmId, vmIp, sshPort, req.UserName)
	// Even though VerifySshUserName is not complete, Try RunRemoteCommand
	// With RunRemoteCommand, error will be checked again
	if err == nil {
		// Just logging the error (but it is net a faultal)
		common.CBLog.Info(err)
	}

	fmt.Println("")
	fmt.Println("[SSH] " + mcisId + "." + vmId + "(" + vmIp + ")" + " with userName: " + userName)
	fmt.Println("[CMD] " + cmd)
	fmt.Println("")

	result, err := RunRemoteCommand(vmIp, sshPort, userName, sshKey, cmd)
	if err != nil {
		return ("[ERROR: " + err.Error() + "]\n " + *result), nil
	}
	return *result, nil

}

// RemoteCommandToMcis is func to command to all VMs in MCIS by SSH
func RemoteCommandToMcis(nsId string, mcisId string, subGroupId string, req *McisCmdReq) ([]SshCmdResult, error) {

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

	// returns InvalidValidationError for bad validation input, nil or ValidationErrors ( []FieldError )
	err = validate.Struct(req)
	if err != nil {

		// this check is only needed when your code could produce
		// an invalid value for validation such as interface with nil
		// value most including myself do not usually have code like this.
		if _, ok := err.(*validator.InvalidValidationError); ok {
			fmt.Println(err)
			temp := []SshCmdResult{}
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

		temp := []SshCmdResult{}
		return temp, err
	}

	check, _ := CheckMcis(nsId, mcisId)

	if !check {
		temp := []SshCmdResult{}
		err := fmt.Errorf("The mcis " + mcisId + " does not exist.")
		return temp, err
	}

	/*
		type contentSub struct {
			McisId string `json:"mcisId"`
			VmId   string `json:"vmId"`
			VmIp   string `json:"vmIp"`
			Result  string `json:"result"`
		}
		var content struct {
			ResultArray []contentSub `json:"resultArray"`
		}
	*/
	//content := RestPostCmdMcisResponseWrapper{}

	vmList, err := ListVmId(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}
	if subGroupId != "" {
		vmListInGroup, err := ListMcisGroupVms(nsId, mcisId, subGroupId)
		if err != nil {
			common.CBLog.Error(err)
			return nil, err
		}
		if vmListInGroup == nil {
			err := fmt.Errorf("No VM in " + subGroupId)
			return nil, err
		}
		vmList = vmListInGroup
	}

	//goroutine sync wg
	var wg sync.WaitGroup

	var resultArray []SshCmdResult

	for _, vmId := range vmList {
		wg.Add(1)
		go RunRemoteCommandAsync(&wg, nsId, mcisId, vmId, req.UserName, req.Command, &resultArray)
	}
	wg.Wait() //goroutine sync wg

	return resultArray, nil
}

// RunRemoteCommand is func to execute a SSH command to a VM (sync call)
func RunRemoteCommand(vmIP string, sshPort string, userName string, privateKey string, cmd string) (*string, error) {

	// Set VM SSH config (serverEndpoint, userName, Private Key)
	serverEndpoint := fmt.Sprintf("%s:%s", vmIP, sshPort)
	sshInfo := sshInfo{
		ServerPort: serverEndpoint,
		UserName:   userName,
		PrivateKey: []byte(privateKey),
	}

	// Execute SSH
	result, err := runSSH(sshInfo, cmd)
	if err != nil {
		return &result, err
	}
	return &result, nil

}

// RunRemoteCommandAsync is func to execute a SSH command to a VM (async call)
func RunRemoteCommandAsync(wg *sync.WaitGroup, nsId string, mcisId string, vmId string, givenUserName string, cmd string, returnResult *[]SshCmdResult) {

	defer wg.Done() //goroutin sync done

	vmIp, sshPort := GetVmIp(nsId, mcisId, vmId)
	userName, sshKey, err := VerifySshUserName(nsId, mcisId, vmId, vmIp, sshPort, givenUserName)
	// Eventhough VerifySshUserName is not complete, Try RunRemoteCommand
	// With RunRemoteCommand, error will be checked again

	fmt.Println("")
	fmt.Println("[SSH] " + mcisId + "." + vmId + "(" + vmIp + ")" + " with userName: " + userName)
	fmt.Println("[CMD] " + cmd)
	fmt.Println("")

	// RunRemoteCommand
	result, err := RunRemoteCommand(vmIp, sshPort, userName, sshKey, cmd)

	sshResultTmp := SshCmdResult{}
	sshResultTmp.McisId = ""
	sshResultTmp.VmId = vmId
	sshResultTmp.VmIp = vmIp

	if err != nil {
		sshResultTmp.Result = ("[ERROR: " + err.Error() + "]\n " + *result)
		sshResultTmp.Err = err
		*returnResult = append(*returnResult, sshResultTmp)
	} else {
		fmt.Println("[Begin] SSH Output")
		fmt.Println(*result)
		fmt.Println("[end] SSH Output")

		sshResultTmp.Result = *result
		sshResultTmp.Err = nil
		*returnResult = append(*returnResult, sshResultTmp)
	}

}

// VerifySshUserName is func to verify SSH username
func VerifySshUserName(nsId string, mcisId string, vmId string, vmIp string, sshPort string, givenUserName string) (string, string, error) {

	fmt.Println("")
	fmt.Println("[Start SSH checking squence]")

	// verify if vm is running with a public ip.
	/*
		if vmIp == "" {
			return "", "", fmt.Errorf("Cannot ssh, VM IP is null")
		}
		vmStatusInfoTmp, err := GetVmStatus(nsId, mcisId, vmId)
		if err != nil {
			common.CBLog.Error(err)
			return "", "", err
		}
		if vmStatusInfoTmp.Status != StatusRunning || vmIp == "" {
			return "", "", fmt.Errorf("Cannot ssh, VM is not Running")
		}
	*/

	/* Code to check endpoint and port connectivity. (disabled for better speed)
	// CheckConnectivity func checks if given port is open and ready.
	// retry: 5 times, sleep: 5 seconds. timeout for each Dial: 20 seconds
	conErr := CheckConnectivity(vmIp, sshPort)
	if conErr != nil {
		return "", "", conErr
	}
	*/

	// find vaild username
	userName, verifiedUserName, privateKey := GetVmSshKey(nsId, mcisId, vmId)
	userNames := []string{
		sshDefaultUserName[0],
		userName,
		givenUserName,
		sshDefaultUserName[1],
		sshDefaultUserName[2],
		sshDefaultUserName[3],
	}

	theUserName := ""
	cmd := "sudo ls"

	if verifiedUserName != "" {
		/* Code for strict check in advance with real SSH (but slow down speed)
		fmt.Printf("\n[Check SSH] (%s) with userName: %s\n", vmIp, verifiedUserName)
		_, err := RunRemoteCommand(vmIp, sshPort, verifiedUserName, privateKey, cmd)
		if err != nil {
			return "", "", fmt.Errorf("Cannot do ssh, with %s, %s", verifiedUserName, err.Error())
		}*/
		theUserName = verifiedUserName
		fmt.Printf("[%s] is a valid UserName\n", theUserName)
		return theUserName, privateKey, nil
	}

	// If we have a varified username, Retrieve ssh username from the given list will not be executed
	fmt.Println("[Retrieve ssh username from the given list]")
	for _, v := range userNames {
		if v != "" {
			fmt.Printf("[Check SSH] (%s) with userName: %s\n", vmIp, v)
			_, err := RunRemoteCommand(vmIp, sshPort, v, privateKey, cmd)
			if err != nil {
				fmt.Printf("Cannot do ssh, with %s, %s", verifiedUserName, err.Error())
			} else {
				theUserName = v
				fmt.Printf("[%s] is a valid UserName\n", theUserName)
				break
			}
			time.Sleep(3 * time.Second)
		}
	}
	if theUserName != "" {
		err := UpdateVmSshKey(nsId, mcisId, vmId, theUserName)
		if err != nil {
			common.CBLog.Error(err)
			return "", "", err
		}
	} else {
		return "", "", fmt.Errorf("Could not find a valid username")
	}

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

		fmt.Println("[Check SSH Port]", host, ":", port)

		if err != nil {
			fmt.Println("SSH Port is NOT accessible yet. retry after 5 seconds sleep ", err)
		} else {
			// port is opened. return nil for error.
			fmt.Println("SSH Port is accessible")
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("SSH Port is NOT not accessible (5 trials)")
}

// GetVmSshKey is func to get VM SShKey
func GetVmSshKey(nsId string, mcisId string, vmId string) (string, string, string) {

	var content struct {
		SshKeyId string `json:"sshKeyId"`
	}

	fmt.Println("[GetVmSshKey]" + vmId)
	key := common.GenMcisKey(nsId, mcisId, vmId)

	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
		err = fmt.Errorf("In GetVmSshKey(); CBStore.Get() returned an error.")
		common.CBLog.Error(err)
		// return nil, err
	}

	json.Unmarshal([]byte(keyValue.Value), &content)

	fmt.Printf("%+v\n", content.SshKeyId)

	sshKey := common.GenResourceKey(nsId, common.StrSSHKey, content.SshKeyId)
	keyValue, _ = common.CBStore.Get(sshKey)
	var keyContent struct {
		Username         string `json:"username"`
		VerifiedUsername string `json:"verifiedUsername"`
		PrivateKey       string `json:"privateKey"`
	}
	json.Unmarshal([]byte(keyValue.Value), &keyContent)

	return keyContent.Username, keyContent.VerifiedUsername, keyContent.PrivateKey
}

// UpdateVmSshKey is func to update VM SShKey
func UpdateVmSshKey(nsId string, mcisId string, vmId string, verifiedUserName string) error {

	var content struct {
		SshKeyId string `json:"sshKeyId"`
	}
	fmt.Println("[GetVmSshKey]" + vmId)
	key := common.GenMcisKey(nsId, mcisId, vmId)
	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
		err = fmt.Errorf("In UpdateVmSshKey(); CBStore.Get() returned an error.")
		common.CBLog.Error(err)
		// return nil, err
	}

	json.Unmarshal([]byte(keyValue.Value), &content)

	sshKey := common.GenResourceKey(nsId, common.StrSSHKey, content.SshKeyId)
	keyValue, _ = common.CBStore.Get(sshKey)

	tmpSshKeyInfo := mcir.TbSshKeyInfo{}
	json.Unmarshal([]byte(keyValue.Value), &tmpSshKeyInfo)

	tmpSshKeyInfo.VerifiedUsername = verifiedUserName

	val, _ := json.Marshal(tmpSshKeyInfo)
	err = common.CBStore.Put(keyValue.Key, string(val))
	if err != nil {
		common.CBLog.Error(err)
		return err
	}
	return nil
}

// Internal functions for SSH

func init() {
	//cblog = config.Cblogger
}

type sshInfo struct {
	UserName   string // ex) "root"
	PrivateKey []byte // ex)  []byte(`-----BEGIN RSA PRIVATE KEY-----
	//      MIIEoQIBAAKCAQEArVNOLwMIp5VmZ4VPZotcoCHdE...`)
	ServerPort string // ex) "node12:22"
}

func clientConnect(sshInfo sshInfo) (scp.Client, error) {
	common.CBLog.Info("SSH call clientConnect()")

	clientConfig, _ := getClientConfig(sshInfo.UserName, sshInfo.PrivateKey, ssh.InsecureIgnoreHostKey())
	client := scp.NewClient(sshInfo.ServerPort, &clientConfig)
	err := client.Connect()
	return client, err
}

func getClientConfig(username string, privateKey []byte, keyCallBack ssh.HostKeyCallback) (ssh.ClientConfig, error) {

	signer, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return ssh.ClientConfig{}, err
	}

	clientConfig := ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: keyCallBack,
	}
	return clientConfig, nil
}

func clientClose(client scp.Client) {
	common.CBLog.Info("SSH call clientClose()")
	client.Close()
}

func runCommand(client scp.Client, cmd string) (string, error) {
	common.CBLog.Info("call runCommand()")

	session := client.Session
	sshOut, err := session.StdoutPipe()
	session.Stderr = os.Stderr

	err = session.Run(cmd)
	//err = session.Start(cmd)

	return stdoutToString(sshOut), err
}

func stdoutToString(sshOut io.Reader) string {
	buf := make([]byte, 1000)
	num, err := sshOut.Read(buf)
	outStr := ""
	if err == nil {
		outStr = string(buf[:num])
	}
	for err == nil {
		num, err = sshOut.Read(buf)
		outStr += string(buf[:num])
		if err != nil {
			if err.Error() != "EOF" {
				common.CBLog.Error(err)
			}
		}

	}
	return strings.Trim(outStr, "\n")
}

// runSSH func execute a command by SSH
func runSSH(sshInfo sshInfo, cmd string) (string, error) {
	common.CBLog.Info("call runSSH()")

	sshCli, err := clientConnect(sshInfo)
	if err != nil {
		return "", err
	}
	defer clientClose(sshCli)

	return runCommand(sshCli, cmd)
}
