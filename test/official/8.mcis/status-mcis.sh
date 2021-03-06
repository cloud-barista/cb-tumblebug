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
	if [ "${CSP}" == "all" ]; then
		echo "[Test for all CSP regions (AWS, Azure, GCP, Alibaba, ...)]"
		CSP="aws"
		INDEX=0
	elif [ "${CSP}" == "aws" ]; then
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
		echo "[No acceptable argument was provided (all, aws, azure, gcp, alibaba, ...). Default: Test for AWS]"
		CSP="aws"
		INDEX=1
	fi

	MCISPREFIX=${4}
	if [ -z "$MCISPREFIX" ]
	then
		MCISID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
		if [ "${INDEX}" == "0" ]; then
			MCISPREFIX=avengers
			MCISID=${MCISPREFIX}-${POSTFIX}
		fi
	else
		MCISID=${MCISPREFIX}-${POSTFIX}
	fi
	curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NS_ID/mcis/${MCISID}?action=status | json_pp


#}

#status_mcis