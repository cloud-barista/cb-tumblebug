#!/bin/bash

# Execute with sudo
# Required Parameters: DNS, EMAIL
# IP is auto-detected from DNS — no need to pass it manually.
# Assumes DNS A record already points to this VM's public IP before running.
echo "[Start Jitsi Video Conference Server]"

SECONDS=0

DNS=${1}
EMAIL=${2}

echo "(1)DNS= $DNS / (2)EMAIL= $EMAIL"

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

# Resolve public IP from DNS record — no dependency on external HTTP services.
# DNS must already point to this VM's public IP before running this script.
# dnsutils (dig) is installed here before the main prerequisite block.
sudo apt update -qq &> /dev/null
sudo apt install -y dnsutils &> /dev/null

IP=$(dig +short "$DNS" A | grep -E '^[0-9]+\.' | head -1)
if [ -z "$IP" ]; then
    echo "[Error] Could not resolve IP from DNS record: $DNS"
    echo "Ensure the DNS A record is set before running this script."
    exit 1
fi
echo "Resolved public IP from DNS: $IP"

# Add /etc/hosts entry so Jitsi services can resolve their own hostname to
# the public IP locally. Without this, the hostname may resolve to 127.0.0.1
# (loopback), causing Jitsi's ICE candidate gathering and XMPP to fail.
# External DNS resolution is not guaranteed to be fast enough at service startup.
# Note: if the VM IP changes, re-run this script on the new VM.
# Guard against duplicate entries on re-runs.
if grep -q "$DNS" /etc/hosts; then
    sudo sed -i "/$DNS/d" /etc/hosts
fi
sudo -- sh -c "echo '$IP $DNS' >> /etc/hosts"

sudo hostnamectl set-hostname "$DNS"
hostname -f

echo "[Remove (if) existing packages to initialize jitsi]"
sudo apt purge jigasi jitsi-meet jitsi-meet-web-config jitsi-meet-prosody \
    jitsi-meet-turnserver jitsi-meet-web jicofo jitsi-videobridge2 -y &> /dev/null

echo "[Install prerequisites]"
sudo apt install -y apt-transport-https curl gnupg2 > /dev/null

# Enable universe repository (required on Ubuntu)
sudo add-apt-repository universe -y > /dev/null
sudo apt update -qq

# Remove needrestart to suppress interactive restart prompts during apt installs
sudo apt remove needrestart -y &> /dev/null

echo "[Add Prosody repository]"
sudo curl -sL https://prosody.im/files/prosody-debian-packages.key \
    -o /usr/share/keyrings/prosody-debian-packages.key
echo "deb [signed-by=/usr/share/keyrings/prosody-debian-packages.key] http://packages.prosody.im/debian $(lsb_release -sc) main" \
    | sudo tee /etc/apt/sources.list.d/prosody-debian-packages.list > /dev/null
sudo apt update -qq
sudo apt install -y lua5.2 > /dev/null

echo "[Add Jitsi repository]"
curl -sL https://download.jitsi.org/jitsi-key.gpg.key \
    | sudo sh -c 'gpg --dearmor > /usr/share/keyrings/jitsi-keyring.gpg'
echo "deb [signed-by=/usr/share/keyrings/jitsi-keyring.gpg] https://download.jitsi.org stable/" \
    | sudo tee /etc/apt/sources.list.d/jitsi-stable.list > /dev/null
sudo apt update -qq

echo "[Install Jitsi]"
# Pre-answer debconf prompts for non-interactive install.
# cert-choice: install with self-signed first, then replace with Let's Encrypt below.
echo "jitsi-videobridge2 jitsi-videobridge/jvb-hostname string $DNS" | sudo debconf-set-selections
echo "jitsi-meet-web-config jitsi-meet/cert-choice select Generate a new self-signed certificate (You will later get a chance to obtain a Let's Encrypt certificate)" | sudo debconf-set-selections
sudo DEBIAN_FRONTEND=noninteractive apt install -y jitsi-meet
if [ $? -ne 0 ]; then
    echo "[Error] jitsi-meet installation failed. Aborting."
    exit 1
fi

echo "[Install Let's Encrypt certificate (requires port 80 open and valid DNS)]"
sudo apt install -y certbot &> /dev/null
echo "$EMAIL" | sudo DEBIAN_FRONTEND=noninteractive /usr/share/jitsi-meet/scripts/install-letsencrypt-cert.sh
if [ $? -ne 0 ]; then
    echo "[Warning] Let's Encrypt certificate issuance failed."
    echo "Jitsi is running with a self-signed certificate."
    echo "Common causes: port 80/TCP not open, DNS not yet propagated, or rate limit reached (5/week per domain)."
    echo "To retry manually after resolving the issue:"
    echo "  echo '$EMAIL' | sudo /usr/share/jitsi-meet/scripts/install-letsencrypt-cert.sh"
fi

echo "[Config system limits for large meetings (>100 participants)]"
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

sudo systemctl daemon-reload
sudo systemctl restart prosody
sudo systemctl restart jicofo
sudo systemctl restart jitsi-videobridge2

# Wait for jitsi-videobridge2 to write its PID file before checking limits.
JVB_PID_FILE=/var/run/jitsi-videobridge/jitsi-videobridge.pid
echo "Waiting for jitsi-videobridge2 to start..."
for i in $(seq 1 15); do
    [ -f "$JVB_PID_FILE" ] && break
    sleep 2
done

if [ -f "$JVB_PID_FILE" ]; then
    sudo cat /proc/$(sudo cat "$JVB_PID_FILE")/limits
else
    echo "[Warning] jitsi-videobridge PID file not found after 30s — check: sudo systemctl status jitsi-videobridge2"
fi

echo "Done! elapsed time: $SECONDS"

PID=$(ps -ef | grep jitsi | grep -v grep | awk '{print $2}' | tr '\n' ' ')

echo ""
echo "[Start Jitsi: complete]"
echo "PID=$PID"
echo "Detected IP: $IP"
echo "Access DNS: $DNS (open in a Web browser)"
