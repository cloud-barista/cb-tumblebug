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
	"runtime"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/rs/zerolog/log"
)

const (
	// kvstore key prefix for scheduled jobs
	keyScheduledJob = "/scheduledJob"

	// Default execution timeout (30 minutes)
	// If a job stays in Executing status longer than this, it's considered stuck
	defaultExecutionTimeout = 30 * time.Minute

	// Max consecutive failures before auto-disabling job
	maxConsecutiveFailures = 5
)

// JobType represents the type of scheduled job
type JobType string

const (
	// JobTypeRegisterCspResources represents CSP resource registration job
	JobTypeRegisterCspResources JobType = "registerCspResources"
	// JobTypeRegisterCspResourcesAll represents all CSP resources registration job
	JobTypeRegisterCspResourcesAll JobType = "registerCspResourcesAll"
)

// JobStatus represents the current status of a scheduled job
type JobStatus string

const (
	// JobStatusRunning indicates the job is actively running
	JobStatusRunning JobStatus = "Running"
	// JobStatusStopped indicates the job has been stopped
	JobStatusStopped JobStatus = "Stopped"
	// JobStatusExecuting indicates the job is currently executing a task
	JobStatusExecuting JobStatus = "Executing"
	// JobStatusScheduled indicates the job is scheduled and waiting for the next execution time
	JobStatusScheduled JobStatus = "Scheduled"
)

// ScheduledJob represents a scheduled recurring job
type ScheduledJob struct {
	// Job identification
	JobId     string    `json:"jobId"`
	JobType   JobType   `json:"jobType"`
	NsId      string    `json:"nsId"`
	CreatedAt time.Time `json:"createdAt"`

	// Job configuration
	IntervalSeconds  int  `json:"intervalSeconds"`  // Interval between executions in seconds
	ExecutionTimeout int  `json:"executionTimeout"` // Max execution time in seconds (0 = use default)
	Enabled          bool `json:"enabled"`

	// Job-specific parameters
	ConnectionName string `json:"connectionName,omitempty"` // For registerCspResources
	MciNamePrefix  string `json:"mciNamePrefix,omitempty"`  // For registerCspResources
	Option         string `json:"option,omitempty"`         // For registerCspResources
	MciFlag        string `json:"mciFlag,omitempty"`        // For registerCspResources

	// Job status
	Status              JobStatus `json:"status"`
	LastExecutedAt      time.Time `json:"lastExecutedAt,omitempty"`
	NextExecutionAt     time.Time `json:"nextExecutionAt,omitempty"`
	ExecutionCount      int       `json:"executionCount"`
	SuccessCount        int       `json:"successCount"`        // Total successful executions
	FailureCount        int       `json:"failureCount"`        // Total failed executions
	ConsecutiveFailures int       `json:"consecutiveFailures"` // Current consecutive failures
	LastError           string    `json:"lastError,omitempty"`
	LastResult          string    `json:"lastResult,omitempty"`
	AutoDisabled        bool      `json:"autoDisabled"` // Whether job was auto-disabled due to failures

	// Internal control
	ctx        context.Context
	cancelFunc context.CancelFunc
	ticker     *time.Ticker
	mu         sync.RWMutex
}

// SchedulerManager manages all scheduled jobs
type SchedulerManager struct {
	jobs map[string]*ScheduledJob
	mu   sync.RWMutex
}

var (
	schedulerManager *SchedulerManager
	schedulerOnce    sync.Once
)

// GetSchedulerManager returns the singleton scheduler manager instance
func GetSchedulerManager() *SchedulerManager {
	schedulerOnce.Do(func() {
		schedulerManager = &SchedulerManager{
			jobs: make(map[string]*ScheduledJob),
		}
		// Load persisted jobs from kvstore on initialization
		if err := schedulerManager.loadJobsFromStore(); err != nil {
			log.Error().Err(err).Msg("Failed to load jobs from kvstore, starting with empty scheduler")
		}
		log.Info().Msgf("Scheduler manager initialized with %d jobs", len(schedulerManager.jobs))
	})
	return schedulerManager
}

// genJobKey generates kvstore key for a scheduled job
// Jobs are not scoped to namespaces, only jobId is used for key
func genJobKey(jobId string) string {
	return fmt.Sprintf("%s/%s", keyScheduledJob, jobId)
}

