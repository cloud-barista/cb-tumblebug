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
	echo "## 7. spec: Register"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	source ../common-functions.sh
	getCloudIndex $CSP

	resp=$(
			$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm spec create-id --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -i json -o json -d \
			"{
				\"nsId\":  \"${NSID}\",
				\"spec\": {
					\"connectionName\": \"${CONN_CONFIG[$INDEX,$REGION]}\", 
					\"name\": \"${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}\",
					\"cspSpecName\": \"${SPEC_NAME[$INDEX,$REGION]}\"
				}
			}"
    ); echo ${resp} | jq ''
    echo ""
#}

#register_spec