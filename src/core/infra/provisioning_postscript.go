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
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/rs/zerolog/log"
)

// executePostScript runs the phases of a post-deployment script against the infra,
// aggregates per-node outcomes, persists status/results, and logs accurately.
// Shared by creation, autopilot, autopilot scale-out, and nodeGroup addition.
//
// defaultNodeGroupId scopes phases that carry no explicit target (used when
// bootstrapping a newly added nodeGroup); "" means all nodes.
func executePostScript(nsId, infraId, defaultNodeGroupId string, phases []model.PostScriptPhase, xRequestId string) (model.PostScriptStatus, error) {
	phases = normalizePostScriptPhases(phases)
	if len(phases) == 0 {
		return model.PostScriptStatusNone, nil
	}
	startedAt := time.Now()

	// SSH readiness gate: fresh nodes often refuse SSH for a short while
	// (cloud-init). Proceed early when reachable; on timeout run anyway so
	// genuine auth/config errors are reported per node rather than hidden.
	waitForSshReadiness(nsId, infraId, defaultNodeGroupId)
	waitForInitScript(nsId, infraId, defaultNodeGroupId)

	phaseResults := make([]model.PostScriptPhaseResult, 0, len(phases))
	overall := model.PostScriptStatusCompleted
	stopped := false
	var firstErr error

	for i := range phases {
		if i > 0 && !stopped {
			// finished phases are visible while the later ones run
			persistPostScriptOutcome(nsId, infraId, model.PostScriptStatusRunning, phaseResults)
		}
		phase := phases[i]
		phaseResult := model.PostScriptPhaseResult{
			Phase:  i + 1,
			Target: phase.Target(),
		}

		if stopped {
			phaseResult.Status = model.PostScriptStatusSkipped
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
		// Pass the request id so SSE subscribers receive this phase's log/status events;
		// the terminal event is published once, after the last phase
		output, err := remoteCommandToInfra(nsId, infraId, nodeGroupId, phase.NodeId, phase.LabelSelector, &cmdReq, xRequestId, false)
		if err != nil {
			phaseResult.Status = model.PostScriptStatusFailed
			phaseResults = append(phaseResults, phaseResult)
			overall = model.PostScriptStatusFailed
			if firstErr == nil {
				firstErr = fmt.Errorf("post-deployment phase %d failed: %w", i+1, err)
			}
			if !phase.ContinueOnError {
				stopped = true
			}
			continue
		}

		result := model.ConvertSshCmdResultsForAPI(output)
		status, failed := aggregatePostScriptResults(&result, cmdReq.GetEffectiveTimeout())
		phaseResult.Status = status
		phaseResult.Results = result
		phaseResults = append(phaseResults, phaseResult)

		total := len(result.Results)
		if status == model.PostScriptStatusCompleted {
			log.Info().Msgf("Post-deployment phase %d completed on %d node(s)", i+1, total)
		} else {
			log.Warn().Msgf("Post-deployment phase %d failed on %d/%d node(s) (%s)", i+1, failed, total, phaseResult.Target)
			appendInfraSystemMessage(nsId, infraId,
				fmt.Sprintf("post-deployment phase %d (%s) failed on %d/%d node(s) (see postScriptResults)",
					i+1, phaseResult.Target, failed, total))
			if !phase.ContinueOnError {
				stopped = true
			}
		}
		overall = mergePostScriptStatus(overall, status)
	}

	persistPostScriptOutcome(nsId, infraId, overall, phaseResults)
	publishPostScriptDone(xRequestId, phaseResults, overall, startedAt, firstErr)
	return overall, firstErr
}

// publishPostScriptDone emits the terminal SSE event so streaming clients can close
func publishPostScriptDone(xRequestId string, phases []model.PostScriptPhaseResult,
	overall model.PostScriptStatus, startedAt time.Time, err error) {
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
	} else if overall != model.PostScriptStatusCompleted {
		summary.Error = fmt.Sprintf("post-deployment script finished with status %s", overall)
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

// updateInfraPostScript changes the stored Infra record in one locked read-modify-write and
// reports whether it wrote. It reads the record alone (no Nodes) and shares the lock with
// UpdateInfraInfo, so it cannot write back fields another flow changed in the meantime.
// Its callers own PostScriptStatus, PostScriptResults and PostScriptRequestId; UpdateInfraInfo
// keeps the stored values of those three.
func updateInfraPostScript(nsId, infraId string, mutate func(info *model.InfraInfo) bool) bool {
	key := common.GenInfraKey(nsId, infraId, "")
	infraInfoMutex.Lock()
	defer infraInfoMutex.Unlock()

	keyValue, exists, err := kvstore.GetKv(key)
	if err != nil || !exists {
		return false
	}
	info := model.InfraInfo{}
	if err := json.Unmarshal([]byte(keyValue.Value), &info); err != nil || info.Id == "" {
		return false
	}
	if !mutate(&info) {
		return false
	}
	info.Node = nil
	val, err := json.Marshal(info)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false
	}
	if err := kvstore.Put(key, string(val)); err != nil {
		log.Error().Err(err).Msg("")
		return false
	}
	InvalidateReadCache(nsId, infraId)
	return true
}

// markPostScriptRunning records the tracking id and the in-progress status so
// clients can stream/poll immediately after the creation response returns
func markPostScriptRunning(nsId, infraId, xRequestId string) {
	if !updateInfraPostScript(nsId, infraId, func(info *model.InfraInfo) bool {
		info.PostScriptStatus = model.PostScriptStatusRunning
		info.PostScriptRequestId = xRequestId
		info.PostScriptResults = nil // results of an earlier run do not belong to this request id
		return true
	}) {
		log.Warn().Msgf("Cannot mark the post-deployment script as running (Infra %s/%s)", nsId, infraId)
	}
}

// ValidatePostScript checks the shape of a post-deployment script: each phase targets
// at most one scope and has something to run, and the cumulative timeout budget is
// bounded (phases run inside the synchronous creation call).
func ValidatePostScript(ps *model.PostScriptReq) error {
	if ps == nil {
		return nil
	}
	totalMinutes := 0
	for i, p := range ps.Phases {
		targets := 0
		for _, t := range []string{p.NodeGroupId, p.NodeId, p.LabelSelector} {
			if t != "" {
				targets++
			}
		}
		if targets > 1 {
			return fmt.Errorf("postScript.phases[%d]: set at most one of nodeGroupId, nodeId, labelSelector", i)
		}
		if len(p.Command) == 0 && p.TemplateId == "" && len(p.Scripts) == 0 {
			return fmt.Errorf("postScript.phases[%d]: command is empty (give command, templateId or scripts)", i)
		}
		totalMinutes += p.GetEffectiveTimeout()
	}
	if totalMinutes > postScriptTotalTimeoutBudgetMinutes {
		return fmt.Errorf("cumulative postScript timeout (%dm) exceeds the %dm budget; split the work or lower timeoutMinutes",
			totalMinutes, postScriptTotalTimeoutBudgetMinutes)
	}
	return nil
}

// postScriptTotalTimeoutBudgetMinutes bounds the sum of phase timeouts
const postScriptTotalTimeoutBudgetMinutes = 120

// normalizePostScriptPhases drops phases without commands
func normalizePostScriptPhases(phases []model.PostScriptPhase) []model.PostScriptPhase {
	normalized := make([]model.PostScriptPhase, 0, len(phases))
	for _, p := range phases {
		if len(p.Command) > 0 || p.TemplateId != "" || len(p.Scripts) > 0 {
			normalized = append(normalized, p)
		}
	}
	return normalized
}

// mergePostScriptStatus combines phase statuses into an overall status
func mergePostScriptStatus(overall, phase model.PostScriptStatus) model.PostScriptStatus {
	if phase == model.PostScriptStatusSkipped || phase == model.PostScriptStatusCompleted {
		if overall == model.PostScriptStatusCompleted && phase == model.PostScriptStatusCompleted {
			return model.PostScriptStatusCompleted
		}
		return overall
	}
	if overall == model.PostScriptStatusCompleted {
		return phase
	}
	if overall == model.PostScriptStatusFailed && phase == model.PostScriptStatusCompletedWithErrors {
		return model.PostScriptStatusCompletedWithErrors
	}
	return overall
}

// waitForSshReadiness polls target nodes until SSH accepts a trivial command
// (bounded); returns regardless so per-node failures are reported by the run.
// initScriptWaitTimeout bounds how long the post-deployment script waits for the first-boot init script.
const initScriptWaitTimeout = 10 * time.Minute

// waitForInitScript holds the post-deployment script until the Nodes' first-boot init script has finished,
// so the two do not run apt or systemd at the same time. SSH opens as soon as the wrapper has created the
// account, which is before its steps have run. The wait reads the status marker on the Node itself, so it
// works without a notifier; NodeGroups whose Nodes all have a final result recorded are not probed, and
// on timeout the script proceeds anyway.
func waitForInitScript(nsId, infraId, nodeGroupId string) {
	ngMap := LoadInfraNodeGroupMap(nsId, infraId)
	results, _ := listInitScriptResults(nsId, infraId)
	nodeIds, _ := ListNodeId(nsId, infraId)
	var groups []string
	for id, ng := range ngMap {
		if (nodeGroupId != "" && id != nodeGroupId) || !initScriptEnabled(ng.InitScript) {
			continue
		}
		pending := false
		for _, nodeId := range nodeIds {
			if !strings.HasPrefix(nodeId, id+"-") {
				continue
			}
			if r, ok := results[nodeId]; !ok || r.Status == model.InitScriptStatusInitializing {
				pending = true
				break
			}
		}
		if pending {
			groups = append(groups, id)
		}
	}
	if len(groups) == 0 {
		return
	}
	secs := int(initScriptWaitTimeout.Seconds())
	cmd := fmt.Sprintf(`f=/var/lib/tb/init-script.status; w=0; while [ ! -f "$f" ] && [ $w -lt %d ]; do sleep 5; w=$((w+5)); done; `+
		`if [ -f "$f" ]; then echo "init script: $(cat "$f")"; else echo "init script: not finished after %ds, continuing"; fi`, secs, secs)
	req := model.InfraCmdReq{Command: []string{cmd}, TimeoutMinutes: int(initScriptWaitTimeout.Minutes()) + 1}
	log.Info().Msgf("Post-deployment script: waiting for the first-boot init script of NodeGroup(s) %v in %s/%s", groups, nsId, infraId)
	var wg sync.WaitGroup
	for _, id := range groups {
		wg.Add(1)
		go func(ngId string) {
			defer wg.Done()
			if _, err := RemoteCommandToInfra(nsId, infraId, ngId, "", "", &req, ""); err != nil {
				log.Warn().Err(err).Msgf("Post-deployment script: init script wait failed for NodeGroup %s; continuing", ngId)
			}
		}(id)
	}
	wg.Wait()
}

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
	log.Warn().Msg("SSH readiness wait timed out; running the post-deployment script anyway")
}

