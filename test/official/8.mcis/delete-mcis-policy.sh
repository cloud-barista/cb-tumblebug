#!/bin/bash

#function delete_mcis_policy() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

	source ../conf.env
	AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

	echo "####################################################################"
	echo "## 8. Delete MCIS Policy"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}
	MCISNAME=${4:-noname}

	source ../common-functions.sh
	getCloudIndex $CSP


	MCISID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}

	if [ "${INDEX}" == "0" ]; then
		MCISPREFIX=avengers
		MCISID=${MCISPREFIX}-${POSTFIX}
	fi

	if [ "${MCISNAME}" != "noname" ]; then
		echo "[MCIS name is given]"
		MCISID=${MCISNAME}
	fi
	

	curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NS_ID/policy/mcis/${MCISID} | json_pp || return 1


#}

#terminate_and_delete_mcis