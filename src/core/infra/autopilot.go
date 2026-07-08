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
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/rs/zerolog/log"
)

// activeAutopilotRun holds the original request and start time for a running autopilot job.
// attemptsBySpec accumulates completed provisioning attempts keyed by NodeSpec name;
// it is updated concurrently via mu and read by GetInfraAutopilotStatus for live polling.
type activeAutopilotRun struct {
	req            *model.InfraAutopilotReq
	startTime      time.Time
	mu             sync.Mutex
	attemptsBySpec map[string][]model.ProvisioningAttempt
}

// activeAutopilotRuns tracks all in-flight autopilot provisioning jobs,
// keyed by "{nsId}/{infraId}" (see autopilotRunKey).
var activeAutopilotRuns sync.Map

// cachedAutopilotPlan wraps a review result with the NodeSpecs it was computed
// for and its creation time, so CreateInfraAutopilot can reject a plan whose
// request has since changed or that has gone stale (CSP stock/pricing drift).
type cachedAutopilotPlan struct {
	result        *model.InfraAutopilotReviewResult
	nodeSpecsJSON string
	storedAt      time.Time
}

// autopilotPlanMaxAge bounds how long a cached review plan stays usable.
const autopilotPlanMaxAge = 30 * time.Minute

// autopilotPlans caches the execution plan produced by ReviewInfraAutopilot so that
// CreateInfraAutopilot can execute the exact same plan without re-running ReviewSpecImagePair.
// Key: "{nsId}/{infraId}" (string) → value: *cachedAutopilotPlan
var autopilotPlans sync.Map

// autopilotRunKey builds the sync.Map key for plans and active runs.
// Including nsId prevents collisions between same-named infras in different namespaces.
func autopilotRunKey(nsId, infraName string) string {
	return nsId + "/" + infraName
}

// ReviewInfraAutopilot performs a pre-flight review of an InfraAutopilotReq
// without creating any resources. It resolves candidate specs and images for
// each NodeSpec, runs ReviewSpecImagePair on each candidate, and returns a
// structured review result.
func ReviewInfraAutopilot(ctx context.Context, nsId string, req *model.InfraAutopilotReq) (*model.InfraAutopilotReviewResult, error) {
	log.Info().Msgf("ReviewInfraAutopilot: ns=%s, name=%s, nodeSpecs=%d", nsId, req.Name, len(req.NodeSpecs))

	result := &model.InfraAutopilotReviewResult{
		Name:    req.Name,
		Reviews: make([]model.NodeSpecReview, 0, len(req.NodeSpecs)),
	}

	summary := model.ReviewSummary{
		Feasibility:      "Feasible",
		CostPerHourMin:   0,
		CostPerHourMax:   0,
		UnreachableSpecs: []string{},
	}

	// Review all NodeSpecs in parallel — each is independent.
	type nsReviewResult struct {
		idx    int
		review *model.NodeSpecReview
		err    error
	}
	nsResults := make([]nsReviewResult, len(req.NodeSpecs))
	var nsWg sync.WaitGroup
	for i, ns := range req.NodeSpecs {
		nsWg.Add(1)
		go func(i int, ns model.NodeSpec) {
			defer nsWg.Done()
			rev, err := planNodeSpec(ctx, nsId, ns, req.Policy)
			nsResults[i] = nsReviewResult{idx: i, review: rev, err: err}
		}(i, ns)
	}
	nsWg.Wait()

	for _, r := range nsResults {
		ns := req.NodeSpecs[r.idx]
		review := r.review
		if r.err != nil {
			log.Warn().Err(r.err).Msgf("failed to review nodeSpec '%s'", ns.Name)
			review = &model.NodeSpecReview{
				NodeSpecName: ns.Name,
				DesiredCount: ns.DesiredCount,
				Feasibility:  "Infeasible",
				Candidates:   []model.CandidateReview{},
			}
			summary.UnreachableSpecs = append(summary.UnreachableSpecs, ns.Name)
			summary.Feasibility = "PartiallyFeasible"
		}
		result.Reviews = append(result.Reviews, *review)

		summary.DesiredTotal += ns.DesiredCount
		summary.ValidCandidates += review.ValidCandidates
		summary.CostPerHourMin += review.CostPerHourMin
		summary.CostPerHourMax += review.CostPerHourMax

		for _, c := range review.Candidates {
			if c.RiskLevel == "high" {
				summary.HighRiskCandidates++
			}
			if c.SuggestedZone != "" {
				alreadyAdded := false
				for _, z := range summary.ConfirmedStockZones {
					if z == c.SuggestedZone {
						alreadyAdded = true
						break
					}
				}
				if !alreadyAdded {
					summary.ConfirmedStockZones = append(summary.ConfirmedStockZones, c.SuggestedZone)
				}
			}
		}

		if review.Feasibility == "Infeasible" {
			summary.Feasibility = "PartiallyFeasible"
		}
	}

	if len(summary.UnreachableSpecs) == len(req.NodeSpecs) {
		summary.Feasibility = "Infeasible"
	}

	result.Summary = summary

	// Cache the plan so CreateInfraAutopilot can execute it without re-running
	// ReviewSpecImagePair. NodeSpecs are recorded alongside so a later create
	// call with modified NodeSpecs does not silently execute a mismatched plan.
	nodeSpecsJSON, err := json.Marshal(req.NodeSpecs)
	if err == nil {
		autopilotPlans.Store(autopilotRunKey(nsId, req.Name), &cachedAutopilotPlan{
			result:        result,
			nodeSpecsJSON: string(nodeSpecsJSON),
			storedAt:      time.Now(),
		})
	}

	return result, nil
}

const (
	// planBatchPerCSP is the number of specs reviewed per CSP per wave during planning.
	// The planner starts with this many; if valid candidates are still insufficient
	// it fetches the next batch from each CSP automatically.
	planBatchPerCSP = 5
	// planMaxTotal caps the total candidates reviewed across all waves to bound latency.
	planMaxTotal = 100
	// maxConcurrentReviews bounds simultaneous outbound CSP API calls.
	maxConcurrentReviews = 20
)

// reviewOneBatch reviews a slice of specs in parallel, returning one CandidateReview per spec.
// poolOffset is added to each PoolIndex so indices are globally unique across waves.
func reviewOneBatch(ctx context.Context, ns model.NodeSpec, batch []model.SpecInfo, poolOffset int) []model.CandidateReview {
	results := make([]model.CandidateReview, len(batch))
	sem := make(chan struct{}, maxConcurrentReviews)
	osType := buildOSType(ns.ImageRequirement)
	rootDiskType := ns.RootDiskType

	var wg sync.WaitGroup
	for i, spec := range batch {
		wg.Add(1)
		go func(i int, spec model.SpecInfo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			cr := model.CandidateReview{
				PoolIndex:           poolOffset + i,
				SpecId:              spec.Id,
				ProviderName:        spec.ProviderName,
				RegionName:          spec.RegionName,
				ConnectionName:      spec.ConnectionName,
				CspSpecName:         spec.CspSpecName,
				VCPU:                float32(spec.VCPU),
				MemoryGiB:           spec.MemoryGiB,
				AcceleratorType:     spec.AcceleratorType,
				AcceleratorModel:    spec.AcceleratorModel,
				AcceleratorCount:    int(spec.AcceleratorCount),
				AcceleratorMemoryGB: spec.AcceleratorMemoryGB,
				CostPerHour:         float64(spec.CostPerHour),
				RiskLevel:           "low",
			}

			imageReq := model.SearchImageRequest{
				MatchedSpecId: spec.Id,
				ProviderName:  spec.ProviderName,
				RegionName:    spec.RegionName,
				OSType:        osType,
				IsGPUImage:    ns.ImageRequirement.IsGPUImage,
			}
			images, _, imgErr := provisionSearchImage(model.SystemCommonNs, imageReq, false)
			if imgErr != nil || len(images) == 0 {
				cr.IsValid = false
				reason := fmt.Sprintf("no image found for osType=%q", osType)
				if imgErr != nil {
					reason = imgErr.Error()
				}
				cr.InvalidReasons = []string{reason}
				cr.RiskLevel = "high"
				results[i] = cr
				return
			}
			cr.ResolvedImageId = images[0].Id

			pairResult, pairErr := ReviewSpecImagePair(ctx, spec.Id, images[0].Id, rootDiskType, "")
			if pairErr != nil {
				cr.IsValid = false
				cr.InvalidReasons = []string{pairErr.Error()}
				cr.RiskLevel = "high"
				results[i] = cr
				return
			}

			cr.IsValid = pairResult.IsValid
			cr.SuggestedZone = pairResult.SuggestedZone
			cr.SuggestedSystemDisk = pairResult.SuggestedSystemDisk

			if !pairResult.IsValid {
				cr.InvalidReasons = append(cr.InvalidReasons, pairResult.Errors...)
				cr.RiskLevel = "high"
				for _, e := range pairResult.Errors {
					lower := strings.ToLower(e)
					if strings.Contains(lower, "capacity") ||
						strings.Contains(lower, "quota") ||
						strings.Contains(lower, "stock") {
						cr.IsCapacityIssue = true
						break
					}
				}
			} else {
				if len(pairResult.Warnings) > 0 {
					cr.RiskLevel = "medium"
					cr.RiskReasons = pairResult.Warnings
				}
			}
			results[i] = cr
		}(i, spec)
	}
	wg.Wait()
	return results
}

