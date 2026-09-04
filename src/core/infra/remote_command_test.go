/*
Copyright 2019 The Cloud-Barista Authors.
<!-- SPDX-License-Identifier: Apache-2.0 -->
*/

package infra

import (
	"testing"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
)

func TestGetNextCommandIndex(t *testing.T) {
	tests := []struct {
		name     string
		history  []model.CommandStatusInfo
		expected int
	}{
		{
			name:     "Empty history returns 1",
			history:  []model.CommandStatusInfo{},
			expected: 1,
		},
		{
			name: "Sequential indices",
			history: []model.CommandStatusInfo{
				{Index: 1},
				{Index: 2},
				{Index: 3},
			},
			expected: 4,
		},
		{
			name: "Gaps in indices returns max+1",
			history: []model.CommandStatusInfo{
				{Index: 5},
				{Index: 2},
				{Index: 10},
			},
			expected: 11,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getNextCommandIndex(tt.history)
			if got != tt.expected {
				t.Errorf("getNextCommandIndex() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestIsTerminalCommandStatus(t *testing.T) {
	terminalStatuses := []model.CommandExecutionStatus{
		model.CommandStatusCompleted,
		model.CommandStatusFailed,
		model.CommandStatusCancelled,
	}

	for _, st := range terminalStatuses {
		if !isTerminalCommandStatus(st) {
			t.Errorf("expected %s to be terminal status", st)
		}
	}

	nonTerminalStatuses := []model.CommandExecutionStatus{
		model.CommandStatusQueued,
		model.CommandStatusHandling,
	}

	for _, st := range nonTerminalStatuses {
		if isTerminalCommandStatus(st) {
			t.Errorf("expected %s to NOT be terminal status", st)
		}
	}
}

func TestTrimCommandStatusHistory(t *testing.T) {
	history := []model.CommandStatusInfo{
		{Index: 1},
		{Index: 2},
		{Index: 3},
		{Index: 4},
		{Index: 5},
	}

	trimCommandStatusHistory(&history, 3)
	if len(history) != 3 {
		t.Fatalf("expected history length 3 after trim, got %d", len(history))
	}

	// Should keep newest entries (indices 3, 4, 5)
	if history[0].Index != 3 || history[2].Index != 5 {
		t.Errorf("expected newest entries preserved (3..5), got indices: %d, %d, %d",
			history[0].Index, history[1].Index, history[2].Index)
	}

	// If limit > len, nothing changed
	trimCommandStatusHistory(&history, 10)
	if len(history) != 3 {
		t.Errorf("expected history length 3 when limit > len, got %d", len(history))
	}
}

func TestFilterCommands(t *testing.T) {
	history := []model.CommandStatusInfo{
		{Index: 1, Status: model.CommandStatusCompleted, CommandRequested: "echo hello", StartedTime: "2024-01-15 10:20:00"},
		{Index: 2, Status: model.CommandStatusFailed, CommandRequested: "exit 1", StartedTime: "2024-01-15 10:25:00"},
		{Index: 3, Status: model.CommandStatusHandling, CommandRequested: "sleep 100", StartedTime: "2024-01-15 10:30:00"},
	}

	t.Run("Filter by status", func(t *testing.T) {
		filter := &model.CommandStatusFilter{
			Status: []model.CommandExecutionStatus{model.CommandStatusCompleted},
		}
		filtered := filterCommands(history, filter)
		if len(filtered) != 1 || filtered[0].Index != 1 {
			t.Fatalf("expected only completed command, got %+v", filtered)
		}
	})

	t.Run("Filter by search keyword", func(t *testing.T) {
		filter := &model.CommandStatusFilter{
			CommandContains: "sleep",
		}
		filtered := filterCommands(history, filter)
		if len(filtered) != 1 || filtered[0].Index != 3 {
			t.Fatalf("expected only sleep command, got %+v", filtered)
		}
	})
}

func TestResolveSshUserName(t *testing.T) {
	tests := []struct {
		name         string
		verifiedUser string
		userName     string
		expected     string
	}{
		{
			name:         "Prefer verified user name",
			verifiedUser: "ubuntu",
			userName:     "root",
			expected:     "ubuntu",
		},
		{
			name:         "Fallback to requested user name",
			verifiedUser: "",
			userName:     "ec2-user",
			expected:     "ec2-user",
		},
		{
			name:         "Fallback to default cb-user if both empty",
			verifiedUser: "",
			userName:     "",
			expected:     model.SshDefaultUserName[0],
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveSshUserName(tt.verifiedUser, tt.userName)
			if got != tt.expected {
				t.Errorf("ResolveSshUserName(%q, %q) = %q, want %q",
					tt.verifiedUser, tt.userName, got, tt.expected)
			}
		})
	}
}

func TestPickBastion(t *testing.T) {
	bastions := []model.BastionNode{
		{NodeId: "bastion-1"},
		{NodeId: "bastion-2"},
	}

	// Deterministic selection: same target node always gets the same bastion
	picked1 := pickBastion(bastions, "ns-1", "infra-1", "target-node-a")
	picked2 := pickBastion(bastions, "ns-1", "infra-1", "target-node-a")

	if picked1.NodeId != picked2.NodeId {
		t.Errorf("pickBastion was non-deterministic: %s != %s", picked1.NodeId, picked2.NodeId)
	}

	// Empty list returns empty BastionNode
	empty := pickBastion(nil, "ns-1", "infra-1", "target-node-a")
	if empty.NodeId != "" {
		t.Errorf("expected empty bastion when list is empty, got %q", empty.NodeId)
	}
}
