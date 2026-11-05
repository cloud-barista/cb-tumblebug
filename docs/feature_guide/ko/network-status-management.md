# 네트워크 상태 관리 (Network Status Management)

## 1. 개요

이 문서는 CB-Tumblebug 네트워크 리소스인 **VNet**, **Subnet**, **VPN**의 상태(수명주기) 관리 시스템을 설명합니다.

이 시스템은 `Conditions`를 단일 진실 공급원(Source of truth)으로 삼고 그로부터 `Status`를 도출(Derive)하는 **Kubernetes Conditions 패턴**을 채택하고 있습니다. 각 리소스는 `Conditions` 배열과 `Status` 문자열 필드를 가집니다. 실패 시에는 `SystemMessage` 필드가 사람이 읽을 수 있는 상세 에러 정보를 제공합니다.

### 1.1. 적용 대상 리소스

| 리소스 | 모델 파일        | 리소스 파일        | Conditions | 상태 도출 함수 (Derive Function) |
| -------- | ----------------- | -------------------- | :--------: | ---------------------- |
| VNet     | `model/vnet.go`   | `resource/vnet.go`   |     ✓      | `DeriveVNetStatus()`   |
| Subnet   | `model/subnet.go` | `resource/subnet.go` |     ✓      | `DeriveSubnetStatus()` |
| VPN      | `model/vpn.go`    | `resource/vpn.go`    |     ✓      | `DeriveVpnStatus()`    |

### 1.2. 주요 설계 결정 사항

- **상태(Status)는 직접 설정되지 않고 도출됩니다.** 모든 상태 전이는 `SetCondition()` + `Derive*Status()`를 거칩니다.
- **실패 상태(Failed state)는 명시적입니다.** API 호출이 실패하면 리소스는 실패한 작업을 나타내는 `Reason`(예: `DeletionFailed`)과 함께 `Failed`로 전이됩니다. 이를 통해 리소스가 `Deleting`과 같은 전이 상태에 영구적으로 갇히는 것을 방지합니다.
- **Label에는 상태를 저장하지 않습니다.** Kubernetes 관례에 따라 `sys.status`는 Label에 저장하지 않으며, 상태는 리소스 객체 자체에서 조회합니다.
- **상태 상수는 `model/condition.go`에 정의됩니다.** 단일 진실 공급원으로서 `NetworkStatus*` 상수가 정의되어 있으며, 모든 값은 리터럴 문자열입니다. 도메인 간 부작용(Side effect)을 방지하기 위해 의도적으로 Infra 상태 상수(`infra.go`)와 분리되어 있습니다.

---

## 2. 상태 값 (Status Values)

### 2.1. 네트워크 상태 상수

`model/condition.go`에 정의됨:

| 상수                     | 값             | VNet | Subnet | VPN | 설명                            |
| ---------------------------- | ----------------- | :--: | :----: | :-: | -------------------------------------- |
| `NetworkStatusAvailable`     | `"Available"`     |  ✓   |   ✓    |  ✓  | 리소스가 준비되어 사용 가능한 상태           |
| `NetworkStatusCreating`      | `"Creating"`      |  ✓   |   ✓    |  ✓  | 생성 작업 진행 중           |
| `NetworkStatusDeleting`      | `"Deleting"`      |  ✓   |   ✓    |  ✓  | 삭제 작업 진행 중           |
| `NetworkStatusRegistering`   | `"Registering"`   |  ✓   |   ✓    |  —  | 등록(Register) 작업 진행 중         |
| `NetworkStatusDeregistering` | `"Deregistering"` |  ✓   |   ✓    |  —  | 등록 해제(Deregister) 작업 진행 중       |
| `NetworkStatusFailed`        | `"Failed"`        |  ✓   |   ✓    |  ✓  | 작업 실패; Condition의 Reason 참조 |
| `NetworkStatusUnknown`       | `"Unknown"`       |  ✓   |   ✓    |  ✓  | Condition 데이터를 사용할 수 없음            |

---

## 3. Conditions

