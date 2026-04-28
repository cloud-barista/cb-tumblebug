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

// Package errutil classifies errors from downstream services (Spider, CSP, KV store)
// and maps them to HTTP status codes for Tumblebug API responses.
//
// Usage:
//
//	In resource layer: log downstream origin, return a clean domain message
//	if err = clientManager.HandleHttpResponse(resp, err); err != nil {
//	    log.Error().Err(err).Msg("")                           // preserves raw text
//	    return fmt.Errorf("failed to create vNet '%s'", id)   // clean domain message
//	}
//
//	In REST handler: map error to HTTP status code
//	return c.JSON(errutil.ApiStatus(err), model.SimpleMsg{Message: err.Error()})
//	→ 404 "not found / does not exist", 409 "already exists / conflict", 500 otherwise
//
// HttpError is defined here and wrapped by client.HandleHttpResponse to carry
// the HTTP status code as a fallback when the error message does not match
// a known pattern.
package errutil

import (
	"errors"
	"net/http"
	"strings"
)

// HttpError carries the HTTP status code from a downstream response so callers
// can inspect it via errors.As without relying solely on string matching.
type HttpError struct {
	StatusCode int
	Err        error
}

func (e *HttpError) Error() string { return e.Err.Error() }
func (e *HttpError) Unwrap() error { return e.Err }

// notFoundPatterns are lower-cased substrings that indicate a resource was not
// found.  Covers:
//   - Tumblebug KV-store misses  ("does not exist", "not found")
//   - Spider / CSP passthrough   ("cannot get", "no such", "resource not found")
var notFoundPatterns = []string{
	"not found",
	"does not exist",
	"cannot get",
	"no such",
	"resource not found",
}

// conflictPatterns are lower-cased substrings that indicate a resource already
// exists or a naming collision occurred.  Covers:
//   - Tumblebug duplicate checks ("already exists")
//   - Spider / CSP responses     ("conflict", "duplicate", "bucket name already")
var conflictPatterns = []string{
	"already exists",
	"conflict",
	"duplicate",
	"bucket name already",
}

// containsAny returns true if s (lower-cased) contains any of the patterns.
func containsAny(s string, patterns []string) bool {
	lower := strings.ToLower(s)
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// IsNotFoundError reports whether err represents a "resource not found"
// condition.
//
// Classification order:
//  1. Error message matches notFoundPatterns (primary signal).
//  2. Error wraps *client.HttpError with StatusCode 404 (fallback).
func IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	if containsAny(err.Error(), notFoundPatterns) {
		return true
	}
	// Fallback: HTTP status code wrapped by client.HandleHttpResponse.
	var httpErr *HttpError
	if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound {
		return true
	}
	return false
}

// IsConflictError reports whether err represents a conflict / already-exists
// condition.
//
// Classification order:
//  1. Error message matches conflictPatterns (primary signal).
//  2. Error wraps *client.HttpError with StatusCode 409 (fallback).
func IsConflictError(err error) bool {
	if err == nil {
		return false
	}
	if containsAny(err.Error(), conflictPatterns) {
		return true
	}
	// Fallback: HTTP status code wrapped by client.HandleHttpResponse.
	var httpErr *HttpError
	if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusConflict {
		return true
	}
	return false
}

// ApiStatus maps err to an HTTP status code for Tumblebug API responses.
//
// Only semantically meaningful domain errors (404, 409) are surfaced to API
// callers; everything else becomes 500.  This ensures that Spider's internal
// codes and raw CSP messages are never exposed directly to the caller — the
// mapping is driven solely by the pattern lists above.
//
// Used exclusively in REST handlers (interface/rest/server/resource/).
func ApiStatus(err error) int {
	switch {
	case IsNotFoundError(err):
		return http.StatusNotFound // 404
	case IsConflictError(err):
		return http.StatusConflict // 409
	default:
		return http.StatusInternalServerError // 500
	}
}
