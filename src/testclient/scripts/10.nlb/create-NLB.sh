#!/bin/bash

echo "####################################################################"
echo "## 10. NLB: Create"
echo "####################################################################"

source ../init.sh

resp=$(
	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID}/nlb -H 'Content-Type: application/json' -d @- <<EOF
	{
		"name": "${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}",
		"connectionName": "${CONN_CONFIG[$INDEX,$REGION]}",
		"vNetId": "${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}",
		"type": "PUBLIC",
		"scope": "REGION",
		"listener": {
			"Protocol": "TCP",
			"Port": "22",
			"DNSName": ""
		},
		"targetGroup": {
			"Protocol" : "TCP",
			"Port" : "22",
			"vmGroupId": "${CONN_CONFIG[$INDEX,$REGION]}"
		},
		"HealthChecker": {
			"Protocol" : "TCP",
			"Port" : "22",
			"Interval" : "10",
			"Timeout" : "9",
			"Threshold" : "3"
		}
	}
EOF
    ); echo ${resp} | jq ''
    echo ""
