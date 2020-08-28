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

	MCISID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}

	#echo $i
	INDEXY=${EXPAND}
	for ((cspj=4;cspj<INDEXY+4;cspj++)); do
		#echo $j
		VMID=${MCISID}-0${cspj}

		echo $MCISID
		echo $VMID

		$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm mcis add-vm --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml  -i json -o json -d \
		'{
			"nsId":  "'${NS_ID}'",
			"mcisId":  "'${MCISID}'",
			"mcisvm": {
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
			}
		}'		

	done




#add-vm-to-mcis