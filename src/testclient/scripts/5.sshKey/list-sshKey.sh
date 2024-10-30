#!/bin/bash

#function list_sshKey() {

    source ../conf.env
    
    echo "####################################################################"
    echo "## 5. sshKey: List"
    echo "####################################################################"

    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/resources/sshKey | jq '.'
    echo ""
#}

#list_sshKey