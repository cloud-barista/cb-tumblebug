#!/bin/bash

#function registerImageWithInfo() {


    TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
    
    echo "####################################################################"
    echo "## 6. image: Search"
    echo "####################################################################"

    resp=$(
        $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm image search --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -i json -o json -d \
        "{
            \"nsId\":  \"${NSID}\",
            \"keywords\": [
                    \"ubuntu\",
                    \"18.04\"
            ]
        }" 
    ); echo ${resp} | jq ''
    echo ""
#}

#registerImageWithInfo
