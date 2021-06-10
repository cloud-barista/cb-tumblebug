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
	
	NUMVM=${5:-3}

	source ../common-functions.sh
	getCloudIndex $CSP

	echo "####################"
	echo " AgentInstallOn: $AgentInstallOn"
	echo "####################"

	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/mcis -H 'Content-Type: application/json' -d \
		'{
			"name": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'",
			"description": "Tumblebug Demo",
			"installMonAgent": "'${AgentInstallOn}'",
			"vm": [ {
				"vmGroupSize": "'${NUMVM}'",
				"name": "'${CONN_CONFIG[$INDEX,$REGION]}'",
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
		}' | jq '' 
#}

#create_mcis