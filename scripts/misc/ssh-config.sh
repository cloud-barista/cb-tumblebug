#!/usr/bin/env bash
#
# Export or import password-encrypted OpenSSH configuration (`~/.ssh/config` + `.pem` key)
# for any VM node provisioned by CB-Tumblebug (`make ssh-config`).
#
# Usage:
#   make ssh-config                 # Interactive: 1) Export  2) Import (auto-imports if no local TB API)
#   make ssh-config HOST=tb         # Direct export for Infra/Node `tb`
#   make ssh-config IMPORT=1        # Direct import on another machine (or FILE=<path>)
#   make ssh-config REMOVE=tb       # Remove managed Host block & .pem from ~/.ssh/

set -euo pipefail

K8S_NAMESPACE="${K8S_NAMESPACE:-cb-tumblebug}"
K8S_CONTEXT="${K8S_CONTEXT:-}"
KUBECTL=(kubectl)
[ -n "$K8S_CONTEXT" ] && KUBECTL+=(--context "$K8S_CONTEXT")

NS_ARG="${NS:-}"
INFRA_ARG="${INFRA:-}"
NODE_ARG="${NODE:-}"
HOST_ARG="${REMOTE_HOST:-${HOST:-${IP:-}}}"
ALIAS_ARG="${ALIAS:-}"
USER_ARG="${USER_OVERRIDE:-${SSH_USER:-}}"
IMPORT_ARG="${IMPORT:-}"
FILE_ARG="${FILE:-}"
REMOVE_ARG="${REMOVE:-}"
OUT_ARG="${OUT:-}"
HOLDER_ARG="${HOLDER:-}"
PASS_ARG="${PASS:-${SSH_BUNDLE_PASS:-}}"

if [ -t 2 ] && [ -z "${NO_COLOR:-}" ]; then
  CY=$'\033[36m'; GN=$'\033[32m'; YL=$'\033[33m'; RD=$'\033[31m'; DM=$'\033[2m'; BD=$'\033[1m'; RS=$'\033[0m'
else
  CY=""; GN=""; YL=""; RD=""; DM=""; BD=""; RS=""
fi

sanitize_alias() {
  printf '%s' "$1" | tr -cs 'A-Za-z0-9._-' '-' | sed 's/^-*//; s/-*$//'
}

# --- Handle REMOVE=<alias> immediately (no API needed) ------------------------
if [ -n "$REMOVE_ARG" ]; then
  REM_ALIAS="$(sanitize_alias "$REMOVE_ARG")"
  if [ -f "$HOME/.ssh/config" ]; then
    awk -v m1="# >>> cb-tumblebug:${REM_ALIAS} >>>" -v m2="# <<< cb-tumblebug:${REM_ALIAS} <<<" '
      $0 == m1 { skip=1; next }
      $0 == m2 { skip=0; next }
      !skip
    ' "$HOME/.ssh/config" > "$HOME/.ssh/config.cb-tmp"
    mv "$HOME/.ssh/config.cb-tmp" "$HOME/.ssh/config"
    chmod 600 "$HOME/.ssh/config" 2>/dev/null || true
  fi
  rm -f "$HOME/.ssh/cb-tb-${REM_ALIAS}.pem" "$HOME/.ssh/cb-tb-${REM_ALIAS}-bastion.pem"
  printf '%b\n' "${GN}✔ Removed SSH host '${REM_ALIAS}' and its managed key files from ~/.ssh/${RS}" >&2
  exit 0
fi

TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/.cb-ssh-cfg.XXXXXX")"
chmod 700 "$TMP_DIR"
PF_PID=""

