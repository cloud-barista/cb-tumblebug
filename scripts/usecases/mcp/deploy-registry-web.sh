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
      .layout { display: flex; gap: 0; align-items: flex-start; }
      .grid { flex: 1; display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 14px; padding: 18px 28px 40px; }
      .card { background: #fff; border: 1px solid #e4e6eb; border-radius: 12px; padding: 16px; display: flex; flex-direction: column; gap: 8px; cursor: pointer; transition: box-shadow .15s; }
      .card:hover { border-color: #f59e0b; box-shadow: 0 2px 10px rgba(0,0,0,.06); }
      .card.new { border-color: #10b981; box-shadow: 0 0 0 3px rgba(16,185,129,.25); animation: pop .4s; }
      @keyframes pop { 0% { transform: scale(.96); } 100% { transform: scale(1); } }
      .name { font-weight: 650; font-size: 15px; word-break: break-all; }
      .badges { display: flex; gap: 6px; flex-wrap: wrap; }
      .badge { font-size: 11px; padding: 2px 8px; border-radius: 999px; background: #eef2ff; color: #4338ca; }
      .badge.format { background: #fef3c7; color: #92400e; }
      .badge.gpu { background: #fee2e2; color: #b91c1c; }
      .badge.newtag { background: #d1fae5; color: #047857; font-weight: 700; }
      .badge.serve-A { background: #dcfce7; color: #166534; }
      .badge.serve-B { background: #ffedd5; color: #9a3412; }
      .badge.serve-C { background: #e5e7eb; color: #374151; }
      .desc { font-size: 13px; color: #4b5563; min-height: 34px; }
      .meta { font-size: 12px; color: #6b7280; display: flex; gap: 12px; flex-wrap: wrap; border-top: 1px solid #f1f3f5; padding-top: 8px; }
      .meta b { color: #374151; }
      .empty { padding: 40px; color: #6b7280; text-align: center; grid-column: 1/-1; }
      .feed { width: 290px; min-width: 290px; margin: 18px 28px 40px 0; background: #fff; border: 1px solid #e4e6eb; border-radius: 12px; padding: 14px; max-height: 75vh; overflow-y: auto; }
      .feed h2 { font-size: 13px; text-transform: uppercase; letter-spacing: .04em; color: #6b7280; margin-bottom: 10px; }
      .feed .ev { font-size: 12.5px; padding: 7px 8px; border-radius: 8px; margin-bottom: 6px; background: #f8f9fb; border-left: 3px solid #d1d5db; }
      .feed .ev.add { border-left-color: #10b981; background: #ecfdf5; }
      .feed .ev.del { border-left-color: #ef4444; background: #fef2f2; }
      .feed .ev time { color: #9ca3af; font-size: 11px; display: block; }
      .feed .none { font-size: 12.5px; color: #9ca3af; }
      .modal-bg { position: fixed; inset: 0; background: rgba(15,17,21,.45); display: none; align-items: center; justify-content: center; z-index: 50; }
      .modal-bg.open { display: flex; }
      .modal { background: #fff; border-radius: 14px; width: min(680px, 92vw); max-height: 86vh; overflow-y: auto; padding: 24px; }
      .modal h2 { font-size: 19px; word-break: break-all; margin-bottom: 4px; }
      .modal .close { float: right; border: 0; background: #f1f3f5; border-radius: 8px; padding: 4px 10px; cursor: pointer; font-size: 14px; }
      .modal table { width: 100%; border-collapse: collapse; margin: 12px 0; font-size: 13px; }
      .modal td { padding: 6px 8px; border-bottom: 1px solid #f1f3f5; }
      .modal td:first-child { color: #6b7280; width: 130px; }
      .serve-box { border: 1px solid #e4e6eb; border-radius: 10px; padding: 12px 14px; margin-top: 6px; font-size: 13px; }
      .serve-box h3 { font-size: 13px; margin-bottom: 8px; }
      .serve-box .m { padding: 7px 9px; border-radius: 8px; margin-bottom: 5px; color: #4b5563; }
      .serve-box .m.pick { background: #ecfdf5; border: 1px solid #a7f3d0; color: #1a1c20; }
      .serve-box .m b { display: inline-block; min-width: 14px; }
      .serve-box code { background: #f1f3f5; padding: 1px 5px; border-radius: 5px; font-size: 12px; }
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
      <span class="live">&#9679; auto-refresh 5s</span>
    </div>
    <div class="layout">
      <div class="grid" id="grid"></div>
      <aside class="feed">
        <h2>&#9889; Live changes (via MCP)</h2>
        <div id="feed"><div class="none">no changes yet &mdash; register or delete a model with MCP tools and watch this feed</div></div>
      </aside>
    </div>
    <div class="modal-bg" id="modalbg"><div class="modal" id="modal"></div></div>
    <script>
      const $ = id => document.getElementById(id);
      const esc = s => String(s == null ? '' : s).replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
      let firstLoad = true, prev = new Map(), recentNew = new Set(), events = [], byId = new Map();

      // Serving-method classification (see scripts/usecases/kserve/examples):
      //  A standard-runtime (KServe): no serving code, no image build
      //  B custom-image (KServe): container build + registry required
      //  C plain-deployment (no KServe): manual API server + scaling
      const STD = ['sklearn','xgboost','lightgbm','onnx','tensorflow','pytorch','huggingface'];
      function serveInfo(fmt) {
        if (STD.includes(fmt)) return { m: 'A', label: 'A &middot; no build', build: false, kserve: true };
        if (fmt === 'custom')  return { m: 'B', label: 'B &middot; image build', build: true, kserve: true };
        return { m: 'C', label: 'C &middot; manual', build: true, kserve: false };
      }
      function fmtParams(p) {
        if (!p) return '-';
        return p >= 1000 ? (p/1000).toFixed(1) + 'B' : p + 'M';
      }
      function pushEvent(kind, r) {
        events.unshift({ kind, name: r.name, task: r.task, t: new Date() });
        events = events.slice(0, 30);
        $('feed').innerHTML = events.map(e => `
          <div class="ev ${e.kind}">
            ${e.kind === 'add' ? '&#10133; registered' : '&#10134; removed'}: <b>${esc(e.name)}</b> (${esc(e.task)})
            <time>${e.t.toLocaleTimeString()}</time>
          </div>`).join('');
      }
      function openModal(id) {
        const r = byId.get(id); if (!r) return;
        const s = serveInfo(r.format);
        const rows = [['Task', r.task], ['Format', r.format], ['Parameters', fmtParams(r.params_m)],
          ['Size', (r.size_mb || '-') + ' MB'], ['License', r.license || '-'],
          ['GPU required', r.gpu_required ? 'yes' : 'no'], ['Downloads', (r.downloads || 0).toLocaleString()],
          ['Updated', r.updated || '-'], ['Description', r.description || '-']];
        const m = (key, title, desc, script) => `
          <div class="m ${s.m === key ? 'pick' : ''}"><b>${key}</b> &mdash; ${title}${s.m === key ? ' &#11088;' : ''}<br>
          <span style="font-size:12px">${desc} &middot; <code>${script}</code></span></div>`;
        $('modal').innerHTML = `
          <button class="close" onclick="document.getElementById('modalbg').classList.remove('open')">&#10005;</button>
          <h2>${esc(r.name)}</h2>
          <div class="badges">
            <span class="badge">${esc(r.task)}</span><span class="badge format">${esc(r.format)}</span>
            ${r.gpu_required ? '<span class="badge gpu">GPU</span>' : ''}
            <span class="badge serve-${s.m}">serving ${s.label}</span>
          </div>
          <table>${rows.map(([k, v]) => `<tr><td>${k}</td><td>${esc(v)}</td></tr>`).join('')}</table>
          <div class="serve-box">
            <h3>How would this model be served? ${s.build ? '&#128230; container build required' : '&#9989; no container build'}</h3>
            ${m('A', 'KServe standard runtime', 'zero serving code, zero image build &mdash; model file + 8-line InferenceService YAML', 'examples/a-sklearn-isvc.sh')}
            ${m('B', 'KServe custom image', 'build a container image, push to a registry, custom-predictor InferenceService', 'examples/build-serve-custom-model.sh')}
            ${m('C', 'Plain Deployment (no KServe)', 'write the API server yourself; manual scaling &amp; rollout', 'examples/c-plain-deployment.sh')}
            <div style="font-size:12px;color:#6b7280;margin-top:6px">Ask the MCP assistant: <code>get_serving_guide</code> explains the recommended path for any model.</div>
          </div>`;
        $('modalbg').classList.add('open');
      }
      $('modalbg').addEventListener('click', e => { if (e.target === $('modalbg')) $('modalbg').classList.remove('open'); });
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
        }
        // Diff against the previous unfiltered view to build the live feed
        const unfiltered = !$('q').value && !$('task').value && !$('format').value;
        if (unfiltered) {
          const cur = new Map(rows.map(r => [r.id, r]));
          if (!firstLoad) {
            for (const [id, r] of cur) if (!prev.has(id)) { pushEvent('add', r); recentNew.add(id); setTimeout(() => recentNew.delete(id), 20000); }
            for (const [id, r] of prev) if (!cur.has(id)) pushEvent('del', r);
          }
          prev = cur;
        }
        byId = new Map(rows.map(r => [r.id, r]));
        firstLoad = false;
        $('grid').innerHTML = rows.map(r => {
          const s = serveInfo(r.format);
          return `
          <div class="card ${recentNew.has(r.id) ? 'new' : ''}" onclick="openModal(${r.id})">
            <div class="name">${esc(r.name)}</div>
            <div class="badges">
              ${recentNew.has(r.id) ? '<span class="badge newtag">NEW</span>' : ''}
              <span class="badge">${esc(r.task)}</span>
              <span class="badge format">${esc(r.format)}</span>
              ${r.gpu_required ? '<span class="badge gpu">GPU</span>' : ''}
              <span class="badge serve-${s.m}">${s.label}</span>
            </div>
            <div class="desc">${esc(r.description)}</div>
            <div class="meta">
              <span><b>${fmtParams(r.params_m)}</b> params</span>
              <span><b>${r.size_mb || '-'}</b> MB</span>
              <span>&#8595; <b>${(r.downloads || 0).toLocaleString()}</b></span>
              <span>${esc(r.license)}</span>
              <span>${esc(r.updated)}</span>
            </div>
          </div>`; }).join('') || '<div class="empty">no models match</div>';
      }
      ['q', 'task', 'format'].forEach(id => $(id).addEventListener('input', load));
      setInterval(load, 5000);
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
kubectl -n ${NS} rollout restart deployment/model-registry-web > /dev/null
kubectl -n ${NS} rollout status deployment/model-registry-web --timeout=3m > /dev/null

NODE_IP=$(hostname -I | awk '{print $1}')
echo ""
echo "=== Smoke test via NodePort ==="
curl -s --max-time 10 "http://${NODE_IP}:${WEB_NODEPORT}/" | grep -q "Model Registry" && echo "  page OK"
# Retry: NodePort endpoints may lag a few seconds after the rollout
for i in {1..10}; do
    if curl -s --max-time 5 "http://${NODE_IP}:${WEB_NODEPORT}/api/models" | python3 -c "
import sys, json
print(f'  /api proxy OK ({len(json.load(sys.stdin))} models)')" 2>/dev/null; then
        break
    fi
    sleep 3
done

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
