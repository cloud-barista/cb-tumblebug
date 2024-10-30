#!/bin/bash

#function list_mci() {


    TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
    
    echo "####################################################################"
    echo "## 8. VM: List MCI"
    echo "####################################################################"


    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/policy/mci | jq '.'
#}

#list_mci