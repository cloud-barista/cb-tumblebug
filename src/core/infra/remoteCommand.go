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

// Package infra is to manage multi-cloud infra
package infra

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strconv"
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

// bastionMaxConcurrency is the maximum number of concurrent SSH connections
// allowed per bastion host. It matches the OpenSSH default MaxStartups value (10)
// so that parallel file transfers and remote commands do not exceed the bastion's
// built-in limit and trigger "unexpected packet in response to channel open" errors.
// Override with the TB_BASTION_MAX_CONCURRENCY environment variable.
var bastionMaxConcurrency = func() int {
	if v := os.Getenv("TB_BASTION_MAX_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 10 // matches OpenSSH default MaxStartups
}()

// bastionSemaphores holds one channel-based semaphore per bastion endpoint.
// Key: bastionEndpoint (host:port string), Value: chan struct{}
var bastionSemaphores sync.Map

// acquireBastionSlot acquires a concurrency slot for the given bastion endpoint.
// It creates the semaphore channel on first use. Call releaseBastionSlot when done.
func acquireBastionSlot(bastionEndpoint string) {
	sem, _ := bastionSemaphores.LoadOrStore(bastionEndpoint, make(chan struct{}, bastionMaxConcurrency))
	sem.(chan struct{}) <- struct{}{}
}

// releaseBastionSlot releases a previously acquired concurrency slot.
func releaseBastionSlot(bastionEndpoint string) {
	if sem, ok := bastionSemaphores.Load(bastionEndpoint); ok {
		<-sem.(chan struct{})
	}
}

// commandStatusHistoryLimit caps the number of CommandStatusInfo records
// retained per VM (Node). Each command status update rewrites the VM's
// entire etcd record, so without a cap this history grows without bound
// and can exhaust etcd's default 2GiB backend quota (see NOSPACE alarm /
// "database space exceeded" errors). Override with TB_COMMAND_STATUS_HISTORY_LIMIT.
var commandStatusHistoryLimit = func() int {
	if v := os.Getenv("TB_COMMAND_STATUS_HISTORY_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 30
}()

// sshDialJitterMaxMs is the upper bound for the per-connection randomized
// pre-dial delay. When a fan-out command targets N VMs sharing one bastion
// (e.g. 100 nodes in a single subnet), N parallel SSH dials from the same
// source IP collide on OpenSSH's PerSourceMaxStartups limiter and a chunk
// of them gets RST/dropped. A small randomized sleep before the actual dial
// spreads the burst over time, dramatically improving success rate without
// noticeably impacting small-N cases. Override with TB_SSH_DIAL_JITTER_MAX_MS.
var sshDialJitterMaxMs = func() int {
	if v := os.Getenv("TB_SSH_DIAL_JITTER_MAX_MS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return 750
}()

// applySSHDialJitter sleeps for a small random duration before an SSH dial,
// respecting the parent context (returns early on cancellation). Safe to call
// even when the cap is 0 (becomes a no-op).
func applySSHDialJitter(ctx context.Context) {
	if sshDialJitterMaxMs <= 0 {
		return
	}
	d := time.Duration(rand.Intn(sshDialJitterMaxMs+1)) * time.Millisecond
	if d == 0 {
		return
	}
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

// nonZeroExitError signals that the SSH transport succeeded end-to-end and
// the remote command ran to completion, but returned a non-zero exit status
// (e.g. user's script reported failure, kernel OOM-killer terminated a child,
// `exit 1`). This is operationally very different from a transport failure
// (bastion auth, dial timeout, mid-session EOF) — the user usually wants to
// see stdout/stderr and treat it as the command's own problem, not retry.
// Callers can detect it with errors.As / isNonZeroExitError.
type nonZeroExitError struct {
	inner error
}

func (e *nonZeroExitError) Error() string { return e.inner.Error() }
func (e *nonZeroExitError) Unwrap() error { return e.inner }

// isNonZeroExitError reports whether err (or anything it wraps) represents a
// successfully-transported remote command that simply returned non-zero.
func isNonZeroExitError(err error) bool {
	var nz *nonZeroExitError
	return errors.As(err, &nz)
}

// isTransientSSHError reports whether err looks like a *transport*-level
// hiccup where a single immediate re-dial is likely to succeed: peer closed
// the connection mid-stream, broken pipe, EOF before exit status, etc.
//
// It is intentionally narrow — these MUST NOT match:
//   - command's own non-zero exit (nonZeroExitError above; e.g. apt-get fail)
//   - context cancellation / deadline
//   - auth failures ("no supported methods remain")
//
// because retrying those would either be wrong (re-running a side-effecting
// command on success would change semantics) or pointless (auth won't suddenly
// work on a redial).
func isTransientSSHError(err error) bool {
	if err == nil || isNonZeroExitError(err) {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	// ExitMissingError: remote session closed without sending an exit status.
	// Typically caused by the channel being torn down mid-execution (kernel
	// reboot, network blip on the bastion, sshd restart) — worth one retry.
	var missing *ssh.ExitMissingError
	if errors.As(err, &missing) {
		return true
	}
	if errors.Is(err, io.EOF) {
		return true
	}
	msg := err.Error()
	// Auth failures are NOT transient — retrying with the same key will fail
	// the same way.
	if strings.Contains(msg, "no supported methods remain") ||
		strings.Contains(msg, "unable to authenticate") {
		return false
	}
	transientPatterns := []string{
		"EOF",
		"connection reset by peer",
		"broken pipe",
		"use of closed network connection",
		"unexpected packet",
		"session closed",
		"connection refused", // sshd briefly unavailable (restart / load)
	}
	for _, p := range transientPatterns {
		if strings.Contains(msg, p) {
			return true
		}
	}
	return false
}

// dialSSHWithContext is a context-aware replacement for ssh.Dial. The stdlib
// ssh.Dial ignores caller context and waits up to ClientConfig.Timeout
// (default 30s) before giving up — meaning when our retryCtx fires earlier
// (e.g. at 20s on the first attempt), the abandoned ssh.Dial keeps trying
// against the target for 10s+ more, in our case PILING extra parallel
// connections onto an already-saturated bastion. During a 100-VM fan-out
// this single oversight was producing the failure spiral observed in
// production: 285 "Connection timeout. Attempt N/3" entries for what
// should have been at most 99×3 = 297 attempts, with hundreds of
// concurrent zombie dials hammering one bastion VM.
//
// We split ssh.Dial into a cancellable net.Dialer.DialContext + a watcher-
// closed ssh.NewClientConn. When ctx fires, the underlying TCP connection
// is force-closed which unblocks NewClientConn within milliseconds.
func dialSSHWithContext(ctx context.Context, network, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	d := &net.Dialer{Timeout: config.Timeout}
	conn, err := d.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}
	// Watcher: force-close the TCP connection if ctx is cancelled before
	// the SSH handshake finishes, so NewClientConn unblocks immediately
	// instead of waiting on its own internal timeout.
	handshakeDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-handshakeDone:
		}
	}()
	ncc, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	close(handshakeDone)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return ssh.NewClient(ncc, chans, reqs), nil
}

// dialTunnelWithContext is the same pattern for the bastion->target
// tunnel step. *ssh.Client.Dial doesn't accept a context either, so we
// race it against ctx.Done and force-close the parent SSH client to
// unblock the tunnel-open if the caller has lost interest. Without this,
// a slow bastion can hold our goroutines blocked in client.Dial well
// past the parent retry window.
func dialTunnelWithContext(ctx context.Context, client *ssh.Client, network, addr string) (net.Conn, error) {
	type result struct {
		conn net.Conn
		err  error
	}
	resCh := make(chan result, 1)
	go func() {
		c, err := client.Dial(network, addr)
		resCh <- result{c, err}
	}()
	select {
	case r := <-resCh:
		return r.conn, r.err
	case <-ctx.Done():
		// Force-close the bastion client to unblock the in-flight Dial.
		// The goroutine will return an error shortly via resCh; we don't
		// wait for it because the parent has already lost interest.
		client.Close()
		return nil, ctx.Err()
	}
}

// sshLogMeta carries streaming context for SSE log publishing.
// It is stored in the context via sshLogMetaKey so that runSSHWithContext
// can publish real-time log lines without changing its function signature.
type sshLogMeta struct {
	XRequestId   string
	NodeId       string
	CommandIndex int
}

// contextKey is an unexported type for context keys in this package
type contextKey string

// sshLogMetaCtxKey is the context key for sshLogMeta
const sshLogMetaCtxKey contextKey = "sshLogMeta"

// withSSHLogMeta returns a new context carrying the given sshLogMeta
func withSSHLogMeta(ctx context.Context, meta *sshLogMeta) context.Context {
	return context.WithValue(ctx, sshLogMetaCtxKey, meta)
}

// getSSHLogMeta extracts sshLogMeta from context, or nil if not present
func getSSHLogMeta(ctx context.Context) *sshLogMeta {
	meta, _ := ctx.Value(sshLogMetaCtxKey).(*sshLogMeta)
	return meta
}

// cancelInfo stores cancel function and metadata for status updates
type cancelInfo struct {
	CancelFunc context.CancelFunc
	NsId       string
	InfraId    string
	NodeId     string
	XRequestId string
	Index      int
}

// cancelFuncs stores cancel functions for active command executions
// Key: "xRequestId:nodeId", Value: cancelInfo
// This allows cancelling running SSH commands per Node and updating their status
var cancelFuncs sync.Map

// makeCancelKey creates a unique key for cancel function storage
func makeCancelKey(xRequestId, nodeId string) string {
	return xRequestId + ":" + nodeId
}

// registerCancelFunc registers a cancel function for an xRequestId and nodeId with metadata
func registerCancelFunc(xRequestId, nodeId, nsId, infraId string, index int, cancel context.CancelFunc) {
	key := makeCancelKey(xRequestId, nodeId)
	info := cancelInfo{
		CancelFunc: cancel,
		NsId:       nsId,
		InfraId:    infraId,
		NodeId:     nodeId,
		XRequestId: xRequestId,
		Index:      index,
	}
	cancelFuncs.Store(key, info)
}

// unregisterCancelFunc removes a cancel function for an xRequestId and nodeId
func unregisterCancelFunc(xRequestId, nodeId string) {
	key := makeCancelKey(xRequestId, nodeId)
	cancelFuncs.Delete(key)
}

// cancelByKey cancels the command execution for a specific xRequestId and nodeId
// Returns true if the cancel function was found and called
func cancelByKey(xRequestId, nodeId string) bool {
	key := makeCancelKey(xRequestId, nodeId)
	if value, ok := cancelFuncs.LoadAndDelete(key); ok {
		if info, ok := value.(cancelInfo); ok {
			info.CancelFunc()
			return true
		}
	}
	return false
}

// CancelActiveCommandsForNode cancels all active command executions for a specific Node
// This is called when a Node is being terminated to immediately stop SSH sessions
// It also updates the command status to Cancelled in kvstore
// Returns the number of cancelled executions
func CancelActiveCommandsForNode(nodeId string) int {
	cancelled := 0
	cancelFuncs.Range(func(key, value any) bool {
		keyStr, ok := key.(string)
		if !ok {
			return true
		}
		// Key format is "xRequestId:nodeId", check if it ends with ":nodeId"
		suffix := ":" + nodeId
		if len(keyStr) > len(suffix) && keyStr[len(keyStr)-len(suffix):] == suffix {
			if info, ok := value.(cancelInfo); ok {
				log.Info().Str("nodeId", nodeId).Str("key", keyStr).Msg("Cancelling active SSH command due to VM termination")
				info.CancelFunc()
				cancelFuncs.Delete(key)

				// Update command status to Cancelled in kvstore
				err := UpdateCommandStatusInfo(info.NsId, info.InfraId, info.NodeId, info.Index,
					model.CommandStatusCancelled, "Command cancelled due to Node termination", "", "", "")
				if err != nil {
					log.Warn().Err(err).Str("nodeId", nodeId).Int("index", info.Index).Msg("Failed to update command status to Cancelled")
				}

				cancelled++
			}
		}
		return true
	})
	if cancelled > 0 {
		log.Info().Str("nodeId", nodeId).Int("cancelled", cancelled).Msg("Cancelled active SSH commands for terminating VM")
	}
	return cancelled
}

// TbInfraCmdReqStructLevelValidation is func to validate fields in model.InfraCmdReq
func TbInfraCmdReqStructLevelValidation(sl validator.StructLevel) {

	// u := sl.Current().Interface().(model.InfraCmdReq)

	// err := common.CheckString(u.Command)
	// if err != nil {
	// 	// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
	// 	sl.ReportError(u.Command, "command", "Command", err.Error(), "")
	// }
}

// RemoteCommandToInfra is func to command to all Nodes in Infra by SSH
// It now supports user-configurable timeout via InfraCmdReq.TimeoutMinutes
// Returns the task ID in x-task-id for tracking and cancellation
func RemoteCommandToInfra(nsId string, infraId string, nodeGroupId string, nodeId string, labelSelector string, req *model.InfraCmdReq, xRequestId string) ([]model.SshCmdResult, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	err = common.CheckString(infraId)
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

	check, _ := CheckInfra(nsId, infraId)

	if !check {
		temp := []model.SshCmdResult{}
		err := fmt.Errorf("The infra %s does not exist.", infraId)
		return temp, err
	}

	nodeList, err := ListNodeId(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	if len(nodeList) == 0 {
		err := fmt.Errorf("Infra %s has no Nodes to execute commands (status: Empty)", infraId)
		return nil, err
	}
	if nodeGroupId != "" {
		nodeListInGroup, err := ListNodeByNodeGroup(nsId, infraId, nodeGroupId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return nil, err
		}
		if nodeListInGroup == nil {
			err := fmt.Errorf("there is no %s nodeGroup or VM in the nodeGroup ", nodeGroupId)
			return nil, err
		}
		nodeList = nodeListInGroup
	}

	if nodeId != "" {
		nodeList = []string{nodeId}
	}

	// Apply label-based filtering if labelSelector is specified
	if labelSelector != "" {
		log.Info().Str("labelSelector", labelSelector).Msg("Filtering Nodes by label selector")

		// Add system label conditions
		systemLabelConditions := fmt.Sprintf("sys.infraId=%s", infraId)

		// Also add nodeGroupId condition if specified
		if nodeGroupId != "" {
			systemLabelConditions += fmt.Sprintf(",sys.nodeGroupId=%s", nodeGroupId)
		}

		labelSelector = systemLabelConditions + "," + labelSelector

		log.Debug().Str("combinedLabelSelector", labelSelector).Msg("Combined label selector")

		// Query resources using label selector
		matchedResources, err := label.GetResourcesByLabelSelector(model.StrNode, labelSelector)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get resources by label selector")
			return nil, fmt.Errorf("label selector error: %v", err)
		}

		if len(matchedResources) == 0 {
			log.Warn().Msg("No Nodes matched the label selector criteria")
			return nil, fmt.Errorf("no Nodes matched the label selector: %s", labelSelector)
		}

		// Extract matching Node IDs only
		filteredNodeIds := make([]string, 0, len(matchedResources))
		for _, resource := range matchedResources {
			if nodeInfo, ok := resource.(*model.NodeInfo); ok {
				filteredNodeIds = append(filteredNodeIds, nodeInfo.Id)
			}
		}

		log.Info().
			Int("matchedNodesCount", len(filteredNodeIds)).
			Str("labelSelector", labelSelector).
			Msg("Nodes filtered by label selector")

		// Replace Node list with label selector filtered Nodes
		nodeList = filteredNodeIds
	}

	// Get effective timeout from request (with validation and defaults)
	timeoutMinutes := req.GetEffectiveTimeout()

	// Create a parent context with timeout for overall execution
	// Each Node will have its own child context for individual cancellation
	timeout := time.Duration(timeoutMinutes) * time.Minute
	parentCtx, parentCancel := context.WithTimeout(context.Background(), timeout)
	defer parentCancel() // Ensure parent context is cancelled when function returns

	log.Info().
		Str("xRequestId", xRequestId).
		Int("timeoutMinutes", timeoutMinutes).
		Int("nodeCount", len(nodeList)).
		Strs("commands", req.Command).
		Msg("Starting remote command execution")

	// goroutine sync wg
	var wg sync.WaitGroup
	var resultMutex sync.Mutex

	var resultArray []model.SshCmdResult
	var completedCount int32

	// Preprocess commands for each Node and add command status info.
	//
	// We parallelize this with a worker pool. Each iteration is a small CPU
	// op (processCommand string substitution) plus one etcd KV round-trip in
	// AddCommandStatusInfo. With 100+ targets the sequential loop spent up to
	// a couple of seconds blocking BEFORE any SSH could even start, and
	// flooded the log with one "Command status added" line per node. The
	// per-Node etcd keys are independent so parallelization is race-free.
	// We cap concurrency to keep etcd from being slammed by a 1000-VM batch.
	const preprocessConcurrency = 20
	type preResult struct {
		nodeId   string
		commands []string
		cmdIndex int
		err      error
	}
	preCh := make(chan preResult, len(nodeList))
	preSem := make(chan struct{}, preprocessConcurrency)
	var preWg sync.WaitGroup
	for i, targetNodeId := range nodeList {
		preWg.Add(1)
		go func(i int, targetNodeId string) {
			defer preWg.Done()
			preSem <- struct{}{}
			defer func() { <-preSem }()

			processedCommands := make([]string, len(req.Command))
			for j, cmd := range req.Command {
				processedCmd, err := processCommand(cmd, nsId, infraId, targetNodeId, i)
				if err != nil {
					preCh <- preResult{nodeId: targetNodeId, err: err}
					return
				}
				processedCommands[j] = processedCmd
			}
			combinedCommand := strings.Join(req.Command, " && ")
			combinedProcessedCommand := strings.Join(processedCommands, " && ")
			cmdIndex, err := AddCommandStatusInfo(nsId, infraId, targetNodeId, xRequestId, combinedCommand, combinedProcessedCommand)
			if err != nil {
				// AddCommandStatusInfo failure is non-fatal: we still run the
				// command, just without tracking. Mirror the previous behavior.
				log.Error().Err(err).Str("nodeId", targetNodeId).Msg("Failed to add command status info")
				preCh <- preResult{nodeId: targetNodeId, commands: processedCommands}
				return
			}
			preCh <- preResult{nodeId: targetNodeId, commands: processedCommands, cmdIndex: cmdIndex}
		}(i, targetNodeId)
	}
	preWg.Wait()
	close(preCh)

	nodeCommands := make(map[string][]string, len(nodeList))
	nodeCommandIndices := make(map[string]int, len(nodeList))
	for r := range preCh {
		if r.err != nil {
			// processCommand error — preserves prior fail-fast semantics for
			// $$Func token errors etc.
			return nil, r.err
		}
		nodeCommands[r.nodeId] = r.commands
		if r.cmdIndex > 0 {
			nodeCommandIndices[r.nodeId] = r.cmdIndex
		}
	}

	// Execute commands in parallel using goroutines with per-Node context.
	//
	// DEPENDENCY-BASED SCHEDULING: when a target VM is *also* serving as the
	// bastion for other targets in the same batch (the classic dense-subnet
	// fan-out: 100 VMs in one subnet -> 1 auto-picked bastion -> 99 tunnels),
	// running the bastion's own (potentially heavy) command in parallel with
	// the 99 tunnels HAMMERS that one VM into the ground. In production we
	// have seen the bastion become unable to respond to TCP SYNs from the
	// tunneling peers AND fail to finish its own command, even when the
	// command would take ~60s on an idle VM. Defer such "active-bastion"
	// targets until every target tunneling THROUGH THEM has finished.
	//
	// The wait is per-bastion, not a global barrier: each deferred bastion
	// launches the moment its own tunneling dependents drain. A global
	// two-phase barrier (the previous design) let a single slow VM in one
	// subnet block the deferred bastions of every other, unrelated subnet.
	activeBastions := map[string]bool{}
	targetBastionOf := map[string]string{} // target -> its bastion, when the bastion is a DIFFERENT VM in this batch
	{
		// Cheap lookup: for each target, find its assigned bastion. If that
		// bastion ID matches another target in this batch (and is a different
		// VM), mark the bastion as "active for siblings". Errors during
		// lookup are non-fatal — we conservatively launch such nodes
		// immediately so behavior degrades to the previous all-parallel mode.
		nodeIdSet := make(map[string]bool, len(nodeList))
		for _, n := range nodeList {
			nodeIdSet[n] = true
		}
		for _, n := range nodeList {
			bs, err := GetBastionNodes(nsId, infraId, n)
			if err != nil || len(bs) == 0 {
				continue
			}
			bastionId := bs[0].NodeId
			if bastionId == "" || bastionId == n {
				continue // self-bastion — no contention with siblings
			}
			if nodeIdSet[bastionId] {
				activeBastions[bastionId] = true
				targetBastionOf[n] = bastionId
			}
		}
	}

	var immediateTargets, deferredBastionTargets []string
	for targetNodeId := range nodeCommands {
		if activeBastions[targetNodeId] {
			deferredBastionTargets = append(deferredBastionTargets, targetNodeId)
		} else {
			immediateTargets = append(immediateTargets, targetNodeId)
		}
	}

	// Count, per deferred bastion, how many immediate targets tunnel through
	// it. Only immediate targets are counted: two bastions using each other
	// (a dependency cycle) would otherwise wait forever — such pairs get a
	// zero count and launch right away, degrading to the previous parallel
	// mode for that pair only.
	pendingDependents := make(map[string]int, len(deferredBastionTargets))
	for _, t := range immediateTargets {
		if b, ok := targetBastionOf[t]; ok {
			pendingDependents[b]++
		}
	}

	if len(deferredBastionTargets) > 0 {
		log.Info().
			Str("xRequestId", xRequestId).
			Int("immediateCount", len(immediateTargets)).
			Int("deferredCount", len(deferredBastionTargets)).
			Strs("deferredBastions", deferredBastionTargets).
			Msg("Dependency-based execution: deferring each active-bastion target until its own tunneling dependents finish")
	}

	// Reserve one WaitGroup slot per target up front so every wg.Add happens
	// strictly before wg.Wait — deferred bastion targets launched dynamically
	// from onImmediateDone only consume a pre-reserved slot. Every target
	// launches exactly once: a deferred bastion is released either right away
	// (no pending dependents) or by its last finishing dependent.
	wg.Add(len(nodeCommands))

	launchOne := func(nodeId string, cmds []string, cmdIndex int, onDone func(nodeId string)) {
		go func() {
			defer wg.Done()

			// Create per-Node cancellable context (child of parent context)
			nodeCtx, nodeCancel := context.WithCancel(parentCtx)
			registerCancelFunc(xRequestId, nodeId, nsId, infraId, cmdIndex, nodeCancel)

			// Inject SSE streaming metadata into context so runSSHWithContext can publish log lines
			nodeCtx = withSSHLogMeta(nodeCtx, &sshLogMeta{
				XRequestId:   xRequestId,
				NodeId:       nodeId,
				CommandIndex: cmdIndex,
			})

			// Execute and clean up
			result := runRemoteCommandWithContextAndStatus(nodeCtx, nsId, infraId, nodeId, req.UserName, cmds, cmdIndex)

			// Unregister cancel func after completion
			unregisterCancelFunc(xRequestId, nodeId)
			nodeCancel() // Release resources

			resultMutex.Lock()
			resultArray = append(resultArray, result)
			completedCount++
			resultMutex.Unlock()

			if onDone != nil {
				onDone(nodeId)
			}
		}()
	}

	var depMutex sync.Mutex
	launchedBastions := make(map[string]bool, len(deferredBastionTargets))

	// onImmediateDone releases the finished target's bastion once ALL of the
	// bastion's tunneling dependents have completed (success or failure —
	// runRemoteCommandWithContextAndStatus always returns a result).
	onImmediateDone := func(nodeId string) {
		b, ok := targetBastionOf[nodeId]
		if !ok {
			return
		}
		depMutex.Lock()
		pendingDependents[b]--
		ready := pendingDependents[b] <= 0 && !launchedBastions[b]
		if ready {
			launchedBastions[b] = true
		}
		depMutex.Unlock()
		if ready {
			log.Info().
				Str("xRequestId", xRequestId).
				Str("bastionNodeId", b).
				Msg("All tunneling dependents finished — launching deferred bastion target")
			launchOne(b, nodeCommands[b], nodeCommandIndices[b], nil)
		}
	}

	for _, targetNodeId := range immediateTargets {
		launchOne(targetNodeId, nodeCommands[targetNodeId], nodeCommandIndices[targetNodeId], onImmediateDone)
	}

	// Deferred bastions with no immediate dependents (e.g., mutual-bastion
	// pairs, or dependents filtered out of this batch) have nothing to wait
	// for — launch them right away.
	for _, targetNodeId := range deferredBastionTargets {
		depMutex.Lock()
		ready := pendingDependents[targetNodeId] == 0 && !launchedBastions[targetNodeId]
		if ready {
			launchedBastions[targetNodeId] = true
		}
		depMutex.Unlock()
		if ready {
			launchOne(targetNodeId, nodeCommands[targetNodeId], nodeCommandIndices[targetNodeId], nil)
		}
	}

	// Waits for every target. All WaitGroup slots were reserved before any
	// goroutine started (wg.Add(len(nodeCommands)) above), so dynamically
	// launched deferred bastions cannot race this Wait.
	wg.Wait()

	// Publish CommandDone event to SSE subscribers
	completedNodes := 0
	failedNodes := 0
	for _, r := range resultArray {
		if r.Err != nil {
			failedNodes++
		} else {
			completedNodes++
		}
	}
	// Calculate wall clock elapsed from the start of the parent context
	// parentCtx was created with timeout, so deadline - timeout = start time
	var elapsedSec int64
	if deadline, ok := parentCtx.Deadline(); ok {
		startTime := deadline.Add(-timeout)
		elapsedSec = int64(time.Since(startTime).Seconds())
	}

	PublishCommandEvent(xRequestId, model.CommandStreamEvent{
		Type:      model.EventCommandDone,
		Timestamp: time.Now().Format(time.RFC3339Nano),
		Summary: &model.CommandDoneSummary{
			TotalNodes:     len(nodeList),
			CompletedNodes: completedNodes,
			FailedNodes:    failedNodes,
			ElapsedSeconds: elapsedSec,
		},
	})

	return resultArray, nil
}

// runRemoteCommandWithContextAndStatus executes SSH command with context and updates status
func runRemoteCommandWithContextAndStatus(ctx context.Context, nsId, infraId, nodeId, userName string, cmds []string, cmdIndex int) model.SshCmdResult {
	nodeIP, _, _, err := GetNodeIp(nsId, infraId, nodeId)

	result := model.SshCmdResult{
		InfraId: infraId,
		NodeId:  nodeId,
		NodeIp:  nodeIP,
		Command: make(map[int]string),
		Stdout:  make(map[int]string),
		Stderr:  make(map[int]string),
	}

	for i, c := range cmds {
		result.Command[i] = c
	}

	// Update status to Handling
	if cmdIndex > 0 {
		// A user may cancel a Queued command before it launches (e.g., a
		// deferred bastion target waiting on a hanging dependent). At that
		// point there is no running context to cancel, so the cancel API can
		// only flip the stored status — honor it here instead of silently
		// overwriting Cancelled with Handling and executing anyway.
		if existingStatus, getErr := GetCommandStatusInfo(nsId, infraId, nodeId, cmdIndex); getErr == nil && existingStatus != nil && existingStatus.Status == model.CommandStatusCancelled {
			log.Info().Str("nodeId", nodeId).Int("cmdIndex", cmdIndex).Msg("Skipping execution: command was cancelled while queued")
			result.Err = fmt.Errorf("command was cancelled before execution")
			return result
		}
		if updateErr := UpdateCommandStatusInfo(nsId, infraId, nodeId, cmdIndex, model.CommandStatusHandling, "", "", "", ""); updateErr != nil {
			log.Error().Err(updateErr).Int("cmdIndex", cmdIndex).Msg("Failed to update command status to Handling")
		}
	}

	if err != nil {
		result.Err = err
		if cmdIndex > 0 {
			UpdateCommandStatusInfo(nsId, infraId, nodeId, cmdIndex, model.CommandStatusFailed, "Failed to get Node IP", err.Error(), "", "")
		}
		return result
	}

	// Check Node status before executing SSH command
	nodeInfo, err := GetNodeObject(nsId, infraId, nodeId)
	if err != nil {
		result.Err = fmt.Errorf("failed to get Node status: %v", err)
		if cmdIndex > 0 {
			UpdateCommandStatusInfo(nsId, infraId, nodeId, cmdIndex, model.CommandStatusFailed, "Failed to get Node status", err.Error(), "", "")
		}
		return result
	}

	// Validate Node status for SSH execution
	if nodeInfo.Status != model.StatusRunning {
		var errorMsg string
		if nodeInfo.Status == model.StatusTerminated {
			errorMsg = fmt.Sprintf("Node '%s' is in '%s' status. SSH connection is impossible for terminated Nodes", nodeId, nodeInfo.Status)
		} else {
			errorMsg = fmt.Sprintf("Node '%s' is in '%s' status (not Running). Please change the Node status to Running and try again", nodeId, nodeInfo.Status)
		}
		result.Err = fmt.Errorf("%s", errorMsg)
		if cmdIndex > 0 {
			UpdateCommandStatusInfo(nsId, infraId, nodeId, cmdIndex, model.CommandStatusFailed, "Node not in running status", errorMsg, "", "")
		}
		return result
	}

	// Execute command with context
	stdout, stderr, err := RunRemoteCommandWithContext(ctx, nsId, infraId, nodeId, userName, cmds)

	result.Stdout = stdout
	result.Stderr = stderr

	if err != nil {
		result.Err = err

		// Determine status based on error type
		var status model.CommandExecutionStatus
		var summary string

		if ctx.Err() == context.DeadlineExceeded {
			status = model.CommandStatusTimeout
			summary = "Command execution timed out"
		} else if ctx.Err() == context.Canceled {
			// Context was cancelled - could be user cancel or Node termination
			// Check if status was already updated to Cancelled, if not, update it now
			if cmdIndex > 0 {
				existingStatus, getErr := GetCommandStatusInfo(nsId, infraId, nodeId, cmdIndex)
				if getErr == nil && existingStatus != nil && existingStatus.Status != model.CommandStatusCancelled {
					// Status not yet updated to Cancelled, do it now
					stdoutStr := mapToString(stdout)
					stderrStr := mapToString(stderr)
					UpdateCommandStatusInfo(nsId, infraId, nodeId, cmdIndex, model.CommandStatusCancelled,
						"Command execution cancelled", err.Error(), stdoutStr, stderrStr)
				}
			}
			log.Info().
				Str("nodeId", nodeId).
				Int("cmdIndex", cmdIndex).
				Msg("Command execution was cancelled")
			return result
		} else if isNonZeroExitError(err) {
			// SSH transport worked end-to-end; the remote command ran and
			// returned non-zero. Surface this as a distinct status so the UI
			// can show "the command failed on the VM" (stdout/stderr is the
			// useful diagnostic) instead of "we couldn't reach the VM".
			status = model.CommandStatusCompletedWithError
			summary = "Command ran with non-zero exit (SSH transport OK)"
		} else {
			status = model.CommandStatusFailed
			summary = "Command execution failed"
		}

		if cmdIndex > 0 {
			stdoutStr := mapToString(stdout)
			stderrStr := mapToString(stderr)
			UpdateCommandStatusInfo(nsId, infraId, nodeId, cmdIndex, status, summary, err.Error(), stdoutStr, stderrStr)
		}
		return result
	}

	// Success
	if cmdIndex > 0 {
		stdoutStr := mapToString(stdout)
		stderrStr := mapToString(stderr)
		UpdateCommandStatusInfo(nsId, infraId, nodeId, cmdIndex, model.CommandStatusCompleted, "Command executed successfully", "", stdoutStr, stderrStr)
	}

	log.Debug().Str("nodeId", nodeId).Msg("Command executed successfully")
	return result
}

// mapToString converts a map[int]string to a single string
func mapToString(m map[int]string) string {
	var result strings.Builder
	for _, v := range m {
		result.WriteString(v)
		result.WriteString("\n")
	}
	return result.String()
}

// resolveTargetIpForBastion returns the IP that should be used as the SSH tunnel
// destination for the given target Node.
//
// For same-Infra/same-NS bastions the target's privateIP is returned unchanged, because
// the bastion is on the same network and can reach it directly.
//
// For cross-Infra or cross-NS bastions the bastion host likely cannot route to the target's
// private network (e.g. OpenStack Neutron subnet). In that case the function prefers the
// public IP (e.g. OpenStack floating IP) retrieved first from the stored Node record and,
// if that is empty, via a live CSP fetch (same path as GetInfraAccessInfo).
//
// nsId/infraId/nodeId identify the *target* VM; bastionNode identifies the bastion.
func resolveTargetIpForBastion(nsId, infraId, nodeId string, bastionNode model.BastionNode) string {
	bastionNsId := bastionNode.NsId
	if bastionNsId == "" {
		bastionNsId = nsId
	}

	isCrossInfra := bastionNode.InfraId != infraId || bastionNsId != nsId
	if !isCrossInfra {
		// Same Infra/NS — the bastion can reach the private IP directly.
		_, privateIP, _, err := GetNodeIp(nsId, infraId, nodeId)
		if err != nil {
			return ""
		}
		return privateIP
	}

	// Cross-Infra/cross-NS: prefer public IP.
	publicIP, privateIP, _, err := GetNodeIp(nsId, infraId, nodeId)
	if err != nil {
		return ""
	}
	if publicIP == "" {
		// publicIP not in etcd — do a live CSP fetch (same path as GetInfraAccessInfo).
		if liveInfo, liveErr := GetNodeCurrentPublicIp(nsId, infraId, nodeId); liveErr == nil && liveInfo.PublicIp != "" {
			log.Info().
				Str("nodeId", nodeId).
				Str("publicIP", liveInfo.PublicIp).
				Msg("Cross-Infra bastion: retrieved publicIP from CSP (not in stored Node info)")
			publicIP = liveInfo.PublicIp
		}
	}
	if publicIP != "" {
		log.Info().
			Str("privateIP", privateIP).
			Str("publicIP", publicIP).
			Msg("Cross-Infra bastion: using publicIP as tunnel target (privateIP may not be routable from bastion)")
		return publicIP
	}
	return privateIP
}

// RunRemoteCommandWithContext executes SSH commands to a Node with context-based timeout and cancellation
// This is the enhanced version that properly propagates context for cancellation support
func RunRemoteCommandWithContext(ctx context.Context, nsId string, infraId string, nodeId string, givenUserName string, cmds []string) (map[int]string, map[int]string, error) {

	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return map[int]string{}, map[int]string{}, fmt.Errorf("operation cancelled before start: %w", ctx.Err())
	default:
	}

	// Get the private IP and SSH port; public IP resolution (for cross-Infra bastions) is
	// deferred until after the bastion node is known (see resolveTargetIpForBastion below).
	_, targetNodeIP, targetSshPort, err := GetNodeIp(nsId, infraId, nodeId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return map[int]string{}, map[int]string{}, err
	}
	targetUserName, targetPrivateKey, err := VerifySshUserName(nsId, infraId, nodeId, targetNodeIP, targetSshPort, givenUserName)
	if err != nil {
		log.Error().Err(err).Msg("")
		return map[int]string{}, map[int]string{}, err
	}

	// Check context again after initial setup
	select {
	case <-ctx.Done():
		return map[int]string{}, map[int]string{}, fmt.Errorf("operation cancelled during setup: %w", ctx.Err())
	default:
	}

	// Set Bastion SSH config (bastionEndpoint, userName, Private Key)
	bastionNodes, err := GetBastionNodes(nsId, infraId, nodeId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return map[int]string{}, map[int]string{}, err
	}

	if len(bastionNodes) == 0 {
		err = fmt.Errorf("no bastion nodes available for VM (ID: %s) in Infra (ID: %s)", nodeId, infraId)
		log.Error().Err(err).Msg("")

		// Assign a Bastion if none (randomly)
		_, err = SetBastionNodes(nsId, infraId, nodeId, "", "", "")
		if err != nil {
			log.Error().Err(err).Msg("no bastion nodes available")
			return map[int]string{}, map[int]string{}, err
		}
		bastionNodes, err = GetBastionNodes(nsId, infraId, nodeId)
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

	// Validate bastion node has valid Node ID
	if bastionNode.NodeId == "" {
		err = fmt.Errorf("bastion node has empty Node ID")
		log.Error().Err(err).Msg("")
		return map[int]string{}, map[int]string{}, err
	}

	// Resolve bastion namespace: fall back to the target's namespace if not set
	bastionNsId := bastionNode.NsId
	if bastionNsId == "" {
		bastionNsId = nsId
	}

	// For cross-Infra/cross-NS bastions the bastion may not be able to route to the target's
	// private network (e.g. OpenStack Neutron). resolveTargetIpForBastion handles this by
	// preferring the public IP (with a live CSP fetch fallback if etcd has no public IP).
	if resolved := resolveTargetIpForBastion(nsId, infraId, nodeId, bastionNode); resolved != "" {
		targetNodeIP = resolved
	}

	// use public IP of the bastion Node
	bastionIp, _, bastionSshPort, err := GetNodeIp(bastionNsId, bastionNode.InfraId, bastionNode.NodeId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return map[int]string{}, map[int]string{}, err
	}

	// Validate bastion IP before proceeding
	if bastionIp == "" {
		err = fmt.Errorf("bastion VM (ID: %s) does not have a public IP address", bastionNode.NodeId)
		log.Error().Err(err).Msg("")
		return map[int]string{}, map[int]string{}, err
	}

	// Validate IP address format
	if net.ParseIP(bastionIp) == nil {
		err = fmt.Errorf("bastion VM (ID: %s) has invalid IP address: %s", bastionNode.NodeId, bastionIp)
		log.Error().Err(err).Msg("")
		return map[int]string{}, map[int]string{}, err
	}

	// SELF-BASTION SHORT-CIRCUIT: when the target VM IS its own bastion, dial
	// it directly. Going through the SSH-jump-to-self path is wasteful, fragile
	// (one transient host-key/auth/sshd-MaxStartups hiccup knocks out *both*
	// the bastion and the target half of the same connection), and obscures
	// real failures behind a "via bastion" error wrap. Compare by full identity
	// (Ns + Infra + Node) — empty bastionNsId is normalised above.
	isSelfBastion := bastionNsId == nsId && bastionNode.InfraId == infraId && bastionNode.NodeId == nodeId

	// BASTION USERNAME RESOLUTION:
	//   - Self-bastion: target == bastion, so reuse the target's resolved
	//     userName/key directly. No separate bastion lookup needed.
	//   - Different VM: the API's req.UserName is for the TARGET — it must NOT
	//     be forwarded as the bastion's user, because the two VMs may have
	//     different SSH users (e.g. bastion=cb-user, target=ubuntu). Passing
	//     givenUserName="default" to a bastion whose stored user is "cb-user"
	//     is exactly what produced "Bastion SSH connection failed … attempted
	//     methods [none publickey]" in production. Pass "" so the bastion
	//     falls back to its own stored userName via GetNodeSshKey.
	var bastionUserName, bastionSshKey string
	if isSelfBastion {
		bastionUserName = targetUserName
		bastionSshKey = targetPrivateKey
	} else {
		bastionUserName, bastionSshKey, err = VerifySshUserName(bastionNsId, bastionNode.InfraId, bastionNode.NodeId, bastionIp, bastionSshPort, "")
		if err != nil {
			log.Error().Err(err).Msg("")
			return map[int]string{}, map[int]string{}, err
		}
	}

	bastionEndpoint := fmt.Sprintf("%s:%d", bastionIp, bastionSshPort)

	// Log bastion connection details for debugging
	log.Debug().
		Str("bastionNodeId", bastionNode.NodeId).
		Str("bastionIp", bastionIp).
		Int("bastionPort", bastionSshPort).
		Str("bastionEndpoint", bastionEndpoint).
		Str("bastionUserName", bastionUserName).
		Bool("selfBastion", isSelfBastion).
		Msg("Bastion connection details")

	bastionSshInfo := model.SshInfo{
		EndPoint:   bastionEndpoint,
		UserName:   bastionUserName,
		PrivateKey: []byte(bastionSshKey),
	}

	log.Debug().Msg("[SSH] " + infraId + "." + nodeId + "(" + targetNodeIP + ")" + " with userName: " + targetUserName)
	for i, v := range cmds {
		log.Debug().Msg("[SSH] cmd[" + fmt.Sprint(i) + "]: " + v)
	}

	// Set Node SSH config (targetEndpoint, userName, Private Key)
	targetEndpoint := fmt.Sprintf("%s:%d", targetNodeIP, targetSshPort)
	targetSshInfo := model.SshInfo{
		EndPoint:   targetEndpoint,
		UserName:   targetUserName,
		PrivateKey: []byte(targetPrivateKey),
	}

	// Set TOFU context for bastion and target VMs
	bastionTofuCtx := tofuContext{
		NsId:    bastionNsId,
		InfraId: bastionNode.InfraId,
		NodeId:  bastionNode.NodeId,
	}
	targetTofuCtx := tofuContext{
		NsId:    nsId,
		InfraId: infraId,
		NodeId:  nodeId,
	}

	// Self-bastion: target VM's private IP is not reachable from this process,
	// but bastionEndpoint IS the same VM's public endpoint. Point the target's
	// SshInfo at the public endpoint so runSSHWithContext can dial it directly
	// (it detects self-bastion via bastionTofuCtx == targetTofuCtx below).
	if isSelfBastion {
		targetSshInfo.EndPoint = bastionEndpoint
		log.Info().
			Str("nodeId", nodeId).
			Str("endpoint", bastionEndpoint).
			Msg("Self-bastion detected — will connect directly (no SSH jump)")
	}

	stdoutResults, stderrResults, err := runSSHWithContext(ctx, bastionSshInfo, targetSshInfo, cmds, bastionTofuCtx, targetTofuCtx)
	if err != nil {
		// Enrich the error log so operators can immediately see WHO failed
		// (bastion vs target identity, endpoints, usernames, mode) without
		// having to grep the surrounding lines for context.
		log.Err(err).
			Str("nsId", nsId).
			Str("infraId", infraId).
			Str("targetNodeId", nodeId).
			Str("targetEndpoint", targetEndpoint).
			Str("targetUserName", targetUserName).
			Str("bastionNodeId", bastionNode.NodeId).
			Str("bastionEndpoint", bastionEndpoint).
			Str("bastionUserName", bastionUserName).
			Bool("selfBastion", isSelfBastion).
			Msg("Error executing commands")
		return stdoutResults, stderrResults, err
	}
	return stdoutResults, stderrResults, nil
}

// RunRemoteCommand is the legacy function for backward compatibility
// It calls RunRemoteCommandWithContext with a background context (no timeout)
// Deprecated: Use RunRemoteCommandWithContext for new implementations
func RunRemoteCommand(nsId string, infraId string, nodeId string, givenUserName string, cmds []string) (map[int]string, map[int]string, error) {
	return RunRemoteCommandWithContext(context.Background(), nsId, infraId, nodeId, givenUserName, cmds)
}

// RunRemoteCommandAsync is func to execute a SSH command to a Node (async call)
func RunRemoteCommandAsync(wg *sync.WaitGroup, nsId string, infraId string, nodeId string, givenUserName string, cmd []string, returnResult *[]model.SshCmdResult) {

	defer wg.Done() //goroutine sync done

	nodeIP, _, _, err := GetNodeIp(nsId, infraId, nodeId)

	sshResultTmp := model.SshCmdResult{}
	sshResultTmp.InfraId = infraId
	sshResultTmp.NodeId = nodeId
	sshResultTmp.NodeIp = nodeIP
	sshResultTmp.Command = make(map[int]string)
	for i, c := range cmd {
		sshResultTmp.Command[i] = c
	}

	if err != nil {
		sshResultTmp.Err = err
		*returnResult = append(*returnResult, sshResultTmp)
		return
	}

	// Check Node status before executing SSH command
	nodeInfo, err := GetNodeObject(nsId, infraId, nodeId)
	if err != nil {
		sshResultTmp.Err = fmt.Errorf("failed to get Node status: %v", err)
		*returnResult = append(*returnResult, sshResultTmp)
		return
	}

	// Validate Node status for SSH execution
	if nodeInfo.Status != model.StatusRunning {
		var errorMsg string
		if nodeInfo.Status == model.StatusTerminated {
			errorMsg = fmt.Sprintf("Node '%s' is in '%s' status. SSH connection is impossible for terminated Nodes", nodeId, nodeInfo.Status)
		} else {
			errorMsg = fmt.Sprintf("Node '%s' is in '%s' status (not Running). Please change the Node status to Running and try again", nodeId, nodeInfo.Status)
		}
		sshResultTmp.Err = fmt.Errorf("%s", errorMsg)
		*returnResult = append(*returnResult, sshResultTmp)
		return
	}

	// RunRemoteCommand
	stdoutResults, stderrResults, err := RunRemoteCommand(nsId, infraId, nodeId, givenUserName, cmd)

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

// RunRemoteCommandAsyncWithStatus is func to execute a SSH command to a Node (async call) with command status tracking
// Deprecated: Use runRemoteCommandWithContextAndStatus instead, which supports context-based cancellation
func RunRemoteCommandAsyncWithStatus(wg *sync.WaitGroup, nsId string, infraId string, nodeId string, givenUserName string, cmd []string, cmdIndex int, returnResult *[]model.SshCmdResult) {

	defer wg.Done() //goroutine sync done

	nodeIP, _, _, err := GetNodeIp(nsId, infraId, nodeId)

	sshResultTmp := model.SshCmdResult{}
	sshResultTmp.InfraId = infraId
	sshResultTmp.NodeId = nodeId
	sshResultTmp.NodeIp = nodeIP
	sshResultTmp.Command = make(map[int]string)
	for i, c := range cmd {
		sshResultTmp.Command[i] = c
	}

	// Update status to Handling
	if cmdIndex > 0 {
		err := UpdateCommandStatusInfo(nsId, infraId, nodeId, cmdIndex, model.CommandStatusHandling, "", "", "", "")
		if err != nil {
			log.Error().Err(err).Int("cmdIndex", cmdIndex).Msg("Failed to update command status to Handling")
		}
	}

	if err != nil {
		sshResultTmp.Err = err
		// Update status to Failed
		if cmdIndex > 0 {
			UpdateCommandStatusInfo(nsId, infraId, nodeId, cmdIndex, model.CommandStatusFailed, "Failed to get Node IP", err.Error(), "", "")
		}
		*returnResult = append(*returnResult, sshResultTmp)
		return
	}

	// Check Node status before executing SSH command
	nodeInfo, err := GetNodeObject(nsId, infraId, nodeId)
	if err != nil {
		sshResultTmp.Err = fmt.Errorf("failed to get Node status: %v", err)
		// Update status to Failed
		if cmdIndex > 0 {
			UpdateCommandStatusInfo(nsId, infraId, nodeId, cmdIndex, model.CommandStatusFailed, "Failed to get Node status", err.Error(), "", "")
		}
		*returnResult = append(*returnResult, sshResultTmp)
		return
	}

	// Validate Node status for SSH execution
	if nodeInfo.Status != model.StatusRunning {
		var errorMsg string
		if nodeInfo.Status == model.StatusTerminated {
			errorMsg = fmt.Sprintf("Node '%s' is in '%s' status. SSH connection is impossible for terminated Nodes", nodeId, nodeInfo.Status)
		} else {
			errorMsg = fmt.Sprintf("Node '%s' is in '%s' status (not Running). Please change the Node status to Running and try again", nodeId, nodeInfo.Status)
		}
		sshResultTmp.Err = fmt.Errorf("%s", errorMsg)
		// Update status to Failed
		if cmdIndex > 0 {
			UpdateCommandStatusInfo(nsId, infraId, nodeId, cmdIndex, model.CommandStatusFailed, "Node not in running status", errorMsg, "", "")
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
		stdout, stderr, err := RunRemoteCommand(nsId, infraId, nodeId, givenUserName, cmd)
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
				var stdoutStr strings.Builder
				stderrStr := ""
				for _, v := range result.stdout {
					stdoutStr.WriteString(v)
					stdoutStr.WriteString("\n")
				}
				for _, v := range result.stderr {
					stderrStr += v + "\n"
				}
				UpdateCommandStatusInfo(nsId, infraId, nodeId, cmdIndex, model.CommandStatusFailed, "Command execution failed", result.err.Error(), stdoutStr.String(), stderrStr)
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
			var stdoutStr strings.Builder
			stderrStr := ""
			for _, v := range result.stdout {
				stdoutStr.WriteString(v)
				stdoutStr.WriteString("\n")
			}
			for _, v := range result.stderr {
				stderrStr += v + "\n"
			}
			UpdateCommandStatusInfo(nsId, infraId, nodeId, cmdIndex, model.CommandStatusCompleted, "Command executed successfully", "", stdoutStr.String(), stderrStr)
		}
		*returnResult = append(*returnResult, sshResultTmp)

	case <-ctx.Done():
		// Command timed out
		timeoutErr := fmt.Errorf("command execution timed out after 30 minutes")
		sshResultTmp.Err = timeoutErr

		// Update status to Timeout
		if cmdIndex > 0 {
			UpdateCommandStatusInfo(nsId, infraId, nodeId, cmdIndex, model.CommandStatusTimeout, "Command execution timed out", timeoutErr.Error(), "", "")
		}

		log.Error().
			Str("nsId", nsId).
			Str("infraId", infraId).
			Str("nodeId", nodeId).
			Int("cmdIndex", cmdIndex).
			Msg("Command execution timed out")

		*returnResult = append(*returnResult, sshResultTmp)
	}
}

// VerifySshUserName is func to verify SSH username
func VerifySshUserName(nsId string, infraId string, nodeId string, nodeIp string, sshPort int, givenUserName string) (string, string, error) {

	// Disable the verification of SSH username (until bastion host is supported)

	// // find vaild username
	// userName, verifiedUserName, privateKey := GetNodeSshKey(nsId, infraId, nodeId)
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
	// 	fmt.Printf("\n[Check SSH] (%s) with userName: %s\n", nodeIp, verifiedUserName)
	// 	_, err := RunRemoteCommand(nodeIp, sshPort, verifiedUserName, privateKey, cmd)
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
	// 		fmt.Printf("[Check SSH] (%s) with userName: %s\n", nodeIp, v)
	// 		_, err := RunRemoteCommand(nodeIp, sshPort, v, privateKey, cmd)
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

	userName, _, privateKey, err := GetNodeSshKey(nsId, infraId, nodeId)
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
	// 	err := UpdateNodeSshKey(nsId, infraId, nodeId, theUserName)
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
	for i := range retrycheck {
		// Fix timeout calculation: start with initialTimeout for first attempt (i=0)
		// then progressively increase for subsequent attempts
		timeout := min(time.Duration(float64(initialTimeout)*(1.0+0.5*float64(i))), maxTimeout)

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

// GetNodeSshKey is func to get Node SshKey. Returns username, verifiedUsername, privateKey
func GetNodeSshKey(nsId string, infraId string, nodeId string) (string, string, string, error) {

	var content struct {
		SshKeyId string `json:"sshKeyId"`
	}

	key := common.GenInfraKey(nsId, infraId, nodeId)

	keyValue, _, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("Cannot find the key from DB. key: %s", key)
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

// UpdateNodeSshKey is func to update Node SshKey
func UpdateNodeSshKey(nsId string, infraId string, nodeId string, verifiedUserName string) error {

	var content struct {
		SshKeyId string `json:"sshKeyId"`
	}

	key := common.GenInfraKey(nsId, infraId, nodeId)
	keyValue, _, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("In UpdateNodeSshKey(); kvstore.GetKv() returned an error.")
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

	bastionNodes, err := GetBastionNodes(nsId, infraId, nodeId)
	if err != nil || len(bastionNodes) == 0 {
		return fmt.Errorf("failed to get bastion nodes: %v", err)
	}

	bastionNode := bastionNodes[0]
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

	bastionNodes, err := GetBastionNodes(nsId, infraId, nodeId)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get bastion nodes: %w", err)
	}
	if len(bastionNodes) == 0 {
		return nil, "", fmt.Errorf("no bastion nodes configured for Infra %s VM %s", infraId, nodeId)
	}

	bastionNode := bastionNodes[0]
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

// SetBastionNodes func sets bastion nodes
func SetBastionNodes(nsId string, infraId string, targetNodeId string, bastionNsId string, bastionInfraId string, bastionNodeId string) (string, error) {

	// Default bastionNsId/bastionInfraId to the target's values when not specified
	if bastionNsId == "" {
		bastionNsId = nsId
	}
	if bastionInfraId == "" {
		bastionInfraId = infraId
	}

	// Check if bastion node already exists for the target VM (for random assignment)
	currentBastion, err := GetBastionNodes(nsId, infraId, targetNodeId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}
	if len(currentBastion) > 0 && bastionNodeId == "" {
		return "", fmt.Errorf("bastion node already exists for VM (ID: %s) in Infra (ID: %s) under namespace (ID: %s)",
			targetNodeId, infraId, nsId)
	}

	nodeObj, err := GetNodeObject(nsId, infraId, targetNodeId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}

	res, err := resource.GetResource(nsId, model.StrVNet, nodeObj.VNetId)
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
		if subnetInfo.Id == nodeObj.SubnetId {

			if bastionNodeId == "" {
				// Auto-select: find a VM with a public IP.
				// For same-Infra, prefer VMs in the same subnet (original behaviour).
				// For cross-Infra/cross-NS, search all VMs in bastionNsId/bastionInfraId.
				isSameInfra := bastionNsId == nsId && bastionInfraId == infraId
				var candidateNodes []string
				var listErr error
				if isSameInfra {
					candidateNodes, listErr = ListNodeByFilter(nsId, infraId, "subnetId", nodeObj.SubnetId)
					if listErr != nil || len(candidateNodes) == 0 {
						// Fall back to all VMs in the Infra if no VM found in the subnet
						candidateNodes, listErr = ListNodeByFilter(nsId, infraId, "", "")
					}
				} else {
					candidateNodes, listErr = ListNodeByFilter(bastionNsId, bastionInfraId, "", "")
				}
				if listErr != nil {
					log.Error().Err(listErr).Msg("")
					return "", fmt.Errorf("failed to list VMs in Infra (ID: %s): %w", bastionInfraId, listErr)
				}

				// Find a VM with public IP to use as bastion
				for _, v := range candidateNodes {
					tmpPublicIp, _, _, err := GetNodeIp(bastionNsId, bastionInfraId, v)
					if err != nil {
						log.Error().Err(err).Msgf("failed to get IP for VM %s", v)
						continue
					}
					if tmpPublicIp != "" {
						bastionNodeId = v
						log.Info().Msgf("Selected VM %s in NS %s / Infra %s as bastion (public IP: %s)", v, bastionNsId, bastionInfraId, tmpPublicIp)
						break
					}
				}

				// If no suitable bastion VM found, return error
				if bastionNodeId == "" {
					return "", fmt.Errorf("no VM with public IP found in NS (ID: %s) Infra (ID: %s) to use as bastion", bastionNsId, bastionInfraId)
				}
			} else {
				// Validate that the specified bastion VM exists in bastionNsId/bastionInfraId
				_, err := GetNodeObject(bastionNsId, bastionInfraId, bastionNodeId)
				if err != nil {
					return "", fmt.Errorf("bastion VM (ID: %s) not found in NS (ID: %s) Infra (ID: %s): %w", bastionNodeId, bastionNsId, bastionInfraId, err)
				}

				// Duplicate check: normalize legacy BastionNode entries that have empty NsId
				// (they were stored before cross-namespace support was added and implicitly
				// belong to the target namespace).
				for _, existingNode := range subnetInfo.BastionNodes {
					effectiveNsId := existingNode.NsId
					if effectiveNsId == "" {
						effectiveNsId = nsId
					}
					if effectiveNsId == bastionNsId && existingNode.InfraId == bastionInfraId && existingNode.NodeId == bastionNodeId {
						return fmt.Sprintf("Bastion (NS: %s, Infra: %s, VM: %s) already exists in subnet (ID: %s) in VNet (ID: %s).",
							bastionNsId, bastionInfraId, bastionNodeId, subnetInfo.Id, nodeObj.VNetId), nil
					}
				}
			}

			bastionCandidate := model.BastionNode{NsId: bastionNsId, InfraId: bastionInfraId, NodeId: bastionNodeId}
			subnetInfo.BastionNodes = append(subnetInfo.BastionNodes, bastionCandidate)
			tempVNetInfo.SubnetInfoList[i] = subnetInfo
			resource.UpdateResourceObject(nsId, model.StrVNet, tempVNetInfo)

			return fmt.Sprintf("Successfully set the bastion (NS: %s, Infra: %s, VM: %s) for subnet (ID: %s) in vNet (ID: %s) for VM (ID: %s) in Infra (ID: %s).",
				bastionNsId, bastionInfraId, bastionNodeId, subnetInfo.Id, nodeObj.VNetId, targetNodeId, infraId), nil
		}
	}
	return "", fmt.Errorf("failed to set bastion. Subnet (ID: %s) not found in VNet (ID: %s) for VM (ID: %s) in Infra (ID: %s) under namespace (ID: %s)",
		nodeObj.SubnetId, nodeObj.VNetId, targetNodeId, infraId, nsId)
}

// RemoveBastionNodes func removes existing bastion nodes info.
// bastionNsId and bastionInfraId narrow the match to a specific bastion identity;
// pass empty strings to match by bastionNodeId alone (legacy / cleanup on VM deletion).
func RemoveBastionNodes(nsId string, infraId string, bastionNsId string, bastionInfraId string, bastionNodeId string) (string, error) {
	resourceListInNs, err := resource.ListResource(nsId, model.StrVNet, "infraId", infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	} else {
		vNets := resourceListInNs.([]model.VNetInfo) // type assertion
		for _, vNet := range vNets {
			removed := false
			for i, subnet := range vNet.SubnetInfoList {
				for j := len(subnet.BastionNodes) - 1; j >= 0; j-- {
					node := subnet.BastionNodes[j]
					if node.NodeId != bastionNodeId {
						continue
					}
					// When bastionNsId/bastionInfraId are provided, also match on them
					// so that two bastions with the same NodeId but different Infras are
					// not accidentally conflated.
					if bastionInfraId != "" {
						effectiveNsId := node.NsId
						if effectiveNsId == "" {
							effectiveNsId = nsId
						}
						effectiveBastionNsId := bastionNsId
						if effectiveBastionNsId == "" {
							effectiveBastionNsId = nsId
						}
						if node.InfraId != bastionInfraId || effectiveNsId != effectiveBastionNsId {
							continue
						}
					}
					subnet.BastionNodes = append(subnet.BastionNodes[:j], subnet.BastionNodes[j+1:]...)
					removed = true
				}
				vNet.SubnetInfoList[i] = subnet
			}
			if removed {
				resource.UpdateResourceObject(nsId, model.StrVNet, vNet)
			}
		}
	}
	return fmt.Sprintf("Successfully removed the bastion (ID: %s) in Infra (ID: %s) from all subnets", bastionNodeId, infraId), nil
}

// GetBastionNodes func retrieves bastion nodes for a given VM
func GetBastionNodes(nsId string, infraId string, targetNodeId string) ([]model.BastionNode, error) {
	returnValue := []model.BastionNode{}
	// Fetch VM object based on nsId, infraId, and targetNodeId
	nodeObj, err := GetNodeObject(nsId, infraId, targetNodeId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return returnValue, err
	}

	// Fetch VNet resource information
	res, err := resource.GetResource(nsId, model.StrVNet, nodeObj.VNetId)
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
		if subnetInfo.Id == nodeObj.SubnetId {
			if subnetInfo.BastionNodes == nil {
				return returnValue, nil
			}
			returnValue = subnetInfo.BastionNodes
			return returnValue, nil
		}
	}

	return returnValue, fmt.Errorf("failed to get bastion in Subnet (ID: %s) of VNet (ID: %s) for VM (ID: %s)",
		nodeObj.SubnetId, nodeObj.VNetId, targetNodeId)
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
func processCommand(command, nsId, infraId, nodeId string, nodeIndex int) (string, error) {
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
			targetInfraId := infraId
			targetNodeId := nodeId
			if val, ok := params["target"]; ok {
				parts := strings.Split(val, ".")
				if len(parts) == 2 {
					targetInfraId = parts[0]
					targetNodeId = parts[1]
					if targetInfraId == "this" {
						targetInfraId = infraId
					}
					if targetNodeId == "this" {
						targetNodeId = nodeId
					}
					// if targetNode or targetInfra is not specified, return error
					if targetInfraId == "" || targetNodeId == "" {
						return "", fmt.Errorf("built-in function %s error: target Infra or VM %s is invalid", funcName, val)
					}

				} else if strings.EqualFold(val, "this") {
					targetInfraId = infraId
					targetNodeId = nodeId
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
				replacement, err = replaceWithPublicIP(nsId, targetInfraId, targetNodeId, prefix, postfix)
			} else {
				// Logic for GetPrivateIP function
				replacement, err = replaceWithPrivateIP(nsId, targetInfraId, targetNodeId, prefix, postfix)
			}
			if err != nil {
				return "", fmt.Errorf("built-in function GetPublicIP error: %s", err.Error())
			}
		} else if strings.EqualFold(funcName, "GetPublicIPs") || strings.EqualFold(funcName, "GetPrivateIPs") {
			// Logic for GetPublicIPs/GetPrivateIPs function
			// Supports optional "label" parameter for filtering VMs by label selector
			// Example: $$Func(GetPublicIPs(separator=' ', label='accelerator=gpu'))
			targetInfraId := infraId
			if val, ok := params["target"]; ok {
				if strings.EqualFold(val, "this") {
					targetInfraId = infraId
				} else {
					targetInfraId = val
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
			labelSelector := ""
			if lbl, ok := params["label"]; ok {
				labelSelector = lbl
			}
			if strings.EqualFold(funcName, "GetPublicIPs") {
				replacement, err = replaceWithPublicIPs(nsId, targetInfraId, separator, prefix, postfix, labelSelector)
			} else {
				replacement, err = replaceWithPrivateIPs(nsId, targetInfraId, separator, prefix, postfix, labelSelector)
			}
			if err != nil {
				return "", fmt.Errorf("built-in function %s error: %s", funcName, err.Error())
			}
		} else if strings.EqualFold(funcName, "AssignTask") {
			// Logic for AssignTask function
			taskListParam, ok := params["task"]
			if !ok {
				return "", fmt.Errorf("built-in function AssignTask error: no task list provided")
			}
			tasks := splitParams(taskListParam)
			replacement = tasks[nodeIndex%len(tasks)]
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
		} else if strings.EqualFold(funcName, "GetInfraId") {
			// Logic for getInfraId function
			prefix := ""
			if pre, ok := params["prefix"]; ok {
				prefix = pre
			}
			postfix := ""
			if post, ok := params["postfix"]; ok {
				postfix = post
			}
			replacement = replaceWithId(infraId, prefix, postfix)
		} else if strings.EqualFold(funcName, "GetNodeId") {
			// Logic for getNodeId function
			prefix := ""
			if pre, ok := params["prefix"]; ok {
				prefix = pre
			}
			postfix := ""
			if post, ok := params["postfix"]; ok {
				postfix = post
			}
			replacement = replaceWithId(nodeId, prefix, postfix)
		} else if strings.EqualFold(funcName, "GetLocationDisplay") ||
			strings.EqualFold(funcName, "GetLocationLatitude") ||
			strings.EqualFold(funcName, "GetLocationLongitude") {
			// Logic for GetLocationDisplay, GetLocationLatitude, GetLocationLongitude functions
			// These return the location info (display name, latitude, longitude) of the target VM.
			// Example: $$Func(GetLocationDisplay(target=this.this))
			// Example: $$Func(GetLocationLatitude())
			// Example: $$Func(GetLocationLongitude(prefix='--longitude '))
			targetInfraId := infraId
			targetNodeId := nodeId
			if val, ok := params["target"]; ok {
				val = strings.TrimSpace(val)
				if val != "" {
					parts := strings.Split(val, ".")
					if len(parts) == 2 {
						targetInfraId = parts[0]
						targetNodeId = parts[1]
						if targetInfraId == "this" {
							targetInfraId = infraId
						}
						if targetNodeId == "this" {
							targetNodeId = nodeId
						}
						if targetInfraId == "" || targetNodeId == "" {
							return "", fmt.Errorf("built-in function %s error: target Infra or VM %s is invalid", funcName, val)
						}
					} else if strings.EqualFold(val, "this") {
						targetInfraId = infraId
						targetNodeId = nodeId
					} else {
						return "", fmt.Errorf("built-in function %s error: target %q has invalid format; expected \"this\" or \"infraId.nodeId\"", funcName, val)
					}
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
			loc, locErr := replaceWithLocation(nsId, targetInfraId, targetNodeId)
			if locErr != nil {
				return "", fmt.Errorf("built-in function %s error: %s", funcName, locErr.Error())
			}
			if strings.EqualFold(funcName, "GetLocationDisplay") {
				replacement = prefix + loc.Display + postfix
			} else if strings.EqualFold(funcName, "GetLocationLatitude") {
				replacement = prefix + fmt.Sprintf("%g", loc.Latitude) + postfix
			} else {
				replacement = prefix + fmt.Sprintf("%g", loc.Longitude) + postfix
			}
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
func replaceWithPublicIP(nsId, infraId, nodeId, prefix, postfix string) (string, error) {
	nodeStatus, err := GetNodeCurrentPublicIp(nsId, infraId, nodeId)
	if err != nil {
		return "", err
	}
	ip := nodeStatus.PublicIp
	return prefix + ip + postfix, err
}

// replaceWithPrivateIP function to get and replace string with the private IP of the target
func replaceWithPrivateIP(nsId, infraId, nodeId, prefix, postfix string) (string, error) {
	nodeStatus, err := GetNodeCurrentPublicIp(nsId, infraId, nodeId)
	if err != nil {
		return "", err
	}
	ip := nodeStatus.PrivateIp
	return prefix + ip + postfix, err
}

// replaceWithPublicIPs returns the public IP list of VMs in the target Infra.
// If labelSelector is non-empty, only VMs matching the label selector are included.
// Example labelSelector: "accelerator=gpu" or "role=worker,env=prod"
func replaceWithPublicIPs(nsId, infraId, separator, prefix, postfix, labelSelector string) (string, error) {
	infraStatus, err := GetInfraStatus(nsId, infraId)
	if err != nil {
		return "", err
	}

	// If labelSelector is specified, filter VMs by label
	if labelSelector != "" {
		filteredNodeIds, err := getNodeIdsByLabel(nsId, infraId, labelSelector)
		if err != nil {
			return "", fmt.Errorf("label filtering failed: %w", err)
		}
		if len(filteredNodeIds) == 0 {
			log.Warn().Str("labelSelector", labelSelector).Msg("GetPublicIPs: no Nodes matched the label selector")
			return "", nil
		}
		allowedIds := make(map[string]bool, len(filteredNodeIds))
		for _, id := range filteredNodeIds {
			allowedIds[id] = true
		}
		var ips []string
		for _, nodeStatus := range infraStatus.Node {
			if allowedIds[nodeStatus.Id] {
				ips = append(ips, prefix+nodeStatus.PublicIp+postfix)
			}
		}
		return strings.Join(ips, separator), nil
	}

	ips := make([]string, len(infraStatus.Node))
	for i, nodeStatus := range infraStatus.Node {
		ips[i] = prefix + nodeStatus.PublicIp + postfix
	}
	return strings.Join(ips, separator), nil
}

// replaceWithPrivateIPs returns the private IP list of VMs in the target Infra.
// If labelSelector is non-empty, only VMs matching the label selector are included.
func replaceWithPrivateIPs(nsId, infraId, separator, prefix, postfix, labelSelector string) (string, error) {
	infraStatus, err := GetInfraStatus(nsId, infraId)
	if err != nil {
		return "", err
	}

	// If labelSelector is specified, filter VMs by label
	if labelSelector != "" {
		filteredNodeIds, err := getNodeIdsByLabel(nsId, infraId, labelSelector)
		if err != nil {
			return "", fmt.Errorf("label filtering failed: %w", err)
		}
		if len(filteredNodeIds) == 0 {
			log.Warn().Str("labelSelector", labelSelector).Msg("GetPrivateIPs: no Nodes matched the label selector")
			return "", nil
		}
		allowedIds := make(map[string]bool, len(filteredNodeIds))
		for _, id := range filteredNodeIds {
			allowedIds[id] = true
		}
		var ips []string
		for _, nodeStatus := range infraStatus.Node {
			if allowedIds[nodeStatus.Id] {
				ips = append(ips, prefix+nodeStatus.PrivateIp+postfix)
			}
		}
		return strings.Join(ips, separator), nil
	}

	ips := make([]string, len(infraStatus.Node))
	for i, nodeStatus := range infraStatus.Node {
		ips[i] = prefix + nodeStatus.PrivateIp + postfix
	}
	return strings.Join(ips, separator), nil
}

// getNodeIdsByLabel returns VM IDs in an Infra that match the given label selector.
// It automatically prepends system label conditions (sys.namespace, sys.infraId) for scoping.
func getNodeIdsByLabel(nsId, infraId, labelSelector string) ([]string, error) {
	// Add system label conditions to scope within the namespace and Infra
	combinedSelector := fmt.Sprintf("%s=%s,%s=%s,%s", model.LabelNamespace, nsId, model.LabelInfraId, infraId, labelSelector)

	log.Debug().Str("combinedLabelSelector", combinedSelector).Msg("GetIPs: filtering VMs by label")

	matchedResources, err := label.GetResourcesByLabelSelector(model.StrNode, combinedSelector)
	if err != nil {
		return nil, err
	}

	nodeIds := make([]string, 0, len(matchedResources))
	for _, resource := range matchedResources {
		if nodeInfo, ok := resource.(*model.NodeInfo); ok {
			nodeIds = append(nodeIds, nodeInfo.Id)
		}
	}

	log.Debug().Int("matchedCount", len(nodeIds)).Str("labelSelector", labelSelector).Msg("GetIPs: VMs matched by label")
	return nodeIds, nil
}

// replaceWithId function to replace string with the prefix and postfix
func replaceWithId(id, prefix, postfix string) string {
	return prefix + id + postfix
}

// replaceWithLocation returns the Location of the target VM
func replaceWithLocation(nsId, infraId, nodeId string) (model.Location, error) {
	nodeInfo, err := GetNodeObject(nsId, infraId, nodeId)
	if err != nil {
		return model.Location{}, err
	}
	return nodeInfo.Location, nil
}

// Command Status Management Functions

// updateNodeCommandStatusSafe safely updates only CommandStatus field of VM with proper locking
func updateNodeCommandStatusSafe(nsId, infraId, nodeId string, updateFunc func(*[]model.CommandStatusInfo) error) error {
	// Use the same mutex as UpdateNodeInfo for consistency
	key := common.GenInfraKey(nsId, infraId, nodeId)

	// Retry mechanism for concurrent access
	maxRetries := 3
	for attempt := range maxRetries {
		// Get current Node info
		keyValue, exists, err := kvstore.GetKv(key)
		if !exists || err != nil {
			return fmt.Errorf("failed to get Node info: %v", err)
		}

		nodeInfo := model.NodeInfo{}
		err = json.Unmarshal([]byte(keyValue.Value), &nodeInfo)
		if err != nil {
			return fmt.Errorf("failed to unmarshal VM info: %v", err)
		}

		// Apply the update function to CommandStatus
		originalCommandStatus := make([]model.CommandStatusInfo, len(nodeInfo.CommandStatus))
		copy(originalCommandStatus, nodeInfo.CommandStatus)

		err = updateFunc(&nodeInfo.CommandStatus)
		if err != nil {
			return err
		}

		// Only update if CommandStatus actually changed
		if reflect.DeepEqual(originalCommandStatus, nodeInfo.CommandStatus) {
			return nil // No change needed
		}

		// Atomic update
		nodeJson, err := json.Marshal(nodeInfo)
		if err != nil {
			return fmt.Errorf("failed to marshal VM info: %v", err)
		}

		err = kvstore.Put(key, string(nodeJson))
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

// isTerminalCommandStatus reports whether a command execution status is a
// final state (as opposed to Queued/Handling), i.e. the attempt is over and
// its outcome (ResultSummary/ErrorMessage) will not change further.
func isTerminalCommandStatus(status model.CommandExecutionStatus) bool {
	switch status {
	case model.CommandStatusCompleted,
		model.CommandStatusCompletedWithError,
		model.CommandStatusFailed,
		model.CommandStatusTimeout,
		model.CommandStatusCancelled,
		model.CommandStatusInterrupted:
		return true
	default:
		return false
	}
}

// mergeCommandStatusRepeat checks whether the just-finalized record at
// curIdx is an exact repeat of the immediately preceding record's terminal
// outcome (same CommandRequested, Status, ResultSummary, and ErrorMessage;
// Stdout/Stderr are intentionally excluded from the comparison since they
// may embed timestamps or other per-run noise that would otherwise defeat
// the match). If it is, curIdx is merged into the preceding record (bumping
// RepeatCount, refreshing LastOccurredTime/Stdout/Stderr/XRequestId) and
// removed from commandStatus, and the merged record is returned. If it is
// not a repeat, commandStatus is left untouched and ok is false.
func mergeCommandStatusRepeat(commandStatus *[]model.CommandStatusInfo, curIdx int, now time.Time) (merged *model.CommandStatusInfo, ok bool) {
	cur := (*commandStatus)[curIdx]
	if curIdx == 0 || !isTerminalCommandStatus(cur.Status) {
		return nil, false
	}

	prev := &(*commandStatus)[curIdx-1]
	if !isTerminalCommandStatus(prev.Status) ||
		prev.CommandRequested != cur.CommandRequested ||
		prev.Status != cur.Status ||
		prev.ResultSummary != cur.ResultSummary ||
		prev.ErrorMessage != cur.ErrorMessage {
		return nil, false
	}

	if prev.RepeatCount == 0 {
		prev.RepeatCount = 2 // the original occurrence plus this repeat
	} else {
		prev.RepeatCount++
	}
	prev.LastOccurredTime = now.Format(time.RFC3339)
	prev.ElapsedTime = cur.ElapsedTime
	prev.Stdout = cur.Stdout
	prev.Stderr = cur.Stderr
	prev.XRequestId = cur.XRequestId

	mergedCopy := *prev
	*commandStatus = append((*commandStatus)[:curIdx], (*commandStatus)[curIdx+1:]...)
	return &mergedCopy, true
}

// trimCommandStatusHistory drops the oldest records once commandStatus grows
// past limit, keeping only the most recent ones. This is a backstop for VMs
// with genuinely varied command history; identical repeats are instead
// merged (not appended) by mergeCommandStatusRepeat.
func trimCommandStatusHistory(commandStatus *[]model.CommandStatusInfo, limit int) {
	if len(*commandStatus) > limit {
		*commandStatus = (*commandStatus)[len(*commandStatus)-limit:]
	}
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
			found := slices.Contains(filter.Status, cmd.Status)
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

	end := min(offset+limit, len(commandStatus))

	return commandStatus[offset:end]
}

// AddCommandStatusInfo adds a new command status record to VM's command history
func AddCommandStatusInfo(nsId, infraId, nodeId, xRequestId, commandRequested, commandExecuted string) (int, error) {
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}
	err = common.CheckString(infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}
	err = common.CheckString(nodeId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}

	var nextIndex int

	err = updateNodeCommandStatusSafe(nsId, infraId, nodeId, func(commandStatus *[]model.CommandStatusInfo) error {
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
		trimCommandStatusHistory(commandStatus, commandStatusHistoryLimit)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}

	// Publish CommandStatus event for newly queued command
	if xRequestId != "" {
		PublishCommandEvent(xRequestId, model.CommandStreamEvent{
			Type:         model.EventCommandStatus,
			NodeId:       nodeId,
			CommandIndex: nextIndex,
			Timestamp:    time.Now().Format(time.RFC3339Nano),
			Status: &model.CommandStatusInfo{
				Index:            nextIndex,
				XRequestId:       xRequestId,
				CommandRequested: commandRequested,
				CommandExecuted:  commandExecuted,
				Status:           model.CommandStatusQueued,
				StartedTime:      time.Now().Format(time.RFC3339),
			},
		})
	}

	log.Info().
		Str("nsId", nsId).
		Str("infraId", infraId).
		Str("nodeId", nodeId).
		Int("index", nextIndex).
		Str("xRequestId", xRequestId).
		Msg("Command status added")

	return nextIndex, nil
}

// UpdateCommandStatusInfo updates an existing command status record
func UpdateCommandStatusInfo(nsId, infraId, nodeId string, index int, status model.CommandExecutionStatus, resultSummary, errorMessage, stdout, stderr string) error {
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	err = common.CheckString(infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	err = common.CheckString(nodeId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	// Track the xRequestId and updated status for SSE publishing
	var updatedXRequestId string
	var updatedStatusInfo *model.CommandStatusInfo
	publishIndex := index

	err = updateNodeCommandStatusSafe(nsId, infraId, nodeId, func(commandStatus *[]model.CommandStatusInfo) error {
		// Find the command status by index using helper function
		cmdStatus, cmdIndex := findCommandByIndex(*commandStatus, index)
		if cmdStatus == nil {
			return fmt.Errorf("command with index %d not found for VM (ID: %s)", index, nodeId)
		}

		// Capture xRequestId for SSE publishing
		updatedXRequestId = cmdStatus.XRequestId

		// Update status and completion time
		startTime, _ := time.Parse(time.RFC3339, cmdStatus.StartedTime)
		currentTime := time.Now()

		(*commandStatus)[cmdIndex].Status = status

		// Only set CompletedTime for final states (terminal). CompletedWithError
		// is included so UI / accounting sees a real finish time even when the
		// command exited non-zero — the SSH session DID complete.
		if status == model.CommandStatusCompleted ||
			status == model.CommandStatusCompletedWithError ||
			status == model.CommandStatusFailed ||
			status == model.CommandStatusTimeout {
			(*commandStatus)[cmdIndex].CompletedTime = currentTime.Format(time.RFC3339)
		}

		// Calculate elapsed time in seconds (not milliseconds)
		(*commandStatus)[cmdIndex].ElapsedTime = int64(currentTime.Sub(startTime).Seconds())
		(*commandStatus)[cmdIndex].ResultSummary = resultSummary
		(*commandStatus)[cmdIndex].ErrorMessage = errorMessage

		// Truncate output if too long (limit to 100000 bytes for history)
		if len(stdout) > 100000 {
			(*commandStatus)[cmdIndex].Stdout = stdout[:100000] + "...(truncated)"
		} else {
			(*commandStatus)[cmdIndex].Stdout = stdout
		}

		if len(stderr) > 100000 {
			(*commandStatus)[cmdIndex].Stderr = stderr[:100000] + "...(truncated)"
		} else {
			(*commandStatus)[cmdIndex].Stderr = stderr
		}

		// Merge into the immediately preceding record when it is an exact
		// repeat of the same terminal outcome, instead of appending a new
		// record. This keeps retry storms (e.g. a failing install script
		// retried repeatedly) from growing this VM's history unbounded.
		if mergedInfo, ok := mergeCommandStatusRepeat(commandStatus, cmdIndex, currentTime); ok {
			updatedStatusInfo = mergedInfo
			publishIndex = mergedInfo.Index
			return nil
		}

		// Capture a copy of the updated status for SSE publishing
		statusCopy := (*commandStatus)[cmdIndex]
		updatedStatusInfo = &statusCopy

		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	// Publish CommandStatus event to SSE subscribers (non-blocking, no-op if no session exists)
	if updatedXRequestId != "" && updatedStatusInfo != nil {
		PublishCommandEvent(updatedXRequestId, model.CommandStreamEvent{
			Type:         model.EventCommandStatus,
			NodeId:       nodeId,
			CommandIndex: publishIndex,
			Timestamp:    time.Now().Format(time.RFC3339Nano),
			Status:       updatedStatusInfo,
		})
	}

	log.Info().
		Str("nsId", nsId).
		Str("infraId", infraId).
		Str("nodeId", nodeId).
		Int("index", publishIndex).
		Str("status", string(status)).
		Msg("Command status updated")

	return nil
}

// GetCommandStatusInfo retrieves a specific command status record
func GetCommandStatusInfo(nsId, infraId, nodeId string, index int) (*model.CommandStatusInfo, error) {
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	err = common.CheckString(infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	err = common.CheckString(nodeId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	// Use existing GetNodeObject function instead of direct kvstore access
	nodeInfo, err := GetNodeObject(nsId, infraId, nodeId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	// Find the command status by index using helper function
	cmdStatus, _ := findCommandByIndex(nodeInfo.CommandStatus, index)
	if cmdStatus == nil {
		return nil, fmt.Errorf("command with index %d not found for VM (ID: %s)", index, nodeId)
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
func ListCommandStatusInfo(nsId, infraId, nodeId string, filter *model.CommandStatusFilter) (*model.CommandStatusListResponse, error) {
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	err = common.CheckString(infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	err = common.CheckString(nodeId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	// Use existing GetNodeObject function instead of direct kvstore access
	nodeInfo, err := GetNodeObject(nsId, infraId, nodeId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	// Apply filters using helper function
	filteredCommands := filterCommands(nodeInfo.CommandStatus, filter)
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
func DeleteCommandStatusInfo(nsId, infraId, nodeId string, index int) error {
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	err = common.CheckString(infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	err = common.CheckString(nodeId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	err = updateNodeCommandStatusSafe(nsId, infraId, nodeId, func(commandStatus *[]model.CommandStatusInfo) error {
		// Find and remove the command status by index
		_, cmdIndex := findCommandByIndex(*commandStatus, index)
		if cmdIndex == -1 {
			return fmt.Errorf("command with index %d not found for VM (ID: %s)", index, nodeId)
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
		Str("infraId", infraId).
		Str("nodeId", nodeId).
		Int("index", index).
		Msg("Command status deleted")

	return nil
}

// DeleteCommandStatusInfoByCriteria deletes multiple command status records by criteria
func DeleteCommandStatusInfoByCriteria(nsId, infraId, nodeId string, filter *model.CommandStatusFilter) (int, error) {
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}
	err = common.CheckString(infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}
	err = common.CheckString(nodeId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}

	var deleteCount int

	err = updateNodeCommandStatusSafe(nsId, infraId, nodeId, func(commandStatus *[]model.CommandStatusInfo) error {
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
		Str("infraId", infraId).
		Str("nodeId", nodeId).
		Int("deleteCount", deleteCount).
		Msg("Command statuses deleted by criteria")

	return deleteCount, nil
}

// ClearAllCommandStatusInfo deletes all command status records for a VM
func ClearAllCommandStatusInfo(nsId, infraId, nodeId string) (int, error) {
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}
	err = common.CheckString(infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}
	err = common.CheckString(nodeId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}

	var clearCount int

	err = updateNodeCommandStatusSafe(nsId, infraId, nodeId, func(commandStatus *[]model.CommandStatusInfo) error {
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
		Str("infraId", infraId).
		Str("nodeId", nodeId).
		Int("clearCount", clearCount).
		Msg("All command statuses cleared")

	return clearCount, nil
}

// GetHandlingCommandCount returns the count of currently handling commands for a VM
// This function is optimized for frequent polling and avoids unnecessary processing
func GetHandlingCommandCount(nsId, infraId, nodeId string) (int, error) {
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}
	err = common.CheckString(infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}
	err = common.CheckString(nodeId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}

	// Use existing GetNodeObject function - optimized for performance
	nodeInfo, err := GetNodeObject(nsId, infraId, nodeId)
	if err != nil {
		// Don't log errors for frequent polling calls to reduce noise
		return 0, err
	}

	// Count handling commands efficiently
	handlingCount := 0
	for _, cmdStatus := range nodeInfo.CommandStatus {
		if cmdStatus.Status == model.CommandStatusHandling {
			handlingCount++
		}
	}

	return handlingCount, nil
}

// GetInfraHandlingCommandCount returns the count of currently handling commands across all VMs in an Infra
// This function is optimized for Infra-level monitoring
func GetInfraHandlingCommandCount(nsId, infraId string) (map[string]int, int, error) {
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, 0, err
	}
	err = common.CheckString(infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, 0, err
	}

	// Get VM list
	nodeList, err := ListNodeId(nsId, infraId)
	if err != nil {
		return nil, 0, err
	}

	nodeHandlingCounts := make(map[string]int)
	totalHandlingCount := 0

	// Process each VM's handling commands
	for _, nodeId := range nodeList {
		handlingCount, err := GetHandlingCommandCount(nsId, infraId, nodeId)
		if err != nil {
			// Continue processing other VMs even if one fails
			log.Debug().Err(err).Msgf("Failed to get handling count for VM %s", nodeId)
			nodeHandlingCounts[nodeId] = 0
			continue
		}

		nodeHandlingCounts[nodeId] = handlingCount
		totalHandlingCount += handlingCount
	}

	return nodeHandlingCounts, totalHandlingCount, nil
}

// CleanupInterruptedCommands marks all "Handling" or "Queued" commands as "Interrupted"
// This should be called during system startup to handle commands that were
// interrupted by a system restart while SSH sessions were still active
func CleanupInterruptedCommands() error {
	log.Info().Msg("Starting cleanup of interrupted commands...")

	// Get all namespaces
	nsList, err := common.ListNsId()
	if err != nil {
		log.Error().Err(err).Msg("Failed to list namespaces for cleanup")
		return err
	}

	totalInterrupted := 0

	for _, nsId := range nsList {
		// Get all Infras in namespace
		infraList, err := ListInfraId(nsId)
		if err != nil {
			log.Debug().Err(err).Str("nsId", nsId).Msg("Failed to list Infras")
			continue
		}

		for _, infraId := range infraList {
			// Get all VMs in Infra
			nodeList, err := ListNodeId(nsId, infraId)
			if err != nil {
				log.Debug().Err(err).Str("infraId", infraId).Msg("Failed to list VMs")
				continue
			}

			for _, nodeId := range nodeList {
				count, err := cleanupNodeInterruptedCommands(nsId, infraId, nodeId)
				if err != nil {
					log.Debug().Err(err).
						Str("nodeId", nodeId).
						Msg("Failed to cleanup interrupted commands for VM")
					continue
				}
				totalInterrupted += count
			}
		}
	}

	if totalInterrupted > 0 {
		log.Info().
			Int("totalInterrupted", totalInterrupted).
			Msg("Cleanup completed: marked interrupted commands")
	} else {
		log.Info().Msg("Cleanup completed: no interrupted commands found")
	}

	return nil
}

// cleanupNodeInterruptedCommands marks Handling/Queued commands as Interrupted for a specific Node
func cleanupNodeInterruptedCommands(nsId, infraId, nodeId string) (int, error) {
	interruptedCount := 0

	err := updateNodeCommandStatusSafe(nsId, infraId, nodeId, func(commandStatus *[]model.CommandStatusInfo) error {
		now := time.Now()
		for i := range *commandStatus {
			cmd := &(*commandStatus)[i]
			// Mark Handling or Queued commands as Interrupted
			if cmd.Status == model.CommandStatusHandling || cmd.Status == model.CommandStatusQueued {
				originalStatus := cmd.Status // Save before changing
				cmd.Status = model.CommandStatusInterrupted
				cmd.CompletedTime = now.Format(time.RFC3339)
				cmd.ErrorMessage = "Command was interrupted by system restart"

				// Calculate elapsed time if started (in seconds)
				if cmd.StartedTime != "" {
					startTime, err := time.Parse(time.RFC3339, cmd.StartedTime)
					if err == nil {
						cmd.ElapsedTime = int64(now.Sub(startTime).Seconds())
					}
				}

				interruptedCount++
				log.Debug().
					Str("nodeId", nodeId).
					Int("index", cmd.Index).
					Str("originalStatus", string(originalStatus)).
					Msg("Marked command as interrupted")
			}
		}
		return nil
	})

	return interruptedCount, err
}

// GetInfraActiveCommands returns command execution tasks for an Infra
// Each VM's command is returned as a separate task for individual tracking and cancellation
func GetInfraActiveCommands(nsId, infraId string, statusFilter []model.CommandExecutionStatus) (*model.ExecutionTaskListResponse, error) {
	if nsId != "" {
		err := common.CheckString(nsId)
		if err != nil {
			return nil, err
		}
	}
	if infraId != "" {
		err := common.CheckString(infraId)
		if err != nil {
			return nil, err
		}
	}

	response := &model.ExecutionTaskListResponse{
		Tasks: []model.ExecutionTask{},
	}

	// Get namespaces to scan
	var nsList []string
	if nsId != "" {
		nsList = []string{nsId}
	} else {
		var err error
		nsList, err = common.ListNsId()
		if err != nil {
			return nil, err
		}
	}

	// statusFilter can be nil/empty to return all statuses

	for _, ns := range nsList {
		// Get Infras to scan
		var infraList []string
		if infraId != "" {
			infraList = []string{infraId}
		} else {
			var err error
			infraList, err = ListInfraId(ns)
			if err != nil {
				continue
			}
		}

		for _, infra := range infraList {
			nodeList, err := ListNodeId(ns, infra)
			if err != nil {
				continue
			}

			for _, nodeId := range nodeList {
				// Get command status for this VM
				commandList, err := ListCommandStatusInfo(ns, infra, nodeId, &model.CommandStatusFilter{
					Status: statusFilter,
				})
				if err != nil {
					continue
				}

				// Create individual task for each VM's command
				for _, cmd := range commandList.Commands {
					task := model.ExecutionTask{
						TaskId:          fmt.Sprintf("%s:%s:%d", cmd.XRequestId, nodeId, cmd.Index), // Unique per VM
						XRequestId:      cmd.XRequestId,
						NsId:            ns,
						InfraId:         infra,
						NodeId:          nodeId,
						CommandIndex:    cmd.Index,
						Command:         []string{cmd.CommandRequested},
						Status:          cmd.Status,
						StartedAt:       cmd.StartedTime,
						CompletedAt:     cmd.CompletedTime,
						ElapsedSeconds:  cmd.ElapsedTime, // Already in seconds
						Message:         cmd.ResultSummary,
						TargetNodeCount: 1,
						CompletedNodeCount: func() int {
							if isTerminalStatus(cmd.Status) {
								return 1
							}
							return 0
						}(),
					}
					response.Tasks = append(response.Tasks, task)
				}
			}
		}
	}

	response.Total = len(response.Tasks)
	return response, nil
}

// isTerminalStatus returns true if the status represents a terminal (finished) state
func isTerminalStatus(status model.CommandExecutionStatus) bool {
	switch status {
	case model.CommandStatusCompleted, model.CommandStatusCompletedWithError,
		model.CommandStatusFailed, model.CommandStatusTimeout,
		model.CommandStatusCancelled, model.CommandStatusInterrupted:
		return true
	default:
		return false
	}
}

// CancelInfraCommand cancels a running command by updating its status to Cancelled
// It also attempts to cancel the in-memory task if still running
// If nodeId is provided, cancels only that specific VM's command
// If nodeId is empty, cancels all VMs with the given xRequestId
func CancelInfraCommand(nsId, infraId, nodeId, xRequestId string, index int, reason string) (*model.CancelTaskResponse, error) {
	err := common.CheckString(nsId)
	if err != nil {
		return nil, err
	}
	err = common.CheckString(infraId)
	if err != nil {
		return nil, err
	}

	response := &model.CancelTaskResponse{
		TaskId:      fmt.Sprintf("%s:%s:%d", xRequestId, nodeId, index),
		CancelledAt: time.Now().Format(time.RFC3339),
	}

	// Update the command status in VM info
	err = UpdateCommandStatusInfo(nsId, infraId, nodeId, index,
		model.CommandStatusCancelled,
		"Cancelled by user request",
		fmt.Sprintf("Cancellation reason: %s", reason),
		"", "")
	if err != nil {
		response.Success = false
		response.Message = fmt.Sprintf("Failed to update command status: %v", err)
		return response, err
	}

	// Cancel the in-memory context for this specific VM if exists
	if xRequestId != "" && nodeId != "" {
		cancelByKey(xRequestId, nodeId)
	}

	response.Success = true
	response.Status = model.CommandStatusCancelled
	response.Message = "Command cancelled successfully"
	return response, nil
}
