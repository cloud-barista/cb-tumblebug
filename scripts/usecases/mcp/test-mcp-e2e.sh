#!/bin/bash

# End-to-end MCP demo through agentgateway (run on K8s control plane)
# Shows the two access paths to the same data: curated API tools vs read-only SQL
# Prerequisites: deploy-agentgateway.sh completed

set -e

NS="mcp-demo"
MCP_NODEPORT="30900"

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
│ MCP E2E DEMO · one gateway, two paths to the same data     │
│  api_* tools : curated REST wrappers — the governed        │
│                write path (create_order checks stock)      │
│  db_*  tools : raw SQL — powerful ad-hoc analysis,         │
│                but READ-ONLY by policy                     │
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
        print('  ' + ' | '.join(f'{k}: {v}' for k, v in row.items()))
    else:
        print('  ' + str(row))
if isinstance(obj, list) and len(obj) > 12:
    print(f'  ... ({len(obj)} rows total)')"
}

call_tool() {  # $1=tool name, $2=arguments json
    mcp_rpc "$SID" "{\"jsonrpc\":\"2.0\",\"id\":9,\"method\":\"tools/call\",\"params\":{\"name\":\"$1\",\"arguments\":$2}}"
}

echo ""
echo "[1/5] Connecting to the federated endpoint: ${GW_URL}"
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
    print(f\"    {t['name']:24s} {(t.get('description') or '').strip().splitlines()[0][:58]}\")"

echo ""
echo "[2/5] API path — api_list_products (curated tool):"
call_tool api_list_products '{}' | show_result

echo ""
echo "[3/5] SQL path — db_query: ad-hoc revenue analysis the API never anticipated:"
SQL="SELECT p.name, SUM(oi.quantity) AS units, SUM(oi.quantity*oi.unit_price) AS revenue FROM order_items oi JOIN products p ON p.id=oi.product_id GROUP BY p.name ORDER BY revenue DESC LIMIT 5"
echo "  > ${SQL}"
call_tool db_query "{\"sql\": \"${SQL}\"}" | show_result

echo ""
echo "[4/5] SQL path is READ-ONLY — a write via db_query gets rejected:"
echo "  > UPDATE products SET stock = 0"
call_tool db_query '{"sql": "UPDATE products SET stock = 0"}' | show_result

echo ""
echo "[5/5] Writes go through the governed API path — api_create_order:"
BEFORE=$(call_tool db_query '{"sql": "SELECT stock FROM products WHERE id=5"}')
echo "  stock of product #5 before:"; echo "$BEFORE" | show_result
call_tool api_create_order '{"customer_id": 5, "product_id": 5, "quantity": 2}' | show_result
echo "  stock of product #5 after (verified via the SQL path):"
call_tool db_query '{"sql": "SELECT stock FROM products WHERE id=5"}' | show_result

echo ""
echo "========================================"
echo "E2E DEMO COMPLETE"
echo "========================================"
echo "  Same data, two governed paths, one MCP endpoint — that is agentgateway federation."
echo "  Connect your own client:  claude mcp add --transport http shop-demo http://<node-public-ip>:${MCP_NODEPORT}/mcp"
echo ""
echo "\$\$CMD[Check MCP stack](kubectl -n mcp-demo get pods)"
exit 0
