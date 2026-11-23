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

# Check whether an explicit (non-wildcard) Host stanza exists in ~/.ssh/config(.d)
ssh_config_has_host() {
  local h="$1" f
  [ -n "$h" ] || return 1
  for f in "$HOME/.ssh/config" "$HOME"/.ssh/config.d/*; do
    [ -f "$f" ] || continue
    awk -v t="$h" '
      tolower($1)=="host" {
        for (i=2; i<=NF; i++) {
          if ($i != "*" && $i !~ /[*?!]/ && $i == t) { found=1; exit 0 }
        }
      }
      END { exit(found ? 0 : 1) }
    ' "$f" 2>/dev/null && return 0
  done
  return 1
}

# List `# >>> cb-tumblebug:<alias> >>>` entries exported via `make ssh-config`
list_ssh_config_cb_hosts() {
  [ -f "$HOME/.ssh/config" ] || return 0
  awk '
    /^# >>> cb-tumblebug:/ {
      alias=$3; sub(/^cb-tumblebug:/, "", alias)
      hn=""; usr="cb-user"; prt="22"; in_blk=1; main_host=0; next
    }
    /^# <<< cb-tumblebug:/ {
      if (in_blk && alias != "" && hn != "") {
        printf "ssh-config\t%s\t%s\t%s\t%s\t%s\t\n", alias, alias, hn, usr, prt
      }
      in_blk=0; alias=""; next
    }
    in_blk {
      k=tolower($1)
      if (k=="host") { main_host = ($2 == alias ? 1 : 0) }
      else if (main_host && k=="hostname") { hn=$2 }
      else if (main_host && k=="user")     { usr=$2 }
      else if (main_host && k=="port")     { prt=$2 }
    }
  ' "$HOME/.ssh/config" 2>/dev/null || true
}

# --- 1) remote target: non-interactive HOST=... or interactive discovery ------
TARGET_ARG="${REMOTE_HOST:-${HOST:-${IP:-}}}"
HOST=""; USER="${SSH_USER:-}"; KEY="${SSH_KEY:-}"; PORT="${SSH_PORT:-}"
[ -n "$KEY" ] && KEY="${KEY/#\~/$HOME}"
RESOLVED_FROM_TB=0

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
      [ -z "${SSH_PORT:-}" ] && PORT="$RES_PORT"
      if [ -n "$RES_SKEY" ]; then
        KEY="$STATE/key.pem"
        curl -sS -k -u "${API_USER}:${API_PASS}" "${LOCAL_TB}/ns/${RES_NS}/resources/sshKey/${RES_SKEY}" \
          | python3 -c "import json,sys;d=json.load(sys.stdin);k=(d.get('privateKey') or '').replace('\\\\n','\n');sys.stdout.write(k if k.endswith('\n') else k+'\n')" > "$KEY" 2>/dev/null || true
        chmod 600 "$KEY" 2>/dev/null || true
      fi
      RESOLVED_FROM_TB=1
    fi
  fi
else
  LOCAL_TB="$(detect_local_tb || true)"
  VM_CANDIDATES=()
  if [ -n "$LOCAL_TB" ]; then
    mapfile -t VM_CANDIDATES < <(
      LOCAL_TB="$LOCAL_TB" API_USER="$API_USER" API_PASS="$API_PASS" python3 - <<'PY' 2>/dev/null || true
import json, os, subprocess
tb = os.environ["LOCAL_TB"]; u = os.environ["API_USER"]; pw = os.environ["API_PASS"]
def get_json(p):
    try:
        return json.loads(subprocess.check_output(["curl","-sS","-k","-u",f"{u}:{pw}","--max-time","4",tb+p], stderr=subprocess.DEVNULL).decode())
    except Exception:
        return {}
ns_list = [x.get("id") for x in (get_json("/ns").get("ns") or []) if x.get("id")]
if "default" in ns_list:
    ns_list.remove("default"); ns_list.insert(0, "default")
for ns in ns_list:
    for iid in (get_json(f"/ns/{ns}/infra?option=id").get("output") or []):
        d = get_json(f"/ns/{ns}/infra/{iid}")
        for v in (d.get("node") or d.get("vm") or []):
            ip = v.get("publicIP") or v.get("publicIp") or ""
            if ip:
                nid = v.get("id") or ""
                usr = v.get("nodeUserName") or v.get("vmUserName") or "cb-user"
                prt = v.get("sshPort") or 22
                skey = v.get("sshKeyId") or v.get("sshKey") or ""
                print(f"{ns}\t{iid}\t{nid}\t{ip}\t{usr}\t{prt}\t{skey}")
PY
    )
  fi

  # Also append cb-tumblebug hosts from ~/.ssh/config (for other workstations via `make ssh-config`)
  while IFS= read -r cfg_row; do
    [ -n "$cfg_row" ] || continue
    IFS=$'\t' read -r _ cfg_alias _ cfg_ip _ _ _ <<< "$cfg_row"
    dup=0
    for existing in "${VM_CANDIDATES[@]}"; do
      IFS=$'\t' read -r _ e_inf e_nid e_ip _ _ _ <<< "$existing"
      if [ "$cfg_alias" = "$e_inf" ] || [ "$cfg_alias" = "$e_nid" ] || [ "$cfg_ip" = "$e_ip" ]; then
        dup=1; break
      fi
    done
    [ "$dup" = "0" ] && VM_CANDIDATES+=("$cfg_row")
  done < <(list_ssh_config_cb_hosts)

  if [ "${#VM_CANDIDATES[@]}" -gt 0 ]; then
    printf '\n  %b\n' "${BD}Select remote CB-Tumblebug VM:${RS}"
    idx=1
    for row in "${VM_CANDIDATES[@]}"; do
      IFS=$'\t' read -r c_ns c_inf c_nid c_ip c_usr c_prt _ <<< "$row"
      if [ "$c_ns" = "ssh-config" ]; then
        printf "    ${CY}%d)${RS} ${BD}%s${RS} ${DM}(~/.ssh/config · %s@%s:%s)${RS}\n" "$idx" "$c_inf" "$c_usr" "$c_ip" "$c_prt"
      else
        printf "    ${CY}%d)${RS} ${BD}%s/%s${RS} ${DM}(%s · %s@%s:${c_prt})${RS}\n" "$idx" "$c_ns" "$c_inf" "$c_nid" "$c_usr" "$c_ip"
      fi
      idx=$((idx + 1))
    done
    manual_idx=$idx
    printf "    ${CY}%d)${RS} Enter remote host / IP manually\n" "$manual_idx"
    sel=$(ask "  Target #" "1")
    if [[ "$sel" =~ ^[0-9]+$ ]] && [ "$sel" -ge 1 ] && [ "$sel" -lt "$manual_idx" ]; then
      picked="${VM_CANDIDATES[$((sel - 1))]}"
      IFS=$'\t' read -r c_ns c_inf c_nid c_ip c_usr c_prt c_skey <<< "$picked"
      TARGET_ARG="$c_inf"
      if [ "$c_ns" = "ssh-config" ]; then
        HOST="$c_inf"
      else
        HOST="$c_ip"; USER="$c_usr"; PORT="$c_prt"
        RESOLVED_FROM_TB=1
        if [ -n "$c_skey" ]; then
          KEY="$STATE/key.pem"
          curl -s -k -u "${API_USER}:${API_PASS}" "${LOCAL_TB}/ns/${c_ns}/resources/sshKey/${c_skey}" 2>/dev/null \
            | python3 -c "import json,sys;d=json.load(sys.stdin);k=(d.get('privateKey') or '').replace('\\\\n','\n');sys.stdout.write(k if k.endswith('\n') else k+'\n')" > "$KEY" 2>/dev/null || true
          chmod 600 "$KEY" 2>/dev/null || true
        fi
      fi
      printf '  %b\n' "${DM}💡 Direct command: ${RS}${CY}make k-tunnel HOST=${c_inf}${RS}"
    fi
  fi

  if [ -z "$HOST" ]; then
    HOST=$(ask "  remote host / SSH alias / public IP" "")
    if ! ssh_config_has_host "$HOST"; then
      KEY=$(ask "  ssh private key path" "$HOME/.ssh/id_rsa"); KEY="${KEY/#\~/$HOME}"
      USER=$(ask "  ssh user" "${USER:-cb-user}")
    fi
  fi
fi

# Build final SSH target & options (respecting ~/.ssh/config when HOST is a configured alias)
if [ "$RESOLVED_FROM_TB" = "0" ] && ssh_config_has_host "$HOST"; then
  SSH_DEST="${USER:+${USER}@}${HOST}"
else
  USER="${USER:-cb-user}"
  PORT="${PORT:-22}"
  SSH_DEST="${USER}@${HOST}"
fi

SSH_CALL=(ssh $SSH_OPTS)
[ -n "$PORT" ] && SSH_CALL+=(-p "$PORT")
[ -n "$KEY" ]  && SSH_CALL+=(-i "$KEY")

if [ "${DRY_RUN:-}" != "1" ] && ! "${SSH_CALL[@]}" -o BatchMode=yes "$SSH_DEST" true >/dev/null 2>&1; then
  if [ -z "$KEY" ] && [ -t 0 ]; then
    printf '%b\n' "  ${YL}⚠ Could not connect to ${SSH_DEST} automatically.${RS}"
    KEY=$(ask "  ssh private key path" "$HOME/.ssh/id_rsa"); KEY="${KEY/#\~/$HOME}"
    USER=$(ask "  ssh user" "${USER:-cb-user}")
    SSH_DEST="${USER}@${HOST}"
    SSH_CALL=(ssh $SSH_OPTS)
    [ -n "$PORT" ] && SSH_CALL+=(-p "$PORT")
    [ -n "$KEY" ]  && SSH_CALL+=(-i "$KEY")
  fi
fi

printf '%b\n' "  ${DM}target:${RS} ${SSH_DEST}${PORT:+:$PORT}  ${DM}key:${RS} ${KEY:-(ssh-config / default)}"

# --- 2) detect which services are listening on the remote node ---------------
printf '%b' "  detecting remote services... "
DETECTED=$("${SSH_CALL[@]}" "$SSH_DEST" \
  "ss -tlnH 2>/dev/null | awk '{print \$4}' | grep -oE '[0-9]+\$' | sort -u" 2>/dev/null \
  | grep -E '^(8443|8080|1323|1324|3000)$' | sort -u | tr '\n' ' ' || true)
printf '%b\n' "${DETECTED:-none}"
has(){ printf ' %s ' "$DETECTED" | grep -q " $1 "; }

# If nothing is forwarded yet on a remote K8s cluster, auto-start k-port-forward once!
if [ -z "$DETECTED" ]; then
  printf '%b\n' "  ${YL}↳ No active forwards detected; attempting to start remote k-port-forward...${RS}"
  "${SSH_CALL[@]}" "$SSH_DEST" \
    'cd "$HOME/go/src/github.com/cloud-barista/cb-tumblebug" 2>/dev/null && make --no-print-directory k-port-forward </dev/null >/dev/null 2>&1 || true' || true
  sleep 2
  DETECTED=$("${SSH_CALL[@]}" "$SSH_DEST" \
    "ss -tlnH 2>/dev/null | awk '{print \$4}' | grep -oE '[0-9]+\$' | sort -u" 2>/dev/null \
    | grep -E '^(8443|8080|1323|1324|3000)$' | sort -u | tr '\n' ' ' || true)
  printf '%b\n' "  re-detected remote services: ${DETECTED:-none}"
fi

MCPBK=$("${SSH_CALL[@]}" "$SSH_DEST" \
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
TUN_SSH=(ssh -N $SSH_OPTS)
[ -n "$PORT" ] && TUN_SSH+=(-p "$PORT")
TUN_SSH+=(-o ExitOnForwardFailure=yes "${FWD[@]}")
[ -n "$KEY" ]  && TUN_SSH+=(-i "$KEY")
TUN_SSH+=("$SSH_DEST")
if [ "${DRY_RUN:-}" = "1" ]; then
  printf '%b\n' "${DM}(dry-run) would open:${RS} ssh -N ${PORT:+-p $PORT }${FWD[*]} $SSH_DEST"
else
  printf '%b\n' "${DM}opening tunnel:${RS} ssh -N ${PORT:+-p $PORT }${FWD[*]} $SSH_DEST"
  if command -v setsid >/dev/null 2>&1; then
    setsid "${TUN_SSH[@]}" </dev/null > "$LOG" 2>&1 &
  else
    nohup "${TUN_SSH[@]}" </dev/null > "$LOG" 2>&1 &
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
