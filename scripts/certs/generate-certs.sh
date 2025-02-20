#!/bin/bash
set -euo pipefail
IFS=$'\n\t'

# SSL Certificate Generator for Local Testing
# Creates self-signed certificates for use with Traefik
# Supports localhost and custom domain testing

# Usage Examples:
# For localhost testing: ./generate-certs.sh
# For custom domain testing: ./generate-certs.sh yourdomain.com

# Output Files:
# cert.key - SSL private key (KEEP SECURE)
# cert.crt - SSL public certificate
# cert.pem - Combined certificate and key (for import)

# Set domain name
DOMAIN_NAME=${1:-"localhost"}
# Set cert directory
CERT_DIR=${2:-"$HOME/.certs"}

if [ ! -f "${CERT_DIR}/cert.key" ]; then

  cd "${CERT_DIR}"

  if [ "$DOMAIN_NAME" = "localhost" ]; then
    echo "No additional domains will be added to cert"
    SAN_LIST="[SAN]\nsubjectAltName=DNS:localhost, DNS:*.localhost"
    printf "%s" "$SAN_LIST"
  else
    echo "You supplied domain $DOMAIN_NAME"
    SAN_LIST="[SAN]\nsubjectAltName=DNS:localhost, DNS:*.localhost, DNS:*.$DOMAIN_NAME, DNS:$DOMAIN_NAME"
    printf "%s" "$SAN_LIST"
  fi

  openssl req \
    -newkey rsa:2048 \
    -x509 \
    -nodes \
    -keyout "cert.key" \
    -new \
    -out "cert.crt" \
    -subj "/CN=compose-dev-tls Self-Signed" \
    -addext "subjectAltName=DNS:localhost,DNS:*.localhost,DNS:$DOMAIN_NAME,DNS:*.$DOMAIN_NAME" \
    -sha256 \
    -days 3650

  cat "cert.crt" "cert.key" > "cert.pem"
  echo "new TLS self-signed certificate created"

else

  echo "certificate files already exist. Skipping"

fi