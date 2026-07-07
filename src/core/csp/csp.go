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
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

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
type BatchVMControlHandlers struct {
	Suspend   BatchVMControlFunc // e.g. AWS StopInstances
	Resume    BatchVMControlFunc // e.g. AWS StartInstances
	Terminate BatchVMControlFunc // e.g. AWS TerminateInstances
	Reboot    BatchVMControlFunc // e.g. Azure BeginRestart
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
// action is case-insensitive: "suspend", "resume", "terminate", or "reboot".
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
	case "reboot":
		return h.Reboot, h.Reboot != nil
	default:
		return nil, false
	}
}

// credentialKeyMap maps each CSP's YAML credential keys to the environment variable
// names expected by OpenTofu providers and cb-tumblebug's runtime credential lookup.
// Must stay in sync with init/openbao/openbao-register-creds.py KEY_MAP.
var credentialKeyMap = map[string]map[string]string{
	"aws": {
		"aws_access_key_id":     "AWS_ACCESS_KEY_ID",
		"aws_secret_access_key": "AWS_SECRET_ACCESS_KEY",
	},
	"azure": {
		"clientId":       "ARM_CLIENT_ID",
		"clientSecret":   "ARM_CLIENT_SECRET",
		"tenantId":       "ARM_TENANT_ID",
		"subscriptionId": "ARM_SUBSCRIPTION_ID",
		"S3AccessKey":    "ARM_STORAGE_ACCOUNT_NAME",
		"S3SecretKey":    "ARM_ACCESS_KEY",
	},
	"gcp": {
		"project_id":     "project_id",
		"client_email":   "client_email",
		"private_key":    "private_key",
		"private_key_id": "private_key_id",
		"client_id":      "client_id",
		"S3AccessKey":    "GCP_S3_ACCESS_KEY",
		"S3SecretKey":    "GCP_S3_SECRET_KEY",
	},
	"alibaba": {
		"AccessKeyId":     "ALIBABA_CLOUD_ACCESS_KEY_ID",
		"AccessKeySecret": "ALIBABA_CLOUD_ACCESS_KEY_SECRET",
	},
	"ibm": {
		"ApiKey":      "IC_API_KEY",
		"S3AccessKey": "IBM_S3_ACCESS_KEY",
		"S3SecretKey": "IBM_S3_SECRET_KEY",
	},
	"ncp": {
		"ncloud_access_key": "NCLOUD_ACCESS_KEY",
		"ncloud_secret_key": "NCLOUD_SECRET_KEY",
	},
	"tencent": {
		"SecretId":  "TENCENTCLOUD_SECRET_ID",
		"SecretKey": "TENCENTCLOUD_SECRET_KEY",
	},
	"kt": {
		"IdentityEndpoint": "KT_IDENTITY_ENDPOINT",
		"Username":         "KT_USERNAME",
		"Password":         "KT_PASSWORD",
		"DomainName":       "KT_DOMAIN_NAME",
		"ProjectID":        "KT_PROJECT_ID",
		"S3AccessKey":      "KT_S3_ACCESS_KEY",
		"S3SecretKey":      "KT_S3_SECRET_KEY",
	},
	"nhn": {
		"IdentityEndpoint": "NHN_IDENTITY_ENDPOINT",
		"Username":         "NHN_USERNAME",
		"Password":         "NHN_PASSWORD",
		"DomainName":       "NHN_DOMAIN_NAME",
		"TenantId":         "NHN_TENANT_ID",
		"S3AccessKey":      "NHN_S3_ACCESS_KEY",
		"S3SecretKey":      "NHN_S3_SECRET_KEY",
	},
	"openstack": {
		"IdentityEndpoint": "OS_AUTH_URL",
		"Username":         "OS_USERNAME",
		"Password":         "OS_PASSWORD",
		"DomainName":       "OS_DOMAIN_NAME",
		"ProjectID":        "OS_PROJECT_ID",
		"S3AccessKey":      "OS_S3_ACCESS_KEY",
		"S3SecretKey":      "OS_S3_SECRET_KEY",
	},
}

