#!/bin/bash
# Installs the CB-Tumblebug observability stack on one Ubuntu VM: nginx (TLS, single port 443) in front of an
# OpenTelemetry Collector gateway (OTLP/HTTP ingest with bearer token), Prometheus (remote write receiver),
# Loki (native OTLP) and Grafana (datasources and dashboards provisioned). Also mirrors the agent packages and
# the agent installer under /dl/ so nodes never download from GitHub.
# Embedded into cb-tumblebug (src/core/embedded) and run on the VM as root by POST /tumblebug/observability/stack.
#
# Required environment:
#   TB_OBS_IP                   public IP of this VM
#   TB_OBS_TOKEN                bearer token agents send
#   TB_OBS_GRAFANA_PASSWORD     Grafana admin password
#   TB_OBS_AGENT_INSTALLER_B64  base64 of install-otel-agent.sh, served at /dl/install-agent.sh
#   TB_OBS_DASHBOARDS_TGZ_B64   base64 tar.gz of the Grafana dashboards (provisioned from files)
# Optional:
#   TB_OBS_FQDN                 DNS name already pointing at TB_OBS_IP; enables a Let's Encrypt certificate.
#                               When empty a self-signed certificate (IP SAN) is used.
#   TB_OBS_RETENTION_DAYS       metrics/logs retention (default 15)
#   TB_OBS_OTELCOL_VERSION, TB_OBS_PROM_VERSION, TB_OBS_LOKI_VERSION, TB_OBS_GPU_EXPORTER_VERSION
#                               fallback versions when the GitHub releases API is unavailable
#
# Prints the server certificate between TB-CA-BEGIN/TB-CA-END markers (self-signed only).
set -eu
export DEBIAN_FRONTEND=noninteractive
IP="${TB_OBS_IP:?}"; TOKEN="${TB_OBS_TOKEN:?}"; GF_PASS="${TB_OBS_GRAFANA_PASSWORD:?}"
FQDN="${TB_OBS_FQDN:-}"; RET="${TB_OBS_RETENTION_DAYS:-15}"
HOST="${FQDN:-$IP}"
OTELCOL_V="${TB_OBS_OTELCOL_VERSION:-0.128.0}"; PROM_V="${TB_OBS_PROM_VERSION:-3.5.0}"; LOKI_V="${TB_OBS_LOKI_VERSION:-3.5.3}"; GPUEXP_V="${TB_OBS_GPU_EXPORTER_VERSION:-1.3.2}"
DL=/var/www/tb/dl
mkdir -p "$DL" /etc/tb-obs

apt-get update -y -qq
apt-get install -y -qq -o DPkg::Lock::Timeout=600 curl unzip openssl nginx jq apt-transport-https gnupg >/dev/null

# ---- helpers
gh_latest() { curl -fsSL --retry 3 "https://api.github.com/repos/$1/releases/latest" 2>/dev/null | jq -r '.tag_name // empty' | sed 's/^v//'; }
# dl <url> <dest>: retries dropped and stalled transfers; 22 is an HTTP error such as a missing release asset
dl() {
  local i rc
  for i in 1 2 3; do
    rc=0
    curl -fsSL --retry 5 --retry-delay 5 --connect-timeout 15 --speed-limit 10240 --speed-time 30 -o "$2" "$1" || rc=$?
    [ "$rc" = 0 ] && return 0
    rm -f "$2"
    [ "$rc" = 22 ] && break
    sleep $((i * 5))
  done
  return 1
}

# ---- OpenTelemetry Collector (gateway) + agent package mirror
V=$(gh_latest open-telemetry/opentelemetry-collector-releases); OTELCOL_V="${V:-$OTELCOL_V}"
for a in amd64 arm64; do for p in deb rpm; do
  dl "https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/v${OTELCOL_V}/otelcol-contrib_${OTELCOL_V}_linux_${a}.${p}" "$DL/otelcol-contrib_${a}.${p}" || echo "warn: otelcol-contrib ${a}.${p} not mirrored"
