#!/bin/bash

#function status_mcis() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

	source ../conf.env
	AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

	echo "####################################################################"
	echo "## 8. VM: Status MCIS"
	echo "####################################################################"


	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}
	MCISPREFIX=${4}
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


	if [ -z "$MCISPREFIX" ]
	then
		$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm mcis status --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json --ns $NSID --mcis ${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
	else
		MCISID=${MCISPREFIX}-${POSTFIX}
		$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm mcis status --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json --ns $NSID --mcis ${MCISID}
	fi

	
#}

#status_mcis