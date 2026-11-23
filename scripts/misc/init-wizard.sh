#!/usr/bin/env bash
#
# Unified Initialization Runner & Interactive Wizard for CB-Tumblebug (`make k-init` / `make init`).
#
# - When invoked interactively with no scope/target flags, guides the user step-by-step:
#     1) Target: Local cluster/compose vs auto-discovered Remote Infra VMs vs Manual IP
#     2) Scope:  Full Init / Credentials only / Backup DB + Templates / Templates only / Fresh CSP fetch
#   and prints the equivalent one-line command for future automation.
# - Also translates flat Make variables (ONLY=cred|assets|templates|price, PROVIDERS=..., YES=1)
#   so users never need nested quotes like ARGS="-y --credentials-only" (while keeping ARGS compatible).

set -euo pipefail

MODE="${INIT_MODE:-k8s}" # k8s | compose
K8S_NAMESPACE="${K8S_NAMESPACE:-cb-tumblebug}"
K8S_CONTEXT="${K8S_CONTEXT:-}"
K8S_INIT_PORT="${K8S_INIT_PORT:-11323}"
KUBECTL_CMD="kubectl${K8S_CONTEXT:+ --context $K8S_CONTEXT}"

TARGET_IN="${REMOTE_HOST:-${HOST:-${IP:-}}}"
SSH_USER_IN="${SSH_USER:-}"
SSH_KEY_IN="${SSH_KEY:-}"
SSH_PORT_IN="${SSH_PORT:-}"
ONLY_IN="${ONLY:-}"
PROVIDERS_IN="${PROVIDERS:-}"
YES_IN="${YES:-}"
RAW_ARGS="${ARGS:-}"

CY=$'\033[36m'; GN=$'\033[32m'; YL=$'\033[33m'; RD=$'\033[31m'; DM=$'\033[2m'; BD=$'\033[1m'; RS=$'\033[0m'

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

detect_local_tb() {
  for cand in "http://localhost:8080/tumblebug" "https://localhost:8443/tumblebug" "http://localhost:1323/tumblebug"; do
    if curl -sf -k -u "${API_USER}:${API_PASS}" --max-time 2 "${cand}/readyz" >/dev/null 2>&1; then
      printf '%s' "$cand"
      return 0
    fi
  done
  return 1
}

discover_remote_vms() {
  local tb="$1"
  [ -n "$tb" ] || return 0
  LOCAL_TB="$tb" API_U="$API_USER" API_P="$API_PASS" python3 - <<'PY' 2>/dev/null || true
import json, os, subprocess
tb = os.environ["LOCAL_TB"]; u = os.environ["API_U"]; p = os.environ["API_P"]
def get_json(path):
    try:
        return json.loads(subprocess.check_output(["curl","-sS","-k","-u",f"{u}:{p}","--max-time","4",tb+path], stderr=subprocess.DEVNULL).decode())
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
            st = (v.get("status") or "").lower()
            if ip and ("run" in st or st == ""):
                nid = v.get("id") or ""
                usr = v.get("nodeUserName") or v.get("vmUserName") or "cb-user"
                prt = v.get("sshPort") or 22
                skey = v.get("sshKeyId") or ""
                print(f"{ns}\t{iid}\t{nid}\t{ip}\t{usr}\t{prt}\t{skey}")
PY
}

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

# Determine whether we should show the interactive wizard:
# Show when running on a TTY and the user did NOT pass explicit scope flags (ONLY / PROVIDERS / ARGS / YES).
HAS_EXPLICIT_SCOPE=0
if [ -n "$ONLY_IN" ] || [ -n "$PROVIDERS_IN" ] || [ -n "$RAW_ARGS" ] || [ -n "$YES_IN" ]; then
  HAS_EXPLICIT_SCOPE=1
fi

WIZARD_EXTRA_ARGS=()

