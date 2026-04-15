#!/bin/bash

#function resume_infra() {

echo "####################################################################"
echo "## 8. VM: Resume from suspended Infra"
echo "####################################################################"

source ../init.sh

if [ "${INDEX}" == "0" ]; then
	InfraID=${POSTFIX}
else
	InfraID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
fi

echo "${InfraID}"

ControlCmd=resume
curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/control/infra/${InfraID}?action=${ControlCmd} | jq '.'

#}

#resume_infra
