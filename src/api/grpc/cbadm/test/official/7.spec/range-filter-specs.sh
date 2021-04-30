#!/bin/bash

#function filter_specs() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

    source ../conf.env
    AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

    echo "####################################################################"
    echo "## 7. spec: filter-by-range"
    echo "####################################################################"

    $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm spec filter-by-range --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -i json -o json -d \
	    '{
            "nsId":  "'${NSID}'",
		    "spec": {
			    "num_vCPU": {
                    "min": 2,
                    "max": 4
                }, 
                "mem_GiB": {
                    "min": 4
                },
                "storage_GiB": {
                    "max": 400
                }
		    }
	    }'

    
    #}

#filter_specs
