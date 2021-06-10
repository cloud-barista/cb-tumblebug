#!/bin/bash

#function list_image() {


    TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
    
    echo "####################################################################"
    echo "## 6. image: List ID"
    echo "####################################################################"


    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/resources/image?option=idOnly | jq '' #|| return 1
#}

#list_image
