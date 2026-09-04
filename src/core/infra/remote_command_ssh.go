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
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/ssh"
)

// SshHostKeyMismatchError represents an SSH host key verification failure
// This error occurs when the stored host key doesn't match the server's current host key
type SshHostKeyMismatchError struct {
	NodeId              string
	StoredKeyType       string
	StoredFingerprint   string
	ReceivedKeyType     string
	ReceivedFingerprint string
}

func (e *SshHostKeyMismatchError) Error() string {
	return fmt.Sprintf("SSH host key verification failed for Node '%s': stored key fingerprint (%s %s) does not match received key (%s %s). "+
		"This could indicate a man-in-the-middle attack or the Node's host key has changed. "+
		"If you trust the new key, use the SSH host key reset API to update it.",
		e.NodeId, e.StoredKeyType, e.StoredFingerprint, e.ReceivedKeyType, e.ReceivedFingerprint)
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

// tofuContext contains Node identification info for TOFU host key verification (internal use only)
type tofuContext struct {
	NsId    string
	InfraId string
	NodeId  string
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
			Str("nodeId", ctx.NodeId).
			Str("hostname", hostname).
			Str("keyType", keyType).
			Str("fingerprint", fingerprint).
			Msg("SSH host key verification")

		// Get current Node info
		nodeInfo, err := GetNodeObject(ctx.NsId, ctx.InfraId, ctx.NodeId)
		if err != nil {
			// If Node info cannot be retrieved, reject connection for security
			log.Warn().
				Err(err).
				Str("nodeId", ctx.NodeId).
				Msg("Cannot retrieve Node info for TOFU verification, rejecting connection")
			return fmt.Errorf("cannot retrieve Node info for TOFU verification: %w", err)
		}

		// First connection (TOFU): store the host key
		if nodeInfo.SshHostKeyInfo == nil || nodeInfo.SshHostKeyInfo.HostKey == "" {
			log.Info().
				Str("nodeId", ctx.NodeId).
				Str("keyType", keyType).
				Str("fingerprint", fingerprint).
				Msg("First SSH connection - storing host key (TOFU)")

			nodeInfo.SshHostKeyInfo = &model.SshHostKeyInfo{
				HostKey:     keyData,
				KeyType:     keyType,
				Fingerprint: fingerprint,
				FirstUsedAt: time.Now().Format(time.RFC3339),
			}

			UpdateNodeInfo(ctx.NsId, ctx.InfraId, nodeInfo)

			return nil
		}

		// Subsequent connections: verify the host key
		if nodeInfo.SshHostKeyInfo.HostKey != keyData {
			log.Warn().
				Str("nodeId", ctx.NodeId).
				Str("storedKeyType", nodeInfo.SshHostKeyInfo.KeyType).
				Str("storedFingerprint", nodeInfo.SshHostKeyInfo.Fingerprint).
				Str("receivedKeyType", keyType).
				Str("receivedFingerprint", fingerprint).
				Msg("SSH host key mismatch detected")

			return &SshHostKeyMismatchError{
				NodeId:              ctx.NodeId,
				StoredKeyType:       nodeInfo.SshHostKeyInfo.KeyType,
				StoredFingerprint:   nodeInfo.SshHostKeyInfo.Fingerprint,
				ReceivedKeyType:     keyType,
				ReceivedFingerprint: fingerprint,
			}
		}

		log.Debug().
			Str("nodeId", ctx.NodeId).
			Str("fingerprint", fingerprint).
			Msg("SSH host key verified successfully")

		return nil
	}
}

// ResetNodeSshHostKey resets the stored SSH host key for a Node
// This should be called when the user trusts a new host key after verification failure
func ResetNodeSshHostKey(nsId string, infraId string, nodeId string) error {
	err := common.CheckString(nsId)
	if err != nil {
		return fmt.Errorf("invalid nsId: %w", err)
	}
	err = common.CheckString(infraId)
	if err != nil {
		return fmt.Errorf("invalid infraId: %w", err)
	}
	err = common.CheckString(nodeId)
	if err != nil {
		return fmt.Errorf("invalid nodeId: %w", err)
	}

	nodeInfo, err := GetNodeObject(nsId, infraId, nodeId)
	if err != nil {
		return fmt.Errorf("failed to get Node info: %w", err)
	}

	log.Info().
		Str("nodeId", nodeId).
		Str("previousKeyType", func() string {
			if nodeInfo.SshHostKeyInfo != nil {
				return nodeInfo.SshHostKeyInfo.KeyType
			}
			return ""
		}()).
		Str("previousFingerprint", func() string {
			if nodeInfo.SshHostKeyInfo != nil {
				return nodeInfo.SshHostKeyInfo.Fingerprint
			}
			return ""
		}()).
		Msg("Resetting SSH host key for Node")

	nodeInfo.SshHostKeyInfo = nil

	UpdateNodeInfo(nsId, infraId, nodeInfo)

	return nil
}

