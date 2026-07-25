#!/bin/bash

# Example B (automated): build a custom model image, push it to the in-cluster
# private registry, and serve it via KServe — end to end on the control plane.
# Prerequisites: deploy-private-registry.sh + config-registry-access.sh (all nodes)

set -e

REGISTRY="localhost:30500"
IMAGE="${REGISTRY}/custom-model:latest"
RAW_BASE="https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/kserve/examples/b-custom-model"

echo "==== Example B: custom model via private registry ===="

if ! curl -s --max-time 5 "http://${REGISTRY}/v2/_catalog" > /dev/null; then
    echo "ERROR: Private registry not reachable at ${REGISTRY}. Run deploy-private-registry.sh first."
    exit 1
fi

# Docker is needed for the build (docker-ce; the docker apt repo is already set up by k8s scripts)
if ! command -v docker > /dev/null 2>&1; then
    echo "Installing docker-ce for image build..."
    sudo DEBIAN_FRONTEND=noninteractive apt-get update -qq
    sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq docker-ce docker-ce-cli > /dev/null
fi

# Fetch the example source and build (docker treats localhost registries as insecure by default)
WORKDIR=$(mktemp -d)
curl -fsSL "${RAW_BASE}/model.py" -o "${WORKDIR}/model.py"
curl -fsSL "${RAW_BASE}/Dockerfile" -o "${WORKDIR}/Dockerfile"
echo "Building and pushing ${IMAGE}..."
sudo docker build -q -t "${IMAGE}" "${WORKDIR}" > /dev/null
sudo docker push "${IMAGE}" > /dev/null
echo "  ✓ Image pushed to the private registry"

cat <<EOF | kubectl apply -f -
apiVersion: serving.kserve.io/v1beta1
kind: InferenceService
metadata:
  name: custom-model
spec:
  predictor:
    containers:
      - name: kserve-container
        image: ${IMAGE}
        resources:
          requests: { cpu: 100m, memory: 256Mi }
          limits: { cpu: "1", memory: 1Gi }
EOF

echo "Waiting for the model to be ready..."
kubectl wait --for=condition=Ready inferenceservice/custom-model --timeout=10m

CLUSTER_IP=$(kubectl get svc custom-model-predictor -o jsonpath='{.spec.clusterIP}')
echo ""
echo "Prediction test:"
curl -s "http://${CLUSTER_IP}/v1/models/custom-model:predict" \
  -H 'Content-Type: application/json' -d '{"instances": [[1, 2, 3]]}'
echo ""
echo ""
echo "========================================"
echo "SUCCESS: custom model built, pushed, and served"
echo "========================================"
echo "To serve your own logic: edit predict() in model.py and re-run this flow."
echo ""
echo "\$\$CMD[Check InferenceService](kubectl get isvc custom-model)"
exit 0
