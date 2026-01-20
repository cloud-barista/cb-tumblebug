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

// Package infra is to handle REST API for scheduled jobs
package infra

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// RestPostScheduleRegisterCspResources godoc
// @ID PostScheduleRegisterCspResources
// @Summary Create scheduled CSP resource registration job
// @Description Create a scheduled job to periodically register CSP-native resources (vNet, securityGroup, sshKey, vm) into CB-Tumblebug
// @Description
// @Description **Resource Registration Behavior:**
// @Description This job registers CSP-native resources based on the `connectionName` field:
// @Description - If `connectionName` is specified: Registers resources from the **specified connection only**
// @Description - If `connectionName` is empty or omitted: Registers resources from **all available connections**
// @Description
// @Description **Usage Examples:**
// @Description - Single connection: `{"jobType": "registerCspResources", "nsId": "default", "intervalSeconds": 60, "connectionName": "aws-ap-northeast-2", "mciNamePrefix": "mci-01"}`
// @Description - All connections: `{"jobType": "registerCspResources", "nsId": "default", "intervalSeconds": 60, "connectionName": "", "mciNamePrefix": "mci-all"}` or `{"jobType": "registerCspResources", "nsId": "default", "intervalSeconds": 60, "mciNamePrefix": "mci-all"}`
// @Description
// @Description **Job Status Values:**
// @Description - `Scheduled`: Job is scheduled and waiting for the next execution time
// @Description - `Executing`: Job is currently running the task
// @Description - `Stopped`: Job has been stopped and deleted
// @Description
// @Description **Job Lifecycle:**
// @Description 1. Create job (this API) → Status: `Scheduled`, **executes immediately**
// @Description 2. First execution starts → Status: `Executing`
// @Description 3. Execution completes → Status: `Scheduled` (waits for interval)
// @Description 4. After interval → Status: `Executing` (cycles back to step 3)
// @Description 5. Pause job → `enabled: false`, Status: `Scheduled` (no execution)
// @Description 6. Resume job → `enabled: true`, Status: `Scheduled` (resumes execution)
// @Description 7. Delete job → Status: `Stopped`, job removed permanently
// @Description
// @Description **Failure Handling:**
// @Description - Tracks `successCount`, `failureCount`, `consecutiveFailures`
// @Description - Auto-disables after 5 consecutive failures (`autoDisabled: true`)
// @Description - Auto-recovers when next execution succeeds
// @Description
// @Description **Timeout Protection:**
// @Description - Default execution timeout: 30 minutes
// @Description - Jobs exceeding timeout are marked as failed
// @Description - Server restart during execution marks job as interrupted
// @Description
// @Description **Duplicate Prevention:**
// @Description - System checks for existing jobs with same configuration
// @Description - Configuration uniqueness based on: jobType + nsId + connectionName + mciNamePrefix + option + mciFlag
// @Description - Returns 409 Conflict if duplicate job exists with existing job ID
// @Tags [Job Scheduler] (WIP) CSP Resource Registration
// @Accept json
// @Produce json
// @Param scheduleRequest body model.ScheduleJobRequest true "Schedule job request (nsId must be specified in request body)"
// @Success 200 {object} model.ScheduleJobStatus
// @Failure 400 {object} model.SimpleMsg
// @Failure 409 {object} model.SimpleMsg "Duplicate job already exists"
// @Failure 500 {object} model.SimpleMsg
// @Router /registerCspResources/schedule [post]
func RestPostScheduleRegisterCspResources(c echo.Context) error {
	// Parse request body
	req := new(model.ScheduleJobRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{
			Message: "Invalid request format: " + err.Error(),
		})
	}

	// Manual validation
	if req.NsId == "" {
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{
			Message: "nsId is required in request body",
		})
	}
	if req.JobType == "" {
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{
			Message: "jobType is required",
		})
	}

	// Force job type
	req.JobType = "registerCspResources"

	// Create scheduled job
	scheduler := infra.GetSchedulerManager()
	job, err := scheduler.CreateScheduledJob(*req)
	if err != nil {
		// Check if it's a duplicate job error
		if strings.Contains(err.Error(), "duplicate job already exists") {
			log.Warn().Err(err).Msg("Duplicate job creation attempt")
			return c.JSON(http.StatusConflict, model.SimpleMsg{
				Message: err.Error(),
			})
		}

		// Check if it's a validation error
		if strings.Contains(err.Error(), "Validation Failed") {
			log.Warn().Err(err).Msg("Invalid registration options")
			return c.JSON(http.StatusUnprocessableEntity, model.SimpleMsg{
				Message: err.Error(),
			})
		}

		// Other errors
		log.Error().Err(err).Msg("Failed to create scheduled job")
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{
			Message: "Failed to create scheduled job: " + err.Error(),
		})
	}

	// Return job status
	status := job.GetStatus()
	return c.JSON(http.StatusOK, status)
}

