/*
Copyright 2019 The Cloud-Barista Authors.
<!-- SPDX-License-Identifier: Apache-2.0 -->
*/

package infra

import (
	"fmt"
	"strings"
	"testing"
	"time"

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

func TestSummarizeCommand(t *testing.T) {
	tests := []struct {
		name     string
		cmd      string
		maxLen   int
		expected string
	}{
		{
			name:     "Short single line",
			cmd:      "echo hello",
			maxLen:   50,
			expected: "echo hello",
		},
		{
			name:     "Multi-line to single-line",
			cmd:      "line1\nline2\r\nline3",
			maxLen:   50,
			expected: "line1 line2 line3",
		},
		{
			name:     "Truncated with ellipsis",
			cmd:      "12345678901234567890",
			maxLen:   10,
			expected: "1234567890...",
		},
		{
			name:     "Whitespace trimmed",
			cmd:      "   spaced command   \n",
			maxLen:   50,
			expected: "spaced command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := summarizeCommand(tt.cmd, tt.maxLen)
			if got != tt.expected {
				t.Errorf("summarizeCommand() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestDisplayCommand(t *testing.T) {
	blob := strings.Repeat("QUJD", 40)
	cases := []struct{ name, in, want string }{
		{"plain command", "ls -la /tmp && echo done", "ls -la /tmp && echo done"},
		{"init script runner", "echo " + blob + " | base64 -d | sudo bash", fmt.Sprintf("[init script runner, %d bytes]", len("echo "+blob+" | base64 -d | sudo bash"))},
		{"env prefix", "export DB_PASSWORD='S3cr3t,Value;9'; export MY_API_KEY='abc'; ./run.sh --flag='x'", "export DB_PASSWORD=********; export MY_API_KEY=********; ./run.sh --flag='x'"},
		{"short echo is not a runner", "echo aGVsbG8= | base64 -d", "echo aGVsbG8= | base64 -d"},
	}
	for _, c := range cases {
		if got := displayCommand(c.in); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestPickBastionPrefersOwnInfra(t *testing.T) {
	auto := model.BastionAssignedAuto
	other := model.BastionNode{InfraId: "infra-a", NodeId: "g1-2", Assigned: auto}
	own := model.BastionNode{InfraId: "infra-b", NodeId: "g1-1", Assigned: auto}
	self := model.BastionNode{InfraId: "infra-b", NodeId: "g1-9", Assigned: auto}
	manual := model.BastionNode{InfraId: "infra-a", NodeId: "jump", Assigned: model.BastionAssignedManual}

	for _, tc := range []struct {
		name     string
		bastions []model.BastionNode
		want     string
	}{
		{"own Infra over another Infra", []model.BastionNode{other, own}, "infra-b/g1-1"},
		{"another Infra when the Infra has none", []model.BastionNode{other}, "infra-a/g1-2"},
		{"own non-self over self", []model.BastionNode{other, self, own}, "infra-b/g1-1"},
		{"self when it is the only own entry", []model.BastionNode{other, self}, "infra-b/g1-9"},
		{"manual over own", []model.BastionNode{own, manual}, "infra-a/jump"},
	} {
		got := pickBastion(tc.bastions, "ns-1", "infra-b", "g1-9")
		if got.InfraId+"/"+got.NodeId != tc.want {
			t.Errorf("%s: picked %s/%s, want %s", tc.name, got.InfraId, got.NodeId, tc.want)
		}
	}
}

func TestBastionCooldown(t *testing.T) {
	ns := "ns-cooldown"
	auto := model.BastionNode{InfraId: "infra-a", NodeId: "g1-1", Assigned: model.BastionAssignedAuto}
	own := model.BastionNode{InfraId: "infra-b", NodeId: "g1-1", Assigned: model.BastionAssignedAuto}
	manual := model.BastionNode{InfraId: "infra-a", NodeId: "jump", Assigned: model.BastionAssignedManual}
	legacy := model.BastionNode{NodeId: "old"}

	if !needsInfraBastion(nil, ns, "infra-b") || !needsInfraBastion([]model.BastionNode{auto}, ns, "infra-b") {
		t.Error("an Infra without its own auto bastion should get one")
	}
	if needsInfraBastion([]model.BastionNode{auto, own}, ns, "infra-b") {
		t.Error("an Infra with its own auto bastion needs no other")
	}
	if needsInfraBastion([]model.BastionNode{manual}, ns, "infra-b") || needsInfraBastion([]model.BastionNode{legacy}, ns, "infra-b") {
		t.Error("manual and legacy entries must keep auto-assignment out")
	}

	markBastionUnreachable(ns, own)
	markBastionUnreachable(ns, manual)
	defer unreachableBastions.Delete(bastionKey(ns, own))
	defer unreachableBastions.Delete(bastionKey(ns, manual))

	got := reachableBastions(ns, []model.BastionNode{auto, own, manual})
	if len(got) != 2 || got[0].InfraId != "infra-a" || got[1].NodeId != "jump" {
		t.Errorf("reachableBastions = %+v, want the other Infra's auto entry and the manual one", got)
	}
	if !needsInfraBastion([]model.BastionNode{auto, own}, ns, "infra-b") {
		t.Error("an Infra whose own bastion is set aside should get another")
	}
	// the cooldown is per namespace/Infra/Node, so a same-named Node of another Infra is unaffected
	if bastionCoolingDown(ns, auto) {
		t.Error("cooldown leaked to another Infra's Node with the same id")
	}

	unreachableBastions.Store(bastionKey(ns, own), time.Now().Add(-time.Second))
	if bastionCoolingDown(ns, own) {
		t.Error("an expired cooldown should no longer apply")
	}
}

func TestRunWithBastionFailover(t *testing.T) {
	auto := model.BastionAssignedAuto
	dead := model.BastionNode{InfraId: "infra-a", NodeId: "dead", Assigned: auto}
	spare := model.BastionNode{InfraId: "infra-b", NodeId: "spare", Assigned: auto}
	manual := model.BastionNode{InfraId: "infra-a", NodeId: "jump", Assigned: model.BastionAssignedManual}
	unreachable := func(b model.BastionNode) error {
		return fmt.Errorf("connect: %w", &bastionUnreachableError{fmt.Errorf("no route to %s", b.NodeId)})
	}
	run := func(ns string, bastions []model.BastionNode, refill func() []model.BastionNode, fail map[string]error) ([]string, error) {
		tried := []string{}
		err := runWithBastionFailover(ns, "infra-t", "target", bastions, refill, func(b model.BastionNode) error {
			tried = append(tried, b.NodeId)
			return fail[b.NodeId]
		})
		return tried, err
	}

	// an unreachable auto bastion is replaced by one from refill, and stays set aside for the next request
	ns := "ns-failover-1"
	refills := 0
	refill := func() []model.BastionNode { refills++; return []model.BastionNode{dead, spare} }
	tried, err := run(ns, []model.BastionNode{dead}, refill, map[string]error{"dead": unreachable(dead)})
	if err != nil || len(tried) != 2 || tried[0] != "dead" || tried[1] != "spare" || refills != 1 {
		t.Errorf("failover: tried %v, refills %d, err %v", tried, refills, err)
	}
	tried, err = run(ns, []model.BastionNode{dead, spare}, refill, map[string]error{"dead": unreachable(dead)})
	if err != nil || len(tried) != 1 || tried[0] != "spare" {
		t.Errorf("second request should skip the set-aside bastion: tried %v, err %v", tried, err)
	}
	unreachableBastions.Delete(bastionKey(ns, dead))

	// a manual bastion is never routed around
	ns = "ns-failover-2"
	tried, err = run(ns, []model.BastionNode{manual, spare}, nil, map[string]error{"jump": unreachable(manual)})
	if !isBastionUnreachable(err) || len(tried) != 1 || tried[0] != "jump" {
		t.Errorf("manual bastion: tried %v, err %v", tried, err)
	}
	if bastionCoolingDown(ns, manual) {
		t.Error("a manual bastion must not be set aside")
	}

	// a failure past the bastion (target side) is returned as is
	ns = "ns-failover-3"
	targetErr := fmt.Errorf("target refused")
	tried, err = run(ns, []model.BastionNode{dead, spare}, nil, map[string]error{"dead": targetErr, "spare": targetErr})
	if err != targetErr || len(tried) != 1 {
		t.Errorf("target error: tried %v, err %v", tried, err)
	}

	// with nothing else to use the failure is reported after one attempt per bastion
	ns = "ns-failover-4"
	tried, err = run(ns, []model.BastionNode{dead}, func() []model.BastionNode { return []model.BastionNode{dead} }, map[string]error{"dead": unreachable(dead)})
	if !isBastionUnreachable(err) || len(tried) != 1 {
		t.Errorf("no spare: tried %v, err %v", tried, err)
	}
	// a later request still dials it rather than failing untried
	tried, err = run(ns, []model.BastionNode{dead}, func() []model.BastionNode { return []model.BastionNode{dead} }, map[string]error{})
	if err != nil || len(tried) != 1 || tried[0] != "dead" {
		t.Errorf("set-aside only bastion: tried %v, err %v", tried, err)
	}
	unreachableBastions.Delete(bastionKey(ns, dead))
}
