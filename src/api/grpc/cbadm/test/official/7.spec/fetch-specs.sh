#!/bin/bash

#function fetch_specs() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

    source ../conf.env
    AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

    echo "####################################################################"
    echo "## 7. spec: Fetch"
    echo "####################################################################"

    $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm spec fetch --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml --ns ns-01
#	    '{
#		    "nsId":  "'${NSID}'"
#	  }'
#}

#fetch_specs
