#!/bin/bash

echo "####################################################################"
echo "## 8. VM: Get MCI Policy"
echo "####################################################################"

source ../init.sh

curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/policy/mci/${MCIID} | jq ''
