#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 2. SecurityGroup: List"
echo "####################################################################"


curl -sX GET http://$TumblebugServer/tumblebug/ns/$NS_ID/resources/securityGroup | json_pp #|| return 1

