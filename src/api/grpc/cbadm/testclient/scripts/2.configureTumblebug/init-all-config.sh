#!/bin/bash

#function init_all_config() {


    # TestSetFile=${4:-../testSet.env}
    # if [ ! -f "$TestSetFile" ]; then
    #     echo "$TestSetFile does not exist."
    #     exit
    # fi
	# source $TestSetFile
    source ../conf.env
    
    echo "####################################################################"
    echo "## 0. Config: Init ALL"
    echo "####################################################################"


    $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm config init-all --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json | jq ''
    echo ""
#}

#init_all_config