// planNodeSpec builds the execution plan for a single NodeSpec.
//
// It discovers candidates via an adaptive CSP-balanced wave (reviewBatchPerCSP per CSP
// per wave, extending until desiredCount valid candidates are found or all specs exhausted),
// then applies strategy-aware ordering and pre-assigns node group names and requested counts.
//
// The returned NodeSpecReview is both the review output shown to the user and the execution
// plan used by CreateInfraAutopilot — valid candidates are ordered for execution and carry
// PlannedNodeGroupName / PlannedRequestedCount; invalid candidates follow for transparency.
func planNodeSpec(ctx context.Context, nsId string, ns model.NodeSpec, policy model.AutopilotPolicy) (*model.NodeSpecReview, error) {
	// Step 1: Fetch all matching specs (caller sends limit:0 — no DB cap).
	specs, err := RecommendSpec(ctx, model.SystemCommonNs, ns.SpecFilter)
	if err != nil {
		return nil, fmt.Errorf("RecommendSpec failed for nodeSpec '%s': %w", ns.Name, err)
	}
	if len(specs) == 0 {
		return &model.NodeSpecReview{
			NodeSpecName: ns.Name,
			DesiredCount: ns.DesiredCount,
			Feasibility:  "Infeasible",
			Candidates:   []model.CandidateReview{},
		}, nil
	}

	// Step 2: Group specs by CSP, preserving cost-ascending order within each group.
	cspGroups := make(map[string][]model.SpecInfo)
	var cspOrder []string
	for _, s := range specs {
		p := s.ProviderName
		if _, seen := cspGroups[p]; !seen {
			cspOrder = append(cspOrder, p)
		}
		cspGroups[p] = append(cspGroups[p], s)
	}

	// Step 3: Adaptive wave — review reviewBatchPerCSP specs per CSP per wave.
	// Extends to the next wave until desiredCount valid candidates are found.
	var allCandidates []model.CandidateReview
	cursors := make(map[string]int)
	validFound := 0
	waveNum := 0

	for len(allCandidates) < planMaxTotal {
		var batch []model.SpecInfo
		for _, csp := range cspOrder {
			start := cursors[csp]
			end := start + planBatchPerCSP
			if end > len(cspGroups[csp]) {
				end = len(cspGroups[csp])
			}
			batch = append(batch, cspGroups[csp][start:end]...)
		}
		if len(batch) == 0 {
			break
		}

		waveNum++
		log.Info().Msgf("planNodeSpec '%s': wave %d — reviewing %d candidates (%d per CSP, %d CSPs)",
			ns.Name, waveNum, len(batch), planBatchPerCSP, len(cspOrder))

		waveCandidates := reviewOneBatch(ctx, ns, batch, len(allCandidates))
		allCandidates = append(allCandidates, waveCandidates...)

		batchIdx := 0
		for _, csp := range cspOrder {
			start := cursors[csp]
			end := start + planBatchPerCSP
			if end > len(cspGroups[csp]) {
				end = len(cspGroups[csp])
			}
			for j := start; j < end; j++ {
				if batchIdx < len(waveCandidates) && waveCandidates[batchIdx].IsValid {
					validFound++
				}
				batchIdx++
			}
			cursors[csp] = end
		}

		if validFound >= ns.DesiredCount {
			log.Info().Msgf("planNodeSpec '%s': found %d valid after wave %d — done",
				ns.Name, validFound, waveNum)
			break
		}

		anyMore := false
		for _, csp := range cspOrder {
			if cursors[csp] < len(cspGroups[csp]) {
				anyMore = true
				break
			}
		}
		if !anyMore {
			break
		}

		log.Info().Msgf("planNodeSpec '%s': wave %d complete — %d valid so far, extending",
			ns.Name, waveNum, validFound)
	}

	// Step 4: Separate valid/invalid candidates.
	var validCandidates, invalidCandidates []model.CandidateReview
	for _, c := range allCandidates {
		if c.IsValid {
			validCandidates = append(validCandidates, c)
		} else {
			invalidCandidates = append(invalidCandidates, c)
		}
	}

	// Step 5: Apply strategy-aware ordering to valid candidates.
	// spread: unique locations (provider+region) come first; overflow follows.
	// pack/default: cost-ascending order is already maintained from the wave.
	strategy := strings.ToLower(ns.PlacementPolicy.Strategy)
	if strategy == "spread" {
		seenLoc := map[string]bool{}
		var firstPass, overflow []model.CandidateReview
		for _, c := range validCandidates {
			lk := c.ProviderName + "+" + c.RegionName
			if !seenLoc[lk] {
				seenLoc[lk] = true
				firstPass = append(firstPass, c)
			} else {
				overflow = append(overflow, c)
			}
		}
		validCandidates = append(firstPass, overflow...)
	}

	// Step 6: Pre-assign node group names and requested counts.
	// These become the authoritative names during execution when a cached plan is used.
	for i := range validCandidates {
		validCandidates[i].PlannedNodeGroupName = fmt.Sprintf("%s-%s-%d",
			ns.Name, sanitizeForName(validCandidates[i].ProviderName), i+1)
		if strategy == "spread" {
			validCandidates[i].PlannedRequestedCount = 1
		} else {
			validCandidates[i].PlannedRequestedCount = ns.DesiredCount
		}
	}

	// Ordered candidates: valid (execution order) first, then invalid (for display).
	ordered := append(validCandidates, invalidCandidates...)

	// Step 7: Aggregate cost range from valid candidates.
	var costMin, costMax float64
	firstCost := true
	for _, c := range validCandidates {
		if firstCost {
			costMin = c.CostPerHour
			costMax = c.CostPerHour
			firstCost = false
		} else {
			if c.CostPerHour < costMin {
				costMin = c.CostPerHour
			}
			if c.CostPerHour > costMax {
				costMax = c.CostPerHour
			}
		}
	}

	validCount := len(validCandidates)
	review := &model.NodeSpecReview{
		NodeSpecName:       ns.Name,
		DesiredCount:       ns.DesiredCount,
		Candidates:         ordered,
		ValidCandidates:    validCount,
		CostPerHourMin:     costMin,
		CostPerHourMax:     costMax,
		ExpectedNodeGroups: validCount,
	}

	if validCount == 0 {
		review.Feasibility = "Infeasible"
	} else if validCount < ns.DesiredCount {
		review.Feasibility = "PartiallyFeasible"
	} else {
		review.Feasibility = "Feasible"
	}

	return review, nil
}

// buildOSType builds a SearchImage-compatible OSType string from ImageRequirement.
func buildOSType(req model.ImageRequirement) string {
	if req.OSVersion != "" {
		return strings.TrimSpace(req.OSType + " " + req.OSVersion)
	}
	return req.OSType
}

