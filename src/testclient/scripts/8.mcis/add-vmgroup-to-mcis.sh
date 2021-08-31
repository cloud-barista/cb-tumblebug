#!/bin/bash

#function add-vm-to-mcis() {

	echo "####################################################################"
	echo "## 8. vm: Create MCIS"
	echo "####################################################################"

	source ../init.sh

	NUMVM=${OPTION01:-1}
	
	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/mcis/$MCISID/vmgroup -H 'Content-Type: application/json' -d \
		'{
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
		}' | jq '' 
#}

#add-vm-to-mcis