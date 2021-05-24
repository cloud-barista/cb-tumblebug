#!/bin/bash

#function lookup_image() {


	TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	
	echo "####################################################################"
	echo "## 6. image: Lookup Image"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	source ../common-functions.sh
	getCloudIndex $CSP

	$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm image get-csp --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json --cc ${CONN_CONFIG[$INDEX,$REGION]} --image ${IMAGE_NAME[$INDEX,$REGION]} | jq ''
    echo ""
#}

#lookup_image
