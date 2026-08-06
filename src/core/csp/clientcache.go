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

package csp

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
)

// SDK clients that hold an authentication token are cached so the token is reused
// instead of re-fetched on every call. Such a cache must be keyed by the credential
// actually in use, not by the account it belongs to: keying by account alone serves
// the client built from the first credential forever, so a rotated credential is
// never picked up and the CSP calls keep failing with the old one.

// CredKey returns a short, non-reversible id of the given credential values.
// Cache keys must include it so that a rotated credential yields a new entry.
func CredKey(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:8])
}

// LoadClient returns the client cached for this account and credential.
func LoadClient(cache *sync.Map, account, credKey string) (any, bool) {
	return cache.Load(account + "|" + credKey)
}

// StoreClient caches a client for this account and credential, and drops the entries
// this account had under a superseded credential so they do not accumulate.
func StoreClient(cache *sync.Map, account, credKey string, client any) any {
	key := account + "|" + credKey
	actual, _ := cache.LoadOrStore(key, client)

	prefix := account + "|"
	cache.Range(func(k, _ any) bool {
		if stored, ok := k.(string); ok && stored != key && strings.HasPrefix(stored, prefix) {
			cache.Delete(stored)
		}
		return true
	})
	return actual
}
