#!/bin/bash

# Model Registry DB for the MCP usecase (run on K8s control plane)
# PostgreSQL seeded with a Hugging Face-style model catalog (metadata only).
# Designed for unattended execution via SSH or pipe (curl | bash)

set -e

NS="mcp-demo"
PG_USER="demo"
PG_PASS="demo1234"
PG_DB="registry"

echo "==== Model Registry DB Setup (namespace: ${NS}) ===="

# The PVC below needs a default StorageClass; install local-path if missing
if ! kubectl get storageclass 2>/dev/null | grep -q "(default)"; then
    echo "No default StorageClass found; installing local-path-provisioner..."
    kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.30/deploy/local-path-storage.yaml
    kubectl patch storageclass local-path -p '{"metadata":{"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
    kubectl -n local-path-storage rollout status deployment/local-path-provisioner --timeout=3m
fi

kubectl create namespace ${NS} 2>/dev/null || true

# Sample catalog: diverse KServe-servable formats, agriculture-heavy CPU models plus GPU LLMs
cat <<'EOF' | kubectl -n mcp-demo apply -f - > /dev/null
apiVersion: v1
kind: ConfigMap
metadata:
  name: registry-db-init
data:
  init.sql: |
    CREATE TABLE models (
      id SERIAL PRIMARY KEY,
      name TEXT UNIQUE NOT NULL,
      task TEXT NOT NULL,
      format TEXT NOT NULL,
      params_m NUMERIC(10,1),
      size_mb INT,
      license TEXT,
      gpu_required BOOLEAN DEFAULT false,
      downloads INT DEFAULT 0,
      description TEXT,
      updated DATE NOT NULL DEFAULT CURRENT_DATE);

    INSERT INTO models
      (name, task, format, params_m, size_mb, license, gpu_required, downloads, description, updated) VALUES
      ('crop-disease-vit-tiny', 'image-classification', 'onnx', 5.7, 22, 'apache-2.0', false, 15420,
       'Detects 38 crop leaf diseases (PlantVillage); runs on CPU edge devices', '2026-05-12'),
      ('rice-yield-xgb', 'tabular-regression', 'xgboost', 0.4, 3, 'mit', false, 8930,
       'Predicts rice yield from soil, weather, and fertilizer features', '2026-04-02'),
      ('soil-moisture-lgbm', 'time-series-forecasting', 'lightgbm', 0.2, 2, 'apache-2.0', false, 4210,
       '7-day soil moisture forecast for irrigation scheduling', '2026-03-18'),
      ('pest-outbreak-risk-xgb', 'tabular-classification', 'xgboost', 0.3, 2, 'apache-2.0', false, 3105,
       'Regional pest outbreak risk from climate and trap-count data', '2026-06-20'),
      ('greenhouse-co2-forecaster', 'time-series-forecasting', 'sklearn', 0.1, 1, 'bsd-3-clause', false, 1870,
       'CO2 setpoint forecasting for greenhouse climate control', '2026-02-27'),
      ('digits-mlp', 'image-classification', 'sklearn', 0.1, 1, 'bsd-3-clause', false, 950,
       'Handwritten digit classifier used in the KServe method A/B/C examples', '2026-07-01'),
      ('fruit-ripeness-mobilenet', 'image-classification', 'tensorflow', 3.5, 14, 'apache-2.0', false, 6740,
       'Classifies fruit ripeness stages from RGB images', '2026-05-30'),
      ('livestock-detect-yolo11n', 'object-detection', 'pytorch', 2.6, 11, 'agpl-3.0', false, 12800,
       'Counts and tracks livestock in drone footage', '2026-06-08'),
      ('weather-downscale-unet', 'image-to-image', 'tensorflow', 31.0, 120, 'apache-2.0', true, 2380,
       'Downscales regional weather grids to farm-level resolution', '2026-01-15'),
      ('sentiment-ko-electra-small', 'text-classification', 'huggingface', 14.0, 54, 'apache-2.0', false, 22150,
       'Korean sentiment analysis fine-tuned on product reviews', '2026-03-05'),
      ('qwen2.5-7b-instruct', 'text-generation', 'huggingface', 7620.0, 15500, 'apache-2.0', true, 184300,
       'General instruction-following LLM; servable with KServe + vLLM', '2026-04-22'),
      ('agri-qa-llama3-8b-lora', 'text-generation', 'huggingface', 8030.0, 16300, 'llama3', true, 5490,
       'Agronomy Q&A assistant (LoRA-tuned Llama 3) for extension services', '2026-06-30');
EOF

cat <<EOF | kubectl -n ${NS} apply -f -
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: registry-db-data
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: 2Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: model-registry-db
spec:
  replicas: 1
  strategy: { type: Recreate }
  selector:
    matchLabels: { app: model-registry-db }
  template:
    metadata:
      labels: { app: model-registry-db }
    spec:
      containers:
        - name: postgres
          image: postgres:16
          ports:
            - containerPort: 5432
          env:
            - name: POSTGRES_USER
              value: "${PG_USER}"
            - name: POSTGRES_PASSWORD
              value: "${PG_PASS}"
            - name: POSTGRES_DB
              value: "${PG_DB}"
            - name: PGDATA
              value: /var/lib/postgresql/data/pgdata
          readinessProbe:
            exec:
              command: ["pg_isready", "-U", "${PG_USER}"]
            initialDelaySeconds: 5
            periodSeconds: 5
          volumeMounts:
            - name: data
              mountPath: /var/lib/postgresql/data
            - name: init
              mountPath: /docker-entrypoint-initdb.d
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: registry-db-data
        - name: init
          configMap:
            name: registry-db-init
---
apiVersion: v1
kind: Service
metadata:
  name: model-registry-db
spec:
  selector: { app: model-registry-db }
  ports:
    - port: 5432
      targetPort: 5432
EOF

echo ""
echo "Waiting for PostgreSQL to become ready..."
kubectl -n ${NS} rollout status deployment/model-registry-db --timeout=5m > /dev/null

for i in $(seq 1 12); do
    kubectl -n ${NS} exec deploy/model-registry-db -- pg_isready -U ${PG_USER} > /dev/null 2>&1 && break
    sleep 5
done

echo ""
echo "=== Seeded model catalog ==="
kubectl -n ${NS} exec deploy/model-registry-db -- psql -U ${PG_USER} -d ${PG_DB} -c \
  "SELECT format, count(*) AS models, sum(downloads) AS downloads
   FROM models GROUP BY format ORDER BY downloads DESC;"

echo "========================================"
echo "SUCCESS: Model Registry DB is running"
echo "========================================"
echo "  In-cluster URI: postgresql://${PG_USER}:${PG_PASS}@model-registry-db.${NS}.svc.cluster.local:5432/${PG_DB}"
echo "  (Note: init.sql only runs on first boot; delete PVC registry-db-data to reseed)"
echo ""
echo "\$\$CMD[Check Registry DB](kubectl -n mcp-demo get pods -l app=model-registry-db)"
exit 0
