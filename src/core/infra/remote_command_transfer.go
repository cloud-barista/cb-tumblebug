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

package infra

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/ssh"
)

// TransferFileToInfra is a function to transfer a file to all VMs in Infra by SSH through bastion hosts
func TransferFileToInfra(nsId string, infraId string, nodeGroupId string, nodeId string, fileData []byte, fileName string, targetPath string) ([]model.SshCmdResult, error) {
	// Get the list of VMs in the Infra
	nodeList, err := ListNodeId(nsId, infraId)
	if err != nil {
		return nil, err
	}
	// If a nodeGroupId is provided, filter the VM list by nodeGroup
	if nodeGroupId != "" {
		nodeListInGroup, err := ListNodeByNodeGroup(nsId, infraId, nodeGroupId)
		if err != nil {
			return nil, err
		}
		nodeList = nodeListInGroup
	}
	// If a specific nodeId is provided, limit the transfer to that VM only
	if nodeId != "" {
		nodeList = []string{nodeId}
	}

	// Create a wait group to sync goroutines
	var wg sync.WaitGroup
	var resultArray []model.SshCmdResult
	var resultMutex sync.Mutex // To safely append to resultArray in concurrent goroutines

	// Iterate over the Node list to transfer the file
	for _, nodeId := range nodeList {
		wg.Add(1)
		go func(nodeId string) {
			defer wg.Done()
			log.Info().Msgf("Transferring file to VM: %s", nodeId)

			_, targetNodeIP, targetSshPort, err := GetNodeIp(nsId, infraId, nodeId)

			// Create the result for this Node
			result := model.SshCmdResult{
				InfraId: infraId,
				NodeId:  nodeId,
				NodeIp:  targetNodeIP,
				Command: map[int]string{0: fmt.Sprintf("scp %s to %s", fileName, targetPath)},
				Stdout:  map[int]string{},
				Stderr:  map[int]string{},
			}

			if err != nil {
				result.Err = err
				result.Stderr[0] = fmt.Sprintf("Failed to get Node IP: %v", err)
				resultMutex.Lock()
				resultArray = append(resultArray, result)
				resultMutex.Unlock()
				return
			}

			// Check Node status before executing file transfer
			nodeInfo, err := GetNodeObject(nsId, infraId, nodeId)
			if err != nil {
				result.Err = fmt.Errorf("failed to get Node status: %v", err)
				result.Stderr[0] = fmt.Sprintf("Failed to get Node status: %v", err)
				resultMutex.Lock()
				resultArray = append(resultArray, result)
				resultMutex.Unlock()
				return
			}

			// Validate Node status for file transfer
			if nodeInfo.Status != model.StatusRunning {
				var errorMsg string
				if nodeInfo.Status == model.StatusTerminated {
					errorMsg = fmt.Sprintf("Node '%s' is in '%s' status. File transfer is impossible for terminated Nodes", nodeId, nodeInfo.Status)
				} else {
					errorMsg = fmt.Sprintf("Node '%s' is in '%s' status (not Running). Please change the Node status to Running and try again", nodeId, nodeInfo.Status)
				}
				result.Err = fmt.Errorf("%s", errorMsg)
				result.Stderr[0] = errorMsg
				resultMutex.Lock()
				resultArray = append(resultArray, result)
				resultMutex.Unlock()
				return
			}

			targetUserName, targetPrivateKey, err := VerifySshUserName(nsId, infraId, nodeId, targetNodeIP, targetSshPort, "")
			if err != nil {
				result.Err = fmt.Errorf("failed to verify SSH username: %v", err)
				result.Stderr[0] = fmt.Sprintf("Failed to verify SSH username: %v", err)
				resultMutex.Lock()
				resultArray = append(resultArray, result)
				resultMutex.Unlock()
				return
			}

			targetSshInfo := model.SshInfo{
				EndPoint:   fmt.Sprintf("%s:%d", targetNodeIP, targetSshPort),
				UserName:   targetUserName,
				PrivateKey: []byte(targetPrivateKey),
			}

			// Transfer file to the Node via bastion
			err = transferFileToNodeViaBastion(nsId, infraId, nodeId, targetSshInfo, fileData, fileName, targetPath)

			if err != nil {
				result.Stderr[0] = fmt.Sprintf("Failed to transfer file: %v", err)
				result.Err = fmt.Errorf("file transfer failed: %v", err)
				log.Error().Err(err).Msgf("Failed to transfer file to VM: %s", nodeId)
			} else {
				result.Stdout[0] = fmt.Sprintf("File transfer successful: %s%s", targetPath, fileName)
				log.Info().Msgf("Successfully transferred file to VM: %s", nodeId)
			}

			// Safely append to resultArray
			resultMutex.Lock()
			resultArray = append(resultArray, result)
			resultMutex.Unlock()
		}(nodeId)
	}
	wg.Wait()

	return resultArray, nil
}

