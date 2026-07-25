#!/bin/bash

# Example B (automated): build a custom model image, push it to the in-cluster
# private registry, and serve it via KServe — end to end on the control plane.
# Prerequisites: deploy-private-registry.sh + config-registry-access.sh (all nodes)

set -e

REGISTRY="localhost:30500"
IMAGE="${REGISTRY}/custom-model:latest"
RAW_BASE="https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/kserve/examples/b-custom-model"

cat <<'BANNER'
┌────────────────────────────────────────────────────────────┐
│ METHOD B · KServe custom container                         │
│  you write   : predict() with custom pre/post-processing   │
│                (~20 lines) + a 4-line Dockerfile           │
│  you provide : image in a registry (in-cluster private)    │
│  platform    : API server, protocol, scaling, canary       │
└────────────────────────────────────────────────────────────┘
BANNER
echo "==== Example B: digit recognition with custom pre/post-processing ===="

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
echo "=== Image is served from the in-cluster private registry ==="
echo "  registry catalog: $(curl -s http://${REGISTRY}/v2/_catalog)"
echo "  isvc image:       $(kubectl get isvc custom-model -o jsonpath='{.spec.predictor.containers[0].image}')"
echo ""
SAMPLE="0,0,12,10,0,0,0,0,0,0,14,16,16,14,0,0,0,0,13,16,15,10,1,0,0,0,11,16,16,7,0,0,0,0,0,4,7,16,7,0,0,0,0,0,4,16,9,0,0,0,5,4,12,16,4,0,0,0,9,16,16,10,0,0"  # a handwritten "5"
echo "=== The input image ==="
python3 -c "
vals = [int(v) for v in '${SAMPLE}'.split(',')]
chars = ' .:-=+*#%@'
for r in range(8):
    print('    ' + ''.join(chars[min(v * len(chars) // 17, len(chars)-1)] * 2 for v in vals[r*8:(r+1)*8]))"
echo ""
echo "=== Prediction (custom postprocessing returns digit + confidence) ==="
RESP=$(curl -s "http://${CLUSTER_IP}/v1/models/custom-model:predict" \
  -H 'Content-Type: application/json' -d "{\"instances\": [[${SAMPLE}]]}")
echo "  → ${RESP}"
echo ""
echo "========================================"
echo "SUCCESS: custom model built, pushed, and served"
echo "========================================"
echo "Method B summary: same task as A, but predict() carries YOUR pre/post-processing"
echo "— that is what justified building an image. Serving is still managed by KServe."
echo ""
echo "\$\$CMD[Check InferenceService](kubectl get isvc custom-model)"
exit 0
