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
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/ssh"
)

// DefaultTerminalTicketTTL is the validity window for a one-time WebSocket terminal ticket.
const DefaultTerminalTicketTTL = 45 * time.Second

// TerminalTicket represents a short-lived, single-use authorization token for opening
// an interactive WebSocket SSH terminal session from browser clients (which cannot attach
// custom Authorization headers directly on the WebSocket upgrade request).
type TerminalTicket struct {
	Ticket           string    `json:"ticket"`
	NsId             string    `json:"nsId"`
	InfraId          string    `json:"infraId"`
	NodeId           string    `json:"nodeId"`
	UserName         string    `json:"userName,omitempty"`
	CredentialHolder string    `json:"credentialHolder,omitempty"`
	ExpiresAt        time.Time `json:"expiresAt"`
}

var terminalTickets sync.Map // map[string]*TerminalTicket

func sweepExpiredTerminalTickets(now time.Time) {
	terminalTickets.Range(func(key, value any) bool {
		if t, ok := value.(*TerminalTicket); ok && now.After(t.ExpiresAt) {
			terminalTickets.Delete(key)
		}
		return true
	})
}

// IssueTerminalTicket validates the target node and mints a single-use ticket for WebSocket terminal upgrade.
func IssueTerminalTicket(nsId, infraId, nodeId, userName, credentialHolder string, ttl time.Duration) (*TerminalTicket, error) {
	if err := common.CheckString(nsId); err != nil {
		return nil, fmt.Errorf("invalid nsId: %w", err)
	}
	if err := common.CheckString(infraId); err != nil {
		return nil, fmt.Errorf("invalid infraId: %w", err)
	}
	if err := common.CheckString(nodeId); err != nil {
		return nil, fmt.Errorf("invalid nodeId: %w", err)
	}

	userName = strings.TrimSpace(userName)
	if strings.EqualFold(userName, "default") {
		userName = ""
	}
	if userName != "" {
		for _, r := range userName {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == '.') {
				return nil, fmt.Errorf("invalid SSH userName %q", userName)
			}
		}
	}

	if _, err := GetNodeObject(nsId, infraId, nodeId); err != nil {
		return nil, fmt.Errorf("node '%s' not found in infra '%s' (namespace '%s'): %w", nodeId, infraId, nsId, err)
	}

	if ttl <= 0 {
		ttl = DefaultTerminalTicketTTL
	}
	now := time.Now()
	sweepExpiredTerminalTickets(now)

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("failed to generate terminal ticket: %w", err)
	}
	token := hex.EncodeToString(raw)

	entry := &TerminalTicket{
		Ticket:           token,
		NsId:             nsId,
		InfraId:          infraId,
		NodeId:           nodeId,
		UserName:         userName,
		CredentialHolder: credentialHolder,
		ExpiresAt:        now.Add(ttl),
	}
	terminalTickets.Store(token, entry)
	return entry, nil
}

// ConsumeTerminalTicket atomically claims and invalidates a single-use terminal ticket.
func ConsumeTerminalTicket(token string) (*TerminalTicket, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("missing terminal ticket")
	}
	val, ok := terminalTickets.LoadAndDelete(token)
	if !ok {
		return nil, fmt.Errorf("invalid or already-used terminal ticket")
	}
	entry, ok := val.(*TerminalTicket)
	if !ok || entry == nil {
		return nil, fmt.Errorf("corrupted terminal ticket")
	}
	if time.Now().After(entry.ExpiresAt) {
		return nil, fmt.Errorf("terminal ticket has expired")
	}
	return entry, nil
}

// TerminalSession encapsulates an active interactive PTY shell over SSH (direct or via bastion).
type TerminalSession struct {
	Stdin          io.WriteCloser
	Stdout         io.Reader
	Stderr         io.Reader
	TargetEndpoint string
	TargetUser     string
	ViaBastion     string

	session       *ssh.Session
	targetClient  *ssh.Client
	bastionClient *ssh.Client
	closeOnce     sync.Once
}

// Resize updates the remote PTY window dimensions (SIGWINCH).
func (ts *TerminalSession) Resize(cols, rows int) error {
	if ts == nil || ts.session == nil {
		return nil
	}
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}
	if cols > 1000 {
		cols = 1000
	}
	if rows > 500 {
		rows = 500
	}
	return ts.session.WindowChange(rows, cols)
}

