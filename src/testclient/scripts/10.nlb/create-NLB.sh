#!/bin/bash

echo "####################################################################"
echo "## 10. NLB: Create"
echo "####################################################################"

source ../init.sh

resp=$(
	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID}/nlb -H 'Content-Type: application/json' -d @- <<EOF
	{
		"type": "PUBLIC",
		"scope": "REGION",
		"listener": {
			"Protocol": "TCP",
			"Port": "22"
		},
		"targetGroup": {
			"Protocol" : "TCP",
			"Port" : "22",
			"subGroupId": "${CONN_CONFIG[$INDEX,$REGION]}"
		},
		"HealthChecker": {
			"Interval" : "default",
			"Timeout" : "default",
			"Threshold" : "default"
		}
	}
EOF
    ); echo ${resp} | jq ''
    echo ""
