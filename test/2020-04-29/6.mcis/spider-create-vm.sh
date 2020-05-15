#!/bin/bash

source ../conf.env

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

curl -sX POST http://localhost:1024/spider/vm -H 'Content-Type: application/json' -d \
	'{ 
		"ConnectionName": "'${CONN_CONFIG[INDEX]}'", 
		"ReqInfo": { 
			"Name": "vm-'$CSP'-'$POSTFIX'",
			"ImageName": "'${IMAGE_NAME[INDEX]}'", 
			"VPCName": "vpc-'$CSP'-'$POSTFIX'",
			"SubnetName": "subnet-'$CSP'-'$POSTFIX'",
			"SecurityGroupNames": [ "sg-'$CSP'-'$POSTFIX'" ], 
			"VMSpecName": "'${SPEC_NAME[INDEX]}'", 
			"KeyPairName": "keypair-'$CSP'-'$POSTFIX'"
		} 
	}' | json_pp
