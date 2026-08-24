# Observability — metrics + log-derived interactions

Optional, toggle-able stack for watching **cb-tumblebug / cb-spider / etcd / node** load
during large provisioning runs (1000–10000 VMs), and for correlating that load with the
**request activity** that drives it. Two data sources, one Grafana:

- **Metrics** (Prometheus) — per-pod CPU/memory/network, etcd, node, kube-state — no app change.
- **Logs** (Loki) — turns cb-tumblebug's own log stream into interaction signals across the
  three layers below, so you can see *what was happening* when resources moved.

## Use

```bash
make k-observability-on        # install (metrics-server + kube-prometheus-stack, lean)
make k-port-forward            # also forwards Grafana at http://localhost:3000 when observability is on
make k-observability-off       # remove everything, free the resources
```

Turn it **on** before a big run and **off** afterward — the stack itself competes for CPU/RAM
on the same cluster it observes, so it is deliberately optional and easy to remove.

## What you get

Installed into the `monitoring` namespace (separate Helm release `mon`, decoupled from `k-up`):

- **metrics-server** — restores `kubectl top nodes/pods` (kind needs `--kubelet-insecure-tls`).
- **kube-prometheus-stack** (lean: 24h retention, ephemeral storage, Alertmanager off):
  - **Prometheus** scraping — no app instrumentation needed:
    - **cAdvisor / kubelet** → per-pod CPU / memory / network (cb-tumblebug, cb-spider, …).
    - **kube-state-metrics** → restarts, **OOMKilled**, pending, requests vs limits.
    - **node-exporter** → host CPU / memory / disk / file descriptors.
    - **etcd** → `cb-tumblebug-etcd:2379/metrics` (DB size, fsync latency, leader, proposals).
  - **Grafana** with the bundled dashboards **plus** two auto-provisioned dashboards:
    **“CB-Tumblebug — Control Plane & etcd”** (`dashboards/cb-controlplane.json`) and
    **“CB-Tumblebug — Interactions & Load”** (`dashboards/cb-interactions.json`).
- **loki-stack** (Loki + Promtail, lean, ephemeral) — Promtail tails all pod logs; Loki is
  wired as a Grafana datasource. Powers the Interactions dashboard.

## Interactions dashboard — three layers from cb-tumblebug's own logs

For a write operation (POST/PUT/DELETE; GET is excluded) you can see when work started and
finished at each layer, and overlay it with CPU/memory on the same timeline:

| Layer | Source log | Fields used |
|-------|-----------|-------------|
| Inbound API (→ cb-tumblebug) | zerologger `request` | `Method`, `URI`, `status`, `latency` (end + duration) |
| cb-tumblebug → cb-spider | client `Internal Call Start/OK/Failed` | `Method`, `URI` (`/spider/<op>`), `latency` (ms), `status` |
| Direct CSP SDK (bypasses spider) | csp `Direct SDK Start/OK/Failed` | `provider`, `op`, `region`, `count`, `latency` (ms) |

The direct-SDK layer is emitted by `src/core/csp/observe.go` and covers vmstatus, bulk
control (suspend/resume/terminate/reboot), remediation terminate, tag upsert, AWS pricing,
and Azure image list/get — uniform start/end logs in the same shape as the Spider client.

**Prerequisites (both already set by this deployment):**

- **JSON logs** — cb-tumblebug must run with `TB_LOGFORMAT=json` (helm `tumblebug.logFormat: json`).
  Console output emits ANSI color codes that render as garbage in Loki and are fragile to parse.
- **debug level** — the `*OK` / `*Start` lines are `DBG`; `tumblebug.logLevel: debug` keeps them.
  Failures (`*Failed`) are `WARN` and show at any level.

Resolution note: Prometheus scrapes every 30s, so CPU/memory correlate well with **sustained**
work (provisioning, release, bulk control) but not with a single fast request. Lower
`scrapeInterval` for finer resolution at higher cost.

## Files

- `kube-prometheus-stack.values.yaml` — lean metrics Helm values (+ Loki datasource).
- `loki-stack.values.yaml` — lean Loki + Promtail Helm values.
- `dashboards/cb-controlplane.json` — control-plane & etcd metrics dashboard.
- `dashboards/cb-interactions.json` — log-derived interactions + load dashboard.

## Not yet (fast-follow / Phase 2)

- **postgres** metrics — needs a `postgres_exporter` wired with the DB secret (both
  `cb-tumblebug-postgres` and `cb-spider-postgres`).
- **App-level OTel** in cb-tumblebug/cb-spider — Go runtime (goroutine/GC/heap), DB-pool
  gauges, per-route request rate/latency/errors, provisioning-op counters, and traces.
  Export OTLP → OTel Collector → Prometheus (+ Tempo/Jaeger for traces). Keep app metrics
  **aggregate** (no per-VM labels) to avoid cardinality blow-up at 10k VMs.
