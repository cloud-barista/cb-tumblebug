/*
Copyright 2019 The Cloud-Barista Authors.
<!-- SPDX-License-Identifier: Apache-2.0 -->
*/

package resource

import (
	"testing"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
)

func TestExtractArchitecture(t *testing.T) {
	tests := []struct {
		name         string
		provider     string
		details      []model.KeyValue
		specName     string
		expectedArch string
	}{
		{
			name:     "AWS ARM instance via ProcessorInfo",
			provider: "aws",
			details: []model.KeyValue{
				{Key: "ProcessorInfo", Value: "{SupportedArchitectures:[arm64],SustainedClockSpeedInGhz:2.6}"},
			},
			specName:     "t4g.nano",
			expectedArch: string(model.ARM64),
		},
		{
			name:     "AWS x86 instance via ProcessorInfo",
			provider: "aws",
			details: []model.KeyValue{
				{Key: "ProcessorInfo", Value: "{SupportedArchitectures:[x86_64],SustainedClockSpeedInGhz:2.5}"},
			},
			specName:     "t3.micro",
			expectedArch: string(model.X86_64),
		},
		{
			name:         "Azure ARM instance by naming pattern",
			provider:     "azure",
			details:      []model.KeyValue{},
			specName:     "Standard_D2ps_v5",
			expectedArch: string(model.ARM64),
		},
		{
			name:         "Azure x86 instance",
			provider:     "azure",
			details:      []model.KeyValue{},
			specName:     "Standard_B2s",
			expectedArch: string(model.X86_64),
		},
		{
			name:         "GCP ARM T2A instance by pattern",
			provider:     "gcp",
			details:      []model.KeyValue{},
			specName:     "t2a-standard-1",
			expectedArch: string(model.ARM64),
		},
		{
			name:         "GCP x86 n2 instance",
			provider:     "gcp",
			details:      []model.KeyValue{},
			specName:     "n2-standard-2",
			expectedArch: string(model.X86_64),
		},
		{
			name:     "Alibaba ARM instance via CpuArchitecture",
			provider: "alibaba",
			details: []model.KeyValue{
				{Key: "CpuArchitecture", Value: "ARM"},
			},
			specName:     "ecs.g8y.small",
			expectedArch: string(model.ARM64),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractArchitecture(tt.provider, tt.details, tt.specName)
			if got != tt.expectedArch {
				t.Errorf("extractArchitecture(%q, details, %q) = %q, want %q",
					tt.provider, tt.specName, got, tt.expectedArch)
			}
		})
	}
}

func TestNormalizeAcceleratorModel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Gaudi3", "Intel GAUDI3"},
		{"Gaudi2", "Intel GAUDI2"},
		{"Gaudi", "Intel GAUDI"},
		{"MI300X", "AMD MI300X"},
		{"Radeon Pro V710", "AMD RADEON PRO V710"},
		{"Instinct", "AMD INSTINCT"},
		{"NVIDIA A100-SXM4-40GB", "NVIDIA A100-SXM4-40GB"},
		{"Unknown Model", "Unknown Model"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeAcceleratorModel(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeAcceleratorModel(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFilterConnConfigsByProvider(t *testing.T) {
	configs := []model.ConnConfig{
		{ConfigName: "aws-tokyo", ProviderName: "aws"},
		{ConfigName: "azure-korea", ProviderName: "azure"},
		{ConfigName: "gcp-asia", ProviderName: "gcp"},
		{ConfigName: "alibaba-shanghai", ProviderName: "alibaba"},
	}

	t.Run("Include target providers only", func(t *testing.T) {
		filtered := filterConnConfigsByProvider(configs, []string{"aws", "gcp"}, nil)
		if len(filtered) != 2 {
			t.Fatalf("expected 2 configs, got %d", len(filtered))
		}
		if filtered[0].ProviderName != "aws" || filtered[1].ProviderName != "gcp" {
			t.Errorf("unexpected filtered results: %+v", filtered)
		}
	})

	t.Run("Exclude providers", func(t *testing.T) {
		filtered := filterConnConfigsByProvider(configs, nil, []string{"alibaba"})
		if len(filtered) != 3 {
			t.Fatalf("expected 3 configs, got %d", len(filtered))
		}
		for _, c := range filtered {
			if c.ProviderName == "alibaba" {
				t.Errorf("expected alibaba to be excluded, but found: %+v", c)
			}
		}
	})

	t.Run("No filters return all", func(t *testing.T) {
		filtered := filterConnConfigsByProvider(configs, nil, nil)
		if len(filtered) != len(configs) {
			t.Fatalf("expected %d configs, got %d", len(configs), len(filtered))
		}
	})
}

func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		specName string
		pattern  string
		expected bool
	}{
		{"t3.nano", "t3.*", true},
		{"t3.micro", "t3.*", true},
		{"m5.large", "t3.*", false},
		{"Standard_B1s", "*_B*s", true},
		{"Standard_D2s_v5", "*_B*s", false},
	}

	for _, tt := range tests {
		t.Run(tt.specName+"_"+tt.pattern, func(t *testing.T) {
			got := matchesPattern(tt.specName, tt.pattern)
			if got != tt.expected {
				t.Errorf("matchesPattern(%q, %q) = %v, want %v", tt.specName, tt.pattern, got, tt.expected)
			}
		})
	}
}

func TestConvertSpiderSpecToTumblebugSpec(t *testing.T) {
	connConfig := model.ConnConfig{
		ConfigName:   "aws-us-east-1",
		ProviderName: "aws",
		RegionDetail: model.RegionDetail{
			RegionName: "us-east-1",
		},
	}

	spiderSpec := model.SpiderSpecInfo{
		Region: "us-east-1",
		Name:   "t4g.micro",
		VCpu: model.SpiderVCpuInfo{
			Count: "2",
		},
		MemSizeMiB: "1024",
		DiskSizeGB: "0",
		KeyValueList: []model.KeyValue{
			{Key: "instanceType", Value: "t4g.micro"},
			{Key: "ProcessorInfo", Value: "{SupportedArchitectures:[arm64]}"},
		},
	}

	tbSpec, err := ConvertSpiderSpecToTumblebugSpec(connConfig, spiderSpec)
	if err != nil {
		t.Fatalf("unexpected error converting spec: %v", err)
	}

	if tbSpec.CspSpecName != "t4g.micro" {
		t.Errorf("expected CspSpecName 't4g.micro', got %q", tbSpec.CspSpecName)
	}
	if tbSpec.ProviderName != "aws" {
		t.Errorf("expected ProviderName 'aws', got %q", tbSpec.ProviderName)
	}
	if tbSpec.RegionName != "us-east-1" {
		t.Errorf("expected RegionName 'us-east-1', got %q", tbSpec.RegionName)
	}
	if tbSpec.VCPU != 2 {
		t.Errorf("expected VCPU 2, got %d", tbSpec.VCPU)
	}
	if tbSpec.MemoryGiB != 1.0 {
		t.Errorf("expected MemoryGiB 1.0, got %f", tbSpec.MemoryGiB)
	}
	if tbSpec.Architecture != string(model.ARM64) {
		t.Errorf("expected architecture arm64, got %q", tbSpec.Architecture)
	}
}
