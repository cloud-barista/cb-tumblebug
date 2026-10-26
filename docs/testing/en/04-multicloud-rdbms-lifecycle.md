# Multi-Cloud Managed RDBMS Lifecycle & In-VPC Bastion CRUD Integration Test Report

## 0. Summary

**All stages completed normally across 9 supported CSPs (Alibaba, AWS, Azure, GCP, IBM, NCP, NHN, OpenStack, Tencent) using MapUI default settings (`dbEngine: "mysql"`, `autoFillDefaults: true`, In-VPC Bastion Set Provisioning): Bastion Infra Create → Network Resolve & Pre-flight Validate → RDBMS Create → Read / Reconcile → Full 5-Step Database & Table CRUD → RDBMS Delete → Bastion Infra Terminate & Delete → Shared Resource Release.**

| Metric | Value |
|---|---|
| Bastion VM Infra Create (request → 9/9 Running + `mysql-client` installed) | 50.8 – 220.1 s (parallel wall-clock: 220.1 s) |
| RDBMS Instance Create (request → 9/9 `Available` with active endpoint) | 112.4 – 661.8 s (excl. IBM; 1,072.2 s / 17m 52s on IBM) |
| Full 5-Step DB & Table CRUD (Control Plane DB API + Data Plane SQL via Bastion) | 1.63 – 37.93 s per CSP |
| RDBMS Instance Delete (request → CSP cluster de-provisioned + stabilization wait) | 22.3 – 500.2 s (excl. Alibaba ENI stabilization wait: 22.3 – 256.6 s) |
| Bastion Terminate + Delete + Shared Resource Release (`DELETE /sharedResources`) | 3.2 – 67.7 s per Infra + 12.8 – 56.4 s shared network release |
| Residual Cloud / Tumblebug Resources (`rdbms`, `infra`, `vNet`, `securityGroup`, `sshKey`) | 0 / 0 / 0 / 0 / 0 |

## 1. Environment

| Item | Detail |
|---|---|
| Time | 2026-10-26 UTC (Full E2E Multi-Cloud RDBMS + Bastion Set Benchmark) |
| CB-Tumblebug | `aea9af49`, deployed on a kind cluster |
| CB-Spider | `ee952524` (with OpenStack Trove guest agent readiness verification) |
| CB-MapUI | `RDBMS Management (Relational DB)` default Bastion Set orchestration (`mysql`, `autoFillDefaults: true`, `publicAccess: false` default with provider-aware adaptation) |
| Target CSPs (9) | Alibaba, AWS, Azure, GCP, IBM, NCP, NHN, OpenStack, Tencent |