// saveJobToStore persists job to kvstore
func (sm *SchedulerManager) saveJobToStore(job *ScheduledJob) error {
	key := genJobKey(job.JobId)

	// Create a persistable version (without internal fields)
	persistJob := &ScheduledJob{
		JobId:               job.JobId,
		JobType:             job.JobType,
		NsId:                job.NsId,
		CreatedAt:           job.CreatedAt,
		IntervalSeconds:     job.IntervalSeconds,
		ExecutionTimeout:    job.ExecutionTimeout,
		Enabled:             job.Enabled,
		ConnectionName:      job.ConnectionName,
		MciNamePrefix:       job.MciNamePrefix,
		Option:              job.Option,
		MciFlag:             job.MciFlag,
		Status:              job.Status,
		LastExecutedAt:      job.LastExecutedAt,
		NextExecutionAt:     job.NextExecutionAt,
		ExecutionCount:      job.ExecutionCount,
		SuccessCount:        job.SuccessCount,
		FailureCount:        job.FailureCount,
		ConsecutiveFailures: job.ConsecutiveFailures,
		LastError:           job.LastError,
		LastResult:          job.LastResult,
		AutoDisabled:        job.AutoDisabled,
	}

	val, err := json.Marshal(persistJob)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	if err := kvstore.Put(key, string(val)); err != nil {
		return fmt.Errorf("failed to store job to kvstore: %w", err)
	}

	return nil
}

// loadJobsFromStore loads all persisted jobs from kvstore and recovers them
func (sm *SchedulerManager) loadJobsFromStore() error {
	keyPrefix := keyScheduledJob + "/"
	keyValues, err := kvstore.GetKvList(keyPrefix)
	if err != nil {
		return fmt.Errorf("failed to list jobs from kvstore: %w", err)
	}

	recoveredCount := 0
	for _, kv := range keyValues {
		var job ScheduledJob
		if err := json.Unmarshal([]byte(kv.Value), &job); err != nil {
			log.Error().Err(err).Str("key", kv.Key).Msg("Failed to unmarshal job, skipping")
			continue
		}

		// Recover job based on its previous status
		if err := sm.recoverJob(&job); err != nil {
			log.Error().Err(err).Str("jobId", job.JobId).Msg("Failed to recover job")
			continue
		}

		recoveredCount++
	}

	log.Info().Msgf("Recovered %d scheduled jobs from kvstore", recoveredCount)
	return nil
}

// recoverJob handles job recovery logic based on previous state
func (sm *SchedulerManager) recoverJob(job *ScheduledJob) error {
	// Check if job was in Executing state - could be interrupted or stuck
	if job.Status == JobStatusExecuting {
		// Calculate how long the job has been in Executing state
		executionTimeout := time.Duration(job.ExecutionTimeout) * time.Second
		if executionTimeout == 0 {
			executionTimeout = defaultExecutionTimeout
		}

		timeInExecuting := time.Since(job.LastExecutedAt)

		if timeInExecuting > executionTimeout {
			// Job was stuck - execution took too long
			log.Warn().Str("jobId", job.JobId).
				Dur("timeInExecuting", timeInExecuting).
				Dur("timeout", executionTimeout).
				Msg("Job was stuck in Executing state (timeout), marking as failed")

			job.LastError = fmt.Sprintf("Job execution timeout after %s (exceeded %s limit)",
				timeInExecuting.Round(time.Second), executionTimeout)
		} else {
			// Job was interrupted by server restart (not stuck, just unlucky timing)
			log.Warn().Str("jobId", job.JobId).
				Dur("timeInExecuting", timeInExecuting).
				Msg("Job was executing during server shutdown, marking as failed and resetting")

			job.LastError = "Job interrupted by server restart"
		}

		job.LastResult = ""
		job.Status = JobStatusScheduled
		// Keep execution count and last execution time as-is for audit trail
	}

	// Re-create runtime context and ticker
	ctx, cancel := context.WithCancel(context.Background())
	job.ctx = ctx
	job.cancelFunc = cancel

	// Store recovered job
	sm.jobs[job.JobId] = job

	// Restart job execution goroutine if enabled
	if job.Enabled {
		go job.start()
		log.Info().Str("jobId", job.JobId).
			Str("status", string(job.Status)).
			Int("execCount", job.ExecutionCount).
			Msg("Recovered and restarted scheduled job")
	} else {
		log.Info().Str("jobId", job.JobId).
			Msg("Recovered job but not starting (disabled)")
	}

	return nil
}

// deleteJobFromStore removes job from kvstore
func (sm *SchedulerManager) deleteJobFromStore(jobId string) error {
	key := genJobKey(jobId)
	if err := kvstore.Delete(key); err != nil {
		return fmt.Errorf("failed to delete job from kvstore: %w", err)
	}
	return nil
}

