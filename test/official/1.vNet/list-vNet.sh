#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 1. VPC: Get"
echo "####################################################################"

curl -sX GET http://$TumblebugServer/tumblebug/ns/$NS_ID/resources/vNet | json_pp #|| return 1

