#!/bin/bash

#function terminate_and_delete_mcis() {

echo "####################################################################"
echo "## 8. VM: Delete MCIS"
echo "####################################################################"

source ../init.sh

MCISID=TBD
if [ "${INDEX}" == "0" ]; then
	MCISID=${MCISPREFIX}-${POSTFIX}
else
	MCISID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
fi

echo "${INDEX} ${REGION} ${MCISID}"

echo ""
echo "Delete [MCIS: $MCISID]"
curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID}?option=${OPTION} | jq ''

#}

#terminate_and_delete_mcis
