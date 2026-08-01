#!/bin/bash

# Two MCP adapters for the model registry (run on K8s control plane), streamable HTTP on :8000/mcp
#   mcp-model-registry-backend : curated registry tools over the REST API (the write path)
#   mcp-model-registry-db      : read-only SQL access to the registry DB (the analysis path)
# Prerequisites: deploy-registry-db.sh and deploy-registry-backend.sh completed

set -e

NS="mcp-demo"

echo "==== MCP Adapters Setup (namespace: ${NS}) ===="

kubectl -n ${NS} get svc model-registry-backend > /dev/null 2>&1 || {
    echo "ERROR: model-registry-backend service not found in ${NS}. Run deploy-registry-backend.sh first."; exit 1; }

# MCP adapter (backend): curated tools that call the registry REST API
cat <<'EOF' | kubectl -n mcp-demo apply -f - > /dev/null
apiVersion: v1
kind: ConfigMap
metadata:
  name: mcp-model-registry-backend-app
data:
  server.py: |
    import os
    import httpx
    from fastmcp import FastMCP

    API = os.environ.get("API_BASE", "http://model-registry-backend:8000")
    mcp = FastMCP("model-registry")

    def get(path, **params):
        r = httpx.get(f"{API}{path}", params=params or None, timeout=10)
        r.raise_for_status()
        return r.json()

    @mcp.tool
    def search_models(query: str = "", task: str = "") -> list:
        """Search the model catalog by keyword and/or task (empty = all models)."""
        return get("/models", query=query, task=task)

    @mcp.tool
    def get_model(model_id: int) -> dict:
        """Get one model's full metadata by id."""
        return get(f"/models/{model_id}")

    @mcp.tool
    def list_tasks() -> list:
        """List ML task categories with model counts."""
        return get("/tasks")

    @mcp.tool
    def register_model(name: str, task: str, format: str, params_m: float = 0,
                       size_mb: int = 0, license: str = "apache-2.0",
                       gpu_required: bool = False, description: str = "") -> dict:
        """Register a new model in the catalog (metadata only).

        Before calling, collect the required fields from the user and confirm:
        - name: unique, kebab-case (e.g. tomato-ripeness-cnn)
        - task: e.g. image-classification, object-detection, text-generation,
          text-classification, tabular-regression, tabular-classification,
          time-series-forecasting, image-to-image
        - format: one of sklearn, xgboost, lightgbm, onnx, tensorflow, pytorch,
          huggingface, custom (KServe-servable formats)
        Optional: params_m (millions of parameters), size_mb, license
        (default apache-2.0), gpu_required (default false), description.
        Ask the user for any required value they have not provided yet.

        Note: the chosen format determines how the model would later be served —
        standard formats need NO container image build (KServe standard runtime),
        while 'custom' requires building an image. Call get_serving_guide to
        explain the serving path for a model."""
        r = httpx.post(f"{API}/models", timeout=10, json={
            "name": name, "task": task, "format": format, "params_m": params_m,
            "size_mb": size_mb, "license": license, "gpu_required": gpu_required,
            "description": description})
        if r.status_code >= 400:
            return {"error": r.status_code, "detail": r.json().get("detail", r.text)}
        return r.json()

    @mcp.tool
    def delete_model(model_id: int) -> dict:
        """Delete a model from the catalog by id."""
        r = httpx.delete(f"{API}/models/{model_id}", timeout=10)
        if r.status_code >= 400:
            return {"error": r.status_code, "detail": r.json().get("detail", r.text)}
        return r.json()

    STANDARD_FORMATS = ["sklearn", "xgboost", "lightgbm", "onnx",
                        "tensorflow", "pytorch", "huggingface"]

    @mcp.tool
    def get_serving_guide(model_id: int = 0, format: str = "") -> dict:
        """Explain HOW a catalog model would be deployed for inference and
        whether a container image build is required. Pass model_id (preferred)
        or a format string. Use this when the user asks how to serve/deploy a
        model, or to compare serving approaches.

        Three serving methods (demo scripts in scripts/usecases/kserve/examples):
        - A standard-runtime: KServe serves standard formats (sklearn, xgboost,
          lightgbm, onnx, tensorflow, pytorch, huggingface) directly — NO serving
          code, NO container build; model artifact + a small InferenceService YAML.
        - B custom-image: 'custom' format needs a container image built and pushed
          to a registry, then a KServe custom-predictor InferenceService.
        - C plain-deployment: any format can also run WITHOUT KServe as a plain
          K8s Deployment/Service — you write the API server and manage scaling
          and rollouts yourself (most control, most work)."""
        model = None
        if model_id:
            model = get(f"/models/{model_id}")
            format = model.get("format", format)
        if not format:
            return {"error": "provide model_id or format"}
        if format in STANDARD_FORMATS:
            method, kserve, build = "A", True, False
            rationale = (f"'{format}' is a KServe standard-runtime format: the model "
                         "artifact is served as-is — no serving code, no image build.")
            example = "scripts/usecases/kserve/examples/a-sklearn-isvc.sh"
        elif format == "custom":
            method, kserve, build = "B", True, True
            rationale = ("'custom' format has its own runtime/dependencies: build a "
                         "container image, push it to a registry, then serve via a "
                         "KServe custom-predictor InferenceService.")
            example = "scripts/usecases/kserve/examples/build-serve-custom-model.sh"
        else:
            method, kserve, build = "C", False, True
            rationale = (f"'{format}' is not a KServe-known format: serve it as a plain "
                         "K8s Deployment/Service without KServe — hand-written API "
                         "server, manual scaling and rollout.")
            example = "scripts/usecases/kserve/examples/c-plain-deployment.sh"
        return {
            "model": model.get("name") if model else None,
            "format": format,
            "recommended_method": method,
            "uses_kserve": kserve,
            "container_build_required": build,
            "gpu_required": model.get("gpu_required") if model else None,
            "rationale": rationale,
            "example_script": example,
            "alternatives": ("Method C (plain Deployment, no KServe) is always possible "
                             "for full manual control; it trades convenience for effort."),
        }

    # stateless: any replica can serve any request (no per-pod session state)
    mcp.run(transport="http", host="0.0.0.0", port=8000, path="/mcp", stateless_http=True)
