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

func TestFirstNonEmpty(t *testing.T) {
	tests := []struct {
		name     string
		values   []string
		expected string
	}{
		{"all empty", []string{"", "", ""}, ""},
		{"first is non-empty", []string{"first", "second"}, "first"},
		{"second is non-empty", []string{"", "second", "third"}, "second"},
		{"single value", []string{"val"}, "val"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := firstNonEmpty(tt.values...)
			if got != tt.expected {
				t.Errorf("firstNonEmpty(%v) = %q, want %q", tt.values, got, tt.expected)
			}
		})
	}
}

func TestConvertSpiderToFirewallRuleInfo(t *testing.T) {
	tests := []struct {
		name     string
		input    model.SpiderSecurityRuleInfo
		expected model.FirewallRuleInfo
	}{
		{
			name: "Single port TCP",
			input: model.SpiderSecurityRuleInfo{
				FromPort:   "80",
				ToPort:     "80",
				IPProtocol: "tcp",
				Direction:  "inbound",
				CIDR:       "0.0.0.0/0",
			},
			expected: model.FirewallRuleInfo{
				Port:      "80",
				Protocol:  "tcp",
				Direction: "inbound",
				CIDR:      "0.0.0.0/0",
			},
		},
		{
			name: "Port range TCP",
			input: model.SpiderSecurityRuleInfo{
				FromPort:   "8000",
				ToPort:     "9000",
				IPProtocol: "tcp",
				Direction:  "inbound",
				CIDR:       "10.0.0.0/16",
			},
			expected: model.FirewallRuleInfo{
				Port:      "8000-9000",
				Protocol:  "tcp",
				Direction: "inbound",
				CIDR:      "10.0.0.0/16",
			},
		},
		{
			name: "ICMP protocol clears port",
			input: model.SpiderSecurityRuleInfo{
				FromPort:   "-1",
				ToPort:     "-1",
				IPProtocol: "icmp",
				Direction:  "inbound",
				CIDR:       "0.0.0.0/0",
			},
			expected: model.FirewallRuleInfo{
				Port:      "",
				Protocol:  "icmp",
				Direction: "inbound",
				CIDR:      "0.0.0.0/0",
			},
		},
		{
			name: "ALL protocol clears port",
			input: model.SpiderSecurityRuleInfo{
				FromPort:   "1",
				ToPort:     "65535",
				IPProtocol: "all",
				Direction:  "outbound",
				CIDR:       "0.0.0.0/0",
			},
			expected: model.FirewallRuleInfo{
				Port:      "",
				Protocol:  "all",
				Direction: "outbound",
				CIDR:      "0.0.0.0/0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertSpiderToFirewallRuleInfo(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ConvertSpiderToFirewallRuleInfo(%+v) = %+v, want %+v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestConvertTbToSpiderSecurityRuleInfo(t *testing.T) {
	tests := []struct {
		name     string
		input    model.FirewallRuleInfo
		expected model.SpiderSecurityRuleInfo
	}{
		{
			name: "Single port TCP",
			input: model.FirewallRuleInfo{
				Port:      "443",
				Protocol:  "tcp",
				Direction: "inbound",
				CIDR:      "0.0.0.0/0",
			},
			expected: model.SpiderSecurityRuleInfo{
				FromPort:   "443",
				ToPort:     "443",
				IPProtocol: "tcp",
				Direction:  "inbound",
				CIDR:       "0.0.0.0/0",
			},
		},
		{
			name: "Port range TCP",
			input: model.FirewallRuleInfo{
				Port:      "20-22",
				Protocol:  "tcp",
				Direction: "inbound",
				CIDR:      "0.0.0.0/0",
			},
			expected: model.SpiderSecurityRuleInfo{
				FromPort:   "20",
				ToPort:     "22",
				IPProtocol: "tcp",
				Direction:  "inbound",
				CIDR:       "0.0.0.0/0",
			},
		},
		{
			name: "ICMP protocol maps to -1",
			input: model.FirewallRuleInfo{
				Port:      "",
				Protocol:  "icmp",
				Direction: "inbound",
				CIDR:      "0.0.0.0/0",
			},
			expected: model.SpiderSecurityRuleInfo{
				FromPort:   "-1",
				ToPort:     "-1",
				IPProtocol: "icmp",
				Direction:  "inbound",
				CIDR:       "0.0.0.0/0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertTbToSpiderSecurityRuleInfo(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ConvertTbToSpiderSecurityRuleInfo(%+v) = %+v, want %+v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestConvertFirewallRuleRequestObjToInfoObjs(t *testing.T) {
	req := model.FirewallRuleReq{
		Ports:     "80,443,8080-8085",
		Protocol:  "tcp",
		Direction: "inbound",
		CIDR:      "0.0.0.0/0",
	}

	infos := ConvertFirewallRuleRequestObjToInfoObjs(req)
	if len(infos) != 3 {
		t.Fatalf("expected 3 firewall rule info objects, got %d", len(infos))
	}

	if infos[0].Port != "80" || infos[1].Port != "443" || infos[2].Port != "8080-8085" {
		t.Errorf("unexpected parsed ports: %+v", infos)
	}
}
