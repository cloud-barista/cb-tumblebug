#!/bin/bash

#function list_vNet() {

    source ../conf.env
    
    echo "####################################################################"
    echo "## 3. VPC: List"
    echo "####################################################################"

    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/resources/vNet | jq '.'
    echo ""
#}

#list_vNet