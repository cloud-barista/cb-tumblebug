#!/bin/bash

#function register_image() {


	TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	
	echo "####################################################################"
	echo "## 6. image: Update"
	echo "####################################################################"

	resp=$(
        curl -H "${AUTH}" -sX PUT http://$TumblebugServer/tumblebug/ns/$NSID/resources/image/mock-seoul-jhseo -H 'Content-Type: application/json' -d @- <<EOF
		{ 
			"description": "UpdateImage() test"
		}
EOF
    ); echo ${resp} | jq ''
    echo ""
#}

#register_image