// GetNodeSshHostKey returns the stored SSH host key information for a Node
func GetNodeSshHostKey(nsId string, infraId string, nodeId string) (model.SshHostKeyInfo, error) {
	err := common.CheckString(nsId)
	if err != nil {
		return model.SshHostKeyInfo{}, fmt.Errorf("invalid nsId: %w", err)
	}
	err = common.CheckString(infraId)
	if err != nil {
		return model.SshHostKeyInfo{}, fmt.Errorf("invalid infraId: %w", err)
	}
	err = common.CheckString(nodeId)
	if err != nil {
		return model.SshHostKeyInfo{}, fmt.Errorf("invalid nodeId: %w", err)
	}

	nodeInfo, err := GetNodeObject(nsId, infraId, nodeId)
	if err != nil {
		return model.SshHostKeyInfo{}, fmt.Errorf("failed to get Node info: %w", err)
	}

	if nodeInfo.SshHostKeyInfo == nil {
		return model.SshHostKeyInfo{}, nil
	}

	return *nodeInfo.SshHostKeyInfo, nil
}

// runSSHWithContext executes SSH commands with context-based timeout and cancellation support.
//
// It transparently handles two connection modes based on the TOFU contexts:
//   - bastion-tunneled: bastionCtx and targetCtx identify different VMs. We
//     dial the bastion via SSH, then tunnel a TCP conn to the target, then
//     run the second SSH handshake over that tunnel.
//   - self-bastion (direct): bastionCtx == targetCtx, meaning the "bastion"
//     IS the target VM. The jump-loopback is wasteful (one transient SSH or
//     host-key hiccup would knock out *both* sides of an otherwise identical
//     connection), so we skip the bastion SSH entirely and dial the target
//     endpoint directly. Caller is responsible for setting targetInfo.EndPoint
//     to a publicly reachable address in this case (private IPs aren't
//     routable from cb-tumblebug).
func runSSHWithContext(ctx context.Context, bastionInfo model.SshInfo, targetInfo model.SshInfo, cmds []string, bastionCtx tofuContext, targetCtx tofuContext) (map[int]string, map[int]string, error) {
	stdoutMap := make(map[int]string)
	stderrMap := make(map[int]string)

	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return stdoutMap, stderrMap, fmt.Errorf("operation cancelled before start: %w", ctx.Err())
	default:
	}

	// Self-bastion shortcut: when the bastion identifies the same VM as the
	// target, there is no real jump host. Skip bastion key parsing & config
	// entirely and dial the target endpoint directly in the loop below.
	isSelfBastion := bastionCtx == targetCtx

	// Log connection details for debugging
	log.Debug().
		Str("bastionEndpoint", bastionInfo.EndPoint).
		Str("bastionUserName", bastionInfo.UserName).
		Str("targetEndpoint", targetInfo.EndPoint).
		Str("targetUserName", targetInfo.UserName).
		Bool("selfBastion", isSelfBastion).
		Msg("SSH connection attempt details (with context)")

	// Parse the private key for the bastion host — only needed when we will
	// actually SSH into the bastion (i.e. not the self-bastion case).
	var bastionConfig *ssh.ClientConfig
	if !isSelfBastion {
		bastionSigner, err := ssh.ParsePrivateKey(bastionInfo.PrivateKey)
		if err != nil {
			return stdoutMap, stderrMap, fmt.Errorf("failed to parse bastion private key: %v", err)
		}
		bastionConfig = &ssh.ClientConfig{
			User:            bastionInfo.UserName,
			Auth:            []ssh.AuthMethod{ssh.PublicKeys(bastionSigner)},
			HostKeyCallback: createTOFUHostKeyCallback(bastionCtx),
			Timeout:         30 * time.Second,
		}
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

	if isSelfBastion {
		log.Info().Msgf("Attempting direct connection to target host %s:%s (self-bastion)", targetHost, targetPort)
	} else {
		log.Info().Msgf("Attempting to connect to target host %s:%s via bastion", targetHost, targetPort)
	}

	acquireBastionSlot(bastionInfo.EndPoint)
	defer releaseBastionSlot(bastionInfo.EndPoint)

	// Anti-thundering-herd: when N targets fan out to the same bastion (e.g.
	// 100 VMs in one subnet sharing one auto-assigned bastion), simultaneous
	// dials from a single source IP trip OpenSSH's PerSourceMaxStartups and
	// MaxStartups, causing a chunk of connections to be RST'd. A small,
	// randomized pre-dial delay desynchronises the burst with negligible
	// impact on small-N cases. See applySSHDialJitter for the bound.
	applySSHDialJitter(ctx)

	// connectAndRun does one full attempt: dial -> SSH handshake -> execute.
	// It is wrapped by an outer transient-retry loop (post-handshake EOF /
	// connection-reset / ExitMissingError get one immediate re-dial). On the
	// retry path we re-enter this closure with a fresh dial; resources from
	// the previous attempt have already been released via the deferred
	// Close() calls inside the closure scope.
	connectAndRun := func() (map[int]string, map[int]string, error) {
		stdoutMap := make(map[int]string)
		stderrMap := make(map[int]string)

		retryCount := 3
		initialTimeout := 20 * time.Second
		maxTimeout := 60 * time.Second
		var bastionClient *ssh.Client
		var conn net.Conn
		var lastErr error

		for i := range retryCount {
			// Check if parent context is cancelled before each retry attempt
			select {
			case <-ctx.Done():
				return stdoutMap, stderrMap, fmt.Errorf("connection cancelled: %w", ctx.Err())
			default:
			}

			// Fix timeout calculation: start with initialTimeout for first attempt (i=0)
			// then progressively increase for subsequent attempts
			timeout := min(time.Duration(float64(initialTimeout)*(1.0+0.5*float64(i))), maxTimeout)

			log.Debug().Msgf("[Check Target via Bastion] %v:%v (Attempt %d/%d, Timeout: %v)",
				targetHost, targetPort, i+1, retryCount, timeout)

			// Use parent context as base for timeout context so cancellation propagates
			retryCtx, retryCancel := context.WithTimeout(ctx, timeout)

			connCh := make(chan net.Conn, 1)
			errCh := make(chan error, 1)
			sshClientCh := make(chan *ssh.Client, 1)

			go func() {
				if isSelfBastion {
					// Direct TCP dial — no bastion SSH hop. We send a nil *ssh.Client
					// down sshClientCh so the receiver's defer Close() stays safe.
					log.Debug().
						Str("targetEndpoint", targetInfo.EndPoint).
						Str("targetNodeId", targetCtx.NodeId).
						Msg("Attempting direct TCP dial to target host (self-bastion)")
					dialer := &net.Dialer{Timeout: timeout}
					targetConn, dErr := dialer.DialContext(retryCtx, "tcp", targetInfo.EndPoint)
					if dErr != nil {
						dErr = fmt.Errorf("[target-direct] failed to dial target %s (targetNodeId=%s, self-bastion): %v",
							targetInfo.EndPoint, targetCtx.NodeId, dErr)
						log.Error().
							Str("targetEndpoint", targetInfo.EndPoint).
							Str("targetNodeId", targetCtx.NodeId).
							Err(dErr).
							Msg("Direct TCP dial to target failed")
						errCh <- dErr
						return
					}
					sshClientCh <- nil
					connCh <- targetConn
					return
				}

				// Setup the bastion host connection. dialSSHWithContext is the
				// context-aware replacement for ssh.Dial — when retryCtx fires
				// it force-closes the underlying TCP socket and unblocks the
				// handshake within ms, instead of the stdlib's blind 30s wait.
				// This is critical during fan-out: without it, every retry
				// burns an extra zombie goroutine still hammering the bastion.
				log.Debug().
					Str("bastionEndpoint", bastionInfo.EndPoint).
					Str("bastionNodeId", bastionCtx.NodeId).
					Str("bastionUserName", bastionInfo.UserName).
					Msg("Attempting to dial bastion host")
				client, err := dialSSHWithContext(retryCtx, "tcp", bastionInfo.EndPoint, bastionConfig)
				if err != nil {
					// Tag the error so the retry/final-wrap layer can call out which
					// SIDE failed (bastion vs target) instead of presenting a single
					// opaque "failed to connect to target host" message.
					err = fmt.Errorf("[bastion] failed to establish SSH connection to bastion %s as user %q (bastionNodeId=%s): %v",
						bastionInfo.EndPoint, bastionInfo.UserName, bastionCtx.NodeId, err)
					log.Error().
						Str("bastionEndpoint", bastionInfo.EndPoint).
						Str("bastionUserName", bastionInfo.UserName).
						Str("bastionNodeId", bastionCtx.NodeId).
						Str("targetNodeId", targetCtx.NodeId).
						Err(err).
						Msg("Bastion SSH connection failed")
					errCh <- err
					return
				}
				log.Debug().Str("bastionEndpoint", bastionInfo.EndPoint).Msg("Successfully connected to bastion host")

				sshClientCh <- client

				// Tunnel dial via bastion. Also context-aware so a saturated
				// bastion can't hang us past retryCtx — without this, even a
				// successful bastion handshake could be wasted waiting for
				// the inner channel open on an overloaded sshd.
				log.Debug().Str("targetEndpoint", targetInfo.EndPoint).Msg("Attempting to dial target host via bastion")
				targetConn, err := dialTunnelWithContext(retryCtx, client, "tcp", targetInfo.EndPoint)
				if err != nil {
					client.Close()
					err = fmt.Errorf("[target-via-bastion] failed to dial target %s through bastion %s (bastionNodeId=%s, targetNodeId=%s): %v",
						targetInfo.EndPoint, bastionInfo.EndPoint, bastionCtx.NodeId, targetCtx.NodeId, err)
					log.Error().
						Str("bastionEndpoint", bastionInfo.EndPoint).
						Str("bastionNodeId", bastionCtx.NodeId).
						Str("targetEndpoint", targetInfo.EndPoint).
						Str("targetNodeId", targetCtx.NodeId).
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
				retryCancel()
				log.Info().Msgf("Successfully connected to target host on attempt %d", i+1)
				goto CONNECTION_ESTABLISHED
			case err := <-errCh:
				retryCancel()
				lastErr = err
				waitTime := time.Duration(3) * time.Second
				log.Warn().Err(err).Msgf("Failed to connect to target host. Attempt %d/%d. Retrying in %v...",
					i+1, retryCount, waitTime)
				// Use select with timer to allow cancellation during wait
				select {
				case <-ctx.Done():
					return stdoutMap, stderrMap, fmt.Errorf("connection cancelled during retry wait: %w", ctx.Err())
				case <-time.After(waitTime):
				}
			case <-retryCtx.Done():
				retryCancel()
				// Check if it's parent context cancellation or just timeout
				if ctx.Err() != nil {
					// Parent context cancelled - exit immediately
					return stdoutMap, stderrMap, fmt.Errorf("connection cancelled: %w", ctx.Err())
				}
				lastErr = retryCtx.Err()
				waitTime := time.Duration(3) * time.Second
				log.Warn().Err(lastErr).Msgf("Connection timeout. Attempt %d/%d. Retrying in %v...",
					i+1, retryCount, waitTime)
				// Use select with timer to allow cancellation during wait
				select {
				case <-ctx.Done():
					return stdoutMap, stderrMap, fmt.Errorf("connection cancelled during retry wait: %w", ctx.Err())
				case <-time.After(waitTime):
				}
			}
		}

		if isSelfBastion {
			return stdoutMap, stderrMap, fmt.Errorf(
				"failed to connect directly to target Node %q at %s (as %q) after %d attempts (self-bastion, no jump): %v",
				targetCtx.NodeId, targetInfo.EndPoint, targetInfo.UserName, retryCount, lastErr)
		}
		return stdoutMap, stderrMap, fmt.Errorf(
			"failed to connect to target Node %q at %s (as %q) via bastion Node %q at %s (as %q) after %d attempts: %v",
			targetCtx.NodeId, targetInfo.EndPoint, targetInfo.UserName,
			bastionCtx.NodeId, bastionInfo.EndPoint, bastionInfo.UserName,
			retryCount, lastErr)

	CONNECTION_ESTABLISHED:
		// bastionClient is nil in the self-bastion path (we never opened a bastion
		// SSH session). Guard the deferred Close to avoid a nil-pointer panic.
		if bastionClient != nil {
			defer bastionClient.Close()
		}
		defer conn.Close()

		// Context-cancellation watcher for the post-dial phase.
		//
		// Up to this point dialing is already context-aware (dialSSHWithContext
		// + dialTunnelWithContext). But the next steps — ssh.NewClientConn (the
		// SSH handshake on the just-established TCP conn), session creation,
		// and command execution inside executeCommandsOnSSHClient — all use
		// stdlib APIs that do NOT accept a context. They only honor the
		// ssh.ClientConfig.Timeout (default 30s) or block indefinitely on I/O.
		//
		// If parent ctx is cancelled (user cancel, infra-level timeout, VM
		// termination) during this window, those calls would keep running
		// until their own deadline fires, holding bastion slots and goroutines
		// for tens of seconds longer than necessary. We close the underlying
		// conn on ctx.Done so any blocked SSH I/O unblocks within milliseconds
		// with a "use of closed network connection" — which the caller then
		// treats as a transport error.
		watchDone := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				_ = conn.Close()
				if bastionClient != nil {
					_ = bastionClient.Close()
				}
			case <-watchDone:
			}
		}()
		defer close(watchDone)

		log.Debug().Msgf("Establishing SSH connection to target host with user: %s", targetInfo.UserName)

		if len(targetInfo.PrivateKey) == 0 {
			return stdoutMap, stderrMap, fmt.Errorf("empty private key for target host")
		}

		var ncc ssh.Conn
		var chans <-chan ssh.NewChannel
		var reqs <-chan *ssh.Request
		var sshErr error
		sshRetryCount := 3
		var lastSSHErr error

		for i := range sshRetryCount {
			ncc, chans, reqs, sshErr = ssh.NewClientConn(conn, targetInfo.EndPoint, targetConfig)
			if sshErr == nil {
				break
			}

			lastSSHErr = sshErr
			log.Warn().Err(sshErr).Msgf("SSH authentication failed. Attempt %d/%d", i+1, sshRetryCount)

			if strings.Contains(sshErr.Error(), "handshake failed") ||
				strings.Contains(sshErr.Error(), "no supported methods remain") {
				waitTime := time.Duration(3*(i+1)) * time.Second
				log.Info().Msgf("Waiting for SSH daemon to initialize. Retrying in %v...", waitTime)
				// Cancellation-aware sleep: user cancel / parent timeout fires
				// during the back-off should unblock immediately instead of
				// holding a bastion slot for the full back-off window.
				select {
				case <-ctx.Done():
					return stdoutMap, stderrMap, fmt.Errorf("operation cancelled during SSH retry wait: %w", ctx.Err())
				case <-time.After(waitTime):
				}
			} else {
				break
			}
		}

		if sshErr != nil {
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

		return executeCommandsOnSSHClient(ctx, client, cmds)
	}

	// Outer transient-retry loop. The inner connectAndRun already retries
	// dial/handshake 3× each, so this layer is specifically for *post-handshake*
	// hiccups: e.g. the bastion RSTs an established session mid-execution, the
	// remote sshd dies, or we get an ExitMissingError because the channel closed
	// without an exit code. One full re-dial usually clears these. Non-transient
	// errors (auth fail, context cancel, non-zero command exit) bypass the retry
	// and surface to the caller immediately.
	const maxOuterAttempts = 2
	var finalStdout, finalStderr map[int]string
	var attemptErr error
	for attempt := 1; attempt <= maxOuterAttempts; attempt++ {
		finalStdout, finalStderr, attemptErr = connectAndRun()
		if attemptErr == nil {
			break
		}
		if attempt >= maxOuterAttempts || !isTransientSSHError(attemptErr) {
			break
		}
		// A retry re-runs the command from the beginning, which is only safe while
		// nothing has run yet. Once the remote side has sent anything back, the
		// command is already executing (or has finished) and re-running a
		// side-effecting script on top of itself does more damage than reporting the
		// dropped transport. Observed in production: a 20-minute DevStack install
		// completed, an unrelated package upgrade restarted the target's sshd, and
		// the retry started a second install over the finished one.
		if producedRemoteOutput(finalStdout, finalStderr) {
			log.Warn().
				Err(attemptErr).
				Str("targetNodeId", targetCtx.NodeId).
				Msg("Transport dropped after the command had started producing output — not re-running it")
			break
		}
		log.Warn().
			Err(attemptErr).
			Str("targetNodeId", targetCtx.NodeId).
			Str("bastionNodeId", bastionCtx.NodeId).
			Bool("selfBastion", isSelfBastion).
			Int("attempt", attempt).
			Int("maxAttempts", maxOuterAttempts).
			Msg("Transient SSH error — reconnecting once with a fresh session")
		// Small settle delay before redial so we don't immediately re-collide
		// with whatever caused the first drop. Cancellation-aware.
		select {
		case <-ctx.Done():
			return finalStdout, finalStderr, fmt.Errorf("operation cancelled before transient retry: %w", ctx.Err())
		case <-time.After(2 * time.Second):
		}
	}
	return finalStdout, finalStderr, attemptErr
}

