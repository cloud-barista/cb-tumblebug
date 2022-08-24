#!/bin/bash

#function list_NLB() {

    source ../conf.env
    
    echo "####################################################################"
    echo "## 10. NLB: List"
    echo "####################################################################"

    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/nlb | jq ''
    echo ""
#}

#list_NLB