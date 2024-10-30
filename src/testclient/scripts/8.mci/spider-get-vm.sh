#!/bin/bash

#function spider_get_vm() {


	TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	
	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	source ../common-functions.sh
	getCloudIndex $CSP

	curl -H "${AUTH}" -sX GET http://$SpiderServer/spider/vm/${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}-01 -H 'Content-Type: application/json' -d \
		'{ 
			"ConnectionName": "'${CONN_CONFIG[$INDEX,$REGION]}'"
		}' | jq '.'
#}

#spider_get_vm