// ApplyCredentialKeyMap transforms a credential key-value list using the CSP-specific
// key mapping. Keys not present in the map are passed through unchanged. Mapped keys
// with no incoming value are filled with "" so consumers of the secret (e.g.
// mc-terrarium's vault_kv_secret_v2 data sources) always find the full expected key
// set — same behavior as init/openbao/openbao-register-creds.py.
func ApplyCredentialKeyMap(provider string, kvList []model.KeyValue) map[string]any {
	keyMap := credentialKeyMap[strings.ToLower(provider)]
	result := make(map[string]any, len(kvList))
	for _, kv := range kvList {
		targetKey := kv.Key
		if keyMap != nil {
			if mapped, ok := keyMap[kv.Key]; ok {
				targetKey = mapped
			}
		}
		result[targetKey] = kv.Value
	}
	for _, terrariumKey := range keyMap {
		if _, ok := result[terrariumKey]; !ok {
			result[terrariumKey] = ""
		}
	}
	return result
}

// BuildSecretPathForHolder builds the OpenBao secret path using holder and provider directly.
// Both holder and provider are lowercased to stay consistent with BuildSecretPath.
func BuildSecretPathForHolder(holder, provider string) string {
	holder = strings.ToLower(holder)
	provider = strings.ToLower(provider)
	if strings.EqualFold(holder, model.DefaultCredentialHolder) {
		return fmt.Sprintf("secret/data/csp/%s", provider)
	}
	return fmt.Sprintf("secret/data/users/%s/csp/%s", holder, provider)
}

// WriteOpenBaoSecret writes key-value data to OpenBao at the given KV v2 path (upsert).
// ctx allows request-scoped cancellation and timeout, consistent with ReadOpenBaoSecret.
func WriteOpenBaoSecret(ctx context.Context, path string, data map[string]any) error {
	if model.VaultToken == "" {
		return fmt.Errorf("VAULT_TOKEN is not set")
	}

	vaultConfig := api.DefaultConfig()
	vaultConfig.Address = model.VaultAddr
	client, err := api.NewClient(vaultConfig)
	if err != nil {
		return fmt.Errorf("failed to create OpenBao client: %w", err)
	}
	client.SetToken(model.VaultToken)

	_, err = client.Logical().WriteWithContext(ctx, path, map[string]any{
		"data": data,
	})
	if err != nil {
		return fmt.Errorf("failed to write secret to OpenBao at %s: %w", path, err)
	}
	return nil
}

// WriteOpenBaoSecretIfAbsent writes key-value data to OpenBao only when no version of
// the secret exists yet (KV v2 check-and-set with cas=0). This is atomic on the server
// side, so it can never overwrite a real credential written concurrently.
// Returns created=false with a nil error when the secret already exists.
func WriteOpenBaoSecretIfAbsent(ctx context.Context, path string, data map[string]any) (created bool, err error) {
	if model.VaultToken == "" {
		return false, fmt.Errorf("VAULT_TOKEN is not set")
	}

	vaultConfig := api.DefaultConfig()
	vaultConfig.Address = model.VaultAddr
	client, err := api.NewClient(vaultConfig)
	if err != nil {
		return false, fmt.Errorf("failed to create OpenBao client: %w", err)
	}
	client.SetToken(model.VaultToken)

	_, err = client.Logical().WriteWithContext(ctx, path, map[string]any{
		"data":    data,
		"options": map[string]any{"cas": 0},
	})
	if err != nil {
		if isCasConflict(err) {
			return false, nil // secret already exists — leave it untouched
		}
		return false, fmt.Errorf("failed to write secret to OpenBao at %s: %w", path, err)
	}
	return true, nil
}

// isCasConflict reports whether err is a KV v2 check-and-set version conflict
// (the secret already has a version). Prefers the structured ResponseError
// (HTTP 400 + error payload) over matching the flattened error string.
func isCasConflict(err error) bool {
	var respErr *api.ResponseError
	if errors.As(err, &respErr) {
		if respErr.StatusCode != 400 {
			return false
		}
		for _, e := range respErr.Errors {
			if strings.Contains(e, "check-and-set") {
				return true
			}
		}
		return false
	}
	return strings.Contains(err.Error(), "check-and-set")
}

// placeholderSweepDone latches once EnsurePlaceholderCredentialSecrets has fully
// succeeded, so repeated credential registrations don't re-sweep every provider.
// placeholderSweepBusy single-flights the sweep: concurrent credential
// registrations (init registers all CSPs in parallel) trigger only one sweep.
var (
	placeholderSweepDone atomic.Bool
	placeholderSweepBusy atomic.Bool
)

