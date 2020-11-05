#!/bin/bash

#function registerImageWithInfo() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

    source ../conf.env
    AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

    echo "####################################################################"
    echo "## 6. image: Search"
    echo "####################################################################"

    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NS_ID/resources/searchImage -H 'Content-Type: application/json' -d \
        '{ 
            "keywords": [
                    "ubuntu",
                    "18.04"
            ]
        }' | json_pp #|| return 1
#}

#registerImageWithInfo
