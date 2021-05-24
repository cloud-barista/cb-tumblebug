#!/bin/bash

#function unregister_cloud() {


    FILE=../credentials.conf
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

	TestSetFile=${5:-../testSet.env}
    
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	source ../credentials.conf
	
	echo "####################################################################"
	echo "## 1. Remove All Cloud Connction Config(s)"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	source ../common-functions.sh
	getCloudIndex $CSP

	OPTION=${4:-none}

	RESTSERVER=localhost


	if [ "${OPTION}" == "leave" ]; then
		echo "[Leave Cloud Credential and Cloud Driver for other Regions]"
		exit
	fi
	
	# for Cloud Connection Config Info
	$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm connect-info delete --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json -n ${CONN_CONFIG[$INDEX,$REGION]} | jq ''
    echo ""


	# for Cloud Region Info
	$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm region delete --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json -n ${RegionName[$INDEX,$REGION]} | jq ''
    echo ""


	# for Cloud Credential Info
	$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm credential delete --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json -n ${CredentialName[$INDEX]}	 | jq ''
    echo ""


	# for Cloud Driver Info
	$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm driver delete --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json -n ${DriverName[$INDEX]}	 | jq ''
    echo ""


#}

#unregister_cloud
