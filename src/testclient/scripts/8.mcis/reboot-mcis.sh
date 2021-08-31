#!/bin/bash

#function reboot_mcis() {

echo "####################################################################"
echo "## 8. VM: Reboot MCIS"
echo "####################################################################"

source ../init.sh

if [ "${INDEX}" == "0" ]; then
	MCISID=${MCISPREFIX}-${POSTFIX}
else
	MCISID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
fi

echo "${MCISID}"

ControlCmd=reboot
curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/control/mcis/${MCISID}?action=${ControlCmd} | jq ''

#}

#reboot_mcis
