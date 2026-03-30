#!/bin/bash

# Execute with sudo
# Required Parameters: DNS, EMAIL
# IP is auto-detected from DNS — no need to pass it manually.
# Assumes DNS A record already points to this VM's public IP before running.

SECONDS=0

DNS=${1}
EMAIL=${2}

# Helper: print a step header with elapsed time
step() {
    echo ""
    echo "[+${SECONDS}s] $1"
}

step "[Start Jitsi Video Conference Server]"
echo "(1)DNS= $DNS / (2)EMAIL= $EMAIL"

# ── Pre-flight checks ───────────────────────────────────────────────────────
echo ""
echo "=== System Requirements ==="
echo "  RAM  : 4 GB minimum (8 GB recommended)"
echo "  CPU  : 2 vCPU minimum"
echo "  Disk : 20 GB minimum"
echo "  OS   : Ubuntu 22.04 LTS"
echo "  Ports: 80/TCP, 443/TCP, 10000/UDP must be open"
echo "==========================="

TOTAL_MEM_KB=$(grep MemTotal /proc/meminfo | awk '{print $2}')
TOTAL_MEM_GB=$(echo "scale=1; $TOTAL_MEM_KB / 1024 / 1024" | bc)
MIN_MEM_KB=$((3 * 1024 * 1024))   # 3 GB — warn below this

echo ""
echo "Detected RAM: ${TOTAL_MEM_GB} GB"
if [ "$TOTAL_MEM_KB" -lt "$MIN_MEM_KB" ]; then
    echo "[Warning] Insufficient memory: ${TOTAL_MEM_GB} GB detected, 4 GB recommended."
    echo "  jitsi-videobridge2 (Java) may be killed by OOM during or after installation."
    echo "  Proceeding anyway — consider upgrading VM spec if the service fails to start."
fi

CPU_CORES=$(nproc)
echo "Detected CPU: ${CPU_CORES} vCPU(s)"
if [ "$CPU_CORES" -lt 2 ]; then
    echo "[Warning] Only ${CPU_CORES} vCPU detected. 2 vCPUs minimum recommended."
fi

DISK_FREE_GB=$(df / --output=avail -BG | tail -1 | tr -d 'G ')
echo "Detected free disk: ${DISK_FREE_GB} GB"
if [ "$DISK_FREE_GB" -lt 10 ]; then
    echo "[Warning] Low disk space: ${DISK_FREE_GB} GB free. 20 GB recommended."
fi
echo ""

if [ -z "$DNS" ]; then
    echo "[Warning] Provide your PublicDNS-RecordName (ex: xxx.cloud-barista.org)"
    echo "PublicDNS-RecordName will be the access point for JITSI Server"
    exit 1
fi

if [ -z "$EMAIL" ]; then
    echo "[Warning] Provide your E-MAIL (ex: xxx@cloudbarista.org)"
    echo "E-MAIL address will be used to issue a Certificate (https) for JITSI"
    exit 1
fi

# ── Step 1: Resolve IP ──────────────────────────────────────────────────────
# Resolve public IP from DNS record — no dependency on external HTTP services.
# DNS must already point to this VM's public IP before running this script.
# dnsutils (dig) is installed here before the main prerequisite block.
step "Resolving public IP from DNS..."
sudo apt-get update -qq 2>&1 | tail -1
sudo apt-get install -y dnsutils 2>&1 | grep -E "^(Setting up|Get:|Err:|E:)" || true

IP=$(dig +short "$DNS" A | grep -E '^[0-9]+\.' | head -1)
if [ -z "$IP" ]; then
    echo "[Error] Could not resolve IP from DNS record: $DNS"
    echo "Ensure the DNS A record is set before running this script."
    exit 1
fi
echo "Resolved public IP from DNS: $IP"

# ── Step 2: Hostname & hosts ────────────────────────────────────────────────
# Add /etc/hosts entry so Jitsi services can resolve their own hostname to
# the public IP locally. Without this, the hostname may resolve to 127.0.0.1
# (loopback), causing Jitsi's ICE candidate gathering and XMPP to fail.
# Guard against duplicate entries on re-runs.
step "Configuring hostname..."
if grep -q "$DNS" /etc/hosts; then
    sudo sed -i "/$DNS/d" /etc/hosts
fi
sudo -- sh -c "echo '$IP $DNS' >> /etc/hosts"
sudo hostnamectl set-hostname "$DNS"
hostname -f

