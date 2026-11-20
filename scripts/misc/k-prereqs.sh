#!/usr/bin/env bash
#
# Check and install the prerequisites `make k-up` needs to bring up a kind-based cluster:
#   - kubectl, helm, kind, and the inotify sysctls kind requires
#   - optionally Docker + base CLI packages when --with-docker / INSTALL_DOCKER=1 is set
#
# Interactive by default (prints the plan and asks y/N).
# Non-interactive / remote bootstrap mode: pass -y / --yes (or YES=1) and --with-docker (or INSTALL_DOCKER=1).
#
# Usage:
#   make k-prereqs
#   make k-prereqs YES=1 INSTALL_DOCKER=1
set -euo pipefail

KIND_VERSION="${KIND_VERSION:-v0.32.0}"
AUTO_YES="${YES:-0}"
INSTALL_DOCKER="${INSTALL_DOCKER:-0}"

for arg in "$@"; do
  case "$arg" in
    -y|--yes)         AUTO_YES=1 ;;
    --with-docker)    INSTALL_DOCKER=1 ;;
  esac
done

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

plan=$(mktemp); trap 'rm -f "$plan"' EXIT

say "${B}${C}▌ Prerequisite check${X} ${D}(arch: $A)${X}"

# Base utilities needed on minimal cloud Ubuntu/Debian images (make, curl, git, ca-certificates)
missing_pkgs=()
for bin in make curl git; do
  command -v "$bin" >/dev/null 2>&1 || missing_pkgs+=("$bin")
done
if [ "${#missing_pkgs[@]}" -gt 0 ]; then
  say "  ${R}✖${X} base packages (${missing_pkgs[*]}) ${D}(will install)${X}"
  if command -v apt-get >/dev/null 2>&1; then
    echo "sudo DEBIAN_FRONTEND=noninteractive apt-get update -qq && sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq make curl wget git ca-certificates" >> "$plan"
  elif command -v dnf >/dev/null 2>&1; then
    echo "sudo dnf install -y make curl wget git ca-certificates" >> "$plan"
  fi
else
  say "  ${G}✔${X} base packages (make, curl, git)"
fi

kver=$(curl -fsSL https://dl.k8s.io/release/stable.txt 2>/dev/null || true)
[ -n "$kver" ] || kver=v1.31.0

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
  printf 'curl -fsSLo /tmp/kind "https://github.com/kubernetes-sigs/kind/releases/download/%s/kind-linux-%s"\n' "$KIND_VERSION" "$A" >> "$plan"
  echo  'sudo install -m 0755 /tmp/kind /usr/local/bin/kind && rm -f /tmp/kind' >> "$plan"
fi

inst=$(cat /proc/sys/fs/inotify/max_user_instances 2>/dev/null || echo 0)
watch=$(cat /proc/sys/fs/inotify/max_user_watches 2>/dev/null || echo 0)
if { [ "$inst" -gt 0 ] && [ "$inst" -lt 512 ]; } || { [ "$watch" -gt 0 ] && [ "$watch" -lt 524288 ]; }; then
  say "  ${R}✖${X} inotify limits low ${D}(instances=$inst, watches=$watch)${X}"
  echo "echo 'fs.inotify.max_user_instances=512' | sudo tee /etc/sysctl.d/99-kind.conf >/dev/null"     >> "$plan"
  echo "echo 'fs.inotify.max_user_watches=524288' | sudo tee -a /etc/sysctl.d/99-kind.conf >/dev/null" >> "$plan"
  echo 'sudo sysctl -p /etc/sysctl.d/99-kind.conf >/dev/null' >> "$plan"
else
  say "  ${G}✔${X} inotify limits"
fi

# Docker check (auto-installed when INSTALL_DOCKER=1 / --with-docker; guidance-only otherwise)
dock=ok
if ! command -v docker >/dev/null 2>&1; then dock=missing
elif ! docker info >/dev/null 2>&1; then
  if sudo -n docker info >/dev/null 2>&1; then dock=group
  else dock=down; fi
fi

if [ "$INSTALL_DOCKER" = "1" ]; then
  case "$dock" in
    ok)
      say "  ${G}✔${X} Docker" ;;
    missing)
      say "  ${R}✖${X} Docker ${D}(will install via get.docker.com + enable daemon + add $USER to docker group)${X}"
      echo 'curl -fsSL https://get.docker.com | sudo sh' >> "$plan"
      echo 'sudo systemctl enable --now docker 2>/dev/null || true' >> "$plan"
      echo 'sudo groupadd docker 2>/dev/null || true' >> "$plan"
      echo "sudo usermod -aG docker ${USER:-$(id -un)}" >> "$plan"
      dock=ok ;;
    group)
      say "  ${Y}⚠${X} Docker group not active for ${USER:-$(id -un)} ${D}(will ensure group membership)${X}"
      echo 'sudo groupadd docker 2>/dev/null || true' >> "$plan"
      echo "sudo usermod -aG docker ${USER:-$(id -un)}" >> "$plan"
      dock=ok ;;
    down)
      say "  ${Y}⚠${X} Docker daemon stopped ${D}(will start docker service)${X}"
      echo 'sudo systemctl enable --now docker' >> "$plan"
      echo 'sudo groupadd docker 2>/dev/null || true' >> "$plan"
      echo "sudo usermod -aG docker ${USER:-$(id -un)}" >> "$plan"
      dock=ok ;;
  esac
else
  case "$dock" in
    ok)      say "  ${G}✔${X} Docker" ;;
    missing) say "  ${R}✖${X} Docker ${D}(kind backend — install manually or pass INSTALL_DOCKER=1)${X}" ;;
    group|down) say "  ${Y}⚠${X} Docker installed but not reachable ${D}(daemon down or missing group)${X}" ;;
  esac
  if [ "$dock" != ok ]; then
    id=$(. /etc/os-release 2>/dev/null && echo "${ID:-}")
    say "${D}  Docker is not auto-installed by default (pass INSTALL_DOCKER=1 to auto-install):${X}"
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
fi

if [ ! -s "$plan" ]; then
  say "${G}✔ All requested prerequisites are present.${X}"
  [ "$dock" = ok ] && say "${D}Next: ${X}${C}make k-up${X}" || say "${D}Resolve Docker above, then: ${X}${C}make k-up${X}"
  exit 0
fi

echo
say "${B}The following commands will run (sudo where shown):${X}"
while IFS= read -r line; do say "  ${C}${line}${X}"; done < "$plan"
echo

if [ "$AUTO_YES" != "1" ]; then
  if [ ! -t 0 ]; then
    say "${D}(non-interactive — pass YES=1 or -y to run automatically.)${X}"
    exit 0
  fi
  printf 'Proceed with the above? [y/N]: '
  read -r ans </dev/tty || ans=""
  case "$ans" in
    [yY]*) ;;
    *) say "Aborted — nothing installed."; exit 0 ;;
  esac
fi

say "${D}Installing...${X}"
if bash "$plan"; then
  say "${G}✔ Done.${X} Verify: ${C}kubectl version --client${X}, ${C}helm version${X}, ${C}kind version${X}"
  [ "$dock" = ok ] && say "${D}Next: ${X}${C}make k-up${X}" || say "${D}Resolve Docker above, then: ${X}${C}make k-up${X}"
else
  say "${R}✖ Some steps failed — see the output above.${X}"
  exit 1
fi
