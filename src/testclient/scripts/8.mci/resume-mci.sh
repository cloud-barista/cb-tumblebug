#!/bin/bash

#function resume_mci() {

echo "####################################################################"
echo "## 8. VM: Resume from suspended MCI"
echo "####################################################################"

source ../init.sh

if [ "${INDEX}" == "0" ]; then
	MCIID=${POSTFIX}
else
	MCIID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
fi

echo "${MCIID}"

ControlCmd=resume
curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/control/mci/${MCIID}?action=${ControlCmd} | jq ''

#}

#resume_mci
