#!/usr/bin/env bash
#
# Interactive / CLI SSH helper for connecting directly to any VM Node managed by CB-Tumblebug.
#
# Automatically discovers the active CB-Tumblebug API endpoint (Gateway :8080/:8443, direct :1323,
# active k-tunnel, or temporary kubectl port-forward), lets you pick Namespace -> Infra -> Node
# interactively (or via flags), fetches the node's private key into a 0600 temporary file, sets up
# automatic Bastion ProxyCommand when the node only has a Private IP, and cleans up all temporary
# keys immediately when the SSH session exits.
#
# Examples:
#   make ssh                                      # Interactive Namespace -> Infra -> Node picker
#   make ssh INFRA=my-infra NODE=g1-1             # Direct jump to a specific node
#   make ssh NODE=ntfy-kr-1                       # Auto-locate node across namespaces/infras
#   make ssh IP=43.202.1.2 USER=root              # Match by IP and override login user
#   make ssh INFRA=my-infra NODE=g1-1 L=8080:80   # Open local port-forward (-L 8080:127.0.0.1:80) + shell
#   make ssh INFRA=my-infra NODE=g1-1 CMD="uptime"# Run a one-off command and exit

set -euo pipefail

K8S_NAMESPACE="${K8S_NAMESPACE:-cb-tumblebug}"
K8S_CONTEXT="${K8S_CONTEXT:-}"
KUBECTL=(kubectl)
[ -n "$K8S_CONTEXT" ] && KUBECTL+=(--context "$K8S_CONTEXT")

NS_ARG="${NS:-}"
INFRA_ARG="${INFRA:-}"
NODE_ARG="${NODE:-}"
IP_ARG="${IP:-${HOST:-}}"
USER_ARG="${USER_OVERRIDE:-${SSH_USER:-}}"
PORT_FWD_ARG="${L:-${FORWARD:-}}"
CMD_ARG="${CMD:-}"
HOLDER_ARG="${HOLDER:-}"
DRY_RUN="${DRY_RUN:-0}"

CY=$'\033[36m'; GN=$'\033[32m'; YL=$'\033[33m'; RD=$'\033[31m'; DM=$'\033[2m'; BD=$'\033[1m'; RS=$'\033[0m'

TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/.cb-ssh-node.XXXXXX")"
chmod 700 "$TMP_DIR"
PF_PID=""

