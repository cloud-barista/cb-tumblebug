/*
Copyright 2019 The Cloud-Barista Authors.
<!-- SPDX-License-Identifier: Apache-2.0 -->
*/

package infra

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/rs/zerolog/log"
)

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
