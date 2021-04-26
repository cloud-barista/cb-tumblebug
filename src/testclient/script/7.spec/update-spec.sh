#!/bin/bash

#function register_spec() {


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
	echo "## 7. spec: Update"
	echo "####################################################################"

	resp=$(
        curl -H "${AUTH}" -sX PUT http://$TumblebugServer/tumblebug/ns/$NS_ID/resources/spec/aws-us-east-1-m5ad.2xlarge -H 'Content-Type: application/json' -d @- <<EOF
		{ 
			"id": "aws-us-east-1-m5ad.2xlarge", 
			"description": "UpdateSpec() test"
		}
EOF
    ); echo ${resp} | jq ''
    echo ""
#}

#register_spec
