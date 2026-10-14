# Multi-Cloud VM Lifecycle Integration Test Report

## 0. Summary

**All stages completed normally on 10 CSPs, 18 regions and 40 nodes: create → read → suspend → resume → reboot → terminate → delete.**

| Metric | Value |
|---|---|
| Create (request → 40/40 Running) | 446 s |
| Suspend → resume (suspend done 553 s + resume done 94 s) | 647 s (suspend 63 s excluding 2 Alibaba Tokyo nodes) |
| Reboot (request → 40/40 Running) | 555 s (49 s excluding the same 2 nodes) |
| Terminate (request → 40/40 Terminated) | 66 s |
| Control failures / read inconsistencies | 0 / 0 |

## 1. Environment

| Item | Detail |
|---|---|
| Time | 2026-10-13 06:58 – 07:38 UTC (2,348 s total) |
| CB-Tumblebug | `6e7da269`, deployed on a kind cluster |
| CB-Spider | `fcee1108` |
| Input | Infra template `infra-across-global` (18 nodeGroups, 40 nodes, root disk `default`) |
| CSPs | Alibaba, AWS, Azure, GCP, IBM, KT, NCP, NHN, OpenStack, Tencent |

| CSP | Region | Spec | Nodes |
|---|---|---|---|
| Alibaba | ap-northeast-1 / us-east-1 | ecs.e-c4m1.large | 2 / 2 |
| AWS | ap-northeast-3 / ca-central-1 | t3.small / t3a.small | 2 / 2 |
| Azure | koreasouth / australiaeast | standard_b2ts_v2 / standard_b1ms | 2 / 2 |
| GCP | asia-northeast1 / europe-west3 | e2-standard-2 / t2d-standard-1 | 2 / 2 |
| IBM | us-south / jp-osa | bxf-2x8 / nxf-2x1 | 2 / 2 |
| KT | kr1 | 2x2.itl | 4 |
| NCP | kr / sgn | s2-g3a / c2-g3 | 2 / 2 |
| NHN | kr2 / jp1 | c2.c2m2 | 2 / 2 |
| OpenStack | regionone | m1.medium | 4 |
| Tencent | ap-seoul / ap-hongkong | s2.medium2 | 2 / 2 |

## 2. Procedure

| Step | Stage | Request | Pass criteria |
|---|---|---|---|
| 1 | Create | `POST /ns/default/infra/template/infra-across-global` | All 40 nodes `Running` |
| 2 | Read | Detail, list, status, access-info and single-node APIs | Required fields populated, values consistent across views |
| 3 | Suspend | `GET /ns/default/control/infra/{id}?action=suspend` | All 40 nodes `Suspended` |
| 4 | Resume | `?action=resume` | All 40 nodes `Running`, IP changes reflected in every view |
| 5 | Reboot | `?action=reboot` | All 40 nodes `Running` |
| 6 | Terminate | `?action=terminate` | All 40 nodes `Terminated` |
| 7 | Delete | `DELETE /ns/default/infra/{id}` | Removed from list and detail |

The reads of step 2 were repeated after every stage to verify information accuracy.

## 3. Result Matrix

| CSP | Create | Read | Suspend | Resume | Reboot | Terminate | Delete | Overall |
|---|---|---|---|---|---|---|---|---|
| Alibaba (4) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| AWS (4) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Azure (4) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| GCP (4) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| IBM (4) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| KT (4) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| NCP (4) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| NHN (4) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| OpenStack (4) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Tencent (4) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Total (40)** | 40/40 | 40/40 | 40/40 | 40/40 | 40/40 | 40/40 | 40/40 | **Pass** |

## 4. Timing by Stage

| Stage | All done (request → 40/40) | Notes |
|---|---|---|
| Create | 446 s (synchronous response) | 4 KT nodes last at 401–444 s |
| Suspend | 553 s | 38 nodes Suspended within 63 s; 2 Alibaba ap-northeast-1 nodes at 544 and 553 s |
| Resume | 94 s | 4 OpenStack nodes at 80–94 s, the rest within 49 s |
| Reboot | 555 s | 38 nodes back to Running within 49 s; 2 Alibaba ap-northeast-1 nodes at 539 and 555 s |
| Terminate | 66 s (synchronous response) | 40/40 Terminated at response time |
| Delete | Immediate | Removes Infra and node records. The Infra's dedicated SGs and SSH keys (18 nodeGroups' worth) remain and need a separate delete |

### Time per CSP (seconds, min–max of 4 nodes)

| CSP | Create | Suspend | Resume | Reboot | Terminate |
|---|---|---|---|---|---|
| AWS (4) | 96–101 | 21–37 | 23 | 6 | 26–32 |
| Azure (4) | 134–142 | 15–26 | 23 | 6 | 9 |
| GCP (4) | 110–128 | 63 | 23 | 6–12 | 63 |
| IBM (4) | 99–103 | 21 | 23 | 6 | 21 |
| NHN (4) | 143–168 | 21–26 | 23–26 | 6–23 | 26–32 |
| OpenStack (4) | 181–224 | 32–37 | 80–94 | 49 | 32 |
| Tencent (4) | 106–109 | 15–21 | 17–23 | 6 | 9 |
| Alibaba (4) | 86–91 | 32–553 | 17–23 | 32–555 | 6 |
| NCP (4) | 178–183 | 15–21 | 49 | 32–49 | 63 |
| KT (4) | 401–444 | 37 | 23 | 23 | 37 |

Create is "request time → node createdTime (Running confirmed)" and includes creating the shared vNet, SG and key per connection. Control stages run from the control request until the node was observed in the target state; observation every 5 s, so up to 5 s of error. Terminate is a synchronous call, so it was observed in parallel with the request. The two Alibaba ap-northeast-1 nodes stayed unchanged at the CSP for about 9 minutes during suspend and reboot before completing (the two us-east-1 nodes of the same spec took 32 s); attributed to the template spec (0.5 GiB memory).

## 5. Information Accuracy

The following checks ran on all 40 nodes at every stage and all passed.

| Check | Result |
|---|---|
| Required fields in the detail view (spec, image, vNet, subnet, sshKey, SG, CSP IDs, IPs, region, location, spec/image summary, createdTime) | 0 blank or placeholder values |
| Detail ↔ list (`GET /infra`) | status, IPs, spec, image, CSP ID, createdTime match (the list view does not expose nodeUserName, so it is excluded) |
| Detail ↔ status view (`?option=status`, single and list) | status, publicIP, cspResourceId match |
| Detail ↔ access info (`?option=accessinfo`) | publicIP, user name, SSH key match |
| Immutable fields across stages (spec, image, key, vNet, subnet, SG, CSP ID, createdTime) | 0 changes |
| IP changes after resume | All 12 changes identical in every view |
| After delete | Removed from `?option=id` and detail |

## 6. Per-CSP Notes (Operations)

| Behavior | CSP |
|---|---|
| Public IP released on suspend, new IP assigned on resume | AWS, GCP |
| Public IP released on suspend, same IP restored on resume | Alibaba |
| Public IP kept while suspended | Azure, IBM, KT, NCP, NHN, OpenStack, Tencent |
| After terminate | Last IP preserved on every CSP (kept for audit until delete; intended) |
| Reboot transition | Most nodes back to Running within 49 s; 2 Alibaba ap-northeast-1 nodes took about 9 minutes |
| Suspend transition | Most nodes Suspended within 63 s; 2 Alibaba ap-northeast-1 nodes took about 9 minutes (the CSP appears to force-stop after waiting for the guest OS) |
