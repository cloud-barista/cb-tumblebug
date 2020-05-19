#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 5. spec: Fetch"
echo "####################################################################"

curl -sX POST http://localhost:1323/tumblebug/ns/$NS_ID/resources/fetchSpecs | json_pp #|| return 1