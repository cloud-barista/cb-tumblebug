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

// Package common is to include common methods for managing multi-cloud infra
package common

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/logger"
	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	modelcsp "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/rs/zerolog/log"
)

func GetCredentialHolderList() (model.CredentialHolderList, error) {
	allConnections, err := GetConnConfigList("", false, false)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get connection config list for credential holders")
		return model.CredentialHolderList{}, err
	}

	type holderStats struct {
		providers       map[string]bool
		connectionCount int
		verifiedCount   int
	}

	holderMap := make(map[string]*holderStats)

	for _, conn := range allConnections.Connectionconfig {
		holder := strings.ToLower(conn.CredentialHolder)
		if holder == "" {
			holder = model.DefaultCredentialHolder
		}
		stats, exists := holderMap[holder]
		if !exists {
			stats = &holderStats{providers: make(map[string]bool)}
			holderMap[holder] = stats
		}
		stats.providers[conn.ProviderName] = true
		stats.connectionCount++
		if conn.Verified {
			stats.verifiedCount++
		}
	}

	var result model.CredentialHolderList
	for holder, stats := range holderMap {
		providers := make([]string, 0, len(stats.providers))
		for p := range stats.providers {
			providers = append(providers, p)
		}
		sort.Strings(providers)
		result.CredentialHolderList = append(result.CredentialHolderList, model.CredentialHolderInfo{
			CredentialHolder:        holder,
			Providers:               providers,
			ConnectionCount:         stats.connectionCount,
			VerifiedConnectionCount: stats.verifiedCount,
			IsDefault:               strings.EqualFold(holder, model.DefaultCredentialHolder),
		})
	}

	// Sort by holder name for consistent output
	sort.Slice(result.CredentialHolderList, func(i, j int) bool {
		return result.CredentialHolderList[i].CredentialHolder < result.CredentialHolderList[j].CredentialHolder
	})

	return result, nil
}

// GetCredentialHolder retrieves a specific credential holder's info derived from connection configs
func GetCredentialHolder(holderId string) (model.CredentialHolderInfo, error) {
	if holderId == "" {
		return model.CredentialHolderInfo{}, fmt.Errorf("holderId is required")
	}
	holderId = strings.ToLower(holderId)

	allConnections, err := GetConnConfigList(holderId, false, false)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to get connection configs for credential holder '%s'", holderId)
		return model.CredentialHolderInfo{}, err
	}

	if len(allConnections.Connectionconfig) == 0 {
		return model.CredentialHolderInfo{}, fmt.Errorf("credential holder '%s' not found (no connections registered)", holderId)
	}

	providerSet := make(map[string]bool)
	verifiedCount := 0
	for _, conn := range allConnections.Connectionconfig {
		providerSet[conn.ProviderName] = true
		if conn.Verified {
			verifiedCount++
		}
	}

	providers := make([]string, 0, len(providerSet))
	for p := range providerSet {
		providers = append(providers, p)
	}
	sort.Strings(providers)

	return model.CredentialHolderInfo{
		CredentialHolder:        holderId,
		Providers:               providers,
		ConnectionCount:         len(allConnections.Connectionconfig),
		VerifiedConnectionCount: verifiedCount,
		IsDefault:               strings.EqualFold(holderId, model.DefaultCredentialHolder),
	}, nil
}

// ResolveConnectionName converts a default credential holder's connection name
// to the appropriate connection name for the given credential holder.
// Default holder connections use the pattern: "{provider}-{region}" (e.g., "aws-ap-northeast-2")
// Non-default holder connections use: "{holder}-{provider}-{region}" (e.g., "team-a-aws-ap-northeast-2")
// If credentialHolder is empty or matches the default, the original name is returned as-is.
func ResolveConnectionName(defaultConnectionName string, credentialHolder string) string {
	if credentialHolder == "" || strings.EqualFold(credentialHolder, model.DefaultCredentialHolder) {
		return defaultConnectionName
	}
	return credentialHolder + "-" + defaultConnectionName
}

// LookupKeyValueList is func to lookup model.KeyValue list
func LookupKeyValueList(kvl []model.KeyValue, key string) string {
	for _, v := range kvl {
		if v.Key == key {
			return v.Value
		}
	}
	return ""
}

