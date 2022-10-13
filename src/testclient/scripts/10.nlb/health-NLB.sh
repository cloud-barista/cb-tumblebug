#!/bin/bash

echo "####################################################################"
echo "## 10. NLB: Get Health"
echo "####################################################################"

source ../init.sh

resp=$(
	curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID}/nlb/${CONN_CONFIG[$INDEX,$REGION]}/healthz
	); echo ${resp} | jq ''
    echo ""
