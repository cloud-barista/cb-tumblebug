#!/bin/bash

#function list_NLB() {

    source ../conf.env
    
    echo "####################################################################"
    echo "## 10. NLB: List ID"
    echo "####################################################################"

    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID}/nlb?option=id | jq ''
    echo ""
#}

#list_NLB