// findDuplicateJob checks if a job with same configuration already exists
func (sm *SchedulerManager) findDuplicateJob(req model.ScheduleJobRequest) (*ScheduledJob, bool) {
	for _, job := range sm.jobs {
		// Check if job has same configuration (regardless of jobId)
		if job.NsId == req.NsId &&
			string(job.JobType) == req.JobType &&
			job.ConnectionName == req.ConnectionName &&
			job.MciNamePrefix == req.MciNamePrefix &&
			job.Option == req.Option &&
			job.MciFlag == req.MciFlag {
			return job, true
		}
	}
	return nil, false
}

// CreateScheduledJob creates a new scheduled job
func (sm *SchedulerManager) CreateScheduledJob(req model.ScheduleJobRequest) (*ScheduledJob, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check for duplicate job configuration
	if existingJob, isDuplicate := sm.findDuplicateJob(req); isDuplicate {
		return nil, fmt.Errorf("duplicate job already exists: %s (jobType=%s, nsId=%s, connectionName=%s, mciNamePrefix=%s, option=%s, mciFlag=%s)",
			existingJob.JobId, existingJob.JobType, existingJob.NsId,
			existingJob.ConnectionName, existingJob.MciNamePrefix, existingJob.Option, existingJob.MciFlag)
	}

	minimumInterval := 10

	// Validate input
	if req.IntervalSeconds < minimumInterval {
		return nil, fmt.Errorf("interval must be at least %d seconds", minimumInterval)
	}

	// Validate option parameters early (before job creation)
	if req.JobType == "registerCspResources" || req.JobType == "registerCspResourcesAll" {
		if _, err := getValidatedOptionMap(req.Option); err != nil {
			return nil, err
		}
	}

	// Generate job ID
	jobId := fmt.Sprintf("%s-%s-%d", req.JobType, req.NsId, time.Now().Unix())

	// Check if job already exists
	if _, exists := sm.jobs[jobId]; exists {
		return nil, fmt.Errorf("job with id %s already exists", jobId)
	}

	// Create job
	ctx, cancel := context.WithCancel(context.Background())
	now := time.Now()
	job := &ScheduledJob{
		JobId:           jobId,
		JobType:         JobType(req.JobType),
		NsId:            req.NsId,
		CreatedAt:       now,
		IntervalSeconds: req.IntervalSeconds,
		Enabled:         true,
		ConnectionName:  req.ConnectionName,
		MciNamePrefix:   req.MciNamePrefix,
		Option:          req.Option,
		MciFlag:         req.MciFlag,
		Status:          JobStatusScheduled,
		NextExecutionAt: now.Add(time.Duration(req.IntervalSeconds) * time.Second),
		ctx:             ctx,
		cancelFunc:      cancel,
	}

	// Store job in memory
	sm.jobs[jobId] = job

	// Persist job to kvstore
	if err := sm.saveJobToStore(job); err != nil {
		// Rollback memory storage if kvstore fails
		delete(sm.jobs, jobId)
		cancel()
		return nil, fmt.Errorf("failed to persist job: %w", err)
	}

	// Start job execution
	go job.start()

	log.Info().Msgf("Created scheduled job: %s (type: %s, interval: %ds, next execution: %s)",
		jobId, req.JobType, req.IntervalSeconds, job.NextExecutionAt.Format(time.RFC3339))

	return job, nil
}

// GetScheduledJob retrieves a scheduled job by ID
func (sm *SchedulerManager) GetScheduledJob(jobId string) (*ScheduledJob, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	job, exists := sm.jobs[jobId]
	if !exists {
		return nil, fmt.Errorf("job not found: %s", jobId)
	}

	return job, nil
}

// ListScheduledJobs returns all scheduled jobs
// Jobs are not scoped to namespaces, so all jobs are returned
func (sm *SchedulerManager) ListScheduledJobs() []*ScheduledJob {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	jobs := make([]*ScheduledJob, 0, len(sm.jobs))
	for _, job := range sm.jobs {
		jobs = append(jobs, job)
	}

	return jobs
}

// StopScheduledJob stops a scheduled job
func (sm *SchedulerManager) StopScheduledJob(jobId string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	job, exists := sm.jobs[jobId]
	if !exists {
		return fmt.Errorf("job not found: %s", jobId)
	}

	// Stop job execution
	job.stop()

	// Delete from kvstore
	if err := sm.deleteJobFromStore(jobId); err != nil {
		log.Error().Err(err).Str("jobId", jobId).Msg("Failed to delete job from kvstore, continuing with memory cleanup")
	}

	// Remove from memory
	delete(sm.jobs, jobId)

	log.Info().Msgf("Stopped and removed scheduled job: %s", jobId)

	return nil
}

