#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 1. vpc: Create"
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

curl -sX POST http://localhost:1323/tumblebug/ns/$NS_ID/resources/vNet -H 'Content-Type: application/json' -d \
	'{
		"name": "vpc-'$CSP'-'$POSTFIX'",
		"connectionName": "'${CONN_CONFIG[INDEX]}'",
		"cidrBlock": "192.168.0.0/16",
		"subnetReqInfoList": [ {
			"Name": "subnet-'$CSP'-'$POSTFIX'",
			"IPv4_CIDR": "192.168.1.0/24"
		} ]
	}' | json_pp #|| return 1