// TransferFileAndCmdToInfra transfers a file to all VMs in Infra and optionally runs a shell command
// on each Node where the file transfer succeeded.
func TransferFileAndCmdToInfra(nsId string, infraId string, nodeGroupId string, nodeId string, fileData []byte, fileName string, targetPath string, command string) (model.InfraFileTransferAndCmdResult, error) {
	result := model.InfraFileTransferAndCmdResult{}

	// Step 1: transfer file to all targeted VMs
	transferResults, err := TransferFileToInfra(nsId, infraId, nodeGroupId, nodeId, fileData, fileName, targetPath)
	if err != nil {
		return result, err
	}
	result.FileTransferResults = transferResults

	if command == "" {
		return result, nil
	}

	// Step 2: run command on VMs where file transfer succeeded
	var wg sync.WaitGroup
	var cmdResultArray []model.SshCmdResult
	var mu sync.Mutex

	for _, tr := range transferResults {
		if tr.Err != nil {
			continue // skip VMs where transfer failed
		}
		wg.Add(1)
		go func(nodeId string, nodeIp string) {
			defer wg.Done()
			stdout, stderr, cmdErr := RunRemoteCommand(nsId, infraId, nodeId, "", []string{command})
			if stdout == nil {
				stdout = map[int]string{}
			}
			if stderr == nil {
				stderr = map[int]string{}
			}
			cmdResult := model.SshCmdResult{
				InfraId: infraId,
				NodeId:  nodeId,
				NodeIp:  nodeIp,
				Command: map[int]string{0: command},
				Stdout:  stdout,
				Stderr:  stderr,
				Err:     cmdErr,
			}
			mu.Lock()
			cmdResultArray = append(cmdResultArray, cmdResult)
			mu.Unlock()
		}(tr.NodeId, tr.NodeIp)
	}
	wg.Wait()
	result.CmdResults = cmdResultArray

	return result, nil
}

