#!/bin/bash

#function add-vm-to-infra() {


	TestSetFile=${6:-../testSet.env}
    
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	
	echo "####################################################################"
	echo "## 8. vm: Create Infra"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}
	EXPAND=${4:-1}
	InfraNAME=${5:-noname}
	

	source ../common-functions.sh
	getCloudIndex $CSP

	InfraID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}

	if [ "${InfraNAME}" != "noname" ]; then
		echo "[Infra name is given]"
		InfraID=${InfraNAME}
	else
		InfraID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
	fi

	#echo $i
	INDEXY=${EXPAND}
	for ((cspj=4;cspj<INDEXY+4;cspj++)); do
		#echo $j
		VMID=${InfraID}-0${cspj}

		echo $InfraID
		echo $VMID

		curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/infra/$InfraID/vm -H 'Content-Type: application/json' -d \
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
		}' | jq '.' 
		

	done




#add-vm-to-infra