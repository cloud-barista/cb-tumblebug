#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## Command (SSH) to MCIS "
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

MCISID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}

curl -sX POST http://$TumblebugServer/tumblebug/ns/$NS_ID/cmd/mcis/$MCISID -H 'Content-Type: application/json' -d \
	'{
		"command": "wget https://gist.githubusercontent.com/seokho-son/92f757bd4caf50127803833787b5a77d/raw/4f6ced2d04c05910f444af4f202bcef475db73ce/setweb.sh -O ~/setweb.sh; chmod +x ~/setweb.sh; sudo ~/setweb.sh"
	}' | json_pp #|| return 1
