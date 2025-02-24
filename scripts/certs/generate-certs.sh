#!/bin/bash
set -euo pipefail
IFS=$'\n\t'

# SSL Certificate Generator for Local Testing
# Creates self-signed certificates for use with Traefik
# Supports localhost, custom domain, and IP (Private/Public)

# Usage Examples:
# Default (localhost + auto-detected IPs): ./generate-certs.sh
# Custom directory: ./generate-certs.sh CERT_DIR=/path/to/certs
# Custom domain: ./generate-certs.sh DOMAIN=yourdomain.com
# Custom IP: ./generate-certs.sh IP=xxx.xxx.xxx.xxx
# All: ./generate-certs.sh DOMAIN=yourdomain.com IP=xxx.xxx.xxx.xxx CERT_DIR=/path/to/certs

# Output Files:
# cert.key - SSL private key (KEEP SECURE)
# cert.crt - SSL public certificate
# cert.pem - Combined certificate and key (for import)

# Parse arguments
for arg in "$@"; do
  case "$arg" in
    CERT_DIR=*) CERT_DIR="${arg#*=}" ;;
    DOMAIN=*) DOMAIN="${arg#*=}" ;;
    IP=*) IP="${arg#*=}" ;;
  esac
done

# Default values
CERT_DIR="${CERT_DIR:-$HOME/.cloud-barista/certs}"
DOMAIN="${DOMAIN:-localhost}"
IP="${IP:-}"

# Check if certificate files already exist
if [ -f "${CERT_DIR}/cert.key" ]; then
  echo "Certificate files already exist. Skipping"
  exit 0
fi

# Create cert directory if it doesn't exist
if [ ! -d "${CERT_DIR}" ]; then
  mkdir -p "${CERT_DIR}"
fi
cd "${CERT_DIR}"

echo "Generating certificate in $CERT_DIR for DOMAIN=${DOMAIN:-N/A}, IP=${IP:-N/A}"

# Detect Private & Public IP if IP=self
if [ "$IP" = "self" ]; then
  PRIVATE_IP=$(ip addr show | grep -oP 'inet \K192\.168\.\d+\.\d+|10\.\d+\.\d+\.\d+|172\.(1[6-9]|2\d|3[0-1])\.\d+\.\d+' | head -n 1 || echo "")
  PUBLIC_IP=$(curl -s ifconfig.me || echo "")

  IP_LIST=()
  [ -n "$PRIVATE_IP" ] && IP_LIST+=("$PRIVATE_IP")
  [ -n "$PUBLIC_IP" ] && IP_LIST+=("$PUBLIC_IP")

  IP=$(IFS=','; echo "${IP_LIST[*]}")
  echo "Detected IP: $IP"
fi

# Build SAN list (localhost is always included)
SAN_ENTRIES=("DNS:localhost" "DNS:*.localhost")

# Add multiple domains if provided
if [ -n "$DOMAIN" ]; then
  IFS=',' read -ra DOMAINS <<< "$DOMAIN"
  for dom in "${DOMAINS[@]}"; do
    [ "$dom" != "localhost" ] && SAN_ENTRIES+=("DNS:$dom" "DNS:*.$dom")
  done
fi

# Add multiple IPs if provided
if [ -n "$IP" ]; then
  IFS=',' read -ra IPS <<< "$IP"
  for ip in "${IPS[@]}"; do
    SAN_ENTRIES+=("IP:$ip")
  done
fi

# Join SAN entries with commas (쉼표를 정확히 삽입)
SAN_LIST="subjectAltName=$(IFS=','; echo "${SAN_ENTRIES[*]}")"

echo "Subject Alternative Name (SAN) list: $SAN_LIST"

# Generate self-signed certificate
openssl req \
  -newkey rsa:2048 \
  -x509 \
  -nodes \
  -keyout "cert.key" \
  -new \
  -out "cert.crt" \
  -subj "/CN=compose-dev-tls Self-Signed" \
  -addext "$SAN_LIST" \
  -sha256 \
  -days 3650

cat "cert.crt" "cert.key" > "cert.pem"
chmod 600 "cert.key"
echo "Certificate created successfully in $CERT_DIR."

cat "cert.crt" "cert.key" > "cert.pem"
echo "New TLS self-signed certificate created"

# Secure the private key
chmod 600 "cert.key"
