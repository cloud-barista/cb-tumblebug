#!/bin/bash

echo "####################################################################"
echo "## 8. Delete MCIS Policy ALL"
echo "####################################################################"

source ../init.sh

curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID/policy/mcis | jq ''
