#!/bin/bash

#function get_ns() {


    TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
    
    echo "####################################################################"
    echo "## 0. Namespace: Get"
    echo "####################################################################"

    INDEX=${1}

    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/checkNs/$NSID | jq ''
    echo ""
#}

#get_ns
