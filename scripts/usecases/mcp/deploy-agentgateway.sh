#!/bin/bash

# agentgateway (standalone) federating the model-registry MCP adapters (run on K8s control plane)
# Routes on NodePort 30900: /mcp (all tools), /mcp-registry (catalog view), /mcp-serving (KServe view)
# Admin UI on NodePort 30901. Prerequisites: deploy-mcp-servers.sh completed

set -e

NS="mcp-demo"
AGW_VERSION="v1.3.1"
MCP_NODEPORT="30900"
UI_NODEPORT="30901"

usage() {
    echo "Usage: $0 [--version <tag>] [--nodeport <port>] [--ui-nodeport <port>]"
    exit 1
}

while [[ $# -gt 0 ]]; do
    case $1 in
        --version) AGW_VERSION="$2"; shift 2 ;;
        --nodeport) MCP_NODEPORT="$2"; shift 2 ;;
        --ui-nodeport) UI_NODEPORT="$2"; shift 2 ;;
        -h|--help) usage ;;
        *) echo "Unknown option: $1"; usage ;;
    esac
done

echo "==== agentgateway Setup (${AGW_VERSION}, namespace: ${NS}) ===="

for SVC in mcp-model-registry-backend mcp-model-registry-db; do
    kubectl -n ${NS} get svc ${SVC} > /dev/null 2>&1 || {
        echo "ERROR: ${SVC} service not found in ${NS}. Run deploy-mcp-servers.sh first."; exit 1; }
done

