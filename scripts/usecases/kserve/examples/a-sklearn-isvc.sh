#!/bin/bash

# Example A: handwritten digit recognition served via the KServe STANDARD RUNTIME.
# Full user journey: train in-cluster → model artifact on a PVC → 8-line YAML → serving.
# The point: zero serving code and zero image builds — compare with examples B and C.
# Run on the control plane.

set -e

TRAFFIC_SECONDS=30

while [[ $# -gt 0 ]]; do
    case $1 in
        --traffic) TRAFFIC_SECONDS="$2"; shift 2 ;;
        *) echo "Usage: $0 [--traffic <seconds>]"; exit 1 ;;
    esac
done

cat <<'BANNER'
┌────────────────────────────────────────────────────────────┐
│ METHOD A · KServe standard runtime (sklearn)               │
│  you write   : NO serving code, NO Dockerfile              │
│  you provide : model artifact (joblib) + 8-line YAML       │
│  platform    : API server, protocol, scaling, canary       │
└────────────────────────────────────────────────────────────┘
BANNER

# ============================================================
# 1) Train the model in-cluster and save the artifact to a PVC
# ============================================================
echo "[1/3] Training digit classifier in-cluster (MLP on sklearn digits)..."
cat <<'EOF' | kubectl apply -f - > /dev/null
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: digits-model
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: 1Gi
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: digits-trainer-script
data:
  train.py: |
    from sklearn.datasets import load_digits
    from sklearn.neural_network import MLPClassifier
    from sklearn.model_selection import train_test_split
    import joblib
    X, y = load_digits(return_X_y=True)
    Xtr, Xte, ytr, yte = train_test_split(X, y, test_size=0.25, random_state=42)
    m = MLPClassifier(hidden_layer_sizes=(64,), max_iter=300, random_state=42).fit(Xtr, ytr)
    print(f"trained on {len(Xtr)} samples, test accuracy: {m.score(Xte, yte):.3f}")
    joblib.dump(m, "/mnt/models/model.joblib")
    print("artifact saved: /mnt/models/model.joblib")
EOF

kubectl delete pod digits-trainer --ignore-not-found > /dev/null 2>&1
# scikit-learn pinned to the version bundled in the kserve sklearnserver runtime
kubectl run digits-trainer --image=python:3.11-slim --restart=Never \
  --overrides='{"spec":{"containers":[{"name":"digits-trainer","image":"python:3.11-slim","command":["sh","-c","pip install -q --root-user-action=ignore --disable-pip-version-check scikit-learn==1.5.2 joblib && python3 /scripts/train.py"],"volumeMounts":[{"name":"models","mountPath":"/mnt/models"},{"name":"scripts","mountPath":"/scripts"}]}],"volumes":[{"name":"models","persistentVolumeClaim":{"claimName":"digits-model"}},{"name":"scripts","configMap":{"name":"digits-trainer-script"}}],"restartPolicy":"Never"}}' > /dev/null

for i in $(seq 1 30); do
    PHASE=$(kubectl get pod digits-trainer -o jsonpath='{.status.phase}' 2>/dev/null)
    echo "  [$(date +%H:%M:%S)] trainer: ${PHASE:-Pending}"
    [ "$PHASE" = "Succeeded" ] && break
    [ "$PHASE" = "Failed" ] && { kubectl logs digits-trainer | tail -5; exit 1; }
    sleep 10
done
kubectl logs digits-trainer | sed 's/^/  /'
kubectl delete pod digits-trainer > /dev/null

# ============================================================
# 2) Serve the artifact — this YAML is ALL the serving config
# ============================================================
echo ""
echo "[2/3] Serving via InferenceService (no code, no image build)..."
cat <<EOF | kubectl apply -f -
apiVersion: serving.kserve.io/v1beta1
kind: InferenceService
metadata:
  name: digits
spec:
  predictor:
    model:
      modelFormat:
        name: sklearn
      storageUri: pvc://digits-model/
      resources:
        requests: { cpu: 100m, memory: 256Mi }
        limits: { cpu: "1", memory: 1Gi }
EOF

for i in $(seq 1 60); do
    READY=$(kubectl get isvc digits -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null)
    echo "  [$(date +%H:%M:%S)] $(kubectl get pods -l serving.kserve.io/inferenceservice=digits --no-headers 2>/dev/null | head -1)"
    [ "$READY" = "True" ] && break
    sleep 10
done
[ "$READY" = "True" ] || { echo "ERROR: not ready. Check: kubectl describe isvc digits"; exit 1; }
echo "  ✓ InferenceService READY"

