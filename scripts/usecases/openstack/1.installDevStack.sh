#!/bin/bash

# DevStack Installation Script for AWS m5.metal (or bare-metal) Instances
# Installs a single-node OpenStack (DevStack) environment for CB-Tumblebug integration testing.
#
# Prerequisites:
#   - Ubuntu 22.04 or 24.04 on a bare-metal instance (e.g., AWS m5.metal)
#   - At least 8 GiB RAM, 50 GiB disk (m5.metal far exceeds this)
#   - Internet access for package downloads
#
# Usage:
#   ./1.installDevStack.sh [--csp-name CSP_NAME] [--password ADMIN_PASSWORD] [--branch OPENSTACK_BRANCH] [--latitude LAT] [--longitude LON] [--location DISPLAY]
#
# Parameters:
#   --csp-name  CSP provider name for CB-Tumblebug registration (default: openstack-devstack)
#   --password  OpenStack admin password (default: cbtumblebug)
#   --branch    DevStack branch to install (default: stable/2025.2)
#   --latitude  Latitude for location info (default: 0)
#   --longitude Longitude for location info (default: 0)
#   --location  Display name for location (default: DevStack)
#
# After completion, run ./2.getRegistrationInfo.sh to get CB-Tumblebug registration details.

set -e

# ============================================================
# Non-interactive mode for SSH remote execution
# ============================================================
export DEBIAN_FRONTEND=noninteractive
export NEEDRESTART_MODE=a
export NEEDRESTART_SUSPEND=1

# Auto-restart services after package upgrades, but never sshd: this script runs
# inside an SSH session, and restarting ssh.service kills it mid-install.
if [ -d /etc/needrestart/conf.d ]; then
    sudo tee /etc/needrestart/conf.d/99-autorestart.conf > /dev/null 2>&1 << 'NRCONF' || true
$nrconf{restart} = 'a';
$nrconf{override_rc} = {
    qr(^ssh\.service$)  => 0,
    qr(^sshd\.service$) => 0,
};
NRCONF
fi

# ============================================================
# Parse arguments
# ============================================================
ADMIN_PASSWORD="cbtumblebug"
OPENSTACK_BRANCH="stable/2025.2"
CSP_NAME="openstack-devstack"
LOCATION_LATITUDE="0"
LOCATION_LONGITUDE="0"
LOCATION_DISPLAY="DevStack"

while [[ "$1" != "" ]]; do
    case $1 in
        --csp-name )  shift; CSP_NAME=$1 ;;
        --password )  shift; ADMIN_PASSWORD=$1 ;;
        --branch )    shift; OPENSTACK_BRANCH=$1 ;;
        --latitude )  shift; LOCATION_LATITUDE=$1 ;;
        --longitude ) shift; LOCATION_LONGITUDE=$1 ;;
        --location )  shift; LOCATION_DISPLAY=$1 ;;
        * )           echo "Usage: $0 [--csp-name CSP_NAME] [--password ADMIN_PASSWORD] [--branch OPENSTACK_BRANCH] [--latitude LAT] [--longitude LON] [--location DISPLAY]"; exit 1 ;;
    esac
    shift
done

echo "============================================================"
echo " DevStack Installation for CB-Tumblebug Integration"
echo "============================================================"
echo " CSP Name       : $CSP_NAME"
echo " Admin Password : $ADMIN_PASSWORD"
echo " Branch         : $OPENSTACK_BRANCH"
echo "============================================================"

# ============================================================
# Run state - make this script safe to re-run
# ============================================================
# The caller (CB-Tumblebug remote command) re-runs the whole command when the SSH
# transport drops. On a 20-40 minute install that lands on top of a run which is
# still going, or has already finished, and a second stack.sh wrecks the first one.
# These markers make a repeat invocation attach to the running install, or skip
# straight to reporting, instead of starting a rival stack.sh.
RUN_LOG=/opt/stack/stack.run.log
RUN_EXIT=/opt/stack/stack.run.exit
RUN_PID=/opt/stack/stack.run.pid
DONE_MARKER=/opt/stack/stack.run.done

