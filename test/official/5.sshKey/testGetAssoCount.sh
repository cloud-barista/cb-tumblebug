#!/bin/bash

#function get_sshKey() {
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
	echo "## 5. sshKey: Get association count"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	source ../common-functions.sh
	getCloudIndex $CSP

	curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NS_ID/testGetAssociatedObjectCount/sshKey/${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX} -H 'Content-Type: application/json' -d \
		'{ 
			"ConnectionName": "'${CONN_CONFIG[$INDEX,$REGION]}'"
		}' #| json_pp #|| return 1
#}

#get_sshKey
