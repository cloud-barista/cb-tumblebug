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
./deploy-registry-db.sh          # PostgreSQL + seeded model catalog (namespace mcp-demo)
./deploy-registry-backend.sh     # FastAPI catalog API (search/get/register/delete)
./deploy-registry-web.sh         # web catalog browser on NodePort 30902
./deploy-mcp-servers.sh          # 2 MCP adapters (catalog tools + read-only SQL), stateless
./deploy-mcp-serving-adapter.sh  # optional 3rd adapter: live KServe InferenceService list
./deploy-agentgateway.sh         # federation gateway on NodePort 30900 (+UI 30901)
./test-mcp-e2e.sh                # scripted demo: search, SQL analytics, register/delete
```

If KServe runs in the same cluster (e.g. built with the KServe usecase), the
optional `serving` target adds `serving_list_served_models` /
`serving_get_served_model`, so an AI can answer questions like *"which catalog
models are not being served right now?"* by joining `registry_*` and
`serving_*` results. Deploy it any time and re-run `deploy-agentgateway.sh` —
the gateway auto-detects it.

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

The gateway exposes **per-consumer route views** from the same backends — the
answer to "won't federating many MCP servers flood the LLM with tools?":

| Route | Tools | Intended consumer |
|---|---|---|
| `/mcp` | all (10) | admin / full-capability agent |
| `/mcp-registry` | `registry_*` + `db_*` (8) | catalog-operations agent |
| `/mcp-serving` | serving view (2, unprefixed) | read-only monitoring agent |

Register different routes in different clients (e.g. `/mcp` in Claude Code,
`/mcp-serving` in VS Code Copilot) and compare their tool lists. Note: a route
with a single target exposes original tool names without the target prefix.

## Live-demo features of the web catalog

The catalog page (30902) is built to make MCP-driven changes visible on stage:

- **Live changes feed** (right panel): every model registered or deleted via
  MCP tools appears as a timestamped event within 5 s; new cards glow with a
  **NEW** badge for 20 s.
- **Detail modal**: click any card — full metadata plus a **serving-method
  panel** showing how that model would be deployed (see below).
- **Serving badges** on every card: `A · no build` / `B · image build` /
  `C · manual`, derived from the model format.

Suggested talk track: ask the LLM to *"register an onnx model called
tomato-ripeness-cnn"* → the card pops into the catalog with NEW + the feed logs
it → click it → the modal explains it would serve with **zero container
builds** → ask the LLM *"how would I serve it?"* → `get_serving_guide` answers
with the same classification.

## Serving-method comparison (A/B/C)

The catalog and the `get_serving_guide` MCP tool classify every model into one
of three serving paths (working demos in `scripts/usecases/kserve/examples/`;
note that not all of them use KServe):

| Method | Container build? | KServe? | You write serving code? | Demo script |
|---|---|---|---|---|
| **A** standard-runtime | **No** | Yes | No — model file + 8-line YAML | `a-sklearn-isvc.sh` |
| **B** custom-image | **Yes** (registry needed) | Yes | Yes (runtime image) | `build-serve-custom-model.sh` |
| **C** plain Deployment | Yes (or inline hack) | **No** | Yes + manual scaling/rollout | `c-plain-deployment.sh` |

Formats `sklearn/xgboost/lightgbm/onnx/tensorflow/pytorch/huggingface` → A,
`custom` → B, anything else → C. Because the mapping lives in the MCP tool
docstrings, the LLM can explain the trade-off unprompted — e.g. ask
*"우리 카탈로그에서 컨테이너 빌드 없이 바로 서빙 가능한 모델만 골라줘"* and it
will reason from formats alone. For the full A/B/C hands-on comparison run the
three example scripts on a KServe cluster (mapui: 🚀 KServe steps 10/11/14).

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
