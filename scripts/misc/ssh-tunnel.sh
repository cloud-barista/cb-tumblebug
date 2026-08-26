#!/usr/bin/env bash
#
# Interactive SSH tunnel to a remote cb-tumblebug cluster, exposing its services on THIS
# machine's localhost so you can open them in a browser. Understands both access layouts and
# auto-detects which one is live on the remote node:
#
#   - Gateway single-entrypoint : HTTP :8080  / HTTPS :8443   (MapUI + API + MCP under one port)
#   - Direct services           : API :1323, MapUI :1324, Grafana :3000
#
# It discovers the remote host + SSH key from a cb-tumblebug server (or you enter them),
# picks conflict-free LOCAL ports, opens the tunnel in the background, and prints ready-to-
# click links.
#
# Prereqs on the REMOTE node (run there first): the matching port-forwards must be up —
#   gateway:  make k-gateway-forward         (:8080 / :8443)
#   direct :  make k-port-forward            (:1323 / :1324, + :3000 when observability is on)
#
# Usage: make k-tunnel     |     stop: make k-tunnel-stop

set -euo pipefail

STATE="${TMPDIR:-/tmp}/cb-ssh-tunnel"; mkdir -p "$STATE"
PIDFILE="$STATE/tunnel.pid"; LOG="$STATE/tunnel.log"; LINKS="$STATE/links.txt"
SSH_OPTS="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=12 -o ServerAliveInterval=30 -o ServerAliveCountMax=3"

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
    pkill -f "ssh -N .*cb-ssh-tunnel" 2>/dev/null && printf '%b\n' "${GN}✔ tunnel stopped${RS}" || printf '%b\n' "${DM}(no tunnel running)${RS}"
  fi
  exit 0
fi

printf '%b\n' "${BD}${CY}▌ cb-tumblebug SSH tunnel${RS} ${DM}(remote cluster → your localhost)${RS}"