if [ -t 0 ] && [ "$HAS_EXPLICIT_SCOPE" = "0" ]; then
  printf '\n%b\n' "${BD}${CY}▌ CB-Tumblebug Initialization Wizard${RS}"

  # Step 1: Target selection (only ask if HOST wasn't already passed)
  if [ -z "$TARGET_IN" ]; then
    LOCAL_TB_URL="$(detect_local_tb || true)"
    mapfile -t VM_ROWS < <(discover_remote_vms "$LOCAL_TB_URL")
    while IFS= read -r cfg_row; do
      [ -n "$cfg_row" ] || continue
      IFS=$'\t' read -r _ cfg_alias _ cfg_ip _ _ _ <<< "$cfg_row"
      dup=0
      for existing in "${VM_ROWS[@]}"; do
        IFS=$'\t' read -r _ e_inf e_nid e_ip _ _ _ <<< "$existing"
        if [ "$cfg_alias" = "$e_inf" ] || [ "$cfg_alias" = "$e_nid" ] || [ "$cfg_ip" = "$e_ip" ]; then
          dup=1; break
        fi
      done
      [ "$dup" = "0" ] && VM_ROWS+=("$cfg_row")
    done < <(list_ssh_config_cb_hosts)

    local_label="Local Kubernetes cluster (this machine)"
    [ "$MODE" = "compose" ] && local_label="Local Docker Compose (this machine)"

    printf '\n  %b\n' "${BD}1) Select target environment:${RS}"
    printf "     ${CY}1)${RS} %s ${DM}[default]${RS}\n" "$local_label"
    idx=2
    for row in "${VM_ROWS[@]}"; do
      IFS=$'\t' read -r r_ns r_inf r_nid r_ip r_usr r_prt _ <<< "$row"
      if [ "$r_ns" = "ssh-config" ]; then
        printf "     ${CY}%d)${RS} Remote VM: ${BD}%s${RS} ${DM}(~/.ssh/config · %s@%s)${RS}\n" "$idx" "$r_inf" "$r_usr" "$r_ip"
      else
        printf "     ${CY}%d)${RS} Remote VM: ${BD}%s/%s${RS} ${DM}(%s · %s@%s)${RS}\n" "$idx" "$r_ns" "$r_inf" "$r_nid" "$r_usr" "$r_ip"
      fi
      idx=$((idx + 1))
    done
    manual_idx=$idx
    printf "     ${CY}%d)${RS} Enter remote host / IP manually\n" "$manual_idx"

    t_sel="$(ask "  Target #" "1")"
    if [ "$t_sel" = "1" ] || [ -z "$t_sel" ]; then
      TARGET_IN=""
    elif [ "$t_sel" = "$manual_idx" ]; then
      TARGET_IN="$(ask "  Remote host / SSH alias / IP" "")"
      [ -n "$TARGET_IN" ] || { printf '%b\n' "${RD}✖ Cancelled.${RS}"; exit 1; }
    elif [[ "$t_sel" =~ ^[0-9]+$ ]] && [ "$t_sel" -ge 2 ] && [ "$t_sel" -lt "$manual_idx" ]; then
      picked="${VM_ROWS[$((t_sel - 2))]}"
      IFS=$'\t' read -r r_ns r_inf r_nid r_ip r_usr r_prt _ <<< "$picked"
      if [ "$r_ns" = "ssh-config" ] || [ "$r_ns" = "default" ]; then
        TARGET_IN="$r_inf"
      else
        TARGET_IN="$r_ip"
      fi
    else
      TARGET_IN="$t_sel"
    fi
  fi

  # Step 2: Initialization scope selection
  printf '\n  %b\n' "${BD}2) Select initialization scope:${RS}"
  printf "     ${CY}1)${RS} Full Init (Recommended)  ${DM}— Credentials + Backup DB (~1m) + Templates [default]${RS}\n"
  printf "     ${CY}2)${RS} Credentials only         ${DM}— Register CSP credentials to Tumblebug & OpenBao (~5s)${RS}\n"
  printf "     ${CY}3)${RS} Backup DB + Templates    ${DM}— Restore specs/images/pricing & templates (no password)${RS}\n"
  printf "     ${CY}4)${RS} Templates only           ${DM}— Reload init/templates/*.json (~3s)${RS}\n"
  printf "     ${CY}5)${RS} Fresh CSP Fetch          ${DM}— Fetch live Specs/Images directly from CSP APIs${RS}\n"

  s_sel="$(ask "  Scope #" "1")"
  case "$s_sel" in
    1|"")
      ONLY_IN="all"
      YES_IN="1"
      ;;
    2)
      ONLY_IN="cred"
      YES_IN="1"
      ;;
    3)
      ONLY_IN="assets"
      YES_IN="1"
      ;;
    4)
      ONLY_IN="templates"
      YES_IN="1"
      ;;
    5)
      printf '\n  %b\n' "${BD}   ↳ Which CSPs to fetch live Specs/Images from?${RS}"
      printf "     ${CY}1)${RS} Selected CSPs (e.g. aws,gcp) ${DM}[fast, ~minutes]${RS}\n"
      printf "     ${CY}2)${RS} All registered CSPs except Azure ${DM}[~15-20m]${RS}\n"
      printf "     ${CY}3)${RS}Interactive full init.py menu (all options incl. a+ CSV patch / Azure)\n"
      f_sel="$(ask "   Choice #" "1")"
      case "$f_sel" in
        1)
          PROVIDERS_IN="$(ask "   Provider names (comma-separated, e.g. aws,alibaba)" "")"
          [ -n "$PROVIDERS_IN" ] || { printf '%b\n' "${RD}✖ Provider name required.${RS}"; exit 1; }
          ONLY_IN="fetch"
          YES_IN="1"
          ;;
        2)
          # Let init.py run credential + fresh load; pass interactive or --load-assets without backup
          ONLY_IN="interactive"
          ;;
        *)
          ONLY_IN="interactive"
          ;;
      esac
      ;;
    *)
      printf '%b\n' "${RD}✖ Invalid selection: ${s_sel}${RS}"
      exit 1
      ;;
  esac

  # Print clean automation tip
  make_verb="k-init"
  [ "$MODE" = "compose" ] && make_verb="init"
  tip_cmd="make ${make_verb}"
  [ -n "$TARGET_IN" ] && tip_cmd+=" HOST=${TARGET_IN}"
  [ -n "$ONLY_IN" ] && [ "$ONLY_IN" != "all" ] && [ "$ONLY_IN" != "interactive" ] && tip_cmd+=" ONLY=${ONLY_IN}"
  [ -n "$PROVIDERS_IN" ] && tip_cmd+=" PROVIDERS=${PROVIDERS_IN}"
  [ "${YES_IN:-}" = "1" ] && tip_cmd+=" YES=1"
  printf '\n  %b\n\n' "${DM}💡 Direct non-interactive command: ${RS}${CY}${tip_cmd}${RS}"
