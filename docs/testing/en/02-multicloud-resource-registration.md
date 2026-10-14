# Multi-Cloud Existing Resource Registration Integration Test Report

## 0. Summary

**Existing resources (VM, vNet, subnet, SG, SSH key) on 10 CSPs were registered into TB; on all 10 the registered node's associations with vNet, subnet, SG and SSH key (28 items) matched; after deregistration no records remained in TB.**

| Metric | Value |
|---|---|
| Create 10 source VMs → remove TB/Spider records (turn into "existing" resources) | 127 s + 1 s |
| Register (`registerCspResources`, one connection at a time) | 10/10, 8–60 s per connection, 260 s total |
| Registered node fields and associations | 10/10 CSPs 28/28, 1.2 s per CSP |
| TB records after deregistration | vNet, SG, key, disk, image 0; Infra 0 (6 s) |

## 1. Environment

| Item | Detail |
|---|---|
| Time | 2026-10-13 09:41 – 10:11 UTC (including cleanup) |
| CB-Tumblebug | `6e7da269`, kind cluster |
| CB-Spider | `fcee1108` |
| Connections | alibaba-ap-northeast-1, aws-ap-northeast-3, azure-koreasouth, gcp-asia-northeast1, ibm-us-south, kt-kr1, ncp-kr, nhn-kr2, openstack-regionone, tencent-ap-seoul |
| Source resources | 1 VM per CSP (created by TB with vNet, subnet, SG and SSH key) → only the TB/Spider records removed, making them "existing" resources |

## 2. Procedure

| Step | Stage | Request | Pass criteria |
|---|---|---|---|
| 1 | Make existing | `deregisterResource` for node, SG, key, vNet (TB also drops the Spider IID) | No TB/Spider record, CSP resources kept |
| 2 | Register | `POST /tumblebug/registerCspResources?infraFlag=y` `{connectionName, nsId, infraNamePrefix}` | Counts per type, 0 failures |
| 3 | Verify associations | Registered Infra node → vNet, subnet, SG, key records | All items in section 5 match |
| 4 | Deregister | `deregisterResource` for node, SG, key, disk, image, vNet | 0 TB records, 0 Spider mappings, CSP resources kept |
| 5 | Cleanup | Register again, delete only what this test created, deregister the rest | Test resources removed |

The harness recorded the time of each stage per CSP (section 4).

## 3. Result Matrix

| CSP | Make existing | Register (vNet / SG / key / VM) | Associations | Deregister | Overall |
|---|---|---|---|---|---|
| Alibaba | ✅ | ✅ 1 / 1 / 1 / 1 (8 s) | ✅ 28/28 | ✅ | ✅ |
| AWS | ✅ | ✅ 3 / 4 / 3 / 1 (12 s) | ✅ 28/28 | ✅ | ✅ |
| Azure | ✅ | ✅ 5 / 7 / 6 / 4 (14 s) | ✅ 28/28 | ✅ | ✅ |
| GCP | ✅ | ✅ 29 / 1 / 2 / 1 (26 s) | ✅ 28/28 | ✅ | ✅ |
| IBM | ✅ | ✅ 1 / 2 / 7 / 1 (60 s) | ✅ 28/28 | ✅ | ✅ |
| KT | ✅ | ✅ 1 / 1 / 1 / 1 (28 s) | ✅ 28/28 | ✅ | ✅ |
| NCP | ✅ | ✅ 10 / 19 / 46 / 2 (17 s) | ✅ 28/28 | ✅ | ✅ |
| NHN | ✅ | ✅ 4 / 3 / 4 / 1 (40 s) | ✅ 28/28 | ✅ | ✅ |
| OpenStack | ✅ | ✅ 11 / 6 / 10 / 1 (22 s) | ✅ 28/28 | ✅ | ✅ |
| Tencent | ✅ | ✅ 5 / 6 / 5 / 3 (28 s) | ✅ 28/28 | ✅ | ✅ |
| **Total** | 10/10 | 10/10 | **10/10** | 10/10 | **10 complete** |

Registered counts include untracked resources that already existed in the region (leftovers of earlier tests, account default VPCs, and so on). This test created 1 vNet, 1 SG, 1 key and 1 VM per CSP. On KT, 34 untracked data disks were registered as well.

## 4. Timing by Stage

| Stage | Time | Notes |
|---|---|---|
| Create 10 source nodes | 127 s | Includes shared vNet, SG and key per connection |
| Make existing (deregister 10 nodes, 10 SGs, 10 keys, 10 vNets) | 1 s | TB deregistration also releases the Spider IID |
| Register 10 connections (sequential) | 260 s | 8–60 s per connection, IBM longest |
| Verify associations (28 items per node) | 12 s | 1.2 s per CSP |
| Deregister everything | 6 s | |
| Cleanup (re-register, terminate, delete) | about 22 min | See the per-CSP table. 4 vNet deletes failed because of dedicated SGs left by other tests and succeeded after those SGs were removed |

