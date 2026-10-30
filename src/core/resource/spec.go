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

// Package resource is to manage multi-cloud infra resource
package resource

import (
	"context"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	validator "github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// executeWithTimeoutRetry executes a function and retries once after 1 minute if a timeout error occurs
func executeWithTimeoutRetry(fn func() error, operationName string, resourceName string) error {
	err := fn()
	if err == nil {
		return nil
	}

	// Check if it's a timeout error - retry once after waiting
	errStr := err.Error()
	if strings.Contains(errStr, "Client.Timeout") || strings.Contains(errStr, "context deadline exceeded") {
		log.Warn().Msgf("%s timeout for '%s', waiting 1 minute before retry...", operationName, resourceName)
		time.Sleep(1 * time.Minute)

		// Retry once
		retryErr := fn()
		if retryErr != nil {
			log.Error().Err(retryErr).Msgf("%s retry also failed for '%s'", operationName, resourceName)
			return retryErr
		}
		log.Info().Msgf("%s retry succeeded for '%s'", operationName, resourceName)
		return nil
	}

	return err
}

// SpecReqStructLevelValidation is a function to validate 'SpecReq' object.
func SpecReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(model.SpecReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

// ConvertSpiderSpecToTumblebugSpec accepts an Spider spec object, converts to and returns an TB spec object
func ConvertSpiderSpecToTumblebugSpec(connConfig model.ConnConfig, spiderSpec model.SpiderSpecInfo) (model.SpecInfo, error) {
	if spiderSpec.Name == "" {
		err := fmt.Errorf("failed convertSpiderSpecToTumblebugSpec. spiderSpec.Name is empty")
		emptyTumblebugSpec := model.SpecInfo{}
		return emptyTumblebugSpec, err
	}

	providerName := connConfig.ProviderName

	tumblebugSpec := model.SpecInfo{}

	tumblebugSpec.Name = spiderSpec.Name
	tumblebugSpec.CspSpecName = spiderSpec.Name
	tumblebugSpec.Uid = common.GenUid()
	tumblebugSpec.RegionName = spiderSpec.Region
	tumblebugSpec.RegionLatitude = connConfig.RegionDetail.Location.Latitude
	tumblebugSpec.RegionLongitude = connConfig.RegionDetail.Location.Longitude
	// log.Debug().Msgf("Region coordinates for spec %s: (%f, %f)", tumblebugSpec.CspSpecName, tumblebugSpec.RegionLatitude, tumblebugSpec.RegionLongitude)
	tumblebugSpec.ProviderName = providerName

	// For Azure, filter out Gen1-only VM families
	if csp.ResolveCloudPlatform(providerName) == csp.Azure {
		// TODO: needs to be merged with a general ignore filtering method
		if isAzureGen1OnlySpec(tumblebugSpec.CspSpecName, spiderSpec.KeyValueList) {
			err := fmt.Errorf("skipping Azure Gen1-only VM family spec: %s", tumblebugSpec.CspSpecName)
			emptyTumblebugSpec := model.SpecInfo{}
			return emptyTumblebugSpec, err
		}
	}

	tempUint64, _ := strconv.ParseUint(spiderSpec.VCpu.Count, 10, 16)
	tumblebugSpec.VCPU = uint16(tempUint64)
	tempFloat64, _ := strconv.ParseFloat(spiderSpec.MemSizeMiB, 32)
	tumblebugSpec.MemoryGiB = normalizeMemoryGiB(float32(tempFloat64 / 1024))
	tempFloat64, _ = strconv.ParseFloat(spiderSpec.DiskSizeGB, 32)
	tumblebugSpec.DiskSizeGB = float32(tempFloat64)
	if rootDiskSizeInt, err := strconv.Atoi(spiderSpec.DiskSizeGB); err == nil {
		tumblebugSpec.RootDiskSize = rootDiskSizeInt
	}

	tumblebugSpec.Details = spiderSpec.KeyValueList

	// Extract Architecture based on CSP
	tumblebugSpec.Architecture = extractArchitecture(tumblebugSpec.ProviderName, tumblebugSpec.Details, tumblebugSpec.CspSpecName)
	if tumblebugSpec.Architecture == string(model.ArchitectureUnknown) {
		log.Debug().Msgf("(%s) architecture for spec %s: %s", tumblebugSpec.ProviderName, tumblebugSpec.CspSpecName, tumblebugSpec.Architecture)
	}

	// GPU(Accelerator) information conversion
	if len(spiderSpec.Gpu) > 0 || len(spiderSpec.Accelerator) > 0 {
		tumblebugSpec.AcceleratorType = "gpu"

		var mfr, modelName, countStr, fractionStr, memSizeGB, totalMemSizeGB string
		if len(spiderSpec.Gpu) > 0 {
			firstGpu := spiderSpec.Gpu[0]
			mfr = firstGpu.Mfr
			modelName = firstGpu.Model
			countStr = firstGpu.Count
			memSizeGB = firstGpu.MemSizeGB
			totalMemSizeGB = firstGpu.TotalMemSizeGB
			if len(spiderSpec.Gpu) > 1 {
				log.Warn().Msgf("Spec %s has multiple GPUs defined (%d GPUs). Only using the first GPU information.",
					spiderSpec.Name, len(spiderSpec.Gpu))
			}
		}
		if len(spiderSpec.Accelerator) > 0 {
			firstAcc := spiderSpec.Accelerator[0]
			if firstAcc.Type != "" {
				tumblebugSpec.AcceleratorType = strings.ToLower(firstAcc.Type)
			}
			if mfr == "" {
				mfr = firstAcc.Mfr
			}
			if modelName == "" {
				modelName = firstAcc.Model
			}
			if countStr == "" || countStr == "-1" {
				countStr = firstAcc.Count
			}
			if firstAcc.Fraction != "" && firstAcc.Fraction != "-1" {
				fractionStr = firstAcc.Fraction
			}
			if (memSizeGB == "" || memSizeGB == "-1") && firstAcc.MemSizeGB != "" && firstAcc.MemSizeGB != "-1" {
				memSizeGB = firstAcc.MemSizeGB
			}
			if (totalMemSizeGB == "" || totalMemSizeGB == "-1") && firstAcc.TotalMemSizeGB != "" && firstAcc.TotalMemSizeGB != "-1" {
				totalMemSizeGB = firstAcc.TotalMemSizeGB
			}
		}

		// Combine Mfr and Model to form AcceleratorModel
		if mfr != "" && modelName != "" {
			if strings.HasPrefix(strings.ToUpper(modelName), strings.ToUpper(mfr)) {
				tumblebugSpec.AcceleratorModel = modelName
			} else {
				tumblebugSpec.AcceleratorModel = mfr + " " + modelName
			}
		} else if modelName != "" {
			tumblebugSpec.AcceleratorModel = modelName
		} else if mfr != "" {
			tumblebugSpec.AcceleratorModel = mfr
		}

		// Convert GPU count and optional partition fraction (handles integers, decimals like "0.125", and fractions like "1/6")
		cnt, partSize := parseAcceleratorCountAndPartition(countStr, fractionStr)
		if cnt > 0 {
			tumblebugSpec.AcceleratorCount = cnt
		}
		if partSize > 0 {
			tumblebugSpec.AcceleratorPartitionSize = partSize
		}

		// Convert GPU memory size: for fractional GPUs, CB-Spider sets MemSizeGB to the full physical card
		// memory and TotalMemSizeGB to the partition's memory available to the VM.
		if tumblebugSpec.AcceleratorPartitionSize > 0 && totalMemSizeGB != "" && totalMemSizeGB != "-1" {
			if tempMemory, err := strconv.ParseFloat(totalMemSizeGB, 32); err == nil && tempMemory > 0 {
				tumblebugSpec.AcceleratorMemoryGB = float32(tempMemory)
			}
		}
		if tumblebugSpec.AcceleratorMemoryGB == 0 && memSizeGB != "" && memSizeGB != "-1" {
			if tempMemory, err := strconv.ParseFloat(memSizeGB, 32); err == nil && tempMemory > 0 {
				if tumblebugSpec.AcceleratorPartitionSize > 0 {
					tumblebugSpec.AcceleratorMemoryGB = float32(math.Round(tempMemory * float64(tumblebugSpec.AcceleratorPartitionSize)))
				} else {
					tumblebugSpec.AcceleratorMemoryGB = float32(tempMemory)
				}
			}
		}

		// Correct manufacturer when Spider reports incorrect info (e.g., IBM labels Gaudi/MI300X as NVIDIA)
		tumblebugSpec.AcceleratorModel = normalizeAcceleratorModel(tumblebugSpec.AcceleratorModel)
	}

	// Enrich missing or incomplete GPU metadata across CSPs using CSP Details and family/model rules
	enrichSpecAcceleratorInfo(&tumblebugSpec)

	return tumblebugSpec, nil
}

// normalizePartitionFraction snaps a fractional GPU partition value (e.g. 0.167 from 3-decimal rounding)
// to its exact standard fraction (1/d or n/d for d in 2, 3, 4, 6, 8, 12, 16, 24, 32) when within tolerance.
func normalizePartitionFraction(f float32) float32 {
	if f <= 0 || f >= 1 {
		return 0
	}
	denoms := []int{2, 3, 4, 6, 8, 12, 16, 24, 32}
	for _, d := range denoms {
		for n := 1; n < d; n++ {
			exact := float64(n) / float64(d)
			if math.Abs(float64(f)-exact) < 0.005 {
				return float32(exact)
			}
		}
	}
	return f
}

// parseFractionString parses a fraction string ("1/6", "0.125") into a float64.
func parseFractionString(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "-1" {
		return 0, false
	}
	if strings.Contains(s, "/") {
		parts := strings.SplitN(s, "/", 2)
		num, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		den, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err1 == nil && err2 == nil && num > 0 && den > 0 {
			return num / den, true
		}
		return 0, false
	}
	if v, err := strconv.ParseFloat(s, 64); err == nil && v > 0 {
		return v, true
	}
	return 0, false
}

// parseAcceleratorCountAndPartition parses count and optional fraction strings from CB-Spider or CSV assets.
// Examples:
//   - countStr="2", fractionStr="" -> count=2, partitionSize=0
//   - countStr="0.125", fractionStr="" -> count=1, partitionSize=0.125
//   - countStr="1", fractionStr="1/6" -> count=1, partitionSize=0.16666667
//   - countStr="1/8", fractionStr="" -> count=1, partitionSize=0.125
func parseAcceleratorCountAndPartition(countStr, fractionStr string) (count uint8, partitionSize float32) {
	if f, ok := parseFractionString(fractionStr); ok && f > 0 && f < 1 {
		partitionSize = normalizePartitionFraction(float32(f))
	}

	if v, ok := parseFractionString(countStr); ok {
		if v > 0 && v < 1 {
			count = 1
			if partitionSize == 0 {
				partitionSize = normalizePartitionFraction(float32(v))
			}
		} else if v >= 1 && v <= 255 {
			count = uint8(math.Round(v))
		}
	} else if partitionSize > 0 {
		count = 1
	}

	return count, partitionSize
}

var (
	reExplicitGPUMemGB = regexp.MustCompile(`(?i)(?:^|[-_\s])(\d+)\s*gb?(?:$|[-_\s])`)
	reTencentRemarkGPU = regexp.MustCompile(`(?i)(?:^\d+\s*\*\s*|\b\d+\s+)?((?:nvidia|amd|intel)\s+[a-z0-9][a-z0-9\s._-]*)`)

	reKTA10080GXe   = regexp.MustCompile(`(?i)a100-80g-[a-z0-9]+-(\d+)ea`)
	reKTA10080GUnd  = regexp.MustCompile(`(?i)a100_80g_(\d+)`)
	reKTA10080G     = regexp.MustCompile(`(?i)a100[-_]80g`)
	reKTA100Count   = regexp.MustCompile(`(?i)a100-(\d+)`)
	reKTV100SCount  = regexp.MustCompile(`(?i)v100s-(\d+)`)
	reKTV100Count   = regexp.MustCompile(`(?i)v100-(\d+)`)
	reKTRTX4000     = regexp.MustCompile(`(?i)rtx4000-(\d+)`)
	reKTRTXShortCnt = regexp.MustCompile(`(?i)-rtx(\d+)`)
	reKTRTXShortOne = regexp.MustCompile(`(?i)-rtx$`)
	reKTT4Count     = regexp.MustCompile(`(?i)t4[-x](\d+)`)
	reKTT4Dot       = regexp.MustCompile(`(?i)\.t4\.`)

	reAWSGpuPartitionSize = regexp.MustCompile(`GpuPartitionSize:([0-9.]+)`)
	reAWSLogicalGpuCount  = regexp.MustCompile(`LogicalGpuCount:(\d+)`)
	reAWSGpuCount         = regexp.MustCompile(`\bCount:(\d+)`)
	reAWSGpuSizeInMiB     = regexp.MustCompile(`SizeInMiB:(\d+)`)
	reAWSGpuMfr           = regexp.MustCompile(`Manufacturer:([^,}]+)`)
	reAWSGpuName          = regexp.MustCompile(`\bName:([^,}]+)`)

	reAlibabaGPUFraction = regexp.MustCompile(`(?i)^(.+?)\s*(?:\*(\d+)\s*/\s*(\d+)|/\s*(\d+))$`)
	reAlibabaVGPUMem     = regexp.MustCompile(`(?i)\bvgpu(\d*)-(\d+)g\b`)
	reAlibabaSpecMemTag  = regexp.MustCompile(`(?i)-m(\d+)s?(?:\.|-|$)`)
)

// inferGPUMemoryGBFromModel infers standard per-card GPU VRAM (GB) from the extracted GPU model name
// and spec name across all CSPs when the CSP driver does not report memory size directly.
func inferGPUMemoryGBFromModel(modelName, specName string) float32 {
	lowerModel := strings.ToLower(strings.TrimSpace(modelName))
	lowerSpec := strings.ToLower(strings.TrimSpace(specName))
	if lowerModel == "" {
		return 0
	}

	// 1. Check if the model string itself encodes an explicit GB size (e.g., "NVIDIA A100-SXM4-40GB", "A100 80GB")
	if m := reExplicitGPUMemGB.FindStringSubmatch(lowerModel); len(m) > 1 {
		if v, err := strconv.ParseFloat(m[1], 32); err == nil && v > 0 {
			return float32(v)
		}
	}

	// 2. Match standard GPU hardware chip models (longest/most specific tokens first)
	switch {
	case strings.Contains(lowerModel, "gb300") || strings.Contains(lowerModel, "b300"):
		return 288
	case strings.Contains(lowerModel, "b200") || strings.Contains(lowerModel, "mi300x"):
		return 192
	case strings.Contains(lowerModel, "gb200"):
		return 186
	case strings.Contains(lowerModel, "h200"):
		return 141
	case strings.Contains(lowerModel, "gaudi3") || strings.Contains(lowerModel, "mi250"):
		return 128
	case strings.Contains(lowerModel, "rtx pro 6000") || strings.Contains(lowerModel, "rtxpro6000") || strings.Contains(lowerModel, "rtx pro server 6000"):
		return 96
	case strings.Contains(lowerModel, "h100") || strings.Contains(lowerModel, "a100p") || strings.Contains(lowerModel, "h800") || strings.Contains(lowerModel, "a800"):
		return 80
	case strings.Contains(lowerModel, "a100"):
		if strings.Contains(lowerSpec, "ultragpu") || strings.Contains(lowerSpec, "amsr") || strings.Contains(lowerSpec, "80g") || strings.HasPrefix(lowerSpec, "gx") {
			return 80
		}
		return 40
	case strings.Contains(lowerModel, "l40s") || strings.Contains(lowerModel, "l40") || strings.Contains(lowerModel, "l20") || strings.Contains(lowerModel, "a40"):
		return 48
	case strings.Contains(lowerModel, "v100s") || strings.Contains(lowerModel, "v620") || strings.Contains(lowerModel, "rtx pro 4500"):
		return 32
	case strings.Contains(lowerModel, "v100"):
		if strings.Contains(lowerSpec, "gn10xp") || strings.Contains(lowerSpec, "hccg5v") || strings.Contains(lowerSpec, "nd40") || strings.Contains(lowerSpec, ".v100") {
			return 32
		}
		return 16
	case strings.Contains(lowerModel, "l4") || strings.Contains(lowerModel, "a10g") || strings.Contains(lowerModel, "a10") || strings.Contains(lowerModel, "p40") || strings.Contains(lowerModel, "v710"):
		return 24
	case strings.Contains(lowerModel, "t4") || strings.Contains(lowerModel, "p100") || strings.Contains(lowerModel, "mi25"):
		return 16
	case strings.Contains(lowerModel, "k80"):
		return 12
	case strings.Contains(lowerModel, "p4") || strings.Contains(lowerModel, "m60") || strings.Contains(lowerModel, "rtx 4000") || strings.Contains(lowerModel, "v520"):
		return 8
	}
	return 0
}

func parseUint8Match(matches []string, fallback uint8) uint8 {
	if len(matches) > 1 {
		if v, err := strconv.ParseUint(matches[1], 10, 8); err == nil && v > 0 {
			return uint8(v)
		}
	}
	return fallback
}

// parseKTAcceleratorFromSpecName extracts GPU model, count, and per-GPU memory (GB) from KT Cloud spec names.
// CB-Spider's KT driver only checks for the substring "gpu" in the spec name and sets Model="NA <SPEC_NAME>",
// missing specs named with ".v100-1", ".A100-4", ".RTX4000-1", ".T4-4", "a100-80g-xe9680-4ea", etc.
func parseKTAcceleratorFromSpecName(specName string) (accelType string, modelName string, count uint8, memGB float32, matched bool) {
	lower := strings.ToLower(specName)

	switch {
	case reKTA10080GXe.MatchString(lower):
		m := reKTA10080GXe.FindStringSubmatch(lower)
		return "gpu", "NVIDIA A100", parseUint8Match(m, 1), 80, true
	case reKTA10080GUnd.MatchString(lower):
		m := reKTA10080GUnd.FindStringSubmatch(lower)
		return "gpu", "NVIDIA A100", parseUint8Match(m, 1), 80, true
	case reKTA10080G.MatchString(lower):
		return "gpu", "NVIDIA A100", 1, 80, true
	case reKTA100Count.MatchString(lower):
		m := reKTA100Count.FindStringSubmatch(lower)
		return "gpu", "NVIDIA A100", parseUint8Match(m, 1), 40, true
	case reKTV100SCount.MatchString(lower):
		m := reKTV100SCount.FindStringSubmatch(lower)
		return "gpu", "NVIDIA V100S", parseUint8Match(m, 1), 32, true
	case reKTV100Count.MatchString(lower):
		m := reKTV100Count.FindStringSubmatch(lower)
		return "gpu", "NVIDIA V100", parseUint8Match(m, 1), 32, true
	case reKTRTX4000.MatchString(lower):
		m := reKTRTX4000.FindStringSubmatch(lower)
		return "gpu", "NVIDIA RTX 4000", parseUint8Match(m, 1), 8, true
	case reKTRTXShortCnt.MatchString(lower):
		m := reKTRTXShortCnt.FindStringSubmatch(lower)
		return "gpu", "NVIDIA RTX 4000", parseUint8Match(m, 1), 8, true
	case reKTRTXShortOne.MatchString(lower):
		return "gpu", "NVIDIA RTX 4000", 1, 8, true
	case reKTT4Count.MatchString(lower):
		m := reKTT4Count.FindStringSubmatch(lower)
		return "gpu", "NVIDIA T4", parseUint8Match(m, 1), 16, true
	case reKTT4Dot.MatchString(lower):
		return "gpu", "NVIDIA T4", 1, 16, true
	case strings.Contains(lower, ".amd.sdgpu"):
		return "gpu", "AMD Instinct MI250", 1, 128, true
	case strings.Contains(lower, ".gpu") || strings.Contains(lower, "gpu"):
		return "gpu", "GPU", 1, 0, true
	default:
		return "", "", 0, 0, false
	}
}

func isPlaceholderAcceleratorModel(m string) bool {
	trimmed := strings.TrimSpace(m)
	return trimmed == "" || trimmed == "NA" || strings.HasPrefix(trimmed, "NA ") || strings.HasSuffix(trimmed, " NA")
}

// convertAWSMiBToGB converts AWS EC2 GpuInfo.MemoryInfo.SizeInMiB to nominal GPU memory (GB).
// AWS reports exact binary GiB multiples (mib % 1024 == 0) for some GPUs (e.g., T4=16384->16, V100=32768->32,
// A100=40960/81920->40/80, H100=81920->80, H200=144384->141, RTX PRO 6000=98304->96), and decimal GB converted
// to MiB (mib % 1024 != 0) for others (e.g., L4=22888->24, L40S=45776->48, B200=183359->192, B300=275040->288).
// CB-Spider's ConvertMiBToGBInt64 unconditionally multiplies by 1.073741824, inflating binary GiB values (16->17, 32->34, 80->86).
func convertAWSMiBToGB(mib int64) float32 {
	if mib <= 0 {
		return 0
	}
	if mib%1024 == 0 {
		return float32(mib / 1024)
	}
	return float32(math.Round(float64(mib) / 1024.0 * 1.073741824))
}

// enrichAWSAcceleratorInfo extracts GPU count, partition fraction (GpuPartitionSize), LogicalGpuCount,
// and memory directly from the EC2 "GpuInfo" object stored in spec.Details.
func enrichAWSAcceleratorInfo(spec *model.SpecInfo) bool {
	gpuInfoRaw := common.LookupKeyValueList(spec.Details, "GpuInfo")
	if gpuInfoRaw == "" {
		return false
	}
	changed := false

	if spec.AcceleratorType == "" {
		spec.AcceleratorType = "gpu"
		changed = true
	}

	if isPlaceholderAcceleratorModel(spec.AcceleratorModel) {
		var mfr, name string
		if m := reAWSGpuMfr.FindStringSubmatch(gpuInfoRaw); len(m) > 1 {
			mfr = strings.TrimSpace(m[1])
		}
		if m := reAWSGpuName.FindStringSubmatch(gpuInfoRaw); len(m) > 1 {
			name = strings.TrimSpace(m[1])
		}
		modelStr := strings.TrimSpace(mfr + " " + name)
		if modelStr != "" && spec.AcceleratorModel != modelStr {
			spec.AcceleratorModel = modelStr
			changed = true
		}
	}

	if m := reAWSGpuPartitionSize.FindStringSubmatch(gpuInfoRaw); len(m) > 1 {
		if partVal, err := strconv.ParseFloat(m[1], 32); err == nil && partVal > 0 && partVal < 1 {
			normPart := normalizePartitionFraction(float32(partVal))
			if spec.AcceleratorPartitionSize != normPart {
				spec.AcceleratorPartitionSize = normPart
				changed = true
			}
			logicalCnt := uint8(1)
			if lm := reAWSLogicalGpuCount.FindStringSubmatch(gpuInfoRaw); len(lm) > 1 {
				if v, err := strconv.ParseUint(lm[1], 10, 8); err == nil && v > 0 {
					logicalCnt = uint8(v)
				}
			}
			if spec.AcceleratorCount == 0 {
				spec.AcceleratorCount = logicalCnt
				changed = true
			}
		}
	} else if spec.AcceleratorCount == 0 {
		if m := reAWSGpuCount.FindStringSubmatch(gpuInfoRaw); len(m) > 1 {
			if v, err := strconv.ParseUint(m[1], 10, 8); err == nil && v > 0 {
				spec.AcceleratorCount = uint8(v)
				changed = true
			}
		}
	}

	if m := reAWSGpuSizeInMiB.FindStringSubmatch(gpuInfoRaw); len(m) > 1 {
		if mib, err := strconv.ParseInt(m[1], 10, 64); err == nil && mib > 0 {
			memGB := convertAWSMiBToGB(mib)
			if memGB > 0 && spec.AcceleratorMemoryGB != memGB {
				spec.AcceleratorMemoryGB = memGB
				changed = true
			}
		}
	}

	return changed
}

// enrichAlibabaAcceleratorInfo parses fractional GPU notation in Alibaba ECS GPUSpec strings
// (e.g., "NVIDIA A10*1/12", "NVIDIA T4/4", "NVIDIA VGPU8-2G") and derives partition ratios
// dynamically using the extracted GPU model and physical card memory.
func enrichAlibabaAcceleratorInfo(spec *model.SpecInfo) bool {
	if spec.AcceleratorType != "gpu" && !strings.Contains(strings.ToLower(spec.CspSpecName), "gn") {
		return false
	}
	changed := false

	// 1. Parse inline fraction suffix in AcceleratorModel (e.g., "NVIDIA A10*1/12" or "NVIDIA T4/4")
	if m := reAlibabaGPUFraction.FindStringSubmatch(strings.TrimSpace(spec.AcceleratorModel)); len(m) > 0 {
		baseModel := strings.TrimSpace(m[1])
		var num, den float64
		if m[2] != "" && m[3] != "" {
			num, _ = strconv.ParseFloat(m[2], 64)
			den, _ = strconv.ParseFloat(m[3], 64)
		} else if m[4] != "" {
			num = 1
			den, _ = strconv.ParseFloat(m[4], 64)
		}
		if baseModel != "" && spec.AcceleratorModel != baseModel {
			spec.AcceleratorModel = baseModel
			changed = true
		}
		if num > 0 && den > num {
			part := normalizePartitionFraction(float32(num / den))
			if spec.AcceleratorPartitionSize != part {
				spec.AcceleratorPartitionSize = part
				changed = true
			}
			if spec.AcceleratorCount == 0 {
				spec.AcceleratorCount = 1
				changed = true
			}
		}
	}

	// 2. Parse "VGPU<gen>-<mem>G" pattern in AcceleratorModel (e.g., "NVIDIA VGPU8-2G")
	if m := reAlibabaVGPUMem.FindStringSubmatch(spec.AcceleratorModel); len(m) > 2 {
		gen := m[1]
		if memVal, err := strconv.ParseFloat(m[2], 32); err == nil && memVal > 0 && spec.AcceleratorMemoryGB == 0 {
			spec.AcceleratorMemoryGB = float32(memVal)
			changed = true
		}
		if gen == "8" && spec.AcceleratorModel != "NVIDIA L20" {
			spec.AcceleratorModel = "NVIDIA L20"
			changed = true
		}
	}

	// 3. For shared/virtual GPU families (sgn* / vgn*), infer missing partition memory or partition ratio
	specLower := strings.ToLower(strings.TrimPrefix(strings.ToLower(spec.CspSpecName), "ecs."))
	if strings.HasPrefix(specLower, "sgn") || strings.HasPrefix(specLower, "vgn") {
		if spec.AcceleratorType == "" {
			spec.AcceleratorType = "gpu"
			changed = true
		}
		if spec.AcceleratorCount == 0 {
			spec.AcceleratorCount = 1
			changed = true
		}
		if spec.AcceleratorMemoryGB == 0 {
			if mm := reAlibabaSpecMemTag.FindStringSubmatch(specLower); len(mm) > 1 {
				if memVal, err := strconv.ParseFloat(mm[1], 32); err == nil && memVal > 0 {
					spec.AcceleratorMemoryGB = float32(memVal)
					changed = true
				}
			}
		}
		cardMem := inferGPUMemoryGBFromModel(spec.AcceleratorModel, spec.CspSpecName)
		if cardMem > 0 && spec.AcceleratorMemoryGB > 0 {
			if spec.AcceleratorMemoryGB < cardMem {
				part := normalizePartitionFraction(spec.AcceleratorMemoryGB / cardMem)
				if spec.AcceleratorPartitionSize != part {
					spec.AcceleratorPartitionSize = part
					changed = true
				}
			} else if spec.AcceleratorPartitionSize != 0 {
				spec.AcceleratorPartitionSize = 0
				changed = true
			}
		}
	}

	return changed
}

// enrichTencentAcceleratorInfo extracts GPU model from Tencent Cloud Details ("Remark", "InstanceFamily")
// when CB-Spider's Tencent driver leaves AcceleratorModel as "NA".
func enrichTencentAcceleratorInfo(spec *model.SpecInfo) bool {
	if spec.AcceleratorType != "gpu" {
		return false
	}
	changed := false

	// 1. Check explicit GPU model in Details["Remark"] first (e.g., "4 NVIDIA T4")
	var inferredModel string
	if remark := strings.TrimSpace(common.LookupKeyValueList(spec.Details, "Remark")); remark != "" {
		if m := reTencentRemarkGPU.FindStringSubmatch(remark); len(m) > 1 {
			inferredModel = strings.TrimSpace(m[1])
		}
	}

	// 2. Fallback to InstanceFamily from Details or spec name prefix when AcceleratorModel is placeholder
	if inferredModel == "" && isPlaceholderAcceleratorModel(spec.AcceleratorModel) {
		family := strings.ToLower(strings.TrimSpace(common.LookupKeyValueList(spec.Details, "InstanceFamily")))
		if family == "" {
			family = strings.ToLower(strings.SplitN(spec.CspSpecName, ".", 2)[0])
		}
		switch {
		case strings.HasPrefix(family, "gt4"):
			inferredModel = "NVIDIA A100"
		case strings.HasPrefix(family, "gn10x"), strings.HasPrefix(family, "hccg5v"):
			inferredModel = "NVIDIA V100"
		case strings.HasPrefix(family, "gn7"), strings.HasPrefix(family, "gi1"), strings.HasPrefix(family, "bmg5t"):
			inferredModel = "NVIDIA T4"
		case strings.HasPrefix(family, "gn8"):
			inferredModel = "NVIDIA P40"
		case strings.HasPrefix(family, "gn6s"):
			inferredModel = "NVIDIA P4"
		case strings.HasPrefix(family, "gnv4"):
			inferredModel = "AMD Radeon Pro V520"
		case strings.HasPrefix(family, "gc50"):
			inferredModel = "NVIDIA L20"
		}
	}

	if inferredModel != "" && spec.AcceleratorModel != inferredModel {
		spec.AcceleratorModel = inferredModel
		// Reset memory if model was corrected (e.g. from Remark) so unified model-to-VRAM inference recalculates it
		if !isPlaceholderAcceleratorModel(spec.AcceleratorModel) {
			if memGB := inferGPUMemoryGBFromModel(inferredModel, spec.CspSpecName); memGB > 0 {
				spec.AcceleratorMemoryGB = memGB
			}
		}
		changed = true
	}

	return changed
}

// enrichAzureAcceleratorInfo derives GPU model, count, partition fraction, and VRAM for Azure N-series
// VM sizes using:
//  1. Azure ResourceSKU "GPUs" capability from spec.Details
//  2. Azure partitioned GPU series ratio rules (NVas_v4, NVads_V710_v5, NGads_V620_v1, NVads_A10_v5)
//  3. Embedded GPU chip token rules in the Azure SKU name (rtxpro6000, gb300, gb200, h200, h100, mi300x, a100, a10, t4)
func enrichAzureAcceleratorInfo(spec *model.SpecInfo) bool {
	lower := strings.ToLower(spec.CspSpecName)
	trimmed := strings.TrimPrefix(lower, "standard_")
	isGPUSeries := strings.HasPrefix(trimmed, "nc") || strings.HasPrefix(trimmed, "nd") ||
		strings.HasPrefix(trimmed, "ng") || strings.HasPrefix(trimmed, "nv")
	if !isGPUSeries && spec.AcceleratorType != "gpu" {
		return false
	}

	changed := false
	if spec.AcceleratorType != "gpu" {
		spec.AcceleratorType = "gpu"
		changed = true
	}

	// Authoritative GPU device count from Azure ResourceSKU capability ("GPUs" in spec.Details)
	var detailsGPUs uint8
	if gpusStr := common.LookupKeyValueList(spec.Details, "GPUs"); gpusStr != "" {
		if v, err := strconv.ParseUint(strings.TrimSpace(gpusStr), 10, 8); err == nil && v > 0 {
			detailsGPUs = uint8(v)
		}
	}

	vcpu := float64(spec.VCPU)
	if vcpu <= 0 {
		if m := regexp.MustCompile(`^(?:nc|nd|ng|nv|ncc)(\d+)`).FindStringSubmatch(trimmed); len(m) > 1 {
			if v, err := strconv.ParseFloat(m[1], 64); err == nil {
				vcpu = v
			}
		}
	}

	// 1. Partitioned (fractional) GPU series in Azure
	type azurePartitionedSeries struct {
		matched     bool
		model       string
		cardMemGB   float32
		fullGpuVCPU float64
		shares      float64
	}

	var partSeries azurePartitionedSeries
	switch {
	case strings.HasPrefix(trimmed, "nv") && strings.Contains(trimmed, "v710"):
		// NVads_V710_v5 series: AMD Radeon PRO V710 (24 GB physical card, 24 vCPUs per full GPU; NV28 is 1 full GPU)
		shares := vcpu / 24.0
		if shares > 1.0 && shares < 1.5 {
			shares = 1.0
		}
		partSeries = azurePartitionedSeries{
			matched:     true,
			model:       "AMD Radeon PRO V710",
			cardMemGB:   24,
			fullGpuVCPU: 24,
			shares:      shares,
		}
	case strings.HasPrefix(trimmed, "ng") && strings.Contains(trimmed, "v620"):
		// NGads_V620_v1 series: AMD Radeon PRO V620 (32 GB physical card, 32 vCPUs per full GPU)
		partSeries = azurePartitionedSeries{
			matched:     true,
			model:       "AMD Radeon PRO V620",
			cardMemGB:   32,
			fullGpuVCPU: 32,
			shares:      vcpu / 32.0,
		}
	case strings.HasPrefix(trimmed, "nv") && strings.Contains(trimmed, "a10"):
		// NVads_A10_v5 series: NVIDIA A10 (24 GB physical card, 36 vCPUs per full GPU).
		// Constrained-vCPU variants (NV4, NV10, NV20, NV30, NV40) map to standard partition tiers (1/6, 1/3, 1/2, 1, 2).
		var shares float64
		switch {
		case vcpu <= 6:
			shares = 1.0 / 6.0
		case vcpu <= 12:
			shares = 1.0 / 3.0
		case vcpu <= 20:
			shares = 0.5
		case vcpu <= 40:
			shares = 1.0
		default:
			shares = math.Round(vcpu / 36.0)
		}
		partSeries = azurePartitionedSeries{
			matched:     true,
			model:       "NVIDIA A10",
			cardMemGB:   24,
			fullGpuVCPU: 36,
			shares:      shares,
		}
	case strings.HasPrefix(trimmed, "nv") && strings.Contains(trimmed, "_v4"):
		// NVas_v4 series: AMD Instinct MI25 (16 GB physical card, 32 vCPUs per full GPU)
		partSeries = azurePartitionedSeries{
			matched:     true,
			model:       "AMD Instinct MI25",
			cardMemGB:   16,
			fullGpuVCPU: 32,
			shares:      vcpu / 32.0,
		}
	}

	if partSeries.matched && partSeries.shares > 0 {
		if spec.AcceleratorModel != partSeries.model {
			spec.AcceleratorModel = partSeries.model
			changed = true
		}
		if partSeries.shares < 1.0 {
			part := normalizePartitionFraction(float32(partSeries.shares))
			partMem := float32(math.Round(float64(partSeries.cardMemGB) * partSeries.shares))
			if spec.AcceleratorCount != 1 {
				spec.AcceleratorCount = 1
				changed = true
			}
			if spec.AcceleratorPartitionSize != part {
				spec.AcceleratorPartitionSize = part
				changed = true
			}
			if spec.AcceleratorMemoryGB != partMem {
				spec.AcceleratorMemoryGB = partMem
				changed = true
			}
		} else {
			cnt := uint8(math.Round(partSeries.shares))
			if detailsGPUs > 0 {
				cnt = detailsGPUs
			}
			if spec.AcceleratorCount != cnt {
				spec.AcceleratorCount = cnt
				changed = true
			}
			if spec.AcceleratorPartitionSize != 0 {
				spec.AcceleratorPartitionSize = 0
				changed = true
			}
			if spec.AcceleratorMemoryGB != partSeries.cardMemGB {
				spec.AcceleratorMemoryGB = partSeries.cardMemGB
				changed = true
			}
		}
		return changed
	}

	// 2. Full-GPU Azure series: infer GPU model, per-GPU VRAM, and default vCPUs-per-GPU from chip token
	var modelName string
	var memPerGPU float32
	var defaultVCPUPerGPU float64
	overrideSpiderFallback := false

	switch {
	case strings.Contains(trimmed, "rtxpro6000"):
		modelName = "NVIDIA RTX PRO 6000"
		memPerGPU = 96
		defaultVCPUPerGPU = 144
		overrideSpiderFallback = true
	case strings.Contains(trimmed, "gb300"):
		modelName = "NVIDIA GB300"
		memPerGPU = 288
		defaultVCPUPerGPU = 32
		overrideSpiderFallback = true
	case strings.Contains(trimmed, "gb200"):
		modelName = "NVIDIA GB200"
		memPerGPU = 186
		defaultVCPUPerGPU = 32
		overrideSpiderFallback = true
	case strings.Contains(trimmed, "b200"):
		modelName = "NVIDIA B200"
		memPerGPU = 192
		defaultVCPUPerGPU = 24
		overrideSpiderFallback = true
	case strings.Contains(trimmed, "h200"):
		modelName = "NVIDIA H200"
		memPerGPU = 141
		defaultVCPUPerGPU = 12
	case strings.Contains(trimmed, "h100"):
		modelName = "NVIDIA H100"
		if strings.HasPrefix(trimmed, "nc") {
			memPerGPU = 94
			defaultVCPUPerGPU = 40
		} else {
			memPerGPU = 80
			defaultVCPUPerGPU = 12
		}
	case strings.Contains(trimmed, "mi300x"):
		modelName = "AMD Instinct MI300X"
		memPerGPU = 192
		defaultVCPUPerGPU = 12
	case strings.Contains(trimmed, "a100"):
		modelName = "NVIDIA A100"
		if strings.Contains(trimmed, "amsr") || strings.HasPrefix(trimmed, "nc") {
			memPerGPU = 80
		} else {
			memPerGPU = 40
		}
		if strings.HasPrefix(trimmed, "nc") {
			defaultVCPUPerGPU = 24
		} else {
			defaultVCPUPerGPU = 12
		}
		if strings.Contains(trimmed, "amsr") {
			overrideSpiderFallback = true
		}
	case strings.Contains(trimmed, "a10"):
		modelName = "NVIDIA A10"
		memPerGPU = 24
		defaultVCPUPerGPU = 16
		overrideSpiderFallback = true
	case strings.Contains(trimmed, "t4"):
		modelName = "NVIDIA Tesla T4"
		memPerGPU = 16
		defaultVCPUPerGPU = 16
	case strings.HasPrefix(trimmed, "nv") && (strings.Contains(trimmed, "_v2") || strings.Contains(trimmed, "_v3")):
		modelName = "NVIDIA Tesla M60"
		memPerGPU = 8
		if strings.Contains(trimmed, "_v3") {
			defaultVCPUPerGPU = 12
		} else {
			defaultVCPUPerGPU = 6
		}
		overrideSpiderFallback = true
	}

	if modelName != "" && (isPlaceholderAcceleratorModel(spec.AcceleratorModel) || overrideSpiderFallback) {
		if spec.AcceleratorModel != modelName {
			spec.AcceleratorModel = modelName
			changed = true
		}
	}

	normalized := normalizeAcceleratorModel(spec.AcceleratorModel)
	if normalized != spec.AcceleratorModel {
		spec.AcceleratorModel = normalized
		changed = true
	}

	if overrideSpiderFallback && spec.AcceleratorPartitionSize != 0 {
		spec.AcceleratorPartitionSize = 0
		changed = true
	}

	if detailsGPUs > 0 && (spec.AcceleratorCount == 0 || overrideSpiderFallback || spec.AcceleratorCount != detailsGPUs) {
		spec.AcceleratorCount = detailsGPUs
		changed = true
	} else if (spec.AcceleratorCount == 0 || overrideSpiderFallback) && defaultVCPUPerGPU > 0 && vcpu > 0 {
		cnt := uint8(math.Max(1, math.Round(vcpu/defaultVCPUPerGPU)))
		if spec.AcceleratorCount != cnt {
			spec.AcceleratorCount = cnt
			changed = true
		}
	}

	if memPerGPU > 0 && (spec.AcceleratorMemoryGB <= 0 || overrideSpiderFallback) {
		if spec.AcceleratorMemoryGB != memPerGPU {
			spec.AcceleratorMemoryGB = memPerGPU
			changed = true
		}
	}

	return changed
}

// normalizeMemoryGiB cleans up floating-point division artifacts when MemSizeMiB is not an exact
// binary GiB fraction (1/8 GiB = 128 MiB). For example, GCP n1-highcpu-* (0.9 GB/vCPU) reports
// 1843 MiB -> 1.7998047 GiB (normalized to 1.8) and AWS m2.xlarge reports 17510 MiB -> 17.09961 GiB (normalized to 17.1).
func normalizeMemoryGiB(memGiB float32) float32 {
	if memGiB <= 0 {
		return 0
	}
	val := float64(memGiB)
	if math.Abs(val*8-math.Round(val*8)) < 1e-4 {
		return float32(math.Round(val*8) / 8)
	}
	return float32(math.Round(val*10) / 10)
}

// enrichSpecAcceleratorInfo fills in missing or placeholder GPU accelerator fields and normalizes
// CSP spec metadata (MemoryGiB precision, Architecture from Details, Azure constrained vCPUs, and
// Azure ResourceDiskSizeInMB unit conversion). Returns true if any field was modified.
func enrichSpecAcceleratorInfo(spec *model.SpecInfo) bool {
	if spec == nil {
		return false
	}
	changed := false

	// 1. Normalize MemoryGiB floating-point precision (e.g., 1.7998047 -> 1.8, 32.015625 -> 32)
	if spec.MemoryGiB > 0 {
		normMem := normalizeMemoryGiB(spec.MemoryGiB)
		if math.Abs(float64(spec.MemoryGiB-normMem)) > 1e-5 {
			spec.MemoryGiB = normMem
			changed = true
		}
	}

	// 2. Re-verify CPU Architecture using Details and updated CSP naming patterns
	if arch := extractArchitecture(spec.ProviderName, spec.Details, spec.CspSpecName); arch != "" && arch != string(model.ArchitectureUnknown) {
		if spec.Architecture != arch {
			spec.Architecture = arch
			changed = true
		}
	}

	switch csp.ResolveCloudPlatform(spec.ProviderName) {
	case csp.AWS:
		if enrichAWSAcceleratorInfo(spec) {
			changed = true
		}

	case csp.Azure:
		// Normalize Azure constrained-vCPU SKUs (e.g. Standard_E16-4s_v5, Standard_F64-16ams_v7)
		// where Details["vCPUsAvailable"] gives the actual usable vCPU count inside the VM.
		if availStr := strings.TrimSpace(common.LookupKeyValueList(spec.Details, "vCPUsAvailable")); availStr != "" {
			if v, err := strconv.ParseUint(availStr, 10, 16); err == nil && v > 0 && spec.VCPU != uint16(v) {
				spec.VCPU = uint16(v)
				changed = true
			}
		}
		// Normalize Azure DiskSizeGB from Details["ResourceDiskSizeInMB"] to fix CB-Spider's 1.07374x inflation (e.g., 16->17, 32->34)
		if rdStr := strings.TrimSpace(common.LookupKeyValueList(spec.Details, "ResourceDiskSizeInMB")); rdStr != "" {
			if mb, err := strconv.ParseInt(rdStr, 10, 64); err == nil && mb > 0 {
				diskGB := convertAWSMiBToGB(mb)
				if diskGB > 0 && spec.DiskSizeGB != diskGB {
					if spec.RootDiskSize == int(spec.DiskSizeGB) {
						spec.RootDiskSize = int(diskGB)
					}
					spec.DiskSizeGB = diskGB
					changed = true
				}
			}
		}
		if enrichAzureAcceleratorInfo(spec) {
			changed = true
		}

	case csp.Alibaba:
		if enrichAlibabaAcceleratorInfo(spec) {
			changed = true
		}

	case csp.KT:
		if accelType, modelName, count, memGB, ok := parseKTAcceleratorFromSpecName(spec.CspSpecName); ok {
			if spec.AcceleratorType != accelType {
				spec.AcceleratorType = accelType
				changed = true
			}
			if (isPlaceholderAcceleratorModel(spec.AcceleratorModel) || spec.AcceleratorModel == "AMD GPU") && modelName != "" && spec.AcceleratorModel != modelName {
				spec.AcceleratorModel = modelName
				changed = true
			}
			if spec.AcceleratorCount == 0 && count > 0 {
				spec.AcceleratorCount = count
				changed = true
			}
			if spec.AcceleratorMemoryGB == 0 && memGB > 0 {
				spec.AcceleratorMemoryGB = memGB
				changed = true
			}
		}

	case csp.Tencent:
		if enrichTencentAcceleratorInfo(spec) {
			changed = true
		}
	}

	// 3. Unified cross-CSP GPU manufacturer normalization and model-to-VRAM inference
	if spec.AcceleratorType == "gpu" {
		normalized := normalizeAcceleratorModel(spec.AcceleratorModel)
		if normalized != spec.AcceleratorModel {
			spec.AcceleratorModel = normalized
			changed = true
		}
		if spec.AcceleratorMemoryGB == 0 && !isPlaceholderAcceleratorModel(spec.AcceleratorModel) {
			if memGB := inferGPUMemoryGBFromModel(spec.AcceleratorModel, spec.CspSpecName); memGB > 0 {
				spec.AcceleratorMemoryGB = memGB
				changed = true
			}
		}
	}

	return changed
}

// normalizeAcceleratorModel corrects the manufacturer prefix based on known GPU model names.
// Spider drivers may report incorrect manufacturer info (e.g., IBM labels Intel Gaudi and AMD MI300X as NVIDIA).
// Models not listed here are left unchanged and trusted as-is.
func normalizeAcceleratorModel(model string) string {
	upper := strings.ToUpper(strings.TrimSpace(model))
	if strings.HasPrefix(upper, "AMD ") || strings.HasPrefix(upper, "INTEL ") {
		return model
	}

	// Intel GPU models (order matters: longer/more specific first)
	for _, m := range []string{"GAUDI3", "GAUDI2", "GAUDI"} {
		if strings.Contains(upper, m) {
			return "Intel " + m
		}
	}

	// AMD GPU models
	for _, m := range []string{
		"RADEON PRO V710", "RADEON PRO V620",
		"MI300X", "MI300A", "MI300", "MI250X", "MI250", "MI210", "MI100", "INSTINCT",
	} {
		if strings.Contains(upper, m) {
			return "AMD " + m
		}
	}

	return model
}

// extractArchitecture extracts architecture information based on CSP-specific logic
func extractArchitecture(providerName string, details []model.KeyValue, cspSpecName string) string {

	// FYI model.OSArchitecture is defined in src/core/model/OSArchitecture.go
	// 	const (
	// 	ARM32          OSArchitecture = "arm32"
	// 	ARM64          OSArchitecture = "arm64"
	// 	ARM64_MAC      OSArchitecture = "arm64_mac"
	// 	X86_32         OSArchitecture = "x86_32"
	// 	X86_64         OSArchitecture = "x86_64"
	// 	X86_32_MAC     OSArchitecture = "x86_32_mac"
	// 	X86_64_MAC     OSArchitecture = "x86_64_mac"
	// 	S390X          OSArchitecture = "s390x"
	// 	ArchitectureNA OSArchitecture = "NA"
	// )

	switch csp.ResolveCloudPlatform(providerName) {
	case csp.AWS:
		// For AWS, look for ProcessorInfo and extract SupportedArchitectures from its value
		archInfo := common.LookupKeyValueList(details, "ProcessorInfo")
		if archInfo != "" {
			// Parse the SupportedArchitectures from ProcessorInfo value
			// Examples:
			// "{SupportedArchitectures:[arm64],SustainedClockSpeedInGhz:2.6}"
			// "{SupportedArchitectures:[x86_64_mac],SustainedClockSpeedInGhz:3.2}"
			// "{SupportedArchitectures:[i386,x86_64],SustainedClockSpeedInGhz:2.5}"

			if strings.Contains(archInfo, "arm64_mac") {
				return string(model.ARM64_MAC)
			} else if strings.Contains(archInfo, "x86_64_mac") {
				return string(model.X86_64_MAC)
			} else if strings.Contains(archInfo, "arm64") {
				return string(model.ARM64)
			} else if strings.Contains(archInfo, "x86_64") {
				return string(model.X86_64)
			} else if strings.Contains(archInfo, "i386") {
				return string(model.X86_32)
			} else {
				return archInfo
			}
		}

	case csp.Alibaba:
		// For Alibaba, CpuArchitecture is a direct key
		archInfo := strings.ToLower(common.LookupKeyValueList(details, "CpuArchitecture"))
		if archInfo != "" {
			if strings.Contains(archInfo, strings.ToLower("ARM")) {
				return string(model.ARM64)
			} else if strings.Contains(archInfo, strings.ToLower("X86")) {
				return string(model.X86_64)
			} else {
				return archInfo
			}
		}

	case csp.IBM:
		// For IBM, look for VcpuArchitecture and extract the actual value
		archInfo := common.LookupKeyValueList(details, "VcpuArchitecture")
		if archInfo != "" {
			// Parse the value from "{type:fixed,value:amd64}"
			if strings.Contains(archInfo, "s390x") {
				return string(model.S390X)
			} else if strings.Contains(archInfo, "amd64") {
				return string(model.X86_64)
			} else {
				return archInfo
			}
		}

	case csp.Tencent:
		// ref: https://www.tencentcloud.com/document/product/213/11518
		patterns := []string{
			"sr1.", // Standard ARM (Ampere Altra)
		}

		for _, pattern := range patterns {
			if strings.Contains(strings.ToLower(cspSpecName), strings.ToLower(pattern)) {
				return string(model.ARM64)
			}
		}
		return string(model.X86_64)

	case csp.Azure:
		// First check explicit CpuArchitectureType in Details (e.g., "Arm64" or "x64")
		archInfo := strings.ToLower(strings.TrimSpace(common.LookupKeyValueList(details, "CpuArchitectureType")))
		if archInfo != "" {
			if strings.Contains(archInfo, "arm") {
				return string(model.ARM64)
			}
			if strings.Contains(archInfo, "x64") || strings.Contains(archInfo, "x86") {
				return string(model.X86_64)
			}
		}
		// Fallback to naming convention: lowercase 'p' (Ampere/Cobalt ARM) or Grace Blackwell (GB200/GB300)
		lowerName := strings.ToLower(cspSpecName)
		if strings.Contains(cspSpecName, "p") || strings.Contains(lowerName, "gb200") || strings.Contains(lowerName, "gb300") {
			return string(model.ARM64)
		}
		return string(model.X86_64)

	case csp.GCP:
		// First check explicit Architecture in Details (e.g., "ARM64" or "X86_64")
		archInfo := strings.ToLower(strings.TrimSpace(common.LookupKeyValueList(details, "Architecture")))
		if archInfo != "" {
			if strings.Contains(archInfo, "arm") {
				return string(model.ARM64)
			}
			if strings.Contains(archInfo, "x86") || strings.Contains(archInfo, "x64") {
				return string(model.X86_64)
			}
		}
		// Fallback to ARM-specific machine family prefixes (Tau T2A, C2A, Axion C4A/N4A, Grace Blackwell A4X)
		patterns := []string{
			"t2a", "c2a", "c4a", "n4a", "a4x",
		}

		for _, pattern := range patterns {
			if strings.Contains(strings.ToLower(cspSpecName), strings.ToLower(pattern)) {
				return string(model.ARM64)
			}
		}
		return string(model.X86_64)

	case csp.KT:
		return string(model.X86_64)

	case csp.NCP:
		return string(model.X86_64)

	case csp.NHN:
		return string(model.X86_64)

	default:
		// For unknown CSPs
		return string(model.X86_64)
	}
	return string(model.ArchitectureUnknown)
}

// RegisterSpecWithCspResourceId accepts spec creation request, creates and returns an TB spec object
func RegisterSpecWithCspResourceId(nsId string, u *model.SpecReq, update bool) (model.SpecInfo, error) {

	content := model.SpecInfo{}

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return content, err
	}

	connConfig, err := common.GetConnConfig(u.ConnectionName)
	if err != nil {
		log.Error().Err(err).Msgf("Cannot GetConnConfig in %s", u.ConnectionName)
		return content, err
	}

	res, err := LookupSpec(u.ConnectionName, u.CspSpecName)
	if err != nil {
		log.Error().Err(err).Msgf("cannot LookupSpec ConnectionName(%s), CspResourceId(%s)", u.ConnectionName, u.CspSpecName)
		return content, err
	}

	content, err = ConvertSpiderSpecToTumblebugSpec(connConfig, res)
	if err != nil {
		log.Error().Err(err).Msg("cannot RegisterSpecWithCspResourceId")
		return content, err
	}

	content.Namespace = nsId
	content.Id = u.Name
	content.Name = u.Name
	content.ConnectionName = u.ConnectionName
	content.AssociatedObjectList = []string{}

	// "INSERT INTO `spec`(`namespace`, `id`, ...) VALUES ('nsId', 'content.Id', ...);
	result := model.ORM.Create(&content)
	if result.Error != nil {
		log.Error().Err(result.Error).Msg("Cannot insert data to RDB")
	} else {
		log.Trace().Msg("SQL: Insert success")
	}

	return content, nil
}

// RegisterSpecWithInfo accepts spec creation request, creates and returns an TB spec object
func RegisterSpecWithInfo(nsId string, content *model.SpecInfo, update bool) (model.SpecInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := model.SpecInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	content.Namespace = nsId
	content.Id = content.Name
	content.AssociatedObjectList = []string{}

	// "INSERT INTO `spec`(`namespace`, `id`, ...) VALUES ('nsId', 'content.Id', ...);
	// Attempt to insert the new record
	result := model.ORM.Create(content)
	if result.Error != nil {
		if update {
			updateResult := model.ORM.Model(&model.SpecInfo{}).
				Where("namespace = ? AND id = ?", content.Namespace, content.Id).
				Updates(content)

			if updateResult.Error != nil {
				log.Error().Err(updateResult.Error).Msg("Error updating spec after insert failure")
				return *content, updateResult.Error
			} else {
				log.Trace().Msg("SQL: Update success after insert failure")
			}
		} else {
			log.Error().Err(result.Error).Msg("Error inserting spec and update flag is false")
			return *content, result.Error
		}
	} else {
		log.Trace().Msg("SQL: Insert success")
	}

	return *content, nil
}

// RegisterSpecWithInfoInBulk register a list of specs in bulk
func RegisterSpecWithInfoInBulk(specList []model.SpecInfo) error {
	// In PostgreSQL, use session_replication_role instead of PRAGMA
	model.ORM.Exec("SET session_replication_role = 'replica'")

	// Batch size - PostgreSQL can handle larger batches
	batchSize := 100

	uniqueSpecs := make(map[string]model.SpecInfo)
	for _, spec := range specList {
		key := spec.Namespace + ":" + spec.Id
		uniqueSpecs[key] = spec
	}
	dedupedSpecList := make([]model.SpecInfo, 0, len(uniqueSpecs))
	for _, spec := range uniqueSpecs {
		dedupedSpecList = append(dedupedSpecList, spec)
	}

	total := len(dedupedSpecList)
	for i := 0; i < total; i += batchSize {
		end := min(i+batchSize, total)
		batch := dedupedSpecList[i:end]

		// Start transaction
		tx := model.ORM.Begin()
		if tx.Error != nil {
			log.Error().Err(tx.Error).Msg("Failed to begin transaction")
			return tx.Error
		}

		// Use PostgreSQL's more concise UPSERT approach: UpdateAll: true
		result := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "namespace"}, {Name: "id"}},
			UpdateAll: true, // Automatically update all fields (no need to specify individual fields)
		}).CreateInBatches(&batch, len(batch))

		if result.Error != nil {
			tx.Rollback()
			log.Error().Err(result.Error).Msg("Error upserting specs in bulk")
			return result.Error
		}

		if err := tx.Commit().Error; err != nil {
			log.Error().Err(err).Msg("Failed to commit transaction")
			return err
		}

		// log.Info().Msgf("Bulk upsert success: batch %d-%d, affected: %d records",
		// 	i, end-1, result.RowsAffected)
	}

	// Re-enable foreign key constraints
	//model.ORM.Exec("SET session_replication_role = 'origin'")
	return nil
}

