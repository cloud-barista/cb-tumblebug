/*
Copyright 2019 The Cloud-Barista Authors.
<!-- SPDX-License-Identifier: Apache-2.0 -->
*/

package resource

import (
	"reflect"
	"testing"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
)

func TestIsCspNotFoundMsg(t *testing.T) {
	tests := []struct {
		name     string
		msg      string
		expected bool
	}{
		{"does not exist", "The VPC does not exist in provider", true},
		{"not found lowercase", "instance not found", true},
		{"NotFound compound", "VNetNotFound: resource id 123", true},
		{"irrelevant error", "failed to connect: timeout", false},
		{"internal server error", "500 Internal Server Error", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isCspNotFoundMsg(tt.msg)
			if got != tt.expected {
				t.Errorf("isCspNotFoundMsg(%q) = %v, want %v", tt.msg, got, tt.expected)
			}
		})
	}
}

func TestTombstoneSupported(t *testing.T) {
	supported := []string{
		model.StrDataDisk,
		model.StrSSHKey,
		model.StrSecurityGroup,
		model.StrCustomImage,
		model.StrVNet,
	}

	for _, resType := range supported {
		if !tombstoneSupported(resType) {
			t.Errorf("expected tombstoneSupported(%q) to be true, got false", resType)
		}
	}

	unsupported := []string{"spec", "image", "unknownResource"}
	for _, resType := range unsupported {
		if tombstoneSupported(resType) {
			t.Errorf("expected tombstoneSupported(%q) to be false, got true", resType)
		}
	}
}

func TestTombstoneStatus(t *testing.T) {
	if got := tombstoneStatus(model.StrVNet, false); got != model.ResourceStatusDeleting {
		t.Errorf("tombstoneStatus(vNet, false) = %q, want %q", got, model.ResourceStatusDeleting)
	}
	if got := tombstoneStatus(model.StrVNet, true); got != model.ResourceStatusFailed {
		t.Errorf("tombstoneStatus(vNet, true) = %q, want %q", got, model.ResourceStatusFailed)
	}
}

func TestGenSpecMapKey(t *testing.T) {
	got := GenSpecMapKey("us-east-1", "t3.Micro")
	expected := "us-east-1-t3.micro"
	if got != expected {
		t.Errorf("GenSpecMapKey = %q, want %q", got, expected)
	}
}

func TestProviderRegionZoneResourceKey(t *testing.T) {
	tests := []struct {
		name         string
		provider     string
		region       string
		zone         string
		resource     string
		expectedKey  string
	}{
		{
			name:        "All 4 fields present",
			provider:    "aws",
			region:      "us-east-1",
			zone:        "us-east-1a",
			resource:    "my-subnet",
			expectedKey: "aws+us-east-1+us-east-1a+my-subnet",
		},
		{
			name:        "Zone empty",
			provider:    "azure",
			region:      "koreacentral",
			zone:        "",
			resource:    "my-vnet",
			expectedKey: "azure+koreacentral+my-vnet",
		},
		{
			name:        "Region and zone empty",
			provider:    "gcp",
			region:      "",
			zone:        "",
			resource:    "global-res",
			expectedKey: "gcp+global-res",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := GetProviderRegionZoneResourceKey(tt.provider, tt.region, tt.zone, tt.resource)
			if key != tt.expectedKey {
				t.Errorf("GetProviderRegionZoneResourceKey() = %q, want %q", key, tt.expectedKey)
			}

			// Test round-trip resolve
			prov, reg, zone, res, err := ResolveProviderRegionZoneResourceKey(key)
			if err != nil {
				t.Fatalf("ResolveProviderRegionZoneResourceKey(%q) error: %v", key, err)
			}
			if prov != tt.provider || reg != tt.region || zone != tt.zone || res != tt.resource {
				t.Errorf("ResolveProviderRegionZoneResourceKey mismatch: got (%q, %q, %q, %q), want (%q, %q, %q, %q)",
					prov, reg, zone, res, tt.provider, tt.region, tt.zone, tt.resource)
			}
		})
	}
}

func TestDelEleInSlice(t *testing.T) {
	slice := []string{"zero", "one", "two", "three"}
	DelEleInSlice(&slice, 1)

	expected := []string{"zero", "two", "three"}
	if !reflect.DeepEqual(slice, expected) {
		t.Errorf("DelEleInSlice result = %v, want %v", slice, expected)
	}

	DelEleInSlice(&slice, 0)
	expected = []string{"two", "three"}
	if !reflect.DeepEqual(slice, expected) {
		t.Errorf("DelEleInSlice result = %v, want %v", slice, expected)
	}
}

func TestParseNetworkAction(t *testing.T) {
	tests := []struct {
		input    string
		expected NetworkAction
		valid    bool
	}{
		{"", ActionNone, true},
		{"force", ActionForce, true},
		{"FORCE", ActionForce, true},
		{"withsubnets", ActionWithSubnets, true},
		{"WithSubnets", ActionWithSubnets, true},
		{"invalid", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := ParseNetworkAction(tt.input)
			if ok != tt.valid {
				t.Errorf("ParseNetworkAction(%q) validity = %v, want %v", tt.input, ok, tt.valid)
			}
			if ok && got != tt.expected {
				t.Errorf("ParseNetworkAction(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestContainsZone(t *testing.T) {
	zones := []string{"us-east-1a", "us-east-1b", "us-east-1c"}
	if !ContainsZone(zones, "us-east-1a") {
		t.Errorf("expected ContainsZone to find us-east-1a")
	}
	if ContainsZone(zones, "us-east-1d") {
		t.Errorf("expected ContainsZone to not find us-east-1d")
	}
	if ContainsZone(nil, "us-east-1a") {
		t.Errorf("expected ContainsZone to return false for nil slice")
	}
}
