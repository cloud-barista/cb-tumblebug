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

package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
)

// gcpLabelableTypes lists CB-Tumblebug resource types that can have GCP Labels set.
// GCP Labels are supported on VM instances and disks (CB-Spider TagHandler also
// only supports VM and DISK for GCP).
var gcpLabelableTypes = map[string]bool{
	"vm":       true, // Compute Engine instance
	"dataDisk": true, // Persistent disk
}

// gcpMaxLabels is the maximum number of labels GCP allows per resource.
const gcpMaxLabels = 64

// gcpKeyMaxLen and gcpValueMaxLen are GCP label length limits.
const gcpKeyMaxLen = 63
const gcpValueMaxLen = 63

// gcpInvalidChars matches any character NOT allowed in GCP label keys/values.
// GCP allows only lowercase letters, digits, hyphens, and underscores.
var gcpInvalidChars = regexp.MustCompile(`[^a-z0-9_-]`)

func init() {
	csp.RegisterBatchTagHandler(csptypes.GCP, BatchUpsertTags)
}

// sanitizeGCPKey converts a label key to GCP-compatible format.
// Rules: lowercase, replace invalid chars with '_', max 63 chars.
// Must start with a lowercase letter (prepend 'l_' if not).
func sanitizeGCPKey(key string) string {
	s := strings.ToLower(key)
	s = gcpInvalidChars.ReplaceAllString(s, "_")

	// GCP keys must start with a lowercase letter
	if len(s) > 0 && (s[0] < 'a' || s[0] > 'z') {
		s = "l_" + s
	}

	if len(s) > gcpKeyMaxLen {
		s = s[:gcpKeyMaxLen]
	}
	return s
}

// sanitizeGCPValue converts a label value to GCP-compatible format.
// Rules: lowercase, replace invalid chars with '_', max 63 chars. Empty is allowed.
func sanitizeGCPValue(value string) string {
	s := strings.ToLower(value)
	s = gcpInvalidChars.ReplaceAllString(s, "_")
	if len(s) > gcpValueMaxLen {
		s = s[:gcpValueMaxLen]
	}
	return s
}

// prioritizeLabels selects labels to fit within GCP's 64-label limit.
// User labels (without "sys." prefix) come first, then sys.* labels.
// Within each group, keys are sorted alphabetically for determinism.
func prioritizeLabels(tags map[string]string) map[string]string {
	type kv struct {
		key, value string
	}

	var userLabels, sysLabels []kv
	for k, v := range tags {
		sk := sanitizeGCPKey(k)
		sv := sanitizeGCPValue(v)
		if sk == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(k), "sys.") || strings.HasPrefix(sk, "sys_") {
			sysLabels = append(sysLabels, kv{sk, sv})
		} else {
			userLabels = append(userLabels, kv{sk, sv})
		}
	}

	// Sort each group for deterministic ordering
	sort.Slice(userLabels, func(i, j int) bool { return userLabels[i].key < userLabels[j].key })
	sort.Slice(sysLabels, func(i, j int) bool { return sysLabels[i].key < sysLabels[j].key })

	result := make(map[string]string, gcpMaxLabels)

	// User labels first
	for _, l := range userLabels {
		if len(result) >= gcpMaxLabels {
			break
		}
		result[l.key] = l.value
	}
	// Then sys.* labels
	for _, l := range sysLabels {
		if len(result) >= gcpMaxLabels {
			break
		}
		result[l.key] = l.value
	}

	return result
}

// truncateLabels reduces merged labels to maxCount by keeping new (non-sys) labels
// and dropping sys.* labels first, then oldest keys alphabetically.
func truncateLabels(merged map[string]string, maxCount int) map[string]string {
	if len(merged) <= maxCount {
		return merged
	}

	type kv struct{ key, value string }
	var user, sys []kv
	for k, v := range merged {
		if strings.HasPrefix(k, "sys_") || strings.HasPrefix(k, "l_sys_") {
			sys = append(sys, kv{k, v})
		} else {
			user = append(user, kv{k, v})
		}
	}
	sort.Slice(user, func(i, j int) bool { return user[i].key < user[j].key })
	sort.Slice(sys, func(i, j int) bool { return sys[i].key < sys[j].key })

	result := make(map[string]string, maxCount)
	for _, l := range user {
		if len(result) >= maxCount {
			break
		}
		result[l.key] = l.value
	}
	for _, l := range sys {
		if len(result) >= maxCount {
			break
		}
		result[l.key] = l.value
	}
	return result
}

// BatchUpsertTags sets labels on a GCP Compute resource (VM instance or disk).
// Labels are sanitized to meet GCP naming rules (lowercase, no special chars).
// Only "vm" and "dataDisk" resource types are supported; others return an error
// so the caller can fall back to CB-Spider.
func BatchUpsertTags(ctx context.Context, region, zone, cspResourceId, resourceType string, tags map[string]string) error {
	if !gcpLabelableTypes[resourceType] {
		return fmt.Errorf("resource type %q is not GCP-labelable via batch", resourceType)
	}

	// GCP Compute API requires zone (e.g., "us-east5-a"), not region (e.g., "us-east5")
	if zone == "" {
		return fmt.Errorf("GCP batch tag requires zone but got empty zone (region=%s)", region)
	}

	creds, err := getGCPCreds(ctx)
	if err != nil {
		return fmt.Errorf("failed to get GCP credentials: %w", err)
	}

	svc, err := newComputeService(ctx, creds)
	if err != nil {
		return fmt.Errorf("failed to create GCP Compute service: %w", err)
	}

	sanitized := prioritizeLabels(tags)
	if len(sanitized) == 0 {
		return nil
	}

	switch resourceType {
	case "vm":
		return upsertVMLabels(svc, creds.ProjectID, zone, cspResourceId, sanitized)
	case "dataDisk":
		return upsertDiskLabels(svc, creds.ProjectID, zone, cspResourceId, sanitized)
	default:
		return fmt.Errorf("unsupported GCP resource type: %s", resourceType)
	}
}

