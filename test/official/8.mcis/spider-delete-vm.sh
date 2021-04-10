#!/bin/bash

#function spider_delete_mcis() {
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

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	source ../common-functions.sh
	getCloudIndex $CSP
	#curl -H "${AUTH}" -sX DELETE http://$SpiderServer/spider/vm/${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX} -H
	curl -H "${AUTH}" -sX DELETE http://$SpiderServer/spider/vm/alibaba-ap-northeast-1-shson-01 -H 'Content-Type: application/json' -d \
		'{ 
			"ConnectionName": "'${CONN_CONFIG[$INDEX,$REGION]}'"
		}' | json_pp
#}

#spider_delete_mcis