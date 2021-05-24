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
	echo "## 2. Namespace: Create"
	echo "####################################################################"

	INDEX=${1}

	resp=$(
		$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm namespace create --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -i json -o json -d \
		"{
			\"name\": \"$NSID\",
			\"description\": \"NameSpace for General Testing\"
		}"       
	); echo ${resp} | jq ''
	echo ""
#}

#create_ns