// RemoveDuplicateSpecsInSQL is to remove duplicate specs in db to refine batch insert duplicates
func RemoveDuplicateSpecsInSQL() error {
	// PostgreSQL deduplication query
	sqlStr := `
    DELETE FROM spec_infos
    WHERE ctid NOT IN (
        SELECT MIN(ctid)
        FROM spec_infos
        GROUP BY namespace, id
    );
    `

	result := model.ORM.Exec(sqlStr)
	if result.Error != nil {
		log.Error().Err(result.Error).Msg("Error deleting duplicate specs")
		return result.Error
	}
	log.Info().Msg("Duplicate specs removed successfully")

	return nil
}

// Range struct is for 'FilterSpecsByRange'
type Range struct {
	Min float32 `json:"min"`
	Max float32 `json:"max"`
}

// specLookupGroup collapses concurrent identical spec lookups into a single DB read. Large infra
// provisioning fans out thousands of nodes that share only a handful of specs; without this, each
// node issued its own query and the burst exhausted the DB connection pool.
var specLookupGroup singleflight.Group

// GetSpec accepts namespace Id and specKey(Id,CspResourceName,...), and returns the TB spec object
func GetSpec(nsId string, specKey string) (model.SpecInfo, error) {
	if err := common.CheckString(nsId); err != nil {
		log.Error().Err(err).Msg("Invalid namespace ID")
		return model.SpecInfo{}, err
	}

	log.Debug().Msg("[Get spec] " + specKey)

	// make comparison case-insensitive
	nsId = strings.ToLower(nsId)
	specKey = strings.ToLower(specKey)

	// Deduplicate concurrent identical lookups so a provisioning burst hits the DB once per spec.
	v, err, _ := specLookupGroup.Do(nsId+"\x00"+specKey, func() (interface{}, error) {
		// ex: tencent+ap-jakarta+ubuntu22.04
		var spec model.SpecInfo
		result := model.ORM.Where("LOWER(namespace) = ? AND LOWER(id) = ?", nsId, specKey).First(&spec)
		if result.Error == nil {
			return spec, nil
		}
		// Only a real "record not found" means the spec is absent; any other error (e.g. a
		// connection-pool timeout under load) must be surfaced as-is so callers can retry
		// instead of mistaking a transient DB failure for a missing spec.
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return model.SpecInfo{}, fmt.Errorf("spec lookup by id failed for %s: %w", specKey, result.Error)
		}

		// ex: spec-487zeit5
		result = model.ORM.Where("LOWER(namespace) = ? AND LOWER(csp_spec_name) = ?", nsId, specKey).First(&spec)
		if result.Error == nil {
			return spec, nil
		}
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return model.SpecInfo{}, fmt.Errorf("spec lookup by cspSpecName failed for %s: %w", specKey, result.Error)
		}

		return model.SpecInfo{}, fmt.Errorf("The specKey %s not found by any of ID, CspSpecName", specKey)
	})
	if err != nil {
		return model.SpecInfo{}, err
	}
	spec := v.(model.SpecInfo)
	EnrichSpecWithAvailability(context.Background(), &spec, false)
	return spec, nil
}

