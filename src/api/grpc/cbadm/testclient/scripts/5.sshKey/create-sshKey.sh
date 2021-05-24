#!/bin/bash

#function create_sshKey() {


	TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	
	echo "####################################################################"
	echo "## 5. sshKey: Create"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	source ../common-functions.sh
	getCloudIndex $CSP

	$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm keypair create --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -i json -o json -d \
	"{
		\"nsId\":  \"${NSID}\",
		\"sshKey\": {
			\"connectionName\": \"${CONN_CONFIG[$INDEX,$REGION]}\", 
			\"name\": \"${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}\", 
			\"username\": \"ubuntu\"
		}
	}" | jq ''

#}

#create_sshKey