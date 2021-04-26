#!/bin/bash

#function register_spec() {


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
	echo "## 7. spec: Register"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	source ../common-functions.sh
	getCloudIndex $CSP

	resp=$(
        curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NS_ID/resources/spec -H 'Content-Type: application/json' -d @- <<EOF
		{ 
			"connectionName": "${CONN_CONFIG[$INDEX,$REGION]}", 
			"name": "${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}",
			"cspSpecName": "${SPEC_NAME[$INDEX,$REGION]}"
		}
EOF
    ); echo ${resp} | jq ''
    echo ""
#}

#register_spec