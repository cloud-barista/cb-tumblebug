#!/bin/bash

echo "####################################################################"
echo "## 8. Delete MCIS Policy"
echo "####################################################################"

source ../init.sh

curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID/policy/mcis/${MCISID} | jq ''
