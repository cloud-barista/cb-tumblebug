#!/usr/bin/env bash
#
# Interactive or non-interactive SSH tunnel to a remote cb-tumblebug cluster, exposing its
# services on THIS machine's localhost so you can open them in a browser.
#
#   - Gateway single-entrypoint : HTTP :8080  / HTTPS :8443   (MapUI + API + MCP under one port)
#   - Direct services           : API :1323, MapUI :1324, Grafana :3000
#
# Usage:
#   make k-tunnel                  # Interactive discovery from local CB-Tumblebug
#   make k-tunnel HOST=tb          # Zero-prompt auto-discovery for Infra/Node/IP `tb`
#   make k-tunnel-stop             # Stop active background SSH tunnel

set -euo pipefail

STATE="${TMPDIR:-/tmp}/cb-ssh-tunnel"; mkdir -p "$STATE"
PIDFILE="$STATE/tunnel.pid"; LOG="$STATE/tunnel.log"; LINKS="$STATE/links.txt"
SSH_OPTS="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -o ConnectTimeout=12 -o ServerAliveInterval=30 -o ServerAliveCountMax=3"

CY=$'\033[36m'; GN=$'\033[32m'; YL=$'\033[33m'; DM=$'\033[2m'; BD=$'\033[1m'; RS=$'\033[0m'
ask(){ local a; read -rp "$1 ${DM}[$2]${RS}: " a; printf '%s' "${a:-$2}"; }
port_used(){ ss -tlnH 2>/dev/null | awk '{print $4}' | grep -qE "[:.]$1$"; }
free_port(){ local p="$1"; while port_used "$p"; do p=$((p+1)); done; printf '%s' "$p"; }

# --- stop sub-command --------------------------------------------------------
if [ "${1:-}" = "stop" ]; then
  if [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
    kill "$(cat "$PIDFILE")" 2>/dev/null || true; rm -f "$PIDFILE"
    printf '%b\n' "${GN}✔ SSH tunnel stopped${RS}"
  else
    pkill -f "ssh -N .*127.0.0.1:" 2>/dev/null && printf '%b\n' "${GN}✔ tunnel stopped${RS}" || printf '%b\n' "${DM}(no tunnel running)${RS}"
  fi
  exit 0
fi

printf '%b\n' "${BD}${CY}▌ cb-tumblebug SSH tunnel${RS} ${DM}(remote cluster → your localhost)${RS}"

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

detect_local_tb() {
  for cand in "http://localhost:8080/tumblebug" "https://localhost:8443/tumblebug" "http://localhost:1323/tumblebug"; do
    if curl -sf -k -u "${API_USER}:${API_PASS}" --max-time 2 "${cand}/readyz" >/dev/null 2>&1; then
      printf '%s' "$cand"
      return 0
    fi
  done
  return 1
}

# --- 1) remote target: non-interactive HOST=... or interactive discovery ------
TARGET_ARG="${REMOTE_HOST:-${HOST:-${IP:-}}}"
HOST=""; USER="${SSH_USER:-cb-user}"; KEY="${SSH_KEY:-}"; PORT="${SSH_PORT:-22}"
[ -n "$KEY" ] && KEY="${KEY/#\~/$HOME}"

if [ -n "$TARGET_ARG" ]; then
  case "$TARGET_ARG" in
    *@*) [ -z "${SSH_USER:-}" ] && USER="${TARGET_ARG%@*}"; HOST="${TARGET_ARG#*@}" ;;
    *)   HOST="$TARGET_ARG" ;;
  esac
  LOCAL_TB="$(detect_local_tb || true)"
  if [ -z "$KEY" ] && [ -n "$LOCAL_TB" ]; then
    FOUND="$(
      LOCAL_TB="$LOCAL_TB" TARGET_Q="$HOST" NS_F="${NS:-}" API_USER="$API_USER" API_PASS="$API_PASS" python3 - <<'PY'
import json, os, subprocess, sys
tb = os.environ["LOCAL_TB"]; q = os.environ["TARGET_Q"]; ns_f = os.environ.get("NS_F", "")
u = os.environ["API_USER"]; pw = os.environ["API_PASS"]
def get_json(p):
    try:
        return json.loads(subprocess.check_output(["curl","-sS","-k","-u",f"{u}:{pw}","--max-time","8",tb+p], stderr=subprocess.DEVNULL).decode())
    except Exception:
        return {}
