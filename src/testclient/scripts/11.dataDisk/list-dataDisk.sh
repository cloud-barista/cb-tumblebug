#!/bin/bash

#function list_dataDisk() {

    source ../conf.env
    
    echo "####################################################################"
    echo "## 11. dataDisk: List"
    echo "####################################################################"

    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/resources/dataDisk | jq ''
    echo ""
#}

#list_dataDisk