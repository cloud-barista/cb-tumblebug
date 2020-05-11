#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 0. Namespace: List"
echo "####################################################################"

INDEX=${1}

curl -sX GET http://localhost:1323/tumblebug/ns | json_pp #|| return 1
