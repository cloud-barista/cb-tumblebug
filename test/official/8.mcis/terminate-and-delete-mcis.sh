#!/bin/bash

#function terminate_and_delete_mcis() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

	source ../conf.env
	AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

	echo "####################################################################"
	echo "## 8. VM: Terminate and Delete MCIS"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}
	MCISPREFIX=${4}

	source ../common-functions.sh
	getCloudIndex $CSP
	

	if [ -z "$MCISPREFIX" ]
	then
		curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NS_ID/mcis/${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX} | json_pp || return 1
	else
		MCISID=${MCISPREFIX}-${POSTFIX}
		curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NS_ID/mcis/${MCISID} | json_pp || return 1
	fi
#}

#terminate_and_delete_mcis