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
        curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/resources/filterSpecsByRange -H 'Content-Type: application/json' -d @- <<EOF
	    { 
		"connectionName": "gcp",
		    "num_vCPU": {
			    "min": 2,
			    "max": 2
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
