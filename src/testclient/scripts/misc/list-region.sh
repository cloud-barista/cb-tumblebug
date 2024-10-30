#!/bin/bash

#function get_ns() {

    source ../conf.env
    
    echo "####################################################################"
    echo "## 0. Region: List"
    echo "####################################################################"

    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/region | jq '.' #|| return 1
#}

#get_ns