// CreateInfraAutopilot is the main entry-point for declarative resilient provisioning.
// It iterates through candidate specs (from RecommendSpec) per NodeSpec, reviewing
// each spec+image pair and attempting CSP provisioning. When a CSP-level failure
// occurs (capacity, quota, zone unavailable), it retries with the next candidate
// spec/region/CSP until DesiredCount is satisfied or all candidates are exhausted.
func CreateInfraAutopilot(ctx context.Context, nsId string, req *model.InfraAutopilotReq) (*model.InfraAutopilotResult, error) {
	startTime := time.Now()
	log.Info().Msgf("CreateInfraAutopilot: ns=%s, name=%s", nsId, req.Name)

	runKey := autopilotRunKey(nsId, req.Name)
	run := &activeAutopilotRun{
		req:            req,
		startTime:      startTime,
		attemptsBySpec: make(map[string][]model.ProvisioningAttempt),
	}
	// Reject a concurrent duplicate run for the same infra: a second submission
	// would race on the same NodeGroups and clobber the first run's live status.
	if _, exists := activeAutopilotRuns.LoadOrStore(runKey, run); exists {
		return nil, fmt.Errorf("autopilot provisioning is already in progress for infra '%s'", req.Name)
	}
	defer activeAutopilotRuns.Delete(runKey)

	// policy.TimeoutMinutes bounds the attempt budget in wall-clock time: once the
	// deadline passes, no NEW candidate attempts are launched (in-flight CSP calls
	// are allowed to finish so partially created NodeGroups are properly accounted).
	if req.Policy.TimeoutMinutes > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(req.Policy.TimeoutMinutes)*time.Minute)
		defer cancel()
	}

	// onAttempt is called after each provisioning attempt completes so that
	// GetInfraAutopilotStatus can return live attempt data during polling.
	onAttempt := func(a model.ProvisioningAttempt) {
		run.mu.Lock()
		run.attemptsBySpec[a.NodeSpecName] = append(run.attemptsBySpec[a.NodeSpecName], a)
		run.mu.Unlock()
	}

	// Load the execution plan produced by a prior ReviewInfraAutopilot call (if any).
	// When a plan is available, provisionNodeSpec skips RecommendSpec and ReviewSpecImagePair
	// and provisions directly from the pre-validated candidate list. The plan is
	// discarded when the NodeSpecs changed since the review or the plan has aged out.
	var cachedPlan *model.InfraAutopilotReviewResult
	if raw, ok := autopilotPlans.Load(runKey); ok {
		cached := raw.(*cachedAutopilotPlan)
		defer autopilotPlans.Delete(runKey)
		nodeSpecsJSON, jsonErr := json.Marshal(req.NodeSpecs)
		switch {
		case jsonErr != nil || string(nodeSpecsJSON) != cached.nodeSpecsJSON:
			log.Warn().Msgf("cached review plan for '%s' ignored: NodeSpecs changed since review", req.Name)
		case time.Since(cached.storedAt) > autopilotPlanMaxAge:
			log.Warn().Msgf("cached review plan for '%s' ignored: older than %s", req.Name, autopilotPlanMaxAge)
		default:
			cachedPlan = cached.result
		}
	}

	infraId := req.Name

	// infraOnce guarantees the base infra is created exactly once, even when multiple
	// NodeSpec goroutines race to call provision() at the same time.
	var infraOnce sync.Once
	var infraMu sync.Mutex // protects infraCreated and infraInfo across goroutines
	var infraCreated bool
	var infraInfo *model.InfraInfo

	// Base infra configuration used when creating the infra with the first NodeGroup.
	// InstallMonAgent and PostCommand are intentionally withheld here: CreateInfraDynamic
	// runs them at the end of the FIRST NodeGroup only (while all other NodeGroups are
	// still blocked on infra creation), so nodes added afterwards would never receive
	// them. Autopilot instead runs both once, after every NodeGroup has completed and
	// excess NodeGroups have been terminated — see the deferred post-processing below.
	baseInfraReq := model.InfraDynamicReq{
		Name:                   req.Name,
		InstallMonAgent:        "no",
		Description:            req.Description,
		Label:                  req.Label,
		PolicyOnPartialFailure: "continue",
	}

	// provision is the CSP provisioning callback passed to provisionNodeSpec.
	// It is goroutine-safe: the first caller creates the infra via CreateInfraDynamic;
	// all subsequent callers (including concurrent ones) add NodeGroups via
	// CreateInfraNodeGroupDynamic once the infra exists.
	//
	// Returns (actualRunning int, err error):
	//   - actualRunning: number of VMs that reached Running state in ngReq.Name NodeGroup.
	//     May be less than ngReq.NodeGroupSize on partial failure.
	//   - err: non-nil if the underlying CSP call failed (all VMs failed).
	//     If err != nil and actualRunning > 0, some VMs survived despite the failure.
	//
	// Cleanup: after every provision call, failed/undefined VMs are refined out of
	// the infra so that subsequent attempts start with a clean slate.
	provision := func(ngReq model.NodeGroupDynamicReq) (int, error) {
		refineFailed := func() {
			infraMu.Lock()
			ok := infraCreated
			infraMu.Unlock()
			if !ok {
				return
			}
			if _, refErr := HandleInfraAction(nsId, infraId, model.ActionRefine, true); refErr != nil {
				log.Warn().Err(refErr).Msgf("refine after provision error failed for nodeGroup '%s'", ngReq.Name)
			}
		}

		// thisIsFirstCall is set to true by the goroutine that runs infraOnce.Do.
		// All other goroutines block inside infraOnce.Do until infra creation completes.
		var thisIsFirstCall bool
		var firstCallErr error

		infraOnce.Do(func() {
			thisIsFirstCall = true
			singleReq := baseInfraReq
			singleReq.NodeGroups = []model.NodeGroupDynamicReq{ngReq}
			result, err := CreateInfraDynamic(ctx, nsId, &singleReq, "")
			infraMu.Lock()
			if err != nil {
				// Infra record may have been partially created before the error.
				if existing, getErr := GetInfraInfo(nsId, infraId); getErr == nil && existing != nil {
					infraInfo = existing
					infraCreated = true
				}
				firstCallErr = err
			} else {
				infraInfo = result
				infraCreated = true
			}
			infraMu.Unlock()
		})

		if thisIsFirstCall {
			actualRunning, failedCount, countErr := countNodeGroupVMs(nsId, infraId, ngReq.Name)
			if firstCallErr != nil {
				refineFailed()
				return actualRunning, firstCallErr
			}
			if failedCount > 0 {
				log.Warn().Msgf(
					"NodeGroup '%s': %d/%d VMs running; %d failed — refining failed nodes",
					ngReq.Name, actualRunning, ngReq.NodeGroupSize, failedCount,
				)
				refineFailed()
			}
			// Only when the count is UNKNOWN (listing failed) assume the nominally
			// successful creation delivered the full group. A confirmed zero count
			// must be reported as zero — inflating it would satisfy the desired
			// count with nodes that do not exist.
			if countErr != nil && actualRunning == 0 {
				log.Warn().Err(countErr).Msgf("NodeGroup '%s': node count unknown after successful creation; assuming full group", ngReq.Name)
				actualRunning = ngReq.NodeGroupSize
			}
			return actualRunning, nil
		}

		// Non-first callers: infra already exists (or creation failed).
		infraMu.Lock()
		ok := infraCreated
		infraMu.Unlock()
		if !ok {
			return 0, fmt.Errorf("infra creation failed; cannot add NodeGroup '%s'", ngReq.Name)
		}

		// CreateInfraNodeGroupDynamic can safely run concurrently for distinct NodeGroup names.
		result, err := CreateInfraNodeGroupDynamic(ctx, nsId, infraId, &ngReq)
		infraMu.Lock()
		if result != nil {
			infraInfo = result
		}
		infraMu.Unlock()

		if err != nil {
			actualRunning, _, _ := countNodeGroupVMs(nsId, infraId, ngReq.Name)
			refineFailed()
			return actualRunning, err
		}

		actualRunning, failedCount, countErr := countNodeGroupVMs(nsId, infraId, ngReq.Name)
		if failedCount > 0 {
			log.Warn().Msgf(
				"NodeGroup '%s': %d/%d VMs running; %d failed — refining failed nodes",
				ngReq.Name, actualRunning, ngReq.NodeGroupSize, failedCount,
			)
			refineFailed()
		}
		// Same rule as the first-call path: assume full group only when the count
		// is unknown; a confirmed zero stays zero.
		if countErr != nil && actualRunning == 0 {
			log.Warn().Err(countErr).Msgf("NodeGroup '%s': node count unknown after successful creation; assuming full group", ngReq.Name)
			actualRunning = ngReq.NodeGroupSize
		}
		return actualRunning, nil
	}

	// nsOutcome carries the result of one provisionNodeSpec goroutine.
	type nsOutcome struct {
		specResult *model.NodeSpecResult
		attempts   []model.ProvisioningAttempt
		err        error
	}

	var allAttempts []model.ProvisioningAttempt
	var nodeSpecResults []model.NodeSpecResult
	stats := model.AutopilotStats{}
	locationsSet := map[string]bool{}

	collectOutcome := func(r nsOutcome) {
		if r.err != nil {
			log.Warn().Err(r.err).Msgf("provisionNodeSpec failed for '%s'", r.specResult.NodeSpecName)
		}
		allAttempts = append(allAttempts, r.attempts...)
		nodeSpecResults = append(nodeSpecResults, *r.specResult)

		stats.TotalAttempts += len(r.attempts)
		for _, a := range r.attempts {
			if a.Status == "succeeded" {
				stats.Succeeded++
				stats.NodeGroupCount++
				loc := a.ConnectionName
				if a.Zone != "" {
					loc = a.ConnectionName + "/" + a.Zone
				}
				locationsSet[loc] = true
			} else {
				stats.Failed++
			}
			stats.TrimmedCount += a.TrimmedCount
		}
	}

	// All NodeSpecs are always provisioned concurrently — they are independent.
	// policy.Parallelism controls within-NodeSpec concurrent candidate waves,
	// handled inside provisionNodeSpec.
	{
		outCh := make(chan nsOutcome, len(req.NodeSpecs))
		var wg sync.WaitGroup
		for _, ns := range req.NodeSpecs {
			wg.Add(1)
			go func(ns model.NodeSpec) {
				defer wg.Done()
				// Look up the per-NodeSpec plan from the cached review (if available).
				var plan *model.NodeSpecReview
				if cachedPlan != nil {
					for i := range cachedPlan.Reviews {
						if cachedPlan.Reviews[i].NodeSpecName == ns.Name {
							r := cachedPlan.Reviews[i]
							plan = &r
							break
						}
					}
				}
				specResult, attempts, err := provisionNodeSpec(ctx, nsId, infraId, ns, req.Policy, plan, onAttempt, provision)
				outCh <- nsOutcome{specResult, attempts, err}
			}(ns)
		}
		go func() { wg.Wait(); close(outCh) }()
		for r := range outCh {
			collectOutcome(r)
		}
	}

	for loc := range locationsSet {
		stats.LocationsUsed = append(stats.LocationsUsed, loc)
	}
	stats.ElapsedSeconds = int64(time.Since(startTime).Seconds())

	// Check rollback policy: if any NodeSpec failed to meet minCount, terminate
	// everything provisioned so far and delete the infra (option=terminate waits
	// for CSP-side termination to propagate before deleting records — force would
	// orphan the surviving VMs).
	if req.Policy.OnPartialFailure == "rollback" {
		for _, r := range nodeSpecResults {
			if !r.MinFulfilled {
				infraMu.Lock()
				created := infraCreated
				infraMu.Unlock()
				if created {
					log.Warn().Msgf("rollback: terminating and deleting infra '%s' (nodeSpec '%s' below minCount)", infraId, r.NodeSpecName)
					if _, delErr := DelInfra(nsId, infraId, model.ActionTerminate); delErr != nil {
						log.Error().Err(delErr).Msgf("rollback: failed to delete infra '%s'; manual cleanup may be required", infraId)
					}
				}
				return &model.InfraAutopilotResult{
					NodeSpecResults:      nodeSpecResults,
					ProvisioningAttempts: allAttempts,
					AutopilotStats:       stats,
				}, fmt.Errorf(
					"rollback: nodeSpec '%s' provisioned %d/%d (minCount not met); provisioned resources were terminated",
					r.NodeSpecName, r.ProvisionedCount, r.DesiredCount,
				)
			}
		}
	}

	if infraInfo == nil {
		log.Warn().Msg("No node groups were provisioned; returning empty infra result")
		infraInfo = &model.InfraInfo{Name: req.Name}
	}

	// Deferred post-processing: monitoring agent installation and post-deployment
	// commands, exactly once against the final node set (withheld from
	// CreateInfraDynamic above). Skipped when nothing was provisioned.
	infraMu.Lock()
	created := infraCreated
	infraMu.Unlock()
	if created && stats.Succeeded > 0 {
		if infraObj, _, getErr := GetInfraObject(nsId, infraId); getErr == nil {
			infraObj.InstallMonAgent = req.InstallMonAgent
			if req.PostCommand != nil {
				infraObj.PostCommand = *req.PostCommand
			}
			UpdateInfraInfo(nsId, infraObj)

			if err := handleMonitoringAgent(nsId, infraId, infraObj, ""); err != nil {
				log.Error().Err(err).Msg("Failed to install monitoring agent, but continuing")
				appendInfraSystemMessage(nsId, infraId, fmt.Sprintf("Monitoring agent installation failed: %s", err.Error()))
			}
			if err := handlePostCommands(nsId, infraId, infraObj); err != nil {
				log.Error().Err(err).Msg("Failed to execute post-deployment commands, but continuing")
				appendInfraSystemMessage(nsId, infraId, fmt.Sprintf("Post-deployment commands failed: %s", err.Error()))
			}

			// Refresh so the result carries PostCommandResult and final status.
			if refreshed, refErr := GetInfraInfo(nsId, infraId); refErr == nil {
				infraInfo = refreshed
			}
		} else {
			log.Warn().Err(getErr).Msg("Cannot load infra for post-processing; skipping monitoring agent and post commands")
		}
	}

	return &model.InfraAutopilotResult{
		InfraInfo:            *infraInfo,
		NodeSpecResults:      nodeSpecResults,
		ProvisioningAttempts: allAttempts,
		AutopilotStats:       stats,
	}, nil
}