stack_is_running() {
    local pid
    pid=$(sudo cat "$RUN_PID" 2>/dev/null) || return 1
    [ -n "$pid" ] || return 1
    # Match on the command line too: a bare kill -0 would be fooled by pid reuse
    # after a reboot, and then we would wait forever on an unrelated process.
    sudo ps -p "$pid" -o args= 2>/dev/null | grep -q 'run-stack\.sh'
}

RUN_MODE=install
if sudo test -f "$DONE_MARKER" 2>/dev/null; then
    RUN_MODE=report
elif stack_is_running; then
    RUN_MODE=attach
fi

# Host addresses are needed by every mode (local.conf, endpoint rewrite, output).
HOST_IP=$(hostname -I | awk '{print $1}')
PUBLIC_IP=$(curl -s --connect-timeout 5 http://169.254.169.254/latest/meta-data/public-ipv4 2>/dev/null || \
            curl -s --connect-timeout 5 https://api.ipify.org 2>/dev/null || \
            echo "$HOST_IP")

case "$RUN_MODE" in
    report)
        echo " This host already has a completed DevStack install; reporting only."
        # The snippets from the original run were saved; show them straight away so a
        # caller whose earlier session dropped can recover them without waiting for the
        # CSP round-trips below to finish.
        if sudo test -f /opt/stack/cb-registration-info.txt 2>/dev/null; then
            echo ""
            echo "============================================================"
            echo " Saved registration info from the original run"
            echo "============================================================"
            sudo cat /opt/stack/cb-registration-info.txt
            echo ""
        fi
        ;;
    attach) echo " A stack.sh run is already in progress (pid $(sudo cat "$RUN_PID")); attaching to it." ;;
esac

# ============================================================
# Wait for apt locks (in case cloud-init is still running)
# ============================================================
wait_for_apt() {
    local max_wait=120
    local waited=0
    while sudo fuser /var/lib/dpkg/lock-frontend >/dev/null 2>&1 || \
          sudo fuser /var/lib/apt/lists/lock >/dev/null 2>&1; do
        if [ $waited -ge $max_wait ]; then
            echo "Warning: apt lock still held after ${max_wait}s, proceeding..."
            break
        fi
        echo "Waiting for apt lock... ($waited/${max_wait}s)"
        sleep 5
        waited=$((waited + 5))
    done
}

wait_for_apt

# ============================================================
# Retry helper for transient network failures
# ============================================================
retry() {
    local max_attempts=3
    local delay=15
    local attempt=1
    while [ $attempt -le $max_attempts ]; do
        if "$@"; then
            return 0
        fi
        echo "Attempt $attempt/$max_attempts failed. Retrying in ${delay}s..."
        sleep $delay
        attempt=$((attempt + 1))
    done
    echo "ERROR: Command failed after $max_attempts attempts: $*"
    return 1
}

# ============================================================
# Steps [Pre-flight] through [4/5] only apply to a fresh install. In attach/report
# mode the host is already prepared, so they are skipped (see "Run state" above).
# ============================================================
if [ "$RUN_MODE" = "install" ]; then

# ============================================================
# Pre-flight checks
# ============================================================
echo ""
echo "[Pre-flight] Checking system requirements..."

# Check available disk space (minimum 50 GiB recommended for DevStack)
MIN_DISK_GB=50
AVAILABLE_GB=$(df --output=avail / | tail -1 | awk '{printf "%d", $1/1024/1024}')
echo "  Disk available: ${AVAILABLE_GB} GiB (minimum: ${MIN_DISK_GB} GiB)"
if [ "$AVAILABLE_GB" -lt "$MIN_DISK_GB" ]; then
    echo "ERROR: Insufficient disk space. DevStack requires at least ${MIN_DISK_GB} GiB free."
    echo "       Current available: ${AVAILABLE_GB} GiB on /"
    echo "       Consider using a larger root volume (100 GiB+ recommended)."
    exit 1
fi

# Check available memory (minimum 8 GiB recommended)
MIN_MEM_GB=8
AVAILABLE_MEM_GB=$(free -g | awk '/^Mem:/{print $2}')
echo "  Memory total  : ${AVAILABLE_MEM_GB} GiB (minimum: ${MIN_MEM_GB} GiB)"
if [ "$AVAILABLE_MEM_GB" -lt "$MIN_MEM_GB" ]; then
    echo "ERROR: Insufficient memory. DevStack requires at least ${MIN_MEM_GB} GiB RAM."
    echo "       Current total: ${AVAILABLE_MEM_GB} GiB"
    exit 1
