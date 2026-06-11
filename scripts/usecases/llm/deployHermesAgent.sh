#!/usr/bin/env bash
# deployHermesAgent.sh
#
# All-in-one deployment script for a Hermes Agent testing environment.
#
# It can install and configure:
#   1. vLLM with an OpenAI-compatible API server
#   2. Hermes Agent configured to use that vLLM endpoint
#   3. Hermes Gateway systemd service
#   4. Discord gateway environment variables
#   5. native ntfy gateway settings plus helper script, memory hint, and notify-ntfy skill
#   6. Tavily web search environment variable
#   7. Hermes Dashboard behind nginx reverse proxy
#   8. systemd services so reboot recovery works
#
# Intended use:
#   curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/deployHermesAgent.sh | bash -s -- [options]
#
# Example:
#   curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/deployHermesAgent.sh | bash -s -- \
#     --run-as-user cb-user \
#     --model Qwen/Qwen3-30B-A3B-Instruct-2507-FP8 \
#     --ctx-len 65536 \
#     --discord-token "YOUR_DISCORD_BOT_TOKEN" \
#     --discord-home-channel "YOUR_CHANNEL_ID" \
#     --discord-home-channel-name "hermes-bot" \
#     --ntfy-topic "etri-son-hermes-agent" \
#     --tavily-api-key "tvly-xxxx"
#
# Notes:
#   - This script assumes Ubuntu/Debian with systemd.
#   - Discord Developer Portal privileged intents cannot be enabled by script.
#   - For production, restrict inbound ports with cloud firewall/security group.
#   - Dashboard Basic Auth is intentionally not enabled by default because it can break Hermes Dashboard WebSocket flows.

set -Eeuo pipefail

if [ -z "${BASH_VERSION:-}" ]; then
  echo "Error: This script requires bash. Use: curl -fsSL <url> | bash -s -- [options]"
  exit 1
fi

START_TS=$(date +%s)
LOG_FILE="/tmp/hermes-agent-deploy-$(date +%Y%m%d-%H%M%S).log"
STEP_NAMES=()
STEP_DURATIONS=()

exec > >(tee -a "$LOG_FILE") 2>&1

trap 'echo "[ERROR] line=$LINENO command=$BASH_COMMAND exit=$?"; echo "Log: $LOG_FILE"' ERR

# -----------------------------
# Defaults
# -----------------------------
RUN_AS_USER="${SUDO_USER:-${USER:-cb-user}}"
MODE="all"                           # all, hermes-only, vllm-only
SKIP_VLLM="false"
SKIP_HERMES="false"

VLLM_MODEL="Qwen/Qwen3-30B-A3B-Instruct-2507-FP8"
VLLM_HOST="0.0.0.0"
VLLM_PORT="8000"
CTX_LEN="65536"
HERMES_CONTEXT_LENGTH=""            # empty = auto-follow CTX_LEN (kept in sync with vLLM --max-model-len)
HERMES_MAX_TOKENS="4096"
GPU_UTIL="0.88"
MAX_BATCHED_TOKENS="4096"
TOOL_CALL_PARSER="hermes"
VLLM_API_KEY="EMPTY"
VLLM_BASE_URL=""
VLLM_VERSION="0.22.0"
VLLM_HEALTH_TIMEOUT="1800"
HF_TOKEN_VALUE=""
HF_TOKEN_FILE=""

OLLAMA_BASE_URL=""
OLLAMA_MODEL="qwen3:30b"

HERMES_DASHBOARD_HOST="127.0.0.1"
HERMES_DASHBOARD_PORT="9119"
NGINX_DASHBOARD_PORT="9120"
HERMES_API_HOST="0.0.0.0"
HERMES_API_PORT="8642"
HERMES_API_KEY=""
HERMES_MAX_TURNS="90"
PROVIDER_TIMEOUT_SECONDS="300"
PROVIDER_STALE_TIMEOUT_SECONDS="60"

DISCORD_TOKEN=""
DISCORD_HOME_CHANNEL=""
DISCORD_HOME_CHANNEL_NAME="hermes-bot"
DISCORD_ALLOWED_USERS=""
GATEWAY_ALLOW_ALL_USERS="true"

NTFY_TOPIC="etri-son-hermes-agent"
NTFY_NATIVE_ENABLED="true"
NTFY_ALLOW_INBOUND="false"
NTFY_SERVER_URL=""
NTFY_TOKEN=""
NTFY_MARKDOWN="true"
TAVILY_API_KEY=""

CREATE_NOTIFY_SKILL="true"
CREATE_CHECKLLM_SKILL="true"
ENABLE_SERVICES="true"
PUBLIC_IP=""

# -----------------------------
# Helpers
# -----------------------------
usage() {
  cat <<'EOF'
Usage:
  bash deployHermesAgent.sh [options]

Core options:
  --mode all|hermes-only|vllm-only       Default: all
  --run-as-user USER                     Default: SUDO_USER or current user
  --skip-vllm                            Skip vLLM install/service
  --skip-hermes                          Skip Hermes install/config/dashboard/gateway

vLLM options:
  --model MODEL                          Default: Qwen/Qwen3-30B-A3B-Instruct-2507-FP8
  --vllm-host HOST                       Default: 0.0.0.0
  --vllm-port PORT                       Default: 8000
  --ctx-len TOKENS                       Default: 65536
  --gpu-util FLOAT                       Default: 0.88
  --max-batched-tokens TOKENS            Default: 4096
  --tool-call-parser NAME                Default: hermes
  --no-tool-call-parser                  Do not pass vLLM tool-call-parser options
  --vllm-api-key VALUE                   Default: EMPTY
  --vllm-base-url URL                    For hermes-only or external vLLM. Example: http://13.231.180.195:8000/v1

Ollama fallback/comparison options:
  --ollama-base-url URL                  Example: http://13.126.141.84:3000/v1
  --ollama-model MODEL                   Default: qwen3:30b

Hermes options:
  --hermes-context-length TOKENS         Default: follows --ctx-len (currently 65536). Set explicitly to override.
  --hermes-max-tokens TOKENS             Default: 4096
  --hermes-api-port PORT                 Default: 8642
  --hermes-api-key KEY                   Default: generated
  --max-turns N                          Default: 90
  --provider-timeout-seconds N           Default: 300
  --provider-stale-timeout-seconds N     Default: 60

Discord options:
  --discord-token TOKEN
  --discord-home-channel CHANNEL_ID
  --discord-home-channel-name NAME       Default: hermes-bot
  --discord-allowed-users CSV
  --gateway-allow-all-users true|false   Default: true for demo convenience

ntfy and web search:
  --ntfy-topic TOPIC                     Default: etri-son-hermes-agent
  --ntfy-native true|false               Configure native Hermes ntfy gateway settings. Default: true
  --ntfy-allow-inbound true|false        Allow inbound ntfy messages to the agent. Default: false
  --ntfy-server-url URL                  Optional. Default used by Hermes is https://ntfy.sh
  --ntfy-token TOKEN                     Optional. For private or reserved ntfy topics
  --ntfy-markdown true|false             Send ntfy replies with markdown. Default: true
  --tavily-api-key KEY                   Optional. If empty, Tavily is skipped.

Dashboard options:
  --dashboard-port PORT                  Hermes local dashboard port. Default: 9119
  --nginx-port PORT                      Public nginx reverse proxy port. Default: 9120

Other:
  --public-ip IP                         Override public IP printed in summary
  --no-services                          Do not enable/start systemd services
  -h, --help

Examples:
  All-in-one on a GPU VM:
    bash deployHermesAgent.sh --discord-token xxx --discord-home-channel 123 --tavily-api-key tvly-xxx

  Hermes-only using an existing remote vLLM endpoint:
    bash deployHermesAgent.sh --mode hermes-only --vllm-base-url http://13.231.180.195:8000/v1 --skip-vllm

EOF
}

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

