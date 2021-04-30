#!/bin/bash

#function just_terminate_mcis() {


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
	echo "## 8. VM: Just Terminate MCIS"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}
	MCISPREFIX=${5}

	source ../common-functions.sh
	getCloudIndex $CSP

	if [ -z "$MCISPREFIX" ]
	then
		curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}?action=terminate | jq ''
	else
		MCISID=${MCISPREFIX}-${POSTFIX}
		curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID}?action=terminate | jq ''
	fi
#}

#just_terminate_mcis