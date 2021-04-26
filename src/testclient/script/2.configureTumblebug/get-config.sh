#!/bin/bash

#function get_ns() {


    TestSetFile=${4:-../testSet.env}
    
    FILE=$TestSetFile
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
    AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

    echo "####################################################################"
    echo "## 0. Config: Get (option: spider-rest-url, dragonfly-rest-url, ...)"
    echo "####################################################################"

    VAR=${1}

    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/config/$VAR | jq ''
    echo ""
#}

#get_config