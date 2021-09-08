#!/bin/bash

#function just_terminate_mcis() {

echo "####################################################################"
echo "## 8. VM: Just Terminate MCIS"
echo "####################################################################"

source ../init.sh

# if [ "${INDEX}" == "0" ]; then
# 	MCISID=${MCISPREFIX}-${POSTFIX}
# else
# 	MCISID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
# fi

echo "${MCISID}"

ControlCmd=terminate
curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/control/mcis/${MCISID}?action=${ControlCmd} | jq ''


#just_terminate_mcis
