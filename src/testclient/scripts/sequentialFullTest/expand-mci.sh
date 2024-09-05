#!/bin/bash

#function add-vm-to-mci() {


	TestSetFile=${6:-../testSet.env}
    
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	
	echo "####################################################################"
	echo "## 8. vm: Create MCI"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}
	EXPAND=${4:-1}
	MCINAME=${5:-noname}
	

	source ../common-functions.sh
	getCloudIndex $CSP

	MCIID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}

	if [ "${MCINAME}" != "noname" ]; then
		echo "[MCI name is given]"
		MCIID=${MCINAME}
	else
		MCIID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
	fi

	#echo $i
	INDEXY=${EXPAND}
	for ((cspj=4;cspj<INDEXY+4;cspj++)); do
		#echo $j
		VMID=${MCIID}-0${cspj}

		echo $MCIID
		echo $VMID

		curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/mci/$MCIID/vm -H 'Content-Type: application/json' -d \
		'{
			"name": "'${VMID}'",
			"imageId": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'",
			"vmUserName": "cb-user",
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
		

	done




#add-vm-to-mci