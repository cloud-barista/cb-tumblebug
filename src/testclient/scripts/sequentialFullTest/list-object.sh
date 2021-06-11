#!/bin/bash


    TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
    
    echo "####################################################################"
    echo "## 0. Object: List"
    echo "####################################################################"

    KEY=${1}

    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/objects?key=$KEY | jq '' 
