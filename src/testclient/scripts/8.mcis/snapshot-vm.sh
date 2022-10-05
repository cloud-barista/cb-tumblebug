#!/bin/bash

echo "####################################################################"
echo "## 8. vm: Snapshot"
echo "####################################################################"

source ../init.sh

NUMVM=${OPTION01:-1}

curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID}/vm/${CONN_CONFIG[$INDEX,$REGION]}-1/snapshot -H 'Content-Type: application/json' -d \
		'{
			"name": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'"
		}' | jq ''
