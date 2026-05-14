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
package apierr

import (
	"errors"
	"net/http"
	"strings"
)

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

// Wrap attaches message to err, preserving StatusCode and Cause if err is already
// a *StatusError (the common case after HandleHttpResponse).
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

// IsNotFound reports whether err represents a not-found condition.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	if containsAny(err.Error(), notFoundPatterns) {
		return true
	}
	var se *StatusError
	if errors.As(err, &se) && se.StatusCode == http.StatusNotFound {
		return true
	}
	return false
}

// IsConflict reports whether err represents a conflict / already-exists condition.
func IsConflict(err error) bool {
	if err == nil {
		return false
	}
	if containsAny(err.Error(), conflictPatterns) {
		return true
	}
	var se *StatusError
	if errors.As(err, &se) && se.StatusCode == http.StatusConflict {
		return true
	}
	return false
}

var notFoundPatterns = []string{
	"not found",
	"does not exist",
	"cannot get",
	"no such",
	"resource not found",
}

var conflictPatterns = []string{
	"already exists",
	"conflict",
	"duplicate",
	"bucket name already",
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