// transferFileToNodeViaBastion is a function to transfer a file to a specific Node via Bastion Host
func transferFileToNodeViaBastion(nsId string, infraId string, nodeId string, targetSshInfo model.SshInfo, fileData []byte, fileName string, targetPath string) error {

	bastionNodes, err := GetUsableBastionNodes(nsId, infraId, nodeId)
	if err != nil {
		return fmt.Errorf("failed to get bastion nodes: %v", err)
	}

	bastionNode := pickBastion(bastionNodes, nsId, infraId, nodeId)
	bastionNsId := bastionNode.NsId
	if bastionNsId == "" {
		bastionNsId = nsId
	}
	bastionIp, _, bastionSshPort, err := GetNodeIp(bastionNsId, bastionNode.InfraId, bastionNode.NodeId)
	if err != nil {
		return fmt.Errorf("failed to get bastion Node IP and SSH port: %v", err)
	}

	// For cross-Infra/cross-NS bastions, override the target endpoint with the public IP.
	_, _, targetSshPort, ipErr := GetNodeIp(nsId, infraId, nodeId)
	if ipErr == nil {
		if resolved := resolveTargetIpForBastion(nsId, infraId, nodeId, bastionNode); resolved != "" {
			targetSshInfo.EndPoint = fmt.Sprintf("%s:%d", resolved, targetSshPort)
		}
	}

	bastionUserName, bastionPrivateKey, err := VerifySshUserName(bastionNsId, bastionNode.InfraId, bastionNode.NodeId, bastionIp, bastionSshPort, "")
	if err != nil {
		return fmt.Errorf("failed to verify SSH username for bastion: %v", err)
	}

	bastionSshInfo := model.SshInfo{
		EndPoint:   fmt.Sprintf("%s:%d", bastionIp, bastionSshPort),
		UserName:   bastionUserName,
		PrivateKey: []byte(bastionPrivateKey),
	}

	// Set TOFU context for bastion and target VMs
	bastionCtx := tofuContext{
		NsId:    bastionNsId,
		InfraId: bastionNode.InfraId,
		NodeId:  bastionNode.NodeId,
	}
	targetCtx := tofuContext{
		NsId:    nsId,
		InfraId: infraId,
		NodeId:  nodeId,
	}

	scpRetryCount := 3
	for attempt := range scpRetryCount {
		acquireBastionSlot(bastionSshInfo.EndPoint)
		err = runSCPWithBastion(bastionSshInfo, targetSshInfo, fileData, fileName, targetPath, bastionCtx, targetCtx)
		releaseBastionSlot(bastionSshInfo.EndPoint)

		if err == nil {
			break
		}

		isTransient := strings.Contains(err.Error(), "unexpected packet") ||
			strings.Contains(err.Error(), "handshake failed") ||
			strings.Contains(err.Error(), "EOF")

		if !isTransient || attempt == scpRetryCount-1 {
			return fmt.Errorf("failed to transfer file to Node via bastion: %v", err)
		}

		waitTime := time.Duration(3*(attempt+1)) * time.Second
		log.Warn().Err(err).Msgf("SCP transient failure to VM %s, retrying in %v (attempt %d/%d)", nodeId, waitTime, attempt+1, scpRetryCount)
		time.Sleep(waitTime)
	}

	log.Info().Msgf("File successfully transferred to VM %s via bastion", nodeId)
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
	targetFullPath := fmt.Sprintf("%s/%s", normalizeRemotePath(targetPath), fileName)
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

// DownloadFileFromInfraNode downloads a file from a specific Node in Infra by SCP through bastion hosts
func DownloadFileFromInfraNode(nsId string, infraId string, nodeId string, sourcePath string) ([]byte, string, error) {

	_, targetNodeIP, targetSshPort, err := GetNodeIp(nsId, infraId, nodeId)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get Node IP: %v", err)
	}

	// Check Node status before executing file download
	nodeInfo, err := GetNodeObject(nsId, infraId, nodeId)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get Node status: %v", err)
	}
	if nodeInfo.Status != model.StatusRunning {
		return nil, "", fmt.Errorf("Node '%s' is in '%s' status (not Running). Please change the Node status to Running and try again", nodeId, nodeInfo.Status)
	}

	targetUserName, targetPrivateKey, err := VerifySshUserName(nsId, infraId, nodeId, targetNodeIP, targetSshPort, "")
	if err != nil {
		return nil, "", fmt.Errorf("failed to verify SSH username: %v", err)
	}

	targetSshInfo := model.SshInfo{
		EndPoint:   fmt.Sprintf("%s:%d", targetNodeIP, targetSshPort),
		UserName:   targetUserName,
		PrivateKey: []byte(targetPrivateKey),
	}

	// Download file from VM via bastion
	fileData, fileName, err := downloadFileFromNodeViaBastion(nsId, infraId, nodeId, targetSshInfo, sourcePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download file from VM: %v", err)
	}

	log.Info().Msgf("Successfully downloaded file '%s' (%d bytes) from VM %s", fileName, len(fileData), nodeId)
	return fileData, fileName, nil
}

