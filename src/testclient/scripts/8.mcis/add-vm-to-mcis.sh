#!/bin/bash

echo "####################################################################"
echo "## 8. vm: Add VM to MCIS"
echo "####################################################################"

source ../init.sh

# NUMVM=${OPTION01:-1}

if [[ -z "${DISK_TYPE[$INDEX,$REGION]}" ]]; then
	RootDiskType="default"
else
	RootDiskType="${DISK_TYPE[$INDEX,$REGION]}"
fi

if [[ -z "${DISK_SIZE[$INDEX,$REGION]}" ]]; then
	RootDiskSize="default"
else
	RootDiskSize="${DISK_SIZE[$INDEX,$REGION]}"
fi

curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/mcis/$MCISID/vm -H 'Content-Type: application/json' -d \
		'{
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
