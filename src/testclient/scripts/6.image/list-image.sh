#!/bin/bash

#function list_image() {

    source ../conf.env
    
    echo "####################################################################"
    echo "## 6. image: List"
    echo "####################################################################"

    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/resources/image | jq '' #|| return 1
#}

#list_image
