#!/bin/bash

echo "####################################################################"
echo "## 8. VM: Refine MCI (remove failed VMs)"
echo "####################################################################"

source ../init.sh

if [ "${INDEX}" == "0" ]; then
	MCIID=${POSTFIX}
else
	MCIID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
fi

echo "${MCIID}"

ControlCmd=refine
curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/control/mci/${MCIID}?action=${ControlCmd} | jq '.'
