#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 6. VM: Create MCIS"
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

curl -sX POST http://localhost:1323/tumblebug/ns/$NS_ID/mcis -H 'Content-Type: application/json' -d \
	'{
		"name": "MCIS-'$CSP'-'$POSTFIX'",
		"vm_num": "1",
		"description": "Tumblebug demo",
		"vm_req": [ {
			"image_id": "IMAGE-'$CSP'-'$POSTFIX'",
			"vm_access_id": "cb-user",
			"config_name": "'${CONN_CONFIG[INDEX]}'",
			"ssh_key_id": "KEYPAIR-'$CSP'-'$POSTFIX'",
			"spec_id": "SPEC-'$CSP'-'$POSTFIX'",
			"security_group_ids": [
				"SG-'$CSP'-'$POSTFIX'"
			],
			"vnet_id": "VPC-'$CSP'-'$POSTFIX'",
			"subnet_id": "Subnet-'$CSP'-'$POSTFIX'",
			"description": "description",
			"vm_access_passwd": "",
			"name": "VM-'$CSP'-'$POSTFIX'"
		} ]
	}' | json_pp || return 1

