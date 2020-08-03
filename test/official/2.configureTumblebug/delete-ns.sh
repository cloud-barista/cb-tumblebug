#!/bin/bash

#function delete_ns() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

    source ../conf.env
    AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

    echo "####################################################################"
    echo "## 0. Namespace: Delete"
    echo "####################################################################"

    INDEX=${1}

    curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NS_ID | json_pp #|| return 1
#}

#delete_ns