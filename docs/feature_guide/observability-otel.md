# Observability (OpenTelemetry)

Guide to the metrics and logs pipeline for CB-Tumblebug Nodes: an OTLP endpoint (and Grafana) that CB-Tumblebug provisions or registers, and an OpenTelemetry Collector agent that Nodes install through the [init script](init-script.md) template `initscript-otel-agent`.

## 📑 Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Setting Up the Stack](#setting-up-the-stack)
4. [Sending Data from Nodes](#sending-data-from-nodes)
5. [Dashboards and Viewer Accounts](#dashboards-and-viewer-accounts)
6. [GPU Metrics](#gpu-metrics)
7. [API Reference](#api-reference)
8. [Scale, Security and Limitations](#scale-security-and-limitations)

---

## Overview

CB-Tumblebug keeps one observability stack registration: the OTLP/HTTP endpoint Nodes export to, the bearer token, the CA (for self-signed stacks) and the Grafana URL. Every init script receives it as `TB_OTEL_ENDPOINT`, `TB_OTEL_TOKEN` and `TB_OTEL_CA_B64`; nothing is installed unless the Infra request includes the `initscript-otel-agent` template. Telemetry is tagged with the CB-Tumblebug identity of the Node (`tb.ns`, `tb.infra`, `tb.nodegroup`, `tb.node`, `cloud.provider`, `cloud.region`), so dashboards group by Infra, NodeGroup and Node out of the box.

| Feature | Description |
|---------|-------------|
| Managed stack | `POST /tumblebug/observability/stack`: one VM with nginx (TLS), OTel Collector gateway, Prometheus, Loki, Grafana and provisioned dashboards |
| External endpoint | `PUT /tumblebug/observability`: any OTLP/HTTP endpoint, optionally with its Grafana |
| Agent | OpenTelemetry Collector Contrib: host metrics, journald/file logs, optional Prometheus scrapes and GPU metrics |
| Identity | Resource attributes from `TB_*` so Prometheus labels and Loki stream labels carry `tb_infra`, `tb_node`, ... |
| Mirror | The stack serves the agent packages and installer under `/dl/`, so Nodes never download from GitHub |
| Accounts | Grafana admin (secrets via `showSecrets=true`) and viewer accounts through the reader API |

---

## Architecture

```
Node (otelcol-contrib agent) ──OTLP/HTTP 443, Bearer token──▶ nginx ──▶ OTel Collector gateway ──▶ Prometheus (remote write)
   hostmetrics, journald, filelog,          TLS                 /v1/*        auth, batch                 Loki (native OTLP)
   prometheus (GPU exporter, app ports)                         /dl/*  ──▶ package mirror + install-agent.sh
   resource: tb.ns/tb.infra/tb.nodegroup/tb.node, cloud.*       /      ──▶ Grafana (dashboards: Infra Overview, Node Detail, Node Logs, GPU, vLLM, Ollama)
```

- The managed stack uses a single port, 443: Grafana at `/`, OTLP ingest at `/v1/metrics` and `/v1/logs`, the agent mirror at `/dl/`. The Security Group template `sg-obs` opens 443, 80 (Let's Encrypt validation only) and 22.
- The gateway converts resource attributes to Prometheus labels (`tb_infra`, `tb_node`, ...). Loki indexes `tb.*`, `cloud.*`, `host.name` and `service.name` as stream labels.
- Retention defaults to 15 days for both stores (`retentionDays` of the stack request).

---

## Setting Up the Stack

**Managed (self-signed certificate)**

```bash
curl -u default:default -X POST http://localhost:1323/tumblebug/observability/stack -H 'Content-Type: application/json' -d '{}'
```

Provisions `infra-obs` (default `m5.2xlarge`, 500 GB) in the `system` namespace, installs everything and registers `https://<public IP>` with the stack's CA. Nodes receive the CA automatically; browsers show a certificate warning for Grafana.

**Managed with a DNS name and Let's Encrypt** (requires a Route53 hosted zone registered in CB-Tumblebug):

```bash
curl -u default:default -X POST http://localhost:1323/tumblebug/observability/stack -H 'Content-Type: application/json' \
  -d '{"domainName": "example.com", "recordName": "obs", "retentionDays": 30}'
```

**Existing endpoint**

```bash
curl -u default:default -X PUT http://localhost:1323/tumblebug/observability -H 'Content-Type: application/json' \
  -d '{"otlpEndpoint": "https://otel.example.com", "writeToken": "...", "grafanaUrl": "https://grafana.example.com", "grafanaAdminUser": "admin", "grafanaAdminPassword": "..."}'
```

Grafana admin access is optional; it enables the round-trip test to verify storage and the viewer-account API. The same values can be given at startup with `TB_OBSERVABILITY_*` (see `conf/setup.env` or the Helm `tumblebug.observability` values).

**Check and remove**

```bash
curl -u default:default http://localhost:1323/tumblebug/observability?showSecrets=true   # token and Grafana admin password
curl -u default:default -X POST http://localhost:1323/tumblebug/observability/test       # sends a metric and a log line, queries them back
curl -u default:default -X DELETE 'http://localhost:1323/tumblebug/observability?deleteInfra=true'
```

---

## Sending Data from Nodes

Add the `initscript-otel-agent` template to the Infra (or NodeGroup) request; it is loaded into the `system` namespace by `make init`:

```json
"initScript": { "templateId": "initscript-otel-agent", "env": { "OTEL_INTERVAL": "30s" } }
```

The template is a small bootstrap: it downloads `install-agent.sh` from the stack and runs it. The installer fetches the collector package from the stack's mirror, writes the collector configuration with the Node's identity, enables the service and reports the result like any init script step (`initScriptStatus`). Data appears in Grafana within one collection interval.

| Env | Default | Meaning |
|-----|---------|---------|
| `OTEL_METRICS` | `on` | host metrics (CPU, memory, load, disk, filesystem, network, paging, processes) |
| `OTEL_LOGS` | `on` | journald (when present) and the init script log `/var/log/tb-init-script.log` from its beginning |
| `OTEL_GPU` | `auto` | `auto` installs the GPU exporter when the spec has an accelerator or `nvidia-smi` exists; `on`/`off` force it |
| `OTEL_INTERVAL` | `60s` | collection and scrape interval |
| `OTEL_LOG_PATHS` | | comma-separated file globs to tail, e.g. `/var/log/nginx/*.log` |
| `OTEL_SCRAPE_TARGETS` | | comma-separated `host:port` Prometheus endpoints to scrape, for endpoints the agent does not find on its own |
| `OTEL_DISCOVER` | `auto` | `auto` finds vLLM servers on the Node and scrapes their metrics endpoint, also servers started after the agent; `off` scrapes only what `OTEL_SCRAPE_TARGETS` names |
| `OTEL_DIST` | `auto` | collector build: `auto` installs the trimmed build when the stack has it and nothing is scraped, otherwise `otelcol-contrib`; `trimmed` and `contrib` force one |
| `OTEL_PACKAGE_URL` | stack `/dl` | base URL of the package mirror, for example a CDN in front of a large fleet |

**Collector build.** The stack builds a trimmed collector, `otelcol-tb`, that holds only the components the agent uses. Measured against `otelcol-contrib` it is 15 MB instead of 114 MB to download and about 45 MiB instead of 90 MiB of memory on a Node. Nodes that scrape Prometheus endpoints (the GPU exporter, `OTEL_SCRAPE_TARGETS` or a vLLM server the agent found) keep `otelcol-contrib`, which has the Prometheus receiver, and so do Nodes of a stack whose build step failed. Downloads are retried when a transfer drops or stalls and are checked against the mirror's `SHA256SUMS`.

**vLLM discovery.** The agent finds a vLLM server on the Node (`vllm serve`, or the `vllm.entrypoints.openai.api_server` module) and scrapes its metrics endpoint without being told the address. A small service installed with the agent, `tb-otel-discover` (a shell loop of about 4 MiB), looks once a minute and re-applies the agent when a server starts answering or its process ends. A server installed long after the agent is picked up within about a minute of answering, and one that is loading a model or restarting is kept. On a Node that runs the trimmed build this switches the collector to `otelcol-contrib`, and back when the server is gone. Only the port is read from the server's arguments. A server in a container is found when its port is published on the same host port; name any other endpoint in `OTEL_SCRAPE_TARGETS`. A Node whose agent was installed before discovery existed gets it when the template runs again over SSH, with the same `env` as before if any was set:

```bash
curl -u default:default -X POST 'http://localhost:1323/tumblebug/ns/default/cmd/infra/infra01?nodeId=g1-1' \
  -H 'Content-Type: application/json' -d '{"templateId": "initscript-otel-agent"}'
```

Combine it with other steps as usual; putting it first means the agent is already running while later steps install software.

On a Node with less than 1 GiB of memory do the opposite. A package install next to the running agent can exhaust memory: on a 0.5 GiB Node without swap the install step stalled for ten minutes or more, while the same steps in the other order finished in under a minute. Put the install steps first, or add swap with the `initscript-swap` template. The review warns when a small spec has an install step after the agent step.

```json
"initScript": { "templateId": "initscript-otel-agent", "scripts": [ { "name": "app", "script": "..." } ] }
```

Without a registered stack the step fails with a clear message (`TB_OTEL_ENDPOINT is empty`) and the rest of the run stops, so register the stack first.

---

## Dashboards and Viewer Accounts

The managed stack provisions six dashboards in the Grafana folder **CB-Tumblebug** (sources: `src/core/embedded/dashboards`):

- **TB Infra Overview** (`/d/tb-infra-overview`): at-a-glance stats (nodes reporting, CPU, memory, root filesystem, load per CPU, network, log lines, journal errors), top-20 time series for CPU, memory, load, processes, disk busy/throughput/IOPS, network receive/transmit, errors and TCP connections, and a Node table with colored gauges whose Node names link to the detail and log views. Filtered by namespace, Infra, NodeGroup and Node.
- **TB Node Detail** (`/d/tb-node-detail`): one Node in depth: CPU by state, memory by state, load against CPU count, processes by status, filesystem use per mount point, per-device disk busy, queue, throughput and IOPS, per-interface throughput, packets, errors and drops, TCP connections by state, paging, and the Node's journal and init script log.
- **TB Node Logs** (`/d/tb-node-logs`): lines per minute by Node and by level (journal priority mapped to error, warning, info, debug), noisiest units and identifiers, journal errors by Node, then the journal (formatted as `node identifier[pid]: message`) and the init script and file logs, with text and unit filters.
- **TB GPU** (`/d/tb-gpu`): tiles for GPU and Node counts, average utilization, idle GPUs, memory in use, the hottest GPU, total power and exporter failures; one row per GPU with utilization and memory bars, temperature, power and the time it last reported; fleet trends for utilization, memory, temperature, power against its limit and the number of GPUs per load band. The `Per GPU` row opens a chart for each GPU, and `Details` holds exporter failures, encoder and decoder use, the SM clock and fan speed. The table lists the last values within the time range, so a GPU that has stopped reporting stays visible.
- **TB vLLM** (`/d/tb-vllm`): servers reporting, requests running and waiting, requests and output tokens per second, time to first token, KV cache use and failed requests; one row per server and model; trends for requests, tokens, latency percentiles, where the request time goes and the prefix cache. vLLM serves Prometheus metrics on its API port, and the agent finds the server on its own (vLLM discovery above).
- **TB Ollama** (`/d/tb-ollama`): requests, failures, request time, generation speed, output tokens and model loads, read from the journal of `ollama.service`. Ollama has no metrics endpoint, so nothing is scraped: the agent's journal logs are enough when Ollama runs as its systemd service (the default install). Requests are not attributed to a model, because the access log does not name it.

MapUI shows **📈 Metrics** and **📜 Logs** links on the Infra Status card when a stack with Grafana is registered, opening these dashboards with the Infra preselected.

Viewer accounts (`POST /tumblebug/observability/reader`) are Grafana users with the Viewer role; the password is returned once. MapUI has the same form under **Observability (OpenTelemetry)** in the Resource Provisioning menu.

---

## GPU Metrics

When the Node's spec reports an accelerator (`acceleratorType`), CB-Tumblebug exports `TB_ACCELERATOR_TYPE`, `TB_ACCELERATOR_MODEL` and `TB_ACCELERATOR_COUNT` to the init script. With `OTEL_GPU=auto` the installer adds an nvidia-smi based exporter (served from the stack mirror) and a Prometheus scrape of `127.0.0.1:9835`. The exporter is installed even when the driver is not there yet; metrics start as soon as `nvidia-smi` works, so installing the driver later over SSH (post-deployment commands) is fine. DCGM-level metrics (NVLink, ECC, MIG) are not covered by this exporter.

---

## API Reference

All endpoints are global (not namespaced) and require the CB-Tumblebug API credentials.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/tumblebug/observability` | Registered stack (204 when none). Secrets masked unless `showSecrets=true` |
| `PUT` | `/tumblebug/observability` | Register an existing OTLP endpoint (`otlpEndpoint`, `writeToken?`, `caCert?`, `grafanaUrl?`, `grafanaAdminUser?`, `grafanaAdminPassword?`). URLs must be `http(s)://host[:port][/path]` without credentials, query or fragment; link-local and metadata addresses are rejected |
| `DELETE` | `/tumblebug/observability` | Unregister. `deleteInfra=true` also deletes the managed Infra and its resources |
| `POST` | `/tumblebug/observability/stack` | Provision from template and register. Body: `nsId`, `templateId`, `name`, `domainName`, `recordName`, `retentionDays` (all optional) |
| `POST` | `/tumblebug/observability/test` | Send a test metric and log line; verify through Grafana when admin access is set (`timeoutSec`) |
| `POST` | `/tumblebug/observability/reader` | Create a Grafana viewer `{name, password?}` |
| `GET` | `/tumblebug/observability/reader` | List viewer accounts created through this API |
| `DELETE` | `/tumblebug/observability/reader/{name}` | Remove a viewer account |

---

## Scale, Security and Limitations

- **Scale.** Host metrics at a 60 s interval are a few hundred series per Node; the default stack (8 vCPU, 32 GB, 500 GB) is sized for a few thousand Nodes. For tens of thousands, raise the interval, give the VM more memory, or point the gateway at a larger store: the agents only know the endpoint and token, so the backend can be replaced without touching Nodes. Packages come from the stack's mirror, so a large rollout does not depend on GitHub.
- **Security.** Ingest requires the bearer token; Grafana needs a login; the only open ports are 443 and 22. Self-signed stacks store the CA in CB-Tumblebug and pass it to Nodes; `showSecrets=true` reveals the token and the Grafana admin password to API users.
- **Network.** Nodes need outbound HTTPS to the stack; the stack needs outbound internet during installation (GitHub releases, apt.grafana.com).
- **Linux only**, like init scripts. The agent is the OpenTelemetry Collector Contrib package for amd64 and arm64 (deb and rpm).
- **Autoscaling policies** that used the removed CB-Dragonfly metrics are not yet wired to this stack; they report "no metric source" until that is done.