// RestGetScheduleRegisterCspResourcesList godoc
// @ID GetScheduleRegisterCspResourcesList
// @Summary List all scheduled CSP resource registration jobs
// @Description Get a list of all scheduled CSP resource registration jobs (jobs are not scoped to namespaces)
// @Tags [Job Scheduler] (WIP) CSP Resource Registration
// @Accept json
// @Produce json
// @Success 200 {object} model.ScheduleJobListResponse
// @Failure 500 {object} model.SimpleMsg
// @Router /registerCspResources/schedule [get]
func RestGetScheduleRegisterCspResourcesList(c echo.Context) error {
	// Get all jobs (jobs are not namespace-scoped)
	scheduler := infra.GetSchedulerManager()
	jobs := scheduler.ListScheduledJobs()

	// Convert to status list
	statusList := make([]model.ScheduleJobStatus, 0, len(jobs))
	for _, job := range jobs {
		statusList = append(statusList, job.GetStatus())
	}

	return c.JSON(http.StatusOK, model.ScheduleJobListResponse{
		Jobs: statusList,
	})
}

// RestGetScheduleRegisterCspResourcesStatus godoc
// @ID GetScheduleRegisterCspResourcesStatus
// @Summary Get scheduled job status
// @Description Get the current status of a specific scheduled CSP resource registration job
// @Description
// @Description **Response Fields Explanation:**
// @Description - `status`: Current job state (Scheduled/Executing/Stopped)
// @Description - `enabled`: Whether job is active (can be paused with false)
// @Description - `executionCount`: Total number of executions attempted
// @Description - `successCount`: Number of successful executions
// @Description - `failureCount`: Number of failed executions
// @Description - `consecutiveFailures`: Current streak of failures (resets on success)
// @Description - `autoDisabled`: True if job was auto-disabled due to 5+ consecutive failures
// @Description - `lastExecutedAt`: Timestamp of most recent execution
// @Description - `nextExecutionAt`: Scheduled time for next execution
// @Description - `lastError`: Error message from most recent failure (empty if success)
// @Description - `lastResult`: Result message from most recent execution
// @Description
// @Description **Monitoring Recommendations:**
// @Description - Check `consecutiveFailures` - alert if >= 3
// @Description - Monitor `autoDisabled` - requires manual intervention if true
// @Description - Compare `successCount` vs `failureCount` for reliability metrics
// @Tags [Job Scheduler] (WIP) CSP Resource Registration
// @Accept json
// @Produce json
// @Param jobId path string true "Job ID"
// @Success 200 {object} model.ScheduleJobStatus
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /registerCspResources/schedule/{jobId} [get]
func RestGetScheduleRegisterCspResourcesStatus(c echo.Context) error {
	jobId := c.Param("jobId")

	// Get job
	scheduler := infra.GetSchedulerManager()
	job, err := scheduler.GetScheduledJob(jobId)
	if err != nil {
		return c.JSON(http.StatusNotFound, model.SimpleMsg{
			Message: "Job not found: " + err.Error(),
		})
	}

	// Return job status
	status := job.GetStatus()
	return c.JSON(http.StatusOK, status)
}