// appendInfraSystemMessage appends a message to the infra's SystemMessage list.
func appendInfraSystemMessage(nsId, infraId, msg string) {
	infraObj, _, err := GetInfraObject(nsId, infraId)
	if err != nil {
		return
	}
	infraObj.SystemMessage = append(infraObj.SystemMessage, msg)
	UpdateInfraInfo(nsId, infraObj)
}

// provisionNodeSpec resolves candidates and provisions node groups for a single NodeSpec.
// It iterates the ordered spec pool (built from RecommendSpec + spread/pack strategy),
// validates each candidate with ReviewSpecImagePair, then calls provision(ngReq).
// If provision returns a CSP error, the attempt is recorded as "csp-failed" and the
// next candidate is tried — enabling transparent retry across specs/regions/CSPs.
// locationKey returns a dedup key for a spec's physical location (provider+region).
func locationKey(spec model.SpecInfo) string {
	return spec.ProviderName + "+" + spec.RegionName
}

// Overridable in tests; production code delegates to the real implementations.
var (
	provisionRecommendSpec = func(ctx context.Context, nsId string, plan model.RecommendSpecReq) ([]model.SpecInfo, error) {
		return RecommendSpec(ctx, nsId, plan)
	}
	provisionSearchImage = func(nsId string, req model.SearchImageRequest, isCustomImage bool) ([]model.ImageInfo, int, error) {
		return resource.SearchImage(nsId, req, isCustomImage)
	}
	provisionReviewPair = func(ctx context.Context, specId, imageId, rootDiskType, zone string) (*model.SpecImagePairReviewResult, error) {
		return ReviewSpecImagePair(ctx, specId, imageId, rootDiskType, zone)
	}
	provisionGetZones = func(ctx context.Context, specId string) (*model.AvailableZonesInfo, *model.AvailableZonesError) {
		return resource.GetAvailableZonesForSpec(ctx, specId)
	}
)