EOF

# MCP adapter (db): read-only SQL tools straight to Postgres
cat <<'EOF' | kubectl -n mcp-demo apply -f - > /dev/null
apiVersion: v1
kind: ConfigMap
metadata:
  name: mcp-model-registry-db-app
data:
  server.py: |
    import os
    import json
    import psycopg
    from psycopg.rows import dict_row
    from fastmcp import FastMCP

    DB = os.environ.get("DATABASE_URI",
                        "postgresql://demo:demo1234@model-registry-db:5432/registry")
    # read-only transactions + 5s statement timeout enforced at the connection level
    OPTS = "-c default_transaction_read_only=on -c statement_timeout=5000"
    mcp = FastMCP("model-registry-sql")

    def run(sql, params=()):
        with psycopg.connect(DB, row_factory=dict_row, options=OPTS) as c:
            cur = c.execute(sql, params)
            rows = cur.fetchmany(200) if cur.description else []
        return json.loads(json.dumps(rows, default=str))

    @mcp.tool
    def list_tables() -> list:
        """List tables in the registry database."""
        return run("""SELECT table_name FROM information_schema.tables
                      WHERE table_schema='public' ORDER BY table_name""")

    @mcp.tool
    def describe_table(table: str) -> list:
        """Show the columns and types of a table."""
        return run("""SELECT column_name, data_type, is_nullable
                      FROM information_schema.columns
                      WHERE table_schema='public' AND table_name=%s
                      ORDER BY ordinal_position""", (table,))

    @mcp.tool
    def query(sql: str) -> list:
        """Run a read-only SQL query (writes are rejected; 5s timeout, first 200 rows)."""
        try:
            return run(sql)
        except Exception as e:
            return [{"error": str(e).strip()}]

    # stateless: any replica can serve any request (no per-pod session state)
    mcp.run(transport="http", host="0.0.0.0", port=8000, path="/mcp", stateless_http=True)