// KeepAlive sends an OpenSSH-compatible keepalive request over the SSH connection.
func (ts *TerminalSession) KeepAlive() error {
	if ts == nil || ts.targetClient == nil {
		return nil
	}
	_, _, err := ts.targetClient.SendRequest("keepalive@openssh.com", true, nil)
	return err
}

// Wait blocks until the remote shell process exits or the SSH channel closes.
func (ts *TerminalSession) Wait() error {
	if ts == nil || ts.session == nil {
		return nil
	}
	return ts.session.Wait()
}

// Close tears down the shell, SSH session, target SSH client, and optional bastion SSH client.
func (ts *TerminalSession) Close() error {
	if ts == nil {
		return nil
	}
	ts.closeOnce.Do(func() {
		if ts.Stdin != nil {
			_ = ts.Stdin.Close()
		}
		if ts.session != nil {
			_ = ts.session.Close()
		}
		if ts.targetClient != nil {
			_ = ts.targetClient.Close()
		}
		if ts.bastionClient != nil {
			_ = ts.bastionClient.Close()
		}
	})
	return nil
}

func resolveTerminalUserAndKey(nsId, infraId, nodeId, nodeIp string, sshPort int, givenUserName string) (string, string, error) {
	keyUser, verifiedUser, privateKey, err := GetNodeSshKey(nsId, infraId, nodeId)
	if err != nil {
		return "", "", err
	}

	user := strings.TrimSpace(givenUserName)
	if strings.EqualFold(user, "default") {
		user = ""
	}
	if user == "" {
		if verifiedUser != "" {
			user = verifiedUser
		} else if keyUser != "" {
			user = keyUser
		} else if nodeObj, nErr := GetNodeObject(nsId, infraId, nodeId); nErr == nil && strings.TrimSpace(nodeObj.NodeUserName) != "" {
			user = strings.TrimSpace(nodeObj.NodeUserName)
		} else {
			user = model.SshDefaultUserName[0]
		}
	}
	return user, privateKey, nil
}

