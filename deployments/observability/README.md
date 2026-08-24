# Observability (metrics) — Phase 1

Optional, toggle-able metrics stack for watching **cb-tumblebug / cb-spider / etcd / node**
load during large provisioning runs (1000–10000 VMs). Metrics only — no app code change
required; app-level OpenTelemetry (request/op traces, DB-pool, Go runtime) is a later phase.

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
  - **Grafana** with the bundled dashboards **plus** the auto-provisioned
    **“CB-Tumblebug — Control Plane & etcd”** dashboard (`dashboards/cb-controlplane.json`).

## Files

- `kube-prometheus-stack.values.yaml` — lean Helm values.
- `dashboards/cb-controlplane.json` — custom dashboard, provisioned as a labeled ConfigMap.

## Not yet (fast-follow / Phase 2)

- **postgres** metrics — needs a `postgres_exporter` wired with the DB secret (both
  `cb-tumblebug-postgres` and `cb-spider-postgres`).
- **App-level OTel** in cb-tumblebug/cb-spider — Go runtime (goroutine/GC/heap), DB-pool
  gauges, per-route request rate/latency/errors, provisioning-op counters, and traces.
  Export OTLP → OTel Collector → Prometheus (+ Tempo/Jaeger for traces). Keep app metrics
  **aggregate** (no per-VM labels) to avoid cardinality blow-up at 10k VMs.
