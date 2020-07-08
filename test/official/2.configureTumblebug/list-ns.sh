#!/bin/bash

source ../conf.env
AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

echo "####################################################################"
echo "## 0. Namespace: List"
echo "####################################################################"

INDEX=${1}

curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns | json_pp #|| return 1
