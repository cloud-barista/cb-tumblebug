#!/bin/bash

#function create_ns() {


	TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	
	echo "####################################################################"
	echo "## 2. Config: Create or Update (Param1: Key, Param2: Value)"
	echo "####################################################################"

	KEY=${1}
	VALUE=${2}

	resp=$(
        curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/config -H 'Content-Type: application/json' -d @- <<EOF
		{
			"name": "${KEY}",
			"value": "${VALUE}"
		}
EOF
    ); echo ${resp} | jq ''
    echo ""
#}

#create_ns