| CSP | Connection / Region | Resolved Engine & Version | Resolved Spec | Storage | Effective Access Mode | Effective Admin User |
|---|---|---|---|---|---|---|
| Alibaba | `alibaba-ap-northeast-2` (`ap-northeast-2`) | MySQL `8.0` | `mysql.n4.large.1` | `20 GB` (`general_essd`) | Private VPC (`false`) | `dbadmin` |
| AWS | `aws-ap-northeast-2` (`ap-northeast-2`) | MySQL `8.0.45` | `db.t3.micro` | `20 GB` (`gp2`) | Private VPC (`false`) | `dbadmin` |
| Azure | `azure-koreacentral` (`koreacentral`) | MySQL `8.0.21` | `Standard_B1ms` | `32 GB` (`Premium_LRS`) | Private VPC (`false`, Empty Subnet #2) | `dbadmin` |
| GCP | `gcp-asia-northeast3` (`asia-northeast3`) | MySQL `MYSQL_8_0` | `db-n1-standard-1` | `32 GB` (`SSD`) | Public Endpoint (`true`, adapted) | `dbadmin` |
| IBM | `ibm-us-south` (`us-south`) | MySQL `8.4` | `multitenant` (3 members) | `32 GB` | Public Endpoint (`true`, required) | `admin` (fixed) |
| NCP | `ncp-kr` (`kr`) | MySQL `8.0.36` | `SVR.VDBAS.STAND.C002.M008.G005` | `10 GB` (`SSD`) | Private VPC (`false`, fixed) | `dbadmin` |
| NHN | `nhn-kr2` (`kr2`) | MySQL `MYSQL_V848` (`8.4.8`) | `m2.c2m4` | `20 GB` (`GENERAL_SSD`) | Private VPC (`false`) | `dbadmin` |
| OpenStack | `openstack-region01` (`regionone`) | MySQL `8.4` | `m1.small` | `20 GB` | Private VPC (`false`) | `dbadmin` |
| Tencent | `tencent-ap-seoul` (`ap-seoul`) | MySQL `8.0` | `8000` (1000 MB RAM) | `100 GB` (`CLOUD_SSD`) | Private VPC (`false`) | `root` (fixed) |

## 2. Procedure

| Step | Stage | Request | Pass Criteria |
|---|---|---|---|
| 1 | Bastion Infra Create | `POST /ns/default/infraDynamic` (1 dedicated Infra per CSP + `postCommands` installing `mysql-client` & `netcat`) | All Bastion VMs `Running` with SSH command execution ready |
| 2 | Network Resolve & Validate | `GET /ns/default/resources/vNet/{id}`, `POST .../securityGroup/{id}/rules` (port `3306`), `POST /ns/default/resources/rdbms/validate` | Subnet/SG resolved and dry-run validation returns `200 OK` |
| 3 | RDBMS Create | `POST /ns/default/resources/rdbms` (`autoFillDefaults: true`) | All instances reach `Available` status with valid `endpoint` and `port` |
| 4 | Read & Reconcile | `GET /ns/default/resources/rdbms`, `GET .../rdbms/{id}`, `PUT .../rdbms/{id}/reconcile`, `GET .../certificate` (IBM) | Required metadata populated; list, detail, and reconciled views match |
| 5 | Full 5-Step DB & Table CRUD | `[1]` Create DB (`POST .../database` or SQL) → `[2]` List DB (`GET .../database` or SQL) → `[3]` Table `CREATE / INSERT` via Bastion → `[4]` Row `UPDATE / SELECT / DELETE / DROP TABLE` via Bastion → `[5]` Drop DB (`DELETE .../database/{dbName}` or SQL) | All 5 steps succeed; updated row (`updated-verified`) returned from Bastion VM; test DB cleanly dropped |
| 6 | RDBMS Delete | `DELETE /ns/default/resources/rdbms/{id}` | Managed DB cluster deleted on CSP and removed from Tumblebug |
| 7 | Bastion & Shared Cleanup | `GET /ns/default/control/infra/{id}?action=terminate` → `DELETE /ns/default/infra/{id}` → `DELETE /ns/default/sharedResources` | `0` remaining RDBMS, Infra, vNet, SecurityGroup, and SSHKey resources |

## 3. Result Matrix

| CSP | Step 1: Bastion Create | Step 2: Net & Validate | Step 3: RDBMS Create | Step 4: Read & Reconcile | Step 5: 5-Step DB/Table CRUD | Step 6: RDBMS Delete | Step 7: Cleanup & Release | Overall |
|---|---|---|---|---|---|---|---|---|
| Alibaba | ✅ | ✅ | ✅ | ✅ | ✅ (API + SQL) | ✅ | ✅ | ✅ |
| AWS | ✅ | ✅ | ✅ | ✅ | ✅ (Direct SQL) | ✅ | ✅ | ✅ |
| Azure | ✅ | ✅ | ✅ | ✅ | ✅ (API + SQL) | ✅ | ✅ | ✅ |
| GCP | ✅ | ✅ | ✅ | ✅ | ✅ (API + SQL) | ✅ | ✅ | ✅ |
| IBM | ✅ | ✅ | ✅ | ✅ (+ TLS Cert) | ✅ (TLS SQL) | ✅ | ✅ | ✅ |
| NCP | ✅ | ✅ | ✅ | ✅ | ✅ (API + SQL) | ✅ | ✅ | ✅ |
| NHN | ✅ | ✅ | ✅ | ✅ | ✅ (API + SQL) | ✅ | ✅ | ✅ |
| OpenStack | ✅ | ✅ | ✅ | ✅ | ✅ (API + SQL) | ✅ | ✅ | ✅ |
| Tencent | ✅ | ✅ | ✅ | ✅ | ✅ (API + SQL) | ✅ | ✅ | ✅ |
| **Total (9 CSPs)** | **9/9** | **9/9** | **9/9** | **9/9** | **9/9** | **9/9** | **9/9 (`0` left)** | **Pass** |

## 4. Timing by Stage

| Stage | Parallel Wall-Clock (9 CSPs) | Per-CSP Range | Notes |
|---|---|---|---|
| Step 1: Bastion VM Infra Create | 220.1 s (~3m 40s) | 50.8 – 220.1 s | Includes shared VNet/Subnet/SG/SSHKey creation + `apt-get install mysql-client netcat-openbsd` |
| Step 2: Network Resolve & Validate | 77.7 s (Alibaba live spec lookup) / 3.1 s (others) | 0.45 – 77.66 s | Adds TCP `3306` inbound rule to Bastion SG and validates resolved defaults |
| Step 3: RDBMS Instance Create | 661.8 s (~11m 02s excl. IBM) / 1,072.2 s (17m 52s incl. IBM) | 112.4 – 1,072.2 s | Alibaba fastest (`112.4 s`), GCP (`179.6 s`), NCP (`661.8 s`), IBM 3-node cluster (`1,072.2 s`) |
| Step 4: Read & Reconcile | 3.9 s | 0.31 – 3.85 s | Queries CSP live status and synchronizes Tumblebug metadata |
| Step 5: Full 5-Step DB & Table CRUD | 37.9 s | 1.63 – 37.93 s | Combines Control Plane DB Create/List/Drop API + SSH Bastion SQL execution |
| Step 6: RDBMS Instance Delete | 500.2 s (~8m 20s) | 22.3 – 500.2 s | Excl. Alibaba ENI release wait: `22.3 – 256.6 s` (IBM fastest at `22.3 s`, NCP `256.6 s`) |
| Step 7: Bastion Terminate + Shared Release | 80.5 s | 3.2 – 67.7 s + 12.8 – 56.4 s | Terminates all Bastion VMs and deletes all shared vNets, SGs, and SSH keys |

### Time per CSP (seconds)

| CSP | Bastion Create (s) | Net + Validate (s) | RDBMS Create (s) | Read / Reconcile (s) | Control Plane Create / Drop DB (s) | Bastion SQL CRUD (s) | Total 5-Step CRUD (s) | RDBMS Delete (s) | Bastion Terminate + Delete (s) |
|---|---|---|---|---|---|---|---|---|---|
| **Alibaba** (`ap-northeast-2`) | **50.8** | 77.66*** | **112.4** (1m 52s) | 1.56 | *(SQL / API)* | 1.77 | 1.77 | 500.2 (8m 20s)* | **3.2** |
| **GCP** (`asia-northeast3`) | 156.0 | 0.62 | **179.6** (2m 59s) | 1.15 | 0.97 / 1.15 | 1.15 | 3.55 | 155.1 (2m 35s) | 56.1 |
| **Tencent** (`ap-seoul`) | 95.1 | 0.65 | **200.7** (3m 21s) | 0.87 | 2.14 / 1.49 | 1.81 | 6.13 | 144.3 (2m 24s) | 16.4 |
| **OpenStack** (`regionone`) | 220.1 | 1.24 | **260.0** (4m 20s) | 0.31 | 5.77 / 5.77 | 2.14 | 13.99 | 52.0 (52s) | 15.1 |
| **AWS** (`ap-northeast-2`) | 144.3 | 0.45 | **296.6** (4m 57s) | 0.42 | *(Direct SQL)* | 1.63 | 1.63 | 135.8 (2m 16s) | 25.6 |
| **Azure** (`koreacentral`) | 105.2 | 1.52 | **442.3** (7m 22s) | 0.87 | 18.47 / 15.92 | 2.28 | 37.93 | 63.2 (1m 03s) | 40.3 |
| **NHN** (`kr2`) | 142.5 | 2.55 | **464.2** (7m 44s) | 1.88 | 15.53 / 16.46 | 2.02 | 35.67 | 64.1 (1m 04s) | 19.3 |
| **NCP** (`kr`) | 206.1 | 1.24 | **661.8** (11m 02s) | 0.69 | 7.22 / 7.21 | 2.15 | 17.27 | 256.6 (4m 17s) | 67.7 |
| **IBM** (`us-south`)** | 108.4 | 3.08 | **1,072.2** (17m 52s) | 3.85 | *(Direct SQL)* | 3.15 | 3.15 | **22.3** (22s) | 21.5 |

> \* **Alibaba RDBMS Delete Note**: Alibaba Cloud RDS deletes the database instance itself in ~13 seconds, after which CB-Spider waits up to `8m 30s` (`500.2 s` total) for Alibaba's backend VPC network controller to release the hidden RDS ENI so subsequent VSwitch/VPC deletion (`DELETE /sharedResources`) succeeds without `DependencyViolation`.
> \*\* **IBM Cloud Note**: IBM Cloud Databases for MySQL (`8.4`, `multitenant`, `32 GB`) provisions a 3-member multi-zone HA cluster, requiring `17m 52s` (`1,072.2 s`) for initial cluster formation and TLS endpoint registration (`:32421`), while deletion completes in `22.3 s`.
> \*\*\* **Alibaba Pre-flight Validate Note**: Alibaba Cloud's `DescribeAvailableClasses` API enumerates all regional storage and instance class combinations (`general_essd`, `mysql.n4.large.1`) during `autoFillDefaults` validation (~77s).

## 5. Information Accuracy & E2E CRUD Verification

The following checks were executed across all 9 CSPs and passed with zero discrepancies:

| Check | Result |
|---|---|
| Required fields in RDBMS detail view (`id`, `name`, `status`, `connectionName`, `dbEngine`, `dbEngineVersion`, `dbInstanceSpec`, `storageType`, `storageSize`, `endpoint`, `port`, `publicAccess`) | 0 blank or placeholder values once `Available` |
| Detail ↔ List (`GET /ns/default/resources/rdbms`) | `status`, `endpoint`, `port`, `dbEngineVersion`, `publicAccess`, and `vNetId` match 100% |
| Reconcile API (`PUT /ns/default/resources/rdbms/{id}/reconcile`) | Live CSP status and endpoint reflected identically without state drift |
| Control Plane DB Lifecycle (`POST / GET / DELETE .../database`) | Verified on all 7 `cspNativeApi` providers (Alibaba, Azure, GCP, NCP, NHN, OpenStack, Tencent) using stateless `X-Admin-User-Name` and `X-Admin-User-Password` headers |
| Data Plane SQL Lifecycle via In-VPC Bastion (`POST /ns/default/cmd/infra/{id}`) | `CREATE TABLE` → `INSERT` (2 rows) → `UPDATE` (`updated-verified`) → `SELECT` → `DELETE` → `DROP TABLE` verified over TCP `3306` on all 9 CSPs |
| Final Teardown & Shared Resource Release (`DELETE /ns/default/sharedResources`) | `0` RDBMS, `0` Infra, `0` vNet, `0` SecurityGroup, `0` SSHKey remaining |

## 6. Per-CSP Notes (Operations & MapUI Adaptations)

| Characteristic / Constraint | CSP | Operational Behavior & MapUI Handling |
|---|---|---|
| **Network Access Policy (`publicAccess`)** | **NCP** | Strictly VPC-Private (`publicAccess: false`). MapUI locks toggle to `OFF` and routes SQL tests through the in-VPC Bastion VM. |
| **Network Access Policy (`publicAccess`)** | **IBM, GCP** | IBM requires `service-endpoints: public`; GCP Private IP requires Service Networking VPC Peering (`servicenetworking.googleapis.com`). MapUI automatically adapts both to `publicAccess: true` in multi-cloud Bastion Set deployments. |
| **Network Access Policy (`publicAccess`)** | **AWS, Azure, Alibaba, Tencent, NHN, OpenStack** | Supports both Private VPC (`false`, default) and Public Access (`true`). In Private VPC mode, Bastion VMs in the same VNet verify full SQL connectivity. |
| **Subnet Delegation Requirement** | **Azure** | Azure MySQL Flexible Server requires a dedicated empty subnet delegated to `Microsoft.DBforMySQL/flexibleServers` (cannot share Subnet #1 with Bastion VM NICs). MapUI automatically selects Subnet #2 (`subnetIds[1]`). |
| **Fixed Admin Username** | **Tencent, IBM** | Tencent fixes the initial superuser to `root`; IBM fixes the admin user to `admin`. MapUI automatically adapts and displays the effective username per cloud. |
| **Password Validation Rules** | **IBM, AWS, Alibaba, NCP, Tencent** | IBM requires 15–72 alphanumeric characters (no special symbols, cannot contain `admin`); AWS forbids `@`, `/`, `"`, and spaces; NCP requires 8–20 chars with at least 1 symbol; Alibaba requires 8–32 chars across 3+ categories. MapUI auto-adapts passwords per CSP via `getCspCompliantAdminPassword()`. |
| **Database CRUD Method (`dbOperationMethod`)** | **AWS, IBM vs. Others** | AWS RDS and IBM Cloud Databases do not expose a CSP REST API for creating logical schemas inside an instance (`sqlClientRequired`); MapUI executes `CREATE/DROP DATABASE` directly via the Bastion SQL client, whereas the other 7 CSPs use Tumblebug's unified Control Plane API (`/database`). |
| **Alibaba Control Plane DB Privilege** | **Alibaba** | When `CreateDatabase` is called via Alibaba RDS API, CB-Spider automatically calls `GrantAccountPrivilege` (`ReadWrite`) for `X-Admin-User-Name` so the master user can immediately execute DDL/DML on the new schema. |
| **OpenStack Trove Guest Agent Readiness** | **OpenStack** | OpenStack Trove transitions instance status to `ACTIVE` before the internal MySQL/MariaDB guest agent finishes initializing the admin user. CB-Spider verifies guest responsiveness via `ListDatabases` before returning from `CreateCluster`. |
| **Post-Delete Stabilization Wait** | **Alibaba, Tencent, Others** | CB-Spider enforces a post-delete stabilization wait (`8m 30s` on Alibaba, `2m 00s` on Tencent, `10s` on others) so hidden database ENIs/ports are fully detached before VPC/Subnet deletion. |