fi

# Check OS version
OS_ID=$(. /etc/os-release && echo "$ID")
OS_VERSION=$(. /etc/os-release && echo "$VERSION_ID")
echo "  OS            : ${OS_ID} ${OS_VERSION}"
if [[ "$OS_ID" != "ubuntu" ]] || [[ "$OS_VERSION" != "22.04" && "$OS_VERSION" != "24.04" ]]; then
    echo "WARNING: DevStack is tested on Ubuntu 22.04/24.04. Current: ${OS_ID} ${OS_VERSION}"
    echo "         Proceeding, but issues may occur."
fi

echo "  All pre-flight checks passed."

# ============================================================
# Step 1: System preparation
# ============================================================
echo ""
echo "[1/5] Updating system packages..."
retry sudo apt-get update -qq

# Hold the OpenSSH packages across the upgrade. Their postinst restarts ssh.service,
# which severs the SSH session this script runs in; the caller then sees a transport
# error and may re-run the whole command on top of a live install.
SSH_PKGS="openssh-server openssh-client openssh-sftp-server"
sudo apt-mark hold $SSH_PKGS > /dev/null 2>&1 || true
retry sudo DEBIAN_FRONTEND=noninteractive apt-get upgrade -y -qq \
    -o Dpkg::Options::="--force-confdef" \
    -o Dpkg::Options::="--force-confold"
sudo apt-mark unhold $SSH_PKGS > /dev/null 2>&1 || true

echo "Installing prerequisites..."
retry sudo apt-get install -y -qq git python3-pip python3-venv net-tools curl jq

# ============================================================
# Step 2: Create stack user (DevStack requirement)
# ============================================================
echo ""
echo "[2/5] Setting up 'stack' user..."

if ! id "stack" &>/dev/null; then
    sudo useradd -s /bin/bash -d /opt/stack -m stack
    sudo chmod +x /opt/stack
fi

# Grant passwordless sudo
echo "stack ALL=(ALL) NOPASSWD: ALL" | sudo tee /etc/sudoers.d/stack > /dev/null

# ============================================================
# Step 3: Clone DevStack
# ============================================================
echo ""
echo "[3/5] Cloning DevStack ($OPENSTACK_BRANCH)..."

# Configure git for reliable large-repo cloning over GnuTLS.
# Ubuntu ships git linked against GnuTLS (not OpenSSL), which is more sensitive to
# server-side TLS anomalies on opendev.org (HAProxy backend failovers, etc.).
# - http.version HTTP/1.1: avoids curl 56 (HTTP/2 GOAWAY frames mishandled by GnuTLS).
# - http.postBuffer: increases curl send buffer to 500 MiB for large pack transfers.
# Note: http.lowSpeedLimit/lowSpeedTime are intentionally NOT set.
#   GIT_TIMEOUT=300 in local.conf lets DevStack's timeout(1) handle stalled connections,
#   and setting lowSpeedLimit would trigger curl 28 on temporarily slow connections
#   (nova is a large repo; brief throughput dips below any fixed threshold are common).
sudo -u stack git config --global http.version HTTP/1.1
sudo -u stack git config --global http.postBuffer 524288000

# Install a git wrapper at /usr/local/bin/git (takes PATH precedence over /usr/bin/git).
# This intercepts every network-facing git call — including those fired by stack.sh's
# internal git_timed() — and retries transient failures (curl 35, curl 56, exit 128).
#
# Why every network subcommand and not just clone: DevStack's git_clone() always runs
# 'git fetch origin <ref>' right after cloning, and its git_timed() only retries exit
# 124 (timeout) — exit 128 goes straight to die(). opendev.org intermittently answers
# /info/refs with a non-git response ("could not determine hash algorithm; is this a
# git repository?"), which is exit 128, so a single bad response aborts the install.
sudo tee /usr/local/bin/git > /dev/null << 'GITWRAP'
#!/bin/bash
# Always point to the real git binary, never to this wrapper itself.
# Using $(which git) would cause infinite recursion on re-runs of this script.
REAL_GIT="/usr/bin/git"

# Report a timeout to the caller if we are interrupted (DevStack wraps git calls in
# 'timeout -s SIGINT $GIT_TIMEOUT' and retries exit 124 on its own).
trap 'exit 124' INT TERM

