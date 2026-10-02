/*
Copyright 2019 The Cloud-Barista Authors.
<!-- SPDX-License-Identifier: Apache-2.0 -->
*/

package secret

import (
	"testing"
)

func TestBuildSshKeySecretPath(t *testing.T) {
	tests := []struct {
		name     string
		nsId     string
		sshKeyId string
		expected string
	}{
		{
			name:     "standard namespace and sshkey",
			nsId:     "default",
			sshKeyId: "my-ssh-key",
			expected: "secret/data/namespaces/default/sshkeys/my-ssh-key",
		},
		{
			name:     "mixed case namespace and sshkey",
			nsId:     "Default-NS",
			sshKeyId: "My-SSH-Key-01",
			expected: "secret/data/namespaces/default-ns/sshkeys/my-ssh-key-01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildSshKeySecretPath(tt.nsId, tt.sshKeyId)
			if got != tt.expected {
				t.Errorf("BuildSshKeySecretPath() = %q, want %q", got, tt.expected)
			}
		})
	}
}
