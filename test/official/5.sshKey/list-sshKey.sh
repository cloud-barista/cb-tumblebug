#!/bin/bash

#function list_sshKey() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

    source ../conf.env
    AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

    echo "####################################################################"
    echo "## 5. sshKey: List"
    echo "####################################################################"

    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NS_ID/resources/sshKey -H 'Content-Type: application/json' -d \
        '{ 
            "ConnectionName": "'${CONN_CONFIG[$INDEX,$REGION]}'"
        }' | json_pp #|| return 1
#}

#list_sshKey