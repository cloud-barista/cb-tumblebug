#!/bin/bash

#function list_sshKey() {

    source ../conf.env
    
    echo "####################################################################"
    echo "## 5. sshKey: List ID"
    echo "####################################################################"

    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/resources/sshKey?option=id | jq '.'
    echo ""
#}

#list_sshKey
