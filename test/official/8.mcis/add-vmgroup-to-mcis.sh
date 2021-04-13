#!/bin/bash

#function add-vm-to-mcis() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

	TestSetFile=${6:-../testSet.env}
    
    FILE=$TestSetFile
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

	echo "####################################################################"
	echo "## 8. vm: Create MCIS"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}
	NUMVM=${4:-3}
	MCISPREFIX=${5:-avengers}
	MCISID=${MCISPREFIX}-${POSTFIX}

	source ../common-functions.sh
	getCloudIndex $CSP

	
	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NS_ID/mcis/$MCISID/vmgroup -H 'Content-Type: application/json' -d \
		'{
			"vmGroupSize": "'${NUMVM}'",
			"name": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'",
			"imageId": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'",
			"vmUserAccount": "cb-user",
			"connectionName": "'${CONN_CONFIG[$INDEX,$REGION]}'",
			"sshKeyId": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'",
			"specId": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'",
			"securityGroupIds": [
				"'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'"
			],
			"vNetId": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'",
			"subnetId": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'",
			"description": "description",
			"vmUserPassword": ""
		}' | jq || return 1
#}

#add-vm-to-mcis