cleanup() {
  trap - EXIT INT TERM
  unset PASS_ARG CB_PW 2>/dev/null || true
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

# Apply validated JSON payload into ~/.ssh/config & ~/.ssh/cb-tb-<alias>.pem
apply_json_payload() {
  local json_file="$1" alias_override="${2:-}"
  JSON_FILE="$json_file" ALIAS_OVR="$alias_override" python3 - <<'PY'
import json, os, re

with open(os.environ["JSON_FILE"], "r", encoding="utf-8") as f:
    d = json.load(f)

if d.get("magic") != "CB-TB-SSH-V1":
    raise SystemExit("Invalid payload magic")

def clean_token(val, fallback=""):
    s = str(val or fallback).strip()
    if any(ch in s for ch in ("\n", "\r", "\t", " ")):
        raise SystemExit(f"Invalid whitespace in field: {s!r}")
    return s

raw_alias = os.environ.get("ALIAS_OVR") or d.get("alias") or "cb-tb-vm"
alias = re.sub(r"[^A-Za-z0-9._-]+", "-", raw_alias).strip("-") or "cb-tb-vm"
host = clean_token(d.get("host"))
user = clean_token(d.get("user"), "cb-user")
port = str(int(d.get("port") or 22))
pem = d.get("pem") or ""
bastion = d.get("bastion")

ssh_dir = os.path.expanduser("~/.ssh")
os.makedirs(ssh_dir, mode=0o700, exist_ok=True)
os.chmod(ssh_dir, 0o700)

def write_pem(path, content):
    text = content if content.endswith("\n") else content + "\n"
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as pf:
        pf.write(text)
    os.chmod(path, 0o600)

if pem:
    write_pem(os.path.join(ssh_dir, f"cb-tb-{alias}.pem"), pem)
if bastion and bastion.get("pem"):
    write_pem(os.path.join(ssh_dir, f"cb-tb-{alias}-bastion.pem"), bastion["pem"])

m_start = f"# >>> cb-tumblebug:{alias} >>>"
m_end   = f"# <<< cb-tumblebug:{alias} <<<"

lines = ["", m_start]
if bastion:
    b_host = clean_token(bastion.get("host"))
    b_user = clean_token(bastion.get("user"), "cb-user")
    b_port = str(int(bastion.get("port") or 22))
    lines += [
        f"Host {alias}-bastion",
        f"    HostName {b_host}",
        f"    User {b_user}",
        f"    Port {b_port}",
    ]
    if bastion.get("pem"):
        lines += [
            f"    IdentityFile ~/.ssh/cb-tb-{alias}-bastion.pem",
            "    IdentitiesOnly yes",
        ]
    lines += [
        "    StrictHostKeyChecking no",
        "    UserKnownHostsFile /dev/null",
        "    LogLevel ERROR",
        "",
    ]

lines += [
    f"Host {alias}",
    f"    HostName {host}",
    f"    User {user}",
    f"    Port {port}",
]
if bastion:
    lines.append(f"    ProxyJump {alias}-bastion")
if pem:
    lines += [
        f"    IdentityFile ~/.ssh/cb-tb-{alias}.pem",
        "    IdentitiesOnly yes",
    ]
lines += [
    "    ServerAliveInterval 30",
    "    ServerAliveCountMax 3",
    "    StrictHostKeyChecking no",
    "    UserKnownHostsFile /dev/null",
    "    LogLevel ERROR",
    m_end,
    "",
]

cfg_path = os.path.join(ssh_dir, "config")
existing = ""
if os.path.exists(cfg_path):
    with open(cfg_path, "r", encoding="utf-8", errors="replace") as cf:
        existing = cf.read()

out_lines = []
skip = False
for line in existing.splitlines():
    if line.strip() == m_start:
        skip = True
        continue
    if line.strip() == m_end:
        skip = False
        continue
    if not skip:
        out_lines.append(line)

while out_lines and out_lines[-1] == "":
    out_lines.pop()

final_cfg = ("\n".join(out_lines) + "\n" if out_lines else "") + "\n".join(lines).lstrip("\n")
tmp_cfg = cfg_path + ".cb-tmp"
fd = os.open(tmp_cfg, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
with os.fdopen(fd, "w", encoding="utf-8") as tf:
    tf.write(final_cfg)
os.replace(tmp_cfg, cfg_path)
os.chmod(cfg_path, 0o600)

print(f"{alias}\t{user}\t{host}\t{port}")
PY
}

# --- IMPORT MODE --------------------------------------------------------------
run_import_mode() {
  command -v openssl >/dev/null 2>&1 || { printf '%b\n' "${RD}✖ openssl is required for decryption.${RS}" >&2; exit 1; }
  printf '%b\n' "${BD}${CY}▌ Import Encrypted SSH Config${RS}" >&2

  local raw_b64=""
  if [ -n "$FILE_ARG" ]; then
    local fpath="${FILE_ARG/#\~/$HOME}"
    [ -f "$fpath" ] || { printf '%b\n' "${RD}✖ File not found: ${fpath}${RS}" >&2; exit 1; }
    raw_b64="$(grep -v '^-----' "$fpath" | tr -cd 'A-Za-z0-9+/=')"
  elif ! [ -t 0 ]; then
    raw_b64="$(grep -v '^-----' | tr -cd 'A-Za-z0-9+/=')"
  else
    printf '  %b\n' "Paste the encrypted code and press Enter:" >&2
    raw_b64="$(
      python3 - <<'PY'
import os, re, select, sys, termios

fd = os.open("/dev/tty", os.O_RDWR)
old = termios.tcgetattr(fd)
new = termios.tcgetattr(fd)
new[3] &= ~termios.ICANON
new[6][termios.VMIN] = 1
new[6][termios.VTIME] = 0

def strip_b64(raw: bytes) -> str:
    txt = raw.decode("utf-8", errors="ignore")
    # Strip ANSI/CSI control sequences (e.g., terminal bracketed paste \x1b[200~ ... \x1b[201~)
    txt = re.sub(r"\x1b\[[0-9;?]*[ -/]*[@-~]", "", txt)
    lines = [l for l in txt.splitlines() if not l.strip().startswith("-----")]
    return re.sub(r"[^A-Za-z0-9+/=]", "", "".join(lines))

buf = b""
try:
    termios.tcsetattr(fd, termios.TCSADRAIN, new)
    while True:
        chunk = os.read(fd, 4096)
        if not chunk:
            break
        if b"\x04" in chunk:
            buf += chunk.split(b"\x04", 1)[0]
            break
        buf += chunk
        cleaned = strip_b64(buf)
        if not cleaned:
            buf = b""
            continue
        if buf.endswith((b"\n", b"\r")):
            r, _, _ = select.select([fd], [], [], 0.12)
            if not r:
                break
finally:
    termios.tcsetattr(fd, termios.TCSADRAIN, old)
    os.close(fd)

if buf and not buf.endswith((b"\n", b"\r")):
    sys.stderr.write("\n")
sys.stdout.write(strip_b64(buf))
PY
    )"
  fi

  if [ -z "$raw_b64" ]; then
    printf '%b\n' "${RD}✖ No encrypted code provided.${RS}" >&2
    exit 1
  fi

  printf '%s' "$raw_b64" > "${TMP_DIR}/cipher.b64"
  if ! (base64 -d < "${TMP_DIR}/cipher.b64" > "${TMP_DIR}/cipher.bin" 2>/dev/null || base64 -D < "${TMP_DIR}/cipher.b64" > "${TMP_DIR}/cipher.bin" 2>/dev/null) || [ ! -s "${TMP_DIR}/cipher.bin" ]; then
    printf '%b\n' "${RD}✖ Invalid encrypted code format — check that the full text was copied.${RS}" >&2
    exit 1
  fi

  local unlocked_info=""
  if [ -n "$PASS_ARG" ]; then
    if CB_PW="$PASS_ARG" openssl enc -d -aes-256-cbc -pbkdf2 -iter 100000 -md sha256 -pass env:CB_PW -in "${TMP_DIR}/cipher.bin" 2>/dev/null \
      | gzip -dc > "${TMP_DIR}/payload.json" 2>/dev/null \
      && unlocked_info="$(apply_json_payload "${TMP_DIR}/payload.json" "$ALIAS_ARG" 2>/dev/null)"; then
      unset PASS_ARG
    else
      unset PASS_ARG
      printf '%b\n' "${RD}✖ Wrong password or corrupted code — SSH config was NOT restored.${RS}" >&2
      exit 1
    fi
  else
    [ -r /dev/tty ] || { printf '%b\n' "${RD}✖ Password required: run interactively or pass PASS='<password>'.${RS}" >&2; exit 1; }
    local attempt=1
    while [ "$attempt" -le 3 ]; do
      printf "  🔐 Enter password to unlock: " >/dev/tty
      IFS= read -rs pw_in </dev/tty || exit 1
      echo "" >/dev/tty
      [ -n "$pw_in" ] || continue
      if CB_PW="$pw_in" openssl enc -d -aes-256-cbc -pbkdf2 -iter 100000 -md sha256 -pass env:CB_PW -in "${TMP_DIR}/cipher.bin" 2>/dev/null \
        | gzip -dc > "${TMP_DIR}/payload.json" 2>/dev/null \
        && unlocked_info="$(apply_json_payload "${TMP_DIR}/payload.json" "$ALIAS_ARG" 2>/dev/null)"; then
        unset pw_in
        break
      fi
      unset pw_in
      rm -f "${TMP_DIR}/payload.json"
      printf '%b\n' "  ${RD}✖ Wrong password (${attempt}/3)${RS}" >/dev/tty
      attempt=$((attempt + 1))
    done
    if [ -z "$unlocked_info" ]; then
      printf '%b\n' "${RD}✖ Aborted: password did not match — SSH config was NOT restored.${RS}" >&2
      exit 1
    fi
  fi

  IFS=$'\t' read -r imp_alias imp_user imp_host imp_port <<< "$unlocked_info"
  printf '\n%b\n' "${GN}✔ Unlocked & configured SSH host '${imp_alias}' (${imp_user}@${imp_host}:${imp_port}) in ~/.ssh/config${RS}" >&2
  printf '\n%b\n' "${BD}Ready to use on THIS machine:${RS}" >&2
  printf "  ${CY}ssh ${imp_alias}${RS}                     ${DM}# Direct SSH or VS Code / Antigravity Remote-SSH${RS}\n" >&2
  printf "  ${CY}make k-tunnel HOST=${imp_alias}${RS}      ${DM}# Open browser / API / MCP tunnel (:18080 / :18443)${RS}\n" >&2
  printf "  ${CY}make k-init   HOST=${imp_alias}${RS}      ${DM}# Push credentials / templates / assets remotely${RS}\n" >&2
  printf "  ${CY}make k-up     HOST=${imp_alias}${RS}      ${DM}# Upgrade remote K8s stack${RS}\n" >&2
  printf "  ${CY}make ssh-config REMOVE=${imp_alias}${RS}  ${DM}# Remove when no longer needed${RS}\n" >&2
  exit 0
}

if [ "$IMPORT_ARG" = "1" ] || [ -n "$FILE_ARG" ]; then
  run_import_mode
fi

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

if [ -z "$TB_BASE" ] && [ -f "${TMPDIR:-/tmp}/cb-ssh-tunnel/links.txt" ]; then
  TUN_API="$(grep -Eo 'https?://localhost:[0-9]+/tumblebug' "${TMPDIR:-/tmp}/cb-ssh-tunnel/links.txt" 2>/dev/null | head -1 || true)"
  if [ -n "$TUN_API" ] && probe_tb "$TUN_API"; then
    TB_BASE="$TUN_API"
  fi
fi

if [ -z "$TB_BASE" ] && command -v kubectl >/dev/null 2>&1; then
  if "${KUBECTL[@]}" get svc -n "$K8S_NAMESPACE" cb-tumblebug >/dev/null 2>&1; then
    disc_port=13523
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

# On a machine without local CB-Tumblebug, `make ssh-config` automatically enters Import mode!
if [ -z "$TB_BASE" ]; then
  if [ -t 0 ] && [ -z "$HOST_ARG" ] && [ -z "$INFRA_ARG" ] && [ -z "$NODE_ARG" ]; then
    run_import_mode
  fi
  printf '%b\n' "${RD}✖ Cannot reach CB-Tumblebug API.${RS} Run ${CY}make ssh-config${RS} without HOST to import an encrypted code." >&2
  exit 1
fi

printf '%b\n' "${BD}${CY}▌ CB-Tumblebug SSH Config${RS} ${DM}(API: ${TB_BASE})${RS}" >&2

if [ -t 0 ] && [ -t 1 ] && [ -z "$HOST_ARG" ] && [ -z "$INFRA_ARG" ] && [ -z "$NODE_ARG" ] && [ -z "$OUT_ARG" ]; then
  printf '\n  %b\n' "${BD}Select action:${RS}" >&2
  printf "    ${CY}1)${RS} Export encrypted SSH config ${DM}(for another machine) [default]${RS}\n" >&2
  printf "    ${CY}2)${RS} Import encrypted SSH config ${DM}(onto this machine)${RS}\n" >&2
  WIZ_MODE="$(ask "  Action #" "1")"
  if [ "$WIZ_MODE" = "2" ]; then
    echo "" >&2
    run_import_mode
  fi
fi

# --- 2) Resolve Namespace / Infra / Node -------------------------------------
if { [ -n "$HOST_ARG" ] || [ -n "$NODE_ARG" ]; } && { [ -z "$NS_ARG" ] || [ -z "$INFRA_ARG" ]; }; then
  MATCH_LINE="$(
    TB_BASE="$TB_BASE" NS_ARG="$NS_ARG" INFRA_ARG="$INFRA_ARG" NODE_ARG="$NODE_ARG" HOST_ARG="$HOST_ARG" \
    API_USER="$API_USER" API_PASS="$API_PASS" HOLDER_ARG="$HOLDER_ARG" python3 - <<'PY'
import json, os, subprocess, sys

base = os.environ["TB_BASE"]
user = os.environ["API_USER"]
pw = os.environ["API_PASS"]
holder = os.environ.get("HOLDER_ARG", "")
ns_filter = os.environ.get("NS_ARG", "")
infra_filter = os.environ.get("INFRA_ARG", "")
node_q = os.environ.get("NODE_ARG", "")
host_q = os.environ.get("HOST_ARG", "")

def get_json(path):
    cmd = ["curl", "-sS", "-k", "-u", f"{user}:{pw}", "--max-time", "8"]
    if holder:
        cmd += ["-H", f"X-Credential-Holder: {holder}"]
    cmd.append(base + path)
    try:
        return json.loads(subprocess.check_output(cmd, stderr=subprocess.DEVNULL).decode("utf-8"))
    except Exception:
        return {}

ns_list = [ns_filter] if ns_filter else [x.get("id") for x in (get_json("/ns").get("ns") or []) if x.get("id")]
if "default" in ns_list:
    ns_list.remove("default")
    ns_list.insert(0, "default")

for ns in ns_list:
    infras = [infra_filter] if infra_filter else (get_json(f"/ns/{ns}/infra?option=id").get("output") or [])
    if host_q and not node_q and host_q in infras:
        d = get_json(f"/ns/{ns}/infra/{host_q}")
        nodes = d.get("node") or d.get("vm") or []
        if len(nodes) == 1:
            print(f"{ns}\t{host_q}\t{nodes[0].get('id','')}")
            sys.exit(0)
        elif len(nodes) > 1:
            print(f"{ns}\t{host_q}\t")
            sys.exit(0)
    for inf in infras:
        d = get_json(f"/ns/{ns}/infra/{inf}")
        for nd in (d.get("node") or d.get("vm") or []):
            nid = nd.get("id") or ""
            pub = nd.get("publicIP") or nd.get("publicIp") or ""
            priv = nd.get("privateIP") or nd.get("privateIp") or ""
            if (node_q and nid == node_q) or (host_q and host_q in (nid, pub, priv)):
                print(f"{ns}\t{inf}\t{nid}")
                sys.exit(0)
PY
  )"
  if [ -n "$MATCH_LINE" ]; then
    IFS=$'\t' read -r NS_ARG INFRA_ARG MATCHED_NODE <<< "$MATCH_LINE"
    [ -z "$NODE_ARG" ] && [ -n "${MATCHED_NODE:-}" ] && NODE_ARG="$MATCHED_NODE"
  else
    printf '%b\n' "${RD}✖ Could not find Infra/Node matching '${HOST_ARG:-$NODE_ARG}' in CB-Tumblebug.${RS}" >&2
    exit 1
  fi
