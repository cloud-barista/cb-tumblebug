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

// Package apierr provides error classification and HTTP status mapping for
// Tumblebug API responses, inspired by pkg.go.dev/k8s.io/apimachinery/pkg/api/errors.
//
// Use Wrap in the resource layer to attach a message to downstream errors,
// and Code in REST handlers to derive the HTTP status code.
//
// IsNotFound and IsConflict classify errors in priority order:
// (1) Spider IID pattern — most reliable, set before any CSP call (Spider PR #1623).
// (2) Message patterns — CSP text Spider proxies as HTTP 500.
// (3) HTTP status — lowest priority; Spider's status is inconsistent (VPC always 500).
package apierr

import (
	"errors"
	"net/http"
	"strings"
)

// Spider's most reliable message patterns from Spider.
// https://github.com/cloud-barista/cb-spider/pull/1623
const (
	spiderNotFoundPattern = "does not exist in connection"
	spiderConflictPattern = "already exists in connection"
)

// normalNotFoundPatterns matches CSP-originated "not found" error text.
// Spider passes CSP errors through as HTTP 500, so message matching is required.
var normalNotFoundPatterns = []string{
	"not found",
	"does not exist",
	"no such",
	"resource not found",
}

// normalConflictPatterns matches CSP-originated "already exists" error text.
var normalConflictPatterns = []string{
	"already exists",
	"conflict",
	"duplicate",
	"bucket name already",
}

// StatusError is the error type used throughout the Tumblebug pipeline.
// HandleHttpResponse sets StatusCode and Cause; Wrap sets Message.
// Error() returns "message: cause", or whichever field is non-empty.
type StatusError struct {
	StatusCode int
	Message    string // Tumblebug-level message; empty until set by Wrap.
	Cause      error  // raw Spider / Terrarium / network error.
}

func (e *StatusError) Error() string {
	if e.Message != "" && e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return "unknown error"
}

// Unwrap implements the errors unwrap interface.
func (e *StatusError) Unwrap() error { return e.Cause }

// Wrap attaches message to err, preserving StatusCode and Cause.
func Wrap(err error, message string) error {
	if err == nil {
		return errors.New(message)
	}
	var se *StatusError
	if errors.As(err, &se) {
		cloned := *se
		cloned.Message = message
		return &cloned
	}
	// Not a *StatusError — wrap it.
	return &StatusError{Message: message, Cause: err}
}

// Code maps err to an HTTP status code (404, 409, or 500).
func Code(err error) int {
	switch {
	case IsNotFound(err):
		return http.StatusNotFound
	case IsConflict(err):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

// spiderMsg returns the raw Spider/CSP error text from StatusError.Cause,
// bypassing any TB-level Wrap message.
func spiderMsg(err error) string {
	var se *StatusError
	if errors.As(err, &se) && se.Cause != nil {
		return se.Cause.Error()
	}
	return err.Error()
}

// IsNotFound reports whether err represents a not-found condition.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := spiderMsg(err)

	if strings.Contains(strings.ToLower(msg), spiderNotFoundPattern) { // Not found in/via Spider
		return true
	}
	if containsAny(msg, normalNotFoundPatterns) { // CSP message patterns
		return true
	}
	var se *StatusError // HTTP 404 fallback (e.g. Terrarium)
	return errors.As(err, &se) && se.StatusCode == http.StatusNotFound
}

// IsConflict reports whether err represents a conflict / already-exists condition.
func IsConflict(err error) bool {
	if err == nil {
		return false
	}
	msg := spiderMsg(err)

	if strings.Contains(strings.ToLower(msg), spiderConflictPattern) { // Conflict in/via Spider
		return true
	}
	if containsAny(msg, normalConflictPatterns) { // CSP message patterns
		return true
	}
	var se *StatusError // HTTP 409 fallback
	return errors.As(err, &se) && se.StatusCode == http.StatusConflict
}

func containsAny(s string, patterns []string) bool {
	lower := strings.ToLower(s)
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}
