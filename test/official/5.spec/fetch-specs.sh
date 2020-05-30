#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 5. spec: Fetch"
echo "####################################################################"

curl -sX POST http://$TumblebugServer/tumblebug/ns/$NS_ID/resources/fetchSpecs | json_pp #|| return 1