fi

if [ -z "$NS_ARG" ]; then
  mapfile -t NS_ITEMS < <(
    tb_get "/ns" | python3 -c '
import json, sys
try:
    ns = [x.get("id") for x in (json.load(sys.stdin).get("ns") or []) if x.get("id")]
    if "default" in ns:
        ns.remove("default"); ns.insert(0, "default")
    print("\n".join(ns))
except Exception:
    pass
'
  )
  NON_EMPTY_NS=()
  for n in "${NS_ITEMS[@]}"; do
    cnt="$(tb_get "/ns/${n}/infra?option=id" | python3 -c 'import json,sys;print(len(json.load(sys.stdin).get("output") or []))' 2>/dev/null || echo 0)"
    [ "${cnt:-0}" -gt 0 ] && NON_EMPTY_NS+=("${n}:${cnt}")
  done
  if [ "${#NON_EMPTY_NS[@]}" -eq 0 ]; then
    printf '%b\n' "${RD}✖ No Infras found in any namespace.${RS}" >&2
    exit 1
  elif [ "${#NON_EMPTY_NS[@]}" -eq 1 ]; then
    NS_ARG="${NON_EMPTY_NS[0]%%:*}"
  else
    printf '\n  %b\n' "${BD}Namespaces with active Infras:${RS}" >&2
    idx=1
    for entry in "${NON_EMPTY_NS[@]}"; do
      printf "    ${CY}%d)${RS} %-18s ${DM}(%s infra(s))${RS}\n" "$idx" "${entry%%:*}" "${entry##*:}" >&2
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

