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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	validator "github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/ssh"
)

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

// producedRemoteOutput reports whether the remote side sent anything back, i.e.
// the command had begun executing before the transport failed. Used to decide
// whether re-running it would be safe.
func producedRemoteOutput(stdout, stderr map[int]string) bool {
	for _, v := range stdout {
		if v != "" {
			return true
		}
	}
	for _, v := range stderr {
		if v != "" {
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
			// Use the same selection as the execution path (pickBastion) so
			// this dependency accounting names the bastion the target will
			// actually tunnel through when several are registered.
			bastionId := pickBastion(bs, nsId, infraId, n).NodeId
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

	// Set Bastion SSH config (bastionEndpoint, userName, Private Key).
	// GetUsableBastionNodes drops a stale (non-Running) bastion and
	// auto-selects a fresh one when needed.
	bastionNodes, err := GetUsableBastionNodes(nsId, infraId, nodeId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return map[int]string{}, map[int]string{}, err
	}

	// Spread load across the subnet's bastions when more than one is
	// registered; deterministic per target so retries reuse the same hop.
	bastionNode := pickBastion(bastionNodes, nsId, infraId, nodeId)

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
