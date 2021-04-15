#!/bin/bash

#function get_ns() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

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
    echo "## 0. Namespace: Get"
    echo "####################################################################"

    INDEX=${1}

    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NS_ID | jq ''
    echo ""
#}

#get_ns