// EnsurePlaceholderCredentialSecrets writes an all-empty placeholder secret for every
// known CSP that has no secret yet under the default credential holder path.
// This keeps consumers that read all CSP secret paths (e.g. mc-terrarium's
// vault_kv_secret_v2 data sources during `tofu plan`) from hard-failing on CSPs whose
// credentials were never provided — they fail gracefully at auth time instead.
// Existing secrets are never touched (CAS-protected), so real credentials always win.
func EnsurePlaceholderCredentialSecrets(ctx context.Context) {
	if placeholderSweepDone.Load() || model.VaultToken == "" || model.VaultAddr == "" {
		return
	}
	if !placeholderSweepBusy.CompareAndSwap(false, true) {
		return // another registration is already sweeping
	}
	defer placeholderSweepBusy.Store(false)
	allOK := true
	for provider, keyMap := range credentialKeyMap {
		placeholder := make(map[string]any, len(keyMap))
		for _, terrariumKey := range keyMap {
			placeholder[terrariumKey] = ""
		}
		path := BuildSecretPathForHolder(model.DefaultCredentialHolder, provider)
		created, err := WriteOpenBaoSecretIfAbsent(ctx, path, placeholder)
		if err != nil {
			allOK = false
			log.Warn().Err(err).Str("provider", provider).Msg("[CSP] failed to ensure placeholder secret in OpenBao")
			continue
		}
		if created {
			log.Info().Msgf("[CSP] placeholder secret registered in OpenBao: %s (%d empty keys)", path, len(placeholder))
		}
	}
	if allOK {
		placeholderSweepDone.Store(true)
	}
}

// ReadOpenBaoSecret reads a secret from OpenBao at the given path and returns the data map.
// It validates that VaultToken is set and the secret exists.
// A context is used for request-scoped cancellation and timeout.
func ReadOpenBaoSecret(ctx context.Context, path string) (map[string]any, error) {
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

	data, ok := secret.Data["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid secret format at %s: 'data' field missing or not a map", path)
	}

	return data, nil
}

// CheckOpenBaoStatus verifies that the OpenBao secret store is usable by
// CB-Tumblebug: endpoint reachable, initialized, unsealed, and the configured
// VAULT_TOKEN accepted. It stops at the first failed step, so Message always
// describes the most actionable problem.
func CheckOpenBaoStatus(ctx context.Context) model.OpenBaoStatusInfo {
	status := model.OpenBaoStatusInfo{
		VaultAddr:     model.VaultAddr,
		VaultTokenSet: model.VaultToken != "",
	}

	if model.VaultAddr == "" {
		status.Message = "VAULT_ADDR is not set in the cb-tumblebug environment"
		return status
	}

	vaultConfig := api.DefaultConfig()
	vaultConfig.Address = model.VaultAddr
	client, err := api.NewClient(vaultConfig)
	if err != nil {
		status.Message = fmt.Sprintf("failed to create OpenBao client: %v", err)
		return status
	}

	// Seal status requires no token — distinguishes unreachable / uninitialized / sealed.
	sealStatus, err := client.Sys().SealStatusWithContext(ctx)
	if err != nil {
		status.Message = fmt.Sprintf("cannot reach OpenBao at %s: %v", model.VaultAddr, err)
		return status
	}
	status.Reachable = true
	status.Initialized = sealStatus.Initialized
	status.Sealed = sealStatus.Sealed
	if !sealStatus.Initialized {
		status.Message = "OpenBao is not initialized"
		return status
	}
	if sealStatus.Sealed {
		status.Message = "OpenBao is sealed; secrets are inaccessible until it is unsealed"
		return status
	}

	if model.VaultToken == "" {
		status.Message = "VAULT_TOKEN is not set in the cb-tumblebug environment"
		return status
	}

	client.SetToken(model.VaultToken)
	if _, err := client.Auth().Token().LookupSelfWithContext(ctx); err != nil {
		status.Message = fmt.Sprintf("VAULT_TOKEN was rejected by OpenBao: %v", err)
		return status
	}
	status.TokenValid = true
	status.Available = true
	status.Message = "OpenBao is available for credential storage"
	return status
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

	if strings.EqualFold(holder, model.DefaultCredentialHolder) {
		return fmt.Sprintf("secret/data/csp/%s", provider)
	}
	return fmt.Sprintf("secret/data/users/%s/csp/%s", holder, provider)
}

// GetString safely extracts a string value from a map.
func GetString(data map[string]any, key string) string {
	v, _ := data[key].(string)
	return v
}

// LogCSP logs a CSP-related message with consistent prefix.
func LogCSP(provider, msg string) {
	log.Debug().Str("provider", provider).Msg("[CSP] " + msg)
}