die() {
  echo "[FATAL] $*" >&2
  exit 1
}

require_root_or_sudo() {
  if [ "$(id -u)" -ne 0 ] && ! command -v sudo >/dev/null 2>&1; then
    die "sudo is required when not running as root."
  fi
}

as_root() {
  if [ "$(id -u)" -eq 0 ]; then
    "$@"
  else
    sudo "$@"
  fi
}

run_as_user() {
  as_root sudo -u "$RUN_AS_USER" -H bash -lc "$*"
}

start_step() {
  STEP_NAME="$1"
  STEP_TS=$(date +%s)
  echo ""
  echo "============================================================"
  log "START: $STEP_NAME"
  echo "============================================================"
}

end_step() {
  local end_ts
  end_ts=$(date +%s)
  local duration=$((end_ts - STEP_TS))
  STEP_NAMES+=("$STEP_NAME")
  STEP_DURATIONS+=("$duration")
  log "DONE: $STEP_NAME (${duration}s)"
}

run_step() {
  start_step "$1"
  shift
  "$@"
  end_step
}

wait_for_http() {
  local url="$1"
  local timeout="${2:-300}"
  local interval="${3:-5}"
  local expected="${4:-200}"
  local elapsed=0
  local code=""

  log "Waiting for HTTP $expected: $url"
  while [ "$elapsed" -lt "$timeout" ]; do
    code=$(curl -sS -o /dev/null -w "%{http_code}" --max-time 5 "$url" 2>/dev/null || true)
    if [ "$code" = "$expected" ]; then
      log "HTTP check OK: $url"
      return 0
    fi
    printf "."
    sleep "$interval"
    elapsed=$((elapsed + interval))
  done
  echo ""
  log "HTTP check failed: url=$url expected=$expected last_code=$code timeout=${timeout}s"
  return 1
}

detect_public_ip() {
  if [ -n "$PUBLIC_IP" ]; then
    echo "$PUBLIC_IP"
    return
  fi
  local ip=""
  ip=$(curl -fsS --max-time 4 https://api.ipify.org 2>/dev/null || true)
  if [ -z "$ip" ]; then
    ip=$(hostname -I 2>/dev/null | awk '{print $1}' || true)
  fi
  echo "${ip:-UNKNOWN}"
}

set_env_kv() {
  local key="$1"
  local value="$2"
  local env_file="$3"

  as_root install -d -m 700 -o "$RUN_AS_USER" -g "$RUN_AS_USER" "$(dirname "$env_file")"
  touch "$env_file"
  as_root chown "$RUN_AS_USER:$RUN_AS_USER" "$env_file"
  as_root chmod 600 "$env_file"

  KEY="$key" VALUE="$value" ENV_FILE="$env_file" python3 - <<'PY'
import os
from pathlib import Path

key = os.environ["KEY"]
value = os.environ["VALUE"]
path = Path(os.environ["ENV_FILE"])

lines = []
if path.exists():
    lines = path.read_text(encoding="utf-8", errors="ignore").splitlines()

prefix = key + "="
out = [line for line in lines if not line.startswith(prefix)]
out.append(f"{key}={value}")
path.write_text("\n".join(out) + "\n", encoding="utf-8")
PY
  as_root chown "$RUN_AS_USER:$RUN_AS_USER" "$env_file"
  as_root chmod 600 "$env_file"
}

print_secret_hint() {
  local label="$1"
  local value="$2"
  if [ -n "$value" ]; then
    echo "$label: set"
  else
    echo "$label: not set"
  fi
}

# -----------------------------
# Parse args
# -----------------------------
while [ $# -gt 0 ]; do
  case "$1" in
    --mode) MODE="${2:?}"; shift 2 ;;
    --run-as-user) RUN_AS_USER="${2:?}"; shift 2 ;;
    --skip-vllm) SKIP_VLLM="true"; shift ;;
    --skip-hermes) SKIP_HERMES="true"; shift ;;

    --model) VLLM_MODEL="${2:?}"; shift 2 ;;
    --vllm-host) VLLM_HOST="${2:?}"; shift 2 ;;
    --vllm-port) VLLM_PORT="${2:?}"; shift 2 ;;
    --ctx-len) CTX_LEN="${2:?}"; shift 2 ;;
    --gpu-util) GPU_UTIL="${2:?}"; shift 2 ;;
    --max-batched-tokens) MAX_BATCHED_TOKENS="${2:?}"; shift 2 ;;
    --tool-call-parser) TOOL_CALL_PARSER="${2:?}"; shift 2 ;;
    --no-tool-call-parser) TOOL_CALL_PARSER=""; shift ;;
    --vllm-api-key) VLLM_API_KEY="${2:?}"; shift 2 ;;
    --vllm-version) VLLM_VERSION="${2:?}"; shift 2 ;;
    --vllm-health-timeout) VLLM_HEALTH_TIMEOUT="${2:?}"; shift 2 ;;
    --hf-token) HF_TOKEN_VALUE="${2:?}"; shift 2 ;;
    --hf-token-file) HF_TOKEN_FILE="${2:?}"; shift 2 ;;
    --vllm-base-url) VLLM_BASE_URL="${2:?}"; shift 2 ;;

    --ollama-base-url) OLLAMA_BASE_URL="${2:?}"; shift 2 ;;
    --ollama-model) OLLAMA_MODEL="${2:?}"; shift 2 ;;

    --hermes-context-length) HERMES_CONTEXT_LENGTH="${2:?}"; shift 2 ;;
    --hermes-max-tokens) HERMES_MAX_TOKENS="${2:?}"; shift 2 ;;
    --hermes-api-port) HERMES_API_PORT="${2:?}"; shift 2 ;;
    --hermes-api-key) HERMES_API_KEY="${2?}"; shift 2 ;;
    --max-turns) HERMES_MAX_TURNS="${2:?}"; shift 2 ;;
    --provider-timeout-seconds) PROVIDER_TIMEOUT_SECONDS="${2:?}"; shift 2 ;;
    --provider-stale-timeout-seconds) PROVIDER_STALE_TIMEOUT_SECONDS="${2:?}"; shift 2 ;;

    --discord-token) DISCORD_TOKEN="${2:?}"; shift 2 ;;
    --discord-home-channel) DISCORD_HOME_CHANNEL="${2:?}"; shift 2 ;;
    --discord-home-channel-name) DISCORD_HOME_CHANNEL_NAME="${2:?}"; shift 2 ;;
    --discord-allowed-users) DISCORD_ALLOWED_USERS="${2:?}"; shift 2 ;;
    --gateway-allow-all-users) GATEWAY_ALLOW_ALL_USERS="${2:?}"; shift 2 ;;

    --ntfy-topic)
      NTFY_TOPIC="${2:?}"
      [[ "$NTFY_TOPIC" =~ ^[a-zA-Z0-9_-]+$ ]] || die "--ntfy-topic may only contain letters, digits, hyphens, and underscores."
      shift 2 ;;
    --ntfy-native) NTFY_NATIVE_ENABLED="${2:?}"; shift 2 ;;
    --ntfy-allow-inbound) NTFY_ALLOW_INBOUND="${2:?}"; shift 2 ;;
    --ntfy-server-url) NTFY_SERVER_URL="${2:?}"; shift 2 ;;
    --ntfy-token) NTFY_TOKEN="${2:?}"; shift 2 ;;
    --ntfy-markdown) NTFY_MARKDOWN="${2:?}"; shift 2 ;;
    --tavily-api-key) TAVILY_API_KEY="${2:?}"; shift 2 ;;

    --dashboard-port) HERMES_DASHBOARD_PORT="${2:?}"; shift 2 ;;
    --nginx-port) NGINX_DASHBOARD_PORT="${2:?}"; shift 2 ;;

    --public-ip) PUBLIC_IP="${2:?}"; shift 2 ;;
    --no-services) ENABLE_SERVICES="false"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "Unknown option: $1. Use --help." ;;
  esac
