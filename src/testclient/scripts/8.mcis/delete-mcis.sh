#!/bin/bash

#function terminate_and_delete_mcis() {

echo "####################################################################"
echo "## 8. VM: Delete MCIS (-x for option. ex: -x terminate)"
echo "####################################################################"

source ../init.sh

OPTION=$OPTION01
echo "${INDEX} ${REGION} ${MCISID}"

echo ""
echo "Delete [MCIS: $MCISID]"
curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID}?option=${OPTION} | jq ''

#}

#terminate_and_delete_mcis
