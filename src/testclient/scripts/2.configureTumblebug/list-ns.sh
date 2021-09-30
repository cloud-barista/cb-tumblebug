#!/bin/bash

echo "####################################################################"
echo "## 0. Namespace: List"
echo "####################################################################"

source ../conf.env

curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns | jq ''
echo ""
