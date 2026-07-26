#!/bin/bash

# Model Registry web page (nginx) for the MCP usecase (run on K8s control plane)
# Hugging Face-style catalog browser on NodePort 30902; /api/* proxies to the backend.
# Prerequisites: deploy-registry-backend.sh completed

set -e

NS="mcp-demo"
WEB_NODEPORT="30902"

while [[ $# -gt 0 ]]; do
    case $1 in
        --nodeport) WEB_NODEPORT="$2"; shift 2 ;;
        *) echo "Usage: $0 [--nodeport <port>]"; exit 1 ;;
    esac
done

echo "==== Model Registry Web Setup (namespace: ${NS}, NodePort: ${WEB_NODEPORT}) ===="

kubectl -n ${NS} get svc model-registry-backend > /dev/null 2>&1 || {
    echo "ERROR: model-registry-backend service not found in ${NS}. Run deploy-registry-backend.sh first."; exit 1; }

cat <<'EOF' | kubectl -n mcp-demo apply -f - > /dev/null
apiVersion: v1
kind: ConfigMap
metadata:
  name: registry-web-conf
data:
  default.conf: |
    server {
      listen 8080;
      location /api/ {
        proxy_pass http://model-registry-backend:8000/;
      }
      location / {
        root /usr/share/nginx/html;
        index index.html;
      }
    }
EOF

cat <<'EOF' | kubectl -n mcp-demo apply -f - > /dev/null
apiVersion: v1
kind: ConfigMap
metadata:
  name: registry-web-html
