#!/bin/bash

#function get_ns() {

    source ../conf.env
    
    echo "####################################################################"
    echo "## 0. Conn Config: List"
    echo "####################################################################"

    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/connConfig | jq '.' #|| return 1
#}

#get_ns
