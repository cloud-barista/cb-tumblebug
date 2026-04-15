#!/bin/bash

echo "####################################################################"
echo "## 10. NLB: Delete"
echo "####################################################################"

source ../init.sh

resp=$(
	curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID/infra/${InfraID}/nlb
	); echo ${resp} | jq '.'
    echo ""