// downloadFileFromNodeViaBastion downloads a file from a specific VM via Bastion Host using SCP
func downloadFileFromNodeViaBastion(nsId string, infraId string, nodeId string, targetSshInfo model.SshInfo, sourcePath string) ([]byte, string, error) {

	bastionNodes, err := GetUsableBastionNodes(nsId, infraId, nodeId)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get bastion nodes: %w", err)
	}

	bastionNode := pickBastion(bastionNodes, nsId, infraId, nodeId)
	bastionNsId := bastionNode.NsId
	if bastionNsId == "" {
		bastionNsId = nsId
	}
	bastionIp, _, bastionSshPort, err := GetNodeIp(bastionNsId, bastionNode.InfraId, bastionNode.NodeId)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get bastion Node IP and SSH port: %v", err)
	}

	// For cross-Infra/cross-NS bastions, override the target endpoint with the public IP.
	_, _, targetSshPort, ipErr := GetNodeIp(nsId, infraId, nodeId)
	if ipErr == nil {
		if resolved := resolveTargetIpForBastion(nsId, infraId, nodeId, bastionNode); resolved != "" {
			targetSshInfo.EndPoint = fmt.Sprintf("%s:%d", resolved, targetSshPort)
		}
	}

	bastionUserName, bastionPrivateKey, err := VerifySshUserName(bastionNsId, bastionNode.InfraId, bastionNode.NodeId, bastionIp, bastionSshPort, "")
	if err != nil {
		return nil, "", fmt.Errorf("failed to verify SSH username for bastion: %v", err)
	}

	bastionSshInfo := model.SshInfo{
		EndPoint:   fmt.Sprintf("%s:%d", bastionIp, bastionSshPort),
		UserName:   bastionUserName,
		PrivateKey: []byte(bastionPrivateKey),
	}

	// Set TOFU context for bastion and target VMs
	bastionCtx := tofuContext{
		NsId:    bastionNsId,
		InfraId: bastionNode.InfraId,
		NodeId:  bastionNode.NodeId,
	}
	targetCtx := tofuContext{
		NsId:    nsId,
		InfraId: infraId,
		NodeId:  nodeId,
	}

	fileData, fileName, err := runSCPDownloadWithBastion(bastionSshInfo, targetSshInfo, sourcePath, bastionCtx, targetCtx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download file from VM via bastion: %v", err)
	}

	return fileData, fileName, nil
}

// normalizeRemotePath rewrites "~"-prefixed paths as home-relative paths.
// Quoted scp paths are not shell-expanded, but scp resolves relative paths
// against the login user's home directory.
func normalizeRemotePath(path string) string {
	if path == "~" {
		return "."
	}
	if strings.HasPrefix(path, "~/") {
		if p := strings.TrimPrefix(path, "~/"); p != "" {
			return p
		}
		return "."
	}
	return path
}

