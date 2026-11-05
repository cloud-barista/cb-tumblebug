# 스토리지 상태 관리 (Storage Status Management)

## 1. 개요

이 문서는 CB-Tumblebug 스토리지 리소스인 **Object Storage**의 상태(수명주기) 관리 시스템을 설명합니다.

이 시스템은 `Conditions`를 단일 진실 공급원(Source of truth)으로 삼고 그로부터 `Status`를 도출(Derive)하는 **Kubernetes Conditions 패턴**을 채택하고 있습니다. 각 리소스는 `Conditions` 배열과 `Status` 문자열 필드를 가집니다. 실패 시에는 `SystemMessage` 필드가 사람이 읽을 수 있는 상세 에러 정보를 제공합니다.

이는 [네트워크 리소스](network-status-management.md)(VNet, Subnet, VPN)에서 사용하는 것과 동일한 패턴입니다.
상태 상수는 `model/condition.go`에 정의된 공통 `ResourceStatus*` 기본 상수를 공유하며, 스토리지 리소스를 위한 도메인별 별칭(`StorageStatus*`)을 제공합니다.

### 1.1. 적용 대상 리소스

| 리소스       | 모델 파일               | 리소스 파일               | Conditions | 상태 도출 함수 (Derive Function) |
| -------------- | ------------------------ | --------------------------- | :--------: | ----------------------------- |
| Object Storage | `model/objectStorage.go` | `resource/objectStorage.go` |     ✓      | `DeriveObjectStorageStatus()` |

### 1.2. 주요 설계 결정 사항

- **상태(Status)는 직접 설정되지 않고 도출됩니다.** 모든 상태 전이는 `SetCondition()` + `DeriveObjectStorageStatus()`를 거칩니다.
- **실패 상태(Failed state)는 명시적입니다.** API 호출이 실패하면 리소스는 실패한 작업을 나타내는 `Reason`(예: `CreationFailed`, `DeletionFailed`)과 함께 `Failed`로 전이됩니다. 이를 통해 리소스가 `Creating`이나 `Deleting`과 같은 전이 상태에 영구적으로 갇히는 것을 방지합니다.
- **Label에는 상태를 저장하지 않습니다.** Kubernetes 관례에 따라 `sys.status`는 Label에 저장하지 않으며, 상태는 리소스 객체 자체에서 조회합니다.
- **상태 상수는 도메인 별칭(Alias)을 사용합니다.** `StorageStatus*` 상수는 `model/condition.go`에 정의된 공통 `ResourceStatus*` 기본 상수의 별칭입니다. 이를 통해 공통의 단일 진실 공급원을 공유하면서도 각 도메인이 독립적으로 확장될 수 있습니다.
- **Register/Deregister를 지원하지 않습니다.** 네트워크 리소스와 달리 스토리지 리소스는 Register/Deregister 작업을 지원하지 않습니다. 따라서 `StorageStatus*` 집합에서는 의도적으로 `Registering` 및 `Deregistering` 상태가 제외되어 있습니다.

---

## 2. 상태 값 (Status Values)

### 2.1. 스토리지 상태 상수

`model/condition.go`에 `ResourceStatus*`의 별칭으로 정의됨:

| 상수                 | 값         | Object Storage | 설명                            |
| ------------------------ | ------------- | :------------: | -------------------------------------- |
| `StorageStatusAvailable` | `"Available"` |       ✓        | 리소스가 준비되어 사용 가능한 상태           |
| `StorageStatusCreating`  | `"Creating"`  |       ✓        | 생성 작업 진행 중           |
| `StorageStatusDeleting`  | `"Deleting"`  |       ✓        | 삭제 작업 진행 중           |
| `StorageStatusFailed`    | `"Failed"`    |       ✓        | 작업 실패; Condition의 Reason 참조 |
| `StorageStatusUnknown`   | `"Unknown"`   |       ✓        | Condition 데이터를 사용할 수 없음            |

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

| 유형     | Object Storage | 설명                                     |
| -------- | :------------: | ----------------------------------------------- |
| `Ready`  |       ✓        | 리소스 자체를 사용할 수 있는지 여부           |
| `Synced` |       ✓        | 리소스가 CSP/Spider와 동기화되어 있는지 여부 |

Object Storage는 하위 리소스가 없으므로 `ChildrenReady` 조건을 사용하지 않습니다.