// Retrieve field-to-column mapping information for the model
func getColumnMapping(modelType any) map[string]string {
	stmt := &gorm.Statement{DB: model.ORM}
	stmt.Parse(modelType)

	mapping := make(map[string]string)
	for _, field := range stmt.Schema.Fields {
		mapping[field.Name] = field.DBName
	}

	return mapping
}

// FilterSpecsByRange accepts criteria ranges for filtering, and returns the list of filtered TB spec objects
func FilterSpecsByRange(nsId string, filter model.FilterSpecsByRangeRequest, orderBy string, ctx ...context.Context) ([]model.SpecInfo, error) {
	if err := common.CheckString(nsId); err != nil {
		log.Error().Err(err).Msg("Invalid namespace ID")
		return nil, err
	}

	reqCtx := context.Background()
	if len(ctx) > 0 && ctx[0] != nil {
		reqCtx = ctx[0]
	}

	// Start building the query using field names as database column names
	query := model.ORM.Where("namespace = ?", nsId)

	specColumnMapping := getColumnMapping(&model.SpecInfo{})
	// Change field names to start with lowercase (GORM convention)
	val := reflect.ValueOf(filter)
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		value := val.Field(i)

		modelFieldName := field.Name

		// Skip Limit and CheckAvailability fields as they are not database columns
		if modelFieldName == "Limit" || modelFieldName == "CheckAvailability" {
			continue
		}

		dbFieldName, exists := specColumnMapping[modelFieldName]
		if !exists {
			log.Warn().Msgf("Field %s not found in the model", modelFieldName)
			return nil, fmt.Errorf("Field %s not found in the model", modelFieldName)
		}

		if value.Kind() == reflect.Struct {
			min := value.FieldByName("Min")
			max := value.FieldByName("Max")

			if min.IsValid() && !min.IsZero() {
				query = query.Where(dbFieldName+" >= ?", min.Interface())
			}
			if max.IsValid() && !max.IsZero() {
				query = query.Where(dbFieldName+" <= ?", max.Interface())
			}
		} else if value.IsValid() && !value.IsZero() {
			switch value.Kind() {
			case reflect.String:
				cleanValue := strings.ToLower(value.String())

				// Define fields that require LIKE search for partial matching only
				// Use LIKE search sparingly due to performance impact on indexing
				likeSearchFields := []string{
					"AcceleratorModel", // e.g., "NVIDIA H100" -> search with "NVIDIA"
					"Description",      // Description text partial search
					// Note: AcceleratorType removed - uses exact matching for better performance
					// since values are typically standardized (GPU, TPU, etc.)
				}

				// Fields matched by AND-ed substrings: every whitespace/comma separated token
				// must appear somewhere in the value. A spec id is a compound of provider,
				// region and CSP name (e.g. "aws+eu-south-1+m5.metal"), so callers reach for
				// fragments of it rather than the whole string - "metal 5" should find
				// m5.metal without the caller reconstructing the full id.
				//
				// This does not cost an index: the surrounding LOWER() already prevents the
				// btree on (id, namespace) from being used, so exact match was scanning the
				// table too. Measured on 73k specs: LOWER(id) = ? took 211 ms, two AND-ed
				// LIKEs took 31 ms (fewer rows survive to be materialised).
				tokenSubstringFields := []string{
					"Id",          // e.g. "metal 5" -> aws+eu-south-1+m5.metal
					"CspSpecName", // e.g. "metal" -> m5.metal, m6i.metal, ...
				}

				// Check if current field requires LIKE search
				useLikeSearch := slices.Contains(likeSearchFields, modelFieldName)

				if slices.Contains(tokenSubstringFields, modelFieldName) {
					// Every token must match; an empty token list means no filter at all.
					tokens := strings.FieldsFunc(cleanValue, func(c rune) bool {
						return c == ',' || c == ' '
					})
					matched := 0
					for _, token := range tokens {
						token = strings.TrimSpace(token)
						if token == "" {
							continue
						}
						query = query.Where("LOWER("+dbFieldName+") LIKE ?", "%"+token+"%")
						matched++
					}
					if matched > 0 {
						log.Info().Msgf("Filtering by %s (SUBSTRING AND): %v", dbFieldName, tokens)
					}
				} else if useLikeSearch {
					// For LIKE search, use the original single value (don't support multiple values for LIKE)
					query = query.Where("LOWER("+dbFieldName+") LIKE ?", "%"+cleanValue+"%")
					log.Info().Msgf("Filtering by %s (LIKE): %s", dbFieldName, cleanValue)
				} else {
					// Check if the value contains multiple items separated by comma or space
					var values []string
					for _, item := range strings.FieldsFunc(cleanValue, func(c rune) bool {
						return c == ',' || c == ' '
					}) {
						if trimmed := strings.TrimSpace(item); trimmed != "" {
							values = append(values, trimmed)
						}
					}

					// For exact match fields, support multiple values
					if len(values) == 1 {
						// Single value - use exact match
						query = query.Where("LOWER("+dbFieldName+") = ?", values[0])
						log.Info().Msgf("Filtering by %s (SINGLE): %s", dbFieldName, values[0])
					} else if len(values) > 1 {
						// Multiple values - use IN clause
						query = query.Where("LOWER("+dbFieldName+") IN ?", values)
						log.Info().Msgf("Filtering by %s (MULTIPLE): %v", dbFieldName, values)
					}
					// Note: if len(values) == 0 (empty after parsing), no filter is applied for this field
					// This allows filtering to be skipped when only whitespace/separators are provided
				}
			}
		}
	}

	startTime := time.Now()

	var specs []model.SpecInfo

	// Apply ORDER BY if specified
	if orderBy != "" {
		query = query.Order(orderBy)
		// log.Debug().Msgf("Applying ORDER BY: %s", orderBy)
	}

	// Apply limit if specified and greater than 0
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
		// log.Debug().Msgf("Applying LIMIT: %d", filter.Limit)
	}

	// Check the query before executing (only in debug mode to avoid performance impact)
	// query = query.Debug()
	result := query.Find(&specs)
	if result.Error != nil {
		log.Error().Err(result.Error).Msg("Failed to execute query")
		return nil, result.Error
	}

	elapsedTime := time.Since(startTime)
	log.Info().
		Int("resultCount", len(specs)).
		Int("limitApplied", filter.Limit).
		Dur("elapsedTime", elapsedTime).
		Msg("ORM:session.Find(&specs)")

	EnrichSpecsWithAvailability(reqCtx, specs, shouldActivelyCheckAvailability(filter, len(specs)))

	return specs, nil
}

