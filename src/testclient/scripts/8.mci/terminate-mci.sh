#!/bin/bash

#function just_terminate_infra() {

echo "####################################################################"
echo "## 8. VM: Just Terminate Infra"
echo "####################################################################"

source ../init.sh

# if [ "${INDEX}" == "0" ]; then
# 	InfraID=${POSTFIX}
# else
# 	InfraID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
# fi

echo "${InfraID}"

ControlCmd=terminate
curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/control/infra/${InfraID}?action=${ControlCmd} | jq '.'


#just_terminate_infra