func provisionNodeSpec(
	ctx context.Context,
	nsId string,
	infraId string,
	ns model.NodeSpec,
	policy model.AutopilotPolicy,
	plan *model.NodeSpecReview, // pre-computed plan from ReviewInfraAutopilot; nil = derive on the fly
	onAttempt func(model.ProvisioningAttempt), // called after each attempt; may be nil
	provision func(model.NodeGroupDynamicReq) (int, error),
) (*model.NodeSpecResult, []model.ProvisioningAttempt, error) {

	specResult := &model.NodeSpecResult{
		NodeSpecName:  ns.Name,
		DesiredCount:  ns.DesiredCount,
		NodeGroupIds:  []string{},
		LocationsUsed: []string{},
	}

	// preResolvedCandidate holds the already-validated image/zone/disk for a planned candidate.
	type preResolvedCandidate struct {
		imageId  string
		zone     string
		rootDisk string
		ngName   string
	}
	// preResolvedByIndex maps pool position → pre-resolved data (populated from plan).
	var preResolvedByIndex map[int]preResolvedCandidate

	strategy := strings.ToLower(ns.PlacementPolicy.Strategy)
	if strategy == "" {
		strategy = "pack"
	}

	maxAttempts := policy.MaxAttemptsPerSpec
	if maxAttempts <= 0 {
		maxAttempts = 10
	}

	remaining := ns.DesiredCount
	var attempts []model.ProvisioningAttempt
	recordAttempt := func(a model.ProvisioningAttempt) {
		attempts = append(attempts, a)
		if onAttempt != nil {
			onAttempt(a)
		}
	}
	nodeGroupIdx := 0
	locationNodeCount := map[string]int{}
	usedLocations := map[string]bool{}

	var poolToIterate []model.SpecInfo
	spreadFirstPassCount := 0

	if plan != nil {
		// ── Plan-based path ──────────────────────────────────────────────────────
		// Use pre-validated candidates from the review plan. ReviewSpecImagePair
		// has already been run; we skip it and provision directly.
		preResolvedByIndex = make(map[int]preResolvedCandidate)
		for _, c := range plan.Candidates {
			if !c.IsValid {
				continue
			}
			rootDisk := c.SuggestedSystemDisk
			if rootDisk == "" {
				rootDisk = ns.RootDiskType
			}
			idx := len(poolToIterate)
			poolToIterate = append(poolToIterate, model.SpecInfo{
				Id:             c.SpecId,
				ProviderName:   c.ProviderName,
				RegionName:     c.RegionName,
				ConnectionName: c.ConnectionName,
				CostPerHour:    float32(c.CostPerHour),
			})
			preResolvedByIndex[idx] = preResolvedCandidate{
				imageId:  c.ResolvedImageId,
				zone:     c.SuggestedZone,
				rootDisk: rootDisk,
				ngName:   c.PlannedNodeGroupName,
			}
		}
		// Allow all plan candidates to be tried; they are already filtered to valid.
		maxAttempts = len(poolToIterate)
		// nodeGroupIdx starts beyond the plan's highest index so zone-retry names don't collide.
		nodeGroupIdx = len(poolToIterate)

		// For spread: the plan already ordered unique-location candidates first.
		// Compute spreadFirstPassCount so the wave-size cap works correctly.
		if strategy == "spread" {
			seenLoc := map[string]bool{}
			for _, s := range poolToIterate {
				lk := locationKey(s)
				if !seenLoc[lk] {
					seenLoc[lk] = true
					spreadFirstPassCount++
				}
			}
		}
	} else {
		// ── Fallback: derive candidates on the fly ───────────────────────────────
		specs, err := provisionRecommendSpec(ctx, model.SystemCommonNs, ns.SpecFilter)
		if err != nil {
			return specResult, nil, fmt.Errorf("RecommendSpec failed: %w", err)
		}

		if policy.ExtendCandidates {
			extFilter := ns.SpecFilter
			extFilter.Limit = 0
			extSpecs, extErr := provisionRecommendSpec(ctx, model.SystemCommonNs, extFilter)
			if extErr == nil {
				seenId := map[string]bool{}
				for _, s := range specs {
					seenId[s.Id] = true
				}
				for _, s := range extSpecs {
					if !seenId[s.Id] {
						seenId[s.Id] = true
						specs = append(specs, s)
					}
				}
			}
			maxAttempts = len(specs)
		}

		if strategy == "spread" {
			seenLoc := map[string]bool{}
			var firstPass, secondPass []model.SpecInfo
			for _, s := range specs {
				lk := locationKey(s)
				if !seenLoc[lk] {
					seenLoc[lk] = true
					firstPass = append(firstPass, s)
				} else {
					secondPass = append(secondPass, s)
				}
			}
			spreadFirstPassCount = len(firstPass)
			poolToIterate = append(firstPass, secondPass...)

			if spreadFirstPassCount > 0 {
				fillRounds := (ns.DesiredCount + spreadFirstPassCount - 1) / spreadFirstPassCount
				if fillRounds < 1 {
					fillRounds = 1
				}
				for r := 0; r < fillRounds; r++ {
					poolToIterate = append(poolToIterate, firstPass...)
				}
			}
		} else {
			poolToIterate = specs
		}
	}

	// effectiveParallelism: how many spec candidates to attempt concurrently within this NodeSpec.
	// For spread strategy, each first-pass placement goes to a distinct location — they are
	// independent by definition, so auto-boost to desiredCount when Parallelism is not set.
	// An explicit policy.Parallelism always takes precedence if it is the higher value.
	effectiveParallelism := policy.Parallelism
	if effectiveParallelism <= 0 {
		effectiveParallelism = 1
	}
	if strategy == "spread" && ns.DesiredCount > effectiveParallelism {
		effectiveParallelism = ns.DesiredCount
	}

	// For spread with fill rounds, maxAttempts must cover the entire pool so fill-round
	// candidates are reachable (they sit beyond the original firstPass boundary).
	if strategy == "spread" && len(poolToIterate) > maxAttempts {
		maxAttempts = len(poolToIterate)
	}

	if effectiveParallelism > 1 {
		// ─── Parallel wave path ───────────────────────────────────────────────────
		// Candidates are processed in waves of effectiveParallelism. Each goroutine in a
		// wave independently resolves an image, runs ReviewSpecImagePair, and calls
		// provision requesting the full current remaining count. After a wave, results
		// are credited in submission order; NodeGroups that arrive after the desired
		// count is already satisfied are terminated to avoid over-provisioning.
		// Zone-retry candidates are injected into zoneRetryQueue on availability failures
		// and drained at the front of subsequent waves.
		parallelism := effectiveParallelism

		type zoneRetryCandidate struct {
			spec     model.SpecInfo
			zone     string
			imageId  string
			rootDisk string
			lk       string
		}
		type waveTask struct {
			spec         model.SpecInfo
			poolIdx      int
			ngName       string
			lk           string
			requested    int
			isZoneRetry  bool   // pre-resolved zone retry; skip SearchImage + ReviewSpecImagePair
			zrZone       string
			zrImageId    string
			zrRootDisk   string
			isPrePlanned bool   // pre-validated from plan; skip SearchImage + ReviewSpecImagePair
			ppImageId    string
			ppZone       string
			ppRootDisk   string
		}
		type waveResult struct {
			attempt      model.ProvisioningAttempt
			running      int
			ngName       string
			lk           string
			connName     string
			zone         string
			canZoneRetry bool   // true → inject zone-retry candidates into zoneRetryQueue
			zrSpec       model.SpecInfo
			zrZone       string // failed zone
			zrImage      string
			zrDisk       string
		}

		var excessNodeGroups []string
		var zoneRetryQueue []zoneRetryCandidate
		injectedZones := map[string]bool{}
		cursor := 0
		// failedFromPool counts main-pool candidates that yielded zero running nodes.
		// Each such failure extends the attempt budget by one so we can try the next
		// candidate in the pool to fill the gap — especially important for spread strategy
		// where all maxAttempts candidates are launched in one wave.
		failedFromPool := 0

		for remaining > 0 && (len(zoneRetryQueue) > 0 || (cursor < len(poolToIterate) && cursor < maxAttempts+failedFromPool)) {
			// Stop launching new waves once the run deadline (policy.TimeoutMinutes) passes.
			if ctx.Err() != nil {
				log.Warn().Msgf("nodeSpec '%s': stopping candidate waves — %v", ns.Name, ctx.Err())
				break
			}
			// Build a wave: drain zone-retry queue first, then pull from main pool.
			var wave []waveTask
			for len(wave) < parallelism && (len(zoneRetryQueue) > 0 || (cursor < len(poolToIterate) && cursor < maxAttempts+failedFromPool)) {
				// Drain zone-retry queue before pulling from main pool candidates.
				if len(zoneRetryQueue) > 0 {
					cand := zoneRetryQueue[0]
					zoneRetryQueue = zoneRetryQueue[1:]
					if ns.MaxPerLocation > 0 && locationNodeCount[cand.lk] >= ns.MaxPerLocation {
						continue
					}
					zrRequested := remaining
					if ns.MaxPerLocation > 0 {
						locRem := ns.MaxPerLocation - locationNodeCount[cand.lk]
						if zrRequested > locRem {
							zrRequested = locRem
						}
					}
					if strategy == "spread" && zrRequested > 1 {
						zrRequested = 1
					}
					nodeGroupIdx++
					zrNgName := fmt.Sprintf("%s-%s-%d", ns.Name, sanitizeForName(cand.spec.ProviderName), nodeGroupIdx)
					wave = append(wave, waveTask{
						spec:        cand.spec,
						poolIdx:     -1,
						ngName:      zrNgName,
						lk:          cand.lk,
						requested:   zrRequested,
						isZoneRetry: true,
						zrZone:      cand.zone,
						zrImageId:   cand.imageId,
						zrRootDisk:  cand.rootDisk,
					})
					continue
				}

				if cursor >= len(poolToIterate) || cursor >= maxAttempts {
					break
				}
				spec := poolToIterate[cursor]
				idx := cursor
				cursor++
				lk := locationKey(spec)

				if ns.MaxPerLocation > 0 && locationNodeCount[lk] >= ns.MaxPerLocation {
					continue
				}
				if strategy == "spread" && usedLocations[lk] {
					hasUnused := false
					for _, s2 := range poolToIterate[cursor:] {
						if !usedLocations[locationKey(s2)] {
							hasUnused = true
							break
						}
					}
					if hasUnused {
						continue
					}
				}

				requested := remaining
				if ns.MaxPerLocation > 0 {
					locRem := ns.MaxPerLocation - locationNodeCount[lk]
					if requested > locRem {
						requested = locRem
					}
				}
				if strategy == "spread" && requested > 1 {
					requested = 1
				}

				// For plan-based candidates: use pre-assigned name and pre-resolved data.
				// For fallback candidates: generate a name and mark for on-the-fly review.
				var ngName string
				var isPP bool
				var ppImg, ppZone, ppDisk string
				if pr, ok := preResolvedByIndex[idx]; ok {
					ngName = pr.ngName
					isPP = true
					ppImg = pr.imageId
					ppZone = pr.zone
					ppDisk = pr.rootDisk
				} else {
					nodeGroupIdx++
					ngName = fmt.Sprintf("%s-%s-%d", ns.Name, sanitizeForName(spec.ProviderName), nodeGroupIdx)
				}
				// Pre-mark location to prevent multiple wave tasks from targeting the same spot.
				if strategy == "spread" {
					usedLocations[lk] = true
				}
				wave = append(wave, waveTask{
					spec:         spec,
					poolIdx:      idx,
					ngName:       ngName,
					lk:           lk,
					requested:    requested,
					isPrePlanned: isPP,
					ppImageId:    ppImg,
					ppZone:       ppZone,
					ppRootDisk:   ppDisk,
				})

				// For fill-round entries (spread strategy, beyond the first-pass boundary),
				// cap the wave to the current remaining count so we don't launch far more
				// goroutines than needed and then have to terminate the excess NodeGroups.
				// If fill-round failures occur, failedFromPool extends the budget for the next wave.
				if strategy == "spread" && idx >= spreadFirstPassCount && len(wave) >= remaining {
					break
				}
			}

			if len(wave) == 0 {
				break
			}

			// Launch all wave tasks concurrently.
			waveResults := make([]waveResult, len(wave))
			var waveWg sync.WaitGroup
			for j, task := range wave {
				waveWg.Add(1)
				go func(j int, task waveTask) {
					defer waveWg.Done()
					spec := task.spec
					attempt := model.ProvisioningAttempt{
						NodeSpecName:        ns.Name,
						SpecId:              spec.Id,
						ConnectionName:      spec.ConnectionName,
						RequestedCount:      task.requested,
						PoolIndex:           task.poolIdx,
						StartedAt:           time.Now().Format(time.RFC3339),
						CostPerHour:         float64(spec.CostPerHour),
						NodeGroupName:       task.ngName,
						AcceleratorModel:    spec.AcceleratorModel,
						AcceleratorCount:    int(spec.AcceleratorCount),
						AcceleratorMemoryGB: spec.AcceleratorMemoryGB,
					}

					var zone, rootDiskType, imageId string
					var resolvedSpec model.SpecInfo

					if task.isZoneRetry {
						// Pre-resolved by a previous wave: skip SearchImage + ReviewSpecImagePair.
						zone = task.zrZone
						imageId = task.zrImageId
						rootDiskType = task.zrRootDisk
						resolvedSpec = task.spec
						attempt.ImageId = imageId
						if zone != "" {
							attempt.Zone = zone
							attempt.ZoneOverridden = true
						}
						attempt.RiskLevel = "low"
					} else if task.isPrePlanned {
						// Pre-validated by the planning phase: use resolved image/zone/disk directly.
						zone = task.ppZone
						imageId = task.ppImageId
						rootDiskType = task.ppRootDisk
						resolvedSpec = task.spec
						attempt.ImageId = imageId
						if zone != "" {
							attempt.Zone = zone
							attempt.ZoneOverridden = true
						}
						if rootDiskType != "" && rootDiskType != ns.RootDiskType {
							attempt.DiskOverridden = true
						}
						attempt.RiskLevel = "low"
					} else {
						resolvedSpec = spec
						osType := buildOSType(ns.ImageRequirement)
						images, _, imgErr := provisionSearchImage(model.SystemCommonNs, model.SearchImageRequest{
							MatchedSpecId: spec.Id,
							ProviderName:  spec.ProviderName,
							RegionName:    spec.RegionName,
							OSType:        osType,
							IsGPUImage:    ns.ImageRequirement.IsGPUImage,
						}, false)
						if imgErr != nil || len(images) == 0 {
							reason := "no matching image found"
							if imgErr != nil {
								reason = imgErr.Error()
							}
							attempt.Status = "failed"
							attempt.FailureReason = reason
							attempt.ReviewRejected = true
							attempt.CompletedAt = time.Now().Format(time.RFC3339)
							waveResults[j] = waveResult{attempt: attempt}
							return
						}
						resolvedImage := images[0]
						imageId = resolvedImage.Id
						attempt.ImageId = imageId

						pairResult, pairErr := provisionReviewPair(ctx, spec.Id, imageId, ns.RootDiskType, "")
						if pairErr != nil {
							attempt.Status = "failed"
							attempt.FailureReason = pairErr.Error()
							attempt.ReviewRejected = true
							attempt.CompletedAt = time.Now().Format(time.RFC3339)
							waveResults[j] = waveResult{attempt: attempt}
							return
						}
						if !pairResult.IsValid {
							attempt.Status = "failed"
							attempt.FailureReason = strings.Join(pairResult.Errors, "; ")
							attempt.ReviewRejected = true
							attempt.CompletedAt = time.Now().Format(time.RFC3339)
							waveResults[j] = waveResult{attempt: attempt}
							return
						}

						zone = pairResult.SuggestedZone
						rootDiskType = ns.RootDiskType
						if pairResult.SuggestedSystemDisk != "" && rootDiskType == "" {
							rootDiskType = pairResult.SuggestedSystemDisk
							attempt.DiskOverridden = true
						}
						if zone != "" {
							attempt.Zone = zone
							attempt.ZoneOverridden = true
						}
						attempt.RiskLevel = "low"
						if len(pairResult.Warnings) > 0 {
							attempt.RiskLevel = "medium"
						}
					}

					actualRunning, provErr := provision(model.NodeGroupDynamicReq{
						Name:          task.ngName,
						NodeGroupSize: task.requested,
						SpecId:        spec.Id,
						ImageId:       imageId,
						RootDiskType:  rootDiskType,
						RootDiskSize:  ns.RootDiskSize,
						Zone:          zone,
						Label:         ns.Label,
					})
					attempt.CompletedAt = time.Now().Format(time.RFC3339)
					attempt.SucceededCount = actualRunning
					attempt.TrimmedCount = task.requested - actualRunning

					// Determine whether to inject zone-retry candidates on the next wave.
					// Only inject from non-retry tasks (one retry hop per spec); only on
					// availability failures with a known zone.
					canRetry := !task.isZoneRetry && // only one zone-retry hop per spec
						provErr != nil &&
						isAvailabilityFailure(provErr.Error()) &&
						zone != ""

					if provErr != nil {
						attempt.Status = "csp-failed"
						attempt.FailureReason = provErr.Error()
					} else {
						attempt.Status = "succeeded"
					}
					waveResults[j] = waveResult{
						attempt:      attempt,
						running:      actualRunning,
						ngName:       task.ngName,
						lk:           task.lk,
						connName:     spec.ConnectionName,
						zone:         zone,
						canZoneRetry: canRetry,
						zrSpec:       resolvedSpec,
						zrZone:       zone,
						zrImage:      imageId,
						zrDisk:       rootDiskType,
					}
				}(j, task)
			}
			waveWg.Wait()

			// Credit results in submission order; schedule excess NodeGroups for termination.
			for k := range waveResults {
				r := waveResults[k]
				if r.running > 0 {
					if remaining > 0 {
						specResult.ProvisionedCount += r.running
						credit := r.running
						if credit > remaining {
							credit = remaining
						}
						remaining -= credit
						locationNodeCount[r.lk] += r.running
						usedLocations[r.lk] = true
						specResult.NodeGroupIds = append(specResult.NodeGroupIds, r.ngName)
						loc := r.connName
						if r.zone != "" {
							loc = r.connName + "/" + r.zone
						}
						specResult.LocationsUsed = append(specResult.LocationsUsed, loc)
					} else {
						// Desired count already satisfied by an earlier result in this wave.
						excessNodeGroups = append(excessNodeGroups, r.ngName)
						waveResults[k].attempt.Status = "trimmed"
						waveResults[k].attempt.FailureReason = "trimmed: desired count satisfied by concurrent attempt"
						log.Info().Msgf("NodeGroup '%s' is excess (%d nodes) — scheduling termination", r.ngName, r.running)
					}
				}
				recordAttempt(waveResults[k].attempt)

				// Count main-pool failures so the outer loop can extend the attempt budget.
				// Each failed candidate earns one more slot from the pool, allowing the
				// system to fill the gap rather than stopping short of the desired count.
				if !wave[k].isZoneRetry && r.running == 0 {
					failedFromPool++
				}

				// Inject zone-retry candidates for the next wave.
				if r.canZoneRetry && remaining > 0 {
					zonesInfo, zonesErr := provisionGetZones(ctx, r.zrSpec.Id)
					if zonesErr == nil && zonesInfo.HasZoneConcept && len(zonesInfo.AvailableZones) > 0 {
						for _, z := range zonesInfo.AvailableZones {
							if strings.EqualFold(z, r.zrZone) {
								continue // skip the zone that just failed
							}
							dedupeKey := r.zrSpec.Id + "|" + z
							if injectedZones[dedupeKey] {
								continue
							}
							injectedZones[dedupeKey] = true
							zoneRetryQueue = append(zoneRetryQueue, zoneRetryCandidate{
								spec:     r.zrSpec,
								zone:     z,
								imageId:  r.zrImage,
								rootDisk: r.zrDisk,
								lk:       r.lk,
							})
						}
					}
				}
			}
		}

		// Terminate NodeGroups over-provisioned by concurrent wave goroutines.
		// Run all terminations in parallel — sequential termination would multiply the
		// per-VM CSP teardown latency (~3 min on AWS) by the number of excess NodeGroups.
		if len(excessNodeGroups) > 0 {
			log.Info().Msgf("Terminating %d excess NodeGroups in parallel", len(excessNodeGroups))
			var termWg sync.WaitGroup
			for _, ngName := range excessNodeGroups {
				termWg.Add(1)
				go func(name string) {
					defer termWg.Done()
					log.Info().Msgf("Terminating excess NodeGroup '%s'", name)
					if termErr := terminateNodeGroup(nsId, infraId, name); termErr != nil {
						log.Warn().Err(termErr).Msgf("failed to terminate excess NodeGroup '%s'", name)
					}
				}(ngName)
			}
			termWg.Wait()
			if _, refErr := HandleInfraAction(nsId, infraId, model.ActionRefine, true); refErr != nil {
				log.Warn().Err(refErr).Msg("refine after excess NodeGroup cleanup failed")
			}
		}
	}

	// Sequential path: only runs when effectiveParallelism <= 1.
	// seqFailed tracks main-pool candidates that yielded zero running nodes; each extends
	// the attempt budget by one so failures don't permanently reduce the provisioned count.
	seqFailed := 0
	for i, spec := range poolToIterate {
		if remaining <= 0 || effectiveParallelism > 1 {
			break
		}
		if i >= maxAttempts+seqFailed {
			break
		}
		// Stop launching new attempts once the run deadline (policy.TimeoutMinutes) passes.
		if ctx.Err() != nil {
			log.Warn().Msgf("nodeSpec '%s': stopping candidate attempts — %v", ns.Name, ctx.Err())
			break
		}

		lk := locationKey(spec)

		// maxPerLocation: enforce cumulative per-region cap.
		if ns.MaxPerLocation > 0 && locationNodeCount[lk] >= ns.MaxPerLocation {
			continue
		}

		// spread: after filling all unique locations once, allow reuse only if needed.
		if strategy == "spread" && usedLocations[lk] && len(usedLocations) < remaining {
			// Prefer not reusing — skip if there are still fresh locations in the pool.
			// (This condition is only reached in secondPass portion.)
			hasUnused := false
			for _, s2 := range poolToIterate[i+1:] {
				if !usedLocations[locationKey(s2)] {
					hasUnused = true
					break
				}
			}
			if hasUnused {
				continue
			}
		}

		startedAt := time.Now().Format(time.RFC3339)

		// Determine how many nodes to request in this group.
		requested := remaining
		if ns.MaxPerLocation > 0 {
			locationRemaining := ns.MaxPerLocation - locationNodeCount[lk]
			if requested > locationRemaining {
				requested = locationRemaining
			}
		}
		if strategy == "spread" && requested > 1 {
			requested = 1
		}

		attempt := model.ProvisioningAttempt{
			NodeSpecName:        ns.Name,
			SpecId:              spec.Id,
			ConnectionName:      spec.ConnectionName,
			RequestedCount:      requested,
			PoolIndex:           i,
			StartedAt:           startedAt,
			CostPerHour:         float64(spec.CostPerHour),
			AcceleratorModel:    spec.AcceleratorModel,
			AcceleratorCount:    int(spec.AcceleratorCount),
			AcceleratorMemoryGB: spec.AcceleratorMemoryGB,
		}

		var zone, rootDiskType, nodeGroupName, imageId string

		if pr, ok := preResolvedByIndex[i]; ok {
			// Pre-validated by plan: skip SearchImage + ReviewSpecImagePair.
			imageId = pr.imageId
			zone = pr.zone
			rootDiskType = pr.rootDisk
			nodeGroupName = pr.ngName
			attempt.ImageId = imageId
			attempt.RiskLevel = "low"
			if zone != "" {
				attempt.Zone = zone
				attempt.ZoneOverridden = true
			}
			if rootDiskType != "" && rootDiskType != ns.RootDiskType {
				attempt.DiskOverridden = true
			}
		} else {
			// Fallback: resolve image and review pair on the fly.
			osType := buildOSType(ns.ImageRequirement)
			images, _, imgErr := provisionSearchImage(model.SystemCommonNs, model.SearchImageRequest{
				MatchedSpecId: spec.Id,
				ProviderName:  spec.ProviderName,
				RegionName:    spec.RegionName,
				OSType:        osType,
				IsGPUImage:    ns.ImageRequirement.IsGPUImage,
			}, false)
			if imgErr != nil || len(images) == 0 {
				reason := "no matching image found"
				if imgErr != nil {
					reason = imgErr.Error()
				}
				attempt.Status = "failed"
				attempt.FailureReason = reason
				attempt.ReviewRejected = true
				attempt.CompletedAt = time.Now().Format(time.RFC3339)
				recordAttempt(attempt)
				seqFailed++
				continue
			}
			imageId = images[0].Id
			attempt.ImageId = imageId

			pairResult, pairErr := provisionReviewPair(ctx, spec.Id, imageId, ns.RootDiskType, "")
			if pairErr != nil {
				attempt.Status = "failed"
				attempt.FailureReason = pairErr.Error()
				attempt.ReviewRejected = true
				attempt.CompletedAt = time.Now().Format(time.RFC3339)
				recordAttempt(attempt)
				seqFailed++
				continue
			}
			if !pairResult.IsValid {
				attempt.Status = "failed"
				attempt.FailureReason = strings.Join(pairResult.Errors, "; ")
				attempt.ReviewRejected = true
				attempt.CompletedAt = time.Now().Format(time.RFC3339)
				recordAttempt(attempt)
				seqFailed++
				continue
			}

			zone = pairResult.SuggestedZone
			rootDiskType = ns.RootDiskType
			if pairResult.SuggestedSystemDisk != "" && rootDiskType == "" {
				rootDiskType = pairResult.SuggestedSystemDisk
				attempt.DiskOverridden = true
			}
			if zone != "" {
				attempt.Zone = zone
				attempt.ZoneOverridden = true
			}
			attempt.RiskLevel = "low"
			if len(pairResult.Warnings) > 0 {
				attempt.RiskLevel = "medium"
			}

			nodeGroupIdx++
			nodeGroupName = fmt.Sprintf("%s-%s-%d", ns.Name, sanitizeForName(spec.ProviderName), nodeGroupIdx)
		}

		attempt.NodeGroupName = nodeGroupName

		ngReq := model.NodeGroupDynamicReq{
			Name:          nodeGroupName,
			NodeGroupSize: requested,
			SpecId:        spec.Id,
			ImageId:       imageId,
			RootDiskType:  rootDiskType,
			RootDiskSize:  ns.RootDiskSize,
			Zone:          zone,
			Label:         ns.Label,
		}

		// Attempt CSP provisioning.
		// actualRunning is the number of VMs that reached Running state in this NodeGroup.
		// provision() handles cleanup (refine) of Failed/Undefined nodes internally.
		actualRunning, provErr := provision(ngReq)
		if provErr != nil {
			attempt.Status = "csp-failed"
			attempt.FailureReason = provErr.Error()
			attempt.CompletedAt = time.Now().Format(time.RFC3339)

			// Credit any VMs that survived despite the overall failure (partial NodeGroup success).
			// provision() already refined away the failed nodes.
			if actualRunning > 0 {
				attempt.SucceededCount = actualRunning
				attempt.TrimmedCount = requested - actualRunning
				specResult.ProvisionedCount += actualRunning
				specResult.NodeGroupIds = append(specResult.NodeGroupIds, nodeGroupName)
				specResult.LocationsUsed = append(specResult.LocationsUsed, spec.ConnectionName)
				locationNodeCount[lk] += actualRunning
				usedLocations[lk] = true
				remaining -= actualRunning
				log.Info().Msgf(
					"NodeGroup '%s': partial csp-failed — %d/%d VMs running, credited to quota",
					nodeGroupName, actualRunning, requested,
				)
			}

			recordAttempt(attempt)
			log.Warn().Err(provErr).Msgf(
				"CSP provisioning failed for nodeGroup '%s' (spec: %s)",
				nodeGroupName, spec.Id,
			)

			// If partial survivors already satisfied the remaining quota, no zone retry needed.
			if remaining <= 0 {
				break
			}

			// Zone-level retry: only for availability failures where a specific zone was used.
			// Enumerate available zones from cloudinfo.yaml and try each explicitly.
			// (zone="" is not supported — cb-spider has removed CSP zone auto-selection.)
			if !isAvailabilityFailure(provErr.Error()) || zone == "" {
				log.Warn().Msgf("Skipping to next candidate (non-availability error or zone already unspecified)")
				seqFailed++
				continue
			}

			zonesInfo, zonesErr := provisionGetZones(ctx, spec.Id)
			if zonesErr != nil || !zonesInfo.HasZoneConcept || len(zonesInfo.AvailableZones) == 0 {
				log.Warn().Msgf("No zone info for spec '%s'; skipping to next candidate", spec.Id)
				seqFailed++
				continue
			}

			for _, retryZone := range zonesInfo.AvailableZones {
				if remaining <= 0 {
					break
				}
				if strings.EqualFold(retryZone, zone) {
					continue // skip the zone that already failed
				}

				nodeGroupIdx++
				retryName := fmt.Sprintf("%s-%s-%d", ns.Name, sanitizeForName(spec.ProviderName), nodeGroupIdx)
				retryNodeCount := remaining
				if ns.MaxPerLocation > 0 {
					locRem := ns.MaxPerLocation - locationNodeCount[lk]
					if locRem <= 0 {
						break // this location is already at its cap
					}
					if retryNodeCount > locRem {
						retryNodeCount = locRem
					}
				}
				if strategy == "spread" && retryNodeCount > 1 {
					retryNodeCount = 1
				}

				retryAttempt := model.ProvisioningAttempt{
					NodeSpecName:   ns.Name,
					SpecId:         spec.Id,
					ConnectionName: spec.ConnectionName,
					RequestedCount: retryNodeCount,
					PoolIndex:      i,
					StartedAt:      time.Now().Format(time.RFC3339),
					CostPerHour:    float64(spec.CostPerHour),
					ImageId:        imageId,
					NodeGroupName:  retryName,
					RiskLevel:      attempt.RiskLevel,
					DiskOverridden: attempt.DiskOverridden,
					Zone:           retryZone,
					ZoneOverridden: true,
				}

				retryReq := ngReq
				retryReq.Name = retryName
				retryReq.Zone = retryZone
				retryReq.NodeGroupSize = retryNodeCount

				log.Info().Msgf("Zone retry for spec '%s': trying zone '%s' (%d nodes, nodeGroup: %s)", spec.Id, retryZone, remaining, retryName)

				actualRetryRunning, retryErr := provision(retryReq)
				retryAttempt.CompletedAt = time.Now().Format(time.RFC3339)
				retryAttempt.SucceededCount = actualRetryRunning
				retryAttempt.TrimmedCount = remaining - actualRetryRunning

				if retryErr != nil {
					retryAttempt.Status = "csp-failed"
					retryAttempt.FailureReason = retryErr.Error()

					if actualRetryRunning > 0 {
						specResult.ProvisionedCount += actualRetryRunning
						specResult.NodeGroupIds = append(specResult.NodeGroupIds, retryName)
						specResult.LocationsUsed = append(specResult.LocationsUsed, spec.ConnectionName+"/"+retryZone)
						locationNodeCount[lk] += actualRetryRunning
						usedLocations[lk] = true
						remaining -= actualRetryRunning
						log.Info().Msgf("NodeGroup '%s': partial zone-retry failed — %d/%d VMs running, credited", retryName, actualRetryRunning, retryAttempt.RequestedCount)
					}

					recordAttempt(retryAttempt)
					log.Warn().Err(retryErr).Msgf("Zone retry (zone=%s) failed for nodeGroup '%s'; trying next zone", retryZone, retryName)
					continue // try next zone
				}

				// This zone retry succeeded.
				retryAttempt.Status = "succeeded"
				recordAttempt(retryAttempt)
				specResult.ProvisionedCount += actualRetryRunning
				specResult.NodeGroupIds = append(specResult.NodeGroupIds, retryName)
				specResult.LocationsUsed = append(specResult.LocationsUsed, spec.ConnectionName+"/"+retryZone)
				locationNodeCount[lk] += actualRetryRunning
				usedLocations[lk] = true
				remaining -= actualRetryRunning
			}
			// After exhausting all zones for this spec, advance to next candidate.
			// Primary had actualRunning==0; zone retries may have partially helped but
			// the slot still counts as a failure for budget extension purposes.
			if actualRunning == 0 {
				seqFailed++
			}
			continue
		}

		// Primary provision succeeded.
		attempt.Status = "succeeded"
		attempt.SucceededCount = actualRunning
		attempt.TrimmedCount = requested - actualRunning
		attempt.CompletedAt = time.Now().Format(time.RFC3339)
		recordAttempt(attempt)

		specResult.ProvisionedCount += actualRunning
		specResult.NodeGroupIds = append(specResult.NodeGroupIds, nodeGroupName)
		loc := spec.ConnectionName
		if zone != "" {
			loc = spec.ConnectionName + "/" + zone
		}
		specResult.LocationsUsed = append(specResult.LocationsUsed, loc)
		locationNodeCount[lk] += actualRunning
		usedLocations[lk] = true
		remaining -= actualRunning
	}

	specResult.Fulfilled = specResult.ProvisionedCount >= ns.DesiredCount
	minCount := ns.MinCount
	if minCount <= 0 {
		minCount = 1
	}
	specResult.MinFulfilled = specResult.ProvisionedCount >= minCount

	return specResult, attempts, nil
}

