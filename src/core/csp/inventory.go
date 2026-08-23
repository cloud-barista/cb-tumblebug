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
	"context"
	"regexp"
	"strings"
	"sync"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
)

// VMRecord is a CSP-side VM as observed directly through the CSP SDK (never via CB-Spider metadata).
type VMRecord struct {
	CspResourceId string            `json:"cspResourceId"`
	Name          string            `json:"name"`
	Status        string            `json:"status"` // TB status string
	Zone          string            `json:"zone,omitempty"`
	PublicIP      string            `json:"publicIp,omitempty"`
	Tags          map[string]string `json:"tags,omitempty"`
}

// ResidualResource is a VM-adjacent resource left behind after a VM is gone (NIC, public IP, disk, ...).
type ResidualResource struct {
	Type   string `json:"type"` // nic, publicIp, disk, eni, eip, volume, floatingIp
	Id     string `json:"id"`
	Name   string `json:"name,omitempty"`
	Zone   string `json:"zone,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// Key identifies a residual across list/delete calls.
func (r ResidualResource) Key() string { return r.Type + ":" + r.Id }

// ListVMsFunc lists every VM in a region (paginated internally). zone is only used by zone-scoped CSPs (KT).
type ListVMsFunc func(ctx context.Context, region, zone string) ([]VMRecord, error)

// ListResidualsFunc lists TB-managed residual resources no longer attached to a VM.
type ListResidualsFunc func(ctx context.Context, region, zone string) ([]ResidualResource, error)

// DeleteResidualsFunc deletes residuals; the result is keyed by ResidualResource.Key() (nil error = deleted).
type DeleteResidualsFunc func(ctx context.Context, region, zone string, items []ResidualResource) map[string]error

// InventoryHandlers groups the direct-SDK "truth" functions for a CSP.
type InventoryHandlers struct {
	ListVMs         ListVMsFunc
	ListResiduals   ListResidualsFunc
	DeleteResiduals DeleteResidualsFunc
}

var (
	inventoryMu       sync.RWMutex
	inventoryHandlers = make(map[string]InventoryHandlers)
)

// RegisterInventoryHandlers registers direct-SDK inventory functions for a CSP (called from init()).
func RegisterInventoryHandlers(provider string, h InventoryHandlers) {
	inventoryMu.Lock()
	defer inventoryMu.Unlock()
	inventoryHandlers[strings.ToLower(provider)] = h
}

// GetInventoryHandlers returns the registered inventory functions for a provider.
func GetInventoryHandlers(provider string) (InventoryHandlers, bool) {
	inventoryMu.RLock()
	defer inventoryMu.RUnlock()
	h, ok := inventoryHandlers[strings.ToLower(provider)]
	return h, ok
}

// ManagerTagValue is the value TB writes to the sys.manager tag on CSP resources.
const ManagerTagValue = "cb-tumblebug"

var tbUidPattern = regexp.MustCompile(`^tb[a-z0-9]{18}`)

// IsTBUid reports whether a CSP resource name starts with a TB-generated uid (tb + 18 base32 chars).
func IsTBUid(name string) bool {
	return tbUidPattern.MatchString(strings.ToLower(name))
}

// IsManagedByTB reports whether a name/tag set identifies a resource created by TB.
// Tag keys are compared case-insensitively, with '.', '-' and '_' treated alike.
func IsManagedByTB(name string, tags map[string]string) bool {
	if IsTBUid(name) {
		return true
	}
	for k, v := range tags {
		nk := strings.NewReplacer("-", ".", "_", ".").Replace(strings.ToLower(k))
		if nk == model.LabelManager && strings.EqualFold(v, ManagerTagValue) {
			return true
		}
	}
	return false
}

// NormalizeTagKey maps CSP-sanitized label keys (sys_uid, sys-uid) back to TB's sys.uid form.
func NormalizeTagKey(k string) string {
	return strings.NewReplacer("-", ".", "_", ".").Replace(strings.ToLower(k))
}
