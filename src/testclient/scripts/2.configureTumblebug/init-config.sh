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

    curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/config/$VAR | jq ''
    echo ""
#}

#init_config