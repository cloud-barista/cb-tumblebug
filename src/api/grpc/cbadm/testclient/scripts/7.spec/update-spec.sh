#!/bin/bash

#function register_spec() {


	TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	
	echo "####################################################################"
	echo "## 7. spec: Update"
	echo "####################################################################"

	resp=$(
			$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm spec update --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -i json -o json -d \
			"{
				\"nsId\":  \"${NSID}\",
				\"spec\": {
						\"id\": \"aws-us-east-1-a1.2xlarge\", 
						\"description\": \"UpdateSpec() test\"
				}
			}"
    ); echo ${resp} | jq ''
    echo ""
#}

#register_spec