### 3.1. Condition 구조체

```go
type Condition struct {
    Type               ConditionType   `json:"type"`
    Status             ConditionStatus `json:"status"`             // "True", "False", "Unknown"
    Reason             string          `json:"reason,omitempty"`   // Machine-readable CamelCase
    Message            string          `json:"message,omitempty"`  // Human-readable detail
    LastTransitionTime string          `json:"lastTransitionTime,omitempty"`
}
```

### 3.2. Condition 유형 (Condition Types)

| 유형            | VNet | Subnet | VPN | 설명                                               |
| --------------- | :--: | :----: | :-: | --------------------------------------------------------- |
| `Ready`         |  ✓   |   ✓    |  ✓  | 리소스 자체를 사용할 수 있는지 여부                     |
| `Synced`        |  ✓   |   ✓    |  ✓  | 리소스가 CSP/Spider/Terrarium과 동기화되어 있는지 여부 |
| `ChildrenReady` |  ✓   |   —    |  —  | 모든 하위 Subnet이 정상(Healthy) 상태인지 여부                     |

### 3.3. Reason 상수

**공통 (Ready 조건)**

| Reason             | 사용 리소스           | 상황                                                                                                                                     |
| ------------------ | ----------------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| `Creating`         | VNet, Subnet, VPN | 생성 진행 중                                                                                                                          |
| `CreationFailed`   | VNet, Subnet, VPN | 생성 실패                                                                                                                               |
| `Deleting`         | VNet, Subnet, VPN | 삭제 진행 중                                                                                                                          |
| `DeletionFailed`   | VNet, Subnet, VPN | 삭제 실패                                                                                                                               |
| `Registering`      | VNet, Subnet      | 등록 진행 중                                                                                                                      |
| `RegisterFailed`   | VNet, Subnet      | 등록 실패                                                                                                                           |
| `Deregistering`    | VNet, Subnet      | 등록 해제 진행 중                                                                                                                    |
| `DeregisterFailed` | VNet, Subnet      | 등록 해제 실패                                                                                                                         |
| `Available`        | VNet, Subnet, VPN | 정상 작동 및 사용 가능 상태                                                                                                                       |
| `Restored`         | VNet, Subnet, VPN | 종료 실패 상태(예: `DeletionFailed`) 이후 CSP 리소스가 여전히 존재하는 것으로 확인되어 Reconcile에 의해 Available로 상태가 복원됨 |

**Synced 조건**

| Reason             | 상황                      |
| ------------------ | ------------------------------ |
| `ResourceNotFound` | CSP에 리소스가 존재하지 않음 |
| `SyncCheckFailed`  | CSP와의 동기화 상태 확인 실패 |

**ChildrenReady 조건 (VNet 전용)**

| Reason             | 상황                                       |
| ------------------ | ----------------------------------------------- |
| `NoChildren`       | Subnet이 존재하지 않음; VNet 자체는 정상 상태        |
| `AllReady`         | 모든 Subnet이 Ready 상태임                  |
| `SubnetFailed`     | 하나 이상의 Subnet이 Failed 상태임         |
| `SubnetInProgress` | 하나 이상의 Subnet에서 작업이 진행 중임 |

---

## 4. 상태 전이 (State Transitions)

### 4.1. VNet 상태 전이

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

### 4.2. Subnet 상태 전이

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

### 4.3. VPN 상태 전이

VPN 리소스는 Terrarium API를 통해 생성 및 삭제됩니다.
Register/Deregister 작업은 지원하지 않습니다.

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

**처리되는 VPN 실패 지점:**

| 함수              | 실패 지점                        | 설정되는 Condition                  |
| --------------------- | ------------------------------------ | ------------------------------ |
| `CreateSiteToSiteVPN` | Terrarium 발급(Issue) 실패                | `Ready=False / CreationFailed` |
| `CreateSiteToSiteVPN` | VPN 생성 API 실패 (재시도 후) | `Ready=False / CreationFailed` |
| `DeleteSiteToSiteVPN` | Terrarium 정보 조회 실패       | `Ready=False / DeletionFailed` |
| `DeleteSiteToSiteVPN` | VPN 삭제 API 실패 (재시도 후) | `Ready=False / DeletionFailed` |

