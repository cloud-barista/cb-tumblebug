#!/bin/bash

echo "####################################################################"
echo "## 10. NLB: Remove VM"
echo "####################################################################"

source ../init.sh

resp=$(
	curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID}/nlb/${CONN_CONFIG[$INDEX,$REGION]}/vm -H 'Content-Type: application/json' -d @- <<EOF
	{
		"targetGroup": {
			"VMs" : [
				"${CONN_CONFIG[$INDEX,$REGION]}-1"
				]
		}
	}
EOF
    ); echo ${resp} | jq ''
    echo ""