// aggregatePostScriptResults derives the overall status from per-node results
// and rewrites timeout errors to be distinguishable from auth/exec failures.
func aggregatePostScriptResults(result *model.InfraSshCmdResultForAPI, timeoutMinutes int) (model.PostScriptStatus, int) {
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
		return model.PostScriptStatusFailed, failed
	case failed > 0:
		return model.PostScriptStatusCompletedWithErrors, failed
	}
	return model.PostScriptStatusCompleted, 0
}

const (
	// sshReadinessTimeout bounds the wait for fresh nodes to accept SSH
	sshReadinessTimeout = 8 * time.Minute // KT and similar CSPs can take several minutes after ACTIVE before sshd answers
	// sshReadinessInterval is the retry interval of the SSH readiness probe
	sshReadinessInterval = 10 * time.Second
)

// cleanupInterruptedPostScript marks a run the previous process left as Running as Interrupted
func cleanupInterruptedPostScript(nsId, infraId string) bool {
	return updateInfraPostScript(nsId, infraId, func(info *model.InfraInfo) bool {
		if info.PostScriptStatus != model.PostScriptStatusRunning {
			return false
		}
		info.PostScriptStatus = model.PostScriptStatusInterrupted
		info.SystemMessage = append(info.SystemMessage, "post-deployment script was interrupted by a system restart")
		return true
	})
}