---

## 5. 작업별 Conditions 전이

### 5.1. 생성 (Create)

| 단계   | Ready                      | Synced               | ChildrenReady (VNet)  | Status      |
| ------- | -------------------------- | -------------------- | --------------------- | ----------- |
| 시작   | `False` / `Creating`       | `False` / `Creating` | —                     | `Creating`  |
| 성공 | `True` / `Available`       | `True` / `Available` | `True` / `NoChildren` | `Available` |
| 실패 | `False` / `CreationFailed` | 변경 없음            | 변경 없음             | `Failed`    |

### 5.2. 삭제 (Delete)

| 단계   | Ready                      | Synced    | ChildrenReady (VNet) | Status     |
| ------- | -------------------------- | --------- | -------------------- | ---------- |
| 시작   | `False` / `Deleting`       | 변경 없음 | 변경 없음            | `Deleting` |
| 성공 | (리소스 제거됨)         |           |                      |            |
| 실패 | `False` / `DeletionFailed` | 변경 없음 | 변경 없음            | `Failed`   |

> **삭제 후 검증 (Post-deletion verification)**
>
> - **대부분의 경우 폴링이 트리거되지 않습니다** — 첫 번째 GET 요청이 즉시 404를 반환하여 첫 시도에 리소스 삭제가 확인됩니다.
> - 폴링은 CSP의 비정상적인 동작을 처리합니다 (Spider 제어 리소스에 한함):
>   - GCP는 이미 삭제된 리소스에 대해 HTTP 200 + `Result:false`를 반환할 수 있습니다.
>   - 일부 CSP는 삭제가 아직 진행 중인 상태에서 성공을 보고합니다.
> - 모든 Spider 제어 리소스는 **"DELETE 신뢰(trust DELETE)" 정책**을 따릅니다. 즉, Spider의 DELETE 성공 응답을 권위 있는 결과로 간주합니다. DELETE 이후에도 GET으로 리소스가 조회되는 현상은 실제 실패가 아니라 CSP의 비동기 삭제 또는 결과적 일관성(Eventual consistency)을 반영한 것입니다. 따라서 폴링 실패는 경고(Warning)로만 기록됩니다.
> - **VPN은 폴링 대상에서 제외됩니다**: Terrarium은 선언형 도구인 OpenTofu를 사용하며 실패 시 이미 삭제를 한 번 재시도합니다. 성공적인 DELETE 응답은 `terraform destroy`가 완료되었음을 보장합니다.

### 5.3. 등록 (Register - VNet/Subnet 전용)

| 단계   | Ready                      | Synced                  | ChildrenReady (VNet) | Status        |
| ------- | -------------------------- | ----------------------- | -------------------- | ------------- |
| 시작   | `False` / `Registering`    | `False` / `Registering` | —                    | `Registering` |
| 성공 | `True` / `Available`       | `True` / `Available`    | Subnet 상태에 따름   | `Available`   |
| 실패 | `False` / `RegisterFailed` | 변경 없음               | 변경 없음            | `Failed`      |

### 5.4. 등록 해제 (Deregister - VNet/Subnet 전용)

| 단계   | Ready                        | Synced    | ChildrenReady (VNet) | Status          |
| ------- | ---------------------------- | --------- | -------------------- | --------------- |
| 시작   | `False` / `Deregistering`    | 변경 없음 | 변경 없음            | `Deregistering` |
| 성공 | (리소스 제거됨)           |           |                      |                 |
| 실패 | `False` / `DeregisterFailed` | 변경 없음 | 변경 없음            | `Failed`        |

### 5.5. 조정 (Reconcile)

`Reconcile`은 Tumblebug 메타데이터(Desired)와 실제 CSP/Spider/Terrarium 리소스(Actual) 간의 차이를 조정합니다. 현재 리소스별로 동작이 조금씩 다릅니다:

