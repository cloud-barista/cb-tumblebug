#!/bin/bash

#function list_image() {


    TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
    
    echo "####################################################################"
    echo "## 6. image: List ID"
    echo "####################################################################"


    $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm image list-id --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o yaml --ns $NSID #| jq '' #|| return 1
#}

#list_image
