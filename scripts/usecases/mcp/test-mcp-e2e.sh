#!/bin/bash

# End-to-end MCP demo through agentgateway (run on K8s control plane)
# Story: browse the registry web page, then let an AI manage the catalog via MCP tools.
# Prerequisites: deploy-agentgateway.sh completed

set -e

NS="mcp-demo"
MCP_NODEPORT="30900"
WEB_NODEPORT="30902"

while [[ $# -gt 0 ]]; do
    case $1 in
        --nodeport) MCP_NODEPORT="$2"; shift 2 ;;
        *) echo "Usage: $0 [--nodeport <port>]"; exit 1 ;;
    esac
done

NODE_IP=$(hostname -I | awk '{print $1}')
GW_URL="http://${NODE_IP}:${MCP_NODEPORT}/mcp"

cat <<'BANNER'
┌────────────────────────────────────────────────────────────┐
│ MCP E2E DEMO · AI-operated model registry                  │
│  registry_* : curated catalog tools — the governed         │
│               write path (register / delete models)        │
│  db_*       : raw SQL — ad-hoc catalog analytics,          │
│               but READ-ONLY by policy                      │
└────────────────────────────────────────────────────────────┘
BANNER

mcp_rpc() {
    local SID="$1" BODY="$2"
    local HDRS=(-H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream')
    [ -n "$SID" ] && HDRS+=(-H "mcp-session-id: $SID")
    curl -s --max-time 20 "${HDRS[@]}" "$GW_URL" -d "$BODY" | sed -n 's/^data: //p; /^{/p' | tail -1
}

# Unwraps a tools/call result (structuredContent or JSON text content) and pretty-prints
show_result() {
    python3 -c "
import sys, json
d = json.load(sys.stdin)
if 'error' in d:
    print('  RPC error:', d['error'].get('message')); sys.exit(1)
r = d['result']
obj = r.get('structuredContent')
if obj is None and r.get('content'):
    try: obj = json.loads(r['content'][0]['text'])
    except Exception: obj = r['content'][0]['text']
if isinstance(obj, dict) and set(obj) == {'result'}:
    obj = obj['result']
rows = obj if isinstance(obj, list) else [obj]
for row in rows[:12]:
    if isinstance(row, dict):
        print('  ' + ' | '.join(f'{k}: {v}' for k, v in row.items() if k not in ('description',)))
    else:
        print('  ' + str(row))
if isinstance(obj, list) and len(obj) > 12:
    print(f'  ... ({len(obj)} rows total)')"
}

extract_id() {  # pulls .id out of a tools/call result (or empty)
    python3 -c "
import sys, json
try:
    d = json.load(sys.stdin)['result']
    obj = d.get('structuredContent') or json.loads(d['content'][0]['text'])
    if isinstance(obj, dict) and set(obj) == {'result'}: obj = obj['result']
    if isinstance(obj, list): obj = obj[0] if obj else {}
    print(obj.get('id', ''))
except Exception:
    print('')"
}

call_tool() {  # $1=tool name, $2=arguments json
    mcp_rpc "$SID" "{\"jsonrpc\":\"2.0\",\"id\":9,\"method\":\"tools/call\",\"params\":{\"name\":\"$1\",\"arguments\":$2}}"
}

echo ""
echo "[1/6] Connecting to the federated endpoint: ${GW_URL}"
SID=$(curl -si --max-time 15 "$GW_URL" \
    -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' \
    -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"e2e-demo","version":"1.0"}}}' \
    | grep -i '^mcp-session-id:' | tr -d '\r' | awk '{print $2}')
[ -n "$SID" ] || { echo "ERROR: initialize failed. Run deploy-agentgateway.sh first."; exit 1; }
mcp_rpc "$SID" '{"jsonrpc":"2.0","method":"notifications/initialized"}' > /dev/null

mcp_rpc "$SID" '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' | python3 -c "
import sys, json
tools = json.load(sys.stdin)['result']['tools']
print(f'  {len(tools)} tools federated behind ONE endpoint:')
for t in tools:
    print(f\"    {t['name']:28s} {(t.get('description') or '').strip().splitlines()[0][:56]}\")"

echo ""
echo "[2/6] Catalog path — registry_search_models (query: disease):"
call_tool registry_search_models '{"query": "disease"}' | show_result

echo ""
echo "[3/6] SQL path — db_query: analytics the REST API never anticipated:"
SQL="SELECT format, count(*) AS models, sum(downloads) AS downloads, bool_or(gpu_required) AS any_gpu FROM models GROUP BY format ORDER BY downloads DESC"
echo "  > ${SQL}"
call_tool db_query "{\"sql\": \"${SQL}\"}" | show_result

echo ""
echo "[4/6] SQL path is READ-ONLY — a write via db_query gets rejected:"
echo "  > DELETE FROM models"
call_tool db_query '{"sql": "DELETE FROM models"}' | show_result

echo ""
echo "[5/6] Writes go through the governed catalog path — registry_register_model:"
OLD_ID=$(call_tool registry_search_models '{"query": "e2e-strawberry-disease-cnn"}' | extract_id)
[ -n "$OLD_ID" ] && call_tool registry_delete_model "{\"model_id\": ${OLD_ID}}" > /dev/null
NEW=$(call_tool registry_register_model '{"name": "e2e-strawberry-disease-cnn", "task": "image-classification", "format": "onnx", "params_m": 4.2, "size_mb": 17, "gpu_required": false, "description": "Strawberry leaf disease classifier registered live by the E2E demo"}')
echo "$NEW" | show_result
NEW_ID=$(echo "$NEW" | extract_id)
echo "  verified via the SQL path:"
call_tool db_query '{"sql": "SELECT id, name, task, format FROM models ORDER BY id DESC LIMIT 1"}' | show_result
echo "  (watch the web page on :${WEB_NODEPORT} — the new model card appears on the next auto-refresh)"

echo ""
echo "[6/6] Cleanup — registry_delete_model removes the demo entry:"
[ -n "$NEW_ID" ] && call_tool registry_delete_model "{\"model_id\": ${NEW_ID}}" | show_result

echo ""
echo "========================================"
echo "E2E DEMO COMPLETE"
echo "========================================"
echo "  One MCP endpoint, two governed paths (catalog tools vs read-only SQL), live web view."
echo "  Connect your own client:  claude mcp add --transport http model-registry http://<node-public-ip>:${MCP_NODEPORT}/mcp"
echo ""
echo "\$\$CMD[Check MCP stack](kubectl -n mcp-demo get pods)"
exit 0
