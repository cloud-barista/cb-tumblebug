#!/bin/bash

#function create_ns() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

	source ../conf.env
	AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

	echo "####################################################################"
	echo "## 2. Namespace: Create"
	echo "####################################################################"

	INDEX=${1}

	$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm namespace create --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -i json -o json -d \
		'{
			"name": "'$NSID'",
			"description": "NameSpace for General Testing"
		}' 
#}

#create_ns