// UpdateSpec accepts to-be TB spec objects,
// updates and returns the updated TB spec objects
func UpdateSpec(nsId string, specId string, fieldsToUpdate model.SpecInfo) (model.SpecInfo, error) {

	result := model.ORM.Model(&model.SpecInfo{}).
		Where("namespace = ? AND id = ?", nsId, specId).
		Updates(fieldsToUpdate)

	if result.Error != nil {
		log.Error().Err(result.Error).Msg("")
		return fieldsToUpdate, result.Error
	} else {
		log.Trace().Msg("SQL: Update success")
	}

	return fieldsToUpdate, nil
}

// BulkUpdateSpec updates multiple specs with proper type casting
func BulkUpdateSpec(nsId string, updates map[string]float32) (int, error) {
	if len(updates) == 0 {
		return 0, nil
	}

	// Extract spec IDs for WHERE IN clause
	specIds := make([]string, 0, len(updates))
	for specId := range updates {
		specIds = append(specIds, specId)
	}

	// Build CASE statement with explicit CAST
	var caseClause strings.Builder
	caseClause.WriteString("CASE id ")
	args := make([]any, 0, len(updates)*2)

	for specId, price := range updates {
		caseClause.WriteString("WHEN ? THEN CAST(? AS NUMERIC) ")
		args = append(args, specId, price)
	}
	caseClause.WriteString("END")

	// Execute with proper casting
	result := model.ORM.Model(&model.SpecInfo{}).
		Where("namespace = ? AND id IN ?", nsId, specIds).
		Update("cost_per_hour", gorm.Expr(caseClause.String(), args...))

	if result.Error != nil {
		return 0, result.Error
	}

	return int(result.RowsAffected), nil
}