// persistPostScriptOutcome stores the aggregated status and per-phase results on the infra
func persistPostScriptOutcome(nsId, infraId string, status model.PostScriptStatus, phases []model.PostScriptPhaseResult) {
	if !updateInfraPostScript(nsId, infraId, func(info *model.InfraInfo) bool {
		info.PostScriptStatus = status
		info.PostScriptResults = phases
		return true
	}) {
		log.Warn().Msgf("Cannot persist the post-deployment script outcome (Infra %s/%s)", nsId, infraId)
	}
}

// handlePostScript runs the post-deployment script requested for the Infra.
// async is the postScriptAsync option of the call that created the Nodes.
//
// Async mode returns immediately after registering the tracking id: nodes are
// already Running (and billing), so callers should not wait for bootstrap. The
// run continues in the background, detached from the request lifecycle, and is
// observable via SSE (xRequestId) or by polling the infra object.
func handlePostScript(nsId, infraId string, infraTmp model.InfraInfo, async bool) error {
	if !infraTmp.PostScript.HasPhases() {
		return nil
	}
	phases := infraTmp.PostScript.Phases

	xRequestId := newPostScriptRequestId(infraId)
	markPostScriptRunning(nsId, infraId, xRequestId)

	if async {
		log.Info().Msgf("Executing the post-deployment script in background (%d phase(s), xRequestId: %s)", len(phases), xRequestId)
		go func() {
			if _, err := executePostScript(nsId, infraId, "", phases, xRequestId); err != nil {
				log.Error().Err(err).Str("xRequestId", xRequestId).Msg("Background post-deployment script failed")
			}
		}()
		return nil
	}

	log.Info().Msgf("Executing the post-deployment script (%d phase(s), xRequestId: %s)", len(phases), xRequestId)
	_, err := executePostScript(nsId, infraId, "", phases, xRequestId)
	return err
}

// newPostScriptRequestId builds the streaming/tracking key of a post-deployment script run
func newPostScriptRequestId(infraId string) string {
	return fmt.Sprintf("ps-%s-%s", infraId, common.GenUid())
}