done; done
V=$(gh_latest utkuozdemir/nvidia_gpu_exporter); GPUEXP_V="${V:-$GPUEXP_V}"
for a in amd64 arm64; do for p in deb rpm; do
  dl "https://github.com/utkuozdemir/nvidia_gpu_exporter/releases/download/v${GPUEXP_V}/nvidia-gpu-exporter_${GPUEXP_V}_linux_${a}.${p}" "$DL/nvidia-gpu-exporter_${a}.${p}" \
    || dl "https://github.com/utkuozdemir/nvidia_gpu_exporter/releases/download/v${GPUEXP_V}/nvidia_gpu_exporter_${GPUEXP_V}_linux_${a}.${p}" "$DL/nvidia-gpu-exporter_${a}.${p}" \
    || echo "warn: nvidia-gpu-exporter ${a}.${p} not mirrored (GPU metrics unavailable for that arch)"
done; done
apt-get install -y -qq "$DL/otelcol-contrib_amd64.deb" >/dev/null
echo "$TB_OBS_AGENT_INSTALLER_B64" | base64 -d > "$DL/install-agent.sh"; chmod 644 "$DL/install-agent.sh"

# ---- trimmed agent collector (otelcol-tb): only the components install-agent.sh configures. No release carries
# this set, so it is built here: about an eighth of the contrib package to download and half its memory on a node.
# Best effort: when the build fails, agents install otelcol-contrib as before.
cat > /tmp/build-otelcol-tb.sh <<'BUILD'
set -e
W=/tmp/otelcol-tb-build; rm -rf "$W"; mkdir -p "$W/tmp"; export TMPDIR="$W/tmp"
GOV=$(curl -fsSL --retry 3 "https://go.dev/VERSION?m=text" | head -1)
dl "https://go.dev/dl/${GOV}.linux-$(dpkg --print-architecture).tar.gz" "$W/go.tgz"
tar -C "$W" -xzf "$W/go.tgz"
cat > "$W/builder.yaml" <<YAML
dist:
  name: otelcol-tb
  description: OpenTelemetry Collector with the components the CB-Tumblebug node agent uses
  output_path: $W/dist
  version: ${OTELCOL_V}
receivers:
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver v${OTELCOL_V}
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver v${OTELCOL_V}
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver v${OTELCOL_V}
processors:
  - gomod: go.opentelemetry.io/collector/processor/memorylimiterprocessor v${OTELCOL_V}
  - gomod: go.opentelemetry.io/collector/processor/batchprocessor v${OTELCOL_V}
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor v${OTELCOL_V}
exporters:
  - gomod: go.opentelemetry.io/collector/exporter/otlphttpexporter v${OTELCOL_V}
YAML
export HOME="$W" PATH="$W/go/bin:$PATH" GOPATH="$W/gopath" GOCACHE="$W/gocache" GOBIN="$W/bin" GOFLAGS=-modcacherw CGO_ENABLED=0
go install "go.opentelemetry.io/collector/cmd/builder@v${OTELCOL_V}"
for a in amd64 arm64; do
  GOOS=linux GOARCH=$a "$W/bin/builder" --config "$W/builder.yaml"
  tar -C "$W/dist" -czf "$DL/otelcol-tb_${a}.tar.gz" otelcol-tb
done
rm -rf "$W"
BUILD
export -f dl
if OTELCOL_V="$OTELCOL_V" DL="$DL" timeout 900 bash /tmp/build-otelcol-tb.sh >/tmp/build-otelcol-tb.log 2>&1; then
  OTELCOL_TB_V="$OTELCOL_V"; rm -f /tmp/build-otelcol-tb.sh /tmp/build-otelcol-tb.log
else
  tail -n 5 /tmp/build-otelcol-tb.log; rm -rf "$DL"/otelcol-tb_*.tar.gz /tmp/otelcol-tb-build
  OTELCOL_TB_V=""; echo "warn: trimmed agent collector not built; agents install otelcol-contrib"