// OpenNodeTerminalSession dials the target Infra node (automatically routing via an assigned
// Bastion when the node is in a private subnet) and starts an interactive xterm-256color shell.
func OpenNodeTerminalSession(ctx context.Context, nsId, infraId, nodeId, givenUserName string, cols, rows int, onStatus func(string)) (*TerminalSession, error) {
	notify := func(msg string) {
		if onStatus != nil {
			onStatus(msg)
		}
	}

	_, targetNodeIP, targetSshPort, err := GetNodeIp(nsId, infraId, nodeId)
	if err != nil {
		return nil, err
	}
	targetUserName, targetPrivateKey, err := resolveTerminalUserAndKey(nsId, infraId, nodeId, targetNodeIP, targetSshPort, givenUserName)
	if err != nil {
		return nil, err
	}

	var opened *TerminalSession
	err = withBastionFailover(nsId, infraId, nodeId, func(bastionNode model.BastionNode) error {
		if bastionNode.NodeId == "" {
			return fmt.Errorf("bastion node has empty Node ID")
		}
		bastionNsId := bastionNode.NsId
		if bastionNsId == "" {
			bastionNsId = nsId
		}

		resolvedTargetIP := targetNodeIP
		if resolved := resolveTargetIpForBastion(nsId, infraId, nodeId, bastionNode); resolved != "" {
			resolvedTargetIP = resolved
		}

		bastionIp, _, bastionSshPort, ipErr := GetNodeIp(bastionNsId, bastionNode.InfraId, bastionNode.NodeId)
		if ipErr != nil {
			return ipErr
		}
		if bastionIp == "" {
			return fmt.Errorf("bastion VM (ID: %s) does not have a public IP address", bastionNode.NodeId)
		}
		if net.ParseIP(bastionIp) == nil {
			return fmt.Errorf("bastion VM (ID: %s) has invalid IP address: %s", bastionNode.NodeId, bastionIp)
		}

		isSelfBastion := bastionNsId == nsId && bastionNode.InfraId == infraId && bastionNode.NodeId == nodeId
		if isSelfBastion {
			if sgErr := checkNodeSshPortOpen(nsId, infraId, nodeId, targetSshPort, "target"); sgErr != nil {
				return sgErr
			}
		} else {
			if sgErr := checkNodeSshPortOpen(bastionNsId, bastionNode.InfraId, bastionNode.NodeId, bastionSshPort, "bastion"); sgErr != nil {
				return sgErr
			}
		}

		var bastionUserName, bastionSshKey string
		if isSelfBastion {
			bastionUserName = targetUserName
			bastionSshKey = targetPrivateKey
		} else {
			bastionUserName, bastionSshKey, ipErr = resolveTerminalUserAndKey(bastionNsId, bastionNode.InfraId, bastionNode.NodeId, bastionIp, bastionSshPort, "")
			if ipErr != nil {
				return ipErr
			}
		}

		bastionEndpoint := fmt.Sprintf("%s:%d", bastionIp, bastionSshPort)
		targetEndpoint := fmt.Sprintf("%s:%d", resolvedTargetIP, targetSshPort)
		if isSelfBastion {
			targetEndpoint = bastionEndpoint
			notify(fmt.Sprintf("Connecting to %s (%s) as %s...", nodeId, targetEndpoint, targetUserName))
		} else {
			notify(fmt.Sprintf("Connecting to %s (%s) as %s via bastion %s (%s)...", nodeId, targetEndpoint, targetUserName, bastionNode.NodeId, bastionEndpoint))
		}

		bastionInfo := model.SshInfo{
			EndPoint:   bastionEndpoint,
			UserName:   bastionUserName,
			PrivateKey: []byte(bastionSshKey),
		}
		targetInfo := model.SshInfo{
			EndPoint:   targetEndpoint,
			UserName:   targetUserName,
			PrivateKey: []byte(targetPrivateKey),
		}
		bastionTofu := tofuContext{NsId: bastionNsId, InfraId: bastionNode.InfraId, NodeId: bastionNode.NodeId}
		targetTofu := tofuContext{NsId: nsId, InfraId: infraId, NodeId: nodeId}

		sess, dialErr := openSSHInteractiveSession(
			ctx,
			bastionInfo,
			targetInfo,
			createTOFUHostKeyCallback(bastionTofu),
			createTOFUHostKeyCallback(targetTofu),
			isSelfBastion,
			cols,
			rows,
		)
		if dialErr != nil {
			return dialErr
		}
		if !isSelfBastion {
			sess.ViaBastion = fmt.Sprintf("%s (%s)", bastionNode.NodeId, bastionEndpoint)
		}
		opened = sess
		return nil
	})
	if err != nil {
		log.Error().Err(err).Str("nsId", nsId).Str("infraId", infraId).Str("nodeId", nodeId).Msg("Failed to open interactive SSH terminal")
		return nil, err
	}
	return opened, nil
}

