package csp

import "strings"

// Supported Cloud Service Providers
const (
	Alibaba   = "alibaba"
	AWS       = "aws"
	Azure     = "azure"
	GCP       = "gcp"
	IBM       = "ibm"
	Tencent   = "tencent"
	NCP       = "ncp"
	NHN       = "nhn"
	KT        = "kt"
	OpenStack = "openstack"
)

// AllCSPs is the list of all supported Cloud Service Providers
var AllCSPs = []string{
	AWS, Azure, GCP, Alibaba, Tencent, IBM, OpenStack, NCP, NHN, KT,
}

// RateLimitConfig holds per-CSP rate limiting configuration.
// All concurrency/rate values are centralized here to ensure consistency
// across VM creation, resource registration, status fetching, and other parallel operations.
// When adding a new CSP, add an entry here to configure its behavior.
type RateLimitConfig struct {
	// Resource registration: max connections processed in parallel per CSP
	MaxConcurrentRegistrations int
	// Resource registration: stagger delay range (ms) to avoid API burst
	RegistrationDelayMinMs int
	RegistrationDelayMaxMs int

	// VM creation: max regions processed in parallel per CSP
	MaxConcurrentRegions int
	// VM creation: max VMs created in parallel per region
	MaxVMsPerRegion int

	// VM status fetching: max regions queried in parallel per CSP
	MaxConcurrentRegionsForStatus int
	// VM status fetching: max VMs queried in parallel per region
	MaxVMsPerRegionForStatus int
}

// Default rate limit values for CSPs not explicitly configured.
// These provide a conservative fallback for unknown/new CSPs.
var defaultRateLimitConfig = RateLimitConfig{
	MaxConcurrentRegistrations:    3,
	RegistrationDelayMinMs:        1000,
	RegistrationDelayMaxMs:        3000,
	MaxConcurrentRegions:          30,
	MaxVMsPerRegion:               20,
	MaxConcurrentRegionsForStatus: 10,
	MaxVMsPerRegionForStatus:      30,
}

// GlobalMaxConcurrentConnections caps the total number of goroutines
// processing connections across all CSPs simultaneously.
const GlobalMaxConcurrentConnections = 10

// rateLimitConfigs holds per-CSP rate limiting configurations.
// Each CSP's API rate limits and concurrency characteristics are captured here.
//
// To add a new CSP:
//  1. Add the CSP name constant above
//  2. Add an entry to AllCSPs
//  3. Add a RateLimitConfig entry here with appropriate limits
//
// Notes on specific CSPs:
//   - Tencent: strict 10 req/sec API limit, keep concurrency low
//   - NCP: stricter API limits, reduced VM creation parallelism
//   - KT: limited infrastructure, conservative settings
var rateLimitConfigs = map[string]RateLimitConfig{
	AWS: {
		MaxConcurrentRegistrations:    5,
		RegistrationDelayMinMs:        500,
		RegistrationDelayMaxMs:        2000,
		MaxConcurrentRegions:          30,
		MaxVMsPerRegion:               20,
		MaxConcurrentRegionsForStatus: 10,
		MaxVMsPerRegionForStatus:      30,
	},
	Azure: {
		MaxConcurrentRegistrations:    4,
		RegistrationDelayMinMs:        500,
		RegistrationDelayMaxMs:        2000,
		MaxConcurrentRegions:          30,
		MaxVMsPerRegion:               20,
		MaxConcurrentRegionsForStatus: 8,
		MaxVMsPerRegionForStatus:      25,
	},
	GCP: {
		MaxConcurrentRegistrations:    4,
		RegistrationDelayMinMs:        1000,
		RegistrationDelayMaxMs:        3000,
		MaxConcurrentRegions:          30,
		MaxVMsPerRegion:               20,
		MaxConcurrentRegionsForStatus: 12,
		MaxVMsPerRegionForStatus:      35,
	},
	Alibaba: {
		MaxConcurrentRegistrations:    3,
		RegistrationDelayMinMs:        1000,
		RegistrationDelayMaxMs:        3000,
		MaxConcurrentRegions:          30,
		MaxVMsPerRegion:               20,
		MaxConcurrentRegionsForStatus: 6,
		MaxVMsPerRegionForStatus:      20,
	},
	Tencent: {
		MaxConcurrentRegistrations:    2,
		RegistrationDelayMinMs:        2000,
		RegistrationDelayMaxMs:        5000, // strict 10 req/sec limit
		MaxConcurrentRegions:          30,
		MaxVMsPerRegion:               20,
		MaxConcurrentRegionsForStatus: 6,
		MaxVMsPerRegionForStatus:      20,
	},
	IBM: {
		MaxConcurrentRegistrations:    3,
		RegistrationDelayMinMs:        500,
		RegistrationDelayMaxMs:        2000,
		MaxConcurrentRegions:          30,
		MaxVMsPerRegion:               20,
		MaxConcurrentRegionsForStatus: 10,
		MaxVMsPerRegionForStatus:      30,
	},
	NCP: {
		MaxConcurrentRegistrations:    2,
		RegistrationDelayMinMs:        1000,
		RegistrationDelayMaxMs:        3000,
		MaxConcurrentRegions:          5,
		MaxVMsPerRegion:               15, // NCP has stricter limits
		MaxConcurrentRegionsForStatus: 3,
		MaxVMsPerRegionForStatus:      15,
	},
	NHN: {
		MaxConcurrentRegistrations:    2,
		RegistrationDelayMinMs:        1000,
		RegistrationDelayMaxMs:        3000,
		MaxConcurrentRegions:          30,
		MaxVMsPerRegion:               20,
		MaxConcurrentRegionsForStatus: 5,
		MaxVMsPerRegionForStatus:      20,
	},
	KT: {
		MaxConcurrentRegistrations:    2,
		RegistrationDelayMinMs:        1000,
		RegistrationDelayMaxMs:        3000,
		MaxConcurrentRegions:          30,
		MaxVMsPerRegion:               20,
		MaxConcurrentRegionsForStatus: 10,
		MaxVMsPerRegionForStatus:      30,
	},
	OpenStack: {
		MaxConcurrentRegistrations:    3,
		RegistrationDelayMinMs:        500,
		RegistrationDelayMaxMs:        2000,
		MaxConcurrentRegions:          30,
		MaxVMsPerRegion:               20,
		MaxConcurrentRegionsForStatus: 5,
		MaxVMsPerRegionForStatus:      15,
	},
}

// GetRateLimitConfig returns the rate limiting configuration for a given CSP.
// Returns the default configuration if the CSP is not explicitly configured.
func GetRateLimitConfig(providerName string) RateLimitConfig {
	normalized := strings.ToLower(providerName)
	if config, exists := rateLimitConfigs[normalized]; exists {
		return config
	}
	return defaultRateLimitConfig
}