# ── Step 3: Purge existing Jitsi ───────────────────────────────────────────
step "Removing existing Jitsi packages (if any)..."
sudo apt-get purge -y jigasi jitsi-meet jitsi-meet-web-config jitsi-meet-prosody \
    jitsi-meet-turnserver jitsi-meet-web jicofo jitsi-videobridge2 2>&1 \
    | grep -E "^(Removing|Purging|dpkg)" || true

# ── Step 4: Add all repositories first, then single apt update ─────────────
step "Installing prerequisites..."
sudo apt-get install -y apt-transport-https curl gnupg2 2>&1 \
    | grep -E "^(Setting up|Get:|Err:|E:)" || true

# Enable universe repository (required on Ubuntu)
sudo add-apt-repository universe -y > /dev/null 2>&1

# Remove needrestart to suppress interactive restart prompts during apt installs
sudo apt-get remove -y needrestart 2>&1 | grep -E "^(Removing|E:)" || true

step "Adding Prosody repository..."
sudo curl -sL https://prosody.im/files/prosody-debian-packages.key \
    -o /usr/share/keyrings/prosody-debian-packages.key
echo "deb [signed-by=/usr/share/keyrings/prosody-debian-packages.key] http://packages.prosody.im/debian $(lsb_release -sc) main" \
    | sudo tee /etc/apt/sources.list.d/prosody-debian-packages.list > /dev/null

step "Adding Jitsi repository..."
curl -sL https://download.jitsi.org/jitsi-key.gpg.key \
    | sudo sh -c 'gpg --dearmor > /usr/share/keyrings/jitsi-keyring.gpg'
echo "deb [signed-by=/usr/share/keyrings/jitsi-keyring.gpg] https://download.jitsi.org stable/" \
    | sudo tee /etc/apt/sources.list.d/jitsi-stable.list > /dev/null

# Single apt update after all repos are added (was 3 separate updates before)
step "Updating package index (once, after all repos added)..."
sudo apt-get update -qq 2>&1 | grep -E "^(Get:|Err:|E:|Hit:)" | tail -5

step "Installing lua5.2..."
sudo apt-get install -y lua5.2 2>&1 | grep -E "^(Setting up|Get:|Err:|E:)" || true

# ── Step 5: Install Jitsi ───────────────────────────────────────────────────
step "Installing jitsi-meet (this takes a few minutes)..."
# Pre-answer debconf prompts for non-interactive install.
# cert-choice: install with self-signed first, then replace with Let's Encrypt below.
echo "jitsi-videobridge2 jitsi-videobridge/jvb-hostname string $DNS" | sudo debconf-set-selections
echo "jitsi-meet-web-config jitsi-meet/cert-choice select Generate a new self-signed certificate (You will later get a chance to obtain a Let's Encrypt certificate)" | sudo debconf-set-selections
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y jitsi-meet 2>&1 \
    | grep -E "^(Setting up|Unpacking|Get:|Selecting|Preparing|E:|dpkg)" || true
if [ ${PIPESTATUS[0]} -ne 0 ]; then
    echo "[Error] jitsi-meet installation failed. Aborting."
    exit 1
fi
step "jitsi-meet installation complete."

# ── Step 6: Let's Encrypt certificate ──────────────────────────────────────
step "Installing certbot and issuing Let's Encrypt certificate (requires port 80 open and valid DNS)..."
sudo apt-get install -y certbot 2>&1 | grep -E "^(Setting up|Get:|Err:|E:)" || true
echo "$EMAIL" | sudo DEBIAN_FRONTEND=noninteractive /usr/share/jitsi-meet/scripts/install-letsencrypt-cert.sh
if [ $? -ne 0 ]; then
    echo "[Warning] Let's Encrypt certificate issuance failed."
    echo "Jitsi is running with a self-signed certificate."
    echo "Common causes: port 80/TCP not open, DNS not yet propagated, or rate limit reached (5/week per domain)."
    echo "To retry manually after resolving the issue:"
    echo "  echo '$EMAIL' | sudo /usr/share/jitsi-meet/scripts/install-letsencrypt-cert.sh"
fi

# ── Step 7: Video quality tuning ───────────────────────────────────────────
step "Applying maximum video quality settings..."
CONFIG_JS="/etc/jitsi/meet/${DNS}-config.js"

if [ ! -f "$CONFIG_JS" ]; then
    echo "[Warning] Jitsi config not found at $CONFIG_JS — skipping video quality tuning."
