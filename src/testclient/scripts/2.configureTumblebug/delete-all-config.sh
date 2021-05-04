#!/bin/bash

#function delete_ns() {


    # TestSetFile=${4:-../testSet.env}
    # if [ ! -f "$TestSetFile" ]; then
    #     echo "$TestSetFile does not exist."
    #     exit
    # fi
	# source $TestSetFile
    source ../conf.env
    
    echo "####################################################################"
    echo "## 0. Config: Delete ALL"
    echo "####################################################################"


    curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/config | jq ''
    echo ""
#}

#delete_ns