# Optional serving adapter (deploy-mcp-serving-adapter.sh): adds a route view and a target
SERVING_ROUTE=""
SERVING_TARGET=""
if kubectl -n ${NS} get svc mcp-kserve-serving > /dev/null 2>&1; then
    echo "  Detected mcp-kserve-serving: adding /mcp-serving route and 'serving' target"
    SERVING_ROUTE="
        - matches:
          - path:
              exact: /mcp-serving
          policies:
            cors:
              allowOrigins: [\"*\"]
              allowHeaders: [\"*\"]
          backends:
          - mcp:
              targets:
              - name: serving
                mcp:
                  host: http://mcp-kserve-serving.${NS}.svc.cluster.local:8000/mcp"
    SERVING_TARGET="
              - name: serving
                mcp:
                  host: http://mcp-kserve-serving.${NS}.svc.cluster.local:8000/mcp"
fi

# Routes: /mcp = everything; /mcp-registry and /mcp-serving = per-consumer tool views
# (a route with a single target exposes tool names without the target prefix)
cat <<EOF | kubectl -n ${NS} apply -f - > /dev/null
apiVersion: v1
kind: ConfigMap
metadata:
  name: agentgateway-config
data:
  config.yaml: |
    config:
      adminAddr: "0.0.0.0:15000"
    binds:
    - port: 3000
      listeners:
      - routes:
        - matches:
          - path:
              exact: /mcp-registry
          policies:
            cors:
              allowOrigins: ["*"]
              allowHeaders: ["*"]
          backends:
          - mcp:
              targets:
              - name: registry
                mcp:
                  host: http://mcp-model-registry-backend.${NS}.svc.cluster.local:8000/mcp
              - name: db
                mcp:
                  host: http://mcp-model-registry-db.${NS}.svc.cluster.local:8000/mcp${SERVING_ROUTE}
        - matches:
          - path:
              exact: /mcp
          policies:
            cors:
              allowOrigins: ["*"]
              allowHeaders: ["*"]
          backends:
          - mcp:
              targets:
              - name: registry
                mcp:
                  host: http://mcp-model-registry-backend.${NS}.svc.cluster.local:8000/mcp
              - name: db
                mcp:
                  host: http://mcp-model-registry-db.${NS}.svc.cluster.local:8000/mcp${SERVING_TARGET}
EOF

echo ""
echo "=== Federation config (this is the entire gateway setup) ==="
kubectl -n ${NS} get configmap agentgateway-config -o jsonpath='{.data.config\.yaml}' | sed 's/^/  /'

cat <<EOF | kubectl -n ${NS} apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: agentgateway
spec:
  replicas: 1
  selector:
    matchLabels: { app: agentgateway }
  template:
    metadata:
      labels: { app: agentgateway }
    spec:
      containers:
        - name: agentgateway
          image: ghcr.io/agentgateway/agentgateway:${AGW_VERSION}
          args: ["-f", "/config/config.yaml"]
          ports:
            - containerPort: 3000
            - containerPort: 15000
          readinessProbe:
            tcpSocket: { port: 3000 }
            initialDelaySeconds: 3
            periodSeconds: 5
          volumeMounts:
            - name: config
              mountPath: /config
      volumes:
        - name: config
          configMap:
            name: agentgateway-config
---
apiVersion: v1
kind: Service
metadata:
  name: agentgateway
spec:
  type: NodePort
  selector: { app: agentgateway }
  ports:
    - name: mcp
      port: 3000
      targetPort: 3000
      nodePort: ${MCP_NODEPORT}
    - name: ui
      port: 15000
      targetPort: 15000
      nodePort: ${UI_NODEPORT}
EOF

echo ""
echo "Waiting for agentgateway to start..."
kubectl -n ${NS} rollout restart deployment/agentgateway > /dev/null
kubectl -n ${NS} rollout status deployment/agentgateway --timeout=5m > /dev/null

# Verify through the NodePort (the same path external clients use)
NODE_IP=$(hostname -I | awk '{print $1}')
GW_URL="http://${NODE_IP}:${MCP_NODEPORT}/mcp"

mcp_rpc() {
    local SID="$1" BODY="$2"
    local HDRS=(-H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream')
    [ -n "$SID" ] && HDRS+=(-H "mcp-session-id: $SID")
    curl -s --max-time 15 "${HDRS[@]}" "$GW_URL" -d "$BODY" | sed -n 's/^data: //p; /^{/p' | tail -1
}

echo ""
echo "=== Verifying federated endpoint via NodePort (${GW_URL}) ==="
SID=""
for i in $(seq 1 12); do
    SID=$(curl -si --max-time 15 "$GW_URL" \
        -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' \
        -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"probe","version":"1.0"}}}' \
        | grep -i '^mcp-session-id:' | tr -d '\r' | awk '{print $2}')
    [ -n "$SID" ] && break
    echo "  [$(date +%H:%M:%S)] gateway not answering yet; retrying..."
    sleep 5
done
[ -n "$SID" ] || { echo "ERROR: initialize via gateway failed"; kubectl -n ${NS} logs deploy/agentgateway --tail=20; exit 1; }

mcp_rpc "$SID" '{"jsonrpc":"2.0","method":"notifications/initialized"}' > /dev/null
TOOLS_JSON=$(mcp_rpc "$SID" '{"jsonrpc":"2.0","id":2,"method":"tools/list"}')
echo "$TOOLS_JSON" | python3 -c "
import sys, json
tools = json.load(sys.stdin)['result']['tools']
names = [t['name'] for t in tools]
print(f'  {len(tools)} federated tools:')
for n in names:
    print(f'    {n}')
assert any(n.startswith('registry_') for n in names), 'registry_* tools missing'
assert any(n.startswith('db_') for n in names), 'db_* tools missing'
print('  OK: both registry_* and db_* tool groups are federated')"
if [ -n "$SERVING_TARGET" ]; then
    echo "$TOOLS_JSON" | grep -q '"serving_' && echo "  OK: serving_* tools federated (KServe adapter)" \
        || { echo "ERROR: serving_* tools missing"; exit 1; }
fi

route_tools() {
    local URL="http://${NODE_IP}:${MCP_NODEPORT}$1"
    local S
    S=$(curl -si --max-time 15 "$URL" \
        -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' \
        -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"probe","version":"1.0"}}}' \
        | grep -i '^mcp-session-id:' | tr -d '\r' | awk '{print $2}')
    curl -s --max-time 15 -H 'Content-Type: application/json' \
        -H 'Accept: application/json, text/event-stream' -H "mcp-session-id: $S" "$URL" \
        -d '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' | sed -n 's/^data: //p; /^{/p' | tail -1 \
        | python3 -c "import sys, json; print(len(json.load(sys.stdin)['result']['tools']))"
}

echo ""
echo "=== Per-consumer route views ==="
echo "  /mcp-registry: $(route_tools /mcp-registry) tools (catalog + read-only SQL)"
[ -n "$SERVING_TARGET" ] && echo "  /mcp-serving:  $(route_tools /mcp-serving) tools (KServe view, unprefixed names)"

echo ""
echo "========================================"
echo "SUCCESS: agentgateway is federating the MCP adapters"
echo "========================================"
echo ""
echo "[MCP_ENDPOINT]"
echo "http://<node-public-ip>:${MCP_NODEPORT}/mcp"
echo ""
echo "[MCP_ENDPOINT_REGISTRY] (catalog + SQL tools only)"
echo "http://<node-public-ip>:${MCP_NODEPORT}/mcp-registry"
if [ -n "$SERVING_TARGET" ]; then
    echo ""
    echo "[MCP_ENDPOINT_SERVING] (KServe serving view only)"
    echo "http://<node-public-ip>:${MCP_NODEPORT}/mcp-serving"
fi
echo ""
echo "[AGENTGATEWAY_UI]"
echo "http://<node-public-ip>:${UI_NODEPORT}/ui/"
echo ""
echo "External access checklist:"
echo "  (1) Allow inbound ${MCP_NODEPORT}/tcp (and ${UI_NODEPORT}/tcp for the UI) in the Security Group"
echo "  (2) Register in Claude Code:  claude mcp add --transport http model-registry http://<node-public-ip>:${MCP_NODEPORT}/mcp"
echo "      (register /mcp-registry or /mcp-serving instead to give a client a narrower tool view)"
echo "  (3) Register in VS Code Copilot: Ctrl+Shift+P -> 'MCP: Add Server...' -> HTTP -> paste the URL above"
echo "      then open Copilot Chat, switch to Agent mode, and enable the tools (wrench icon)"
echo "  (4) Or inspect with:          npx @modelcontextprotocol/inspector  (Streamable HTTP)"
echo "  (5) Demo endpoint has NO auth — delete the infra (or close the SG port) after the demo"
echo ""
echo "\$\$CMD[Check agentgateway](kubectl -n mcp-demo get pods -l app=agentgateway)"
exit 0
