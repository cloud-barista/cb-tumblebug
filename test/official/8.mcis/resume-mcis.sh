#!/bin/bash

source ../conf.env
AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

echo "####################################################################"
echo "## 8. VM: Resume from suspended MCIS"
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

curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NS_ID/mcis/${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}?action=resume | json_pp
