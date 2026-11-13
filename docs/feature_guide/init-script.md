# Init Script (first-boot provisioning)

Guide to `initScript`: a first-boot script that CB-Tumblebug delivers to every Linux Node as cloud-init user data, with the SSH account, reusable script templates, ordered steps and result reporting through the [Notifier](notifier-ntfy.md).

## 📑 Table of Contents

1. [Overview](#overview)
2. [How It Works](#how-it-works)
3. [Request Format](#request-format)
4. [Script Templates](#script-templates)
5. [Variables Available to Scripts](#variables-available-to-scripts)
6. [Reviewing and Checking Results](#reviewing-and-checking-results)
7. [API Reference](#api-reference)
8. [Limitations and Troubleshooting](#limitations-and-troubleshooting)

---

## Overview

### What is an init script?

Every CSP runs user data once on the first boot of a VM (cloud-init on most images). CB-Spider used to own that user data: it created the `cb-user` account and injected the SSH key. With `initScript`, CB-Tumblebug renders the user data itself and passes it to CB-Spider as `UserData`. The Node then:

1. creates the SSH account (`cb-user` by default) with the Infra's key and passwordless sudo,
2. runs the user's scripts in order, stopping at the first failure,
3. reports to the Notifier as each step starts and once at the end; CB-Tumblebug records the latest snapshot on the Node as `initScriptStatus` (`pending` → `initializing` → `ok`/`fail`).

### Why use it?

- **No SSH needed at scale.** Bootstrapping 10,000 Nodes over `postCommands` means 10,000 SSH sessions from CB-Tumblebug. An init script runs on the VM by itself and the VM reports back, even when CB-Tumblebug sits in a private network.
- **Same behaviour on every CSP.** The wrapper script is the same on AWS, Azure, GCP, Alibaba, Tencent, IBM, OpenStack, KT, NCP and NHN.
- **Reusable steps.** Scripts can be stored as templates in the `system` namespace (shared) or in a user namespace, and combined in order, for example base hardening, then a monitoring agent, then a role-specific script.

### Key highlights

| Feature | Description |
|---------|-------------|
| Modes | `default` (account + scripts; also what you get when `initScript` is omitted), `custom` (scripts only; the image's own account stays), `none` (CB-Spider default) |
| Steps | the template's scripts, then `script`, then `scripts[]` run in that order; the first non-zero exit stops the run |
| Templates | `/ns/{nsId}/template/initScript`: an ordered list of scripts plus default env; looked up in the request namespace, then in `system` |
| Env | Template `env` defaults, overridden by the request `env`, exported to every step |
| Result | `initScriptStatus` (`pending`/`initializing`/`ok`/`fail`) and `initScriptMessage` on the Node; counts and per-step details from `GET /ns/{nsId}/infra/{infraId}/initScript` |
| Reporting | On by default when a Notifier is registered; `notify: false` turns it off for a NodeGroup |
| Size limit | 16 KB of rendered user data (the AWS limit; checked by the Review API) |

---

## How It Works

```
Infra / NodeGroup request ──▶ resolve templates ──▶ render user data ──▶ CB-Spider (UserData)
      initScript                (copied into the         #!/bin/bash                 │
                                 NodeGroup blueprint)    export TB_*=…               ▼
                                                         wrapper                 VM first boot
                                                                                     │
                                            Node.initScriptStatus ◀── Notifier ◀── report (ntfy)
```

- Templates are resolved when the Infra or NodeGroup is created and the resulting steps are stored in the NodeGroup. Nodes added later by scale-out get exactly the same content even if the template changed afterwards.
- The wrapper is embedded in the binary (`src/core/embedded/scripts/init-script.sh`). It is idempotent: if the status marker `/var/lib/tb/init-script.status` exists, it exits immediately, which matters on CSPs that run user data on every boot (GCP).
- Step output goes to `/var/log/tb-init-script.log` on the Node. Each step is saved as `/var/lib/tb/init-script-<n>.sh`.
- When a request has no `initScript`, CB-Tumblebug applies the default (`mode: default`, `cb-user`, no steps) so every Linux Node behaves the same and reports when it has booted. If the user data cannot be rendered for such an implicit request (for example the SSH public key is unavailable), the Node falls back to the CB-Spider default user data and notes it in `systemMessage`; an explicit `initScript` fails the Node instead.
- Reports are stored per Node in their own record (one write per report, no update of the Node record), so thousands of Nodes reporting at once do not slow provisioning down.
- Windows Nodes and `mode: none` keep the CB-Spider default user data.

---

## Request Format

`initScript` can be set on an Infra request (applies to every NodeGroup) and overridden per NodeGroup. It is accepted by `POST /ns/{nsId}/infraDynamic`, `POST /ns/{nsId}/infraDynamicReview`, `POST /ns/{nsId}/infra/{infraId}/nodeGroupDynamic` and Infra templates.

```json
{
  "name": "web",
  "initScript": {
    "mode": "default",
    "userName": "cb-user",
    "templateId": "initscript-nginx",
    "scripts": [
      { "name": "role", "script": "#!/bin/bash\necho web > /etc/role\n" }
    ],
    "env": { "ROLE": "web", "OTEL_ENDPOINT": "https://otel.example.com" }
  },
  "nodeGroups": [ { "name": "g1", "specId": "aws+ap-northeast-2+t3.small", "nodeGroupSize": 2 } ]
}
```

| Field | Description |
|-------|-------------|
| `mode` | `default` creates the account and runs the steps. `custom` only runs the steps; use it with images that already provide an account (the Node's `nodeUserName` must then be set to that account). `none` disables the feature. |
| `userName` | Account to create in `default` mode (default `cb-user`). |
| `templateId` | Init script template whose scripts run first, in order. One template per request; its default `env` is applied under the request `env`. |
| `script` | Shorthand for an inline step named `inline`. |
| `scripts[]` | Inline steps `{name?, script, continueOnError?, timeoutSec?, labelSelector?}` appended after the template's steps. Unnamed steps are named `step-<n>`. |
| `env` | Variables exported to every step. Keys must be valid shell names and must not start with `TB_`; values must not contain single quotes or newlines. |
| `notify` | `false` stops the Node from reporting to the Notifier. Default: report when a Notifier is registered. |

Steps run in the order template scripts, `script`, `scripts[]`. A step does not need a shebang; each one is run with `bash`.

Per-step options (also valid inside templates):

| Option | Effect |
|--------|--------|
| `continueOnError` | The run goes on when this step fails; the Node reports `ok` with `initScriptMessage: optional step(s) failed: ...` and the step shows `fail` in the step list |
| `timeoutSec` | The step is stopped after this many seconds (exit code 124) and counted as failed |
| `labelSelector` | The step runs only on Nodes whose labels match (same syntax as post-deployment commands, e.g. `role=gpu`); other Nodes skip it. Lets one Infra-level script serve groups with different roles |

**Infra and NodeGroup levels.** `initScript` on the Infra applies to every NodeGroup. A NodeGroup `initScript` that does not set `mode`, `userName` or `notify` *extends* the Infra one: its `templateId`, `script` and `scripts` run after the Infra steps and its env overrides. A NodeGroup `initScript` that sets `mode`, `userName` or `notify` *replaces* the Infra one for that group. Steps of a template are named after the template id (`initscript-nginx`, or `initscript-nginx-2` for a multi-step template) unless the template names them.

---

## Script Templates

An init script template stores an ordered list of scripts (`scripts[]`, or `script` for a single one) and its default `env`, so one template can describe a whole profile such as base setup, agent install and application start. Templates in the `system` namespace are shared with every namespace; a template with the same id in the request namespace takes precedence. `make init` loads `init/templates/initscript-*.json` into `system`: `initscript-hello` (writes a marker) and `initscript-nginx` (installs nginx and serves a page identifying the node; pair it with `sgTemplateId: sg-usecase-web` so port 80 is open, as the `infra-usecase-aws-ap-northeast-2-web` template does). `initscript-swap` adds a swap file for Nodes with less than 1 GiB of memory, so a package install next to a running service does not stall them.

```bash
# Create a template in the system namespace (shared)
curl -u default:default -X POST http://localhost:1323/tumblebug/ns/system/template/initScript \
  -H 'Content-Type: application/json' -d '{
    "name": "web-profile",
    "description": "nginx plus the OpenTelemetry collector",
    "scripts": [
      { "name": "nginx", "script": "#!/bin/bash\napt-get install -y -qq nginx\n" },
      { "name": "otel-agent", "script": "#!/bin/bash\ncurl -fsSL https://example.com/otel.sh | OTEL_ENDPOINT=$OTEL_ENDPOINT bash\n" }
    ],
    "env": { "OTEL_ENDPOINT": "https://otel.example.com" }
  }'

# Create an Infra from the across-global template, which references initscript-hello
curl -u default:default -X POST http://localhost:1323/tumblebug/ns/default/infra/template/infra-across-global \
  -H 'Content-Type: application/json' -d '{"name": "demo"}'
```

Updating a template changes Infras and NodeGroups created afterwards only. Template ids follow the usual resource id rules (lower-case letters, digits and `-`).

---

## Variables Available to Scripts

| Variable | Meaning |
|----------|---------|
| `TB_NS_ID`, `TB_INFRA_ID`, `TB_NODEGROUP_ID`, `TB_NODE_ID` | Identity of the Node in CB-Tumblebug |
| `TB_CSP`, `TB_REGION` | Provider and region of the Node |
| `TB_INIT_USER` | Account name of this init script (`cb-user` unless `userName` was set) |
| `TB_NOTIFY_URL`, `TB_NOTIFY_TOPIC` | The Notifier the script reports to (present only when one is registered) |
| `TB_OTEL_ENDPOINT`, `TB_OTEL_TOKEN`, `TB_OTEL_CA_B64` | The [observability stack](observability-otel.md) (present only when one is registered) |
| `TB_ACCELERATOR_TYPE`, `TB_ACCELERATOR_MODEL`, `TB_ACCELERATOR_COUNT` | Accelerator of the Node spec (present only for specs with one, e.g. `gpu`) |
| user `env` | Every key of the merged template and request `env` |

Variables beginning with `TB_` are reserved; the request `env` cannot set them.

---

## Reviewing and Checking Results

**Review before creating.** `POST /ns/{nsId}/infraDynamicReview` resolves the templates and renders the user data with placeholder values. Each Node review gets an `info` line with the mode, user, steps and approximate size, an `error` when a template is missing or the user data exceeds 16 KB, and a `warning` when no Notifier is registered (the Node then cannot report).

**Result on the Node.** A Node that has an init script with reporting on shows `initScriptStatus: pending` until its first report. While steps run it shows `initializing` with `initScriptMessage` such as `initializing: initscript-nginx (1/2)`. Once the VM finishes, the Node carries `ok` or `fail` and, on failure, `initScriptMessage` starting with the failed step name and exit code, followed by the last lines of the log:

```json
{ "id": "g1-1", "initScriptStatus": "fail", "initScriptMessage": "role exit 1: Job for nginx.service failed ..." }
```

**List view.** `GET /ns/{nsId}/infra` (the view MapUI polls for the map) carries `initScriptStatus`, `initScriptMessage` and, while initializing or failed, `initScriptSteps` on each Node. These come from an in-memory index of the reports, so polling a large Infra adds no etcd reads.

**Whole Infra at once.** `GET /ns/{nsId}/infra/{infraId}/initScript` returns the counts and every Node's result with per-step details, without loading the Node records. Use `option=summary` for the counts only and `status=ok|fail|pending|notApplicable` to list one group. A Node is `pending` until its report arrives, `noReport` when it has been Running for longer than `TB_INIT_SCRIPT_NO_REPORT_TIMEOUT` (default `3m`) without any report (user data did not run, or the Notifier is unreachable from that region; a late report still replaces it), and `notApplicable` when its NodeGroup has `mode: none` or the Node never reached Running (Failed, Terminated).

```json
{ "infraId": "web", "total": 100, "notApplicable": 0, "reported": 98, "ok": 97, "fail": 1, "pending": 2,
  "nodes": [
    { "nodeId": "g1-1", "nodeGroupId": "g1", "status": "fail", "exitCode": 1, "durationSec": 42, "reportedAt": "2026-01-01T00:00:10Z",
      "steps": [ { "name": "initscript-hello", "status": "ok", "exitCode": 0, "durationSec": 0 },
                 { "name": "role", "status": "fail", "exitCode": 1, "durationSec": 41 } ],
      "message": "..." },
    { "nodeId": "g1-2", "nodeGroupId": "g1", "status": "pending", "exitCode": 0, "durationSec": 0 } ] }
```

On the VM itself: `/var/lib/tb/init-script.status` holds `ok|fail <exit code> <time>` and `/var/log/tb-init-script.log` the full output.

---

## API Reference

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/ns/{nsId}/template/initScript` | Create a template `{name, description?, scripts[] or script, env?}` |
| `GET` | `/ns/{nsId}/template/initScript` | List templates in a namespace (`filterKeyword` optional) |
| `GET` | `/ns/{nsId}/template/initScript/{templateId}` | Get one template |
| `PUT` | `/ns/{nsId}/template/initScript/{templateId}` | Replace scripts, env and description |
| `DELETE` | `/ns/{nsId}/template/initScript/{templateId}` | Delete one template |
| `DELETE` | `/ns/{nsId}/template/initScript` | Delete all templates in the namespace |

| `GET` | `/ns/{nsId}/infra/{infraId}/initScript` | Counts and per-Node results (`option=summary`, `status=` filter) |

`initScript` itself is a field of the Infra, Infra template and NodeGroup dynamic requests; the Node object exposes `initScript`, `initScriptStatus` and `initScriptMessage`.

**Running a template later over SSH.** Remote commands (`POST /ns/{nsId}/cmd/infra/{infraId}`, with `nodeGroupId`, `nodeId` or `labelSelector`) and post-deployment phases accept `templateId` and/or `scripts` instead of `command`, plus `env`. The steps then run as root with the same `TB_*` variables and rules as at first boot. Use it to apply a template to existing Nodes or to re-run a failed init script without recreating the Node:

```bash
curl -u default:default -X POST http://localhost:1323/tumblebug/ns/default/cmd/infra/web?nodeId=g1-1 \
  -H 'Content-Type: application/json' -d '{"templateId": "initscript-nginx", "env": {"SITE_TITLE": "retry"}}'
```

Such a run reports `event: done` through the Notifier like a first boot, so the Node's `initScriptStatus` and steps follow the latest run. In MapUI, a Node whose init failed shows a **re-run via SSH** button in the Infra card that repeats the Node's own steps; the post-deployment command composer lets a phase run a template instead of commands, and the review dialog can add template or inline steps per NodeGroup.

Plain `command` requests may also carry `env`; it is exported in front of each command.

---

## Limitations and Troubleshooting

- **Linux only.** Windows Nodes keep the CB-Spider default (account and password via the CSP).
- **16 KB limit.** The rendered user data (variables, wrapper and all steps, base64-encoded) must stay under 16 KB. Put large payloads behind a download step rather than inlining them.
- **No report without a Notifier.** The script still runs, but the Node stays `pending`. The status marker on the VM remains the source of truth.
- **Full logs.** The report carries the last lines only; `/var/log/tb-init-script.log` on the Node has everything, and the OTel agent ships that file to the observability stack when one is used.
- **Secrets in `env`.** CB-Tumblebug keeps script bodies, `env` values and the rendered user data out of its own logs, debug output and the remote-command history; they appear as `[N bytes redacted]` or `********`. They are still stored in the Node's blueprint, which the API returns to authenticated users, and user data is plain text on the VM itself. Do not put long-lived secrets in `env`.
- **Reporting is best effort.** Progress reports (one per step start) are sent once; the final report is retried three times; if the Notifier is unreachable from the VM's network, check the Security Group of the Notifier VM (443) and the egress of the Node.
- **`custom` mode and SSH.** CB-Tumblebug does not create an account in `custom` mode, so remote commands use the image's default account; set `nodeUserName` accordingly.
- **Marker already present.** Custom images built from a Node that ran an init script carry `/var/lib/tb/init-script.status`, so the wrapper exits immediately on new VMs. Remove the file before creating the image.
