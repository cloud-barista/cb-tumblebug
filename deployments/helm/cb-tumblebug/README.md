# CB-Tumblebug Helm Chart

Deploys the full CB-Tumblebug stack on Kubernetes: cb-tumblebug, cb-spider,
mc-terrarium, cb-mapui, OpenBao, etcd, and two PostgreSQL instances — with the
same topology and service names as the docker-compose deployment.

## Prerequisites

On a fresh machine (e.g., a new VM):

| Tool | Needed for | Note |
|---|---|---|
| kubectl, helm | `make k-up` (always) | any recent version |
| an existing K8s cluster (kubeconfig) — **or** Docker + kind | `make k-up` | with no reachable cluster, `k-up` auto-creates a kind cluster; kind itself requires Docker (`./scripts/installDocker.sh`) |
| python ≥ 3.10, uv, openssl, curl | `make k-init` | same requirements as the compose `make init` |

Credentials are prepared exactly like compose:
`make gen-cred` → edit `~/.cloud-barista/credentials.yaml` → `make enc-cred`.

For a single-VM **persistent** deployment, prefer a lightweight distribution
such as k3s over kind (kind is a dev tool running the cluster inside Docker);
once a kubeconfig exists, the flow below is identical.

### Using an existing cluster (kubeadm etc.)

`make k-up` targets **whatever your current kubeconfig context points at** —
it shows the target and asks for confirmation when the context is not a kind
cluster. Pin a context explicitly with `make k-up K8S_CONTEXT=<context>`
(propagated to every helm/kubectl call of the k-* commands).

Preflight checks will stop/warn you about the two most common existing-cluster
pitfalls:

- **No default StorageClass** (PVCs would stay Pending) — mark one as default,
  or pass explicit classes via
  `HELM_ARGS="--set etcd.storageClassName=<sc> --set tumblebugPostgres.storageClassName=<sc> ..."`.
- **Single-node kubeadm control-plane taint** (pods would stay Pending) —
  `kubectl taint nodes --all node-role.kubernetes.io/control-plane:NoSchedule-`.

On failure, `k-up` automatically prints diagnostics (pending pods, PVC status,
Warning events). `make k-token` creates a **cluster-admin** ServiceAccount —
use it only on dedicated/local clusters.

## Quick start

```bash
# From the repository root (creates a kind cluster if no cluster is reachable):
make k-up

# Register credentials and load assets (same flow as compose):
make k-init          # interactive
MULTI_INIT_PWD=... make k-init ARGS="-y"   # headless

# Access (starts port-forwards; idempotent — restarts stale ones)
make k-port-forward
#   API / Swagger : http://localhost:1323/tumblebug/api
#   MapUI         : http://localhost:1324
```

Or with Helm directly:

```bash
helm upgrade --install cb-tumblebug ./deployments/helm/cb-tumblebug \
  --namespace cb-tumblebug --create-namespace
```

## How the pieces fit

- **OpenBao bootstrap is automatic.** A post-install Job initializes OpenBao
  (shares=1), stores the unseal key and root token in `Secret/openbao-keys`,
  unseals, and enables KV v2 at `secret/`. An `unsealer` sidecar re-unseals
  after every pod restart. cb-tumblebug and mc-terrarium reference the Secret
  as a required env, so they intentionally stay in
  `CreateContainerConfigError` until bootstrap completes.
- **cb-tumblebug runs 1 replica with `strategy: Recreate`** — multi-replica is
  not supported yet (in-process coordination state). Probes use
  `/tumblebug/livez` (liveness) and `/tumblebug/readyz` with
  `TB_READYZ_CHECK_DEPS=true` (readiness incl. etcd/PostgreSQL connectivity).
- **cb-spider volumes follow the filesystem audit**
  (docs/design/k8s-deployment.md §5.4): emptyDir for `log/`, `cache/`, `/tmp`.
  If you use the KT/KTCLASSIC providers, their SG/VPC records are file-only —
  mount PVCs for `cloud-driver-libs/.securitygroup-kt` and `.vpc-kt`.
- **`make k-init`** port-forwards the API and reuses the standard
  init flow; DB restore runs through `ASSETS_PG_BACKEND=kubectl`
  (pod auto-detected via the `app=cb-tumblebug-postgres` label).

## Profiles

- `values.yaml` — dev defaults (kind/minikube, bundled datastores, default creds)
- `values-prod.yaml` — production overlay example (real passwords, resources)

## Operations

```bash
make k-status              # release/pods/services/port-forwards (alias: make k-ps)
make k-down                # uninstall + wait for pod termination, keep data (PVCs)
make k-clean               # full reset incl. PVCs + openbao-keys Secret
make k-token               # admin token file for K8s UIs like Headlamp
                           #   (~/.cloud-barista/k8s-admin.token; cluster-admin, dev only)
```

`Secret/openbao-keys` holds the unseal key and root token — back it up if the
OpenBao data PVC matters to you; losing the Secret while keeping the PVC makes
the vault unrecoverable (reset with `make k-clean`).

## Troubleshooting

- **`kubectl port-forward` fails with "address already in use":** a stale
  port-forward from a previous session is still holding the port — they keep
  listening even after their target pod is gone (e.g., after `make k-clean` /
  `make k-up` recreated the stack). Just run `make k-port-forward` — it stops
  the deployment's old forwards and starts fresh ones (`make k-status` lists
  active forwards; `make k-port-forward-stop` stops them).
- **cb-mapui stuck with no log output (then restarted by probes):** the Parcel
  dev server needs inotify watchers; on shared/dev kernels (kind on WSL2 etc.)
  the default `fs.inotify.max_user_instances=128` can be exhausted. Raise it on
  the node: `sysctl -w fs.inotify.max_user_instances=512`.
- **cb-tumblebug / mc-terrarium in `CreateContainerConfigError`:** normal until
  the `openbao-init` Job stores `Secret/openbao-keys` (startup ordering).

## Not included yet

- External exposure via Gateway API (`Gateway`/`HTTPRoute`) + TLS — Phase 2;
  Ingress is not planned (ingress-nginx EOL'd 2026-03; the Ingress API itself
  remains usable with third-party controllers if you wire it manually)
- Metabase dashboard; MCP server is `mcp.enabled: false` by default
- Multi-replica / HA (Phase 3)
