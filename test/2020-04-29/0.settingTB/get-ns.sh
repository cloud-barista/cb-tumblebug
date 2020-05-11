#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 0. Namespace: Get"
echo "####################################################################"

INDEX=${1}

curl -sX GET http://localhost:1323/tumblebug/ns/$NS_ID | json_pp #|| return 1
