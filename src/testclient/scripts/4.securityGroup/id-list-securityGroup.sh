#!/bin/bash

#function list_securityGroup() {

    source ../conf.env
    
    echo "####################################################################"
    echo "## 4. SecurityGroup: List ID"
    echo "####################################################################"

    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/resources/securityGroup?option=id | jq ''
#}

#list_securityGroup
