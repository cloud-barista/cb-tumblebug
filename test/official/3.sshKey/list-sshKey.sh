#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 3. sshKey: List"
echo "####################################################################"

curl -sX GET http://$TumblebugServer/tumblebug/ns/$NS_ID/resources/sshKey -H 'Content-Type: application/json' -d \
    '{ 
        "ConnectionName": "'${CONN_CONFIG[$INDEX,$REGION]}'"
    }' | json_pp #|| return 1