done

case "$MODE" in
  all) ;;
  hermes-only) SKIP_VLLM="true" ;;
  vllm-only) SKIP_HERMES="true" ;;
  *) die "Invalid --mode: $MODE" ;;
esac

require_root_or_sudo

if ! id "$RUN_AS_USER" >/dev/null 2>&1; then
  die "User not found: $RUN_AS_USER"
fi

USER_HOME=$(getent passwd "$RUN_AS_USER" | cut -d: -f6)
[ -n "$USER_HOME" ] || die "Cannot resolve home for $RUN_AS_USER"

# Keep Hermes context length in sync with the vLLM serving window unless the
# user explicitly provided --hermes-context-length. This prevents a mismatch
# where vLLM serves a different max-model-len than Hermes believes it has.
if [ -z "$HERMES_CONTEXT_LENGTH" ]; then
  HERMES_CONTEXT_LENGTH="$CTX_LEN"
fi

if [ -n "$HF_TOKEN_FILE" ]; then
  if [ ! -f "$HF_TOKEN_FILE" ]; then
    die "HF token file not found: $HF_TOKEN_FILE"
  fi
  HF_TOKEN_VALUE="$(tr -d '\r\n' < "$HF_TOKEN_FILE")"
fi

if [ -z "$HERMES_API_KEY" ]; then
  # /proc/sys/kernel/random/uuid is always available on Linux, produces a finite
  # value (no SIGPIPE risk), and requires no external tools beyond tr.
  HERMES_API_KEY="hermes-$(tr -d '-\n' < /proc/sys/kernel/random/uuid)"
fi

if [ -z "$VLLM_BASE_URL" ]; then
  VLLM_BASE_URL="http://127.0.0.1:${VLLM_PORT}/v1"
  if [ "$SKIP_VLLM" = "true" ]; then
    log "--vllm-base-url was not provided. Using local default for existing vLLM: $VLLM_BASE_URL"
  fi
fi

PUBLIC_IP_EFFECTIVE=$(detect_public_ip)

cat <<EOF
============================================================
Hermes Agent Deployment
============================================================
Mode:                     $MODE
Run as user:              $RUN_AS_USER
Home:                     $USER_HOME
Log file:                 $LOG_FILE

vLLM skip:                $SKIP_VLLM
vLLM model:               $VLLM_MODEL
vLLM serve:               ${VLLM_HOST}:${VLLM_PORT}
vLLM ctx len:             $CTX_LEN
vLLM version:             $VLLM_VERSION
vLLM health timeout:      ${VLLM_HEALTH_TIMEOUT}s
vLLM base URL for Hermes: $VLLM_BASE_URL
HF token:                 $( [ -n "$HF_TOKEN_VALUE" ] && echo set || echo not-set )

Hermes skip:              $SKIP_HERMES
Hermes API port:          $HERMES_API_PORT
Dashboard local port:     $HERMES_DASHBOARD_PORT
Dashboard public port:    $NGINX_DASHBOARD_PORT

Discord token:            $( [ -n "$DISCORD_TOKEN" ] && echo set || echo not-set )
Discord home channel:     ${DISCORD_HOME_CHANNEL:-not-set}
ntfy topic:               ${NTFY_TOPIC:-not-set}
ntfy native:              $NTFY_NATIVE_ENABLED
ntfy inbound:             $NTFY_ALLOW_INBOUND
ntfy token:               $( [ -n "$NTFY_TOKEN" ] && echo set || echo not-set )
Tavily key:               $( [ -n "$TAVILY_API_KEY" ] && echo set || echo not-set )
Public IP:                $PUBLIC_IP_EFFECTIVE
============================================================
EOF

# -----------------------------
# Step implementations
# -----------------------------
install_system_deps() {
  export DEBIAN_FRONTEND=noninteractive
  as_root apt-get update -qq
  as_root apt-get install -y \
    ca-certificates curl jq nginx openssl gnupg \
    python3 python3-pip python3-venv python3-yaml \
    lsof procps net-tools \
    build-essential cmake ninja-build gcc g++ make > /dev/null
  if ! command -v ninja >/dev/null 2>&1; then
    die "ninja command is not available after installing ninja-build."
  fi
  as_root systemctl enable nginx >/dev/null 2>&1 || true
}

detect_gpu() {
  if command -v nvidia-smi >/dev/null 2>&1; then
    GPU_TYPE="nvidia"
    log "Detected NVIDIA GPU:"
    nvidia-smi --query-gpu=name,memory.total --format=csv,noheader || true
  elif command -v rocm-smi >/dev/null 2>&1; then
    GPU_TYPE="amd"
    log "Detected AMD GPU:"
    rocm-smi --showproductname || true
  else
    die "No supported GPU found. Use --skip-vllm if this is a Hermes-only VM."
  fi
}

