#!/bin/bash

#function init_config() {

    echo "####################################################################"
    echo "## 0. Config: Init (option: SPIDER_REST_URL, DRAGONFLY_REST_URL, ...)"
    echo "####################################################################"

    source ../init.sh

    VAR=${OPTION01}

    curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/config/$VAR | jq ''
    echo ""
#}

#init_config