cleanup() {
  trap - EXIT INT TERM
  [ -n "$PF_PID" ] && kill "$PF_PID" 2>/dev/null || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

ask() {
  local prompt="$1" def="${2:-}" ans=""
  if [ -n "$def" ]; then
    read -rp "$prompt ${DM}[$def]${RS}: " ans </dev/tty || ans=""
    printf '%s' "${ans:-$def}"
  else
    read -rp "$prompt: " ans </dev/tty || ans=""
    printf '%s' "$ans"
  fi
}

# Load API credentials from env or .env fallback
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

CURL_COMMON=(-sS -k -u "${API_USER}:${API_PASS}" --max-time 8)
[ -n "$HOLDER_ARG" ] && CURL_COMMON+=(-H "X-Credential-Holder: ${HOLDER_ARG}")

tb_get() {
  curl "${CURL_COMMON[@]}" "${TB_BASE}$1" 2>/dev/null
}

probe_tb() {
  local base="$1"
  curl -sf -k -u "${API_USER}:${API_PASS}" --max-time 2 "${base}/readyz" >/dev/null 2>&1
}

# --- 1) Discover active CB-Tumblebug API endpoint ----------------------------
TB_BASE="${TB_ENDPOINT:-}"
TB_BASE="${TB_BASE%/}"
if [ -n "$TB_BASE" ] && [[ "$TB_BASE" != */tumblebug ]]; then
  TB_BASE="${TB_BASE}/tumblebug"
fi

if [ -z "$TB_BASE" ]; then
  for cand in \
    "http://localhost:8080/tumblebug" \
    "https://localhost:8443/tumblebug" \
    "http://localhost:1323/tumblebug"; do
    if probe_tb "$cand"; then
      TB_BASE="$cand"
      break
    fi
  done
fi

# Check active k-tunnel links if still not found
if [ -z "$TB_BASE" ] && [ -f "${TMPDIR:-/tmp}/cb-ssh-tunnel/links.txt" ]; then
  TUN_API="$(grep -Eo 'https?://localhost:[0-9]+/tumblebug' "${TMPDIR:-/tmp}/cb-ssh-tunnel/links.txt" 2>/dev/null | head -1 || true)"
  if [ -n "$TUN_API" ] && probe_tb "$TUN_API"; then
    TB_BASE="$TUN_API"
  fi
fi

# Fallback: open temporary kubectl port-forward to svc/cb-tumblebug
if [ -z "$TB_BASE" ] && command -v kubectl >/dev/null 2>&1; then
  if "${KUBECTL[@]}" get svc -n "$K8S_NAMESPACE" cb-tumblebug >/dev/null 2>&1; then
    disc_port=13323
    while ss -tlnH 2>/dev/null | awk '{print $4}' | grep -qE "[:.]${disc_port}$"; do
      disc_port=$((disc_port + 1))
    done
    "${KUBECTL[@]}" port-forward -n "$K8S_NAMESPACE" svc/cb-tumblebug "${disc_port}:1323" >/dev/null 2>&1 &
    PF_PID=$!
    for _ in 1 2 3 4 5; do
      if probe_tb "http://localhost:${disc_port}/tumblebug"; then
        TB_BASE="http://localhost:${disc_port}/tumblebug"
        break
      fi
      sleep 0.4
    done
  fi
fi

if [ -z "$TB_BASE" ]; then
  printf '%b\n' "${RD}✖ Cannot reach CB-Tumblebug API.${RS} Start the stack (${CY}make up${RS} / ${CY}make k-up${RS} / ${CY}make k-port-forward${RS}) or pass ${CY}TB_ENDPOINT=http://host:port${RS}."
  exit 1
fi

printf '%b\n' "${BD}${CY}▌ CB-Tumblebug Node SSH${RS} ${DM}(API: ${TB_BASE})${RS}"

# --- 2) Resolve Namespace / Infra / Node -------------------------------------
# Fast path: when NODE or IP is specified without NS or INFRA, scan namespaces to locate it automatically.
if { [ -n "$NODE_ARG" ] || [ -n "$IP_ARG" ]; } && { [ -z "$NS_ARG" ] || [ -z "$INFRA_ARG" ]; }; then
  MATCH_LINE="$(
    TB_BASE="$TB_BASE" NS_ARG="$NS_ARG" INFRA_ARG="$INFRA_ARG" NODE_ARG="$NODE_ARG" IP_ARG="$IP_ARG" \
    API_USER="$API_USER" API_PASS="$API_PASS" HOLDER_ARG="$HOLDER_ARG" python3 - <<'PY'
import json, os, subprocess, sys

base = os.environ["TB_BASE"]
user = os.environ["API_USER"]
pw = os.environ["API_PASS"]
holder = os.environ.get("HOLDER_ARG", "")
ns_filter = os.environ.get("NS_ARG", "")
infra_filter = os.environ.get("INFRA_ARG", "")
node_q = os.environ.get("NODE_ARG", "")
ip_q = os.environ.get("IP_ARG", "")

def get_json(path):
    cmd = ["curl", "-sS", "-k", "-u", f"{user}:{pw}", "--max-time", "8"]
    if holder:
        cmd += ["-H", f"X-Credential-Holder: {holder}"]
    cmd.append(base + path)
    try:
        out = subprocess.check_output(cmd, stderr=subprocess.DEVNULL)
        return json.loads(out.decode("utf-8"))
    except Exception:
        return {}

ns_list = [ns_filter] if ns_filter else [x.get("id") for x in (get_json("/ns").get("ns") or []) if x.get("id")]
for ns in ns_list:
    infras = [infra_filter] if infra_filter else (get_json(f"/ns/{ns}/infra?option=id").get("output") or [])
    for inf in infras:
        d = get_json(f"/ns/{ns}/infra/{inf}")
        for nd in (d.get("node") or d.get("vm") or []):
            nid = nd.get("id") or ""
            pub = nd.get("publicIP") or nd.get("publicIp") or ""
            priv = nd.get("privateIP") or nd.get("privateIp") or ""
            if (node_q and nid == node_q) or (ip_q and ip_q in (pub, priv)):
                print(f"{ns}\t{inf}\t{nid}")
                sys.exit(0)
PY
  )"
  if [ -n "$MATCH_LINE" ]; then
    IFS=$'\t' read -r NS_ARG INFRA_ARG NODE_ARG <<< "$MATCH_LINE"
    printf '%b\n' "  ${GN}✔ Matched node:${RS} ${BD}${NS_ARG} / ${INFRA_ARG} / ${NODE_ARG}${RS}"
  fi
