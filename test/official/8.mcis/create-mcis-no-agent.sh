#!/bin/bash

#function create_mcis() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

	source ../conf.env
	AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

	echo "####################################################################"
	echo "## 8. vm: Create MCIS"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}
	if [ "${CSP}" == "aws" ]; then
		echo "[Test for AWS]"
		INDEX=1
	elif [ "${CSP}" == "azure" ]; then
		echo "[Test for Azure]"
		INDEX=2
	elif [ "${CSP}" == "gcp" ]; then
		echo "[Test for GCP]"
		INDEX=3
	elif [ "${CSP}" == "alibaba" ]; then
		echo "[Test for Alibaba]"
		INDEX=4
	else
		echo "[No acceptable argument was provided (aws, azure, gcp, alibaba, ...). Default: Test for AWS]"
		CSP="aws"
		INDEX=1
	fi

	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NS_ID/mcis -H 'Content-Type: application/json' -d \
		'{
			"name": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'",
			"description": "Tumblebug Demo",
			"installMonAgent": "no",
			"vm": [ {
				"name": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'-01",
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