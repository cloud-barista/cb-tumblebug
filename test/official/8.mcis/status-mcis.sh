#!/bin/bash

#function status_mcis() {
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
	echo "## 8. VM: Status MCIS"
	echo "####################################################################"


	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	source ../common-functions.sh
	getCloudIndex $CSP

	MCISPREFIX=${5}
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
	curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NS_ID/mcis/${MCISID}?action=status | jq 
	
	#HTTP_CODE=$(curl -o /dev/null -w "%{http_code}\n" -H "${AUTH}" "http://${TumblebugServer}/tumblebug/ns/$NS_ID/mcis/${MCISID}?action=status" --silent)
	#echo "Status: $HTTP_CODE" 

#}

#status_mcis