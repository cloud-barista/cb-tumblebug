/*
Copyright 2019 The Cloud-Barista Authors.
<!-- SPDX-License-Identifier: Apache-2.0 -->
*/

// Package secret provides unified access to secret storage (OpenBao / HashiCorp Vault).
package secret

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/openbao/openbao/api/v2"
)

var (
	vaultClientOnce sync.Once
	vaultClient     *api.Client
	vaultClientErr  error
	secretCacheMu   sync.RWMutex
	secretCache     = map[string]secretCacheEntry{}
)

const secretCacheTTL = 5 * time.Minute

type secretCacheEntry struct {
	data   map[string]any
	expiry time.Time
}

func sharedVaultClient() (*api.Client, error) {
	vaultClientOnce.Do(func() {
		cfg := api.DefaultConfig()
		cfg.Address = model.VaultAddr
		vaultClient, vaultClientErr = api.NewClient(cfg)
		if vaultClientErr != nil {
			vaultClientErr = fmt.Errorf("failed to create OpenBao client: %w", vaultClientErr)
			return
		}
		vaultClient.SetToken(model.VaultToken)
	})
	return vaultClient, vaultClientErr
}

func cachedSecret(path string) (map[string]any, bool) {
	secretCacheMu.RLock()
	defer secretCacheMu.RUnlock()
	e, ok := secretCache[path]
	if !ok || time.Now().After(e.expiry) {
		return nil, false
	}
	return e.data, true
}

func storeSecretCache(path string, data map[string]any) {
	secretCacheMu.Lock()
	secretCache[path] = secretCacheEntry{data: data, expiry: time.Now().Add(secretCacheTTL)}
	secretCacheMu.Unlock()
}

// InvalidateSecretCache drops a cached secret (call after writes or deletes).
func InvalidateSecretCache(path string) {
	secretCacheMu.Lock()
	delete(secretCache, path)
	secretCacheMu.Unlock()
}

// WriteSecret writes key-value data to OpenBao at the given KV v2 path (upsert).
func WriteSecret(ctx context.Context, path string, data map[string]any) error {
	defer InvalidateSecretCache(path)
	if model.VaultToken == "" {
		return fmt.Errorf("VAULT_TOKEN is not set")
	}

	client, err := sharedVaultClient()
	if err != nil {
		return fmt.Errorf("failed to create OpenBao client: %w", err)
	}

	_, err = client.Logical().WriteWithContext(ctx, path, map[string]any{
		"data": data,
	})
	if err != nil {
		return fmt.Errorf("failed to write secret to OpenBao at %s: %w", path, err)
	}
	return nil
}

// WriteSecretIfAbsent writes key-value data to OpenBao only when no version of
// the secret exists yet (KV v2 check-and-set with cas=0).
func WriteSecretIfAbsent(ctx context.Context, path string, data map[string]any) (created bool, err error) {
	if model.VaultToken == "" {
		return false, fmt.Errorf("VAULT_TOKEN is not set")
	}

	client, err := sharedVaultClient()
	if err != nil {
		return false, fmt.Errorf("failed to create OpenBao client: %w", err)
	}

	_, err = client.Logical().WriteWithContext(ctx, path, map[string]any{
		"data":    data,
		"options": map[string]any{"cas": 0},
	})
	if err != nil {
		if IsCasConflict(err) {
			return false, nil // secret already exists — leave untouched
		}
		return false, fmt.Errorf("failed to write secret to OpenBao at %s: %w", path, err)
	}
	return true, nil
}

// IsCasConflict reports whether err is a KV v2 check-and-set version conflict.
func IsCasConflict(err error) bool {
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

// ReadSecret reads a secret from OpenBao at the given path and returns the data map.
func ReadSecret(ctx context.Context, path string) (map[string]any, error) {
	if model.VaultToken == "" {
		return nil, fmt.Errorf("VAULT_TOKEN is not set")
	}
	if data, ok := cachedSecret(path); ok {
		return data, nil
	}
	client, err := sharedVaultClient()
	if err != nil {
		return nil, err
	}

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
	storeSecretCache(path, data)
	return data, nil
}

// DeleteSecret deletes the secret at the given KV v2 path in OpenBao.
// It removes all versions and metadata for the secret.
func DeleteSecret(ctx context.Context, path string) error {
	defer InvalidateSecretCache(path)
	if model.VaultToken == "" {
		return fmt.Errorf("VAULT_TOKEN is not set")
	}

	client, err := sharedVaultClient()
	if err != nil {
		return err
	}

	metadataPath := strings.Replace(path, "secret/data/", "secret/metadata/", 1)
	_, err = client.Logical().DeleteWithContext(ctx, metadataPath)
	if err != nil {
		return fmt.Errorf("failed to delete secret from OpenBao at %s: %w", metadataPath, err)
	}
	return nil
}

// CheckStatus verifies that the OpenBao secret store is usable by CB-Tumblebug.
func CheckStatus(ctx context.Context) model.OpenBaoStatusInfo {
	status := model.OpenBaoStatusInfo{
		VaultAddr: model.VaultAddr,
	}

	if model.VaultAddr == "" || model.VaultToken == "" {
		status.Message = "OpenBao is not configured (VAULT_ADDR or VAULT_TOKEN not set)"
		return status
	}

	cfg := api.DefaultConfig()
	cfg.Address = model.VaultAddr
	client, err := api.NewClient(cfg)
	if err != nil {
		status.Message = fmt.Sprintf("failed to create OpenBao client: %v", err)
		return status
	}

	sealStatus, err := client.Sys().SealStatusWithContext(ctx)
	if err != nil {
		status.Message = fmt.Sprintf("cannot reach OpenBao at %s: %v", model.VaultAddr, err)
		return status
	}
	status.Reachable = true

	if !sealStatus.Initialized {
		status.Message = "OpenBao is not initialized"
		return status
	}
	status.Initialized = true

	if sealStatus.Sealed {
		status.Message = "OpenBao is sealed; secrets are inaccessible until it is unsealed"
		return status
	}
	status.Sealed = false

	client.SetToken(model.VaultToken)
	_, err = client.Auth().Token().LookupSelfWithContext(ctx)
	if err != nil {
		status.Message = fmt.Sprintf("VAULT_TOKEN was rejected by OpenBao: %v", err)
		return status
	}
	status.TokenValid = true

	status.Message = "OpenBao is available for credential storage"
	return status
}