- **VNet 및 Subnet**은 비파괴적(Non-destructive) `Reconcile`/`Prune` 패턴을 따릅니다(전체 설계는 `docs/feature_guide/resource-reconciliation.md` 참조). `VNetReconciler`(`src/core/reconcile/vnetReconcile.go`)는 공유 헬퍼인 `GetResourceSyncState`를 통해 상태를 진단하고, `SetCondition`을 통해 `Ready`/`Synced`를 직접 설정한 다음, `DeriveVNetStatus`를 통해 `Status`를 재계산합니다. 즉, **절대로 메타데이터를 삭제하지 않습니다**. 고아 메타데이터는 별도의 명시적인 `POST /ns/{nsId}/resources/vNet/reconcile/prune` 호출(`PruneVNets`)을 통해서만 제거됩니다.
- **VPN**은 아직 이 패턴으로 마이그레이션되지 않았습니다. `ReconcileSiteToSiteVPN`(`src/core/resource/vpn.go`)은 아래 표와 같이 CSP/Terrarium 리소스가 사라진 것을 확인하면 즉시 메타데이터를 삭제합니다.

| 시나리오                            | 메타데이터                                       | CSP/Terrarium | 수행 액션                                | 결과                                                                             |
| ----------------------------------- | ---------------------------------------------- | ------------- | ---------------------------------------- | ------------------------------------------------------------------------------------ |
| 정상 (Healthy)                             | 존재함 / `Available`                           | 존재함        | `NoActionNeeded`                        | 변경 없음                                                                           |
| 고아 메타데이터 (VPN)             | 존재함                                         | 없음 (404) | `MetadataRemoved`                       | 메타데이터 + 레이블 즉시 삭제                                               |
| 고아 메타데이터 (VNet/Subnet)     | 존재함                                         | 없음 (404) | 진단만 수행 (`CspResourceMissing`)   | `Status=Failed`, 명시적인 `Prune` 호출 전까지 메타데이터 **보존**             |
| **종료 실패 상태에 멈춤 (자동 관리되는 `DeletionFailed`, 또는 모든 `DeregisterFailed`)** | `Failed(DeletionFailed` 또는 `DeregisterFailed)` | 존재함 | **`StatusRestored`** | `Ready=True / Restored`, `Synced`는 실제 동기화 상태 반영, `Status=Available` |
| **종료 실패 상태에 멈춤 (사용자 소유 VNet, `DeletionFailed`)** | `Failed(DeletionFailed)` | 존재함 | `NoActionNeeded` (고정됨/sticky) | `Ready`/`Status` 변경 없음 — `PUT .../restore`로 해제 (**TBD**, `resource-reconciliation.md`의 Open Items 참조); 아래 복원 가드 참조 |
| Spider/Terrarium 일시적 장애   | 무관                                            | 5xx / 네트워크 | (없음)                                   | 에러 반환, 상태 변경 없음                                                    |

**복원 가드 (보수적 정책):** 상태는 다음 조건이 **모두** 충족될 때만 `Available`로 복원됩니다:

1. CSP/Spider/Terrarium GET (또는 HEAD) 요청이 성공을 반환하여 리소스가 살아 있음이 확인된 경우.
2. `ConditionReady.Status == False`인 경우.
3. `ConditionReady.Reason ∈ { DeletionFailed, DeregisterFailed }`인 경우 — 종료 실패(Terminal-failure) 상태만 복원됩니다.
4. **VNet 전용** — `ConditionReady.Reason == DeletionFailed`인 경우, 해당 VNet이 자동 관리 리소스(`IsAutoManagedResource`: 공유 이름 접두사 또는 `sys.purpose=infra-dynamic` 레이블 보유)여야 합니다. 사용자 소유 VNet의 `DeletionFailed` 삭제 표시(Tombstone)는 고정(Sticky)되어 있어 CSP 리소스가 살아 있는 것으로 확인되더라도 자동으로 복원되지 **않으며**, 명시적인 `PUT .../restore` 호출을 통해 해제합니다(**TBD**: 아직 Swagger에 문서화되지 않음; `resource-reconciliation.md`의 Open Items 참조). Subnet에는 이러한 소유권 게이트가 없습니다.

