#!/bin/bash

# Private Container Registry (run on K8s control plane)
# Deploys registry:2 with a PVC, exposed on NodePort 30500 so every node can
# reach it as localhost:30500. Intended for in-vNet use only — do NOT open the
# port in the Security Group.
# After this, run config-registry-access.sh on ALL nodes to allow plain-HTTP pulls.

set -e

NODE_PORT="30500"
STORAGE="10Gi"

while [[ $# -gt 0 ]]; do
    case $1 in
        --nodeport) NODE_PORT="$2"; shift 2 ;;
        --storage) STORAGE="$2"; shift 2 ;;
        *) echo "Usage: $0 [--nodeport 30500] [--storage 10Gi]"; exit 1 ;;
    esac
done

echo "==== Private Registry Setup ===="

if ! kubectl get storageclass 2>/dev/null | grep -q "(default)"; then
    echo "ERROR: No default StorageClass. Run deploy-kserve-stack.sh first."
    exit 1
fi

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: registry-data
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: ${STORAGE}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: registry
spec:
  replicas: 1
  selector:
    matchLabels: { app: registry }
  template:
    metadata:
      labels: { app: registry }
    spec:
      containers:
        - name: registry
          image: registry:2
          ports:
            - containerPort: 5000
          volumeMounts:
            - name: data
              mountPath: /var/lib/registry
          resources:
            requests: { cpu: 100m, memory: 128Mi }
            limits: { cpu: "1", memory: 512Mi }
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: registry-data
---
apiVersion: v1
kind: Service
metadata:
  name: registry
spec:
  type: NodePort
  selector: { app: registry }
  ports:
    - port: 5000
      targetPort: 5000
      nodePort: ${NODE_PORT}
EOF

kubectl rollout status deployment/registry --timeout=5m > /dev/null

# NodePort makes the registry reachable as localhost:<port> on every node
sleep 3
if curl -s --max-time 10 "http://localhost:${NODE_PORT}/v2/_catalog" | grep -q repositories; then
    echo "  ✓ Registry is responding"
else
    echo "  ⚠ Registry not responding yet; check: kubectl get pods -l app=registry"
fi

echo ""
echo "========================================"
echo "SUCCESS: Private registry is running"
echo "========================================"
echo ""
echo "[REGISTRY_URL]"
echo "localhost:${NODE_PORT}"
echo ""
echo "Next: run config-registry-access.sh on ALL nodes (containerd plain-HTTP config),"
echo "then push images as localhost:${NODE_PORT}/<image>:<tag> from any node."
echo "Keep NodePort ${NODE_PORT} closed in the Security Group (no auth/TLS; vNet-internal only)."
echo ""
echo "\$\$CMD[List Images](curl -s http://localhost:${NODE_PORT}/v2/_catalog)"
exit 0
