#!/bin/bash

#function get_config() {

    echo "####################################################################"
    echo "## 0. Config: Get (option: SPIDER_REST_URL, DRAGONFLY_REST_URL, ...)"
    echo "####################################################################"

    source ../init.sh

    VAR=${OPTION01:-tb01}

    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/config/$VAR | jq ''
    echo ""
#}

#get_config