fi

# Build final arguments for multi-init.sh / init.py
FINAL_ARGS=()
if [ "${YES_IN:-}" = "1" ] || [ "${YES_IN:-}" = "true" ] || [ "${YES_IN:-}" = "yes" ]; then
  FINAL_ARGS+=("-y")
fi

case "${ONLY_IN:-}" in
  ""|all|full|interactive)
    ;;
  cred|creds|credentials|credential)
    FINAL_ARGS+=("--credentials-only")
    ;;
  assets|db|backup)
    FINAL_ARGS+=("--load-assets-only" "--load-templates-only")
    ;;
  assets-only)
    FINAL_ARGS+=("--load-assets-only")
    ;;
  templates|template|tpl)
    FINAL_ARGS+=("--load-templates-only")
    ;;
  price|pricing)
    FINAL_ARGS+=("--fetch-price-only")
    ;;
  fetch)
    ;;
  *)
    printf '%b\n' "${RD}✖ Unknown ONLY='${ONLY_IN}' (expected: cred | assets | templates | price | all)${RS}"
    exit 1
    ;;
esac

if [ -n "$PROVIDERS_IN" ]; then
  FINAL_ARGS+=("--providers" "$PROVIDERS_IN")
fi

if [ "${#WIZARD_EXTRA_ARGS[@]}" -gt 0 ]; then
  FINAL_ARGS+=("${WIZARD_EXTRA_ARGS[@]}")
fi

if [ -n "$RAW_ARGS" ]; then
  # Append raw ARGS tokens while preserving user spacing
  read -r -a raw_arr <<< "$RAW_ARGS"
  FINAL_ARGS+=("${raw_arr[@]}")
fi

# Dispatch to Remote (r-init) vs Local K8s vs Local Compose
if [ -n "$TARGET_IN" ]; then
  exec make --no-print-directory r-init \
    HOST="$TARGET_IN" \
    SSH_USER="$SSH_USER_IN" \
    SSH_KEY="$SSH_KEY_IN" \
    SSH_PORT="$SSH_PORT_IN" \
    K8S_NAMESPACE="$K8S_NAMESPACE" \
    K8S_CONTEXT="$K8S_CONTEXT" \
    K8S_INIT_PORT="$K8S_INIT_PORT" \
    ARGS="${FINAL_ARGS[*]}"
fi

if [ "$MODE" = "k8s" ]; then
  pf_port="$K8S_INIT_PORT"
  while ss -tlnH 2>/dev/null | awk '{print $4}' | grep -qE "[:.]${pf_port}$"; do
    pf_port=$((pf_port + 1))
  done
  printf '%b\n' "${DM}Port-forwarding cb-tumblebug (${pf_port} -> 1323)...${RS}"
  $KUBECTL_CMD port-forward -n "$K8S_NAMESPACE" svc/cb-tumblebug "${pf_port}:1323" >/dev/null 2>&1 &
  PF_PID=$!
  trap 'kill "$PF_PID" 2>/dev/null || true' EXIT INT TERM
  sleep 2
  chmod +x ./init/multi-init.sh 2>/dev/null || true
  TUMBLEBUG_SERVER="localhost:${pf_port}" \
  ASSETS_PG_BACKEND="kubectl" \
  TB_K8S_NAMESPACE="$K8S_NAMESPACE" \
  ./init/multi-init.sh "${FINAL_ARGS[@]}"
else
  chmod +x ./init/multi-init.sh 2>/dev/null || true
  ./init/multi-init.sh "${FINAL_ARGS[@]}"
fi
