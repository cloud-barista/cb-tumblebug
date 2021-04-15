#!/bin/bash

#function terminate_and_delete_mcis() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

	TestSetFile=${4:-../testSet.env}
    
    FILE=$TestSetFile
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

	echo "####################################################################"
	echo "## 8. VM: Terminate and Delete MCIS"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}
	MCISPREFIX=${5}
	GIVENMCISID=${6}

	source ../common-functions.sh
	getCloudIndex $CSP
	

	if [ -z "$MCISPREFIX" ]
	then
		curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NS_ID/mcis/${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX} | jq '' 
	else
		if [ ! -z "$GIVENMCISID" ]
		then
			MCISID=${GIVENMCISID}
		else
			MCISID=${MCISPREFIX}-${POSTFIX}
		fi
		echo ""
		echo "Terminate and Delete [MCIS: $MCISID]"
		curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NS_ID/mcis/${MCISID} | jq '' 
	fi


#}

#terminate_and_delete_mcis