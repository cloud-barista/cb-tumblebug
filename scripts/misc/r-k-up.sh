#!/usr/bin/env bash
#
# Remote Kubernetes bootstrapper for CB-Tumblebug (`make k-up HOST=<infraId|nodeId|ip|user@ip>`).
#
# Uses your local CB-Tumblebug environment as a bootstrapper to turn a clean Ubuntu VM into a
# ready-to-use Kubernetes-based CB-Tumblebug deployment with zero manual SSH setup:
#   1. Auto-discovers the target VM's Public IP, SSH user, and SSH private key from local CB-Tumblebug
#      (or accepts an explicit IP / user@ip + SSH_KEY).
#   2. Installs missing prerequisites on the remote VM (Docker, kubectl, helm, kind, uv, inotify sysctls).
#   3. Clones cb-tumblebug, prepares .env, and syncs bootstrapper Makefile/scripts.
#   4. Creates the kind cluster, installs Envoy Gateway + add-ons, deploys CB-Tumblebug via Helm,
#      and starts resilient background port-forwards (:8080 / :8443) ready for `make k-init HOST=...`
#      and `make k-tunnel HOST=...`.
#
# Optional toggles (defaults match interactive `make k-up` defaults, minus heavy Observability):
#   WITH_GATEWAY=1   (1=install Envoy Gateway single entrypoint, 0=direct services)
#   WITH_TLS=1       (1=enable self-signed HTTPS :8443 on Gateway)
#   WITH_MCP=1       (1=enable TB-MCP server)
#   WITH_AGW=1       (1=enable agentgateway in front of MCP)
#   WITH_MCP_AUTH=1  (1=enable local JWT auth on MCP route)
#   WITH_OBS=0       (1=also install Prometheus + Grafana stack in monitoring namespace)
#   KIND_WORKERS=0   (0=single-node kind, N=1 control-plane + N workers)

set -euo pipefail

TARGET_RAW="${REMOTE_HOST:-${HOST:-${IP:-}}}"
NS_FILTER="${NS:-}"
USER_IN="${SSH_USER:-}"
KEY_IN="${SSH_KEY:-}"
PORT_IN="${SSH_PORT:-22}"
HOLDER_IN="${HOLDER:-}"

K8S_NAMESPACE="${K8S_NAMESPACE:-cb-tumblebug}"
HELM_RELEASE="${HELM_RELEASE:-cb-tumblebug}"
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-cb-tumblebug}"
KIND_VERSION="${KIND_VERSION:-v0.32.0}"
KIND_WORKERS="${KIND_WORKERS:-0}"
KIND_TAINT_CP="${KIND_TAINT_CP:-false}"

WITH_GATEWAY="${WITH_GATEWAY:-1}"
WITH_TLS="${WITH_TLS:-1}"
WITH_MCP="${WITH_MCP:-1}"
WITH_AGW="${WITH_AGW:-1}"
WITH_MCP_AUTH="${WITH_MCP_AUTH:-1}"
WITH_OBS="${WITH_OBS:-0}"

CY=$'\033[36m'; GN=$'\033[32m'; YL=$'\033[33m'; RD=$'\033[31m'; DM=$'\033[2m'; BD=$'\033[1m'; RS=$'\033[0m'

if [ -z "$TARGET_RAW" ]; then
  if [ -t 0 ]; then
    read -rp "Remote target (Infra ID, Node ID, IP, or user@ip): " TARGET_RAW
  fi
  [ -n "$TARGET_RAW" ] || { printf '%b\n' "${RD}✖ Remote target is required: make k-up HOST=<infraId|ip>${RS}"; exit 1; }
fi

TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/.cb-r-kup.XXXXXX")"
chmod 700 "$TMP_DIR"
DISC_PF_PID=""

cleanup() {
  trap - EXIT INT TERM
  [ -n "$DISC_PF_PID" ] && kill "$DISC_PF_PID" 2>/dev/null || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

HOST_VAL="$TARGET_RAW"
USER_VAL="$USER_IN"
case "$TARGET_RAW" in
  *@*)
    [ -z "$USER_VAL" ] && USER_VAL="${TARGET_RAW%@*}"
    HOST_VAL="${TARGET_RAW#*@}"
    ;;
esac
[ -n "$KEY_IN" ] && KEY_IN="${KEY_IN/#\~/$HOME}"

# Load local API credentials from env or .env fallback
API_USER="${TB_API_USERNAME:-}"
API_PASS="${TB_API_PASSWORD:-}"
if [ -z "$API_USER" ] && [ -f .env ]; then
  API_USER="$(grep -E '^TB_API_USERNAME=' .env 2>/dev/null | head -1 | cut -d= -f2- | tr -d '"' | tr -d "'" || true)"
