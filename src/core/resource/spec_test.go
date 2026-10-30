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

func TestGeneralizedGPUAcceleratorEnrichment(t *testing.T) {
	t.Run("ConvertSpiderSpecToTumblebugSpec fractional GPU from Spider", func(t *testing.T) {
		connConfig := model.ConnConfig{
			ConfigName:   "azure-eastus",
			ProviderName: "azure",
			RegionDetail: model.RegionDetail{RegionName: "eastus"},
		}
		spiderSpec := model.SpiderSpecInfo{
			Region:     "eastus",
			Name:       "Standard_NV6ads_A10_v5",
			VCpu:       model.SpiderVCpuInfo{Count: "6"},
			MemSizeMiB: "56320",
			DiskSizeGB: "1023",
			Gpu: []model.SpiderGpuInfo{
				{
					Mfr:            "NVIDIA",
					Model:          "A10",
					Count:          "0.167",
					MemSizeGB:      "24",
					TotalMemSizeGB: "4",
				},
			},
		}
		tbSpec, err := ConvertSpiderSpecToTumblebugSpec(connConfig, spiderSpec)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tbSpec.AcceleratorCount != 1 {
			t.Errorf("expected AcceleratorCount 1, got %d", tbSpec.AcceleratorCount)
		}
		if tbSpec.AcceleratorPartitionSize != float32(1.0/6.0) {
			t.Errorf("expected AcceleratorPartitionSize 1/6, got %f", tbSpec.AcceleratorPartitionSize)
		}
		if tbSpec.AcceleratorMemoryGB != 4 {
			t.Errorf("expected AcceleratorMemoryGB 4, got %f", tbSpec.AcceleratorMemoryGB)
		}
	})

	t.Run("AWS dynamic extraction from GpuInfo Details", func(t *testing.T) {
		spec := model.SpecInfo{
			ProviderName:     "aws",
			CspSpecName:      "g7f.future-xlarge",
			AcceleratorType:  "gpu",
			AcceleratorModel: "NVIDIA L4",
			AcceleratorCount: 0,
			Details: []model.KeyValue{
				{
					Key:   "GpuInfo",
					Value: "{Gpus:[{Count:0,GpuPartitionSize:0.125,LogicalGpuCount:1,Manufacturer:NVIDIA,MemoryInfo:{SizeInMiB:2861},Name:L4}],TotalGpuMemoryInMiB:2861}",
				},
			},
		}
		enrichSpecAcceleratorInfo(&spec)
		if spec.AcceleratorCount != 1 || spec.AcceleratorPartitionSize != 0.125 || spec.AcceleratorMemoryGB != 3 {
			t.Errorf("unexpected AWS enrichment: count=%d, part=%f, mem=%f",
				spec.AcceleratorCount, spec.AcceleratorPartitionSize, spec.AcceleratorMemoryGB)
		}

		// Binary GiB multiple in AWS EC2 SizeInMiB (e.g., T4 16384 MiB = 16 GiB, where Spider's 1.07374 multiplier gave 17 GB)
		t4Spec := model.SpecInfo{
			ProviderName:        "aws",
			CspSpecName:         "g4dn.xlarge",
			AcceleratorType:     "gpu",
			AcceleratorModel:    "NVIDIA T4",
			AcceleratorCount:    1,
			AcceleratorMemoryGB: 17,
			Details: []model.KeyValue{
				{
					Key:   "GpuInfo",
					Value: "{Gpus:[{Count:1,Manufacturer:NVIDIA,MemoryInfo:{SizeInMiB:16384},Name:T4}],TotalGpuMemoryInMiB:16384}",
				},
			},
		}
		enrichSpecAcceleratorInfo(&t4Spec)
		if t4Spec.AcceleratorMemoryGB != 16 {
			t.Errorf("expected T4 AcceleratorMemoryGB=16, got %f", t4Spec.AcceleratorMemoryGB)
		}

		// Decimal GB reported in MiB (e.g., L4 22888 MiB = 24 GB)
		l4Spec := model.SpecInfo{
			ProviderName:        "aws",
			CspSpecName:         "g6.xlarge",
			AcceleratorType:     "gpu",
			AcceleratorModel:    "NVIDIA L4",
			AcceleratorCount:    1,
			AcceleratorMemoryGB: 24,
			Details: []model.KeyValue{
				{
					Key:   "GpuInfo",
					Value: "{Gpus:[{Count:1,Manufacturer:NVIDIA,MemoryInfo:{SizeInMiB:22888},Name:L4}],TotalGpuMemoryInMiB:22888}",
				},
			},
		}
		enrichSpecAcceleratorInfo(&l4Spec)
		if l4Spec.AcceleratorMemoryGB != 24 {
			t.Errorf("expected L4 AcceleratorMemoryGB=24, got %f", l4Spec.AcceleratorMemoryGB)
		}
	})

	t.Run("Alibaba dynamic GPUSpec fraction and family rules", func(t *testing.T) {
		cases := []struct {
			specName string
			inModel  string
			inMem    float32
			wantMod  string
			wantPart float32
			wantMem  float32
		}{
			{"ecs.sgn7i-vws-m2.xlarge", "NVIDIA A10*1/12", 2, "NVIDIA A10", float32(1.0 / 12.0), 2},
			{"ecs.vgn6i-m4-vws.xlarge", "NVIDIA T4/4", 4, "NVIDIA T4", 0.25, 4},
			{"ecs.sgn8ia-m4.2xlarge", "NVIDIA VGPU8-4G", 0, "NVIDIA L20", float32(1.0 / 12.0), 4},
			{"ecs.sgn8ia-m48.24xlarge", "NVIDIA VGPU8-48G", 0, "NVIDIA L20", 0, 48},
		}
		for _, tc := range cases {
			spec := model.SpecInfo{
				ProviderName:        "alibaba",
				CspSpecName:         tc.specName,
				AcceleratorType:     "gpu",
				AcceleratorModel:    tc.inModel,
				AcceleratorCount:    1,
				AcceleratorMemoryGB: tc.inMem,
			}
			enrichSpecAcceleratorInfo(&spec)
			if spec.AcceleratorModel != tc.wantMod || spec.AcceleratorPartitionSize != tc.wantPart || spec.AcceleratorMemoryGB != tc.wantMem {
				t.Errorf("%s: got (model=%q, part=%f, mem=%f), want (model=%q, part=%f, mem=%f)",
					tc.specName, spec.AcceleratorModel, spec.AcceleratorPartitionSize, spec.AcceleratorMemoryGB,
					tc.wantMod, tc.wantPart, tc.wantMem)
			}
		}
	})

	t.Run("Azure dynamic series ratio and Details GPUs extraction", func(t *testing.T) {
		cases := []struct {
			specName string
			vcpu     uint16
			details  []model.KeyValue
			wantMod  string
			wantCnt  uint8
			wantPart float32
			wantMem  float32
		}{
			{"Standard_NV4as_v4", 4, nil, "AMD Instinct MI25", 1, 0.125, 2},
			{"Standard_NV4ads_V710_v5", 4, nil, "AMD Radeon PRO V710", 1, float32(1.0 / 6.0), 4},
			{"Standard_NV8ads_V710_v5", 8, nil, "AMD Radeon PRO V710", 1, float32(1.0 / 3.0), 8},
			{"Standard_NG8ads_V620_v1", 8, nil, "AMD Radeon PRO V620", 1, 0.25, 8},
			{"Standard_NV4ads_A10_v5", 4, nil, "NVIDIA A10", 1, float32(1.0 / 6.0), 4},
			{"Standard_NV12ads_A10_v5", 12, nil, "NVIDIA A10", 1, float32(1.0 / 3.0), 8},
			{"Standard_NV48s_v3", 48, nil, "NVIDIA Tesla M60", 4, 0, 8},
			{
				"Standard_NC288ds_xl_RTXPRO6000BSE_v6",
				288,
				[]model.KeyValue{{Key: "GPUs", Value: "2"}},
				"NVIDIA RTX PRO 6000",
				2,
				0,
				96,
			},
			{"Standard_ND128isr_GB300_v6", 128, nil, "NVIDIA GB300", 4, 0, 288},
		}
		for _, tc := range cases {
			spec := model.SpecInfo{
				ProviderName: "azure",
				CspSpecName:  tc.specName,
				VCPU:         tc.vcpu,
				Details:      tc.details,
			}
			enrichSpecAcceleratorInfo(&spec)
			if spec.AcceleratorModel != tc.wantMod || spec.AcceleratorCount != tc.wantCnt ||
				spec.AcceleratorPartitionSize != tc.wantPart || spec.AcceleratorMemoryGB != tc.wantMem {
				t.Errorf("%s: got (model=%q, cnt=%d, part=%f, mem=%f), want (model=%q, cnt=%d, part=%f, mem=%f)",
					tc.specName, spec.AcceleratorModel, spec.AcceleratorCount, spec.AcceleratorPartitionSize, spec.AcceleratorMemoryGB,
					tc.wantMod, tc.wantCnt, tc.wantPart, tc.wantMem)
			}
		}
	})

	t.Run("Spec metadata normalization (Architecture, vCPUsAvailable, MemoryGiB, DiskSizeGB)", func(t *testing.T) {
		gcpArm := model.SpecInfo{
			ProviderName: "gcp",
			CspSpecName:  "c4a-standard-4",
			VCPU:         4,
			MemoryGiB:    16,
			Architecture: string(model.X86_64),
			Details:      []model.KeyValue{{Key: "Architecture", Value: "ARM64"}},
		}
		enrichSpecAcceleratorInfo(&gcpArm)
		if gcpArm.Architecture != string(model.ARM64) {
			t.Errorf("expected GCP c4a-standard-4 Architecture=arm64, got %s", gcpArm.Architecture)
		}

		gcpHighCpu := model.SpecInfo{
			ProviderName: "gcp",
			CspSpecName:  "n1-highcpu-2",
			VCPU:         2,
			MemoryGiB:    float32(1843.0 / 1024.0), // 1.7998047
		}
		enrichSpecAcceleratorInfo(&gcpHighCpu)
		if gcpHighCpu.MemoryGiB != 1.8 {
			t.Errorf("expected GCP n1-highcpu-2 MemoryGiB=1.8, got %f", gcpHighCpu.MemoryGiB)
		}

		azureConstrained := model.SpecInfo{
			ProviderName: "azure",
			CspSpecName:  "Standard_E16-4s_v5",
			VCPU:         16,
			MemoryGiB:    128,
			DiskSizeGB:   17,
			RootDiskSize: 17,
			Architecture: string(model.X86_64),
			Details: []model.KeyValue{
				{Key: "vCPUs", Value: "16"},
				{Key: "vCPUsAvailable", Value: "4"},
				{Key: "ResourceDiskSizeInMB", Value: "16384"},
				{Key: "CpuArchitectureType", Value: "x64"},
			},
		}
		enrichSpecAcceleratorInfo(&azureConstrained)
		if azureConstrained.VCPU != 4 || azureConstrained.DiskSizeGB != 16 || azureConstrained.RootDiskSize != 16 {
			t.Errorf("unexpected Azure constrained normalization: vcpu=%d, disk=%f, rootDisk=%d",
				azureConstrained.VCPU, azureConstrained.DiskSizeGB, azureConstrained.RootDiskSize)
		}

		tencentRemark := model.SpecInfo{
			ProviderName:     "tencent",
			CspSpecName:      "BMG5t.24XLARGE384",
			AcceleratorType:  "gpu",
			AcceleratorModel: "NVIDIA",
			AcceleratorCount: 4,
			Details: []model.KeyValue{
				{Key: "InstanceFamily", Value: "BMG5t"},
				{Key: "Remark", Value: "4 NVIDIA T4"},
			},
		}
		enrichSpecAcceleratorInfo(&tencentRemark)
		if tencentRemark.AcceleratorModel != "NVIDIA T4" || tencentRemark.AcceleratorMemoryGB != 16 {
			t.Errorf("expected Tencent BMG5t.24XLARGE384 model=NVIDIA T4 mem=16, got model=%q mem=%f",
				tencentRemark.AcceleratorModel, tencentRemark.AcceleratorMemoryGB)
		}

		ibmGaudi := model.SpecInfo{
			ProviderName:     "ibm",
			CspSpecName:      "gx3d-160x1792x8gaudi3",
			AcceleratorType:  "gpu",
			AcceleratorModel: "NVIDIA GAUDI3",
			AcceleratorCount: 8,
		}
		enrichSpecAcceleratorInfo(&ibmGaudi)
		if ibmGaudi.AcceleratorModel != "Intel GAUDI3" || ibmGaudi.AcceleratorMemoryGB != 128 {
			t.Errorf("expected IBM gx3d-160x1792x8gaudi3 model=Intel GAUDI3 mem=128, got model=%q mem=%f",
				ibmGaudi.AcceleratorModel, ibmGaudi.AcceleratorMemoryGB)
		}
	})
}

