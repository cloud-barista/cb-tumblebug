/*
Copyright 2019 The Cloud-Barista Authors.
<!-- SPDX-License-Identifier: Apache-2.0 -->
*/

package infra

import (
	"reflect"
	"testing"
)

func TestNlbNormalizeOs(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ubuntu18.04", "ubuntu 18.04"},
		{"ubuntu22.04", "ubuntu 22.04"},
		{"centos7.9", "centos 7.9"},
		{"debian11.0", "debian 11.0"},
		{"ami-12345678", ""}, // no version with dot
		{"", ""},
		{"   ", ""},
		{"ubuntu", ""}, // no digits
		{"18.04", ""},  // starts with digit, name is empty
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := nlbNormalizeOs(tt.input)
			if got != tt.expected {
				t.Errorf("nlbNormalizeOs(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNlbOsCandidates(t *testing.T) {
	// Candidate for configured "ubuntu20.04"
	candidates := nlbOsCandidates("ubuntu20.04")
	if len(candidates) == 0 {
		t.Fatalf("expected non-empty candidates")
	}
	if candidates[0] != "ubuntu 20.04" {
		t.Errorf("expected first candidate to be configured OS 'ubuntu 20.04', got %q", candidates[0])
	}

	// Verify no duplicates
	seen := make(map[string]bool)
	for _, c := range candidates {
		if seen[c] {
			t.Errorf("duplicate candidate found: %q", c)
		}
		seen[c] = true
	}

	// Candidate for empty config should still have Ubuntu LTS fallbacks
	defaultCandidates := nlbOsCandidates("")
	if len(defaultCandidates) < 3 {
		t.Errorf("expected at least 3 default candidates, got %d", len(defaultCandidates))
	}
}

func TestSplitNLBKey(t *testing.T) {
	nsPrefix := "/ns/test-ns/infra/"

	tests := []struct {
		name          string
		key           string
		expectInfraId string
		expectNlbId   string
		expectOk      bool
	}{
		{
			name:          "Valid NLB key",
			key:           "/ns/test-ns/infra/my-infra/nlb/my-nlb",
			expectInfraId: "my-infra",
			expectNlbId:   "my-nlb",
			expectOk:      true,
		},
		{
			name:          "Not an NLB key (node key)",
			key:           "/ns/test-ns/infra/my-infra/node/my-node",
			expectInfraId: "",
			expectNlbId:   "",
			expectOk:      false,
		},
		{
			name:          "Invalid key depth",
			key:           "/ns/test-ns/infra/my-infra/nlb",
			expectInfraId: "",
			expectNlbId:   "",
			expectOk:      false,
		},
		{
			name:          "Completely different prefix",
			key:           "/other/key/path",
			expectInfraId: "",
			expectNlbId:   "",
			expectOk:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			infraId, nlbId, ok := splitNLBKey(tt.key, nsPrefix)
			if ok != tt.expectOk {
				t.Errorf("splitNLBKey(%q) ok = %v, want %v", tt.key, ok, tt.expectOk)
			}
			if ok {
				if infraId != tt.expectInfraId || nlbId != tt.expectNlbId {
					t.Errorf("splitNLBKey(%q) = (%q, %q), want (%q, %q)",
						tt.key, infraId, nlbId, tt.expectInfraId, tt.expectNlbId)
				}
			}
		})
	}
}

func TestRemoveStringSlice(t *testing.T) {
	input := []string{"apple", "banana", "cherry"}
	result := remove(input, "banana")
	expected := []string{"apple", "cherry"}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("remove() = %v, want %v", result, expected)
	}

	// Non-existent element
	result = remove(result, "not-in-list")
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("remove() on non-existent element changed slice: %v", result)
	}
}