if [ -z "$INFRA_ARG" ]; then
  mapfile -t INFRA_IDS < <(
    tb_get "/ns/${NS_ARG}/infra?option=id" | python3 -c 'import json,sys;print("\n".join(json.load(sys.stdin).get("output") or []))'
  )
  if [ "${#INFRA_IDS[@]}" -eq 0 ]; then
    printf '%b\n' "${RD}✖ No Infras found in namespace '${NS_ARG}'.${RS}" >&2
    exit 1
  elif [ "${#INFRA_IDS[@]}" -eq 1 ]; then
    INFRA_ARG="${INFRA_IDS[0]}"
  else
    printf '\n  %b\n' "${BD}Infras in namespace '${NS_ARG}':${RS}" >&2
    idx=1
    for inf in "${INFRA_IDS[@]}"; do
      printf "    ${CY}%d)${RS} %s\n" "$idx" "$inf" >&2
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
  printf '%b\n' "${RD}✖ No nodes found in '${NS_ARG}/${INFRA_ARG}'.${RS}" >&2
  exit 1
fi

SELECTED_ROW=""
if [ -n "$NODE_ARG" ]; then
  for r in "${NODE_ROWS[@]}"; do
    [ "${r%%$'\t'*}" = "$NODE_ARG" ] && SELECTED_ROW="$r" && break
  done
  if [ -z "$SELECTED_ROW" ]; then
    printf '%b\n' "${RD}✖ Node '${NODE_ARG}' not found in '${NS_ARG}/${INFRA_ARG}'.${RS}" >&2
    exit 1
  fi
