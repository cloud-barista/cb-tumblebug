#!/bin/bash

#function suspend_mci() {

echo "####################################################################"
echo "## 8. VM: Suspend MCI"
echo "####################################################################"

source ../init.sh

if [ "${INDEX}" == "0" ]; then
	MCIID=${POSTFIX}
else
	MCIID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
fi

echo "${MCIID}"

ControlCmd=suspend
curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/control/mci/${MCIID}?action=${ControlCmd} | jq ''


#suspend_mci
