#!/bin/bash

#function add-vm-to-mcis() {


	TestSetFile=${5:-../testSet.env}
    
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
	MCISPREFIX=${4:-avengers}
	MCISID=${MCISPREFIX}-${POSTFIX}

	source ../common-functions.sh
	getCloudIndex $CSP

	
	$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm mcis add-vm --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml  -i json -o json -d \
	"{
		\"nsId\":  \"${NSID}\",
		\"mcisId\": \"${MCISID}\",
		\"mcisvm\": {
			\"name\": \"${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}\",
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
	}" | jq '' 
#}

#add-vm-to-mcis