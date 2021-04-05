#!/bin/bash

#function lookup_spec() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

	source ../conf.env
	AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

	echo "####################################################################"
	echo "## 7. spec: Lookup Spec"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	source ../common-functions.sh
	getCloudIndex $CSP


	curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/lookupSpec -H 'Content-Type: application/json' -d \
		'{ 
			"connectionName": "'${CONN_CONFIG[$INDEX,$REGION]}'",
            "cspSpecName": "'${SPEC_NAME[$INDEX,$REGION]}'"
		}' | json_pp #|| return 1
#}

#lookup_spec