#!/bin/bash

#function spider_create_mcis() {


	TestSetFile=${4:-../testSet.env}
    
    FILE=$TestSetFile
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	source ../common-functions.sh
	getCloudIndex $CSP

	curl -H "${AUTH}" -sX POST http://$SpiderServer/spider/vm -H 'Content-Type: application/json' -d \
		'{ 
			"ConnectionName": "'${CONN_CONFIG[$INDEX,$REGION]}'", 
			"ReqInfo": { 
				"Name": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'-01",
				"ImageName": "'${IMAGE_NAME[$INDEX,$REGION]}'", 
				"VPCName": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'",
				"SubnetName": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'",
				"SecurityGroupNames": [ "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'" ], 
				"VMSpecName": "'${SPEC_NAME[$INDEX,$REGION]}'", 
				"KeyPairName": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'"
			} 
		}' | jq ''
#}

#spider_create_mcis