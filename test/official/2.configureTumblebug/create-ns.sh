#!/bin/bash

#function create_ns() {
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
	echo "## 2. Namespace: Create"
	echo "####################################################################"

	INDEX=${1}

    resp=$(
        curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns -H 'Content-Type: application/json' -d @- <<EOF
        {
			"name": "$NS_ID",
			"description": "NameSpace for General Testing"
		}
EOF
    ); echo ${resp} | jq ''
    echo ""
#}

#create_ns