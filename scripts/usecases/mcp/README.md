# MCP + agentgateway demo usecase

Expose in-cluster data services as **Model Context Protocol (MCP)** servers and
federate them behind a single external endpoint with
[agentgateway](https://agentgateway.dev) — so any MCP client (Claude Code,
MCP Inspector, IDEs) can use them remotely.

```
[External MCP client: Claude Code / MCP Inspector]
        │  streamable HTTP   http://<node-public-ip>:30900/mcp
        ▼
┌─ K8s namespace: mcp-demo ─────────────────────────────────┐
│  agentgateway (NodePort 30900, admin UI 30901)            │
│    ├─ target "api" → mcp-api :8000/mcp                    │
│    │                   └─ REST → demo-api (FastAPI)       │
│    └─ target "db"  → mcp-db  :8000/mcp                    │
│                        └─ read-only SQL ↘                 │
│                              postgres (+ web-shop data)   │
└───────────────────────────────────────────────────────────┘
```

**Demo story**: both MCP servers reach the *same* PostgreSQL data through two
governed paths — `api_*` tools are curated REST wrappers (the only write path,
with stock checks), `db_*` tools give raw ad-hoc SQL but are read-only by
policy. agentgateway federates both so the client sees one tool list
(`api_list_products`, `db_query`, ...) on one endpoint.

## Quickstart (on the K8s control plane)

```bash
./deploy-demo-db.sh        # PostgreSQL + sample web-shop data (namespace mcp-demo)
./deploy-demo-api.sh       # FastAPI shop API backed by the DB
./deploy-mcp-servers.sh    # MCP server (API) + MCP server (DB), streamable HTTP
./deploy-agentgateway.sh   # federation gateway on NodePort 30900 (+UI 30901)
./test-mcp-e2e.sh          # scripted demo: tools/list, SQL analysis, write governance
```

No GPU needed; CPU nodes are enough. All steps are available as the
**"MCP (agentgateway)"** category in CB-MapUI's remote command popup
(cluster build steps 1-4, MCP stack steps 5-10).

## Connect from your machine

Allow inbound `30900/tcp` (and `30901/tcp` for the UI) in the Security Group, then:

```bash
# Claude Code
claude mcp add --transport http shop-demo http://<node-public-ip>:30900/mcp
# then ask e.g.: "Find products with stock below 5 and show their recent orders"

# MCP Inspector (browser UI)
npx @modelcontextprotocol/inspector   # transport: Streamable HTTP, URL as above
```

The agentgateway admin UI (`http://<node-public-ip>:30901/ui/`) shows the
federated targets and tool list.

## Notes

- Component versions: agentgateway `v1.3.1` (standalone mode, static config),
  `postgres:16`, FastMCP 2.x.
- The MCP servers are FastMCP apps with code in ConfigMaps (no image build).
  For production Postgres access consider
  [postgres-mcp](https://github.com/crystaldba/postgres-mcp) (Postgres MCP Pro).
- The demo endpoint has **no authentication** — it is meant for short-lived
  demo infra. For real use, add an agentgateway JWT/authorization policy and
  TLS, or keep the port closed and tunnel.
- Cleanup: `kubectl delete namespace mcp-demo`.
