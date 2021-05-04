#!/bin/bash

#function register_spec() {


	TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	
	echo "####################################################################"
	echo "## 7. spec: Update"
	echo "####################################################################"

	resp=$(
        curl -H "${AUTH}" -sX PUT http://$TumblebugServer/tumblebug/ns/$NSID/resources/spec/aws-us-east-1-m5ad.2xlarge -H 'Content-Type: application/json' -d @- <<EOF
		{ 
			"id": "aws-us-east-1-m5ad.2xlarge", 
			"description": "UpdateSpec() test"
		}
EOF
    ); echo ${resp} | jq ''
    echo ""
#}

#register_spec
