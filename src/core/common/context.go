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

// Package common provides request-scoped context helpers for cross-cutting concerns.
// These helpers allow injecting and extracting metadata (credential holder, request ID, etc.)
// via context.Context, avoiding parameter drilling across function call chains.
package common

import (
	"context"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
)

// WithCredentialHolder returns a new context with the given credential holder value.
func WithCredentialHolder(ctx context.Context, holder string) context.Context {
	return context.WithValue(ctx, model.CtxKeyCredentialHolder, holder)
}

// CredentialHolderFromContext extracts the credential holder from the context.
// Returns model.DefaultCredentialHolder if not set or empty.
func CredentialHolderFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(model.CtxKeyCredentialHolder).(string); ok && v != "" {
		return v
	}
	return model.DefaultCredentialHolder
}

// WithRequestID returns a new context with the given request ID value.
func WithRequestID(ctx context.Context, reqID string) context.Context {
	return context.WithValue(ctx, model.CtxKeyRequestID, reqID)
}

// RequestIDFromContext extracts the request ID from the context.
// Returns empty string if not set.
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(model.CtxKeyRequestID).(string); ok {
		return v
	}
	return ""
}

// NewDefaultContext creates a context with default credential holder.
// Use this for internal/system calls that don't originate from HTTP requests.
func NewDefaultContext() context.Context {
	return WithCredentialHolder(context.Background(), model.DefaultCredentialHolder)
}
