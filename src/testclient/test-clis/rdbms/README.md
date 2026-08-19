# RDBMS Batch Test CLI

A CLI tool for batch-testing the RDBMS lifecycle via the CB-Tumblebug API:

**rdbms/support (once) → per case: vNet+subnets → securityGroup → rdbms/capability
→ RDBMS create → get → list → database create → database list → dummy data
write/read/verify/delete → database delete → RDBMS delete → securityGroup
delete → subnets delete → vNet delete**

Verified CSPs (see [docs/feature_guide/rdbms-management.md](../../../../docs/feature_guide/rdbms-management.md)):
`aws`, `azure`, `gcp`, `alibaba`, `tencent`, `ibm`, `openstack`, `ncp`, `nhn`

> **RDBMS creation takes minutes.** CB-Tumblebug blocks the `POST` call server-side
> (polling every 30s, up to a 20-minute timeout) until the instance reaches
> `Available` or `Failed`, so each test case can take several minutes to a bit
> over an hour when run with multiple CSPs sequentially. Use `--parallel` to run
> CSPs concurrently.

---

## Getting Started

### Step 1 — Prerequisites

- CB-Tumblebug is running and reachable (default: `http://localhost:1323`)
- Cloud connections for the target CSPs are registered in CB-Spider
- Go 1.21+ is installed (required only for building from source)

---

### Step 2 — Navigate to the CLI directory

```bash
cd src/testclient/test-clis/rdbms
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

Edit `test-config.yaml` — each entry defines a full lifecycle run for one CSP:

```yaml
tumblebug:
  endpoint: http://localhost:1323
  nsId: default

testCases:
  - rdbmsId: test-rdbms-aws
    connectionName: aws-ap-northeast-2
    vNetName: test-rdbms-vnet-aws
    cidrBlock: 10.0.0.0/16
    subnets:
      - name: subnet-1
        cidr: 10.0.1.0/24
        zone: ap-northeast-2a
      - name: subnet-2
        cidr: 10.0.2.0/24
        zone: ap-northeast-2c
    securityGroupName: test-rdbms-sg-aws
    dbEngine: mysql
    autoFillDefaults: true # fills dbEngineVersion/dbSpec/storageType/storageSize
    adminUserName: admin
    adminUserPassword: Password123!
    publicAccess: true
    highAvailability: false
    execute: true # set to true to include in the test run
```

Only test cases with `execute: true` are run. See the template's comments for
CSP-specific requirements (e.g. AWS needs 2 subnets in distinct AZs; IBM
rejects special characters in the password; NCP requires one).

**`autoFillDefaults: true`** picks the first CB-Spider-reported supported option
for `dbEngineVersion`/`dbSpec`/`storageType`/`storageSize` — a
capability-valid pick, not a cost/performance recommendation. Set it to `false`
and fill those four fields explicitly for full control.

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

After all test cases complete, a summary markdown document (one results table
per CSP, covering every step from VNet creation through Database
create/list/dummy-data-test/delete to final cleanup) is printed and written to
`test-results/summary.md`.

Per-CSP detailed API traces (request/response bodies, with secrets masked) are
saved to `test-results/<rdbmsId>.md`.

---

## Cleanup on failure

Steps 10–14 (delete Database, RDBMS, SecurityGroup, subnets, vNet) always run —
even if an earlier step failed — so a failed run doesn't leave billed CSP
resources behind. (Delete Database only runs if Create Database actually
succeeded; the rest run whenever the corresponding create was attempted, not
just when it reported success.)
If a step still fails (e.g. the RDBMS instance never left `Failed`), check
`test-results/<rdbmsId>.md` for the exact error and clean up manually via:

```bash
curl -u "$TB_API_USERNAME:$TB_API_PASSWORD" -X DELETE \
  "http://localhost:1323/tumblebug/ns/default/resources/rdbms/test-rdbms-aws?option=force"
```

---

## CLI reference

```
go run . test [flags]

Flags:
  -n, --nsId string   Namespace ID (overrides config tumblebug.nsId)
      --parallel      Run test cases in parallel (default: sequential)
  -h, --help          Show help
```