// isAzureGen1OnlySpec checks if the given Azure VM spec supports only Hyper-V Gen1.
// It first consults the HyperVGenerations field from the Azure API (authoritative),
// then falls back to name-based heuristics when that field is absent.
func isAzureGen1OnlySpec(specName string, keyValueList []model.KeyValue) bool {
	if specName == "" {
		return false
	}

	// Prefer the authoritative HyperVGenerations field returned by the Azure API.
	for _, kv := range keyValueList {
		if kv.Key == "HyperVGenerations" {
			v := strings.TrimSpace(kv.Value)
			supportsV1 := strings.Contains(v, "V1")
			supportsV2 := strings.Contains(v, "V2")
			return supportsV1 && !supportsV2
		}
	}

	// Fallback: name-based heuristics for specs missing the HyperVGenerations field.
	lowerSpecName := strings.ToLower(specName)

	var family string
	if strings.HasPrefix(lowerSpecName, "standard_") {
		remaining := lowerSpecName[9:]
		if len(remaining) > 0 {
			family = string(remaining[0])
		}
	} else if strings.HasPrefix(lowerSpecName, "basic_") {
		remaining := lowerSpecName[6:]
		if len(remaining) > 0 {
			family = string(remaining[0])
		}
	} else {
		if len(lowerSpecName) > 0 {
			family = string(lowerSpecName[0])
		}
	}

	switch family {
	case "a":
		// All A-series VMs are Gen1-only, including Av2 (Standard_A1_v2 etc.).
		// The "_v2" suffix denotes a size revision, NOT Hyper-V Generation 2.
		return true
	case "d":
		// D-family with digit+s pattern (e.g., Standard_D2s_v3) supports Gen2.
		for i := 0; i < len(lowerSpecName)-1; i++ {
			if lowerSpecName[i] >= '0' && lowerSpecName[i] <= '9' && lowerSpecName[i+1] == 's' {
				return false
			}
		}
		return true
	}

	return false
}

