#!/bin/bash

#function spider_delete_mci() {


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
	#curl -H "${AUTH}" -sX DELETE http://$SpiderServer/spider/vm/${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX} -H
	curl -H "${AUTH}" -sX DELETE http://$SpiderServer/spider/vm/alibaba-ap-northeast-1-shson-01 -H 'Content-Type: application/json' -d \
		'{ 
			"ConnectionName": "'${CONN_CONFIG[$INDEX,$REGION]}'"
		}' | jq '.'
#}

#spider_delete_mci