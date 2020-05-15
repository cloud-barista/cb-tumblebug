#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 3. sshKey: Create"
echo "####################################################################"

CSP=${1}
POSTFIX=${2:-developer}
if [ "${CSP}" == "aws" ]; then
	echo "[Test for AWS]"
	INDEX=1
elif [ "${CSP}" == "azure" ]; then
	echo "[Test for Azure]"
	INDEX=2
elif [ "${CSP}" == "gcp" ]; then
	echo "[Test for GCP]"
	INDEX=3
else
	echo "[No acceptable argument was provided (aws, azure, gcp, ..). Default: Test for AWS]"
	CSP="aws"
	INDEX=1
fi

curl -sX POST http://localhost:1323/tumblebug/ns/$NS_ID/resources/sshKey -H 'Content-Type: application/json' -d \
	'{ 
		"connectionName": "'${CONN_CONFIG[INDEX]}'", 
		"name": "keypair-'$CSP'-'$POSTFIX'", 
		"username": "ubuntu"
	}' | json_pp #|| return 1
