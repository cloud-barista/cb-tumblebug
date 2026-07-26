# MCP + agentgateway demo usecase: AI-operated model registry

A Hugging Face-style **model registry** (metadata catalog) running on K8s,
exposed to AI clients through the **Model Context Protocol (MCP)** and federated
behind a single external endpoint with [agentgateway](https://agentgateway.dev).
Demo flow: browse the registry web page → connect an AI client (Claude Code,
MCP Inspector) → let the AI search / analyze / register / delete models →
watch the web page update live.

```
[Browser]                    [External MCP client: Claude Code / Inspector]
   │ :30902                        │ :30900  streamable HTTP /mcp
   ▼                               ▼
┌─ K8s namespace: mcp-demo ──────────────────────────────────────┐
│  model-registry-web (nginx)   agentgateway (admin UI :30901)   │
│    │ /api proxy                 ├─ target "registry" ─┐        │
│    ▼                            ▼                     │        │
│  model-registry-backend ◀── mcp-model-registry-backend│        │
│   (FastAPI: model CRUD)       (MCP adapter, write path)│       │
│    │                            ┌─ target "db" ◀──────┘        │
│    ▼                            ▼                              │
│  model-registry-db ◀───── mcp-model-registry-db                │
│   (PostgreSQL + catalog)      (MCP adapter, read-only SQL)     │
└────────────────────────────────────────────────────────────────┘
```

**Demo story**: both MCP adapters reach the *same* catalog through two governed
paths — `registry_*` tools are curated API wrappers (the only write path, with
duplicate checks), `db_*` tools give raw ad-hoc SQL but are read-only by
policy. agentgateway federates both so the client sees one tool list
(`registry_search_models`, `registry_register_model`, `db_query`, ...) on one
endpoint. The seeded catalog covers the KServe-servable formats (sklearn,
xgboost, lightgbm, onnx, tensorflow, pytorch, huggingface) with CPU-friendly
agriculture models plus GPU LLM entries.

## Quickstart (on the K8s control plane)

```bash
./deploy-registry-db.sh        # PostgreSQL + seeded model catalog (namespace mcp-demo)
./deploy-registry-backend.sh   # FastAPI catalog API (search/get/register/delete)
./deploy-registry-web.sh       # web catalog browser on NodePort 30902
./deploy-mcp-servers.sh        # 2 MCP adapters (catalog tools + read-only SQL), stateless
./deploy-agentgateway.sh       # federation gateway on NodePort 30900 (+UI 30901)
./test-mcp-e2e.sh              # scripted demo: search, SQL analytics, register/delete
```

No GPU needed; CPU nodes are enough. All steps are available as the
**"MCP (agentgateway)"** category in CB-MapUI's remote command popup
(cluster build steps 1-4, registry + MCP stack steps 5-11).

## Connect from your machine

Allow inbound `30900/tcp` and `30902/tcp` (plus `30901/tcp` for the gateway UI)
in the Security Group, then:

```bash
# Web catalog
open http://<node-public-ip>:30902

# Claude Code
claude mcp add --transport http model-registry http://<node-public-ip>:30900/mcp
# then ask e.g.: "Find CPU-only agriculture models, then register a new
#   onnx model called tomato-ripeness-cnn and show it in the catalog"

# MCP Inspector (browser UI; use the token URL printed at startup)
npx @modelcontextprotocol/inspector   # transport: Streamable HTTP, URL as above
```

## Notes

- Component versions: agentgateway `v1.3.1` (standalone mode, static config),
  `postgres:16`, `nginx:1.27-alpine`, FastMCP 2.x.
- The MCP adapters are FastMCP apps with code in ConfigMaps (no image build)
  and run with `stateless_http=True`, so they can be scaled to multiple
  replicas freely (session-stateful MCP servers would need sticky routing).
  The gateway itself is also replica-safe: it encodes backend session info in
  the client token instead of storing state.
- With a static config file the agentgateway admin UI runs in read-only "dump"
  mode: the sidebar only offers Home, Traffic (Listeners/Routes), and CEL
  Playground; the "Enable LLM/MCP" wizard buttons and the MCP Tool Playground
  exist only in UI-managed bootstrap mode (running without `-f`). Our
  federation appears under **Traffic → Routes**. For browser-based live tool
  calls, use MCP Inspector instead.
- For production Postgres access consider
  [postgres-mcp](https://github.com/crystaldba/postgres-mcp) (Postgres MCP Pro).
- The demo endpoints have **no authentication** — they are meant for
  short-lived demo infra. For real use, add an agentgateway JWT/authorization
  policy and TLS, or keep the ports closed and tunnel.
- Cleanup: `kubectl delete namespace mcp-demo`.