install_vllm() {
  detect_gpu

  local venv="$USER_HOME/venv_vllm"
  run_as_user "python3 -m venv '$venv'"

  run_as_user "source '$venv/bin/activate' && python -m pip install -U pip 'setuptools<82' wheel packaging ninja"

  if [ "$GPU_TYPE" = "nvidia" ]; then
    run_as_user "source '$venv/bin/activate' && pip install -U 'vllm==${VLLM_VERSION}' 'transformers>=4.51.0' 'accelerate' 'openai' 'huggingface_hub' > '$USER_HOME/vllm_install.log' 2>&1"
  else
    run_as_user "source '$venv/bin/activate' && pip install -U 'uv' >> '$USER_HOME/vllm_install.log' 2>&1 && uv pip install 'vllm==${VLLM_VERSION}' --extra-index-url 'https://wheels.vllm.ai/rocm/' >> '$USER_HOME/vllm_install.log' 2>&1"
  fi

  run_as_user "source '$venv/bin/activate' && python - <<'PY'
import shutil
import vllm
print('vLLM version:', vllm.__version__)
print('venv ninja:', shutil.which('ninja'))
if shutil.which('ninja') is None:
    raise SystemExit('ninja is not available in the vLLM virtual environment')
PY"
  log "system ninja: $(command -v ninja || true)"
}

pre_download_hf_model() {
  if [ -z "${HF_TOKEN_VALUE:-}" ]; then
    log "HF token not provided. Skipping authenticated Hugging Face pre-download."
    return 0
  fi

  log "Pre-downloading Hugging Face model with temporary HF token. The token will not be written to systemd or shell profiles."

  sudo -u "$RUN_AS_USER" -H env \
    HF_TOKEN="$HF_TOKEN_VALUE" \
    HUGGING_FACE_HUB_TOKEN="$HF_TOKEN_VALUE" \
    MODEL_NAME="$VLLM_MODEL" \
    bash -lc "
      set -e
      source '$USER_HOME/venv_vllm/bin/activate'
      python - <<'PY'
import os
from huggingface_hub import snapshot_download
repo_id = os.environ['MODEL_NAME']
snapshot_download(repo_id=repo_id)
print('Downloaded or verified Hugging Face cache for:', repo_id)
PY
    "

  HF_TOKEN_VALUE=""
  unset HF_TOKEN HUGGING_FACE_HUB_TOKEN
  log "Hugging Face pre-download completed. HF token cleared from the deploy script environment."
}

print_vllm_diagnostics() {
  log "vLLM service status:"
  as_root systemctl status vllm --no-pager || true
  log "vLLM recent log:"
  as_root tail -n 160 "$USER_HOME/vllm-serve.log" || true
  log "GPU status:"
  nvidia-smi || true
  log "ninja status:"
  command -v ninja || true
  run_as_user "source '$USER_HOME/venv_vllm/bin/activate' && python -c 'import shutil; print(shutil.which(\"ninja\"))'" || true
}

configure_vllm_service() {
  local venv="$USER_HOME/venv_vllm"
  local extra_args=""
  if [ -n "$TOOL_CALL_PARSER" ]; then
    extra_args="--enable-auto-tool-choice --tool-call-parser ${TOOL_CALL_PARSER}"
  fi

  as_root install -o "$RUN_AS_USER" -g "$RUN_AS_USER" -m 0644 /dev/null "$USER_HOME/vllm-serve.log"

  as_root tee /etc/systemd/system/vllm.service >/dev/null <<EOF
[Unit]
Description=vLLM OpenAI-compatible API Server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${RUN_AS_USER}
WorkingDirectory=${USER_HOME}
Environment=HOME=${USER_HOME}
Environment=HF_HOME=${USER_HOME}/.cache/huggingface
Environment=VLLM_WORKER_MULTIPROC_METHOD=spawn
Environment=PATH=${venv}/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
ExecStart=${venv}/bin/vllm serve ${VLLM_MODEL} --host ${VLLM_HOST} --port ${VLLM_PORT} --trust-remote-code --max-model-len ${CTX_LEN} --gpu-memory-utilization ${GPU_UTIL} --max-num-batched-tokens ${MAX_BATCHED_TOKENS} ${extra_args}
Restart=on-failure
RestartSec=10
TimeoutStartSec=${VLLM_HEALTH_TIMEOUT}
StandardOutput=append:${USER_HOME}/vllm-serve.log
StandardError=append:${USER_HOME}/vllm-serve.log

[Install]
WantedBy=multi-user.target
EOF

  as_root systemctl daemon-reload
  if [ "$ENABLE_SERVICES" = "true" ]; then
    as_root systemctl enable vllm
    as_root systemctl restart vllm
  fi
}

wait_vllm() {
  if [ "$ENABLE_SERVICES" != "true" ]; then
    log "Skipping vLLM wait because --no-services was set."
    return 0
  fi

  local url="http://127.0.0.1:${VLLM_PORT}/v1/models"
  local elapsed=0
  local interval=10
  local code=""
  local fatal_pattern='FileNotFoundError:.*ninja|No such file or directory:.*ninja|Engine core initialization failed|CUDA out of memory|No space left on device|ModuleNotFoundError|ImportError|RuntimeError: Engine core initialization failed'

  log "Waiting for HTTP 200: $url"
  while [ "$elapsed" -lt "$VLLM_HEALTH_TIMEOUT" ]; do
    code=$(curl -sS -o /dev/null -w "%{http_code}" --max-time 5 "$url" 2>/dev/null || true)
    if [ "$code" = "200" ]; then
      log "HTTP check OK: $url"
      log "vLLM /v1/models:"
      curl -sS --max-time 10 "$url" | jq . || true
      return 0
    fi

    if as_root systemctl is-failed --quiet vllm; then
      log "vLLM service entered failed state. Not waiting for the full timeout."
      print_vllm_diagnostics
      return 1
    fi

    if [ -f "$USER_HOME/vllm-serve.log" ] && grep -E "$fatal_pattern" "$USER_HOME/vllm-serve.log" >/dev/null 2>&1; then
      log "vLLM fatal error detected in log. Not waiting for the full timeout."
      print_vllm_diagnostics
      return 1
    fi

    if [ $((elapsed % 60)) -eq 0 ] && [ "$elapsed" -gt 0 ]; then
      echo ""
      log "Still waiting for vLLM. elapsed=${elapsed}s last_http=${code}"
      as_root tail -n 40 "$USER_HOME/vllm-serve.log" || true
      nvidia-smi || true
    else
      printf "."
    fi

    sleep "$interval"
    elapsed=$((elapsed + interval))
  done

  echo ""
  log "vLLM did not become healthy within ${VLLM_HEALTH_TIMEOUT}s. last_http=${code}"
  print_vllm_diagnostics
  return 1
}

