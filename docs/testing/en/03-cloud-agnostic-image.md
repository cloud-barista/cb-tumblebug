# Cloud-Agnostic Image Creation and Management Integration Test Report

## 0. Summary

**On all 10 CSPs a custom image containing nginx was built automatically with a single call (10/10); on 9 CSPs an infrastructure was recreated from that image, the SG opened port 80 and external HTTP access succeeded.** On KT the KT-side job that creates the boot volume from the custom image ended in error (Nova confirmed it after waiting 600 s), so the recreate failed in this run; an earlier run the same day succeeded in 711 s.

| Metric | Value |
|---|---|
| Image build (create 10 source nodes → install nginx → snapshot → Available → clean up source) | 10/10, 1,359 s in one call |
| Recreate from image | 9/10 (KT: CSP-side volume creation) |
| Open SG port 80 → external HTTP 200 with the baked page | 9/9 |
| Leftovers after cleanup | 0 images, 0 Infra |

## 1. Environment

| Item | Detail |
|---|---|
| Time | 2026-10-13 08:27 – 09:06 UTC (10 CSPs); KT and NCP supplementary run 09:07 – 09:41 UTC |
| CB-Tumblebug | `6e7da269` + sync before snapshot (uncommitted), kind cluster |
| CB-Spider | `fcee1108` |
| Input | 1 node per CSP, Ubuntu 22.04 base image, 50 GB `default` root disk. Specs as in the `infra-across-global` template, except Alibaba uses ecs.e-c1m2.large (4 GiB) |
| Connections | alibaba-ap-northeast-1, aws-ap-northeast-3, azure-koreasouth, gcp-asia-northeast1, ibm-us-south, kt-kr1, ncp-kr, nhn-kr2, openstack-regionone, tencent-ap-seoul |

## 2. Procedure

| Step | Stage | Request | Pass criteria |
|---|---|---|---|
| 1 | Build image | `POST /ns/default/buildAgnosticImage` — create the source Infra (10 nodes), install nginx, write the page and sync via `postCommands`, snapshot each node, wait for Available, clean up the source Infra | 10 images `Available`, source Infra deleted |
| 2 | Verify image | `GET /ns/default/resources/customImage/{id}` | status, OS and CSP image ID populated |
| 3 | Recreate from image | `POST /ns/default/infraDynamic` (`imageId` = custom image, 1 node per CSP) | Node Running |
| 4 | Open SG | `PUT /ns/default/infra/{id}/associatedSecurityGroups` (inbound TCP 22, 80, ICMP; outbound ALL) | Rules applied |
| 5 | Web access | `http://<publicIP>/` from outside (retried for up to 4 minutes) | HTTP 200 with the page baked into the image |
| 6 | Cleanup | Terminate and delete the Infra, delete the custom images | 0 leftovers in TB and CSP |

In step 1 the harness polled every 10 s and recorded, per CSP, when the source node reached Running and when the image became Available.

## 3. Result Matrix

| CSP | nginx install | Image build | Image verify | Recreate | Open SG 80 | HTTP 200 | Cleanup | Overall |
|---|---|---|---|---|---|---|---|---|
| Alibaba | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| AWS | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Azure | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| GCP | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| IBM | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| NHN | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| OpenStack | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Tencent | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| NCP | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ (supplementary run) |
| KT | ✅ | ✅ | ✅ | ✖ KT-side volume creation failed (error after 600 s wait) | ✅ | — | ✅ | Partial |
| **Total** | **10/10** | **10/10** | **10/10** | 9/10 | **10/10** | 9/9 | **10/10** | 9 complete, 1 partial |

In the 10-CSP run the NCP recreated node's public IP was not yet in the create response, so the HTTP check was skipped; the supplementary run passed every stage. KT passed every stage in the 02:36 run the same day (`agimg-kt2`, recreate 711 s) but failed with the same error in the three runs that followed.

## 4. Timing by Stage

