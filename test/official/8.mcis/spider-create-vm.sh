#!/bin/bash

source ../conf.env
AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

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

curl -H "${AUTH}" -sX POST http://$SpiderServer/spider/vm -H 'Content-Type: application/json' -d \
	'{ 
		"ConnectionName": "'${CONN_CONFIG[$INDEX,$REGION]}'", 
		"ReqInfo": { 
			"Name": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'-01",
			"ImageName": "'${IMAGE_NAME[$INDEX,$REGION]}'", 
			"VPCName": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'",
			"SubnetName": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'",
			"SecurityGroupNames": [ "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'" ], 
			"VMSpecName": "'${SPEC_NAME[$INDEX,$REGION]}'", 
			"KeyPairName": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'"
		} 
	}' | json_pp