fi

# Select Namespace
if [ -z "$NS_ARG" ]; then
  mapfile -t NS_ITEMS < <(
    tb_get "/ns" | python3 -c '
import json, sys
try:
    ns = [x.get("id") for x in (json.load(sys.stdin).get("ns") or []) if x.get("id")]
    if "default" in ns:
        ns.remove("default")
        ns.insert(0, "default")
    print("\n".join(ns))
except Exception:
    pass
'
  )
  if [ "${#NS_ITEMS[@]}" -eq 0 ]; then
    printf '%b\n' "${RD}✖ No namespaces found.${RS}"
    exit 1
  elif [ "${#NS_ITEMS[@]}" -eq 1 ]; then
    NS_ARG="${NS_ITEMS[0]}"
  else
    # Filter to namespaces that actually contain at least 1 Infra for cleaner UX
    NON_EMPTY_NS=()
    for n in "${NS_ITEMS[@]}"; do
      cnt="$(tb_get "/ns/${n}/infra?option=id" | python3 -c 'import json,sys;print(len(json.load(sys.stdin).get("output") or []))' 2>/dev/null || echo 0)"
      if [ "${cnt:-0}" -gt 0 ]; then
        NON_EMPTY_NS+=("${n}:${cnt}")
      fi
    done
    if [ "${#NON_EMPTY_NS[@]}" -eq 0 ]; then
      printf '%b\n' "${YL}⚠ No Infras found in any namespace (${NS_ITEMS[*]}).${RS}"
      exit 1
    elif [ "${#NON_EMPTY_NS[@]}" -eq 1 ]; then
      NS_ARG="${NON_EMPTY_NS[0]%%:*}"
      printf '%b\n' "  ${DM}namespace:${RS} ${BD}${NS_ARG}${RS}"
    else
      printf '%b\n' "  ${BD}Namespaces with active Infras:${RS}"
      idx=1
      for entry in "${NON_EMPTY_NS[@]}"; do
        printf "    ${CY}%d)${RS} %-18s ${DM}(%s infra(s))${RS}\n" "$idx" "${entry%%:*}" "${entry##*:}"
        idx=$((idx + 1))
      done
      sel="$(ask "  Select namespace" "1")"
      if [[ "$sel" =~ ^[0-9]+$ ]] && [ "$sel" -ge 1 ] && [ "$sel" -le "${#NON_EMPTY_NS[@]}" ]; then
        NS_ARG="${NON_EMPTY_NS[$((sel - 1))]%%:*}"
      else
        NS_ARG="$sel"
      fi
    fi
  fi
fi

# Select Infra
if [ -z "$INFRA_ARG" ]; then
  mapfile -t INFRA_IDS < <(
    tb_get "/ns/${NS_ARG}/infra?option=id" | python3 -c 'import json,sys;print("\n".join(json.load(sys.stdin).get("output") or []))'
  )
  if [ "${#INFRA_IDS[@]}" -eq 0 ]; then
    printf '%b\n' "${RD}✖ No Infras found in namespace '${NS_ARG}'.${RS}"
    exit 1
  elif [ "${#INFRA_IDS[@]}" -eq 1 ]; then
    INFRA_ARG="${INFRA_IDS[0]}"
    printf '%b\n' "  ${DM}infra:${RS}     ${BD}${INFRA_ARG}${RS}"
  else
    printf '%b\n' "  ${BD}Infras in namespace '${NS_ARG}':${RS}"
    idx=1
    for inf in "${INFRA_IDS[@]}"; do
      printf "    ${CY}%d)${RS} %s\n" "$idx" "$inf"
      idx=$((idx + 1))
    done
    sel="$(ask "  Select infra" "1")"
    if [[ "$sel" =~ ^[0-9]+$ ]] && [ "$sel" -ge 1 ] && [ "$sel" -le "${#INFRA_IDS[@]}" ]; then
      INFRA_ARG="${INFRA_IDS[$((sel - 1))]}"
    else
      INFRA_ARG="$sel"
    fi
  fi
