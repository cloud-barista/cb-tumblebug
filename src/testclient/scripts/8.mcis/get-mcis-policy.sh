#!/bin/bash

echo "####################################################################"
echo "## 8. VM: Get MCIS Policy"
echo "####################################################################"

source ../init.sh

curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/policy/mcis/${MCISID} | jq ''