// StopAllScheduledJobs stops all scheduled jobs in the system
// Jobs are not scoped to namespaces, so all jobs in the system are deleted
func (sm *SchedulerManager) StopAllScheduledJobs() (int, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Collect all jobs to delete
	jobsToDelete := make([]string, 0, len(sm.jobs))
	for jobId := range sm.jobs {
		jobsToDelete = append(jobsToDelete, jobId)
	}

	if len(jobsToDelete) == 0 {
		return 0, nil
	}

	deletedCount := 0
	var lastErr error

	// Stop and delete each job
	for _, jobId := range jobsToDelete {
		job := sm.jobs[jobId]

		// Stop job execution
		job.stop()

		// Delete from kvstore
		if err := sm.deleteJobFromStore(jobId); err != nil {
			log.Error().Err(err).Str("jobId", jobId).Msg("Failed to delete job from kvstore, continuing with cleanup")
			lastErr = err
		}

		// Remove from memory
		delete(sm.jobs, jobId)
		deletedCount++

		log.Info().Msgf("Stopped and removed scheduled job: %s", jobId)
	}

	log.Warn().Msgf("Stopped and removed all %d scheduled jobs", deletedCount)

	return deletedCount, lastErr
}

// UpdateScheduledJob updates job configuration
func (sm *SchedulerManager) UpdateScheduledJob(jobId string, req model.UpdateScheduleJobRequest) (*ScheduledJob, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	job, exists := sm.jobs[jobId]
	if !exists {
		return nil, fmt.Errorf("job not found: %s", jobId)
	}

	job.mu.Lock()
	defer job.mu.Unlock()

	// Update interval if provided
	if req.IntervalSeconds != nil && *req.IntervalSeconds >= 60 {
		job.IntervalSeconds = *req.IntervalSeconds
		// Restart ticker with new interval
		if job.ticker != nil {
			job.ticker.Reset(time.Duration(job.IntervalSeconds) * time.Second)
		}
		log.Info().Msgf("Updated job %s interval to %ds", jobId, job.IntervalSeconds)
	}

	// Update enabled status if provided
	if req.Enabled != nil {
		job.Enabled = *req.Enabled
		log.Info().Msgf("Updated job %s enabled status to %t", jobId, job.Enabled)
	}

	// Persist updated job to kvstore
	if err := sm.saveJobToStore(job); err != nil {
		log.Error().Err(err).Str("jobId", jobId).Msg("Failed to persist job update")
		return nil, fmt.Errorf("failed to persist job update: %w", err)
	}

	return job, nil
}

// start begins the scheduled job execution loop
func (job *ScheduledJob) start() {
	log.Info().Msgf("Starting scheduled job: %s (next execution: %s)",
		job.JobId, job.NextExecutionAt.Format(time.RFC3339))

	job.mu.Lock()
	job.Status = JobStatusScheduled
	job.ticker = time.NewTicker(time.Duration(job.IntervalSeconds) * time.Second)
	job.mu.Unlock()

	// Execute immediately on start
	if job.Enabled {
		log.Info().Msgf("Executing job immediately on start: %s", job.JobId)
		job.execute()
	}

	for {
		select {
		case <-job.ctx.Done():
			log.Info().Msgf("Scheduled job stopped: %s", job.JobId)
			return

		case <-job.ticker.C:
			if job.Enabled {
				job.execute()
			} else {
				log.Debug().Msgf("Job %s is disabled, skipping execution", job.JobId)
			}

			// Update next execution time
			job.mu.Lock()
			job.NextExecutionAt = time.Now().Add(time.Duration(job.IntervalSeconds) * time.Second)
			job.mu.Unlock()
		}
	}
}

// stop halts the scheduled job
func (job *ScheduledJob) stop() {
	job.mu.Lock()
	defer job.mu.Unlock()

	if job.ticker != nil {
		job.ticker.Stop()
	}
	if job.cancelFunc != nil {
		job.cancelFunc()
	}
	job.Status = JobStatusStopped

	log.Info().Msgf("Job stopped: %s", job.JobId)
}

