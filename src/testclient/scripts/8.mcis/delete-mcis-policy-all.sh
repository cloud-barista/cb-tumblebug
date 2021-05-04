#!/bin/bash

#function delete_mcis_policy() {


	TestSetFile=${5:-../testSet.env}
    
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	
	echo "####################################################################"
	echo "## 8. Delete MCIS Policy ALL"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}
	MCISPREFIX=${4}

	source ../common-functions.sh
	getCloudIndex $CSP
	
	curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID/policy/mcis | jq '' 

#}

#terminate_and_delete_mcis