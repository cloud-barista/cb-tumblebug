#!/bin/bash

#function list_securityGroup() {

    source ../conf.env
    
    echo "####################################################################"
    echo "## 4. SecurityGroup: List"
    echo "####################################################################"

    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/resources/securityGroup | jq '.'
#}

#list_securityGroup