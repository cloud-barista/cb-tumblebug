#!/bin/bash

# Example C: the SAME digit recognition task as example A, but as a plain
# Deployment + Service — no KServe. You write the API server yourself and manage
# scaling/rollout manually; the scaling demo below shows what that looks like.
# Run on the control plane.
# (In practice you would bake an image; inline pip install keeps the demo registry-free.)

set -e

REPLICAS=3
TRAFFIC_REQUESTS=30

cat <<'BANNER'
┌────────────────────────────────────────────────────────────┐
│ METHOD C · plain Deployment (no KServe)                    │
│  you write   : the API server code (FastAPI) + Deployment/ │
│                Service YAML                                │
│  you manage  : scaling, load balancing, rollout — manually │
│  platform    : only generic k8s primitives                 │
└────────────────────────────────────────────────────────────┘
BANNER

# Reset so re-runs always start the demo from 1 replica with the latest app code
kubectl delete deploy plain-model --ignore-not-found > /dev/null

cat <<'EOF' | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: plain-model-app
data:
  app.py: |
    import socket
    from fastapi import FastAPI
    from sklearn.datasets import load_digits
    from sklearn.neural_network import MLPClassifier

    X, y = load_digits(return_X_y=True)
    model = MLPClassifier(hidden_layer_sizes=(64,), max_iter=300, random_state=42).fit(X, y)
    app = FastAPI()

    @app.post("/predict")
    def predict(body: dict):
        return {
            "predictions": [int(p) for p in model.predict(body["instances"])],
            "served_by": socket.gethostname(),
        }
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: plain-model
spec:
  replicas: 1
  selector:
    matchLabels: { app: plain-model }
  template:
    metadata:
      labels: { app: plain-model }
    spec:
      containers:
        - name: api
          image: python:3.11-slim
          command: ["sh", "-c"]
          args:
            - pip install --no-cache-dir -q fastapi uvicorn scikit-learn &&
              uvicorn app:app --host 0.0.0.0 --port 8000 --app-dir /app
          volumeMounts:
            - { name: app, mountPath: /app }
          readinessProbe:
            httpGet: { path: /docs, port: 8000 }
            initialDelaySeconds: 20
            periodSeconds: 5
          resources:
            requests: { cpu: 100m, memory: 512Mi }
            limits: { cpu: "1", memory: 1Gi }
      volumes:
        - name: app
          configMap: { name: plain-model-app }
---
apiVersion: v1
kind: Service
metadata:
  name: plain-model
spec:
  selector: { app: plain-model }
  ports:
    - port: 80
      targetPort: 8000
EOF

echo ""
echo "Deploying 1 replica... (pip install + model training at startup; status every 10s)"
for i in $(seq 1 36); do
    READY=$(kubectl get deploy plain-model -o jsonpath='{.status.readyReplicas}' 2>/dev/null)
    echo "  [$(date +%H:%M:%S)] $(kubectl get pods -l app=plain-model --no-headers 2>/dev/null | head -1)"
    [ "${READY:-0}" -ge 1 ] && break
    sleep 10
done
[ "${READY:-0}" -ge 1 ] || { echo "ERROR: not ready. Check: kubectl describe deploy plain-model"; exit 1; }

CLUSTER_IP=$(kubectl get svc plain-model -o jsonpath='{.spec.clusterIP}')
SAMPLE="0,0,7,15,13,1,0,0,0,8,13,6,15,4,0,0,0,2,1,13,13,0,0,0,0,0,2,15,11,1,0,0,0,0,0,1,12,12,1,0,0,0,0,0,1,10,8,0,0,0,8,4,5,14,9,0,0,0,7,13,13,9,0,0"  # a handwritten "3"

predict() {
    curl -s --max-time 10 "http://${CLUSTER_IP}/predict" -H 'Content-Type: application/json' \
        -d "{\"instances\": [[${SAMPLE}]]}"
}

echo ""
echo "=== The input image (a handwritten digit) ==="
python3 -c "
vals = [int(v) for v in '${SAMPLE}'.split(',')]
chars = ' .:-=+*#%@'
for r in range(8):
    print('    ' + ''.join(chars[min(v * len(chars) // 17, len(chars)-1)] * 2 for v in vals[r*8:(r+1)*8]))"

echo ""
echo "=== Single pod: every request answered by the same pod ==="
for i in 1 2 3; do
    echo "  request $i → $(predict)"
done

echo ""
echo "=== Scaling to ${REPLICAS} replicas — by hand (kubectl scale) ==="
kubectl scale deploy plain-model --replicas=${REPLICAS} > /dev/null
for i in $(seq 1 36); do
    READY=$(kubectl get deploy plain-model -o jsonpath='{.status.readyReplicas}' 2>/dev/null)
    echo "  [$(date +%H:%M:%S)] ready ${READY:-0}/${REPLICAS}"
    [ "${READY:-0}" -ge "${REPLICAS}" ] && break
    sleep 10
done
kubectl get pods -l app=plain-model --no-headers | sed 's/^/  /'

echo ""
echo "=== ${TRAFFIC_REQUESTS} requests: load-balanced across pods ==="
declare -A HITS
for i in $(seq 1 ${TRAFFIC_REQUESTS}); do
    POD=$(predict | grep -oE '"served_by":"[^"]+"' | cut -d'"' -f4)
    HITS[$POD]=$(( ${HITS[$POD]:-0} + 1 ))
done
for POD in "${!HITS[@]}"; do
    BAR=$(printf '█%.0s' $(seq 1 ${HITS[$POD]}))
    printf "  %-34s %3d  %s\n" "$POD" "${HITS[$POD]}" "$BAR"
done

echo ""
echo "Method C summary: same model as A, but the API server, scaling, and rollout"
echo "were all hand-built — everything KServe automated in method A."
echo "\$\$CMD[Check Pods](kubectl get pods -l app=plain-model)"
exit 0
