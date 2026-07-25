#!/bin/bash

# Demo PostgreSQL for the MCP usecase (run on K8s control plane)
# Creates namespace mcp-demo and a Postgres instance seeded with web-shop sample data.
# Designed for unattended execution via SSH or pipe (curl | bash)

set -e

NS="mcp-demo"
PG_USER="demo"
PG_PASS="demo1234"
PG_DB="demo"

echo "==== Demo PostgreSQL Setup (namespace: ${NS}) ===="

# The PVC below needs a default StorageClass; install local-path if missing
if ! kubectl get storageclass 2>/dev/null | grep -q "(default)"; then
    echo "No default StorageClass found; installing local-path-provisioner..."
    kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.30/deploy/local-path-storage.yaml
    kubectl patch storageclass local-path -p '{"metadata":{"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
    kubectl -n local-path-storage rollout status deployment/local-path-provisioner --timeout=3m
fi

kubectl create namespace ${NS} 2>/dev/null || true

# Sample data: a tiny web shop (customers / products / orders)
cat <<'EOF' | kubectl -n mcp-demo apply -f - > /dev/null
apiVersion: v1
kind: ConfigMap
metadata:
  name: postgres-init
data:
  init.sql: |
    CREATE TABLE customers (
      id SERIAL PRIMARY KEY, name TEXT NOT NULL, email TEXT UNIQUE, country TEXT);
    CREATE TABLE products (
      id SERIAL PRIMARY KEY, name TEXT NOT NULL, category TEXT,
      price NUMERIC(10,2) NOT NULL, stock INT NOT NULL);
    CREATE TABLE orders (
      id SERIAL PRIMARY KEY, customer_id INT REFERENCES customers(id),
      status TEXT DEFAULT 'completed', order_date DATE NOT NULL);
    CREATE TABLE order_items (
      order_id INT REFERENCES orders(id), product_id INT REFERENCES products(id),
      quantity INT NOT NULL, unit_price NUMERIC(10,2) NOT NULL);

    INSERT INTO customers (name, email, country) VALUES
      ('Ada Lovelace', 'ada@example.com', 'UK'),
      ('Grace Hopper', 'grace@example.com', 'US'),
      ('Alan Turing', 'alan@example.com', 'UK'),
      ('Margaret Hamilton', 'margaret@example.com', 'US'),
      ('Sejong Kim', 'sejong@example.com', 'KR'),
      ('Linus Virtanen', 'linus@example.com', 'FI');

    INSERT INTO products (name, category, price, stock) VALUES
      ('Raspberry Pi 5 (8GB)', 'sbc', 79.99, 42),
      ('Arduino Uno R4 WiFi', 'mcu', 27.50, 120),
      ('Jetson Orin Nano Devkit', 'edge-ai', 499.00, 8),
      ('Coral USB Accelerator', 'edge-ai', 59.99, 3),
      ('ESP32 DevKit v1', 'mcu', 8.99, 200),
      ('LoRaWAN Gateway', 'network', 189.00, 4),
      ('NVMe SSD 2TB', 'storage', 149.00, 25),
      ('10G Ethernet Switch', 'network', 899.00, 2),
      ('USB Logic Analyzer', 'tools', 12.50, 60),
      ('Edge AI Camera Module', 'edge-ai', 249.00, 15);

    INSERT INTO orders (customer_id, status, order_date) VALUES
      (1, 'completed', '2026-06-02'), (2, 'completed', '2026-06-05'),
      (3, 'completed', '2026-06-11'), (1, 'completed', '2026-06-18'),
      (4, 'completed', '2026-06-23'), (5, 'completed', '2026-06-29'),
      (2, 'completed', '2026-07-03'), (6, 'completed', '2026-07-08'),
      (3, 'completed', '2026-07-12'), (5, 'shipped',   '2026-07-17'),
      (4, 'shipped',   '2026-07-21'), (1, 'pending',   '2026-07-24');

    INSERT INTO order_items (order_id, product_id, quantity, unit_price) VALUES
      (1, 1, 2, 79.99),  (1, 5, 5, 8.99),
      (2, 3, 1, 499.00), (2, 4, 2, 59.99),
      (3, 2, 3, 27.50),  (3, 9, 1, 12.50),
      (4, 8, 1, 899.00),
      (5, 6, 2, 189.00), (5, 5, 10, 8.99),
      (6, 1, 1, 79.99),  (6, 7, 2, 149.00),
      (7, 10, 1, 249.00),(7, 4, 1, 59.99),
      (8, 5, 20, 8.99),  (8, 2, 2, 27.50),
      (9, 3, 1, 499.00),
      (10, 7, 1, 149.00),(10, 9, 2, 12.50),
      (11, 10, 2, 249.00),
      (12, 1, 1, 79.99), (12, 6, 1, 189.00);
EOF

cat <<EOF | kubectl -n ${NS} apply -f -
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgres-data
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: 2Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: postgres
spec:
  replicas: 1
  strategy: { type: Recreate }
  selector:
    matchLabels: { app: postgres }
  template:
    metadata:
      labels: { app: postgres }
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
            claimName: postgres-data
        - name: init
          configMap:
            name: postgres-init
---
apiVersion: v1
kind: Service
metadata:
  name: postgres
spec:
  selector: { app: postgres }
  ports:
    - port: 5432
      targetPort: 5432
EOF

echo ""
echo "Waiting for PostgreSQL to become ready..."
kubectl -n ${NS} rollout status deployment/postgres --timeout=5m > /dev/null

for i in $(seq 1 12); do
    kubectl -n ${NS} exec deploy/postgres -- pg_isready -U ${PG_USER} > /dev/null 2>&1 && break
    sleep 5
done

echo ""
echo "=== Seeded data ==="
kubectl -n ${NS} exec deploy/postgres -- psql -U ${PG_USER} -d ${PG_DB} -c \
  "SELECT 'customers' AS table, count(*) FROM customers
   UNION ALL SELECT 'products', count(*) FROM products
   UNION ALL SELECT 'orders', count(*) FROM orders
   UNION ALL SELECT 'order_items', count(*) FROM order_items;"

echo "========================================"
echo "SUCCESS: PostgreSQL is running"
echo "========================================"
echo "  In-cluster URI: postgresql://${PG_USER}:${PG_PASS}@postgres.${NS}.svc.cluster.local:5432/${PG_DB}"
echo "  (Note: init.sql only runs on first boot; delete PVC postgres-data to reseed)"
echo ""
echo "\$\$CMD[Check Postgres](kubectl -n mcp-demo get pods -l app=postgres)"
exit 0
