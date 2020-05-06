#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 1. VPC: Delete"
echo "####################################################################"
curl -sX DELETE http://localhost:1323/tumblebug/ns/$NS_ID/resources/vNet/VPC-01 -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp #|| return 1

