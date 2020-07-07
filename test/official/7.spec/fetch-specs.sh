#!/bin/bash

source ../conf.env
AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

echo "####################################################################"
echo "## 7. spec: Fetch"
echo "####################################################################"

curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NS_ID/resources/fetchSpecs | json_pp #|| return 1