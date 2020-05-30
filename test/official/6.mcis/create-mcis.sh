#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 6. vm: Create MCIS"
echo "####################################################################"

CSP=${1}
REGION=${2:-1}
POSTFIX=${3:-developer}
if [ "${CSP}" == "aws" ]; then
	echo "[Test for AWS]"
	INDEX=1
elif [ "${CSP}" == "azure" ]; then
	echo "[Test for Azure]"
	INDEX=2
elif [ "${CSP}" == "gcp" ]; then
	echo "[Test for GCP]"
	INDEX=3
elif [ "${CSP}" == "alibaba" ]; then
	echo "[Test for Alibaba]"
	INDEX=4
else
	echo "[No acceptable argument was provided (aws, azure, gcp, alibaba, ...). Default: Test for AWS]"
	CSP="aws"
	INDEX=1
fi

curl -sX POST http://$TumblebugServer/tumblebug/ns/$NS_ID/mcis -H 'Content-Type: application/json' -d \
	'{
		"name": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'",
		"description": "Tumblebug Demo",
		"vm_req": [ {
			"name": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'-01",
			"image_id": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'",
			"vm_access_id": "cb-user",
			"config_name": "'${CONN_CONFIG[$INDEX,$REGION]}'",
			"ssh_key_id": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'",
			"spec_id": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'",
			"security_group_ids": [
				"'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'"
			],
			"vnet_id": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'",
			"subnet_id": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'",
			"description": "description",
			"vm_access_passwd": ""
		},
		{
			"name": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'-02",
			"image_id": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'",
			"vm_access_id": "cb-user",
			"config_name": "'${CONN_CONFIG[$INDEX,$REGION]}'",
			"ssh_key_id": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'",
			"spec_id": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'",
			"security_group_ids": [
				"'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'"
			],
			"vnet_id": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'",
			"subnet_id": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'",
			"description": "description",
			"vm_access_passwd": ""
		},
		{
			"name": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'-03",
			"image_id": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'",
			"vm_access_id": "cb-user",
			"config_name": "'${CONN_CONFIG[$INDEX,$REGION]}'",
			"ssh_key_id": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'",
			"spec_id": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'",
			"security_group_ids": [
				"'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'"
			],
			"vnet_id": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'",
			"subnet_id": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'",
			"description": "description",
			"vm_access_passwd": ""
		} ]
	}' | json_pp || return 1

