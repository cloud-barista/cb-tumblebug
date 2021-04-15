#!/bin/bash

#function filter_specs() {
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
    echo "## 7. spec: filter"
    echo "####################################################################"

    resp=$(
        curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NS_ID/resources/filterSpecsByRange -H 'Content-Type: application/json' -d @- <<EOF
	    { 
		    "num_vCPU": {
			    "min": 2,
			    "max": 4
		    }, 
		    "mem_GiB": {
			    "min": 4
		    },
		    "storage_GiB": {
			    "max": 400
		    }
	    }
EOF
    ); echo ${resp} | jq ''
    echo ""
#}

#filter_specs