// logSpecsToIgnoreInfo logs the specs to ignore information in structured format
// This is more suitable for containerized environments than creating files
func logSpecsToIgnoreInfo(provider string, specsToIgnore map[string]map[string][]string, globalIgnoreSpecs map[string][]string) {
	// Prepare structured data for logging
	data := model.SpecsToIgnoreData{
		LastUpdated:          time.Now(),
		Description:          "Specs that should be ignored during availability checks. Global specs are unavailable in all regions, region-specific specs are unavailable only in specific regions.",
		GlobalIgnoreSpecs:    make(map[string][]string),
		RegionSpecificIgnore: make(map[string]map[string][]string),
	}

	// Prepare global ignore specs for logging
	for cspProvider, specs := range globalIgnoreSpecs {
		if len(specs) > 0 {
			sort.Strings(specs)
			data.GlobalIgnoreSpecs[cspProvider] = specs
		}
	}

	// Prepare region-specific ignore specs for logging
	for cspProvider, regions := range specsToIgnore {
		if data.RegionSpecificIgnore[cspProvider] == nil {
			data.RegionSpecificIgnore[cspProvider] = make(map[string][]string)
		}

		for region, specs := range regions {
			if len(specs) > 0 {
				sort.Strings(specs)
				data.RegionSpecificIgnore[cspProvider][region] = specs
			}
		}
	}

	// Log the specs to ignore information in structured format
	globalCount := 0
	regionalCount := 0

	// Count global specs
	if globalSpecs, exists := data.GlobalIgnoreSpecs[provider]; exists {
		globalCount = len(globalSpecs)
	}

	// Count regional specs
	if regions, exists := data.RegionSpecificIgnore[provider]; exists {
		for _, specs := range regions {
			regionalCount += len(specs)
		}
	}

	// Create detailed log entry with all specs information
	logEvent := log.Info().
		Str("provider", provider).
		Int("globalIgnoreCount", globalCount).
		Int("regionSpecificCount", regionalCount).
		Time("timestamp", data.LastUpdated)

	// Add global specs to log if any
	if globalSpecs, exists := data.GlobalIgnoreSpecs[provider]; exists && len(globalSpecs) > 0 {
		logEvent = logEvent.Strs("globalIgnoreSpecs", globalSpecs)
	}

	// Add regional specs to log if any
	if regions, exists := data.RegionSpecificIgnore[provider]; exists && len(regions) > 0 {
		for region, specs := range regions {
			if len(specs) > 0 {
				logEvent = logEvent.Strs(fmt.Sprintf("regionIgnoreSpecs_%s", region), specs)
			}
		}
	}

	// Log the complete information
	logEvent.Msg("Specs to ignore information logged for container environment")

	// Also log a summary for easy monitoring
	log.Debug().
		Str("provider", provider).
		Interface("specsToIgnoreData", data).
		Msg("Complete specs to ignore data structure")
}