진행 중인 상태(`Creating`, `Deleting`, `Registering`, `Deregistering`)와 `CreationFailed`는 동시 실행 중인 작업을 가리거나 부분적으로만 생성된 리소스를 정상으로 잘못 표시하는 일을 막기 위해 의도적으로 **제외**됩니다. `CreationFailed`의 경우 사용자는 리소스를 명시적으로 삭제하고 다시 생성해야 합니다.

하위 Subnet의 상태가 복원되면 상위 VNet의 `ChildrenReady` Condition도 함께 재계산되어 저장됩니다.

**삭제 실패 후 자동 트리거 (자가 치유 / Self-healing):** `DeleteVNet`, `DeleteSubnet`, `DeleteSiteToSiteVPN`, 또는 `DeleteObjectStorage`가 실패하여 리소스를 `Failed(DeletionFailed)`로 기록하면, 실패가 저장된 직후 해당 리소스에 대해 Reconcile이 자동으로 **1회** 호출됩니다. 원래의 삭제 에러는 변경 없이 호출자에게 그대로 반환되며, 자동 Reconcile은 기회주의적(Opportunistic)으로 동작하여 여기서 발생하는 모든 에러는 WARN 레벨로만 로깅됩니다. 이는 다음을 의미합니다:

- 다음 사용자 액션 이전에 제거된 일시적 종속성 때문에 삭제가 실패했던 경우, 다음 읽기 시점에는 수동 복구 없이도 이미 `Available`로 표시됩니다.
- 실제로 CSP 리소스가 사라진 경우, 고아 메타데이터가 자동으로 정리됩니다.
- 실패가 지속적인 경우(예: 장기 실행 종속성 존재), 리소스는 `Failed(DeletionFailed)` 상태로 유지되며 사용자는 종속성을 제거한 후 Reconcile을 호출하거나 삭제를 재시도할 수 있습니다.

자동 트리거는 삭제 실패당 최대 1회만 시도하며, 내부적인 재시도 루프나 백그라운드 스케줄러는 없습니다.

---

## 6. 상태 도출 (Status Derivation)

> [!IMPORTANT]
> **어떤 코드 경로에서든 절대로 `Status`에 직접 값을 할당하지 마세요.** Reconciler나 리소스 함수가 `Ready`와 무관하게 `Status`를 독립적으로 설정하면, 실제로는 Conditions가 뒷받침하지 않는 `Available`(또는 기타 상태 값)로 리소스를 조용히 표시해 버릴 수 있습니다. 실제로 과거에 이와 똑같은 실수(`ApplySyncState`가 `Ready`에 상관없이 `Status`를 설정함)로 인해 복원 불가능해야 할 고정(Sticky) 삭제 표시(Tombstone)가 조용히 `Available`로 되살아나는 문제가 발생해 수정된 바 있습니다. 항상 `SetCondition()`을 통해 `Ready`/`Synced`를 먼저 업데이트한 다음, `Status = Derive*Status(Conditions)`로 상태를 재계산하세요.

상태는 절대 직접 설정되지 않습니다. 항상 Conditions가 업데이트된 후 `Derive*Status()` 함수에 의해 계산됩니다.

### 6.1. DeriveVNetStatus

```go
func DeriveVNetStatus(conditions []Condition) string
```

| Ready.Status     | Ready.Reason    | 결과          |
| ---------------- | --------------- | --------------- |
| `Unknown` 또는 nil | —               | `Unknown`       |
| `False`          | `Creating`      | `Creating`      |
| `False`          | `Deleting`      | `Deleting`      |
| `False`          | `Registering`   | `Registering`   |
| `False`          | `Deregistering` | `Deregistering` |
| `False`          | (기타)         | `Failed`        |
| `True`           | —               | `Available`     |

### 6.2. DeriveSubnetStatus

