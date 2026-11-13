#!/bin/bash
# Installs and configures an ntfy server for CB-Tumblebug notifications.
# Embedded into cb-tumblebug (src/core/notifier) and run on the VM as root by POST /tumblebug/notifier/ntfy.
#
# Required environment:
#   TB_NTFY_IP            public IP of this VM
#   TB_NTFY_TOPIC         topic CB-Tumblebug uses (e.g. tb-a1b2c3d4)
#   TB_NTFY_WRITE_TOKEN   ntfy token for the write-only account (tk_...)
#   TB_NTFY_READ_TOKEN    ntfy token for the read-only account (tk_...)
#   TB_NTFY_WRITER_HASH   bcrypt password hash for user tb-writer
#   TB_NTFY_READER_HASH   bcrypt password hash for user tb-reader
# Optional:
#   TB_NTFY_FQDN          DNS name already pointing at TB_NTFY_IP; enables a Let's Encrypt certificate.
#                         When empty a self-signed certificate (IP SAN) is used.
#   TB_NTFY_VERSION       ntfy version to install when the GitHub releases API is unavailable
#
# Prints the server certificate between TB-CA-BEGIN/TB-CA-END markers; CB-Tumblebug stores it as the CA
# for self-signed servers.
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

: "${TB_NTFY_IP:?TB_NTFY_IP is required}"
: "${TB_NTFY_TOPIC:?TB_NTFY_TOPIC is required}"
: "${TB_NTFY_WRITE_TOKEN:?TB_NTFY_WRITE_TOKEN is required}"
: "${TB_NTFY_READ_TOKEN:?TB_NTFY_READ_TOKEN is required}"
: "${TB_NTFY_WRITER_HASH:?TB_NTFY_WRITER_HASH is required}"
: "${TB_NTFY_READER_HASH:?TB_NTFY_READER_HASH is required}"
FQDN="${TB_NTFY_FQDN:-}"
VERSION="${TB_NTFY_VERSION:-2.37.2}"

apt-get update -y -qq
apt-get install -y -qq curl openssl >/dev/null

ARCH=$(dpkg --print-architecture)
DEB_URL=$(curl -fsSL https://api.github.com/repos/binwiederhier/ntfy/releases/latest | grep -oE "https://[^\"]+_linux_${ARCH}\.deb" | head -1 || true)
[ -n "$DEB_URL" ] || DEB_URL="https://github.com/binwiederhier/ntfy/releases/download/v${VERSION}/ntfy_${VERSION}_linux_${ARCH}.deb"
for i in 1 2 3 4 5; do
  curl -fsSL --connect-timeout 15 --speed-limit 10240 --speed-time 30 -o /tmp/ntfy.deb "$DEB_URL" && break
  [ "$i" = 5 ] && { echo "cannot download $DEB_URL"; exit 1; }
  sleep $((i * 5))
done
apt-get install -y -qq /tmp/ntfy.deb >/dev/null

mkdir -p /etc/ntfy /var/lib/ntfy /var/cache/ntfy
systemctl stop ntfy >/dev/null 2>&1 || true

if [ -n "$FQDN" ]; then
  for _ in $(seq 1 36); do getent hosts "$FQDN" | grep -q "$TB_NTFY_IP" && break; sleep 5; done
  getent hosts "$FQDN" | grep -q "$TB_NTFY_IP" || { echo "DNS for $FQDN does not resolve to $TB_NTFY_IP" >&2; exit 1; }
  apt-get install -y -qq certbot >/dev/null
  cat > /etc/ntfy/deploy-cert.sh <<EOF
#!/bin/bash
set -e
cp "/etc/letsencrypt/live/${FQDN}/fullchain.pem" /etc/ntfy/server.crt
cp "/etc/letsencrypt/live/${FQDN}/privkey.pem" /etc/ntfy/server.key
chown ntfy:ntfy /etc/ntfy/server.crt /etc/ntfy/server.key
chmod 640 /etc/ntfy/server.key
systemctl restart ntfy >/dev/null 2>&1 || true
EOF
  chmod 755 /etc/ntfy/deploy-cert.sh
  certbot certonly --standalone --non-interactive --agree-tos --register-unsafely-without-email \
    -d "$FQDN" --deploy-hook /etc/ntfy/deploy-cert.sh >/dev/null
  cp "/etc/letsencrypt/live/$FQDN/fullchain.pem" /etc/ntfy/server.crt
  cp "/etc/letsencrypt/live/$FQDN/privkey.pem" /etc/ntfy/server.key
  BASE_URL="https://$FQDN"
  UPSTREAM='upstream-base-url: "https://ntfy.sh"'
else
  openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 -nodes -days 3650 \
    -keyout /etc/ntfy/server.key -out /etc/ntfy/server.crt \
    -subj "/CN=tb-ntfy" -addext "subjectAltName=IP:${TB_NTFY_IP}" 2>/dev/null
  BASE_URL="https://${TB_NTFY_IP}"
  UPSTREAM=""
fi

# Values come from variable expansion, so bcrypt hashes containing '$' are written literally.
cat > /etc/ntfy/server.yml <<EOF
base-url: "${BASE_URL}"
${UPSTREAM}
listen-http: "127.0.0.1:2586"
listen-https: ":443"
key-file: /etc/ntfy/server.key
cert-file: /etc/ntfy/server.crt
cache-file: /var/cache/ntfy/cache.db
cache-duration: "24h"
auth-file: /var/lib/ntfy/user.db
auth-default-access: "deny-all"
auth-users:
  - "tb-writer:${TB_NTFY_WRITER_HASH}:user"
  - "tb-reader:${TB_NTFY_READER_HASH}:user"
auth-access:
  - "tb-writer:${TB_NTFY_TOPIC}:wo"
  - "tb-reader:${TB_NTFY_TOPIC}:ro"
auth-tokens:
  - "tb-writer:${TB_NTFY_WRITE_TOKEN}:vm-report"
  - "tb-reader:${TB_NTFY_READ_TOKEN}:tb-subscribe"
visitor-request-limit-burst: 1000
visitor-request-limit-replenish: "100ms"
visitor-message-daily-limit: 1000000
EOF

chown -R ntfy:ntfy /etc/ntfy /var/lib/ntfy /var/cache/ntfy
chmod 640 /etc/ntfy/server.key
systemctl daemon-reload
systemctl enable ntfy >/dev/null 2>&1
systemctl restart ntfy
for _ in $(seq 1 15); do curl -fsSk https://127.0.0.1/v1/health >/dev/null 2>&1 && break; sleep 2; done
curl -fsSk https://127.0.0.1/v1/health
echo
echo "-----TB-CA-BEGIN-----"
cat /etc/ntfy/server.crt
echo "-----TB-CA-END-----"
