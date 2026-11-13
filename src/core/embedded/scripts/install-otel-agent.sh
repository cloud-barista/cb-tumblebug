#!/bin/bash
# Installs the OpenTelemetry Collector as the host agent of a CB-Tumblebug node and points it at the
# observability stack. Served by the stack at /dl/install-agent.sh and run by the initscript-otel-agent template.
# The trimmed build (otelcol-tb: only the components this agent uses) is installed when the stack offers it;
# otelcol-contrib is used for nodes that scrape Prometheus endpoints (GPU, OTEL_SCRAPE_TARGETS, a local vLLM server)
# or as the fallback.
#
# Required (exported by CB-Tumblebug into every init script when an observability stack is registered):
#   TB_OTEL_ENDPOINT   OTLP/HTTP base URL, e.g. https://obs.example.com (also hosts /dl/ with the packages)
#   TB_OTEL_TOKEN      bearer token
# Optional:
#   TB_OTEL_CA_B64     base64 PEM of the server CA (self-signed stacks)
#   TB_NS_ID TB_INFRA_ID TB_NODEGROUP_ID TB_NODE_ID TB_CSP TB_REGION    node identity -> resource attributes
#   TB_ACCELERATOR_TYPE TB_ACCELERATOR_MODEL TB_ACCELERATOR_COUNT        from the node spec; gpu enables GPU metrics
#   OTEL_METRICS=on|off  OTEL_LOGS=on|off  OTEL_GPU=auto|on|off  OTEL_INTERVAL=60s
#   OTEL_LOG_PATHS       comma-separated file globs to tail in addition to journald
#   OTEL_SCRAPE_TARGETS  comma-separated host:port Prometheus endpoints to scrape (e.g. 127.0.0.1:8000 for vLLM)
#   OTEL_DISCOVER        auto (default) | off: scrape local vLLM servers without naming them, also ones started later
#   OTEL_DIST            auto (default) | trimmed | contrib
#   OTEL_PACKAGE_URL     base URL of the package mirror (default: TB_OTEL_ENDPOINT/dl), e.g. a CDN for large fleets
set -eu
: "${TB_OTEL_ENDPOINT:?TB_OTEL_ENDPOINT is required}"
ENDPOINT="${TB_OTEL_ENDPOINT%/}"
TOKEN="${TB_OTEL_TOKEN:-}"
METRICS="${OTEL_METRICS:-on}"; LOGS="${OTEL_LOGS:-on}"; GPU="${OTEL_GPU:-auto}"; INTERVAL="${OTEL_INTERVAL:-60s}"
DIST="${OTEL_DIST:-auto}"
DISCOVER="${OTEL_DISCOVER:-auto}"
PKG_URL="${OTEL_PACKAGE_URL:-$ENDPOINT/dl}"; PKG_URL="${PKG_URL%/}"
TB_DIR=/var/lib/tb
mkdir -p "$TB_DIR"
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
# one run at a time: tb-otel-discover re-applies this installer on its own
exec 9> "$TB_DIR/otel-agent.lock"
flock -w 300 9 2>/dev/null || true

CA_OPT=""; CA_FILE=""
if [ -n "${TB_OTEL_CA_B64:-}" ]; then
  echo "$TB_OTEL_CA_B64" | base64 -d > "$TB_DIR/otel-ca.pem"; chmod 644 "$TB_DIR/otel-ca.pem"
  CA_FILE="$TB_DIR/otel-ca.pem"; CA_OPT="--cacert $CA_FILE"
fi

case "$(uname -m)" in x86_64|amd64) ARCH=amd64 ;; aarch64|arm64) ARCH=arm64 ;; *) echo "unsupported architecture $(uname -m)"; exit 2 ;; esac
if command -v dpkg >/dev/null 2>&1; then PKG=deb; elif command -v rpm >/dev/null 2>&1; then PKG=rpm; else echo "unsupported package manager"; exit 2; fi

# SUMS holds the mirror's checksums when the stack publishes them (stacks installed earlier do not)
# shellcheck disable=SC2086
SUMS=$(curl -fsSL $CA_OPT --connect-timeout 15 --max-time 30 "$PKG_URL/SHA256SUMS" 2>/dev/null || true)
verified() {
  local want
  want=$(printf '%s\n' "$SUMS" | awk -v n="$1" '$2 == n {print $1; exit}')
  [ -z "$want" ] || [ "$(sha256sum "$2" | cut -d' ' -f1)" = "$want" ]
}

