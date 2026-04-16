#!/bin/bash

echo "####################################################################"
echo "## 8. VM: List ID"
echo "####################################################################"

source ../init.sh

if [ "${INDEX}" == "0" ]; then
	InfraID=${POSTFIX}
else
	InfraID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
fi

echo "${InfraID}"

curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/infra/${InfraID}/nodegroup | jq '.'


#get_infra
