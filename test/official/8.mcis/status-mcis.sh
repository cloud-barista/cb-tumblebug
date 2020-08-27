#!/bin/bash

#function status_mcis() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

	source ../conf.env
	AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

	echo "####################################################################"
	echo "## 8. VM: Status MCIS"
	echo "####################################################################"


	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}
	MCISPREFIX=${4}
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


	if [ -z "$MCISPREFIX" ]
	then
		curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NS_ID/mcis/${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}?action=status | json_pp
	else
		MCISID=${MCISPREFIX}-${POSTFIX}
		curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NS_ID/mcis/${MCISID}?action=status | json_pp
	fi

	
#}

#status_mcis