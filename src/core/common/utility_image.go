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

// Package common is to include common methods for managing multi-cloud infra
package common

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
)

var (
	reNumericParts  = regexp.MustCompile(`\d+`)
	reWinShortToken = regexp.MustCompile(`(?:^|[^a-z])win(?:$|[^a-z]|std|ent|dc|svr|server|\d)`)
	reUbuntuStyle   = regexp.MustCompile(`(?:^|[^0-9.])(\d{2})[.\s_-]?(04|10)(?:$|[^0-9])`)
	reWinServerYear = regexp.MustCompile(`(?:^|[^0-9])(2025|2022|2019|2016|2012|2008)(?:$|[^0-9])`)
	reWinShortYear  = regexp.MustCompile(`win[-_\s]?(25|22|19|16)(?:std|ent|dc|svr|[^0-9]|$)`)
	reWinDesktopVer = regexp.MustCompile(`(?:windows|win)[-_\s:]*(11|10)(?:$|[^0-9.])`)
	reAzureURNVer   = regexp.MustCompile(`:(\d{1,2})\.\d+\.\d+`)
	reNoiseTokens   = regexp.MustCompile(`\b(x86_64|x86_32|x86|x64|amd64|arm64|aarch64|s390x|i386|i686|gen[12]|v[123]|g[123]|tk4|cu\d+|sp\d+|\d{2}g|\d+bit|\d+-bit|20\d{6,})\b`)
	reMajorVersion  = regexp.MustCompile(`(?:^|[^0-9.])(\d{1,4})(?:\.\d+)?(?:$|[^0-9])`)
)

func ExtractOSInfo(combinedInfo string) string {
	if combinedInfo == "" {
		return ""
	}

	infoLower := strings.ToLower(combinedInfo)

	// Find the earliest matching OS pattern in infoLower (longer pattern wins ties)
	// so map iteration randomness or middleware names ("SQL Server 2019 on Windows Server 2025")
	// do not cause non-deterministic or wrong OS matches.
	var bestOS *model.OSTypeDetail
	bestPos := len(infoLower) + 1
	bestPat := ""

	for _, osInfo := range RuntimeExtractPatternsInfo.ExtractPatterns.OSType {
		osCopy := osInfo
		for _, pattern := range osCopy.Patterns {
			patLower := strings.ToLower(pattern)
			pos := -1
			if patLower == "win" {
				if loc := reWinShortToken.FindStringIndex(infoLower); loc != nil {
					pos = loc[0]
				}
			} else {
				pos = strings.Index(infoLower, patLower)
			}
			if pos != -1 {
				if pos < bestPos || (pos == bestPos && len(patLower) > len(bestPat)) {
					bestPos = pos
					bestOS = &osCopy
					bestPat = patLower
				}
			}
		}
	}

	if bestOS == nil {
		return ""
	}

	osName := bestOS.Name
	versions := bestOS.Versions
	if len(versions) == 0 {
		return osName
	}

	// Check if versions are multi-part (e.g., Ubuntu "24.04", "22.04")
	firstParts := reNumericParts.FindAllString(versions[0], -1)
	if len(firstParts) >= 2 {
		searchAreas := []string{infoLower[bestPos:], infoLower}
		for _, area := range searchAreas {
			if m := reUbuntuStyle.FindStringSubmatch(area); len(m) == 3 {
				cand := fmt.Sprintf("%s.%s", m[1], m[2])
				if slices.Contains(versions, cand) {
					return fmt.Sprintf("%s %s", osName, cand)
				}
				if m[1] == "12" || m[1] == "14" {
					return osName
				}
			}
			for _, version := range versions {
				numericParts := reNumericParts.FindAllString(version, -1)
				if len(numericParts) > 0 {
					versionPattern := strings.Join(numericParts, `[.\s-_]?`)
					bounded := fmt.Sprintf(`(?:^|[^0-9.])%s(?:$|[^0-9])`, versionPattern)
					if re, err := regexp.Compile(bounded); err == nil && re.MatchString(area) {
						return fmt.Sprintf("%s %s", osName, version)
					}
				}
			}
		}
		return osName
	}

	// Single-number major versions (Rocky, RHEL, Debian, CentOS, Alma, Oracle, SUSE, Fedora, Windows, Amazon, Alibaba, Tencent, Azure)
	tail := infoLower[bestPos:]

	if osName == "Windows" {
		if m := reWinServerYear.FindStringSubmatch(tail); len(m) == 2 {
			if slices.Contains(versions, m[1]) {
				return fmt.Sprintf("Windows %s", m[1])
			}
			return "Windows"
		}
		if m := reWinShortYear.FindStringSubmatch(tail); len(m) == 2 {
			yr := "20" + m[1]
			if slices.Contains(versions, yr) {
				return fmt.Sprintf("Windows %s", yr)
			}
			return "Windows"
		}
		if !strings.Contains(tail, "server") {
			if m := reWinDesktopVer.FindStringSubmatch(tail); len(m) == 2 {
				if slices.Contains(versions, m[1]) {
					return fmt.Sprintf("Windows %s", m[1])
				}
			}
		}
		return "Windows"
	}

	// Azure URN fourth segment (Publisher:Offer:Sku:Major.Minor.Date)
	if m := reAzureURNVer.FindStringSubmatch(tail); len(m) == 2 {
		if slices.Contains(versions, m[1]) {
			return fmt.Sprintf("%s %s", osName, m[1])
		}
		return osName
	}

	// Find the first major version token after the OS pattern, ignoring architecture/bitness/generation tokens
	// and never matching a minor version preceded by "<digit>." (such as .8 in 9.8).
	cleanedTail := reNoiseTokens.ReplaceAllString(tail, " ")
	for _, m := range reMajorVersion.FindAllStringSubmatch(cleanedTail, -1) {
		if len(m) == 2 && slices.Contains(versions, m[1]) {
			return fmt.Sprintf("%s %s", osName, m[1])
		}
	}

	return osName
}