# Options that consume the next argument as their value; skipped when looking for
# the subcommand and when locating a clone's destination directory.
opt_takes_value() {
    case "$1" in
        -C|-c|--git-dir|--work-tree|--namespace|--exec-path|\
        -b|--branch|-o|--origin|-u|--upload-pack|--reference|--depth|\
        --shallow-since|--shallow-exclude|-j|--jobs|--filter|--recurse-submodules)
            return 0 ;;
    esac
    return 1
}

# Collect positional args (subcommand first) so both the dispatch below and the
# clone destination lookup see the same view of the command line.
positional=()
skip_next=false
for arg in "$@"; do
    if $skip_next; then skip_next=false; continue; fi
    if opt_takes_value "$arg"; then skip_next=true; continue; fi
    case "$arg" in
        -*) ;;
        *) positional+=("$arg") ;;
    esac
done
subcmd="${positional[0]}"

case "$subcmd" in
    clone|fetch|pull|ls-remote|remote) ;;  # network operations: retry
    *) exec "$REAL_GIT" "$@" ;;            # everything else: pass through
esac

# Identify a clone's destination directory so a partial clone can be cleared
# before retrying. git clone syntax: git clone [options] <repository> [<directory>]
# positional[0] = 'clone', [1] = repository, [2] = optional directory.
# Safety: only remove the directory if it contains a .git entry, i.e. it really is
# a partial clone and not an unrelated directory that happens to share the name.
dest_dir=""
if [ "$subcmd" = "clone" ]; then
    if [ ${#positional[@]} -ge 3 ]; then
        dest_dir="${positional[2]}"
    elif [ ${#positional[@]} -ge 2 ]; then
        dest_dir="$(basename "${positional[1]}" .git)"
    fi
fi

# Exponential backoff: the observed opendev.org outages lasted longer than the
# flat 3x30s this wrapper used to allow.
delays=(10 20 40 60)
max_attempts=$(( ${#delays[@]} + 1 ))
attempt=1
while :; do
    "$REAL_GIT" "$@" && exit 0
    exit_code=$?
    [ $attempt -ge $max_attempts ] && break
    delay="${delays[$((attempt - 1))]}"
    echo "git $subcmd failed (attempt $attempt/$max_attempts, exit: $exit_code). Retrying in ${delay}s..." >&2
    if [ -n "$dest_dir" ] && [ -d "$dest_dir" ] && [ -e "$dest_dir/.git" ]; then
        rm -rf "$dest_dir"
    fi
    sleep "$delay"
    attempt=$((attempt + 1))
done
exit $exit_code
GITWRAP
sudo chmod +x /usr/local/bin/git

retry sudo -u stack bash -c "OPENSTACK_BRANCH='$OPENSTACK_BRANCH'
    cd /opt/stack
    if [ -d devstack ]; then
        echo 'DevStack directory exists, pulling latest...'
        cd devstack && git checkout \"\$OPENSTACK_BRANCH\" && git pull
    else
        git clone https://opendev.org/openstack/devstack -b \"\$OPENSTACK_BRANCH\"
    fi
"

# ============================================================
# Step 4: Configure DevStack (local.conf)
# ============================================================
echo ""
echo "[4/5] Generating local.conf..."

sudo -u stack bash -c "cat > /opt/stack/devstack/local.conf << 'LOCALCONF'
[[local|localrc]]
# -------------------------------------------------------
# Credentials (use only alphanumeric characters)
# -------------------------------------------------------
ADMIN_PASSWORD=${ADMIN_PASSWORD}
DATABASE_PASSWORD=\${ADMIN_PASSWORD}
RABBIT_PASSWORD=\${ADMIN_PASSWORD}
SERVICE_PASSWORD=\${ADMIN_PASSWORD}

# -------------------------------------------------------
# Host configuration
# -------------------------------------------------------
HOST_IP=${HOST_IP}

# Note: Do NOT set SERVICE_HOST to Public IP on AWS.
# AWS public IPs are NAT'd and not bound to local interfaces,
# so services (e.g., etcd) cannot bind to them.
# The registration script (2.getRegistrationInfo.sh) handles
# rewriting internal IPs to public IPs in the output snippets.

# Always re-clone source repos. This doubles the number of opendev.org requests,
# but it is what makes a re-run self-healing: a run that dies mid-clone leaves a
# repo with an unborn HEAD, and with RECLONE=no DevStack keeps that directory and
# fails later on a missing file (e.g. requirements/upper-constraints.txt).
# Transient opendev.org failures are handled by the git wrapper above instead.
RECLONE=yes

# -------------------------------------------------------
# Git clone timeout and retry
# -------------------------------------------------------
# GIT_TIMEOUT: per-operation timeout in seconds.
# DevStack's git_timed() retries up to 3x on timeout (exit 124).
# Default is 0 (no timeout = no retry). Setting a value activates
# the built-in retry mechanism for slow/stalled connections.
GIT_TIMEOUT=300

# -------------------------------------------------------
# Disable Tempest (test framework, not needed for operation)
# -------------------------------------------------------
disable_service tempest

# -------------------------------------------------------
# Hypervisor - use KVM on bare-metal
# -------------------------------------------------------
LIBVIRT_TYPE=kvm

# -------------------------------------------------------
# Guest image - Ubuntu cloud image for VM provisioning
# -------------------------------------------------------
IMAGE_URLS=\"https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-amd64.img\"

# -------------------------------------------------------
# Logging
# -------------------------------------------------------
LOGFILE=/opt/stack/logs/stack.sh.log
LOG_COLOR=False
LOGDAYS=1

# -------------------------------------------------------
# Cinder volume size
# -------------------------------------------------------
VOLUME_BACKING_FILE_SIZE=50G

# -------------------------------------------------------
# Octavia (Load Balancer) - required by CB-Spider NLBClient
# gophercloud v2 service catalog type: "load-balancer"
# -------------------------------------------------------
enable_plugin octavia https://opendev.org/openstack/octavia ${OPENSTACK_BRANCH}

# -------------------------------------------------------
# Manila (Shared File System) - required by CB-Spider SharedFileSystemClient
# gophercloud v2 service catalog type: "shared-file-system" (alias: "sharev2")
# -------------------------------------------------------
enable_plugin manila https://opendev.org/openstack/manila ${OPENSTACK_BRANCH}
LOCALCONF
"

echo "Generated local.conf with HOST_IP=$HOST_IP"

fi  # end of "$RUN_MODE" = install

# ============================================================
# Step 5: Run DevStack installation (git retries via the wrapper above)
# ============================================================
# stack.sh is started detached (setsid, stdio on files) and then followed from here.
# If the SSH session dies mid-install the install carries on instead of wedging on a
# dead pipe, and a later invocation attaches to it rather than starting a rival run.
start_stack() {
    sudo -u stack tee /opt/stack/run-stack.sh > /dev/null << RUNNER
#!/bin/bash
# PATH is prepended so the git retry wrapper takes precedence over /usr/bin/git even
# when sudo resets the environment (Ubuntu default: env_reset in /etc/sudoers).
export PATH=/usr/local/bin:\$PATH
cd /opt/stack/devstack || exit 1
echo \$\$ > $RUN_PID
rm -f $RUN_EXIT
./stack.sh > $RUN_LOG 2>&1 < /dev/null
echo \$? > $RUN_EXIT
RUNNER
    sudo chmod +x /opt/stack/run-stack.sh
    sudo -u stack rm -f "$RUN_EXIT" "$RUN_PID"
    sudo -u stack setsid /opt/stack/run-stack.sh < /dev/null > /dev/null 2>&1 &
    # Wait for the runner to publish its pid, so stack_is_running() is meaningful.
    for _ in $(seq 1 20); do
        if stack_is_running; then break; fi
        sleep 1
    done
}

follow_stack() {
    sudo -u stack touch "$RUN_LOG"
    sudo tail -n +1 -F "$RUN_LOG" &
    local tail_pid=$!
    while [ ! -f "$RUN_EXIT" ]; do
        if ! stack_is_running; then
            # Runner is gone; give it a moment to record its exit code.
            sleep 5
            break
        fi
        sleep 10
    done
    sleep 3
    # The follower runs under sudo, so it belongs to root and this script cannot signal it:
    # a plain kill fails silently, the wait below never returns, and the caller sits on a
    # finished install until its timeout (measured: stack.sh done in 998 s, the command
    # still running 40 minutes later). Signal it as root, children first.
    sudo pkill -P "$tail_pid" 2>/dev/null || true
    sudo kill "$tail_pid" 2>/dev/null || true
    wait "$tail_pid" 2>/dev/null || true
}

if [ "$RUN_MODE" = "report" ]; then
    STACK_EXIT=0
else
    echo ""
    echo "[5/5] Running stack.sh (this takes 20-40 minutes with Octavia/Manila)..."
    echo "      Logs: /opt/stack/logs/stack.sh.log"

    if [ "$RUN_MODE" = "install" ]; then
        start_stack
    fi
    follow_stack

    STACK_EXIT=$(sudo cat "$RUN_EXIT" 2>/dev/null || echo 1)
    if [ "$STACK_EXIT" = "0" ]; then
        sudo -u stack touch "$DONE_MARKER"
    fi
fi

echo ""
echo "============================================================"
if [ $STACK_EXIT -eq 0 ]; then
    echo " DevStack installation COMPLETED successfully!"
    echo "============================================================"

    # ----------------------------------------------------------
    # Gather registration info for CB-Tumblebug
    # ----------------------------------------------------------
    source /opt/stack/devstack/openrc admin admin 2>/dev/null

    # Every openstack call is capped: the catalog is rewritten to the Public IP further
    # down, and openstackclient follows the catalog after authenticating, so on a re-run
    # against a host whose port 80 is closed each call would otherwise block forever.
    os() { timeout 60 openstack "$@"; }

    # Check reachability first — it needs no CLI, and it explains any missing values
    # below when the catalog is already public and the port is shut.
    API_REACHABLE=yes
    if ! timeout 10 curl -s -o /dev/null "http://${PUBLIC_IP}/identity/v3" 2>/dev/null; then
        API_REACHABLE=no
    fi
    if [ "$API_REACHABLE" = "no" ] && [ "$RUN_MODE" = "report" ]; then
        echo ""
        echo " NOTE: the service catalog already points at ${PUBLIC_IP}, which is not"
        echo "       reachable, so the details below may be incomplete. See the warning"
        echo "       at the end for how to open the port."
    fi

    PROJECT_ID=$(os project show admin -f value -c id 2>/dev/null || echo "UNKNOWN")
    REGION=$(os region list -f value -c Region 2>/dev/null | head -1 || echo "RegionOne")
    AZ=$(os availability zone list --compute -f value -c "Zone Name" 2>/dev/null | grep -v "^internal$" | head -1 || echo "nova")

    # Everything the caller actually needs is known now, so print it BEFORE the catalog
    # rewrite and the service checks. Those steps talk to the CSP and can stall or drop the
    # SSH session; when that happened the registration snippets were lost with it, even
    # though the install had succeeded. Printing first means a dropped connection costs
    # progress messages, not the one output that is hard to reconstruct.
    #
    # It is written to a file as well: a remote command that loses its channel loses its
    # stdout entirely, and this is the only way to get the snippets back without re-running
    # anything.
    REGISTRATION_INFO=/opt/stack/cb-registration-info.txt
    { echo ""
    echo " Horizon Dashboard  : http://${PUBLIC_IP}/dashboard"
    echo " Keystone Auth URL  : http://${PUBLIC_IP}/identity/v3$([ "$API_REACHABLE" = "no" ] && echo '   <-- port 80 CLOSED, see warning above')"
    echo " Username / Password: admin / ${ADMIN_PASSWORD}"
    echo " Project ID         : ${PROJECT_ID}"
    echo " Region / AZ        : ${REGION} / ${AZ}"
    echo ""

    echo "============================================================"
    echo " credentials.yaml snippet"
    echo "============================================================"
    cat << CRED_EOF

    ${CSP_NAME}:
      IdentityEndpoint: http://${PUBLIC_IP}/identity/v3
      Username: admin
      Password: ${ADMIN_PASSWORD}
      DomainName: Default
      ProjectID: ${PROJECT_ID}

CRED_EOF

    echo "============================================================"
    echo " cloudinfo.yaml snippet"
    echo "============================================================"
    cat << CLOUD_EOF

  ${CSP_NAME}:
    description: DevStack (${PUBLIC_IP})
    cloudPlatform: openstack
    driver: openstack-driver-v1.0.so
    region:
      ${REGION}:
        id: ${REGION}
        description: DevStack ${REGION}
        location:
          display: ${LOCATION_DISPLAY}
          latitude: ${LOCATION_LATITUDE}
          longitude: ${LOCATION_LONGITUDE}
        zone:
        - ${AZ}

CLOUD_EOF

    echo "============================================================"
    echo " Next Steps"
    echo "============================================================"
    echo ""
    echo " 1. Add the snippets above to:"
    echo "    - ~/.cloud-barista/credentials.yaml  (credentials)"
    echo "    - cb-tumblebug/assets/cloudinfo.yaml  (cloud info)"
    echo ""
    echo " 2. Re-initialize CB-Tumblebug:"
    echo "    make enc-cred && make init"
    echo ""
    echo " For detailed info, run: ./2.getRegistrationInfo.sh"
    echo "\$\$ENDPOINT[Horizon Dashboard](http://0.0.0.0/dashboard)"
    echo "\$\$CREDENTIAL[Admin Login](admin / ${ADMIN_PASSWORD})"
    } 2>&1 | sudo tee "$REGISTRATION_INFO"
    sudo chmod 644 "$REGISTRATION_INFO" 2>/dev/null || true

    echo ""
    echo " (the block above is saved on this host at $REGISTRATION_INFO)"

    # ----------------------------------------------------------
    # Verify CB-Spider required services in service catalog
    # CB-Spider's OpenStack driver (gophercloud v2) requires
    # these service types during connection initialization:
    #   NewLoadBalancerV2()      -> type "load-balancer"
    #   NewBlockStorageV3()      -> type "block-storage" (aliases: "volumev3", "volumev2", "volume")
    #   NewSharedFileSystemV2()  -> type "shared-file-system" (aliases: "sharev2", "share")
    #
    # gophercloud v2 uses ServiceTypeAliases, so:
    #   - Cinder "block-storage" is matched directly (no alias entry needed)
    #   - Manila "sharev2" or "shared-file-system" both work
    #   - Octavia/Manila are optional plugins; create placeholders if not installed
    # ----------------------------------------------------------
    echo ""
    echo " Verifying CB-Spider required services..."
    PLACEHOLDER_CREATED=0

    # A previous run may already have registered a placeholder. Reporting that as
    # "installed" hides the fact that the service does not exist, so look at the
    # endpoint URL rather than only at the service type.
    service_is_placeholder() {
        os endpoint list --service "$1" -f value -c URL 2>/dev/null | grep -q '/placeholder/'
    }

    # Cinder (Block Storage) - gophercloud v2 type: "block-storage"
    # gophercloud v2 ServiceTypeAliases: "block-storage" -> ["volumev3", "volumev2", "volume", "block-store"]
    # No alias entry needed; "block-storage" is matched directly.
    if os service list -f value -c Type 2>/dev/null | grep -qE "^(block-storage|volumev3)$"; then
        echo "   ✓ block-storage (Cinder) - installed"
    else
        echo "   ✗ block-storage (Cinder) - NOT found"
        echo "     WARNING: Cinder is not available. Disk operations will fail."
    fi

    # Octavia (Load Balancer) - gophercloud v2 type: "load-balancer"
    if os service list -f value -c Type 2>/dev/null | grep -q "^load-balancer$"; then
        if service_is_placeholder load-balancer; then
            echo "   ⚠ load-balancer - PLACEHOLDER only, Octavia is not installed"
            echo "     CB-Spider will initialize, but every NLB call against it fails."
            echo "     To install it for real, add to local.conf next to enable_plugin octavia:"
            echo "       enable_service octavia o-api o-cw o-hm o-hk o-da"
            echo "     (Octavia's plugin does not enable its own services; Manila's does.)"
        else
            echo "   ✓ load-balancer (Octavia) - installed"
        fi
    else
        echo "   ✗ load-balancer (Octavia) - NOT found, creating placeholder..."
        os service create --name octavia --description "Load Balancer (placeholder for CB-Spider)" load-balancer && \
        os endpoint create --region "$REGION" load-balancer public "http://${PUBLIC_IP}/placeholder/load-balancer/v2.0" && \
        PLACEHOLDER_CREATED=$((PLACEHOLDER_CREATED + 1))
    fi

    # Manila (Shared File System) - gophercloud v2 type: "shared-file-system" (alias: "sharev2")
    if os service list -f value -c Type 2>/dev/null | grep -qE "^(shared-file-system|sharev2)$"; then
        if service_is_placeholder shared-file-system; then
            echo "   ⚠ shared-file-system - PLACEHOLDER only, Manila is not installed"
        else
            echo "   ✓ shared-file-system (Manila) - installed"
        fi
    else
        echo "   ✗ shared-file-system (Manila) - NOT found, creating placeholder..."
        os service create --name manilav2 --description "Shared File System (placeholder for CB-Spider)" shared-file-system && \
        os endpoint create --region "$REGION" shared-file-system public "http://${PUBLIC_IP}/placeholder/shared-file-system/v2" && \
        PLACEHOLDER_CREATED=$((PLACEHOLDER_CREATED + 1))
    fi

    if [ $PLACEHOLDER_CREATED -gt 0 ]; then
        echo "   ⚠ Created $PLACEHOLDER_CREATED placeholder(s) - plugin install may have failed"
    fi

    # Update all service catalog endpoints to use Public IP
    # so external clients (CB-Spider) can reach them.
    INTERNAL_IP="$HOST_IP"
    CHANGED=0
    # List once up front: the per-endpoint 'show' calls this used to make would other-
    # wise run against a catalog that is being rewritten underneath them. Identity is
    # rewritten last for the same reason.
    ALL_ENDPOINTS=$(os endpoint list -f value -c ID -c "Service Type" -c URL 2>/dev/null || true)

    rewrite_endpoint() {
        local eid="$1" eurl="$2" new_url
        case "$eurl" in *"$INTERNAL_IP"*) ;; *) return 0 ;; esac
        new_url=$(echo "$eurl" | sed "s/$INTERNAL_IP/$PUBLIC_IP/g")
        if os endpoint set --url "$new_url" "$eid" 2>/dev/null; then
            CHANGED=$((CHANGED + 1))
        fi
    }

    identity_endpoints=()
    while read -r eid etype eurl; do
        [ -n "$eid" ] || continue
        if [ "$etype" = "identity" ]; then
            identity_endpoints+=("$eid $eurl")
            continue
        fi
        rewrite_endpoint "$eid" "$eurl"
    done <<< "$ALL_ENDPOINTS"

    # Identity last — nothing below needs the catalog entry we are about to move.
    for entry in "${identity_endpoints[@]}"; do
        rewrite_endpoint "${entry%% *}" "${entry#* }"
    done

    if [ $CHANGED -gt 0 ]; then
        echo ""
        echo " Updated $CHANGED service catalog endpoint(s): $INTERNAL_IP -> $PUBLIC_IP"
    fi

    # A public catalog is only useful if the port is actually open. Since CB-Tumblebug
    # 0.12.30 the default security group opens TCP 22 only, so the rewrite above
    # silently produces a cloud that no external client can reach. Say so plainly.
    if [ "$API_REACHABLE" = "no" ]; then
        echo ""
        echo " WARNING: http://${PUBLIC_IP}/identity/v3 is NOT reachable from outside."
        echo "          The OpenStack API is served on TCP 80, but this Node's security"
        echo "          group allows only SSH (the default since CB-Tumblebug 0.12.30)."
        echo "          CB-Spider cannot use this cloud until that port is opened:"
        echo ""
        echo "            POST /tumblebug/ns/{nsId}/resources/securityGroup/{sgId}/rules"
        echo "            {\"firewallRules\":[{\"ports\":\"80\",\"protocol\":\"TCP\",\"direction\":\"inbound\",\"cidr\":\"<your-cidr>\"}]}"
        echo ""
        echo "          Or create the Infra with sgTemplateId set to a template that opens it."
    fi


else
    echo " DevStack installation FAILED (exit code: $STACK_EXIT)"
    echo " Check logs: /opt/stack/logs/stack.sh.log (runner output: $RUN_LOG)"
    echo "============================================================"
    echo ""
    echo " Reported error:"
    sudo cat /opt/stack/logs/error.log 2>/dev/null | tail -5 || true
    echo ""
    echo " Re-run this script to resume; it will skip the host preparation steps"
    echo " and will not start a second stack.sh while one is still running."
    exit 1
fi
