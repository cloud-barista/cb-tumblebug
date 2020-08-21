#!/bin/bash

#function delete_ns() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

    source ../conf.env
    AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

    echo "####################################################################"
    echo "## 0. Namespace: Delete"
    echo "####################################################################"

    INDEX=${1}

    $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm namespaces delete --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json --ns $NS_ID
#}

#delete_ns