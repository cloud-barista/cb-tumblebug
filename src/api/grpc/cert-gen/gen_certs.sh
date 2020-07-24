#!/bin/bash

if [ ! -d "./certs" ]; then
  mkdir $CBTUMBLEBUG_ROOT/certs
fi

# Create Root signing Key
openssl genrsa -out $CBTUMBLEBUG_ROOT/certs/ca.key 4096

# Generate self-signed Root certificate
openssl req -new -x509 -key $CBTUMBLEBUG_ROOT/certs/ca.key -sha256 -subj "/C=KR/ST=DJ/O=Test CA, Inc." -days 3650 -out $CBTUMBLEBUG_ROOT/certs/ca.crt

# Create a Key certificate for your service
openssl genrsa -out $CBTUMBLEBUG_ROOT/certs/server.key 4096

# Create signing CSR
openssl req -new -key $CBTUMBLEBUG_ROOT/certs/server.key -out $CBTUMBLEBUG_ROOT/certs/server.csr -config certificate.conf

# Generate a certificate for the service
openssl x509 -req -in $CBTUMBLEBUG_ROOT/certs/server.csr -CA $CBTUMBLEBUG_ROOT/certs/ca.crt -CAkey $CBTUMBLEBUG_ROOT/certs/ca.key -CAcreateserial -out $CBTUMBLEBUG_ROOT/certs/server.crt -days 3650 -sha256 -extfile certificate.conf -extensions req_ext