elif [ "${#NODE_ROWS[@]}" -eq 1 ]; then
  SELECTED_ROW="${NODE_ROWS[0]}"
elif [ -t 0 ]; then
  printf '\n  %b\n' "${BD}Nodes in ${NS_ARG}/${INFRA_ARG}:${RS}" >&2
  printf "  ${DM}%-4s %-20s %-14s %-18s %-16s %-15s %-10s${RS}\n" "#" "NODE ID" "STATUS" "CSP/REGION" "PUBLIC IP" "PRIVATE IP" "USER:PORT" >&2
  idx=1
  for r in "${NODE_ROWS[@]}"; do
    IFS=$'\t' read -r nid _ st pub prv usr prt _ cspreg <<< "$r"
    printf "  ${CY}%-4s${RS} %-20s %-14s %-18s %-16s %-15s %-10s\n" "${idx})" "$nid" "$st" "$cspreg" "$pub" "$prv" "${usr}:${prt}" >&2
    idx=$((idx + 1))
  done
  sel="$(ask "  Select node (# or ID)" "1")"
  if [[ "$sel" =~ ^[0-9]+$ ]] && [ "$sel" -ge 1 ] && [ "$sel" -le "${#NODE_ROWS[@]}" ]; then
    SELECTED_ROW="${NODE_ROWS[$((sel - 1))]}"
  else
    for r in "${NODE_ROWS[@]}"; do
      [ "${r%%$'\t'*}" = "$sel" ] && SELECTED_ROW="$r" && break
    done
  fi
  [ -n "$SELECTED_ROW" ] || { printf '%b\n' "${RD}✖ Invalid node selection.${RS}" >&2; exit 1; }
else
  SELECTED_ROW="${NODE_ROWS[0]}"
fi

IFS=$'\t' read -r TARGET_NODE TARGET_GRP TARGET_STATUS TARGET_PUB TARGET_PRIV TARGET_NODE_USER TARGET_PORT TARGET_KEY_ID TARGET_CSPREG <<< "$SELECTED_ROW"

DEFAULT_ALIAS="$INFRA_ARG"
if [ "${#NODE_ROWS[@]}" -gt 1 ]; then
  DEFAULT_ALIAS="$TARGET_NODE"
fi
ALIAS_FINAL="$(sanitize_alias "${ALIAS_ARG:-$DEFAULT_ALIAS}")"
[ -n "$ALIAS_FINAL" ] || ALIAS_FINAL="cb-tb-vm"

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
  printf '%s' "$meta" | python3 -c '
import json, sys
d = json.load(sys.stdin)
print(d.get("verifiedUsername") or d.get("username") or "")
'
}

