#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 5. spec: Fetch"
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

curl -sX GET http://$SpiderServer/spider/vmspec/${SPEC_NAME[$INDEX,$REGION]} -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG[$INDEX,$REGION]}'" }' | json_pp

