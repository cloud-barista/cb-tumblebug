#!/bin/bash

#function create_mcis() {


	TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	
	echo "####################################################################"
	echo "## 8. vm: Create MCIS"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	source ../common-functions.sh
	getCloudIndex $CSP

	$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm mcis create --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml  -i json -o json -d \
	"{
		\"nsId\":  \"${NSID}\",
		\"mcis\": {
				\"name\": \"${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}\",
				\"description\": \"Tumblebug Demo\",
				\"installMonAgent\": \"no\",
				\"vm\": [ {
					\"name\": \"${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}-01\",
					\"imageId\": \"${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}\",
					\"vmUserAccount\": \"cb-user\",
					\"connectionName\": \"${CONN_CONFIG[$INDEX,$REGION]}\",
					\"sshKeyId\": \"${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}\",
					\"specId\": \"${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}\",
					\"securityGroupIds\": [
						\"${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}\"
					],
					\"vNetId\": \"${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}\",
					\"subnetId\": \"${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}\",
					\"description\": \"description\",
					\"vmUserPassword\": \"\"
				}
				]
		}
	}" | jq '' 
#}

#create_mcis