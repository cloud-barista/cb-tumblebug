#!/bin/bash

#function init_config() {


    TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
    
    echo "####################################################################"
    echo "## 0. Config: Init (option: SPIDER_REST_URL, DRAGONFLY_REST_URL, ...)"
    echo "####################################################################"

    VAR=${1}

    $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm config init --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json --id $VAR | jq ''
    echo ""
#}

#init_config