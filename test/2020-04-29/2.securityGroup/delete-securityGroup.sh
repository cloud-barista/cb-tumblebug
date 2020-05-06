#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 2. SecurityGroup: Delete"
echo "####################################################################"
curl -sX DELETE http://localhost:1323/tumblebug/ns/$NS_ID/resources/securityGroup/SG-01 -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp #|| return 1

