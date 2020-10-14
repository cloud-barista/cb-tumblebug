#!/bin/bash

#function get_ns() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

    source ../conf.env
    AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

    echo "####################################################################"
    echo "## 0. Config: Get (option: spider, dragonfly, ...)"
    echo "####################################################################"

    VAR=${1}

    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/config/$VAR | json_pp #|| return 1
#}

#get_config