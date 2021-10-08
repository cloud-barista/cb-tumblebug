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
        curl -H "${AUTH}" -sX PUT http://$TumblebugServer/tumblebug/ns/$NSID/resources/spec/mock-seoul-jhseo -H 'Content-Type: application/json' -d @- <<EOF
		{ 
			"description": "UpdateSpec() test"
		}
EOF
    ); echo ${resp} | jq ''
    echo ""
#}

#register_spec