fi
if [ -z "$API_PASS" ] && [ -f .env ]; then
  API_PASS="$(grep -E '^TB_API_PASSWORD=' .env 2>/dev/null | head -1 | cut -d= -f2- | tr -d '"' | tr -d "'" || true)"
fi
API_USER="${API_USER:-default}"
API_PASS="${API_PASS:-default}"

# --- 1) Auto-discover target IP & SSH key from local CB-Tumblebug ------------
KEY_VAL="$KEY_IN"
if [ -z "$KEY_VAL" ]; then
  LOCAL_TB=""
  for cand in \
    "http://localhost:8080/tumblebug" \
    "https://localhost:8443/tumblebug" \
    "http://localhost:1323/tumblebug"; do
    if curl -sf -k -u "${API_USER}:${API_PASS}" --max-time 2 "${cand}/readyz" >/dev/null 2>&1; then
      LOCAL_TB="$cand"
      break
    fi
  done

  if [ -z "$LOCAL_TB" ] && command -v kubectl >/dev/null 2>&1 && kubectl get svc -n "$K8S_NAMESPACE" cb-tumblebug >/dev/null 2>&1; then
    disc_port=13423
    while ss -tlnH 2>/dev/null | awk '{print $4}' | grep -qE "[:.]${disc_port}$"; do disc_port=$((disc_port + 1)); done
    kubectl port-forward -n "$K8S_NAMESPACE" svc/cb-tumblebug "${disc_port}:1323" >/dev/null 2>&1 &
    DISC_PF_PID=$!
    sleep 1
    if curl -sf -u "${API_USER}:${API_PASS}" --max-time 2 "http://localhost:${disc_port}/tumblebug/readyz" >/dev/null 2>&1; then
      LOCAL_TB="http://localhost:${disc_port}/tumblebug"
    fi
  fi

  if [ -n "$LOCAL_TB" ]; then
    FOUND="$(
      LOCAL_TB="$LOCAL_TB" TARGET_Q="$HOST_VAL" NS_FILTER="$NS_FILTER" \
      API_USER="$API_USER" API_PASS="$API_PASS" HOLDER_IN="$HOLDER_IN" python3 - <<'PY'
import json, os, subprocess, sys

tb = os.environ["LOCAL_TB"]
q = os.environ["TARGET_Q"]
ns_f = os.environ.get("NS_FILTER", "")
user = os.environ["API_USER"]
pw = os.environ["API_PASS"]
holder = os.environ.get("HOLDER_IN", "")

def get_json(path):
    cmd = ["curl", "-sS", "-k", "-u", f"{user}:{pw}", "--max-time", "8"]
    if holder:
        cmd += ["-H", f"X-Credential-Holder: {holder}"]
    cmd.append(tb + path)
    try:
        return json.loads(subprocess.check_output(cmd, stderr=subprocess.DEVNULL).decode("utf-8"))
    except Exception:
        return {}

ns_list = [ns_f] if ns_f else [x.get("id") for x in (get_json("/ns").get("ns") or []) if x.get("id")]
if "default" in ns_list:
    ns_list.remove("default")
    ns_list.insert(0, "default")

def emit(ns, iid, v):
    ip = v.get("publicIP") or v.get("publicIp") or ""
    if not ip:
        return False
    skey = v.get("sshKeyId") or v.get("sshKey") or ""
    u = v.get("nodeUserName") or v.get("vmUserName") or v.get("userName") or "cb-user"
    prt = str(v.get("sshPort") or 22)
    print(f"{ip}\t{skey}\t{u}\t{prt}\t{ns}\t{iid}\t{v.get('id','')}")
    return True

for ns in ns_list:
    infras = get_json(f"/ns/{ns}/infra?option=id").get("output") or []
    # Fast exact Infra ID match first
    if q in infras:
        d = get_json(f"/ns/{ns}/infra/{q}")
        for v in (d.get("node") or d.get("vm") or []):
            if emit(ns, q, v):
                sys.exit(0)
    for iid in infras:
        if iid == q:
            continue
        d = get_json(f"/ns/{ns}/infra/{iid}")
        for v in (d.get("node") or d.get("vm") or []):
            ip = v.get("publicIP") or v.get("publicIp") or ""
            nid = v.get("id") or ""
            if q in (ip, nid):
                if emit(ns, iid, v):
                    sys.exit(0)
