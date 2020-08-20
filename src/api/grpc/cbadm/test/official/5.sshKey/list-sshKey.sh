#!/bin/bash

#function list_sshKey() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

    source ../conf.env
    AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

    echo "####################################################################"
    echo "## 5. sshKey: List"
    echo "####################################################################"

    $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm keypairs list --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json --ns $NS_ID

#}

#list_sshKey