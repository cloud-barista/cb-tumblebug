#!/bin/bash

#function create_securityGroup() {


	TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	
	echo "####################################################################"
	echo "## 3. securityGroup: Status"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	source ../common-functions.sh
	getCloudIndex $CSP

    resp=$(
        curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/inspectResources -H 'Content-Type: application/json' -d @- <<EOF
        {
			"connectionName": "${CONN_CONFIG[$INDEX,$REGION]}",
			"type": "securityGroup"
		}
EOF
    ); echo ${resp} | jq ''
    echo ""
#}

#create_securityGroup
