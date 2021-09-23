#!/bin/bash

echo "####################################################################"
echo "## 8. VM: Refine MCIS (remove failed VMs)"
echo "####################################################################"

source ../init.sh

if [ "${INDEX}" == "0" ]; then
	MCISID=${MCISPREFIX}-${POSTFIX}
else
	MCISID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
fi

echo "${MCISID}"

ControlCmd=refine
curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/control/mcis/${MCISID}?action=${ControlCmd} | jq ''
