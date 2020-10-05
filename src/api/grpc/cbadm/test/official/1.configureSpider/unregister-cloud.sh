#!/bin/bash

#function unregister_cloud() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

    FILE=../credentials.conf
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

	source ../conf.env
	source ../credentials.conf
	AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

	echo "####################################################################"
	echo "## 1. Remove All Cloud Connction Config(s)"
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

	OPTION=${4:-none}

	# for Cloud Connection Config Info
	$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm connect-info delete --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json -n ${CONN_CONFIG[$INDEX,$REGION]}

	# for Cloud Region Info
	$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm region delete --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json -n ${RegionName[$INDEX,$REGION]}

	if [ "${OPTION}" == "leave" ]; then
		echo "[Leave Cloud Credential and Cloud Driver for other Regions]"
		exit
	fi

	# for Cloud Credential Info
	$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm credential delete --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json -n ${CredentialName[INDEX]}
	
	# for Cloud Driver Info
	$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm driver delete --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json -n ${DriverName[INDEX]}
#}

#unregister_cloud
