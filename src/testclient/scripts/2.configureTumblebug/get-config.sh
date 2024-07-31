#!/bin/bash

#function get_config() {

    echo "####################################################################"
    echo "## 0. Config: Get (option: TB_SPIDER_REST_URL, TB_DRAGONFLY_REST_URL, ...)"
    echo "####################################################################"

    source ../init.sh

    VAR=${OPTION01:-ns01}

    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/config/$VAR | jq ''
    echo ""
#}

#get_config