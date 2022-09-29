#!/bin/bash

#function list_dataDisk() {

    source ../conf.env
    
    echo "####################################################################"
    echo "## 11. dataDisk: List ID"
    echo "####################################################################"

    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/resources/dataDisk?option=id | jq ''
    echo ""
#}

#list_dataDisk
