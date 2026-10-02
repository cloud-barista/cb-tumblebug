/*
Copyright 2019 The Cloud-Barista Authors.
<!-- SPDX-License-Identifier: Apache-2.0 -->
*/

package secret

import (
	"context"
	"fmt"
	"strings"
)

// BuildSshKeySecretPath builds the OpenBao secret path for a given SSH key.
func BuildSshKeySecretPath(nsId, sshKeyId string) string {
	return fmt.Sprintf("secret/data/namespaces/%s/sshkeys/%s", strings.ToLower(nsId), strings.ToLower(sshKeyId))
}

// SaveSshKey stores the SSH private key in OpenBao at the namespace/sshkey path.
func SaveSshKey(ctx context.Context, nsId, sshKeyId, privateKey string) error {
	if privateKey == "" {
		return nil
	}
	path := BuildSshKeySecretPath(nsId, sshKeyId)
	return WriteSecret(ctx, path, map[string]any{
		"privateKey": privateKey,
	})
}

// GetSshKey retrieves the SSH private key from OpenBao for the given namespace and SSH key ID.
func GetSshKey(ctx context.Context, nsId, sshKeyId string) (string, error) {
	path := BuildSshKeySecretPath(nsId, sshKeyId)
	data, err := ReadSecret(ctx, path)
	if err != nil {
		return "", err
	}
	if val, ok := data["privateKey"].(string); ok {
		return val, nil
	}
	return "", nil
}

// DeleteSshKey removes the SSH key secret from OpenBao.
func DeleteSshKey(ctx context.Context, nsId, sshKeyId string) error {
	path := BuildSshKeySecretPath(nsId, sshKeyId)
	return DeleteSecret(ctx, path)
}