# fetch <name> <dest>: packages come from the stack's mirror so thousands of nodes never hit GitHub at once.
# The loop retries dropped and stalled transfers: curl --retry skips connection resets, and the option that
# covers them is missing from the curl of older distributions.
fetch() {
  local i rc
  for i in 1 2 3 4 5; do
    rc=0
    # shellcheck disable=SC2086
    curl -fsSL $CA_OPT --retry 2 --retry-delay 3 --connect-timeout 15 --speed-limit 10240 --speed-time 30 -o "$2" "$PKG_URL/$1" || rc=$?
    if [ "$rc" = 0 ]; then
      verified "$1" "$2" && return 0
      echo "checksum mismatch: $1"
    fi
    rm -f "$2"
    # 22: the mirror answered with an HTTP error (the file is not there); another try cannot help
    [ "$rc" = 22 ] && break
    [ "$i" = 5 ] || sleep $((i * 5))
  done
  echo "download failed: $1 (curl exit $rc)"
  return 1
}
install_pkg() {
  if [ "$PKG" = deb ]; then
    DEBIAN_FRONTEND=noninteractive dpkg -i "$1" >/dev/null 2>&1 || { DEBIAN_FRONTEND=noninteractive apt-get install -y -qq -f >/dev/null; }
  else
    rpm -Uvh --replacepkgs "$1" >/dev/null
  fi
}

# GPU metrics through an nvidia-smi based exporter; installed whenever the node is (or will be) a GPU node so
# it starts reporting as soon as the driver is present, even if the driver is installed later over SSH.
GPU_ON=0
if [ "$GPU" = on ]; then GPU_ON=1
elif [ "$GPU" = auto ] && { command -v nvidia-smi >/dev/null 2>&1 || [ "$(echo "${TB_ACCELERATOR_TYPE:-}" | tr '[:upper:]' '[:lower:]')" = gpu ]; }; then GPU_ON=1; fi
if [ "$GPU_ON" = 1 ] && ! command -v nvidia_gpu_exporter >/dev/null 2>&1; then
  if fetch "nvidia-gpu-exporter_${ARCH}.${PKG}" /tmp/nvidia-gpu-exporter.pkg; then
    install_pkg /tmp/nvidia-gpu-exporter.pkg
    systemctl enable --now nvidia_gpu_exporter >/dev/null 2>&1 || true
  else
    echo "GPU exporter package not available on the stack; GPU metrics disabled"; GPU_ON=0
  fi
fi

