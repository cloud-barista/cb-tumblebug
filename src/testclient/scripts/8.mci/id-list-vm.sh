#!/bin/bash

echo "####################################################################"
echo "## 8. VM: List ID"
echo "####################################################################"

source ../init.sh

if [ "${INDEX}" == "0" ]; then
	MCIID=${POSTFIX}
else
	MCIID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
fi

echo "${MCIID}"

curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mci/${MCIID}?option=id | jq '.'


#get_mci