// Global variable to cache the ignore config
var (
	ignoreConfig     *model.CloudSpecIgnoreConfig
	ignoreConfigOnce sync.Once
	ignoreConfigErr  error
)

// loadCloudSpecIgnoreConfig loads the cloudspec_ignore.yaml file using Viper
func loadCloudSpecIgnoreConfig() (*model.CloudSpecIgnoreConfig, error) {
	ignoreConfigOnce.Do(func() {
		// Create a new Viper instance for the ignore config
		ignoreViper := viper.New()

		// Add possible config paths using centralized helper
		common.SetupViperPaths(ignoreViper)
		ignoreViper.SetConfigName("cloudspec_ignore")
		ignoreViper.SetConfigType("yaml")

		// Try to read the config file
		err := ignoreViper.ReadInConfig()
		if err != nil {
			log.Warn().Err(err).Msg("Could not load cloudspec_ignore.yaml, no spec filtering will be applied")
			ignoreConfigErr = err
			return
		}

		log.Debug().Str("path", ignoreViper.ConfigFileUsed()).Msg("Found cloudspec_ignore.yaml")

		// Manual extraction to handle Viper's type conversion issues
		var config model.CloudSpecIgnoreConfig

		// Extract global patterns
		if globalPatternsRaw := ignoreViper.Get("global.patterns"); globalPatternsRaw != nil {
			if patterns, ok := globalPatternsRaw.([]any); ok {
				for _, pattern := range patterns {
					if str, ok := pattern.(string); ok {
						config.Global.Patterns = append(config.Global.Patterns, str)
					}
				}
			}
		}

		// Extract CSP-specific patterns
		config.CSPs = make(map[string]model.CSPIgnorePatterns)
		if cspsRaw := ignoreViper.Get("csps"); cspsRaw != nil {
			if cspsMap, ok := cspsRaw.(map[string]any); ok {
				for cspName, cspDataRaw := range cspsMap {
					if cspData, ok := cspDataRaw.(map[string]any); ok {
						var cspConfig model.CSPIgnorePatterns

						// Extract description
						if desc, exists := cspData["description"]; exists {
							if descStr, ok := desc.(string); ok {
								cspConfig.Description = descStr
							}
						}

						// Extract global_patterns with proper type handling
						if globalPatternsRaw, exists := cspData["global_patterns"]; exists && globalPatternsRaw != nil {
							if patterns, ok := globalPatternsRaw.([]any); ok {
								for _, pattern := range patterns {
									if str, ok := pattern.(string); ok {
										cspConfig.GlobalPatterns = append(cspConfig.GlobalPatterns, str)
									}
								}
							}
						}

						// Extract regions
						if regionsRaw, exists := cspData["regions"]; exists && regionsRaw != nil {
							if regionsMap, ok := regionsRaw.(map[string]any); ok {
								cspConfig.Regions = make(map[string]model.RegionIgnorePatterns)
								for regionName, regionDataRaw := range regionsMap {
									var regionConfig model.RegionIgnorePatterns

									// New format: direct array under region name
									if regionPatterns, ok := regionDataRaw.([]any); ok {
										for _, pattern := range regionPatterns {
											if str, ok := pattern.(string); ok {
												regionConfig.Patterns = append(regionConfig.Patterns, str)
											}
										}
									}

									cspConfig.Regions[regionName] = regionConfig
								}
							}
						}

						config.CSPs[cspName] = cspConfig
					}
				}
			}
		}

		ignoreConfig = &config
		log.Info().Msg("Successfully loaded cloudspec_ignore.yaml")

		// Debug: Print loaded config structure
		log.Debug().
			Int("globalPatterns", len(config.Global.Patterns)).
			Interface("globalPatterns", config.Global.Patterns).
			Msg("Loaded global patterns")

	})

	return ignoreConfig, ignoreConfigErr
}

