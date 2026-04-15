#!/bin/bash

echo "####################################################################"
echo "## 8. Get Infra"
echo "####################################################################"

source ../init.sh


if [ "${INDEX}" == "0" ]; then
	InfraID=${POSTFIX}
else
	InfraID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
fi

curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/infra/${InfraID} | jq '.'

