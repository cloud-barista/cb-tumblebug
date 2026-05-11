# Network Status Management

## 1. Overview

This document describes the status (lifecycle) management system for CB-Tumblebug network resources:
**VNet**, **Subnet**, and **VPN**.

The system adopts the **Kubernetes Conditions pattern**, where `Conditions` are the source of truth
and `Status` is derived from them. Each resource carries a `Conditions` array and a `Status` string
field. A `SystemMessage` field provides human-readable error details on failure.

### 1.1. Applicable Resources

| Resource | Model File        | Resource File        | Conditions | Derive Function        |
| -------- | ----------------- | -------------------- | :--------: | ---------------------- |
| VNet     | `model/vnet.go`   | `resource/vnet.go`   |     ✓      | `DeriveVNetStatus()`   |
| Subnet   | `model/subnet.go` | `resource/subnet.go` |     ✓      | `DeriveSubnetStatus()` |
| VPN      | `model/vpn.go`    | `resource/vpn.go`    |     ✓      | `DeriveVpnStatus()`    |

### 1.2. Key Design Decisions

- **Status is derived, not set directly.** All status transitions go through `SetCondition()` + `Derive*Status()`.
- **Failed states are explicit.** When an API call fails, the resource transitions to `Failed` with a `Reason` indicating the failed operation (e.g., `DeletionFailed`). This prevents resources from being stuck in transitional states like `Deleting`.
- **Labels do not carry status.** Following Kubernetes practice, `sys.status` is not stored in Labels. Status is queried from the resource object itself.
- **Status constants are defined in `model/condition.go`** as the single source of truth (`NetworkStatus*` constants). All values are literal strings, intentionally decoupled from Infra status constants (`infra.go`) to prevent cross-domain side effects.

---

## 2. Status Values

### 2.1. Network Status Constants

Defined in `model/condition.go`:

| Constant                     | Value             | VNet | Subnet | VPN | Description                            |
| ---------------------------- | ----------------- | :--: | :----: | :-: | -------------------------------------- |
| `NetworkStatusAvailable`     | `"Available"`     |  ✓   |   ✓    |  ✓  | Resource is ready and usable           |
| `NetworkStatusCreating`      | `"Creating"`      |  ✓   |   ✓    |  ✓  | Create operation in progress           |
| `NetworkStatusDeleting`      | `"Deleting"`      |  ✓   |   ✓    |  ✓  | Delete operation in progress           |
| `NetworkStatusRegistering`   | `"Registering"`   |  ✓   |   ✓    |  —  | Register operation in progress         |
| `NetworkStatusDeregistering` | `"Deregistering"` |  ✓   |   ✓    |  —  | Deregister operation in progress       |
| `NetworkStatusFailed`        | `"Failed"`        |  ✓   |   ✓    |  ✓  | Operation failed; see Condition Reason |
| `NetworkStatusUnknown`       | `"Unknown"`       |  ✓   |   ✓    |  ✓  | No Condition data available            |

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

| Type            | VNet | Subnet | VPN | Description                                               |
| --------------- | :--: | :----: | :-: | --------------------------------------------------------- |
| `Ready`         |  ✓   |   ✓    |  ✓  | Whether the resource itself is usable                     |
| `Synced`        |  ✓   |   ✓    |  ✓  | Whether the resource is in sync with CSP/Spider/Terrarium |
| `ChildrenReady` |  ✓   |   —    |  —  | Whether all child Subnets are healthy                     |

### 3.3. Reason Constants

**Common (Ready condition)**

| Reason             | Used By           | Situation                                                                                                                                     |
| ------------------ | ----------------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| `Creating`         | VNet, Subnet, VPN | Creation in progress                                                                                                                          |
| `CreationFailed`   | VNet, Subnet, VPN | Creation failed                                                                                                                               |
| `Deleting`         | VNet, Subnet, VPN | Deletion in progress                                                                                                                          |
| `DeletionFailed`   | VNet, Subnet, VPN | Deletion failed                                                                                                                               |
| `Registering`      | VNet, Subnet      | Registration in progress                                                                                                                      |
| `RegisterFailed`   | VNet, Subnet      | Registration failed                                                                                                                           |
| `Deregistering`    | VNet, Subnet      | Deregistration in progress                                                                                                                    |
| `DeregisterFailed` | VNet, Subnet      | Deregistration failed                                                                                                                         |
| `Available`        | VNet, Subnet, VPN | Operational and healthy                                                                                                                       |
| `Restored`         | VNet, Subnet, VPN | Status restored to Available by Reconcile after a terminal-failure (e.g., `DeletionFailed`) when the CSP resource is confirmed to still exist |