TARGET_KEY_FILE="${TMP_DIR}/target.pem"
KEY_USER="$(fetch_ssh_key "$NS_ARG" "$TARGET_KEY_ID" "$TARGET_KEY_FILE" || true)"
if [ ! -s "$TARGET_KEY_FILE" ]; then
  printf '%b\n' "${RD}✖ Could not retrieve SSH private key '${TARGET_KEY_ID}' for node '${TARGET_NODE}'.${RS}" >&2
  exit 1
fi

LOGIN_USER="${USER_ARG:-${KEY_USER:-${TARGET_NODE_USER:-cb-user}}}"

# --- 3) Resolve Direct Public IP vs Bastion ProxyJump ------------------------
CONNECT_HOST="$TARGET_PUB"
HAS_BASTION=0
B_IP=""; B_PORT="22"; B_LOGIN_USER="cb-user"
BASTION_KEY_FILE=""

if [ -z "$CONNECT_HOST" ] || [ "$CONNECT_HOST" = "-" ]; then
  CONNECT_HOST="$TARGET_PRIV"
  if [ -z "$CONNECT_HOST" ] || [ "$CONNECT_HOST" = "-" ]; then
    printf '%b\n' "${RD}✖ Node '${TARGET_NODE}' has neither Public IP nor Private IP.${RS}" >&2
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
  [ -n "$BASTION_INFO" ] && IFS=$'\t' read -r B_NS B_INFRA B_NODE <<< "$BASTION_INFO"
  [ -z "$B_NS" ] && B_NS="$NS_ARG"
  [ -z "$B_INFRA" ] && B_INFRA="$INFRA_ARG"

  if [ -z "$B_NODE" ]; then
    for r in "${NODE_ROWS[@]}"; do
      IFS=$'\t' read -r nid _ _ pub _ _ _ _ _ <<< "$r"
      if [ -n "$pub" ] && [ "$pub" != "-" ] && [ "$nid" != "$TARGET_NODE" ]; then
        B_NODE="$nid"
        break
      fi
    done
  fi

  if [ -z "$B_NODE" ]; then
    printf '%b\n' "${RD}✖ Node '${TARGET_NODE}' only has Private IP (${CONNECT_HOST}) and no reachable Bastion was found.${RS}" >&2
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
  [ -n "${B_IP:-}" ] || { printf '%b\n' "${RD}✖ Bastion '${B_NODE}' has no Public IP.${RS}" >&2; exit 1; }
  BASTION_KEY_FILE="${TMP_DIR}/bastion.pem"
  B_KEY_USER="$(fetch_ssh_key "$B_NS" "$B_KEY_ID" "$BASTION_KEY_FILE" || true)"
  [ -s "$BASTION_KEY_FILE" ] || { printf '%b\n' "${RD}✖ Could not retrieve Bastion SSH key '${B_KEY_ID}'.${RS}" >&2; exit 1; }
  B_LOGIN_USER="${B_KEY_USER:-${B_NODE_USER:-cb-user}}"
  HAS_BASTION=1