### 3.3. Reason 상수

**Ready 조건**

| Reason           | 사용 리소스        | 상황                                                                                                        |
| ---------------- | -------------- | ---------------------------------------------------------------------------------------------------------------- |
| `Creating`       | Object Storage | 생성 진행 중                                                                                             |
| `CreationFailed` | Object Storage | 생성 실패                                                                                                  |
| `Deleting`       | Object Storage | 삭제 진행 중                                                                                             |
| `DeletionFailed` | Object Storage | 삭제 실패                                                                                                  |
| `Available`      | Object Storage | 정상 작동 가능 상태                                                                                                      |
| `Restored`       | Object Storage | `DeletionFailed` 이후 CSP 버킷이 여전히 존재하는 것으로 확인되어 Reconcile에 의해 Available로 상태가 복원됨 |

**Synced 조건**

| Reason                | 상황                                                               |
| ---------------------- | ------------------------------------------------------------------------ |
| `Creating`             | 아직 동기화가 수립되지 않음                                                 |
| `Available` / `InSync` | 리소스가 CSP와 동기화됨                                             |
| `CspResourceMissing`   | TB 및 Spider 메타데이터는 존재하지만 CSP 버킷이 사라짐                   |
| `SpMetaMissing`        | TB 메타데이터가 존재하고 CSP 버킷도 살아 있지만 Spider가 IID를 잃어버림  |
| `TbMetaOnly`           | 버킷이 Spider와 CSP 양쪽 모두에 존재하지 않음 — 고스트(Ghost) 메타데이터              |

전체 진단 상태 매트릭스는 `docs/feature_guide/resource-reconciliation.md`를 참조하세요. 위 표는 각 상황에 대한 수행 액션이 아니라 Reason 값만 나열한 것입니다.

---

## 4. 상태 전이 (State Transitions)

### 4.1. Object Storage 상태 전이

Object Storage 리소스는 Spider S3 호환 API를 통해 생성 및 삭제됩니다.
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

**처리되는 Object Storage 실패 지점:**

| 함수              | 실패 지점                               | 설정되는 Condition                  |
| --------------------- | ------------------------------------------- | ------------------------------ |
| `CreateObjectStorage` | Spider PUT 재시도 소진 (409 Conflict) | `Ready=False / CreationFailed` |
| `CreateObjectStorage` | Spider PUT 실패 (충돌 이외의 에러)       | `Ready=False / CreationFailed` |
| `CreateObjectStorage` | PUT 이후 Spider GET 실패                  | `Ready=False / CreationFailed` |
| `DeleteObjectStorage` | Spider DELETE 재시도 소진             | `Ready=False / DeletionFailed` |

---

## 5. 작업별 Conditions 전이

### 5.1. 생성 (Create)

| 단계   | Ready                      | Synced               | Status      |
| ------- | -------------------------- | -------------------- | ----------- |
| 시작   | `False` / `Creating`       | `False` / `Creating` | `Creating`  |
| 성공 | `True` / `Available`       | `True` / `Available` | `Available` |
| 실패 | `False` / `CreationFailed` | 변경 없음            | `Failed`    |

실패 시 `SystemMessage`에 에러 상세 정보가 설정되고 리소스가 kvstore에 저장되어 `Failed` 상태가 영구적으로 유지됩니다.

### 5.2. 삭제 (Delete)

| 단계   | Ready                      | Synced    | Status     |
| ------- | -------------------------- | --------- | ---------- |
| 시작   | `False` / `Deleting`       | 변경 없음 | `Deleting` |
| 성공 | (리소스 제거됨)         |           |            |
| 실패 | `False` / `DeletionFailed` | 변경 없음 | `Failed`   |

실패 시 `SystemMessage`에 에러 상세 정보가 설정되고 리소스가 kvstore에 저장됩니다.

### 5.3. 조정 (Reconcile)

Reconcile은 **비파괴적 진단(Non-destructive diagnosis) 전용**으로, 절대로 메타데이터를 삭제하거나 파괴적인 Spider API를 호출하지 않습니다. 이는 `ObjectStorageReconciler`(`src/core/reconcile/objectStorageReconcile.go`)에 의해 구현되며, VNet 및 RDBMS와 마찬가지로 공유 헬퍼인 `GetResourceSyncState`(`src/core/resource/common.go`)를 사용한 다음, `SetCondition`을 통해 `Ready`/`Synced`를 직접 설정하고 `DeriveObjectStorageStatus`를 통해 `Status`를 재계산합니다. Reconcile이 왜 절대로 Spider `DELETE`를 호출해서는 안 되는지를 포함한 전체 설계는 `docs/feature_guide/resource-reconciliation.md`를 참조하세요.

