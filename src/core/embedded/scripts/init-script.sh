#!/bin/bash
# CB-Tumblebug init script wrapper. Delivered to the VM as user data and run once on first boot.
# Embedded into cb-tumblebug (src/core/embedded); CB-Tumblebug prepends the TB_* variables below.
#
#   TB_INIT_ACCESS            1 to create the SSH account (TB_INIT_USER with TB_INIT_PUBKEY), 0 to skip
#   TB_INIT_USER              login account name
#   TB_INIT_PUBKEY            SSH public key for the account
#   TB_INIT_SCRIPT_COUNT      number of user scripts, run in order; the first failure stops the run
#   TB_INIT_SCRIPT_<i>_NAME   name of script i (1-based), echoed in the report
#   TB_INIT_SCRIPT_<i>_B64    base64 of script i
#   TB_NS_ID, TB_INFRA_ID, TB_NODEGROUP_ID, TB_NODE_ID, TB_CSP, TB_REGION   node identity
#   TB_NOTIFY_URL, TB_NOTIFY_TOPIC, TB_NOTIFY_TOKEN, TB_NOTIFY_CA_B64        optional ntfy report target
#
# Result: $TB_DIR/init-script.status ("ok|fail <exit code> <time>"), script output in $TB_LOG, and ntfy messages of
# kind initScript when TB_NOTIFY_URL is set: one "step" event (status initializing) as each step starts and one "done" event.
set -u
TB_DIR=${TB_DIR:-/var/lib/tb}
TB_LOG=${TB_LOG:-/var/log/tb-init-script.log}
[ -f "$TB_DIR/init-script.status" ] && exit 0   # some CSPs run user data on every boot
mkdir -p "$TB_DIR"
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

if [ "${TB_INIT_ACCESS:-0}" = "1" ] && [ -n "${TB_INIT_USER:-}" ]; then
  if getent group sudo >/dev/null 2>&1; then SUDO_GROUP=sudo
  elif getent group wheel >/dev/null 2>&1; then SUDO_GROUP=wheel
  else groupadd sudo && SUDO_GROUP=sudo; fi
  id -u "$TB_INIT_USER" >/dev/null 2>&1 || useradd -s /bin/bash -m -G "$SUDO_GROUP" "$TB_INIT_USER"
  HOME_DIR=$(getent passwd "$TB_INIT_USER" | cut -d: -f6)
  mkdir -p "$HOME_DIR/.ssh"
  grep -qF "$TB_INIT_PUBKEY" "$HOME_DIR/.ssh/authorized_keys" 2>/dev/null || echo "$TB_INIT_PUBKEY" >> "$HOME_DIR/.ssh/authorized_keys"
  chmod 700 "$HOME_DIR/.ssh"; chmod 600 "$HOME_DIR/.ssh/authorized_keys"
  chown -R "$TB_INIT_USER:$TB_INIT_USER" "$HOME_DIR/.ssh"
  command -v restorecon >/dev/null 2>&1 && restorecon -R "$HOME_DIR/.ssh" >/dev/null 2>&1
  echo "$TB_INIT_USER ALL=(ALL:ALL) NOPASSWD: ALL" > "/etc/sudoers.d/$TB_INIT_USER"
  chmod 440 "/etc/sudoers.d/$TB_INIT_USER"
fi

CA_OPT=""
if [ -n "${TB_NOTIFY_URL:-}" ] && [ -n "${TB_NOTIFY_CA_B64:-}" ]; then
  echo "$TB_NOTIFY_CA_B64" | base64 -d > "$TB_DIR/notify-ca.pem"
  CA_OPT="--cacert $TB_DIR/notify-ca.pem"
fi

COUNT=${TB_INIT_SCRIPT_COUNT:-0}
declare -a S_NAME S_STATUS S_EXIT S_SEC
i=1
while [ "$i" -le "$COUNT" ]; do
  NAME_VAR="TB_INIT_SCRIPT_${i}_NAME"
  S_NAME[$i]=${!NAME_VAR:-step-$i}; S_STATUS[$i]=pending; S_EXIT[$i]=0; S_SEC[$i]=0
  i=$((i+1))
done

steps_json() {
  local out="" j=1
  while [ "$j" -le "$COUNT" ]; do
    out="$out${out:+,}$(printf '{"name":"%s","status":"%s","exitCode":%s,"durationSec":%s}' "${S_NAME[$j]}" "${S_STATUS[$j]}" "${S_EXIT[$j]}" "${S_SEC[$j]}")"
    j=$((j+1))
  done
  echo "$out"
}

# report <event> <status> <current step> <attempts>: sends a full snapshot so CB-Tumblebug can just store the latest one
report() {
  [ -n "${TB_NOTIFY_URL:-}" ] || return 0
  local msg body n=0
  msg=$(tail -c 300 "$TB_LOG" 2>/dev/null | tr -d '\000-\037"\\' | tail -c 200)
  body=$(printf '{"kind":"initScript","nsId":"%s","infraId":"%s","nodeGroupId":"%s","nodeId":"%s","csp":"%s","region":"%s","event":"%s","status":"%s","step":"%s","exitCode":%s,"durationSec":%s,"hostname":"%s","steps":[%s],"message":"%s"}' \
    "${TB_NS_ID:-}" "${TB_INFRA_ID:-}" "${TB_NODEGROUP_ID:-}" "${TB_NODE_ID:-}" "${TB_CSP:-}" "${TB_REGION:-}" \
    "$1" "$2" "$3" "$TB_EXIT" "$(( $(date +%s) - TB_START ))" "$(hostname)" "$(steps_json)" "$msg")
  while [ "$n" -lt "$4" ]; do
    # shellcheck disable=SC2086
    curl -fsS -m 10 $CA_OPT -H "Authorization: Bearer ${TB_NOTIFY_TOKEN:-}" -d "$body" "$TB_NOTIFY_URL/$TB_NOTIFY_TOPIC" >/dev/null 2>&1 && return 0
    n=$((n+1)); [ "$n" -lt "$4" ] && sleep 5
  done
  return 0
}

TB_STATUS=ok; TB_EXIT=0; TB_START=$(date +%s)
i=1
while [ "$i" -le "$COUNT" ]; do
  B64_VAR="TB_INIT_SCRIPT_${i}_B64"
  echo "${!B64_VAR:-}" | base64 -d > "$TB_DIR/init-script-$i.sh"
  chmod 700 "$TB_DIR/init-script-$i.sh"
  S_STATUS[$i]=running
  echo "== ${S_NAME[$i]} $(date -u +%FT%TZ)" >> "$TB_LOG"
  report step initializing "${S_NAME[$i]}" 1   # progress is best effort: one attempt, never delays the boot much
  STEP_START=$(date +%s); STEP_EXIT=0
  bash "$TB_DIR/init-script-$i.sh" >> "$TB_LOG" 2>&1 || STEP_EXIT=$?
  S_EXIT[$i]=$STEP_EXIT; S_SEC[$i]=$(( $(date +%s) - STEP_START ))
  if [ "$STEP_EXIT" -ne 0 ]; then S_STATUS[$i]=fail; TB_STATUS=fail; TB_EXIT=$STEP_EXIT; break; fi
  S_STATUS[$i]=ok
  i=$((i+1))
done
echo "$TB_STATUS $TB_EXIT $(date -u +%FT%TZ)" > "$TB_DIR/init-script.status"
report done "$TB_STATUS" "" 3
exit 0
