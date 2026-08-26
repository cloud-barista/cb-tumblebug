#!/usr/bin/env bash
#
# Check and (with confirmation) install the local prerequisites `make k-up` needs to bring up a
# kind-based cluster: kubectl, helm, kind, and the inotify sysctls kind requires. Docker is the
# one backend this script only *guides* — installing it needs distro packages, a running daemon,
# and a group re-login, none of which are safe to do silently.
#
# It prints the exact commands it would run and asks for a y/N confirmation before anything runs.
# Nothing is installed without that confirmation. Usage: make k-prereqs
set -euo pipefail

KIND_VERSION="${KIND_VERSION:-v0.32.0}"

# Colors (respect NO_COLOR / non-tty). Real ESC bytes, so all output goes through printf '%s'.
if [ -n "${NO_COLOR:-}" ] || [ ! -t 1 ]; then
  B= ; G= ; Y= ; R= ; C= ; D= ; X=
else
  B=$'\033[1m'; G=$'\033[32m'; Y=$'\033[33m'; R=$'\033[31m'; C=$'\033[36m'; D=$'\033[2m'; X=$'\033[0m'
fi
say() { printf '%s\n' "$1"; }

os=$(uname -s)
if [ "$os" != Linux ]; then
  say "${Y}⚠ k-prereqs supports Linux (incl. WSL2) only.${X} On $os, install kubectl/helm/kind/Docker manually."
  exit 0
fi

case "$(uname -m)" in
  x86_64|amd64)  A=amd64 ;;
  aarch64|arm64) A=arm64 ;;
  *) A=amd64; say "${Y}⚠ Unknown arch $(uname -m); assuming amd64.${X}" ;;
esac

kver=$(curl -fsSL https://dl.k8s.io/release/stable.txt 2>/dev/null || true)
[ -n "$kver" ] || kver=v1.31.0

plan=$(mktemp); trap 'rm -f "$plan"' EXIT

say "${B}${C}▌ Prerequisite check${X} ${D}(arch: $A)${X}"

if command -v kubectl >/dev/null 2>&1; then
  say "  ${G}✔${X} kubectl"
else
  say "  ${R}✖${X} kubectl ${D}(will install $kver)${X}"
  printf 'curl -fsSLo /tmp/kubectl "https://dl.k8s.io/release/%s/bin/linux/%s/kubectl"\n' "$kver" "$A" >> "$plan"
  echo  'sudo install -m 0755 /tmp/kubectl /usr/local/bin/kubectl && rm -f /tmp/kubectl' >> "$plan"
fi

if command -v helm >/dev/null 2>&1; then
  say "  ${G}✔${X} helm"
else
  say "  ${R}✖${X} helm ${D}(will install latest)${X}"
  echo 'curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash' >> "$plan"
fi

if command -v kind >/dev/null 2>&1; then
  say "  ${G}✔${X} kind"
else
  say "  ${R}✖${X} kind ${D}(will install $KIND_VERSION)${X}"
  printf 'curl -fsSLo /tmp/kind "https://kind.sigs.k8s.io/dl/%s/kind-linux-%s"\n' "$KIND_VERSION" "$A" >> "$plan"
  echo  'sudo install -m 0755 /tmp/kind /usr/local/bin/kind && rm -f /tmp/kind' >> "$plan"
fi

inst=$(cat /proc/sys/fs/inotify/max_user_instances 2>/dev/null || echo 0)
watch=$(cat /proc/sys/fs/inotify/max_user_watches 2>/dev/null || echo 0)
if { [ "$inst" -gt 0 ] && [ "$inst" -lt 512 ]; } || { [ "$watch" -gt 0 ] && [ "$watch" -lt 524288 ]; }; then
  say "  ${R}✖${X} inotify limits low ${D}(instances=$inst, watches=$watch)${X}"
  echo "echo 'fs.inotify.max_user_instances=512' | sudo tee /etc/sysctl.d/99-kind.conf"     >> "$plan"
  echo "echo 'fs.inotify.max_user_watches=524288' | sudo tee -a /etc/sysctl.d/99-kind.conf" >> "$plan"
  echo 'sudo sysctl --system' >> "$plan"
else
  say "  ${G}✔${X} inotify limits"
fi

# Docker: detect only; installing it is left to the user (distro packages + daemon + group re-login).
dock=ok
if ! command -v docker >/dev/null 2>&1; then dock=missing
elif ! docker info >/dev/null 2>&1; then dock=down; fi
case "$dock" in
  ok)      say "  ${G}✔${X} Docker" ;;
  missing) say "  ${R}✖${X} Docker ${D}(kind backend — install manually)${X}" ;;
  down)    say "  ${Y}⚠${X} Docker installed but not reachable ${D}(daemon down or missing group)${X}" ;;
esac
if [ "$dock" != ok ]; then
  id=$(. /etc/os-release 2>/dev/null && echo "${ID:-}")
  say "${D}  Docker is not auto-installed (needs distro packages + a group re-login):${X}"
  if [ "$dock" = missing ]; then
    say "    ${C}curl -fsSL https://get.docker.com | sh${X}   ${D}(distro-agnostic)${X}"
    case "$id" in
      ubuntu|debian)                       say "    ${D}apt:${X} sudo apt-get update && sudo apt-get install -y docker.io" ;;
      fedora|rhel|centos|rocky|almalinux)  say "    ${D}dnf:${X} sudo dnf install -y docker" ;;
    esac
    say "    ${C}sudo usermod -aG docker \$USER${X}   ${D}then log out/in (or: newgrp docker)${X}"
  else
    say "    ${C}sudo systemctl enable --now docker${X}   ${D}(or start Docker Desktop / enable WSL integration)${X}"
    say "    ${D}permission denied? ${X}sudo usermod -aG docker \$USER, then re-login"
  fi
fi

if [ ! -s "$plan" ]; then
  say "${G}✔ All auto-installable prerequisites are present.${X}"
  [ "$dock" = ok ] && say "${D}Next: ${X}${C}make k-up${X}" || say "${D}Resolve Docker above, then: ${X}${C}make k-up${X}"
  exit 0
fi

echo
say "${B}The following commands will run (sudo where shown):${X}"
while IFS= read -r line; do say "  ${C}${line}${X}"; done < "$plan"
echo

if [ ! -t 0 ]; then
  say "${D}(non-interactive — not running. Copy the commands above, or re-run in a terminal.)${X}"
  exit 0
fi
printf 'Proceed with the above? [y/N]: '
read -r ans </dev/tty || ans=""
case "$ans" in
  [yY]*) ;;
  *) say "Aborted — nothing installed."; exit 0 ;;
esac

say "${D}Installing...${X}"
if bash "$plan"; then
  say "${G}✔ Done.${X} Verify: ${C}kubectl version --client${X}, ${C}helm version${X}, ${C}kind version${X}"
  [ "$dock" = ok ] && say "${D}Next: ${X}${C}make k-up${X}" || say "${D}Resolve Docker above, then: ${X}${C}make k-up${X}"
else
  say "${R}✖ Some steps failed — see the output above.${X}"
  exit 1
fi