// countNodeGroupVMs returns the number of Running and non-Running final-state
// (Failed/Undefined) VMs in the named NodeGroup. Transitional states (Creating,
// empty) are ignored — provision() waits for all VMs to reach a final state before
// returning, so those should not appear in practice.
// A non-nil err means the count is unknown (listing or a record read failed),
// NOT that zero VMs exist. A node record that no longer exists (e.g. removed by
// a concurrent refine) is skipped and does not make the count unknown.
func countNodeGroupVMs(nsId, infraId, nodeGroupId string) (running, failed int, err error) {
	nodeIds, err := ListNodeByNodeGroup(nsId, infraId, nodeGroupId)
	if err != nil {
		return 0, 0, err
	}
	var readErr error
	for _, nodeId := range nodeIds {
		nodeInfo, gerr := GetNodeObject(nsId, infraId, nodeId)
		if gerr != nil {
			if strings.Contains(gerr.Error(), "no Node found") {
				continue // record legitimately gone; the remaining count stays reliable
			}
			// Store/unmarshal failure: the node may well exist — count is unknown.
			if readErr == nil {
				readErr = fmt.Errorf("cannot read node '%s' in nodeGroup '%s': %w", nodeId, nodeGroupId, gerr)
			}
			continue
		}
		switch {
		case strings.EqualFold(nodeInfo.Status, model.StatusRunning):
			running++
		case strings.EqualFold(nodeInfo.Status, model.StatusFailed),
			strings.EqualFold(nodeInfo.Status, model.StatusUndefined):
			failed++
		}
	}
	return running, failed, readErr
}

