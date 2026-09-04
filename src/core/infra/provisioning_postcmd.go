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
	"fmt"
	"strings"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/rs/zerolog/log"
)

// executePostCommands runs post-deployment command phases against the infra,
// aggregates per-node outcomes, persists status/results, and logs accurately.
// Shared by creation, autopilot, autopilot scale-out, and nodeGroup addition.
//
// defaultNodeGroupId scopes phases that carry no explicit target (used when
// bootstrapping a newly added nodeGroup); "" means all nodes.
func executePostCommands(nsId, infraId, defaultNodeGroupId string, phases []model.PostCommandReq, xRequestId string) (model.PostCommandStatus, error) {
	phases = normalizePostCommandPhases(phases)
	if len(phases) == 0 {
		return model.PostCommandStatusNone, nil
	}
	startedAt := time.Now()

	// SSH readiness gate: fresh nodes often refuse SSH for a short while
	// (cloud-init). Proceed early when reachable; on timeout run anyway so
	// genuine auth/config errors are reported per node rather than hidden.
	waitForSshReadiness(nsId, infraId, defaultNodeGroupId)

	phaseResults := make([]model.PostCommandPhaseResult, 0, len(phases))
	overall := model.PostCommandStatusCompleted
	stopped := false
	var firstErr error

	for i := range phases {
		phase := phases[i]
		phaseResult := model.PostCommandPhaseResult{
			Phase:  i + 1,
			Target: phase.Target(),
		}

		if stopped {
			phaseResult.Status = model.PostCommandStatusSkipped
			phaseResults = append(phaseResults, phaseResult)
			continue
		}

		nodeGroupId := phase.NodeGroupId
		if nodeGroupId == "" && phase.NodeId == "" && phase.LabelSelector == "" {
			nodeGroupId = defaultNodeGroupId
			if nodeGroupId != "" {
				phaseResult.Target = "nodeGroupId=" + nodeGroupId
			}
		}

		log.Info().Msgf("Post-deployment phase %d/%d (%s): %v", i+1, len(phases), phaseResult.Target, phase.Command)
		cmdReq := phase.InfraCmdReq
		// Pass the request id so SSE subscribers receive this phase's log/status events
		output, err := RemoteCommandToInfra(nsId, infraId, nodeGroupId, phase.NodeId, phase.LabelSelector, &cmdReq, xRequestId)
		if err != nil {
			phaseResult.Status = model.PostCommandStatusFailed
			phaseResults = append(phaseResults, phaseResult)
			overall = model.PostCommandStatusFailed
			if firstErr == nil {
				firstErr = fmt.Errorf("post-deployment phase %d failed: %w", i+1, err)
			}
			if !phase.ContinueOnError {
				stopped = true
			}
			continue
		}

		result := model.ConvertSshCmdResultsForAPI(output)
		status, failed := aggregatePostCommandResults(&result, cmdReq.GetEffectiveTimeout())
		phaseResult.Status = status
		phaseResult.Results = result
		phaseResults = append(phaseResults, phaseResult)

		total := len(result.Results)
		if status == model.PostCommandStatusCompleted {
			log.Info().Msgf("Post-deployment phase %d completed on %d node(s)", i+1, total)
		} else {
			log.Warn().Msgf("Post-deployment phase %d failed on %d/%d node(s) (%s)", i+1, failed, total, phaseResult.Target)
			appendInfraSystemMessage(nsId, infraId,
				fmt.Sprintf("post-deployment phase %d (%s) failed on %d/%d node(s) (see postCommandResults)",
					i+1, phaseResult.Target, failed, total))
			if !phase.ContinueOnError {
				stopped = true
			}
		}
		overall = mergePostCommandStatus(overall, status)
	}

	persistPostCommandOutcome(nsId, infraId, overall, phaseResults)
	publishPostCommandDone(xRequestId, phaseResults, overall, startedAt, firstErr)
	return overall, firstErr
}