// PrintJsonPretty prints v as indented JSON for debugging. This output bypasses the logger, so script bodies,
// env values and VM user data are replaced by placeholders and the log masking is applied here.
func PrintJsonPretty(v any) {
	prettyJSON, err := redactedPrettyJSON(v)
	if err != nil {
		fmt.Printf("%+v\n", v)
	} else {
		fmt.Printf("%s\n", string(prettyJSON))
	}
}

func redactedPrettyJSON(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var tree any
	if err := dec.Decode(&tree); err != nil {
		return nil, err
	}
	pretty, err := json.MarshalIndent(redactForPrint(tree), "", "  ")
	if err != nil {
		return nil, err
	}
	return logger.MaskSensitiveData(pretty), nil
}

// redactForPrint walks a decoded JSON value and hides script bodies, env values and VM user data, which can
// carry tokens and user secrets under any key name.
func redactForPrint(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			switch strings.ToLower(k) {
			case "script", "userdata":
				if str, ok := val.(string); ok && str != "" {
					t[k] = fmt.Sprintf("[%d bytes redacted]", len(str))
					continue
				}
			case "env":
				if env, ok := val.(map[string]any); ok {
					for name := range env {
						env[name] = "********"
					}
					continue
				}
			}
			t[k] = redactForPrint(val)
		}
		return t
	case []any:
		for i := range t {
			t[i] = redactForPrint(t[i])
		}
		return t
	}
	return v
}

// GenResourceKey is func to generate a key from resource type and id

func GetPublicKeyForCredentialEncryption() (model.PublicKeyResponse, error) {

	privateKey, err := rsa.GenerateKey(crand.Reader, 4096)
	if err != nil {
		return model.PublicKeyResponse{}, fmt.Errorf("failed to generate RSA key: %w", err)
	}

	uid := GenUid()

	mu.Lock()
	privateKeyStore[uid] = privateKey
	mu.Unlock()

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(&privateKey.PublicKey),
	})

	return model.PublicKeyResponse{
		PublicKeyTokenId: uid,
		PublicKey:        string(publicKeyPEM),
	}, nil
}

// hashFunction is the hash function used for RSA-OAEP decryption
var hashFunction = sha256.New

// unpad function to remove padding after AES decryption
func unpad(data []byte, blockSize int) ([]byte, error) {
	length := len(data)
	unpadding := int(data[length-1])
	if unpadding > blockSize || unpadding > length {
		return nil, fmt.Errorf("invalid padding size")
	}
	return data[:(length - unpadding)], nil
}

