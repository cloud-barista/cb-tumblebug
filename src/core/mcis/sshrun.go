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

package mcis

import (

	//"github.com/sirupsen/logrus"

	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bramvdbogaerde/go-scp"
	"github.com/bramvdbogaerde/go-scp/auth"
	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"golang.org/x/crypto/ssh"
)

//var cblog *logrus.Logger

func init() {
	//cblog = config.Cblogger
}

//====================================================================
type SSHInfo struct {
	UserName   string // ex) "root"
	PrivateKey []byte // ex)   []byte(`-----BEGIN RSA PRIVATE KEY-----
	//              MIIEoQIBAAKCAQEArVNOLwMIp5VmZ4VPZotcoCHdEzimKalAsz+ccLfvAA1Y2ELH
	//              ...`)
	ServerPort string // ex) "node12:22"
}

//====================================================================

func Connect(sshInfo SSHInfo) (scp.Client, error) {
	common.CBLog.Info("call Connect()")

	clientConfig, _ := getClientConfig(sshInfo.UserName, sshInfo.PrivateKey, ssh.InsecureIgnoreHostKey())
	client := scp.NewClient(sshInfo.ServerPort, &clientConfig)
	err := client.Connect()
	return client, err
}

//====================================================================
type SSHKeyPathInfo struct {
	UserName   string // ex) "root"
	KeyPath    string // ex) "/root/.ssh/id_rsa // You should use the full path.
	ServerPort string // ex) "node12:22"
}

//====================================================================

func ConnectKeyPath(sshKeyPathInfo SSHKeyPathInfo) (scp.Client, error) {
	common.CBLog.Info("call ConnectKeyPath()")

	clientConfig, _ := auth.PrivateKey(sshKeyPathInfo.UserName, sshKeyPathInfo.KeyPath, ssh.InsecureIgnoreHostKey())
	client := scp.NewClient(sshKeyPathInfo.ServerPort, &clientConfig)
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

func Close(client scp.Client) {
	common.CBLog.Info("call Close()")

	client.Close()
}

func RunCommand(client scp.Client, cmd string) (string, error) {
	common.CBLog.Info("call RunCommand()")

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

func Copy(client scp.Client, sourcePath string, remotePath string) error {
	common.CBLog.Info("call Copy()")

	// Open a file
	file, _ := os.Open(sourcePath)
	defer file.Close()
	return client.CopyFile(file, remotePath, "0755")
}

//=============== for One Call Service
func SSHRun(sshInfo SSHInfo, cmd string) (string, error) {
	common.CBLog.Info("call SSHRun()")

	sshCli, err := Connect(sshInfo)
	if err != nil {
		return "", err
	}
	defer Close(sshCli)

	return RunCommand(sshCli, cmd)
}

func SSHRunByKeyPath(sshInfo SSHKeyPathInfo, cmd string) (string, error) {
	common.CBLog.Info("call SSHRunKeyPath()")

	sshCli, err := ConnectKeyPath(sshInfo)
	if err != nil {
		return "", err
	}
	defer Close(sshCli)

	return RunCommand(sshCli, cmd)
}

func SSHCopy(sshInfo SSHInfo, sourcePath string, remotePath string) error {
	common.CBLog.Info("call SSHCopy()")

	sshCli, err := Connect(sshInfo)
	if err != nil {
		return err
	}
	defer Close(sshCli)

	return Copy(sshCli, sourcePath, remotePath)
}

func SSHCopyByKeyPath(sshInfo SSHKeyPathInfo, sourcePath string, remotePath string) error {
	common.CBLog.Info("call SSHCopyByKeyPath()")

	sshCli, err := ConnectKeyPath(sshInfo)
	if err != nil {
		return err
	}
	defer Close(sshCli)

	return Copy(sshCli, sourcePath, remotePath)
}

// CheckConnectivity func checks if given port is open and ready.
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

// VerifySshUserName is func to verify SSH username
func VerifySshUserName(nsId string, mcisId string, vmId string, vmIp string, sshPort string, givenUserName string) (string, string, error) {

	fmt.Println("")
	fmt.Println("[Start SSH checking squence]")

	// verify if vm is running with a public ip.
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

	// CheckConnectivity func checks if given port is open and ready.
	// retry: 5 times, sleep: 5 seconds. timeout for each Dial: 20 seconds
	conErr := CheckConnectivity(vmIp, sshPort)
	if conErr != nil {
		return "", "", conErr
	}

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
		_, err := RunSSH(vmIp, sshPort, verifiedUserName, privateKey, cmd)
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
			_, err := RunSSH(vmIp, sshPort, v, privateKey, cmd)
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

// RunSSH is func to execute a SSH command to a VM (sync call)
func RunSSH(vmIP string, sshPort string, userName string, privateKey string, cmd string) (*string, error) {

	// Set VM SSH config (serverEndpoint, userName, Private Key)
	serverEndpoint := fmt.Sprintf("%s:%s", vmIP, sshPort)
	sshInfo := SSHInfo{
		ServerPort: serverEndpoint,
		UserName:   userName,
		PrivateKey: []byte(privateKey),
	}

	// Execute SSH
	if result, err := SSHRun(sshInfo, cmd); err != nil {
		return nil, err
	} else {
		return &result, nil
	}
}

// RunSSHAsync is func to execute a SSH command to a VM (async call)
func RunSSHAsync(wg *sync.WaitGroup, vmID string, vmIP string, sshPort string, userName string, privateKey string, cmd string, returnResult *[]SshCmdResult) {

	defer wg.Done() //goroutin sync done

	// RunSSH
	result, err := RunSSH(vmIP, sshPort, userName, privateKey, cmd)

	sshResultTmp := SshCmdResult{}
	sshResultTmp.McisId = ""
	sshResultTmp.VmId = vmID
	sshResultTmp.VmIp = vmIP

	if err != nil {
		sshResultTmp.Result = err.Error()
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
