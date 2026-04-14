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

package model

import "time"

/*
 * Condition-based Status Management
 *
 * Inspired by Kubernetes Conditions pattern for declarative-compatible state management.
 * Conditions are the source of truth; Status is derived from Conditions.
 */

// ConditionType represents the type of a condition observation
type ConditionType string

const (
	// ConditionReady indicates whether the resource itself is usable
	ConditionReady ConditionType = "Ready"
	// ConditionSynced indicates whether the resource is in sync with the CSP/Spider
	ConditionSynced ConditionType = "Synced"
	// ConditionChildrenReady indicates whether all child resources are healthy (e.g., VNet's Subnets)
	ConditionChildrenReady ConditionType = "ChildrenReady"
)

// ConditionStatus represents the status of a condition
type ConditionStatus string

const (
	ConditionTrue    ConditionStatus = "True"
	ConditionFalse   ConditionStatus = "False"
	ConditionUnknown ConditionStatus = "Unknown"
)

// Reason constants for Condition.Reason field (CamelCase, machine-readable)
const (
	// Common reasons for Ready condition
	ReasonCreating         = "Creating"
	ReasonCreationFailed   = "CreationFailed"
	ReasonDeleting         = "Deleting"
	ReasonDeletionFailed   = "DeletionFailed"
	ReasonRegistering      = "Registering"
	ReasonRegisterFailed   = "RegisterFailed"
	ReasonDeregistering    = "Deregistering"
	ReasonDeregisterFailed = "DeregisterFailed"
	ReasonAvailable        = "Available"

	// Reasons for Synced condition
	ReasonResourceNotFound = "ResourceNotFound"
	ReasonSyncCheckFailed  = "SyncCheckFailed"

	// Reasons for ChildrenReady condition
	ReasonNoChildren       = "NoChildren"
	ReasonAllReady         = "AllReady"
	ReasonSubnetFailed     = "SubnetFailed"
	ReasonSubnetInProgress = "SubnetInProgress"
)

// Network resource status constants.
// These are the single source of truth for network resource status values.
// All values are literal strings, intentionally decoupled from MCI status
// constants (mci.go) to prevent unintended side effects when either domain evolves.
const (
	NetworkStatusAvailable     = "Available"
	NetworkStatusCreating      = "Creating"
	NetworkStatusDeleting      = "Deleting"
	NetworkStatusFailed        = "Failed"
	NetworkStatusRegistering   = "Registering"
	NetworkStatusDeregistering = "Deregistering"
	NetworkStatusUnknown       = "Unknown"
)

// Condition represents an observation about a resource's state
type Condition struct {
	Type               ConditionType   `json:"type"`
	Status             ConditionStatus `json:"status"`
	Reason             string          `json:"reason,omitempty"`
	Message            string          `json:"message,omitempty"`
	LastTransitionTime string          `json:"lastTransitionTime,omitempty"`
}

// SetCondition sets or updates a condition in the conditions slice.
// If a condition with the same Type already exists and has a different Status, it is updated
// (including LastTransitionTime). If only Reason/Message changed, those are updated without
// changing LastTransitionTime.
func SetCondition(conditions *[]Condition, condType ConditionType, status ConditionStatus, reason, message string) {
	now := time.Now().UTC().Format(time.RFC3339)
	if conditions == nil {
		return
	}
	for i, c := range *conditions {
		if c.Type == condType {
			if c.Status != status {
				(*conditions)[i].Status = status
				(*conditions)[i].LastTransitionTime = now
			}
			(*conditions)[i].Reason = reason
			(*conditions)[i].Message = message
			return
		}
	}
	// Condition not found, add a new one
	*conditions = append(*conditions, Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
	})
}

// GetCondition returns the condition with the given type, or nil if not found.
func GetCondition(conditions []Condition, condType ConditionType) *Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}

// IsConditionTrue returns true if the condition with the given type has status True.
func IsConditionTrue(conditions []Condition, condType ConditionType) bool {
	c := GetCondition(conditions, condType)
	return c != nil && c.Status == ConditionTrue
}

// DeriveVNetStatus derives the VNet status from its Conditions.
func DeriveVNetStatus(conditions []Condition) string {
	ready := GetCondition(conditions, ConditionReady)
	if ready == nil || ready.Status == ConditionUnknown {
		return NetworkStatusUnknown
	}

	if ready.Status == ConditionFalse {
		switch ready.Reason {
		case ReasonCreating:
			return NetworkStatusCreating
		case ReasonDeleting:
			return NetworkStatusDeleting
		case ReasonRegistering:
			return NetworkStatusRegistering
		case ReasonDeregistering:
			return NetworkStatusDeregistering
		default:
			return NetworkStatusFailed
		}
	}

	// ready.Status == ConditionTrue
	return NetworkStatusAvailable
}

// DeriveSubnetStatus derives the Subnet status from its Conditions.
func DeriveSubnetStatus(conditions []Condition) string {
	ready := GetCondition(conditions, ConditionReady)
	if ready == nil || ready.Status == ConditionUnknown {
		return NetworkStatusUnknown
	}

	if ready.Status == ConditionFalse {
		switch ready.Reason {
		case ReasonCreating:
			return NetworkStatusCreating
		case ReasonDeleting:
			return NetworkStatusDeleting
		case ReasonRegistering:
			return NetworkStatusRegistering
		case ReasonDeregistering:
			return NetworkStatusDeregistering
		default:
			return NetworkStatusFailed
		}
	}

	return NetworkStatusAvailable
}

// DeriveVpnStatus derives the VPN status from its Conditions.
func DeriveVpnStatus(conditions []Condition) string {
	ready := GetCondition(conditions, ConditionReady)
	if ready == nil || ready.Status == ConditionUnknown {
		return NetworkStatusUnknown
	}

	if ready.Status == ConditionFalse {
		switch ready.Reason {
		case ReasonCreating:
			return NetworkStatusCreating
		case ReasonDeleting:
			return NetworkStatusDeleting
		default:
			return NetworkStatusFailed
		}
	}

	return NetworkStatusAvailable
}