PY
    )"
    if [ -n "$FOUND" ]; then
      IFS=$'\t' read -r RES_IP RES_SKEY RES_USER RES_PORT RES_NS RES_INFRA RES_NODE <<< "$FOUND"
      HOST_VAL="$RES_IP"
      [ -z "$USER_VAL" ] && USER_VAL="$RES_USER"
      [ "$PORT_IN" = "22" ] && [ -n "$RES_PORT" ] && PORT_IN="$RES_PORT"
      if [ -n "$RES_SKEY" ]; then
        CURL_KEY=(-sS -k -u "${API_USER}:${API_PASS}" --max-time 8)
        [ -n "$HOLDER_IN" ] && CURL_KEY+=(-H "X-Credential-Holder: ${HOLDER_IN}")
        curl "${CURL_KEY[@]}" "${LOCAL_TB}/ns/${RES_NS}/resources/sshKey/${RES_SKEY}" \
          | python3 -c '
import json, sys
d = json.load(sys.stdin)
pk = d.get("privateKey") or ""
if not pk:
    for kv in (d.get("keyValueList") or []):
        if str(kv.get("key", "")).lower() in ("privatekey", "keymaterial"):
            pk = kv.get("value") or ""
            break
pk = pk.replace("\\n", "\n")
if pk and not pk.endswith("\n"):
    pk += "\n"
sys.stdout.write(pk)
' > "${TMP_DIR}/key.pem" 2>/dev/null || true
        if [ -s "${TMP_DIR}/key.pem" ]; then
          chmod 600 "${TMP_DIR}/key.pem"
          KEY_VAL="${TMP_DIR}/key.pem"
          printf '%b\n' "${GN}✔ Auto-discovered target from local CB-Tumblebug:${RS} ${BD}${USER_VAL}@${HOST_VAL}:${PORT_IN}${RS} ${DM}(${RES_NS}/${RES_INFRA}/${RES_NODE})${RS}"
        fi
      fi
    fi
  fi
  if [ -n "$DISC_PF_PID" ]; then kill "$DISC_PF_PID" 2>/dev/null || true; DISC_PF_PID=""; fi
fi

USER_VAL="${USER_VAL:-cb-user}"
SSH_OPTS=(
  -o StrictHostKeyChecking=no
  -o UserKnownHostsFile=/dev/null
  -o LogLevel=ERROR
  -o ConnectTimeout=15
  -o ServerAliveInterval=30
  -o ServerAliveCountMax=6
  -p "$PORT_IN"
)
[ -n "$KEY_VAL" ] && SSH_OPTS+=(-i "$KEY_VAL")

if ! ssh "${SSH_OPTS[@]}" -o BatchMode=yes "${USER_VAL}@${HOST_VAL}" true >/dev/null 2>&1; then
  if [ -z "$KEY_VAL" ] && [ -t 0 ]; then
    printf '%b\n' "${YL}⚠ Could not connect to ${USER_VAL}@${HOST_VAL} without an SSH key.${RS}"
    read -rp "  SSH private key path [$HOME/.ssh/id_rsa]: " k_in
    KEY_VAL="${k_in:-$HOME/.ssh/id_rsa}"; KEY_VAL="${KEY_VAL/#\~/$HOME}"
    SSH_OPTS+=(-i "$KEY_VAL")
  else
    printf '%b\n' "${RD}✖ Cannot reach ${USER_VAL}@${HOST_VAL}:${PORT_IN} over SSH.${RS}"
    exit 1
  fi
fi

printf '%b\n' "${BD}${CY}▌ Remote CB-Tumblebug Kubernetes Bootstrap${RS} ${DM}(target: ${USER_VAL}@${HOST_VAL})${RS}"

# --- 2) Install remote host prerequisites + clone/sync repo ------------------
printf '%b\n' "${CY}[1/3]${RS} Installing host prerequisites (Docker, kubectl, helm, kind, uv, sysctls) on ${HOST_VAL}..."
ssh "${SSH_OPTS[@]}" "${USER_VAL}@${HOST_VAL}" "cat > /tmp/cb-k-prereqs.sh" < ./scripts/misc/k-prereqs.sh
ssh "${SSH_OPTS[@]}" "${USER_VAL}@${HOST_VAL}" "
  set -e
  YES=1 INSTALL_DOCKER=1 KIND_VERSION='${KIND_VERSION}' bash /tmp/cb-k-prereqs.sh
  rm -f /tmp/cb-k-prereqs.sh
  export PATH=\"\$HOME/.local/bin:\$HOME/.cargo/bin:/usr/local/bin:\$PATH\"
  if ! command -v uv >/dev/null 2>&1; then
    curl -LsSf https://astral.sh/uv/install.sh | sh >/dev/null 2>&1 || curl -LsSf https://astral.sh/uv/install.sh | sh
    grep -q '\.local/bin' ~/.bashrc 2>/dev/null || echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ~/.bashrc
  fi