fi

# Fetch Infra JSON & list Nodes
INFRA_JSON="$(tb_get "/ns/${NS_ARG}/infra/${INFRA_ARG}")"
mapfile -t NODE_ROWS < <(
  printf '%s' "$INFRA_JSON" | python3 -c '
import json, sys
try:
    d = json.load(sys.stdin)
except Exception:
    sys.exit(0)
for nd in (d.get("node") or d.get("vm") or []):
    nid = nd.get("id") or ""
    grp = nd.get("nodeGroupId") or nd.get("subGroupId") or "-"
    st  = nd.get("status") or "-"
    pub = nd.get("publicIP") or nd.get("publicIp") or "-"
    prv = nd.get("privateIP") or nd.get("privateIp") or "-"
    usr = nd.get("nodeUserName") or nd.get("vmUserName") or "cb-user"
    prt = str(nd.get("sshPort") or 22)
    key = nd.get("sshKeyId") or ""
    csp = (nd.get("connectionConfig") or {}).get("providerName") or nd.get("connectionName") or "-"
    reg_obj = nd.get("region") or {}
    reg = reg_obj.get("region") or reg_obj.get("Region") or "-"
    print("\t".join([nid, grp, st, pub, prv, usr, prt, key, f"{csp}/{reg}"]))
'
)

if [ "${#NODE_ROWS[@]}" -eq 0 ]; then
  printf '%b\n' "${RD}✖ No nodes found in '${NS_ARG}/${INFRA_ARG}'.${RS}"
  exit 1
fi

SELECTED_ROW=""
if [ -n "$NODE_ARG" ]; then
  for r in "${NODE_ROWS[@]}"; do
    nid="${r%%$'\t'*}"
    if [ "$nid" = "$NODE_ARG" ]; then
      SELECTED_ROW="$r"
      break
    fi
  done
  if [ -z "$SELECTED_ROW" ]; then
    printf '%b\n' "${RD}✖ Node '${NODE_ARG}' not found in '${NS_ARG}/${INFRA_ARG}'.${RS}"
    exit 1
  fi
elif [ "${#NODE_ROWS[@]}" -eq 1 ]; then
  SELECTED_ROW="${NODE_ROWS[0]}"