// runSCPDownloadWithBastion downloads a file using SCP over SSH via a Bastion host (SCP source mode: scp -f)
func runSCPDownloadWithBastion(bastionInfo model.SshInfo, targetInfo model.SshInfo, sourcePath string, bastionCtx tofuContext, targetCtx tofuContext) ([]byte, string, error) {
	log.Info().Msgf("Setting up SCP download connection via Bastion Host for: %s", sourcePath)

	// Parse the private key for the bastion host
	bastionSigner, err := ssh.ParsePrivateKey(bastionInfo.PrivateKey)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse bastion private key: %v", err)
	}

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
		return nil, "", fmt.Errorf("failed to parse target private key: %v", err)
	}

	targetConfig := &ssh.ClientConfig{
		User: targetInfo.UserName,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(targetSigner),
		},
		HostKeyCallback: createTOFUHostKeyCallback(targetCtx),
		Timeout:         30 * time.Second,
	}

	// Setup the bastion host connection
	bastionClient, err := ssh.Dial("tcp", bastionInfo.EndPoint, bastionConfig)
	if err != nil {
		return nil, "", fmt.Errorf("failed to dial bastion: %v", err)
	}
	defer bastionClient.Close()

	// Setup the actual SSH client through the bastion host
	conn, err := bastionClient.Dial("tcp", targetInfo.EndPoint)
	if err != nil {
		return nil, "", fmt.Errorf("failed to dial target via bastion: %v", err)
	}

	ncc, chans, reqs, err := ssh.NewClientConn(conn, targetInfo.EndPoint, targetConfig)
	if err != nil {
		conn.Close()
		return nil, "", fmt.Errorf("failed to create target SSH connection: %v", err)
	}
	client := ssh.NewClient(ncc, chans, reqs)
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return nil, "", fmt.Errorf("failed to create SSH session: %v", err)
	}
	defer session.Close()

	// Set up pipes
	stdout, err := session.StdoutPipe()
	if err != nil {
		return nil, "", fmt.Errorf("failed to set up stdout pipe: %v", err)
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return nil, "", fmt.Errorf("failed to set up stderr pipe: %v", err)
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		return nil, "", fmt.Errorf("failed to set up stdin for SCP: %v", err)
	}

	// Validate sourcePath to prevent command injection
	if strings.ContainsAny(sourcePath, "'\"\n\r\x00") {
		stdin.Close()
		return nil, "", fmt.Errorf("invalid sourcePath: contains disallowed characters")
	}
	sourcePath = normalizeRemotePath(sourcePath)

	// Start SCP in source mode (download: scp -f)
	cmd := fmt.Sprintf("scp -f '%s'", sourcePath)
	log.Info().Msgf("Executing SCP download command: %s", cmd)

	if err := session.Start(cmd); err != nil {
		stdin.Close()
		return nil, "", fmt.Errorf("failed to start SCP download command: %v", err)
	}

	// Capture stderr in background for error diagnostics
	stderrBuf := new(bytes.Buffer)
	go io.Copy(stderrBuf, stderr)

	reader := bufio.NewReader(stdout)

	// Step 1: Send initial ready signal (\x00 = null byte)
	if _, err := stdin.Write([]byte{0}); err != nil {
		stdin.Close()
		return nil, "", fmt.Errorf("failed to send initial ready signal: %v", err)
	}

	// Step 2: Read file header line: "C<mode> <size> <filename>\n"
	headerLine, err := reader.ReadString('\n')
	if err != nil {
		stdin.Close()
		return nil, "", fmt.Errorf("failed to read SCP file header: %v, stderr: %s", err, stderrBuf.String())
	}
	headerLine = strings.TrimRight(headerLine, "\n")

	// Check for error response from SCP (starts with \x01 or \x02)
	if len(headerLine) > 0 && (headerLine[0] == 1 || headerLine[0] == 2) {
		stdin.Close()
		return nil, "", fmt.Errorf("SCP server error: %s", headerLine[1:])
	}

	// Parse the header: C<mode> <size> <filename>
	if !strings.HasPrefix(headerLine, "C") {
		stdin.Close()
		return nil, "", fmt.Errorf("unexpected SCP header format: %s", headerLine)
	}

	var mode string
	var fileSize int64
	var fileName string
	_, err = fmt.Sscanf(headerLine, "C%s %d %s", &mode, &fileSize, &fileName)
	if err != nil {
		stdin.Close()
		return nil, "", fmt.Errorf("failed to parse SCP header '%s': %v", headerLine, err)
	}

	log.Info().Msgf("SCP download: receiving file '%s' (size: %d bytes, mode: %s)", fileName, fileSize, mode)

	// File size limit: 200MB
	fileSizeLimit := int64(200 * 1024 * 1024)
	if fileSize > fileSizeLimit {
		stdin.Close()
		return nil, "", fmt.Errorf("file too large: %d bytes (limit: %d bytes)", fileSize, fileSizeLimit)
	}

	// Step 3: Acknowledge header (send \x00)
	if _, err := stdin.Write([]byte{0}); err != nil {
		stdin.Close()
		return nil, "", fmt.Errorf("failed to acknowledge SCP header: %v", err)
	}

	// Step 4: Read the file data
	fileData := make([]byte, fileSize)
	_, err = io.ReadFull(reader, fileData)
	if err != nil {
		stdin.Close()
		return nil, "", fmt.Errorf("failed to read file data: %v", err)
	}

	// Step 5: Read the trailing null byte (\x00) from server indicating transfer complete
	eofByte := make([]byte, 1)
	_, err = io.ReadFull(reader, eofByte)
	if err != nil {
		stdin.Close()
		return nil, "", fmt.Errorf("failed to read EOF marker: %v", err)
	}

	// Step 6: Send final acknowledgment (\x00)
	if _, err := stdin.Write([]byte{0}); err != nil {
		// Non-fatal: file data already received
		log.Warn().Err(err).Msg("Failed to send final acknowledgment (non-fatal)")
	}

	stdin.Close()

	// Wait for session to complete
	if err := session.Wait(); err != nil {
		// Log but don't fail — file data already received successfully
		log.Warn().Err(err).Msgf("SCP session exit (file data received successfully), stderr: %s", stderrBuf.String())
	}

	log.Info().Msgf("File '%s' (%d bytes) successfully downloaded via Bastion", fileName, fileSize)
	return fileData, fileName, nil
}