| 시나리오                            | 메타데이터           | CSP 버킷    | 설정되는 Synced Reason        | 결과                                                                       |
| ------------------------------------ | ------------------- | ------------- | -------------------------- | ------------------------------------------------------------------------------ |
| 정상 (Healthy)                             | 존재함 / `Available` | 존재함        | `InSync`                  | 변경 없음                                                                     |
| 생성된 적 없음 / 고아 메타데이터   | 존재함              | 없음 (404) | `CspResourceMissing`      | `Status=Failed`, **메타데이터 보존** — 오직 `Prune`(별도의 명시적 `POST .../reconcile/prune` 호출)만 이를 제거함 |
| Spider가 IID를 잃었으나 CSP는 살아 있음 | 존재함              | 존재함        | `SpMetaMissing`           | `Synced=False/SpMetaMissing`; `Status`는 그대로 유지됨 (`Failed`로 강제 전환하지 않음)     |
| **종료 실패 상태에 멈춤 (자동 관리)** | `Failed(DeletionFailed)` | 존재함 | `Available` (복원됨) | `Ready=True/Restored` (이전 실패 내용은 메시지에 유지됨), `Status=Available` |
| **종료 실패 상태에 멈춤 (사용자 소유)** | `Failed(DeletionFailed)` | 존재함 | `NoActionNeeded` (고정됨/sticky) | `Ready`/`Status` 변경 없음 — 복구 경로는 **TBD** (Object Storage에는 아직 `PUT .../restore`가 연결되지 않음, `resource-reconciliation.md`의 Open Items 참조); 아래 복원 가드 참조 |
| Spider 일시적 장애             | 무관                 | 5xx / 네트워크 | (없음)                    | 에러 반환, 상태 변경 없음                                             |

**복원 가드:** 상태는 CSP 버킷이 존재하는 것으로 확인되고, `ConditionReady`가 `False`이면서 `Reason`이 `{DeletionFailed, DeregisterFailed}`에 속하며(`model.ShouldRestoreToAvailable`), 특히 `DeletionFailed`의 경우 해당 오브젝트 스토리지가 자동 관리 리소스(`IsAutoManagedResource`: 공유 이름 접두사 또는 `sys.purpose=infra-dynamic` 레이블 보유)일 때만 `Available`로 복원됩니다. 사용자 소유 버킷의 `DeletionFailed` 삭제 표시(Tombstone)는 고정(Sticky)되어 있으며 자동으로 복원되지 **않습니다**. 복구 경로는 **TBD**입니다 — 범용 `PUT .../restore` 핸들러(`RestRestoreResource`)는 존재하지만 아직 Object Storage에는 연결되지 않았습니다(`resource-reconciliation.md`의 Open Items 참조). `CreationFailed` 및 진행 중인 상태는 의도적으로 제외됩니다.

**삭제 실패 후 자동 트리거:** `DeleteObjectStorage`가 실패하면 리소스를 `Failed(DeletionFailed)`로 표시하고 에러를 반환하며, 절대로 `kvstore.Delete`나 Spider `DELETE`를 호출하지 않습니다. 그런 다음 REST 핸들러(`RestDeleteObjectStorage`, `src/interface/rest/server/resource/objectStorage.go`)가 VNet의 패턴과 동일하게 실패가 저장된 직후 `reconcile.GetManager().RunReconcile(...)`을 **1회** 호출합니다. 이는 온디맨드 `PUT .../reconcile` 엔드포인트(`ObjectStorageReconciler.reconcileFailed`)와 동일한 진단 및 조건부 복원 검사를 수행합니다. 원래의 삭제 에러는 변경 없이 호출자에게 그대로 반환되며, 자동 Reconcile은 기회주의적(Opportunistic)으로 동작하여 발생하는 모든 에러는 WARN 레벨로만 로깅됩니다. 내부적인 재시도 루프나 백그라운드 스케줄러는 없습니다.

