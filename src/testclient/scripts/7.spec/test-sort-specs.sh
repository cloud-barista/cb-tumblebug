#!/bin/bash

#function filter_specs() {


    TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
    
    echo "####################################################################"
    echo "## 7. spec: filter"
    echo "####################################################################"

    resp=$(
        curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/resources/testSortSpecs -H 'Content-Type: application/json' -d @- <<EOF
	    { 
		    "num_vCPU": 4
	    }
EOF
    ); echo ${resp} | jq ''
    echo ""
#}

#filter_specs