| Stage | Time | Notes |
|---|---|---|
| Whole image build (one `buildAgnosticImage` call) | 1,359 s | Source create 210 s (KT longest) + bootstrap about 180 s + snapshot and Available wait (KT at 1,277 s set the total) + source cleanup |
| Read 10 images | Immediate | All `Available`, OS `Ubuntu 22.04`, CSP image ID populated |
| Recreate 10 nodes from images | 720 s | 9 nodes Running within 185 s; KT failure confirmed at 667 s |
| Apply SG rules (10 SGs) | 110 s | Includes the NCP ACG propagation retry |
| HTTP check | 200 right after the SG apply | All 9 CSPs returned 200 on the first attempt |
| Terminate, delete and delete 10 images | 61 s + 60 s | 0 Infra and images left. The dedicated SGs and SSH keys of the source and recreated Infra (2 pairs per CSP) are independent of Infra deletion and need a separate delete |

### Time per CSP (seconds from the image build request)

| CSP | Source node Running | Image Available | Recreate (request → Running) | SG rule add / remove* | HTTP 200 |
|---|---|---|---|---|---|
| Alibaba | 37 | 445 | 40 | <1 / <1 | right after SG apply |
| AWS | 51 | 643 | 48 | <1 / <1 | right after SG apply |
| Azure | 53 | 435 | 80 | 2.2 / 1.6 | right after SG apply |
| GCP | 52 | 493 | 68 | 8.1 / 6.5 | right after SG apply |
| IBM | 52 | 435 | 67 | 4.5 / 2.7 | right after SG apply |
| NHN | 93 | 540 | 185 | <1 / 1.2 | right after SG apply |
| OpenStack | 120 | 388 | 85 | <1 / <1 | right after SG apply |
| Tencent | 55 | 420 | 56 | 1.1 / <1 | right after SG apply |
| NCP | 127 (suppl. 118) | 403 (suppl. 278) | 124 (suppl. 107) | <1 / 13 | right after SG apply (suppl.) |
| KT | 210 (suppl. 59) | 1,277 (suppl. 1,069) | failed (711 in the 02:36 run) | 2.1 / 1.9 | — (200 in the 02:36 run) |

`*` SG rule add and remove were measured separately (10-14: an inbound TCP 80 rule added and removed sequentially on the SG of one node per CSP); in the main run the SG apply (10 SGs at once, 110 s) was set by the NCP ACG propagation retry. The bootstrap (nginx install, page, sync) ran once all nodes were Running, so the snapshots were requested at about 388 s; "Image Available − 388" is the per-CSP snapshot time: OpenStack immediate, AWS 255 s, KT 889 s. Recreate time is the difference between the node `createdTime` in the recreate response and the request time.

## 5. Information Accuracy

| Check | Result |
|---|---|
| Custom image records (status, osType, cspImageName) | 10/10 populated, named per source nodeGroup (`<name>-<nodeGroup>`) |
| Recreated node's `imageId` points to the custom image | 9/9 |
| Recreated node serves the baked page (`CB-TB cloud-agnostic image`) | 9/9 |
| Leftovers in TB, Spider and CSP after delete | 0 images, 0 Infra (dedicated SGs and keys cleaned up separately) |

## 6. Per-CSP Notes (Operations)

| Note | CSP |
|---|---|
| Snapshot completion takes longest (AWS about 4 min, NHN about 2.5 min, KT about 15 min) | AWS, NHN, KT |
| A snapshot taken right after the bootstrap misses files not yet written to disk. Package files survive while the page and config edits written directly disappear, showing as "nginx default page with 200". Resolved by running sync before the snapshot | Observed on Alibaba, GCP, NHN, OpenStack, Tencent, NCP |
| The KT-side job that creates the boot volume from a custom image takes about 10 minutes and sometimes ends in error. Nova checks the volume after waiting 600 s and aborts the instance build, leaving the VM in ERROR. 1 of 4 attempts the same day succeeded (711 s). KT platform behavior, not a TB or Spider timeout | KT |
| On a recreated VM cloud-init writes the new SSH key several minutes after the OS boots, so a manual remote command right away fails authentication | KT |
| The recreated node's public IP may be filled in after the create response; a second read returns it | NCP |
| The image boots with the kernel option ipv6.disable=1, so the nginx default site's `listen [::]:80` fails; the bootstrap removes that line | NCP |
| Sending another ACG change while one is being applied is rejected (1007009); resolved by the retry in the CB-Spider NCP driver | NCP |
| The template spec ecs.e-c4m1.large (0.5 GiB memory) does not boot to an SSH-ready state; replaced with a 4 GiB spec | Alibaba |
| Machine images are global, so the same image is visible from other region connections | GCP |
