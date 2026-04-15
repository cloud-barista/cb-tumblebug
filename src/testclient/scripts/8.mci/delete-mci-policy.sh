#!/bin/bash

echo "####################################################################"
echo "## 8. Delete Infra Policy"
echo "####################################################################"

source ../init.sh

curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID/policy/infra/${InfraID} | jq '.'
