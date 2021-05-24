#!/bin/bash

#function get_cloud() {


    FILE=../credentials.conf
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi


    TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
    source ../credentials.conf
    
    echo "####################################################################"
    echo "## 0. Get Cloud Connction Config"
    echo "####################################################################"

    CSP=${1}
    REGION=${2:-1}
    POSTFIX=${3:-developer}
    
	source ../common-functions.sh
	getCloudIndex $CSP

    RESTSERVER=localhost

    # for Cloud Connection Config Info
    $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm connect-info get --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json -n ${CONN_CONFIG[$INDEX,$REGION]} | jq ''
    echo ""


    # for Cloud Region Info
    $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm region get --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json -n ${RegionName[$INDEX,$REGION]}  | jq ''
    echo ""


    # for Cloud Credential Info
    $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm credential get --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json -n ${CredentialName[$INDEX]} | jq ''
    echo ""

    
    # for Cloud Driver Info
    $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm driver get --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json -n ${DriverName[$INDEX]} | jq ''
    echo ""
#}

#get_cloud
