#!/bin/bash

#function add-vm-to-mcis() {
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
	EXPAND=${4:-1}
	MCISNAME=${5:-noname}
	

	source ../common-functions.sh
	getCloudIndex $CSP

	MCISID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}

	if [ "${MCISNAME}" != "noname" ]; then
		echo "[MCIS name is given]"
		MCISID=${MCISNAME}
	else
		MCISID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
	fi

	#echo $i
	INDEXY=${EXPAND}
	for ((cspj=4;cspj<INDEXY+4;cspj++)); do
		#echo $j
		VMID=${MCISID}-0${cspj}

		echo $MCISID
		echo $VMID

		curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NS_ID/mcis/$MCISID/vm -H 'Content-Type: application/json' -d \
		'{
			"name": "'${VMID}'",
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
		}' | json_pp || return 1
		

	done




#add-vm-to-mcis