else
  printf '\n  %b\n' "${BD}Nodes in ${NS_ARG}/${INFRA_ARG}:${RS}"
  printf "  ${DM}%-4s %-20s %-14s %-18s %-16s %-15s %-10s${RS}\n" "#" "NODE ID" "STATUS" "CSP/REGION" "PUBLIC IP" "PRIVATE IP" "USER:PORT"
  idx=1
  for r in "${NODE_ROWS[@]}"; do
    IFS=$'\t' read -r nid grp st pub prv usr prt key cspreg <<< "$r"
    printf "  ${CY}%-4s${RS} %-20s %-14s %-18s %-16s %-15s %-10s\n" "${idx})" "$nid" "$st" "$cspreg" "$pub" "$prv" "${usr}:${prt}"
    idx=$((idx + 1))
  done
  echo ""
  sel="$(ask "  Select node (# or ID)" "1")"
  if [[ "$sel" =~ ^[0-9]+$ ]] && [ "$sel" -ge 1 ] && [ "$sel" -le "${#NODE_ROWS[@]}" ]; then
    SELECTED_ROW="${NODE_ROWS[$((sel - 1))]}"
  else
    for r in "${NODE_ROWS[@]}"; do
      [ "${r%%$'\t'*}" = "$sel" ] && SELECTED_ROW="$r" && break
    done
  fi
  if [ -z "$SELECTED_ROW" ]; then
    printf '%b\n' "${RD}✖ Invalid node selection: ${sel}${RS}"
    exit 1
  fi
fi

IFS=$'\t' read -r TARGET_NODE TARGET_GRP TARGET_STATUS TARGET_PUB TARGET_PRIV TARGET_NODE_USER TARGET_PORT TARGET_KEY_ID TARGET_CSPREG <<< "$SELECTED_ROW"

fetch_ssh_key() {
  local ns="$1" key_id="$2" out_file="$3"
  local meta
  meta="$(tb_get "/ns/${ns}/resources/sshKey/${key_id}")"
  printf '%s' "$meta" | python3 -c '
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
' > "$out_file"
  chmod 600 "$out_file"
  # Also extract key's verifiedUsername / username if present
  printf '%s' "$meta" | python3 -c '
import json, sys
d = json.load(sys.stdin)
print(d.get("verifiedUsername") or d.get("username") or "")
'
}

TARGET_KEY_FILE="${TMP_DIR}/target.pem"
KEY_USER="$(fetch_ssh_key "$NS_ARG" "$TARGET_KEY_ID" "$TARGET_KEY_FILE" || true)"
if [ ! -s "$TARGET_KEY_FILE" ]; then
  printf '%b\n' "${RD}✖ Could not retrieve SSH private key '${TARGET_KEY_ID}' for node '${TARGET_NODE}'.${RS}"
  exit 1
fi

LOGIN_USER="${USER_ARG:-${KEY_USER:-${TARGET_NODE_USER:-cb-user}}}"

# Interactive quick options when invoked with `make ssh` (no NODE/IP/CMD/USER/L passed on CLI)
if [ -t 0 ] && [ "$DRY_RUN" != "1" ] && [ -z "$CMD_ARG" ] && [ -z "$USER_ARG" ] && [ -z "$PORT_FWD_ARG" ] && [ -z "${NODE:-}" ] && [ -z "${IP:-${HOST:-}}" ]; then
  u_ans="$(ask "  Login user" "$LOGIN_USER")"
  [ -n "$u_ans" ] && LOGIN_USER="$u_ans"
  l_ans="$(ask "  Local port-forward (e.g. 8080:80, Enter=none)" "")"
  [ -n "$l_ans" ] && PORT_FWD_ARG="$l_ans"
  tip_ssh="make ssh NODE=${TARGET_NODE}"
  [ "$LOGIN_USER" != "${KEY_USER:-${TARGET_NODE_USER:-cb-user}}" ] && tip_ssh+=" USER=${LOGIN_USER}"
  [ -n "$PORT_FWD_ARG" ] && tip_ssh+=" L=${PORT_FWD_ARG}"
  printf '  %b\n' "${DM}💡 Direct command: ${RS}${CY}${tip_ssh}${RS}"
fi

SSH_BASE_OPTS=(
  -o StrictHostKeyChecking=no
  -o UserKnownHostsFile=/dev/null
  -o LogLevel=ERROR
  -o ConnectTimeout=15
  -o ServerAliveInterval=30
  -o ServerAliveCountMax=3
)

SSH_CMD=(ssh "${SSH_BASE_OPTS[@]}" -i "$TARGET_KEY_FILE" -p "$TARGET_PORT")

# --- 3) Handle Private Subnet via Bastion ProxyCommand if no Public IP --------
CONNECT_HOST="$TARGET_PUB"
if [ -z "$CONNECT_HOST" ] || [ "$CONNECT_HOST" = "-" ]; then
  CONNECT_HOST="$TARGET_PRIV"
  if [ -z "$CONNECT_HOST" ] || [ "$CONNECT_HOST" = "-" ]; then
    printf '%b\n' "${RD}✖ Node '${TARGET_NODE}' has neither Public IP nor Private IP.${RS}"
    exit 1
  fi

  BASTION_INFO="$(
    tb_get "/ns/${NS_ARG}/infra/${INFRA_ARG}/node/${TARGET_NODE}/bastion" | python3 -c '
import json, sys
try:
    d = json.load(sys.stdin)
    items = d if isinstance(d, list) else (d.get("output") or d.get("bastionNodes") or [])
    if items:
        b = items[0]
        print("\t".join([b.get("nsId") or "", b.get("infraId") or "", b.get("nodeId") or ""]))
except Exception:
    pass
'
  )"

  B_NS=""; B_INFRA=""; B_NODE=""
  if [ -n "$BASTION_INFO" ]; then
    IFS=$'\t' read -r B_NS B_INFRA B_NODE <<< "$BASTION_INFO"
  fi
  [ -z "$B_NS" ] && B_NS="$NS_ARG"
  [ -z "$B_INFRA" ] && B_INFRA="$INFRA_ARG"

  # Fallback: pick any running node with a public IP in the same Infra if bastion API returned empty
  if [ -z "$B_NODE" ]; then
    for r in "${NODE_ROWS[@]}"; do
      IFS=$'\t' read -r nid _ st pub _ _ _ _ _ <<< "$r"
      if [ -n "$pub" ] && [ "$pub" != "-" ] && [ "$nid" != "$TARGET_NODE" ]; then
        B_NODE="$nid"
        break
      fi
    done
  fi

  if [ -z "$B_NODE" ]; then
    printf '%b\n' "${RD}✖ Node '${TARGET_NODE}' only has Private IP (${CONNECT_HOST}) and no reachable Bastion node was found.${RS}"
    exit 1
  fi

  B_NODE_JSON="$(tb_get "/ns/${B_NS}/infra/${B_INFRA}/node/${B_NODE}")"
  read -r B_IP B_PORT B_NODE_USER B_KEY_ID < <(
    printf '%s' "$B_NODE_JSON" | python3 -c '
import json, sys
d = json.load(sys.stdin)
print(
    d.get("publicIP") or d.get("publicIp") or "",
    d.get("sshPort") or 22,
    d.get("nodeUserName") or d.get("vmUserName") or "cb-user",
    d.get("sshKeyId") or "",
)
'
  )

  if [ -z "${B_IP:-}" ]; then
    printf '%b\n' "${RD}✖ Bastion node '${B_NODE}' has no Public IP.${RS}"
    exit 1
  fi

  BASTION_KEY_FILE="${TMP_DIR}/bastion.pem"
  B_KEY_USER="$(fetch_ssh_key "$B_NS" "$B_KEY_ID" "$BASTION_KEY_FILE" || true)"
  if [ ! -s "$BASTION_KEY_FILE" ]; then
    printf '%b\n' "${RD}✖ Could not retrieve SSH private key for bastion '${B_NODE}'.${RS}"
    exit 1
  fi
  B_LOGIN_USER="${B_KEY_USER:-${B_NODE_USER:-cb-user}}"

  PROXY_CMD="ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=15 -i ${BASTION_KEY_FILE} -p ${B_PORT} -W %h:%p ${B_LOGIN_USER}@${B_IP}"
  SSH_CMD+=(-o "ProxyCommand=${PROXY_CMD}")
  printf '%b\n' "  ${YL}↪ Bastion jump:${RS} ${B_LOGIN_USER}@${B_IP}:${B_PORT} (${B_NODE}) → ${LOGIN_USER}@${CONNECT_HOST}:${TARGET_PORT}"
