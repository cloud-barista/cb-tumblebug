#!/bin/bash

echo "####################################################################"
echo "## 10. NLB: List ID"
echo "####################################################################"

source ../init.sh

curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/infra/$InfraID/nlb?option=id | jq '.'
