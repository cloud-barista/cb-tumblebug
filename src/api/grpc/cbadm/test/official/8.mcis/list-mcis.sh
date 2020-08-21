#!/bin/bash

#function list_mcis() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

    source ../conf.env
    AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

    echo "####################################################################"
    echo "## 8. VM: List MCIS"
    echo "####################################################################"

    $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm mcis list --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json --ns $NS_ID --option status

#}

#list_mcis