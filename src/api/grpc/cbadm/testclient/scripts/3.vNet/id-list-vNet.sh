#!/bin/bash

#function list_vNet() {


    TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
    
    echo "####################################################################"
    echo "## 3. VPC: List ID"
    echo "####################################################################"

    $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm network list-id --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json --ns $NSID | jq ''
    echo ""
#}

#list_vNet
