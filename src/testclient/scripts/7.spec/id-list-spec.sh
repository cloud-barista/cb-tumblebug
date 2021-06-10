#!/bin/bash

#function list_spec() {


    TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
    
    echo "####################################################################"
    echo "## 7. spec: List ID"
    echo "####################################################################"


    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/resources/spec?option=idOnly | jq '' #|| return 1
#}

#list_spec