// executeCommandsOnSSHClient runs the given commands sequentially on an already
// established *ssh.Client and returns per-command stdout/stderr maps. It honors
// context cancellation between commands and during execution, and — when the
// context carries SSH log metadata (see withSSHLogMeta) — publishes line-level
// events to the SSE log broker for live streaming to UI clients.
//
// Both connection modes inside runSSHWithContext (bastion-tunneled and
// self-bastion direct) converge here once an *ssh.Client is established, so
// the SSH session/IO/streaming logic lives in exactly one place.
func executeCommandsOnSSHClient(ctx context.Context, client *ssh.Client, cmds []string) (map[int]string, map[int]string, error) {
	stdoutMap := make(map[int]string)
	stderrMap := make(map[int]string)

	// Run the commands with context support
	for i, cmd := range cmds {
		// Check if context is cancelled before each command
		select {
		case <-ctx.Done():
			log.Warn().Int("commandIndex", i).Msg("Context cancelled, stopping command execution")
			return stdoutMap, stderrMap, fmt.Errorf("operation cancelled: %w", ctx.Err())
		default:
		}

		log.Debug().Int("commandIndex", i).Str("command", cmd).Msg("Executing SSH command")

		// Create a new SSH session for each command
		session, err := client.NewSession()
		if err != nil {
			return stdoutMap, stderrMap, err
		}

		// Get pipes for stdout and stderr
		stdoutPipe, err := session.StdoutPipe()
		if err != nil {
			session.Close()
			return stdoutMap, stderrMap, err
		}

		stderrPipe, err := session.StderrPipe()
		if err != nil {
			session.Close()
			return stdoutMap, stderrMap, err
		}

		// Start the command
		if err := session.Start(cmd); err != nil {
			session.Close()
			return stdoutMap, stderrMap, err
		}

		// Read stdout and stderr with context awareness
		var stdoutBuf, stderrBuf bytes.Buffer
		stdoutDone := make(chan struct{})
		stderrDone := make(chan struct{})
		waitDone := make(chan error, 1)

		// Check if SSE streaming metadata is available in the context
		logMeta := getSSHLogMeta(ctx)

		// maxLogLineLen is the max bytes per log line published to SSE
		const maxLogLineLen = 131072 // 128KB per line (enough for base64-encoded files like kubeconfig)

		go func() {
			if logMeta != nil {
				// Streaming mode: use bufio.Scanner to publish lines in real time
				stdoutLineNum := 0
				scanner := bufio.NewScanner(io.TeeReader(stdoutPipe, io.MultiWriter(os.Stdout, &stdoutBuf)))
				scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // up to 1MB lines
				for scanner.Scan() {
					stdoutLineNum++
					line := scanner.Text()
					if len(line) > maxLogLineLen {
						line = line[:maxLogLineLen] + "...(truncated)"
					}
					PublishCommandEvent(logMeta.XRequestId, model.CommandStreamEvent{
						Type:         model.EventCommandLog,
						NodeId:       logMeta.NodeId,
						CommandIndex: logMeta.CommandIndex,
						Timestamp:    time.Now().Format(time.RFC3339Nano),
						Log: &model.CommandLogEntry{
							Stream:     "stdout",
							Line:       line,
							LineNumber: stdoutLineNum,
						},
					})
				}
				if err := scanner.Err(); err != nil {
					log.Error().Err(err).Msg("Error reading stdout from command")
				}
			} else {
				// Legacy mode: bulk copy
				io.Copy(io.MultiWriter(os.Stdout, &stdoutBuf), stdoutPipe)
			}
			close(stdoutDone)
		}()

		go func() {
			if logMeta != nil {
				stderrLineNum := 0
				scanner := bufio.NewScanner(io.TeeReader(stderrPipe, io.MultiWriter(os.Stderr, &stderrBuf)))
				scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
				for scanner.Scan() {
					stderrLineNum++
					line := scanner.Text()
					if len(line) > maxLogLineLen {
						line = line[:maxLogLineLen] + "...(truncated)"
					}
					PublishCommandEvent(logMeta.XRequestId, model.CommandStreamEvent{
						Type:         model.EventCommandLog,
						NodeId:       logMeta.NodeId,
						CommandIndex: logMeta.CommandIndex,
						Timestamp:    time.Now().Format(time.RFC3339Nano),
						Log: &model.CommandLogEntry{
							Stream:     "stderr",
							Line:       line,
							LineNumber: stderrLineNum,
						},
					})
				}
				if err := scanner.Err(); err != nil {
					log.Error().Err(err).Msg("Error reading stderr from command")
				}
			} else {
				io.Copy(io.MultiWriter(os.Stderr, &stderrBuf), stderrPipe)
			}
			close(stderrDone)
		}()

		// Wait for command completion in a separate goroutine
		go func() {
			waitDone <- session.Wait()
		}()

		// Wait for either context cancellation or command completion
		var waitErr error
		select {
		case <-ctx.Done():
			// Context cancelled - try to signal the remote process to terminate
			log.Warn().Int("commandIndex", i).Msg("Context cancelled during command execution, attempting to close session")

			// Send SIGTERM/SIGKILL to the remote process
			if signalErr := session.Signal(ssh.SIGTERM); signalErr != nil {
				log.Debug().Err(signalErr).Msg("Failed to send SIGTERM, trying to close session")
			}

			// Close the session to forcefully terminate
			session.Close()

			// Wait briefly for I/O goroutines to complete
			select {
			case <-stdoutDone:
			case <-time.After(2 * time.Second):
			}
			select {
			case <-stderrDone:
			case <-time.After(2 * time.Second):
			}

			stdoutMap[i] = stdoutBuf.String()
			stderrMap[i] = fmt.Sprintf("(cancelled: %s)\nStderr: %s", ctx.Err(), stderrBuf.String())
			return stdoutMap, stderrMap, fmt.Errorf("command execution cancelled: %w", ctx.Err())

		case waitErr = <-waitDone:
			// Command completed normally
			<-stdoutDone
			<-stderrDone
			session.Close()
		}

		if waitErr != nil {
			stderrMap[i] = fmt.Sprintf("(%s)\nStderr: %s", waitErr, stderrBuf.String())
			stdoutMap[i] = stdoutBuf.String()
			log.Warn().Err(waitErr).Int("commandIndex", i).Msg("Command execution failed")
			// Distinguish a clean non-zero exit (SSH transport OK, the command
			// itself reported failure) from a transport-level failure (EOF,
			// reset, dial timeout). Callers act on these very differently:
			// non-zero exit is the user's program's problem and stdout/stderr
			// is the real diagnostic; transport failure means a retry / a
			// different bastion / a routing fix is needed.
			var exitErr *ssh.ExitError
			if errors.As(waitErr, &exitErr) {
				return stdoutMap, stderrMap, &nonZeroExitError{inner: waitErr}
			}
			return stdoutMap, stderrMap, waitErr
		}

		stdoutMap[i] = stdoutBuf.String()
		stderrMap[i] = stderrBuf.String()
		log.Debug().Int("commandIndex", i).Msg("Command executed successfully")
	}

	return stdoutMap, stderrMap, nil
}

// runSSH is the legacy function maintained for backward compatibility
// It calls runSSHWithContext with a background context (no timeout)
// Deprecated: Use runSSHWithContext for new implementations
func runSSH(bastionInfo model.SshInfo, targetInfo model.SshInfo, cmds []string, bastionCtx tofuContext, targetCtx tofuContext) (map[int]string, map[int]string, error) {
	return runSSHWithContext(context.Background(), bastionInfo, targetInfo, cmds, bastionCtx, targetCtx)
}
