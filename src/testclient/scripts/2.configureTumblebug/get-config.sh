#!/bin/bash

#function get_config() {


    TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
    
    echo "####################################################################"
    echo "## 0. Config: Get (option: SPIDER_REST_URL, DRAGONFLY_REST_URL, ...)"
    echo "####################################################################"

    VAR=${1}

    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/config/$VAR | jq ''
    echo ""
#}

#get_config