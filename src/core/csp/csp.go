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

// Package csp provides direct CSP (Cloud Service Provider) API call utilities.
// This bypasses CB-Spider for cases where direct SDK calls are more efficient
// or where CB-Spider does not provide the needed functionality.
package csp

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/openbao/openbao/api/v2"
	"github.com/rs/zerolog/log"
)

// BatchVMStatusFunc queries a CSP directly for the statuses of the given resource IDs.
// ctx must carry model.CtxKeyCredentialHolder for credential lookup.
// region is the CSP-specific region identifier (e.g., "ap-northeast-2" for AWS).
// instanceIds are the CspResourceId values for each VM — format varies per CSP:
//
//	AWS / Alibaba: "i-0abc123def456"
//	Tencent:       "ins-xxxxxxxx"
//	Azure:         full ARM path "/subscriptions/{sub}/resourceGroups/{rg}/.../virtualMachines/{name}"
//	GCP:           instance name (zone-scoped; region used for zone-prefix filtering)
//
// Returns a map of CspResourceId → TB status string (model.StatusRunning, etc.).
// Missing keys mean the instance was not found; treat as model.StatusUndefined.
type BatchVMStatusFunc func(ctx context.Context, region string, instanceIds []string) (map[string]string, error)

var (
	batchVMStatusMu       sync.RWMutex
	batchVMStatusHandlers = make(map[string]BatchVMStatusFunc)
)

// RegisterBatchVMStatusHandler registers a direct-SDK batch VM status function for a CSP.
// Each CSP package calls this from its init() function.
func RegisterBatchVMStatusHandler(provider string, fn BatchVMStatusFunc) {
	batchVMStatusMu.Lock()
	defer batchVMStatusMu.Unlock()
	batchVMStatusHandlers[strings.ToLower(provider)] = fn
}

// GetBatchVMStatusHandler returns the registered BatchVMStatusFunc for the given provider.
func GetBatchVMStatusHandler(provider string) (BatchVMStatusFunc, bool) {
	batchVMStatusMu.RLock()
	defer batchVMStatusMu.RUnlock()
	fn, ok := batchVMStatusHandlers[strings.ToLower(provider)]
	return fn, ok
}

// BatchVMControlFunc sends a lifecycle control action to multiple instances in one SDK call.
// ctx must carry model.CtxKeyCredentialHolder for credential lookup.
// region is the CSP-native region identifier (e.g., "ap-northeast-2" for AWS).
// instanceIds are the CspResourceId values for each VM.
//
// Returns a map of CspResourceId → transient TB status string (e.g., model.StatusSuspending).
// Missing keys mean the instance was not found or accepted; callers treat them as failed.
type BatchVMControlFunc func(ctx context.Context, region string, instanceIds []string) (map[string]string, error)

// BatchVMControlHandlers groups bulk lifecycle control functions for a CSP.
// Reboot is excluded — it is rare, order-sensitive, and not cost-effective to batch.
type BatchVMControlHandlers struct {
	Suspend   BatchVMControlFunc // e.g. AWS StopInstances
	Resume    BatchVMControlFunc // e.g. AWS StartInstances
	Terminate BatchVMControlFunc // e.g. AWS TerminateInstances
}

var (
	batchVMControlMu       sync.RWMutex
	batchVMControlHandlers = make(map[string]BatchVMControlHandlers)
)

// RegisterBatchVMControlHandlers registers bulk lifecycle control functions for a CSP.
// Each CSP package calls this from its init() function.
func RegisterBatchVMControlHandlers(provider string, h BatchVMControlHandlers) {
	batchVMControlMu.Lock()
	defer batchVMControlMu.Unlock()
	batchVMControlHandlers[strings.ToLower(provider)] = h
}

// GetBatchVMControlHandler returns the bulk control function for the given provider and action.
// action is case-insensitive: "suspend", "resume", or "terminate".
func GetBatchVMControlHandler(provider, action string) (BatchVMControlFunc, bool) {
	batchVMControlMu.RLock()
	defer batchVMControlMu.RUnlock()
	h, ok := batchVMControlHandlers[strings.ToLower(provider)]
	if !ok {
		return nil, false
	}
	switch strings.ToLower(action) {
	case "suspend":
		return h.Suspend, h.Suspend != nil
	case "resume":
		return h.Resume, h.Resume != nil
	case "terminate":
		return h.Terminate, h.Terminate != nil
	default:
		return nil, false
	}
}

// ReadOpenBaoSecret reads a secret from OpenBao at the given path and returns the data map.
// It validates that VaultToken is set and the secret exists.
// A context is used for request-scoped cancellation and timeout.
func ReadOpenBaoSecret(ctx context.Context, path string) (map[string]interface{}, error) {
	if model.VaultToken == "" {
		return nil, fmt.Errorf("VAULT_TOKEN is not set")
	}

	vaultConfig := api.DefaultConfig()
	vaultConfig.Address = model.VaultAddr
	client, err := api.NewClient(vaultConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenBao client: %w", err)
	}
	client.SetToken(model.VaultToken)

	secret, err := client.Logical().ReadWithContext(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to read secret from OpenBao at %s: %w", path, err)
	}
	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("secret not found at %s", path)
	}

	data, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid secret format at %s: 'data' field missing or not a map", path)
	}

	return data, nil
}

// BuildSecretPath builds the OpenBao secret path for a given CSP provider
// based on the credential holder from context.
// For "admin" holder: "secret/data/csp/{provider}"
// For other holders:  "secret/data/users/{holder}/csp/{provider}"
func BuildSecretPath(ctx context.Context, provider string) string {
	// Inline credential holder extraction to avoid importing common (prevents import cycle)
	holder := model.DefaultCredentialHolder
	if v, ok := ctx.Value(model.CtxKeyCredentialHolder).(string); ok && v != "" {
		holder = v
	}

	if holder == "admin" {
		return fmt.Sprintf("secret/data/csp/%s", provider)
	}
	return fmt.Sprintf("secret/data/users/%s/csp/%s", holder, provider)
}

// GetString safely extracts a string value from a map.
func GetString(data map[string]interface{}, key string) string {
	v, _ := data[key].(string)
	return v
}

// LogCSP logs a CSP-related message with consistent prefix.
func LogCSP(provider, msg string) {
	log.Debug().Str("provider", provider).Msg("[CSP] " + msg)
}
