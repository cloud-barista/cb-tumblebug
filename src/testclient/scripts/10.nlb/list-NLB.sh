#!/bin/bash

echo "####################################################################"
echo "## 10. NLB: List"
echo "####################################################################"

source ../init.sh

curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mcis/$MCISID/nlb | jq ''
