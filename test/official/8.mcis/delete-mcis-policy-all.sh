#!/bin/bash

#function delete_mcis_policy() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

	TestSetFile=${5:-../testSet.env}
    
    FILE=$TestSetFile
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

	echo "####################################################################"
	echo "## 8. Delete MCIS Policy ALL"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}
	MCISPREFIX=${4}

	source ../common-functions.sh
	getCloudIndex $CSP
	
	curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NS_ID/policy/mcis | json_pp || return 1

#}

#terminate_and_delete_mcis