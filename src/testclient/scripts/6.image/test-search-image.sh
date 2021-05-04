#!/bin/bash

#function registerImageWithInfo() {


    TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
    
    echo "####################################################################"
    echo "## 6. image: Search"
    echo "####################################################################"

    resp=$(
        curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/resources/searchImage -H 'Content-Type: application/json' -d @- <<EOF
        { 
            "keywords": [
                    "ubuntu",
                    "18.04"
            ]
        }
EOF
    ); echo ${resp} | jq ''
    echo ""
#}

#registerImageWithInfo