fi

# --- 4) Optional Local Port Forwarding (-L) ----------------------------------
if [ -n "$PORT_FWD_ARG" ]; then
  IFS=', ' read -r -a FWD_SPECS <<< "$PORT_FWD_ARG"
  for spec in "${FWD_SPECS[@]}"; do
    [ -z "$spec" ] && continue
    # Allow shorthand L=8080:80 -> 8080:127.0.0.1:80
    colon_cnt="$(awk -F: '{print NF-1}' <<< "$spec")"
    if [ "$colon_cnt" -eq 1 ]; then
      spec="${spec%%:*}:127.0.0.1:${spec##*:}"
    fi
    SSH_CMD+=(-L "$spec")
    printf '%b\n' "  ${GN}⇄ Forwarding:${RS}  localhost:${spec%%:*} → remote ${spec#*:}"
  done
fi

SSH_CMD+=("${LOGIN_USER}@${CONNECT_HOST}")

if [ -n "$CMD_ARG" ]; then
  SSH_CMD+=("$CMD_ARG")
fi

printf '%b\n' "  ${GN}▶ Connecting:${RS}  ${BD}${LOGIN_USER}@${CONNECT_HOST}:${TARGET_PORT}${RS} ${DM}(${NS_ARG}/${INFRA_ARG}/${TARGET_NODE}, ${TARGET_CSPREG})${RS}"

if [ "$DRY_RUN" = "1" ]; then
  printf '%b\n' "${DM}(dry-run) ${SSH_CMD[*]}${RS}"
  exit 0
fi

exec_rc=0
"${SSH_CMD[@]}" || exec_rc=$?
exit "$exec_rc"