install_hermes() {
  if [ ! -x "$USER_HOME/.local/bin/hermes" ]; then
    log "Installing Hermes Agent for $RUN_AS_USER"
    run_as_user "curl -fsSL https://raw.githubusercontent.com/NousResearch/hermes-agent/main/scripts/install.sh | bash"
  else
    log "Hermes already installed: $USER_HOME/.local/bin/hermes"
  fi

  if [ ! -x /usr/local/bin/hermes ]; then
    as_root ln -sf "$USER_HOME/.local/bin/hermes" /usr/local/bin/hermes
  fi

  run_as_user "'$USER_HOME/.local/bin/hermes' --version || true"
}

install_nodejs_for_dashboard() {
  # Hermes Dashboard web frontend uses Vite. Vite 7 requires a modern Node.js,
  # so Ubuntu's default nodejs package can be too old. Install Node.js 22 LTS via NodeSource when needed.
  local need_node="true"
  if command -v node >/dev/null 2>&1; then
    local major
    major=$(node -p "process.versions.node.split('.')[0]" 2>/dev/null || echo 0)
    if [ "${major:-0}" -ge 20 ]; then
      need_node="false"
    fi
  fi

  if [ "$need_node" = "true" ]; then
    log "Installing Node.js 22 for Hermes Dashboard frontend build"
    as_root bash -lc "curl -fsSL https://deb.nodesource.com/setup_22.x | bash -"
    export DEBIAN_FRONTEND=noninteractive
    as_root apt-get install -y nodejs > /dev/null
  else
    log "Node.js already available: $(node --version)"
  fi

  command -v npm >/dev/null 2>&1 || die "npm is not available after Node.js installation."
  log "Node.js: $(node --version), npm: $(npm --version)"
}

build_hermes_dashboard_frontend() {
  local project_dir="$USER_HOME/.hermes/hermes-agent"
  local web_dir="$project_dir/web"
  local dist_index="$project_dir/hermes_cli/web_dist/index.html"
  local legacy_dist_index="$web_dir/dist/index.html"

  if [ ! -d "$web_dir" ]; then
    die "Hermes web directory not found: $web_dir"
  fi

  # Hermes Agent builds the Vite frontend into hermes_cli/web_dist, not web/dist.
  # Older/local builds may still use web/dist, so accept both.
  if [ -f "$dist_index" ]; then
    log "Hermes Dashboard frontend already built: $dist_index"
    return 0
  fi
  if [ -f "$legacy_dist_index" ]; then
    log "Hermes Dashboard frontend already built: $legacy_dist_index"
    return 0
  fi

  install_nodejs_for_dashboard

  log "Building Hermes Dashboard frontend in $web_dir"
  if [ -f "$web_dir/package-lock.json" ]; then
    run_as_user "cd '$web_dir' && npm ci && npm run build"
  else
    run_as_user "cd '$web_dir' && npm install && npm run build"
  fi

  if [ -f "$dist_index" ]; then
    log "Hermes Dashboard frontend build output verified: $dist_index"
    return 0
  fi
  if [ -f "$legacy_dist_index" ]; then
    log "Hermes Dashboard frontend build output verified: $legacy_dist_index"
    return 0
  fi

  die "Hermes Dashboard frontend build did not produce $dist_index or $legacy_dist_index"
}

configure_hermes_env() {
  local env_file="$USER_HOME/.hermes/.env"
  as_root install -d -m 700 -o "$RUN_AS_USER" -g "$RUN_AS_USER" "$USER_HOME/.hermes"

  set_env_kv "API_SERVER_ENABLED" "true" "$env_file"
  set_env_kv "API_SERVER_HOST" "$HERMES_API_HOST" "$env_file"
  set_env_kv "API_SERVER_PORT" "$HERMES_API_PORT" "$env_file"
  set_env_kv "API_SERVER_KEY" "$HERMES_API_KEY" "$env_file"

  set_env_kv "GATEWAY_ALLOW_ALL_USERS" "$GATEWAY_ALLOW_ALL_USERS" "$env_file"

  if [ "$NTFY_NATIVE_ENABLED" = "true" ] && [ -n "$NTFY_TOPIC" ]; then
    set_env_kv "NTFY_TOPIC" "$NTFY_TOPIC" "$env_file"
    set_env_kv "NTFY_PUBLISH_TOPIC" "$NTFY_TOPIC" "$env_file"
    set_env_kv "NTFY_HOME_CHANNEL" "$NTFY_TOPIC" "$env_file"
    set_env_kv "NTFY_HOME_CHANNEL_NAME" "$NTFY_TOPIC" "$env_file"
    set_env_kv "NTFY_MARKDOWN" "$NTFY_MARKDOWN" "$env_file"
    if [ -n "$NTFY_SERVER_URL" ]; then
      set_env_kv "NTFY_SERVER_URL" "$NTFY_SERVER_URL" "$env_file"
    fi
    if [ -n "$NTFY_TOKEN" ]; then
      set_env_kv "NTFY_TOKEN" "$NTFY_TOKEN" "$env_file"
    fi
    if [ "$NTFY_ALLOW_INBOUND" = "true" ]; then
      set_env_kv "NTFY_ALLOWED_USERS" "$NTFY_TOPIC" "$env_file"
    fi
  fi

  if [ -n "$DISCORD_TOKEN" ]; then
    set_env_kv "DISCORD_BOT_TOKEN" "$DISCORD_TOKEN" "$env_file"
  fi
  if [ -n "$DISCORD_HOME_CHANNEL" ]; then
    set_env_kv "DISCORD_HOME_CHANNEL" "$DISCORD_HOME_CHANNEL" "$env_file"
  fi
  if [ -n "$DISCORD_HOME_CHANNEL_NAME" ]; then
    set_env_kv "DISCORD_HOME_CHANNEL_NAME" "$DISCORD_HOME_CHANNEL_NAME" "$env_file"
  fi
  if [ -n "$DISCORD_ALLOWED_USERS" ]; then
    set_env_kv "DISCORD_ALLOWED_USERS" "$DISCORD_ALLOWED_USERS" "$env_file"
  fi
  if [ -n "$TAVILY_API_KEY" ]; then
    set_env_kv "TAVILY_API_KEY" "$TAVILY_API_KEY" "$env_file"
  fi

  as_root chown "$RUN_AS_USER:$RUN_AS_USER" "$env_file"
  as_root chmod 600 "$env_file"
}

