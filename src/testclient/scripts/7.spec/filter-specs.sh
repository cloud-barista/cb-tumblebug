#!/bin/bash

#function filter_specs() {


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
    echo "## 7. spec: filter"
    echo "####################################################################"

    resp=$(
        curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/resources/filterSpecs -H 'Content-Type: application/json' -d @- <<EOF
	    { 
		    "num_vCPU": 1, 
		    "mem_GiB": 2
	    }
EOF
    ); echo ${resp} | jq ''
    echo ""
#}

#filter_specs
