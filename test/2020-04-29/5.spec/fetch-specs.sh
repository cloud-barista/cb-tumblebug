#!/bin/bash

source ../conf.env

INDEX=${1}

echo "####################################################################"
echo "## 4. spec: Fetch"
echo "####################################################################"

curl -sX POST http://localhost:1323/tumblebug/ns/$NS_ID/resources/fetchSpecs #| json_pp
