#!/bin/bash

# Two demo MCP servers (run on K8s control plane), both streamable HTTP on :8000/mcp
#   mcp-api : wraps the Demo Shop REST API as curated MCP tools (the write path)
#   mcp-db  : exposes read-only SQL access to the demo PostgreSQL (the analysis path)
# Prerequisites: deploy-demo-db.sh and deploy-demo-api.sh completed

set -e

NS="mcp-demo"

echo "==== Demo MCP Servers Setup (namespace: ${NS}) ===="

kubectl -n ${NS} get svc demo-api > /dev/null 2>&1 || {
    echo "ERROR: demo-api service not found in ${NS}. Run deploy-demo-api.sh first."; exit 1; }

# MCP server (API): curated tools that call the REST API — the governed write path
cat <<'EOF' | kubectl -n mcp-demo apply -f - > /dev/null
apiVersion: v1
kind: ConfigMap
metadata:
  name: mcp-api-app
data:
  server.py: |
    import os
    import httpx
    from fastmcp import FastMCP

    API = os.environ.get("API_BASE", "http://demo-api:8000")
    mcp = FastMCP("demo-shop-api")

    def get(path, **params):
        r = httpx.get(f"{API}{path}", params=params or None, timeout=10)
        r.raise_for_status()
        return r.json()

    @mcp.tool
    def list_products() -> list:
        """List all shop products with category, price, and current stock."""
        return get("/products")

    @mcp.tool
    def get_product(product_id: int) -> dict:
        """Get one product by id."""
        return get(f"/products/{product_id}")

    @mcp.tool
    def list_customers() -> list:
        """List all customers."""
        return get("/customers")

    @mcp.tool
    def list_orders(customer_id: int | None = None) -> list:
        """List orders with totals, optionally filtered by customer id."""
        return get("/orders", **({"customer_id": customer_id} if customer_id else {}))

    @mcp.tool
    def get_order(order_id: int) -> dict:
        """Get one order including its line items."""
        return get(f"/orders/{order_id}")

    @mcp.tool
    def create_order(customer_id: int, product_id: int, quantity: int = 1) -> dict:
        """Create an order (checks and decrements stock). The only write path in this demo."""
        r = httpx.post(f"{API}/orders", timeout=10, json={
            "customer_id": customer_id, "product_id": product_id, "quantity": quantity})
        if r.status_code >= 400:
            return {"error": r.status_code, "detail": r.json().get("detail", r.text)}
        return r.json()

    mcp.run(transport="http", host="0.0.0.0", port=8000, path="/mcp")
EOF

# MCP server (DB): read-only SQL tools straight to Postgres — the analysis path
cat <<'EOF' | kubectl -n mcp-demo apply -f - > /dev/null
apiVersion: v1
kind: ConfigMap
metadata:
  name: mcp-db-app
data:
  server.py: |
    import os
    import json
    import psycopg
    from psycopg.rows import dict_row
    from fastmcp import FastMCP

    DB = os.environ.get("DATABASE_URI", "postgresql://demo:demo1234@postgres:5432/demo")
    # read-only transactions + 5s statement timeout enforced at the connection level
    OPTS = "-c default_transaction_read_only=on -c statement_timeout=5000"
    mcp = FastMCP("demo-postgres")

    def run(sql, params=()):
        with psycopg.connect(DB, row_factory=dict_row, options=OPTS) as c:
            cur = c.execute(sql, params)
            rows = cur.fetchmany(200) if cur.description else []
        return json.loads(json.dumps(rows, default=str))

    @mcp.tool
    def list_tables() -> list:
        """List tables in the demo database."""
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

    mcp.run(transport="http", host="0.0.0.0", port=8000, path="/mcp")
EOF

for APP in mcp-api mcp-db; do
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
              value: http://demo-api:8000
            - name: DATABASE_URI
              value: postgresql://demo:demo1234@postgres:5432/demo
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
---
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
echo "Waiting for MCP servers to start (pip install at boot; typically 1-2 min)..."
kubectl -n ${NS} rollout status deployment/mcp-api --timeout=5m > /dev/null
kubectl -n ${NS} rollout status deployment/mcp-db --timeout=5m > /dev/null

# JSON-RPC helper for streamable HTTP (unwraps SSE-framed responses)
mcp_rpc() {
    local URL="$1" SID="$2" BODY="$3"
    local HDRS=(-H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream')
    [ -n "$SID" ] && HDRS+=(-H "mcp-session-id: $SID")
    curl -s --max-time 15 "${HDRS[@]}" "$URL" -d "$BODY" | sed -n 's/^data: //p; /^{/p' | tail -1
}

mcp_check() {
    local NAME="$1"
    local URL="http://$(kubectl -n ${NS} get svc ${NAME} -o jsonpath='{.spec.clusterIP}'):8000/mcp"
    local SID
    SID=$(curl -si --max-time 15 "$URL" \
        -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' \
        -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"probe","version":"1.0"}}}' \
        | grep -i '^mcp-session-id:' | tr -d '\r' | awk '{print $2}')
    [ -n "$SID" ] || { echo "ERROR: ${NAME} initialize failed"; exit 1; }
    mcp_rpc "$URL" "$SID" '{"jsonrpc":"2.0","method":"notifications/initialized"}' > /dev/null
    mcp_rpc "$URL" "$SID" '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' | python3 -c "
import sys, json
tools = json.load(sys.stdin)['result']['tools']
print(f'  ${NAME}: {len(tools)} tools -> ' + ', '.join(t['name'] for t in tools))"
}

echo ""
echo "=== Verifying MCP handshake (initialize + tools/list) ==="
mcp_check mcp-api
mcp_check mcp-db

echo ""
echo "========================================"
echo "SUCCESS: Both MCP servers are serving (streamable HTTP)"
echo "========================================"
echo "  mcp-api: http://mcp-api.${NS}.svc.cluster.local:8000/mcp"
echo "  mcp-db:  http://mcp-db.${NS}.svc.cluster.local:8000/mcp"
echo "  Next: deploy-agentgateway.sh federates both behind one external endpoint"
echo ""
echo "\$\$CMD[Check MCP Servers](kubectl -n mcp-demo get pods -l 'app in (mcp-api,mcp-db)')"
exit 0
