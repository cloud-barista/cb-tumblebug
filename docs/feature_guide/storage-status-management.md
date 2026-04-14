# Storage Status Management

## 1. Overview

This document describes the status (lifecycle) management system for CB-Tumblebug storage resources:
**Object Storage**.

The system adopts the **Kubernetes Conditions pattern**, where `Conditions` are the source of truth
and `Status` is derived from them. Each resource carries a `Conditions` array and a `Status` string
field. A `SystemMessage` field provides human-readable error details on failure.

This is the same pattern used by [network resources](network-status-management.md) (VNet, Subnet, VPN).
Status constants share a common `ResourceStatus*` base defined in `model/condition.go`, with
domain-specific aliases (`StorageStatus*`) for storage resources.

### 1.1. Applicable Resources

| Resource       | Model File               | Resource File               | Conditions | Derive Function               |
| -------------- | ------------------------ | --------------------------- | :--------: | ----------------------------- |
| Object Storage | `model/objectStorage.go` | `resource/objectStorage.go` |     ✓      | `DeriveObjectStorageStatus()` |

### 1.2. Key Design Decisions

- **Status is derived, not set directly.** All status transitions go through `SetCondition()` + `DeriveObjectStorageStatus()`.
- **Failed states are explicit.** When an API call fails, the resource transitions to `Failed` with a `Reason` indicating the failed operation (e.g., `CreationFailed`, `DeletionFailed`). This prevents resources from being stuck in transitional states like `Creating` or `Deleting`.
- **Labels do not carry status.** Following Kubernetes practice, `sys.status` is not stored in Labels. Status is queried from the resource object itself.
- **Status constants use domain aliases.** `StorageStatus*` constants are aliases of the common `ResourceStatus*` base, defined in `model/condition.go`. This allows each domain to extend independently while sharing a common source of truth.
- **No Register/Deregister.** Unlike network resources, storage resources do not support Register/Deregister operations. The `StorageStatus*` set intentionally excludes `Registering` and `Deregistering` states.

---

## 2. Status Values

### 2.1. Storage Status Constants

Defined in `model/condition.go` as aliases of `ResourceStatus*`:

| Constant                 | Value         | Object Storage | Description                            |
| ------------------------ | ------------- | :------------: | -------------------------------------- |
| `StorageStatusAvailable` | `"Available"` |       ✓        | Resource is ready and usable           |
| `StorageStatusCreating`  | `"Creating"`  |       ✓        | Create operation in progress           |
| `StorageStatusDeleting`  | `"Deleting"`  |       ✓        | Delete operation in progress           |
| `StorageStatusFailed`    | `"Failed"`    |       ✓        | Operation failed; see Condition Reason |
| `StorageStatusUnknown`   | `"Unknown"`   |       ✓        | No Condition data available            |

---

## 3. Conditions

### 3.1. Condition Structure

```go
type Condition struct {
    Type               ConditionType   `json:"type"`
    Status             ConditionStatus `json:"status"`             // "True", "False", "Unknown"
    Reason             string          `json:"reason,omitempty"`   // Machine-readable CamelCase
    Message            string          `json:"message,omitempty"`  // Human-readable detail
    LastTransitionTime string          `json:"lastTransitionTime,omitempty"`
}
```

### 3.2. Condition Types

| Type     | Object Storage | Description                                     |
| -------- | :------------: | ----------------------------------------------- |
| `Ready`  |       ✓        | Whether the resource itself is usable           |
| `Synced` |       ✓        | Whether the resource is in sync with CSP/Spider |

Object Storage does not use the `ChildrenReady` condition (no child resources).

### 3.3. Reason Constants

**Ready condition**

| Reason           | Used By        | Situation            |
| ---------------- | -------------- | -------------------- |
| `Creating`       | Object Storage | Creation in progress |
| `CreationFailed` | Object Storage | Creation failed      |
| `Deleting`       | Object Storage | Deletion in progress |
| `DeletionFailed` | Object Storage | Deletion failed      |
| `Available`      | Object Storage | Operational          |

**Synced condition**

| Reason      | Situation                    |
| ----------- | ---------------------------- |
| `Creating`  | Sync not yet established     |
| `Available` | Resource is in sync with CSP |

---

## 4. State Transitions

### 4.1. Object Storage State Transitions

Object Storage resources are created and deleted via the Spider S3-compatible API.
They do not support Register/Deregister operations.