### Time per CSP (seconds)

| CSP | Source VM create | Register | Associations | Re-register | VM terminate | SG delete | Key delete | vNet delete |
|---|---|---|---|---|---|---|---|---|
| Alibaba | 39 | 8 | 1.2 | 7 | 1 | 1.3 | 0.7 | failed* → retry ok |
| AWS | 46 | 12 | 1.2 | 11 | 1 | 0.6 | 0.7 | failed* → retry ok |
| Azure | 81 | 14 | 1.2 | 16 | 50 | 1.6 | 0.7 | 28.8 |
| GCP | 51 | 26 | 1.2 | 28 | 57 | 19.3 | <0.1 | failed* → retry ok |
| IBM | 53 | 60 | 1.2 | 46 | 19 | 8.2 | 409 (135) | 16.6 |
| KT | 55 | 28 | 1.2 | 24 | 7 | <0.1 | 0.4 | 12.6 (tier) |
| NCP | 126 | 17 | 1.2 | 13 | 39 | 1.6 | 0.2 | 37.8 (2 attempts) |
| NHN | 85 | 40 | 1.3 | 35 | 20 | 2.4 | 0.3 | 24.4 |
| OpenStack | 117 | 22 | 1.2 | 27 | 27 | 2.3 | 0.1 | failed** → deleted via Spider |
| Tencent | 49 | 28 | 1.2 | 20 | 15 | 0.4 | 1.1 | 2.8 |

Source VM create is the request time to the node `createdTime`. Register time scans every resource in the connection, so it scales with the resource count: GCP (29 VPCs), NCP (46 keys) and IBM (7 keys) took longest. The IBM key delete 409 is a known driver issue (it looks the registered key up by name and does not find it); only the record was deregistered. `*` The AWS, Alibaba and GCP vNets failed with DependencyViolation because dedicated SGs left by earlier tests the same day (lifecycle, image) were still attached to the VPC; deleting those SGs made the vNet delete succeed. `**` OpenStack failed through the TB path because a router port was using the subnet; cleaned up with the Spider VPC delete.

## 5. Association Checks (28 items per node, passed on all 10 CSPs)

| Target | Check |
|---|---|
| Node fields | status Running; specId, imageId, sshKeyId, vNetId, subnetId, securityGroupIds; cspVNetId, cspSubnetId, cspSshKeyId, cspSpecName, cspImageName; publicIP and privateIP populated |
| Against the source | The node's CSP vNet, subnet and key IDs equal the source VM's |
| vNet | Record exists, `cspResourceId` = node `cspVNetId`, node subnet in the vNet's subnet list, subnet `cspResourceId` = node `cspSubnetId`, node in `associatedObjectList` |
| SG | Record exists, vNet link matches, rules present, node in `associatedObjectList` |
| SSH key | Record exists, CSP ID matches (Azure ARM IDs compared case-insensitively), node in `associatedObjectList` |

## 6. Per-CSP Notes (Operations)

| Note | CSP |
|---|---|
| In the first run (10-12) the created node's `cspResourceId` was the full ARM path while the registered node's was the VM name, so automatic comparison failed. After CB-Tumblebug was changed to keep the ARM path as `cspResourceId` for Azure too and derive names from the last segment, and CB-Spider to pass the short name when looking up keys and VPCs for registration, 28/28 passed. ARM IDs can differ in the case of the resource-group segment between responses (the key ID referenced by the VM uses `koreasouth`, the key resource ID uses `KOREASOUTH`), so comparisons must ignore case | Azure |
| Registering one connection pulls in every untracked resource of the region (GCP 29 VPCs, NCP 46 keys, KT 34 disks, …) | All CSPs |
| Deleting an Infra removes only its nodes; the Infra's dedicated SG and SSH key records and CSP resources remain. If such SGs stay attached to a VPC, the vNet delete fails with DependencyViolation, so delete SGs and keys first when cleaning up | AWS, Alibaba, GCP (applies to all CSPs) |
| Deleting a registered key returns 409: the driver looks the key up by name and does not find it. Clean up the CSP key in the console | IBM |
| A router port using the subnet makes the TB vNet delete fail; the Spider VPC delete succeeds | OpenStack |
| The account has a single VPC, so vNet registration is a VPC record and deletion removes only the tier this test created | KT |
| Terminated instances stay in the list for a while and show up as "VM only on CSP" | AWS |
