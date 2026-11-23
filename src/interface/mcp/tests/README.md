# MCP server checks

Three scripts, from cheapest to most real. None of them is run by CI yet; run them by hand.

| Command | What it proves | Needs | Cost |
|---|---|---|---|
| `make mcp-check` | The tool definitions are coherent and every request matches the REST API | Docker | none |
| `make mcp-scenarios` | The deployed endpoint answers: discovery, a reviewed plan, refusal gates, metadata round trips | A running cluster, `make k-port-forward` | none |
| `make mcp-scenarios ARGS="--spend"` | A real Infra can be created, operated and removed through MCP alone | CSP credentials | a few cents |
| `make mcp-scenarios ARGS="--spend --only services"` | Load balancers, a custom image and a managed database work around an Infra | CSP credentials | under a dollar, 20 minutes |
| `make mcp-scenarios ARGS="--spend --only kubernetes"` | A managed Kubernetes cluster can be created, used and removed | CSP credentials | under a dollar, 30 minutes |
| `make mcp-bench` | What a session pays in bytes for the common calls | A running cluster | none |

## `check_contract.py` - no cluster, seconds

Runs inside the MCP image (`fastmcp` and friends are there), imports `tb-mcp.py` and checks
the tool registry against the repository.

| Check | What fails it |
|---|---|
| `golden` | A tool or parameter disappeared, or appeared, without `tools.golden.json` being updated |
| `requests` | A tool sends a path, method, query name or body key that `swagger.yaml` does not declare; or a tool has no sample call |
| `read-only` | A tool announced as read-only sends something that changes state; `MCP_READ_ONLY` publishes a writing tool or lets a write through |
| `budget` | An over-budget creation goes through, an approval works twice or covers a larger plan |
| `journal` | A refusal or a read is journaled, or a credential reaches the journal |
| `schema-size` | `tools/list` grows past 90 KB, or one tool past 4 KB |
| `param-docs` | A parameter has no description, or the docstring describes one that does not exist |
| `stale-refs`, `prompts` | Prose or a prompt names a tool that does not exist |
| `scripts` | A predefined script has no commands, or downloads a file that is not under `scripts/` |
| `shadowed`, `undefined`, `internal-calls`, `secrets`, `errors`, `duplicates`, `annotations` | Structural mistakes in `tb-mcp.py` |

The request check works by calling every tool against a recording HTTP layer that answers
from the response schemas in `swagger.yaml`, so no server is involved. Each tool needs at
least one entry in `SAMPLES`; a new tool fails the check until it has one. When a sample
needs a field the API does not document, the fix is usually the handler's Swagger
annotation, followed by `make swag`.

```bash
make mcp-check                       # verify
make mcp-check ARGS=--requests       # also print every request each tool sends
make mcp-check-update                # rewrite tools.golden.json after an intended change
```

Read a failure before regenerating the golden file: `--update` is for an intended change.

## `scenarios.py` - the deployed endpoint

Drives the MCP endpoint (`MCP_URL`, default `http://localhost:8080/mcp`; a token is minted
with `make k-mcp-token` when auth is on) the way an agent would, and checks what comes back.

- `discovery`  server instructions, tool list, prompts, usage guides, the cheap reads
- `plan`       spec, image, spec options, pair review, alternative spec, a reviewed plan
- `gates`      every refusal code, idempotent retry, namespace and template round trips, the journal
- `existing`   reads of the Infras already in the namespace
- `lifecycle`  only with `--spend`: create one node, run commands, transfer a file, move
  an object through a bucket with presigned URLs, label, open and close a port, scale out
  by one, delete that node, save as a template, terminate, release. It terminates in a
  `finally`, and refuses to start above `MAX_USD_PER_HOUR`.
- `services`   only with `--spend --only services`: two web nodes, a regional NLB that
  serves the page, a global NLB that serves the page, a custom image and a node started
  from it, a managed database queried from a node over the private network
- `kubernetes` only with `--spend --only kubernetes`: cluster, node group, kubeconfig,
  resize, the release guard, delete

`MCP_TEST_PROVIDER` / `MCP_TEST_REGION` (default `aws` / `ap-northeast-2`) choose where the
plan is made. The run ends with the tools it did not exercise.

Everything a run creates is named `mcpt-...`. Two runs against one cluster at the same
time must not share that prefix - one would terminate the other's Infra during cleanup:
give each its own with `MCP_TEST_PREFIX`. A run that was killed leaves its Infra behind;
the next run with the same prefix removes it before it starts.

## `bench.py` - response sizes

Measures the whole JSON-RPC response over the wire against `baseline.json`.
`BENCH_INFRA=<id>` adds the reads of an existing Infra. `--save-baseline` records the
current numbers.

## `mcp_client.py`

The minimal Streamable HTTP client the two scripts above share. Standard library only.
