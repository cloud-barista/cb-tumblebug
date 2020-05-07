#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 2. SecurityGroup: Delete"
echo "####################################################################"

INDEX=${1-"1"}

curl -sX DELETE http://localhost:1323/tumblebug/ns/$NS_ID/resources/securityGroup/SG-0$INDEX -H 'Content-Type: application/json' -d \
    '{ 
        "ConnectionName": "'${CONN_CONFIG[INDEX]}'"
    }' | json_pp #|| return 1

