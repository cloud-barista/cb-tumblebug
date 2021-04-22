#!/bin/bash

#function create_securityGroup() {


	TestSetFile=${4:-../testSet.env}
    
    FILE=$TestSetFile
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

	echo "####################################################################"
	echo "## 4. SecurityGroup: Create"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	source ../common-functions.sh
	getCloudIndex $CSP

    resp=$(
        curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NS_ID/resources/securityGroup -H 'Content-Type: application/json' -d @- <<EOF
        {
			"name": "${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}",
			"connectionName": "${CONN_CONFIG[$INDEX,$REGION]}",
			"vNetId": "${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}",
			"description": "test description",
				"firewallRules": [
					{
						"FromPort": "1",
						"ToPort": "65535",
						"IPProtocol": "tcp",
						"Direction": "inbound"
					},
					{
						"FromPort": "1",
						"ToPort": "65535",
						"IPProtocol": "udp",
						"Direction": "inbound"
					},
					{
						"FromPort": "-1",
						"ToPort": "-1",
						"IPProtocol": "icmp",
						"Direction": "inbound"
					}
				]
			}
EOF
    ); echo ${resp} | jq ''
    echo ""
#}

#create_securityGroup