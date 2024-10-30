#!/bin/bash

#function init_all_config() {


    # TestSetFile=${4:-../testSet.env}
    # if [ ! -f "$TestSetFile" ]; then
    #     echo "$TestSetFile does not exist."
    #     exit
    # fi
	# source $TestSetFile
    source ../conf.env
    
    echo "####################################################################"
    echo "## 0. Config: Init ALL"
    echo "####################################################################"


    curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/config | jq '.'
    echo ""
#}

#init_all_config