```go
func DeriveSubnetStatus(conditions []Condition) string
```

`DeriveVNetStatus`와 매핑이 동일합니다.

### 6.3. DeriveVpnStatus

```go
func DeriveVpnStatus(conditions []Condition) string
```

| Ready.Status     | Ready.Reason | 결과      |
| ---------------- | ------------ | ----------- |
| `Unknown` 또는 nil | —            | `Unknown`   |
| `False`          | `Creating`   | `Creating`  |
| `False`          | `Deleting`   | `Deleting`  |
| `False`          | (기타)      | `Failed`    |
| `True`           | —            | `Available` |

VPN은 `Registering` 또는 `Deregistering` 상태를 지원하지 않습니다.

---

## 7. Subnet이 VNet에 미치는 영향

Subnet의 상태가 변경되면 상위 VNet의 `ChildrenReady` Condition이 업데이트됩니다.
VNet 자체의 `Status`는 Subnet의 존재 여부나 개수에 관계없이 `Available`로 유지됩니다.

| Subnet 이벤트                   | VNet ChildrenReady           | VNet Status             |
| ------------------------------ | ---------------------------- | ----------------------- |
| Subnet 추가 성공      | `True` / `AllReady`          | `Available` (변경 없음) |
| Subnet 삭제됨 (다른 Subnet 남아 있음) | `True` / `AllReady`          | `Available` (변경 없음) |
| 마지막 Subnet 삭제됨            | `True` / `NoChildren`        | `Available` (변경 없음) |
| Subnet 삭제 실패         | `False` / `SubnetFailed`     | `Available` (변경 없음) |
| Subnet 작업 진행 중   | `False` / `SubnetInProgress` | `Available` (변경 없음) |

**원칙**: Subnet의 실패가 VNet의 `Ready` Condition을 변경하거나 VNet의 Status를 `Failed`로 만들지 않습니다.
VNet 자체는 계속 정상 작동 가능한 상태로 유지되며, Subnet의 상태는 오직 `ChildrenReady` Condition을 통해서만 추적됩니다.

---

## 8. 구현 파일

| 파일                          | 내용                                                                                                                                                                                                            |
| ----------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `src/core/model/condition.go` | `Condition` 구조체, `ConditionType`/`ConditionStatus` 타입, `NetworkStatus*` 상수, `SetCondition()`, `GetCondition()`, `IsConditionTrue()`, `DeriveVNetStatus()`, `DeriveSubnetStatus()`, `DeriveVpnStatus()` |
| `src/core/model/vnet.go`      | `VNetInfo.Conditions []Condition`, `VNetInfo.SystemMessage string`                                                                                                                                                  |
| `src/core/model/subnet.go`    | `SubnetInfo.Conditions []Condition`, `SubnetInfo.SystemMessage string`                                                                                                                                              |
| `src/core/model/vpn.go`       | `VpnInfo.Conditions []Condition`, `VpnInfo.SystemMessage string`                                                                                                                                                    |
| `src/core/resource/vnet.go`   | CreateVNet, DeleteVNet, RegisterVNet, DeregisterVNet, PruneVNets에 대한 Conditions 전이                                                                                                                        |
| `src/core/resource/subnet.go` | CreateSubnet, DeleteSubnet, RegisterSubnet, DeregisterSubnet, SyncSubnetsForVNet(VNetReconciler에서 호출되며 메타데이터를 절대 삭제하지 않음)에 대한 Conditions 전이                                                    |
| `src/core/resource/vpn.go`    | CreateSiteToSiteVPN, DeleteSiteToSiteVPN, ReconcileSiteToSiteVPN(아직 비파괴적 패턴으로 마이그레이션되지 않음)에 대한 Conditions 전이                                                                      |
| `src/core/reconcile/vnetReconcile.go` | `VNetReconciler` — `PUT .../vNet/reconcile`에서 사용하는 `Reconcile`/`ReconcileAll` 진입점으로, `SyncSubnetsForVNet`을 통해 하위 Subnet까지 재귀적으로 조정함                                                             |