// terminateNodeGroup terminates every node in a NodeGroup by calling HandleInfraNodeAction
// for each node. The caller is responsible for calling HandleInfraAction(ActionRefine)
// afterwards to purge the terminated nodes from the infra record.
func terminateNodeGroup(nsId, infraId, nodeGroupId string) error {
	nodeIds, err := ListNodeByNodeGroup(nsId, infraId, nodeGroupId)
	if err != nil {
		return fmt.Errorf("ListNodeByNodeGroup: %w", err)
	}
	var errs []string
	for _, nodeId := range nodeIds {
		if _, err := HandleInfraNodeAction(nsId, infraId, nodeId, model.ActionTerminate, true); err != nil {
			errs = append(errs, nodeId+": "+err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("terminate failed for %d/%d nodes: %s", len(errs), len(nodeIds), strings.Join(errs, "; "))
	}
	return nil
}

// sanitizeForName returns a safe name component from a string.
func sanitizeForName(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	return s
}

// isAvailabilityFailure reports whether a CSP error is due to transient zone/region
// capacity shortage, making it a candidate for zone-level retry.
//
// Only zone-specific shortages qualify — account-level quota errors (InstanceLimitExceeded,
// VcpuLimitExceeded, etc.) are NOT included because a different zone in the same account
// would hit the same limit.
//
// Sources:
//   GCP: ZONE_RESOURCE_POOL_EXHAUSTED / ZONE_RESOURCE_POOL_EXHAUSTED_WITH_DETAILS
//        https://cloud.google.com/compute/docs/troubleshooting/troubleshooting-resource-availability
//   AWS: InsufficientInstanceCapacity, InsufficientCapacity, UnfulfillableCapacity
//        https://docs.aws.amazon.com/AWSEC2/latest/APIReference/errors-overview.html
//   Tencent: ResourceInsufficient.SpecifiedInstanceType (handled by pre-flight checker)
//   Alibaba: similar stock/capacity messages
func isAvailabilityFailure(errMsg string) bool {
	lower := strings.ToLower(errMsg)

	// Zone-specific capacity keywords (GCP / Alibaba / Tencent / generic)
	zoneCapacityKeywords := []string{
		"not enough resources",       // GCP: "does not have enough resources available"
		"resource type:compute",      // GCP: ZONE_RESOURCE_POOL_EXHAUSTED detail
		"zone_resource_pool_exhausted", // GCP: error code in message
		"resourceinsufficient",       // Tencent: ResourceInsufficient.SpecifiedInstanceType
		"out of stock",               // Alibaba / generic
		"stock",                      // Alibaba: SOLD_OUT stock status
		"insufficient_resources",     // generic
		"no available",               // generic
		"resourcesexhausted",         // gRPC: RESOURCE_EXHAUSTED mapped by some CSPs
	}

	// AWS-specific capacity keywords (zone-level, not account quota)
	// "InsufficientInstanceCapacity" message: "sufficient capacity in the Availability Zone"
	// "InsufficientCapacity" / "UnfulfillableCapacity": contain "capacity"
	// Excluded: InstanceLimitExceeded, VcpuLimitExceeded (account quota, not zone-specific)
	awsCapacityKeywords := []string{
		"insufficientinstancecapacity", // AWS error code (lower-cased)
		"insufficient capacity",        // AWS: "do not have sufficient capacity"
		"unfulfillablecapacity",        // AWS Spot: UnfulfillableCapacity
	}

	for _, kw := range zoneCapacityKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	for _, kw := range awsCapacityKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// GetInfraAutopilotStatus returns a live status snapshot of an in-flight autopilot run.
// While a run is active, it counts Running VMs per NodeSpec by scanning NodeGroups whose
// names start with "{nodeSpecName}-". Falls back to a bare infra-status-only response
// when no active run is registered (e.g. the infra exists but autopilot has finished).
func GetInfraAutopilotStatus(nsId string, infraId string) (*model.InfraAutopilotStatus, error) {
	infraInfo, err := GetInfraInfo(nsId, infraId)
	if err != nil {
		return nil, fmt.Errorf("GetInfraInfo failed for '%s': %w", infraId, err)
	}

	status := &model.InfraAutopilotStatus{
		InfraId:     infraId,
		InfraStatus: infraInfo.Status,
		Specs:       []model.NodeSpecStatus{},
	}

	runVal, active := activeAutopilotRuns.Load(autopilotRunKey(nsId, infraId))
	if !active {
		return status, nil
	}
	run := runVal.(*activeAutopilotRun)
	status.ElapsedSeconds = int64(time.Since(run.startTime).Seconds())

	ngIds, err := ListNodeGroupId(nsId, infraId)
	if err != nil {
		ngIds = nil
	}

	// Assign each NodeGroup to the NodeSpec with the LONGEST matching name prefix.
	// A plain prefix scan would double-count when one NodeSpec name is a prefix of
	// another (e.g. "web" would absorb "web-gpu-aws-1", which belongs to "web-gpu").
	ngOwner := make(map[string]string, len(ngIds))
	for _, ngId := range ngIds {
		bestLen := -1
		for _, ns := range run.req.NodeSpecs {
			prefix := ns.Name + "-"
			if strings.HasPrefix(ngId, prefix) && len(prefix) > bestLen {
				bestLen = len(prefix)
				ngOwner[ngId] = ns.Name
			}
		}
	}

	for _, ns := range run.req.NodeSpecs {
		running := 0
		for _, ngId := range ngIds {
			if ngOwner[ngId] == ns.Name {
				r, _, _ := countNodeGroupVMs(nsId, infraId, ngId)
				running += r
			}
		}
		specStatus := model.NodeSpecStatus{
			NodeSpecName:     ns.Name,
			DesiredCount:     ns.DesiredCount,
			ProvisionedCount: running,
			Status:           "provisioning",
		}
		if running >= ns.DesiredCount {
			specStatus.Status = "fulfilled"
		}

		// Attach live attempt log for this NodeSpec.
		run.mu.Lock()
		if recorded, ok := run.attemptsBySpec[ns.Name]; ok {
			specStatus.Attempts = append([]model.ProvisioningAttempt{}, recorded...)
		}
		run.mu.Unlock()

		status.Specs = append(status.Specs, specStatus)
	}

	return status, nil
}