# --- 1) remote target: discover from cb-tumblebug, or enter manually ---------
HOST=""; USER="cb-user"; KEY=""
if [ "$(ask "Discover target from a cb-tumblebug server? (y/n)" "y")" = "y" ]; then
  TB=$(ask "  cb-tumblebug API endpoint" "localhost:1323")
  NS=$(ask "  namespace" "default")
  mapfile -t INFRAS < <(curl -s -u default:default "http://$TB/tumblebug/ns/$NS/infra?option=id" 2>/dev/null \
      | python3 -c "import json,sys;print('\n'.join(json.load(sys.stdin).get('output') or []))" 2>/dev/null || true)
  if [ "${#INFRAS[@]}" -eq 0 ]; then
    printf '%b\n' "${YL}⚠ no infras found via $TB — switch to manual entry${RS}"
  else
    if [ "${#INFRAS[@]}" -eq 1 ]; then INFRA="${INFRAS[0]}"; else
      echo "  infras:"; i=1; for x in "${INFRAS[@]}"; do printf "   %d) %s\n" "$i" "$x"; i=$((i+1)); done
      sel=$(ask "  pick" "1"); INFRA="${INFRAS[$((sel-1))]}"
    fi
    printf '%b\n' "  ${DM}using infra:${RS} $INFRA"
    # node public IP + ssh key id + user (first node with a public IP)
    NODE_JSON=$(curl -s -u default:default "http://$TB/tumblebug/ns/$NS/infra/$INFRA" 2>/dev/null || true)
    NODE_LINE=$(printf '%s' "$NODE_JSON" | python3 -c "
import json,sys
try: d=json.load(sys.stdin)
except Exception: sys.exit(0)
for v in (d.get('node') or d.get('vm') or []):
    ip=v.get('publicIP') or v.get('publicIp')
    if ip:
        print(ip, v.get('sshKeyId') or v.get('sshKey') or '', v.get('nodeUserName') or v.get('vmUserName') or v.get('userName') or 'cb-user'); break
" 2>/dev/null || true)
    HOST=""; SKEY=""; NUSER=""
    read -r HOST SKEY NUSER <<< "$NODE_LINE" || true
    [ -n "${NUSER:-}" ] && USER="$NUSER"
    if [ -n "${SKEY:-}" ]; then
      KEY="$STATE/key.pem"
      curl -s -u default:default "http://$TB/tumblebug/ns/$NS/resources/sshKey/$SKEY" 2>/dev/null \
        | python3 -c "import json,sys;k=json.load(sys.stdin).get('privateKey','');sys.stdout.write(k if k.endswith('\n') else k+'\n')" > "$KEY" 2>/dev/null || true
      chmod 600 "$KEY" 2>/dev/null || true
      [ -s "$KEY" ] && printf '%b\n' "  ${DM}ssh key fetched → $KEY${RS}"
    fi
  fi
fi
[ -n "$HOST" ] || HOST=$(ask "  remote host / public IP" "")
[ -n "$KEY" ]  || { KEY=$(ask "  ssh private key path" "$HOME/.ssh/id_rsa"); KEY="${KEY/#\~/$HOME}"; }
USER=$(ask "  ssh user" "$USER")
printf '%b\n' "  ${DM}target:${RS} $USER@$HOST  ${DM}key:${RS} $KEY"

# --- 2) detect which services are listening on the remote node ---------------
printf '%b' "  detecting remote services... "
DETECTED=$(ssh $SSH_OPTS -i "$KEY" "$USER@$HOST" \
  "ss -tlnH 2>/dev/null | awk '{print \$4}' | grep -oE '[0-9]+\$' | sort -u" 2>/dev/null \
  | grep -E '^(8443|8080|1323|1324|3000)$' | sort -u | tr '\n' ' ' || true)
printf '%b\n' "${DETECTED:-none}"
has(){ printf ' %s ' "$DETECTED" | grep -q " $1 "; }

# MCP is optional (mcp.enabled) and its /mcp backend is either the mcp-server or, when
# agentgateway is on, the governed agentgateway. Only advertise MCP if it's actually deployed.
MCPBK=$(ssh $SSH_OPTS -i "$KEY" "$USER@$HOST" \
  "kubectl get svc -A -o name 2>/dev/null | grep -oE 'agentgateway|cb-tumblebug-mcp-server' | head -1" 2>/dev/null || true)

# --- 3) choose what to tunnel (default from detection) -----------------------
# Build parallel arrays: remote port, scheme, local port, and the links to print.
RP=(); SCHEME=(); LABEL=()
add(){ RP+=("$1"); SCHEME+=("$2"); LABEL+=("$3"); }

# Reflect the real gateway endpoint: HTTPS when gateway TLS is on (:8443), else HTTP (:8080).
# The SSH tunnel already encrypts the hop, so if you'd rather skip the self-signed-cert
# warning, force the HTTP port with:  SCHEME=http make k-tunnel   (when :8080 is up).
GW=""
if [ "${SCHEME:-}" = "http" ] && has 8080; then GW="8080:http"
elif has 8443; then GW="8443:https"
elif has 8080; then GW="8080:http"; fi
# Auto-pick the mode (no confusing prompt): gateway when a gateway port is up, else direct.
MODE="${MODE:-$([ -n "$GW" ] && echo gateway || echo direct)}"
printf '%b\n' "  mode: ${BD}$MODE${RS} ${DM}(auto — override with: MODE=gateway|direct make k-tunnel)${RS}"

if [ "$MODE" = "gateway" ] && [ -n "$GW" ]; then
  add "${GW%%:*}" "${GW##*:}" gateway
else
  [ "$MODE" = "gateway" ] && printf '%b\n' "${YL}  ⚠ no gateway (8080/8443) detected — using direct services${RS}"
  has 1323 && add 1323 http api
  has 1324 && add 1324 http mapui
fi
# Grafana is outside the gateway — always tunnel it when present (orthogonal to the mode).
has 3000 && add 3000 http grafana

if [ "${#RP[@]}" -eq 0 ]; then
  printf '%b\n' "${YL}✖ no services detected on the remote node.${RS}"
  printf '%b\n' "  Start the forwards there first — ${CY}make k-gateway-forward${RS} and/or ${CY}make k-port-forward${RS} — then retry."
  exit 1
fi

# --- 4) close any previous tunnel first, then assign conflict-free local ports
# (stopping before selection lets a re-run reuse the same ports instead of climbing) --------
[ "${DRY_RUN:-}" = "1" ] || "$0" stop >/dev/null 2>&1 || true
LP=()
for i in "${!RP[@]}"; do LP+=("$(free_port $(( ${RP[$i]} + 10000 )) )"); done

# --- 5) open the tunnel ------------------------------------------------------
FWD=(); for i in "${!RP[@]}"; do FWD+=(-L "${LP[$i]}:127.0.0.1:${RP[$i]}"); done
if [ "${DRY_RUN:-}" = "1" ]; then
  printf '%b\n' "${DM}(dry-run) would open:${RS} ssh -N ${FWD[*]} $USER@$HOST"
else
  printf '%b\n' "${DM}opening tunnel:${RS} ssh -N ${FWD[*]} $USER@$HOST"
  nohup ssh -N $SSH_OPTS -o ExitOnForwardFailure=yes "${FWD[@]}" -i "$KEY" "$USER@$HOST" > "$LOG" 2>&1 &
  echo $! > "$PIDFILE"
  sleep 4
  if ! kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
    printf '%b\n' "${YL}✖ tunnel failed to start — log:${RS}"; grep -vi "Permanently added" "$LOG" | head; exit 1
  fi
fi

# --- 6) print clickable links ------------------------------------------------
: > "$LINKS"
if [ "${DRY_RUN:-}" = "1" ]; then printf '%b\n' "" "${DM}(dry-run — tunnel not opened)${RS}"
else printf '%b\n' "" "${GN}✔ tunnel up${RS} ${DM}(stop: make k-tunnel-stop)${RS}"; fi
# vfy: probe a URL, return a ✓ / ⚠<code> marker (empty in dry-run) so links are trustworthy.
vfy(){ [ "${DRY_RUN:-}" = "1" ] && return 0
  local code; code=$(curl -sk -o /dev/null -w '%{http_code}' --max-time 6 "$1" 2>/dev/null || echo 000)
  case "$code" in 2*|3*) printf " ${GN}✓${RS}";; *) printf " ${YL}⚠ %s${RS}" "$code";; esac; }
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
        printf "  ${CY}%-7s${RS} %s\n" "MCP" "$base/mcp" | tee -a "$LINKS"   # streaming endpoint — not probed
        [ "$MCPBK" = agentgateway ] && printf '%b\n' "${DM}          (governed via agentgateway — token may be needed: make k-mcp-token)${RS}"
      else
        printf '%b\n' "  ${DM}MCP     (not deployed — enable with: make k-mcp-on)${RS}"
      fi ;;
    api)     link API     "$base/tumblebug/api" ;;
    mapui)   link MapUI   "$base/" ;;
    grafana) link Grafana "$base/" ;;
  esac
done
[ "$have_https" = 1 ] && printf '%b\n' "${DM}  https is self-signed → accept the browser warning once (or: SCHEME=http make k-tunnel to use the gateway's HTTP port)${RS}"
[ "$MODE" = direct ] && printf '%b\n' "${DM}  direct mode: in MapUI, set its API endpoint to the API link above.${RS}"