// publishPostCommandDone emits the terminal SSE event so streaming clients can close
func publishPostCommandDone(xRequestId string, phases []model.PostCommandPhaseResult,
	overall model.PostCommandStatus, startedAt time.Time, err error) {
	if xRequestId == "" {
		return
	}
	total, completed, failed := 0, 0, 0
	for _, ph := range phases {
		for _, r := range ph.Results.Results {
			total++
			if r.Error == "" {
				completed++
			} else {
				failed++
			}
		}
	}
	summary := &model.CommandDoneSummary{
		TotalNodes:     total,
		CompletedNodes: completed,
		FailedNodes:    failed,
		ElapsedSeconds: int64(time.Since(startedAt).Seconds()),
	}
	if err != nil {
		summary.Error = err.Error()
	} else if overall != model.PostCommandStatusCompleted {
		summary.Error = fmt.Sprintf("post-deployment commands finished with status %s", overall)
	}
	// Brief delay so a client that just received the creation response can subscribe
	// before the terminal event (the ring buffer also replays it)
	time.Sleep(500 * time.Millisecond)
	PublishCommandEvent(xRequestId, model.CommandStreamEvent{
		Type:      model.EventCommandDone,
		Timestamp: time.Now().Format(time.RFC3339Nano),
		Summary:   summary,
	})
}

// markPostCommandRunning records the tracking id and the in-progress status so
// clients can stream/poll immediately after the creation response returns
func markPostCommandRunning(nsId, infraId, xRequestId string) {
	infraTmp, _, err := GetInfraObject(nsId, infraId)
	if err != nil {
		log.Warn().Err(err).Msg("Cannot load infra to mark post-command as running")
		return
	}
	infraTmp.PostCommandStatus = model.PostCommandStatusRunning
	infraTmp.PostCommandRequestId = xRequestId
	UpdateInfraInfo(nsId, infraTmp)
}

// ValidatePostCommandRequest checks post-deployment command request shape:
// postCommand and postCommands are mutually exclusive, each phase targets at
// most one scope, and the cumulative timeout budget is bounded (phases run
// inside the synchronous creation call).
func ValidatePostCommandRequest(phases []model.PostCommandReq) error {
	all := phases
	totalMinutes := 0
	for i, p := range all {
		targets := 0
		for _, t := range []string{p.NodeGroupId, p.NodeId, p.LabelSelector} {
			if t != "" {
				targets++
			}
		}
		if targets > 1 {
			return fmt.Errorf("postCommands[%d]: set at most one of nodeGroupId, nodeId, labelSelector", i)
		}
		if len(p.Command) == 0 {
			return fmt.Errorf("postCommands[%d]: command is empty", i)
		}
		totalMinutes += p.GetEffectiveTimeout()
	}
	if totalMinutes > postCommandTotalTimeoutBudgetMinutes {
		return fmt.Errorf("cumulative postCommands timeout (%dm) exceeds the %dm budget; split the work or lower timeoutMinutes",
			totalMinutes, postCommandTotalTimeoutBudgetMinutes)
	}
	return nil
}

// postCommandTotalTimeoutBudgetMinutes bounds the sum of phase timeouts
const postCommandTotalTimeoutBudgetMinutes = 120

// normalizePostCommandPhases drops phases without commands
func normalizePostCommandPhases(phases []model.PostCommandReq) []model.PostCommandReq {
	normalized := make([]model.PostCommandReq, 0, len(phases))
	for _, p := range phases {
		if len(p.Command) > 0 {
			normalized = append(normalized, p)
		}
	}
	return normalized
}

// mergePostCommandStatus combines phase statuses into an overall status
func mergePostCommandStatus(overall, phase model.PostCommandStatus) model.PostCommandStatus {
	if phase == model.PostCommandStatusSkipped || phase == model.PostCommandStatusCompleted {
		if overall == model.PostCommandStatusCompleted && phase == model.PostCommandStatusCompleted {
			return model.PostCommandStatusCompleted
		}
		return overall
	}
	if overall == model.PostCommandStatusCompleted {
		return phase
	}
	if overall == model.PostCommandStatusFailed && phase == model.PostCommandStatusCompletedWithErrors {
		return model.PostCommandStatusCompletedWithErrors
	}
	return overall
}

