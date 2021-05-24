#!/bin/bash

#function registerImageWithId() {


	TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	
	echo "####################################################################"
	echo "## 6. image: Register"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	source ../common-functions.sh
	getCloudIndex $CSP

	resp=$(
			$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm image create-id --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -i json -o json -d \
			"{
				\"nsId\":  \"${NSID}\",
				\"image\": {
					\"connectionName\": \"${CONN_CONFIG[$INDEX,$REGION]}\", 
					\"name\": \"${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}\",
					\"cspImageId\": \"${IMAGE_NAME[$INDEX,$REGION]}\",
					\"description\": \"Canonical, Ubuntu, 18.04 LTS, amd64 bionic\",
					\"guestOS\": \"Ubuntu\"			
				}
			}"
    ); echo ${resp} | jq ''
    echo ""
#}

#registerImageWithId
