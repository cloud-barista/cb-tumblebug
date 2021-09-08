#!/bin/bash

#function list_spec() {

    source ../conf.env
    
    echo "####################################################################"
    echo "## 7. spec: List ID"
    echo "####################################################################"


    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/resources/spec?option=id | jq '' #|| return 1
#}

#list_spec
