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
package csp

import (
	"regexp"
	"strings"
)

// Query parameters whose value identifies or authenticates the caller. Matched
// case-insensitively, so AccessKeyId and accessKeyID are both covered.
var sensitiveQueryParams = []string{
	"accesskeyid",
	"accesskey",
	"secretaccesskey",
	"signature",
	"signaturenonce",
	"credential",
	"password",
	"token",
	"apikey",
	"clientsecret",
	"x-amz-signature",
	"x-amz-credential",
	"x-amz-security-token",
	"sig",
	"sas",
}

// A query parameter is "name=value" up to the next & or the closing quote/space that
// ends the URL inside an error message.
var queryParamPattern = regexp.MustCompile(`([?&])([A-Za-z0-9_\-]+)=([^&"'\s]*)`)

// RedactSecrets removes credential material from a message before it is logged or
// returned to a caller.
//
// SDKs that sign with query parameters - Alibaba Cloud is the one CB-Tumblebug calls
// directly - put the whole signed URL in their error strings. Wrapping such an error
// with %w and logging it publishes the access key and signature; the same string is
// also copied into the errorMsg field of fetch results, so it reaches API clients too.
// Only the parameter values are removed: the host and the Action stay, which is what
// makes the error diagnosable.
func RedactSecrets(message string) string {
	if message == "" || !strings.ContainsAny(message, "?&") {
		return message
	}
	return queryParamPattern.ReplaceAllStringFunc(message, func(match string) string {
		parts := queryParamPattern.FindStringSubmatch(match)
		if len(parts) != 4 {
			return match
		}
		separator, name, value := parts[1], parts[2], parts[3]
		if value == "" {
			return match
		}
		for _, sensitive := range sensitiveQueryParams {
			if strings.EqualFold(name, sensitive) {
				return separator + name + "=[redacted]"
			}
		}
		return match
	})
}

// RedactErr is RedactSecrets over an error, for use where an SDK error is wrapped into
// a message. It returns "" for a nil error so callers can format it directly.
func RedactErr(err error) string {
	if err == nil {
		return ""
	}
	return RedactSecrets(err.Error())
}
