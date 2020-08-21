#!/bin/bash

#function fetch_images() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

    source ../conf.env
    AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

    echo "####################################################################"
    echo "## 6. image: Fetch"
    echo "####################################################################"

    curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NS_ID/resources/fetchImages | json_pp #|| return 1
#}

#fetch_images