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
#   TB_NTFY_FQDN          DNS name already pointing at TB_NTFY_IP; enables a Let's Encrypt certificate on :443.
#                         When empty a self-signed leaf certificate (IP SAN, CA:FALSE) is used on :9443.
#   TB_NTFY_PORT          HTTPS listen port (defaults to 443 with TB_NTFY_FQDN, 9443 when self-signed)
#   TB_NTFY_VERSION       ntfy release version to install (default 2.37.2)
#
# Prints the server certificate between TB-CA-BEGIN/TB-CA-END markers; CB-Tumblebug stores it as the pinned
# certificate for self-signed servers.
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
WANT_SHA256=""
if [ "$VERSION" = "2.37.2" ]; then
  case "$ARCH" in
    amd64) WANT_SHA256="7535968088fd7eded44ec1c32371a85c5cacad7b46b40171d602af2a658bf927" ;;
    arm64) WANT_SHA256="b604b8d121a0bfb1e1ace0d0157be5313d6a7e5bf50e222932d3a3e0c967bc09" ;;
  esac
fi

verify_deb() {
  [ -s "$1" ] || return 1
  [ -z "$WANT_SHA256" ] || [ "$(sha256sum "$1" | cut -d' ' -f1)" = "$WANT_SHA256" ]
}

if ! verify_deb /tmp/ntfy.deb; then
  REL_PATH="binwiederhier/ntfy/releases/download/v${VERSION}/ntfy_${VERSION}_linux_${ARCH}.deb"
  DOWNLOADED=0
  for BASE in "https://github.com" "https://gh-proxy.com/https://github.com" "https://ghfast.top/https://github.com"; do
    DEB_URL="${BASE}/${REL_PATH}"
    for i in 1 2; do
      rm -f /tmp/ntfy.deb
      if curl -fsSL --connect-timeout 8 --speed-limit 20480 --speed-time 10 -o /tmp/ntfy.deb "$DEB_URL" && verify_deb /tmp/ntfy.deb; then
        DOWNLOADED=1
        break 2
      fi
      sleep $((i * 2))
    done
  done
  [ "$DOWNLOADED" = 1 ] || { echo "cannot download verified ntfy package (${REL_PATH})" >&2; exit 1; }
fi
apt-get install -y -qq /tmp/ntfy.deb >/dev/null

mkdir -p /etc/ntfy /var/lib/ntfy /var/cache/ntfy
systemctl stop ntfy >/dev/null 2>&1 || true

if [ -n "$FQDN" ]; then
  PORT="${TB_NTFY_PORT:-443}"
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
  [ "$PORT" != "443" ] && BASE_URL="https://${FQDN}:${PORT}"
  UPSTREAM='upstream-base-url: "https://ntfy.sh"'
else
  PORT="${TB_NTFY_PORT:-9443}"
  openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 -nodes -days 3650 \
    -keyout /etc/ntfy/server.key -out /etc/ntfy/server.crt \
    -subj "/CN=tb-ntfy" -addext "subjectAltName=IP:${TB_NTFY_IP}" -addext "basicConstraints=critical,CA:FALSE" 2>/dev/null
  BASE_URL="https://${TB_NTFY_IP}:${PORT}"
  [ "$PORT" = "443" ] && BASE_URL="https://${TB_NTFY_IP}"
  UPSTREAM=""
fi

# Values come from variable expansion, so bcrypt hashes containing '$' are written literally.
cat > /etc/ntfy/server.yml <<EOF
base-url: "${BASE_URL}"
${UPSTREAM}
listen-http: "127.0.0.1:2586"
listen-https: ":${PORT}"
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
message-size-limit: "32k"
visitor-request-limit-burst: 1000
visitor-request-limit-replenish: "100ms"
visitor-message-daily-limit: 1000000
EOF

chown -R ntfy:ntfy /etc/ntfy /var/lib/ntfy /var/cache/ntfy
chmod 640 /etc/ntfy/server.key
systemctl daemon-reload
systemctl enable ntfy >/dev/null 2>&1
systemctl restart ntfy
for _ in $(seq 1 15); do curl -fsSk "https://127.0.0.1:${PORT}/v1/health" >/dev/null 2>&1 && break; sleep 2; done
curl -fsSk "https://127.0.0.1:${PORT}/v1/health"
echo
echo "-----TB-CA-BEGIN-----"
cat /etc/ntfy/server.crt
echo "-----TB-CA-END-----"
