#!/bin/bash

echo "####################################################################"
echo "## 8. Get MCI"
echo "####################################################################"

source ../init.sh


if [ "${INDEX}" == "0" ]; then
	MCIID=${POSTFIX}
else
	MCIID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
fi

curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mci/${MCIID} | jq ''