configure_hermes_config() {
  local cfg_file="$USER_HOME/.hermes/config.yaml"
  as_root install -d -m 700 -o "$RUN_AS_USER" -g "$RUN_AS_USER" "$USER_HOME/.hermes"
  touch "$cfg_file"
  as_root chown "$RUN_AS_USER:$RUN_AS_USER" "$cfg_file"

  CFG_FILE="$cfg_file" \
  VLLM_MODEL="$VLLM_MODEL" \
  VLLM_BASE_URL="$VLLM_BASE_URL" \
  HERMES_CONTEXT_LENGTH="$HERMES_CONTEXT_LENGTH" \
  HERMES_MAX_TOKENS="$HERMES_MAX_TOKENS" \
  HERMES_MAX_TURNS="$HERMES_MAX_TURNS" \
  HERMES_API_HOST="$HERMES_API_HOST" \
  HERMES_API_PORT="$HERMES_API_PORT" \
  HERMES_API_KEY="$HERMES_API_KEY" \
  PROVIDER_TIMEOUT_SECONDS="$PROVIDER_TIMEOUT_SECONDS" \
  PROVIDER_STALE_TIMEOUT_SECONDS="$PROVIDER_STALE_TIMEOUT_SECONDS" \
  OLLAMA_BASE_URL="$OLLAMA_BASE_URL" \
  OLLAMA_MODEL="$OLLAMA_MODEL" \
  NTFY_NATIVE_ENABLED="$NTFY_NATIVE_ENABLED" \
  NTFY_MARKDOWN="$NTFY_MARKDOWN" \
  python3 - <<'PY'
import os
from pathlib import Path
import yaml

path = Path(os.environ["CFG_FILE"])
if path.exists() and path.read_text(encoding="utf-8", errors="ignore").strip():
    try:
        cfg = yaml.safe_load(path.read_text(encoding="utf-8")) or {}
    except Exception:
        backup = path.with_suffix(".yaml.bak")
        backup.write_text(path.read_text(encoding="utf-8", errors="ignore"), encoding="utf-8")
        cfg = {}
else:
    cfg = {}

cfg["model"] = {
    "provider": "custom",
    "default": os.environ["VLLM_MODEL"],
    "base_url": os.environ["VLLM_BASE_URL"],
    "context_length": int(os.environ["HERMES_CONTEXT_LENGTH"]),
    "max_tokens": int(os.environ["HERMES_MAX_TOKENS"]),
}
cfg["max_turns"] = int(os.environ["HERMES_MAX_TURNS"])

cfg["API_SERVER_ENABLED"] = True
cfg["API_SERVER_HOST"] = os.environ["HERMES_API_HOST"]
cfg["API_SERVER_PORT"] = int(os.environ["HERMES_API_PORT"])
cfg["API_SERVER_KEY"] = os.environ["HERMES_API_KEY"]

providers = cfg.setdefault("providers", {})
custom = providers.setdefault("custom", {})
custom["request_timeout_seconds"] = int(os.environ["PROVIDER_TIMEOUT_SECONDS"])
custom["stale_timeout_seconds"] = int(os.environ["PROVIDER_STALE_TIMEOUT_SECONDS"])
models = custom.setdefault("models", {})
models[os.environ["VLLM_MODEL"]] = {
    "timeout_seconds": int(os.environ["PROVIDER_TIMEOUT_SECONDS"]),
    "stale_timeout_seconds": int(os.environ["PROVIDER_STALE_TIMEOUT_SECONDS"]),
}

if os.environ.get("OLLAMA_BASE_URL"):
    cfg["ollama_fallback"] = {
        "base_url": os.environ["OLLAMA_BASE_URL"],
        "model": os.environ.get("OLLAMA_MODEL") or "qwen3:30b",
        "note": "Comparison or fallback endpoint. Confirm Hermes native fallback support before relying on automatic fallback.",
    }

if os.environ.get("NTFY_NATIVE_ENABLED") == "true":
    platforms = cfg.setdefault("platforms", {})
    ntfy = platforms.setdefault("ntfy", {})
    extra = ntfy.setdefault("extra", {})
    extra["markdown"] = os.environ.get("NTFY_MARKDOWN", "true").lower() == "true"

path.write_text(yaml.safe_dump(cfg, sort_keys=False, allow_unicode=True), encoding="utf-8")
PY

  as_root chown "$RUN_AS_USER:$RUN_AS_USER" "$cfg_file"
  as_root chmod 600 "$cfg_file"
}