fi

# --- 4) Build Structured JSON Payload & Encrypt ------------------------------
PAYLOAD_JSON="${TMP_DIR}/payload.json"
ALIAS_FINAL="$ALIAS_FINAL" CONNECT_HOST="$CONNECT_HOST" LOGIN_USER="$LOGIN_USER" TARGET_PORT="$TARGET_PORT" \
TARGET_KEY_FILE="$TARGET_KEY_FILE" \
HAS_BASTION="$HAS_BASTION" B_IP="$B_IP" B_LOGIN_USER="$B_LOGIN_USER" B_PORT="$B_PORT" BASTION_KEY_FILE="$BASTION_KEY_FILE" \
python3 - <<'PY' > "$PAYLOAD_JSON"
import json, os

has_bastion = os.environ["HAS_BASTION"] == "1"
obj = {
    "magic": "CB-TB-SSH-V1",
    "alias": os.environ["ALIAS_FINAL"],
    "host": os.environ["CONNECT_HOST"],
    "user": os.environ["LOGIN_USER"],
    "port": int(os.environ["TARGET_PORT"] or 22),
    "pem": open(os.environ["TARGET_KEY_FILE"], encoding="utf-8").read(),
    "bastion": None,
}
if has_bastion:
    obj["bastion"] = {
        "host": os.environ["B_IP"],
        "user": os.environ["B_LOGIN_USER"],
        "port": int(os.environ["B_PORT"] or 22),
        "pem": open(os.environ["BASTION_KEY_FILE"], encoding="utf-8").read(),
    }
