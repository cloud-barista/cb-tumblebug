#!/bin/bash

#function create_mcis() {
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
	echo "## 8. Create MCIS with a single VM"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}
	NUMVM=${4:-3}
	MCISPREFIX=${5:-avengers}
	MCISID=${MCISPREFIX}-${POSTFIX}

	source ../common-functions.sh
	getCloudIndex $CSP

	echo "####################"
	echo " AgentInstallOn: $AgentInstallOn"
	echo "####################"

	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NS_ID/mcis -H 'Content-Type: application/json' -d \
		'{
			"name": "'${MCISID}'",
			"description": "Tumblebug Demo",
			"installMonAgent": "'${AgentInstallOn}'",
			"vm": [ {
				"vmGroupSize": "'${NUMVM}'",
				"name": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'-lead",
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
			}
			]
		}' | json_pp || return 1
#}

#create_mcis