else
    # Back up original config
    sudo cp "$CONFIG_JS" "${CONFIG_JS}.bak"

    # resolution: set to 1080p
    sudo sed -i 's|// resolution:.*|resolution: 1080,|' "$CONFIG_JS"
    if ! grep -q "^resolution:" "$CONFIG_JS"; then
        sudo sed -i "s|^var config = {|var config = {\n    resolution: 1080,|" "$CONFIG_JS"
    fi

    # constraints: override video height/framerate
    # Inject after the resolution line if not already present
    if ! grep -q "constraints:" "$CONFIG_JS"; then
        sudo sed -i "/resolution: 1080,/a\\
    constraints: {\\
        video: {\\
            height: { ideal: 1080, max: 1080, min: 240 },\\
            frameRate: { ideal: 30, max: 30 }\\
        }\\
    }," "$CONFIG_JS"
    fi

    # startBitrate
    sudo sed -i 's|// startBitrate:.*|startBitrate: '"'"'4000'"'"',|' "$CONFIG_JS"
    if ! grep -q "startBitrate:" "$CONFIG_JS"; then
        sudo sed -i "/resolution: 1080,/a\\    startBitrate: '4000'," "$CONFIG_JS"
    fi

    # videoQuality block: inject before closing of config if not present
    if ! grep -q "videoQuality:" "$CONFIG_JS"; then
        sudo sed -i "/startBitrate:/a\\
    videoQuality: {\\
        preferredCodec: 'VP9',\\
        maxBitratesVideo: {\\
            low:      200000,\\
            standard: 1000000,\\
            high:     8000000\\
        }\\
    }," "$CONFIG_JS"
    fi

    # p2p block: enable VP9 for P2P (2-person calls bypass JVB entirely)
    sudo sed -i 's|// p2p:|p2p:|' "$CONFIG_JS"
    if ! grep -q "preferredCodec.*VP9" "$CONFIG_JS"; then
        sudo sed -i "/p2p: {/a\\        preferredCodec: 'VP9'," "$CONFIG_JS"
    fi

    echo "  Video quality settings applied to $CONFIG_JS"
    echo "  Backup saved at ${CONFIG_JS}.bak"
fi

# ── Step 8: System limits ───────────────────────────────────────────────────
step "Configuring system limits for large meetings (>100 participants)..."
# Guard against duplicate entries on re-runs.
if ! grep -q "DefaultLimitNOFILE=65000" /etc/systemd/system.conf; then
    sudo -- sh -c "echo DefaultLimitNOFILE=65000 >> /etc/systemd/system.conf"
    sudo -- sh -c "echo DefaultLimitNPROC=65000 >> /etc/systemd/system.conf"
    sudo -- sh -c "echo DefaultTasksMax=65000 >> /etc/systemd/system.conf"
else
    echo "System limits already set, skipping."
fi

# Ref: to add room password authentication
# https://www.digitalocean.com/community/tutorials/how-to-install-jitsi-meet-on-ubuntu-20-04

# ── Step 8: Restart services ────────────────────────────────────────────────
step "Restarting Jitsi services..."
sudo systemctl daemon-reload
sudo systemctl restart prosody  && echo "  prosody: restarted"
sudo systemctl restart jicofo   && echo "  jicofo:  restarted"
sudo systemctl restart jitsi-videobridge2 && echo "  jitsi-videobridge2: restarted"

# Wait for jitsi-videobridge2 to write its PID file before checking limits.
JVB_PID_FILE=/var/run/jitsi-videobridge/jitsi-videobridge.pid
echo "Waiting for jitsi-videobridge2 to start..."
for i in $(seq 1 15); do
    [ -f "$JVB_PID_FILE" ] && break
    echo "  ... waiting (${i}/15)"
    sleep 2
done

if [ -f "$JVB_PID_FILE" ]; then
    sudo cat /proc/$(sudo cat "$JVB_PID_FILE")/limits
else
    echo "[Warning] jitsi-videobridge PID file not found after 30s — check: sudo systemctl status jitsi-videobridge2"
fi

PID=$(ps -ef | grep jitsi | grep -v grep | awk '{print $2}' | tr '\n' ' ')

echo ""
echo "============================================"
echo "[Start Jitsi: complete]  elapsed: ${SECONDS}s"
echo "============================================"
echo "PID=$PID"
echo "Detected IP: $IP"
echo "Access DNS: $DNS (open in a Web browser)"