// RegisterCredential is func to register credential and all related connection configs
func RegisterCredential(req model.CredentialReq) (model.CredentialInfo, error) {

	mu.Lock()
	privateKey, exists := privateKeyStore[req.PublicKeyTokenId]
	mu.Unlock()

	if !exists {
		return model.CredentialInfo{}, fmt.Errorf("private key not found for token ID: %s", req.PublicKeyTokenId)
	}

	// Validate credential key formats before any decryption work. Keys are plaintext
	// (only values are encrypted), so unknown/misspelled keys can be rejected up front
	// with a message listing the accepted keys for the provider. Only the provided keys
	// are checked — a subset is allowed, so optional keys (e.g. S3AccessKey/S3SecretKey)
	// need not be present.
	providedKeys := make([]string, len(req.CredentialKeyValueList))
	for i, keyValue := range req.CredentialKeyValueList {
		providedKeys[i] = keyValue.Key
	}
	if err := csp.ValidateCredentialKeys(req.ProviderName, providedKeys); err != nil {
		return model.CredentialInfo{}, err
	}

	// PrintJsonPretty(req)

	// Decrypt the AES key
	encryptedAesKey, err := base64.StdEncoding.DecodeString(req.EncryptedClientAesKeyByPublicKey)
	if err != nil {
		return model.CredentialInfo{}, fmt.Errorf("failed to decode encrypted AES key: %w", err)
	}

	aesKey, err := rsa.DecryptOAEP(
		sha256.New(), crand.Reader, privateKey, encryptedAesKey, nil,
	)
	if err != nil {
		return model.CredentialInfo{}, fmt.Errorf("failed to decrypt AES key: %w", err)
	}

	// Clear AES key from memory after use
	defer func() {
		for i := range aesKey {
			aesKey[i] = 0
		}
	}()

	decryptedKeyValueList := make([]model.KeyValue, len(req.CredentialKeyValueList))

	// Decrypt all encrypted values and populate the new list
	for i, keyValue := range req.CredentialKeyValueList {
		encryptedBytes, err := base64.StdEncoding.DecodeString(keyValue.Value)
		if err != nil {
			log.Error().Err(err).Msg("")
			return model.CredentialInfo{}, fmt.Errorf("failed to decode encrypted value: %w", err)
		}

		aesCipher, err := aes.NewCipher(aesKey)
		if err != nil {
			return model.CredentialInfo{}, fmt.Errorf("failed to create AES cipher: %w", err)
		}

		iv := encryptedBytes[:aes.BlockSize]
		ciphertext := encryptedBytes[aes.BlockSize:]
		aesBlock := cipher.NewCBCDecrypter(aesCipher, iv)
		decryptedValue := make([]byte, len(ciphertext))
		aesBlock.CryptBlocks(decryptedValue, ciphertext)

		// Remove padding
		decryptedValue, err = unpad(decryptedValue, aes.BlockSize)
		if err != nil {
			return model.CredentialInfo{}, fmt.Errorf("failed to unpad decrypted value: %w", err)
		}

		decryptedKeyValueList[i] = model.KeyValue{
			Key:   keyValue.Key,
			Value: string(decryptedValue),
		}
	}

	// Delete the private key from memory after use
	mu.Lock()
	delete(privateKeyStore, req.PublicKeyTokenId)
	mu.Unlock()

	req.CredentialHolder = strings.ToLower(req.CredentialHolder)
	if err := ValidateCredentialHolderName(req.CredentialHolder); err != nil {
		return model.CredentialInfo{}, fmt.Errorf("invalid credentialHolder: %w", err)
	}
	req.ProviderName = strings.ToLower(req.ProviderName)
	genneratedCredentialName := req.CredentialHolder + "-" + req.ProviderName
	if strings.EqualFold(req.CredentialHolder, model.DefaultCredentialHolder) {
		// credential with default credential holder (e.g., admin) has no prefix
		genneratedCredentialName = req.ProviderName
	}

	// replace `\\n` with `\n` in the value to restore the original PEM value
	for i, keyValue := range decryptedKeyValueList {
		decryptedKeyValueList[i].Value = strings.ReplaceAll(keyValue.Value, "\\n", "\n")
	}

	// Resolve cloud platform type for Spider registration.
	// Spider uses ProviderName to select the correct driver handler,
	// so it must be the platform type (e.g., "OPENSTACK") not the CSP instance name (e.g., "OPENSTACK-NEW01").
	platformName := modelcsp.ResolveCloudPlatform(req.ProviderName)

	reqToSpider := model.CredentialInfo{
		CredentialName:   genneratedCredentialName,
		ProviderName:     strings.ToUpper(platformName),
		KeyValueInfoList: decryptedKeyValueList,
	}

	client := clientManager.NewHttpClient()
	url := model.SpiderRestUrl + "/credential"
	method := "POST"
	var callResult model.CredentialInfo
	requestBody := reqToSpider

	//PrintJsonPretty(requestBody)

	_, err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&callResult,
		clientManager.MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return model.CredentialInfo{}, err
	}
	//PrintJsonPretty(callResult)

	// Register credentials in OpenBao for runtime CSP access (non-fatal: warn and
	// continue if unavailable). The outcome is reported in the response via
	// OpenBaoStatus so init tooling can surface silent failures to the user —
	// without OpenBao, direct CSP API features cannot access this credential.
	if model.VaultToken == "" {
		callResult.OpenBaoStatus = "skipped: VAULT_TOKEN is not set in the cb-tumblebug environment; credential NOT stored in OpenBao"
		log.Warn().Msgf("OpenBao registration skipped (VAULT_TOKEN not set): provider=%s holder=%s", req.ProviderName, req.CredentialHolder)
	} else {
		// Bound the OpenBao calls so a slow/unreachable OpenBao cannot stall
		// credential registration for long (write + placeholder sweep).
		openBaoCtx, openBaoCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer openBaoCancel()

		secretPath := csp.BuildSecretPathForHolder(req.CredentialHolder, req.ProviderName)
		secretData := csp.ApplyCredentialKeyMap(req.ProviderName, decryptedKeyValueList)
		if err := csp.WriteOpenBaoSecret(openBaoCtx, secretPath, secretData); err != nil {
			callResult.OpenBaoStatus = fmt.Sprintf("failed: %v; credential NOT stored in OpenBao", err)
			log.Warn().Err(err).Msgf("Failed to register credential in OpenBao (non-fatal): provider=%s holder=%s", req.ProviderName, req.CredentialHolder)
		} else {
			callResult.OpenBaoStatus = "registered at " + secretPath
			log.Info().Msgf("Registered credential in OpenBao: path=%s", secretPath)
		}
		// Ensure every known CSP has at least a placeholder secret so consumers
		// that read all CSP paths (e.g. mc-terrarium's tofu plan) don't hard-fail
		// on providers without credentials. CAS-protected: never overwrites.
		csp.EnsurePlaceholderCredentialSecrets(openBaoCtx)
	}

	callResult.CredentialHolder = req.CredentialHolder
	callResult.ProviderName = strings.ToLower(callResult.ProviderName)
	for callResultKey := range callResult.KeyValueInfoList {
		callResult.KeyValueInfoList[callResultKey].Value = "************"
	}

	// Use the original CSP name (req.ProviderName) for CloudInfo lookup,
	// not Spider's returned ProviderName which is the platform type.
	// e.g., look up "openstack-new01" in cloudinfo.yaml, not "openstack"
	cloudInfo, err := GetCloudInfo()
	if err != nil {
		return callResult, err
	}
	cspDetail, ok := cloudInfo.CSPs[req.ProviderName]
	if !ok {
		return callResult, fmt.Errorf("cloudType '%s' not found", req.ProviderName)
	}

	// Register connection config for all regions defined in CloudInfo for this CSP.
	// Iterate CloudInfo regions directly instead of filtering Spider regions by ProviderName,
	// to avoid prefix collision (e.g., "openstack-" matching both "openstack" and "openstack-new01" regions).
	// Also builds expectedConfigNames in the same loop for later validation.
	expectedConfigNames := make(map[string]bool)
	for regionName := range cspDetail.Regions {
		// Region was registered in Spider as: providerName + "-" + regionName
		spiderRegionName := req.ProviderName + "-" + regionName
		configName := req.CredentialHolder + "-" + spiderRegionName
		if strings.EqualFold(req.CredentialHolder, model.DefaultCredentialHolder) {
			configName = spiderRegionName
		}
		connConfig := model.ConnConfig{
			ConfigName:         configName,
			ProviderName:       req.ProviderName, // CSP instance name (e.g., "openstack-new01")
			DriverName:         cspDetail.Driver,
			CredentialName:     callResult.CredentialName,
			RegionZoneInfoName: spiderRegionName,
			CredentialHolder:   req.CredentialHolder,
		}
		_, err := RegisterConnectionConfig(connConfig)
		if err != nil {
			log.Error().Err(err).Msg("")
			return callResult, err
		}
		expectedConfigNames[configName] = true
	}

	// Override callResult.ProviderName back to the CSP instance name for downstream use
	callResult.ProviderName = req.ProviderName

	validate := true
	// filter only verified
	if validate {
		allConnections, err := GetConnConfigList(req.CredentialHolder, false, false)
		if err != nil {
			log.Error().Err(err).Msg("")
			return callResult, err
		}

		// (expectedConfigNames already built above)

		filteredConnections := model.ConnConfigList{}
		for _, connConfig := range allConnections.Connectionconfig {
			if expectedConfigNames[connConfig.ConfigName] {
				connConfig.ProviderName = strings.ToLower(connConfig.ProviderName)
				filteredConnections.Connectionconfig = append(filteredConnections.Connectionconfig, connConfig)
			}
		}

		var wg sync.WaitGroup
		results := make(chan model.ConnConfig, len(filteredConnections.Connectionconfig))

		total := len(filteredConnections.Connectionconfig)
		log.Info().Msgf("[%s] Verifying %d connection(s) in parallel (timeout: %s each) ...",
			req.ProviderName, total, clientManager.AvailabilityCheckTimeout)

		for _, connConfig := range filteredConnections.Connectionconfig {
			wg.Add(1)
			go func(connConfig model.ConnConfig) {
				defer wg.Done()
				RandomSleep(0, 10*1000)
				log.Info().Msgf("[%s] Checking availability: %s", req.ProviderName, connConfig.ConfigName)
				verified, err := CheckConnConfigAvailable(connConfig.ConfigName)
				if err != nil {
					log.Error().Err(err).Msgf("[%s] Cannot check ConnConfig %s (will mark unverified)", req.ProviderName, connConfig.ConfigName)
					connConfig.VerifiedMessage = csp.ExplainCredentialError(connConfig.ProviderName, err)
				}
				connConfig.Verified = verified
				status := "✗"
				if verified {
					status = "✓"
				}
				log.Info().Msgf("[%s] %s %s", req.ProviderName, status, connConfig.ConfigName)
				if verified {
					regionInfo, err := GetRegion(connConfig.ProviderName, connConfig.RegionDetail.RegionName)
					if err != nil {
						log.Error().Err(err).Msgf("Cannot get region for %s", connConfig.RegionDetail.RegionName)
						connConfig.Verified = false
					} else {
						connConfig.RegionDetail = regionInfo
					}
				}
				results <- connConfig
			}(connConfig)
		}

		go func() {
			wg.Wait()
			close(results)
		}()

		for result := range results {
			// Store the failure reason too, so the caller can tell an expired secret
			// from a permission or network problem without reading server logs
			if !result.Verified && result.VerifiedMessage == "" {
				continue
			}
			key := GenConnectionKey(result.ConfigName)
			val, err := json.Marshal(result)
			if err != nil {
				return model.CredentialInfo{}, err
			}
			err = kvstore.Put(string(key), string(val))
			if err != nil {
				return callResult, err
			}
		}
	}

	setRegionRepresentative := true
	if setRegionRepresentative {
		allConnections, err := GetConnConfigList(req.CredentialHolder, false, false)
		if err != nil {
			log.Error().Err(err).Msg("")
			return callResult, err
		}

		// Filter connections for this CSP instance using expected config names
		filteredConnections := model.ConnConfigList{}
		for _, connConfig := range allConnections.Connectionconfig {
			if expectedConfigNames[connConfig.ConfigName] {
				filteredConnections.Connectionconfig = append(filteredConnections.Connectionconfig, connConfig)
			}
		}
		log.Info().Msgf("[%s] filtered connection config: %d", req.ProviderName, len(filteredConnections.Connectionconfig))
		regionRepresentative := make(map[string]model.ConnConfig)
		for _, connConfig := range allConnections.Connectionconfig {
			prefix := req.ProviderName + "-" + connConfig.RegionDetail.RegionName
			if strings.EqualFold(connConfig.RegionZoneInfoName, prefix) {
				if _, exists := regionRepresentative[prefix]; !exists {
					regionRepresentative[prefix] = connConfig
				}
			}
		}
		for _, connConfig := range regionRepresentative {
			connConfig.RegionRepresentative = true
			key := GenConnectionKey(connConfig.ConfigName)
			val, err := json.Marshal(connConfig)
			if err != nil {
				return callResult, err
			}
			err = kvstore.Put(string(key), string(val))
			if err != nil {
				return callResult, err
			}
		}
	}

	verifyRegionRepresentativeAndUpdateZone := true
	if verifyRegionRepresentativeAndUpdateZone {
		verifiedConnections, err := GetConnConfigList(req.CredentialHolder, true, false)
		if err != nil {
			log.Error().Err(err).Msg("")
			return callResult, err
		}
		allRepresentativeRegionConnections, err := GetConnConfigList(req.CredentialHolder, false, true)
		for _, connConfig := range allRepresentativeRegionConnections.Connectionconfig {
			if expectedConfigNames[connConfig.ConfigName] {
				verified := false
				for _, verifiedConnConfig := range verifiedConnections.Connectionconfig {
					if strings.EqualFold(connConfig.ConfigName, verifiedConnConfig.ConfigName) {
						verified = true
					}
				}
				// update representative regionZone with the verified regionZone
				if !verified {
					for _, verifiedConnConfig := range verifiedConnections.Connectionconfig {
						if strings.HasPrefix(verifiedConnConfig.ConfigName, connConfig.ConfigName) {
							connConfig.RegionZoneInfoName = verifiedConnConfig.RegionZoneInfoName
							connConfig.RegionZoneInfo = verifiedConnConfig.RegionZoneInfo
							break
						}
					}
					// update DB
					key := GenConnectionKey(connConfig.ConfigName)
					val, err := json.Marshal(connConfig)
					if err != nil {
						return callResult, err
					}
					err = kvstore.Put(string(key), string(val))
					if err != nil {
						return callResult, err
					}
				}
			}
		}
	}

	callResult.AllConnections, err = GetConnConfigList(req.CredentialHolder, false, false)
	if err != nil {
		log.Error().Err(err).Msg("")
		return callResult, err
	}

	return callResult, nil
}

// RegisterConnectionConfig is func to register connection config to CB-Spider