fi
printf 'otelcol-contrib=%s\notelcol-tb=%s\nnvidia-gpu-exporter=%s\nprometheus=%s\nloki=%s\n' "$OTELCOL_V" "$OTELCOL_TB_V" "$GPUEXP_V" "$PROM_V" "$LOKI_V" > "$DL/VERSIONS"
# checksums let the agent installer detect a damaged download
(cd "$DL" && find . -maxdepth 1 -type f \( -name '*.deb' -o -name '*.rpm' -o -name '*.tar.gz' \) -printf '%f\n' | sort | xargs -r sha256sum > SHA256SUMS)

cat > /etc/otelcol-contrib/config.yaml <<YAML
extensions:
  bearertokenauth:
    scheme: Bearer
    token: '${TOKEN}'
  health_check:
    endpoint: 127.0.0.1:13133
receivers:
  otlp:
    protocols:
      http:
        endpoint: 127.0.0.1:4318
        auth:
          authenticator: bearertokenauth
processors:
  memory_limiter:
    check_interval: 5s
    limit_percentage: 75
    spike_limit_percentage: 15
  batch:
    timeout: 5s
    send_batch_size: 8192
exporters:
  prometheusremotewrite:
    endpoint: http://127.0.0.1:9090/api/v1/write
    resource_to_telemetry_conversion:
      enabled: true
    tls:
      insecure: true
  otlphttp/loki:
    endpoint: http://127.0.0.1:3100/otlp
    tls:
      insecure: true
service:
  extensions: [bearertokenauth, health_check]
  telemetry:
    metrics:
      level: none
  pipelines:
    metrics:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [prometheusremotewrite]
    logs:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [otlphttp/loki]
YAML

# ---- Prometheus
V=$(gh_latest prometheus/prometheus); PROM_V="${V:-$PROM_V}"
id prometheus >/dev/null 2>&1 || useradd --system --no-create-home --shell /usr/sbin/nologin prometheus
dl "https://github.com/prometheus/prometheus/releases/download/v${PROM_V}/prometheus-${PROM_V}.linux-amd64.tar.gz" /tmp/prom.tgz
tar -xzf /tmp/prom.tgz -C /tmp && install -m 755 "/tmp/prometheus-${PROM_V}.linux-amd64/prometheus" "/tmp/prometheus-${PROM_V}.linux-amd64/promtool" /usr/local/bin/
mkdir -p /etc/prometheus /var/lib/prometheus && chown prometheus:prometheus /var/lib/prometheus
cat > /etc/prometheus/prometheus.yml <<YAML
global:
  scrape_interval: 60s
scrape_configs:
  - job_name: prometheus
    static_configs: [{targets: ['127.0.0.1:9090']}]
YAML
cat > /etc/systemd/system/prometheus.service <<UNIT
[Unit]
Description=Prometheus (CB-Tumblebug observability)
After=network-online.target
[Service]
User=prometheus
ExecStart=/usr/local/bin/prometheus --config.file=/etc/prometheus/prometheus.yml --storage.tsdb.path=/var/lib/prometheus --storage.tsdb.retention.time=${RET}d --web.enable-remote-write-receiver --web.listen-address=127.0.0.1:9090
Restart=always
[Install]
WantedBy=multi-user.target
UNIT

# ---- Loki (single binary, filesystem storage, native OTLP)
V=$(curl -fsSL --retry 3 "https://api.github.com/repos/grafana/loki/releases?per_page=40" 2>/dev/null | jq -r '[.[].tag_name | select(startswith("v3."))][0] // empty' | sed 's/^v//'); LOKI_V="${V:-$LOKI_V}"
id loki >/dev/null 2>&1 || useradd --system --no-create-home --shell /usr/sbin/nologin loki
dl "https://github.com/grafana/loki/releases/download/v${LOKI_V}/loki-linux-amd64.zip" /tmp/loki.zip
unzip -oq /tmp/loki.zip -d /tmp && install -m 755 /tmp/loki-linux-amd64 /usr/local/bin/loki
mkdir -p /etc/loki /var/lib/loki && chown -R loki:loki /var/lib/loki
cat > /etc/loki/config.yaml <<YAML
auth_enabled: false
server:
  http_listen_address: 127.0.0.1
  http_listen_port: 3100
  grpc_listen_address: 127.0.0.1
  grpc_listen_port: 9096
