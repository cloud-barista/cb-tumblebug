#!/bin/bash

#function terminate_and_delete_infra() {

echo "####################################################################"
echo "## 8. VM: Delete Infra (-x for option. ex: -x terminate)"
echo "####################################################################"

source ../init.sh

OPTION=$OPTION01
echo "${INDEX} ${REGION} ${InfraID}"

echo ""
echo "Delete [Infra: $InfraID]"
curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID/infra/${InfraID}?option=${OPTION} | jq '.'

#}

#terminate_and_delete_infra