// openSSHInteractiveSession dials the SSH client(s) — holding the bastion MaxStartups slot ONLY
// during TCP+SSH handshake — and allocates a PTY + interactive shell.
func openSSHInteractiveSession(
	ctx context.Context,
	bastionInfo model.SshInfo,
	targetInfo model.SshInfo,
	bastionHostKeyCb ssh.HostKeyCallback,
	targetHostKeyCb ssh.HostKeyCallback,
	isSelfBastion bool,
	cols, rows int,
) (*TerminalSession, error) {
	if cols <= 0 {
		cols = 100
	}
	if rows <= 0 {
		rows = 30
	}

	targetSigner, err := ssh.ParsePrivateKey(targetInfo.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse target SSH private key: %w", err)
	}
	targetConfig := &ssh.ClientConfig{
		User:            targetInfo.UserName,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(targetSigner)},
		HostKeyCallback: targetHostKeyCb,
		Timeout:         25 * time.Second,
	}

	var bastionConfig *ssh.ClientConfig
	if !isSelfBastion {
		bastionSigner, bErr := ssh.ParsePrivateKey(bastionInfo.PrivateKey)
		if bErr != nil {
			return nil, fmt.Errorf("failed to parse bastion SSH private key: %w", bErr)
		}
		bastionConfig = &ssh.ClientConfig{
			User:            bastionInfo.UserName,
			Auth:            []ssh.AuthMethod{ssh.PublicKeys(bastionSigner)},
			HostKeyCallback: bastionHostKeyCb,
			Timeout:         25 * time.Second,
		}
	}

	dialCtx, dialCancel := context.WithTimeout(ctx, 30*time.Second)
	defer dialCancel()

	var bastionClient *ssh.Client
	var targetClient *ssh.Client

	// Hold the per-bastion handshake concurrency slot ONLY while authenticating so
	// long-running interactive shells do not block batch RemoteCommand executions.
	handshakeErr := func() error {
		acquireBastionSlot(bastionInfo.EndPoint)
		defer releaseBastionSlot(bastionInfo.EndPoint)

		if isSelfBastion {
			c, dErr := dialSSHWithContext(dialCtx, "tcp", targetInfo.EndPoint, targetConfig)
			if dErr != nil {
				return fmt.Errorf("direct SSH connection to %s as %q failed: %w", targetInfo.EndPoint, targetInfo.UserName, dErr)
			}
			targetClient = c
			return nil
		}

		bClient, bErr := dialSSHWithContext(dialCtx, "tcp", bastionInfo.EndPoint, bastionConfig)
		if bErr != nil {
			return &bastionUnreachableError{fmt.Errorf("bastion SSH connection to %s as %q failed: %w", bastionInfo.EndPoint, bastionInfo.UserName, bErr)}
		}

		tunConn, tErr := dialTunnelWithContext(dialCtx, bClient, "tcp", targetInfo.EndPoint)
		if tErr != nil {
			_ = bClient.Close()
			return fmt.Errorf("tunnel from bastion %s to target %s failed: %w", bastionInfo.EndPoint, targetInfo.EndPoint, tErr)
		}

		handshakeDone := make(chan struct{})
		go func() {
			select {
			case <-dialCtx.Done():
				_ = tunConn.Close()
			case <-handshakeDone:
			}
		}()
		ncc, chans, reqs, cErr := ssh.NewClientConn(tunConn, targetInfo.EndPoint, targetConfig)
		close(handshakeDone)
		if cErr != nil {
			_ = tunConn.Close()
			_ = bClient.Close()
			return fmt.Errorf("SSH handshake with target %s as %q (via bastion %s) failed: %w", targetInfo.EndPoint, targetInfo.UserName, bastionInfo.EndPoint, cErr)
		}

		bastionClient = bClient
		targetClient = ssh.NewClient(ncc, chans, reqs)
		return nil
	}()
	if handshakeErr != nil {
		return nil, handshakeErr
	}

	cleanupClients := func() {
		if targetClient != nil {
			_ = targetClient.Close()
		}
		if bastionClient != nil {
			_ = bastionClient.Close()
		}
	}

	session, err := targetClient.NewSession()
	if err != nil {
		cleanupClients()
		return nil, fmt.Errorf("failed to open SSH session: %w", err)
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 115200,
		ssh.TTY_OP_OSPEED: 115200,
	}
	if err := session.RequestPty("xterm-256color", rows, cols, modes); err != nil {
		_ = session.Close()
		cleanupClients()
		return nil, fmt.Errorf("failed to allocate remote PTY: %w", err)
	}

	stdinPipe, err := session.StdinPipe()
	if err != nil {
		_ = session.Close()
		cleanupClients()
		return nil, fmt.Errorf("failed to attach stdin pipe: %w", err)
	}
	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		_ = session.Close()
		cleanupClients()
		return nil, fmt.Errorf("failed to attach stdout pipe: %w", err)
	}
	stderrPipe, err := session.StderrPipe()
	if err != nil {
		_ = session.Close()
		cleanupClients()
		return nil, fmt.Errorf("failed to attach stderr pipe: %w", err)
	}

	if err := session.Shell(); err != nil {
		_ = session.Close()
		cleanupClients()
		return nil, fmt.Errorf("failed to start remote interactive shell: %w", err)
	}

	return &TerminalSession{
		Stdin:          stdinPipe,
		Stdout:         stdoutPipe,
		Stderr:         stderrPipe,
		TargetEndpoint: targetInfo.EndPoint,
		TargetUser:     targetInfo.UserName,
		session:        session,
		targetClient:  targetClient,
		bastionClient: bastionClient,
	}, nil
}
