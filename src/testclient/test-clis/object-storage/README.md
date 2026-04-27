# Object Storage Batch Test CLI

A CLI tool for batch-testing the Object Storage (bucket) lifecycle —
**create → get → delete** — across multiple CSPs via the CB-Tumblebug API.

Supported CSPs: `aws`, `gcp`, `alibaba`, `tencent`, `ibm`, `openstack`, `ncp`, `nhn`, `kt`

---

## Getting Started

### Step 1 — Prerequisites

- CB-Tumblebug is running and reachable (default: `http://localhost:1323`)
- Cloud connections for the target CSPs are registered in CB-Spider
- Go 1.21+ is installed (required only for building from source)

---

### Step 2 — Navigate to the CLI directory

```bash
cd src/testclient/test-clis/object-storage
```

---

### Step 3 — Configure credentials

Copy the example file and fill in your CB-Tumblebug API credentials:

```bash
cp .env.example .env
```

Edit `.env`:

```env
TB_API_USERNAME=your-username
TB_API_PASSWORD=your-password
```

---

### Step 4 — Configure test cases

Copy the template config and edit it for your environment:

```bash
cp template-test-config.yaml test-config.yaml
```

Edit `test-config.yaml`:

```yaml
tumblebug:
  endpoint: http://localhost:1323 # CB-Tumblebug API endpoint
  nsId: default # Namespace to use for all test operations

testCases:
  - osId: test-bucket-aws
    connectionName: aws-ap-northeast-2
    execute: true # Set to true to include in the test run

  - osId: test-bucket-gcp
    connectionName: gcp-asia-northeast3
    execute: false # Skipped


  # ... add or adjust entries for other CSPs as needed
```

Only test cases with `execute: true` are run.

---

### Step 5 — Run the test

**Sequential** (default) — test cases run one after another:

```bash
go run . test
```

**Parallel** — all enabled test cases run concurrently:

```bash
go run . test --parallel
```

**Override namespace at runtime:**

```bash
go run . test --nsId my-namespace
go run . test --nsId my-namespace --parallel
```

---

### Step 6 — Review the results

After all test cases complete, a summary table is printed:

```
[test-bucket-aws(aws-ap-northeast-2)]   Create: OK (status=Running)  Get: OK (status=Running)  Delete: OK
[test-bucket-gcp(gcp-asia-northeast3)]  Create: OK (status=Running)  Get: OK (status=Running)  Delete: OK
```

Each column shows `OK` on success or `FAIL: <reason>` on error.

---

## Reconcile orphaned metadata

If a bucket creation partially failed and the metadata is stuck in a `Failed`
state, you can clean it up without touching the CSP by calling the Tumblebug
API directly:

```bash
curl -X DELETE \
  "http://localhost:1323/tumblebug/ns/default/resources/objectStorage/test-bucket-tencent?reconcile=true" \
  -u "$TB_API_USERNAME:$TB_API_PASSWORD"
```

The `reconcile=true` option checks whether the CSP bucket actually exists and
removes only the Tumblebug metadata if the bucket is absent, leaving the CSP
resource untouched.

---

## CLI reference

```
go run . test [flags]

Flags:
  -n, --nsId string   Namespace ID (overrides config tumblebug.nsId)
      --parallel      Run test cases in parallel (default: sequential)
  -h, --help          Show help
```