// shouldIgnoreSpec checks if a spec should be ignored based on the ignore configuration
func shouldIgnoreSpec(specName, providerName, regionName string) bool {
	config, err := loadCloudSpecIgnoreConfig()
	if err != nil || config == nil {
		// If config can't be loaded, don't ignore any specs
		return false
	}

	// Check global patterns first
	for _, pattern := range config.Global.Patterns {
		if matchesPattern(specName, pattern) {
			return true
		}
	}

	// Get CSP-specific patterns from the CSPs map
	// Use ResolveCloudPlatform to handle derived CSPs (e.g., openstack-new01 → openstack)
	cspPatterns, exists := config.CSPs[csp.ResolveCloudPlatform(providerName)]
	if !exists {
		return false
	}

	// Check CSP global patterns
	for _, pattern := range cspPatterns.GlobalPatterns {
		if matchesPattern(specName, pattern) {
			return true
		}
	}

	// Check region-specific patterns
	if regionPatterns, regionExists := cspPatterns.Regions[regionName]; regionExists {
		// Check direct patterns array
		for _, pattern := range regionPatterns.Patterns {
			if matchesPattern(specName, pattern) {
				return true
			}
		}
	}

	return false
}

// matchesPattern checks if a spec name matches a given pattern
// Supports basic wildcard matching with * and ?
func matchesPattern(specName, pattern string) bool {
	// Simple wildcard matching implementation
	// * matches any sequence of characters
	// ? matches any single character

	matched, err := filepath.Match(pattern, specName)
	if err != nil {
		log.Debug().Err(err).
			Str("spec", specName).
			Str("pattern", pattern).
			Msg("Error matching pattern, assuming no match")
		return false
	}

	return matched
}

// Error codes for GetAvailableZonesForSpec