create_ntfy_script_and_skill() {
  local scripts_dir="$USER_HOME/.hermes/scripts"
  local notify_script="$scripts_dir/notify_ntfy.sh"
  as_root install -d -m 700 -o "$RUN_AS_USER" -g "$RUN_AS_USER" "$scripts_dir"

  as_root tee "$notify_script" >/dev/null <<EOF
#!/usr/bin/env bash
set -euo pipefail

TOPIC="${NTFY_TOPIC}"
TITLE="\${1:-Hermes Agent}"
MESSAGE="\${2:-Hermes notification}"
PRIORITY="\${3:-default}"
TAGS="\${4:-bell}"

curl -sS \\
  -H "Title: \${TITLE}" \\
  -H "Priority: \${PRIORITY}" \\
  -H "Tags: \${TAGS}" \\
  -d "\${MESSAGE}" \\
  "https://ntfy.sh/\${TOPIC}" >/dev/null
EOF
  as_root chown "$RUN_AS_USER:$RUN_AS_USER" "$notify_script"
  as_root chmod 700 "$notify_script"

  if [ "$CREATE_NOTIFY_SKILL" = "true" ]; then
    local skill_dir="$USER_HOME/.hermes/skills/devops/notify-ntfy"
    as_root install -d -m 700 -o "$RUN_AS_USER" -g "$RUN_AS_USER" "$skill_dir"

    as_root tee "$skill_dir/SKILL.md" >/dev/null <<EOF
---
name: notify-ntfy
description: Send short ntfy notifications to the configured topic when the user asks for an alert, alarm, notification, cron result notice, or completion notice. Use the local helper script and do not request credentials.
---

# notify-ntfy

Use this skill when the user asks for 알림, 알람, notify, notification, 완료 알림, 크론 결과 알림, "끝나면 알려줘", or "완료되면 알려줘".

Target topic: ${NTFY_TOPIC}

Native Hermes delivery:
- For cron jobs, prefer deliver="ntfy" when a delivery target is needed.
- For direct platform delivery, use target="ntfy:${NTFY_TOPIC}" when supported.

Fallback shell command:

\`\`\`bash
~/.hermes/scripts/notify_ntfy.sh "<title>" "<message>" "<priority>" "<tags>"
\`\`\`

Defaults:
- title: Hermes Agent
- priority: default
- tags: bell

Priority mapping:
- success or completion: default, heavy_check_mark
- warning: high, warning
- failure or outage: urgent, rotating_light
- cron result: default, calendar

Rules:
- Keep messages short.
- Do not expose secrets.
- Do not ask for ntfy credentials.
- If used inside a cron script, call the helper script explicitly at the end.
EOF
    as_root chown -R "$RUN_AS_USER:$RUN_AS_USER" "$skill_dir"
  fi

  local mem_dir="$USER_HOME/.hermes/memories"
  as_root install -d -m 700 -o "$RUN_AS_USER" -g "$RUN_AS_USER" "$mem_dir"
  local mem_file="$mem_dir/MEMORY.md"
  touch "$mem_file"
  if ! grep -qF "${NTFY_TOPIC}" "$mem_file" 2>/dev/null; then
    cat >> "$mem_file" <<EOF

- When the user asks for alerts, alarms, notifications, completion notices, or cron result notices, send a short ntfy notification when appropriate. Prefer Hermes native ntfy delivery with deliver="ntfy" for cron jobs or target="ntfy:${NTFY_TOPIC}" for direct messages. If native delivery is not available, use: ~/.hermes/scripts/notify_ntfy.sh "<title>" "<message>" "<priority>" "<tags>". The configured ntfy topic is ${NTFY_TOPIC}. Do not send ntfy if the user explicitly says not to.
EOF
  fi
  as_root chown -R "$RUN_AS_USER:$RUN_AS_USER" "$mem_dir"
}

create_checkllm_skill() {
  if [ "$CREATE_CHECKLLM_SKILL" != "true" ]; then
    return 0
  fi

  local skill_dir="$USER_HOME/.hermes/skills/devops/checkllm"
  as_root install -d -m 700 -o "$RUN_AS_USER" -g "$RUN_AS_USER" "$skill_dir"

  as_root tee "$skill_dir/SKILL.md" >/dev/null <<EOF
---
name: checkllm
description: Check LLM backend health using API-only checks for vLLM primary, optional Ollama comparison endpoint, Hermes config, latency, streaming, and metrics. Do not use SSH. Do not call a tool named check_llm.
---

# checkllm

This is a skill instruction, not a callable tool. Do not call a tool named checkllm, check_llm, or check-llm.

Use this skill when the user asks to check LLM status, vLLM status, Ollama fallback status, Qwen3 serving status, Hermes model connection, slow inference, /v1/models, /v1/chat/completions, OpenAI-compatible endpoint health, or whether Hermes is using vLLM instead of Ollama.

Restrictions:
- Do not use SSH.
- Do not request SSH keys.
- Use HTTP API checks and local Hermes config checks only.
- Do not print secret values.

Known endpoints:
- vLLM primary: ${VLLM_BASE_URL}
- vLLM model: ${VLLM_MODEL}
- Ollama comparison endpoint: ${OLLAMA_BASE_URL:-not-configured}
- Ollama model: ${OLLAMA_MODEL}

Checks:
1. vLLM models:
\`\`\`bash
curl -sS --max-time 10 "${VLLM_BASE_URL}/models" | jq .
\`\`\`

2. vLLM chat completion latency:
\`\`\`bash
time curl -sS --max-time 120 "${VLLM_BASE_URL}/chat/completions" \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer ${VLLM_API_KEY}" \\
  -d '{"model":"${VLLM_MODEL}","messages":[{"role":"user","content":"한국어로 한 문장만 답해줘. vLLM API 연결 테스트야."}],"max_tokens":64,"stream":false}' | jq .
\`\`\`

3. vLLM streaming:
\`\`\`bash
curl -N -sS --max-time 120 "${VLLM_BASE_URL}/chat/completions" \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer ${VLLM_API_KEY}" \\
  -d '{"model":"${VLLM_MODEL}","messages":[{"role":"user","content":"짧게 한 문장으로 응답해줘. streaming 테스트야."}],"max_tokens":64,"stream":true}'
\`\`\`

4. vLLM metrics, optional:
\`\`\`bash
curl -sS --max-time 10 "${VLLM_BASE_URL%/v1}/metrics" | head -40
\`\`\`

5. Ollama comparison, only if configured:
\`\`\`bash
curl -sS --max-time 10 "${OLLAMA_BASE_URL:-http://127.0.0.1:3000/v1}/models" | jq .
\`\`\`

6. Hermes config:
\`\`\`bash
grep -nA20 '^model:' ~/.hermes/config.yaml || true
grep -nA20 -E 'fallback|ollama|providers:' ~/.hermes/config.yaml || true
hermes config
\`\`\`

Report sections:
1. Summary
2. vLLM primary status
3. Ollama comparison status
4. Latency comparison
5. Streaming status
6. Metrics status
7. Hermes config status
8. Likely cause
9. Recommended next command
EOF
  as_root chown -R "$RUN_AS_USER:$RUN_AS_USER" "$skill_dir"
}

configure_hermes_services() {
  as_root tee /etc/systemd/system/hermes-gateway.service >/dev/null <<EOF
[Unit]
Description=Hermes Agent Gateway and API Server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${RUN_AS_USER}
WorkingDirectory=${USER_HOME}
Environment=HOME=${USER_HOME}
ExecStart=${USER_HOME}/.local/bin/hermes gateway run
Restart=always
RestartSec=5
StandardOutput=append:${USER_HOME}/hermes-gateway.log
StandardError=append:${USER_HOME}/hermes-gateway.log

[Install]
WantedBy=multi-user.target
EOF

  as_root tee /etc/systemd/system/hermes-dashboard.service >/dev/null <<EOF
[Unit]
Description=Hermes Dashboard
After=network-online.target hermes-gateway.service
Wants=network-online.target

[Service]
Type=simple
User=${RUN_AS_USER}
WorkingDirectory=${USER_HOME}
Environment=HOME=${USER_HOME}
ExecStart=${USER_HOME}/.local/bin/hermes dashboard --host ${HERMES_DASHBOARD_HOST} --port ${HERMES_DASHBOARD_PORT} --tui --no-open
Restart=always
RestartSec=5
StandardOutput=append:${USER_HOME}/hermes-dashboard.log
StandardError=append:${USER_HOME}/hermes-dashboard.log

[Install]
WantedBy=multi-user.target
EOF

  as_root systemctl daemon-reload
  if [ "$ENABLE_SERVICES" = "true" ]; then
    as_root systemctl enable hermes-gateway hermes-dashboard
    as_root systemctl restart hermes-gateway
    as_root systemctl restart hermes-dashboard
  fi
}

configure_nginx() {
  as_root rm -f /etc/nginx/sites-enabled/default

  as_root tee /etc/nginx/sites-available/hermes-dashboard >/dev/null <<EOF
server {
    listen ${NGINX_DASHBOARD_PORT};
    server_name _;

    location / {
        proxy_pass http://127.0.0.1:${HERMES_DASHBOARD_PORT};
        proxy_http_version 1.1;

        proxy_set_header Host 127.0.0.1:${HERMES_DASHBOARD_PORT};
        proxy_set_header Origin http://127.0.0.1:${HERMES_DASHBOARD_PORT};
        proxy_set_header X-Real-IP 127.0.0.1;
        proxy_set_header X-Forwarded-For 127.0.0.1;
        proxy_set_header X-Forwarded-Proto http;

        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";

        proxy_read_timeout 3600;
        proxy_send_timeout 3600;
        proxy_buffering off;
    }
}
EOF

  as_root ln -sf /etc/nginx/sites-available/hermes-dashboard /etc/nginx/sites-enabled/hermes-dashboard
  as_root nginx -t
  if [ "$ENABLE_SERVICES" = "true" ]; then
    as_root systemctl enable nginx
    as_root systemctl restart nginx
  fi
}

wait_hermes() {
  if [ "$ENABLE_SERVICES" != "true" ]; then
    log "Skipping Hermes wait because --no-services was set."
    return 0
  fi

  wait_for_http "http://127.0.0.1:${HERMES_API_PORT}/health" 180 5 200 || {
    log "Hermes API health failed. Gateway log:"
    as_root tail -n 120 "$USER_HOME/hermes-gateway.log" || true
    return 1
  }

  wait_for_http "http://127.0.0.1:${HERMES_DASHBOARD_PORT}/api/status" 180 5 200 || {
    log "Hermes Dashboard status failed. Dashboard log:"
    as_root tail -n 120 "$USER_HOME/hermes-dashboard.log" || true
    return 1
  }

  wait_for_http "http://127.0.0.1:${NGINX_DASHBOARD_PORT}/api/status" 180 5 200 || {
    log "nginx dashboard proxy failed. nginx error log:"
    as_root tail -n 120 /var/log/nginx/error.log || true
    return 1
  }
}

test_ntfy() {
  if [ -n "$NTFY_TOPIC" ] && [ -x "$USER_HOME/.hermes/scripts/notify_ntfy.sh" ]; then
    run_as_user "'$USER_HOME/.hermes/scripts/notify_ntfy.sh' 'Hermes Agent' 'Hermes Agent deployment completed.' 'default' 'heavy_check_mark'" || true
  fi
}

print_summary() {
  local total
  total=$(($(date +%s) - START_TS))

  echo ""
  echo "============================================================"
  echo "Deployment Summary"
  echo "============================================================"
  printf "%-45s %10s\n" "Step" "Duration"
  printf "%-45s %10s\n" "----" "--------"
  local i
  for i in "${!STEP_NAMES[@]}"; do
    printf "%-45s %9ss\n" "${STEP_NAMES[$i]}" "${STEP_DURATIONS[$i]}"
  done
  printf "%-45s %9ss\n" "TOTAL" "$total"

  echo ""
  echo "Endpoints"
  echo "---------"
  echo "Hermes Dashboard:"
  echo "  http://${PUBLIC_IP_EFFECTIVE}:${NGINX_DASHBOARD_PORT}/chat"
  echo ""
  echo "Hermes Dashboard status:"
  echo "  http://${PUBLIC_IP_EFFECTIVE}:${NGINX_DASHBOARD_PORT}/api/status"
  echo ""
  echo "Hermes API health:"
  echo "  http://${PUBLIC_IP_EFFECTIVE}:${HERMES_API_PORT}/health"
  echo ""
  echo "Hermes OpenAI-compatible API:"
  echo "  http://${PUBLIC_IP_EFFECTIVE}:${HERMES_API_PORT}/v1"
  echo "  API key: stored in ${USER_HOME}/.hermes/.env as API_SERVER_KEY"
  echo ""
  if [ "$SKIP_VLLM" != "true" ]; then
    echo "vLLM API:"
    echo "  http://${PUBLIC_IP_EFFECTIVE}:${VLLM_PORT}/v1"
    echo "  model: ${VLLM_MODEL}"
    echo ""
  fi
  if [ -n "$OLLAMA_BASE_URL" ]; then
    echo "Ollama comparison endpoint:"
    echo "  ${OLLAMA_BASE_URL}"
    echo "  model: ${OLLAMA_MODEL}"
    echo ""
  fi
  if [ -n "$DISCORD_TOKEN" ]; then
    echo "Discord:"
    echo "  token configured"
    echo "  home channel: ${DISCORD_HOME_CHANNEL:-not-set}"
    echo "  Note: Discord Developer Portal privileged intents must be enabled manually."
    echo ""
  fi
  if [ -n "$NTFY_TOPIC" ]; then
    echo "ntfy:"
    echo "  topic: ${NTFY_TOPIC}"
    echo "  test URL: https://ntfy.sh/${NTFY_TOPIC}"
    echo ""
  fi
  if [ -n "$TAVILY_API_KEY" ]; then
    echo "Tavily:"
    echo "  key configured"
    echo ""
  fi
  echo "Logs"
  echo "----"
  echo "Deployment log:       $LOG_FILE"
  echo "vLLM log:             ${USER_HOME}/vllm-serve.log"
  echo "Hermes gateway log:   ${USER_HOME}/hermes-gateway.log"
  echo "Hermes dashboard log: ${USER_HOME}/hermes-dashboard.log"
  echo ""
  echo "Useful commands"
  echo "---------------"
  echo "sudo systemctl status vllm --no-pager"
  echo "sudo systemctl status hermes-gateway --no-pager"
  echo "sudo systemctl status hermes-dashboard --no-pager"
  echo "sudo systemctl status nginx --no-pager"
  echo "curl -s http://127.0.0.1:${NGINX_DASHBOARD_PORT}/api/status | jq ."
  echo "curl -s http://127.0.0.1:${HERMES_API_PORT}/health | jq ."
  echo "============================================================"
}

# -----------------------------
# Main
# -----------------------------
run_step "Install system dependencies" install_system_deps

if [ "$SKIP_VLLM" != "true" ]; then
  run_step "Install vLLM" install_vllm
  run_step "Pre-download Hugging Face model if token is provided" pre_download_hf_model
  run_step "Configure vLLM systemd service" configure_vllm_service
  run_step "Wait for vLLM health" wait_vllm
fi

if [ "$SKIP_HERMES" != "true" ]; then
  run_step "Install Hermes Agent" install_hermes
  run_step "Build Hermes Dashboard frontend" build_hermes_dashboard_frontend
  run_step "Configure Hermes env" configure_hermes_env
  run_step "Configure Hermes model and API server" configure_hermes_config
  run_step "Create ntfy helper and skill" create_ntfy_script_and_skill
  run_step "Create checkllm skill" create_checkllm_skill
  run_step "Configure Hermes systemd services" configure_hermes_services
  run_step "Configure nginx reverse proxy" configure_nginx
  run_step "Wait for Hermes and Dashboard health" wait_hermes
  run_step "Send optional ntfy completion notification" test_ntfy
fi

print_summary
