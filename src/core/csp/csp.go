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

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/openbao/openbao/api/v2"
	"github.com/rs/zerolog/log"
)

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
	holder := common.CredentialHolderFromContext(ctx)
	if holder == "" || strings.EqualFold(holder, "admin") {
		holder = "admin"
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
