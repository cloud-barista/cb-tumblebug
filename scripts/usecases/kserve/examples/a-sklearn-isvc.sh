#!/bin/bash

# Example A: serve a standard-format model via KServe — no container image to build.
# Deploys KServe's public sklearn iris model, then shows how to serve your own model from a PVC.
# Run on the control plane after deploy-kserve-stack.sh.

set -e

echo "==== Example A: sklearn model via KServe standard runtime ===="

cat <<EOF | kubectl apply -f -
apiVersion: serving.kserve.io/v1beta1
kind: InferenceService
metadata:
  name: sklearn-iris
spec:
  predictor:
    model:
      modelFormat:
        name: sklearn
      storageUri: gs://kfserving-examples/models/sklearn/1.0/model
      resources:
        requests: { cpu: 100m, memory: 256Mi }
        limits: { cpu: "1", memory: 1Gi }
EOF

echo "Waiting for the model to be ready..."
kubectl wait --for=condition=Ready inferenceservice/sklearn-iris --timeout=10m

CLUSTER_IP=$(kubectl get svc sklearn-iris-predictor -o jsonpath='{.spec.clusterIP}')
echo ""
echo "Prediction test (V1 inference protocol):"
curl -s "http://${CLUSTER_IP}/v1/models/sklearn-iris:predict" \
  -H 'Content-Type: application/json' \
  -d '{"instances": [[6.8, 2.8, 4.8, 1.4], [6.0, 3.4, 4.5, 1.6]]}'
echo ""

cat <<'GUIDE'

========================================
Serving YOUR OWN model artifact instead
========================================
1. Export on your VM:        joblib.dump(model, "model.joblib")
2. Copy it into a PVC:
     kubectl apply -f - <<EOF
     apiVersion: v1
     kind: PersistentVolumeClaim
     metadata: { name: my-model }
     spec: { accessModes: ["ReadWriteOnce"], resources: { requests: { storage: 1Gi } } }
     EOF
     kubectl run pvc-helper --image=busybox --restart=Never --overrides='{"spec":{"containers":[{"name":"pvc-helper","image":"busybox","command":["sleep","3600"],"volumeMounts":[{"name":"m","mountPath":"/mnt/models"}]}],"volumes":[{"name":"m","persistentVolumeClaim":{"claimName":"my-model"}}]}}'
     kubectl cp model.joblib pvc-helper:/mnt/models/model.joblib
     kubectl delete pod pvc-helper
3. Point the InferenceService at it:
     storageUri: pvc://my-model/
GUIDE
exit 0
