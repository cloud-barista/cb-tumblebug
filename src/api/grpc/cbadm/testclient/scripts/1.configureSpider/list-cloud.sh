#!/bin/bash

#function list_cloud() {


    TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
    #source ../credentials.conf
    
    echo "####################################################################"
    echo "## 0. List Cloud Connction Config(s)"
    echo "####################################################################"


    #INDEX=${1}

    RESTSERVER=localhost

    # for Cloud Connection Config Info
    $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm connect-info list --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json | jq ''
    echo ""


    # for Cloud Region Info
    $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm region list --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json | jq ''
    echo ""


    # for Cloud Credential Info
    $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm credential list --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json | jq ''
    echo ""
    
    
    # for Cloud Driver Info
    $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm driver list --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json | jq ''
    echo ""
#}

#list_cloud
