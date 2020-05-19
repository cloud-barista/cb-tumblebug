#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 2. SecurityGroup: List"
echo "####################################################################"


curl -sX GET http://localhost:1323/tumblebug/ns/$NS_ID/resources/securityGroup | json_pp #|| return 1

