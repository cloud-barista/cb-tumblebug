#!/bin/bash

# Example C: a model served as a plain Deployment + Service — no KServe involved.
# Shows the DIY path: full control, but scaling/canary/model management are on you.
# The demo app trains a tiny sklearn iris model at startup and serves it with FastAPI.
# (In practice you would bake this into an image; inline pip install keeps the demo registry-free.)

set -e

echo "==== Example C: plain Deployment serving (no KServe) ===="

cat <<'EOF' | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: plain-model-app
data:
  app.py: |
    from fastapi import FastAPI
    from sklearn.datasets import load_iris
    from sklearn.linear_model import LogisticRegression

    iris = load_iris()
    model = LogisticRegression(max_iter=200).fit(iris.data, iris.target)
    app = FastAPI()

    @app.post("/predict")
    def predict(body: dict):
        return {"predictions": model.predict(body["instances"]).tolist()}
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

echo "Waiting for the app (pip install at startup takes 1-2 min)..."
kubectl rollout status deployment/plain-model --timeout=5m

CLUSTER_IP=$(kubectl get svc plain-model -o jsonpath='{.spec.clusterIP}')
sleep 5
echo ""
echo "Prediction test:"
curl -s --retry 5 --retry-delay 5 --retry-connrefused "http://${CLUSTER_IP}/predict" \
  -H 'Content-Type: application/json' \
  -d '{"instances": [[6.8, 2.8, 4.8, 1.4], [6.0, 3.4, 4.5, 1.6]]}'
echo ""
echo ""
echo "Compare with Example A: same model, but here HPA/canary/versioning are all manual."
exit 0