// execute runs the actual job task with timeout protection
func (job *ScheduledJob) execute() {
	// Memory monitoring - capture stats before execution
	var memBefore, memAfter runtime.MemStats
	runtime.ReadMemStats(&memBefore)
	goroutinesBefore := runtime.NumGoroutine()

	// Panic recovery to prevent job from getting stuck
	defer func() {
		// Memory monitoring - capture stats after execution
		runtime.ReadMemStats(&memAfter)
		goroutinesAfter := runtime.NumGoroutine()

		// Calculate memory delta
		allocDelta := int64(memAfter.Alloc) - int64(memBefore.Alloc)
		goroutineDelta := goroutinesAfter - goroutinesBefore

		// Log memory stats
		log.Debug().
			Str("jobId", job.JobId).
			Uint64("allocMB", memAfter.Alloc/1024/1024).
			Int64("allocDeltaKB", allocDelta/1024).
			Uint32("numGoroutine", uint32(goroutinesAfter)).
			Int("goroutineDelta", goroutineDelta).
			Msg("Job execution memory stats")

		// Warn if suspicious memory growth
		if allocDelta > 50*1024*1024 { // > 50MB growth
			log.Warn().
				Str("jobId", job.JobId).
				Int64("allocDeltaMB", allocDelta/1024/1024).
				Msg("Large memory allocation detected during job execution")
		}

		// Warn if goroutine leak suspected
		if goroutineDelta > 5 {
			log.Warn().
				Str("jobId", job.JobId).
				Int("goroutineDelta", goroutineDelta).
				Uint32("totalGoroutines", uint32(goroutinesAfter)).
				Msg("Possible goroutine leak detected - goroutines increased")
		}

		if r := recover(); r != nil {
			job.mu.Lock()
			job.LastError = fmt.Sprintf("Job panic: %v", r)
			job.LastResult = ""
			job.Status = JobStatusScheduled
			job.mu.Unlock()

			log.Error().Str("jobId", job.JobId).Interface("panic", r).Msg("Job execution panicked")

			// Persist panic status
			sm := GetSchedulerManager()
			if err := sm.saveJobToStore(job); err != nil {
				log.Error().Err(err).Str("jobId", job.JobId).Msg("Failed to persist panic status")
			}
		}
	}()

	// Update status to Executing and persist
	job.mu.Lock()
	job.Status = JobStatusExecuting
	job.LastExecutedAt = time.Now()
	job.ExecutionCount++
	executionNum := job.ExecutionCount
	job.mu.Unlock()

	// Persist executing status to kvstore (for crash recovery)
	sm := GetSchedulerManager()
	if err := sm.saveJobToStore(job); err != nil {
		log.Error().Err(err).Str("jobId", job.JobId).Msg("Failed to persist executing status")
	}

	log.Info().Msgf("Executing job %s (execution #%d)", job.JobId, executionNum)

	// Create execution context with timeout
	executionTimeout := time.Duration(job.ExecutionTimeout) * time.Second
	if executionTimeout == 0 {
		executionTimeout = defaultExecutionTimeout
	}

	ctx, cancel := context.WithTimeout(job.ctx, executionTimeout)
	defer cancel()

	// Execute task in goroutine to support context cancellation
	type executionResult struct {
		result interface{}
		err    error
	}
	// Use buffered channel to prevent goroutine leak on timeout
	resultChan := make(chan executionResult, 1)

	go func() {
		// Panic recovery within goroutine
		defer func() {
			if r := recover(); r != nil {
				panicErr := fmt.Errorf("job execution panic: %v", r)
				log.Error().Str("jobId", job.JobId).Interface("panic", r).Msg("Job goroutine panicked")

				// Try to send panic error to channel, but don't block if channel is closed/full
				select {
				case resultChan <- executionResult{err: panicErr}:
				default:
					// Channel not ready (timeout already occurred), just log
					log.Warn().Str("jobId", job.JobId).Msg("Panic occurred but result channel not available")
				}
			}
		}()

		var result interface{}
		var err error

		// Execute based on job type
		switch job.JobType {
		case JobTypeRegisterCspResources:
			// Smart routing based on connectionName (similar to REST API handler)
			if job.ConnectionName == "" {
				// Route to all-connections handler
				result, err = RegisterCspNativeResourcesAll(
					job.NsId,
					job.MciNamePrefix,
					job.Option,
					job.MciFlag,
				)
			} else {
				// Route to specific connection handler
				result, err = RegisterCspNativeResources(
					job.NsId,
					job.ConnectionName,
					job.MciNamePrefix,
					job.Option,
					job.MciFlag,
				)
			}

		case JobTypeRegisterCspResourcesAll:
			result, err = RegisterCspNativeResourcesAll(
				job.NsId,
				job.MciNamePrefix,
				job.Option,
				job.MciFlag,
			)

		default:
			err = fmt.Errorf("unknown job type: %s", job.JobType)
		}

		// Send result to channel with context awareness to prevent goroutine leak
		select {
		case resultChan <- executionResult{result: result, err: err}:
			// Successfully sent result
		case <-ctx.Done():
			// Context cancelled/timed out while we were working
			// Don't block on channel send - just exit goroutine
			log.Warn().Str("jobId", job.JobId).Msg("Job completed but context already done, discarding result")
			return
		}
	}()

	// Wait for execution or timeout
	var execResult executionResult
	select {
	case execResult = <-resultChan:
		// Execution completed normally

	case <-ctx.Done():
		// Timeout or cancellation
		if ctx.Err() == context.DeadlineExceeded {
			execResult.err = fmt.Errorf("job execution timeout after %s", executionTimeout)
			log.Error().Str("jobId", job.JobId).Dur("timeout", executionTimeout).
				Msg("Job execution timed out - goroutine will exit when operation completes")
		} else {
			execResult.err = fmt.Errorf("job execution cancelled: %w", ctx.Err())
		}
	}

	// Update job status and persist
	job.mu.Lock()
	if execResult.err != nil {
		// Handle failure
		job.FailureCount++
		job.ConsecutiveFailures++
		job.LastError = execResult.err.Error()
		job.LastResult = ""

		log.Error().Err(execResult.err).
			Int("consecutiveFailures", job.ConsecutiveFailures).
			Int("totalFailures", job.FailureCount).
			Msgf("Job %s execution #%d failed", job.JobId, executionNum)

		// Auto-disable job if too many consecutive failures
		if job.ConsecutiveFailures >= maxConsecutiveFailures && !job.AutoDisabled {
			job.Enabled = false
			job.AutoDisabled = true

			log.Error().Str("jobId", job.JobId).
				Int("consecutiveFailures", job.ConsecutiveFailures).
				Msgf("Job auto-disabled due to %d consecutive failures", maxConsecutiveFailures)

			// Stop the ticker to prevent further executions
			if job.ticker != nil {
				job.ticker.Stop()
			}
		}
	} else {
		// Handle success
		job.SuccessCount++
		job.ConsecutiveFailures = 0 // Reset consecutive failures on success
		job.LastError = ""
		job.LastResult = fmt.Sprintf("Success (execution #%d)", executionNum)

		// Re-enable if was auto-disabled (recovery)
		if job.AutoDisabled {
			job.AutoDisabled = false
			log.Info().Str("jobId", job.JobId).
				Msg("Job recovered from auto-disabled state after successful execution")
		}

		log.Info().Int("successCount", job.SuccessCount).
			Msgf("Job %s execution #%d completed successfully", job.JobId, executionNum)
	}
	job.Status = JobStatusScheduled
	job.mu.Unlock()

	// Persist completion status to kvstore
	if err := sm.saveJobToStore(job); err != nil {
		log.Error().Err(err).Str("jobId", job.JobId).Msg("Failed to persist completion status")
	}

	// Log result summary if available
	if execResult.result != nil {
		log.Debug().Msgf("Job %s result: %+v", job.JobId, execResult.result)
	}
}

// GetStatus returns the current status of the job (thread-safe)
func (job *ScheduledJob) GetStatus() model.ScheduleJobStatus {
	job.mu.RLock()
	defer job.mu.RUnlock()

	return model.ScheduleJobStatus{
		JobId:               job.JobId,
		JobType:             string(job.JobType),
		NsId:                job.NsId,
		Status:              string(job.Status),
		IntervalSeconds:     job.IntervalSeconds,
		Enabled:             job.Enabled,
		CreatedAt:           job.CreatedAt,
		LastExecutedAt:      job.LastExecutedAt,
		NextExecutionAt:     job.NextExecutionAt,
		ExecutionCount:      job.ExecutionCount,
		SuccessCount:        job.SuccessCount,
		FailureCount:        job.FailureCount,
		ConsecutiveFailures: job.ConsecutiveFailures,
		AutoDisabled:        job.AutoDisabled,
		LastError:           job.LastError,
		LastResult:          job.LastResult,
		ConnectionName:      job.ConnectionName,
		MciNamePrefix:       job.MciNamePrefix,
		Option:              job.Option,
		MciFlag:             job.MciFlag,
	}
}
