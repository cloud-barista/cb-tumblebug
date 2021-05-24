#!/bin/bash

#function create_ns() {


	TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	
	echo "####################################################################"
	echo "## 2. Config: Create or Update (Param1: Key, Param2: Value)"
	echo "####################################################################"

	KEY=${1}
	VALUE=${2}

	resp=$(
		$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm config create --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -i json -o json -d \
			"{
				\"name\": \"${KEY}\",
				\"value\": \"${VALUE}\"
			}" 
    ); echo ${resp} | jq ''
    echo ""
#}

#create_ns