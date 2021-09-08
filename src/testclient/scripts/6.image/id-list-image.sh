#!/bin/bash

#function list_image() {

    source ../conf.env
    
    echo "####################################################################"
    echo "## 6. image: List ID"
    echo "####################################################################"

    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/resources/image?option=id | jq '' #|| return 1
#}

#list_image