data:
  index.html: |
    <!DOCTYPE html>
    <html lang="en">
    <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Model Registry</title>
    <style>
      * { box-sizing: border-box; margin: 0; }
      body { font-family: -apple-system, 'Segoe UI', Roboto, sans-serif; background: #f8f9fb; color: #1a1c20; }
      header { background: #fff; border-bottom: 1px solid #e4e6eb; padding: 18px 28px; display: flex; align-items: baseline; gap: 14px; flex-wrap: wrap; }
      header h1 { font-size: 22px; }
      header .sub { color: #6b7280; font-size: 13px; }
      header .stats { margin-left: auto; font-size: 13px; color: #6b7280; }
      header .stats b { color: #1a1c20; }
      .controls { display: flex; gap: 10px; padding: 16px 28px 0; flex-wrap: wrap; align-items: center; }
      .controls input, .controls select { padding: 8px 12px; border: 1px solid #d1d5db; border-radius: 8px; font-size: 14px; background: #fff; }
      .controls input { width: 260px; }
      .controls .live { font-size: 12px; color: #059669; margin-left: auto; }
      .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); gap: 14px; padding: 18px 28px 40px; }
      .card { background: #fff; border: 1px solid #e4e6eb; border-radius: 12px; padding: 16px; display: flex; flex-direction: column; gap: 8px; }
      .card:hover { border-color: #f59e0b; box-shadow: 0 2px 10px rgba(0,0,0,.06); }
      .name { font-weight: 650; font-size: 15px; word-break: break-all; }
      .badges { display: flex; gap: 6px; flex-wrap: wrap; }
      .badge { font-size: 11px; padding: 2px 8px; border-radius: 999px; background: #eef2ff; color: #4338ca; }
      .badge.format { background: #fef3c7; color: #92400e; }
      .badge.gpu { background: #fee2e2; color: #b91c1c; }
      .desc { font-size: 13px; color: #4b5563; min-height: 34px; }
      .meta { font-size: 12px; color: #6b7280; display: flex; gap: 12px; flex-wrap: wrap; border-top: 1px solid #f1f3f5; padding-top: 8px; }
      .meta b { color: #374151; }
      .empty { padding: 40px; color: #6b7280; text-align: center; grid-column: 1/-1; }
    </style>
    </head>
    <body>
    <header>
      <h1>&#127806; Model Registry</h1>
      <span class="sub">CB-Tumblebug MCP demo &mdash; browse here, control via MCP tools</span>
      <span class="stats"><b id="count">-</b> models &middot; <b id="dl">-</b> downloads</span>
    </header>
    <div class="controls">
      <input id="q" placeholder="Search models... (name, description)">
      <select id="task"><option value="">All tasks</option></select>
      <select id="format"><option value="">All formats</option></select>
      <span class="live">&#9679; auto-refresh 8s</span>
    </div>
    <div class="grid" id="grid"></div>
    <script>
      const $ = id => document.getElementById(id);
      let firstLoad = true;
      function fmtParams(p) {
        if (!p) return '-';
        return p >= 1000 ? (p/1000).toFixed(1) + 'B' : p + 'M';
      }
      async function load() {
        const u = '/api/models?query=' + encodeURIComponent($('q').value)
                + '&task=' + encodeURIComponent($('task').value)
                + '&format=' + encodeURIComponent($('format').value);
        let rows;
        try { rows = await (await fetch(u)).json(); }
        catch (e) { $('grid').innerHTML = '<div class="empty">backend unreachable</div>'; return; }
        $('count').textContent = rows.length;
        $('dl').textContent = rows.reduce((s, r) => s + (r.downloads || 0), 0).toLocaleString();
        if (firstLoad) {
          const opts = (id, vals) => vals.forEach(v => {
            const o = document.createElement('option'); o.value = o.textContent = v; $(id).appendChild(o);
          });
          opts('task', [...new Set(rows.map(r => r.task))]);
          opts('format', [...new Set(rows.map(r => r.format))]);
          firstLoad = false;
        }
        $('grid').innerHTML = rows.map(r => `
          <div class="card">
            <div class="name">${r.name}</div>
            <div class="badges">
              <span class="badge">${r.task}</span>
              <span class="badge format">${r.format}</span>
              ${r.gpu_required ? '<span class="badge gpu">GPU</span>' : ''}
            </div>
            <div class="desc">${r.description || ''}</div>
            <div class="meta">
              <span><b>${fmtParams(r.params_m)}</b> params</span>
              <span><b>${r.size_mb || '-'}</b> MB</span>
              <span>&#8595; <b>${(r.downloads || 0).toLocaleString()}</b></span>
              <span>${r.license || ''}</span>
              <span>${r.updated || ''}</span>
            </div>
          </div>`).join('') || '<div class="empty">no models match</div>';
      }
      ['q', 'task', 'format'].forEach(id => $(id).addEventListener('input', load));
      setInterval(load, 8000);
      load();
    </script>
    </body>
    </html>
EOF

cat <<EOF | kubectl -n ${NS} apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: model-registry-web
spec:
  replicas: 1
  selector:
    matchLabels: { app: model-registry-web }
  template:
    metadata:
      labels: { app: model-registry-web }
    spec:
      containers:
        - name: nginx
          image: nginx:1.27-alpine
          ports:
            - containerPort: 8080
          readinessProbe:
            httpGet: { path: /, port: 8080 }
            initialDelaySeconds: 3
            periodSeconds: 5
          volumeMounts:
            - name: conf
              mountPath: /etc/nginx/conf.d
            - name: html
              mountPath: /usr/share/nginx/html
      volumes:
        - name: conf
          configMap:
            name: registry-web-conf
        - name: html
          configMap:
            name: registry-web-html
---
apiVersion: v1
kind: Service
metadata:
  name: model-registry-web
spec:
  type: NodePort
  selector: { app: model-registry-web }
  ports:
    - port: 8080
      targetPort: 8080
      nodePort: ${WEB_NODEPORT}
EOF

echo ""
echo "Waiting for the web page to start..."
kubectl -n ${NS} rollout status deployment/model-registry-web --timeout=3m > /dev/null

NODE_IP=$(hostname -I | awk '{print $1}')
echo ""
echo "=== Smoke test via NodePort ==="
curl -s --max-time 10 "http://${NODE_IP}:${WEB_NODEPORT}/" | grep -q "Model Registry" && echo "  page OK"
curl -s --max-time 10 "http://${NODE_IP}:${WEB_NODEPORT}/api/models" | python3 -c "
import sys, json
print(f'  /api proxy OK ({len(json.load(sys.stdin))} models)')"

echo ""
echo "========================================"
echo "SUCCESS: Model Registry web is running"
echo "========================================"
echo ""
echo "[MODEL_REGISTRY_WEB]"
echo "http://<node-public-ip>:${WEB_NODEPORT}"
echo ""
echo "  (1) Allow inbound ${WEB_NODEPORT}/tcp in the Security Group"
echo "  (2) The page auto-refreshes; models registered/deleted via MCP appear live"
echo ""
echo "\$\$CMD[Check Registry Web](kubectl -n mcp-demo get pods -l app=model-registry-web)"
exit 0
