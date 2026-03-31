# Multi-Cloud Site-to-Site VPN Test CLI

This CLI automates site-to-site VPN validation across multiple CSPs with AWS as the hub side. It provisions an MCI, creates VPNs for enabled test pairs, verifies connectivity with ping, and then cleans up the created resources.

## Features

- Automated end-to-end batch test flow: MCI provisioning, VPN create, VPN query, ping test, VPN delete, cleanup
- Per-test Markdown reports with API request/response traces
- Config-driven test selection via `test-config.yaml`
- Automatic rollback and cleanup when provisioning or test execution fails
- Dynamic MCI filtering: only CSPs used by `execute: true` test cases are included during batch provisioning

## Prerequisites

- CB-Tumblebug is running and reachable
- Required CSP credentials are already registered in CB-Tumblebug
- `test-config.yaml`, `.env`, and `mciDynamic.json` exist in this directory

## Before Running

1. Start CB-Tumblebug.

```bash
cd /path/to/cb-tumblebug
make up
```

2. Initialize CB-Tumblebug and load assets.

- For most VPN tests, run `make init` and choose the appropriate option for your environment.
- For OpenStack VPN testing, choose option `b` during `make init`.
- Note - OpenStack test execution requires fresh asset loading instead of the backup-based initialization path.

```bash
make init
```

3. Create local authentication file for this test CLI.

```bash
cp .env.example .env
```

4. Edit `.env` with your Tumblebug API credentials.

```env
TB_API_USERNAME=your-username
TB_API_PASSWORD=your-password
```

## Configuration

### `test-config.yaml`

`test-config.yaml` controls the Tumblebug endpoint, namespace/MCI identifiers, and which VPN test pairs are executed.

```yaml
tumblebug:
  endpoint: http://localhost:1323
  demo:
    nsId: default
    mciId: mci01
  api:
    mciDynamic:
      reqBody: mciDynamic.json

testTargetPairs:
  testCases:
    - site1: aws
      site2: ibm
      vpnId: vpn-aws-ibm
      execute: true
    - site1: aws
      site2: openstack
      vpnId: vpn-aws-openstack
      execute: true
    - site1: aws
      site2: azure
      vpnId: vpn-aws-azure
      execute: false
```

Notes:

- Only test cases with `execute: true` are executed.
- During batch testing, the MCI is created only with the CSPs required by enabled test cases.
- AWS is always kept as the hub side for the current aws-to-site VPN scenarios.

### `mciDynamic.json`

`mciDynamic.json` remains the API request body template for MCI creation. The batch test does not change its schema. Instead, the CLI reads it and filters `subGroups` in memory based on enabled test cases.

This means:

- You can keep a superset of CSP definitions in `mciDynamic.json`.
- Batch execution provisions only the CSPs actually needed for the enabled test pairs.
- Manual `create mci` execution uses the original `mciDynamic.json` content without filtering.

## Usage

### Run Batch VPN Tests

This is the recommended workflow.

```bash
# Change directory (Use 'popd' to return to the previous directory)
pushd src/testclient/test-clis/site-to-site-vpn
# Run batch test
go run app.go test vpn
```

Batch flow:

1. Read `test-config.yaml`
2. Collect CSPs from `execute: true` test cases
3. Filter `mciDynamic.json` `subGroups` in memory
4. Create MCI
5. Run VPN tests sequentially
6. Clean up VPNs, MCI, and shared resources

### Manual Commands

Use these commands for step-by-step troubleshooting.

Create infrastructure:

```bash
go run app.go create mci -n default -m mci01 -f mciDynamic.json
```

Create a VPN:

```bash
go run app.go create vpn -n default -m mci01 -v vpn01 -t gcp
```

Get VPN info:

```bash
go run app.go get vpn -n default -m mci01 -v vpn01
```

Delete resources:

```bash
go run app.go delete vpn -n default -m mci01 -v vpn01
go run app.go delete mci -n default -m mci01 -o terminate
go run app.go delete shared -n default
```

## Output Files

Generated reports are saved under `test-results/`:

- `summary.md`: Summary of all executed test cases
- `provision.md`: Infrastructure provisioning log
- `cleanup.md`: Cleanup phase log
- `<site1>-to-<site2>-vpn.md`: Detailed per-test API and result log

## Directory Structure

```text
site-to-site-vpn/
├── .env.example
├── app.go
├── mciDynamic.json
├── README.md
├── test-config.yaml
└── test-results/
```
