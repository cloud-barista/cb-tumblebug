#!/bin/bash

#function filter_specs() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

    source ../conf.env
    AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

    echo "####################################################################"
    echo "## 7. spec: filter"
    echo "####################################################################"

    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NS_ID/resources/testSortSpecs -H 'Content-Type: application/json' -d \
	    '{ 
		    "num_vCPU": '4'
	    }' | json_pp #|| return 1

#}

#filter_specs