**고아 메타데이터 정리:** Reconcile은 `CspResourceMissing`을 진단만 하며, 실제로 해당 메타데이터를 제거하려면 별도의 명시적인 `POST /ns/{nsId}/resources/objectStorage/reconcile/prune` 호출(`PruneObjectStorages`)이 필요합니다. 이 호출은 `ConditionSynced.Reason == CspResourceMissing`인 항목만 처리합니다.

---

## 6. 상태 도출 (Status Derivation)

> [!IMPORTANT]
> **어떤 코드 경로에서든 절대로 `Status`에 직접 값을 할당하지 마세요.** Reconciler나 리소스 함수가 `Ready`와 무관하게 `Status`를 독립적으로 설정하면, 실제로는 Conditions가 뒷받침하지 않는 `Available`(또는 기타 상태 값)로 리소스를 조용히 표시해 버릴 수 있습니다. 실제로 과거에 이와 똑같은 실수(`ApplySyncState`가 `Ready`에 상관없이 `Status`를 설정함)로 인해 복원 불가능해야 할 고정(Sticky) 삭제 표시(Tombstone)가 조용히 `Available`로 되살아나는 문제가 발생해 수정된 바 있습니다. 항상 `SetCondition()`을 통해 `Ready`/`Synced`를 먼저 업데이트한 다음, `Status = DeriveObjectStorageStatus(Conditions)`로 상태를 재계산하세요.

상태는 절대 직접 설정되지 않습니다. 항상 Conditions가 업데이트된 후 `DeriveObjectStorageStatus()`에 의해 계산됩니다.

### 6.1. DeriveObjectStorageStatus

```go
func DeriveObjectStorageStatus(conditions []Condition) string
```

| Ready.Status     | Ready.Reason | 결과      |
| ---------------- | ------------ | ----------- |
| `Unknown` 또는 nil | —            | `Unknown`   |
| `False`          | `Creating`   | `Creating`  |
| `False`          | `Deleting`   | `Deleting`  |
| `False`          | (기타)      | `Failed`    |
| `True`           | —            | `Available` |

Object Storage는 `Registering` 또는 `Deregistering` 상태를 지원하지 않습니다.

---

## 7. 네트워크 상태 관리와의 관계

스토리지 리소스와 네트워크 리소스는 동일한 Conditions 인프라를 공유합니다:

| 공유 컴포넌트   | 위치             | 설명                                              |
| ------------------ | -------------------- | -------------------------------------------------------- |
| `Condition` 구조체 | `model/condition.go` | 모든 리소스 도메인에서 동일한 구조체 사용                     |
| `SetCondition()`   | `model/condition.go` | 조건 업데이트를 위한 동일한 함수 사용                    |
| `GetCondition()`   | `model/condition.go` | 조건 조회를 위한 동일한 함수 사용                     |
| `ResourceStatus*`  | `model/condition.go` | 모든 도메인이 공유하는 공통 기본 상수              |
| Reason 상수   | `model/condition.go` | `Creating`, `CreationFailed` 등 도메인 간 공유 |

도메인별 차이점:

| 항목          | 네트워크 (`NetworkStatus*`)                      | 스토리지 (`StorageStatus*`) |
| --------------- | ----------------------------------------------- | -------------------------- |
| 상태 값   | 7개 (Registering, Deregistering 포함)         | 5개 (Register/Deregister 없음) |
| Condition 유형 | Ready, Synced, ChildrenReady                    | Ready, Synced              |
| 지원 작업      | Create, Delete, Register, Deregister, Reconcile | Create, Delete, Reconcile  |

---

## 8. 구현 파일

| 파일                                 | 내용                                                                                                      |
| ------------------------------------ | ------------------------------------------------------------------------------------------------------------- |
| `src/core/model/condition.go`        | `Condition` 구조체, `ResourceStatus*` 기본 상수, `StorageStatus*` 별칭, `DeriveObjectStorageStatus()` |
| `src/core/model/objectStorage.go`    | `ObjectStorageInfo.Conditions []Condition`, `ObjectStorageInfo.SystemMessage string`                          |
| `src/core/resource/objectStorage.go` | `CreateObjectStorage`, `DeleteObjectStorage`, `PruneObjectStorages`에 대한 Conditions 전이 |
| `src/core/reconcile/objectStorageReconcile.go` | `ObjectStorageReconciler` — `PUT .../reconcile`에서 사용하는 `Reconcile`/`ReconcileAll` 진입점 |