// RestPutScheduleRegisterCspResources godoc
// @ID PutScheduleRegisterCspResources
// @Summary Update scheduled job configuration
// @Description Update the configuration of a scheduled CSP resource registration job (interval, enabled status)
// @Description
// @Description **Updatable Fields:**
// @Description - `intervalSeconds`: Change execution frequency (minimum 10 seconds)
// @Description - `enabled`: Enable (true) or disable (false) the job
// @Description
// @Description **Usage Examples:**
// @Description - Change interval: `{"intervalSeconds": 30}` (30 seconds)
// @Description - Pause job: `{"enabled": false}`
// @Description - Resume job: `{"enabled": true}`
// @Description - Change both: `{"intervalSeconds": 10, "enabled": true}`
// @Description
// @Description **Note:** For simpler pause/resume operations, consider using dedicated `/pause` and `/resume` endpoints
// @Tags [Job Scheduler] (WIP) CSP Resource Registration
// @Accept json
// @Produce json
// @Param jobId path string true "Job ID"
// @Param updateRequest body model.UpdateScheduleJobRequest true "Update schedule job request"
// @Success 200 {object} model.ScheduleJobStatus
// @Failure 400 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /registerCspResources/schedule/{jobId} [put]
func RestPutScheduleRegisterCspResources(c echo.Context) error {
	jobId := c.Param("jobId")

	// Parse request body
	req := new(model.UpdateScheduleJobRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{
			Message: "Invalid request format: " + err.Error(),
		})
	}

	// Update job
	scheduler := infra.GetSchedulerManager()
	job, err := scheduler.UpdateScheduledJob(jobId, *req)
	if err != nil {
		if err.Error() == "job not found: "+jobId {
			return c.JSON(http.StatusNotFound, model.SimpleMsg{
				Message: err.Error(),
			})
		}
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{
			Message: "Failed to update scheduled job: " + err.Error(),
		})
	}

	// Return updated job status
	status := job.GetStatus()
	return c.JSON(http.StatusOK, status)
}

// RestDeleteScheduleRegisterCspResources godoc
// @ID DeleteScheduleRegisterCspResources
// @Summary Stop and delete scheduled job
// @Description Stop and permanently delete a scheduled CSP resource registration job
// @Description
// @Description **Warning:** This operation is irreversible!
// @Description - Job will be stopped immediately
// @Description - All job data and execution history will be deleted
// @Description - Cannot be recovered after deletion
// @Description
// @Description **Alternatives:**
// @Description - To temporarily stop: Use `/pause` endpoint instead
// @Description - To keep history: Set `enabled: false` via PUT endpoint
// @Tags [Job Scheduler] (WIP) CSP Resource Registration
// @Accept json
// @Produce json
// @Param jobId path string true "Job ID"
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /registerCspResources/schedule/{jobId} [delete]
func RestDeleteScheduleRegisterCspResources(c echo.Context) error {
	jobId := c.Param("jobId")

	// Stop and delete job
	scheduler := infra.GetSchedulerManager()
	err := scheduler.StopScheduledJob(jobId)
	if err != nil {
		if err.Error() == "job not found: "+jobId {
			return c.JSON(http.StatusNotFound, model.SimpleMsg{
				Message: err.Error(),
			})
		}
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{
			Message: "Failed to stop scheduled job: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, model.SimpleMsg{
		Message: "Scheduled job stopped and deleted successfully: " + jobId,
	})
}

