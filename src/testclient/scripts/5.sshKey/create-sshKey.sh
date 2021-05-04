#!/bin/bash

#function create_sshKey() {


	TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	
	echo "####################################################################"
	echo "## 5. sshKey: Create"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	source ../common-functions.sh
	getCloudIndex $CSP

	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/resources/sshKey -H 'Content-Type: application/json' -d \
		'{ 
			"connectionName": "'${CONN_CONFIG[$INDEX,$REGION]}'", 
			"name": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'", 
			"username": "ubuntu"
		}' | jq '.message'
#}

#create_sshKey