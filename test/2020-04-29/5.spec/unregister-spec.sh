#!/bin/bash

source ../conf.env

INDEX=${1}

echo "####################################################################"
echo "## 3. spec: Unregister"
echo "####################################################################"

curl -sX DELETE http://localhost:1323/tumblebug/ns/$NS_ID/resources/spec/SPEC-0$INDEX -H 'Content-Type: application/json' -d \
    '{ 
        "ConnectionName": "'${CONN_CONFIG[INDEX]}'"
    }' | json_pp #|| return 1
