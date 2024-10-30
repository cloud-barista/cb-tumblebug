#!/bin/bash

#function reboot_mci() {

echo "####################################################################"
echo "## 8. VM: Reboot MCI"
echo "####################################################################"

source ../init.sh

if [ "${INDEX}" == "0" ]; then
	MCIID=${POSTFIX}
else
	MCIID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
fi

echo "${MCIID}"

ControlCmd=reboot
curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/control/mci/${MCIID}?action=${ControlCmd} | jq '.'

#}

#reboot_mci
