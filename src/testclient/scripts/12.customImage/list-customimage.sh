#!/bin/bash

#function list_customImage() {

    source ../conf.env
    
    echo "####################################################################"
    echo "## 6. customImage: List"
    echo "####################################################################"

    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/resources/customImage | jq '' #|| return 1
#}

#list_customImage