// RestDeleteScheduleRegisterCspResourcesAll godoc
// @ID DeleteScheduleRegisterCspResourcesAll
// @Summary Delete ALL scheduled jobs
// @Description ⚠️ **DANGER: This operation deletes ALL scheduled jobs in the system!**
// @Description
// @Description **⚠️ CRITICAL WARNINGS:**
// @Description - This will PERMANENTLY DELETE **ALL** scheduled jobs across all namespaces
// @Description - All job execution history will be lost
// @Description - This operation is IRREVERSIBLE and cannot be undone
// @Description - Use with EXTREME CAUTION in production environments
// @Description
// @Description **Use Cases:**
// @Description - Cleaning up test/development environments
// @Description - Emergency shutdown of all scheduled operations
// @Description - System maintenance or reset
// @Description
// @Description **Safer Alternatives:**
// @Description - Delete individual jobs: Use `DELETE /registerCspResources/schedule/{jobId}`
// @Description - Temporarily stop all jobs: Pause each job individually via `/pause` endpoint
// @Description - Disable without deleting: Update each job with `enabled: false`
// @Description
// @Description **Response Information:**
// @Description - Returns the count of deleted jobs
// @Description - Returns 200 even if no jobs were found (count will be 0)
// @Tags [Job Scheduler] (WIP) CSP Resource Registration
// @Accept json
// @Produce json
// @Success 200 {object} model.SimpleMsg "Successfully deleted all jobs (message includes count)"
// @Failure 500 {object} model.SimpleMsg
// @Router /registerCspResources/schedule [delete]
func RestDeleteScheduleRegisterCspResourcesAll(c echo.Context) error {
	// Stop and delete all jobs (jobs are not namespace-scoped)
	scheduler := infra.GetSchedulerManager()
	deletedCount, err := scheduler.StopAllScheduledJobs()

	if err != nil {
		log.Error().Err(err).Int("deletedCount", deletedCount).
			Msg("Errors occurred while deleting some jobs, but operation completed")
		return c.JSON(http.StatusOK, model.SimpleMsg{
			Message: fmt.Sprintf("Deleted %d scheduled jobs with some errors. Check logs for details.", deletedCount),
		})
	}

	if deletedCount == 0 {
		return c.JSON(http.StatusOK, model.SimpleMsg{
			Message: "No scheduled jobs found",
		})
	}

	log.Warn().Int("deletedCount", deletedCount).
		Msg("All scheduled jobs deleted")

	return c.JSON(http.StatusOK, model.SimpleMsg{
		Message: fmt.Sprintf("Successfully deleted all %d scheduled jobs", deletedCount),
	})
}

// RestPutScheduleRegisterCspResourcesPause godoc
// @ID PutScheduleRegisterCspResourcesPause
// @Summary Pause a scheduled job
// @Description Temporarily pause a scheduled job without deleting it. The job can be resumed later.
// @Description This sets enabled=false and preserves all job state and execution history.
// @Tags [Job Scheduler] (WIP) CSP Resource Registration
// @Accept json
// @Produce json
// @Param jobId path string true "Job ID"
// @Success 200 {object} model.ScheduleJobStatus
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /registerCspResources/schedule/{jobId}/pause [put]
func RestPutScheduleRegisterCspResourcesPause(c echo.Context) error {
	jobId := c.Param("jobId")

	// Pause job (set enabled=false)
	scheduler := infra.GetSchedulerManager()
	enabled := false
	job, err := scheduler.UpdateScheduledJob(jobId, model.UpdateScheduleJobRequest{
		Enabled: &enabled,
	})
	if err != nil {
		if err.Error() == "job not found: "+jobId {
			return c.JSON(http.StatusNotFound, model.SimpleMsg{
				Message: err.Error(),
			})
		}
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{
			Message: "Failed to pause scheduled job: " + err.Error(),
		})
	}

	log.Info().Str("jobId", jobId).Msg("Scheduled job paused by user request")

	// Return updated job status
	status := job.GetStatus()
	return c.JSON(http.StatusOK, status)
}

// RestPutScheduleRegisterCspResourcesResume godoc
// @ID PutScheduleRegisterCspResourcesResume
// @Summary Resume a paused scheduled job
// @Description Resume a previously paused scheduled job to continue periodic execution.
// @Description This sets enabled=true and restarts the job scheduler.
// @Tags [Job Scheduler] (WIP) CSP Resource Registration
// @Accept json
// @Produce json
// @Param jobId path string true "Job ID"
// @Success 200 {object} model.ScheduleJobStatus
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /registerCspResources/schedule/{jobId}/resume [put]
func RestPutScheduleRegisterCspResourcesResume(c echo.Context) error {
	jobId := c.Param("jobId")

	// Resume job (set enabled=true)
	scheduler := infra.GetSchedulerManager()
	enabled := true
	job, err := scheduler.UpdateScheduledJob(jobId, model.UpdateScheduleJobRequest{
		Enabled: &enabled,
	})
	if err != nil {
		if err.Error() == "job not found: "+jobId {
			return c.JSON(http.StatusNotFound, model.SimpleMsg{
				Message: err.Error(),
			})
		}
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{
			Message: "Failed to resume scheduled job: " + err.Error(),
		})
	}

	log.Info().Str("jobId", jobId).Msg("Scheduled job resumed by user request")

	// Return updated job status
	status := job.GetStatus()
	return c.JSON(http.StatusOK, status)
}
