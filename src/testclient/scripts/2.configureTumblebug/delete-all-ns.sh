#!/bin/bash

echo "####################################################################"
echo "## 0. Namespace: Delete ALL"
echo "####################################################################"

source ../init.sh

curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns | jq ''
echo ""