# ============================================================
# 3) Predict: render each 8x8 input image and classify it
# ============================================================
CLUSTER_IP=$(kubectl get svc digits-predictor -o jsonpath='{.spec.clusterIP}')
URL="http://${CLUSTER_IP}/v1/models/digits:predict"

SAMPLES=(
  "0|0,0,5,13,9,1,0,0,0,0,13,15,10,15,5,0,0,3,15,2,0,11,8,0,0,4,12,0,0,8,8,0,0,5,8,0,0,9,8,0,0,4,11,0,1,12,7,0,0,2,14,5,10,12,0,0,0,0,6,13,10,0,0,0"
  "3|0,0,7,15,13,1,0,0,0,8,13,6,15,4,0,0,0,2,1,13,13,0,0,0,0,0,2,15,11,1,0,0,0,0,0,1,12,12,1,0,0,0,0,0,1,10,8,0,0,0,8,4,5,14,9,0,0,0,7,13,13,9,0,0"
  "5|0,0,12,10,0,0,0,0,0,0,14,16,16,14,0,0,0,0,13,16,15,10,1,0,0,0,11,16,16,7,0,0,0,0,0,4,7,16,7,0,0,0,0,0,4,16,9,0,0,0,5,4,12,16,4,0,0,0,9,16,16,10,0,0"
  "7|0,0,7,8,13,16,15,1,0,0,7,7,4,11,12,0,0,0,0,0,8,13,1,0,0,4,8,8,15,15,6,0,0,2,11,15,15,4,0,0,0,0,0,16,5,0,0,0,0,0,9,15,1,0,0,0,0,0,13,5,0,0,0,0"
  "9|0,0,11,12,0,0,0,0,0,2,16,16,16,13,0,0,0,3,16,12,10,14,0,0,0,1,16,1,12,15,0,0,0,0,13,16,9,15,2,0,0,0,0,3,0,9,11,0,0,0,0,0,9,15,4,0,0,0,9,12,13,3,0,0"
)

render() {  # 8x8 grayscale (0-16) → ASCII art
    python3 -c "
vals = [int(v) for v in '$1'.split(',')]
chars = ' .:-=+*#%@'
for r in range(8):
    print('    ' + ''.join(chars[min(v * len(chars) // 17, len(chars)-1)] * 2 for v in vals[r*8:(r+1)*8]))"
}

predict() {
    curl -s --max-time 10 "$URL" -H 'Content-Type: application/json' \
        -d "{\"instances\": [[$1]]}" | grep -oE '[0-9]+' | tail -1
}

echo ""
echo "[3/3] Handwritten digit recognition:"
CORRECT=0
for s in "${SAMPLES[@]}"; do
    TRUE="${s%|*}"; FEAT="${s#*|}"
    render "$FEAT"
    T0=$(date +%s%N)
    P=$(predict "$FEAT")
    MS=$(( ($(date +%s%N) - T0) / 1000000 ))
    MARK="✗"; [ "$P" = "$TRUE" ] && { MARK="✓"; CORRECT=$((CORRECT+1)); }
    echo "    → predicted: ${P} (actual: ${TRUE}) ${MARK}  [${MS}ms]"
    echo ""
done
echo "  Result: ${CORRECT}/${#SAMPLES[@]} correct"

if [ "$TRAFFIC_SECONDS" -gt 0 ]; then
    echo ""
    echo "=== Traffic burst for ${TRAFFIC_SECONDS}s (watch the Grafana kserve panels) ==="
    END=$(( $(date +%s) + TRAFFIC_SECONDS ))
    COUNT=0; TOTAL_MS=0
    while [ "$(date +%s)" -lt "$END" ]; do
        s="${SAMPLES[$((RANDOM % ${#SAMPLES[@]}))]}"
        T0=$(date +%s%N)
        predict "${s#*|}" > /dev/null
        TOTAL_MS=$(( TOTAL_MS + ($(date +%s%N) - T0) / 1000000 ))
        COUNT=$((COUNT+1))
        [ $((COUNT % 100)) -eq 0 ] && echo "  [$(date +%H:%M:%S)] ${COUNT} requests, avg $((TOTAL_MS / COUNT))ms"
    done
    echo "  ✓ ${COUNT} requests in ${TRAFFIC_SECONDS}s (avg $((TOTAL_MS / COUNT))ms)"
fi

echo ""
echo "Method A summary: train → artifact on PVC → 8-line YAML. No serving code, no image."
echo "\$\$CMD[Check InferenceService](kubectl get isvc digits)"
exit 0
