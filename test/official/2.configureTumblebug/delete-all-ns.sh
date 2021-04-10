#!/bin/bash

#function delete_ns() {
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
    echo "## 0. Namespace: Delete"
    echo "####################################################################"

    INDEX=${1}

    curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns | json_pp #|| return 1
#}

#delete_ns
