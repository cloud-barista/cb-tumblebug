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
    echo "## 8. MCI: List ID"
    echo "####################################################################"


    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mci?option=id | jq '.'
#}

#list_mci
