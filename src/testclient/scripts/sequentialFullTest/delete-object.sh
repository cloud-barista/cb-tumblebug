#!/bin/bash


    TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
    
    echo "####################################################################"
    echo "## 0. Object: Delete"
    echo "####################################################################"

    KEY=${1}

    curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/object?key=$KEY | jq '.' 
