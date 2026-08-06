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
	"regexp"
	"strings"
)

// credentialErrorHints maps a CSP error signature to an actionable message. Without
// them every failure looks the same to an API caller ("verified: false"), even though
// an expired secret needs a reissue while a permission error needs a role assignment.
var credentialErrorHints = []struct {
	provider string // empty: any provider
	signature string
	hint      string
}{
	{"azure", "AADSTS7000222", "The client secret has expired. Issue a new secret in the Azure portal and register the credential again."},
	{"azure", "AADSTS7000215", "The client secret is invalid. Check the value and register the credential again."},
	{"azure", "AADSTS700016", "The application (clientId) was not found in this tenant. Check clientId and tenantId."},
	{"azure", "AADSTS90002", "The tenantId does not exist. Check the tenantId value."},
	{"azure", "AuthorizationFailed", "The credential is valid but lacks permission on the subscription. Assign a role (e.g. Contributor)."},
	{"aws", "InvalidClientTokenId", "The access key id does not exist. Check the key or issue a new one."},
	{"aws", "SignatureDoesNotMatch", "The secret access key does not match the access key id."},
	{"aws", "AuthFailure", "The credential is valid but lacks permission for this region or action."},
	{"gcp", "invalid_grant", "The service account key is invalid or was revoked. Issue a new key and register the credential again."},
	{"gcp", "PERMISSION_DENIED", "The credential is valid but lacks permission on the project."},
	{"alibaba", "InvalidAccessKeyId", "The access key id is invalid or disabled."},
	{"alibaba", "SignatureDoesNotMatch", "The access key secret does not match the access key id."},
	{"tencent", "AuthFailure.SecretIdNotFound", "The SecretId does not exist. Check the value or issue a new key."},
	{"tencent", "AuthFailure.SignatureFailure", "The SecretKey does not match the SecretId."},
	{"ncp", "Authentication failed", "The access key or secret key is invalid."},
	{"", "context deadline exceeded", "The CSP endpoint did not respond in time. This may be a network issue rather than a credential problem."},
	{"", "no such host", "The CSP endpoint could not be resolved. Check network/DNS from the server."},
}

// spiderWrapper matches the call-site suffix CB-Spider appends to relayed CSP errors,
// e.g. " (from cb-spider:1024/spider/allkeypair (500 Internal Server Error))".
var spiderWrapper = regexp.MustCompile(`\s*\(from [^)]*\([^)]*\)\)\s*$`)

// ExplainCredentialError turns a CSP error into a short, actionable message for the
// API caller. Unmatched errors are summarized rather than dropped.
func ExplainCredentialError(provider string, err error) string {
	if err == nil {
		return ""
	}
	raw := spiderWrapper.ReplaceAllString(err.Error(), "")
	provider = strings.ToLower(provider)

	for _, h := range credentialErrorHints {
		if h.provider != "" && h.provider != provider {
			continue
		}
		if strings.Contains(raw, h.signature) {
			return h.hint + " (CSP error: " + h.signature + ")"
		}
	}
	return summarizeError(raw)
}

// cspErrorCode matches error codes CSPs put late in a long HTTP dump, which plain
// truncation would cut off.
var cspErrorCode = regexp.MustCompile(`AADSTS\d+`)

// summarizeError keeps the message short enough for an API response while retaining
// the part that identifies the cause.
func summarizeError(raw string) string {
	const maxLen = 300
	compact := strings.Join(strings.Fields(raw), " ")
	if len(compact) > maxLen {
		compact = compact[:maxLen] + " ..."
		if code := cspErrorCode.FindString(raw); code != "" && !strings.Contains(compact, code) {
			compact = code + ": " + compact
		}
	}
	return compact
}
