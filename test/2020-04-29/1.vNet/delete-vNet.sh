#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 1. VPC: Delete"
echo "####################################################################"

INDEX=${1}

curl -sX DELETE http://localhost:1323/tumblebug/ns/$NS_ID/resources/vNet/VPC-0$INDEX -H 'Content-Type: application/json' -d \
    '{ 
        "ConnectionName": "'${CONN_CONFIG[INDEX]}'"
    }' | json_pp #|| return 1

