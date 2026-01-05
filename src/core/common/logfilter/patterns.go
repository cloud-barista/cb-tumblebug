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

// Package logfilter provides centralized log filtering configuration with minimal dependencies.
// It defines shared skip patterns and related types for log filtering across the project.
//
// Usage:
//   - RequestSkipPatterns: Echo middleware request logging
//   - APISkipPatterns: API call logging/tracking
//   - InternalCallSkipPatterns: Internal HTTP client calls (Spider, Terrarium, etc.)
//
// Pattern Format:
//   - Method: HTTP method to match ("" = any method, "GET", "POST", etc.)
//   - Patterns: URL patterns that must ALL match (AND condition)
//   - Single pattern: {"/path"} - matches if URL contains "/path"
//   - Multiple patterns: {"/path", "param=value"} - matches if URL contains BOTH
package logfilter

// SkipRule defines a log skip rule with optional method filtering
type SkipRule struct {
	Method   string   // HTTP method ("" = any, "GET", "POST", "PUT", "DELETE", etc.)
	Patterns []string // URL patterns - ALL must match (AND condition)
}

// ==============================================================================
// REQUEST LOG SKIP PATTERNS
// Used by: Echo middleware (zerologger, request tracker)
// Purpose: Skip logging for high-frequency or utility endpoints
// ==============================================================================

var RequestSkipPatterns = []SkipRule{
	// Swagger/API documentation
	{Patterns: []string{"/tumblebug/api"}},

	// Health check and utility endpoints
	{Patterns: []string{"/tumblebug/readyz"}},
	{Patterns: []string{"/tumblebug/httpVersion"}},
	{Patterns: []string{"/tumblebug/testStreamResponse"}},

	// Request tracking endpoints (avoid recursive logging)
	{Patterns: []string{"/tumblebug/request"}},
	{Patterns: []string{"/tumblebug/requests"}},
}

// ==============================================================================
// API LOG SKIP PATTERNS
// Used by: API call logging middleware
// Purpose: Skip verbose logging for frequently polled or list endpoints
// ==============================================================================

var APISkipPatterns = []SkipRule{
	// Swagger/API documentation
	{Patterns: []string{"/tumblebug/api"}},

	// Health check endpoints
	{Patterns: []string{"/tumblebug/readyz"}},
	{Patterns: []string{"/tumblebug/httpVersion"}},

	// MCI status polling (very frequent) - GET only
	{Method: "GET", Patterns: []string{"/mci", "option=status"}},
	{Method: "GET", Patterns: []string{"/mci"}},

	// Kubernetes cluster operations - GET only
	{Method: "GET", Patterns: []string{"/k8sCluster"}},

	// Resource list operations (frequent polling from UI) - GET only
	{Method: "GET", Patterns: []string{"/resources/vNet"}},
	{Method: "GET", Patterns: []string{"/resources/securityGroup"}},
	{Method: "GET", Patterns: []string{"/resources/vpn"}},
	{Method: "GET", Patterns: []string{"/resources/sshKey"}},
	{Method: "GET", Patterns: []string{"/resources/customImage"}},
	{Method: "GET", Patterns: []string{"/resources/dataDisk"}},
}

// ==============================================================================
// INTERNAL CALL LOG SKIP PATTERNS
// Used by: Internal HTTP client (client.go)
// Purpose: Skip logging for high-frequency internal API calls
// ==============================================================================

var InternalCallSkipPatterns = []SkipRule{
	// Spider registration APIs (high frequency during init/sync) - any method
	{Patterns: []string{"/spider/region"}},
	{Patterns: []string{"/spider/credential"}},
	{Patterns: []string{"/spider/driver"}},
	{Patterns: []string{"/spider/connectionconfig"}},

	// Examples with method filtering:
	// {Method: "GET", Patterns: []string{"/spider/vm", "option=status"}},  // GET VM status only
	// {Method: "POST", Patterns: []string{"/spider/vm"}},                   // POST VM creation
	// {Patterns: []string{"/terrarium/"}},                                  // Any method for terrarium
}