// CheckBasicOSImage checks if the given combined info matches any basic OS image patterns
// Returns true if the image is considered a basic OS image based on predefined patterns
// providerName is used to apply CSP-specific rules (if defined) or common rules (as fallback)
//
// Logic:
// 1. Determine include patterns:
//   - If CSP-specific include exists and not empty → use CSP-specific include
//   - Otherwise → use common include
//
// 2. Determine exclude patterns:
//   - If CSP-specific exclude exists and not empty → use CSP-specific exclude
//   - Otherwise → use common exclude
//
// 3. Check exclude patterns first (return false if matched)
// 4. Check include patterns (return true if matched)
func CheckBasicOSImage(combinedInfo string, providerName string) bool {
	if combinedInfo == "" {
		return false
	}

	// Normalize provider name to lowercase for consistent matching
	providerNameLower := strings.ToLower(providerName)

	// Loop through all OS types from extraction patterns
	for _, osInfo := range RuntimeExtractPatternsInfo.ExtractPatterns.OSType {
		// Skip if no basic image rules defined
		if osInfo.BasicImageRules == nil {
			continue
		}

		// Start with common patterns as default
		includePatterns := osInfo.BasicImageRules.Common.Include
		excludePatterns := osInfo.BasicImageRules.Common.Exclude

		// Override with CSP-specific patterns if they exist and are not empty
		if osInfo.BasicImageRules.CspSpecific != nil {
			if cspPatterns, exists := osInfo.BasicImageRules.CspSpecific[providerNameLower]; exists {
				// Use CSP-specific include if defined and not empty
				if len(cspPatterns.Include) > 0 {
					includePatterns = cspPatterns.Include
				}
				// Use CSP-specific exclude if defined and not empty
				if len(cspPatterns.Exclude) > 0 {
					excludePatterns = cspPatterns.Exclude
				}
			}
		}

		// Step 1: Check exclude patterns first (higher priority)
		for _, excludePattern := range excludePatterns {
			if matchesPattern(combinedInfo, excludePattern) {
				// This image matches an exclude pattern, so it's not a basic image
				return false
			}
		}

		// Step 2: Check include patterns
		for _, includePattern := range includePatterns {
			if matchesPattern(combinedInfo, includePattern) {
				// Pattern matched and not excluded
				return true
			}
		}
	}

	return false
}

// matchesPattern checks if a string matches a pattern with wildcard support
// The pattern supports '*' as wildcard that matches any sequence of characters
func matchesPattern(text, pattern string) bool {
	if pattern == "" {
		return text == ""
	}

	// Split pattern by '*' to get literal parts
	parts := strings.Split(pattern, "*")

	// If no wildcards, do exact match (case-insensitive)
	if len(parts) == 1 {
		return strings.EqualFold(text, pattern)
	}

	textLower := strings.ToLower(text)

	// Check if text starts with first part
	if len(parts[0]) > 0 {
		if !strings.HasPrefix(textLower, strings.ToLower(parts[0])) {
			return false
		}
		textLower = textLower[len(parts[0]):]
	}

	// Check if text ends with last part
	if len(parts[len(parts)-1]) > 0 {
		lastPart := strings.ToLower(parts[len(parts)-1])
		if !strings.HasSuffix(textLower, lastPart) {
			return false
		}
		textLower = textLower[:len(textLower)-len(lastPart)]
	}

	// Check middle parts in order
	for i := 1; i < len(parts)-1; i++ {
		part := strings.ToLower(parts[i])
		if part == "" {
			continue // empty part between consecutive wildcards
		}

		index := strings.Index(textLower, part)
		if index == -1 {
			return false
		}

		// Move past this part
		textLower = textLower[index+len(part):]
	}

	return true
}