"

printf '%b\n' "${CY}[2/3]${RS} Preparing CB-Tumblebug repository & syncing bootstrapper scripts on ${HOST_VAL}..."
ssh "${SSH_OPTS[@]}" "${USER_VAL}@${HOST_VAL}" '
  set -e
  CB_DIR="$HOME/go/src/github.com/cloud-barista/cb-tumblebug"
  mkdir -p "$(dirname "$CB_DIR")" "$HOME/.cloud-barista"
  if [ ! -d "$CB_DIR/.git" ]; then
    git clone --depth 1 https://github.com/cloud-barista/cb-tumblebug.git "$CB_DIR"
  fi
  if [ ! -f "$CB_DIR/.env" ] && [ -f "$CB_DIR/.env.example" ]; then
    cp "$CB_DIR/.env.example" "$CB_DIR/.env"
    sed -i "s/^TB_API_USERNAME=$/TB_API_USERNAME=default/" "$CB_DIR/.env"
    sed -i "s/^TB_API_PASSWORD=$/TB_API_PASSWORD=default/" "$CB_DIR/.env"
    sed -i "s/^SPIDER_USERNAME=$/SPIDER_USERNAME=default/" "$CB_DIR/.env"
    sed -i "s/^SPIDER_PASSWORD=$/SPIDER_PASSWORD=default/" "$CB_DIR/.env"
    sed -i "s/^TERRARIUM_API_USERNAME=$/TERRARIUM_API_USERNAME=default/" "$CB_DIR/.env"
    sed -i "s/^TERRARIUM_API_PASSWORD=$/TERRARIUM_API_PASSWORD=default/" "$CB_DIR/.env"
  fi
'

# Overlay up-to-date local orchestration files (never copies local .env, credentials, or values-*.yaml state)
tar -cf - \
  Makefile \
  scripts/set-tb.sh \
  scripts/restore-assets.sh \
  scripts/backup-assets.sh \
  scripts/lib/pg-backend.sh \
  scripts/misc/k-prereqs.sh \
  scripts/misc/pf-watch.sh \
  scripts/misc/ssh-tunnel.sh \
  scripts/misc/ssh-node.sh \
  scripts/misc/r-k-up.sh 2>/dev/null \
  | ssh "${SSH_OPTS[@]}" "${USER_VAL}@${HOST_VAL}" 'tar -xf - -C "$HOME/go/src/github.com/cloud-barista/cb-tumblebug"'

# --- 3) Deploy kind + Envoy Gateway + CB-Tumblebug Helm stack + port-forward --
printf '%b\n' "${CY}[3/3]${RS} Deploying Kubernetes cluster & CB-Tumblebug stack on ${HOST_VAL}..."

ssh "${SSH_OPTS[@]}" "${USER_VAL}@${HOST_VAL}" \
  "K8S_NAMESPACE='${K8S_NAMESPACE}' HELM_RELEASE='${HELM_RELEASE}' KIND_CLUSTER_NAME='${KIND_CLUSTER_NAME}' \
   KIND_WORKERS='${KIND_WORKERS}' KIND_TAINT_CP='${KIND_TAINT_CP}' \
   WITH_GATEWAY='${WITH_GATEWAY}' WITH_TLS='${WITH_TLS}' WITH_MCP='${WITH_MCP}' \
   WITH_AGW='${WITH_AGW}' WITH_MCP_AUTH='${WITH_MCP_AUTH}' WITH_OBS='${WITH_OBS}' \
   bash -s" <<'REMOTE_EOF'
set -euo pipefail
export PATH="$HOME/.local/bin:$HOME/.cargo/bin:/usr/local/go/bin:/usr/local/bin:$PATH"
unset HOST IP REMOTE_HOST MAKEFLAGS || true
cd "$HOME/go/src/github.com/cloud-barista/cb-tumblebug"

cat > /tmp/cb-remote-kup-inner.sh <<'INNER_EOF'
set -euo pipefail
export PATH="$HOME/.local/bin:$HOME/.cargo/bin:/usr/local/go/bin:/usr/local/bin:$PATH"
unset HOST IP REMOTE_HOST MAKEFLAGS || true
cd "$HOME/go/src/github.com/cloud-barista/cb-tumblebug"

