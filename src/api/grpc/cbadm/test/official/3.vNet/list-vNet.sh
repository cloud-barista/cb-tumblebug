#!/bin/bash

#function list_vNet() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

    source ../conf.env
    AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

    echo "####################################################################"
    echo "## 1. VPC: Get"
    echo "####################################################################"

    $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm network list --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json --ns $NS_ID
#}

#list_vNet