common:
  path_prefix: /var/lib/loki
  instance_addr: 127.0.0.1   # ring members must dial the loopback gRPC listener, not the VM's private IP
  storage:
    filesystem:
      chunks_directory: /var/lib/loki/chunks
      rules_directory: /var/lib/loki/rules
  replication_factor: 1
  ring:
    kvstore:
      store: inmemory
schema_config:
  configs:
    - from: 2024-01-01
      store: tsdb
      object_store: filesystem
      schema: v13
      index:
        prefix: index_
        period: 24h
compactor:
  working_directory: /var/lib/loki/compactor
  retention_enabled: true
  delete_request_store: filesystem
limits_config:
  retention_period: ${RET}d
  allow_structured_metadata: true
  otlp_config:
    resource_attributes:
      attributes_config:
        - action: index_label
          attributes: [tb.ns, tb.infra, tb.nodegroup, tb.node, cloud.provider, cloud.region, host.name, service.name]
YAML
cat > /etc/systemd/system/loki.service <<UNIT
[Unit]
Description=Loki (CB-Tumblebug observability)
After=network-online.target
[Service]
User=loki
ExecStart=/usr/local/bin/loki -config.file=/etc/loki/config.yaml
Restart=always
[Install]
WantedBy=multi-user.target
UNIT

# ---- Grafana (apt repository), provisioned datasources and dashboards
if [ ! -f /etc/apt/keyrings/grafana.gpg ]; then
  mkdir -p /etc/apt/keyrings
  curl -fsSL https://apt.grafana.com/gpg.key | gpg --dearmor -o /etc/apt/keyrings/grafana.gpg
  echo "deb [signed-by=/etc/apt/keyrings/grafana.gpg] https://apt.grafana.com stable main" > /etc/apt/sources.list.d/grafana.list
  apt-get update -y -qq
fi
apt-get install -y -qq grafana >/dev/null
cat > /etc/grafana/grafana.ini <<INI
[paths]
data = /var/lib/grafana
logs = /var/log/grafana
plugins = /var/lib/grafana/plugins
provisioning = /etc/grafana/provisioning
[server]
http_addr = 127.0.0.1
http_port = 3000
root_url = https://${HOST}/
[security]
admin_user = admin
admin_password = ${GF_PASS}
cookie_secure = true
[users]
allow_sign_up = false
[auth.anonymous]
enabled = false
[analytics]
reporting_enabled = false
check_for_updates = false
[news]
news_feed_enabled = false
INI
mkdir -p /etc/grafana/provisioning/datasources /etc/grafana/provisioning/dashboards /var/lib/grafana/dashboards/tb
cat > /etc/grafana/provisioning/datasources/tb.yaml <<YAML
apiVersion: 1
datasources:
  - name: Prometheus
    uid: tb-prom
    type: prometheus
    access: proxy
    url: http://127.0.0.1:9090
    isDefault: true
    editable: false
  - name: Loki
    uid: tb-loki
    type: loki
    access: proxy
    url: http://127.0.0.1:3100
    editable: false
YAML
cat > /etc/grafana/provisioning/dashboards/tb.yaml <<YAML
apiVersion: 1
providers:
  - name: cb-tumblebug
    folder: CB-Tumblebug
    type: file
    disableDeletion: false
    allowUiUpdates: true
    options:
      path: /var/lib/grafana/dashboards/tb
YAML
# Dashboards are built into cb-tumblebug (src/core/embedded/dashboards) and arrive as a tar.gz; their panels use the
# labels the gateway produces from the agents' resource attributes (tb_ns, tb_infra, tb_nodegroup, tb_node).
echo "$TB_OBS_DASHBOARDS_TGZ_B64" | base64 -d | tar xz -C /var/lib/grafana/dashboards/tb
chown -R grafana:grafana /var/lib/grafana/dashboards

