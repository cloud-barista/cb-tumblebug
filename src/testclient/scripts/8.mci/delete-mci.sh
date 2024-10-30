#!/bin/bash

#function terminate_and_delete_mci() {

echo "####################################################################"
echo "## 8. VM: Delete MCI (-x for option. ex: -x terminate)"
echo "####################################################################"

source ../init.sh

OPTION=$OPTION01
echo "${INDEX} ${REGION} ${MCIID}"

echo ""
echo "Delete [MCI: $MCIID]"
curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID/mci/${MCIID}?option=${OPTION} | jq '.'

#}

#terminate_and_delete_mci
