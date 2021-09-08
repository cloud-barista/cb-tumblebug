#!/bin/bash

#function list_ns() {

    echo "####################################################################"
    echo "## 0. Namespace: List"
    echo "####################################################################"

    source ../conf.env

    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns?option=id | jq ''
    echo ""
#}

#list_ns
