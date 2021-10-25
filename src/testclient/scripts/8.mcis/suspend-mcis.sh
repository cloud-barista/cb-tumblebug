#!/bin/bash

#function suspend_mcis() {

echo "####################################################################"
echo "## 8. VM: Suspend MCIS"
echo "####################################################################"

source ../init.sh

if [ "${INDEX}" == "0" ]; then
	MCISID=${POSTFIX}
else
	MCISID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
fi

echo "${MCISID}"

ControlCmd=suspend
curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/control/mcis/${MCISID}?action=${ControlCmd} | jq ''


#suspend_mcis
