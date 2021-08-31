#!/bin/bash

#function filter_specs() {


    TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
    
    echo "####################################################################"
    echo "## 7. spec: filter"
    echo "####################################################################"

    resp=$(
        $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm spec filter-by-range --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -i json -o json -d \
	    "{
          \"nsId\":  \"${NSID}\",
		  \"filter\": {
              \"connectionName\": \"gcp\",
			    \"numvCPU\": {
                    \"min\": 2,
                    \"max\": 4
                }, 
                \"memGiB\": {
                    \"min\": 4
                },
                \"storageGiB\": {
                    \"max\": 400
                }
		    }
	    }"
    ); echo ${resp} | jq ''
    echo ""
#}

#filter_specs
