#!/bin/bash

#function resume_mcis() {

echo "####################################################################"
echo "## 8. VM: Resume from suspended MCIS"
echo "####################################################################"

source ../init.sh

if [ "${INDEX}" == "0" ]; then
	MCISID=${POSTFIX}
else
	MCISID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
fi

echo "${MCISID}"

ControlCmd=resume
curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/control/mcis/${MCISID}?action=${ControlCmd} | jq ''

#}

#resume_mcis