# ---- local vLLM servers serve Prometheus metrics on their API port; tb-otel-discover repeats this lookup every
# minute and re-applies this installer when the set changes, so a server started after the agent is scraped too.
STATE="$TB_DIR/otel-discovered"
SELF="$TB_DIR/install-otel-agent.sh"
discover_targets() {
  local pid ports port alive="" serving="" kept="" t
  for pid in $(pgrep -f 'vllm serve|vllm\.entrypoints\.openai\.api_server' 2>/dev/null); do
    # only the port is taken from the arguments, which can hold tokens; a wrapper shell prints nothing
    ports=$({ tr '\0' '\n' < "/proc/$pid/cmdline"; } 2>/dev/null | awk '
      { name = $0; sub(/.*\//, "", name) }
      prev == "vllm" && $0 == "serve" { server = 1 }
      $0 == "vllm.entrypoints.openai.api_server" { server = 1 }
      want { port = $0; want = 0 }
      $0 == "--port" { want = 1 }
      /^--port=/ { port = substr($0, 8) }
      { prev = name }
      END { if (server) print (port == "" ? 8000 : port) }')
    [ -n "$ports" ] || continue
    # a port set in a config file shows up only as a listening socket
    ports="$ports $(ss -ltnpH 2>/dev/null | awk -v p="pid=$pid," 'index($0, p) { n = split($4, a, ":"); print a[n] }')"
    for port in $(echo $ports | tr ' ' '\n' | sort -u); do
      case "$port" in *[!0-9]*) continue ;; esac
      alive="$alive 127.0.0.1:$port"
      if curl -s -m 3 "http://127.0.0.1:$port/metrics" 2>/dev/null | grep -q '^vllm:'; then serving="$serving 127.0.0.1:$port"; fi
    done
  done
  # an endpoint found earlier stays while its process lives, so loading a model or a restart does not drop it
  for t in $(cat "$STATE" 2>/dev/null | tr ',' ' '); do
    case " $alive " in *" $t "*) kept="$kept $t" ;; esac
  done
  echo $kept $serving | tr ' ' '\n' | sort -u | paste -sd, -
}
if [ "$DISCOVER" = off ]; then DISCOVERED=""
elif [ -n "${OTEL_DISCOVERED_TARGETS+x}" ]; then DISCOVERED="$OTEL_DISCOVERED_TARGETS"   # handed over by tb-otel-discover
else DISCOVERED=$(discover_targets); fi
TARGETS=$(echo "${OTEL_SCRAPE_TARGETS:-},$DISCOVERED" | tr -d ' ' | tr ',' '\n' | sed 's/^localhost:/127.0.0.1:/' | awk 'length($0) && !seen[$0]++' | paste -sd, -)

# ---- collector: the trimmed build when the stack has one and no Prometheus endpoint is scraped, else contrib
SVC=otelcol-contrib
if [ "$DIST" != contrib ] && [ "$GPU_ON" = 0 ] && [ -z "$TARGETS" ]; then
  if [ -x /usr/local/bin/otelcol-tb ] || { fetch "otelcol-tb_${ARCH}.tar.gz" /tmp/otelcol-tb.tgz >/dev/null 2>&1 && tar -C /usr/local/bin -xzf /tmp/otelcol-tb.tgz otelcol-tb; }; then
    SVC=otelcol-tb
  elif [ "$DIST" = trimmed ]; then
    echo "the stack has no trimmed collector for $ARCH"; exit 1
  fi
fi
rm -f /tmp/otelcol-tb.tgz
if [ "$SVC" = otelcol-contrib ]; then
  OTHER=otelcol-tb
  if ! command -v otelcol-contrib >/dev/null 2>&1; then
    fetch "otelcol-contrib_${ARCH}.${PKG}" /tmp/otelcol-contrib.pkg
    install_pkg /tmp/otelcol-contrib.pkg
    rm -f /tmp/otelcol-contrib.pkg
  fi
else
  OTHER=otelcol-contrib
  id otelcol-tb >/dev/null 2>&1 || useradd --system --no-create-home --shell /usr/sbin/nologin otelcol-tb
  cat > /etc/systemd/system/otelcol-tb.service <<UNIT
[Unit]
Description=OpenTelemetry Collector (CB-Tumblebug node agent)
After=network-online.target
Wants=network-online.target

[Service]
User=otelcol-tb
ExecStart=/usr/local/bin/otelcol-tb --config=/etc/otelcol-tb/config.yaml
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
UNIT
fi
CONF_DIR=/etc/$SVC
mkdir -p "$CONF_DIR"

# ---- collector configuration (values are expanded here; nothing is left for the collector to substitute)
q() { printf '%s' "$1" | sed "s/'/''/g"; }   # YAML single-quote escaping
CONF="$CONF_DIR/config.yaml"
# the trimmed build has no resourcedetection processor; host.name and os.type are set as plain attributes instead
if [ "$SVC" = otelcol-contrib ]; then PROCS="memory_limiter, resourcedetection, resource, batch"; else PROCS="memory_limiter, resource, batch"; fi
{
echo "receivers:"
if [ "$METRICS" = on ]; then
cat <<YAML
  hostmetrics:
    collection_interval: ${INTERVAL}
    scrapers:
      cpu:
        metrics:
          system.cpu.utilization: {enabled: true}
      memory:
        metrics:
          system.memory.utilization: {enabled: true}
      load: {}
      disk: {}
      filesystem:
        metrics:
          system.filesystem.utilization: {enabled: true}
        exclude_fs_types:
          fs_types: [autofs, binfmt_misc, bpf, cgroup2, configfs, debugfs, devpts, devtmpfs, fusectl, hugetlbfs, mqueue, nsfs, overlay, proc, procfs, pstore, rpc_pipefs, securityfs, selinuxfs, squashfs, sysfs, tracefs, tmpfs]
          match_type: strict
      network: {}
      paging: {}
      processes: {}
YAML
fi
SCRAPES=""
[ "$GPU_ON" = 1 ] && SCRAPES="127.0.0.1:9835"
if [ -n "$TARGETS" ]; then SCRAPES="${SCRAPES:+$SCRAPES,}${TARGETS}"; fi
if [ -n "$SCRAPES" ]; then
  echo "  prometheus:"
  echo "    config:"
  echo "      scrape_configs:"
  i=0
  for t in $(echo "$SCRAPES" | tr ',' ' '); do
    i=$((i+1)); job=scrape-$i; [ "$t" = "127.0.0.1:9835" ] && job=gpu
    printf "        - job_name: %s\n          scrape_interval: %s\n          static_configs: [{targets: ['%s']}]\n" "$job" "$INTERVAL" "$t"
  done
fi
if [ "$LOGS" = on ]; then
  if [ -d /var/log/journal ] || [ -d /run/log/journal ]; then
    echo "  journald:"
    echo "    priority: info"
  fi
  # the init script log is always shipped from the beginning, so Grafana shows the whole first boot
  echo "  filelog/tbinit:"
  echo "    include: [/var/log/tb-init-script.log]"
  echo "    start_at: beginning"
  if [ -n "${OTEL_LOG_PATHS:-}" ]; then
    echo "  filelog:"
    echo "    start_at: end"
    echo "    include:"
    for pth in $(echo "$OTEL_LOG_PATHS" | tr ',' ' '); do echo "      - '$(q "$pth")'"; done
  fi
fi
cat <<YAML
processors:
  memory_limiter:
    check_interval: 5s
    limit_mib: 200
    spike_limit_mib: 50
YAML
if [ "$SVC" = otelcol-contrib ]; then
  printf "  resourcedetection:\n    detectors: [system]\n    system:\n      hostname_sources: [os]\n"
fi
echo "  resource:"
echo "    attributes:"
if [ "$SVC" != otelcol-contrib ]; then
  echo "      - {key: host.name, value: '$(q "$(hostname)")', action: upsert}"
  echo "      - {key: os.type, value: 'linux', action: upsert}"
fi
cat <<YAML
      - {key: tb.ns, value: '$(q "${TB_NS_ID:-}")', action: upsert}
      - {key: tb.infra, value: '$(q "${TB_INFRA_ID:-}")', action: upsert}
      - {key: tb.nodegroup, value: '$(q "${TB_NODEGROUP_ID:-}")', action: upsert}
      - {key: tb.node, value: '$(q "${TB_NODE_ID:-}")', action: upsert}
      - {key: cloud.provider, value: '$(q "${TB_CSP:-}")', action: upsert}
      - {key: cloud.region, value: '$(q "${TB_REGION:-}")', action: upsert}
      - {key: service.namespace, value: '$(q "${TB_INFRA_ID:-}")', action: upsert}
      - {key: service.name, value: '$(q "${TB_NODE_ID:-}")', action: upsert}
  batch:
    timeout: 10s
exporters:
  otlphttp:
    endpoint: '$(q "$ENDPOINT")'
    headers:
      Authorization: 'Bearer $(q "$TOKEN")'
    compression: gzip
    retry_on_failure:
      enabled: true
      max_elapsed_time: 10m
    sending_queue:
      enabled: true
      queue_size: 2000
YAML
[ -n "$CA_FILE" ] && printf "    tls:\n      ca_file: %s\n" "$CA_FILE"
echo "service:"
echo "  telemetry:"
echo "    metrics:"
echo "      level: none"
echo "  pipelines:"
MR=""; [ "$METRICS" = on ] && MR="hostmetrics"; [ -n "$SCRAPES" ] && MR="${MR:+$MR, }prometheus"
if [ -n "$MR" ]; then
  echo "    metrics:"
  echo "      receivers: [$MR]"
  echo "      processors: [$PROCS]"
  echo "      exporters: [otlphttp]"
fi
LR=""
if [ "$LOGS" = on ]; then
  { [ -d /var/log/journal ] || [ -d /run/log/journal ]; } && LR="journald"
  LR="${LR:+$LR, }filelog/tbinit"
  [ -n "${OTEL_LOG_PATHS:-}" ] && LR="${LR:+$LR, }filelog"
fi
if [ -n "$LR" ]; then
  echo "    logs:"
  echo "      receivers: [$LR]"
  echo "      processors: [$PROCS]"
  echo "      exporters: [otlphttp]"
fi
} > "$CONF"

# the collector runs as its own user; journald and /var/log need these groups
for g in systemd-journal adm; do usermod -aG "$g" "$SVC" 2>/dev/null || true; done
# the config holds the write token: readable by the collector's group only
if chgrp "$SVC" "$CONF" 2>/dev/null; then chmod 640 "$CONF"; fi
systemctl daemon-reload
# a re-run can pick the other build; only one may report
systemctl disable --now "$OTHER" >/dev/null 2>&1 || true
systemctl enable "$SVC" >/dev/null 2>&1 || true
systemctl restart "$SVC"
sleep 3
systemctl is-active --quiet "$SVC" || { journalctl -u "$SVC" -n 30 --no-pager; echo "$SVC failed to start"; exit 1; }

# ---- discovery: tb-otel-discover re-applies this installer with the settings of this run
if [ "$DISCOVER" != off ]; then
  if [ -f "$0" ] && grep -q OTEL_DISCOVERED_TARGETS "$0" 2>/dev/null; then
    [ "$(readlink -f "$0")" = "$SELF" ] || cp "$0" "$SELF"
  elif ! { fetch install-agent.sh "$SELF.new" >/dev/null 2>&1 && grep -q OTEL_DISCOVERED_TARGETS "$SELF.new" && mv "$SELF.new" "$SELF"; }; then
    rm -f "$SELF.new"; DISCOVER=off
    echo "discovery is off: this installer was not run from a file and the stack serves an older one"
  fi
fi
( umask 077
  for v in TB_OTEL_ENDPOINT TB_OTEL_TOKEN TB_OTEL_CA_B64 TB_NS_ID TB_INFRA_ID TB_NODEGROUP_ID TB_NODE_ID TB_CSP TB_REGION \
           TB_ACCELERATOR_TYPE OTEL_METRICS OTEL_LOGS OTEL_GPU OTEL_INTERVAL OTEL_LOG_PATHS OTEL_SCRAPE_TARGETS \
           OTEL_DIST OTEL_PACKAGE_URL OTEL_DISCOVER; do
    if [ -n "${!v+x}" ]; then printf 'export %s=%q\n' "$v" "${!v}"; fi
  done > "$TB_DIR/otel-agent.env" )
printf '%s\n' "$DISCOVERED" > "$STATE"
{
  echo '#!/bin/bash'
  echo '# Written by install-otel-agent.sh: re-applies the node agent when a local vLLM metrics endpoint comes or goes'
  echo 'set -u'
  echo "STATE=$STATE"
  echo ". $TB_DIR/otel-agent.env"
  declare -f discover_targets
  echo 'while :; do'
  echo '  now=$(discover_targets)'
  echo '  if [ "$now" != "$(cat "$STATE" 2>/dev/null)" ]; then'
  echo '    echo "vLLM metrics endpoints: ${now:-none}"'
  # a failed run leaves the state as it was and is tried again after a pause
  echo "    OTEL_DISCOVERED_TARGETS=\"\$now\" bash $SELF || sleep 240"
  echo '  fi'
  echo '  sleep 60'
  echo 'done'
} > /usr/local/bin/tb-otel-discover.new
chmod 755 /usr/local/bin/tb-otel-discover.new
mv /usr/local/bin/tb-otel-discover.new /usr/local/bin/tb-otel-discover
cat > /etc/systemd/system/tb-otel-discover.service <<UNIT
[Unit]
Description=CB-Tumblebug node agent: find local vLLM metrics endpoints
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/tb-otel-discover
Restart=always
RestartSec=30

[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
if [ "$DISCOVER" = off ]; then
  systemctl disable --now tb-otel-discover >/dev/null 2>&1 || true
elif [ -n "${OTEL_DISCOVERED_TARGETS+x}" ]; then
  systemctl enable tb-otel-discover >/dev/null 2>&1 || true   # this run is its child: a restart would end it
else
  systemctl enable tb-otel-discover >/dev/null 2>&1 || true
  systemctl restart tb-otel-discover
fi
echo "otel agent ready: collector=$SVC metrics=$METRICS logs=$LOGS gpu=$GPU_ON scrape=${TARGETS:-none} discover=$DISCOVER interval=$INTERVAL endpoint=$ENDPOINT"