ns_list = [ns_f] if ns_f else [x.get("id") for x in (get_json("/ns").get("ns") or []) if x.get("id")]
if "default" in ns_list:
    ns_list.remove("default"); ns_list.insert(0, "default")
for ns in ns_list:
    infras = get_json(f"/ns/{ns}/infra?option=id").get("output") or []
    order = ([q] + [x for x in infras if x != q]) if q in infras else infras
    for iid in order:
        d = get_json(f"/ns/{ns}/infra/{iid}")
        for v in (d.get("node") or d.get("vm") or []):
            ip = v.get("publicIP") or v.get("publicIp") or ""
            nid = v.get("id") or ""
            if ip and (q in (iid, ip, nid)):
                print(f"{ip}\t{v.get('sshKeyId') or v.get('sshKey') or ''}\t{v.get('nodeUserName') or v.get('vmUserName') or 'cb-user'}\t{v.get('sshPort') or 22}\t{ns}")
                sys.exit(0)
PY
    )"
    if [ -n "$FOUND" ]; then
      IFS=$'\t' read -r RES_IP RES_SKEY RES_USER RES_PORT RES_NS <<< "$FOUND"
      HOST="$RES_IP"
      [ -z "${SSH_USER:-}" ] && USER="$RES_USER"
      [ "${SSH_PORT:-22}" = "22" ] && PORT="$RES_PORT"
      if [ -n "$RES_SKEY" ]; then
        KEY="$STATE/key.pem"
        curl -sS -k -u "${API_USER}:${API_PASS}" "${LOCAL_TB}/ns/${RES_NS}/resources/sshKey/${RES_SKEY}" \
          | python3 -c "import json,sys;d=json.load(sys.stdin);k=(d.get('privateKey') or '').replace('\\\\n','\n');sys.stdout.write(k if k.endswith('\n') else k+'\n')" > "$KEY" 2>/dev/null || true
        chmod 600 "$KEY" 2>/dev/null || true
      fi
    fi
  fi
  [ -n "$KEY" ] || KEY="$HOME/.ssh/id_rsa"
