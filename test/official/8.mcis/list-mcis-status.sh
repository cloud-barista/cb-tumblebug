#!/bin/bash

#function list_mcis() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

    source ../conf.env
    AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

    echo "####################################################################"
    echo "## 8. VM: List MCIS"
    echo "####################################################################"


    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NS_ID/mcis?option=status | json_pp
#}

#list_mcis