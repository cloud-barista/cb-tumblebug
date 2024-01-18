#!/bin/bash

echo "####################################################################"
echo "## 13. CLUSTER: List"
echo "####################################################################"

source ../init.sh

echo "NSID: "${NSID}

curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/cluster | jq ''