else
  DEF_TB="$(detect_local_tb || echo "http://localhost:1323/tumblebug")"
  if [ "$(ask "Discover target from a cb-tumblebug server? (y/n)" "y")" = "y" ]; then
    TB_IN=$(ask "  cb-tumblebug base URL" "$DEF_TB")
    TB_IN="${TB_IN%/}"
    [[ "$TB_IN" == http* ]] || TB_IN="http://${TB_IN}"
    [[ "$TB_IN" == */tumblebug ]] || TB_IN="${TB_IN}/tumblebug"
    NS=$(ask "  namespace" "default")
    mapfile -t INFRAS < <(curl -s -k -u "${API_USER}:${API_PASS}" "${TB_IN}/ns/${NS}/infra?option=id" 2>/dev/null \
        | python3 -c "import json,sys;print('\n'.join(json.load(sys.stdin).get('output') or []))" 2>/dev/null || true)
    if [ "${#INFRAS[@]}" -eq 0 ]; then
      printf '%b\n' "${YL}⚠ no infras found via $TB_IN — switch to manual entry${RS}"
    else
      if [ "${#INFRAS[@]}" -eq 1 ]; then INFRA="${INFRAS[0]}"; else
        echo "  infras:"; i=1; for x in "${INFRAS[@]}"; do printf "   %d) %s\n" "$i" "$x"; i=$((i+1)); done
        sel=$(ask "  pick" "1"); INFRA="${INFRAS[$((sel-1))]}"
      fi
      printf '%b\n' "  ${DM}using infra:${RS} $INFRA"
      NODE_JSON=$(curl -s -k -u "${API_USER}:${API_PASS}" "${TB_IN}/ns/${NS}/infra/${INFRA}" 2>/dev/null || true)
      NODE_LINE=$(printf '%s' "$NODE_JSON" | python3 -c "
import json,sys
try: d=json.load(sys.stdin)
except Exception: sys.exit(0)
for v in (d.get('node') or d.get('vm') or []):
    ip=v.get('publicIP') or v.get('publicIp')
    if ip:
        print(ip, v.get('sshKeyId') or v.get('sshKey') or '', v.get('nodeUserName') or v.get('vmUserName') or v.get('userName') or 'cb-user', v.get('sshPort') or 22); break
" 2>/dev/null || true)
      SKEY=""; NUSER=""; NPORT=""
      read -r HOST SKEY NUSER NPORT <<< "$NODE_LINE" || true
      [ -n "${NUSER:-}" ] && USER="$NUSER"
      [ -n "${NPORT:-}" ] && PORT="$NPORT"
      if [ -n "${SKEY:-}" ]; then
        KEY="$STATE/key.pem"
        curl -s -k -u "${API_USER}:${API_PASS}" "${TB_IN}/ns/${NS}/resources/sshKey/${SKEY}" 2>/dev/null \
          | python3 -c "import json,sys;d=json.load(sys.stdin);k=(d.get('privateKey') or '').replace('\\\\n','\n');sys.stdout.write(k if k.endswith('\n') else k+'\n')" > "$KEY" 2>/dev/null || true
        chmod 600 "$KEY" 2>/dev/null || true
        [ -s "$KEY" ] && printf '%b\n' "  ${DM}ssh key fetched → $KEY${RS}"
      fi
    fi
  fi
  [ -n "$HOST" ] || HOST=$(ask "  remote host / public IP" "")
  [ -n "$KEY" ]  || { KEY=$(ask "  ssh private key path" "$HOME/.ssh/id_rsa"); KEY="${KEY/#\~/$HOME}"; }
  USER=$(ask "  ssh user" "$USER")
fi

printf '%b\n' "  ${DM}target:${RS} $USER@$HOST:$PORT  ${DM}key:${RS} $KEY"

# --- 2) detect which services are listening on the remote node ---------------
printf '%b' "  detecting remote services... "
DETECTED=$(ssh $SSH_OPTS -p "$PORT" -i "$KEY" "$USER@$HOST" \
  "ss -tlnH 2>/dev/null | awk '{print \$4}' | grep -oE '[0-9]+\$' | sort -u" 2>/dev/null \
  | grep -E '^(8443|8080|1323|1324|3000)$' | sort -u | tr '\n' ' ' || true)
printf '%b\n' "${DETECTED:-none}"
has(){ printf ' %s ' "$DETECTED" | grep -q " $1 "; }

# If nothing is forwarded yet on a remote K8s cluster, auto-start k-port-forward once!
if [ -z "$DETECTED" ]; then
  printf '%b\n' "  ${YL}↳ No active forwards detected; attempting to start remote k-port-forward...${RS}"
  ssh $SSH_OPTS -p "$PORT" -i "$KEY" "$USER@$HOST" \
    'cd "$HOME/go/src/github.com/cloud-barista/cb-tumblebug" 2>/dev/null && make --no-print-directory k-port-forward </dev/null >/dev/null 2>&1 || true' || true
  sleep 2
  DETECTED=$(ssh $SSH_OPTS -p "$PORT" -i "$KEY" "$USER@$HOST" \
    "ss -tlnH 2>/dev/null | awk '{print \$4}' | grep -oE '[0-9]+\$' | sort -u" 2>/dev/null \
    | grep -E '^(8443|8080|1323|1324|3000)$' | sort -u | tr '\n' ' ' || true)
  printf '%b\n' "  re-detected remote services: ${DETECTED:-none}"
fi

MCPBK=$(ssh $SSH_OPTS -p "$PORT" -i "$KEY" "$USER@$HOST" \
  "kubectl get svc -A -o name 2>/dev/null | grep -oE 'agentgateway|cb-tumblebug-mcp-server' | head -1" 2>/dev/null || true)

# --- 3) choose what to tunnel (default from detection) -----------------------
RP=(); SCHEME=(); LABEL=()
add(){ RP+=("$1"); SCHEME+=("$2"); LABEL+=("$3"); }

MODE="${MODE:-$( (has 8080 || has 8443) && echo gateway || echo direct )}"
printf '%b\n' "  mode: ${BD}$MODE${RS} ${DM}(auto — override with: MODE=gateway|direct make k-tunnel)${RS}"

if [ "$MODE" = "gateway" ] && { has 8080 || has 8443; }; then
  if [ "${SCHEME_OVERRIDE:-${SCHEME:-}}" = "http" ]; then
    has 8080 && add 8080 http gateway
  elif [ "${SCHEME_OVERRIDE:-${SCHEME:-}}" = "https" ]; then
    has 8443 && add 8443 https gateway
  else
    has 8080 && add 8080 http gateway
    has 8443 && add 8443 https gateway
  fi
else
  [ "$MODE" = "gateway" ] && printf '%b\n' "${YL}  ⚠ no gateway (8080/8443) detected — using direct services${RS}"
  has 1323 && add 1323 http api
  has 1324 && add 1324 http mapui
fi
has 3000 && add 3000 http grafana

if [ "${#RP[@]}" -eq 0 ]; then
  printf '%b\n' "${YL}✖ no services detected on the remote node.${RS}"
  printf '%b\n' "  Deploy first with ${CY}make k-up HOST=${TARGET_ARG:-<ip>}${RS} or start forwards with ${CY}make k-port-forward${RS}."
  exit 1
fi

# --- 4) close any previous tunnel first, then assign conflict-free local ports
[ "${DRY_RUN:-}" = "1" ] || "$0" stop >/dev/null 2>&1 || true
LP=()
for i in "${!RP[@]}"; do LP+=("$(free_port $(( ${RP[$i]} + 10000 )) )"); done

# --- 5) open the tunnel ------------------------------------------------------
FWD=(); for i in "${!RP[@]}"; do FWD+=(-L "${LP[$i]}:127.0.0.1:${RP[$i]}"); done
if [ "${DRY_RUN:-}" = "1" ]; then
  printf '%b\n' "${DM}(dry-run) would open:${RS} ssh -N -p $PORT ${FWD[*]} $USER@$HOST"
else
  printf '%b\n' "${DM}opening tunnel:${RS} ssh -N -p $PORT ${FWD[*]} $USER@$HOST"
  if command -v setsid >/dev/null 2>&1; then
    setsid ssh -N $SSH_OPTS -p "$PORT" -o ExitOnForwardFailure=yes "${FWD[@]}" -i "$KEY" "$USER@$HOST" </dev/null > "$LOG" 2>&1 &
  else
    nohup ssh -N $SSH_OPTS -p "$PORT" -o ExitOnForwardFailure=yes "${FWD[@]}" -i "$KEY" "$USER@$HOST" </dev/null > "$LOG" 2>&1 &
  fi
  echo $! > "$PIDFILE"
  sleep 3
  if ! kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
    printf '%b\n' "${YL}✖ tunnel failed to start — log:${RS}"; grep -vi "Permanently added" "$LOG" | head; exit 1
  fi
fi

# --- 6) print clickable links ------------------------------------------------
: > "$LINKS"
if [ "${DRY_RUN:-}" = "1" ]; then printf '%b\n' "" "${DM}(dry-run — tunnel not opened)${RS}"
else printf '%b\n' "" "${GN}✔ tunnel up${RS} ${DM}(stop: make k-tunnel-stop)${RS}"; fi

vfy(){ [ "${DRY_RUN:-}" = "1" ] && return 0
  local code; code=$(curl -sk -o /dev/null -w '%{http_code}' --max-time 6 "$1" 2>/dev/null || echo 000)
  case "$code" in 2*|3*|401) printf " ${GN}✓${RS}";; *) printf " ${YL}⚠ %s${RS}" "$code";; esac; }