**Synced condition**

| Reason             | Situation                      |
| ------------------ | ------------------------------ |
| `ResourceNotFound` | Resource does not exist in CSP |
| `SyncCheckFailed`  | Failed to verify sync with CSP |

**ChildrenReady condition (VNet only)**

| Reason             | Situation                                       |
| ------------------ | ----------------------------------------------- |
| `NoChildren`       | No Subnets exist; VNet itself is healthy        |
| `AllReady`         | All Subnets are in Ready state                  |
| `SubnetFailed`     | One or more Subnets are in Failed state         |
| `SubnetInProgress` | One or more Subnets have operations in progress |

---

## 4. State Transitions

### 4.1. VNet State Transitions

```
                         ┌──────────────────────────────────────────┐
                         │                                          │
    ┌──────────┐  OK     │  ┌───────────┐                           │
────▶ Creating  ├────────▶──▶ Available  │                           │
    └────┬─────┘         │  └─────┬─────┘                           │
         │ Fail           │        │                                 │
         ▼               │        │ From Available:                  │
    ┌─────────┐  Reconcile│        │                                 │
    │ Failed  ├──────────▶│        ├── Delete ──▶ Deleting ──OK──▶ (removed)
    │(Reason: │          │        │                 │ Fail           │
    │Creating)│          │        │                 ▼               │
    └─────────┘          │        │           ┌──────────┐          │
                         │        │           │ Failed   │─Reconcile▶
                         │        │           │(Deleting)│  /Force  │
                         │        │           └──────────┘          │
                         │        │                                 │
                         │        └── Deregister ──▶ Deregistering   │
                         │                              │ OK→(removed)
                         │                              │ Fail       │
                         │                              ▼           │
                         │                        ┌──────────────┐  │
                         │                        │   Failed     │──▶
                         │                        │(Deregistering)│  │
                         │                        └──────────────┘  │
                         │                                          │
    ┌─────────────┐ OK   │                                          │
────▶ Registering  ├─────▶──────────────────────────────────────────┘
    └──────┬──────┘
           │ Fail
           ▼
    ┌─────────────┐
    │   Failed    │──Reconcile──▶ (cleanup)
    │(Registering)│
    └─────────────┘
```

### 4.2. Subnet State Transitions

```
    ┌──────────┐  OK    ┌───────────┐
────▶ Creating  ├───────▶│ Available  │
    └────┬─────┘        └─────┬─────┘
         │ Fail                │
         ▼                    ├── Delete ──▶ Deleting
    ┌───────────┐             │                  │ OK → (removed)
    │  Failed   │             │                  │ Fail
    │(Creating) │──Reconcile──▶                  ▼
    └───────────┘  (cleanup)  │            ┌──────────┐
                              │            │ Failed   │
                              │            │(Deleting)│──Reconcile/Force
                              │            └──────────┘     ──▶ (cleanup)
                              │
                              └── Deregister ──▶ Deregistering
                                                     │ OK → (removed)
                                                     │ Fail → Failed

    ┌─────────────┐  OK    ┌───────────┐
────▶ Registering  ├───────▶│ Available  │
    └──────┬──────┘        └───────────┘
           │ Fail
           ▼
    ┌───────────┐
    │  Failed   │──Reconcile──▶ (cleanup)
    └───────────┘
```

### 4.3. VPN State Transitions

VPN resources are created and deleted via the Terrarium API.
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

**VPN failure points handled:**

| Function              | Failure Point                        | Condition Set                  |
| --------------------- | ------------------------------------ | ------------------------------ |
| `CreateSiteToSiteVPN` | Terrarium issue fails                | `Ready=False / CreationFailed` |
| `CreateSiteToSiteVPN` | VPN creation API fails (after retry) | `Ready=False / CreationFailed` |
| `DeleteSiteToSiteVPN` | Terrarium info retrieval fails       | `Ready=False / DeletionFailed` |
| `DeleteSiteToSiteVPN` | VPN deletion API fails (after retry) | `Ready=False / DeletionFailed` |

---

## 5. Conditions Transitions by Operation

### 5.1. Create

| Phase   | Ready                      | Synced               | ChildrenReady (VNet)  | Status      |
| ------- | -------------------------- | -------------------- | --------------------- | ----------- |
| Start   | `False` / `Creating`       | `False` / `Creating` | —                     | `Creating`  |
| Success | `True` / `Available`       | `True` / `Available` | `True` / `NoChildren` | `Available` |
| Failure | `False` / `CreationFailed` | unchanged            | unchanged             | `Failed`    |