// waitForSshReadiness polls target nodes until SSH accepts a trivial command
// (bounded); returns regardless so per-node failures are reported by the run.
func waitForSshReadiness(nsId, infraId, nodeGroupId string) {
	probe := model.InfraCmdReq{Command: []string{"true"}, TimeoutMinutes: 1}
	deadline := time.Now().Add(sshReadinessTimeout)
	for attempt := 1; time.Now().Before(deadline); attempt++ {
		results, err := RemoteCommandToInfra(nsId, infraId, nodeGroupId, "", "", &probe, "")
		if err == nil && len(results) > 0 {
			ready := true
			for _, r := range results {
				if r.Err != nil {
					ready = false
					break
				}
			}
			if ready {
				log.Info().Msgf("SSH ready on %d node(s) after %d attempt(s)", len(results), attempt)
				return
			}
		}
		log.Debug().Msgf("SSH not ready yet (attempt %d); retrying", attempt)
		time.Sleep(sshReadinessInterval)
	}
	log.Warn().Msg("SSH readiness wait timed out; running post-deployment commands anyway")
}

// aggregatePostCommandResults derives the overall status from per-node results
// and rewrites timeout errors to be distinguishable from auth/exec failures.
func aggregatePostCommandResults(result *model.InfraSshCmdResultForAPI, timeoutMinutes int) (model.PostCommandStatus, int) {
	failed := 0
	for i := range result.Results {
		if result.Results[i].Error == "" {
			continue
		}
		failed++
		if strings.Contains(result.Results[i].Error, "context deadline exceeded") {
			result.Results[i].Error = fmt.Sprintf("timed out after %dm: %s", timeoutMinutes, result.Results[i].Error)
		}
	}
	total := len(result.Results)
	switch {
	case total == 0 || failed == total:
		return model.PostCommandStatusFailed, failed
	case failed > 0:
		return model.PostCommandStatusCompletedWithErrors, failed
	}
	return model.PostCommandStatusCompleted, 0
}

const (
	// sshReadinessTimeout bounds the wait for fresh nodes to accept SSH
	sshReadinessTimeout = 3 * time.Minute
	// sshReadinessInterval is the retry interval of the SSH readiness probe
	sshReadinessInterval = 10 * time.Second
)

// persistPostCommandOutcome stores the aggregated post-command status/results on the infra.
// The legacy postCommandResult field mirrors phase 1 for backward compatibility.
func persistPostCommandOutcome(nsId, infraId string, status model.PostCommandStatus, phases []model.PostCommandPhaseResult) {
	infraTmp, _, err := GetInfraObject(nsId, infraId)
	if err != nil {
		log.Warn().Err(err).Msg("Cannot load infra to persist post-command outcome")
		return
	}
	infraTmp.PostCommandStatus = status
	infraTmp.PostCommandResults = phases
	UpdateInfraInfo(nsId, infraTmp)
}

// handlePostCommands handles post-deployment command execution.
// Phases (postCommands) take precedence; the legacy single postCommand runs as one phase.
//
// Async mode returns immediately after registering the tracking id: nodes are
// already Running (and billing), so callers should not wait for bootstrap. The
// run continues in the background, detached from the request lifecycle, and is
// observable via SSE (xRequestId) or by polling the infra object.
func handlePostCommands(nsId, infraId string, infraTmp model.InfraInfo) error {
	phases := infraTmp.PostCommands
	if len(phases) == 0 {
		return nil
	}

	xRequestId := newPostCommandRequestId(infraId)
	markPostCommandRunning(nsId, infraId, xRequestId)

	if infraTmp.PostCommandAsync {
		log.Info().Msgf("Executing post-deployment commands in background (%d phase(s), xRequestId: %s)", len(phases), xRequestId)
		go func() {
			if _, err := executePostCommands(nsId, infraId, "", phases, xRequestId); err != nil {
				log.Error().Err(err).Str("xRequestId", xRequestId).Msg("Background post-deployment commands failed")
			}
		}()
		return nil
	}

	log.Info().Msgf("Executing post-deployment commands (%d phase(s), xRequestId: %s)", len(phases), xRequestId)
	_, err := executePostCommands(nsId, infraId, "", phases, xRequestId)
	return err
}

// newPostCommandRequestId builds the streaming/tracking key of a post-command run
func newPostCommandRequestId(infraId string) string {
	return fmt.Sprintf("pc-%s-%s", infraId, common.GenUid())
}