link(){ printf "  ${CY}%-7s${RS} %s%b\n" "$1" "$2" "$(vfy "$2")" | tee -a "$LINKS"; }

printf '%b\n' "${BD}Open in your browser:${RS}"
have_https=0
for i in "${!RP[@]}"; do
  s="${SCHEME[$i]}"; l="${LP[$i]}"; base="$s://localhost:$l"
  [ "$s" = https ] && have_https=1
  case "${LABEL[$i]}" in
    gateway)
      link MapUI "$base/"
      link API   "$base/tumblebug/api"
      if [ -n "$MCPBK" ]; then
        printf "  ${CY}%-7s${RS} %s\n" "MCP" "$base/mcp" | tee -a "$LINKS"
        [ "$MCPBK" = agentgateway ] && printf '%b\n' "${DM}          (governed via agentgateway)${RS}"
      else
        printf '%b\n' "  ${DM}MCP     (not deployed — enable with: make k-mcp-on)${RS}"
      fi ;;
    api)     link API     "$base/tumblebug/api" ;;
    mapui)   link MapUI   "$base/" ;;
    grafana) link Grafana "$base/" ;;
  esac
done
[ "$have_https" = 1 ] && printf '%b\n' "${DM}  https is self-signed → accept the browser warning once (or: SCHEME_OVERRIDE=http make k-tunnel HOST=${TARGET_ARG:-...} to use HTTP)${RS}" || true
[ "$MODE" = direct ] && printf '%b\n' "${DM}  direct mode: in MapUI, set its API endpoint to the API link above.${RS}" || true
