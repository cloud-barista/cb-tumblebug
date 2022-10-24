#!/bin/bash

echo "####################################################################"
echo "## 10. NLB: Get"
echo "####################################################################"

source ../init.sh

resp=$(
	curl -H "${AUTH}" -sX GET http://$SpiderServer/spider/nlb/${NSID}-${CONN_CONFIG[$INDEX,$REGION]} -H 'Content-Type: application/json' -d @- <<EOF
        { 
			"ConnectionName": "${CONN_CONFIG[$INDEX,$REGION]}"
		}
EOF
	); echo ${resp} | jq ''
    echo ""
