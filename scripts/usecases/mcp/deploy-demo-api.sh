#!/bin/bash

# Demo REST API (FastAPI) for the MCP usecase (run on K8s control plane)
# A small web-shop API backed by the demo PostgreSQL; later wrapped by an MCP server.
# Prerequisites: deploy-demo-db.sh completed

set -e

NS="mcp-demo"
DB_URI="postgresql://demo:demo1234@postgres:5432/demo"

echo "==== Demo REST API Setup (FastAPI, namespace: ${NS}) ===="

kubectl -n ${NS} get svc postgres > /dev/null 2>&1 || {
    echo "ERROR: postgres service not found in ${NS}. Run deploy-demo-db.sh first."; exit 1; }

# API code lives in a ConfigMap; no image build needed
cat <<'EOF' | kubectl -n mcp-demo apply -f - > /dev/null
apiVersion: v1
kind: ConfigMap
metadata:
  name: demo-api-app
data:
  app.py: |
    import os
    from fastapi import FastAPI, HTTPException
    from pydantic import BaseModel
    import psycopg
    from psycopg.rows import dict_row

    DB = os.environ.get("DATABASE_URI", "postgresql://demo:demo1234@postgres:5432/demo")
    app = FastAPI(title="Demo Shop API", version="1.0.0",
                  description="Sample web-shop API for the CB-Tumblebug MCP usecase")

    def q(sql, params=(), one=False):
        with psycopg.connect(DB, row_factory=dict_row) as c:
            rows = c.execute(sql, params).fetchall()
        return (rows[0] if rows else None) if one else rows

    @app.get("/healthz")
    def healthz():
        q("SELECT 1")
        return {"ok": True}

    @app.get("/products")
    def list_products():
        return q("SELECT * FROM products ORDER BY id")

    @app.get("/products/{pid}")
    def get_product(pid: int):
        r = q("SELECT * FROM products WHERE id=%s", (pid,), one=True)
        if not r:
            raise HTTPException(404, "product not found")
        return r

    @app.get("/customers")
    def list_customers():
        return q("SELECT * FROM customers ORDER BY id")

    @app.get("/orders")
    def list_orders(customer_id: int | None = None):
        sql = """SELECT o.id, o.customer_id, c.name AS customer, o.status, o.order_date,
                        COALESCE(SUM(oi.quantity * oi.unit_price), 0) AS total
                 FROM orders o JOIN customers c ON c.id = o.customer_id
                 LEFT JOIN order_items oi ON oi.order_id = o.id
                 {} GROUP BY o.id, c.name ORDER BY o.id"""
        if customer_id:
            return q(sql.format("WHERE o.customer_id=%s"), (customer_id,))
        return q(sql.format(""))

    @app.get("/orders/{oid}")
    def get_order(oid: int):
        o = q("""SELECT o.id, o.customer_id, c.name AS customer, o.status, o.order_date
                 FROM orders o JOIN customers c ON c.id=o.customer_id WHERE o.id=%s""",
              (oid,), one=True)
        if not o:
            raise HTTPException(404, "order not found")
        o["items"] = q("""SELECT oi.product_id, p.name, oi.quantity, oi.unit_price
                          FROM order_items oi JOIN products p ON p.id=oi.product_id
                          WHERE oi.order_id=%s""", (oid,))
        return o

    class OrderIn(BaseModel):
        customer_id: int
        product_id: int
        quantity: int = 1

    @app.post("/orders", status_code=201)
    def create_order(o: OrderIn):
        with psycopg.connect(DB, row_factory=dict_row) as c:
            with c.transaction():
                p = c.execute("SELECT * FROM products WHERE id=%s FOR UPDATE",
                              (o.product_id,)).fetchone()
                if not p:
                    raise HTTPException(404, "product not found")
                if p["stock"] < o.quantity:
                    raise HTTPException(409, f"insufficient stock ({p['stock']} left)")
                c.execute("UPDATE products SET stock=stock-%s WHERE id=%s",
                          (o.quantity, o.product_id))
                oid = c.execute(
                    "INSERT INTO orders (customer_id, status, order_date) "
                    "VALUES (%s, 'pending', CURRENT_DATE) RETURNING id",
                    (o.customer_id,)).fetchone()["id"]
                c.execute("INSERT INTO order_items VALUES (%s, %s, %s, %s)",
                          (oid, o.product_id, o.quantity, p["price"]))
        return {"order_id": oid, "product": p["name"], "quantity": o.quantity,
                "total": float(p["price"]) * o.quantity, "status": "pending"}
EOF

cat <<'EOF' | kubectl -n mcp-demo apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo-api
spec:
  replicas: 1
  selector:
    matchLabels: { app: demo-api }
  template:
    metadata:
      labels: { app: demo-api }
    spec:
      containers:
        - name: demo-api
          image: python:3.11-slim
          workingDir: /app
          command: ["sh", "-c"]
          args:
            - pip install -q --root-user-action=ignore --disable-pip-version-check
              fastapi uvicorn "psycopg[binary]" &&
              uvicorn app:app --host 0.0.0.0 --port 8000
          env:
            - name: DATABASE_URI
              value: postgresql://demo:demo1234@postgres:5432/demo
          ports:
            - containerPort: 8000
          readinessProbe:
            httpGet: { path: /healthz, port: 8000 }
            initialDelaySeconds: 15
            periodSeconds: 5
          volumeMounts:
            - name: app
              mountPath: /app
      volumes:
        - name: app
          configMap:
            name: demo-api-app
---
apiVersion: v1
kind: Service
metadata:
  name: demo-api
spec:
  selector: { app: demo-api }
  ports:
    - port: 8000
      targetPort: 8000
EOF

echo ""
echo "Waiting for the API to start (pip install at boot; typically 1-2 min)..."
kubectl -n ${NS} rollout status deployment/demo-api --timeout=5m > /dev/null

CLUSTER_IP=$(kubectl -n ${NS} get svc demo-api -o jsonpath='{.spec.clusterIP}')
echo ""
echo "=== Smoke test: GET /products ==="
curl -s --max-time 10 "http://${CLUSTER_IP}:8000/products" | python3 -c "
import sys, json
rows = json.load(sys.stdin)
print(f'  {len(rows)} products; low stock items:')
for r in rows:
    if r['stock'] < 5:
        print(f\"    #{r['id']} {r['name']} — stock {r['stock']}\")"

echo ""
echo "========================================"
echo "SUCCESS: Demo Shop API is running"
echo "========================================"
echo "  In-cluster URL: http://demo-api.${NS}.svc.cluster.local:8000"
echo "  OpenAPI spec:   http://demo-api.${NS}.svc.cluster.local:8000/openapi.json"
echo ""
echo "\$\$CMD[Check Demo API](kubectl -n mcp-demo get pods -l app=demo-api)"
exit 0
