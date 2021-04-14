#!/bin/bash

#function registerImageWithInfo() {
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
    echo "## 6. image: Search"
    echo "####################################################################"

    resp=$(
        curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NS_ID/resources/searchImage -H 'Content-Type: application/json' -d @- <<EOF
        { 
            "keywords": [
                    "ubuntu",
                    "18.04"
            ]
        }
EOF
    ); echo ${resp} | jq
    echo ""
#}

#registerImageWithInfo