// IsGPUImage checks if an image has GPU support
func IsGPUImage(combinedInfo string) bool {
	if combinedInfo == "" {
		return false
	}

	infoLower := strings.ToLower(combinedInfo)

	// Check exclude patterns first — any match overrides positive GPU patterns
	for _, exc := range RuntimeExtractPatternsInfo.ExtractPatterns.GPUExcludePatterns {
		if strings.Contains(infoLower, strings.ToLower(exc)) {
			return false
		}
	}

	for _, pattern := range RuntimeExtractPatternsInfo.ExtractPatterns.GPUPatterns {
		if strings.Contains(infoLower, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}

// CheckBasicGpuImage checks if the given combined info matches any basic GPU image patterns.
// Uses CSP-specific include patterns with a common exclude pre-filter.
func CheckBasicGpuImage(combinedInfo string, providerName string) bool {
	rules := RuntimeExtractPatternsInfo.ExtractPatterns.BasicGpuImageRules
	if rules == nil {
		return false
	}

	providerNameLower := strings.ToLower(providerName)

	// Apply common exclude patterns first
	for _, exc := range rules.Common.Exclude {
		if matchesPattern(combinedInfo, exc) {
			return false
		}
	}

	// Determine include patterns: CSP-specific overrides common
	includePatterns := rules.Common.Include
	if rules.CspSpecific != nil {
		if cspPatterns, exists := rules.CspSpecific[providerNameLower]; exists && len(cspPatterns.Include) > 0 {
			includePatterns = cspPatterns.Include
		}
	}

	for _, pattern := range includePatterns {
		if matchesPattern(combinedInfo, pattern) {
			return true
		}
	}

	return false
}

// hasWordBoundedToken checks whether token appears in text bounded by non-alphanumeric characters.
func hasWordBoundedToken(textLower, tokenLower string) bool {
	start := 0
	for {
		idx := strings.Index(textLower[start:], tokenLower)
		if idx == -1 {
			return false
		}
		pos := start + idx
		end := pos + len(tokenLower)
		leftOK := pos == 0 || !isAlphaNumASCII(textLower[pos-1])
		rightOK := end == len(textLower) || !isAlphaNumASCII(textLower[end])
		if leftOK && rightOK {
			return true
		}
		start = pos + 1
	}
}

func isAlphaNumASCII(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}

// IsK8sImage checks if an image is for Kubernetes
func IsK8sImage(combinedInfo string) bool {
	if combinedInfo == "" {
		return false
	}

	infoLower := strings.ToLower(combinedInfo)

	// Handle NHN Cloud explicit NKS compatibility metadata in Properties:
	// Every NHN image includes "nhncloud_allow_nks_cpu_flavor:false,nhncloud_allow_nks_gpu_flavor:false"
	// (or ":true" for Container/NKS images). Respect these flags directly so "nks" in the key name
	// does not cause false positives on standard NHN VM images.
	if strings.Contains(infoLower, "nhncloud_allow_nks_cpu_flavor:true") ||
		strings.Contains(infoLower, "nhncloud_allow_nks_gpu_flavor:true") {
		return true
	}
	if strings.Contains(infoLower, "nhncloud_allow_nks_cpu_flavor:false") &&
		strings.Contains(infoLower, "nhncloud_allow_nks_gpu_flavor:false") {
		return false
	}

	for _, pattern := range RuntimeExtractPatternsInfo.ExtractPatterns.K8sPatterns {
		patLower := strings.ToLower(strings.TrimSpace(pattern))
		if patLower == "" {
			continue
		}
		// Short acronym patterns (eks, aks, gke, ack, tke, nks, iks, k8s) must match on
		// word boundaries so "ack" never matches "default_backend" (KT), "backports" (Azure), or "backup" (OpenStack).
		if len(patLower) <= 4 {
			if hasWordBoundedToken(infoLower, patLower) {
				return true
			}
		} else if strings.Contains(infoLower, patLower) {
			return true
		}
	}

	return false
}

// IsDeprecatedImage checks if an image is deprecated
func IsDeprecatedImage(combinedInfo string) bool {
	if combinedInfo == "" {
		return false
	}

	infoLower := strings.ToLower(combinedInfo)

	// Check for deprecated patterns
	patterns := []string{
		"deprecated",
		"end of life",
		"eol",
	}

	for _, pattern := range patterns {
		if strings.Contains(infoLower, pattern) {
			return true
		}
	}

	return false
}

// ConvertToBaseCurrency converts a cost value from a specific currency to the base currency (USD)
// This function handles currency conversion using predefined exchange rates
