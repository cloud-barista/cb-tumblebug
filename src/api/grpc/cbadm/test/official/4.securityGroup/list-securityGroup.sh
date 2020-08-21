#!/bin/bash

#function list_securityGroup() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

    source ../conf.env
    AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

    echo "####################################################################"
    echo "## 4. SecurityGroup: List"
    echo "####################################################################"


    $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm securitygroup list --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json --ns $NS_ID
#}

#list_securityGroup