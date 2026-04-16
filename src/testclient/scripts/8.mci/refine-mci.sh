#!/bin/bash

echo "####################################################################"
echo "## 8. VM: Refine Infra (remove failed VMs)"
echo "####################################################################"

source ../init.sh

if [ "${INDEX}" == "0" ]; then
	InfraID=${POSTFIX}
else
	InfraID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
fi

echo "${InfraID}"

ControlCmd=refine
curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/control/infra/${InfraID}?action=${ControlCmd} | jq '.'