# ---- TLS: self-signed with IP SAN, or Let's Encrypt when a DNS name is given
mkdir -p /etc/tb-obs/tls
if [ ! -f /etc/tb-obs/tls/server.crt ]; then
  openssl req -x509 -nodes -newkey rsa:2048 -days 3650 -keyout /etc/tb-obs/tls/server.key -out /etc/tb-obs/tls/server.crt \
    -subj "/CN=${HOST}" -addext "subjectAltName=IP:${IP}${FQDN:+,DNS:$FQDN}" >/dev/null 2>&1
fi
CERT=/etc/tb-obs/tls/server.crt; KEY=/etc/tb-obs/tls/server.key

# ---- nginx: 443 -> Grafana (default), /v1/* -> collector OTLP/HTTP, /dl/* -> package mirror
rm -f /etc/nginx/sites-enabled/default
cat > /etc/nginx/sites-available/tb-obs <<NGINX
server {
    listen 80;
    server_name ${HOST};
    location /.well-known/acme-challenge/ { root /var/www/html; }
    location / { return 301 https://\$host\$request_uri; }
}
server {
    listen 443 ssl;
    http2 on;
    server_name ${HOST};
    ssl_certificate ${CERT};
    ssl_certificate_key ${KEY};
    client_max_body_size 64m;
    location /v1/ {
        proxy_pass http://127.0.0.1:4318;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_read_timeout 60s;
    }
    location /dl/ {
        alias ${DL}/;
        autoindex off;
    }
    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header X-Forwarded-Proto https;
    }
}
NGINX
ln -sf /etc/nginx/sites-available/tb-obs /etc/nginx/sites-enabled/tb-obs
nginx -t >/dev/null

systemctl daemon-reload
for svc in prometheus loki otelcol-contrib grafana-server nginx; do systemctl enable "$svc" >/dev/null 2>&1 || true; systemctl restart "$svc"; done

if [ -n "$FQDN" ]; then
  apt-get install -y -qq certbot python3-certbot-nginx >/dev/null
  certbot --nginx -d "$FQDN" --non-interactive --agree-tos --register-unsafely-without-email --no-redirect >/dev/null
  systemctl reload nginx
fi

# Services report ready some time after start (Loki holds /ready at 503 while its ring warms up), so each
# one is polled; a service that never becomes ready fails the install with its journal for diagnosis.
wait_ready() {
  for i in $(seq 1 60); do curl -fsS -o /dev/null "$2" 2>/dev/null && return 0; sleep 3; done
  echo "$1 did not become ready at $2"; journalctl -u "$1" -n 30 --no-pager; return 1
}
# grafana.ini's admin_password only applies when the database is created, so an install (or reinstall on a
# reused VM) starts Grafana from an empty database; dashboards and datasources are provisioned from files.
systemctl stop grafana-server
rm -f /var/lib/grafana/grafana.db
systemctl start grafana-server
wait_ready grafana-server http://127.0.0.1:3000/api/health
curl -fsS -u "admin:${GF_PASS}" -o /dev/null http://127.0.0.1:3000/api/org || { echo "grafana admin login failed after password reset"; exit 1; }
wait_ready prometheus http://127.0.0.1:9090/-/ready
wait_ready loki http://127.0.0.1:3100/ready
wait_ready otelcol-contrib http://127.0.0.1:13133/

if [ -z "$FQDN" ]; then
  echo "-----TB-CA-BEGIN-----"; cat "$CERT"; echo "-----TB-CA-END-----"
fi
cp "$CERT" "$DL/ca.pem" 2>/dev/null || true
echo "observability stack ready: https://${HOST}  (otelcol ${OTELCOL_V}, prometheus ${PROM_V}, loki ${LOKI_V}, grafana $(dpkg-query -W -f='${Version}' grafana 2>/dev/null))"