### 5.2. Delete

| Phase   | Ready                      | Synced    | ChildrenReady (VNet) | Status     |
| ------- | -------------------------- | --------- | -------------------- | ---------- |
| Start   | `False` / `Deleting`       | unchanged | unchanged            | `Deleting` |
| Success | (resource removed)         |           |                      |            |
| Failure | `False` / `DeletionFailed` | unchanged | unchanged            | `Failed`   |

> **Post-deletion verification**
>
> - **In most cases polling does not trigger** — the first GET returns 404 immediately and the resource is confirmed deleted on the first attempt.
> - Polling handles CSP anomalies (Spider-controlled resources only):
>   - GCP may return HTTP 200 + `Result:false` for an already-deleted resource;
>   - some CSPs report success while deletion is still in-flight.
> - All Spider-controlled resources follow a **"trust DELETE" policy**: Spider's DELETE success is authoritative. GET visibility after DELETE reflects CSP async deletion or eventual consistency, not a real failure. Polling failures are logged as warnings only.
> - **VPN is exempt from polling**: Terrarium uses OpenTofu (declarative) and already retries deletion once on failure. A successful DELETE response guarantees `terraform destroy` completed.

### 5.3. Register (VNet/Subnet only)

| Phase   | Ready                      | Synced                  | ChildrenReady (VNet) | Status        |
| ------- | -------------------------- | ----------------------- | -------------------- | ------------- |
| Start   | `False` / `Registering`    | `False` / `Registering` | —                    | `Registering` |
| Success | `True` / `Available`       | `True` / `Available`    | depends on Subnets   | `Available`   |
| Failure | `False` / `RegisterFailed` | unchanged               | unchanged            | `Failed`      |

### 5.4. Deregister (VNet/Subnet only)

| Phase   | Ready                        | Synced    | ChildrenReady (VNet) | Status          |
| ------- | ---------------------------- | --------- | -------------------- | --------------- |
| Start   | `False` / `Deregistering`    | unchanged | unchanged            | `Deregistering` |
| Success | (resource removed)           |           |                      |                 |
| Failure | `False` / `DeregisterFailed` | unchanged | unchanged            | `Failed`        |

### 5.5. Reconcile

`Reconcile` reconciles the gap between Tumblebug metadata (Desired) and the actual
CSP/Spider/Terrarium resource (Actual). It is the single corrective entry point used
when the metadata and the real resource have drifted apart — typically because a
previous operation failed, was interrupted, or was bypassed.

| Scenario                            | Metadata                                       | CSP/Terrarium | Action               | Result                                                                 |
| ----------------------------------- | ---------------------------------------------- | ------------- | -------------------- | ---------------------------------------------------------------------- |
| Healthy                             | exists / `Available`                           | exists        | `NoActionNeeded`     | unchanged                                                              |
| Orphaned metadata                   | exists                                         | missing (404) | `MetadataRemoved`    | metadata + label deleted                                               |
| **Stuck in terminal-failure state** | `Failed(DeletionFailed` or `DeregisterFailed)` | exists        | **`StatusRestored`** | `Ready=True / Restored`, `Synced=True / Available`, `Status=Available` |
| Spider/Terrarium transient outage   | any                                            | 5xx / network | (none)               | error returned, status unchanged                                       |

**Restore guard (conservative):** Status is restored to `Available` only when **all** of
the following hold:

1. CSP/Spider/Terrarium GET (or HEAD) returns success — the resource is confirmed alive.
2. `ConditionReady.Status == False`.
3. `ConditionReady.Reason ∈ { DeletionFailed, DeregisterFailed }` — only terminal-failure
   states are restored.

In-flight states (`Creating`, `Deleting`, `Registering`, `Deregistering`) and
`CreationFailed` are intentionally **excluded** to avoid masking concurrent operations
or partially-created resources. For `CreationFailed`, users should explicitly delete and
recreate the resource.

When a child Subnet's status is restored, the parent VNet's `ChildrenReady` Condition is
recomputed and persisted alongside.

**Auto-trigger after delete failure (self-healing):** When `DeleteVNet`, `DeleteSubnet`,
`DeleteSiteToSiteVPN`, or `DeleteObjectStorage` fails and records the resource as
`Failed(DeletionFailed)`, Reconcile is automatically invoked **once** for the affected
resource immediately after the failure is persisted. The original delete error is still
returned to the caller unchanged; the auto-reconcile is opportunistic and any error from
it is logged at WARN level only. This means:

- If the failure was caused by a transient dependency that was removed before the next
  user action, the next read will already show `Available` (no manual recovery needed).
- If the CSP resource is in fact gone, the orphaned metadata is cleaned up automatically.
- If the failure is persistent (e.g., a long-lived dependency), the resource remains
  `Failed(DeletionFailed)` and the user can call Reconcile (or retry the delete after
  removing the dependency).

The auto-trigger performs at most one attempt per delete failure — there is no internal
retry loop or background scheduler.

---

## 6. Status Derivation

Status is never set directly. It is always computed by a `Derive*Status()` function after Conditions are updated.

### 6.1. DeriveVNetStatus

```go
func DeriveVNetStatus(conditions []Condition) string
```

| Ready.Status     | Ready.Reason    | Result          |
| ---------------- | --------------- | --------------- |
| `Unknown` or nil | —               | `Unknown`       |
| `False`          | `Creating`      | `Creating`      |
| `False`          | `Deleting`      | `Deleting`      |
| `False`          | `Registering`   | `Registering`   |
| `False`          | `Deregistering` | `Deregistering` |
| `False`          | (other)         | `Failed`        |
| `True`           | —               | `Available`     |

### 6.2. DeriveSubnetStatus

```go
func DeriveSubnetStatus(conditions []Condition) string
```

Same mapping as `DeriveVNetStatus`.

### 6.3. DeriveVpnStatus

```go
func DeriveVpnStatus(conditions []Condition) string
```

| Ready.Status     | Ready.Reason | Result      |
| ---------------- | ------------ | ----------- |
| `Unknown` or nil | —            | `Unknown`   |
| `False`          | `Creating`   | `Creating`  |
| `False`          | `Deleting`   | `Deleting`  |
| `False`          | (other)      | `Failed`    |
| `True`           | —            | `Available` |

VPN does not support `Registering` or `Deregistering` states.

---

## 7. Subnet Impact on VNet

When a Subnet changes state, the parent VNet's `ChildrenReady` Condition is updated.
The VNet `Status` itself remains `Available` regardless of Subnet presence or count.

| Subnet Event                   | VNet ChildrenReady           | VNet Status             |
| ------------------------------ | ---------------------------- | ----------------------- |
| Subnet added successfully      | `True` / `AllReady`          | `Available` (unchanged) |
| Subnet deleted (others remain) | `True` / `AllReady`          | `Available` (unchanged) |
| Last Subnet deleted            | `True` / `NoChildren`        | `Available` (unchanged) |
| Subnet deletion failed         | `False` / `SubnetFailed`     | `Available` (unchanged) |
| Subnet operation in progress   | `False` / `SubnetInProgress` | `Available` (unchanged) |

**Principle**: A Subnet failure does not change the VNet's `Ready` Condition or make its Status `Failed`.
The VNet itself remains operational. Subnet health is tracked solely via the `ChildrenReady` Condition.

---

## 8. Implementation Files

| File                          | Contents                                                                                                                                                                                                            |
| ----------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `src/core/model/condition.go` | `Condition` struct, `ConditionType`/`ConditionStatus` types, `NetworkStatus*` constants, `SetCondition()`, `GetCondition()`, `IsConditionTrue()`, `DeriveVNetStatus()`, `DeriveSubnetStatus()`, `DeriveVpnStatus()` |
| `src/core/model/vnet.go`      | `VNetInfo.Conditions []Condition`, `VNetInfo.SystemMessage string`                                                                                                                                                  |
| `src/core/model/subnet.go`    | `SubnetInfo.Conditions []Condition`, `SubnetInfo.SystemMessage string`                                                                                                                                              |
| `src/core/model/vpn.go`       | `VpnInfo.Conditions []Condition`, `VpnInfo.SystemMessage string`                                                                                                                                                    |
| `src/core/resource/vnet.go`   | Conditions transitions for CreateVNet, DeleteVNet, RegisterVNet, DeregisterVNet, ReconcileVNet                                                                                                                      |
| `src/core/resource/subnet.go` | Conditions transitions for CreateSubnet, DeleteSubnet, RegisterSubnet, DeregisterSubnet, ReconcileSubnet                                                                                                            |
| `src/core/resource/vpn.go`    | Conditions transitions for CreateSiteToSiteVPN, DeleteSiteToSiteVPN                                                                                                                                                 |