if ! kubectl cluster-info >/dev/null 2>&1; then
  cfg=""
  if [ "${KIND_WORKERS:-0}" -gt 0 ] 2>/dev/null; then
    cfg=$(mktemp)
    {
      echo "kind: Cluster"
      echo "apiVersion: kind.x-k8s.io/v1alpha4"
      echo "nodes:"
      echo "  - role: control-plane"
      w=0
      while [ "$w" -lt "$KIND_WORKERS" ]; do
        echo "  - role: worker"
        w=$((w + 1))
      done
    } > "$cfg"
  fi
  kind create cluster --name "$KIND_CLUSTER_NAME" ${cfg:+--config "$cfg"}
  [ -n "$cfg" ] && rm -f "$cfg"
  if [ "${KIND_TAINT_CP:-false}" = "true" ] && [ "${KIND_WORKERS:-0}" -gt 0 ] 2>/dev/null; then
    kubectl taint nodes "${KIND_CLUSTER_NAME}-control-plane" node-role.kubernetes.io/control-plane=:NoSchedule --overwrite >/dev/null 2>&1 || true
  fi
fi

# Configure default add-ons on a fresh Helm release so headless remote installs get the same
# single-entrypoint Gateway + MCP setup as interactive k-up
if ! helm status "$HELM_RELEASE" -n "$K8S_NAMESPACE" >/dev/null 2>&1; then
  if [ "${WITH_GATEWAY:-1}" = "1" ]; then
    make --no-print-directory k-gateway-install K8S_NAMESPACE="$K8S_NAMESPACE" HOST="" IP=""
    if [ "${WITH_TLS:-1}" = "1" ]; then
      printf "gateway:\n  enabled: true\n  tls:\n    enabled: true\n" > deployments/helm/cb-tumblebug/values-gateway.yaml
    else
      printf "gateway:\n  enabled: true\n" > deployments/helm/cb-tumblebug/values-gateway.yaml
    fi
  fi
  if [ "${WITH_MCP:-1}" = "1" ]; then
    printf "mcp:\n  enabled: true\n" > deployments/helm/cb-tumblebug/values-mcp.yaml
    if [ "${WITH_AGW:-1}" = "1" ]; then
      printf "agentgateway:\n  enabled: true\n" > deployments/helm/cb-tumblebug/values-agentgateway.yaml
      if [ "${WITH_MCP_AUTH:-1}" = "1" ]; then
        make --no-print-directory k-mcp-authkey K8S_NAMESPACE="$K8S_NAMESPACE" HOST="" IP=""
      fi
    fi
  fi
fi

NO_ADDON_WIZARD=1 PF_DEFER=1 make --no-print-directory k-up \
  K8S_NAMESPACE="$K8S_NAMESPACE" HELM_RELEASE="$HELM_RELEASE" KIND_CLUSTER_NAME="$KIND_CLUSTER_NAME" HOST="" IP=""

if [ "${WITH_GATEWAY:-1}" = "1" ] && [ "${WITH_TLS:-1}" = "1" ]; then
  sleep 2
  kubectl annotate gateway cb-tumblebug-gateway -n "$K8S_NAMESPACE" \
    cloud-barista/tls-nudge="$(date +%s)" --overwrite >/dev/null 2>&1 || true
fi

if [ "${WITH_OBS:-0}" = "1" ]; then
  make --no-print-directory k-observability-on K8S_NAMESPACE="$K8S_NAMESPACE" HOST="" IP="" || true
fi

make --no-print-directory k-port-forward K8S_NAMESPACE="$K8S_NAMESPACE" HOST="" IP="" </dev/null
INNER_EOF

trap 'rm -f /tmp/cb-remote-kup-inner.sh' EXIT
if docker info >/dev/null 2>&1; then
  bash /tmp/cb-remote-kup-inner.sh
else
  sg docker -c "bash /tmp/cb-remote-kup-inner.sh"
fi
REMOTE_EOF

printf '\n%b\n' "${GN}✔ Remote CB-Tumblebug Kubernetes deployment is Ready on ${USER_VAL}@${HOST_VAL}!${RS}"
printf '%b\n' "${BD}Next steps from THIS bootstrapper machine:${RS}"
printf '%b\n' "  1) Initialize credentials, DB backup & templates:  ${CY}make k-init HOST=${TARGET_RAW} ARGS=\"-y\"${RS}"
printf '%b\n' "  2) Open local browser tunnel (MapUI / API / MCP):  ${CY}make k-tunnel HOST=${TARGET_RAW}${RS}"
