#!/bin/bash

#function just_terminate_mci() {

echo "####################################################################"
echo "## 8. VM: Just Terminate MCI"
echo "####################################################################"

source ../init.sh

# if [ "${INDEX}" == "0" ]; then
# 	MCIID=${POSTFIX}
# else
# 	MCIID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
# fi

echo "${MCIID}"

ControlCmd=terminate
curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/control/mci/${MCIID}?action=${ControlCmd} | jq '.'


#just_terminate_mci
