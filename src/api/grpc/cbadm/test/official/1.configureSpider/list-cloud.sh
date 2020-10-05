#!/bin/bash

#function list_cloud() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

    source ../conf.env
    #source ../credentials.conf
    AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

    echo "####################################################################"
    echo "## 0. List Cloud Connction Config(s)"
    echo "####################################################################"


    #INDEX=${1}

    # for Cloud Connection Config Info
    $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm connect-info list --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json


    # for Cloud Region Info
    $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm region list --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json


    # for Cloud Credential Info
    $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm credential list --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json

    
    # for Cloud Driver Info
    $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm driver list --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json
#}

#list_cloud