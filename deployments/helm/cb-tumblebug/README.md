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

## MCP server & agentgateway (optional)

```bash
make k-mcp-on              # enable the TB-MCP server (persistent toggle)
make k-agentgateway-on     # put agentgateway in front of it (enables MCP too)
#   MCP endpoint via gateway: port-forward svc/agentgateway 3000 → http://localhost:3000/mcp
make k-agentgateway-off    # remove the gateway (MCP stays)
make k-mcp-off             # disable both
```

agentgateway runs standalone (static ConfigMap config, no Gateway API/kgateway
required) and proxies the TB-MCP backend; more MCP backends can be federated
by extending the config. TB-MCP runs Streamable HTTP in stateless mode
(MCP spec 2026-07-28 aligned), so both components are replica/LB-friendly.

### MCP endpoint authentication (optional, no IdP required)

```bash
make k-mcp-auth-on    # enable strict JWT auth on the /mcp route (requires agentgateway)
make k-mcp-token      # mint a dev JWT (default TTL 720h; MCP_TOKEN_TTL_HOURS=<n>)
make k-mcp-auth-off   # disable (signing key kept)
```

How it works: an RSA keypair is generated on **your host**
(`~/.cloud-barista/mcp-auth/`, private key never leaves it); only the public
JWKS goes to the cluster and agentgateway verifies `Authorization: Bearer`
tokens in **strict** mode (no/invalid token → 401). `make k-info` switches its
client snippets to the header-included form automatically, e.g. for Claude Code:

```bash
TOKEN=$(make -s k-mcp-token | grep -o 'eyJ[A-Za-z0-9_.-]*')
claude mcp add --transport http cb-tumblebug http://localhost:8080/mcp \
  --header "Authorization: Bearer $TOKEN"
```

Note: MCP clients treat 401 as "start an OAuth flow" — without an IdP that
auto-negotiation fails (VS Code shows redirect URIs to register manually);
configuring the `Authorization` header up front, as above, avoids it entirely.
For an OAuth/IdP
flow later, point `agentgateway.auth` issuer/JWKS at your organization's IdP —
the client side stays the same.

### Connecting LLM clients

Pick an endpoint (after the matching port-forward):

| Path | Endpoint |
|---|---|
| MCP server direct | `http://localhost:8000/mcp` (`port-forward svc/cb-tumblebug-mcp-server 8000:8000`) |
| via agentgateway | `http://localhost:3000/mcp` (`port-forward svc/agentgateway 3000:3000`) |
| via Gateway API entrypoint | `http://localhost:8080/mcp` (`make k-gateway-forward`) |

Then configure the client (all use streamable HTTP):

```jsonc
// VS Code / GitHub Copilot — .vscode/mcp.json
{ "servers": { "cb-tumblebug": { "type": "http", "url": "http://localhost:8000/mcp" } } }
```

```bash
# Claude Code (CLI)
claude mcp add --transport http cb-tumblebug http://localhost:8000/mcp
```

```jsonc
// Cursor — ~/.cursor/mcp.json
{ "mcpServers": { "cb-tumblebug": { "url": "http://localhost:8000/mcp" } } }
```

Claude Desktop cannot connect to remote HTTP servers directly — use the stdio
proxy bridge shipped in `src/interface/mcp/` (`mcp-simple-proxy.py`; see that
directory's README and `claude_desktop_config.json` example).
`make k-info` prints these snippets with the URL matching what is enabled.

**Testing with MCP Inspector** (official MCP debug tool):

```bash
# Web UI: pick "Streamable HTTP" and enter the endpoint URL
npx @modelcontextprotocol/inspector

# Headless smoke check
npx @modelcontextprotocol/inspector --cli http://localhost:8080/mcp --method tools/list
npx @modelcontextprotocol/inspector --cli http://localhost:8080/mcp \
  --method tools/call --tool-name get_namespaces
```

## Gateway API entrypoint (optional)

```bash
make k-gateway-on        # single entrypoint: / mapui, /tumblebug API, /mcp MCP
                         #   (installs Envoy Gateway automatically on kind if missing)
make k-gateway-forward   # http://localhost:8080 (idempotent port-forward)
make k-gateway-off       # remove routes/gateway (controller stays installed)
```

The chart emits implementation-neutral `Gateway`/`HTTPRoute` resources
(`gateway.className` values; any Gateway API implementation works — Ingress is
not used, see ingress-nginx EOL). The `/mcp` route disables the request timeout
for long-lived MCP streams. mapui served through the gateway automatically
calls the API on the same origin (`<origin>/tumblebug`) — no CORS or popup
setup needed; on its canonical port 1324 it keeps the classic host:port model.

**TLS**: set `gateway.tls.enabled=true` for an HTTPS listener (:443 → forwarded
to :8443 by `k-gateway-forward`). Without `tls.secretName` a self-signed cert
is generated once and **reused across upgrades**; point `secretName` at a
cert-manager-issued `kubernetes.io/tls` Secret for trusted certs.
`gateway.hostnames` enables host-based routing (optional — no domain needed).

Note (kind): the gateway Service is `LoadBalancer` and stays `<pending>`
without an LB provider — `Programmed=False` on the Gateway is cosmetic there;
use `make k-gateway-forward`.

## Developing against the cluster (compose `--build` equivalent)

```bash
make k-build C=cb-tumblebug   # build ./ (local source) and run it in the cluster
make k-build C=cb-mapui       # build ../cb-mapui
make k-build C=mcp            # build src/interface/mcp
make k-build-sp               # build ../cb-spider (occasional)
make k-build-off              # back to published images (C=<comp> for one)
```

Each run rebuilds the image, `kind load`s it, pins it via a persistent local
values file (survives `k-up`), and restarts the deployment. kind-only — on
remote clusters push to a registry and set `images.<component>` instead.
For watch-loop workflows consider Skaffold/Tilt on top of the same chart.

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
the vault unrecoverable (reset with `make k-clean`). On shared clusters,
restrict who can read Secrets in this namespace (RBAC) — that Secret is the
key to all stored CSP credentials. `networkPolicy.enabled=true` adds an
ingress lockdown (default-deny + intra-namespace + gateway data plane) on
NetworkPolicy-enforcing CNIs.

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

- OpenBao TLS listener (in-cluster traffic is currently plaintext HTTP) — deferred with the scoped-token item;
  Ingress is not planned (ingress-nginx EOL'd 2026-03; the Ingress API itself
  remains usable with third-party controllers if you wire it manually)
- Metabase dashboard; MCP server is `mcp.enabled: false` by default
- Multi-replica / HA (Phase 3)
