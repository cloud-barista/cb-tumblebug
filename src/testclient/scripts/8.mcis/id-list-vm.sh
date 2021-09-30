#!/bin/bash

echo "####################################################################"
echo "## 8. VM: List ID"
echo "####################################################################"

source ../init.sh

if [ "${INDEX}" == "0" ]; then
	MCISID=${MCISPREFIX}-${POSTFIX}
else
	MCISID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
fi

echo "${MCISID}"

curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID}?option=id | jq ''


#get_mcis
