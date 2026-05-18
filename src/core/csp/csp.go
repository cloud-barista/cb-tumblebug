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
		"ApiKey":    "IC_API_KEY",
		"S3AccessKey": "IBM_S3_ACCESS_KEY",
		"S3SecretKey": "IBM_S3_SECRET_KEY",
	},
	"ncp": {
		"ncloud_access_key": "NCLOUD_ACCESS_KEY",
		"ncloud_secret_key": "NCLOUD_SECRET_KEY",
	},
	"tencent": {
		"SecretId": "TENCENTCLOUD_SECRET_ID",
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
// key mapping. Keys not present in the map are passed through unchanged.
func ApplyCredentialKeyMap(provider string, kvList []model.KeyValue) map[string]interface{} {
	keyMap := credentialKeyMap[strings.ToLower(provider)]
	result := make(map[string]interface{}, len(kvList))
	for _, kv := range kvList {
		targetKey := kv.Key
		if keyMap != nil {
			if mapped, ok := keyMap[kv.Key]; ok {
				targetKey = mapped
			}
		}
		result[targetKey] = kv.Value
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
func WriteOpenBaoSecret(ctx context.Context, path string, data map[string]interface{}) error {
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

	_, err = client.Logical().WriteWithContext(ctx, path, map[string]interface{}{
		"data": data,
	})
	if err != nil {
		return fmt.Errorf("failed to write secret to OpenBao at %s: %w", path, err)
	}
	return nil
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

	if strings.EqualFold(holder, model.DefaultCredentialHolder) {
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
