#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 0. Namespace: Create"
echo "####################################################################"

INDEX=${1}

curl -sX GET http://localhost:1323/tumblebug/ns | json_pp #|| return 1
