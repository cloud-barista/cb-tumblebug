#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 0. Namespace: Delete"
echo "####################################################################"

INDEX=${1}

curl -sX DELETE http://$TumblebugServer/tumblebug/ns/$NS_ID | json_pp #|| return 1