EOF

for APP in mcp-model-registry-backend mcp-model-registry-db; do
cat <<EOF | kubectl -n ${NS} apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${APP}
spec:
  replicas: 1
  selector:
    matchLabels: { app: ${APP} }
  template:
    metadata:
      labels: { app: ${APP} }
    spec:
      containers:
        - name: ${APP}
          image: python:3.11-slim
          workingDir: /app
          command: ["sh", "-c"]
          args:
            - pip install -q --root-user-action=ignore --disable-pip-version-check
              "fastmcp<3" httpx "psycopg[binary]" &&
              python3 server.py
          env:
            - name: API_BASE
              value: http://model-registry-backend:8000
            - name: DATABASE_URI
              value: postgresql://demo:demo1234@model-registry-db:5432/registry
          ports:
            - containerPort: 8000
          readinessProbe:
            tcpSocket: { port: 8000 }
            initialDelaySeconds: 15
            periodSeconds: 5
          volumeMounts:
            - name: app
              mountPath: /app
      volumes:
        - name: app
          configMap:
            name: ${APP}-app
EOF
cat <<EOF | kubectl -n ${NS} apply -f -
apiVersion: v1
kind: Service
metadata:
  name: ${APP}
spec:
  selector: { app: ${APP} }
  ports:
    - port: 8000
      targetPort: 8000
EOF
done

echo ""
echo "Waiting for MCP adapters to start (pip install at boot; typically 1-2 min)..."
kubectl -n ${NS} rollout restart deployment/mcp-model-registry-backend deployment/mcp-model-registry-db > /dev/null
kubectl -n ${NS} rollout status deployment/mcp-model-registry-backend --timeout=5m > /dev/null
kubectl -n ${NS} rollout status deployment/mcp-model-registry-db --timeout=5m > /dev/null

# JSON-RPC helper for streamable HTTP (unwraps SSE-framed responses)
mcp_rpc() {
    local URL="$1" SID="$2" BODY="$3"
    local HDRS=(-H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream')
    [ -n "$SID" ] && HDRS+=(-H "mcp-session-id: $SID")
    curl -s --max-time 15 "${HDRS[@]}" "$URL" -d "$BODY" | sed -n 's/^data: //p; /^{/p' | tail -1
}

mcp_check() {
    local NAME="$1"
    kubectl -n ${NS} port-forward svc/${NAME} 18000:8000 > /dev/null 2>&1 &
    local PF_PID=$!
    sleep 3
    local URL="http://127.0.0.1:18000/mcp"
    mcp_rpc "$URL" "" '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"probe","version":"1.0"}}}' \
        | grep -q '"serverInfo"' || { kill ${PF_PID} 2>/dev/null; echo "ERROR: ${NAME} initialize failed"; exit 1; }
    mcp_rpc "$URL" "" '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' | python3 -c "
import sys, json
tools = json.load(sys.stdin)['result']['tools']
print(f'  ${NAME}: {len(tools)} tools -> ' + ', '.join(t['name'] for t in tools))"
    kill ${PF_PID} 2>/dev/null
    wait ${PF_PID} 2>/dev/null || true
}

echo ""
echo "=== Verifying MCP handshake (initialize + tools/list, stateless) ==="
mcp_check mcp-model-registry-backend
mcp_check mcp-model-registry-db

echo ""
echo "========================================"
echo "SUCCESS: Both MCP adapters are serving (streamable HTTP, stateless)"
echo "========================================"
echo "  backend adapter: http://mcp-model-registry-backend.${NS}.svc.cluster.local:8000/mcp"
echo "  db adapter:      http://mcp-model-registry-db.${NS}.svc.cluster.local:8000/mcp"
echo "  Next: deploy-agentgateway.sh federates both behind one external endpoint"
echo ""
echo "\$\$CMD[Check MCP Adapters](kubectl -n mcp-demo get pods | grep mcp-)"
exit 0
