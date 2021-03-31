#!/bin/bash

#function unregister_spec() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

	source ../conf.env
	AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

	echo "####################################################################"
	echo "## 7. spec: Unregister"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	source ../common-functions.sh
	getCloudIndex $CSP

	curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NS_ID/resources/spec/${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX} | json_pp #-H 'Content-Type: application/json' -d \
#		'{ 
#			"ConnectionName": "'${CONN_CONFIG[$INDEX,$REGION]}'"
#		}' | json_pp #|| return 1

#}

#unregister_spec
