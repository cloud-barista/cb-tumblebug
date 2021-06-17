#!/bin/bash

#function list_ns() {


    TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
    
    echo "####################################################################"
    echo "## 0. Namespace: List"
    echo "####################################################################"

    INDEX=${1}

    $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm namespace list-id --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json | jq ''
    echo ""
#}

#list_ns
