/*
Copyright 2019 The Cloud-Barista Authors.
<!-- SPDX-License-Identifier: Apache-2.0 -->
*/

package resource

import (
	"strings"
	"testing"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
)

func TestNormalizePrivateKey(t *testing.T) {
	tests := []struct {
		name         string
		privateKey   string
		keyValueList []model.KeyValue
		expected     string
	}{
		{
			name:         "Already normalized key with real newlines",
			privateKey:   "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----\n",
			keyValueList: nil,
			expected:     "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----\n",
		},
		{
			name:         "Key containing escaped newlines",
			privateKey:   "-----BEGIN RSA PRIVATE KEY-----\\nMIIEowIBAAKCAQEA...\\n-----END RSA PRIVATE KEY-----\\n",
			keyValueList: nil,
			expected:     "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----\n",
		},
		{
			name:       "Key in keyValueList from Tencent format",
			privateKey: "",
			keyValueList: []model.KeyValue{
				{Key: "OtherKey", Value: "OtherVal"},
				{Key: "PrivateKey", Value: "-----BEGIN RSA PRIVATE KEY-----\\nMIIEowIBAAKCAQEA...\\n-----END RSA PRIVATE KEY-----\\n"},
			},
			expected: "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----\n",
		},
		{
			name:         "Empty key and empty keyValueList",
			privateKey:   "",
			keyValueList: []model.KeyValue{},
			expected:     "",
		},
		{
			name:       "Single line without newlines unchanged",
			privateKey: "some-raw-private-key-string",
			keyValueList: []model.KeyValue{
				{Key: "SomeKey", Value: "SomeVal"},
			},
			expected: "some-raw-private-key-string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizePrivateKey(tt.privateKey, tt.keyValueList)
			if got != tt.expected {
				t.Errorf("normalizePrivateKey() = %q, want %q", got, tt.expected)
			}
			if tt.expected != "" && strings.Contains(tt.expected, "\n") && !strings.Contains(got, "\n") {
				t.Errorf("expected newlines in output, got none")
			}
		})
	}
}