print(json.dumps(obj))
PY

command -v openssl >/dev/null 2>&1 || { printf '%b\n' "${RD}✖ openssl is required for encryption.${RS}" >&2; exit 1; }

if [ -z "$PASS_ARG" ]; then
  if ! [ -r /dev/tty ]; then
    printf '%b\n' "${RD}✖ Password required in non-interactive mode: pass PASS='<your-password>'.${RS}" >&2
    exit 1
  fi
  printf '\n  %b\n' "${BD}🔐 Set password to encrypt SSH config '${ALIAS_FINAL}'${RS} ${DM}(${NS_ARG}/${INFRA_ARG}/${TARGET_NODE})${RS}" >&2
  while true; do
    printf "  Enter password (hidden): " >/dev/tty
    IFS= read -rs pw1 </dev/tty || exit 1
    echo "" >/dev/tty
    if [ -z "$pw1" ]; then
      printf '%b\n' "  ${YL}⚠ Password cannot be empty.${RS}" >&2
      continue
    fi
    printf "  Confirm password:        " >/dev/tty
    IFS= read -rs pw2 </dev/tty || exit 1
    echo "" >/dev/tty
    if [ "$pw1" != "$pw2" ]; then
      printf '%b\n' "  ${YL}⚠ Passwords do not match. Please try again.${RS}" >&2
      continue
    fi
    PASS_ARG="$pw1"
    unset pw1 pw2
    break
  done
fi

CODE_FILE="${TMP_DIR}/code.txt"
gzip -cn "$PAYLOAD_JSON" \
  | CB_PW="$PASS_ARG" openssl enc -aes-256-cbc -salt -pbkdf2 -iter 100000 -md sha256 -pass env:CB_PW \
  | base64 | tr -d ' \t\r\n' > "$CODE_FILE"
echo "" >> "$CODE_FILE"
unset PASS_ARG

if [ -n "$OUT_ARG" ]; then
  OUT_PATH="${OUT_ARG/#\~/$HOME}"
  cp "$CODE_FILE" "$OUT_PATH"
  chmod 600 "$OUT_PATH"
  printf '%b\n' "${GN}✔ Saved encrypted SSH code to:${RS} ${BD}${OUT_PATH}${RS}" >&2
  printf '%b\n' "  ${DM}On the other machine, run:${RS} ${CY}make ssh-config FILE=${OUT_PATH}${RS}" >&2
else
  if [ -t 1 ]; then
    printf '\n%b\n' "${GN}✔ Encrypted SSH code ready${RS} ${DM}(alias: ${ALIAS_FINAL})${RS}" >&2
    echo ""
    cat "$CODE_FILE"
    echo ""
    printf '%b\n' "${BD}On the other machine:${RS} run ${CY}make ssh-config${RS}, select Import (auto-selected if no local TB), paste the code above, and enter your password." >&2
  else
    cat "$CODE_FILE"
  fi
fi
