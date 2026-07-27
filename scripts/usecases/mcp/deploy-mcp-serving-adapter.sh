#!/bin/bash

# Optional third MCP adapter: live KServe serving status via the K8s API (run on K8s control plane)
# Lets AI clients compare the registry catalog against models actually served by KServe.
# Use when KServe runs in the SAME cluster; re-run deploy-agentgateway.sh afterwards to federate it.

set -e

NS="mcp-demo"

echo "==== MCP Serving Adapter Setup (KServe reader, namespace: ${NS}) ===="

kubectl create namespace ${NS} 2>/dev/null || true

if kubectl get crd inferenceservices.serving.kserve.io > /dev/null 2>&1; then
    echo "  KServe CRD detected: live InferenceService listing will work"
else
    echo "  NOTE: KServe is not installed; tools will report that until it is"
fi

# Read-only RBAC for InferenceServices across all namespaces
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: mcp-kserve-serving
  namespace: ${NS}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: mcp-kserve-serving-read
rules:
  - apiGroups: ["serving.kserve.io"]
    resources: ["inferenceservices"]
    verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: mcp-kserve-serving-read
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: mcp-kserve-serving-read
subjects:
  - kind: ServiceAccount
    name: mcp-kserve-serving
    namespace: ${NS}
EOF

cat <<'EOF' | kubectl -n mcp-demo apply -f - > /dev/null
apiVersion: v1
kind: ConfigMap
metadata:
  name: mcp-kserve-serving-app
data:
  server.py: |
    import os
    import httpx
    from fastmcp import FastMCP

    SA = "/var/run/secrets/kubernetes.io/serviceaccount"
    API = os.environ.get("K8S_API", "https://kubernetes.default.svc")
    mcp = FastMCP("kserve-serving")

    def k8s_get(path):
        token = open(f"{SA}/token").read().strip()
        r = httpx.get(f"{API}{path}", headers={"Authorization": f"Bearer {token}"},
                      verify=f"{SA}/ca.crt", timeout=10)
        if r.status_code == 404:
            return None
        r.raise_for_status()
        return r.json()

    @mcp.tool
    def list_served_models() -> list:
        """List models currently served by KServe (InferenceServices in all namespaces)."""
        data = k8s_get("/apis/serving.kserve.io/v1beta1/inferenceservices")
        if data is None:
            return [{"info": "KServe is not installed in this cluster (no InferenceService CRD)"}]
        out = []
        for i in data.get("items", []):
            st = i.get("status", {})
            conds = {c.get("type"): c.get("status") for c in st.get("conditions", [])}
            model = i.get("spec", {}).get("predictor", {}).get("model", {})
            out.append({
                "name": i["metadata"]["name"],
                "namespace": i["metadata"]["namespace"],
                "ready": conds.get("Ready", "Unknown"),
                "url": st.get("url"),
                "model_format": (model.get("modelFormat") or {}).get("name", "custom"),
                "runtime": model.get("runtime"),
                "storage_uri": model.get("storageUri"),
            })
        return out or [{"info": "KServe is installed but no InferenceService is deployed"}]

    @mcp.tool
    def get_served_model(name: str, namespace: str = "default") -> dict:
        """Get one InferenceService's detailed status (conditions, predictor spec)."""
        i = k8s_get(f"/apis/serving.kserve.io/v1beta1/namespaces/{namespace}/inferenceservices/{name}")
        if i is None:
            return {"error": f"InferenceService '{name}' not found in namespace '{namespace}'"}
        st = i.get("status", {})
        return {
            "name": name, "namespace": namespace, "url": st.get("url"),
            "conditions": [{"type": c.get("type"), "status": c.get("status"),
                            "reason": c.get("reason")} for c in st.get("conditions", [])],
            "predictor": i.get("spec", {}).get("predictor"),
        }

    # stateless: any replica can serve any request (no per-pod session state)
    mcp.run(transport="http", host="0.0.0.0", port=8000, path="/mcp", stateless_http=True)
EOF

cat <<'EOF' | kubectl -n mcp-demo apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mcp-kserve-serving
spec:
  replicas: 1
  selector:
    matchLabels: { app: mcp-kserve-serving }
  template:
    metadata:
      labels: { app: mcp-kserve-serving }
    spec:
      serviceAccountName: mcp-kserve-serving
      containers:
        - name: mcp-kserve-serving
          image: python:3.11-slim
          workingDir: /app
          command: ["sh", "-c"]
          args:
            - pip install -q --root-user-action=ignore --disable-pip-version-check
              "fastmcp<3" httpx &&
              python3 server.py
          ports:
            - containerPort: 8000
          readinessProbe:
            tcpSocket: { port: 8000 }
            initialDelaySeconds: 15
            periodSeconds: 5
          volumeMounts:
            - name: app
              mountPath: /app
      volumes:
        - name: app
          configMap:
            name: mcp-kserve-serving-app
---
apiVersion: v1
kind: Service
metadata:
  name: mcp-kserve-serving
spec:
  selector: { app: mcp-kserve-serving }
  ports:
    - port: 8000
      targetPort: 8000
EOF

echo ""
echo "Waiting for the serving adapter to start (pip install at boot; typically 1-2 min)..."
kubectl -n ${NS} rollout restart deployment/mcp-kserve-serving > /dev/null
kubectl -n ${NS} rollout status deployment/mcp-kserve-serving --timeout=5m > /dev/null

# Smoke test via port-forward (works on control plane or remote kubectl)
mcp_rpc() {
    curl -s --max-time 15 -H 'Content-Type: application/json' \
        -H 'Accept: application/json, text/event-stream' \
        "http://127.0.0.1:18000/mcp" -d "$1" | sed -n 's/^data: //p; /^{/p' | tail -1
}
kubectl -n ${NS} port-forward svc/mcp-kserve-serving 18000:8000 > /dev/null 2>&1 &
PF_PID=$!
trap "kill ${PF_PID} 2>/dev/null" EXIT
sleep 3
echo ""
echo "=== Smoke test: serving tools ==="
mcp_rpc '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_served_models","arguments":{}}}' | python3 -c "
import sys, json
r = json.load(sys.stdin)['result']
obj = r.get('structuredContent') or json.loads(r['content'][0]['text'])
if isinstance(obj, dict) and set(obj) == {'result'}: obj = obj['result']
for row in obj[:8]:
    print('  ' + ' | '.join(f'{k}: {v}' for k, v in row.items() if v is not None))"

echo ""
echo "========================================"
echo "SUCCESS: KServe serving adapter is running"
echo "========================================"
echo "  adapter: http://mcp-kserve-serving.${NS}.svc.cluster.local:8000/mcp"
echo "  Next: re-run deploy-agentgateway.sh to federate it as the 'serving' target"
echo ""
echo "\$\$CMD[Check Serving Adapter](kubectl -n mcp-demo get pods -l app=mcp-kserve-serving)"
exit 0