// upsertVMLabels merges labels onto a GCP VM instance.
func upsertVMLabels(svc *compute.Service, projectID, zone, instanceName string, labels map[string]string) error {
	instance, err := svc.Instances.Get(projectID, zone, instanceName).Do()
	if err != nil {
		return fmt.Errorf("GCP Instances.Get failed for %s: %w", instanceName, err)
	}

	// Merge: existing labels + new labels (new overwrites on conflict)
	merged := make(map[string]string)
	for k, v := range instance.Labels {
		merged[k] = v
	}
	for k, v := range labels {
		merged[k] = v
	}

	// Enforce GCP's 64-label limit after merge
	if len(merged) > gcpMaxLabels {
		merged = truncateLabels(merged, gcpMaxLabels)
	}

	req := &compute.InstancesSetLabelsRequest{
		LabelFingerprint: instance.LabelFingerprint,
		Labels:           merged,
	}

	op, err := svc.Instances.SetLabels(projectID, zone, instanceName, req).Do()
	if err != nil {
		return fmt.Errorf("GCP Instances.SetLabels failed for %s: %w", instanceName, err)
	}
	if op.Error != nil {
		return fmt.Errorf("GCP Instances.SetLabels operation error for %s: %v", instanceName, op.Error.Errors)
	}

	log.Debug().
		Str("instance", instanceName).
		Int("labelCount", len(merged)).
		Msg("[GCP] Batch labels upserted on VM instance")

	return nil
}

// upsertDiskLabels merges labels onto a GCP persistent disk.
func upsertDiskLabels(svc *compute.Service, projectID, zone, diskName string, labels map[string]string) error {
	disk, err := svc.Disks.Get(projectID, zone, diskName).Do()
	if err != nil {
		return fmt.Errorf("GCP Disks.Get failed for %s: %w", diskName, err)
	}

	// Merge: existing labels + new labels
	merged := make(map[string]string)
	for k, v := range disk.Labels {
		merged[k] = v
	}
	for k, v := range labels {
		merged[k] = v
	}

	// Enforce GCP's 64-label limit after merge
	if len(merged) > gcpMaxLabels {
		merged = truncateLabels(merged, gcpMaxLabels)
	}

	req := &compute.ZoneSetLabelsRequest{
		LabelFingerprint: disk.LabelFingerprint,
		Labels:           merged,
	}

	op, err := svc.Disks.SetLabels(projectID, zone, diskName, req).Do()
	if err != nil {
		return fmt.Errorf("GCP Disks.SetLabels failed for %s: %w", diskName, err)
	}
	if op.Error != nil {
		return fmt.Errorf("GCP Disks.SetLabels operation error for %s: %v", diskName, op.Error.Errors)
	}

	log.Debug().
		Str("disk", diskName).
		Int("labelCount", len(merged)).
		Msg("[GCP] Batch labels upserted on disk")

	return nil
}

// gcpCreds holds GCP service account credentials from OpenBao.
type gcpCreds struct {
	ClientEmail string
	PrivateKey  string
	ProjectID   string
}

// getGCPCreds retrieves GCP credentials from OpenBao.
func getGCPCreds(ctx context.Context) (*gcpCreds, error) {
	path := csp.BuildSecretPath(ctx, "gcp")
	data, err := csp.ReadOpenBaoSecret(ctx, path)
	if err != nil {
		return nil, err
	}

	clientEmail := csp.GetString(data, "client_email")
	privateKey := csp.GetString(data, "private_key")
	projectID := csp.GetString(data, "project_id")

	if clientEmail == "" || privateKey == "" || projectID == "" {
		return nil, fmt.Errorf("GCP credentials incomplete at %s (need client_email, private_key, project_id)", path)
	}

	return &gcpCreds{
		ClientEmail: clientEmail,
		PrivateKey:  privateKey,
		ProjectID:   projectID,
	}, nil
}

// newComputeService creates a GCP Compute Engine API service using service account credentials.
func newComputeService(ctx context.Context, creds *gcpCreds) (*compute.Service, error) {
	// OpenBao stores the private key with literal "\n" (two chars: backslash + n).
	// Convert to actual newlines for PEM parsing.
	privateKey := strings.ReplaceAll(creds.PrivateKey, `\n`, "\n")

	// Build service account JSON for JWT authentication
	saJSON, err := json.Marshal(map[string]string{
		"type":         "service_account",
		"client_email": creds.ClientEmail,
		"private_key":  privateKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to build GCP service account JSON: %w", err)
	}

	conf, err := google.JWTConfigFromJSON(saJSON, compute.ComputeScope)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCP JWT config: %w", err)
	}

	svc, err := compute.NewService(ctx, option.WithHTTPClient(conf.Client(ctx)))
	if err != nil {
		return nil, fmt.Errorf("failed to create GCP Compute service: %w", err)
	}

	return svc, nil
}
