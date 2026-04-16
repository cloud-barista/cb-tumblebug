#!/bin/bash

#function reboot_infra() {

echo "####################################################################"
echo "## 8. VM: Reboot Infra"
echo "####################################################################"

source ../init.sh

if [ "${INDEX}" == "0" ]; then
	InfraID=${POSTFIX}
else
	InfraID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
fi

echo "${InfraID}"

ControlCmd=reboot
curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/control/infra/${InfraID}?action=${ControlCmd} | jq '.'

#}

#reboot_infra
