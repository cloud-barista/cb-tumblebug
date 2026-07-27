#!/bin/bash

# Model Registry backend (FastAPI) for the MCP usecase (run on K8s control plane)
# Hugging Face-style model catalog API: search, get, register, delete (metadata only).
# Prerequisites: deploy-registry-db.sh completed

set -e

NS="mcp-demo"

echo "==== Model Registry Backend Setup (FastAPI, namespace: ${NS}) ===="

kubectl -n ${NS} get svc model-registry-db > /dev/null 2>&1 || {
    echo "ERROR: model-registry-db service not found in ${NS}. Run deploy-registry-db.sh first."; exit 1; }

# API code lives in a ConfigMap; no image build needed
cat <<'EOF' | kubectl -n mcp-demo apply -f - > /dev/null
apiVersion: v1
kind: ConfigMap
metadata:
  name: registry-backend-app
data:
  app.py: |
    import os
    from fastapi import FastAPI, HTTPException
    from pydantic import BaseModel
    import psycopg
    from psycopg.rows import dict_row

    DB = os.environ.get("DATABASE_URI",
                        "postgresql://demo:demo1234@model-registry-db:5432/registry")
    app = FastAPI(title="Model Registry API", version="1.0.0",
                  description="Hugging Face-style model catalog for the CB-Tumblebug MCP usecase")

    def q(sql, params=(), one=False):
        with psycopg.connect(DB, row_factory=dict_row) as c:
            rows = c.execute(sql, params).fetchall()
        return (rows[0] if rows else None) if one else rows

    @app.get("/healthz")
    def healthz():
        q("SELECT 1")
        return {"ok": True}

    @app.get("/models")
    def search_models(query: str = "", task: str = "", format: str = ""):
        sql = """SELECT * FROM models
                 WHERE (name ILIKE %s OR description ILIKE %s)
                   AND (%s = '' OR task = %s) AND (%s = '' OR format = %s)
                 ORDER BY downloads DESC"""
        like = f"%{query}%"
        return q(sql, (like, like, task, task, format, format))

    @app.get("/models/{mid}")
    def get_model(mid: int):
        r = q("SELECT * FROM models WHERE id=%s", (mid,), one=True)
        if not r:
            raise HTTPException(404, "model not found")
        return r

    @app.get("/tasks")
    def list_tasks():
        return q("""SELECT task, count(*) AS models FROM models
                    GROUP BY task ORDER BY models DESC""")

    FORMATS = ["sklearn", "xgboost", "lightgbm", "onnx", "tensorflow",
               "pytorch", "huggingface", "custom"]

    class ModelIn(BaseModel):
        name: str
        task: str
        format: str
        params_m: float = 0
        size_mb: int = 0
        license: str = "apache-2.0"
        gpu_required: bool = False
        description: str = ""

    @app.post("/models", status_code=201)
    def register_model(m: ModelIn):
        if m.format not in FORMATS:
            raise HTTPException(422, f"unknown format '{m.format}'; use one of: {', '.join(FORMATS)}")
        if not m.name.replace("-", "").replace(".", "").isalnum() or m.name != m.name.lower():
            raise HTTPException(422, f"invalid name '{m.name}'; use lowercase kebab-case (e.g. tomato-ripeness-cnn)")
        if q("SELECT 1 FROM models WHERE name=%s", (m.name,), one=True):
            raise HTTPException(409, f"model '{m.name}' already exists")
        return q("""INSERT INTO models
                    (name, task, format, params_m, size_mb, license, gpu_required, downloads, description)
                    VALUES (%s,%s,%s,%s,%s,%s,%s,0,%s) RETURNING *""",
                 (m.name, m.task, m.format, m.params_m, m.size_mb,
                  m.license, m.gpu_required, m.description), one=True)

    @app.delete("/models/{mid}")
    def delete_model(mid: int):
        r = q("DELETE FROM models WHERE id=%s RETURNING id, name", (mid,), one=True)
        if not r:
            raise HTTPException(404, "model not found")
        return {"deleted": r}
EOF

cat <<'EOF' | kubectl -n mcp-demo apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: model-registry-backend
spec:
  replicas: 1
  selector:
    matchLabels: { app: model-registry-backend }
  template:
    metadata:
      labels: { app: model-registry-backend }
    spec:
      containers:
        - name: backend
          image: python:3.11-slim
          workingDir: /app
          command: ["sh", "-c"]
          args:
            - pip install -q --root-user-action=ignore --disable-pip-version-check
              fastapi uvicorn "psycopg[binary]" &&
              uvicorn app:app --host 0.0.0.0 --port 8000
          env:
            - name: DATABASE_URI
              value: postgresql://demo:demo1234@model-registry-db:5432/registry
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
            name: registry-backend-app
---
apiVersion: v1
kind: Service
metadata:
  name: model-registry-backend
spec:
  selector: { app: model-registry-backend }
  ports:
    - port: 8000
      targetPort: 8000
EOF

echo ""
echo "Waiting for the backend to start (pip install at boot; typically 1-2 min)..."
kubectl -n ${NS} rollout restart deployment/model-registry-backend > /dev/null
kubectl -n ${NS} rollout status deployment/model-registry-backend --timeout=5m > /dev/null

echo ""
echo "=== Smoke test: GET /models ==="
kubectl -n ${NS} port-forward svc/model-registry-backend 18000:8000 > /dev/null 2>&1 &
PF_PID=$!
trap "kill ${PF_PID} 2>/dev/null" EXIT
sleep 3
curl -s --max-time 10 "http://127.0.0.1:18000/models" | python3 -c "
import sys, json
rows = json.load(sys.stdin)
print(f'  {len(rows)} models registered; top downloads:')
for r in rows[:3]:
    print(f\"    {r['name']} ({r['task']}, {r['format']}) — {r['downloads']} downloads\")"

echo ""
echo "========================================"
echo "SUCCESS: Model Registry backend is running"
echo "========================================"
echo "  In-cluster URL: http://model-registry-backend.${NS}.svc.cluster.local:8000"
echo "  OpenAPI spec:   http://model-registry-backend.${NS}.svc.cluster.local:8000/openapi.json"
echo ""
echo "\$\$CMD[Check Registry Backend](kubectl -n mcp-demo get pods -l app=model-registry-backend)"
exit 0