```
    ┌──────────┐  OK    ┌───────────┐
────▶ Creating  ├───────▶│ Available  │
    └────┬─────┘        └─────┬─────┘
         │ Fail                │
         ▼                    └── Delete ──▶ Deleting
    ┌───────────┐                                │ OK → (removed)
    │  Failed   │                                │ Fail
    │(Creating) │                                ▼
    └───────────┘                          ┌──────────┐
                                           │ Failed   │
                                           │(Deleting)│
                                           └──────────┘
```

**Object Storage failure points handled:**

| Function              | Failure Point                               | Condition Set                  |
| --------------------- | ------------------------------------------- | ------------------------------ |
| `CreateObjectStorage` | Spider PUT retries exhausted (409 Conflict) | `Ready=False / CreationFailed` |
| `CreateObjectStorage` | Spider PUT fails (non-conflict error)       | `Ready=False / CreationFailed` |
| `CreateObjectStorage` | Spider GET after PUT fails                  | `Ready=False / CreationFailed` |
| `DeleteObjectStorage` | Spider DELETE retries exhausted             | `Ready=False / DeletionFailed` |

---

## 5. Conditions Transitions by Operation

### 5.1. Create

| Phase   | Ready                      | Synced               | Status      |
| ------- | -------------------------- | -------------------- | ----------- |
| Start   | `False` / `Creating`       | `False` / `Creating` | `Creating`  |
| Success | `True` / `Available`       | `True` / `Available` | `Available` |
| Failure | `False` / `CreationFailed` | unchanged            | `Failed`    |

On failure, `SystemMessage` is set with the error detail and the resource is persisted to kvstore
so that the Failed state is durable.

### 5.2. Delete

| Phase   | Ready                      | Synced    | Status     |
| ------- | -------------------------- | --------- | ---------- |
| Start   | `False` / `Deleting`       | unchanged | `Deleting` |
| Success | (resource removed)         |           |            |
| Failure | `False` / `DeletionFailed` | unchanged | `Failed`   |

On failure, `SystemMessage` is set with the error detail and the resource is persisted to kvstore.

---

## 6. Status Derivation

Status is never set directly. It is always computed by `DeriveObjectStorageStatus()` after Conditions are updated.

### 6.1. DeriveObjectStorageStatus

```go
func DeriveObjectStorageStatus(conditions []Condition) string
```

| Ready.Status     | Ready.Reason | Result      |
| ---------------- | ------------ | ----------- |
| `Unknown` or nil | —            | `Unknown`   |
| `False`          | `Creating`   | `Creating`  |
| `False`          | `Deleting`   | `Deleting`  |
| `False`          | (other)      | `Failed`    |
| `True`           | —            | `Available` |

Object Storage does not support `Registering` or `Deregistering` states.

---

## 7. Relationship to Network Status Management

Storage and network resources share the same Conditions infrastructure:

| Shared Component   | Location             | Description                                              |
| ------------------ | -------------------- | -------------------------------------------------------- |
| `Condition` struct | `model/condition.go` | Same struct for all resource domains                     |
| `SetCondition()`   | `model/condition.go` | Same function for updating conditions                    |
| `GetCondition()`   | `model/condition.go` | Same function for reading conditions                     |
| `ResourceStatus*`  | `model/condition.go` | Common base constants shared by all domains              |
| Reason constants   | `model/condition.go` | `Creating`, `CreationFailed`, etc. shared across domains |

Domain-specific differences:

| Aspect          | Network (`NetworkStatus*`)              | Storage (`StorageStatus*`) |
| --------------- | --------------------------------------- | -------------------------- |
| Status values   | 7 (includes Registering, Deregistering) | 5 (no Register/Deregister) |
| Condition types | Ready, Synced, ChildrenReady            | Ready, Synced              |
| Operations      | Create, Delete, Register, Deregister    | Create, Delete             |

---

## 8. Implementation Files

| File                                 | Contents                                                                                                      |
| ------------------------------------ | ------------------------------------------------------------------------------------------------------------- |
| `src/core/model/condition.go`        | `Condition` struct, `ResourceStatus*` base constants, `StorageStatus*` aliases, `DeriveObjectStorageStatus()` |
| `src/core/model/objectStorage.go`    | `ObjectStorageInfo.Conditions []Condition`, `ObjectStorageInfo.SystemMessage string`                          |
| `src/core/resource/objectStorage.go` | Conditions transitions for `CreateObjectStorage`, `DeleteObjectStorage`                                       |
