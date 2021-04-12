#!/bin/bash

#function list_spec() {
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
    echo "## 7. spec: List"
    echo "####################################################################"


    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NS_ID/resources/spec | json_pp #|| return 1
#}

#list_spec